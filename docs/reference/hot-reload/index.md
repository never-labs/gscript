# Leia Hot Reload

Leia hot reload is a Go embedding API for replacing script code while a host
process stays alive. The loader does not start a filesystem watcher; hosts call
reload methods from their watcher, admin endpoint, editor integration, or
deployment hook.

## Two Levels

| API | Use |
|---|---|
| `HotLoader` + `ModuleHandle` | Atomically publish the latest successfully compiled program. The host chooses the VM used for each run. |
| `HotInstance` | Own one persistent VM and automatically preserve ordinary script state while replacing code. |

Use `ModuleHandle` when each request has its own VM. Use `HotInstance` when
one live script instance should keep counters, tables, caches, and other
runtime state across reloads.

## Module Handles

```go
loader := leia.NewHotLoader()
handle, err := loader.Load("logic.leia")
if err != nil {
    return err
}

_ = loader.Reload("logic.leia")
out, err := handle.Call(leia.New(leia.WithVM()), "answer")
```

`Reload` compiles first and swaps only on success. If parsing or compilation
fails, the old generation remains active.

`ReloadIfChanged` hashes source bytes and skips unchanged files. Unchanged
reloads do not advance the generation.

## Hot Instances

```go
loader := leia.NewHotLoader(leia.WithHotLoaderVMOptions(leia.WithVM()))
inst, err := loader.LoadInstance("logic.leia")
if err != nil {
    return err
}

_, _ = inst.Call("tick")
_ = inst.Reload()
_, _ = inst.Call("tick")
```

`HotInstance` runs the program in a persistent VM. On reload it rewrites the
new program before execution so existing non-function globals are preserved by
default and changed functions are replaced.

## State Preservation

Current preservation rules:

- existing scalar globals are preserved;
- existing table globals keep identity;
- new table fields and defaults are merged into existing tables;
- changed functions are replaced;
- unchanged function declarations may be skipped to avoid recreating closure
  state;
- top-level assignments to existing non-function globals are skipped during
  reload;
- new globals are added.

This gives the common online-reload behavior: state stays live, new code takes
effect, and unchanged closures can keep their captured state.

## Rollback

Hot reload is publish-on-success:

- parse/compile failure does not advance generation;
- runtime failure during `HotInstance.Reload` restores previous globals;
- table mutations made by a failed reload are rolled back from a deep snapshot;
- new globals from a failed reload are removed.

The previous generation remains callable after a failed reload.

## Concurrency

`HotLoader` serializes load/reload operations and publishes handles atomically.
Each `Program` and each `VM` still follows the normal embedding rule: do not run
the same instance concurrently unless the API explicitly owns locking.

`HotInstance` locks around reload and calls on its owned VM. Callers must not
use `inst.VM()` concurrently with `HotInstance` methods.

## Watcher Pattern

Use `ReloadIfChanged` from a watcher or polling loop:

```go
result, err := inst.ReloadIfChanged()
if err != nil {
    log.Printf("reload failed: %v", err)
}
if result.Changed {
    log.Printf("reloaded generation %d", result.Generation)
}
```

## Limits

Hot reload is not a whole-program migration system:

- running goroutines are not migrated;
- externally stored old closures are not rewritten;
- arbitrary semantic migrations still belong in user code or host code;
- `HotLoader` does not register files into `require()`;
- top-level side effects should be designed with reload in mind.

For explicit migrations, expose a host API or script function and call it after
a successful generation change.
