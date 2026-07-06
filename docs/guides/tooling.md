# Tooling

Leia treats tools as part of the language product.

## Daily Loop

```bash
go run ./cmd/leia eval 'print(1 + 2 + 3)'
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia fmt --check --json tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia lint --json tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke/01_basic.leia
go run ./cmd/leia examples check examples/hello/dialects.leia
go run ./cmd/leia test --json --output test-report.json tests/smoke/01_basic.leia
go run ./cmd/leia check --no-docs --no-editor --no-examples .
go run ./cmd/leia check --quick .
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
go run ./cmd/leia capabilities --json
go run ./cmd/leia test --json --output test-report.json tests/smoke/01_basic.leia
go run ./cmd/leia evaluate --json --report eval-report.json examples/evaluate/basic_assert.leia
```

`capabilities --json` includes `tooling.report_count` and `tooling.reports`,
the registry of CLI and release-script JSON reports.
Each registry entry advertises `status_field`, `scalar_fields`, `count_fields`,
`collection_fields`, and `collection_item_fields` when the report exposes
machine-readable release evidence. Field names use dotted JSON paths; `[]`
marks per-item array paths such as `commands[].args`.
Entries with `template: true` contain placeholders such as `FILE`, `DIST_DIR`,
or `BIN_DIR`; callers must substitute the listed `template_parameters` before
executing the command.
Release dashboards should read those fields instead of scraping human output;
for example,
`scripts/run.sh public-blockers --json` exposes blocker kind counts
and `scripts/run.sh production --list --json` exposes runnable, skipped, and
release-critical run/skip counts plus structured skip details and the configured
release-critical names.

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
| `smoke` | Fast local sanity: selected Go tests, manifest coverage, module path validation, and tooling checks. |
| `pr` | Full Go tests, manifest coverage, docs check, and performance smoke. |
| `perf` | Full performance validation. |
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
go run ./cmd/leia ci release --release-version vX.Y.Z --list
go run ./cmd/leia ci release --release-version vX.Y.Z --list --json
scripts/run.sh worktree --json
```

`scripts/run.sh worktree --json` reports `finding_status_count` and
`finding_statuses` for dirty, prunable, missing, unreadable, and ahead/behind
worktree summaries.

Run `smoke` before small changes, `pr` before review, and `release` only when
preparing a tag or release candidate.

## Manifest Checks

```bash
scripts/run.sh manifest-check tests benchmarks
scripts/run.sh worktree
```

The manifest check keeps test and benchmark discovery explicit so local and
hosted validation run the same discovered coverage.

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
go run ./cmd/leia doc check --json
scripts/run.sh docs
```

Generated reference pages are checked in. `scripts/run.sh docs` verifies
generated CLI/stdlib/dialect references, spec HTML freshness, spec runnable examples,
Markdown links, release reference coverage, retired naming, and documented
repository script entrypoints. Use `--json` for machine-readable documentation
evidence with `failure_kinds` and `failure_details`.

GitHub Pages publishes `docs/` through `.github/workflows/pages.yml`. The
workflow runs `scripts/run.sh docs` before building the site and
`scripts/run.sh site --site-dir ./_site` after rendering.

## Editors

```bash
scripts/run.sh editor
scripts/run.sh editor --require-tree-sitter
go run ./cmd/leia editor smoke
```

The editor check validates shared TextMate grammars, VS Code syntax assets,
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

## Playground

```bash
go run ./cmd/leia playground --help
go run ./cmd/leia playground --addr 127.0.0.1:8080
```

The browser playground serves local execution APIs plus runnable Playground,
Tour, Examples, Evaluate, and AI tabs. It is backed by checked-in repository
examples, so playground content is exercised by the same example and release
validation as command-line examples.

## Performance

```bash
go run ./cmd/leia bench --quick
go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1
go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3
go run ./cmd/leia bench strict --bench table/table_array_access --runs 3 --warmup 1 \
  --json /tmp/leia-strict.json \
  --markdown /tmp/leia-strict.md
scripts/run.sh perf --feature-smoke
scripts/run.sh perf --validate-only /tmp/leia_performance_gate/timing_gate.json --json
```

Use `--no-luajit` when LuaJIT is not installed or when a benchmark has no useful
Lua reference. When LuaJIT data is present and script-timed, the check also
enforces `--luajit-threshold` so published performance claims remain tied to
measured validation instead of report-only comparisons. See the
[performance reference](../reference/performance/index.md).

## Diagnostics

```bash
go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks
go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-go-tests --skip-benchmarks --json
scripts/run.sh diag table/table_array_access
```

Use diagnostics bundles when filing performance or correctness issues. They
collect environment, docs/test status, and optional benchmark summaries.

## Release Evidence

```bash
go run ./cmd/leia capabilities --json
scripts/run.sh production --quick
scripts/run.sh production --quick --list --json
scripts/run.sh production --quick --list --out-dir /tmp/leia-release-plan
scripts/run.sh test release-matrix
scripts/run.sh editor --json
scripts/run.sh release-dist --json
scripts/run.sh release-check --json --version vX.Y.Z
scripts/run.sh release-check --build
```

The release process is documented in [Release Process](../release/index.md).
