package utils

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Return a new, random 32-bit integer
func GetTransactionID() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return uint32(binary.BigEndian.Uint32(b[:])), nil
}

// Given a byte slice, set bit at position pos in big endian order
func SetBit(data *[]byte, pos int) error {
	if pos >= len(*data)*8 {
		return errors.New("pos index out of range")
	}

	(*data)[pos/8] = (*data)[pos/8] | (1 << (7 - (pos % 8)))

	return nil
}

func UnsetBit(data *[]byte, pos int) error {
	if pos >= len(*data)*8 {
		return errors.New("pos index out of range")
	}

	(*data)[pos/8] = (*data)[pos/8] & ^(1 << (7 - (pos % 8)))

	return nil
}

// Given a byte slice, return whether byte as position pos (big endian) is set
// Ie BitIsSet(10, [0 64]) -> true
func BitIsSet(data []byte, pos int) (bool, error) {
	if pos >= len(data)*8 {
		return false, errors.New("pos index out of range")
	}

	return data[(pos/int(8))]>>(7-(pos%8))&1 == 1, nil
}

// TODO: overhaul on link parsing, this was a bit of a hack
// only supporting udp links
func parseMagnetLink(magnetLink string) {
	data := strings.Split(magnetLink, "&")
	for i := 0; i < len(data); i++ {
		switch data[i][:2] {
		case "dn":
			fmt.Println(fmt.Sprintf("%s", magnetLink))
			// torrent.name = strings.Replace(data[i][3:], "%20", " ", -1)
		case "tr":
			trackerLink := data[i][3:] // cut off the tr=
			trackerLen := len(trackerLink)
			index := 0

			for index < trackerLen {
				if strings.Compare(string(trackerLink[index]), "%") == 0 {
					token, err := hex.DecodeString(string(trackerLink[index+1 : index+3]))
					if err != nil {
						panic(err)
					}
					trackerLink = string(trackerLink[0:index]) + string(token) + string(trackerLink[index+3:])
					trackerLen -= 2
				}
				index++
			}
			if trackerLink[0:3] == "udp" {
				if strings.Contains(trackerLink, "announce") {
					trackerLink = trackerLink[:len(trackerLink)-len("/announce")]
				}
				// newTracker := newTracker(trackerLink[6:])
				// torrent.trackers = append(torrent.trackers, *newTracker)
			}
		default:
			// hash, err := hex.DecodeString(data[i][strings.LastIndex(data[i], ":")+1:])
			// if err != nil {
			// panic(err)
			// }
			// torrent.infoHash = hash
		}
	}
}
