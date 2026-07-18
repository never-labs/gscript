# Functions

Functions are first-class callable values. They may be named declarations or
anonymous literals.

## Parameters

Parameters are lexical bindings initialized from call arguments. Missing
arguments become `nil`; extra arguments are discarded unless the function has a
vararg parameter. The call does not expose an argument count to fixed
parameters; a fixed parameter that receives an explicit `nil` is
indistinguishable from one filled because the argument is missing.

```leia run all
func first(a, b) {
    return a, b
}

one, two := first(1, 2, 3)
missing_a, missing_b := first()
explicit_nil, after_nil := first(nil, "x")

assert(one == 1 && two == 2)                  // extra argument discarded
assert(missing_a == nil && missing_b == nil)  // missing arguments become nil
assert(explicit_nil == nil && after_nil == "x")
```

## Varargs

The parameter `...` accepts any remaining arguments. Inside the function,
`[...]` constructs a list containing those arguments. A function may have at
most one vararg parameter, and it must be the final parameter. Fixed parameters
are filled before the vararg list is formed.

```leia run all
func count(...) {
    args := [...]
    return #args
}

assert(count(1, 2, 3) == 3)
```

```leia run all
func rest(head, ...) {
    tail := [...]
    return [head, tail[1], tail[2]]
}

values := rest("a", "b", "c")
assert(values[1] == "a")
assert(values[2] == "b")
assert(values[3] == "c")
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

```leia run all
func triple() {
    return 10, 20, 30
}

a, b, c := triple() // 10, 20, 30
x, y := triple()    // 10, 20
func forward() {
    return triple()
}
first, second, third := forward()

assert(a == 10 && b == 20 && c == 30)
assert(x == 10 && y == 20)
assert(first == 10 && second == 20 && third == 30)
```

A parenthesized call is no longer in an expanding position and contributes
exactly one value.

```leia run all
func triple() {
    return 10, 20, 30
}

a, b := (triple()) // a == 10, b == nil
assert(a == 10)
assert(b == nil)
```

When a call appears before the final expression in an expression list, it
contributes exactly one value. A final call may expand.

```leia run all
func triple() {
    return 10, 20, 30
}

a, b, c, d := triple(), triple()
// a == 10; b == 10; c == 20; d == 30
assert(a == 10)
assert(b == 10)
assert(c == 20)
assert(d == 30)
```

Function-call arguments, including calls to host functions, and table
constructors use the same expression-list rule: non-final calls contribute one
value; final calls expand. Use `spread(call())` to expand a call in a non-final
position.

```leia run all
func triple() {
    return 10, 20, 30
}

func pack(...) { return table.pack(...) }

plain := pack(triple(), "x")              // receives 10, "x"
expanded := pack(spread(triple()), "x")   // receives 10, 20, 30, "x"
host_expanded := table.pack(triple())     // host calls use the same rule
list := [triple()]                        // [10, 20, 30]
single := [(triple())]                    // [10]

assert(plain[1] == 10 && plain[2] == "x")
assert(expanded[1] == 10 && expanded[2] == 20 && expanded[3] == 30 && expanded[4] == "x")
assert(host_expanded[1] == 10 && host_expanded[2] == 20 && host_expanded[3] == 30)
assert(list[1] == 10 && list[2] == 20 && list[3] == 30)
assert(single[1] == 10 && single[2] == nil)
```

If a function has fixed parameters followed by `...`, fixed parameters are
filled first and only the remaining adjusted argument values are captured by the
vararg binding. When the final argument expression expands, all of its remaining
results may enter `...`.

```leia run all
func triple() {
    return 10, 20, 30
}

func rest(a, ...) {
    return a, [...]
}

first, tail := rest(triple()) // first == 10; tail == [20, 30]
assert(first == 10)
assert(tail[1] == 20)
assert(tail[2] == 30)
```

Multi-return adjustment is not transitive through variables or table fields. A
variable holding the first result of a call is an ordinary single value.

```leia run all
func triple() {
    return 10, 20, 30
}

v := triple() // v == 10
a, b := v     // a == 10; b == nil
assert(v == 10)
assert(a == 10)
assert(b == nil)
```

```leia run all
func triple() {
    return 10, 20, 30
}

a, b, c := triple()
assert(a == 10)
assert(b == 20)
assert(c == 30)

func sum3(x, y, z) {
    return x + y + z
}

assert(sum3(triple()) == 60)
assert(sum3(triple(), 1, 2) == 13)

expanded := [1, spread(triple()), 40]
assert(expanded[1] == 1)
assert(expanded[2] == 10)
assert(expanded[3] == 20)
assert(expanded[4] == 30)
assert(expanded[5] == 40)

v := triple()
x, y := v
assert(x == 10)
assert(y == nil)
```

## Closures

Closures capture lexical variables by reference. Mutating a captured variable is
visible to all closures that share the binding. Each evaluation of a function
literal creates a distinct function value. Returning or assigning an existing
function value preserves that value's identity and captured environment.

```leia run all
func counter() {
    n := 0
    inc := func() {
        n = n + 1
        return n
    }
    get := func() {
        return n
    }
    return inc, get
}

next, current := counter()
other, other_current := counter()
same_next := next

assert(next() == 1)
assert(current() == 1)        // sibling closure sees the same binding
assert(next() == 2)
assert(current() == 2)
assert(other() == 1)          // a separate call has separate captures
assert(other_current() == 1)
assert(next == same_next)
assert(next != other)
```

## Host Functions

Script functions and host functions share the same script-visible call result
model: argument adjustment, result adjustment, protected-call behavior, and
vararg collection are defined at the script boundary rather than by the
implementation language of the callee. Host functions may return structured
recoverable errors where their module contract specifies `nil, err` behavior.
Uncaught host failures surface as ordinary runtime errors to script code.

## Tail Calls

Tail-call optimization is not part of the stable function contract. A call in
tail position has the same observable result adjustment as any other returned
call, but the specification does not promise stack reuse, frame elision,
debug-frame shape, or unbounded recursion unless a section explicitly promises
stack behavior for a feature.
