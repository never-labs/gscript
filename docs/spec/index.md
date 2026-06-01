# Leia Language Specification

This is the normative reference for the Leia programming language. It defines
the source syntax, value model, execution semantics, module behavior,
AI-native syntax, and implementation obligations that user-facing tools,
interpreters, bytecode VMs, JITs, and embedding APIs must preserve.

Leia uses Go-flavored syntax with dynamic values, Lua-compatible table and
multi-return behavior where useful, explicit host capabilities, Go-native
embedding, source-level hot reload, and AI-native agent syntax.

## Version

This specification describes the language surface implemented by the current
repository version. A release may label a subset as stable, experimental, or
implementation-defined in release notes. Stable behavior is behavior covered by
this specification and by the release feature matrix.

## Normative Documents

- [Notation](notation.md): EBNF notation, terminology, and normative wording.
- [Source Code Representation](source.md): source text, comments, directives,
  and lexical preprocessing.
- [Lexical Elements](lexical.md): identifiers, keywords, literals, operators,
  punctuation, and tokenization rules.
- [Declarations And Scope](declarations.md): declarations, lexical scope,
  constants, functions, imports, and module-level bindings.
- [Values And Types](values.md): dynamic values, truthiness, numbers, strings,
  tables, functions, channels, coroutines, and host values.
- [Expressions](expressions.md): operands, operators, calls, indexing, member
  selection, literals, multi-return adjustment, and evaluation order.
- [Statements](statements.md): blocks, conditionals, loops, select, go, defer,
  labels, goto, return, break, and continue.
- [Functions](functions.md): functions, closures, varargs, multi-return, tail
  behavior, and callable values.
- [Tables And Metatables](tables.md): table identity, constructors, raw access,
  sequence behavior, metatables, and metamethods.
- [Concurrency](concurrency.md): goroutine-like tasks, channels, select, sync,
  cancellation, and host scheduling boundaries.
- [AI-Native Syntax](ai-native.md): models, tools, agents, turns, messages,
  flows, budgets, output validation, providers, trace, and replay.
- [Modules And Loading](modules.md): `require`, `import "go:..."`, `leia.mod`,
  `leia.sum`, vendoring, module caches, and capabilities.
- [Errors And Diagnostics](errors.md): runtime errors, recoverable errors,
  diagnostics, protected calls, and host failure reporting.
- [Implementation Requirements](implementation.md): interpreter baseline,
  bytecode VM, JIT, optimization correctness, sandboxing, and release gates.
- [Grammar Appendix](grammar.ebnf): stable user-facing EBNF grammar.

## Compatibility Entry Point

The previous single-page overview remains available as
[language.md](language.md). New normative changes should be made in the
chaptered specification and mirrored into `language.md` only when a concise
overview is useful.

## Stability Contract

Stable behavior requires:

1. a normative section in this specification;
2. a corresponding entry in `tests/feature_matrix.json`;
3. at least one semantic or conformance gate;
4. release notes or migration notes for user-visible changes.

Experimental behavior may exist in examples, feature flags, or implementation
packages, but it must not be advertised as stable.
