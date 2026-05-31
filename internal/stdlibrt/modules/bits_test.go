package modules

import "testing"

func TestBitsNativeBitwise(t *testing.T) {
	interp := runProgram(t, `
		a := bits.and(255, 15, 7)
		b := bits.or(240, 15)
		c := bits.xor(255, 15)
		d := bits.not(0)
	`)
	if got := interp.GetGlobal("a").Int(); got != 7 {
		t.Fatalf("bits.and = %d, want 7", got)
	}
	if got := interp.GetGlobal("b").Int(); got != 255 {
		t.Fatalf("bits.or = %d, want 255", got)
	}
	if got := interp.GetGlobal("c").Int(); got != 240 {
		t.Fatalf("bits.xor = %d, want 240", got)
	}
	if got := interp.GetGlobal("d").Int(); got != -1 {
		t.Fatalf("bits.not = %d, want -1", got)
	}
}

func TestBitsNativeShiftsAndRotates(t *testing.T) {
	interp := runProgram(t, `
		a := bits.shl(1, 8)
		b := bits.shr(256, 8)
		c := bits.sar(-8, 1)
		d := bits.rotl(1, 4)
		e := bits.rotr(16, 4)
	`)
	if got := interp.GetGlobal("a").Int(); got != 256 {
		t.Fatalf("bits.shl = %d, want 256", got)
	}
	if got := interp.GetGlobal("b").Int(); got != 1 {
		t.Fatalf("bits.shr = %d, want 1", got)
	}
	if got := interp.GetGlobal("c").Int(); got != -4 {
		t.Fatalf("bits.sar = %d, want -4", got)
	}
	if got := interp.GetGlobal("d").Int(); got != 16 {
		t.Fatalf("bits.rotl = %d, want 16", got)
	}
	if got := interp.GetGlobal("e").Int(); got != 1 {
		t.Fatalf("bits.rotr = %d, want 1", got)
	}
}

func TestBitsNativeBitPositions(t *testing.T) {
	interp := runProgram(t, `
		a := bits.test(10, 1)
		b := bits.test(10, 2)
		c := bits.set(0, 3)
		d := bits.clear(15, 1)
		e := bits.toggle(8, 3)
		f := bits.ones(255)
		g := bits.leadingZeros(1)
		h := bits.trailingZeros(16)
	`)
	if !interp.GetGlobal("a").Bool() {
		t.Fatalf("bits.test(10, 1) = false, want true")
	}
	if interp.GetGlobal("b").Bool() {
		t.Fatalf("bits.test(10, 2) = true, want false")
	}
	checks := map[string]int64{
		"c": 8,
		"d": 13,
		"e": 0,
		"f": 8,
		"g": 63,
		"h": 4,
	}
	for name, want := range checks {
		if got := interp.GetGlobal(name).Int(); got != want {
			t.Fatalf("%s = %d, want %d", name, got, want)
		}
	}
}

func TestBitsNativeArgumentErrors(t *testing.T) {
	interp := runProgram(t, `
		ok1 := pcall(bits.shl, 1, -1)
		ok2 := pcall(bits.test, 1, 64)
		ok3 := pcall(bits.and, "x", 1)
	`)
	if interp.GetGlobal("ok1").Bool() {
		t.Fatalf("bits.shl accepted negative shift")
	}
	if interp.GetGlobal("ok2").Bool() {
		t.Fatalf("bits.test accepted out-of-range bit position")
	}
	if interp.GetGlobal("ok3").Bool() {
		t.Fatalf("bits.and accepted non-number")
	}
}
