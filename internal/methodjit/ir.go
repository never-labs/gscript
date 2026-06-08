package methodjit

import (
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// LoopTableArrayFact describes a table-array access admitted by loop-region
// versioning. The preheader has already verified table/metatable/kind and
// loaded stable len/data facts; an existing loop header branch proves the
// access key is below that len on every continuing iteration.
type LoopTableArrayFact struct {
	HeaderBlockID    int
	PreheaderBlockID int
	TableID          int
	TableHeaderID    int
	LenID            int
	DataID           int
	KeyID            int
	Kind             int64
	AccessOp         Op
}

// GuardedConstCallFoldFact records a callsite whose callee is a guarded
// guarded entry and whose integer arguments are compile-time constants or
// guarded stable globals. Codegen guards dependencies before materializing the
// folded Result; guard miss falls back to the normal call-exit path.
type GuardedConstCallFoldFact struct {
	CalleeProto    *vm.FuncProto
	Result         int64
	GuardConsts    []int
	GuardProtos    []*vm.FuncProto
	IntGuardConsts []int
	IntGuardValues []int64
}

type CallSiteNoResultRuntimeSpecializationBatchCall struct {
	FuncConst int
	ArgConsts []int
}

type CallSiteNoResultRuntimeSpecializationBatchFact struct {
	LoopBase int
	ExitPC   int
	Calls    []CallSiteNoResultRuntimeSpecializationBatchCall
}

type StringSplitSubSpec struct {
	TokenIndex   int64
	Start        int64
	End          int64
	HasEnd       bool
	SubCallCount int
	FirstStart   int64
	FirstEnd     int64
	FirstHasEnd  bool
	SecondStart  int64
	SecondEnd    int64
	SecondHasEnd bool
}

type QFrameSelectColumnSpec struct {
	Shape             string
	SourceColumnConst int
	MaskSpecConst     int
	ProjectConst      int
	ResultColumnConst int
	CompareOp         runtime.DenseArrayBinaryOp
}

type RecordArraySpecializationSourceKind uint8

const (
	RecordArraySpecializationSourceField RecordArraySpecializationSourceKind = iota
	RecordArraySpecializationSourceScalar
	RecordArraySpecializationSourceOp
)

type RecordArraySpecializationFloatOpKind uint8

const (
	RecordArraySpecializationFloatOpMul RecordArraySpecializationFloatOpKind = iota
	RecordArraySpecializationFloatOpFMA
)

type RecordArraySpecializationSource struct {
	Kind  RecordArraySpecializationSourceKind
	Index int
}

type RecordArraySpecializationFloatOp struct {
	Kind RecordArraySpecializationFloatOpKind
	A    RecordArraySpecializationSource
	B    RecordArraySpecializationSource
	C    RecordArraySpecializationSource
}

type RecordArraySpecializationStore struct {
	Field int
	Value RecordArraySpecializationSource
}

// RecordArrayLoopSpecializationSpec is a compact dataflow graph for a generated native
// loop over a table array whose elements are fixed-shape records. Args on the
// IR op are [arrayData, arrayLen, limit, scalar...]. The spec supplies record
// shape validation, per-record field loads, scalar float operands, float ops,
// and field stores.
type RecordArrayLoopSpecializationSpec struct {
	ShapeID     uint32
	FieldLoads  []int
	ScalarCount int
	Ops         []RecordArraySpecializationFloatOp
	Stores      []RecordArraySpecializationStore
	MaxField    int
	Cache       *RecordArrayLoopSpecializationCache
}

const RecordArrayLoopSpecializationMaxCachedSvals = 8192

// RecordArrayLoopSpecializationCache memoizes a successful full-shape validation for a
// native record-array loop. It is guarded by the outer table's array version
// and the record shape's layout epoch, so value-only field writes do not force
// the next call to re-scan every element. Svals caches the record payload
// pointers discovered during validation so the hot update loop can avoid
// re-decoding every boxed table element on subsequent calls.
type RecordArrayLoopSpecializationCache struct {
	Table            uintptr
	ArrayVersion     uint64
	ShapeLayoutEpoch uint64
	Limit            int64
	Svals            [RecordArrayLoopSpecializationMaxCachedSvals + 1]uintptr
}

// Function is the complete IR for one compiled function.
type Function struct {
	Entry    *Block          // entry basic block
	Blocks   []*Block        // all blocks in RPO (reverse postorder)
	Proto    *vm.FuncProto   // source bytecode
	NumRegs  int             // number of VM registers used
	nextID   int             // next value ID
	Analysis *AnalysisResult // analysis results from optimization passes

	// StringConstTables records small immutable lookup tables used by narrow
	// string-format lowerings. CompiledFunction keeps these slices alive after
	// codegen embeds their backing-array addresses.
	StringConstTables [][]runtime.Value

	// StringFormatPatterns records immutable pattern metadata shared by
	// string.format lowerings. Patterns are accepted by syntax shape or guarded
	// constant identity, not by fixed workload literal value.
	StringFormatPatterns []string

	// StringSplitSubSpecs records immutable split-token substring coordinates
	// shared by string.split(...)[k] + string.sub(...) fusion lowerings.
	StringSplitSubSpecs []StringSplitSubSpec

	// RecordArrayLoopCaches tracks record-array loop specialization cache objects.
	// This slice must stay in Function because it owns the cached data
	// lifetime, not analysis results.
	RecordArrayLoopCaches []*RecordArrayLoopSpecializationCache

	// QFrameSelectColumnSpecs records q primitive hot-path metadata consumed by
	// fused runtime-kernel op-exits.
	QFrameSelectColumnSpecs []QFrameSelectColumnSpec

	// Unpromotable, when true, signals that this function cannot be safely
	// compiled at Tier 2 because BuildGraph encountered bytecode patterns
	// it does not model. Set by the graph builder and checked by
	// compileTier2; an unpromotable function stays at Tier 1.
	//
	// Currently set when OP_CALL B==0 (variadic args threaded via top) is
	// seen: the graph builder cannot statically determine the argument
	// count, so emitting an OpCall would drop arguments and corrupt the
	// call. Patterns like `outer(x, inner(...))` and `return f(g(...))`
	// compile to CALL B=0.
	Unpromotable bool

	// CarryPreheaderInvariants, when true, enables the register allocator
	// to pin selected loop-invariant values across loop-body blocks. Today
	// this covers LICM-hoisted float values in FPRs and typed-array len/data
	// facts in GPRs. Set to true by compileTier2 after LICM runs. Defaults
	// to false (Go zero value).
	CarryPreheaderInvariants bool

	// Remarks is an optional diagnostic sink for optimization decisions.
	// Production compiles leave it nil; CompileForDiagnostics wires it so
	// passes can explain important changes and misses without stderr prints.
	Remarks *OptimizationRemarks
}

type FieldCallPolyLenFusion struct {
	LenValueID int
	FieldAux   int64
	ShapeID    uint32
	Len        int64
}

// CallABIDescriptor is the stable callsite ABI contract for one OpCall.
// It is intentionally exact: the callee proto, argument/result counts, and
// raw-int parameter/result representations must all match before codegen can
// use a specialized call path.
type CallABIDescriptor struct {
	Callee       *vm.FuncProto
	NumArgs      int
	NumRets      int
	RawIntParams []bool
	RawIntReturn bool
	TypedPeer    bool
	ParamReps    []SpecializedABIParamRep
	ReturnRep    SpecializedABIReturnRep
	ArgFacts     map[int]FixedShapeTableFact
}

// CallABIAnnotationConfig supplies global function facts to the call ABI
// annotation pass. The pass also derives conservative stable globals from the
// current proto when possible.
type CallABIAnnotationConfig struct {
	Globals                 map[string]*vm.FuncProto
	NumericGlobalValues     map[string]runtime.Value
	GlobalArrayElementFacts map[string]FixedShapeTableFact
	TableShapes             *TableShapeFacts
	CallFacts               *CallFacts
	NumericFacts            *NumericFacts
	SpeculationFacts        *SpeculationFacts
	DependencyRegistry      *CompilationDependencyRegistry
}

// TableArrayDataPtrFact describes the guard-backed ABI contract for a typed
// table-array backing pointer. HeaderID is the OpTableArrayHeader guard that
// proved TableID is a table with the requested array kind and no metatable.
// LenID is optional but present for the normal lowerer shape.
type TableArrayDataPtrFact struct {
	TableID  int
	HeaderID int
	LenID    int
	Kind     int64
}

// newValueID allocates a unique ID for a new SSA value.
func (f *Function) newValueID() int {
	id := f.nextID
	f.nextID++
	return id
}

// ensureAnalysis initializes the Analysis field if nil and returns it.
func (f *Function) ensureAnalysis() *AnalysisResult {
	if f.Analysis == nil {
		f.Analysis = NewAnalysisResult()
	}
	return f.Analysis
}

// Block represents a basic block in the control flow graph.
type Block struct {
	ID     int      // unique block ID
	Instrs []*Instr // instructions (last one is always a terminator)
	Preds  []*Block // predecessor blocks
	Succs  []*Block // successor blocks

	// SSA construction state (used by graph builder, not needed after)
	sealed     bool            // all predecessors known
	incomplete []incompletePhi // phis waiting for predecessors
	defs       map[int]*Value  // slot → current SSA value definition in this block
}

// incompletePhi tracks a phi node that needs more args when predecessors are sealed.
type incompletePhi struct {
	slot int
	phi  *Instr
}

// Instr is one SSA instruction within a basic block.
type Instr struct {
	ID    int      // unique instruction ID (= its Value ID)
	Op    Op       // operation
	Type  Type     // result type
	Args  []*Value // SSA value inputs
	Aux   int64    // auxiliary data (constant value, field index, slot number, etc.)
	Aux2  int64    // second auxiliary (e.g., for Branch: true block ID)
	Block *Block   // owning block

	// Source metadata links this IR instruction back to the bytecode that
	// produced it. HasSource is false for synthetic instructions introduced by
	// passes or CFG repair unless the pass explicitly copies source metadata.
	HasSource   bool
	SourceProto *vm.FuncProto
	SourcePC    int
	SourceLine  int
}

// Value returns the SSA value produced by this instruction.
func (i *Instr) Value() *Value {
	return &Value{ID: i.ID, Def: i}
}

// Value represents an SSA value (the result of an instruction).
type Value struct {
	ID  int    // unique value ID
	Def *Instr // instruction that defines this value (nil for function parameters)
}

// Type represents the type of an SSA value.
type Type uint8

const (
	TypeUnknown Type = iota
	TypeInt
	TypeFloat
	TypeBool
	TypeString
	TypeTable
	TypeNil
	TypeFunction
	TypeAny // unspecialized (dynamic)
)

var typeNames = [...]string{
	TypeUnknown:  "unknown",
	TypeInt:      "int",
	TypeFloat:    "float",
	TypeBool:     "bool",
	TypeString:   "string",
	TypeTable:    "table",
	TypeNil:      "nil",
	TypeFunction: "function",
	TypeAny:      "any",
}

func (t Type) String() string {
	if int(t) < len(typeNames) {
		return typeNames[t]
	}
	return "?"
}
