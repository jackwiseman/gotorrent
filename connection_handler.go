package main

import (
	"log"
	"sync"
	"time"
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
	// channel for peers to notify connection handler that they've disconnected, allows us to not just run in a ticker
	done_chan chan *Peer

	logger *log.Logger
}

func (torrent *Torrent) new_connection_handler() *Connection_Handler {
	var ch Connection_Handler

	ch.torrent = torrent

	ch.done_chan = make(chan *Peer)
	ch.logger = log.New(torrent.log_file, "[Connection Handler] ", log.Ltime|log.Lshortfile)
	//	ch.logger.SetOutput(ioutil.Discard)

	return &ch
}

func (ch *Connection_Handler) run() {
	defer ch.logger.Println("Finished running")

	for {
		bad_peers := 0
		alive_peers := 0
		// attempt to fill up missing connections to reach max_peers
		for i := 0; i < len(ch.torrent.peers); i++ {
			if len(ch.active_connections) >= ch.torrent.max_peers {
				break
			}
			switch ch.torrent.peers[i].status {
			case BAD:
				bad_peers++
				if i == len(ch.torrent.peers)-1 && bad_peers == len(ch.torrent.peers) {
					// all peers are bad
					return
				}
			case ALIVE:
				alive_peers++
				continue
			default:
				ch.active_connections = append(ch.active_connections, &ch.torrent.peers[i])
				ch.logger.Printf(" + %s", ch.torrent.peers[i].String())
				go ch.active_connections[len(ch.active_connections)-1].run(ch.done_chan)
			}
		}
		ch.logger.Printf("Bad: %d Alive: %d Total: %d\n", bad_peers, alive_peers, len(ch.torrent.peers))
		ch.logger.Println("------------------------")
		ch.remove_connection(<-ch.done_chan) // block until someone disconnects
	}
}

// remove peer from the connection slice
func (ch *Connection_Handler) remove_connection(peer *Peer) {
	ch.logger.Printf(" - %s", peer.String())
	if len(ch.active_connections) == 1 {
		ch.active_connections = []*Peer{}
	} else {
		for i := 0; i < len(ch.active_connections); i++ {
			if ch.active_connections[i].ip == peer.ip {
				ch.active_connections[i] = ch.active_connections[len(ch.active_connections)-1]
				ch.active_connections = ch.active_connections[:len(ch.active_connections)-1]
			}
		}
	}
}

// Prints all alive connections
func (ch *Connection_Handler) String() string {
	if len(ch.active_connections) == 0 {
		return "No peers connected"
	}
	var s string
	for i := 0; i < len(ch.active_connections); i++ {
		s += ch.active_connections[i].ip + " "
	}
	return s
}
