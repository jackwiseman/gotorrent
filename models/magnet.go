package models

import (
	"errors"
	"fmt"
	"net/url"
)

type Magnet struct {
	DisplayName string
	Trackers    []string
	ExactTopic  string
}

func NewMagnet(linkRaw string) (*Magnet, error) {

	var ml Magnet

	link, err := url.Parse(linkRaw)
	if err != nil {
		return nil, err
	}

	if link.Scheme != "magnet" {
		return nil, errors.New("not a magnet link")
	}

	params := link.Query()

	xt, ok := params["xt"]
	if !ok {
		return nil, errors.New("magnet is missing xt param")
	}
	if len(xt) != 1 {
		return nil, errors.New(fmt.Sprintf("xt has wrong number of values, 1 expected, %d received", len(xt)))
	}
	xtParsed, err := url.Parse(xt[0])
	if err != nil {
		return nil, err
	}

	if xtParsed.Scheme != "urn" {
		return nil, errors.New("magnet xt param missing urn")
	}
	ml.ExactTopic = xtParsed.Opaque

	displayNames := params["dn"]
	if len(displayNames) != 1 {
		return nil, errors.New(fmt.Sprintf("displayNames has wrong number of values, 1 expected, %d received", len(displayNames)))
	}
	ml.DisplayName = displayNames[0]

	trackers := params["tr"]
	if len(trackers) > 0 {
		ml.Trackers = trackers
	}

	return &ml, nil
}

// TODO: overhaul on link parsing, this was a bit of a hack
// only supporting udp links

// func parseMagnetLink(magnetLink string) {
// 	data := strings.Split(magnetLink, "&")
// 	for i := 0; i < len(data); i++ {
// 		switch data[i][:2] {
// 		case "dn":
// 			fmt.Println(fmt.Sprintf("%s", magnetLink))
// 			// torrent.name = strings.Replace(data[i][3:], "%20", " ", -1)
// 		case "tr":
// 			trackerLink := data[i][3:] // cut off the tr=
// 			trackerLen := len(trackerLink)
// 			index := 0
//
// 			for index < trackerLen {
// 				if strings.Compare(string(trackerLink[index]), "%") == 0 {
// 					token, err := hex.DecodeString(string(trackerLink[index+1 : index+3]))
// 					if err != nil {
// 						panic(err)
// 					}
// 					trackerLink = string(trackerLink[0:index]) + string(token) + string(trackerLink[index+3:])
// 					trackerLen -= 2
// 				}
// 				index++
// 			}
// 			if trackerLink[0:3] == "udp" {
// 				if strings.Contains(trackerLink, "announce") {
// 					trackerLink = trackerLink[:len(trackerLink)-len("/announce")]
// 				}
// 				// newTracker := newTracker(trackerLink[6:])
// 				// torrent.trackers = append(torrent.trackers, *newTracker)
// 			}
// 		default:
// 			// hash, err := hex.DecodeString(data[i][strings.LastIndex(data[i], ":")+1:])
// 			// if err != nil {
// 			// panic(err)
// 			// }
// 			// torrent.infoHash = hash
// 		}
// 	}
// }
