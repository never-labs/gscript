package stdlibrt

import "github.com/never-labs/gscript/internal/runtime"

type TestkitRuntime interface {
	TestkitAccessEnabled() bool
	TestkitMemorySnapshot() *runtime.Table
}

type TestkitOptions struct {
	Runtime TestkitRuntime
	Call    runtime.ScriptFunctionCaller
}
