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
- `close(ch)`
- `for v := range ch { ... }`
- `select { case v := <-ch: ... }`
- `select { case ch <- value: ... default: ... }`
- `len(ch)` for queued buffered values
- `cap(ch)` for channel capacity
- `sync.waitgroup()` with `add`, `done`, and `wait`
- `sync.mutex()` with `lock`, `unlock`, and `trylock`
- `sync.rwmutex()` with `lock`, `unlock`, `rlock`, `runlock`, `trylock`, and `tryrlock`
- `sync.once()` with `do(fn)`

The implementation uses isolated child VMs for `go` calls. Globals are snapshotted for lock-light reads, while heap objects such as tables and channels are shared by pointer. This keeps ordinary single-threaded code on the existing fast path.

`select` without `default` blocks until a case is ready. `select` with `default` is non-blocking and covers polling, fan-in probes, timeout checks, and backpressure-aware sends without adding scheduler checks to ordinary code paths.

`sync.waitgroup()` mirrors Go's coarse task-join pattern for scripts that need to wait for a fixed set of goroutines.

`sync.mutex()`, `sync.rwmutex()`, and `sync.once()` are host-backed coarse synchronization primitives. They are intended for shared heap objects that are explicitly touched by goroutines, without adding locks to ordinary local computation.
