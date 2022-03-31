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
}
