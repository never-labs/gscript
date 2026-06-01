package testkit

import "github.com/never-labs/gscript/internal/runtime"

type Runtime interface {
	TestkitAccessEnabled() bool
	TestkitMemorySnapshot() *runtime.Table
}

type Options struct {
	Runtime Runtime
	Call    runtime.ScriptFunctionCaller
}
