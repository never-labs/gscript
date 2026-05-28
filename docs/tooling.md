# GScript Tooling Roadmap

本文审计 GScript 的生产级工具链现状，并给出缺口、优先级和建议的
CLI/API 形状。范围覆盖 CLI、formatter、linter、package/module 管理、
test runner、benchmark runner、文档生成、诊断输出、REPL、source map 和 CI
入口。

## Executive Summary

GScript 当前最强的工具链资产是运行时/性能诊断：`cmd/gscript` 已提供 JIT
开关、profile、Tier 2 统计、runtime path 统计、warm dump、source/PC map
等生产路径诊断；`benchmarks/` 里也有 `timing_compare.py`、`strict_guard.py`、
`diagnose.py`、`triage.py` 等成熟 harness。CLI 现在也有最小可用的
`gscript fmt`、`gscript lint` 和 `gscript test` 子命令入口。

主要缺口在用户级开发工具：`fmt`/`lint`/`test` 已有脚手架，但 formatter
仍不是 AST pretty printer，linter 目前只做词法/语法诊断；还没有稳定的
`gscript bench`、`gscript doc` 子命令；REPL 仍是最小可用；
module 管理只有 `require()` 解析和 `package.loaded` 缓存，没有 manifest、
lockfile、版本、发布或 dependency graph；CI 没有仓库内统一入口。

生产级路线应避免再增加零散脚本，优先收敛到一个稳定的 `gscript` 子命令
界面，同时保留现有 Python/shell harness 作为底层实现，逐步迁移公共输出
schema。

## Priority Model

| Priority | Meaning |
|---|---|
| P0 | 阻塞生产使用或回归防护，应先做 |
| P1 | 显著提升日常开发/CI/发布质量 |
| P2 | 生态与 IDE 体验，适合在核心入口稳定后推进 |

## CLI

### Current State

`cmd/gscript/main.go` 是主入口，使用 Go `flag` 包提供平铺 flags 和少量
子命令：

- `-e` 执行字符串。
- 无参数进入 REPL。
- 文件参数执行脚本。
- `-vm` 强制 bytecode VM，`-jit` 默认启用 JIT 并隐式启用 VM。
- `-cpuprofile`、`-memprofile` 输出 Go pprof。
- JIT/diagnostic flags 包括 `-jit-stats`、`-jit-timeline`、
  `-jit-timeline-format`、`-jit-dump-warm`、`-jit-dump-proto`、
  `-exit-stats`、`-exit-stats-json`、`-tier2-perf-stats`、
  `-tier2-perf-stats-json`、`-tier2-spec-state-json`、
  `-tier2-spec-worklist-json`、`-jit-op-audit`、`-jit-op-audit-json`、
  `-coroutine-stats`、`-runtime-path-stats`、`-runtime-path-stats-json`。
- `gscript test <path-or-dir>` 递归运行 `.gs` 文件，可用同名 `.out` 做
  stdout golden 对比。
- `gscript fmt [--check] [--write] [--stdin-file-name FILE] <path-or-dir> [...]`
  解析并规范基础空白。
- `gscript lint <path-or-dir> [...]` 解析文件/目录并报告 `GS1001` 词法或
  语法错误。

There are also developer binaries:

- `cmd/dump` disassembles a named child proto.
- `cmd/dump_bytecode` recursively dumps bytecode and JIT eligibility decisions.

### Gaps

- Flat flags do not scale to formatter/linter/test/bench/doc/package workflows.
- No `--help` hierarchy, machine-readable command inventory, shell completion,
  config discovery, or stable exit-code contract.
- Diagnostics mostly write to stderr; JSON flags exist but are not unified under
  one schema envelope.
- `cmd/dump` and `cmd/dump_bytecode` are useful but not discoverable as
  supported commands.

### Recommendations

P0:

- Introduce subcommands while preserving current invocation compatibility:
  `gscript run [flags] FILE [-- args...]`, `gscript eval EXPR`,
  `gscript repl`, `gscript diag ...`.
- Define common flags: `--json`, `--output PATH`, `--quiet`, `--verbose`,
  `--config PATH`, `--no-config`, `--color=auto|always|never`.
- Define exit codes: `0` success, `1` runtime/test failure, `2` usage/config
  error, `3` parse/lint failure, `4` internal/tool failure, `124` timeout.

P1:

- Fold `dump` and `dump_bytecode` into `gscript inspect bytecode`.
- Add `gscript capabilities --json` reporting platform, JIT availability,
  stdlib modules, diagnostic support, and benchmark dependencies.

Suggested CLI:

```bash
gscript run script.gs -- arg1 arg2
gscript run --vm script.gs
gscript run --jit --diag=exit,tier2,runtime-path --json script.gs
gscript inspect bytecode script.gs --proto sum
gscript capabilities --json
```

Suggested Go API:

```go
type RunOptions struct {
    Mode        ExecutionMode
    Args        []string
    Diagnostics DiagnosticMask
    Output      io.Writer
    ErrorOutput io.Writer
}

func RunFile(ctx context.Context, path string, opts RunOptions) (*RunResult, error)
func RunString(ctx context.Context, source, name string, opts RunOptions) (*RunResult, error)
```

## Formatter

### Current State

`gscript fmt` is a minimal parser-backed formatter scaffold. It accepts one or
more `.gs` files or directories, recursively discovers `.gs` files in
directories, and parses each file before changing bytes. The current formatter
contract is intentionally narrow:

- normalize CRLF/CR line endings to LF;
- trim trailing spaces and tabs from each line;
- collapse trailing blank lines;
- ensure exactly one final newline for non-empty and empty files;
- refuse to write files that fail lexing or parsing.

Default file mode writes changed files in place and prints each changed filename
to stdout. `--check` reports files that would change without writing. `--write`
is accepted as an explicit spelling of the default write mode. `--check` and
`--write` are mutually exclusive.

Editor stdin mode is available with `--stdin-file-name FILE`. In this mode
`gscript fmt --stdin-file-name foo.gs < in.gs` reads source from stdin, uses the
provided filename for diagnostics, writes the formatted result to stdout, and
never writes files. `--check --stdin-file-name foo.gs < in.gs` exits non-zero
and reports `foo.gs: not formatted` when stdin would change.

The AST/parser currently do not expose a complete comment-preserving pretty
printer. Until that exists, `gscript fmt` remains a whitespace normalizer with
a clear no-op boundary for AST layout.

### Gaps

- No AST pretty printer or indentation engine.
- Unknown comment attachment and AST round-trip fidelity beyond parse success.
- No formatter golden tests.

### Recommendations

P0:

- Build a parser-backed formatter package, not regex formatting.
- Start with stable, narrow formatting: whitespace, indentation, statement
  layout, table literals, function bodies, control blocks.
- Add golden tests under a future formatter test package before exposing write
  mode.

P1:

- Add comment preservation and idempotence tests.

Suggested CLI:

```bash
gscript fmt file.gs
gscript fmt --check ./...
gscript fmt --write ./examples ./tests
gscript fmt --stdin-file-name scratch.gs < scratch.gs
```

Suggested API:

```go
type FormatOptions struct {
    IndentWidth int
    LineWidth   int
}

func Format(filename string, src []byte, opts FormatOptions) ([]byte, error)
```

## Linter

### Current State

`gscript lint` is a minimal parser-backed linter scaffold. It accepts one or
more `.gs` files or directories, recursively discovers `.gs` files in
directories, and reports lexer/parser failures as `GS1001` errors:

```text
path/to/file.gs: GS1001 error: parse error: parse error at 1:6: expected ...
```

This provides a stable command entry point and file traversal path for future
lint rules without introducing static-analysis behavior before the diagnostic
model is designed.

### Gaps

- No static checks for unused locals, unreachable code, shadowing hazards,
  malformed `require()` paths, unsupported JIT patterns, or deprecated stdlib
  APIs.
- No diagnostic code registry or severity model.
- No SARIF/JSON output for CI annotation.

### Recommendations

P0:

- Add parser-backed `gscript lint` with diagnostics using stable codes such as
  `GS1001` parse, `GS1101` unreachable, `GS1201` unresolved require,
  `GS1301` suspicious global, `GS2001` JIT portability warning.
- Emit line/column, source span, severity, message, and optional fix hint.

P1:

- Add `--format=text|json|sarif`.
- Add config suppressions and inline ignore comments.

Suggested CLI:

```bash
gscript lint ./...
gscript lint --format=json tests/01_basic.gs
gscript lint --deny=warning --format=sarif --output /tmp/gscript.sarif ./...
```

Suggested diagnostic schema:

```json
{
  "code": "GS1201",
  "severity": "error",
  "message": "module not found",
  "file": "main.gs",
  "range": {"start": {"line": 10, "column": 12}, "end": {"line": 10, "column": 25}},
  "hint": "checked script dir and configured module paths"
}
```

## Package And Module Management

### Current State

Runtime module loading exists:

- Tree-walker `require(name)` checks `package.loaded`, an interpreter module
  cache, builtin modules, then resolves `strings.ReplaceAll(name, ".", "/") +
  ".gs"` relative to script dir.
- Bytecode VM `require(name)` checks `package.loaded`, globally registered
  table/function modules, then loads the resolved `.gs` file.
- `gscript.WithRequirePath(path)` sets the base directory for embedded usage.
- `ExecFile` and CLI file execution set script dir to the executed file's
  directory.
- Standard library modules are documented under `docs/stdlib/`.

### Gaps

- No module manifest, dependency lockfile, semantic versioning, package registry,
  vendoring, checksum verification, or module graph command.
- `require()` path rules are implicit and split between tree-walker and VM
  implementations.
- No clear search path model beyond script dir / `WithRequirePath`.
- No package authoring/testing/publishing workflow.

### Recommendations

P0:

- Document and centralize module resolution rules so interpreter and VM share
  one resolver.
- Add a project manifest, e.g. `gscript.toml`, with `name`, `version`,
  `main`, `paths`, `dependencies`, and `tool` sections.
- Add `gscript mod graph` and `gscript mod verify` before adding networked
  installs.

P1:

- Add lockfile with content hashes.
- Add local path dependencies and vendoring.
- Add builtin module inventory through `gscript mod stdlib --json`.

Suggested CLI:

```bash
gscript mod init
gscript mod graph
gscript mod verify
gscript mod vendor
gscript mod stdlib --json
```

Suggested resolver API:

```go
type ModuleResolver interface {
    Resolve(ctx context.Context, fromFile, name string) (ResolvedModule, error)
}

type ResolvedModule struct {
    Name       string
    Path       string
    Source     []byte
    Builtin    bool
    Version    string
    Checksum   string
}
```

## Test Runner

### Current State

Testing is Go-driven:

- Top-level README recommends `go test ./... -count=1 -p 1 -timeout=600s`.
- `docs/testing-matrix.md` documents core VM/JIT/runtime tests, hand-written
  `tests/01_basic.gs` through `tests/12_advanced.gs`, official Lua translated
  cases, and optional JIT parity through `GSCRIPT_OFFICIAL_CHECK_JIT=1`.
- Official translated cases compare Lua output to GScript output and are not
  performance timings.

### Current `gscript test` behavior

`gscript test <path-or-dir>` runs `.gs` files directly. A single file path must
end in `.gs`; a directory path is walked recursively and all `.gs` files are run
in sorted order. By default the runner only checks whether each script succeeds.
If a sibling `<name>.out` file exists next to `<name>.gs`, the runner captures
stdout and compares it exactly to the golden file. Mismatches report the `.gs`
file, the `.out` file, and an expected/got stdout summary.

### Gaps

- Test discovery is encoded in Go tests, not a language-level test manifest.
- No standard assertion/test API, fixture layout, per-test timeout, JSON report,
  JUnit report, coverage, or watch mode.

### Recommendations

P0:

- Add `gscript test` as a wrapper over language-level tests and existing Go
  semantic harnesses.
- Define test file discovery: `*_test.gs`, `tests/**/*.gs`, and manifest opt-in.
- Add JSON and JUnit output for CI.

P1:

- Add package-level fixtures, filtered runs, parallelism controls, and timeout.
- Add coverage once source mapping is stable enough for line attribution.

Suggested CLI:

```bash
gscript test ./...
gscript test tests --run TestStrings --mode=vm
gscript test tests/official_lua_cases --mode=jit --json --output /tmp/test.json
gscript test --junit /tmp/gscript-junit.xml --timeout=60s ./...
```

Suggested test API:

```go
type TestOptions struct {
    Mode     ExecutionMode
    Pattern  string
    Timeout  time.Duration
    Parallel int
}

func RunTests(ctx context.Context, roots []string, opts TestOptions) (*TestReport, error)
```

## Benchmark Runner

### Current State

Benchmark tooling is the most mature area:

- `benchmarks/timing_compare.py` is the primary local before/after harness for
  current worktree vs clean `HEAD` vs LuaJIT, with calibrated repeats, timing
  source tracking, confidence intervals, scaling, JSON and Markdown output.
- `benchmarks/strict_guard.py` is the broad truth pass for suite, extended, and
  variants, with modes `vm`, `default`, `no_filter`, and `luajit`.
- `benchmarks/diagnose.py` collects timing, exits, runtime-path counters,
  Tier 2 perf, speculation state/worklist, and optional pprof/warm dumps.
- `benchmarks/triage.py`, `profile_exits.py`, `jit_addr_map.py`,
  `regression_guard.py`, and shell wrappers cover focused diagnosis and
  compatibility workflows.
- `scripts/diag.sh` produces production-parity Tier 2 diagnostic dumps through
  Go tests.

### Gaps

- Benchmark entry points are powerful but fragmented across Python, shell, and
  Go test.
- No single `gscript bench` UX for users or CI.
- Result schemas are close but not centralized/versioned.
- No explicit machine-readable benchmark manifest schema beyond current JSON
  files and script conventions.

### Recommendations

P0:

- Add `gscript bench` as a stable facade over existing harnesses.
- Version benchmark result JSON schema.
- Keep suite/extended/variants/official as separate groups; do not collapse to
  one score.

P1:

- Add CI profiles: `smoke`, `pull-request`, `release`, `publish`.
- Add benchmark environment capture: OS, arch, Go version, LuaJIT version,
  CPU model, thermal/power hints when available.

Suggested CLI:

```bash
gscript bench --group=suite --mode=default --mode=luajit --runs=5
gscript bench --profile=pr --json /tmp/bench.json --markdown /tmp/bench.md
gscript bench diagnose --bench=suite/spectral_norm --pprof --warm-dump
gscript bench strict --all-groups --runs=3 --warmup=1
```

Suggested schema envelope:

```json
{
  "schema": "gscript.benchmark.v1",
  "environment": {},
  "groups": ["suite", "extended"],
  "results": []
}
```

## Documentation Generation

### Current State

Docs are Markdown/HTML under `docs/`, including stdlib pages and performance
writeups. `docs/_config.yml` and `docs/_layouts/default.html` indicate static
site generation support. There is no visible doc generator command.

### Gaps

- Stdlib docs appear hand-maintained.
- No API/doc extraction from runtime builtin registration.
- No docs freshness check for CLI flags, stdlib modules, or language features.
- No link checker or generated command reference.

### Recommendations

P0:

- Add `gscript doc generate` that emits CLI reference and stdlib inventory from
  code metadata.
- Add `gscript doc check` for stale generated docs and broken internal links.

P1:

- Generate language feature matrix references from `tests/feature_matrix.json`
  and semantic test inventory.
- Add examples validation so docs snippets execute in CI.

Suggested CLI:

```bash
gscript doc generate --output docs/reference
gscript doc check
gscript doc check --snippets docs
```

Suggested metadata API:

```go
type DocProvider interface {
    Commands() []CommandDoc
    StdlibModules() []ModuleDoc
    Diagnostics() []DiagnosticDoc
}
```

## Diagnostic Output

### Current State

Diagnostics are a strong existing capability:

- CLI can emit JIT stats, exit stats text/JSON, Tier 2 perf text/JSON,
  speculation state/worklist JSON, coroutine stats, runtime path stats
  text/JSON, JIT timelines, op audit text/JSON, warm dumps, CPU profiles, and
  memory profiles.
- Warm dumps include source maps and PC maps for mapping native code ranges back
  to IR/opcode/source metadata.
- `benchmarks/diagnose.py`, `triage.py`, `debug_artifact.py`, and
  `jit_addr_map.py` turn raw diagnostics into artifact bundles and summaries.
- `scripts/diagnostics_bundle.sh` is the current scriptable local collection
  entrypoint. It writes an ignored `diagnostics/<timestamp>/` bundle by default,
  or another explicitly supplied ignored/external directory, with git revision,
  Go environment summary, quick Go test logs, and quick benchmark/strict-guard
  summaries when the local harnesses are available.

### Gaps

- Diagnostic outputs are not wrapped in one versioned schema.
- Some JSON writes to stderr, which complicates scripting.
- No common redaction, artifact manifest, or stable diagnostic code registry.
- External profiler integration still needs direct JIT symbol/perf-map support.

### Recommendations

P0:

- Add `--diag-output PATH` and `--diag-format=json|jsonl|text`.
- Wrap diagnostic JSON with schema name, version, command, file, mode, platform,
  timestamp, and payload.
- Make stderr human-first and output files machine-first.

P1:

- Add `gscript diag collect` to create a portable artifact directory.
- Add direct JIT symbol/perf-map integration where platform support allows.

Suggested CLI:

```bash
gscript run --jit --diag=jit,exit,tier2,runtime-path script.gs \
  --diag-output /tmp/gscript-diag.json
gscript diag collect --bench=suite/spectral_norm --out-dir=/tmp/gscript-diag
gscript diag map-pc --warm-dir=/tmp/gscript-warm --profile=/tmp/cpu.pprof
```

## REPL

### Current State

The no-argument CLI enters a simple REPL:

- Prompt is `>` or `>>`.
- `exit` and `quit` terminate.
- It executes each buffered input through the tree-walking interpreter path.

### Gaps

- No multiline completeness detection; buffer resets after any parse/runtime
  error.
- No bytecode VM/JIT REPL mode.
- No history, completion, introspection commands, paste mode, or structured
  error display.
- No REPL tests or terminal capability abstraction.

### Recommendations

P0:

- Move REPL to explicit `gscript repl`.
- Add parse-completeness detection before execution.
- Add `--mode=interp|vm|jit`.

P1:

- Add history file, completion for globals/modules, commands such as
  `.help`, `.load`, `.mode`, `.stats`, `.inspect`.
- Add optional JSON event stream for embedding.

Suggested CLI:

```bash
gscript repl
gscript repl --mode=vm
gscript repl --mode=jit --stats
```

Suggested REPL commands:

```text
.help
.load path/to/file.gs
.mode vm
.inspect bytecode functionName
.stats jit
```

## Source Map

### Current State

Source metadata exists and is used:

- `vm.FuncProto` has `Source` and `LineInfo`.
- The compiler records source line per bytecode instruction.
- CLI and public API set proto source names for files.
- MethodJIT tracks IR source proto, bytecode PC, source line, and emitted
  native code ranges.
- `BuildIRASMMap` emits JSON-friendly rows with proto, source, source line,
  bytecode PC/op, IR instruction/op/type, and native code start/end.
- Warm dumps write per-function sourcemap/pcmap and aggregate `pcmap.json`.

### Gaps

- No user-facing source-map format contract.
- No source column spans yet, only line and bytecode PC.
- Source maps are diagnostic artifacts, not broadly consumed by errors,
  coverage, formatter, linter, or IDE tooling.
- Eval/REPL source naming is minimal.

### Recommendations

P0:

- Define `gscript.sourcemap.v1` with source file, line, optional column,
  bytecode PC, IR id, native code range, and inlining/source-proto fields.
- Use the same range model for parser/linter/runtime diagnostics.

P1:

- Add columns/spans in lexer/parser and carry them to AST, bytecode, and IR.
- Add coverage and profiler consumers once spans are stable.

Suggested CLI:

```bash
gscript inspect sourcemap script.gs --json
gscript run --jit --emit-sourcemap=/tmp/script.sourcemap.json script.gs
```

## CI Entry Points

### Current State

No `.github/workflows`, Makefile, justfile, Taskfile, or golangci config was
found in the reviewed scope. README and docs recommend direct commands:

```bash
go test ./... -count=1 -p 1 -timeout=600s
python3 benchmarks/timing_compare.py --all --runs 7 --warmup 2 --timeout 900 --time-source script
```

`docs/testing-matrix.md` documents more precise correctness and performance
commands.

### Gaps

- No canonical `ci` command for local and hosted CI parity.
- No fast/slow/release split.
- No formatter/linter/doc checks yet.
- No benchmark guard profile with explicit thresholds for PR vs release.

### Recommendations

P0:

- Add one stable script or subcommand as the CI entry. Prefer `gscript ci` once
  subcommands exist; until then add a small shell script in a later change.
- Define profiles:
  `smoke`, `pr`, `full`, `perf`, `release`.

P1:

- Add hosted CI workflows after local commands are deterministic.
- Add artifacts upload for benchmark Markdown/JSON, diagnose bundles on failure,
  and test reports.

Suggested CLI:

```bash
gscript ci smoke
gscript ci pr
gscript ci perf --json /tmp/perf.json --markdown /tmp/perf.md
gscript ci release
```

Suggested profile commands:

```bash
# smoke
go test ./cmd/... ./gscript ./internal/... ./tests -count=1 -timeout=120s

# pr
go test ./... -count=1 -p 1 -timeout=600s
python3 benchmarks/strict_guard.py --group suite --runs=3 --warmup=1 --timeout=90 --json /tmp/gscript-pr-bench.json

# release
go test ./... -count=1 -p 1 -timeout=600s
python3 benchmarks/strict_guard.py --group suite --group extended --group variants --group official --runs=5 --warmup=2 --timeout=240 --json /tmp/gscript-release-bench.json --markdown /tmp/gscript-release-bench.md
python3 benchmarks/official_perf_coverage.py --check --json /tmp/official_perf_coverage.json --markdown /tmp/official_perf_coverage.md
```

## Ordered Roadmap

### P0: Stabilize Entry Points

1. Add `gscript run`, `eval`, `repl`, `inspect`, `diag`, `test`, `bench`, `doc`,
   `mod`, and `ci` command skeletons while preserving legacy flags.
2. Add shared JSON diagnostic envelope and output routing.
3. Add `gscript test` facade over existing Go semantic harnesses.
4. Add `gscript bench` facade over `strict_guard.py`, `timing_compare.py`, and
   `diagnose.py`.
5. Centralize module resolution and document `require()` semantics.

### P1: Make CI And Developer Loops Reliable

1. Add `gscript fmt --check` and `gscript lint --format=json|sarif`.
2. Add CI profiles and hosted workflow.
3. Add generated CLI/stdlib docs plus docs freshness checks.
4. Add REPL completeness, history, and VM/JIT mode.
5. Version benchmark, diagnostic, sourcemap, and test-report schemas.

### P2: Build Ecosystem Tooling

1. Add lockfile, vendoring, package verification, and local package publishing.
2. Add line/column source spans through lexer, parser, bytecode, IR, and
   diagnostics.
3. Add coverage, LSP-friendly APIs, shell completion, and editor integrations.
4. Add direct profiler symbolization for JIT frames.

## Command Inventory Target

```text
gscript run
gscript eval
gscript repl
gscript fmt
gscript lint
gscript test
gscript bench
gscript bench diagnose
gscript inspect bytecode
gscript inspect sourcemap
gscript diag collect
gscript diag map-pc
gscript mod init
gscript mod graph
gscript mod verify
gscript doc generate
gscript doc check
gscript ci smoke|pr|perf|release
gscript capabilities
```

## Non-Goals For The First Iteration

- Do not replace the existing Python benchmark harnesses immediately; wrap them
  first and migrate only after schemas and UX settle.
- Do not introduce networked package install before local manifest, graph,
  verification, and lockfile semantics are stable.
- Do not make formatter write mode default before round-trip/comment tests are
  strong enough.
- Do not collapse benchmark groups into a single score.
