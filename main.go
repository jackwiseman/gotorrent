package main

import (
	"flag"
	"fmt"
	"gotorrent/models"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

// var seed bool
var connections int
var debug bool

func init() {
	// flag.BoolVar(&seed, "seed", false, "continue seeding after download")
	flag.IntVar(&connections, "connections", 50, "number of connections to use")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.Parse()
}

func main() {
	// flush current debug log
	// TODO: deprecate all debug logs
	err := os.Remove("debug.log")
	if err != nil {
		fmt.Println("[WARN] no debug.log found")
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	if len(os.Args) < 2 {
		fmt.Printf("Provide a magnet link\n")
		return
	}
	magnetLink, err := models.NewMagnet(os.Args[1])

	torr := models.NewTorrent(magnetLink, connections)

	torr.StartDownload()
}
