package methodjit

import (
	"fmt"
	"strings"
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
	Requires   []AnalysisFact
	Provides   []AnalysisFact
	Updates    []AnalysisFact
}

// Tier2ModuleRunCallback is called after each optimizer module run, including
// failed modules.
type Tier2ModuleRunCallback func(Tier2ModuleRun)

// Tier2PipelineDiagnostics groups per-run diagnostic hooks. The legacy fields
// on Tier2OptimizerContext remain supported for callers that construct the
// context directly.
type Tier2PipelineDiagnostics struct {
	SnapshotCallback  Tier2SnapshotCallback
	ModuleRunCallback Tier2ModuleRunCallback
}

// Tier2ModuleContract is the declared analysis-fact contract for one optimizer
// module. It intentionally reports declarations only; it does not diff or copy
// the full AnalysisResult.
type Tier2ModuleContract struct {
	Phase      Tier2OptimizerPhase `json:"phase"`
	ModuleName string              `json:"module"`
	StageName  string              `json:"stage"`
	Requires   []AnalysisFact      `json:"requires,omitempty"`
	Provides   []AnalysisFact      `json:"provides,omitempty"`
	Updates    []AnalysisFact      `json:"updates,omitempty"`
}

// Tier2PipelineData is the per-compilation data object shared across Tier 2
// optimizer modules. It is deliberately small for now: modules can record
// compile-time dependencies, diagnostics can inspect the constructed plan, and
// future pipeline-owned state has a single home instead of growing ad hoc
// fields on Tier2OptimizerContext.
type Tier2PipelineData struct {
	CompilationDependencies *CompilationDependencyRegistry
	Plan                    *Tier2OptimizerPlan
	ValidatedPlan           *Tier2ValidatedOptimizerPlan
	Diagnostics             Tier2PipelineDiagnostics
}

func NewTier2PipelineData() *Tier2PipelineData {
	return &Tier2PipelineData{
		CompilationDependencies: NewCompilationDependencyRegistry(),
	}
}

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
	// PipelineData is the pipeline-owned per-compilation data shared by
	// optimizer modules.
	PipelineData *Tier2PipelineData
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
	// DependencyRegistry records compile-time assumptions made by optimizer
	// modules and validated before the optimized function leaves the pipeline.
	DependencyRegistry *CompilationDependencyRegistry
	// ModuleRegistry is the source of optimizer modules for this compilation.
	// Nil uses DefaultModuleRegistry.
	ModuleRegistry *ModuleRegistry
	// FeatureFlags enables or disables optimizer phases and modules for this
	// compilation. The zero value leaves the default plan unchanged.
	FeatureFlags Tier2OptimizerFeatureFlags
}

func newTier2OptimizerContext(data *Tier2PipelineData) *Tier2OptimizerContext {
	if data == nil {
		data = NewTier2PipelineData()
	} else if data.CompilationDependencies == nil {
		data.CompilationDependencies = NewCompilationDependencyRegistry()
	}
	return &Tier2OptimizerContext{PipelineData: data}
}

func (ctx *Tier2OptimizerContext) ensurePipelineData() *Tier2PipelineData {
	if ctx == nil {
		return nil
	}
	if ctx.PipelineData == nil {
		ctx.PipelineData = NewTier2PipelineData()
	} else if ctx.PipelineData.CompilationDependencies == nil {
		ctx.PipelineData.CompilationDependencies = NewCompilationDependencyRegistry()
	}
	return ctx.PipelineData
}

// Tier2OptimizerModule is the smallest pluggable optimization unit. Modules
// are intentionally plain functions with metadata: this keeps hot code out of
// interfaces while giving diagnostics, ordering, and future feature switches a
// single place to hook into.
//
// Modules within a phase execute in registration order by default. Non-zero
// Order values opt modules into stable intra-phase sorting ahead of unordered
// modules; duplicate Order values and zero-Order modules keep registration
// order.
type Tier2OptimizerModule struct {
	Name           string
	Phase          Tier2OptimizerPhase
	Order          int            // optional stable intra-phase order; zero leaves the module unordered
	Requires       []AnalysisFact // Facts this module needs to be already computed
	Provides       []AnalysisFact // Facts this module computes
	Updates        []AnalysisFact // Facts this module refreshes after an earlier provider
	Run            func(*Function, *Tier2PipelineOpts) (*Function, error)
	RunWithContext func(*Function, *Tier2PipelineOpts, *Tier2OptimizerContext) (*Function, error)
}

// Tier2OptimizerFeatureFlags controls coarse optimizer participation by phase
// or module name. Disabled phases remove all modules in that phase; disabled
// modules remove only matching module names.
type Tier2OptimizerFeatureFlags struct {
	DisabledPhases  map[Tier2OptimizerPhase]bool
	DisabledModules map[string]bool
}

func (flags Tier2OptimizerFeatureFlags) moduleEnabled(module Tier2OptimizerModule) bool {
	if flags.DisabledPhases != nil && flags.DisabledPhases[module.Phase] {
		return false
	}
	if flags.DisabledModules != nil && flags.DisabledModules[module.Name] {
		return false
	}
	return true
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

func tier2PassModuleWithUpdates(name string, phase Tier2OptimizerPhase, requires, updates []AnalysisFact, pass PassFunc) Tier2OptimizerModule {
	return Tier2OptimizerModule{
		Name:     name,
		Phase:    phase,
		Requires: requires,
		Updates:  updates,
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

// Tier2ValidatedOptimizerPlan is an optimizer plan whose dependency ordering
// has already been checked.
type Tier2ValidatedOptimizerPlan struct {
	plan Tier2OptimizerPlan
}

func (validated *Tier2ValidatedOptimizerPlan) Plan() Tier2OptimizerPlan {
	if validated == nil {
		return Tier2OptimizerPlan{}
	}
	return validated.plan
}

func newTier2OptimizerPlan(ctx *Tier2OptimizerContext) Tier2OptimizerPlan {
	registry := DefaultModuleRegistry
	if ctx != nil && ctx.ModuleRegistry != nil {
		registry = ctx.ModuleRegistry
	}
	plan := registry.BuildModulePlan(ctx)
	if ctx != nil {
		plan = filterTier2OptimizerPlan(plan, ctx.FeatureFlags)
	}
	if ctx != nil {
		data := ctx.ensurePipelineData()
		data.Plan = &plan
	}
	return plan
}

func runTier2OptimizerPlan(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext, plan Tier2OptimizerPlan) (*Function, error) {
	validated, err := validateTier2OptimizerPlanForRun(ctx, plan)
	if err != nil {
		return nil, err
	}
	return runTier2OptimizerValidatedPlan(fn, opts, ctx, validated)
}

func ValidateTier2OptimizerPlan(plan Tier2OptimizerPlan) (*Tier2ValidatedOptimizerPlan, error) {
	if err := ValidateDependencyOrder(plan); err != nil {
		return nil, fmt.Errorf("tier2 optimizer dependency validation: %w", err)
	}
	return &Tier2ValidatedOptimizerPlan{plan: plan}, nil
}

func validateTier2OptimizerPlanForRun(ctx *Tier2OptimizerContext, plan Tier2OptimizerPlan) (*Tier2ValidatedOptimizerPlan, error) {
	validated, err := ValidateTier2OptimizerPlan(plan)
	if err != nil {
		return nil, err
	}
	if data := ctx.ensurePipelineData(); data != nil {
		data.ValidatedPlan = validated
	}
	return validated, nil
}

func runTier2OptimizerValidatedPlan(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext, validated *Tier2ValidatedOptimizerPlan) (*Function, error) {
	if validated == nil {
		return nil, fmt.Errorf("tier2 optimizer validated plan is nil")
	}
	plan := validated.plan
	if ctx == nil {
		ctx = newTier2OptimizerContext(nil)
	}
	if data := ctx.ensurePipelineData(); data != nil {
		data.Plan = &plan
		data.ValidatedPlan = validated
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

func filterTier2OptimizerPlan(plan Tier2OptimizerPlan, flags Tier2OptimizerFeatureFlags) Tier2OptimizerPlan {
	if flags.DisabledPhases == nil && flags.DisabledModules == nil {
		return plan
	}
	filtered := Tier2OptimizerPlan{
		Phases:      make([]Tier2OptimizerPhase, 0, len(plan.Phases)),
		Modules:     make([]Tier2OptimizerModule, 0, len(plan.Modules)),
		PhaseGroups: make([]Tier2OptimizerPhaseGroup, 0, len(plan.PhaseGroups)),
	}
	for _, group := range plan.phaseGroups() {
		filteredGroup := Tier2OptimizerPhaseGroup{
			Phase:   group.Phase,
			Modules: make([]Tier2OptimizerModule, 0, len(group.Modules)),
		}
		for _, module := range group.Modules {
			if flags.moduleEnabled(module) {
				filteredGroup.Modules = append(filteredGroup.Modules, module)
				filtered.Modules = append(filtered.Modules, module)
			}
		}
		if len(filteredGroup.Modules) == 0 {
			continue
		}
		filtered.Phases = append(filtered.Phases, filteredGroup.Phase)
		filtered.PhaseGroups = append(filtered.PhaseGroups, filteredGroup)
	}
	return filtered
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
		if err == nil && tier2VerifyIROptEnabled(opts) {
			if verifyErr := VerifyIRLightweight(fn); verifyErr != nil {
				err = fmt.Errorf("%s: lightweight IR verifier failed: %w", stageName, verifyErr)
			}
		}
		duration := time.Since(start)
		if opts != nil && opts.OptimizerTimings != nil {
			*opts.OptimizerTimings = append(*opts.OptimizerTimings, newNestedPipelineStageTiming(stageName, duration, err))
		}
		if callback := ctxModuleRunCallback(ctx); callback != nil {
			callback(Tier2ModuleRun{
				Phase:      phase,
				ModuleName: module.Name,
				StageName:  stageName,
				Function:   fn,
				Duration:   duration,
				Err:        err,
				Requires:   cloneAnalysisFacts(module.Requires),
				Provides:   cloneAnalysisFacts(module.Provides),
				Updates:    cloneAnalysisFacts(module.Updates),
			})
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", module.Name, err)
		}
		attachRemarks(fn, opts)

		// Invoke snapshot callback if set.
		if callback := ctxSnapshotCallback(ctx); callback != nil {
			callback(module.Name, fn, duration)
		}
	}
	return fn, nil
}

func tier2VerifyIROptEnabled(opts *Tier2PipelineOpts) bool {
	return opts != nil && opts.VerifyIR
}

func (run Tier2ModuleRun) Contract() Tier2ModuleContract {
	return Tier2ModuleContract{
		Phase:      run.Phase,
		ModuleName: run.ModuleName,
		StageName:  run.StageName,
		Requires:   cloneAnalysisFacts(run.Requires),
		Provides:   cloneAnalysisFacts(run.Provides),
		Updates:    cloneAnalysisFacts(run.Updates),
	}
}

func moduleContractsFromRuns(runs []Tier2ModuleRun) []Tier2ModuleContract {
	if len(runs) == 0 {
		return nil
	}
	contracts := make([]Tier2ModuleContract, 0, len(runs))
	for _, run := range runs {
		contract := run.Contract()
		if len(contract.Requires) == 0 && len(contract.Provides) == 0 && len(contract.Updates) == 0 {
			continue
		}
		contracts = append(contracts, contract)
	}
	return contracts
}

func cloneAnalysisFacts(facts []AnalysisFact) []AnalysisFact {
	if len(facts) == 0 {
		return nil
	}
	return append([]AnalysisFact(nil), facts...)
}

// FormatTier2ModuleContracts renders declared module analysis-fact contracts
// for human diagnostics.
func FormatTier2ModuleContracts(contracts []Tier2ModuleContract) string {
	if len(contracts) == 0 {
		return "(not recorded)\n"
	}
	var b strings.Builder
	for _, contract := range contracts {
		fmt.Fprintf(&b, "  %s/%s\n", contract.Phase, contract.ModuleName)
		fmt.Fprintf(&b, "    requires: %s\n", formatAnalysisFacts(contract.Requires))
		fmt.Fprintf(&b, "    provides: %s\n", formatAnalysisFacts(contract.Provides))
		fmt.Fprintf(&b, "    updates:  %s\n", formatAnalysisFacts(contract.Updates))
	}
	return b.String()
}

func formatAnalysisFacts(facts []AnalysisFact) string {
	if len(facts) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(facts))
	for _, fact := range facts {
		parts = append(parts, string(fact))
	}
	return strings.Join(parts, ", ")
}

func tier2OptimizerModuleStageName(phase Tier2OptimizerPhase, moduleName string) string {
	return fmt.Sprintf("RunTier2Pipeline/%s/%s", phase, moduleName)
}

func ctxModuleRunCallback(ctx *Tier2OptimizerContext) Tier2ModuleRunCallback {
	if ctx == nil {
		return nil
	}
	if ctx.ModuleRunCallback != nil {
		return ctx.ModuleRunCallback
	}
	if ctx.PipelineData == nil {
		return nil
	}
	return ctx.PipelineData.Diagnostics.ModuleRunCallback
}

func ctxSnapshotCallback(ctx *Tier2OptimizerContext) Tier2SnapshotCallback {
	if ctx == nil {
		return nil
	}
	if ctx.SnapshotCallback != nil {
		return ctx.SnapshotCallback
	}
	if ctx.PipelineData == nil {
		return nil
	}
	return ctx.PipelineData.Diagnostics.SnapshotCallback
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

func ctxDependencyRegistry(ctx *Tier2OptimizerContext) *CompilationDependencyRegistry {
	if ctx == nil {
		return nil
	}
	return ctx.DependencyRegistry
}
