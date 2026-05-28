// pass_fixed_shape_table_seed.go: seeding and propagation of fixed-shape table
// facts onto SSA values — guarded callsite/array-element/polymorphic arg facts,
// profiled dynamic table value facts, phi propagation, same-shape merges, and
// local string-map / field / array-element value fact seeding.
// Pure code movement from pass_fixed_shape_table.go; no behavior change.

package methodjit

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/gscript/gscript/internal/vm"
)

func seedGuardedFixedShapeArgFacts(fn *Function, tableShapes *TableShapeFacts, facts map[int]FixedShapeTableFact, argFacts map[int]FixedShapeTableFact) {
	if fn == nil || fn.Proto == nil || tableShapes == nil || len(argFacts) == 0 {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpLoadSlot || instr.Aux < 0 || int(instr.Aux) >= fn.Proto.NumParams {
				continue
			}
			fact, ok := guardedFixedShapeArgFact(argFacts[int(instr.Aux)])
			if !ok {
				continue
			}
			facts[instr.ID] = fact
			tableShapes.RecordFixedShapeArgFact(int(instr.Aux), fact)
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("parameter %d carries guarded fixed table shape %v", instr.Aux, fact.FieldNames))
		}
	}
}

func seedGuardedFixedShapeArrayElementArgFacts(fn *Function, facts map[int]FixedShapeTableFact, argFacts map[int]FixedShapeTableFact) {
	if fn == nil || fn.Proto == nil || len(argFacts) == 0 {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			var tableDef *Instr
			switch instr.Op {
			case OpGetTable:
				if len(instr.Args) < 2 || instr.Args[0] == nil {
					continue
				}
				tableDef = instr.Args[0].Def
			case OpTableArrayLoad:
				if tv, ok := loweredTableArrayLoadTableValue(instr); ok {
					tableDef = tv.Def
				}
			default:
				continue
			}
			if tableDef == nil || tableDef.Op != OpLoadSlot || tableDef.Aux < 0 || int(tableDef.Aux) >= fn.Proto.NumParams {
				continue
			}
			fact, ok := guardedFixedShapeArgFact(argFacts[int(tableDef.Aux)])
			if !ok {
				continue
			}
			facts[instr.ID] = fact
			if instr.Type == TypeAny || instr.Type == TypeUnknown {
				instr.Type = TypeTable
			}
			if instr.Op == OpGetTable && tableKeyProvenInt(instr.Args[1]) && instr.Aux2 == 0 {
				instr.Aux2 = int64(vm.FBKindMixed)
			}
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("parameter %d array element carries guarded fixed table shape %v", tableDef.Aux, fact.FieldNames))
		}
	}
}

func seedGuardedPolyShapeArgFacts(fn *Function, tableShapes *TableShapeFacts, argFacts map[int][]FixedShapeTableFact) {
	if fn == nil || fn.Proto == nil || tableShapes == nil || len(argFacts) == 0 {
		return
	}
	valueFacts := make(map[int][]FixedShapeTableFact)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch instr.Op {
			case OpLoadSlot:
				if instr.Aux < 0 || int(instr.Aux) >= fn.Proto.NumParams {
					continue
				}
				poly := guardedFixedShapePolyFacts(argFacts[int(instr.Aux)])
				if len(poly) == 0 {
					continue
				}
				valueFacts[instr.ID] = poly
				tableShapes.RecordFieldPolyShapeReceiver(instr.ID)
				if instr.Type == TypeAny || instr.Type == TypeUnknown {
					instr.Type = TypeTable
				}
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("parameter %d carries %d guarded polymorphic shapes", instr.Aux, len(poly)))
			default:
				if !opIsFieldRead(instr.Op) {
					continue
				}
				if len(instr.Args) == 0 || instr.Args[0] == nil {
					continue
				}
				poly := valueFacts[instr.Args[0].ID]
				if len(poly) == 0 {
					continue
				}
				name := fieldNameFromAux(fn, instr.Aux)
				if name == "" {
					continue
				}
				cases, typ := fieldPolyShapeCases(poly, name)
				if len(cases) < 2 {
					continue
				}
				tableShapes.RecordFieldPolyShapeCases(instr.ID, cases)
				instr.Aux2 = 0
				if typ != TypeUnknown && typ != TypeAny {
					instr.Type = typ
				}
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("prefilled parameter polymorphic field cache for %q with %d shapes", name, len(cases)))
			}
		}
	}
}

func seedGuardedPolyShapeArrayElementArgFacts(fn *Function, tableShapes *TableShapeFacts, facts map[int]FixedShapeTableFact, argFacts map[int][]FixedShapeTableFact) {
	if fn == nil || fn.Proto == nil || tableShapes == nil || len(argFacts) == 0 {
		return
	}
	valueFacts := make(map[int][]FixedShapeTableFact)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			var tableDef *Instr
			switch instr.Op {
			case OpGetTable:
				if len(instr.Args) < 2 || instr.Args[0] == nil {
					continue
				}
				tableDef = instr.Args[0].Def
			case OpTableArrayLoad:
				if tv, ok := loweredTableArrayLoadTableValue(instr); ok {
					tableDef = tv.Def
				}
			default:
				tableDef = nil
			}
			if tableDef != nil && tableDef.Op == OpLoadSlot && tableDef.Aux >= 0 && int(tableDef.Aux) < fn.Proto.NumParams {
				poly := guardedFixedShapePolyFacts(argFacts[int(tableDef.Aux)])
				if len(poly) > 0 {
					valueFacts[instr.ID] = poly
					tableShapes.RecordFieldPolyShapeReceiver(instr.ID)
					if instr.Type == TypeAny || instr.Type == TypeUnknown {
						instr.Type = TypeTable
					}
					if instr.Op == OpGetTable && tableKeyProvenInt(instr.Args[1]) && instr.Aux2 == 0 {
						instr.Aux2 = int64(vm.FBKindMixed)
					}
					functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
						fmt.Sprintf("parameter %d array element carries %d guarded polymorphic shapes", tableDef.Aux, len(poly)))
				}
			}
			if opIsFieldRead(instr.Op) {
				if len(instr.Args) == 0 || instr.Args[0] == nil {
					continue
				}
				poly := valueFacts[instr.Args[0].ID]
				if len(poly) == 0 {
					continue
				}
				name := fieldNameFromAux(fn, instr.Aux)
				if name == "" {
					continue
				}
				cases, typ := fieldPolyShapeCases(poly, name)
				if len(cases) < 2 {
					continue
				}
				tableShapes.RecordFieldPolyShapeCases(instr.ID, cases)
				instr.Aux2 = 0
				if typ != TypeUnknown && typ != TypeAny {
					instr.Type = typ
				}
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("prefilled polymorphic field cache for %q with %d shapes", name, len(cases)))
			}
		}
	}
}

func recordFieldPolyShapeCatalog(tableShapes *TableShapeFacts, cases []FieldPolyShapeCase) {
	if tableShapes == nil || len(cases) == 0 {
		return
	}
	tableShapes.RecordFieldPolyShapeCatalogCases(cases)
}

func recordFixedShapeCatalogFact(tableShapes *TableShapeFacts, fact FixedShapeTableFact) {
	if tableShapes == nil || fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
		return
	}
	tableShapes.RecordFixedShapeCatalogFact(fact)
}

func guardedFixedShapePolyFacts(facts []FixedShapeTableFact) []FixedShapeTableFact {
	if len(facts) < 2 {
		return nil
	}
	out := make([]FixedShapeTableFact, 0, len(facts))
	seen := make(map[uint32]bool, len(facts))
	for _, fact := range facts {
		if fact.ShapeID == 0 || len(fact.FieldNames) == 0 || seen[fact.ShapeID] {
			continue
		}
		fact.Guarded = true
		fact.FieldNames = append([]string(nil), fact.FieldNames...)
		fact.FieldTypes = cloneStringTypeMap(fact.FieldTypes)
		fact.FieldRanges = cloneStringRangeMap(fact.FieldRanges)
		fact.FieldLenRanges = cloneStringRangeMap(fact.FieldLenRanges)
		out = append(out, fact)
		seen[fact.ShapeID] = true
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

func seedProfiledDynamicTableValueFacts(fn *Function, facts map[int]FixedShapeTableFact) {
	if fn == nil || fn.Proto == nil || fn.Proto.TableKeyFeedback == nil {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpGetTable || !instr.HasSource || instr.SourcePC < 0 || instr.SourcePC >= len(fn.Proto.TableKeyFeedback) {
				continue
			}
			feedback := fn.Proto.TableKeyFeedback[instr.SourcePC]
			fact, ok := fixedShapeFactFromProfiledValueShape(feedback.ValueShape)
			if !ok {
				continue
			}
			facts[instr.ID] = fact
			if feedback.ValueType == vm.FBTable && (instr.Type == TypeAny || instr.Type == TypeUnknown) {
				instr.Type = TypeTable
			}
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("dynamic table value carries guarded fixed table shape %v", fact.FieldNames))
		}
	}
}

func fixedShapeFactFromProfiledValueShape(feedback vm.ArgArrayElementShapeFeedback) (FixedShapeTableFact, bool) {
	shapeID, fields, ok := feedback.StableShape()
	if !ok {
		return FixedShapeTableFact{}, false
	}
	return FixedShapeTableFact{
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
	}, true
}

func propagateFixedShapePhiFacts(fn *Function, facts map[int]FixedShapeTableFact) {
	if fn == nil || len(facts) == 0 {
		return
	}
	changed := true
	for changed {
		changed = false
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if instr.Op != OpPhi || len(instr.Args) == 0 {
					continue
				}
				if _, exists := facts[instr.ID]; exists {
					continue
				}
				fact, ok := mergeFixedShapePhiArgs(instr, facts)
				if !ok {
					continue
				}
				facts[instr.ID] = fact
				if instr.Type == TypeAny || instr.Type == TypeUnknown {
					instr.Type = TypeTable
				}
				changed = true
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("phi carries guarded fixed table shape %v", fact.FieldNames))
			}
		}
	}
}

func mergeFixedShapePhiArgs(phi *Instr, facts map[int]FixedShapeTableFact) (FixedShapeTableFact, bool) {
	var merged FixedShapeTableFact
	seen := false
	for _, arg := range phi.Args {
		if arg == nil {
			return FixedShapeTableFact{}, false
		}
		fact, ok := facts[arg.ID]
		if !ok || fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
			return FixedShapeTableFact{}, false
		}
		if !seen {
			merged = cloneFixedShapeTableFact(fact)
			seen = true
			continue
		}
		next, ok := mergeSameShapeFacts(merged, fact)
		if !ok {
			return FixedShapeTableFact{}, false
		}
		merged = next
	}
	if !seen {
		return FixedShapeTableFact{}, false
	}
	merged.Guarded = true
	return merged, true
}

func mergeSameShapeFacts(a, b FixedShapeTableFact) (FixedShapeTableFact, bool) {
	if a.ShapeID != b.ShapeID || !a.sameShape(b) {
		return FixedShapeTableFact{}, false
	}
	out := cloneFixedShapeTableFact(a)
	out.ObservationCount += b.ObservationCount
	out.FieldTypes = mergeFieldTypeFacts(a.FieldTypes, b.FieldTypes)
	out.FieldRanges = mergeFieldRangeFacts(a.FieldRanges, b.FieldRanges)
	out.FieldLenRanges = mergeFieldRangeFacts(a.FieldLenRanges, b.FieldLenRanges)
	out.FieldVMProtos = mergeFieldProtoFacts(a.FieldVMProtos, b.FieldVMProtos)
	out.FieldVMClosures = mergeFieldClosureFacts(a.FieldVMClosures, b.FieldVMClosures)
	out.FieldTableFacts = mergeNestedTableFacts(a.FieldTableFacts, b.FieldTableFacts)
	out.StringValueFact = mergeStringValueFacts(a.StringValueFact, b.StringValueFact)
	if a.ArrayElementType == b.ArrayElementType {
		out.ArrayElementType = a.ArrayElementType
	}
	if a.ArrayElementRange.known && b.ArrayElementRange.known {
		out.ArrayElementRange = intRange{
			min:   minInt64(a.ArrayElementRange.min, b.ArrayElementRange.min),
			max:   maxInt64(a.ArrayElementRange.max, b.ArrayElementRange.max),
			known: true,
		}
	}
	return out, true
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func mergeFieldTypeFacts(a, b map[string]Type) map[string]Type {
	if len(a) == 0 {
		return cloneStringTypeMap(b)
	}
	if len(b) == 0 {
		return cloneStringTypeMap(a)
	}
	out := make(map[string]Type, len(a)+len(b))
	for name, left := range a {
		if left == TypeUnknown || left == TypeAny {
			continue
		}
		right, ok := b[name]
		if !ok {
			out[name] = left
			continue
		}
		if right == TypeUnknown || right == TypeAny {
			out[name] = left
			continue
		}
		if left == right {
			out[name] = left
		}
	}
	for name, right := range b {
		if right == TypeUnknown || right == TypeAny {
			continue
		}
		if _, ok := a[name]; !ok {
			out[name] = right
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeFieldRangeFacts(a, b map[string]intRange) map[string]intRange {
	if len(a) == 0 {
		return cloneStringRangeMap(b)
	}
	if len(b) == 0 {
		return cloneStringRangeMap(a)
	}
	out := make(map[string]intRange)
	for name, left := range a {
		right, ok := b[name]
		if !ok || !left.known || !right.known {
			continue
		}
		if right.min < left.min {
			left.min = right.min
		}
		if right.max > left.max {
			left.max = right.max
		}
		out[name] = left
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeFieldProtoFacts(a, b map[string]*vm.FuncProto) map[string]*vm.FuncProto {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make(map[string]*vm.FuncProto)
	for name, left := range a {
		if right := b[name]; left != nil && left == right {
			out[name] = left
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeFieldClosureFacts(a, b map[string]uintptr) map[string]uintptr {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make(map[string]uintptr)
	for name, left := range a {
		if right := b[name]; left != 0 && left == right {
			out[name] = left
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeNestedTableFacts(a, b map[string]FixedShapeTableFact) map[string]FixedShapeTableFact {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make(map[string]FixedShapeTableFact)
	for name, left := range a {
		right, ok := b[name]
		if !ok || left.ShapeID != right.ShapeID || !left.sameShape(right) {
			continue
		}
		merged, ok := mergeSameShapeFacts(left, right)
		if ok {
			out[name] = merged
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeStringValueFacts(a, b *FixedShapeTableFact) *FixedShapeTableFact {
	if a == nil || b == nil {
		return nil
	}
	if a.ShapeID != b.ShapeID || !a.sameShape(*b) {
		return nil
	}
	merged, ok := mergeSameShapeFacts(*a, *b)
	if !ok {
		return nil
	}
	return cloneFixedShapeTableFactPtr(merged)
}

func fixedShapeFactsEqual(a, b FixedShapeTableFact) bool {
	return a.ShapeID == b.ShapeID &&
		a.ObservationCount == b.ObservationCount &&
		reflect.DeepEqual(a.FieldNames, b.FieldNames) &&
		reflect.DeepEqual(a.FieldTypes, b.FieldTypes) &&
		reflect.DeepEqual(a.FieldRanges, b.FieldRanges) &&
		reflect.DeepEqual(a.FieldLenRanges, b.FieldLenRanges) &&
		reflect.DeepEqual(a.FieldTableFacts, b.FieldTableFacts) &&
		reflect.DeepEqual(a.StringValueFact, b.StringValueFact) &&
		a.ArrayElementType == b.ArrayElementType &&
		a.ArrayElementRange == b.ArrayElementRange &&
		a.Guarded == b.Guarded &&
		a.EntryGuarded == b.EntryGuarded
}

func seedLocalStringMapValueFacts(fn *Function, facts map[int]FixedShapeTableFact) {
	if fn == nil || len(facts) == 0 {
		return
	}
	changed := true
	for changed {
		changed = false
		if seedLocalStringMapValueFactsOnce(fn, facts) {
			changed = true
		}
		if propagateStringMapValueFactsThroughPhiArgs(fn, facts) {
			changed = true
		}
	}
}

func seedLocalStringMapValueFactsOnce(fn *Function, facts map[int]FixedShapeTableFact) bool {
	changed := false
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpSetTable || len(instr.Args) < 3 ||
				instr.Args[0] == nil || instr.Args[1] == nil || instr.Args[2] == nil {
				continue
			}
			if !tableKeyProvenString(fn, instr, instr.Args[1]) {
				continue
			}
			valueFact, ok := facts[instr.Args[2].ID]
			if !ok || valueFact.ShapeID == 0 || len(valueFact.FieldNames) == 0 {
				continue
			}
			tableFact := facts[instr.Args[0].ID]
			stripped := withoutFieldValues(valueFact)
			if tableFact.StringValueFact == nil {
				tableFact.StringValueFact = cloneFixedShapeTableFactPtr(stripped)
			} else if merged := mergeStringValueFacts(tableFact.StringValueFact, &stripped); merged != nil {
				if fixedShapeFactsEqual(*tableFact.StringValueFact, *merged) {
					continue
				}
				tableFact.StringValueFact = merged
			} else {
				continue
			}
			facts[instr.Args[0].ID] = tableFact
			changed = true
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("local string-map value carries fixed table shape %v", stripped.FieldNames))
		}
	}
	return changed
}

func propagateStringMapValueFactsThroughPhiArgs(fn *Function, facts map[int]FixedShapeTableFact) bool {
	if fn == nil || len(facts) == 0 {
		return false
	}
	changed := false
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpPhi || len(instr.Args) == 0 {
				continue
			}
			phiFact, ok := facts[instr.ID]
			if !ok || phiFact.StringValueFact == nil {
				continue
			}
			for _, arg := range instr.Args {
				if arg == nil {
					continue
				}
				argFact := facts[arg.ID]
				if argFact.StringValueFact != nil {
					merged := mergeStringValueFacts(argFact.StringValueFact, phiFact.StringValueFact)
					if merged == nil || fixedShapeFactsEqual(*argFact.StringValueFact, *merged) {
						continue
					}
					argFact.StringValueFact = merged
				} else {
					argFact.StringValueFact = cloneFixedShapeTableFactPtrFromPtr(phiFact.StringValueFact)
				}
				facts[arg.ID] = argFact
				changed = true
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					"propagated string-map value fact through phi argument")
			}
		}
	}
	return changed
}

func seedLocalFieldTableFacts(fn *Function, facts map[int]FixedShapeTableFact) {
	if fn == nil || len(facts) == 0 {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpSetField || len(instr.Args) < 2 ||
				instr.Args[0] == nil || instr.Args[1] == nil {
				continue
			}
			receiverFact, ok := facts[instr.Args[0].ID]
			if !ok || receiverFact.ShapeID == 0 {
				continue
			}
			valueFact, ok := facts[instr.Args[1].ID]
			if !ok || !fixedShapeTableFactHasUsableTableFact(valueFact) {
				continue
			}
			name := fixedShapeFieldNameFromAux(fn, instr)
			if name == "" {
				continue
			}
			if _, ok := receiverFact.FieldTableFacts[name]; ok {
				continue
			}
			if receiverFact.FieldTableFacts == nil {
				receiverFact.FieldTableFacts = make(map[string]FixedShapeTableFact)
			}
			receiverFact.FieldTableFacts[name] = withoutFieldValues(valueFact)
			if receiverFact.FieldTypes == nil {
				receiverFact.FieldTypes = make(map[string]Type)
			}
			receiverFact.FieldTypes[name] = TypeTable
			facts[instr.Args[0].ID] = receiverFact
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("field %q carries local table fact", name))
		}
	}
}

func inferLocalArrayElementTableFacts(fn *Function, valueFacts map[int]FixedShapeTableFact) map[int]FixedShapeTableFact {
	if fn == nil || len(valueFacts) == 0 {
		return nil
	}
	states := make(map[int]arrayElementTableFactState)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || len(instr.Args) == 0 || instr.Args[0] == nil {
				continue
			}
			switch fixedShapeArrayElementWriteRole(instr.Op) {
			case OpFixedShapeArrayElementWriteSingle:
				if len(instr.Args) < 3 || instr.Args[1] == nil || instr.Args[2] == nil || !tableKeyProvenInt(instr.Args[1]) {
					continue
				}
				valueFact, ok := valueFacts[instr.Args[2].ID]
				if !ok || valueFact.ShapeID == 0 || len(valueFact.FieldNames) == 0 {
					continue
				}
				states[instr.Args[0].ID] = mergeArrayElementTableFactState(states[instr.Args[0].ID], valueFact)
			case OpFixedShapeArrayElementWriteVariadic:
				st := states[instr.Args[0].ID]
				for _, arg := range instr.Args[1:] {
					if arg == nil {
						continue
					}
					valueFact, ok := valueFacts[arg.ID]
					if !ok || valueFact.ShapeID == 0 || len(valueFact.FieldNames) == 0 {
						st.conflict = true
						continue
					}
					st = mergeArrayElementTableFactState(st, valueFact)
				}
				states[instr.Args[0].ID] = st
			case OpFixedShapeArrayElementWriteConflict:
				st := states[instr.Args[0].ID]
				if st.seen {
					st.conflict = true
					states[instr.Args[0].ID] = st
				}
			}
		}
	}
	if len(states) == 0 {
		return nil
	}
	out := make(map[int]FixedShapeTableFact)
	for id, st := range states {
		if st.seen && !st.conflict {
			st.fact.Guarded = true
			out[id] = st.fact
		}
	}
	propagateArrayElementFactsThroughGlobals(fn, out)
	if len(out) == 0 {
		return nil
	}
	return out
}

type arrayElementTableFactState struct {
	fact     FixedShapeTableFact
	seen     bool
	conflict bool
}

func mergeArrayElementTableFactState(st arrayElementTableFactState, valueFact FixedShapeTableFact) arrayElementTableFactState {
	valueFact = withoutFieldValues(valueFact)
	if !st.seen {
		st.fact = valueFact
		st.seen = true
		return st
	}
	if st.fact.ShapeID != valueFact.ShapeID || !st.fact.sameShape(valueFact) {
		st.conflict = true
		return st
	}
	if merged, ok := mergeSameShapeFacts(st.fact, valueFact); ok {
		st.fact = merged
	}
	return st
}

func propagateArrayElementFactsThroughGlobals(fn *Function, arrayElementFacts map[int]FixedShapeTableFact) {
	if fn == nil || len(arrayElementFacts) == 0 {
		return
	}
	type state struct {
		fact     FixedShapeTableFact
		seen     bool
		conflict bool
	}
	globals := make(map[int64]state)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || !opIsGlobalWrite(instr.Op) || len(instr.Args) == 0 || instr.Args[0] == nil {
				continue
			}
			fact, ok := arrayElementFacts[instr.Args[0].ID]
			st := globals[instr.Aux]
			if !ok {
				st.conflict = true
				globals[instr.Aux] = st
				continue
			}
			if !st.seen {
				st.fact = cloneFixedShapeTableFact(fact)
				st.seen = true
			} else if st.fact.ShapeID != fact.ShapeID || !st.fact.sameShape(fact) {
				st.conflict = true
			}
			globals[instr.Aux] = st
		}
	}
	if len(globals) == 0 {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || !opIsGlobalRead(instr.Op) {
				continue
			}
			st := globals[instr.Aux]
			if !st.seen || st.conflict {
				continue
			}
			arrayElementFacts[instr.ID] = cloneFixedShapeTableFact(st.fact)
		}
	}
}

func seedLocalArrayElementTableFacts(fn *Function, facts map[int]FixedShapeTableFact, arrayElementFacts map[int]FixedShapeTableFact) {
	if fn == nil || len(arrayElementFacts) == 0 {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			var tableValue *Value
			switch fixedShapeArrayElementReadRole(instr.Op) {
			case OpFixedShapeArrayElementReadDirect:
				if len(instr.Args) < 2 || instr.Args[0] == nil {
					continue
				}
				tableValue = instr.Args[0]
			case OpFixedShapeArrayElementReadLoweredArray:
				if table, ok := loweredTableArrayLoadTableValue(instr); ok {
					tableValue = table
				}
			default:
				continue
			}
			if tableValue == nil {
				continue
			}
			fact, ok := arrayElementFacts[tableValue.ID]
			if !ok || fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
				continue
			}
			facts[instr.ID] = cloneFixedShapeTableFact(fact)
			if instr.Type == TypeAny || instr.Type == TypeUnknown {
				instr.Type = TypeTable
			}
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("local array element carries guarded fixed table shape %v", fact.FieldNames))
		}
	}
}

func fieldPolyShapeCases(facts []FixedShapeTableFact, name string) ([]FieldPolyShapeCase, Type) {
	cases := make([]FieldPolyShapeCase, 0, len(facts))
	typ := TypeUnknown
	for _, fact := range facts {
		idx, ok := fact.fieldIndex(name)
		if !ok {
			return nil, TypeUnknown
		}
		caseType := fact.FieldTypes[name]
		if caseType == TypeUnknown || caseType == TypeAny {
			typ = TypeUnknown
		} else if typ == TypeUnknown {
			typ = caseType
		} else if typ != caseType {
			typ = TypeUnknown
		}
		cases = append(cases, FieldPolyShapeCase{
			ShapeID:      fact.ShapeID,
			Count:        fact.ObservationCount,
			FieldIdx:     idx,
			Type:         caseType,
			VMProto:      fact.FieldVMProtos[name],
			VMClosure:    fact.FieldVMClosures[name],
			ReceiverFact: fact,
		})
	}
	sort.SliceStable(cases, func(i, j int) bool {
		return cases[i].Count > cases[j].Count
	})
	return cases, typ
}

func markEntryGuardedFixedShapeArgFacts(fn *Function, tableShapes *TableShapeFacts, facts map[int]FixedShapeTableFact, argFacts map[int]FixedShapeTableFact) {
	if fn == nil || fn.Proto == nil || tableShapes == nil || len(argFacts) == 0 {
		return
	}
	for paramIdx, fact := range argFacts {
		if paramIdx < 0 || paramIdx >= fn.Proto.NumParams || fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
			continue
		}
		fact.EntryGuarded = true
		tableShapes.RecordFixedShapeEntryGuard(paramIdx, fact)
		tableShapes.RecordFixedShapeArgFact(paramIdx, fact)
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if instr.Op == OpLoadSlot && int(instr.Aux) == paramIdx {
					facts[instr.ID] = fact
				}
			}
		}
	}
}

func guardedFixedShapeArgFact(fact FixedShapeTableFact) (FixedShapeTableFact, bool) {
	if fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
		return FixedShapeTableFact{}, false
	}
	return FixedShapeTableFact{
		ShapeID:           fact.ShapeID,
		FieldNames:        append([]string(nil), fact.FieldNames...),
		FieldTypes:        cloneStringTypeMap(fact.FieldTypes),
		FieldRanges:       cloneStringRangeMap(fact.FieldRanges),
		FieldLenRanges:    cloneStringRangeMap(fact.FieldLenRanges),
		FieldVMProtos:     cloneStringProtoMap(fact.FieldVMProtos),
		FieldVMClosures:   cloneStringUintptrMap(fact.FieldVMClosures),
		FieldTableFacts:   cloneFixedShapeTableFactMap(fact.FieldTableFacts),
		StringValueFact:   cloneFixedShapeTableFactPtrFromPtr(fact.StringValueFact),
		ArrayElementType:  fact.ArrayElementType,
		ArrayElementRange: fact.ArrayElementRange,
		Guarded:           true,
	}, true
}
