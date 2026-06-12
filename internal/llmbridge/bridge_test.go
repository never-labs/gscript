package llmbridge

import (
	"context"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/llm"
)

func TestPublicTraceEventCarriesReplayEnvelopeFields(t *testing.T) {
	event := PublicTraceEvent(runtime.LLMTraceEvent{
		TraceID:         "trace-1",
		EventID:         "event-2",
		ParentEventID:   "event-1",
		TurnID:          "turn-3",
		ReplaySessionID: "session-4",
		Sequence:        5,
		TimestampMS:     12345,
		Type:            "replay_record_matched",
		Model:           "mock",
		Status:          "matched",
		ReplayKey:       "turn:0",
		RequestHash:     "sha256:req",
		ResponseHash:    "sha256:res",
		ReplayMode:      "fixture_replay",
		ProviderFree:    true,
	})
	if event.TraceID != "trace-1" ||
		event.EventID != "event-2" ||
		event.ParentEventID != "event-1" ||
		event.TurnID != "turn-3" ||
		event.ReplaySessionID != "session-4" ||
		event.Sequence != 5 ||
		event.TimestampMS != 12345 ||
		event.Type != "replay_record_matched" ||
		event.ReplayKey != "turn:0" ||
		event.RequestHash != "sha256:req" ||
		event.ResponseHash != "sha256:res" ||
		event.ReplayMode != "fixture_replay" ||
		!event.ProviderFree {
		t.Fatalf("trace event mapping lost replay envelope fields: %#v", event)
	}
}

type replayMatchProvider struct {
	match llm.ReplayMatch
}

func (p replayMatchProvider) Turn(context.Context, llm.TurnRequest) (llm.TurnResult, error) {
	return llm.TurnResult{Status: "final_answer"}, nil
}

func (p replayMatchProvider) LastReplayMatch() (llm.ReplayMatch, bool) {
	return p.match, true
}

func TestProviderAdapterCarriesReplayMatchProvider(t *testing.T) {
	provider := ProviderAdapter(replayMatchProvider{match: llm.ReplayMatch{
		Turn:            3,
		ReplayKey:       "turn:3",
		RequestHash:     "sha256:req",
		ResponseHash:    "sha256:res",
		ReplayMode:      "fixture_replay",
		ReplaySessionID: "session-1",
		ProviderFree:    true,
	}})
	replay, ok := provider.(runtime.LLMReplayMatchProvider)
	if !ok {
		t.Fatal("ProviderAdapter did not expose runtime replay match provider")
	}
	match, ok := replay.LastLLMReplayMatch()
	if !ok || match.Turn != 3 || match.ReplayKey != "turn:3" || match.ReplaySessionID != "session-1" || !match.ProviderFree {
		t.Fatalf("LastLLMReplayMatch = %#v ok=%v", match, ok)
	}
}
