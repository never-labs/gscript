package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// CompilationDependencyKind names a compile-time assumption that must still be
// true before optimized MethodJIT code may be installed or reused.
type CompilationDependencyKind string

const (
	CompilationDependencyTableShape CompilationDependencyKind = "table-shape"
	CompilationDependencyMetatable  CompilationDependencyKind = "metatable"
	CompilationDependencyGlobal     CompilationDependencyKind = "global"
	CompilationDependencyCallABI    CompilationDependencyKind = "call-abi"
)

// CompilationDependencyContext supplies validation-time state. Most dependency
// records carry their own snapshot; the context provides mutable VM-level
// lookup surfaces that are intentionally not owned by the compiler.
type CompilationDependencyContext struct {
	Globals CompilationGlobalLookup
}

// CompilationGlobalLookup is the small VM surface needed to validate global
// assumptions without tying the registry to a concrete TieringManager.
type CompilationGlobalLookup interface {
	GetGlobal(name string) runtime.Value
}

type compilationGlobalValueVersioned interface {
	GlobalValueVersion(name string) (uint64, bool)
}

type compilationGlobalIndexedValueVersioned interface {
	GlobalIndex(name string) (int, bool)
	GlobalValueVersionByIndex(idx int) (uint64, bool)
}

// CompilationDependency is a single recorded assumption. Implementations should
// be immutable snapshots so registry de-duplication can be key based.
type CompilationDependency interface {
	Kind() CompilationDependencyKind
	Key() string
	Validate(ctx CompilationDependencyContext) error
	String() string
}

// CompilationDependencyInvalidation is the structured reason a dependency set
// can no longer be used for an optimized compilation artifact.
type CompilationDependencyInvalidation struct {
	Dependency CompilationDependency
	Kind       CompilationDependencyKind
	Key        string
	Reason     string
	Err        error
}

func newCompilationDependencyInvalidation(dep CompilationDependency, err error) *CompilationDependencyInvalidation {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return &CompilationDependencyInvalidation{
		Dependency: dep,
		Kind:       dep.Kind(),
		Key:        dep.Key(),
		Reason:     reason,
		Err:        err,
	}
}

func (i *CompilationDependencyInvalidation) Error() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%s dependency %s invalid: %s", i.Kind, i.Key, i.Reason)
}

func (i *CompilationDependencyInvalidation) Unwrap() error {
	if i == nil {
		return nil
	}
	return i.Err
}

// CompilationDependencyCommitter is the installation hook. A future Tier 2
// manager can use it to attach validated dependencies to compiled code for
// invalidation; tests and dry runs may pass nil to only validate.
type CompilationDependencyCommitter interface {
	CommitCompilationDependencies([]CompilationDependency) error
}

// CompilationDependencyRegistry records dependencies during compilation and
// validates them as a single commit step before installation.
type CompilationDependencyRegistry struct {
	deps   []CompilationDependency
	seen   map[compilationDependencyKey]struct{}
	sealed bool
}

type compilationDependencyKey struct {
	kind CompilationDependencyKind
	key  string
}

func NewCompilationDependencyRegistry() *CompilationDependencyRegistry {
	return &CompilationDependencyRegistry{}
}

// Record adds dep unless an equivalent dependency key has already been seen.
// It returns true when dep was added and false for nil or duplicate records.
func (r *CompilationDependencyRegistry) Record(dep CompilationDependency) bool {
	if r == nil || dep == nil || r.sealed {
		return false
	}
	key := compilationDependencyKey{kind: dep.Kind(), key: dep.Key()}
	if r.seen == nil {
		r.seen = make(map[compilationDependencyKey]struct{})
	}
	if _, ok := r.seen[key]; ok {
		return false
	}
	r.seen[key] = struct{}{}
	r.deps = append(r.deps, dep)
	return true
}

func (r *CompilationDependencyRegistry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.deps)
}

func (r *CompilationDependencyRegistry) Dependencies() []CompilationDependency {
	if r == nil || len(r.deps) == 0 {
		return nil
	}
	out := make([]CompilationDependency, len(r.deps))
	copy(out, r.deps)
	return out
}

func (r *CompilationDependencyRegistry) Sealed() bool {
	return r != nil && r.sealed
}

// Validate returns the first invalidated dependency, if any.
func (r *CompilationDependencyRegistry) Validate(ctx CompilationDependencyContext) *CompilationDependencyInvalidation {
	if r == nil {
		return nil
	}
	return ValidateCompilationDependencies(r.deps, ctx)
}

// ValidateCompilationDependencies checks an already-published dependency set
// and returns a structured invalidation reason for the first stale assumption.
func ValidateCompilationDependencies(deps []CompilationDependency, ctx CompilationDependencyContext) *CompilationDependencyInvalidation {
	for _, dep := range deps {
		if dep == nil {
			continue
		}
		if err := dep.Validate(ctx); err != nil {
			return newCompilationDependencyInvalidation(dep, err)
		}
	}
	return nil
}

// CommitOrValidate validates every recorded dependency and, if committer is
// non-nil, publishes the validated records. A failed validation leaves the
// registry open so callers can discard it without publishing stale code.
func (r *CompilationDependencyRegistry) CommitOrValidate(ctx CompilationDependencyContext, committer CompilationDependencyCommitter) error {
	if r == nil {
		return nil
	}
	if invalid := r.Validate(ctx); invalid != nil {
		return invalid
	}
	if committer != nil {
		if err := committer.CommitCompilationDependencies(r.Dependencies()); err != nil {
			return fmt.Errorf("commit compilation dependencies: %w", err)
		}
	}
	r.sealed = true
	return nil
}

func (r *CompilationDependencyRegistry) RecordTableShape(shapeID uint32) bool {
	return r.Record(TableShapeDependency{
		ShapeID:     shapeID,
		LayoutEpoch: runtime.ShapeLayoutMutationCount(shapeID),
	})
}

func (r *CompilationDependencyRegistry) RecordShapeField(shapeID uint32, fieldIdx int) bool {
	return r.Record(TableShapeDependency{
		ShapeID:       shapeID,
		FieldIdx:      fieldIdx,
		LayoutEpoch:   runtime.ShapeLayoutMutationCount(shapeID),
		FieldEpoch:    runtime.ShapeFieldMutationCount(shapeID, fieldIdx),
		HasFieldEpoch: true,
	})
}

func (r *CompilationDependencyRegistry) RecordNoMetatable(table *runtime.Table) bool {
	return r.Record(NoMetatableDependency{Table: table})
}

func (r *CompilationDependencyRegistry) RecordGlobalValue(globals CompilationGlobalLookup, name string) bool {
	dep := GlobalValueDependency{Name: name}
	if globals != nil {
		dep.Value = globals.GetGlobal(name)
		if versioned, ok := globals.(compilationGlobalValueVersioned); ok {
			if version, ok := versioned.GlobalValueVersion(name); ok {
				dep.Version = version
				dep.HasVersion = true
			}
		}
		if indexed, ok := globals.(compilationGlobalIndexedValueVersioned); ok {
			if idx, ok := indexed.GlobalIndex(name); ok {
				dep.Index = idx
				dep.HasIndex = true
			}
		}
	}
	return r.Record(dep)
}

func (r *CompilationDependencyRegistry) RecordCallABI(callID int, desc CallABIDescriptor) bool {
	return r.Record(NewCallABIDependency(callID, desc))
}

// TableShapeDependency snapshots a shape layout epoch, and optionally one
// field-level value mutation epoch for future field load/method load guards.
type TableShapeDependency struct {
	ShapeID       uint32
	FieldIdx      int
	LayoutEpoch   uint64
	FieldEpoch    uint64
	HasFieldEpoch bool
}

func (d TableShapeDependency) Kind() CompilationDependencyKind {
	return CompilationDependencyTableShape
}

func (d TableShapeDependency) Key() string {
	if d.HasFieldEpoch {
		return fmt.Sprintf("shape=%d/field=%d", d.ShapeID, d.FieldIdx)
	}
	return fmt.Sprintf("shape=%d/layout", d.ShapeID)
}

func (d TableShapeDependency) Validate(CompilationDependencyContext) error {
	if d.ShapeID == 0 || runtime.LookupShapeByID(d.ShapeID) == nil {
		return fmt.Errorf("shape %d is not registered", d.ShapeID)
	}
	if got := runtime.ShapeLayoutMutationCount(d.ShapeID); got != d.LayoutEpoch {
		return fmt.Errorf("layout epoch changed from %d to %d", d.LayoutEpoch, got)
	}
	if d.HasFieldEpoch {
		if got := runtime.ShapeFieldMutationCount(d.ShapeID, d.FieldIdx); got != d.FieldEpoch {
			return fmt.Errorf("field %d epoch changed from %d to %d", d.FieldIdx, d.FieldEpoch, got)
		}
	}
	return nil
}

func (d TableShapeDependency) String() string {
	return d.Key()
}

// NoMetatableDependency captures the common fast-path assumption that a table
// has no metatable, so ordinary table operations cannot dispatch metamethods.
type NoMetatableDependency struct {
	Table *runtime.Table
}

func (d NoMetatableDependency) Kind() CompilationDependencyKind {
	return CompilationDependencyMetatable
}

func (d NoMetatableDependency) Key() string {
	return fmt.Sprintf("table=%p/no-metatable", d.Table)
}

func (d NoMetatableDependency) Validate(CompilationDependencyContext) error {
	if d.Table == nil {
		return fmt.Errorf("nil table")
	}
	if d.Table.HasMetatable() {
		return fmt.Errorf("table gained metatable")
	}
	return nil
}

func (d NoMetatableDependency) String() string {
	return d.Key()
}

// GlobalValueDependency snapshots one global binding. When the VM exposes a
// per-binding epoch, validation can ignore writes to unrelated globals while
// still invalidating on any write to the named binding, including write-same-
// value cases that may matter for host interop and native caches.
type GlobalValueDependency struct {
	Name       string
	Value      runtime.Value
	Version    uint64
	Index      int
	HasVersion bool
	HasIndex   bool
}

func (d GlobalValueDependency) Kind() CompilationDependencyKind {
	return CompilationDependencyGlobal
}

func (d GlobalValueDependency) Key() string {
	return "global=" + d.Name
}

func (d GlobalValueDependency) Validate(ctx CompilationDependencyContext) error {
	if ctx.Globals == nil {
		return fmt.Errorf("no global lookup context")
	}
	if d.HasVersion {
		if d.HasIndex {
			if indexed, ok := ctx.Globals.(compilationGlobalIndexedValueVersioned); ok {
				gotVersion, ok := indexed.GlobalValueVersionByIndex(d.Index)
				if !ok {
					return fmt.Errorf("global %q version unavailable", d.Name)
				}
				if gotVersion != d.Version {
					return fmt.Errorf("global %q version changed from %d to %d", d.Name, d.Version, gotVersion)
				}
				return nil
			}
		}
		versioned, ok := ctx.Globals.(compilationGlobalValueVersioned)
		if !ok {
			return fmt.Errorf("no global version context")
		}
		gotVersion, ok := versioned.GlobalValueVersion(d.Name)
		if !ok {
			return fmt.Errorf("global %q version unavailable", d.Name)
		}
		if gotVersion != d.Version {
			return fmt.Errorf("global %q version changed from %d to %d", d.Name, d.Version, gotVersion)
		}
		return nil
	}
	if got := ctx.Globals.GetGlobal(d.Name); got != d.Value {
		return fmt.Errorf("global %q changed from %v to %v", d.Name, d.Value, got)
	}
	return nil
}

func (d GlobalValueDependency) String() string {
	return d.Key()
}

// CallABIDependency snapshots the callsite ABI contract that native call code
// plans to use. This intentionally validates only stable descriptor structure
// plus optional published typed-entry ABI signature; pass-specific proof still
// belongs in the producing pass.
type CallABIDependency struct {
	CallID               int
	Callee               *vm.FuncProto
	NumArgs              int
	NumRets              int
	RawIntParams         []bool
	RawIntReturn         bool
	TypedPeer            bool
	ParamReps            []SpecializedABIParamRep
	ReturnRep            SpecializedABIReturnRep
	ExpectedTypedABI     uint64
	RequireTypedABIMatch bool
}

func NewCallABIDependency(callID int, desc CallABIDescriptor) CallABIDependency {
	dep := CallABIDependency{
		CallID:       callID,
		Callee:       desc.Callee,
		NumArgs:      desc.NumArgs,
		NumRets:      desc.NumRets,
		RawIntParams: append([]bool(nil), desc.RawIntParams...),
		RawIntReturn: desc.RawIntReturn,
		TypedPeer:    desc.TypedPeer,
		ParamReps:    append([]SpecializedABIParamRep(nil), desc.ParamReps...),
		ReturnRep:    desc.ReturnRep,
	}
	if desc.TypedPeer && desc.Callee != nil && desc.Callee.Tier2TypedEntryABI != 0 {
		dep.ExpectedTypedABI = desc.Callee.Tier2TypedEntryABI
		dep.RequireTypedABIMatch = true
	}
	return dep
}

func (d CallABIDependency) Kind() CompilationDependencyKind {
	return CompilationDependencyCallABI
}

func (d CallABIDependency) Key() string {
	return fmt.Sprintf("call=%d/callee=%p", d.CallID, d.Callee)
}

func (d CallABIDependency) Validate(CompilationDependencyContext) error {
	if d.CallID < 0 {
		return fmt.Errorf("negative call id")
	}
	if d.Callee == nil {
		return fmt.Errorf("nil callee")
	}
	if d.NumArgs < 0 || d.NumRets < 0 {
		return fmt.Errorf("negative arity")
	}
	if len(d.RawIntParams) != 0 && len(d.RawIntParams) != d.NumArgs {
		return fmt.Errorf("raw-int param count %d does not match NumArgs %d", len(d.RawIntParams), d.NumArgs)
	}
	if d.TypedPeer {
		if len(d.ParamReps) != d.NumArgs {
			return fmt.Errorf("typed-peer param rep count %d does not match NumArgs %d", len(d.ParamReps), d.NumArgs)
		}
		if d.RequireTypedABIMatch && d.Callee.Tier2TypedEntryABI != d.ExpectedTypedABI {
			return fmt.Errorf("typed entry ABI changed from %#x to %#x", d.ExpectedTypedABI, d.Callee.Tier2TypedEntryABI)
		}
	}
	return nil
}

func (d CallABIDependency) String() string {
	return d.Key()
}

func (cf *CompiledFunction) ValidateCompilationDependencies(ctx CompilationDependencyContext) *CompilationDependencyInvalidation {
	if cf == nil {
		return nil
	}
	return ValidateCompilationDependencies(cf.CompilationDependencies, ctx)
}

func (cf *CompiledFunction) DependencyInvalidationReason(ctx CompilationDependencyContext) (string, bool) {
	invalid := cf.ValidateCompilationDependencies(ctx)
	if invalid == nil {
		return "", false
	}
	return invalid.Error(), true
}
