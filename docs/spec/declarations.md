# Declarations And Scope

Leia uses lexical scope. A declaration creates a binding for an identifier in
the innermost enclosing lexical block, or in module scope when the declaration
appears at top level.

For `:=`, `const`, `func`, and `import`, the declaration point is the end of
the declaration statement. The new binding is visible after that point through
the end of the block that owns it, including nested blocks and function
literals created in that region. The initializer or right-hand side of a
declaration is evaluated before the new binding is visible, so names in that
expression resolve exactly as they would immediately before the declaration.
Module-scope declarations are visible to later module-scope code and to nested
blocks in the same module.

```leia run all
x := 10

if true {
    x := x + 1 // RHS uses the outer x; the inner x starts after this statement
    assert(x == 11)
}

assert(x == 10)
```

An inner declaration may shadow an outer binding with the same name. While the
inner binding is in scope, unqualified uses resolve to the inner binding. The
outer binding remains alive if it is captured or otherwise still reachable, but
it is not directly addressable by that name until the inner scope ends.

## Variables

Assignment with `:=` introduces local bindings. Assignment with `=` updates
existing targets. Multiple assignment adjusts values according to the
multi-return rules in [Expressions](expressions.md).

```leia run all
x := 1
x = x + 1

if true {
    x := "inner"
    assert(x == "inner")
}

assert(x == 2)
```

The inner `x` shadows the outer `x` only inside the block.

`:=` creates bindings in the current lexical block. Reusing a name that is
already declared in an outer block creates a new shadowing binding; it does not
update the outer binding. Use `=` to update an existing visible binding.

```leia run all
value := 1

if true {
    value := 2 // new binding in this block
    value = 3  // updates the inner binding
    assert(value == 3)
}

assert(value == 1)
```

Same-block redeclaration is reserved by the v1.0 contract: portable programs
must not declare the same identifier twice in one lexical block unless a later
spec section explicitly permits a narrower form. The current implementation
accepts some same-block redeclarations, including repeated `:=` and repeated
`func` declarations, but interpreter and VM capture behavior is not part of the
stable contract for those programs. Implementations may reject same-block
redeclarations in stable mode.

## Constants

```ebnf
const_decl = "const" identifier ( "=" | ":=" ) expr ;
```

A const binding may not be rebound. Const does not freeze the internals of a
mutable value such as a table.

```leia run all
const limit := 10
// limit = 11       // invalid: const binding

const settings := { retries: 3 }
settings.retries = 4 // valid: table contents remain mutable
assert(limit == 10)
assert(settings.retries == 4)
```

Const visibility and shadowing follow the ordinary lexical binding rules. An
inner declaration may shadow an outer const, but code that resolves to the const
binding may not assign to that binding with `=`, compound assignment, or
increment/decrement forms.

```leia fail all
const limit := 10
limit = 11
```

The read-only property belongs to the binding, not to the identifier spelling.
A shadowing declaration creates a separate binding.

```leia run all
const n := 1

if true {
    n := 2
    assert(n == 2)
}

assert(n == 1)
```

## Functions

```ebnf
func_decl = "func" identifier param_list block ;
```

A function declaration binds the function value to the declared name in the
enclosing scope. Function bodies capture lexical bindings by reference.

```leia run all
func add(a, b) {
    return a + b
}

total := 0
func bump() {
    total = total + 1
    return total
}

assert(add(2, 3) == 5)
assert(bump() == 1)
assert(bump() == 2)
```

The function declaration's binding is visible to code after the declaration
statement. The function body is evaluated later, in an environment where the
function name resolves to that binding, so the body may refer to the function
for recursion. Parameters are local bindings in the function body. They shadow
outer bindings with the same names for the entire body.

```leia run all
func factorial(n) {
    if n <= 1 {
        return 1
    }
    return n * factorial(n - 1)
}

assert(factorial(5) == 120)
```

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

The import alias is a lexical binding in the declaration's scope. It is visible
only after the import declaration statement completes, and it may be
shadowed by an inner declaration. Portable programs must avoid same-block alias
conflicts with any other declaration.

An import path names a capability provided by the module loader or embedder.
The command-line runner rejects unallowlisted Go imports.

```leia fail all
import "go:math" as math
```

## Labels

Labels are declared with `name:`. Label names live in a function-level label
namespace that is separate from lexical value bindings. A label may have the
same spelling as a variable, constant, function, parameter, or import alias.
Labels do not shadow lexical bindings, and lexical bindings do not shadow
labels. Within a function, each label name may be declared at most once.

A `goto` must not jump into a deeper lexical scope or over a local declaration.

```leia run all
i := 0
again:
i = i + 1
if i < 3 {
    goto again
}
assert(i == 3)
```

```leia run all
done:
done := true
assert(done)
```

```leia fail all
again:
again:
```

```leia fail all
goto inner
if true {
    x := 1
    inner:
}
```

## AI Declarations

`models`, `tool`, `agent defaults`, and named `agent` declarations are
module-level or block-level declarations according to their source position.
Their detailed semantics are specified in [AI-Native Syntax](ai-native.md).
