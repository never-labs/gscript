# Leia Language Specification

本文定义 Leia 作为 Go-style scripting language 的生产级语义边界。它不是普通设计文档，而是 Phase 0 的硬产出：embedding API、formatter、linter、安全沙盒、VM、JIT、stdlib 和官方兼容测试都必须以这里写下来的语法和行为为基准。目标不是复刻 Lua 5.4 的每个表面行为，而是把已经依赖的 Lua 兼容项、故意不同项、仍需规范项和验证 gate 固定下来，避免 JIT、VM、解释器、stdlib 或 host API 在后续优化中漂移。

当前语义基线：

- 兼容 oracle：`tests/language/MANIFEST.md` 中 441 个 translated official Lua case，默认比较 Lua oracle 与 `leia -vm` 输出；设置 `LEIA_OFFICIAL_CHECK_JIT=1` 时还比较 `leia -jit`。
- 能力 ledger：`tests/language/MISSING_CAPABILITIES.md`。它记录已覆盖能力、明确非目标和后续翻译官方 case 时新增的候选能力。
- 覆盖矩阵：`docs/language-feature-checklist.md` 与 `tests/feature_matrix.json`。语言语义稳定项必须同时有 parser/bytecode/runtime/official gate 或明确标为 `semantic_only` / `not_applicable`。
- 实现锚点：`internal/runtime` 是值模型、标准库、解释器、错误、表、协程和 channel 语义源；`internal/vm` 是文件模式 bytecode、JIT gate 与 VM parity 源。

## Phase 0 Hard Deliverables

Phase 0 不以“代码能跑”为完成条件，而以“语义可被引用、测试、实现”为完成条件。

必须交付：

1. 本文件包含 Leia 自己的 BNF/EBNF、词法边界、表达式优先级、语句语义、值模型、错误模型、模块模型和 host capability 边界。
2. 每个稳定语言特性都能回答三件事：语法是什么、运行时行为是什么、哪些测试锁定它。
3. `docs/language-feature-checklist.md` 和 `tests/feature_matrix.json` 每一行都能映射到本文件某个章节或明确 non-goal。
4. formatter/linter 只格式化/诊断本文件承诺的语法；不能从 parser 实现反推出未写入规范的用户承诺。
5. embedding API 只能暴露本文件承诺的行为；如果底层行为还在 Tier 1/Tier 2，API 必须标为实验或不暴露。
6. sandbox 只能按本文件定义的 capability surface 隔离：文件、网络、进程、环境变量、模块加载、时间、随机、debug、host callback、CPU/内存/递归/协程/channel。
7. VM/JIT 优化不能改变本文件的行为；遇到未规范语义必须 VM fallback 或保持解释器路径。

Phase 0 验收 gate：

```bash
go test ./tests -run 'TestFeatureMatrix|TestLanguageConformanceTranslatedCases' -count=1
go test ./internal/runtime ./internal/vm ./leia -count=1
```

新增或修改语言行为时，必须先改本文件和矩阵，再改实现。

## Lexical Grammar

Leia 源码是 UTF-8 文本。词法层当前按 byte 扫描标识符与操作符；标识符稳定承诺为 ASCII 字母、数字和 `_`，且首字符不能是数字。非 ASCII 标识符不是 Phase 0 承诺。

空白包括空格、tab、换行和回车。换行本身不是语句分隔符；`;` 是可选分隔符，主要用于一行多个语句或 C-style `for`。文件以 EOF 结束。

注释：

```ebnf
line_comment  = "//" { any_char_except_newline } ;
block_comment = "/*" { any_char } "*/" ;
```

关键字：

```text
func return if else elseif for range break continue in var go chan defer const goto
true false nil
```

`in` 和 `var` 当前是保留词；除非本文件后续定义它们的语义，否则用户代码不得依赖它们可作为标识符。

字面量：

```ebnf
identifier     = letter { letter | digit | "_" } ;
letter         = "A".."Z" | "a".."z" | "_" ;
digit          = "0".."9" ;

number_lit     = decimal_lit | base_int_lit ;
decimal_int_lit
              = digit { digit | "_" } ;
decimal_lit    = digit { digit | "_" } [ "." { digit | "_" } ] [ exponent ] ;
exponent       = ( "e" | "E" ) [ "+" | "-" ] digit { digit | "_" } ;
base_int_lit   = "0" ( "x" | "X" | "b" | "B" | "o" | "O" ) base_digit { base_digit | "_" } ;

string_lit     = quoted_string | raw_string ;
quoted_string  = '"' { string_char | escape } '"' ;
raw_string     = "`" { any_char_except_backtick } "`" ;
escape         = "\\" ( "n" | "r" | "t" | "\\" | "\"" | "0"
                 | "x" hex hex
                 | "u" hex hex hex hex
                 | decimal_byte_escape ) ;
```

数字 `_` 分隔符只用于可读性，不改变值。base integer 支持 `0x`、`0b`、`0o`。raw string 不处理 escape。

## Syntactic Grammar

本节是 Phase 0 语法合同。它描述用户可写语法；parser 内部辅助规则不等于用户承诺。

约定：`{ x }` 表示重复 0 次或多次，`[ x ]` 表示可选，`|` 表示选择。

```ebnf
program       = { separator | statement } EOF ;
separator     = ";" ;
block         = "{" { separator | statement } "}" ;

statement     = func_decl
              | if_stmt
              | for_stmt
              | return_stmt
              | break_stmt
              | continue_stmt
              | goto_stmt
              | label_stmt
              | go_stmt
              | defer_stmt
              | const_decl
              | simple_stmt ;

func_decl     = "func" identifier param_list block ;
param_list    = "(" [ param { "," param } [ "," vararg_param ]
                    | vararg_param ] ")" ;
param         = identifier ;
vararg_param  = "..." | identifier "..." ;

if_stmt       = "if" expr block { "elseif" expr block } [ "else" block ] ;

for_stmt      = "for" block
              | "for" expr block
              | "for" simple_stmt ";" expr ";" simple_stmt block
              | "for" identifier [ "," identifier ] ":=" "range" expr block ;

return_stmt   = "return" [ expr_list ] ;
break_stmt    = "break" ;
continue_stmt = "continue" ;
goto_stmt     = "goto" identifier ;
label_stmt    = identifier ":" ;
go_stmt       = "go" call_expr ;
defer_stmt    = "defer" call_expr ;

const_decl    = "const" identifier { "," identifier } ( ":=" | "=" ) expr_list ;

simple_stmt   = expr ":=" expr_list
              | expr "=" expr_list
              | expr compound_op expr
              | expr ( "++" | "--" )
              | expr { "," expr } ( ":=" | "=" ) expr_list
              | expr "<-" expr
              | call_expr
              | method_call_expr
              | recv_expr ;

compound_op   = "+=" | "-=" | "*=" | "/=" ;
expr_list     = expr { "," expr } ;
```

`:=` 声明左侧必须是标识符列表。`=` 赋值左侧可以是变量、字段或索引表达式。`go` 和 `defer` 后面必须是普通调用或 method call。`label_stmt` 与 `obj:method(...)` 的歧义按 method call 优先：只有 `identifier ":"` 且后面不是 `identifier "("` 时才是 label。

表达式语法：

```ebnf
expr           = or_expr ;
or_expr        = and_expr { "||" and_expr } ;
and_expr       = compare_expr { "&&" compare_expr } ;
compare_expr   = concat_expr { compare_op concat_expr } ;
concat_expr    = additive_expr [ ".." concat_expr ] ;
additive_expr  = multiplicative_expr { additive_op multiplicative_expr } ;
multiplicative_expr
               = unary_expr { multiplicative_op unary_expr } ;
unary_expr     = ( "-" | "!" | "#" | "^" | "<-" ) unary_expr
               | power_expr ;
power_expr     = postfix_expr [ "**" unary_expr ] ;
postfix_expr   = primary { selector | index | call | method_call } ;

selector       = "." identifier ;
index          = "[" expr "]" ;
call           = "(" [ argument_list ] ")" ;
method_call    = ":" identifier "(" [ argument_list ] ")" ;
argument_list  = expr { "," expr } ;

primary        = number_lit
               | string_lit
               | "true"
               | "false"
               | "nil"
               | "..."
               | identifier
               | "(" expr ")"
               | func_lit
               | typed_dense_lit
               | table_lit ;

func_lit       = "func" param_list block ;
typed_dense_lit
              = ( "[]" | "[" decimal_int_lit "]" ) dense_dtype
                "{" [ expr { ( "," | ";" ) expr } [ "," | ";" ] ] "}" ;
dense_dtype    = "f64" | "f32" | "i64" | "i32" | "bool" ;
table_lit      = "{" [ table_field { ( "," | ";" ) table_field } [ "," | ";" ] ] "}" ;
table_field    = "[" expr "]" ":" expr
               | identifier ":" expr
               | expr ;

compare_op     = "==" | "!=" | "<" | "<=" | ">" | ">=" ;
additive_op    = "+" | "-" | "|" | "^" ;
multiplicative_op
               = "*" | "/" | "%" | "<<" | ">>" | "&" | "&^" ;
```

Special forms:

```ebnf
make_channel   = "make" "(" "chan" [ "," expr ] ")" ;
recv_expr      = "<-" expr ;
```

`make(chan)` and `make(chan, n)` are parsed as channel construction only when the callee is the identifier `make` and the first argument token is `chan`.

## Operator Precedence

从低到高：

| Level | Operators | Associativity | Notes |
|---|---|---|---|
| 1 | `||` | left | short-circuit, returns operand value |
| 2 | `&&` | left | short-circuit, returns operand value |
| 3 | `==` `!=` `<` `<=` `>` `>=` | left | comparison/metamethod aware where specified |
| 4 | `..` | right | string concat, may use `__concat` |
| 5 | `+` `-` `|` `^` | left | arithmetic and bitwise xor |
| 6 | `*` `/` `%` `<<` `>>` `&` `&^` | left | multiplication, division, modulo, shifts, bitwise and/and-not |
| 7 | `**` | right | exponentiation |
| 8 | unary `-` `!` `#` `^` `<-` | right | negate, logical not, length, bitwise not, receive |
| 9 | `.` `[]` `()` `:` | left | field/index/call/method call |

Unary `^` is bitwise not, while binary `^` is bitwise xor. This is an intentional Leia difference from Lua.

## Core Behavioral Rules

这些行为是语言规范，而不是实现建议。

### Program and blocks

- A source file is a chunk. Top-level statements execute in order in the selected global environment.
- `{ ... }` creates a lexical block for local declarations and labels.
- A statement may be followed by `;`; semicolons are otherwise only required by C-style `for init; cond; post`.
- A runtime error aborts the current protected boundary unless caught by `pcall`/`xpcall`.

### Variables and assignment

- `:=` declares lexical locals in the current scope. `const` declares readonly lexical locals.
- `=` assigns to existing variables or to table field/index targets.
- Multiple assignment evaluates right-hand expressions before assignment and then adjusts arity: missing values become `nil`; extra values are discarded; final multi-return expression can expand.
- `const` prevents rebinding the binding, not mutation of a table value stored in it.
- `++` and `--` are statement forms equivalent to numeric `target = target +/- 1` with the same target assignment rules.
- Compound assignments evaluate the target location once at language level and then apply the corresponding arithmetic operation before storing.

### Calls and returns

- Function calls pass arguments after multi-return adjustment rules. Extra arguments are accepted; missing parameters are `nil`.
- `...` is only valid inside vararg functions or vararg-compatible chunks.
- `return` may return zero or more values. Function calls used in tail position of return/argument/list contexts follow the language multi-return adjustment rules.
- Method call `obj:method(args...)` passes `obj` as receiver/self according to existing method-call semantics; it is distinct from label syntax.

### Control flow

- `if`, `elseif`, `for expr`, `&&`, `||`, and `!` use truthiness: only `nil` and `false` are falsey.
- `for {}` is an infinite loop until `break`, `return`, `goto`, error, coroutine yield, or explicit host-backed cancellation logic observes a cancellation channel.
- `for cond {}` tests `cond` before each iteration.
- `for init; cond; post {}` executes `init` once, then `cond` before each iteration, then `post` after each normal iteration.
- `for k := range expr {}` and `for k, v := range expr {}` use the Leia range protocol for tables/channels/iterables as implemented and tested; exact extension points must be written before new range sources become stable.
- `break` and `continue` apply to the innermost loop.
- `goto` is function-local. It may jump out of a block but must not jump into a deeper scope or over declarations whose lifetime would be skipped.

### Defer and goroutines

- `defer call(...)` evaluates the deferred call target and arguments according to current tested behavior and drains in LIFO order when the enclosing function exits normally or by error.
- `go call(...)` starts asynchronous execution through host-backed goroutine semantics. Scheduling order is intentionally unspecified; synchronization must use channels or explicit waits.
- `defer` and `go` accept only calls because non-call expressions would not define useful side effects.

### Tables and objects

- `{ expr, expr }` creates array-style fields with 1-based sequence convention.
- `{ name: expr }` creates a string key field equivalent to `["name"]: expr`.
- `{ [expr]: value }` evaluates `expr` as a computed key.
- Table keys use raw identity/equality for lookup; `__eq` does not affect key identity.
- `nil` keys are invalid. NaN key behavior must be rejected or normalized consistently and is not stable until covered by tests.

### Experimental data-oriented literals

- `[]f64{1, 2}` creates a typed dense array literal AST node with dynamic length.
- `[3]f64{1, 2, 3}` creates a typed dense array literal AST node with fixed length `3`.
- The experimental dense dtypes are exactly `f64`, `f32`, `i64`, `i32`, and `bool`.
- This is currently a parser/AST surface. Runtime representation, bytecode lowering, coercion rules, fixed-length validation, mutation semantics, and interop with table APIs are intentionally not stable here.
- A typed dense literal may appear wherever an expression may appear, including as an array-style table field: `{[]f32{1, 2}}`.

### Channels

- `make(chan)` creates an unbuffered channel. `make(chan, n)` creates a buffered channel with non-negative capacity `n`.
- `ch <- value` sends to a channel. `<-ch` receives from a channel.
- `value, ok := <-ch` receives a value and a boolean. Closed-and-drained channels return `nil, false`.
- `select { case ...: ... }` blocks until a case is ready when no `default` exists. With `default`, it performs a non-blocking probe.
- Receive cases support `case value := <-ch:` and `case value, ok := <-ch:`. Send cases use `case ch <- value:`.
- `time.after(seconds)` returns a channel that becomes ready after the delay. `context.withCancel()` and `context.withTimeout(seconds)` expose cancellation through `ctx.done`.
- Blocking behavior follows Go-channel intuition, but scheduling and fairness are not deterministic user contracts.

## Stability Contract

### Tier 0: Stable now

这些行为已经可以被用户文档和第三方脚本依赖。任何变更都必须先改规范、补 translated official case 或 runtime/vm focused test，再改实现。

- 值类型集合：`nil`、`boolean`、`number`、`string`、`table`、`function`、`coroutine`、`channel`。
- `number` 包含 integer 与 float 两个内部子类；`type(1)` 与 `type(1.0)` 都返回 `"number"`，`math.type` 暴露子类。
- Lua 风格 truthiness：只有 `nil` 和 `false` 为 false。
- 多返回、vararg、普通赋值、调用、表构造的尾部展开/非尾部单值调整；Leia 额外提供显式 `spread(expr)` / `table.spread(...)`。
- lexical local、closure/upvalue、shadowing、loop-local capture、函数返回后 upvalue 存活。
- `const` readonly binding 与 `defer` LIFO cleanup。
- `label:` / `goto label` 的函数内跳转规则。
- table 基础语义、metatable 常用事件、raw helpers、`pairs` / `ipairs` / `next`。
- Lua 风格 `pcall` / `xpcall` / `error` / `assert`，并允许非字符串错误对象。
- coroutine create/resume/yield/wrap/status/isyieldable 多值传递。
- Go-style channel/goroutine host concurrency surface。
- source diagnostics：`SourceError` 形态为 `source:line:column: error`，支持底层错误 unwrap。
- stdlib module identity：全局模块表、`require("name")` 返回值、`package.loaded[name]` 必须保持同一表身份。

### Tier 1: Stable after written spec and gates

这些行为已有实现和测试，但还需要更精确的用户可见 contract。

- table 长度在 sparse/nil hole/0 key/large integer key 场景的完整定义。
- `pairs` 默认顺序是否承诺稳定。当前只能承诺遍历所有 live raw keys；除测试显式依赖处外，不应承诺排序。
- string pattern dialect：当前覆盖 Lua pattern 大量行为和 Go-host `regexp`，但需要把 magic set、NUL、frontier、balanced、replacement callback、empty match 推进规则写成单独表。
- numeric coercion：算术、比较、`tonumber`、bitwise、`bit32`、`bits` 的 string-to-number 和 overflow 边界需要一张统一矩阵。
- JIT semantic gates：哪些 op 可 native，哪些必须回退 VM，应从优化文档提升为 language stability gate。
- host stdlib 错误形状：参数错误是抛 runtime error，I/O 操作错误是 `nil, err` 返回，process exit 是可捕获错误，这些需要逐模块列明。

### Tier 2: Explicitly unstable

这些不得写成用户承诺，除非先进入 Tier 1。

- JIT 是否为某段代码生成 native code。
- table 内部 array/hash/shape/typed-array/dense-matrix/lazy-tree 表示。
- GC 时间、对象扫描顺序、finalizer 行为。Leia 不承诺 Lua `__gc` 表 finalizer；资源清理由 `defer` 表达。
- exact error wording，除非 specific official case 或 runtime test 已锁定。稳定的是错误类别、抛/返回模型、source 坐标和错误对象可见值。
- debug hook 的内部事件数量和 VM/JIT instruction 级位置。稳定的是 documented event surface。

## Lua-Compatible Surface

Leia 保留 Lua 兼容项的原则：脚本语义兼容优先于文案兼容；用户可观察输出兼容优先于内部调度兼容；Lua 私有 C API 和 VM 调试协议不作为目标。

已稳定兼容项：

- literals：字符串 escape、NUL 字节、数字 literal、long-string translated cases。Leia 额外支持 Go-style raw string；Lua long bracket delimiter 不作为源语法承诺。
- locals/functions：local scope、shadowing、closures、upvalues、tail-style calls、anonymous invocation、method `:` self 调用。
- control flow：`if`、loops、`break`、numeric for、generic for、short-circuit `and` / `or` value semantics。
- calls：固定参数、extra args ignored、missing args nil-filled、多返回调整、vararg forwarding、nested call adjustment。
- tables：constructor、array/hash keys、string fields、`table.insert/remove/concat/sort/unpack/move/pack`、rawget/rawset/rawlen/rawequal。
- iteration：`pairs` / `ipairs` / `next` protocol，`__pairs`，generic-for nil-first termination，多返回 iterator payload prefix。
- metamethods：`__index`、`__newindex`、`__call`、`__tostring`、`__len`、`__concat`、`__eq`、`__lt`、`__le`、arithmetic metamethods、protected `__metatable`。
- numbers/math：int/float arithmetic、division returns float、modulo Lua sign semantics、NaN comparisons false、Infinity/NaN handling in math helpers、`tonumber` decimal/hex/base parsing、`math.random` protocol、`math.ult`。
- strings：byte/sub/char/format/rep/reverse/find/match/gmatch/gsub/table.concat binary strings、tostring metamethod, lazy concat must be semantics-preserving.
- errors：`error` any value, `assert(false, value)`, `pcall`/`xpcall` return protocol, handler transformation, nested protected calls.
- coroutines：yield/resume multi values including interior nil, dead status behavior, wrap generator behavior, yieldability checks.
- GC control subset：`collectgarbage("collect"|"stop"|"restart"|"isrunning"|"count"|"step"|"incremental"|"generational"|"stats")` with Go runtime backed diagnostics.

## Intentional Differences From Lua

这些差异是语言设计，不是缺口。

| Area | Leia contract | Rationale / compat mapping |
|---|---|---|
| Local attributes | `const name := expr` / `const name = expr`; `defer call(...)` | 不支持 Lua `<const>` / `<close>` 语法。`const` 禁止重绑定但不冻结 table 内容；`defer` 是确定性 cleanup。 |
| Environment | `script.env(table)` / `script.sandbox(table)` / script options | 不支持 Lua `_ENV` 隐式 upvalue 作为语言机制。loader/env 必须显式。 |
| Raw strings | Go-style backtick raw string | 不承诺 Lua `[=[...]=]` delimiter 源语法。translated official case 可保留等价值。 |
| Bitwise operators | `&` `|` `^` `&^` `<<` `>>` and unary `^` | 不使用 Lua `~` 语法；保留 `bit32` 兼容库与 `bits` Go-host 64-bit helper。 |
| Spread | `spread(expr)` / `table.spread(t, i, j)` | 非尾部主动展开必须显式。兼容层把需要的 Lua idiom 翻译为 spread。 |
| Debug | `debug.info` / `stack` / `globals` / `value` / `traceback` / event hooks | 不复刻 `debug.getlocal/getupvalue/getinfo` slot protocol、line/count/coroutine hooks。 |
| C API tests | `testkit` | 不复刻 Lua private `T.testC` 指令协议。 |
| Binary chunks | source-level `load` / `loadfile` / `script.compile` / `script.eval` | 不承诺 `string.dump` 或 stripped chunk binary compatibility。 |
| Finalization | `defer` and `collectgarbage("stats")` | 不承诺 table `__gc` finalizer 调度。 |
| Host stdlib | Go standard-library shaped modules | `fs`、`path`、`net`、`http`、`regexp`、`json` 等以 Go-host semantics 为准，不逐字复刻 Lua ecosystem。 |

## Errors And Diagnostics

错误模型必须同时适合脚本、host embedding、VM/JIT fallback。

Stable rules:

- Runtime error 是 Go `error` 层面的失败；script `error(value)` 使用 `LuaError{Value}` 包装，`Value` 可为任意 Leia 值。
- `pcall(f, ...)` 成功返回 `true, ...results`；失败返回 `false, errValue`。如果错误不是 `LuaError` value，应返回稳定 string/diagnostic value。
- `xpcall(f, handler, ...)` 对 protected error 调用 handler；handler 的多返回按 single result 调整，避免把 handler 多返回误当 protected results。
- `assert(v, msg)` 当 `v` truthy 时返回原参数；当 falsey 时抛 `msg` 或默认错误。`msg` 不强制 string 化。
- Compile/runtime diagnostics 通过 `SourceError` 增加 `SourceName`、`Line`、`Column`，并保留底层错误 unwrap。
- Parser/lexer/token 错误文案可以 Leia 风格，但必须包含可定位 source 坐标；文件模式、`load`、`script.compile`、`script.eval` 要一致。
- 参数错误使用 `bad argument #N to 'name' (...)` 风格；exact text 只有被 official case 锁定时才是 breaking surface。
- I/O、process、network 这类 host 操作的可恢复失败优先返回 `nil, err` 或结构化 result；类型错误、缺参、非法 option 抛 runtime error。
- `defer` 在 normal return 与 error path 都必须 LIFO drain；defer 本身出错时，当前实现的 error propagation/order 必须由 official/runtime case 锁定后才能改。

Roadmap gates:

1. 新增 `docs/error-model.md` 或本文件附录表，列出 core builtin、stdlib、loader、channel、coroutine 的抛错/返回错误策略。
2. 为每类错误新增一个 official translated case 或 focused test：compile source, runtime source, non-string error, host recoverable error, defer-on-error, xpcall-handler-error。
3. VM/JIT fallback 必须带 source frame；native path 不得丢失 script function name/sourceName/line/column。

## Modules, Loading, And Scope

Stable rules:

- Global environment 是 lexical environment；chunk 可以通过 host/script API 显式选择 env 或 sandbox。
- `load`、`loadfile`、`dofile`、`require` 在 VM 文件模式下执行 bytecode chunk，并共享当前 globals、script dir 和 `package.loaded`。
- Builtin stdlib modules 必须同时存在于 globals、`require(name)`、`package.loaded[name]`，且 identity 相同。
- File-backed `require` 使用 script directory relative resolution；成功后 cache 到 `package.loaded`，重复 require 不重复执行模块 body。
- `package.path` 当前不是 Lua package searcher contract；`package.path` 以 `scriptDir` 诊断/兼容为主，不能承诺 Lua pattern searchers。
- `const` 是 binding-level readonly。host `SetGlobal` 或 embedding API 可绕过 script-level const，但脚本内 assignment 必须失败。
- `goto` label function-local unique；允许跳出 block/loop，禁止跳入更深 block 或跳过同 block local/function 声明。

Open specification items:

- 明确 top-level `return` 在 REPL、`load` chunk、file mode、`require` module 中的结果/缓存语义。
- 明确 `script.env` 是否 sync back 到 source table，哪些 loader path 会 sync back，哪些 sandbox 完全 isolated。
- 明确 module error 时是否污染 `package.loaded`，以及 partially loaded module 的 cleanup 策略。
- 明确 circular require 策略：preload sentinel、deadlock/error、还是允许 partially initialized table。

Executable route:

1. 增加 loader conformance table：`load` / `loadfile` / `dofile` / `require` / `script.compile` / `script.eval` / `script.loadFile` / `script.runFile`。
2. 每行列出 sourceName、scriptDir、env fallback、sandbox、return values、error wrapping、cache behavior。
3. 为 circular require、module error、relative path、builtin identity、env sync 各补一个 official case。

## Tables, Arrays, And Metatables

Stable rules:

- Table 是唯一 aggregate/object 类型，支持 integer、float、string、boolean、table、function 等 key；`nil` key invalid，NaN key must be rejected or normalized consistently at public API boundary.
- Integer array convention is 1-based for sequence library functions. Internal array slot 0 may exist for raw storage, but sequence APIs use Lua-compatible 1..n unless a case explicitly targets key `0`.
- Raw operations bypass metatable：`rawget`、`rawset`、`rawlen`、`rawequal`。
- `__index` and `__newindex` support function and table forwarding, including chains and self-referential guards covered by tests.
- Existing-key writes bypass `__newindex`; missing-key writes trigger it.
- `__len` affects `#t` and VM-aware proxy helpers where implemented; `rawlen` bypasses it.
- `__pairs` controls `pairs(t)`; default `pairs` returns iterator/state/control triple.
- `ipairs` iterates positive integer keys from 1 until first nil and must visit false values.
- `__eq` is not used for table key lookup; table keys are identity/raw key based.
- `table.unpack` / `table.spread` hard limit: at most `1_000_000` return values, else immediate `"too many results"` error without scanning sparse range.
- VM-aware proxy helpers: `table.insert` / `remove` / `move` / `unpack` / `sort` / `concat` must interact with `__index` / `__newindex` / `__len` where tests require.
- Go-host raw helpers such as `table.keys` / `values` / `count` / `copy` / `merge` / `unique` / `reverse` / `slice` / `zip` inspect stored raw table content and do not promise `__pairs` or virtual `__index` semantics.

Open specification items:

- Exact `#t` boundary for mixed sparse numeric keys, holes, negative keys, float integer-equivalent keys and key `0`.
- Mutation during `pairs` order and visibility. Current safe contract should be “no duplicate live key guarantee is not promised”; official cases may lock specific delete-current behavior.
- NaN key diagnostics across `rawset`, constructor fields, stdlib helpers and VM opcodes.
- Metamethod recursion depth and cycle diagnostics for `__index` / `__newindex` / arithmetic chains.
- `table.sort` default ordering between mixed number/string/table values should be limited to currently tested comparable subsets unless specified.

Executable route:

1. Create a table semantics matrix with rows: raw lookup, indexed lookup, assignment, length, iteration, stdlib sequence helper, raw helper.
2. Columns: ordinary table, sparse table, proxy `__index`, proxy `__newindex`, `__len`, `__pairs`, protected metatable, concurrent table.
3. Add official cases for every cell where behavior is user-facing and currently relied on.
4. Make JIT semantic gate reject native table ops unless it implements the same metamethod/error path or has a proven guard+deopt contract.

## Coroutines And Concurrency

Stable coroutine rules:

- `coroutine.create(fn)` returns suspended coroutine; first resume starts execution with resume args.
- `coroutine.resume(co, ...)` returns `true, ...yieldedOrReturned` on success, `false, err` on failure.
- `coroutine.yield(...)` transfers all values including interior nil; next resume args are returned from the yield expression.
- Coroutine statuses: `"suspended"`, `"running"`, `"normal"`, `"dead"`.
- Resuming dead coroutine is a protected failure; `coroutine.wrap` propagates errors according to current tested behavior.
- Coroutine `defer` frames survive across yield and drain on return/error.
- Yield across unsupported host/JIT boundaries must be rejected by semantic gate or routed through VM path; native JIT may not replay side effects around yield.

Stable concurrency rules:

- `channel` wraps Go channel semantics: buffered/unbuffered capacity, blocking send/receive, receive from closed returns `nil, false`, send-on-closed and close-of-closed are errors.
- `make(chan, capacity)` capacity must be non-negative integer in host int range.
- `len(ch)` and `cap(ch)` expose queued length and buffer capacity.
- `select` is a stable statement form. It supports blocking cases, non-blocking `default`, send cases, receive cases, and receive comma-ok cases.
- `time.after(seconds)` is the stable timeout-channel primitive. Negative durations are errors.
- `context.background()`, `context.withCancel()`, and `context.withTimeout(seconds)` are the stable script-level cancellation surface. Cancellation is cooperative through `ctx.done`.
- `sync.waitgroup()`, `sync.mutex()`, `sync.rwmutex()`, and `sync.once()` are host-backed coarse synchronization primitives.
- Concurrent table mutation is not generally safe unless a table has been explicitly marked concurrent internally; script-level public contract should avoid data-race guarantees until specified.
- Goroutine scheduling order is not deterministic and must not be part of official outputs except through synchronization with channels or explicit waits.

Open specification items:

- Panic/recover analogue: current language should use `pcall` / `xpcall`, not Go panic semantics.
- Cross-coroutine `debug` visibility and hook event order.

Executable route:

1. Keep tests synchronized through channels or explicit waits; do not assert sleep-based ordering.
2. Keep JIT disabled across resume/yield/send/receive/select until side-effect replay and blocking-point deopt contracts are written.
3. Extend host API cancellation integration gradually by routing blocking stdlib operations through explicit context/options rather than VM-wide preemption.

## Numbers, Strings, And Stdlib

### Numbers

Stable rules:

- `IntValue` stores small signed integers directly; integer overflow may promote to float. This is representation, but user-visible `math.type` may observe int vs float.
- `/` always returns float. Integer-looking float and int compare numerically.
- `%` follows Lua modulo sign semantics.
- Numeric strings are accepted by arithmetic coercions and `tonumber` where official cases cover decimal, hex and whitespace forms.
- `tonumber(value, base)` supports bases 2..36 and returns nil for invalid forms.
- NaN is never equal, and order comparisons involving NaN are false.
- `math.random`/`math.randomseed` protocol is deterministic only under explicit seed; unseeded randomness is not stable.
- `bit32` is 32-bit compatibility library; `bits` is Go-style 64-bit helper; direct bitwise operators are Leia syntax.

Open items:

- Exact overflow rules for `+ - * <<` across int48 storage, int64 expectations and float promotion.
- Whether arithmetic coercion should accept `"nan"` / `"inf"` forever. Current parsing rejects NaN/Inf string to number in several paths; lock with tests.
- Rounding mode table for `math.floor` / `ceil` / `round` / `trunc` over huge floats.

### Strings

Stable rules:

- Strings are byte strings and may contain NUL.
- `string` library supports Lua-compatible byte/sub/char/format/rep/reverse/find/match/gmatch/gsub where translated official cases cover behavior.
- `..` concatenates strings and numbers directly; other values require `__concat` or error.
- Lazy concat and native string arena are invisible optimizations; materialized value must match eager concat.
- `utf8` uses Go `unicode/utf8` rules for strict validation and exposes Leia helpers `validate`, `sanitize`, case conversion and codepoint operations.
- `regexp` is Go RE2 style, separate from Lua pattern matching.

Open items:

- Full Lua pattern compatibility grid: classes, captures, balanced, frontier, empty matches, replacement table/function/fallback, invalid pattern diagnostics.
- `string.format` unsupported specifiers and pointer formatting contract.
- Unicode case conversion locale behavior: should be Go default, not locale-dependent.

### Stdlib

Stable module list:

`base64`, `binary`, `bit32`, `bits`, `bytes`, `color`, `compress`, `container`, `crypto`, `csv`, `debug`, `encoding`, `fs`, `hash`, `http`, `io`, `json`, `log`, `math`, `matrix`, `net`, `os`, `path`, `process`, `rand`, `regexp`, `rl`, `script`, `sort`, `string`, `table`, `testkit`, `time`, `url`, `utf8`, `uuid`, `vec`.

Stable stdlib rules:

- Builtin modules are plain tables and can be monkey-patched unless protected by future explicit design.
- Host wrappers use Go canonical behavior where documented: `path`/`fs`, `net/url`, `encoding/csv`, `regexp`, crypto, compression, time layouts.
- JSON encoding treats array/sparse/mixed tables according to Leia Go-host JSON tests; NaN/Inf must be rejected.
- Binary pack/unpack uses explicit endian and Go-style field tokens. `string.pack`/`unpack` may expose compatibility entry points but do not promise Lua format-string parity.
- `io` file handles expose open/closed state; `file:read` supports `"a"`, `"l"`, `"L"`, `"n"`, `read(n)`, `read(0)` and multiple formats.

Open items:

- Per-module argument coercion matrix.
- Per-module recoverable error return shape.
- Which modules are safe in sandbox by default.
- Whether monkey-patching stdlib tables affects VM/JIT fast stdcall paths immediately, or whether fast paths require identity/version guards.

Executable route:

1. Generate or maintain `stdlib-contract.md` from `StdlibModuleNames()` and public exported function tables.
2. For every module, list: pure/deterministic, host I/O, network, process, crypto/random, sandbox-safe, error model, VM/JIT fast path allowed.
3. Add a module identity test whenever a new stdlib module is registered.

## VM, JIT, And Semantic Gates

Language stability is owned by VM semantics. JIT is an optimization tier and must be semantically invisible.

Required gates:

- `go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v`
- `LEIA_OFFICIAL_CHECK_JIT=1 go test ./tests -run TestLanguageConformanceTranslatedCases -count=1 -v`
- Focused `internal/runtime` tests for new interpreter/runtime behavior.
- Focused `internal/vm` tests for compiler/opcode/source-diagnostic/JIT-gate parity.
- `tests/feature_matrix.json` row updated for any new language feature.

JIT must VM-fallback or gate when encountering:

- multi-return ABI not representable in native path,
- top-level side-effect replay risk,
- `const` / `defer` / readonly control state,
- upvalue arithmetic without deopt recovery,
- dynamic arithmetic/len/concat/comparison requiring metamethod or precise error slow path,
- call/self/generic-for/resume/yield boundaries,
- blocking channel operations,
- stdlib monkey-patchable function identity without guard.

Before enabling a native path, add:

1. Positive official case with `LEIA_OFFICIAL_CHECK_JIT=1`.
2. Negative/deopt case where guard fails after warmup.
3. Source diagnostic case if the native op can throw.
4. Side-effect replay case if fallback can happen after mutation/call/I/O.

## Production Roadmap

### Phase A: Freeze the contract

Deliverables:

- Keep this file as the language contract index.
- Add appendices or sibling docs for error model, table semantics, loader/module semantics, numeric/string semantics and stdlib contract.
- Mark each item Stable / Stable after gates / Unstable.

Exit criteria:

- Every row in `docs/language-feature-checklist.md` links to at least one spec section or explicit non-goal.
- `MISSING_CAPABILITIES.md` only contains capability ledger/backlog, not hidden language policy.

### Phase B: Turn boundaries into executable matrices

Deliverables:

- `tests/feature_matrix.json` gains references for semantic boundary cases, not only broad feature names.
- Official translated cases cover table sparse length, loader env/cache errors, coroutine defer, channel close/drain, numeric coercion, pattern edge cases and stdlib module identity.
- Runtime/vm tests isolate each behavior that is hard to express through official stdout.

Exit criteria:

- A contributor can answer “is this a language change?” by finding one matrix row and one test.
- JIT semantic gate list has no undocumented fallback reason.

### Phase C: Specify host-facing embedding contract

Deliverables:

- Host API docs for creating interpreters, setting globals, loading files, script env/sandbox, process args/entry, output capture and error unwrapping.
- Error object mapping: Leia value -> Go error -> `pcall` result.
- Concurrency contract for channels and goroutines.

Exit criteria:

- Embedding users can rely on module identity, source diagnostics, env isolation and recoverable host errors without reading implementation code.

### Phase D: Compatibility layer policy

Deliverables:

- Separate “Lua compatibility translation guide”: `_ENV`, `<const>/<close>`, long brackets, Lua bitwise, debug API, `string.dump`, package searchers and C API tests.
- Mark which differences are permanent and which may be implemented as optional compat library.

Exit criteria:

- No production language decision is blocked on “what would Lua do?” unless the row is explicitly Lua-compatible.

## Change-Control Checklist

Before changing language-visible behavior:

1. Identify the section in this document.
2. If absent, add the boundary first and classify it Stable / Tier 1 / Unstable.
3. Add or update official translated case when stdout/value behavior is user-visible.
4. Add runtime/vm focused test when behavior is internal to source diagnostics, JIT fallback, host errors or embedding.
5. Update `tests/feature_matrix.json` if parser/bytecode/runtime/JIT/official coverage changes.
6. Run VM official gate; run JIT official gate if bytecode, VM, stdlib fast path or JIT policy changed.
7. Do not change unrelated semantics to make a JIT optimization pass; gate native and keep VM semantics stable.
