# Performance Change Evidence

Performance changes need enough evidence to separate real improvements from
timing noise, startup effects, or benchmark-specific behavior.

## Minimum Evidence

For small performance-sensitive changes, include:

```bash
go run ./cmd/leia bench compare --bench selector/name --runs 5 --warmup 1
bash scripts/performance_gate.sh --feature-smoke
```

For JIT, VM, table, call, concurrency, stdlib hot-path, or benchmark harness
changes, include the selector changed, neighboring selectors that could regress,
platform, CPU, Go version, and whether LuaJIT was available.

## Interpreting Results

- Prefer medians over single runs.
- Treat high coefficient of variation as inconclusive.
- Explain suspiciously large wins.
- Compare hot-loop timing, not CLI startup time.
- Check correctness before trusting a speedup.
- Avoid benchmark-specific recognizers; optimizations should be runtime-shape,
  semantic, or library-contract driven.

## Useful Commands

```bash
go run ./cmd/leia bench --quick
go run ./cmd/leia bench compare --bench numeric/matmul_dense --runs 5 --warmup 1
go run ./cmd/leia bench strict --bench table/table_array_access --runs 5 --warmup 1 \
  --json /tmp/leia-strict.json \
  --markdown /tmp/leia-strict.md
bash scripts/performance_gate.sh --feature-smoke
bash scripts/performance_gate.sh --full --no-luajit
```

Use `--no-luajit` only when LuaJIT is not installed or the workload has no
meaningful Lua reference. Say so in the PR.

## Reporting Format

```text
Platform: darwin/arm64, Apple M-series, Go X.Y.Z
Command:  go run ./cmd/leia bench compare --bench table/table_array_access --runs 5 --warmup 1
Result:   Leia median A -> B, LuaJIT C, ratio D
Noise:    CV E%
Notes:    why this is general and which adjacent selectors were checked
```

Release-level performance evidence belongs in the release notes template under
`docs/release/notes-template.md`.
