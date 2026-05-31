package debug

import "testing"

func TestDefaultHookOptionsWantsAllKnownEventsAndKinds(t *testing.T) {
	opts := DefaultHookOptions()
	for _, event := range []string{"call", "return", "error", "emit"} {
		for _, kind := range []string{"script", "native", "diagnostic"} {
			if !HookWants(opts, event, kind) {
				t.Fatalf("default hook rejected event=%s kind=%s", event, kind)
			}
		}
	}
	if HookWants(opts, "unknown", "script") {
		t.Fatalf("unknown event should be rejected")
	}
}

func TestHookWantsRespectsEventAndKindFilters(t *testing.T) {
	opts := HookOptions{Call: true, Script: true}
	if !HookWants(opts, "call", "script") {
		t.Fatalf("call script should be accepted")
	}
	if HookWants(opts, "return", "script") {
		t.Fatalf("return should be filtered")
	}
	if HookWants(opts, "call", "native") {
		t.Fatalf("native should be filtered")
	}
}

func TestFormatTraceback(t *testing.T) {
	got := FormatTraceback("boom", []Frame{
		{Name: "outer", Kind: "script", SourceName: "a.gs", Line: 2, Column: 3},
		{Name: "inner", Kind: "native"},
	})
	want := "boom\nstack traceback:\n  native inner\n  script outer @ a.gs:2:3"
	if got != want {
		t.Fatalf("traceback mismatch\nwant: %q\n got: %q", want, got)
	}
}
