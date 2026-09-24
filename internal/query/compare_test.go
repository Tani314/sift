package query

import "testing"

// TestNumericComparison pins the bug that motivated numeric coercion:
// lexical ordering says "100" < "90".
func TestNumericComparison(t *testing.T) {
	r := rec("dur=100", "dur", "100", "code", "500", "ratio", "0.25")

	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"dur>90", true}, // the regression: lexically "100" < "90"
		{"dur>100", false},
		{"dur>=100", true},
		{"dur<200", true},
		{"dur<100", false},
		{"dur<=100", true},
		{"code>=500", true},
		{"ratio<0.5", true},
		{"ratio>0.5", false},
	} {
		if got := match(t, tc.query, r); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestDurationComparison(t *testing.T) {
	r := rec("dur=1500ms", "dur", "1500ms")

	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"dur>1s", true},
		{"dur<2s", true},
		{"dur>2s", false},
		{"dur>=1500ms", true},
	} {
		if got := match(t, tc.query, r); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.query, got, tc.want)
		}
	}
}

// TestTextComparisonWhenNotNumeric checks the fallback still works, since
// ordering strings is genuinely useful for timestamps and levels.
func TestTextComparisonWhenNotNumeric(t *testing.T) {
	r := rec("ts=2026-09-23T10:00:00Z", "ts", "2026-09-23T10:00:00Z", "level", "error")

	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"ts>2026-09-23T09:00:00Z", true},
		{"ts<2026-09-23T11:00:00Z", true},
		{"ts>2026-09-24T00:00:00Z", false},
		{"level>debug", true}, // "error" sorts after "debug"
	} {
		if got := match(t, tc.query, r); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.query, got, tc.want)
		}
	}
}

// TestMixedOperandsFallBackToText checks the all-or-nothing rule: a number
// against a word has no numeric answer.
func TestMixedOperandsFallBackToText(t *testing.T) {
	r := rec("code=500", "code", "500")

	// "500" < "abc" lexically, so this is the text answer, not a numeric one.
	if !match(t, "code<abc", r) {
		t.Error("a numeric field against a word should compare as text")
	}
}

func TestRegexMatching(t *testing.T) {
	r := rec("msg=connection refused", "msg", "connection refused", "path", "/health")

	for _, tc := range []struct {
		query string
		want  bool
	}{
		{`msg=~"refused|timeout"`, true},
		{`msg=~"^connection"`, true},
		{`msg=~"^refused"`, false},
		{`msg!~"accepted"`, true},
		{`msg!~"refused"`, false},
		{`path=~"^/health$"`, true},
	} {
		if got := match(t, tc.query, r); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestRegexOnMissingFieldNeverMatches(t *testing.T) {
	r := rec("level=info", "level", "info")

	if match(t, `msg=~".*"`, r) {
		t.Error(`msg=~".*" matched a record with no msg field`)
	}
	// Even the negated form: absent is absent, not "does not match".
	if match(t, `msg!~"anything"`, r) {
		t.Error("a negated regex should not match a record with no such field")
	}
}

func TestValidateRejectsBadPatternsUpFront(t *testing.T) {
	expr, err := Parse(`msg=~"(unclosed"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(expr); err == nil {
		t.Fatal("Validate should reject an invalid pattern before any input is read")
	}

	good, err := Parse(`msg=~"fine" and level=error`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(good); err != nil {
		t.Fatalf("Validate rejected a valid query: %v", err)
	}
}

func TestValidateWalksNestedExpressions(t *testing.T) {
	expr, err := Parse(`level=error and not (svc=api or msg=~"(bad")`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(expr); err == nil {
		t.Fatal("Validate should reach patterns nested inside not(...) and or(...)")
	}
}
