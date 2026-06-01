# Concurrency

Leia supports Go-style concurrency constructs adapted to a scripting runtime.

`go call()` starts a goroutine-like task. Implementations may schedule tasks on
host threads where safe. The host runtime remains responsible for resource
budgets, cancellation, and integration with embedding policies.

Channels carry runtime values. Send and receive use `<-`. Receive expressions
may use comma-ok style bindings in `select` and supported receive forms.

`select` chooses a ready communication case. If multiple cases are ready, the
implementation may choose among them. If no case is ready and a `default` case
exists, `default` runs immediately.

Long-running concurrent tasks must cooperate with cancellation and resource
limits provided by the host. The language does not promise that every script
operation is preemptible at instruction-level granularity.

JIT and bytecode execution must preserve concurrency-visible behavior:
communication order, channel close behavior, synchronization results, and
protected error boundaries must not change when optimizations are enabled.
