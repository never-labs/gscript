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
//
// The implementation deliberately reuses the runtime builders while the LLM
// adapter logic still lives in internal/runtime. This keeps the public binding
// layout in stdlibrt without forking provider, message, history, chat, or loop
// behavior.
func InstallLLM(installer runtime.StdlibInstaller, opts LLMOptions) {
	if installer == nil {
		return
	}
	llmLib := runtime.BuildLLMLib(
		opts.Call,
		opts.Provider,
		opts.ProviderFactory,
		opts.MaxHostResult,
		opts.Context,
		opts.Trace,
	)
	installer.RegisterTable("llm", llmLib)
	installer.RegisterAlias("toolof", llmLib.RawGetString("toolof"))
	installer.RegisterTable("msg", runtime.BuildLLMMessageLib())
	installer.RegisterTable("history", runtime.BuildLLMHistoryLib())
	installer.RegisterTable("chat", runtime.BuildChatLib())
	installer.RegisterTable("loop", runtime.BuildLLMLoopLib(
		opts.Call,
		opts.Provider,
		opts.MaxHostResult,
		opts.Context,
		opts.Trace,
	))
}
