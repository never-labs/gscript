# Testing And Release Validation

Leia uses several test layers:

- Go unit tests for parser, runtime, VM, JIT, stdlib, CLI, package management,
  and public SDK behavior.
- `.leia` functional tests under `tests/`.
- Translated language conformance cases under `tests/language/`.
- Benchmark manifests and performance checks under `benchmarks/`.
- Documentation and release evidence checks.
- Runnable examples embedded in `docs/spec/*.md` code fences marked
  `leia run`, `leia run all`, `leia fail`, or `leia fail all`.

Important inputs:

- `tests/language/MISSING_CAPABILITIES.md`
- `tests/language/KNOWN_FAILURES.md`
- `tests/language/MANIFEST.md`
- `docs/reference/stdlib/index.md`
- feature coverage records under `tests/`
- `docs/spec/index.md`

Release validation commands:

```bash
time scripts/run.sh perf --syntax-smoke --no-luajit
scripts/run.sh test release-matrix
scripts/run.sh test spec-examples
scripts/run.sh language-conformance
scripts/run.sh test correctness
scripts/run.sh docs
scripts/run.sh perf --feature-smoke
```

Language conformance uses the pinned Lua 5.5.0 oracle installed by
`scripts/install_lua_oracle.sh`; set `LUA_BIN` to that executable. The
translated corpus remains derived from the separately recorded Lua 5.4.8 test
suite. LuaJIT is a performance reference and is not the conformance oracle.

Use the `--syntax-smoke` command first after syntax-only language changes when
you need quick hot-path performance evidence without the longer strict pass.

See [Performance and benchmarks](reference/performance/index.md) for benchmark
selectors, timing quality rules, strict guard modes, and artifact conventions.
