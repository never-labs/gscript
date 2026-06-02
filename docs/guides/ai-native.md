# AI-Native Leia

Leia's AI-native surface is designed for scripts that call models, use tools,
keep message history, validate structured output, and replay agent behavior in
tests. The syntax is concise for scripts; the Go host still controls providers,
credentials, capabilities, tracing, and replay.

## Mental Model

Use the simplest layer that fits the workflow:

| Layer | Use |
|---|---|
| `turn { ... }` | One model request. |
| `agent { ... }` | Reusable model-backed function. |
| `tool name(...) { ... }` | Script function callable by a model. |
| `flow { ... }` | Custom multi-turn loop. |
| `llm.*`, `msg.*`, `history.*` | Lower-level runtime helpers. |

The stable contract is in the [AI-native reference](../reference/ai/index.md).

## How The Pieces Fit

`turn` is the primitive: it sends one request with one message history. `agent`
is a reusable wrapper around turns. An agent can build history from `system` and
`user`, inherit a default model and tools, and run the built-in tool loop. Add
`flow` only when the script must own that loop.

Smallest shape:

```leia
result, err := turn {
    messages: messages {
        user: "Return one sentence about Leia."
    }
}
```

Reusable shape with defaults, assuming `lookup_runbook` is a declared tool:

```leia
agent defaults {
    model: "fast"
    tools: [lookup_runbook]
}

agent answer(question) {
    system: "Use tool evidence when it helps."
    user: question
}
```

Here `answer` inherits `model` and `tools`. The runtime creates history from the
agent's `system` and `user`, calls a turn, dispatches requested tools, appends
tool messages, and repeats until the model returns a final answer.

Custom shape:

```leia
agent answer_with_review(question) {
    model: "fast"
    system: "Use tool evidence and then write a final answer."
    tools: [lookup_runbook]
} flow {
    h := messages {
        system: system
        user: question
    }
    first, err := turn { model: model, messages: h, tools: tools }
    if err != nil { return nil, err }
    return first, nil
}
```

Inside `flow`, the script decides which history and tools each `turn` receives.
The merged agent config exposes `model`, `system`, `tools`, and
`capabilities` as ordinary lexical bindings. Other fields such as `user`,
`budget`, `response_format`, and `metadata` stay in the ambient agent config
and are inherited by `turn` when omitted, but they are not injected as local
variables. No tool call is dispatched unless the flow calls `llm.dispatch`.

## Start Offline

Most tests should use a mock or replay provider from Go. That keeps tests
deterministic and avoids committing secrets.

```go
vm := leia.New(
	leia.WithLLMProvider(mockProvider),
	leia.WithLLMRecorder(rec.Record),
)
```

Replay later:

```go
records, err := llm.LoadRecords("testdata/turns.json")
if err != nil {
	return err
}
vm := leia.New(leia.WithLLMReplay(records))
```

## Configure Models

Scripts can name models, while the host decides whether script-declared model
configs are honored.

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

Never put API keys in source. Use environment variables or host-injected
providers.

## One Turn

```leia
result, err := turn {
    model: "fast"
    messages: messages {
        system: "Be concise."
        user: "Return exactly: ok"
    }
    max_tokens: 16
    temperature: 0
}

if err != nil {
    return nil, err
}
print(result.text)
```

Use `turn` when the script owns the message history and the provider should do
exactly one request. If a `turn` is inside an agent, it can inherit ambient
agent settings such as the default model; pass `messages` explicitly when the
flow owns the history.

## Agent

```leia
summarize := agent(text) {
    model: "fast"
    system: "Return one short sentence."
    user: text
    output: { summary: "string" }
}

result, err := summarize("Leia embeds into Go and supports hot reload.")
```

Use `agent` when you want a function-shaped abstraction that carries model,
system prompt, input, tools, output expectations, and budget defaults. Without a
custom `flow`, the agent converts `system` and `user` into the first history and
runs the built-in turn/tool loop.

## Tools

Tools are ordinary Leia functions with model-facing metadata.

```leia
// Looks up a local runbook.
//leia:requires docs.read
//leia:param service production service name
tool lookup_runbook(service) {
    return {service: service, steps: {"check metrics", "restart if needed"}}, nil
}

agent support(question) {
    model: "fast"
    system: "Use tools before answering operational questions."
    user: question
    tools: [lookup_runbook]
}
```

Use `//leia:requires none` for pure local tools. Capability comments make tool
use auditable by lint and module tooling.

## Multi-Turn Flow

Use `flow` when you need explicit history, tool dispatch, repair, or custom
branching.

```leia
agent incident(service) {
    model: "fast"
    system: "Create a short incident update from verified evidence."
    tools: [lookup_runbook]
} flow {
    h := messages {
        system: system
        user: "Investigate " .. service
    }

    first, err := turn { model: model, messages: h, tools: tools }
    if err != nil {
        return nil, err
    }

    call := first.calls[1]
    evidence, dispatch_err := llm.dispatch(call, tools)
    if dispatch_err != nil {
        return nil, dispatch_err
    }

    h[#h + 1] = msg.assistant_call(call)
    h[#h + 1] = msg.tool_result(call.id, evidence)
    return turn { model: model, messages: h, tools: tools }
}
```

Inside `flow`, the lexical bindings are the same stable subset described
above: `model`, `system`, `tools`, and `capabilities`. Build the user message
from function parameters or pass prompt fields through `turn {}` inheritance
instead of relying on a `user` local.

## Agent As Tool

Agents can be used directly as tools for a supervisor agent:

```leia
agent extract_memory(note) {
    system: "Extract project and owner."
    user: note
    output: { project: "ORCHID", owner: "ADA" }
}

agent supervisor(question) {
    model: "fast"
    tools: [extract_memory]
    user: question
}
```

The runtime wraps the agent as a tool and derives parameter names from the
agent signature. This avoids maintaining duplicate wrapper schemas.

## Budgets

Set budgets close to the workflow:

```leia
agent defaults {
    budget: {turns: 4, calls: 4, tokens: 2000, time: 30}
}

budget { turns: 1, calls: 0, tokens: 256, time: 10 } {
    result, err := turn { user: "short answer only" }
}
```

Budget errors are ordinary `err` results. Host-side context cancellation still
applies to provider calls.

## Live Provider Tests

Live tests must be opt-in:

```bash
LEIA_LLM_INTEGRATION=1 \
LEIA_ANTHROPIC_COMPAT_BASE_URL=https://provider.example/v1 \
LEIA_ANTHROPIC_COMPAT_API_KEY=... \
LEIA_ANTHROPIC_COMPAT_MODEL=... \
go test ./tests/integration/llm -run GLM -count=1
```

The GLM path also accepts:

```bash
LEIA_GLM_BASE_URL=...
LEIA_GLM_API_KEY=...
LEIA_GLM_MODEL=...
```

Do not commit provider tokens, recorded secrets, raw prompts that contain
secrets, or live-provider outputs that expose private data.

## Useful Examples

Offline examples:

- `examples/llm/agent.leia`: model defaults, tools, agent, and turn syntax.
- `examples/llm/incident_response.leia`: custom flow with manual dispatch.

Live-provider examples:

- `examples/llm/glm_smoke.leia`: live provider smoke path.
