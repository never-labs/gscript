//go:build darwin && arm64

// jitstack.go implements a dedicated non-Go stack for Tier 2 JIT execution
// (R5-K feasibility prototype).
//
// Today JIT code runs on the goroutine stack (callJIT). That is safe only
// because a goroutine inside JIT code is never at an async-preemption-safe
// point, so the runtime never scans or copies the stack while JIT frames are
// live on it — but it also means JIT code can never BLR into a preemptible
// Go helper, because while the helper runs the runtime CAN suspend the
// goroutine and would then have to unwind across a funcdata-less JIT frame
// ("unknown pc" in gentraceback, unfixable in copystack).
//
// The alternate-stack discipline (cgocallback-style, see
// trampoline_stack_arm64.s):
//
//   - JIT frames live on a separate mmap'd stack, invisible to the runtime.
//   - callJITOnStack keeps NO frame on the goroutine stack; everything it
//     needs lives in the JIT stack header.
//   - A JIT->Go helper call (jitGoHelperEntry) switches SP back to the
//     goroutine stack and opens a real, unwindable frame (jitGoHelperBody)
//     whose saved LR is fabricated to point at the Go caller of
//     callJITOnStack — the same trick runtime.cgocallback uses with the
//     saved gobuf PC. GC scans, stack growth and shrink (copystack), panics
//     and async preemption during the helper all see a fully legal stack.
//   - After the helper returns, jitGoHelperReturn refreshes the header's
//     goroutine SP/FP copies (they may have been invalidated by copystack)
//     and switches back to the JIT stack.
package jit

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// jitStackHeaderSize is the reserved header at the top of a JIT stack.
// Must match the layout documented in trampoline_stack_arm64.s and be a
// multiple of 16.
const jitStackHeaderSize = 192

// JITStack is a fixed-size non-Go stack for JIT execution with a guard page
// at the low end. The header at the top holds the goroutine<->JIT transition
// state.
type JITStack struct {
	mem []byte
	hdr uintptr
}

// NewJITStack allocates a JIT stack with at least size usable bytes and a
// PROT_NONE guard page below it (stack overflow faults deterministically
// instead of corrupting memory).
func NewJITStack(size int) (*JITStack, error) {
	pageSize := syscall.Getpagesize()
	if size < 64<<10 {
		size = 64 << 10
	}
	usable := (size + jitStackHeaderSize + pageSize - 1) &^ (pageSize - 1)
	total := usable + pageSize // low guard page
	mem, err := syscall.Mmap(-1, 0, total,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON)
	if err != nil {
		return nil, fmt.Errorf("jit: stack mmap failed: %w", err)
	}
	if err := syscall.Mprotect(mem[:pageSize], syscall.PROT_NONE); err != nil {
		_ = syscall.Munmap(mem)
		return nil, fmt.Errorf("jit: stack guard mprotect failed: %w", err)
	}
	base := uintptr(unsafe.Pointer(&mem[0]))
	return &JITStack{
		mem: mem,
		hdr: base + uintptr(total) - jitStackHeaderSize,
	}, nil
}

// Hdr returns the JIT stack header address (== initial JIT SP). Emitted code
// passes this to jitGoHelperEntry in R0 for direct Go-helper calls.
func (s *JITStack) Hdr() uintptr { return s.hdr }

// Release unmaps the stack. The stack must not be in use.
func (s *JITStack) Release() error {
	if s == nil || s.mem == nil {
		return nil
	}
	mem := s.mem
	s.mem = nil
	s.hdr = 0
	return syscall.Munmap(mem)
}

// CallJITOnStack runs JIT code at fn with SP switched to the given JIT
// stack. Calling convention for fn is identical to CallJIT (ctx in R0).
// The caller must guarantee single-threaded use of s for the duration.
func CallJITOnStack(fn, ctx uintptr, s *JITStack) {
	callJITOnStack(fn, ctx, s.hdr)
	runtime.KeepAlive(s)
}

// callJITOnStack is implemented in trampoline_stack_arm64.s.
//
//go:noescape
func callJITOnStack(fn, ctx, hdr uintptr)

// jitGoHelperEntryAddr is implemented in trampoline_stack_arm64.s.
func jitGoHelperEntryAddr() uintptr

// jitGoHelperBody is the goroutine-stack frame owner for JIT-initiated Go
// helper calls (trampoline_stack_arm64.s). Never called from Go; declared so
// vet can check the assembly.
func jitGoHelperBody()

// JITHelperEntryPC returns the native address of the JIT->Go helper thunk.
// Emitted JIT code calls it with BLR: R0 = stack header, R19 = ExecContext.
// X19-X28 and D8-D11 are preserved across the call; all other registers are
// clobbered (the helper runs arbitrary Go code).
func JITHelperEntryPC() uintptr { return jitGoHelperEntryAddr() }

// jitHelperBridgeFn dispatches JIT-initiated Go-helper calls. Set once at
// init time by the method JIT (before any JIT code can BLR the thunk).
var jitHelperBridgeFn func(ctx uintptr)

// SetJITHelperBridge installs the Go helper dispatcher. The dispatcher runs
// on the goroutine stack with full Go semantics (preemption, GC, stack
// growth, panics) but MUST NOT re-enter JIT code on the same JIT stack.
func SetJITHelperBridge(fn func(ctx uintptr)) { jitHelperBridgeFn = fn }

// helperBridge is the Go function called from jitGoHelperBody (assembly).
func helperBridge(ctx uintptr) {
	jitHelperBridgeFn(ctx)
}
