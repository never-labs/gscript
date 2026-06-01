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

In assignment and return positions, a final call expands to as many values as
needed. Missing values become `nil`; extra values are discarded.

```leia
func triple() {
    return 10, 20, 30
}

a, b, c := triple() // 10, 20, 30
x, y := triple()    // 10, 20
return triple()     // returns 10, 20, 30
```

A parenthesized call contributes exactly one value.

```leia
a, b := (triple()) // a == 10, b == nil
```

When a call appears before the final expression in an expression list, it
contributes exactly one value. A final call may expand.

```leia
a, b, c, d := triple(), triple()
// a == 10; b == 10; c == 20; d == 30
```

Function-call arguments and table constructors use the same adjustment rule:
non-final calls contribute one value; final calls expand. Use `spread(call())`
to expand a call in a non-final position.

```leia
func pack(...) { return table.pack(...) }

pack(triple(), "x")          // receives 10, "x"
pack(spread(triple()), "x")  // receives 10, 20, 30, "x"
{triple()}                   // {10, 20, 30}
{(triple())}                 // {10}
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
