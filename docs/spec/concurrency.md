# Concurrency

Leia supports Go-style concurrent tasks and channels. The stable contract is
intentionally small: tasks run independently, channels are the synchronization
primitive, and host policies control cancellation and resource limits.

## Tasks

A `go` statement starts a concurrent task:

```ebnf
go_stmt = "go" call_expr ;
```

The callee and all arguments are evaluated in the current task before the new
task is started. The new task then calls the evaluated function or method with
those evaluated arguments. Later changes to variables used to compute the callee
or arguments do not change that call, though captured tables and other reference
values remain shared values.

```leia run all
func worker(input, output) {
    value := <-input
    output <- value * 2
}

input := make(chan)
output := make(chan)
go worker(input, output)
input <- 21
answer := <-output
assert(answer == 42)
```

A `go` statement accepts only function-call and method-call forms. Starting a
task may raise a runtime error if a host goroutine budget is exhausted. Return
values from the task are discarded. An uncaught error in the task does not become
a return value of the spawning task; implementations report it through their
runtime diagnostic path.

Tasks are scheduled by the implementation and host runtime. Implementations may
run tasks on host threads where safe, but Leia does not promise a particular
thread, interleaving, fairness policy, or instruction-level preemption point.
Programs that need completion should communicate explicitly, usually by sending
a result on a channel.

## Channels

A channel is created with `make(chan)` or `make(chan, capacity)`. The capacity
must be a non-negative integer. `make(chan)` is equivalent to `make(chan, 0)` and
creates an unbuffered channel. `type(ch)` returns `"channel"`; v1.0 has no
script-visible send-only, receive-only, or direction-parameterized channel
types.

```leia run all
ch := make(chan, 2)
assert(type(ch) == "channel")
assert(cap(ch) == 2)
assert(rawlen(ch) == 0)

ch <- "a"
ch <- "b"
assert(rawlen(ch) == 2)
```

Channels carry ordinary Leia values. Sending uses `ch <- value`; receiving uses
`<-ch`. A receive can be used as a single value or in comma-ok form:

```leia run all
ch := make(chan, 2)
ch <- "a"
ch <- "b"
close(ch)

first, ok1 := <-ch
second, ok2 := <-ch
third, ok3 := <-ch

assert(first == "a" && ok1)
assert(second == "b" && ok2)
assert(third == nil && ok3 == false)
```

For a single channel, successful sends are received in send order. With one
sender this is program order. With multiple senders, the order is the order in
which sends successfully commit to that channel; scheduling determines which
sender commits first. Buffered channels preserve FIFO order among values already
queued in that buffer.

Unbuffered channels rendezvous: a send and its matching receive complete
together. A buffered send may complete before a receiver accepts the value if the
buffer has space. A send blocks while the channel buffer is full. A receive
blocks while the channel buffer is empty and the channel is still open.

Closing a channel prevents future sends. Receives continue to drain buffered
values before reporting closure. Once the channel is closed and empty, a receive
expression yields `nil`; comma-ok receive yields `nil, false`. Sending on a
closed channel or closing an already closed channel is a runtime error.

```leia run all
ch := make(chan, 1)
close(ch)

value, ok := <-ch
assert(value == nil)
assert(ok == false)

sent, err := pcall(func() {
    ch <- "late"
})
assert(sent == false)
assert(string.find(err, "send on closed channel", 1, true) != nil)

closed, err := pcall(close, ch)
assert(closed == false)
assert(string.find(err, "closed channel", 1, true) != nil)
```

Receiving from or sending to a non-channel value is a runtime error. Creating a
channel with a negative or non-integer capacity is a runtime error. Host options
may impose a maximum channel capacity.

## Select

`select` waits on channel send or receive cases:

```ebnf
select_stmt = "select" "{" { select_case } "}" ;
select_case = ( "case" recv_clause | "case" send_clause | "default" ) ":" block ;
recv_clause = [ identifier [ "," identifier ] ":=" ] "<-" expression ;
send_clause = expression "<-" expression ;
```

A receive case may bind the received value, or the received value and the
receive-ok boolean. Those bindings are scoped to the selected case body. A send
case evaluates and sends the case value when that case is selected.

If one or more communication cases can proceed, one ready case is chosen. If no
communication case can proceed and a `default` case exists, `default` is chosen
immediately. If no case can proceed and no `default` exists, `select` blocks
until a case can proceed. If multiple cases are ready, the implementation may
choose any ready case.

```leia run all
ch := make(chan, 1)
selected := ""

select {
case value := <-ch:
    selected = "receive"
default:
    selected = "default"
}
assert(selected == "default")

ch <- 7
select {
case value, ok := <-ch:
    selected = "ready"
    assert(value == 7)
    assert(ok == true)
default:
    selected = "wrong"
}
assert(selected == "ready")
```

A receive case on a closed and empty channel is ready and receives `nil, false`.
A send case on a closed channel raises a runtime error if selected or if a
blocking select discovers the closed send case. A `select` with no cases is a
runtime error.

```leia run all
ch := make(chan, 1)
close(ch)

state := "unset"
select {
case value, ok := <-ch:
    state = "closed"
    assert(value == nil)
    assert(ok == false)
default:
    state = "default"
}
assert(state == "closed")
```

Timeouts and cancellation are library protocols built on channels. For example,
`time.after(duration)` returns a channel that becomes ready after the duration.
The language-level `select` semantics do not require a special timeout case.

## Defer, Errors, And Task Boundaries

`defer call()` is function-scoped. It evaluates the callee and arguments when the
`defer` statement executes, and runs deferred calls in last-in, first-out order
when the current function returns or unwinds through a protected boundary.
Concurrent tasks have their own defer stack; a task started by `go` does not
inherit the spawning task's pending defers.

```leia run all
events := {}
func record(x) {
    events[#events + 1] = x
}

func work() {
    defer record("outer")
    defer record("inner")
}

work()
assert(events[1] == "inner")
assert(events[2] == "outer")
```

Protected calls (`pcall` and `xpcall`) catch runtime errors that occur while the
protected function is executing in the same task, including deferred-call errors
from that protected function. They do not catch uncaught errors from other tasks
started with `go`; use channels or runtime diagnostics to report those errors.

```leia run all
ok, err := pcall(func() {
    defer error("cleanup failed")
})
assert(ok == false)
assert(string.find(err, "cleanup failed", 1, true) != nil)
```

A task that needs to report failure to another task should send an explicit
result value, such as `{ok: false, err: message}`, on a channel. This keeps error
ownership visible in the script and avoids depending on scheduler timing.

## Cancellation

The embedding host may run programs with a cancellation context, deadline, step
budget, goroutine budget, or other resource budget. Cancellation is observed at
implementation-defined checkpoints such as statement boundaries, loop checks,
blocking channel helpers, and cancellation-aware library calls. The language does
not promise that every primitive operation is preemptible.

Script-level cancellation APIs, such as `context.withCancel`, are ordinary
library values. They become useful when a task or library call selects on the
context's done channel or explicitly checks the context state.

```leia run all
ctx, cancel := context.withCancel()
done := ctx.done
out := make(chan, 1)

go func() {
    total := 0
    for {
        select {
        case signal := <-done:
            out <- total
            return;
        default:
            total = total + 1
            if total == 3 {
                cancel()
            }
        }
    }
}()

result := <-out
assert(result >= 3)
assert(ctx.cancelled())
```

Cancellation, budget exhaustion, process termination, and host shutdown may stop
execution without running arbitrary future script steps. Defers that are already
on the stack run when execution unwinds through a Leia protected boundary; host
termination outside such a boundary is controlled by the embedding contract.

## Memory Ordering

Channel communication is the synchronization primitive in the stable language
contract:

- A successful send synchronizes with the receive that obtains that value.
- Closing a channel synchronizes with receives that observe the closed state
  after queued values have been consumed.
- For unbuffered channels, send and receive complete as a rendezvous; neither
  side completes before the other side is ready.
- For buffered channels, a send may complete before a receiver accepts the
  value, but the value and writes sequenced before the send become visible to
  the receiver that later obtains that value.

Outside those synchronization edges, concurrent tasks have no specified memory
ordering. Programs must not depend on the relative timing of ordinary reads and
writes performed by different tasks unless they are ordered by channels or by a
host-provided synchronization object whose module contract says so.

Mutable tables are shared by identity. Concurrent unsynchronized mutation of the
same table has unspecified script-level results: reads may observe any
interleaving allowed by the implementation, and compound operations such as
`x = x + 1` are not atomic. Implementations must protect host memory safety, but
programs that require deterministic results should transfer ownership through
channels or guard shared tables with a synchronization library.

```leia run all
requests := make(chan, 4)
replies := make(chan, 4)

go func() {
    state := {count: 0}
    for i := 1; i <= 4; i++ {
        delta := <-requests
        state.count = state.count + delta
        replies <- state.count
    }
}()

for i := 1; i <= 4; i++ {
    requests <- 1
}

last := 0
for i := 1; i <= 4; i++ {
    last = <-replies
}
assert(last == 4)
```

## Optimization Contract

JIT, bytecode, and interpreter execution must preserve concurrency-visible
behavior: communication order, channel close behavior, synchronization results,
select readiness, protected error boundaries, and host cancellation boundaries
must not change when optimizations are enabled. Optimizations may change timing,
scheduling, or whether a task reaches a cancellation checkpoint before another
task, except where the program has established ordering through channels or a
specified synchronization library.
