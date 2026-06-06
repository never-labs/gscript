# Tagged Dialects

Leia supports DSL-native tagged dialects for compact host automation, data
format handling, web routing, q-style analytics, spreadsheets, and AI workflows.
A dialect is not a separate language mode for the whole file. It is an explicit
tagged expression that returns an ordinary Leia value.

## Forms

Tagged string literals pass raw text plus interpolation metadata to the dialect:

```leia
status := sh`git status --short`
files := glob`examples/**/*.leia`
pattern := re`\bagent\s+\w+`
```

`tag!` is the fail-fast form. The dialect receives the same payload, but errors
raise instead of returning a recoverable result when the dialect supports that
mode:

```leia
sh!`printf checked`
```

The `$` tag is the shell shortcut and follows the same result shape as `sh`:

```leia
out := $`printf hello`
assert(out.ok && out.text == "hello")
```

Tagged blocks are configuration-style dialect forms. The block is evaluated as
dialect input, and the dialect validates fields, defaults, capabilities, and
result shape:

```leia
reviewer := agent {
    name: "release_reviewer"
    config: func(summary) {
        return {
            model: "mock-fast",
            system: "Review the release gate summary.",
            user: summary,
        }, nil
    }
    params: {"summary"}
}
```

## Interpolation

`${expr}` is the interpolation form inside tagged strings. Dialects decide how
the expression is encoded. Shell-oriented dialects preserve structured command
results and must avoid treating ordinary interpolation as a license to bypass
the host capability policy. Data dialects encode values according to their
format rules.

```leia
name := "Leia"
encoded := json`{"name": ${name}}`
assert(encoded.name == "Leia")
```

## Core Built-In Categories

The built-in registry is intentionally broad but still explicit. Use
`leia capabilities --json` to inspect the exact dialect list for a binary.

| Category | Representative tags | Purpose |
|---|---|---|
| Host automation | `sh`, `cmd`, `$`, `glob`, `path`, `env` | Shell-compatible commands, argv-safe commands, file discovery, paths, and environment lookup. |
| Text and formats | `re`, `json`, `jsonl`, `csv`, `tsv`, `yaml`, `xml`, `template` | Parsing, encoding, validation, and compact data literals. |
| Protocols | `url`, `httpmsg`, `headers`, `cookie`, `sse`, `multipart`, `jwt`, `pem` | Structured protocol fixtures and validation. |
| Web | `html`, `urlpath`, `serve` | HTML/text helpers and high-level route declarations. |
| Data | `sql`, `q`, `xlsx`, `excel`, `binary` | SQL-shaped input, q-style analytics, spreadsheet round-tripping, and binary fixtures. |
| AI | `model`, `turn`, `tool`, `agent`, `prompt`, `quote` | AI provider configuration, turns, tools, agents, and prompt/quote values. |

## Capabilities

Dialects participate in the same host capability model as ordinary standard
library calls. Examples:

| Dialect | Typical capability |
|---|---|
| `sh` | `process.shell` |
| `cmd` | `process.exec` |
| `glob` | `fs.read` |
| `serve` | `network.listen` |
| `turn` / `agent` | `llm.turn` |

Embedding hosts can disable process execution, shell execution, network access,
filesystem access, or LLM providers. A disabled capability must fail the dialect
instead of silently falling back to a less restricted path.

## Examples And Gates

The example tree contains both small dialect examples and cross-domain projects:

```bash
go run ./cmd/leia examples check examples/hello/dialects.leia examples/dialects/text_parsing.leia
go run ./cmd/leia examples run repo-tooling-release_gate_project-main
```

The release gate project combines fixture discovery, shell/process dialects,
SQLite frames, q-style aggregation, spreadsheet round-tripping, a mocked AI
agent, and a loopback web route. It is tracked by the feature matrix so the
README dialect promise stays tied to executable evidence.
