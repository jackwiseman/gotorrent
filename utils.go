package main

import (
	"crypto/rand"
	"encoding/binary"
	math_rand "math/rand"
	"time"
)

// Return a new, random 32-bit integer
func get_transaction_id() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return uint32(binary.BigEndian.Uint32(b[:])), nil
}

// return true if given an array representing pieces of a 
// metadata/torrent [1, 0, 0, 1, 1] where 1s are downloaded
// and 0s are not, check if it is full of 1s
func need_piece(pieces []int) (bool) {
	for _, v := range pieces {
		if v == 0 {
			return true
		}
	}
	return false
}

// return random index of piece that has not been downloaded
// return -1 if we don't need pieces
func get_rand_piece(pieces []int) (int) {
	if !need_piece(pieces) {
		return -1
	}

	math_rand.Seed(time.Now().UnixNano())

	for {
		index := math_rand.Intn(len(pieces))
		if pieces[index] == 0 {
			return index
		}
	}
}
