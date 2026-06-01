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
exactly one request.

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
system prompt, input, tools, output expectations, and budget defaults.

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

Inside `flow`, agent config fields such as `model`, `system`, `user`, and
`tools` are available as lexical bindings.

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

- `examples/llm/agent.leia`: model defaults, tools, agent, and turn syntax.
- `examples/llm/agent_as_tool.leia`: direct agent-as-tool delegation.
- `examples/llm/incident_response.leia`: custom flow with manual dispatch.
- `examples/llm/glm_smoke.leia`: live provider smoke path.
