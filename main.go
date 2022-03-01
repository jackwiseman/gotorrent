package main

import (
	"encoding/binary"
	"crypto/rand"
	"net"
	"fmt"
	"time"
	"math"
)

type Connection_Request struct {
	connection_id uint64
	action uint32
	transaction_id uint32
}

type Connection_Response struct {
	action uint32
	transaction_id uint32
	connection_id uint64
}

func (conn_req *Connection_Request) encode() ([]byte) {
	packet := make([]byte, 16)
	binary.BigEndian.PutUint64(packet[0:], 0x41727101980)
	binary.BigEndian.PutUint32(packet[8:], 0)
	binary.BigEndian.PutUint32(packet[12:], conn_req.transaction_id)
	return packet
}

func (conn_resp *Connection_Response) decode(packet []byte) {
	conn_resp.action = binary.BigEndian.Uint32(packet[0:])
	conn_resp.transaction_id = binary.BigEndian.Uint32(packet[4:])
	conn_resp.connection_id = binary.BigEndian.Uint64(packet[8:])
}

// Return a new, random 32-bit integer
func get_transaction_id() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return uint32(binary.BigEndian.Uint32(b[:])), nil
}

func get_connection_id(conn net.Conn) (uint64) {
	timeout := time.Second * 15
	got_response := false
	retries := 8
	var response Connection_Response

	for i := 0; i <= retries; i++ {
		// Create a new Connection_Request with a new transaction_id
		transaction_id, err := get_transaction_id(); if err != nil {
			panic(err)
		}
		var conn_request Connection_Request
		conn_request.transaction_id = transaction_id

		// Encode Connection_Request as a byte slice
		packet := conn_request.encode()

		conn.SetWriteDeadline(time.Now().Add(timeout))

		fmt.Println("Writing...")
		bytes_written, err := conn.Write(packet); if err != nil {
			panic(err)
		}
		if bytes_written < len(packet) {
			panic("Error: did not write 16 bytes")
		}

		buf := make([]byte, 16)
		// Per BitTorrent.org specificiations, "If a response is not recieved after 15 * 2 ^ n seconds, client should retransmit, where n increases to 8 from 0
		conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15 * math.Pow(2,float64(i))))))

		fmt.Println("Reading...")
		bytes_read, err := conn.Read(buf)
		if err != nil {
			if i == retries {
				panic(err)
			}
		} else {
			if bytes_read != 16 {
				fmt.Println("Error: did not read 16 bytes")
			} else {
				fmt.Println("Got something...")
				response.decode(buf)
				if response.transaction_id == conn_request.transaction_id {
					got_response = true
					break
				}
			}
		}
		fmt.Println("Retrying...")
	}

	if !got_response {
		return 0
	}
	// Mainly for debug
	fmt.Println("Action: ", response.action)
	fmt.Println("Transaction ID: ", response.transaction_id)
	fmt.Println("Connection ID: ", response.connection_id)

	return response.connection_id
}

func scrape(conn net.Conn, torrent Torrent, cid uint64) {

	timeout := time.Second * 15
	retries := 8

	for i := 0; i <= retries; i++ {
		transaction_id, err := get_transaction_id(); if err != nil {
			panic(err)
		}

		infohash := []byte(torrent.hash) // doublecheck
		packet_len := 16 + (20)

		packet := make([]byte, 16 + (20 ))
		binary.BigEndian.PutUint64(packet[0:], cid)
		binary.BigEndian.PutUint32(packet[8:], 2) // action 2 = scrape
		binary.BigEndian.PutUint32(packet[12:], transaction_id)
		
		copy(packet[16:], infohash)

		conn.SetWriteDeadline(time.Now().Add(timeout))

		fmt.Println("Writing...")
		bytes_written, err := conn.Write(packet); if err != nil {
			panic(err)
		}
		if bytes_written < len(packet) {
			panic("Error: did not write scrape request")
		}

		buf := make([]byte, 8 + (12 * packet_len))
		// Per BitTorrent.org specificiations, "If a response is not recieved after 15 * 2 ^ n seconds, client should retransmit, where n increases to 8 from 0
		conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15 * math.Pow(2,float64(i))))))

		fmt.Println("Reading...")
		bytes_read, err := conn.Read(buf)
		if err != nil {
			if i == retries {
				panic(err)
			}
		} else {
			if bytes_read >= 8 + (12 * len(infohash)) {
				fmt.Printf("Error: did not read correct # of bytes")
			} else {
				fmt.Println("Got something...")

				action := binary.BigEndian.Uint32(buf[0:])
				recieved_tid := binary.BigEndian.Uint32(buf[4:])
				seeders := binary.BigEndian.Uint32(buf[8:])
				completed := binary.BigEndian.Uint32(buf[12:])
				leechers := binary.BigEndian.Uint32(buf[16:])

				fmt.Printf("Action: ", action)
				fmt.Printf("Recieved Transaction ID: ", recieved_tid)
				fmt.Printf("Seeders: ", seeders)
				fmt.Printf("Completed: ", completed)
				fmt.Printf("Leechers: ", leechers)
				break

				}
			}
		}
		fmt.Println("Retrying...")
}

func main() {
	link := "magnet:?xt=urn:btih:e0974c84449826e34d8cc96c943cba2af18ab514&tr=https%3A%2F%2Facademictorrents.com%2Fannounce.php&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce"
	var torrent Torrent
	torrent.magnet_link = link

	torrent.parse_magnet_link()
	torrent.print_info()

	timeout := time.Second * 15

	fmt.Println("Connecting...")
	tracker := "tracker.opentrackr.org:1337"
	//conn, err := net.DialTimeout("udp", torrent.trackers[1][6:], timeout)
	conn, err := net.DialTimeout("udp", tracker, timeout)
	if err != nil {
		panic(err)
	}

	cid := get_connection_id(conn)
	scrape(conn, torrent, cid)
	fmt.Println(cid)
}
