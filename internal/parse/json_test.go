package parse

import (
	"reflect"
	"testing"
)

func TestJSONParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want map[string]string
		ok   bool
	}{
		{
			name: "flat object",
			line: `{"level":"error","msg":"boom"}`,
			want: map[string]string{"level": "error", "msg": "boom"},
			ok:   true,
		},
		{
			name: "integers keep their integer form",
			line: `{"status":500,"dur":1.5}`,
			want: map[string]string{"status": "500", "dur": "1.5"},
			ok:   true,
		},
		{
			name: "nested objects flatten with dots",
			line: `{"http":{"status":404,"method":"GET"},"level":"warn"}`,
			want: map[string]string{"http.status": "404", "http.method": "GET", "level": "warn"},
			ok:   true,
		},
		{
			name: "deeply nested",
			line: `{"a":{"b":{"c":"deep"}}}`,
			want: map[string]string{"a.b.c": "deep"},
			ok:   true,
		},
		{
			name: "booleans and nulls",
			line: `{"ok":true,"err":null}`,
			want: map[string]string{"ok": "true", "err": ""},
			ok:   true,
		},
		{
			name: "arrays keep their JSON form",
			line: `{"tags":["a","b"]}`,
			want: map[string]string{"tags": `["a","b"]`},
			ok:   true,
		},
		{name: "not json", line: "level=error msg=boom", ok: false},
		{name: "truncated json", line: `{"level":"err`, ok: false},
		{name: "json array, not an object", line: `[1,2,3]`, ok: false},
		{name: "empty object has nothing to offer", line: `{}`, ok: false},
		{name: "empty line", line: "", ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := JSON{}.Parse(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (fields: %v)", ok, tc.ok, got)
			}
			if tc.ok && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("fields = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJSONHandlesLeadingWhitespace(t *testing.T) {
	got, ok := JSON{}.Parse("   " + `{"level":"info"}`)
	if !ok || got["level"] != "info" {
		t.Fatalf("indented JSON should still parse: %v, %v", got, ok)
	}
}
