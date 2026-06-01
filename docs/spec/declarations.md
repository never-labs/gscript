# Declarations And Scope

Leia uses lexical scope. A declaration binds an identifier in the innermost
enclosing block or module scope according to the declaration form.

## Variables

Assignment with `:=` introduces local bindings. Assignment with `=` updates
existing targets. Multiple assignment adjusts values according to the
multi-return rules in [Expressions](expressions.md).

## Constants

```ebnf
const_decl = "const" identifier ( "=" | ":=" ) expr ;
```

A const binding may not be rebound. Const does not freeze the internals of a
mutable value such as a table.

## Functions

```ebnf
func_decl = "func" identifier param_list block ;
```

A function declaration binds the function value to the declared name in the
enclosing scope. Function bodies capture lexical bindings by reference.

## Imports

```ebnf
import_decl = "import" string_lit "as" identifier ;
```

`import "go:..." as name` declares an explicit host-provided Go binding. The
source path does not reflect arbitrary Go packages by itself; the embedder must
provide an allowlisted binding through the Go API.

## Labels

Labels are declared with `::name::`. Label names live in a function-level label
namespace. A `goto` must not jump into a deeper lexical scope or over a local
declaration.

## AI Declarations

`models`, `tool`, `agent defaults`, and named `agent` declarations are
module-level or block-level declarations according to their source position.
Their detailed semantics are specified in [AI-Native Syntax](ai-native.md).
