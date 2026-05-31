package vm

import (
	"strings"
	"testing"
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
