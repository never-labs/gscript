package methodjit

import (
	"os"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
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
	fixedShapeFacts := fixedShapeTableFacts()
	fixedShapePreInlineAllowed := allowedDomainsForModule(nil, fixedShapeFacts, nil)
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
			Provides: fixedShapeFacts,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return FixedShapeTableFactsPassCtx(fixedShapeTableFactsConfigFromOpts(globals, opts))(newPassContext(fn, opts, fixedShapePreInlineAllowed, passContextEnforce))
			},
		},
	}
}

func tier2InlineCallModules(globals map[string]*vm.FuncProto, maxSize int) []Tier2OptimizerModule {
	fixedShapeFacts := fixedShapeTableFacts()
	fixedShapePostInlineAllowed := allowedDomainsForModule(nil, nil, fixedShapeFacts)
	inlineAllowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactInlineComplete, AnalysisFactFixedShapeTables),
		analysisFacts(AnalysisFactSpecDependencyProtos),
		nil,
		analysisFacts(AnalysisFactGlobals),
	)
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
				passCtx := newPassContext(fn, opts, inlineAllowed, passContextEnforce)
				config := InlineConfig{
					Globals:                  globals,
					GlobalFacts:              passCtx.Global(),
					SpeculationFacts:         passCtx.Speculation(),
					TableShapes:              passCtx.TableShape(),
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
		{
			Name:    "SourceFeedbackRefresh (post-inline)",
			Phase:   Tier2PhaseInlineCall,
			Updates: analysisFacts(AnalysisFactFieldPolyShapeFacts),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				if ctx == nil || !ctx.InlineApplied {
					return fn, nil
				}
				return SourceFeedbackRefreshPassCtx(newPassContext(fn, opts, sourceFeedbackRefreshAllowedDomains, passContextEnforce))
			},
		},
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
			Updates:  fixedShapeFacts,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				if ctx == nil || !ctx.InlineApplied {
					return fn, nil
				}
				return FixedShapeTableFactsPassCtx(fixedShapeTableFactsConfigFromOpts(globals, opts))(newPassContext(fn, opts, fixedShapePostInlineAllowed, passContextEnforce))
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
	callABIRequires := analysisFacts(AnalysisFactFixedShapeTables)
	callABIProvides := analysisFacts(AnalysisFactCallABIs, AnalysisFactGlobalArrayElementFacts)
	callABIUpdates := analysisFacts(AnalysisFactSpecDependencyProtos)
	callABIAllowed := allowedDomainsForModule(
		callABIRequires,
		callABIProvides,
		callABIUpdates,
		analysisFacts(AnalysisFactIntRanges),
	)
	guardedConstCallFoldRequires := analysisFacts(AnalysisFactCallABIs)
	guardedConstCallFoldProvides := analysisFacts(AnalysisFactGuardedConstCallFolds)
	guardedConstCallFoldAllowed := allowedDomainsForModule(
		guardedConstCallFoldRequires,
		guardedConstCallFoldProvides,
		nil,
	)
	callSiteRuntimeSpecializationRequires := analysisFacts(AnalysisFactCallABIs)
	callSiteRuntimeSpecializationProvides := analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches)
	callSiteRuntimeSpecializationAllowed := allowedDomainsForModule(
		callSiteRuntimeSpecializationRequires,
		callSiteRuntimeSpecializationProvides,
		nil,
	)
	return []Tier2OptimizerModule{
		{
			Name:          "CallABI",
			Phase:         Tier2PhaseCallLower,
			Requires:      callABIRequires,
			Provides:      callABIProvides,
			Updates:       callABIUpdates,
			OptionalReads: analysisFacts(AnalysisFactIntRanges),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return runCallABIModule(fn, ctx, newPassContext(fn, opts, callABIAllowed, passContextEnforce))
			},
		},
		tier2PassModuleWithCtx("CallReturnProjection", Tier2PhaseCallLower, callReturnProjectionFacts(), nil, CallReturnProjectionPassCtx),
		tier2PassModuleWith("ModularCallFloorReduce", Tier2PhaseCallLower, nil, nil, ModularCallFloorReducePass),
		tier2PassModuleWithCtxOptionalReads("CallResultRangeGuard", Tier2PhaseCallLower, callResultRangeGuardFacts(), nil, analysisFacts(AnalysisFactGlobals), CallResultRangeGuardPassCtx),
		tier2PassModuleWith("ConstProp", Tier2PhaseCallLower, nil, nil, ConstPropPass),
		{
			Name:     "GuardedConstCallFold",
			Phase:    Tier2PhaseCallLower,
			Requires: guardedConstCallFoldRequires,
			Provides: guardedConstCallFoldProvides,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return GuardedConstCallFoldPassCtx(specializationGlobals)(newPassContext(fn, opts, guardedConstCallFoldAllowed, passContextEnforce))
			},
		},
		{
			Name:     "CallSiteRuntimeSpecializationExit",
			Phase:    Tier2PhaseCallLower,
			Requires: callSiteRuntimeSpecializationRequires,
			Provides: callSiteRuntimeSpecializationProvides,
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

func runCallABIModule(fn *Function, optCtx *Tier2OptimizerContext, passCtx *PassContext) (*Function, error) {
	if fn == nil {
		return AnnotateCallABIsPass(CallABIAnnotationConfig{
			Globals:            ctxGlobals(optCtx),
			DependencyRegistry: ctxDependencyRegistry(optCtx),
		})(fn)
	}
	fn.ensureAnalysis()
	globalFacts := passCtx.Global()
	tableShapes := passCtx.TableShape()
	globalArrayFacts := mergeGlobalArrayElementFacts(globalFacts.GlobalArrayElementFactsMap(), collectStableGlobalArrayElementFactsWithFacts(fn, tableShapes))
	globalFacts.SetGlobalArrayElementFacts(cloneFixedShapeTableFactMap(globalArrayFacts))
	return AnnotateCallABIsPass(CallABIAnnotationConfig{
		Globals:                 ctxGlobals(optCtx),
		NumericGlobalValues:     globalFacts.NumericGlobalValuesMap(),
		GlobalArrayElementFacts: globalArrayFacts,
		TableShapes:             tableShapes,
		CallFacts:               passCtx.Call(),
		NumericFacts:            passCtx.Numeric(),
		SpeculationFacts:        passCtx.Speculation(),
		DependencyRegistry:      ctxDependencyRegistry(optCtx),
	})(fn)
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
		tier2PassModuleWithCtxOptionalReads("CallResultRangeGuard (post-rewrite)", Tier2PhasePostRewrite, callResultRangeGuardFacts(), nil, analysisFacts(AnalysisFactGlobals), CallResultRangeGuardPassCtx),
		tier2PassModuleWith("DCE", Tier2PhasePostRewrite, nil, nil, DCEPass),
		{
			Name:     "TypeSpecialize (post-escape)",
			Phase:    Tier2PhasePostRewrite,
			Requires: nil,
			Provides: nil,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, _ *Tier2OptimizerContext) (*Function, error) {
				return runPostRewriteTypeSpecialize(fn, opts, "post-escape")
			},
		},
	}
}

func tier2FinalCallModules(specializationGlobals map[string]*vm.FuncProto) []Tier2OptimizerModule {
	callABIFinalRequires := analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactIntRanges)
	callABIFinalUpdates := analysisFacts(AnalysisFactCallABIs, AnalysisFactGlobalArrayElementFacts, AnalysisFactSpecDependencyProtos)
	callABIFinalAllowed := allowedDomainsForModule(
		callABIFinalRequires,
		nil,
		callABIFinalUpdates,
	)
	callSiteRuntimeSpecializationFinalRequires := analysisFacts(AnalysisFactCallABIs)
	callSiteRuntimeSpecializationFinalUpdates := analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches)
	callSiteRuntimeSpecializationFinalAllowed := allowedDomainsForModule(
		callSiteRuntimeSpecializationFinalRequires,
		nil,
		callSiteRuntimeSpecializationFinalUpdates,
	)
	modules := []Tier2OptimizerModule{
		{
			Name:     "CallABI (final)",
			Phase:    Tier2PhaseFinalCall,
			Requires: callABIFinalRequires,
			Updates:  callABIFinalUpdates,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return runCallABIModule(fn, ctx, newPassContext(fn, opts, callABIFinalAllowed, passContextEnforce))
			},
		},
		{
			Name:     "CallSiteRuntimeSpecializationExit (final)",
			Phase:    Tier2PhaseFinalCall,
			Requires: callSiteRuntimeSpecializationFinalRequires,
			Updates:  callSiteRuntimeSpecializationFinalUpdates,
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				return CallSiteRuntimeSpecializationExitPassCtx(specializationGlobals)(newPassContext(fn, opts, callSiteRuntimeSpecializationFinalAllowed, passContextEnforce))
			},
		},
		tier2PassModuleWithCtx("CallReturnProjection (final)", Tier2PhaseFinalCall, callReturnProjectionFacts(), nil, CallReturnProjectionPassCtx),
		tier2PassModuleWith("ModularCallFloorReduce (final)", Tier2PhaseFinalCall, nil, nil, ModularCallFloorReducePass),
		tier2PassModuleWithCtxOptionalReads("CallResultRangeGuard (final)", Tier2PhaseFinalCall, callResultRangeGuardFacts(), nil, analysisFacts(AnalysisFactGlobals), CallResultRangeGuardPassCtx),
		tier2PassModuleWithCtx("FieldCallPolyLenFusion", Tier2PhaseFinalCall, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, FieldCallPolyLenFusionPassCtx),
		tier2PassModuleWithCtxUpdates("RangeAnalysis (post-final-call)", Tier2PhaseFinalCall, nil, rangeAnalysisFacts(), RangeAnalysisPassCtx),
		tier2PassModuleWith("QQueryHotPath", Tier2PhaseFinalCall, nil, nil, QQueryHotPathRemarkPass),
		tier2PassModuleWith("QQueryNativeLowering", Tier2PhaseFinalCall, nil, nil, QQueryNativeLoweringPass),
	}
	if os.Getenv("LEIA_FIELD_SHAPE_SPLIT") == "1" {
		modules = append(modules, tier2PassModuleWithCtx("FieldShapeCallSplit (experimental)", Tier2PhaseFinalCall, analysisFacts(AnalysisFactFieldPolyShapeFacts), nil, FieldShapeCallSplitPassCtx))
	}
	return modules
}
