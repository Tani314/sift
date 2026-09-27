package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Tani314/sift/internal/record"
)

// JSON emits one JSON object per line.
//
// This is the renderer that makes sift composable with jq and with anything
// else that reads a stream of objects. It normalises across input formats:
// a logfmt file and a JSON file both come out the same shape, so a pipeline
// downstream does not need to know which it was.
type JSON struct {
	// Meta adds the source file and line number to each object, for tracing
	// a result back to where it came from.
	Meta bool
}

// Write emits one object.
func (j JSON) Write(w io.Writer, r record.Record) error {
	out := make(map[string]any, len(r.Fields)+3)
	for k, v := range r.Fields {
		out[k] = v
	}

	if !r.Parsed() {
		// An unparsed line still has to round-trip, or piping through sift
		// would silently drop the stack traces.
		out["message"] = r.Line
	}
	if j.Meta {
		out["_source"] = r.Source
		out["_line"] = r.Num
	}

	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("encoding line %d of %s: %w", r.Num, r.Source, err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// Flush does nothing: JSON streams, one object per line.
func (JSON) Flush(io.Writer) error { return nil }
