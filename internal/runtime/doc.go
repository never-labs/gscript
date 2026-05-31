// Package runtime implements the tree-walking runtime for GScript.
//
// It owns the dynamic value model, tables, closures, coroutines, host-facing
// budgets, standard-library module implementations, and the interpreter that
// executes AST programs directly. Bytecode execution lives in internal/vm, and
// native-code generation lives below internal/jit and internal/methodjit.
package runtime
