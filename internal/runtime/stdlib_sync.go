package runtime

import (
	"fmt"
	"sync"

	stdlibsync "github.com/never-labs/gscript/internal/stdlib/host/sync"
)

type scriptWaitGroup struct {
	wg sync.WaitGroup
}

type scriptTaskGroup struct {
	wg     sync.WaitGroup
	errs   stdlibsync.TaskErrors
	ctx    Value
	cancel Value
	call   ScriptFunctionCaller
}

type scriptOnce struct {
	once sync.Once
	err  error
}

type SyncTaskLauncher func(Value, []Value, func(error))

func BuildSyncLibWithCaller(call ScriptFunctionCaller) *Table {
	return BuildSyncLibWithTaskLauncher(call, defaultSyncTaskLauncher(call))
}

func BuildSyncLibWithTaskLauncher(call ScriptFunctionCaller, launch SyncTaskLauncher) *Table {
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
	t.RawSetString("group", FunctionValue(&GoFunction{
		Name: "sync.group",
		Fn: func(args []Value) ([]Value, error) {
			return syncGroupFromArgs(call, launch, args)
		},
	}))
	return t
}

func syncGroupFromArgs(call ScriptFunctionCaller, launch SyncTaskLauncher, args []Value) ([]Value, error) {
	if len(args) == 0 || args[0].IsNil() {
		state := newScriptContextState()
		ctx := TableValue(newScriptContextTable(state))
		cancel := scriptContextCancelValue(state)
		return []Value{TableValue(newScriptTaskGroupTable(call, launch, ctx, cancel))}, nil
	}
	if !args[0].IsTable() {
		return nil, fmt.Errorf("sync.group: context table expected")
	}
	ctx := args[0]
	cancel := NilValue()
	if c := ctx.Table().RawGetString("cancel"); c.IsFunction() {
		cancel = c
	}
	return []Value{TableValue(newScriptTaskGroupTable(call, launch, ctx, cancel))}, nil
}

func defaultSyncTaskLauncher(call ScriptFunctionCaller) SyncTaskLauncher {
	return func(fn Value, args []Value, done func(error)) {
		go func() {
			var err error
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
				done(err)
			}()
			_, err = call(fn, args)
		}()
	}
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
	return stdlibsync.AddWaitGroup(&state.wg, delta)
}

func newScriptTaskGroupTable(call ScriptFunctionCaller, launch SyncTaskLauncher, ctx, cancel Value) *Table {
	state := &scriptTaskGroup{ctx: ctx, cancel: cancel, call: call}
	if launch == nil {
		launch = defaultSyncTaskLauncher(func(Value, []Value) ([]Value, error) {
			return nil, fmt.Errorf("sync.group: no task launcher configured")
		})
	}
	t := NewTable()
	startFn := FunctionValue(&GoFunction{
		Name: "sync.group.go",
		Fn: func(args []Value) ([]Value, error) {
			args = stripMethodSelf(args)
			if len(args) < 1 || !args[0].IsFunction() {
				return nil, fmt.Errorf("sync.group.start: function expected")
			}
			fn := args[0]
			taskArgs := append([]Value(nil), args[1:]...)
			if !state.ctx.IsNil() {
				taskArgs = append([]Value{state.ctx}, taskArgs...)
			}
			state.wg.Add(1)
			launch(fn, taskArgs, func(err error) {
				state.record(err)
				state.wg.Done()
			})
			return []Value{NilValue()}, nil
		},
	})
	t.RawSetString("go", startFn)
	t.RawSetString("start", startFn)
	t.RawSetString("wait", FunctionValue(&GoFunction{
		Name: "sync.group.wait",
		Fn: func(args []Value) ([]Value, error) {
			state.wg.Wait()
			if err := state.error(); err != nil {
				return []Value{BoolValue(false), StringValue(err.Error()), IntValue(int64(state.errorCount()))}, nil
			}
			return []Value{BoolValue(true), NilValue(), IntValue(0)}, nil
		},
	}))
	t.RawSetString("context", FunctionValue(&GoFunction{
		Name: "sync.group.context",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{state.ctx}, nil
		},
	}))
	t.RawSetString("cancel", FunctionValue(&GoFunction{
		Name: "sync.group.cancel",
		Fn: func(args []Value) ([]Value, error) {
			state.cancelGroup()
			return []Value{NilValue()}, nil
		},
	}))
	return t
}

func (g *scriptTaskGroup) record(err error) {
	if g.errs.Record(err) {
		g.cancelGroup()
	}
}

func (g *scriptTaskGroup) error() error {
	return g.errs.Error()
}

func (g *scriptTaskGroup) errorCount() int {
	return g.errs.Count()
}

func (g *scriptTaskGroup) cancelGroup() {
	if g == nil || g.cancel.IsNil() || g.call == nil {
		return
	}
	_, _ = g.call(g.cancel, nil)
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

func newScriptOnceTable(call ScriptFunctionCaller) *Table {
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
