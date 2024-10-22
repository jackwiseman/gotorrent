package models

import (
	"encoding/binary"
	"errors"
	"fmt"
	"gotorrent/utils"
	"math"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Tracker is a database which returns peers in a swarm when given a torrent hash
type Tracker struct {
	link         url.URL
	conn         net.Conn
	timeout      time.Duration // default is 15 seconds
	connectionID uint64
	retries      int
}

// return a new tracker from a string representing the link
func NewTracker(link url.URL) *Tracker {
	return &Tracker{link: link, timeout: 15 * time.Second, retries: 1}
}

// send 2x announce requests to all trackers, the first to find out how many peers they have,
// the second to request that many, so that we have a large pool to pull from
func (tracker *Tracker) FindPeers(torrent *Torrent, wg *sync.WaitGroup) {
	defer wg.Done()

	err := tracker.connect()

	if err != nil {
		return
	}

	err = tracker.setConnectionID()
	if err != nil {
		return
	}

	seeders, err := tracker.announce(torrent, 0)
	if err != nil {
		return
	}

	numSeeders, err := tracker.announce(torrent, seeders)
	if err != nil {
		return
	}
	log.Info().Msg(fmt.Sprintf("tracker %s has %d seeders", tracker.link.String(), numSeeders))

	err = tracker.disconnect()
	if err != nil {
		panic(err)
	}
}

// for now, only works for udp links, which seem to be standard
func (tracker *Tracker) connect() error {
	// limit to udp for now
	if tracker.link.Scheme != "udp" {
		log.Warn().Msg(fmt.Sprintf("skipping %s of type %s (unsupported)", tracker.link.String(), tracker.link.Scheme))
		return errors.New("only udp trackers are currently supported")
	}

	conn, err := net.DialTimeout(tracker.link.Scheme, tracker.link.Host, tracker.timeout)
	if err != nil {
		return (err)
	}
	tracker.conn = conn
	return nil
}

func (tracker *Tracker) disconnect() error {
	err := tracker.conn.Close()
	if err != nil {
		return err
	}
	return nil
}

// the first step in getting peers from the tracker is getting a connection_id, which is valid for 2 minutes
func (tracker *Tracker) setConnectionID() error {
	for i := 0; i <= tracker.retries; i++ {
		// Create a new Connection_Request with a new transactionID
		transactionID, err := utils.GetTransactionID()
		if err != nil {
			return (err)
		}

		// Serialize a 16 byte request for a connection id where
		// Offset	Name		Value
		// 0		protocol_id	0x41727101980 - magic constant
		// 8		action 		0 - connect
		// 12		transaction_id 	which is randomly generated per request
		packet := make([]byte, 16)
		binary.BigEndian.PutUint64(packet[0:], 0x41727101980)
		binary.BigEndian.PutUint32(packet[8:], 0)
		binary.BigEndian.PutUint32(packet[12:], transactionID)

		// Set a write deadline of 15 seconds and connect
		err = tracker.conn.SetWriteDeadline(time.Now().Add(tracker.timeout))
		if err != nil {
			panic(err)
		}
		bytesWritten, err := tracker.conn.Write(packet)
		if err != nil {
			continue
		}
		if bytesWritten < len(packet) {
			return (errors.New("did not write 16 bytes"))
		}

		// Per BitTorrent.org specificiations, "If a response is not recieved after 15 * 2 ^ n seconds, client should retransmit, where n increases to 8 from 0
		err = tracker.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15*math.Pow(2, float64(i))))))
		if err != nil {
			panic(err)
		}

		// Expecting a 16 byte response where
		// Offset	Name		Value
		// 0		action		0 - connect
		// 4		transaction_id	should be same that was sent
		// 8		connection_id
		buf := make([]byte, 16)

		bytesRead, err := tracker.conn.Read(buf)
		if err != nil {
			if i == tracker.retries {
				return (err)
			}
			continue
		}
		if bytesRead != 16 {
			return errors.New("did not read 16 bytes") // try again?
		}

		// Make sure the response has the connect action and the same transaction id that we sent
		if binary.BigEndian.Uint32(buf[0:]) != 0 || binary.BigEndian.Uint32(buf[4:]) != transactionID {
			return errors.New("received bad connect data from tracker")
		}

		tracker.connectionID = binary.BigEndian.Uint64(buf[8:])
		return nil
	}
	return nil
}

// announce to a tracker requesting num_peers ip addresses
// returns # of seeders
func (tracker *Tracker) announce(torrent *Torrent, numWant int) (int, error) {
	for i := 0; i <= tracker.retries; i++ {
		transactionID, err := utils.GetTransactionID()
		if err != nil {
			return 0, err
		}

		// Create announce packet
		packet := make([]byte, 98)
		// connection_id
		binary.BigEndian.PutUint64(packet[0:], tracker.connectionID)
		// action
		binary.BigEndian.PutUint32(packet[8:], 1)
		// transaction_id
		binary.BigEndian.PutUint32(packet[12:], transactionID)
		// info_hash
		copy(packet[16:], torrent.infoHash)
		// peerID (20 bytes)
		peerID := "GoLangTorrent_v0.0.1" // should be randomly set
		copy(packet[36:], []byte(peerID))
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
		binary.BigEndian.PutUint32(packet[92:], uint32(numWant))
		// port
		binary.BigEndian.PutUint16(packet[96:], 6881)

		// Set timeout
		//tracker.conn.SetWriteDeadline(time.Now().Add(tracker.timeout))

		bytesWritten, err := tracker.conn.Write(packet)
		if err != nil || bytesWritten < len(packet) {
			return 0, errors.New("could not write announce request")
		}

		buf := make([]byte, 20+(6*numWant))
		err = tracker.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(int(15*math.Pow(2, float64(i)))))) // BEP 15 - If a response is not received after 15 * 2 ^ n seconds, the client should retransmit the request, where n starts at 0 and is increased up to 8 (3840 seconds) after every retransmission
		if err != nil {
			panic(err)
		}

		bytesRead, err := tracker.conn.Read(buf)
		if bytesRead < 20 || err != nil {
			if i >= tracker.retries {
				return 0, err
			}
			continue
		}

		seeders := int(binary.BigEndian.Uint32(buf[16:]))
		for j := 0; j < int(math.Min(float64(numWant), float64(seeders))); j++ {
			ipAddressRaw := binary.BigEndian.Uint32(buf[20+(6*j):])
			port := binary.BigEndian.Uint16(buf[24+(6*j):])

			// convert ipAddress into a string representation
			ipAddress := make(net.IP, 4)
			binary.BigEndian.PutUint32(ipAddress, ipAddressRaw)

			torrent.peers = append(torrent.peers, *newPeer(ipAddress.String(), strconv.Itoa(int(port)), torrent))
		}
		return seeders, nil
	}
	return 0, errors.New("tracker timed out")
}
