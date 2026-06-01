# Leia Language Specification

This document is the normative language contract for Leia. It is intentionally
separate from implementation notes: parser, interpreter, bytecode VM, JIT,
formatter, linter, sandboxing, and embedding APIs must either implement this
behavior or mark a feature as experimental before exposing it to users.

Leia uses Go-flavored syntax with dynamic values and Lua-compatible table and
multi-return behavior where that compatibility is useful. It is not a Lua clone:
the stable language is defined here, and Lua-derived tests are compatibility
oracles rather than the source of truth.

## Phase 0 Hard Deliverables

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

Stable keywords:

```text
func return if else elseif for range break continue in var go chan defer const goto
true false nil
```

Numeric literals support decimal integers/floats and `0x`, `0b`, `0o` integer
forms with `_` separators. String literals support quoted strings with escapes
and raw backtick strings.

## Syntactic Grammar

The grammar below is the short user-facing contract. The fuller syntax appendix
is maintained in [`grammar.ebnf`](grammar.ebnf). Internal parser helper rules
are not public syntax.

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
label_stmt    = "::" identifier "::" ;
goto_stmt     = "goto" identifier ;
go_stmt       = "go" call_expr ;
defer_stmt    = "defer" call_expr ;
const_decl    = "const" identifier "=" expr ;

simple_stmt   = assignment | expr ;
assignment    = expr_list ( "=" | ":=" | "+=" | "-=" | "*=" | "/=" | "%=" ) expr_list ;
expr_list     = expr { "," expr } ;
```

Tables, calls, member selection, indexing, anonymous functions, agent
expressions, tool declarations, and message blocks are expressions or
declarations layered on this core grammar. AI-native syntax desugars to the
`llm`, `msg`, `history`, and `loop` standard-library modules.

## AI-Native Syntax

AI-native syntax is part of the language surface, but it is deliberately a
desugaring layer over the standard library rather than a separate execution
engine. The parser accepts `models`, `tool`, `agent defaults`, named `agent`
declarations, anonymous `agent` expressions, `turn` expressions, `messages`
blocks, `flow` blocks, and `budget` blocks as described in
[`grammar.ebnf`](grammar.ebnf).

The stable lowering target is the `llm`, `msg`, `history`, and `loop` module
family. That keeps scripted and embedded agents on the same runtime path:
provider selection, tool dispatch, output validation, record/replay, tracing,
and cancellation must behave the same whether the user writes AI-native syntax
or calls the library directly.

An `agent` is a callable value. Its configuration fields are defaults for the
turns executed by that agent: explicit fields on a `turn` expression override
agent configuration, and agent configuration overrides process or host defaults.
A `flow` block provides the agent body when the default one-turn behavior is not
enough. The flow body is lexical code; the implementation may inject documented
bindings for the agent configuration, but those bindings must be specified and
testable before they are treated as stable.

`messages { ... }` constructs ordered message tables. It is intended for common
system/user/assistant/tool histories; advanced or computed histories may still
use the message helper modules directly. `tool` declarations produce tool values
with a callable body and optional comment-derived metadata. Agents may be used
as tools when the runtime can derive a stable tool schema from the agent's
configuration and output contract.

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
6. logical `and`;
7. logical `or`;
8. assignment forms.

Parentheses override precedence. Short-circuit operators return operand values,
not coerced booleans.

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
structures, not separate primitive value kinds unless a future spec revision
promotes them.

See [`../reference/data-oriented/index.md`](../reference/data-oriented/index.md)
for dense-array literals, SoA layout, masks, and column kernels.

## Modules, Loading, And Scope

`require(name)` first resolves enabled built-in standard-library modules, then
project/module paths according to runtime module options and the active
capability policy. Module results are cached in `package.loaded`.

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
- `defer`, `go`, channels, AI-native agents, SOA, and module tooling are Leia
  features, not Lua compatibility features.
- Debug and GC internals are not promised to match Lua.

## Production Roadmap

Production readiness requires a stable spec, generated reference docs,
repeatable tests, package management, security policy, and examples. The current
docs tree is organized around those release surfaces.

## Change-Control Checklist

Before changing language behavior:

1. update this specification;
2. update `tests/feature_matrix.json`;
3. add or update semantic tests;
4. verify interpreter, VM, and JIT behavior;
5. update generated or reference docs if user-visible APIs changed.
