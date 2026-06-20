# Leia Performance And Benchmarks

Leia keeps performance evidence in the repository instead of treating it as an
external benchmark notebook. The benchmark harnesses are intended for local
optimization, public validation, and regression diagnosis.

## Benchmark Layout

Benchmarks live under `benchmarks/<domain>/<name>.leia`.

| Domain | Focus |
|---|---|
| `numeric` | Arithmetic kernels, matrices, nbody, spectral norm. |
| `recursion` | Recursive calls and recursive data structures. |
| `table` | Arrays, maps, traversal, sort, and metamethod tables. |
| `calls` | Closures, varargs, coroutine calls, and method dispatch. |
| `string` | String, pattern, regexp, UTF-8, and tokenization workloads. |
| `concurrency` | Goroutine-like tasks, channels, select, cancellation, and sync. |
| `data` | Data-oriented and SoA kernels. |
| `app` | Larger application-shaped workloads. |
| `control` | Control flow and protected execution. |

Benchmark selectors use the domain ID, for example `numeric/matmul` or
`table/json_table_walk`. Historical selector families are not accepted.

LuaJIT reference programs, where meaningful, live under
`benchmarks/lua_ref/<domain>/`.

## Commands

The CLI forwards to the Python harnesses:

```bash
leia bench --quick
leia bench numeric/matmul --runs 5 --warmup 1
leia bench --full --runs 5 --warmup 1 --sort luajit-gap
leia bench strict --runs 3 --warmup 1 \
  --json /tmp/leia_strict.json \
  --markdown /tmp/leia_strict.md
leia diagnose table/table_array_access --out-dir /tmp/leia-diag
```

The shell gate wraps the same harnesses with repository defaults:

```bash
scripts/run.sh perf --syntax-smoke --no-luajit
scripts/run.sh perf --smoke
scripts/run.sh perf --feature-smoke
scripts/run.sh perf --full
```

Use `--no-luajit` when LuaJIT is not installed or the workload has no useful
Lua reference. Without `--no-luajit`, script-timed current/LuaJIT rows are
validated against `--luajit-threshold` (default `0.80`).

For fast local checks, use `leia bench --quick`. For release-quality evidence,
write JSON and Markdown reports from the full and strict benchmark commands.
Pass `--jobs N` explicitly only for exploratory timing runs where local CPU
contention is acceptable.

## Timing Modes

`timing_compare.py` is the main optimization harness. It compares:

- the local checkout binary;
- a clean baseline binary built from `--head-ref`;
- optional LuaJIT reference timing.

It reports median time, coefficient of variation, repeat count, current-vs-HEAD
ratio, LuaJIT ratio, and Tier 2 exits.

Low-resolution script timers are not treated as wins. The harness can increase
repeat counts and fall back to repeated command wall time when the script time
is below the timer resolution.

## Strict Guard

Strict benchmark mode is the regression truth pass. It runs selected benchmarks
in these modes:

| Mode | Meaning |
|---|---|
| `vm` | Bytecode VM without JIT. |
| `default` | Normal execution with configured accelerators. |
| `no_filter` | JIT-enabled path with optimization filters disabled where supported. |
| `luajit` | External LuaJIT reference when available. |

The strict pass checks output stability, timing quality, suspicious
benchmark-only wins, and LuaJIT comparisons where references exist.

## Execution Performance Contract

The interpreter is the semantic baseline. The bytecode VM and ARM64 JIT are
execution accelerators, not a promise that every language feature or function
runs as native code: supported hot paths may run natively, but unsupported
operations must fall back to the VM/runtime without changing visible results,
errors, capability checks, resource-budget behavior, or deoptimization behavior.

The LuaJIT comparison is a validation baseline, not a marketing claim. For
script-timed rows with a Lua reference, `scripts/run.sh perf --full` runs the
performance submit guard and fails when `current / LuaJIT` exceeds the configured
`--luajit-threshold` (default `0.80`). Use `--no-luajit` only when LuaJIT is
unavailable or the selected workload has no meaningful Lua reference, and record
that limitation in release evidence.

Production and release plans keep this bottom line active through
`scripts/run.sh production --full --release-profile`,
`go run ./cmd/leia ci release --list`, and
`scripts/run.sh perf --full`.

## Artifacts

Benchmark commands can write JSON and Markdown reports:

```bash
leia bench --full \
  --json /tmp/leia_timing.json \
  --markdown /tmp/leia_timing.md

leia bench strict \
  --json /tmp/leia_strict.json \
  --markdown /tmp/leia_strict.md
```

`scripts/performance_gate.sh --validate-only FILE --json` validates an
existing timing report and exposes `failure_kind_count`, `failure_kinds`,
`failure_details`, `validate_target`, and captured `output_lines` for release
dashboards. `validate_target` records the checked timing report path, whether
it exists, whether it is a file, and its byte size.

Release and diagnostic scripts collect these artifacts into evidence bundles.
Do not commit temporary timing output. Only maintainers should update
intentional baseline or history files under `benchmarks/data/`.

## Adding Benchmarks

Add a benchmark when it covers a language feature, runtime mechanism, stdlib
path, or user-facing workload shape that is not already represented.

Rules:

- place the `.leia` file under the correct domain;
- print a stable result so the harness can detect semantic breakage;
- print `Time: <seconds>` for script-level timing when possible;
- add a Lua reference only when the workload maps naturally to Lua;
- avoid benchmark-specific implementation hooks in runtime or JIT code.

Use `bash benchmarks/coverage_guard.sh` to audit semantic-family performance
coverage.
