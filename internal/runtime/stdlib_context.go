package runtime

import (
	"fmt"
	"sync"
	"time"
)

type scriptContextState struct {
	done   *Channel
	once   sync.Once
	mu     sync.RWMutex
	err    Value
	closed bool
}

func buildContextLib() *Table {
	t := NewTable()
	t.RawSetString("background", FunctionValue(&GoFunction{
		Name: "context.background",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{TableValue(newScriptContextTable(nil))}, nil
		},
	}))
	t.RawSetString("withCancel", FunctionValue(&GoFunction{
		Name: "context.withCancel",
		Fn: func(args []Value) ([]Value, error) {
			state := newScriptContextState()
			return []Value{TableValue(newScriptContextTable(state)), scriptContextCancelValue(state)}, nil
		},
	}))
	t.RawSetString("withTimeout", FunctionValue(&GoFunction{
		Name: "context.withTimeout",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'context.withTimeout'")
			}
			secs := toFloat(args[0])
			if secs < 0 {
				return nil, fmt.Errorf("bad argument #1 to 'context.withTimeout' (non-negative duration expected)")
			}
			state := newScriptContextState()
			time.AfterFunc(time.Duration(secs*float64(time.Second)), func() {
				state.cancel(StringValue("deadline exceeded"))
			})
			return []Value{TableValue(newScriptContextTable(state)), scriptContextCancelValue(state)}, nil
		},
	}))
	return t
}

func newScriptContextState() *scriptContextState {
	return &scriptContextState{
		done: NewChannel(1),
		err:  NilValue(),
	}
}

func newScriptContextTable(state *scriptContextState) *Table {
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
	t.RawSetString("cancel", scriptContextCancelValue(state))
	t.RawSetString("cancelled", FunctionValue(&GoFunction{
		Name: "context.cancelled",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{BoolValue(state.cancelled())}, nil
		},
	}))
	t.RawSetString("err", FunctionValue(&GoFunction{
		Name: "context.err",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{state.errorValue()}, nil
		},
	}))
	return t
}

func scriptContextCancelValue(state *scriptContextState) Value {
	return FunctionValue(&GoFunction{
		Name: "context.cancel",
		Fn: func(args []Value) ([]Value, error) {
			state.cancel(StringValue("cancelled"))
			return []Value{NilValue()}, nil
		},
	})
}

func (s *scriptContextState) cancel(reason Value) {
	s.once.Do(func() {
		s.mu.Lock()
		s.err = reason
		s.closed = true
		s.mu.Unlock()
		_ = s.done.Close()
	})
}

func (s *scriptContextState) cancelled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *scriptContextState) errorValue() Value {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}
