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
	// 	setByte(&b, i)
	// 	fmt.Printf("%v %v\n", b, byteIsSet(b, i))
	// }
	defaultConns := 50

	// Links defined in magnet_links.go for now
	torr := newTorrent(LINK, defaultConns)
	torr.startDownload()
}
