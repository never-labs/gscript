package soa

import "testing"

func TestNewKernelArgs(t *testing.T) {
	args := NewKernelArgs("dst", "src", 2.5)
	if args.Dst != "dst" || args.Src != "src" || args.Scale != 2.5 {
		t.Fatalf("kernel args mismatch: %+v", args)
	}
}

func TestNewAffineArgs(t *testing.T) {
	args := NewAffineArgs("dst", "src", 2.5, -1)
	if args.Dst != "dst" || args.Src != "src" || args.Scale != 2.5 || args.Bias != -1 {
		t.Fatalf("affine args mismatch: %+v", args)
	}
}

func TestNewAffineTerm(t *testing.T) {
	term := NewAffineTerm("dst", "src", 1.25, 3)
	if term.Dst != "dst" || term.Src != "src" || term.Scale != 1.25 || term.Bias != 3 {
		t.Fatalf("affine term mismatch: %+v", term)
	}
}

func TestDefaultAffineBias(t *testing.T) {
	if got := DefaultAffineBias(false, 42); got != 0 {
		t.Fatalf("missing bias default = %v, want 0", got)
	}
	if got := DefaultAffineBias(true, 42); got != 42 {
		t.Fatalf("explicit bias = %v, want 42", got)
	}
}

func TestSliceRange(t *testing.T) {
	start, end := SliceRange(2, 5)
	if start != 1 || end != 5 {
		t.Fatalf("SliceRange(2, 5) = (%d, %d), want (1, 5)", start, end)
	}
}
