# srt-tidy

Subtitle files rot. Frame rate conversions leave indices out of order,
some editors write `HH:MM:SS.mmm` instead of the comma the SubRip spec
actually calls for, and every tool seems to disagree about how many
blank lines belong between entries. `srt-tidy` parses `.srt` files
strictly enough to catch that, and reprints them in a normalized form.

It also reads WebVTT (`.vtt`) input, detected automatically from the
`WEBVTT` signature line, and normalizes it into the same SubRip output.
Cue settings, styling, and regions in the source file are dropped since
SubRip has no equivalent for them.

## Usage

Validate a file:

```
$ srt-tidy check movie.srt
movie.srt: ok, 214 subtitles
```

Pretty-print to stdout (renumbers entries, zero-pads timestamps,
collapses blank-line runs to one):

```
$ srt-tidy fmt movie.srt > movie.clean.srt
```

Both commands read from stdin when no file is given, or when the
file argument is `-`. That means it composes with pipelines instead
of needing a temp file:

```
$ curl -s https://example.invalid/movie.srt | srt-tidy check
$ ffmpeg -i movie.mkv -map 0:s:0 -f srt - | srt-tidy fmt > movie.srt
```

## What "valid" means here

- entries are numbered sequentially starting at 1
- each entry has a `HH:MM:SS,mmm --> HH:MM:SS,mmm` timing line
- the start time is strictly before the end time
- every entry has at least one line of text
- entries are sorted by start time, with no overlap between consecutive entries

Anything else is reported as `line N: <reason>` pointing at the
offending line in the input.

## Building

Standard library only, no third-party modules.

```
go build ./...
```

## Status

Early. The parser and formatter handle plain SubRip and the common
subset of WebVTT, including overlap and ordering checks between
consecutive cues; there are no unit tests yet, `fmt` only writes to
stdout (no in-place rewrite), and non-UTF8 input including a leading
BOM is not handled.

## License

MIT, see LICENSE.
