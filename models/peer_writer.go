package models

import (
	"bufio"
	"sync"
	"time"
)

// KeepAlive messages are 4 byte, 0 values to signal to the peer we are still connected
const KeepAlive = uint8(0)

// PeerWriter is how we write to a peer over the connection
type PeerWriter struct {
	writer  *bufio.Writer
	bufSize int
	peer    *Peer

	messageCh chan []byte // probably need to initialize this?

	// for scheduling purposes
	keepAliveTicker       *time.Ticker
	metadataRequestTicker *time.Ticker
}

func newPeerWriter(peer *Peer) *PeerWriter {
	var pw PeerWriter
	pw.peer = peer
	pw.bufSize = 6 // len + id + (extension id if id == 20)
	pw.writer = bufio.NewWriterSize(pw.peer.conn, pw.bufSize)
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
	pw.write(Message{1, Stop, nil})
	if pw.keepAliveTicker != nil {
		pw.keepAliveTicker.Stop()
	}
	if pw.metadataRequestTicker != nil {
		pw.metadataRequestTicker.Stop()
	}
}

// request specified metadata piece
func (pw *PeerWriter) sendMetadataRequest() error {
	piece, err := pw.peer.torrent.getRandMetadataPiece()
	if err != nil {
		return err
	}
	payload := encodeMetadataRequest(piece)
	// marshall will ensure the length_prefix is set, we don't need to specify it here
	pw.writeExtended(ExtendedMessage{0, 20, uint8(pw.peer.extensions["ut_metadata"]), []byte(payload)})

	return nil
}

func (pw *PeerWriter) keepAliveScheduler() {
	pw.keepAliveTicker = time.NewTicker(1 * time.Minute)
	for range pw.keepAliveTicker.C {
		_, err := pw.peer.conn.Write([]byte{0, 0, 0, 0})
		if err != nil {
			return
		}
	}
	pw.keepAliveTicker.Stop()
}

func (pw *PeerWriter) run(wg *sync.WaitGroup) {
	defer wg.Done()

	go pw.keepAliveScheduler()

	if !pw.peer.torrent.hasMetadata {
		go pw.sendMetadataRequest()
	}

	for {
		msg := <-pw.messageCh

		if int(msg[4]) == Stop {
			return
		}

		_, err := pw.peer.conn.Write(msg)
		if err != nil {
			return
		}
	}
}
