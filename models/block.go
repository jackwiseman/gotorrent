package models

// BlockLen is the default 16KiB block length
const BlockLen = 16 * 1024

// Block is the lowest form of data, what we receive from a peer in a piece message, if we request the entire block
type Block struct {
	data []byte
}
