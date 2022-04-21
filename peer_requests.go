package main

import "sync"

// PeerRequests bundles together the pieces that this peer is requesting and the associated blocks
type PeerRequests struct {
	pq       *PieceQueue
	blocks   map[int][]byte // maps the piece to the corresponding blocks
	blocksMX sync.Mutex
}
