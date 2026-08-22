package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Subtitle is a single cue: an index, a time range, and one or more lines of text.
type Subtitle struct {
	Index int
	Start time.Duration
	End   time.Duration
	Text  []string

	// Line is the input line number of the timing line, kept around so
	// cross-entry checks (overlap, ordering) can point at the right place.
	Line int
}

// ParseError points at the input line that failed validation, since SRT
// files are hand-edited often enough that a bare "invalid input" isn't useful.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

type lineRec struct {
	num  int
	text string
}

// ParseSRT reads a SubRip file and returns its cues, or the first validation
// error encountered. It accepts both LF and CRLF line endings.
func ParseSRT(r io.Reader) ([]Subtitle, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var recs []lineRec
	num := 0
	for scanner.Scan() {
		num++
		recs = append(recs, lineRec{num: num, text: strings.TrimRight(scanner.Text(), "\r")})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	blocks := splitBlocks(recs)
	if len(blocks) == 0 {
		return nil, errors.New("no subtitle entries found")
	}

	subs := make([]Subtitle, 0, len(blocks))
	for i, block := range blocks {
		sub, err := parseBlock(block, i+1)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}

	if err := checkOrdering(subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// checkOrdering verifies that cues are sorted by start time and don't
// overlap. Both problems are common after a lossy frame rate conversion,
// and playback behavior when they occur is undefined per-player, so it's
// worth catching here rather than letting each renderer guess differently.
func checkOrdering(subs []Subtitle) error {
	for i := 1; i < len(subs); i++ {
		prev, cur := subs[i-1], subs[i]
		if cur.Start < prev.Start {
			return &ParseError{
				Line: cur.Line,
				Msg:  fmt.Sprintf("entry %d starts before entry %d (out of order)", cur.Index, prev.Index),
			}
		}
		if cur.Start < prev.End {
			return &ParseError{
				Line: cur.Line,
				Msg:  fmt.Sprintf("entry %d overlaps entry %d", cur.Index, prev.Index),
			}
		}
	}
	return nil
}

// splitBlocks groups lines into cues separated by one or more blank lines.
func splitBlocks(lines []lineRec) [][]lineRec {
	var blocks [][]lineRec
	var cur []lineRec
	for _, l := range lines {
		if strings.TrimSpace(l.text) == "" {
			if len(cur) > 0 {
				blocks = append(blocks, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, l)
	}
	if len(cur) > 0 {
		blocks = append(blocks, cur)
	}
	return blocks
}

func parseBlock(block []lineRec, position int) (Subtitle, error) {
	if len(block) < 3 {
		return Subtitle{}, &ParseError{Line: block[0].num, Msg: "entry has no subtitle text"}
	}

	idxLine := block[0]
	index, err := strconv.Atoi(strings.TrimSpace(idxLine.text))
	if err != nil {
		return Subtitle{}, &ParseError{Line: idxLine.num, Msg: fmt.Sprintf("invalid subtitle index %q", idxLine.text)}
	}
	if index != position {
		return Subtitle{}, &ParseError{Line: idxLine.num, Msg: fmt.Sprintf("subtitle index out of order: got %d, expected %d", index, position)}
	}

	timingLine := block[1]
	start, end, err := parseTimingLine(timingLine.text)
	if err != nil {
		return Subtitle{}, &ParseError{Line: timingLine.num, Msg: err.Error()}
	}
	if start >= end {
		return Subtitle{}, &ParseError{Line: timingLine.num, Msg: "start time must be before end time"}
	}

	text := make([]string, 0, len(block)-2)
	for _, l := range block[2:] {
		text = append(text, l.text)
	}

	return Subtitle{Index: index, Start: start, End: end, Text: text, Line: timingLine.num}, nil
}

func parseTimingLine(s string) (start, end time.Duration, err error) {
	parts := strings.SplitN(s, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("malformed timing line %q", s)
	}
	start, err = parseTimestamp(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("start time: %w", err)
	}
	// Some encoders tack rendering hints (X1:.. Y1:..) after the end
	// timestamp on the same line; only the first field matters here.
	fields := strings.Fields(parts[1])
	if len(fields) == 0 {
		return 0, 0, errors.New("missing end time")
	}
	end, err = parseTimestamp(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("end time: %w", err)
	}
	return start, end, nil
}

// parseTimestamp parses the SubRip HH:MM:SS,mmm format.
func parseTimestamp(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	var h, m, sec, ms int
	n, err := fmt.Sscanf(s, "%d:%d:%d,%d", &h, &m, &sec, &ms)
	if err != nil || n != 4 {
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

func formatTimestamp(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// WriteSRT writes cues back out in normalized form: sequential indices,
// zero-padded timestamps, and a single blank line between entries.
func WriteSRT(w io.Writer, subs []Subtitle) error {
	for i, s := range subs {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%d\n%s --> %s\n", i+1, formatTimestamp(s.Start), formatTimestamp(s.End)); err != nil {
			return err
		}
		for _, line := range s.Text {
			if _, err := fmt.Fprintln(w, strings.TrimRight(line, " \t")); err != nil {
				return err
			}
		}
	}
	return nil
}
