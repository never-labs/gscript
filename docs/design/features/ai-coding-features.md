# AI Coding Features

## Goal

Leia should become a strong language for AI-assisted coding. The user should be
able to inspect, approve, replay, and roll back automated changes without
building a custom harness for every project.

## Plan / Explain / Run

```leia
p := plan {
    step "scan" {
        sh`rg "TODO" .`
    }
    step "edit" {
        patch "main.leia" {
            replace `old` with `new`
        }
    }
    step "test" {
        sh!`go test ./...`
    }
}

p.explain()
p.run()
```

Requirements:

- plans are values;
- plans are inspectable before execution;
- plans can be serialized;
- plans can be run step-by-step;
- plan steps can use shell, patch, code query, AI, and tests.

## Patch DSL

```leia
patch "internal/parser/parser.go" {
    find `case lexer.TOKEN_STRING:`
    insert_after `
        // new behavior
    `
}
```

Requirements:

- file-scoped patch operations;
- dry-run diff;
- exact-match operations;
- structured diagnostics when a patch does not apply;
- rollback integration;
- no silent partial patching.

## Code Query DSL

```leia
code {
    find func where name contains "agent"
    find calls to "llm.turn"
    find imports of "internal/runtime"
}
```

Requirements:

- query source by symbol, call, import, function, file, and package;
- return structured results;
- allow text fallback where semantic data is unavailable;
- useful from AI agents and CLI tools.

## Transaction / Rollback

```leia
transaction {
    patch "main.leia" {
        replace `old` with `new`
    }
    sh!`go test ./...`
}
```

Requirements:

- file changes are reversible;
- failed commands trigger rollback;
- generated diff is inspectable;
- user can keep or discard changes explicitly;
- works with patch, shell, code query, and formatter.

## Record / Replay

```leia
record "run-001" {
    result := agent { user: `Summarize ${topic}` }
    resp := http`GET https://example.com`
    tests := sh`go test ./...`
}

replay "run-001" {
    ...
}
```

Requirements:

- record LLM, HTTP, shell, and other nondeterministic effects;
- replay without network/process side effects where possible;
- expose mismatch diagnostics;
- support CI regression use.

## Approval

```leia
approve "Apply generated patch?" {
    diff()
}
```

Requirements:

- human-in-the-loop checkpoint;
- structured prompt/body;
- can return approve/deny;
- works in CLI and embedding contexts;
- should not require AI-specific code.

## Non-Goals

- Do not hide file edits behind model calls.
- Do not silently execute generated plans.
- Do not make patching a raw string replacement API only.
