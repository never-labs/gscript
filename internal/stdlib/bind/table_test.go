package bind

import "testing"

func TestTableModuleProxyMoveAndUnpackUseHooks(t *testing.T) {
	interp := runProgram(t, `
		src := {[1]: "a", [2]: "b", [3]: "c"}
		dst := {}
		reads := 0
		writes := 0
		proxySrc := setmetatable({}, {
			__len: func() { return 3 },
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

		moved := table.move(proxySrc, 2, 3, 1, proxyDst)
		u2, u3 := table.unpack(proxySrc, 2, 3)
		sameDst := moved == proxyDst
	`)

	if got := interp.GetGlobal("sameDst"); !got.IsBool() || !got.Bool() {
		t.Fatalf("sameDst = %v, want true", got)
	}
	if got := interp.GetGlobal("u2").Str(); got != "b" {
		t.Fatalf("u2 = %q, want b", got)
	}
	if got := interp.GetGlobal("u3").Str(); got != "c" {
		t.Fatalf("u3 = %q, want c", got)
	}
	if got := interp.GetGlobal("dst").Table().RawGetString("1"); !got.IsNil() {
		t.Fatalf("unexpected string-key write in dst: %v", got)
	}
	if got := interp.GetGlobal("dst").Table().RawGet(IntValue(1)).Str(); got != "b" {
		t.Fatalf("dst[1] = %q, want b", got)
	}
	if got := interp.GetGlobal("dst").Table().RawGet(IntValue(2)).Str(); got != "c" {
		t.Fatalf("dst[2] = %q, want c", got)
	}
	if got := interp.GetGlobal("reads").Int(); got != 4 {
		t.Fatalf("reads = %d, want 4", got)
	}
	if got := interp.GetGlobal("writes").Int(); got != 2 {
		t.Fatalf("writes = %d, want 2", got)
	}
}

func TestTableAppendReturnsSameTableAndAppendsValues(t *testing.T) {
	interp := runProgram(t, `
		xs := []
		same := table.append(xs, 1, 2, 3)
		table.append(xs, 4)
		ok := same == xs && #xs == 4 && xs[1] == 1 && xs[4] == 4
	`)

	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok = %v, want true", got)
	}
}
