# AI-Native Syntax

Leia's AI-native syntax is a language layer over the `llm`, `msg`,
`history`, and `loop` standard-library modules. The syntax is stable only when
it uses the same provider, tracing, record/replay, cancellation, budget, and
capability paths as direct library calls. Implementations must desugar the
syntax to those runtime paths rather than maintaining a separate AI execution
engine.

An AI operation is deterministic until it reaches a provider, host callback, or
tool body with side effects. Tests and replay fixtures should therefore record
provider turns at the host boundary.

## Models

`models { ... }` declares module-scoped model aliases and provider
configuration. A field key is the public alias used by script code. A field
value is either another alias string or a provider configuration table.

```leia
models {
    default: "fast"
    fast: {
        protocol: "anthropic_compatible"
        provider_model: "glm-4.5"
    }
}
```

`default` is the model used when neither a `turn` nor an ambient agent
configuration supplies `model`. `default` may point to another alias. Alias
cycles are validation errors. If an alias maps to a provider configuration, the
host's provider factory may resolve it to an actual provider model name; if no
factory is installed, the configuration is still visible to tooling but cannot
perform live requests.

Source code must not embed API keys as string literals. Credentials belong in
environment variables, host-provided configuration, or host-owned provider
factories.

## Tools

`tool name(params) { body }` declares a callable tool value in the surrounding
lexical scope. The declaration desugars to a runtime tool table containing the
tool name, function, parameter names, description, and required capabilities.
The body follows ordinary function rules and should return `(value, nil)` on
success or `(nil, err)` on recoverable failure.

Leading `//leia:` comments immediately above the declaration define stable
tool metadata:

| Directive | Meaning |
|---|---|
| `//leia:requires none` | The tool requires no host capability. |
| `//leia:requires a,b` | The tool requires every listed capability. |
| `//leia:param name text` | Describes one declared parameter. |

Every stable `tool` declaration must have an explicit `//leia:requires`
directive. Duplicate tool names, duplicate parameter docs, unknown parameter
docs, malformed capability names, and `none` mixed with other capabilities are
validation errors.

```leia run all
// Searches local project notes.
//leia:requires none
//leia:param topic topic to search
tool search_notes(topic) {
    return "notes:" .. topic, nil
}

value, err := search_notes.fn("runtime")
return search_notes.name, value, err
```

Tool values are ordinary values. They can be placed in `tools` lists, passed to
`llm.dispatch`, inspected with `llm.tool_caps`, or constructed dynamically with
`llm.tool` and `llm.toolof` when syntax is not convenient.

## Agents

An `agent` is a callable workflow value. Calling an agent returns `(result,
err)`. Without a custom `flow`, the runtime executes the built-in ReAct-style
loop: synthesize an initial message history from configuration, perform turns,
dispatch requested tools, append tool results, and stop on a final answer,
provider stop, budget error, approval pause, cancellation, or unrecoverable
tool/provider error.

Named agents bind their name in the current scope:

```leia
agent summarize(text) {
    system: "Return one concise paragraph."
    user: text
}
```

Anonymous agents are expressions and may be assigned, passed, or immediately
called:

```leia run all
echo := agent(text) {
    model: "mock"
    user: text
} flow {
    return {text: text, model_seen: model}, nil
}

out, err := echo("hello")
return out.text, out.model_seen, err
```

The no-parameter form `agent { ... }` is equivalent to an anonymous agent with
an empty parameter list.

## Agent Configuration

Agent configuration is a table-like field block. The stable fields are:

| Field | Meaning |
|---|---|
| `model` | Model alias or provider model name. |
| `system` | System prompt used when messages are synthesized. |
| `user` | User prompt or expression used when messages are synthesized. |
| `tools` | Ordered list of tool values or agent values. |
| `capabilities` / `caps` | Capability names available to the agent frame. |
| `budget` | `{turns, calls, tokens, time}` limits for the agent frame. |
| `output` | Example output shape for structured validation. |
| `output_retries` | Number of structured-output repair attempts. |
| `output_repair` | Repair prompt or truthy flag for default repair prompt. |
| `response_format` | Provider response-format hint. |
| `max_tokens` | Provider output token hint. |
| `temperature` / `top_p` | Sampling hints. |
| `metadata` | Host/provider metadata. |
| `description` | Tool-facing agent description when used as a tool. |
| `approve_when` | HITL predicate for tool calls. |

`agent defaults { ... }` declares module-level defaults for later agents in
the same module. There may be at most one defaults declaration per module.
Defaults are merged when an agent is called:

1. host defaults and registered `models`;
2. module `agent defaults`;
3. the agent's own configuration;
4. explicit fields on an inner `turn`.

For scalar fields, later values replace earlier values. `tools` is inherited as
one ordered list; an agent that sets `tools` replaces the default list rather
than appending to it. `budget` dimensions compose by taking every specified
limit from the closest frame and leaving unspecified dimensions inherited.
The merged configuration is ambient for `turn {}` inheritance. Flow-local
identifier injection, described below, is generated from fields written
directly in the agent declaration or literal.

```leia run all
agent defaults {
    model: "fast"
    system: "default system"
    budget: {turns: 2}
}

agent probe(q) {
    model: "fast"
    system: "agent system"
    user: q
} flow {
    return {system: system, model: model}, nil
}

out, err := probe("hello")
return out.system, out.model, err
```

## Flow Scope

`flow { ... }` supplies a custom agent body. The flow body is ordinary lexical
code with a small, explicit set of generated locals from the agent's explicit
configuration block:

| Injected local | Source |
|---|---|
| `model` | explicit `model` field |
| `system` | explicit `system` field |
| `tools` | explicit `tools` field |
| `capabilities` | explicit `capabilities` field |
| `caps` | explicit `caps` field |
| `output` | explicit `output` field |

Only fields present in that explicit block create locals; omitted fields do not
create `nil` locals. Defaults can still affect ambient `turn {}` behavior, but
they do not create flow-local identifiers. No other config field is injected.
In particular, `user`, `budget`, `response_format`, `metadata`, `max_tokens`,
and sampling fields remain ambient configuration for `turn {}` but are not
local variables. User declarations may shadow injected locals under the normal
lexical scoping rules.

```leia run all
user := "outer"

agent scope(q) {
    model: "m"
    system: "s"
    capabilities: ["read"]
    user: q
} flow {
    observed := model .. "|" .. system .. "|" .. capabilities[1] .. "|" .. user
    model := "local"
    return {observed: observed, shadowed: model}, nil
}

out, err := scope("inner")
return out.observed, out.shadowed, err
```

## Messages

`messages { ... }` constructs an ordered message list. Role fields are
converted to normalized message tables:

```leia
history := messages {
    system: "Use evidence."
    user: "Find release notes."
}
```

The stable shorthand roles are `system`, `user`, and `assistant`. A field whose
key is not one of those roles becomes `{role: key, text: value}`. A field
without a key is inserted as-is, which allows incremental histories to mix
message helper results in the same block.

```leia run all
call := {id: "call_1", tool: "lookup", args: {query: "leia"}}
history := messages {
    system: "Use tool evidence."
    user: "Find docs."
    msg.assistant_call(call)
    msg.tool_result("call_1", {summary: "found"})
    user: "Summarize."
}

return #history, history[1].role, history[3].tool_call.tool, history[4].value.summary
```

For histories built across multiple turns, append helper results explicitly:
`msg.system`, `msg.user`, `msg.assistant`, `msg.assistant_call`,
`msg.tool_result`, and `msg.tool_error`.

## Turns

`turn { ... }` performs exactly one provider request and returns `(result,
err)`. It never dispatches tools by itself. If the provider returns tool calls,
the result has `status: "tool_calls"` and the caller or surrounding agent loop
decides whether to dispatch them.

Stable request fields include:

| Field | Meaning |
|---|---|
| `model` | Alias or provider model name. |
| `messages` | Ordered message list. |
| `user` | Shorthand for a one-message user history when `messages` is absent. |
| `tools` | Ordered tool list; agent values are auto-wrapped as tools. |
| `force_tool` | Request one named tool when supported by the provider. |
| `max_tokens` | Output token hint. |
| `temperature` / `top_p` | Sampling hints. |
| `response_format` | Provider response-format hint. |
| `stream` | Provider streaming hint. |
| `stop` | Stop sequences. |
| `metadata` | Host/provider metadata. |

If a `turn` runs inside an agent, absent `model`, `tools`, `budget`,
`response_format`, `metadata`, `max_tokens`, and sampling fields inherit from
the ambient agent configuration. If both `messages` and `user` are absent, an
ambient agent `system` and `user` are used to synthesize messages. Outside an
agent, a `turn` must have explicit `messages` or `user`, unless the host
installs a provider layer with different documented behavior.

Result fields are stable:

| Field | Meaning |
|---|---|
| `status` | `final_answer`, `tool_calls`, `stop`, or provider-specific status. |
| `text` | Final text when available. |
| `calls` | Tool calls requested by the provider. |
| `reason` | Stop reason. |
| `usage` | Usage metadata such as token counts. |

Recoverable failures return `nil, err`; see [Errors](errors.md) for error
object conventions.

## Structured Output

`output` is an example shape, not a static type declaration. When a built-in
agent loop receives a final text response and `output` is present, it parses
the text as JSON, validates that the decoded table has the same required
fields and primitive shapes as the example, and returns the parsed value in the
agent result. Missing fields, type mismatches, non-object JSON, trailing JSON,
and invalid JSON are validation errors.

If `output_retries` or `output_repair` is configured, the built-in loop may
perform additional repair turns before returning a validation error. Repair
turns consume normal turn and token budgets.

A custom `flow` that returns arbitrary values is not automatically validated.
Inside a flow, use `llm.validate_output(value, output)` when the flow wants to
enforce the same shape explicitly. A `turn {}` inside a flow still inherits an
ambient `output` as a provider `response_format` hint when no explicit
`response_format` is supplied.

```leia run all
ok, ok_msg := llm.validate_output({summary: "done", score: 1}, {summary: "x", score: 0})
bad, bad_msg := llm.validate_output({summary: 1}, {summary: "x"})
return ok, ok_msg, bad, bad_msg != ""
```

## Agent As Tool

An agent value may appear directly in another agent's `tools` list or in a
`turn` tools list. The runtime wraps it as a tool at the boundary. The wrapper
derives the tool name from the named agent when available, derives parameter
names from the agent signature, uses `description` or `system` as the tool
description, and uses `output` as the tool-result shape.

When the wrapper calls the delegated agent:

1. tool call arguments are mapped to agent parameters by name;
2. the agent is executed under the same provider, budget, trace, replay, and
   cancellation paths as an ordinary agent call;
3. successful agent results are unwrapped so structured `result.value` becomes
   the tool result;
4. pending approval, cancellation, provider, validation, and tool failures are
   returned as structured tool errors.

```leia run all
agent extract(topic) {
    description: "Extract one finding."
    output: {summary: "short finding"}
} flow {
    return {value: {summary: "finding:" .. topic}}, nil
}

wrapped := llm.agent_as_tool(extract)
value, err := wrapped.fn("runtime")
return wrapped.name, value.summary, err
```

## Budgets, Cancellation, And Errors

Public budget dimensions are `turns`, `calls`, `tokens`, and `time`. Provider
usage may include cost metadata, but money accounting is not a stable
script-level budget dimension.

```leia
budget { turns: 2, calls: 4, tokens: 1000, time: 30 } {
    result, err := turn { user: "short answer" }
}
```

Budgets are checked before turns, before tool calls, and after provider usage
is charged. Cancellation is checked before each provider turn and before each
tool dispatch. Recoverable provider, budget, validation, cancellation, approval,
and tool failures return structured `nil, err` results unless a specific API
documents a runtime error.

Trace events must avoid prompt text and tool-result values unless explicitly
configured by the host. The stable trace surface is metadata such as event
type, model, step, status, message count, tool count, tool name, and usage.
