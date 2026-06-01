# Leia Release Notes Template

Use this template for release candidates and public tags.

## Summary

- Version:
- Commit:
- Date:
- Platforms:
- Go version:
- License:

## Compatibility

- Stable language changes:
- Experimental language changes:
- Standard-library changes:
- Embedding API changes:
- CLI/tooling changes:
- Migration notes:

## Highlights

- AI-native:
- Go embedding:
- Hot reload:
- Concurrency:
- Data-oriented programming:
- Package/module tooling:

## Security

- Sandbox/capability changes:
- Host API changes:
- AI provider/secret-handling changes:
- Known risks:

## Performance

Include command, platform, CPU, Go version, LuaJIT availability, artifact path,
and caveats.

```bash
bash scripts/performance_gate.sh --feature-smoke
```

## Validation

```bash
bash scripts/production_check.sh --quick
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
bash scripts/docs_check.sh
bash scripts/performance_gate.sh --feature-smoke
```

## Known Issues

- 

## Checksums And Artifacts

| Artifact | SHA256 |
|---|---|
| | |
