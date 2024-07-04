package utils

import (
	"crypto/rand"
	"encoding/binary"
)

// Return a new, random 32-bit integer
func GetTransactionID() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return uint32(binary.BigEndian.Uint32(b[:])), nil
}

// Given a byte slice, set byte at position pos in big endian order
// Ie SetBit(10, b) -> [0] [64]
func SetBit(data *[]byte, pos int) {
	(*data)[pos/8] = (*data)[pos/8] | (1 << (7 - (pos % 8)))
}

func UnsetBit(data *[]byte, pos int) {
	(*data)[pos/8] = (*data)[pos/8] & ^(1 << (7 - (pos % 8)))
}

// Given a byte slice, return whether byte as position pos (big endian) is set
// Ie BitIsSet(10, [0 64]) -> true
func BitIsSet(data []byte, pos int) bool {
	return data[(pos/int(8))]>>(7-(pos%8))&1 == 1
}
