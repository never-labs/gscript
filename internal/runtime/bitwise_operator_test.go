package runtime

import "testing"

func TestBitwiseExpressionOperators(t *testing.T) {
	interp := NewCore()

	execBinaryIOTest(t, interp, `
		a := 240 & 60
		b := 240 | 15
		c := 255 ^ 15
		d := ^0
		e := 1 << 8
		f := 256 >> 4
		g := -8 >> 1
		h := 255 &^ 15
		i := 1 + 2 << 3
	`)

	checks := map[string]int64{
		"a": 0x30,
		"b": 0xFF,
		"c": 0xF0,
		"d": -1,
		"e": 256,
		"f": 16,
		"g": -4,
		"h": 0xF0,
		"i": 17,
	}
	for name, want := range checks {
		if got := interp.GetGlobal(name).Int(); got != want {
			t.Fatalf("%s = %d, want %d", name, got, want)
		}
	}
}

func TestBitwiseExpressionErrors(t *testing.T) {
	interp := NewCore()

	execBinaryIOTest(t, interp, `
		okShift, errShift := pcall(func() { return 1 << -1 })
		okType, errType := pcall(func() { return "x" & 1 })
	`)

	if interp.GetGlobal("okShift").Truthy() {
		t.Fatalf("negative shift should fail")
	}
	if interp.GetGlobal("okType").Truthy() {
		t.Fatalf("non-number bitwise operand should fail")
	}
	if !interp.GetGlobal("errShift").IsString() || !interp.GetGlobal("errType").IsString() {
		t.Fatalf("expected string errors")
	}
}
