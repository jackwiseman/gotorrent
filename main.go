package main

import (
	"os"
)

func main() {
	// flush current debug log
	err := os.Remove("debug.log")
	if err != nil {
		panic(err)
	}

	defaultConns := 50

	// Links defined in magnet_links.go for now
	torr := newTorrent(LINK, defaultConns)
	torr.startDownload()
}
