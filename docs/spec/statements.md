# Statements

Statements control execution.

Blocks introduce lexical scope. A block is a sequence of statements enclosed in
braces.

`if` evaluates its condition and executes the first matching branch. Conditions
use Leia truthiness: only `nil` and `false` are false.

`for` supports indefinite loops, condition loops, C-style loops, and range
loops. Loop bodies may use `break` and `continue`.

`select` waits on channel send or receive cases. A `default` case is selected
when no communication can proceed immediately.

`go call()` starts a goroutine-like task. `defer call()` schedules cleanup to
run when the current function returns or unwinds through a protected boundary.

`return` exits the current function and returns zero or more values. Return
values use multi-return adjustment.

`goto` transfers control to a label in the same function subject to lexical
scope restrictions. It must not jump into a block or over a local declaration.

`budget { ... } { ... }` applies an AI budget to the enclosed block. Public
budget dimensions are specified in [AI-Native Syntax](ai-native.md).
