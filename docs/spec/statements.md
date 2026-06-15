# Statements

Statements control execution.

## Blocks

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

## If Statements

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

## Simple Statements

Simple statements are assignments, compound assignments, increment and
decrement statements, send statements, calls, and expression statements.
Expression statements evaluate their expression for side effects and discard
all results.

## Assignment

An assignment evaluates the right-hand side expression list left-to-right, then
evaluates any address subexpressions needed by non-variable targets, then stores
adjusted values into the targets from left to right. A target may be a visible
variable binding, a table or host-backed index expression, or a member
selection. Assignment to an invalid target raises a runtime error. `:=` creates
fresh lexical bindings in the current block for identifier targets; `=` updates
existing targets. Multi-value adjustment for the right-hand side follows the
rules in [Functions](functions.md).

```leia run all
events := {}
func mark(name, value) {
    events[#events + 1] = name
    return value
}

box := {slot: 0}
box[mark("key", "slot")] = mark("value", 7)
assert(box.slot == 7)
assert(events[1] == "value")
assert(events[2] == "key")

func pair() {
    return "left", "right"
}
a, b, c := pair()
assert(a == "left")
assert(b == "right")
assert(c == nil)
```

## Compound Assignment And Inc/Dec

Compound assignment `x += y`, `x -= y`, `x *= y`, and `x /= y` is equivalent
to evaluating the target once, reading its current value, applying the
corresponding binary operator to that value and the right-hand expression, then
storing the result back into the same target. The operator may use ordinary
primitive coercions or metamethod dispatch. `x++` and `x--` are statements
equivalent to adding or subtracting integer `1` from the target once.

```leia run all
count := 1
count += 2
count *= 3
count--
assert(count == 8)

log := {}
t := setmetatable({}, {
    __index: func(_, key) {
        log[#log + 1] = "read:" .. key
        return 10
    },
    __newindex: func(_, key, value) {
        log[#log + 1] = "write:" .. key .. "=" .. value
    },
})
t.score += 5
assert(log[1] == "read:score")
assert(log[2] == "write:score=15")
```

```leia fail all
1 = 2
```

```leia fail all
name := "x"
name++
```

## Send Statements

`ch <- value` as a statement sends `value` to channel `ch`. Send blocking,
close, ordering, and cancellation semantics are specified in
[Concurrency](concurrency.md).

```leia run all
ch := make(chan, 1)
ch <- "sent"
assert(<-ch == "sent")
```

## For Statements

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

## Range Loops

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
items := [10, 20, 30]
sum := 0
keySum := 0

for k, v := range pairs(items) {
    keySum += k
    sum += v
}

assert(keySum == 6)
assert(sum == 60)
```

```leia run all
calls := {}

for _, value := range pairs([10, 20, 30]) {
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

## Break And Continue

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

## Select Statements

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

## Go And Defer Statements

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

## Return Statements

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

## Deferred Calls

`defer` evaluates the callee and arguments when the `defer` statement executes,
then invokes the deferred call later. Deferred calls run in last-in, first-out
order when the current function exits normally, returns explicitly, or unwinds
through a protected boundary. A deferred call cannot change the already
computed return values unless it mutates an identity-bearing value reachable
from those return values.

The evaluated deferred call is a call plus its already adjusted argument
values. Later assignment to a variable that was used as an argument does not
change that deferred argument value. A deferred closure with no argument,
however, still closes over lexical bindings in the ordinary way; when it runs,
it observes the current value of those captured bindings.

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

```leia run all
events := {}
func record(value) {
    events[#events + 1] = value
}

func scoped() {
    value := "initial"
    defer record(value) // argument value captured now
    defer func() {
        record("closure:" .. value) // lexical binding read when defer runs
    }()
    value = "changed"
}

scoped()
assert(events[1] == "closure:changed")
assert(events[2] == "initial")
```

## Labels And Goto

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

## Budget Control

Leia does not define a standalone `budget { ... } { ... }` statement. Budget
control is expressed through ordinary values at the boundary that consumes the
budget:

- Go embedding options enforce VM, native-call, host-result, module, and
  concurrency budgets.
- AI `turn`, `agent`, and loop option tables may carry AI budget fields.
- `leia evaluate` exposes `eval.budget(table)` for per-case evaluation gates.

Because budgets are ordinary option tables rather than a statement form, their
scope and error behavior are defined by the host API, AI runtime, or evaluation
harness that receives the table.
