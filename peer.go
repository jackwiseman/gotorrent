package main

import (
	"encoding/binary"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Peer struct {
	ip            string
	port          string
	conn          net.Conn
	uses_extended bool // false by default
	extensions    map[string]int
	choked        bool // whether we are choked by this peer or not, will likely need a name change upon seed support
	bitfield      []byte

	torrent *Torrent // associated torrent

	// wrapped io.Reader/io.Writer interfaces
	pw *Peer_Writer
	pr *Peer_Reader

	requests []byte
	// responses []byte

	logger *log.Logger
}

func (peer *Peer) set_extensions(extensions map[string]int) {
	peer.extensions = extensions
}

func new_peer(ip string, port string, torrent *Torrent) *Peer {
	var peer Peer

	peer.ip = ip
	peer.port = port
	peer.torrent = torrent
	peer.choked = true
	peer.logger = log.New(peer.torrent.log_file, "[Peer] "+peer.ip+": ", log.Ltime|log.Lshortfile)
	peer.logger.SetOutput(ioutil.Discard)

	return &peer
}

func (peer *Peer) run(done_ch chan *Peer) {
	defer peer.disconnect(done_ch) // ideally this mutex shouldn't just be passed around as much

	peer.logger.Printf("Connecting")

	err := peer.connect()
	if err != nil {
		peer.logger.Println("Connection error: " + err.Error())

		done_ch <- peer
		return
	}
	err = peer.perform_handshake()

	if err != nil {
		peer.logger.Println("Handshake error: " + err.Error())
		done_ch <- peer
		return
	}

	err = peer.get_bitfield()
	if err != nil {
		peer.logger.Println("Bitfield error: " + err.Error())
		done_ch <- peer
		return
	}

	var wg sync.WaitGroup

	wg.Add(2)
	go peer.pr.run(&wg)
	go peer.pw.run(&wg)

	wg.Wait()

}

func (peer *Peer) supports_metadata_requests() bool {
	if !peer.uses_extended {
		return false
	}
	_, ok := peer.extensions["ut_metadata"]
	if !ok {
		return false
	}
	return true
}

// Connect to peer via TCP and create a peer_reader over connection
func (peer *Peer) connect() error {
	timeout := time.Second * 10
	conn, err := net.DialTimeout("tcp", peer.ip+":"+peer.port, timeout)

	if err != nil {
		return err
	}

	peer.conn = conn
	peer.pr = new_peer_reader(peer)
	peer.pw = new_peer_writer(peer)
	return nil
}

// Sends an INTERESTED message about the given torrent so that we can be unchoked
func (peer *Peer) send_interested() {
	if peer.pw == nil {
		return
	}
	peer.pw.write(Message{1, INTERESTED, nil})
}

// send a request message to peer asking for specified block
func (peer *Peer) request_block(piece_num int, offset int) {
	peer.logger.Printf("\nRequsting block (%d, %d)", piece_num, offset)
	if peer.pw == nil {
		return
	}

	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:], uint32(piece_num))
	binary.BigEndian.PutUint32(payload[4:], uint32(offset*BLOCK_LEN))

	// if this is the last blocks, we need to request the correct len
	if (piece_num*peer.torrent.get_num_blocks_in_piece())+offset+1 == peer.torrent.get_num_blocks() {
		binary.BigEndian.PutUint32(payload[8:], uint32(peer.torrent.metadata.Length%BLOCK_LEN))
	} else {
		binary.BigEndian.PutUint32(payload[8:], uint32(BLOCK_LEN))
	}

	peer.pw.write(Message{13, REQUEST, payload})
}

// TODO: ensure read/write are closed
func (peer *Peer) disconnect(done_ch chan *Peer) {
	if peer.conn != nil { // need to look into this, also keeping it open
		peer.conn.Close()
	}
	done_ch <- peer
}

func (peer *Peer) perform_handshake() error {
	if peer.conn == nil {
		return errors.New("Error: peer's connection is nil")
	}

	outgoing_handshake := get_handshake_message(peer.torrent)
	_, err := peer.conn.Write(outgoing_handshake)
	if err != nil {
		return errors.New("Error: unable to write to peer")
	}

	// Read from peer
	pstrlen_buf := make([]byte, 1)
	_, err = peer.conn.Read(pstrlen_buf)
	if err != nil {
		return errors.New("Error: could not read from peer")
	}

	pstrlen := int(pstrlen_buf[0])

	buf := make([]byte, 48+pstrlen)
	_, err = peer.conn.Read(buf)
	if err != nil {
		return errors.New("Error: could not read from peer")
	}

	// TODO: confirm that peerid is the same as supplied on tracker

	// if the peer utilizes extended messages (most likely), we next need to send an extended handshake, mostly just for getting metadata
	if buf[24]&0x10 == 16 {
		peer.uses_extended = true
		outgoing_extended_handshake := get_extended_handshake_message()

		bytes_written, err := peer.conn.Write(outgoing_extended_handshake)
		if err != nil || bytes_written < len(outgoing_extended_handshake) {
			return errors.New("Error: unable to write to peer in extended handshake")
		}

		length_prefix_buf := make([]byte, 4)
		_, err = peer.conn.Read(length_prefix_buf)
		if err != nil {
			return err
		}

		length_prefix := binary.BigEndian.Uint32(length_prefix_buf[0:])

		buf = make([]byte, int(length_prefix))
		_, err = peer.conn.Read(buf)
		if err != nil {
			return err
		}

		result := decode_handshake(buf[2:])
		peer.set_extensions(result.M)

		if result.Metadata_size != 0 && peer.torrent.metadata_size == 0 { // make sure they attached metadata size, also no reason to overwrite if we already set
			peer.torrent.metadata_size = result.Metadata_size
			peer.torrent.metadata_raw = make([]byte, result.Metadata_size)
			peer.torrent.metadata_pieces = make([]byte, (peer.torrent.num_metadata_pieces()+7)/8)
			peer.logger.Println(peer.torrent.metadata_pieces)
			peer.logger.Println(peer.torrent.num_metadata_pieces())
		}
	}
	return nil
}

// Read the bitfield, should be called directly after a handshake
func (peer *Peer) get_bitfield() error {
	length_prefix_buf := make([]byte, 4)
	message_id_buf := make([]byte, 1)

	_, err := peer.conn.Read(length_prefix_buf)
	if err != nil {
		return err
	}

	_, err = peer.conn.Read(message_id_buf)
	if err != nil {
		return err
	}

	length_prefix := int(binary.BigEndian.Uint32(length_prefix_buf[0:]))
	message_id := int(message_id_buf[0])

	if message_id != BITFIELD {
		peer.logger.Printf("Unexpected BITFIELD: length: %d, id: %d", length_prefix, message_id)
		return errors.New("Got unexpected message from peer, expecting BITFIELD")
	}

	bitfield_buf := make([]byte, length_prefix-1)
	total_read := 0
	for total_read < length_prefix-1 {
		temp_buf := make([]byte, len(bitfield_buf)-total_read)
		n, err := peer.conn.Read(temp_buf)
		if err != nil {
			return err
		}

		bitfield_buf_remainder := bitfield_buf[total_read+n:]
		bitfield_buf = append(bitfield_buf[0:total_read], temp_buf[:n]...)
		bitfield_buf = append(bitfield_buf, bitfield_buf_remainder...)
		peer.logger.Println(n)
		total_read += n
	}

	peer.bitfield = bitfield_buf
	peer.logger.Println(peer.bitfield)
	return nil
}

// Request queue_size blocks from peer, so that time is not lost between each received block and each new requested one
func (peer *Peer) queue_blocks(queue_size int) {
	peer.send_interested()

	for {
		if !peer.choked {
			break
		}
	}
	peer.requests = make([]byte, ((peer.torrent.get_num_blocks())+8)/8)

	for i := 0; i < queue_size; i++ {
		piece, offset := peer.get_new_block()
		peer.logger.Printf("- (%d, %d)", piece, offset)
		if piece == -1 {
			continue
		}
		index := (piece * peer.torrent.get_num_blocks_in_piece()) + offset
		peer.requests[index/8] = peer.requests[index/8] | (1 << (7 - (index % 8)))
		peer.request_block(piece, offset)
	}
}

// Send a request block message to this peer asking for a random non-downloaded block
func (peer *Peer) request_new_block() {
	if !peer.torrent.has_all_metadata() {
		return
	}
	go peer.torrent.check_download_status()
	piece, offset := peer.get_new_block()
	if piece == -1 {
		return
	}

	peer.request_block(piece, offset)
}

// Return true if we have requested this block from this peer
func (peer *Peer) made_request(piece int, offset int) bool {
	index := (piece * peer.torrent.get_num_blocks_in_piece()) + offset
	return (peer.requests[index/8]>>(7-(index%8)))&1 == 1
}

// Return a random piece + offset pair corresponding to a non downloaded block
// or -1, -1 in the case of all blocks already downloaded
func (peer *Peer) get_new_block() (int, int) {
	rand.Seed(time.Now().UnixNano())
	if peer.torrent.has_all_data() {
		peer.logger.Println("Option 1")
		return -1, -1
	}

	for {
		test_piece := rand.Intn(len(peer.torrent.pieces))
		test_offset := rand.Intn(len(peer.torrent.pieces[test_piece].blocks))

		if peer.has_piece(test_piece) && !peer.made_request(test_piece, test_offset) && !peer.torrent.has_block(test_piece, test_offset*BLOCK_LEN) {
			return test_piece, test_offset
		}
	}
}

// Return true if peer's bitfield indicates that they have the inputed piece
func (peer *Peer) has_piece(piece_num int) bool {
	return (peer.bitfield[(piece_num/int(8))]>>(7-(piece_num%8)))&1 == 1
}
