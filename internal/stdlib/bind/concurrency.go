package bind

import "github.com/never-labs/leia/internal/runtime"

type TaskLauncher func(runtime.Value, []runtime.Value, func(error))

type ConcurrencyOptions struct {
	Call   runtime.ScriptFunctionCaller
	Launch TaskLauncher
}
