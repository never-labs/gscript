// pass_fixed_shape_table_args.go: interprocedural and profiled fixed-shape
// argument fact inference — guarded callsite arg facts per callee proto,
// conflict tracking, and extraction of fixed-shape facts from profiling
// feedback vectors (arg, array-element, polymorphic, nested, field metadata).
// Pure code movement from pass_fixed_shape_table.go; no behavior change.

package methodjit

import (
	"github.com/gscript/gscript/internal/vm"
)

func inferGuardedFixedShapeArgFactsForProto(target *vm.FuncProto, globals map[string]*vm.FuncProto) map[int]FixedShapeTableFact {
	facts, _ := inferGuardedFixedShapeArgFactsAndConflictsForProto(target, globals)
	return facts
}

func guardedFixedShapeArgConflictParamsForProto(target *vm.FuncProto, globals map[string]*vm.FuncProto) map[int]bool {
	_, conflicts := inferGuardedFixedShapeArgFactsAndConflictsForProto(target, globals)
	return conflicts
}

func inferGuardedFixedShapeArgFactsAndConflictsForProto(target *vm.FuncProto, globals map[string]*vm.FuncProto) (map[int]FixedShapeTableFact, map[int]bool) {
	if target == nil || len(globals) == 0 {
		return nil, nil
	}
	type argFactState struct {
		fact     FixedShapeTableFact
		seen     bool
		conflict bool
	}
	states := make(map[int]argFactState)
	seenCallsite := false
	for _, caller := range uniqueFuncProtos(globals) {
		if caller == nil {
			continue
		}
		fn := BuildGraph(caller)
		if fn == nil || fn.Unpromotable {
			continue
		}
		facts := inferFixedShapeValuesForArgs(fn, globals)
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if instr.Op != OpCall {
					continue
				}
				_, callee := resolveCallee(instr, fn, InlineConfig{Globals: globals})
				if callee != target {
					continue
				}
				seenCallsite = true
				for i := 1; i < len(instr.Args) && i <= target.NumParams; i++ {
					arg := instr.Args[i]
					if arg == nil {
						continue
					}
					fact, ok := facts[arg.ID]
					if !ok {
						continue
					}
					guarded, ok := guardedFixedShapeArgFact(fact)
					if !ok {
						continue
					}
					paramIdx := i - 1
					state := states[paramIdx]
					if !state.seen {
						state.fact = guarded
						state.seen = true
						states[paramIdx] = state
						continue
					}
					if !state.fact.sameShape(guarded) || state.fact.ShapeID != guarded.ShapeID {
						state.conflict = true
						states[paramIdx] = state
					}
				}
			}
		}
	}
	if !seenCallsite || len(states) == 0 {
		return nil, nil
	}
	out := make(map[int]FixedShapeTableFact, len(states))
	conflicts := make(map[int]bool)
	for idx, state := range states {
		if state.seen && !state.conflict {
			out[idx] = state.fact
		} else if state.conflict {
			conflicts[idx] = true
		}
	}
	if len(out) == 0 {
		out = nil
	}
	if len(conflicts) == 0 {
		conflicts = nil
	}
	return out, conflicts
}

func inferGuardedFixedShapeArrayElementArgFactsForProto(target *vm.FuncProto, globals map[string]*vm.FuncProto) map[int]FixedShapeTableFact {
	facts, _ := inferGuardedFixedShapeArrayElementArgFactsAndConflictsForProto(target, globals)
	return facts
}

func guardedFixedShapeArrayElementArgConflictParamsForProto(target *vm.FuncProto, globals map[string]*vm.FuncProto) map[int]bool {
	_, conflicts := inferGuardedFixedShapeArrayElementArgFactsAndConflictsForProto(target, globals)
	return conflicts
}

func inferGuardedFixedShapeArrayElementArgFactsAndConflictsForProto(target *vm.FuncProto, globals map[string]*vm.FuncProto) (map[int]FixedShapeTableFact, map[int]bool) {
	if target == nil || len(globals) == 0 {
		return nil, nil
	}
	type argFactState struct {
		fact     FixedShapeTableFact
		seen     bool
		conflict bool
	}
	states := make(map[int]argFactState)
	seenCallsite := false
	for _, caller := range uniqueFuncProtos(globals) {
		if caller == nil {
			continue
		}
		fn := BuildGraph(caller)
		if fn == nil || fn.Unpromotable {
			continue
		}
		arrayFacts := inferArrayElementValuesForArgs(fn, globals)
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if instr.Op != OpCall {
					continue
				}
				_, callee := resolveCallee(instr, fn, InlineConfig{Globals: globals})
				if callee != target {
					continue
				}
				seenCallsite = true
				for i := 1; i < len(instr.Args) && i <= target.NumParams; i++ {
					arg := instr.Args[i]
					if arg == nil {
						continue
					}
					fact, ok := arrayFacts[arg.ID]
					if !ok {
						continue
					}
					guarded, ok := guardedFixedShapeArgFact(fact)
					if !ok {
						continue
					}
					paramIdx := i - 1
					state := states[paramIdx]
					if !state.seen {
						state.fact = guarded
						state.seen = true
						states[paramIdx] = state
						continue
					}
					if !state.fact.sameShape(guarded) || state.fact.ShapeID != guarded.ShapeID {
						state.conflict = true
						states[paramIdx] = state
					}
				}
			}
		}
	}
	if !seenCallsite || len(states) == 0 {
		return nil, nil
	}
	out := make(map[int]FixedShapeTableFact, len(states))
	conflicts := make(map[int]bool)
	for idx, state := range states {
		if state.seen && !state.conflict {
			out[idx] = state.fact
		} else if state.conflict {
			conflicts[idx] = true
		}
	}
	if len(out) == 0 {
		out = nil
	}
	if len(conflicts) == 0 {
		conflicts = nil
	}
	return out, conflicts
}

func profiledFixedShapeArrayElementArgFactsForProto(target *vm.FuncProto) map[int]FixedShapeTableFact {
	return profiledFixedShapeFactsFromFeedback(target, targetArgArrayElementShapeFeedback(target))
}

func profiledFixedShapeArgFactsForProto(target *vm.FuncProto) map[int]FixedShapeTableFact {
	return profiledFixedShapeFactsFromFeedback(target, targetArgShapeFeedback(target))
}

func profiledFixedShapeArgPolyFactsForProto(target *vm.FuncProto) map[int][]FixedShapeTableFact {
	return profiledFixedShapePolyFactsFromFeedback(target, targetArgShapeFeedback(target))
}

func targetArgArrayElementShapeFeedback(target *vm.FuncProto) vm.ArgArrayElementShapeFeedbackVector {
	if target == nil {
		return nil
	}
	return target.ArgArrayElementShapeFeedback
}

func targetArgShapeFeedback(target *vm.FuncProto) vm.ArgArrayElementShapeFeedbackVector {
	if target == nil {
		return nil
	}
	return target.ArgShapeFeedback
}

func profiledFixedShapeFactsFromFeedback(target *vm.FuncProto, feedbacks vm.ArgArrayElementShapeFeedbackVector) map[int]FixedShapeTableFact {
	if target == nil || len(feedbacks) == 0 {
		return nil
	}
	out := make(map[int]FixedShapeTableFact)
	for idx, feedback := range feedbacks {
		if idx < 0 || idx >= target.NumParams {
			continue
		}
		shapeID, fields, ok := feedback.StableShape()
		if !ok {
			continue
		}
		out[idx] = FixedShapeTableFact{
			ShapeID:           shapeID,
			FieldNames:        append([]string(nil), fields...),
			FieldTypes:        profiledFixedShapeFieldTypes(feedback),
			FieldRanges:       profiledFixedShapeFieldRanges(feedback),
			FieldLenRanges:    profiledFixedShapeFieldLenRanges(feedback),
			FieldTableFacts:   profiledNestedFixedShapeTableFacts(feedback),
			StringValueFact:   profiledStringValueFixedShapeTableFact(feedback),
			ArrayElementType:  profiledArrayElementType(feedback),
			ArrayElementRange: profiledArrayElementRange(feedback),
			Guarded:           true,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledFixedShapeArrayElementPolyFactsForProto(target *vm.FuncProto) map[int][]FixedShapeTableFact {
	return profiledFixedShapePolyFactsFromFeedback(target, targetArgArrayElementShapeFeedback(target))
}

func profiledFixedShapePolyFactsFromFeedback(target *vm.FuncProto, feedbacks vm.ArgArrayElementShapeFeedbackVector) map[int][]FixedShapeTableFact {
	if target == nil || len(feedbacks) == 0 {
		return nil
	}
	out := make(map[int][]FixedShapeTableFact)
	for idx, feedback := range feedbacks {
		if idx < 0 || idx >= target.NumParams {
			continue
		}
		shapes := feedback.PolymorphicShapes()
		if len(shapes) < 2 {
			continue
		}
		facts := make([]FixedShapeTableFact, 0, len(shapes))
		for _, shape := range shapes {
			facts = append(facts, FixedShapeTableFact{
				ShapeID:          shape.ShapeID,
				ObservationCount: shape.Count,
				FieldNames:       append([]string(nil), shape.FieldNames...),
				FieldTypes:       profiledShapeCaseFieldTypes(shape),
				FieldRanges:      profiledShapeCaseFieldRanges(shape),
				FieldLenRanges:   profiledShapeCaseFieldLenRanges(shape),
				FieldVMProtos:    profiledShapeCaseFieldVMProtos(shape),
				FieldVMClosures:  profiledShapeCaseFieldVMClosures(shape),
				FieldTableFacts:  profiledNestedFixedShapeTableFacts(feedback),
				StringValueFact:  profiledStringValueFixedShapeTableFact(feedback),
				Guarded:          true,
			})
		}
		if len(facts) >= 2 {
			out[idx] = facts
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeFixedShapeTableFacts(preferred, fallback map[int]FixedShapeTableFact) map[int]FixedShapeTableFact {
	if len(preferred) == 0 {
		return fallback
	}
	if len(fallback) == 0 {
		return preferred
	}
	out := make(map[int]FixedShapeTableFact, len(preferred)+len(fallback))
	for idx, fact := range fallback {
		out[idx] = fact
	}
	for idx, fact := range preferred {
		out[idx] = fact
	}
	return out
}

func cloneFixedShapeTableFactIntMap(in map[int]FixedShapeTableFact) map[int]FixedShapeTableFact {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]FixedShapeTableFact, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func profiledNestedFixedShapeTableFacts(feedback vm.ArgArrayElementShapeFeedback) map[string]FixedShapeTableFact {
	if len(feedback.Nested) == 0 {
		return nil
	}
	out := make(map[string]FixedShapeTableFact)
	for name, nested := range feedback.Nested {
		shapeID, fields, ok := nested.StableShape()
		arrayType := profiledArrayElementType(nested)
		arrayRange := profiledArrayElementRange(nested)
		stringValueFact := profiledStringValueFixedShapeTableFact(nested)
		if !ok && arrayType == TypeUnknown && !arrayRange.known && stringValueFact == nil {
			continue
		}
		out[name] = FixedShapeTableFact{
			ShapeID:           shapeID,
			FieldNames:        append([]string(nil), fields...),
			FieldTypes:        profiledFixedShapeFieldTypes(nested),
			FieldRanges:       profiledFixedShapeFieldRanges(nested),
			FieldLenRanges:    profiledFixedShapeFieldLenRanges(nested),
			FieldTableFacts:   profiledNestedFixedShapeTableFacts(nested),
			StringValueFact:   stringValueFact,
			ArrayElementType:  profiledArrayElementType(nested),
			ArrayElementRange: profiledArrayElementRange(nested),
			Guarded:           true,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledStringValueFixedShapeTableFact(feedback vm.ArgArrayElementShapeFeedback) *FixedShapeTableFact {
	if feedback.StringValueShape == nil {
		return nil
	}
	fact, ok := fixedShapeFactFromProfiledValueShape(*feedback.StringValueShape)
	if !ok {
		return nil
	}
	return cloneFixedShapeTableFactPtr(fact)
}

func profiledFixedShapeFieldTypes(feedback vm.ArgArrayElementShapeFeedback) map[string]Type {
	if len(feedback.FieldTypes) == 0 {
		return nil
	}
	out := make(map[string]Type)
	for name, fbType := range feedback.FieldTypes {
		typ, ok := feedbackToIRType(fbType)
		if !ok {
			continue
		}
		out[name] = typ
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledFixedShapeFieldRanges(feedback vm.ArgArrayElementShapeFeedback) map[string]intRange {
	if len(feedback.FieldRanges) == 0 {
		return nil
	}
	out := make(map[string]intRange)
	for name, rangeFeedback := range feedback.FieldRanges {
		min, max, ok := rangeFeedback.StableRange()
		if !ok {
			continue
		}
		out[name] = intRange{min: min, max: max, known: true}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledFixedShapeFieldLenRanges(feedback vm.ArgArrayElementShapeFeedback) map[string]intRange {
	if len(feedback.FieldLenRanges) == 0 {
		return nil
	}
	out := make(map[string]intRange)
	for name, rangeFeedback := range feedback.FieldLenRanges {
		min, max, ok := rangeFeedback.StableRange()
		if !ok {
			continue
		}
		out[name] = intRange{min: min, max: max, known: true}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledShapeCaseFieldTypes(shape vm.ArgArrayElementShapeCase) map[string]Type {
	if len(shape.FieldTypes) == 0 {
		return nil
	}
	out := make(map[string]Type)
	for name, fbType := range shape.FieldTypes {
		typ, ok := feedbackToIRType(fbType)
		if !ok {
			continue
		}
		out[name] = typ
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledShapeCaseFieldRanges(shape vm.ArgArrayElementShapeCase) map[string]intRange {
	if len(shape.FieldRanges) == 0 {
		return nil
	}
	out := make(map[string]intRange)
	for name, rangeFeedback := range shape.FieldRanges {
		min, max, ok := rangeFeedback.StableRange()
		if !ok {
			continue
		}
		out[name] = intRange{min: min, max: max, known: true}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledShapeCaseFieldLenRanges(shape vm.ArgArrayElementShapeCase) map[string]intRange {
	if len(shape.FieldLenRanges) == 0 {
		return nil
	}
	out := make(map[string]intRange)
	for name, rangeFeedback := range shape.FieldLenRanges {
		min, max, ok := rangeFeedback.StableRange()
		if !ok {
			continue
		}
		out[name] = intRange{min: min, max: max, known: true}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledShapeCaseFieldVMProtos(shape vm.ArgArrayElementShapeCase) map[string]*vm.FuncProto {
	if len(shape.FieldVMProtos) == 0 {
		return nil
	}
	out := make(map[string]*vm.FuncProto)
	for name, proto := range shape.FieldVMProtos {
		if proto != nil {
			out[name] = proto
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledShapeCaseFieldVMClosures(shape vm.ArgArrayElementShapeCase) map[string]uintptr {
	if len(shape.FieldVMClosures) == 0 {
		return nil
	}
	out := make(map[string]uintptr)
	for name, closure := range shape.FieldVMClosures {
		if closure != 0 && closure != ^uintptr(0) {
			out[name] = closure
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func profiledArrayElementType(feedback vm.ArgArrayElementShapeFeedback) Type {
	typ, ok := feedbackToIRType(feedback.ArrayElementType)
	if !ok {
		return TypeUnknown
	}
	return typ
}

func profiledArrayElementRange(feedback vm.ArgArrayElementShapeFeedback) intRange {
	min, max, ok := feedback.ArrayElementRange.StableRange()
	if !ok {
		return intRange{}
	}
	return intRange{min: min, max: max, known: true}
}

func inferFixedShapeValuesForArgs(fn *Function, globals map[string]*vm.FuncProto) map[int]FixedShapeTableFact {
	facts := inferLocalFixedShapeTables(fn)
	if len(facts) == 0 {
		facts = make(map[int]FixedShapeTableFact)
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpCall {
				continue
			}
			_, callee := resolveCallee(instr, fn, InlineConfig{Globals: globals})
			if callee == nil {
				continue
			}
			fact, ok := AnalyzeFixedShapeReturnFact(callee)
			if !ok {
				continue
			}
			facts[instr.ID] = fact
		}
	}
	seedLocalStringMapValueFacts(fn, facts)
	seedLocalFieldTableFacts(fn, facts)
	if len(facts) == 0 {
		return nil
	}
	return facts
}

func inferArrayElementValuesForArgs(fn *Function, globals map[string]*vm.FuncProto) map[int]FixedShapeTableFact {
	out := make(map[int]FixedShapeTableFact)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpCall {
				continue
			}
			_, callee := resolveCallee(instr, fn, InlineConfig{Globals: globals})
			if callee == nil {
				continue
			}
			fact, ok := AnalyzeFixedShapeArrayElementReturnFact(callee, globals)
			if !ok {
				continue
			}
			out[instr.ID] = fact
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func uniqueFuncProtos(globals map[string]*vm.FuncProto) []*vm.FuncProto {
	seen := make(map[*vm.FuncProto]bool, len(globals))
	out := make([]*vm.FuncProto, 0, len(globals))
	for _, proto := range globals {
		if proto == nil || seen[proto] {
			continue
		}
		seen[proto] = true
		out = append(out, proto)
	}
	return out
}
