# GScript AI-Native Syntax Design

Status: design freeze candidate. No parser/runtime work should start from this
document until the syntax and lowering rules have been reviewed.

This document defines the language-level AI surface for GScript. The goal is to
make AI workflows feel native while preserving the existing runtime investment:
new syntax lowers to the current `llm.*` / `loop.*` standard library wherever
possible.

## 1. Design Goals

GScript should be usable in two roles:

- A Go-embedded scripting language with a small, explicit host boundary.
- A standalone AI-native language where agents, tools, budgets, model routing,
  and replayable LLM calls are first-class source concepts.

The AI syntax must:

- Keep GScript dynamically typed. No static type annotations are introduced.
- Preserve normal function/table/closure semantics.
- Make all LLM network I/O pass through one primitive: `turn`.
- Make tool capabilities visible to tooling before execution.
- Keep secrets, HTTP clients, and provider policy in the Go host by default.
- Lower to existing stdlib APIs so the first implementation is incremental.

## 2. Reserved Words And Attributes

New reserved words:

```text
tool agent turn react budget models
```

`models` declares logical model aliases only. It does not construct providers or
read secrets.

New attribute syntax:

```gscript
@requires("net.http", "fs.read")
@desc("Search project docs.")
@params({query: "search query"})
```

Attributes attach to the immediately following `tool` or `agent` declaration.
They are semantic, unlike comments. Comments remain documentation only.

Rationale: previous `//gs:` directives were easy to implement but too fragile
for a production language. Attributes are explicit syntax, format-friendly, and
easy for lint/IDE tools to inspect.

Additional AI-layer literals:

```text
500ms  60s  2m  1h
```

Duration literals are available only in AI metadata/budget positions in v1.
They lower to the existing duration handling used by runtime budget options.
Money literals such as `$0.25` are deferred; use `money: 0.25`.

## 3. Capability Model

Capabilities are strings, not bitwise syntax in user code:

```gscript
@requires("fs.read", "net.http")
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

- `none` means no host-sensitive capability.
- `all` is legal only with an explicit linter warning suppression.
- Unknown capability names are warnings by default and can be errors under
  strict mode, allowing host applications to define private capabilities.
- Tool capabilities are checked at load/lint time when all referenced tools are
  statically visible.
- Runtime checks remain authoritative. Static checks are a developer feedback
  layer, not a security boundary.

## 4. Provider And Model Binding

Scripts use logical model names:

```gscript
agent summarize(text) {
    model: "fast"
} {
    return react { messages: {user(text)} }
}
```

The Go host binds `"fast"` to a provider, endpoint, key, timeout, retry policy,
cost model, and audit policy.

Optional script-level aliases are allowed, but they do not create providers:

```gscript
models {
    default: "fast"
    strong:  "reasoning"
    cheap:   "small"
}
```

Lowering target:

```gscript
llm.register_model_alias({
    default: "fast",
    strong: "reasoning",
    cheap: "small",
})
```

Non-goal for v1:

- Script syntax that directly reads API keys.
- Script syntax that constructs HTTP provider clients.
- Provider-specific keywords such as `openai` or `anthropic`.

Reasoning: secrets and transport policy belong to the embedding host. This also
keeps GLM/OpenAI/Anthropic/local gateways from leaking into the language syntax.

## 5. Tool Declaration

Syntax:

```gscript
@requires("docs.read")
@desc("Search indexed documentation.")
@params({
    query: "natural-language search query",
    limit: "maximum result count",
})
tool search_docs(query, limit) {
    return docs.search(query, limit), nil
}
```

Grammar:

```ebnf
ToolDecl = AttrList "tool" Ident "(" [ParamList] ")" Block ;
AttrList = { Attribute } ;
Attribute = "@" Ident "(" [ExprList] ")" ;
```

Rules:

- `@requires(...)` is required. Use `@requires("none")` for pure tools.
- `@desc(...)` is optional. If absent, formatter/linter may derive a summary
  from the leading doc comment.
- `@params({...})` is optional. Parameter names still come from the declaration.
- Tool bodies return `(value, err)`.
- Tool declarations bind a value with the same name in the surrounding scope.
- Tools can close over state exactly like normal functions.

Lowering:

```gscript
search_docs := llm.tool("search_docs", func(query, limit) {
    return docs.search(query, limit), nil
}, {
    description: "Search indexed documentation.",
    params: {"query", "limit"},
    param_docs: {
        query: "natural-language search query",
        limit: "maximum result count",
    },
    requires: {"docs.read"},
})
```

## 6. Turn Block

`turn` is the only language primitive that may cause an LLM provider call.

Syntax:

```gscript
result, err := turn {
    model: "fast"
    messages: {system("Be concise."), user(question)}
    tools: {search_docs}
    max: {tokens: 256}
}
```

Named turn templates:

```gscript
turn classify(text) {
    model: "cheap"
    messages: {
        system("Classify as positive, negative, or neutral."),
        user(text),
    }
    max: {tokens: 16}
}

result, err := classify("GScript is useful")
```

Grammar:

```ebnf
TurnExpr = "turn" "{" FieldBindings "}" ;
TurnDecl = "turn" Ident "(" [ParamList] ")" "{" FieldBindings "}" ;
```

Fields:

```text
model          optional string; defaults from nearest agent, then host default
messages       required table/list of messages
tools          optional table/list; defaults from nearest agent
force_tool     optional tool value | "any" | "none"
max            optional table: {tokens, time}
temperature    optional number
top_p          optional number
stop           optional list of strings
metadata       optional table of strings
stream         optional bool or stream sink
json           optional schema/table, see structured output
```

Return shape:

```text
(result, err)

result.status = "final_answer" | "tool_calls" | "stop"
result.text
result.calls
result.reason
result.usage = {input_tokens, output_tokens, cost, latency}

err.kind = "budget" | "deadline" | "cancelled" | "provider" | "network"
```

Lowering:

```gscript
result, err := llm.turn({
    model: "fast",
    messages: {llm.system("Be concise."), llm.user(question)},
    tools: {search_docs},
    max_tokens: 256,
})
```

If `model` or `tools` are absent, lowering leaves them absent and the runtime
fills them from the ambient agent frame.

## 7. Structured Output

Structured output is part of `turn`, not a separate keyword.

```gscript
result, err := turn {
    messages: {user("Extract name and age from: Ada, 37")}
    json: {
        name: "string",
        age:  "number",
    }
}

person := result.value
```

Rules:

- `json:` requests valid JSON and validates the response.
- On success, `result.value` contains the decoded table.
- On model formatting failure, the runtime may perform bounded repair retries
  according to host policy.
- If repair fails, return `err.kind == "validation"`.
- Schema syntax is intentionally lightweight. It maps to provider-native JSON
  schema when supported and to prompt+validator fallback otherwise.

Supported schema atoms:

```text
"string" "number" "int" "bool" "any"
{"array": <schema>}
{"object": {field: schema, ...}}
{"enum": {"a", "b", "c"}}
```

## 8. Streaming

Streaming is expressed through `stream:` on `turn`.

```gscript
result, err := turn {
    messages: {user("Write a short haiku.")}
    stream: func(event) {
        if event.type == "text" {
            print(event.text)
        }
    }
}
```

Stream event shape:

```text
{type: "start"}
{type: "text", text}
{type: "tool_call_delta", id, tool, args_delta}
{type: "usage", usage}
{type: "end", status}
{type: "error", err}
```

Rules:

- Streaming is optional. Providers without streaming may buffer and emit one
  text event.
- The final return value is still `(result, err)`.
- Record/replay stores enough event data to replay stream callbacks
  deterministically when enabled.

## 9. Agent Declaration

Agents are callable values with metadata and a body.

```gscript
agent research(question) {
    model: "strong"
    tools: {search_docs, read_url}
    system: "You are a careful research assistant."
    budget: {turns: 8, tokens: 16000, time: 60s}
} {
    return react {
        messages: {user(question)}
    }
}
```

Grammar:

```ebnf
AgentDecl = AttrList "agent" Ident "(" [ParamList] ")" AgentMeta Block ;
AgentExpr = AttrList "agent" "(" [ParamList] ")" AgentMeta Block ;
AgentMeta = "{" FieldBindings "}" ;
```

Metadata fields:

```text
model       optional logical model name
tools       optional table/list of tool values
system      optional string
budget      optional table, same fields as budget block
memory      optional memory strategy
caps        optional list of capabilities; default is union(tools.requires)
policy      optional table, host/lint interpreted
```

Rules:

- Calling an agent pushes an ambient agent frame.
- `turn` and `react` inside the body inherit model/tools/system/budget from the
  nearest agent frame.
- Agent body returns ordinary `(value, err)`.
- `system`, `model`, `tools`, and `memory` are also visible as local variables
  in the body.
- Agents can be passed around as normal values.

Lowering outline:

```gscript
research := llm.agent("research", {
    model: "strong",
    tools: {search_docs, read_url},
    system: "You are a careful research assistant.",
    budget: {turns: 8, tokens: 16000, time: 60s},
}, func(question) {
    return loop.react({
        messages: {llm.user(question)},
    })
})
```

The exact lowering can be compiler-internal; `llm.agent` need not be a public
API if a direct ambient-frame bytecode operation is cleaner later.

## 10. React Block

`react` is syntax sugar for the common ReAct loop. It is not an LLM-call
primitive; it eventually calls `turn`.

```gscript
answer, err := react {
    messages: {user(question)}
    max_steps: 6
    approve_when: func(call) {
        return call.tool == "refund" && call.args.amount > 100
    }
}
```

Fields are the existing `llm.react` / `loop.react` options:

```text
messages tools model max_steps max_history_tokens max_tool_retries
force_tool budget metadata approve_when store
```

Lowering:

```gscript
answer, err := llm.react({...})
```

Rationale: a first-class `react` block is more ergonomic than requiring every
user to write the manual turn/dispatch loop, while preserving `turn` as the only
provider boundary.

## 11. Budget Block

Syntax:

```gscript
budget {
    turns: 8
    tokens: 20000
    time: 60s
    money: 0.25
} {
    return research(question)
}
```

Rules:

- Budget blocks are statements.
- Budgets are ambient and nest by intersection.
- Counters charge all active frames.
- Exhaustion is observed at the next `turn` or tool dispatch.

Lowering:

```gscript
llm.with_budget({turns: 8, tokens: 20000, time: 60s, money: 0.25}, func() {
    return research(question)
})
```

Money literal `$0.25` is deferred. v1 uses normal numeric money fields to avoid
adding another lexical edge before the rest of the syntax is proven.

## 12. Agent As Tool

Agents can be exposed as tools explicitly:

```gscript
@requires("none")
agent summarize(text) {
    model: "fast"
    system: "Summarize in three bullets."
} {
    return react { messages: {user(text)} }
}

summary_tool := toolof(summarize, {
    name: "summarize",
    description: "Summarize text.",
})
```

`toolof` is a stdlib helper, not a keyword.

Reasoning: implicit agent-as-tool conversion is convenient but too magical. An
explicit helper makes capability and naming choices visible.

## 13. Memory

Memory is configured as agent metadata or passed to `react`.

```gscript
agent support(message) {
    model: "fast"
    memory: memory.window({tokens: 4000})
} {
    return react { messages: {user(message)} }
}
```

Required memory strategy interface:

```text
load(session)              -> history, err
save(session, history)     -> nil, err
compact(history, budget)   -> history, err
```

Built-in memory helpers are stdlib:

```text
memory.none()
memory.window({tokens})
memory.summary({model, tokens})
memory.store(store, opts)
```

Non-goal for syntax:

- Dedicated `memory {}` block.
- Hidden persistent state unless an agent explicitly receives a session/store
  through metadata or call options.

## 14. Retrieval

RAG is stdlib, not syntax.

```gscript
@requires("docs.read")
tool search_docs(query) {
    return rag.search("project-docs", query, {limit: 5}), nil
}
```

Required stdlib areas:

```text
embed.provider(...)
vector.index(...)
rag.search(index, query, opts)
rag.cite(results)
```

Reasoning: retrieval backends differ too much to deserve syntax. The AI-native
part is that tools and agents make retrieval capability-visible and easy to
compose.

## 15. Error Model

Errors remain plain tables.

Common shape:

```text
{
    kind:    "provider" | "network" | "validation" | "policy" |
             "budget" | "deadline" | "cancelled" | "capability" |
             "user" | "internal" | "stopped",
    message: string,
    status:  optional number,
    cause:   optional value,
}
```

Tool error handling categories:

```text
transient:   network provider internal
recoverable: validation policy user capability
fatal:       everything else
```

The categories are defaults for `react`. Manual loops can choose different
policy.

## 16. Message Constructors

AI syntax imports short message constructors into agent bodies:

```gscript
system("...")
user("...")
assistant("...")
tool_result(id, value)
tool_error(id, message)
assistant_call(call)
```

Outside agent bodies, the canonical stdlib names remain available:

```gscript
llm.system(...)
llm.user(...)
```

Lowering inside agent body rewrites short names to `llm.*` only if no local
binding shadows them.

## 17. Static Analysis

The compiler/linter should build an AI metadata index:

```text
tools:
  name, params, desc, requires, source span
agents:
  name, model, tools, caps, budget, source span
turns:
  model, tools, json schema, stream, source span
```

Checks:

- `tool` missing `@requires`.
- `agent.tools` references unknown/non-tool value when statically known.
- `agent.caps` does not include tool requirements.
- host capability policy does not include agent/tool requirements.
- `turn` has neither explicit model nor ambient/default model.
- `turn` has neither explicit messages nor messages inherited by a helper.
- `json:` schema is malformed.
- `stream:` callback is not callable when statically known.
- duplicate model aliases.

Static analysis never replaces runtime checks.

## 18. Desugaring Order

Source pipeline:

```text
lex/parse
  -> attach attributes
  -> build AI metadata index
  -> AI lint/capability checks
  -> desugar AI syntax to core AST/std-lib calls
  -> normal compile/interpreter/VM/JIT pipeline
```

This order keeps formatter/linter aware of source syntax while keeping runtime
execution mostly on existing mechanisms.

## 19. Compatibility With Existing Stdlib API

The current stdlib remains public:

```gscript
llm.tool(...)
llm.turn(...)
llm.react(...)
loop.react(...)
```

The new syntax is preferred for authored source, but stdlib calls remain useful
for dynamic construction and embedding tests.

The following equivalences must hold:

```gscript
tool t(x) { return x, nil }
```

is equivalent to a generated `llm.tool`.

```gscript
turn { messages: {user("hi")} }
```

is equivalent to `llm.turn({...})` after ambient defaults are applied.

```gscript
react { messages: {user("hi")} }
```

is equivalent to `llm.react({...})` after ambient defaults are applied.

## 20. Examples

### 20.1 Single Turn

```gscript
agent classify(text) {
    model: "cheap"
    system: "Return one of: positive, negative, neutral."
} {
    r, err := turn {
        messages: {system(system), user(text)}
        max: {tokens: 8}
    }
    if err != nil { return nil, err }
    return r.text, nil
}
```

### 20.2 Tool-Using Agent

```gscript
@requires("docs.read")
@desc("Search project documentation.")
tool search_docs(query) {
    return docs.search(query, {limit: 5}), nil
}

agent answer(question) {
    model: "strong"
    tools: {search_docs}
    system: "Answer using tools when needed. Cite the source title."
    budget: {turns: 6, tokens: 8000}
} {
    return react {
        messages: {system(system), user(question)}
        max_tool_retries: 1
    }
}
```

### 20.3 Structured Extraction

```gscript
agent extract_contact(text) {
    model: "fast"
} {
    r, err := turn {
        messages: {user(text)}
        json: {
            name: "string",
            email: "string",
            tags: {"array": "string"},
        }
    }
    if err != nil { return nil, err }
    return r.value, nil
}
```

### 20.4 Streaming

```gscript
agent draft(topic) {
    model: "fast"
} {
    return turn {
        messages: {user("Draft a paragraph about " .. topic)}
        stream: func(e) {
            if e.type == "text" { print(e.text) }
        }
    }
}
```

### 20.5 Manual Turn Loop With HITL

```gscript
@requires("payment.refund")
tool refund(order_id, amount) {
    return payments.refund(order_id, amount), nil
}

agent support(message) {
    model: "strong"
    tools: {refund}
    system: "Help the customer. Ask for approval before large refunds."
} {
    history := {system(system), user(message)}
    for i := 0; i < 8; i++ {
        r, err := turn { messages: history }
        if err != nil { return nil, err }
        if r.status == "final_answer" { return r.text, nil }
        if r.status == "stop" {
            return nil, {kind: "stopped", message: r.reason}
        }
        for _, c := range r.calls {
            history = append(history, assistant_call(c))
            if c.tool == "refund" && c.args.amount > 100 {
                return {status: "pending", token: snapshot(history, c)}, nil
            }
            v, e := dispatch(c)
            if e != nil {
                history = append(history, tool_error(c.id, e.message))
            } else {
                history = append(history, tool_result(c.id, v))
            }
        }
    }
    return nil, {kind: "stopped", message: "max turns"}
}
```

## 21. Open Design Decisions

These must be decided before implementation:

1. Should `react` become a keyword/block, or stay `llm.react` plus `agent`
   ambient defaults? This document recommends a `react` block because it is the
   common authoring path.
2. Should short message constructors be globally available or only inside agent
   bodies? This document recommends agent-body-only sugar.
3. Should model aliases be syntax or config file only? This document permits
   syntax but keeps provider binding in host code.
4. Should `budget {}` be allowed as an expression? This document says no.
5. Should money literals use `$0.25`? This document defers them.

## 22. Implementation Milestones After Design Approval

1. Parser accepts attributes and stores them on AST nodes.
2. Parser accepts `tool` declarations and lowers to existing `llm.tool`.
3. Runtime adds ambient agent frame support used by `turn`, `dispatch`, and
   `react`.
4. Parser accepts `agent` declarations and lowers to ambient-frame execution.
5. Parser accepts `turn` block expressions/declarations and lowers to
   `llm.turn`.
6. Parser accepts `react` block expressions and lowers to `llm.react`.
7. Parser accepts `budget` blocks and lowers to ambient budget frames.
8. Linter emits AI metadata/capability diagnostics.
9. Formatter preserves attributes and formats AI blocks.
10. Integration tests cover stdlib-vs-syntax equivalence, real provider gated
    smoke, record/replay, streaming, structured output, and capability failures.
