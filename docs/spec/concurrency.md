# Concurrency

Leia supports Go-style concurrency constructs adapted to a scripting runtime.

`go call()` starts a goroutine-like task. Implementations may schedule tasks on
host threads where safe. The host runtime remains responsible for resource
budgets, cancellation, and integration with embedding policies.

```leia
func worker(input, output) {
    value := <-input
    output <- value * 2
}

input := make(chan)
output := make(chan)
go worker(input, output)
input <- 21
answer := <-output // 42
```

Channels carry runtime values. Send and receive use `<-`. Receive expressions
may use comma-ok style bindings in `select` and supported receive forms.

```leia
ch := make(chan, 1)
ch <- "ready"
value := <-ch
close(ch)
value, ok := <-ch // ok is false after the buffered values are consumed
```

For a single channel, successful sends are received in send order. With one
sender this is program order. With multiple senders, the order is the order in
which sends successfully commit to that channel; scheduling determines which
sender commits first. Buffered channels preserve FIFO order among values already
queued in that buffer.

Closing a channel prevents future sends. Receives continue to drain buffered
values before reporting closure. Once the channel is closed and empty, a receive
returns `nil, false` in comma-ok forms, and a receive expression yields `nil`.
Sending on a closed channel or closing an already closed channel is a runtime
error.

`select` chooses a ready communication case. If multiple cases are ready, the
implementation may choose among them. If no case is ready and a `default` case
exists, `default` runs immediately.

```leia
select {
case value := <-left:
    print(value)
case right <- 20:
    print("sent")
default:
    print("idle")
}
```

Channel communication is the synchronization primitive exposed by the stable
language contract:

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
host-provided synchronization object whose module contract says so. Data races
on mutable script values have unspecified results, though implementations must
not turn them into host memory corruption.

The language does not promise fairness among runnable tasks, blocked sends,
blocked receives, or simultaneously ready `select` cases. An implementation may
make progress decisions according to its scheduler and host integration. Code
that requires fairness must encode it explicitly with protocol state, timeouts,
or host-provided primitives.

Long-running concurrent tasks must cooperate with cancellation and resource
limits provided by the host. The language does not promise that every script
operation is preemptible at instruction-level granularity.

JIT and bytecode execution must preserve concurrency-visible behavior:
communication order, channel close behavior, synchronization results, and
protected error boundaries must not change when optimizations are enabled.
