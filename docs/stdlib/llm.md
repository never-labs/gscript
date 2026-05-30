# LLM, Agents, and History

The `llm`, `msg`, `history`, `chat`, and `loop` tables are the standard library
target for AI-native syntax. The parser may accept `agent`, `turn`, `tool`,
`messages`, `models`, and direct agent tools, but those forms lower to ordinary
stdlib calls and data tables. Hosts can therefore embed, test, record, replay,
or sandbox the AI runtime through the same library surface.

Enable the library from Go with `LibLLM` and a provider:

```go
vm := gscript.New(
    gscript.WithLibs(gscript.LibString|gscript.LibLLM),
    gscript.WithLLMProvider(provider),
)
```

## Message and History Helpers

`msg.*` constructors create the plain message tables used by `llm.turn`,
`llm.react`, agent calls, and `messages {}` blocks:

| API | Result |
|---|---|
| `msg.system(text)` | `{role: "system", text: text}` |
| `msg.user(text)` | `{role: "user", text: text}` |
| `msg.assistant(text)` | `{role: "assistant", text: text}` |
| `msg.assistant_call(call)` | Assistant message carrying `tool_call`. |
| `msg.tool_result(call_id, value)` | Tool result message with `tool_use_id` and `value`. |
| `msg.tool_error(call_id, message)` | Tool result message with `tool_use_id` and `error`. |

`history` helpers operate on the history arrays returned by agents and
`llm.react`. They are useful when scripts should not rely on fixed indexes:

| API | Description |
|---|---|
| `history.find(h [, opts])` | Return the first matching message and its 1-based index, or `nil, -1`. |
| `history.find_all(h [, opts])` | Return a new array containing every matching message. |
| `history.last(h [, opts])` | Return the last matching message and its 1-based index, or `nil, -1`. |
| `history.append(h, message)` | Append `message` in place and return `h`. |

Matcher fields include `role`, `tool`, `tool_use_id`/`id`, `has_error`, and
plain message fields such as `text`. `tool` matches assistant tool-call
messages; tool result messages do not carry the tool name directly.

```gscript
h := messages {
    user: "Find docs."
    msg.assistant_call({id: "call_1", tool: "lookup", args: {query: "gscript"}})
    msg.tool_result("call_1", {summary: "docs"})
}

tool_msg, tool_idx := history.find(h, {role: "tool"})
assistant_msg, _ := history.last(h, {tool: "lookup"})
users := history.find_all(h, {role: "user"})
history.append(h, msg.user("Summarize."))
```

## Tools and Agent Tools

`llm.tool(name, fn [, opts])` creates a tool table from a script function.
`opts` can include `description`, `params`, `requires`, and `schema`.

```gscript
lookup := llm.tool("lookup", func(query) {
    return "docs:" .. query, nil
}, {
    description: "Look up documentation.",
    params: {"query"},
    requires: {"docs.read"},
})
```

Agents can be exposed as tools explicitly with `toolof`, `llm.toolof`, or
`llm.agent_as_tool`:

```gscript
agent extract_research(topic) {
    model: "fast"
    user: "Research " .. topic
    output: {
        summary: "short finding"
        confidence: 1
    }
}

delegate := llm.toolof(extract_research, {
    name: "delegate_research",
    description: "Delegate research to a specialist agent.",
    requires: {"none"},
})
```

The wrapper invokes the original agent. If `params` is omitted, the agent's
parameter names become the tool parameters. If `schema` is omitted and the
agent declares `output:`, that output shape becomes the tool schema. When the
agent returns a structured result containing `value`, the tool result is that
value; otherwise the wrapper returns the result text or raw result table.

For the common static case, an agent value can appear directly in another
agent's `tools:` list:

```gscript
agent supervisor(question) {
    model: "fast"
    tools: [extract_research]
    user: question
}
```

This is stdlib desugaring shorthand for `toolof(extract_research)` at runtime.
Use the explicit form when the call site needs a custom name, description,
capabilities, or schema.

## Output Validation

`llm.validate_output(value, schema)` checks a table or JSON string against the
example-table shape used by `agent output:`. It returns `true, ""` on success
or `false, message` on failure; malformed JSON is reported as a validation
failure instead of raising.

```gscript
ok, msg := llm.validate_output({summary: "docs"}, {summary: "example"})
bad, why := llm.validate_output({summary: 1}, {summary: "example"})
json_ok, _ := llm.validate_output('{"summary":"docs"}', {summary: "example"})
```

The validator is intentionally lightweight. It verifies the same structured
output shape that agent calls use after a provider turn, so tests can assert
agent output contracts without contacting a provider.

## Turn and Loop Surface

`llm.turn(opts)` sends one provider request. `llm.react(opts)` performs the
standard tool loop over `llm.turn`, `llm.dispatch`, and message history.
`loop.react`, `loop.simple`, `loop.plan_execute`, `loop.reflect`,
`loop.snapshot`, and `loop.resume` are convenience helpers built on the same
runtime tables. See [embedding.md](../embedding.md#native-llm-integration) for
the host provider, record/replay, trace, HITL, and real-provider smoke details.
