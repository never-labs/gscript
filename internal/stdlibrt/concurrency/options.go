package concurrency

import "github.com/never-labs/gscript/internal/runtime"

type TaskLauncher func(runtime.Value, []runtime.Value, func(error))

type Options struct {
	Call   runtime.ScriptFunctionCaller
	Launch TaskLauncher
}
