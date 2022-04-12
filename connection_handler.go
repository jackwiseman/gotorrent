package main

import (
	"log"
)

// ConnectionHandler runs once we have a list of non-duplicate peers, connecting to those peers and managing their connections
type ConnectionHandler struct {
	// contains all peers which are either active or connecting
	activeConns []*Peer
	// associated torrent
	torrent *Torrent
	// channel for peers to notify connection handler that they've disconnected, allows us to not just run in a ticker
	doneChan chan *Peer

	logger *log.Logger
}

func (torrent *Torrent) newConnHandler() *ConnectionHandler {
	var ch ConnectionHandler
	ch.torrent = torrent
	ch.doneChan = make(chan *Peer)
	ch.logger = log.New(torrent.logFile, "[Connection Handler] ", log.Ltime|log.Lshortfile)
	return &ch
}

func (ch *ConnectionHandler) run() {
	defer ch.logger.Println("Finished running")

	for {
		badPeers := 0
		alivePeers := 0
		// attempt to fill up missing connections to reach max_peers
		for i := 0; i < len(ch.torrent.peers); i++ {
			if len(ch.activeConns) >= ch.torrent.maxPeers {
				break
			}
			switch ch.torrent.peers[i].status {
			case Bad:
				badPeers++
				if i == len(ch.torrent.peers)-1 && badPeers == len(ch.torrent.peers) {
					// all peers are bad
					return
				}
			case Alive:
				alivePeers++
				continue
			default:
				ch.activeConns = append(ch.activeConns, &ch.torrent.peers[i])
				ch.logger.Printf(" + %s", ch.torrent.peers[i].String())
				go ch.activeConns[len(ch.activeConns)-1].run(ch.doneChan)
			}
		}
		ch.logger.Printf("Bad: %d Alive: %d Total: %d\n", badPeers, alivePeers, len(ch.torrent.peers))
		ch.logger.Println("------------------------")
		ch.removeConnection(<-ch.doneChan) // block until someone disconnects
	}
}

// remove peer from the connection slice
func (ch *ConnectionHandler) removeConnection(peer *Peer) {
	ch.logger.Printf(" - %s", peer.String())
	if len(ch.activeConns) == 1 {
		ch.activeConns = []*Peer{}
	} else {
		for i := 0; i < len(ch.activeConns); i++ {
			if ch.activeConns[i].ip == peer.ip {
				ch.activeConns[i] = ch.activeConns[len(ch.activeConns)-1]
				ch.activeConns = ch.activeConns[:len(ch.activeConns)-1]
			}
		}
	}
}

// Prints all alive connections
func (ch *ConnectionHandler) String() string {
	if len(ch.activeConns) == 0 {
		return "No peers connected"
	}
	var s string
	for i := 0; i < len(ch.activeConns); i++ {
		s += ch.activeConns[i].ip + " "
	}
	return s
}
