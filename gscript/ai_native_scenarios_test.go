package gscript_test

import gs "github.com/never-labs/gscript/gscript"

func aiNativeScenarioOptions(provider gs.LLMProvider, opts ...gs.Option) []gs.Option {
	base := []gs.Option{
		gs.WithLibs(gs.LibString | gs.LibLLM),
		gs.WithLLMProvider(provider),
	}
	return append(base, opts...)
}
