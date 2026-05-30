package gscript

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
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
type LLMProviderConfig = runtime.LLMProviderConfig
type LLMProviderFactory func(LLMProviderConfig) (LLMProvider, error)

type LLMRecord struct {
	Request LLMTurnRequest
	Result  LLMTurnResult
	Error   string
}

// LLMTraceRecorder is a thread-safe trace sink for tests, diagnostics, and
// host-side observability. Pass recorder.Record to WithLLMTrace.
type LLMTraceRecorder struct {
	mu     sync.Mutex
	events []LLMTraceEvent
}

func NewLLMTraceRecorder(events ...LLMTraceEvent) *LLMTraceRecorder {
	rec := &LLMTraceRecorder{}
	rec.events = append(rec.events, events...)
	return rec
}

func (r *LLMTraceRecorder) Record(event LLMTraceEvent) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *LLMTraceRecorder) Events() []LLMTraceEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]LLMTraceEvent, len(r.events))
	copy(out, r.events)
	return out
}

func (r *LLMTraceRecorder) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}

// LLMRecorder is a thread-safe record sink for deterministic LLM replay
// fixtures. Pass recorder.Record to WithLLMRecorder.
type LLMRecorder struct {
	mu      sync.Mutex
	records []LLMRecord
}

func NewLLMRecorder(records ...LLMRecord) *LLMRecorder {
	rec := &LLMRecorder{}
	for _, record := range records {
		rec.records = append(rec.records, cloneLLMRecord(record))
	}
	return rec
}

func (r *LLMRecorder) Record(record LLMRecord) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, cloneLLMRecord(record))
}

func (r *LLMRecorder) Records() []LLMRecord {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]LLMRecord, len(r.records))
	for i := range r.records {
		out[i] = cloneLLMRecord(r.records[i])
	}
	return out
}

func (r *LLMRecorder) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = nil
}

func (r *LLMRecorder) Save(path string) error {
	return SaveLLMRecords(path, r.Records())
}

func LoadLLMRecorder(path string) (*LLMRecorder, error) {
	records, err := LoadLLMRecords(path)
	if err != nil {
		return nil, err
	}
	return NewLLMRecorder(records...), nil
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

func configuredLLMProviderFactory(opts vmOptions) runtime.LLMProviderFactory {
	var factory LLMProviderFactory
	if opts.llmProviderFactory != nil {
		factory = opts.llmProviderFactory
	} else if opts.llmProvider == nil {
		factory = defaultLLMProviderFactory
	}
	if factory == nil {
		return nil
	}
	return func(cfg runtime.LLMProviderConfig) (runtime.LLMProvider, error) {
		p, err := factory(cfg)
		if err != nil || p == nil || opts.llmRecordSink == nil {
			return p, err
		}
		return recordingLLMProvider{provider: p, sink: opts.llmRecordSink}, nil
	}
}

func defaultLLMProviderFactory(cfg LLMProviderConfig) (LLMProvider, error) {
	protocol := strings.ToLower(strings.ReplaceAll(cfg.Protocol, "_", "-"))
	switch protocol {
	case "openai", "openai-compatible", "openai-compat", "chat-completions":
		return OpenAICompatibleLLMProvider{
			Endpoint: openAIChatCompletionsEndpoint(cfg.BaseURL),
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
		}, nil
	case "anthropic", "anthropic-compatible", "anthropic-compat", "messages":
		return AnthropicCompatibleLLMProvider{
			Endpoint: cfg.BaseURL,
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
		}, nil
	default:
		if cfg.Protocol == "" {
			return nil, fmt.Errorf("llm provider protocol not configured for model %q", cfg.Name)
		}
		return nil, fmt.Errorf("unsupported llm provider protocol %q for model %q", cfg.Protocol, cfg.Name)
	}
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

// MarshalLLMRecords serializes replay records as stable JSON for deterministic
// tests and offline evaluation fixtures.
func MarshalLLMRecords(records []LLMRecord) ([]byte, error) {
	out := make([]LLMRecord, len(records))
	for i := range records {
		out[i] = cloneLLMRecord(records[i])
	}
	return json.MarshalIndent(out, "", "  ")
}

// UnmarshalLLMRecords parses replay fixture JSON. Integer-valued JSON numbers
// are normalized back to int64 so strict replay matching remains compatible
// with values produced by the GScript runtime.
func UnmarshalLLMRecords(data []byte) ([]LLMRecord, error) {
	var records []LLMRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	out := make([]LLMRecord, len(records))
	for i := range records {
		out[i] = normalizeLLMRecordJSON(records[i])
	}
	return out, nil
}

func SaveLLMRecords(path string, records []LLMRecord) error {
	data, err := MarshalLLMRecords(records)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadLLMRecords(path string) ([]LLMRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return UnmarshalLLMRecords(data)
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
	if req.Temperature != nil {
		v := *req.Temperature
		out.Temperature = &v
	}
	if req.TopP != nil {
		v := *req.TopP
		out.TopP = &v
	}
	out.ResponseFormat = cloneLLMAny(req.ResponseFormat)
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

func normalizeLLMRecordJSON(record LLMRecord) LLMRecord {
	return LLMRecord{
		Request: normalizeLLMRequestJSON(record.Request),
		Result:  normalizeLLMResultJSON(record.Result),
		Error:   record.Error,
	}
}

func normalizeLLMRequestJSON(req LLMTurnRequest) LLMTurnRequest {
	out := cloneLLMRequest(req)
	for i := range out.Messages {
		out.Messages[i].Value = normalizeLLMAnyJSON(out.Messages[i].Value)
		if out.Messages[i].ToolCall != nil {
			call := normalizeLLMToolCallJSON(*out.Messages[i].ToolCall)
			out.Messages[i].ToolCall = &call
		}
	}
	for i := range out.Tools {
		out.Tools[i].Schema = normalizeLLMAnyJSON(out.Tools[i].Schema)
	}
	out.ResponseFormat = normalizeLLMAnyJSON(out.ResponseFormat)
	return out
}

func normalizeLLMResultJSON(res LLMTurnResult) LLMTurnResult {
	out := cloneLLMResult(res)
	for i := range out.Calls {
		out.Calls[i] = normalizeLLMToolCallJSON(out.Calls[i])
	}
	return out
}

func normalizeLLMToolCallJSON(call LLMToolCall) LLMToolCall {
	out := cloneLLMToolCall(call)
	for k, v := range out.Args {
		out.Args[k] = normalizeLLMAnyJSON(v)
	}
	return out
}

func normalizeLLMAnyJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = normalizeLLMAnyJSON(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = normalizeLLMAnyJSON(v)
		}
		return out
	case float64:
		if math.Trunc(x) == x && x >= float64(math.MinInt64) && x <= float64(math.MaxInt64) {
			return int64(x)
		}
		return x
	default:
		return x
	}
}

func llmRequestsEqual(a, b LLMTurnRequest) bool {
	return a.Model == b.Model &&
		a.ForceTool == b.ForceTool &&
		a.MaxTokens == b.MaxTokens &&
		reflect.DeepEqual(a.Temperature, b.Temperature) &&
		reflect.DeepEqual(a.TopP, b.TopP) &&
		reflect.DeepEqual(a.ResponseFormat, b.ResponseFormat) &&
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
