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

Before a public tag, update install instructions, examples, compatibility
notes, known issues, benchmark caveats, and security notes.

