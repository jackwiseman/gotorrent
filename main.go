package main

import (
	"encoding/binary"
	"crypto/rand"
	"net"
	"fmt"
	"time"
)

type Connection_Request struct {
	connection_id uint64
	action uint32
	transaction_id uint32
}

func (cr *Connection_Request) encode() ([]byte) {
	packet := make([]byte, 16)
	binary.BigEndian.PutUint64(packet[0:], uint64(41727101980))
	binary.BigEndian.PutUint32(packet[8:], uint32(0))
	binary.BigEndian.PutUint32(packet[12:], cr.transaction_id)
	return packet
}

func create_connection_id() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return uint32(binary.BigEndian.Uint32(b[:])), nil
}

func main() {
	var torrent Torrent
	torrent.magnet_link = link

	torrent.parse_magnet_link()
	torrent.print_info()

	id, err := create_connection_id()
	if err != nil {
		panic(err)
	}

	for i := 0; i < 15; i++ {
		var conn_request Connection_Request
		conn_request.transaction_id = id
		packet := conn_request.encode()
		conn, err := net.DialTimeout("udp", torrent.trackers[1][6:], 15*time.Second)
		if err != nil {
			panic(err)
		}
		if _, err = conn.Write(packet); err != nil {
			panic(err)
		}

		p := make([]byte, 16)
		nn, err := conn.Read(p)
		if err != nil {
			panic(err)
		}
		fmt.Printf(string(p[:nn]))
		fmt.Printf("Retrying")
	}
}
