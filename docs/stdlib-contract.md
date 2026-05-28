# Standard Library Contract Index

This file is the machine-checkable index for the current GScript standard
library contract. The module list is intentionally mirrored from
`internal/runtime/stdlib.go` and each row records the externally visible risk
and optimization policy for that module.

Safety levels:

- `Pure`: deterministic in-process computation with no host I/O or ambient
  process access.
- `Bounded host`: reads host state or uses bounded host services, but should
  not create persistent external effects by default.
- `Privileged host`: can mutate host state, open listeners, spawn processes, or
  expose sensitive diagnostics; embedders should gate it with capability policy.
- `Test-only`: intended for conformance diagnostics, not production scripts.

Error models:

- `runtime error`: bad arguments or invalid script-visible usage raise a
  runtime error.
- `nil, err`: external failures are returned as a nil/false result plus a
  string error.
- `result table`: operation status is reported in a structured table.
- `sentinel`: unavailable or empty state returns nil/false/empty data without
  treating it as an error.

JIT fast path policy:

- `intrinsic`: method JIT may lower selected calls to IR/native operations.
- `native identity`: VM/JIT may guard the exact stdlib Go function identity and
  use specialized call paths.
- `runtime specialization`: whole-loop or host-driver recognizers may bypass
  generic bytecode after guarding stdlib identity.
- `VM fallback`: calls must remain semantically identical when JIT declines or
  exits to the VM.

## Module Index

| Module | Safety | Host capability | Error model | JIT fast path |
|---|---|---|---|---|
| `base64` | Pure | none; string/byte encoding only | runtime error for bad argument shape; `nil, err` for malformed decode | VM fallback |
| `binary` | Pure | none; binary pack/unpack over strings | runtime error for bad argument shape; `nil, err` for malformed fields or bounds | VM fallback |
| `bit32` | Pure | none; 32-bit integer bit operations | runtime error | native identity for selected bit ops; VM fallback otherwise |
| `bits` | Pure | none; Go `math/bits` style helpers | runtime error | VM fallback |
| `bytes` | Pure | none; byte-string buffers and transforms | runtime error for bad argument shape; `nil, err` for malformed hex/bounds | VM fallback |
| `color` | Pure | none; color conversion and geometry helpers | runtime error | VM fallback |
| `compress` | Pure | CPU/memory only through compression codecs | runtime error for bad argument shape; `nil, err` for malformed compressed data | VM fallback |
| `container` | Pure | in-process set/queue/deque/stack/heap objects | runtime error; sentinel nil/false for empty pop/peek/lookup | VM callback and VM fallback |
| `crypto` | Bounded host | cryptographic random source and AEAD primitives | runtime error for invalid arguments; `nil, err` for invalid keys/ciphertext | VM fallback |
| `csv` | Pure | none; CSV parser/encoder over strings/tables | runtime error for bad options; `nil, err` for malformed input | VM fallback |
| `debug` | Privileged host | stack, globals, Go stack, hook/sink diagnostics | runtime error; result tables for info/stack/value | VM fallback |
| `encoding` | Pure | none; encoding conversion helpers | runtime error; `nil, err` for malformed input | VM fallback |
| `fs` | Privileged host | filesystem read/write/remove/glob/cwd operations | runtime error for bad arguments; `nil, err` for OS failures | VM fallback |
| `hash` | Pure | none; hash digests over strings/bytes | runtime error | VM fallback |
| `http` | Privileged host | HTTP client/server, listener lifecycle, request/response I/O | runtime error for bad arguments; `nil, err` or result table for network/server failures | runtime specialization for stdlib host driver cases; VM fallback |
| `io` | Privileged host | process stdio and file handles | runtime error for bad arguments; `nil, err` for file/stream failures; sentinel for EOF | VM fallback |
| `json` | Pure | none; JSON codec over strings/tables | runtime error for bad arguments; `nil, err` for malformed JSON or unsupported values | runtime specialization for stdlib host driver cases; VM fallback |
| `log` | Bounded host | in-process log sink and log level state | runtime error; sentinel empty data for no records | VM fallback |
| `math` | Pure | none; numeric functions and constants | runtime error | intrinsic for `sqrt`/`floor`; native identity/FastArg paths for selected functions |
| `matrix` | Pure | in-process dense matrix allocation and numeric kernels | runtime error | native matrix/table fast paths where recognized; VM fallback |
| `net` | Privileged host | network client/server helpers | runtime error for bad arguments; `nil, err` or result table for network failures | runtime specialization for stdlib host driver cases; VM fallback |
| `os` | Privileged host | environment, args, pid/host, file remove/rename, process exit | runtime error for bad arguments; `nil, err` or sentinel for host failures depending on Lua-compatible API | VM fallback |
| `path` | Bounded host | host filepath rules and current platform separators | runtime error for bad arguments; `nil, err` for invalid rel/match cases | VM fallback |
| `process` | Privileged host | subprocess execution, shell, env, cwd, args, exit | runtime error for bad arguments; result table for run/exec; `nil, err` for lookup/setup failures | runtime specialization for stdlib host driver cases; VM fallback |
| `rand` | Bounded host | PRNG state and random byte generation | runtime error for invalid ranges/options | VM fallback |
| `regexp` | Pure | none; Go RE2 compile/match/replace/split | runtime error for bad arguments; `nil, err` for invalid patterns in non-must APIs | runtime specialization for regexp hot driver cases; VM fallback |
| `rl` | Privileged host | optional raylib window, drawing, input, audio; default build is a stub | runtime error for invalid calls; stub-safe sentinel behavior when bindings are unavailable | VM fallback |
| `script` | Privileged host | script path, loader, and entry metadata | runtime error; sentinel nil/empty for unavailable metadata | VM fallback |
| `sort` | Pure | in-process sort helpers with optional script callbacks | runtime error, including invalid comparator/order cases | runtime specialization for numeric sort cases; VM callback and VM fallback |
| `string` | Pure | none; Lua-style byte string and pattern helpers plus Go-style helpers | runtime error | native identity for `format`, `sub`, `split`, `find`, `match`, `gsub`; VM fallback otherwise |
| `table` | Pure | in-process table mutation, iteration, raw/proxy helpers, callback helpers | runtime error; sentinel nil/empty for absent values | native identity for raw helpers/iterators; VM callback and VM fallback |
| `testkit` | Test-only | runtime diagnostics for translated conformance tests | runtime error; diagnostic result tables | VM fallback |
| `time` | Bounded host | wall clock, sleep, formatting/parsing, duration constants | runtime error for bad arguments; `nil, err` for parse failures | VM fallback |
| `url` | Pure | none; URL parse/escape/query helpers | runtime error for bad arguments; `nil, err` for malformed URLs | VM fallback |
| `utf8` | Pure | none; UTF-8 validation/codepoint helpers | runtime error; sentinel nil/false for invalid positions/sequences where API defines it | native identity for selected codepoint/codes helpers; runtime specialization for math/bit/UTF-8 loops |
| `uuid` | Bounded host | random UUID generation and UUID parsing | runtime error for bad arguments; `nil, err` for malformed UUID input | VM fallback |
| `vec` | Pure | none; vector geometry and numeric helpers | runtime error | VM fallback |

## Contract Rules

- Every module above must be obtainable both as a global and through
  `require(name)`, with the same table identity recorded in `package.loaded`.
- Pure modules must not perform ambient host I/O. Bounded and privileged modules
  must remain candidates for future interpreter-level capability policy.
- Argument and programmer errors raise runtime errors. External host/resource
  failures should be returned as `nil, err` or a documented result table unless
  Lua-compatible behavior requires a sentinel.
- JIT and runtime specializations are optional accelerators. They must guard the
  exact stdlib identity they depend on and must preserve VM-visible results and
  side effects when disabled, declined, or deoptimized.
