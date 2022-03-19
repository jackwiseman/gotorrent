package main

import (
	"net"
	"bufio"
)

type Peer_Writer struct {
	conn net.Conn
	writer *bufio.Writer
	buf_size int

	message_ch chan []byte // probably need to initialize this?
}

func new_peer_writer(peer Peer) (*Peer_Writer) {
	var pw Peer_Writer
	pw.conn = peer.conn
	pw.buf_size = 6 // len + id + (extension id if id == 20)
	pw.writer = bufio.NewWriterSize(pw.conn, pw.buf_size)
	return &pw
}

func (pw *Peer_Writer) write(message Message) {
	pw.message_ch <- message.marshall()
}

func (pw *Peer_Writer) stop() {
	pw.write(Message{1, STOP})
}

// will need to also include keepalive messages
func (pw *Peer_Writer) run() {
	for {
		msg := <- pw.message_ch

		if int(msg[5]) == 99 { // bad typecast comparison
			return
		}

		_, err := pw.conn.Write(msg)
		if err != nil {
			panic(err)
		}
	}
}

// func keep_alive_scheduler()
