package gscript

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"sync"

	"github.com/gscript/gscript/internal/runtime"
)

// LLMProvider is the Go embedding hook behind the llm standard library.
// Implementations can call a remote model API, a local model, or a test double.
type LLMProvider interface {
	Turn(context.Context, LLMTurnRequest) (LLMTurnResult, error)
}

type LLMMessage = runtime.LLMMessage
type LLMTool = runtime.LLMTool
type LLMToolCall = runtime.LLMToolCall
type LLMTurnRequest = runtime.LLMTurnRequest
type LLMTurnResult = runtime.LLMTurnResult
type LLMTurnUsage = runtime.LLMTurnUsage
type LLMTraceEvent = runtime.LLMTraceEvent
type LLMTraceSink func(LLMTraceEvent)
type LLMRecordSink func(LLMRecord)

type LLMRecord struct {
	Request LLMTurnRequest
	Result  LLMTurnResult
	Error   string
}

// WithLLMProvider installs the provider used by llm.turn. A nil provider makes
// llm.turn return a provider error.
func WithLLMProvider(provider LLMProvider) Option {
	return func(o *vmOptions) { o.llmProvider = provider }
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
// local tooling and tests, including wrappers such as glm_cc. The prompt is
// rendered from the request messages and passed as the final command argument.
func WithLLMCommand(command string, args ...string) Option {
	return WithLLMProvider(CommandLLMProvider{Command: command, Args: args})
}

type CommandLLMProvider struct {
	Command string
	Args    []string
	Model   string
}

func (p CommandLLMProvider) Turn(ctx context.Context, req LLMTurnRequest) (LLMTurnResult, error) {
	if p.Command == "" {
		return LLMTurnResult{}, fmt.Errorf("llm command not configured")
	}
	args := append([]string{}, p.Args...)
	if p.Model != "" && req.Model == "" {
		req.Model = p.Model
	}
	if req.Model != "" && !containsModelFlag(args) {
		args = append(args, "--model", req.Model)
	}
	args = append(args, renderLLMPrompt(req))
	out, err := exec.CommandContext(ctx, p.Command, args...).CombinedOutput()
	if err != nil {
		return LLMTurnResult{}, fmt.Errorf("%s: %w: %s", p.Command, err, strings.TrimSpace(string(out)))
	}
	return LLMTurnResult{
		Status: "final_answer",
		Text:   strings.TrimRight(string(out), "\n"),
	}, nil
}

type llmProviderAdapter struct {
	provider LLMProvider
}

func (a llmProviderAdapter) Turn(ctx context.Context, req runtime.LLMTurnRequest) (runtime.LLMTurnResult, error) {
	return a.provider.Turn(ctx, req)
}

func llmTraceAdapter(sink LLMTraceSink) runtime.LLMTraceSink {
	if sink == nil {
		return nil
	}
	return func(event runtime.LLMTraceEvent) {
		sink(event)
	}
}

type recordingLLMProvider struct {
	provider LLMProvider
	sink     LLMRecordSink
}

func configuredLLMProvider(opts vmOptions) LLMProvider {
	if opts.llmProvider == nil || opts.llmRecordSink == nil {
		return opts.llmProvider
	}
	return recordingLLMProvider{provider: opts.llmProvider, sink: opts.llmRecordSink}
}

func (p recordingLLMProvider) Turn(ctx context.Context, req LLMTurnRequest) (LLMTurnResult, error) {
	res, err := p.provider.Turn(ctx, req)
	if p.sink != nil {
		record := LLMRecord{Request: cloneLLMRequest(req), Result: cloneLLMResult(res)}
		if err != nil {
			record.Error = err.Error()
		}
		p.sink(record)
	}
	return res, err
}

// LLMReplayProvider is a deterministic test provider for recorded LLM turns.
type LLMReplayProvider struct {
	mu      sync.Mutex
	records []LLMRecord
	next    int
}

// LLMReplayMismatchError reports a deterministic replay request mismatch.
type LLMReplayMismatchError struct {
	Turn     int
	Expected LLMTurnRequest
	Actual   LLMTurnRequest
}

func (e *LLMReplayMismatchError) Error() string {
	if e == nil {
		return "llm replay request mismatch"
	}
	return fmt.Sprintf("llm replay request mismatch at turn %d", e.Turn)
}

// LLMReplayExhaustedError reports that replay consumed all recorded turns.
type LLMReplayExhaustedError struct {
	Turn int
}

func (e *LLMReplayExhaustedError) Error() string {
	if e == nil {
		return "llm replay exhausted"
	}
	return fmt.Sprintf("llm replay exhausted at turn %d", e.Turn)
}

func NewLLMReplayProvider(records []LLMRecord) *LLMReplayProvider {
	out := make([]LLMRecord, len(records))
	for i := range records {
		out[i] = cloneLLMRecord(records[i])
	}
	return &LLMReplayProvider{records: out}
}

func (p *LLMReplayProvider) Turn(_ context.Context, req LLMTurnRequest) (LLMTurnResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.next >= len(p.records) {
		return LLMTurnResult{}, &LLMReplayExhaustedError{Turn: p.next}
	}
	record := p.records[p.next]
	p.next++
	if !llmRequestsEqual(req, record.Request) {
		return LLMTurnResult{}, &LLMReplayMismatchError{
			Turn:     p.next - 1,
			Expected: cloneLLMRequest(record.Request),
			Actual:   cloneLLMRequest(req),
		}
	}
	if record.Error != "" {
		return LLMTurnResult{}, fmt.Errorf("%s", record.Error)
	}
	return cloneLLMResult(record.Result), nil
}

func (p *LLMReplayProvider) Remaining() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.records) - p.next
}

func (p *LLMReplayProvider) Consumed() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.next
}

func (p *LLMReplayProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = 0
}

func (p *LLMReplayProvider) Records() []LLMRecord {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]LLMRecord, len(p.records))
	for i := range p.records {
		out[i] = cloneLLMRecord(p.records[i])
	}
	return out
}

func containsModelFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--model" || strings.HasPrefix(arg, "--model=") {
			return true
		}
	}
	return false
}

func cloneLLMRecord(record LLMRecord) LLMRecord {
	return LLMRecord{
		Request: cloneLLMRequest(record.Request),
		Result:  cloneLLMResult(record.Result),
		Error:   record.Error,
	}
}

func cloneLLMRequest(req LLMTurnRequest) LLMTurnRequest {
	out := req
	out.Messages = append([]LLMMessage(nil), req.Messages...)
	for i := range out.Messages {
		if out.Messages[i].ToolCall != nil {
			call := cloneLLMToolCall(*out.Messages[i].ToolCall)
			out.Messages[i].ToolCall = &call
		}
		out.Messages[i].Value = cloneLLMAny(out.Messages[i].Value)
	}
	out.Tools = append([]LLMTool(nil), req.Tools...)
	for i := range out.Tools {
		out.Tools[i].Params = append([]string(nil), req.Tools[i].Params...)
		out.Tools[i].Requires = append([]string(nil), req.Tools[i].Requires...)
		out.Tools[i].Schema = cloneLLMAny(req.Tools[i].Schema)
	}
	out.Stop = append([]string(nil), req.Stop...)
	if req.Metadata != nil {
		out.Metadata = make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			out.Metadata[k] = v
		}
	}
	return out
}

func cloneLLMResult(res LLMTurnResult) LLMTurnResult {
	out := res
	out.Calls = make([]LLMToolCall, len(res.Calls))
	for i := range res.Calls {
		out.Calls[i] = cloneLLMToolCall(res.Calls[i])
	}
	return out
}

func cloneLLMToolCall(call LLMToolCall) LLMToolCall {
	out := call
	if call.Args != nil {
		out.Args = make(map[string]any, len(call.Args))
		for k, v := range call.Args {
			out.Args[k] = cloneLLMAny(v)
		}
	}
	return out
}

func cloneLLMAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = cloneLLMAny(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = cloneLLMAny(v)
		}
		return out
	default:
		return v
	}
}

func llmRequestsEqual(a, b LLMTurnRequest) bool {
	return a.Model == b.Model &&
		a.ForceTool == b.ForceTool &&
		a.MaxTokens == b.MaxTokens &&
		a.Stream == b.Stream &&
		reflect.DeepEqual(a.Stop, b.Stop) &&
		reflect.DeepEqual(a.Metadata, b.Metadata) &&
		reflect.DeepEqual(a.Messages, b.Messages) &&
		reflect.DeepEqual(a.Tools, b.Tools)
}

func renderLLMPrompt(req LLMTurnRequest) string {
	var b strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			b.WriteString("System: ")
		case "assistant":
			b.WriteString("Assistant: ")
		case "tool":
			b.WriteString("Tool: ")
		default:
			b.WriteString("User: ")
		}
		if msg.Text != "" {
			b.WriteString(msg.Text)
		} else if msg.Error != "" {
			b.WriteString(msg.Error)
		} else if msg.Value != nil {
			b.WriteString(fmt.Sprint(msg.Value))
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
