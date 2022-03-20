package main

import (
	"net"
	"io"
//	"time"
//	"bufio"
	"fmt"
	"encoding/binary"
)

type Peer_Reader struct {
	conn net.Conn
//	reader bufio.Reader
//	buf_size int

}

func new_peer_reader(peer *Peer) (*Peer_Reader) {
	var pr Peer_Reader
	pr.conn = peer.conn
//	pr.buf_size = 6 // len + id + (extension id if id == 20)
//	pr.reader = bufio.NewReaderSize(pr.conn, pr.buf_size)
	return &pr
}

// will need to also include keepalive messages
func (pr *Peer_Reader) run() {
//	timeout := 2 * time.Minute
//	pr.conn.SetReadTimeout(timeout)
	for {
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
			continue
		}


//		msg := <- pr.message_ch

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

//		fmt.Printf("\nLength: %d, Message_id: %d\n", length_prefix, message_id)
		
		switch int(message_id) {
			// choke
			case 0:
				if length_prefix != 1 {
					continue
				}
				fmt.Println("Received choke")
			// unchoke
			case 1:
				if length_prefix != 1 {
					continue
				}
				fmt.Println("Received unchoke")
			// interrested
			case 2:
				if length_prefix != 1 {
					continue
				}
				fmt.Println("Received interrested")
			// not interrested
			case 3:
				if length_prefix != 1 {
					continue
				}
				fmt.Println("Received not interrested")
			// have
			case 4:
				fmt.Println("Received have")
			// bitfield
			case 5:
				fmt.Println("Received bitfield")
				// should only accept this FIRST
			// request
			case 6:
				fmt.Println("Received request")
			// piece
			case 7:
				fmt.Println("Received piece")
			// cancel
			case 8:
				fmt.Println("Received cancel")
			// port
			case 9:
				fmt.Println("Received port")
			// extended
			case 20:
				fmt.Println("Received extended")
			default:
				fmt.Println("Received bad message_id")
		}

	}
}
