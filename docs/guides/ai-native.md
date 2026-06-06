# AI-Native Leia

Leia's AI surface is a standard-library runtime exposed through a small set of
tagged dialect forms and ordinary modules. Scripts use concise `model`, `tool`,
`agent`, and `turn` blocks; the Go host still controls providers, credentials,
capabilities, tracing, recording, and replay.

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
    messages: {llm.system("Be concise."), llm.user("Return exactly: ok")}
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

Use `requires: {"none"}` or omit capability checks for pure local tools when
the host policy allows that. Capability-aware hosts can inspect tool metadata
before exposing it to a provider.

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
`LEIA_GLM_MODEL`.

Offline examples such as `examples/llm/agent.leia` and
`examples/llm/incident_response.leia` are runnable without network access.
Live-provider examples are kept separate; use `examples/llm/glm_smoke.leia`
only when the local GLM-compatible environment variables are configured.

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
