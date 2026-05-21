package methodjit

func init() {
	RegisterModuleBuilder(Tier2PhaseStringNative, 40, func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		return tier2StringNativeModules()
	})
}

func tier2StringNativeModules() []Tier2OptimizerModule {
	return []Tier2OptimizerModule{
		{
			Name:     "StringNativeCleanup",
			Phase:    Tier2PhaseStringNative,
			Requires: nil,
			Provides: analysisFacts(AnalysisFactStringConstTables, AnalysisFactStringFormatPatterns, AnalysisFactStringSplitSubSpecs),
			RunWithContext: func(fn *Function, opts *Tier2PipelineOpts, ctx *Tier2OptimizerContext) (*Function, error) {
				out, notes := StringNativeCleanupPass(fn)
				if ctx != nil && len(notes) > 0 {
					ctx.IntrinsicNotes = append(ctx.IntrinsicNotes, notes...)
				}
				return out, nil
			},
		},
	}
}
