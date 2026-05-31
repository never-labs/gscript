package runtime

import (
	"strings"
	"testing"
)

func TestRuntimeTableProxyMoveUnpackThroughMetamethods(t *testing.T) {
	interp := runProgram(t, `
		src := {[1]: "a", [2]: "b", [3]: "c", [4]: "d"}
		dst := {}
		ops := {}
		opn := 0
		reads := 0
		writes := 0
		proxySrc := setmetatable({}, {
			__len: func() { return 4 },
			__index: func(_, k) {
				reads = reads + 1
				opn = opn + 1
				ops[opn] = "r" .. k
				return src[k]
			},
		})
		proxyDst := setmetatable({}, {
			__newindex: func(_, k, v) {
				writes = writes + 1
				opn = opn + 1
				ops[opn] = "w" .. k .. "=" .. v
				dst[k] = v
			},
		})

		moved := table.move(proxySrc, 2, 4, 1, proxyDst)
		u2, u3, u4 := table.unpack(proxySrc, 2, 4)

		r1 := dst[1]
		r2 := dst[2]
		r3 := dst[3]
		sameDst := moved == proxyDst
	`)
	if got := interp.GetGlobal("r1").Str(); got != "b" {
		t.Fatalf("r1 = %q, want b", got)
	}
	if got := interp.GetGlobal("r2").Str(); got != "c" {
		t.Fatalf("r2 = %q, want c", got)
	}
	if got := interp.GetGlobal("r3").Str(); got != "d" {
		t.Fatalf("r3 = %q, want d", got)
	}
	if got := interp.GetGlobal("u2").Str(); got != "b" {
		t.Fatalf("u2 = %q, want b", got)
	}
	if got := interp.GetGlobal("u3").Str(); got != "c" {
		t.Fatalf("u3 = %q, want c", got)
	}
	if got := interp.GetGlobal("u4").Str(); got != "d" {
		t.Fatalf("u4 = %q, want d", got)
	}
	if got := interp.GetGlobal("reads").Int(); got != 6 {
		t.Fatalf("reads = %d, want 6", got)
	}
	if got := interp.GetGlobal("writes").Int(); got != 3 {
		t.Fatalf("writes = %d, want 3", got)
	}
	if got := interp.GetGlobal("sameDst"); !got.IsBool() || !got.Bool() {
		t.Fatalf("sameDst = %v, want true", got)
	}
	ops := interp.GetGlobal("ops").Table()
	wantOps := []string{"r2", "w1=b", "r3", "w2=c", "r4", "w3=d"}
	for i, want := range wantOps {
		if got := ops.RawGet(IntValue(int64(i + 1))).Str(); got != want {
			t.Fatalf("ops[%d] = %q, want %q", i+1, got, want)
		}
	}
}

func TestRuntimeTableMoveRejectsNonTableDestination(t *testing.T) {
	err := runProgramExpectError(t, `
		table.move({1, 2, 3}, 1, 2, 1, "not-a-table")
	`)
	if err == nil {
		t.Fatal("table.move accepted non-table destination")
	}
	if !strings.Contains(err.Error(), "table.move") {
		t.Fatalf("error = %v, want table.move", err)
	}
}

func TestRawTableMoveRejectsNonTableDestination(t *testing.T) {
	lib := buildTableLib()
	moveFn := lib.RawGet(StringValue("move")).GoFunction()
	if moveFn == nil {
		t.Fatal("raw table.move is not a Go function")
	}
	src := NewTable()
	src.RawSet(IntValue(1), IntValue(10))
	_, err := moveFn.Fn([]Value{
		TableValue(src),
		IntValue(1),
		IntValue(1),
		IntValue(1),
		StringValue("not-a-table"),
	})
	if err == nil {
		t.Fatal("raw table.move accepted non-table destination")
	}
	if !strings.Contains(err.Error(), "table.move") {
		t.Fatalf("error = %v, want table.move", err)
	}
}

func TestRuntimeTableConcatProxyCurrentlyBypassesMetamethods(t *testing.T) {
	interp := runProgram(t, `
		backing := {[1]: "a", [2]: "b", [3]: "c"}
		reads := 0
		proxy := setmetatable({}, {
			__len: func() { return 3 },
			__index: func(_, k) {
				reads = reads + 1
				return backing[k]
			},
		})
		joined := table.concat(proxy, ":")
	`)
	if got := interp.GetGlobal("joined").Str(); got != "" {
		t.Fatalf("joined = %q, want current raw concat result", got)
	}
	if got := interp.GetGlobal("reads").Int(); got != 0 {
		t.Fatalf("reads = %d, want current concat to bypass __index", got)
	}
}

func TestRuntimeTableSortComparatorErrorPropagates(t *testing.T) {
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
