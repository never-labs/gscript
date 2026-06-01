# Tooling

Leia treats tools as part of the language product.

Daily commands:

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke
go run ./cmd/leia check --no-docs .
```

Project-level checks:

```bash
go run ./cmd/leia ci smoke
python3 tests/manifest.py check tests benchmarks
bash scripts/worktree_audit.sh
```

Documentation:

```bash
go run ./cmd/leia doc generate --layout site --output docs
go run ./cmd/leia doc generate --format json
go run ./cmd/leia doc check
```

Performance and diagnostics:

```bash
go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1
go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks
```
