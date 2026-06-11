# Generic AI Dialect Package Boundaries

This document records the reusable package boundary architecture behind the
generic AI dialect index. The FinRobot translation examples are the audit source
that exposed the missing boundaries, but the boundaries below are generic AI
application packages: they do not define FinRobot syntax, import FinRobot
dependencies, require live model providers, or depend on built-in language
features.

The dialect is package-composed. A program builds AI behavior by selecting
provider-neutral packages for model resolution, turn execution, tool contracts,
agent loops, workflow orchestration, evaluation, replay, trace emission,
approval policy, and package-boundary auditing. The host language only loads
packages, schemas, fixtures, manifests, and tests. It does not need a special
AI keyword, a privileged runtime hook, or a FinRobot-specific command surface.

## Non-Goals

- Not a built-in language feature: the dialect is represented by checked-in
  package manifests, contract JSON, fixtures, examples, and tests.
- Not a FinRobot-only surface: the package names and capability IDs use
  `generic.ai.*` semantics and avoid vendor, provider, or application-specific
  syntax.
- Not a live provider integration: the default execution mode is deterministic
  fixture replay with provider credentials and live network access disabled.
- Not q runtime work: these documents and docs tests only inspect AI example
  package directories.

## Boundary Packages

The following checked-in package directories are the package boundary set for
the generic AI dialect. Each directory owns a package manifest plus schemas,
contracts, and fixtures for its boundary.

<!-- ai-dialect-boundary-package-list:start -->
- model: `examples/ai/finrobot_translation/live_packages/generic_model_registry`
- turn: `examples/ai/finrobot_translation/live_packages/generic_turn_runner`
- tool: `examples/ai/finrobot_translation/live_packages/generic_tool_contracts`
- agent: `examples/ai/finrobot_translation/live_packages/generic_agent_runner`
- workflow: `examples/ai/finrobot_translation/live_packages/generic_workflow_orchestrator`
- eval: `examples/ai/finrobot_translation/live_packages/generic_evaluation_harness`
- replay: `examples/ai/finrobot_translation/live_packages/generic_record_replay`
- trace: `examples/ai/finrobot_translation/live_packages/generic_trace_events`
- approval: `examples/ai/finrobot_translation/live_packages/generic_approval_policy`
- package-audit: `examples/ai/finrobot_translation/live_packages/generic_package_boundary_auditor`
<!-- ai-dialect-boundary-package-list:end -->

## Composition Model

Complex applications are built by combining these packages along explicit data
contracts:

- model resolves a provider-neutral alias into replay-safe execution
  descriptors, policy flags, and redaction rules.
- turn consumes model descriptors, structured messages, tool-call envelopes,
  replay fixtures, and output schemas to produce one deterministic model-turn
  result.
- tool validates a tool request, checks approval and capability policy, runs or
  replays the implementation, and returns a normalized result envelope.
- agent joins turns, tools, memory snapshots, structured output validation, and
  loop budgets into a declarative agent run.
- workflow coordinates multiple agents or stages, handoffs, retries, cache
  policy, artifacts, and trace emission.
- eval runs replayed cases over agents or workflows and produces metrics,
  findings, and golden-gate decisions without live judges.
- replay records and matches deterministic model, tool, approval, artifact, and
  trace fixtures so tests stay provider-free.
- trace emits normalized events for turns, streams, tools, approvals, replays,
  artifacts, and workflow stages.
- approval applies policy decisions before tool calls, workflow actions, and
  capability-gated operations.
- package-audit checks package manifests, fixture indexes, examples, provider
  gates, and missing-boundary records so the dialect remains package-owned.

The composition is intentionally orthogonal. A single-turn assistant can use
only model, turn, replay, and trace. A tool-using analyst adds tool and
approval. A multi-step product workflow adds agent and workflow. Regression
gates add eval. Release checks add package-audit. None of these combinations
requires changing the host language or specializing the dialect for one
application domain.

## Example Assembly

1. Resolve a model alias through the model package into a replay-safe execution
   descriptor.
2. Execute a turn package request with structured messages, output schema, and
   replay fixture keys.
3. Route any requested tool through tool contracts and approval policy before
   returning a normalized tool result.
4. Let an agent package repeat turns and tool calls under loop, budget, memory,
   and structured-output rules.
5. Use workflow orchestration when several agents or stages need handoffs,
   retries, cache decisions, and artifact production.
6. Emit trace events for every turn, tool call, approval, replay match, and
   workflow stage.
7. Run eval packages against the same replay fixtures and trace output.
8. Run package-audit gates before treating the package set as a stable dialect
   boundary.

This is the intended convergence point: generic AI applications are assembled
from small package boundaries with stable contracts, while examples such as
FinRobot remain consumers of the packages rather than owners of the dialect.
