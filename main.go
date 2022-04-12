package main

import (
	"os"
)

func main() {
	// flush current debug log
	os.Remove("debug.log")

	defaultConns := 50

	// Links defined in magnet_links.go for now
	torr := newTorrent(LINK, defaultConns)
	torr.startDownload()
}
