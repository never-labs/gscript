# Concurrency

GScript supports Go-style lightweight asynchronous work at the script level:

```gscript
ch := make(chan, 4)

go func() {
    ch <- 42
}()

value := <-ch
close(ch)
```

Supported core forms:

- `go f(args...)` and `go obj.method(args...)`
- `make(chan)` and `make(chan, capacity)`
- `ch <- value`
- `<-ch`
- `value, ok := <-ch`
- `close(ch)`
- `for v := range ch { ... }`
- `select { case v := <-ch: ... }`
- `select { case v, ok := <-ch: ... }`
- `select { case ch <- value: ... default: ... }`
- `len(ch)` for queued buffered values
- `cap(ch)` for channel capacity
- `sync.waitgroup()` with `add`, `done`, and `wait`
- `sync.mutex()` with `lock`, `unlock`, and `trylock`
- `sync.rwmutex()` with `lock`, `unlock`, `rlock`, `runlock`, `trylock`, and `tryrlock`
- `sync.once()` with `do(fn)`
- `time.after(seconds)` for timeout channels in `select`
- `context.withCancel()` and `context.withTimeout(seconds)` for shared cancellation
- `time.sleep(ctx, seconds)` for cancellable sleep

The implementation uses isolated child VMs for `go` calls. Globals are snapshotted for lock-light reads, while heap objects such as tables and channels are shared by pointer. This keeps ordinary single-threaded code on the existing fast path.

`select` without `default` blocks until a case is ready. `select` with `default` is non-blocking and covers polling, fan-in probes, timeout checks, and backpressure-aware sends without adding scheduler checks to ordinary code paths.

`sync.waitgroup()` mirrors Go's coarse task-join pattern for scripts that need to wait for a fixed set of goroutines.

`sync.mutex()`, `sync.rwmutex()`, and `sync.once()` are host-backed coarse synchronization primitives. They are intended for shared heap objects that are explicitly touched by goroutines, without adding locks to ordinary local computation.

`time.after(seconds)` returns a channel that receives the current time once after the delay, so timeout logic can stay in ordinary `select` syntax.

`context.withCancel()` returns `ctx, cancel`. `ctx.done` is closed when the context is cancelled, `ctx.cancelled()` reports the state, and `ctx.err()` returns `"cancelled"` or `"deadline exceeded"`. `context.withTimeout(seconds)` uses the same shape and cancels automatically after the deadline.

`time.sleep(ctx, seconds)` returns `true, nil` when the sleep completes or `false, err` when `ctx.done` closes first. `time.sleep(seconds)` remains the simple blocking form.
