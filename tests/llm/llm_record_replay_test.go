package leia_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMRecorderAndReplay(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{
		Status: "final_answer",
		Text:   "recorded",
		Usage:  llm.TurnUsage{InputTokens: 5, OutputTokens: 6},
	}}
	var records []llm.Record
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(provider),
		leia.WithLLMRecorder(func(record llm.Record) {
			records = append(records, record)
		}),
	)
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.system("short"), llm.user("hello")},
    max_tokens: 16,
})
text := result.text
`); err != nil {
		t.Fatalf("record Exec: %v", err)
	}
	if len(records) != 1 || records[0].Request.Model != "mock-fast" || records[0].Result.Text != "recorded" {
		t.Fatalf("records = %#v", records)
	}
	replay := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMReplay(records),
	)
	if err := replay.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.system("short"), llm.user("hello")},
    max_tokens: 16,
})
text := result.text
usage := result.usage.output_tokens
`); err != nil {
		t.Fatalf("replay Exec: %v", err)
	}
	text, _ := replay.Get("text")
	usage, _ := replay.Get("usage")
	if text != "recorded" || usage != int64(6) {
		t.Fatalf("replay text=%#v usage=%#v", text, usage)
	}
}

func TestLLMRecorderHelper(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{
		Status: "final_answer",
		Text:   "recorded",
	}}
	recorder := llm.NewRecorder()
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(provider),
		leia.WithLLMRecorder(recorder.Record),
	)
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("hello")},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	records := recorder.Records()
	if len(records) != 1 || records[0].Result.Text != "recorded" {
		t.Fatalf("records = %#v", records)
	}
	records[0].Result.Text = "mutated"
	if recorder.Records()[0].Result.Text != "recorded" {
		t.Fatalf("Records returned mutable internal state")
	}
	path := filepath.Join(t.TempDir(), "records.json")
	if err := recorder.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := llm.LoadRecorder(path)
	if err != nil {
		t.Fatalf("LoadLLMRecorder: %v", err)
	}
	replay := llm.NewReplayProvider(loaded.Records())
	res, err := replay.Turn(context.Background(), recorder.Records()[0].Request)
	if err != nil || res.Text != "recorded" {
		t.Fatalf("Turn res=%#v err=%v", res, err)
	}
	recorder.Reset()
	if got := recorder.Records(); len(got) != 0 {
		t.Fatalf("after Reset records = %#v", got)
	}
}

func TestLLMReplayRejectsMismatchedRequest(t *testing.T) {
	records := []llm.Record{{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "expected"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "ok"},
	}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMReplay(records))
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("actual")},
})
kind := err.kind
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("kind")
	if kind != "provider" {
		t.Fatalf("kind = %#v", kind)
	}
}

func TestLLMReplayTypedErrors(t *testing.T) {
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
		t.Fatalf("err = %T %v, want LLMReplayMismatchError", err, err)
	}
	if mismatch.Turn != 0 || mismatch.Expected.Messages[0].Text != "expected" || mismatch.Actual.Messages[0].Text != "actual" {
		t.Fatalf("mismatch = %#v", mismatch)
	}
	mismatch.Expected.Messages[0].Text = "mutated"
	if replay.Remaining() != 0 {
		t.Fatalf("remaining = %d, want 0", replay.Remaining())
	}

	empty := llm.NewReplayProvider(nil)
	_, err = empty.Turn(context.Background(), llm.TurnRequest{})
	var exhausted *llm.ReplayExhaustedError
	if !errors.As(err, &exhausted) || exhausted.Turn != 0 {
		t.Fatalf("err = %T %v, exhausted=%#v", err, err, exhausted)
	}
}

func TestLLMReplayProviderStateHelpers(t *testing.T) {
	record := llm.Record{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "hello"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "ok"},
	}
	replay := llm.NewReplayProvider([]llm.Record{record})
	if replay.Consumed() != 0 || replay.Remaining() != 1 {
		t.Fatalf("initial consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
	records := replay.Records()
	records[0].Request.Messages[0].Text = "mutated"
	if replay.Records()[0].Request.Messages[0].Text != "hello" {
		t.Fatalf("Records returned mutable internal state")
	}
	res, err := replay.Turn(context.Background(), record.Request)
	if err != nil || res.Text != "ok" {
		t.Fatalf("Turn res=%#v err=%v", res, err)
	}
	if replay.Consumed() != 1 || replay.Remaining() != 0 {
		t.Fatalf("after turn consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
	replay.Reset()
	if replay.Consumed() != 0 || replay.Remaining() != 1 {
		t.Fatalf("after reset consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
	res, err = replay.Turn(context.Background(), record.Request)
	if err != nil || res.Text != "ok" {
		t.Fatalf("Turn after reset res=%#v err=%v", res, err)
	}
}

func TestLLMRecordJSONRoundTrip(t *testing.T) {
	records := []llm.Record{{
		Request: llm.TurnRequest{
			Model: "mock-fast",
			Messages: []llm.Message{{
				Role: "user",
				Text: "hello",
				Value: map[string]any{
					"count": int64(3),
					"tags":  []any{"a", int64(2)},
				},
			}},
			Tools: []llm.Tool{{
				Name:     "lookup",
				Params:   []string{"name"},
				Requires: []string{"docs.read"},
				Schema: map[string]any{
					"type":  "object",
					"limit": int64(3),
				},
			}},
			Metadata: map[string]string{"trace_id": "abc"},
		},
		Result: llm.TurnResult{
			Status: "tool_calls",
			Calls: []llm.ToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "leia", "limit": int64(3)},
			}},
		},
	}}
	data, err := llm.MarshalRecords(records)
	if err != nil {
		t.Fatalf("MarshalLLMRecords: %v", err)
	}
	decoded, err := llm.UnmarshalRecords(data)
	if err != nil {
		t.Fatalf("UnmarshalLLMRecords: %v", err)
	}
	replay := llm.NewReplayProvider(decoded)
	res, err := replay.Turn(context.Background(), records[0].Request)
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if got := res.Calls[0].Args["limit"]; got != int64(3) {
		t.Fatalf("limit = %#v (%T), want int64(3)", got, got)
	}
}

func TestLLMRecordFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	records := []llm.Record{{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "hello"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "ok"},
	}}
	if err := llm.SaveRecords(path, records); err != nil {
		t.Fatalf("SaveLLMRecords: %v", err)
	}
	decoded, err := llm.LoadRecords(path)
	if err != nil {
		t.Fatalf("LoadLLMRecords: %v", err)
	}
	replay := llm.NewReplayProvider(decoded)
	res, err := replay.Turn(context.Background(), records[0].Request)
	if err != nil || res.Text != "ok" {
		t.Fatalf("Turn res=%#v err=%v", res, err)
	}
}
