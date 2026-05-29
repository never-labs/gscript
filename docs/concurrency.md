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
- `select { case v := <-ch: ... default: ... }`
- `select { case ch <- value: ... default: ... }`
- `len(ch)` for queued buffered values
- `cap(ch)` for channel capacity

The implementation uses isolated child VMs for `go` calls. Globals are snapshotted for lock-light reads, while heap objects such as tables and channels are shared by pointer. This keeps ordinary single-threaded code on the existing fast path.

`select` is currently implemented as a non-blocking statement with a required `default` clause. This covers polling, fan-in probes, timeout checks, and backpressure-aware sends without adding scheduler checks to ordinary code paths.
