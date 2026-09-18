package parse

import (
	"reflect"
	"testing"
)

func TestLogFmtParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want map[string]string
		ok   bool
	}{
		{
			name: "plain pairs",
			line: "level=error code=500",
			want: map[string]string{"level": "error", "code": "500"},
			ok:   true,
		},
		{
			name: "quoted value with spaces",
			line: `level=error msg="connection refused"`,
			want: map[string]string{"level": "error", "msg": "connection refused"},
			ok:   true,
		},
		{
			name: "escaped quote inside a value",
			line: `msg="he said \"no\"" level=warn`,
			want: map[string]string{"msg": `he said "no"`, "level": "warn"},
			ok:   true,
		},
		{
			name: "empty value",
			line: "level=info msg=",
			want: map[string]string{"level": "info", "msg": ""},
			ok:   true,
		},
		{
			name: "tabs separate pairs too",
			line: "level=info\tcode=200",
			want: map[string]string{"level": "info", "code": "200"},
			ok:   true,
		},
		{
			// The case that matters: a stack trace must be refused outright,
			// not turned into a field.
			name: "prose is not logfmt",
			line: "    at com.example.Thing.method(Thing.java:42)",
			ok:   false,
		},
		{name: "bare words", line: "starting up now", ok: false},
		{name: "empty line", line: "", ok: false},
		{name: "value with no key", line: "=orphan", ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := LogFmt{}.Parse(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (fields: %v)", ok, tc.ok, got)
			}
			if tc.ok && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("fields = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLogFmtName(t *testing.T) {
	if got := (LogFmt{}).Name(); got != "logfmt" {
		t.Fatalf("Name() = %q", got)
	}
}
