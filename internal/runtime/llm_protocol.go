package runtime

import (
	"context"
	"errors"
	"net"
)

// LLMProvider is the host boundary behind llm.turn. Implementations may call a
// remote API, a local model, or a test double; the runtime only sees this small
// protocol shape.
type LLMProvider interface {
	Turn(context.Context, LLMTurnRequest) (LLMTurnResult, error)
}

// LLMStreamingProvider is an optional extension used when a script requests
// stream:true. It emits incremental output while preserving the final TurnResult
// contract of LLMProvider.
type LLMStreamingProvider interface {
	StreamTurn(context.Context, LLMTurnRequest, LLMStreamSink) (LLMTurnResult, error)
}

type LLMStreamEvent struct {
	Type   string
	Token  string
	Text   string
	Status string
	Reason string
	Usage  LLMTurnUsage
}

type LLMStreamSink func(LLMStreamEvent) error

// LLMProviderConfig is the runtime shape of one models {} provider entry.
// The runtime records this data, but construction is delegated to the host
// package to avoid coupling the interpreter to concrete HTTP providers.
type LLMProviderConfig struct {
	Name          string
	Protocol      string
	BaseURL       string
	APIKey        string
	ProviderModel string
	Provider      string
}

type LLMProviderFactory func(LLMProviderConfig) (LLMProvider, error)

type LLMMessage struct {
	Role      string
	Text      string
	ToolCall  *LLMToolCall
	ToolUseID string
	Value     any
	Error     string
}

type LLMTool struct {
	Name        string
	Description string
	Params      []string
	Requires    []string
	Schema      any
}

type LLMToolCall struct {
	ID   string
	Tool string
	Args map[string]any
}

type LLMTurnRequest struct {
	Model          string
	Messages       []LLMMessage
	Tools          []LLMTool
	ForceTool      string
	MaxTokens      int64
	Temperature    *float64
	TopP           *float64
	ResponseFormat any
	Stream         bool
	Stop           []string
	Metadata       map[string]string
}

type LLMTurnUsage struct {
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	LatencyMS    int64
}

type LLMTurnResult struct {
	Status string
	Text   string
	Calls  []LLMToolCall
	Reason string
	Usage  LLMTurnUsage
}

type LLMTraceEvent struct {
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
	Usage        LLMTurnUsage
}

type LLMTraceSink func(LLMTraceEvent)

const (
	LLMProviderErrorNetwork   = "network"
	LLMProviderErrorAuth      = "auth"
	LLMProviderErrorRateLimit = "rate_limit"
	LLMProviderErrorRequest   = "request"
	LLMProviderErrorProvider  = "provider"
)

type llmProviderErrorKind interface {
	LLMProviderErrorKind() string
}

// ClassifyLLMProviderError maps provider failures into a stable diagnostic
// category without inspecting prompts, messages, or token values.
func ClassifyLLMProviderError(err error) string {
	if err == nil {
		return ""
	}
	var typed llmProviderErrorKind
	if errors.As(err, &typed) {
		if kind := typed.LLMProviderErrorKind(); kind != "" {
			return kind
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return LLMProviderErrorNetwork
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return LLMProviderErrorNetwork
	}
	return LLMProviderErrorProvider
}
