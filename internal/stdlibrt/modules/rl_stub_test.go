//go:build !rl
// +build !rl

package modules

import "testing"

func TestBuildRLStub(t *testing.T) {
	lib := BuildRL()
	if lib == nil {
		t.Fatal("BuildRL returned nil")
	}
	stub := lib.RawGetString("_stub")
	if !stub.IsBool() || !stub.Bool() {
		t.Fatalf("rl._stub = %v, want true", stub)
	}
}
