# Leia Cookbook

Cookbook recipes are short, copyable starting points. Each recipe points to a
checked-in example or command that can be run locally.

## Clean And Group Records

Use tables for record-like data and standard-library helpers for cleanup.

```bash
go run ./cmd/leia examples run examples/data_processing/string_processing.leia
```

Related references:

- [Tables and metatables](../spec/tables.md)
- [Standard library](../reference/stdlib/index.md)

## Run Script Tests With A JSON Report

Use `leia test` for deterministic script tests. JSON output is suitable for CI
dashboards and local tooling.

```bash
go run ./cmd/leia test --json --output test-report.json tests/smoke/01_basic.leia
```

Use `--golden=require` when every test must have an expected stdout file.

## Evaluate An Agent With Replay

Use `evaluate` blocks for agent regressions and replay fixtures for
deterministic runs.

```bash
go run ./cmd/leia evaluate --format=text --replay examples/evaluate/agent_replay.records.json examples/evaluate/agent_replay.leia
```

Update a fixture only through the explicit golden update mode:

```bash
go run ./cmd/leia evaluate --format=text --update-golden examples/evaluate/agent_replay.records.json examples/evaluate/agent_replay.leia
```

Related references:

- [Evaluate reference](../reference/evaluate/index.md)
- [AI-native guide](../guides/ai-native.md)

## Embed Leia In A Go Service

Start with the executable Go examples:

```bash
go test ./examples/embedding -run Example -count=1
```

They cover compiling a script, exposing Go functions, injecting LLM providers,
and using hot reload.

Related references:

- [Embedding guide](../guides/embedding.md)
- [Embedding reference](../reference/embedding/index.md)
- [Hot reload](../reference/hot-reload/index.md)

## Run Concurrent Work

Use goroutines and channels for fan-out/fan-in workloads.

```bash
go run ./cmd/leia examples check examples/concurrency/goroutines_channels.leia examples/concurrency/select_timeout.leia
```

Use cancellation-aware host operations for long-running scripts.

```bash
go run ./cmd/leia examples run examples/concurrency/context_cancel.leia
```

Related reference:

- [Concurrency](../reference/concurrency/index.md)

## Use Data-Oriented Arrays

Use dense arrays and SoA helpers for numeric kernels or column-oriented data.

```bash
go run ./cmd/leia examples run examples/data_processing/data_oriented/soa_kernels.leia
go run ./cmd/leia examples check examples/data_processing/data_oriented/particle_integration.leia
```

Related references:

- [Data-oriented programming](../reference/data-oriented/index.md)
- [Performance and benchmarks](../reference/performance/index.md)

## Inspect Tooling And Environment

Use local tooling checks before sending a change for review.

```bash
go run ./cmd/leia env
go run ./cmd/leia check --no-docs .
bash scripts/production_check.sh --quick
```

Related guide:

- [Tooling](../guides/tooling.md)
