# Project Structure Audit

This audit maps the current repository layout for tools, scripts, tests,
examples, demos, and performance suites. Its purpose is to keep release-facing
assets discoverable as Leia grows into a public scripting language project.

## Current Layout

| Area | Path | Current Role | Assessment |
|---|---|---|---|
| CLI | `cmd/leia/` | Public command surface: run, eval, repl, fmt, lint, test, bench, doc, diag, ci, mod, env, config, inspect, capabilities, version. | Good public entry point. Keep developer-only binaries out of README unless routed through `leia inspect` or `leia diag`. |
| Developer dump tools | `cmd/dump/`, `cmd/dump_bytecode/` | Low-level bytecode/debug tools. | Useful internally, but should remain secondary to `leia inspect bytecode`. |
| Release scripts | `scripts/production_check.sh`, `scripts/release_artifacts*.sh`, `scripts/docs_check.sh`, `scripts/performance_gate.sh`, `scripts/diagnostics_bundle.sh` | Machine-checkable release and diagnostics gates. | Strong foundation. Public docs should point at `leia ci` and these scripts only for contributor/release workflows. |
| Misc scripts | `scripts/arch_check.sh`, `scripts/worktree_audit.sh`, `scripts/diag.sh`, `scripts/diag_summary.py` | Local developer support. | Keep documented as maintainer tools, not user-facing language tools. |
| Smoke tests | `tests/01_basic.leia` through `tests/12_advanced.leia` | Hand-written language examples and regression smoke tests. | Good beginner corpus, but should be indexed from an examples gallery. |
| Official semantics | `tests/language/` | Translated Lua semantic oracle pairs and ledgers. | Good correctness evidence. Keep separate from performance; do not present as demos. |
| Feature metadata | `tests/feature_matrix.json`, release matrix tests, stdlib contract tests. | Machine-readable support and release evidence. | Good release infrastructure. Keep in sync with docs and `leia capabilities`. |
| Examples | `examples/*.leia`, `examples/concurrency/`, `examples/data_oriented/`, `examples/embedding/`, `examples/game_engine/` | User-facing demos and smoke material. | Needs a curated `examples/README.md` before public release so live-provider demos and no-network demos are clearly separated. |
| AI-native demos | `examples/ai_native_*.leia` | Agent syntax, LLM provider, GLM live smoke, direct agent tools. | Strong differentiator, but live-provider examples must always document required secrets and have mock equivalents. |
| Data-oriented demos | `examples/data_oriented/`, `benchmarks/data_oriented_hot/` | SOA and dense-array examples plus hot-path measurement. | Good match for Odin-inspired positioning. Keep examples readable and benchmarks hot-loop focused. |
| Benchmark harness | `benchmarks/timing_compare.py`, `strict_guard.py`, `regression_guard.py`, `performance_gate_test.py`, etc. | Performance comparison, strict correctness/timing gates, and regression checks. | Mature but dense. Public docs should route users through `leia bench` / `scripts/performance_gate.sh`, not individual Python scripts first. |
| Benchmark suites | `benchmarks/suite/`, `extended/`, `variants/`, `official_hot/`, `concurrency_hot/`, `data_oriented_hot/`, Lua mirrors. | Hot-loop benchmark corpus and LuaJIT references. | Good coverage. Keep naming stable: Leia and Lua mirrors should remain paired by group/name. |
| Benchmark artifacts | `benchmarks/data/` | Baselines, latest results, strict guard outputs, history. | Treat as evidence snapshots. Release reports should copy relevant outputs into release artifacts. |
| Documentation | `docs/` | Specs, roadmaps, stdlib docs, release/security/performance/design docs, historical blog posts. | Rich but sprawling. Public docs need a user-first landing path separate from compiler engineering posts. |

## Recommended Public Taxonomy

Use this taxonomy in README, docs navigation, and release notes:

| Public Bucket | Primary Files | Audience |
|---|---|---|
| Start | `README.md`, `docs/language-spec.md`, `docs/embedding.md`, `examples/` | New users evaluating the language. |
| AI-native | `docs/ai-native-syntax-design.md`, `docs/stdlib/llm.md`, `examples/ai_native_*.leia` | Users trying the killer feature. |
| Data-oriented | `docs/data-oriented-design.md`, `docs/stdlib/soa.md`, `examples/data_oriented/`, `benchmarks/data_oriented_hot/` | Users who care about hot numeric/data workloads. |
| Tooling | `docs/tooling.md`, `cmd/leia/`, `scripts/production_check.sh` | Contributors and CI/editor integration authors. |
| Correctness | `tests/`, `docs/testing-matrix.md`, `docs/stdlib-contract.md` | Maintainers and adopters checking maturity. |
| Performance | `benchmarks/`, `docs/performance.md`, `docs/benchmark-timing-audit.md` | Performance reviewers. |
| Release | `docs/release.md`, `docs/production-readiness-checklist.md`, `docs/open-source-release-readiness.md` | Release managers and external packagers. |

## Near-Term Cleanup

1. Add `examples/README.md` that classifies examples by no-network, host
   capability, AI live-provider, data-oriented, concurrency, and embedding.
2. Add a docs landing page that separates "Use Leia" from historical JIT
   posts.
3. Keep benchmark group pairs aligned:
   `benchmarks/<group>/<name>.leia` and `benchmarks/lua_<group>/<name>.lua`.
4. Keep all live-provider examples opt-in and secret-free.
5. Prefer adding public workflows to `leia` subcommands first, then call
   scripts as implementation details.
