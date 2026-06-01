package methodjit

import (
	"sort"

	rt "github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func runtimeFieldPolyShapeCasesFromFeedback(fn *Function, instr *Instr) ([]FieldPolyShapeCase, Type) {
	if fn == nil || instr == nil || !opIsFieldRead(instr.Op) || !instr.HasSource || instr.SourcePC < 0 {
		return nil, TypeUnknown
	}
	proto := instrSourceProto(fn, instr)
	if proto == nil || len(proto.FieldPolyCache) == 0 {
		return nil, TypeUnknown
	}
	name := fixedShapeFieldNameFromAux(fn, instr)
	if name == "" {
		return nil, TypeUnknown
	}
	slot := rt.FieldPolyCacheSlot(proto.FieldPolyCache, instr.SourcePC)
	if len(slot) == 0 {
		return nil, TypeUnknown
	}
	siteType := sourceFeedbackFieldValueTypeOrRuntime(proto, instr.SourcePC)
	cases := make([]FieldPolyShapeCase, 0, len(slot))
	seen := make(map[uint32]bool, len(slot))
	for _, entry := range slot {
		if entry.ShapeID == 0 || entry.FieldIdx < 0 || seen[entry.ShapeID] {
			continue
		}
		seen[entry.ShapeID] = true
		caseType := siteType
		if vt, stable := rt.ShapeFieldStableType(entry.ShapeID, entry.FieldIdx); stable {
			if typ, ok := runtimeValueTypeToIRType(vt); ok {
				caseType = typ
			}
		}
		cases = append(cases, FieldPolyShapeCase{
			ShapeID:  entry.ShapeID,
			Count:    1,
			FieldIdx: entry.FieldIdx,
			Type:     caseType,
			ReceiverFact: FixedShapeTableFact{
				ShapeID:    entry.ShapeID,
				FieldNames: fieldNamesWithIndex(name, entry.FieldIdx),
				FieldTypes: fieldTypesForRuntimePolyCase(name, caseType),
				Guarded:    true,
			},
		})
	}
	if len(cases) < 2 {
		return nil, TypeUnknown
	}
	sort.SliceStable(cases, func(i, j int) bool {
		return cases[i].ShapeID < cases[j].ShapeID
	})
	return cases, commonRuntimeFieldPolyCaseType(cases)
}

func getFieldReceiverFixedShapeFact(fn *Function, instr *Instr) (FixedShapeTableFact, int, bool) {
	return getFieldReceiverFixedShapeFactWithFacts(fn, functionTableShapeFacts(fn), instr)
}

func getFieldReceiverFixedShapeFactWithFacts(fn *Function, tableShapes *TableShapeFacts, instr *Instr) (FixedShapeTableFact, int, bool) {
	if fn == nil || instr == nil || !opIsFieldRead(instr.Op) || len(instr.Args) == 0 || instr.Args[0] == nil {
		return FixedShapeTableFact{}, 0, false
	}
	if tableShapes == nil {
		return FixedShapeTableFact{}, 0, false
	}
	facts := tableShapes.FixedShapeTableMap()
	if len(facts) == 0 {
		return FixedShapeTableFact{}, 0, false
	}
	fact, ok := facts[instr.Args[0].ID]
	if !ok || fact.ShapeID == 0 {
		return FixedShapeTableFact{}, 0, false
	}
	name := fixedShapeFieldNameFromAux(fn, instr)
	if name == "" {
		return FixedShapeTableFact{}, 0, false
	}
	idx, ok := fact.fieldIndex(name)
	if !ok {
		return FixedShapeTableFact{}, 0, false
	}
	return fact, idx, true
}

type setFieldAppendShapeCase struct {
	PreShapeID  uint32
	PostShapeID uint32
	FieldIdx    int
	PostShape   *rt.Shape
}

func fixedShapeAppendSetFieldCases(fn *Function, tableShapes *TableShapeFacts, instr *Instr) []setFieldAppendShapeCase {
	if fn == nil || instr == nil || instr.Op != OpSetField || len(instr.Args) < 2 || instr.Args[1] == nil {
		return nil
	}
	field := fixedShapeFieldNameFromAux(fn, instr)
	if field == "" || !valueProvenNonNil(instr.Args[1]) {
		return nil
	}
	if tableShapes == nil {
		return nil
	}
	facts := tableShapes.FixedShapeTableMap()
	if len(facts) == 0 {
		return nil
	}
	seenFacts := make(map[uint32]bool)
	seenCases := make(map[uint32]bool)
	cases := make([]setFieldAppendShapeCase, 0, rt.FieldPolyCacheWays)
	for _, fact := range facts {
		if fact.ShapeID == 0 || len(fact.FieldNames) == 0 || len(fact.FieldNames) >= rt.SmallFieldCap || seenFacts[fact.ShapeID] {
			continue
		}
		seenFacts[fact.ShapeID] = true
		if _, exists := fact.fieldIndex(field); exists {
			continue
		}
		for _, keys := range fixedShapeSparseKeyCasesForAppend(fact) {
			if len(keys) >= rt.SmallFieldCap {
				continue
			}
			preShape := rt.GetShape(keys)
			if preShape == nil || preShape.ID == 0 || seenCases[preShape.ID] {
				continue
			}
			postKeys := append(append([]string(nil), keys...), field)
			postShape := rt.GetShape(postKeys)
			if postShape == nil || postShape.ID == 0 {
				continue
			}
			seenCases[preShape.ID] = true
			cases = append(cases, setFieldAppendShapeCase{
				PreShapeID:  preShape.ID,
				PostShapeID: postShape.ID,
				FieldIdx:    len(keys),
				PostShape:   postShape,
			})
			if len(cases) >= rt.FieldPolyCacheWays {
				return cases
			}
		}
	}
	return cases
}

func fixedShapeSparseKeyCasesForAppend(fact FixedShapeTableFact) [][]string {
	if len(fact.FieldNames) == 0 || len(fact.FieldNames) > 8 {
		return nil
	}
	required := uint64(0)
	for i, field := range fact.FieldNames {
		if typ, ok := fact.FieldTypes[field]; ok && typ != TypeUnknown && typ != TypeAny && typ != TypeNil {
			required |= uint64(1) << uint(i)
		}
	}
	if required == 0 {
		return [][]string{append([]string(nil), fact.FieldNames...)}
	}
	allMask := uint64(1)<<uint(len(fact.FieldNames)) - 1
	out := make([][]string, 0, 4)
	for mask := uint64(1); mask <= allMask; mask++ {
		if mask&required != required {
			continue
		}
		keys := make([]string, 0, len(fact.FieldNames))
		for i, field := range fact.FieldNames {
			if mask&(uint64(1)<<uint(i)) != 0 {
				keys = append(keys, field)
			}
		}
		out = append(out, keys)
		if len(out) >= rt.FieldPolyCacheWays {
			break
		}
	}
	return out
}

func sourceFeedbackFieldValueTypeOrRuntime(proto *vm.FuncProto, pc int) Type {
	if typ, ok := sourceFeedbackFieldValueType(proto, pc); ok {
		return typ
	}
	return TypeUnknown
}

func fieldNamesWithIndex(name string, idx int) []string {
	if idx < 0 {
		return nil
	}
	names := make([]string, idx+1)
	names[idx] = name
	return names
}

func fieldTypesForRuntimePolyCase(name string, typ Type) map[string]Type {
	if typ == TypeUnknown || typ == TypeAny || name == "" {
		return nil
	}
	return map[string]Type{name: typ}
}

func commonRuntimeFieldPolyCaseType(cases []FieldPolyShapeCase) Type {
	typ := TypeUnknown
	for _, c := range cases {
		if c.Type == TypeUnknown || c.Type == TypeAny {
			return TypeUnknown
		}
		if typ == TypeUnknown {
			typ = c.Type
			continue
		}
		if typ != c.Type {
			return TypeUnknown
		}
	}
	return typ
}
