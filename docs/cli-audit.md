# GScript CLI and Toolchain Audit

本文审计当前 CLI/工具链形态，并给出把 GScript 作为独立语言发布前需要收敛的命令集合与落地顺序。范围限于用户可见命令、开发者工具入口、benchmark/test 入口，以及这些入口和 Go embedding API 的边界。

## Current Command Shape

### `cmd/gscript`

`cmd/gscript/main.go` 是当前主入口，采用 Go `flag` 包的一层平铺 flags：

- `gscript -e SOURCE [args...]` 执行字符串。
- `gscript` 无参数进入最小 REPL。
- `gscript FILE [args...]` 执行文件。
- `-vm` 使用 bytecode VM 并关闭 JIT，前提是用户没有显式设置 `-jit`。
- `-jit` 默认启用，并隐式走 bytecode VM。
- `-cpuprofile`、`-memprofile` 输出 Go pprof。
- JIT/运行时诊断 flags 包括 `-jit-stats`、`-jit-timeline`、`-jit-timeline-format`、`-jit-dump-warm`、`-jit-dump-proto`、`-exit-stats`、`-exit-stats-json`、`-tier2-perf-stats`、`-tier2-perf-stats-json`、`-tier2-spec-state-json`、`-tier2-spec-worklist-json`、`-jit-op-audit`、`-jit-op-audit-json`、`-coroutine-stats`、`-runtime-path-stats`、`-runtime-path-stats-json`。

这个入口现在更像“运行器 + JIT 实验室控制台”。它适合维护者调性能，但还不是独立语言发行版的稳定命令面。

### `cmd/dump` and `cmd/dump_bytecode`

`cmd/dump` 接受 `FILE PROTO_NAME`，编译后查找指定 child proto 并打印反汇编。它没有参数校验、help、统一错误输出或稳定 JSON 输出。

`cmd/dump_bytecode` 接受 `FILE`，递归打印 `<main>` 和所有 child proto 的 bytecode，并附带 params、maxstack、vararg、JIT disabled、Tier 1/Tier 2 callable decision。它是有价值的 inspect 能力，但当前作为独立开发者二进制存在，不在 `gscript` 用户命令树里。

### Benchmarks

`benchmarks/` 是当前最成熟的工具链区域，主要入口包括：

- `python3 benchmarks/timing_compare.py`：当前 worktree、clean `HEAD` 和 LuaJIT 的本地 timing 对比，支持 suite/extended/variants/official 分组、calibrated repeats、CI、scale/param、JSON/Markdown 输出。
- `python3 benchmarks/strict_guard.py` 或 `bash benchmarks/strict_guard.sh`：suite + extended + variants 的严格 truth pass，覆盖 VM/default JIT/no-filter/LuaJIT、checksum、JIT stats、exit stats 和 suspicious-win review。
- `python3 benchmarks/diagnose.py`：按 benchmark 收集 timing、exit、runtime path、Tier 2 perf、spec state/worklist、可选 pprof/warm dump。
- `python3 benchmarks/triage.py`：面向单个或少量 benchmark 的 timing + exit + 可选 diag/pprof/warm-dump bundle。
- `python3 benchmarks/profile_exits.py`：Tier 2 exit/deopt profile。
- `python3 benchmarks/jit_addr_map.py`：把 warm JIT PC map 和 pprof/raw PC 离线关联。
- `python3 benchmarks/official_perf_coverage.py` 或 `bash benchmarks/coverage_guard.sh`：审计 translated official Lua 语义 case 与 hot benchmark 的覆盖关系。
- `python3 benchmarks/regression_guard.py` 或 `bash benchmarks/regression_guard.sh`：旧的 baseline regression workflow，仍保留兼容价值。
- `bash benchmarks/set_baseline.sh`、`plot_history.sh`、`diagnose_tier2.sh`、`benchmarks/extended/run_all.sh`、`benchmarks/precision/run.sh` 等辅助入口。

这些工具的能力强，但入口分散在 Python、shell、Go CLI flags 和环境变量之间。输出 schema 也按工具演化，没有统一的 command envelope、artifact manifest 或 exit-code 规范。

### Tests

当前测试入口主要是 Go 测试：

- 顶层推荐入口：`go test ./... -count=1 -p 1 -timeout=600s`。
- `tests/integration_test.go` 直接通过 lexer/parser/interpreter 执行 `tests/01_basic.gs` 到 `tests/12_advanced.gs`。
- `tests/official_lua_semantics_test.go` 构建 `./cmd/gscript`，用 `lua` 或 `LUA_BIN` 作为 oracle，对 `tests/official_lua_cases/*.lua` 与对应 `.gs` 比较输出；设置 `GSCRIPT_OFFICIAL_CHECK_JIT=1` 时额外比较 `gscript -jit`。
- `tests/feature_matrix_test.go` 校验 `tests/feature_matrix.json` schema 和 repo-relative refs。
- `tests/jit_*.go`、`tests/trace_exec_test.go`、`benchmarks/*_test.go` 覆盖 JIT、benchmark correctness、warm micro-benchmark 等内部行为。

用户视角还没有 `gscript test`。测试能力存在，但它绑定 Go module、Go test package、环境变量和 repo layout，不是语言用户能直接使用的项目级 test runner。

## User-Facing Gaps

- 命令模型没有子命令层级。运行、REPL、诊断、inspect、benchmark、test 都混在平铺 flags 或外部脚本里。
- 没有稳定 help 结构、shell completion、machine-readable command inventory、统一配置发现或项目根发现。
- 没有独立语言用户需要的 `fmt`、`lint`、`test`、`bench`、`doc`、`mod/pkg` 命令。
- `cmd/dump` 和 `cmd/dump_bytecode` 是有用能力，但不可发现，且没有纳入兼容承诺。
- JSON 输出只覆盖部分诊断点，缺少统一 envelope：command、version、platform、inputs、diagnostics、artifacts、exit status。
- 诊断输出大量写 stderr；这对维护者可用，但对 CI、IDE、编辑器和 web runner 不稳定。
- benchmark harness 很强，但名字、参数和输出属于仓库维护工具，不是 `gscript bench` 的产品化接口。
- 测试入口依赖 `go test` 和 translated official oracle；没有 GScript 原生测试文件约定、断言库、fixture、snapshot、coverage 或 watch mode。
- REPL 只有逐行 parse/exec、`exit`/`quit`；没有 multiline awareness、history、completion、inspect、load file、engine mode 显示或错误位置体验。
- 没有 package manifest、lockfile、module graph、dependency cache、registry/publish/install/update/audit 命令。
- 没有统一 exit-code contract。脚本 runtime failure、parse failure、usage failure、tool failure、timeout 需要被 CI 稳定区分。

## Required Command Set for an Independent Language Release

第一阶段应保留现有 `gscript FILE`、`gscript -e`、`gscript` 兼容入口，但新增子命令，并把新文档只承诺子命令面。

| Command | Purpose |
|---|---|
| `gscript run FILE [-- args...]` | 执行脚本；承载 `--vm`、`--jit`、profile、runtime args、diagnostics。 |
| `gscript eval SOURCE [-- args...]` | 执行字符串，替代直接暴露 `-e` 作为主体验。 |
| `gscript repl` | 交互式 shell，显示 engine、history、completion、multiline 输入。 |
| `gscript check PATH...` | parse/compile/type-adjacent 静态检查，不运行程序。 |
| `gscript fmt PATH...` | parser-backed formatter，支持 `--check`、`--write`、stdin。 |
| `gscript lint PATH...` | 静态诊断，输出 text/json/sarif，使用稳定 diagnostic codes。 |
| `gscript test [PATH...]` | 原生测试 runner，支持 package/project discovery、filter、timeout、JSON。 |
| `gscript bench [PATH...]` | 用户级 benchmark runner；内部可调用现有 harness，但输出稳定。 |
| `gscript diag ...` | JIT/runtime 诊断入口，收敛现有 `-exit-stats-json`、runtime path、warm dump、spec worklist。 |
| `gscript inspect bytecode FILE` | 收敛 `cmd/dump`、`cmd/dump_bytecode`，支持 `--proto`、`--json`。 |
| `gscript doc [PATH...]` | 生成 API/stdlib/project docs。 |
| `gscript mod init/tidy/graph/vendor` | 项目和依赖管理。 |
| `gscript pkg publish/install/update/audit` | registry/package 操作；可在 registry 就绪前先保留 experimental。 |
| `gscript env` | 打印 toolchain、cache、config、platform、JIT availability。 |
| `gscript version --json` | 稳定版本和 build metadata。 |
| `gscript capabilities --json` | 机器可读能力：engine、JIT、stdlib、diagnostic、platform、benchmark deps。 |

建议的公共约定：

- 通用 flags：`--json`、`--output PATH`、`--quiet`、`--verbose`、`--config PATH`、`--no-config`、`--color=auto|always|never`、`--timeout DURATION`。
- 通用 exit codes：`0` 成功，`1` 程序或测试失败，`2` usage/config 错误，`3` parse/check/lint 失败，`4` tool/internal 错误，`124` timeout。
- JSON envelope 至少包含 `schema_version`、`command`、`tool_version`、`platform`、`inputs`、`diagnostics`、`artifacts`、`status`。
- 所有能写文件的命令默认支持 dry-run/check 模式，便于 CI 和 editor integration。

## Boundary with the Embedding API

CLI 和 Go embedding API 应共享编译、执行、诊断、module resolution 的底层能力，但不要互相暴露内部实现细节。

CLI 负责：

- 面向人和 CI 的命令语义、参数解析、配置发现、项目根发现、stdout/stderr/JSON 输出。
- 文件系统工作流：run、fmt、lint、test、bench、doc、package、artifact bundle。
- 稳定 exit code、shell completion、help、artifact manifest。
- 把 JIT/runtime 诊断组织成用户可消费的报告。

Embedding API 负责：

- Go 进程内 VM lifecycle、context cancellation、sandbox/limits、host function binding、typed value conversion。
- 可缓存的 compiled program/module artifact。
- 可插拔 module loader、stdout/stderr hooks、error/stack diagnostics。
- 并发和 pool 语义。

两者边界：

- CLI 不应该要求用户理解 `internal/runtime.Value`、`internal/vm.FuncProto`、MethodJIT native PC map 结构或 Go test package layout。
- Embedding API 不应该依赖 CLI flags、stderr 文本、Python benchmark scripts 或 shell wrappers 作为稳定集成点。
- `gscript run/check/test/bench` 可以调用同一组 public/internal service packages，但 public Go API 应提供结构化 options/result，而 CLI 只是其中一个 adapter。
- `cmd/dump`/`dump_bytecode` 的能力应先产品化为 `inspect` 输出；embedding API 只在确有外部需求时提供稳定 introspection object，避免过早承诺 bytecode layout。
- Benchmark harness 可以继续作为维护者工具存在；`gscript bench` 应定义更小、更稳定的用户契约，再选择性复用这些 harness。

## Landing Plan

### P0

- 新增 `gscript run/eval/repl` 子命令，同时保留旧入口兼容；文档把子命令定义为推荐入口。
- 定义全局 exit-code contract 和 JSON envelope v1。
- 把 `cmd/dump`、`cmd/dump_bytecode` 收敛为 `gscript inspect bytecode FILE [--proto NAME] [--json]`。
- 新增 `gscript check`：只做 lex/parse/compile，不运行程序，输出稳定 source diagnostics。
- 新增 `gscript capabilities --json` 与 `gscript version --json`，让 CI/IDE 能探测 JIT、stdlib 和平台能力。
- 为现有 benchmark/test 文档补一张“维护者入口 vs 未来用户入口”映射，避免继续扩散零散命令。

### P1

- 新增 `gscript test`，先支持 repo 内 `.gs` test convention、filter、timeout、JSON；official Lua oracle 保持维护者 gate，不作为普通用户默认依赖。
- 新增 `gscript fmt --check/--write/stdin`，基于 parser/AST/golden tests，不做 regex formatter。
- 新增 `gscript lint --format=text|json|sarif`，定义 diagnostic code registry、severity、suppression。
- 新增 `gscript bench` 的稳定用户层，内部可复用 `timing_compare.py`/`strict_guard.py`，但输出使用 CLI envelope。
- 把现有 JIT flags 分组到 `gscript diag`，保留 run flags 的兼容 alias。
- 建立项目配置文件和项目根发现规则，例如 `gscript.toml`。

### P2

- 新增 `gscript mod` 和 `gscript pkg`，覆盖 manifest、lockfile、dependency graph、registry、publish/install/update/audit。
- 新增 `gscript doc` 和 stdlib/project documentation generation。
- 新增 shell completion、editor/LSP integration hooks、watch mode。
- 新增 coverage、snapshot、fixture、benchmark history 等 test/bench 生态能力。
- 将维护者 benchmark artifacts 统一成 manifest，支持跨机器比较、CI 上传和长期趋势查询。
- 为 inspect/diagnostics 增加 symbolized JIT profile、guard timeline、pass summary、source map viewer 等高级报告。

## Immediate Recommendation

不要继续新增独立 `cmd/*` 或一次性 shell wrapper 作为用户入口。短期最佳路径是先建立 `gscript` 子命令框架和输出契约，把已有强能力包进稳定命令面；随后再补 fmt/lint/test/bench/package 等独立语言发布必需的用户级工具。
