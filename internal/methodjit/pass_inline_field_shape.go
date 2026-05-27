// pass_inline_field_shape.go contains field-shape inline eligibility,
// callee preparation, and instruction-safety summaries split out of
// pass_inline.go by pure code movement (no behavior change).

package methodjit

import (
	"fmt"
	"sort"
	"strings"
)

func prepareFieldShapeInlineCallee(calleeFn *Function, c FieldPolyShapeCase) *Function {
	out, _ := prepareFieldShapeInlineCalleeWithReason(calleeFn, c)
	return out
}

func prepareFieldShapeInlineCalleeWithReason(calleeFn *Function, c FieldPolyShapeCase) (*Function, string) {
	if calleeFn == nil || c.ReceiverFact.ShapeID == 0 {
		return calleeFn, "missing receiver shape fact"
	}
	out, err := TypeSpecializePass(calleeFn)
	if err != nil || out == nil {
		return calleeFn, "initial type specialization failed"
	}
	out, err = FixedShapeTableFactsPassWith(FixedShapeTableFactsConfig{
		ArgFacts: map[int]FixedShapeTableFact{0: c.ReceiverFact},
	})(out)
	if err != nil || out == nil {
		return calleeFn, "fixed-shape fact propagation failed"
	}
	out, err = TypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "post-fact type specialization failed"
	}
	out, err = FieldLenFoldPass(out)
	if err != nil || out == nil {
		return calleeFn, "field length folding failed"
	}
	out, err = SourceFeedbackRefreshPass(out)
	if err != nil || out == nil {
		return calleeFn, "source feedback refresh failed"
	}
	out, err = TableArrayLowerPass(out)
	if err != nil || out == nil {
		return calleeFn, "table-array lowering failed"
	}
	out, err = TableArrayLoadTypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "table-array load type specialization failed"
	}
	out, err = SourceFeedbackRefreshPass(out)
	if err != nil || out == nil {
		return calleeFn, "post-load source feedback refresh failed"
	}
	out, err = TypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "post-load type specialization failed"
	}
	out, err = TableArrayStoreLowerPass(out)
	if err != nil || out == nil {
		return calleeFn, "table-array store lowering failed"
	}
	out, err = TypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "post-store type specialization failed"
	}
	out, err = LoadEliminationPass(out)
	if err != nil || out == nil {
		return calleeFn, "post-store load elimination failed"
	}
	out, err = TypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "post-store-load-elim type specialization failed"
	}
	out, err = TableArrayNestedLoadPass(out)
	if err != nil || out == nil {
		return calleeFn, "nested table-array load lowering failed"
	}
	out, err = TypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "post-nested-load type specialization failed"
	}
	out, err = FieldSvalsLowerPass(out)
	if err != nil || out == nil {
		return calleeFn, "field-svals lowering failed"
	}
	out, err = FixedShapeTableFactsPassWith(FixedShapeTableFactsConfig{
		ArgFacts: map[int]FixedShapeTableFact{0: c.ReceiverFact},
	})(out)
	if err != nil || out == nil {
		return calleeFn, "post-field-svals fixed-shape fact propagation failed"
	}
	out, err = TableArrayLowerPass(out)
	if err != nil || out == nil {
		return calleeFn, "post-field-svals table-array lowering failed"
	}
	out, err = TableArrayLoadTypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "post-field-svals table-array load type specialization failed"
	}
	out, err = LoadEliminationPass(out)
	if err != nil || out == nil {
		return calleeFn, "post-field-svals load elimination failed"
	}
	out, err = TypeSpecializePass(out)
	if err != nil || out == nil {
		return calleeFn, "post-field-svals type specialization failed"
	}
	out, err = ConstPropPass(out)
	if err != nil || out == nil {
		return calleeFn, "const propagation failed"
	}
	out, err = DCEPass(out)
	if err != nil || out == nil {
		return calleeFn, "dce failed"
	}
	return out, ""
}

func fieldShapeInlineSplitEligibilitySummary(fn *Function, instr *Instr, config InlineConfig, block *Block) string {
	cases := fieldShapeCalleeCases(fn, instr)
	if len(cases) < 2 {
		return ""
	}
	callArgs, ok := inlineCallArgumentValues(instr)
	if !ok {
		return "blocked: unsupported call argument layout"
	}
	loopBlock := false
	if fn != nil && block != nil {
		loopBlock = computeLoopInfo(fn).loopBlocks[block.ID]
	}
	eligible := 0
	reasons := make(map[string]int)
	for _, c := range cases {
		reason := fieldShapeInlineSplitCaseRejectReason(c, callArgs, config, loopBlock)
		if reason == "" {
			eligible++
			continue
		}
		if detail := fieldShapeReceiverFactSummary(c.ReceiverFact); detail != "" {
			reason = reason + "/" + detail
		}
		reasons[reason]++
	}
	parts := []string{fmt.Sprintf("eligible=%d/%d", eligible, len(cases))}
	keys := make([]string, 0, len(reasons))
	for reason := range reasons {
		keys = append(keys, reason)
	}
	sort.Strings(keys)
	for _, reason := range keys {
		n := reasons[reason]
		parts = append(parts, fmt.Sprintf("%s=%d", reason, n))
	}
	if effectSummary := fieldShapeInlineEffectSummary(cases); effectSummary != "" {
		parts = append(parts, effectSummary)
	}
	return strings.Join(parts, ", ")
}

func fieldShapeInlineEffectSummary(cases []FieldPolyShapeCase) string {
	if len(cases) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cases))
	for _, c := range cases {
		if c.VMProto == nil {
			continue
		}
		effects := SummarizeFieldEffects(c.VMProto)
		parts = append(parts, fmt.Sprintf("%s=%s", fieldShapeCaseProtoName(c), effects.FormatParam(0)))
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	return "effects{" + strings.Join(parts, ";") + "}"
}

func fieldShapeReceiverFactSummary(fact FixedShapeTableFact) string {
	if len(fact.FieldTableFacts) == 0 {
		return ""
	}
	names := make([]string, 0, len(fact.FieldTableFacts))
	for name, nested := range fact.FieldTableFacts {
		part := name
		if nested.ArrayElementType != TypeUnknown && nested.ArrayElementType != TypeAny {
			part += "[]=" + nested.ArrayElementType.String()
		}
		if nested.ShapeID != 0 {
			part += fmt.Sprintf("#%d", nested.ShapeID)
		}
		names = append(names, part)
	}
	sort.Strings(names)
	return "receiver-nested{" + strings.Join(names, ",") + "}"
}

func fieldShapeInlineSplitCaseRejectReason(c FieldPolyShapeCase, callArgs []*Value, config InlineConfig, callerLoopBlock bool) string {
	if c.ShapeID == 0 {
		return "missing-shape"
	}
	if c.FieldIdx < 0 {
		return "missing-field-index"
	}
	if c.VMProto == nil {
		return "missing-proto"
	}
	if c.ReceiverFact.ShapeID == 0 {
		return "missing-receiver-fact"
	}
	if len(callArgs) != c.VMProto.NumParams {
		return "arg-count"
	}
	if len(c.VMProto.Code) > config.MaxSize {
		return "size"
	}
	if inlineCalleeHasRuntimeSpecializationEntry(c.VMProto, config.Globals) {
		return "runtime-specialization-entry"
	}
	calleeFn := BuildGraph(c.VMProto)
	if calleeFn == nil || calleeFn.Unpromotable {
		return "unpromotable"
	}
	calleeFn = applyInlineArgTypeFacts(calleeFn, callArgs)
	var prepReason string
	calleeFn, prepReason = prepareFieldShapeInlineCalleeWithReason(calleeFn, c)
	if prepReason != "" {
		return "callee-prep:" + prepReason
	}
	if callerLoopBlock && computeLoopInfo(calleeFn).hasLoops() {
		if reason := pureNumericInlineRejectReason(calleeFn); reason != "" {
			return "callee-loop"
		}
		if callABICalleeHasShiftAddOverflowVersion(c.VMProto, nil) {
			return "overflow-versioned-loop"
		}
	}
	if callerLoopBlock {
		if reason := fieldShapeLoopPreInlineUnsafeReason(calleeFn); reason != "" {
			return reason
		}
	}
	if config.RequirePureNumeric {
		if reason := pureNumericInlineRejectReason(calleeFn); reason != "" {
			_ = reason
			return "not-pure-numeric"
		}
	}
	return ""
}

func fieldShapeLoopPreInlineUnsafeReason(calleeFn *Function) string {
	if calleeFn == nil {
		return "callee-graph"
	}
	seenSideEffect := false
	for _, block := range calleeFn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			if instr.Op == OpReturn || instr.Op == OpLoadSlot || instr.Op == OpStoreSlot ||
				instr.Op == OpConstInt || instr.Op == OpConstFloat || instr.Op == OpConstBool ||
				instr.Op == OpConstNil || instr.Op == OpConstString {
				continue
			}
			if !seenSideEffect && fieldShapePreEffectInlineInstrSafe(instr) {
				// Reads before the first callee side effect may exit safely:
				// replaying the original call has not duplicated any callee
				// mutation yet. Post-effect code stays stricter below.
			} else if !fieldShapeSplitInlineInstrSafe(instr) {
				return "loop-inline-exit-unsafe:" + fieldShapeInlineInstrSummary(instr)
			}
			if seenSideEffect && !fieldShapePostEffectInlineInstrSafe(instr) {
				return "loop-inline-post-effect-unsafe:" + fieldShapeInlineInstrSummary(instr)
			}
			if fieldShapeInlineInstrHasSideEffect(instr) {
				seenSideEffect = true
			}
		}
	}
	return ""
}

func fieldShapeInlineInstrSummary(instr *Instr) string {
	if instr == nil {
		return "<nil>"
	}
	parts := make([]string, 0, len(instr.Args)+1)
	parts = append(parts, instr.Op.String())
	if instr.Type != TypeUnknown && instr.Type != TypeAny {
		parts = append(parts, "type="+instr.Type.String())
	}
	for i, arg := range instr.Args {
		if arg == nil || arg.Def == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("arg%d=%s", i, arg.Def.Type.String()))
	}
	return strings.Join(parts, "/")
}

func fieldShapePreEffectInlineInstrSafe(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	if ok && spec.FieldShapePreEffectInlineSafe {
		return true
	}
	return fieldShapeSplitInlineInstrSafe(instr)
}

func fieldShapeInlineInstrHasSideEffect(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldShapeInlineSideEffect
}

func fieldShapePostEffectInlineInstrSafe(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	if ok && spec.FieldShapePostEffectInlineUnsafe {
		return false
	}
	return fieldShapeSplitInlineInstrSafe(instr)
}
