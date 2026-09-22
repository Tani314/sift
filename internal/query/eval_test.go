package query

import (
	"testing"

	"github.com/Tani314/sift/internal/record"
)

func rec(line string, kv ...string) record.Record {
	fields := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		fields[kv[i]] = kv[i+1]
	}
	return record.Record{Line: line, Fields: fields}
}

// match is a helper that parses and evaluates in one step, as the CLI does.
func match(t *testing.T, q string, r record.Record) bool {
	t.Helper()
	expr, err := Parse(q)
	if err != nil {
		t.Fatalf("Parse(%q): %v", q, err)
	}
	ok, err := Eval(expr, r)
	if err != nil {
		t.Fatalf("Eval(%q): %v", q, err)
	}
	return ok
}

func TestEvalComparisons(t *testing.T) {
	r := rec("level=error code=500 msg=boom", "level", "error", "code", "500", "msg", "boom")

	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"level=error", true},
		{"level=info", false},
		{"level!=info", true},
		{"level!=error", false},
		{"code=500", true},
	} {
		if got := match(t, tc.query, r); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.query, got, tc.want)
		}
	}
}

// TestEvalMissingFieldNeverMatches is the property that keeps queries usable
// against real, ragged log files.
func TestEvalMissingFieldNeverMatches(t *testing.T) {
	r := rec("level=info", "level", "info")

	for _, q := range []string{"dur>100", "dur<100", "dur=5", "svc=api"} {
		if match(t, q, r) {
			t.Errorf("%s matched a record that has no such field", q)
		}
	}
	// Including the negated form: "not dur>100" is true because dur>100 is
	// false, which is the behaviour a filter pipeline expects.
	if !match(t, "not dur>100", r) {
		t.Error("not <false> should be true even when the field is absent")
	}
}

func TestEvalBooleans(t *testing.T) {
	r := rec("level=error svc=api", "level", "error", "svc", "api")

	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"level=error and svc=api", true},
		{"level=error and svc=web", false},
		{"level=warn or svc=api", true},
		{"level=warn or svc=web", false},
		{"not level=warn", true},
		{"not level=error", false},
		{"(level=error or level=warn) and svc=api", true},
		{"level=error and (svc=web or svc=api)", true},
		{"not (level=error and svc=api)", false},
	} {
		if got := match(t, tc.query, r); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestEvalBareWordSearchesTheWholeLine(t *testing.T) {
	r := rec("level=error msg=connection refused", "level", "error")

	if !match(t, "refused", r) {
		t.Error("a bare word should match anywhere in the raw line")
	}
	if match(t, "accepted", r) {
		t.Error("a bare word that is absent should not match")
	}
	// It searches the raw line, so it finds text in fields that were never
	// parsed out.
	if !match(t, "connection", r) {
		t.Error("a bare word should match unparsed text in the line")
	}
}

func TestEvalUnparsedRecordsStillSearchable(t *testing.T) {
	r := record.Record{Line: "    at com.example.Thing.method(Thing.java:42)"}

	if !match(t, "Thing.java", r) {
		t.Error("a line with no fields should still match a substring search")
	}
	if match(t, "level=error", r) {
		t.Error("a line with no fields should not match a field comparison")
	}
}
