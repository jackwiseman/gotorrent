package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	bencode "github.com/jackpal/bencode-go"
)

type Torrent struct {
	magnet_link  string
	display_name string
	info_hash    []byte

	trackers  []Tracker
	peers     []Peer // all peers collected by the tracker, not necessarily connected
	max_peers int

	// Metadata-specific
	metadata_size   int // in bytes, given by first extended handshake
	metadata_raw    []byte
	metadata_pieces []byte // array of [1/0, 1/0,...] denoting whether we have the piece or not
	metadata        Metadata
	metadata_mx     sync.Mutex // to ensure that that we only trigger "building" the metadata once

	pieces          []Piece
	obtained_blocks []byte // similar to 'metadata pieces', allows for quick bitwise checking which pieces we have, if the ith bit is set to 1 we have that block
	num_pieces_mx   sync.Mutex
	num_pieces      int

	is_downloaded bool
	downloaded_mx sync.Mutex

	log_file *os.File

	conn_handler *Connection_Handler
	progress_bar Bar
}

// for simplicity, only magnet links will be supportd for no
func new_torrent(magnet_link string, max_peers int) *Torrent {
	var torrent Torrent
	torrent.log_file, _ = os.Create("debug.log")
	torrent.magnet_link = magnet_link
	torrent.max_peers = max_peers
	torrent.parse_magnet_link()
	torrent.conn_handler = torrent.new_connection_handler()

	return &torrent
}

// TODO: overhaul on link parsing, this was a bit of a hack
// only supporting udp links
func (torrent *Torrent) parse_magnet_link() {
	data := strings.Split(torrent.magnet_link, "&")
	for i := 0; i < len(data); i++ {
		switch data[i][:2] {
		case "dn":
			torrent.display_name = strings.Replace(data[i][3:], "%20", " ", -1)
		case "tr":
			tracker_link := data[i][3:] // cut off the tr=
			tracker_len := len(tracker_link)
			index := 0

			for index < tracker_len {
				if strings.Compare(string(tracker_link[index]), "%") == 0 {
					token, err := hex.DecodeString(string(tracker_link[index+1 : index+3]))
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
					tracker_link = tracker_link[:len(tracker_link)-len("/announce")]
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

func (torrent *Torrent) print_info() {
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
	if torrent.metadata.Length != 0 {
		fmt.Println("Metadata info (" + strconv.Itoa(torrent.metadata_size) + " bytes with " + strconv.Itoa(torrent.num_metadata_pieces()) + " pieces)")
		fmt.Println("-------------")
		fmt.Println(torrent.metadata.String())
	}
}

// send 2x announce requests to all trackers, the first to find out how many peers they have,
// the second to request that many, so that we have a large pool to pull from
func (torrent *Torrent) find_peers() {
	var wg sync.WaitGroup

	fmt.Println("Contacting trackers...")
	// TODO: fix bad trackers?
	for i := 0; i < len(torrent.trackers); i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, tracker Tracker) {
			defer wg.Done()

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
		}(&wg, torrent.trackers[i])
	}
	wg.Wait()

	torrent.remove_duplicate_peers()
	fmt.Printf("%d peers in swarm\n", len(torrent.peers))
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

// assumes the filename is "metadata.torrent",whichof course will not be valid in the future if there are multiple torrents
func (torrent *Torrent) parse_metadata_file() error {
	data, err := ioutil.ReadFile("metadata.torrent")
	if err != nil {
		return err
	}

	var result = Metadata{"", "", 0, "", 0, nil}
	reader := bytes.NewReader(data)
	err = bencode.Unmarshal(reader, &result)
	if err != nil {
		return err
	}

	torrent.metadata = result

	if len(result.Files) >= 1 {
		for i := 0; i < len(result.Files); i++ {
			torrent.metadata.Length += result.Files[i].Length
		}
	}

	torrent.display_name = torrent.metadata.Name

	// create empty pieces slice
	torrent.pieces = make([]Piece, int(math.Ceil(float64(torrent.metadata.Length)/float64(torrent.metadata.Piece_len))))
	for i := 0; i < len(torrent.pieces)-1; i++ {
		torrent.pieces[i].blocks = make([]Block, torrent.metadata.Piece_len/(BLOCK_LEN))
	}
	torrent.pieces[len(torrent.pieces)-1].blocks = make([]Block, int(math.Ceil(float64(torrent.metadata.Length-(torrent.metadata.Piece_len*(len(torrent.pieces)-1)))/float64(BLOCK_LEN))))
	torrent.obtained_blocks = make([]byte, int(math.Ceil(float64(torrent.get_num_blocks())/float64(8))))

	torrent.progress_bar.new_option(0, int64(torrent.get_num_blocks()))
	return nil
}

// "main" function of a torrent
func (torrent *Torrent) start_download() {
	// get num_want peers and store in masterlist of peers
	torrent.find_peers()

	// eventually this will be backgrounded but ok to just connect for now

	torrent.conn_handler.run()

	torrent.print_info()
}

func (torrent *Torrent) set_block(piece_index int, offset int, data []byte) {
	defer torrent.num_pieces_mx.Unlock()
	torrent.num_pieces_mx.Lock()

	if torrent.has_block(piece_index, offset) {
		return
	}

	torrent.pieces[piece_index].blocks[offset/BLOCK_LEN].data = data
	block_index := (piece_index*torrent.get_num_blocks_in_piece() + (offset / BLOCK_LEN))
	torrent.obtained_blocks[block_index/8] = torrent.obtained_blocks[block_index/8] | (1 << (7 - (block_index % 8)))
	torrent.num_pieces++
	torrent.progress_bar.play(int64(torrent.num_pieces))
	//	fmt.Printf("%g%% - %d/%d blocks received\n", math.Round((float64(torrent.num_pieces)/float64(torrent.get_num_blocks())*10000))/100, torrent.num_pieces, torrent.get_num_blocks())
}

func (torrent *Torrent) has_block(piece_index int, offset int) bool {
	if torrent.obtained_blocks == nil {
		return false
	}
	block_index := (piece_index*torrent.get_num_blocks_in_piece() + (offset / BLOCK_LEN))
	return torrent.obtained_blocks[block_index/8]>>(7-(block_index%8))&1 == 1
}

func (torrent *Torrent) get_num_blocks() int {
	return int(math.Ceil(float64(torrent.metadata.Length) / float64(BLOCK_LEN)))
}

func (torrent *Torrent) get_num_blocks_in_piece() int {
	return torrent.metadata.Piece_len / BLOCK_LEN
}

func (torrent *Torrent) check_download_status() {
	torrent.downloaded_mx.Lock()
	if torrent.has_all_data() && torrent.is_downloaded == false {
		torrent.build_file()
		torrent.is_downloaded = true
	}
	torrent.downloaded_mx.Unlock()
}

func (torrent *Torrent) has_all_data() bool {
	for i := 0; i < torrent.get_num_blocks()/8; i++ {
		if int(torrent.obtained_blocks[i]) != 255 {
			return false
		}
	}

	if torrent.get_num_blocks()%8 == 0 {
		return true
	}

	return int(torrent.obtained_blocks[len(torrent.obtained_blocks)-1])>>(8-torrent.get_num_blocks()%8) == (255 >> (8 - torrent.get_num_blocks()%8))
}

func (torrent *Torrent) build_file() {
	torrent.progress_bar.finish()
	if len(torrent.metadata.Files) > 1 {
		// Create new directory
		path := "downloads/" + torrent.display_name + "/"
		err := os.MkdirAll("downloads/"+torrent.display_name, 0770)
		if err != nil {
			fmt.Println(err)
		}

		var bytes_written int
		for i := 0; i < len(torrent.metadata.Files); i++ {
			torrent.create_file(bytes_written, torrent.metadata.Files[i].Length, path, torrent.metadata.Files[i].Path[0])
			bytes_written += torrent.metadata.Files[i].Length
		}
	} else {
		// Single files
		file, _ := os.Create("downloads/" + torrent.metadata.Name)
		for i := 0; i < len(torrent.pieces); i++ {
			for j := 0; j < len(torrent.pieces[i].blocks); j++ {
				_, _ = file.Write(torrent.pieces[i].blocks[j].data)
			}
		}
	}
}

// Writes one file to given directory (util function for build_file)
func (torrent *Torrent) create_file(offset int, file_size int, path string, name string) {
	file, _ := os.Create(path + name)

	if file_size < BLOCK_LEN {
	}

	// write data spilling into front block, ie block 1 here [  xx] [xxxx] [xx  ]

	start_piece := offset / BLOCK_LEN / torrent.get_num_blocks_in_piece()
	start_block := offset / BLOCK_LEN % torrent.get_num_blocks_in_piece()

	bytes_written, _ := file.Write(torrent.pieces[start_piece].blocks[start_block].data[offset%BLOCK_LEN:])

	// finish this piece for easy iteration -- this assumes that there is > BLOCK_LEN * torrent.get_num_blocks_in_piece() data left to write
	for i := start_block + 1; i < len(torrent.pieces[start_piece].blocks); i++ {
		b, err := file.Write(torrent.pieces[start_piece].blocks[i].data)
		if err != nil {
		}
		bytes_written += b
	}
	start_piece++

	blocks_to_write := (file_size - bytes_written) / BLOCK_LEN

	for i := start_piece; i < start_piece+(blocks_to_write/torrent.get_num_blocks_in_piece()); i++ {
		for j := 0; j < torrent.get_num_blocks_in_piece(); j++ {
			if i == start_piece+(blocks_to_write/torrent.get_num_blocks_in_piece())-1 && j == (blocks_to_write%torrent.get_num_blocks_in_piece()) {
				break
			}
			b, _ := file.Write(torrent.pieces[i].blocks[j].data)
			bytes_written += b
		}

	}

	// write data spilling into back block
	last_piece := (file_size + offset) / BLOCK_LEN / torrent.get_num_blocks_in_piece()
	last_block := (file_size + offset) / BLOCK_LEN % torrent.get_num_blocks_in_piece()
	_, _ = file.Write(torrent.pieces[last_piece].blocks[last_block].data[:file_size%BLOCK_LEN])

	for i := 0; i < len(torrent.pieces); i++ {
		for j := 0; j < len(torrent.pieces[i].blocks); j++ {
			_, _ = file.Write(torrent.pieces[i].blocks[j].data)
		}
	}
}
