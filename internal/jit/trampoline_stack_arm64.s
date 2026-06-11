#include "textflag.h"
#include "funcdata.h"

// JIT alternate-stack trampoline (R5-K feasibility prototype).
//
// Layout of the JIT stack header (hdr is the 16-byte-aligned top of the
// usable JIT stack; fields live at positive offsets ABOVE hdr, JIT frames
// grow DOWN from hdr):
//
//   0(hdr)    goSP   goroutine SP at callJITOnStack entry. Refreshed by
//                    jitGoHelperReturn after every Go-helper call so a
//                    copystack during the helper cannot leave it stale.
//   8(hdr)    goFP   goroutine R29 at entry (refreshed like goSP).
//   16(hdr)   goLR   return PC into the Go caller of callJITOnStack. Also
//                    used as the fabricated return PC for jitGoHelperBody,
//                    so the runtime unwinder walks helper -> body -> Go
//                    caller of callJITOnStack, never seeing a JIT frame.
//   24(hdr)   g      goroutine g pointer (R28). g never changes for the
//                    lifetime of a goroutine, so this stays valid across
//                    helper-time preemption/migration.
//   32(hdr)   jitSP  JIT-stack SP saved across one in-flight helper call.
//   40(hdr)   jitLR  JIT resume PC saved across one in-flight helper call.
//   48..120   X19-X28 JIT register state saved across one helper call.
//   128..152  D8-D11  JIT callee-saved FPR state saved across one helper call.
//   160(hdr)  jitFP  JIT-side R29 saved across one helper call.
//
// Invariants:
//   - At most one Go-helper call is in flight per JIT stack (helpers must
//     not re-enter JIT code on the same stack).
//   - JIT code is never at an async-preemption-safe point, so the runtime
//     never scans/unwinds while SP is on the JIT stack. All unwinding
//     happens either fully outside JIT execution or during a helper call,
//     where the fabricated chain below is precise-walkable.

// func callJITOnStack(fn, ctx, hdr uintptr)
//
// Runs JIT code with SP switched to the JIT stack. Leaf + NOFRAME: this
// function deliberately keeps NO frame on the goroutine stack, so during a
// helper call the unwind chain can skip it entirely (jitGoHelperBody claims
// our caller as its own caller). All state needed to return lives in the
// header, and the parts that point into the goroutine stack (goSP/goFP) are
// refreshed by jitGoHelperReturn after any potential copystack.
TEXT ·callJITOnStack(SB), NOSPLIT|NOFRAME, $0-24
	MOVD	fn+0(FP), R16
	MOVD	ctx+8(FP), R0
	MOVD	hdr+16(FP), R3
	// Save goroutine-stack execution state into the header.
	MOVD	RSP, R1
	MOVD	R1, 0(R3)	// goSP
	MOVD	R29, 8(R3)	// goFP
	MOVD	R30, 16(R3)	// goLR (return into our Go caller)
	MOVD	g, 24(R3)	// g
	// Switch to the JIT stack and enter JIT code. Indirect call: JIT code
	// is invisible to the linker's nosplit analysis anyway, and the JIT
	// prologue saves R30 on the JIT stack.
	MOVD	R3, RSP
	CALL	(R16)
	// JIT epilogue is SP-symmetric: SP == hdr again.
	MOVD	RSP, R3
	MOVD	24(R3), g
	MOVD	16(R3), R30
	MOVD	8(R3), R29	// possibly copystack-adjusted via jitGoHelperReturn
	MOVD	0(R3), R1
	MOVD	R1, RSP		// possibly copystack-adjusted via jitGoHelperReturn
	RET

// jitGoHelperEntry is BLR'd directly from JIT code to run a Go helper.
//
// Entry contract (JIT side):
//   R0  = hdr (JIT stack header)
//   R19 = ctx (*methodjit.ExecContext as uintptr; opaque here)
//   R30 = JIT resume address (BLR link)
//   SP  = current JIT-stack SP
//
// Saves all JIT callee-saved state into the header, restores the goroutine
// execution environment (g, SP, FP) and fabricates R30 = hdr.goLR so that
// jitGoHelperBody's assembler prologue records the Go caller of
// callJITOnStack as its return PC. The runtime unwinder then sees a fully
// legal Go frame chain while the helper runs.
TEXT ·jitGoHelperEntry(SB), NOSPLIT|NOFRAME, $0-0
	// Save JIT state.
	MOVD	R29, 160(R0)	// jitFP (JIT frame pointer; restored on resume)
	MOVD	RSP, R1
	MOVD	R1, 32(R0)	// jitSP
	MOVD	R30, 40(R0)	// jitLR
	STP	(R19, R20), 48(R0)
	STP	(R21, R22), 64(R0)
	STP	(R23, R24), 80(R0)
	STP	(R25, R26), 96(R0)
	STP	(R27, g), 112(R0)	// g register == R28: JIT uses it as an allocatable GPR
	FSTPD	(F8, F9), 128(R0)
	FSTPD	(F10, F11), 144(R0)
	// Restore the Go execution environment.
	MOVD	24(R0), g
	MOVD	8(R0), R29	// goroutine FP
	MOVD	16(R0), R30	// fabricated return PC: Go caller of callJITOnStack
	MOVD	0(R0), R1
	MOVD	R1, RSP		// goroutine stack
	B	·jitGoHelperBody(SB)

// jitGoHelperBody owns the goroutine-stack frame that exists while the Go
// helper runs. It is entered by branch (not call) with:
//   SP  = hdr.goSP (the SP our Go caller used to call callJITOnStack)
//   R30 = fabricated return PC (hdr.goLR)
//   R29 = hdr.goFP
//   R0  = hdr
//   R19 = ctx
//
// The hand-written prologue below reproduces the Go compiler's small-frame
// layout (saved LR at the frame bottom, caller FP one word below SP,
// R29 = SP-8) with a fixed 48-byte frame, so GC stack scans, stack growth
// (copystack) and panic unwinding all walk helper -> jitGoHelperBody ->
// (Go caller of callJITOnStack) seamlessly. NOFRAME keeps the assembler from
// padding the frame (its $n rounding would desync the manual epilogue).
// IMPORTANT: the SP adjustments MUST be the `SUB/ADD $const, RSP` forms —
// those are the only ones obj/arm64 assigns Spadj (and hence pcsp entries)
// for in hand-written assembly; pre/post-indexed writeback forms (MOVD.W/
// MOVD.P) leave the pcsp table flat and desync the unwinder. No register
// writes to RSP here: the function must NOT carry the SPWRITE flag, because
// its frame sits mid-stack during precise tracebacks.
TEXT ·jitGoHelperBody(SB), NOSPLIT|NOFRAME, $0-0
	NO_LOCAL_POINTERS
	SUB	$48, RSP	// open frame (Spadj-tracked)
	MOVD	R30, 0(RSP)	// saved LR slot (fabricated return PC)
	MOVD	R29, -8(RSP)	// caller FP one word below SP (arm64 Go layout)
	SUB	$8, RSP, R29
	MOVD	R0, 24(RSP)	// stash hdr (off-stack pointer; empty local map)
	MOVD	R19, 8(RSP)	// ctx argument for the bridge
	CALL	·helperBridge(SB)
	MOVD	24(RSP), R0	// reload hdr
	// Epilogue, then hand off to the SPWRITE return thunk with this frame
	// fully popped. SP/R29 now hold post-copystack values if the stack
	// moved during the helper.
	MOVD	-8(RSP), R29
	MOVD	0(RSP), R30
	ADD	$48, RSP	// close frame (Spadj-tracked)
	B	·jitGoHelperReturn(SB)

// jitGoHelperReturn switches back to the JIT stack after the helper.
// Entry: SP = current goroutine SP (== refreshed goSP), R29 = current
// goroutine FP, R0 = hdr. Refreshes the header copies (they may be stale
// after a copystack during the helper), restores the JIT register state and
// resumes JIT code. SPWRITE is fine here: this function only ever executes
// as the innermost frame and is never unwound across.
TEXT ·jitGoHelperReturn(SB), NOSPLIT|NOFRAME, $0-0
	MOVD	RSP, R1
	MOVD	R1, 0(R0)	// hdr.goSP refresh
	MOVD	R29, 8(R0)	// hdr.goFP refresh
	// Restore JIT register state.
	LDP	48(R0), (R19, R20)
	LDP	64(R0), (R21, R22)
	LDP	80(R0), (R23, R24)
	LDP	96(R0), (R25, R26)
	LDP	112(R0), (R27, g)	// restore JIT's R28 value
	FLDPD	128(R0), (F8, F9)
	FLDPD	144(R0), (F10, F11)
	MOVD	40(R0), R30	// JIT resume address
	MOVD	32(R0), R1
	MOVD	R1, RSP		// back on the JIT stack
	MOVD	160(R0), R29	// restore JIT frame pointer
	RET

// func jitGoHelperEntryAddr() uintptr
TEXT ·jitGoHelperEntryAddr(SB), NOSPLIT, $0-8
	MOVD	$·jitGoHelperEntry(SB), R0
	MOVD	R0, ret+0(FP)
	RET
