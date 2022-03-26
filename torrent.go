package main

import (
	"fmt"
	"os"
	"math"
	"strings"
	"strconv"
	"encoding/hex"
	"sync"	
	"io/ioutil"
	"bytes"
	bencode "github.com/jackpal/bencode-go"
)

type Torrent struct {
	magnet_link string
	display_name string
	info_hash []byte

	trackers []Tracker
	peers []Peer // all peers collected by the tracker, not necessarily connected
	max_peers int

	// Metadata-specific
	metadata_size int // in bytes, given by first extended handshake
	metadata_raw []byte
	metadata_pieces []byte // array of [1/0, 1/0,...] denoting whether we have the piece or not
	metadata Metadata
	metadata_mx sync.Mutex // to ensure that that we only trigger "building" the metadata once

	pieces []Piece
	obtained_blocks []byte // similar to 'metadata pieces', allows for quick bitwise checking which pieces we have, if the ith bit is set to 1 we have that block

	log_file *os.File

	conn_handler *Connection_Handler
}

// for simplicity, only magnet links will be supported for now
func new_torrent(magnet_link string, max_peers int) (*Torrent) {
	var torrent Torrent
	torrent.log_file, _ = os.Create("debug.log")
	torrent.magnet_link = magnet_link
	torrent.max_peers = max_peers
	torrent.parse_magnet_link()
	torrent.conn_handler = torrent.new_connection_handler()

	//	var m sync.Mutex
	//	torrent.metadata_mx = &m
	return &torrent
}

// TODO: overhaul on link parsing, this was a bit of a hack
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
	if torrent.has_all_metadata() {
		fmt.Println("Metadata info (" + strconv.Itoa(torrent.metadata_size) + " bytes with " + strconv.Itoa(torrent.num_metadata_pieces()) + " pieces)")
		fmt.Println("-------------")
		fmt.Println(torrent.metadata.String())
	}
}

// send 2x announce requests to all trackers, the first to find out how many peers they have,
// the second to request that many, so that we have a large pool to pull from
func (torrent *Torrent) find_peers() {
	var wg sync.WaitGroup

	// TODO: fix bad trackers?
	for i := 0; i < len(torrent.trackers); i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, tracker Tracker) {
			defer wg.Done()
			fmt.Println("Connecting to " + tracker.link)

			err := tracker.connect()
			if err != nil {
				return
			}

			tracker.set_connection_id()
			if err != nil {
				return
			}

			seeders, err := tracker.announce(torrent, 0)
			if err != nil {
				return
			}

			_, err = tracker.announce(torrent, seeders)
			if err != nil {
				return
			}

			tracker.disconnect()
		} (&wg, torrent.trackers[i])
	}
	wg.Wait()

	// fmt.Printf("Found %d peers\n", len(torrent.peers))
	// fmt.Println("Trimming...")
	torrent.remove_duplicate_peers()
	// fmt.Printf("Finished trim with %d peers left\n", len(torrent.peers))
}

// remove all instances of repeating peer ip addresses from torrent.peers
func (torrent *Torrent) remove_duplicate_peers() {
	seen := map[string]bool{}
	trimmed := []Peer{}

	for i := range torrent.peers {
		if !seen[torrent.peers[i].ip] {
			seen[torrent.peers[i].ip] = true
			trimmed = append(trimmed, torrent.peers[i])
		} else {
		}
	}
	torrent.peers = trimmed
}

// assumes the filename is "metadata.torrent", which of course will not be valid in the future if there are multiple torrents
func (torrent *Torrent) parse_metadata_file() (error) {
	data, err := ioutil.ReadFile("metadata.torrent")
	if err != nil {
		return err
	}

	var result = Metadata{"", "", 0, "", 0, 0}
	reader := bytes.NewReader(data)
	err = bencode.Unmarshal(reader, &result)
	if err != nil {
		return err
	}
	fmt.Println(result)
	torrent.metadata = result
	torrent.display_name = torrent.metadata.Name

	// create empty pieces slice
	fmt.Println(torrent.metadata.Length)
	fmt.Println(torrent.metadata.Piece_len)
	torrent.pieces = make([]Piece, int(math.Ceil(float64(torrent.metadata.Length) / float64(torrent.metadata.Piece_len))))
	for i := 0; i < len(torrent.pieces) - 1; i++ {
		torrent.pieces[i].blocks = make([]Block, torrent.metadata.Piece_len / (BLOCK_LEN))
	}
	torrent.pieces[len(torrent.pieces) - 1].blocks = make([]Block, int(math.Ceil(float64(torrent.metadata.Length - (torrent.metadata.Piece_len * (len(torrent.pieces) - 1))) / float64(BLOCK_LEN))))
	fmt.Println(torrent.pieces)
	torrent.obtained_blocks = make([]byte, int(math.Ceil(float64(torrent.get_num_blocks()) / float64(8))))
	
	return nil
}

// "main" function of a torrent
func (torrent *Torrent) start_download() {
	// get num_want peers and store in masterlist of peers
	torrent.find_peers()

	// eventually this will be backgrounded but ok to just connect for now
	torrent.conn_handler.run()

	torrent.print_info()

	/*	var wg sync.WaitGroup

	for i := 0; i < len(torrent.peers); i++ {
		wg.Add(1)
		go torrent.peers[i].run(torrent, &wg)
	}

	wg.Wait()*/
}

func (torrent *Torrent) set_block(piece_index int, offset int, data []byte) {
	torrent.pieces[piece_index].blocks[offset/BLOCK_LEN].data = data
	block_index := (piece_index * torrent.get_num_blocks_in_piece() + (offset / BLOCK_LEN))
	// changed to / 8 rather than 7
	torrent.obtained_blocks[block_index/8] = torrent.obtained_blocks[block_index/8] | (1 << (7 - (block_index % 8)))
	fmt.Printf("\nPiece (%d, %d) recieved\n", piece_index, offset/BLOCK_LEN)
	fmt.Println(( 1 << (7 - (block_index % 8))))
	fmt.Printf("\nBlock_index -> %d", block_index)
	fmt.Println(torrent.obtained_blocks)
	fmt.Println(torrent.has_block(piece_index, offset))
}

func (torrent *Torrent) has_block(piece_index int, offset int) (bool) {
	if torrent.obtained_blocks == nil {
		return false
	}
	// fmt.Println("-----")
	// fmt.Printf("%d, %d\n", piece_index, offset)
	// fmt.Println(torrent.obtained_blocks)
	block_index := (piece_index * torrent.get_num_blocks_in_piece() + (offset / BLOCK_LEN))
	// fmt.Println(block_index)
	// fmt.Println(torrent.obtained_blocks[block_index / 7])
	// fmt.Println(7 - (block_index % 8))
	// fmt.Println("-----")
	return torrent.obtained_blocks[block_index / 8] >> (7 - (block_index % 8)) & 1 == 1
}

func (torrent *Torrent) get_num_blocks() (int) {
	return int(math.Ceil(float64(torrent.metadata.Length) / float64(BLOCK_LEN)))
}

func (torrent *Torrent) get_num_blocks_in_piece() (int) {
	return torrent.metadata.Piece_len / BLOCK_LEN
}

func (torrent *Torrent) has_all_data() (bool) {
	for i := 0; i < torrent.get_num_blocks() / 8; i++ {
		if int(torrent.obtained_blocks[i]) != 255 {
			return false
		}
	}

	if torrent.get_num_blocks() % 8 == 0 {
		return true
	}
	fmt.Println("---")
	fmt.Println("Shift amt:")
	fmt.Println(7 - torrent.get_num_blocks() % 8)
	fmt.Println(int(torrent.obtained_blocks[len(torrent.obtained_blocks) - 1]) >> (7 - torrent.get_num_blocks() % 8))
	fmt.Println((255 >> (7 - torrent.get_num_blocks() % 8)))
	fmt.Println("---")

	return int(torrent.obtained_blocks[len(torrent.obtained_blocks) - 1]) >> (7 - torrent.get_num_blocks() % 8) == (255 >> (7 - torrent.get_num_blocks() % 8))
}
