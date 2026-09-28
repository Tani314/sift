// Package stats summarises matching records.
//
// The question that follows "show me the errors" is almost always "which
// ones, and how many" -- and the usual answer is a pipeline of awk, sort and
// uniq -c that nobody remembers the shape of. Counting is common enough to
// belong in the tool.
package stats

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Tani314/sift/internal/record"
)

// Missing is the bucket for records that lack the counted field. It is
// counted rather than dropped: "how many lines had no status at all" is
// usually the interesting part of the answer.
const Missing = "(none)"

// Counter tallies values of one field.
type Counter struct {
	// Field is the field being counted.
	Field string

	counts map[string]int
	total  int
}

// New builds a Counter for a field.
func New(field string) *Counter {
	return &Counter{Field: field, counts: map[string]int{}}
}

// Add tallies one record.
func (c *Counter) Add(r record.Record) {
	v, ok := r.Get(c.Field)
	if !ok {
		v = Missing
	}
	c.counts[v]++
	c.total++
}

// Total is the number of records counted.
func (c *Counter) Total() int { return c.total }

// Distinct is the number of different values seen.
func (c *Counter) Distinct() int { return len(c.counts) }

// Entry is one value and its tally.
type Entry struct {
	Value string
	Count int
	Share float64 // fraction of the total, 0..1
}

// Top returns the n most common values, most frequent first.
//
// Ties break alphabetically so repeated runs over the same data produce
// identical output -- a summary that reshuffles itself is impossible to
// diff between two log files.
func (c *Counter) Top(n int) []Entry {
	entries := make([]Entry, 0, len(c.counts))
	for v, count := range c.counts {
		share := 0.0
		if c.total > 0 {
			share = float64(count) / float64(c.total)
		}
		entries = append(entries, Entry{Value: v, Count: count, Share: share})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Value < entries[j].Value
	})

	if n > 0 && len(entries) > n {
		entries = entries[:n]
	}
	return entries
}

// barWidth is the width of the histogram in characters.
const barWidth = 32

// Render writes the summary as a small histogram.
func (c *Counter) Render(w io.Writer, n int) error {
	entries := c.Top(n)
	if len(entries) == 0 {
		_, err := fmt.Fprintf(w, "no records matched\n")
		return err
	}

	widest := 0
	for _, e := range entries {
		if len(e.Value) > widest {
			widest = len(e.Value)
		}
	}

	for _, e := range entries {
		bar := strings.Repeat("#", int(e.Share*barWidth))
		if _, err := fmt.Fprintf(w, "%-*s  %6d  %5.1f%%  %s\n",
			widest, e.Value, e.Count, e.Share*100, bar); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(w, "\n%d records, %d distinct %s\n", c.total, c.Distinct(), c.Field)
	return err
}
