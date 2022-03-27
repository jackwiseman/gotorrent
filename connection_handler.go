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
	// channel for peers to notify connection handler that they've disconnected, allows us to not just run in a ticker
	done_chan chan *Peer

	logger *log.Logger

}

func (torrent *Torrent) new_connection_handler() (*Connection_Handler) {
	var ch Connection_Handler

	ch.torrent = torrent
	ch.done_chan = make(chan *Peer)
	ch.logger = log.New(torrent.log_file, "[Connection Handler] ", log.Ltime | log.Lshortfile)
	
	return &ch
}

func (ch *Connection_Handler) run () {
	defer ch.logger.Println("Connection handler finished running")

	for {
		ch.logger.Println(ch.String())
		ch.logger.Printf("Need to add %d new peers", ch.torrent.max_peers - len(ch.active_connections))
		for i := 0; i < ch.torrent.max_peers - len(ch.active_connections); i++ {
			if len(ch.active_connections) >= ch.torrent.max_peers {
				ch.logger.Println("This should never be triggered")
				// no need to add more peers -- realistically this shouldn't happen though?
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
			go ch.active_connections[len(ch.active_connections) - 1].run(ch.done_chan)
			ch.next_peer_index++



			//if ch.torrent.has_all_metadata() {
			//	for i := 0; i < len(ch.active_connections); i++ {
			//		if ch.active_connections[i].choked {
			//			ch.active_connections[i].send_interested()
			//		} else {
			//			if !ch.torrent.has_all_data() {
			//				ch.logger.Println("Attempting to get a new block")
	////						ch.active_connections[i].request_block()
			//				go ch.active_connections[i].get_new_block()
			//			} else {
			//				ch.logger.Println("Got all data, exiting!")
			//				ch.torrent.build_file()
			//				return
			//			}
			//		}
			//	}
			//}
		}
		ch.remove_connection(<- ch.done_chan) // block until someone disconnects
	}
}

// remove peer from the connection slice
func (ch *Connection_Handler) remove_connection(peer *Peer) {
	ch.logger.Println("Goodbye")
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
