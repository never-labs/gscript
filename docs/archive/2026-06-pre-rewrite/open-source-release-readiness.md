# Open Source Release Readiness Draft

This draft scopes the non-language work required before presenting Leia as a
mature open source project. It intentionally does not repeat the language
feature matrix, parser/JIT implementation plan, or benchmark optimization
roadmap. The question here is narrower: if the language core is good enough,
what would still block a serious user, contributor, packager, or adopter from
trusting the project?

Related existing documents:

- [Release Engineering Roadmap](release.md) defines tags, artifacts,
  compatibility gates, and release evidence.
- [Production Readiness Checklist](production-readiness-checklist.md) defines
  release gates across runtime, tooling, security, docs, and performance.
- [Leia CLI and Toolchain Audit](cli-audit.md) defines the standalone
  command surface still needed for a language release.
- [Security and Isolation Roadmap](security.md) defines the runtime isolation
  model that public examples must use.

## Release Target

The first mature open source release should be a `v0.1.0` or `v0.1.0-rc.1`
release candidate, not a broad "1.0" claim. It should promise:

- a clear product position: embeddable Go scripting runtime plus standalone
  Leia CLI, with AI-native syntax as the differentiating demo surface;
- source availability with license, security policy, contribution process, and
  issue templates;
- reproducible local binary artifacts with checksums for at least the current
  primary platform;
- a documented support matrix that separates VM portability from JIT support;
- runnable examples that work from a clean checkout and from a release binary;
- benchmark evidence that is dated, reproducible, and tied to a commit.

Do not publish the release as "production ready" until every P0 item below has
an artifact, an owner, and a passing or explicitly deferred gate.

## P0 Release Blockers

| Area | Current evidence | Gap | Release-ready deliverable | Acceptance gate |
|---|---|---|---|---|
| Positioning and README | `README.md` has a short identity sentence and basic commands. | It does not explain who should use Leia, what is stable, how AI-native work differs from ordinary embedding, or what is explicitly experimental. | Rewrite the README into a product entry point: value proposition, 60-second install/run path, VM/JIT support matrix, AI-native demo link, embedding demo link, docs map, maturity statement, and support policy. | A new user can run one script and one embedding example from README commands without reading repository internals. |
| License and legal basics | No license file is visible in the repository root. | Open source consumers cannot legally adopt, redistribute, package, or contribute with confidence. | Add `LICENSE`, optional `NOTICE`, and a one-line license badge/statement in README. Pick the license before accepting external contributions. | `LICENSE` exists at repo root and release notes name the license. |
| Install and distribution | `docs/release.md` documents `scripts/release_artifacts.sh`; README only shows `go run`. | Users have no stable install path, no binary naming table, no checksum instructions, and no upgrade path. | Document `go install`, source build, local release tarball, checksum verification, PATH setup, and unsupported package managers. Publish at least one signed or checksummed binary archive per release candidate. | `bash scripts/release_artifacts_check.sh --build` passes and README install commands are copy-pasteable. |
| Version and compatibility strategy | `docs/release.md` defines semver, compatibility layers, and release matrix. | The policy is not surfaced where users decide adoption; no deprecation policy or experimental marker vocabulary is prominent. | Add README/release-notes summary: stable, experimental, internal, deprecated. Define how `v0.x` can break and what needs a migration note. | `leia version --json` output is documented and release notes include compatibility notes for every public change. |
| Documentation entry point | `docs/index.html` is a JIT blog index. Many docs exist, but the user path is not curated. | A user lands in performance posts instead of install, tutorial, language, stdlib, embedding, tooling, and security docs. | Add a docs landing page or restructure `docs/index.html` into two paths: "Use Leia" and "Engineering Notes". Keep the blog/archive discoverable but not primary. | Docs home links to install, quickstart, language spec, stdlib, embedding, tooling, security, release, examples, and benchmark methodology. |
| Examples and demos | `examples/` exists and README has tiny snippets. | Examples are not packaged as a coherent gallery; AI-native examples likely require local secrets and provider setup. | Create `examples/README.md` with tiers: no-network basics, embedding, stdlib, data-oriented, AI-native. Each example must list required env vars and expected output. | `leia test examples` or an equivalent smoke command runs all no-network examples; AI examples have dry-run/mock-provider coverage. |
| Benchmark evidence | `docs/performance.md`, `docs/perf/*`, and benchmark harnesses are strong. | Public claims need a current release report, not scattered engineering posts or local anecdotes. | Generate `docs/perf/release-v0.1.0-rc.1.md` from `timing_compare.py`, `strict_guard.py`, and `performance_gate.sh`, including platform, CPU, Go version, LuaJIT availability, commit, checksums, and caveats. | `bash scripts/performance_gate.sh --full --out-dir <release-artifacts>` passes or release notes clearly mark incomplete LuaJIT/runner coverage. |
| Security policy | `docs/security.md` is a roadmap. | There is no repo-root `SECURITY.md`, vulnerability reporting channel, supported versions table, or public default-sandbox statement. | Add `SECURITY.md`: supported versions, reporting address/process, disclosure timeline, sandbox status, known unsafe modules, and secret-handling policy for AI providers. | README links `SECURITY.md`; every AI/network/process example uses safe defaults or labels required trust. |
| Contribution governance | No visible `CONTRIBUTING.md`, code of conduct, maintainer policy, or decision process. | External contributors do not know how to propose changes, run tests, format code, or understand performance guard expectations. | Add `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, maintainer/review policy, DCO/CLA decision, branch/tag policy, and benchmark evidence requirements for performance PRs. | A first-time contributor can find setup, test, docs, issue, PR, and review expectations from README. |
| CI and release automation | Scripts exist: `production_check.sh`, `docs_check.sh`, `release_artifacts_check.sh`, `performance_gate.sh`. No `.github/workflows` directory is visible. | Release gates are local knowledge; public contributors cannot see required CI status or tag behavior. | Add hosted CI workflows for quick tests, docs check, release matrix gate, and artifact dry-run. Add manual release workflow only after artifact retention and secret policy are reviewed. | PRs run quick gate and docs gate; tags run release smoke without auto-publishing baselines. |
| Package management and ecosystem | `docs/tooling.md` describes early `leia mod` and no networked registry. | Language users lack manifest/lockfile docs, module naming rules, version constraints, and package discovery. | For `v0.1.0`, explicitly scope package management as local-only. Document `leia.toml`, `require()` resolution, vendoring/non-goals, and future registry design. | README and docs do not imply a package registry exists; local module examples work from a clean checkout. |
| Community templates | No `.github` directory is visible. | Issues and PRs will arrive without repro data, platform info, security routing, or benchmark artifacts. | Add issue templates for bug, performance regression, language proposal, docs issue, security redirect, and feature request. Add PR template with required test/docs/perf evidence. | New issues collect OS/arch, Go version, Leia version, command, expected/actual output, and minimal repro. |
| AI-native killer demo | `docs/ai-native-syntax-design.md` and `docs/agent-spec.md` define the direction. | The public release needs one real demo that justifies a new language rather than a Go library wrapper. | Build a flagship demo: a constrained AI agent that reads project docs, runs `leia check`/tests, uses tools, respects budget/security policy, and produces a patch or release report. Include mock-provider mode and live-provider mode. | Demo runs offline with mock responses in CI and live with documented `OPENAI_API_KEY` or compatible provider env vars. |

## P1 Release Hardening

P1 items should be in progress before `v0.1.0`, but they can ship in follow-up
patch/minor releases if P0 is honest about the limits.

| Area | Deliverable | Acceptance gate |
|---|---|---|
| Docs site quality | Navigation, search, generated CLI reference, generated stdlib index, old post archive, canonical URLs. | A user can answer install, syntax, stdlib, embedding, CLI, security, benchmark, and contribution questions from docs navigation. |
| Release notes template | `docs/release-notes-template.md` or `.github/release.yml` with sections for compatibility, install, platform, benchmark, security, known issues, contributors. | Every release candidate uses the same template and links evidence artifacts. |
| Binary provenance | SBOM, build metadata, checksum verification docs, optional signing. | Release artifact metadata includes commit, tag, Go version, OS/arch, dirty status, and SHA256. |
| Editor onboarding | Syntax highlighting, file extension docs, minimal LSP/diagnostic JSON plan. | README names current editor support honestly and links setup. |
| Contributor performance workflow | A short "how to prove a performance PR" doc. | Performance PR template asks for strict guard, timing report, suspicious-win review, and platform details. |
| Public roadmap | `docs/roadmap.md` oriented toward users, not only compiler engineering. | Roadmap names shipped, next, later, and non-goals with dates or release targets. |

## Execution Plan

### Week 1: Make The Project Legible

1. Decide license and add `LICENSE`.
2. Rewrite README around positioning, install, quickstart, docs map, maturity,
   and safety.
3. Add `SECURITY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, and issue/PR
   templates.
4. Add a docs landing page that separates user docs from compiler/JIT posts.
5. Create `examples/README.md` and classify every example by dependency and
   expected output.

Exit criteria:

```bash
bash scripts/docs_check.sh
go run ./cmd/leia version --json
go run ./cmd/leia help
```

### Week 2: Make The Release Reproducible

1. Document install and upgrade paths in README.
2. Run release artifact smoke with a version override.
3. Generate a release-candidate benchmark report and link it from release notes.
4. Add CI workflows for quick Go tests, docs check, release matrix gate, and
   artifact dry-run.
5. Add release notes template and known-issues section.

Exit criteria:

```bash
bash scripts/release_artifacts_check.sh --version v0.1.0-rc.1 --build
bash scripts/production_check.sh --quick
go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1
```

### Week 3: Make The Demo Convincing

1. Build the AI-native release assistant demo with mock and live providers.
2. Use `SecuritySandbox` or the documented safe equivalent by default.
3. Add a scripted demo runner that records expected transcript/output.
4. Link the demo from README and docs home.
5. Add CI smoke for mock-provider mode.

Exit criteria:

```bash
go run ./cmd/leia examples/ai_native/release_assistant_mock.leia
go run ./cmd/leia check examples/ai_native/release_assistant_mock.leia
bash scripts/performance_gate.sh --feature-smoke
```

## README Rewrite Requirements

The README should stop being only a command scratchpad. Minimum structure:

1. What Leia is.
2. Why it exists: embeddable scripting, Lua-like runtime semantics, Go-hosted
   integration, ARM64 JIT path, and AI-native agent syntax.
3. Maturity statement: what is stable, experimental, and unsupported.
4. Install: binary archive, checksum, `go install`, source build.
5. Quickstart: run a script, run an AI-native mock demo, embed in Go.
6. Platform support: VM and JIT matrix.
7. Docs map: language, stdlib, embedding, tooling, security, performance,
   release, contribution.
8. Community and security: issue templates, contribution guide, vulnerability
   reporting.

Avoid benchmark headlines in the README unless they name the date, commit,
machine, LuaJIT availability, and report link.

## Documentation Site Requirements

The docs home should prioritize user journeys:

| Path | Required pages |
|---|---|
| Start | install, quickstart, examples, FAQ |
| Write Leia | language spec, stdlib, AI-native syntax, tooling |
| Embed Leia | Go API, host functions, sandboxing, cancellation, stdio hooks |
| Operate Leia | security, diagnostics, release, compatibility, benchmark methodology |
| Contribute | contributing, test matrix, performance workflow, roadmap |
| Archive | JIT engineering posts and historical benchmark notes |

The current JIT post index can remain, but it should not be the default mental
model for a new user evaluating the language.

## Package And Ecosystem Scope

For the first release, do not imply a public registry exists. The honest scope
is:

- local modules loaded by documented resolution rules;
- a project manifest such as `leia.toml` if the command already supports it;
- vendored examples inside the repo;
- no remote dependency resolution unless it is implemented, cached, audited,
  and security-reviewed;
- package registry design as a roadmap item, not a release promise.

Minimum ecosystem docs:

- module path naming rules;
- import/require resolution order;
- lockfile status;
- cache directory status;
- how to publish examples today;
- how future packages will handle versions, checksums, yanks, and security
  advisories.

## AI-Native Killer Demo Requirements

The flagship demo should prove something specific: Leia can express an agent
workflow with less host glue while still preserving tool boundaries, budgets,
and sandbox policy.

Recommended demo: `examples/ai_native/release_assistant.leia`.

Behavior:

1. Reads the project README and selected docs.
2. Runs `leia version --json`, `leia check`, and optionally
   `scripts/docs_check.sh` through approved tools.
3. Produces a release readiness report with missing evidence and concrete next
   commands.
4. In live mode, calls an OpenAI-compatible or Anthropic-compatible provider.
5. In mock mode, returns deterministic responses for CI.

Required safety shape:

- default to no network and no process execution unless explicitly enabled;
- keep API keys in environment variables or host secret references only;
- enforce turn, tool-call, token, and wall-time budgets;
- log tool calls and denied capabilities;
- use fixture docs for CI so the demo does not depend on external services.

Acceptance gates:

```bash
go run ./cmd/leia examples/ai_native/release_assistant_mock.leia
go run ./cmd/leia examples/ai_native/release_assistant_mock.leia --json
```

Live-provider mode should be documented separately and must never be required
for default CI.

## Release Decision Checklist

Before publishing a release candidate, answer each item with a link to an
artifact:

- What is the license?
- What exact commit produced the release?
- Which binaries were built, and where are their checksums?
- Which platforms are VM-only, and which support JIT?
- Which public APIs and CLI commands are stable?
- Which features are experimental?
- Which examples are guaranteed to run without network access?
- Which benchmark report backs performance claims?
- How does a user report a vulnerability?
- How does a contributor run the required checks?
- Which package-management features exist today?
- What is the one AI-native demo users should try first?

If any answer is "tribal knowledge", the release is not mature open source yet.
