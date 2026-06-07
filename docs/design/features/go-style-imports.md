# Go-Style Imports

## Goal

Leia should use Go-style imports as the primary module surface. Lua-style
`require()` should not be the main way users write code.

## Forms

```leia
import "json"
import p "path"
import rl "github.com/never-labs/leia-raylib/raylib"
```

Grouped imports:

```leia
import (
    "json"
    p "path"
    rl "github.com/never-labs/leia-raylib/raylib"
)
```

## Binding Rules

- `import "json"` binds the default name `json`.
- `import p "path"` binds `p`.
- Default names come from the final path element unless a module declares a
  canonical package name.
- Imports are file/module declarations, not arbitrary runtime statements.

## Remote Module Paths

Leia package paths should be compatible with Git-hosted modules:

```leia
import raylib "github.com/never-labs/leia-raylib/raylib"
```

The module system may use `leia.mod` / `leia.sum`, but source imports should
look like normal code, not a registry-specific package name.

## Dynamic Loading

Dynamic loading should be explicit:

```leia
mod := import_dynamic(name)
```

Ordinary `import` should be static enough for lint, format, LSP, package
resolution, capability summaries, and build tooling.

## Require Compatibility

`require()` may remain temporarily for compatibility, but it should be treated
as legacy style. New examples, generated code, and tutorials should use import.

## Non-Goals

- No dot import.
- No implicit package import by first use.
- No npm-style publish workflow requirement.
