// Package source reads log lines from files and from standard input.
//
// Everything here streams. A log file is routinely larger than the machine's
// memory, and the one time anybody reaches for a log tool is the one time
// they cannot afford it to allocate a gigabyte -- so lines are handed to a
// callback as they are read and nothing accumulates.
package source

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
)

// Stdin is the path that means standard input, following the convention
// every other command-line tool uses.
const Stdin = "-"

// maxLineBytes bounds a single line. Structured logs with embedded stack
// traces or base64 payloads routinely exceed bufio.Scanner's 64KB default,
// and the failure mode there is a truncated line that silently stops
// matching -- so the limit is raised to something no real log line reaches.
const maxLineBytes = 8 * 1024 * 1024

// Line is one line of input with the context needed to report it.
type Line struct {
	Text   string
	Num    int
	Source string
}

// Handler receives each line. Returning an error stops the scan.
type Handler func(Line) error

// Scan reads a file (or standard input) and calls fn for each line.
func Scan(path string, fn Handler) error {
	r, name, closer, err := open(path)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	num := 0
	for scanner.Scan() {
		num++
		if err := fn(Line{Text: scanner.Text(), Num: num, Source: name}); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	return nil
}

// ScanAll reads several inputs in order, so line numbers and source names
// stay attributable when output from many files is interleaved.
func ScanAll(paths []string, fn Handler) error {
	if len(paths) == 0 {
		return Scan(Stdin, fn)
	}
	for _, p := range paths {
		if err := Scan(p, fn); err != nil {
			return err
		}
	}
	return nil
}

// Sample reads up to n lines without consuming the input, for format
// detection. Standard input cannot be rewound, so sampling it is not
// offered: the CLI asks for an explicit --format there instead of silently
// eating the first lines of a pipe.
func Sample(path string, n int) ([]string, error) {
	if path == Stdin {
		return nil, nil
	}

	var lines []string
	err := Scan(path, func(l Line) error {
		if strings.TrimSpace(l.Text) != "" {
			lines = append(lines, l.Text)
		}
		if len(lines) >= n {
			return errEnough
		}
		return nil
	})
	if err != nil && err != errEnough {
		return nil, err
	}
	return lines, nil
}

// errEnough stops a scan early without being a real failure.
var errEnough = fmt.Errorf("sample complete")

// open resolves a path to a reader, transparently decompressing gzip.
//
// Rotated logs are almost always gzipped, and needing to remember which of
// app.log and app.log.1.gz can be searched directly is exactly the friction
// that sends people back to zcat and a pipeline.
func open(path string) (io.Reader, string, io.Closer, error) {
	if path == Stdin {
		return os.Stdin, "(stdin)", nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, "", nil, fmt.Errorf("opening %s: %w", path, err)
	}

	if !strings.HasSuffix(path, ".gz") {
		return f, path, f, nil
	}

	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, "", nil, fmt.Errorf("reading %s as gzip: %w", path, err)
	}
	return gz, path, multiCloser{gz, f}, nil
}

// multiCloser closes a decompressor and the file underneath it.
type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var firstErr error
	for _, c := range m {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
