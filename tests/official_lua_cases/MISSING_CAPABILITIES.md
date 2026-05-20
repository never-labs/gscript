# GScript 后续能力候选

这些条目最初来自继续翻译官方 Lua 5.4.8 `db.lua`、`api.lua`、`errors.lua` 时遇到的阻塞点。随着后续补齐实现与 passing tests，本文现在是能力 ledger：记录已经覆盖的 GScript 等价能力、明确非目标，以及后续翻译官方 case 时如果再次发现问题才需要新增的候选项。

记录原则：这些不是“必须逐字复刻 Lua”的清单。涉及标准库时，优先考虑用 Go 标准库实现功能对等的 GScript API；语法、错误文案、调试字段和边界行为可以按 GScript/Go 运行时模型设计，再按需要提供 Lua 兼容层。

## 2026-05-20 覆盖审计结论

当前默认官方翻译集已扩展到 437 个 passing case。`KNOWN_FAILURES.md`
仍没有 skipped known failures。本文现在记录三类内容：已经覆盖的
GScript 等价能力、明确不追求 Lua 逐字兼容的设计取舍，以及后续翻译官方
case 时如果再次发现问题才需要新增的能力候选。

本轮新增 VM passing case 覆盖了以下此前主要靠 runtime 单测证明的能力：

- `code_explicit_spread_more`: 显式 `spread(expr)` / `table.spread` 在调用参数和表构造中的展开。
- `db_gscript_diagnostics_more`: `debug.info(function)` 和 `debug.value` 的文件模式 VM 可用性。
- `db_vm_debug_parity_more`: VM 文件模式 `debug.stack`、numeric `debug.info(level)`、sourceName/line/column/function name metadata，以及 debug hook/sink 事件可观测性。
- `main_script_process_more`: `script.eval`/`script.compile` 环境注入，以及 `process.setArgs`/`process.args`/`process.entry`。
- `attrib_const_defer_gscript`: VM compiler 对 Go-style `const` 只读 binding 与 `defer` LIFO cleanup 的支持，覆盖错误路径 drain。
- `attrib_require_builtin_modules_more`: builtin 标准库模块可通过 `require` 返回同一个表，并同步到 `package.loaded`。
- `bitwise_direct_ops_more`: VM compiler/runtime 对 Go-style 直接位运算表达式的支持。
- `coroutine_multi_yield_resume_more`: 多值 `coroutine.yield` 经 `coroutine.resume` 透传，包括中间 nil 和 resume 参数回传。
- `files_io_read_formats_more`: `file:read` 的 `"L"`、`read(n)`、`read(0)` EOF probe、partial EOF 和一次多格式项返回。
- `pm_pattern_runtime_more`: `string.gmatch(s, p, init)` 初始位置和 `string.gsub` 函数 replacement 回调。
- `files_seek_overwrite_more` / `files_tmpfile_flush_type_more`: 文件 seek、flush、tmpfile、关闭状态和回读行为。
- `sort_table_proxy_metamethods`: `table.move` / `table.unpack` / `table.sort` 通过 `__index`、`__newindex`、`__len` 与 proxy table 交互。
- `events_concat_numeric_operand_more`: VM concat 链在触发 `__concat` 时保持右结合调度，并把相邻数字操作数按 number 传给 metamethod。
- `sort_unpack_sparse_boundary_more`: 极值 sparse `table.unpack` / `table.spread` 范围超过 host 多返回上限时快速报错，避免逐项扫描。
- `nextvar_table_insert_remove_proxy_metamethods`: `table.insert` / `table.remove` 的 proxy table 路径在 JIT slow path 下走 VM `__index` / `__newindex` 语义。
- `api_arith_metamethod_chain_more`: `__add` / `__mod` / `__unm` 算术 metamethod 链式组合。
- `tracegc_stats_progress_more`: `collectgarbage("stats")` 的 Go-host 诊断形状和显式 collection 后进度字段。
- `main_generated_chunk_eval_more` / `big_generated_eval_env_more` / `heavy_generated_concat_more`: 生成代码 compile/eval 的显式环境、边界表访问和拼接压力路径。
- `calls_fixed_arity_nested_adjust_more`: 固定参数调用里的嵌套多返回参数按单值调整，同时保持数值互递归路径可 JIT。
- `binary_namespace_more`: `binary.pack` / `binary.unpack` / `binary.size` 的 Go-style endian/field token API。
- `strings_go_helpers_more`: `string.split`、trim variants、`replaceAll`、`join`、`title`、padding、`isNumeric` 等 Go-host helper。
- `utf8_go_helpers_more`: `utf8.reverse`、codepoint `sub`、Unicode case conversion、`charclass`、validate/sanitize。
- `debug_host_helpers_more`: `debug.traceback` / `stack` / `globals` / `info` / `value` / `goStack` 的 host 诊断面。
- `files_file_lines_streams_more`: `file:lines()`、标准流表和 `io.type` open/closed 状态。
- `table_go_helpers_more`: table keys/values/contains/indexOf/copy/merge/count/unique/reverse/slice/zip 等非回调 helper。
- `table_raw_helpers_more`: `table.toArray` / `table.unique` 与 rawget/rawset/rawlen/rawequal raw helper 语义、invalid nil/NaN key diagnostics。
- `table_higher_order_vm_callbacks_more`: table map/filter/reduce/fromArray 在 VM 文件模式下调用脚本 callback。
- `table_proxy_concat_flatten_more`: `table.concat` 通过 proxy `__len` / `__index` 读表，以及 inline nested table literal 供 `table.flatten` 正确消费。
- `go_channel_host_more`: Go-style channel/goroutine 在 VM 文件模式下覆盖 buffered producer、range over closed channel、closed receive 和容量/关闭错误路径。
- `matrix_host_dense_more`: `matrix.dense` / `matrix.getf` / `matrix.setf` 在脚本层覆盖 flat backing 读写、普通索引一致性和稳定参数错误。
- `attrib_require_go_host_modules_more`: Go-host 标准库模块通过 `require` 和 `package.loaded` 保持全局表身份一致。
- `attrib_require_all_stdlib_more`: 剩余 Go-host stdlib 模块通过 `require` 和 `package.loaded` 保持全局表身份一致，包括 stub-safe `rl`。
- `http_background_server_more`: `http.listen` / `router.listen` 支持 `{background: true}`，返回可 `close` / `shutdown` / `wait` 的 server handle，脚本层可稳定做本地请求回环测试。
- `json_go_host_more`: `json.encode` / `decode` / `pretty` 在脚本层覆盖嵌套结构、round-trip、非法 JSON 和 trailing data 拒绝。
- `regexp_go_host_more`: Go RE2 风格 `regexp` helper 和 compiled object 覆盖 match/find/submatch/split/replace 与 invalid pattern 错误。
- `fs_path_go_host_more`: Go-host `fs` / `path` helper 覆盖临时路径、读写追加、stat、copy/rename、readdir、removeAll 和 path match/clean/split。
- `fs_path_glob_cwd_more`: `fs.cwd` / `fs.chdir` / `fs.glob` / `fs.tempdir` 和 `path.abs` / `isAbs` / `rel` / separator helpers 覆盖 cwd 状态与临时目录恢复。
- `url_go_host_more`: Go-host URL parse/build/escape/query/join helper 覆盖结构化字段、query table、invalid escape 和 validity checks。
- `bytes_hash_base64_go_host_more`: `bytes` buffer、hex/XOR/repeat/concat、base64/url-base64、hash/HMAC/CRC32 覆盖 deterministic binary helper。
- `bytes_numeric_buffer_more`: `bytes.new()` buffer 覆盖 numeric little-endian writes、byte/string reads、hex round-trip、concat/fromString 和 reset。
- `time_compress_encoding_go_host_more`: 固定时间格式化/解析/diff，gzip/zlib/deflate round-trip，以及 hex/base32/INI/XML 编解码覆盖。
- `compress_error_levels_more`: gzip/zlib/deflate explicit/fallback compression levels、bad input decode errors 和 missing-argument diagnostics。
- `encoding_ini_xml_roundtrip_more`: INI encode/decode round-trip、XML escape/unescape numeric entity round-trip 和 malformed base32/base32hex decode error 路径。
- `net_http_background_roundtrip_more`: `net.get` / `post` / `request` 通过本地 `http.listen(..., {background:true})` 覆盖 response shape、JSON helper、404 和参数错误路径。
- `net_http_methods_more`: `net.put` / `patch` / `delete` / configurable `request` 覆盖 loopback method/body/header/timeout/followRedirects 行为。
- `csv_go_host_more`: `csv.parse` / `parseWithHeaders` / `encode` / `encodeWithHeaders` 覆盖 quoted fields、自定义分隔符、header 映射和 malformed input。
- `bits_go_host_more`: Go-style 64-bit `bits` helper 覆盖按位组合、shift/rotate、bit position 操作、count 和参数错误。
- `uuid_go_host_more`: `uuid` helper 覆盖 nil UUID、validation、parse metadata 和 generated UUID shape。
- `vec_color_go_host_more`: `vec` / `color` helper 覆盖 vector constructors、length/dot/cross/normalize/clamp，以及 RGB/hex/alpha/invert/lerp 和 invalid input。
- `vec_color_geometry_hsl_more`: vec geometry helpers 与 color HSL/HSV/lighten/darken/grayscale/mix/withAlpha transform 覆盖。
- `container_sort_go_host_more`: `container` / `sort` helper 覆盖 set/queue/stack/heap、ascending/descending/reverse、binary search、unique、partition 和 key sort。
- `sort_callback_helpers_more`: `sort.by` comparator 与 `sort.min` / `max` key callback 在 VM 文件模式下调用脚本 closure，并传播 callback error。
- `crypto_go_host_more`: `crypto` helper 覆盖 constant-time equality、AES-GCM round-trip、decrypt error returns、random byte/hex shape 和 key-size validation。
- `rand_go_host_more`: `rand` helper 覆盖 seeded replay、range/shape invariants、choice/shuffle/sample/weighted helpers、UUID/bytes shape 和参数校验。
- `log_go_host_more`: `log` helper 覆盖 level constants、format、deterministic log output、history/count/clear 和 level filtering。
- `math_go_helpers_more`: `math.clamp` / `lerp` / `sign` / `round` / `trunc` / `floorDiv` / `hypot` / `isnan` / `isinf` 的 Go-host helper 覆盖。
- `os_go_host_env_expand_more`: `os` helper 覆盖环境变量 get/set/unset、ExpandEnv 风格展开和参数错误。
- `os_time_date_host_more`: `os.time` / `clock` / fixed `date` formatting / hostname / pid / args 的 Go-host 诊断形状覆盖。
- `process_go_host_entry_exit_more`: `process` helper 覆盖 host-controlled args/entrypoint 与可捕获的 process exit error。
- `process_exec_run_more`: `process.which` / `pid` / `env` / `run` / `exec` / `shell` 覆盖命令查找、stdin/env、stdout/stderr、exit code 和错误路径。
- `main_script_file_vm_more`: `script` helper 在 VM 文件模式下覆盖当前 VM globals、相对文件加载、env sync 和 sandbox，不再退回 tree-walker chunk。
- `main_vm_loader_more`: 全局 `load` / `loadfile` / `dofile` / file-backed `require` 在 VM 文件模式下执行 bytecode chunk，共享当前 globals、相对 script dir 与 `package.loaded` cache。
- `regexp_submatch_limits_more`: `regexp.findAllSubmatch`、compiled regexp submatch helpers、find/split limit 和 `numSubexp` 覆盖。
- `io_current_stream_more`: `io.input` / `io.output` 的当前流切换覆盖 path 与 file handle 两种入口，并验证全局 `io.read` / `io.write` / `io.lines` 和 closed-file diagnostics。
- `http_router_json_edges_more`: `http.newRouter` method gating、response helper、handler error response，以及 `json.encode` 对 array/sparse/mixed table 与 NaN/Inf 的 Go-host 语义覆盖。
- `time_color_hash_edges_more`: 固定 UTC time boundary、color operator metamethod、SHA-512 和 hash 参数错误路径覆盖。
- `binary_numeric_fields_more`: `binary` / `string` pack/unpack 的 signed、unsigned、float、endian alias、token alias、offset 和 range error 覆盖。
- `base64_raw_url_edges_more`: base64 empty/binary round-trip、standard padding、raw URL no-padding contract 和 decode error 路径覆盖。
- `csv_options_quotes_more`: `csv.parse` 的 comment/lazyQuotes/trimSpace option 组合，以及 Go `encoding/csv` quoting 边界覆盖。
- `control_coroutine_defer_more`: cached coroutine function values、Go-style `defer` LIFO under return/protected errors，以及 `const` capture/shadowing 语义覆盖。
- `url_canonical_edges_more`: Go `net/url` canonical behavior 覆盖 IPv6 host/port、userinfo、percent encoding、duplicate query、invalid query 和 absolute join。
- `utf8_testkit_edges_more`: UTF-8 helper 边界与 `testkit` deterministic diagnostic/protect/functionInfo 形状覆盖。
- `time_process_debug_edges_more`: Go layout time parse/format、`process.run` string/dir option，以及 debug hook filtering/getHook/clear 行为覆盖。

当前能力状态与设计取舍：

本轮继续补 passing case 时，另观察到这些 Lua 兼容细节尚未纳入 passing
set。后续实现时按 GScript/Go-host 直觉设计，不要求逐字复刻 Lua：

- 普通多表达式赋值、表构造和调用参数中的多返回调整已有 passing 覆盖；
  固定参数目标函数会按 arity 预登记把嵌套多返回实参压成单值。需要主动展开
  非尾部多返回时，GScript 仍推荐显式 `spread(...)` / `table.spread(...)`。
- table library 已补 bounded proxy/metatable table 读写：`table.insert` /
  `table.sort` / `table.remove` / `table.unpack` / `table.move` 与
  `table.concat` 会通过 `__index` / `__newindex` / `__len` 交互；passing official 覆盖了
  `table.move` / `table.unpack` / `table.sort` / `table.insert` /
  `table.remove` / `table.concat`，并在 `GSCRIPT_OFFICIAL_CHECK_JIT=1`
  下通过。
- `table.keys` / `table.values` / `table.count` / `table.copy` /
  `table.merge` / `table.unique` / `table.reverse` / `table.slice` /
  `table.zip` 是 Go-host raw table helper：它们检查真实表内容，不走
  `__pairs` 或虚拟 `__index` 字段。需要用户语义上的 proxy 遍历时，使用
  `pairs`；需要 bounded array-style proxy 读写时，使用上面已 VM-aware 的 helper。
- 极值 sparse `table.unpack` 范围已补 GScript host 边界：一次
  `table.unpack` / `table.spread` 最多展开 1,000,000 个返回值，超过时立即报
  “too many results” 错误，不逐项扫描 sparse range。
当前没有 skipped known failures；此前明确记录的 VM `const`/`defer` parity、VM 文件模式 debug parity、VM table higher-order callback 和 table concat proxy 语义均已按 GScript/Go-host 语义补齐。后续继续翻译更大的官方切片时，如发现新能力缺口，再按本文件记录。

## JIT capability matrix

这些条目不是语言能力缺口。它们描述的是 native JIT 在 semantic-check 模式下主动退回 VM 的边界，或 methodjit 内部保留的 skipped 优化/正确性 repro。官方 translated correctness 仍由 VM 负责，`GSCRIPT_OFFICIAL_CHECK_JIT=1` 只要求 `gscript -jit` 输出一致，不要求每个 case 都 native 编译。

| 范围 | 语言/VM 状态 | JIT 当前策略 | 后续方向 |
|---|---|---|---|
| 多返回 ABI | VM 已覆盖普通赋值、调用、构造、vararg 和显式 `spread` | semantic gate 避免单 boxed return ABI 无法表达的 native 路径 | 设计 native 多返回 ABI 或统一 op-exit result buffer |
| top-level `<main>` | 文件模式副作用、错误和 source diagnostics 已由 VM 覆盖 | semantic gate 避免 native restart/fallback replay 可观察副作用 | 只有在 fallback 可证明不重放副作用后再放开 |
| `const` / `defer` / readonly control | 解释器和 VM compiler 已覆盖 | VM-only control，native 不负责 cleanup/control-state 合约 | 先保持 VM 执行，必要时设计 defer unwinding protocol |
| upvalue arithmetic | 闭包/upvalue 语言语义已覆盖 | upvalue + arithmetic 被 gate，避免 guard/boxing 不完整 | 补 upvalue numeric guard 与 deopt 恢复 |
| dynamic arithmetic/len/concat | metamethod、类型错误和 table proxy 语义已覆盖 | 动态 operators 被 gate，避免 native fallback 语义不完整 | 按 opcode 补完整 metamethod/error slow path |
| comparison branches | `__eq`/`__lt`/`__le` 与 raw equality 已覆盖 | comparison branches 被 gate | 补比较分支的 metamethod、error 和 deopt 合约 |
| call/coroutine boundaries | call、xpcall、yield/resume 等语言语义已覆盖 | call/self/generic-for/resume/yield 被 gate | 先补 native call boundary 的 error/yield/fallback 协议 |
| skipped Tier 2 quicksort / driver-loop tests | 不影响官方 translated VM/JIT output parity | methodjit 内部保留 skipped known bug/design-record tests | 作为 JIT correctness backlog，不写进 official `KNOWN_FAILURES.md` |
| whole-call no-result kernel | 语言侧无缺口 | disabled optimization，fallback contract 尚未证明 | 定义无返回 call op-exit 的恢复与副作用合约后再启用 |

## Covered GScript equivalents

| 能力 | 来源片段 | 状态 |
|---|---|---|
| `debug` 标准库基础信息查询 | `db.lua`: `debug.getinfo`, `debug.getlocal`, `debug.getupvalue` | 已补 GScript 风格 `debug.info`、`debug.stack`、`debug.globals`、`debug.value`，暴露函数 kind/name/参数数量/vararg/upvalue 数、运行时调用栈和 globals 快照；不复刻 Lua 局部变量槽位枚举。VM 文件模式已有 `debug.info(function)` / `debug.value` 和 frame/source 诊断官方 passing case。 |
| `debug.traceback` 与保护调用栈信息 | `db.lua` traceback checks, `errors.lua` line/stack-message checks | 已补 `debug.traceback([message])`，基于真实 script/native 调用栈生成稳定 GScript 格式；另提供 `debug.goStack()` 给 host 诊断 Go goroutine 栈。解释器与 VM 文件模式均带 source/name/line/column 元数据。 |
| 调试 hook | `db.lua`: `debug.sethook`, `debug.gethook` | 已补 GScript 事件式 `debug.setHook`/`debug.getHook`/`debug.emit`/`debug.setSink`，支持 script/native call/return/error 和显式 diagnostic emit 事件；不复刻 Lua line/count/coroutine hook。runtime/interpreter 与 VM 文件模式均有覆盖。 |
| 嵌套调用/构造中的多返回展开 | `db.lua`: transfer-value checks using `table.unpack`, returned varargs, and table constructors | 已补 GScript 风格显式展开：`spread(expr)` 可在调用参数/表构造任意位置展开 `expr` 的多返回；`table.spread(t [, i [, j]])` 可将表区间作为多返回显式展开。普通赋值、调用和构造保留 Lua 风格的尾部自动展开/非尾部单值调整；固定参数目标函数会按 arity 预登记把嵌套多返回实参压成单值。`table.values(t)` 维持原有“返回值数组表”API，不作为展开标记。Lua 兼容层后续可把需要的 `table.unpack` 非尾部展开场景翻译到显式 spread 形式。 |
| C API 测试库替代层 | `api.lua`: `T.testC`, `T.makeCfunc`, `T.checkmemory` | 已补 GScript/Go-host 风格 `testkit` 标准库：`memory`/`snapshot`/`diff`/`checkMemory` 提供内存诊断，`value`/`typeOf`/`equal` 提供值检查，`protect` 提供结构化保护调用结果，`functionInfo`/`sameFunction` 提供 native/script 函数身份和基础 introspection；不复刻 Lua 私有 `T.testC` 指令协议。 |
| `load`/`string.dump` 调试信息交互 | `db.lua`, `errors.lua` stripped chunk checks | 已通过 `script.compile`/`script.eval`/`script.loadFile`/全局 `load`/`loadfile` 的 sourceName（字符串第二参数或 `{sourceName: ...}` / `{source: ...}`）和 `SourceError` 保留源码名、行列与底层错误；不提供 Lua `string.dump`/stripped chunk 二进制语义，调试侧由 `debug.traceback`/`debug.info` 覆盖。 |
| 精细错误位置与 token 诊断 | `errors.lua`: syntax/runtime line checks and token-message checks | 已补 `SourceError` 包装 lexer/parser/runtime 错误，稳定输出 `source:line:column: error` 形态并支持 Go `errors.As` 解包；lexer/parser 保留字符或 token 类型/文本，错误文案保持 GScript 风格。 |
| `assert` 非字符串失败载荷 | `errors.lua`: `pcall(assert, false, t)` | 已支持：`assert(false, value)` 会把任意 GScript 值作为失败载荷交给 `pcall`/`xpcall`，不是字符串化后再传播。 |
| Label-directed control flow | `goto.lua`: labels, forward/backward jumps, repeated-label diagnostics, local-scope jumps | 已补 Go-style `label:` + `goto label`。标签为函数内唯一名称；允许向前/向后跳转和跳出嵌套 block/loop；禁止跳入更深 block 或跳过同一 block 内 local/function 声明。解释器和 VM 使用同一套静态校验。 |
| Local variable attributes | `constructs.lua`, `locals.lua`: `<const>`, `<close>` | 已补 Go-style `const name := expr` / `const name = expr` 只读绑定，以及 `defer call(...)` / `defer obj:method(...)` 资源清理能力；const 禁止重绑定但不冻结表内部状态，不复刻 Lua `<const>`/`<close>` 语法。runtime/interpreter 与 VM compiler 均有 focused tests，官方翻译集以 GScript 风格 case 覆盖 VM 文件模式。 |
| Lexical environment injection | `locals.lua`: `_ENV`, `load(..., env)`, environment upvalues | 已补 GScript 风格 `script.env(table)` / `script.sandbox(table)`，用于 `script.compile`/`script.eval`/`script.loadFile`/`script.runFile` 的显式环境注入；不复刻 Lua `_ENV` 隐式 upvalue 机制。 |
| GC finalizer tracing and runner-integrated progress output | `tracegc.lua`: table `__gc`, repeated remarking for finalization, `io.stderr` progress writes; `all.lua`: `require"tracegc".start()` | 已补 Go runtime 风格 `collectgarbage("stats")`，返回 alloc/sys/heapObjects/numGC/rootLog/running/mode 诊断表，供 runner 和测试观测 GC 状态；对象 finalizer 调度不按 Lua `__gc` 复刻，资源清理由 GScript `defer` 承担。 |
| UTF-8 strict/nonstrict validation helpers | `utf8.lua`: invalid byte sequences, non-strict decoding edge cases | 已补 `utf8.validate(s)` 结构化诊断和 `utf8.sanitize(s [, replacement])` 非严格清洗 API。严格路径继续由 `utf8.valid`/`utf8.len`/`utf8.codepoint`/`utf8.codes` 使用 Go `unicode/utf8` 规则；非严格路径显式 opt-in，不复刻 Lua 内部 nonstrict 参数形态。 |
| Binary string packing/unpacking | `tpack.lua`: `string.pack`, `string.unpack`, `string.packsize` | 已补 GScript/Go 风格 `binary.pack`/`binary.unpack`/`binary.size`，并在 `string.pack`/`string.unpack`/`string.packsize` 暴露同一套兼容入口；格式串仍采用显式 endian 和 Go-style 字段 token，不声明 Lua 格式串逐字兼容。 |
| Full file-handle and standard-stream IO controls | `files.lua`: `file:read`, `file:seek`, `file:flush`, `io.input`, `io.output`, `io.type`, `io.tmpfile`, close-file edge cases | 已补文件句柄 `read` 的 `"a"` 全量读取、`"l"` 去换行行读取、`"L"` 保留换行行读取、`"n"` 数字读取、`read(n)` 字节数读取、`read(0)` EOF probe 和一次多个格式项顺序多返回；同时已补 `seek`/`flush`、默认输入输出流 `io.input`/`io.output`、`io.type`、`io.tmpfile`、标准流表和关闭后状态。错误返回沿用文件操作 `nil, err`、参数错误抛运行时错误的现有风格。 |
| Process entrypoint and script-loading controls | `main.lua`, `attrib.lua`: command-line args, `-e`, script-level `dofile`, `loadfile`, `require`, process exit/status behavior | 已补 `Interpreter.SetArgs`、全局 `arg`、`process.args`、`process.entry`、host-controlled `process.exit`、基于脚本目录解析的 `dofile`/`loadfile`/`require` 路径逻辑，以及 `require("string")`/`require("math")` 等 builtin module alias 与 `package.loaded`。CLI 将 `-e`、REPL 和文件模式入口参数统一写入 runtime；当前不声明支持 Lua CLI `-l module` preload flag，脚本内预加载使用 `require`。 |
| Runtime source compilation and generated-program execution | `big.lua`, `verybig.lua`, `heavy.lua`, `code.lua`: generated chunks executed with `load` | 已补 GScript 风格 `script.compile`/`script.eval`/`script.loadFile`/`script.runFile`，并保留兼容全局 `load`/`loadfile` 入口；支持显式 env/sandbox、sourceName/source 与 scriptDir 配置，不复刻 Lua chunk dump 语义。 |
| Raw and multiline string literal ergonomics | `literals.lua`: long brackets, delimiter-level raw strings, escape suppression | 已支持 Go-style 反引号 raw string，可跨行且不解释反斜杠转义；不复刻 Lua `[=[...]=]` 分隔符层级。 |
| First-class integer bit operations in expressions | `bwcoercion.lua`, `code.lua`: direct bitwise expressions and coercion stress | 已支持 Go-style 表达式运算符 `&`、`|`、`^`、`&^`、`<<`、`>>` 和一元 `^`，并保留 `bits` 标准库的 rotate/bit-position/count 辅助；不复刻 Lua `~` 语法。 |

## Non-goals / compat-layer mappings

这些不是当前运行时缺口，除非项目后续决定提供 Lua 逐字兼容层：

- Lua `debug.getlocal` / `debug.getupvalue` / `debug.getinfo` 的局部槽位协议；GScript 使用 `debug.info`、`debug.stack`、`debug.globals`、`debug.value`。
- Lua line/count/coroutine debug hooks；GScript 使用事件式 `debug.setHook`、`debug.emit`、`debug.setSink`。
- Lua 私有 `T.testC` API 测试协议；GScript 使用 `testkit`。
- Lua `string.dump` / stripped binary chunk；GScript 使用 source-level `script.compile` / `script.eval` 与 source diagnostics。
- Lua `<const>` / `<close>` 语法；GScript 使用 Go-style `const` 与 `defer`。
- Lua `_ENV` 隐式 upvalue；GScript 使用显式 `script.env` / `script.sandbox`。
- Lua table `__gc` finalizer；GScript 资源清理由 `defer` 表达，GC 观测用 `collectgarbage("stats")`。
- Lua long-bracket delimiter 语法和 Lua `~` bitwise 语法；GScript 使用 Go-style raw string 与 bitwise operators。
