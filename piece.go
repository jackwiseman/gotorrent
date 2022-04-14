package main

// Piece stores a collection of blocks, so that a torrent file can be easily written
type Piece struct {
	blocks []Block
}
