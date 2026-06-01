# Testing And Release Gates

Leia uses several test layers:

- Go unit tests for parser, runtime, VM, JIT, stdlib, CLI, package management,
  and public SDK behavior.
- `.leia` functional tests under `tests/`.
- Translated language conformance cases under `tests/language/`.
- Benchmark manifests and performance gates under `benchmarks/`.
- Documentation and release evidence checks.
- Runnable examples embedded in `docs/spec/*.md` code fences marked
  `leia run` or `leia run all`.

Important inputs:

- `tests/language/MISSING_CAPABILITIES.md`
- `tests/language/KNOWN_FAILURES.md`
- `tests/language/MANIFEST.md`
- `docs/reference/stdlib/index.md`
- `tests/feature_matrix.json`
- `docs/spec/index.md`

Release-gate commands:

```bash
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
go test ./tests -run TestSpecRunnableExamples -count=1
go test ./tests -run 'TestFeatureMatrix|TestLanguageConformanceTranslatedCases' -count=1
go test ./...
bash scripts/docs_check.sh
bash scripts/performance_gate.sh --feature-smoke
```

See [Performance and benchmarks](reference/performance/index.md) for benchmark
selectors, timing quality rules, strict guard modes, and artifact conventions.
