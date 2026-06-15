package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/never-labs/leia/llm"
)

func TestEvaluateLLMRecordModeWritesGlobalAndCaseFixtures(t *testing.T) {
	type anthropicRequest struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	var (
		mu       sync.Mutex
		requests []anthropicRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, `data: {"type":"message_start","message":{"usage":{"input_tokens":7}}}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"OK"}}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"-"}}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"1"}}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":7,"output_tokens":3}}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"type":"text","text":"OK-2"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`)
	}))
	defer server.Close()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "evaluate_llm_record.leia")
	recordPath := filepath.Join(dir, "turns.records.json")
	source := `
llm.register_models({
    default: "local"
    local: {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_LLM_BASE_URL")
        api_key: os.getenv("LEIA_LLM_API_KEY")
        provider_model: os.getenv("LEIA_LLM_MODEL")
    }
})

evaluate "streamed record case" {
    result, err := llm.turn({
        messages: [llm.user("stream one")]
        stream: true
        max_tokens: 16
    })
    assert(err == nil)
    assert(result.text == "OK-1")
    assert(eval.usage().stream_events == 3)
}

evaluate "plain record case" {
    result, err := llm.turn({
        messages: [llm.user("plain two")]
        max_tokens: 16
    })
    assert(err == nil)
    assert(result.text == "OK-2")
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEIA_LLM_BASE_URL", server.URL)
	t.Setenv("LEIA_LLM_API_KEY", "test-key")
	t.Setenv("LEIA_LLM_MODEL", "mock-model")

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", "--parallel=4", "--llm-record", recordPath, sourcePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --llm-record code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report struct {
		Status string `json:"status"`
		LLM    *struct {
			Mode          string `json:"mode"`
			RecordPath    string `json:"record_path"`
			RecordedTurns int    `json:"recorded_turns"`
			Turns         int    `json:"turns"`
			StreamEvents  int    `json:"stream_events"`
			InputTokens   int64  `json:"input_tokens"`
			OutputTokens  int64  `json:"output_tokens"`
		} `json:"llm"`
		Cases []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			LLM    *struct {
				RecordPath   string `json:"record_path"`
				Turns        int    `json:"turns"`
				StreamEvents int    `json:"stream_events"`
				Events       []struct {
					Type     string `json:"type"`
					TraceID  string `json:"trace_id"`
					EventID  string `json:"event_id"`
					Sequence int64  `json:"sequence"`
				} `json:"events"`
			} `json:"llm"`
		} `json:"cases"`
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if report.Status != "ok" || report.LLM == nil || report.LLM.Mode != "record" || report.LLM.RecordPath != recordPath || report.LLM.RecordedTurns != 2 || report.LLM.Turns != 2 {
		t.Fatalf("llm report = %+v, want record mode with two turns", report.LLM)
	}
	if report.LLM.InputTokens != 12 || report.LLM.OutputTokens != 5 {
		t.Fatalf("llm tokens = %d/%d, want 12/5", report.LLM.InputTokens, report.LLM.OutputTokens)
	}
	if report.LLM.StreamEvents != 3 {
		t.Fatalf("llm stream_events = %d, want 3", report.LLM.StreamEvents)
	}
	if !containsEvaluateLLMString(report.Notes, "parallel evaluate execution disabled for deterministic LLM fixture mode") {
		t.Fatalf("notes = %#v, want deterministic fixture parallel guard note", report.Notes)
	}
	if len(report.Cases) != 2 || report.Cases[0].Status != "passed" || report.Cases[1].Status != "passed" {
		t.Fatalf("cases = %+v, want two passed cases", report.Cases)
	}
	if report.Cases[0].LLM == nil || report.Cases[0].LLM.StreamEvents != 3 || report.Cases[0].LLM.Turns != 1 {
		t.Fatalf("streamed case llm = %+v, want one turn and three stream events", report.Cases[0].LLM)
	}
	if !evaluateLLMEventsContain(report.Cases[0].LLM.Events, "turn_start", "turn_stream", "turn_end") {
		t.Fatalf("streamed case events = %+v, want turn_start/turn_stream/turn_end", report.Cases[0].LLM.Events)
	}
	if report.Cases[1].LLM == nil || report.Cases[1].LLM.StreamEvents != 0 || report.Cases[1].LLM.Turns != 1 {
		t.Fatalf("plain case llm = %+v, want one non-streaming turn", report.Cases[1].LLM)
	}
	if !evaluateLLMEventsContain(report.Cases[1].LLM.Events, "turn_start", "turn_end") {
		t.Fatalf("plain case events = %+v, want turn_start/turn_end", report.Cases[1].LLM.Events)
	}
	if !evaluateLLMEventsSequenced(report.Cases[0].LLM.Events) || !evaluateLLMEventsSequenced(report.Cases[1].LLM.Events) {
		t.Fatalf("case events are not sequenced: streamed=%+v plain=%+v", report.Cases[0].LLM.Events, report.Cases[1].LLM.Events)
	}
	for _, c := range report.Cases {
		if c.LLM == nil || c.LLM.RecordPath == "" {
			t.Fatalf("case %q missing per-case record path: %+v", c.Name, c.LLM)
		}
		if _, err := os.Stat(c.LLM.RecordPath); err != nil {
			t.Fatalf("case record %s missing: %v", c.LLM.RecordPath, err)
		}
	}
	records, err := llm.LoadRecords(recordPath)
	if err != nil {
		t.Fatalf("load global record: %v", err)
	}
	if len(records) != 2 || !records[0].Request.Stream || records[0].Result.Text != "OK-1" || records[1].Request.Stream {
		t.Fatalf("records = %#v, want first streaming turn and second plain turn", records)
	}
	if len(records[0].StreamEvents) != 3 || records[0].StreamEvents[0].Token != "OK" || records[0].StreamEvents[2].Token != "1" {
		t.Fatalf("stream record events = %#v, want recorded stream tokens", records[0].StreamEvents)
	}
	mu.Lock()
	gotRequests := append([]anthropicRequest(nil), requests...)
	mu.Unlock()
	if len(gotRequests) != 2 || gotRequests[0].Model != "mock-model" || gotRequests[1].Model != "mock-model" {
		t.Fatalf("requests = %#v, want two mock-model requests", gotRequests)
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--json", "--llm-replay", recordPath, sourcePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --llm-replay code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var replayReport struct {
		Status string `json:"status"`
		LLM    *struct {
			Mode           string `json:"mode"`
			ReplayedTurns  int    `json:"replayed_turns"`
			RemainingTurns int    `json:"remaining_turns"`
			StreamEvents   int    `json:"stream_events"`
		} `json:"llm"`
		Cases []struct {
			Name string `json:"name"`
			LLM  *struct {
				StreamEvents int `json:"stream_events"`
				Events       []struct {
					Type            string `json:"type"`
					Status          string `json:"status"`
					ReplayMode      string `json:"replay_mode"`
					ReplayKey       string `json:"replay_key"`
					RequestHash     string `json:"request_hash"`
					ResponseHash    string `json:"response_hash"`
					ProviderFree    bool   `json:"provider_free"`
					ReplaySessionID string `json:"replay_session_id"`
				} `json:"events"`
			} `json:"llm"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &replayReport); err != nil {
		t.Fatalf("replay stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if replayReport.Status != "ok" || replayReport.LLM == nil || replayReport.LLM.Mode != "replay" || replayReport.LLM.ReplayedTurns != 2 || replayReport.LLM.RemainingTurns != 0 {
		t.Fatalf("replay llm report = %+v", replayReport.LLM)
	}
	if replayReport.LLM.StreamEvents != 3 {
		t.Fatalf("replay llm stream_events = %d, want 3", replayReport.LLM.StreamEvents)
	}
	if len(replayReport.Cases) != 2 || replayReport.Cases[0].LLM == nil || replayReport.Cases[0].LLM.StreamEvents != 3 {
		t.Fatalf("replay cases = %+v, want streamed case to replay three stream events", replayReport.Cases)
	}
	if !evaluateLLMReplayMatchedEventOK(replayReport.Cases[0].LLM.Events) || !evaluateLLMReplayMatchedEventOK(replayReport.Cases[1].LLM.Events) {
		t.Fatalf("replay events missing matched provider-free metadata: %+v", replayReport.Cases)
	}
}

func TestEvaluateLLMReplayAliasAndFixtureModeGuards(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "examples", "evaluate", "multiturn_replay.leia")
	replayPath := filepath.Join(root, "examples", "evaluate", "multiturn_replay.records.json")

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", "--parallel=4", "--llm-replay", replayPath, sourcePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --llm-replay code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report struct {
		Status string `json:"status"`
		LLM    *struct {
			Mode           string `json:"mode"`
			ReplayPath     string `json:"replay_path"`
			LoadedTurns    int    `json:"loaded_turns"`
			ReplayedTurns  int    `json:"replayed_turns"`
			RemainingTurns int    `json:"remaining_turns"`
		} `json:"llm"`
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if report.Status != "ok" || report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.ReplayPath != replayPath || report.LLM.LoadedTurns != 2 || report.LLM.ReplayedTurns != 2 || report.LLM.RemainingTurns != 0 {
		t.Fatalf("llm report = %+v, want replay alias to consume both turns", report.LLM)
	}
	if !containsEvaluateLLMString(report.Notes, "parallel evaluate execution disabled for deterministic LLM fixture mode") {
		t.Fatalf("notes = %#v, want deterministic fixture parallel guard note", report.Notes)
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--record", filepath.Join(t.TempDir(), "new.records.json"), "--llm-replay", replayPath, sourcePath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("evaluate accepted conflicting fixture modes, stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "llm record, replay, and update-golden modes are mutually exclusive") {
		t.Fatalf("stderr = %q, want mutually exclusive fixture mode error", stderr.String())
	}
}

func TestEvaluateLLMReplayReportsDeterministicFixtureDrift(t *testing.T) {
	for _, tc := range []struct {
		name        string
		source      string
		records     []llm.Record
		wantFinding string
	}{
		{
			name: "mismatch",
			source: `
evaluate "replay mismatch case" {
    result, err := llm.turn({
        model: "mock-fast"
        messages: [llm.user("actual prompt")]
    })
    assert(err == nil)
    _ = result
}
`,
			records: []llm.Record{{
				Request: llm.TurnRequest{
					Model:    "mock-fast",
					Messages: []llm.Message{{Role: "user", Text: "expected prompt"}},
				},
				Result: llm.TurnResult{Status: "final_answer", Text: "ok"},
			}},
			wantFinding: "llm_replay_mismatch",
		},
		{
			name: "unconsumed",
			source: `
evaluate "replay unconsumed case" {
    assert(true)
}
`,
			records: []llm.Record{{
				Request: llm.TurnRequest{
					Model:    "mock-fast",
					Messages: []llm.Message{{Role: "user", Text: "unused prompt"}},
				},
				Result: llm.TurnResult{Status: "final_answer", Text: "unused"},
			}},
			wantFinding: "llm_replay_unconsumed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			sourcePath := filepath.Join(dir, "drift.leia")
			replayPath := filepath.Join(dir, "drift.records.json")
			if err := os.WriteFile(sourcePath, []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := llm.SaveRecords(replayPath, tc.records); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			code := runEvaluateCommand([]string{"--json", "--llm-replay", replayPath, sourcePath}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("evaluate --llm-replay code = %d, want 1; stderr = %q stdout = %q", code, stderr.String(), stdout.String())
			}
			var report struct {
				Status string `json:"status"`
				LLM    *struct {
					Mode           string `json:"mode"`
					LoadedTurns    int    `json:"loaded_turns"`
					ReplayedTurns  int    `json:"replayed_turns"`
					RemainingTurns int    `json:"remaining_turns"`
				} `json:"llm"`
				Findings []struct {
					Kind    string         `json:"kind"`
					Path    string         `json:"path"`
					Details map[string]any `json:"details"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
			}
			if report.Status != "failed" || report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.LoadedTurns != 1 {
				t.Fatalf("report status/llm = %q/%+v, want failed replay report", report.Status, report.LLM)
			}
			if tc.wantFinding == "llm_replay_mismatch" && (report.LLM.ReplayedTurns != 1 || report.LLM.RemainingTurns != 0) {
				t.Fatalf("mismatch llm report = %+v, want consumed mismatched turn", report.LLM)
			}
			if tc.wantFinding == "llm_replay_unconsumed" && (report.LLM.ReplayedTurns != 0 || report.LLM.RemainingTurns != 1) {
				t.Fatalf("unconsumed llm report = %+v, want one remaining turn", report.LLM)
			}
			if !evaluateReportHasFinding(report.Findings, tc.wantFinding, sourcePath, replayPath) {
				t.Fatalf("findings = %+v, want %s tied to replay/source path", report.Findings, tc.wantFinding)
			}
		})
	}
}

func evaluateReportHasFinding(findings []struct {
	Kind    string         `json:"kind"`
	Path    string         `json:"path"`
	Details map[string]any `json:"details"`
}, wantKind, sourcePath, replayPath string) bool {
	for _, finding := range findings {
		if finding.Kind != wantKind {
			continue
		}
		switch wantKind {
		case "llm_replay_mismatch":
			return finding.Path == sourcePath && finding.Details["case_name"] == "replay mismatch case"
		case "llm_replay_unconsumed":
			return finding.Path == replayPath && finding.Details["remaining_turns"] == float64(1)
		default:
			return true
		}
	}
	return false
}

func containsEvaluateLLMString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func evaluateLLMEventsContain(events []struct {
	Type     string `json:"type"`
	TraceID  string `json:"trace_id"`
	EventID  string `json:"event_id"`
	Sequence int64  `json:"sequence"`
}, wants ...string) bool {
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Type] = true
	}
	for _, want := range wants {
		if !seen[want] {
			return false
		}
	}
	return true
}

func evaluateLLMEventsSequenced(events []struct {
	Type     string `json:"type"`
	TraceID  string `json:"trace_id"`
	EventID  string `json:"event_id"`
	Sequence int64  `json:"sequence"`
}) bool {
	for i, event := range events {
		if event.TraceID == "" || event.EventID == "" || event.Sequence != int64(i+1) {
			return false
		}
	}
	return len(events) > 0
}

func evaluateLLMReplayMatchedEventOK(events []struct {
	Type            string `json:"type"`
	Status          string `json:"status"`
	ReplayMode      string `json:"replay_mode"`
	ReplayKey       string `json:"replay_key"`
	RequestHash     string `json:"request_hash"`
	ResponseHash    string `json:"response_hash"`
	ProviderFree    bool   `json:"provider_free"`
	ReplaySessionID string `json:"replay_session_id"`
}) bool {
	for _, event := range events {
		if event.Type == "replay_record_matched" {
			return event.Status == "matched" &&
				event.ReplayMode == "fixture_replay" &&
				event.ReplayKey != "" &&
				strings.HasPrefix(event.RequestHash, "sha256:") &&
				strings.HasPrefix(event.ResponseHash, "sha256:") &&
				event.ProviderFree &&
				strings.HasPrefix(event.ReplaySessionID, "llm-replay:")
		}
	}
	return false
}
