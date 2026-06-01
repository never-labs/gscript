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
one error object. If the failure came from `error(value)`, the object is
`value`; if it came from a runtime, host, parser, budget, or capability failure,
the protected error object is a diagnostic string.

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

Diagnostics should include source location when available. CLI diagnostics may
be emitted as text, JSON, or SARIF according to the command contract.
