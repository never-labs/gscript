package methodjit

import (
	"errors"
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

type recordingDependencyCommitter struct {
	records []CompilationDependency
}

func (c *recordingDependencyCommitter) CommitCompilationDependencies(records []CompilationDependency) error {
	c.records = append([]CompilationDependency(nil), records...)
	return nil
}

func TestCompilationDependencyRegistryRecordsDedupesAndCommits(t *testing.T) {
	tbl := runtime.NewTable()
	tbl.RawSetString("x", runtime.IntValue(1))
	tbl.RawSetString("y", runtime.IntValue(2))

	v := vm.New(map[string]runtime.Value{"limit": runtime.IntValue(10)})
	defer v.Close()

	callee := &vm.FuncProto{
		Name:               "callee",
		NumParams:          1,
		Tier2TypedEntryABI: 0x1234,
	}
	desc := CallABIDescriptor{
		Callee:    callee,
		NumArgs:   1,
		NumRets:   1,
		TypedPeer: true,
		ParamReps: []SpecializedABIParamRep{SpecializedABIParamRawInt},
		ReturnRep: SpecializedABIReturnRawInt,
	}

	reg := NewCompilationDependencyRegistry()
	if !reg.RecordTableShape(tbl.ShapeID()) {
		t.Fatalf("expected first table shape dependency to be recorded")
	}
	if reg.RecordTableShape(tbl.ShapeID()) {
		t.Fatalf("duplicate table shape dependency should be ignored")
	}
	if !reg.RecordShapeField(tbl.ShapeID(), 0) {
		t.Fatalf("expected field dependency to be recorded")
	}
	if !reg.RecordNoMetatable(tbl) {
		t.Fatalf("expected metatable dependency to be recorded")
	}
	if !reg.RecordGlobalValue(v, "limit") {
		t.Fatalf("expected global dependency to be recorded")
	}
	if !reg.RecordCallABI(7, desc) {
		t.Fatalf("expected call ABI dependency to be recorded")
	}
	if reg.RecordCallABI(7, desc) {
		t.Fatalf("duplicate call ABI dependency should be ignored")
	}
	if got, want := reg.Len(), 5; got != want {
		t.Fatalf("registry Len() = %d, want %d", got, want)
	}

	committer := &recordingDependencyCommitter{}
	err := reg.CommitOrValidate(CompilationDependencyContext{Globals: v}, committer)
	if err != nil {
		t.Fatalf("CommitOrValidate failed: %v", err)
	}
	if got, want := len(committer.records), 5; got != want {
		t.Fatalf("committed records = %d, want %d", got, want)
	}
}

func TestCompilationDependencyRegistryTableShapeValidationFailsAfterMutation(t *testing.T) {
	tbl := runtime.NewTable()
	tbl.RawSetString("x", runtime.IntValue(1))

	reg := NewCompilationDependencyRegistry()
	reg.RecordTableShape(tbl.ShapeID())
	runtime.RecordShapeLayoutMutation(tbl.ShapeID())

	err := reg.CommitOrValidate(CompilationDependencyContext{}, nil)
	if err == nil {
		t.Fatalf("expected layout mutation to invalidate dependency")
	}
	if !strings.Contains(err.Error(), "layout epoch changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompilationDependencyRegistryShapeFieldValidationFailsAfterMutation(t *testing.T) {
	tbl := runtime.NewTable()
	tbl.RawSetString("x", runtime.IntValue(1))

	reg := NewCompilationDependencyRegistry()
	reg.RecordShapeField(tbl.ShapeID(), 0)
	runtime.RecordShapeFieldMutation(tbl.ShapeID(), 0)

	err := reg.CommitOrValidate(CompilationDependencyContext{}, nil)
	if err == nil {
		t.Fatalf("expected field mutation to invalidate dependency")
	}
	if !strings.Contains(err.Error(), "field 0 epoch changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompilationDependencyRegistryMetatableValidationFails(t *testing.T) {
	tbl := runtime.NewTable()

	reg := NewCompilationDependencyRegistry()
	reg.RecordNoMetatable(tbl)
	tbl.SetMetatable(runtime.NewTable())

	err := reg.CommitOrValidate(CompilationDependencyContext{}, nil)
	if err == nil {
		t.Fatalf("expected metatable dependency to fail")
	}
	if !strings.Contains(err.Error(), "gained metatable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompilationDependencyRegistryGlobalValidationFails(t *testing.T) {
	v := vm.New(map[string]runtime.Value{"limit": runtime.IntValue(10)})
	defer v.Close()

	reg := NewCompilationDependencyRegistry()
	reg.RecordGlobalValue(v, "limit")
	v.SetGlobal("limit", runtime.IntValue(11))

	err := reg.CommitOrValidate(CompilationDependencyContext{Globals: v}, nil)
	if err == nil {
		t.Fatalf("expected global dependency to fail")
	}
	var invalid *CompilationDependencyInvalidation
	if !errors.As(err, &invalid) {
		t.Fatalf("error type = %T, want CompilationDependencyInvalidation", err)
	}
	if invalid.Kind != CompilationDependencyGlobal || invalid.Key != "global=limit" || invalid.Reason == "" {
		t.Fatalf("unexpected invalidation: %+v", invalid)
	}
	if !strings.Contains(err.Error(), "global version changed") &&
		!strings.Contains(err.Error(), "global \"limit\" changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompilationDependencyRegistryCallABIValidationFails(t *testing.T) {
	callee := &vm.FuncProto{
		Name:               "callee",
		NumParams:          1,
		Tier2TypedEntryABI: 0x1234,
	}
	desc := CallABIDescriptor{
		Callee:    callee,
		NumArgs:   1,
		NumRets:   1,
		TypedPeer: true,
		ParamReps: []SpecializedABIParamRep{SpecializedABIParamRawInt},
		ReturnRep: SpecializedABIReturnRawInt,
	}

	reg := NewCompilationDependencyRegistry()
	reg.RecordCallABI(3, desc)
	callee.Tier2TypedEntryABI = 0x5678

	err := reg.CommitOrValidate(CompilationDependencyContext{}, nil)
	if err == nil {
		t.Fatalf("expected call ABI dependency to fail")
	}
	if !strings.Contains(err.Error(), "typed entry ABI changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTier2PipelineCommitsEmptyDependencyRegistry(t *testing.T) {
	reg := NewCompilationDependencyRegistry()
	committer := &recordingDependencyCommitter{}

	_, _, err := runTier2PipelineWithPlan(&Function{Analysis: NewAnalysisResult()}, &Tier2PipelineOpts{
		DependencyRegistry:  reg,
		DependencyCommitter: committer,
	}, func(*Tier2OptimizerContext) Tier2OptimizerPlan {
		return Tier2OptimizerPlan{}
	})
	if err != nil {
		t.Fatalf("runTier2PipelineWithPlan failed: %v", err)
	}
	if got := len(committer.records); got != 0 {
		t.Fatalf("committed records = %d, want 0", got)
	}
	if !reg.Sealed() {
		t.Fatalf("registry should be sealed after pipeline dependency commit")
	}
	if reg.RecordNoMetatable(runtime.NewTable()) {
		t.Fatalf("registry should be sealed after pipeline dependency commit")
	}
}

func TestRunTier2PipelineCommitsDependencyRecordedByModule(t *testing.T) {
	tbl := runtime.NewTable()
	reg := NewCompilationDependencyRegistry()
	committer := &recordingDependencyCommitter{}

	_, _, err := runTier2PipelineWithPlan(&Function{Analysis: NewAnalysisResult()}, &Tier2PipelineOpts{
		DependencyRegistry:  reg,
		DependencyCommitter: committer,
	}, func(*Tier2OptimizerContext) Tier2OptimizerPlan {
		return Tier2OptimizerPlan{
			PhaseGroups: []Tier2OptimizerPhaseGroup{{
				Phase: Tier2PhaseEarlyCanonical,
				Modules: []Tier2OptimizerModule{{
					Name:  "RecordTestDependency",
					Phase: Tier2PhaseEarlyCanonical,
					RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
						if ctxDependencyRegistry(ctx) != reg {
							t.Fatalf("context dependency registry was not propagated")
						}
						ctxDependencyRegistry(ctx).RecordNoMetatable(tbl)
						return fn, nil
					},
				}},
			}},
		}
	})
	if err != nil {
		t.Fatalf("runTier2PipelineWithPlan failed: %v", err)
	}
	if got, want := len(committer.records), 1; got != want {
		t.Fatalf("committed records = %d, want %d", got, want)
	}
	if committer.records[0].Kind() != CompilationDependencyMetatable {
		t.Fatalf("committed dependency kind = %s, want %s", committer.records[0].Kind(), CompilationDependencyMetatable)
	}
}

func TestRunTier2PipelineDependencyValidationFailureIsClear(t *testing.T) {
	tbl := runtime.NewTable()
	reg := NewCompilationDependencyRegistry()

	_, _, err := runTier2PipelineWithPlan(&Function{Analysis: NewAnalysisResult()}, &Tier2PipelineOpts{
		DependencyRegistry: reg,
	}, func(*Tier2OptimizerContext) Tier2OptimizerPlan {
		return Tier2OptimizerPlan{
			PhaseGroups: []Tier2OptimizerPhaseGroup{{
				Phase: Tier2PhaseEarlyCanonical,
				Modules: []Tier2OptimizerModule{{
					Name:  "RecordInvalidTestDependency",
					Phase: Tier2PhaseEarlyCanonical,
					RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
						ctxDependencyRegistry(ctx).RecordNoMetatable(tbl)
						tbl.SetMetatable(runtime.NewTable())
						return fn, nil
					},
				}},
			}},
		}
	})
	if err == nil {
		t.Fatalf("expected dependency validation failure")
	}
	if !strings.Contains(err.Error(), "tier2 dependency validation") ||
		!strings.Contains(err.Error(), "metatable dependency") ||
		!strings.Contains(err.Error(), "gained metatable") {
		t.Fatalf("unexpected error: %v", err)
	}
	var invalid *CompilationDependencyInvalidation
	if !errors.As(err, &invalid) {
		t.Fatalf("error type = %T, want CompilationDependencyInvalidation", err)
	}
	if invalid.Kind != CompilationDependencyMetatable || invalid.Reason != "table gained metatable" {
		t.Fatalf("unexpected invalidation: %+v", invalid)
	}
}

func TestCompiledFunctionDependencyInvalidationReason(t *testing.T) {
	tbl := runtime.NewTable()
	cf := &CompiledFunction{
		CompilationDependencies: []CompilationDependency{NoMetatableDependency{Table: tbl}},
	}

	if reason, stale := cf.DependencyInvalidationReason(CompilationDependencyContext{}); stale || reason != "" {
		t.Fatalf("fresh dependencies reported stale=%v reason=%q", stale, reason)
	}

	tbl.SetMetatable(runtime.NewTable())
	reason, stale := cf.DependencyInvalidationReason(CompilationDependencyContext{})
	if !stale {
		t.Fatalf("expected stale dependency")
	}
	if !strings.Contains(reason, "metatable dependency") || !strings.Contains(reason, "gained metatable") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestTieringManagerExplicitDependencyInvalidationClearsCompiledEntry(t *testing.T) {
	tm := NewTieringManager()
	proto := &vm.FuncProto{Name: "stale"}
	tbl := runtime.NewTable()
	cf := &CompiledFunction{
		Proto: proto,
		CompilationDependencies: []CompilationDependency{
			NoMetatableDependency{Table: tbl},
		},
	}
	tm.markTier2Compiled(proto, cf)

	if invalidated, reason := tm.InvalidateTier2CompiledDependencies(proto, CompilationDependencyContext{}); invalidated || reason != "" {
		t.Fatalf("fresh dependencies invalidated=%v reason=%q", invalidated, reason)
	}

	tbl.SetMetatable(runtime.NewTable())
	invalidated, reason := tm.InvalidateTier2CompiledDependencies(proto, CompilationDependencyContext{})
	if !invalidated {
		t.Fatalf("expected compiled entry to be invalidated")
	}
	if !strings.Contains(reason, "metatable dependency") || !strings.Contains(reason, "gained metatable") {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if _, ok := tm.tier2CompiledFor(proto); ok {
		t.Fatalf("compiled entry was not cleared")
	}
}
