package methodjit

import "github.com/gscript/gscript/internal/vm"

func init() {
	RegisterModuleBuilder(Tier2PhaseTableObjectPrep, 50, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2TableObjectPreparationModules(ctxGlobals(ctx))
	})
	RegisterModuleBuilder(Tier2PhaseTableArrayLower, 80, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2TableArrayNativeLoweringModules()
	})
	RegisterModuleBuilder(Tier2PhaseTableFieldLower, 100, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2TableFieldNativeLoweringModules(ctxGlobals(ctx))
	})
}

func tier2TableObjectPreparationModules(globals map[string]*vm.FuncProto) []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("TablePreallocHint", Tier2PhaseTableObjectPrep, nil, nil, TablePreallocHintPass),
		tier2PassModuleWith("TypeSpecialize (post-table-prealloc)", Tier2PhaseTableObjectPrep, nil, nil, TypeSpecializePass),
		{
			Name:     "FixedShapeTableFacts",
			Phase:    Tier2PhaseTableObjectPrep,
			Requires: nil,
			Provides: nil, // Updates existing facts
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				return FixedShapeTableFactsPassWith(FixedShapeTableFactsConfig{
					Globals:               globals,
					ArgFacts:              optsFixedShapeArgFacts(opts),
					ArgPolyFacts:          optsFixedShapeArgPolyFacts(opts),
					ArrayElementArgFacts:  optsFixedShapeArrayElementArgFacts(opts),
					ArrayElementPolyFacts: optsFixedShapeArrayElementPolyFacts(opts),
					EntryGuardedArgs:      optsFixedShapeEntryGuards(opts),
				})(fn)
			},
		},
		tier2PassModuleWith("LoadElimination", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), nil, LoadEliminationPass),
		tier2PassModuleWith("FieldLenFold", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), nil, FieldLenFoldPass),
		tier2PassModuleWith("StaticTableLenFold", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), nil, StaticTableLenFoldPass),
		tier2PassModuleWith("EscapeAnalysis", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), nil, EscapeAnalysisPass),
		tier2PassModuleWith("FixedTableConstructorLowering", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactFixedTableConstructors), nil, FixedTableConstructorLoweringPass),
		tier2PassModuleWith("TablePreallocHint (post-fixed-table-lowering)", Tier2PhaseTableObjectPrep, nil, nil, TablePreallocHintPass),
		{
			Name:     "EscapeAnalysis (post-fixed-table-lowering)",
			Phase:    Tier2PhaseTableObjectPrep,
			Requires: analysisFacts(AnalysisFactFixedShapeTables),
			Provides: nil,
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				if !hasFixedTableScalarReplacementCandidate(fn) {
					return fn, nil
				}
				return EscapeAnalysisPass(fn)
			},
		},
		tier2PassModuleWith("RedundantGuardElimination", Tier2PhaseTableObjectPrep, nil, nil, RedundantGuardEliminationPass),
	}
}

func tier2TableArrayNativeLoweringModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("TableArrayLower", Tier2PhaseTableArrayLower, nil, nil, TableArrayLowerPass),
		tier2PassModuleWith("TableArrayLoadTypeSpecialize", Tier2PhaseTableArrayLower, nil, nil, TableArrayLoadTypeSpecializePass),
		tier2PassModuleWith("StringEnumCompare", Tier2PhaseTableArrayLower, nil, nil, StringEnumComparePass),
		tier2PassModuleWith("TableArrayNestedLoad", Tier2PhaseTableArrayLower, nil, nil, TableArrayNestedLoadPass),
	}
}

func tier2TableFieldNativeLoweringModules(globals map[string]*vm.FuncProto) []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("TableArrayStoreLower", Tier2PhaseTableFieldLower, nil, nil, TableArrayStoreLowerPass),
		tier2PassModuleWith("GuardFieldCallee", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactCallABIs), nil, GuardFieldCalleePass),
		tier2PassModuleWith("FieldPolyLenPhi", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactFixedShapeTables), nil, FieldPolyLenPhiPass),
		tier2PassModuleWith("FieldSvalsLower", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactFixedShapeEntryGuards), nil, FieldSvalsLowerPass),
		tier2PassModuleWith("FieldSvalsCSE", Tier2PhaseTableFieldLower, nil, nil, FieldSvalsCSEPass),
		{
			Name:     "FixedShapeTableFacts (post-FieldSvalsLower)",
			Phase:    Tier2PhaseTableFieldLower,
			Requires: nil,
			Provides: nil, // Updates existing facts
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				return FixedShapeTableFactsPassWith(FixedShapeTableFactsConfig{
					Globals:               globals,
					ArgFacts:              optsFixedShapeArgFacts(opts),
					ArgPolyFacts:          optsFixedShapeArgPolyFacts(opts),
					ArrayElementArgFacts:  optsFixedShapeArrayElementArgFacts(opts),
					ArrayElementPolyFacts: optsFixedShapeArrayElementPolyFacts(opts),
					EntryGuardedArgs:      optsFixedShapeEntryGuards(opts),
				})(fn)
			},
		},
		tier2PassModuleWith("GuardFieldCallee (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactCallABIs), nil, GuardFieldCalleePass),
		tier2PassModuleWith("TableArrayLower (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TableArrayLowerPass),
		tier2PassModuleWith("TableArrayLoadTypeSpecialize (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TableArrayLoadTypeSpecializePass),
		tier2PassModuleWith("StringEnumCompare (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, StringEnumComparePass),
		tier2PassModuleWith("TableArrayStoreLower (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TableArrayStoreLowerPass),
		tier2PassModuleWith("TypeSpecialize (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TypeSpecializePass),
		tier2PassModuleWith("FloorPhiSplit", Tier2PhaseTableFieldLower, nil, nil, FloorPhiSplitPass),
		tier2PassModuleWith("DCE (post-FloorPhiSplit)", Tier2PhaseTableFieldLower, nil, nil, DCEPass),
		tier2PassModuleWith("ShapeFieldTypeGuard", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactFixedShapeTables), analysisFacts(AnalysisFactShapeFieldTypeElided), ShapeFieldTypeGuardPass),
		tier2PassModuleWith("LateModuloMultiplyOverflowBoxing", Tier2PhaseTableFieldLower, nil, nil, LateModuloMultiplyOverflowBoxingPass),
		tier2PassModuleWith("ProfiledStringLenFold", Tier2PhaseTableFieldLower, nil, nil, ProfiledStringLenFoldPass),
		tier2PassModuleWith("RangeAnalysis (post-TableFieldLower)", Tier2PhaseTableFieldLower, nil, rangeAnalysisFacts(), RangeAnalysisPass),
		tier2PassModuleWith("TableArrayStaticBounds", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactIntRanges), nil, TableArrayStaticBoundsPass),
		tier2PassModuleWith("DCE (post-TableArrayStoreLower)", Tier2PhaseTableFieldLower, nil, nil, DCEPass),
	}
}

func tier2TableLoopKernelModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("BoolTableFillLoop", Tier2PhaseLoopKernel, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactIntRanges), nil, BoolTableFillLoopPass),
		tier2PassModuleWith("TableArrayStoreLoopVersion", Tier2PhaseLoopKernel, analysisFacts(AnalysisFactIntRanges), nil, TableArrayStoreLoopVersionPass),
		tier2PassModuleWith("RecordArrayLoopKernel", Tier2PhaseLoopKernel, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactCallABIs), analysisFacts(AnalysisFactRecordArrayLoopKernel, AnalysisFactRecordArrayLoopCaches), RecordArrayLoopKernelPass),
		tier2PassModuleWith("TableIntArrayKernel", Tier2PhaseLoopKernel, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactCallABIs), nil, TableIntArrayKernelPass),
		tier2PassModuleWith("BoolTableCountLoop", Tier2PhaseLoopKernel, analysisFacts(AnalysisFactFixedShapeTables), nil, BoolTableCountLoopPass),
	}
}

func tier2TableLoopPostLoadElimModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("TableArraySwapFusion", Tier2PhaseLoopKernel, nil, nil, TableArraySwapFusionPass),
		tier2PassModuleWith("TableIntArrayKernel (post-swap-fusion)", Tier2PhaseLoopKernel, nil, nil, TableIntArrayKernelPass),
	}
}
