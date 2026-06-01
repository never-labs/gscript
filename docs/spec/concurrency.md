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

Long-running concurrent tasks must cooperate with cancellation and resource
limits provided by the host. The language does not promise that every script
operation is preemptible at instruction-level granularity.

JIT and bytecode execution must preserve concurrency-visible behavior:
communication order, channel close behavior, synchronization results, and
protected error boundaries must not change when optimizations are enabled.
