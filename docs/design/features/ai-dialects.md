# AI Dialects

## Goal

Leia should expose AI as official dialects, not only as `llm.*` function calls.
The existing `llm` runtime remains the substrate, but user-facing code should be
able to express models, turns, tools, agents, memory, replay, and evaluation as
DSL-native Leia.

## Core Dialects

```text
model
turn
tool
agent
evaluate
```

`prompt` and `quote` should not be high-priority public dialects unless they
gain clear structured behavior beyond ordinary interpolated strings.

## Model

```leia
model "glm" {
    protocol: "anthropic-compatible"
    base_url: env("LEIA_GLM_BASE_URL")
    api_key: env("LEIA_GLM_API_KEY")
    model: env("LEIA_GLM_MODEL")
}
```

Requirements:

- named model aliases;
- explicit provider protocol;
- no secrets committed to source;
- environment-variable use should be straightforward;
- model aliases must be inspectable by tools.

## Turn

```leia
result := turn {
    model: "glm"
    system: `Return one concise sentence.`
    user: `Explain ${topic}.`
    max_tokens: 128
}
```

A turn performs exactly one provider request. It does not dispatch tools by
itself. Tool calls returned by the provider are data for an agent loop or user
flow to handle.

## Tool

Preferred first form:

```leia
lookup_order := tool {
    name: "lookup_order"
    desc: "Look up order status."
    params: {
        order_id: "string"
    }
    caps: ["db.read"]
    run: fn(order_id) {
        return orders[order_id], nil
    }
}
```

Requirements:

- tool name;
- description;
- parameter schema or names;
- capabilities;
- callable body;
- structured error return.

## Agent

```leia
support := agent {
    model: "glm"
    system: `You are a support agent.`
    tools: [lookup_order]
    output: {
        action: "refund"
        confidence: 0.9
        reason: ""
    }
}

result := support(`Order ${order_id} arrived damaged.`)
```

Requirements:

- agent is a callable value;
- agent may have default model/system/tools/output schema;
- agent may inherit module-level defaults;
- agent can be passed as a tool to another agent;
- output validation should be available but not mandatory;
- streaming should be supported where the provider supports it;
- record/replay should work for deterministic tests.

## Agent Flow

Simple agents use the built-in loop. Custom flows are needed for advanced
multi-turn logic:

```leia
researcher := agent {
    model: "glm"
    tools: [search, read_url]

    flow: fn(question) {
        first := try turn {
            user: question
            tools: tools
        }
        return first
    }
}
```

Flow semantics:

- no hidden turns inside a custom flow;
- no hidden tool dispatch;
- the flow author controls history;
- ambient model/tools may be available as explicit flow bindings.

## Evaluate

```leia
evaluate {
    name: "refund classifier"
    cases: glob`tests/refund/*.jsonl`
    run: fn(case) {
        result := support(case.input)
        metric("correct", result.action == case.expected)
    }
}
```

Requirements:

- named evaluations;
- case iteration;
- metrics;
- record/replay support;
- CI-friendly result report;
- deterministic replay mode;
- optional real-provider mode.

## Composition

AI dialects must compose with:

- `glob` for case discovery;
- `json/jsonl/csv` for corpora;
- `data` for evaluation metrics;
- `workflow` for CI;
- `record/replay` for deterministic regression;
- `approve` for human-in-the-loop operations.

## Non-Goals

- Do not make `agent`, `turn`, or `tool` hardcoded parser keywords.
- Do not require `prompt` or `quote` wrappers around normal text.
- Do not make `turn` dispatch tools automatically.
