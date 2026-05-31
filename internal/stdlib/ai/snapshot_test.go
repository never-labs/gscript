package ai

import "testing"

func TestSnapshotToken(t *testing.T) {
	first, err := SnapshotToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := SnapshotToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("token lengths = %d/%d, want 32/32", len(first), len(second))
	}
	if first == second {
		t.Fatalf("tokens should be unique: %q", first)
	}
}
