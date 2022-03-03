package main

import (
	"fmt"
	"strings"
	"encoding/hex"
)

type Torrent struct {
	magnet_link string
	display_name string
	info_hash []byte
	trackers []Tracker
	metadata_size int // in bytes
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
}

func (torrent *Torrent) find_peers() {
	for i := 0; i < len(torrent.trackers); i++ {
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


