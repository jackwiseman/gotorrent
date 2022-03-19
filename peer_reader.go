package main

import (
	"net"
	"time"
	"bufio"
)

type Peer_Reader struct {
	conn net.Conn
	reader *bufio.Reader
	buf_size int

	message_chan chan []byte
}

func new_peer_reader(peer Peer) (*Peer_Reader) {
	var pr Peer_Writer
	pr.conn = peer.conn
	pr.buf_size = 6 // len + id + (extension id if id == 20)
	pr.writer = bufio.NewReaderSize(pr.conn, pr.buf_size)
	return &pr
}

func (pr *Peer_Reader) write(message Message) {
	pr.message_ch <- message.marshall()
}

func (pr *Peer_Reader) stop() {
	pr.write(Message{1, STOP})
}

// will need to also include keepalive messages
func (pr *Peer_Reader) run() {
	timeout := 2 * time.Minute
	pr.conn.SetReadTimeout(timeout)
	for {
		length_prefix := make([]byte, 4)
		err := pr.conn.Read()
		if err != nil {
			panic(err)
		}
		
		if int(length_prefix) == 0 { // keepalive
			continue
		}

		msg := <- pr.message_ch

		if int(msg[5]) == 99 { // bad typecast comparison
			return
		}

		_, err := pr.conn.Write(msg)
		if err != nil {
			panic(err)
		}
	}
}

/
