package bind

import (
	"context"

	"github.com/never-labs/leia/internal/runtime"
)

type LLMOptions struct {
	Call            runtime.ScriptFunctionCaller
	Provider        func() runtime.LLMProvider
	ProviderFactory func() runtime.LLMProviderFactory
	MaxHostResult   func() int64
	Context         func() context.Context
	Trace           runtime.LLMTraceSink
}
