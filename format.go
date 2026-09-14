package main

import (
	"errors"
	"io"
)

// Parse sniffs the input's format from its first line and parses it with
// the matching parser. WebVTT is identified by its required signature
// line; anything else is treated as SubRip.
func Parse(r io.Reader) ([]Subtitle, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errors.New("no subtitle entries found")
	}
	if looksLikeWebVTT(lines[0].text) {
		return parseVTTLines(lines)
	}
	return parseSRTLines(lines)
}
