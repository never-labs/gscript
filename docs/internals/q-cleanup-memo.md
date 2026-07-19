# Leia q Cleanup Memo

Repository: Leia repo working tree

Branch: `main`

Remote: `https://github.com/never-labs/leia.git`

## Current State

The q cleanup work shipped in Leia v0.1.0. The release tag and `main` pointed
to `176ed5da` when the release artifacts were built and published. The q
extension remains in the repository behind the `leia_q` build tag; it is not
part of the default v0.1.0 product surface.

The commits that established that boundary are:

- `c19a162c refactor: gate q extension behind build tag`
- `d8b4b601 refactor: remove q from default product surface`
- `3aba757b refactor: remove q gate entrypoints`
- `a4afd6f0 refactor: make q optional extension surface`

## Completed

The default Leia product surface is mostly decoupled from q:

- Default VM/CLI no longer depends on `internal/stdlib/lib/q`.
- `q` and `qsql` are no longer in the default playground dialect allowlist.
- The lexer no longer treats `q {}` or `qsql {}` as special raw block forms.
- `q` is no longer a default compiler predeclared global.
- q-specific bind, methodjit, benchmark, and test paths are mostly gated behind the `leia_q` build tag.
- Default docs and feature matrix no longer use q examples as core feature evidence.
- Generic default-build frame/data helpers have been renamed away from
  q-prefixed names:
  `qDataFrameFromSoA`, `qDataArrayFromDense`, `qLooksLikeFrame`, and
  `qNativeFrameRuntimeKind`.
- Release docs now treat extension semantics as optional/experimental rather
  than part of the default stable surface.
- Default benchmark discovery excludes q/qsql workloads; a CLI built with the
  `leia_q` tag includes them in the same manifest-backed benchmark tooling.

Validation that already passed:

```sh
go test ./...
go test -tags leia_q ./...
git diff --check
go list -deps ./internal/vm ./cmd/leia | rg 'internal/stdlib/lib/q' || true
go test ./internal/stdlib/bind
go test -tags leia_q ./internal/stdlib/bind -count=1
```

The final `go list` command produced no output, confirming the default VM/CLI dependency graph no longer includes the q package.

## Completion Estimate

If the target is "q is not part of the default Leia product surface/build":

- Estimated completion: 90-95%

If the target is "q is fully removed from the main repository":

- Estimated completion: 35-45%

If the target is "q becomes an independent optional extension package":

- Estimated completion: 50-60%

## Remaining Work

### 1. Post-v0.1 extension boundary

In v0.1.0, q is an in-repository experimental extension enabled only
with the `leia_q` build tag. It is not part of the default CLI/VM dependency
graph, stable language specification, default documentation narrative, or
platform support claim.

Moving q to an independent package or removing it remains a post-v0.1 design
decision. The first public release does not promise compatibility for q syntax,
APIs, diagnostics, performance, or extension internals.

### 2. Continue neutralizing q-prefixed default helpers

The generic default-build helpers for frame conversion and runtime frame
classification have been renamed to data/runtime terminology:

- `dataLibFrameFromSoA`
- `dataLibArrayFromDense`
- `looksLikeFrame`
- `nativeFrameRuntimeKind`

Remaining q-prefixed runtime-kernel names are mostly extension-facing or
methodjit internals. Clean those only after deciding whether the q provider
stays in-repo or moves into a separate extension package.

One default-build file still carries q-oriented interpolation helper names and
diagnostics in `internal/stdlib/bind/dialect.go`. The q dialect handlers are
already build-tagged, but moving these helpers requires a cleaner extension
boundary for interpolation and bound-value encoding, so this should be handled
with the methodjit/runtime-kernel boundary work rather than as a mechanical
rename.

### 3. Clean up methodjit q residue

Default builds no longer depend on q, but methodjit internals still contain q-oriented naming in diagnostics, comments, and backend descriptors.

Target architecture:

- typed runtime backend
- pipeline backend
- extension-provided kernel descriptors
- optional q provider registered only under `leia_q`

### 4. Define optional q validation

The optional extension validation path is:

```sh
go test ./...
go test -tags leia_q ./...
```

Benchmark discovery distinguishes the default Leia set from q extension
workloads at build time. Keep both default and `leia_q` discovery tests when
adding q/qsql benchmark files.

### 5. Clean up docs and site references

Default README/spec should emphasize:

- Go embeddable scripting language
- JIT execution
- first-class extensible dialects
- data-oriented runtime and in-memory computation

If q remains, it should live under optional extension documentation rather than default core language documentation.

## Recommended Next Step

Do the low-risk architecture cleanup first:

1. Keep q out of the default README/spec/site narrative and default dependency
   graph.
2. Run the opt-in validation path before tags that intentionally include the
   extension source.
3. Continue MethodJIT/runtime-kernel naming cleanup behind that boundary as
   post-v0.1 architecture work.
