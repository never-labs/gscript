# Tooling

Leia treats tools as part of the language product.

## Daily Loop

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke/01_basic.leia
go run ./cmd/leia check --no-docs .
```

`leia check` runs formatter, linter, `.leia` tests, manifest coverage, and docs
checks. Use skip flags only while iterating:

```bash
go run ./cmd/leia check --no-test --no-docs tests/smoke/01_basic.leia
go run ./cmd/leia check --json .
```

## CI Profiles

```bash
go run ./cmd/leia ci smoke --list
go run ./cmd/leia ci smoke
go run ./cmd/leia ci pr --no-luajit
go run ./cmd/leia ci perf --no-luajit
```

Profiles:

| Profile | Use |
|---|---|
| `smoke` | Fast local sanity: selected Go tests, manifest coverage, module path gate, tooling check, worktree audit. |
| `pr` | Full Go tests, manifest coverage, docs check, and performance smoke. |
| `perf` | Full performance gate. |
| `release` | Production and release artifact checks. |

Use `--list` before running a profile when you want to see exactly which shell
commands will execute.

The checked-in CI profile is the source of truth for hosted or local automation.
If hosted CI is unavailable, contributors can still reproduce the same command
set locally:

```bash
go run ./cmd/leia ci smoke --list
go run ./cmd/leia ci pr --list
go run ./cmd/leia ci release --list
```

Run `smoke` before small changes, `pr` before review, and `release` only when
preparing a tag or release candidate.

## Manifest And Worktree Checks

```bash
python3 tests/manifest.py check tests benchmarks
bash scripts/worktree_audit.sh
```

The manifest check keeps test and benchmark discovery explicit. The worktree
audit catches stale or confusing local worktrees before large refactors.

## Documentation

```bash
go run ./cmd/leia doc generate --layout site --output docs
go run ./cmd/leia doc generate --format json
go run ./cmd/leia doc check
bash scripts/docs_check.sh
```

Generated reference pages are checked in. `scripts/docs_check.sh` verifies
generated CLI/stdlib references, Markdown links, release reference coverage,
and retired naming.

## Performance

```bash
go run ./cmd/leia bench --quick
go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1
go run ./cmd/leia bench strict --bench table/table_array_access --runs 3 --warmup 1 \
  --json /tmp/leia-strict.json \
  --markdown /tmp/leia-strict.md
bash scripts/performance_gate.sh --feature-smoke
```

Use `--no-luajit` when LuaJIT is not installed or when a benchmark has no useful
Lua reference. See the [performance reference](../reference/performance/index.md).

## Diagnostics

```bash
go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks
bash scripts/diag.sh table/table_array_access
```

Use diagnostics bundles when filing performance or correctness issues. They
collect environment, docs/test status, and optional benchmark summaries.

## Release Evidence

```bash
bash scripts/production_check.sh --quick
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
bash scripts/release_artifacts_check.sh
```

The release process is documented in [Release Process](../release/index.md).
