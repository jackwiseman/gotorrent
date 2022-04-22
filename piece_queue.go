package main

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

// PieceQueue allows for goroutine-safe piece fetching to reduce piece collisions among peers and faster piece selection
type PieceQueue struct {
	pieces   []int
	pieceMap map[int]struct{}
	piecesMX sync.Mutex
}

func (pq *PieceQueue) push(pieceIndex int) {
	pq.piecesMX.Lock()
	defer pq.piecesMX.Unlock()

	pq.pieces = append(pq.pieces, pieceIndex)

	if pq.pieceMap == nil {
		pq.pieceMap = make(map[int]struct{}, 0)
	}
	pq.pieceMap[pieceIndex] = struct{}{}
	return
}

func (pq *PieceQueue) pop() (int, error) {
	pq.piecesMX.Lock()
	defer pq.piecesMX.Unlock()

	if len(pq.pieces) == 0 {
		return -1, errors.New("piece queue is empty")
	}

	popped := pq.pieces[0]
	pq.pieces = pq.pieces[1:]
	delete(pq.pieceMap, popped)
	return popped, nil
}

func (pq *PieceQueue) contains(pieceIndex int) bool {
	pq.piecesMX.Lock()
	defer pq.piecesMX.Unlock()
	_, in := pq.pieceMap[pieceIndex]
	return in
}

func newPieceQueue(size int, shuffle bool) *PieceQueue {
	pq := new(PieceQueue)
	pq.pieces = make([]int, size)

	for i := 0; i < len(pq.pieces); i++ {
		pq.pieces[i] = i
	}

	if shuffle {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(pq.pieces), func(i, j int) { pq.pieces[i], pq.pieces[j] = pq.pieces[j], pq.pieces[i] })
	}

	return pq
}
