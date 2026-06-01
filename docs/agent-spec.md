# Leia Agent Layer Specification — v1 draft

> Status: design draft.
> Scope: `models` / `tool` / `budget` / `turn` / `agent` plus the capability + result/error conventions around them.
> General language features (functions, tables, control flow, multi-return) follow `docs/language-spec.md`.

---

## 0. Design principles (binding)

1. **Dynamic, Lua-semantics, Go syntax.** No type system. No typed struct literals. No type annotations.
2. **Five keywords**: `models`, `tool`, `budget`, `turn`, `agent`. Nothing else gets reserved.
3. **`turn` is the only LLM-call primitive.** All LLM I/O — including the LLM calls inside any `agent` body — goes through `turn`. Runtime is free to inline, but observable behavior must match.
4. **`agent` is metadata + body.** Two blocks: the first declares model/tools/caps/system; the second is regular Leia code where the user writes the loop and calls `turn` directly.
5. **`budget` is ambient.** `budget { config } { body }` pushes a frame; nested calls read it via runtime helpers. No `ctx` parameter threaded through Leia code.
6. **Declaration metadata in comments.** `tool` annotations use Go-style `//leia:` directives.
7. **`err` vs `result.status`.** `err` = "something went wrong, propagate." `result.status` = "the call finished one of several ways, dispatch here." Both are plain tables with string fields.
8. **No hidden machinery in `agent`.** Schema validation, HITL, retry, dispatch, memory — none of these are language features. They are patterns the user writes in the body, optionally helped by `loop.*` and `msg.*` standard library.

---

## 1. Lexical additions

### 1.1 Keywords

```
models  tool  budget  turn  agent
```

Five total. `requires`, `desc`, `params`, `caps`, `example`, `shape`, `cap` — none are keywords (`cap` is a reserved namespace root; `caps` is a field name; rest are identifiers).

### 1.2 Literals

| Form | Type | Example |
|---|---|---|
| integer + `ms` / `s` / `m` / `h` | duration | `60s`, `500ms`, `2m` |
| `$<decimal>` | money (USD) | `$0.20`, `$1.50` |
| underscore-grouped int | integer | `30_000`, `1_000_000` |

### 1.3 Capability expression

```
CapExpr  = CapTerm { '|' CapTerm }
CapTerm  = 'cap' '.' Ident '.' Ident
         | 'cap' '.' 'none'
         | 'cap' '.' 'all'
         | '(' CapExpr ')'
```

### 1.4 Comment directives

Lines beginning with `//leia:` immediately preceding a `tool` declaration are parsed as directives.

```
//leia:requires <CapExpr>
```

Only one directive in v1. All other `//` lines before the tool are doc comment; Go-style first-sentence-as-summary applies.

---

## 2. Capability flags

Built-in flags (v1, closed set):

```
cap.fs.read           cap.fs.write
cap.net.http
cap.db.read           cap.db.write
cap.env.read          cap.env.write
cap.payments.refund   cap.payments.charge
cap.smtp.send         cap.calendar.write
cap.cmd.exec
```

Plus `cap.none` (empty) and `cap.all` (warning at load).

Stored as 64-bit bitmask. `|` union, `&` intersection. `a ⊆ b` iff `a & b == a`.

---

## 3. `models` declaration

```leia
models {
    "default":          anthropic({ api_key: env("ANTHROPIC_API_KEY"), model: "claude-opus-4-7" })
    "claude-haiku-4-5": anthropic({ api_key: env("ANTHROPIC_API_KEY"), model: "claude-haiku-4-5" })
    "gpt-5":            openai({ api_key: env("OPENAI_API_KEY"), model: "gpt-5" })
}
```

**Rules**:
- File-scope only.
- Cross-file merging; duplicate key = load error.
- Lazy: right-hand side invoked once on first reference to the key.
- Key `"default"` is the implicit fallback when `model:` is omitted on a `turn`.

**Built-in providers (v1)**: `anthropic`, `openai`, `openai_compat`.

---

## 4. `tool` declaration

```leia
// issue_refund issues a refund to the original payment method.
// amount is in USD.
//
//leia:requires cap.payments.refund
tool issue_refund(order_id, amount, reason) {
    return payments.refund(order_id, amount, reason)
}
```

**Syntax**:
```
ToolDecl = 'tool' Ident '(' [ParamList] ')' Block
```

**Rules**:
- `//leia:requires <CapExpr>` is mandatory (pure tools write `cap.none`).
- Body returns `(value, err)` like every Leia function.
- Doc comment becomes the LLM-facing description; first sentence is the summary.
- No kind hints on parameters — LLM infers from name + doc; runtime tries lightweight coercion on incoming args.

**Error categorization** (used when a tool is invoked via `dispatch` inside an agent body):

| `err.kind` | Category | Caller behavior |
|---|---|---|
| `network`, `provider`, `internal` | transient | retry with backoff |
| `validation`, `policy`, `user` | recoverable | feed `err.message` to LLM as tool error |
| anything else | fatal | propagate up |

This categorization is library convention (`dispatch` implements it). User code can categorize differently.

---

## 5. `budget` block

```leia
budget {
    tokens: 30_000
    time:   60s
    money:  $0.20
    cancel: signal
    store:  redis_store
} {
    // body
}
```

**Syntax**:
```
BudgetStmt = 'budget' '{' FieldBindings '}' Block
```

A **statement**, not an expression. Pushes an ambient frame on entry, pops on exit.

**Fields** (all optional):

| Field | Type | Meaning |
|---|---|---|
| `tokens` | number | max LLM tokens across nested turns |
| `calls` | number | max tool dispatches across nested |
| `turns` | number | max LLM turns across nested |
| `time` | duration | wall-clock deadline from block entry |
| `money` | money | cost cap |
| `cancel` | value with `is_cancelled()` | external cancellation signal |
| `store` | value with `load/save/delete(token)` | checkpoint store |

**Nesting**: `time` takes the minimum of parent/child. Counters charge both parent and child; the more restrictive wins.

**Exhaustion**: the next `turn` returns `(nil, { kind: "budget" | "deadline" | "cancelled", ... })`.

---

## 6. `turn` — the only LLM-call primitive

`turn` is a callable value, parallel to `func`:

| Form | Meaning |
|---|---|
| `turn name(params) { spec }` | named declaration |
| `turn { spec }` | anonymous value |
| `turn { spec }()` | IIFE |

```
TurnLit  = 'turn' [ '(' [ParamList] ')' ] '{' FieldBindings '}'
TurnDecl = 'turn' Ident '(' [ParamList] ')' '{' FieldBindings '}'
TurnCall = TurnLit '(' [ArgList] ')'  |  Ident '(' [ArgList] ')'
```

**Fields**:

| Field | Required | Meaning |
|---|---|---|
| `model` | no (inherits ambient agent's model, then `"default"`) | model key from `models` |
| `messages` | yes | conversation messages |
| `tools` | no (inherits ambient agent's tools) | tool values; given to LLM as schema; turn does **not** dispatch them |
| `force_tool` | no | tool value / `"any"` / `"none"` |
| `max` | no | per-call cap `{ tokens, time }` |
| `stream` | no | bool |

**Return shape** `(result, err)`:

```
result.status: "final_answer" | "tool_calls" | "stop"
result.text                                (final_answer)
result.calls: [{ id, tool, args }, ...]    (tool_calls)
result.reason                              (stop)
result.usage:  { input_tokens, output_tokens, cost, latency }

err.kind: "budget" | "deadline" | "cancelled" | "network" | "provider"
```

**Invariant**: `turn` is the only operation in the language that reaches the LLM. It does not dispatch tools, validate schema, retry, or persist state. It is a pure protocol-layer translation.

---

## 7. `agent` — metadata + body

`agent` is a callable value with two consecutive blocks: a metadata block followed by a body block.

| Form | Meaning |
|---|---|
| `agent name(params) { meta } { body }` | named declaration |
| `agent(params) { meta } { body }` | anonymous with params (value) |
| `agent { meta } { body }` | anonymous no params (value) |
| `agent { meta } { body }()` | IIFE |
| `name(args)` | invoke a declared agent |

```
AgentDecl = 'agent' Ident '(' [ParamList] ')' '{' MetaFields '}' Block
AgentLit  = 'agent' [ '(' [ParamList] ')' ] '{' MetaFields '}' Block
AgentCall = AgentLit '(' [ArgList] ')'  |  Ident '(' [ArgList] ')'

MetaFields = { Ident ':' Expression [ ',' ] }
```

### 7.1 Metadata fields

| Field | Required | Meaning |
|---|---|---|
| `model` | no | default model for `turn` calls inside body |
| `tools` | no | default tools for `turn` calls inside body; participates in cap flow |
| `caps` | no (default = `union(tools.requires)`) | upper bound on capabilities |
| `system` | no | string; available inside body as variable `system` |

**Not** in metadata: `example`, `approve_when`, `budget`, `max_turns`. These are body responsibilities or come from ambient.

### 7.2 Body

The body is a normal Leia code block. In the implemented syntax this body is
introduced by `flow { ... }` when the user wants a custom loop. Inside, the
following names are injected as ordinary lexical locals at the top of the body:

- the agent's `params`
- `model`, `tools`, `system`, `caps`, and `capabilities` when those metadata
  fields are present
- ambient `turn { ... }()` which inherits the agent's `model` and `tools` as defaults
- standard library: `msg.*`, `dispatch`, `resume`, `snapshot`, `loop.*`

Other metadata fields, such as `user`, `response_format`, and `metadata`, are
not injected as implicit locals. This keeps the flow body close to lexical
scoping: the injection is equivalent to local declarations before the user's
first statement, and the user may shadow those names with later local
declarations.

The body returns `(value, err)` like a function. There is no spec-enforced shape
on `value`. Agent `output:` validation is automatic only for the default agent
loop; a custom `flow` returns exactly what it returns, so output validation must
be performed explicitly by user code or helpers.

### 7.3 `messages { ... }`

`messages { ... }` is an ordered message constructor. Role fields are shorthand
for message constructors, and bare expressions are inserted as-is, so initial
prompts and dynamic tool history can use one sequence:

```leia
history := messages {
    system: system
    user: question
    msg.assistant_call(call)
    msg.tool_result(call.id, value)
    user: "Summarize the evidence."
}
```

The role shorthands `system:`, `user:`, and `assistant:` lower to
`msg.system`, `msg.user`, and `msg.assistant`. Bare expressions are expected to
already be message-shaped tables.

### 7.4 Capability flow (load-time lint)

1. Compute `effective_caps`:
   - If `caps:` is given, use it.
   - Else, `union(t.requires for t in tools)`.
2. For each `t in tools`, require `t.requires ⊆ effective_caps`.
3. The enclosing scope's caps (`leia.WithCapabilities`) must include `effective_caps`.
4. If body declares sub-agents in scope, their `effective_caps` are also checked transitively.

### 7.5 Example

```leia
agent support(message) {
    model:  "default"
    tools:  [lookup_order, issue_refund, send_email]
    caps:   cap.db.read | cap.payments.refund | cap.smtp.send
    system: "Customer support. Decide: refund | exchange | escalate."
} {
    history := [msg.system(system), msg.user(message)]

    for i := 0; i < 16; i++ {
        t, err := turn { messages: history }()
        if err != nil { return nil, err }

        switch t.status {
        case "final_answer":
            return t.text, nil
        case "tool_calls":
            for _, c := range t.calls {
                history = append(history, msg.assistant_call(c))
                if c.tool == "issue_refund" and c.args.amount > 100 {
                    return { status: "pending", token: snapshot(history, c), payload: c }, nil
                }
                v, e := dispatch(c)
                if e != nil { return nil, e }
                history = append(history, msg.tool_result(c.id, v))
            }
        case "stop":
            return nil, { kind: "stopped", reason: t.reason }
        }
    }
    return nil, { kind: "stopped", reason: "max_turns" }
}
```

---

## 8. Standard library helpers

These are plain functions, **not keywords**.

### 8.1 `msg.*` — message constructors

```
msg.system(text)              → { role: "system",    text }
msg.user(text)                → { role: "user",      text }
msg.assistant(text)           → { role: "assistant", text }
msg.assistant_call(call)      → { role: "assistant", tool_call: call }
msg.tool_result(call_id, v)   → { role: "tool",      tool_use_id, value }
msg.tool_error(call_id, m)    → { role: "tool",      tool_use_id, error }
```

### 8.2 `toolof(agent, opts)` — expose an agent as a tool

```
toolof(agent_value, {
    name: "delegate_research",
    description: "Delegate research to a specialist agent.",
    requires: ["none"],
})
```

`toolof` is also available as `llm.toolof` and `llm.agent_as_tool`. It returns a
normal tool table whose implementation invokes the original agent. If `params`
is omitted, the agent's parameter names are used. If `schema` is omitted and the
agent has an `output:` shape, that output shape becomes the tool schema. When
the agent returns a structured result with `value`, the tool result is that
`value`; otherwise it returns the result text or the raw result table.

For the common static case, an agent value may also appear directly in an
agent's `tools:` list:

```leia
agent supervisor(q) {
    tools: [delegate_research]
    user: q
}
```

This is equivalent to applying `toolof(delegate_research)` at runtime. The
explicit `toolof` form remains useful when the caller wants to override name,
description, capabilities, or schema.

### 8.3 `dispatch(call)` — execute a tool call from LLM

```
dispatch(call) → (value, err)
```

Looks up `call.tool` in the ambient agent's `tools` list. Applies lightweight coercion to `call.args` based on the tool's parameter names. Invokes the tool. Returns whatever the tool returned. If lookup fails, returns `{ kind: "capability", message: "tool not in scope" }`.

### 8.4 `snapshot(history, pending)` and `resume(token, approval)` — HITL primitives

```
snapshot(history, pending_call) → token
resume(token, approval)         → (result, err)
```

`snapshot` serializes `{ history, pending_call, current_ambient_meta }` under a token, writing to ambient `store` if set. Returns the token.

`resume` loads the snapshot, materializes the approval into history (success → dispatch + tool_result; deny → synthetic tool_error), then **re-enters the agent body** at the point after the original `snapshot` call returned. Fresh budget.

`approval` shape: `{ ok: bool, reason: string?, args: table? }`. If `args` is set on approval, the pending call's args are replaced before dispatch.

### 8.5 `loop.*` — convenience loops (optional)

```
loop.react({ user, example?, approve_when?, max_steps? })
loop.simple({ user, max_steps? })
loop.plan_execute({ user, plan_model?, exec_model? })
loop.reflect({ user, max_iters? })
```

Each is a regular function callable from inside an agent body. They use ambient `tools` / `model` / `system`. Each returns `(result, err)` with `result.status ∈ {"done", "pending", "stopped"}`.

`loop.react` implements the standard pattern from §7.4. Users who want custom semantics write the loop directly.

### 8.6 `chat.*` — memory strategies (optional)

```
chat.token_count(history)               → number
chat.window(history, max_tokens: N)     → trimmed history
chat.summarize(history, opts)           → compressed history
chat.merge(prior, additions)            → concat
```

Used by callers to manage cross-call history. Agent itself is stateless across invocations.

---

## 9. Result and error conventions

### 9.1 Errors

```
err.kind     "budget" | "deadline" | "cancelled" | "network"
            | "provider" | "capability" | "validation"
            | "policy"   | "user"      | "internal"
            | "stopped"
err.message  string
err.dimension  (kind == "budget")
err.provider   (kind == "provider")
err.status     (kind == "provider", HTTP status)
err.cause      (optional, nested error)
```

Check by direct field access:

```leia
if err != nil {
    if err.kind == "budget" { /* graceful */ }
    return nil, err
}
```

No `errors.is` / `errors.as`. No typed error hierarchy.

### 9.2 Result status convention

By convention, agent bodies return tables with a `status` field for sum-type-style dispatch. Common conventions:

```
{ status: "done",     value: <answer> }
{ status: "pending",  token, payload }       // HITL
{ status: "stopped",  reason }
```

The convention is followed by `loop.*` helpers and recommended for user-written bodies, but **not enforced** by the language. A body may return any shape.

---

## 10. Ambient state

The runtime maintains a stack of ambient frames. Each frame may contribute:

```
budget.tokens, budget.calls, budget.turns, budget.time, budget.money
budget.cancel, budget.store

agent.model, agent.tools, agent.system, agent.caps
```

**Stack mutation**:
- `budget { } { }` pushes / pops a budget frame.
- Entering an `agent { meta } { body }` invocation pushes an agent frame.

**Read**:
- `turn { ... }()` reads `model` and `tools` from the nearest agent frame as defaults.
- `dispatch(call)` reads `tools` from the nearest agent frame.
- Library helpers (`runtime.deadline()`, `runtime.cancelled()`, `runtime.budget_remaining()`) read budget frames.

The Go embedding API seeds the bottom frame:

```go
vm.Run(ctx, prog,
    leia.WithBudget(leia.Budget{Tokens: 100_000, Time: 5*time.Minute}),
    leia.WithCheckpointStore(store),
    leia.WithCapabilities(cap.DBRead | cap.SMTPSend),
)
```

Leia code never sees `ctx` as a parameter.

---

## 11. Reference grammar (agent-layer subset)

```
Module       = { TopDecl }
TopDecl      = ModelsDecl | ToolDecl | TurnDecl | AgentDecl | <base-spec top-level>

ModelsDecl   = 'models' '{' { ModelBinding } '}'
ModelBinding = StringLit ':' Expression [ ',' ]

ToolDecl     = 'tool' Ident '(' [ParamList] ')' Block
               // metadata extracted from preceding //leia:requires + doc comment

TurnDecl     = 'turn'  Ident '(' [ParamList] ')' '{' FieldBindings '}'
TurnLit      = 'turn'  [ '(' [ParamList] ')' ] '{' FieldBindings '}'

AgentDecl    = 'agent' Ident '(' [ParamList] ')' '{' FieldBindings '}' Block
AgentLit     = 'agent' [ '(' [ParamList] ')' ] '{' FieldBindings '}' Block

BudgetStmt   = 'budget' '{' FieldBindings '}' Block

FieldBindings = { Ident ':' Expression [ ',' ] }

CapExpr      = CapTerm { '|' CapTerm }
CapTerm      = 'cap' '.' Ident '.' Ident | 'cap' '.' 'none' | 'cap' '.' 'all' | '(' CapExpr ')'

LitDuration  = <int> ( 'ms' | 's' | 'm' | 'h' )
LitMoney     = '$' <decimal>
```

---

## 12. Worked examples

### 12.1 Single-turn classifier

```leia
models {
    "default": anthropic({ api_key: env("ANTHROPIC_API_KEY"), model: "claude-haiku-4-5" })
}

agent classify(text) {
    model:  "default"
    system: "Classify sentiment as positive | negative | neutral. Reply with one word."
} {
    t, err := turn { messages: [msg.system(system), msg.user(text)] }()
    if err != nil { return nil, err }
    return t.text, nil
}
```

### 12.2 Self-correcting SQL agent

```leia
//leia:requires cap.db.read
tool list_tables() {
    return db.query("SELECT name FROM sqlite_master WHERE type='table'")
}

//leia:requires cap.db.read
tool describe(table) {
    return db.query("PRAGMA table_info(" + table + ")")
}

//leia:requires cap.db.read
tool run_sql(query) {
    if not is_readonly(query) {
        return nil, { kind: "validation", message: "only SELECT allowed" }
    }
    rows, err := db.query(query)
    if err != nil { return nil, { kind: "user", message: err.message } }
    return { rows: rows }, nil
}

agent ask_db(question) {
    model:  "default"
    tools:  [list_tables, describe, run_sql]
    caps:   cap.db.read
    system: "SQL analyst. Inspect schema, query. If SQL errors, fix it and retry."
} {
    history := [msg.system(system), msg.user(question)]
    errors_seen := 0

    for i := 0; i < 20; i++ {
        t, err := turn { messages: history }()
        if err != nil { return nil, err }

        switch t.status {
        case "final_answer":
            return t.text, nil
        case "tool_calls":
            for _, c := range t.calls {
                history = append(history, msg.assistant_call(c))
                v, e := dispatch(c)
                if e != nil {
                    if e.kind == "validation" or e.kind == "user" {
                        errors_seen = errors_seen + 1
                        if errors_seen > 5 {
                            return nil, { kind: "user", message: "too many SQL errors" }
                        }
                        history = append(history, msg.tool_error(c.id, e.message))
                    } else {
                        return nil, e
                    }
                } else {
                    history = append(history, msg.tool_result(c.id, v))
                }
            }
        case "stop":
            return nil, { kind: "stopped", reason: t.reason }
        }
    }
    return nil, { kind: "stopped", reason: "max_turns" }
}
```

### 12.3 Customer support with HITL (using `loop.react`)

```leia
import "loop"

agent support(message) {
    model:  "default"
    tools:  [lookup_order, issue_refund, send_email]
    caps:   cap.db.read | cap.payments.refund | cap.smtp.send
    system: "Customer support. Decide: refund | exchange | escalate."
} {
    return loop.react({
        user:    message
        example: { action: "refund", amount: 50, reason: "duplicate" }
        approve_when: func(c) {
            return c.tool == "issue_refund" and c.args.amount > 100
        }
    })
}

func handle_request(req) {
    budget { tokens: 30_000, time: 60s, money: $0.20, store: app.store } {
        result, err := support(req.message)
        if err != nil { return nil, err }
        switch result.status {
        case "done":    return { ok: true, decision: result.value }, nil
        case "pending": return { ok: false, awaiting: result.token, data: result.payload }, nil
        case "stopped": return { ok: false, stopped: result.reason }, nil
        }
    }
}

func handle_approval(token, approval) {
    budget { tokens: 20_000, time: 30s, store: app.store } {
        return resume(token, approval)
    }
}
```

### 12.4 Deep research (multi-agent composition)

```leia
agent plan(question) {
    model:  "default"
    system: "Decompose into 3-5 parallel sub-queries. Reply JSON: {subqueries: [...]}."
} {
    t, err := turn { messages: [msg.system(system), msg.user(question)] }()
    if err != nil { return nil, err }
    return json.parse(t.text), nil
}

agent investigate(subquery) {
    model:  "claude-haiku-4-5"
    tools:  [search_web, fetch_page]
    caps:   cap.net.http
    system: "Investigate one sub-query. Synthesize a one-paragraph finding."
} {
    history := [msg.system(system), msg.user(subquery)]
    for i := 0; i < 8; i++ {
        t, err := turn { messages: history }()
        if err != nil { return nil, err }
        switch t.status {
        case "final_answer":
            return t.text, nil
        case "tool_calls":
            for _, c := range t.calls {
                history = append(history, msg.assistant_call(c))
                v, e := dispatch(c)
                if e != nil { return nil, e }
                history = append(history, msg.tool_result(c.id, v))
            }
        case "stop":
            return nil, { kind: "stopped", reason: t.reason }
        }
    }
    return nil, { kind: "stopped", reason: "max_turns" }
}

agent synthesize(question, findings) {
    model:  "default"
    system: "Write a research brief from findings with inline citations."
} {
    body := "Q: " + question + "\n\nFindings:\n" + strings.join(findings, "\n\n")
    t, err := turn { messages: [msg.system(system), msg.user(body)] }()
    if err != nil { return nil, err }
    return t.text, nil
}

func deep_research(question) {
    budget { tokens: 200_000, time: 5m, money: $1.50 } {
        p, err := plan(question)
        if err != nil { return nil, err }

        g := errgroup.new()
        findings := make([], len(p.subqueries))
        for i, sq := range p.subqueries {
            i, sq := i, sq
            g.go(func() {
                r, e := investigate(sq)
                if e != nil { return e }
                findings[i] = r
                return nil
            })
        }
        if e := g.wait(); e != nil { return nil, e }

        return synthesize(question, findings)
    }
}
```

---

## 13. Test cases

Each test is a pair `(source.leia, expect.json)` under `tests/agent/`. Mock provider used.

### 13.1 Lexer

```leia
// T01 — duration arithmetic
assert(60s + 30s == 90s)
assert(500ms < 1s)

// T02 — money arithmetic
assert($1.00 + $0.50 == $1.50)

// T03 — underscore grouping
assert(1_000_000 == 1000000)
```

### 13.2 `models` lazy + default key

```leia
// T10 — module loads even with a "broken" model (lazy)
models { "broken": anthropic({ api_key: "" }) }
// expect: load OK; error on first use

// T11 — duplicate key across modules → load error

// T12 — "default" is fallback
models { "default": mock_provider() }
result, err := turn { messages: [msg.user("hi")] }()    // no model:
assert(err == nil)

// T13 — no "default", omitted model → load error
```

### 13.3 `tool` declaration & comment directives

```leia
// T20 — direct call is fully dynamic
//leia:requires cap.none
tool t1(x, y) { return x + y, nil }
v, _ := t1(1, 2)
assert(v == 3)
v, _ = t1("a", "b")
assert(v == "ab")

// T21 — missing //leia:requires → load error

// T22 — categorized errors flow correctly through dispatch
//leia:requires cap.none
tool flaky(x) { return nil, { kind: "network", message: "transient" } }
// when called via dispatch inside an agent body, dispatch retries
```

### 13.4 Capability flow

```leia
//leia:requires cap.db.read
tool needs_db(q) { return q, nil }

// T30 — caps default to union of tools.requires
agent a1() {
    tools: [needs_db]    // caps auto = cap.db.read
} {
    return nil, nil
}

// T31 — explicit caps too small → load error
agent a2() {
    caps:  cap.none
    tools: [needs_db]    // ← load error
} {
    return nil, nil
}

// T32 — explicit caps larger is allowed (upper bound)
agent a3() {
    caps:  cap.db.read | cap.smtp.send
    tools: [needs_db]
} {
    return nil, nil
}

// T33 — embedding-side caps gate
// Go: leia.WithCapabilities(cap.None)
//   any agent with non-empty caps → compile error
```

### 13.5 `budget` propagation

```leia
// T40 — exhausted budget → err.kind == "budget"
budget { tokens: 10 } {
    _, err := turn { messages: [msg.user("x")] }()
    assert(err.kind == "budget")
}

// T41 — nested budget is more restrictive
budget { tokens: 1000 } {
    budget { tokens: 30 } {
        _, err := turn { messages: [msg.user("x")] }()
        assert(err.kind == "budget")    // child hit first
    }
}

// T42 — deadline
budget { time: 10ms } {
    sleep(20ms)
    _, err := turn { messages: [msg.user("x")] }()
    assert(err.kind == "deadline")
}

// T43 — cancellation
sig := signal.new()
budget { cancel: sig } {
    sig.cancel()
    _, err := turn { messages: [msg.user("x")] }()
    assert(err.kind == "cancelled")
}
```

### 13.6 `turn` returns

```leia
// T50 — final_answer
result, err := turn { messages: [msg.user("hi")] }()
assert(result.status == "final_answer")

// T51 — tool_calls (turn itself doesn't dispatch)
//leia:requires cap.none
tool d(x) { return x, nil }
result, _ := turn { tools: [d], messages: [msg.user("use it")] }()
assert(result.status == "tool_calls")
assert(#result.calls == 1)

// T52 — max soft stop
result, _ := turn { messages: [msg.user("x")], max: { tokens: 50 } }()
assert(result.status == "stop")
assert(result.reason == "max")
```

### 13.7 `agent` body execution

```leia
// T60 — named agent call
agent classify(text) {
    system: "Reply 'positive' or 'negative'."
} {
    t, err := turn { messages: [msg.system(system), msg.user(text)] }()
    if err != nil { return nil, err }
    return t.text, nil
}
v, err := classify("good")
assert(v == "positive")

// T61 — ambient model/tools inherited by turn inside body
agent a(msg_text) {
    model: "claude-haiku-4-5"
    tools: [some_tool]
} {
    t, _ := turn { messages: [msg.user(msg_text)] }()   // no model:, no tools:
    // mock verifies the request was sent to claude-haiku-4-5 with some_tool schema
}

// T62 — agent as a value
my_agent := agent { system: "..." } { ... }
v, e := my_agent()
```

### 13.8 HITL

```leia
// T70 — snapshot returns a token; resume continues
agent flow(x) {
    tools: [issue_refund]
} {
    t, _ := turn { messages: [msg.user(x)] }()
    if t.status == "tool_calls" {
        c := t.calls[1]
        if c.args.amount > 100 {
            return { status: "pending", token: snapshot([msg.user(x)], c), payload: c }, nil
        }
    }
    // ...
}

budget { store: in_memory_store() } {
    r, _ := flow("refund $200")
    assert(r.status == "pending")
    
    r2, _ := resume(r.token, { ok: true })
    assert(r2.status == "done")
}
```

### 13.9 Embedding API (Go side)

```go
// G01 — top-level budget
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
_, err := vm.Run(ctx, prog, leia.WithBudget(leia.Budget{Tokens: 50_000}))

// G02 — capability gate
_, err := leia.Compile(src, leia.WithCapabilities(leia.CapDBRead))
// any agent declaring caps outside CapDBRead → compile error

// G03 — checkpoint store for HITL
_, err := vm.Run(ctx, prog, leia.WithCheckpointStore(redisStore))
```

---

## 14. Review pass — alternatives reconsidered

### A. Why both `turn` and `agent` instead of just one
- `turn` is the protocol primitive (atomic LLM call).
- `agent` is the orchestration primitive (metadata + body).
- They have orthogonal responsibilities. One cannot replace the other.

### B. Why `agent` body is user code instead of a built-in loop
- Built-in loops (auto schema retry, auto HITL, auto dispatch) hide semantics.
- Different scenarios (ReAct / Plan-Execute / Reflect / supervisor) want different loops.
- Common patterns are offered as `loop.*` library functions; users can use them or write their own.
- Single design rule: the language exposes mechanism, the library provides policy.

### C. Why metadata + body uses two consecutive blocks
- Same shape as `budget { config } { body }`.
- Visually separates "what this agent is" (metadata) from "what it does" (body).
- Avoids parser ambiguity between `name: expr` fields and statements.

### D. Why no `example:` field on `agent`
- Schema validation is a body-level concern (the user decides what shapes are acceptable).
- If wanted, use `loop.react({ example: ... })` from stdlib.

### E. Why no `approve_when:` on `agent`
- HITL is a body-level concern; the user already writes the loop.
- `loop.react({ approve_when: ... })` covers the common case via stdlib.

### F. Why `budget` is not an `agent` field
- Budget composes across agents (multiple agents share a budget).
- `budget { } { }` block represents that composition naturally.
- Putting budget on each agent invites scope confusion.

### G. Why `tool` metadata is in comments
- Go-idiomatic (`//go:build`, `//go:generate`, doc comments).
- Keeps signatures clean.
- Reduces keyword surface (no `requires`/`desc`/`params` keywords).

### H. Why errors are tables with `kind` instead of typed errors
- Dynamic language; typed errors don't fit Lua semantics.
- `err.kind` is debuggable and serializable.
- No parallel error taxonomy to learn.

### I. Why `result.status` is a convention, not enforced
- `agent` body returns anything the user wants.
- `loop.*` helpers follow the convention.
- Future linting could check exhaustiveness on `switch result.status` if needed.

### J. Why `turn` is mandatory for all LLM calls
- Single chokepoint for budget accounting, tracing, mocking, record/replay.
- Runtime can inline within `agent` for performance; semantics unchanged.

### K. v1 non-goals
- RAG primitives (vector stores, embedders, retrievers).
- Prompt template DSL.
- Custom providers.
- Capability self-declaration.
- Multi-modal input.
- Hosted observability platform.
- LangGraph-style state machine DSL.

---

## 15. Open questions

1. **`store` interface contract** — minimum methods `load(token)`, `save(token, snapshot)`, `delete(token)`. Validate against Redis + SQLite.
2. **Streaming with HITL** — if a body calls `turn { stream: true }()` and pauses, how are events delivered? v1 punts: streaming and HITL are mutually exclusive on a per-body basis.
3. **String concat operator** — `+` (Go) or `..` (Lua)? Resolve against base `language-spec.md`.
4. **Snapshot token format** — opaque; suggested 128-bit random base64url.
5. **File-level `caps` declaration** — phase 2; not in v1.
6. **`loop.*` standard library spec** — separate document; this spec only commits to the four functions in §8.4 existing.

---

## 16. Versioning

| Version | Scope |
|---|---|
| **v1** | This document. Behind feature flag (`leia.WithAgentLayer(true)`). |
| **v2** | Streaming inside HITL, file-level caps, more providers, MCP server export from `tool` decls, expanded `loop.*` library. |
| **v3** | `record` / `replay` / `diff`, eval suite primitives, `pool.HotSwap` integration. |
| **v4+** | Custom providers, capability self-declaration, semantic caching, multi-modal. |

---

## 17. Implementation map (informational)

| Construct | File |
|---|---|
| Lexer additions | `internal/lexer/lexer.go` |
| Parser (`models`, `tool`, `budget`, `turn`, `agent`) | `internal/parser/parser.go` |
| AST nodes | `internal/ast/ast.go` |
| `//leia:` directive extractor | new `internal/parser/directives.go` |
| Capability flow lint | new `internal/runtime/cap_lint.go` |
| `tool` runtime descriptor | new `internal/runtime/tool.go` |
| `turn` runtime engine | new `internal/runtime/turn.go` |
| `agent` runtime (ambient frame push/pop) | new `internal/runtime/agent.go` |
| Provider abstraction + built-ins | new `internal/runtime/providers/{anthropic,openai,openai_compat}.go` |
| Ambient frame stack | extension of `internal/runtime/interpreter.go` |
| HITL `snapshot` / `resume` | new `internal/runtime/hitl.go` |
| `msg.*` / `dispatch` / `loop.*` / `chat.*` stdlib | new `internal/runtime/agent_stdlib/` |
| Embedding API extensions | `leia/options.go`, `leia/vm.go`, `leia/program.go` |
| Test harness | new `tests/agent/` |

---

End of v1 draft. Open questions in §15 require resolution before promoting to stable. Section 14 records the binding rationale for current choices.
