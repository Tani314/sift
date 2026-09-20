// Package query implements sift's filter expressions.
//
//	level=error and dur>100
//	msg=~"timeout|refused" and not status=200
//
// The language is deliberately small. It has to be typed correctly at a
// terminal, under pressure, by someone reading it for the first time out of
// a runbook -- so it has comparisons, three boolean operators, and nothing
// else. No arithmetic, no functions, no precedence rules to memorise beyond
// the ones everyone already knows.
package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Kind classifies a token.
type Kind int

// The token kinds.
const (
	EOF Kind = iota
	Ident
	String
	Op
	And
	Or
	Not
	LParen
	RParen
)

func (k Kind) String() string {
	switch k {
	case EOF:
		return "end of input"
	case Ident:
		return "word"
	case String:
		return "string"
	case Op:
		return "operator"
	case And:
		return "and"
	case Or:
		return "or"
	case Not:
		return "not"
	case LParen:
		return "("
	case RParen:
		return ")"
	}
	return "unknown"
}

// Token is one lexical unit, carrying the position it started at so a parse
// error can point at the offending character.
type Token struct {
	Kind Kind
	Text string
	Pos  int
}

// operators are matched longest-first, so <= never lexes as < followed by =.
var operators = []string{"=~", "!~", "==", "!=", "<=", ">=", "=", "<", ">"}

// Lex splits a filter expression into tokens.
func Lex(input string) ([]Token, error) {
	var tokens []Token
	runes := []rune(input)

	for i := 0; i < len(runes); {
		r := runes[i]

		switch {
		case unicode.IsSpace(r):
			i++

		case r == '(':
			tokens = append(tokens, Token{Kind: LParen, Text: "(", Pos: i})
			i++

		case r == ')':
			tokens = append(tokens, Token{Kind: RParen, Text: ")", Pos: i})
			i++

		case r == '"' || r == '\'':
			text, next, err := lexString(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, Token{Kind: String, Text: text, Pos: i})
			i = next

		default:
			if op, ok := matchOperator(runes, i); ok {
				tokens = append(tokens, Token{Kind: Op, Text: op, Pos: i})
				i += len([]rune(op))
				continue
			}
			text, next := lexWord(runes, i)
			if text == "" {
				return nil, fmt.Errorf("unexpected character %q at position %d", string(r), i)
			}
			tokens = append(tokens, Token{Kind: keyword(text), Text: text, Pos: i})
			i = next
		}
	}

	return append(tokens, Token{Kind: EOF, Pos: len(runes)}), nil
}

// keyword classifies a bare word, case-insensitively so AND and and both work.
func keyword(word string) Kind {
	switch strings.ToLower(word) {
	case "and":
		return And
	case "or":
		return Or
	case "not":
		return Not
	}
	return Ident
}

func matchOperator(runes []rune, i int) (string, bool) {
	rest := string(runes[i:])
	for _, op := range operators {
		if strings.HasPrefix(rest, op) {
			return op, true
		}
	}
	return "", false
}

// lexWord reads until whitespace, a bracket, or an operator. Field names and
// bare values are both words, which is what lets level=error be written
// without quoting anything.
func lexWord(runes []rune, i int) (string, int) {
	start := i
	for i < len(runes) {
		r := runes[i]
		if unicode.IsSpace(r) || r == '(' || r == ')' {
			break
		}
		if _, isOp := matchOperator(runes, i); isOp {
			break
		}
		i++
	}
	return string(runes[start:i]), i
}

// lexString reads a quoted run, allowing the other quote character inside
// without escaping.
func lexString(runes []rune, i int) (string, int, error) {
	quote := runes[i]
	var b strings.Builder

	for j := i + 1; j < len(runes); j++ {
		switch runes[j] {
		case '\\':
			if j+1 < len(runes) {
				j++
				b.WriteRune(runes[j])
				continue
			}
			b.WriteRune('\\')
		case quote:
			return b.String(), j + 1, nil
		default:
			b.WriteRune(runes[j])
		}
	}
	return "", 0, fmt.Errorf("unterminated string starting at position %d", i)
}
