package render

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Tani314/sift/internal/record"
)

// Table aligns records into columns.
//
// This is the only renderer that buffers, and it has to: column widths are
// not known until the widest value has been seen. That makes it unsuitable
// for following a live log, which is why Raw stays the default.
type Table struct {
	// Names are the columns, in order. When empty, the columns are the
	// union of every field seen, sorted -- useful for a first look at a log
	// whose shape you do not know yet.
	Names []string

	rows []record.Record
	seen map[string]bool
}

// Write buffers a record.
func (t *Table) Write(_ io.Writer, r record.Record) error {
	if t.seen == nil {
		t.seen = map[string]bool{}
	}
	for _, k := range r.Keys() {
		t.seen[k] = true
	}
	t.rows = append(t.rows, r)
	return nil
}

// Flush writes the aligned table.
func (t *Table) Flush(w io.Writer) error {
	columns := t.columns()
	if len(columns) == 0 || len(t.rows) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	for i, c := range columns {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, upper(c))
	}
	fmt.Fprintln(tw)

	for _, r := range t.rows {
		for i, c := range columns {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			v, ok := r.Get(c)
			if !ok {
				v = "-"
			}
			fmt.Fprint(tw, v)
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

// columns resolves which columns to print.
func (t *Table) columns() []string {
	if len(t.Names) > 0 {
		return t.Names
	}
	out := make([]string, 0, len(t.seen))
	for k := range t.seen {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// upper renders a column heading.
func upper(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		out = append(out, r)
	}
	return string(out)
}

// sortStrings is a small insertion sort, kept local so the package pulls in
// nothing for a list that is never longer than a log line has fields.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
