package gscript

import (
	llm "github.com/never-labs/gscript/llm"
	llmcommand "github.com/never-labs/gscript/llm/command"
)

// LLMProvider is the Go embedding hook behind the llm standard library.
// Implementations can call a remote model API, a local model, or a test double.
type LLMProvider = llm.Provider

type LLMMessage = llm.Message
type LLMTool = llm.Tool
type LLMToolCall = llm.ToolCall
type LLMTurnRequest = llm.TurnRequest
type LLMTurnUsage = llm.TurnUsage
type LLMTurnResult = llm.TurnResult
type LLMTraceEvent = llm.TraceEvent
type LLMTraceSink = llm.TraceSink
type LLMRecordSink = llm.RecordSink

// LLMProviderConfig is the public host-side shape of one script models {}
// provider entry.
type LLMProviderConfig = llm.ProviderConfig

type LLMProviderFactory = llm.ProviderFactory

type LLMRecord = llm.Record

const (
	LLMProviderErrorNetwork   = llm.ProviderErrorNetwork
	LLMProviderErrorAuth      = llm.ProviderErrorAuth
	LLMProviderErrorRateLimit = llm.ProviderErrorRateLimit
	LLMProviderErrorRequest   = llm.ProviderErrorRequest
	LLMProviderErrorProvider  = llm.ProviderErrorProvider
)

// ClassifyLLMProviderError returns a stable diagnostic category for provider
// errors without inspecting prompts, messages, or tokens.
func ClassifyLLMProviderError(err error) string {
	return llm.ClassifyProviderError(err)
}

// LLMTraceRecorder is a thread-safe trace sink for tests, diagnostics, and
// host-side observability. Pass recorder.Record to WithLLMTrace.
type LLMTraceRecorder = llm.TraceRecorder

func NewLLMTraceRecorder(events ...LLMTraceEvent) *LLMTraceRecorder {
	return llm.NewTraceRecorder(events...)
}

// LLMRecorder is a thread-safe record sink for deterministic LLM replay
// fixtures. Pass recorder.Record to WithLLMRecorder.
type LLMRecorder = llm.Recorder

func NewLLMRecorder(records ...LLMRecord) *LLMRecorder {
	return llm.NewRecorder(records...)
}

func LoadLLMRecorder(path string) (*LLMRecorder, error) {
	return llm.LoadRecorder(path)
}

// WithLLMProvider installs the provider used by llm.turn. A nil provider makes
// llm.turn return a provider error.
func WithLLMProvider(provider LLMProvider) Option {
	return func(o *vmOptions) { o.llmProvider = provider }
}

// WithLLMProviderFactory installs a constructor for script-declared models {}
// provider configs. Host-injected providers still take precedence for ordinary
// llm.turn calls; this hook is used when no provider is otherwise configured.
func WithLLMProviderFactory(factory LLMProviderFactory) Option {
	return func(o *vmOptions) { o.llmProviderFactory = factory }
}

// WithLLMTrace installs a host-side metadata trace sink for llm.turn/react.
// Events intentionally omit prompt text and tool result values by default.
func WithLLMTrace(sink LLMTraceSink) Option {
	return func(o *vmOptions) { o.llmTraceSink = sink }
}

// WithLLMRecorder records provider turns after execution. It is intended for
// reproducible tests and offline agent evaluation. Recorder entries include the
// request/result protocol shape, so hosts should store them according to their
// own prompt-retention policy.
func WithLLMRecorder(sink LLMRecordSink) Option {
	return func(o *vmOptions) { o.llmRecordSink = sink }
}

// WithLLMReplay installs a deterministic sequential provider backed by records
// produced by WithLLMRecorder. Each incoming request is checked against the next
// recorded request before the recorded result or error is returned.
func WithLLMReplay(records []LLMRecord) Option {
	return WithLLMProvider(NewLLMReplayProvider(records))
}

// WithLLMCommand installs a simple command-backed provider. It is intended for
// local tooling and tests. The prompt is rendered from the request messages and
// passed as the final command argument.
func WithLLMCommand(command string, args ...string) Option {
	return WithLLMProvider(CommandLLMProvider{Command: command, Args: args})
}

type CommandLLMProvider = llmcommand.Provider

// LLMReplayProvider is a deterministic test provider for recorded LLM turns.
type LLMReplayProvider = llm.ReplayProvider

// LLMReplayMismatchError reports a deterministic replay request mismatch.
type LLMReplayMismatchError = llm.ReplayMismatchError

// LLMReplayExhaustedError reports that replay consumed all recorded turns.
type LLMReplayExhaustedError = llm.ReplayExhaustedError

func NewLLMReplayProvider(records []LLMRecord) *LLMReplayProvider {
	return llm.NewReplayProvider(records)
}

// MarshalLLMRecords serializes replay records as stable JSON for deterministic
// tests and offline evaluation fixtures.
func MarshalLLMRecords(records []LLMRecord) ([]byte, error) {
	return llm.MarshalRecords(records)
}

// UnmarshalLLMRecords parses replay fixture JSON. Integer-valued JSON numbers
// are normalized back to int64 so strict replay matching remains compatible
// with values produced by the GScript runtime.
func UnmarshalLLMRecords(data []byte) ([]LLMRecord, error) {
	return llm.UnmarshalRecords(data)
}

func SaveLLMRecords(path string, records []LLMRecord) error {
	return llm.SaveRecords(path, records)
}

func LoadLLMRecords(path string) ([]LLMRecord, error) {
	return llm.LoadRecords(path)
}
