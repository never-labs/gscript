# Release Process

Leia releases need machine-checkable evidence, not hand-written claims.

## Machine-Checkable Release Evidence

Run at least:

```bash
go run ./cmd/leia ci release --release-version vX.Y.Z --list
scripts/run.sh production --full --release-profile --release-version vX.Y.Z
go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1
scripts/run.sh docs
scripts/run.sh perf --full
scripts/run.sh release-dist --require-goreleaser
scripts/run.sh release-check --build
```

Machine-readable release evidence:

```bash
go run ./cmd/leia capabilities --json
bash scripts/diagnostics_bundle.sh --output /tmp/leia-diag --skip-go-tests --skip-benchmarks --json
scripts/run.sh production --full --release-profile --release-version vX.Y.Z --list --out-dir /tmp/leia-release-plan
scripts/run.sh production --full --release-profile --release-version vX.Y.Z --list --json
go run ./cmd/leia doc check --json
bash scripts/q_conformance_gate.sh --scope core --bench smoke --json
LEIA_SKIP_TIMING_COMPARE=1 bash benchmarks/q_performance_suite.sh > /tmp/leia-q-perf-output.txt
python3 benchmarks/q_perf_report.py --from-output /tmp/leia-q-perf-output.txt --check --json /tmp/leia-q-perf-report.json --markdown /tmp/leia-q-perf-report.md
scripts/run.sh editor --json
bash scripts/public_release_blockers_check.sh --json
bash scripts/release_notes_check.sh --json --version vX.Y.Z
scripts/run.sh release-dist --json
bash scripts/install.sh --version vX.Y.Z --os darwin --arch arm64 --bin-dir /tmp/leia-bin --dry-run --json
bash scripts/release_artifacts.sh --dry-run --version vX.Y.Z --json
bash scripts/release_artifacts_check.sh --json --version vX.Y.Z
```

`leia capabilities --json` includes `tooling.report_count` and
`tooling.reports`, the registry of CLI and release-script JSON reports. Each
entry advertises `status_field`, `scalar_fields`, `count_fields`, and
`collection_fields`, and `collection_item_fields` when the report exposes
machine-readable release evidence. Nested fields use dotted JSON paths; `[]`
marks per-item array paths.
`scripts/production_check.sh --out-dir DIR` writes `plan.txt`, `plan.json`,
and `commands.log` so the resolved release plan can be archived with both
human-readable and machine-readable evidence. The JSON plan includes
`output_dir` when an artifact directory is requested, plus
`release_critical_runs`, `skipped_check_details`, and
`release_critical_skip_details` so release automation can distinguish required
gates from optional local checks without scraping command text.

`leia ci release` delegates to the same production release profile. That
profile is the release validation source of truth: correctness, documentation,
performance, q conformance, language conformance, public blockers,
distribution configuration, and local artifact installation evidence are all
listed there. Documentation evidence is produced by `leia doc check --json`
inside that profile.

The release evidence should cite:

- `docs/spec/index.md`
- feature coverage records under `tests/`
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
- `docs/reference/scientific/index.md`
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
`scripts/public_release_blockers_check.sh` reports each unresolved release
decision with its area and the short action recorded in that table.
Its JSON report includes `blocker_count` plus kind-specific counts:
`missing_file_count`, `release_decision_count`, `stale_text_count`,
`unconfirmed_policy_count`, `missing_guidance_count`, and
`missing_doc_snippet_count`. It also exposes `open_blocker_count`,
`blocker_status_count`, `blocker_statuses`, and `blocker_status_details` so
dashboards can summarize decision state without walking every detail row. Use
`blocker_details[].kind` for dashboards that need to group the exact unresolved
work.

Distribution checks are split between local artifacts and hosted workflow
presence. The local check validates GoReleaser metadata, the install script
dry-run combinations, and local `file://` tar.gz/zip install fixtures even when
GitHub workflow files are intentionally absent:

```bash
scripts/public_release_blockers_check.sh --require-resolved
scripts/run.sh release-dist --require-goreleaser
bash scripts/install.sh --version v0.1.0 --os darwin --arch arm64 --dry-run
bash scripts/install.sh --version v0.1.0 --base-url file:///tmp/leia-release --bin-dir /tmp/leia-bin
```

Release archives must include both executables:

- `leia`, the CLI and script runner;
- `leia-lsp`, the shared language server used by editor integrations.

`scripts/install.sh --dry-run --json` reports `install_entries` so automation
can map each executable role to the exact install path.
`scripts/release_artifacts.sh --dry-run --json` reports `artifact_entries` for
the same role/name/path mapping before files are written.

Use [`notes-template.md`](notes-template.md) for release candidates and public
tags. Candidate notes live under [`notes/`](notes/) as `vX.Y.Z.md` so
compatibility, security, performance, validation, and artifact evidence are
recorded consistently and can be passed to GoReleaser.

## Release-Critical Gates

`scripts/production_check.sh --full --release-profile` treats these gates as
release-critical:

| Gate | Evidence |
|---|---|
| Correctness | Go tests, release matrix, spec examples, and stdlib contracts. |
| Architecture Health | methodjit file size, pass-pipeline, debt marker, and test-gap scan. |
| Manifest Coverage | Test and benchmark manifest coverage. |
| Module Path Gate | Published module path validation. |
| Shell Script Syntax | Bash syntax parsing for release and benchmark scripts. |
| Documentation References | Markdown links, spec HTML, generated references, and runnable examples. |
| Editor Assets | TextMate, tree-sitter, VS Code, and editor smoke checks. |
| Performance Gate | LuaJIT-class timing and strict performance evidence. |
| Q Performance Gate | q benchmark report generation and threshold checks. |
| Language Conformance Surface | Language conformance inventory. |
| Q Conformance Gate | q language, example, and benchmark conformance. |
| Release Smoke | Release-profile smoke checks. |
| CLI Experience | CLI examples and user-facing command checks. |
| Public Release Blockers | License, security, platform, channel, signing, and compatibility decisions. |
| Release Distribution | GoReleaser config, workflows, install targets, and local install fixtures. |
| Release Notes | Candidate notes for the tag, including all archive targets and checksums. |
| Release Artifacts | Local artifact build, tag, cleanliness, and install archive checks. |

`scripts/release_distribution_check.sh --json` reports
`failure_kind_count`, `failure_count`, `workflow_count`, and
`install_target_count`. Its `install_target_details` field splits each target
into `goos` and `goarch`, and its `failure_kinds` and `failure_details` fields
make missing workflow, install-plan, fixture, and local tool failures
machine-readable.
`scripts/release_artifacts_check.sh --json` uses the same failure fields for
version, tag, clean-worktree, checksum, artifact, and local install failures,
and reports `artifact_entries` for the verified release artifact roles.
`scripts/release_snapshot_install_check.sh --json` verifies the GoReleaser
snapshot archive through `scripts/install.sh` with a staged local `file://`
release directory.
`scripts/arch_check.sh --json` reports methodjit source/test size,
`large_file_details`, `debt_marker_details`, and `missing_test_files` so
architecture debt is visible to release dashboards.
`scripts/site_check.sh --json` reports rendered-site HTML, local link, asset,
fragment-anchor, and failure details after the GitHub Pages build.

## Release Compatibility Checklist

Every public tag must have release notes. The notes must identify:

- stable behavior covered by the specification, feature coverage, and release
  validation;
- experimental behavior that can change or disappear without compatibility
  guarantees;
- implementation-defined behavior that depends on host capabilities, execution
  mode, platform, provider, or build configuration;
- tested OS/architecture combinations and Go version;
- execution modes used for validation, including interpreter, VM, and any JIT
  coverage;
- disabled capabilities, live providers, or external integrations;
- release artifacts, including `leia`, `leia-lsp`, and SHA256 checksums;
- evidence links for the spec, feature coverage, security reference, platform
  reference, and performance validation.

`scripts/release_notes_check.sh --json --version vX.Y.Z` reports
`checked_file_count`, `required_artifact_count`, `artifact_checksum_count`,
`failure_kind_count`, and `failure_count`. Its `checked_file_details` and
`required_artifact_details` fields expose checked file roles, existence, and
artifact checksum status. Its `failure_kinds` and `failure_details` fields make
missing files, missing required text, template placeholders, and missing
checksums machine-groupable.

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
