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
scoring and workflow orchestration are reserved for later phases.

```sh
leia evaluate --json path/to/script.leia
leia evaluate --format=text path/to/project
leia evaluate --list --filter "refund flow" tests/agents
leia evaluate --filter "refund flow" tests/agents
leia evaluate --llm-replay tests/agent.records.json tests/agent.leia
leia evaluate --update-golden tests/agent.records.json tests/agent.leia
```

`--list` discovers evaluate cases without executing their bodies. Listed cases
use status `listed`, keep source metadata and assertion metadata, and do not
consume LLM replay fixtures.

`--filter TEXT` runs only cases whose name, source path, or case id contains
`TEXT`. Discovery counts still include all evaluate blocks so dashboards can
distinguish "not discovered" from "discovered but filtered out".

LLM record/replay uses the same JSON fixture format as the public `llm`
package:

| Flag | Meaning |
|---|---|
| `--llm-replay FILE` | Use `FILE` as a deterministic provider transcript. Request mismatches, exhausted replay, or unconsumed turns fail the report. |
| `--llm-record FILE` | Run against the configured provider and write observed turns to `FILE`. |
| `--update-golden FILE` | Run against the configured provider and rewrite `FILE` as the new golden transcript. This is intentionally explicit so CI can forbid it. |

The three LLM fixture modes are mutually exclusive.

The JSON report is versioned with `schema_version: 1` and includes:

| Field | Meaning |
|---|---|
| `phase` | Currently `runtime-minimal`. |
| `status` | `ok` or `failed`. Syntax, validation, and case runtime errors make the report fail. |
| `started_at` | UTC RFC3339 timestamp for the evaluation run. |
| `runtime` | Leia and Go runtime metadata: version, OS/arch, and build VCS fields when available. |
| `summary` | File, parse, AI declaration, TODO, selected/skipped case, pass/fail/list, assertion, duration, and pass-rate counts. |
| `llm` | Optional LLM fixture metadata: mode, paths, loaded turns, replayed turns, remaining turns, and recorded turns. |
| `inputs` | Per-input file status. |
| `cases` | Evaluate blocks with `case_id`, `name`, source path, range, status, per-case `started_at`, duration, assertions, and diagnostics. |
| `findings` | TODO, IO, lex, parse, AI syntax, case runtime, and replay-drift findings. Replay mismatch findings include stable JSON `details.expected` and `details.actual` request summaries; exhausted and unconsumed replay findings include turn/count details. |
| `notes` | Explicit scope notes so callers do not confuse this with full eval scoring. |

Ordinary script execution still treats evaluate blocks as runtime no-ops. The
minimal runner only changes `leia evaluate`.

Summary fields are intentionally dashboard-friendly. `evaluate_blocks` is the
number of discovered blocks before filtering. `cases_selected` is the number
that matched `--filter` and were listed or executed. `cases_skipped` is the
number filtered out. `cases_passed` and `cases_failed` count executed cases;
`cases_listed` counts `--list` results. `pass_rate` is
`cases_passed / (cases_passed + cases_failed)` and is `0` when no case ran.
