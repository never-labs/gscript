//go:build darwin && arm64

// tier1_call.go emits ARM64 templates for native function calls in the Tier 1
// baseline compiler. Instead of exiting to Go for every OP_CALL (exit-resume),
// this emits a native BLR sequence that calls the callee's compiled code
// directly when the callee is a compiled vm.Closure.
//
// The native call sequence:
//   1. Load the function value from the register file
//   2. Type-check: must be a vm.Closure (ptrSubVMClosure = 8)
//   3. Load Proto.CompiledCodePtr; if zero, fall to slow path
//   4. Save caller state on stack (X26, X27, X29, X30)
//   5. Copy arguments from caller's registers to callee's register window
//   6. Set up callee's context (Regs, Constants, ClosurePtr)
//   7. BLR to callee's direct entry point
//   8. Restore caller state from stack
//   9. Check callee exit code (0 = normal return)
//  10. Move return value to destination register
//
// Supports variable-return (C=0) and variable-arg (B=0) calls natively
// by reading/writing Top via TopPtr in ExecContext.
//
// Falls back to the existing exit-resume path (slow path) for:
//   - GoFunctions
//   - Uncompiled closures (CompiledCodePtr == 0)
//   - Non-function values
//   - Register file overflow (callee window exceeds allocated regs)

package methodjit

import (
	"sync"
	"unsafe"

	"github.com/never-labs/gscript/internal/jit"
	"github.com/never-labs/gscript/internal/vm"
)

// Struct layout constants for vm.Closure and vm.FuncProto.
// Verified at init time via unsafe.Offsetof.
var (
	vmClosureOffProto          int // vm.Closure.Proto offset (should be 0)
	vmClosureOffUpvalues       int // vm.Closure.Upvalues offset (should be 8)
	vmClosureOffInlineUpvalue0 int // vm.Closure.inlineUpvalue[0] offset

	funcProtoOffCompiledCodePtr           int // vm.FuncProto.CompiledCodePtr offset
	funcProtoOffDirectEntryPtr            int // vm.FuncProto.DirectEntryPtr offset
	funcProtoOffTier2DirectEntryPtr       int // vm.FuncProto.Tier2DirectEntryPtr offset
	funcProtoOffTier2LeafEntryPtr         int // vm.FuncProto.Tier2LeafEntryPtr offset
	funcProtoOffDirectEntryVersion        int // vm.FuncProto.DirectEntryVersion offset
	funcProtoOffTier2NumericEntryPtr      int // vm.FuncProto.Tier2NumericEntryPtr offset
	funcProtoOffTier2TypedEntryPtr        int // vm.FuncProto.Tier2TypedEntryPtr offset
	funcProtoOffTier2TypedClobberEntryPtr int // vm.FuncProto.Tier2TypedClobberEntryPtr offset
	funcProtoOffTier2TypedEntryABI        int // vm.FuncProto.Tier2TypedEntryABI offset
	funcProtoOffConstants                 int // vm.FuncProto.Constants offset (slice header)
	funcProtoOffFieldCache                int // vm.FuncProto.FieldCache offset (slice header)
	funcProtoOffFieldPolyCache            int // vm.FuncProto.FieldPolyCache offset (slice header)
	funcProtoOffTableStringKeyCache       int // vm.FuncProto.TableStringKeyCache offset (slice header)
	funcProtoOffMaxStack                  int // vm.FuncProto.MaxStack offset
	funcProtoOffNumParams                 int // vm.FuncProto.NumParams offset
	funcProtoOffIsVarArg                  int // vm.FuncProto.IsVarArg offset
	funcProtoOffGlobalValCachePtr         int // vm.FuncProto.GlobalValCachePtr offset
	funcProtoOffTier2GlobalCachePtr       int // vm.FuncProto.Tier2GlobalCachePtr offset
	funcProtoOffTier2GlobalCacheGenPtr    int // vm.FuncProto.Tier2GlobalCacheGenPtr offset
	funcProtoOffTier2GlobalIndexPtr       int // vm.FuncProto.Tier2GlobalIndexPtr offset
	funcProtoOffCallCount                 int // vm.FuncProto.CallCount offset
	funcProtoOffTier2Promoted             int // vm.FuncProto.Tier2Promoted offset
	funcProtoOffLeafNoCall                int // vm.FuncProto.LeafNoCall offset
	funcProtoOffTier2LeafNoCall           int // vm.FuncProto.Tier2LeafNoCall offset
	funcProtoOffNoGlobalOps               int // vm.FuncProto.NoGlobalOps offset
)

// NaN-boxing pointer sub-type constants for ARM64 type checks.
const (
	nbPtrSubShift     = 44
	nbPtrSubVMClosure = 8 // ptrSubVMClosure = 8 << 44
)

const (
	baselineCallCacheStride      = 4
	baselineCallCacheBoxedOff    = 0
	baselineCallCacheEntryOff    = 8
	baselineCallCacheProtoOff    = 16
	baselineCallCacheVersionOff  = 24
	baselineCallCacheStrideBytes = baselineCallCacheStride * 8
)

// mRegSelfClosure caches the NaN-boxed closure value of the current function
// in callee-saved register X21. At CALL sites, comparing R(A) directly with
// X21 detects self-calls in 2 instructions instead of ~14.
const mRegSelfClosure = jit.X21

// nbClosureTagBits is the NaN-boxing tag for a VMClosure pointer:
// 0xFFFF800000000000 = NB_TagPtr | (ptrSubVMClosure << nbPtrSubShift).
const nbClosureTagBits = ^int64(1<<47 - 1)

type accumulatorClosureFastPath struct {
	proto      *vm.FuncProto
	valueUpval int
	deltaKind  accumulatorDeltaKind
	delta      int64
	deltaUpval int
}

type simpleClosureExprFastPath struct {
	proto *vm.FuncProto
	expr  simpleClosureExpr
}

type immediateClosureFactoryFastPath struct {
	proto      *vm.FuncProto
	expr       simpleClosureExpr
	upvalSlots []int
}

type simpleClosureExprKind uint8

const (
	simpleClosureExprParam simpleClosureExprKind = iota
	simpleClosureExprIntConst
	simpleClosureExprUpval
	simpleClosureExprAdd
	simpleClosureExprMul
)

type simpleClosureExpr struct {
	kind  simpleClosureExprKind
	value int64
	upval int
	left  *simpleClosureExpr
	right *simpleClosureExpr
}

type accumulatorDeltaKind uint8

const (
	accumulatorDeltaConst accumulatorDeltaKind = iota
	accumulatorDeltaUpval
)

var (
	accumulatorClosureProgramFastPathsMu sync.RWMutex
	accumulatorClosureProgramFastPaths   = make(map[*vm.FuncProto][]accumulatorClosureFastPath)

	simpleClosureExprProgramFastPathsMu sync.RWMutex
	simpleClosureExprProgramFastPaths   = make(map[*vm.FuncProto][]simpleClosureExprFastPath)

	immediateClosureFactoryProgramFastPathsMu sync.RWMutex
	immediateClosureFactoryProgramFastPaths   = make(map[*vm.FuncProto][]immediateClosureFactoryFastPath)
)

// emitBaselineNativeCall emits a native ARM64 call sequence for OP_CALL.
// For compiled vm.Closure targets, this uses BLR instead of exit-resume.
// For all other cases, falls through to the slow path (exit-resume).
//
// Parameters:
//   - asm: the assembler
//   - inst: the OP_CALL instruction
//   - pc: the bytecode PC of this instruction
//   - callerProto: the caller's FuncProto (for MaxStack)

func emitImmediateClosureFactoryExprValue(asm *jit.Assembler, expr simpleClosureExpr, argSlot, factoryArgBase int, upvalSlots []int, dst, rhs, tagScratch jit.Reg, missLabel string) {
	switch expr.kind {
	case simpleClosureExprParam:
		loadSlot(asm, dst, argSlot)
		emitCheckIsIntPinned(asm, dst, tagScratch)
		asm.BCond(jit.CondNE, missLabel)
		jit.EmitUnboxInt(asm, dst, dst)
	case simpleClosureExprIntConst:
		asm.LoadImm64(dst, expr.value)
	case simpleClosureExprUpval:
		if expr.upval < 0 || expr.upval >= len(upvalSlots) {
			asm.B(missLabel)
			return
		}
		loadSlot(asm, dst, factoryArgBase+upvalSlots[expr.upval])
		emitCheckIsIntPinned(asm, dst, tagScratch)
		asm.BCond(jit.CondNE, missLabel)
		jit.EmitUnboxInt(asm, dst, dst)
	case simpleClosureExprAdd, simpleClosureExprMul:
		if expr.left == nil || expr.right == nil {
			asm.B(missLabel)
			return
		}
		emitImmediateClosureFactoryExprValue(asm, *expr.left, argSlot, factoryArgBase, upvalSlots, dst, rhs, tagScratch, missLabel)
		emitImmediateClosureFactoryExprValue(asm, *expr.right, argSlot, factoryArgBase, upvalSlots, rhs, dst, tagScratch, missLabel)
		switch expr.kind {
		case simpleClosureExprAdd:
			asm.ADDreg(dst, dst, rhs)
		case simpleClosureExprMul:
			asm.MUL(dst, dst, rhs)
		}
		asm.SBFX(tagScratch, dst, 0, 48)
		asm.CMPreg(tagScratch, dst)
		asm.BCond(jit.CondNE, missLabel)
	default:
		asm.B(missLabel)
	}
}

func emitLoadClosureUpvalueRef(asm *jit.Assembler, closureReg jit.Reg, upval, upvalCount int, dstRefReg, upvalReg, dataReg jit.Reg, slowLabel string) {
	if upvalCount == 1 && upval == 0 {
		asm.LDR(upvalReg, closureReg, vmClosureOffInlineUpvalue0)
	} else {
		asm.LDR(dataReg, closureReg, vmClosureOffUpvalues)
		asm.CBZ(dataReg, slowLabel)
		asm.LDR(upvalReg, dataReg, upval*8)
	}
	asm.CBZ(upvalReg, slowLabel)
	asm.LDR(dstRefReg, upvalReg, 0)
	asm.CBZ(dstRefReg, slowLabel)
}

func emitLoadClosureTwoUpvalueRefs(asm *jit.Assembler, closureReg jit.Reg, upval1, upval2, upvalCount int, dstRef1Reg, dstRef2Reg, dataReg, upval1Reg, upval2Reg jit.Reg, slowLabel string) {
	if upval1 < 0 || upval2 < 0 || upval1 >= upvalCount || upval2 >= upvalCount || upval1 == upval2 {
		asm.B(slowLabel)
		return
	}
	if upvalCount == 2 && ((upval1 == 0 && upval2 == 1) || (upval1 == 1 && upval2 == 0)) {
		if upval1 == 0 {
			asm.LDP(upval1Reg, upval2Reg, closureReg, vmClosureOffInlineUpvalue0)
		} else {
			asm.LDP(upval2Reg, upval1Reg, closureReg, vmClosureOffInlineUpvalue0)
		}
		asm.CBZ(upval1Reg, slowLabel)
		asm.CBZ(upval2Reg, slowLabel)
		asm.LDR(dstRef1Reg, upval1Reg, 0)
		asm.LDR(dstRef2Reg, upval2Reg, 0)
		asm.CBZ(dstRef1Reg, slowLabel)
		asm.CBZ(dstRef2Reg, slowLabel)
		return
	}
	asm.LDR(dataReg, closureReg, vmClosureOffUpvalues)
	asm.CBZ(dataReg, slowLabel)
	if upval1+1 == upval2 {
		asm.LDP(upval1Reg, upval2Reg, dataReg, upval1*8)
	} else if upval2+1 == upval1 {
		asm.LDP(upval2Reg, upval1Reg, dataReg, upval2*8)
	} else {
		asm.LDR(upval1Reg, dataReg, upval1*8)
		asm.LDR(upval2Reg, dataReg, upval2*8)
	}
	asm.CBZ(upval1Reg, slowLabel)
	asm.CBZ(upval2Reg, slowLabel)
	asm.LDR(dstRef1Reg, upval1Reg, 0)
	asm.LDR(dstRef2Reg, upval2Reg, 0)
	asm.CBZ(dstRef1Reg, slowLabel)
	asm.CBZ(dstRef2Reg, slowLabel)
}

func registerAccumulatorClosureFastPaths(root *vm.FuncProto) {
	if root == nil {
		return
	}
	fastPaths := collectAccumulatorClosureFastPaths(root)
	exprFastPaths := collectSimpleClosureExprFastPaths(root)
	factoryFastPaths := collectImmediateClosureFactoryFastPaths(root)
	if len(fastPaths) == 0 && len(exprFastPaths) == 0 && len(factoryFastPaths) == 0 {
		return
	}
	protos := collectProtoTree(root)
	accumulatorClosureProgramFastPathsMu.Lock()
	for _, proto := range protos {
		if len(fastPaths) != 0 {
			accumulatorClosureProgramFastPaths[proto] = fastPaths
		}
	}
	accumulatorClosureProgramFastPathsMu.Unlock()
	simpleClosureExprProgramFastPathsMu.Lock()
	for _, proto := range protos {
		if len(exprFastPaths) != 0 {
			simpleClosureExprProgramFastPaths[proto] = exprFastPaths
		}
	}
	simpleClosureExprProgramFastPathsMu.Unlock()
	immediateClosureFactoryProgramFastPathsMu.Lock()
	for _, proto := range protos {
		if len(factoryFastPaths) != 0 {
			immediateClosureFactoryProgramFastPaths[proto] = factoryFastPaths
		}
	}
	immediateClosureFactoryProgramFastPathsMu.Unlock()
}

func accumulatorClosureFastPathsForProto(proto *vm.FuncProto) []accumulatorClosureFastPath {
	if proto == nil {
		return nil
	}
	accumulatorClosureProgramFastPathsMu.RLock()
	if fastPaths := accumulatorClosureProgramFastPaths[proto]; len(fastPaths) != 0 {
		accumulatorClosureProgramFastPathsMu.RUnlock()
		return fastPaths
	}
	accumulatorClosureProgramFastPathsMu.RUnlock()
	return collectAccumulatorClosureFastPaths(proto)
}

func simpleClosureExprFastPathsForProto(proto *vm.FuncProto) []simpleClosureExprFastPath {
	if proto == nil {
		return nil
	}
	simpleClosureExprProgramFastPathsMu.RLock()
	if fastPaths := simpleClosureExprProgramFastPaths[proto]; len(fastPaths) != 0 {
		simpleClosureExprProgramFastPathsMu.RUnlock()
		return fastPaths
	}
	simpleClosureExprProgramFastPathsMu.RUnlock()
	return collectSimpleClosureExprFastPaths(proto)
}

func immediateClosureFactoryFastPathsForProto(proto *vm.FuncProto) []immediateClosureFactoryFastPath {
	if proto == nil {
		return nil
	}
	immediateClosureFactoryProgramFastPathsMu.RLock()
	if fastPaths := immediateClosureFactoryProgramFastPaths[proto]; len(fastPaths) != 0 {
		immediateClosureFactoryProgramFastPathsMu.RUnlock()
		return fastPaths
	}
	immediateClosureFactoryProgramFastPathsMu.RUnlock()
	return collectImmediateClosureFactoryFastPaths(proto)
}

func collectProtoTree(root *vm.FuncProto) []*vm.FuncProto {
	seen := make(map[*vm.FuncProto]bool)
	var out []*vm.FuncProto
	var walk func(*vm.FuncProto)
	walk = func(proto *vm.FuncProto) {
		if proto == nil || seen[proto] {
			return
		}
		seen[proto] = true
		out = append(out, proto)
		for _, child := range proto.Protos {
			walk(child)
		}
	}
	walk(root)
	return out
}

func collectAccumulatorClosureFastPaths(root *vm.FuncProto) []accumulatorClosureFastPath {
	seen := make(map[*vm.FuncProto]bool)
	var out []accumulatorClosureFastPath
	var walk func(*vm.FuncProto)
	walk = func(proto *vm.FuncProto) {
		if proto == nil || seen[proto] {
			return
		}
		seen[proto] = true
		if fast, ok := accumulatorClosurePattern(proto); ok {
			out = append(out, fast)
		}
		for _, child := range proto.Protos {
			walk(child)
		}
	}
	walk(root)
	return out
}

func collectSimpleClosureExprFastPaths(root *vm.FuncProto) []simpleClosureExprFastPath {
	seen := make(map[*vm.FuncProto]bool)
	var out []simpleClosureExprFastPath
	var walk func(*vm.FuncProto)
	walk = func(proto *vm.FuncProto) {
		if proto == nil || seen[proto] {
			return
		}
		seen[proto] = true
		if expr, ok := simpleClosureExprPattern(proto); ok {
			out = append(out, simpleClosureExprFastPath{proto: proto, expr: expr})
		}
		for _, child := range proto.Protos {
			walk(child)
		}
	}
	walk(root)
	return out
}

func collectImmediateClosureFactoryFastPaths(root *vm.FuncProto) []immediateClosureFactoryFastPath {
	seen := make(map[*vm.FuncProto]bool)
	var out []immediateClosureFactoryFastPath
	var walk func(*vm.FuncProto)
	walk = func(proto *vm.FuncProto) {
		if proto == nil || seen[proto] {
			return
		}
		seen[proto] = true
		if fast, ok := immediateClosureFactoryPattern(proto); ok {
			out = append(out, fast)
		}
		for _, child := range proto.Protos {
			walk(child)
		}
	}
	walk(root)
	return out
}

func accumulatorClosurePattern(proto *vm.FuncProto) (accumulatorClosureFastPath, bool) {
	if proto == nil || proto.NumParams != 0 || proto.IsVarArg || len(proto.Code) != 6 {
		return accumulatorClosureFastPath{}, false
	}
	if vm.DecodeOp(proto.Code[0]) != vm.OP_GETUPVAL ||
		vm.DecodeOp(proto.Code[2]) != vm.OP_ADD ||
		vm.DecodeOp(proto.Code[3]) != vm.OP_SETUPVAL ||
		vm.DecodeOp(proto.Code[4]) != vm.OP_GETUPVAL ||
		vm.DecodeOp(proto.Code[5]) != vm.OP_RETURN {
		return accumulatorClosureFastPath{}, false
	}
	uv := vm.DecodeB(proto.Code[0])
	if uv < 0 || uv >= len(proto.Upvalues) {
		return accumulatorClosureFastPath{}, false
	}
	if vm.DecodeB(proto.Code[3]) != uv || vm.DecodeB(proto.Code[4]) != uv {
		return accumulatorClosureFastPath{}, false
	}
	loadReg := vm.DecodeA(proto.Code[0])
	addDst := vm.DecodeA(proto.Code[2])
	addB := vm.DecodeB(proto.Code[2])
	addC := vm.DecodeC(proto.Code[2])
	if addDst != vm.DecodeA(proto.Code[3]) {
		return accumulatorClosureFastPath{}, false
	}
	retReg := vm.DecodeA(proto.Code[4])
	if vm.DecodeA(proto.Code[5]) != retReg || vm.DecodeB(proto.Code[5]) != 2 {
		return accumulatorClosureFastPath{}, false
	}

	fast := accumulatorClosureFastPath{proto: proto, valueUpval: uv}
	switch vm.DecodeOp(proto.Code[1]) {
	case vm.OP_LOADINT:
		constReg := vm.DecodeA(proto.Code[1])
		if !((addB == loadReg && addC == constReg) || (addB == constReg && addC == loadReg)) {
			return accumulatorClosureFastPath{}, false
		}
		fast.deltaKind = accumulatorDeltaConst
		fast.delta = int64(vm.DecodesBx(proto.Code[1]))
		return fast, true
	case vm.OP_GETUPVAL:
		deltaReg := vm.DecodeA(proto.Code[1])
		deltaUpval := vm.DecodeB(proto.Code[1])
		if deltaUpval < 0 || deltaUpval >= len(proto.Upvalues) {
			return accumulatorClosureFastPath{}, false
		}
		if !((addB == loadReg && addC == deltaReg) || (addB == deltaReg && addC == loadReg)) {
			return accumulatorClosureFastPath{}, false
		}
		fast.deltaKind = accumulatorDeltaUpval
		fast.deltaUpval = deltaUpval
		return fast, true
	default:
		return accumulatorClosureFastPath{}, false
	}
}

func simpleClosureExprPattern(proto *vm.FuncProto) (simpleClosureExpr, bool) {
	if proto == nil || proto.NumParams != 1 || proto.IsVarArg || len(proto.Code) < 2 || len(proto.Code) > 6 {
		return simpleClosureExpr{}, false
	}
	exprs := map[int]simpleClosureExpr{
		0: {kind: simpleClosureExprParam},
	}
	for pc, inst := range proto.Code {
		op := vm.DecodeOp(inst)
		if op == vm.OP_RETURN {
			if pc != len(proto.Code)-1 || vm.DecodeB(inst) != 2 {
				return simpleClosureExpr{}, false
			}
			retReg := vm.DecodeA(inst)
			expr, ok := exprs[retReg]
			if !ok || simpleClosureExprCost(expr) > 6 {
				return simpleClosureExpr{}, false
			}
			return expr, true
		}
		a := vm.DecodeA(inst)
		switch op {
		case vm.OP_LOADINT:
			exprs[a] = simpleClosureExpr{kind: simpleClosureExprIntConst, value: int64(vm.DecodesBx(inst))}
		case vm.OP_GETUPVAL:
			uv := vm.DecodeB(inst)
			if uv < 0 || uv >= len(proto.Upvalues) {
				return simpleClosureExpr{}, false
			}
			exprs[a] = simpleClosureExpr{kind: simpleClosureExprUpval, upval: uv}
		case vm.OP_ADD, vm.OP_MUL:
			left, ok := exprs[vm.DecodeB(inst)]
			if !ok {
				return simpleClosureExpr{}, false
			}
			right, ok := exprs[vm.DecodeC(inst)]
			if !ok {
				return simpleClosureExpr{}, false
			}
			kind := simpleClosureExprAdd
			if op == vm.OP_MUL {
				kind = simpleClosureExprMul
			}
			leftCopy := left
			rightCopy := right
			exprs[a] = simpleClosureExpr{kind: kind, left: &leftCopy, right: &rightCopy}
		default:
			return simpleClosureExpr{}, false
		}
	}
	return simpleClosureExpr{}, false
}

func immediateClosureFactoryPattern(proto *vm.FuncProto) (immediateClosureFactoryFastPath, bool) {
	if proto == nil || proto.NumParams == 0 || proto.IsVarArg || len(proto.Protos) == 0 || len(proto.Code) < 2 {
		return immediateClosureFactoryFastPath{}, false
	}
	if vm.DecodeOp(proto.Code[0]) != vm.OP_CLOSURE || vm.DecodeOp(proto.Code[1]) != vm.OP_RETURN {
		return immediateClosureFactoryFastPath{}, false
	}
	closureReg := vm.DecodeA(proto.Code[0])
	if vm.DecodeA(proto.Code[1]) != closureReg || vm.DecodeB(proto.Code[1]) != 2 {
		return immediateClosureFactoryFastPath{}, false
	}
	childIdx := vm.DecodeBx(proto.Code[0])
	if childIdx < 0 || childIdx >= len(proto.Protos) {
		return immediateClosureFactoryFastPath{}, false
	}
	for i := 2; i < len(proto.Code); i++ {
		op := vm.DecodeOp(proto.Code[i])
		if op != vm.OP_CLOSE && op != vm.OP_RETURN {
			return immediateClosureFactoryFastPath{}, false
		}
	}
	child := proto.Protos[childIdx]
	expr, ok := simpleClosureExprPattern(child)
	if !ok || len(child.Upvalues) == 0 {
		return immediateClosureFactoryFastPath{}, false
	}
	upvalSlots := make([]int, len(child.Upvalues))
	for i, desc := range child.Upvalues {
		if !desc.InStack || desc.Index < 0 || desc.Index >= proto.NumParams {
			return immediateClosureFactoryFastPath{}, false
		}
		upvalSlots[i] = desc.Index
	}
	return immediateClosureFactoryFastPath{proto: proto, expr: expr, upvalSlots: upvalSlots}, true
}

func simpleClosureExprCost(expr simpleClosureExpr) int {
	switch expr.kind {
	case simpleClosureExprParam, simpleClosureExprIntConst, simpleClosureExprUpval:
		return 1
	case simpleClosureExprAdd, simpleClosureExprMul:
		if expr.left == nil || expr.right == nil {
			return 1000
		}
		return 1 + simpleClosureExprCost(*expr.left) + simpleClosureExprCost(*expr.right)
	default:
		return 1000
	}
}

func emitBaselineSelfTailNoReturnFastPath(asm *jit.Assembler, inst uint32, pc int, callerProto *vm.FuncProto, slowLabel string) bool {
	if !isBaselineStaticSelfTailNoReturnCall(callerProto, inst, pc) {
		return false
	}
	a := vm.DecodeA(inst)
	nArgs := vm.DecodeB(inst) - 1
	fallthroughLabel := nextLabel("self_tail_fallthrough")

	loadSlot(asm, jit.X0, a)
	asm.CMPreg(jit.X0, mRegSelfClosure)
	asm.BCond(jit.CondNE, fallthroughLabel)

	// Preserve the existing tiering trigger: the threshold call exits through
	// Go so the TieringManager can attempt promotion. Calls above/below the
	// threshold stay in-frame and avoid the native-call stack entirely.
	asm.LoadImm64(jit.X1, int64(uintptr(unsafe.Pointer(callerProto))))
	asm.LDR(jit.X3, jit.X1, funcProtoOffCallCount)
	asm.ADDimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, jit.X1, funcProtoOffCallCount)
	asm.CMPimm(jit.X3, tmDefaultTier2Threshold)
	asm.BCond(jit.CondEQ, slowLabel)

	scratch := []jit.Reg{jit.X4, jit.X5, jit.X6, jit.X7}
	for i := 0; i < nArgs; i++ {
		loadSlot(asm, scratch[i], a+1+i)
	}
	for i := 0; i < nArgs; i++ {
		storeSlot(asm, i, scratch[i])
	}
	asm.B(pcLabel(0))

	asm.Label(fallthroughLabel)
	return true
}

func isBaselineStaticSelfTailNoReturnCall(proto *vm.FuncProto, inst uint32, pc int) bool {
	if proto == nil || proto.IsVarArg || !baselineSelfTailNoReturnSafe(proto) {
		return false
	}
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	c := vm.DecodeC(inst)
	if b == 0 || c != 1 {
		return false
	}
	nArgs := b - 1
	if nArgs != proto.NumParams || nArgs > 4 {
		return false
	}
	if pc+1 >= len(proto.Code) {
		return false
	}
	next := proto.Code[pc+1]
	if vm.DecodeOp(next) != vm.OP_RETURN || vm.DecodeB(next) != 1 {
		return false
	}
	return isBaselineStaticSelfCall(proto, pc, a)
}

func baselineSelfTailNoReturnSafe(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_CLOSURE, vm.OP_CLOSE, vm.OP_GETUPVAL, vm.OP_SETUPVAL, vm.OP_VARARG:
			return false
		}
	}
	return true
}

// emitDirectEntryPrologue emits the lightweight direct entry point for native BLR
// calls. This is placed after the normal prologue and before the first bytecode.
// It only saves FP+LR (16 bytes) and reloads pinned registers from ctx.
func emitDirectEntryPrologue(asm *jit.Assembler) {
	asm.Label("direct_entry")
	// Save FP+LR with pre-index (SP -= 16)
	asm.STPpre(jit.X29, jit.X30, jit.SP, -16)
	asm.ADDimm(jit.X29, jit.SP, 0) // FP = SP

	// Set up pinned registers from ctx (X0 = ctx, set by caller)
	asm.MOVreg(mRegCtx, jit.X0)                       // X19 = ctx
	asm.LDR(mRegRegs, mRegCtx, execCtxOffRegs)        // X26 = ctx.Regs
	asm.LDR(mRegConsts, mRegCtx, execCtxOffConstants) // X27 = ctx.Constants
	// X24 (tagInt) and X25 (tagBool) are callee-saved, preserved from caller.

	// Cache NaN-boxed self-closure for fast self-call detection.
	asm.LDR(mRegSelfClosure, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.LoadImm64(jit.X3, nbClosureTagBits)
	asm.ORRreg(mRegSelfClosure, mRegSelfClosure, jit.X3)

	// Pin R(0): load from callee's register window.
	asm.LDR(mRegR0, mRegRegs, 0)

	// Jump to first bytecode.
	asm.B("pc_0")
}

func isBaselineStaticSelfCall(proto *vm.FuncProto, callPC, callA int) bool {
	if proto == nil || callPC <= 0 || callPC >= len(proto.Code) {
		return false
	}
	for pc := callPC - 1; pc >= 0; pc-- {
		inst := proto.Code[pc]
		op := vm.DecodeOp(inst)
		a := vm.DecodeA(inst)
		if op == vm.OP_GETGLOBAL && a == callA {
			bx := vm.DecodeBx(inst)
			return bx >= 0 && bx < len(proto.Constants) && proto.Constants[bx].IsString() && proto.Constants[bx].Str() == proto.Name
		}
		if baselineOpWritesSlot(op) && a == callA {
			return false
		}
	}
	return false
}

func baselineOpWritesSlot(op vm.Opcode) bool {
	switch op {
	case vm.OP_JMP, vm.OP_EQ, vm.OP_LT, vm.OP_LE, vm.OP_TEST, vm.OP_SETGLOBAL,
		vm.OP_SETUPVAL, vm.OP_CLOSE, vm.OP_RETURN, vm.OP_TFORLOOP,
		vm.OP_GO, vm.OP_SEND, vm.OP_TRYSEND:
		return false
	default:
		return true
	}
}

// emitSelfCallEntryPrologue emits a lightweight entry point used only by
// self-call BL instructions. For self-calls, the caller and callee are the
// same function, so:
//   - X19 (mRegCtx) is already set (same context)
//   - X26 (mRegRegs) was already updated by the caller's step 6
//   - X27 (mRegConsts) is preserved (same proto → same constants)
//
// This avoids the MOVreg X19,X0 and the two LDR for Regs/Constants that
// the normal direct_entry prologue performs.
func emitSelfCallEntryPrologue(asm *jit.Assembler) {
	asm.Label("self_call_entry")
	// Save FP+LR with pre-index (SP -= 16)
	asm.STPpre(jit.X29, jit.X30, jit.SP, -16)
	asm.ADDimm(jit.X29, jit.SP, 0) // FP = SP
	// Skip: MOVreg X19, X0 — X19 already holds ctx for self-call
	// Skip: LDR X26 from ctx.Regs — already set by caller's step 6
	// Skip: LDR X27 from ctx.Constants — same function, preserved

	// Pin R(0): load from callee's register window.
	// For fixed-arg self-calls, X22 was already set by the caller's arg copy,
	// but we reload for safety (covers vararg self-calls).
	asm.LDR(mRegR0, mRegRegs, 0)

	asm.B("pc_0")
}

// emitDirectExitEpilogue emits the direct exit path for functions entered via
// native BLR. RETURN jumps here when CallMode == 1.
func emitDirectExitEpilogue(asm *jit.Assembler) {
	asm.Label("direct_epilogue")
	asm.MOVimm16(jit.X0, 0) // ExitNormal
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)

	asm.Label("direct_exit")
	// Restore FP+LR with post-index (SP += 16)
	asm.LDPpost(jit.X29, jit.X30, jit.SP, 16)
	asm.RET()
}
