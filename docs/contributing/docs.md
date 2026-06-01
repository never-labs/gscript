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

When adding a standard-library function, put its user-visible contract near the
module implementation and update catalog metadata if the module's safety layer
or capabilities change. Generated docs should be refreshed with:

```bash
go run ./cmd/leia doc generate --output docs/reference/generated
```

