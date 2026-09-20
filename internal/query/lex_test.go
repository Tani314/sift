package query

import "testing"

// kinds renders a token stream compactly so a table test can compare shapes.
func kinds(tokens []Token) []Kind {
	out := make([]Kind, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, t.Kind)
	}
	return out
}

func texts(tokens []Token) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t.Kind != EOF {
			out = append(out, t.Text)
		}
	}
	return out
}

func TestLex(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple comparison",
			input: "level=error",
			want:  []string{"level", "=", "error"},
		},
		{
			// The reason operators are matched longest-first.
			name:  "two-character operators stay whole",
			input: "dur>=100 and code<=500",
			want:  []string{"dur", ">=", "100", "and", "code", "<=", "500"},
		},
		{
			name:  "regex operators",
			input: `msg=~"time.*out" and path!~/health`,
			want:  []string{"msg", "=~", "time.*out", "and", "path", "!~", "/health"},
		},
		{
			name:  "no spaces needed around operators",
			input: "a=1 and(b=2 or c=3)",
			want:  []string{"a", "=", "1", "and", "(", "b", "=", "2", "or", "c", "=", "3", ")"},
		},
		{
			name:  "quoted value keeps its spaces",
			input: `msg="connection refused"`,
			want:  []string{"msg", "=", "connection refused"},
		},
		{
			name:  "single quotes work too",
			input: `msg='has "quotes" inside'`,
			want:  []string{"msg", "=", `has "quotes" inside`},
		},
		{
			name:  "keywords are case insensitive",
			input: "a=1 AND b=2 Or NOT c=3",
			want:  []string{"a", "=", "1", "AND", "b", "=", "2", "Or", "NOT", "c", "=", "3"},
		},
		{
			name:  "empty input",
			input: "",
			want:  []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := Lex(tc.input)
			if err != nil {
				t.Fatalf("Lex(%q): %v", tc.input, err)
			}
			got := texts(tokens)
			if len(got) != len(tc.want) {
				t.Fatalf("tokens = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("token %d = %q, want %q (full: %q)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

func TestLexKeywordsBecomeOperators(t *testing.T) {
	tokens, err := Lex("a=1 and b=2")
	if err != nil {
		t.Fatal(err)
	}
	want := []Kind{Ident, Op, Ident, And, Ident, Op, Ident, EOF}
	got := kinds(tokens)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kind %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLexErrors(t *testing.T) {
	if _, err := Lex(`msg="never closed`); err == nil {
		t.Error("an unterminated string should be an error")
	}
}

func TestLexRecordsPositions(t *testing.T) {
	tokens, err := Lex("level=error")
	if err != nil {
		t.Fatal(err)
	}
	// Positions are what lets a parse error point at the offending character.
	if tokens[0].Pos != 0 || tokens[1].Pos != 5 || tokens[2].Pos != 6 {
		t.Fatalf("positions = %d, %d, %d; want 0, 5, 6", tokens[0].Pos, tokens[1].Pos, tokens[2].Pos)
	}
}
