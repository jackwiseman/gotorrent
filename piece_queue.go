package main

// FIFO queue pieces
type Piece_Queue struct {
	pieces []Piece_Offset_Pair	
	requests []byte // like a bitfield, this stores the contained
	blocks_per_piece int
}

type Piece_Offset_Pair struct {
	piece int
	offset int
}

// in order to not link to the torrent we need this info to build the requests array
func new_piece_queue(num_pieces int, blocks_per_piece int) (*Piece_Queue) {
	pq := Piece_Queue{nil, make([]byte, (num_pieces + 8) / 8 ), blocks_per_piece}
	return &pq
}

func (pq *Piece_Queue) push(piece int, offset int) {
	if len(pq.pieces) == 0 {
		pq.pieces = make([]Piece_Offset_Pair, 1)
		pq.pieces[1] = Piece_Offset_Pair{piece, offset}
	} else {
		copy(pq.pieces[1:], pq.pieces[0:])
		pq.pieces[0] = Piece_Offset_Pair{piece, offset}
	}

	index := (piece * pq.blocks_per_piece) + (offset / BLOCK_LEN)
	pq.requests[index/8] = pq.requests[index/8] | (1 << (7 - (index % 8)))
}

func (pq *Piece_Queue) pop () (int, int) {
	piece := pq.pieces[len(pq.pieces) - 1]
	pq.pieces = pq.pieces[:len(pq.pieces) - 1]

	index := (piece.piece * pq.blocks_per_piece) + (piece.offset / BLOCK_LEN)
	pq.requests[index/8] = pq.requests[index/8] & (0 << (7 - (index % 8)))


	return piece.piece, piece.offset
}

func (pq *Piece_Queue) contains(piece int, offset int) (bool) {
	index := (piece * pq.blocks_per_piece) + (offset / BLOCK_LEN)
	return pq.requests[index/8] & (1 << (7 - (index % 8))) == 1
}
