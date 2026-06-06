# Leia Standard Library Inventory

Generated from `internal/stdlib/catalog`, the code-owned standard-library metadata.

| Layer | Module | Description | Safe default | Capabilities |
|---|---|---|---|---|
| `base` | `base64` | Base64 and URL-safe base64 encode/decode helpers. | true | none |
| `base` | `bits` | Go math/bits-style integer bit counting and rotation helpers. | true | none |
| `base` | `bytes` | Byte-string transforms, buffers, hex helpers, and byte comparisons. | true | none |
| `base` | `color` | RGBA colors, color-space conversion, interpolation, and common constants. | true | none |
| `base` | `compress` | Compression and decompression helpers over strings. | true | none |
| `base` | `container` | Sets, queues, stacks, deques, and heaps implemented in process. | true | none |
| `base` | `context` | Cancellation, timeout, and done-channel helpers. | true | none |
| `base` | `crypto` | Secure random data and high-level cryptographic primitives. | true | none |
| `base` | `encoding` | Text and byte encoding conversion helpers. | true | none |
| `base` | `hash` | Hash digest helpers over strings and byte data. | true | none |
| `base` | `json` | JSON encode/decode/validate helpers over Leia values. | true | none |
| `base` | `math` | Numeric functions, constants, and Lua-compatible math helpers. | true | none |
| `base` | `path` | Host filepath join, clean, split, match, rel, and separator helpers. | true | none |
| `base` | `rand` | Pseudo-random values, ranges, bytes, and seedable random helpers. | true | none |
| `base` | `regexp` | Go RE2 regular expression compile, match, find, replace, and split helpers. | true | none |
| `base` | `sort` | Sort helpers for arrays, numbers, tables, and callback-based ordering. | true | none |
| `base` | `string` | Lua-style byte-string helpers plus Go-style string utilities. | true | none |
| `base` | `sync` | In-process waitgroup, mutex, rwmutex, and once primitives. | true | none |
| `base` | `table` | Lua-compatible table helpers plus higher-order table utilities. | true | none |
| `base` | `time` | Wall-clock, duration, sleep, parse, format, and timeout helpers. | true | none |
| `base` | `url` | URL parse, escape, query, and construction helpers. | true | none |
| `base` | `utf8` | UTF-8 validation, codepoint, length, and offset helpers. | true | none |
| `base` | `uuid` | UUID generation, parsing, validation, and metadata helpers. | true | none |
| `host` | `db` | Built-in SQLite database runtime for parameterized SQL and columnar query frames. | false | db.open, db.exec, db.query, db.one, db.frame |
| `host` | `debug` | Runtime stack, globals, hook, and diagnostic helpers. | false | debug |
| `host` | `fs` | Filesystem read, write, stat, directory, glob, and path-affecting operations. | false | fs.read, fs.write |
| `host` | `http` | HTTP client/server helpers and request/response adaptation. | false | net.listen |
| `host` | `io` | File handles, process stdio, and stream helpers. | false | io |
| `host` | `log` | In-process log sink, levels, and log records. | false | io.write |
| `host` | `net` | Network helpers and HTTP-facing host integration. | false | net.http |
| `host` | `os` | Environment, process metadata, temporary names, and Lua-style OS helpers. | false | env.read, env.write |
| `host` | `process` | Subprocess execution, shell execution, lookup, args, and entry metadata. | false | process.exec, process.shell |
| `host` | `script` | Script compilation, evaluation, loader, source, and entrypoint helpers. | false | script.eval, module.load |
| `host` | `serve` | High-level HTTP application router with parameter routes and automatic responses. | false | net.listen |
| `host` | `testkit` | Conformance and diagnostic helpers intended for tests. | false | testkit |
| `llm` | `chat` | Lightweight chat and conversation helpers for AI-native scripts. | false | llm.turn |
| `llm` | `history` | Conversation history search, append, and recall helpers. | false | none |
| `llm` | `llm` | Model turns, tools, validation, record/replay, and provider-backed AI calls. | false | llm.turn |
| `llm` | `loop` | Reusable AI agent loop drivers such as react and plan/execute. | false | llm.turn |
| `llm` | `msg` | Normalized LLM message constructors for system, user, assistant, and tool roles. | false | none |
| `data` | `array` | Dense typed arrays and conversion helpers for hot data loops. | true | none |
| `data` | `binary` | Binary pack/unpack over Leia strings using declarative field formats. | true | none |
| `data` | `csv` | CSV parse and encode helpers backed by Go's CSV behavior. | true | none |
| `data` | `matrix` | Dense matrix values and numeric matrix helpers. | true | none |
| `data` | `q` | Table-driven column queries over SoA values for high-level data analysis. | true | none |
| `data` | `soa` | Structure-of-arrays records and column-oriented data processing. | true | none |
| `data` | `vec` | Vector construction, arithmetic, geometry, and numeric helpers. | true | none |
| `compat` | `bit32` | Lua-compatible 32-bit bit operations. | true | none |
