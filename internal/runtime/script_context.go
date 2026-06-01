package runtime

import (
	"sync"
)

type ScriptContextState struct {
	done   *Channel
	once   sync.Once
	mu     sync.RWMutex
	err    Value
	closed bool
}

func NewScriptContextState() *ScriptContextState {
	return &ScriptContextState{
		done: NewChannel(1),
		err:  NilValue(),
	}
}

func NewScriptContextTable(state *ScriptContextState) *Table {
	t := NewTable()
	if state == nil {
		t.RawSetString("done", ChannelValue(NewChannel(1)))
		t.RawSetString("cancelled", FunctionValue(&GoFunction{
			Name: "context.cancelled",
			Fn: func(args []Value) ([]Value, error) {
				return []Value{BoolValue(false)}, nil
			},
		}))
		t.RawSetString("err", FunctionValue(&GoFunction{
			Name: "context.err",
			Fn: func(args []Value) ([]Value, error) {
				return []Value{NilValue()}, nil
			},
		}))
		return t
	}
	t.RawSetString("done", ChannelValue(state.done))
	t.RawSetString("cancel", ScriptContextCancelValue(state))
	t.RawSetString("cancelled", FunctionValue(&GoFunction{
		Name: "context.cancelled",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{BoolValue(state.Cancelled())}, nil
		},
	}))
	t.RawSetString("err", FunctionValue(&GoFunction{
		Name: "context.err",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{state.ErrorValue()}, nil
		},
	}))
	return t
}

func ScriptContextCancelValue(state *ScriptContextState) Value {
	return FunctionValue(&GoFunction{
		Name: "context.cancel",
		Fn: func(args []Value) ([]Value, error) {
			state.Cancel(StringValue("cancelled"))
			return []Value{NilValue()}, nil
		},
	})
}

// ScriptContextDoneAndErr returns the context done channel and err function for
// a script context table. It is intentionally narrow so stdlib bindings
// modules can use context cancellation without depending on context/channel internals.
func ScriptContextDoneAndErr(v Value) (*Channel, Value, bool) {
	if !v.IsTable() {
		return nil, NilValue(), false
	}
	t := v.Table()
	done := t.RawGetString("done")
	if !done.IsChannel() {
		return nil, NilValue(), false
	}
	return done.Channel(), t.RawGetString("err"), true
}

// ScriptContextErrValue evaluates a script context err function using the same
// fallback behavior as runtime-owned stdlib helpers.
func ScriptContextErrValue(errFn Value) Value {
	gf := errFn.GoFunction()
	if gf == nil || gf.Fn == nil {
		return StringValue("cancelled")
	}
	vals, err := gf.Fn(nil)
	if err != nil || len(vals) == 0 || vals[0].IsNil() {
		return StringValue("cancelled")
	}
	return vals[0]
}

func (s *ScriptContextState) Cancel(reason Value) {
	s.once.Do(func() {
		s.mu.Lock()
		s.err = reason
		s.closed = true
		s.mu.Unlock()
		_ = s.done.Close()
	})
}

func (s *ScriptContextState) Cancelled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *ScriptContextState) ErrorValue() Value {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}
