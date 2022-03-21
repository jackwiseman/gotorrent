package main

import (
	"net"
	"time"
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
func (pw *Peer_Writer) send_metadata_request(piece_num int) {
	payload := encode_metadata_request(piece_num)
	// marshall will ensure the length_prefix is set, we don't need to specify it here
	pw.write_extended(Extended_Message{0, 20, uint8(pw.peer.extensions["ut_metadata"]), []byte(payload)})
	fmt.Println("Requesting metadata...")
}

// use a time.Ticker to repeatedly request a metadata piece until we have the full file
// todo make sure mutexes are used when checking pieces
func (pw *Peer_Writer) metadata_request_scheduler() {
	ticker := time.NewTicker(10 * time.Second)
	for _ = range(ticker.C) {
		if pw.peer.torrent.has_all_metadata() {
			ticker.Stop()
			
			// for debug -- just trying to collect metadata for the time being
			pw.stop()
			return
		}
		pw.send_metadata_request(pw.peer.torrent.get_rand_metadata_piece())
	}
}

// will need to also include keepalive messages
func (pw *Peer_Writer) run(wg *sync.WaitGroup) {
	defer wg.Done()

	if !pw.peer.torrent.has_all_metadata() {
		// schedule piece requests every 5 seconds until it's collected, should set a global timeout
		go pw.metadata_request_scheduler()
	}

	for {
		msg := <- pw.message_ch

		if int(msg[4]) == STOP { // bad typecast comparison
			fmt.Println("Peer_Writer received STOP, exiting...")
			return
		}

		_, err := pw.conn.Write(msg)
		if err != nil {
			// closed connection, need to disable reader
			return
		}
		//fmt.Printf("\nWrote %d bytes\n", b)
	}
}

// func keep_alive_scheduler()
