package modules

import "testing"

func TestSOAZipLenAndFastPath(t *testing.T) {
	interp := runWithLib(t, `
points := soa.zip({x: []f64{1, 2, 3}, y: []f64{4, 5, 6}})
result := soa.len(points)
`, "soa", BuildSOA())
	got := interp.GetGlobal("result")
	if !got.IsInt() || got.Int() != 3 {
		t.Fatalf("soa.len result = %v, want 3", got)
	}

	lib := BuildSOA()
	gf := lib.RawGetString("len").GoFunction()
	if gf == nil || gf.Name != "soa.len" || gf.FastArg1 == nil {
		t.Fatalf("soa.len fast binding missing: %#v", gf)
	}
}
