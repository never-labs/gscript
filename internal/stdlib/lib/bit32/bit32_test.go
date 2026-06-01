package bit32

import "testing"

func TestShiftAndRotate(t *testing.T) {
	if got := Lshift(1, 4); got != 16 {
		t.Fatalf("Lshift = %d, want 16", got)
	}
	if got := Rshift(16, 4); got != 1 {
		t.Fatalf("Rshift = %d, want 1", got)
	}
	if got := Lrotate(1, 1); got != 2 {
		t.Fatalf("Lrotate = %d, want 2", got)
	}
	if got := Rrotate(2, 1); got != 1 {
		t.Fatalf("Rrotate = %d, want 1", got)
	}
}

func TestExtractReplace(t *testing.T) {
	got, err := Extract(65280, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	if got != 15 {
		t.Fatalf("Extract = %d, want 15", got)
	}
	got, err = Replace(65280, 10, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	if got != 64000 {
		t.Fatalf("Replace = %d, want 64000", got)
	}
}

func TestBitQueries(t *testing.T) {
	if !BtestResult(And(255, 15)) {
		t.Fatal("Btest returned false")
	}
	if Countbits(255) != 8 {
		t.Fatalf("Countbits = %d, want 8", Countbits(255))
	}
	if Highbit(0) != -1 || Highbit(1) != 0 || Highbit(2147483648) != 31 {
		t.Fatalf("unexpected Highbit results")
	}
}
