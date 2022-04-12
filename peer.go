package main

import (
	"encoding/binary"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
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
	peer.logger.SetOutput(ioutil.Discard)
	peer.status = Unknown // implied by default

	return &peer
}

func (peer *Peer) String() string {
	return peer.ip + " " + strconv.Itoa(peer.status)
}

func (peer *Peer) run(doneCh chan *Peer) {
	defer func() { doneCh <- peer }()

	peer.logger.Printf(" + %s", peer.String())

	peer.status = Alive

	err := peer.connect()
	if err != nil {
		peer.logger.Println("Connection error: " + err.Error())
		peer.status = Bad
		return
	}

	err = peer.performHandshake()
	if err != nil {
		peer.logger.Println("Handshake error: " + err.Error())
		peer.status = Bad
		return
	}

	err = peer.getBitfield()
	if err != nil {
		peer.logger.Println("Bitfield error: " + err.Error())
		peer.status = Bad
		return
	}

	// Drop this peer if we don't have metadata yet and they aren't equipped to send it
	if !peer.supportsMetadataRequests() && !peer.torrent.hasAllMetadata() {
		peer.disconnect()
		return
	}

	var wg sync.WaitGroup

	wg.Add(2)
	go peer.pr.run(&wg)
	go peer.pw.run(&wg)
	wg.Wait()

	peer.disconnect()
}

func (peer *Peer) supportsMetadataRequests() bool {
	if !peer.usesExtended {
		return false
	}
	_, ok := peer.extensions["ut_metadata"]
	return ok
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
/*func (peer *Peer) sendInterested() {
	if peer.pw == nil {
		return
	}
	peer.pw.write(Message{1, Interested, nil})
}*/

// send a request message to peer asking for specified block
func (peer *Peer) requestBlock(pieceNum int, offset int) {
	peer.logger.Printf("\nRequsting block (%d, %d)", pieceNum, offset)
	if peer.pw == nil {
		return
	}

	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:], uint32(pieceNum))
	binary.BigEndian.PutUint32(payload[4:], uint32(offset*BlockLen))

	// if this is the last blocks, we need to request the correct len
	if (pieceNum*peer.torrent.getNumBlocksInPiece())+offset+1 == peer.torrent.getNumBlocks() {
		binary.BigEndian.PutUint32(payload[8:], uint32(peer.torrent.metadata.Length%BlockLen))
	} else {
		binary.BigEndian.PutUint32(payload[8:], uint32(BlockLen))
	}

	peer.pw.write(Message{13, Request, payload})
}

// TODO: ensure read/write are closed
func (peer *Peer) disconnect() {
	if peer.conn != nil { // need to look into this, also keeping it open
		peer.conn.Close()
	}
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
		_, err = peer.conn.Read(buf)
		if err != nil {
			return err
		}

		result := decodeHandshake(buf[2:])
		peer.setExtensions(result.M)

		if result.MetadataSize != 0 && peer.torrent.metadataSize == 0 { // make sure they attached metadata size, also no reason to overwrite if we already set
			peer.torrent.metadataSize = result.MetadataSize
			peer.torrent.metadataRaw = make([]byte, result.MetadataSize)
			peer.torrent.metadataPieces = make([]byte, (peer.torrent.numMetadataPieces()+7)/8)
			peer.logger.Println(peer.torrent.metadataPieces)
			peer.logger.Println(peer.torrent.numMetadataPieces())
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
		peer.logger.Printf("Unexpected BITFIELD: length: %d, id: %d", lengthPrefix, messageID)
		return errors.New("got unexpected message from peer, expecting BITFIELD")
	}

	bitfieldBuf := make([]byte, lengthPrefix-1)
	totalRead := 0
	for totalRead < lengthPrefix-1 {
		tempBuf := make([]byte, len(bitfieldBuf)-totalRead)
		n, err := peer.conn.Read(tempBuf)
		if err != nil {
			return err
		}

		bitfieldBufRemainder := bitfieldBuf[totalRead+n:]
		bitfieldBuf = append(bitfieldBuf[0:totalRead], tempBuf[:n]...)
		bitfieldBuf = append(bitfieldBuf, bitfieldBufRemainder...)
		peer.logger.Println(n)
		totalRead += n
	}

	peer.bitfield = bitfieldBuf
	peer.logger.Println(peer.bitfield)
	return nil
}

// Request queue_size blocks from peer, so that time is not lost between each received block and each new requested one
/*func (peer *Peer) queueBlocks(queueSize int) {
	peer.sendInterested()

	for {
		if !peer.choked {
			break
		}
	}

	for i := 0; i < queueSize; i++ {
		piece, offset := peer.getNewBlock()
		peer.logger.Printf("- (%d, %d)", piece, offset)
		if piece == -1 {
			continue
		}
		peer.requestBlock(piece, offset)
	}
}*/

// Send a request block message to this peer asking for a random non-downloaded block
func (peer *Peer) requestNewBlock() {
	if !peer.torrent.hasAllMetadata() {
		return
	}
	go peer.torrent.checkDownloadStatus()
	piece, offset := peer.getNewBlock()
	if piece == -1 {
		return
	}

	peer.requestBlock(piece, offset)
}

// Return a random piece + offset pair corresponding to a non downloaded block
// or -1, -1 in the case of all blocks already downloaded
func (peer *Peer) getNewBlock() (int, int) {
	rand.Seed(time.Now().UnixNano())
	if peer.torrent.hasAllData() {
		peer.logger.Println("Option 1")
		return -1, -1
	}

	for {
		testPiece := rand.Intn(len(peer.torrent.pieces))
		testOffset := rand.Intn(len(peer.torrent.pieces[testPiece].blocks))

		if peer.hasPiece(testPiece) /*&& !peer.made_request(test_piece, test_offset)*/ && !peer.torrent.hasBlock(testPiece, testOffset*BlockLen) {
			return testPiece, testOffset
		}
	}
}

// Return true if peer's bitfield indicates that they have the inputed piece
func (peer *Peer) hasPiece(pieceNum int) bool {
	return (peer.bitfield[(pieceNum/int(8))]>>(7-(pieceNum%8)))&1 == 1
}
