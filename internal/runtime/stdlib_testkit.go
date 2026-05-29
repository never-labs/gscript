package runtime

import (
	"errors"
	"fmt"
	goruntime "runtime"
)

func buildTestkitLib(interp *Interpreter) *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{
			Name: "testkit." + name,
			Fn: func(args []Value) ([]Value, error) {
				if interp != nil && !interp.testkitAccess {
					return nil, fmt.Errorf("testkit access disabled")
				}
				return fn(args)
			},
		}))
	}

	set("memory", func(args []Value) ([]Value, error) {
		return []Value{TableValue(interp.testkitMemorySnapshot())}, nil
	})
	set("snapshot", func(args []Value) ([]Value, error) {
		return []Value{TableValue(interp.testkitMemorySnapshot())}, nil
	})
	set("diff", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.diff' (memory snapshot table expected)")
		}
		after := interp.testkitMemorySnapshot()
		if len(args) > 1 && !args[1].IsNil() {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'testkit.diff' (memory snapshot table expected)")
			}
			after = args[1].Table()
		}
		return []Value{TableValue(testkitMemoryDiff(args[0].Table(), after))}, nil
	})
	set("checkMemory", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.checkMemory' (memory snapshot table expected)")
		}
		if len(args) > 1 && !args[1].IsNil() && !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument #2 to 'testkit.checkMemory' (options table expected)")
		}
		opts := (*Table)(nil)
		if len(args) > 1 && args[1].IsTable() {
			opts = args[1].Table()
			if opts.RawGetString("collect").Truthy() {
				goruntime.GC()
			}
		}
		report := testkitMemoryDiff(args[0].Table(), interp.testkitMemorySnapshot())
		ok := true
		if opts != nil {
			if limit, has := testkitOptionalInt(opts, "maxAllocBytesGrowth"); has && testkitTableInt(report, "allocBytes") > limit {
				ok = false
			}
			if limit, has := testkitOptionalInt(opts, "maxHeapObjectsGrowth"); has && testkitTableInt(report, "heapObjects") > limit {
				ok = false
			}
			if limit, has := testkitOptionalInt(opts, "maxRootLogGrowth"); has && testkitTableInt(report, "rootLog") > limit {
				ok = false
			}
		}
		report.RawSetString("ok", BoolValue(ok))
		return []Value{BoolValue(ok), TableValue(report)}, nil
	})
	set("value", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.value' (value expected)")
		}
		return []Value{TableValue(testkitValueInfo(args[0]))}, nil
	})
	set("typeOf", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.typeOf' (value expected)")
		}
		return []Value{StringValue(args[0].TypeName())}, nil
	})
	set("equal", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument #%d to 'testkit.equal' (value expected)", len(args)+1)
		}
		return []Value{BoolValue(args[0].Equal(args[1]))}, nil
	})
	set("protect", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
		}
		if !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
		}
		results, err := interp.callFunction(args[0], args[1:])
		out := NewTable()
		if err != nil {
			out.RawSetString("ok", BoolValue(false))
			var luaErr *LuaError
			if errors.As(err, &luaErr) {
				out.RawSetString("error", luaErr.Value)
			} else {
				out.RawSetString("error", StringValue(err.Error()))
			}
			return []Value{TableValue(out)}, nil
		}
		out.RawSetString("ok", BoolValue(true))
		out.RawSetString("values", TableValue(testkitArray(results)))
		out.RawSetString("n", IntValue(int64(len(results))))
		return []Value{TableValue(out)}, nil
	})
	set("functionInfo", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'testkit.functionInfo' (function expected)")
		}
		info := debugFunctionInfo(args[0])
		info.RawSetString("identity", StringValue(testkitIdentity(args[0])))
		info.RawSetString("raw", StringValue(fmt.Sprintf("0x%x", args[0].Raw())))
		return []Value{TableValue(info)}, nil
	})
	set("sameFunction", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument #%d to 'testkit.sameFunction' (function expected)", len(args)+1)
		}
		return []Value{BoolValue(args[0].IsFunction() && args[1].IsFunction() && args[0].Equal(args[1]))}, nil
	})

	return t
}

func (interp *Interpreter) testkitMemorySnapshot() *Table {
	var stats goruntime.MemStats
	goruntime.ReadMemStats(&stats)
	t := NewTable()
	t.RawSetString("allocBytes", IntValue(int64(stats.Alloc)))
	t.RawSetString("allocKB", FloatValue(float64(stats.Alloc)/1024))
	t.RawSetString("sysBytes", IntValue(int64(stats.Sys)))
	t.RawSetString("heapObjects", IntValue(int64(stats.HeapObjects)))
	t.RawSetString("numGC", IntValue(int64(stats.NumGC)))
	t.RawSetString("rootLog", IntValue(GCRootLogSize()))
	t.RawSetString("rootScanners", IntValue(int64(GCRootScannerCount())))
	t.RawSetString("running", BoolValue(interp.gcRunning))
	t.RawSetString("mode", StringValue(interp.gcMode))
	return t
}

func testkitMemoryDiff(before, after *Table) *Table {
	out := NewTable()
	for _, key := range []string{"allocBytes", "sysBytes", "heapObjects", "numGC", "rootLog"} {
		out.RawSetString(key, IntValue(testkitTableInt(after, key)-testkitTableInt(before, key)))
	}
	beforeKB := testkitTableNumber(before, "allocKB")
	afterKB := testkitTableNumber(after, "allocKB")
	out.RawSetString("allocKB", FloatValue(afterKB-beforeKB))
	out.RawSetString("before", TableValue(before))
	out.RawSetString("after", TableValue(after))
	return out
}

func testkitValueInfo(v Value) *Table {
	out := NewTable()
	out.RawSetString("type", StringValue(v.TypeName()))
	out.RawSetString("text", StringValue(v.String()))
	out.RawSetString("truthy", BoolValue(v.Truthy()))
	out.RawSetString("raw", StringValue(fmt.Sprintf("0x%x", v.Raw())))
	switch {
	case v.IsInt():
		out.RawSetString("numberKind", StringValue("int"))
	case v.IsFloat():
		out.RawSetString("numberKind", StringValue("float"))
	case v.IsString():
		out.RawSetString("len", IntValue(int64(len(v.Str()))))
	case v.IsTable():
		out.RawSetString("len", IntValue(int64(v.Table().Len())))
	case v.IsFunction():
		out.RawSetString("identity", StringValue(testkitIdentity(v)))
		if gf := v.GoFunction(); gf != nil {
			out.RawSetString("functionKind", StringValue("native"))
			out.RawSetString("name", StringValue(gf.Name))
		} else if cl := v.Closure(); cl != nil && cl.Proto != nil {
			out.RawSetString("functionKind", StringValue("script"))
			name := cl.Proto.Name
			if name == "" {
				name = "<anonymous>"
			}
			out.RawSetString("name", StringValue(name))
		}
	}
	return out
}

func testkitArray(values []Value) *Table {
	t := NewTable()
	for i, v := range values {
		t.RawSet(IntValue(int64(i+1)), v)
	}
	return t
}

func testkitOptionalInt(t *Table, key string) (int64, bool) {
	v := t.RawGetString(key)
	if v.IsNil() {
		return 0, false
	}
	if !v.IsNumber() {
		return 0, false
	}
	return toInt(v), true
}

func testkitTableInt(t *Table, key string) int64 {
	v := t.RawGetString(key)
	if !v.IsNumber() {
		return 0
	}
	return toInt(v)
}

func testkitTableNumber(t *Table, key string) float64 {
	v := t.RawGetString(key)
	if !v.IsNumber() {
		return 0
	}
	return v.Number()
}

func testkitIdentity(v Value) string {
	switch {
	case v.IsFunction():
		return fmt.Sprintf("function:%x", v.Raw())
	case v.IsTable():
		return fmt.Sprintf("table:%x", v.Raw())
	case v.IsCoroutine():
		return fmt.Sprintf("coroutine:%x", v.Raw())
	default:
		return fmt.Sprintf("%s:%x", v.TypeName(), v.Raw())
	}
}
