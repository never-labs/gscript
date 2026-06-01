package bind

import "github.com/never-labs/leia/internal/runtime"

type TestkitRuntime interface {
	TestkitAccessEnabled() bool
	TestkitMemorySnapshot() *runtime.Table
}

type TestkitOptions struct {
	Runtime TestkitRuntime
	Call    runtime.ScriptFunctionCaller
}
