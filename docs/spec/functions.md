# Functions

Functions are first-class callable values. They may be named declarations or
anonymous literals.

Parameters are lexical bindings initialized from call arguments. Missing
arguments become `nil`; extra arguments are discarded unless the function has a
vararg parameter.

```leia
func first(a, b) {
    return a
}

first(1, 2, 3) // 1; the extra argument is discarded
first()        // nil; missing arguments become nil
```

The parameter `...` accepts any remaining arguments. Inside the function,
`{...}` constructs a table containing those arguments.

```leia
func count(...) {
    args := {...}
    return #args
}

count(1, 2, 3) // 3
```

```leia run all
func count(...) {
    args := {...}
    return #args
}

assert(count(1, 2, 3) == 3)
```

## Multiple Results

Function calls and some built-ins may produce multiple results. Leia adjusts
those results according to the syntactic position where the call appears.

The stable adjustment rule is:

1. In an expression list, every non-final expression contributes exactly one
   value. If that expression is a call, only its first result is used, or `nil`
   if it returns no values.
2. If the final expression is a call, its full result list is available to the
   enclosing assignment, return, call argument list, or table constructor.
3. The enclosing form then consumes the available values. Missing assignment
   targets or parameters receive `nil`; surplus values are discarded unless the
   enclosing form preserves them, such as `return` or vararg forwarding.

In assignment and return positions, a final call expands. Missing assignment
values become `nil`; extra assignment values are discarded.

```leia
func triple() {
    return 10, 20, 30
}

a, b, c := triple() // 10, 20, 30
x, y := triple()    // 10, 20
return triple()     // returns 10, 20, 30
```

A parenthesized call is no longer in an expanding position and contributes
exactly one value.

```leia
a, b := (triple()) // a == 10, b == nil
```

When a call appears before the final expression in an expression list, it
contributes exactly one value. A final call may expand.

```leia
a, b, c, d := triple(), triple()
// a == 10; b == 10; c == 20; d == 30
```

Function-call arguments and table constructors use the same expression-list
rule: non-final calls contribute one value; final calls expand. Use
`spread(call())` to expand a call in a non-final position.

```leia
func pack(...) { return table.pack(...) }

pack(triple(), "x")          // receives 10, "x"
pack(spread(triple()), "x")  // receives 10, 20, 30, "x"
{triple()}                   // {10, 20, 30}
{(triple())}                 // {10}
```

If a function has fixed parameters followed by `...`, fixed parameters are
filled first and only the remaining adjusted argument values are captured by the
vararg binding. When the final argument expression expands, all of its remaining
results may enter `...`.

```leia
func rest(a, ...) {
    return a, {...}
}

first, tail := rest(triple()) // first == 10; tail == {20, 30}
```

Multi-return adjustment is not transitive through variables or table fields. A
variable holding the first result of a call is an ordinary single value.

```leia
v := triple() // v == 10
a, b := v     // a == 10; b == nil
```

Closures capture lexical variables by reference. Mutating a captured variable is
visible to all closures that share the binding.

```leia
func counter() {
    n := 0
    return func() {
        n = n + 1
        return n
    }
}

next := counter()
next() // 1
next() // 2
```

```leia run all
func counter() {
    n := 0
    return func() {
        n = n + 1
        return n
    }
}

next := counter()
assert(next() == 1)
assert(next() == 2)
```

Script functions and host functions share the same script-visible call result
model. Host functions may return structured recoverable errors where their
module contract specifies `nil, err` behavior.

Tail-call optimization is an implementation detail unless a section explicitly
promises stack behavior for a feature.
