# debug

The `debug` library exposes runtime diagnostics, source-aware stack frames, and
coarse event hooks.

VM note: file-mode VM execution has official translated coverage for
`debug.info(function)`, `debug.value`, stack/source metadata, numeric frame
queries, and hook/sink events. The API is intentionally Leia/Go-host shaped
instead of a byte-for-byte Lua `debug` clone.

## Source Information

Stack frames and script functions can include:

- `name` -- function/native name
- `kind` -- `"script"` or `"native"`
- `sourceName` -- configured diagnostic source name, when known
- `line` and `column` -- source coordinates, when known

`sourceName` comes from normal file loading, `script.compile`/`script.eval`
options, or host-side script options.

## Functions

### debug.stack() -> table

Return the current runtime stack as a 1-indexed table of frame tables.

### debug.traceback([message]) -> string

Return a formatted traceback string. If `message` is provided, it is prepended
before `stack traceback:`.

### debug.info([target]) -> table | nil

With no target or `nil`, return the current frame. With a numeric level, return
that stack frame, where `0` is the caller of `debug.info`. With a function,
return function metadata.

Function info for script functions includes `params`, `vararg`, and `upvalues`
in addition to source fields.

```
fn := script.compile("func f(x) { return debug.info(0) }\nreturn f(1)", {
    sourceName: "virtual/generated.leia",
})
info := fn()
print(info.sourceName, info.line, info.column)
```

### debug.globals() -> table

Return a snapshot table of current globals.

### debug.value(v) -> table

Return diagnostic information for a value:

- `type`
- `text`
- `truthy`
- `raw` -- raw value bits as hex text

### debug.goStack() -> string

Return the current Go goroutine stack.

### debug.setHook(fn [, opts])

Install a diagnostic hook, or clear it with `nil`. The hook receives an event
table. Options are booleans:

- events: `call`, `return`, `error`, `emit`
- frame kinds: `script`, `native`

If no event options are specified, all event types are enabled. If no kind
options are specified, both script and native events are enabled.

### debug.getHook() -> fn, opts | nil

Return the current hook and normalized options, or `nil` if no hook is set.

### debug.setSink(fn) -> previousFn | nil

Install an emit sink, or clear it with `nil`. The sink receives `debug.emit`
events.

### debug.emit(name [, data]) -> true

Emit a diagnostic event to the hook and sink.

```
debug.setHook(func(event) {
    print(event.type, event.kind, event.name)
}, {emit: true, script: true, native: false})

debug.setSink(func(event) {
    print(event.event, event.data.done)
})

debug.emit("progress", {done: 10})
debug.setHook(nil)
debug.setSink(nil)
```
