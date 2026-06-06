# Leia Examples

The `examples/` tree is part of the product surface. It shows how the language
is meant to feel for small scripts, host embeddings, AI agents, data-oriented
code, concurrency, web scripts, and game-style programs.

The repository-local example policy and dependency notes live in
[`../../examples/README.md`](../../examples/README.md).
Commands on this page assume the current directory is the repository root.

## First Examples

Use the repository example entrypoints to discover runnable scripts, check the
registered set, and run one by ID or path:

```bash
go run ./cmd/leia examples list
go run ./cmd/leia examples show repo-hello-fib
go run ./cmd/leia examples check examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia
go run ./cmd/leia examples run repo-hello-fib
```

For tooling and CI, the same surface has stable JSON output:

```bash
go run ./cmd/leia examples list --json
go run ./cmd/leia examples check --json examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia
```

`examples check` executes runnable examples and reports manual examples as
skipped with their `requires` reason. `examples show` prints the registered
metadata followed by source, which is the quickest way to inspect the exact
entrypoint behind an example ID. Use `--jobs=N` when checking a larger selected
set.

| Directory | Focus |
|---|---|
| `examples/hello/` | Core syntax, functions, tables, closures, metatables, coroutines, errors, and object-style patterns. |
| `examples/api/` | Offline API-client style scripts for host-facing workflow examples. |
| `examples/automation/` | Offline business and release automation workflows. |
| `examples/dialects/` | Built-in text, protocol, SQL-shaped, Markdown/table, binary, and validation dialect examples. |
| `examples/data/` | Focused data-language examples, including q/kdb+-style symbolic vectors and SQLite-to-columnar analytics projects. |
| `examples/data_processing/` | Data structures, string processing, dense arrays, and SoA kernels. |
| `examples/database/` | Package-managed database capability examples. |
| `examples/concurrency/` | Goroutines, channels, select, sync primitives, context cancellation, and process cancellation. |
| `examples/ai/` | Tagged AI dialect workflows, replay/trace projects, and coding-agent replay examples. |
| `examples/llm/` | AI-native models, tools, agents, direct agent-as-tool, custom flow, and live provider smoke scripts. |
| `examples/embedding/` | Go embedding examples as executable Go doc tests. |
| `examples/evaluate/` | Deterministic evaluation and replay examples. |
| `examples/macos/` | Package-managed macOS automation capability examples. |
| `examples/operations/` | Local operations and release-risk reporting workflows. |
| `examples/performance/` | Execution mode and benchmark policy examples. |
| `examples/security/` | Supply-chain and vendor security workflow examples. |
| `examples/site/` | Static site and release dashboard generation examples. |
| `examples/testing/` | `leia test` workflow and JSONL golden-evaluation examples. |
| `examples/tooling/` | Release evidence, diagnostics, and cross-domain release-gate project examples. |
| `examples/ui/` | Package-managed UI capability examples. |
| `examples/web/` | HTTP/server-oriented scripts, including high-level `serve { ... }` route dialect examples. |
| `examples/game_engine/` | Game scripting patterns and larger interactive workloads. |
| `examples/workflow/` | Service-quality, status-rollup, and support-triage workflows. |

## Data-Oriented Examples

```bash
go run ./cmd/leia examples run examples/data_processing/data_oriented/soa_kernels.leia
go run ./cmd/leia examples check examples/data/q_vector_basics.leia
go run ./cmd/leia examples check examples/data/q_trade_analytics_project
go run ./cmd/leia examples check examples/data/db_q_frame_project
go run ./cmd/leia examples run repo-tooling-release_gate_project-main
go run ./cmd/leia examples check examples/data_processing/data_oriented/particle_integration.leia
```

Use these with the [data-oriented reference](../reference/data-oriented/index.md)
and `benchmarks/data/` when evaluating numeric or SoA-heavy code. The
`q_trade_analytics_project` example covers q/kdb+-style symbolic vectors,
dictionaries, scans, filters, and table rollups. The `db_q_frame_project`
example exercises SQLite `db.frame`, SoA-backed `q.query`, and `xlsx`/`excel`
round-tripping as one runnable data workflow. The
`release_gate_project` example combines fixture globbing, shell/process
dialects, SQLite, q-style columnar aggregation, Excel round-tripping, AI agent
mocking, and a loopback web route in one runnable release-gate workflow. The
particle integration example runs through the repository's higher-step-budget
example runner; use the explicit `examples check` command above instead of the
playground's default step budget when validating it.

## Concurrency Examples

```bash
go run ./cmd/leia examples check examples/concurrency/goroutines_channels.leia examples/concurrency/select_timeout.leia
go run ./cmd/leia examples run repo-concurrency-sync_group
```

Use these with the [concurrency reference](../reference/concurrency/index.md).

## Dialect Examples

```bash
go run ./cmd/leia examples check examples/hello/dialects.leia examples/dialects/text_parsing.leia examples/dialects/sql_result_analytics.leia examples/dialects/shell_filesystem.leia
go run ./cmd/leia examples run repo-dialects-sql_result_analytics
```

The runnable dialect examples cover the CLI-visible built-in dialect surface,
including shell/process literals, `env` lookup literals, `sql`,
`markdown`/`md`, Markdown table parsing/encoding, AI prompt/quote literals,
protocol fixtures, and data-format fixtures.

For a larger cross-domain dialect example:

```bash
go run ./cmd/leia examples run repo-tooling-release_gate_project-main
```

## AI Examples

AI examples under `examples/llm/` demonstrate agent declarations, direct
agent-as-tool, manual tool history, incident-response flow, and live GLM smoke
scripts. They require a host-injected mock/replay provider or explicit
live-provider environment variables, so they are not first-run smoke commands.
Never run live-provider examples with committed secrets.

See [AI-native Leia](../guides/ai-native.md).

## Evaluate Examples

```bash
go run ./cmd/leia evaluate --replay examples/evaluate/agent_replay.records.json examples/evaluate/agent_replay.leia
go run ./cmd/leia evaluate --replay examples/evaluate/llm_replay.records.json examples/evaluate/llm_replay.leia
go run ./cmd/leia evaluate --replay examples/evaluate/multiturn_replay.records.json examples/evaluate/multiturn_replay.leia
go run ./cmd/leia evaluate --replay examples/evaluate/project_agent_regression.records.json examples/evaluate/project_agent_regression.leia
```

Evaluate examples are source-level regression checks. Some are ordinary local
assertions; LLM examples pair the `.leia` source with a replay fixture so they
run deterministically without provider credentials. The project agent regression
fixture covers memory/history behavior, tool dispatch, and streaming replay
usage accounting.

## Embedding Examples

Run the Go examples:

```bash
go test ./examples/embedding -run Example -count=1
```

These cover compilation, public values, host functions, host modules, LLM
provider injection, `SecuritySandbox`, Go import allowlists, host result
budgets, `HotLoader`, and `HotInstance` state-preserving reloads.

See [Embedding Leia](../guides/embedding.md).

## Game And Web Examples

Game and web examples are source references for larger host integrations,
long-running servers, graphical bindings, and interactive workloads. They are
not first-run smoke commands unless `examples list` marks an entry as runnable.

Review these files directly when working on those areas:

- `examples/game_engine/event_system.leia`
- `examples/game_engine/game_of_life.leia`
- `examples/game_engine/tetris.leia`
- `examples/web/serve_dialect_app.leia`
- `examples/web/route_workbench.leia`
- `examples/web/access_log_report.leia`
- `examples/web/tiny_fullstack_app.leia`
- `examples/web/hello_server.leia`

Prefer the smoke, SDK, and integration tests for deterministic correctness
checks.

## Package-Managed Examples

Package-managed examples keep `leia.mod` metadata beside the script so package,
capability, and native binding evidence stays visible:

- `examples/database/package_managed/`
- `examples/macos/package_managed/`
- `examples/ui/package_managed/`
- `examples/tooling/release_gate_project/`

Use these with the [package guide](../guides/packages.md) and
[modules reference](../reference/modules/index.md).
