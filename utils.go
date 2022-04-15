package main

import (
	"crypto/rand"
	"encoding/binary"
)

// Return a new, random 32-bit integer
func getTransactionID() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return uint32(binary.BigEndian.Uint32(b[:])), nil
}

// Given a byte slice, set byte at position pos in big endian order
// Ie setByte(10, b) -> [0] [64]
func setByte(data *[]byte, pos int) {
	(*data)[pos/8] = (*data)[pos/8] | (1 << (7 - (pos % 8)))
}

// Given a byte slice, return whether byte as position pos (big endian) is set
// Ie byteIsSet(10, [0 64]) -> true
func byteIsSet(data []byte, pos int) bool {
	return data[(pos/int(8))]>>(7-(pos%8))&1 == 1
}
