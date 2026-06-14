# Leia Tagged Dialects

The registry table is generated from the current `leia` binary dialect registry; explanatory sections below are maintained with the generated reference.

Leia supports DSL-native tagged dialects for compact host automation, data format handling, web routing, q-style analytics, spreadsheets, and AI workflows. A dialect is an explicit tagged expression that returns an ordinary Leia value.

## Forms

```leia
status := sh`git status --short`
checked := sh!`printf checked`
argv_checked := cmd!`printf checked`
out := $`printf hello`
files := glob`examples/**/*.leia`
data := json`{"name": ${name}}`
reviewer := agent {
    name: "release_reviewer"
    config: func(summary) {
        return {model: "mock-fast", user: summary}, nil
    }
    params: {"summary"}
}
```

`${expr}` is the interpolation form inside tagged strings. Each dialect decides how interpolated values are encoded. `tag!` is the fail-fast form for dialects that support recoverable errors; `sh!` and `cmd!` raise on non-zero command exits while preserving the same result shape for successful commands.

## Built-In Dialects

| Tag | Category | Eval | Block | Capabilities | Aliases |
|---|---|---|---|---|---|
| `agent` | `llm` | false | true | llm.turn | none |
| `base32` | `data` | true | false | none | none |
| `base64` | `data` | true | false | none | none |
| `binary` | `data` | true | false | none | none |
| `cidr` | `protocol` | true | false | none | none |
| `cmd` | `host` | true | false | process.exec, env.write | none |
| `cookie` | `protocol` | true | true | none | cookies |
| `cookies` | `protocol` | true | true | none | cookie |
| `csv` | `text` | true | true | none | none |
| `deflate` | `data` | true | false | none | none |
| `duration` | `text` | true | true | none | none |
| `emailaddr` | `protocol` | true | true | none | mailaddr |
| `env` | `host` | true | true | env.read | none |
| `excel` | `data` | true | false | none | xlsx |
| `form` | `protocol` | true | true | none | urlform |
| `glob` | `host` | true | false | fs.read | none |
| `gzip` | `data` | true | false | none | none |
| `hash` | `data` | true | false | none | none |
| `headers` | `protocol` | true | true | none | http_headers |
| `hex` | `data` | true | false | none | none |
| `hostport` | `protocol` | true | false | none | none |
| `html` | `protocol` | true | true | none | none |
| `html_escape` | `protocol` | true | false | none | none |
| `http_headers` | `protocol` | true | true | none | headers |
| `httpmsg` | `protocol` | true | true | none | none |
| `ini` | `text` | true | true | none | none |
| `ipaddr` | `protocol` | true | false | none | none |
| `json` | `text` | true | true | none | none |
| `jsonl` | `text` | true | true | none | none |
| `jsonptr` | `text` | true | true | none | none |
| `junit` | `compat` | true | false | none | none |
| `jwt` | `protocol` | true | true | none | none |
| `kv` | `text` | true | true | none | none |
| `lines` | `text` | true | false | none | split |
| `logfmt` | `text` | true | true | none | none |
| `mailaddr` | `protocol` | true | true | none | emailaddr |
| `markdown` | `text` | true | true | none | md |
| `md` | `text` | true | true | none | markdown |
| `mdtable` | `text` | true | true | none | none |
| `mime` | `protocol` | true | true | none | none |
| `model` | `llm` | false | true | llm.turn | none |
| `multipart` | `protocol` | true | true | none | none |
| `numbers` | `text` | true | false | none | nums |
| `nums` | `text` | true | false | none | numbers |
| `path` | `text` | true | false | none | none |
| `pem` | `data` | true | false | none | none |
| `prompt` | `llm` | true | true | none | none |
| `q` | `data` | true | false | none | none |
| `quote` | `llm` | true | true | none | none |
| `re` | `text` | true | false | none | regexp |
| `regexp` | `text` | true | false | none | re |
| `rfc3339` | `text` | true | true | none | timestamp |
| `semver` | `text` | true | true | none | none |
| `serve` | `web` | true | true | net.listen | none |
| `sh` | `host` | true | false | process.shell | none |
| `shellwords` | `text` | true | true | none | none |
| `split` | `text` | true | false | none | lines |
| `sql` | `database` | true | true | none | none |
| `sse` | `protocol` | true | true | none | none |
| `tap` | `text` | true | true | none | none |
| `template` | `text` | true | true | none | none |
| `timestamp` | `text` | true | true | none | rfc3339 |
| `tool` | `llm` | false | true | llm.turn | none |
| `tsv` | `text` | true | true | none | none |
| `turn` | `llm` | false | true | llm.turn | none |
| `url` | `protocol` | true | false | none | none |
| `urlform` | `protocol` | true | true | none | form |
| `urlpath` | `protocol` | true | true | none | none |
| `urlquery` | `protocol` | true | true | none | none |
| `uuid` | `data` | true | false | none | none |
| `words` | `text` | true | false | none | none |
| `xlsx` | `data` | true | false | none | excel |
| `xml` | `text` | true | true | none | none |
| `yaml` | `text` | true | true | none | yml |
| `yml` | `text` | true | true | none | yaml |
| `zlib` | `data` | true | false | none | none |

## Capability Categories

- Host automation tags such as `sh`, `cmd`, `glob`, and `env` use host filesystem, process, or environment capabilities.
- Web and network-facing tags such as `serve` must be denied when the embedding host has not granted the relevant network capability.
- AI tags such as `model`, `turn`, `tool`, and `agent` use the same `llm.turn` policy as the `llm` standard library.
- Pure text, protocol, and data tags return ordinary values and should be covered by examples before being promoted in README-facing claims.

## Important Result Shapes

| Dialect | Result shape |
|---|---|
| `sh`, `$` | Command result table with `ok`, `code`, `stdout`, `stderr`, and `text`. |
| `cmd` | Argv-safe command result table with the same command result shape as `sh`. |
| `glob` | Sorted path array. |
| `sql` | `{query, args, names}` with named parameters lowered to positional placeholders. |
| `q` | Symbolic q-style vectors, dictionaries, and tables. SoA rollups use `q.query(soa, plan_table)`. |
| `xlsx` encode | Workbook byte string suitable for writing or decoding with `excel`. |
| `excel` decode | Row array; with `{headers: true}`, rows are tables keyed by the first worksheet row. |
| `serve` | Route server descriptor/loopback result as documented by runnable web examples. |
| `turn` | `(result, err)` for a single LLM provider request. |
| `agent` | Callable agent value. |

## Examples And Gates

```bash
go run ./cmd/leia examples check examples/hello/dialects.leia examples/dialects/text_parsing.leia
go run ./cmd/leia examples run repo-tooling-release_gate_project-main
```

The release gate project combines fixture discovery, shell/process dialects, SQLite frames, q-style aggregation, spreadsheet round-tripping, a mocked AI agent, and a loopback web route. It is tracked by the feature matrix so published dialect coverage stays tied to executable evidence.

Additional focused evidence lives in:

- `examples/dialects/ai_prompt_quote.leia` for prompt and quote tags.
- `examples/dialects/data_aggregation_report.leia` for mixed data-format tags.
- `examples/dialects/http_protocol_trace.leia`, `examples/dialects/sse_stream_fixture.leia`, and `examples/dialects/multipart_form_fixture.leia` for web/protocol tags.
- `examples/data/q_trade_analytics_project/main.leia` for q-style vectors, dictionaries, scans, filters, and table rollups.
- `examples/web/serve_dialect_app.leia` and `examples/web/route_workbench.leia` for route/server evidence.
