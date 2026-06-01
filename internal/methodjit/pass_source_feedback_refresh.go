package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/vm"
)

// SourceFeedbackRefreshPass reapplies bytecode feedback from an instruction's
// original SourceProto after inlining. GraphBuilder consumes feedback while
// building the callee graph, but some profile facts can mature after the callee
// was first compiled or be lost when a caller-owned pipeline rewrites inlined
// instructions. SourceProto lets this pass recover those generic facts without
// coupling to a specific workload.
func SourceFeedbackRefreshPass(fn *Function) (*Function, error) {
	return SourceFeedbackRefreshPassCtx(newPassContext(fn, nil, sourceFeedbackRefreshAllowedDomains, false))
}

var sourceFeedbackRefreshAllowedDomains = allowedDomainsForModule(nil, nil, analysisFacts(AnalysisFactFieldPolyShapeFacts))

func SourceFeedbackRefreshPassCtx(ctx *PassContext) (*Function, error) {
	if ctx == nil {
		return nil, nil
	}
	fn := ctx.Func()
	if fn == nil {
		return fn, nil
	}
	fn.ensureAnalysis()
	tableShapes := ctx.TableShape()
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || !instr.HasSource || instr.SourceProto == nil || instr.SourcePC < 0 {
				continue
			}
			switch sourceFeedbackPolicy(instr.Op) {
			case OpSourceFeedbackGetField:
				sourceFeedbackRefreshGetField(fn, tableShapes, block, instr)
			case OpSourceFeedbackSetField:
				sourceFeedbackRefreshSetField(fn, block, instr)
			case OpSourceFeedbackGetTable:
				sourceFeedbackRefreshGetTable(fn, block, instr)
			case OpSourceFeedbackSetTable:
				sourceFeedbackRefreshSetTable(fn, block, instr)
			case OpSourceFeedbackResultType:
				sourceFeedbackRefreshResultType(fn, block, instr)
			}
		}
	}
	return fn, nil
}

func sourceFeedbackRefreshGetField(fn *Function, tableShapes *TableShapeFacts, block *Block, instr *Instr) {
	if tableShapes != nil && tableShapes.HasFieldPolyShapeCases(instr.ID) {
		if typ, ok := sourceFeedbackFieldValueType(instr.SourceProto, instr.SourcePC); ok &&
			(instr.Type == TypeAny || instr.Type == TypeUnknown) {
			instr.Type = typ
			functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
				"restored source field type "+typ.String())
		}
		return
	}
	if cases, typ := runtimeFieldPolyShapeCasesFromFeedback(fn, instr); len(cases) >= 2 {
		if tableShapes != nil {
			tableShapes.RecordFieldPolyShapeCases(instr.ID, cases)
		}
		instr.Aux2 = 0
		if typ != TypeUnknown && typ != TypeAny && (instr.Type == TypeAny || instr.Type == TypeUnknown) {
			instr.Type = typ
		}
		functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
			fmt.Sprintf("restored source polymorphic field cache with %d shapes", len(cases)))
		return
	}
	if aux2 := sourceFeedbackFieldShapeAux2(instr.SourceProto, instr.SourcePC); aux2 != 0 && instr.Aux2 == 0 {
		instr.Aux2 = aux2
		functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
			"restored source field shape")
	}
	if typ, ok := sourceFeedbackFieldValueType(instr.SourceProto, instr.SourcePC); ok &&
		(instr.Type == TypeAny || instr.Type == TypeUnknown) {
		instr.Type = typ
		functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
			"restored source field type "+typ.String())
	}
}

func sourceFeedbackRefreshSetField(fn *Function, block *Block, instr *Instr) {
	aux2 := sourceFeedbackFieldShapeAux2(instr.SourceProto, instr.SourcePC)
	if aux2 == 0 || instr.Aux2 != 0 {
		return
	}
	instr.Aux2 = aux2
	functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
		"restored source field shape")
}

func sourceFeedbackRefreshGetTable(fn *Function, block *Block, instr *Instr) {
	kind := sourceFeedbackTableKind(instr.SourceProto, instr.SourcePC)
	if kind != 0 && instr.Aux2 == 0 {
		instr.Aux2 = kind
		functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
			fmt.Sprintf("restored source table kind %d", kind))
	}
	if typ, ok := sourceFeedbackResultType(instr.SourceProto, instr.SourcePC); ok &&
		(instr.Type == TypeAny || instr.Type == TypeUnknown) {
		instr.Type = typ
		functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
			"restored source result type "+typ.String())
	}
}

func sourceFeedbackRefreshSetTable(fn *Function, block *Block, instr *Instr) {
	kind := sourceFeedbackTableKind(instr.SourceProto, instr.SourcePC)
	if kind == 0 || instr.Aux2 != 0 {
		return
	}
	instr.Aux2 = kind
	functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
		fmt.Sprintf("restored source table store kind %d", kind))
}

func sourceFeedbackRefreshResultType(fn *Function, block *Block, instr *Instr) {
	if typ, ok := sourceFeedbackResultType(instr.SourceProto, instr.SourcePC); ok &&
		(instr.Type == TypeAny || instr.Type == TypeUnknown) {
		instr.Type = typ
		functionRemarks(fn).Add("SourceFeedbackRefresh", "changed", block.ID, instr.ID, instr.Op,
			"restored source result type "+typ.String())
	}
}

func sourceFeedbackTableKind(proto *vm.FuncProto, pc int) int64 {
	if proto == nil || pc < 0 {
		return 0
	}
	if pc < len(proto.Feedback) {
		fb := proto.Feedback[pc]
		if fb.Kind != vm.FBKindUnobserved && fb.Kind != vm.FBKindPolymorphic {
			return int64(fb.Kind)
		}
	}
	if pc < len(proto.TableKeyFeedback) {
		switch proto.TableKeyFeedback[pc].ArrayKind {
		case vm.FBKindMixed, vm.FBKindInt, vm.FBKindFloat, vm.FBKindBool:
			return int64(proto.TableKeyFeedback[pc].ArrayKind)
		}
	}
	return 0
}

func sourceFeedbackResultType(proto *vm.FuncProto, pc int) (Type, bool) {
	if proto == nil || pc < 0 {
		return TypeUnknown, false
	}
	if pc < len(proto.Feedback) {
		if typ, ok := feedbackToIRType(proto.Feedback[pc].Result); ok {
			return typ, true
		}
	}
	if pc < len(proto.TableKeyFeedback) {
		if typ, ok := feedbackToIRType(proto.TableKeyFeedback[pc].ValueType); ok {
			return typ, true
		}
	}
	return TypeUnknown, false
}

func sourceFeedbackFieldShapeAux2(proto *vm.FuncProto, pc int) int64 {
	if proto == nil || pc < 0 || proto.FieldAccessFeedback == nil || pc >= len(proto.FieldAccessFeedback) {
		return 0
	}
	feedback := proto.FieldAccessFeedback[pc]
	if feedback.Count == 0 {
		return 0
	}
	shapeID, fieldIdx, ok := feedback.StableShapeField()
	if !ok {
		return 0
	}
	return int64(shapeID)<<32 | int64(uint32(fieldIdx))
}

func sourceFeedbackFieldValueType(proto *vm.FuncProto, pc int) (Type, bool) {
	if proto == nil || pc < 0 || proto.FieldAccessFeedback == nil || pc >= len(proto.FieldAccessFeedback) {
		return TypeUnknown, false
	}
	return feedbackToIRType(proto.FieldAccessFeedback[pc].ValueType)
}
