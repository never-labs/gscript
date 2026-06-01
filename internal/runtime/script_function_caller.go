package runtime

// ScriptFunctionCaller invokes a Leia function value from the active
// execution engine. Stdlib functions that accept callbacks use it to stay
// interpreter/VM-aware.
type ScriptFunctionCaller func(Value, []Value) ([]Value, error)

// FunctionCaller is kept as a source-compatible alias for older runtime users.
type FunctionCaller = ScriptFunctionCaller
