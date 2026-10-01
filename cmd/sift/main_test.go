package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sample is a log with the awkward parts real logs have: mixed levels, a
// stack trace line that parses as nothing, and durations in several units.
const sample = `ts=2026-09-30T09:00:00Z level=info svc=api msg="server started" dur=2ms
ts=2026-09-30T09:00:04Z level=error svc=api msg="connection refused" dur=1500ms status=500
ts=2026-09-30T09:00:07Z level=warn svc=worker msg="retry scheduled" dur=90ms
    at com.example.Thing.method(Thing.java:42)
ts=2026-09-30T09:01:00Z level=error svc=worker msg="timeout waiting for lock" dur=3s status=504
ts=2026-09-30T09:01:30Z level=info svc=api msg="request ok" dur=12ms status=200
`

// build compiles sift once and returns the binary path. Driving the real
// binary is the only way to cover argument handling and exit codes, which is
// where the flag-ordering bug lived.
func build(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sift")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("building sift: %v\n%s", err, out)
	}
	return bin
}

func logFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatalf("writing log: %v", err)
	}
	return path
}

// runSift executes sift and returns stdout and the exit code.
func runSift(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &strings.Builder{}

	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("running sift: %v", err)
	}
	return stdout.String(), code
}

func lines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func TestFilters(t *testing.T) {
	bin, log := build(t), logFile(t)

	for _, tc := range []struct {
		name  string
		query string
		want  int
	}{
		{"equality", "level=error", 2},
		{"inequality", "level!=info", 3}, // the stack trace has no level, so it does not match
		{"boolean and", "level=error and svc=api", 1},
		{"boolean or", "level=error or level=warn", 3},
		{"negation", "not level=info", 4}, // but negation does match it: level=info is false there
		{"regex", `msg=~"timeout|refused"`, 2},
		{"substring", "Thing.java", 1},
		{"duration compare", "dur>1s", 2},
		{"numeric compare", "status>=500", 2},
		{"grouping", "(level=error or level=warn) and svc=worker", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := runSift(t, bin, tc.query, log)
			if got := len(lines(out)); got != tc.want {
				t.Fatalf("%s matched %d lines, want %d:\n%s", tc.query, got, tc.want, out)
			}
		})
	}
}

// TestFlagsAfterQuery covers the bug that the unit tests could not see: Go's
// flag package stops at the first positional argument, so every flag after
// the query was being read as a filename.
func TestFlagsAfterQuery(t *testing.T) {
	bin, log := build(t), logFile(t)

	out, code := runSift(t, bin, "dur>1s", "--fields", "svc,dur", log)
	if code != 0 {
		t.Fatalf("exit %d, want 0:\n%s", code, out)
	}
	got := lines(out)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(got), out)
	}
	if !strings.Contains(got[0], "api") || !strings.Contains(got[0], "1500ms") {
		t.Fatalf("first line = %q, want the selected fields only", got[0])
	}
}

func TestFlagsBeforeQueryAlsoWork(t *testing.T) {
	bin, log := build(t), logFile(t)

	out, _ := runSift(t, bin, "--fields", "svc", "dur>1s", log)
	if len(lines(out)) != 2 {
		t.Fatalf("flags before the query should work too:\n%s", out)
	}
}

func TestExitCodesFollowGrep(t *testing.T) {
	bin, log := build(t), logFile(t)

	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"match", []string{"level=error", log}, 0},
		{"no match", []string{"level=absent", log}, 1},
		{"bad query", []string{"level=", log}, 2},
		{"bad pattern", []string{`msg=~"(unclosed"`, log}, 2},
		{"missing file", []string{"level=error", log + ".nope"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, code := runSift(t, bin, tc.args...); code != tc.want {
				t.Fatalf("exit %d, want %d", code, tc.want)
			}
		})
	}
}

func TestLimitStopsEarly(t *testing.T) {
	bin, log := build(t), logFile(t)

	out, code := runSift(t, bin, "level!=nothing", "--limit", "2", log)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := len(lines(out)); got != 2 {
		t.Fatalf("got %d lines, want exactly the 2 asked for", got)
	}
}

func TestCountSummarises(t *testing.T) {
	bin, log := build(t), logFile(t)

	out, _ := runSift(t, bin, "level=error", "--count", "svc", log)
	for _, want := range []string{"api", "worker", "2 records", "distinct svc"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestReadsStdin(t *testing.T) {
	bin := build(t)

	cmd := exec.Command(bin, "level=error")
	cmd.Stdin = strings.NewReader(sample)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running against stdin: %v", err)
	}
	if got := len(lines(string(out))); got != 2 {
		t.Fatalf("got %d lines from stdin, want 2:\n%s", got, out)
	}
}

func TestNoArgumentsExplainsItself(t *testing.T) {
	bin := build(t)

	cmd := exec.Command(bin)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	_ = cmd.Run()

	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("running with no arguments should print usage, got:\n%s", stderr.String())
	}
}
