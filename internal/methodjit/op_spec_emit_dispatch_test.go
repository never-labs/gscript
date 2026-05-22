//go:build darwin && arm64

package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestOpEmitterFamiliesMatchEmitDispatch(t *testing.T) {
	handled := emitInstrHandledOps(t)
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.EmitterFamily == OpEmitterMatrix {
			continue
		}
		dispatchFamily, inDispatchTable := emitInstrDispatchFamily(op)
		_, inEmitSwitch := handled[op]
		if inDispatchTable != inEmitSwitch {
			t.Fatalf("%s emit_dispatch case=%v, ownership table=%v", spec.Name, inEmitSwitch, inDispatchTable)
		}
		if inEmitSwitch {
			if spec.EmitterFamily != dispatchFamily {
				t.Fatalf("%s OpSpec family=%v, emit_dispatch family=%v", spec.Name, spec.EmitterFamily, dispatchFamily)
			}
			continue
		}
		if !opIntentionallyNotHandledByEmitInstr(op) {
			t.Fatalf("%s has OpSpec family %v but is not handled by emit_dispatch", spec.Name, spec.EmitterFamily)
		}
	}
}

func emitInstrHandledOps(t *testing.T) map[Op]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "emit_dispatch.go", nil, 0)
	if err != nil {
		t.Fatalf("parse emit_dispatch.go: %v", err)
	}
	ops := make(map[Op]bool)
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "emitInstr" {
			continue
		}
		found = true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc := stmt.(*ast.CaseClause)
				for _, expr := range cc.List {
					if ident, ok := expr.(*ast.Ident); ok {
						if op, ok := opByName(ident.Name); ok {
							ops[op] = true
						}
					}
				}
			}
			return false
		})
	}
	if !found {
		t.Fatal("emitInstr not found in emit_dispatch.go")
	}
	return ops
}

func opByName(name string) (Op, bool) {
	if name == "OpTableArrayNestedLoad" {
		return OpTableArrayNestedLoad, true
	}
	for op := Op(0); op < OpMax; op++ {
		if "Op"+op.String() == name {
			return op, true
		}
	}
	return 0, false
}

func opIntentionallyNotHandledByEmitInstr(op Op) bool {
	switch op {
	case OpBoxInt, OpBoxFloat, OpUnboxInt, OpUnboxFloat:
		return true
	case OpYield:
		return true
	default:
		return false
	}
}

func emitInstrDispatchFamily(op Op) (OpEmitterFamily, bool) {
	switch op {
	case OpConstInt, OpConstNil, OpConstBool, OpConstFloat, OpConstString:
		return OpEmitterConst, true
	case OpLoadSlot, OpStoreSlot:
		return OpEmitterSlot, true
	case OpAdd, OpSub, OpMul, OpMod, OpAddInt, OpSubInt, OpMulInt, OpModInt,
		OpDivIntExact, OpAddFloat, OpSubFloat, OpMulFloat, OpDiv, OpDivFloat,
		OpUnm, OpNegInt, OpNegFloat, OpSqrt, OpFloor, OpFMA, OpFMSUB, OpNot,
		OpPow, OpLen:
		return OpEmitterArithmetic, true
	case OpMatrixDense, OpMatrixGetF, OpMatrixSetF, OpMatrixFlat, OpMatrixStride,
		OpMatrixLoadFAt, OpMatrixStoreFAt, OpMatrixRowPtr, OpMatrixLoadFRow,
		OpMatrixStoreFRow, OpMatrixLoadFRowConst, OpMatrixStoreFRowConst:
		return OpEmitterMatrix, true
	case OpComplexEscapeInSet, OpComplexEscapeRowCount, OpRecordArrayLoopKernel:
		return OpEmitterKernel, true
	case OpLt, OpLe, OpEq, OpLtInt, OpLeInt, OpEqInt, OpEqString, OpModZeroInt,
		OpLtFloat, OpLeFloat:
		return OpEmitterCompare, true
	case OpConcat, OpStringConstLookup, OpStringFormatInt, OpStringFormatConst,
		OpStringFormatConstLen, OpGetTableStringFormatInt, OpStringSplitPart,
		OpStringSplitSubstr, OpStringSplitSubstrNumber:
		return OpEmitterString, true
	case OpNewTable, OpNewFixedTable, OpGetTable, OpSetTable, OpTableArrayHeader,
		OpTableArrayLen, OpTableArrayData, OpTableArrayLoad, OpTableShapeID,
		OpTableArrayStore, OpTableArraySwap, OpTableArraySwapPairs,
		OpTableBoolArrayFill, OpTableBoolArrayCount, OpTableIntArrayReversePrefix,
		OpTableIntArrayCopyPrefix, OpTableArrayNestedLoad, OpSetList, OpAppend:
		return OpEmitterTable, true
	case OpGetField, OpGetFieldNumToFloat, OpFieldPolyLen, OpFieldSvals,
		OpFieldLoad, OpFieldLoadNumToFloat, OpFieldStore, OpSetField:
		return OpEmitterField, true
	case OpGetGlobal, OpSetGlobal:
		return OpEmitterGlobal, true
	case OpGetUpval, OpSetUpval:
		return OpEmitterUpvalue, true
	case OpNumToFloat:
		return OpEmitterConversion, true
	case OpGuardType, OpGuardIntRange, OpGuardGlobalConst, OpGuardConstString,
		OpGuardTableKind, OpGuardCalleeProto, OpGuardFieldCalleeProto,
		OpGuardShapeFieldType, OpGuardShapeFieldTypeMask, OpGuardTruthy,
		OpGuardNonNil:
		return OpEmitterGuard, true
	case OpJump, OpBranch, OpReturn, OpTestSet:
		return OpEmitterControl, true
	case OpCall, OpCallFloor, OpFieldCallFloor, OpResume, OpSelf:
		return OpEmitterCall, true
	case OpForPrep, OpForLoop, OpTForCall, OpTForLoop:
		return OpEmitterLoop, true
	case OpClosure, OpClose:
		return OpEmitterClosure, true
	case OpVararg:
		return OpEmitterVararg, true
	case OpGo, OpMakeChan, OpSend, OpRecv:
		return OpEmitterConcurrency, true
	case OpPhi:
		return OpEmitterPhi, true
	case OpNop:
		return OpEmitterSpecial, true
	default:
		return OpEmitterInvalid, false
	}
}
