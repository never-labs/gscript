# Leia Errors And Diagnostics

Leia exposes structured errors through the Go embedding API and structured
diagnostics through CLI commands. Hosts and editor integrations should prefer
typed errors and JSON/SARIF output over parsing human text.

## Go Errors

Execution APIs return ordinary Go `error` values. Use `errors.As` to inspect
typed failures.

| Type | Meaning |
|---|---|
| `*leia.Error` | Lexer, parser, runtime, or script error. |
| `*leia.HostCallbackError` | Registered Go callback returned a non-nil error. |
| `*leia.HostCallbackPanicError` | Registered Go callback panicked and was recovered. |
| `*leia.ExitError` | Script requested process exit. |
| `*leia.BudgetError` | VM resource budget was exceeded. |

`leia.Error.Kind` is one of:

| Kind | Meaning |
|---|---|
| `lex` | Lexer/tokenization failure. |
| `parse` | Parser failure. |
| `runtime` | Runtime or host integration failure. |
| `script` | Script called `error()` or raised a script value. |

Example:

```go
err := vm.Exec(source)
var scriptErr *leia.Error
if errors.As(err, &scriptErr) {
    log.Printf("%s:%d:%d %s", scriptErr.File, scriptErr.Line, scriptErr.Col, scriptErr.Message)
}
var budgetErr *leia.BudgetError
if errors.As(err, &budgetErr) {
    log.Printf("budget %s exceeded at %d", budgetErr.Resource, budgetErr.Limit)
}
```

## CLI Diagnostics

| Command | Structured output |
|---|---|
| `leia lint --json` | Versioned report with `schema_version`, `status`, diagnostic counts, and diagnostics. |
| `leia lint --format=json` | Legacy array of diagnostics with `file`, `code`, `severity`, `message`, `line`, and `column`. |
| `leia lint --format=sarif` | SARIF 2.1.0 for code scanning integrations. |
| `leia fmt --check --json` | Formatter check report with changed files and per-file errors. |
| `leia check --json` | Aggregate check report with step status and exit codes. |
| `leia config --json` | Resolved project configuration with `ok`, discovery status, and diagnostics. |
| `leia test --json` | Test run report with schema version, pass/fail counts, seed, golden mode, and per-file results. |
| `leia test --list --json` | Test discovery report with schema version, list mode, file count, and selected files. |
| `leia inspect bytecode --json` | Compiled bytecode metadata, disassembly text, nested proto summary, and JIT callable decisions. |
| `leia inspect directives --json` | Versioned file-directive report with `schema_version`, `status`, `directive_count`, and parsed `//leia:` directives. |
| `leia mod ... --json` | Module graph, list, verify, capability, and vendoring reports. |
| `leia capabilities --json` | Binary feature, command, stdlib, default-import, builtin dialect, LLM, and tooling capabilities, including `tooling.report_count` and the `tooling.reports` JSON report registry with `status_field`, `scalar_fields`, `count_fields`, `collection_fields`, and `collection_item_fields`. |

Release scripts that emit JSON follow the same pattern: a `schema_version`,
status field, stable scalar fields, top-level count fields, collection fields,
and object-array item fields. The advertised field names are listed in `leia capabilities --json` under `tooling.reports`. Nested report fields use dotted
JSON paths, and `[]` marks per-item array paths.
In particular,
`scripts/run.sh public-blockers --json` exposes `blocker_count` and
kind counts for missing files, open release decisions, stale text, unconfirmed
policies, missing guidance, missing documentation snippets, plus
`open_blocker_count`, `blocker_status_count`, `blocker_statuses`,
`decision_area_count`, and `decision_areas` for the required maintainer
decision domains.
`leia diag bundle --json` and
`scripts/run.sh diagnostics --json` expose `file_count` for the generated
bundle files listed in `files`, and `failure_details` for failed collected
checks.
`leia doc check --json` and `scripts/run.sh docs --json` expose
`failure_kind_count`, `failure_kinds`, and `failure_details` for documentation
gate failures.
`scripts/run.sh editor --json` uses the same failure fields for editor asset
and optional tool checks.
`scripts/run.sh perf --validate-only FILE --json` exposes
`failure_kind_count`, `failure_kinds`, `failure_details`, and
`output_line_count` for captured validation output.
`scripts/run.sh release-notes --json` exposes `failure_kind_count`,
`failure_kinds`, and `failure_details` so release note issues can be grouped
without parsing human-readable failure strings.

Current lint codes:

| Code | Meaning |
|---|---|
| `LEIA0001` | File discovery failed. |
| `LEIA1001` | Lexer or parser error. |
| `LEIA2001` | Positional `{...}` table literal. Use `[...]` for list literals and reserve `{...}` for keyed records. |

## Diagnostic Bundles

`leia diag` delegates to repository scripts:

```bash
leia diag dump
leia diag bundle --output /tmp/leia-diag --skip-benchmarks
leia diag bundle --output /tmp/leia-diag --skip-go-tests --skip-benchmarks --json
scripts/run.sh diagnostics --output /tmp/leia-diag --skip-go-tests --skip-benchmarks --json
```

Bundles are intended for compiler/runtime investigations. User-facing tooling
should normally start with `leia check --json`, `leia lint --format=sarif`, and
`leia inspect`.

## Stability Notes

Human-readable messages may change to improve clarity. Field names in documented
JSON outputs and public Go error types should remain compatible across patch
releases.
