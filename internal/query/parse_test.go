package query

import "testing"

func TestParsePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single comparison",
			input: "level=error",
			want:  "level==error",
		},
		{
			name:  "and binds tighter than or",
			input: "level=error or level=warn and svc=api",
			want:  "(level==error or (level==warn and svc==api))",
		},
		{
			name:  "parentheses override precedence",
			input: "(level=error or level=warn) and svc=api",
			want:  "((level==error or level==warn) and svc==api)",
		},
		{
			name:  "not binds tightest",
			input: "not level=debug and svc=api",
			want:  "(not level==debug and svc==api)",
		},
		{
			name:  "not applies to a parenthesised group",
			input: "not (level=debug or level=trace)",
			want:  "not (level==debug or level==trace)",
		},
		{
			name:  "double negation",
			input: "not not level=error",
			want:  "not not level==error",
		},
		{
			name:  "bare word is a substring match",
			input: "timeout",
			want:  "timeout",
		},
		{
			name:  "bare word combines with comparisons",
			input: "timeout and level=error",
			want:  "(timeout and level==error)",
		},
		{
			name:  "= and == mean the same thing",
			input: "level==error",
			want:  "level==error",
		},
		{
			name:  "left associative chain",
			input: "a=1 and b=2 and c=3",
			want:  "((a==1 and b==2) and c==3)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.input, err)
			}
			if got := expr.String(); got != tc.want {
				t.Fatalf("Parse(%q) = %s, want %s", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"empty query", ""},
		{"dangling operator", "level="},
		{"dangling and", "level=error and"},
		{"missing close paren", "(level=error"},
		{"stray close paren", "level=error)"},
		{"operator with no field", "=error"},
		{"nothing but not", "not"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if expr, err := Parse(tc.input); err == nil {
				t.Fatalf("Parse(%q) should have failed, got %s", tc.input, expr)
			}
		})
	}
}

// TestParseErrorsPointAtTheProblem checks the message is useful, since a
// query is usually typed by hand at a terminal.
func TestParseErrorsPointAtTheProblem(t *testing.T) {
	_, err := Parse("level=error)")
	if err == nil {
		t.Fatal("expected an error")
	}
	if want := "position 11"; !contains(err.Error(), want) {
		t.Fatalf("error %q does not mention %q", err, want)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
