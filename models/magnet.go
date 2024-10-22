package models

import (
	"errors"
	"fmt"
	"net/url"
)

type Magnet struct {
	DisplayName string
	Trackers    []*Tracker
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

	// all params can be found here: https://en.wikipedia.org/wiki/Magnet_URI_scheme
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
	if len(displayNames) == 1 {
		ml.DisplayName = displayNames[0]
	}

	trackers := params["tr"]
	ml.Trackers = make([]*Tracker, 0)
	for _, trackerUrl := range trackers {
		// convert raw url string to url.URL
		url, err := url.Parse(trackerUrl)
		if err != nil {
			return nil, err
		}
		ml.Trackers = append(ml.Trackers, NewTracker(*url))
	}

	for _, t := range ml.Trackers {
		fmt.Println(t)
	}

	return &ml, nil
}
