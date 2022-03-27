package main

import (
	"os"
)

func main() {
	// flush current debug log
	os.Remove("debug.log")

	default_connections := 30

	// Links defined in magnet_links.go for now
	torrent := new_torrent(LINK, default_connections)
	torrent.start_download()
	

	// testing logic
	//blocks_in_piece := 2 // for now
////	ob := 
	//for piece_index := 0; piece_index < 15; piece_index++ {
	//	for offset := 0; offset < BLOCK_LEN * blocks_in_piece; offset += BLOCK_LEN {
	//		pos_in_array := (piece_index * blocks_in_piece + (offset / BLOCK_LEN))
	//		shift:= 1 << (7- (pos_in_array % 8))
	//		fmt.Printf("(%d, %d) - %d, %d\n", piece_index, offset/BLOCK_LEN, shift, pos_in_array)
	//	}
	//}
//	block_index := (piece_index * torrent.get_num_blocks_in_piece() + (offset / BLOCK_LEN))
//	return ob[block_index / 8] >> (7 - (block_index % 8)) & 1 == 1
}
