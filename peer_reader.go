package main

import (
	"net"
	"log"
	"time"
	"strconv"
	"errors"
	"sync"
	"encoding/binary"
	"bytes"
)

// this structure likely should be changed, kind of redundant
type Peer_Reader struct {
	peer *Peer
	conn net.Conn
	logger *log.Logger

//	reader bufio.Reader
//	buf_size int

}

func new_peer_reader(peer *Peer) (*Peer_Reader) {
	var pr Peer_Reader
	pr.conn = peer.conn
	pr.peer = peer
	pr.logger = log.New(peer.torrent.log_file, "[Peer Reader] " + pr.peer.ip + ": ", log.Ltime | log.Lshortfile)
	return &pr
}

// will need to also include keepalive messages
func (pr *Peer_Reader) run(wg *sync.WaitGroup) {
	defer pr.peer.pw.stop()
	defer wg.Done()

	for {
		// disconnect if we don't receive a KEEP ALIVE (or any message) for 2 minutes
		pr.conn.SetReadDeadline(time.Now().Add(time.Minute * time.Duration(2)))

		length_prefix_buf := make([]byte, 4)
		_, err := pr.conn.Read(length_prefix_buf)
		if err != nil {
			pr.logger.Println(err)
			return
		}

		length_prefix := int(binary.BigEndian.Uint32(length_prefix_buf))
		
		if length_prefix == 0 {
			pr.logger.Println("Received KEEP ALIVE")
			continue
		}

		message_id_buf := make([]byte, 1)
		_, err = pr.conn.Read(message_id_buf)
		if err != nil {
			pr.logger.Println(err)
			return
		}

		message_id := int(message_id_buf[0])

		pr.logger.Printf("Message received - Length: %d, Message_id: %d\n", length_prefix, message_id)
		
		switch int(message_id) {
			// no payload
			case CHOKE:
				pr.logger.Println("Received CHOKE")
				continue
			case UNCHOKE:
				pr.logger.Println("Received UNCHOKE")
				pr.peer.choked = false
				continue
			case INTERESTED:
				pr.logger.Println("Received INTERRESTED")
				continue
			case NOT_INTERESTED:
				pr.logger.Println("Received NOT INTERRESTED")
				continue
			case HAVE:
				pr.logger.Println("Received HAVE")
			case BITFIELD:
				pr.logger.Println("Received BITFIELD")
				bitfield_buf := make([]byte, length_prefix - 1)
				_, err = pr.conn.Read(bitfield_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}
				pr.peer.bitfield = bitfield_buf
			case REQUEST:
				pr.logger.Println("Received REQUEST")
			case PIECE:
				pr.logger.Println("Received PIECE")
			case CANCEL:
				pr.logger.Println("Received CANCEL")
			case PORT:
				pr.logger.Println("Received PORT")
			case EXTENDED:
				pr.logger.Println("Received EXTENDED")
				extended_id_buf := make([]byte, 1)
				_, err = pr.conn.Read(extended_id_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}

				if extended_id_buf[0] != uint8(0) {
					pr.logger.Println("Received unsupported extended message")
					continue
				}

				payload_buf := make([]byte, length_prefix - 2)
				_, err = pr.conn.Read(payload_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}

				bencode_end := bytes.Index(payload_buf, []byte("ee")) + 2
				bencode := payload_buf[0:bencode_end]
				
				response := decode_metadata_request(bencode)
				
				if response.Msg_type == 2 { // reject
					pr.logger.Println("Peer does not have requested metadata piece")
					continue
				}

				metadata_piece := payload_buf[bencode_end:]
				//fmt.Println(string(bencode))
				//fmt.Println(string(metadata_piece))
	
				// ensure metadata is built once and only once
				pr.peer.torrent.metadata_mx.Lock()
				
				before_append := pr.peer.torrent.has_all_metadata()
			
				err = pr.peer.torrent.set_metadata_piece(response.Piece, metadata_piece)
				if err != nil {
					pr.logger.Println(errors.New("Error in setting metadata piece\n - len_prefix: " + strconv.Itoa(length_prefix) + "\n - message_id: " + strconv.Itoa(message_id) + "\n - bencode info: " + string(bencode) + "\n - raw length " + strconv.Itoa(len(pr.peer.torrent.metadata_raw))))
				}
				
				if before_append != pr.peer.torrent.has_all_metadata() { // true iff we inserted the last piece
					err = pr.peer.torrent.build_metadata_file(pr.peer.ip + ".torrent")
					if err != nil {
						pr.logger.Println(err)
						return
					}
				}

				pr.peer.torrent.metadata_mx.Unlock()

			default:
				pr.logger.Println("Received bad message_id")
		}

	}
}
