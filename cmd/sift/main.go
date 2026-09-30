// Command sift queries structured log files.
//
//	sift 'level=error and dur>1s' app.log
//	sift 'msg=~timeout' --fields ts,svc,msg app.log.gz
//	sift 'level=error' --count svc app.log
//	kubectl logs deploy/api | sift 'status>=500' --format json
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Tani314/sift/internal/parse"
	"github.com/Tani314/sift/internal/query"
	"github.com/Tani314/sift/internal/record"
	"github.com/Tani314/sift/internal/render"
	"github.com/Tani314/sift/internal/source"
	"github.com/Tani314/sift/internal/stats"
)

// Exit codes follow grep's, because that is what scripts already expect:
// 0 found something, 1 found nothing, 2 something went wrong.
const (
	exitMatched = 0
	exitNoMatch = 1
	exitError   = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		format    = flag.String("format", "auto", "input format: auto, json or logfmt")
		fields    = flag.String("fields", "", "comma-separated fields to print instead of the raw line")
		table     = flag.Bool("table", false, "align output into columns (buffers all matches)")
		asJSON    = flag.Bool("json", false, "print each match as a JSON object")
		meta      = flag.Bool("meta", false, "include source file and line number in JSON output")
		countBy   = flag.String("count", "", "summarise matches by this field instead of printing them")
		top       = flag.Int("top", 0, "with --count, show only the N most common values")
		limit     = flag.Int("limit", 0, "stop after N matches")
		showNames = flag.Bool("with-filename", false, "prefix each line with file:line")
	)

	flag.Usage = usage
	args := parseInterspersed(os.Args[1:])
	if len(args) == 0 {
		usage()
		return exitError
	}

	expr, err := query.Parse(args[0])
	if err != nil {
		return fail("%v", err)
	}
	// Compile patterns before reading a byte, so a typo in a regex is
	// reported now rather than after streaming the whole file.
	if err := query.Validate(expr); err != nil {
		return fail("%v", err)
	}

	paths := args[1:]
	parser, err := chooseParser(*format, paths)
	if err != nil {
		return fail("%v", err)
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	renderer := chooseRenderer(*fields, *table, *asJSON, *meta, *showNames || len(paths) > 1)
	counter := (*stats.Counter)(nil)
	if *countBy != "" {
		counter = stats.New(*countBy)
	}

	matched := 0
	err = source.ScanAll(paths, func(l source.Line) error {
		r := record.Record{
			Line:   l.Text,
			Fields: parse.Apply(parser, l.Text),
			Source: l.Source,
			Num:    l.Num,
		}

		ok, err := query.Eval(expr, r)
		if err != nil || !ok {
			return err
		}

		matched++
		if counter != nil {
			counter.Add(r)
		} else if err := renderer.Write(out, r); err != nil {
			return err
		}

		if *limit > 0 && matched >= *limit {
			return errLimitReached
		}
		return nil
	})
	if err != nil && err != errLimitReached {
		return fail("%v", err)
	}

	if counter != nil {
		if err := counter.Render(out, *top); err != nil {
			return fail("%v", err)
		}
	} else if err := renderer.Flush(out); err != nil {
		return fail("%v", err)
	}

	if matched == 0 {
		return exitNoMatch
	}
	return exitMatched
}

// parseInterspersed parses flags that appear after positional arguments.
//
// Go's flag package stops at the first non-flag argument, which would make
// `sift 'level=error' --json app.log` treat --json as a filename. Since the
// query has to come first to read naturally, flags have to be allowed after
// it -- so parsing alternates between consuming flags and collecting one
// positional argument until the arguments run out.
func parseInterspersed(args []string) []string {
	var positional []string

	for {
		if err := flag.CommandLine.Parse(args); err != nil {
			// flag has already reported the problem and, with the default
			// ExitOnError, will not reach here.
			return nil
		}
		if flag.NArg() == 0 {
			return positional
		}
		positional = append(positional, flag.Arg(0))
		args = flag.Args()[1:]
	}
}

// errLimitReached stops the scan once --limit matches are in hand. Stopping
// early is the point of the flag: `--limit 10` on a 40GB file should not read
// 40GB.
var errLimitReached = fmt.Errorf("limit reached")

// chooseParser resolves --format, sniffing the input when set to auto.
func chooseParser(format string, paths []string) (parse.Parser, error) {
	if format != "auto" {
		return parse.ByName(format)
	}
	if len(paths) == 0 {
		// Standard input cannot be sampled without consuming it, so auto
		// detection is not possible there. Rather than eat the first lines
		// of a pipe, fall back to logfmt and let --format override.
		return parse.LogFmt{}, nil
	}

	sample, err := source.Sample(paths[0], 20)
	if err != nil {
		return nil, err
	}
	return parse.Detect(sample), nil
}

// chooseRenderer picks the output shape from the flags.
func chooseRenderer(fields string, table, asJSON, meta, withNames bool) render.Renderer {
	names := splitFields(fields)

	switch {
	case asJSON:
		return render.JSON{Meta: meta}
	case table:
		return &render.Table{Names: names}
	case len(names) > 0:
		return render.Fields{Names: names}
	default:
		return render.Raw{ShowSource: withNames}
	}
}

func splitFields(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fail(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "sift: "+format+"\n", args...)
	return exitError
}

func usage() {
	fmt.Fprint(os.Stderr, `sift -- query structured log files

usage: sift <query> [file...]

Reads standard input when no file is given. Files ending in .gz are
decompressed transparently.

query syntax:
  field=value        equality (== also works)
  field!=value       inequality
  field<value        ordering; numeric when both sides are numbers,
  field>=value       and durations like 1.5s compare against 250ms
  field=~pattern     regular expression
  field!~pattern     negated regular expression
  word               substring search over the whole line
  and, or, not, ()   combine conditions; not binds tightest, then and, then or

examples:
  sift 'level=error' app.log
  sift 'level=error and dur>1s' --fields ts,svc,msg app.log
  sift 'status>=500 or msg=~timeout' --table app.log.gz
  sift 'level=error' --count svc --top 10 app.log
  kubectl logs deploy/api | sift 'status>=500' --format json

exit codes:
  0  at least one line matched
  1  nothing matched
  2  the query or the input could not be read

flags:
`)
	flag.PrintDefaults()
}
