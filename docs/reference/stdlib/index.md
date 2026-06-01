# Leia Standard Library

This page is the machine-checkable standard-library contract index. It is
organized from the runtime catalog in `internal/stdlib/catalog`; future detailed
function pages should be generated from module-owned comments and metadata.

Safety levels:

- `Pure`: deterministic in-process computation with no host I/O.
- `Bounded host`: uses bounded host services or ambient state.
- `Privileged host`: can mutate host state, open listeners, spawn processes, or
  expose sensitive diagnostics.
- `Test-only`: intended for conformance diagnostics.

| Module | Safety | Host capability | Error model | JIT fast path |
|---|---|---|---|---|
| `array` | Pure | none | runtime error for bad argument shape | VM fallback; runtime specialization candidate |
| `base64` | Pure | none | runtime error; `nil, err` for malformed decode | VM fallback |
| `binary` | Pure | none | runtime error; `nil, err` for malformed fields or bounds | VM fallback |
| `bit32` | Pure | none | runtime error | native identity for selected bit ops; VM fallback |
| `bits` | Pure | none | runtime error | VM fallback |
| `bytes` | Pure | none | runtime error; `nil, err` for malformed hex/bounds | VM fallback |
| `chat` | Bounded host | `llm.turn` through installed provider when used with model calls | runtime error; provider failures as structured errors | VM fallback |
| `color` | Pure | none | runtime error | VM fallback |
| `compress` | Pure | CPU/memory only | runtime error; `nil, err` for malformed compressed data | VM fallback |
| `container` | Pure | none | runtime error; sentinel nil/false for empty pop/peek/lookup | VM callback and VM fallback |
| `context` | Bounded host | cancellation and timeout state | runtime error; sentinel nil for uncancelled state | VM fallback |
| `crypto` | Bounded host | random source and cryptographic primitives | runtime error; `nil, err` for invalid keys/ciphertext | VM fallback |
| `csv` | Pure | none | runtime error; `nil, err` for malformed input | VM fallback |
| `debug` | Privileged host | `debug` | runtime error; result tables for stack/value data | VM fallback |
| `encoding` | Pure | none | runtime error; `nil, err` for malformed input | VM fallback |
| `fs` | Privileged host | `fs.read`, `fs.write` | runtime error; `nil, err` for OS failures | VM fallback |
| `hash` | Pure | none | runtime error | VM fallback |
| `history` | Pure | none | runtime error | VM fallback |
| `http` | Privileged host | `net.listen`, network client/server capability | runtime error; `nil, err` or result table for network failures | runtime specialization for guarded host drivers; VM fallback |
| `io` | Privileged host | `io`, filesystem-backed handles where enabled | runtime error; `nil, err`; sentinel for EOF | VM fallback |
| `json` | Pure | none | runtime error; `nil, err` for malformed JSON or unsupported values | runtime specialization for guarded hot paths; VM fallback |
| `llm` | Bounded host | `llm.turn` | runtime error; `(nil, err-table)` for provider/validation failures | VM callback and VM fallback |
| `log` | Bounded host | `io.write` | runtime error; sentinel empty data for no records | VM fallback |
| `loop` | Bounded host | `llm.turn` | runtime error; `(nil, err-table)` for provider/validation failures | VM callback and VM fallback |
| `math` | Pure | none | runtime error | intrinsic/native identity for selected functions |
| `matrix` | Pure | none | runtime error | guarded matrix/table fast paths; VM fallback |
| `msg` | Pure | none | runtime error | VM fallback |
| `net` | Privileged host | `net.http` | runtime error; `nil, err` or result table for network failures | runtime specialization for guarded host drivers; VM fallback |
| `os` | Privileged host | `env.read`, `env.write`, process metadata and selected file operations | runtime error; `nil, err` or sentinel depending on API | VM fallback |
| `path` | Bounded host | host filepath rules | runtime error; `nil, err` for invalid rel/match cases | VM fallback |
| `process` | Privileged host | `process.exec`, `process.shell` | runtime error; result tables; `nil, err` for setup failures | runtime specialization for guarded host drivers; VM fallback |
| `rand` | Bounded host | PRNG state and random bytes | runtime error | VM fallback |
| `regexp` | Pure | none | runtime error; `nil, err` for invalid patterns in non-must APIs | runtime specialization for hot regexp paths; VM fallback |
| `script` | Privileged host | `script.eval`, `module.load` | runtime error; sentinel nil/empty for unavailable metadata | VM fallback |
| `soa` | Pure | none | runtime error | runtime specialization for recognized column kernels; VM fallback |
| `sort` | Pure | none plus script callback execution when provided | runtime error | runtime specialization for numeric sort cases; VM callback and fallback |
| `string` | Pure | none | runtime error | native identity for selected helpers; VM fallback |
| `sync` | Bounded host | in-process synchronization primitives | runtime error | VM fallback |
| `table` | Pure | none | runtime error; sentinel nil/empty for absent values | native identity for selected raw helpers/iterators; VM fallback |
| `testkit` | Test-only | `testkit` diagnostics | runtime error; diagnostic result tables | VM fallback |
| `time` | Bounded host | wall clock, timers, sleep | runtime error; `nil, err` for parse/cancel failures | VM fallback |
| `url` | Pure | none | runtime error; `nil, err` for malformed URLs | VM fallback |
| `utf8` | Pure | none | runtime error; sentinel nil/false for invalid positions | native identity for selected helpers; runtime specialization candidates |
| `uuid` | Bounded host | random UUID generation | runtime error; `nil, err` for malformed UUID input | VM fallback |
| `vec` | Pure | none | runtime error | VM fallback |

Contract rules:

- Every listed module must be obtainable through `require(name)` when enabled.
- Pure modules must not perform ambient host I/O.
- Host-backed modules must remain capability-gated by embedders.
- JIT and runtime specializations are optional accelerators and must preserve VM
  results when disabled, declined, or deoptimized.

