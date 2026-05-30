# Test Matrix

本文梳理 GScript 当前的 correctness 与 performance gate。测试目录按语言能力组织，不再按历史 Lua official / suite / variant 分组。

## Correctness

| 范围 | 文件 | 命令 | 说明 |
|---|---|---|---|
| 全量 Go 测试 | `./...` | `go test ./... -count=1` | 覆盖 parser/runtime/VM/JIT/CLI/stdlib。 |
| 语言一致性 | `tests/language/*.lua` + `*.gs` | `go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v` | 用 Lua 输出作为 oracle，对比 GScript VM 输出。 |
| JIT 输出一致性 | 同上 | `GSCRIPT_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v` | 在 VM parity 外增加 `gscript -jit` 输出对比。 |
| Feature/release 元数据 | `tests/feature_matrix.json` | `go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1` | 检查 spec、semantic gate、conformance case、perf hot case 的覆盖关系。 |

Release-gate ledger inputs: `tests/language/MISSING_CAPABILITIES.md`,
`tests/language/KNOWN_FAILURES.md`, `tests/language/MANIFEST.md`, and
`docs/stdlib-contract.md`.

## Performance

| 范围 | 文件 | 命令 | 说明 |
|---|---|---|---|
| domain strict guard | `benchmarks/{numeric,recursion,table,calls,string,concurrency,data,app,control}/*.gs` | `python3 benchmarks/strict_guard.py --runs 3 --warmup 1 --timeout 90` | VM/default/no_filter/LuaJIT 采样，检查输出 hash、checksum、Tier 2 stats 和 timing quality。 |
| current vs HEAD vs LuaJIT | 同上 | `python3 benchmarks/timing_compare.py --all-groups --runs 5 --warmup 1 --sort luajit-gap` | 优化决策入口，判断当前改动相对 HEAD 和 LuaJIT 的差距。 |
| semantic-family perf audit | `benchmarks/conformance_perf_coverage.py` | `bash benchmarks/coverage_guard.sh` | 检查 conformance family 是否映射到合适的 hot benchmark。 |
| diagnostic bundle | `benchmarks/diagnose.py`, `benchmarks/triage.py`, `scripts/diag.sh` | 按目标 benchmark 运行 | 收集 exits、runtime path、Tier 2 IR/ASM、pprof、warm dump 等证据。 |

LuaJIT 参考文件放在 `benchmarks/lua_ref/<domain>/`。没有 LuaJIT 对照的 GScript benchmark 仍可用于 VM/JIT 回归和 host/data/concurrency 能力覆盖。

## CI 建议

| 层级 | 命令 | 频率 |
|---|---|---|
| P0 correctness | `go test ./... -count=1` | 每个提交 |
| P0 conformance | `go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v` | 每个提交；CI 需安装 Lua 或设置 `LUA_BIN` |
| P0 metadata | `go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1` | 每个提交 |
| P1 benchmark audit | `bash benchmarks/coverage_guard.sh` | benchmark 或 conformance case 改动 |
| P1 representative perf | `python3 benchmarks/strict_guard.py --bench numeric/spectral_norm --bench table/table_array_access --bench calls/method_dispatch --runs 3 --warmup 1` | JIT/runtime 改动 |
| P2 full perf | `python3 benchmarks/timing_compare.py --all-groups --runs 5 --warmup 1 --sort luajit-gap` | release/nightly |
