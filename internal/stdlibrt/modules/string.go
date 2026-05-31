package modules

import "github.com/never-labs/gscript/internal/runtime"

// BuildString creates the "string" standard library table.
//
// The string module still owns runtime-facing fast paths and binary pack
// compatibility hooks, so the stdlibrt boundary intentionally delegates to
// runtime's builder instead of duplicating string semantics.
func BuildString(caller ScriptFunctionCaller, maxHostResult func() int64) *Table {
	return runtime.BuildStringLibWithCaller(caller, maxHostResult)
}
