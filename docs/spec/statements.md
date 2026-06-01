# Statements

Statements control execution.

Blocks introduce lexical scope. A block is a sequence of statements enclosed in
braces. Locals declared inside the block are not visible after the closing
brace. A block evaluates its statements in source order until control transfers,
an error unwinds, or the block ends normally.

`if` evaluates its condition and executes the first matching branch. Conditions
use Leia truthiness: only `nil` and `false` are false. `elseif` branches are
tested left-to-right after earlier conditions are false. At most one branch
runs, and branch blocks have their own lexical scopes.

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

for key, value := range pairs({ a: 1, b: 2 }) {
    print(key, value)
}
```

`for { ... }` repeats until `break`, `return`, `goto`, an error, or host
cancellation exits the loop. `for condition { ... }` evaluates the condition
before every iteration using ordinary truthiness.

In `for init; condition; post { ... }`, the init statement runs once before the
first condition check. The condition is checked before each iteration. The post
statement runs after each normal iteration and after `continue`; it does not run
after `break`, `return`, `goto`, or an unwind out of the loop body.

`for key := range iterator { ... }` and
`for key, value := range iterator { ... }` iterate over a runtime iterator. For
ordinary tables, use `pairs(table)` to obtain the table iterator. Sequence-style
table entries use 1-based integer keys. Non-sequence table iteration order is
unspecified and must not be used for stable program behavior.

The range variables are fresh lexical bindings for each iteration. Closures
created inside the loop capture that iteration's bindings, not one shared loop
slot.

```leia run
items := {10, 20, 30}
sum := 0
keySum := 0

for k, v := range pairs(items) {
    keySum += k
    sum += v
}

assert(keySum == 6)
assert(sum == 60)
```

`break` exits the innermost enclosing loop. `continue` starts the next
iteration of the innermost enclosing loop. Outside a loop they are invalid.

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

`defer` evaluates the callee and arguments when the `defer` statement executes,
then invokes the deferred call later. Deferred calls run in last-in, first-out
order when the current function exits normally or unwinds through a protected
boundary.

`goto` transfers control to a label in the same function subject to lexical
scope restrictions. It must not jump into a block or over a local declaration.

`budget { ... } { ... }` applies an AI budget to the enclosed block. Public
budget dimensions are specified in [AI-Native Syntax](ai-native.md).

```leia
budget { turns: 1, tokens: 256, time: 30 } {
    result, err := answer("short task")
}
```
