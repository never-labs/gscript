# Release Notes

Public tag notes live in this directory as `vX.Y.Z.md`.

`scripts/run.sh release-notes --require-ready --version vX.Y.Z` validates
that the candidate notes are filled in before the release workflow publishes
artifacts. Candidate notes list the artifact contract; final per-archive hashes
are generated from the tagged commit and shipped in the published
`SHA256SUMS` asset.

Each candidate note must list:

- `leia_vX.Y.Z_darwin_amd64.tar.gz`
- `leia_vX.Y.Z_darwin_arm64.tar.gz`
- `leia_vX.Y.Z_linux_amd64.tar.gz`
- `leia_vX.Y.Z_linux_arm64.tar.gz`
- `leia_vX.Y.Z_windows_amd64.zip`
- `leia_vX.Y.Z_windows_arm64.zip`
- `SHA256SUMS`
