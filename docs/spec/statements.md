# Statements

Statements control execution.

Blocks introduce lexical scope. A block is a sequence of statements enclosed in
braces.

`if` evaluates its condition and executes the first matching branch. Conditions
use Leia truthiness: only `nil` and `false` are false.

```leia
if 0 {
    print("zero is truthy")
}

if nil {
    print("not reached")
} else {
    print("nil is false")
}
```

`for` supports indefinite loops, condition loops, C-style loops, and range
loops. Loop bodies may use `break` and `continue`.

```leia
sum := 0
for i := 1; i <= 3; i++ {
    sum += i
}

for key, value := range { a: 1, b: 2 } {
    print(key, value)
}
```

`select` waits on channel send or receive cases. A `default` case is selected
when no communication can proceed immediately.

```leia
select {
case value := <-ch:
    print(value)
default:
    print("nothing ready")
}
```

`go call()` starts a goroutine-like task. `defer call()` schedules cleanup to
run when the current function returns or unwinds through a protected boundary.

```leia
func work(ch) {
    defer print("done")
    ch <- "ready"
}

go work(ch)
```

`return` exits the current function and returns zero or more values. Return
values use multi-return adjustment.

`goto` transfers control to a label in the same function subject to lexical
scope restrictions. It must not jump into a block or over a local declaration.

`budget { ... } { ... }` applies an AI budget to the enclosed block. Public
budget dimensions are specified in [AI-Native Syntax](ai-native.md).

```leia
budget { turns: 1, tokens: 256, time: 30 } {
    result, err := answer("short task")
}
```
