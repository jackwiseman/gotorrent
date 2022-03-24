package main

import (
	"time"
	"sync"
	"log"
)

type Connection_Handler struct {
	// contains all peers which are either active or connecting
	active_connections []*Peer
	// index of where to grab next peer from torrent's master list of tracker-discovered peers
	next_peer_index int
	// ensures that peers are able to concurrently remove themselves from the connection handler
	edit_connected_peers sync.Mutex
	// check the connections every 'ticker seconds
	ticker *time.Ticker
	// associated torrent
	torrent *Torrent

	logger *log.Logger

}

func (torrent *Torrent) new_connection_handler() (*Connection_Handler) {
	var ch Connection_Handler

	ch.torrent = torrent
	ch.logger = log.New(torrent.log_file, "[Connection Handler] ", log.Ltime | log.Lshortfile)
	
	return &ch
}

func (ch *Connection_Handler) run () {
	ch.ticker = time.NewTicker(10 * time.Second)
	for _ = range(ch.ticker.C) {
		ch.logger.Println(ch.String())
//		ch.logger.Printf("There are currently %d connected peers", len(ch.active_connections))
		for {
			if ch.torrent.has_all_metadata() {
				for i := 0; i < len(ch.active_connections); i++ {
					if ch.active_connections[i].choked {
						ch.active_connections[i].send_interested()
					} else {
						ch.active_connections[i].request_block()
					}
				}
			}

			if len(ch.active_connections) >= ch.torrent.max_peers {
				// no need to add more peers
				break
			}

			if ch.next_peer_index >= len(ch.torrent.peers) - 1 {
				if len(ch.active_connections) == 0 {
					ch.logger.Println("Connected to all peers")
					return
				}
				ch.logger.Printf("No more peers left to add")
				// no more peers left to add
				break
			}

			ch.active_connections = append(ch.active_connections, &ch.torrent.peers[ch.next_peer_index])
			go ch.active_connections[len(ch.active_connections) - 1].run()
//			go ch.torrent.peers[ch.next_peer_index].run()
			ch.next_peer_index++
		}
	}
}

// remove peer from the connection slice
func (ch *Connection_Handler) remove_connection(peer *Peer) {
	ch.edit_connected_peers.Lock()
	if len(ch.active_connections) == 1 {
		ch.active_connections = []*Peer{}
	} else {
		for i := 0; i < len(ch.active_connections); i++ {
			if ch.active_connections[i].ip == peer.ip {
				ch.active_connections[i] = ch.active_connections[len(ch.active_connections) - 1]
				ch.active_connections = ch.active_connections[:len(ch.active_connections) - 1] 
			}
		}
	}
	ch.edit_connected_peers.Unlock()
}

// Prints all alive connections
func (ch *Connection_Handler) String() (string) {
	if len(ch.active_connections) == 0 {
		return "No peers connected"
	}
	var s string
	for i:=0; i<len(ch.active_connections); i++ {
		s += ch.active_connections[i].ip + " "
	}
	return s
}
