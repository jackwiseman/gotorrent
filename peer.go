package main

import (
	"strconv"
	"strings"
	"net"
	"time"
	"errors"
	"io"
	"fmt"
	"math"
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
	if peer.conn != nil { // need to look into this, also keeping it open
		peer.conn.Close()
	}
}

func (peer *Peer) perform_handshake (torrent *Torrent) (error) {
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
		//fmt.Println("Peer supports extended messaging, performing extended message handshake")
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
		result := decode_handshake(buf[6:6+message_length])
		peer.set_extensions(result.M)

		if result.Metadata_size != 0 {
			torrent.metadata_size = result.Metadata_size
			torrent.metadata_pieces = int(math.Round(float64(torrent.metadata_size)/float64(16384) + 1.0))
			//	fmt.Println("Metadata size:")
			//	fmt.Println(torrent.metadata_size)
			//	fmt.Println("Pieces: ")
			//	fmt.Println(int(math.Round(float64(torrent.metadata_size)/float64(16384) + 1.0))) // this is not correct
		}
	} else {
		//fmt.Println("Peer does not support extended messaging")
	}
	return nil
}

func (peer *Peer) request_metadata(ch chan Metadata_Piece, pieces *[]int) /*(bool, []byte)*/ {
	for need_piece(*pieces) {
		curr_piece := get_rand_piece(*pieces)

		//	build the request packet
		//	<len><id-20><extension-id><payload>
		payload := []byte(encode_metadata_request(curr_piece))
		msg_len := len(payload) + 2
		packet := make([]byte, msg_len + 4) //payload + 2 * uint8 and 1 * uint32
		binary.BigEndian.PutUint32(packet[0:], uint32(msg_len))
		copy(packet[4:], []byte([]uint8{uint8(20)}))
		metadata_id := uint8(peer.extensions["ut_metadata"])
		copy(packet[5:], []byte([]uint8{metadata_id}))
		copy(packet[6:], payload)

		bytes_written, err := peer.conn.Write(packet)
		if err != nil || bytes_written < len(packet) {
			peer.is_alive = false
			fmt.Println(errors.New("Error: unable to write to peer, here is the error dump:"))
			fmt.Println(peer)
			continue
		}

		buf := make([]byte, 0, 19968) // big buffer
		tmp := make([]byte, 256)     // using small tmo buffer for demonstrating

		read_start := time.Now().Add(time.Second * 15)
		var expected int

		//	peer.conn.SetReadDeadline(time.Second * time.Duration(5))

		for i := 0; i < 78; i++ {
			n, err := peer.conn.Read(tmp)
			if err != nil {
				if err != io.EOF {
					fmt.Println("read error:", err)
				}
				//return false, nil
				break
			}

			buf = append(buf, tmp[:n]...)
//			fmt.Println("got", n, "bytes from " + peer.ip)

			if i == 0 && len(tmp) == 256 {
				response_bencode_string := string(buf[0:100]) // only issue is if the 'ee' isnt found here
				start_index := strings.Index(response_bencode_string, "d")
				if start_index == -1 {
					continue
				}
				fmt.Println(response_bencode_string)
				expected = int(binary.BigEndian.Uint32(buf[start_index - 6:]))
			}
			if i > 0 && len(buf) > 256 {
//				fmt.Println(len(buf))
//				fmt.Println(expected)
				if len(buf) >= expected {
					break
				}
			}
			if time.Now().After(read_start) {
				break
			}
		}
		//fmt.Printf("? " + strconv.Itoa(len(buf)))
		if len(buf) > 99 {
			response_len := binary.BigEndian.Uint32(buf[0:])
			//	response_id := buf[4]
			//	response_extension := buf[5]
			response_bencode_string := string(buf[0:100]) // only issue is if the 'ee' isnt found here
			start_index := strings.Index(response_bencode_string, "d")
			index := strings.Index(response_bencode_string, "ee")
			if index == -1 || start_index == -1 {
				continue
			}
			response_len = binary.BigEndian.Uint32(buf[start_index - 6:])
			//bencode := response_bencode_string[start_index:index+2] // there's also a chance they don't have it
			//fmt.Println("Bencode info: " + bencode)
			fmt.Println("This piece is " + strconv.Itoa(len(buf[index+2:response_len+4])) + " bytes")
			ch <- Metadata_Piece{curr_piece, buf[index+2:response_len+4]} // add 4 to the response len because it does not include the initial uint32
			continue
		}
	}
}
