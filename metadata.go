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

// Metadata stores the torrent's metadata, since we don't deal with .torrent files
type Metadata struct {
	Name     string `bencode:"name"`
	NameUtf  string `bencode:"name.utf-8"`
	PieceLen int    `bencode:"piece length"`
	Pieces   string `bencode:"pieces"`
	// contains one of the following, where 'length' means there is one file, and 'files' means there are multiple, only single file downloads will be allowed for the moment
	Length int            `bencode:"length"`
	Files  []MetadataFile `bencode:"files"`
}

// MetadataFile is a subset of Metadata for use in bencoding, since a torrent can contain multiple files
type MetadataFile struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

func (md *Metadata) String() string {
	s := "Name: " + md.Name + "\nPiece length: " + strconv.Itoa(md.PieceLen) + "\nLength: " + strconv.Itoa(md.Length)
	return s
}

func (torrent *Torrent) numMetadataPieces() int {
	return int(math.Round(float64(torrent.metadataSize)/float64(BlockLen) + 1.0))
}

func (torrent *Torrent) hasAllMetadata() bool {
	if torrent.numMetadataPieces() == 0 {
		return false
	}
	for i := 0; i < torrent.numMetadataPieces(); i++ {
		if !torrent.hasMetadataPiece(i) {
			return false
		}
	}
	return true
}

func (torrent *Torrent) getRandMetadataPiece() int {
	if torrent.hasAllMetadata() {
		return -1
	}

	rand.Seed(time.Now().UnixNano())

	for {
		testPiece := rand.Intn(torrent.numMetadataPieces())
		if !torrent.hasMetadataPiece(testPiece) {
			return testPiece
		}
	}
}

func (torrent *Torrent) hasMetadataPiece(pieceNum int) bool {
	if len(torrent.metadataPieces) == 0 {
		return false
	}
	return (torrent.metadataPieces[pieceNum/int(7)]>>(7-pieceNum%8))&1 == 1
}

func (torrent *Torrent) setMetadataPiece(pieceNum int, metadataPiece []byte) error {
	if torrent.hasAllMetadata() {
		return nil
	}
	// insert into raw byte array
	startIndex := pieceNum*BlockLen + len(metadataPiece)
	if startIndex > len(torrent.metadataRaw) {
		return errors.New("metadata piece is out of bounds")
	}

	temp := torrent.metadataRaw[pieceNum*BlockLen+len(metadataPiece):]
	torrent.metadataRaw = append(torrent.metadataRaw[0:pieceNum*BlockLen], metadataPiece...)
	torrent.metadataRaw = append(torrent.metadataRaw, temp...)

	// set as "have"
	torrent.metadataPieces[pieceNum/int(7)] = torrent.metadataPieces[pieceNum/int(7)] | (1 << (7 - pieceNum%8))

	return nil
}

func (torrent *Torrent) buildMetadataFile() error {
	err := os.WriteFile("metadata.torrent", torrent.metadataRaw, 0644)
	fmt.Println("Received metadata")
	if err != nil {
		return err
	}
	return nil
}
