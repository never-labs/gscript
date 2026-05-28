# Data-Oriented and Array Programming Design

This document is a design slice for adding data-oriented and array-programming
capabilities to GScript. It references Odin because Odin exposes several of the
same ideas as language features, but GScript is a dynamic embeddable scripting
language. The goal is to migrate the shape of the ideas, not Odin's static
compiler model.

Primary Odin references:

- [Odin overview: fixed arrays and array programming](https://odin-lang.org/docs/overview/#array-programming)
- [Odin overview: swizzle operations](https://odin-lang.org/docs/overview/#swizzle-operations)
- [Odin overview: SOA data types](https://odin-lang.org/docs/overview/#soa-data-types)
- [Odin overview: matrix type](https://odin-lang.org/docs/overview/#matrix-type)
- [Odin `core:simd` package](https://pkg.odin-lang.org/core/simd/)
- [Odin `base:builtin` package: `swizzle`, `soa_zip`, `soa_unzip`, `raw_data`](https://pkg.odin-lang.org/base/builtin/)

## Odin Ideas Worth Migrating

### Fixed arrays as element-wise values

Odin fixed arrays are contiguous values with a known length and one element
type. Odin applies array programming to them directly: arithmetic and comparison
operators can operate element-wise on fixed arrays. That works well because the
compiler knows the element type, length, and storage layout before codegen.

GScript should migrate the user model, not the exact type model:

- typed dense arrays should have a single numeric element kind and contiguous
  backing storage;
- element-wise operations should require shape compatibility and should fail
  clearly instead of falling back to table-like behavior;
- scalar broadcasting can be supported where it is unambiguous, for example
  `dense + 1`, but array-array operations must use equal lengths unless a later
  phase explicitly adds ranked shapes;
- the first implementation should be library/API surface, with syntax only when
  semantics and parser ownership are ready.

### Swizzle as a small-vector projection

Odin has explicit `swizzle(a, 2, 1, 0)` and implicit fields such as `xyzw` and
`rgba` for arrays with length at most four. This is useful for graphics and
small vector math because it makes lane projection, duplication, and reordering
cheap and readable.

GScript should start with explicit projection operations. Implicit field syntax
looks attractive, but it collides with GScript table/property semantics and is
not suitable for a dynamic language until the parser and object model have a
clear distinction between vector lanes and table fields.

Portable design:

- `vec.swizzle(v, 2, 1, 0)` or `v:swizzle(2, 1, 0)` style API first;
- lane names can be accepted as strings or constants, for example `"zyx"`, only
  after error behavior is specified;
- swizzle result should preserve the vector family and element kind;
- duplicate lanes are allowed for reads; writable swizzles are out of scope.

### Matrix as explicit layout plus math contract

Odin's matrix type has a declared element type and dimensions. Its default
internal representation is column-major to support SIMD-friendly access, and it
has an explicit row-major directive for alternate storage. Odin can make those
layout decisions statically.

GScript should separate matrix semantics from storage details:

- matrix values should carry element kind, rows, columns, and layout metadata;
- math APIs should define whether operations are element-wise, matrix
  multiplication, transpose, dot, or reduce operations;
- serialization and host embedding should not expose accidental internal
  padding;
- row-major/column-major should be constructor options, not syntax in Phase B;
- JIT specialization can choose internal kernels later as long as visible layout
  APIs keep their contract.

### SOA as layout control without ergonomic loss

Odin's `#soa` lets code access records as if they were an array of structs while
storing fields as separate arrays. It also provides `soa_zip` and `soa_unzip` so
existing slices can be treated as one structured sequence without copying.

This is the most important Odin idea for GScript: users should get the cache and
SIMD benefits of structure-of-arrays without writing index plumbing everywhere.

Portable design:

- start with an explicit `soa.zip({x = xs, y = ys, z = zs})` or constructor API;
- expose row views for ergonomic reads, but document whether a row view is a
  live proxy or a copied table;
- expose column arrays directly for hot loops;
- require all columns to have equal length at construction and after resizing;
- make mutation rules explicit so aliases do not silently desynchronize columns.

### SIMD as an implementation tier, not the semantic baseline

Odin exposes SIMD vectors as `#simd [N]T`, with lane operations and support
procedures such as gather, scatter, select, reductions, fused multiply-add, and
runtime swizzle. That is appropriate for a compiled systems language.

For GScript, SIMD should first be a backend optimization for dense arrays,
vectors, matrices, and SoA columns. The user-facing contract should be ordinary
numeric semantics with stable fallbacks. SIMD should only become visible through
capability/debug APIs once correctness, determinism, and platform fallback rules
are tested.

## What Not To Copy Directly

- Do not copy Odin's compile-time fixed-array syntax as a Phase A feature.
  GScript currently has dynamic syntax and table semantics; parser-level array
  types would create a large language-spec burden.
- Do not copy implicit `.x`, `.y`, `.rgba`, or writable swizzle fields until
  table field lookup, method lookup, and vector lane lookup have a documented
  precedence model.
- Do not expose raw pointers or Odin-style `raw_data` semantics to scripts.
  Host APIs may need controlled buffer access, but script-level pointer access
  conflicts with embedding safety and sandboxing.
- Do not expose `#no_bounds_check`-style controls. Bounds checks are part of
  GScript safety; optimized kernels can remove checks internally only when they
  preserve observable errors.
- Do not require users to understand alignment, padding, or CPU lane width.
  These belong to runtime specialization and diagnostics, not core semantics.
- Do not promise hardware SIMD as a language feature. The same script must run
  correctly on unsupported architectures and with JIT disabled.

## Phased Plan

### Phase A: Typed dense arrays

Introduce dense homogeneous arrays as library/runtime values without parser
changes.

Deliverables:

- constructors for `f64`, `f32`, `i64`, `i32`, and `bool` dense arrays;
- length, capacity, indexing, slicing, copy, fill, append, resize, and
  conversion to/from ordinary GScript tables;
- element-wise `add`, `sub`, `mul`, `div`, comparisons, min/max, clamp, and
  reductions through stdlib functions;
- clear shape/type errors for mismatched lengths and unsupported element kinds;
- host embedding conversion rules for Go slices and dense GScript values.

Acceptance:

- dense operations produce the same results in interpreter, VM, and JIT-disabled
  modes;
- no dense operation silently boxes every element into a table on hot paths;
- docs define scalar broadcasting, length mismatch, NaN, integer overflow, and
  mutation behavior;
- benchmark coverage includes table numeric loops versus dense-array loops.

### Phase B: Vec and matrix values

Add small fixed-width vectors and 2D matrices as structured dense values.

Deliverables:

- `vec2`, `vec3`, `vec4` constructors over numeric element kinds;
- explicit swizzle/projection function with read-only duplicate-lane support;
- dot, cross where defined, norm, normalize, element-wise arithmetic, and
  scalar broadcast;
- matrix constructors with rows, columns, element kind, and row-major or
  column-major layout option;
- matrix transpose, element-wise arithmetic, matrix-vector, and matrix-matrix
  multiplication APIs.

Acceptance:

- vector and matrix operations have documented shape rules and error messages;
- swizzle does not use table property lookup or parser-level lane fields;
- matrix serialization round-trips without exposing padding;
- layout-sensitive APIs identify row-major versus column-major behavior.

### Phase C: SOA layout

Add structure-of-arrays values for records with homogeneous-length columns.

Deliverables:

- `soa.zip` and `soa.unzip` equivalents for named dense arrays;
- column access for hot loops and row access for ergonomic code;
- append, remove, resize, slice, filter, and map-like helpers that preserve
  column alignment;
- host embedding support for building SoA values from Go column slices;
- diagnostics that report column names, element kinds, and layout.

Acceptance:

- construction rejects unequal column lengths;
- row mutation rules are explicit and tested as either live proxy or copy;
- slicing preserves layout and does not materialize row tables unless requested;
- benchmark coverage includes AoS table rows versus SoA columns.

### Phase D: Runtime specialization and JIT kernels

Specialize dense, vector, matrix, and SoA operations after semantics are stable.

Deliverables:

- runtime shape/type guards for dense-array kernels;
- fallback paths that preserve Phase A-C errors and results;
- optional SIMD kernels for supported CPUs;
- JIT lowering for common element-wise loops, reductions, matrix kernels, and
  SoA column loops;
- diagnostics showing when a kernel specialized, bailed out, or fell back.

Acceptance:

- disabling JIT or SIMD does not change output;
- every specialized kernel has interpreter/VM parity tests;
- deoptimization preserves live dense arrays and SoA aliases correctly;
- performance gates track both speedups and fallback regressions.

### Phase E: Demos and stdlib polish

Make the feature useful outside synthetic benchmarks.

Deliverables:

- stdlib reference pages for dense arrays, vectors, matrices, and SoA;
- examples for particle simulation, image filters, pathfinding grids, and small
  linear algebra tasks;
- embedding examples that pass Go slices into dense or SoA values;
- debugging/inspection helpers for shapes, layouts, and kernel selection.

Acceptance:

- examples run in CI as smoke tests;
- demos have correctness oracles, not only printed timings;
- docs show fallback behavior and unsupported platform behavior;
- release notes list which parts are stable and which remain experimental.

## Kill Features

Kill features are explicit scope cuts. They prevent the project from growing a
new language before the runtime contract is proven.

- No parser-level typed array, vector, matrix, or SoA syntax in Phase A-C.
- No implicit swizzle fields such as `v.xyz` or `v.rgba` in Phase A-C.
- No writable swizzles.
- No script-visible raw pointers, unchecked indexing, alignment directives, or
  manual SIMD lane-width controls.
- No automatic conversion of arbitrary tables into dense arrays inside hot
  arithmetic. Users must construct dense values intentionally.
- No hardware-SIMD guarantee in the language spec.
- No ranked N-dimensional array language until 1D dense arrays and 2D matrices
  have shipped with tests and docs.

## Cross-Phase Acceptance Checklist

Any implementation phase should be blocked until it satisfies these checks:

- semantics are documented before parser/runtime behavior is exposed as stable;
- interpreter, VM, JIT-disabled, and JIT-enabled paths agree on results and
  errors for the same feature surface;
- dense and SoA values have bounded, documented host conversion behavior;
- sandbox/resource accounting treats large dense allocations and derived slices
  as real memory use;
- benchmarks compare against table-based equivalents and include correctness
  oracles;
- unsupported platforms fall back to portable kernels without changing output;
- docs name non-goals and experimental APIs instead of implying full NumPy,
  APL, Odin, or shader-language compatibility.
