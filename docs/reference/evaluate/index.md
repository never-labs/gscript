# Leia Evaluate Reference

`leia evaluate` is the entry point for agent evaluation work. It emits a
versioned report and runs each discovered evaluate block body as ordinary Leia
code under the evaluation harness.

The command parses Leia source, runs existing AI-native source validation, and
discovers `evaluate "case name" { ... }` blocks. For each case it executes the
file's top-level setup code and then the evaluate body, so normal Leia
assertions such as `assert(result == "ok")` determine whether the case passes.
It reports local `TODO` lines as informational findings and summarizes evaluate
blocks, selected cases, executed cases, assertions, metrics, and LLM usage.
During the run, the harness-only `eval` module can load JSONL corpora, run named
subcases, record metrics, inspect LLM usage, enforce local budgets, run bounded
judge turns, skip fixtures, and fail gates without adding more syntax.

```sh
leia evaluate --json path/to/script.leia
leia evaluate --json --report eval-report.json path/to/project
leia evaluate --format=text path/to/project
leia evaluate --format=html --report eval-report.html path/to/project
leia evaluate --gate tests/agents
leia evaluate --parallel=8 tests/agents
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
| `--parallel N` | Execute selected top-level evaluate blocks with up to `N` workers. Discovery and report merging stay deterministic. LLM record, replay, and update-golden modes run serially so fixture turn order remains stable. |
| `--baseline FILE` | Load a previous JSON evaluate report and attach a comparison section to the current report. |
| `--compare OLD NEW` | Compare two existing JSON evaluate reports without executing source. The comparison is attached to `NEW` and rendered with the selected `--format`. |
| `--regression-threshold N` | Allow bool pass-rate regressions up to `N` when `--baseline` is used. Summary pass-rate and bool metric pass-rate regressions beyond the threshold fail the report. Number and string metrics are compared but not treated as regressions because directionality is metric-specific. |
| `--replay FILE` | Use `FILE` as a deterministic provider transcript. Request mismatches, exhausted replay, or unconsumed turns fail the report. |
| `--record FILE` | Run against the configured provider and write observed turns to `FILE`. |
| `--update-golden FILE` | Run against the configured provider and rewrite `FILE` as the new golden transcript. This is intentionally explicit so CI can forbid it. |

The three LLM fixture modes are mutually exclusive. The explicit
`--llm-replay` and `--llm-record` spellings are also accepted when a script or
dashboard wants the fixture type in the flag name.

When `--record` or `--update-golden` is used, the command still writes the
global fixture requested by the flag. It also writes per-case fixtures under a
stable sibling directory such as `agent.records.cases/001-refund-flow.records.json`
and reports each path as `cases[].llm.record_path`. These per-case fixtures are
for debugging and future replay workflows; the global fixture remains the
canonical replay input.

## Harness module

`leia evaluate` installs an `eval` module only while it executes evaluate
blocks. Ordinary script execution does not install this module.

The `cost` and `money` keys below are evaluate harness and report metrics only.
They let evaluation jobs gate on provider-reported cost metadata when available;
they do not make money accounting a stable script-level AI budget dimension.

| Function | Meaning |
|---|---|
| `eval.case(id, fn)` | Run one named subcase. Runtime errors inside `fn` fail that subcase and return `(false, err)` without stopping later subcases. |
| `eval.metric(name, value)` | Record a bool, number, string, nil, or JSON-like metric on the active subcase, or on the outer evaluate block when no subcase is active. |
| `eval.load_jsonl(path)` | Load a JSON Lines corpus relative to the evaluate source file. |
| `eval.skip_if(cond, reason)` | Mark the active subcase skipped when `cond` is truthy. Returns whether it skipped. |
| `eval.fail_if(cond, message)` | Stop the current evaluate block when `cond` is truthy. |
| `eval.usage()` | Return current case LLM usage: `turns`, `input_tokens`, `output_tokens`, `tokens`, `latency_ms`, `cost`, `tool_calls`, `stream_events`, and `errors`. |
| `eval.budget(table)` | Fail the current case when current LLM usage exceeds a positive limit in `table`. Supported keys are `turns`, `tokens`, `input_tokens`, `output_tokens`, `latency_ms`, `cost`, and `money`. |
| `eval.judge(options)` | Call `llm.turn(options)` for judge-style scoring. If absent, it defaults `max_tokens` to `200` and `budget` to `{tokens: 512, turns: 1}`. It records `judge_cost`, `judge_input_tokens`, `judge_output_tokens`, and `judge_tokens` metrics when provider usage is available. |

Example:

```leia
evaluate "refund classifier corpus" {
    cases := eval.load_jsonl("refund_cases.jsonl")
    for _, row := range cases {
        eval.case(row.id, func() {
            result, err := classify_refund(row.input)
            assert(err == nil)
            eval.metric("correct", result.action == row.expected)
            eval.budget({tokens: 5000, cost: 0.02})
        })
    }
}
```

The JSON report is versioned with `schema_version: 1` and includes:

| Field | Meaning |
|---|---|
| `phase` | Currently `runtime-minimal`. |
| `status` | `ok` or `failed`. Syntax, validation, and case runtime errors make the report fail. |
| `started_at` | UTC RFC3339 timestamp for the evaluation run. |
| `runtime` | Leia and Go runtime metadata: version, OS/arch, and build VCS fields when available. |
| `summary` | File, parse, TODO, selected/skipped case, pass/fail/list, assertion, duration, and pass-rate counts. |
| `llm` | Optional LLM metadata: mode, fixture paths, loaded/replayed/remaining/recorded turn counts, aggregate turn count, provider errors, input/output tokens, latency, and cost. |
| `inputs` | Per-input file status. |
| `cases` | Evaluate blocks with `case_id`, `name`, source path, range, status, per-case `started_at`, duration, optional per-case `llm` stats and `llm.record_path`, raw metrics, subcases, assertions, and diagnostics. |
| `metrics` | Top-level summaries aggregated from `eval.metric` values. Bool metrics report pass rate, true, and false counts. Number metrics report mean, min, and max. String metrics report category counts. |
| `comparison` | Optional baseline comparison with summary pass-rate deltas and metric deltas. |
| `findings` | TODO, IO, lex, parse, AI syntax, case runtime, and replay-drift findings. Replay mismatch findings include stable JSON `details.expected` and `details.actual` request summaries plus `case_id`/`case_name` attribution when the drift happened inside an evaluate case; exhausted and unconsumed replay findings include turn/count details. |
| `notes` | Explicit scope notes so callers do not confuse this with full eval scoring. |

Ordinary script execution still treats evaluate blocks as runtime no-ops. The
evaluation harness only changes `leia evaluate`.

Summary fields are intentionally dashboard-friendly. `evaluate_blocks` is the
number of discovered blocks before filtering. `cases_selected` is the number
that matched `--filter` and were listed or executed. `cases_skipped` is the
number filtered out. `cases_passed` and `cases_failed` count executed cases;
`cases_listed` counts `--list` results. `pass_rate` is
`cases_passed / (cases_passed + cases_failed)` and is `0` when no case ran.
