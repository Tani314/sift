package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tani314/sift/internal/record"
)

// Eval reports whether a record satisfies the expression.
func Eval(e Expr, r record.Record) (bool, error) {
	switch n := e.(type) {
	case Compare:
		return evalCompare(n, r)
	case Match:
		return strings.Contains(r.Line, n.Text), nil
	case AndExpr:
		left, err := Eval(n.Left, r)
		if err != nil || !left {
			// Short-circuit: the right side may compile a regex, and there
			// is no reason to pay for it once the answer is settled.
			return false, err
		}
		return Eval(n.Right, r)
	case OrExpr:
		left, err := Eval(n.Left, r)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil
		}
		return Eval(n.Right, r)
	case NotExpr:
		inner, err := Eval(n.X, r)
		if err != nil {
			return false, err
		}
		return !inner, nil
	}
	return false, fmt.Errorf("cannot evaluate %T", e)
}

// evalCompare tests one field.
//
// A missing field never matches, and never errors. Logs are not uniform --
// half the lines in a file may lack the field being asked about -- so
// `dur>100` quietly skips lines with no dur rather than failing the query
// on the first one.
func evalCompare(c Compare, r record.Record) (bool, error) {
	got, ok := r.Get(c.Field)
	if !ok {
		return false, nil
	}

	switch c.Op {
	case "==":
		return got == c.Value, nil
	case "!=":
		return got != c.Value, nil
	case "=~", "!~":
		re, err := pattern(c.Value)
		if err != nil {
			return false, err
		}
		return re.MatchString(got) == (c.Op == "=~"), nil
	case "<", "<=", ">", ">=":
		return compareOrdered(c.Op, got, c.Value)
	}
	return false, fmt.Errorf("unknown operator %q", c.Op)
}

// compareOrdered handles the ordering operators.
//
// Numbers compare as numbers when both sides look numeric, and as text
// otherwise. Without this, dur>90 would miss a line with dur=100, because
// "100" sorts before "90" as a string -- the kind of wrong answer that is
// worse than an error, since nothing about the output says it happened.
//
// Durations with a unit suffix are normalised first, so dur>1s matches
// dur=1500ms.
func compareOrdered(op, got, want string) (bool, error) {
	if a, b, ok := asNumbers(got, want); ok {
		return compareFloats(op, a, b)
	}

	cmp := strings.Compare(got, want)

	switch op {
	case "<":
		return cmp < 0, nil
	case "<=":
		return cmp <= 0, nil
	case ">":
		return cmp > 0, nil
	case ">=":
		return cmp >= 0, nil
	}
	return false, fmt.Errorf("unknown operator %q", op)
}

// compareFloats applies an ordering operator to two numbers.
func compareFloats(op string, a, b float64) (bool, error) {
	switch op {
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	}
	return false, fmt.Errorf("unknown operator %q", op)
}

// asNumbers converts both operands to numbers, reporting whether both could
// be read that way. It is all-or-nothing on purpose: comparing a number
// against a word has no sensible numeric answer, so those fall back to text.
func asNumbers(got, want string) (float64, float64, bool) {
	a, okA := asNumber(got)
	b, okB := asNumber(want)
	return a, b, okA && okB
}

// asNumber reads a plain number, or a duration such as 1.5s or 250ms.
func asNumber(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n, true
	}
	if d, err := time.ParseDuration(s); err == nil {
		// Durations normalise to milliseconds, so 1s and 1000ms compare
		// equal regardless of which unit the log happened to use.
		return float64(d) / float64(time.Millisecond), true
	}
	return 0, false
}
