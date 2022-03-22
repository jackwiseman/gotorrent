package main

import (
	"time"
	"sync"
	"fmt"
)

func (torrent *Torrent) peer_connection_handler () {
	
	// slice of peers currently connected
	var connected []Peer
	// index of where to grab next peer from master list of all peers discovered via trackers
	var next_peer_index int
	var edit_connected_peers sync.Mutex

	ticker := time.NewTicker(10 * time.Second)
	// and we stop this... how?
	for _ = range(ticker.C) {
//		fmt.Println("Currently connected:")
		//fmt.Println(connected)
		for {

			if len(connected) >= torrent.max_peers {
				// no need to add more peers
				fmt.Println("Enough peers connected for now")
				break
			}

			// // attempt to add more peers
			// if next_peer_index >= torrent.max_peers - 1 {
			// 	ticker.Stop()
			// 	fmt.Println("Added all possible peers")
			// 	return
			// 	// we can't add any more peers
			// }

			if next_peer_index >= len(torrent.peers) - 1 {
				ticker.Stop()
				// no more peers left to add
//				fmt.Println("No more peers left to add")
				return
			}

			fmt.Printf(" + %s\n", torrent.peers[next_peer_index].ip)
			connected = append(connected, torrent.peers[next_peer_index])
			go torrent.peers[next_peer_index].run(&connected, &edit_connected_peers)
			next_peer_index++
		}
//		fmt.Println("After adding some peers, here's what I've got: ")
		//fmt.Println(connected)
	}
}
