package main

import (
	"net"
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
	for {
		fmt.Println("Waiting for message")
		length_prefix_buf := make([]byte, 4)
		_, err := pr.conn.Read(length_prefix_buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Peer closed connection")
				return
			}
			panic(err)
		}

		length_prefix := int(binary.BigEndian.Uint32(length_prefix_buf))
		fmt.Printf("Received length_prefix of %d\n", length_prefix)
		
		if length_prefix == 0 { // keepalive
			fmt.Println("Received keepalive")
			fmt.Println("Moving to next peer for now")
			return
//			continue
		}

		message_id_buf := make([]byte, 1)
		_, err = pr.conn.Read(message_id_buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Peer closed connection")
				return
			}
			panic(err)
		}

		message_id := int(message_id_buf[0])

		fmt.Printf("\nLength: %d, Message_id: %d\n", length_prefix, message_id)
		
		switch int(message_id) {
			case CHOKE:
				fmt.Println("Received choke")
			case UNCHOKE:
				fmt.Println("Received unchoke")
			case INTERESTED:
				fmt.Println("Received interrested")
			case NOT_INTERESTED:
				fmt.Println("Received not interrested")
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
				}

				payload_buf := make([]byte, length_prefix - 2)
				_, err = pr.conn.Read(payload_buf)
				if err != nil {
					panic(err)
				}

				bencode_end := bytes.Index(payload_buf, []byte("ee")) + 2
				bencode := payload_buf[0:bencode_end]
				metadata_piece := payload_buf[bencode_end:]
				fmt.Println(string(bencode))
				fmt.Println(string(metadata_piece))
				
				

				
			//	pr.peer.torrent.metadata

				return
			default:
				fmt.Println("Received bad message_id")
		}

	}
}
