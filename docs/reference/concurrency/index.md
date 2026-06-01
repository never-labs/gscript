# Leia Concurrency

Leia exposes Go-style concurrency to scripts without requiring embedders to
rewrite their host application around script scheduling. Script goroutines run
as host goroutines, channels are runtime values, and synchronization helpers
live in the `sync`, `context`, and `time` standard-library modules.

## Goroutines

`go` starts an asynchronous task from a function or method call.

```leia
func worker(start, stop, out) {
    total := 0
    for i := start; i <= stop; i++ {
        total = total + i
    }
    out <- total
}

out := make(chan, 4)
go worker(1, 1000, out)
sum := <-out
```

The expression after `go` must be a call. Returned values are discarded; send
results through a channel or use `sync.group` when failures matter.

## Channels

Create channels with `make(chan)` or `make(chan, capacity)`.

```leia
ch := make(chan, 1)
ch <- "ready"
value := <-ch
close(ch)
```

`len(ch)` returns the number of buffered values. `cap(ch)` returns the channel
capacity. Receiving from a closed channel supports comma-ok form:

```leia
value, ok := <-ch
```

When `ok` is false, the channel is closed and no more values will arrive.

## Select

`select` waits on channel send and receive cases. `default` runs immediately
when no case is ready.

```leia
select {
case v := <-left:
    print(v)
case right <- 20:
    print("sent")
default:
    print("idle")
}
```

Supported case forms:

```leia
case <-ch:
case v := <-ch:
case v, ok := <-ch:
case ch <- value:
default:
```

`select` is statement-position syntax. A function named `select` can still be
called as `select(...)`.

Timeouts are usually expressed with `time.after(seconds)`:

```leia
select {
case msg := <-done:
    print(msg)
case <-time.after(0.5):
    print("timeout")
}
```

## Wait Groups And Locks

The `sync` module provides familiar in-process primitives:

| API | Use |
|---|---|
| `sync.waitgroup()` | `add(n)`, `done()`, and `wait()`. |
| `sync.mutex()` | `lock()`, `unlock()`, and `trylock()`. |
| `sync.rwmutex()` | `lock`, `unlock`, `rlock`, `runlock`, and try variants. |
| `sync.once()` | Run one function once with `do(fn)`. |
| `sync.group([ctx])` | Start cancellable tasks and collect failures. |

Example:

```leia
wg := sync.waitgroup()
out := make(chan, 2)

wg.add(2)
go func() { out <- 1; wg.done() }()
go func() { out <- 2; wg.done() }()
wg.wait()

print(<-out + <-out)
```

`sync.group()` is preferred when one task failure should cancel the rest:

```leia
group := sync.group()

group.start(func(ctx) {
    ok, err := time.sleep(ctx, 10)
    if !ok {
        return err
    }
})

ok, err, failures := group.wait()
```

## Contexts And Cancellation

The `context` module creates script-visible cancellation tables.

| API | Returns |
|---|---|
| `context.background()` | Non-cancelling context table. |
| `context.withCancel()` | `ctx, cancel`. |
| `context.withTimeout(seconds)` | `ctx, cancel`, cancelled after the timeout. |

Context-aware APIs such as `time.sleep(ctx, seconds)` and `process.run(ctx,
args)` return `(ok, err)` style results when cancelled.

Hosts can also cancel through Go contexts passed to `ExecContext`, `RunContext`,
`CallContext`, compile entrypoints, and hot reload methods.

## Shared State

Tables are optimized for single-threaded use by default. When a table is shared
between script goroutines, synchronization belongs in user code through
channels or `sync` primitives.

Prefer message passing:

```leia
updates := make(chan)
go func() { updates <- {count: 1} }()
msg := <-updates
```

Use locks only when shared mutable state is the simpler model.

## Budgets And JIT

Embedders can limit concurrency with:

```go
leia.WithMaxGoroutines(64)
leia.WithMaxChannelCapacity(1024)
```

Setting concurrency budgets disables JIT for that VM so task creation and
channel allocation cannot bypass runtime checks. `SecuritySandbox()` also
disables JIT by default.

Concurrency benchmark selectors live under `benchmarks/concurrency/`; examples
live under `examples/concurrency/`.
