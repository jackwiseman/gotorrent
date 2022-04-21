package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	// flush current debug log
	err := os.Remove("debug.log")
	if err != nil {
		fmt.Println("[WARN] no debug.log found")
	}

	connections := 50
	var magnetLink string

	switch len(os.Args) {
	case 2:
		magnetLink = os.Args[1]
	case 3:
		magnetLink = os.Args[1]
		connections, err = strconv.Atoi(os.Args[2])
		if err != nil {
			panic(err)
		}
	default:
		fmt.Printf("Usage: ./main {magnet link} {optional # conections}\n")
	}

	torr := newTorrent(magnetLink, connections)
	torr.startDownload()
}
