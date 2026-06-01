# Errors And Diagnostics

Leia distinguishes runtime errors from recoverable structured failures.

Programmer errors such as wrong argument types, invalid operations, missing
fields where an API requires them, and invalid control flow generally raise
runtime errors.

```leia
error("boom")
```

Recoverable host and provider failures should return `nil, err` or a
structured result when the API is designed for recovery. LLM provider failures,
filesystem failures, network failures, and sandbox denials must not leak data
forbidden by the active capability policy.

`pcall` and `xpcall`-style protected execution convert runtime errors into
values according to their standard-library contract.

```leia
ok, value := pcall(func() {
    error({kind: "demo"})
})
// ok == false; value.kind == "demo"

ok, handled := xpcall(error, func(err) {
    return "handled:" .. tostring(err)
}, "boom")
// ok == false; handled == "handled:boom"
```

Diagnostics should include source location when available. CLI diagnostics may
be emitted as text, JSON, or SARIF according to the command contract.
