# sift

Query structured log files from the command line.

```console
$ sift 'level=error and dur>1s' app.log
```

`grep` does not know that `dur=1500ms` is more than a second, that `status=500`
is a number, or that the stack trace indented under a log line belongs to it.
So the usual answer is a pipeline of `grep`, `awk`, `jq` and `sort | uniq -c`
that nobody remembers the shape of, rebuilt from scratch every time something
breaks. `sift` is that pipeline, as one command.

Standard library only. A log tool that needs a dependency tree is a log tool
you will not install on the box that is actually on fire.

## Install

```console
$ go install github.com/Tani314/sift/cmd/sift@latest
```

## Query syntax

| Form | Meaning |
|---|---|
| `field=value` | equality (`==` also works) |
| `field!=value` | inequality |
| `field<value` `field>=value` | ordering — numeric when both sides are numbers |
| `field=~pattern` | regular expression |
| `field!~pattern` | negated regular expression |
| `word` | substring search over the whole raw line |
| `and` `or` `not` `( )` | combine conditions |

Precedence is the one you already know: `not` binds tightest, then `and`, then
`or`. So `level=error or level=warn and svc=api` reads as
`level=error or (level=warn and svc=api)`.

## What it does that grep does not

**Numbers compare as numbers.** `dur>90` matches `dur=100`. As text, `"100"`
sorts before `"90"` — a wrong answer rather than an error, and nothing in the
output tells you it happened.

**Durations carry their units.** `dur>1s` matches `dur=1500ms`. Units normalise
before comparison, so the log can use whichever one it likes.

**Missing fields are not errors.** Real logs are ragged: half the lines may lack
the field you are asking about. `dur>100` skips lines with no `dur` rather than
failing on the first one.

**Unparseable lines survive.** A Java stack trace wedged between two log lines
still prints, and still matches a substring search. It just has no fields to
query — which is honest, and better than half-parsing it into a field that then
starts matching queries it should not.

**The format is detected, not assumed.** JSON and logfmt are both read, decided
once per file by majority vote over a sample. One JSON-shaped line inside a
logfmt file does not flip the format for everything after it.

**Nested JSON flattens.** `{"http":{"status":500}}` is queryable as
`http.status=500`, with no path syntax to learn.

**`.gz` files open directly.** Rotated logs are almost always compressed, and
remembering which of `app.log` and `app.log.1.gz` can be searched is exactly the
friction that sends people back to `zcat`.

## Examples

Read a field or two instead of the whole line:

```console
$ sift 'level=error' --fields ts,svc,msg app.log
2026-09-30T09:00:04Z  api     connection refused
2026-09-30T09:01:00Z  worker  timeout waiting for lock
```

Line things up when you are reading rather than piping:

```console
$ sift 'status>=500' --table app.log.gz
DUR     LEVEL  MSG                       STATUS  SVC     TS
1500ms  error  connection refused        500     api     2026-09-30T09:00:04Z
3s      error  timeout waiting for lock  504     worker  2026-09-30T09:01:00Z
```

Answer "which ones, and how many":

```console
$ sift 'level=error' --count svc app.log
worker       2   66.7%  #####################
api          1   33.3%  ##########

3 records, 2 distinct svc
```

Hand it to `jq`, whatever the input format was:

```console
$ kubectl logs deploy/api | sift 'status>=500' --format json --json | jq .msg
```

## Flags

| Flag | Purpose |
|---|---|
| `--format` | `auto`, `json` or `logfmt` (default `auto`) |
| `--fields` | comma-separated fields to print instead of the raw line |
| `--table` | align output into columns |
| `--json` | print each match as a JSON object |
| `--meta` | add source file and line number to JSON output |
| `--count FIELD` | summarise matches by a field instead of printing them |
| `--top N` | with `--count`, show only the N most common values |
| `--limit N` | stop after N matches |
| `--with-filename` | prefix each line with `file:line` |

Flags work before or after the query. Files ending in `.gz` are decompressed
transparently, and standard input is read when no file is given.

## Exit codes

Same as `grep`, because that is what scripts already expect:

| Code | Meaning |
|---|---|
| `0` | at least one line matched |
| `1` | nothing matched |
| `2` | the query or the input could not be read |

## Design notes

**Everything streams.** Log files are routinely larger than memory, and the one
time anybody reaches for a log tool is the one time they cannot afford it to
allocate a gigabyte. `--limit 10` against a 40GB file stops after ten matches
rather than filtering afterwards.

**Raw output is the default.** `sift` is usually one stage in a pipeline, and a
tool that reformats what it forwards breaks everything downstream of it. The
original line is kept verbatim rather than re-serialised from the parsed
fields, so a parser that guessed wrong cannot silently rewrite your logs.

**Patterns compile before any input is read.** A typo in a regex is reported
immediately, not after streaming half a gigabyte and matching nothing.

**`--table` is the one renderer that buffers**, because column widths are not
known until the widest value has been seen. That is exactly why it is not the
default.

## Limits worth knowing

- **No follow mode.** There is no `-f`. Use `tail -f app.log | sift '...'`,
  which works today, with `--table` left off since it buffers.
- **Two formats.** JSON and logfmt. Not syslog, not the Apache combined log
  format, not Windows event logs.
- **Timestamps are text.** `ts>2026-09-30T09:00:00Z` works because RFC 3339
  sorts correctly as a string. Any other timestamp layout will not.
- **One field per `--count`.** No grouping by two fields, no sums or averages —
  for anything beyond a tally, pipe `--json` into `jq`.

## Development

```console
$ go test ./...
$ go build ./cmd/sift
```

Tests run against temp files and the real binary, with no fixtures to keep in
sync. The end-to-end tests in `cmd/sift` drive the compiled command, which is
the only place argument handling and exit codes can actually be checked.
