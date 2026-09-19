package parse

import (
	"fmt"
	"strings"
)

// All is every parser sift knows, in the order detection tries them. JSON
// first because its check is a single byte comparison, and because a JSON
// line can never be valid logfmt anyway.
var All = []Parser{JSON{}, LogFmt{}}

// ByName returns the parser a --format flag asked for.
func ByName(name string) (Parser, error) {
	for _, p := range All {
		if p.Name() == name {
			return p, nil
		}
	}
	names := make([]string, 0, len(All)+1)
	for _, p := range All {
		names = append(names, p.Name())
	}
	names = append(names, "auto")
	return nil, fmt.Errorf("unknown format %q (known: %s)", name, strings.Join(names, ", "))
}

// Detect picks a parser by trying each one against sample lines.
//
// Detection is decided once per file rather than per line, and it takes a
// majority rather than the first success. One JSON-shaped line in a logfmt
// file -- an embedded payload, say -- should not flip the format for
// everything after it.
func Detect(sample []string) Parser {
	best, bestScore := Parser(nil), 0

	for _, p := range All {
		score := 0
		for _, line := range sample {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if _, ok := p.Parse(line); ok {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = p, score
		}
	}
	return best
}

// Apply runs a parser over a line, tolerating the ones it cannot read.
//
// A nil parser means detection found nothing structured, which is a normal
// outcome for a plain text log: the line still comes back, just with no
// fields to query.
func Apply(p Parser, line string) map[string]string {
	if p == nil {
		return nil
	}
	fields, ok := p.Parse(line)
	if !ok {
		return nil
	}
	return fields
}
