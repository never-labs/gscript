package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestOpSpecPolicyTablesDoNotExceedOpSpace(t *testing.T) {
	tables := []struct {
		name  string
		table any
	}{
		{"opBackendPolicies", opBackendPolicies},
		{"opKeepUnusedPolicies", opKeepUnusedPolicies},
		{"opNativeReplayMayExitPolicies", opNativeReplayMayExitPolicies},
		{"opNativeReplayVisibleSideEffectPolicies", opNativeReplayVisibleSideEffectPolicies},
		{"opNativeReplayVisibleTableMutationPolicies", opNativeReplayVisibleTableMutationPolicies},
		{"opNativeCalleeResumeUnsafePolicies", opNativeCalleeResumeUnsafePolicies},
		{"opRestartVisibleSideEffectPolicies", opRestartVisibleSideEffectPolicies},
		{"opFieldShapeSplitInlineSafePolicies", opFieldShapeSplitInlineSafePolicies},
		{"opFieldShapePreEffectInlineSafePolicies", opFieldShapePreEffectInlineSafePolicies},
		{"opFieldShapeInlineSideEffectPolicies", opFieldShapeInlineSideEffectPolicies},
		{"opFieldShapePostEffectInlineUnsafePolicies", opFieldShapePostEffectInlineUnsafePolicies},
		{"opGlobalConstUnsafePolicies", opGlobalConstUnsafePolicies},
		{"opNestedCallLikePolicies", opNestedCallLikePolicies},
		{"opLoadElimConstCSEPolicies", opLoadElimConstCSEPolicies},
		{"opLiteralConstPolicies", opLiteralConstPolicies},
		{"opLoadElimPureCSEPolicies", opLoadElimPureCSEPolicies},
		{"opLoadElimShapeFactKillerPolicies", opLoadElimShapeFactKillerPolicies},
		{"opNoSSAResultPolicies", opNoSSAResultPolicies},
		{"opRawIntResultPolicies", opRawIntResultPolicies},
		{"opRawTablePtrResultPolicies", opRawTablePtrResultPolicies},
		{"opRawDataPtrResultPolicies", opRawDataPtrResultPolicies},
		{"opRawFloatResultPolicies", opRawFloatResultPolicies},
		{"opMatrixNativePolicies", opMatrixNativePolicies},
		{"opTableArrayGPRInvariantPolicies", opTableArrayGPRInvariantPolicies},
		{"opTableArrayGPRInvariantRankPolicies", opTableArrayGPRInvariantRankPolicies},
		{"opTableArrayGPRInvariantUseMaskPolicies", opTableArrayGPRInvariantUseMaskPolicies},
		{"opTableArrayKeyArgIndexPolicies", opTableArrayKeyArgIndexPolicies},
		{"opLICMHoistablePolicies", opLICMHoistablePolicies},
		{"opLICMInterestingMissPolicies", opLICMInterestingMissPolicies},
		{"opLICMIntArithPolicies", opLICMIntArithPolicies},
		{"opPureNumericInlinePolicies", opPureNumericInlinePolicies},
		{"opNativeEffectLoopInlinePolicies", opNativeEffectLoopInlinePolicies},
		{"opDirectDeoptWithoutFullFlushPolicies", opDirectDeoptWithoutFullFlushPolicies},
		{"opGenericSpecializablePolicies", opGenericSpecializablePolicies},
		{"opTypeSpecializationPolicies", opTypeSpecializationPolicies},
		{"opNumToFloatInsertCandidatePolicies", opNumToFloatInsertCandidatePolicies},
		{"opIntRecurrencePolicies", opIntRecurrencePolicies},
		{"opNumericOperandPolicies", opNumericOperandPolicies},
		{"opFieldSvalsCrossBlockBarrierPolicies", opFieldSvalsCrossBlockBarrierPolicies},
		{"opFieldSvalsGlobalBarrierPolicies", opFieldSvalsGlobalBarrierPolicies},
		{"opFieldLenFoldBarrierPolicies", opFieldLenFoldBarrierPolicies},
		{"opFieldCallPolyLenFusionBarrierPolicies", opFieldCallPolyLenFusionBarrierPolicies},
		{"opBoxableIntArithmeticPolicies", opBoxableIntArithmeticPolicies},
		{"opUnsafeIntArithmeticCandidatePolicies", opUnsafeIntArithmeticCandidatePolicies},
		{"opInt48SafeRangeCandidatePolicies", opInt48SafeRangeCandidatePolicies},
		{"opExactDivAllowedExternalUsePolicies", opExactDivAllowedExternalUsePolicies},
		{"opNonNegativeDerivationCandidatePolicies", opNonNegativeDerivationCandidatePolicies},
		{"opNonNegativeDerivationKindPolicies", opNonNegativeDerivationKindPolicies},
		{"opInt48RuntimeValuePolicies", opInt48RuntimeValuePolicies},
		{"opFusableComparisonPolicies", opFusableComparisonPolicies},
		{"opLoopBoundComparisonPolicies", opLoopBoundComparisonPolicies},
		{"opConstPoolUserPolicies", opConstPoolUserPolicies},
		{"opRawStringResultPolicies", opRawStringResultPolicies},
		{"opDynamicStringQueryCacheKeyPolicies", opDynamicStringQueryCacheKeyPolicies},
		{"opUnrollCloneablePolicies", opUnrollCloneablePolicies},
		{"opNestedFloatPhiOverrideSafePolicies", opNestedFloatPhiOverrideSafePolicies},
		{"opFloatReductionWideUnrollBarrierPolicies", opFloatReductionWideUnrollBarrierPolicies},
		{"opFloatReductionLatencyUnrollSeedPolicies", opFloatReductionLatencyUnrollSeedPolicies},
		{"opFloatReductionLatencyUnrollBlockPolicies", opFloatReductionLatencyUnrollBlockPolicies},
		{"opFloatReductionDivOpPolicies", opFloatReductionDivOpPolicies},
		{"opConstantPhiBranchThreadPurePolicies", opConstantPhiBranchThreadPurePolicies},
		{"opNeedsTier2FieldCachePolicies", opNeedsTier2FieldCachePolicies},
		{"opFieldReadPolicies", opFieldReadPolicies},
		{"opFieldSlotLoadPolicies", opFieldSlotLoadPolicies},
		{"opFieldWritePolicies", opFieldWritePolicies},
		{"opBoolTableFillBodyBenignPolicies", opBoolTableFillBodyBenignPolicies},
		{"opBoolTableFillStorePolicies", opBoolTableFillStorePolicies},
		{"opBoolTableCountLoadBodyBenignPolicies", opBoolTableCountLoadBodyBenignPolicies},
		{"opBoolTableCountLoadPolicies", opBoolTableCountLoadPolicies},
		{"opBoolTableCountIncrementBenignPolicies", opBoolTableCountIncrementBenignPolicies},
		{"opBoolTableCountIncrementPolicies", opBoolTableCountIncrementPolicies},
		{"opCallResultRangeGuardCandidatePolicies", opCallResultRangeGuardCandidatePolicies},
		{"opModuloReducibleCallFloorPolicies", opModuloReducibleCallFloorPolicies},
		{"opCallFloorSpecStableCalleePolicies", opCallFloorSpecStableCalleePolicies},
		{"opCallFloorSpecFieldShapePolicies", opCallFloorSpecFieldShapePolicies},
		{"opTier2LoopCallPolicies", opTier2LoopCallPolicies},
		{"opTier2LoopFeedbackVMProtoCallPolicies", opTier2LoopFeedbackVMProtoCallPolicies},
		{"opTier2ResidualCallBlockerPolicies", opTier2ResidualCallBlockerPolicies},
		{"opTier2LoopNativeCandidatePolicies", opTier2LoopNativeCandidatePolicies},
		{"opCallUserArgStartPolicies", opCallUserArgStartPolicies},
		{"opSpeculativeIntUseCandidatePolicies", opSpeculativeIntUseCandidatePolicies},
		{"opFloatRegResultPolicies", opFloatRegResultPolicies},
		{"opFloatRegResultBlockedPolicies", opFloatRegResultBlockedPolicies},
		{"opRawIntCarryValuePolicies", opRawIntCarryValuePolicies},
		{"opTableResultRawTablePtrPolicies", opTableResultRawTablePtrPolicies},
		{"opTableArrayRegionGlobalBarrierPolicies", opTableArrayRegionGlobalBarrierPolicies},
		{"opTableArrayRegionAliasingCallPolicies", opTableArrayRegionAliasingCallPolicies},
		{"opTableArrayRegionAliasingAlwaysPolicies", opTableArrayRegionAliasingAlwaysPolicies},
		{"opTableArrayRegionTableMutationPolicies", opTableArrayRegionTableMutationPolicies},
		{"opTableMetatableMutationBarrierPolicies", opTableMetatableMutationBarrierPolicies},
		{"opRuntimeOverflowBoxablePolicies", opRuntimeOverflowBoxablePolicies},
		{"opRuntimeGuardRefreshablePolicies", opRuntimeGuardRefreshablePolicies},
		{"opNativeNumericValueProducerPolicies", opNativeNumericValueProducerPolicies},
		{"opPureNumericUnknownValuePolicies", opPureNumericUnknownValuePolicies},
		{"opTableArraySwapPureBetweenPolicies", opTableArraySwapPureBetweenPolicies},
		{"opStaticTableLenBenignUsePolicies", opStaticTableLenBenignUsePolicies},
		{"opFixedResultTypePolicies", opFixedResultTypePolicies},
		{"opProvesNonNilResultPolicies", opProvesNonNilResultPolicies},
		{"opGuardProvenResultTypePolicies", opGuardProvenResultTypePolicies},
		{"opRawFloatValueProducerPolicies", opRawFloatValueProducerPolicies},
		{"opFieldFactWideKillerPolicies", opFieldFactWideKillerPolicies},
		{"opTableMutationFirstArgPolicies", opTableMutationFirstArgPolicies},
		{"opCallLikeFactBarrierPolicies", opCallLikeFactBarrierPolicies},
		{"opRawCarryClobberPolicies", opRawCarryClobberPolicies},
		{"opExactDivComponentPolicies", opExactDivComponentPolicies},
		{"opIntNarrowCandidatePolicies", opIntNarrowCandidatePolicies},
		{"opIntNarrowAllArgsConstraintPolicies", opIntNarrowAllArgsConstraintPolicies},
		{"opFieldNumFusionGapSafePolicies", opFieldNumFusionGapSafePolicies},
		{"opRawIntSpecializationBlockerPolicies", opRawIntSpecializationBlockerPolicies},
		{"opRawIntSpecializedOpPolicies", opRawIntSpecializedOpPolicies},
		{"opExactIntNarrowOpPolicies", opExactIntNarrowOpPolicies},
		{"opBoxedFallbackOpPolicies", opBoxedFallbackOpPolicies},
		{"opBoxedFallbackResultUnknownPolicies", opBoxedFallbackResultUnknownPolicies},
		{"opSourceFeedbackPolicies", opSourceFeedbackPolicies},
		{"opRangeRefineKindPolicies", opRangeRefineKindPolicies},
	}
	for _, table := range tables {
		if got := reflect.ValueOf(table.table).Len(); got > int(OpMax) {
			t.Fatalf("%s has length %d beyond OpMax %d", table.name, got, OpMax)
		}
	}
}

func TestOpSpecLookupAndTargetIntegrity(t *testing.T) {
	seenNames := make(map[string]Op, int(OpMax))
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if prior, exists := seenNames[spec.Name]; exists {
			t.Fatalf("duplicate OpSpec name %q for %s and %s", spec.Name, prior, op)
		}
		seenNames[spec.Name] = op
		if got, ok := OpByName(spec.Name); !ok || got != op {
			t.Fatalf("OpByName(%q)=(%s,%v), want (%s,true)", spec.Name, got, ok, op)
		}
		assertOpSpecTarget(t, op, "TypeSpecializeIntOp", spec.TypeSpecializeIntOp)
		assertOpSpecTarget(t, op, "TypeSpecializeFloatOp", spec.TypeSpecializeFloatOp)
		assertOpSpecTarget(t, op, "TypeSpecializeStringOp", spec.TypeSpecializeStringOp)
		assertOpSpecTarget(t, op, "RawIntSpecializedOp", spec.RawIntSpecializedOp)
		assertOpSpecTarget(t, op, "ExactIntNarrowOp", spec.ExactIntNarrowOp)
		assertOpSpecTarget(t, op, "BoxedFallbackOp", spec.BoxedFallbackOp)
	}
	if len(seenNames) != int(OpMax) {
		t.Fatalf("OpSpec name lookup saw %d names, want %d", len(seenNames), OpMax)
	}
}

func TestOpSpecUnsetSentinelsDoNotLookLikePolicies(t *testing.T) {
	for _, op := range []Op{OpConstInt, OpConstBool, OpNop, OpReturn} {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if spec.TypeSpecializeIntOp != OpMax || spec.TypeSpecializeFloatOp != OpMax || spec.TypeSpecializeStringOp != OpMax {
			t.Fatalf("%s has unexpected type-specialization defaults: int=%s float=%s string=%s",
				op, spec.TypeSpecializeIntOp, spec.TypeSpecializeFloatOp, spec.TypeSpecializeStringOp)
		}
		if spec.ExactIntNarrowOp != OpMax {
			t.Fatalf("%s ExactIntNarrowOp default=%s, want OpMax", op, spec.ExactIntNarrowOp)
		}
		if spec.BoxedFallbackOp != OpMax {
			t.Fatalf("%s BoxedFallbackOp default=%s, want OpMax", op, spec.BoxedFallbackOp)
		}
		if spec.CallUserArgStart != -1 {
			t.Fatalf("%s CallUserArgStart default=%d, want -1", op, spec.CallUserArgStart)
		}
		if spec.TableArrayKeyArgIndex != -1 {
			t.Fatalf("%s TableArrayKeyArgIndex default=%d, want -1", op, spec.TableArrayKeyArgIndex)
		}
		if _, ok := exactIntNarrowOp(op); ok {
			t.Fatalf("%s should not report an exact int-narrow target", op)
		}
		if _, ok := boxedFallbackOp(op); ok {
			t.Fatalf("%s should not report a boxed fallback target", op)
		}
		if _, ok := rawIntSpecializedOp(op); ok {
			t.Fatalf("%s should not report a raw-int specialization target", op)
		}
		if _, ok := callUserArgStart(op); ok {
			t.Fatalf("%s should not report a call-user arg start", op)
		}
		if _, ok := tableArrayKeyArgIndex(op); ok {
			t.Fatalf("%s should not report a table-array key arg index", op)
		}
	}
}

func assertOpSpecTarget(t *testing.T, owner Op, field string, target Op) {
	t.Helper()
	if target == 0 || target == OpMax {
		return
	}
	if target < 0 || target >= OpMax {
		t.Fatalf("%s.%s targets invalid op %d", owner, field, target)
	}
	if _, ok := target.Spec(); !ok {
		t.Fatalf("%s.%s targets op %d without OpSpec", owner, field, target)
	}
}

func TestOpSpecPolicyTableIntegrityCoversEveryPolicyVar(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	self, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(file), err)
	}
	covered := string(self)
	var missing []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "op_spec_policy_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range valueSpec.Names {
					if strings.HasPrefix(ident.Name, "op") && strings.HasSuffix(ident.Name, "Policies") &&
						!strings.Contains(covered, `"`+ident.Name+`"`) {
						missing = append(missing, ident.Name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan OpSpec policy files: %v", err)
	}
	if len(missing) > 0 {
		t.Fatalf("OpSpec policy integrity test does not cover policy vars: %s", strings.Join(missing, ", "))
	}
}
