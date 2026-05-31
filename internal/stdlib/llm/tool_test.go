package llm

import "testing"

func TestToolCapabilitiesDeduplicatesInOrder(t *testing.T) {
	got := ToolCapabilities([]ToolSummary{
		{Name: "search", Requires: []string{"net.http", "fs.read"}},
		{Name: "cache", Requires: []string{"fs.read", "", "kv.write"}},
	})
	want := []string{"net.http", "fs.read", "kv.write"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cap[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestCheckToolCapabilities(t *testing.T) {
	tools := []ToolSummary{
		{Name: "search", Requires: []string{"net.http"}},
		{Name: "noop", Requires: []string{"cap.none"}},
	}
	if missing := CheckToolCapabilities(tools, []string{"net.http"}); missing != nil {
		t.Fatalf("missing = %#v, want nil", missing)
	}
	if missing := CheckToolCapabilities(tools, []string{"*"}); missing != nil {
		t.Fatalf("wildcard missing = %#v, want nil", missing)
	}
	missing := CheckToolCapabilities(tools, []string{"cap.none"})
	if missing == nil {
		t.Fatal("missing = nil, want net.http")
	}
	if missing.Tool != "search" || missing.Capability != "net.http" {
		t.Fatalf("missing = %#v, want search/net.http", missing)
	}
}
