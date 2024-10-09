package models

import (
	"bytes"
	"crypto/sha1"
)

// Piece stores a collection of blocks, so that a torrent file can be easily written
type Piece struct {
	blocks     []Block
	hash       []byte // sha1 hash of len 20
	isVerified bool   // whether this piece has been verified via sha1 hash
	numSet     int    // number of blocks that currently have data in them
}

func (piece *Piece) verify() bool {
	// build slice of entire piece
	var joined []byte
	for i := 0; i < len(piece.blocks); i++ {
		joined = append(joined, piece.blocks[i].data...)
	}

	checksum := sha1.Sum(joined)
	if bytes.Compare(checksum[:], piece.hash) != 0 {
		return false
	}
	return true

}
