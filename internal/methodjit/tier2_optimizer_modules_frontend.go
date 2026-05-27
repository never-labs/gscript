package methodjit

import (
	"os"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

func init() {
	RegisterModuleBuilder(Tier2PhaseEarlyCanonical, 10, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2EarlyCanonicalModules(ctxGlobals(ctx))
	})
	RegisterModuleBuilder(Tier2PhaseInlineCall, 20, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2InlineCallModules(ctxGlobals(ctx), ctxInlineMaxSize(ctx))
	})
	RegisterModuleBuilder(Tier2PhaseCallLower, 30, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2CallLoweringModules(ctxSpecializationGlobals(ctx))
	})
	RegisterModuleBuilder(Tier2PhasePostRewrite, 60, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2PostRewriteModules()
	})
	RegisterModuleBuilder(Tier2PhaseFinalCall, 140, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2FinalCallModules(ctxSpecializationGlobals(ctx))
	})
}

func tier2EarlyCanonicalModules(globals map[string]*vm.FuncProto) []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWith("SimplifyPhis", Tier2PhaseEarlyCanonical, nil, nil, SimplifyPhisPass),
		tier2PassModuleWith("TypeSpecialize", Tier2PhaseEarlyCanonical, nil, nil, TypeSpecializePass),
		{
			Name:     "Intrinsic",
			Phase:    Tier2PhaseEarlyCanonical,
			Requires: nil,
			Provides: analysisFacts(AnalysisFactInlineComplete),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				out, notes := IntrinsicPass(fn)
				if ctx != nil && len(notes) > 0 {
					ctx.IntrinsicNotes = append(ctx.IntrinsicNotes, notes...)
				}
				return out, nil
			},
		},
		{
			Name:     "GlobalConstSpecialization",
			Phase:    Tier2PhaseEarlyCanonical,
			Requires: nil,
			Provides: nil,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				if opts == nil || len(opts.GlobalConstValues) == 0 {
					return fn, nil
				}
				return GlobalConstSpecializationPassWith(GlobalConstSpecializationConfig{
					Values:             opts.GlobalConstValues,
					Globals:            opts.DependencyContext.Globals,
					DependencyRegistry: ctxDependencyRegistry(ctx),
				})(fn)
			},
		},
		tier2PassModuleWith("TypeSpecialize (post-intrinsic)", Tier2PhaseEarlyCanonical, nil, nil, TypeSpecializePass),
		{
			Name:     "FixedShapeTableFacts (pre-inline)",
			Phase:    Tier2PhaseEarlyCanonical,
			Requires: nil,
			Provides: fixedShapeTableFacts(),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				allowed := allowedDomainsForModule(nil, fixedShapeTableFacts(), nil, "FixedShapeTableFacts (pre-inline)")
				return FixedShapeTableFactsPassCtx(fixedShapeTableFactsConfigFromOpts(globals, opts))(newPassContext(fn, opts, allowed, passContextEnforce))
			},
		},
	}
}

func tier2InlineCallModules(globals map[string]*vm.FuncProto, maxSize int) []Tier2OptimizerModule {
	modules := []Tier2OptimizerModule{
		tier2PassModuleWithCtx("FieldShapeCallSplitPreInline", Tier2PhaseInlineCall, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, FieldShapeCallSplitPreInlinePassCtx),
		{
			Name:     "Inline",
			Phase:    Tier2PhaseInlineCall,
			Requires: analysisFacts(AnalysisFactInlineComplete, AnalysisFactFixedShapeTables),
			Provides: analysisFacts(AnalysisFactSpecDependencyProtos),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				if len(globals) == 0 && !hasInlineFeedbackCallee(fn) {
					if countOpHelper(fn, OpCall) > 0 {
						functionRemarks(fn).Add("Inline", "missed", 0, 0, OpCall,
							"inline pass skipped because no inline globals were available")
					}
					if ctx != nil {
						ctx.InlineApplied = false
					}
					return fn, nil
				}
				config := InlineConfig{
					Globals:                  globals,
					MaxSize:                  maxSize,
					MaxRecursion:             8,
					MaxCumulativeSize:        120,
					MaxHotLoopCumulativeSize: 600,
					PreserveSelfCalls:        staticallyCallsOnlySelf(fn.Proto),
				}
				out, err := InlinePassWith(config)(fn)
				if ctx != nil {
					ctx.InlineApplied = err == nil
				}
				return out, err
			},
		},
		tier2PostInlinePassModuleWith("SimplifyPhis (post-inline)", nil, SimplifyPhisPass),
		tier2PostInlinePassModuleWith("SourceFeedbackRefresh (post-inline)", nil, SourceFeedbackRefreshPass),
		{
			Name:     "Intrinsic (post-inline)",
			Phase:    Tier2PhaseInlineCall,
			Requires: nil,
			Provides: nil,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				if ctx == nil || !ctx.InlineApplied {
					return fn, nil
				}
				out, notes := IntrinsicPass(fn)
				if len(notes) > 0 {
					ctx.IntrinsicNotes = append(ctx.IntrinsicNotes, notes...)
				}
				return out, nil
			},
		},
		tier2PostInlinePassModuleWith("TypeSpecialize (post-inline)", nil, TypeSpecializePass),
		{
			Name:     "FixedShapeTableFacts (post-inline)",
			Phase:    Tier2PhaseInlineCall,
			Requires: nil,
			Updates:  fixedShapeTableFacts(),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				if ctx == nil || !ctx.InlineApplied {
					return fn, nil
				}
				allowed := allowedDomainsForModule(nil, nil, fixedShapeTableFacts(), "FixedShapeTableFacts (post-inline)")
				return FixedShapeTableFactsPassCtx(fixedShapeTableFactsConfigFromOpts(globals, opts))(newPassContext(fn, opts, allowed, passContextEnforce))
			},
		},
	}
	return modules
}

func tier2PostInlinePassModule(name string, pass PassFunc) Tier2OptimizerModule {
	return Tier2OptimizerModule{
		Name:     name,
		Phase:    Tier2PhaseInlineCall,
		Requires: nil,
		Provides: nil,
		RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
			if ctx == nil || !ctx.InlineApplied {
				return fn, nil
			}
			return pass(fn)
		},
	}
}

func tier2PostInlinePassModuleWith(name string, provides []AnalysisFact, pass PassFunc) Tier2OptimizerModule {
	return Tier2OptimizerModule{
		Name:     name,
		Phase:    Tier2PhaseInlineCall,
		Requires: nil,
		Provides: provides,
		RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
			if ctx == nil || !ctx.InlineApplied {
				return fn, nil
			}
			return pass(fn)
		},
	}
}

func tier2CallLoweringModules(specializationGlobals map[string]*vm.FuncProto) []Tier2OptimizerModule {
	guardedConstCallFoldAllowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactCallABIs),
		analysisFacts(AnalysisFactGuardedConstCallFolds),
		nil,
		"GuardedConstCallFold",
	)
	callSiteRuntimeSpecializationAllowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactCallABIs),
		analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches),
		nil,
		"CallSiteRuntimeSpecializationExit",
	)
	return []Tier2OptimizerModule{
		{
			Name:     "CallABI",
			Phase:    Tier2PhaseCallLower,
			Requires: analysisFacts(AnalysisFactFixedShapeTables),
			Provides: analysisFacts(AnalysisFactCallABIs, AnalysisFactGlobalArrayElementFacts),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				globalArrayFacts := mergeGlobalArrayElementFacts(fn.Analysis.GlobalFacts().GlobalArrayElementFactsMap(), collectStableGlobalArrayElementFacts(fn))
				fn.Analysis.GlobalFacts().SetGlobalArrayElementFacts(cloneFixedShapeTableFactMap(globalArrayFacts))
				return AnnotateCallABIsPass(CallABIAnnotationConfig{
					Globals:                 ctxGlobals(ctx),
					NumericGlobalValues:     fn.Analysis.GlobalFacts().NumericGlobalValuesMap(),
					GlobalArrayElementFacts: globalArrayFacts,
					DependencyRegistry:      ctxDependencyRegistry(ctx),
				})(fn)
			},
		},
		tier2PassModuleWithCtx("CallReturnProjection", Tier2PhaseCallLower, callReturnProjectionFacts(), nil, CallReturnProjectionPassCtx),
		tier2PassModuleWith("ModularCallFloorReduce", Tier2PhaseCallLower, nil, nil, ModularCallFloorReducePass),
		tier2PassModuleWithCtx("CallResultRangeGuard", Tier2PhaseCallLower, callResultRangeGuardFacts(), nil, CallResultRangeGuardPassCtx),
		tier2PassModuleWith("ConstProp", Tier2PhaseCallLower, nil, nil, ConstPropPass),
		{
			Name:     "GuardedConstCallFold",
			Phase:    Tier2PhaseCallLower,
			Requires: analysisFacts(AnalysisFactCallABIs),
			Provides: analysisFacts(AnalysisFactGuardedConstCallFolds),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return GuardedConstCallFoldPassCtx(specializationGlobals)(newPassContext(fn, opts, guardedConstCallFoldAllowed, passContextEnforce))
			},
		},
		{
			Name:     "CallSiteRuntimeSpecializationExit",
			Phase:    Tier2PhaseCallLower,
			Requires: analysisFacts(AnalysisFactCallABIs),
			Provides: analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return CallSiteRuntimeSpecializationExitPassCtx(specializationGlobals)(newPassContext(fn, opts, callSiteRuntimeSpecializationAllowed, passContextEnforce))
			},
		},
	}
}

func callResultRangeGuardFacts() []AnalysisFact {
	return analysisFacts(
		AnalysisFactSpecDependencyProtos,
		AnalysisFactCallABIs,
		AnalysisFactFieldPolyShapeFacts,
	)
}

func callReturnProjectionFacts() []AnalysisFact {
	return analysisFacts(
		AnalysisFactCallABIs,
		AnalysisFactSpecDependencyProtos,
		AnalysisFactFieldPolyShapeFacts,
	)
}

func optsNumericGlobalValuesByName(fn *Function, opts *Tier2PipelineOpts) map[string]runtime.Value {
	if fn == nil || fn.Proto == nil || opts == nil || len(opts.GlobalConstValues) == 0 {
		return nil
	}
	out := make(map[string]runtime.Value)
	for constIdx, v := range opts.GlobalConstValues {
		if constIdx < 0 || constIdx >= len(fn.Proto.Constants) {
			continue
		}
		c := fn.Proto.Constants[constIdx]
		if !c.IsString() || (!v.IsInt() && !v.IsFloat()) {
			continue
		}
		out[c.Str()] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func tier2PostRewriteModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		tier2PassModuleWithCtx("CallReturnProjection (post-rewrite)", Tier2PhasePostRewrite, callReturnProjectionFacts(), nil, CallReturnProjectionPassCtx),
		tier2PassModuleWith("ModularCallFloorReduce (post-rewrite)", Tier2PhasePostRewrite, nil, nil, ModularCallFloorReducePass),
		tier2PassModuleWithCtx("CallResultRangeGuard (post-rewrite)", Tier2PhasePostRewrite, callResultRangeGuardFacts(), nil, CallResultRangeGuardPassCtx),
		tier2PassModuleWith("DCE", Tier2PhasePostRewrite, nil, nil, DCEPass),
		{
			Name:     "TypeSpecialize (post-escape)",
			Phase:    Tier2PhasePostRewrite,
			Requires: nil,
			Provides: nil,
			Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
				return runPostRewriteTypeSpecialize(fn, opts, "post-escape")
			},
		},
	}
}

func tier2FinalCallModules(specializationGlobals map[string]*vm.FuncProto) []Tier2OptimizerModule {
	callSiteRuntimeSpecializationFinalAllowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactCallABIs),
		nil,
		analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches),
		"CallSiteRuntimeSpecializationExit (final)",
	)
	modules := []Tier2OptimizerModule{
		{
			Name:     "CallABI (final)",
			Phase:    Tier2PhaseFinalCall,
			Requires: analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactIntRanges),
			Updates:  analysisFacts(AnalysisFactCallABIs, AnalysisFactGlobalArrayElementFacts),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				globalArrayFacts := mergeGlobalArrayElementFacts(fn.Analysis.GlobalFacts().GlobalArrayElementFactsMap(), collectStableGlobalArrayElementFacts(fn))
				fn.Analysis.GlobalFacts().SetGlobalArrayElementFacts(cloneFixedShapeTableFactMap(globalArrayFacts))
				return AnnotateCallABIsPass(CallABIAnnotationConfig{
					Globals:                 ctxGlobals(ctx),
					NumericGlobalValues:     fn.Analysis.GlobalFacts().NumericGlobalValuesMap(),
					GlobalArrayElementFacts: globalArrayFacts,
					DependencyRegistry:      ctxDependencyRegistry(ctx),
				})(fn)
			},
		},
		{
			Name:     "CallSiteRuntimeSpecializationExit (final)",
			Phase:    Tier2PhaseFinalCall,
			Requires: analysisFacts(AnalysisFactCallABIs),
			Updates:  analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return CallSiteRuntimeSpecializationExitPassCtx(specializationGlobals)(newPassContext(fn, opts, callSiteRuntimeSpecializationFinalAllowed, passContextEnforce))
			},
		},
		tier2PassModuleWithCtx("CallReturnProjection (final)", Tier2PhaseFinalCall, callReturnProjectionFacts(), nil, CallReturnProjectionPassCtx),
		tier2PassModuleWith("ModularCallFloorReduce (final)", Tier2PhaseFinalCall, nil, nil, ModularCallFloorReducePass),
		tier2PassModuleWithCtx("CallResultRangeGuard (final)", Tier2PhaseFinalCall, callResultRangeGuardFacts(), nil, CallResultRangeGuardPassCtx),
		tier2PassModuleWithCtx("FieldCallPolyLenFusion", Tier2PhaseFinalCall, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, FieldCallPolyLenFusionPassCtx),
		tier2PassModuleWithCtxUpdates("RangeAnalysis (post-final-call)", Tier2PhaseFinalCall, nil, rangeAnalysisFacts(), RangeAnalysisPassCtx),
	}
	if os.Getenv("GSCRIPT_FIELD_SHAPE_SPLIT") == "1" {
		modules = append(modules, tier2PassModuleWithCtx("FieldShapeCallSplit (experimental)", Tier2PhaseFinalCall, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, FieldShapeCallSplitPassCtx))
	}
	return modules
}
