package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "check":
		err = runCheck(args)
	case "fmt":
		err = runFmt(args)
	case "help", "-h", "--help":
		usage(os.Stdout)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `srt-tidy - validate and pretty-print SubRip (.srt) subtitle files

Usage:
  srt-tidy check [file]   validate a subtitle file, reading stdin if file is omitted or "-"
  srt-tidy fmt [file]     print a normalized version of a subtitle file to stdout
`)
}

// openInput resolves the input argument: no argument or "-" means stdin,
// anything else is treated as a file path.
func openInput(args []string) (r io.Reader, name string, closeFn func() error, err error) {
	if len(args) == 0 || args[0] == "-" {
		return os.Stdin, "<stdin>", func() error { return nil }, nil
	}
	f, err := os.Open(args[0])
	if err != nil {
		return nil, "", nil, err
	}
	return f, args[0], f.Close, nil
}

func runCheck(args []string) error {
	r, name, closeFn, err := openInput(args)
	if err != nil {
		return err
	}
	defer closeFn()

	subs, err := ParseSRT(r)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	fmt.Printf("%s: ok, %d subtitles\n", name, len(subs))
	return nil
}

func runFmt(args []string) error {
	r, name, closeFn, err := openInput(args)
	if err != nil {
		return err
	}
	defer closeFn()

	subs, err := ParseSRT(r)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return WriteSRT(os.Stdout, subs)
}
