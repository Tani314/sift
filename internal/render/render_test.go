package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Tani314/sift/internal/record"
)

func rec(line string, kv ...string) record.Record {
	fields := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		fields[kv[i]] = kv[i+1]
	}
	return record.Record{Line: line, Fields: fields, Source: "app.log", Num: 7}
}

// render runs a renderer over records and returns the output.
func render(t *testing.T, r Renderer, records ...record.Record) string {
	t.Helper()
	var b strings.Builder
	for _, rec := range records {
		if err := r.Write(&b, rec); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := r.Flush(&b); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	return b.String()
}

func TestRawReproducesTheLine(t *testing.T) {
	line := `level=error msg="connection refused"`
	got := render(t, Raw{}, rec(line, "level", "error"))

	if got != line+"\n" {
		t.Fatalf("Raw output = %q, want the original line verbatim", got)
	}
}

func TestRawWithSource(t *testing.T) {
	got := render(t, Raw{ShowSource: true}, rec("hello", "a", "1"))

	if !strings.HasPrefix(got, "app.log:7: ") {
		t.Fatalf("output = %q, want a file:line prefix", got)
	}
}

func TestFieldsPrintsSelectedValuesInOrder(t *testing.T) {
	r := rec("x", "level", "error", "svc", "api", "code", "500")
	got := render(t, Fields{Names: []string{"svc", "level"}}, r)

	if got != "api  error\n" {
		t.Fatalf("output = %q, want the named fields in the order given", got)
	}
}

func TestFieldsMarksMissingValues(t *testing.T) {
	got := render(t, Fields{Names: []string{"level", "absent"}}, rec("x", "level", "info"))

	if got != "info  -\n" {
		t.Fatalf("output = %q, want a placeholder so columns stay aligned", got)
	}
}

func TestTableAlignsColumns(t *testing.T) {
	got := render(t, &Table{Names: []string{"level", "msg"}},
		rec("a", "level", "error", "msg", "boom"),
		rec("b", "level", "info", "msg", "ok"),
	)

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want a header plus two rows:\n%s", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "LEVEL") {
		t.Errorf("header = %q, want upper-cased column names", lines[0])
	}
	// Alignment means the msg column starts at the same offset on both rows.
	if strings.Index(lines[1], "boom") != strings.Index(lines[2], "ok") {
		t.Errorf("columns are not aligned:\n%s", got)
	}
}

func TestTableDiscoversColumnsWhenNoneNamed(t *testing.T) {
	got := render(t, &Table{},
		rec("a", "level", "error"),
		rec("b", "svc", "api"),
	)

	// Union of both records' fields, sorted.
	if !strings.HasPrefix(got, "LEVEL  SVC") {
		t.Fatalf("header = %q, want the sorted union of all fields", strings.SplitN(got, "\n", 2)[0])
	}
	if !strings.Contains(got, "-") {
		t.Error("a record missing a discovered column should show a placeholder")
	}
}

func TestTableEmptyInputPrintsNothing(t *testing.T) {
	if got := render(t, &Table{}); got != "" {
		t.Fatalf("output = %q, want nothing at all when no records matched", got)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	got := render(t, JSON{}, rec("x", "level", "error", "code", "500"))

	var back map[string]any
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, got)
	}
	if back["level"] != "error" || back["code"] != "500" {
		t.Fatalf("decoded = %v", back)
	}
}

func TestJSONKeepsUnparsedLines(t *testing.T) {
	got := render(t, JSON{}, record.Record{Line: "    at Thing.java:42", Source: "app.log", Num: 3})

	var back map[string]any
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if back["message"] != "    at Thing.java:42" {
		t.Fatalf("an unparsed line should survive as message, got %v", back)
	}
}

func TestJSONMeta(t *testing.T) {
	got := render(t, JSON{Meta: true}, rec("x", "level", "info"))

	var back map[string]any
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if back["_source"] != "app.log" || back["_line"] != float64(7) {
		t.Fatalf("metadata missing or wrong: %v", back)
	}
}
