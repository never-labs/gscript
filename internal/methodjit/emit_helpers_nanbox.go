//go:build darwin && arm64

// emit_helpers_nanbox.go centralizes small, register-parameterized NaN-box
// tag-check sequences that were previously inlined byte-for-byte at many sites.
//
// These helpers emit the EXACT same ARM64 instruction sequences that the
// inline call sites used; they exist purely to remove duplication. Generated
// machine code is unchanged. See emit_call.go's emitCheckIsInt family for the
// canonical (compare-only) variants.

package methodjit

import "github.com/never-labs/gscript/internal/jit"

// emitIntTagCheckBranch emits the full NaN-box integer tag-check-and-branch
// sequence:
//
//	LSRimm(shiftDst, valReg, 48)        // shiftDst = top 16 bits of valReg
//	MOVimm16(tagReg, NB_TagIntShr48)    // tagReg   = 0xFFFE (int tag)
//	CMPreg(shiftDst, tagReg)
//	BCond(cond, target)
//
// This is the precise sequence that appeared inline at the table array
// codegen sites (cond is always CondNE there, branching to a miss/deopt
// label). It is parameterized only over the registers, condition, and label.
func (ec *emitContext) emitIntTagCheckBranch(valReg, shiftDst, tagReg jit.Reg, cond jit.Cond, target string) {
	asm := ec.asm
	asm.LSRimm(shiftDst, valReg, 48)
	asm.MOVimm16(tagReg, uint16(jit.NB_TagIntShr48))
	asm.CMPreg(shiftDst, tagReg)
	asm.BCond(cond, target)
}
