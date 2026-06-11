//go:build darwin && arm64

package jit

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

// buildSPProbe assembles JIT-style code that stores the live SP value into
// 0(ctx) and returns.
func buildSPProbe(t *testing.T) *CodeBlock {
	t.Helper()
	asm := NewAssembler()
	asm.SUBimm(SP, SP, 16)
	asm.STP(X29, X30, SP, 0)
	asm.ADDimm(X1, SP, 0) // X1 = SP after frame push
	asm.STR(X1, X0, 0)    // *(ctx) = SP
	asm.LDP(X29, X30, SP, 0)
	asm.ADDimm(SP, SP, 16)
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

// TestCallJITOnStackSwitchesSP verifies that JIT code really executes with
// SP inside the JIT stack region and that the trampoline restores the
// goroutine environment on return.
func TestCallJITOnStackSwitchesSP(t *testing.T) {
	stk, err := NewJITStack(256 << 10)
	if err != nil {
		t.Fatal(err)
	}
	defer stk.Release()

	block := buildSPProbe(t)
	defer block.Free()

	var observedSP uintptr
	CallJITOnStack(uintptr(block.Ptr()), uintptr(unsafe.Pointer(&observedSP)), stk)

	base := uintptr(unsafe.Pointer(&stk.mem[0]))
	if observedSP < base || observedSP > stk.hdr {
		t.Fatalf("JIT SP %#x not within JIT stack [%#x, %#x]", observedSP, base, stk.hdr)
	}
	if observedSP != stk.hdr-16 {
		t.Fatalf("JIT SP %#x, want hdr-16 = %#x", observedSP, stk.hdr-16)
	}
}

// buildHelperLoop assembles JIT-style code that BLRs the Go-helper thunk
// iters times. Thunk contract: R0 = hdr, R19 = ctx. The test passes the
// header address as ctx, so the bridge receives hdr.
func buildHelperLoop(t *testing.T, iters int64) *CodeBlock {
	t.Helper()
	asm := NewAssembler()
	// Prologue on the JIT stack (mirrors Tier 2 conventions).
	asm.SUBimm(SP, SP, 48)
	asm.STP(X29, X30, SP, 0)
	asm.ADDimm(X29, SP, 0)
	asm.STP(X19, X20, SP, 16)
	asm.STP(X21, X22, SP, 32)
	asm.MOVreg(X19, X0) // X19 = ctx (== hdr in this test)
	asm.LoadImm64(X20, iters)
	asm.LoadImm64(X21, 0x1122334455667788) // canary: must survive helper calls
	asm.Label("loop")
	asm.MOVreg(X0, X19) // R0 = hdr
	asm.LoadImm64(X16, int64(JITHelperEntryPC()))
	asm.BLR(X16)
	asm.SUBimm(X20, X20, 1)
	asm.CBNZ(X20, "loop")
	// Verify the callee-saved canary survived every helper round trip.
	asm.LoadImm64(X0, 0x1122334455667788)
	asm.CMPreg(X21, X0)
	asm.BCond(CondEQ, "ok")
	asm.LoadImm64(X1, 0xDEAD)
	asm.STR(X1, X19, 32) // hdr+32 (jitSP slot, dead after return) = failure marker
	asm.Label("ok")
	asm.LDP(X21, X22, SP, 32)
	asm.LDP(X19, X20, SP, 16)
	asm.LDP(X29, X30, SP, 0)
	asm.ADDimm(SP, SP, 48)
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

var growStackSink byte

//go:noinline
func growStack(depth int) {
	var pad [256]byte
	pad[0] = byte(depth)
	growStackSink ^= pad[0]
	if depth > 0 {
		growStack(depth - 1)
	}
}

// TestJITGoHelperCallUnderGC is the go/no-go stress for direct BLR Go-helper
// calls from JIT code running on the alternate stack. The helper:
//   - periodically recurses deep enough to force stack growth (copystack
//     while the fabricated jitGoHelperBody frame is mid-stack), and
//   - periodically runs a full GC (precise stack scan across the fabricated
//     frame chain), while a sibling goroutine hammers runtime.GC().
//
// Any unwinder/copystack incompatibility fatals the runtime ("unknown pc",
// "traceback did not unwind completely", "unexpected SPWRITE"), so the test
// passing at all is the signal.
func TestJITGoHelperCallUnderGC(t *testing.T) {
	stk, err := NewJITStack(256 << 10)
	if err != nil {
		t.Fatal(err)
	}
	defer stk.Release()

	const iters = 20000
	var calls int64
	prev := jitHelperBridgeFn
	SetJITHelperBridge(func(ctx uintptr) {
		if ctx != stk.Hdr() {
			t.Errorf("bridge ctx = %#x, want hdr %#x", ctx, stk.Hdr())
		}
		n := atomic.AddInt64(&calls, 1)
		if n%16 == 0 {
			growStack(512) // force morestack/copystack under the fabricated frame
		}
		if n%1024 == 0 {
			runtime.GC() // force a precise scan of this goroutine's stack
		}
	})
	defer SetJITHelperBridge(prev)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				runtime.GC()
				time.Sleep(time.Millisecond)
			}
		}
	}()

	block := buildHelperLoop(t, iters)
	defer block.Free()
	CallJITOnStack(uintptr(block.Ptr()), stk.Hdr(), stk)
	close(stop)
	<-done

	if got := atomic.LoadInt64(&calls); got != iters {
		t.Fatalf("helper calls = %d, want %d", got, iters)
	}
	hdrOff := len(stk.mem) - jitStackHeaderSize
	marker := *(*uint64)(unsafe.Pointer(&stk.mem[hdrOff+32]))
	if marker == 0xDEAD {
		t.Fatalf("callee-saved canary (X21) corrupted across helper call")
	}
}

// TestJITGoHelperCallPreemption keeps a helper-calling JIT loop running long
// enough for async preemption + concurrent GC cycles to interleave with the
// JIT<->Go transitions on multiple goroutines at once.
func TestJITGoHelperCallPreemption(t *testing.T) {
	const workers = 4
	const iters = 50000
	var calls int64
	prev := jitHelperBridgeFn
	SetJITHelperBridge(func(ctx uintptr) {
		atomic.AddInt64(&calls, 1)
	})
	defer SetJITHelperBridge(prev)

	block := buildHelperLoop(t, iters)
	defer block.Free()

	errc := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			stk, err := NewJITStack(256 << 10)
			if err != nil {
				errc <- err
				return
			}
			defer stk.Release()
			CallJITOnStack(uintptr(block.Ptr()), stk.Hdr(), stk)
			errc <- nil
		}()
	}
	for i := 0; i < workers; i++ {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt64(&calls); got != workers*iters {
		t.Fatalf("helper calls = %d, want %d", got, workers*iters)
	}
}

// buildNoopReturn assembles JIT code that returns immediately.
func buildNoopReturn(t testing.TB) *CodeBlock {
	asm := NewAssembler()
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

// buildHelperLoopBench is buildHelperLoop without the canary plumbing,
// reusable from benchmarks.
func buildHelperLoopBench(t testing.TB, iters int64) *CodeBlock {
	asm := NewAssembler()
	asm.SUBimm(SP, SP, 32)
	asm.STP(X29, X30, SP, 0)
	asm.ADDimm(X29, SP, 0)
	asm.STP(X19, X20, SP, 16)
	asm.MOVreg(X19, X0)
	asm.LoadImm64(X20, iters)
	asm.Label("loop")
	asm.MOVreg(X0, X19)
	asm.LoadImm64(X16, int64(JITHelperEntryPC()))
	asm.BLR(X16)
	asm.SUBimm(X20, X20, 1)
	asm.CBNZ(X20, "loop")
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

// BenchmarkCallJITGoroutineStack is the legacy trampoline floor: the cost of
// one Go->JIT->Go round trip on the goroutine stack (the minimum cost of one
// exit-resume round trip, excluding all Go-side exit handler work).
func BenchmarkCallJITGoroutineStack(b *testing.B) {
	block := buildNoopReturn(b)
	defer block.Free()
	fn := uintptr(block.Ptr())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CallJIT(fn, 0)
	}
}

// BenchmarkCallJITOnStack is the alternate-stack trampoline floor.
func BenchmarkCallJITOnStack(b *testing.B) {
	stk, err := NewJITStack(256 << 10)
	if err != nil {
		b.Fatal(err)
	}
	defer stk.Release()
	block := buildNoopReturn(b)
	defer block.Free()
	fn := uintptr(block.Ptr())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CallJITOnStack(fn, 0, stk)
	}
}

// BenchmarkJITGoHelperCall measures one direct BLR JIT->Go helper round trip
// (thunk save/switch + fabricated frame + bridge dispatch to a no-op helper +
// switch back), amortized over an in-JIT loop.
func BenchmarkJITGoHelperCall(b *testing.B) {
	stk, err := NewJITStack(256 << 10)
	if err != nil {
		b.Fatal(err)
	}
	defer stk.Release()
	prev := jitHelperBridgeFn
	SetJITHelperBridge(func(ctx uintptr) {})
	defer SetJITHelperBridge(prev)

	const inner = 1024
	block := buildHelperLoopBench(b, inner)
	defer block.Free()
	fn := uintptr(block.Ptr())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CallJITOnStack(fn, stk.Hdr(), stk)
	}
	b.StopTimer()
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(int64(b.N)*inner), "ns/helper-call")
}
