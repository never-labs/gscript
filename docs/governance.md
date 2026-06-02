# Governance

Leia is currently maintained as a small maintainer-led project. The goal of
this document is to make decisions predictable without adding heavy process
before the community needs it.

## Maintainer Responsibilities

Maintainers are responsible for:

- Keeping the language specification, implementation, tests, documentation, and
  examples aligned.
- Protecting compatibility for users embedding Leia in Go programs.
- Rejecting changes that weaken sandboxing, capability checks, or resource
  budgets without a clear replacement.
- Requiring performance evidence for runtime, VM, JIT, or standard-library hot
  path changes.
- Keeping public discussion focused on reproducible behavior and explicit
  tradeoffs.

## Decision Model

Most changes can be accepted through ordinary pull requests. A maintainer may
merge a change after review when it is covered by tests and fits the current
spec.

Use the RFC process for changes that affect stable contracts:

- New syntax or grammar.
- Semantic changes to existing syntax.
- Public Go API changes.
- Standard-library module additions, removals, or capability changes.
- Package manager behavior.
- AI-native agent/evaluate semantics.
- Compatibility-breaking behavior.

See [RFC process](contributing/rfcs.md).

## Compatibility

Leia is not yet at a v1.0 stability point, but contributors should still prefer
explicit migration paths over surprise breakage. Before v1.0, breaking changes
are allowed when they make the language simpler, safer, or more coherent. After
v1.0, breaking changes should go through an RFC and a documented deprecation
period.

See [Deprecation policy](contributing/deprecations.md).

## Release Readiness

A release candidate should have:

- Passing local production checks.
- Current spec HTML generated from `docs/spec`.
- CLI and standard-library references regenerated.
- Updated examples for any new public feature.
- Performance evidence for runtime-sensitive changes.
- Security review notes for host access, LLM providers, Go bindings, or module
  loading changes.

## Maintainer Metadata

GitHub CODEOWNERS should be added once the repository has a stable public
maintainer user or team name. Do not add placeholder owners: invalid CODEOWNERS
files create false confidence and noisy GitHub diagnostics.

