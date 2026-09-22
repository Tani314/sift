package query

import (
	"fmt"
	"strings"

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
	case "<", "<=", ">", ">=":
		return compareOrdered(c.Op, got, c.Value)
	}
	return false, fmt.Errorf("unknown operator %q", c.Op)
}

// compareOrdered handles the ordering operators.
func compareOrdered(op, got, want string) (bool, error) {
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
