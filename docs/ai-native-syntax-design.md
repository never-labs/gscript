# GScript AI-Native Syntax Design

Status: accepted syntax proposal with a partial implementation in the current
workspace. The implemented parser surface lowers to the existing LLM standard
library; this document separates stable syntax decisions from current
implementation status.

GScript's AI layer is designed around one simple promise: a useful agent should
be writable with almost no ceremony, while advanced users can still drop to a
single-turn protocol primitive when they need exact control.

Current implementation snapshot:

- AI-native words are still lexed as ordinary identifiers. The parser recognizes
  them contextually in the syntax positions described below.
- Implemented parser/AST/desugar coverage includes list literals, `tool`
  declarations, named and anonymous `agent` values, optional `flow`, `agent
  defaults`, `models`, `messages`, `turn`, and ambient `budget` blocks.
- Lowering currently targets `llm.tool`, `llm.agent`, `llm.agent_defaults`,
  `llm.register_models`, and `llm.turn`. Non-flow agents lower to
  `llm.agent(name, config_fn)`. Flow agents lower to
  `llm.agent(name, config_fn, flow_fn)`, so model aliases/defaults, budget, and
  ambient `turn {}` inheritance are owned by the same stdlib path.
- `models { ... }` lowers to `llm.register_models({ ... })`. The stdlib stores
  aliases/configs and resolves `default`, aliases, and `provider_model` for
  `llm.turn` / `llm.agent`. When no host provider is injected, entries with
  `protocol`, `base_url`, `api_key`, and `provider_model` can construct the
  built-in OpenAI-compatible or Anthropic-compatible provider at runtime.
- Agent-local `budget: { ... }` tables are honored by the current `llm.agent`
  / `llm.react` loop. Standalone `budget { ... } { ... }` blocks lower to
  `llm.with_budget(config, func(){ ... })`; nested scopes intersect naturally,
  so every active frame must allow the next turn/tool call.
- Tool declarations lower with the tool name, function body, parameter list,
  Go-style doc comments, and `gscript:` directives for `description`,
  `requires`, and `param_docs`.

Current status is summarized in [Implementation Status](#24-implementation-status).

## 1. Core Shape

The primary user-facing form is `agent`.

```gscript
agent ask(q) {
    user: q
}

r, err := ask("What is GScript?")
print(r.text)
```

Default behavior:

- `model`: host default
- `system`: empty
- `tools`: empty list
- `max_steps`: `1` without tools, host/default loop limit with tools
- `budget`: host default
- `output`: plain text
- execution: automatic multi-turn agent loop

The simple rule is:

```text
agent config says what to run
user field is the current user input
runtime handles the default multi-turn loop
```

## 2. Keywords

AI-native words are contextual soft keywords. They are only special in their
syntax positions, so existing library and table code such as `llm.turn(...)`
and `{budget: ...}` keeps working.

```text
agent tool turn flow budget models
```

Not keywords:

```text
react memory rag provider output user system tools defaults
```

`react` remains a library strategy/helper, not a language keyword. The default
`agent` execution strategy is ReAct-style multi-turn tool use, but users do not
need to name it for the common case.

Implementation uses ordinary identifier tokens plus parser lookahead, matching
the soft-keyword contract above.

## 3. Lists

AI syntax uses ordinary list literals for ordered data:

```gscript
tools: [search_docs, read_url]
```

Lists lower to the existing array-table representation. Tool order is preserved
for stable provider schemas, tracing, and tests.

## 4. Models

Scripts can declare named models directly in GScript. This keeps the language
self-contained: users do not need a separate TOML/JSON configuration format to
use their LLM subscriptions.

```gscript
models {
    default: "fast"

    "fast": {
        provider: "glm"
        protocol: "anthropic"
        provider_model: "glm-5.1"
        base_url: "https://open.bigmodel.cn/api/anthropic"
        api_key: env("GLM_API_KEY")
        timeout: 45s
        retries: 2
    }

    "strong": {
        provider: "openai"
        protocol: "openai"
        provider_model: "gpt-5"
        base_url: "https://api.openai.com/v1"
        api_key: env("OPENAI_API_KEY")
    }
}
```

`model: "fast"` in an agent refers to the user-defined model name. The provider
service's actual model identifier lives in `provider_model`.

`models` entries can be aliases or provider configs:

```gscript
models {
    default: "glm-fast"
    coding:  "openai-strong"

    "glm-fast": {
        provider: "glm"
        protocol: "anthropic"
        provider_model: "glm-5.1"
        base_url: "https://open.bigmodel.cn/api/anthropic"
        api_key: env("GLM_API_KEY")
    }

    "openai-strong": {
        provider: "openai"
        protocol: "openai"
        provider_model: "gpt-5"
        base_url: "https://api.openai.com/v1"
        api_key: env("OPENAI_API_KEY")
    }
}
```

Lowering target:

```gscript
llm.register_models({
    default: "glm-fast",
    coding: "openai-strong",
    "glm-fast": {
        provider: "glm",
        protocol: "anthropic",
        provider_model: "glm-5.1",
        base_url: "https://open.bigmodel.cn/api/anthropic",
        api_key: env("GLM_API_KEY"),
    },
})
```

Implementation lowers this form to `llm.register_models({...})`; `llm.models`
is also available as a stdlib alias. Runtime resolution maps `default` and
named aliases to the provider-side model string, preferring `provider_model`
and then `model` inside config tables. If no host provider was installed with
`WithLLMProvider`, config tables with a `protocol` are passed to the host
provider factory. The default Go factory supports `"openai"` /
`"openai_compatible"` and `"anthropic"` / `"anthropic_compatible"`.

Production policy:

- Host-installed providers are the production default. `WithLLMProvider` takes
  precedence over script-declared provider fields, and host routers may still
  resolve script model aliases to provider-side model names.
- Script-declared provider configs are allowed when the host intentionally
  leaves `WithLLMProvider` unset or installs `WithLLMProviderFactory` to
  construct approved providers from `models {}`. This is useful for examples,
  tests, local tools, and deployments where provider selection is part of the
  script contract.
- The default factory only accepts the built-in wire protocols:
  `"openai"`, `"openai_compatible"`, `"openai_compat"`,
  `"chat_completions"`, `"anthropic"`, `"anthropic_compatible"`,
  `"anthropic_compat"`, and `"messages"`. Underscores and hyphens are treated
  equivalently by the Go factory.
- A provider config with `protocol` must include either `provider_model` or
  `model`. Alias-only entries and host-only configs without `protocol` remain
  valid and do not trigger provider construction.
- `api_key` must come from `env("NAME")`, a host secret reference, or another
  explicit expression that the deployment treats as a secret source. Literal
  API keys in source are rejected before lowering.

Field rules:

- `provider` is a human-readable provider label for trace/audit.
- `protocol` selects the wire format, such as `"openai"` or `"anthropic"`.
- `provider_model` is the provider-side model ID.
- `base_url` is the protocol endpoint/base URL.
- `default` is the model used when neither agent nor defaults specify `model`.
- Aliases can point to another named model. Alias cycles are load errors.
- Go embedding APIs may still inject or override named models for production
  hosts.

Non-goals:

- Provider-specific keywords such as `openai` or `anthropic`.
- Storing plaintext API keys in source.
- Requiring a separate config file format for core LLM setup.

## 5. Agent Defaults

Repeated agent configuration should be factored once without wrapping normal
code in a nested block. `agent defaults { ... }` is a module-scope declaration
that supplies defaults for every agent in that module.

```gscript
agent defaults {
    model: "strong"
    system: "Answer clearly and cite sources."
    tools: [search_docs]
    budget: {turns: 6, calls: 12, tokens: 8000, time: 60s}
}

agent answer_docs(q) {
    user: q
}

agent summarize(text) {
    system: "Summarize clearly."
    user: text
}
```

Rules:

- Explicit fields on an agent override defaults.
- Missing fields are inherited from `agent defaults`, then host defaults.
- `tools` defaults are inherited as a whole. An agent that sets `tools` replaces
  the default list.
- `budget` defaults are nested/intersected with agent-local budget.
- `system` defaults are replaced by explicit `system`. To extend a default
  system prompt, write `system: system + "\nExtra instruction."`.
- Defaults apply to named agents, anonymous agents, and agent IIFEs in the same
  module.
- There is at most one `agent defaults` declaration per module. Duplicate
  declarations are a load/lint error.
- `agent defaults` does not create providers or call models. Its fields are
  merged into each agent's config and evaluated under the same rules as that
  agent config.

Grammar:

```ebnf
AgentDefaultsDecl = "agent" "defaults" AgentConfig ;
```

## 6. Tool Declarations

Tools use Go-style documentation plus `gscript:` directives. There is no
decorator/attribute syntax.

```gscript
// search_docs searches indexed project documentation.
//gscript:requires docs.read
//gscript:param query natural-language search query
tool search_docs(query) {
    return docs.search(query, {limit: 5}), nil
}
```

Grammar:

```ebnf
ToolDecl = DocComment "tool" Ident "(" [ParamList] ")" Block ;
```

Rules:

- `gscript:requires` is required. Use `gscript:requires none` for pure tools.
- `gscript:param <name> <description>` is optional.
- The doc comment summary becomes the LLM-facing tool description.
- Tool bodies return `(value, err)`.
- Tool declarations bind a value with the declared name.
- Tools are normal closures and can capture surrounding state.

Implementation attaches only immediately-adjacent Go-style comments. The
generated `llm.tool` options include `params`, `description`, `requires`, and
`param_docs`. Validation rejects missing `gscript:requires`, malformed or
duplicate capability entries, `none` mixed with other capabilities, unknown
parameter docs, and duplicate parameter docs.

Lowering:

```gscript
search_docs := llm.tool("search_docs", func(query) {
    return docs.search(query, {limit: 5}), nil
}, {
    description: "search_docs searches indexed project documentation.",
    params: ["query"],
    param_docs: {
        query: "natural-language search query",
    },
    requires: ["docs.read"],
})
```

## 7. Agent Values

Agents are first-class callable values, like functions.

Named agent:

```gscript
agent answer(q) {
    system: "Answer briefly."
    user: q
}
```

Anonymous agent assigned to a variable:

```gscript
answer := agent(q) {
    system: "Answer briefly."
    user: q
}
```

Anonymous no-arg agent, immediately invoked:

```gscript
r, err := agent {
    user: "What is GScript?"
}()
```

Higher-order agent construction:

```gscript
func make_agent(style) {
    return agent(q) {
        system: "Answer in this style: " .. style
        user: q
    }
}

brief := make_agent("brief")
r, err := brief("Explain GScript")
```

Grammar:

```ebnf
AgentDecl = "agent" Ident [ParamList] AgentConfig [FlowBlock] ;
AgentExpr = "agent" [ParamList] AgentConfig [FlowBlock] ;
AgentConfig = "{" FieldBindings "}" ;
FlowBlock = "flow" Block ;
FieldBindings = { Ident ":" Expr [ "," | ";" ] } ;
```

`ParamList` follows function parameter syntax. `agent { ... }` is a no-argument
anonymous agent.

## 8. Agent Config Fields

Common fields:

```text
model        optional logical model name
system       optional system prompt string
user         current user input; string/table/list accepted
tools        ordered list of tool values
output       output example / shape hint
max_steps    max automatic loop steps
budget       local budget table
temperature  optional number
top_p        optional number
stream       optional stream callback
metadata     optional trace/provider metadata table
memory       optional stdlib memory strategy
approve_when optional HITL predicate for tool calls
```

Defaults:

```text
model      = host default
system     = ""
user       = nil unless provided by config/body evaluation
tools      = []
max_steps  = 1 when tools is empty, host/default tool-loop limit otherwise
budget     = host/default budget
output     = plain text
memory     = none
stream     = off
```

The only required field for a no-arg agent expression is usually `user`.

```gscript
agent { user: "Summarize this file: " .. text }()
```

For named agents, `user` usually references a parameter:

```gscript
agent summarize(text) {
    system: "Summarize clearly."
    user: text
}
```

## 9. Default Agent Execution

Calling an agent returns `(result, err)`.

```gscript
r, err := answer("What is GScript?")
```

Result shape:

```text
result.status   "done" | "pending" | "stopped"
result.text     final text answer
result.value    structured output when output/json is used
result.history  full updated conversation history
result.usage    token/cost/latency metadata when provider reports it
result.steps    number of model turns
```

Error shape:

```text
err.kind      "provider" | "network" | "timeout" | "cancelled" |
              "budget" | "validation" | "capability" |
              "tool" | "internal" | "stopped"
err.message   human-readable message
err.retryable optional bool
err.status    optional provider status code
err.tool      optional tool name
err.cause     optional nested error/table
```

Default loop semantics:

1. Build initial messages from `system`, optional existing history from runtime
   call options, and `user`.
2. Perform one `turn`.
3. If the model returns final text, return `result.status == "done"`.
4. If the model requests tools, dispatch them, append tool results to history,
   and continue until final text, pending approval, `max_steps`, budget, cancel,
   or provider/tool error.
5. If `approve_when(call)` returns true, return `result.status == "pending"`
   with a resumable token.

The default strategy should be good enough for common tool-using agents.

## 10. Output Examples

`output` is an output example or shape hint, not an input/few-shot example.

Text-shape hint:

```gscript
agent classify(text) {
    system: "Classify sentiment."
    user: text
    output: "positive"
}
```

Structured output hint:

```gscript
agent extract_contact(text) {
    system: "Extract contact information."
    user: text
    output: {
        name: "Ada Lovelace"
        email: "ada@example.com"
        company: "Analytical Engines"
    }
}

r, err := extract_contact(email_body)
print(r.value.email)
```

Rules:

- `output` guides the model toward the desired shape.
- Table output hints request structured output validation. The runtime checks
  that the final JSON object contains hinted fields and performs deep type
  checks from example values for string, number, bool, object, and array fields.
  For arrays, the first hinted element is used as the shape for returned items.
- On success, decoded structured data is placed in `result.value`.
- By default, validation failure returns `err.kind == "validation"` without an
  extra model call.
- `output_repair: true` enables one repair turn. `output_retries: N` enables up
  to `N` repair turns. The repair turn appends a prompt containing the
  validation error, output shape example, and previous response, then asks the
  model to return only matching JSON. Repair turns use the same provider path,
  response format, trace events, cancellation, and LLM budget counters as normal
  `llm.react` turns.

Current implementation note: direct `turn { response_format: ... }` is lowered
to `llm.turn` and forwarded to the provider request. Agent-level table
`output` now requests provider JSON mode with `response_format:
{type: "json_object"}` when no explicit `response_format` is present, and a
final JSON object/table response is decoded into `result.value`. Invalid JSON,
missing hinted fields, and deep field type mismatches return
`err.kind == "validation"` unless `output_repair` or `output_retries` is set.
Nested schema generation remains pending runtime work.

## 11. History And Call Options

The default syntax stays simple:

```gscript
r, err := chat("How do I embed it?")
```

History is optional and should be supplied by host/runtime call options rather
than forcing every call to use an envelope table. The exact call-option syntax is
an implementation detail for the parser/API phase, but the semantic model is:

```text
input is this call's user input
history is previous conversation state
runtime builds: system + history + input
result.history is the updated history to save
```

If the language later gains named call options, the intended surface is:

```gscript
r, err := chat("How do I embed it?", history: session.history)
session.history = r.history
```

Until then, Go embedding APIs can pass history through host call options and the
stdlib may accept explicit option tables.

## 12. Custom Multi-Turn Logic With `flow`

Most agents should not need custom loop logic. When they do, an optional `flow`
block takes over execution.

```gscript
agent support(message) {
    system: "Support agent."
    tools: [refund, lookup_order]
    user: message
    max_steps: 8
} flow {
    history := messages {
        system: system
        user: message
    }

    for i := 0; i < max_steps; i++ {
        r, err := turn {
            messages: history
            tools: tools
        }
        if err != nil { return nil, err }

        if r.status == "final_answer" {
            return {status: "done", text: r.text, history: history}, nil
        }

        for _, c := range r.calls {
            if c.tool == "refund" && c.args.amount > 100 {
                return snapshot(history, c), nil
            }

            v, e := dispatch(c)
            history = append(history, assistant_call(c))
            if e != nil {
                history = append(history, tool_error(c.id, e.message))
            } else {
                history = append(history, tool_result(c.id, v))
            }
        }
    }

    return nil, {kind: "stopped", message: "max steps"}
}
```

Rules:

- If no `flow` block is present, the default automatic multi-turn loop runs.
- If `flow` is present, it is responsible for calling `turn`, dispatching tools,
  and returning `(result, err)`.
- Agent config fields are available in `flow` as local variables.
- `flow` does not change provider binding or capability policy.

## 13. Turn

`turn` is the low-level primitive for one model call. It is for advanced users
and for `flow` blocks.

```gscript
r, err := turn {
    model: "strong"
    messages: messages {
        system: "Be concise."
        user: question
    }
    tools: [search_docs]
}
```

Semantics:

- Calls the selected provider once.
- Does not dispatch tools.
- Does not loop.
- Does not manage memory or history beyond the provided messages.
- Returns `(result, err)`.

Turn result shape:

```text
result.status = "final_answer" | "tool_calls" | "stop"
result.text
result.calls
result.reason
result.usage
```

Fields:

```text
model          optional logical model name
messages       required ordered message list
tools          optional ordered tool list
force_tool     optional tool value | "any" | "none"
max            optional {tokens, time}
temperature    optional number
top_p          optional number
stop           optional list of strings
metadata       optional table of strings
stream         optional bool or callback
output         optional output hint, same meaning as agent output
```

Lowering target:

```gscript
r, err := llm.turn({...})
```

## 14. Messages

`messages` is a lightweight constructor for ordered message lists.

```gscript
history := messages {
    system: "You are concise."
    user: "Hello"
    assistant: "Hi"
    user: "Summarize this."
}
```

It lowers to:

```gscript
[
    {role: "system", text: "You are concise."},
    {role: "user", text: "Hello"},
    {role: "assistant", text: "Hi"},
    {role: "user", text: "Summarize this."},
]
```

Advanced entries may still be raw tables:

```gscript
history = append(history, [
    {role: "assistant", tool_call: c},
    {role: "tool", tool_use_id: c.id, value: v},
])
```

`messages` exists to avoid forcing normal users to write `{role, text}` tables
for common prompts. It is a data constructor, not a provider call.

## 15. Budget

Budget is optional and does not include money in the first language surface.

```gscript
budget {
    turns: 8
    calls: 16
    tokens: 16000
    time: 60s
} {
    return research(question)
}
```

Agent-local budget:

```gscript
agent research(q) {
    tools: [search_docs]
    user: q
    budget: {turns: 8, calls: 16, tokens: 16000, time: 60s}
}
```

Rules:

- Budget blocks are statements.
- Budgets are ambient and nest by intersection.
- Counters charge all active frames.
- Exhaustion is observed at the next `turn` or tool dispatch.
- Money/cost accounting remains host/provider policy for now.

Implementation enforces agent-local budget tables in the `llm.agent` /
`llm.react` loop for turns, tool calls, tokens, money, and time when those
limits are present in the options table. Standalone `budget` blocks lower to
ambient `llm.with_budget` scopes. Nested scopes intersect naturally, and active
frames are charged for turns, tool calls, token usage, and deadlines around
enclosed `turn` / tool dispatch operations.

## 16. Capabilities

Capabilities are strings:

```gscript
//gscript:requires docs.read net.http
```

Built-in capability namespaces:

```text
fs.read       fs.write
net.http      net.listen
db.read       db.write
env.read      env.write
process.run   process.exec
time.now      rand.read
llm.call      llm.stream
email.send    payment.charge payment.refund
host.call
all           none
```

Rules:

- Static checks are developer feedback.
- Runtime capability checks remain authoritative.
- Unknown capabilities are warnings by default and may be errors in strict mode.
- `all` requires explicit lint suppression.

## 17. Memory And Retrieval

Memory and retrieval are stdlib namespaces, not syntax.

```gscript
agent support(q) {
    memory: memory.window({tokens: 4000})
    user: q
}
```

```gscript
// search_docs searches project documentation.
//gscript:requires docs.read
tool search_docs(query) {
    return rag.search("project-docs", query, {limit: 5}), nil
}
```

Required stdlib areas:

```text
memory.none()
memory.window({tokens})
memory.summary({model, tokens})
memory.store(store, opts)

embed.provider(...)
vector.index(...)
rag.search(index, query, opts)
rag.cite(results)
```

## 18. Agent As Tool

Agents can be exposed as tools explicitly:

```gscript
agent summarize(text) {
    system: "Summarize in three bullets."
    user: text
}

summary_tool := toolof(summarize, {
    name: "summarize",
    description: "Summarize text.",
    requires: ["none"],
})
```

An agent value can also be placed directly in an agent `tools:` list for the
common static case:

```gscript
agent supervisor(q) {
    tools: [summarize]
    user: q
}
```

The runtime treats this as `toolof(summarize)`. Explicit conversion remains the
recommended form when name, description, capability, or schema choices need to
be visible at the call site.

## 19. Static Analysis

The compiler/linter should build an AI metadata index:

```text
tools:
  name, params, doc, requires, source span
agents:
  name, params, model, tools, output, budget, source span
turns:
  model, tools, output, stream, source span
```

The current lint index records declared tools and exposes their source
metadata, including `requires`, for follow-on checks. When an `agent`,
`agent defaults`, or `turn` has a statically named literal `tools` list, the
validator aggregates the referenced tool requirements. If the same config also
has a static `capabilities` or `caps` string list/table, validation checks that
the declared capabilities cover those requirements. Dynamic tool expressions
and dynamic capability expressions are intentionally left to runtime checks.
For concrete agents, validation also conservatively applies module-level
`agent defaults` using the same shallow field override rule as runtime:
defaults are copied first and the agent config overwrites same-named fields.
When the defaults and agent `tools` / `capabilities` / `caps` fields are all
static where present, the merged effective config is checked for required
capability coverage. If either side uses a dynamic expression for those fields,
the cross-defaults check is skipped and runtime remains authoritative.

Checks:

- `tool` missing `gscript:requires`.
- Malformed or duplicate `gscript:requires` entries, including `none` mixed
  with other capabilities.
- Unknown or duplicate `gscript:param` docs.
- Duplicate statically named entries in literal `agent.tools` / `turn.tools`
  lists.
- `agent.tools` references unknown/non-tool values when statically known.
- Statically declared `agent` / `turn` `capabilities` or `caps` do not cover
  the aggregated requirements of statically named tools.
- Statically merged `agent defaults` plus concrete agent configuration does not
  cover the aggregated requirements of the effective statically named tools.
- Host capability policy does not include tool requirements.
- `turn` has no messages.
- `output` table is malformed.
- `stream` callback is not callable when statically known.
- Duplicate model aliases.

Static analysis never replaces runtime checks.

## 20. Desugaring Order

```text
lex/parse
  -> attach doc directives
  -> build AI metadata index
  -> AI lint/capability checks
  -> desugar AI syntax to core AST/std-lib calls
  -> normal compile/interpreter/VM/JIT pipeline
```

This keeps formatter/linter aware of source syntax while preserving the existing
runtime as the first lowering target.

## Tooling Status

The current CLI tooling understands AI-native syntax through the same
parser-backed path used by normal `.gs` files:

- `gscript fmt` accepts `tool`, `agent`, `turn`, `messages`, `models`, and
  `budget` syntax before rewriting bytes. It is still a narrow whitespace
  normalizer: CRLF/CR becomes LF, trailing spaces/tabs are trimmed, trailing
  blank lines are collapsed, brace-driven line indentation is normalized, and a
  final newline is ensured. It does not yet pretty-print expressions or rebuild
  source from AST nodes.
- `gscript lint` reports lexer/parser failures, including AI-native syntax
  errors, as `GS1001` diagnostics in text, JSON, or SARIF output. The broader
  AI metadata/capability lint index is still pending.
- `examples/ai_native_agent.gs` is a parser/tooling example that covers the
  AI-native surface without requiring runtime changes.

## 21. Canonical Desugaring

Implementations may use direct AST nodes internally, but observable behavior
must match these shapes.

Tool:

```gscript
// search_docs searches docs.
//gscript:requires docs.read
tool search_docs(query) {
    return docs.search(query), nil
}
```

as if:

```gscript
search_docs := llm.tool("search_docs", func(query) {
    return docs.search(query), nil
}, {
    description: "search_docs searches docs.",
    params: ["query"],
    requires: ["docs.read"],
})
```

Simple agent:

```gscript
agent answer(q) {
    system: "Answer briefly."
    user: q
}
```

as if:

```gscript
answer := llm.agent("answer", func(q) {
    return {
        system: "Answer briefly.",
        user: q,
    }
})
```

Anonymous IIFE:

```gscript
r, err := agent { user: "hello" }()
```

as if:

```gscript
r, err := llm.agent("", func() {
    return {user: "hello"}
})()
```

Flow agent:

```gscript
agent support(q) {
    tools: [refund]
    user: q
} flow {
    return turn { messages: messages { user: q }, tools: tools }
}
```

as if:

```gscript
support := llm.agent("support", func(q) {
    return {
        tools: [refund],
        user: q,
    }
}, func(q) {
    return llm.turn({
        messages: messages { user: q },
        tools: tools,
    })
})
```

Flow-agent lowering uses this canonical `llm.agent` shape. The flow body
receives config fields such as `tools`, `model`, and `system` as local bindings
for compatibility, and `llm.agent` also pushes the merged agent config as
ambient context while the flow runs. This lets simple `turn {}` calls inherit
the agent's model, tools, system, user, and budget unless the turn overrides
them explicitly.

The compiler can avoid allocating closure objects when a cleaner internal
representation is available, but record/replay, trace events, budgets, and
errors must remain equivalent.

Agent defaults:

```gscript
agent defaults {
    model: "strong"
    tools: [search_docs]
}

agent answer(q) {
    user: q
}
```

as if each agent in the module captured merged config:

```gscript
answer := llm.agent("answer", func(q) {
    return {
        model: "strong",
        tools: [search_docs],
        user: q,
    }
})
```

## 22. Examples

### 22.1 One-Off Question

```gscript
r, err := agent {
    user: "What is GScript?"
}()
print(r.text)
```

### 22.2 Named Assistant

```gscript
agent answer(q) {
    system: "Answer in one short paragraph."
    user: q
}
```

### 22.3 Tool-Using Agent

```gscript
// search_docs searches project documentation.
//gscript:requires docs.read
tool search_docs(query) {
    return docs.search(query, {limit: 5}), nil
}

agent answer_docs(q) {
    system: "Use docs when useful. Cite sources."
    tools: [search_docs]
    user: q
    max_steps: 6
}
```

### 22.4 Structured Extraction

```gscript
agent extract_contact(text) {
    system: "Extract contact information."
    user: text
    output: {
        name: "Ada Lovelace"
        email: "ada@example.com"
    }
}
```

### 22.5 Module Defaults

```gscript
agent defaults {
    model: "strong"
    system: "Use docs when useful. Cite sources."
    tools: [search_docs]
    max_steps: 6
}

agent answer_docs(q) {
    user: q
}

agent explain_code(code) {
    system: system + "\nFocus on correctness risks."
    user: code
}
```

### 22.6 Custom Flow

```gscript
agent manual(q) {
    tools: [search_docs]
    user: q
} flow {
    history := messages { user: q }
    r, err := turn {
        messages: history
        tools: tools
    }
    if err != nil { return nil, err }
    return r, nil
}
```

## 23. Accepted Decisions

1. `agent` is the main user-facing AI construct and defaults to automatic
   multi-turn execution.
2. `agent` is a first-class callable value: named, anonymous, assignable,
   passable, returnable, and IIFE-callable.
3. `react` is not a keyword. It remains a stdlib/default strategy concept.
4. Tool metadata uses Go-style doc comments plus `gscript:` directives, not
   decorators.
5. `output` is an output example/shape hint, not an input example.
6. `flow` is the advanced escape hatch for custom multi-turn logic.
7. `turn` is the single-call low-level primitive.
8. Model/provider binding can be declared in GScript `models {}` or injected by
   Go host code; plaintext API keys in source are forbidden.
9. Budget does not include money in the first language surface.
10. `agent defaults { ... }` provides module-scope default agent configuration
    without nesting ordinary code.

## 24. Implementation Status

Current status in this workspace:

Done:

- Parser accepts list literals (`[...]`) and AI-native list syntax lowers to
  array-table literals.
- Parser preserves immediately-adjacent tool doc comments and extracts
  `gscript:` directives for tool metadata.
- Parser accepts `tool` declarations and lowers to `llm.tool` with parameters,
  description, requires, and param-doc options.
- Parser accepts named and anonymous `agent` values, including IIFE use.
  Non-flow agents lower to `llm.agent(name, config_fn)`.
- Parser accepts optional `flow` blocks. Flow agents lower to
  `llm.agent(name, config_fn, flow_fn)`.
- Parser accepts module-scope `agent defaults` declarations, validates
  duplicates, and lowers to `llm.agent_defaults`.
- Parser accepts `messages` constructors and lowers recognized roles to message
  constructor calls.
- Parser accepts `turn` blocks and lowers to `llm.turn`; the stdlib also
  supports the `user` shorthand by normalizing it into messages.
- Parser accepts `budget` blocks and lowers them to ambient
  `llm.with_budget` scopes. Nested/intersecting budgets charge every active
  frame for turns, token usage, tool calls, and deadlines.
- Runtime `llm.agent` owns merged agent defaults, model alias/default
  resolution, agent-local budget, and ambient context for enclosed `turn {}`
  calls.

Partial:

- `models` declarations are implemented for module-scope parsing, literal
  `api_key` rejection, alias-cycle rejection, and lowering to
  `llm.register_models`. Runtime resolves `default`, aliases, and
  `provider_model`, and can construct OpenAI-compatible or
  Anthropic-compatible providers from protocol/base-url/api-key fields when no
  host provider is already installed. Broader provider policy and lint
  integration remain host/tooling work.
- Validation rejects duplicate `agent defaults`, non-module `agent defaults` /
  `models`, model alias cycles, literal model `api_key` strings,
  missing/malformed/duplicate tool `gscript:requires`, unknown or duplicate
  tool `gscript:param` docs, duplicate statically named tools in literal
  agent/defaults tool lists, and static capability coverage failures after
  applying module-level `agent defaults` to concrete agents.
- `gscript fmt` and `gscript lint` accept AI-native source through the
  parser-backed tooling path. The formatter currently normalizes whitespace and
  brace-driven indentation; a full AST pretty printer for AI-native blocks and
  doc directives remains pending.
- Tests cover parser acceptance, stdlib-vs-syntax execution for interpreter and
  bytecode, anonymous agents/IIFE, defaults, model alias resolution, direct turn
  sugar, direct turn `response_format` forwarding, agent-level table `output`
  JSON-mode requests and `result.value` decoding, validation errors, flow
  behavior, directive lowering, ambient budget behavior, and AI-native
  agent/turn record-replay through the existing LLM stdlib recorder/replay API.
  A gated real-provider smoke also covers AI-native `models {}` provider
  construction and `turn {}` execution when `GSCRIPT_LLM_INTEGRATION=1` and the
  Anthropic-compatible endpoint/model/key environment variables are present.

Pending:

- Broader AI metadata/capability lint index described in
  [Static Analysis](#19-static-analysis).
- AST-level pretty printing for AI-native syntax.
- Broader production policy checks for provider/model construction beyond the
  current stdlib factory behavior.
- Complete agent-level structured output validation coverage.
