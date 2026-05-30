# Testing Matrix

This document maps the correctness and performance suites to their files and
runner commands.

## Correctness

| Area | Files | Runner |
|---|---|---|
| Core VM/JIT/runtime unit tests | `internal/vm`, `internal/runtime`, `internal/jit`, `internal/methodjit`, `tests` | `go test ./internal/vm ./internal/runtime ./internal/jit ./internal/methodjit ./tests -count=1` |
| Hand-written language smoke tests | `tests/01_basic.gs` through `tests/12_advanced.gs` | Covered by `go test ./tests -count=1` |
| Official Lua translated semantics | `tests/official_lua_cases/*.lua` and `tests/official_lua_cases/*.gs` | `go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v` |
| Official JIT parity | same official case pairs | `GSCRIPT_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v` |
| Gated real provider LLM smoke | `gscript/llm_integration_test.go`, `docs/embedding.md` | `GSCRIPT_LLM_INTEGRATION=1 GSCRIPT_ANTHROPIC_COMPAT_BASE_URL=... GSCRIPT_ANTHROPIC_COMPAT_API_KEY=... GSCRIPT_ANTHROPIC_COMPAT_MODEL=... go test ./gscript -run 'Test(AnthropicCompatibleLLMIntegration|AINativeSyntaxAnthropicCompatibleLLMIntegration)' -count=1 -v` |

Official translated cases are semantic parity tests. They compare Lua output to
GScript output and are not used as performance timings.

The real provider LLM smoke tests are opt-in only. They skip under the default
`go test` and `gscript ci` gates unless `GSCRIPT_LLM_INTEGRATION` and all
provider endpoint/model/key variables are set. Do not commit provider tokens;
use local environment variables or CI secret storage.

## Performance

| Group | GScript files | LuaJIT refs | Purpose | Runner |
|---|---|---|---|---|
| `suite` | `benchmarks/suite/*.gs` | `benchmarks/lua/*.lua` | Stable core LuaJIT comparison set | `python3 benchmarks/strict_guard.py --group suite ...` |
| `extended` | `benchmarks/extended/*.gs` and `benchmarks/extended/manifest.json` | `benchmarks/lua_extended/*.lua` | Broader workload coverage | `python3 benchmarks/strict_guard.py --group extended ...` |
| `variants` | `benchmarks/variants/*.gs` | `benchmarks/lua_variants/*.lua` | Overfit and structural variant pressure | `python3 benchmarks/strict_guard.py --group variants ...` |
| `official` | `benchmarks/official_hot/*.gs` | `benchmarks/lua_official_hot/*.lua` | Hot loops extracted from official semantic families | `python3 benchmarks/strict_guard.py --group official ...` |

Default strict guard intentionally runs only `suite`, `extended`, and
`variants`:

```bash
python3 benchmarks/strict_guard.py \
  --mode default --mode luajit \
  --group suite --group extended --group variants \
  --runs 7 --warmup 3 --min-sample-seconds 0.10 \
  --timeout 180 \
  --json /tmp/gscript_hot_luajit_current.json \
  --markdown /tmp/gscript_hot_luajit_current.md
```

Run the official hot group separately while these newly extracted gaps are being
optimized:

```bash
python3 benchmarks/strict_guard.py \
  --group official \
  --mode default --mode luajit \
  --runs 3 --warmup 1 --min-sample-seconds 0.05 \
  --timeout 240 \
  --json /tmp/gscript_official_hot.json \
  --markdown /tmp/gscript_official_hot.md
```

For a true all-performance sweep, include all four groups explicitly:

```bash
python3 benchmarks/strict_guard.py \
  --mode default --mode luajit \
  --group suite --group extended --group variants --group official \
  --runs 3 --warmup 1 --min-sample-seconds 0.05 \
  --timeout 240 \
  --json /tmp/gscript_all_perf.json \
  --markdown /tmp/gscript_all_perf.md
```

Use script `Time:` output for throughput claims. Do not use process wall time
for short semantic cases; it mostly measures startup and compilation noise.

## Coverage Audit

`benchmarks/official_perf_coverage.py` maps the translated official semantic
families to hot performance coverage:

```bash
python3 benchmarks/official_perf_coverage.py \
  --check \
  --markdown /tmp/official_perf_coverage.md \
  --json /tmp/official_perf_coverage.json
```

Current interpretation:

- `covered`: a semantic family has a corresponding hot benchmark.
- `partial`: a family has some coverage but still needs a focused hot case.
- `semantic_only`: the family is mostly cold semantics, host integration, IO,
  diagnostics, or other behavior that should remain a correctness test unless
  a real hot workload appears.

For a guard-style invocation that fails on unclassified official families or
broken benchmark references:

```bash
bash benchmarks/coverage_guard.sh
```
