package methodjit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOptimizationRemarksAddWithFieldsPreservesStructuredFields(t *testing.T) {
	remarks := &OptimizationRemarks{}
	remarks.AddWithFields("QQueryNativeLowering", "missed", 1, 2, OpCall, "human readable fallback", map[string]string{
		"kernel":      "QJoin",
		"shape":       "join/left",
		"reason_code": "join_call",
	})

	list := remarks.List()
	if len(list) != 1 {
		t.Fatalf("remarks length = %d, want 1", len(list))
	}
	got := list[0]
	if got.Fields["kernel"] != "QJoin" || got.Fields["shape"] != "join/left" || got.Fields["reason_code"] != "join_call" {
		t.Fatalf("remark fields = %+v, want structured q lowering fields", got.Fields)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal remark: %v", err)
	}
	if !strings.Contains(string(encoded), `"fields"`) || !strings.Contains(string(encoded), `"reason_code":"join_call"`) {
		t.Fatalf("remark json = %s, want fields with reason_code", encoded)
	}
}

func TestOptimizationRemarksFieldsParticipateInDedupe(t *testing.T) {
	remarks := &OptimizationRemarks{}
	remarks.AddWithFields("QQueryNativeLowering", "missed", 1, 2, OpCall, "fallback", map[string]string{
		"shape":       "join/left",
		"reason_code": "join_call",
	})
	remarks.AddWithFields("QQueryNativeLowering", "missed", 1, 2, OpCall, "fallback", map[string]string{
		"shape":       "join/inner",
		"reason_code": "join_call",
	})
	remarks.AddWithFields("QQueryNativeLowering", "missed", 1, 2, OpCall, "fallback", map[string]string{
		"shape":       "join/inner",
		"reason_code": "join_call",
	})

	if got := remarks.Len(); got != 2 {
		t.Fatalf("remarks length = %d, want fields-distinct remarks deduped to 2", got)
	}
}

func TestOptimizationRemarksListClonesFields(t *testing.T) {
	remarks := &OptimizationRemarks{}
	remarks.AddWithFields("QVectorNativeLowering", "missed", 1, 2, OpVectorReduce, "fallback", map[string]string{
		"shape":       "gather/vector-reduce",
		"reason_code": "shared_gather",
	})

	list := remarks.List()
	list[0].Fields["reason_code"] = "mutated"
	again := remarks.List()
	if got := again[0].Fields["reason_code"]; got != "shared_gather" {
		t.Fatalf("remark fields mutated through List: reason_code = %q", got)
	}
}
