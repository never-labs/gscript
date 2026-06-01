# Documentation Maintenance

The documentation source of truth is split by ownership:

- Language syntax and semantics: `docs/spec/language.md`.
- Standard-library module inventory: `internal/stdlib/catalog` plus generated
  reference under `docs/reference/stdlib/`.
- CLI command inventory: `cmd/leia` command registry plus generated reference
  under `docs/reference/cli/`.
- Tutorials and guides: hand-written prose under `docs/tutorial/` and
  `docs/guides/`.

Prefer generated reference for long API inventories. Hand-written docs should
explain concepts, examples, safety boundaries, and tradeoffs.

When adding a standard-library module, update `internal/stdlib/catalog` with its
layer, description, safe-default status, and required capabilities. The docs
generator reads that metadata directly. When adding functions to an existing
module, put the user-visible contract near the module implementation until the
function-level generator is added. Generated docs should be refreshed with:

```bash
go run ./cmd/leia doc generate --output docs/reference/generated
```
