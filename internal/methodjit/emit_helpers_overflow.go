//go:build darwin && arm64

// emit_helpers_overflow.go centralizes the small, register-parameterized
// int48 NaN-box sequences that were previously inlined byte-for-byte at many
// codegen sites.
//
// These helpers emit the EXACT same ARM64 instructions that the inline call
// sites used; they exist purely to remove duplication. Generated machine code
// is unchanged. The int48 *overflow check* (SBFX; CMPreg; BCond + deopt tail)
// already lives in emitInt48OverflowCheck in emit_arith.go and is reused at
// all of its call sites, so it is not duplicated here.

package methodjit

import "github.com/gscript/gscript/internal/jit"

// emitUnboxInt48 sign-extends a NaN-boxed integer's low 48 bits in place,
// emitting the single instruction:
//
//	SBFX(reg, reg, 0, 48)
//
// This is the canonical plain unbox that appeared inline (with dst == src) at
// every int48 unbox site in emit_table_array.go, either after an int tag-check
// branch or in the already-known-int fast path. It is parameterized only over
// the register, so the emitted instruction is identical to the inline form.
func (ec *emitContext) emitUnboxInt48(reg jit.Reg) {
	ec.asm.SBFX(reg, reg, 0, 48)
}
