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
