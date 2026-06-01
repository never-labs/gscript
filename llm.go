package gscript

import (
	llm "github.com/never-labs/gscript/llm"
	llmcommand "github.com/never-labs/gscript/llm/command"
)

// ClassifyLLMProviderError returns a stable diagnostic category for provider
// errors without inspecting prompts, messages, or tokens.
//
// Deprecated: use llm.ClassifyProviderError from github.com/never-labs/gscript/llm.
func ClassifyLLMProviderError(err error) string {
	return llm.ClassifyProviderError(err)
}

// WithLLMProvider installs the provider used by llm.turn. A nil provider makes
// llm.turn return a provider error.
func WithLLMProvider(provider llm.Provider) Option {
	return func(o *vmOptions) { o.llmProvider = provider }
}

// WithLLMProviderFactory installs a constructor for script-declared models {}
// provider configs. Host-injected providers still take precedence for ordinary
// llm.turn calls; this hook is used when no provider is otherwise configured.
func WithLLMProviderFactory(factory llm.ProviderFactory) Option {
	return func(o *vmOptions) { o.llmProviderFactory = factory }
}

// WithLLMTrace installs a host-side metadata trace sink for llm.turn/react.
// Events intentionally omit prompt text and tool result values by default.
func WithLLMTrace(sink llm.TraceSink) Option {
	return func(o *vmOptions) { o.llmTraceSink = sink }
}

// WithLLMRecorder records provider turns after execution. It is intended for
// reproducible tests and offline agent evaluation. Recorder entries include the
// request/result protocol shape, so hosts should store them according to their
// own prompt-retention policy.
func WithLLMRecorder(sink llm.RecordSink) Option {
	return func(o *vmOptions) { o.llmRecordSink = sink }
}

// WithLLMReplay installs a deterministic sequential provider backed by records
// produced by WithLLMRecorder. Each incoming request is checked against the next
// recorded request before the recorded result or error is returned.
func WithLLMReplay(records []llm.Record) Option {
	return WithLLMProvider(llm.NewReplayProvider(records))
}

// WithLLMCommand installs a simple command-backed provider. It is intended for
// local tooling and tests. The prompt is rendered from the request messages and
// passed as the final command argument.
//
// Deprecated: use WithLLMProvider with command.Provider from
// github.com/never-labs/gscript/llm/command.
func WithLLMCommand(command string, args ...string) Option {
	return WithLLMProvider(llmcommand.Provider{Command: command, Args: args})
}
