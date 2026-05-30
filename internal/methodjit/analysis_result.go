// Package methodjit implements a V8 Maglev-style method JIT compiler.
package methodjit

import (
	"github.com/Never-Labs/gscript/internal/runtime"
	"github.com/Never-Labs/gscript/internal/vm"
)

// AnalysisResult groups all analysis fact domains produced and consumed by
// optimizer modules. Passes should reach these domains through PassContext or
// the function*Facts helpers so module contracts can observe and validate reads
// and writes.
type AnalysisResult struct {
	// Numeric groups integer range and arithmetic safety facts. It is the single
	// source of truth for numeric analysis; access goes through its accessors.
	Numeric *NumericFacts

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

	// Global groups cross-proto global/ABI input facts. It is the single source
	// of truth for global analysis; access goes through its accessors.
	Global *GlobalFacts

	// accessObserver, when non-nil, records which fact domains were accessed
	// through the domain accessors during an observed module run. It is nil on
	// the production hot path; the module runner sets it only when a module-run
	// callback is active (same gating as the write-side fact diff). See
	// analysis_fact_domain.go and analysis_read_contract.go.
	accessObserver *factAccessObserver
}

// NumericFacts groups integer range and arithmetic safety facts produced and
// consumed by numeric optimization passes.
type NumericFacts struct {
	owner *AnalysisResult

	// Int48Safe is the set of integer arithmetic SSA value IDs whose runtime
	// result is provably within the int48 signed range.
	int48Safe map[int]bool

	// IntModNonZeroDivisor is the set of ModInt SSA value IDs whose divisor
	// range excludes zero.
	intModNonZeroDivisor map[int]bool

	// IntModNoSignAdjust is the set of ModInt SSA value IDs whose operand signs
	// prove that ARM64 SDIV/MSUB already matches Lua modulo semantics.
	intModNoSignAdjust map[int]bool

	// IntRanges records integer range facts computed by range analysis.
	intRanges map[int]intRange

	// ProfiledIntRanges records guarded integer range facts from runtime
	// feedback, keyed by SSA value ID.
	profiledIntRanges map[int]intRange

	// ProfiledLenRanges records guarded len() result ranges keyed by the value
	// whose length is being read.
	profiledLenRanges map[int]intRange

	// IntNonNegative is the set of integer SSA value IDs whose runtime result is
	// provably >= 0.
	intNonNegative map[int]bool
}

func NewNumericFacts() *NumericFacts {
	n := &NumericFacts{}
	n.Initialize()
	return n
}

func (n *NumericFacts) Initialize() {
	if n.int48Safe == nil {
		n.int48Safe = make(map[int]bool)
	}
	if n.intModNonZeroDivisor == nil {
		n.intModNonZeroDivisor = make(map[int]bool)
	}
	if n.intModNoSignAdjust == nil {
		n.intModNoSignAdjust = make(map[int]bool)
	}
	if n.intRanges == nil {
		n.intRanges = make(map[int]intRange)
	}
	if n.profiledIntRanges == nil {
		n.profiledIntRanges = make(map[int]intRange)
	}
	if n.profiledLenRanges == nil {
		n.profiledLenRanges = make(map[int]intRange)
	}
	if n.intNonNegative == nil {
		n.intNonNegative = make(map[int]bool)
	}
}

func (n *NumericFacts) SetInt48Safe(facts map[int]bool) {
	if n == nil {
		return
	}
	n.int48Safe = facts
	n.bindOwner()
}

func (n *NumericFacts) IsInt48Safe(id int) bool {
	return n != nil && n.int48Safe != nil && n.int48Safe[id]
}

func (n *NumericFacts) Int48SafeCount() int {
	if n == nil {
		return 0
	}
	return len(n.int48Safe)
}

func (n *NumericFacts) SetIntModNonZeroDivisor(facts map[int]bool) {
	if n == nil {
		return
	}
	n.intModNonZeroDivisor = facts
	n.bindOwner()
}

func (n *NumericFacts) IsIntModNonZeroDivisor(id int) bool {
	return n != nil && n.intModNonZeroDivisor != nil && n.intModNonZeroDivisor[id]
}

func (n *NumericFacts) SetIntModNoSignAdjust(facts map[int]bool) {
	if n == nil {
		return
	}
	n.intModNoSignAdjust = facts
	n.bindOwner()
}

func (n *NumericFacts) IsIntModNoSignAdjust(id int) bool {
	return n != nil && n.intModNoSignAdjust != nil && n.intModNoSignAdjust[id]
}

func (n *NumericFacts) SetIntRanges(facts map[int]intRange) {
	if n == nil {
		return
	}
	n.intRanges = facts
	n.bindOwner()
}

func (n *NumericFacts) IntRange(id int) (intRange, bool) {
	if n == nil || n.intRanges == nil {
		return intRange{}, false
	}
	r, ok := n.intRanges[id]
	return r, ok
}

func (n *NumericFacts) IntRangeMap() map[int]intRange {
	if n == nil {
		return nil
	}
	return n.intRanges
}

func (n *NumericFacts) SetProfiledIntRanges(facts map[int]intRange) {
	if n == nil {
		return
	}
	n.profiledIntRanges = facts
	n.bindOwner()
}

func (n *NumericFacts) ProfiledIntRange(id int) (intRange, bool) {
	if n == nil || n.profiledIntRanges == nil {
		return intRange{}, false
	}
	r, ok := n.profiledIntRanges[id]
	return r, ok
}

func (n *NumericFacts) ProfiledIntRangeMap() map[int]intRange {
	if n == nil {
		return nil
	}
	return n.profiledIntRanges
}

func (n *NumericFacts) RecordProfiledIntRange(id int, r intRange) {
	if n == nil || !r.known {
		return
	}
	if n.profiledIntRanges == nil {
		n.profiledIntRanges = make(map[int]intRange)
	}
	n.profiledIntRanges[id] = r
	n.bindOwner()
}

func (n *NumericFacts) DeleteProfiledIntRange(id int) {
	if n == nil || n.profiledIntRanges == nil {
		return
	}
	delete(n.profiledIntRanges, id)
	n.bindOwner()
}

func (n *NumericFacts) SetProfiledLenRanges(facts map[int]intRange) {
	if n == nil {
		return
	}
	n.profiledLenRanges = facts
	n.bindOwner()
}

func (n *NumericFacts) ProfiledLenRange(id int) (intRange, bool) {
	if n == nil || n.profiledLenRanges == nil {
		return intRange{}, false
	}
	r, ok := n.profiledLenRanges[id]
	return r, ok
}

func (n *NumericFacts) ProfiledLenRangeMap() map[int]intRange {
	if n == nil {
		return nil
	}
	return n.profiledLenRanges
}

func (n *NumericFacts) RecordProfiledLenRange(id int, r intRange) {
	if n == nil || id == 0 || !r.known {
		return
	}
	if n.profiledLenRanges == nil {
		n.profiledLenRanges = make(map[int]intRange)
	}
	n.profiledLenRanges[id] = r
	n.bindOwner()
}

func (n *NumericFacts) DeleteProfiledLenRange(id int) {
	if n == nil || n.profiledLenRanges == nil {
		return
	}
	delete(n.profiledLenRanges, id)
	n.bindOwner()
}

func (n *NumericFacts) SetIntNonNegative(facts map[int]bool) {
	if n == nil {
		return
	}
	n.intNonNegative = facts
	n.bindOwner()
}

func (n *NumericFacts) RecordIntNonNegative(id int) {
	if n == nil {
		return
	}
	if n.intNonNegative == nil {
		n.intNonNegative = make(map[int]bool)
	}
	n.intNonNegative[id] = true
	n.bindOwner()
}

func (n *NumericFacts) IsIntNonNegative(id int) bool {
	return n != nil && n.intNonNegative != nil && n.intNonNegative[id]
}

func (n *NumericFacts) SetComputedRanges(safe map[int]bool, ranges map[int]intRange, nonNegative map[int]bool) {
	if n == nil {
		return
	}
	n.int48Safe = safe
	n.intRanges = ranges
	n.intNonNegative = nonNegative
	n.bindOwner()
}

func (n *NumericFacts) SetModuloFacts(nonZeroDivisor map[int]bool, noSignAdjust map[int]bool) {
	if n == nil {
		return
	}
	n.intModNonZeroDivisor = nonZeroDivisor
	n.intModNoSignAdjust = noSignAdjust
	n.bindOwner()
}

func (n *NumericFacts) bindOwner() {}

func (a *AnalysisResult) NumericFacts() *NumericFacts {
	if a == nil {
		return nil
	}
	if a.accessObserver != nil {
		a.accessObserver.note(factDomainNumeric)
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
	callABIs map[int]CallABIDescriptor

	// GuardedConstCallFolds records guarded call-site guarded
	// constants keyed by OpCall instruction ID.
	guardedConstCallFolds map[int]GuardedConstCallFoldFact

	// CallSiteNoResultRuntimeSpecializations records stable structural
	// no-result call-site runtime specializations keyed by OpCall instruction
	// ID. Codegen routes them through a precise op-exit rather than the generic
	// CallExit path.
	callSiteNoResultRuntimeSpecializations map[int]bool

	// CallSiteNoResultRuntimeSpecializationBatches records loop-tail no-result
	// call-site runtime specialization sites that can safely batch future
	// complete loop iterations after the current iteration's final call has run.
	callSiteNoResultRuntimeSpecializationBatches map[int]CallSiteNoResultRuntimeSpecializationBatchFact
}

func NewCallFacts() *CallFacts {
	c := &CallFacts{}
	c.Initialize()
	return c
}

func (c *CallFacts) Initialize() {
	if c.callABIs == nil {
		c.callABIs = make(map[int]CallABIDescriptor)
	}
	if c.guardedConstCallFolds == nil {
		c.guardedConstCallFolds = make(map[int]GuardedConstCallFoldFact)
	}
	if c.callSiteNoResultRuntimeSpecializations == nil {
		c.callSiteNoResultRuntimeSpecializations = make(map[int]bool)
	}
	if c.callSiteNoResultRuntimeSpecializationBatches == nil {
		c.callSiteNoResultRuntimeSpecializationBatches = make(map[int]CallSiteNoResultRuntimeSpecializationBatchFact)
	}
}

func (c *CallFacts) SetCallABIs(facts map[int]CallABIDescriptor) {
	c.callABIs = facts
	c.bindOwner()
}

func (c *CallFacts) CallABI(id int) (CallABIDescriptor, bool) {
	if c == nil || c.callABIs == nil {
		return CallABIDescriptor{}, false
	}
	desc, ok := c.callABIs[id]
	return desc, ok
}

func (c *CallFacts) CallABICount() int {
	if c == nil {
		return 0
	}
	return len(c.callABIs)
}

// CallABIMap returns the underlying callsite ABI descriptor map. Callers read
// or iterate without mutating; mutation goes through SetCallABIs.
func (c *CallFacts) CallABIMap() map[int]CallABIDescriptor {
	if c == nil {
		return nil
	}
	return c.callABIs
}

func (c *CallFacts) ForEachCallABI(visit func(int, CallABIDescriptor) bool) {
	if c == nil || visit == nil {
		return
	}
	for id, desc := range c.callABIs {
		if !visit(id, desc) {
			return
		}
	}
}

func (c *CallFacts) SetGuardedConstCallFolds(facts map[int]GuardedConstCallFoldFact) {
	c.guardedConstCallFolds = facts
	c.bindOwner()
}

func (c *CallFacts) GuardedConstCallFold(id int) (GuardedConstCallFoldFact, bool) {
	if c == nil || c.guardedConstCallFolds == nil {
		return GuardedConstCallFoldFact{}, false
	}
	fact, ok := c.guardedConstCallFolds[id]
	return fact, ok
}

func (c *CallFacts) GuardedConstCallFoldCount() int {
	if c == nil {
		return 0
	}
	return len(c.guardedConstCallFolds)
}

func (c *CallFacts) ForEachGuardedConstCallFold(visit func(int, GuardedConstCallFoldFact) bool) {
	if c == nil || visit == nil {
		return
	}
	for id, fact := range c.guardedConstCallFolds {
		if !visit(id, fact) {
			return
		}
	}
}

func (c *CallFacts) SetCallSiteNoResultRuntimeSpecializations(facts map[int]bool) {
	c.callSiteNoResultRuntimeSpecializations = facts
	c.bindOwner()
}

func (c *CallFacts) CallSiteNoResultRuntimeSpecialization(id int) bool {
	return c != nil && c.callSiteNoResultRuntimeSpecializations != nil && c.callSiteNoResultRuntimeSpecializations[id]
}

// CallSiteNoResultRuntimeSpecializationCount reports the number of recorded
// no-result call-site runtime specializations.
func (c *CallFacts) CallSiteNoResultRuntimeSpecializationCount() int {
	if c == nil {
		return 0
	}
	return len(c.callSiteNoResultRuntimeSpecializations)
}

// CallSiteNoResultRuntimeSpecializationMap returns the underlying map of
// no-result call-site runtime specializations. Callers read or iterate without
// mutating; mutation goes through SetCallSiteNoResultRuntimeSpecializations.
func (c *CallFacts) CallSiteNoResultRuntimeSpecializationMap() map[int]bool {
	if c == nil {
		return nil
	}
	return c.callSiteNoResultRuntimeSpecializations
}

func (c *CallFacts) SetCallSiteNoResultRuntimeSpecializationBatches(facts map[int]CallSiteNoResultRuntimeSpecializationBatchFact) {
	c.callSiteNoResultRuntimeSpecializationBatches = facts
	c.bindOwner()
}

func (c *CallFacts) CallSiteNoResultRuntimeSpecializationBatch(id int) (CallSiteNoResultRuntimeSpecializationBatchFact, bool) {
	if c == nil || c.callSiteNoResultRuntimeSpecializationBatches == nil {
		return CallSiteNoResultRuntimeSpecializationBatchFact{}, false
	}
	fact, ok := c.callSiteNoResultRuntimeSpecializationBatches[id]
	return fact, ok
}

func (c *CallFacts) CallSiteNoResultRuntimeSpecializationBatchMap() map[int]CallSiteNoResultRuntimeSpecializationBatchFact {
	if c == nil {
		return nil
	}
	return c.callSiteNoResultRuntimeSpecializationBatches
}

func (c *CallFacts) bindOwner() {}

func (a *AnalysisResult) CallFacts() *CallFacts {
	if a == nil {
		return nil
	}
	if a.accessObserver != nil {
		a.accessObserver.note(factDomainCall)
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
	tableArrayUpperBoundSafe map[int]bool

	// TableArrayLowerBoundSafe is the set of table-array access instruction IDs
	// whose key >= 0 check is already guaranteed by loop-region induction facts.
	tableArrayLowerBoundSafe map[int]bool

	// LoopTableArrayFacts records the table/len/data/key contract behind each
	// TableArrayUpperBoundSafe access.
	loopTableArrayFacts map[int]LoopTableArrayFact

	// RecordArrayLoopSpecializations records generated loop-body dataflow graphs keyed
	// by OpRecordArrayLoopSpecialization instruction ID.
	recordArrayLoopSpecializations map[int]RecordArrayLoopSpecializationSpec

	// TableArrayDataPtrs records typed table-array data pointer SSA values. The
	// key is an OpTableArrayData value ID; consumers can resolve it as a raw
	// backing-array pointer only while the matching header guard remains valid.
	tableArrayDataPtrs map[int]TableArrayDataPtrFact
}

func NewLoopSpecializationFacts() *LoopSpecializationFacts {
	k := &LoopSpecializationFacts{}
	k.Initialize()
	return k
}

func (k *LoopSpecializationFacts) Initialize() {
	if k.tableArrayUpperBoundSafe == nil {
		k.tableArrayUpperBoundSafe = make(map[int]bool)
	}
	if k.tableArrayLowerBoundSafe == nil {
		k.tableArrayLowerBoundSafe = make(map[int]bool)
	}
	if k.loopTableArrayFacts == nil {
		k.loopTableArrayFacts = make(map[int]LoopTableArrayFact)
	}
	if k.recordArrayLoopSpecializations == nil {
		k.recordArrayLoopSpecializations = make(map[int]RecordArrayLoopSpecializationSpec)
	}
	if k.tableArrayDataPtrs == nil {
		k.tableArrayDataPtrs = make(map[int]TableArrayDataPtrFact)
	}
}

func (k *LoopSpecializationFacts) SetTableArrayUpperBoundSafe(facts map[int]bool) {
	k.tableArrayUpperBoundSafe = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) TableArrayUpperBoundIsSafe(id int) bool {
	return k != nil && k.tableArrayUpperBoundSafe != nil && k.tableArrayUpperBoundSafe[id]
}

func (k *LoopSpecializationFacts) TableArrayUpperBoundSafeMap() map[int]bool {
	if k == nil {
		return nil
	}
	return k.tableArrayUpperBoundSafe
}

func (k *LoopSpecializationFacts) RecordTableArrayUpperBoundSafe(id int) {
	if k == nil {
		return
	}
	if k.tableArrayUpperBoundSafe == nil {
		k.tableArrayUpperBoundSafe = make(map[int]bool)
	}
	k.tableArrayUpperBoundSafe[id] = true
	k.bindOwner()
}

func (k *LoopSpecializationFacts) SetTableArrayLowerBoundSafe(facts map[int]bool) {
	k.tableArrayLowerBoundSafe = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) TableArrayLowerBoundIsSafe(id int) bool {
	return k != nil && k.tableArrayLowerBoundSafe != nil && k.tableArrayLowerBoundSafe[id]
}

func (k *LoopSpecializationFacts) TableArrayLowerBoundSafeMap() map[int]bool {
	if k == nil {
		return nil
	}
	return k.tableArrayLowerBoundSafe
}

func (k *LoopSpecializationFacts) RecordTableArrayLowerBoundSafe(id int) {
	if k == nil {
		return
	}
	if k.tableArrayLowerBoundSafe == nil {
		k.tableArrayLowerBoundSafe = make(map[int]bool)
	}
	k.tableArrayLowerBoundSafe[id] = true
	k.bindOwner()
}

func (k *LoopSpecializationFacts) SetLoopTableArrayFacts(facts map[int]LoopTableArrayFact) {
	k.loopTableArrayFacts = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) LoopTableArrayFact(id int) (LoopTableArrayFact, bool) {
	if k == nil || k.loopTableArrayFacts == nil {
		return LoopTableArrayFact{}, false
	}
	fact, ok := k.loopTableArrayFacts[id]
	return fact, ok
}

func (k *LoopSpecializationFacts) RecordLoopTableArrayFact(id int, fact LoopTableArrayFact) {
	if k == nil {
		return
	}
	if k.loopTableArrayFacts == nil {
		k.loopTableArrayFacts = make(map[int]LoopTableArrayFact)
	}
	k.loopTableArrayFacts[id] = fact
	k.bindOwner()
}

func (k *LoopSpecializationFacts) SetRecordArrayLoopSpecializations(facts map[int]RecordArrayLoopSpecializationSpec) {
	k.recordArrayLoopSpecializations = facts
	k.bindOwner()
}

func (k *LoopSpecializationFacts) RecordArrayLoopSpecialization(id int) (RecordArrayLoopSpecializationSpec, bool) {
	if k == nil || k.recordArrayLoopSpecializations == nil {
		return RecordArrayLoopSpecializationSpec{}, false
	}
	spec, ok := k.recordArrayLoopSpecializations[id]
	return spec, ok
}

func (k *LoopSpecializationFacts) SetRecordArrayLoopSpecialization(id int, spec RecordArrayLoopSpecializationSpec) {
	if k == nil {
		return
	}
	if k.recordArrayLoopSpecializations == nil {
		k.recordArrayLoopSpecializations = make(map[int]RecordArrayLoopSpecializationSpec)
	}
	k.recordArrayLoopSpecializations[id] = spec
	k.bindOwner()
}

func (k *LoopSpecializationFacts) bindOwner() {}

func (a *AnalysisResult) LoopSpecializationFacts() *LoopSpecializationFacts {
	if a == nil {
		return nil
	}
	if a.accessObserver != nil {
		a.accessObserver.note(factDomainLoopSpec)
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

// GlobalFacts groups cross-proto global/ABI input facts. These are seeded at
// pipeline/manager time rather than produced by an analysis pass, and consumed
// by call lowering, the IR-interpreter oracle, and ABI specialization.
type GlobalFacts struct {
	owner *AnalysisResult

	// Globals maps global function names to their protos for the IR interpreter
	// to resolve residual cross-function calls, such as calls left after bounded
	// recursive inlining. Nil is a sentinel meaning the inline pass may still
	// install its config.Globals; an empty non-nil map means globals were
	// intentionally provided and no fallback install should occur. Production
	// code paths never consult this field; it exists only as a hook for the IR
	// correctness oracle.
	globals map[string]*vm.FuncProto

	// NumericGlobalValues and GlobalArrayElementFacts are stable cross-proto
	// facts supplied by the Tier 2 manager for ABI analysis. They are hints:
	// missing facts disable specialized entries, while emitted guards still
	// protect any optimized path that consumes present facts.
	numericGlobalValues     map[string]runtime.Value
	globalArrayElementFacts map[string]FixedShapeTableFact
}

func NewGlobalFacts() *GlobalFacts {
	g := &GlobalFacts{}
	g.Initialize()
	return g
}

func (g *GlobalFacts) Initialize() {
	// Globals is a sentinel field (nil is meaningful); it is intentionally not
	// allocated here.
	if g.numericGlobalValues == nil {
		g.numericGlobalValues = make(map[string]runtime.Value)
	}
	if g.globalArrayElementFacts == nil {
		g.globalArrayElementFacts = make(map[string]FixedShapeTableFact)
	}
}

func (g *GlobalFacts) bindOwner() {}

func (a *AnalysisResult) GlobalFacts() *GlobalFacts {
	if a == nil {
		return nil
	}
	if a.accessObserver != nil {
		a.accessObserver.note(factDomainGlobal)
	}
	if a.Global == nil {
		a.Global = &GlobalFacts{}
		a.Global.Initialize()
	}
	a.Global.owner = a
	return a.Global
}

func functionGlobalFacts(fn *Function) *GlobalFacts {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return fn.Analysis.GlobalFacts()
}

func (a *AnalysisResult) initializeGlobalFacts() {
	if a == nil {
		return
	}
	a.GlobalFacts().Initialize()
}

// SpeculationFacts groups analysis facts produced and consumed by Tier 2
// speculation and guard-specialization paths.
type SpeculationFacts struct {
	owner *AnalysisResult

	// SpecDependencyProtos records other protos whose runtime feedback or native
	// entry publication can change this function's optimized shape. It covers
	// inlined callees and guarded polymorphic call targets.
	specDependencyProtos map[*vm.FuncProto]bool

	// SuppressedSpecGuardPCs records bytecode PCs whose runtime guards have
	// already failed for this proto version.
	suppressedSpecGuardPCs map[int]bool

	// SuppressedSpecGuardKinds is the guard-kind-scoped form of
	// SuppressedSpecGuardPCs. Nil is a sentinel meaning kind information is
	// unavailable and consumers must fall back to SuppressedSpecGuardPCs.
	suppressedSpecGuardKinds map[int]map[string]bool
}

func NewSpeculationFacts() *SpeculationFacts {
	s := &SpeculationFacts{}
	s.Initialize()
	return s
}

func (s *SpeculationFacts) Initialize() {
	if s.specDependencyProtos == nil {
		s.specDependencyProtos = make(map[*vm.FuncProto]bool)
	}
	if s.suppressedSpecGuardPCs == nil {
		s.suppressedSpecGuardPCs = make(map[int]bool)
	}
}

func (s *SpeculationFacts) SetSpecDependencyProtos(facts map[*vm.FuncProto]bool) {
	s.specDependencyProtos = facts
	s.bindOwner()
}

func (s *SpeculationFacts) RecordSpecDependencyProto(owner *vm.FuncProto, callee *vm.FuncProto) {
	if s == nil || callee == nil || callee == owner {
		return
	}
	if s.specDependencyProtos == nil {
		s.specDependencyProtos = make(map[*vm.FuncProto]bool)
	}
	s.specDependencyProtos[callee] = true
	s.bindOwner()
}

func (s *SpeculationFacts) SetSuppressedSpecGuardPCs(facts map[int]bool) {
	s.suppressedSpecGuardPCs = facts
	s.bindOwner()
}

func (s *SpeculationFacts) SetSuppressedSpecGuardKinds(facts map[int]map[string]bool) {
	s.suppressedSpecGuardKinds = facts
	s.bindOwner()
}

func (s *SpeculationFacts) SpecGuardKindSuppressed(pc int, kind string) bool {
	if s == nil {
		return false
	}
	if s.suppressedSpecGuardKinds != nil {
		if global := s.suppressedSpecGuardKinds[tier2GlobalGuardSuppressPC]; len(global) > 0 && (global[kind] || global["*"]) {
			return true
		}
		if pc < 0 {
			return false
		}
		kinds := s.suppressedSpecGuardKinds[pc]
		return kinds[kind] || kinds["*"]
	}
	if pc < 0 {
		return false
	}
	return s.suppressedSpecGuardPCs != nil && s.suppressedSpecGuardPCs[pc]
}

func (s *SpeculationFacts) bindOwner() {}

func (a *AnalysisResult) SpeculationFacts() *SpeculationFacts {
	if a == nil {
		return nil
	}
	if a.accessObserver != nil {
		a.accessObserver.note(factDomainSpeculation)
	}
	if a.Speculation == nil {
		a.Speculation = &SpeculationFacts{}
		a.Speculation.Initialize()
	}
	a.Speculation.owner = a
	return a.Speculation
}

func functionSpeculationFacts(fn *Function) *SpeculationFacts {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return fn.Analysis.SpeculationFacts()
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
	fieldPolyShapeFacts map[int][]FieldPolyShapeCase

	// FieldPolyShapeReceivers records table SSA values known to carry guarded
	// polymorphic shape facts.
	fieldPolyShapeReceivers map[int]bool

	// FieldPolyShapeCatalog records the receiver facts behind polymorphic field
	// caches keyed by shape ID.
	fieldPolyShapeCatalog map[uint32]FixedShapeTableFact

	// FieldCallPolyLenFusions records same-block guarded field-call/field-len
	// pairs keyed by OpFieldCallFloor instruction ID.
	fieldCallPolyLenFusions map[int][]FieldCallPolyLenFusion

	// FixedShapeTables records SSA table values whose field layout is known
	// without consulting the runtime field cache.
	fixedShapeTables map[int]FixedShapeTableFact

	// FixedShapeArgFacts records guarded fixed-shape facts keyed by parameter
	// index.
	fixedShapeArgFacts map[int]FixedShapeTableFact

	// FixedTableConstructors records OpNewTable values that came from a
	// bytecode-level fixed string-field table constructor.
	fixedTableConstructors map[int]FixedTableConstructorFact

	// FixedRecordNewTableSites records OpNewFixedTable values that remain local
	// to fixed-record-aware field reads.
	fixedRecordNewTableSites map[int]bool

	// FixedShapeEntryGuards records parameter shape guards that codegen must
	// execute before entering the optimized body.
	fixedShapeEntryGuards map[int]FixedShapeTableFact

	// ShapeFieldTypeElidedLoads marks fixed-shape field loads whose result type
	// is guarded once by shape field type epochs.
	shapeFieldTypeElidedLoads map[int]bool
}

func NewTableShapeFacts() *TableShapeFacts {
	t := &TableShapeFacts{}
	t.Initialize()
	return t
}

func (t *TableShapeFacts) Initialize() {
	if t.fieldPolyShapeFacts == nil {
		t.fieldPolyShapeFacts = make(map[int][]FieldPolyShapeCase)
	}
	if t.fieldPolyShapeReceivers == nil {
		t.fieldPolyShapeReceivers = make(map[int]bool)
	}
	if t.fieldPolyShapeCatalog == nil {
		t.fieldPolyShapeCatalog = make(map[uint32]FixedShapeTableFact)
	}
	if t.fieldCallPolyLenFusions == nil {
		t.fieldCallPolyLenFusions = make(map[int][]FieldCallPolyLenFusion)
	}
	if t.fixedShapeTables == nil {
		t.fixedShapeTables = make(map[int]FixedShapeTableFact)
	}
	if t.fixedShapeArgFacts == nil {
		t.fixedShapeArgFacts = make(map[int]FixedShapeTableFact)
	}
	if t.fixedTableConstructors == nil {
		t.fixedTableConstructors = make(map[int]FixedTableConstructorFact)
	}
	if t.fixedRecordNewTableSites == nil {
		t.fixedRecordNewTableSites = make(map[int]bool)
	}
	if t.fixedShapeEntryGuards == nil {
		t.fixedShapeEntryGuards = make(map[int]FixedShapeTableFact)
	}
	if t.shapeFieldTypeElidedLoads == nil {
		t.shapeFieldTypeElidedLoads = make(map[int]bool)
	}
}

func (t *TableShapeFacts) SetFieldPolyShapeFacts(facts map[int][]FieldPolyShapeCase) {
	t.fieldPolyShapeFacts = facts
	t.bindOwner()
}

func (t *TableShapeFacts) FieldPolyShapeCases(id int) ([]FieldPolyShapeCase, bool) {
	if t == nil || t.fieldPolyShapeFacts == nil {
		return nil, false
	}
	cases, ok := t.fieldPolyShapeFacts[id]
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
	return len(t.fieldPolyShapeFacts)
}

func (t *TableShapeFacts) ForEachFieldPolyShapeCase(visit func(id int, c FieldPolyShapeCase) bool) {
	if t == nil || t.fieldPolyShapeFacts == nil || visit == nil {
		return
	}
	for id, cases := range t.fieldPolyShapeFacts {
		for _, c := range cases {
			if !visit(id, c) {
				return
			}
		}
	}
}

func (t *TableShapeFacts) RangeFieldPolyShapeCases(yield func(id int, cases []FieldPolyShapeCase) bool) {
	if t == nil || t.fieldPolyShapeFacts == nil || yield == nil {
		return
	}
	for id, cases := range t.fieldPolyShapeFacts {
		if !yield(id, cases) {
			return
		}
	}
}

func (t *TableShapeFacts) RecordFieldPolyShapeCases(id int, cases []FieldPolyShapeCase) {
	if t == nil {
		return
	}
	if t.fieldPolyShapeFacts == nil {
		t.fieldPolyShapeFacts = make(map[int][]FieldPolyShapeCase)
	}
	t.fieldPolyShapeFacts[id] = cases
	t.RecordFieldPolyShapeCatalogCases(cases)
	t.bindOwner()
}

func (t *TableShapeFacts) DeleteFieldPolyShapeCases(id int) {
	if t == nil || t.fieldPolyShapeFacts == nil {
		return
	}
	delete(t.fieldPolyShapeFacts, id)
	t.bindOwner()
}

func (t *TableShapeFacts) SetFieldPolyShapeReceivers(facts map[int]bool) {
	t.fieldPolyShapeReceivers = facts
	t.bindOwner()
}

func (t *TableShapeFacts) RecordFieldPolyShapeReceiver(id int) {
	if t == nil {
		return
	}
	if t.fieldPolyShapeReceivers == nil {
		t.fieldPolyShapeReceivers = make(map[int]bool)
	}
	t.fieldPolyShapeReceivers[id] = true
	t.bindOwner()
}

func (t *TableShapeFacts) FieldPolyShapeReceiver(id int) bool {
	return t != nil && t.fieldPolyShapeReceivers != nil && t.fieldPolyShapeReceivers[id]
}

func (t *TableShapeFacts) SetFieldPolyShapeCatalog(facts map[uint32]FixedShapeTableFact) {
	t.fieldPolyShapeCatalog = facts
	t.bindOwner()
}

func (t *TableShapeFacts) FieldPolyShapeCatalogFact(shapeID uint32) (FixedShapeTableFact, bool) {
	if t == nil || t.fieldPolyShapeCatalog == nil {
		return FixedShapeTableFact{}, false
	}
	fact, ok := t.fieldPolyShapeCatalog[shapeID]
	return fact, ok
}

func (t *TableShapeFacts) ForEachFieldPolyShapeCatalogFact(visit func(shapeID uint32, fact FixedShapeTableFact) bool) {
	if t == nil || t.fieldPolyShapeCatalog == nil || visit == nil {
		return
	}
	for shapeID, fact := range t.fieldPolyShapeCatalog {
		if !visit(shapeID, fact) {
			return
		}
	}
}

func (t *TableShapeFacts) RecordFieldPolyShapeCatalogCases(cases []FieldPolyShapeCase) {
	if t == nil || len(cases) == 0 {
		return
	}
	if t.fieldPolyShapeCatalog == nil {
		t.fieldPolyShapeCatalog = make(map[uint32]FixedShapeTableFact, len(cases))
	}
	for _, c := range cases {
		if c.ShapeID == 0 || c.ReceiverFact.ShapeID != c.ShapeID {
			continue
		}
		t.fieldPolyShapeCatalog[c.ShapeID] = cloneFixedShapeTableFact(c.ReceiverFact)
	}
	t.bindOwner()
}

func (t *TableShapeFacts) RecordFixedShapeCatalogFact(fact FixedShapeTableFact) {
	if t == nil || fact.ShapeID == 0 || len(fact.FieldNames) == 0 {
		return
	}
	if t.fieldPolyShapeCatalog == nil {
		t.fieldPolyShapeCatalog = make(map[uint32]FixedShapeTableFact, 1)
	}
	t.fieldPolyShapeCatalog[fact.ShapeID] = cloneFixedShapeTableFact(fact)
	t.bindOwner()
}

func (t *TableShapeFacts) SetFieldCallPolyLenFusions(facts map[int][]FieldCallPolyLenFusion) {
	t.fieldCallPolyLenFusions = facts
	t.bindOwner()
}

func (t *TableShapeFacts) FieldCallPolyLenFusionCases(id int) ([]FieldCallPolyLenFusion, bool) {
	if t == nil || t.fieldCallPolyLenFusions == nil {
		return nil, false
	}
	fusions, ok := t.fieldCallPolyLenFusions[id]
	return fusions, ok
}

func (t *TableShapeFacts) RecordFieldCallPolyLenFusions(id int, fusions []FieldCallPolyLenFusion) {
	if t == nil || len(fusions) == 0 {
		return
	}
	if t.fieldCallPolyLenFusions == nil {
		t.fieldCallPolyLenFusions = make(map[int][]FieldCallPolyLenFusion)
	}
	t.fieldCallPolyLenFusions[id] = append(t.fieldCallPolyLenFusions[id], fusions...)
	t.bindOwner()
}

func functionTableShapeFacts(fn *Function) *TableShapeFacts {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return fn.Analysis.TableShapeFacts()
}

func (t *TableShapeFacts) bindOwner() {}

func (a *AnalysisResult) TableShapeFacts() *TableShapeFacts {
	if a == nil {
		return nil
	}
	if a.accessObserver != nil {
		a.accessObserver.note(factDomainTableShape)
	}
	if a.TableShape == nil {
		a.TableShape = &TableShapeFacts{}
		a.TableShape.Initialize()
	}
	a.TableShape.owner = a
	return a.TableShape
}

func (a *AnalysisResult) initializeTableShapeFacts() {
	if a == nil {
		return
	}
	a.TableShapeFacts().Initialize()
}

// NewAnalysisResult creates a new AnalysisResult with all non-sentinel maps initialized.
func NewAnalysisResult() *AnalysisResult {
	a := &AnalysisResult{
		Numeric:            NewNumericFacts(),
		Call:               NewCallFacts(),
		Speculation:        NewSpeculationFacts(),
		TableShape:         NewTableShapeFacts(),
		LoopSpecialization: NewLoopSpecializationFacts(),
		Global:             NewGlobalFacts(),
	}
	a.Numeric.owner = a
	a.Call.owner = a
	a.Speculation.owner = a
	a.TableShape.owner = a
	a.LoopSpecialization.owner = a
	a.Global.owner = a
	return a
}

// Initialize initializes nil non-sentinel maps in the AnalysisResult.
func (a *AnalysisResult) Initialize() {
	a.initializeNumericFacts()
	a.initializeLoopSpecializationFacts()
	a.initializeCallFacts()
	a.initializeSpeculationFacts()
	a.initializeTableShapeFacts()
	a.initializeGlobalFacts()
}
