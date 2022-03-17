package main

import (
	"fmt"
	"os"
	"strings"
	"strconv"
	"encoding/hex"
)

type Torrent struct {
	magnet_link string
	display_name string
	info_hash []byte
	trackers []Tracker
	metadata_size int // in bytes
	metadata_pieces int
	peers []Peer
}

// for simplicity, only magnet links will be supported for now
func new_torrent(magnet_link string) (*Torrent) {
	var torrent Torrent
	torrent.magnet_link = magnet_link
	torrent.parse_magnet_link()
	return &torrent
}

// only supporting udp links
func (torrent *Torrent) parse_magnet_link() {
	data := strings.Split(torrent.magnet_link, "&")
	for i := 0; i < len(data); i++ {
		switch(data[i][:2]) {
		case "dn":
			torrent.display_name = strings.Replace(data[i][3:], "%20", " ", -1)
		case "tr":
			tracker_link := data[i][3:] // cut off the tr=
			tracker_len := len(tracker_link)
			index := 0

			for index < tracker_len {
				if strings.Compare(string(tracker_link[index]), "%") == 0 {
					token, err := hex.DecodeString(string(tracker_link[index+1:index+3]))
					if err != nil {
						panic(err)
					}
					tracker_link = string(tracker_link[0:index]) + string(token) + string(tracker_link[index+3:])
					tracker_len -= 2
				}
				index++
			}
			if tracker_link[0:3] == "udp" {
				if strings.Contains(tracker_link, "announce") {
					tracker_link = tracker_link[:len(tracker_link) - len("/announce")]
				}
				new_tracker := new_tracker(tracker_link[6:])
				torrent.trackers = append(torrent.trackers, *new_tracker)
			}
		default:
			hash, err := hex.DecodeString(data[i][strings.LastIndex(data[i], ":")+1:])
			if err != nil {
				panic(err)
			}
			torrent.info_hash = hash
		}
	}

}

func (torrent Torrent) print_info() {
	fmt.Println("Name: " + torrent.display_name)
	fmt.Println("Magnet: " + torrent.magnet_link)
	fmt.Println("Trackers:")
	for i := 0; i < len(torrent.trackers); i++ {
		fmt.Println(" -- " + torrent.trackers[i].link)
	}
	fmt.Println("Known peers:")
	if len(torrent.peers) == 0 {
		fmt.Println(" -- None")
	} else {
		for i := 0; i < len(torrent.peers); i++ {
			fmt.Println(" -- " + torrent.peers[i].ip)
		}
	}
	if torrent.metadata_size != 0 {
		fmt.Println("Metadata size: " + strconv.Itoa(torrent.metadata_size) + " (" + strconv.Itoa(torrent.metadata_pieces) + " pieces)")
	}
}

func (torrent *Torrent) find_peers() {
	// coppertracker is being a pain, so i'm just going to skip it for now
	// TODO: replace i=1 with i=0 before deploying
	for i := 1; i < len(torrent.trackers); i++ {
		fmt.Println("Connecting to " + torrent.trackers[i].link)

		err := torrent.trackers[i].connect()
		if err != nil {
			continue
		}

		torrent.trackers[i].set_connection_id()
		if err != nil {
			continue
		}

		torrent.trackers[i].announce(torrent)
		if err != nil {
			continue
		}

		torrent.trackers[i].disconnect()
	}
}

func (torrent *Torrent) get_metadata() {
	// first let's find an alive peer to find the size of the file

	//var metadata_peers []Peer // we'll store num_pieces of peers in this slice so we can request a different piece of the metadata from each, eventually we can do this algorithmically, but for testing purposes this will do

	pieces := 0
	var metadata_raw []byte

	for i := 0; i < len(torrent.peers); i++ {
		torrent.peers[i].connect()
		torrent.peers[i].perform_handshake(torrent)
		if torrent.peers[i].uses_extended {
			_, supports_metadata := torrent.peers[i].extensions["ut_metadata"]
			if supports_metadata {
				fmt.Println("Requesting piece " + strconv.Itoa(pieces))
				successful, data := torrent.peers[i].request_metadata(pieces)
				if successful {
					pieces = pieces + 1
					metadata_raw = append(metadata_raw[:], data[:]...)
				}
				//				metadata_peers = append(metadata_peers, torrent.peers[i])
				//				if len(metadata_peers) == torrent.metadata_pieces {
				if pieces == torrent.metadata_pieces {
					torrent.peers[i].disconnect()
					break
				}
			}
		}
		torrent.peers[i].disconnect()
	}

	// Write all the data we've gotten to a file
	file, err := os.OpenFile(
		"metadata.torrent",
		os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
		0666,
	)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Write bytes to file
	bytesWritten, err := file.Write(metadata_raw)
	if err != nil {
		panic(err)
	}
	fmt.Println("Wrote " + strconv.Itoa(bytesWritten) + " bytes.")

	// now that we have our metadata_peers, ask each of them for a different piece (later needs to be done synchronously)
	//	for i := 0; i < torrent.metadata_pieces; i++ {
	//		fmt.Println("Requesting piece " + strconv.Itoa(i))
	//		metadata_peers[i].request_metadata(i)
	//	}
}
