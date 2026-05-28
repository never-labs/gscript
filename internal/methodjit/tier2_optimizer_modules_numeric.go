package methodjit

func init() {
	RegisterModuleBuilder(Tier2PhaseNumeric, 70, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2NumericModules()
	})
	RegisterModuleBuilder(Tier2PhaseMatrixNative, 90, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2MatrixNativeLoweringModules()
	})
	RegisterModuleBuilder(Tier2PhaseFloatNumeric, 110, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2FloatNumericModules()
	})
	RegisterModuleBuilder(Tier2PhaseLoopSpecialization, 120, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2LoopSpecializationModules()
	})
	RegisterModuleBuilder(Tier2PhaseLoopPost, 130, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2LoopPostModules()
	})
}

func tier2NumericModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWithCtx("LoopBoundRangeGuard", Tier2PhaseNumeric, analysisFacts(AnalysisFactSpecDependencyProtos), nil, LoopBoundRangeGuardPassCtx),
		tier2PassModuleWithCtx("ObservedParamRangeGuard", Tier2PhaseNumeric, analysisFacts(AnalysisFactSpecDependencyProtos), nil, ObservedParamRangeGuardPassCtx),
		tier2PassModuleWith("ObservedParamTypeGuard", Tier2PhaseNumeric, nil, nil, ObservedParamTypeGuardPass),
		tier2PassModuleWith("ExactGuardConst", Tier2PhaseNumeric, nil, nil, ExactGuardConstPass),
		tier2PassModuleWith("ConstProp (post-ExactGuardConst)", Tier2PhaseNumeric, nil, nil, ConstPropPass),
		tier2PassModuleWithCtx("RangeAnalysis", Tier2PhaseNumeric, nil, rangeAnalysisFacts(), RangeAnalysisPassCtx),
		tier2PassModuleWithCtx("OverflowBoxing", Tier2PhaseNumeric, analysisFacts(AnalysisFactIntRanges), nil, OverflowBoxingPassCtx),
		tier2PassModuleWithCtx("IntExactDivision", Tier2PhaseNumeric, nil, nil, IntExactDivisionPassCtx),
		tier2PassModuleWithCtxUpdates("RangeAnalysis (post-IntExactDivision)", Tier2PhaseNumeric, nil, rangeAnalysisFacts(), func(ctx *PassContext) (*Function, error) {
			if opts := ctx.Opts(); opts != nil && !opts.LastPassChanged {
				functionRemarks(ctx.Func()).Add("RangeAnalysis", "skipped", 0, 0, OpNop,
					"IntExactDivision had no candidate rewrite")
				return ctx.Func(), nil
			}
			return RangeAnalysisPassCtx(ctx)
		}),
		tier2PassModuleWithCtx("ModRangeSimplify", Tier2PhaseNumeric, analysisFacts(AnalysisFactIntRanges), nil, ModRangeSimplifyPassCtx),
		tier2PassModuleWith("DCE (post-ModRangeSimplify)", Tier2PhaseNumeric, nil, nil, DCEPass),
		tier2PassModuleWithCtx("ModZeroCompare", Tier2PhaseNumeric, analysisFacts(AnalysisFactIntModNonZeroDivisor), nil, ModZeroComparePassCtx),
		tier2PassModuleWith("DCE (post-ModZeroCompare)", Tier2PhaseNumeric, nil, nil, DCEPass),
		tier2PassModuleWith("ConstantPhiBranchThreading", Tier2PhaseNumeric, nil, nil, ConstantPhiBranchThreadingPass),
		tier2PassModuleWith("JumpOnlyThreading", Tier2PhaseNumeric, nil, nil, JumpOnlyThreadingPass),
		tier2PassModuleWith("SimplifyPhis (post-ConstantPhiBranchThreading)", Tier2PhaseNumeric, nil, nil, SimplifyPhisPass),
	}
}

func tier2MatrixNativeLoweringModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("DenseMatrixNestedLoadLower", Tier2PhaseMatrixNative, nil, nil, DenseMatrixNestedLoadLowerPass),
		tier2PassModuleWith("MatrixLower", Tier2PhaseMatrixNative, nil, nil, MatrixLowerPass),
		{
			Name:     "LoadElimination (post-MatrixLower)",
			Phase:    Tier2PhaseMatrixNative,
			Requires: nil,
			Provides: nil,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, _ *Tier2OptimizerContext) (*Function, error) {
				if !hasMatrixNativeIR(fn) {
					return fn, nil
				}
				return LoadEliminationPass(fn)
			},
		},
		tier2PassModuleWith("MatrixRowPtrFactoring", Tier2PhaseMatrixNative, nil, nil, MatrixRowPtrFactoringPass),
		tier2PassModuleWith("MatrixUnitStride", Tier2PhaseMatrixNative, nil, nil, MatrixUnitStridePass),
	}
}

func hasMatrixNativeIR(fn *Function) bool {
	if fn == nil {
		return false
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpMatrixDense, OpMatrixGetF, OpMatrixSetF,
				OpMatrixFlat, OpMatrixStride, OpMatrixLoadFAt, OpMatrixStoreFAt,
				OpMatrixRowPtr, OpMatrixLoadFRow, OpMatrixStoreFRow,
				OpMatrixLoadFRowConst, OpMatrixStoreFRowConst:
				return true
			}
		}
	}
	return false
}

func tier2FloatNumericModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("FMAFusion", Tier2PhaseFloatNumeric, nil, nil, FMAFusionPass),
		tier2PassModuleWith("FloatStrengthReduction", Tier2PhaseFloatNumeric, nil, nil, FloatStrengthReductionPass),
		tier2PassModuleWith("FMAFusion (post-FloatStrengthReduction)", Tier2PhaseFloatNumeric, nil, nil, FMAFusionPass),
		tier2PassModuleWith("ComplexEscapeLoop", Tier2PhaseFloatNumeric, nil, nil, ComplexEscapeLoopPass),
	}
}

func tier2LoopSpecializationModules() []Tier2OptimizerModule {
	modules := []Tier2OptimizerModule{
		tier2PassModuleWithCtxOptionalReads("LICM", Tier2PhaseLoopSpecialization, analysisFacts(AnalysisFactInt48Safe, AnalysisFactCallABIs, AnalysisFactFixedShapeTables), nil, analysisFacts(AnalysisFactGlobals), LICMPassCtx),
		tier2PassModuleWith("LoopGlobalStoreSink", Tier2PhaseLoopSpecialization, nil, nil, LoopGlobalStoreSinkPass),
	}
	modules = append(modules, tier2TableLoopSpecializationModules()...)
	modules = append(modules,
		tier2PassModuleWith("FieldNumToFloatFusion (post-LICM)", Tier2PhaseLoopSpecialization, nil, nil, FieldNumToFloatFusionPass),
		tier2PassModuleWith("ClosureUpvalueScalar", Tier2PhaseLoopSpecialization, nil, nil, ClosureUpvalueScalarPass),
		tier2PassModuleWith("LoadElimination (post-LICM)", Tier2PhaseLoopSpecialization, nil, nil, LoadEliminationPass),
	)
	modules = append(modules, tier2TableLoopPostLoadElimModules()...)
	modules = append(modules, tier2PassModuleWith("DCE (post-LICM LoadElim)", Tier2PhaseLoopSpecialization, nil, nil, DCEPass))
	return modules
}

func tier2LoopPostModules() []Tier2OptimizerModule {
	licmPostRequires := analysisFacts(AnalysisFactInt48Safe, AnalysisFactCallABIs, AnalysisFactFixedShapeTables)
	licmPostOptionalReads := analysisFacts(AnalysisFactGlobals)
	licmPostAllowed := allowedDomainsForModule(licmPostRequires, nil, nil, licmPostOptionalReads)
	return []Tier2OptimizerModule{
		tier2PassModuleWith("UnrollAndJam", Tier2PhaseLoopPost, nil, nil, UnrollAndJamPass),
		tier2PassModuleWith("MatrixRowPtrFactoring (post-UnrollAndJam)", Tier2PhaseLoopPost, nil, nil, MatrixRowPtrFactoringPass),
		tier2PassModuleWith("FMAFusion (post-UnrollAndJam)", Tier2PhaseLoopPost, nil, nil, FMAFusionPass),
		{
			Name:          "LICM (post-MatrixRowPtrFactoring)",
			Phase:         Tier2PhaseLoopPost,
			Requires:      licmPostRequires,
			Provides:      nil,
			OptionalReads: licmPostOptionalReads,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, optCtx *Tier2OptimizerContext) (*Function, error) {
				if !hasMatrixNativeIR(fn) {
					return fn, nil
				}
				return LICMPassCtx(newPassContext(fn, opts, licmPostAllowed, passContextEnforce))
			},
		},
		tier2PassModuleWith("QuadraticStepStrengthReduction", Tier2PhaseLoopPost, analysisFacts(AnalysisFactIntRanges), nil, QuadraticStepStrengthReductionPass),
		tier2PassModuleWithCtxUpdates("RangeAnalysis (post-UnrollAndJam)", Tier2PhaseLoopPost, nil, rangeAnalysisFacts(), RangeAnalysisPassCtx),
		tier2PassModuleWithCtx("IntAlgebraSimplify", Tier2PhaseLoopPost, analysisFacts(AnalysisFactIntRanges), nil, IntAlgebraSimplifyPassCtx),
		tier2PassModuleWithCtxUpdates("TableArrayStaticBounds (post-RangeAnalysis)", Tier2PhaseLoopPost, analysisFacts(AnalysisFactIntRanges), analysisFacts(AnalysisFactTableArrayBoundsSafe), func(ctx *PassContext) (*Function, error) {
			return TableArrayStaticBoundsPass(ctx.Func())
		}),
		tier2PassModuleWith("DCE (post-UnrollAndJam)", Tier2PhaseLoopPost, nil, nil, DCEPass),
		tier2PassModuleWithCtxProvidesUpdatesOptionalReads("LoopRegionVersioning", Tier2PhaseLoopPost,
			analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactIntRanges, AnalysisFactRecordArrayLoopSpecialization),
			analysisFacts(AnalysisFactLoopTableArrayFacts),
			analysisFacts(AnalysisFactTableArrayBoundsSafe),
			analysisFacts(AnalysisFactGlobals),
			LoopRegionVersioningPassCtx),
		tier2PassModuleWithCtxUpdates("TableArrayStaticBounds (post-LoopRegionVersioning)", Tier2PhaseLoopPost, analysisFacts(AnalysisFactIntRanges, AnalysisFactRecordArrayLoopSpecialization), analysisFacts(AnalysisFactTableArrayBoundsSafe), func(ctx *PassContext) (*Function, error) {
			return TableArrayStaticBoundsPass(ctx.Func())
		}),
		tier2PassModuleWith("ScalarPromotion", Tier2PhaseLoopPost, analysisFacts(AnalysisFactFixedShapeTables), nil, ScalarPromotionPass),
		tier2PassModuleWithCtx("TableArrayDataPtrFact", Tier2PhaseLoopPost, analysisFacts(AnalysisFactFixedShapeTables), analysisFacts(AnalysisFactTableArrayDataPtrs), TableArrayDataPtrFactPassCtx),
	}
}
