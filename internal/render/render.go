// Package render writes matching records out.
//
// The renderers divide by what the output is for. Raw is for piping onward
// and reproduces the input byte for byte. Table is for reading, and is the
// only one that buffers, because aligning columns means knowing the widest
// value before anything can be printed.
package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/Tani314/sift/internal/record"
)

// Renderer writes records to a writer.
type Renderer interface {
	// Write emits one record, or buffers it until Flush.
	Write(w io.Writer, r record.Record) error
	// Flush emits anything held back. Always called, even when no records
	// matched, so a renderer can print a header or an empty-result note.
	Flush(w io.Writer) error
}

// Raw reproduces the original line.
//
// This is the default because sift is usually one stage in a pipeline, and a
// tool that reformats what it forwards breaks every command downstream of it.
type Raw struct {
	// ShowSource prefixes each line with file:line, the way grep does when
	// it is given more than one file.
	ShowSource bool
}

// Write prints the original line.
func (raw Raw) Write(w io.Writer, r record.Record) error {
	if raw.ShowSource {
		_, err := fmt.Fprintf(w, "%s:%d: %s\n", r.Source, r.Num, r.Line)
		return err
	}
	_, err := fmt.Fprintln(w, r.Line)
	return err
}

// Flush does nothing: Raw streams.
func (Raw) Flush(io.Writer) error { return nil }

// Fields prints selected fields, one record per line.
//
// It is the shape people actually want when reading: a log line carries
// twenty fields and an investigation cares about three.
type Fields struct {
	// Names are the fields to print, in the order given.
	Names []string
}

// Write prints the selected fields.
func (f Fields) Write(w io.Writer, r record.Record) error {
	parts := make([]string, 0, len(f.Names))
	for _, name := range f.Names {
		v, ok := r.Get(name)
		if !ok {
			// A gap is printed rather than skipped, so columns stay
			// aligned with each other across ragged records.
			v = "-"
		}
		parts = append(parts, v)
	}
	_, err := fmt.Fprintln(w, strings.Join(parts, "  "))
	return err
}

// Flush does nothing: Fields streams.
func (Fields) Flush(io.Writer) error { return nil }
