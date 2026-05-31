package modules

import (
	"fmt"
	goruntime "runtime"

	"github.com/never-labs/gscript/internal/runtime"
)

type TestkitRuntime interface {
	TestkitAccessEnabled() bool
	TestkitMemorySnapshot() *runtime.Table
}

type TestkitOptions struct {
	Runtime TestkitRuntime
	Call    runtime.ScriptFunctionCaller
}

func BuildTestkit(opts TestkitOptions) *runtime.Table {
	t := runtime.NewTable()
	tk := opts.Runtime

	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSetString(name, runtime.FunctionValue(&runtime.GoFunction{
			Name: "testkit." + name,
			Fn: func(args []runtime.Value) ([]runtime.Value, error) {
				if tk != nil && !tk.TestkitAccessEnabled() {
					return nil, fmt.Errorf("testkit access disabled")
				}
				return fn(args)
			},
		}))
	}

	snapshot := func() *runtime.Table {
		if tk == nil {
			return runtime.NewTable()
		}
		return tk.TestkitMemorySnapshot()
	}

	set("memory", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.TableValue(snapshot())}, nil
	})
	set("snapshot", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.TableValue(snapshot())}, nil
	})
	set("diff", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.diff' (memory snapshot table expected)")
		}
		after := snapshot()
		if len(args) > 1 && !args[1].IsNil() {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'testkit.diff' (memory snapshot table expected)")
			}
			after = args[1].Table()
		}
		return []runtime.Value{runtime.TableValue(runtime.TestkitMemoryDiff(args[0].Table(), after))}, nil
	})
	set("checkMemory", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.checkMemory' (memory snapshot table expected)")
		}
		if len(args) > 1 && !args[1].IsNil() && !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument #2 to 'testkit.checkMemory' (options table expected)")
		}
		optsTable := (*runtime.Table)(nil)
		if len(args) > 1 && args[1].IsTable() {
			optsTable = args[1].Table()
			if optsTable.RawGetString("collect").Truthy() {
				goruntime.GC()
			}
		}
		report := runtime.TestkitMemoryDiff(args[0].Table(), snapshot())
		ok := true
		if optsTable != nil {
			if limit, has := runtime.TestkitOptionalInt(optsTable, "maxAllocBytesGrowth"); has && runtime.TestkitTableInt(report, "allocBytes") > limit {
				ok = false
			}
			if limit, has := runtime.TestkitOptionalInt(optsTable, "maxHeapObjectsGrowth"); has && runtime.TestkitTableInt(report, "heapObjects") > limit {
				ok = false
			}
			if limit, has := runtime.TestkitOptionalInt(optsTable, "maxRootLogGrowth"); has && runtime.TestkitTableInt(report, "rootLog") > limit {
				ok = false
			}
		}
		report.RawSetString("ok", runtime.BoolValue(ok))
		return []runtime.Value{runtime.BoolValue(ok), runtime.TableValue(report)}, nil
	})
	set("value", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.value' (value expected)")
		}
		return []runtime.Value{runtime.TableValue(runtime.TestkitValueInfo(args[0]))}, nil
	})
	set("typeOf", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.typeOf' (value expected)")
		}
		return []runtime.Value{runtime.StringValue(args[0].TypeName())}, nil
	})
	set("equal", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument #%d to 'testkit.equal' (value expected)", len(args)+1)
		}
		return []runtime.Value{runtime.BoolValue(args[0].Equal(args[1]))}, nil
	})
	t.RawSetString("protect", runtime.FunctionValue(runtime.BuildTestkitProtectFunction(opts.Call, func() bool {
		return tk == nil || tk.TestkitAccessEnabled()
	})))
	set("functionInfo", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.functionInfo' (function expected)")
		}
		info := runtime.DebugFunctionInfo(args[0])
		info.RawSetString("identity", runtime.StringValue(runtime.TestkitIdentity(args[0])))
		info.RawSetString("raw", runtime.StringValue(fmt.Sprintf("0x%x", args[0].Raw())))
		return []runtime.Value{runtime.TableValue(info)}, nil
	})
	set("sameFunction", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument #%d to 'testkit.sameFunction' (function expected)", len(args)+1)
		}
		return []runtime.Value{runtime.BoolValue(args[0].IsFunction() && args[1].IsFunction() && args[0].Equal(args[1]))}, nil
	})

	return t
}
