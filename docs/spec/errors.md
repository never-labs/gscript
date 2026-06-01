# Errors And Diagnostics

Leia distinguishes runtime errors from recoverable structured failures.

Programmer errors such as wrong argument types, invalid operations, missing
fields where an API requires them, invalid control flow, exhausted resource
budgets, and denied capabilities generally raise runtime errors.

An error object is any Leia value carried by an error path. `error(value)` raises
that exact value, without converting it to a string. `error()` raises a string
error object. Extra arguments to `error` are ignored by the v1.0 contract.

```leia run all
payload := {kind: "demo", message: "boom"}
ok, err := pcall(func() {
    error(payload)
})
assert(!ok)
assert(err == payload)
assert(err.kind == "demo")
```

`pcall(fn, ...)` calls `fn(...)` in protected mode. On success it returns
`true` followed by all results from the call. On failure it returns `false` and
one error object. If the failure came from `error(value)` or `assert(false,
value)`, the object is `value`; if it came from a runtime, host, parser,
budget, or capability failure, the protected error object is the current
diagnostic string.

```leia run
ok, a, b := pcall(func() {
    return "a", "b"
})

assert(ok)
assert(a == "a")
assert(b == "b")
```

`xpcall(fn, handler, ...)` calls `fn(...)` in protected mode. On success it
returns `true` followed by all results from `fn`. On failure it calls
`handler(err)` and returns `false` plus the handler's first result. If the
handler itself fails, `xpcall` returns `false` plus the handler failure's error
object.

```leia run all
ok, value := pcall(func(a, b) {
    return a + b, "kept"
}, 2, 5)
assert(ok && value == 7)

ok, handled := xpcall(error, func(err) {
    return "handled:" .. tostring(err)
}, "boom")
assert(!ok && handled == "handled:boom")
```

```leia run all
payload := {kind: "assertion"}
ok, err := pcall(func() {
    assert(false, payload)
})

assert(!ok)
assert(err == payload)
```

Runtime errors raised by invalid operations become diagnostic strings inside
`pcall` and `xpcall`.

```leia run all
ok, err := pcall(func() {
    return "x" + 3
})
assert(!ok)
assert(type(err) == "string")

ok, handled := xpcall(func() {
    return {}()
}, func(err) {
    return "kind=" .. type(err)
})
assert(!ok && handled == "kind=string")
```

Protected calls are recovery boundaries for ordinary script failures. They do
not make process termination, cancellation, host shutdown, or bugs in the host
process safe to continue unless the embedding API explicitly reports those
conditions as ordinary errors. A protected call whose own first argument is
invalid raises an unprotected argument error; wrap the call site itself if that
failure must be recovered.

```leia fail all
pcall()
```

Recoverable host and provider failures should return `nil, err` or a
structured result when the API is designed for recovery. Error result tables use
stable, lowercase string fields when structured recovery is intended:

| Field | Meaning |
| --- | --- |
| `kind` | Machine-readable category such as `validation`, `provider`, `network`, `rate_limit`, `budget`, `capability`, or an API-specific kind. |
| `message` | Human-readable diagnostic string safe to expose under the active capability policy. |
| `code` | Optional machine-readable API/provider code. |
| `source` | Optional subsystem or provider name. |
| `retryable` | Optional boolean hint for transient failures. |

Host and provider APIs should use stable `kind` and `code` values when callers
need branching behavior:

| Kind | Typical codes |
| --- | --- |
| `host` | API-specific codes for callback, provider, process, filesystem, network, or environment failures. Public Go callback failures surface as `*leia.HostCallbackError`; callback panics surface as `*leia.HostCallbackPanicError`. |
| `capability` | A denied capability name or API-specific denial code, such as disabled filesystem read/write, network, process execution, debug, testkit, dynamic eval, or module loading. Capability denials are currently runtime diagnostics unless a library intentionally returns a structured error table. |
| `budget` | One of the current public budget resources: `steps`, `native_calls`, `call_depth`, `goroutines`, `channel_capacity`, `host_result_bytes`, `module_bytes`, or `module_depth`. Public execution APIs expose these through `*leia.BudgetError.Resource` and `Limit`. |
| `provider` | Provider-specific API codes, model errors, rate limits, authentication failures, or validation failures. |

LLM provider failures, filesystem failures, network failures, and sandbox
denials must not leak data forbidden by the active capability policy.

Common runtime errors include:

| Error class | Examples |
| --- | --- |
| Arity/type errors | Calling a builtin without a required argument; passing a non-string to `require`; using a table where a number is required. |
| Invalid operators | Arithmetic, concatenation, length, or comparison over unsupported values when no matching metamethod applies. |
| Invalid calls | Calling a non-function value with no `__call` metamethod. |
| Table/metatable protocol errors | Excessive `__index` or `__newindex` chains; invalid `__tostring` return type; attempts to change a protected metatable. |
| Module loading errors | Module not found, module loading disabled, module path escaping the configured filesystem root, module byte/depth budget exceeded. |
| Coroutine errors | Yielding outside a yieldable coroutine boundary or resuming an invalid coroutine state. |
| Resource and capability errors | Step budget exceeded, host-result byte budget exceeded, filesystem/network/process/debug access denied. |

Public Go execution APIs expose typed errors before text formatting. `*leia.Error`
has stable fields `Kind`, `Message`, `Line`, `Col`, `File`, `Err`, and `Value`.
`Kind` is `lex`, `parse`, `runtime`, or `script`; `script` is used for
`error(value)` and carries the converted original value in `Value` when
conversion succeeds. Runtime, parser, and lexer errors use `Message` as a human
diagnostic. `Err` unwraps to the underlying cause, including `*leia.BudgetError`,
`*leia.ExitError`, host callback errors, and host callback panic errors when
available.

Inside Leia code, typed diagnostics are not automatically reified as tables.
The current boundary is:

| Boundary | Error representation |
| --- | --- |
| Public Go APIs | Typed Go errors such as `*leia.Error`, `*leia.BudgetError`, `*leia.HostCallbackError`, `*leia.HostCallbackPanicError`, and `*leia.ExitError`. |
| `pcall` / `xpcall` for `error(value)` | The original Leia value. |
| `pcall` / `xpcall` for runtime, parser, host, budget, or capability failures | A diagnostic string. |
| Recoverable library/provider APIs | API-specific results, commonly `nil, err` or a table with fields such as `kind`, `message`, `code`, `source`, and `retryable`. |
| CLI diagnostics | Command-specific text, JSON, or SARIF. JSON field names documented for a command are the stable interface; prose messages may change. |

Diagnostics should include source location when available. Stable source fields
are source name or file path, line, and column. Public Go APIs map those to
`File`, `Line`, and `Col`. String diagnostics may include the same coordinates
in text, but callers that need stability should use typed Go errors or CLI JSON.

Stack and traceback text is diagnostic output, not a stable machine interface.
Debug APIs may expose source names and frames for humans, but frame formatting,
function names, native/script labels, and stack depth are not stable unless a
specific command or API documents them as structured fields.
