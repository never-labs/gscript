# Generic AI Dialect Package Boundaries

This document records the package architecture for building complex AI projects
with a small amount of Leia code. The FinRobot translation examples are the
audit source that exposed the missing boundaries, but the boundaries below are
generic AI application packages: they do not define FinRobot syntax, import
FinRobot dependencies, require live model providers, or depend on built-in
language features.

The dialect is package-composed. A Leia program describes the project shape:
model aliases, turns, tools, agents, workflows, approval rules, replay fixtures,
trace sinks, and evaluation gates. Ordinary packages own the contracts behind
those declarations. The host language loads package manifests, schemas,
fixtures, examples, and tests; it does not need a special AI keyword, a
privileged runtime hook, or a FinRobot-specific command surface.

The result is a thin application layer over explicit package boundaries. A
project can stay small because the repeated machinery is delegated to packages:
deterministic model replay, tool envelopes, human approval state, trace event
normalization, evaluation reports, and package-boundary checks.

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
- model-io: `examples/ai/finrobot_translation/live_packages/generic_model_io_envelope`
- document-rag: `examples/ai/finrobot_translation/live_packages/generic_document_rag_pipeline`
- prompt-role: `examples/ai/finrobot_translation/live_packages/generic_prompt_role_catalog`
- evidence-report-artifacts: `examples/ai/finrobot_translation/live_packages/generic_evidence_report_artifacts`
- ui-snapshot: `examples/ai/finrobot_translation/live_packages/generic_ui_snapshot_evaluator`
- chart-render: `examples/ai/finrobot_translation/live_packages/generic_chart_render_contracts`
- memory: `examples/ai/finrobot_translation/live_packages/generic_memory_store`
- turn: `examples/ai/finrobot_translation/live_packages/generic_turn_runner`
- tool: `examples/ai/finrobot_translation/live_packages/generic_tool_contracts`
- tool-registry: `examples/ai/finrobot_translation/live_packages/generic_tool_registry`
- coding-workspace: `examples/ai/finrobot_translation/live_packages/generic_coding_workspace`
- agent: `examples/ai/finrobot_translation/live_packages/generic_agent_runner`
- planning: `examples/ai/finrobot_translation/live_packages/generic_planning_graph`
- workflow: `examples/ai/finrobot_translation/live_packages/generic_workflow_orchestrator`
- eval: `examples/ai/finrobot_translation/live_packages/generic_evaluation_harness`
- replay: `examples/ai/finrobot_translation/live_packages/generic_record_replay`
- trace: `examples/ai/finrobot_translation/live_packages/generic_trace_events`
- approval: `examples/ai/finrobot_translation/live_packages/generic_approval_policy`
- package-audit: `examples/ai/finrobot_translation/live_packages/generic_package_boundary_auditor`
<!-- ai-dialect-boundary-package-list:end -->

## Package Composition

Complex applications are built by combining these packages along explicit data
contracts instead of embedding orchestration in user scripts. The packages are
orthogonal and layered:

- model resolves a provider-neutral alias into replay-safe execution
  descriptors, policy flags, and redaction rules.
- model-io owns request, stream chunk, response, usage, replay correlation, and
  redaction envelopes between model resolution and turn execution.
- document-rag owns document conversion, section extraction, chunk provenance,
  corpus manifests, retrieval results, citation consistency, and provider-free
  adapter boundaries.
- prompt-role owns prompt catalogs, role profile versions, prompt template
  snapshots, delegation triggers, output schemas, evidence validation, and
  termination conventions.
- evidence-report-artifacts owns source annotations, citation envelopes,
  report outlines, section dependency DAGs, artifact manifests, render manifests,
  snapshot metadata, stale-data warnings, accessibility checks, and renderer
  clean-skip envelopes.
- ui-snapshot owns route DOM schemas, viewport matrices, visual diff budgets,
  accessibility summaries, artifact URI manifests, redaction policy, static
  asset policy, and browser clean-skip envelopes.
- chart-render owns chart specs, recipe semantic matrices, render requests and
  results, source metadata, deterministic snapshot hashes, and unsupported
  renderer clean-skip envelopes.
- memory owns namespace policy, memory items, deterministic retrieval ranking,
  context windows, provenance, and provider-free adapter boundaries.
- turn consumes model descriptors, structured messages, tool-call envelopes,
  replay fixtures, and output schemas to produce one deterministic model-turn
  result.
- tool validates a tool request, checks capability metadata, runs or replays the
  implementation, and returns a normalized result envelope.
- tool-registry declares reusable tool descriptors, validates schemas, records
  invocation traces, and keeps effectful approval edges provider-free by default.
- coding-workspace owns sandbox command envelopes, approval gates, stdout/stderr
  captures, file and image artifact manifests, notebook display metadata,
  cleanup policy, deterministic replay, and execution clean skips.
- agent joins turns, tools, memory snapshots, structured output validation, and
  loop budgets into a declarative agent run.
- planning turns goals into explicit nodes, dependencies, retry rules,
  branch/merge joins, and trace evidence before workflow execution.
- workflow coordinates multiple agents or stages, handoffs, retries, cache
  policy, artifact plans, and trace hooks.
- approval applies policy decisions before tool calls, workflow actions, and
  capability-gated operations.
- trace emits normalized metadata events for turns, streams, tools, approvals,
  replays, artifacts, and workflow stages.
- replay records and matches deterministic model, tool, approval, artifact, and
  trace fixtures so tests stay provider-free.
- eval runs replayed cases over agents or workflows and produces metrics,
  findings, and golden-gate decisions without live judges.
- package-audit checks package manifests, fixture indexes, examples, provider
  gates, and missing-boundary records so the dialect remains package-owned.

A single-turn assistant can use only model, turn, provider-free replay, and
trace. A tool-using analyst adds tool and approval. A multi-step product
workflow adds agent and workflow. Regression gates add evaluation. Release
checks add package-audit. None of these combinations requires changing the host
language or specializing the dialect for one application domain.

## Responsibility Boundaries

| Responsibility | Owning package | Boundary |
| --- | --- | --- |
| Package composition | `generic_model_registry`, `generic_model_io_envelope`, `generic_memory_store`, `generic_turn_runner`, `generic_tool_contracts`, `generic_agent_runner`, `generic_workflow_orchestrator` | Leia scripts declare aliases, messages, tools, agents, memory, and workflow graphs; packages own validation, envelopes, context windows, loop semantics, and stage I/O. |
| Document RAG | `generic_document_rag_pipeline`, `generic_memory_store` | Document conversion, sections, chunks, citations, retrieval results, and adapter clean-skip boundaries are generic package data; domain adapters provide source documents outside the core language. |
| Prompt and role catalogs | `generic_prompt_role_catalog`, `generic_agent_runner`, `generic_workflow_orchestrator` | Prompt templates, role profiles, output schemas, delegation triggers, evidence validation, and termination conventions are package-owned fixtures and contracts, not built-in language syntax. |
| Evidence/report artifacts | `generic_evidence_report_artifacts`, `generic_document_rag_pipeline`, `generic_workflow_orchestrator`, `generic_trace_events` | Source annotations, citation envelopes, section DAGs, render manifests, snapshots, stale warnings, accessibility checks, and clean-skip renderer gates are package data shared by workflows, traces, replay, and reporting packages. |
| Chart rendering | `generic_chart_render_contracts`, `generic_evidence_report_artifacts`, `generic_ui_snapshot_evaluator` | Chart specs, recipe matrices, render envelopes, source metadata, and deterministic snapshot hashes are produced by chart-render; evidence-report consumes chart artifacts; UI snapshot evaluates rendered routes and accessibility without owning chart semantics. |
| UI snapshot evaluation | `generic_ui_snapshot_evaluator`, `generic_evidence_report_artifacts`, `generic_trace_events` | Route DOM schemas, viewport matrices, visual diff budgets, accessibility summaries, artifact URI manifests, redaction policy, static asset policy, and browser clean-skip metadata stay generic UI package data rather than product-specific web code. |
| Provider-free replay | `generic_turn_runner`, `generic_record_replay` | Default examples and tests read checked-in records and fixtures; live provider credentials, network calls, and provider SDK imports remain outside the boundary. |
| Approval | `generic_approval_policy`, `generic_tool_contracts` | Capability checks produce pending, approved, denied, or clean-skip outcomes before side-effecting work; approval replay traces make the decision deterministic. |
| Coding workspace | `generic_coding_workspace`, `generic_approval_policy`, `generic_tool_contracts`, `generic_record_replay` | Command envelopes, stdout/stderr captures, generated artifacts, notebook display metadata, cleanup intent, and sandbox clean skips are package-owned fixtures behind explicit approval gates. |
| Trace | `generic_trace_events`, `generic_workflow_orchestrator`, `generic_agent_runner` | Runtime actions emit metadata-only event envelopes with correlation IDs, redaction policy, replay markers, and artifact references. |
| Evaluation | `generic_evaluation_harness` | Evaluation cases consume replay records, metric specs, judge fixtures, and golden gates; they do not call live judges by default. |
| Record/replay | `generic_record_replay` | Record files define ordered matching, mismatch findings, unconsumed-record checks, usage snapshots, and portable fixtures shared by turns, tools, approvals, trace, and evaluation. |
| Package audit | `generic_package_boundary_auditor` | Audits prove manifests, fixtures, examples, provider-free flags, and documented package directories are present before the dialect is treated as stable. |

## Small Leia Assembly

The intended application code is a thin assembly layer. It names package-owned
contracts and wires their results:

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
7. Run evaluation packages against the same replay fixtures and trace output.
8. Run package-audit gates before treating the package set as a stable dialect
   boundary.

This is the convergence point for the AI dialect: complex AI applications are
assembled from small Leia declarations plus stable package contracts, while
examples such as FinRobot remain consumers of the packages rather than owners
of the dialect.
