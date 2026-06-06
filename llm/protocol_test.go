package llm_test

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/never-labs/leia/llm"
)

type classifiedError string

func (e classifiedError) Error() string {
	return string(e)
}

func (e classifiedError) LLMProviderErrorKind() string {
	return llm.ProviderErrorRateLimit
}

func TestRecorderAndReplayProvider(t *testing.T) {
	record := llm.Record{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "hello"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "ok"},
	}
	recorder := llm.NewRecorder(record)
	records := recorder.Records()
	records[0].Result.Text = "mutated"
	if got := recorder.Records()[0].Result.Text; got != "ok" {
		t.Fatalf("records mutated internal state: %q", got)
	}

	replay := llm.NewReplayProvider(recorder.Records())
	if replay.Consumed() != 0 || replay.Remaining() != 1 {
		t.Fatalf("initial consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
	res, err := replay.Turn(context.Background(), record.Request)
	if err != nil || res.Text != "ok" {
		t.Fatalf("Turn res=%#v err=%v", res, err)
	}
	if replay.Consumed() != 1 || replay.Remaining() != 0 {
		t.Fatalf("after turn consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
}

func TestReplayProviderStreamsRecordedEvents(t *testing.T) {
	record := llm.Record{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "hello"}},
			Stream:   true,
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "hello stream"},
		StreamEvents: []llm.StreamEvent{
			{Type: "token", Token: "hello", Text: "hello"},
			{Type: "token", Token: " ", Text: " "},
			{Type: "token", Token: "stream", Text: "stream"},
		},
	}
	replay := llm.NewReplayProvider([]llm.Record{record})
	var tokens []string
	res, err := replay.StreamTurn(context.Background(), record.Request, func(event llm.StreamEvent) error {
		tokens = append(tokens, event.Token)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamTurn: %v", err)
	}
	if res.Text != "hello stream" || len(tokens) != 3 || tokens[0]+tokens[1]+tokens[2] != "hello stream" {
		t.Fatalf("res=%#v tokens=%#v", res, tokens)
	}
}

func TestReplayProviderSynthesizesLegacyStreamEvent(t *testing.T) {
	record := llm.Record{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "hello"}},
			Stream:   true,
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "legacy stream"},
	}
	replay := llm.NewReplayProvider([]llm.Record{record})
	var tokens []string
	res, err := replay.StreamTurn(context.Background(), record.Request, func(event llm.StreamEvent) error {
		tokens = append(tokens, event.Token)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamTurn: %v", err)
	}
	if res.Text != "legacy stream" || len(tokens) != 1 || tokens[0] != "legacy stream" {
		t.Fatalf("res=%#v tokens=%#v", res, tokens)
	}
}

func TestReplayTypedErrors(t *testing.T) {
	replay := llm.NewReplayProvider([]llm.Record{{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "expected"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "ok"},
	}})
	_, err := replay.Turn(context.Background(), llm.TurnRequest{
		Model:    "mock-fast",
		Messages: []llm.Message{{Role: "user", Text: "actual"}},
	})
	var mismatch *llm.ReplayMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %T %v, want ReplayMismatchError", err, err)
	}
	if mismatch.Expected.Messages[0].Text != "expected" || mismatch.Actual.Messages[0].Text != "actual" {
		t.Fatalf("mismatch = %#v", mismatch)
	}

	_, err = llm.NewReplayProvider(nil).Turn(context.Background(), llm.TurnRequest{})
	var exhausted *llm.ReplayExhaustedError
	if !errors.As(err, &exhausted) || exhausted.Turn != 0 {
		t.Fatalf("err = %T %v, exhausted=%#v", err, err, exhausted)
	}
}

func TestRecordJSONAndFileRoundTrip(t *testing.T) {
	records := []llm.Record{{
		Request: llm.TurnRequest{
			Model: "mock-fast",
			Messages: []llm.Message{{
				Role:  "user",
				Text:  "hello",
				Value: map[string]any{"count": int64(3)},
			}},
			Tools: []llm.Tool{{
				Name:   "lookup",
				Schema: map[string]any{"limit": int64(3)},
			}},
		},
		Result: llm.TurnResult{
			Status: "final_answer",
			Text:   "ok",
			Calls: []llm.ToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"limit": int64(3)},
			}},
		},
	}}

	data, err := llm.MarshalRecords(records)
	if err != nil {
		t.Fatalf("MarshalRecords: %v", err)
	}
	decoded, err := llm.UnmarshalRecords(data)
	if err != nil {
		t.Fatalf("UnmarshalRecords: %v", err)
	}
	if got := decoded[0].Request.Messages[0].Value.(map[string]any)["count"]; got != int64(3) {
		t.Fatalf("decoded count = %#v (%T), want int64(3)", got, got)
	}

	path := filepath.Join(t.TempDir(), "records.json")
	if err := llm.SaveRecords(path, decoded); err != nil {
		t.Fatalf("SaveRecords: %v", err)
	}
	loaded, err := llm.LoadRecords(path)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if _, err := llm.NewReplayProvider(loaded).Turn(context.Background(), records[0].Request); err != nil {
		t.Fatalf("replay loaded records: %v", err)
	}
}

func TestTraceRecorder(t *testing.T) {
	recorder := llm.NewTraceRecorder(llm.TraceEvent{Type: "seed"})
	recorder.Record(llm.TraceEvent{Type: "turn", Model: "mock-fast"})
	events := recorder.Events()
	events[0].Type = "mutated"
	if got := recorder.Events()[0].Type; got != "seed" {
		t.Fatalf("events mutated internal state: %q", got)
	}
	recorder.Reset()
	if got := recorder.Events(); len(got) != 0 {
		t.Fatalf("after reset events = %#v", got)
	}
}

func TestClassifyProviderError(t *testing.T) {
	if got := llm.ClassifyProviderError(classifiedError("rate limited")); got != llm.ProviderErrorRateLimit {
		t.Fatalf("typed error kind = %q", got)
	}
	if got := llm.ClassifyProviderError(context.DeadlineExceeded); got != llm.ProviderErrorNetwork {
		t.Fatalf("deadline kind = %q", got)
	}
	if got := llm.ClassifyProviderError(&net.DNSError{IsTimeout: true}); got != llm.ProviderErrorNetwork {
		t.Fatalf("net error kind = %q", got)
	}
	if got := llm.ClassifyProviderError(errors.New("provider failed")); got != llm.ProviderErrorProvider {
		t.Fatalf("default kind = %q", got)
	}
	if got := llm.ClassifyProviderError(context.Canceled); got != llm.ProviderErrorNetwork {
		t.Fatalf("canceled kind = %q", got)
	}
	if got := llm.ClassifyProviderError(timeoutError{}); got != llm.ProviderErrorNetwork {
		t.Fatalf("timeout kind = %q", got)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string {
	return "timeout"
}

func (timeoutError) Timeout() bool {
	return true
}

func (timeoutError) Temporary() bool {
	return false
}

func (timeoutError) Deadline() (time.Time, bool) {
	return time.Time{}, false
}
