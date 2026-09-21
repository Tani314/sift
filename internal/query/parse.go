package query

import "fmt"

// Parse turns a filter expression into an Expr tree.
//
// Precedence follows what people already expect from every other language
// that has these operators: not binds tightest, then and, then or. So
//
//	level=error or level=warn and svc=api
//
// reads as level=error or (level=warn and svc=api). Parentheses are there for
// anyone who would rather not rely on remembering that.
func Parse(input string) (Expr, error) {
	tokens, err := Lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens}

	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != EOF {
		return nil, fmt.Errorf("unexpected %s %q at position %d", p.peek().Kind, p.peek().Text, p.peek().Pos)
	}
	return expr, nil
}

type parser struct {
	tokens []Token
	pos    int
}

func (p *parser) peek() Token { return p.tokens[p.pos] }

func (p *parser) next() Token {
	t := p.tokens[p.pos]
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}

func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == Or {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = OrExpr{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == And {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = AndExpr{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (Expr, error) {
	if p.peek().Kind == Not {
		p.next()
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return NotExpr{X: inner}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	tok := p.next()

	switch tok.Kind {
	case LParen:
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().Kind != RParen {
			return nil, fmt.Errorf("missing closing parenthesis for the one at position %d", tok.Pos)
		}
		p.next()
		return inner, nil

	case Ident, String:
		// A word followed by an operator is a comparison; a word on its own
		// is a substring search.
		if p.peek().Kind == Op {
			op := p.next()
			value := p.next()
			if value.Kind != Ident && value.Kind != String {
				return nil, fmt.Errorf("expected a value after %q at position %d, got %s",
					op.Text, op.Pos, value.Kind)
			}
			return Compare{Field: tok.Text, Op: normalizeOp(op.Text), Value: value.Text}, nil
		}
		return Match{Text: tok.Text}, nil

	case EOF:
		return nil, fmt.Errorf("unexpected end of query -- expected a condition")
	}

	return nil, fmt.Errorf("unexpected %s %q at position %d", tok.Kind, tok.Text, tok.Pos)
}

// normalizeOp folds = into == so the evaluator has one spelling to handle.
func normalizeOp(op string) string {
	if op == "=" {
		return "=="
	}
	return op
}
