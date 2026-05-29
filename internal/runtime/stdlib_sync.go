package runtime

import (
	"fmt"
	"sync"
)

type scriptWaitGroup struct {
	wg sync.WaitGroup
}

func buildSyncLib() *Table {
	t := NewTable()
	t.RawSetString("waitgroup", FunctionValue(&GoFunction{
		Name: "sync.waitgroup",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{TableValue(newScriptWaitGroupTable())}, nil
		},
	}))
	return t
}

func newScriptWaitGroupTable() *Table {
	state := &scriptWaitGroup{}
	t := NewTable()
	t.RawSetString("add", FunctionValue(&GoFunction{
		Name: "sync.waitgroup.add",
		Fn: func(args []Value) ([]Value, error) {
			args = stripMethodSelf(args)
			if len(args) < 1 || !args[0].IsInt() {
				return nil, fmt.Errorf("sync.waitgroup.add: delta must be an integer")
			}
			if err := waitGroupAdd(state, int(args[0].Int())); err != nil {
				return nil, err
			}
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("done", FunctionValue(&GoFunction{
		Name: "sync.waitgroup.done",
		Fn: func(args []Value) ([]Value, error) {
			if err := waitGroupAdd(state, -1); err != nil {
				return nil, err
			}
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("wait", FunctionValue(&GoFunction{
		Name: "sync.waitgroup.wait",
		Fn: func(args []Value) ([]Value, error) {
			state.wg.Wait()
			return []Value{NilValue()}, nil
		},
	}))
	return t
}

func stripMethodSelf(args []Value) []Value {
	if len(args) > 0 && args[0].IsTable() {
		return args[1:]
	}
	return args
}

func waitGroupAdd(state *scriptWaitGroup, delta int) (err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("sync.waitgroup: invalid counter state")
		}
	}()
	state.wg.Add(delta)
	return nil
}
