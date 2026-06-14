# Leia Language Specification Overview

This page is the compatibility overview for older links. The chaptered
normative specification starts at [index.md](index.md); new language-visible
changes should update the relevant chapter there first.

Leia's specification is intentionally separate from implementation notes:
parser, interpreter, bytecode VM, JIT, formatter, linter, sandboxing, and
embedding APIs must either implement the stable behavior or mark a feature as
experimental before exposing it to users.

Leia is a Go-embedded scripting language with Go-like syntax, dynamic values,
and Lua-compatible table and multi-return behavior where that compatibility is
useful. It is not a Lua clone: the stable language is defined here, and
Lua-derived tests are compatibility oracles rather than the source of truth.

## Specification Baseline

The hard deliverable is a written, testable semantic baseline:

1. Syntax is specified before it is promised by formatter, linter, examples, or
   public APIs.
2. Each stable feature maps to at least one semantic gate in
   `tests/feature_matrix.json`.
3. Standard-library and host capabilities have explicit safety and error
   contracts.
4. VM and JIT optimizations must preserve this document's behavior when enabled,
   disabled, or deoptimized.

## Lexical Grammar

Source files are UTF-8 text. Stable identifiers are ASCII:

```ebnf
identifier = ( "A".."Z" | "a".."z" | "_" )
             { "A".."Z" | "a".."z" | "0".."9" | "_" } ;
digit      = "0".."9" ;
```

Whitespace separates tokens but does not by itself terminate statements.
Semicolons are optional separators.

Comments:

```ebnf
line_comment  = "//" { any_char_except_newline } ;
block_comment = "/*" { any_char } "*/" ;
```

Line comments that begin with `//leia:` before the first parsed token are file
directives. They are metadata for tools, sandbox policy, build selection, and
test classification; they do not execute and do not grant capabilities by
themselves. The directive contract is documented in
[`../reference/directives/index.md`](../reference/directives/index.md).

Stable keywords:

```text
func return if else elseif for range break continue in var go chan defer const goto
true false nil
```

Numeric literals support decimal integers/floats and `0x`, `0b`, `0o` integer
forms with `_` separators. String literal forms are:

- `"..."`: escapes plus `${expr}` interpolation;
- `'...'`: escapes, no interpolation;
- `` `...` ``: Go-style raw string, no escapes and no interpolation;
- `` tag`...` ``: tagged raw dialect body, with dialect interpolation support;
- `` tag```...``` ``: fenced tagged/raw dialect body, with multiline content
  and embedded single backticks.

## Syntactic Grammar

The grammar below is a short orientation summary, not a second grammar source.
The normative syntax appendix is [`grammar.ebnf`](grammar.ebnf). Internal
parser helper rules are not public syntax.

```ebnf
program       = { separator | statement } EOF ;
separator     = ";" ;
block         = "{" { separator | statement } "}" ;

statement     = func_decl | if_stmt | for_stmt | return_stmt
              | break_stmt | continue_stmt | goto_stmt | label_stmt
              | go_stmt | defer_stmt | const_decl | simple_stmt ;

func_decl     = "func" identifier param_list block ;
param_list    = "(" [ param { "," param } [ "," vararg_param ]
                    | vararg_param ] ")" ;
param         = identifier ;
vararg_param  = "..." | identifier "..." ;

if_stmt       = "if" expr block { "elseif" expr block } [ "else" block ] ;
for_stmt      = "for" block
              | "for" expr block
              | "for" simple_stmt ";" expr ";" simple_stmt block
              | "for" identifier [ "," identifier ] ":=" "range" expr block ;

return_stmt   = "return" [ expr_list ] ;
break_stmt    = "break" ;
continue_stmt = "continue" ;
label_stmt    = identifier ":" ;
goto_stmt     = "goto" identifier ;
go_stmt       = "go" call_expr ;
defer_stmt    = "defer" call_expr ;
const_decl    = "const" identifier ( "=" | ":=" ) expr ;

simple_stmt   = assignment | compound_assignment | inc_dec_stmt
              | send_clause | call_expr | expr ;
assignment    = expr_list ( "=" | ":=" ) expr_list ;
compound_assignment = expr ( "+=" | "-=" | "*=" | "/=" ) expr ;
inc_dec_stmt  = expr ( "++" | "--" ) ;
expr_list     = expr { "," expr } ;
```

Tables, calls, member selection, indexing, anonymous functions, dense arrays,
imports, and tagged dialect forms are layered on this core grammar. Dialect
tags and blocks are the generic extension mechanism; q columnar analytics,
shell/data/web forms, spreadsheets, and AI workflows all use the same dialect
boundary.

## Tagged Dialects And AI Syntax

Leia supports tagged dialect forms. A tagged string has the shape
<code>tag`...`</code>, <code>tag!`...`</code>, <code>tag```...```</code>, or
<code>tag!```...```</code>; <code>$`...`</code> is shorthand for the shell dialect. Tagged
raw bodies may contain `${expr}` interpolation; each dialect owns encoding of
the interpolated value. Untagged raw strings do not interpolate. A tagged block
has the shape `tag { ... }` or `tag! { ... }`. The non-bang form returns the
dialect result and any structured error according to the dialect contract. The
bang form is fail-fast and raises a runtime error when the dialect cannot parse
or execute the form.

`q` is Leia's core dialect for high-performance in-memory columnar analytics.
It works with q-style vectors, dictionaries, tables, and SoA-backed query plans;
it is a dialect implementation over ordinary Leia values, not a second
language runtime.

The AI surface is an optional standard-library layer rather than a separate
execution engine or an "AI-intrinsic" language mode. It is one dialect
implementation built on the same tagged dialect mechanism as q and other DSLs.
Use `turn { ... }` or `llm.turn({...})` for model calls and `model { ... }` or
`llm.register_models({...})` for model configuration. Tools, agents, messages,
history manipulation, output validation, record/replay, tracing, and
cancellation are ordinary values and calls on the `llm`, `msg`, `history`, and
`loop` modules. This keeps scripted and embedded agents on the same runtime
path.

An agent is a callable value produced by `agent { ... }` or `llm.agent`. Its
configuration function returns the turn fields for each call. There is no
implicit flow scope: custom flow functions are ordinary lexical functions, and
all configuration values that the body needs
must be passed or closed over explicitly.

Messages are ordinary ordered tables, usually built with `llm.system`,
`llm.user`, `msg.assistant`, `msg.assistant_call`, and tool-result helpers.
Recoverable provider, budget, validation, and tool failures return structured
`nil, err` results unless the API explicitly documents a panic-style runtime
error. Host-provided model credentials and endpoints are embedding policy, not
source-level secrets.

## Core Behavioral Rules

Leia is dynamically typed. Values are nil, booleans, numbers, strings, tables,
functions, coroutines, channels, and host-backed userdata-like values represented
through tables or native functions.

Only `nil` and `false` are falsy. Numbers, including `0`, and empty strings are
truthy.

Assignment adjusts multi-return values in Lua-compatible positions. A function
call in final expression-list position may expand; elsewhere it contributes one
value unless explicitly spread by the language feature in use.

Function bodies are lexically scoped. Closures capture variables by reference to
their lexical binding, not by copying the value at declaration time.

```leia run all
func pair() {
    return "left", "right"
}

first, second, missing := pair()
assert(first == "left" && second == "right" && missing == nil)

count := 0
func bump() {
    count += 1
    return count
}
assert(bump() == 1 && bump() == 2)
```

## Stability Contract

Stable behavior is behavior covered by this spec plus tests. Experimental
behavior may exist behind examples, feature flags, or implementation packages,
but it must not be advertised as a compatibility promise.

Breaking changes require:

- an update to this specification;
- a feature matrix update;
- migration notes or release notes when user-visible behavior changes;
- tests that pin the new behavior.

## Lua-Compatible Surface

Leia intentionally keeps several Lua-facing semantics:

- table identity, metatables, raw access helpers, and 1-based sequence helpers;
- varargs and multi-return adjustment;
- `pcall`/`xpcall` style protected execution;
- string/table/math/os compatibility where it serves migration.

Compatibility is not unlimited. Binary chunks, Lua debug slot protocols,
`_ENV` mutation semantics, and exact Lua GC/finalizer behavior are not stable
Leia promises unless separately specified.

## Operator Precedence

Operators follow the current parser precedence contract:

1. postfix call/index/member;
2. unary operators;
3. multiplicative arithmetic;
4. additive arithmetic and concatenation where supported;
5. comparisons;
6. logical `&&`;
7. logical `||`;
8. assignment forms.

Parentheses override precedence. Short-circuit operators are `&&` and `||`;
they return operand values, not coerced booleans. Unary logical negation is
`!`.

## Numbers, Strings, And Stdlib

Numbers are represented as integers or floating-point values where possible.
Arithmetic may preserve integer representation when exact and fall back to float
or boxed runtime operations as needed. JIT raw integer optimizations are
implementation details and must preserve script-visible results.

Strings are byte strings. UTF-8 helpers live in the `utf8` module; byte-oriented
string APIs remain byte-indexed unless an API explicitly says otherwise.

The standard library is documented at
[`../reference/stdlib/index.md`](../reference/stdlib/index.md). Pure modules
must not perform ambient host I/O; host modules must be capability-gated by
embedders.

## Tables, Arrays, And Metatables

Tables are mutable key/value maps with optimized array and record paths. The
optimization layout is not observable except through performance.

Metatables may define arithmetic, comparison, length, indexing, and call
behavior where the runtime supports them. Raw helpers bypass metamethods by
contract. Sequence length follows Leia's stable runtime behavior; sparse-table
edge cases should be tested before being relied on as data model.

Typed arrays, matrices, vectors, and SOA data are standard-library data
structures, not separate primitive value kinds unless a separate stable spec
section explicitly promotes them.

See [`../reference/data-oriented/index.md`](../reference/data-oriented/index.md)
for dense-array literals, SoA layout, masks, and column kernels.

## Modules, Loading, And Scope

`require(name)` first resolves enabled built-in standard-library modules, then
project/module paths according to runtime module options and the active
capability policy. Module results are cached in `package.loaded`.

`import name "go:..."` is source syntax for explicit host-provided Go bindings.
The import path does not reflect arbitrary Go packages by itself: embedders must
allow the binding through the Go API, and the active capability policy may still
reject use at load time or call time. `import "path"` infers the alias from the
final path element; `import ( ... )` groups import declarations.

`leia.mod` describes a Leia module. `leia.sum` records remote or vendored module
hashes when the module toolchain is used. Go imports are explicit host bindings;
scripts do not automatically reflect arbitrary Go packages.

## VM, JIT, And Semantic Gates

The interpreter is the semantic baseline. The bytecode VM and JIT must preserve
interpreter behavior. A JIT optimization may specialize at runtime after guards,
but benchmark-specific kernels or static recognition of benchmark names are not
part of the language contract.

Semantic gates include:

```bash
go test ./tests -run 'TestFeatureMatrix|TestLanguageConformanceTranslatedCases' -count=1
go test ./internal/runtime ./internal/vm -count=1
```

## Errors And Diagnostics

Programmer errors such as wrong argument types generally raise runtime errors.
Host/resource failures should return `nil, err` or a structured result table
when the API is designed for recoverable failure.

Diagnostics should identify source locations where available. Host-facing error
messages must avoid leaking paths, environment variables, or data forbidden by
the active capability policy.

## Coroutines And Concurrency

Leia supports coroutines and Go-style concurrency constructs. `go` starts an
asynchronous task where supported by the runtime. Channels provide typed-by-use
message passing at script level.

Long-running concurrent tasks must remain cancellable through host controls or
runtime resource budgets. JIT participation is allowed only where semantic
checks and deoptimization remain correct.

See [`../reference/concurrency/index.md`](../reference/concurrency/index.md)
for channel syntax, `select`, `sync`, `context`, and embedding budgets.

## Intentional Differences From Lua

Leia differs from Lua when Go-native embedding, safety, or product clarity wins:

- Go-style `//` comments and `func` syntax are canonical.
- Host capabilities are explicit and can be sandboxed.
- `defer`, `go`, channels, tagged dialect helpers, SOA and q columnar
  analytics, and module tooling are Leia features, not Lua compatibility
  features.
- Debug and GC internals are not promised to match Lua.

## Release Readiness

Release readiness is tracked by the chaptered spec, generated reference docs,
repeatable tests, package management, security policy, and examples. The docs
tree is organized around those release surfaces.

## Change-Control Checklist

Before changing language behavior:

1. update this specification;
2. update `tests/feature_matrix.json`;
3. add or update semantic tests;
4. verify interpreter, VM, and JIT behavior;
5. update generated or reference docs if user-visible APIs changed.
