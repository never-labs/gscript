# Leia Examples

Examples are grouped by what they demonstrate and by whether they need host
services.

## No-Network Scripts

These should run from a clean checkout without secrets:

```bash
# from the examples/ directory
go run ../cmd/leia examples list
go run ../cmd/leia examples check hello/fib.leia hello/types_demo.leia hello/dialects.leia
go run ../cmd/leia examples check dialects/text_parsing.leia dialects/sql_result_analytics.leia
go run ../cmd/leia examples check data/q_vector_basics.leia
go run ../cmd/leia examples check data/db_q_frame_project
go run ../cmd/leia examples check automation/invoice_reconciliation.leia
go run ../cmd/leia examples check ai/coding_agent_project
go run ../cmd/leia examples check operations/local_ops_report.leia
go run ../cmd/leia examples check database/package_managed
go run ../cmd/leia examples check web/serve_dialect_app.leia
go run ../cmd/leia examples check web/tiny_fullstack_app.leia
go run ../cmd/leia examples check site/static_docs_generator.leia site/release_dashboard.leia
go run ../cmd/leia examples run data_processing/data_oriented/soa_kernels.leia
go run ../cmd/leia examples run concurrency/select_timeout.leia
go run ../cmd/leia examples check concurrency/pipeline_project
```

Directories:

| Directory | Purpose |
|---|---|
| `hello/` | Core language features and small idioms. |
| `api/` | Offline API-client style scripts for host-facing workflow examples. |
| `dialects/` | Runnable checks for built-in shell/env, text, protocol, SQL-shaped, Markdown/table, binary, and validation dialects. |
| `automation/` | Project-level offline workflows for release, fixture, and business-ops automation. |
| `ai/` | Replay-backed AI workflows, including a project-level coding-agent repair loop. |
| `operations/` | Project-level offline workflows for local logs, backup hygiene, deploy risk, and ops reporting. |
| `tooling/` | Project-level offline workflows for release evidence, diagnostics, and CLI gate planning. |
| `performance/` | User-facing execution mode and benchmark policy examples. |
| `data/` | Focused data-language examples, including q/kdb+-style symbolic vector evaluation and SQLite-to-columnar q analytics. |
| `data_processing/` | Strings, containers, dense data, vectors, matrices, and SoA. |
| `database/` | Package-managed SQLite ledger analytics project. |
| `concurrency/` | Goroutine-like tasks, channels, select, sync, and context helpers. |
| `embedding/` | Go embedding examples as executable Go doc tests and hot-reload project tests. |
| `evaluate/` | Deterministic evaluation and replay examples. |
| `llm/` | LLM models, tools, agents, direct turns, streaming, and provider smoke scripts. |
| `macos/` | Package-managed macOS automation capability examples. |
| `security/` | Supply-chain and vendor security workflow examples. |
| `site/` | Static site and release dashboard generation examples. |
| `testing/` | `leia test` workflow and JSONL golden-evaluation examples. |
| `ui/` | Package-managed UI capability examples. |
| `web/` | HTTP/server-oriented scripts, including high-level `serve { ... }` route dialect examples. |
| `game_engine/` | Larger script examples for event loops and game-style state. |
| `workflow/` | Service-quality, status-rollup, and support-triage workflows. |

## Embedding

The Go embedding examples are executable doc tests:

```bash
go test ./embedding -run Example -count=1
go test ./embedding/hot_reload_project -count=1
```

They cover public value conversion, host functions, host modules, LLM provider
injection, hot reload, persistent instances, and a project-level reload gate
with rollback, host import allowlisting, and budget preservation.

## AI And Host-Backed Examples

AI examples live under `ai/` and `llm/`. Most are intended to work with a mock
or replay provider in tests. `ai/coding_agent_project/main.leia` is the
project-level offline coding-agent gate, while `llm/direct_turn.leia` shows the
ordinary `llm.turn` request shape without an agent wrapper. Live-provider examples
require `LEIA_LLM_INTEGRATION=1` plus provider environment variables and must
never commit API keys.

Evaluate examples live under `evaluate/`. Run replay-backed agent checks with:

```bash
# from the examples/ directory
go run ../cmd/leia evaluate --replay evaluate/agent_replay.records.json evaluate/agent_replay.leia
go run ../cmd/leia evaluate --replay evaluate/judge_replay.records.json evaluate/judge_replay.leia
go run ../cmd/leia evaluate --replay evaluate/multiturn_replay.records.json evaluate/multiturn_replay.leia
go run ../cmd/leia evaluate --replay evaluate/project_agent_regression.records.json evaluate/project_agent_regression.leia
```

Examples that open network listeners or touch host resources, such as `web/`,
should be run intentionally and reviewed with the security reference. The
`web/serve_dialect_app.leia` example is the deterministic smoke for the
high-level `serve { ... }` route dialect; `web/tiny_fullstack_app.leia`
combines `serve`, SQLite, HTML, JSON, form handling, and static assets in one
runnable full-stack smoke.

## Release Expectations

Examples linked from the README or release notes should either:

- run without external services; or
- clearly state required environment variables, capabilities, network access,
  and whether they are live-provider smoke tests.

The curated documentation page is [`../docs/examples/index.md`](../docs/examples/index.md).
