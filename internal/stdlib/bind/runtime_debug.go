package bind

import "github.com/never-labs/leia/internal/runtime"

type DebugRuntime interface {
	DebugAccessEnabled() bool
	DebugStackSnapshot(skip int) []runtime.DebugFrame
	DebugGlobalsSnapshot() *runtime.Table
	SetDebugHookValue(hook runtime.Value, opts runtime.DebugHookOptions)
	ClearDebugHookValue()
	DebugHookValue() (runtime.Value, runtime.DebugHookOptions)
	SetDebugSinkValue(sink runtime.Value) runtime.Value
	EmitDebugHook(eventType, kind, name string, data runtime.Value) error
	EmitDebugSink(event *runtime.Table) error
}
