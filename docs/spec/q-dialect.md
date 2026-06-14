# q Dialect

The `q` dialect is Leia's core dialect for high-performance in-memory columnar
analytics. It provides q-style concise syntax for vectors, dictionaries, tables,
adverbs, qSQL, and columnar query plans while remaining an implementation over
ordinary Leia values and runtime helpers.

Leia does not aim to be a byte-for-byte kdb+/q clone. The stable goal is a
native Leia analytics dialect with q-like density, predictable embedding in Go,
and runtime/JIT paths that can specialize columnar shapes.

## Scope

The stable q dialect surface includes:

- scalar atoms, symbols, strings, and temporal values supported by the runtime;
- homogeneous and mixed vectors;
- dictionaries and keyed dictionaries;
- flipped tables and keyed tables;
- q-style unary and dyadic verbs implemented by the current runtime;
- adverbs and scans covered by the q conformance matrix;
- qSQL select, exec, update, delete, joins, grouping, ordering, and projection
  forms covered by the q conformance matrix;
- functional query forms that lower to the same query plan representation;
- SoA-backed query plans used by `q.query` and data-oriented libraries.

Unsupported or intentionally different kdb+/q behavior must fail explicitly or
be documented as implementation-defined. A construct is not stable merely
because a parser accepts part of its spelling.

## Values

q dialect values cross the dialect boundary as ordinary Leia values:

| q concept | Leia-visible shape |
|---|---|
| Atom | Boolean, integer, float, string, symbol, temporal, or null-like value. |
| Vector | Dense or ordinary sequence value, depending on element kind and runtime path. |
| Dictionary | Table-like key/value mapping with q symbol keys when applicable. |
| Table | Frame value with named columns and a stable schema. |
| Keyed table | Frame plus key metadata. |
| Parse tree | Ordinary list/dictionary structure accepted by functional query helpers. |

Implementations may use internal typed carriers, frames, vectors, masks, and
runtime kernels. Those carriers are implementation details unless exposed by a
documented standard-library API.

## Tagged Forms

The q dialect uses the generic tagged dialect syntax:

```text
q`1 2 3`
q```flip `sym`px!(`AAPL`MSFT;100 101.5)```
```

The fenced form is preferred for q source that contains symbol backticks,
queries, or multiline tables. The short form is useful for simple expressions
that do not contain backticks inside the body.

`dialect.eval("q", body, opts)` is equivalent to invoking the registered q
dialect implementation directly. Hosts may use it when the q source is dynamic
or when the tag spelling would be inconvenient.

```leia run all
v := q`1 2 3`
assert(v[1] == 1)
assert(v[3] == 3)

trades := dialect.eval("q", "flip `sym`px`qty!(`AAPL`MSFT`AAPL;100 101.5 100.75;10 12 8)")
leader := q.sql(trades, "select qty:sum qty, avg_px:avg px by sym from trades order by qty desc")
assert(leader[1].sym == "AAPL")
assert(leader[1].qty == 18)
assert(leader[1].avg_px == 100.375)
```

## qSQL

qSQL is part of the q dialect, not a separate language runtime. qSQL forms
lower to query plans over frame-like values. The implementation may execute a
plan with interpreter helpers, typed runtime kernels, cached shape plans, or JIT
handoff, but the visible result must be the same for supported shapes.

Stable qSQL behavior includes:

- `select` projections over table columns;
- `where` filters and typed comparison masks;
- aggregate projections such as `sum`, `avg`, `min`, `max`, and `count`;
- `by` grouping for supported key and expression shapes;
- ordering and limiting for supported table values;
- update and delete forms covered by conformance tests;
- joins, as-of joins, and keyed operations covered by conformance tests.

qSQL may also be invoked through standard helper APIs such as `q.sql(table,
query)` when a Leia value already holds the input frame.

## Functional Query Forms

Functional query forms such as `?[t;c;b;a]` and `![t;c;b;a]` are supported when
their operands can be represented as stable q values or parse-tree structures.
They lower to the same internal query plan family as qSQL.

Callable q values, host functions, and arbitrary Leia closures are not
implicitly bridgeable into functional q parse trees. Implementations must reject
unbridgeable operands with a diagnostic instead of silently falling back to a
different meaning.

## Columnar Runtime Contract

The q dialect is allowed to use specialized runtime and JIT paths. Stable
columnar optimizations include, when implemented for a shape:

- typed column loads;
- typed comparison masks;
- boolean mask combination;
- `where` index extraction;
- gather/filter operations;
- select projection;
- grouped aggregate kernels;
- join and as-of join kernels;
- schema-stable plan caching.

These optimizations are not independent semantics. A typed kernel, cached plan,
or JIT handoff must preserve:

- result values and column order;
- null and missing-value behavior;
- error messages for unsupported stable shapes where specified;
- resource-budget and capability behavior;
- fallback observability for unsupported or unoptimized shapes.

Fallback must be explainable. Runtime statistics should distinguish typed
kernel hits, cache hits, JIT handoffs, and fallback reasons so unsupported paths
can be reduced without changing q semantics.

## Compatibility

The q dialect follows q/kdb+ spelling where doing so serves concise analytics
inside Leia. Full kdb+ compatibility, IPC wire compatibility, remote execution,
and every q system command are not language goals unless a future release marks
them stable.

The q conformance matrix defines the current supported, rejected, and extended
surfaces. Stable q features must have language tests or examples and, for hot
paths, benchmark coverage.

## Conformance

Stable q behavior requires:

- a row in the q conformance matrix;
- at least one q language test or runnable example for accepted behavior;
- an explicit rejected test for intentionally unsupported q/kdb+ spelling when
  the parser could otherwise accept it ambiguously;
- benchmark coverage for public hot paths such as qSQL filters, groupings,
  joins, projections, typed comparisons, and vector/adverb pipelines;
- release notes for user-visible compatibility changes.

The q conformance matrix is descriptive of the current stable surface. It must
not be used to justify test-specific optimizations. Runtime and JIT
specialization must be driven by general shape recognition, type information,
and schema-stable plans.
