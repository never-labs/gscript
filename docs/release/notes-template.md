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
- Optional extensions:
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
scripts/run.sh perf --release
```

## Validation

```bash
go run ./cmd/leia ci release --release-version vX.Y.Z --list
scripts/run.sh production --full --release-profile --release-version vX.Y.Z
scripts/run.sh test release-matrix
scripts/run.sh docs
scripts/run.sh perf --release
scripts/run.sh public-blockers --require-resolved
scripts/run.sh release-dist --require-goreleaser
scripts/run.sh release-notes --require-ready --version vX.Y.Z
scripts/run.sh release-check --build --require-clean --require-tag --version vX.Y.Z
```

## Known Issues

List known issues, or write `None known` after release validation.

## Checksums And Artifacts

Final checksums are generated from the tagged commit and distributed in the
published `SHA256SUMS` release asset. Verify that file before installing an
archive. Candidate notes list the complete artifact contract without embedding
pre-tag hashes that would change when the release commit changes.

| Artifact | Checksum source |
|---|---|
| `leia_vX.Y.Z_darwin_amd64.tar.gz` | published `SHA256SUMS` |
| `leia_vX.Y.Z_darwin_arm64.tar.gz` | published `SHA256SUMS` |
| `leia_vX.Y.Z_linux_amd64.tar.gz` | published `SHA256SUMS` |
| `leia_vX.Y.Z_linux_arm64.tar.gz` | published `SHA256SUMS` |
| `leia_vX.Y.Z_windows_amd64.zip` | published `SHA256SUMS` |
| `leia_vX.Y.Z_windows_arm64.zip` | published `SHA256SUMS` |
| `SHA256SUMS` | GitHub Release asset |

Each archive includes `leia` and `leia-lsp`.

## Release Decisions

- License:
- Security reporting:
- Platform support:
- Release channels:
- Artifact signing:
- Compatibility policy:
