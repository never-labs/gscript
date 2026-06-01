# Product Direction

Leia should present itself as an embeddable Go scripting language with two
clear differentiators:

1. **AI-native automation**: agents, tools, model configuration, history,
   record/replay, budget, and provider integration are language-level workflows
   that lower to ordinary stdlib/runtime primitives.
2. **Data-oriented hot paths**: dense arrays and SOA layouts let scripts keep a
   dynamic workflow while giving the runtime enough shape information to
   specialize numeric and record-heavy loops.

This direction borrows from Odin's public clarity around data-oriented
programming, but keeps Leia dynamic, embeddable, and Go-host friendly.

## Positioning

Short form:

> Leia is an embeddable Go scripting language for AI-native automation and
> data-oriented hot paths.

What this means:

- Go applications can embed Leia as a sandboxable runtime.
- Users can run Leia as a standalone CLI language.
- AI workflows are first-class enough to be readable in scripts, but still
  testable through provider injection, record/replay, and capability checks.
- Data-oriented workloads can opt into dense/SOA storage without turning the
  language into a static systems language.

## Standard Library Layers

Use these layers in docs, examples, capability policy, and future module index
work:

| Layer | Modules | Contract |
|---|---|---|
| `base` | `string`, `table`, `math`, `json`, `time`, `regexp`, `utf8`, `bit32`, `bits`, `bytes`, `binary`, `base64`, `hash`, `uuid`, `rand` | Pure or mostly pure utilities safe for ordinary scripts. |
| `host` | `fs`, `path`, `io`, `os`, `process`, `net`, `http`, `debug`, `log`, `script` | Host effects gated by embedding capabilities and sandbox policy. |
| `ai` | `llm`, `msg`, `history`, `chat`, `loop`, AI-native syntax lowering targets | Provider-backed automation; never commit secrets; live calls are opt-in. |
| `data` | `array`, `soa`, `vec`, `color`, matrix/dense-array future APIs | Hot data layout and numeric kernels with VM fallback and JIT specialization opportunities. |
| `compat` | Lua-facing compatibility functions and translated official cases | Compatibility surface for migration and oracle coverage, not the product identity. |
| `vendor` | Future approved Go/third-party bindings | Explicit host allowlist only; no arbitrary reflection import by default. |

## Data-Oriented Language Experience

The initial language-level SOA rule is intentionally small:

```leia
points := soa.zip({
    x: []f64{1, 2, 3},
    y: []f64{10, 20, 30},
})

points.x[2] = 42      // live column access
row := points[2]      // copied row table
row.y = 200
points[2] = row       // row writeback
```

This mirrors Odin's ergonomic goal: callers can write record-shaped code while
the storage remains columnar. Hot loops should still prefer live columns and
fused kernels (`soa.affine`, `soa.affineMany`, `soa.sumWhere`) instead of row
materialization.

## Tooling Direction

`leia test` should become the public script test runner rather than a hidden
Go harness. The first stable primitives are:

- stdout golden files through sibling `.out`;
- `--list` for deterministic discovery and future CI sharding;
- `--seed` for deterministic randomized tests through `LEIA_TEST_SEED`;
- JSON output for CI and editor integration.

Future work should add a script-level `testing` module, per-test timeouts,
JUnit output, and fixture helpers.

## What Not To Copy From Odin

- Do not add static type declarations or raw pointer controls as core product
  features.
- Do not expose CPU SIMD as a required language feature; keep it as an
  optimization tier with portable fallback.
- Do not make implicit context another hidden global mechanism. Host policy,
  cancellation, and budgets should remain explicit until the language has a
  settled context design.
