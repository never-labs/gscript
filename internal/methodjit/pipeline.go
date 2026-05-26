// pipeline.go implements the optimization pass pipeline for the Method JIT.
package methodjit

import (
	"fmt"
	"strings"
	"time"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// PassFunc is the signature for an optimization pass.
// Takes a Function, returns a (possibly modified) Function and an error.
// Passes MUST NOT modify the input Function in place — return a new one
// or return the same pointer if no changes were made.
type PassFunc func(*Function) (*Function, error)

// PipelineStageTiming records one observed pipeline stage or pass.
type PipelineStageTiming struct {
	Name          string `json:"name"`
	DurationNanos int64  `json:"duration_nanos"`
	Error         string `json:"error,omitempty"`
	Nested        bool   `json:"nested,omitempty"`
}

func newPipelineStageTiming(name string, duration time.Duration, err error) PipelineStageTiming {
	timing := PipelineStageTiming{
		Name:          name,
		DurationNanos: int64(duration),
	}
	if err != nil {
		timing.Error = err.Error()
	}
	return timing
}

func newNestedPipelineStageTiming(name string, duration time.Duration, err error) PipelineStageTiming {
	timing := newPipelineStageTiming(name, duration, err)
	timing.Nested = true
	return timing
}

// FormatPipelineStageTimings returns a compact, human-readable timing summary.
func FormatPipelineStageTimings(stages []PipelineStageTiming) string {
	if len(stages) == 0 {
		return "(not recorded)\n"
	}
	var total time.Duration
	for _, stage := range stages {
		if stage.Nested {
			continue
		}
		total += time.Duration(stage.DurationNanos)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "total: %s (%d stages)\n", total, len(stages))
	for _, stage := range stages {
		fmt.Fprintf(&sb, "  %-32s %s", stage.Name, time.Duration(stage.DurationNanos))
		if stage.Error != "" {
			fmt.Fprintf(&sb, " error=%q", stage.Error)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Production Tier 2 pipeline helpers
// ---------------------------------------------------------------------------

// Tier2PipelineOpts configures the production Tier 2 optimization pipeline.
// A nil *Tier2PipelineOpts uses defaults (MaxSize 40, no globals).
type Tier2PipelineOpts struct {
	InlineGlobals                   map[string]*vm.FuncProto       // global function protos for inlining
	SpecializationGlobals           map[string]*vm.FuncProto       // stable globals available for guarded runtime-specialization folds
	GlobalConstValues               map[int]runtime.Value          // const-pool global name index -> observed numeric value
	InlineMaxSize                   int                            // max callee bytecode count; 0 → 40
	FixedShapeArgFacts              map[int]FixedShapeTableFact    // guarded fixed-shape facts for callee params
	FixedShapeArgPolyFacts          map[int][]FixedShapeTableFact  // guarded polymorphic facts for callee params
	FixedShapeArrayElementArgFacts  map[int]FixedShapeTableFact    // guarded fixed-shape facts for callee param array elements
	FixedShapeArrayElementPolyFacts map[int][]FixedShapeTableFact  // guarded polymorphic facts for callee param array elements
	GlobalArrayElementFacts         map[string]FixedShapeTableFact // guarded global table array-element facts learned by other protos
	FixedShapeEntryGuards           bool                           // emit callee-entry shape guards for FixedShapeArgFacts
	ForceBoxIntIDs                  map[int]bool                   // IR value IDs forced out of raw-int after overflow feedback
	Remarks                         *OptimizationRemarks           // optional structured optimization diagnostics
	OptimizerTimings                *[]PipelineStageTiming         // optional per-module compile diagnostics
	DependencyRegistry              *CompilationDependencyRegistry // optional dependency recorder for compile-time assumptions
	DependencyContext               CompilationDependencyContext   // validation-time state for dependency commit
	DependencyCommitter             CompilationDependencyCommitter // optional dependency publication hook
	VerifyIR                        bool                           // run lightweight IR verification after each optimizer module
	ModuleRuns                      *[]Tier2ModuleRun              // optional per-module contract/run diagnostics
	ModuleRunCallback               Tier2ModuleRunCallback         // optional per-module observer
	ModuleRegistry                  *ModuleRegistry                // optional optimizer module registry; nil uses DefaultModuleRegistry
	ValidatedPlan                   *Tier2ValidatedOptimizerPlan   // optional pre-built, dependency-validated optimizer plan
	FeatureFlags                    Tier2OptimizerFeatureFlags     // optional phase/module optimizer switches
	LastPassChanged                 bool                           // scratch flag for adjacent optimizer modules
}

// RunTier2Pipeline runs the full production Tier 2 optimization pipeline:
//
//	TypeSpec → Intrinsic → TypeSpec → Inline → TypeSpec → ConstProp →
//	LoadElim → EscapeAnalysis → DCE → PostRewriteTypeSpec →
//	LoopBoundRangeGuard → RangeAnalysis → OverflowBoxing → FMAFusion →
//	FloatStrengthReduction → FMAFusion → LICM → FieldNumToFloatFusion →
//	LoadElim → DCE → UnrollAndJam → RangeAnalysis → DCE
//
// Returns the optimized function, any intrinsic rewrite notes (non-nil means
// the function uses intrinsics that Tier 1 would execute differently), and an
// error if a pass fails.
//
// If opts is nil, defaults are used (MaxSize: 40, no globals).
func RunTier2Pipeline(fn *Function, opts *Tier2PipelineOpts) (*Function, []string, error) {
	return runTier2PipelineWithPlan(fn, opts, newTier2OptimizerPlan)
}

func runTier2PipelineWithPlan(fn *Function, opts *Tier2PipelineOpts, buildPlan func(*Tier2OptimizerContext) Tier2OptimizerPlan) (*Function, []string, error) {
	var err error
	if opts != nil && opts.Remarks != nil {
		fn.Remarks = opts.Remarks
	}

	maxSize := 40
	var globals map[string]*vm.FuncProto
	if opts != nil {
		globals = callABIMergeGlobals(opts.InlineGlobals, opts.SpecializationGlobals)
		fn.Analysis.GlobalFacts().SetNumericGlobalValues(optsNumericGlobalValuesByName(fn, opts))
		fn.Analysis.GlobalFacts().SetGlobalArrayElementFacts(cloneFixedShapeTableFactMap(opts.GlobalArrayElementFacts))
		if opts.InlineMaxSize > 0 {
			maxSize = opts.InlineMaxSize
		}
	}
	specializationGlobals := globals
	if opts != nil && len(opts.SpecializationGlobals) > 0 {
		specializationGlobals = opts.SpecializationGlobals
	}

	data := NewTier2PipelineData()
	if reg := optsDependencyRegistry(opts); reg != nil {
		data.CompilationDependencies = reg
	}
	ctx := newTier2OptimizerContext(data)
	ctx.Globals = globals
	ctx.SpecializationGlobals = specializationGlobals
	ctx.InlineMaxSize = maxSize
	ctx.DependencyRegistry = data.CompilationDependencies
	if opts != nil {
		ctx.ModuleRegistry = opts.ModuleRegistry
		ctx.FeatureFlags = opts.FeatureFlags
	}
	if opts != nil && (opts.ModuleRuns != nil || opts.ModuleRunCallback != nil) {
		data.Diagnostics.ModuleRunCallback = func(run Tier2ModuleRun) {
			if opts.ModuleRuns != nil {
				*opts.ModuleRuns = append(*opts.ModuleRuns, run)
			}
			if opts.ModuleRunCallback != nil {
				opts.ModuleRunCallback(run)
			}
		}
	}

	if buildPlan == nil {
		buildPlan = newTier2OptimizerPlan
	}
	if opts != nil && opts.ValidatedPlan != nil {
		fn, err = runTier2OptimizerValidatedPlan(fn, opts, ctx, opts.ValidatedPlan)
	} else {
		fn, err = runTier2OptimizerPlan(fn, opts, ctx, buildPlan(ctx))
	}
	if err != nil {
		return nil, nil, err
	}
	if err := commitTier2PipelineDependencies(opts, ctx); err != nil {
		return nil, nil, err
	}

	return fn, ctx.IntrinsicNotes, nil
}

func optsDependencyRegistry(opts *Tier2PipelineOpts) *CompilationDependencyRegistry {
	if opts == nil {
		return nil
	}
	return opts.DependencyRegistry
}

func commitTier2PipelineDependencies(opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) error {
	registry := optsDependencyRegistry(opts)
	if registry == nil && ctx != nil {
		registry = ctx.DependencyRegistry
	}
	if registry == nil && ctx != nil && ctx.PipelineData != nil {
		registry = ctx.PipelineData.CompilationDependencies
	}
	if registry == nil {
		return nil
	}
	var depCtx CompilationDependencyContext
	var committer CompilationDependencyCommitter
	if opts != nil {
		depCtx = opts.DependencyContext
		committer = opts.DependencyCommitter
	}
	if err := registry.CommitOrValidate(depCtx, committer); err != nil {
		return fmt.Errorf("tier2 dependency validation: %w", err)
	}
	return nil
}

func optsFixedShapeArgFacts(opts *Tier2PipelineOpts) map[int]FixedShapeTableFact {
	if opts == nil {
		return nil
	}
	return opts.FixedShapeArgFacts
}

func optsFixedShapeArgPolyFacts(opts *Tier2PipelineOpts) map[int][]FixedShapeTableFact {
	if opts == nil {
		return nil
	}
	return opts.FixedShapeArgPolyFacts
}

func optsFixedShapeArrayElementArgFacts(opts *Tier2PipelineOpts) map[int]FixedShapeTableFact {
	if opts == nil {
		return nil
	}
	return opts.FixedShapeArrayElementArgFacts
}

func optsFixedShapeArrayElementPolyFacts(opts *Tier2PipelineOpts) map[int][]FixedShapeTableFact {
	if opts == nil {
		return nil
	}
	return opts.FixedShapeArrayElementPolyFacts
}

func optsFixedShapeEntryGuards(opts *Tier2PipelineOpts) bool {
	return opts != nil && opts.FixedShapeEntryGuards
}

func runPostRewriteTypeSpecialize(fn *Function, opts *Tier2PipelineOpts, stage string) (*Function, error) {
	if !typeSpecializeCouldChange(fn) {
		return fn, nil
	}
	functionRemarks(fn).Add("TypeSpec", "changed", 0, 0, OpNop,
		"reran after "+stage+" rewrite exposed typed SSA values")
	out, err := TypeSpecializePass(fn)
	if err != nil {
		return nil, fmt.Errorf("TypeSpecialize (%s): %w", stage, err)
	}
	attachRemarks(out, opts)
	return out, nil
}

func attachRemarks(fn *Function, opts *Tier2PipelineOpts) {
	if fn != nil && opts != nil && opts.Remarks != nil {
		fn.Remarks = opts.Remarks
	}
}
