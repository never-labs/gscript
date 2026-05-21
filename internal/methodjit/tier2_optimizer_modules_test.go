package methodjit

import (
	"errors"
	"strings"
	"testing"
)

func assertTier2ModuleOrder(t *testing.T, mods []Tier2OptimizerModule, phase Tier2OptimizerPhase, want []string) {
	t.Helper()
	if len(mods) != len(want) {
		t.Fatalf("%s module count=%d want %d: %v", phase, len(mods), len(want), tier2ModuleNames(mods))
	}
	for i, wantName := range want {
		if mods[i].Phase != phase || mods[i].Name != wantName {
			t.Fatalf("module[%d]=%s/%s want %s/%s; all=%v", i, mods[i].Phase, mods[i].Name, phase, wantName, tier2ModuleNames(mods))
		}
	}
}

func tier2ModuleNames(mods []Tier2OptimizerModule) []string {
	names := make([]string, 0, len(mods))
	for _, mod := range mods {
		names = append(names, mod.Name)
	}
	return names
}

type testCompilationDependency struct{}

func (testCompilationDependency) Kind() CompilationDependencyKind {
	return CompilationDependencyGlobal
}

func (testCompilationDependency) Key() string {
	return "test-dependency"
}

func (testCompilationDependency) Validate(CompilationDependencyContext) error {
	return nil
}

func (testCompilationDependency) String() string {
	return "test-dependency"
}

func findTier2Module(t *testing.T, plan Tier2OptimizerPlan, name string) Tier2OptimizerModule {
	t.Helper()
	for _, module := range plan.Modules {
		if module.Name == name {
			return module
		}
	}
	t.Fatalf("module %q not found", name)
	return Tier2OptimizerModule{}
}

func assertAnalysisFacts(t *testing.T, got []AnalysisFact, want ...AnalysisFact) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("facts=%v want %v", got, want)
	}
	counts := make(map[AnalysisFact]int, len(got))
	for _, fact := range got {
		counts[fact]++
	}
	for _, fact := range want {
		if counts[fact] == 0 {
			t.Fatalf("facts=%v missing %s", got, fact)
		}
		counts[fact]--
	}
	for fact, count := range counts {
		if count != 0 {
			t.Fatalf("facts=%v has unexpected %s", got, fact)
		}
	}
}

func TestTier2TableObjectPreparationModuleOrder(t *testing.T) {
	mods := tier2TableObjectPreparationModules(nil)
	want := []string{
		"TablePreallocHint",
		"TypeSpecialize (post-table-prealloc)",
		"FixedShapeTableFacts",
		"LoadElimination",
		"FieldLenFold",
		"StaticTableLenFold",
		"EscapeAnalysis",
		"FixedTableConstructorLowering",
		"TablePreallocHint (post-fixed-table-lowering)",
		"EscapeAnalysis (post-fixed-table-lowering)",
		"RedundantGuardElimination",
	}
	assertTier2ModuleOrder(t, mods, Tier2PhaseTableObjectPrep, want)
}

func TestTier2OptimizerPlanPhaseOrder(t *testing.T) {
	plan := newTier2OptimizerPlan(&Tier2OptimizerContext{InlineMaxSize: 40})
	want := []Tier2OptimizerPhase{
		Tier2PhaseEarlyCanonical,
		Tier2PhaseInlineCall,
		Tier2PhaseCallLower,
		Tier2PhaseStringNative,
		Tier2PhaseTableObjectPrep,
		Tier2PhasePostRewrite,
		Tier2PhaseNumeric,
		Tier2PhaseTableArrayLower,
		Tier2PhaseMatrixNative,
		Tier2PhaseTableFieldLower,
		Tier2PhaseFloatNumeric,
		Tier2PhaseLoopKernel,
		Tier2PhaseLoopPost,
		Tier2PhaseFinalCall,
	}
	if len(plan.Phases) != len(want) {
		t.Fatalf("phase count=%d want %d: %v", len(plan.Phases), len(want), plan.Phases)
	}
	for i, phase := range want {
		if plan.Phases[i] != phase {
			t.Fatalf("phase[%d]=%s want %s; all=%v", i, plan.Phases[i], phase, plan.Phases)
		}
	}
}

func TestTier2OptimizerPlanCoversModulePhases(t *testing.T) {
	plan := newTier2OptimizerPlan(&Tier2OptimizerContext{InlineMaxSize: 40})
	phases := make(map[Tier2OptimizerPhase]bool, len(plan.Phases))
	for _, phase := range plan.Phases {
		if phases[phase] {
			t.Fatalf("duplicate phase in plan: %s", phase)
		}
		phases[phase] = true
	}
	names := make(map[string]bool, len(plan.Modules))
	for _, module := range plan.Modules {
		if !phases[module.Phase] {
			t.Fatalf("module %s uses phase %s missing from plan", module.Name, module.Phase)
		}
		if names[module.Name] {
			t.Fatalf("duplicate module name: %s", module.Name)
		}
		names[module.Name] = true
	}
}

func TestRunTier2PipelineCreatesPipelineDataWithDependencyRegistry(t *testing.T) {
	oldRegistry := DefaultModuleRegistry
	registry := NewModuleRegistry()
	DefaultModuleRegistry = registry
	t.Cleanup(func() {
		DefaultModuleRegistry = oldRegistry
	})

	var seenCtx *Tier2OptimizerContext
	if err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 10, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return []Tier2OptimizerModule{
			{
				Name:  "DependencyProbe",
				Phase: Tier2PhaseNumeric,
				RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
					seenCtx = ctx
					if ctx == nil || ctx.PipelineData == nil {
						t.Fatal("pipeline data missing from optimizer context")
					}
					if ctx.PipelineData.CompilationDependencies == nil {
						t.Fatal("compilation dependency registry missing from pipeline data")
					}
					ctx.PipelineData.CompilationDependencies.Record(testCompilationDependency{})
					return fn, nil
				},
			},
		}
	}); err != nil {
		t.Fatalf("RegisterModuleBuilder: %v", err)
	}

	_, notes, err := RunTier2Pipeline(&Function{Analysis: NewAnalysisResult()}, nil)
	if err != nil {
		t.Fatalf("RunTier2Pipeline: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("intrinsic notes = %v, want none", notes)
	}
	if seenCtx == nil {
		t.Fatal("probe module did not run")
	}
	if seenCtx.PipelineData.Plan == nil {
		t.Fatal("pipeline data did not retain optimizer plan")
	}
	if got := seenCtx.PipelineData.CompilationDependencies.Len(); got != 1 {
		t.Fatalf("dependency registry length = %d, want 1", got)
	}
}

func TestTier2PipelineDataDiagnosticsModuleCallback(t *testing.T) {
	var runs []Tier2ModuleRun
	data := NewTier2PipelineData()
	data.Diagnostics.ModuleRunCallback = func(run Tier2ModuleRun) {
		runs = append(runs, run)
	}
	ctx := newTier2OptimizerContext(data)
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:  "DiagnosticsProbe",
				Phase: Tier2PhaseNumeric,
				Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
					return fn, nil
				},
			},
		},
	}

	_, err := runTier2OptimizerPlan(&Function{Analysis: NewAnalysisResult()}, nil, ctx, plan)
	if err != nil {
		t.Fatalf("runTier2OptimizerPlan: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("module run callbacks = %d, want 1", len(runs))
	}
	if runs[0].ModuleName != "DiagnosticsProbe" || runs[0].StageName != "RunTier2Pipeline/numeric/DiagnosticsProbe" {
		t.Fatalf("unexpected callback payload: %+v", runs[0])
	}
}

func TestTier2OptimizerPlanDeduplicatesFallbackPhases(t *testing.T) {
	var ran []string
	makeModule := func(name string) Tier2OptimizerModule {
		return Tier2OptimizerModule{
			Name:  name,
			Phase: Tier2PhaseNumeric,
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				ran = append(ran, name)
				return fn, nil
			},
		}
	}
	plan := Tier2OptimizerPlan{
		Phases:  []Tier2OptimizerPhase{Tier2PhaseNumeric, Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{makeModule("first"), makeModule("second")},
	}

	_, err := runTier2OptimizerPlan(&Function{Analysis: NewAnalysisResult()}, nil, nil, plan)
	if err != nil {
		t.Fatalf("runTier2OptimizerPlan failed: %v", err)
	}
	want := []string{"first", "second"}
	if len(ran) != len(want) {
		t.Fatalf("ran %v want %v", ran, want)
	}
	for i := range want {
		if ran[i] != want[i] {
			t.Fatalf("ran %v want %v", ran, want)
		}
	}
}

func TestTier2OptimizerModuleFailureRecordsScope(t *testing.T) {
	wantErr := errors.New("synthetic failure")
	var runs []Tier2ModuleRun
	var timings []PipelineStageTiming
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:     "ProviderModule",
				Phase:    Tier2PhaseNumeric,
				Provides: analysisFacts(AnalysisFactIntRanges, AnalysisFactInt48Safe),
				Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
					return fn, nil
				},
			},
			{
				Name:     "FailingModule",
				Phase:    Tier2PhaseNumeric,
				Requires: analysisFacts(AnalysisFactIntRanges),
				Updates:  analysisFacts(AnalysisFactInt48Safe),
				Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
					return fn, wantErr
				},
			},
		},
	}
	ctx := &Tier2OptimizerContext{
		ModuleRunCallback: func(run Tier2ModuleRun) {
			runs = append(runs, run)
		},
	}
	_, err := runTier2OptimizerPlan(&Function{Analysis: NewAnalysisResult()}, &Tier2PipelineOpts{OptimizerTimings: &timings}, ctx, plan)
	if err == nil {
		t.Fatal("expected optimizer failure")
	}
	if !strings.Contains(err.Error(), "FailingModule") || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("error = %v, want module and cause", err)
	}
	if len(runs) != 2 {
		t.Fatalf("module runs = %d, want 2", len(runs))
	}
	run := runs[1]
	if run.Phase != Tier2PhaseNumeric || run.ModuleName != "FailingModule" {
		t.Fatalf("unexpected run scope: %+v", run)
	}
	wantStage := "RunTier2Pipeline/numeric/FailingModule"
	if run.StageName != wantStage {
		t.Fatalf("stage = %q, want %q", run.StageName, wantStage)
	}
	if run.Err == nil || !strings.Contains(run.Err.Error(), wantErr.Error()) {
		t.Fatalf("run err = %v, want %v", run.Err, wantErr)
	}
	if len(run.Requires) != 1 || run.Requires[0] != AnalysisFactIntRanges {
		t.Fatalf("run requires = %v, want IntRanges", run.Requires)
	}
	if len(run.Updates) != 1 || run.Updates[0] != AnalysisFactInt48Safe {
		t.Fatalf("run updates = %v, want Int48Safe", run.Updates)
	}
	if run.Duration < 0 {
		t.Fatalf("duration = %s, want non-negative", run.Duration)
	}
	if len(timings) != 2 {
		t.Fatalf("timings = %d, want 2", len(timings))
	}
	if timings[1].Name != wantStage || timings[1].Error != wantErr.Error() || !timings[1].Nested {
		t.Fatalf("unexpected timing: %+v", timings[1])
	}
}

func TestTier2OptimizerModuleVerifyIRRecordsScope(t *testing.T) {
	var runs []Tier2ModuleRun
	var timings []PipelineStageTiming
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:  "BreakIR",
				Phase: Tier2PhaseNumeric,
				Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
					fn.Blocks[0].Instrs = append(fn.Blocks[0].Instrs, nil)
					return fn, nil
				},
			},
		},
	}
	block := &Block{ID: 1}
	fn := &Function{Entry: block, Blocks: []*Block{block}, Analysis: NewAnalysisResult()}
	ctx := &Tier2OptimizerContext{
		ModuleRunCallback: func(run Tier2ModuleRun) {
			runs = append(runs, run)
		},
	}

	_, err := runTier2OptimizerPlan(fn, &Tier2PipelineOpts{OptimizerTimings: &timings, VerifyIR: true}, ctx, plan)
	if err == nil {
		t.Fatal("expected verifier failure")
	}
	wantStage := "RunTier2Pipeline/numeric/BreakIR"
	for _, want := range []string{"BreakIR", wantStage, "lightweight IR verifier failed", "nil instruction"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want substring %q", err, want)
		}
	}
	if len(runs) != 1 {
		t.Fatalf("module runs = %d, want 1", len(runs))
	}
	if runs[0].ModuleName != "BreakIR" || runs[0].StageName != wantStage {
		t.Fatalf("unexpected run scope: %+v", runs[0])
	}
	if runs[0].Err == nil || !strings.Contains(runs[0].Err.Error(), "lightweight IR verifier failed") {
		t.Fatalf("run err = %v, want verifier failure", runs[0].Err)
	}
	if len(timings) != 1 || timings[0].Name != wantStage || !strings.Contains(timings[0].Error, "lightweight IR verifier failed") {
		t.Fatalf("unexpected timings: %+v", timings)
	}
}

func TestTier2EarlyCanonicalModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2EarlyCanonicalModules(nil), Tier2PhaseEarlyCanonical, []string{
		"SimplifyPhis",
		"TypeSpecialize",
		"Intrinsic",
		"GlobalConstSpecialization",
		"TypeSpecialize (post-intrinsic)",
		"FixedShapeTableFacts (pre-inline)",
	})
}

func TestTier2InlineCallModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2InlineCallModules(nil, 40), Tier2PhaseInlineCall, []string{
		"FieldShapeCallSplitPreInline",
		"Inline",
		"SimplifyPhis (post-inline)",
		"SourceFeedbackRefresh (post-inline)",
		"Intrinsic (post-inline)",
		"TypeSpecialize (post-inline)",
		"FixedShapeTableFacts (post-inline)",
	})
}

func TestTier2CallLoweringModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2CallLoweringModules(nil), Tier2PhaseCallLower, []string{
		"CallABI",
		"CallReturnProjection",
		"ModularCallFloorReduce",
		"CallResultRangeGuard",
		"ConstProp",
		"ProtocolConstCallFold",
		"WholeCallKernelExit",
	})
}

func TestTier2StringNativeModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2StringNativeModules(), Tier2PhaseStringNative, []string{
		"StringNativeCleanup",
	})
}

func TestTier2PostRewriteModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2PostRewriteModules(), Tier2PhasePostRewrite, []string{
		"CallReturnProjection (post-rewrite)",
		"ModularCallFloorReduce (post-rewrite)",
		"CallResultRangeGuard (post-rewrite)",
		"DCE",
		"TypeSpecialize (post-escape)",
	})
}

func TestTier2NumericModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2NumericModules(), Tier2PhaseNumeric, []string{
		"LoopBoundRangeGuard",
		"ObservedParamRangeGuard",
		"ObservedParamTypeGuard",
		"ExactGuardConst",
		"ConstProp (post-ExactGuardConst)",
		"RangeAnalysis",
		"OverflowBoxing",
		"IntExactDivision",
		"RangeAnalysis (post-IntExactDivision)",
		"ModRangeSimplify",
		"DCE (post-ModRangeSimplify)",
		"ModZeroCompare",
		"DCE (post-ModZeroCompare)",
		"ConstantPhiBranchThreading",
		"JumpOnlyThreading",
		"SimplifyPhis (post-ConstantPhiBranchThreading)",
	})
}

func TestTier2TableNativeLoweringModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2TableArrayNativeLoweringModules(), Tier2PhaseTableArrayLower,
		[]string{"TableArrayLower", "TableArrayLoadTypeSpecialize", "StringEnumCompare", "TableArrayNestedLoad"})
	assertTier2ModuleOrder(t, tier2TableFieldNativeLoweringModules(nil), Tier2PhaseTableFieldLower,
		[]string{
			"TableArrayStoreLower",
			"GuardFieldCallee",
			"FieldPolyLenPhi",
			"FieldSvalsLower",
			"FieldSvalsCSE",
			"FixedShapeTableFacts (post-FieldSvalsLower)",
			"GuardFieldCallee (post-FieldSvalsLower)",
			"TableArrayLower (post-FieldSvalsLower)",
			"TableArrayLoadTypeSpecialize (post-FieldSvalsLower)",
			"StringEnumCompare (post-FieldSvalsLower)",
			"TableArrayStoreLower (post-FieldSvalsLower)",
			"TypeSpecialize (post-FieldSvalsLower)",
			"FloorPhiSplit",
			"DCE (post-FloorPhiSplit)",
			"ShapeFieldTypeGuard",
			"LateModuloMultiplyOverflowBoxing",
			"ProfiledStringLenFold",
			"RangeAnalysis (post-TableFieldLower)",
			"TableArrayStaticBounds",
			"DCE (post-TableArrayStoreLower)",
		})
	assertTier2ModuleOrder(t, tier2TableLoopKernelModules(), Tier2PhaseLoopKernel,
		[]string{"BoolTableFillLoop", "TableArrayStoreLoopVersion", "RecordArrayLoopKernel", "TableIntArrayKernel", "BoolTableCountLoop"})
	assertTier2ModuleOrder(t, tier2TableLoopPostLoadElimModules(), Tier2PhaseLoopKernel,
		[]string{"TableArraySwapFusion", "TableIntArrayKernel (post-swap-fusion)"})
}

func TestTier2MatrixNativeLoweringModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2MatrixNativeLoweringModules(), Tier2PhaseMatrixNative, []string{
		"DenseMatrixNestedLoadLower",
		"MatrixLower",
		"LoadElimination (post-MatrixLower)",
		"MatrixRowPtrFactoring",
		"MatrixUnitStride",
	})
}

func TestTier2FloatNumericModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2FloatNumericModules(), Tier2PhaseFloatNumeric, []string{
		"FMAFusion",
		"FloatStrengthReduction",
		"FMAFusion (post-FloatStrengthReduction)",
		"ComplexEscapeLoop",
	})
}

func TestTier2LoopKernelModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2LoopKernelModules(), Tier2PhaseLoopKernel, []string{
		"LICM",
		"LoopGlobalStoreSink",
		"BoolTableFillLoop",
		"TableArrayStoreLoopVersion",
		"RecordArrayLoopKernel",
		"TableIntArrayKernel",
		"BoolTableCountLoop",
		"FieldNumToFloatFusion (post-LICM)",
		"ClosureUpvalueScalar",
		"LoadElimination (post-LICM)",
		"TableArraySwapFusion",
		"TableIntArrayKernel (post-swap-fusion)",
		"DCE (post-LICM LoadElim)",
	})
}

func TestTier2LoopPostModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2LoopPostModules(), Tier2PhaseLoopPost, []string{
		"UnrollAndJam",
		"MatrixRowPtrFactoring (post-UnrollAndJam)",
		"FMAFusion (post-UnrollAndJam)",
		"LICM (post-MatrixRowPtrFactoring)",
		"QuadraticStepStrengthReduction",
		"RangeAnalysis (post-UnrollAndJam)",
		"IntAlgebraSimplify",
		"TableArrayStaticBounds (post-RangeAnalysis)",
		"DCE (post-UnrollAndJam)",
		"LoopRegionVersioning",
		"TableArrayStaticBounds (post-LoopRegionVersioning)",
		"ScalarPromotion",
		"TableArrayDataPtrFact",
	})
}

func TestTier2FinalCallModuleOrder(t *testing.T) {
	assertTier2ModuleOrder(t, tier2FinalCallModules(nil), Tier2PhaseFinalCall, []string{
		"CallABI (final)",
		"WholeCallKernelExit (final)",
		"CallReturnProjection (final)",
		"ModularCallFloorReduce (final)",
		"CallResultRangeGuard (final)",
		"FieldCallPolyLenFusion",
		"RangeAnalysis (post-final-call)",
	})
}

func TestTier2FinalCallModuleOrderExperimentalFieldShapeSplit(t *testing.T) {
	t.Setenv("GSCRIPT_FIELD_SHAPE_SPLIT", "1")
	assertTier2ModuleOrder(t, tier2FinalCallModules(nil), Tier2PhaseFinalCall, []string{
		"CallABI (final)",
		"WholeCallKernelExit (final)",
		"CallReturnProjection (final)",
		"ModularCallFloorReduce (final)",
		"CallResultRangeGuard (final)",
		"FieldCallPolyLenFusion",
		"RangeAnalysis (post-final-call)",
		"FieldShapeCallSplit (experimental)",
	})
}

func TestTier2FinalCallModuleOrderIgnoresOtherFieldShapeSplitValues(t *testing.T) {
	t.Setenv("GSCRIPT_FIELD_SHAPE_SPLIT", "0")
	assertTier2ModuleOrder(t, tier2FinalCallModules(nil), Tier2PhaseFinalCall, []string{
		"CallABI (final)",
		"WholeCallKernelExit (final)",
		"CallReturnProjection (final)",
		"ModularCallFloorReduce (final)",
		"CallResultRangeGuard (final)",
		"FieldCallPolyLenFusion",
		"RangeAnalysis (post-final-call)",
	})
}

func TestDependencyOrder(t *testing.T) {
	plan := newTier2OptimizerPlan(&Tier2OptimizerContext{InlineMaxSize: 40})
	if err := ValidateDependencyOrder(plan); err != nil {
		t.Fatalf("dependency validation failed: %v", err)
	}
}

func TestTier2RepeatedAnalysisModulesDeclareUpdates(t *testing.T) {
	plan := newTier2OptimizerPlan(&Tier2OptimizerContext{InlineMaxSize: 40})

	for _, name := range []string{
		"FixedShapeTableFacts (post-inline)",
		"FixedShapeTableFacts",
		"FixedShapeTableFacts (post-FieldSvalsLower)",
	} {
		module := findTier2Module(t, plan, name)
		if len(module.Provides) != 0 {
			t.Fatalf("%s Provides=%v, want none", name, module.Provides)
		}
		assertAnalysisFacts(t, module.Updates, fixedShapeTableFacts()...)
	}

	assertAnalysisFacts(t, findTier2Module(t, plan, "CallABI (final)").Updates, AnalysisFactCallABIs)
	assertAnalysisFacts(t, findTier2Module(t, plan, "WholeCallKernelExit (final)").Updates,
		AnalysisFactWholeCallNoResultKernels,
		AnalysisFactWholeCallNoResultBatches,
	)
	for _, name := range []string{
		"RangeAnalysis (post-IntExactDivision)",
		"RangeAnalysis (post-TableFieldLower)",
		"RangeAnalysis (post-UnrollAndJam)",
		"RangeAnalysis (post-final-call)",
	} {
		module := findTier2Module(t, plan, name)
		if len(module.Provides) != 0 {
			t.Fatalf("%s Provides=%v, want none", name, module.Provides)
		}
		assertAnalysisFacts(t, module.Updates, rangeAnalysisFacts()...)
	}
}

func TestDependencyOrderRejectsMissingRequirement(t *testing.T) {
	// Build a plan where a module requires a fact that is never provided.
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical, Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:     "FakeProvider",
				Phase:    Tier2PhaseEarlyCanonical,
				Requires: nil,
				Provides: []AnalysisFact{"FactA"},
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
			{
				Name:     "FakeConsumer",
				Phase:    Tier2PhaseNumeric,
				Requires: []AnalysisFact{"FactA", "FactB"}, // FactB is never provided
				Provides: nil,
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}
	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected error for missing FactB dependency, got nil")
	}
	if !strings.Contains(err.Error(), "FactB") {
		t.Fatalf("error should mention FactB, got: %v", err)
	}
	if !strings.Contains(err.Error(), "never provided") {
		t.Fatalf("error should say 'never provided', got: %v", err)
	}
}

func TestDependencyOrderRejectsOutOfOrderDependency(t *testing.T) {
	// Build a plan where a module requires a fact that is provided later.
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical, Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:     "Consumer",
				Phase:    Tier2PhaseEarlyCanonical,
				Requires: []AnalysisFact{"LateFact"},
				Provides: nil,
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
			{
				Name:     "LateProvider",
				Phase:    Tier2PhaseNumeric,
				Requires: nil,
				Provides: []AnalysisFact{"LateFact"},
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}
	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected error for out-of-order dependency, got nil")
	}
	if !strings.Contains(err.Error(), "LateFact") {
		t.Fatalf("error should mention LateFact, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not yet available") {
		t.Fatalf("error should say 'not yet available', got: %v", err)
	}
}

func TestDependencyOrderRejectsMissingUpdateProvider(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:    "Updater",
				Phase:   Tier2PhaseNumeric,
				Updates: []AnalysisFact{"MissingFact"},
				Run:     func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}
	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected error for missing update provider, got nil")
	}
	if !strings.Contains(err.Error(), "updates fact MissingFact") {
		t.Fatalf("error should mention updated fact, got: %v", err)
	}
	if !strings.Contains(err.Error(), "never provided") {
		t.Fatalf("error should say 'never provided', got: %v", err)
	}
}

func TestDependencyOrderRejectsOutOfOrderUpdate(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical, Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:    "Updater",
				Phase:   Tier2PhaseEarlyCanonical,
				Updates: []AnalysisFact{"LateFact"},
				Run:     func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
			{
				Name:     "LateProvider",
				Phase:    Tier2PhaseNumeric,
				Provides: []AnalysisFact{"LateFact"},
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}
	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected error for out-of-order update, got nil")
	}
	if !strings.Contains(err.Error(), "updates fact LateFact") {
		t.Fatalf("error should mention updated fact, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not yet available") {
		t.Fatalf("error should say 'not yet available', got: %v", err)
	}
}

func TestDependencyOrderAllowsMultipleProviders(t *testing.T) {
	// The same fact can be provided by multiple modules (e.g., RangeAnalysis
	// runs multiple times, each providing Int48Safe).
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical, Tier2PhaseNumeric},
		Modules: []Tier2OptimizerModule{
			{
				Name:     "Provider1",
				Phase:    Tier2PhaseEarlyCanonical,
				Requires: nil,
				Provides: []AnalysisFact{"SharedFact"},
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
			{
				Name:     "Consumer1",
				Phase:    Tier2PhaseNumeric,
				Requires: []AnalysisFact{"SharedFact"},
				Provides: nil,
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
			{
				Name:     "Provider2",
				Phase:    Tier2PhaseNumeric,
				Requires: nil,
				Provides: []AnalysisFact{"SharedFact"},
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
			{
				Name:     "Consumer2",
				Phase:    Tier2PhaseNumeric,
				Requires: []AnalysisFact{"SharedFact"},
				Provides: nil,
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}
	if err := ValidateDependencyOrder(plan); err != nil {
		t.Fatalf("expected no error for multiple providers, got: %v", err)
	}
}
