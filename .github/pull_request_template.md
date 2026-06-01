## Summary

## Validation

```bash
go run ./cmd/leia check .
go test ./...
```

## Compatibility

- [ ] Language-visible behavior is unchanged, or `docs/spec/language.md` is updated.
- [ ] Stable syntax is unchanged, or `docs/spec/grammar.ebnf` and parser tests are updated.
- [ ] Stable feature coverage is unchanged, or `tests/feature_matrix.json` is updated.
- [ ] Standard-library catalog metadata is unchanged, or `internal/stdlib/catalog` is updated.
- [ ] Performance-sensitive code is unchanged, or benchmark evidence is included using `docs/contributing/performance.md`.

## Security

List any changed host capabilities, Go bindings, network/process/file access,
AI provider behavior, or resource-budget behavior.
