//go:build darwin && arm64

package jit

import (
	"runtime"
	"testing"
	"unsafe"
)

// buildOneHelperCall assembles JIT code that performs exactly one Go-helper
// BLR and returns.
func buildOneHelperCall(t *testing.T) *CodeBlock {
	t.Helper()
	asm := NewAssembler()
	asm.SUBimm(SP, SP, 32)
	asm.STP(X29, X30, SP, 0)
	asm.ADDimm(X29, SP, 0)
	asm.STP(X19, X20, SP, 16)
	asm.MOVreg(X19, X0) // ctx = hdr
	asm.MOVreg(X0, X19)
	asm.LoadImm64(X16, int64(JITHelperEntryPC()))
	asm.BLR(X16)
	asm.LDP(X19, X20, SP, 16)
	asm.LDP(X29, X30, SP, 0)
	asm.ADDimm(SP, SP, 32)
	asm.RET()
	code, err := asm.Finalize()
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	block, err := AllocExec(len(code))
	if err != nil {
		t.Fatalf("alloc: %v", err)
	}
	if err := block.WriteCode(code); err != nil {
		t.Fatalf("write: %v", err)
	}
	return block
}

// TestJITHelperTracebackDiag captures runtime.Callers from inside the bridge
// (validating the fabricated unwind chain without copystack), then triggers
// stack growth.
func TestJITHelperTracebackDiag(t *testing.T) {
	stk, err := NewJITStack(256 << 10)
	if err != nil {
		t.Fatal(err)
	}
	defer stk.Release()
	block := buildOneHelperCall(t)
	defer block.Free()

	var pcs [32]uintptr
	var n int
	mode := 0
	prev := jitHelperBridgeFn
	SetJITHelperBridge(func(ctx uintptr) {
		switch mode {
		case 0:
			n = runtime.Callers(0, pcs[:])
		case 1:
			growStack(512)
		case 2:
			runtime.GC()
		}
	})
	defer SetJITHelperBridge(prev)

	t.Logf("hdr=%#x", stk.Hdr())

	mode = 0
	CallJITOnStack(uintptr(block.Ptr()), stk.Hdr(), stk)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		t.Logf("frame: %s (%#x)", f.Function, f.PC)
		if !more {
			break
		}
	}

	mode = 1
	t.Logf("before growStack mode")
	CallJITOnStack(uintptr(block.Ptr()), stk.Hdr(), stk)
	t.Logf("growStack survived")

	mode = 2
	CallJITOnStack(uintptr(block.Ptr()), stk.Hdr(), stk)
	t.Logf("GC survived")
	_ = unsafe.Pointer(nil)
}
