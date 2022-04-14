package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// PeerReader reads from the peer's connection and parses the messages
type PeerReader struct {
	peer   *Peer
	conn   net.Conn
	logger *log.Logger
}

func newPeerReader(peer *Peer) *PeerReader {
	var pr PeerReader
	pr.conn = peer.conn
	pr.peer = peer
	pr.logger = log.New(peer.torrent.logFile, "[Peer Reader] "+pr.peer.ip+": ", log.Ltime|log.Lshortfile)
	return &pr
}

func (pr *PeerReader) run(wg *sync.WaitGroup) {
	defer func() {
		pr.peer.status = Dead
		pr.peer.pw.stop()
		wg.Done()
	}()

	for {
		// disconnect if we don't receive a KEEP ALIVE (or any message) for 2 minutes
		err := pr.conn.SetReadDeadline(time.Now().Add(time.Minute * time.Duration(2)))
		if err != nil {
			panic(err)
		}

		lengthPrefixBuf := make([]byte, 4)
		b, err := io.ReadFull(pr.peer.conn, lengthPrefixBuf)
		if err != nil {
			pr.logger.Println(err)
			pr.logger.Println(b)
			return
		}

		lengthPrefix := int(binary.BigEndian.Uint32(lengthPrefixBuf))

		if lengthPrefix == 0 {
			pr.logger.Println("Received KEEP ALIVE")
			if !pr.peer.choked {
				pr.peer.requestNewBlock()
			}
			continue
		}

		messageIDBuf := make([]byte, 1)
		_, err = pr.conn.Read(messageIDBuf)
		if err != nil {
			pr.logger.Println(err)
			return
		}

		messageID := int(messageIDBuf[0])

		pr.logger.Printf("Message received - Length: %d, Message_id: %d\n", lengthPrefix, messageID)

		switch int(messageID) {
		// no payload
		case Choke:
			pr.logger.Println("Received CHOKE")
			pr.peer.choked = true
			continue
		case Unchoke:
			pr.logger.Println("Received UNCHOKE")
			pr.peer.choked = false
			go pr.peer.requestNewBlock()
			continue
		case Interested:
			pr.logger.Println("Received INTERESTED")
			continue
		case NotInterested:
			pr.logger.Println("Received NOT INTERESTED")
			continue
		case Have:
			pr.logger.Println("Received HAVE")
			pieceIndexBuf := make([]byte, 4)
			_, err = pr.conn.Read(pieceIndexBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
		case Bitfield:
			pr.logger.Println("Received BITFIELD")
			bitfieldBuf := make([]byte, lengthPrefix-1)
			_, err = pr.conn.Read(bitfieldBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
			pr.peer.bitfield = bitfieldBuf
		case Request:
			pr.logger.Println("Received REQUEST")

			// index, begin, length
			payloadBuf := make([]byte, 12)

			_, err = pr.conn.Read(payloadBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
		case PIECE:
			pr.logger.Println("Received PIECE")

			if lengthPrefix > BlockLen+9 {
				pr.logger.Println("PIECE MESSAGE WAS TOO LONG")
				continue
			}
			indexBuf := make([]byte, 4)
			beginBuf := make([]byte, 4)

			_, err = pr.conn.Read(indexBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}

			_, err = pr.conn.Read(beginBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}

			index := int(binary.BigEndian.Uint32(indexBuf))
			begin := int(binary.BigEndian.Uint32(beginBuf))

			pr.logger.Printf("Index: %d, Begin: %d", index, begin)

			blockBuf := make([]byte, lengthPrefix-9)
			totalRead := 0
			for totalRead < lengthPrefix-9 {
				tempBuf := make([]byte, len(blockBuf)-totalRead)
				n, err := pr.conn.Read(tempBuf)
				if err != nil {
					pr.logger.Println(err)
					return
				}
				blockBufRemainder := blockBuf[totalRead+n:]
				blockBuf = append(blockBuf[0:totalRead], tempBuf[:n]...)
				blockBuf = append(blockBuf, blockBufRemainder...)
				pr.logger.Println(n)
				totalRead += n
			}

			pr.logger.Printf("Extended message info - Got %d bytes, expecting %d\n", totalRead, len(blockBuf))

			pr.peer.torrent.setBlock(index, begin, blockBuf)

			go pr.peer.requestNewBlock()
		case Cancel:
			pr.logger.Println("Received CANCEL")

			// index, begin, length
			payloadBuf := make([]byte, 12)

			_, err = pr.conn.Read(payloadBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
		case Port:
			pr.logger.Println("Received PORT")
			listenPortBuf := make([]byte, 2)
			_, err = pr.conn.Read(listenPortBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}
		case Extended:
			pr.logger.Println("Received EXTENDED")
			extendedIDBuf := make([]byte, 1)
			_, err = pr.conn.Read(extendedIDBuf)
			if err != nil {
				pr.logger.Println(err)
				return
			}

			if extendedIDBuf[0] != uint8(0) {
				pr.logger.Println("Received unsupported extended message")
				continue
			}

			payloadBuf := make([]byte, lengthPrefix-2)
			_, err = io.ReadFull(pr.conn, payloadBuf)
			if err != nil {
				return
			}

			bencodeEnd := bytes.Index(payloadBuf, []byte("ee")) + 2
			bencode := payloadBuf[0:bencodeEnd]

			response := decodeMetadataRequest(bencode)

			if response.MsgType == 2 { // reject
				pr.logger.Println("Peer does not have requested metadata piece")
				continue
			}

			metadataPiece := payloadBuf[bencodeEnd:]

			// ensure metadata is built once and only once
			pr.peer.torrent.metadataMx.Lock()

			beforeAppend := pr.peer.torrent.hasAllMetadata()

			fmt.Println(response.Piece)

			err = pr.peer.torrent.setMetadataPiece(response.Piece, metadataPiece)
			if err != nil {
				fmt.Println(err)
			}

			if beforeAppend != pr.peer.torrent.hasAllMetadata() { // true iff we inserted the last piece
				err = pr.peer.torrent.buildMetadataFile()
				if err != nil {
					pr.logger.Println(err)
					return
				}
				err = pr.peer.torrent.parseMetadataFile()
				if err != nil {
					pr.logger.Println(err)
					return
				}
			}

			pr.peer.torrent.metadataMx.Unlock()
			pr.peer.pw.sendMetadataRequest()
		default:
			pr.logger.Println("Received bad message_id")
		}

	}
}
