// pass_fixed_shape_table_annotate.go: IR annotation driven by fixed-shape table
// facts for field accesses — prefilling GetField/SetField/field-load shape
// caches, mutable-field detection and range tracking, string-value access
// annotation, and shape-cache propagation through phis.
// Pure code movement from pass_fixed_shape_table.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/vm"
)

func annotateFixedShapeGetFields(fn *Function, facts map[int]FixedShapeTableFact) {
	mutableFields := collectFixedShapeMutableFields(fn)
	mutableRanges := collectFixedShapeMutableFieldRanges(fn, facts, mutableFields)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if annotateFixedShapeFieldLoad(fn, block, instr, facts, mutableFields, mutableRanges) {
				continue
			}
			if instr.Op != OpGetField || len(instr.Args) == 0 || instr.Args[0] == nil {
				if instr.Op == OpGuardType && len(instr.Args) > 0 && instr.Args[0] != nil && instr.Type == TypeTable {
					if fact, ok := facts[instr.Args[0].ID]; ok {
						facts[instr.ID] = fact
					}
				}
				continue
			}
			fact, ok := facts[instr.Args[0].ID]
			if !ok || fact.ShapeID == 0 {
				continue
			}
			name := fixedShapeFieldNameFromAux(fn, instr)
			if name == "" {
				continue
			}
			idx, ok := fact.fieldIndex(name)
			if !ok {
				continue
			}
			if instr.Aux2 == 0 {
				instr.Aux2 = int64(fact.ShapeID)<<32 | int64(uint32(idx))
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("prefilled fixed-shape field cache for %q", name))
			}
			if typ, ok := fact.FieldTypes[name]; ok && typ != TypeUnknown && typ != TypeAny {
				instr.Type = typ
			}
			if r, ok := fixedShapeFieldLoadRange(fact, name, idx, mutableFields, mutableRanges); ok {
				fn.Analysis.NumericFacts().RecordProfiledIntRange(instr.ID, r)
				if instr.Type == TypeAny || instr.Type == TypeUnknown {
					instr.Type = TypeInt
				}
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("field %q carries guarded int range [%d,%d]", name, r.min, r.max))
			}
			if r, ok := fact.FieldLenRanges[name]; ok && r.known && !fixedShapeFieldMayMutate(mutableFields, fact.ShapeID, idx) {
				recordProfiledLenRange(fn, instr.ID, r)
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("field %q carries guarded string-len range [%d,%d]", name, r.min, r.max))
			}
			if nested, ok := fact.FieldTableFacts[name]; ok && fixedShapeTableFactHasUsableTableFact(nested) {
				facts[instr.ID] = nested
				if instr.Type == TypeAny || instr.Type == TypeUnknown {
					instr.Type = TypeTable
				}
				if nested.ShapeID != 0 {
					functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
						fmt.Sprintf("field %q carries guarded nested fixed table shape %v", name, nested.FieldNames))
				} else {
					functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
						fmt.Sprintf("field %q carries guarded nested array element type %s", name, nested.ArrayElementType))
				}
			}
		}
	}
}

func annotateFixedShapeStringValueAccesses(fn *Function, tableShapes *TableShapeFacts, facts map[int]FixedShapeTableFact) {
	if fn == nil || len(facts) == 0 {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || len(instr.Args) == 0 || instr.Args[0] == nil {
				continue
			}
			var table *Value
			switch instr.Op {
			case OpGetTable:
				if len(instr.Args) < 2 || !tableKeyProvenString(fn, instr, instr.Args[1]) {
					continue
				}
				table = instr.Args[0]
			case OpGetTableStringFormatInt:
				table = instr.Args[0]
			default:
				continue
			}
			fact, ok := facts[table.ID]
			if !ok || fact.StringValueFact == nil || !fixedShapeTableFactHasUsableTableFact(*fact.StringValueFact) {
				continue
			}
			valueFact := cloneFixedShapeTableFact(*fact.StringValueFact)
			facts[instr.ID] = valueFact
			recordFixedShapeCatalogFact(tableShapes, valueFact)
			if instr.Type == TypeAny || instr.Type == TypeUnknown {
				instr.Type = TypeTable
			}
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("string-map value carries guarded fixed table shape %v", valueFact.FieldNames))
		}
	}
}

func tableKeyProvenString(fn *Function, instr *Instr, key *Value) bool {
	if key != nil && key.Def != nil && (key.Def.Type == TypeString || key.Def.Op == OpConstString || key.Def.Op == OpStringFormatInt || key.Def.Op == OpStringFormatConst) {
		return true
	}
	if key != nil && key.Def != nil && isStringFieldCall(fn, key.Def, "format") {
		return true
	}
	proto := instrSourceProto(fn, instr)
	if proto == nil || instr == nil || !instr.HasSource || instr.SourcePC < 0 {
		return false
	}
	return instr.SourcePC < len(proto.Feedback) && proto.Feedback[instr.SourcePC].Right == vm.FBString
}

func annotateFixedShapeFieldLoad(fn *Function, block *Block, instr *Instr, facts map[int]FixedShapeTableFact, mutableFields map[uint32]map[int]bool, mutableRanges map[uint32]map[int]intRange) bool {
	if instr == nil || (instr.Op != OpFieldLoad && instr.Op != OpFieldLoadNumToFloat) || len(instr.Args) == 0 || instr.Args[0] == nil {
		return false
	}
	svals := instr.Args[0].Def
	if svals == nil || svals.Op != OpFieldSvals || len(svals.Args) == 0 || svals.Args[0] == nil {
		return true
	}
	fact, ok := fixedShapeFactForFieldSvals(fn, facts, svals)
	if !ok || fact.ShapeID == 0 || fact.ShapeID != uint32(svals.Aux) {
		return true
	}
	fieldIdx := int(instr.Aux)
	if fieldIdx < 0 || fieldIdx >= len(fact.FieldNames) {
		return true
	}
	name := fact.FieldNames[fieldIdx]
	if typ, ok := fact.FieldTypes[name]; ok && typ != TypeUnknown && typ != TypeAny && instr.Op == OpFieldLoad {
		instr.Type = typ
	}
	fieldMayMutate := fixedShapeFieldMayMutate(mutableFields, fact.ShapeID, fieldIdx)
	if fieldMayMutate {
		numeric := fn.Analysis.NumericFacts()
		numeric.DeleteProfiledIntRange(instr.ID)
		numeric.DeleteProfiledLenRange(instr.ID)
	}
	if r, ok := fixedShapeFieldLoadRange(fact, name, fieldIdx, mutableFields, mutableRanges); ok {
		fn.Analysis.NumericFacts().RecordProfiledIntRange(instr.ID, r)
		if instr.Op == OpFieldLoad && (instr.Type == TypeAny || instr.Type == TypeUnknown) {
			instr.Type = TypeInt
		}
		functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
			fmt.Sprintf("field-load %q carries guarded int range [%d,%d]", name, r.min, r.max))
	}
	if r, ok := fact.FieldLenRanges[name]; ok && r.known && !fieldMayMutate {
		recordProfiledLenRange(fn, instr.ID, r)
		functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
			fmt.Sprintf("field-load %q carries guarded string-len range [%d,%d]", name, r.min, r.max))
	}
	if nested, ok := fact.FieldTableFacts[name]; ok && fixedShapeTableFactHasUsableTableFact(nested) && instr.Op == OpFieldLoad {
		facts[instr.ID] = nested
		if instr.Type == TypeAny || instr.Type == TypeUnknown {
			instr.Type = TypeTable
		}
		if nested.ShapeID != 0 {
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("field-load %q carries guarded nested fixed table shape %v", name, nested.FieldNames))
		} else {
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("field-load %q carries guarded nested array element type %s", name, nested.ArrayElementType))
		}
	}
	return true
}

func collectFixedShapeMutableFields(fn *Function) map[uint32]map[int]bool {
	mutable := make(map[uint32]map[int]bool)
	if fn == nil {
		return mutable
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpFieldStore || len(instr.Args) == 0 || instr.Args[0] == nil {
				continue
			}
			svals := instr.Args[0].Def
			if svals == nil || svals.Op != OpFieldSvals || svals.Aux == 0 {
				continue
			}
			fieldIdx := int(instr.Aux)
			if fieldIdx < 0 {
				continue
			}
			shapeID := uint32(svals.Aux)
			if mutable[shapeID] == nil {
				mutable[shapeID] = make(map[int]bool)
			}
			mutable[shapeID][fieldIdx] = true
		}
	}
	return mutable
}

func collectFixedShapeMutableFieldRanges(fn *Function, facts map[int]FixedShapeTableFact, mutable map[uint32]map[int]bool) map[uint32]map[int]intRange {
	out := make(map[uint32]map[int]intRange)
	if fn == nil || len(mutable) == 0 || !fixedShapeMutableRangeFactsSafeFunction(fn) {
		return out
	}
	numeric := functionNumericFacts(fn)

	merge := func(shapeID uint32, fieldIdx int, r intRange) {
		if shapeID == 0 || fieldIdx < 0 || !r.known || !fixedShapeFieldMayMutate(mutable, shapeID, fieldIdx) {
			return
		}
		if out[shapeID] == nil {
			out[shapeID] = make(map[int]intRange)
		}
		if old, ok := out[shapeID][fieldIdx]; ok {
			out[shapeID][fieldIdx] = joinRange(old, r)
		} else {
			out[shapeID][fieldIdx] = r
		}
	}

	type mutableFieldStore struct {
		shapeID  uint32
		fieldIdx int
		value    *Value
	}
	stores := make(map[uint32]map[int][]mutableFieldStore)
	addStore := func(shapeID uint32, fieldIdx int, value *Value) {
		if shapeID == 0 || fieldIdx < 0 || !fixedShapeFieldMayMutate(mutable, shapeID, fieldIdx) {
			return
		}
		if stores[shapeID] == nil {
			stores[shapeID] = make(map[int][]mutableFieldStore)
		}
		stores[shapeID][fieldIdx] = append(stores[shapeID][fieldIdx], mutableFieldStore{
			shapeID:  shapeID,
			fieldIdx: fieldIdx,
			value:    value,
		})
	}

	for _, fact := range facts {
		if fact.ShapeID == 0 {
			continue
		}
		for fieldIdx, name := range fact.FieldNames {
			if r, ok := fact.FieldRanges[name]; ok {
				merge(fact.ShapeID, fieldIdx, r)
			}
		}
	}
	if fn.Analysis != nil {
		fn.Analysis.TableShapeFacts().ForEachFieldPolyShapeCatalogFact(func(_ uint32, fact FixedShapeTableFact) bool {
			if fact.ShapeID == 0 {
				return true
			}
			for fieldIdx, name := range fact.FieldNames {
				if r, ok := fact.FieldRanges[name]; ok {
					merge(fact.ShapeID, fieldIdx, r)
				}
			}
			return true
		})
	}

	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpFieldStore || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil {
				continue
			}
			svals := instr.Args[0].Def
			if svals == nil || svals.Op != OpFieldSvals || svals.Aux == 0 {
				continue
			}
			shapeID := uint32(svals.Aux)
			fieldIdx := int(instr.Aux)
			addStore(shapeID, fieldIdx, instr.Args[1])
		}
	}
	closed := make(map[uint32]map[int]bool)
	markClosed := func(shapeID uint32, fieldIdx int) {
		if closed[shapeID] == nil {
			closed[shapeID] = make(map[int]bool)
		}
		closed[shapeID][fieldIdx] = true
	}
	for iter := 0; iter < 4; iter++ {
		changed := false
		for shapeID, fields := range stores {
			for fieldIdx, fieldStores := range fields {
				before, hadBefore := out[shapeID][fieldIdx]
				next := before
				hadNext := hadBefore
				okAll := true
				for _, store := range fieldStores {
					r, selfDep, ok := rangeForMutableFieldStoreValue(numeric, store.value, out, shapeID, fieldIdx)
					if !ok {
						okAll = false
						break
					}
					if selfDep {
						if !hadBefore || r.min < before.min {
							okAll = false
							break
						}
						r = intRange{min: before.min, max: MaxInt48, known: true}
					}
					if r.nonNegative() && store.value != nil {
						numeric.RecordIntNonNegative(store.value.ID)
					}
					if hadNext {
						next = joinRange(next, r)
					} else {
						next = r
						hadNext = true
					}
				}
				if !okAll || !hadNext {
					continue
				}
				markClosed(shapeID, fieldIdx)
				if !hadBefore || !rangeEqual(before, next) {
					if out[shapeID] == nil {
						out[shapeID] = make(map[int]intRange)
					}
					out[shapeID][fieldIdx] = next
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	for shapeID, fields := range stores {
		for fieldIdx := range fields {
			if closed[shapeID] != nil && closed[shapeID][fieldIdx] {
				continue
			}
			if out[shapeID] != nil {
				delete(out[shapeID], fieldIdx)
			}
		}
	}
	return out
}

func fixedShapeMutableRangeFactsSafeFunction(fn *Function) bool {
	if fn == nil {
		return false
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpCall, OpCallFloor, OpFieldCallFloor, OpResume, OpYield, OpTForCall:
				return false
			}
		}
	}
	return true
}

func rangeForMutableFieldStoreValue(numeric *NumericFacts, v *Value, mutableRanges map[uint32]map[int]intRange, targetShape uint32, targetField int) (intRange, bool, bool) {
	if c, ok := constIntFromValue(v); ok {
		return pointRange(c), false, true
	}
	if v == nil {
		return intRange{}, false, false
	}
	if numeric != nil {
		if r, ok := numeric.IntRange(v.ID); ok && r.known {
			return r, false, true
		}
		if r, ok := numeric.ProfiledIntRange(v.ID); ok && r.known {
			return r, false, true
		}
		if numeric.IsIntNonNegative(v.ID) {
			return intRange{min: 0, max: MaxInt48, known: true}, false, true
		}
	}
	if r, ok := simpleForwardInductionValueRange(v); ok {
		return r, false, true
	}
	if v.Def == nil || !v.Def.Type.isIntegerLike() {
		return intRange{}, false, false
	}
	return rangeForMutableFieldStoreInstr(numeric, v.Def, mutableRanges, targetShape, targetField)
}

func simpleForwardInductionValueRange(v *Value) (intRange, bool) {
	if v == nil || v.Def == nil || v.Def.Op != OpAddInt {
		return intRange{}, false
	}
	step, phi := tableArrayForwardStepWithPhi(v.Def)
	if step < 0 || phi == nil {
		return intRange{}, false
	}
	initKnown := false
	minValue := int64(0)
	for _, arg := range phi.Args {
		if arg == nil {
			return intRange{}, false
		}
		if arg.ID == v.ID {
			continue
		}
		init, ok := constIntFromValue(arg)
		if !ok || init < 0 {
			return intRange{}, false
		}
		candidate := satAdd(init, step)
		if !initKnown || candidate < minValue {
			minValue = candidate
			initKnown = true
		}
	}
	if !initKnown {
		return intRange{}, false
	}
	return intRange{min: minValue, max: MaxInt48, known: true}, true
}

func rangeForMutableFieldStoreInstr(numeric *NumericFacts, instr *Instr, mutableRanges map[uint32]map[int]intRange, targetShape uint32, targetField int) (intRange, bool, bool) {
	if instr == nil {
		return intRange{}, false, false
	}
	switch instr.Op {
	case OpFieldLoad:
		if len(instr.Args) == 0 || instr.Args[0] == nil || instr.Args[0].Def == nil {
			return intRange{}, false, false
		}
		svals := instr.Args[0].Def
		if svals.Op != OpFieldSvals || svals.Aux == 0 {
			return intRange{}, false, false
		}
		shapeID := uint32(svals.Aux)
		fieldIdx := int(instr.Aux)
		fields := mutableRanges[shapeID]
		r, ok := fields[fieldIdx]
		if !ok || !r.known {
			return intRange{}, false, false
		}
		return r, shapeID == targetShape && fieldIdx == targetField, true
	case OpAddInt, OpSubInt, OpMulInt:
		if len(instr.Args) < 2 {
			return intRange{}, false, false
		}
		left, leftSelf, leftOK := rangeForMutableFieldStoreValue(numeric, instr.Args[0], mutableRanges, targetShape, targetField)
		right, rightSelf, rightOK := rangeForMutableFieldStoreValue(numeric, instr.Args[1], mutableRanges, targetShape, targetField)
		if !leftOK || !rightOK {
			return intRange{}, false, false
		}
		switch instr.Op {
		case OpAddInt:
			if leftSelf != rightSelf {
				selfRange, otherRange := left, right
				if rightSelf {
					selfRange, otherRange = right, left
				}
				if selfRange.known && otherRange.nonNegative() {
					return intRange{min: satAdd(selfRange.min, otherRange.min), max: MaxInt48, known: true}, true, true
				}
			}
			return addRange(left, right), leftSelf || rightSelf, true
		case OpSubInt:
			return subRange(left, right), leftSelf || rightSelf, true
		default:
			return mulRange(left, right), leftSelf || rightSelf, true
		}
	case OpModInt:
		if len(instr.Args) < 2 {
			return intRange{}, false, false
		}
		divisor, ok := constIntFromValue(instr.Args[1])
		if !ok || divisor <= 0 {
			return intRange{}, false, false
		}
		_, selfDep, lhsOK := rangeForMutableFieldStoreValue(numeric, instr.Args[0], mutableRanges, targetShape, targetField)
		if !lhsOK {
			return intRange{}, false, false
		}
		return intRange{min: 0, max: divisor - 1, known: true}, selfDep, true
	default:
		return intRange{}, false, false
	}
}

func fixedShapeFieldLoadRange(fact FixedShapeTableFact, name string, fieldIdx int, mutableFields map[uint32]map[int]bool, mutableRanges map[uint32]map[int]intRange) (intRange, bool) {
	if fixedShapeFieldMayMutate(mutableFields, fact.ShapeID, fieldIdx) {
		if fields := mutableRanges[fact.ShapeID]; fields != nil {
			r, ok := fields[fieldIdx]
			return r, ok && r.known
		}
		return intRange{}, false
	}
	r, ok := fact.FieldRanges[name]
	return r, ok && r.known
}

func fixedShapeFieldMayMutate(mutable map[uint32]map[int]bool, shapeID uint32, fieldIdx int) bool {
	if len(mutable) == 0 || shapeID == 0 || fieldIdx < 0 {
		return false
	}
	return mutable[shapeID][fieldIdx]
}

func fixedShapeFactForFieldSvals(fn *Function, facts map[int]FixedShapeTableFact, svals *Instr) (FixedShapeTableFact, bool) {
	if svals == nil || len(svals.Args) == 0 || svals.Args[0] == nil || svals.Aux == 0 {
		return FixedShapeTableFact{}, false
	}
	shapeID := uint32(svals.Aux)
	if fact, ok := facts[svals.Args[0].ID]; ok && fact.ShapeID == shapeID {
		return fact, true
	}
	if fn != nil {
		tableShapes := fn.Analysis.TableShapeFacts()
		if fact, ok := tableShapes.FieldPolyShapeCatalogFact(shapeID); ok && fact.ShapeID == shapeID {
			return fact, true
		}
		if tableShapes.FieldPolyShapeFactCount() == 0 {
			return FixedShapeTableFact{}, false
		}
		var found FixedShapeTableFact
		ambiguous := false
		tableShapes.RangeFieldPolyShapeCases(func(_ int, cases []FieldPolyShapeCase) bool {
			for _, c := range cases {
				if c.ShapeID != shapeID || c.ReceiverFact.ShapeID != shapeID {
					continue
				}
				if found.ShapeID != 0 {
					ambiguous = true
					return false
				}
				found = c.ReceiverFact
			}
			return true
		})
		if ambiguous || found.ShapeID == 0 {
			return FixedShapeTableFact{}, false
		}
		return found, true
	}
	return FixedShapeTableFact{}, false
}

func annotateFixedShapeSetFields(fn *Function, facts map[int]FixedShapeTableFact) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpSetField || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil {
				continue
			}
			if instr.Aux2 != 0 || !valueProvenNonNil(instr.Args[1]) {
				continue
			}
			fact, ok := facts[instr.Args[0].ID]
			if !ok || fact.ShapeID == 0 {
				continue
			}
			name := fixedShapeFieldNameFromAux(fn, instr)
			if name == "" {
				continue
			}
			idx, ok := fact.fieldIndex(name)
			if !ok {
				continue
			}
			instr.Aux2 = int64(fact.ShapeID)<<32 | int64(uint32(idx))
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("prefilled fixed-shape setfield cache for %q", name))
		}
	}
}

// propagateShapeCacheFromSetFieldToGetField propagates fixed-shape cache and
// field-type information from SetField instructions to GetField instructions
// on Phi-merged table values. This handles the common pattern where a table is
// created with a known fixed shape in one branch, stored via SetTable, and
// later loaded via GetTable + Phi in a subsequent iteration. The Phi-merged
// value has no direct shape fact (because one Phi input is a dynamic GetTable
// result), but the SetField instructions on the original constructor carry
// shape cache metadata.
//
// The pass works in two phases:
//  1. Collect per-shape field info (field indices and types) from SetField
//     instructions with known shape caches.
//  2. For each GetField whose table argument is a Phi where at least one input
//     has a known fixed-shape fact with the same shape, propagate the shape
//     cache and field type.
func propagateShapeCacheFromSetFieldToGetField(fn *Function, facts map[int]FixedShapeTableFact) {
	if fn == nil || len(facts) == 0 {
		return
	}

	// shapeFieldInfo holds field index and type info for one shape.
	type shapeFieldInfo struct {
		fieldIdx map[string]int
	}
	// Collect field info per shape ID from SetField instructions.
	shapeFields := make(map[uint32]*shapeFieldInfo)

	// Build a shape-ID -> field-types lookup from existing facts.
	shapeFieldTypes := make(map[uint32]map[string]Type)
	for _, fact := range facts {
		if fact.ShapeID != 0 && len(fact.FieldTypes) > 0 {
			if existing, ok := shapeFieldTypes[fact.ShapeID]; !ok || len(existing) < len(fact.FieldTypes) {
				shapeFieldTypes[fact.ShapeID] = fact.FieldTypes
			}
		}
	}

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpSetField || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Aux2 == 0 {
				continue
			}
			shapeID := uint32(instr.Aux2 >> 32)
			fieldIdx := int(int32(instr.Aux2 & 0xFFFFFFFF))
			name := fixedShapeFieldNameFromAux(fn, instr)
			if name == "" || shapeID == 0 {
				continue
			}
			info := shapeFields[shapeID]
			if info == nil {
				info = &shapeFieldInfo{
					fieldIdx: make(map[string]int),
				}
				shapeFields[shapeID] = info
			}
			info.fieldIdx[name] = fieldIdx
		}
	}

	if len(shapeFields) == 0 {
		return
	}

	// Build a map from table SSA value ID to the best known shape ID.
	// For values that already have a fact, use that. For Phi values where
	// at least one input has a fixed-shape fact, propagate the shape ID,
	// provided all known-shape inputs agree.
	tableShapeID := make(map[int]uint32)
	for id, fact := range facts {
		if fact.ShapeID != 0 {
			tableShapeID[id] = fact.ShapeID
		}
	}
	// Propagate through Phis: if a Phi has at least one input with a known
	// shape AND all known-shape inputs agree on the same shape ID, propagate.
	changed := true
	for changed {
		changed = false
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if instr.Op != OpPhi || len(instr.Args) == 0 {
					continue
				}
				if _, exists := tableShapeID[instr.ID]; exists {
					continue
				}
				var candidate uint32
				conflict := false
				for _, arg := range instr.Args {
					if arg == nil {
						continue
					}
					if sid, ok := tableShapeID[arg.ID]; ok {
						if candidate == 0 {
							candidate = sid
						} else if candidate != sid {
							conflict = true
							break
						}
					}
				}
				if candidate != 0 && !conflict {
					tableShapeID[instr.ID] = candidate
					changed = true
				}
			}
		}
	}

	// First annotate SetField instructions on Phi-merged table values
	// that don't yet have a shape cache. This is needed so that
	// FieldSvalsLower can see them as lowerable.
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpSetField || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil || instr.Aux2 != 0 {
				continue
			}
			shapeID, ok := tableShapeID[instr.Args[0].ID]
			if !ok {
				continue
			}
			info, ok := shapeFields[shapeID]
			if !ok {
				continue
			}
			name := fixedShapeFieldNameFromAux(fn, instr)
			if name == "" {
				continue
			}
			fieldIdx, ok := info.fieldIdx[name]
			if !ok {
				continue
			}
			if !valueProvenNonNil(instr.Args[1]) {
				continue
			}
			instr.Aux2 = int64(shapeID)<<32 | int64(uint32(fieldIdx))
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("prefilled fixed-shape setfield cache for %q from shape propagation", name))
		}
	}

	// Now annotate GetField instructions that don't yet have a shape cache.
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpGetField || len(instr.Args) == 0 || instr.Args[0] == nil || instr.Aux2 != 0 {
				continue
			}
			shapeID, ok := tableShapeID[instr.Args[0].ID]
			if !ok {
				continue
			}
			info, ok := shapeFields[shapeID]
			if !ok {
				continue
			}
			name := fixedShapeFieldNameFromAux(fn, instr)
			if name == "" {
				continue
			}
			fieldIdx, ok := info.fieldIdx[name]
			if !ok {
				continue
			}
			instr.Aux2 = int64(shapeID)<<32 | int64(uint32(fieldIdx))
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("prefilled fixed-shape field cache for %q from matching SetField", name))
			// Propagate field type from the shape-level type info.
			if sft, ok := shapeFieldTypes[shapeID]; ok {
				if typ, ok := sft[name]; ok && typ != TypeUnknown && typ != TypeAny {
					instr.Type = typ
				}
			}
		}
	}
}

func fixedShapeTableFactHasUsableTableFact(fact FixedShapeTableFact) bool {
	return fact.ShapeID != 0 || fact.ArrayElementType != TypeUnknown || fact.ArrayElementRange.known || fact.StringValueFact != nil
}

func fixedShapeFieldNameFromAux(fn *Function, instr *Instr) string {
	if instr == nil {
		return ""
	}
	proto := instrSourceProto(fn, instr)
	if proto == nil || instr.Aux < 0 || int(instr.Aux) >= len(proto.Constants) {
		return fieldNameFromAux(fn, instr.Aux)
	}
	k := proto.Constants[instr.Aux]
	if !k.IsString() {
		return fieldNameFromAux(fn, instr.Aux)
	}
	return k.Str()
}
