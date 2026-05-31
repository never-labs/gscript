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
