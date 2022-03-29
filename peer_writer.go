package main

import (
	"net"
	"time"
//	"encoding/binary"
	"sync"
	"bufio"
	"log"
)

const KEEP_ALIVE = uint8(0)

type Peer_Writer struct {
	conn net.Conn
	writer *bufio.Writer
	buf_size int
	peer *Peer
	logger *log.Logger

	message_ch chan []byte // probably need to initialize this?

	// for scheduling purposes 
	keep_alive_ticker *time.Ticker
	metadata_request_ticker *time.Ticker
}

func new_peer_writer(peer *Peer) (*Peer_Writer) {
	var pw Peer_Writer
	pw.peer = peer
	pw.conn = peer.conn
	pw.buf_size = 6 // len + id + (extension id if id == 20)
	pw.writer = bufio.NewWriterSize(pw.conn, pw.buf_size)
	pw.logger = log.New(peer.torrent.log_file, "[Peer Writer] " + pw.peer.ip + ": ", log.Ltime | log.Lshortfile)
	pw.message_ch = make(chan []byte)
	return &pw
}

func (pw *Peer_Writer) write(message Message) {
	// TODO: investigate crash happening here
	if pw.message_ch == nil {
		return
	}
	pw.message_ch <- message.marshall()
}

func (pw *Peer_Writer) write_extended(message Extended_Message) {
	pw.message_ch <- message.marshall()
}

func (pw *Peer_Writer) stop() {
	defer pw.logger.Println("Closed")
	pw.write(Message{1, STOP, nil})
	if pw.keep_alive_ticker != nil {
		pw.keep_alive_ticker.Stop()
	}
	if pw.metadata_request_ticker != nil {
		pw.metadata_request_ticker.Stop()
	}
}

// request specified metadata piece
func (pw *Peer_Writer) send_metadata_request(piece_num int) {
	payload := encode_metadata_request(piece_num)
	// marshall will ensure the length_prefix is set, we don't need to specify it here
	pw.write_extended(Extended_Message{0, 20, uint8(pw.peer.extensions["ut_metadata"]), []byte(payload)})
}

// use a time.Ticker to repeatedly request a metadata piece until we have the full file
// todo make sure mutexes are used when checking pieces
func (pw *Peer_Writer) metadata_request_scheduler() {
	pw.metadata_request_ticker = time.NewTicker(15 * time.Second)
	for _ = range(pw.metadata_request_ticker.C) {
		if pw.peer.torrent.has_all_metadata() {
			pw.metadata_request_ticker.Stop()
			return
		} else {
			if pw.peer.supports_metadata_requests() {
				pw.logger.Println("Requesting metadata")
				pw.send_metadata_request(pw.peer.torrent.get_rand_metadata_piece())
			}
		}
	}
}

func (pw *Peer_Writer) keep_alive_scheduler() {
	pw.keep_alive_ticker = time.NewTicker(1 * time.Minute)
	for _ = range(pw.keep_alive_ticker.C) {
		pw.conn.Write([]byte{0, 0, 0, 0})
	}
	pw.keep_alive_ticker.Stop() 
}

func (pw *Peer_Writer) run(wg *sync.WaitGroup) {
	defer wg.Done()

	go pw.keep_alive_scheduler()

	if !pw.peer.torrent.has_all_metadata() /*&& pw.peer.supports_metadata_requests()*/ {
		go pw.metadata_request_scheduler()
	}

	for {
		msg := <- pw.message_ch

		pw.logger.Println(msg)

		if int(msg[4]) == STOP {
			return
		}

		_, err := pw.conn.Write(msg)
		if err != nil {
			return
		}
	}
}
