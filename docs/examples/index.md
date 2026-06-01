# Leia Examples

The `examples/` tree is part of the product surface. It shows how the language
is meant to feel for small scripts, host embeddings, AI agents, data-oriented
code, concurrency, web scripts, and game-style programs.

The repository-local example policy and dependency notes live in
[`../../examples/README.md`](../../examples/README.md).

## First Examples

```bash
go run ./cmd/leia run examples/hello/fib.leia
go run ./cmd/leia run examples/hello/types_demo.leia
go run ./cmd/leia run examples/hello/counter.leia
go run ./cmd/leia run examples/hello/functional.leia
```

| Directory | Focus |
|---|---|
| `examples/hello/` | Core syntax, functions, tables, closures, metatables, coroutines, errors, and object-style patterns. |
| `examples/data_processing/` | Data structures, string processing, dense arrays, and SoA kernels. |
| `examples/concurrency/` | Goroutines, channels, select, sync primitives, context cancellation, and process cancellation. |
| `examples/llm/` | AI-native models, tools, agents, direct agent-as-tool, custom flow, and live provider smoke scripts. |
| `examples/embedding/` | Go embedding examples as executable Go doc tests. |
| `examples/web/` | HTTP/server-oriented scripts. |
| `examples/game_engine/` | Game scripting patterns and larger interactive workloads. |

## Data-Oriented Examples

```bash
go run ./cmd/leia run examples/data_processing/data_oriented/soa_kernels.leia
go run ./cmd/leia run examples/data_processing/data_oriented/particle_integration.leia
```

Use these with the [data-oriented reference](../reference/data-oriented/index.md)
and `benchmarks/data/` when evaluating numeric or SoA-heavy code.

## Concurrency Examples

```bash
go run ./cmd/leia run examples/concurrency/goroutines_channels.leia
go run ./cmd/leia run examples/concurrency/select_timeout.leia
go run ./cmd/leia run examples/concurrency/sync_group_cancel.leia
```

Use these with the [concurrency reference](../reference/concurrency/index.md).

## AI Examples

```bash
go run ./cmd/leia run examples/llm/agent.leia
go run ./cmd/leia run examples/llm/incident_response.leia
```

Most AI examples are useful with a mock/replay provider in tests. Live-provider
examples such as `glm_smoke.leia` require opt-in environment variables and must
not be run with committed secrets.

See [AI-native Leia](../guides/ai-native.md).

## Embedding Examples

Run the Go examples:

```bash
go test ./examples/embedding -run Example -count=1
```

These cover compilation, public values, host functions, host modules, LLM
provider injection, `HotLoader`, and `HotInstance`.

See [Embedding Leia](../guides/embedding.md).

## Game And Web Examples

Game and web examples are source references for larger host integrations,
long-running servers, graphical bindings, and interactive workloads. They are
not first-run smoke commands.

Review these files directly when working on those areas:

- `examples/game_engine/game_of_life.leia`
- `examples/game_engine/tetris.leia`
- `examples/web/hello_server.leia`

Prefer the smoke, SDK, and integration tests for deterministic correctness
checks.
