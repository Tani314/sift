package query

import "strings"

// Expr is a node in a parsed filter.
type Expr interface {
	// String renders the expression back to source form, which makes both
	// tests and --explain output readable.
	String() string
}

// Compare tests one field against a literal value.
type Compare struct {
	Field string
	Op    string
	Value string
}

func (c Compare) String() string {
	if strings.ContainsAny(c.Value, " \t") {
		return c.Field + c.Op + `"` + c.Value + `"`
	}
	return c.Field + c.Op + c.Value
}

// Match is a bare word with no operator: a substring search over the whole
// line. It exists so `sift timeout app.log` works before anyone has learned
// the query language at all.
type Match struct {
	Text string
}

func (m Match) String() string { return m.Text }

// AndExpr is a conjunction.
type AndExpr struct {
	Left, Right Expr
}

func (a AndExpr) String() string { return "(" + a.Left.String() + " and " + a.Right.String() + ")" }

// OrExpr is a disjunction.
type OrExpr struct {
	Left, Right Expr
}

func (o OrExpr) String() string { return "(" + o.Left.String() + " or " + o.Right.String() + ")" }

// NotExpr negates its operand.
type NotExpr struct {
	X Expr
}

func (n NotExpr) String() string { return "not " + n.X.String() }
