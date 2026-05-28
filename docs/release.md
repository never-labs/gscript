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

Artifact builds should be reproducible from a clean checkout:

```bash
git status --short
go test ./... -count=1 -p 1 -timeout=600s
go build -trimpath -ldflags="-s -w" -o dist/gscript ./cmd/gscript
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

## CI Roadmap

There is no `.github` workflow in this checkout, so CI should be added as a
release-engineering milestone:

- PR workflow: format/build/test on supported host platforms.
- JIT workflow: run ARM64 JIT tests on a `darwin/arm64` or other supported ARM64
  runner.
- Benchmark workflow: scheduled regression guard with archived JSON/Markdown.
- Release workflow: tag-triggered artifact build, checksum generation, and
  upload.
- Manual publish workflow: run release-candidate performance reports and attach
  them to the draft release.

CI should never auto-update baselines. Baseline promotion requires a reviewed
publish-grade run and an explicit commit.
