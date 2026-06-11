package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const genericTraceEnvelopeFixture = "examples/ai/finrobot_translation/ai_dialect_index/fixtures/generic_trace_envelope_fixture.json"

type genericTraceEnvelopeFixtureDoc struct {
	SchemaVersion       int    `json:"schema_version"`
	ID                  string `json:"id"`
	PackageBoundaryID   string `json:"package_boundary_id"`
	ProviderFree        bool   `json:"provider_free"`
	DomainSpecific      bool   `json:"domain_specific"`
	LiveNetwork         bool   `json:"live_network"`
	LiveModel           bool   `json:"live_model"`
	CredentialsRequired bool   `json:"credentials_required"`
	TraceEnvelopeSchema struct {
		Name                string   `json:"name"`
		RequiredFields      []string `json:"required_fields"`
		CorrelationIDFields []string `json:"correlation_id_fields"`
	} `json:"trace_envelope_schema"`
	SourceCoverage []struct {
		SourceID string   `json:"source_id"`
		Surface  string   `json:"surface"`
		Covers   []string `json:"covers"`
	} `json:"source_coverage"`
	Events []genericTraceEnvelopeEvent `json:"events"`
}

type genericTraceEnvelopeEvent struct {
	TraceID        string            `json:"trace_id"`
	EventID        string            `json:"event_id"`
	EventType      string            `json:"event_type"`
	TimestampMS    int64             `json:"timestamp_ms"`
	Sequence       int               `json:"sequence"`
	Status         string            `json:"status"`
	Correlation    map[string]string `json:"correlation"`
	rawEnvelopeSet map[string]bool
}

func TestGenericTraceEventsUnifiedEnvelopeCoversWorkflowCompositionCorrelations(t *testing.T) {
	root := repoRoot(t)
	doc := loadGenericTraceEnvelopeFixture(t, root)

	if doc.SchemaVersion != 1 ||
		doc.ID != "generic-ai-trace-envelope-fixture" ||
		doc.PackageBoundaryID != "generic-ai-trace-events" {
		t.Fatalf("unexpected trace envelope header: %#v", doc)
	}
	if !doc.ProviderFree || doc.DomainSpecific || doc.LiveNetwork || doc.LiveModel || doc.CredentialsRequired {
		t.Fatalf("trace envelope fixture is not provider-free and generic: %#v", doc)
	}

	fixtureData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(genericTraceEnvelopeFixture)))
	if err != nil {
		t.Fatal(err)
	}
	assertGenericTraceFixtureHasNoSpecializedMarkers(t, string(fixtureData))

	assertGenericTraceSourceCoverage(t, doc)
	assertGenericTraceSchemaFields(t, doc.TraceEnvelopeSchema.RequiredFields, []string{
		"correlation",
		"event_id",
		"event_type",
		"sequence",
		"timestamp_ms",
		"trace_id",
	})
	assertGenericTraceSchemaFields(t, doc.TraceEnvelopeSchema.CorrelationIDFields, []string{
		"approval_id",
		"agent_run_id",
		"parent_event_id",
		"replay_session_id",
		"tool_call_id",
		"turn_id",
		"workflow_run_id",
		"workflow_step_id",
	})

	wantByEventType := map[string][]string{
		"turn_start":            {"turn_id", "workflow_run_id", "replay_session_id"},
		"turn_end":              {"turn_id", "workflow_run_id", "replay_session_id", "parent_event_id"},
		"tool_call":             {"turn_id", "tool_call_id", "workflow_run_id", "workflow_step_id", "replay_session_id"},
		"tool_result":           {"turn_id", "tool_call_id", "workflow_run_id", "workflow_step_id", "replay_session_id", "parent_event_id"},
		"approval_replay_trace": {"approval_id", "tool_call_id", "workflow_run_id", "workflow_step_id", "replay_session_id"},
		"workflow_step":         {"workflow_run_id", "workflow_step_id", "parent_event_id"},
		"replay_record_matched": {"turn_id", "tool_call_id", "workflow_run_id", "replay_session_id", "parent_event_id"},
		"agent_start":           {"agent_run_id"},
		"agent_turn_tool_dispatch": {
			"agent_run_id",
			"turn_id",
			"tool_call_id",
			"parent_event_id",
		},
		"agent_done": {"agent_run_id", "parent_event_id"},
	}
	seenTypes := map[string]bool{}
	seenIDs := map[string]bool{}
	var lastSequence int
	var lastTimestamp int64
	for _, event := range doc.Events {
		if event.TraceID == "" || event.EventID == "" || event.EventType == "" || event.TimestampMS == 0 || event.Sequence == 0 || event.Status == "" {
			t.Fatalf("trace event missing unified envelope fields: %#v", event)
		}
		if seenIDs[event.EventID] {
			t.Fatalf("duplicate event_id %q", event.EventID)
		}
		seenIDs[event.EventID] = true
		if event.Sequence <= lastSequence || event.TimestampMS < lastTimestamp {
			t.Fatalf("trace events are not stable ordered: %#v after seq=%d ts=%d", event, lastSequence, lastTimestamp)
		}
		lastSequence = event.Sequence
		lastTimestamp = event.TimestampMS
		wantCorrelation, ok := wantByEventType[event.EventType]
		if !ok {
			t.Fatalf("unexpected event_type %q", event.EventType)
		}
		assertGenericTraceCorrelationIDs(t, event, wantCorrelation)
		seenTypes[event.EventType] = true
	}
	for eventType := range wantByEventType {
		if !seenTypes[eventType] {
			t.Fatalf("missing trace event type %q; got %v", eventType, sortedGenericTraceKeys(seenTypes))
		}
	}
}

func loadGenericTraceEnvelopeFixture(t *testing.T, root string) genericTraceEnvelopeFixtureDoc {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(genericTraceEnvelopeFixture))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	var doc genericTraceEnvelopeFixtureDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode typed %s: %v", path, err)
	}
	for i := range doc.Events {
		doc.Events[i].rawEnvelopeSet = map[string]bool{}
		for key := range raw.Events[i] {
			doc.Events[i].rawEnvelopeSet[key] = true
		}
	}
	return doc
}

func assertGenericTraceSourceCoverage(t *testing.T, doc genericTraceEnvelopeFixtureDoc) {
	t.Helper()
	want := map[string][]string{
		"generic_trace_events":            {"approval", "replay", "tool", "turn", "workflow"},
		"generic_workflow_orchestration":  {"approval", "workflow"},
		"generic_ai_workflow_composition": {"replay", "tool", "turn"},
		"generic_agent_loop_composition":  {"agent", "tool", "trace", "turn"},
	}
	got := map[string][]string{}
	for _, coverage := range doc.SourceCoverage {
		if coverage.SourceID == "" || coverage.Surface == "" || len(coverage.Covers) == 0 {
			t.Fatalf("incomplete source coverage entry: %#v", coverage)
		}
		got[coverage.SourceID] = append([]string(nil), coverage.Covers...)
		sort.Strings(got[coverage.SourceID])
	}
	for sourceID, covers := range want {
		sort.Strings(covers)
		if strings.Join(got[sourceID], ",") != strings.Join(covers, ",") {
			t.Fatalf("%s source coverage = %#v, want %#v", sourceID, got[sourceID], covers)
		}
	}
}

func assertGenericTraceSchemaFields(t *testing.T, got, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("schema fields = %#v, want %#v", got, want)
	}
}

func assertGenericTraceCorrelationIDs(t *testing.T, event genericTraceEnvelopeEvent, want []string) {
	t.Helper()
	for _, field := range []string{"trace_id", "event_id", "event_type", "timestamp_ms", "sequence", "correlation"} {
		if !event.rawEnvelopeSet[field] {
			t.Fatalf("%s missing raw envelope field %q", event.EventID, field)
		}
	}
	if len(event.Correlation) == 0 {
		t.Fatalf("%s missing correlation envelope", event.EventID)
	}
	for _, key := range want {
		if event.Correlation[key] == "" {
			t.Fatalf("%s %s correlation id is empty: %#v", event.EventID, key, event.Correlation)
		}
	}
}

func assertGenericTraceFixtureHasNoSpecializedMarkers(t *testing.T, fixture string) {
	t.Helper()
	lower := strings.ToLower(fixture)
	for _, forbidden := range []string{
		`"provider":`,
		"anthropic",
		"autogen",
		"finance",
		"fingpt",
		"openai",
		"openbb",
		"trading",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("generic trace envelope fixture contains specialized marker %q", forbidden)
		}
	}
}

func sortedGenericTraceKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
