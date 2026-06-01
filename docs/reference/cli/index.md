# Leia CLI Reference

This page describes the stable command families. Detailed generated command
tables are emitted by:

```bash
go run ./cmd/leia doc generate --output docs/reference/generated
```

Stable commands:

| Command | Purpose |
|---|---|
| `leia run` | Run a script file. |
| `leia eval` | Run source passed on the command line. |
| `leia repl` | Start the interactive shell. |
| `leia fmt` | Normalize source formatting. |
| `leia lint` | Report syntax and source diagnostics. |
| `leia test` | Run `.leia` tests and optional stdout goldens. |
| `leia check` | Run local formatter, linter, test, manifest, and docs gates. |
| `leia bench` | Run benchmark harnesses. |
| `leia diag` / `leia diagnose` | Collect runtime and JIT diagnostics. |
| `leia mod` | Manage `leia.mod`, `leia.sum`, vendoring, and module checks. |
| `leia doc` | Generate or check documentation. |
| `leia ci` | Run canonical local CI profiles. |
| `leia capabilities` | Print binary capabilities and tooling support. |
| `leia env` | Print environment, cache, project, and platform state. |
| `leia version` | Print version and build metadata. |

