# Leia Evaluate Reference

`leia evaluate` is the P0 entry point for agent evaluation work. It currently
emits a standard JSON report for syntax-level discovery only.

The command parses Leia source, runs existing AI-native source validation, and
discovers `evaluate "case name" { ... }` blocks. It also counts static
declarations such as `agent`, `tool`, `models`, and `budget`, and reports local
`TODO` lines as informational findings. It does not run providers, dispatch
tools, execute agent flows, compare outputs, or score model quality.

```sh
leia evaluate --json path/to/script.leia
leia evaluate --format=text path/to/project
```

The JSON report is versioned with `schema_version: 1` and includes:

| Field | Meaning |
|---|---|
| `phase` | Currently `syntax-static`. |
| `status` | `ok` or `failed`. Syntax and validation errors make the report fail. |
| `summary` | File, parse, AI declaration, and TODO counts. |
| `inputs` | Per-input file status. |
| `cases` | Discovered evaluate blocks with `case_id`, `name`, source path, range, and discovery status. |
| `findings` | TODO, IO, lex, parse, and AI syntax findings. |
| `notes` | Explicit scope notes so callers do not confuse P0 with full eval. |

This first phase is intentionally conservative: it provides a stable report
shape, source-level case discovery, and local TODO discovery without changing
Leia runtime semantics.
