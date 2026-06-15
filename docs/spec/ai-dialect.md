# AI Dialect Syntax

Leia's AI dialect is an optional standard-library runtime exposed through
tagged dialect forms and ordinary modules. The language implementation must
route these forms through the same `llm`, `msg`, `history`, `dialect`, and
host-provider paths as direct library calls. There is no separate AI execution
engine, and Leia is not an AI-intrinsic language. AI is one DSL implementation on
top of the generic tagged-dialect mechanism.

An AI operation is deterministic until it reaches a provider, host callback, or
tool body with side effects. Conformance tests that need stable behavior should
use host-side replay records or mock providers.

## Design Principle

AI is a dialect layer, not language intrinsic. The grammar may provide small
tagged forms for common AI workflows, yet the language core does not gain
model-specific evaluation rules, hidden prompt state, or a privileged AI
scheduler. Every AI dialect form must lower to ordinary runtime helpers and
ordinary values before provider I/O occurs. This is the same extension boundary
used by other dialects, including `q` for columnar analytics.

This principle has four consequences:

- Dialect syntax is a convenience boundary, not a semantic fork. `agent`,
  `turn`, `tool`, `model`, prompt message objects, output schemas, budgets,
  trace, and replay all use the same `llm`, `msg`, `history`, and host-provider
  paths that direct library calls use.
- Prompts and messages are data. A prompt object is a normalized message table
  when it contains `role` and `text`; it is not a special string literal, hidden
  compiler directive, or lexical scope.
- Structured output is validation over provider results. Request fields may ask
  the provider for a shape and the runtime may validate that shape, but Leia
  does not treat model text as typed language syntax unless a helper explicitly
  parses and validates it.
- Operational controls are host-visible. Budgets, cancellation, approval,
  trace, record, and replay must remain observable through runtime options and
  host hooks instead of being embedded as unreachable language behavior.

## Stable Contract

The stable AI dialect contract is the following lowering boundary:

| Source construct | Stable runtime meaning |
|---|---|
| `model { ... }` | Register model aliases through `llm.register_models`. |
| `tool { ... }` | Build a tool descriptor through `llm.tool`. |
| `turn { ... }` | Perform exactly one provider request through `llm.turn`. |
| `agent { ... }` | Build a callable agent through `llm.agent`. |
| `evaluate "name" { ... }` | Declare a regression case discovered by `leia evaluate`. |
| `llm.*`, `msg.*`, `history.*` | Lower-level helpers for custom flows and message history. |

The dialect forms are not declarations with hidden lexical rules. Each form
evaluates to an ordinary Leia value or result pair according to its runtime
helper. Tools and agents are ordinary values that can be assigned, passed,
returned, inspected, and stored in tables.

The implementation must not fork behavior between dialect syntax and stdlib
calls. For example, a tool created with `tool { ... }` and a tool created with
`llm.tool(...)` must go through the same tool validation, capability metadata,
dispatch, tracing, and replay paths.

Lower-level AI helpers such as `llm.workflow`, `llm.step`, `llm.stage`,
`llm.workflow_graph`, `llm.doc`, `llm.collection`, `llm.retrieve`,
`llm.context`, `llm.evidence`, `llm.schema`, `llm.output_schema`,
`llm.schema_info`, `llm.handoff`, `llm.delegate`, and `llm.sections`,
`llm.tool_info`, `llm.tool_schema`, `llm.validate_tools`, `llm.policy`,
`llm.check_policy`, `llm.approval_trace`, and `llm.report_artifact_contract`
are part of the same runtime layer. They are ordinary functions that return
ordinary tables, callables, or `(value, err)` pairs. They must not introduce
separate prompt memory, provider bypasses, hidden network access, global
document stores, hidden approval stores, report renderers, or a second
orchestration engine.

This stable lowering is observable without a live provider. A tagged tool must
preserve helper-visible capability metadata:

```leia run all
search_runbook := tool {
    name: "search_runbook"
    params: ["service"]
    description: "Search local runbooks."
    requires: ["docs.read", "runbooks.read"]
    fn: func(service) {
        return "runbook:" .. service, nil
    }
}

caps := llm.tool_caps([search_runbook])
assert(#caps == 2)
assert(caps[1] == "docs.read")
assert(caps[2] == "runbooks.read")

ok, err := llm.check_tools([search_runbook], ["docs.read", "runbooks.read"])
assert(ok == true)
assert(err == nil)
```

## Models

`model { ... }` registers script-visible aliases for the current VM.

```text
model {
    default: "fast"
    fast: {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_LLM_BASE_URL")
        api_key: os.getenv("LEIA_LLM_API_KEY")
        provider_model: os.getenv("LEIA_LLM_MODEL")
    }
}
```

A model field value may be a string alias or a provider configuration table.
The alias named `default` is used when a request does not specify a model and
the host has not installed a more specific default. Alias cycles are invalid.

Source code must not embed API keys as string literals. Credentials belong in
environment variables, host-provided configuration, or a host-owned provider
factory. Hosts decide whether script-declared provider configuration is
honored.

The normalized host provider configuration shape is `llm.ProviderConfig` with
`Name`, `Protocol`, `BaseURL`, `APIKey`, `ProviderModel`, and `Provider`
fields. Script model aliases are stable routing names. They must not be treated
as secret scopes, authorization grants, or guarantees that a specific remote
provider will be used. A host may ignore, rewrite, or reject script-declared
provider configuration according to policy.

## Messages And History

Provider requests carry an ordered array of normalized message tables. Leia
does not require a special message block syntax. Scripts may build message
arrays directly or through the `llm` and `msg` helper modules.

```text
history := [
    llm.system("Answer concisely."),
    llm.user("Summarize this incident."),
]

history[#history + 1] = msg.assistant("draft")
history[#history + 1] = msg.user("Now include the owner.")
```

Stable roles are `system`, `user`, `assistant`, and `tool`. Tool-call and
tool-result messages are paired by provider call id. A later turn observes
prior work only through the `messages` value supplied to that turn.

Prompt field blocks are message object shorthand. A table produced by
`prompt { role: "system", text: "..." }` is valid anywhere a normalized message
table is valid:

```text
messages := [
    prompt { role: "system", text: "Answer from the runbook only." }
    prompt { role: "user", text: "How do I restart search?" }
]
```

The shorthand has no separate prompt lifetime. It must preserve the same role,
text, name, metadata, tool-call, and tool-result fields that the `msg` helpers
produce, subject to provider support and normalization rules. Hosts and helper
modules may redact prompt text from trace or replay artifacts according to
their own policies; the source-level object remains ordinary data.

## Tools

`tool { ... }` returns a tool descriptor backed by a Leia function.

```text
search_runbook := tool {
    name: "search_runbook"
    params: ["service"]
    description: "Search local runbooks."
    requires: ["docs.read"]
    fn: func(service) {
        return "runbook:" .. service, nil
    }
}
```

`llm.tool_outcome(call_or_tool_message, result_or_err, opts)` projects tool
dispatch metadata into an ordinary table with `kind: "tool_outcome"`,
`schema_version: 1`, `version: "tool_outcome.v1"`, `tool_call_id`,
`tool_name`, `status`, `result_status`, `result_type`, `result_ref`,
`arg_names`, `error_kind`, redaction metadata, and correlation fields.
`llm.tool_outcome_event(outcome, opts)` projects that table into the generic
trace event shape; `llm.tool_outcome_trace_event` is an equivalent alias. These
helpers must not copy raw tool arguments, raw result values, prompt text,
provider completions, response bodies, stack traces, or credential-bearing
metadata into the outcome payload.

The required fields are:

| Field | Meaning |
|---|---|
| `name` | Provider-visible tool name. |
| `fn` | Function called when the tool is dispatched. |

The optional fields are:

| Field | Meaning |
|---|---|
| `params` | Ordered parameter names. |
| `description` | Provider-visible description. |
| `requires` | Capability labels for host policy and audit. |

The tool function should return `(value, nil)` on success or `(nil, err)` on a
recoverable tool failure. Tool descriptors may be placed in `tools` lists,
passed to `llm.dispatch`, inspected with `llm.tool_caps`, or generated from
ordinary functions with lower-level helpers.

Tool descriptors may also carry contract metadata for validation, inventory,
and replay:

| Field | Meaning |
|---|---|
| `capabilities` | Lower-level option-table alias for `requires`. |
| `schema` | Optional tool input schema metadata. |
| `result` / `output` | Script-visible success shape. |
| `error` | Script-visible structured error shape. |
| `replay_key` | Stable key template for deterministic fixture lookup. |

`llm.tool_schema(tool_or_tools)` and `llm.tool_info(tool_or_tools)` must expose
normalized contract tables without executing tool functions. `llm.validate_tools`
must return `(true, nil)` when every supplied tool has the required contract
metadata for contract-checked workflows and `(nil, err)` when metadata is
missing or invalid. The error must be structured and include at least
`kind: "validation"` plus the failing `tool` and `field` when known.

Tool contracts are advisory metadata. They must not authorize side effects,
coerce return values, or hide runtime tool errors. Host policy, capability
checks, and explicit validation remain separate steps.

`llm.tool_registry(tools, opts)` projects a tool list into a provider-free
registry table with normalized descriptors, capability inventory, effect and
approval counts, redaction metadata, and a compact summary. The registry
helper does not execute tools, register host callbacks, contact providers, or
authorize side effects. `llm.validate_tool_registry(registry, opts)` returns a
validation table with `ok`, `status`, `findings`, and `finding_count`; it must
flag live-network/provider-credential boundaries, duplicate tool names, missing
descriptor fields, provider wire formats other than `none`, secret parameters,
and effectful tools without explicit approval policy. `llm.tool_invocation_trace`
projects a tool call/result boundary into the generic registry trace shape with
ordered `registered`, `schema_validated`, `approval_checked`, `invoked`, and
`result_recorded` events plus metadata-only result and provenance envelopes.

## Turns

`turn { ... }` performs exactly one provider request and returns
`(result, err)`. It does not dispatch tools by itself.

```text
result, err := turn {
    model: "fast"
    messages: [llm.user("Reply exactly: ok")]
    tools: [search_runbook]
    max_tokens: 32
    temperature: 0
}
```

Important request fields are:

| Field | Meaning |
|---|---|
| `model` | Alias or provider model name. |
| `messages` | Ordered message array. |
| `user` | Shorthand for a one-message user request when `messages` is absent. |
| `system` | System prompt used with `user` when `messages` is absent. |
| `tools` | Tool descriptors or supported agent-as-tool values. |
| `force_tool` | Force one tool by name when supported by the provider. |
| `max_tokens` | Output token limit. |
| `temperature`, `top_p` | Sampling controls. |
| `response_format` | Provider response-format hint. |
| `stream` | Request incremental provider output when supported. |
| `on_stream`, `onStream` | Optional callback for streaming events. |
| `stop` | Stop sequences. |
| `metadata` | String metadata passed to the provider. |

With streaming, callbacks and trace events may receive incremental token data.
The script still receives one complete final result table after the provider
finishes. A provider that does not support streaming may ignore `stream` or
return a provider error according to its implementation.

Result fields include `status`, `text`, `calls`, `reason`, and `usage`.
Provider or validation failures are ordinary error values, usually tables with
at least `kind` and `message`.

## Agents

`agent { ... }` returns a callable workflow value. The `config` function builds
the request configuration for each call.

```text
answer := agent {
    name: "answer"
    params: ["question"]
    description: "Answer with local documentation when useful."
    config: func(question) {
        return {
            model: "fast"
            system: "Use tool evidence when it helps."
            user: question
            tools: [search_runbook]
        }, nil
    }
}

result, err := answer("How do I restart search?")
```

As a shorthand for common prompt capsules, `agent { ... }` may omit `config`
and provide request fields directly:

```text
answer := agent {
    name: "answer"
    params: ["question"]
    model: "fast"
    instructions: prompt { role: "system", text: "Use tool evidence." }
    tools: [search_runbook]
    output: {answer: "short"}
}
```

The shorthand lowers to a generated config function passed to `llm.agent`. It
does not create a second agent type. The generated function copies declarative
request fields into a fresh request table for each call and then applies the
same normalization as an explicit `config` function:

- `model`, `tools`, `output`, `response_format`, sampling controls, metadata,
  budgets, and other request fields are copied as ordinary table fields.
- When `messages` is absent, the first call argument is used as `user`.
- `instructions` is copied to `system` unless `system` is already present.
- Prompt field blocks that contain `role` and `text` are valid message tables
  and can be placed directly in `messages`.
- If argument mapping, branching, dynamic tool selection, or persistent
  workflow state is needed, use an explicit `config` or `flow` function.

The required fields are:

| Field | Meaning |
|---|---|
| `name` | Runtime and provider-visible agent name. |
| `config` or `fn` | Optional function that returns an agent request table and optional error. Omit it to use declarative request fields. |

Optional fields include `params`, `description`, `output`, and `flow`. Without
a custom `flow` function, Leia executes the built-in loop:

1. call the config function for the current agent arguments;
2. synthesize `messages` from `system` and `user` when `messages` is absent;
3. perform a provider turn;
4. dispatch requested tools through `llm.dispatch`;
5. append assistant-call and tool-result or tool-error messages;
6. repeat until a final answer, provider stop, budget error, approval pause,
   cancellation, or unrecoverable provider/tool error.

When the `flow` field is present, its function owns the workflow. No hidden
turn or tool dispatch happens. Custom flows should call `llm.turn`,
`llm.dispatch`, `msg.assistant_call`, `msg.tool_result`, and `msg.tool_error`
explicitly.

Custom flows that need auditable tool evidence should also use
`llm.tool_outcome` and `llm.tool_outcome_event` rather than placing raw tool
inputs or outputs in trace payloads.

Custom flows that need resumable or auditable agent state should project state
explicitly through `llm.agent_state_checkpoint(state, opts)`. The checkpoint is
a provider-free, ref-only table with `kind: "agent_state_checkpoint"`,
`schema_version: 1`, `version: "agent_state_checkpoint.v1"`, identifiers such
as `agent_run_id`, `session_id`, `state_version`, and `turn_id`, ref tables such
as `input_refs`, `output_refs`, and `memory_refs`, deterministic
`checkpoint_key` and `cache_key` values, a `resume_token`, redaction metadata,
and trace correlation fields. `llm.agent_state_checkpoint_event(checkpoint,
opts)` projects the checkpoint into the generic trace event shape;
`llm.agent_state_checkpoint_trace_event` is an equivalent explicit trace-named
alias.

Agent state checkpoints are observational metadata. They must not persist state,
perform provider I/O, import live dependencies, or copy raw prompts, raw
provider completions, raw input/output values, credentials, tokens, cookies, or
API keys into the checkpoint payload. Durable storage, locking, and resume
execution remain host or application responsibilities.

For complex flows that need exact control over metadata or call parameters,
scripts may use the lower-level helper directly:

```text
incident := llm.agent("incident", incident_config, incident_flow, {
    params: ["service"]
})
```

## Workflow Helpers

`llm.workflow(steps)` creates a sequential workflow value from `llm.step(name,
fn, opts)`, `llm.stage(name, fn, opts)`, or plain functions. The workflow
exposes `run(input, opts)` and `mock(fixtures)`. A step may call `llm.turn`,
call an agent, dispatch tools, or perform non-AI work.

Workflow execution is deterministic except for the operations performed inside
steps. Each step receives a context table with `input`, `initial_input`,
`previous`, accumulated `results`, and named `context`. The helper records
ordered step results and a name-indexed context table, feeds the prior step's
text or value to the next step, and returns a final workflow result plus an
error value when a step fails. Mock fixtures replace matching steps for tests
and examples. Fixture lookup is deterministic: `fixture_key` when declared,
then step or stage name, then 1-based numeric step index.

`llm.stage` has the same execution semantics as `llm.step`, but marks the
value as a stage and preserves stage-oriented metadata in `opts`, such as
`depends_on`, `input_ref`, `output_ref`, `input_schema`, `output_schema`,
`capability`, and `fixture_key`.

`llm.workflow_graph(config)` wraps the same runner with a static graph
contract. It accepts ordered `stages` and optional `edges`, validates that
dependencies and edges reference known earlier stages, exposes graph metadata,
and then executes sequentially in the supplied order.
Stage metadata that is useful for replay and audit evidence, including
`capability`, `fixture_key`, `input_ref`, `output_ref`, `input_schema`,
`output_schema`, and `depends_on`, is preserved in graph metadata and in each
executed step's `trace.metadata`.

For graph contracts that should not execute anything, `llm.plan_node(id, opts)`,
`llm.planning_graph(nodes, edges, opts)`,
`llm.validate_planning_graph(graph, opts)`, and
`llm.planning_trace(graph, events, opts)` build provider-free planning metadata
only. These helpers normalize plan nodes, dependency edges, retry policy,
branch/merge metadata, trace evidence refs, validation findings, and trace
summaries. They do not schedule work, sleep for retries, run branches in
parallel, call providers, or persist state. `planning_graph` is therefore a
portable contract and replay-evidence shape; `workflow_graph` remains the
sequential execution helper.

These helpers are sequencing helpers, not durable orchestration. They do not
imply parallelism, retry policy, persistence, transactions, approval storage, or
provider calls outside the step functions. Provider replay still observes only
the turns actually made by step bodies.

## Agent As Tool

An agent can be exposed as a tool with `llm.agent_as_tool(agent_value)`.
The wrapper preserves the agent's name and metadata where available.

```text
supervisor := agent {
    name: "supervisor"
    params: ["question"]
    config: func(question) {
        return {
            model: "fast"
            user: question
            tools: [llm.agent_as_tool(answer)]
        }, nil
    }
}
```

`llm.handoff(agent, opts)` and `llm.delegate(agent, opts)` are aliases for the
same agent-as-tool composition boundary. They may copy tool metadata from
`opts` and agent metadata, including `name`, `description`, `params`,
`requires`, and script-visible `output`. The provider-facing tool input schema
must describe tool arguments only; an agent's output shape must not be sent as
the tool input schema. If a nested agent returns a pending approval result, tool
dispatch must surface a structured pending error instead of converting it to
ordinary tool text.

Agent-as-tool wrappers must expose `trace_contract: "agent_tool.v1"` and must
attach generic trace nodes to successful or structured-error results when
available. Trace nodes are ordinary tables with:

| Field | Meaning |
|---|---|
| `type` | Node kind such as `agent_tool`, `agent`, `workflow`, or `workflow_step`. |
| `name` | Node name. |
| `status` | Node status. |
| `parent` | Optional parent reference. |
| `children` | Ordered child trace nodes. |
| `error` | Structured error table, if any. |
| `budget` | Budget error copy when the node failed on budget. |
| `cancel` | Cancellation error copy when the node was cancelled. |
| `metadata` | Metadata table. |

This result-level trace contract is distinct from host trace sinks. It must not
require prompt text or tool result text to be present.

## Output Validation

Agents and turns may request structured output through request fields such as
`output` or provider response-format hints. `output` is a script-visible shape
contract and `response_format` is a provider-facing hint; either may be present
without changing Leia's core type system. Built-in agent execution validates
known output shapes when configured to do so. Custom flows that return
arbitrary values are responsible for calling `llm.validate_output(value,
schema)` if they want the same check.

Validation failures are runtime errors or `(nil, err)` results according to
the helper being used. They must not be silently converted to provider text.
When validation succeeds, the validated value is still an ordinary Leia value.
When parsing or validation fails, callers must handle a structured error rather
than relying on best-effort text coercion.

`llm.schema(spec)` normalizes lightweight schema specs and JSON Schema-like
tables to JSON Schema tables. `llm.schema_info(spec)` and `llm.schemaInfo(spec)`
return inspection metadata including `kind`. `llm.output_schema(name, spec,
opts)` returns a provider-facing `response_format` table using JSON Schema,
with strict mode enabled unless options override it. These helpers define
request and validation metadata only. They do not add a language-level type
checker for model text.

## Retrieval Context And Evidence

`llm.doc`/`llm.document`, `llm.collection`/`llm.docs`, `llm.retrieve`/`llm.search`,
`llm.context`, and `llm.evidence` package local source text for AI requests.
Documents are ordinary tables with `text` and optional metadata. Collections
are ordinary tables with ordered `docs` and `count`. Retrieval returns local
matches according to the runtime helper's ranking rules and options such as
`limit` and `label`.

Document metadata may include `id`, `title`, `source`, `tags`, `sections`, and
additional script fields. Section maps are ordinary source text fields used by
retrieval and report assembly helpers; they do not create a global document
index or hidden memory.

`llm.context` and `llm.evidence` create message wrappers. When a turn, agent, or
section request contains `context` or `evidence`, the runtime appends the
corresponding normalized messages before the provider request. These helpers
do not provide persistence, embeddings, vector search, citation verification,
secret filtering, or authorization. Hosts and tools must still enforce access
policy before source text is added to a request.

`llm.memory_outcome(context_or_matches, opts)` and its alias
`llm.retrieval_outcome(...)` project retrieval/context results into ordinary
tables with `kind: "memory_outcome"`, `version: "memory_outcome.v1"`,
`source_kind`, `status`, `result_status`, `query`, `label`, `match_count`,
`top_id`, `top_score`, `top_match`, and ordered `match_refs`.
`llm.memory_outcome_event(outcome, opts)` projects the outcome into the generic
trace event shape; `llm.memory_outcome_trace_event` and
`llm.retrieval_outcome_event` are equivalent aliases. The projection is
ref-only by default and must not copy raw memory text, snippets, prompt text,
provider completions, or hidden store state into trace payloads.

`llm.evidence_outcome(evidence_or_refs, opts)` and its alias
`llm.citation_outcome(...)` project evidence refs into ordinary tables with
`kind: "evidence_outcome"`, `schema_version: 1`, `version:
"evidence_outcome.v1"`, `source_kind`, `status`, `result_status`,
`evidence_count`, `citation_count`, `source_count`, `artifact_count`, top
evidence identity fields, ordered `evidence_refs`, redaction metadata, and
correlation fields. `llm.evidence_outcome_event(outcome, opts)` projects the
outcome into the generic trace event shape; `llm.evidence_outcome_trace_event`
and `llm.citation_outcome_event` are equivalent aliases. The helper is
observational only: it does not verify citations, persist documents, authorize
source access, call providers, or copy raw evidence text, snippets, prompts,
completions, or credential-bearing metadata into trace payloads.

`llm.model_io_envelope(spec, opts)` projects a model request/response boundary
into an ordinary provider-free metadata table with `kind:
"model_io_envelope"`, `schema_version: 1`, `version:
"model_io_envelope.v1"`, operation/capability fields, provider/model routing
metadata, redacted request headers/auth, message/tool summaries, response
completion summaries, usage totals, schema hints, refs, redaction metadata, and
a compact summary. `llm.model_call_envelope(...)` and `llm.turn_envelope(...)`
are aliases. The helper never calls a provider, executes tools, stores raw
prompts/messages/completions, or requires provider credentials. It is the shared
shape for dialects and packages that need model-call provenance without making
model I/O a built-in language concept.

## Section Generation

`llm.sections(config)` and `llm.generate_sections(config)` run a shared request
configuration once per section descriptor in `config.sections`. Each section
must have a non-empty `name` and may provide `instructions` or `prompt`,
`output`, evidence, messages, or other request fields. Top-level request fields
such as `model`, `messages`, `tools`, `context`, and `evidence` are copied into
each section request unless section-local fields override them according to the
helper's normalization rules.

The result is `(generated, nil)` on success. `generated.sections` preserves
section order, `generated.results` indexes raw agent results by section name,
and `generated.values` indexes parsed or validated values by section name.
Validation and provider failures are returned as ordinary errors. Section
generation is a convenience for independent report parts; it does not guarantee
cross-section consistency, automatic citations, hidden shared memory, or a
document rendering pipeline.

## Report Artifact Contracts

`llm.report_artifact_contract(opts)` returns an ordinary contract table for
report-producing workflows. The helper must accept an optional options table
with `name` and `version` string fields and return a table containing:

| Field | Meaning |
|---|---|
| `name` | Contract name. |
| `version` | Contract version. |
| `offline_verifiable` | Boolean indicating that the contract can be checked without a live provider. |
| `renderer_required` | Boolean indicating whether the contract itself requires rendering. |
| `schemas` | Tables for report sections, chart specs, artifact manifests, and source annotations. |
| `manifest_template` | Starting manifest table with contract/version metadata. |
| `required_markers` | Ordered marker names expected in report outputs or manifests. |

The schemas define generic report data boundaries: ordered sections, chart
intent, artifact metadata, source annotations, freshness warnings, and AI
disclosure. The helper must not render reports, fetch data, verify citations,
or encode a product-specific API. Generated report data remains ordinary Leia
tables and should be validated with `llm.validate_output`, schemas, or ordinary
assertions.

## Budgets, Cancellation, And Approval

Budgets may be attached to agent or turn option tables and may also be enforced
by the host. Stable dimensions include turn count, call count, token count, and
time. Provider usage may include cost metadata, but Leia does not promise money
accounting as a stable script-level budget dimension.

A budget check gates runtime work; it does not change expression evaluation
outside AI helpers. A budget may stop a built-in agent loop before the next
provider request, before the next tool dispatch, or after usage is reported by
a provider. Custom flows that bypass the built-in loop should use lower-level
budget helpers when they need the same accounting.

Hosts may cancel a running AI operation through the VM context or provider
context. Cancellation should stop future provider/tool calls and return a
recoverable error when possible.

Human approval and pause/resume state are runtime features of the `llm` helper
layer. Dialect syntax must preserve those hooks by lowering through the helper
layer instead of bypassing it.

`llm.policy(opts)` returns a capability policy table with `kind:
"capability_policy"`, `version: "capability_policy.v1"`, `default:
"deny_high_risk"`, `deny`, and optional `allow`. The default denied capability
classes are `trading`, `portfolio`, `generated-code`, `network`, `credential`,
and `publish`. `llm.check_policy(tool_or_tools, policy)` must inspect tool
capability labels and return `(true, nil)` or `(nil, err)`. A denied error must
have `kind: "policy"` and include the denied `capability`, denied `class`, and
policy version when known.

`llm.policy_outcome(tool_or_tools, policy, opts)` returns the same policy
decision as a non-throwing table with `kind: "policy_outcome"`,
`version: "policy_outcome.v1"`, `status`, `result_status`, `capabilities`,
policy metadata, and booleans such as `allowed`, `denied`, `clean_skip`,
`approval_required`, and `side_effect_allowed`. `opts.clean_skip` represents
a host or dependency gate that skipped execution without granting the
capability.

`llm.policy_outcome_event(outcome, opts)` projects a policy outcome into the
generic trace event shape. `llm.policy_outcome_trace_event` is an equivalent
alias for callers that prefer explicit trace naming. The default event type is
`policy_outcome`. The projection may include safe policy decision fields,
capabilities, dependency skip metadata, and correlation IDs, but must not clone
executable tool values or raw error tables into the event payload.

`llm.budget_outcome(err, opts)` projects a budget or deadline error table into
a portable, non-throwing outcome with `kind: "budget_outcome"`, `version:
"budget_outcome.v1"`, `source_kind`, `status`, `result_status`, `ok`,
`blocked`, and safe fields such as `dimension`, `limit`, `used`, and
`message`. `llm.budget_outcome_event(outcome, opts)` projects that result into
the generic trace event shape; `llm.budget_outcome_trace_event` is an
equivalent explicit trace-named alias. These helpers are observational only:
they do not reserve budget, mutate accounting, call providers, or promise money
accounting.

Policy checks are runtime gates over declared capabilities. They are not
credential management, sandboxing, network enforcement, or proof that a tool
body is harmless. Hosts must still enforce real resource boundaries.

`llm.approval_trace(table)` returns a portable approval replay table with
`kind: "approval_replay_trace"` and `version: "approval_replay.v1"`. It may
copy `token`, `pending`, `approval`, `result`, and `policy` fields from the
input. If the approval contains `ok: true`, the derived decision status is
`approved`; otherwise it is `denied` and may include `reason`. Approval traces
are audit/replay data. They must not by themselves resume execution or mutate a
host approval store.

`llm.approval_trace_event(trace, opts)` projects an approval replay trace into
the generic trace event shape. The default event type is
`approval_replay_trace`; `opts.event_type` may override it for package-specific
projections. The projection must copy only audit identity and summary fields
such as decision status, result status, tool or operation name, capability,
policy version/default, and correlation IDs. It must not clone raw pending
arguments, approval tokens, or raw result values into the event payload.

## Trace, Record, And Replay

Hosts can install trace sinks, recorders, or replay providers. Trace events are
metadata-oriented and should avoid prompt text and tool result values unless an
explicit host policy permits capture.

Recorders observe normalized provider requests and results after dialect
lowering, so replay does not depend on whether the source used `turn { ... }`,
declarative `agent { ... }`, or direct `llm.turn`. Replay fixtures are strict:
a request mismatch, exhausted fixture, or leftover unconsumed turn is a
failure. Updating a replay fixture is an explicit command or host action, not
an ordinary script side effect.

Trace and replay serve different purposes. Trace is for audit and operational
visibility and may redact content. Replay is for deterministic execution and
must contain enough normalized request/result data to satisfy the replay
provider under the host's chosen recording policy.

`llm.replay_record(spec)` and `llm.replay_fixture(records, opts)` are script
helpers for provider-free fixtures. They construct ordinary tables around the
same identity fields used by the strict ordered matcher: `operation`,
`capability`, `replay_key`, and `request_hash`. They must not create hidden
provider state, bypass host policy, or introduce a second matching algorithm.
The fixture matcher must reuse the same `llm.replay_index` semantics as direct
record/replay tests. `fixture.replay(request, replay_key)` is a convenience
over that matcher: it derives the request hash from the normalized turn request,
consumes according to the fixture policy, and returns replay options for
`llm.turn` or a validation error table.

`llm.fixture_index(spec, opts)` normalizes offline fixture index metadata into
an ordinary provider-free table with `kind: "fixture_index"`, `schema_version:
1`, `version: "fixture_index.v1"`, `fixture_id`, provider/offline flags,
`mode`, `strategy`, normalized `fixtures`, `matching`, deterministic summary
order, and `summary`. Fixture entries may use `fixture_key`, `key`, or `id` for
identity and may carry `path`, `schema`, `schema_path`, `schemas`,
`capability`, and `metadata`. `llm.validate_fixture_index(index, opts)` returns
a gate table with `ok`, `status`, `result_status`, `fixture_count`,
`finding_count`, and `findings`.

Fixture index helpers must remain metadata-only. They may validate table shape,
provider-free/offline flags, replay-ready metadata when requested, and safe
relative reference strings, but they must not open fixture files, resolve paths,
read schemas, scan package directories, require a specific filename, or embed a
project-specific package layout.

`llm.provider_free_package_contract(spec, opts)` normalizes package boundary
metadata into an ordinary table with `kind: "provider_free_package_contract"`,
`schema_version: 1`, `version: "provider_free_package_contract.v1"`,
`package_id`, `package_name`, provider-free/offline flags, `default_policy`,
`credentials`, entrypoint/schema/fixture/capability counts, and `summary`.
`llm.validate_package_contract(contract, opts)` returns a gate table with
`ok`, `status`, `result_status`, `finding_count`, `findings`, and `summary`.

Package contract helpers must remain metadata-only. They may validate offline
defaults, credential-free policy, empty required credential refs, optional
fixture-index references, and safe relative reference strings, but they must
not load package files, resolve directories, read schemas, require a specific
entrypoint name, or embed a project-specific package layout.

`llm.replay_http_record(spec, opts)` normalizes HTTP/API replay metadata into an
ordinary table with `kind: "replay_http_record"`, `schema_version: 1`,
`version: "replay_http_record.v1"`, replay identity, provider-free/offline
flags, redacted `request`, metadata-only `response`, optional `retry`,
`pagination`, `rate_limit`, `error`, `terms`, `redaction`, and `summary`.
`llm.replay_artifact_record(spec, opts)` does the same for downloaded artifact
metadata with `kind: "replay_artifact_record"` and an `artifact` metadata table.

Replay metadata record helpers must not perform HTTP requests, open artifact
files, resolve paths, compute file hashes from disk, or require referenced
resources to exist. They may redact sensitive headers and auth references and
may record caller-provided hashes, byte counts, mock URIs, rate-limit metadata,
pagination metadata, and terms metadata. Raw response bodies and raw artifact
content must not be copied into normalized records by default.

Scripts may validate trace evidence without a host trace sink.
`llm.trace_summary(envelope_or_events)` returns deterministic event counts,
ordered event types, replay keys, status counts, and ordering/correlation
diagnostics. `llm.trace_assert(input, opts)` returns `{ok, status, summary,
findings}` and may require provider-free traces, deny live network/model flags,
require event types, require correlation fields, require payload fields on all
events, or require payload fields for specific event types via
`require_event_payload_fields` or the equivalent
`required_payload_fields_by_event_type`. It may require minimum status counts
with `require_status_counts` and maximum status counts with
`limit_status_counts` or `max_status_counts`. It may also deny truthy redaction
states on each event with `deny_secret_values_present` (alias:
`deny_secret_values`), `deny_raw_prompt_stored`, and
`deny_raw_completion_stored`. These helpers are provider-free; they inspect
ordinary trace tables and must not persist telemetry, redact content, or call
providers.

`llm.replay_trace_event(match, opts)` projects the result of
`llm.replay_index(...).match(...)` or a replay fixture matcher into a standard
trace event. It must preserve the replay matcher's status without re-matching:
`matched`, `mismatch`, and `exhausted` become redacted replay trace events with
safe identity fields such as `replay_key`, `request_hash`, `response_hash`,
`record_id`, summary counters, and finding metadata. It must not clone raw
request messages, raw records, or replay responses into the trace payload, and
must not call providers, persist telemetry, or introduce a second replay
matching algorithm.

## Evaluate Blocks

`evaluate "case name" { ... }` declares a source-level regression case. The
case name is a required string literal. In ordinary script execution an
evaluate block has no runtime effect. The `leia evaluate` command owns
discovery and evaluation semantics.

During evaluation, the harness installs an `eval` module for dataset-style
regression tests:

| Function | Meaning |
|---|---|
| `eval.case(id, fn)` | Run one named subcase. Runtime errors inside `fn` fail that subcase but do not stop later subcases. |
| `eval.metric(name, value)` | Record a bool, number, string, nil, or JSON-like metric. |
| `eval.load_jsonl(path)` | Load a JSON Lines corpus relative to the evaluate source file. |
| `eval.skip_if(cond, reason)` | Mark the active subcase skipped when `cond` is truthy. |
| `eval.fail_if(cond, message)` | Raise a runtime error when `cond` is truthy. |
| `eval.usage()` | Return current case LLM usage. |
| `eval.budget(table)` | Raise when current usage exceeds a positive limit. |
| `eval.judge(options)` | Run a bounded judge turn by calling `llm.turn(options)`. |

```text
evaluate "answer corpus" {
    rows := eval.load_jsonl("answer_cases.jsonl")
    for _, row := range rows {
        ok, err := eval.case(row.id, func() {
            result, err := answer(row.question)
            assert(err == nil)
            eval.metric("correct", result.text == row.expected)
        })
        assert(ok || err != nil)
    }
}
```

Evaluation reports preserve raw metrics on each case and include top-level
metric summaries. Boolean metrics are summarized as pass rates. Numeric metrics
are summarized as count, mean, min, and max. String metrics are summarized as
category counts. Skipped subcases do not contribute to metric summaries.

The `leia evaluate` CLI may render reports as JSON, text, or HTML; list cases
without executing them; filter selected cases; record, replay, or update LLM
replay fixtures; compare against a baseline; and run in gate mode. JSON is the
stable exchange format for baselines and comparisons. Fixture replay must use
normalized provider requests after dialect lowering and must fail on request
mismatch, exhausted fixtures, or leftover unconsumed turns. Evaluation harness
functions are available only while evaluate blocks run under the harness and
must not become ordinary runtime globals.

## Capability Checks

AI dialects declare `llm.turn` capability usage. Tool descriptors may declare
additional `requires` labels. A host may reject scripts or tool exposure when
the requested capabilities exceed policy.

Capability failure is a host/runtime error. It must not be hidden as a provider
answer.

## Live Provider Tests

Live LLM tests must be opt-in and must not commit tokens. Integration tests
should read credentials from environment variables and skip when credentials
are absent.
