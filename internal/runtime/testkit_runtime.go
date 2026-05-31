package runtime

import (
	"fmt"
	goruntime "runtime"
)

func (interp *Interpreter) TestkitAccessEnabled() bool {
	return interp == nil || interp.testkitAccess
}

func (interp *Interpreter) TestkitMemorySnapshot() *Table {
	if interp == nil {
		return NewTable()
	}
	return interp.testkitMemorySnapshot()
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

func TestkitMemoryDiff(before, after *Table) *Table {
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

func TestkitValueInfo(v Value) *Table {
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

func TestkitArray(values []Value) *Table {
	t := NewTable()
	for i, v := range values {
		t.RawSet(IntValue(int64(i+1)), v)
	}
	return t
}

func TestkitOptionalInt(t *Table, key string) (int64, bool) {
	v := t.RawGetString(key)
	if v.IsNil() {
		return 0, false
	}
	if !v.IsNumber() {
		return 0, false
	}
	return toInt(v), true
}

func TestkitTableInt(t *Table, key string) int64 {
	v := t.RawGetString(key)
	if !v.IsNumber() {
		return 0
	}
	return toInt(v)
}

func TestkitTableNumber(t *Table, key string) float64 {
	v := t.RawGetString(key)
	if !v.IsNumber() {
		return 0
	}
	return v.Number()
}

func TestkitIdentity(v Value) string {
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

func testkitMemoryDiff(before, after *Table) *Table { return TestkitMemoryDiff(before, after) }

func testkitValueInfo(v Value) *Table { return TestkitValueInfo(v) }

func testkitArray(values []Value) *Table { return TestkitArray(values) }

func testkitOptionalInt(t *Table, key string) (int64, bool) {
	return TestkitOptionalInt(t, key)
}

func testkitTableInt(t *Table, key string) int64 { return TestkitTableInt(t, key) }

func testkitTableNumber(t *Table, key string) float64 { return TestkitTableNumber(t, key) }

func testkitIdentity(v Value) string { return TestkitIdentity(v) }
