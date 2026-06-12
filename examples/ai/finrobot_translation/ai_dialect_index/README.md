# Generic AI Dialect Package Index Audit

This directory records a provider-free package index for the generic AI dialect
surface exercised by the FinRobot translation work. The index is intentionally
not a FinRobot syntax contract: each entry names a generic AI capability,
points to an example, a Go test, and a fixture, and records the package boundary
that owns the reusable contract.

The audit is read-only. It does not import live packages, call model providers,
touch external networks, or depend on the q runtime package.

`backend_plan.json` groups the same generic capabilities into a small set of
provider-neutral backend shapes. The plan is intentionally implementation
agnostic: it names inputs, outputs, and verification fixtures so the dialect can
later move from examples into reusable packages without adding FinRobot-specific
syntax or built-in language coupling.

`PACKAGE_BOUNDARIES.md` is the architecture note for using small Leia assembly
code to build complex AI projects. It explains that the dialect is composed
from ordinary packages rather than built-in language features, that the
packages are not FinRobot-specific, and how package composition,
provider-free replay, approval, trace, evaluation, and record/replay boundaries
compose into model, turn, tool, agent, workflow, and package-audit
responsibilities.

The docs tests parse the package list in `PACKAGE_BOUNDARIES.md` and verify
that every documented package directory exists under the checked-in
provider-free live package set.
