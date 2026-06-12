# Leia AI-Native Reference

Leia's AI-native feature is a standard-library layer over host-installed LLM
providers. The language-level surface is intentionally small: tagged `model`,
`tool`, `agent`, and `turn` blocks plus ordinary `llm.*`, `msg.*`, and
`history.*` helpers.

AI native does not mean AI intrinsic. The tagged forms are syntax for building
ordinary values and calling ordinary runtime helpers; they do not add hidden
prompt memory, model-specific evaluation rules, or a separate agent engine.
Provider I/O, tool dispatch, budgets, trace, record, and replay all pass through
the same host-visible `llm` runtime paths whether the source uses dialect syntax
or direct helper calls.

## Host Contract

Embedders install providers through Go options:

| Go API | Purpose |
|---|---|
| `leia.WithLLMProvider(provider)` | Provider for ordinary `turn` and agent calls. |
| `leia.WithLLMProviderFactory(factory)` | Builds providers for script-declared `model {}` entries when allowed. |
| `leia.WithLLMTrace(sink)` | Receives metadata-only trace events. |
| `leia.WithLLMRecorder(sink)` | Records provider turns for offline replay. |
| `leia.WithLLMReplay(records)` | Replays recorded turns deterministically. |

Provider implementations use `github.com/never-labs/leia/llm.Provider`.
The same package exposes `ProviderConfig`, `ProviderFactory`, `TurnRequest`,
`TurnResult`, `Tool`, `ToolCall`, `Message`, replay helpers such as
`NewRecorder`, `LoadRecords`, `SaveRecords`, `NewReplayProvider`, trace helpers
such as `NewTraceRecorder`, and provider error classifications such as
`ProviderErrorNetwork`.

## Model Dialect

`model { ... }` registers model aliases for the current script.

```leia
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

Rules:

- `default` may point to another model alias.
- Alias cycles are invalid.
- Secrets should come from environment variables or host-injected providers.
- Host policy decides whether script-declared provider configs are honored.
- Provider config fields are `protocol`, `base_url`, `api_key`,
  `provider_model`, and optional host/provider names. Go hosts receive the
  normalized shape as `llm.ProviderConfig`.
- A model alias is a routing name, not a credential boundary. Scripts should
  keep aliases stable across environments and let the host map them to the
  actual provider/model through `WithLLMProvider`, `WithLLMProviderFactory`, or
  environment-backed config.
- Do not log secrets, copy secret values into traces, or commit replay records
  made with unredacted provider configuration.

## Tool Dialect

`tool { ... }` returns a model-facing tool descriptor backed by a Leia function.

```leia
search_runbook := tool {
    name: "search_runbook"
    params: {"service"}
    description: "Search local runbooks."
    requires: {"docs.read", "metrics.read"}
    fn: func(service) {
        return "runbook:" .. service, nil
    }
}
```

Tool fields:

| Field | Meaning |
|---|---|
| `name` | Provider-visible tool name. |
| `fn` | Leia function called when the tool is dispatched. |
| `params` | Ordered parameter names. |
| `description` | Provider-visible description. |
| `requires` | Capability labels for host policy and audit. |
| `capabilities` | Alias for `requires` in lower-level `llm.tool` option tables. |
| `schema` | Optional provider/tool input schema metadata. |
| `result` / `output` | Script-visible success shape for contract inventory and validation. |
| `error` | Script-visible structured error shape for contract inventory. |
| `replay_key` | Stable key template for deterministic tool fixtures and audit logs. |

Runtime helpers remain available through `llm.tool`, `llm.toolof`,
`llm.agent_as_tool`, `llm.dispatch`, `llm.tool_caps`, `llm.check_tools`,
`llm.tool_schema`, `llm.tool_info`, and `llm.validate_tools`.

Tool contracts are metadata for hosts, tests, replay fixtures, and provider
adapter schemas. They are not authorization by themselves and they do not
coerce tool return values. Use `llm.validate_tools(tools)` to fail fast when a
tool list is missing required contract metadata for a workflow:

```leia
lookup := llm.tool("lookup", func(query) {
    return {answer: "docs:" .. query}, nil
}, {
    params: {"query"}
    description: "Lookup local documents."
    capabilities: {"docs.read", "replay.local"}
    result: {answer: "string"}
    error: {kind: "validation", message: "string"}
    replay_key: "lookup:{query}"
})

info := llm.tool_info({lookup})
ok, err := llm.validate_tools({lookup})
```

`llm.tool_info(tool_or_tools)` returns a contract inventory with normalized
`name`, `kind`, `type`, `description`, `params`, `schema`, `capabilities`,
`requires`, `result`, `error`, and `replay_key` fields. `llm.tool_schema(...)`
returns the provider-facing input schema and script-visible result/error
shapes. Agent-as-tool values report `type: "agent"` and carry
`trace_contract: "agent_tool.v1"`; their provider-facing input schema describes
arguments only, not the delegated agent's output shape.

## Messages And History

`turn` and `llm.turn` receive ordinary message arrays. Use `llm.system`,
`llm.user`, `msg.assistant`, `msg.assistant_call`, `msg.tool_result`, and
`msg.tool_error` to build normalized messages.

```leia
history := {llm.system("You are concise."), llm.user("Summarize this.")}
history[#history + 1] = msg.assistant("draft")
```

Message tables use normalized roles: `system`, `user`, `assistant`, and
`tool`.

A `prompt { role: "...", text: "..." }` block is message-object shorthand. It
produces the same kind of normalized table accepted by `messages` and by the
`msg` helpers:

```leia
messages := {
    prompt { role: "system", text: "Answer from local evidence." }
    prompt { role: "user", text: "Summarize the release." }
}
```

Prompt message objects are data, not compiler directives. They may appear in
message arrays, agent `instructions`, or generated config tables. Trace sinks
may redact prompt text according to host policy.

## Turn Dialect

`turn { ... }` performs exactly one provider request and returns
`(result, err)`. It never dispatches tools by itself.

```leia
result, err := turn {
    model: "fast"
    messages: {llm.user("Reply with ok.")}
    tools: {search_runbook}
    max_tokens: 32
    temperature: 0
}
```

Important fields:

| Field | Meaning |
|---|---|
| `model` | Alias or provider model name. |
| `messages` | Ordered message array. |
| `tools` | Tool descriptors or supported agent-as-tool values. |
| `force_tool` | Force one tool by name when supported by provider. |
| `max_tokens` | Output token limit. |
| `temperature` / `top_p` | Sampling controls. |
| `response_format` | Provider response-format hint. |
| `stream` | Request incremental provider output when supported. |
| `on_stream` / `onStream` | Optional callback for streaming events. |
| `stop` | Stop sequences. |
| `metadata` | String metadata passed to the provider. |

With streaming, callbacks and trace events can receive incremental token data,
but the script still receives one complete final result table after the provider
finishes.

Result fields include `status`, `text`, `calls`, `reason`, and `usage`.
Errors are ordinary result values such as `{kind: "provider", message: "..."}`.

## Agent Dialect

`agent { ... }` returns a callable workflow value. The `config` function builds
the request configuration for each call.

```leia
answer := agent {
    name: "answer"
    params: {"question"}
    description: "Answer with local documentation when useful."
    config: func(question) {
        return {
            model: "fast"
            system: "Use local documentation when useful."
            user: question
            tools: {search_runbook}
        }, nil
    }
}

result, err := answer("What changed?")
```

For common single-input agents, the dialect also accepts a declarative shorthand.
The shorthand synthesizes the same config function and still lowers through
`llm.agent`; it does not introduce a separate execution engine.

```leia
summarize := agent {
    name: "summarize"
    params: {"topic"}
    model: "fast"
    instructions: prompt { role: "system", text: "Use evidence and be concise." }
    tools: {search_runbook}
    output: {summary: "short"}
}

result, err := summarize("release process")
```

When `messages` is omitted, the first call argument becomes `user`. The
`instructions` field is treated as `system` unless `system` is already present.
Prompt field blocks with `role` and `text` are ordinary message tables, so they
can be placed directly in `messages`.

Use the shorthand when the agent is a prompt capsule: fixed model, fixed
instructions, optional tools, sampling controls, metadata, budget, and expected
output shape. Use an explicit `config` function when call arguments need custom
mapping or dynamic request fields. Use a custom `flow` only when the script must
own turn sequencing, message history, or dispatch.

For an agent without a custom flow function, Leia runs the built-in loop:
synthesize messages from `system` and `user`, call one turn, dispatch returned
tool calls, append assistant tool-call and tool-result messages, and repeat
until the provider returns a final answer or the workflow stops.

For explicit custom control, use `llm.agent(name, configFn, flowFn, opts)`.
Custom flow functions call `llm.turn` and `llm.dispatch` directly; no hidden
turn or dispatch occurs.

## Agent As Tool

Use `llm.agent_as_tool(agent_value)` when a supervisor agent should call another
agent as a tool.

```leia
supervisor := agent {
    name: "supervisor"
    params: {"question"}
    config: func(question) {
        return {
            model: "fast"
            user: question
            tools: {llm.agent_as_tool(answer)}
        }, nil
    }
}
```

`llm.handoff(agent, opts)` and `llm.delegate(agent, opts)` are agent-as-tool
aliases for composition-oriented code. They wrap an existing agent as a tool
descriptor, copy supported metadata such as `name`, `description`, `params`,
`requires`, and `output`, and invoke the nested agent through the same dispatch
path as other tools. They do not create a private scheduler or send the nested
agent's output shape as the provider-facing tool input schema. If the delegated
agent pauses for approval, dispatch returns a structured pending error so the
host can persist or resume the outer workflow.

Agent-as-tool dispatch also attaches a generic trace contract. A successful
delegation result may contain `trace` or `tool_trace` with:

| Field | Meaning |
|---|---|
| `type` | Node type such as `agent_tool`, `agent`, `workflow`, or `workflow_step`. |
| `name` | Tool, agent, workflow, or step name. |
| `status` | Runtime status such as `ok`, `done`, `pending`, `stopped`, or `error`. |
| `parent` | Lightweight parent reference when nested. |
| `children` | Ordered nested trace nodes. |
| `error` | Structured error table when the node failed or paused. |
| `budget` / `cancel` | Copied budget or cancellation error details when applicable. |
| `metadata` | Host/script metadata table. |

This trace contract is script-visible result metadata. It complements host
trace sinks, which receive metadata events such as `turn_start`, `turn_end`,
`turn_error`, and streaming events.

## Structured Output

Agents and turns can request structured output with an `output` shape and can
also pass provider-facing hints through `response_format`.

```leia
extract := agent {
    name: "extract"
    params: {"note"}
    model: "fast"
    instructions: prompt { role: "system", text: "Extract project and owner." }
    output: {project: "ORCHID", owner: "ADA"}
}
```

`output` is a validation contract over provider results. It does not make model
text part of Leia syntax or add a new type system rule. Built-in agent
execution validates configured shapes; custom flows should call
`llm.validate_output(value, schema)` when they need the same check. Validation
failures are structured errors, not provider answers.

The lower-level schema helpers are useful when the same shape is shared across
tools, turns, agents, and section generation:

| Helper | Meaning |
|---|---|
| `llm.schema(spec)` | Normalize a lightweight shape or JSON Schema-like table to a JSON Schema table. |
| `llm.schema_info(spec)` / `llm.schemaInfo(spec)` | Return `{schema, json_schema, kind}` for inspection. |
| `llm.output_schema(name, spec, opts)` | Build a provider-facing `response_format` table of type `json_schema`. |

Lightweight object specs treat string values as field types, a trailing `?` as
optional, one-element arrays as array item specs, and descriptor tables as field
schemas with metadata such as `description`.

```leia
contact_schema := llm.schema({
    name: {type: "string", description: "Display name"}
    score: "number"
    nickname: "string?"
    tags: {"string"}
})

format := llm.output_schema("contact", contact_schema)
result, err := llm.turn({
    model: "fast"
    messages: {llm.user("Extract the contact.")}
    response_format: format
})
ok, message := llm.validate_output(result.text, contact_schema)
```

Schemas are request and validation metadata. They do not make model output a
typed Leia value until the script parses or validates it.

## Retrieval Context And Evidence

The memory/RAG helpers build ordinary tables and messages for small local
retrieval workflows:

| Helper | Meaning |
|---|---|
| `llm.doc(value, opts)` / `llm.document(value, opts)` | Build a document table with `text` plus optional `id`, `title`, `source`, `tags`, `sections`, or other metadata. |
| `llm.collection(docs)` / `llm.docs(docs)` | Build a collection table with `docs` and `count`. |
| `llm.retrieve(collection, query, opts)` / `llm.search(...)` | Return ranked local matches; `opts.limit` caps results and `opts.label` labels generated context. |
| `llm.context(matches, opts)` | Build a context message wrapper, default label `Context`. |
| `llm.evidence(matches, opts)` | Build an evidence message wrapper, default label `Evidence`. |

`context` and `evidence` fields on turn, agent, and section request tables are
expanded into user messages before the provider call. These helpers are for
script-local packaging and simple lexical retrieval; they are not a vector
database, persistence layer, citation verifier, or permission boundary.

```leia
docs := llm.collection({
    llm.doc("Checkout runbook says payment queue owns sev2 incidents.", {
        id: "runbook"
        title: "Checkout runbook"
        source: "local/runbook"
        tags: {"checkout", "payments"}
    })
})

ctx := llm.retrieve(docs, "checkout payment sev2", {limit: 1})
result, err := llm.turn({
    model: "fast"
    user: "Who owns the incident?"
    evidence: llm.evidence(ctx.matches, {label: "Runbook evidence"})
})
```

Documents may carry section text and provenance metadata so report or RAG code
can preserve source identity before prompt assembly:

```leia
policy_doc := llm.document({
    id: "policy"
    title: "Release policy"
    source: "docs/release.md"
    sections: {
        approvals: "Production releases require owner approval."
        rollback: "Rollback plans must name the on-call."
    }
})
```

The retrieval helpers rank and package only the collection passed to them. A
host or tool must filter documents before collection construction when access
control, tenant boundaries, licensing, freshness, or secret handling matter.

## Workflows

`llm.workflow(steps)` creates a sequential runner for named AI or non-AI steps.
Each step is built with `llm.step(name, fn, opts)`, with the stage-shaped alias
`llm.stage(name, fn, opts)`, or supplied as a function. A workflow value exposes
`run(input, opts)` and `mock(fixtures)`.

```leia
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return llm.turn({model: "fast", messages: {llm.user(ctx.input)}})
    })
    llm.step("revise", func(ctx) {
        return llm.turn({model: "fast", messages: {llm.user(ctx.input)}})
    })
})

result, err := flow.run("release notes")
```

Step functions receive a context table with `input`, `initial_input`,
`previous`, accumulated `results`, and named `context`. A step may
return a provider/agent result table, a plain value, and optionally an error.
The next step receives the prior step's text or value as its input. The final
workflow result includes `status`, `text`, `value`, ordered `steps`, and named
`context`.

Use workflows for deterministic sequencing, replay, and test fixtures around
agent calls. They are not parallel execution, durable orchestration, retries, or
transaction management. `flow.mock(fixtures)` replaces steps with fixtures and
is intended for tests and offline examples. Fixture lookup prefers a step or
stage `fixture_key`, then the step name, then the 1-based numeric step index.

`llm.workflow_graph(config)` adds a static graph contract around the same
sequential runner. It accepts ordered `stages` and optional `edges`, validates
that dependencies point to earlier stages, exposes `graph` metadata, and then
runs the stages in the supplied order. This is useful when an AI dialect package
needs stable stage ids, capability metadata, input/output refs, and replayable
fixtures without introducing a separate scheduler.

```leia
graph := llm.workflow_graph({
    workflow_id: "research-flow"
    entrypoint: "ai.workflow.orchestrate"
    stages: {
        llm.stage("plan", plan_fn, {output_ref: "plan_result"})
        llm.stage("finalize", final_fn, {
            depends_on: {"plan"}
            input_ref: "plan_result"
            output_ref: "final_result"
        })
    }
    edges: {{from: "plan", to: "finalize"}}
})
result, err := graph.run("topic")
```

The graph helper is intentionally conservative: it validates and records graph
shape, but it does not perform DAG scheduling, parallel execution, retries,
persistence, or hidden provider calls.
Stage metadata fields such as `capability`, `fixture_key`, `input_ref`,
`output_ref`, `input_schema`, `output_schema`, and `depends_on` are also copied
into each executed step's `trace.metadata`, so replay trace events and workflow
trace nodes can be correlated without inspecting raw step inputs or outputs.

## Section Generation

`llm.sections(config)` and `llm.generate_sections(config)` run the same agent
request shape once per requested section and return ordered and name-indexed
results.

```leia
generated, err := llm.sections({
    model: "fast"
    messages: {llm.system("Return JSON."), llm.user("Draft the report.")}
    evidence: "Evidence: launch checklist is complete."
    sections: {
        {
            name: "summary"
            instructions: "Create the summary section."
            output: {headline: "Short headline", confidence: 0.5}
        }
        {
            name: "risk"
            prompt: "Create the risk section."
            output: {risk: "Low", owner: "team"}
        }
    }
})
headline := generated.values.summary.headline
```

Top-level request fields such as `model`, `messages`, `tools`, `context`, and
`evidence` are shared. Each section must have `name` and may provide
`instructions` or `prompt`, `output`, section-local evidence, and other request
fields. Results are returned as `sections` in order, `results` by section name,
and parsed `values` by section name when structured output is configured.

Sections are a convenience for independent report parts. They do not guarantee
cross-section consistency, automatic citations, or shared hidden memory beyond
the request fields supplied to each section.

## Report Artifact Contract

`llm.report_artifact_contract(opts)` returns a generic, offline-verifiable
contract table for report-producing workflows. It is useful when an agent or
workflow produces sections, chart plans, source annotations, and artifact
manifests that another renderer or CI check will validate.

```leia
contract := llm.report_artifact_contract({
    name: "release_report"
    version: "report.v1"
})

manifest := contract.manifest_template
section_schema := contract.schemas.report_section
```

The returned table includes `name`, `version`, `offline_verifiable`,
`renderer_required`, `schemas`, `manifest_template`, and `required_markers`.
The schemas cover:

| Schema | Purpose |
|---|---|
| `report_section` | Ordered section content with chart/source references and AI disclosure flags. |
| `chart_spec` | Chart intent plus source references and planned/rendered artifact metadata. |
| `artifact_manifest` | Report id, generated time, sections, chart specs, artifacts, source annotations, warnings, and disclosure. |
| `source_annotation` | Source identity, locator, freshness fields, optional license, retrieval time, and evidence hash. |

This helper declares an artifact boundary; it does not render HTML/PDF, fetch
data, verify citations, or make product-specific report APIs. Scripts should
validate produced tables with `llm.validate_output` or ordinary assertions and
leave rendering to explicit code or host tooling.

## Budgets, Policy, Approval, Replay, And Trace

Budgets can be attached to agent config tables or managed with lower-level
helpers such as `llm.with_budget`. Public dimensions include `turns`, `calls`,
`tokens`, and `time`. Provider usage may include cost metadata, but Leia does
not promise money accounting as a stable script-level budget dimension.

Budgets gate AI runtime work before or after provider turns and tool dispatch.
They do not change ordinary expression evaluation outside the helper paths.
Declarative agents, explicit agents, and direct turns share the same accounting
when they lower through `llm`.

Policy helpers operate on tool capability labels:

```leia
policy := llm.policy()
ok, err := llm.check_policy({search_runbook}, policy)
```

The default policy is `capability_policy.v1` with `default:
"deny_high_risk"`. It denies capability classes for trading, portfolio,
generated-code, network, credential, and publish actions unless an exact
capability is listed in `allow`. `llm.check_policy(tool_or_tools, policy)`
returns `(true, nil)` or `(nil, {kind: "policy", capability, class, policy,
message})`.

Use `llm.policy_outcome(tool_or_tools, policy, opts)` when scripts need a
portable decision table instead of tuple-style control flow. It returns
`kind: "policy_outcome"`, `status`, `result_status`, `capabilities`,
`allowed`, `denied`, `clean_skip`, `approval_required`, and
`side_effect_allowed`. Passing `{clean_skip: true, reason, dependency}` records
a skipped host/dependency gate without granting the requested capability.

Human approval and resume use the lower-level loop helpers. A pending snapshot
is host-persistable data; `loop.resume(token, approval)` applies an approval
table such as `{ok: true}` or `{ok: false, reason: "..."}`. Use
`llm.approval_trace({token, pending, approval, result, policy})` to record a
portable approval replay trace with `kind: "approval_replay_trace"` and
`version: "approval_replay.v1"`.

Use `llm.approval_trace_event(trace, opts)` when the approval decision should
join a generic trace envelope. It returns a redacted `llm.trace_event`-compatible
event with the default `event_type: "approval_replay_trace"`, correlation IDs,
decision/result status, and policy summary fields. It does not copy approval
tokens, raw tool arguments, or raw result values into the event payload.

Use host-side record/replay for deterministic tests:

```go
rec := llm.NewRecorder()
vm := leia.New(leia.WithLLMProvider(provider), leia.WithLLMRecorder(rec.Record))
// run script
_ = llm.SaveRecords("testdata/turns.json", rec.Records())

records, _ := llm.LoadRecords("testdata/turns.json")
vm = leia.New(leia.WithLLMReplay(records))
replay := llm.NewReplayProvider(records)
_ = replay
```

Use `llm.NewTraceRecorder()` or `leia.WithLLMTrace` for metadata events. Trace
events intentionally omit prompt text and tool result values by default.
Record/replay observes normalized requests after dialect lowering, so replay
fixtures should not depend on whether the source used shorthand syntax or
direct helper calls. Trace is for operational visibility and may redact content;
replay is for deterministic provider behavior and must be strict about request
matching.

Scripts can also build provider-free replay fixtures directly. `llm.replay_record`
normalizes one record, fills `operation`, `capability`, `request_hash`,
`response_hash`, and a `replay` table that can be passed to `llm.turn`.
`llm.replay_fixture(records, opts)` wraps normalized records with the same
strict ordered matcher used by `llm.replay_index`. `fixture.replay(request,
replay_key)` matches a normalized turn request and returns replay options for
`llm.turn`.
`llm.replay_trace_event(match, opts)` projects a `fixture.match(...)` or
`llm.replay_index(...).match(...)` result into a redacted trace event. Matched,
mismatched, and exhausted replay states become `replay_record_matched`,
`replay_record_mismatch`, and `replay_record_exhausted` events with replay
identity hashes and summary counters, not raw prompts or response bodies.
`llm.trace_summary(envelope_or_events)` returns deterministic trace counts,
event types, replay keys, and ordering diagnostics. `llm.trace_assert(input,
opts)` turns those checks into `{ok, status, summary, findings}` without
contacting a provider, so replay and workflow packages can gate trace evidence
with ordinary assertions.

```leia
record, err := llm.replay_record({
    replay_key: "turn:1"
    request: {model: "fast", messages: {llm.user("hello")}}
    response: {status: "final_answer", text: "fixture answer"}
})
fixture, err := llm.replay_fixture({record}, {fixture_id: "unit-fixture"})
request := {model: "fast", messages: {llm.user("hello")}}
replay, err := fixture.replay(request, "turn:1")
result, err := llm.turn({
    model: "fast"
    messages: {llm.user("hello")}
    replay: replay
})
event := llm.trace_event({
    trace_id: "unit-trace"
    event_type: "turn_end"
    replay_key: "turn:1"
    request_hash: replay.request_hash
    response_hash: replay.response_hash
})
trace := llm.trace_envelope({event}, {trace_id: "unit-trace"})
gate := llm.trace_assert(trace, {
    require_provider_free: true
    deny_live_network: true
    required_event_types: {"turn_end"}
})
match := fixture.match({
    operation: "llm.turn"
    capability: "generic.ai.turn"
    replay_key: "turn:1"
    request_hash: replay.request_hash
})
replay_event := llm.replay_trace_event(match, {
    trace_id: "unit-trace"
    replay_session_id: "unit-fixture"
})
```

## Evaluation Harness

`evaluate "name" { ... }` declares a regression case discovered by
`leia evaluate`. The `eval` module is available only while the evaluation
harness runs the block.

| Function | Meaning |
|---|---|
| `eval.case(id, fn)` | Run one named subcase and continue after subcase failures. |
| `eval.metric(name, value)` | Record bool, number, string, nil, or JSON-like metrics. |
| `eval.load_jsonl(path)` | Load a JSON Lines corpus relative to the evaluate source. |
| `eval.skip_if(cond, reason)` | Skip the active subcase when `cond` is truthy. |
| `eval.fail_if(cond, message)` | Fail the active subcase when `cond` is truthy. |
| `eval.usage()` | Return current case LLM usage. |
| `eval.budget(table)` | Fail when current usage exceeds a positive limit. |
| `eval.judge(options)` | Run a bounded judge turn through `llm.turn(options)`. |

The CLI supports `--list`, `--filter`, `--parallel`, `--format json|text|html`,
`--output`/`--report`, `--gate`, `--baseline`, `--compare`,
`--regression-threshold`, `--llm-record`, `--llm-replay`, and
`--update-golden`. JSON reports preserve case metrics, usage, diagnostics, and
optional baseline comparison. Boolean metrics become pass-rate summaries;
numeric metrics become count/mean/min/max summaries; string metrics become
category counts. LLM fixture modes are intended for deterministic provider
behavior and may serialize execution even when `--parallel` is set.

## Live Provider Tests

Live LLM tests must be opt-in and must not commit tokens. The repository uses:

```bash
LEIA_LLM_INTEGRATION=1
LEIA_ANTHROPIC_COMPAT_BASE_URL=...
LEIA_ANTHROPIC_COMPAT_API_KEY=...
LEIA_ANTHROPIC_COMPAT_MODEL=...
```

The GLM smoke path also accepts `LEIA_GLM_BASE_URL`, `LEIA_GLM_API_KEY`, and
`LEIA_GLM_MODEL`. For compatibility with existing local setups, the GLM path
also accepts `SENTINEL_GLM_API_KEY`, `GLM_API_KEY`, `ANTHROPIC_AUTH_TOKEN`,
`GLM_MODEL`, and `ANTHROPIC_MODEL`.

Run the live GLM smoke suite explicitly:

```bash
LEIA_LLM_INTEGRATION=1 \
LEIA_GLM_API_KEY=... \
LEIA_GLM_MODEL=glm-5.1 \
go test ./tests/integration/llm -run 'Test(GLMAnthropicCompatibleLLMIntegration|LLMSyntaxGLMIntegration|LLMSyntaxGLMStreamingIntegration|LLMSyntaxGLMDirectAgentToolsIntegration|GLMExamplesRunWithRealProviderIntegration)$' -count=1 -v
```

`leia examples check` skips live-provider examples by default. Directly running
`examples/llm/glm_smoke.leia` or `examples/llm/glm_direct_agent_tools.leia`
requires both `LEIA_LLM_INTEGRATION=1` and a configured GLM key.

## Evidence

Stable AI-native coverage is tracked in `tests/feature_matrix.json` under
`llm_native_integration`. The main evidence set includes:

| Surface | Evidence |
|---|---|
| Models and provider adapters | `tests/llm/llm_ai_dialect_test.go`, `tests/integration/llm/llm_provider_test.go`, `tests/integration/llm/llm_openai_provider_test.go`, `tests/integration/llm/llm_glm_integration_test.go`, `examples/llm/glm_smoke.leia`, `examples/llm/glm_direct_agent_tools.leia` |
| Tools and agents | `tests/llm/llm_agent_examples_test.go`, `tests/llm/llm_agent_tools_test.go`, `examples/llm/agent.leia`, `examples/llm/agent_as_tool.leia`, `examples/ai/coding_agent_replay.leia` |
| Messages, turns, and streaming | `tests/llm/llm_runtime_test.go`, `tests/llm/llm_loop_test.go`, `examples/llm/direct_turn.leia`, `examples/llm/prompt_tagged_messages.leia`, `examples/llm/streaming_turn.leia` |
| Record, replay, and trace | `tests/llm/llm_record_replay_test.go`, `tests/llm/llm_trace_test.go`, `cmd/leia/main_examples_test.go`, `examples/ai/record_replay_trace_project.leia`, `examples/evaluate/llm_replay.leia`, `examples/evaluate/judge_replay.leia`, `examples/evaluate/multiturn_replay.leia`, `examples/evaluate/project_agent_regression.leia` |
| Tagged AI dialects | `examples/ai/tagged_agent_workflow.leia`, `examples/dialects/ai_prompt_quote.leia` |
