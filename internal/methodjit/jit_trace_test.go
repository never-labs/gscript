//go:build darwin && arm64

package methodjit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/vm"
)

func TestJITTimelineJSONL(t *testing.T) {
	var buf bytes.Buffer
	timeline, err := NewJITTimeline(&buf, JITTimelineJSONL)
	if err != nil {
		t.Fatalf("NewJITTimeline: %v", err)
	}

	timeline.Record(JITTimelineEvent{
		Event: "tier2_attempt",
		Tier:  "tier2",
		Proto: "hot",
		Attrs: map[string]any{"attempt": 1},
	})
	if err := timeline.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1: %q", len(lines), buf.String())
	}
	var ev JITTimelineEvent
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal JSONL event: %v", err)
	}
	if ev.Seq != 1 || ev.Event != "tier2_attempt" || ev.Tier != "tier2" || ev.Proto != "hot" {
		t.Fatalf("unexpected event: %+v", ev)
	}
	if ev.Time == "" || ev.UnixNano == 0 {
		t.Fatalf("timestamp fields not populated: %+v", ev)
	}
}

func TestJITTimelineJSON(t *testing.T) {
	var buf bytes.Buffer
	timeline, err := NewJITTimeline(&buf, JITTimelineJSON)
	if err != nil {
		t.Fatalf("NewJITTimeline: %v", err)
	}

	timeline.Record(JITTimelineEvent{Event: "tier1_compile", Tier: "tier1", Proto: "a"})
	timeline.Record(JITTimelineEvent{Event: "fallback", Tier: "tier0", Proto: "b", Attrs: map[string]any{"target": "interpreter"}})
	if err := timeline.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var events []JITTimelineEvent
	if err := json.Unmarshal(buf.Bytes(), &events); err != nil {
		t.Fatalf("unmarshal JSON events: %v\n%s", err, buf.String())
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("sequence not monotonic: %+v", events)
	}
	if events[1].Attrs["target"] != "interpreter" {
		t.Fatalf("missing attrs: %+v", events[1].Attrs)
	}
}

func TestNewJITTimelineRejectsUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewJITTimeline(&buf, "yaml"); err == nil {
		t.Fatal("expected unknown format error")
	}
}

func TestTraceTier2SuccessIncludesSpecializationVersion(t *testing.T) {
	var buf bytes.Buffer
	timeline, err := NewJITTimeline(&buf, JITTimelineJSON)
	if err != nil {
		t.Fatalf("NewJITTimeline: %v", err)
	}
	tm := NewTieringManager()
	tm.SetTimeline(timeline)
	proto := &vm.FuncProto{Name: "hot"}
	tm.suppressTier2Guard(proto, 7)
	tm.traceTier2Success(proto, &CompiledFunction{
		SpecializationVersion: Tier2SpecializationVersion{Hash: 0xabc, GuardCount: 3},
		numRegs:               5,
	}, 2)
	if err := timeline.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var events []JITTimelineEvent
	if err := json.Unmarshal(buf.Bytes(), &events); err != nil {
		t.Fatalf("unmarshal JSON events: %v\n%s", err, buf.String())
	}
	if len(events) != 1 {
		t.Fatalf("events=%d want 1", len(events))
	}
	attrs := events[0].Attrs
	if attrs["version_hash"] != "abc" || attrs["guard_count"].(float64) != 3 {
		t.Fatalf("missing specialization attrs: %#v", attrs)
	}
	if attrs["suppressed_count"].(float64) != 1 {
		t.Fatalf("missing suppressed count: %#v", attrs)
	}
}

func TestTracePromotionDecisionRecordsReasonAndCallablePolicy(t *testing.T) {
	var buf bytes.Buffer
	timeline, err := NewJITTimeline(&buf, JITTimelineJSON)
	if err != nil {
		t.Fatalf("NewJITTimeline: %v", err)
	}
	tm := NewTieringManager()
	tm.SetTimeline(timeline)
	proto := &vm.FuncProto{Name: "vararg_unused", IsVarArg: true}

	tm.tracePromotionDecision(proto, PromotionDecision{
		Action:       TieringActionUseTier1,
		Reason:       PromotionReasonNotReadyForTier2,
		Gate:         blockGate("FeedbackReadiness", "waiting for feedback"),
		PromoteTier2: false,
	})
	tm.tracePromotionDecision(&vm.FuncProto{Name: "hot", CallCount: 10}, PromotionDecision{
		Action:       TieringActionPromoteTier2,
		Reason:       PromotionReasonSmartTier2,
		Gate:         allowGate("SmartTiering", "profile selected Tier 2"),
		PromoteTier2: true,
	})
	if err := timeline.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var events []JITTimelineEvent
	if err := json.Unmarshal(buf.Bytes(), &events); err != nil {
		t.Fatalf("unmarshal JSON events: %v\n%s", err, buf.String())
	}
	if len(events) != 2 {
		t.Fatalf("events=%d want 2", len(events))
	}
	first := events[0].Attrs
	if first["action"] != string(TieringActionUseTier1) || first["reason"] != string(PromotionReasonNotReadyForTier2) {
		t.Fatalf("missing non-promotion reason attrs: %#v", first)
	}
	if first["tier1_callable"] != true || first["tier1_callable_reason"] != vm.MethodJITCallableReasonDeclaredVarargTier1 {
		t.Fatalf("missing Tier 1 callable attrs: %#v", first)
	}
	if first["tier2_callable"] != true || first["tier2_callable_reason"] != vm.MethodJITCallableReasonDeclaredVarargTier2 {
		t.Fatalf("missing Tier 2 callable attrs: %#v", first)
	}
	second := events[1].Attrs
	if second["action"] != string(TieringActionPromoteTier2) || second["reason"] != string(PromotionReasonSmartTier2) || second["promote_tier2"] != true {
		t.Fatalf("missing promotion reason attrs: %#v", second)
	}
}
