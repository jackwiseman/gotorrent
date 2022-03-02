package main

import(
	"encoding/binary"
	"fmt"
	bencode "github.com/jackpal/bencode-go"
	"bytes"
)

type message_id uint8

const(
//	KEEP_ALIVE message_id := // zero bytes, len prefix = 0 
	CHOKE message_id = 0
	UNCHOKE message_id = 1
	INTERESTED message_id = 2
	NOT_INTERESTED message_id = 3
	HAVE message_id = 4
	BITFIELD message_id = 5
	REQUEST message_id = 6
	PIECE message_id = 7
	CANCEL message_id = 8
	PORT message_id = 9
	EXTENDED message_id = 20
)

// <length prefix><message ID><payload>
// choke - not_interrested do not have a payload
type Message struct {
	length_prefix int32
	id message_id
}

// all variables need to be uppercase for visibility in the bencode package
type ExtendedMessagePayload struct {
	M map[string]int
	V string // client version
	Metadata_size int
	P int // tcp listen port
	// should extend to give support for ipv4 and ipv6
	Reqq int // number of outstanding request messages this client supports
}

func decode_handshake(payload []byte) { // not sure what to do with result yet, no return value
	var result = ExtendedMessagePayload{nil, "v", 0, 0, 0}
	//var result = make([]ExtendedMessagePayload, 0)
	reader := bytes.NewReader([]byte(payload))
	bencode.Unmarshal(reader, &result)
	fmt.Println(result)
}

func get_handshake_message() ([]byte) {
	// <message_len><message_id == 20><handshake_identifier == 0><payload>
	payload_raw := "dut_metadata:1e" // bencoded dict setting metadata to 1, as this is the only thing we should support
	payload := []byte(payload_raw)
	message_len := uint32(len(payload))

	packet := make([]byte, message_len + 6) // 4 from length, 2 from message id and extended message id
	fmt.Println("Message length:")
	fmt.Println(message_len)
	binary.BigEndian.PutUint32(packet[0:], message_len)
	copy(packet[4:], []byte([]uint8{uint8(EXTENDED)}))
	copy(packet[5:], []byte([]uint8{uint8(0)}))
	copy(packet[6:], payload)

	return packet
}
