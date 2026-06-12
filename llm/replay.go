package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sync"
)

// Recorder is a thread-safe record sink for deterministic LLM replay fixtures.
// Pass recorder.Record to leia.WithLLMRecorder.
type Recorder struct {
	mu      sync.Mutex
	records []Record
}

func NewRecorder(records ...Record) *Recorder {
	rec := &Recorder{}
	for _, record := range records {
		rec.records = append(rec.records, cloneRecord(record))
	}
	return rec
}

func (r *Recorder) Record(record Record) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, cloneRecord(record))
}

func (r *Recorder) Records() []Record {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Record, len(r.records))
	for i := range r.records {
		out[i] = cloneRecord(r.records[i])
	}
	return out
}

func (r *Recorder) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = nil
}

func (r *Recorder) Save(path string) error {
	return SaveRecords(path, r.Records())
}

func LoadRecorder(path string) (*Recorder, error) {
	records, err := LoadRecords(path)
	if err != nil {
		return nil, err
	}
	return NewRecorder(records...), nil
}

// ReplayProvider is a deterministic test provider for recorded LLM turns.
type ReplayProvider struct {
	mu           sync.Mutex
	records      []Record
	next         int
	lastMatch    ReplayMatch
	hasLastMatch bool
}

// ReplayMismatchError reports a deterministic replay request mismatch.
type ReplayMismatchError struct {
	Turn     int
	Expected TurnRequest
	Actual   TurnRequest
}

func (e *ReplayMismatchError) Error() string {
	if e == nil {
		return "llm replay request mismatch"
	}
	return fmt.Sprintf("llm replay request mismatch at turn %d", e.Turn)
}

// ReplayExhaustedError reports that replay consumed all recorded turns.
type ReplayExhaustedError struct {
	Turn int
}

func (e *ReplayExhaustedError) Error() string {
	if e == nil {
		return "llm replay exhausted"
	}
	return fmt.Sprintf("llm replay exhausted at turn %d", e.Turn)
}

func NewReplayProvider(records []Record) *ReplayProvider {
	out := make([]Record, len(records))
	for i := range records {
		out[i] = cloneRecord(records[i])
	}
	return &ReplayProvider{records: out}
}

// MarshalRecords serializes replay records as stable JSON for deterministic
// tests and offline evaluation fixtures.
func MarshalRecords(records []Record) ([]byte, error) {
	out := make([]Record, len(records))
	for i := range records {
		out[i] = cloneRecord(records[i])
	}
	return json.MarshalIndent(out, "", "  ")
}

// UnmarshalRecords parses replay fixture JSON. Integer-valued JSON numbers
// are normalized back to int64 so strict replay matching remains compatible
// with values produced by the Leia runtime.
func UnmarshalRecords(data []byte) ([]Record, error) {
	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	out := make([]Record, len(records))
	for i := range records {
		out[i] = normalizeRecordJSON(records[i])
	}
	return out, nil
}

func SaveRecords(path string, records []Record) error {
	data, err := MarshalRecords(records)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadRecords(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return UnmarshalRecords(data)
}

func (p *ReplayProvider) Turn(_ context.Context, req TurnRequest) (TurnResult, error) {
	record, err := p.nextRecord(req)
	if err != nil {
		return TurnResult{}, err
	}
	if record.Error != "" {
		return TurnResult{}, fmt.Errorf("%s", record.Error)
	}
	return cloneResult(record.Result), nil
}

// StreamTurn replays recorded stream events before returning the recorded final
// result. Older fixtures without StreamEvents synthesize one token event from
// the final text so stream callbacks and trace accounting still run offline.
func (p *ReplayProvider) StreamTurn(_ context.Context, req TurnRequest, sink StreamSink) (TurnResult, error) {
	record, err := p.nextRecord(req)
	if err != nil {
		return TurnResult{}, err
	}
	if record.Error != "" {
		return TurnResult{}, fmt.Errorf("%s", record.Error)
	}
	events := cloneStreamEvents(record.StreamEvents)
	if len(events) == 0 && req.Stream && record.Result.Text != "" {
		events = []StreamEvent{{Type: "token", Token: record.Result.Text, Text: record.Result.Text}}
	}
	for _, event := range events {
		if sink != nil {
			if err := sink(event); err != nil {
				return TurnResult{}, err
			}
		}
	}
	return cloneResult(record.Result), nil
}

func (p *ReplayProvider) nextRecord(req TurnRequest) (Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.next >= len(p.records) {
		p.lastMatch = ReplayMatch{}
		p.hasLastMatch = false
		return Record{}, &ReplayExhaustedError{Turn: p.next}
	}
	record := p.records[p.next]
	turn := p.next
	p.next++
	if !requestsEqual(req, record.Request) {
		p.lastMatch = ReplayMatch{}
		p.hasLastMatch = false
		return Record{}, &ReplayMismatchError{
			Turn:     turn,
			Expected: cloneRequest(record.Request),
			Actual:   cloneRequest(req),
		}
	}
	p.lastMatch = replayMatchForRecord(record, turn)
	p.hasLastMatch = true
	return cloneRecord(record), nil
}

func (p *ReplayProvider) Remaining() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.records) - p.next
}

func (p *ReplayProvider) Consumed() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.next
}

func (p *ReplayProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = 0
	p.lastMatch = ReplayMatch{}
	p.hasLastMatch = false
}

func (p *ReplayProvider) Records() []Record {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Record, len(p.records))
	for i := range p.records {
		out[i] = cloneRecord(p.records[i])
	}
	return out
}

func (p *ReplayProvider) LastReplayMatch() (ReplayMatch, bool) {
	if p == nil {
		return ReplayMatch{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastMatch, p.hasLastMatch
}

func replayMatchForRecord(record Record, turn int) ReplayMatch {
	replayKey := record.Request.Metadata["replay_key"]
	if replayKey == "" {
		replayKey = fmt.Sprintf("turn:%d", turn)
	}
	requestHash := firstRecordMetadata(record, "request_hash")
	if requestHash == "" {
		requestHash = stableRecordHash(record.Request)
	}
	responseHash := firstRecordMetadata(record, "response_hash")
	if responseHash == "" {
		responseHash = stableRecordHash(record.Result)
	}
	replayMode := firstRecordMetadata(record, "replay_mode")
	if replayMode == "" {
		replayMode = "fixture_replay"
	}
	return ReplayMatch{
		Turn:         turn,
		ReplayKey:    replayKey,
		RequestHash:  requestHash,
		ResponseHash: responseHash,
		ReplayMode:   replayMode,
		ProviderFree: true,
	}
}

func firstRecordMetadata(record Record, key string) string {
	if record.Request.Metadata == nil {
		return ""
	}
	return record.Request.Metadata[key]
}

func stableRecordHash(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneRecord(record Record) Record {
	return Record{
		Request:      cloneRequest(record.Request),
		Result:       cloneResult(record.Result),
		StreamEvents: cloneStreamEvents(record.StreamEvents),
		Error:        record.Error,
	}
}

func cloneStreamEvents(events []StreamEvent) []StreamEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]StreamEvent, len(events))
	copy(out, events)
	return out
}

func cloneRequest(req TurnRequest) TurnRequest {
	out := req
	out.Messages = append([]Message(nil), req.Messages...)
	for i := range out.Messages {
		if out.Messages[i].ToolCall != nil {
			call := cloneToolCall(*out.Messages[i].ToolCall)
			out.Messages[i].ToolCall = &call
		}
		out.Messages[i].Value = cloneAny(out.Messages[i].Value)
	}
	out.Tools = append([]Tool(nil), req.Tools...)
	for i := range out.Tools {
		out.Tools[i].Params = append([]string(nil), req.Tools[i].Params...)
		out.Tools[i].Requires = append([]string(nil), req.Tools[i].Requires...)
		out.Tools[i].Schema = cloneAny(req.Tools[i].Schema)
	}
	if req.Temperature != nil {
		v := *req.Temperature
		out.Temperature = &v
	}
	if req.TopP != nil {
		v := *req.TopP
		out.TopP = &v
	}
	out.ResponseFormat = cloneAny(req.ResponseFormat)
	out.Stop = append([]string(nil), req.Stop...)
	if req.Metadata != nil {
		out.Metadata = make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			out.Metadata[k] = v
		}
	}
	return out
}

func cloneResult(res TurnResult) TurnResult {
	out := res
	out.Calls = make([]ToolCall, len(res.Calls))
	for i := range res.Calls {
		out.Calls[i] = cloneToolCall(res.Calls[i])
	}
	return out
}

func cloneToolCall(call ToolCall) ToolCall {
	out := call
	if call.Args != nil {
		out.Args = make(map[string]any, len(call.Args))
		for k, v := range call.Args {
			out.Args[k] = cloneAny(v)
		}
	}
	return out
}

func cloneAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = cloneAny(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = cloneAny(v)
		}
		return out
	default:
		return v
	}
}

func normalizeRecordJSON(record Record) Record {
	return Record{
		Request:      normalizeRequestJSON(record.Request),
		Result:       normalizeResultJSON(record.Result),
		StreamEvents: cloneStreamEvents(record.StreamEvents),
		Error:        record.Error,
	}
}

func normalizeRequestJSON(req TurnRequest) TurnRequest {
	out := cloneRequest(req)
	for i := range out.Messages {
		out.Messages[i].Value = normalizeAnyJSON(out.Messages[i].Value)
		if out.Messages[i].ToolCall != nil {
			call := normalizeToolCallJSON(*out.Messages[i].ToolCall)
			out.Messages[i].ToolCall = &call
		}
	}
	for i := range out.Tools {
		out.Tools[i].Schema = normalizeAnyJSON(out.Tools[i].Schema)
	}
	out.ResponseFormat = normalizeAnyJSON(out.ResponseFormat)
	return out
}

func normalizeResultJSON(res TurnResult) TurnResult {
	out := cloneResult(res)
	for i := range res.Calls {
		out.Calls[i] = normalizeToolCallJSON(res.Calls[i])
	}
	return out
}

func normalizeToolCallJSON(call ToolCall) ToolCall {
	out := cloneToolCall(call)
	for k, v := range call.Args {
		out.Args[k] = normalizeAnyJSON(v)
	}
	return out
}

func normalizeAnyJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = normalizeAnyJSON(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = normalizeAnyJSON(v)
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

func requestsEqual(a, b TurnRequest) bool {
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
