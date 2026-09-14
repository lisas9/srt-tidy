package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ParseVTT reads a WebVTT file and returns its cues in the same Subtitle
// form as ParseSRT, so both formats can feed the same formatter. Only the
// cue-timing subset of WebVTT is understood: styling, regions, and cue
// settings are accepted where present but ignored.
func ParseVTT(r io.Reader) ([]Subtitle, error) {
	recs, err := readLines(r)
	if err != nil {
		return nil, err
	}
	return parseVTTLines(recs)
}

func parseVTTLines(lines []lineRec) ([]Subtitle, error) {
	if len(lines) == 0 || !looksLikeWebVTT(lines[0].text) {
		return nil, errors.New("missing WEBVTT signature")
	}

	// Everything between the signature and the first blank line is header
	// metadata (Kind:, Language:, etc.); it isn't needed for reprinting.
	idx := 1
	for idx < len(lines) && strings.TrimSpace(lines[idx].text) != "" {
		idx++
	}
	var body []lineRec
	if idx < len(lines) {
		body = lines[idx+1:]
	}

	blocks := splitBlocks(body)

	var subs []Subtitle
	for _, block := range blocks {
		first := strings.TrimSpace(block[0].text)
		if strings.HasPrefix(first, "NOTE") || first == "STYLE" || first == "REGION" {
			continue
		}

		timingIdx := -1
		switch {
		case strings.Contains(block[0].text, "-->"):
			timingIdx = 0
		case len(block) > 1 && strings.Contains(block[1].text, "-->"):
			timingIdx = 1
		default:
			return nil, &ParseError{Line: block[0].num, Msg: "cue missing timing line"}
		}

		timingLine := block[timingIdx]
		start, end, err := parseVTTTimingLine(timingLine.text)
		if err != nil {
			return nil, &ParseError{Line: timingLine.num, Msg: err.Error()}
		}
		if start >= end {
			return nil, &ParseError{Line: timingLine.num, Msg: "start time must be before end time"}
		}

		textLines := block[timingIdx+1:]
		if len(textLines) == 0 {
			return nil, &ParseError{Line: timingLine.num, Msg: "entry has no subtitle text"}
		}
		text := make([]string, 0, len(textLines))
		for _, l := range textLines {
			text = append(text, l.text)
		}

		subs = append(subs, Subtitle{
			Index: len(subs) + 1,
			Start: start,
			End:   end,
			Text:  text,
			Line:  timingLine.num,
		})
	}

	if len(subs) == 0 {
		return nil, errors.New("no subtitle entries found")
	}
	if err := checkOrdering(subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func looksLikeWebVTT(s string) bool {
	s = strings.TrimSpace(s)
	return s == "WEBVTT" || strings.HasPrefix(s, "WEBVTT ") || strings.HasPrefix(s, "WEBVTT\t")
}

func parseVTTTimingLine(s string) (start, end time.Duration, err error) {
	parts := strings.SplitN(s, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("malformed timing line %q", s)
	}
	start, err = parseVTTTimestamp(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("start time: %w", err)
	}
	// Cue settings (align:, position:, etc.) may follow the end timestamp
	// on the same line; only the first field matters here.
	fields := strings.Fields(parts[1])
	if len(fields) == 0 {
		return 0, 0, errors.New("missing end time")
	}
	end, err = parseVTTTimestamp(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("end time: %w", err)
	}
	return start, end, nil
}

// parseVTTTimestamp parses WebVTT's timestamp format, which uses a period
// instead of a comma before milliseconds and allows the hours field to be
// omitted for durations under an hour.
func parseVTTTimestamp(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	fields := strings.Split(s, ":")

	var h, m int
	var secField string
	var err error
	switch len(fields) {
	case 3:
		if h, err = strconv.Atoi(fields[0]); err != nil {
			return 0, fmt.Errorf("invalid timestamp %q", s)
		}
		if m, err = strconv.Atoi(fields[1]); err != nil {
			return 0, fmt.Errorf("invalid timestamp %q", s)
		}
		secField = fields[2]
	case 2:
		if m, err = strconv.Atoi(fields[0]); err != nil {
			return 0, fmt.Errorf("invalid timestamp %q", s)
		}
		secField = fields[1]
	default:
		return 0, fmt.Errorf("invalid timestamp %q", s)
	}

	secParts := strings.SplitN(secField, ".", 2)
	if len(secParts) != 2 || len(secParts[1]) != 3 {
		return 0, fmt.Errorf("invalid timestamp %q", s)
	}
	sec, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp %q", s)
	}
	ms, err := strconv.Atoi(secParts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp %q", s)
	}
	if m > 59 || sec > 59 || ms > 999 {
		return 0, fmt.Errorf("timestamp %q out of range", s)
	}

	d := time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(ms)*time.Millisecond
	return d, nil
}
