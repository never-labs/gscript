package bind

import "testing"

func TestExplicitTableSpreadInNestedCall(t *testing.T) {
	v := getGlobal(t, `
		func join(a, b, c, d) { return a .. b .. c .. d }
		result := join("a", table.spread({"b", "c"}), "d")
	`, "result")
	if !v.IsString() || v.Str() != "abcd" {
		t.Errorf("expected abcd, got %v", v)
	}
}

func TestExplicitTableSpreadInTableConstructor(t *testing.T) {
	interp := runProgram(t, `
		func pair() { return 2, 3 }
		t := {1, spread(pair()), 4, table.spread({5, 6})}
	`)
	tbl := interp.GetGlobal("t").Table()
	for i, want := range []int64{1, 2, 3, 4, 5, 6} {
		got := tbl.RawGet(IntValue(int64(i + 1)))
		if !got.IsInt() || got.Int() != want {
			t.Fatalf("t[%d] = %v, want %d", i+1, got, want)
		}
	}
}
