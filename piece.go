package main

// Piece stores a collection of blocks, so that a torrent file can be easily written
type Piece struct {
	blocks   []Block
	hash     [20]byte // sha1 hash
	verified bool     // whether this piece has been verified via sha1 hash
	numSet   int      // number of blocks that currently have data in them
}
