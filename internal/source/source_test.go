package source

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write creates a file in a temp dir and returns its path.
func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// writeGzip creates a gzipped file and returns its path.
func writeGzip(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(content)); err != nil {
		t.Fatalf("compressing: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing gzip: %v", err)
	}
	return path
}

// collect gathers every line a scan produces.
func collect(t *testing.T, path string) []Line {
	t.Helper()
	var lines []Line
	if err := Scan(path, func(l Line) error {
		lines = append(lines, l)
		return nil
	}); err != nil {
		t.Fatalf("Scan(%s): %v", path, err)
	}
	return lines
}

func TestScanNumbersLines(t *testing.T) {
	path := write(t, "app.log", "first\nsecond\nthird\n")
	lines := collect(t, path)

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for i, want := range []string{"first", "second", "third"} {
		if lines[i].Text != want {
			t.Errorf("line %d text = %q, want %q", i, lines[i].Text, want)
		}
		if lines[i].Num != i+1 {
			t.Errorf("line %d numbered %d, want %d (1-based)", i, lines[i].Num, i+1)
		}
		if lines[i].Source != path {
			t.Errorf("line %d source = %q, want %q", i, lines[i].Source, path)
		}
	}
}

func TestScanHandlesMissingTrailingNewline(t *testing.T) {
	lines := collect(t, write(t, "app.log", "first\nsecond"))
	if len(lines) != 2 || lines[1].Text != "second" {
		t.Fatalf("a file with no trailing newline lost its last line: %v", lines)
	}
}

func TestScanEmptyFile(t *testing.T) {
	if lines := collect(t, write(t, "empty.log", "")); len(lines) != 0 {
		t.Fatalf("an empty file produced %d lines", len(lines))
	}
}

func TestScanGzip(t *testing.T) {
	path := writeGzip(t, "app.log.gz", "level=info msg=up\nlevel=error msg=down\n")
	lines := collect(t, path)

	if len(lines) != 2 {
		t.Fatalf("got %d lines from a gzipped file, want 2", len(lines))
	}
	if !strings.Contains(lines[1].Text, "down") {
		t.Errorf("second line = %q", lines[1].Text)
	}
}

// TestScanLongLines covers the reason the scanner's buffer is raised: the
// default would truncate and silently stop matching.
func TestScanLongLines(t *testing.T) {
	long := strings.Repeat("x", 200*1024)
	lines := collect(t, write(t, "big.log", "short\n"+long+"\n"))

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if len(lines[1].Text) != len(long) {
		t.Fatalf("long line truncated to %d bytes, want %d", len(lines[1].Text), len(long))
	}
}

func TestScanStopsOnHandlerError(t *testing.T) {
	path := write(t, "app.log", "one\ntwo\nthree\n")
	seen := 0

	err := Scan(path, func(Line) error {
		seen++
		if seen == 2 {
			return os.ErrClosed
		}
		return nil
	})
	if err == nil {
		t.Fatal("a handler error should stop the scan and surface")
	}
	if seen != 2 {
		t.Fatalf("handler ran %d times, want 2 -- the scan should stop immediately", seen)
	}
}

func TestScanMissingFile(t *testing.T) {
	if err := Scan(filepath.Join(t.TempDir(), "nope.log"), func(Line) error { return nil }); err == nil {
		t.Fatal("scanning a missing file should fail")
	}
}

func TestScanAllKeepsSourcesDistinct(t *testing.T) {
	a := write(t, "a.log", "from a\n")
	b := write(t, "b.log", "from b\n")

	var sources []string
	if err := ScanAll([]string{a, b}, func(l Line) error {
		sources = append(sources, l.Source)
		// Both files have one line, so both lines are numbered 1: numbering
		// is per file, not global.
		if l.Num != 1 {
			t.Errorf("line from %s numbered %d, want 1", l.Source, l.Num)
		}
		return nil
	}); err != nil {
		t.Fatalf("ScanAll: %v", err)
	}

	if len(sources) != 2 || sources[0] != a || sources[1] != b {
		t.Fatalf("sources = %v, want [%s %s] in order", sources, a, b)
	}
}

func TestSampleStopsEarly(t *testing.T) {
	path := write(t, "app.log", "1\n2\n3\n4\n5\n6\n7\n8\n")

	got, err := Sample(path, 3)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Sample returned %d lines, want 3", len(got))
	}
}

func TestSampleSkipsBlankLines(t *testing.T) {
	path := write(t, "app.log", "\n   \nreal line\n\nanother\n")

	got, err := Sample(path, 10)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Sample returned %v, want only the two non-blank lines", got)
	}
}

func TestSampleDeclinesStdin(t *testing.T) {
	// Standard input cannot be rewound, so sampling it would eat the first
	// lines of the pipe.
	got, err := Sample(Stdin, 5)
	if err != nil || got != nil {
		t.Fatalf("Sample(stdin) = %v, %v; want nil, nil", got, err)
	}
}
