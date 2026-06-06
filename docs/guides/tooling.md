# Tooling

Leia treats tools as part of the language product.

## Daily Loop

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke/01_basic.leia
go run ./cmd/leia test --json --output test-report.json tests/smoke/01_basic.leia
go run ./cmd/leia check --no-docs --no-editor --no-examples .
```

`leia check` runs formatter, linter, `.leia` tests, manifest coverage, and docs
checks, editor asset checks, and runnable repository example checks. Use skip
flags only while iterating:

```bash
go run ./cmd/leia check --no-test --no-docs tests/smoke/01_basic.leia
go run ./cmd/leia check --json .
```

Use JSON reports when another tool or CI step needs stable machine-readable
results:

```bash
go run ./cmd/leia test --json --output test-report.json tests/smoke/01_basic.leia
go run ./cmd/leia evaluate --json --report eval-report.json examples/evaluate/basic_assert.leia
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

## Modules

```bash
go run ./cmd/leia mod verify --json examples/ui/package_managed
go run ./cmd/leia mod verify --json examples/tooling/package_manager_workflow
go run ./cmd/leia mod graph --json examples/ui/package_managed
go run ./cmd/leia mod capability --json examples/ui/package_managed
```

`leia mod` manages local module metadata, require graphs, vendored dependency
plans, lockfiles, native `go.mod` interop, and declared capability evidence.
Use the [modules reference](../reference/modules/index.md) for the full command
surface.

## Documentation

```bash
go run ./cmd/leia doc generate --layout site --output docs
go run ./cmd/leia doc generate --format json
go run ./cmd/leia doc check
bash scripts/docs_check.sh
```

Generated reference pages are checked in. `scripts/docs_check.sh` verifies
generated CLI/stdlib/dialect references, spec HTML freshness, spec runnable examples,
Markdown links, release reference coverage, retired naming, and documented
repository script entrypoints.

## Editors

```bash
bash scripts/editor_check.sh
bash scripts/editor_check.sh --require-tree-sitter
python3 -m unittest tools.editor.smoke.editor_check_test
```

The editor gate validates shared TextMate grammars, VS Code syntax assets,
snippets, language configuration, extension JavaScript syntax, the spec preview
helper, editor smoke fixtures, and tree-sitter corpus tests when the CLI is
available. Use `--require-tree-sitter` in environments where the tree-sitter
dependency is expected to be installed.

## Examples

```bash
go run ./cmd/leia examples list
go run ./cmd/leia examples show repo-hello-fib
go run ./cmd/leia examples check examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia
go run ./cmd/leia examples run repo-hello-fib
```

`leia examples check` is the same checker used by the repository-wide
`leia check` examples step. Runnable examples execute locally; manual examples
are reported as skipped with their required host capability, service,
credential, or step-budget reason. When an example lives inside a directory
with `leia.mod`, the checker validates that module with `leia mod verify --json`
before running the example so package metadata cannot drift away from runnable
project examples.

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
Lua reference. When LuaJIT data is present and script-timed, the gate also
enforces `--luajit-threshold` so README performance claims cannot drift into a
report-only comparison. See the
[performance reference](../reference/performance/index.md).

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
go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1
bash scripts/release_artifacts_check.sh --build
```

The release process is documented in [Release Process](../release/index.md).
