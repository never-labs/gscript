# Production Readiness Checklist

This checklist turns the production roadmap into release gates. It is intended
to be run before a public tag, and to stay useful while Leia is still moving
quickly.

## Scope

Leia is considered production-ready only when it is usable in both modes:

- as an embedded Go scripting runtime through the public `leia` package;
- as a standalone language through `cmd/leia` and the surrounding toolchain.

The checklist is grouped by release gate rather than implementation package.
Each gate names the owning roadmap doc, the current command or artifact, and the
minimum condition for a release candidate.

## Release Gates

| Gate | Roadmap doc | Current evidence | Release condition |
|---|---|---|---|
| Language semantics | `docs/language-spec.md` | `tests/feature_matrix.json`, `tests/language/MANIFEST.md`, `tests/language/MISSING_CAPABILITIES.md` | Stable behavior is documented, intentional Lua differences are explicit, and known unsupported features have a decision. |
| Go embedding API | `docs/embedding.md` | `leia/*.go`, `leia/leia_test.go` | A Go host can compile, run, call functions, bind host functions, convert values, cancel execution, and configure libraries without importing `internal/*`. |
| Standard library | `docs/stdlib.md` | `internal/runtime/stdlib_*.go`, official translated stdlib cases | Each exported module has documented contracts, error behavior, permission requirements, and tests. |
| Security and isolation | `docs/security.md` | sandbox options, timeout/cancel tests, permission tests | Untrusted scripts can be run with CPU, wall-time, memory, recursion, IO, network, process, and module access bounded by host policy. |
| Tooling | `docs/tooling.md` | `cmd/leia`, `cmd/dump`, `benchmarks/*.py`, test harnesses | Users have documented commands for run, test, format/lint decisions, benchmark, diagnose, and debug workflows. |
| Performance and JIT | `docs/performance.md` | `benchmarks/timing_compare.py`, `benchmarks/strict_guard.py`, benchmark history | Optimizations are guarded by correctness oracles, no benchmark-specific kernels are accepted, and release reports separate hot-loop timing from startup noise. |
| Release engineering | `docs/release.md` | git tags, README, benchmark reports | Versioning, artifacts, compatibility policy, supported platforms, and release notes are complete. |

Supporting audit docs:

- `docs/api-audit.md`: public Go API and internal/public boundary.
- `docs/test-matrix.md`: correctness and oracle coverage map.
- `docs/benchmark-timing-audit.md`: timing-source and hot-loop measurement
  risks.
- `docs/cli-audit.md`: command-line and standalone tooling gaps.

## Required Commands

These are the current best known commands. The quick subset is the repository
CI recipe for pull requests, `main` pushes, version tag pushes, and manual
release-gate runs; hosted workflow files can call the same commands when the
publishing credentials permit workflow updates.

The local release-gate entrypoint is:

```bash
scripts/production_check.sh
```

For handoff or failure triage, collect a local diagnostics bundle:

```bash
scripts/diagnostics_bundle.sh
```

The bundle defaults to the git-ignored `diagnostics/<timestamp>/` directory and
captures git revision/status, Go environment summary, quick Go test logs, and
quick benchmark/strict-guard summaries when those local tools are available.

Use `scripts/production_check.sh --quick` for a short preflight that runs the
core Go package tests, feature/integration checks, release matrix metadata
gate, the standard-library contract check, and the documentation reference gate
when `scripts/docs_check.sh` is present, without the long benchmark passes. The
default `--full` mode runs the correctness gates, release matrix metadata gate,
the documentation reference gate when available, the release smoke, and the
repeatable performance gate through
`scripts/performance_gate.sh --full`. Use
`scripts/production_check.sh --list` to print the available command subset for
the current checkout. Add `--out-dir DIR` to write the resolved plan and command
logs to a local artifact directory; this works with `--quick`, default `--full`,
and `--list` runs, and leaving it unset preserves the normal console-only
behavior. If LuaJIT is unavailable, benchmark commands that support it are run
with `--no-luajit`; if optional tools such as `pytest` are absent, the script
reports that clearly instead of failing before the Go gates run.

The integrated documentation reference gate is:

```bash
scripts/docs_check.sh
```

It checks README and `docs/**/*.md` relative `.md` links, verifies fenced code
blocks that mention `production_check`, `performance_gate`,
`diagnostics_bundle`, or `release_artifacts` point at executable scripts, and
keeps the AI-native language gate plus machine-checkable release evidence
sections present.

The short hot-path feature performance guard is:

```bash
bash scripts/performance_gate.sh --feature-smoke
```

It focuses on newer language features, including AI-native official hot cases,
concurrency, stdlib host dispatch, and data-oriented SOA paths. Use it before
and after language-facing runtime changes when a full benchmark sweep would slow
down iteration.

The release artifact smoke is:

```bash
bash scripts/release_artifacts_check.sh
```

By default it dry-runs `scripts/release_artifacts.sh` and verifies the planned
binary, metadata, checksum paths, and metadata fields without building or
writing `dist/`. Before publishing a tag or release candidate, run it with
`--build` to build into a temporary directory, verify the generated
`SHA256SUMS`, and execute the built CLI against `tests/01_basic.leia`.

### CI Quick Gates

The minimum CI gate is intentionally small and does not publish release
artifacts:

```bash
go test ./leia ./cmd/leia ./internal/lexer ./internal/parser ./internal/runtime ./internal/vm -count=1
bash scripts/production_check.sh --quick
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
```

These commands cover fast Go correctness, the repository-owned production
preflight including documentation link/script-reference drift when the checker
is available, and the release matrix metadata gate. A version tag is not
releasable until the full local checklist below has also been run and archived
with the release evidence.

### AI-Native Language Gates

AI-facing language work is release-blocking when it changes syntax, semantics,
tool-visible diagnostics, or host capability boundaries. The release candidate
must keep the human spec and machine ledgers aligned:

```bash
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
bash scripts/docs_check.sh
```

Expected result:

- every stable `docs/language-spec.md` section is referenced by
  `tests/feature_matrix.json` and has a semantic or official-case gate;
- `tests/language/MANIFEST.md`,
  `tests/language/KNOWN_FAILURES.md`, and
  `tests/language/MISSING_CAPABILITIES.md` classify translated
  oracle coverage and intentional gaps;
- `docs/stdlib-contract.md` modules remain linked from official coverage or a
  documented capability entry;
- this checklist and `docs/release.md` keep the machine-checkable commands that
  `scripts/docs_check.sh` requires.

### Correctness

```bash
go test ./... -count=1
```

Expected result:

- all package tests pass;
- official translated cases covered by Go tests remain green;
- method JIT oracle, OpSpec, dependency, and boundary tests pass.

### Feature Matrix

```bash
go test ./tests -run 'TestFeatureMatrix|TestIntegration' -count=1
```

Expected result:

- `tests/feature_matrix.json` matches implemented language features;
- integration `.leia` programs execute successfully;
- changes to feature support are reflected in the matrix and docs.

### Official Lua Compatibility Surface

```bash
go test ./tests -run Official -count=1
```

Expected result:

- translated official cases pass or are listed in
  `tests/language/KNOWN_FAILURES.md`;
- `tests/language/MISSING_CAPABILITIES.md` records deliberate
  Leia differences and future host-language-shaped capabilities.

### Performance Gate

```bash
bash scripts/performance_gate.sh --full
```

Expected result:

- every release-gate benchmark has a comparable Leia cell;
- official hot cases do not report unexplained `low_resolution` results;
- any wall-time fallback is marked as startup-sensitive, not hot-loop evidence;
- LuaJIT gaps are triaged before release notes claim a performance win.
- suite, extended, and variant checksums match;
- VM, default JIT, and no-filter JIT agree on output;
- suspicious benchmark wins are reviewed for runtime-discovered specialization
  rather than benchmark-specific protocol matching.

`scripts/production_check.sh` runs this command in default `--full` mode and
adds `--no-luajit` automatically when `luajit` is unavailable. Quick mode does
not run the performance gate.

### Release Smoke

```bash
go run ./cmd/leia tests/01_basic.leia
go run ./cmd/leia -jit benchmarks/table/table_field_access.leia
go run ./cmd/dump_bytecode tests/01_basic.leia
bash scripts/release_artifacts_check.sh --build
```

Expected result:

- CLI entry points build from a clean checkout;
- JIT can be enabled explicitly;
- diagnostic commands produce useful output without depending on local temp
  state.
- the release artifact script has a validated dry-run plan, real local build,
  and matching SHA256 checksums before artifact publication.

## Documentation Checklist

Before a release candidate, confirm:

- `README.md` states what Leia is, install/run commands, supported
  platforms, and links to production docs;
- `docs/language-spec.md` names intentional Lua differences and non-goals;
- `docs/embedding.md` documents every stable public Go API;
- `docs/stdlib.md` documents permission requirements for host-backed modules;
- `docs/security.md` documents default-deny behavior and resource limits;
- `docs/tooling.md` lists stable CLI commands and diagnostic workflows;
- `docs/performance.md` explains benchmark methodology and guardrails;
- `docs/release.md` defines versioning, artifacts, and compatibility policy.
- `scripts/docs_check.sh` passes for README and `docs/**/*.md`.

## API Checklist

The public `leia` package should expose production concepts with stable names:

- engine or VM construction with options;
- compile and run APIs that accept source name and source text;
- function lookup and call APIs;
- conversion between Go values and script values;
- host function binding with panic-to-error isolation;
- module/library loader configuration;
- context cancellation and wall-time budget;
- structured errors with source location and stack trace;
- explicit JIT enablement and platform fallback behavior.

Do not expose:

- bytecode internals;
- method-JIT IR or pass internals;
- benchmark-only tuning switches;
- raw VM frame, stack, or register layout;
- unsafe host pointers or implementation-owned caches.

## Security Checklist

Default embedded execution should be safe for semi-trusted scripts:

- filesystem, network, process, environment, and module loading are denied
  unless explicitly granted;
- CPU and wall-time limits are enforced in interpreter, VM, and JIT paths;
- recursion, coroutine resume depth, metamethod depth, and host callback depth
  are bounded;
- memory-sensitive structures have table/string/result-size limits;
- host panics become structured script errors;
- denied operations are observable through diagnostics or audit hooks.

## Performance Checklist

Before accepting a JIT optimization:

- add or identify a correctness oracle for the optimized behavior;
- record the fallback path and deopt reason;
- show current-vs-HEAD-vs-LuaJIT timing with stable timing source;
- check no unrelated benchmark regresses beyond the accepted threshold;
- avoid static benchmark names, fixed problem sizes, or protocol-specific
  kernels unless the feature is a documented general runtime specialization;
- update diagnostics if the optimization introduces new guards, exits, or
  specialization states.

## Open P0 Decisions

These decisions should be closed before the next implementation-heavy phase:

1. Which `leia` package APIs become v1-stable?
2. What is the default sandbox profile for embedded execution?
3. Which standard-library modules are enabled by default in standalone CLI vs
   embedded hosts?
4. Should the official translated Lua cases be compatibility tests, regression
   tests, or both?
5. Which benchmark groups block release, and which remain advisory?
6. What platforms are supported when method JIT is unavailable?
7. What generated reports are release artifacts, and where are they stored?
