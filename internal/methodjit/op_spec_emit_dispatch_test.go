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
		dispatchFamily, isHandled := handled[op]
		if isHandled {
			if dispatchFamily != OpEmitterInvalid && spec.EmitterFamily != dispatchFamily {
				t.Fatalf("%s OpSpec family=%v, emitter helper family=%v", spec.Name, spec.EmitterFamily, dispatchFamily)
			}
			continue
		}
		if !opIntentionallyNotHandledByEmitInstr(op) {
			t.Fatalf("%s has OpSpec family %v but is not handled by emit_dispatch", spec.Name, spec.EmitterFamily)
		}
	}
}

func TestEmitInstrDelegatesRegisteredEmitterFamilies(t *testing.T) {
	calls := emitterMethodCalls(t, "emit_dispatch.go", "emitInstr")
	for _, delegate := range emitterFamilyDelegateRegistry() {
		if !calls[delegate.funcName] {
			t.Fatalf("emitInstr does not delegate to %s for family %v", delegate.funcName, delegate.family)
		}
	}
}

func TestOpBackendPolicyMatchesEmitDispatchExplicitCleanup(t *testing.T) {
	explicitClears := emitInstrOpsCalling(t, "clearTableArrayBoundedKeys")
	for _, delegate := range emitterFamilyDelegateRegistry() {
		mergeOps(explicitClears, emitterOpsCalling(t, delegate.filename, delegate.funcName, "clearTableArrayBoundedKeys"))
	}
	shapeInvalidations := emitInstrOpsInvalidatingShape(t)
	for _, delegate := range emitterFamilyDelegateRegistry() {
		mergeOps(shapeInvalidations, emitterOpsInvalidatingShape(t, delegate.filename, delegate.funcName))
	}
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

		wantUnknown = policy&OpBackendPreservesScratchFPRCache != 0
		wantFloat = wantUnknown || policy&OpBackendPreservesScratchFPRCacheForFloatResult != 0
		if got := instrPreservesScratchFPRCache(unknownInstr); got != wantUnknown {
			t.Fatalf("%s scratch FPR preserve helper with unknown type=%v, policy=%v", spec.Name, got, wantUnknown)
		}
		if got := instrPreservesScratchFPRCache(floatInstr); got != wantFloat {
			t.Fatalf("%s scratch FPR preserve helper with float type=%v, policy=%v", spec.Name, got, wantFloat)
		}
	}
}

func TestScratchFPRCachePreservePolicyBoundaries(t *testing.T) {
	floatResultOps := []Op{
		OpAddFloat,
		OpSubFloat,
		OpMulFloat,
		OpDivFloat,
		OpNegFloat,
		OpSqrt,
		OpFMA,
		OpFMSUB,
	}
	for _, op := range floatResultOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.BackendPolicy&OpBackendPreservesScratchFPRCacheForFloatResult == 0 {
			t.Fatalf("%s missing scratch FPR float-result preserve policy", spec.Name)
		}
		if instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeUnknown}) {
			t.Fatalf("%s preserves scratch FPR cache without float result type", spec.Name)
		}
		if !instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeFloat}) {
			t.Fatalf("%s does not preserve scratch FPR cache with float result type", spec.Name)
		}
	}

	generalOps := []Op{OpLtFloat, OpLeFloat, OpNop}
	for _, op := range generalOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.BackendPolicy&OpBackendPreservesScratchFPRCache == 0 {
			t.Fatalf("%s missing unconditional scratch FPR preserve policy", spec.Name)
		}
		if !instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeUnknown}) {
			t.Fatalf("%s should preserve scratch FPR cache independent of result type", spec.Name)
		}
	}

	clearingOps := []Op{OpAddInt, OpLtInt, OpNumToFloat, OpBoxFloat}
	for _, op := range clearingOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.BackendPolicy&(OpBackendPreservesScratchFPRCache|OpBackendPreservesScratchFPRCacheForFloatResult) != 0 {
			t.Fatalf("%s unexpectedly declares scratch FPR preserve policy", spec.Name)
		}
		if instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeFloat}) {
			t.Fatalf("%s unexpectedly preserves scratch FPR cache", spec.Name)
		}
	}
}

func TestScratchFPRCachePreserveHelperUsesBackendPolicy(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "emit_dispatch.go", nil, 0)
	if err != nil {
		t.Fatalf("parse emit_dispatch.go: %v", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "instrPreservesScratchFPRCache" {
			continue
		}

		foundGeneralPolicy := false
		foundFloatResultPolicy := false
		foundSwitch := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SwitchStmt:
				foundSwitch = true
			case *ast.Ident:
				switch node.Name {
				case "OpBackendPreservesScratchFPRCache":
					foundGeneralPolicy = true
				case "OpBackendPreservesScratchFPRCacheForFloatResult":
					foundFloatResultPolicy = true
				}
			}
			return true
		})
		if foundSwitch {
			t.Fatalf("instrPreservesScratchFPRCache should be policy-based, not an op switch")
		}
		if !foundGeneralPolicy || !foundFloatResultPolicy {
			t.Fatalf("instrPreservesScratchFPRCache does not reference scratch FPR backend policies")
		}
		return
	}

	t.Fatalf("instrPreservesScratchFPRCache not found")
}

type emitterFamilyDelegate struct {
	filename string
	funcName string
	family   OpEmitterFamily
}

func emitterFamilyDelegateRegistry() []emitterFamilyDelegate {
	return []emitterFamilyDelegate{
		{filename: "emit_const.go", funcName: "emitConstInstr", family: OpEmitterConst},
		{filename: "emit_slot.go", funcName: "emitSlotInstr", family: OpEmitterSlot},
		{filename: "emit_arithmetic.go", funcName: "emitArithmeticInstr", family: OpEmitterArithmetic},
		{filename: "emit_compare.go", funcName: "emitCompareInstr", family: OpEmitterCompare},
		{filename: "emit_matrix.go", funcName: "emitMatrixInstr", family: OpEmitterMatrix},
		{filename: "emit_string.go", funcName: "emitStringInstr", family: OpEmitterString},
		{filename: "emit_table_field_instr.go", funcName: "emitTableInstr", family: OpEmitterTable},
		{filename: "emit_table_field_instr.go", funcName: "emitFieldInstr", family: OpEmitterField},
		{filename: "emit_guard_instr.go", funcName: "emitGuardInstr", family: OpEmitterGuard},
		{filename: "emit_call_instr.go", funcName: "emitCallInstr", family: OpEmitterCall},
		{filename: "emit_global_instr.go", funcName: "emitGlobalInstr", family: OpEmitterGlobal},
		{filename: "emit_misc_instr.go", funcName: "emitKernelInstr", family: OpEmitterKernel},
		{filename: "emit_control_instr.go", funcName: "emitControlInstr", family: OpEmitterControl},
		{filename: "emit_misc_instr.go", funcName: "emitUpvalueInstr", family: OpEmitterUpvalue},
		{filename: "emit_misc_instr.go", funcName: "emitConversionInstr", family: OpEmitterConversion},
		{filename: "emit_misc_instr.go", funcName: "emitLoopInstr", family: OpEmitterLoop},
		{filename: "emit_misc_instr.go", funcName: "emitClosureInstr", family: OpEmitterClosure},
		{filename: "emit_misc_instr.go", funcName: "emitVarargInstr", family: OpEmitterVararg},
		{filename: "emit_misc_instr.go", funcName: "emitConcurrencyInstr", family: OpEmitterConcurrency},
		{filename: "emit_phi_instr.go", funcName: "emitPhiInstr", family: OpEmitterPhi},
		{filename: "emit_misc_instr.go", funcName: "emitSpecialInstr", family: OpEmitterSpecial},
	}
}

func emitInstrHandledOps(t *testing.T) map[Op]OpEmitterFamily {
	t.Helper()

	ops := make(map[Op]OpEmitterFamily)
	for op := range emitInstrSwitchHandledOps(t) {
		ops[op] = OpEmitterInvalid
	}
	for _, delegate := range emitterFamilyDelegateRegistry() {
		for op := range emitterSwitchHandledOps(t, delegate.filename, delegate.funcName) {
			if existingFamily, exists := ops[op]; exists {
				t.Fatalf("%s handled by both emitInstr/direct family %v and %s", op, existingFamily, delegate.funcName)
			}
			ops[op] = delegate.family
		}
	}
	return ops
}

func emitInstrSwitchHandledOps(t *testing.T) map[Op]bool {
	t.Helper()
	return emitterSwitchHandledOps(t, "emit_dispatch.go", "emitInstr")
}

func emitterSwitchHandledOps(t *testing.T, filename, funcName string) map[Op]bool {
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
				for _, expr := range cc.List {
					if ident, ok := expr.(*ast.Ident); ok {
						if op, ok := opByName(ident.Name); ok {
							ops[op] = true
							continue
						}
						t.Fatalf("%s has unknown op case %s", funcName, ident.Name)
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

func emitterOpsInvalidatingShape(t *testing.T, filename, funcName string) map[Op]bool {
	t.Helper()
	return emitterOpsWithCaseBehavior(t, filename, funcName, func(cc *ast.CaseClause) bool {
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

func emitterMethodCalls(t *testing.T, filename, funcName string) map[string]bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	calls := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if ok {
				calls[sel.Sel.Name] = true
			}
			return true
		})
		return calls
	}

	t.Fatalf("%s not found in %s", funcName, filename)
	return nil
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
