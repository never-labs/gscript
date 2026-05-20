// Package methodjit implements a V8 Maglev-style method JIT compiler.
package methodjit

import (
	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// AnalysisResult contains all analysis maps produced and consumed by optimization passes.
// This struct extracts analysis state from the Function god object.
type AnalysisResult struct {
	// Int48Safe is the set of integer arithmetic SSA value IDs whose runtime
	// result is provably within the int48 signed range. Populated by
	// RangeAnalysisPass. The emitter consults this set to skip the
	// SBFX+CMP+B.NE overflow check for provably safe AddInt/SubInt/MulInt/NegInt.
	Int48Safe map[int]bool

	// IntModNonZeroDivisor is the set of ModInt SSA value IDs whose divisor
	// range excludes zero. Populated by RangeAnalysisPass so the emitter can
	// skip the modulo-by-zero deopt guard at those sites.
	IntModNonZeroDivisor map[int]bool

	// IntModNoSignAdjust is the set of ModInt SSA value IDs whose operand signs
	// prove that ARM64 SDIV/MSUB already matches Lua modulo semantics. Populated
	// by RangeAnalysisPass so the emitter can skip the sign-adjust slow path.
	IntModNoSignAdjust map[int]bool

	// IntRanges records the integer range facts computed by RangeAnalysisPass.
	// Unlike Int48Safe, consumers must treat these facts as optimization hints:
	// missing or unknown ranges mean "top", not failure. OverflowBoxing uses
	// this to distinguish bounded linear inductions from overflow-prone
	// arithmetic recurrences such as multiplicative LCGs.
	IntRanges map[int]intRange

	// ProfiledIntRanges records guarded integer range facts from runtime
	// feedback, keyed by SSA value ID. RangeAnalysis consumes these as seeds;
	// the guarding shape/type facts are still emitted separately, so missing
	// or invalid profile data only disables the optimization.
	ProfiledIntRanges map[int]intRange

	// ProfiledLenRanges records guarded len() result ranges for SSA values,
	// keyed by the value whose length is being read. It lets RangeAnalysis seed
	// OpLen without changing the value's own type or representation.
	ProfiledLenRanges map[int]intRange

	// IntNonNegative is the set of integer SSA value IDs whose runtime result is
	// provably >= 0. Populated by RangeAnalysisPass for consumers that only need
	// a sign fact and must not reuse Int48Safe's overflow-specific meaning.
	IntNonNegative map[int]bool

	// TableArrayUpperBoundSafe is the set of table-array access instruction IDs
	// whose key < len check is already guaranteed by an enclosing loop-region
	// fact. The emitter still performs key type and non-negative checks unless
	// separate facts prove those safe.
	TableArrayUpperBoundSafe map[int]bool

	// TableArrayLowerBoundSafe is the set of table-array access instruction IDs
	// whose key >= 0 check is already guaranteed by loop-region induction facts.
	// It is separate from IntNonNegative so versioning-derived facts remain
	// local to the guarded loop region.
	TableArrayLowerBoundSafe map[int]bool

	// LoopTableArrayFacts records the table/len/data/key contract behind each
	// TableArrayUpperBoundSafe access. It is diagnostic and a staging point for
	// broader loop-region versioning; codegen treats missing entries as a lack
	// of optimization, not as an error.
	LoopTableArrayFacts map[int]LoopTableArrayFact

	// ShapeFieldTypeElidedLoads marks fixed-shape field loads whose result type
	// is guarded once by shape field type epochs. Codegen may skip the per-load
	// NaN-box tag check for these loads.
	ShapeFieldTypeElidedLoads map[int]bool

	// TableArrayDataPtrs records typed table-array data pointer SSA values. The
	// key is an OpTableArrayData value ID; consumers can resolve it as a raw
	// backing-array pointer only while the matching header guard remains valid.
	TableArrayDataPtrs map[int]TableArrayDataPtrFact

	// RecordArrayLoopKernels records generated loop-body dataflow graphs keyed
	// by OpRecordArrayLoopKernel instruction ID.
	RecordArrayLoopKernels map[int]RecordArrayLoopKernelSpec

	// CallABIs records stable callsite ABI facts keyed by OpCall instruction
	// ID. A descriptor is required before codegen may use a specialized
	// cross-proto raw-int call path; OpCall.Type alone is not authoritative.
	CallABIs map[int]CallABIDescriptor

	// SpecDependencyProtos records other protos whose runtime feedback or native
	// entry publication can change this function's optimized shape. It covers
	// inlined callees and guarded polymorphic call targets.
	SpecDependencyProtos map[*vm.FuncProto]bool

	// SuppressedSpecGuardPCs records bytecode PCs whose runtime guards have
	// already failed for this proto version. Later passes must treat matching
	// feedback as unstable and keep the generic path instead of regenerating
	// the same guarded specialization.
	SuppressedSpecGuardPCs map[int]bool

	// SuppressedSpecGuardKinds is the guard-kind-scoped form of
	// SuppressedSpecGuardPCs. Passes that know the guard they are about to emit
	// should consult this map so one unstable guard does not disable unrelated
	// specializations at the same bytecode PC.
	SuppressedSpecGuardKinds map[int]map[string]bool

	// ProtocolConstCallFolds records guarded whole-call protocol constants
	// keyed by OpCall instruction ID.
	ProtocolConstCallFolds map[int]ProtocolConstCallFoldFact

	// WholeCallNoResultKernels records stable structural no-result whole-call
	// kernels keyed by OpCall instruction ID. Codegen routes them through a
	// precise op-exit rather than the generic CallExit path.
	WholeCallNoResultKernels map[int]bool

	// WholeCallNoResultBatches records loop-tail no-result whole-call kernel
	// sites that can safely batch future complete loop iterations after the
	// current iteration's final kernel call has run.
	WholeCallNoResultBatches map[int]WholeCallNoResultBatchFact

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

	// Globals, if non-nil, maps global function names to their protos.
	// Used by the IR interpreter to resolve residual cross-function calls
	// (e.g., those left after bounded recursive inlining). Populated by
	// the inline pass when its config includes a globals map. Production
	// code paths never consult this field — it exists only as a hook for
	// the IR correctness oracle.
	Globals map[string]*vm.FuncProto

	// NumericGlobalValues and GlobalArrayElementFacts are stable cross-proto
	// facts supplied by the Tier 2 manager for ABI analysis. They are hints:
	// missing facts disable specialized entries, while emitted guards still
	// protect any optimized path that consumes present facts.
	NumericGlobalValues     map[string]runtime.Value
	GlobalArrayElementFacts map[string]FixedShapeTableFact
}

// NewAnalysisResult creates a new AnalysisResult with all maps initialized.
func NewAnalysisResult() *AnalysisResult {
	return &AnalysisResult{
		Int48Safe:                  make(map[int]bool),
		IntModNonZeroDivisor:       make(map[int]bool),
		IntModNoSignAdjust:         make(map[int]bool),
		IntRanges:                  make(map[int]intRange),
		ProfiledIntRanges:          make(map[int]intRange),
		ProfiledLenRanges:          make(map[int]intRange),
		IntNonNegative:             make(map[int]bool),
		TableArrayUpperBoundSafe:   make(map[int]bool),
		TableArrayLowerBoundSafe:   make(map[int]bool),
		LoopTableArrayFacts:        make(map[int]LoopTableArrayFact),
		ShapeFieldTypeElidedLoads:  make(map[int]bool),
		TableArrayDataPtrs:         make(map[int]TableArrayDataPtrFact),
		RecordArrayLoopKernels:     make(map[int]RecordArrayLoopKernelSpec),
		CallABIs:                   make(map[int]CallABIDescriptor),
		SpecDependencyProtos:       make(map[*vm.FuncProto]bool),
		SuppressedSpecGuardPCs:     make(map[int]bool),
		SuppressedSpecGuardKinds:   make(map[int]map[string]bool),
		ProtocolConstCallFolds:     make(map[int]ProtocolConstCallFoldFact),
		WholeCallNoResultKernels:   make(map[int]bool),
		WholeCallNoResultBatches:   make(map[int]WholeCallNoResultBatchFact),
		FixedShapeTables:           make(map[int]FixedShapeTableFact),
		FieldPolyShapeFacts:        make(map[int][]FieldPolyShapeCase),
		FieldPolyShapeReceivers:    make(map[int]bool),
		FieldPolyShapeCatalog:      make(map[uint32]FixedShapeTableFact),
		FieldCallPolyLenFusions:    make(map[int][]FieldCallPolyLenFusion),
		FixedShapeArgFacts:         make(map[int]FixedShapeTableFact),
		FixedTableConstructors:     make(map[int]FixedTableConstructorFact),
		FixedRecordNewTableSites:   make(map[int]bool),
		FixedShapeEntryGuards:      make(map[int]FixedShapeTableFact),
		Globals:                    make(map[string]*vm.FuncProto),
		NumericGlobalValues:        make(map[string]runtime.Value),
		GlobalArrayElementFacts:    make(map[string]FixedShapeTableFact),
	}
}

// Initialize initializes all maps in the AnalysisResult.
func (a *AnalysisResult) Initialize() {
	if a.Int48Safe == nil {
		a.Int48Safe = make(map[int]bool)
	}
	if a.IntModNonZeroDivisor == nil {
		a.IntModNonZeroDivisor = make(map[int]bool)
	}
	if a.IntModNoSignAdjust == nil {
		a.IntModNoSignAdjust = make(map[int]bool)
	}
	if a.IntRanges == nil {
		a.IntRanges = make(map[int]intRange)
	}
	if a.ProfiledIntRanges == nil {
		a.ProfiledIntRanges = make(map[int]intRange)
	}
	if a.ProfiledLenRanges == nil {
		a.ProfiledLenRanges = make(map[int]intRange)
	}
	if a.IntNonNegative == nil {
		a.IntNonNegative = make(map[int]bool)
	}
	if a.TableArrayUpperBoundSafe == nil {
		a.TableArrayUpperBoundSafe = make(map[int]bool)
	}
	if a.TableArrayLowerBoundSafe == nil {
		a.TableArrayLowerBoundSafe = make(map[int]bool)
	}
	if a.LoopTableArrayFacts == nil {
		a.LoopTableArrayFacts = make(map[int]LoopTableArrayFact)
	}
	if a.ShapeFieldTypeElidedLoads == nil {
		a.ShapeFieldTypeElidedLoads = make(map[int]bool)
	}
	if a.TableArrayDataPtrs == nil {
		a.TableArrayDataPtrs = make(map[int]TableArrayDataPtrFact)
	}
	if a.RecordArrayLoopKernels == nil {
		a.RecordArrayLoopKernels = make(map[int]RecordArrayLoopKernelSpec)
	}
	if a.CallABIs == nil {
		a.CallABIs = make(map[int]CallABIDescriptor)
	}
	if a.SpecDependencyProtos == nil {
		a.SpecDependencyProtos = make(map[*vm.FuncProto]bool)
	}
	if a.SuppressedSpecGuardPCs == nil {
		a.SuppressedSpecGuardPCs = make(map[int]bool)
	}
	if a.SuppressedSpecGuardKinds == nil {
		a.SuppressedSpecGuardKinds = make(map[int]map[string]bool)
	}
	if a.ProtocolConstCallFolds == nil {
		a.ProtocolConstCallFolds = make(map[int]ProtocolConstCallFoldFact)
	}
	if a.WholeCallNoResultKernels == nil {
		a.WholeCallNoResultKernels = make(map[int]bool)
	}
	if a.WholeCallNoResultBatches == nil {
		a.WholeCallNoResultBatches = make(map[int]WholeCallNoResultBatchFact)
	}
	if a.FixedShapeTables == nil {
		a.FixedShapeTables = make(map[int]FixedShapeTableFact)
	}
	if a.FieldPolyShapeFacts == nil {
		a.FieldPolyShapeFacts = make(map[int][]FieldPolyShapeCase)
	}
	if a.FieldPolyShapeReceivers == nil {
		a.FieldPolyShapeReceivers = make(map[int]bool)
	}
	if a.FieldPolyShapeCatalog == nil {
		a.FieldPolyShapeCatalog = make(map[uint32]FixedShapeTableFact)
	}
	if a.FieldCallPolyLenFusions == nil {
		a.FieldCallPolyLenFusions = make(map[int][]FieldCallPolyLenFusion)
	}
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
	if a.Globals == nil {
		a.Globals = make(map[string]*vm.FuncProto)
	}
	if a.NumericGlobalValues == nil {
		a.NumericGlobalValues = make(map[string]runtime.Value)
	}
	if a.GlobalArrayElementFacts == nil {
		a.GlobalArrayElementFacts = make(map[string]FixedShapeTableFact)
	}
}