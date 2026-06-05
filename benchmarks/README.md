# Leia Benchmarks

Benchmarks are grouped by capability domain:

| Domain | Focus |
|---|---|
| `numeric` | math, matrices, nbody, spectral norm |
| `recursion` | recursive calls and recursive data structures |
| `table` | arrays, maps, sort, traversal, metamethod tables |
| `calls` | closures, varargs, coroutine and method dispatch |
| `string` | strings, patterns, regexp-style workloads |
| `concurrency` | goroutine-like runtime features |
| `data` | data-oriented and SoA kernels |
| `app` | larger application-shaped workloads |
| `control` | control-flow and protected execution |
| `precision` | numeric precision checks and stability probes |

LuaJIT references live under `benchmarks/lua_ref/<domain>/`.

## Main Commands

```bash
# Fast grammar-change hot-path gate; writes reports under /tmp by default.
bash scripts/performance_gate.sh --syntax-smoke --no-luajit

# Current worktree vs clean HEAD vs LuaJIT.
bash scripts/performance_gate.sh --full

# Investigation-only current worktree vs clean HEAD vs LuaJIT.
python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --sort=luajit-gap \
  --json /tmp/leia_timing_compare.json \
  --markdown /tmp/leia_timing_compare.md

# Strict truth pass across hot, script-timed groups.
python3 benchmarks/strict_guard.py --runs=3 --warmup=1 --timeout=90 \
  --json benchmarks/data/strict_guard_latest.json \
  --markdown benchmarks/data/strict_guard_latest.md

# Representative subset while iterating.
python3 benchmarks/strict_guard.py --runs=3 --warmup=0 --max-repeat=8 \
  --bench=numeric/matmul --bench=numeric/matmul_row \
  --bench=table/json_table_walk

# Hot-loop scaling profile for low-resolution workloads.
python3 benchmarks/timing_compare.py --runs=5 --warmup=1 \
  --scale-profile=hot --sort=luajit-gap \
  --json /tmp/leia_hot_timing.json \
  --markdown /tmp/leia_hot_timing.md

# Semantic-family performance coverage audit.
bash benchmarks/coverage_guard.sh
```

Use `--syntax-smoke` after lexer/parser/grammar-only work when you need a
quick current-vs-HEAD check over control, calls, table, string, and data hot
paths plus the Leia-only dialect truth pass. Add `--no-strict` only when the
change does not need VM/default/no-filter output stability evidence.

## Diagnostics

```bash
python3 benchmarks/profile_exits.py --bench=numeric/spectral_norm --top=30

python3 benchmarks/triage.py --bench=numeric/spectral_norm \
  --scale=numeric/spectral_norm:N=2000 --time-source=script \
  --diag --pprof --memprofile --warm-dump --out-dir=/tmp/leia-triage

python3 benchmarks/diagnose.py \
  --bench=calls/method_dispatch \
  --bench=table/groupby_nested_agg \
  --out-dir=/tmp/leia-diagnose

bash scripts/diag.sh table/table_array_access
```

`timing_compare.py` is the primary local optimization harness. It records timing
source, repeat count, CI, parameter scaling, current-vs-HEAD deltas, and LuaJIT
gaps.

`strict_guard.py` is the release/regression truth pass for hot, script-timed
workloads. It runs VM, default JIT, no-filter JIT, and LuaJIT where a reference
exists, then checks output stability and timing quality. Concurrency benchmarks
remain available via explicit `--group=concurrency` or `--bench=...`, but they
are not part of the default strict pass because many are semantic or blocking
wall-time workloads rather than comparable hot loops.

Benchmark selectors are domain IDs such as `numeric/matmul` or
`table/json_table_walk`. Historical selector families are intentionally not
accepted by the harness.
