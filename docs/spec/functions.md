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

Script functions and host functions share the same script-visible call result
model. Host functions may return structured recoverable errors where their
module contract specifies `nil, err` behavior.

Tail-call optimization is an implementation detail unless a section explicitly
promises stack behavior for a feature.
