---
layout: page
title: Scientific Numeric Programming
---

# Leia Scientific Numeric Programming

Leia keeps scientific code in ordinary Leia source. Numeric modules expose
small reusable primitives over typed vectors, matrices, sample sets, state
models, and q-compatible values; whole algorithms stay visible in user code.

## Default Numeric Imports

When the owning standard-library modules are enabled, common numeric helpers
are also installed as globals:

| Source module | Default helpers |
|---|---|
| `math` | `sqrt`, `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `exp`, `log`, `abs`, `floor`, `ceil`, `round`, `sign`, `near` |
| `linalg` | `vec`, `row`, `col`, `mat`, `eye`, `diag`, `at`, `T`, `trace`, `dot`, `norm`, `matvec`, `matmul`, `solve`, `axpy`, `add_scaled` |
| `stats` | `sum`, `mean`, `avg`, `variance`, `std`, `rms`, `rmse`, `cumsum`, `diff`, `describe` |
| `rand` | `seed`, `randn`, `sample`, `shuffle` |

Globals are convenience bindings, not syntax. They follow `WithLibs`: disabling
a module removes that module's default helpers while leaving ordinary language
semantics unchanged.

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
checksum := q {
+/${state.x}
}

assert(abs(residual) < 1.0)
assert(energy > 0)
assert(near(checksum, state.x.position + state.x.velocity, 0.000000001))
```

The same values can move through Leia functions, q raw blocks, tagged q strings,
and host embeddings without forcing users to serialize numeric lists into
source text.

## Design Rules

- Prefer typed vector and matrix values over table-shaped adapters.
- Prefer reusable primitives over vertical algorithm facades.
- Let q handle compact vector and columnar expressions when it makes the code
  shorter.
- Keep fallback observable in runtime and JIT diagnostics instead of changing
  language semantics.

Acceptance programs live under `examples/scientific/`.
