package main

import(
	"encoding/binary"
//	"fmt"
	bencode "github.com/jackpal/bencode-go"
	"bytes"
)

const(
//	KEEP_ALIVE message_id := // zero bytes, len prefix = 0 
	CHOKE = 0
	UNCHOKE = 1
	INTERESTED = 2
	NOT_INTERESTED = 3
	HAVE = 4
	BITFIELD = 5
	REQUEST = 6
	PIECE = 7
	CANCEL = 8
	PORT = 9
	EXTENDED = 20

	// for internal use only
	STOP = 99 
)

// <length prefix><message ID><payload>
// choke - not_interrested do not have a payload
type Message struct {
	length_prefix uint32
	id int
	payload []byte
}


type Extended_Message struct {
	length_prefix uint32
	id uint8
	extended_id uint8
	payload []byte
}

// all variables need to be uppercase for visibility in the bencode package
type Extended_Handshake_Payload struct {
	M map[string]int
	V string // client version
	Metadata_size int
	P int // tcp listen port
	// should extend to give support for ipv4 and ipv6
	Reqq int // number of outstanding request messages this client supports
}

type Metadata_Request struct {
	Msg_type int "msg_type"
	Piece int "piece"
}

type Metadata_Response struct {
	Msg_type int
	Piece int
	Total_size int
}

func (message *Message) marshall() ([]byte) {
	packet := make([]byte, 4) // len_prefix (4) + id (1)
	binary.BigEndian.PutUint32(packet[0:], message.length_prefix)
	packet = append(packet, uint8(message.id))
	//fmt.Println(packet)
	return packet
}

func (message *Extended_Message) marshall() ([]byte) {
	packet := make([]byte, 4) // length_prefix (4) + id (1) + extended_id (1) + remaining payload
	length_prefix := uint32(len(message.payload) + 2)
	binary.BigEndian.PutUint32(packet[0:], length_prefix)
	packet = append(packet, uint8(message.id))
	packet = append(packet, uint8(message.extended_id))
	packet = append(packet, message.payload...)

	return packet
}

func encode_metadata_request(piece_number int) (string) {
	var b bytes.Buffer
	var data Metadata_Request
	data.Msg_type = 0
	data.Piece = piece_number
	bencode.Marshal(&b, data)
	//fmt.Println(b.String())
	return b.String()
}

func decode_metadata_request(payload []byte) (Metadata_Response) {
	var result = Metadata_Response{0, 0, 0}
	reader := bytes.NewReader([]byte(payload))
	bencode.Unmarshal(reader, &result)
//	fmt.Println("Response: {'msg_type': " + strconv.Itoa(result.Msg_type) + ", 'piece': " + strconv.Itoa(result.Msg_type) + "}")
//	fmt.Println("Raw:")
//	fmt.Println(result)
	return result
}

func decode_handshake(payload []byte) (*Extended_Handshake_Payload) { // not sure what to do with result yet, no return value
	var result = Extended_Handshake_Payload{nil, "v", 0, 0, 0}
	//var result = make([]ExtendedMessagePayload, 0)
	reader := bytes.NewReader([]byte(payload))
	bencode.Unmarshal(reader, &result)
	//fmt.Println(result)
	return &result
}

func get_handshake_message(torrent *Torrent) ([]byte) {
	pstrlen := 19
	pstr := "BitTorrent protocol"

	packet := make([]byte, 49 + pstrlen)
	copy(packet[0:], []uint8{uint8(pstrlen)})
	copy(packet[1:], []byte(pstr))
	packet[25] = 16
	copy(packet[28:], torrent.info_hash)
//	fmt.Println(string(torrent.info_hash))
	peer_id := "GoLangTorrent_v0.0.1" // TODO: generate a random peer_id?
	copy(packet[48:], []byte(peer_id))

	return packet
}

func get_extended_handshake_message() ([]byte) {
	// <message_len><message_id == 20><handshake_identifier == 0><payload>
	payload_raw := "d11:ut_metadatai1ee" // bencoded dict setting metadata to 1, as this is the only thing we should support
	payload := []byte(payload_raw)
//	message_len := uint32(len(payload))
	message_len := uint32(len(payload) + 2)

	packet := make([]byte, message_len + 4)
//	packet := make([]byte, message_len + 6) // 4 from length, 2 from message id and extended message id
//	fmt.Println("Message length:")
//	fmt.Println(message_len)
	binary.BigEndian.PutUint32(packet[0:], message_len)
	copy(packet[4:], []byte([]uint8{uint8(EXTENDED)}))
	copy(packet[5:], []byte([]uint8{uint8(0)}))
	copy(packet[6:], payload)

	return packet
}
