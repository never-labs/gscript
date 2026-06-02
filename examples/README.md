# Leia Examples

Examples are grouped by what they demonstrate and by whether they need host
services.

## No-Network Scripts

These should run from a clean checkout without secrets:

```bash
go run ../cmd/leia run hello/fib.leia
go run ../cmd/leia run hello/types_demo.leia
go run ../cmd/leia run data_processing/data_oriented/soa_kernels.leia
go run ../cmd/leia run concurrency/select_timeout.leia
```

Directories:

| Directory | Purpose |
|---|---|
| `hello/` | Core language features and small idioms. |
| `data_processing/` | Strings, containers, dense data, vectors, matrices, and SoA. |
| `concurrency/` | Goroutine-like tasks, channels, select, sync, and context helpers. |
| `game_engine/` | Larger script examples for event loops and game-style state. |

## Embedding

The Go embedding examples are executable doc tests:

```bash
go test ./embedding -run Example -count=1
```

They cover public value conversion, host functions, host modules, LLM provider
injection, hot reload, and persistent instances.

## AI And Host-Backed Examples

AI examples live under `llm/`. Most are intended to work with a mock or replay
provider in tests. Live-provider examples require opt-in environment variables
and must never commit API keys.

Evaluate examples live under `evaluate/`. Run replay-backed agent checks with:

```bash
go run ../cmd/leia evaluate --llm-replay evaluate/agent_replay.records.json evaluate/agent_replay.leia
go run ../cmd/leia evaluate --llm-replay evaluate/multiturn_replay.records.json evaluate/multiturn_replay.leia
```

Examples that open network listeners or touch host resources, such as `web/`,
should be run intentionally and reviewed with the security reference.

## Release Expectations

Examples linked from the README or release notes should either:

- run without external services; or
- clearly state required environment variables, capabilities, network access,
  and whether they are live-provider smoke tests.

The curated documentation page is [`../docs/examples/index.md`](../docs/examples/index.md).
