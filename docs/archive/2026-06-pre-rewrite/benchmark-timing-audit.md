# Benchmark Timing Audit

This audit covers the benchmark timing mechanisms used by `benchmarks/official_hot`
and the full `timing_compare.py` path. It is intentionally documentation-only:
no benchmark scripts or harness code were changed.

## Current Commands

Repeatable performance gate for core hot paths:

```bash
bash scripts/performance_gate.sh
```

Short smoke gate:

```bash
bash scripts/performance_gate.sh --smoke
```

Primary full timing command documented in `benchmarks/README.md` and
`docs/performance.md`:

```bash
python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --sort=luajit-gap \
  --json /tmp/leia_timing_compare.json \
  --markdown /tmp/leia_timing_compare.md
```

Hot-loop scaling profile for short workloads:

```bash
python3 benchmarks/timing_compare.py --runs=5 --warmup=1 \
  --scale-profile=hot --sort=luajit-gap \
  --json /tmp/leia_hot_timing.json \
  --markdown /tmp/leia_hot_timing.md
```

Script-only timing for publishable hot-loop claims:

```bash
python3 benchmarks/timing_compare.py --all-groups --runs=7 --warmup=2 \
  --timeout=900 --time-source=script
```

Official hot subset through strict guard:

```bash
python3 benchmarks/strict_guard.py --group=official --runs=3 --warmup=1 \
  --timeout=90 --allow-wall-time
```

Warm Go microbenchmarks, which use a separate Go benchmark timer and do not
measure CLI process startup or script parsing:

```bash
go test ./benchmarks/ -bench=Warm -benchtime=3s
```

Note: the top-level `README.md` currently shows `timing_compare.py --all`, but
the parser exposes `--all-groups`. Treat `--all` as stale documentation unless
the harness grows that alias.

## Current State

`scripts/performance_gate.sh` is the stable gate entrypoint for performance
regression checks. It deliberately stays outside the JIT/VM implementation: it
selects representative hot benchmarks, invokes `timing_compare.py` with
calibrated repeats and the hot scale profile, validates the JSON artifact, and
then invokes `strict_guard.py` for the requested selector set, or a smaller
VM-safe truth subset for the default core profile, unless disabled. The validator
prints a sorted current/clean-HEAD slowdown table, fails ordinary script-timed
rows above the configured threshold, applies a separate wider threshold to
wall-timed rows, and fails any row that remains low-resolution or unavailable.

`timing_compare.py` is the main full timing harness. It exports a clean `HEAD`
snapshot, builds current and `HEAD` binaries, optionally runs LuaJIT, and
discovers `suite`, `extended`, `variants`, and `official` groups. With
`--all-groups`, `official_hot` is included as group `official`.

Each `timing_compare.py` cell runs a command repeatedly until the summed sample
time is large enough. The in-script `Time:` value is parsed first. If script
time is below the configured floor and wall fallback is allowed, the harness
uses high-resolution Python `time.perf_counter()` command wall time. Output
labels include `script_repeat`, `wall_repeat`, or `wall_hr`; low script timing
can become `low_resolution` when fallback is forbidden.

`strict_guard.py` has a similar repeat calibration path, but with different
defaults: default groups are `suite`, `extended`, and `variants`; `official`
must be requested explicitly. Wall fallback is disabled unless
`--allow-wall-time` is set. It also marks `control/defer_protected` as a
logical-time benchmark and prefers wall time for that case when wall timing is
allowed.

The official hot scripts mostly print `Time: %.3fs` from an in-script timer:
Leia uses `time.now()` / `time.since()`, while Lua uses `os.clock()`.
`control/defer_protected` is the exception: both Leia and Lua versions
print deterministic synthetic `logicalTime`, not elapsed wall or CPU time.

The hot region is not fully uniform across official hot cases. Most scripts put
setup and warmup outside the timer and time only one workload call. Examples:
`events_metamethod_hot` and `nextvar_table_hot` run a warm call before `t0`.
`strings_patterns_hot` starts the timer before building the generated text blob,
so its timed region includes setup plus hot string/pattern work. This is still
valid as an end-to-end benchmark, but it is not the same thermal boundary as the
other official hot files.

Go `Benchmark*Warm` tests are a third mechanism. They construct and warm a VM
before `b.ResetTimer()` and then repeatedly call an already-defined function.
Those numbers are useful for steady-state VM/JIT call throughput, but they are
not comparable to CLI script benchmarks unless the report says startup, parse,
compile, and warmup are excluded.

## Risks

Low-resolution risk: many current results are a few milliseconds or less after
repeat division. The raw scripts print only millisecond precision (`%.3fs`), so
a single run can report `0.000s` or `0.001s`. Repeating helps, but the per-run
median still inherits quantization from the script timer unless the workload is
scaled above the floor.

Startup-noise risk: `wall_repeat` and `wall_hr` include process startup, script
load, parsing, compilation, JIT stats printing, and LuaJIT startup. This is the
right measurement for startup/end-to-end latency, but it can mis-rank hot-loop
throughput. `timing_compare.py` already suppresses Current/LuaJIT gap ranking
when either side is wall-timed, but the raw table still needs explicit source
fields.

The performance gate treats wall-timed current/HEAD rows as noisier evidence:
they stay visible in the sorted table and can still fail when the regression is
large enough, but they use a wider threshold than script-timed rows. This keeps
startup fallback from hiding severe slowdowns without letting process startup
jitter dominate hot-loop decisions.

Timer semantic risk: Leia `time.since()` and Lua `os.clock()` may not have
identical semantics. The Leia timer appears to be wall-style elapsed time,
while `os.clock()` is CPU time in Lua. Comparing those is acceptable only when
the workload is CPU-bound and not affected by sleep, I/O, or scheduling noise.
Host-heavy official cases such as `stdlib_host_hot` should be treated carefully.

Logical-time risk: `control/defer_protected` reports a deterministic cost
model, so its `Time:` output is not performance time. It should never be mixed
into throughput rankings unless the reported source says logical or the harness
substitutes wall time.

Hot-region risk: official hot scripts do not all measure the same phase. Some
exclude warmup/setup; `strings_patterns_hot` includes setup; logical time
excludes all real elapsed effects. A full report that only says `Time` hides
these boundaries and can make regressions look inconsistent.

Command drift risk: docs and generated reports currently mix `--all`,
`--all-groups`, `--group=official`, `--time-source=auto`, and
`--time-source=script`. Without recording the exact command and effective
arguments, two full timing runs may look comparable while measuring different
groups or different fallback policy.

## Recommended Unified Output Fields

Every benchmark report row should expose these fields, even if some are empty:

| Field | Meaning |
|---|---|
| `benchmark_id` | Stable `group/name`, e.g. `app/stdlib_host`. |
| `subject` | `current`, `head`, `luajit`, or another compared runtime. |
| `mode` | `default`, `vm`, `no_filter`, `luajit`, or Go benchmark mode. |
| `status` | `ok`, `partial`, `low_resolution`, `no_time`, `timeout`, `error`, `skipped`, or `missing`. |
| `seconds_median` | Median per logical benchmark invocation. |
| `time_source` | `script_repeat`, `script`, `wall_repeat`, `wall_hr`, `go_benchmark`, or `logical`. |
| `script_total_seconds` | Summed parsed `Time:` seconds for one sample. |
| `wall_total_seconds` | Summed harness wall seconds for one sample. |
| `repeat` | Command invocations per sample after calibration. |
| `runs` | Measured sample count, excluding calibration and warmup. |
| `warmup_runs` | Harness warmup samples after calibration. |
| `timer_resolution_seconds` | Effective script timer resolution floor. |
| `min_sample_seconds` | Minimum accepted total sample duration. |
| `low_resolution` | Boolean derived from status/source, not inferred from display precision. |
| `includes_startup` | True for `wall_repeat`/`wall_hr` and CLI wall timing. |
| `includes_parse_compile` | True for one-shot CLI script timing; false for Go warm `VM.Call` benchmarks. |
| `timed_region` | `hot_only`, `setup_plus_hot`, `logical_model`, `startup_end_to_end`, or `go_warm_call`. |
| `scale` | Explicit parameter overrides, e.g. `N:420->1200`. |
| `checksum` | Printed checksum or stable output hash. |
| `cv_pct` | Coefficient of variation for the measured subject. |
| `ci95_half_width_pct` | 95% confidence interval half-width when available. |
| `t2_attempted` / `t2_entered` / `t2_failed` | Tier 2 counters for Leia JIT modes. |
| `exit_total` | Total JIT exits for Leia JIT modes. |
| `command` | Exact command used for the report. |
| `platform` | OS, arch, Go version, LuaJIT path/version when available. |
| `commit` / `dirty` | Git identity and dirty-worktree flag. |

## P0 Fixes

Completed: add a repeatable performance gate entrypoint. `scripts/performance_gate.sh`
now wraps `timing_compare.py` and `strict_guard.py`, records JSON/Markdown
artifacts, sorts current/HEAD results, fails low-resolution/unavailable rows,
and applies separate script-time vs wall-time regression thresholds.

1. Make `logical` a first-class timing source. `control/defer_protected`
   should not appear as ordinary `script_repeat`; reports should display
   `time_source=logical` or require wall substitution explicitly.

2. Record `timed_region` for every benchmark row. Start with coarse labels:
   `hot_only`, `setup_plus_hot`, `logical_model`, `startup_end_to_end`, and
   `go_warm_call`.

3. Fix the documented full command drift by either changing the top-level
   command to `--all-groups` or adding a real `--all` alias.

4. For publishable hot-loop comparisons, require `--time-source=script` and
   reject or clearly segregate `wall_repeat` rows. Wall-timed rows can remain
   in raw output but should not drive hot gap ranking.

## P1 Fixes

1. Normalize official hot timer boundaries. Either move setup outside `t0` for
   cases intended as hot-only workloads or mark them as `setup_plus_hot`.

2. Add a checked-in official hot scale profile or per-file default sizes that
   push each script-timed sample above `--min-sample-seconds` without depending
   on wall fallback.

3. Emit exact effective command and arguments into Markdown output, not only
   JSON. This makes report screenshots and copied tables auditable.

4. Add a report warning when Leia and LuaJIT use different timing semantics
   for the same row, especially for host-heavy or logical-time official cases.

## P2 Fixes

1. Unify `strict_guard.py` and `timing_compare.py` field names:
   `source` vs `time_source`, `runs` vs `measured_runs`, and timer-resolution
   keys should converge.

2. Add optional per-sample raw arrays to Markdown collapsible sections or
   artifact JSON so outliers can be inspected without rerunning.

3. Add a small metadata manifest beside official hot cases declaring workload
   parameters, timer boundary, expected checksum, and Lua reference path.

4. Keep Go warm benchmark output in the same schema with
   `time_source=go_benchmark` and `timed_region=go_warm_call`, so users can
   compare it with CLI reports without confusing it for the same measurement.
