package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type Metadata struct {
	Name      string "name"
	Name_utf  string "name.utf-8"
	Piece_len int    "piece length"
	Pieces    string "pieces"
	// contains one of the following, where 'length' means there is one file, and 'files' means there are multiple, only single file downloads will be allowed for the moment
	Length int             "length"
	Files  []Metadata_file "files"
}

// for use in bencoding
type Metadata_file struct {
	Length int      "length"
	Path   []string "path"
	//	Path map[string]string //"path"
}

func (md *Metadata) String() string {
	s := "Name: " + md.Name + "\nPiece length: " + strconv.Itoa(md.Piece_len) + "\nLength: " + strconv.Itoa(md.Length)
	return s
}

func (torrent *Torrent) num_metadata_pieces() int {
	return int(math.Round(float64(torrent.metadata_size)/float64(16*1024) + 1.0))
}

func (torrent *Torrent) has_all_metadata() bool {
	if torrent.num_metadata_pieces() == 0 {
		return false
	}
	for i := 0; i < torrent.num_metadata_pieces(); i++ {
		if !torrent.has_metadata_piece(i) {
			return false
		}
	}
	return true
}

func (torrent *Torrent) get_rand_metadata_piece() int {
	if torrent.has_all_metadata() {
		return -1
	}

	rand.Seed(time.Now().UnixNano())

	for {
		test_piece := rand.Intn(torrent.num_metadata_pieces())
		if !torrent.has_metadata_piece(test_piece) {
			return test_piece
		}
	}
}

func (torrent *Torrent) has_metadata_piece(piece_num int) bool {
	if len(torrent.metadata_pieces) == 0 {
		return false
	}
	return (torrent.metadata_pieces[piece_num/int(7)]>>(7-piece_num%8))&1 == 1
}

func (torrent *Torrent) set_metadata_piece(piece_num int, metadata_piece []byte) error {
	// insert into raw byte array
	start_index := piece_num*(1024*16) + len(metadata_piece)
	if start_index > len(torrent.metadata_raw) {
		return errors.New("Out of bounds")
	}

	temp := torrent.metadata_raw[piece_num*(1024*16)+len(metadata_piece):]
	torrent.metadata_raw = append(torrent.metadata_raw[0:piece_num*(1024*16)], metadata_piece...)
	torrent.metadata_raw = append(torrent.metadata_raw, temp...)

	// set as "have"
	torrent.metadata_pieces[piece_num/int(7)] = torrent.metadata_pieces[piece_num/int(7)] | (1 << (7 - piece_num%8))

	return nil
}

func (torrent *Torrent) build_metadata_file() error {
	err := os.WriteFile("metadata.torrent", torrent.metadata_raw, 0644)
	fmt.Println("Received metadata")
	if err != nil {
		return err
	}
	return nil
}
