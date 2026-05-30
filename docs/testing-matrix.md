# Testing Matrix

This document maps the main correctness and performance gates to their files.

## Correctness

| Area | Files | Command |
|---|---|---|
| Core Go packages | `./...` | `go test ./... -count=1` |
| Language conformance | `tests/language/*.lua` + `*.gs` | `go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v` |
| JIT conformance parity | same conformance pairs | `GSCRIPT_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v` |
| Feature and release metadata | `tests/feature_matrix.json`, release matrix tests | `go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1` |

The conformance gate uses Lua output as the oracle and compares GScript VM
output. The JIT parity mode compares `gscript -jit` output as well.

## Performance

| Domain | Files | LuaJIT refs | Example |
|---|---|---|---|
| `numeric` | `benchmarks/numeric/*.gs` | `benchmarks/lua_ref/numeric/*.lua` | `python3 benchmarks/strict_guard.py --group numeric ...` |
| `recursion` | `benchmarks/recursion/*.gs` | `benchmarks/lua_ref/recursion/*.lua` | `python3 benchmarks/strict_guard.py --group recursion ...` |
| `table` | `benchmarks/table/*.gs` | `benchmarks/lua_ref/table/*.lua` | `python3 benchmarks/strict_guard.py --group table ...` |
| `calls` | `benchmarks/calls/*.gs` | `benchmarks/lua_ref/calls/*.lua` | `python3 benchmarks/strict_guard.py --group calls ...` |
| `string` | `benchmarks/string/*.gs` | `benchmarks/lua_ref/string/*.lua` | `python3 benchmarks/strict_guard.py --group string ...` |
| `concurrency` | `benchmarks/concurrency/*.gs` | `benchmarks/lua_ref/concurrency/*.lua` | `python3 benchmarks/strict_guard.py --group concurrency ...` |
| `data` | `benchmarks/data/*.gs` | `benchmarks/lua_ref/data/*.lua` | `python3 benchmarks/strict_guard.py --group data ...` |
| `app` | `benchmarks/app/*.gs` | `benchmarks/lua_ref/app/*.lua` | `python3 benchmarks/strict_guard.py --group app ...` |
| `control` | `benchmarks/control/*.gs` | `benchmarks/lua_ref/control/*.lua` | `python3 benchmarks/strict_guard.py --group control ...` |

Full timing comparison:

```bash
python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 \
  --sort=luajit-gap \
  --json /tmp/gscript_timing.json \
  --markdown /tmp/gscript_timing.md
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
