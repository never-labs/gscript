# Application Runtime Features

## Goal

Leia should support larger, long-running scripts and AI-driven applications.
These features are not all first-phase language changes, but they shape the
future application model.

## Actor Model

```leia
actor worker {
    receive job {
        result := process(job)
        send result
    }
}
```

Requirements:

- actor values;
- mailbox/message semantics;
- structured shutdown;
- useful for NPCs, agents, workers, and background tasks;
- should compose with existing Go-like concurrency.

## Agent Object Protocol

```leia
npc_agent := agentify {
    state: npc
    perceive: fn(world) { return context }
    decide: fn(ctx) { return intent }
    remember: fn(event) { ... }
}
```

Requirements:

- any object/function can expose agent behavior;
- standard hooks: perceive, decide, act, remember;
- useful for games, automation agents, and coding agents;
- no hardcoded game-specific behavior in the language.

## Timeline / Event Sourcing

```leia
timeline town {
    event npc_spoke(actor, text)
    event npc_moved(actor, location)

    reduce state {
        npc_spoke => memory.append(actor, text)
        npc_moved => locations[actor] = location
    }
}
```

Requirements:

- append-only event stream;
- reducer-defined state reconstruction;
- replay from a point in time;
- inspectable history;
- suitable for games, agents, workflows, and tests.

## Time Travel Debugging

```leia
debug town {
    at "1596-04-16 noon"
    inspect npc("chen_qing")
    replay next 10
}
```

Requirements:

- inspect historical state;
- replay forward;
- compare before/after;
- integrate with record/replay.

## Self-Describing Program

```leia
describe module
describe dialect "agent"
describe support_agent
```

Requirements:

- expose schema, capabilities, examples, tools, and docs;
- useful to LSP, docs, AI coding agents, and runtime introspection;
- no dependency on generated external docs.

## Notebook / Cell

```leia
cell "load events" {
    events := jsonl`persistence/events.jsonl`
}

cell "show conversations" {
    events |> filter(fn(e) { return e.type == "conversation" })
}
```

Requirements:

- exploratory execution units;
- stable state visibility;
- integration with playground/tour later.

## Priority

These are future-facing. The most important early requirements are:

1. self-describing program;
2. record/replay integration;
3. actor/agent object protocol;
4. timeline/event sourcing.
