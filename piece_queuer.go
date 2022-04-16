package main

import (
	"encoding/binary"
	"math/rand"
	"sync"
	"time"
)

// PieceQueuer runs as a goroutine inside a peer, adding blocks when it's channel gets a message
type PieceQueuer struct {
	ch          chan bool
	peer        *Peer
	requests    int
	maxRequests int
	requestsMTX sync.Mutex
}

func newPieceQueuer(peer *Peer) *PieceQueuer {
	var pieceQueuer PieceQueuer
	pieceQueuer.peer = peer
	pieceQueuer.maxRequests = peer.maxRequests
	return &pieceQueuer
}

func (pieceQueuer *PieceQueuer) run() {
	for {
		switch <-pieceQueuer.ch {
		case true:

		case false:
			return
		}
	}
}

func (pieceQueuer *PieceQueuer) requestBlock() {
	rand.Seed(time.Now().UnixNano())

	piece := rand.Intn(len(pieceQueuer.peer.torrent.pieces))
	offset := rand.Intn(len(pieceQueuer.peer.torrent.pieces[piece].blocks))
	for !pieceQueuer.peer.hasPiece(piece) && pieceQueuer.peer.torrent.hasBlock(piece, offset) {
		piece = rand.Intn(len(pieceQueuer.peer.torrent.pieces))
		offset = rand.Intn(len(pieceQueuer.peer.torrent.pieces[piece].blocks))
	}

	pieceQueuer.peer.logger.Printf("\nRequsting block (%d, %d)", piece, offset)

	// Create message
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:], uint32(piece))
	binary.BigEndian.PutUint32(payload[4:], uint32(offset*BlockLen))

	// if this is the last blocks, we need to request the correct len
	if (piece*pieceQueuer.peer.torrent.getNumBlocksInPiece())+offset+1 == pieceQueuer.peer.torrent.getNumBlocks() {
		binary.BigEndian.PutUint32(payload[8:], uint32(pieceQueuer.peer.torrent.metadata.Length%BlockLen))
	} else {
		binary.BigEndian.PutUint32(payload[8:], uint32(BlockLen))
	}

	// Confirm that we should request
	pieceQueuer.requestsMTX.Lock()
	if pieceQueuer.requests < pieceQueuer.maxRequests {
		pieceQueuer.peer.pw.write(Message{13, Request, payload})
		pieceQueuer.requestsMTX.Unlock()
		pieceQueuer.requests++
	}
}
