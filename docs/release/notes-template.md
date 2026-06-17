# Leia Release Notes Template

Use this template for release candidates and public tags.

## Summary

- Version:
- Commit:
- Date:
- Platforms:
- Go version:
- Execution modes tested:
- License:

## Compatibility

- Stable language changes:
- Experimental language changes:
- Standard-library changes:
- Embedding API changes:
- CLI/tooling changes:
- Migration notes:

## Highlights

- Go embedding:
- Data-oriented programming:
- q analytics:
- DSL/dialect changes:
- Performance:
- Concurrency:
- Hot reload:
- Package/module tooling:
- LLM/AI integrations, if changed:

## Security

- Sandbox/capability changes:
- Host API changes:
- LLM provider/secret-handling changes, if any:
- Known risks:

## Performance

Include command, platform, CPU, Go version, LuaJIT availability, artifact path,
and caveats.

```bash
bash scripts/performance_gate.sh --full
```

## Validation

```bash
go run ./cmd/leia ci release --release-version vX.Y.Z --list
bash scripts/production_check.sh --full --release-profile --release-version vX.Y.Z
go test ./tests -run 'TestReleaseMatrix' -count=1
bash scripts/docs_check.sh
bash scripts/performance_gate.sh --full
bash scripts/public_release_blockers_check.sh --require-resolved
bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows
bash scripts/release_notes_check.sh --require-ready --version vX.Y.Z
bash scripts/release_artifacts_check.sh --build --require-clean --require-tag --version vX.Y.Z
```

## Known Issues

List known issues, or write `None known` after release validation.

## Checksums And Artifacts

| Artifact | SHA256 |
|---|---|
| `leia_vX.Y.Z_darwin_amd64.tar.gz` | |
| `leia_vX.Y.Z_darwin_arm64.tar.gz` | |
| `leia_vX.Y.Z_linux_amd64.tar.gz` | |
| `leia_vX.Y.Z_linux_arm64.tar.gz` | |
| `leia_vX.Y.Z_windows_amd64.zip` | |
| `leia_vX.Y.Z_windows_arm64.zip` | |
| `SHA256SUMS` | |

Each archive includes `leia` and `leia-lsp`.

## Release Decisions

- License:
- Security reporting:
- Platform support:
- Release channels:
- Artifact signing:
- Compatibility policy:
