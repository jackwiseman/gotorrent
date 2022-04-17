package main

import (
	"fmt"
	"os"
)

func main() {
	// flush current debug log
	err := os.Remove("debug.log")
	if err != nil {
		fmt.Println("[WARN] no debug.log found")
	}

	// size := 2
	// b := make([]byte, size)
	// for i := 0; i < 8*size; i++ {
	// 	setBit(&b, i)
	// 	fmt.Printf("%v %v\n", b, bitIsSet(b, i))
	// }
	// for i := 0; i < 8*size; i++ {
	// 	unsetBit(&b, i)
	// 	fmt.Printf("%v %v\n", b, bitIsSet(b, i))
	// }

	defaultConns := 50

	// Links defined in magnet_links.go for now
	torr := newTorrent(LINK, defaultConns)
	torr.startDownload()
}
