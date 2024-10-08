package main

import (
	"flag"
	"fmt"
	"os"
)

// var seed bool
var connections int

func init() {
	// flag.BoolVar(&seed, "seed", false, "continue seeding after download")
	flag.IntVar(&connections, "connections", 50, "number of connections to use")
	flag.Parse()
}

func main() {
	// flush current debug log
	err := os.Remove("debug.log")
	if err != nil {
		fmt.Println("[WARN] no debug.log found")
	}

	connections := 50
	var magnetLink string

	if len(os.Args) < 2 {
		fmt.Printf("Provide a magnet link\n")
		return
	}

	// TODO: validate
	magnetLink = os.Args[1]

	torr := newTorrent(magnetLink, connections)
	torr.startDownload()
}
