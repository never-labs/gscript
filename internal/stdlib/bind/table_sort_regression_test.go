package bind

import (
	"strings"
	"testing"
)

func TestRuntimeTableSortComparatorNonBoolReturn(t *testing.T) {
	interp := runProgram(t, `
		values := {
			{name: "b", rank: 2},
			{name: "a", rank: 1},
			{name: "c", rank: 3},
		}
		calls := 0
		table.sort(values, func(a, b) {
			calls = calls + 1
			if a.rank < b.rank {
				return "truthy-less"
			}
			return nil
		})
		out := values[1].name .. "," .. values[2].name .. "," .. values[3].name
	`)
	if got := interp.GetGlobal("out").Str(); got != "a,b,c" {
		t.Fatalf("out = %q, want a,b,c", got)
	}
	if got := interp.GetGlobal("calls").Int(); got == 0 {
		t.Fatalf("calls = %d, want comparator to be called", got)
	}
}

func TestRuntimeTableSortComparatorCallbackErrorPropagates(t *testing.T) {
	err := runProgramExpectError(t, `
		t := {3, 2, 1}
		table.sort(t, func(a, b) {
			if a == 2 || b == 2 {
				error("sort-callback-boom")
			}
			return a < b
		})
	`)
	if err == nil {
		t.Fatal("table.sort comparator error was not propagated")
	}
	if !strings.Contains(err.Error(), "sort-callback-boom") {
		t.Fatalf("error = %v, want sort-callback-boom", err)
	}
}

func TestRuntimeTableSortInvalidOrderFunction(t *testing.T) {
	err := runProgramExpectError(t, `
		t := {1, 2, 3, 4}
		table.sort(t, func(a, b) {
			return true
		})
	`)
	if err == nil {
		t.Fatal("table.sort invalid order function did not fail")
	}
	if !strings.Contains(err.Error(), "invalid order function") {
		t.Fatalf("error = %v, want invalid order function", err)
	}
}

func TestRuntimeTableSortDefaultUsesLtMetamethod(t *testing.T) {
	interp := runProgram(t, `
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
	if got := interp.GetGlobal("out").Str(); got != "a,b,c" {
		t.Fatalf("out = %q, want a,b,c", got)
	}
}

func TestRuntimeTableSortProxyUsesMetamethods(t *testing.T) {
	interp := runProgram(t, `
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
	if got := interp.GetGlobal("out").Str(); got != "1,2,3,4" {
		t.Fatalf("out = %q, want 1,2,3,4", got)
	}
	if got := interp.GetGlobal("lenCalls").Int(); got != 1 {
		t.Fatalf("lenCalls = %d, want 1", got)
	}
	if got := interp.GetGlobal("reads").Int(); got != 4 {
		t.Fatalf("reads = %d, want 4", got)
	}
	if got := interp.GetGlobal("writes").Int(); got != 4 {
		t.Fatalf("writes = %d, want 4", got)
	}
	if got := interp.GetGlobal("rawProxyEmpty"); !got.IsBool() || !got.Bool() {
		t.Fatalf("rawProxyEmpty = %v, want true", got)
	}
}
