# Leia AI-Native Reference

Leia's AI-native feature is a standard-library layer over host-installed LLM
providers. The language-level surface is intentionally small: tagged `model`,
`tool`, `agent`, and `turn` blocks plus ordinary `llm.*`, `msg.*`, and
`history.*` helpers.

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

Runtime helpers remain available through `llm.tool`, `llm.toolof`,
`llm.agent_as_tool`, `llm.dispatch`, `llm.tool_caps`, and `llm.check_tools`.

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

## Budgets, Replay, And Trace

Budgets can be attached to agent config tables or managed with lower-level
helpers such as `llm.with_budget`. Public dimensions include `turns`, `calls`,
`tokens`, and `time`. Provider usage may include cost metadata, but Leia does
not promise money accounting as a stable script-level budget dimension.

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

## Live Provider Tests

Live LLM tests must be opt-in and must not commit tokens. The repository uses:

```bash
LEIA_LLM_INTEGRATION=1
LEIA_ANTHROPIC_COMPAT_BASE_URL=...
LEIA_ANTHROPIC_COMPAT_API_KEY=...
LEIA_ANTHROPIC_COMPAT_MODEL=...
```

The GLM smoke path also accepts `LEIA_GLM_BASE_URL`, `LEIA_GLM_API_KEY`, and
`LEIA_GLM_MODEL`.
