# Release Process

Leia releases need machine-checkable evidence, not hand-written claims.

## Machine-Checkable Release Evidence

Run at least:

```bash
bash scripts/production_check.sh --full --release-profile
go run ./cmd/leia ci release --list
```

`leia ci release` delegates to the same production release profile. That
profile is the release gate source of truth: correctness, documentation,
performance, q conformance, language conformance, public blockers,
distribution configuration, and local artifact installation evidence are all
listed there. Documentation evidence is produced by `scripts/docs_check.sh`
inside that profile.

The release evidence should cite:

- `docs/spec/index.md`
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
- `docs/reference/platforms/index.md`
- `docs/reference/diagnostics/index.md`
- `examples/README.md`
- `docs/release/decisions.md`
- `scripts/public_release_blockers_check.sh`

Before a public tag, update install instructions, examples, compatibility
notes, known issues, benchmark caveats, and security notes.

Use [`decisions.md`](decisions.md) to record maintainer decisions that cannot
be inferred from tests, local release evidence, or implementation defaults.

Distribution checks are split between local artifacts and hosted workflow
presence. The local check validates GoReleaser metadata, the install script
dry-run matrix, and local `file://` tar.gz/zip install fixtures even when
GitHub workflow files are intentionally absent:

```bash
bash scripts/public_release_blockers_check.sh --require-resolved
bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows
bash scripts/install.sh --version v0.1.0 --os darwin --arch arm64 --dry-run
bash scripts/install.sh --version v0.1.0 --base-url file:///tmp/leia-release --bin-dir /tmp/leia-bin
```

Release archives must include both executables:

- `leia`, the CLI and script runner;
- `leia-lsp`, the shared language server used by editor integrations.

Use [`notes-template.md`](notes-template.md) for release candidates and public
tags so compatibility, security, performance, validation, and artifact evidence
are recorded consistently.

## Release Compatibility Checklist

Every public tag must have release notes. The notes must identify:

- stable behavior covered by the specification, feature matrix, and gates;
- experimental behavior that can change or disappear without compatibility
  guarantees;
- implementation-defined behavior that depends on host capabilities, execution
  mode, platform, provider, or build configuration;
- tested OS/architecture combinations and Go version;
- execution modes used for validation, including interpreter, VM, and any JIT
  coverage;
- disabled capabilities, live providers, or external integrations;
- release artifacts, including `leia`, `leia-lsp`, and SHA256 checksums;
- evidence links for the spec, feature matrix, security reference, platform
  reference, and performance gates.

## Public Release Blockers

Do not cut a public release until these repository-level decisions are complete:

- choose a license and add a root `LICENSE` file;
- confirm the vulnerability reporting route in `SECURITY.md`;
- complete the release decisions recorded in `docs/release/decisions.md`;
- verify install commands against the published module path;
- state tested platforms and execution modes;
- run release evidence on the target platforms;
- fill out `docs/release/notes-template.md` for the candidate;
- commit the candidate notes at `docs/release/notes/vX.Y.Z.md`;
- document any experimental language, stdlib, AI, package, or JIT behavior that
  is intentionally outside the compatibility promise.
