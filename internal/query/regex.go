package query

import (
	"fmt"
	"regexp"
	"sync"
)

// Patterns are compiled once and reused. A filter is evaluated against every
// line in the file, so recompiling per line would dominate the runtime on
// anything larger than a toy log.
var (
	patternsMu sync.RWMutex
	patterns   = map[string]*regexp.Regexp{}
)

// pattern compiles a regex, caching the result.
func pattern(expr string) (*regexp.Regexp, error) {
	patternsMu.RLock()
	re, ok := patterns[expr]
	patternsMu.RUnlock()
	if ok {
		return re, nil
	}

	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", expr, err)
	}

	patternsMu.Lock()
	patterns[expr] = re
	patternsMu.Unlock()
	return re, nil
}

// Validate walks an expression and compiles every pattern in it.
//
// The CLI calls this before reading any input, so a typo in a regex is
// reported immediately rather than after streaming half a gigabyte of logs
// and matching nothing.
func Validate(e Expr) error {
	switch n := e.(type) {
	case Compare:
		if n.Op == "=~" || n.Op == "!~" {
			_, err := pattern(n.Value)
			return err
		}
	case AndExpr:
		if err := Validate(n.Left); err != nil {
			return err
		}
		return Validate(n.Right)
	case OrExpr:
		if err := Validate(n.Left); err != nil {
			return err
		}
		return Validate(n.Right)
	case NotExpr:
		return Validate(n.X)
	}
	return nil
}
