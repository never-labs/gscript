# Leia Evaluate Reference

`leia evaluate` is the P0 entry point for agent evaluation work. It emits a
standard JSON report and runs each discovered evaluate block body as ordinary
Leia code.

The command parses Leia source, runs existing AI-native source validation, and
discovers `evaluate "case name" { ... }` blocks. For each case it executes the
file's top-level setup code and then the evaluate body, so normal Leia
assertions such as `assert(result == "ok")` determine whether the case passes.
It also counts static declarations such as `agent`, `tool`, `models`, and
`budget`, and reports local `TODO` lines as informational findings. Provider
scoring, golden updates, and workflow orchestration are reserved for later
phases.

```sh
leia evaluate --json path/to/script.leia
leia evaluate --format=text path/to/project
```

The JSON report is versioned with `schema_version: 1` and includes:

| Field | Meaning |
|---|---|
| `phase` | Currently `runtime-minimal`. |
| `status` | `ok` or `failed`. Syntax, validation, and case runtime errors make the report fail. |
| `summary` | File, parse, AI declaration, and TODO counts. |
| `inputs` | Per-input file status. |
| `cases` | Evaluate blocks with `case_id`, `name`, source path, range, and `passed` or `failed` status. |
| `findings` | TODO, IO, lex, parse, AI syntax, and case runtime findings. |
| `notes` | Explicit scope notes so callers do not confuse this with full eval scoring. |

Ordinary script execution still treats evaluate blocks as runtime no-ops. The
minimal runner only changes `leia evaluate`.
