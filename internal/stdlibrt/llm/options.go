package llm

import (
	"context"

	"github.com/never-labs/gscript/internal/runtime"
)

type Options struct {
	Call            runtime.ScriptFunctionCaller
	Provider        func() runtime.LLMProvider
	ProviderFactory func() runtime.LLMProviderFactory
	MaxHostResult   func() int64
	Context         func() context.Context
	Trace           runtime.LLMTraceSink
}
