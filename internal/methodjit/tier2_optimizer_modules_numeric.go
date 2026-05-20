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
	RegisterModuleBuilder(Tier2PhaseLoopKernel, 120, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2LoopKernelModules()
	})
	RegisterModuleBuilder(Tier2PhaseLoopPost, 130, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2LoopPostModules()
	})
}

func tier2NumericModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("LoopBoundRangeGuard", Tier2PhaseNumeric, nil, nil, LoopBoundRangeGuardPass),
		tier2PassModuleWith("ObservedParamRangeGuard", Tier2PhaseNumeric, nil, nil, ObservedParamRangeGuardPass),
		tier2PassModuleWith("ExactGuardConst", Tier2PhaseNumeric, nil, nil, ExactGuardConstPass),
		tier2PassModuleWith("ConstProp (post-ExactGuardConst)", Tier2PhaseNumeric, nil, nil, ConstPropPass),
		tier2PassModuleWith("RangeAnalysis", Tier2PhaseNumeric, nil, []string{"Int48Safe", "IntRanges", "IntNonNegative", "IntModNonZeroDivisor", "IntModNoSignAdjust"}, RangeAnalysisPass),
		{
			Name:     "OverflowBoxing",
			Phase:    Tier2PhaseNumeric,
			Requires: []string{"IntRanges"},
			Provides: nil,
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				force := map[int]bool(nil)
				if opts != nil {
					force = opts.ForceBoxIntIDs
				}
				return OverflowBoxingPassWith(force)(fn)
			},
		},
		{
			Name:     "IntExactDivision",
			Phase:    Tier2PhaseNumeric,
			Requires: nil,
			Provides: nil,
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				out, changed, err := runIntExactDivisionIfCandidate(fn)
				if opts != nil {
					opts.LastPassChanged = changed
				}
				return out, err
			},
		},
		{
			Name:     "RangeAnalysis (post-IntExactDivision)",
			Phase:    Tier2PhaseNumeric,
			Requires: nil,
			Provides: []string{"Int48Safe", "IntRanges", "IntNonNegative", "IntModNonZeroDivisor", "IntModNoSignAdjust"},
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				if opts != nil && !opts.LastPassChanged {
					functionRemarks(fn).Add("RangeAnalysis", "skipped", 0, 0, OpNop,
						"IntExactDivision had no candidate rewrite")
					return fn, nil
				}
				return RangeAnalysisPass(fn)
			},
		},
		tier2PassModuleWith("ModRangeSimplify", Tier2PhaseNumeric, []string{"IntRanges"}, nil, ModRangeSimplifyPass),
		tier2PassModuleWith("DCE (post-ModRangeSimplify)", Tier2PhaseNumeric, nil, nil, DCEPass),
		tier2PassModuleWith("ModZeroCompare", Tier2PhaseNumeric, []string{"IntModNonZeroDivisor"}, nil, ModZeroComparePass),
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
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
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

func tier2LoopKernelModules() []Tier2OptimizerModule {
	modules := []Tier2OptimizerModule{
		tier2PassModuleWith("LICM", Tier2PhaseLoopKernel, []string{"Int48Safe"}, nil, LICMPass),
		tier2PassModuleWith("LoopGlobalStoreSink", Tier2PhaseLoopKernel, nil, nil, LoopGlobalStoreSinkPass),
	}
	modules = append(modules, tier2TableLoopKernelModules()...)
	modules = append(modules,
		tier2PassModuleWith("FieldNumToFloatFusion (post-LICM)", Tier2PhaseLoopKernel, nil, nil, FieldNumToFloatFusionPass),
		tier2PassModuleWith("ClosureUpvalueScalar", Tier2PhaseLoopKernel, nil, nil, ClosureUpvalueScalarPass),
		tier2PassModuleWith("LoadElimination (post-LICM)", Tier2PhaseLoopKernel, nil, nil, LoadEliminationPass),
	)
	modules = append(modules, tier2TableLoopPostLoadElimModules()...)
	modules = append(modules, tier2PassModuleWith("DCE (post-LICM LoadElim)", Tier2PhaseLoopKernel, nil, nil, DCEPass))
	return modules
}

func tier2LoopPostModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("UnrollAndJam", Tier2PhaseLoopPost, nil, nil, UnrollAndJamPass),
		tier2PassModuleWith("MatrixRowPtrFactoring (post-UnrollAndJam)", Tier2PhaseLoopPost, nil, nil, MatrixRowPtrFactoringPass),
		{
			Name:     "LICM (post-MatrixRowPtrFactoring)",
			Phase:    Tier2PhaseLoopPost,
			Requires: []string{"Int48Safe"},
			Provides: nil,
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				if !hasMatrixNativeIR(fn) {
					return fn, nil
				}
				return LICMPass(fn)
			},
		},
		tier2PassModuleWith("QuadraticStepStrengthReduction", Tier2PhaseLoopPost, []string{"IntRanges"}, nil, QuadraticStepStrengthReductionPass),
		tier2PassModuleWith("RangeAnalysis (post-UnrollAndJam)", Tier2PhaseLoopPost, nil, []string{"Int48Safe", "IntRanges", "IntNonNegative", "IntModNonZeroDivisor", "IntModNoSignAdjust"}, RangeAnalysisPass),
		tier2PassModuleWith("IntAlgebraSimplify", Tier2PhaseLoopPost, []string{"IntRanges"}, nil, IntAlgebraSimplifyPass),
		tier2PassModuleWith("TableArrayStaticBounds (post-RangeAnalysis)", Tier2PhaseLoopPost, []string{"IntRanges"}, nil, TableArrayStaticBoundsPass),
		tier2PassModuleWith("DCE (post-UnrollAndJam)", Tier2PhaseLoopPost, nil, nil, DCEPass),
		tier2PassModuleWith("LoopRegionVersioning", Tier2PhaseLoopPost, []string{"FixedShapeTables"}, nil, LoopRegionVersioningPass),
		tier2PassModuleWith("TableArrayStaticBounds (post-LoopRegionVersioning)", Tier2PhaseLoopPost, []string{"IntRanges"}, nil, TableArrayStaticBoundsPass),
		tier2PassModuleWith("ScalarPromotion", Tier2PhaseLoopPost, []string{"FixedShapeTables"}, nil, ScalarPromotionPass),
		tier2PassModuleWith("TableArrayDataPtrFact", Tier2PhaseLoopPost, []string{"FixedShapeTables"}, []string{"TableArrayDataPtrs"}, TableArrayDataPtrFactPass),
	}
}
