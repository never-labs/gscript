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

## Budgets, Replay, And Trace

Attach budgets to agent config tables or use lower-level helpers such as
`llm.with_budget`. Provider usage may include cost metadata, but Leia does not
promise money accounting as a stable script-level budget.

Budget dimensions are runtime controls such as turns, tool calls, tokens, and
time. They gate provider and tool work in the AI helper layer; they are not a
general language statement and do not change normal expression evaluation.

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

## Human Review And Resume

The lower-level loop helpers expose pause/resume hooks for human-in-the-loop
workflows. A loop may return a pending result with a token and payload when an
`approve_when` policy asks for review; hosts can persist that snapshot and later
resume it through the matching `loop.resume` helper. Keep this at the helper
layer for now: tagged `agent` and `turn` syntax lowers through the same runtime
so future review policies do not bypass provider, tool, budget, trace, or
replay controls.

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
