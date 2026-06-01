# Release Process

Leia releases need machine-checkable evidence, not hand-written claims.

## Machine-Checkable Release Evidence

Run at least:

```bash
bash scripts/production_check.sh --quick
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
bash scripts/docs_check.sh
bash scripts/performance_gate.sh --feature-smoke
```

The release evidence should cite:

- `docs/spec/language.md`
- `tests/feature_matrix.json`
- `tests/language/MANIFEST.md`
- `tests/language/KNOWN_FAILURES.md`
- `tests/language/MISSING_CAPABILITIES.md`
- `docs/reference/stdlib/index.md`
- `docs/reference/cli/index.md`
- `docs/reference/modules/index.md`
- `docs/reference/embedding/index.md`
- `docs/reference/security/index.md`
- `docs/reference/hot-reload/index.md`
- `docs/reference/ai/index.md`
- `docs/reference/concurrency/index.md`
- `docs/reference/data-oriented/index.md`
- `docs/reference/performance/index.md`
- `docs/reference/diagnostics/index.md`

Before a public tag, update install instructions, examples, compatibility
notes, known issues, benchmark caveats, and security notes.
