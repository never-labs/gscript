//go:build darwin && arm64

package methodjit

import (
	"testing"
	"unsafe"
)

// TestExitCodeABIUnchanged asserts the typed ExitKind field is still an
// 8-byte, int64-aligned value living at the same offset the generated ARM64
// code uses via execCtxOffExitCode. If this regresses, every STR/LDR of the
// exit code emitted by the JIT would target the wrong bytes.
func TestExitCodeABIUnchanged(t *testing.T) {
	if got := unsafe.Sizeof(ExecContext{}.ExitCode); got != 8 {
		t.Fatalf("ExecContext.ExitCode size = %d, want 8 (int64-backed)", got)
	}
	if got := unsafe.Alignof(ExecContext{}.ExitCode); got != 8 {
		t.Fatalf("ExecContext.ExitCode align = %d, want 8", got)
	}
	if got := int(unsafe.Offsetof(ExecContext{}.ExitCode)); got != execCtxOffExitCode {
		t.Fatalf("ExecContext.ExitCode offset = %d, want execCtxOffExitCode=%d", got, execCtxOffExitCode)
	}
	if got := unsafe.Sizeof(ExitKind(0)); got != 8 {
		t.Fatalf("sizeof(ExitKind) = %d, want 8", got)
	}
}

// TestExitKindNumericValues pins the ABI numeric values: generated native code
// writes these literals into ExecContext.ExitCode, so they must never drift.
func TestExitKindNumericValues(t *testing.T) {
	cases := []struct {
		kind ExitKind
		want int64
		name string
	}{
		{ExitNormal, 0, "ExitNormal"},
		{ExitDeopt, 2, "ExitDeopt"},
		{ExitCallExit, 3, "ExitCallExit"},
		{ExitGlobalExit, 4, "ExitGlobalExit"},
		{ExitTableExit, 5, "ExitTableExit"},
		{ExitOpExit, 6, "ExitOpExit"},
		{ExitBaselineOpExit, 7, "ExitBaselineOpExit"},
		{ExitNativeCallExit, 8, "ExitNativeCallExit"},
		{ExitOSR, 9, "ExitOSR"},
		{ExitCoroutineYieldFast, 10, "ExitCoroutineYieldFast"},
		{ExitQEvalPipelinePlan, 11, "ExitQEvalPipelinePlan"},
	}
	for _, c := range cases {
		if int64(c.kind) != c.want {
			t.Errorf("%s = %d, want %d", c.name, int64(c.kind), c.want)
		}
		if c.kind.String() != c.name {
			t.Errorf("%s.String() = %q, want %q", c.name, c.kind.String(), c.name)
		}
		if !IsKnownExitKind(c.kind) {
			t.Errorf("IsKnownExitKind(%s) = false, want true", c.name)
		}
	}
	if IsKnownExitKind(ExitKind(99)) {
		t.Errorf("IsKnownExitKind(99) = true, want false")
	}
	if got := ExitKind(99).String(); got != "ExitKind(99)" {
		t.Errorf("ExitKind(99).String() = %q, want %q", got, "ExitKind(99)")
	}
}

// TestExitKindDispatchExhaustive verifies that every enumerated exit kind is
// handled by some tier's Go-side exit dispatch. The set of handled kinds below
// must be kept in sync with the switch statements in tiering_execute.go (Tier
// 2), tier1_manager.go (Tier 1), and the in-native handling in
// tier1_compile.go (ExitCoroutineYieldFast). Adding a new ExitKind without
// wiring a handler trips this test.
func TestExitKindDispatchExhaustive(t *testing.T) {
	// Kinds handled by the Tier 2 Go-side exit loop (tiering_execute.go).
	tier2Handled := map[ExitKind]bool{
		ExitNormal:            true,
		ExitDeopt:             true,
		ExitCallExit:          true,
		ExitNativeCallExit:    true,
		ExitGlobalExit:        true,
		ExitTableExit:         true,
		ExitOpExit:            true,
		ExitQEvalPipelinePlan: true,
		ExitQEvalHelperErr:    true,
	}
	// Kinds handled by the Tier 1 Go-side exit loop (tier1_manager.go).
	tier1Handled := map[ExitKind]bool{
		ExitNormal:         true,
		ExitBaselineOpExit: true,
		ExitNativeCallExit: true,
		ExitOSR:            true,
		ExitDeopt:          true,
	}
	// Kinds resolved entirely within native code (never reach a Go switch as a
	// standalone dispatch target).
	nativeHandled := map[ExitKind]bool{
		ExitCoroutineYieldFast: true,
	}
	for _, info := range allExitKinds {
		if tier2Handled[info.Kind] || tier1Handled[info.Kind] || nativeHandled[info.Kind] {
			continue
		}
		t.Errorf("exit kind %s (%d) has no Go-side or native dispatch handler", info.Name, int64(info.Kind))
	}
}
