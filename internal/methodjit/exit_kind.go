package methodjit

import "fmt"

// ExitKind is the typed enumeration of the JIT↔Go exit protocol codes.
//
// It is backed by int64 so the ARM64 ABI / ExecContext struct layout is
// byte-identical to the previous untyped int64 ExitCode: generated code still
// stores and loads the same 8-byte values, and unsafe.Offsetof/Sizeof of the
// ExecContext.ExitCode field are unchanged. The type exists purely for
// compile-time safety and centralized diagnostics; it does not alter runtime
// behavior or emitted instructions.
type ExitKind int64

// Exit protocol kinds. The numeric values are part of the ARM64 ABI and MUST
// NOT change: generated native code writes these literals into
// ExecContext.ExitCode, and the Go-side dispatch switches on them.
const (
	ExitNormal             ExitKind = 0  // normal return
	ExitDeopt              ExitKind = 2  // deopt: bail to interpreter for the entire function
	ExitCallExit           ExitKind = 3  // call-exit: pause JIT, execute call via VM, resume JIT
	ExitGlobalExit         ExitKind = 4  // global-exit: pause JIT, load global via VM, resume JIT
	ExitTableExit          ExitKind = 5  // table-exit: pause JIT, do table op via Go, resume JIT
	ExitOpExit             ExitKind = 6  // op-exit: pause JIT, Go handles the operation, resume JIT
	ExitBaselineOpExit     ExitKind = 7  // baseline op-exit: bytecode-level exit for Tier 1
	ExitNativeCallExit     ExitKind = 8  // native call exit: callee hit exit-resume during BLR call
	ExitOSR                ExitKind = 9  // OSR: Tier 1 loop counter expired, request Tier 2 compilation
	ExitCoroutineYieldFast ExitKind = 10 // coroutine yield fast path
)

// exitKindInfo describes one exit kind in the centralized table.
type exitKindInfo struct {
	Kind ExitKind
	Name string
}

// allExitKinds enumerates every exit kind together with its diagnostic name.
// This is the single source of truth consumed by String() and by the
// validation that the Go-side exit dispatch handles every kind.
var allExitKinds = []exitKindInfo{
	{ExitNormal, "ExitNormal"},
	{ExitDeopt, "ExitDeopt"},
	{ExitCallExit, "ExitCallExit"},
	{ExitGlobalExit, "ExitGlobalExit"},
	{ExitTableExit, "ExitTableExit"},
	{ExitOpExit, "ExitOpExit"},
	{ExitBaselineOpExit, "ExitBaselineOpExit"},
	{ExitNativeCallExit, "ExitNativeCallExit"},
	{ExitOSR, "ExitOSR"},
	{ExitCoroutineYieldFast, "ExitCoroutineYieldFast"},
}

// exitKindNames is the lookup map derived from allExitKinds.
var exitKindNames = func() map[ExitKind]string {
	m := make(map[ExitKind]string, len(allExitKinds))
	for _, info := range allExitKinds {
		m[info.Kind] = info.Name
	}
	return m
}()

// String returns the diagnostic name of the exit kind, or a synthetic
// "ExitKind(N)" form for any unknown value.
func (k ExitKind) String() string {
	if name, ok := exitKindNames[k]; ok {
		return name
	}
	return fmt.Sprintf("ExitKind(%d)", int64(k))
}

// IsKnownExitKind reports whether k is one of the enumerated exit kinds.
func IsKnownExitKind(k ExitKind) bool {
	_, ok := exitKindNames[k]
	return ok
}
