# AI-Native Syntax

AI-native syntax is a language layer over the `llm`, `msg`, `history`, and
`loop` standard-library modules. It must use the same provider, tracing,
record/replay, cancellation, and capability paths as direct library calls.

## Models

`models {}` declares model aliases and provider configuration. `default` may
refer to another alias. Alias cycles are invalid. Source code must not embed
API keys as string literals; credentials belong in environment variables or
host-provided configuration.

```leia
models {
    default: "fast"
    fast: {
        protocol: "anthropic_compatible"
        provider_model: "glm-4.5"
    }
}
```

The host supplies credentials and transport details for `fast`.

## Tools

`tool name(params) { body }` declares a callable tool value. Leading
`//leia:` comments define tool metadata such as capability requirements and
parameter descriptions. Every stable tool declaration must have an explicit
`//leia:requires` directive, using `none` when no capability is required.

```leia
// Searches local project notes.
//leia:requires docs.read
//leia:param topic topic to search
tool search_notes(topic) {
    return "notes:" .. topic, nil
}
```

## Agents

An `agent` is a callable workflow value. Agent configuration supplies defaults
for turns executed by the agent. Explicit fields on a `turn` override agent
configuration; agent configuration overrides host defaults.

Named agents bind their name in scope. Anonymous `agent { ... }` or
`agent(params) { ... }` expressions produce values that may be assigned or
called.

`flow { ... }` supplies a custom agent body. The flow body is lexical code.
Only `model`, `system`, `tools`, and `capabilities` are injected as flow-local
bindings from merged agent configuration. User declarations in the flow body
may shadow those names. `user`, `budget`, `response_format`, and `metadata` are
ambient configuration, not injected variables.

```leia
agent summarize(text) {
    model: "fast"
    system: "Return one concise paragraph."
    user: text
}

answer, err := summarize("Explain Leia agents.")
```

Custom flow code can control multiple turns explicitly:

```leia
agent research(topic) {
    system: "Use tools before answering."
    tools: [search_notes]
    user: topic
} flow {
    history := messages {
        system: system
        user: topic
    }
    first, err := turn { messages: history, tools: tools }
    if err != nil {
        return nil, err
    }
    return first, nil
}
```

## Turns And Messages

`turn { ... }` performs one provider request and returns `(result, err)`.

```leia
result, err := turn {
    model: "fast"
    messages: messages {
        system: "Be precise."
        user: "Reply with ok."
    }
    max_tokens: 32
}
```

`messages { ... }` constructs an ordered message list. Static histories may use
role fields such as `system`, `user`, and `assistant`; computed histories may
use message helper modules directly.

```leia
history := messages {
    system: "Use tool evidence."
    user: "Find release notes."
}
history[#history + 1] = msg.assistant("I will search.")
history[#history + 1] = msg.tool_result("call_1", { summary: "found" })
```

## Budgets

Public budget dimensions are `turns`, `calls`, `tokens`, and `time`. Provider
usage may include cost metadata, but money accounting is not a stable
script-level budget dimension.

```leia
budget { turns: 2, calls: 4, tokens: 1000, time: 30 } {
    result, err := research("runtime specialization")
}
```

## Errors

Recoverable provider, budget, validation, and tool failures return structured
`nil, err` results unless an API explicitly documents a runtime error. Trace
events must avoid prompt and tool-result leakage unless explicitly configured
by the host.
