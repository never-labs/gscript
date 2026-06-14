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
time bash scripts/performance_gate.sh --syntax-smoke --no-luajit
go test ./tests -run 'TestReleaseMatrix' -count=1
go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1
go test ./tests -run 'TestLanguageConformanceTranslatedCases' -count=1
go test ./...
bash scripts/docs_check.sh
bash scripts/performance_gate.sh --feature-smoke
```

Use the `--syntax-smoke` command first after syntax-only language changes when
you need quick hot-path performance evidence without the longer strict pass.

See [Performance and benchmarks](reference/performance/index.md) for benchmark
selectors, timing quality rules, strict guard modes, and artifact conventions.
