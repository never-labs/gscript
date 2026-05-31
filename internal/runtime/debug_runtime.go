package runtime

import (
	"fmt"

	"github.com/never-labs/gscript/internal/debugstate"
)

// DebugFrame describes one active runtime call in GScript-shaped terms.
type DebugFrame = debugstate.Frame

// DebugHookOptions describes the coarse-grained GScript debug hook filters.
type DebugHookOptions = debugstate.HookOptions

func DefaultDebugHookOptions() DebugHookOptions {
	return debugstate.DefaultHookOptions()
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
	return debugstate.HookWants(opts, eventType, kind)
}

func (interp *Interpreter) pushDebugFrame(name, kind string) {
	interp.callStack = append(interp.callStack, DebugFrame{Name: name, Kind: kind})
}

func (interp *Interpreter) pushDebugFrameWithSource(name, kind, sourceName string, line, column int) {
	frame := DebugFrame{
		Name:       name,
		Kind:       kind,
		SourceName: sourceName,
		Line:       line,
		Column:     column,
	}
	interp.callStack = append(interp.callStack, frame)
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

func (interp *Interpreter) DebugAccessEnabled() bool {
	return interp == nil || interp.debugAccess
}

func (interp *Interpreter) DebugStackSnapshot(skip int) []DebugFrame {
	if interp == nil {
		return nil
	}
	return interp.debugStackSnapshot(skip)
}

func (interp *Interpreter) DebugGlobalsSnapshot() *Table {
	out := NewTable()
	if interp == nil || interp.globals == nil {
		return out
	}
	for name, uv := range interp.globals.vars {
		out.RawSetString(name, uv.Get())
	}
	return out
}

func (interp *Interpreter) SetDebugHookValue(hook Value, opts DebugHookOptions) {
	if interp == nil {
		return
	}
	interp.debugHook = hook
	interp.debugOpts = opts
}

func (interp *Interpreter) ClearDebugHookValue() {
	if interp == nil {
		return
	}
	interp.debugHook = NilValue()
	interp.debugOpts = DebugHookOptions{}
}

func (interp *Interpreter) DebugHookValue() (Value, DebugHookOptions) {
	if interp == nil || interp.debugHook.IsNil() {
		return NilValue(), DebugHookOptions{}
	}
	return interp.debugHook, interp.debugOpts
}

func (interp *Interpreter) SetDebugSinkValue(sink Value) Value {
	if interp == nil {
		return NilValue()
	}
	prev := interp.debugSink
	if sink.IsNil() {
		interp.debugSink = NilValue()
	} else {
		interp.debugSink = sink
	}
	if prev.IsNil() {
		return NilValue()
	}
	return prev
}

func (interp *Interpreter) EmitDebugHook(eventType, kind, name string, data Value) error {
	if interp == nil {
		return nil
	}
	return interp.emitDebugHook(eventType, kind, name, data)
}

func (interp *Interpreter) EmitDebugSink(event *Table) error {
	if interp == nil {
		return nil
	}
	return interp.emitDebugSink(event)
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

func DebugFramesTable(frames []DebugFrame) *Table {
	out := NewTable()
	for i, frame := range frames {
		out.RawSet(IntValue(int64(i+1)), TableValue(DebugFrameTable(i+1, frame)))
	}
	return out
}

func DebugFrameTable(index int, frame DebugFrame) *Table {
	out := NewTable()
	out.RawSetString("index", IntValue(int64(index)))
	out.RawSetString("name", StringValue(frame.Name))
	out.RawSetString("kind", StringValue(frame.Kind))
	if frame.SourceName != "" {
		out.RawSetString("sourceName", StringValue(frame.SourceName))
	}
	if frame.Line > 0 {
		out.RawSetString("line", IntValue(int64(frame.Line)))
		out.RawSetString("column", IntValue(int64(frame.Column)))
	}
	return out
}

func DebugFunctionInfo(fn Value) *Table {
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
		if cl.Proto.SourceName != "" {
			out.RawSetString("sourceName", StringValue(cl.Proto.SourceName))
		}
		if cl.Proto.Line > 0 {
			out.RawSetString("line", IntValue(int64(cl.Proto.Line)))
			out.RawSetString("column", IntValue(int64(cl.Proto.Column)))
		}
		out.RawSetString("params", IntValue(int64(len(cl.Proto.Params))))
		out.RawSetString("vararg", BoolValue(cl.Proto.HasVarArg))
		out.RawSetString("upvalues", IntValue(int64(len(cl.Upvalues))))
		return out
	}
	out.RawSetString("name", StringValue("<unknown>"))
	out.RawSetString("kind", StringValue("unknown"))
	return out
}

func FormatDebugTraceback(message string, frames []DebugFrame) string {
	return debugstate.FormatTraceback(message, frames)
}

func debugFramesTable(frames []DebugFrame) *Table { return DebugFramesTable(frames) }

func debugFrameTable(index int, frame DebugFrame) *Table { return DebugFrameTable(index, frame) }

func debugFunctionInfo(fn Value) *Table { return DebugFunctionInfo(fn) }

func formatDebugTraceback(message string, frames []DebugFrame) string {
	return FormatDebugTraceback(message, frames)
}
