package main

import (
	"net"
	"time"
	"errors"
	"io"
	"fmt"
	"encoding/binary"
)

type Peer struct {
	// use defaults in the future?
	ip string
	port string
	conn net.Conn
	uses_extended bool // false by default
	is_alive bool // assume yes until a handshake fails
	extensions map[string]int
}

func (peer *Peer) set_extensions(extensions map[string]int) {
	peer.extensions = extensions
}

func new_peer(ip string, port string) (*Peer) {
	return &Peer{ip, port, nil, false, false, nil}
}

// Connect to peer via TCP, returning false otherwise
// TODO: return error instead?
func (peer *Peer) connect() (error) {
	timeout := time.Second * 3
	conn, err := net.DialTimeout("tcp", peer.ip + ":" + peer.port, timeout)
	if err != nil {
		peer.is_alive = false
		return err
	} else {
		peer.conn = conn
		return nil
	}
}

func (peer *Peer) disconnect() {
	peer.conn.Close()
}

func (peer *Peer) perform_handshake (torrent Torrent) (error) {
	if peer.conn == nil {
		return errors.New("Error: peer's connection is nil")
	}

	// Create handshake message
	pstrlen := 19
	pstr := "BitTorrent protocol"

	handshake := make([]byte, 49 + pstrlen)
	copy(handshake[0:], []byte([]uint8{uint8(pstrlen)}))
	copy(handshake[1:], []byte(pstr))
	handshake[25] = 16 // we need to demonstrate that this client supports extended messages, so we set the 20th bit (big endian) to 1
	copy(handshake[28:], torrent.info_hash)
	peer_id := "GoLangTorrent_v0.0.1" // TODO: generate a random peer_id?
	copy(handshake[48:], []byte(peer_id))

	// Write to peer
	bytes_written, err := peer.conn.Write(handshake)
	if err != nil || bytes_written < 49 + pstrlen {
		peer.is_alive = false
		return errors.New("Error: unable to write to peer")
	}

	// Read from peer
	buf := make([]byte, 49 + pstrlen) // should probably be longer, but for now I'll assume all will use the default pstrlen = 19 for 'BitTorrent protocol'
	_, err = peer.conn.Read(buf)

	if err != nil {
		peer.is_alive = false
		return errors.New("Error: unable to read from peer")
	}

	// if the peer utilizes extended messages (most likely), we next need to send an extended handshake, mostly just for getting metadata
	if buf[25] & 0x10 == 16 {
		peer.uses_extended = true
		fmt.Println("Peer supports extended messaging, performing extended message handshake")
		packet := get_handshake_message()

		bytes_written, err := peer.conn.Write(packet)
		if err != nil || bytes_written < len(packet) {
			peer.is_alive = false
			return errors.New("Error: unable to write to peer in extended handshake")
		}

		buf = make([]byte, 2048) // we don't know how many different extensions the peer's client will have
		_, err = peer.conn.Read(buf)
		if err != io.EOF && err != nil {
			return(err)
		}

		message_length := binary.BigEndian.Uint32(buf[0:])
		//bittorrent_message_id := buf[4]
		//extended_message_id := buf[5]
		decode_handshake(buf[6:6+message_length])
	} else {
		fmt.Println("Peer does not support extended messaging")
	}
	return nil
}
