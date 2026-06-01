# tree-sitter-leia

Minimal tree-sitter grammar for Leia source files.

This first version is intentionally scoped to editor/tooling use and tracks the
stable user-facing grammar in `docs/spec/grammar.ebnf` where practical. It
covers:

- basic declarations and statements
- functions and function literals
- table, list, dense, message, turn, and agent literals
- goroutine/channel/select syntax
- AI-native `models`, `agent`, `tool`, and `budget` forms

## Verify

From this directory:

```sh
npm install
npm test
```

If the tree-sitter CLI is already available globally, `tree-sitter generate`
and `tree-sitter test` are sufficient.
