package library

import (
	"encoding/json"
)

type Stream struct {
	Index     int
	CodecType string `json:"codec_type"`
	Tags      struct {
		Language string
	}
}

type VideoMedia struct {
	Format struct {
		Duration string
		Size     string
	}
	Streams []Stream
}

func ParseVideoMedia(path string) (*VideoMedia, error) {
	infos, err := Probe(path)
	if err != nil {
		return nil, err
	}

	var video VideoMedia
	if err := json.Unmarshal([]byte(infos), &video); err != nil {
		return nil, err
	}

	return &video, nil
}
