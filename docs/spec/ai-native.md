# AI-Native Syntax

Leia's AI-native surface is a standard-library runtime exposed through
tagged dialect forms and ordinary modules. The language implementation must
route these forms through the same `llm`, `msg`, `history`, `dialect`, and
host-provider paths as direct library calls. There is no separate AI execution
engine.

An AI operation is deterministic until it reaches a provider, host callback, or
tool body with side effects. Conformance tests that need stable behavior should
use host-side replay records or mock providers.

## Stable Contract

The stable AI-native contract is the following lowering boundary:

| Source construct | Stable runtime meaning |
|---|---|
| `model { ... }` | Register model aliases through `llm.register_models`. |
| `tool { ... }` | Build a tool descriptor through `llm.tool`. |
| `turn { ... }` | Perform exactly one provider request through `llm.turn`. |
| `agent { ... }` | Build a callable agent through `llm.agent`. |
| `evaluate "name" { ... }` | Declare a regression case discovered by `leia evaluate`. |
| `llm.*`, `msg.*`, `history.*` | Lower-level helpers for custom flows and message history. |

The dialect forms are not declarations with hidden lexical rules. Each form
evaluates to an ordinary Leia value or result pair according to its runtime
helper. Tools and agents are ordinary values that can be assigned, passed,
returned, inspected, and stored in tables.

The implementation must not fork behavior between dialect syntax and stdlib
calls. For example, a tool created with `tool { ... }` and a tool created with
`llm.tool(...)` must go through the same tool validation, capability metadata,
dispatch, tracing, and replay paths.

## Models

`model { ... }` registers script-visible aliases for the current VM.

```text
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

A model field value may be a string alias or a provider configuration table.
The alias named `default` is used when a request does not specify a model and
the host has not installed a more specific default. Alias cycles are invalid.

Source code must not embed API keys as string literals. Credentials belong in
environment variables, host-provided configuration, or a host-owned provider
factory. Hosts decide whether script-declared provider configuration is
honored.

## Messages And History

Provider requests carry an ordered array of normalized message tables. Leia
does not require a special message block syntax. Scripts may build message
arrays directly or through the `llm` and `msg` helper modules.

```text
history := {
    llm.system("Answer concisely."),
    llm.user("Summarize this incident."),
}

history[#history + 1] = msg.assistant("draft")
history[#history + 1] = msg.user("Now include the owner.")
```

Stable roles are `system`, `user`, `assistant`, and `tool`. Tool-call and
tool-result messages are paired by provider call id. A later turn observes
prior work only through the `messages` value supplied to that turn.

## Tools

`tool { ... }` returns a tool descriptor backed by a Leia function.

```text
search_runbook := tool {
    name: "search_runbook"
    params: {"service"}
    description: "Search local runbooks."
    requires: {"docs.read"}
    fn: func(service) {
        return "runbook:" .. service, nil
    }
}
```

The required fields are:

| Field | Meaning |
|---|---|
| `name` | Provider-visible tool name. |
| `fn` | Function called when the tool is dispatched. |

The optional fields are:

| Field | Meaning |
|---|---|
| `params` | Ordered parameter names. |
| `description` | Provider-visible description. |
| `requires` | Capability labels for host policy and audit. |

The tool function should return `(value, nil)` on success or `(nil, err)` on a
recoverable tool failure. Tool descriptors may be placed in `tools` lists,
passed to `llm.dispatch`, inspected with `llm.tool_caps`, or generated from
ordinary functions with lower-level helpers.

## Turns

`turn { ... }` performs exactly one provider request and returns
`(result, err)`. It does not dispatch tools by itself.

```text
result, err := turn {
    model: "fast"
    messages: {llm.user("Reply exactly: ok")}
    tools: {search_runbook}
    max_tokens: 32
    temperature: 0
}
```

Important request fields are:

| Field | Meaning |
|---|---|
| `model` | Alias or provider model name. |
| `messages` | Ordered message array. |
| `user` | Shorthand for a one-message user request when `messages` is absent. |
| `system` | System prompt used with `user` when `messages` is absent. |
| `tools` | Tool descriptors or supported agent-as-tool values. |
| `force_tool` | Force one tool by name when supported by the provider. |
| `max_tokens` | Output token limit. |
| `temperature`, `top_p` | Sampling controls. |
| `response_format` | Provider response-format hint. |
| `stream` | Request incremental provider output when supported. |
| `on_stream`, `onStream` | Optional callback for streaming events. |
| `stop` | Stop sequences. |
| `metadata` | String metadata passed to the provider. |

With streaming, callbacks and trace events may receive incremental token data.
The script still receives one complete final result table after the provider
finishes. A provider that does not support streaming may ignore `stream` or
return a provider error according to its implementation.

Result fields include `status`, `text`, `calls`, `reason`, and `usage`.
Provider or validation failures are ordinary error values, usually tables with
at least `kind` and `message`.

## Agents

`agent { ... }` returns a callable workflow value. The `config` function builds
the request configuration for each call.

```text
answer := agent {
    name: "answer"
    params: {"question"}
    description: "Answer with local documentation when useful."
    config: func(question) {
        return {
            model: "fast"
            system: "Use tool evidence when it helps."
            user: question
            tools: {search_runbook}
        }, nil
    }
}

result, err := answer("How do I restart search?")
```

The required fields are:

| Field | Meaning |
|---|---|
| `name` | Runtime and provider-visible agent name. |
| `config` or `fn` | Function that returns an agent request table and optional error. |

Optional fields include `params`, `description`, `output`, and `flow`. Without
a custom `flow` function, Leia executes the built-in loop:

1. call the config function for the current agent arguments;
2. synthesize `messages` from `system` and `user` when `messages` is absent;
3. perform a provider turn;
4. dispatch requested tools through `llm.dispatch`;
5. append assistant-call and tool-result or tool-error messages;
6. repeat until a final answer, provider stop, budget error, approval pause,
   cancellation, or unrecoverable provider/tool error.

When the `flow` field is present, its function owns the workflow. No hidden
turn or tool dispatch happens. Custom flows should call `llm.turn`,
`llm.dispatch`, `msg.assistant_call`, `msg.tool_result`, and `msg.tool_error`
explicitly.

For complex flows that need exact control over metadata or call parameters,
scripts may use the lower-level helper directly:

```text
incident := llm.agent("incident", incident_config, incident_flow, {
    params: {"service"}
})
```

## Agent As Tool

An agent can be exposed as a tool with `llm.agent_as_tool(agent_value)`.
The wrapper preserves the agent's name and metadata where available.

```text
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

## Output Validation

Agents and turns may request structured output through request fields such as
`output` or provider response-format hints. Built-in agent execution validates
known output shapes when configured to do so. Custom flows that return
arbitrary values are responsible for calling `llm.validate_output(value,
schema)` if they want the same check.

Validation failures are runtime errors or `(nil, err)` results according to
the helper being used. They must not be silently converted to provider text.

## Budgets, Cancellation, And Approval

Budgets may be attached to agent or turn option tables and may also be enforced
by the host. Stable dimensions include turn count, call count, token count, and
time. Provider usage may include cost metadata, but Leia does not promise money
accounting as a stable script-level budget dimension.

Hosts may cancel a running AI operation through the VM context or provider
context. Cancellation should stop future provider/tool calls and return a
recoverable error when possible.

Human approval and pause/resume state are runtime features of the `llm` helper
layer. Dialect syntax must preserve those hooks by lowering through the helper
layer instead of bypassing it.

## Trace, Record, And Replay

Hosts can install trace sinks, recorders, or replay providers. Trace events are
metadata-oriented and should avoid prompt text and tool result values unless an
explicit host policy permits capture.

Replay fixtures are strict. A request mismatch, exhausted fixture, or leftover
unconsumed turn is a failure. Updating a replay fixture is an explicit command
or host action, not an ordinary script side effect.

## Evaluate Blocks

`evaluate "case name" { ... }` declares a source-level regression case. The
case name is a required string literal. In ordinary script execution an
evaluate block has no runtime effect. The `leia evaluate` command owns
discovery and evaluation semantics.

During evaluation, the harness installs an `eval` module for dataset-style
regression tests:

| Function | Meaning |
|---|---|
| `eval.case(id, fn)` | Run one named subcase. Runtime errors inside `fn` fail that subcase but do not stop later subcases. |
| `eval.metric(name, value)` | Record a bool, number, string, nil, or JSON-like metric. |
| `eval.load_jsonl(path)` | Load a JSON Lines corpus relative to the evaluate source file. |
| `eval.skip_if(cond, reason)` | Mark the active subcase skipped when `cond` is truthy. |
| `eval.fail_if(cond, message)` | Raise a runtime error when `cond` is truthy. |
| `eval.usage()` | Return current case LLM usage. |
| `eval.budget(table)` | Raise when current usage exceeds a positive limit. |
| `eval.judge(options)` | Run a bounded judge turn by calling `llm.turn(options)`. |

```text
evaluate "answer corpus" {
    rows := eval.load_jsonl("answer_cases.jsonl")
    for _, row := range rows {
        ok, err := eval.case(row.id, func() {
            result, err := answer(row.question)
            assert(err == nil)
            eval.metric("correct", result.text == row.expected)
        })
        assert(ok || err != nil)
    }
}
```

Evaluation reports preserve raw metrics on each case and include top-level
metric summaries. Boolean metrics are summarized as pass rates. Numeric metrics
are summarized as count, mean, min, and max. String metrics are summarized as
category counts. Skipped subcases do not contribute to metric summaries.

## Capability Checks

AI dialects declare `llm.turn` capability usage. Tool descriptors may declare
additional `requires` labels. A host may reject scripts or tool exposure when
the requested capabilities exceed policy.

Capability failure is a host/runtime error. It must not be hidden as a provider
answer.

## Live Provider Tests

Live LLM tests must be opt-in and must not commit tokens. Integration tests
should read credentials from environment variables and skip when credentials
are absent.
