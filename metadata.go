package main

import (
	bencode "github.com/jackpal/bencode-go"
	"io/ioutil"
	"bytes"
	"fmt"
)

type Metadata struct {
	Name string "name"
	Name_utf string "name.utf-8"
	Piece_len int "piece length"
	Pieces string "pieces"
	// contains one of the following, where 'length' means there is one file, and 'files' means there are multiple, only single file downloads will be allowed for the moment
	Length int "length"
	Files int "files"
}

// assumes the filename is "metadata.torrent", which of course will not be valid in the future if there are multiple torrents
func (torrent *Torrent) parse_metadata_file() {
	data, err := ioutil.ReadFile("metadata.torrent")
	if err != nil {
		panic(err)
	}
	
	var result = Metadata{"", "", 0, "", 0, 0}
	reader := bytes.NewReader(data)
	err = bencode.Unmarshal(reader, &result)
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
//	return &result
}
