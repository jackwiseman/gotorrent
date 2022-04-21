package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"log"
	"sync"
	"time"
)

// PeerReader reads from the peer's connection and parses the messages
type PeerReader struct {
	peer   *Peer
	logger *log.Logger
}

func newPeerReader(peer *Peer) *PeerReader {
	var pr PeerReader
	pr.peer = peer
	pr.logger = log.New(peer.torrent.logFile, "[Peer Reader] "+pr.peer.ip+": ", log.Ltime|log.Lshortfile)
	pr.logger.SetOutput(ioutil.Discard)
	return &pr
}

func (pr *PeerReader) run(wg *sync.WaitGroup) {
	defer func() {
		if pr.peer.status != Bad {
			pr.peer.status = Dead
		}
		pr.peer.pw.stop()
		wg.Done()
	}()

	for {
		// disconnect if we don't receive a KEEP ALIVE (or any message) for 2 minutes
		err := pr.peer.conn.SetReadDeadline(time.Now().Add(time.Minute * time.Duration(2)))
		if err != nil {
			panic(err)
		}

		lengthPrefixBuf := make([]byte, 4)
		_, err = io.ReadFull(pr.peer.conn, lengthPrefixBuf)
		if err != nil {
			// NOTE: most of these erros end up being EOF, not entirely sure why
			return
		}

		lengthPrefix := int(binary.BigEndian.Uint32(lengthPrefixBuf))

		if lengthPrefix == 0 {
			continue
		}

		messageIDBuf := make([]byte, 1)
		_, err = pr.peer.conn.Read(messageIDBuf)
		if err != nil {
			pr.logger.Println(err)
			return
		}

		messageID := int(messageIDBuf[0])

		switch int(messageID) {
		// no payload
		case Choke:
			pr.peer.choked = true
			continue
		case Unchoke:
			pr.peer.choked = false
			if pr.peer.torrent.hasMetadata {
				go pr.peer.requestPieces()
			}
			continue
		case Interested:
			continue
		case NotInterested:
			continue
		case Have:
			pieceIndexBuf := make([]byte, 4)
			_, err = io.ReadFull(pr.peer.conn, pieceIndexBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
			// A malicious peer may send a HAVE message with a piece we'll never download
			//setBit(&pr.peer.bitfield, int(binary.BigEndian.Uint32(pieceIndexBuf)))
		case Bitfield:
			bitfieldBuf := make([]byte, lengthPrefix-1)
			_, err = pr.peer.conn.Read(bitfieldBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
			pr.peer.bitfield = bitfieldBuf
		case Request:
			// index, begin, length
			payloadBuf := make([]byte, 12)

			_, err = pr.peer.conn.Read(payloadBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
		case PIECE:
			if lengthPrefix > BlockLen+9 {
				continue
			}

			indexBuf := make([]byte, 4)
			beginBuf := make([]byte, 4)

			_, err = pr.peer.conn.Read(indexBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}

			_, err = pr.peer.conn.Read(beginBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}

			index := int(binary.BigEndian.Uint32(indexBuf))
			offset := int(binary.BigEndian.Uint32(beginBuf))

			if offset/BlockLen > pr.peer.torrent.getNumBlocksInPiece() {
				// got bad data
				return
			}

			pr.logger.Printf("Index: %d, Begin: %d (%v)", index, offset, offset/BlockLen)

			blockBuf := make([]byte, lengthPrefix-9)
			_, err = io.ReadFull(pr.peer.conn, blockBuf)
			if err != nil {
				return
			}

			block := TorrentBlock{index, offset, blockBuf}
			pr.peer.torrent.torrentBlockCH <- block
			//			pr.peer.torrent.setBlock(index, offset, blockBuf)
			pr.peer.requestsMX.Lock()
			pr.peer.requests--
			pr.peer.requestsMX.Unlock()
			go pr.peer.requestPieces()
		case Cancel:

			// index, begin, length
			payloadBuf := make([]byte, 12)

			_, err = pr.peer.conn.Read(payloadBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
		case Port:
			listenPortBuf := make([]byte, 2)
			_, err = pr.peer.conn.Read(listenPortBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
		case Extended:
			extendedIDBuf := make([]byte, 1)
			_, err = pr.peer.conn.Read(extendedIDBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}

			if extendedIDBuf[0] != uint8(0) {
				// Unsupported message type
				continue
			}

			payloadBuf := make([]byte, lengthPrefix-2)
			_, err = io.ReadFull(pr.peer.conn, payloadBuf)
			if err != nil {
				return
			}

			bencodeEnd := bytes.Index(payloadBuf, []byte("ee")) + 2
			bencode := payloadBuf[0:bencodeEnd]

			response, err := decodeMetadataRequest(bencode)
			if err != nil {
				return
			}

			if response.MsgType == 2 || response.MsgType == 0 { // reject || request
				continue
			}

			metadataPiece := payloadBuf[bencodeEnd:]

			pr.logger.Println(string(bencode))

			pr.peer.torrent.metadataPieceCH <- MetadataPiece{response.Piece, metadataPiece}

			if !pr.peer.torrent.hasMetadata {
				pr.peer.pw.sendMetadataRequest()
			}
		default:
			return
		}

	}
}
