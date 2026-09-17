// Package parse turns log lines into fields.
//
// Each parser reports whether it understood the line rather than returning an
// error, because failing to parse a line is ordinary: a log file is usually a
// structured stream with a stack trace or a startup banner wedged into the
// middle of it. Those lines still have to survive and still have to print.
package parse

import "strings"

// Parser extracts fields from a single line.
type Parser interface {
	// Name identifies the format, for --format and for error messages.
	Name() string
	// Parse returns the fields, and whether the line was understood.
	Parse(line string) (map[string]string, bool)
}

// LogFmt reads key=value pairs, the format emitted by logrus, Go's slog text
// handler, and most of the Heroku-descended ecosystem.
//
//	level=error msg="connection refused" dur=1.2s
type LogFmt struct{}

// Name identifies this parser.
func (LogFmt) Name() string { return "logfmt" }

// Parse splits a line into key=value pairs, honouring quoted values.
//
// A line counts as logfmt only if every non-empty token carries a key. Lines
// of bare words are rejected rather than half-parsed, which is what keeps a
// stack trace from being read as a single mangled field.
func (LogFmt) Parse(line string) (map[string]string, bool) {
	fields := map[string]string{}

	for _, tok := range tokens(line) {
		key, value, found := strings.Cut(tok, "=")
		if !found || key == "" {
			return nil, false
		}
		fields[key] = unquote(value)
	}

	if len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

// tokens splits on whitespace, but keeps quoted runs together so a value like
// msg="connection refused" survives as one token.
func tokens(line string) []string {
	var (
		out     []string
		current strings.Builder
		inQuote bool
		escaped bool
	)

	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && inQuote:
			current.WriteRune(r)
			escaped = true
		case r == '"':
			inQuote = !inQuote
			current.WriteRune(r)
		case (r == ' ' || r == '\t') && !inQuote:
			if current.Len() > 0 {
				out = append(out, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

// unquote strips surrounding quotes and unescapes the usual pairs.
func unquote(v string) string {
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return v
	}
	inner := v[1 : len(v)-1]
	r := strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\t`, "\t")
	return r.Replace(inner)
}
