# Packages And Modules

Leia uses a Go-style decentralized package model. A module path can be a
GitHub-style repository path, and the repository's `leia.mod` file is the
source of truth. There is no npm-style publish step or central registry in the
core workflow.

## Create A Module

```bash
go run ./cmd/leia mod init --module github.com/example/leia-tools
```

This writes `leia.mod`:

```text
module github.com/example/leia-tools
leia 0.1
```

Add capabilities as the project grows:

```text
capability fs.read
capability llm.turn
```

Capabilities are summaries for hosts and reviewers. They do not grant access by
themselves; host configuration still decides what a script may do.

## Add Dependencies

```bash
go run ./cmd/leia mod add github.com/never-labs/leia-raylib@v0.1.0
go run ./cmd/leia mod graph --json
go run ./cmd/leia mod capability --json
```

`require` directives should name modules by stable repository paths:

```text
require github.com/never-labs/leia-raylib v0.1.0
```

Use `replace` for local development:

```text
replace github.com/never-labs/leia-raylib => ../leia-raylib
```

## Download, Verify, And Vendor

Resolve remote modules into the local cache:

```bash
go run ./cmd/leia mod download --json
```

The command records hashes in `leia.sum`. Verify cached and vendored modules:

```bash
go run ./cmd/leia mod verify --json
```

Vendor dependencies for hermetic execution:

```bash
go run ./cmd/leia mod vendor --json
go run ./cmd/leia run --mod=vendor main.leia
```

Use readonly mode in CI when dependency changes should be explicit:

```bash
go run ./cmd/leia run --mod=readonly main.leia
```

## Go-Native Bindings

Leia packages that expose Go bindings can keep Go requirements in the same
`leia.mod` file:

```text
go 1.24
go require github.com/gen2brain/raylib-go/raylib v0.55.1
go replace github.com/example/native => ./native
```

Generate a Go module view when building bindings:

```bash
go run ./cmd/leia mod gomod
```

This keeps Leia package metadata and Go dependency metadata coupled without
inventing a second package manager.

## Package Discovery

The preferred public discovery path is repository-based:

1. Publish a normal GitHub repository.
2. Include `leia.mod`.
3. Tag releases with semantic versions.
4. Document capabilities and examples in the repository.

A future `pkg.leia.dev`-style site can index public repositories and render
their `leia.mod`, docs, examples, and capability summaries. That index should
not be required for install or execution.

## Release Checklist For A Package

- `leia.mod` has a stable `module` path.
- `leia mod verify --json` passes.
- `leia mod capability --json` shows expected host capabilities.
- Examples run without hidden local paths.
- Go-native bindings use explicit `go require` and `go replace` directives.
- README documents any required host allowlist entries.

See [module reference](../reference/modules/index.md) for directive details.

