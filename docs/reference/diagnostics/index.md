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
| `leia capabilities --json` | Binary feature, command, stdlib, default-import, builtin dialect, LLM, and tooling capabilities, including `tooling.report_count` and the `tooling.reports` JSON report registry with `status_field`, `count_fields`, and `collection_fields`. |

Release scripts that emit JSON follow the same pattern: a `schema_version`,
status field, top-level count fields, and collection fields. The advertised
field names are listed in `leia capabilities --json` under `tooling.reports`.
In particular,
`scripts/public_release_blockers_check.sh --json` exposes `blocker_count` and
kind counts for missing files, open release decisions, stale text, unconfirmed
policies, missing guidance, and missing documentation snippets.
`leia diag bundle --json` exposes `file_count` for the generated bundle files
listed in `files`.
`scripts/performance_gate.sh --validate-only FILE --json` exposes
`output_line_count` for captured validation output.

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
```

Bundles are intended for compiler/runtime investigations. User-facing tooling
should normally start with `leia check --json`, `leia lint --format=sarif`, and
`leia inspect`.

## Stability Notes

Human-readable messages may change to improve clarity. Field names in documented
JSON outputs and public Go error types should remain compatible across patch
releases.
