package main

import (
	"errors"
	"fmt"
	"gotorrent/utils"
	"math"
	"math/rand"
	"os"
	"strconv"
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
	return int(math.Ceil(float64(torrent.metadataSize) / float64(BlockLen)))
}

func (torrent *Torrent) hasAllMetadata() (bool, error) {
	if torrent.numMetadataPieces() == 0 {
		return false, nil
	}

	for i := 0; i < torrent.numMetadataPieces(); i++ {
		isSet, err := utils.BitIsSet(torrent.metadataPieces, i)
		if err != nil {
			return false, err
		}

		if !isSet {
			return false, nil
		}
	}
	return true, nil
}

func (torrent *Torrent) getRandMetadataPiece() (int, error) {
	hasAllMetadata, err := torrent.hasAllMetadata()
	if err != nil {
		return -1, err
	}
	if hasAllMetadata {
		return -1, nil
	}

	for {
		testPiece := rand.Intn(torrent.numMetadataPieces())
		hasPiece, err := torrent.hasMetadataPiece(testPiece)
		if err != nil {
			return -1, err
		}
		if !hasPiece {
			return testPiece, nil
		}
	}
}

func (torrent *Torrent) hasMetadataPiece(pieceNum int) (bool, error) {
	if len(torrent.metadataPieces) == 0 {
		return false, nil
	}
	return utils.BitIsSet(torrent.metadataPieces, pieceNum)
}

func (torrent *Torrent) setMetadataPiece(pieceNum int, metadataPiece []byte) error {
	hasAll, err := torrent.hasAllMetadata()
	if err != nil {
		return err
	}
	if hasAll {
		return nil
	}
	// insert into raw byte array
	startIndex := pieceNum * BlockLen
	if startIndex > len(torrent.metadataRaw) {
		return errors.New("metadata piece is out of bounds")
	}

	temp := torrent.metadataRaw[pieceNum*BlockLen+len(metadataPiece):]
	torrent.metadataRaw = append(torrent.metadataRaw[0:pieceNum*BlockLen], metadataPiece...)
	torrent.metadataRaw = append(torrent.metadataRaw, temp...)

	// set as "have"
	utils.SetBit(&torrent.metadataPieces, pieceNum)

	return nil
}

func (torrent *Torrent) buildMetadataFile() error {
	err := os.WriteFile("metadata.torrent", torrent.metadataRaw, 0644)
	fmt.Println("Received metadata")
	if err != nil {
		panic(err)
	}
	return nil
}
