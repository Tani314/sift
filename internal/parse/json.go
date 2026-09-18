package parse

import (
	"encoding/json"
	"strconv"
	"strings"
)

// JSON reads one JSON object per line, the format emitted by slog's JSON
// handler, zap, bunyan and most container runtimes.
//
// Nested objects are flattened with dots, so a query can say
// http.status=500 without needing a path syntax of its own.
type JSON struct{}

// Name identifies this parser.
func (JSON) Name() string { return "json" }

// Parse decodes a JSON object into flat string fields.
func (JSON) Parse(line string) (map[string]string, bool) {
	line = strings.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		// Cheap rejection first. Handing every stack trace line to the JSON
		// decoder is a measurable cost on a large file.
		return nil, false
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, false
	}
	if len(raw) == 0 {
		return nil, false
	}

	fields := map[string]string{}
	flatten("", raw, fields)
	return fields, true
}

// flatten walks nested objects, joining keys with dots.
func flatten(prefix string, in map[string]any, out map[string]string) {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]any); ok {
			flatten(key, nested, out)
			continue
		}
		out[key] = stringify(v)
	}
}

// stringify renders a JSON value as the text a query will compare against.
func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		// encoding/json gives every number as float64. Printing 500 rather
		// than 500.000000 matters: the value is compared as text unless both
		// sides look numeric, and nobody queries for status=500.000000.
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		// Arrays and anything else keep their JSON form, which at least
		// stays searchable by substring.
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}
