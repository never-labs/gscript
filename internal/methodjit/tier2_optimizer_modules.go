package methodjit

import (
	"fmt"
	"time"

	"github.com/gscript/gscript/internal/vm"
)

// Tier2SnapshotCallback is called after each successful optimizer module run,
// for IR dump/snapshot. New diagnostics that need failure details should use
// Tier2ModuleRunCallback.
type Tier2SnapshotCallback func(moduleName string, fn *Function, duration time.Duration)

// Tier2ModuleRun records one optimizer module execution scope.
type Tier2ModuleRun struct {
	Phase      Tier2OptimizerPhase
	ModuleName string
	StageName  string
	Function   *Function
	Duration   time.Duration
	Err        error
}

// Tier2ModuleRunCallback is called after each optimizer module run, including
// failed modules.
type Tier2ModuleRunCallback func(Tier2ModuleRun)

// Tier2OptimizerPhase names a coarse-grained extension point in the Tier 2
// optimizer. New native optimizations should enter through a phase module
// instead of being inserted ad hoc into RunTier2Pipeline.
type Tier2OptimizerPhase string

const (
	// Tier2PhaseEarlyCanonical runs initial simplification, type specialization,
	// intrinsic rewriting, and pre-inline fixed-shape table analysis.
	Tier2PhaseEarlyCanonical Tier2OptimizerPhase = "early_canonical"

	// Tier2PhaseInlineCall performs call inlining and post-inline cleanup passes.
	Tier2PhaseInlineCall Tier2OptimizerPhase = "inline_call"

	// Tier2PhaseCallLower annotates callsites with ABI facts and folds
	// protocol-level constant calls.
	Tier2PhaseCallLower Tier2OptimizerPhase = "call_lower"

	// Tier2PhaseStringNative rewrites string intrinsics into native IR ops.
	Tier2PhaseStringNative Tier2OptimizerPhase = "string_native"

	// Tier2PhaseTableObjectPrep runs table allocation hints, fixed-shape analysis,
	// load elimination, escape analysis, and scalar replacement.
	Tier2PhaseTableObjectPrep Tier2OptimizerPhase = "table_object_prep"

	// Tier2PhasePostRewrite runs cleanup after escape analysis rewrites.
	Tier2PhasePostRewrite Tier2OptimizerPhase = "post_rewrite"

	// Tier2PhaseNumeric runs integer range analysis, overflow boxing, division
	// simplification, and branch threading.
	Tier2PhaseNumeric Tier2OptimizerPhase = "numeric"

	// Tier2PhaseTableArrayLower lowers table array accesses to native ops.
	Tier2PhaseTableArrayLower Tier2OptimizerPhase = "table_array_lower"

	// Tier2PhaseMatrixNative lowers dense matrix ops to native stride/load ops.
	Tier2PhaseMatrixNative Tier2OptimizerPhase = "matrix_native"

	// Tier2PhaseTableFieldLower lowers field accesses to native SVALS ops and
	// runs post-lowering range analysis.
	Tier2PhaseTableFieldLower Tier2OptimizerPhase = "table_field_lower"

	// Tier2PhaseFloatNumeric runs FMA fusion and float strength reduction.
	Tier2PhaseFloatNumeric Tier2OptimizerPhase = "float_numeric"

	// Tier2PhaseLoopKernel runs LICM, loop-global store sinking, table loop
	// kernels, and post-LICM load elimination.
	Tier2PhaseLoopKernel Tier2OptimizerPhase = "loop_kernel"

	// Tier2PhaseLoopPost runs loop unrolling, quadratic strength reduction,
	// and scalar promotion.
	Tier2PhaseLoopPost Tier2OptimizerPhase = "loop_post"

	// Tier2PhaseFinalCall re-runs call ABI annotation and final-range analysis
	// after all other lowering is complete.
	Tier2PhaseFinalCall Tier2OptimizerPhase = "final_call"
)

// Tier2OptimizerContext carries per-compilation context shared across modules
// within a single Tier 2 optimization run. It is created by RunTier2Pipeline
// and passed to modules that declare RunWithContext instead of Run.
type Tier2OptimizerContext struct {
	// Globals maps global function names to their protos, used for inlining
	// and call ABI annotation.
	Globals map[string]*vm.FuncProto
	// ProtocolGlobals maps stable protocol globals available for guarded folds.
	ProtocolGlobals map[string]*vm.FuncProto
	// IntrinsicNotes collects human-readable notes from intrinsic rewrites.
	IntrinsicNotes []string
	// InlineApplied reports whether the inline pass made changes.
	InlineApplied bool
	// InlineMaxSize is the maximum callee bytecode count for inlining (default 40).
	InlineMaxSize int
	// SnapshotCallback is an optional callback invoked after each module for
	// IR dump/snapshot. The duration is the time the module took to execute.
	SnapshotCallback Tier2SnapshotCallback
	// ModuleRunCallback is an optional structured callback invoked after each
	// module run, including failures.
	ModuleRunCallback Tier2ModuleRunCallback
}

// Tier2OptimizerModule is the smallest pluggable optimization unit. Modules
// are intentionally plain functions with metadata: this keeps hot code out of
// interfaces while giving diagnostics, ordering, and future feature switches a
// single place to hook into.
//
// Modules within a phase execute in registration order (the Order field is
// reserved for future stable intra-phase sorting).
type Tier2OptimizerModule struct {
	Name           string
	Phase          Tier2OptimizerPhase
	Order          int            // reserved for stable intra-phase ordering (future use)
	Requires       []AnalysisFact // Facts this module needs to be already computed
	Provides       []AnalysisFact // Facts this module computes
	Run            func(*Function, *Tier2PipelineOpts) (*Function, error)
	RunWithContext func(*Function, *Tier2PipelineOpts, *Tier2OptimizerContext) (*Function, error)
}

func tier2PassModule(name string, phase Tier2OptimizerPhase, pass PassFunc) Tier2OptimizerModule {
	return Tier2OptimizerModule{
		Name:  name,
		Phase: phase,
		Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
			return pass(fn)
		},
	}
}

func tier2PassModuleWith(name string, phase Tier2OptimizerPhase, requires, provides []AnalysisFact, pass PassFunc) Tier2OptimizerModule {
	return Tier2OptimizerModule{
		Name:     name,
		Phase:    phase,
		Requires: requires,
		Provides: provides,
		Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
			return pass(fn)
		},
	}
}

// Tier2OptimizerPhaseGroup is a stable execution unit in the optimizer plan.
// Builders may contribute modules to the same phase; the phase itself must
// still execute once.
type Tier2OptimizerPhaseGroup struct {
	Phase   Tier2OptimizerPhase
	Modules []Tier2OptimizerModule
}

// Tier2OptimizerPlan is the fully constructed optimization plan. PhaseGroups
// is the canonical execution shape; Phases and Modules are retained as flat
// views for tests, diagnostics, and compatibility with older helpers.
type Tier2OptimizerPlan struct {
	Phases      []Tier2OptimizerPhase
	Modules     []Tier2OptimizerModule
	PhaseGroups []Tier2OptimizerPhaseGroup
}

func newTier2OptimizerPlan(ctx *Tier2OptimizerContext) Tier2OptimizerPlan {
	return BuildModulePlan(ctx)
}

func runTier2OptimizerPlan(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext, plan Tier2OptimizerPlan) (*Function, error) {
	if err := ValidateDependencyOrder(plan); err != nil {
		return nil, fmt.Errorf("tier2 optimizer dependency validation: %w", err)
	}
	var err error
	for _, group := range plan.phaseGroups() {
		fn, err = runTier2OptimizerModulesWithContext(fn, opts, ctx, group.Phase, group.Modules)
		if err != nil {
			return nil, err
		}
	}
	return fn, nil
}

func (plan Tier2OptimizerPlan) phaseGroups() []Tier2OptimizerPhaseGroup {
	if len(plan.PhaseGroups) > 0 {
		return plan.PhaseGroups
	}
	groups := make([]Tier2OptimizerPhaseGroup, 0, len(plan.Phases))
	seen := make(map[Tier2OptimizerPhase]bool, len(plan.Phases))
	for _, phase := range plan.Phases {
		if seen[phase] {
			continue
		}
		seen[phase] = true
		group := Tier2OptimizerPhaseGroup{Phase: phase}
		for _, module := range plan.Modules {
			if module.Phase == phase {
				group.Modules = append(group.Modules, module)
			}
		}
		groups = append(groups, group)
	}
	return groups
}

func runTier2OptimizerModulesWithContext(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext, phase Tier2OptimizerPhase, modules []Tier2OptimizerModule) (*Function, error) {
	var err error
	for _, module := range modules {
		if module.Phase != phase {
			continue
		}
		if module.RunWithContext == nil && module.Run == nil {
			return nil, fmt.Errorf("%s: missing optimizer module runner", module.Name)
		}
		stageName := tier2OptimizerModuleStageName(phase, module.Name)
		start := time.Now()
		if module.RunWithContext != nil {
			fn, err = module.RunWithContext(fn, opts, ctx)
		} else {
			fn, err = module.Run(fn, opts)
		}
		duration := time.Since(start)
		if opts != nil && opts.OptimizerTimings != nil {
			*opts.OptimizerTimings = append(*opts.OptimizerTimings, newNestedPipelineStageTiming(stageName, duration, err))
		}
		if ctx != nil && ctx.ModuleRunCallback != nil {
			ctx.ModuleRunCallback(Tier2ModuleRun{
				Phase:      phase,
				ModuleName: module.Name,
				StageName:  stageName,
				Function:   fn,
				Duration:   duration,
				Err:        err,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", module.Name, err)
		}
		attachRemarks(fn, opts)

		// Invoke snapshot callback if set.
		if ctx != nil && ctx.SnapshotCallback != nil {
			ctx.SnapshotCallback(module.Name, fn, duration)
		}
	}
	return fn, nil
}

func tier2OptimizerModuleStageName(phase Tier2OptimizerPhase, moduleName string) string {
	return fmt.Sprintf("RunTier2Pipeline/%s/%s", phase, moduleName)
}

func ctxGlobals(ctx *Tier2OptimizerContext) map[string]*vm.FuncProto {
	if ctx == nil {
		return nil
	}
	return ctx.Globals
}

func ctxProtocolGlobals(ctx *Tier2OptimizerContext) map[string]*vm.FuncProto {
	if ctx == nil {
		return nil
	}
	return ctx.ProtocolGlobals
}

func ctxInlineMaxSize(ctx *Tier2OptimizerContext) int {
	if ctx == nil || ctx.InlineMaxSize <= 0 {
		return 40
	}
	return ctx.InlineMaxSize
}
