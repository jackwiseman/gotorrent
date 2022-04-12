package main

import (
	"net"
	"time"

	//	"encoding/binary"
	"bufio"
	"log"
	"sync"
)

const KeepAlive = uint8(0)

type PeerWriter struct {
	conn    net.Conn
	writer  *bufio.Writer
	bufSize int
	peer    *Peer
	logger  *log.Logger

	messageCh chan []byte // probably need to initialize this?

	// for scheduling purposes
	keepAliveTicker       *time.Ticker
	metadataRequestTicker *time.Ticker
}

func newPeerWriter(peer *Peer) *PeerWriter {
	var pw PeerWriter
	pw.peer = peer
	pw.conn = peer.conn
	pw.bufSize = 6 // len + id + (extension id if id == 20)
	pw.writer = bufio.NewWriterSize(pw.conn, pw.bufSize)
	pw.logger = log.New(peer.torrent.logFile, "[Peer Writer] "+pw.peer.ip+": ", log.Ltime|log.Lshortfile)
	pw.messageCh = make(chan []byte)
	return &pw
}

func (pw *PeerWriter) write(message Message) {
	// TODO: investigate crash happening here
	if pw.messageCh == nil {
		return
	}
	pw.messageCh <- message.marshall()
}

func (pw *PeerWriter) writeExtended(message ExtendedMessage) {
	pw.messageCh <- message.marshall()
}

func (pw *PeerWriter) stop() {
	defer pw.logger.Println("Closed")
	pw.write(Message{1, Stop, nil})
	if pw.keepAliveTicker != nil {
		pw.keepAliveTicker.Stop()
	}
	if pw.metadataRequestTicker != nil {
		pw.metadataRequestTicker.Stop()
	}
}

// request specified metadata piece
func (pw *PeerWriter) sendMetadataRequest() {
	payload := encodeMetadataRequest(pw.peer.torrent.getRandMetadataPiece())
	// marshall will ensure the length_prefix is set, we don't need to specify it here
	pw.writeExtended(ExtendedMessage{0, 20, uint8(pw.peer.extensions["ut_metadata"]), []byte(payload)})
}

// use a time.Ticker to repeatedly request a metadata piece until we have the full file
// todo make sure mutexes are used when checking pieces
func (pw *PeerWriter) metadataRequestScheduler() {
	pw.metadataRequestTicker = time.NewTicker(15 * time.Second)
	for range pw.metadataRequestTicker.C {
		if pw.peer.torrent.hasAllMetadata() {
			pw.metadataRequestTicker.Stop()
			go pw.peer.requestNewBlock()
			// go pw.peer.queue_blocks()
			return
		} else {
			if pw.peer.supportsMetadataRequests() {
				pw.logger.Println("Requesting metadata")
				pw.sendMetadataRequest()
			}
		}
	}
}

func (pw *PeerWriter) keepAliveScheduler() {
	pw.keepAliveTicker = time.NewTicker(1 * time.Minute)
	for range pw.keepAliveTicker.C {
		pw.conn.Write([]byte{0, 0, 0, 0})
	}
	pw.keepAliveTicker.Stop()
}

func (pw *PeerWriter) run(wg *sync.WaitGroup) {
	defer wg.Done()

	go pw.keepAliveScheduler()

	pw.logger.Println(pw.peer.torrent.hasAllMetadata())
	if !pw.peer.torrent.hasAllMetadata() {
		go pw.sendMetadataRequest()
	}

	for {
		msg := <-pw.messageCh
		pw.logger.Println(msg)

		if int(msg[4]) == Stop {
			return
		}

		_, err := pw.conn.Write(msg)
		if err != nil {
			return
		}
	}
}
