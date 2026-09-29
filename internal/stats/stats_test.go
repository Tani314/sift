package stats

import (
	"strings"
	"testing"

	"github.com/Tani314/sift/internal/record"
)

func rec(kv ...string) record.Record {
	fields := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		fields[kv[i]] = kv[i+1]
	}
	return record.Record{Fields: fields}
}

func count(values ...string) *Counter {
	c := New("level")
	for _, v := range values {
		if v == "" {
			c.Add(record.Record{})
			continue
		}
		c.Add(rec("level", v))
	}
	return c
}

func TestTopOrdersByFrequency(t *testing.T) {
	c := count("error", "info", "error", "warn", "error", "info")

	top := c.Top(0)
	if len(top) != 3 {
		t.Fatalf("got %d entries, want 3", len(top))
	}
	if top[0].Value != "error" || top[0].Count != 3 {
		t.Fatalf("first entry = %+v, want error x3", top[0])
	}
	if top[1].Value != "info" || top[1].Count != 2 {
		t.Fatalf("second entry = %+v, want info x2", top[1])
	}
}

// TestTiesBreakAlphabetically is what makes two runs diffable.
func TestTiesBreakAlphabetically(t *testing.T) {
	c := count("warn", "error", "info")

	top := c.Top(0)
	for i, want := range []string{"error", "info", "warn"} {
		if top[i].Value != want {
			t.Fatalf("entry %d = %q, want %q -- equal counts must sort by value", i, top[i].Value, want)
		}
	}
}

func TestTopLimits(t *testing.T) {
	c := count("a", "b", "c", "d", "e")

	if got := len(c.Top(2)); got != 2 {
		t.Fatalf("Top(2) returned %d entries", got)
	}
	if got := len(c.Top(0)); got != 5 {
		t.Fatalf("Top(0) returned %d entries, want all of them", got)
	}
	if got := len(c.Top(99)); got != 5 {
		t.Fatalf("Top(99) returned %d entries, want all 5", got)
	}
}

func TestShares(t *testing.T) {
	c := count("error", "error", "info", "info")

	for _, e := range c.Top(0) {
		if e.Share != 0.5 {
			t.Fatalf("%s share = %v, want 0.5", e.Value, e.Share)
		}
	}
}

// TestMissingFieldIsCounted covers the bucket that usually holds the answer.
func TestMissingFieldIsCounted(t *testing.T) {
	c := count("error", "", "", "info")

	var missing int
	for _, e := range c.Top(0) {
		if e.Value == Missing {
			missing = e.Count
		}
	}
	if missing != 2 {
		t.Fatalf("records with no level counted %d, want 2", missing)
	}
	if c.Total() != 4 {
		t.Fatalf("total = %d, want 4", c.Total())
	}
	if c.Distinct() != 3 {
		t.Fatalf("distinct = %d, want 3 (error, info and the missing bucket)", c.Distinct())
	}
}

func TestEmptyCounter(t *testing.T) {
	c := New("level")

	if c.Total() != 0 || c.Distinct() != 0 || len(c.Top(0)) != 0 {
		t.Fatal("a counter with nothing added should be empty")
	}

	var b strings.Builder
	if err := c.Render(&b, 0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(b.String(), "no records matched") {
		t.Fatalf("empty render = %q, want a clear note rather than blank output", b.String())
	}
}

func TestRender(t *testing.T) {
	c := count("error", "error", "error", "info")

	var b strings.Builder
	if err := c.Render(&b, 0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := b.String()

	for _, want := range []string{"error", "75.0%", "info", "25.0%", "4 records", "2 distinct level"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "#") {
		t.Errorf("output has no histogram bar:\n%s", out)
	}
}

func TestRenderIsStable(t *testing.T) {
	first, second := &strings.Builder{}, &strings.Builder{}
	count("a", "b", "c", "a").Render(first, 0)
	count("a", "b", "c", "a").Render(second, 0)

	if first.String() != second.String() {
		t.Fatalf("two runs over the same data differ:\n%s\n---\n%s", first, second)
	}
}
