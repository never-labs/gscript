# Statements

Statements control execution.

Blocks introduce lexical scope. A block is a sequence of statements enclosed in
braces. Locals declared inside the block are not visible after the closing
brace. A block evaluates its statements in source order until control transfers,
an error unwinds, or the block ends normally.

```leia fail all
if true {
    hidden := 1
}
assert(hidden == 1)
```

`if` evaluates its condition and executes the first matching branch. Conditions
use Leia truthiness: only `nil` and `false` are false. `elseif` branches are
tested left-to-right after earlier conditions are false. At most one branch
runs, and branch blocks have their own lexical scopes.

```leia run all
seen := {}
if 0 {
    seen.zero = true
}

if nil {
    seen.nilBranch = true
} else {
    seen.elseBranch = true
}

assert(seen.zero)
assert(seen.elseBranch)
assert(seen.nilBranch == nil)
```

`for` supports indefinite loops, condition loops, C-style loops, and range
loops. Loop bodies may use `break` and `continue`.

```leia run all
sum := 0
for i := 1; i <= 3; i++ {
    sum += i
}
assert(sum == 6)

valueSum := 0
for key, value := range pairs({ a: 1, b: 2 }) {
    assert(#key == 1)
    valueSum += value
}
assert(valueSum == 3)
```

`for { ... }` repeats until `break`, `return`, `goto`, an error, or host
cancellation exits the loop. `for condition { ... }` evaluates the condition
before every iteration using ordinary truthiness.

In `for init; condition; post { ... }`, the init statement runs once before the
first condition check. The condition is checked before each iteration. The post
statement runs after each normal iteration and after `continue`; it does not run
after `break`, `return`, `goto`, or an unwind out of the loop body.

```leia run all
postCount := 0
bodyCount := 0

for i := 0; i < 4; postCount++ {
    if i == 1 {
        i++
        continue
    }
    if i == 3 {
        break
    }
    bodyCount += 1
    i++
}

assert(postCount == 3)
assert(bodyCount == 2)
```

`for key := range expr { ... }` and
`for key, value := range expr { ... }` evaluate `expr` once, then iterate
according to the resulting value category. The range variables are fresh lexical
bindings for each iteration. Closures created inside the loop capture that
iteration's bindings, not one shared loop slot.

The v1.0 range algorithm is:

1. Evaluate `expr` once before the loop begins.
2. If the value is a table, repeatedly call the table's next-entry operation.
   With one range variable, bind the key. With two range variables, bind the key
   and value. Sequence-style table entries use 1-based integer keys.
   Non-sequence table iteration order is unspecified and must not be used for
   stable program behavior.
3. If the value is a function, call it with no arguments before each iteration.
   If the call returns no values or first returns `nil`, stop. Bind the first
   result to the first range variable. Portable programs should not depend on a
   second range variable for function iterators in v1.0; return a table or tuple
   value when an iterator needs to carry multiple fields.
4. If the value is a channel, receive until the channel is closed and drained.
   With one range variable, bind each received value. With two range variables,
   the second variable is currently not assigned by the stable channel range
   contract and portable programs should use one variable.
5. Any other value raises a runtime error.

`break` exits the loop. `continue` skips to the next iteration and causes the
range source to be advanced again according to the same algorithm. A `return`
from the body exits the enclosing function.

```leia run all
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

```leia run
calls := {}

for _, value := range pairs({10, 20, 30}) {
    calls[#calls + 1] = func() {
        return value
    }
}

assert(calls[1]() == 10)
assert(calls[2]() == 20)
assert(calls[3]() == 30)
```

```leia run all
func counter(limit) {
    n := 0
    return func() {
        n = n + 1
        if n > limit {
            return nil
        }
        return {value: n, square: n * n}
    }
}

sum := 0
for pair := range counter(3) {
    sum += pair.value + pair.square
}
assert(sum == 20)

ch := make(chan, 2)
ch <- "a"
ch <- "b"
close(ch)

text := ""
for value := range ch {
    text = text .. value
}
assert(text == "ab")

ok := pcall(func() {
    for _ := range 123 {
    }
})
assert(!ok)
```

`break` exits the innermost enclosing loop. `continue` starts the next
iteration of the innermost enclosing loop. Outside a loop they are invalid.

```leia run all
values := {}
for i := 1; i <= 6; i++ {
    if i % 2 == 0 {
        continue
    }
    if i > 5 {
        break
    }
    values[#values + 1] = i
}

assert(#values == 3)
assert(values[1] == 1)
assert(values[2] == 3)
assert(values[3] == 5)
```

`select` waits on channel send or receive cases. Its full communication
semantics are specified in [Concurrency](concurrency.md). Statement-level
rules are:

1. receive-case bindings are scoped to the selected case body;
2. a send case evaluates its channel and value expressions before attempting the
   selected send;
3. a `default` case is selected immediately when no communication can proceed;
4. a `select` with no cases raises a runtime error.

```leia run all
ch := make(chan, 1)
observed := ""
select {
case value := <-ch:
    observed = "recv:" .. value
default:
    observed = "default"
}
assert(observed == "default")

ch <- "ready"
select {
case value := <-ch:
    observed = "recv:" .. value
default:
    observed = "default"
}
assert(observed == "recv:ready")
```

`go call()` starts a goroutine-like task. `defer call()` schedules cleanup to
run when the current function returns or unwinds through a protected boundary.

```leia run all
func work(ready, done) {
    defer func() { done <- "done" }()
    ready <- "ready"
}

ready := make(chan, 1)
done := make(chan, 1)
go work(ready, done)
assert(<-ready == "ready")
assert(<-done == "done")
```

`return` exits the current function and returns zero or more values. Return
values use the multi-return adjustment rules in [Functions](functions.md).
At module top level, `return` stops execution of the current chunk or module and
produces the module result used by loaders such as `require`.

```leia run all
func pair() {
    return "left", "right"
}

func forward() {
    return pair()
}

a, b := forward()
assert(a == "left")
assert(b == "right")
```

`defer` evaluates the callee and arguments when the `defer` statement executes,
then invokes the deferred call later. Deferred calls run in last-in, first-out
order when the current function exits normally, returns explicitly, or unwinds
through a protected boundary. A deferred call cannot change the already
computed return values unless it mutates an identity-bearing value reachable
from those return values.

```leia run all
events := {}
func scoped() {
    defer func() { events[#events + 1] = "last" }()
    defer func() { events[#events + 1] = "first" }()
    events[#events + 1] = "body"
}

scoped()
assert(events[1] == "body")
assert(events[2] == "first")
assert(events[3] == "last")
```

Labels are declared as `name:` statements. Label names live in a function-level
namespace separate from ordinary lexical variables. `goto name` transfers
control to a label in the same function. It must not jump into a deeper lexical
scope, into a loop body from outside that loop, or over a local declaration that
would be in scope at the target. It may jump forward or backward within the same
scope when doing so does not bypass such declarations.

```leia run all
i := 0
again:
i = i + 1
if i < 3 {
    goto again
}
assert(i == 3)
```

```leia fail all
goto inside
if true {
    inside:
    print("unreachable")
}
```

`budget { ... } { ... }` applies an AI budget to the enclosed block. Public
budget dimensions are specified in [AI-Native Syntax](ai-native.md).

Non-executable AI budget sketch:

```text
budget { turns: 1, tokens: 256, time: 30 } {
    result, err := answer("short task")
}
```
