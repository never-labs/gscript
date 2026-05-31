package modules

import (
	"context"

	"github.com/never-labs/gscript/internal/runtime"
)

type LLMOptions struct {
	Call            runtime.ScriptFunctionCaller
	Provider        func() runtime.LLMProvider
	ProviderFactory func() runtime.LLMProviderFactory
	MaxHostResult   func() int64
	Context         func() context.Context
	Trace           runtime.LLMTraceSink
}

// InstallLLM registers the public LLM-facing standard-library bindings.
func InstallLLM(installer runtime.StdlibInstaller, opts LLMOptions) {
	if installer == nil {
		return
	}
	llmLib := BuildLLMLib(
		opts.Call,
		opts.Provider,
		opts.ProviderFactory,
		opts.MaxHostResult,
		opts.Context,
		opts.Trace,
	)
	installer.RegisterTable("llm", llmLib)
	installer.RegisterAlias("toolof", llmLib.RawGetString("toolof"))
	installer.RegisterTable("msg", BuildLLMMessageLib())
	installer.RegisterTable("history", BuildLLMHistoryLib())
	installer.RegisterTable("chat", BuildChatLib())
	installer.RegisterTable("loop", BuildLLMLoopLib(
		opts.Call,
		opts.Provider,
		opts.MaxHostResult,
		opts.Context,
		opts.Trace,
	))
}
