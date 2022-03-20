package main

import (
	"net"
	"sync"
	"bufio"
	"fmt"
)

type Peer_Writer struct {
	conn net.Conn
	writer *bufio.Writer
	buf_size int
	peer *Peer

	message_ch chan []byte // probably need to initialize this?
}

func new_peer_writer(peer *Peer) (*Peer_Writer) {
	var pw Peer_Writer
	pw.peer = peer
	pw.conn = peer.conn
	pw.buf_size = 6 // len + id + (extension id if id == 20)
	pw.writer = bufio.NewWriterSize(pw.conn, pw.buf_size)
	pw.message_ch = make(chan []byte)
	return &pw
}

func (pw *Peer_Writer) write(message Message) {
	pw.message_ch <- message.marshall()
}

func (pw *Peer_Writer) write_extended(message Extended_Message) {
	pw.message_ch <- message.marshall()
}

func (pw *Peer_Writer) stop() {
	pw.write(Message{1, STOP})
}

// request specified metadata piece
func (pw *Peer_Writer) send_metadata_request(piece_num int, wg *sync.WaitGroup) {
	defer wg.Done()
	payload := encode_metadata_request(piece_num)
	// marshall will ensure the length_prefix is set, we don't need to specify it here
	pw.write_extended(Extended_Message{0, 20, uint8(pw.peer.extensions["ut_metadata"]), []byte(payload)})
}

// will need to also include keepalive messages
func (pw *Peer_Writer) run(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		msg := <- pw.message_ch

		if int(msg[4]) == 99 { // bad typecast comparison
			return
		}

		b, err := pw.conn.Write(msg)
		if err != nil {
			panic(err)
		}
		fmt.Printf("\nWrote %d bytes\n", b)
		fmt.Println(msg)
//		return
		if int(msg[4]) == 20 {
			fmt.Println("Returning peer_writer")
			return
		}
	}
}

// func keep_alive_scheduler()
