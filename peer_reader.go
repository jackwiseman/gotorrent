package main

import (
	"net"
	"log"
	"time"
	"sync"
	"encoding/binary"
	"bytes"
)

// this structure likely should be changed, kind of redundant
type Peer_Reader struct {
	peer *Peer
	conn net.Conn
	logger *log.Logger
}

func new_peer_reader(peer *Peer) (*Peer_Reader) {
	var pr Peer_Reader
	pr.conn = peer.conn
	pr.peer = peer
	pr.logger = log.New(peer.torrent.log_file, "[Peer Reader] " + pr.peer.ip + ": ", log.Ltime | log.Lshortfile)
	return &pr
}

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
				pr.peer.choked = true
				continue
			case UNCHOKE:
				pr.logger.Println("Received UNCHOKE")
				pr.peer.choked = false
				continue
			case INTERESTED:
				pr.logger.Println("Received INTERESTED")
				continue
			case NOT_INTERESTED:
				pr.logger.Println("Received NOT INTERESTED")
				continue
			case HAVE:
				pr.logger.Println("Received HAVE")
				piece_index_buf := make([]byte, 4)
				_, err = pr.conn.Read(piece_index_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}
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

				// index, begin, length
				payload_buf := make([]byte, 12)

				_, err = pr.conn.Read(payload_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}
				
			case PIECE:
				pr.logger.Println("Received PIECE")

				if length_prefix > BLOCK_LEN + 9 {
					pr.logger.Println("PIECE MESSAGE WAS TOO LONG")
					continue
				}
				index_buf := make([]byte, 4)
				begin_buf := make([]byte, 4)

				_, err = pr.conn.Read(index_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}

				_, err = pr.conn.Read(begin_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}

				block_buf := make([]byte, length_prefix - 9)
				total_read := 0
				for total_read < length_prefix - 9 { // sometimes the block doesn't get fully read in one call here, TODO: investigate this behavior globally
					temp_buf := make([]byte, len(block_buf) - total_read)
					n, err := pr.conn.Read(temp_buf)
					if err != nil {
						pr.logger.Println(err)
						return
					}
					block_buf_remainder := block_buf[total_read + n:]
					block_buf = append(block_buf[0:total_read], temp_buf[:n]...)
					block_buf = append(block_buf, block_buf_remainder...)
					pr.logger.Println(n)
					total_read += n
				}

				index := int(binary.BigEndian.Uint32(index_buf))
				begin := int(binary.BigEndian.Uint32(begin_buf))

				pr.logger.Printf("Index: %d, Begin: %d", index, begin)

				pr.peer.torrent.set_block(index, begin, block_buf)

				go pr.peer.request_new_block()
			case CANCEL:
				pr.logger.Println("Received CANCEL")

				// index, begin, length
				payload_buf := make([]byte, 12)

				_, err = pr.conn.Read(payload_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}
			case PORT:
				pr.logger.Println("Received PORT")
				listen_port_buf := make([]byte, 2)
				_, err = pr.conn.Read(listen_port_buf)
				if err != nil {
					pr.logger.Println(err)
					return
				}
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
				total_read := 0
				for total_read < length_prefix - 2 { 
					temp_buf := make([]byte, len(payload_buf) - total_read)
					n, err := pr.conn.Read(temp_buf)
					if err != nil {
						pr.logger.Println(err)
						return
					}
					payload_buf_remainder := payload_buf[total_read + n:]
					payload_buf = append(payload_buf[0:total_read], temp_buf[:n]...)
					payload_buf = append(payload_buf, payload_buf_remainder...)
					pr.logger.Println(n)
					total_read += n
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
				}
				
				if before_append != pr.peer.torrent.has_all_metadata() { // true iff we inserted the last piece
					err = pr.peer.torrent.build_metadata_file()
					if err != nil {
						pr.logger.Println(err)
						return
					}
					err = pr.peer.torrent.parse_metadata_file()
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
