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
