package main

import (
	"net"
	"errors"
	"strconv"
	"sync"
	"io"
//	"time"
//	"bufio"
	"fmt"
	"encoding/binary"
	"bytes"
)

// this structure likely should be changed, kind of redundant
type Peer_Reader struct {
	peer *Peer
	conn net.Conn

//	reader bufio.Reader
//	buf_size int

}

func new_peer_reader(peer *Peer) (*Peer_Reader) {
	var pr Peer_Reader
	pr.conn = peer.conn
	pr.peer = peer
	return &pr
}

// will need to also include keepalive messages
func (pr *Peer_Reader) run(wg *sync.WaitGroup) {
	defer wg.Done()
	defer pr.peer.pw.stop()

	for {
//		fmt.Println("Waiting for message")
		length_prefix_buf := make([]byte, 4)
		_, err := pr.conn.Read(length_prefix_buf)
		if err != nil {
			if err == io.EOF {
//				fmt.Println("Peer closed connection")
				return
			}
			panic(err)
		}

		length_prefix := int(binary.BigEndian.Uint32(length_prefix_buf))
//		fmt.Printf("Received length_prefix of %d\n", length_prefix)
		
		if length_prefix == 0 { // keepalive
//			fmt.Println("Received keepalive")
		//	pr.peer.pw.stop()
//			fmt.Println("Moving to next peer for now")
			return
//			continue
		}

		message_id_buf := make([]byte, 1)
		_, err = pr.conn.Read(message_id_buf)
		if err != nil {
			if err == io.EOF {
//				fmt.Println("Peer closed connection")
		//		pr.peer.pw.stop()
				return
			}
			panic(err)
		}

		message_id := int(message_id_buf[0])

//		fmt.Printf("\nLength: %d, Message_id: %d\n", length_prefix, message_id)
		
		switch int(message_id) {
			// no payload
			case CHOKE:
//				fmt.Println("Received choke")
				continue
			case UNCHOKE:
//				fmt.Println("Received unchoke")
				continue
			case INTERESTED:
//				fmt.Println("Received interrested")
				continue
			case NOT_INTERESTED:
//				fmt.Println("Received not interrested")
				continue
			case HAVE:
				fmt.Println("Received have")
			case BITFIELD:
				fmt.Println("Received bitfield")
				bitfield_buf := make([]byte, length_prefix - 1)
				_, err = pr.conn.Read(bitfield_buf)
				if err != nil {
					panic(err)
				}
				pr.peer.bitfield = bitfield_buf
			case REQUEST:
				fmt.Println("Received request")
			case PIECE:
				fmt.Println("Received piece")
			case CANCEL:
				fmt.Println("Received cancel")
			case PORT:
				fmt.Println("Received port")
			case EXTENDED:
				fmt.Println("Received extended!!")
				extended_id_buf := make([]byte, 1)
				_, err = pr.conn.Read(extended_id_buf)
				if err != nil {
					panic(err)
				}

				if extended_id_buf[0] != uint8(0) {
					fmt.Println("Received unsupported extended message")
					continue
				}

				payload_buf := make([]byte, length_prefix - 2)
				_, err = pr.conn.Read(payload_buf)
				if err != nil {
					panic(err)
				}

				bencode_end := bytes.Index(payload_buf, []byte("ee")) + 2
				bencode := payload_buf[0:bencode_end]
				
				response := decode_metadata_request(bencode)
				
				if response.Msg_type == 2 { // reject
					fmt.Println("Peer does not have piece")
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
					panic(errors.New("Error in setting metadata piece\n - len_prefix: " + strconv.Itoa(length_prefix) + "\n - message_id: " + strconv.Itoa(message_id) + "\n - bencode info: " + string(bencode) + "\n - raw length " + strconv.Itoa(len(pr.peer.torrent.metadata_raw))))
				}
//				fmt.Println(pr.peer.torrent.metadata_raw)
				
				if before_append != pr.peer.torrent.has_all_metadata() { // true iff we inserted the last piece
					err = pr.peer.torrent.build_metadata_file(pr.peer.ip + ".torrent")
					if err != nil {
//						panic(fmt.Printf("Error dump: %d, %d", length_prefix, message_id)
						panic(errors.New("Error: len_prefix:" + strconv.Itoa(length_prefix) + ", message_id:" + strconv.Itoa(message_id)))
				//		panic(err)
					}
				}

				pr.peer.torrent.metadata_mx.Unlock()

			default:
				fmt.Println("Received bad message_id")
		}

	}
}
