# Contributing

Leia is still moving quickly. Changes should keep the language contract,
runtime behavior, tooling, docs, and performance gates aligned.

Participation is covered by [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

## Local Setup

Use a recent Go toolchain and run commands from the repository root.

```bash
go run ./cmd/leia version
go run ./cmd/leia check .
go test ./...
```

Focused loops:

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke
bash scripts/docs_check.sh
```

CI profiles are inspectable and reproducible locally:

```bash
go run ./cmd/leia ci smoke --list
go run ./cmd/leia ci pr --list
go run ./cmd/leia ci smoke
```

## Change Expectations

- Language-visible behavior needs a spec update in `docs/spec/language.md`.
- Stable syntax needs `docs/spec/grammar.ebnf` and parser tests.
- Stable features need coverage in `tests/feature_matrix.json`.
- Standard-library module changes need `internal/stdlib/catalog` metadata when
  module visibility, capabilities, or safety defaults change.
- Performance-sensitive changes should run an appropriate benchmark or
  `bash scripts/performance_gate.sh --feature-smoke`. See
  [`docs/contributing/performance.md`](docs/contributing/performance.md) for
  the evidence format.

## Commit Style

Use short imperative commit messages with a prefix when useful, for example:

```text
docs: update AI-native reference
runtime: fix channel close handling
bench: add soa column kernel case
```

## Pull Request Checklist

Before opening a pull request, include the commands you ran and any skipped
checks with a reason. At minimum, prefer:

```bash
go run ./cmd/leia check .
go test ./...
```

Security-sensitive changes should also mention enabled capabilities, host APIs,
and whether untrusted scripts can reach the changed behavior.

Issue and pull request templates live under `.github/`. Use the language
proposal template for syntax or semantic changes so spec, grammar, feature
matrix, and tests stay connected.
