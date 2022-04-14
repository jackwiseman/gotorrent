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
