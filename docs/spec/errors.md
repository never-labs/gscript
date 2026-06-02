# Errors And Diagnostics

Leia distinguishes runtime errors from recoverable structured failures. A
runtime error aborts the current expression or statement and unwinds the active
call stack until it reaches a protected-call boundary or the top-level execution
boundary. Deferred calls run according to the statement semantics for `defer`;
errors raised while unwinding replace or report through the same runtime error
path as other deferred-call failures.

## Error Values

An error value is any Leia value carried by an error path. Script code can raise
strings, numbers, booleans, `nil`, tables, functions, and other values. Leia
does not normalize a script-raised table into a standard diagnostic shape and
does not stringify script-raised non-strings before handing them to `pcall` or
an `xpcall` handler.

The v1.0 contract for script-raised error values is identity-preserving for
identity-bearing values. A table raised with `error(t)` is the same table
received by `pcall` or by an `xpcall` handler. If the table already has fields
such as `kind`, `message`, `code`, `source`, or `retryable`, those fields are
ordinary user data unless a specific library documents that table as a
structured result.

```leia run all
payload := {kind: "demo", message: "boom"}
ok, err := pcall(func() {
    error(payload)
})
assert(!ok)
assert(err == payload)
assert(err.kind == "demo")
```

```leia run all
ok, err := pcall(error, nil)
assert(!ok)
assert(err == nil)

ok, err = pcall(error, false)
assert(!ok)
assert(err == false)
```

Runtime errors raised by invalid operations, host callbacks, parser/compiler
operations called from script, resource budgets, and capability checks are
represented inside `pcall` and `xpcall` as diagnostic strings.

```leia run all
ok, err := pcall(func() {
    return "x" + 3
})
assert(!ok)
assert(type(err) == "string")
```

## `error`

`error(value)` raises exactly `value`. `error()` raises a string error value.
Extra arguments to `error` are evaluated by the call expression but ignored by
the v1.0 `error` contract; the first argument is the only raised value.

```leia run all
ok, err := pcall(func() {
    error()
})
assert(!ok)
assert(type(err) == "string")

ok, err = pcall(error, "first", "ignored")
assert(!ok)
assert(err == "first")
```

`assert(false, value)` and `assert(nil, value)` raise `value` when a second
argument is present. If no message argument is present, the failure is a runtime
diagnostic string.

```leia run all
payload := {kind: "assertion"}
ok, err := pcall(func() {
    assert(false, payload)
})
assert(!ok)
assert(err == payload)

ok, err = pcall(assert, false)
assert(!ok)
assert(type(err) == "string")
```

## Protected Calls

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

The first argument to `pcall` is evaluated before the protected call begins. The
protected region includes the attempt to call that value, so a non-callable
first argument is returned as `false, err`. With no first argument, `pcall()`
raises an unprotected argument error at the call site.

Arguments after the function are evaluated before entering the callee, then
passed normally. Once the callable is entered, both script-raised errors and
runtime errors are converted to the protected result form.

```leia run all
ok, err := pcall(17)
assert(!ok)
assert(type(err) == "string")
```

`xpcall(fn, handler, ...)` calls `fn(...)` in protected mode. On success it
returns `true` followed by all results from `fn`. On failure it calls
`handler(err)` and returns `false` plus the handler's first result. Extra
handler results are discarded. If the handler returns no values, the second
`xpcall` result is `nil`. If the handler itself fails, `xpcall` returns `false`
plus the handler failure's error object.

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
ok, a, b := xpcall(func() {
    error("bad")
}, func(err) {
    return "handled:" .. err, "discarded"
})
assert(!ok)
assert(a == "handled:bad")
assert(b == nil)
```

```leia run all
ok, value := xpcall(func() {
    error("bad")
}, func(err) {
})
assert(!ok)
assert(value == nil)
```

Like `pcall`, `xpcall()` and `xpcall(fn)` raise unprotected argument errors
because the protected function and handler arguments are missing.

```leia fail all
pcall()
```

```leia fail all
xpcall(print)
```

Protected calls are recovery boundaries for ordinary script failures. They do
not make process termination, cancellation, host shutdown, or bugs in the host
process safe to continue unless the embedding API explicitly reports those
conditions as ordinary errors.

## Runtime Errors

The following conditions must raise runtime errors when they occur during
execution:

| Error class | Required triggers |
| --- | --- |
| Missing or invalid arguments | Calling a builtin without a required argument; passing a value of the wrong type; passing an invalid option, index, range, mode, pattern, or table shape required by that API. |
| Invalid calls | Calling a non-function value with no `__call` metamethod; calling `pcall` or `xpcall` without their required protected-call arguments. |
| Invalid operators | Arithmetic, bitwise, concatenation, length, or ordering over unsupported values when no matching metamethod applies. |
| Invalid assignment and table protocol | Assigning through an invalid target; indexing or assigning through a table/metatable protocol that exceeds supported `__index` or `__newindex` chains; using an invalid metamethod result such as a non-string `__tostring`; attempting to change a protected metatable. |
| Invalid control flow and coroutine use | Yielding outside a yieldable coroutine boundary; resuming an invalid coroutine state; invalid channel send, receive, close, or select operations. |
| Module and script loading failures | Module not found; module loading disabled; module path escaping the configured filesystem root; malformed source passed to a script compile/load API. |
| Resource and capability failures | Step, native-call, call-depth, goroutine, channel-capacity, host-result, module-byte, or module-depth budget exhaustion; filesystem, network, process, debug, testkit, dynamic-eval, or module-loading capability denial. |
| Host failures surfaced as runtime errors | Registered callback errors, callback panics recovered by the embedding API, provider errors, filesystem errors, network errors, and sandbox denials when the API is not designed to return a recoverable structured result. |

Runtime error diagnostic strings are for humans. The stable script-level
contract is their type (`string`), recovery boundary, and association with the
failing operation, not exact wording. Tests and portable programs must not parse
runtime diagnostic prose unless a specific CLI JSON field or library result
table documents that text as machine-readable.

```leia run all
func fails(f) {
    ok, err := pcall(f)
    assert(!ok)
    assert(type(err) == "string")
}

fails(func() { return {}() })
fails(func() { return "a" .. false })
fails(func() { return math.sin() })
fails(func() { coroutine.yield() })
```

## Structured Recoverable Failures

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

Inside Leia code, typed diagnostics are not automatically reified as tables.
The current boundary is:

| Boundary | Error representation |
| --- | --- |
| Public Go APIs | Typed Go errors such as `*leia.Error`, `*leia.BudgetError`, `*leia.HostCallbackError`, `*leia.HostCallbackPanicError`, and `*leia.ExitError`. |
| `pcall` / `xpcall` for `error(value)` | The original Leia value. |
| `pcall` / `xpcall` for runtime, parser, host, budget, or capability failures | A diagnostic string. |
| Recoverable library/provider APIs | API-specific results, commonly `nil, err` or a table with fields such as `kind`, `message`, `code`, `source`, and `retryable`. |
| CLI diagnostics | Command-specific text, JSON, or SARIF. JSON field names documented for a command are the stable interface; prose messages may change. |

## Public API Diagnostics

Public Go execution APIs expose typed errors before text formatting. `*leia.Error`
has stable fields `Kind`, `Message`, `Line`, `Col`, `File`, `Err`, and `Value`.
`Kind` is `lex`, `parse`, `runtime`, or `script`; `script` is used for
`error(value)` and carries the converted original value in `Value` when
conversion succeeds. Runtime, parser, and lexer errors use `Message` as a human
diagnostic. `Err` unwraps to the underlying cause, including `*leia.BudgetError`,
`*leia.ExitError`, host callback errors, and host callback panic errors when
available.

Diagnostics should include source location when available. Stable source fields
are source name or file path, line, and column. Public Go APIs map those to
`File`, `Line`, and `Col`. Script-side debug APIs may expose `sourceName`,
`line`, and `column` fields for frames and functions when debug access is
enabled. String diagnostics may include the same coordinates in text, but
callers that need stability should use typed Go errors, documented debug table
fields, or CLI JSON.

Stack and traceback text is diagnostic output, not a stable machine interface.
Frame ordering, formatting, function names, native/script labels, and stack
depth are not stable unless a specific command or API documents them as
structured fields.
