package record

import (
	"reflect"
	"testing"
)

func TestKeysAreSorted(t *testing.T) {
	r := Record{Fields: map[string]string{"level": "error", "at": "12:00", "msg": "boom"}}
	want := []string{"at", "level", "msg"}
	if got := r.Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys() = %v, want %v -- map order must not leak into output", got, want)
	}
}

func TestGet(t *testing.T) {
	r := Record{Fields: map[string]string{"level": "error"}}

	if v, ok := r.Get("level"); !ok || v != "error" {
		t.Fatalf("Get(level) = %q, %v", v, ok)
	}
	if _, ok := r.Get("absent"); ok {
		t.Fatal("Get reported a field that is not there")
	}
}

func TestParsed(t *testing.T) {
	if (Record{Line: "not structured at all"}).Parsed() {
		t.Error("a line with no fields should not count as parsed")
	}
	if !(Record{Fields: map[string]string{"a": "1"}}).Parsed() {
		t.Error("a line with fields should count as parsed")
	}
	// An empty (non-nil) map is still nothing parsed.
	if (Record{Fields: map[string]string{}}).Parsed() {
		t.Error("an empty field map should not count as parsed")
	}
}
