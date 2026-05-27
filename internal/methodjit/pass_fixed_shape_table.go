package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/vm"
)

// FixedShapeTableFact describes a table SSA value whose hidden-class shape is
// statically known. FieldValueIDs is populated only for constructors in the
// current Function; call-return facts expose only stable FieldFacts that can be
// interpreted in the caller.
type FixedShapeTableFact struct {
	ShapeID           uint32
	ObservationCount  uint32
	FieldNames        []string
	FieldValueIDs     map[string]int
	FieldFacts        map[string]FixedShapeFieldFact
	FieldTypes        map[string]Type
	FieldRanges       map[string]intRange
	FieldLenRanges    map[string]intRange
	FieldVMProtos     map[string]*vm.FuncProto
	FieldVMClosures   map[string]uintptr
	FieldTableFacts   map[string]FixedShapeTableFact
	StringValueFact   *FixedShapeTableFact
	ArrayElementType  Type
	ArrayElementRange intRange
	Guarded           bool
	EntryGuarded      bool
}

type FieldPolyShapeCase struct {
	ShapeID      uint32
	Count        uint32
	FieldIdx     int
	Type         Type
	VMProto      *vm.FuncProto
	VMClosure    uintptr
	ReceiverFact FixedShapeTableFact
}

type FixedShapeFieldKind uint8

const (
	FixedShapeFieldUnknown FixedShapeFieldKind = iota
	FixedShapeFieldNil
	FixedShapeFieldParam
)

// FixedShapeFieldFact is the caller-safe state for one fixed-shape field.
// MaybeNil covers empty-shape return paths where a missing field reads as nil.
// MaybeMaterialized marks paths where the field value still comes from a real
// runtime value, so consumers must not replace the read with nil.
type FixedShapeFieldFact struct {
	Kind              FixedShapeFieldKind
	ParamIndex        int
	MaybeNil          bool
	MaybeMaterialized bool
}

// FixedTableConstructorFact describes a bytecode-level fixed-field table
// constructor that is still represented as OpNewTable plus OpSetField stores in
// early IR. Exactly one constructor index is non-negative.
type FixedTableConstructorFact struct {
	Ctor2Index int
	CtorNIndex int
	FieldNames []string
}

func (f FixedShapeTableFact) fieldIndex(name string) (int, bool) {
	for i, field := range f.FieldNames {
		if field == name {
			return i, true
		}
	}
	return -1, false
}

func (f FixedShapeTableFact) sameShape(other FixedShapeTableFact) bool {
	if len(f.FieldNames) != len(other.FieldNames) {
		return false
	}
	for i := range f.FieldNames {
		if f.FieldNames[i] != other.FieldNames[i] {
			return false
		}
	}
	return true
}

// FixedShapeTableFactsConfig supplies facts that are safe to consume in the
// current function. ArgFacts are guarded callsite facts for callee parameters.
// EntryGuardedArgs asks codegen to validate those shapes before the optimized
// body so the guarded facts can be consumed as callee-local shape facts.
type FixedShapeTableFactsConfig struct {
	Globals               map[string]*vm.FuncProto
	ArgFacts              map[int]FixedShapeTableFact
	ArgPolyFacts          map[int][]FixedShapeTableFact
	ArrayElementArgFacts  map[int]FixedShapeTableFact
	ArrayElementPolyFacts map[int][]FixedShapeTableFact
	EntryGuardedArgs      bool
}

// FixedShapeTableFactsPass records fixed-shape table facts and uses
// interprocedural return facts from stable global callees to prefill GetField
// shape-cache metadata. It deliberately leaves runtime shape guards intact.
func FixedShapeTableFactsPass(globals map[string]*vm.FuncProto) PassFunc {
	return FixedShapeTableFactsPassWith(FixedShapeTableFactsConfig{Globals: globals})
}

// FixedShapeTableFactsPassWith is the configurable fixed-shape pass entry.
func FixedShapeTableFactsPassWith(config FixedShapeTableFactsConfig) PassFunc {
	return func(fn *Function) (*Function, error) {
		if fn == nil {
			return fixedShapeTableFactsPass(fn, config, nil, nil)
		}
		fn.ensureAnalysis()
		return fixedShapeTableFactsPass(fn, config, fn.Analysis.TableShapeFacts(), fn.Analysis.NumericFacts())
	}
}

func FixedShapeTableFactsPassCtx(config FixedShapeTableFactsConfig) CtxPassFunc {
	return func(ctx *PassContext) (*Function, error) {
		fn := ctx.Func()
		if fn != nil {
			fn.ensureAnalysis()
		}
		return fixedShapeTableFactsPass(fn, config, ctx.TableShape(), ctx.Numeric())
	}
}

func fixedShapeTableFactsPass(fn *Function, config FixedShapeTableFactsConfig, tableShapes *TableShapeFacts, numeric *NumericFacts) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	facts := inferLocalFixedShapeTables(fn)
	if len(facts) == 0 {
		facts = make(map[int]FixedShapeTableFact)
	}
	seedGuardedFixedShapeArgFacts(fn, tableShapes, facts, config.ArgFacts)
	seedGuardedFixedShapeArrayElementArgFacts(fn, facts, config.ArrayElementArgFacts)
	seedGuardedPolyShapeArgFacts(fn, tableShapes, config.ArgPolyFacts)
	seedGuardedPolyShapeArrayElementArgFacts(fn, tableShapes, facts, config.ArrayElementPolyFacts)
	seedProfiledDynamicTableValueFacts(fn, facts)
	if config.EntryGuardedArgs && tableShapes != nil {
		markEntryGuardedFixedShapeArgFacts(fn, tableShapes, facts, tableShapes.FixedShapeArgFactMap())
	}
	propagateFixedShapePhiFacts(fn, facts)

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpCall {
				continue
			}
			_, callee := resolveCallee(instr, fn, InlineConfig{Globals: config.Globals})
			if callee == nil {
				continue
			}
			fact, ok := AnalyzeFixedShapeReturnFact(callee)
			if !ok {
				continue
			}
			facts[instr.ID] = fact
			if instr.Type == TypeAny || instr.Type == TypeUnknown {
				instr.Type = TypeTable
			}
			functionRemarks(fn).Add("FixedShapeTableFacts", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("call result carries fixed table shape %v", fact.FieldNames))
		}
	}
	seedLocalStringMapValueFacts(fn, facts)
	seedLocalFieldTableFacts(fn, facts)
	arrayElementFacts := inferLocalArrayElementTableFacts(fn, facts)
	seedLocalArrayElementTableFacts(fn, facts, arrayElementFacts)

	if tableShapes == nil {
		return fn, nil
	}
	if len(facts) == 0 && tableShapes.FieldPolyShapeFactCount() == 0 {
		return fn, nil
	}
	tableShapes.SetFixedShapeTables(facts)
	annotateFixedShapeStringValueAccesses(fn, tableShapes, facts)
	propagateFixedShapePhiFacts(fn, facts)
	annotateFixedShapeGetFields(fn, tableShapes, numeric, facts)
	annotateFixedShapeStringValueAccesses(fn, tableShapes, facts)
	propagateFixedShapePhiFacts(fn, facts)
	annotateFixedShapeGetFields(fn, tableShapes, numeric, facts)
	annotateFixedShapeSetFields(fn, facts)
	propagateShapeCacheFromSetFieldToGetField(fn, facts)
	annotateFixedShapeArrayElementAccesses(fn, numeric, facts)
	forwardFixedShapeGetFields(fn, facts)
	return fn, nil
}

func fixedShapeContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneStringIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringTypeMap(in map[string]Type) map[string]Type {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]Type, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func recordProfiledLenRange(numeric *NumericFacts, valueID int, r intRange) {
	if numeric == nil {
		return
	}
	numeric.RecordProfiledLenRange(valueID, r)
}

func cloneStringRangeMap(in map[string]intRange) map[string]intRange {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]intRange, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringProtoMap(in map[string]*vm.FuncProto) map[string]*vm.FuncProto {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*vm.FuncProto, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringUintptrMap(in map[string]uintptr) map[string]uintptr {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]uintptr, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneFixedShapeTableFactMap(in map[string]FixedShapeTableFact) map[string]FixedShapeTableFact {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]FixedShapeTableFact, len(in))
	for k, v := range in {
		out[k] = cloneFixedShapeTableFact(v)
	}
	return out
}

func cloneFixedShapeTableFactPtr(fact FixedShapeTableFact) *FixedShapeTableFact {
	cloned := cloneFixedShapeTableFact(fact)
	return &cloned
}

func cloneFixedShapeTableFactPtrFromPtr(fact *FixedShapeTableFact) *FixedShapeTableFact {
	if fact == nil {
		return nil
	}
	return cloneFixedShapeTableFactPtr(*fact)
}

func cloneFixedShapeTableFact(fact FixedShapeTableFact) FixedShapeTableFact {
	fact.FieldNames = append([]string(nil), fact.FieldNames...)
	fact.FieldValueIDs = cloneStringIntMap(fact.FieldValueIDs)
	fact.FieldFacts = cloneFixedShapeFieldFactMap(fact.FieldFacts)
	fact.FieldTypes = cloneStringTypeMap(fact.FieldTypes)
	fact.FieldRanges = cloneStringRangeMap(fact.FieldRanges)
	fact.FieldLenRanges = cloneStringRangeMap(fact.FieldLenRanges)
	fact.FieldVMProtos = cloneStringProtoMap(fact.FieldVMProtos)
	fact.FieldVMClosures = cloneStringUintptrMap(fact.FieldVMClosures)
	fact.FieldTableFacts = cloneFixedShapeTableFactMap(fact.FieldTableFacts)
	fact.StringValueFact = cloneFixedShapeTableFactPtrFromPtr(fact.StringValueFact)
	return fact
}

func cloneFixedShapeFieldFactMap(in map[string]FixedShapeFieldFact) map[string]FixedShapeFieldFact {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]FixedShapeFieldFact, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
