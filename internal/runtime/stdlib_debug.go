package runtime

import (
	"fmt"
	"runtime"
	"strings"
)

// DebugFrame describes one active runtime call. It is intentionally GScript
// shaped rather than a Lua debug.getinfo clone.
type DebugFrame struct {
	Name string
	Kind string
}

// DebugHookOptions describes the coarse-grained GScript debug hook filters.
// It is intentionally event-oriented rather than a Lua line/count clone.
type DebugHookOptions struct {
	Call   bool
	Return bool
	Error  bool
	Emit   bool
	Script bool
	Native bool
}

func DefaultDebugHookOptions() DebugHookOptions {
	return DebugHookOptions{
		Call:   true,
		Return: true,
		Error:  true,
		Emit:   true,
		Script: true,
		Native: true,
	}
}

func ParseDebugHookOptions(v Value) (DebugHookOptions, error) {
	if v.IsNil() {
		return DefaultDebugHookOptions(), nil
	}
	if !v.IsTable() {
		return DebugHookOptions{}, fmt.Errorf("bad argument #2 to 'debug.setHook' (table or nil expected)")
	}
	t := v.Table()
	opts := DebugHookOptions{Script: true, Native: true}
	seenEvent := false
	for _, key := range []struct {
		name string
		dst  *bool
	}{
		{"call", &opts.Call},
		{"return", &opts.Return},
		{"error", &opts.Error},
		{"emit", &opts.Emit},
	} {
		if opt := t.RawGetString(key.name); !opt.IsNil() {
			if !opt.IsBool() {
				return DebugHookOptions{}, fmt.Errorf("bad option %q to 'debug.setHook' (boolean expected)", key.name)
			}
			*key.dst = opt.Bool()
			seenEvent = true
		}
	}
	if !seenEvent {
		opts.Call = true
		opts.Return = true
		opts.Error = true
		opts.Emit = true
	}

	seenKind := false
	for _, key := range []struct {
		name string
		dst  *bool
	}{
		{"script", &opts.Script},
		{"native", &opts.Native},
	} {
		if opt := t.RawGetString(key.name); !opt.IsNil() {
			if !opt.IsBool() {
				return DebugHookOptions{}, fmt.Errorf("bad option %q to 'debug.setHook' (boolean expected)", key.name)
			}
			*key.dst = opt.Bool()
			seenKind = true
		}
	}
	if !seenKind {
		opts.Script = true
		opts.Native = true
	}
	return opts, nil
}

func DebugHookOptionsTable(opts DebugHookOptions) *Table {
	out := NewTable()
	out.RawSetString("call", BoolValue(opts.Call))
	out.RawSetString("return", BoolValue(opts.Return))
	out.RawSetString("error", BoolValue(opts.Error))
	out.RawSetString("emit", BoolValue(opts.Emit))
	out.RawSetString("script", BoolValue(opts.Script))
	out.RawSetString("native", BoolValue(opts.Native))
	return out
}

func DebugEventTable(eventType, kind, name string, data Value) *Table {
	out := NewTable()
	out.RawSetString("type", StringValue(eventType))
	if kind != "" {
		out.RawSetString("kind", StringValue(kind))
	}
	if name != "" {
		out.RawSetString("name", StringValue(name))
	}
	if !data.IsNil() {
		if eventType == "error" {
			out.RawSetString("error", data)
		} else {
			out.RawSetString("data", data)
		}
	}
	return out
}

func DebugHookWants(opts DebugHookOptions, eventType, kind string) bool {
	switch eventType {
	case "call":
		if !opts.Call {
			return false
		}
	case "return":
		if !opts.Return {
			return false
		}
	case "error":
		if !opts.Error {
			return false
		}
	case "emit":
		if !opts.Emit {
			return false
		}
	default:
		return false
	}
	switch kind {
	case "script":
		return opts.Script
	case "native":
		return opts.Native
	default:
		return true
	}
}

func (interp *Interpreter) pushDebugFrame(name, kind string) {
	interp.callStack = append(interp.callStack, DebugFrame{Name: name, Kind: kind})
}

func (interp *Interpreter) popDebugFrame() {
	if len(interp.callStack) == 0 {
		return
	}
	interp.callStack = interp.callStack[:len(interp.callStack)-1]
}

func (interp *Interpreter) debugStackSnapshot(skip int) []DebugFrame {
	if skip < 0 {
		skip = 0
	}
	n := len(interp.callStack) - skip
	if n < 0 {
		n = 0
	}
	frames := make([]DebugFrame, n)
	copy(frames, interp.callStack[:n])
	return frames
}

func (interp *Interpreter) emitDebugHook(eventType, kind, name string, data Value) error {
	if interp.debugBusy {
		return nil
	}
	hook := interp.debugHook
	if hook.IsNil() || !DebugHookWants(interp.debugOpts, eventType, kind) {
		return nil
	}
	interp.debugBusy = true
	defer func() {
		interp.debugBusy = false
	}()
	_, err := interp.callFunction(hook, []Value{TableValue(DebugEventTable(eventType, kind, name, data))})
	return err
}

func (interp *Interpreter) emitDebugSink(event *Table) error {
	if interp.debugBusy || interp.debugSink.IsNil() {
		return nil
	}
	interp.debugBusy = true
	defer func() {
		interp.debugBusy = false
	}()
	_, err := interp.callFunction(interp.debugSink, []Value{TableValue(event)})
	return err
}

func buildDebugLib(interp *Interpreter) *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{
			Name: "debug." + name,
			Fn:   fn,
		}))
	}

	set("stack", func(args []Value) ([]Value, error) {
		frames := interp.debugStackSnapshot(1)
		return []Value{TableValue(debugFramesTable(frames))}, nil
	})

	set("traceback", func(args []Value) ([]Value, error) {
		message := ""
		if len(args) > 0 && !args[0].IsNil() {
			message = args[0].String()
		}
		frames := interp.debugStackSnapshot(1)
		return []Value{StringValue(formatDebugTraceback(message, frames))}, nil
	})

	set("info", func(args []Value) ([]Value, error) {
		if len(args) == 0 || args[0].IsNil() {
			frames := interp.debugStackSnapshot(1)
			if len(frames) == 0 {
				return []Value{NilValue()}, nil
			}
			return []Value{TableValue(debugFrameTable(len(frames), frames[len(frames)-1]))}, nil
		}
		if args[0].IsNumber() {
			level := int(toInt(args[0]))
			if level < 0 {
				return []Value{NilValue()}, nil
			}
			frames := interp.debugStackSnapshot(1)
			idx := len(frames) - 1 - level
			if idx < 0 || idx >= len(frames) {
				return []Value{NilValue()}, nil
			}
			return []Value{TableValue(debugFrameTable(idx+1, frames[idx]))}, nil
		}
		if !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'debug.info' (function, number, or nil expected)")
		}
		return []Value{TableValue(debugFunctionInfo(args[0]))}, nil
	})

	set("globals", func(args []Value) ([]Value, error) {
		out := NewTable()
		for name, uv := range interp.globals.vars {
			out.RawSetString(name, uv.Get())
		}
		return []Value{TableValue(out)}, nil
	})

	set("value", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'debug.value' (value expected)")
		}
		out := NewTable()
		out.RawSetString("type", StringValue(args[0].TypeName()))
		out.RawSetString("text", StringValue(args[0].String()))
		out.RawSetString("truthy", BoolValue(args[0].Truthy()))
		out.RawSetString("raw", StringValue(fmt.Sprintf("0x%x", args[0].Raw())))
		return []Value{TableValue(out)}, nil
	})

	set("goStack", func(args []Value) ([]Value, error) {
		buf := make([]byte, 64<<10)
		n := runtime.Stack(buf, false)
		return []Value{StringValue(string(buf[:n]))}, nil
	})

	set("setHook", func(args []Value) ([]Value, error) {
		if len(args) == 0 || args[0].IsNil() {
			interp.debugHook = NilValue()
			interp.debugOpts = DebugHookOptions{}
			return nil, nil
		}
		if !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'debug.setHook' (function or nil expected)")
		}
		optsArg := NilValue()
		if len(args) > 1 {
			optsArg = args[1]
		}
		opts, err := ParseDebugHookOptions(optsArg)
		if err != nil {
			return nil, err
		}
		interp.debugHook = args[0]
		interp.debugOpts = opts
		return nil, nil
	})

	set("getHook", func(args []Value) ([]Value, error) {
		if interp.debugHook.IsNil() {
			return []Value{NilValue()}, nil
		}
		return []Value{interp.debugHook, TableValue(DebugHookOptionsTable(interp.debugOpts))}, nil
	})

	set("setSink", func(args []Value) ([]Value, error) {
		prev := interp.debugSink
		if len(args) == 0 || args[0].IsNil() {
			interp.debugSink = NilValue()
		} else {
			if !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'debug.setSink' (function or nil expected)")
			}
			interp.debugSink = args[0]
		}
		if prev.IsNil() {
			return []Value{NilValue()}, nil
		}
		return []Value{prev}, nil
	})

	set("emit", func(args []Value) ([]Value, error) {
		if len(args) == 0 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'debug.emit' (string expected)")
		}
		data := NilValue()
		if len(args) > 1 {
			data = args[1]
		}
		event := DebugEventTable("emit", "diagnostic", args[0].Str(), data)
		event.RawSetString("event", args[0])
		if err := interp.emitDebugHook("emit", "diagnostic", args[0].Str(), data); err != nil {
			return nil, err
		}
		if err := interp.emitDebugSink(event); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	})

	return t
}

func debugFramesTable(frames []DebugFrame) *Table {
	out := NewTable()
	for i, frame := range frames {
		out.RawSet(IntValue(int64(i+1)), TableValue(debugFrameTable(i+1, frame)))
	}
	return out
}

func debugFrameTable(index int, frame DebugFrame) *Table {
	out := NewTable()
	out.RawSetString("index", IntValue(int64(index)))
	out.RawSetString("name", StringValue(frame.Name))
	out.RawSetString("kind", StringValue(frame.Kind))
	return out
}

func debugFunctionInfo(fn Value) *Table {
	out := NewTable()
	out.RawSetString("type", StringValue("function"))
	if gf := fn.GoFunction(); gf != nil {
		out.RawSetString("name", StringValue(gf.Name))
		out.RawSetString("kind", StringValue("native"))
		return out
	}
	if cl := fn.Closure(); cl != nil && cl.Proto != nil {
		name := cl.Proto.Name
		if name == "" {
			name = "<anonymous>"
		}
		out.RawSetString("name", StringValue(name))
		out.RawSetString("kind", StringValue("script"))
		out.RawSetString("params", IntValue(int64(len(cl.Proto.Params))))
		out.RawSetString("vararg", BoolValue(cl.Proto.HasVarArg))
		out.RawSetString("upvalues", IntValue(int64(len(cl.Upvalues))))
		return out
	}
	out.RawSetString("name", StringValue("<unknown>"))
	out.RawSetString("kind", StringValue("unknown"))
	return out
}

func formatDebugTraceback(message string, frames []DebugFrame) string {
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
		b.WriteString(" ")
		b.WriteString(frame.Name)
	}
	return b.String()
}
