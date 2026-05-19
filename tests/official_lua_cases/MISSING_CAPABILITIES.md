# GScript 后续能力候选

这些条目来自继续翻译官方 Lua 5.4.8 `db.lua`、`api.lua`、`errors.lua` 时遇到的阻塞点。当前阶段只补充 passing tests，因此未把这些片段加入测试用例。

记录原则：这些不是“必须逐字复刻 Lua”的清单。涉及标准库时，优先考虑用 Go 标准库实现功能对等的 GScript API；语法、错误文案、调试字段和边界行为可以按 GScript/Go 运行时模型设计，再按需要提供 Lua 兼容层。

| 能力候选 | 来源片段 | 说明 |
|---|---|---|
| `debug` 标准库基础信息查询 | `db.lua`: `debug.getinfo`, `debug.getlocal`, `debug.getupvalue` | 已补 GScript 风格 `debug.info`、`debug.stack`、`debug.globals`、`debug.value`，暴露函数 kind/name/参数数量/vararg/upvalue 数、运行时调用栈和 globals 快照；不复刻 Lua 局部变量槽位枚举。 |
| `debug.traceback` 与保护调用栈信息 | `db.lua` traceback checks, `errors.lua` line/stack-message checks | 已补 `debug.traceback([message])`，基于解释器真实 script/native 调用栈生成稳定 GScript 格式；另提供 `debug.goStack()` 给 host 诊断 Go goroutine 栈。 |
| 调试 hook | `db.lua`: `debug.sethook`, `debug.gethook` | 官方测试依赖 call/return/line/count hook 和协程 hook；GScript 后续可按 VM 指令或源码行建立更明确的 hook 语义。 |
| 嵌套调用/构造中的多返回展开 | `db.lua`: transfer-value checks using `table.unpack`, returned varargs, and table constructors | 当前新增 case 只覆盖直接赋值和显式 vararg packing 的稳定子集。后续可设计 GScript/Go 风格的显式 spread 或多返回传递规则，再决定 Lua 兼容层如何映射嵌套函数调用与表构造位置。 |
| C API 测试库替代层 | `api.lua`: `T.testC`, `T.makeCfunc`, `T.checkmemory` | 官方 `api.lua` 大量依赖 Lua C 测试库。GScript 可考虑提供面向自身 runtime 的测试辅助库，而不是逐字实现 `ltests`。 |
| `load`/`string.dump` 调试信息交互 | `db.lua`, `errors.lua` stripped chunk checks | 若 GScript 后续支持编译/反序列化 chunk，可定义是否保留源码名、行表和局部变量名。 |
| 精细错误位置与 token 诊断 | `errors.lua`: syntax/runtime line checks and token-message checks | 当前 passing tests 只校验错误发生和值传播；后续可逐步稳定行号、调用层级和 token 文案，文案可保持 GScript 风格。 |
| `assert` 非字符串失败载荷 | `errors.lua`: `pcall(assert, false, t)` | 已支持：`assert(false, value)` 会把任意 GScript 值作为失败载荷交给 `pcall`/`xpcall`，不是字符串化后再传播。 |
| Label-directed control flow | `goto.lua`: labels, forward/backward jumps, repeated-label diagnostics, local-scope jumps | GScript 当前不解析 Lua `goto`/label。后续可考虑 Go 风格 label/goto，或显式状态机/结构化跳转能力，并清晰定义块作用域和变量初始化边界。 |
| Local variable attributes | `constructs.lua`, `locals.lua`: `<const>`, `<close>` | 可用更贴近 Go 的 `const`、`defer`/`using` 或资源作用域语法表达只读局部和离开作用域清理，不必复刻 Lua 局部属性语法。 |
| Lexical environment injection | `locals.lua`: `_ENV`, `load(..., env)`, environment upvalues | 已补 GScript 风格 `script.env(table)` / `script.sandbox(table)`，用于 `script.compile`/`script.eval`/`script.loadFile`/`script.runFile` 的显式环境注入；不复刻 Lua `_ENV` 隐式 upvalue 机制。 |
| GC finalizer tracing and runner-integrated progress output | `tracegc.lua`: table `__gc`, repeated remarking for finalization, `io.stderr` progress writes; `all.lua`: `require"tracegc".start()` | `tracegc.lua` 是当前唯一没有独立 passing case 的官方顶层文件。后续可优先设计 Go-style finalization/cleanup hooks、diagnostic output sinks 和 explicit test-runner extension points，而不是追求 Lua finalizer 调度或 module-loading 细节逐字一致。 |
| Binary string packing/unpacking | `tpack.lua`: `string.pack`, `string.unpack`, `string.packsize` | 已补 GScript/Go 风格 `binary.pack`/`binary.unpack`/`binary.size`，基于 `encoding/binary` 支持显式 endian、常见 int/uint/float、长度前缀 string/bytes 和定长 raw string/bytes。尚未声明 Lua `string.pack` 格式串逐字兼容。 |
| Full file-handle and standard-stream IO controls | `files.lua`: `file:seek`, `file:flush`, `io.input`, `io.output`, `io.type`, `io.tmpfile`, close-file edge cases | 已补文件句柄 `seek`/`flush`、默认输入输出流 `io.input`/`io.output`、`io.type`、`io.tmpfile`、标准流表和关闭后状态。错误返回沿用文件操作 `nil, err`、参数错误抛运行时错误的现有风格。 |
| Process entrypoint and script-loading controls | `main.lua`: command-line args, `-e`/`-l` style loading, `dofile`, `loadfile`, `require`, process exit/status behavior | 已补 `Interpreter.SetArgs`、全局 `arg`、`process.args`、`process.entry`、host-controlled `process.exit`、以及基于脚本目录解析的 `dofile`/`loadfile`/`require` 路径逻辑。CLI 将 `-e`、REPL 和文件模式入口参数统一写入 runtime。 |
| Runtime source compilation and generated-program execution | `big.lua`, `verybig.lua`, `heavy.lua`, `code.lua`: generated chunks executed with `load` | 已补 GScript 风格 `script.compile`/`script.eval`/`script.loadFile`/`script.runFile`，并保留兼容全局 `load`/`loadfile` 入口；支持显式 env/sandbox 与 source/scriptDir 配置，不复刻 Lua chunk dump 语义。 |
| Raw and multiline string literal ergonomics | `literals.lua`: long brackets, delimiter-level raw strings, escape suppression | 已支持 Go-style 反引号 raw string，可跨行且不解释反斜杠转义；不复刻 Lua `[=[...]=]` 分隔符层级。 |
| First-class integer bit operations in expressions | `bwcoercion.lua`, `code.lua`: direct bitwise expressions and coercion stress | 已新增 Go-style `bits` 标准库作为功能对等入口，覆盖 64-bit and/or/xor/not、shift、rotate、bit positions 和 bit counts。若后续需要表达式层语法，可在 `bits` 语义基础上设计 GScript/Go 风格运算符，不要求 Lua 运算符语法完全一致。 |
