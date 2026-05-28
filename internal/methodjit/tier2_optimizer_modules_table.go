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
	fixedShapeFacts := fixedShapeTableFacts()
	fixedShapeAllowed := allowedDomainsForModule(nil, nil, fixedShapeFacts)
	escapePostAllowed := allowedDomainsForModule(analysisFacts(AnalysisFactFixedShapeTables), nil, nil)
	return []Tier2OptimizerModule{
		tier2PassModuleWith("TablePreallocHint", Tier2PhaseTableObjectPrep, nil, nil, TablePreallocHintPass),
		tier2PassModuleWith("TypeSpecialize (post-table-prealloc)", Tier2PhaseTableObjectPrep, nil, nil, TypeSpecializePass),
		{
			Name:     "FixedShapeTableFacts",
			Phase:    Tier2PhaseTableObjectPrep,
			Requires: nil,
			Updates:  fixedShapeFacts,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return FixedShapeTableFactsPassCtx(fixedShapeTableFactsConfigFromOpts(globals, opts))(newPassContext(fn, opts, fixedShapeAllowed, passContextEnforce))
			},
		},
		tier2PassModuleWith("LoadElimination", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), nil, LoadEliminationPass),
		tier2PassModuleWithCtxUpdates("FieldLenFold", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), analysisFacts(AnalysisFactProfiledIntRanges), FieldLenFoldPassCtx),
		tier2PassModuleWith("StaticTableLenFold", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), nil, StaticTableLenFoldPass),
		tier2PassModuleWithCtxOptionalReads("EscapeAnalysis", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables), nil, analysisFacts(AnalysisFactGlobals), EscapeAnalysisPassCtx),
		tier2PassModuleWithCtx("FixedTableConstructorLowering", Tier2PhaseTableObjectPrep, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactFixedTableConstructors), nil, FixedTableConstructorLoweringPassCtx),
		tier2PassModuleWith("TablePreallocHint (post-fixed-table-lowering)", Tier2PhaseTableObjectPrep, nil, nil, TablePreallocHintPass),
		{
			Name:     "EscapeAnalysis (post-fixed-table-lowering)",
			Phase:    Tier2PhaseTableObjectPrep,
			Requires: analysisFacts(AnalysisFactFixedShapeTables),
			Provides: nil,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, optCtx *Tier2OptimizerContext) (*Function, error) {
				if !hasFixedTableScalarReplacementCandidate(fn) {
					return fn, nil
				}
				return EscapeAnalysisPassCtx(newPassContext(fn, opts, escapePostAllowed, passContextEnforce))
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

func fixedShapeTableFactsConfigFromOpts(globals map[string]*vm.FuncProto, opts *Tier2PipelineOpts) FixedShapeTableFactsConfig {
	return FixedShapeTableFactsConfig{
		Globals:               globals,
		ArgFacts:              optsFixedShapeArgFacts(opts),
		ArgPolyFacts:          optsFixedShapeArgPolyFacts(opts),
		ArrayElementArgFacts:  optsFixedShapeArrayElementArgFacts(opts),
		ArrayElementPolyFacts: optsFixedShapeArrayElementPolyFacts(opts),
		EntryGuardedArgs:      optsFixedShapeEntryGuards(opts),
	}
}

func tier2TableFieldNativeLoweringModules(globals map[string]*vm.FuncProto) []Tier2OptimizerModule {
	fixedShapeFacts := fixedShapeTableFacts()
	fixedShapePostFieldSvalsRequires := analysisFacts(AnalysisFactIntRanges)
	fixedShapePostFieldSvalsAllowed := allowedDomainsForModule(fixedShapePostFieldSvalsRequires, nil, fixedShapeFacts)
	shapeFieldTypeGuardRequires := analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactFieldPolyShapeCatalog)
	shapeFieldTypeGuardProvides := analysisFacts(AnalysisFactShapeFieldTypeElided)
	shapeFieldTypeGuardAllowed := allowedDomainsForModule(shapeFieldTypeGuardRequires, shapeFieldTypeGuardProvides, nil)
	return []Tier2OptimizerModule{
		tier2PassModuleWith("TableArrayStoreLower", Tier2PhaseTableFieldLower, nil, nil, TableArrayStoreLowerPass),
		tier2PassModuleWithCtx("GuardFieldCallee", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, GuardFieldCalleePassCtx),
		tier2PassModuleWithCtx("FieldPolyLenPhi", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, FieldPolyLenPhiPassCtx),
		{
			Name:     "FieldSvalsLower",
			Phase:    Tier2PhaseTableFieldLower,
			Requires: analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactFixedShapeEntryGuards),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return FieldSvalsLowerPassCtx(ctxDependencyRegistry(ctx))(newPassContext(fn, opts, fieldSvalsLowerPassAllowedDomains, passContextEnforce))
			},
		},
		tier2PassModuleWith("FieldSvalsCSE", Tier2PhaseTableFieldLower, nil, nil, FieldSvalsCSEPass),
		{
			Name:     "FixedShapeTableFacts (post-FieldSvalsLower)",
			Phase:    Tier2PhaseTableFieldLower,
			Requires: fixedShapePostFieldSvalsRequires,
			Updates:  fixedShapeFacts,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return FixedShapeTableFactsPassCtx(fixedShapeTableFactsConfigFromOpts(globals, opts))(newPassContext(fn, opts, fixedShapePostFieldSvalsAllowed, passContextEnforce))
			},
		},
		tier2PassModuleWithCtx("GuardFieldCallee (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, GuardFieldCalleePassCtx),
		{
			Name:     "StableFieldCalleeGuard",
			Phase:    Tier2PhaseTableFieldLower,
			Requires: analysisFacts(AnalysisFactFieldPolyShapeFacts),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return StableFieldCalleeGuardPassCtx(ctxDependencyRegistry(ctx))(newPassContext(fn, opts, stableFieldCalleeGuardPassAllowedDomains, passContextEnforce))
			},
		},
		tier2PassModuleWith("TableArrayLower (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TableArrayLowerPass),
		tier2PassModuleWith("TableArrayLoadTypeSpecialize (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TableArrayLoadTypeSpecializePass),
		tier2PassModuleWith("StringEnumCompare (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, StringEnumComparePass),
		tier2PassModuleWith("TableArrayStoreLower (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TableArrayStoreLowerPass),
		tier2PassModuleWith("TypeSpecialize (post-FieldSvalsLower)", Tier2PhaseTableFieldLower, nil, nil, TypeSpecializePass),
		tier2PassModuleWith("FloorPhiSplit", Tier2PhaseTableFieldLower, nil, nil, FloorPhiSplitPass),
		tier2PassModuleWith("DCE (post-FloorPhiSplit)", Tier2PhaseTableFieldLower, nil, nil, DCEPass),
		{
			Name:     "ShapeFieldTypeGuard",
			Phase:    Tier2PhaseTableFieldLower,
			Requires: shapeFieldTypeGuardRequires,
			Provides: shapeFieldTypeGuardProvides,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return ShapeFieldTypeGuardPassCtx(newPassContext(fn, opts, shapeFieldTypeGuardAllowed, passContextEnforce), ctxDependencyRegistry(ctx))
			},
		},
		tier2PassModuleWith("LateModuloMultiplyOverflowBoxing", Tier2PhaseTableFieldLower, nil, nil, LateModuloMultiplyOverflowBoxingPass),
		tier2PassModuleWithCtxUpdates("ProfiledStringLenFold", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactIntRanges, AnalysisFactFixedShapeTables), analysisFacts(AnalysisFactProfiledIntRanges), ProfiledStringLenFoldPassCtx),
		tier2PassModuleWithCtxUpdates("RangeAnalysis (post-TableFieldLower)", Tier2PhaseTableFieldLower, nil, rangeAnalysisFacts(), RangeAnalysisPassCtx),
		tier2PassModuleWith("TableArrayStaticBounds", Tier2PhaseTableFieldLower, analysisFacts(AnalysisFactIntRanges), analysisFacts(AnalysisFactTableArrayBoundsSafe), TableArrayStaticBoundsPass),
		tier2PassModuleWith("DCE (post-TableArrayStoreLower)", Tier2PhaseTableFieldLower, nil, nil, DCEPass),
	}
}

func tier2TableLoopSpecializationModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("BoolTableFillLoop", Tier2PhaseLoopSpecialization, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactIntRanges), nil, BoolTableFillLoopPass),
		tier2PassModuleWith("TableArrayStoreLoopVersion", Tier2PhaseLoopSpecialization, analysisFacts(AnalysisFactIntRanges), nil, TableArrayStoreLoopVersionPass),
		tier2PassModuleWithCtx("RecordArrayLoopSpecialization", Tier2PhaseLoopSpecialization, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactCallABIs), analysisFacts(AnalysisFactRecordArrayLoopSpecialization, AnalysisFactRecordArrayLoopCaches), RecordArrayLoopSpecializationPassCtx),
		tier2PassModuleWith("TableIntArraySpecialization", Tier2PhaseLoopSpecialization, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactCallABIs), nil, TableIntArraySpecializationPass),
		tier2PassModuleWith("BoolTableCountLoop", Tier2PhaseLoopSpecialization, analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactIntRanges), nil, BoolTableCountLoopPass),
	}
}

func tier2TableLoopPostLoadElimModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("TableArraySwapFusion", Tier2PhaseLoopSpecialization, nil, nil, TableArraySwapFusionPass),
		tier2PassModuleWith("TableIntArraySpecialization (post-swap-fusion)", Tier2PhaseLoopSpecialization, nil, nil, TableIntArraySpecializationPass),
	}
}
