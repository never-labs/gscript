# Embedding Examples

This directory contains executable Go examples for embedding GScript from the
public `gscript` package. The examples live in
[`embedding_test.go`](embedding_test.go), so `go test` keeps the snippets and
their documented output in sync.

## Coverage

- `Example_compileRun` covers compiling source with `Compile`, preserving a
  source name, running the compiled program on a VM, and reading a global.
- `Example_value` covers public `Value` constructors, VM globals, value
  inspection, and encoding a Go value for script use.
- `Example_hostFunctionBinding` covers registering a reflected Go function with
  `RegisterFunc` and calling it from GScript.
- `Example_hostModuleRequire` covers `RegisterModule`, `require("go/...")`,
  and the distinction between explicit host modules and filesystem module
  loading.
- `Example_hotLoader` covers `HotLoader`, generation swaps, and failed reloads
  preserving the previous compiled program.
- `Example_hotInstance` covers online reload with persistent VM state,
  automatic non-function global preservation, and function replacement.
- `Example_sandboxAndMaxSteps` covers `WithSandbox`, disabled filesystem
  globals, and statement/instruction budgeting with `WithMaxSteps`.
- `Example_structuredErrors` covers structured script and host callback errors
  with standard `errors.As` and `errors.Is` handling.

Package-level examples in `../../gscript/example_test.go` exercise the same
public surface from the `gscript` package documentation.

## Running

From the repository root:

```sh
go test ./examples/embedding ./gscript
```
