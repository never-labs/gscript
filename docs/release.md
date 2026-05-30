# Release Engineering Roadmap

This document defines the release process needed to ship GScript as a
production artifact while preserving language compatibility, JIT correctness,
and performance visibility.

## Release Goals

Each release must answer four questions:

1. What version is this?
2. Which programs should continue to work?
3. Which platforms and artifacts are supported?
4. What performance evidence backs the release?

The release is not complete until binaries, source archives, checksums, README
updates, examples, compatibility notes, and performance reports are available
from the same tagged commit.

## Versioning

Use semantic versioning for public releases:

- `MAJOR` changes when language semantics, standard-library contracts, CLI
  behavior, or bytecode/runtime compatibility are intentionally broken.
- `MINOR` changes when new language features, standard-library APIs, CLI
  options, diagnostics, or JIT capabilities are added compatibly.
- `PATCH` changes when fixing correctness bugs, crashes, regressions, packaging
  problems, or documentation errors without changing public contracts.
- Pre-releases use `vMAJOR.MINOR.PATCH-rc.N`, `-beta.N`, or `-alpha.N`.

Tags should be immutable and use the form `vX.Y.Z`. Release candidates should
be cut from the same branch that will produce the final release. If a release
candidate fails compatibility or performance gates, fix forward and create a new
candidate tag.

## Compatibility Strategy

Compatibility has three layers:

| Layer | Policy |
|---|---|
| Language syntax and runtime semantics | Compatible within a major version unless a release note explicitly marks a breaking change. |
| Standard library | Existing documented functions keep behavior and error shape within a major version. Additions are minor releases. |
| CLI and diagnostics | Stable flags remain compatible. Diagnostic output may grow fields, but machine-readable JSON should avoid breaking field removals in minor/patch releases. |

The VM remains the semantic oracle for JIT behavior. A release candidate must
show that default JIT and no-filter JIT preserve VM-visible behavior on the
benchmark and test surface. LuaJIT is a performance and cross-check reference
for translated Lua benchmarks, not the final authority for GScript-only
features.

Breaking changes require:

- a migration note in the release notes;
- README and examples updated in the same release;
- tests that pin the new behavior;
- a major version bump unless the affected behavior was explicitly experimental.

## Cross-Platform Policy

The current JIT implementation is ARM64-oriented and many native JIT files are
guarded for `darwin/arm64`. Release engineering must separate general runtime
support from JIT support:

- Build and test the interpreter/VM on every supported Go platform.
- Build and test the JIT only on platforms where native emission is supported.
- Make unsupported JIT platforms fail gracefully or run without JIT rather than
  crashing at startup.
- Mark release artifacts with OS/architecture and JIT support level.

Minimum release matrix target:

| Platform | Artifact | Expected support |
|---|---|---|
| `darwin/arm64` | tarball or zip with `gscript` binary | VM and ARM64 JIT |
| `linux/amd64` | tarball with `gscript` binary | VM; JIT only if backend support exists |
| `linux/arm64` | tarball with `gscript` binary | VM; JIT only if backend support exists |
| `windows/amd64` | zip with `gscript.exe` | VM; JIT only if backend support exists |

Release notes must state which artifacts include JIT support and which run in
VM-only mode.

## Release Artifacts

Each tagged release should publish:

- source archive from the tag;
- platform binaries named `gscript_vX.Y.Z_<os>_<arch>.<ext>`;
- SHA256 checksums for every binary archive;
- SBOM or dependency manifest when packaging becomes automated;
- release notes with compatibility, migration, known issues, and performance
  summary;
- benchmark JSON and Markdown reports from the release candidate run;
- optional debug symbols or diagnostic bundles when native JIT support is
  shipped.

Artifact builds should be reproducible from a clean checkout. Pass
`--version vX.Y.Z` for release-candidate and tagged-release artifact names when
the build is not running from an exact tag:

```bash
git status --short
go test ./... -count=1 -p 1 -timeout=600s
bash scripts/release_artifacts.sh --version vX.Y.Z
```

The local artifact script builds only the current platform CLI binary and writes
all outputs under `dist/` by default. It also records git, Go, and platform
metadata and writes `SHA256SUMS` for the generated binary and metadata file. It
does not tag, publish, upload, or create a release. Without `--version`, the
script uses an exact git tag when available, otherwise `dev-<commit>`.

Use `--dry-run` to inspect the planned binary, metadata, and checksum paths
before building. Dry-run mode prints the metadata content that would be written
and does not create the output directory, compile the binary, or write files:

```bash
bash scripts/release_artifacts.sh --version vX.Y.Z \
  --output-dir /tmp/gscript-release-smoke \
  --dry-run
```

The repository-owned artifact smoke check wraps that dry-run path and verifies
the planned names and metadata without touching `dist/`:

```bash
bash scripts/release_artifacts_check.sh --version vX.Y.Z
```

Use a temporary output directory for local smoke tests without touching `dist/`:

```bash
bash scripts/release_artifacts.sh --version vX.Y.Z \
  --output-dir /tmp/gscript-release-smoke
/tmp/gscript-release-smoke/gscript_vX.Y.Z_<goos>_<goarch> -e 'print("ok")'
cat /tmp/gscript-release-smoke/SHA256SUMS
```

For release-candidate evidence, run the same check with a real local build.
When `--output-dir` is omitted, it builds into an auto-created temporary
directory, verifies both `SHA256SUMS` entries against SHA256, and runs the built
CLI on `tests/01_basic.gs`:

```bash
bash scripts/release_artifacts_check.sh --version vX.Y.Z --build
```

When version metadata is wired into the binary, include tag, commit SHA, build
date, and dirty-tree status in `gscript --version` or an equivalent command.

## README And Examples

The README is part of the release surface. Before tagging:

- Update install instructions for all published artifacts.
- Document the current JIT support matrix.
- Keep the quick test and benchmark commands current.
- Include at least one interpreter-only example and one JIT-enabled example.
- Link standard-library docs and performance reports.
- Remove stale benchmark claims or clearly label them with platform and date.

Examples should be treated as compatibility tests. Any example shown in the
README or release notes should run from a clean release artifact without
requiring repository-internal paths unless it is explicitly a contributor
workflow.

## Performance Report

Every release candidate must include a performance report generated from a known
machine. The report should use the same guardrails as performance development:

```bash
python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --sort=luajit-gap \
  --json benchmarks/data/release_timing_compare.json \
  --markdown benchmarks/data/release_timing_compare.md

python3 benchmarks/strict_guard.py --runs=3 --warmup=1 --timeout=90 \
  --json benchmarks/data/release_strict_guard.json \
  --markdown benchmarks/data/release_strict_guard.md

bash benchmarks/regression_guard.sh --runs=5 --timeout=90 \
  --json benchmarks/data/release_regression_guard.json \
  --csv benchmarks/data/release_regression_guard.csv \
  --markdown benchmarks/data/release_regression_guard.md
```

The release notes should summarize:

- platform, CPU, Go version, LuaJIT version, and commit SHA;
- whether LuaJIT was available;
- checksum status across VM, default JIT, no-filter JIT, and LuaJIT references;
- top LuaJIT gaps;
- regressions against `benchmarks/data/baseline.json`;
- suspicious wins and how they were reviewed;
- whether the release updates the checked-in baseline.

Do not publish a benchmark headline unless the corresponding report includes
the command, timing source, repeat count, and checksum status.

## Machine-Checkable Release Evidence

Every release candidate must include evidence that can be re-run from the tag.
Keep this section in sync with `docs/production-readiness-checklist.md` and
`scripts/docs_check.sh`; the docs checker treats these commands and ledgers as
release-readiness contract.

Required machine gates:

```bash
bash scripts/production_check.sh --quick
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
bash scripts/docs_check.sh
bash scripts/performance_gate.sh --feature-smoke
```

The release evidence archive should also name the exact revisions of these
machine-readable ledgers:

| Evidence | Required file |
|---|---|
| Language feature matrix | `tests/feature_matrix.json` |
| Language spec contract | `docs/language-spec.md` |
| Official translated cases | `tests/language/MANIFEST.md` |
| Known official-case skips | `tests/language/KNOWN_FAILURES.md` |
| Intentional capability gaps | `tests/language/MISSING_CAPABILITIES.md` |
| Standard library contract | `docs/stdlib-contract.md` |

The release is blocked if a language-facing change updates only prose or only
tests. Syntax, semantic, stdlib, diagnostics, and host-capability changes need a
spec reference, matrix row or documented non-goal, and a release gate that can
fail in CI.

## Release Checklist

1. Start from a clean release branch and confirm no unrelated local changes.
2. Run full tests: `go test ./... -count=1 -p 1 -timeout=600s`.
3. Run compatibility and performance gates from the release candidate commit.
4. Review strict-guard checksum mismatches, skipped LuaJIT rows, and suspicious
   wins.
5. Build all release artifacts and checksums.
6. Smoke-test each artifact on its target platform or documented runner.
7. Update README, examples, release notes, and performance report links.
8. Tag `vX.Y.Z` or `vX.Y.Z-rc.N` from the exact commit that produced artifacts.
9. Publish artifacts, checksums, source archive, and performance reports.
10. If the publish-grade performance run is accepted as the new baseline, update
    `benchmarks/data/baseline.json` in a follow-up commit with the archived
    guard output.

## CI And Release Gates

The minimum release-engineering CI recipe is the command set below. It does not
publish binaries. Hosted CI should run it on every pull request, `main` push,
version tag push, and manual release-gate run.

The required CI jobs are:

| Job | Command | Purpose |
|---|---|---|
| `go test quick` | `go test ./gscript ./cmd/gscript ./internal/lexer ./internal/parser ./internal/runtime ./internal/vm -count=1` | Fast coverage for public API, CLI package, parser/runtime/VM, and core implementation packages. |
| `production_check --quick` | `bash scripts/production_check.sh --quick` | Repository-owned preflight that mirrors the quick production checklist. |
| `release matrix gate` | `go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1` | Metadata gate for feature matrix coverage, release matrix refs, official-case ledgers, and stdlib contract linkage. |

Tag-triggered runs are release gates only. Artifact packaging, checksum
generation, SBOM creation, draft-release upload, and binary publishing remain
manual until those steps have reproducible scripts and reviewed artifact
retention policy.

Future CI milestones:

- JIT workflow: run ARM64 JIT tests on a `darwin/arm64` or other supported ARM64
  runner.
- Benchmark workflow: scheduled regression guard with archived JSON/Markdown.
- Release workflow: tag-triggered artifact build, checksum generation, and
  upload after the build script is stable.
- Manual publish workflow: run release-candidate performance reports and attach
  them to the draft release.

CI must never auto-update baselines. Baseline promotion requires a reviewed
publish-grade run and an explicit commit.
