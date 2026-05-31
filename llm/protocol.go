package llm

import (
	"context"
	"errors"
	"net"
)

// Provider is the Go embedding hook behind the llm standard library.
// Implementations can call a remote model API, a local model, or a test double.
type Provider interface {
	Turn(context.Context, TurnRequest) (TurnResult, error)
}

type Message struct {
	Role      string
	Text      string
	ToolCall  *ToolCall
	ToolUseID string
	Value     any
	Error     string
}

type Tool struct {
	Name        string
	Description string
	Params      []string
	Requires    []string
	Schema      any
}

type ToolCall struct {
	ID   string
	Tool string
	Args map[string]any
}

type TurnRequest struct {
	Model          string
	Messages       []Message
	Tools          []Tool
	ForceTool      string
	MaxTokens      int64
	Temperature    *float64
	TopP           *float64
	ResponseFormat any
	Stream         bool
	Stop           []string
	Metadata       map[string]string
}

type TurnUsage struct {
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	LatencyMS    int64
}

type TurnResult struct {
	Status string
	Text   string
	Calls  []ToolCall
	Reason string
	Usage  TurnUsage
}

type TraceEvent struct {
	Type         string
	Model        string
	Status       string
	Tool         string
	CallID       string
	Token        string
	ErrorKind    string
	Message      string
	Step         int64
	Attempt      int64
	MessageCount int
	ToolCount    int
	Store        bool
	Usage        TurnUsage
}

type TraceSink func(TraceEvent)
type RecordSink func(Record)

// ProviderConfig is the public host-side shape of one script models {}
// provider entry.
type ProviderConfig struct {
	Name          string
	Protocol      string
	BaseURL       string
	APIKey        string
	ProviderModel string
	Provider      string
}

type ProviderFactory func(ProviderConfig) (Provider, error)

type Record struct {
	Request TurnRequest
	Result  TurnResult
	Error   string
}

const (
	ProviderErrorNetwork   = "network"
	ProviderErrorAuth      = "auth"
	ProviderErrorRateLimit = "rate_limit"
	ProviderErrorRequest   = "request"
	ProviderErrorProvider  = "provider"
)

type providerErrorKind interface {
	LLMProviderErrorKind() string
}

// ClassifyProviderError returns a stable diagnostic category for provider
// errors without inspecting prompts, messages, or tokens.
func ClassifyProviderError(err error) string {
	if err == nil {
		return ""
	}
	var typed providerErrorKind
	if errors.As(err, &typed) {
		if kind := typed.LLMProviderErrorKind(); kind != "" {
			return kind
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ProviderErrorNetwork
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ProviderErrorNetwork
	}
	return ProviderErrorProvider
}
