# GScript 后续能力候选

这些条目来自继续翻译官方 Lua 5.4.8 `db.lua`、`api.lua`、`errors.lua` 时遇到的阻塞点。当前阶段只补充 passing tests，因此未把这些片段加入测试用例。

记录原则：这些不是“必须逐字复刻 Lua”的清单。涉及标准库时，优先考虑用 Go 标准库实现功能对等的 GScript API；语法、错误文案、调试字段和边界行为可以按 GScript/Go 运行时模型设计，再按需要提供 Lua 兼容层。

| 能力候选 | 来源片段 | 说明 |
|---|---|---|
| `debug` 标准库基础信息查询 | `db.lua`: `debug.getinfo`, `debug.getlocal`, `debug.getupvalue` | 可先提供适合 GScript/Go 运行时的函数、行号、参数、upvalue 与局部变量可观测模型，不必逐字复刻 Lua 的所有字段。 |
| `debug.traceback` 与保护调用栈信息 | `db.lua` traceback checks, `errors.lua` line/stack-message checks | 可结合 Go 风格错误栈输出设计稳定格式，再为 Lua 兼容层提供常见字段和层级参数。 |
| 调试 hook | `db.lua`: `debug.sethook`, `debug.gethook` | 官方测试依赖 call/return/line/count hook 和协程 hook；GScript 后续可按 VM 指令或源码行建立更明确的 hook 语义。 |
| 嵌套调用/构造中的多返回展开 | `db.lua`: transfer-value checks using `table.unpack`, returned varargs, and table constructors | 当前新增 case 只覆盖直接赋值和显式 vararg packing 的稳定子集。后续可设计 GScript/Go 风格的显式 spread 或多返回传递规则，再决定 Lua 兼容层如何映射嵌套函数调用与表构造位置。 |
| C API 测试库替代层 | `api.lua`: `T.testC`, `T.makeCfunc`, `T.checkmemory` | 官方 `api.lua` 大量依赖 Lua C 测试库。GScript 可考虑提供面向自身 runtime 的测试辅助库，而不是逐字实现 `ltests`。 |
| `load`/`string.dump` 调试信息交互 | `db.lua`, `errors.lua` stripped chunk checks | 若 GScript 后续支持编译/反序列化 chunk，可定义是否保留源码名、行表和局部变量名。 |
| 精细错误位置与 token 诊断 | `errors.lua`: syntax/runtime line checks and token-message checks | 当前 passing tests 只校验错误发生和值传播；后续可逐步稳定行号、调用层级和 token 文案，文案可保持 GScript 风格。 |
| `assert` 非字符串失败载荷 | `errors.lua`: `pcall(assert, false, t)` | Lua 会把非字符串消息对象作为失败结果传播；GScript 当前稳定覆盖字符串消息和普通失败，后续可考虑支持任意值载荷。 |
| Label-directed control flow | `goto.lua`: labels, forward/backward jumps, repeated-label diagnostics, local-scope jumps | GScript 当前不解析 Lua `goto`/label。后续可考虑 Go 风格 label/goto，或显式状态机/结构化跳转能力，并清晰定义块作用域和变量初始化边界。 |
| Local variable attributes | `constructs.lua`, `locals.lua`: `<const>`, `<close>` | 可用更贴近 Go 的 `const`、`defer`/`using` 或资源作用域语法表达只读局部和离开作用域清理，不必复刻 Lua 局部属性语法。 |
| Lexical environment injection | `locals.lua`: `_ENV`, `load(..., env)`, environment upvalues | 若需要类似能力，可设计为显式模块环境、sandbox globals 或脚本执行上下文注入，而不必复刻 Lua `_ENV` 机制。 |
| GC finalizer tracing and runner-integrated progress output | `tracegc.lua`: table `__gc`, repeated remarking for finalization, `io.stderr` progress writes; `all.lua`: `require"tracegc".start()` | `tracegc.lua` 是当前唯一没有独立 passing case 的官方顶层文件。后续可优先设计 Go-style finalization/cleanup hooks、diagnostic output sinks 和 explicit test-runner extension points，而不是追求 Lua finalizer 调度或 module-loading 细节逐字一致。 |
| Binary string packing/unpacking | `tpack.lua`: `string.pack`, `string.unpack`, `string.packsize` | 当前只补 `table.pack`/`table.unpack`/`select` 的 passing 子集。后续可用 Go 的 `encoding/binary`、`math` 和 byte-slice/string 转换实现 GScript 风格的二进制布局 API，再按需提供 Lua 兼容格式串映射。 |
| Full file-handle and standard-stream IO controls | `files.lua`: `file:seek`, `file:flush`, `io.input`, `io.output`, `io.type`, `io.tmpfile`, close-file edge cases | 当前 passing case 覆盖 open/read/write/append/lines/remove/rename。后续可基于 Go `os.File`、`bufio` 和 `io` 设计功能对等的文件句柄、默认输入输出流和临时文件 API，错误返回可保持 GScript/Go 风格。 |
| Process entrypoint and script-loading controls | `main.lua`: command-line args, `-e`/`-l` style loading, `dofile`, `loadfile`, `require`, process exit/status behavior | 当前 `main.lua` 只覆盖 stdout 写入和入口可见的基础函数。后续可按 GScript CLI 模型定义脚本参数表、模块搜索路径、显式加载接口和退出状态，不必复刻 Lua 命令行选项文案。 |
| Runtime source compilation and generated-program execution | `big.lua`, `verybig.lua`, `heavy.lua`, `code.lua`: generated chunks executed with `load` | 官方压力测试会动态生成大块源码再执行。GScript 后续可提供 Go-style compile/eval API、脚本 runner 或测试辅助入口，明确输入、环境和错误模型，不必复刻 Lua chunk 语法或 `load` 参数细节。 |
| Raw and multiline string literal ergonomics | `literals.lua`: long brackets, delimiter-level raw strings, escape suppression | 当前 passing tests 用等价值字符串覆盖内容语义。后续可考虑 Go-style raw string 或 heredoc 形式来表达大段文本和无需转义的字符串，不必复刻 Lua `[=[...]=]` 分隔符层级。 |
| First-class integer bit operations in expressions | `bwcoercion.lua`, `code.lua`: direct bitwise expressions and coercion stress | 当前 passing tests 通过 `bit32`/标准库风格 API 覆盖功能。若要扩展表达式层能力，可按 GScript/Go 的整数模型设计位运算、移位和字符串到整数的转换边界，而不要求 Lua 运算符语法完全一致。 |
