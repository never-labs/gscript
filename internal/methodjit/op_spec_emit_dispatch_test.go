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
		if spec.EmitterFamily == OpEmitterMatrix || spec.EmitterFamily == OpEmitterString {
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

func TestOpBackendPolicyMatchesEmitDispatchExplicitCleanup(t *testing.T) {
	explicitClears := emitInstrOpsCalling(t, "clearTableArrayBoundedKeys")
	mergeOps(explicitClears, emitterOpsCalling(t, "emit_string.go", "emitStringInstr", "clearTableArrayBoundedKeys"))
	shapeInvalidations := emitInstrOpsInvalidatingShape(t)
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if got, want := spec.BackendPolicy&OpBackendClearsTableArrayBounds != 0, explicitClears[op]; got != want {
			t.Fatalf("%s clears table array bounds policy=%v, emit_dispatch=%v", spec.Name, got, want)
		}
		if got, want := spec.BackendPolicy&OpBackendInvalidatesShape != 0, shapeInvalidations[op]; got != want {
			t.Fatalf("%s invalidates shape policy=%v, emit_dispatch=%v", spec.Name, got, want)
		}
	}
}

func TestOpBackendPolicyMatchesDispatchPreserveHelpers(t *testing.T) {
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		policy := spec.BackendPolicy
		if got, want := instrPreservesTableArrayBoundedKeys(&Instr{Op: op}), policy&OpBackendPreservesTableArrayBounds != 0; got != want {
			t.Fatalf("%s table array bounds preserve helper=%v, policy=%v", spec.Name, got, want)
		}

		unknownInstr := &Instr{Op: op, Type: TypeUnknown}
		floatInstr := &Instr{Op: op, Type: TypeFloat}
		wantUnknown := policy&OpBackendPreservesFieldSvalsCache != 0
		wantFloat := wantUnknown || policy&OpBackendPreservesFieldSvalsCacheForFloatResult != 0
		if got := instrPreservesFieldSvalsCache(unknownInstr); got != wantUnknown {
			t.Fatalf("%s field svals preserve helper with unknown type=%v, policy=%v", spec.Name, got, wantUnknown)
		}
		if got := instrPreservesFieldSvalsCache(floatInstr); got != wantFloat {
			t.Fatalf("%s field svals preserve helper with float type=%v, policy=%v", spec.Name, got, wantFloat)
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

func emitInstrOpsCalling(t *testing.T, method string) map[Op]bool {
	t.Helper()
	return emitInstrOpsWithCaseBehavior(t, func(cc *ast.CaseClause) bool {
		found := false
		ast.Inspect(cc, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == method {
				found = true
				return false
			}
			return true
		})
		return found
	})
}

func emitInstrOpsInvalidatingShape(t *testing.T) map[Op]bool {
	t.Helper()
	return emitInstrOpsWithCaseBehavior(t, func(cc *ast.CaseClause) bool {
		found := false
		ast.Inspect(cc, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "shapeVerified" {
					found = true
					return false
				}
			}
			return true
		})
		return found
	})
}

func emitInstrOpsWithCaseBehavior(t *testing.T, hasBehavior func(*ast.CaseClause) bool) map[Op]bool {
	t.Helper()
	return emitterOpsWithCaseBehavior(t, "emit_dispatch.go", "emitInstr", hasBehavior)
}

func emitterOpsCalling(t *testing.T, filename, funcName, method string) map[Op]bool {
	t.Helper()
	return emitterOpsWithCaseBehavior(t, filename, funcName, func(cc *ast.CaseClause) bool {
		found := false
		ast.Inspect(cc, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == method {
				found = true
				return false
			}
			return true
		})
		return found
	})
}

func emitterOpsWithCaseBehavior(t *testing.T, filename, funcName string, hasBehavior func(*ast.CaseClause) bool) map[Op]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	ops := make(map[Op]bool)
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
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
				if !hasBehavior(cc) {
					continue
				}
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
		t.Fatalf("%s not found in %s", funcName, filename)
	}
	return ops
}

func mergeOps(dst, src map[Op]bool) {
	for op := range src {
		dst[op] = true
	}
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
