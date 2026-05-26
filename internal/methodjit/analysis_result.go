// Package methodjit implements a V8 Maglev-style method JIT compiler.
package methodjit

import (
	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// AnalysisResult contains all analysis maps produced and consumed by optimization passes.
// This struct extracts analysis state from the Function god object.
type AnalysisResult struct {
	// Numeric groups integer range and arithmetic safety facts. It is the single
	// source of truth for numeric analysis; access goes through its accessors.
	Numeric *NumericFacts

	// ShapeFieldTypeElidedLoads marks fixed-shape field loads whose result type
	// is guarded once by shape field type epochs. Codegen may skip the per-load
	// NaN-box tag check for these loads.
	ShapeFieldTypeElidedLoads map[int]bool

	// Call groups call-oriented analysis facts. It is the single source of truth
	// for call analysis; access goes through its accessors.
	Call *CallFacts

	// Speculation groups Tier 2 speculation facts. It is the single source of
	// truth for speculation analysis; access goes through its accessors.
	Speculation *SpeculationFacts

	// TableShape groups table/field shape analysis facts. The map fields below
	// remain for compatibility and are kept pointed at this domain's maps by
	// constructors, Initialize, and domain mutators.
	TableShape *TableShapeFacts

	// LoopSpecialization groups loop-specialization and table-array bound facts.
	// It is the single source of truth for loop-specialization analysis; access
	// goes through its accessors.
	LoopSpecialization *LoopSpecializationFacts

	// FixedShapeTables records SSA table values whose field layout is known
	// without consulting the runtime field cache. The initial producer is a
	// static table constructor or a call to a function whose every return path
	// creates the same fixed-shape table. Consumers may use this as a guarded
	// shape fact; it is not an aliasing proof and must not remove runtime shape
	// checks by itself.
	FixedShapeTables map[int]FixedShapeTableFact

	// FieldPolyShapeFacts records small guarded polymorphic field caches keyed
	// by OpGetField instruction ID. Each case maps receiver shapeID to the
	// field index for that static field name.
	FieldPolyShapeFacts map[int][]FieldPolyShapeCase

	// FieldPolyShapeReceivers records table SSA values known to carry guarded
	// polymorphic shape facts. Monomorphic fixed-shape lowering must not turn
	// reads from these receivers into a single-shape FieldSvals path before
	// FieldPolyShapeFacts can be attached to each field access.
	FieldPolyShapeReceivers map[int]bool

	// FieldPolyShapeCatalog records the receiver facts behind polymorphic
	// field caches keyed by shape ID. Unlike FieldPolyShapeFacts, it is not
	// tied to a specific IR value ID, so later split/inline/lowering passes can
	// recover nested table facts from an OpFieldSvals(shape) after the original
	// field-cache instruction has been cloned or removed.
	FieldPolyShapeCatalog map[uint32]FixedShapeTableFact

	// FieldCallPolyLenFusions records same-block guarded field-call/field-len
	// pairs keyed by OpFieldCallFloor instruction ID. When a field call's shape
	// dispatch succeeds and the callee is proven not to mutate the later length
	// field, codegen can materialize the later OpFieldPolyLen result directly
	// from the already-matched shape case and skip a second shape dispatch.
	FieldCallPolyLenFusions map[int][]FieldCallPolyLenFusion

	// FixedShapeArgFacts records guarded fixed-shape facts keyed by parameter
	// index. These facts come from callsites, not from the callee body, so
	// consumers may use them only through runtime guards such as field-cache
	// shape checks.
	FixedShapeArgFacts map[int]FixedShapeTableFact

	// FixedTableConstructors records OpNewTable values that came from a
	// bytecode-level fixed string-field table constructor. The graph builder
	// keeps the constructor expanded as NewTable+SetField so scalar replacement
	// can still see ordinary field stores; late lowering may combine surviving
	// constructors into OpNewFixedTable for native codegen.
	FixedTableConstructors map[int]FixedTableConstructorFact

	// FixedRecordNewTableSites records OpNewFixedTable values that remain local
	// to fixed-record-aware field reads. Constructors that escape through calls,
	// stores, returns, or generic table operations must materialize ordinary
	// tables so downstream code observes normal table semantics.
	FixedRecordNewTableSites map[int]bool

	// FixedShapeEntryGuards records parameter shape guards that codegen must
	// execute before entering the optimized body. Once these guards have run,
	// matching FixedShapeArgFacts are safe as callee-local shape facts.
	FixedShapeEntryGuards map[int]FixedShapeTableFact

	// Globals maps global function names to their protos for the IR interpreter
	// to resolve residual cross-function calls, such as calls left after bounded
	// recursive inlining. Nil is a sentinel meaning the inline pass may still
	// install its config.Globals; an empty non-nil map means globals were
	// intentionally provided and no fallback install should occur. Production
	// code paths never consult this field; it exists only as a hook for the IR
	// correctness oracle.
	Globals map[string]*vm.FuncProto

	// NumericGlobalValues and GlobalArrayElementFacts are stable cross-proto
	// facts supplied by the Tier 2 manager for ABI analysis. They are hints:
	// missing facts disable specialized entries, while emitted guards still
	// protect any optimized path that consumes present facts.
	NumericGlobalValues     map[string]runtime.Value
	GlobalArrayElementFacts map[string]FixedShapeTableFact
}

// NumericFacts groups integer range and arithmetic safety facts produced and
// consumed by numeric optimization passes.
type NumericFacts struct {
	owner *AnalysisResult

	// Int48Safe is the set of integer arithmetic SSA value IDs whose runtime
	// result is provably within the int48 signed range.
	Int48Safe map[int]bool

	// IntModNonZeroDivisor is the set of ModInt SSA value IDs whose divisor
	// range excludes zero.
	IntModNonZeroDivisor map[int]bool

	// IntModNoSignAdjust is the set of ModInt SSA value IDs whose operand signs
	// prove that ARM64 SDIV/MSUB already matches Lua modulo semantics.
	IntModNoSignAdjust map[int]bool

	// IntRanges records integer range facts computed by range analysis.
	IntRanges map[int]intRange

	// ProfiledIntRanges records guarded integer range facts from runtime
	// feedback, keyed by SSA value ID.
	ProfiledIntRanges map[int]intRange

	// ProfiledLenRanges records guarded len() result ranges keyed by the value
	// whose length is being read.
	ProfiledLenRanges map[int]intRange

	// IntNonNegative is the set of integer SSA value IDs whose runtime result is
	// provably >= 0.
	IntNonNegative map[int]bool
}

func NewNumericFacts() *NumericFacts {
	n := &NumericFacts{}
	n.Initialize()
	return n
}

func (n *NumericFacts) Initialize() {
	if n.Int48Safe == nil {
		n.Int48Safe = make(map[int]bool)
	}
	if n.IntModNonZeroDivisor == nil {
		n.IntModNonZeroDivisor = make(map[int]bool)
	}
	if n.IntModNoSignAdjust == nil {
		n.IntModNoSignAdjust = make(map[int]bool)
	}
	if n.IntRanges == nil {
		n.IntRanges = make(map[int]intRange)
	}
	if n.ProfiledIntRanges == nil {
		n.ProfiledIntRanges = make(map[int]intRange)
	}
	if n.ProfiledLenRanges == nil {
		n.ProfiledLenRanges = make(map[int]intRange)
	}
	if n.IntNonNegative == nil {
		n.IntNonNegative = make(map[int]bool)
	}
}

func (n *NumericFacts) SetInt48Safe(facts map[int]bool) {
	if n == nil {
		return
	}
	n.Int48Safe = facts
	n.bindOwner()
}

func (n *NumericFacts) IsInt48Safe(id int) bool {
	return n != nil && n.Int48Safe != nil && n.Int48Safe[id]
}

func (n *NumericFacts) Int48SafeCount() int {
	if n == nil {
		return 0
	}
	return len(n.Int48Safe)
}

func (n *NumericFacts) SetIntModNonZeroDivisor(facts map[int]bool) {
	if n == nil {
		return
	}
	n.IntModNonZeroDivisor = facts
	n.bindOwner()
}

func (n *NumericFacts) IsIntModNonZeroDivisor(id int) bool {
	return n != nil && n.IntModNonZeroDivisor != nil && n.IntModNonZeroDivisor[id]
}

func (n *NumericFacts) SetIntModNoSignAdjust(facts map[int]bool) {
	if n == nil {
		return
	}
	n.IntModNoSignAdjust = facts
	n.bindOwner()
}

func (n *NumericFacts) IsIntModNoSignAdjust(id int) bool {
	return n != nil && n.IntModNoSignAdjust != nil && n.IntModNoSignAdjust[id]
}

func (n *NumericFacts) SetIntRanges(facts map[int]intRange) {
	if n == nil {
		return
	}
	n.IntRanges = facts
	n.bindOwner()
}

func (n *NumericFacts) IntRange(id int) (intRange, bool) {
	if n == nil || n.IntRanges == nil {
		return intRange{}, false
	}
	r, ok := n.IntRanges[id]
	return r, ok
}

func (n *NumericFacts) IntRangeMap() map[int]intRange {
	if n == nil {
		return nil
	}
	return n.IntRanges
}

func (n *NumericFacts) SetProfiledIntRanges(facts map[int]intRange) {
	if n == nil {
		return
	}
	n.ProfiledIntRanges = facts
	n.bindOwner()
}

func (n *NumericFacts) ProfiledIntRange(id int) (intRange, bool) {
	if n == nil || n.ProfiledIntRanges == nil {
		return intRange{}, false
	}
	r, ok := n.ProfiledIntRanges[id]
	return r, ok
}

func (n *NumericFacts) ProfiledIntRangeMap() map[int]intRange {
	if n == nil {
		return nil
	}
	return n.ProfiledIntRanges
}

func (n *NumericFacts) RecordProfiledIntRange(id int, r intRange) {
	if n == nil || !r.known {
		return
	}
	if n.ProfiledIntRanges == nil {
		n.ProfiledIntRanges = make(map[int]intRange)
	}
	n.ProfiledIntRanges[id] = r
	n.bindOwner()
}

func (n *NumericFacts) DeleteProfiledIntRange(id int) {
	if n == nil || n.ProfiledIntRanges == nil {
		return
	}
	delete(n.ProfiledIntRanges, id)
	n.bindOwner()
}

func (n *NumericFacts) SetProfiledLenRanges(facts map[int]intRange) {
	if n == nil {
		return
	}
	n.ProfiledLenRanges = facts
	n.bindOwner()
}

func (n *NumericFacts) ProfiledLenRange(id int) (intRange, bool) {
	if n == nil || n.ProfiledLenRanges == nil {
		return intRange{}, false
	}
	r, ok := n.ProfiledLenRanges[id]
	return r, ok
}

func (n *NumericFacts) ProfiledLenRangeMap() map[int]intRange {
	if n == nil {
		return nil
	}
	return n.ProfiledLenRanges
}

func (n *NumericFacts) RecordProfiledLenRange(id int, r intRange) {
	if n == nil || id == 0 || !r.known {
		return
	}
	if n.ProfiledLenRanges == nil {
		n.ProfiledLenRanges = make(map[int]intRange)
	}
	n.ProfiledLenRanges[id] = r
	n.bindOwner()
}

func (n *NumericFacts) DeleteProfiledLenRange(id int) {
	if n == nil || n.ProfiledLenRanges == nil {
		return
	}
	delete(n.ProfiledLenRanges, id)
	n.bindOwner()
}

func (n *NumericFacts) SetIntNonNegative(facts map[int]bool) {
	if n == nil {
		return
	}
	n.IntNonNegative = facts
	n.bindOwner()
}

func (n *NumericFacts) RecordIntNonNegative(id int) {
	if n == nil {
		return
	}
	if n.IntNonNegative == nil {
		n.IntNonNegative = make(map[int]bool)
	}
	n.IntNonNegative[id] = true
	n.bindOwner()
}

func (n *NumericFacts) IsIntNonNegative(id int) bool {
	return n != nil && n.IntNonNegative != nil && n.IntNonNegative[id]
}

func (n *NumericFacts) SetComputedRanges(safe map[int]bool, ranges map[int]intRange, nonNegative map[int]bool) {
	if n == nil {
		return
	}
	n.Int48Safe = safe
	n.IntRanges = ranges
	n.IntNonNegative = nonNegative
	n.bindOwner()
}

func (n *NumericFacts) SetModuloFacts(nonZeroDivisor map[int]bool, noSignAdjust map[int]bool) {
	if n == nil {
		return
	}
	n.IntModNonZeroDivisor = nonZeroDivisor
	n.IntModNoSignAdjust = noSignAdjust
	n.bindOwner()
}

func (n *NumericFacts) bindOwner() {}

func (a *AnalysisResult) NumericFacts() *NumericFacts {
	if a == nil {
		return nil
	}
	if a.Numeric == nil {
		a.Numeric = &NumericFacts{}
		a.Numeric.Initialize()
	}
	a.Numeric.owner = a
	return a.Numeric
}

func functionNumericFacts(fn *Function) *NumericFacts {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return fn.Analysis.NumericFacts()
}

func (a *AnalysisResult) initializeNumericFacts() {
	if a == nil {
		return
	}
	a.NumericFacts().Initialize()
}

// CallFacts groups analysis facts produced and consumed by call-specialization passes.
type CallFacts struct {
	owner *AnalysisResult

	// CallABIs records stable callsite ABI facts keyed by OpCall instruction
	// ID. A descriptor is required before codegen may use a specialized
	// cross-proto raw-int call path; OpCall.Type alone is not authoritative.
	CallABIs map[int]CallABIDescriptor

	// GuardedConstCallFolds records guarded call-site guarded
	// constants keyed by OpCall instruction ID.
	GuardedConstCallFolds map[int]GuardedConstCallFoldFact

	// CallSiteNoResultRuntimeSpecializations records stable structural
	// no-result call-site runtime specializations keyed by OpCall instruction
	// ID. Codegen routes them through a precise op-exit rather than the generic
	// CallExit path.
	CallSiteNoResultRuntimeSpecializations map[int]bool

	// CallSiteNoResultRuntimeSpecializationBatches records loop-tail no-result
	// call-site runtime specialization sites that can safely batch future
	// complete loop iterations after the current iteration's final call has run.
	CallSiteNoResultRuntimeSpecializationBatches map[int]CallSiteNoResultRuntimeSpecializationBatchFact
}

func NewCallFacts() *CallFacts {
	c := &CallFacts{}
	c.Initialize()
	return c
}

func (c *CallFacts) Initialize() {
	if c.CallABIs == nil {
		c.CallABIs = make(map[int]CallABIDescriptor)
	}
	if c.GuardedConstCallFolds == nil {
		c.GuardedConstCallFolds = make(map[int]GuardedConstCallFoldFact)
	}
	if c.CallSiteNoResultRuntimeSpecializations == nil {
		c.CallSiteNoResultRuntimeSpecializations = make(map[int]bool)
	}
	if c.CallSiteNoResultRuntimeSpecializationBatches == nil {
		c.CallSiteNoResultRuntimeSpecializationBatches = make(map[int]CallSiteNoResultRuntimeSpecializationBatchFact)
	}
}

func (c *CallFacts) SetCallABIs(facts map[int]CallABIDescriptor) {
	c.CallABIs = facts
	c.bindOwner()
}

func (c *CallFacts) CallABI(id int) (CallABIDescriptor, bool) {
	if c == nil || c.CallABIs == nil {
		return CallABIDescriptor{}, false
	}
	desc, ok := c.CallABIs[id]
	return desc, ok
}

func (c *CallFacts) CallABICount() int {
	if c == nil {
		return 0
	}
	return len(c.CallABIs)
}

func (c *CallFacts) ForEachCallABI(visit func(int, CallABIDescriptor) bool) {
	if c == nil || visit == nil {
		return
	}
	for id, desc := range c.CallABIs {
		if !visit(id, desc) {
			return
		}
	}
}

func (c *CallFacts) SetGuardedConstCallFolds(facts map[int]GuardedConstCallFoldFact) {
	c.GuardedConstCallFolds = facts
	c.bindOwner()
}

func (c *CallFacts) GuardedConstCallFold(id int) (GuardedConstCallFoldFact, bool) {
	if c == nil || c.GuardedConstCallFolds == nil {
		return GuardedConstCallFoldFact{}, false
	}
	fact, ok := c.GuardedConstCallFolds[id]
	return fact, ok
}

func (c *CallFacts) GuardedConstCallFoldCount() int {
	if c == nil {
		return 0
	}
	return len(c.GuardedConstCallFolds)
}

func (c *CallFacts) ForEachGuardedConstCallFold(visit func(int, GuardedConstCallFoldFact) bool) {
	if c == nil || visit == nil {
		return
	}
	for id, fact := range c.GuardedConstCallFolds {
		if !visit(id, fact) {
			return
		}
	}
}

func (c *CallFacts) SetCallSiteNoResultRuntimeSpecializations(facts map[int]bool) {
	c.CallSiteNoResultRuntimeSpecializations = facts
	c.bindOwner()
}

func (c *CallFacts) CallSiteNoResultRuntimeSpecialization(id int) bool {
	return c != nil && c.CallSiteNoResultRuntimeSpecializations != nil && c.CallSiteNoResultRuntimeSpecializations[id]
}

func (c *CallFacts) SetCallSiteNoResultRuntimeSpecializationBatches(facts map[int]CallSiteNoResultRuntimeSpecializationBatchFact) {
	c.CallSiteNoResultRuntimeSpecializationBatches = facts
	c.bindOwner()
}

func (c *CallFacts) CallSiteNoResultRuntimeSpecializationBatch(id int) (CallSiteNoResultRuntimeSpecializationBatchFact, bool) {
	if c == nil || c.CallSiteNoResultRuntimeSpecializationBatches == nil {
		return CallSiteNoResultRuntimeSpecializationBatchFact{}, false
	}
	fact, ok := c.CallSiteNoResultRuntimeSpecializationBatches[id]
	return fact, ok
}

func (c *CallFacts) CallSiteNoResultRuntimeSpecializationBatchMap() map[int]CallSiteNoResultRuntimeSpecializationBatchFact {
	if c == nil {
		return nil
	}
	return c.CallSiteNoResultRuntimeSpecializationBatches
}

func (c *CallFacts) bindOwner() {}

func (a *AnalysisResult) CallFacts() *CallFacts {
	if a == nil {
		return nil
	}
	if a.Call == nil {
		a.Call = &CallFacts{}
		a.Call.Initialize()
	}
	a.Call.owner = a
	return a.Call
}

func functionCallFacts(fn *Function) *CallFacts {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return fn.Analysis.CallFacts()
}

func (a *AnalysisResult) initializeCallFacts() {
	if a == nil {
		return
	}
	a.CallFacts().Initialize()
}

// LoopSpecializationFacts groups analysis facts produced and consumed by loop-specialization and
// table-array bound passes.
type LoopSpecializationFacts struct {
	owner *AnalysisResult

	// TableArrayUpperBoundSafe is the set of table-array access instruction IDs
	// whose key < len check is already guaranteed by an enclosing loop-region
	// fact.
	TableArrayUpperBoundSafe map[int]bool

	// TableArrayLowerBoundSafe is the set of table-array access instruction IDs
	// whose key >= 0 check is already guaranteed by loop-region induction facts.
	TableArrayLowerBoundSafe map[int]bool

	// LoopTableArrayFacts records the table/len/data/key contract behind each
	// TableArrayUpperBoundSafe access.
	LoopTableArrayFacts map[int]LoopTableArrayFact

	// RecordArrayLoopSpecializations records generated loop-body dataflow graphs keyed
	// by OpRecordArrayLoopSpecialization instruction ID.
	RecordArrayLoopSpecializations map[int]RecordArrayLoopSpecializationSpec

	// TableArrayDataPtrs records typed table-array data pointer SSA values. The
	// key is an OpTableArrayData value ID; consumers can resolve it as a raw
	// backing-array pointer only while the matching header guard remains valid.
	TableArrayDataPtrs map[int]TableArrayDataPtrFact
}

func NewLoopSpecializationFacts() *LoopSpecializationFacts {
	k := &LoopSpecializationFacts{}
	k.Initialize()
	return k
}

func (k *LoopSpecializationFacts) Initialize() {
	if k.TableArrayUpperBoundSafe == nil {
		k.TableArrayUpperBoundSafe = make(map[int]bool)
	}
	if k.TableArrayLowerBoundSafe == nil {
		k.TableArrayLowerBoundSafe = make(map[int]bool)
	}
	if k.LoopTableArrayFacts == nil {
		k.LoopTableArrayFacts = make(map[int]LoopTableArrayFact)
	}
	if k.RecordArrayLoopSpecializations == nil {
		k.RecordArrayLoopSpecializations = make(map[int]RecordArrayLoopSpecializationSpec)
	}
	if k.TableArrayDataPtrs == nil {
		k.TableArrayDataPtrs = make(map[int]TableArrayDataPtrFact)
	}
}

func (k *LoopSpecializationFacts) SetTableArrayUpperBoundSafe(facts map[int]bool) {
	k.TableArrayUpperBoundSafe = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) TableArrayUpperBoundIsSafe(id int) bool {
	return k != nil && k.TableArrayUpperBoundSafe != nil && k.TableArrayUpperBoundSafe[id]
}

func (k *LoopSpecializationFacts) TableArrayUpperBoundSafeMap() map[int]bool {
	if k == nil {
		return nil
	}
	return k.TableArrayUpperBoundSafe
}

func (k *LoopSpecializationFacts) RecordTableArrayUpperBoundSafe(id int) {
	if k == nil {
		return
	}
	if k.TableArrayUpperBoundSafe == nil {
		k.TableArrayUpperBoundSafe = make(map[int]bool)
	}
	k.TableArrayUpperBoundSafe[id] = true
	k.bindOwner()
}

func (k *LoopSpecializationFacts) SetTableArrayLowerBoundSafe(facts map[int]bool) {
	k.TableArrayLowerBoundSafe = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) TableArrayLowerBoundIsSafe(id int) bool {
	return k != nil && k.TableArrayLowerBoundSafe != nil && k.TableArrayLowerBoundSafe[id]
}

func (k *LoopSpecializationFacts) TableArrayLowerBoundSafeMap() map[int]bool {
	if k == nil {
		return nil
	}
	return k.TableArrayLowerBoundSafe
}

func (k *LoopSpecializationFacts) RecordTableArrayLowerBoundSafe(id int) {
	if k == nil {
		return
	}
	if k.TableArrayLowerBoundSafe == nil {
		k.TableArrayLowerBoundSafe = make(map[int]bool)
	}
	k.TableArrayLowerBoundSafe[id] = true
	k.bindOwner()
}

func (k *LoopSpecializationFacts) SetLoopTableArrayFacts(facts map[int]LoopTableArrayFact) {
	k.LoopTableArrayFacts = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) LoopTableArrayFact(id int) (LoopTableArrayFact, bool) {
	if k == nil || k.LoopTableArrayFacts == nil {
		return LoopTableArrayFact{}, false
	}
	fact, ok := k.LoopTableArrayFacts[id]
	return fact, ok
}

func (k *LoopSpecializationFacts) RecordLoopTableArrayFact(id int, fact LoopTableArrayFact) {
	if k == nil {
		return
	}
	if k.LoopTableArrayFacts == nil {
		k.LoopTableArrayFacts = make(map[int]LoopTableArrayFact)
	}
	k.LoopTableArrayFacts[id] = fact
	k.bindOwner()
}

func (k *LoopSpecializationFacts) SetRecordArrayLoopSpecializations(facts map[int]RecordArrayLoopSpecializationSpec) {
	k.RecordArrayLoopSpecializations = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) RecordArrayLoopSpecialization(id int) (RecordArrayLoopSpecializationSpec, bool) {
	if k == nil || k.RecordArrayLoopSpecializations == nil {
		return RecordArrayLoopSpecializationSpec{}, false
	}
	spec, ok := k.RecordArrayLoopSpecializations[id]
	return spec, ok
}

func (k *LoopSpecializationFacts) SetRecordArrayLoopSpecialization(id int, spec RecordArrayLoopSpecializationSpec) {
	if k == nil {
		return
	}
	if k.RecordArrayLoopSpecializations == nil {
		k.RecordArrayLoopSpecializations = make(map[int]RecordArrayLoopSpecializationSpec)
	}
	k.RecordArrayLoopSpecializations[id] = spec
	k.bindOwner()
}

func (k *LoopSpecializationFacts) bindOwner() {}

func (a *AnalysisResult) LoopSpecializationFacts() *LoopSpecializationFacts {
	if a == nil {
		return nil
	}
	if a.LoopSpecialization == nil {
		a.LoopSpecialization = &LoopSpecializationFacts{}
		a.LoopSpecialization.Initialize()
	}
	a.LoopSpecialization.owner = a
	return a.LoopSpecialization
}

func functionLoopSpecializationFacts(fn *Function) *LoopSpecializationFacts {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return fn.Analysis.LoopSpecializationFacts()
}

func (a *AnalysisResult) initializeLoopSpecializationFacts() {
	if a == nil {
		return
	}
	a.LoopSpecializationFacts().Initialize()
}

// SpeculationFacts groups analysis facts produced and consumed by Tier 2
// speculation and guard-specialization paths.
type SpeculationFacts struct {
	owner *AnalysisResult

	// SpecDependencyProtos records other protos whose runtime feedback or native
	// entry publication can change this function's optimized shape. It covers
	// inlined callees and guarded polymorphic call targets.
	SpecDependencyProtos map[*vm.FuncProto]bool

	// SuppressedSpecGuardPCs records bytecode PCs whose runtime guards have
	// already failed for this proto version.
	SuppressedSpecGuardPCs map[int]bool

	// SuppressedSpecGuardKinds is the guard-kind-scoped form of
	// SuppressedSpecGuardPCs. Nil is a sentinel meaning kind information is
	// unavailable and consumers must fall back to SuppressedSpecGuardPCs.
	SuppressedSpecGuardKinds map[int]map[string]bool
}

func NewSpeculationFacts() *SpeculationFacts {
	s := &SpeculationFacts{}
	s.Initialize()
	return s
}

func (s *SpeculationFacts) Initialize() {
	if s.SpecDependencyProtos == nil {
		s.SpecDependencyProtos = make(map[*vm.FuncProto]bool)
	}
	if s.SuppressedSpecGuardPCs == nil {
		s.SuppressedSpecGuardPCs = make(map[int]bool)
	}
}

func (s *SpeculationFacts) SetSpecDependencyProtos(facts map[*vm.FuncProto]bool) {
	s.SpecDependencyProtos = facts
	s.bindOwner()
}

func (s *SpeculationFacts) RecordSpecDependencyProto(owner *vm.FuncProto, callee *vm.FuncProto) {
	if s == nil || callee == nil || callee == owner {
		return
	}
	if s.SpecDependencyProtos == nil {
		s.SpecDependencyProtos = make(map[*vm.FuncProto]bool)
	}
	s.SpecDependencyProtos[callee] = true
	s.bindOwner()
}

func (s *SpeculationFacts) SetSuppressedSpecGuardPCs(facts map[int]bool) {
	s.SuppressedSpecGuardPCs = facts
	s.bindOwner()
}

func (s *SpeculationFacts) SetSuppressedSpecGuardKinds(facts map[int]map[string]bool) {
	s.SuppressedSpecGuardKinds = facts
	s.bindOwner()
}

func (s *SpeculationFacts) SpecGuardKindSuppressed(pc int, kind string) bool {
	if s == nil {
		return false
	}
	if s.SuppressedSpecGuardKinds != nil {
		if global := s.SuppressedSpecGuardKinds[tier2GlobalGuardSuppressPC]; len(global) > 0 && (global[kind] || global["*"]) {
			return true
		}
		if pc < 0 {
			return false
		}
		kinds := s.SuppressedSpecGuardKinds[pc]
		return kinds[kind] || kinds["*"]
	}
	if pc < 0 {
		return false
	}
	return s.SuppressedSpecGuardPCs != nil && s.SuppressedSpecGuardPCs[pc]
}

func (s *SpeculationFacts) bindOwner() {}

func (a *AnalysisResult) SpeculationFacts() *SpeculationFacts {
	if a == nil {
		return nil
	}
	if a.Speculation == nil {
		a.Speculation = &SpeculationFacts{}
		a.Speculation.Initialize()
	}
	a.Speculation.owner = a
	return a.Speculation
}

func (a *AnalysisResult) initializeSpeculationFacts() {
	if a == nil {
		return
	}
	a.SpeculationFacts().Initialize()
}

// TableShapeFacts groups analysis facts produced and consumed by table/field
// shape passes.
type TableShapeFacts struct {
	owner *AnalysisResult

	// FieldPolyShapeFacts records small guarded polymorphic field caches keyed
	// by OpGetField instruction ID. Each case maps receiver shapeID to the
	// field index for that static field name.
	FieldPolyShapeFacts map[int][]FieldPolyShapeCase

	// FieldPolyShapeReceivers records table SSA values known to carry guarded
	// polymorphic shape facts.
	FieldPolyShapeReceivers map[int]bool

	// FieldPolyShapeCatalog records the receiver facts behind polymorphic field
	// caches keyed by shape ID.
	FieldPolyShapeCatalog map[uint32]FixedShapeTableFact

	// FieldCallPolyLenFusions records same-block guarded field-call/field-len
	// pairs keyed by OpFieldCallFloor instruction ID.
	FieldCallPolyLenFusions map[int][]FieldCallPolyLenFusion
}

func NewTableShapeFacts() *TableShapeFacts {
	t := &TableShapeFacts{}
	t.Initialize()
	return t
}

func (t *TableShapeFacts) Initialize() {
	if t.FieldPolyShapeFacts == nil {
		t.FieldPolyShapeFacts = make(map[int][]FieldPolyShapeCase)
	}
	if t.FieldPolyShapeReceivers == nil {
		t.FieldPolyShapeReceivers = make(map[int]bool)
	}
	if t.FieldPolyShapeCatalog == nil {
		t.FieldPolyShapeCatalog = make(map[uint32]FixedShapeTableFact)
	}
	if t.FieldCallPolyLenFusions == nil {
		t.FieldCallPolyLenFusions = make(map[int][]FieldCallPolyLenFusion)
	}
}

func (t *TableShapeFacts) SetFieldPolyShapeFacts(facts map[int][]FieldPolyShapeCase) {
	t.FieldPolyShapeFacts = facts
	t.bindOwner()
}

func (t *TableShapeFacts) FieldPolyShapeCases(id int) ([]FieldPolyShapeCase, bool) {
	if t == nil || t.FieldPolyShapeFacts == nil {
		return nil, false
	}
	cases, ok := t.FieldPolyShapeFacts[id]
	return cases, ok
}

func (t *TableShapeFacts) HasFieldPolyShapeCases(id int) bool {
	cases, _ := t.FieldPolyShapeCases(id)
	return len(cases) > 0
}

func (t *TableShapeFacts) FieldPolyShapeFactCount() int {
	if t == nil {
		return 0
	}
	return len(t.FieldPolyShapeFacts)
}

func (t *TableShapeFacts) ForEachFieldPolyShapeCase(visit func(id int, c FieldPolyShapeCase) bool) {
	if t == nil || t.FieldPolyShapeFacts == nil || visit == nil {
		return
	}
	for id, cases := range t.FieldPolyShapeFacts {
		for _, c := range cases {
			if !visit(id, c) {
				return
			}
		}
	}
}

func (t *TableShapeFacts) RangeFieldPolyShapeCases(yield func(id int, cases []FieldPolyShapeCase) bool) {
	if t == nil || t.FieldPolyShapeFacts == nil || yield == nil {
		return
	}
	for id, cases := range t.FieldPolyShapeFacts {
		if !yield(id, cases) {
			return
		}
	}
}

func (t *TableShapeFacts) RecordFieldPolyShapeCases(id int, cases []FieldPolyShapeCase) {
	if t == nil {
		return
	}
	if t.FieldPolyShapeFacts == nil {
		t.FieldPolyShapeFacts = make(map[int][]FieldPolyShapeCase)
	}
	t.FieldPolyShapeFacts[id] = cases
	t.RecordFieldPolyShapeCatalogCases(cases)
	t.bindOwner()
}

func (t *TableShapeFacts) DeleteFieldPolyShapeCases(id int) {
	if t == nil || t.FieldPolyShapeFacts == nil {
		return
	}
	delete(t.FieldPolyShapeFacts, id)
	t.bindOwner()
}

func (t *TableShapeFacts) SetFieldPolyShapeReceivers(facts map[int]bool) {
	t.FieldPolyShapeReceivers = facts
	t.bindOwner()
}

func (t *TableShapeFacts) RecordFieldPolyShapeReceiver(id int) {
	if t == nil {
		return
	}
	if t.FieldPolyShapeReceivers == nil {
		t.FieldPolyShapeReceivers = make(map[int]bool)
	}
	t.FieldPolyShapeReceivers[id] = true
	t.bindOwner()
}

func (t *TableShapeFacts) FieldPolyShapeReceiver(id int) bool {
	return t != nil && t.FieldPolyShapeReceivers != nil && t.FieldPolyShapeReceivers[id]
}

func (t *TableShapeFacts) SetFieldPolyShapeCatalog(facts map[uint32]FixedShapeTableFact) {
	t.FieldPolyShapeCatalog = facts
	t.bindOwner()
}

func (t *TableShapeFacts) FieldPolyShapeCatalogFact(shapeID uint32) (FixedShapeTableFact, bool) {
	if t == nil || t.FieldPolyShapeCatalog == nil {
		return FixedShapeTableFact{}, false
	}
	fact, ok := t.FieldPolyShapeCatalog[shapeID]
	return fact, ok
}

func (t *TableShapeFacts) ForEachFieldPolyShapeCatalogFact(visit func(shapeID uint32, fact FixedShapeTableFact) bool) {
	if t == nil || t.FieldPolyShapeCatalog == nil || visit == nil {
		return
	}
	for shapeID, fact := range t.FieldPolyShapeCatalog {
		if !visit(shapeID, fact) {
			return
		}
	}
}

func (t *TableShapeFacts) RecordFieldPolyShapeCatalogCases(cases []FieldPolyShapeCase) {
	if t == nil || len(cases) == 0 {
		return
	}
	if t.FieldPolyShapeCatalog == nil {
		t.FieldPolyShapeCatalog = make(map[uint32]FixedShapeTableFact, len(cases))
	}
	for _, c := range cases {
		if c.ShapeID == 0 || c.ReceiverFact.ShapeID != c.ShapeID {
			continue
		}
		t.FieldPolyShapeCatalog[c.ShapeID] = cloneFixedShapeTableFact(c.ReceiverFact)
	}
	t.bindOwner()
}

func (t *TableShapeFacts) RecordFixedShapeCatalogFact(fact FixedShapeTableFact) {
	if t == nil || fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
		return
	}
	if t.FieldPolyShapeCatalog == nil {
		t.FieldPolyShapeCatalog = make(map[uint32]FixedShapeTableFact, 1)
	}
	t.FieldPolyShapeCatalog[fact.ShapeID] = cloneFixedShapeTableFact(fact)
	t.bindOwner()
}

func (t *TableShapeFacts) SetFieldCallPolyLenFusions(facts map[int][]FieldCallPolyLenFusion) {
	t.FieldCallPolyLenFusions = facts
	t.bindOwner()
}

func (t *TableShapeFacts) FieldCallPolyLenFusionCases(id int) ([]FieldCallPolyLenFusion, bool) {
	if t == nil || t.FieldCallPolyLenFusions == nil {
		return nil, false
	}
	fusions, ok := t.FieldCallPolyLenFusions[id]
	return fusions, ok
}

func (t *TableShapeFacts) RecordFieldCallPolyLenFusions(id int, fusions []FieldCallPolyLenFusion) {
	if t == nil || len(fusions) == 0 {
		return
	}
	if t.FieldCallPolyLenFusions == nil {
		t.FieldCallPolyLenFusions = make(map[int][]FieldCallPolyLenFusion)
	}
	t.FieldCallPolyLenFusions[id] = append(t.FieldCallPolyLenFusions[id], fusions...)
	t.bindOwner()
}

func functionTableShapeFacts(fn *Function) *TableShapeFacts {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return fn.Analysis.TableShapeFacts()
}

func (t *TableShapeFacts) bindOwner() {
	if t != nil && t.owner != nil {
		t.owner.bindTableShapeCompatibilityFields()
	}
}

func (a *AnalysisResult) TableShapeFacts() *TableShapeFacts {
	if a == nil {
		return nil
	}
	if a.TableShape == nil {
		a.TableShape = &TableShapeFacts{
			FieldPolyShapeFacts:     a.FieldPolyShapeFacts,
			FieldPolyShapeReceivers: a.FieldPolyShapeReceivers,
			FieldPolyShapeCatalog:   a.FieldPolyShapeCatalog,
			FieldCallPolyLenFusions: a.FieldCallPolyLenFusions,
		}
	} else {
		if a.FieldPolyShapeFacts != nil || a.TableShape.FieldPolyShapeFacts == nil {
			a.TableShape.FieldPolyShapeFacts = a.FieldPolyShapeFacts
		}
		if a.FieldPolyShapeReceivers != nil || a.TableShape.FieldPolyShapeReceivers == nil {
			a.TableShape.FieldPolyShapeReceivers = a.FieldPolyShapeReceivers
		}
		if a.FieldPolyShapeCatalog != nil || a.TableShape.FieldPolyShapeCatalog == nil {
			a.TableShape.FieldPolyShapeCatalog = a.FieldPolyShapeCatalog
		}
		if a.FieldCallPolyLenFusions != nil || a.TableShape.FieldCallPolyLenFusions == nil {
			a.TableShape.FieldCallPolyLenFusions = a.FieldCallPolyLenFusions
		}
	}
	a.TableShape.owner = a
	a.bindTableShapeCompatibilityFields()
	return a.TableShape
}

func (a *AnalysisResult) initializeTableShapeFacts() {
	if a == nil {
		return
	}
	a.TableShapeFacts().Initialize()
	a.bindTableShapeCompatibilityFields()
}

func (a *AnalysisResult) bindTableShapeCompatibilityFields() {
	if a == nil || a.TableShape == nil {
		return
	}
	a.FieldPolyShapeFacts = a.TableShape.FieldPolyShapeFacts
	a.FieldPolyShapeReceivers = a.TableShape.FieldPolyShapeReceivers
	a.FieldPolyShapeCatalog = a.TableShape.FieldPolyShapeCatalog
	a.FieldCallPolyLenFusions = a.TableShape.FieldCallPolyLenFusions
}

// NewAnalysisResult creates a new AnalysisResult with all non-sentinel maps initialized.
func NewAnalysisResult() *AnalysisResult {
	a := &AnalysisResult{
		Numeric:                   NewNumericFacts(),
		ShapeFieldTypeElidedLoads: make(map[int]bool),
		Call:                      NewCallFacts(),
		Speculation:               NewSpeculationFacts(),
		TableShape:                NewTableShapeFacts(),
		LoopSpecialization:        NewLoopSpecializationFacts(),

		FixedShapeTables:         make(map[int]FixedShapeTableFact),
		FixedShapeArgFacts:       make(map[int]FixedShapeTableFact),
		FixedTableConstructors:   make(map[int]FixedTableConstructorFact),
		FixedRecordNewTableSites: make(map[int]bool),
		FixedShapeEntryGuards:    make(map[int]FixedShapeTableFact),
		NumericGlobalValues:      make(map[string]runtime.Value),
		GlobalArrayElementFacts:  make(map[string]FixedShapeTableFact),
	}
	a.Numeric.owner = a
	a.Call.owner = a
	a.Speculation.owner = a
	a.TableShape.owner = a
	a.LoopSpecialization.owner = a
	a.bindTableShapeCompatibilityFields()
	return a
}

// Initialize initializes nil non-sentinel maps in the AnalysisResult.
func (a *AnalysisResult) Initialize() {
	a.initializeNumericFacts()
	a.initializeLoopSpecializationFacts()
	if a.ShapeFieldTypeElidedLoads == nil {
		a.ShapeFieldTypeElidedLoads = make(map[int]bool)
	}
	a.initializeCallFacts()
	a.initializeSpeculationFacts()
	if a.FixedShapeTables == nil {
		a.FixedShapeTables = make(map[int]FixedShapeTableFact)
	}
	a.initializeTableShapeFacts()
	if a.FixedShapeArgFacts == nil {
		a.FixedShapeArgFacts = make(map[int]FixedShapeTableFact)
	}
	if a.FixedTableConstructors == nil {
		a.FixedTableConstructors = make(map[int]FixedTableConstructorFact)
	}
	if a.FixedRecordNewTableSites == nil {
		a.FixedRecordNewTableSites = make(map[int]bool)
	}
	if a.FixedShapeEntryGuards == nil {
		a.FixedShapeEntryGuards = make(map[int]FixedShapeTableFact)
	}
	if a.NumericGlobalValues == nil {
		a.NumericGlobalValues = make(map[string]runtime.Value)
	}
	if a.GlobalArrayElementFacts == nil {
		a.GlobalArrayElementFacts = make(map[string]FixedShapeTableFact)
	}
}
