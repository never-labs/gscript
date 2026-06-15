---
layout: page
title: Scientific Numeric Runtime
---

# Scientific Numeric Runtime

Leia's scientific stack is built as a small set of reusable runtime layers. The
goal is not to add one-off helpers for Kalman filters, particle filters, or
control demos. Those programs should fall out of the same typed numeric,
columnar, and dialect infrastructure used by ordinary Leia, q analytics, and
host embeddings.

## Goals

- Keep Leia's core language small and Go-shaped.
- Treat dense numeric data as runtime values, not ad hoc tables.
- Route Leia, q, and domain libraries through the same typed kernels.
- Make fallback paths observable and removable.
- Keep domain power in reusable stdlib modules and dialects.
- Let examples double as conformance and performance regression tests.

## Runtime Layers

### 1. Typed Values

The existing `DenseArray`, `DenseMatrix`, and SoA frame runtime values are the
base representation for scientific data:

- `DenseArray[f64|i64|bool|string]` for vectors, masks, and typed columns.
- `DenseMatrix[f64]` for dense numeric matrices with contiguous backing.
- SoA frames for qSQL and columnar analytics.
- Views are preferred over copies when a future operation can preserve aliasing
  safely.

New library code must reuse these values before introducing new containers.

### 2. Kernel Backend

Numeric operations lower to a small kernel vocabulary:

- elementwise: add, sub, mul, div, pow, abs, sqrt, sin, cos, exp, log;
- reductions: sum, min, max, mean, norm, dot;
- scans: cumulative sum, deltas, running windows;
- masks: compare, where, filter, gather;
- matrix: transpose, matvec, matmul, solve, small-matrix fast paths;
- frame: column load, typed compare mask, filter, projection, group aggregate.

Kernels may be implemented in Go, lowered to MethodJIT native loops, or routed
to a future native/BLAS backend. All routes must preserve VM/runtime semantics.

### 3. Pipeline IR

Leia expressions, q expressions, and domain helpers should lower to a shared
pipeline shape before execution:

- `VectorPipeline`
- `MatrixPipeline`
- `FramePipeline`
- `LoopKernel`
- `Reduction`
- `Scan`
- `Solver`

The lowering stage records why a shape cannot use a typed kernel:

- dynamic dtype;
- unknown shape;
- unsupported op;
- aliasing risk;
- closure escape;
- null policy not supported.

### 4. Standard Library Facades

The public API is layered by domain:

- `linalg`: typed vector/matrix construction, algebra, solve, norm;
- `stats`: reductions, normalization, cumulative sums, resampling helpers;
- `ode`: reusable integrators such as RK4;
- `control`: control-system helpers such as saturation, angle wrapping, LQR;
- `q`: q-style vector and columnar analytics, backed by the same kernels;
- `plot`: future artifact-producing visualization module.

Domain modules are facades over typed runtime values and kernels. They must not
encode behavior specific to a single example.

The compatibility rule for the first implementation is:

- public constructors may accept ordinary Leia numeric lists and nested lists;
- runtime operations must also accept existing `DenseArray` and `DenseMatrix`
  values;
- legacy table metadata shapes are accepted as adapter inputs, not used as the
  primary storage format;
- vector hot paths return typed dense arrays and matrix hot paths return
  DenseMatrix-compatible values, so MethodJIT, q, and `matrix` can share the
  same backing data.

The next cleanup step is to lower dense linalg operations through shared
kernel descriptors so q, Leia, and future JIT routes can observe the same
fallback and hit-rate diagnostics.

### 5. q Integration

q keeps its parser and semantics, but q runtime execution should call the same
typed backend:

- q vector verbs map to vector kernels;
- q matrix verbs map to linalg kernels;
- qSQL maps to frame pipelines;
- tagged q interpolation converts Leia values to typed q values without string
  re-parsing when possible.
- q raw source blocks support top-level newline statement separators, so
  multi-line q algorithms can be written as `q { ... }` instead of quoted
  source strings.
- q raw source blocks and q tagged strings share `${...}` interpolation and
  Leia-to-q literal encoding for scalars, lists, and dense arrays.

Leia-to-q bridges must support dense arrays, dense matrices, frames, ordinary
lists, scalars, strings, booleans, and nil values.

### 6. JIT Integration

MethodJIT should recognize the shared pipeline IR rather than many q-specific
special cases. The first supported routes are:

- scalar numeric loops;
- vector elementwise pipelines;
- reductions and scans;
- dense matrix get/set, matvec, and small matrix operations;
- frame column load, typed compare mask, filter, project, group aggregate;
- inlinable small closures used by `ode.rk4` and simulation loops.

Every JIT miss or runtime fallback must produce stable diagnostic metadata.

## Test Strategy

The scientific examples are acceptance tests:

- Kalman filter: matrix algebra, deterministic simulation, state estimation;
- particle filter: vector math, random/replayable sampling, resampling;
- inverted pendulum: ODE integration, closures, LQR/control switching.

Each example must:

- execute as ordinary Leia source;
- use generic `linalg`, `stats`, `ode`, `control`, `q`, and `math` APIs;
- print deterministic summary values;
- avoid example-specific native helpers;
- remain small enough to demonstrate the product direction.

Implementation tests live next to the owning modules. End-to-end example tests
live under `tests/scientific_numeric_examples_test.go`, run the translated Leia
programs through the CLI, parse deterministic summary fields, and execute the
same acceptance checks in the default and bytecode VM modes.

## Initial Milestones

1. Register `linalg`, `stats`, `ode`, and `control` modules using existing
   DenseArray and DenseMatrix values.
2. Add reusable vector/matrix constructors and small-matrix algebra.
3. Add generic statistics and resampling helpers.
4. Add RK4 and control helpers that accept ordinary Leia functions.
5. Convert the three MATLAB-style examples into Leia acceptance tests.
6. Route hot vector/matrix operations into the shared kernel/JIT diagnostics.

## Current Surface

The current standard-library surface intentionally favors reusable pieces over
example-specific shortcuts:

- `math.near` for tolerance checks.
- `linalg.eye(n[, scale])`, `linalg.diag(values...)`,
  `linalg.identity_minus`, `linalg.affine`, `linalg.axpy`, and
  `linalg.add_scaled` for compact matrix construction and scaled
  vector/matrix updates.
- `linalg.at` for shape-friendly vector, row, column, and matrix access.
- `linalg.matmul` as a variadic matrix-chain facade that also supports
  matrix-vector tails, plus `linalg.matmul_t`, `linalg.T`/`linalg.t`,
  `linalg.vec`, and `linalg.sandwich_add` for compact linear algebra.
- `stats.normal`, `stats.pdf`, `stats.logpdf`, `stats.loglik`,
  `stats.samples(values[, weights])`, `stats.update(samples,
  log_likelihoods[, opts])`, `stats.observe(samples, distribution,
  observed[, opts])`, `stats.bayes_update`,
  `stats.describe(values[, weights])`/`stats.describe(samples)`,
  `stats.describe_fields(table[, weights])`, and `stats.importance_update` for
  distribution-aware sequential Monte Carlo code and compact statistical
  summaries.
- `stats.gaussian_state`, `stats.linear_predict`, `stats.linear_update`, and
  `stats.linear_filter` for reusable linear Gaussian state-space updates and
  compact filter loops with diagnostics such as innovation, innovation
  covariance, gain, and optional state trajectories.
- `rand.sample(distribution[, n])` and `rand.add_noise(values,
  distribution[, drift])` for scalar draws, dense-vector draws, and noisy
  vector evolution from reusable distribution objects. `rand.add_noise` and
  `stats.loglik` also preserve or consume weighted sample-set objects so
  Monte Carlo code can stay object-shaped.
- `control.lqr`, `control.feedback`, `control.saturate`, and
  `control.wrap_angle` for small control systems.
- `control.policy(gain[, opts])` and `control.apply(policy, state[, opts])`
  for reusable feedback policies over dense-vector or named-table state, with
  optional per-call overrides and periodic-coordinate wrapping.
- `ode.integrate` and `ode.solve` for RK4-style simulation with projection,
  scalar or table-shaped observation hooks, and result-object access.
- `ode.solve(..., {state_names: {...}, wrap_angles: {...}})` for named final
  state access, named trajectories, and periodic coordinate normalization.
- `ode.solve(..., {state_names: {...}, named_state: true})` for optional
  named state tables in dynamics, projection, and observation hooks while the
  default dense-vector hot path remains unchanged.
- `q { ... }` raw blocks for compact q snippets without quoted source strings.
