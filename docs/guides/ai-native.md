# AI-Native Leia

Leia's AI surface is a standard-library runtime exposed through a small set of
tagged dialect forms and ordinary modules. Scripts use concise `model`, `tool`,
`agent`, and `turn` blocks; the Go host still controls providers, credentials,
capabilities, tracing, recording, and replay.

The design rule is: AI native, but not language intrinsic. The syntax helps you
write agent-shaped programs without making prompts, model calls, or traces
special language semantics. Everything lowers to `llm`, `msg`, `history`, and
host-provider APIs, so the same code can be tested with mocks, replayed from
records, or embedded under host policy.

## Mental Model

Use the simplest layer that fits the workflow:

| Layer | Use |
|---|---|
| `turn { ... }` | One provider request. |
| `agent { ... }` | Reusable model-backed function. |
| `tool { ... }` | Tool descriptor backed by a Leia function. |
| `llm.*`, `msg.*`, `history.*` | Lower-level runtime helpers for custom loops. |

The stable contract is in the [AI-native reference](../reference/ai/index.md).

## Model Defaults

`model { ... }` registers script-visible aliases. Hosts may also inject a
provider directly with `leia.WithLLMProvider`.

```leia
model {
    default: "fast"
}
```

Never put API keys in source. Use environment variables or host-injected
providers.

For portable scripts, keep aliases generic and put provider details behind
environment variables or host configuration:

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

Aliases route requests; they are not permission grants. A host can accept,
rewrite, or reject script-declared provider configs.

## One Turn

`turn { ... }` performs exactly one request. It returns `(result, err)` and does
not dispatch tools by itself.

```leia
result, err := turn {
    model: "fast"
    messages: {
        prompt { role: "system", text: "Be concise." }
        prompt { role: "user", text: "Return exactly: ok" }
    }
    max_tokens: 16
    temperature: 0
}

if err != nil {
    return nil, err
}
print(result.text)
```

Use `turn` when the script owns the message history and wants one provider
round trip.

Prompt blocks with `role` and `text` are message objects. They are interchangeable
with `llm.system`, `llm.user`, and `msg.*` helper output in a `messages` array.
They do not create hidden prompt state; later turns see only the message array
you pass.

## Tools

Tools are ordinary Leia functions wrapped with model-facing metadata.

```leia
lookup_runbook := tool {
    name: "lookup_runbook"
    params: {"service"}
    description: "Look up a local runbook."
    requires: {"docs.read"}
    fn: func(service) {
        return {service: service, steps: {"check metrics", "restart if needed"}}, nil
    }
}
```

Omit `requires` for pure local tools when the host policy allows that.
Capability-aware hosts can inspect tool metadata before exposing it to a
provider.

For reusable tools, include the full contract so tests, hosts, and replay
fixtures can inspect the tool without executing it:

```leia
lookup_runbook := llm.tool("lookup_runbook", func(service) {
    return {service: service, steps: {"check metrics", "restart if needed"}}, nil
}, {
    params: {"service"}
    description: "Look up a local runbook."
    capabilities: {"docs.read", "replay.local"}
    result: {service: "string", steps: {"string"}}
    error: {kind: "validation", message: "string"}
    replay_key: "lookup_runbook:{service}"
})

info := llm.tool_info({lookup_runbook})
ok, err := llm.validate_tools({lookup_runbook})
```

`llm.tool_schema` and `llm.tool_info` are inventory helpers. They do not call
the tool and they do not authorize side effects.

## Agents

`agent { ... }` returns a callable value. The `config` function builds the
request table for each call.

```leia
support := agent {
    name: "support"
    params: {"question"}
    description: "Answer operational questions."
    config: func(question) {
        return {
            model: "fast"
            system: "Use tool evidence when it helps."
            user: question
            tools: {lookup_runbook}
        }, nil
    }
}

result, err := support("How do I restart search?")
```

Without a custom flow function, the runtime converts `system` and `user` into a
message history, runs the built-in turn/tool loop, dispatches requested tools,
and stops when the provider returns a final answer.

## Custom Flow

For explicit multi-turn control, use the lower-level `llm.agent` helper with a
flow function. The flow owns history and dispatch.

```leia
func incident_config(service) {
    return {
        model: "fast"
        messages: {llm.system("Create a short incident update."), llm.user(service)}
        tools: {lookup_runbook}
    }, nil
}

incident := llm.agent("incident", incident_config, func(service) {
    cfg, err := incident_config(service)
    if err != nil {
        return nil, err
    }
    first, err := llm.turn(cfg)
    if err != nil {
        return nil, err
    }
    call := first.calls[1]
    evidence, dispatch_err := llm.dispatch(call, cfg.tools)
    if dispatch_err != nil {
        return nil, dispatch_err
    }
    cfg.messages[#cfg.messages + 1] = msg.assistant_call(call)
    cfg.messages[#cfg.messages + 1] = msg.tool_result(call.id, evidence)
    return llm.turn(cfg)
}, {params: {"service"}})
```

No hidden turn or dispatch happens inside a custom flow; the script calls
`llm.turn` and `llm.dispatch` explicitly.

For agents that only need to bind a model, instructions, tools, and output
shape, use the declarative shorthand. It remains a normal `llm.agent` value,
but the dialect generates the config function for you:

```leia
extract := agent {
    name: "extract"
    params: {"note"}
    model: "fast"
    instructions: prompt { role: "system", text: "Extract project and owner." }
    output: {project: "ORCHID", owner: "ADA"}
}

result, err := extract("Owner Ada is handling Orchid.")
```

The shorthand is best for prompt capsules: fixed instructions, a small set of
request options, optional tools, and an expected output shape. The first call
argument becomes `user` when `messages` is absent, and `instructions` becomes
`system` unless you set `system` yourself. Use explicit `config` or `flow`
functions when argument mapping, branching, tool dispatch, or multi-turn state
needs custom code.

Structured output is validation, not magic parsing. The `output` field tells
the runtime what shape you expect from the provider result; provider-specific
JSON or schema hints can still travel through `response_format`. Built-in agent
execution validates configured shapes. Custom flows should call
`llm.validate_output(value, schema)` if they return provider-derived values that
code will consume.

For reusable shapes, normalize them once with `llm.schema` and use
`llm.output_schema` when the provider supports JSON Schema response-format
hints:

```leia
contact_schema := llm.schema({
    name: {type: "string", description: "Display name"}
    score: "number"
    nickname: "string?"
})

format := llm.output_schema("contact", contact_schema)
result, err := llm.turn({
    model: "fast"
    messages: {llm.user("Extract Ada with score 0.99.")}
    response_format: format
})
ok, message := llm.validate_output(result.text, contact_schema)
```

Use `llm.schema_info(schema).kind` when generic helper code needs to inspect a
shape. These helpers create request and validation metadata; they do not turn
unparsed model text into typed data by themselves.

## Agent As Tool

Agents can be placed in another agent's tool list by using
`llm.agent_as_tool(agent_value)` or by passing agent values through APIs that
document agent-as-tool support.

```leia
extract := agent {
    name: "extract"
    params: {"note"}
    config: func(note) {
        return {
            model: "fast"
            system: "Extract project and owner."
            user: note
            output: {project: "ORCHID", owner: "ADA"}
        }, nil
    }
}

supervisor := agent {
    name: "supervisor"
    params: {"question"}
    config: func(question) {
        return {
            model: "fast"
            user: question
            tools: {llm.agent_as_tool(extract)}
        }, nil
    }
}
```

For supervisor/specialist flows, `llm.handoff(agent, opts)` and
`llm.delegate(agent, opts)` are clearer aliases around the same tool boundary:

```leia
reviewer := llm.agent("reviewer", func(topic) {
    return {
        model: "fast"
        system: "Review delegated work."
        user: topic
        output: {summary: "short finding", confidence: 1}
    }, nil
}, nil, {params: {"topic"}})

delegate_review := llm.delegate(reviewer, {
    name: "delegate_review"
    description: "Delegate review to a specialist agent."
})
```

Delegation is still tool dispatch. The supervisor sees a tool result or a
structured pending/error result; there is no hidden parallel agent runtime.

Delegated results can carry `trace_contract: "agent_tool.v1"` metadata. Use it
to audit composition without reading provider prompts:

```leia
contract_name := delegate_review.trace_contract
```

Trace nodes use ordinary tables with `type`, `name`, `status`, `parent`,
`children`, `error`, `budget`, `cancel`, and `metadata`.

## Retrieval Context

Use `llm.doc`, `llm.collection`, and `llm.retrieve` for small local evidence
sets that should be packaged into a turn or agent request.

```leia
docs := llm.collection({
    llm.doc("Checkout runbook says payment queue owns sev2 incidents.", {
        id: "runbook"
        title: "Checkout runbook"
        source: "local/runbook"
        tags: {"checkout", "payments"}
    })
    llm.document({
        id: "notes"
        title: "Release notes"
        text: "Search indexing work is unrelated to checkout incidents."
        source: "local/notes"
    })
})

ctx := llm.retrieve(docs, "checkout payment sev2", {limit: 1})
result, err := llm.turn({
    model: "fast"
    user: "Who owns the incident?"
    evidence: llm.evidence(ctx.matches, {label: "Runbook evidence"})
})
```

`llm.context` and `llm.evidence` create labeled messages from documents or
matches. They are useful for prompt assembly, not for access control or durable
memory. Put only source text into the collection that the current request is
allowed to send to the provider.

Documents can also hold named sections and provenance fields for report or RAG
pipelines:

```leia
policy_doc := llm.document({
    id: "release_policy"
    title: "Release policy"
    source: "docs/release.md"
    sections: {
        approval: "Production releases require owner approval."
        rollback: "Rollback plans must name the on-call."
    }
    tags: {"release", "operations"}
})
```

The helper only packages local data. Filter secrets, tenant-specific content,
licensed text, and stale sources before building the collection.

## Workflows

Use `llm.workflow` when you want a deterministic sequence of named steps that
can mix agents, direct turns, and normal Leia code. Use `llm.stage` when the
same step also needs workflow-graph metadata such as dependencies, capability
labels, and input/output refs.

```leia
writer := llm.agent("writer", func(topic) {
    return {model: "fast", messages: {llm.user(topic)}}, nil
})

flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return writer(ctx.input)
    })
    llm.step("final", func(ctx) {
        return writer(ctx.input)
    })
})

result, err := flow.run("release notes")
```

Each step receives `ctx.input`, `ctx.initial_input`, `ctx.previous`,
`ctx.results`, and `ctx.context`.
The next step receives the previous step's text or value. The final result
contains ordered `steps` plus named `context`, which makes tests and replay
assertions straightforward.

For offline tests, replace steps with fixtures:

```leia
mocked := flow.mock({draft: {text: "mock draft"}})
result, err := mocked.run("release notes")
```

Workflow helpers sequence work inside one script run. They are not a durable
queue, retry engine, or parallel scheduler.
For offline tests, graph and stage fixtures can use stable `fixture_key` values
such as `"turn:plan"` instead of depending on display names or step positions.

When a package needs an explicit orchestration contract, wrap ordered stages
with `llm.workflow_graph`. The graph helper validates that `depends_on` and
`edges` reference known earlier stages, exposes `graph` metadata, and still
runs through the same sequential workflow engine.

```leia
graph := llm.workflow_graph({
    workflow_id: "research-flow"
    stages: {
        llm.stage("plan", plan_fn, {output_ref: "plan_result"})
        llm.stage("final", final_fn, {
            depends_on: {"plan"}
            input_ref: "plan_result"
        })
    }
    edges: {{from: "plan", to: "final"}}
})
result, err := graph.run("release notes")
```

## Sections

Use `llm.sections` when a report or response has independent parts that should
share the same request context but have separate instructions and output
shapes.

```leia
generated, err := llm.sections({
    model: "fast"
    messages: {
        llm.system("Use the provided evidence and return JSON.")
        llm.user("Project: reusable generation helpers.")
    }
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
risk_owner := generated.values.risk.owner
```

Top-level fields are copied into each section request; section-local fields can
add prompts, evidence, and output shapes. The helper returns ordered
`sections`, raw `results` by name, and parsed `values` by name. It does not
prove that sections agree with each other, so validate cross-section invariants
in ordinary Leia code when that matters.

## Report Artifacts

Use `llm.report_artifact_contract` when an AI workflow produces a report-shaped
artifact that should be checked offline before rendering or publishing.

```leia
contract := llm.report_artifact_contract({
    name: "release_report"
    version: "report.v1"
})

section_schema := contract.schemas.report_section
manifest := contract.manifest_template
manifest.report_id = "release-2026-06"
manifest.report_sections = {"summary", "risk"}
manifest.source_annotations = {}
manifest.ai_disclosure = "AI-assisted draft reviewed by owner."
```

The contract covers section records, chart plans, artifact manifests, source
annotations, freshness warnings, and AI disclosure. It does not render HTML or
PDF and it does not fetch or verify sources. Validate the tables you produce
with `llm.validate_output` or normal assertions before handing them to a
renderer.

## Budgets, Replay, And Trace

Attach budgets to agent config tables or use lower-level helpers such as
`llm.with_budget`. Provider usage may include cost metadata, but Leia does not
promise money accounting as a stable script-level budget.

Budget dimensions are runtime controls such as turns, tool calls, tokens, and
time. They gate provider and tool work in the AI helper layer; they are not a
general language statement and do not change normal expression evaluation.

For traceable budget gates, project errors explicitly:

```leia
outcome := llm.budget_outcome(err, {workflow_run_id: "run-42"})
event := llm.budget_outcome_event(outcome, {trace_id: "budget-review"})
```

`llm.budget_outcome_trace_event` is the explicit trace-named alias. These
helpers only build portable tables; they do not reserve budget, change
accounting, or call providers.

Retrieval and memory context can be traced the same way without storing raw
memory text:

```leia
ctx := llm.retrieve(docs, "checkout payment sev2", {limit: 1})
outcome := llm.memory_outcome(ctx, {workflow_run_id: "run-42"})
event := llm.memory_outcome_event(outcome, {trace_id: "memory-review"})
```

The outcome carries match refs, citations, rank, score, and correlation fields.
It is not a durable memory store, vector index, or permission boundary.

Tool dispatch can use the same trace-safe pattern:

```leia
value, err := llm.dispatch(call, tools)
outcome := llm.tool_outcome(call, value, {workflow_run_id: "run-42"})
event := llm.tool_outcome_event(outcome, {trace_id: "tool-review"})
```

The outcome records tool identity, result/error status, arg names, result refs,
and redaction flags without copying raw args or raw result values.

Longer custom flows can checkpoint resumable state with the same ref-only trace
pattern:

```leia
checkpoint := llm.agent_state_checkpoint({
    agent_run_id: "agent-run-42"
    session_id: "session-7"
    state_version: 3
    input_refs: {llm.doc("input", "memory://input/42", {kind: "ref"})}
    output_refs: {llm.doc("draft", "memory://draft/42", {kind: "ref"})}
}, {
    workflow_run_id: "run-42"
    workflow_step_id: "draft"
})
event := llm.agent_state_checkpoint_event(checkpoint, {
    trace_id: "state-review"
})
```

The checkpoint records identity, refs, deterministic checkpoint/cache keys,
resume token, redaction flags, and correlation fields. It is not a durable
state store and it does not copy raw prompts, raw inputs, raw outputs, or
credentials.

Use policy checks before exposing high-risk tools:

```leia
policy := llm.policy()
ok, err := llm.check_policy({lookup_runbook}, policy)
if err != nil {
    return nil, err
}
outcome := llm.policy_outcome({lookup_runbook}, policy)
event := llm.policy_outcome_event(outcome, {
    trace_id: "policy-review"
    workflow_run_id: "run-42"
})
```

The default policy denies declared capability classes such as network,
credential, publish, generated-code, trading, and portfolio actions unless the
exact capability is allowed. Policy metadata is a runtime gate; the host still
owns real sandboxing, credentials, network access, and approval storage.
Use `llm.policy_outcome` when a workflow needs to log, trace, or branch on a
portable decision table instead of returning a policy error immediately.
Use `llm.policy_outcome_event` or its explicit alias
`llm.policy_outcome_trace_event` when that decision should join the same trace
envelope as model, tool, replay, approval, and workflow events.

Record and replay are host-side:

```go
rec := llm.NewRecorder()
vm := leia.New(leia.WithLLMProvider(provider), leia.WithLLMRecorder(rec.Record))
// run script
_ = llm.SaveRecords("testdata/turns.json", rec.Records())

records, _ := llm.LoadRecords("testdata/turns.json")
vm = leia.New(leia.WithLLMReplay(records))
```

Use `llm.NewTraceRecorder()` or `leia.WithLLMTrace` for metadata events. Trace
events intentionally omit prompt text and tool result values by default.
Replay fixtures match normalized provider requests after dialect lowering, so a
test should keep working if you rewrite a direct `llm.turn` as `turn { ... }`
without changing the resulting request. Trace is for audit and visibility;
replay is for deterministic provider behavior.

For compact script-owned fixtures, use `llm.replay_record` and
`llm.replay_fixture`. A replay record fills stable identity fields and exposes a
replay table. A replay fixture can match a normalized turn request and return
options that drive `llm.turn` without contacting the provider.

```leia
record, err := llm.replay_record({
    replay_key: "turn:1"
    request: {model: "fast", messages: {llm.user("hello")}}
    response: {status: "final_answer", text: "fixture answer"}
})
fixture, err := llm.replay_fixture({record}, {fixture_id: "unit-fixture"})
replay, err := fixture.replay({
    model: "fast"
    messages: {llm.user("hello")}
}, "turn:1")
result, err := llm.turn({
    model: "fast"
    messages: {llm.user("hello")}
    replay: replay
})
```

## Human Review And Resume

The lower-level loop helpers expose pause/resume hooks for human-in-the-loop
workflows. A loop may return a pending result with a token and payload when an
`approve_when` policy asks for review; hosts can persist that snapshot and later
resume it through the matching `loop.resume` helper. Keep this at the helper
layer for now: tagged `agent` and `turn` syntax lowers through the same runtime
so future review policies do not bypass provider, tool, budget, trace, or
replay controls.

When you persist a human decision, record a portable approval trace:

```leia
trace := llm.approval_trace({
    token: token
    pending: pending
    approval: {ok: false, reason: "owner rejected publish"}
    result: denied
    policy: policy
})
event := llm.approval_trace_event(trace, {
    trace_id: "release-review"
    workflow_run_id: "release-42"
})
```

The trace is audit data. Resuming execution still happens through the matching
loop resume helper and host-owned state.

## Evaluation Harness

Use `evaluate` blocks for regression suites that should run through
`leia evaluate`, not during ordinary script execution.

```leia
evaluate "answer corpus" {
    rows := eval.load_jsonl("answer_cases.jsonl")
    for _, row := range rows {
        eval.case(row.id, func() {
            result, err := support(row.question)
            eval.fail_if(err != nil, "agent failed")
            eval.metric("correct", result.text == row.expected)
            eval.metric("chars", #result.text)
            eval.budget({turns: 2, tokens: 2000})
        })
    }
}
```

Useful CLI modes:

```bash
leia evaluate --list examples/evaluate
leia evaluate --format=json --output report.json examples/evaluate
leia evaluate --llm-replay testdata/turns.json examples/evaluate
leia evaluate --baseline baseline.json --gate examples/evaluate
```

JSON reports are the baseline/comparison format. Replay fixtures match
normalized provider requests after dialect lowering, so changing from
`llm.turn` to `turn { ... }` should not require a new fixture unless the actual
request changes.

## Live-provider examples

Live LLM tests are opt-in and must not commit tokens. The generic
Anthropic-compatible path uses:

```bash
LEIA_LLM_INTEGRATION=1
LEIA_ANTHROPIC_COMPAT_BASE_URL=...
LEIA_ANTHROPIC_COMPAT_API_KEY=...
LEIA_ANTHROPIC_COMPAT_MODEL=...
```

The GLM smoke path also accepts `LEIA_GLM_BASE_URL`, `LEIA_GLM_API_KEY`, and
`LEIA_GLM_MODEL`. It also accepts the local convenience aliases
`SENTINEL_GLM_API_KEY`, `GLM_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `GLM_MODEL`, and
`ANTHROPIC_MODEL`.

Offline examples such as `examples/llm/agent.leia` and
`examples/llm/incident_response.leia` are runnable without network access.
Live-provider examples are kept separate; use `examples/llm/glm_smoke.leia`
only when the local GLM-compatible environment variables are configured and
`LEIA_LLM_INTEGRATION=1` is set.

```bash
LEIA_LLM_INTEGRATION=1 \
LEIA_GLM_API_KEY=... \
LEIA_GLM_MODEL=glm-5.1 \
go test ./tests/integration/llm -run 'Test(GLMAnthropicCompatibleLLMIntegration|LLMSyntaxGLMIntegration|LLMSyntaxGLMStreamingIntegration|LLMSyntaxGLMDirectAgentToolsIntegration|GLMExamplesRunWithRealProviderIntegration)$' -count=1 -v
```

## Evidence

The README AI-native promise is tied to deterministic tests and examples:

- `tests/llm/llm_runtime_test.go`, `tests/llm/llm_agent_tools_test.go`,
  `tests/llm/llm_record_replay_test.go`, and
  `tests/llm/llm_ai_dialect_test.go`
- `tests/integration/llm/llm_provider_test.go`,
  `tests/integration/llm/llm_openai_provider_test.go`, and
  `tests/integration/llm/llm_glm_integration_test.go`
- `examples/llm/direct_turn.leia`, `examples/llm/agent_as_tool.leia`,
  `examples/llm/prompt_tagged_messages.leia`, and
  `examples/llm/streaming_turn.leia`
- `examples/ai/coding_agent_replay.leia`,
  `examples/ai/tagged_agent_workflow.leia`, and
  `examples/ai/record_replay_trace_project.leia`
- `examples/evaluate/llm_replay.leia`,
  `examples/evaluate/agent_replay.leia`, and
  `examples/evaluate/multiturn_replay.leia`
