# Leia Evaluate Reference

`leia evaluate` is the entry point for agent evaluation work. It emits a
versioned report and runs each discovered evaluate block body as ordinary Leia
code under the evaluation harness.

The command parses Leia source, runs existing AI-native source validation, and
discovers `evaluate "case name" { ... }` blocks. For each case it executes the
file's top-level setup code and then the evaluate body, so normal Leia
assertions such as `assert(result == "ok")` determine whether the case passes.
It also counts static declarations such as `agent`, `tool`, `models`, and
`budget`, and reports local `TODO` lines as informational findings. During the
run, the harness-only `eval` module can load JSONL corpora, run named subcases,
record metrics, skip fixtures, and fail gates without adding more syntax.

```sh
leia evaluate --json path/to/script.leia
leia evaluate --json --report eval-report.json path/to/project
leia evaluate --format=text path/to/project
leia evaluate --format=html --report eval-report.html path/to/project
leia evaluate --gate tests/agents
leia evaluate --baseline baseline.json --regression-threshold 0.05 tests/agents
leia evaluate --compare baseline.json current.json --format=text
leia evaluate --list --filter "refund flow" tests/agents
leia evaluate --filter "refund flow" tests/agents
leia evaluate --replay tests/agent.records.json tests/agent.leia
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
| `--format json\|text\|html` | Select the report renderer. |
| `--report FILE` | Write the rendered report to `FILE` instead of stdout. `--output FILE` is accepted as an alias. The command still exits non-zero when the report status is `failed`. |
| `--gate` | Explicit CI gate mode. The current command already exits non-zero for failed reports; the flag documents that intent in scripts. |
| `--baseline FILE` | Load a previous JSON evaluate report and attach a comparison section to the current report. |
| `--compare OLD NEW` | Compare two existing JSON evaluate reports without executing source. The comparison is attached to `NEW` and rendered with the selected `--format`. |
| `--regression-threshold N` | Allow bool pass-rate regressions up to `N` when `--baseline` is used. Summary pass-rate and bool metric pass-rate regressions beyond the threshold fail the report. Number and string metrics are compared but not treated as regressions because directionality is metric-specific. |
| `--replay FILE` | Use `FILE` as a deterministic provider transcript. Request mismatches, exhausted replay, or unconsumed turns fail the report. |
| `--record FILE` | Run against the configured provider and write observed turns to `FILE`. |
| `--update-golden FILE` | Run against the configured provider and rewrite `FILE` as the new golden transcript. This is intentionally explicit so CI can forbid it. |

The three LLM fixture modes are mutually exclusive. The explicit
`--llm-replay` and `--llm-record` spellings are also accepted when a script or
dashboard wants the fixture type in the flag name.

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
| `cases` | Evaluate blocks with `case_id`, `name`, source path, range, status, per-case `started_at`, duration, raw metrics, subcases, assertions, and diagnostics. |
| `metrics` | Top-level summaries aggregated from `eval.metric` values. Bool metrics report pass rate, true, and false counts. Number metrics report mean, min, and max. String metrics report category counts. |
| `comparison` | Optional baseline comparison with summary pass-rate deltas and metric deltas. |
| `findings` | TODO, IO, lex, parse, AI syntax, case runtime, and replay-drift findings. Replay mismatch findings include stable JSON `details.expected` and `details.actual` request summaries; exhausted and unconsumed replay findings include turn/count details. |
| `notes` | Explicit scope notes so callers do not confuse this with full eval scoring. |

Ordinary script execution still treats evaluate blocks as runtime no-ops. The
evaluation harness only changes `leia evaluate`.

Summary fields are intentionally dashboard-friendly. `evaluate_blocks` is the
number of discovered blocks before filtering. `cases_selected` is the number
that matched `--filter` and were listed or executed. `cases_skipped` is the
number filtered out. `cases_passed` and `cases_failed` count executed cases;
`cases_listed` counts `--list` results. `pass_rate` is
`cases_passed / (cases_passed + cases_failed)` and is `0` when no case ran.
