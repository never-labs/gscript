# Documentation Maintenance

The documentation source of truth is split by ownership:

- Language syntax and semantics: `docs/spec/index.md`.
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
function-level generator is added. Generated Markdown reference pages should be
refreshed with:

```bash
go run ./cmd/leia doc generate --layout site --output docs
```

The same source metadata can be emitted as machine-readable JSON for site
builders or release tooling:

```bash
go run ./cmd/leia doc generate --format json
```

`scripts/docs_check.sh` compares the checked-in Markdown reference pages with
the current generated output, so stale generated docs fail the docs gate.

The public spec entrypoint `docs/spec/index.md` is a generated single-page
specification assembled from the chapter files in `docs/spec/`. Edit the chapter
files first, then refresh the published spec and local preview with:

```bash
go run ./cmd/leia doc spec-preview --write-index --output docs/spec/index.html
```

Run the same gate through the CLI when checking documentation locally:

```bash
go run ./cmd/leia doc check
```
