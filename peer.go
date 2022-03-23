package main

import (
	"sync"
	"net"
	"time"
	"errors"
	"log"
	"encoding/binary"
)

type Peer struct {
	ip string
	port string
	conn net.Conn
	uses_extended bool // false by default
	is_alive bool // assume yes until a handshake fails
	extensions map[string]int
	bitfield []byte

	torrent *Torrent // associated torrent

	// wrapped io.Reader/io.Writer interfaces
	pw *Peer_Writer
	pr *Peer_Reader

	logger *log.Logger

}

func (peer *Peer) set_extensions(extensions map[string]int) {
	peer.extensions = extensions
}

func new_peer(ip string, port string, torrent *Torrent) (*Peer) {
	var peer Peer

	peer.ip = ip
	peer.port = port
	peer.torrent = torrent
	peer.logger = log.New(peer.torrent.log_file, "[Peer] " + peer.ip + ": ", 0/*log.Ltime | log.Lshortfile*/)

	return &peer
}

func (peer *Peer) run() {
	defer peer.disconnect() // ideally this mutex shouldn't just be passed around as much

	peer.logger.Printf("Connecting")

	err := peer.connect()
	if err != nil {
		peer.logger.Println("Connection error: " + err.Error())
		return
	}
	err = peer.perform_handshake()

	if err != nil {
		peer.logger.Println("Handshake error: " + err.Error())
		return
	}

	err = peer.get_bitfield()
	if err != nil {
		peer.logger.Println("Bitfield error: " + err.Error())
		return
	}

	var wg sync.WaitGroup

	wg.Add(2)
	go peer.pr.run(&wg)
	go peer.pw.run(&wg)

	wg.Wait()
}

func (peer *Peer) supports_metadata_requests() (bool) {
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
func (peer *Peer) connect() (error) {
	timeout := time.Second * 10 
	conn, err := net.DialTimeout("tcp", peer.ip + ":" + peer.port, timeout)

	if err != nil {
		peer.is_alive = false
		return err
	} 

	peer.conn = conn
	peer.pr = new_peer_reader(peer)
	peer.pw = new_peer_writer(peer)
	return nil
}

// Sends an INTERESTED message about the given torrent so that we can be unchoked
func (peer *Peer) send_interested() {
	peer.pw.write(Message{1, INTERESTED})
}

// TODO: ensure read/write are closed
func (peer *Peer) disconnect() {
	if peer.conn != nil { // need to look into this, also keeping it open
		peer.conn.Close()
	}
	peer.torrent.conn_handler.remove_connection(peer)
	peer.logger.Println("Disconnected")
}

func (peer *Peer) perform_handshake () (error) {
	if peer.conn == nil {
		return errors.New("Error: peer's connection is nil")
	}

	outgoing_handshake := get_handshake_message(peer.torrent)
	_, err := peer.conn.Write(outgoing_handshake)
	if err != nil {
		peer.is_alive = false
		return errors.New("Error: unable to write to peer")
	}

	// Read from peer
	pstrlen_buf := make([]byte, 1)
	_, err = peer.conn.Read(pstrlen_buf)
	if err != nil {
		peer.is_alive = false
		return errors.New("Error: could not read from peer")
	}

	pstrlen := int(pstrlen_buf[0])

	buf := make([]byte, 48 + pstrlen)
	_, err = peer.conn.Read(buf)
	if err != nil {
		peer.is_alive = false
		return errors.New("Error: could not read from peer")
	}

	// need to confirm that peerid is the same as supplied on tracker

	// if the peer utilizes extended messages (most likely), we next need to send an extended handshake, mostly just for getting metadata
	if buf[24] & 0x10 == 16 {
		peer.uses_extended = true
		//fmt.Println("Peer supports extended messaging, performing extended message handshake")
		outgoing_extended_handshake := get_extended_handshake_message()

		bytes_written, err := peer.conn.Write(outgoing_extended_handshake)
		if err != nil || bytes_written < len(outgoing_extended_handshake) {
			peer.is_alive = false
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
			peer.torrent.metadata_pieces = make([]byte, peer.torrent.num_metadata_pieces())
			//	fmt.Println("Metadata size:")
			//	fmt.Println(torrent.metadata_size)
			//	fmt.Println("Pieces: ")
			//	fmt.Println(int(math.Round(float64(torrent.metadata_size)/float64(16384) + 1.0))) // this is not correct
		}
	} 
	return nil
}

// nominally we'd like to just disconnect from this peer if they don't provide a bitfield, since seeding will probably be implemented later
func (peer *Peer) get_bitfield () (error) {
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
		return errors.New("Got unexpected message from peer, expecting BITFIELD")
	}

	bitfield_buf := make([]byte, length_prefix - 1)
	_, err = peer.conn.Read(bitfield_buf)
	if err != nil {
		return err
	}

	peer.bitfield = bitfield_buf
	return nil
}
