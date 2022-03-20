package main

import (
	"bufio"
	"strconv"
	"sync"
//	"strings"
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

	// wrapped io.Reader/io.Writer interfaces
//	pw Peer_Writer
	pr *Peer_Reader
}

func (peer *Peer) set_extensions(extensions map[string]int) {
	peer.extensions = extensions
}

func new_peer(ip string, port string) (*Peer) {
	var peer Peer

	peer.ip = ip
	peer.port = port

	return &peer
	//return &Peer{ip, port, nil, false, false, nil}
}

func (peer *Peer) run(torrent *Torrent) {
	defer peer.disconnect()

	fmt.Println("Connecting...")
	err := peer.connect()
	if err != nil {
		fmt.Println("Bad peer")
		return
	}
	fmt.Println("Handshaking...")
	peer.perform_handshake(torrent)
	if err != nil {
		fmt.Println("Bad handshake")
		fmt.Println(err)
		return
	}
//	fmt.Println(peer.conn)
	fmt.Println("Running Peer_Reader...")
	peer.pr.run()
}

// Connect to peer via TCP and create a peer_reader over connection
func (peer *Peer) connect() (error) {
	timeout := time.Second * 3
	conn, err := net.DialTimeout("tcp", peer.ip + ":" + peer.port, timeout)
	if err != nil {
		peer.is_alive = false
		return err
	} else {
		peer.conn = conn
		peer.pr = new_peer_reader(peer)
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
	// pstrlen := 19
	// pstr := "BitTorrent protocol"

	// handshake := make([]byte, 49 + pstrlen)
	// copy(handshake[0:], []byte([]uint8{uint8(pstrlen)}))
	// copy(handshake[1:], []byte(pstr))
	// handshake[25] = 16 // we need to demonstrate that this client supports extended messages, so we set the 20th bit (big endian) to 1
	// copy(handshake[28:], torrent.info_hash)
	// peer_id := "GoLangTorrent_v0.0.1" // TODO: generate a random peer_id?
	// copy(handshake[48:], []byte(peer_id))

	// // Write to peer
	// bytes_written, err := peer.conn.Write(handshake)
	// if err != nil || bytes_written < 49 + pstrlen {
	// 	peer.is_alive = false
	// 	return errors.New("Error: unable to write to peer")
	// }

	outgoing_handshake := get_handshake_message(torrent)
	_, err := peer.conn.Write(outgoing_handshake)
	if err != nil {
		peer.is_alive = false
		return errors.New("Error: unable to write to peer")
	}

	// Read from peer
	pstrlen_buf := make([]byte, 1)
	_, err = peer.conn.Read(pstrlen_buf)
	if err != nil {
		peer.is_alive = false
		return errors.New("Error: could not read from peer")
	}

	pstrlen := int(pstrlen_buf[0])
	fmt.Println(pstrlen)


	buf := make([]byte, 48 + pstrlen)
	_, err = peer.conn.Read(buf)
	if err != nil {
		peer.is_alive = false
		return errors.New("Error: could not read from peer")
	}

	fmt.Println(string(buf[0:pstrlen]))

	
	// if the peer utilizes extended messages (most likely), we next need to send an extended handshake, mostly just for getting metadata
	if buf[24] & 0x10 == 16 {
		peer.uses_extended = true
		//fmt.Println("Peer supports extended messaging, performing extended message handshake")
		outgoing_extended_handshake := get_extended_handshake_message()

		bytes_written, err := peer.conn.Write(outgoing_extended_handshake)
		if err != nil || bytes_written < len(outgoing_extended_handshake) {
			peer.is_alive = false
			return errors.New("Error: unable to write to peer in extended handshake")
		}

		length_prefix_buf := make([]byte, 4)
		_, err = peer.conn.Read(length_prefix_buf)
		if err != nil {
			return err
		}
		
		length_prefix := binary.BigEndian.Uint32(length_prefix_buf[0:])

		buf = make([]byte, int(length_prefix))
		_, err = peer.conn.Read(buf)
		if err != nil {
			return err
		}

		result := decode_handshake(buf[2:])
		peer.set_extensions(result.M)

		if result.Metadata_size != 0 && torrent.metadata_size == 0 { // make sure they attached metadata size, also no reason to overwrite if we already set
			torrent.metadata_size = result.Metadata_size
			torrent.metadata_pieces = int(math.Round(float64(torrent.metadata_size)/float64(16384) + 1.0))
			torrent.metadata_raw = make([]byte, result.Metadata_size)
			//	fmt.Println("Metadata size:")
			//	fmt.Println(torrent.metadata_size)
			//	fmt.Println("Pieces: ")
			//	fmt.Println(int(math.Round(float64(torrent.metadata_size)/float64(16384) + 1.0))) // this is not correct
		}
	} 
	return nil
}

func (peer *Peer) request_metadata(metadata_raw *[]byte, pieces *[]int, wg *sync.WaitGroup, metadata_size int) /*(bool, []byte)*/ {
	defer wg.Done()

	var attempts int // we only want to try reading a few times, so increment this 10 times and then exit

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
			// try reconnecting + handshaking ?
			fmt.Println(peer)
			return
		}


		// Read metadata piece
		var length_prefix uint32
		var message_id uint8
		var extension_id uint8
		
		// Set a read deadline of a few seconds
		timeout := 5 * time.Second
		peer.conn.SetReadDeadline(time.Now().Add(timeout))

		peer_reader := bufio.NewReaderSize(peer.conn, 6) // len + id + extension id

		err = binary.Read(peer_reader, binary.BigEndian, &length_prefix)
		if err != nil {
			fmt.Println("Error reading length_prefix")
			return
		}

		err = binary.Read(peer_reader, binary.BigEndian, &message_id)
		if err != nil {
			fmt.Println("Error reading message_id")
			fmt.Println(err)
			return
		}

		
		err = binary.Read(peer_reader, binary.BigEndian, &extension_id)
		if err != nil {
			fmt.Println("Error reading extension_id")
			fmt.Println(err)
			return
		}

		/*fmt.Println("----")
		fmt.Println(peer.ip)
		fmt.Println(length_prefix)
		fmt.Println(message_id)
		fmt.Println(extension_id)
		fmt.Println("----")
		return*/

		
		if int(message_id) != 20 || int(extension_id) != 0 {
			if attempts > 10 {
				fmt.Println("Too many attempts :(")
				return
			}
			attempts++
			continue
		}

		// unless this is the last peer, the bencoded info must be length_prefix - (1024 * 16) - 2 bytes
		var bencode_length int
		if curr_piece < len(*pieces) - 1 {
			bencode_length = int(length_prefix) - (1024 * 16) - 2
		} else {
			bencode_length = int(length_prefix) - (metadata_size % (1024 * 16)) - 2
		}

		bencode_buf := make([]byte, bencode_length)
		_, err = io.ReadFull(peer_reader, bencode_buf)
		if err != nil {
			fmt.Println("Error reading bencode")
			fmt.Println(err)
			return
		}


		fmt.Println("----")
		fmt.Println(length_prefix)
		fmt.Println(message_id)
		fmt.Println(extension_id)
		fmt.Println(string(bencode_buf))
		fmt.Println("----")

		fmt.Println("Received piece " + strconv.Itoa(curr_piece) + " from " + peer.ip)

		metadata_piece := make([]byte, int(length_prefix) - bencode_length - 2)
		_, err = io.ReadFull(peer_reader, metadata_piece)
		if err != nil {
			fmt.Println(err)
			return
		}

		temp := (*metadata_raw)[curr_piece * (1024 * 16) + len(metadata_piece):]
		*metadata_raw = append((*metadata_raw)[0:curr_piece * 16384], metadata_piece...)
		*metadata_raw = append(*metadata_raw, temp...)
		(*pieces)[curr_piece] = 1
		attempts = 0
		return
	}
}
