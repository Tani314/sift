// Package record holds the one shape every other package agrees on.
//
// A log line becomes a Record exactly once, at the point it is read, and
// everything downstream -- filtering, rendering, statistics -- works on
// Records rather than on strings. That keeps the parsers as the only code
// that has to know what a log line looks like.
package record

import "sort"

// Record is a single parsed log line.
type Record struct {
	// Line is the original text, kept verbatim. Rendering raw output means
	// reproducing what was in the file, not re-serialising the fields, so
	// nothing is lost to a parser that guessed wrong.
	Line string

	// Fields are the extracted key/value pairs. Nil for a line no parser
	// could make sense of, which is not an error: unparseable lines still
	// match a raw search and still print.
	Fields map[string]string

	// Source names the file the line came from, so output from several files
	// stays attributable.
	Source string

	// Num is the 1-based line number within Source.
	Num int
}

// Get returns the value of a field.
func (r Record) Get(key string) (string, bool) {
	v, ok := r.Fields[key]
	return v, ok
}

// Keys returns the field names in sorted order, so output built from a map
// does not shuffle between runs.
func (r Record) Keys() []string {
	keys := make([]string, 0, len(r.Fields))
	for k := range r.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Parsed reports whether any parser understood this line.
func (r Record) Parsed() bool { return len(r.Fields) > 0 }
