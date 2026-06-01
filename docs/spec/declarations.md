# Declarations And Scope

Leia uses lexical scope. A declaration binds an identifier in the innermost
enclosing block or module scope according to the declaration form.

A binding is visible from its declaration point through the end of the block
that owns it, including nested blocks and function literals created in that
region. A use before the declaration point does not see the later binding.
Module-scope declarations are visible to later module-scope code and to nested
blocks in the same module.

An inner declaration may shadow an outer binding with the same name. While the
inner binding is in scope, unqualified uses resolve to the inner binding. The
outer binding remains alive if it is captured or otherwise still reachable, but
it is not directly addressable by that name until the inner scope ends.

## Variables

Assignment with `:=` introduces local bindings. Assignment with `=` updates
existing targets. Multiple assignment adjusts values according to the
multi-return rules in [Expressions](expressions.md).

```leia
x := 1
x = x + 1

if true {
    x := "inner"
    print(x) // inner
}

print(x) // 2
```

The inner `x` shadows the outer `x` only inside the block.

`:=` creates bindings in the current lexical block. Reusing a name that is
already declared in an outer block creates a new shadowing binding; it does not
update the outer binding. Use `=` to update an existing visible binding.

```leia
value := 1

{
    value := 2 // new binding in this block
    value = 3  // updates the inner binding
}

// value is still 1 here
```

Same-block redeclaration is reserved by the v1.0 contract: portable programs
must not declare the same identifier twice in one lexical block unless a later
spec section explicitly permits a narrower form. A declaration may not shadow a
label because labels use a separate function-level namespace.

## Constants

```ebnf
const_decl = "const" identifier ( "=" | ":=" ) expr ;
```

A const binding may not be rebound. Const does not freeze the internals of a
mutable value such as a table.

```leia
const limit := 10
// limit = 11       // invalid: const binding

const settings := { retries: 3 }
settings.retries = 4 // valid: table contents remain mutable
```

Const visibility and shadowing follow the ordinary lexical binding rules. An
inner declaration may shadow an outer const, but code that resolves to the const
binding may not assign to that binding with `=`, compound assignment, or
increment/decrement forms.

## Functions

```ebnf
func_decl = "func" identifier param_list block ;
```

A function declaration binds the function value to the declared name in the
enclosing scope. Function bodies capture lexical bindings by reference.

```leia
func add(a, b) {
    return a + b
}

total := 0
func bump() {
    total = total + 1
    return total
}
```

The function name is bound in the enclosing scope before the function body is
evaluated, so the body may refer to the function for recursion. Parameters are
local bindings in the function body. They shadow outer bindings with the same
names for the entire body.

Function declarations follow the same same-block redeclaration portability rule
as other declarations. A nested function declaration may shadow an outer
function or variable name in its own block.

## Imports

```ebnf
import_decl = "import" string_lit "as" identifier ;
```

`import "go:..." as name` declares an explicit host-provided Go binding. The
source path does not reflect arbitrary Go packages by itself; the embedder must
provide an allowlisted binding through the Go API.

The import alias is a lexical binding in the declaration's scope. It may be
shadowed by an inner declaration. Portable programs must avoid same-block alias
conflicts.

## Labels

Labels are declared with `::name::`. Label names live in a function-level label
namespace. A `goto` must not jump into a deeper lexical scope or over a local
declaration.

```leia
i := 0
::again::
i = i + 1
if i < 3 {
    goto again
}
```

## AI Declarations

`models`, `tool`, `agent defaults`, and named `agent` declarations are
module-level or block-level declarations according to their source position.
Their detailed semantics are specified in [AI-Native Syntax](ai-native.md).
