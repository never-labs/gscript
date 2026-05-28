# 标准库与宿主能力生产路线

本文记录 GScript 标准库从“可用能力集合”走向“生产级宿主能力层”的路线。范围以 `internal/runtime/stdlib*.go` 中已注册模块为准，并参考官方 Lua 翻译能力 ledger 中的现状记录。目标不是逐字复刻 Lua 5.4 标准库，而是在兼容 Lua 用户直觉的同时，提供 Go 生态中更可预测、更容易测试、更适合嵌入宿主的 API。

## 总体原则

- 模块必须可以通过全局名与 `require(name)` 取得同一张表，并同步进入 `package.loaded`。新增模块先进入注册表，再补官方层 require 身份测试。
- Go 标准库已有清晰模型时优先 Go-style：`json`、`csv`、`regexp`、`path`、`time`、`http`、`process` 等保持 Go 行为、Go 错误边界和结构化返回。
- Lua 兼容只补用户迁移高频路径：`string`、`table`、`math`、`os`、`io` 保留 Lua 命名和多返回习惯；不承诺调试槽位、二进制 chunk、`_ENV`、`<close>`、line/count hook 等内部协议。
- I/O、网络、进程、环境变量、时间、随机数和密码学属于宿主能力，必须能被 embedder 限权。生产化前应形成统一 capability policy，而不是让脚本默认拥有完整 OS 权限。
- 错误模型分层：参数错误抛 runtime error；外部资源失败返回 `nil, err` 或结果表 `{ok, stdout, stderr, code}`；安全敏感 API 不吞错、不猜测修复。
- 表数据转换要稳定：array/object、稀疏表、混合 key、NaN/Inf、二进制字符串和 UTF-8 文本都要有文档化行为。
- 所有 host API 需要同时覆盖解释器、VM 文件模式、`require` 入口和 JIT parity gate。JIT 可以退回 VM，但输出和副作用不能变化。

## 能力矩阵

| 分类 | 现状 | 主要缺口 | 路线 |
|---|---|---|---|
| `json` | 已有 `encode`、`decode`、`valid`、`pretty`/`indent`，使用 Go `encoding/json`，拒绝 trailing data，表按 array/object 转换，NaN/Inf 编码为 `null`。 | 缺少 streaming decoder/encoder、query/path、schema 或字段约束、稳定 key 排序选项、精确大整数策略和循环表诊断。 | 固化 table 映射规则；增加 `{sortKeys, escapeHTML, number="int|float|string"}` 选项；补 query/path 读取 helper；循环检测返回明确错误；大 payload 增加 reader/writer 形态。 |
| `csv` | 已有 parse/encode、header 版本和 `sep`、`comment`、`trimSpace`、`lazyQuotes` 选项，基于 Go `encoding/csv`。 | 缺少 streaming、字段数策略、BOM/换行标准化、header 重名策略和巨大文件内存上限。 | 增加 row iterator；文档化 header 覆盖/保留策略；所有 malformed input 保持可断言错误；大文件测试覆盖内存和 partial read。 |
| `http` | 已有 `listen`、`newRouter`、`get`，router 支持 method handler，server handle 支持 background、close、shutdown、wait；request/response 有 query、headers、body、json、status、header、redirect。 | client 侧过薄，缺少统一 `request`、timeout、headers/body、TLS/control、redirect 策略；server 缺少 middleware、path param、context cancellation、body size limit。 | 将 `net.request` 风格并入或桥接到 `http.request`；server 所有 listener 必须可关闭且测试无泄漏；新增 `{timeout, maxBody, headers, method}`；handler panic/error 形状稳定。 |
| `time` | 已有 `now`、`sleep`、`since`、`unix`、`format`、`parse`、`add`、`diff`、`isBefore`、`isAfter`、`weekday`、`month` 和 duration 常量；支持 Go layout 与部分 strftime 转换。 | sleep 不可取消；timezone/location 能力有限；单调时钟与 wall clock 未区分；parse/format layout 文档不足。 | 引入 context-aware sleep/timer；明确 time table 字段不变式；补 location/UTC/local；测试固定 UTC 边界、DST、nsec round-trip 和 layout 错误。 |
| `regexp` | 已有 Go RE2 风格 compile/mustCompile/match/find/findAll/submatch/replace/split/numSubexp，支持 compiled object 与 cache。 | 不兼容 Lua pattern；缺少 named capture、byte/rune index 说明、replace callback、cache 上限。 | 坚持 RE2 作为 `regexp`；Lua pattern 留在 `string`；新增 named group table 和 callback replace；文档化所有 index 按字节还是字符。 |
| `bytes` | 已有 buffer、fromString/fromHex/toHex/xor/compare/repeat/concat，buffer 支持多种 little-endian numeric write/read、hex、len、reset。 | 缺少 endian 参数、read numeric 系列完整性、切片零拷贝语义、base64/hash/crypto 之间的二进制约定。 | 定义“GScript string 可承载 bytes”；补 endian option 和 bounds error；避免默认 UTF-8 假设；与 `binary` 共享字段 token。 |
| `os` | 已有 `time`、`clock`、`date`、`exit`、`getenv`、`setenv`、`unsetenv`、`expand`、`remove`、`rename`、`tmpname`、`args`、`hostname`、`getpid`。 | `os.exit` 直接退出进程，不适合 embedded production；文件操作与 `fs` 边界重叠；环境变量修改缺少 sandbox policy。 | 推荐生产入口使用 `process.exit` 的可捕获错误；把危险 OS API 纳入 capability policy；文档化 `os` 是 Lua 兼容表，`process`/`fs` 是 Go-host 表。 |
| `process` | 已有 `run`、`exec`、`shell`、`which`、`pid`、`env`、`args`、`entry`、`setArgs`、`exit`；`run` 支持 stdin/env/dir/timeout 和结构化结果。 | shell 注入风险、Windows shell 差异、streaming stdout/stderr、取消传播、环境白名单和 resource limit。 | `run(tableArgs)` 作为生产推荐；`shell` 标为显式危险能力；增加 context cancellation、max output、stream callback；测试 timeout kill、cwd、env merge 和 non-zero exit。 |
| `crypto` | 已有 secure random bytes/hex、AES-GCM encrypt/decrypt、generateKey、constant-time equal。 | 缺少 AEAD associated data、nonce 策略可控性、KDF、签名、hash/HMAC 与 `hash` 模块边界说明。 | 保持高层安全默认；新增 `{aad, nonce}` 但默认随机 nonce；补 HKDF/PBKDF2/Ed25519 时要求 test vectors；禁止弱算法进入默认命名空间。 |
| `path` | 已有 separator/listSeparator、join/dir/base/ext/abs/isAbs/clean/split/match/rel，基于 `filepath`。 | OS-specific 行为会影响跨平台测试；缺少 URL path 与 filepath 区分；glob 在 `fs` 而不在 `path`。 | 明确 `path` 是 host filepath；URL 使用 `url`；跨平台测试只断言不变量，平台具体 separator 用 golden per OS。 |
| `context` | 目前没有脚本层 `context` 模块；Go 内部在 `http`/`process.run` timeout 中使用 context。 | 无统一取消、deadline、signal、request-scoped value；`sleep`、HTTP client/server、process、goroutine/channel 无共同取消模型。 | 新增 `context` 模块作为宿主能力骨架：`background`、`withCancel`、`withTimeout`、`done`、`err`、`cancel`；先接入 `process.run`、`http.request`、`time.sleep`，再扩展并发 API。 |
| `container` | 已有 set、queue/deque、stack、heap，使用表或 Go heap 包装。 | set/queue 当前是可变表协议，内部字段可被脚本破坏；heap comparator/priority 类型约束有限；迭代顺序未定义。 | 生产对象应隐藏内部状态或加 metatable guard；为 set/queue/stack/heap 提供 method 风格别名；所有结构文档化复杂度和 mutation 行为。 |
| `math` | 保留 Lua math，并补 Go helper：clamp、lerp、sign、round、trunc、floorDiv、hypot、isnan、isinf 等；常量含 pi/huge/mininteger/maxinteger。 | random 与 `rand` 重叠；整数/浮点边界、NaN 传播、溢出策略需统一；缺少更多 Go `math/bits` 联动说明。 | `math` 负责数值函数，确定性随机迁到 `rand`；所有边界补 property tests；JIT fast path 与 VM 结果保持 bit-level 可解释。 |
| `string` | 保留 Lua `byte/char/find/format/gmatch/gsub/len/lower/match/pack/packsize/rep/sub/upper`，并补 split/trim/replaceAll/join/title/pad/isNumeric/hasPrefix/hasSuffix 等 Go helper。 | Lua pattern 与 Go regexp 两套语义易混；Unicode 与 byte index 边界需更清楚；format/pack 兼容面仍需 ledger 驱动。 | `string` 维持 Lua-facing 名称，Go 正则只进 `regexp`；文档化 byte-oriented API 与 `utf8` API；官方 Lua case 发现缺口时按 GScript 语义补兼容层。 |
| `table` | 已有 sort/insert/remove/unpack/spread/move/concat、keys/values/contains/indexOf/copy/merge/count/unique/reverse/slice/zip/flatten/toArray，以及 map/filter/reduce/fromArray；部分 helper 通过 VM callback 和 proxy/metatable。 | raw helper 与 proxy-aware helper 边界需要文档化；大范围 unpack 已有限制但用户指南不足；higher-order callback error/coroutine 交互需继续压测。 | 把 API 分成 Lua-compatible、raw Go helper、callback helper 三组；新增稳定顺序选项时必须显式；继续覆盖 sparse/proxy/metamethod/JIT parity。 |
| `debug` | 已有 traceback/stack/globals/info/value/goStack、事件式 setHook/getHook/emit/setSink；不复刻 Lua local/upvalue 槽位协议。 | 生产环境可能泄露 globals、source path、Go stack；hook/sink 需要并发和重入策略；缺少权限控制。 | `debug` 默认视作 privileged capability；嵌入场景允许禁用 goStack/globals/value；事件结构版本化；测试 hook ordering、filter、error propagation。 |

## Lua 兼容与 Go-style 取舍

`string`、`table`、`math`、`os` 是 Lua 用户最容易直接触达的兼容层。这里优先保留函数名、多返回、1-based 下标和常见错误形状，但实现可以采用 GScript/Go 运行时模型：显式 `spread` 解决非尾部展开，`defer` 代替 `<close>`，`debug.info`/`debug.stack` 代替 Lua 调试槽位协议。

`json`、`csv`、`http`、`regexp`、`bytes`、`crypto`、`path`、`process`、未来 `context` 是 Go-host 能力层。这里优先采用 Go 标准库术语和结构化 option table，错误边界以可测试、可限权、可嵌入为准。不要为了 Lua 逐字兼容牺牲 Go 宿主可预测性。

同名能力的分工应保持清楚：`os` 提供 Lua 风格薄包装，`process` 提供可捕获、可测试的进程控制；`path` 是 host filepath，`url` 是 URL；`string` 处理 Lua pattern/byte string，`regexp` 处理 RE2；`math.random` 是兼容入口，`rand` 是确定性和扩展随机工具。

## `path` 模块

`path` 模块映射 Go `path/filepath`，处理宿主操作系统文件路径，不处理 URL path。URL 拆分和拼接应使用 `url` 模块；文件枚举 glob 属于 `fs` 模块。

| API | 返回 | 说明 |
|---|---|---|
| `path.separator` | string | 宿主路径分隔符，等价于 Go `os.PathSeparator`。 |
| `path.listSeparator` | string | 宿主 PATH 列表分隔符，等价于 Go `os.PathListSeparator`。 |
| `path.clean(p)` | string | 清理 `.`、`..` 和重复分隔符，等价于 `filepath.Clean`。 |
| `path.join(...)` | string | 拼接并清理路径片段，等价于 `filepath.Join`；无参数返回空字符串。 |
| `path.base(p)` | string | 返回路径最后一个元素，等价于 `filepath.Base`。 |
| `path.dir(p)` | string | 返回路径目录部分，等价于 `filepath.Dir`。 |
| `path.ext(p)` | string | 返回最后一个路径元素的扩展名，等价于 `filepath.Ext`。 |
| `path.isAbs(p)` / `path.isabs(p)` | bool | 判断路径是否为宿主绝对路径；`isabs` 是兼容别名。 |
| `path.abs(p)` | string 或 `nil, err` | 返回绝对路径，外部失败时返回 `nil, err`。 |
| `path.rel(base, target)` | string 或 `nil, err` | 返回 `target` 相对 `base` 的路径，不能构造时返回 `nil, err`。 |
| `path.split(p)` | `dir, file` | 拆成目录和文件名，等价于 `filepath.Split`；目录保留尾部分隔符。 |
| `path.match(pattern, name)` | bool 或 `false, err` | 使用 `filepath.Match` 语法匹配单个路径名；pattern 语法错误时返回 `false, err`。 |

## 宿主能力与权限

生产级标准库需要一层统一的 capability policy：

- `LibFlags` 只控制标准库表是否出现在全局环境、是否能通过内建
  `require(name)` 取得；它不代表脚本拥有对应宿主资源权限。
- `CapabilityFlags` 控制已经可见的脚本 API 是否能触达宿主效果。当前
  公共能力位是 `CapModuleLoading`、`CapFilesystemRead` 和
  `CapFilesystemWrite`：前者控制 `require` 是否能从宿主文件系统加载
  `.gs` 文件；`CapFilesystemRead` 控制 `fs.readfile`、`fs.stat`、
  `fs.readdir`、`dofile`、`loadfile` 等读取入口；`CapFilesystemWrite`
  控制 `fs.writefile`、`fs.remove`、`fs.rename`、`fs.mkdir`、`fs.chdir`、
  `fs.tempfile` 等变更入口。`CapFilesystem` 保留为兼容别名，等价于
  `CapFilesystemRead | CapFilesystemWrite`。
- `WithSandbox()` 等价于选择 `LibSafe` 并关闭宿主文件系统能力
  (`CapSafe`)。因此安全内建模块仍可 `require("json")`，但文件模块
  `require("helper")`、`fs`、`dofile`、`loadfile` 默认不可用。
- `WithFilesystemRoot(root)` 会启用文件系统能力并把脚本侧路径限制在
  `root` 内；它当前授予 `CapFilesystem`，也就是同时授予读写能力。
  相对路径在 root 内解析，清理后的绝对路径不得等于 root 外部位置。
  当前 root confinement 主要防 `..`/绝对路径逃逸，符号链接解析、
  独立读写根、特殊文件拒绝和大小限制仍是生产路线项目。
- `WithFilesystemRead(false)` 可在保留写能力时禁用读取入口；
  `WithFilesystemWrite(false)` 可在保留读能力时禁用变更入口；
  `WithFilesystem(false)` 同时清除读写能力并移除 `fs`、`dofile`、
  `loadfile`。
- Options 按传入顺序应用；要组合 root confinement 和只读/只写能力，应先传
  `WithFilesystemRoot(root)`，再传 `WithFilesystemWrite(false)` 或
  `WithFilesystemRead(false)`。
- `WithModuleLoading(false)` 只关闭文件系统 `.gs` 模块加载；启用的内建
  标准库模块仍可通过 `require` 获得。
- `fs`、`os.remove`、`os.rename`、`process.run`、`process.shell`、`http.listen`、`net/http client`、`debug.goStack`、`debug.globals` 应可按 interpreter 禁用或限制。
- 默认 CLI 可以保持当前完整能力；embedded runtime 应允许最小权限启动，再按模块或函数授予。
- 所有后台 server、subprocess、timer、goroutine/channel 都必须有可关闭句柄或可取消 context，测试结束不得留下宿主资源。
- 错误和诊断不能泄露超出权限的路径、环境变量或源码内容；debug 能力需要单独开关。

## 测试策略

- 每个模块保留 runtime 单测，覆盖参数错误、外部失败、边界值、并发/关闭和资源释放。
- 每个对脚本可见的生产能力都要有 official translated 或 GScript official case，覆盖 VM 文件模式与 `require` 身份。
- 对 table/string/math 这类 JIT 敏感 API，新增 case 要在 `GSCRIPT_OFFICIAL_CHECK_JIT=1` 下验证输出一致；允许 JIT 退回 VM，但不能改变副作用。
- 对 Go 标准库映射使用 golden + property test：JSON round-trip、CSV malformed、regexp submatch、path clean/rel、time parse/format、crypto test vectors。
- 对 host API 使用本地 loopback 和临时目录：HTTP server 必须 background 启动并 close/shutdown；process 必须覆盖 timeout、stdin、env、cwd、exit code；文件和路径测试必须恢复 cwd/env。
- 对安全能力增加 negative tests：bad key size、bad ciphertext、shell disabled、debug disabled、path outside sandbox、context cancellation。
- 对跨平台行为避免硬编码 separator、hostname、pid、clock、locale；只断言 shape、不变量和可控输入输出。

## 分阶段路线

1. 基线冻结：为现有 stdlib 生成 API 快照，补齐 `require` 身份、错误 shape 和文档示例，禁止无测试新增导出函数。
2. 权限层：实现 interpreter-level capability policy，先覆盖 process/os/fs/http/debug，再给 CLI 和 embedded runtime 不同默认配置。
3. 取消模型：新增 `context` 模块，并把 process/http/time 的 timeout、shutdown、sleep 迁到统一 cancellation contract。
4. 数据边界：完善 json/csv/bytes/binary/crypto 的大 payload、streaming、二进制字符串和数字精度策略。
5. 兼容收敛：继续用 `tests/official_lua_cases/MISSING_CAPABILITIES.md` 记录 Lua 官方 case 发现的新缺口，只补高价值兼容层，不追逐 Lua 内部实现协议。
6. 生产审计：为每个 host 能力补资源泄漏测试、race 测试、sandbox negative tests 和平台矩阵，形成发布前 checklist。
