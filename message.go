package main

import (
	"bytes"
	"encoding/binary"

	bencode "github.com/jackpal/bencode-go"
)

const (
	//	KEEP_ALIVE message_id := // zero bytes, len prefix = 0
	Choke         = 0
	Unchoke       = 1
	Interested    = 2
	NotInterested = 3
	Have          = 4
	Bitfield      = 5
	Request       = 6
	PIECE         = 7 //TODO
	Cancel        = 8
	Port          = 9
	Extended      = 20

	// for internal use only
	Stop = 99
)

// <length prefix><message ID><payload>
// choke - not_interrested do not have a payload
type Message struct {
	lengthPrefix uint32
	id           int
	payload      []byte
}

type ExtendedMessage struct {
	lengthPrefix uint32
	id           uint8
	extendedId   uint8
	payload      []byte
}

// all variables need to be uppercase for visibility in the bencode package
type ExtendedHandshakePayload struct {
	M            map[string]int
	V            string // client version
	MetadataSize int
	P            int // tcp listen port
	// should extend to give support for ipv4 and ipv6
	Reqq int // number of outstanding request messages this client supports
}

type MetadataRequest struct {
	MsgType int "msg_type"
	Piece   int "piece"
}

type MetadataResponse struct {
	MsgType   int
	Piece     int
	TotalSize int
}

func (message *Message) marshall() []byte {
	packet := make([]byte, 4) // len_prefix (4) + id (1)
	binary.BigEndian.PutUint32(packet[0:], message.lengthPrefix)
	packet = append(packet, uint8(message.id))
	if message.payload != nil {
		packet = append(packet, message.payload...)
	}
	return packet
}

func (message *ExtendedMessage) marshall() []byte {
	packet := make([]byte, 4) // lengthPrefix (4) + id (1) + extended_id (1) + remaining payload
	lengthPrefix := uint32(len(message.payload) + 2)
	binary.BigEndian.PutUint32(packet[0:], lengthPrefix)
	packet = append(packet, uint8(message.id))
	packet = append(packet, uint8(message.extendedId))
	packet = append(packet, message.payload...)

	return packet
}

func encodeMetadataRequest(pieceNumber int) string {
	var b bytes.Buffer
	var data MetadataRequest
	data.MsgType = 0
	data.Piece = pieceNumber
	bencode.Marshal(&b, data)
	//fmt.Println(b.String())
	return b.String()
}

func decodeMetadataRequest(payload []byte) MetadataResponse {
	var result = MetadataResponse{0, 0, 0}
	reader := bytes.NewReader([]byte(payload))
	bencode.Unmarshal(reader, &result)
	return result
}

func decodeHandshake(payload []byte) *ExtendedHandshakePayload {
	var result = ExtendedHandshakePayload{nil, "v", 0, 0, 0}
	reader := bytes.NewReader([]byte(payload))
	bencode.Unmarshal(reader, &result)
	return &result
}

func getHandshakeMessage(torrent *Torrent) []byte {
	pstrlen := 19
	pstr := "BitTorrent protocol"

	packet := make([]byte, 49+pstrlen)
	copy(packet[0:], []uint8{uint8(pstrlen)})
	copy(packet[1:], []byte(pstr))
	packet[25] = 16
	copy(packet[28:], torrent.infoHash)
	peerId := "GoLangTorrent_v0.0.1" // TODO: generate a random peer_id?
	copy(packet[48:], []byte(peerId))

	return packet
}

func getExtendedHandshakeMessage() []byte {
	// <message_len><message_id == 20><handshake_identifier == 0><payload>
	payloadRaw := "d11:ut_metadatai1ee" // bencoded dict setting metadata to 1, as this is the only thing we should support
	payload := []byte(payloadRaw)
	messageLen := uint32(len(payload) + 2)

	packet := make([]byte, messageLen+4)
	binary.BigEndian.PutUint32(packet[0:], messageLen)
	copy(packet[4:], []byte([]uint8{uint8(Extended)}))
	copy(packet[5:], []byte([]uint8{uint8(0)}))
	copy(packet[6:], payload)

	return packet
}
