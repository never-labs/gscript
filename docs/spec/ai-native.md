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

## Stable Contract

The stable AI-native contract is the following lowering boundary:

| Source construct | Stable runtime meaning |
|---|---|
| `agent` without `flow` | A callable frame that runs the built-in loop. |
| `agent` with `flow` | A callable frame whose body is exactly the user flow. |
| `turn { ... }` | One provider request. It does not dispatch tools. |
| `messages { ... }` / history | The ordered transcript carried between turns. |
| `tool` | A callable tool table checked against declared capabilities. |
| `evaluate` | A source-level regression case discovered by `leia evaluate`. |

An agent frame owns ambient configuration: `model`, `system`, `user`, `tools`,
budget, capabilities, output shape, tracing, cancellation, replay, and
provider selection. A `turn` may inherit from that frame, but the turn remains
a single provider boundary operation. Tool dispatch is a separate operation
performed either by the built-in agent loop or by explicit user code.

`history` and `messages` name the same stable data shape: an ordered list of
normalized message tables. `messages { ... }` constructs such a list; a
history variable is just a program variable holding that list. Histories, not
tool lists or ambient config, are the durable evidence passed from one provider
turn to the next.

## Evaluate Blocks

`evaluate "case name" { ... }` declares a source-level agent regression case.
The case name is a required string literal. The block body uses ordinary Leia
statements, so tools can parse declarations, setup code, agent calls, and later
assertion syntax without relying on an external fixture format.

In ordinary script execution an evaluate block has no runtime effect. The
`leia evaluate` command owns discovery and evaluation semantics. The runner
discovers cases, reports their source position, validates the body with the
same parser and AI-native syntax checks used for normal code, then executes the
body as ordinary Leia code under the evaluation harness. Built-in `assert`
failures or other runtime errors mark that case failed.

During `leia evaluate`, the harness installs an `eval` module for dataset-style
regression tests. This module is not part of ordinary script execution.

| Function | Meaning |
|---|---|
| `eval.case(id, fn)` | Run one named subcase. Runtime errors inside `fn` fail that subcase but do not stop later subcases. Returns `(ok, err)`. |
| `eval.metric(name, value)` | Record a bool, number, string, nil, or JSON-like value on the active subcase, or on the outer evaluate block when no subcase is active. |
| `eval.load_jsonl(path)` | Load a JSON Lines corpus relative to the evaluate source file and return an array table of decoded rows. |
| `eval.skip_if(cond, reason)` | If `cond` is truthy, mark the active subcase skipped and return true. The caller should usually `return` after it. |
| `eval.fail_if(cond, message)` | If `cond` is truthy, raise a runtime error with `message`. |
| `eval.usage()` | Return current case LLM usage, including `turns`, token counts, latency, cost, tool calls, stream events, and provider errors. |
| `eval.budget(table)` | Raise a runtime error when current LLM usage exceeds a positive limit in `table`. Supported limits are `turns`, `tokens`, `input_tokens`, `output_tokens`, `latency_ms`, `cost`, and `money`. |
| `eval.judge(options)` | Run a bounded judge turn by calling `llm.turn(options)`. Missing `max_tokens` defaults to `200`; missing `budget` defaults to `{tokens: 512, turns: 1}`. Provider usage is recorded as `judge_*` metrics. |

The evaluation report preserves raw metrics on each case and subcase and also
includes top-level metric summaries. Boolean metrics are summarized as true,
false, and pass-rate counts. Numeric metrics are summarized as count, mean, min,
and max. String metrics are summarized as category counts. Skipped subcases do
not contribute to metric summaries.

The `leia evaluate` command may install a deterministic LLM replay provider or
record provider turns into a golden fixture. Replay fixtures are strict: a
request mismatch, an exhausted fixture, or leftover unconsumed turns is an
evaluation failure. Updating a golden fixture is an explicit command-line mode,
not an ordinary script side effect.

Evaluate cases should be written so their non-provider behavior is deterministic.
Runnable examples in this spec use local tools, custom flows, replay, or
validation helpers rather than live network LLMs.

```leia run all
//leia:requires none
tool lookup(topic) {
    return "docs:" .. topic, nil
}

agent answer(question) {
    model: "mock"
    user: question
    tools: [lookup]
}

evaluate "answer can use lookup" {
    result, err := lookup("evaluate")
    assert(err == nil)
    assert(result == "docs:evaluate")
}

return lookup.name
```

Inside an evaluation run, subcases and metrics can be recorded with the harness
module:

```text
evaluate "answer corpus" {
    rows := eval.load_jsonl("answer_cases.jsonl")
    for _, row := range rows {
        ok, err := eval.case(row.id, func() {
            if eval.skip_if(row.skip, "fixture disabled") {
                return
            }
            result, err := lookup(row.topic)
            assert(err == nil)
            eval.metric("correct", result == row.expected)
        })
        assert(ok || err != nil)
    }
}
```

## Models

`models { ... }` declares module-scoped model aliases and provider
configuration. A field key is the public alias used by script code. A field
value is either another alias string or a provider configuration table.

```text
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

The built-in loop is the default because an agent declaration is normally a
complete workflow specification. Adding `flow { ... }` replaces that loop with
the supplied code. It does not customize one step of the default loop; it owns
the whole workflow body.

Every agent call creates a fresh agent frame. The frame is visible to nested
`turn {}` calls as ambient configuration and to nested agent-as-tool calls as
shared provider, trace, replay, budget, cancellation, and approval context.

Named agents bind their name in the current scope:

```text
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

## Agents And Turns

`agent` and `turn` are different layers:

- an **agent** is a callable workflow frame with configuration, defaults,
  budget, tools, output handling, tracing, replay, and optional custom `flow`;
- a **turn** is one provider request made from inside or outside such a frame;
- a built-in agent, meaning an agent without `flow`, repeatedly performs turns
  and dispatches tools for the caller;
- a custom-flow agent, meaning an agent with `flow`, performs no hidden turns
  and no hidden tool dispatch. It runs ordinary Leia code, and the author must
  call `turn { ... }`, `llm.dispatch`, or other helpers explicitly.

The stable invariants are:

1. A `turn` consumes a request table and produces one provider result or error.
2. Passing `tools` to a turn only serializes available tool schemas into that
   provider request.
3. A provider tool-call result is data. It has no effect until dispatched.
4. Dispatch results become model-visible only after the surrounding code appends
   paired assistant-call and tool-result or tool-error messages to history.
5. A later turn sees prior work only through its `messages` field.

The default agent loop can be understood as this conceptual shape:

```text
agent answer(question) {
    model: "fast"
    system: "Use tools when useful."
    user: question
    tools: [lookup]
}

// Calling the agent runs the built-in loop:
// 1. build messages from system + user;
// 2. call turn { model, messages, tools };
// 3. if result.status == "tool_calls", dispatch requested tools;
// 4. append tool results to history;
// 5. call another turn with the updated history;
// 6. validate structured output when configured;
// 7. stop on final answer, provider stop, budget, approval, cancellation, or error.
result, err := answer("What changed?")
```

The same structure can be written manually with a custom `flow` when the
program needs precise multi-turn control:

```text
agent answer(question) {
    model: "fast"
    system: "Use tools when useful."
    tools: [lookup]
} flow {
    history := messages {
        system: system
        user: question
    }

    first, err := turn {
        messages: history
        tools: tools
    }
    if err != nil {
        return nil, err
    }

    if first.status == "tool_calls" {
        call := first.calls[1]
        value, tool_err := llm.dispatch(call, tools)
        if tool_err != nil {
            history[#history + 1] = msg.tool_error(call.id, tool_err.message)
        } else {
            history[#history + 1] = msg.assistant_call(call)
            history[#history + 1] = msg.tool_result(call.id, value)
        }
        return turn { messages: history }
    }

    return first, nil
}
```

Both examples use the same `turn` operation at the provider boundary. The
difference is where the loop policy lives: in the built-in agent loop, or in
user-written `flow` code.

```leia run all
agent scripted(question) {
    model: "local"
    system: "s"
    tools: [{name: "lookup"}]
} flow {
    planned := messages {
        system: system
        user: question
    }
    return {
        inherited_model: model,
        inherited_tools: #tools,
        message_count: #planned,
    }, nil
}

out, err := scripted("hello")
return out.inherited_model, out.inherited_tools, out.message_count, err
```

An important consequence is that `turn {}` never calls tools by itself. Passing
`tools` to a turn only tells the provider which tool schemas are available. If
the provider responds with tool calls, either the built-in agent loop or custom
flow code must dispatch them and then make a later turn with the updated
history.

Agent results from the built-in loop include the final provider data and the
history accumulated by the loop. The stable fields are:

| Field | Meaning |
|---|---|
| `status` | `done`, `tool_calls`, `stop`, `approval_pending`, or error-specific status. |
| `text` | Final answer text when available. |
| `value` | Structured value when `output` validation succeeds. |
| `history` | Final message history accumulated by the loop. |
| `usage` | Aggregated provider usage when available. |

Custom-flow agents return whatever the flow returns. They do not receive the
built-in agent result wrapper unless the flow constructs it explicitly.

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

Ambient configuration is not a lexical scope. It is a request-construction
input read by AI runtime operations. In particular:

| Item | Ambient inheritance | Flow-local identifier |
|---|---:|---:|
| `model` | yes | only when explicit |
| `system` | yes | only when explicit |
| `user` | yes, for synthesized messages | no |
| `tools` | yes | only when explicit |
| `budget` | yes | no |
| `output` | yes | only when explicit |
| provider hints | yes | no |

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

The flow receives ordinary call parameters through its lexical scope. For prompt
inputs, prefer using parameters directly (`q` in the examples) over relying on
the `user` config field, because `user` is ambient prompt configuration and is
not injected as a local.

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

`messages { ... }` constructs an ordered message list. A history is any value
with that same ordered list shape. Role fields are converted to normalized
message tables:

```text
history := messages {
    system: "Use evidence."
    user: "Find release notes."
}
```

The stable shorthand roles are `system`, `user`, and `assistant`. A field whose
key is not one of those roles becomes `{role: key, text: value}`. A field
without a key is inserted as-is, which allows incremental histories to mix
message helper results in the same block.

The stable normalized fields are:

| Message kind | Required stable fields |
|---|---|
| text message | `role`, `text` |
| assistant tool call | `role: "assistant"`, `tool_call` |
| tool result | `role: "tool"`, `tool_use_id`, `value` |
| tool error | `role: "tool"`, `tool_use_id`, `error` or error text |

Implementations may carry provider-specific metadata, but code that wants to be
portable must preserve at least these fields.

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

Histories are the only stable handoff between provider turns. A tool list in a
request is schema/context for the provider; it is not evidence by itself. When
a provider requests a tool call, the call and its result become visible to a
later turn only if the surrounding agent loop or custom flow appends both an
assistant tool-call message and a matching tool-result or tool-error message to
the next history.

Tool-call messages and tool-result messages are paired by call id. The
assistant message records the provider's requested tool name and arguments. The
tool message records the local dispatch result, recoverable dispatch error, and
the same call id. Implementations must preserve this pairing when lowering the
built-in agent loop, and user-written flows should preserve it when constructing
manual histories.

```leia run all
history := messages {
    system: "s"
    user: "u"
}

history[#history + 1] = msg.assistant("draft")
history[#history + 1] = msg.user("revise")

return #history, history[3].role, history[4].text
```

```leia run all
//leia:requires none
//leia:param topic topic to look up
tool lookup(topic) {
    return "note:" .. topic, nil
}

history := messages {
    system: "Use tool evidence."
    user: "Find docs for Leia."
}

first := {
    status: "tool_calls",
    calls: {{
        id: "call_lookup_1",
        tool: "lookup",
        args: {topic: "Leia"},
    }},
}

call := first.calls[1]
value, err := llm.dispatch(call, [lookup])
if err != nil {
    history[#history + 1] = msg.tool_error(call.id, err.message)
} else {
    history[#history + 1] = msg.assistant_call(call)
    history[#history + 1] = msg.tool_result(call.id, value)
}

return #history, history[3].tool_call.tool, history[4].tool_use_id, history[4].value, err
```

## Turns

`turn { ... }` performs exactly one provider request and returns `(result,
err)`. It never dispatches tools by itself. If the provider returns tool calls,
the result has `status: "tool_calls"` and the caller or surrounding agent loop
decides whether to dispatch them.

The operation is equivalent to:

1. merge explicit request fields over ambient agent configuration, if any;
2. synthesize `messages` from `system` and `user` only when `messages` is absent;
3. normalize tools, including agent-as-tool wrappers;
4. check budget and cancellation;
5. call the provider once;
6. return the provider result or a structured error.

No step above calls `llm.dispatch`.

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
| `stream` | Request incremental provider output when supported. |
| `on_stream` / `onStream` | Optional script callback for streaming events. The callback receives one event table per provider event and automatically enables `stream`. |
| `stop` | Stop sequences. |
| `metadata` | Host/provider metadata. |

When `stream` is true and the installed provider implements streaming, the host
trace sink receives `turn_stream` events for incremental text tokens. Streaming
does not change script-level return values: `turn { ... }` still returns one
complete result table and one error value after the provider finishes. If the
provider does not support streaming, the field is a provider hint and the turn
falls back to the ordinary complete-result path.

When `on_stream` is a function, the implementation calls it with an event table
containing `type`, `token`, `text`, `status`, `reason`, and `usage`. The
callback is best used for UI updates, logs, or incremental buffers; the final
turn result remains authoritative.

If a `turn` runs inside an agent, absent `model`, `tools`, `budget`,
`response_format`, `metadata`, `max_tokens`, and sampling fields inherit from
the ambient agent configuration. If both `messages` and `user` are absent, an
ambient agent `system` and `user` are used to synthesize messages. Outside an
agent, a `turn` must have explicit `messages` or `user`, unless the host
installs a provider layer with different documented behavior.

Inside a custom `flow`, inheritance and local variables are related but not the
same thing. Ambient inheritance affects `turn {}` request construction.
Flow-local injection only creates identifiers such as `model`, `system`, and
`tools` for fields explicitly written in the agent configuration. For example:

```text
agent inherit(q) {
    model: "fast"
    system: "Brief."
    user: q
} flow {
    // Uses inherited model, system, and user to build the request.
    first, err := turn {}

    // Equivalent explicit form for the message list, while still inheriting
    // model and other omitted request fields.
    second, err := turn {
        messages: messages {
            system: system
            user: q
        }
    }

    return second, err
}
```

Outside an agent there is no ambient frame, so request construction must be
explicit:

```text
result, err := turn {
    model: "fast"
    messages: messages {
        system: "Brief."
        user: "Explain Leia."
    }
}
```

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

The stable validation rule is structural:

| Example value | Accepted decoded value |
|---|---|
| string | string |
| number | number |
| boolean | boolean |
| table object | table with the same required keys recursively validated |

Extra fields may be preserved. Missing required fields fail. Arrays and richer
schemas are provider or library extensions until specified separately.

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
json_ok, _ := llm.validate_output("{\"summary\":\"done\"}", {summary: "x"})
return ok, ok_msg, bad, bad_msg != "", json_ok
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

Wrapping an agent does not make it a provider request. It is still a local tool
dispatch path until that tool is advertised to a provider by a later `turn` or
built-in agent loop.

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

```text
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

## Record, Replay, And Evaluate

Record/replay observes provider turns, not agents, tools, or arbitrary runtime
values. A recorded turn contains the normalized request sent to the provider
and the provider result or provider error returned from that exact boundary.
Local tool execution is represented indirectly: the next recorded request
contains any assistant-call and tool-result messages that the loop or flow
appended to history.

Replay is strict and ordered:

1. each provider request must match the next fixture request after model alias
   resolution and request normalization;
2. replay returns the fixture result instead of calling a live provider;
3. an unexpected request, exhausted fixture, or unconsumed fixture record is an
   evaluation failure;
4. replay does not hide local tool, validation, budget, or cancellation errors.

`leia evaluate` composes this with source-level cases. Each `evaluate` block is
a named regression case. The harness may run the case with a live provider in
explicit record mode, then persist the provider turns as a golden fixture. In
normal replay mode, the same case must consume that fixture exactly. Fixture
updates are command-driven and must never occur as a side effect of `leia run`.

```leia run all
record := {
    request: {
        model: "mock-chat"
        messages: messages { user: "draft" }
        tools: {}
    }
    result: {status: "final_answer", text: "ok"}
}

return record.request.model, record.request.messages[1].text, record.result.text
```
