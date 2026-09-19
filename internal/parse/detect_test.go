package parse

import "testing"

func TestDetect(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sample []string
		want   string // parser name, or "" for none
	}{
		{
			name:   "json file",
			sample: []string{`{"level":"info"}`, `{"level":"error"}`},
			want:   "json",
		},
		{
			name:   "logfmt file",
			sample: []string{"level=info msg=up", "level=error code=500"},
			want:   "logfmt",
		},
		{
			name:   "unstructured file",
			sample: []string{"Starting server...", "Listening on :8080"},
			want:   "",
		},
		{
			name:   "blank lines are ignored",
			sample: []string{"", "   ", `{"level":"info"}`},
			want:   "json",
		},
		{
			// The reason detection votes instead of taking the first hit.
			name: "one stray JSON line does not flip a logfmt file",
			sample: []string{
				"level=info msg=start",
				`{"embedded":"payload"}`,
				"level=info msg=work",
				"level=error msg=fail",
			},
			want: "logfmt",
		},
		{name: "no sample at all", sample: nil, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Detect(tc.sample)
			name := ""
			if got != nil {
				name = got.Name()
			}
			if name != tc.want {
				t.Fatalf("Detect = %q, want %q", name, tc.want)
			}
		})
	}
}

func TestByName(t *testing.T) {
	for _, name := range []string{"json", "logfmt"} {
		p, err := ByName(name)
		if err != nil || p.Name() != name {
			t.Errorf("ByName(%q) = %v, %v", name, p, err)
		}
	}
	if _, err := ByName("xml"); err == nil {
		t.Error("an unknown format should be an error, not a silent fallback")
	}
}

func TestApplyToleratesUnreadableLines(t *testing.T) {
	if got := Apply(nil, "anything"); got != nil {
		t.Errorf("a nil parser should yield no fields, got %v", got)
	}
	if got := Apply(LogFmt{}, "a stack trace line"); got != nil {
		t.Errorf("an unparseable line should yield no fields, got %v", got)
	}
	if got := Apply(LogFmt{}, "level=info"); got["level"] != "info" {
		t.Errorf("a parseable line should yield fields, got %v", got)
	}
}
