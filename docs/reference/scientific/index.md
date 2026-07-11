---
layout: page
title: Scientific Numeric Programming
---

# Leia Scientific Numeric Programming

Leia keeps scientific code in ordinary Leia source. Numeric modules expose
small reusable primitives over typed vectors, matrices, sample sets, state
models, and columnar data values; whole algorithms stay visible in user code.

## Default Numeric Imports

When the owning standard-library modules are enabled, common numeric helpers
are also installed as globals:

| Source module | Default helpers |
|---|---|
| `table` | `append` |
| `math` | `abs`, `sqrt`, `exp`, `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `floor`, `ceil`, `round`, `min`, `max`, `clamp`, `near`, `pow` |
| `linalg` | `vector`, `vec`, `mat`, `row`, `col`, `eye`, `diag`, `zeros`, `ones`, `at`, `norm`, `dot`, `matvec`, `matmul`, `axpy`, `solve`, `trace`, `transpose` |
| `stats` | `sum`, `mean`, `avg`, `variance`, `std`, `describe`, `rms`, `rmse`, `cumsum`, `diff`, `normalize` |
| `rand` | `randn`, `sample` |

Globals are convenience bindings, not syntax. They follow `WithLibs`: disabling
a module removes that module's default helpers while leaving ordinary language
semantics unchanged. `append` is the core table convenience binding and remains
available under the core table runtime contract.

## Primitive Composition

```leia
F := mat([[1.0, 1.0], [0.0, 1.0]])
H := row(1.0, 0.0)
Q := eye(2, 0.01)

state := stats.gaussian_state([0.0, 1.0], eye(2), {names: ["position", "velocity"], named_state: true})
state = stats.linear_predict(state, F, Q)
state = stats.linear_update(state, H, 0.95, 0.04)

residual := state.innovation[1]
energy := dot([state.x.position, state.x.velocity], [state.x.position, state.x.velocity])
checksum := sum([state.x.position, state.x.velocity])

assert(abs(residual) < 1.0)
assert(energy > 0)
assert(near(checksum, state.x.position + state.x.velocity, 0.000000001))
```

The same values can move through Leia functions, tagged dialects, and host
embeddings without forcing users to serialize numeric lists into source text.

## Design Rules

- Prefer typed vector and matrix values over table-shaped adapters.
- Prefer reusable primitives over vertical algorithm facades.
- Keep compact numeric programs in ordinary Leia syntax unless a domain
  extension is explicitly installed.
- Keep fallback observable in runtime and JIT diagnostics instead of changing
  language semantics.

Acceptance programs live under `examples/scientific/`.
