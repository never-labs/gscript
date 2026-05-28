// pass_fixed_shape_table_local.go: local and return-fact inference — analysis
// of fixed-shape table constructors (OpNewTable/OpNewFixedTable plus field
// stores) within a function, plus the caller-safe return-fact analyzers and
// field classification used for interprocedural propagation.
// Pure code movement from pass_fixed_shape_table.go; no behavior change.

package methodjit

import (
	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// AnalyzeFixedShapeArrayElementReturnFact reports whether proto returns an
// array-like table whose element stores all carry the same fixed table shape.
func AnalyzeFixedShapeArrayElementReturnFact(proto *vm.FuncProto, globals map[string]*vm.FuncProto) (FixedShapeTableFact, bool) {
	if proto == nil {
		return FixedShapeTableFact{}, false
	}
	fn := BuildGraph(proto)
	if fn == nil || fn.Unpromotable {
		return FixedShapeTableFact{}, false
	}
	values := inferFixedShapeValuesForArgs(fn, globals)
	if len(values) == 0 {
		return FixedShapeTableFact{}, false
	}
	returned := make(map[int]bool)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpReturn || len(instr.Args) != 1 || instr.Args[0] == nil {
				continue
			}
			returned[instr.Args[0].ID] = true
		}
	}
	if len(returned) == 0 {
		return FixedShapeTableFact{}, false
	}
	var out FixedShapeTableFact
	seenStore := false
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if len(instr.Args) == 0 || instr.Args[0] == nil || !returned[instr.Args[0].ID] {
				continue
			}
			switch fixedShapeReturnArrayElementRole(instr.Op) {
			case OpFixedShapeReturnArrayElementStore:
				if len(instr.Args) < 3 || instr.Args[2] == nil {
					return FixedShapeTableFact{}, false
				}
				fact, ok := values[instr.Args[2].ID]
				if !ok || fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
					return FixedShapeTableFact{}, false
				}
				if !seenStore {
					out = withoutFieldValues(fact)
					seenStore = true
					continue
				}
				if out.ShapeID != fact.ShapeID || !out.sameShape(fact) {
					return FixedShapeTableFact{}, false
				}
			case OpFixedShapeReturnArrayElementInvalidator:
				return FixedShapeTableFact{}, false
			}
		}
	}
	if !seenStore {
		return FixedShapeTableFact{}, false
	}
	return out, true
}

// AnalyzeFixedShapeReturnFact reports whether every non-empty return in proto
// returns a freshly allocated table with the same ordered static string fields.
func AnalyzeFixedShapeReturnFact(proto *vm.FuncProto) (FixedShapeTableFact, bool) {
	if proto == nil {
		return FixedShapeTableFact{}, false
	}
	fn := BuildGraph(proto)
	if fn == nil || fn.Unpromotable {
		return FixedShapeTableFact{}, false
	}
	facts := inferLocalFixedShapeTables(fn)
	seedLocalStringMapValueFacts(fn, facts)
	seedLocalFieldTableFacts(fn, facts)
	instrByID := fixedShapeInstrByID(fn)
	var out FixedShapeTableFact
	var fieldAgg map[string]fixedShapeFieldAccumulator
	seenReturn := false
	seenEmpty := false
	emptyReturnCount := 0
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpReturn {
				continue
			}
			if len(instr.Args) != 1 || instr.Args[0] == nil {
				return FixedShapeTableFact{}, false
			}
			fact, ok := facts[instr.Args[0].ID]
			if !ok {
				return FixedShapeTableFact{}, false
			}
			if len(fact.FieldNames) == 0 {
				seenEmpty = true
				emptyReturnCount++
				seenReturn = true
				if fieldAgg != nil {
					for _, name := range out.FieldNames {
						fieldAgg[name] = mergeFixedShapeField(fieldAgg[name], FixedShapeFieldFact{
							Kind:     FixedShapeFieldNil,
							MaybeNil: true,
						})
					}
				}
				continue
			}
			if !seenReturn {
				out = withoutFieldValues(fact)
				fieldAgg = make(map[string]fixedShapeFieldAccumulator, len(out.FieldNames))
				for _, name := range out.FieldNames {
					for i := 0; i < emptyReturnCount; i++ {
						fieldAgg[name] = mergeFixedShapeField(fieldAgg[name], FixedShapeFieldFact{
							Kind:     FixedShapeFieldNil,
							MaybeNil: true,
						})
					}
					fieldAgg[name] = mergeFixedShapeField(fieldAgg[name],
						classifyReturnedField(fn, instrByID, fact, name))
				}
				seenReturn = true
				continue
			}
			if len(out.FieldNames) != 0 && !out.sameShape(fact) {
				return FixedShapeTableFact{}, false
			}
			if len(out.FieldNames) == 0 {
				out = withoutFieldValues(fact)
				fieldAgg = make(map[string]fixedShapeFieldAccumulator, len(out.FieldNames))
			}
			for _, name := range out.FieldNames {
				if !fieldAgg[name].seen {
					for i := 0; i < emptyReturnCount; i++ {
						fieldAgg[name] = mergeFixedShapeField(fieldAgg[name], FixedShapeFieldFact{
							Kind:     FixedShapeFieldNil,
							MaybeNil: true,
						})
					}
				}
				fieldAgg[name] = mergeFixedShapeField(fieldAgg[name],
					classifyReturnedField(fn, instrByID, fact, name))
			}
		}
	}
	if !seenReturn {
		return FixedShapeTableFact{}, false
	}
	if seenEmpty {
		out.ShapeID = 0
	}
	if len(fieldAgg) > 0 {
		out.FieldFacts = make(map[string]FixedShapeFieldFact, len(fieldAgg))
		for _, name := range out.FieldNames {
			out.FieldFacts[name] = fieldAgg[name].finish()
		}
	}
	return out, true
}

func withoutFieldValues(fact FixedShapeTableFact) FixedShapeTableFact {
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
	}
}

type fixedShapeFieldAccumulator struct {
	seen              bool
	kind              FixedShapeFieldKind
	paramIndex        int
	maybeNil          bool
	maybeMaterialized bool
}

func mergeFixedShapeField(acc fixedShapeFieldAccumulator, next FixedShapeFieldFact) fixedShapeFieldAccumulator {
	if !acc.seen {
		return fixedShapeFieldAccumulator{
			seen:              true,
			kind:              next.Kind,
			paramIndex:        next.ParamIndex,
			maybeNil:          next.MaybeNil,
			maybeMaterialized: next.MaybeMaterialized,
		}
	}
	if acc.kind != next.Kind || (acc.kind == FixedShapeFieldParam && acc.paramIndex != next.ParamIndex) {
		acc.kind = FixedShapeFieldUnknown
		acc.paramIndex = 0
	}
	acc.maybeNil = acc.maybeNil || next.MaybeNil
	acc.maybeMaterialized = acc.maybeMaterialized || next.MaybeMaterialized
	return acc
}

func (acc fixedShapeFieldAccumulator) finish() FixedShapeFieldFact {
	if !acc.seen {
		return FixedShapeFieldFact{Kind: FixedShapeFieldUnknown, MaybeMaterialized: true}
	}
	return FixedShapeFieldFact{
		Kind:              acc.kind,
		ParamIndex:        acc.paramIndex,
		MaybeNil:          acc.maybeNil,
		MaybeMaterialized: acc.maybeMaterialized,
	}
}

func classifyReturnedField(fn *Function, instrByID map[int]*Instr, fact FixedShapeTableFact, name string) FixedShapeFieldFact {
	valueID, ok := fact.FieldValueIDs[name]
	if !ok {
		return FixedShapeFieldFact{Kind: FixedShapeFieldNil, MaybeNil: true}
	}
	def := instrByID[valueID]
	if def == nil {
		return FixedShapeFieldFact{Kind: FixedShapeFieldUnknown, MaybeMaterialized: true}
	}
	switch def.Op {
	case OpConstNil:
		return FixedShapeFieldFact{Kind: FixedShapeFieldNil, MaybeNil: true}
	case OpLoadSlot:
		if fn != nil && fn.Proto != nil && def.Aux >= 0 && int(def.Aux) < fn.Proto.NumParams {
			return FixedShapeFieldFact{
				Kind:              FixedShapeFieldParam,
				ParamIndex:        int(def.Aux),
				MaybeMaterialized: true,
			}
		}
	}
	return FixedShapeFieldFact{Kind: FixedShapeFieldUnknown, MaybeMaterialized: true}
}

func inferLocalFixedShapeTables(fn *Function) map[int]FixedShapeTableFact {
	if fn == nil || fn.Proto == nil {
		return nil
	}
	out := make(map[int]FixedShapeTableFact)
	for _, block := range fn.Blocks {
		globalTypes := localSetGlobalTypes(block)
		allocFields := make(map[int][]string)
		allocValues := make(map[int]map[string]int)
		allocTypes := make(map[int]map[string]Type)
		allocRanges := make(map[int]map[string]intRange)
		allocFieldTableFacts := make(map[int]map[string]FixedShapeTableFact)
		allocStringValueFacts := make(map[int]*FixedShapeTableFact)
		killed := make(map[int]bool)
		for _, instr := range block.Instrs {
			switch instr.Op {
			case OpNewTable:
				allocFields[instr.ID] = nil
				allocValues[instr.ID] = make(map[string]int)
				allocTypes[instr.ID] = make(map[string]Type)
				allocRanges[instr.ID] = make(map[string]intRange)
				allocFieldTableFacts[instr.ID] = make(map[string]FixedShapeTableFact)
				out[instr.ID] = FixedShapeTableFact{}
			case OpNewFixedTable:
				fact, ok := fixedShapeFactForFixedConstructor(fn, instr, globalTypes)
				if ok {
					out[instr.ID] = fact
				}
			case OpSetField:
				if len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil {
					continue
				}
				allocID := instr.Args[0].ID
				if _, ok := allocValues[allocID]; !ok || killed[allocID] {
					continue
				}
				name := fieldNameFromAux(fn, instr.Aux)
				if name == "" || fixedShapeContainsString(allocFields[allocID], name) {
					killed[allocID] = true
					delete(out, allocID)
					continue
				}
				allocFields[allocID] = append(allocFields[allocID], name)
				allocValues[allocID][name] = instr.Args[1].ID
				if instr.Args[1].Def != nil {
					if typ := inferFixedCtorArgType(instr.Args[1].Def, globalTypes, make(map[int]bool)); typ != TypeUnknown && typ != TypeAny {
						allocTypes[allocID][name] = typ
					}
				}
				if r, ok := inferFixedCtorArgRange(fn, instr.Args[1]); ok {
					allocRanges[allocID][name] = r
				}
				if valueFact, ok := out[instr.Args[1].ID]; ok && fixedShapeTableFactHasUsableTableFact(valueFact) {
					allocFieldTableFacts[allocID][name] = withoutFieldValues(valueFact)
					allocTypes[allocID][name] = TypeTable
				}
				out[allocID] = FixedShapeTableFact{
					ShapeID:         runtime.GetShapeID(allocFields[allocID]),
					FieldNames:      append([]string(nil), allocFields[allocID]...),
					FieldValueIDs:   cloneStringIntMap(allocValues[allocID]),
					FieldTypes:      cloneStringTypeMap(allocTypes[allocID]),
					FieldRanges:     cloneStringRangeMap(allocRanges[allocID]),
					FieldTableFacts: cloneFixedShapeTableFactMap(allocFieldTableFacts[allocID]),
				}
			case OpSetTable:
				if len(instr.Args) < 3 || instr.Args[0] == nil || instr.Args[1] == nil || instr.Args[2] == nil {
					continue
				}
				allocID := instr.Args[0].ID
				if _, ok := allocValues[allocID]; !ok || killed[allocID] {
					continue
				}
				valueFact, hasValueFact := out[instr.Args[2].ID]
				if tableKeyProvenString(fn, instr, instr.Args[1]) && hasValueFact && valueFact.ShapeID != 0 && len(valueFact.FieldNames) != 0 {
					stripped := withoutFieldValues(valueFact)
					if existing := allocStringValueFacts[allocID]; existing == nil {
						allocStringValueFacts[allocID] = cloneFixedShapeTableFactPtr(stripped)
					} else if merged := mergeStringValueFacts(existing, &stripped); merged != nil {
						allocStringValueFacts[allocID] = merged
					} else {
						killed[allocID] = true
						delete(out, allocID)
						continue
					}
					fact := out[allocID]
					fact.StringValueFact = cloneFixedShapeTableFactPtrFromPtr(allocStringValueFacts[allocID])
					out[allocID] = fact
					continue
				}
				killed[allocID] = true
				delete(out, allocID)
			case OpAppend, OpSetList:
				if len(instr.Args) == 0 || instr.Args[0] == nil {
					continue
				}
				allocID := instr.Args[0].ID
				if _, ok := allocValues[allocID]; ok {
					killed[allocID] = true
					delete(out, allocID)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func fixedShapeFactForFixedConstructor(fn *Function, instr *Instr, globalTypes map[int64]Type) (FixedShapeTableFact, bool) {
	if fn == nil || fn.Proto == nil || instr == nil || instr.Op != OpNewFixedTable {
		return FixedShapeTableFact{}, false
	}
	fieldCount := int(instr.Aux2)
	if fieldCount <= 0 || len(instr.Args) != fieldCount {
		return FixedShapeTableFact{}, false
	}
	var fields []string
	if fieldCount == 2 {
		ctorIdx := int(instr.Aux)
		if ctorIdx < 0 || ctorIdx >= len(fn.Proto.TableCtors2) {
			return FixedShapeTableFact{}, false
		}
		ctor := fn.Proto.TableCtors2[ctorIdx].Runtime
		if ctor.Key1 == ctor.Key2 {
			return FixedShapeTableFact{}, false
		}
		fields = []string{ctor.Key1, ctor.Key2}
	} else {
		ctorIdx := int(instr.Aux)
		if ctorIdx < 0 || ctorIdx >= len(fn.Proto.TableCtorsN) {
			return FixedShapeTableFact{}, false
		}
		ctor := fn.Proto.TableCtorsN[ctorIdx].Runtime
		if len(ctor.Keys) != fieldCount || ctor.Shape == nil {
			return FixedShapeTableFact{}, false
		}
		fields = append([]string(nil), ctor.Keys...)
	}
	values := make(map[string]int, len(fields))
	types := make(map[string]Type, len(fields))
	ranges := make(map[string]intRange, len(fields))
	for i, field := range fields {
		values[field] = instr.Args[i].ID
		if instr.Args[i].Def != nil {
			if typ := inferFixedCtorArgType(instr.Args[i].Def, globalTypes, make(map[int]bool)); typ != TypeUnknown && typ != TypeAny {
				types[field] = typ
			}
		}
		if r, ok := inferFixedCtorArgRange(fn, instr.Args[i]); ok {
			ranges[field] = r
		}
	}
	return FixedShapeTableFact{
		ShapeID:       runtime.GetShapeID(fields),
		FieldNames:    fields,
		FieldValueIDs: values,
		FieldTypes:    types,
		FieldRanges:   ranges,
	}, true
}

func inferFixedCtorArgRange(fn *Function, v *Value) (intRange, bool) {
	if c, ok := constIntFromValue(v); ok {
		return pointRange(c), true
	}
	if v == nil {
		return intRange{}, false
	}
	if numeric := functionNumericFacts(fn); numeric != nil {
		if r, ok := numeric.IntRange(v.ID); ok && r.known {
			return r, true
		}
		if r, ok := numeric.ProfiledIntRange(v.ID); ok && r.known {
			return r, true
		}
	}
	if v.Def == nil || !v.Def.Type.isIntegerLike() {
		return intRange{}, false
	}
	return intRange{}, false
}
