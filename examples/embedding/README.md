# Embedding Examples

This directory contains executable Go examples for embedding Leia from the
public `leia` package. The examples live in
[`embedding_test.go`](embedding_test.go), so `go test` keeps the snippets and
their documented output in sync.

[`hot_reload_project/`](hot_reload_project/) is a small host-side project test
that loads a real Leia file, edits it through reload scenarios, and gates the
project-level embedding claim outside package-level doc tests.

## Coverage

- `Example_compileRun` covers compiling source with `Compile`, preserving a
  source name, running the compiled program on a VM, and reading a global.
- `Example_value` covers public `Value` constructors, VM globals, value
  inspection, and encoding a Go value for script use.
- `Example_hostFunctionBinding` covers registering a reflected Go function with
  `RegisterFunc` and calling it from Leia.
- `Example_hostModuleImport` covers `RegisterModule`, Go-style
  `import "go/..."`, and the distinction between explicit host modules and
  filesystem module loading.
- `Example_hotLoader` covers `HotLoader`, generation swaps, and failed reloads
  preserving the previous compiled program.
- `Example_hotInstance` covers online reload with persistent VM state,
  automatic non-function global preservation, and function replacement.
- `Example_productionEmbedding` covers the recommended production embedding
  shape: `SecuritySandbox`, resource budgets, an explicit `WithGoImports`
  allowlist, rejected unauthorized Go imports, and hot reload preserving state.
- `hot_reload_project` covers a project-level host integration with persistent
  state, function replacement, failed reload rollback, explicit host import
  allowlisting, and host-result budget preservation.
- `Example_sandboxAndMaxSteps` covers `WithSandbox`, disabled filesystem
  globals, and statement/instruction budgeting with `WithMaxSteps`.
- `Example_securitySandboxAndBudgets` covers `SecuritySandbox`, its `LibSafe`
  baseline, explicit host callback registration, disabled unsafe globals, and
  host-result resource budgeting.
- `Example_structuredErrors` covers structured script and host callback errors
  with standard `errors.As` and `errors.Is` handling.
- `Example_llmProvider` covers the native `llm` module with a Go-provided
  model backend, message constructors, and a script-side `llm.turn` call.

Package-level examples in `../../example_test.go` exercise the same
public surface from the `leia` package documentation.

## Running

From the repository root:

```sh
go test ./examples/embedding -run Example -count=1
go test ./examples/embedding/hot_reload_project -count=1
```
