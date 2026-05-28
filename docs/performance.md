# Performance and JIT Roadmap

This document defines the production guardrails for performance work in
GScript. Performance wins are only accepted when they preserve semantics,
generalize beyond the benchmark that exposed the gap, and remain visible in the
benchmark history.

## Principles

- Correctness is the first performance metric. A faster JIT path that changes a
  checksum, skips an observable side effect, changes an error, or depends on a
  benchmark-only shape is a bug.
- Benchmarks are witnesses, not targets. Use them to expose runtime behavior,
  validate a hypothesis, and prevent regression; do not tune code to recognize
  benchmark names, fixed constants, or one checked-in loop body.
- LuaJIT is the external throughput reference where a comparable Lua program
  exists. It is not the semantic oracle for GScript-only language behavior.
- Runtime specialization must be guarded by recorded feedback, dependency
  checks, and deoptimization paths. A specialization is production-ready only
  when invalidation and fallback are tested.
- CI gates must fail loudly on correctness drift and sustained regressions, but
  timing noise must be handled with medians, calibrated repeats, and reviewed
  baselines.

## Correctness Oracle

Every optimization must run against a correctness oracle before its timing is
trusted:

1. Compare VM, default JIT, and no-filter JIT output for the same GScript
   benchmark. The VM is the primary semantic oracle for GScript.
2. Compare explicit `checksum:` output, when present. If a benchmark lacks a
   stable checksum, add one before using it for performance decisions.
3. Compare the full output hash for programs whose visible result is more than
   one checksum line.
4. Compare LuaJIT output only for benchmark pairs with matching Lua references.
   Treat mismatches as either a benchmark-porting bug or a documented language
   difference; do not hide them in the harness.
5. Run structural variants for any large win. A core-suite win that is not
   confirmed by a variant is a review signal, not a release headline.

Preferred truth pass:

```bash
python3 benchmarks/strict_guard.py --runs=3 --warmup=1 --timeout=90 \
  --json benchmarks/data/strict_guard_latest.json \
  --markdown benchmarks/data/strict_guard_latest.md
```

`strict_guard.py` exercises suite, extended, and variant benchmarks across VM,
default JIT, no-filter JIT, and LuaJIT when available. Its "Suspicious Kernel
Wins" section must be reviewed before claiming an optimization as general.

## Benchmark Harness

Use the benchmark tools for different jobs:

| Tool | Production role |
|---|---|
| `benchmarks/timing_compare.py` | Primary local timing harness: current checkout vs clean `HEAD` vs LuaJIT, calibrated repeats, confidence intervals, and gap ranking. |
| `benchmarks/strict_guard.py` | Correctness and generalization oracle for release/regression gates. |
| `benchmarks/regression_guard.sh` | Baseline regression gate for default JIT performance. |
| `benchmarks/diagnose.py` | Artifact bundle for one or more benchmarks: timing, exits, runtime paths, Tier 2 perf, speculation state, worklist, and optional pprof/warm dumps. |
| `benchmarks/profile_exits.py` | Exit/deopt attribution; use it to explain timing, not as the success metric. |
| `benchmarks/official_perf_coverage.py` | Coverage audit from official semantic cases to hot performance cases. |

Default local timing command:

```bash
python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --sort=luajit-gap \
  --json /tmp/gscript_timing_compare.json \
  --markdown /tmp/gscript_timing_compare.md
```

Rules for benchmark quality:

- Keep suite, extended, variants, and official-hot as separate report groups.
  Do not collapse them into a single score.
- Checked-in throughput benchmarks should be hot enough on their own. Treat
  sub-50ms default runs as smoke coverage unless the benchmark is specifically
  about startup or compile latency.
- Use `--scale`, `--param`, or `--scale-profile=hot` to investigate a hypothesis,
  but record changed parameters and do not publish scaled numbers as if they
  were the checked-in workload.
- Publish medians, repeat counts, timing source, CV or CI width, platform, Go
  version, commit, and LuaJIT availability with every performance report.

## LuaJIT Comparison

LuaJIT comparison has two jobs:

- It ranks remaining gaps on workloads where a faithful Lua reference exists.
- It detects suspicious wins when GScript appears much faster on a narrow kernel
  but related variants do not show the same advantage.

Interpret `Current/LuaJIT` and `JIT/LJ` as ratios, not absolute quality labels.
Lower is faster relative to LuaJIT. A strong GScript result still needs matching
checksums, stable timing, and variant coverage.

LuaJIT must be optional in developer loops but present in publish-grade reports.
If `luajit` is missing from `PATH`, the report must say so explicitly and the
release checklist must mark LuaJIT comparison as incomplete.

## No Benchmark-Specific Optimization

The JIT must not contain benchmark-specific recognition. The following are not
acceptable:

- matching file names, benchmark names, printed labels, or fixed problem sizes;
- specializing on constants that are only profitable because a benchmark uses
  one exact input;
- bypassing language semantics for known benchmark call graphs;
- adding a fast path that works only because a benchmark omits a mutation,
  metamethod, nil case, coroutine interaction, or string/table corner case.

Acceptable specialization is based on runtime feedback and guarded facts:

- observed operand, field, table-shape, global, callsite, or range facts;
- dependency records with invalidation checks;
- deopt exits that resume through a semantically equivalent path;
- structural variants that perturb the benchmark without defeating the real
  workload shape.

Reviewer checklist for any large win:

1. What runtime fact made the optimization legal?
2. Where is the guard or dependency checked?
3. What invalidates the compiled artifact?
4. Which test proves fallback correctness?
5. Which variant proves the win is not benchmark-specific?

## Runtime Specialization

Runtime specialization is the main path for production JIT gains. It should be
implemented as a controlled loop:

1. Observe stable facts in Tier 1 or interpreter execution.
2. Admit Tier 2 only after feedback readiness is sufficient.
3. Build optimized IR with explicit facts in the pass context.
4. Emit guards, dependency checks, and exit metadata.
5. Record Tier 2 attempted/entered/failed counts and exit kinds.
6. Diagnose regressions with runtime-path stats, warm dumps, and JIT PC maps.

Production-ready specialization must have:

- correctness tests for hit, miss, invalidation, and deopt resume;
- benchmark coverage in at least one stable suite or extended workload;
- a structural variant when the optimization is exposed by a small kernel;
- diagnostic visibility in `diagnose.py`, warm dumps, or exit stats;
- a clear fallback path when architecture, feedback, or dependency support is
  unavailable.

## CI Performance Gate

The CI gate should be staged so common changes remain fast while release
signals stay reliable:

| Stage | Trigger | Command | Gate |
|---|---|---|---|
| Correctness smoke | every PR | `go test ./... -count=1 -p 1 -timeout=600s` | no test failures |
| Benchmark coverage | every PR touching runtime/JIT/benchmarks | `bash benchmarks/coverage_guard.sh` | official hot coverage remains mapped |
| Regression guard | performance-sensitive PRs and nightly | `bash benchmarks/regression_guard.sh --runs=3 --threshold=10` | no default-JIT row more than 10% slower than baseline |
| Strict truth pass | nightly and release candidates | `python3 benchmarks/strict_guard.py --runs=3 --warmup=1 --timeout=90` | no checksum drift; suspicious wins reviewed |
| Publish-grade run | release candidate | `bash benchmarks/regression_guard.sh --runs=5 --timeout=90` plus full `timing_compare.py` | report archived and baseline decision made |

Baseline updates are release engineering actions, not routine PR cleanup:

```bash
bash benchmarks/regression_guard.sh --runs=5 --timeout=90 \
  --json benchmarks/data/regression_guard_latest.json
bash benchmarks/set_baseline.sh benchmarks/data/regression_guard_latest.json
```

Only update `benchmarks/data/baseline.json` after a clean publish-grade run on a
known machine and after the performance change has been reviewed.

## Reporting Requirements

A performance report attached to a release or major JIT change must include:

- commit SHA and whether the tree was clean;
- OS, architecture, CPU model, Go version, and LuaJIT version;
- exact benchmark commands;
- JSON and Markdown artifacts from `timing_compare.py`, `strict_guard.py`, and
  the publish-grade regression guard;
- top regressions, top LuaJIT gaps, and top suspicious wins;
- checksum status and any skipped LuaJIT references;
- decision on whether the checked-in baseline was updated.
