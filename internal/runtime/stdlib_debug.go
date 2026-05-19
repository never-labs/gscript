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
