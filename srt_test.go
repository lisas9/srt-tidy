package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseSRTValid(t *testing.T) {
	input := "1\n" +
		"00:00:01,000 --> 00:00:02,500\n" +
		"Hello there\n" +
		"\n" +
		"2\n" +
		"00:00:03,000 --> 00:00:04,000\n" +
		"Line one\n" +
		"Line two\n"

	subs, err := ParseSRT(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("got %d subtitles, want 2", len(subs))
	}
	if subs[0].Start != time.Second || subs[0].End != 2500*time.Millisecond {
		t.Errorf("entry 1 timing = %v --> %v", subs[0].Start, subs[0].End)
	}
	if len(subs[1].Text) != 2 || subs[1].Text[0] != "Line one" || subs[1].Text[1] != "Line two" {
		t.Errorf("entry 2 text = %v", subs[1].Text)
	}
}

func TestParseSRTCRLF(t *testing.T) {
	input := "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"
	subs, err := ParseSRT(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 1 || subs[0].Text[0] != "Hello" {
		t.Fatalf("got %+v", subs)
	}
}

func TestParseSRTMultipleBlankLinesBetweenEntries(t *testing.T) {
	input := "1\n00:00:01,000 --> 00:00:02,000\nA\n\n\n\n2\n00:00:03,000 --> 00:00:04,000\nB\n"
	subs, err := ParseSRT(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("got %d subtitles, want 2", len(subs))
	}
}

func TestParseSRTRenderingHintsIgnored(t *testing.T) {
	input := "1\n00:00:01,000 --> 00:00:02,000 X1:100 X2:200 Y1:10 Y2:20\nHello\n"
	subs, err := ParseSRT(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subs[0].End != 2*time.Second {
		t.Errorf("end time = %v, want 2s", subs[0].End)
	}
}

func TestParseSRTErrors(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLine  int
		wantMatch string
	}{
		{
			name:      "no text",
			input:     "1\n00:00:01,000 --> 00:00:02,000\n",
			wantLine:  1,
			wantMatch: "no subtitle text",
		},
		{
			name:      "non-numeric index",
			input:     "x\n00:00:01,000 --> 00:00:02,000\nHi\n",
			wantLine:  1,
			wantMatch: "invalid subtitle index",
		},
		{
			name:      "index out of order",
			input:     "2\n00:00:01,000 --> 00:00:02,000\nHi\n",
			wantLine:  1,
			wantMatch: "expected 1",
		},
		{
			name:      "missing arrow",
			input:     "1\n00:00:01,000 - 00:00:02,000\nHi\n",
			wantLine:  2,
			wantMatch: "malformed timing line",
		},
		{
			name:      "invalid timestamp",
			input:     "1\nab:cd:ef,000 --> 00:00:02,000\nHi\n",
			wantLine:  2,
			wantMatch: "invalid timestamp",
		},
		{
			name:      "minutes out of range",
			input:     "1\n00:61:00,000 --> 00:62:00,000\nHi\n",
			wantLine:  2,
			wantMatch: "out of range",
		},
		{
			name:      "start not before end",
			input:     "1\n00:00:05,000 --> 00:00:02,000\nHi\n",
			wantLine:  2,
			wantMatch: "start time must be before end time",
		},
		{
			name:      "out of order entries",
			input:     "1\n00:00:05,000 --> 00:00:06,000\nA\n\n2\n00:00:01,000 --> 00:00:02,000\nB\n",
			wantLine:  6,
			wantMatch: "out of order",
		},
		{
			name:      "overlapping entries",
			input:     "1\n00:00:01,000 --> 00:00:05,000\nA\n\n2\n00:00:03,000 --> 00:00:06,000\nB\n",
			wantLine:  6,
			wantMatch: "overlaps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSRT(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var perr *ParseError
			if !errors.As(err, &perr) {
				t.Fatalf("error is not *ParseError: %v", err)
			}
			if perr.Line != tt.wantLine {
				t.Errorf("line = %d, want %d", perr.Line, tt.wantLine)
			}
			if !strings.Contains(perr.Msg, tt.wantMatch) {
				t.Errorf("message %q does not contain %q", perr.Msg, tt.wantMatch)
			}
		})
	}
}

func TestParseSRTEmptyInput(t *testing.T) {
	_, err := ParseSRT(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "no subtitle entries found") {
		t.Errorf("got error %v", err)
	}
}

func TestParseSRTBlankOnlyInput(t *testing.T) {
	_, err := ParseSRT(strings.NewReader("\n\n\n"))
	if err == nil {
		t.Fatal("expected error for blank-only input")
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"00:00:00,000", 0, false},
		{"01:02:03,004", time.Hour + 2*time.Minute + 3*time.Second + 4*time.Millisecond, false},
		{"00:59:59,999", 59*time.Minute + 59*time.Second + 999*time.Millisecond, false},
		{"not a timestamp", 0, true},
		{"00:60:00,000", 0, true},
		{"00:00:60,000", 0, true},
		{"00:00:00,1000", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTimestamp(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseTimestamp(%q): expected error, got %v", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTimestamp(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseTimestamp(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestWriteSRT(t *testing.T) {
	subs := []Subtitle{
		{Index: 5, Start: time.Second, End: 2*time.Second + 500*time.Millisecond, Text: []string{"Hello  "}},
		{Index: 9, Start: 3 * time.Second, End: 4 * time.Second, Text: []string{"Line one", "Line two"}},
	}

	var buf bytes.Buffer
	if err := WriteSRT(&buf, subs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "1\n00:00:01,000 --> 00:00:02,500\nHello\n" +
		"\n" +
		"2\n00:00:03,000 --> 00:00:04,000\nLine one\nLine two\n"
	if buf.String() != want {
		t.Errorf("output mismatch\ngot:\n%s\nwant:\n%s", buf.String(), want)
	}
}
