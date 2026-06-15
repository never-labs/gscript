# Tagged Dialects

Tagged dialects are Leia's generic DSL extension mechanism. They let a source
file embed domain syntax while keeping the language core, evaluator, VM, JIT,
security model, and Go embedding API small and uniform.

Dialect syntax is part of the language. Dialect semantics are supplied by a
registered dialect implementation. A dialect implementation must return
ordinary Leia values, `(value, err)` result pairs, or runtime errors according
to the contract described in this chapter.

## Forms

A tagged raw expression has one of these forms:

```text
tag`body`
tag!`body`
tag```body```
tag!```body```
```

The fenced forms may span lines and may contain single backticks inside the
body. The short raw form cannot contain a backtick inside the body. `$` is the
standard shorthand tag for the shell dialect.

A tagged block has one of these forms:

```text
tag { fields }
tag! { fields }
```

Tagged blocks use normal Leia block/table syntax at the outer boundary. The
dialect owns the meaning of the fields inside its registered block contract.

The q dialect also accepts a raw source block:

```text
q {
    select avg price by sym from trades
}
q! {
    +/1 2 3
}
```

In this form the text inside the braces is q source, not Leia fields. It may
span lines. The outer lexer matches balanced braces and skips braces inside
quoted strings and comments. Raw q blocks support `${expr}` interpolation with
the same binding semantics as tagged raw strings:

```text
q`sum ${xs}`
q {
    sum ${xs}
}
```

## Interpolation

Untagged raw strings do not interpolate. Tagged raw strings and fenced tagged
raw strings may contain `${expr}` interpolation segments.

For each interpolation segment, Leia evaluates `expr` in the surrounding
lexical scope before the dialect receives the body. The dialect receives the raw
body plus the evaluated values. The dialect decides whether an interpolated
value is escaped as text, encoded as JSON, bound as a parameter, converted to a
q value, or rejected.

Literal source text and interpolated values are distinct at the dialect
boundary. Source text is preserved byte-for-byte; only `${expr}` results pass
through the dialect's value encoder. This lets a dialect safely treat a tagged
body such as `sum ${xs}` as raw dialect source `sum ` plus an encoded Leia
value, instead of forcing every value through generic string conversion.

Interpolation must preserve normal Leia evaluation order and error behavior.
If evaluating an interpolation expression fails, the dialect implementation is
not invoked.

```leia run all
name := "Ada"
profile := json`{"name": "${name}", "score": ${40 + 2}}`
assert(profile.name == "Ada")
assert(profile.score == 42)
```

## Bang Form

The non-bang form returns according to the dialect's ordinary result contract.
For dialects with recoverable parse or execution errors, this usually means a
result plus a structured error value.

The bang form is fail-fast. When the dialect reports an error that is
recoverable in the non-bang form, the bang form raises a runtime error instead.
Successful bang and non-bang evaluation must return equivalent result values.

## Runtime Boundary

A dialect boundary never creates a second language runtime. Dialects do not get
private lexical scope, hidden prompt memory, ambient host capabilities, or
separate scheduling semantics. They execute through registered runtime helpers,
host callbacks, and capability checks.

Dialect implementations must obey:

- the active host capability policy;
- resource budgets and cancellation;
- module and import policy;
- interpreter-visible error behavior;
- VM and JIT equivalence requirements.

Optimizations may recognize stable dialect shapes, but optimized execution must
preserve the same visible result, error, capability, and budget behavior as the
interpreter path.

## Registration

Built-in dialects are installed by the standard runtime configuration. Embedders
may register host-defined dialects through the Go API or through standard
library helpers when those helpers are enabled.

Registration defines:

- the tag name and aliases;
- whether raw expressions are accepted;
- whether block forms are accepted;
- the required capabilities;
- the result and error shape;
- whether bang fail-fast behavior is supported.

Source syntax alone does not import or enable a dialect. A source tag such as
`q`, `json`, `sh`, `turn`, or `agent` selects a registered implementation; if no
implementation is installed, evaluation fails.

## Standard Dialect Families

Leia's standard dialect families include:

- `q`, the core high-performance in-memory columnar analytics dialect;
- data and protocol dialects such as `json`, `yaml`, `csv`, `urlquery`, and
  `headers`;
- host automation dialects such as `sh`, `$`, `cmd`, `glob`, and `env`;
- document and tabular dialects such as `xlsx` and `excel`;
- web/server dialects such as `serve`;
- optional LLM dialects such as `model`, `tool`, `turn`, and `agent`.

The standard dialect list is discoverable through the runtime capabilities
surface. Each dialect family may have its own reference and, for stable core
dialects, its own normative specification chapter.

## Conformance

Stable dialect syntax requires parser coverage, interpreter coverage, and at
least one semantic gate that exercises accepted and rejected forms. A stable
dialect family must document its result shape and error behavior before it is
advertised as public surface.

Adding a new standard dialect family requires:

- lexer or parser coverage for any new syntax;
- runtime tests for raw, bang, interpolation, and block forms that the dialect
  accepts;
- capability-boundary tests when the dialect reaches host resources;
- reference documentation for user-facing result shapes;
- a feature-matrix entry when the dialect is part of the stable release
  surface.
