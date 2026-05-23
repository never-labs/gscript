package vm

import (
	"fmt"
	goruntime "runtime"
	"strings"

	"github.com/gscript/gscript/internal/runtime"
)

func (vm *VM) RegisterDebugLib() {
	debugLib := runtime.TableValue(vm.newDebugLib())
	vm.SetGlobal("debug", debugLib)
	vm.setPackageLoaded("debug", debugLib)
}

func (vm *VM) newDebugLib() *runtime.Table {
	t := runtime.NewTable()
	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSetString(name, runtime.FunctionValue(&runtime.GoFunction{
			Name: "debug." + name,
			Fn:   fn,
		}))
	}

	set("stack", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.TableValue(vm.debugFramesTable(vm.debugStackSnapshot()))}, nil
	})
	set("traceback", func(args []runtime.Value) ([]runtime.Value, error) {
		message := ""
		if len(args) > 0 && !args[0].IsNil() {
			message = args[0].String()
		}
		return []runtime.Value{runtime.StringValue(formatVMDebugTraceback(message, vm.debugStackSnapshot()))}, nil
	})
	set("info", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) == 0 || args[0].IsNil() {
			return vm.debugInfoForLevel(0), nil
		}
		if args[0].IsNumber() {
			level := int(args[0].Number())
			if level < 0 {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			return vm.debugInfoForLevel(level), nil
		}
		if !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'debug.info' (function, number, or nil expected)")
		}
		return []runtime.Value{runtime.TableValue(vm.debugFunctionInfo(args[0]))}, nil
	})
	set("globals", func(args []runtime.Value) ([]runtime.Value, error) {
		out := runtime.NewTable()
		for name, val := range vm.globals {
			out.RawSetString(name, val)
		}
		return []runtime.Value{runtime.TableValue(out)}, nil
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
		if len(args) == 0 || args[0].IsNil() {
			vm.debugHook = runtime.NilValue()
			vm.debugOpts = runtime.DebugHookOptions{}
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
		vm.debugHook = args[0]
		vm.debugOpts = opts
		return nil, nil
	})
	set("getHook", func(args []runtime.Value) ([]runtime.Value, error) {
		if vm.debugHook.IsNil() {
			return []runtime.Value{runtime.NilValue()}, nil
		}
		return []runtime.Value{vm.debugHook, runtime.TableValue(runtime.DebugHookOptionsTable(vm.debugOpts))}, nil
	})
	set("setSink", func(args []runtime.Value) ([]runtime.Value, error) {
		prev := vm.debugSink
		if len(args) == 0 || args[0].IsNil() {
			vm.debugSink = runtime.NilValue()
		} else {
			if !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'debug.setSink' (function or nil expected)")
			}
			vm.debugSink = args[0]
		}
		if prev.IsNil() {
			return []runtime.Value{runtime.NilValue()}, nil
		}
		return []runtime.Value{prev}, nil
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
		if err := vm.emitDebugHook("emit", "diagnostic", args[0].Str(), data); err != nil {
			return nil, err
		}
		if err := vm.emitDebugSink(event); err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.BoolValue(true)}, nil
	})
	return t
}

func (vm *VM) debugInfoForLevel(level int) []runtime.Value {
	frames := vm.debugStackSnapshot()
	idx := len(frames) - 1 - level
	if idx < 0 || idx >= len(frames) {
		return []runtime.Value{runtime.NilValue()}
	}
	return []runtime.Value{runtime.TableValue(debugFrameTable(idx+1, frames[idx]))}
}

func (vm *VM) debugStackSnapshot() []runtime.DebugFrame {
	frames := make([]runtime.DebugFrame, 0, vm.frameCount)
	for i := 0; i < vm.frameCount; i++ {
		frame := &vm.frames[i]
		frames = append(frames, vm.debugFrame(frame))
	}
	return frames
}

func (vm *VM) debugFrame(frame *CallFrame) runtime.DebugFrame {
	if frame == nil || frame.closure == nil || frame.closure.Proto == nil {
		return runtime.DebugFrame{Name: "<unknown>", Kind: "script"}
	}
	proto := frame.closure.Proto
	line := proto.LineDefined
	pc := frame.pc - 1
	if pc >= 0 && pc < len(proto.LineInfo) && proto.LineInfo[pc] > 0 {
		line = proto.LineInfo[pc]
	}
	return runtime.DebugFrame{
		Name:       debugProtoName(proto),
		Kind:       "script",
		SourceName: proto.Source,
		Line:       line,
		Column:     1,
	}
}

func (vm *VM) debugFramesTable(frames []runtime.DebugFrame) *runtime.Table {
	out := runtime.NewTable()
	for i, frame := range frames {
		out.RawSet(runtime.IntValue(int64(i+1)), runtime.TableValue(debugFrameTable(i+1, frame)))
	}
	return out
}

func debugFrameTable(index int, frame runtime.DebugFrame) *runtime.Table {
	out := runtime.NewTable()
	out.RawSetString("index", runtime.IntValue(int64(index)))
	out.RawSetString("name", runtime.StringValue(frame.Name))
	out.RawSetString("kind", runtime.StringValue(frame.Kind))
	if frame.SourceName != "" {
		out.RawSetString("sourceName", runtime.StringValue(frame.SourceName))
	}
	if frame.Line > 0 {
		out.RawSetString("line", runtime.IntValue(int64(frame.Line)))
		out.RawSetString("column", runtime.IntValue(int64(frame.Column)))
	}
	return out
}

func (vm *VM) debugFunctionInfo(fn runtime.Value) *runtime.Table {
	out := runtime.NewTable()
	out.RawSetString("type", runtime.StringValue("function"))
	if gf := fn.GoFunction(); gf != nil {
		out.RawSetString("name", runtime.StringValue(gf.Name))
		out.RawSetString("kind", runtime.StringValue("native"))
		return out
	}
	if cl, ok := closureFromValue(fn); ok && cl != nil && cl.Proto != nil {
		proto := cl.Proto
		out.RawSetString("name", runtime.StringValue(debugProtoName(proto)))
		out.RawSetString("kind", runtime.StringValue("script"))
		if proto.Source != "" {
			out.RawSetString("sourceName", runtime.StringValue(proto.Source))
		}
		if proto.LineDefined > 0 {
			out.RawSetString("line", runtime.IntValue(int64(proto.LineDefined)))
			out.RawSetString("column", runtime.IntValue(1))
		}
		out.RawSetString("params", runtime.IntValue(int64(proto.NumParams)))
		out.RawSetString("vararg", runtime.BoolValue(proto.IsVarArg))
		out.RawSetString("upvalues", runtime.IntValue(int64(len(cl.Upvalues))))
		return out
	}
	out.RawSetString("name", runtime.StringValue("<unknown>"))
	out.RawSetString("kind", runtime.StringValue("unknown"))
	return out
}

func debugProtoName(proto *FuncProto) string {
	if proto == nil || proto.Name == "" {
		return "<anonymous>"
	}
	return proto.Name
}

func formatVMDebugTraceback(message string, frames []runtime.DebugFrame) string {
	var b strings.Builder
	if message != "" {
		b.WriteString(message)
		b.WriteByte('\n')
	}
	b.WriteString("stack traceback:")
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		b.WriteString("\n  ")
		b.WriteString(frame.Kind)
		b.WriteByte(' ')
		b.WriteString(frame.Name)
		if frame.SourceName != "" && frame.Line > 0 {
			b.WriteString(" @ ")
			b.WriteString(frame.SourceName)
			b.WriteString(":")
			b.WriteString(fmt.Sprintf("%d:%d", frame.Line, frame.Column))
		}
	}
	return b.String()
}

func (vm *VM) emitDebugHook(eventType, kind, name string, data runtime.Value) error {
	if vm.debugBusy || vm.debugHook.IsNil() || !runtime.DebugHookWants(vm.debugOpts, eventType, kind) {
		return nil
	}
	vm.debugBusy = true
	defer func() {
		vm.debugBusy = false
	}()
	_, err := vm.callValue(vm.debugHook, []runtime.Value{runtime.TableValue(runtime.DebugEventTable(eventType, kind, name, data))})
	return err
}

func (vm *VM) emitDebugSink(event *runtime.Table) error {
	if vm.debugBusy || vm.debugSink.IsNil() {
		return nil
	}
	vm.debugBusy = true
	defer func() {
		vm.debugBusy = false
	}()
	_, err := vm.callValue(vm.debugSink, []runtime.Value{runtime.TableValue(event)})
	return err
}

func (vm *VM) callGoFunction(gf *runtime.GoFunction, args []runtime.Value) ([]runtime.Value, error) {
	if gf == nil {
		return nil, fmt.Errorf("attempt to call a nil native function")
	}
	var a0, a1, a2, a3 runtime.Value
	if len(args) > 0 {
		a0 = args[0]
	}
	if len(args) > 1 {
		a1 = args[1]
	}
	if len(args) > 2 {
		a2 = args[2]
	}
	if len(args) > 3 {
		a3 = args[3]
	}
	fixedArgFastPath := (len(args) == 1 && gf.FastArg1 != nil) ||
		(len(args) == 2 && (gf.FastArg2Ret2 != nil || gf.FastArg2 != nil)) ||
		(len(args) == 3 && gf.FastArg3 != nil) ||
		(len(args) == 4 && gf.FastArg4 != nil)
	if !fixedArgFastPath {
		args = stableGoFunctionArgs(args)
	}
	if err := vm.emitDebugHook("call", "native", gf.Name, runtime.NilValue()); err != nil {
		return nil, err
	}
	var results []runtime.Value
	var err error
	if len(args) == 2 && gf.FastArg2Ret2 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		var r0, r1 runtime.Value
		var n int
		r0, r1, n, err = gf.FastArg2Ret2(a0, a1)
		if err == nil {
			switch {
			case n <= 0:
				results = nil
			case n == 1:
				results = []runtime.Value{r0}
			default:
				results = []runtime.Value{r0, r1}
			}
		}
	} else if len(args) == 1 && gf.FastArg1 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		var v runtime.Value
		v, err = gf.FastArg1(a0)
		if err == nil {
			results = []runtime.Value{v}
		}
	} else if len(args) == 2 && gf.FastArg2 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		var v runtime.Value
		v, err = gf.FastArg2(a0, a1)
		if err == nil {
			results = []runtime.Value{v}
		}
	} else if len(args) == 3 && gf.FastArg3 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		var v runtime.Value
		v, err = gf.FastArg3(a0, a1, a2)
		if err == nil {
			results = []runtime.Value{v}
		}
	} else if len(args) == 4 && gf.FastArg4 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		var v runtime.Value
		v, err = gf.FastArg4(a0, a1, a2, a3)
		if err == nil {
			results = []runtime.Value{v}
		}
	} else if gf.Fast1 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		var v runtime.Value
		v, err = gf.Fast1(args)
		if err == nil {
			results = []runtime.Value{v}
		}
	} else {
		runtime.RecordRuntimePathNativeCallFallbackFor(gf)
		results, err = gf.Fn(args)
	}
	if err != nil {
		_ = vm.emitDebugHook("error", "native", gf.Name, runtime.StringValue(err.Error()))
		return nil, err
	}
	if err := vm.emitDebugHook("return", "native", gf.Name, runtime.NilValue()); err != nil {
		return nil, err
	}
	return results, nil
}

func stableGoFunctionArgs(args []runtime.Value) []runtime.Value {
	if len(args) == 0 {
		return args
	}
	stable := make([]runtime.Value, len(args))
	copy(stable, args)
	return stable
}
