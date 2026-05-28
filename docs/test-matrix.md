# Test Matrix

本文梳理 correctness、official Lua 翻译测试、stdlib 测试、benchmark correctness/oracle 的入口与覆盖关系。当前仓库没有 `.github` CI 配置目录；下面的 CI gate 是建议分层，而不是现有流水线事实。

## 入口总览

| 目录或文件 | 角色 | 主要命令 | 覆盖关系 |
|---|---|---|---|
| `internal/*/*_test.go` | lexer/parser/runtime/vm/JIT/methodjit/nanbox 单元与集成正确性 | `go test ./internal/... -count=1 -p 1 -timeout=600s` | 覆盖实现内部契约、Tier 1/Tier 2 pipeline、emit/deopt/oracle、stdlib Go 层行为。 |
| `gscript/*_test.go` | public Go API smoke | `go test ./gscript -count=1` | 覆盖 embeddable API 与 runtime 入口，不替代 CLI 文件模式测试。 |
| `tests/*.gs` + `tests/*_test.go` | 手写语言 smoke、feature matrix、release matrix、CLI/JIT 行为 | `go test ./tests -count=1 -p 1 -timeout=600s` | 覆盖 parser/runtime/VM/JIT 的端到端语言行为；`tests/feature_matrix.json` 记录 feature 到 semantic/perf 覆盖的映射，`tests/release_matrix_test.go` 把 spec、official cases、known-gap docs 和 stdlib contract 接成 release gate。 |
| `tests/official_lua_cases/*.lua` + `*.gs` | official Lua 5.4 语义翻译对照 | `go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v` | 用 `lua` 运行 `.lua` 参考，再用 `gscript -vm` 运行 `.gs`，比较标准输出。需要 `lua` 或 `LUA_BIN`，缺失时跳过。 |
| `tests/official_lua_cases/MANIFEST.md` / `tests/official_lua_cases/KNOWN_FAILURES.md` / `tests/official_lua_cases/MISSING_CAPABILITIES.md` | official case 状态、分类和 known-gap ledger | `go test ./tests -run TestReleaseMatrix -count=1` | 每个 paired official case 必须有 passing manifest row 或 skipped known-failure entry；known-gap 文档必须作为 release gate 输入出现在本矩阵。 |
| 同上，加 `GSCRIPT_OFFICIAL_CHECK_JIT=1` | official 翻译的 JIT parity | `GSCRIPT_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v` | 在 VM parity 之外再用 `gscript -jit` 比较输出；这是 JIT 输出一致性 gate，不要求每个 case 都 native 编译。 |
| `internal/runtime/stdlib*_test.go` | stdlib Go 层单元测试 | `go test ./internal/runtime -run 'Stdlib|Standard|String|Table|Utf8|Bit|Json|Fs|Io|Net|Process' -count=1` 或直接 `go test ./internal/runtime -count=1` | 覆盖内建/host stdlib 的参数、边界、错误和 Go helper 行为。 |
| `tests/official_lua_cases/*` 中 stdlib 前缀 case | stdlib 文件模式/官方翻译语义 | `go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v` | 覆盖 string/table/utf8/bit32/math/io/fs/net/process/json/hash/url/uuid 等模块在脚本入口下的输出一致性。 |
| `docs/stdlib-contract.md` | stdlib contract index | `go test ./tests -run TestReleaseMatrixStdlibContractHasOfficialCoverageEntry -count=1` | 每个 contract module 必须能在 official translated case、feature matrix 或 capability ledger 中找到覆盖入口。 |
| `benchmarks/*_test.go` | benchmark package correctness 和 Go benchmark smoke | `go test ./benchmarks -count=1`；微基准用 `go test ./benchmarks -bench=Warm -benchtime=3s` | `verify_test.go` 验证多 runtime fib 结果，`inline_correctness_test.go` 验证 JIT inline 调用正确性；`Benchmark*` 不是默认 correctness gate。 |
| `benchmarks/suite/*.gs` + `benchmarks/lua/*.lua` | 核心 hot benchmark 与 LuaJIT 参考 | `python3 benchmarks/strict_guard.py --group suite --runs 3 --warmup 1 --timeout 90` | strict guard 对 VM/default/no_filter/LuaJIT 采样，检查输出 hash/checksum、Tier 2 stats 和 suspicious wins。 |
| `benchmarks/extended/*.gs` + `benchmarks/lua_extended/*.lua` | 扩展 workload correctness/perf | `python3 benchmarks/strict_guard.py --group extended --runs 3 --warmup 1 --timeout 90` | 覆盖更接近业务形态的 JSON walk、pipeline、aggregation、format 等 workload。 |
| `benchmarks/variants/*.gs` + `benchmarks/lua_variants/*.lua` | anti-overfit 变体 | `python3 benchmarks/strict_guard.py --group variants --runs 3 --warmup 1 --timeout 90` | 验证 suite win 是否能经受结构变体压力；用于防止 benchmark-specific 优化误判。 |
| `benchmarks/official_hot/*.gs` + `benchmarks/lua_official_hot/*.lua` | official semantic family 的 hot-loop 性能代表 | `python3 benchmarks/strict_guard.py --group official --runs 3 --warmup 1 --timeout 240` | 不替代 official correctness；用于把 official case family 映射到可计时 hot path。 |
| `benchmarks/official_perf_coverage.py` / `coverage_guard.sh` | official correctness 到 hot benchmark 的覆盖审计 | `bash benchmarks/coverage_guard.sh` | 检查 official family 是否 classified 为 `covered`/`partial`/`semantic_only`，以及 benchmark 引用是否存在。 |
| `benchmarks/timing_compare.py` | 当前 worktree vs clean HEAD vs LuaJIT timing oracle | `python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 --time-source=auto --sort=luajit-gap` | 性能决策入口；记录 timing source、repeat、CI、current/HEAD/LuaJIT delta，不应作为短 semantic case 的 correctness oracle。 |

## 覆盖关系

Correctness 的主闭环是 `go test ./... -count=1 -p 1 -timeout=600s`。它覆盖 internal 单元测试、public API、`tests` 语言 smoke、official Lua 翻译测试、benchmark package 的 correctness tests。注意 official Lua 翻译测试依赖本机 `lua`；如果 CI 镜像没有 Lua，它会被 skip，因此不能只看 `go test ./...` 通过就认为 official gate 已执行。

official Lua 翻译测试是 semantic parity：每个 `.lua` 必须有同名 `.gs`，Lua 输出是 oracle，GScript VM 输出必须一致。`GSCRIPT_OFFICIAL_CHECK_JIT=1` 扩展同一批 case 到 `-jit` 输出 parity，但它仍然是输出一致性测试，不是 Tier 2 native coverage 测试；`tests/official_lua_cases/MISSING_CAPABILITIES.md` 明确记录了 semantic-check 模式下允许 VM/fallback 的边界。

stdlib 有三层覆盖。第一层是 `internal/runtime/stdlib*_test.go`，直接打 Go runtime/module API 和错误边界。第二层是 official translated case 中的 stdlib families，覆盖文件模式、`require`、host stdlib 与 Lua/GScript 兼容入口。第三层是 `benchmarks/official_hot/stdlib_host_hot.gs`、`math_bit_utf8_hot.gs`、`regexp_random_hot.gs` 等 hot-loop 代表，只用于性能面，不应用来证明冷路径语义完整。

benchmark correctness/oracle 分两类。`go test ./benchmarks -count=1` 是轻量 correctness smoke，确保 benchmark package 里的 JIT inline/verify 等逻辑仍对。`strict_guard.py` 是 hot benchmark truth pass：构建一次 `cmd/gscript`，发现 suite/extended/variants/official 组，按 VM/default/no_filter/LuaJIT 运行，检查输出 hash/checksum 并记录 timing。`timing_compare.py` 是优化决策 oracle，比较当前 worktree、clean HEAD、LuaJIT；它回答“变快/变慢是否可信”，不回答“语言语义是否完整”。

## CI Gate 建议

| Gate | 必跑命令 | 触发 | 失败条件 |
|---|---|---|---|
| P0 correctness | `go test ./... -count=1 -p 1 -timeout=600s` | 每个 PR/提交 | 任意 Go 测试失败；但要单独确认 official 是否因缺 Lua 被 skip。 |
| P0 official VM parity | 安装 Lua 后运行 `go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v` | 每个 PR/提交 | official `.lua`/`.gs` 配对缺失、Lua/GScript VM 输出不一致、case timeout。 |
| P0 official JIT parity | `GSCRIPT_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v` | JIT/runtime/parser/stdlib 相关改动 | `gscript -jit` 输出与 Lua oracle 不一致。 |
| P0 coverage metadata | `go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1` | 每个 PR/提交 | `tests/feature_matrix.json` schema、status、repo-relative ref 失效；stable spec section 没有 semantic gate；official case 缺 manifest/known-failure 状态；known-gap 或 stdlib contract 缺 release-gate 入口。 |
| P1 benchmark coverage audit | `bash benchmarks/coverage_guard.sh` | official case 或 benchmark 改动 | official family 未分类或 coverage 引用不存在。 |
| P1 strict representative | `python3 benchmarks/strict_guard.py --runs 3 --warmup 1 --timeout 90 --bench=suite/matmul --bench=variants/matmul_row_variant --bench=extended/json_table_walk` | JIT/perf 相关 PR | checksum/hash mismatch、timeout/crash、明显 suspicious win 未解释。 |
| P1 strict full default | `python3 benchmarks/strict_guard.py --runs 3 --warmup 1 --timeout 90` | merge 前或 nightly | suite+extended+variants 默认组退化、输出不一致、timeout。 |
| P2 all-groups timing | `python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 --time-source=auto --sort=luajit-gap` | nightly/release/perf claim | current vs HEAD 明显 regression，或短样本 fallback 未标注就被用于结论。 |

## 已知缺口

- CI 配置缺失：仓库当前没有 `.github` 目录，无法确认真实 CI 是否安装 Lua/LuaJIT 或执行 benchmark guards。
- official VM parity 可能被静默 skip：`lua` 找不到时 `TestOfficialLuaTranslatedCases` 会 `Skip`，需要 CI 显式安装 Lua 或设置 `LUA_BIN`。
- coverage metadata 只检查入口关系：`TestReleaseMatrix*` 会阻止 unmapped spec、unclassified official case、orphaned known-gap docs 和缺 official/capability 入口的 stdlib contract module，但不替代实际 semantic parity 执行。
- LuaJIT 是 optional：`strict_guard.py` 找不到 `luajit` 时相关 cell 不可用；性能 gate 需要显式准备 LuaJIT，否则只能做 GScript 内部模式比较。
- official JIT parity 不等于 native Tier 2 coverage：`GSCRIPT_OFFICIAL_CHECK_JIT=1` 只比较输出，不能证明每个官方 case 已进入 native code。
- stdlib 目录不是源码目录：没有顶层 `stdlib/` 测试目录，stdlib 测试主要在 `internal/runtime/stdlib*_test.go`、docs 和 official translated cases 中。
- benchmark correctness 与 benchmark timing 混在同一 package：`go test ./benchmarks` 覆盖 correctness smoke，但不会运行 `Benchmark*` timing；性能结论必须走 `strict_guard.py` 或 `timing_compare.py`。
- official hot 默认不在 `strict_guard.py` default groups：default groups 是 `suite, extended, variants`，`official` 需要 `--group official` 或 `timing_compare.py --all-groups`。

## P0/P1/P2 落地步骤

P0:

1. 在 CI 镜像中安装并固定 `lua`，设置 `LUA_BIN`，避免 official translated cases 被 skip。
2. 每次提交执行 `go test ./... -count=1 -p 1 -timeout=600s`。
3. 每次提交显式执行 `go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1`，确保 release matrix 元数据没有漂移。
4. 每次提交显式执行 `go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v`，并把 skip 视为配置失败。
5. JIT/runtime/parser/stdlib 改动追加 `GSCRIPT_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v`。

P1:

1. 把 `bash benchmarks/coverage_guard.sh` 接入 PR gate，覆盖 official family 到 hot benchmark 的引用完整性。
2. 为 JIT/perf 相关 PR 跑 representative strict subset，至少包含一个 suite、一个 variant、一个 extended workload。
3. merge 前或 nightly 跑 `python3 benchmarks/strict_guard.py --runs 3 --warmup 1 --timeout 90`，保留 JSON/Markdown artifact。
4. official case family 新增后，同步更新 `benchmarks/official_perf_coverage.py` 分类；确实有 hot path 时新增 `benchmarks/official_hot` 和 `benchmarks/lua_official_hot` 配对。

P2:

1. 安装并固定 LuaJIT，nightly 跑 `timing_compare.py --all-groups`，记录 current/HEAD/LuaJIT 趋势。
2. 对 sub-50ms 或 timer-resolution 边缘 benchmark，先提升 checked-in workload 或使用明确 scale profile，再用于性能结论。
3. 将 `strict_guard.py` full default、official hot group、`timing_compare.py --all-groups` 的 JSON/Markdown 归档为 release evidence。
4. 扩展 feature matrix，使新增语言/stdlib/JIT 能力必须同时声明 parser、bytecode、interpreter、tier1、tier2、semantic gate、official case、perf hot case 的覆盖状态。
