# AI Dialect Architecture Contract

## Goal

Leia's AI surface should be a small set of general dialects that make models,
turns, tools, agents, evaluation, memory, trace, and replay inspectable in Leia
source. Domain systems such as FinRobot should be built from these dialects,
ordinary Leia packages, and Leia-native data/workflow/document capabilities.

This contract narrows the boundary of the general AI dialect layer. It is not a
runtime implementation plan.

## Layer Boundary

The general AI layer owns provider-neutral orchestration:

- `model`: named model/provider aliases, protocol selection, endpoint
  configuration, credentials by environment or host injection, and routing
  metadata;
- `turn`: exactly one model request with explicit inputs, options, tool
  schemas, streaming mode, and structured response data;
- `tool`: callable Leia values with names, descriptions, parameter schemas,
  capabilities, structured errors, approval requirements, and trace metadata;
- `agent`: callable orchestration values over turns, tools, history, memory,
  output validation, custom flows, cancellation, and error policy;
- `evaluate`: named case runners, metrics, fixtures, provider-free replay, and
  CI-friendly reports;
- `memory`: explicit retrieval and persistence contracts when an agent keeps or
  recalls state;
- `record` / `replay`: deterministic capture of nondeterministic AI, tool, web,
  process, file, and clock effects when tests require reproducibility.

The layer does not own domain knowledge:

- no finance, legal, medical, support, coding, or research-specific syntax;
- no built-in FinRobot roles, report sections, valuation formulas, data vendors,
  SEC heuristics, or prompt templates;
- no provider-specific response objects as the public contract;
- no automatic hidden tool dispatch in `turn`;
- no hidden network, filesystem, process, or credential access outside declared
  capabilities;
- no special parser keywords for each AI concept beyond the normal dialect tag
  mechanism.

AI dialect outputs are ordinary Leia values. They must compose with assignment,
functions, modules, pipelines, `try`, destructuring, `test`, and `workflow`
without requiring a separate agent runtime language.

## Core Contracts

`turn` is the atomic model request. It may include provider options and tool
schemas, but it returns tool-call proposals as data. It must not invoke tools,
mutate memory, append hidden history, or retry invisibly unless the caller asks
for a retry policy that is visible in trace output.

`agent` is the orchestration boundary. The default agent loop may execute
multiple turns and tools, but the loop must be observable as a sequence of
events. A custom `flow` owns history, tool dispatch, approval gates, memory
reads/writes, retries, and termination. Agent defaults are explicit values that
can be inspected, overridden, and passed to other agents.

`tool` is the side-effect boundary. A tool declaration must expose a stable
schema, required capabilities, approval policy, timeout/cancellation behavior,
and structured success/error shape. Tools can wrap Leia functions, host
functions, API calls, shell commands, or other agents, but callers see the same
contract.

`model` is a provider alias, not a global mutable singleton. Source code may
refer to aliases such as `"research"` or `"cheap-fast"`, while deployment
configuration supplies endpoints and credentials. Routing policy may choose
between aliases, but the selected provider and model must be present in traces.

`evaluate` is an executable measurement contract, not just a prompt convention.
It must support static fixtures, recorded traces, real-provider runs when
allowed, metric emission, and regression comparison.

## Composition With Leia Native Features

AI dialects should reuse Leia-native features instead of cloning them:

- `api`, `web`, `db`, `mail`, `sh`, and file/path dialects remain the way tools
  reach external systems;
- `data`, `json`, `jsonl`, `csv`, tables, vectors, and matrices remain the way
  corpora, metrics, and analytic outputs are represented;
- document, chart, HTML, and PDF packages produce user-facing reports from
  ordinary data and metadata;
- `workflow` sequences agent calls, data fetches, report generation, approvals,
  artifacts, and CI gates;
- `test` and `evaluate` share fixtures, golden outputs, trace replay, and
  assertion/reporting infrastructure;
- capabilities and approval gates apply uniformly to AI and non-AI side
  effects.

This means a finance research flow should look like a Leia workflow that calls
finance packages, data transforms, document parsers, chart/report packages, and
AI agents. It should not require a new `finance_agent` syntax tier.

## Composition With Other Dialects

AI values may be embedded in higher-level dialects, and higher-level dialects
may produce inputs for AI:

- a `workflow` step can call an agent and capture its artifacts;
- an `api` client can be wrapped as a `tool` with a JSON schema and rate-limit
  metadata;
- a `db` query can feed a retrieval tool or evaluation corpus;
- a `document` parser can produce chunks for RAG memory;
- a `chart` or report section can include AI-generated analysis with provenance
  and disclosure metadata;
- a `test` can replay a recorded agent trace without network or provider calls.

Composition must remain explicit. If a dialect invokes another side-effecting
dialect, the composed capabilities, approvals, timeouts, retries, trace events,
and artifacts must be visible to host policy and review tooling.

## Agent-As-Tool Contract

Agents can be used as tools for multi-agent systems, but this is ordinary value
composition, not a separate group-chat primitive.

Required behavior:

- agent tools expose a name, description, input schema, output schema if known,
  required capabilities, and max-turn or budget limits;
- cancellation propagates into nested agent turns and tool calls;
- nested traces preserve parent/child span relationships;
- errors preserve whether they came from model failure, validation failure,
  tool failure, timeout, approval denial, or budget exhaustion;
- replay can replace the nested agent call with a recorded result or replay its
  full internal trace, depending on test mode.

Group chat, leader-directed teams, reflection loops, shadow assistants, and
planner/executor patterns should be expressed as libraries over this contract.

## FinRobot Migration Route

FinRobot-like workloads should migrate in stages:

1. Model and credential mapping: declare provider aliases with `model`, move API
   keys to env/host secrets, and record selected provider/model per turn.
2. Tool inventory: translate each data source, SEC helper, document parser,
   chart generator, code executor, and report writer into a `tool` or ordinary
   package function with schemas, capabilities, rate limits, and structured
   errors.
3. Single-agent flows: translate single assistant and RAG assistant workflows to
   callable `agent` values with explicit tools, memory, output schemas, and
   termination rules.
4. Multi-agent flows: express group chat, leader-directed workflows, shadow
   assistants, and role libraries as agent libraries using agent-as-tool
   composition or explicit custom `flow` functions.
5. Data pipeline extraction: move Yahoo/Finnhub/FMP/SEC fetches, statement
   normalization, metrics, peer tables, forecasts, backtests, and provenance
   into finance/data packages, not AI dialect syntax.
6. Report pipeline extraction: generate HTML/PDF/charts from data and section
   metadata through document/report packages. AI-written sections must carry
   source provenance, generation trace IDs, timestamps, and disclosure markers.
7. Evaluation and replay: build fixtures from recorded FinRobot runs, add
   provider-free replay for CI, and keep real-provider evaluations optional and
   capability-gated.
8. Production hardening: add workflow artifacts, approval gates for trading,
   generated code, or high-risk actions, package-level service declarations,
   and deployment-specific routing policies.

This route keeps FinRobot as a package family and workload benchmark. It should
not introduce `finrobot`, `finance`, `equity`, `sec`, or `trading` dialects in
the core language.

## Performance Requirements

The first contract can be interpreted, but it must not block future low-overhead
execution. Public APIs should leave room for:

- stable normalized request/response envelopes that avoid provider-specific
  shape checks in hot paths;
- precompiled tool schemas and output validators;
- reusable provider clients and connection pools owned by host configuration;
- streaming events delivered incrementally without building full transcripts
  first;
- bounded history and memory materialization so agents can control token and
  storage growth;
- batch evaluation runs with shared fixtures, caches, and rate-limit-aware
  scheduling;
- trace sampling or redaction that does not change program behavior;
- deterministic replay lookup by stable trace keys rather than fuzzy prompt
  matching.

Performance metadata should be observable as data: token counts, provider
latency, tool latency, retry count, cache hits, replay hits, validation time,
and artifact sizes.

## Trace Contract

Every AI run should be traceable as a tree of typed events:

- run/session start and end;
- model selection and routing decision;
- turn request and response metadata;
- streamed response chunks when streaming is enabled;
- tool proposal, approval, invocation, result, and error;
- memory read/write/retrieval events;
- validation success/failure;
- retry, timeout, cancellation, and budget events;
- produced artifacts and provenance links;
- redaction markers for secrets and sensitive payload fields.

Trace records must support human review and machine replay. They should use
stable IDs, parent IDs, timestamps, capabilities, model aliases, provider/model
names, token/latency counters, and redaction metadata. Secrets and raw sensitive
payloads must not be required for useful traces.

## Replay Contract

Replay is required for deterministic regression tests and migration safety.

Minimum requirements:

- replay can satisfy `turn`, `tool`, `api`/`web`, process, file, clock, and
  nested agent effects from a recorded trace when configured;
- replay mode must fail loudly on missing, ambiguous, or incompatible events;
- mismatch diagnostics should show the normalized request fields that changed;
- streaming replays preserve chunk order and final aggregate response;
- approval outcomes can be recorded and replayed, but real approval mode remains
  available for interactive runs;
- replayed tool failures preserve original structured error categories;
- tests can choose strict full-trace replay or boundary replay that replaces an
  agent/tool subtree with its recorded result;
- replay artifacts are versioned so schema changes produce actionable migration
  diagnostics.

Replay must never silently fall back to live provider/network/process calls in a
test mode that claims to be deterministic.

## Open Questions

- Whether `memory` is a first public dialect or a library contract used by
  `agent` and RAG packages.
- How much model-routing policy belongs in `model` versus ordinary workflow or
  package code.
- Which trace envelope fields are stable in the first reviewed contract.
- How redaction policies are declared for provider payloads, tool arguments,
  retrieved documents, and report artifacts.

## Non-Goals

- Do not design the parser, VM, JIT, provider adapters, or storage format here.
- Do not make AI dialects a privileged runtime that bypasses ordinary Leia
  values, modules, capabilities, or tests.
- Do not encode FinRobot or any other domain application as core syntax.
- Do not require real model calls for CI tests.
- Do not make traces require secret-bearing raw payloads.
