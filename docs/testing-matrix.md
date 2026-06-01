# Testing Matrix

This document maps the main correctness and performance gates to their files.

## Correctness

| Area | Files | Command |
|---|---|---|
| Core Go packages | `./...` | `go test ./... -count=1` |
| Language conformance | `tests/language/*.lua` + `*.leia` | `go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v` |
| JIT conformance parity | same conformance pairs | `LEIA_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v` |
| Feature and release metadata | `tests/feature_matrix.json`, release matrix tests | `go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1` |

The conformance gate uses Lua output as the oracle and compares Leia VM
output. The JIT parity mode compares `leia -jit` output as well.

## Performance

| Domain | Files | LuaJIT refs | Example |
|---|---|---|---|
| `numeric` | `benchmarks/numeric/*.leia` | `benchmarks/lua_ref/numeric/*.lua` | `python3 benchmarks/strict_guard.py --group numeric ...` |
| `recursion` | `benchmarks/recursion/*.leia` | `benchmarks/lua_ref/recursion/*.lua` | `python3 benchmarks/strict_guard.py --group recursion ...` |
| `table` | `benchmarks/table/*.leia` | `benchmarks/lua_ref/table/*.lua` | `python3 benchmarks/strict_guard.py --group table ...` |
| `calls` | `benchmarks/calls/*.leia` | `benchmarks/lua_ref/calls/*.lua` | `python3 benchmarks/strict_guard.py --group calls ...` |
| `string` | `benchmarks/string/*.leia` | `benchmarks/lua_ref/string/*.lua` | `python3 benchmarks/strict_guard.py --group string ...` |
| `concurrency` | `benchmarks/concurrency/*.leia` | `benchmarks/lua_ref/concurrency/*.lua` | `python3 benchmarks/strict_guard.py --group concurrency ...` |
| `data` | `benchmarks/data/*.leia` | `benchmarks/lua_ref/data/*.lua` | `python3 benchmarks/strict_guard.py --group data ...` |
| `app` | `benchmarks/app/*.leia` | `benchmarks/lua_ref/app/*.lua` | `python3 benchmarks/strict_guard.py --group app ...` |
| `control` | `benchmarks/control/*.leia` | `benchmarks/lua_ref/control/*.lua` | `python3 benchmarks/strict_guard.py --group control ...` |

Full timing comparison:

```bash
python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 \
  --sort=luajit-gap \
  --json /tmp/leia_timing.json \
  --markdown /tmp/leia_timing.md
```

Strict truth pass:

```bash
python3 benchmarks/strict_guard.py --runs=3 --warmup=1 --timeout=90
```

Coverage audit:

```bash
bash benchmarks/coverage_guard.sh
```

`benchmarks/conformance_perf_coverage.py` maps conformance families to hot-loop
benchmarks. It does not time tiny conformance programs directly because startup
noise would dominate those results.
