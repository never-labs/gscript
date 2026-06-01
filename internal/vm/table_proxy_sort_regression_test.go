package vm

import (
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVMTableProxyMoveConcatUnpackThroughMetamethods(t *testing.T) {
	g := compileAndRun(t, `
		src := {[1]: "a", [2]: "b", [3]: "c", [4]: "d"}
		dst := {}
		reads := 0
		writes := 0
		proxySrc := setmetatable({}, {
			__len: func() { return 4 },
			__index: func(_, k) {
				reads = reads + 1
				return src[k]
			},
		})
		proxyDst := setmetatable({}, {
			__newindex: func(_, k, v) {
				writes = writes + 1
				dst[k] = v
			},
		})

		moved := table.move(proxySrc, 2, 4, 1, proxyDst)
		joined := table.concat(proxySrc, ":", 1, 3)
		u2, u3, u4 := table.unpack(proxySrc, 2, 4)

		r1 := dst[1]
		r2 := dst[2]
		r3 := dst[3]
		sameDst := moved == proxyDst
	`)
	expectGlobalString(t, g, "r1", "b")
	expectGlobalString(t, g, "r2", "c")
	expectGlobalString(t, g, "r3", "d")
	expectGlobalString(t, g, "joined", "a:b:c")
	expectGlobalString(t, g, "u2", "b")
	expectGlobalString(t, g, "u3", "c")
	expectGlobalString(t, g, "u4", "d")
	expectGlobalInt(t, g, "reads", 9)
	expectGlobalInt(t, g, "writes", 3)
	expectGlobalBool(t, g, "sameDst", true)
}

func TestVMTableSortProxyUsesVMClosureComparator(t *testing.T) {
	g := compileAndRun(t, `
		backing := {[1]: 4, [2]: 1, [3]: 3, [4]: 2}
		reads := 0
		writes := 0
		calls := 0
		proxy := setmetatable({}, {
			__len: func() { return 4 },
			__index: func(_, k) {
				reads = reads + 1
				return backing[k]
			},
			__newindex: func(_, k, v) {
				writes = writes + 1
				backing[k] = v
			},
		})
		threshold := 0
		func descending(a, b) {
			calls = calls + 1
			return (a + threshold) > (b + threshold)
		}

		table.sort(proxy, descending)
		out := table.concat(backing, ",")
		rawProxyEmpty := next(proxy) == nil
	`)
	expectGlobalString(t, g, "out", "4,3,2,1")
	expectGlobalInt(t, g, "reads", 4)
	expectGlobalInt(t, g, "writes", 4)
	expectGlobalBool(t, g, "rawProxyEmpty", true)
	if calls := g["calls"]; !calls.IsInt() || calls.Int() == 0 {
		t.Fatalf("calls = %v, want VM comparator to be invoked", calls)
	}
}

func TestVMTableSortExposesFastArgPaths(t *testing.T) {
	globals := vmtest.NewInterpreterGlobals()
	New(globals)
	sortFn := globals["table"].Table().RawGetString("sort").GoFunction()
	if sortFn == nil || sortFn.FastArg1 == nil || sortFn.FastArg2 == nil {
		t.Fatalf("table.sort missing VM FastArg paths: %#v", sortFn)
	}

	tbl := runtime.NewTable()
	tbl.RawSet(runtime.IntValue(1), runtime.IntValue(3))
	tbl.RawSet(runtime.IntValue(2), runtime.IntValue(1))
	tbl.RawSet(runtime.IntValue(3), runtime.IntValue(2))
	if _, err := sortFn.FastArg1(runtime.TableValue(tbl)); err != nil {
		t.Fatalf("table.sort FastArg1: %v", err)
	}
	if got := tbl.RawGet(runtime.IntValue(1)).Int(); got != 1 {
		t.Fatalf("FastArg1 sorted first = %d, want 1", got)
	}

	desc := runtime.FunctionValue(&runtime.GoFunction{
		Name: "desc",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			return []runtime.Value{runtime.BoolValue(args[0].Int() > args[1].Int())}, nil
		},
	})
	if _, err := sortFn.FastArg2(runtime.TableValue(tbl), desc); err != nil {
		t.Fatalf("table.sort FastArg2: %v", err)
	}
	if got := tbl.RawGet(runtime.IntValue(1)).Int(); got != 3 {
		t.Fatalf("FastArg2 sorted first = %d, want 3", got)
	}
}

func TestVMTableSortComparatorNonBoolReturn(t *testing.T) {
	g := compileAndRun(t, `
		values := {
			{name: "b", rank: 2},
			{name: "a", rank: 1},
			{name: "c", rank: 3},
		}
		calls := 0
		func byRank(a, b) {
			calls = calls + 1
			if a.rank < b.rank {
				return "truthy-less"
			}
			return nil
		}
		table.sort(values, byRank)
		out := values[1].name .. "," .. values[2].name .. "," .. values[3].name
	`)
	expectGlobalString(t, g, "out", "a,b,c")
	if calls := g["calls"]; !calls.IsInt() || calls.Int() == 0 {
		t.Fatalf("calls = %v, want VM comparator to be invoked", calls)
	}
}

func TestVMTableSortComparatorErrorPropagates(t *testing.T) {
	err := compileAndRunExpectError(t, `
		t := {3, 2, 1}
		func bad(a, b) {
			if a == 2 || b == 2 {
				error("sort-callback-boom")
			}
			return a < b
		}
		table.sort(t, bad)
	`)
	if err == nil {
		t.Fatal("table.sort comparator error was not propagated")
	}
	if !strings.Contains(err.Error(), "sort-callback-boom") {
		t.Fatalf("error = %v, want sort-callback-boom", err)
	}
}

func TestVMTableSortInvalidOrderFunction(t *testing.T) {
	err := compileAndRunExpectError(t, `
		t := {1, 2, 3, 4}
		func bad(a, b) {
			return true
		}
		table.sort(t, bad)
	`)
	if err == nil {
		t.Fatal("table.sort invalid order function did not fail")
	}
	if !strings.Contains(err.Error(), "invalid order function") {
		t.Fatalf("error = %v, want invalid order function", err)
	}
}

func TestVMTableSortDefaultUsesLtMetamethod(t *testing.T) {
	g := compileAndRun(t, `
		mt := {
			__lt: func(a, b) {
				return a.rank < b.rank
			},
		}
		values := {
			setmetatable({name: "b", rank: 2}, mt),
			setmetatable({name: "a", rank: 1}, mt),
			setmetatable({name: "c", rank: 3}, mt),
		}
		table.sort(values)
		out := values[1].name .. "," .. values[2].name .. "," .. values[3].name
	`)
	expectGlobalString(t, g, "out", "a,b,c")
}

func TestVMTableSortProxyUsesDefaultOrdering(t *testing.T) {
	g := compileAndRun(t, `
		backing := {[1]: 4, [2]: 1, [3]: 3, [4]: 2}
		lenCalls := 0
		reads := 0
		writes := 0
		proxy := setmetatable({}, {
			__len: func() {
				lenCalls = lenCalls + 1
				return 4
			},
			__index: func(_, k) {
				reads = reads + 1
				return backing[k]
			},
			__newindex: func(_, k, v) {
				writes = writes + 1
				backing[k] = v
			},
		})

		table.sort(proxy)
		out := table.concat(backing, ",")
		rawProxyEmpty := next(proxy) == nil
	`)
	expectGlobalString(t, g, "out", "1,2,3,4")
	expectGlobalInt(t, g, "lenCalls", 1)
	expectGlobalInt(t, g, "reads", 4)
	expectGlobalInt(t, g, "writes", 4)
	expectGlobalBool(t, g, "rawProxyEmpty", true)
}
