// pass_fixed_shape_table_array.go: fixed-shape annotation of table array-element
// accesses (array kind/element-type/range tagging across GetTable,
// TableArrayLoad, SetTable) and forwarding of fixed-shape field reads to nil or
// to the originating call argument, plus their small access helpers.
// Pure code movement from pass_fixed_shape_table.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/vm"
)

func annotateFixedShapeArrayElementAccesses(fn *Function, numeric *NumericFacts, facts map[int]FixedShapeTableFact) {
	if fn == nil || len(facts) == 0 {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || len(instr.Args) < 2 || instr.Args[0] == nil {
				continue
			}
			factValue := instr.Args[0]
			keyArgIdx := 1
			if instr.Op == OpTableArrayLoad {
				if len(instr.Args) < 3 || instr.Args[2] == nil {
					continue
				}
				tableValue, ok := loweredTableArrayLoadTableValue(instr)
				if !ok {
					continue
				}
				factValue = tableValue
				keyArgIdx = 2
			}
			fact, ok := facts[factValue.ID]
			if !ok {
				continue
			}
			kind, ok := fixedShapeArrayElementFBKind(fact)
			if !ok || !tableKeyProvenInt(instr.Args[keyArgIdx]) {
				continue
			}
			switch instr.Op {
			case OpGetTable:
				if instr.Aux2 == 0 {
					instr.Aux2 = kind
				}
				if typ, ok := tableArrayKindElementType(kind); ok && (instr.Type == TypeAny || instr.Type == TypeUnknown) {
					instr.Type = typ
				}
				if r := fact.ArrayElementRange; r.known && numeric != nil {
					numeric.RecordProfiledIntRange(instr.ID, r)
				}
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("table value carries guarded array element kind %d", kind))
			case OpTableArrayLoad:
				if instr.Aux == 0 || instr.Aux == int64(vm.FBKindMixed) {
					instr.Aux = kind
					setLoweredTableArrayPipelineKind(instr, kind)
				}
				if typ, ok := tableArrayKindElementType(kind); ok && (instr.Type == TypeAny || instr.Type == TypeUnknown) {
					instr.Type = typ
				}
				if r := fact.ArrayElementRange; r.known && numeric != nil {
					numeric.RecordProfiledIntRange(instr.ID, r)
				}
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("lowered table value carries guarded array element kind %d", kind))
			case OpSetTable:
				if instr.Aux2 == 0 && fixedShapeSetTableValueMatchesArrayKind(instr, kind) {
					instr.Aux2 = kind
					functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
						fmt.Sprintf("table store carries guarded array element kind %d", kind))
				}
			}
		}
	}
}

func loweredTableArrayLoadTableValue(instr *Instr) (*Value, bool) {
	if instr == nil || instr.Op != OpTableArrayLoad || len(instr.Args) < 1 || instr.Args[0] == nil {
		return nil, false
	}
	data := instr.Args[0].Def
	if data == nil || data.Op != OpTableArrayData || len(data.Args) < 1 || data.Args[0] == nil {
		return nil, false
	}
	header := data.Args[0].Def
	if header == nil || header.Op != OpTableArrayHeader || len(header.Args) < 1 || header.Args[0] == nil {
		return nil, false
	}
	return header.Args[0], true
}

func setLoweredTableArrayPipelineKind(load *Instr, kind int64) {
	if load == nil || load.Op != OpTableArrayLoad || len(load.Args) < 2 || load.Args[0] == nil || load.Args[1] == nil {
		return
	}
	data := load.Args[0].Def
	length := load.Args[1].Def
	if data == nil || data.Op != OpTableArrayData || length == nil || length.Op != OpTableArrayLen {
		return
	}
	if len(data.Args) < 1 || data.Args[0] == nil || len(length.Args) < 1 || length.Args[0] == nil {
		return
	}
	header := data.Args[0].Def
	if header == nil || header.Op != OpTableArrayHeader || length.Args[0].ID != header.ID {
		return
	}
	header.Aux = kind
	length.Aux = kind
	data.Aux = kind
}

func fixedShapeArrayElementFBKind(fact FixedShapeTableFact) (int64, bool) {
	switch fact.ArrayElementType {
	case TypeInt:
		return int64(vm.FBKindInt), true
	case TypeFloat:
		return int64(vm.FBKindFloat), true
	case TypeBool:
		return int64(vm.FBKindBool), true
	case TypeAny, TypeUnknown:
		if fact.ArrayElementRange.known {
			return int64(vm.FBKindInt), true
		}
	}
	return 0, false
}

func fixedShapeSetTableValueMatchesArrayKind(instr *Instr, kind int64) bool {
	if instr == nil || len(instr.Args) < 3 || instr.Args[2] == nil || instr.Args[2].Def == nil {
		return false
	}
	switch kind {
	case int64(vm.FBKindInt):
		return callABIValueIsInt(instr.Args[2])
	case int64(vm.FBKindFloat):
		return instr.Args[2].Def.Type == TypeFloat
	case int64(vm.FBKindBool):
		return instr.Args[2].Def.Type == TypeBool
	case int64(vm.FBKindMixed):
		return true
	default:
		return false
	}
}

func forwardFixedShapeGetFields(fn *Function, facts map[int]FixedShapeTableFact) {
	if len(facts) == 0 {
		return
	}
	instrByID := fixedShapeInstrByID(fn)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpGetField || len(instr.Args) == 0 || instr.Args[0] == nil {
				continue
			}
			fact, ok := facts[instr.Args[0].ID]
			if !ok || (len(fact.FieldFacts) == 0 && len(fact.FieldNames) != 0) {
				continue
			}
			if fact.Guarded {
				continue
			}
			name := fieldNameFromAux(fn, instr.Aux)
			if name == "" {
				continue
			}
			if len(fact.FieldNames) == 0 {
				if !fixedShapeReadForwardSafe(block, instr) {
					continue
				}
				instr.Op = OpConstNil
				instr.Type = TypeNil
				instr.Args = nil
				instr.Aux = 0
				instr.Aux2 = 0
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, OpGetField,
					fmt.Sprintf("forwarded empty fixed-shape field %q to nil", name))
				continue
			}
			fieldFact, ok := fact.FieldFacts[name]
			if !ok {
				continue
			}
			switch fieldFact.Kind {
			case FixedShapeFieldNil:
				if fieldFact.MaybeMaterialized {
					continue
				}
				if !fixedShapeReadForwardSafe(block, instr) {
					continue
				}
				instr.Op = OpConstNil
				instr.Type = TypeNil
				instr.Args = nil
				instr.Aux = 0
				instr.Aux2 = 0
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, OpGetField,
					fmt.Sprintf("forwarded fixed-shape field %q to nil", name))
			case FixedShapeFieldParam:
				if fieldFact.MaybeNil || fieldFact.ParamIndex < 0 {
					continue
				}
				if !fixedShapeReadForwardSafe(block, instr) {
					continue
				}
				call := instrByID[instr.Args[0].ID]
				if call == nil || call.Op != OpCall || len(call.Args) <= 1+fieldFact.ParamIndex {
					continue
				}
				actual := call.Args[1+fieldFact.ParamIndex]
				if actual == nil || actual.Def == nil {
					continue
				}
				replaceAllUses(fn, instr.ID, actual.Def)
				instr.Op = OpNop
				instr.Args = nil
				instr.Aux = 0
				instr.Aux2 = 0
				functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, OpGetField,
					fmt.Sprintf("forwarded fixed-shape field %q to call arg %d", name, fieldFact.ParamIndex))
			}
		}
	}
}

func fixedShapeReadForwardSafe(block *Block, get *Instr) bool {
	if block == nil || get == nil || get.Op != OpGetField || len(get.Args) == 0 || get.Args[0] == nil {
		return false
	}
	objID := get.Args[0].ID
	def := get.Args[0].Def
	if def == nil || def.Op != OpCall || def.Block != block {
		return false
	}
	defIdx := -1
	getIdx := -1
	for i, instr := range block.Instrs {
		if instr == def {
			defIdx = i
		}
		if instr == get {
			getIdx = i
		}
	}
	if defIdx < 0 || getIdx <= defIdx {
		return false
	}
	for _, instr := range block.Instrs[defIdx+1 : getIdx] {
		if instr == nil {
			continue
		}
		for argIdx, arg := range instr.Args {
			if arg == nil || arg.ID != objID {
				continue
			}
			switch instr.Op {
			case OpGetField:
				if argIdx == 0 {
					continue
				}
			case OpStoreSlot:
				continue
			}
			return false
		}
	}
	return true
}

func fixedShapeInstrByID(fn *Function) map[int]*Instr {
	out := make(map[int]*Instr)
	if fn == nil {
		return out
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			out[instr.ID] = instr
		}
	}
	return out
}
