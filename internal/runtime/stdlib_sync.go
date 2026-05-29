package runtime

import (
	"fmt"
	"sync"
)

type scriptWaitGroup struct {
	wg sync.WaitGroup
}

type scriptOnce struct {
	once sync.Once
	err  error
}

type SyncFunctionCaller func(Value, []Value) ([]Value, error)

func BuildSyncLibWithCaller(call SyncFunctionCaller) *Table {
	t := NewTable()
	t.RawSetString("waitgroup", FunctionValue(&GoFunction{
		Name: "sync.waitgroup",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{TableValue(newScriptWaitGroupTable())}, nil
		},
	}))
	t.RawSetString("mutex", FunctionValue(&GoFunction{
		Name: "sync.mutex",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{TableValue(newScriptMutexTable())}, nil
		},
	}))
	t.RawSetString("rwmutex", FunctionValue(&GoFunction{
		Name: "sync.rwmutex",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{TableValue(newScriptRWMutexTable())}, nil
		},
	}))
	t.RawSetString("once", FunctionValue(&GoFunction{
		Name: "sync.once",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{TableValue(newScriptOnceTable(call))}, nil
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

func newScriptMutexTable() *Table {
	var mu sync.Mutex
	t := NewTable()
	t.RawSetString("lock", FunctionValue(&GoFunction{
		Name: "sync.mutex.lock",
		Fn: func(args []Value) ([]Value, error) {
			mu.Lock()
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("unlock", FunctionValue(&GoFunction{
		Name: "sync.mutex.unlock",
		Fn: func(args []Value) (out []Value, err error) {
			defer func() {
				if recover() != nil {
					out = nil
					err = fmt.Errorf("sync.mutex.unlock: mutex is not locked")
				}
			}()
			mu.Unlock()
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("trylock", FunctionValue(&GoFunction{
		Name: "sync.mutex.trylock",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{BoolValue(mu.TryLock())}, nil
		},
	}))
	return t
}

func newScriptRWMutexTable() *Table {
	var mu sync.RWMutex
	t := NewTable()
	t.RawSetString("lock", FunctionValue(&GoFunction{
		Name: "sync.rwmutex.lock",
		Fn: func(args []Value) ([]Value, error) {
			mu.Lock()
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("unlock", FunctionValue(&GoFunction{
		Name: "sync.rwmutex.unlock",
		Fn: func(args []Value) (out []Value, err error) {
			defer func() {
				if recover() != nil {
					out = nil
					err = fmt.Errorf("sync.rwmutex.unlock: mutex is not locked")
				}
			}()
			mu.Unlock()
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("rlock", FunctionValue(&GoFunction{
		Name: "sync.rwmutex.rlock",
		Fn: func(args []Value) ([]Value, error) {
			mu.RLock()
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("runlock", FunctionValue(&GoFunction{
		Name: "sync.rwmutex.runlock",
		Fn: func(args []Value) (out []Value, err error) {
			defer func() {
				if recover() != nil {
					out = nil
					err = fmt.Errorf("sync.rwmutex.runlock: mutex is not read-locked")
				}
			}()
			mu.RUnlock()
			return []Value{NilValue()}, nil
		},
	}))
	t.RawSetString("trylock", FunctionValue(&GoFunction{
		Name: "sync.rwmutex.trylock",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{BoolValue(mu.TryLock())}, nil
		},
	}))
	t.RawSetString("tryrlock", FunctionValue(&GoFunction{
		Name: "sync.rwmutex.tryrlock",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{BoolValue(mu.TryRLock())}, nil
		},
	}))
	return t
}

func newScriptOnceTable(call SyncFunctionCaller) *Table {
	state := &scriptOnce{}
	t := NewTable()
	t.RawSetString("do", FunctionValue(&GoFunction{
		Name: "sync.once.do",
		Fn: func(args []Value) ([]Value, error) {
			args = stripMethodSelf(args)
			if len(args) < 1 || !args[0].IsFunction() {
				return nil, fmt.Errorf("sync.once.do: function expected")
			}
			fn := args[0]
			state.once.Do(func() {
				_, state.err = call(fn, nil)
			})
			if state.err != nil {
				return nil, state.err
			}
			return []Value{NilValue()}, nil
		},
	}))
	return t
}
