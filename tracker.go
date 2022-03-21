package main

import (
	"net"
	"time"
	"errors"
	"encoding/binary"
	"math"
	"fmt"
	"strconv"
)

type Tracker struct {
	link string
	conn net.Conn
	timeout time.Duration // default is 15 seconds
	connection_id uint64
	expires time.Time
	retries int
}

// return a new tracker from a string representing the link
func new_tracker(link string) (*Tracker) {
	return &Tracker{link: link, timeout: 15 * time.Second, retries: 1}
}

func (tracker *Tracker) set_timeout(seconds int) {
	tracker.timeout = time.Second * time.Duration(seconds)
}

// for now, only works for udp links, which seem to be standard
func (tracker *Tracker) connect() (error) {
	conn, err := net.DialTimeout("udp", tracker.link, tracker.timeout)
	if err != nil {
		return(err)
	}
	tracker.conn = conn
	return nil
}

func (tracker *Tracker) disconnect() {
	tracker.conn.Close()
}

// the first step in getting peers from the tracker is getting a connection_id, which is valid for 2 minutes
func (tracker *Tracker) set_connection_id() (error) {
	for i := 0; i <= tracker.retries; i++ {
		// Create a new Connection_Request with a new transaction_id
		transaction_id, err := get_transaction_id(); if err != nil {
			return(err)
		}

		// Serialize a 16 byte request for a connection id where
		// Offset	Name		Value
		// 0		protocol_id	0x41727101980 - magic constant
		// 8		action 		0 - connect
		// 12		transaction_id 	which is randomly generated per request
		packet := make([]byte, 16)
		binary.BigEndian.PutUint64(packet[0:], 0x41727101980)
		binary.BigEndian.PutUint32(packet[8:], 0)
		binary.BigEndian.PutUint32(packet[12:], transaction_id)

		// Set a write deadline of 15 seconds and connect
		tracker.conn.SetWriteDeadline(time.Now().Add(tracker.timeout))
		bytes_written, err := tracker.conn.Write(packet); if err != nil {
			return(err)
		}
		if bytes_written < len(packet) {
			return(errors.New("Error: did not write 16 bytes"))
		}

		// Per BitTorrent.org specificiations, "If a response is not recieved after 15 * 2 ^ n seconds, client should retransmit, where n increases to 8 from 0
		tracker.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15 * math.Pow(2,float64(i))))))

		// Expecting a 16 byte response where
		// Offset	Name		Value
		// 0		action		0 - connect
		// 4		transaction_id	should be same that was sent	
		// 8		connection_id		
		buf := make([]byte, 16)

		bytes_read, err := tracker.conn.Read(buf)
		if err != nil && i == tracker.retries {
			return(err)
		} else {
			if bytes_read != 16 {
				return errors.New("Error: did not read 16 bytes") // try again?
			} else {
				// Make sure the response has the connect action and the same transaction id that we sent
				if binary.BigEndian.Uint32(buf[0:]) == 0 && binary.BigEndian.Uint32(buf[4:]) == transaction_id {
					tracker.connection_id = binary.BigEndian.Uint64(buf[8:])
					return nil
				} else {
					return errors.New("Error: received bad connect data from tracker")
				}
			}
		}
	}
	return nil
}

// not really necessary at the moment, maybe use strictly to print info?
// only really interresting for # of seeders
func (tracker *Tracker) scrape(torrent *Torrent) (error) {
	for i := 0; i <= tracker.retries; i++ {
		transaction_id, err := get_transaction_id(); if err != nil {
			return(err)
		}

		packet_len := 16 + len(torrent.info_hash) // this will always be 20, but will stay here for future scalability

		packet := make([]byte, 16 + (20 ))
		binary.BigEndian.PutUint64(packet[0:], tracker.connection_id)
		binary.BigEndian.PutUint32(packet[8:], 2) // action 2 = scrape
		binary.BigEndian.PutUint32(packet[12:], transaction_id)

		copy(packet[16:], torrent.info_hash)

		tracker.conn.SetWriteDeadline(time.Now().Add(tracker.timeout))

		bytes_written, err := tracker.conn.Write(packet); if err != nil {
			return(err)
		}
		if bytes_written < len(packet) {
			return errors.New("Error: did not write scrape request")
		}

		buf := make([]byte, 8 + (12 * packet_len))
		// Per BitTorrent.org specificiations, "If a response is not recieved after 15 * 2 ^ n seconds, client should retransmit, where n increases to 8 from 0
		tracker.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15 * math.Pow(2,float64(i))))))

		bytes_read, err := tracker.conn.Read(buf)
		if err != nil {
			if i == tracker.retries {
				return(err)
			}
		} else {
			if bytes_read <= 16 {
				fmt.Println("Error: did not read correct # of bytes")
				fmt.Printf("\nGot %d bytes", bytes_read)
				fmt.Println("Retrying...")

			} else {
				fmt.Println("Got something...")

				action := binary.BigEndian.Uint32(buf[0:])
				recieved_tid := binary.BigEndian.Uint32(buf[4:])
				seeders := binary.BigEndian.Uint32(buf[8:])
				completed := binary.BigEndian.Uint32(buf[12:])
				leechers := binary.BigEndian.Uint32(buf[16:])

				fmt.Println("\n-- Scrape Results --")
				fmt.Printf("Action: %d", action)
				fmt.Printf("\nRecieved Transaction ID: %d", recieved_tid)
				fmt.Printf("\nSeeders: %d", seeders)
				fmt.Printf("\nCompleted: %d", completed)
				fmt.Printf("\nLeechers: %d\n", leechers)
				break

			}
		}
	}
	return nil
}

// announce to a tracker requesting num_peers ip addresses
// returns # of seeders
func (tracker *Tracker) announce(torrent *Torrent, num_want int) (int, error) {
	for i := 0; i <= tracker.retries; i++ {
		transaction_id, err := get_transaction_id(); if err != nil {
			return 0, err
		}

		// Create announce packet
		packet := make([]byte, 98)
		// connection_id
		binary.BigEndian.PutUint64(packet[0:], tracker.connection_id)
		// action
		binary.BigEndian.PutUint32(packet[8:], 1)
		// transaction_id
		binary.BigEndian.PutUint32(packet[12:], transaction_id)
		// info_hash
		copy(packet[16:], torrent.info_hash)
		// peer_id (20 bytes)
		peer_id := "GoLangTorrent_v0.0.1" // should be randomly set
		copy(packet[36:], []byte(peer_id))
		// downloaded
		binary.BigEndian.PutUint64(packet[56:], 0)
		// left
		binary.BigEndian.PutUint64(packet[64:], 0)
		// uploaded
		binary.BigEndian.PutUint64(packet[72:], 0)
		// event
		binary.BigEndian.PutUint32(packet[80:], 0)
		// ip_address
		binary.BigEndian.PutUint32(packet[84:], 0)
		// key
		binary.BigEndian.PutUint32(packet[88:], 0)
		// num_want
		binary.BigEndian.PutUint32(packet[92:], uint32(num_want))
		// port
		binary.BigEndian.PutUint16(packet[96:], 6881)

		// Set timeout
		//tracker.conn.SetWriteDeadline(time.Now().Add(tracker.timeout))

		bytes_written, err := tracker.conn.Write(packet)
		if err != nil || bytes_written < len(packet) {
			return 0, errors.New("Error: could not write announce request")
		}

		buf := make([]byte, 20 + (6 * num_want))
		tracker.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15 * math.Pow(2,float64(i)))))) // BEP 15 - If a response is not received after 15 * 2 ^ n seconds, the client should retransmit the request, where n starts at 0 and is increased up to 8 (3840 seconds) after every retransmission

		bytes_read, err := tracker.conn.Read(buf)
		if (bytes_read < 20 || err != nil) {
			if i >= tracker.retries {
				return 0, err
			}
			continue
		}

		seeders := int(binary.BigEndian.Uint32(buf[16:]))
		for j := 0; j < int(math.Min(float64(num_want), float64(seeders))); j++ {
			ip_address_raw := binary.BigEndian.Uint32(buf[20 + (6 * j):])
			port := binary.BigEndian.Uint16(buf[24 + (6 * j):])

			// convert ip_address into a string representation
			ip_address := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip_address, ip_address_raw)

			torrent.peers = append(torrent.peers, *new_peer(ip_address.String(), strconv.Itoa(int(port)), torrent))
		}
		return seeders, nil
	}
	return 0, errors.New("Tracker timed out")
}
