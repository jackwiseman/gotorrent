package main

import (
	"encoding/binary"
	"crypto/rand"
	"net"
	"fmt"
	"time"
	"math"
	"strconv"
//	"io"
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

		packet_len := 16 + len(torrent.info_hash) // this will always be 20, but will stay here for future scalability

		packet := make([]byte, 16 + (20 ))
		binary.BigEndian.PutUint64(packet[0:], cid)
		binary.BigEndian.PutUint32(packet[8:], 2) // action 2 = scrape
		binary.BigEndian.PutUint32(packet[12:], transaction_id)

		copy(packet[16:], torrent.info_hash)

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
}

// via BEP_15
// TODO: Error handling for no peers found
func announce(conn net.Conn, torrent Torrent, cid uint64) ([][]string) {
	timeout := time.Second * 15
	retries := 8
	peers := make([][]string, 0) // likely should not always be a string

	for i := 0; i <= retries; i++ {
		transaction_id, err := get_transaction_id(); if err != nil {
			panic(err)
		}

		num_want := 10

		// Create announce packet
		packet := make([]byte, 98)
		// connection_id
		binary.BigEndian.PutUint64(packet[0:], cid)
		// action
		binary.BigEndian.PutUint32(packet[8:], 1)
		// transaction_id
		binary.BigEndian.PutUint32(packet[12:], transaction_id)
		// info_hash
		copy(packet[16:], torrent.info_hash)
		// peer_id (20 bytes)
		peer_id := "GoLangTorrent_v0.0.1"
		copy(packet[36:], []byte(peer_id)) // not sure if this will even be accepted
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
		conn.SetWriteDeadline(time.Now().Add(timeout))

		fmt.Println("Writing...")
		bytes_written, err := conn.Write(packet); if err != nil {
			panic(err)
		}
		if bytes_written < len(packet) {
			panic("Error: did not write announce request")
		}

		buf := make([]byte, 20 + (6 * num_want))
		// Per BitTorrent.org specificiations, "If a response is not recieved after 15 * 2 ^ n seconds, client should retransmit, where n increases to 8 from 0
		conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15 * math.Pow(2,float64(i))))))

		fmt.Println("Reading...")
		bytes_read, err := conn.Read(buf)
		if err != nil {
			if i == retries {
				panic(err)
			}
		} else {
			if bytes_read < 20 + (6 * num_want) {
				fmt.Println("Error: did not read correct # of bytes")
				fmt.Printf("\nGot %d bytes", bytes_read)
				fmt.Println("Retrying...")

			} else {
				fmt.Println("Got something...")

				action := binary.BigEndian.Uint32(buf[0:])
				recieved_tid := binary.BigEndian.Uint32(buf[4:])
				interval := binary.BigEndian.Uint32(buf[8:])
				leechers := binary.BigEndian.Uint32(buf[12:])
				seeders := binary.BigEndian.Uint32(buf[16:])

				for j := 0; j < num_want; j++ {
					ip_address_raw := binary.BigEndian.Uint32(buf[20 + (6 * j):])
					port := binary.BigEndian.Uint16(buf[24 + (6 * j):])

					// convert ip_address into a string representation
					ip_address := make(net.IP, 4)
					binary.BigEndian.PutUint32(ip_address, ip_address_raw)

					new_peer := make([]string, 0)
					new_peer = append(new_peer, ip_address.String(), strconv.Itoa(int(port)))
					peers = append(peers, new_peer)
				}

				fmt.Println("\n-- Announce Results --")
				fmt.Printf("Action: %d", action)
				fmt.Printf("\nRecieved Transaction ID: %d", recieved_tid)
				fmt.Printf("\nInterval: %d", interval)
				fmt.Printf("\nLeechers: %d\n", leechers)
				fmt.Printf("\nSeeders: %d", seeders)
				fmt.Printf("\nPeers: ")
				fmt.Println(peers)
				break
			}
		}
	}
	return peers
}

func handshake(peer []string, torrent Torrent) {
	timeout := 15 * time.Second

	fmt.Printf("Performing handshake to %s:%s\n", peer[0], peer[1])
	fmt.Println("Connecting...")
	conn, err := net.DialTimeout("tcp", peer[0] + ":" + peer[1], timeout)
	if err != nil {
		panic(err)
	}

	// Create handshake message
	pstrlen := 19
	pstr := "BitTorrent protocol"

	handshake := make([]byte, 49 + pstrlen)
	copy(handshake[0:], []byte([]uint8{uint8(pstrlen)}))
	copy(handshake[1:], []byte(pstr))
	copy(handshake[28:], torrent.info_hash)
	peer_id := "GoLangTorrent_v0.0.1"
	copy(handshake[48:], []byte(peer_id))

	fmt.Println("Writing...")
	fmt.Println(handshake)
	bytes_written, err := conn.Write(handshake); if err != nil {
		panic(err)
	}
	if bytes_written < 49 + pstrlen {
		panic("Error: did not write handshake")
	}

	conn.SetReadDeadline(time.Now().Add(timeout))

	fmt.Println("Reading...")

	buf := make([]byte, 49 + pstrlen) // should probably be longer, but for now I'll assume all will use the default pstrlen = 19 for 'BitTorrent protocol'
	_, err = conn.Read(buf)
	if err != nil {
		panic(err)
	}

	conn.Close()
	fmt.Println(string(buf[1:20])) // should be 'BitTorrent protocol'
	fmt.Println(string(buf[48:])) // should be peer id
}

func main() {
	link := "magnet:?xt=urn:btih:bdc0bb1499b1992a5488b4bbcfc9288c30793c08&tr=https%3A%2F%2Facademictorrents.com%2Fannounce.php&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce"
	var torrent Torrent
	torrent.magnet_link = link

	torrent.parse_magnet_link()
	torrent.print_info()

//	timeout := time.Second * 15

//	fmt.Println("Connecting...")

//	tracker := "tracker.opentrackr.org:1337"
	//conn, err := net.DialTimeout("udp", torrent.trackers[1][6:], timeout)
	//conn, err := net.DialTimeout("udp", tracker, timeout)
	//if err != nil {
//		panic(err)
	//}

	//cid := get_connection_id(conn) // needs to be regenerated every 2 minutes
	//scrape(conn, torrent, cid)
	//peers := announce(conn, torrent, cid)

	// close connection to tracker
//	conn.Close()

	// now that we have our list of peers, lets perform a handshake with one
	handshake(torrent)

}
