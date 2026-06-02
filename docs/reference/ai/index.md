# Leia AI-Native Reference

Leia's AI-native feature is a language syntax layer over standard-library
modules. The syntax is concise for scripts; the host remains in control of
providers, credentials, replay fixtures, tracing, and capabilities.

## Host Contract

Embedders install providers through Go options:

| Go API | Purpose |
|---|---|
| `leia.WithLLMProvider(provider)` | Provider for ordinary `turn` and agent calls. |
| `leia.WithLLMProviderFactory(factory)` | Builds providers for script-declared `models {}` entries. |
| `leia.WithLLMTrace(sink)` | Receives metadata-only trace events. |
| `leia.WithLLMRecorder(sink)` | Records provider turns for offline replay. |
| `leia.WithLLMReplay(records)` | Replays recorded turns deterministically. |

Provider implementations use `github.com/never-labs/leia/llm.Provider`.
The same package exposes the host/provider contract types used by embedders:
`ProviderConfig`, `ProviderFactory`, `TurnRequest`, `TurnResult`, `Tool`,
`ToolCall`, `Message`, and provider error classifications such as
`ProviderErrorNetwork`.

Deterministic tests can use `llm.NewRecorder`, `llm.SaveRecords`,
`llm.LoadRecords`, `llm.NewReplayProvider`, and `llm.NewTraceRecorder` without
calling a live model.

## Model Declarations

`models {}` declares model aliases and provider configs. It is module-scoped.

```leia
models {
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
- `api_key` must not be a string literal; read secrets from environment or
  host-provided values.
- Supported protocol literals are provider-specific but currently include
  Anthropic-compatible and OpenAI-compatible shapes.
- Host-injected providers may ignore script provider configs.

## Tools

Declare tools with `tool name(params) { ... }`. Tool metadata is read from
leading `//leia:` comments immediately above the tool.

```leia
// Searches local runbooks.
//leia:requires docs.read,metrics.read
//leia:param service production service name
tool search_runbook(service) {
    return "runbook:" .. service, nil
}
```

Tool contracts:

- Every `tool` declaration must have `//leia:requires ...`.
- Use `//leia:requires none` for local tools with no host capability.
- `//leia:param NAME TEXT` documents declared parameters.
- Duplicate tool names, duplicate params, and malformed capabilities are
  validation errors.

Runtime tool helpers remain available through `llm.tool`, `llm.toolof`,
`llm.agent_as_tool`, `llm.dispatch`, `llm.tool_caps`, and `llm.check_tools`.

## Messages

For simple histories, use the syntax block:

```leia
history := messages {
    system: "You are concise."
    user: "Summarize this."
}
```

For incremental histories, use `msg` helpers:

```leia
history[#history + 1] = msg.assistant(result.text)
history[#history + 1] = msg.assistant_call(call)
history[#history + 1] = msg.tool_result(call.id, evidence)
history[#history + 1] = msg.tool_error(call.id, "not found")
```

Message tables use normalized roles: `system`, `user`, `assistant`, and
`tool`.

## Agents, Turns, Histories, And Tools

The AI surface has four distinct responsibilities:

- `agent` defines a callable workflow frame: defaults, tools, budgets, output
  validation, tracing, replay, and optional `flow` code.
- `turn` performs exactly one provider request. It returns provider output and
  never dispatches tools by itself.
- `messages`/`history` hold the ordered context passed from one turn to the
  next.
- `tools` describe callable capabilities available to a turn or agent; actual
  tool execution happens in the built-in agent loop or in explicit flow code
  through `llm.dispatch`.

Composition rules:

- `model`, `tools`, budget, output, `response_format`, and metadata fields are
  agent configuration. `agent defaults` supplies module-level values;
  agent-local fields override them.
- `system` and `user` are prompts used to create the first message history for
  the built-in agent loop. A manual `flow` may use `system`, but it should build
  user messages from parameters or pass prompt fields through `turn` inheritance.
- `messages`/`history` is the actual ordered context sent to a provider. Once a
  script passes `messages` to `turn`, that array is the context for that request.
- `turn` may receive `tools`, but it only exposes possible tool calls in
  `result.calls`. It does not execute the calls or append tool results.

For an agent without a custom `flow`, Leia runs the built-in loop: synthesize
messages from `system` and `user`, call one `turn`, dispatch any returned tool
calls, append assistant tool-call and tool-result messages, and repeat until
the provider returns a final answer or the workflow stops.

For an agent with `flow`, no hidden turns or dispatches occur. The flow decides
when to call `turn`, which `tools` list to pass, which tool calls to dispatch,
and what history to send into the next turn. The runnable example
[`examples/llm/manual_tool_history.leia`](../../../examples/llm/manual_tool_history.leia)
shows this manual pattern under a mock provider.

## Single Turns

`turn { ... }` performs one provider request and returns `(result, err)`.

```leia
result, err := turn {
    model: "fast"
    messages: messages { user: "Reply with ok." }
    max_tokens: 32
    temperature: 0
}
```

Important fields:

| Field | Meaning |
|---|---|
| `model` | Alias or provider model name. |
| `messages` | Message array. Required unless inherited by a higher-level agent form. |
| `tools` | Tool declarations, runtime tools, or agent values. |
| `force_tool` | Force one tool by name when supported by provider. |
| `max_tokens` | Output token limit. |
| `temperature` / `top_p` | Sampling controls. |
| `response_format` | Provider response-format hint. |
| `stream` | Request incremental provider output when supported. |
| `on_stream` / `onStream` | Optional script callback for streaming events. The callback receives `{type, token, text, status, reason, usage}` tables and automatically enables `stream`. |
| `stop` | Stop sequences. |
| `metadata` | String metadata passed to the provider. |

With `stream: true`, providers that implement streaming emit incremental
`turn_stream` trace events through `leia.WithLLMTrace`. The returned `result`
remains the complete final turn result. Providers without streaming support
ignore the hint and return normally.

With `on_stream`, scripts can consume provider tokens as they arrive while still
receiving the final complete result:

```leia
text := ""
result, err := llm.turn({
    messages: {llm.user("Reply slowly.")}
    on_stream: func(event) {
        text = text .. event.token
    }
})
```

When `model` is omitted, the turn uses the ambient agent model if it is running
inside an agent. Otherwise it uses the module's `models { default: ... }` alias
or the host provider's default behavior. Host-injected providers may ignore
script-declared provider config, but the script-level alias still names the
requested model.

Result shape:

| Field | Meaning |
|---|---|
| `status` | Provider status, usually `final_answer`, `tool_calls`, or `stop`. |
| `text` | Final text when available. |
| `calls` | Tool calls requested by the model. |
| `reason` | Stop or provider reason. |
| `usage` | Token/cost/latency usage fields when provider reports them. |

Errors are ordinary result values such as `{kind: "provider", message: "..."}`.

## Agents

An agent is a reusable function-shaped AI workflow.

```leia
agent answer(question) {
    model: "fast"
    system: "Use local documentation when useful."
    user: question
    tools: [search_runbook]
}

result, err := answer("What changed?")
```

Anonymous agents can be assigned and called:

```leia
summarize := agent(text) {
    system: "Return one sentence."
    user: text
}
result, err := summarize("...")
```

`agent defaults { ... }` supplies module-level defaults for later agents:

```leia
agent defaults {
    model: "fast"
    tools: [search_runbook]
    budget: {turns: 4, calls: 4, tokens: 2000}
}
```

Agent config fields are merged with defaults. Agent-local fields override
defaults. For the built-in loop, the merged config becomes:

- Model request settings: `model`, sampling fields, response format, metadata.
- Initial history: `system` followed by `user` when present.
- Tool availability: merged `tools`, including plain tools and agent-as-tool
  values.
- Limits and validation: `budget` and `output`.

## Custom Flow

Use `flow { ... }` when the built-in agent turn loop is not enough. The flow
body can access the merged agent config fields `model`, `system`, `tools`, and
`capabilities` as lexical bindings. These names are ordinary locals and can be
shadowed inside the flow body. Other config fields, including `user`, `budget`,
`response_format`, and `metadata`, are not injected as variables; `turn {}` can
still inherit them through the ambient agent configuration.

A flow owns history. To continue a conversation, append assistant messages,
assistant tool calls, and tool results to the same `messages` array before the
next `turn`. Passing `tools` to the next `turn` makes those tools available to
the model again; it still does not dispatch them automatically.

```leia
agent incident_brief(service) {
    model: "planner"
    system: "Create a brief incident update."
    tools: [search_runbook]
} flow {
    history := messages {
        system: system
        user: "Prepare brief for " .. service
    }
    first, err := turn { model: model, messages: history, tools: tools }
    if err != nil { return nil, err }
    call := first.calls[1]
    evidence, dispatch_err := llm.dispatch(call, tools)
    if dispatch_err != nil { return nil, dispatch_err }
    history[#history + 1] = msg.assistant_call(call)
    history[#history + 1] = msg.tool_result(call.id, evidence)
    return turn { model: model, messages: history, tools: tools }
}
```

## Agent As Tool

Agents may be placed directly in another agent's `tools` list:

```leia
agent extract(topic) {
    output: { summary: "short finding", confidence: 1 }
    user: "Research " .. topic
}

agent supervisor(question) {
    tools: [extract]
    user: question
}
```

The runtime wraps the agent as a tool and derives parameter names from the
agent signature. Structured `output` examples are used for output validation
and tool-result shape.

## Budgets And Cancellation

Budgets can be attached to agents or used as blocks:

```leia
budget { turns: 1, calls: 0, tokens: 256, time: 30 } {
    result, err := answer("short task")
}
```

Public budget dimensions are `turns`, `calls`, `tokens`, and `time`. Provider
usage may include cost metadata, but Leia does not promise money accounting as
a stable script-level budget dimension.

## Record, Replay, And Trace

Use host-side record/replay for deterministic tests:

```go
rec := llm.NewRecorder()
vm := leia.New(leia.WithLLMProvider(provider), leia.WithLLMRecorder(rec.Record))
// run script
_ = rec.Save("testdata/turns.json")

records, _ := llm.LoadRecords("testdata/turns.json")
vm = leia.New(leia.WithLLMReplay(records))
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
