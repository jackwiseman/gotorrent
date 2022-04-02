package main

const BLOCK_LEN = 16 * 1024 // 16 KiB default block len

type Block struct {
	data   []byte
	length int
}
