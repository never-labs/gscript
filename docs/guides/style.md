# Leia Style Guide

This guide describes the house style used by Leia examples, tests, and
documentation. `leia fmt` owns whitespace and layout; this page covers choices
that are still semantic or stylistic.

## Names

Use lower snake case for ordinary script bindings:

```leia
order_total := 42
retry_count := 3

func normalize_name(name) {
    return string.lower(string.trim(name))
}
```

Use short names for small loop scopes, and descriptive names for values that
cross function, agent, or module boundaries.

```leia
sum := 0
for _, n := range values {
    sum = sum + n
}

customer_profile := load_customer(id)
```

Tool and agent names should read like capabilities or actions:

```leia
tool lookup_order(id) { ... }

agent support_triage(message) { ... }
```

## Data Shapes

Prefer tables with named fields at API boundaries. Positional arrays are good
for small local lists, but named fields age better when Go hosts, LLM tools, or
tests consume the value.

```leia
return {
    id: order.id,
    status: order.status,
    refundable: order.refundable,
}
```

Keep dense arrays and SoA data in clearly data-oriented code. Convert back to
plain records before crossing into host UI, logs, or LLM prompts.

## Errors

Use result-plus-error returns for recoverable failures.

```leia
func parse_row(row) {
    parts := string.split(row, ":")
    if #parts != 2 {
        return nil, "bad row: " .. row
    }
    return { name: parts[1], score: tonumber(parts[2]) }, nil
}
```

Use `error` for invalid program state or contract violations that should abort
the current operation. Use `pcall` only at a boundary where recovery is useful.

```leia
ok, value := pcall(require_field, row, "name")
if !ok {
    print("skip row", value)
}
```

## Modules And Capabilities

Keep host effects explicit. A script that needs filesystem, network, process,
or LLM access should make that dependency obvious through imports, tools, or
file directives.

```leia
//leia:cap fs.read,llm.turn
//leia:feature ai-native
```

For tools, describe parameters with directives so hosts, docs, and editor
integrations can surface useful metadata.

```leia
//leia:requires orders.read
//leia:param id order id
tool lookup_order(id) {
    return orders[id], nil
}
```

## AI-Native Code

Use the simplest AI construct that fits the job:

- `turn { user: ... }` for one request.
- `agent name(args) { ... }` for reusable prompt capsules.
- `flow { ... }` only when you need custom multi-step control.
- `evaluate` blocks for regression checks.

Prefer explicit message history when memory matters.

```leia
history := messages {
    system: "Remember facts exactly."
    user: "project=ORCHID owner=ADA"
}

first, err := turn { messages: history }
if err != nil { return nil, err }

history[#history + 1] = msg.assistant(first.text)
history[#history + 1] = msg.user("Recall the project and owner.")
```

For agent outputs consumed by code, ask for JSON and validate the returned
shape. Avoid parsing free-form prose in production paths.

## Concurrency

Prefer channel-based ownership transfer over shared mutable tables. When shared
state is required, use synchronization primitives and keep the critical section
small.

```leia
out := make(chan, #jobs)
for _, job := range jobs {
    go func(item) {
        out <- process(item)
    }(job)
}
```

Use context-aware host operations for long-running scripts so callers can
cancel work predictably.

## Tests

Put ordinary script tests under `tests/` and run them with `leia test`. Use
golden stdout files when the output is the contract. Use `evaluate` when the
contract is agent behavior, replay drift, or prompt regression.

```bash
go run ./cmd/leia test --json --output test-report.json tests
go run ./cmd/leia evaluate --replay examples/evaluate/agent_replay.records.json examples/evaluate/agent_replay.leia
```

