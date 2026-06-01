package modules

import (
	"fmt"
	goruntime "runtime"

	"github.com/never-labs/gscript/internal/runtime"
	debugrt "github.com/never-labs/gscript/internal/stdlibrt/debug"
)

func BuildDebug(dbg debugrt.Runtime) *runtime.Table {
	t := runtime.NewTable()

	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSetString(name, runtime.FunctionValue(&runtime.GoFunction{
			Name: "debug." + name,
			Fn: func(args []runtime.Value) ([]runtime.Value, error) {
				if dbg != nil && !dbg.DebugAccessEnabled() {
					return nil, fmt.Errorf("debug access disabled")
				}
				return fn(args)
			},
		}))
	}

	set("stack", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.TableValue(runtime.DebugFramesTable(debugStack(dbg, 1)))}, nil
	})

	set("traceback", func(args []runtime.Value) ([]runtime.Value, error) {
		message := ""
		if len(args) > 0 && !args[0].IsNil() {
			message = args[0].String()
		}
		return []runtime.Value{runtime.StringValue(runtime.FormatDebugTraceback(message, debugStack(dbg, 1)))}, nil
	})

	set("info", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) == 0 || args[0].IsNil() {
			frames := debugStack(dbg, 1)
			if len(frames) == 0 {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			return []runtime.Value{runtime.TableValue(runtime.DebugFrameTable(len(frames), frames[len(frames)-1]))}, nil
		}
		if args[0].IsNumber() {
			level := int(toInt(args[0]))
			if level < 0 {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			frames := debugStack(dbg, 1)
			idx := len(frames) - 1 - level
			if idx < 0 || idx >= len(frames) {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			return []runtime.Value{runtime.TableValue(runtime.DebugFrameTable(idx+1, frames[idx]))}, nil
		}
		if !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'debug.info' (function, number, or nil expected)")
		}
		return []runtime.Value{runtime.TableValue(runtime.DebugFunctionInfo(args[0]))}, nil
	})

	set("globals", func(args []runtime.Value) ([]runtime.Value, error) {
		if dbg == nil {
			return []runtime.Value{runtime.TableValue(runtime.NewTable())}, nil
		}
		return []runtime.Value{runtime.TableValue(dbg.DebugGlobalsSnapshot())}, nil
	})

	set("value", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'debug.value' (value expected)")
		}
		out := runtime.NewTable()
		out.RawSetString("type", runtime.StringValue(args[0].TypeName()))
		out.RawSetString("text", runtime.StringValue(args[0].String()))
		out.RawSetString("truthy", runtime.BoolValue(args[0].Truthy()))
		out.RawSetString("raw", runtime.StringValue(fmt.Sprintf("0x%x", args[0].Raw())))
		return []runtime.Value{runtime.TableValue(out)}, nil
	})

	set("goStack", func(args []runtime.Value) ([]runtime.Value, error) {
		buf := make([]byte, 64<<10)
		n := goruntime.Stack(buf, false)
		return []runtime.Value{runtime.StringValue(string(buf[:n]))}, nil
	})

	set("setHook", func(args []runtime.Value) ([]runtime.Value, error) {
		if dbg == nil {
			return nil, nil
		}
		if len(args) == 0 || args[0].IsNil() {
			dbg.ClearDebugHookValue()
			return nil, nil
		}
		if !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'debug.setHook' (function or nil expected)")
		}
		optsArg := runtime.NilValue()
		if len(args) > 1 {
			optsArg = args[1]
		}
		opts, err := runtime.ParseDebugHookOptions(optsArg)
		if err != nil {
			return nil, err
		}
		dbg.SetDebugHookValue(args[0], opts)
		return nil, nil
	})

	set("getHook", func(args []runtime.Value) ([]runtime.Value, error) {
		if dbg == nil {
			return []runtime.Value{runtime.NilValue()}, nil
		}
		hook, opts := dbg.DebugHookValue()
		if hook.IsNil() {
			return []runtime.Value{runtime.NilValue()}, nil
		}
		return []runtime.Value{hook, runtime.TableValue(runtime.DebugHookOptionsTable(opts))}, nil
	})

	set("setSink", func(args []runtime.Value) ([]runtime.Value, error) {
		if dbg == nil {
			return []runtime.Value{runtime.NilValue()}, nil
		}
		sink := runtime.NilValue()
		if len(args) > 0 && !args[0].IsNil() {
			if !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'debug.setSink' (function or nil expected)")
			}
			sink = args[0]
		}
		return []runtime.Value{dbg.SetDebugSinkValue(sink)}, nil
	})

	set("emit", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) == 0 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'debug.emit' (string expected)")
		}
		data := runtime.NilValue()
		if len(args) > 1 {
			data = args[1]
		}
		event := runtime.DebugEventTable("emit", "diagnostic", args[0].Str(), data)
		event.RawSetString("event", args[0])
		if dbg != nil {
			if err := dbg.EmitDebugHook("emit", "diagnostic", args[0].Str(), data); err != nil {
				return nil, err
			}
			if err := dbg.EmitDebugSink(event); err != nil {
				return nil, err
			}
		}
		return []runtime.Value{runtime.BoolValue(true)}, nil
	})

	return t
}

func debugStack(dbg debugrt.Runtime, skip int) []runtime.DebugFrame {
	if dbg == nil {
		return nil
	}
	return dbg.DebugStackSnapshot(skip)
}
