package main

import (
	"encoding/binary"
	"errors"
	"gotorrent/utils"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

// Identifiers for peer status to denote whether we should attempt to connect to them again or not
const (
	Bad     = -1 // could not connect at all
	Unknown = 0  // have not attempted connected yet
	Dead    = 1  // lost connection some time after handshake + bitfield -- ie we can try to connect to them again
	Alive   = 2  // currently connected
)

// Peer is a connection that we read/write to to download files from, discovered through the Tracker
type Peer struct {
	ip           string
	port         string
	conn         net.Conn
	usesExtended bool // false by default
	extensions   map[string]int
	choked       bool // whether we are choked by this peer or not, will likely need a name change upon seed support
	bitfield     []byte
	status       int

	requests    int // number of pieces that have been requested and not yet fulfilled
	requestsMX  sync.Mutex
	maxRequests int
	pieceQueue  *PieceQueue

	torrent *Torrent // associated torrent

	// wrapped io.Reader/io.Writer interfaces
	pw *PeerWriter
	pr *PeerReader

	logger *log.Logger
}

func (peer *Peer) setExtensions(extensions map[string]int) {
	peer.extensions = extensions
}

func newPeer(ip string, port string, torrent *Torrent) *Peer {
	var peer Peer

	peer.ip = ip
	peer.port = port
	peer.torrent = torrent
	peer.choked = true
	peer.logger = log.New(peer.torrent.logFile, "[Peer] "+peer.ip+": ", log.Ltime|log.Lshortfile)
	peer.logger.SetOutput(io.Discard)
	peer.status = Unknown // implied by default
	peer.pieceQueue = newPieceQueue(0, false)

	// rand.Seed(time.Now().UnixNano())
	return &peer
}

func (peer *Peer) String() string {
	return peer.ip + " " + strconv.Itoa(peer.status)
}

func (peer *Peer) run(doneCh chan *Peer) {
	defer func() { doneCh <- peer }()

	// if we are reconnecting to this peer we need to reset some variables
	peer.status = Alive
	peer.choked = true
	peer.requests = 0
	peer.logger.Printf(" + %v", peer)

	err := peer.connect()
	if err != nil {
		//		peer.logger.Println("Connection error: " + err.Error())
		peer.status = Bad
		return
	}

	err = peer.performHandshake()
	if err != nil {
		//		peer.logger.Println("Handshake error: " + err.Error())
		peer.status = Bad
		return
	}

	err = peer.getBitfield()
	if err != nil {
		//		peer.logger.Println("Bitfield error: " + err.Error())
		peer.status = Bad
		return
	}

	// Drop this peer if we don't have metadata yet and they aren't equipped to send it
	if !peer.supportsMetadataRequests() && !peer.torrent.hasMetadata {
		return
	}

	var wg sync.WaitGroup

	wg.Add(2)
	go peer.pr.run(&wg)
	go peer.pw.run(&wg)

	if peer.torrent.hasMetadata {
		peer.sendInterested()
	}
	wg.Wait()
}

func (peer *Peer) supportsMetadataRequests() bool {
	if !peer.usesExtended {
		return false
	}
	_, ok := peer.extensions["ut_metadata"]
	return ok
}

// TODO: ensure read/write are closed
func (peer *Peer) disconnect() {
	for i := 0; i < len(peer.pieceQueue.pieces); i++ {
		p, _ := peer.pieceQueue.pop()
		peer.torrent.pieceQueue.push(p)
	}
	if peer.conn != nil { // need to look into this, also keeping it open
		err := peer.conn.Close()
		if err != nil {
			// Doesn't really matter if the connection is already closed
			return
		}
	}
}

// Connect to peer via TCP and create a peer_reader over connection
func (peer *Peer) connect() error {
	timeout := time.Second * 10
	conn, err := net.DialTimeout("tcp", peer.ip+":"+peer.port, timeout)

	if err != nil {
		return err
	}

	peer.conn = conn
	peer.pr = newPeerReader(peer)
	peer.pw = newPeerWriter(peer)
	return nil
}

// Sends an INTERESTED message about the given torrent so that we can be unchoked
func (peer *Peer) sendInterested() {
	if peer.pw == nil {
		return
	}
	peer.pw.write(Message{1, Interested, nil})
}

func (peer *Peer) performHandshake() error {
	if peer.conn == nil {
		return errors.New("peer's connection is nil")
	}

	outgoingHandshake := getHandshakeMessage(peer.torrent)
	_, err := peer.conn.Write(outgoingHandshake)
	if err != nil {
		return errors.New("unable to write to peer")
	}

	// Read from peer
	pstrlenBuf := make([]byte, 1)
	_, err = peer.conn.Read(pstrlenBuf)
	if err != nil {
		return errors.New("could not read from peer")
	}

	pstrlen := int(pstrlenBuf[0])

	buf := make([]byte, 48+pstrlen)
	_, err = peer.conn.Read(buf)
	if err != nil {
		return errors.New("could not read from peer")
	}

	// TODO: confirm that peerid is the same as supplied on tracker

	// if the peer utilizes extended messages (most likely), we next need to send an extended handshake, mostly just for getting metadata
	if buf[24]&0x10 == 16 {
		peer.usesExtended = true
		outgoingExtendedHandshake := getExtendedHandshakeMessage()

		bytesWritten, err := peer.conn.Write(outgoingExtendedHandshake)
		if err != nil || bytesWritten < len(outgoingExtendedHandshake) {
			return errors.New("unable to write to peer in extended handshake")
		}

		lengthPrefixBuf := make([]byte, 4)
		_, err = peer.conn.Read(lengthPrefixBuf)
		if err != nil {
			return err
		}

		lengthPrefix := binary.BigEndian.Uint32(lengthPrefixBuf[0:])
		buf = make([]byte, int(lengthPrefix))

		_, err = io.ReadFull(peer.conn, buf)
		if err != nil {
			return err
		}

		result, err := decodeHandshake(buf[2:])
		if err != nil {
			peer.status = Bad
			return err
		}

		peer.setExtensions(result.Extensions)
		peer.maxRequests = result.Requests

		if result.MetadataSize != 0 && peer.torrent.metadataSize == 0 { // make sure they attached metadata size, also no reason to overwrite if we already set
			peer.torrent.metadataSize = result.MetadataSize
			peer.torrent.metadataRaw = make([]byte, result.MetadataSize)
			peer.torrent.metadataPieces = make([]byte, (peer.torrent.numMetadataPieces()+7)/8)
		}
	}
	return nil
}

// Read the bitfield, should be called directly after a handshake
func (peer *Peer) getBitfield() error {
	lengthPrefixBuf := make([]byte, 4)
	messageIDBuf := make([]byte, 1)

	_, err := peer.conn.Read(lengthPrefixBuf)
	if err != nil {
		return err
	}

	_, err = peer.conn.Read(messageIDBuf)
	if err != nil {
		return err
	}

	lengthPrefix := int(binary.BigEndian.Uint32(lengthPrefixBuf[0:]))
	messageID := int(messageIDBuf[0])

	if messageID != Bitfield {
		return errors.New("got unexpected message from peer, expecting BITFIELD")
	}

	bitfieldBuf := make([]byte, lengthPrefix-1)
	_, err = io.ReadFull(peer.conn, bitfieldBuf)
	if err != nil {
		return err
	}
	peer.bitfield = bitfieldBuf
	peer.logger.Println(peer.bitfield)
	return nil
}

// Send a request block message to this peer asking for a random non-downloaded block
func (peer *Peer) requestPieces() error {
	// Make sure it's a good idea to request blocks
	if !peer.torrent.hasMetadata {
		return errors.New("block requested before metadata was downloaded")
	}

	peer.torrent.checkDownloadStatus()
	if peer.torrent.isDownloaded {
		panic("torrent is downloaded, no need to queue more blocks")
	}

	peer.updatePieceQueue()

	// Request as many pieces as we can without exceeding the peer's maxRequests
	for {
		peer.requestsMX.Lock()
		if peer.requests+peer.torrent.getNumBlocksInPiece() > peer.maxRequests {
			peer.requestsMX.Unlock()
			break
		}
		peer.requestsMX.Unlock()

		// Get a new random piece

		piece, err := peer.torrent.pieceQueue.pop()
		//		offset := rand.Intn(len(peer.torrent.pieces[piece].blocks))

		for {
			if err != nil {
				return err
			}

			if peer.hasPiece(piece) {
				peer.pieceQueue.push(piece)
				break
			}

			peer.torrent.pieceQueue.push(piece)
			piece, err = peer.torrent.pieceQueue.pop()
			//piece = rand.Intn(len(peer.torrent.pieces))
			//offset = rand.Intn(len(peer.torrent.pieces[piece].blocks))
		}

		peer.logger.Printf("\nRequesting block %d", piece)

		for offset := 0; offset < len(peer.torrent.pieces[piece].blocks); offset++ {
			// Make sure we need this piece, otherwise skip it
			if peer.torrent.hasBlock(piece, offset*BlockLen) {
				continue
			}

			// Create message
			payload := make([]byte, 12)
			binary.BigEndian.PutUint32(payload[0:], uint32(piece))
			binary.BigEndian.PutUint32(payload[4:], uint32(offset*BlockLen))

			// if this is the last blocks, we need to request the correct len
			if offset == len(peer.torrent.pieces[piece].blocks)-1 {
				// if last piece
				if piece == len(peer.torrent.pieces)-1 {
					binary.BigEndian.PutUint32(payload[8:], uint32(peer.torrent.metadata.Length%BlockLen))
				} else {
					// if last block in piece
					binary.BigEndian.PutUint32(payload[8:], uint32(peer.torrent.metadata.PieceLen%BlockLen))
				}
			} else {
				binary.BigEndian.PutUint32(payload[8:], uint32(BlockLen))
			}

			peer.pw.write(Message{13, Request, payload})

			peer.requestsMX.Lock()
			peer.requests++
			peer.requestsMX.Unlock()
		}
	}
	return nil
}

// trim any pieces from the queue that are either verified or have been readded to the torrent's queue due to a failed hash check
func (peer *Peer) updatePieceQueue() {
	peer.pieceQueue.piecesMX.Lock()
	defer peer.pieceQueue.piecesMX.Unlock()

	var removed int

	for i := 0; i < len(peer.pieceQueue.pieces); i++ {
		if peer.torrent.hasPiece(peer.pieceQueue.pieces[i-removed]) || peer.torrent.pieceQueue.contains(peer.pieceQueue.pieces[i-removed]) {
			delete(peer.pieceQueue.pieceMap, peer.pieceQueue.pieces[i-removed])
			peer.pieceQueue.pieces = append(peer.pieceQueue.pieces[0:i-removed], peer.pieceQueue.pieces[i-removed+1:]...)
			removed++
		}
	}
	return
}

// Return true if peer's bitfield indicates that they have the inputed piece
func (peer *Peer) hasPiece(pieceNum int) bool {
	return utils.BitIsSet(peer.bitfield, pieceNum)
	//	return (peer.bitfield[(pieceNum/int(8))]>>(7-(pieceNum%8)))&1 == 1
}
