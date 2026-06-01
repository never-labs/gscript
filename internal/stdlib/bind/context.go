package bind

import (
	"fmt"
	"time"

	"github.com/never-labs/leia/internal/runtime"
)

func BuildContext() *runtime.Table {
	t := runtime.NewTable()
	t.RawSetString("background", runtime.FunctionValue(&runtime.GoFunction{
		Name: "context.background",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			return []runtime.Value{runtime.TableValue(runtime.NewScriptContextTable(nil))}, nil
		},
	}))
	t.RawSetString("withCancel", runtime.FunctionValue(&runtime.GoFunction{
		Name: "context.withCancel",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			state := runtime.NewScriptContextState()
			return []runtime.Value{runtime.TableValue(runtime.NewScriptContextTable(state)), runtime.ScriptContextCancelValue(state)}, nil
		},
	}))
	t.RawSetString("withTimeout", runtime.FunctionValue(&runtime.GoFunction{
		Name: "context.withTimeout",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'context.withTimeout'")
			}
			secs := toFloat(args[0])
			if secs < 0 {
				return nil, fmt.Errorf("bad argument #1 to 'context.withTimeout' (non-negative duration expected)")
			}
			state := runtime.NewScriptContextState()
			time.AfterFunc(time.Duration(secs*float64(time.Second)), func() {
				state.Cancel(runtime.StringValue("deadline exceeded"))
			})
			return []runtime.Value{runtime.TableValue(runtime.NewScriptContextTable(state)), runtime.ScriptContextCancelValue(state)}, nil
		},
	}))
	return t
}
