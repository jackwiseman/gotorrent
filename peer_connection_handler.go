package main

import (
	"time"
	"sync"
//	"log"
)

func (torrent *Torrent) peer_connection_handler () {
	
	// slice of peers currently connected
	var connected []Peer
	// index of where to grab next peer from master list of all peers discovered via trackers
	var next_peer_index int
	var edit_connected_peers sync.Mutex
//	logger := log.New(torrent.log_file, "[Connection Handler] ", log.Ltime)

	ticker := time.NewTicker(10 * time.Second)
	for _ = range(ticker.C) {
		for {
			// TODO: clean up this logic
			// TODO: global force stop, disconnects all current peers

			if len(connected) >= torrent.max_peers {
				// no need to add more peers
				break
			}

			if next_peer_index >= len(torrent.peers) - 1 {
				ticker.Stop()
				// no more peers left to add
				return
			}

			connected = append(connected, torrent.peers[next_peer_index])
			go torrent.peers[next_peer_index].run(&connected, &edit_connected_peers)
			next_peer_index++
		}
	}
}
