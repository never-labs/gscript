//go:build darwin && arm64

package methodjit

import "github.com/Never-Labs/gscript/internal/vm"

// ForEachSpecDependencyProto visits each recorded spec-dependency proto. The
// visit callback may return false to stop iteration early. It is a read-only
// accessor and does not mutate the facts, so no bindOwner() is required.
func (s *SpeculationFacts) ForEachSpecDependencyProto(visit func(proto *vm.FuncProto) bool) {
	if s == nil || s.specDependencyProtos == nil || visit == nil {
		return
	}
	for proto := range s.specDependencyProtos {
		if !visit(proto) {
			return
		}
	}
}

// SpecDependencyProtoCount reports the number of recorded spec-dependency
// protos. Read-only; no bindOwner() required.
func (s *SpeculationFacts) SpecDependencyProtoCount() int {
	if s == nil {
		return 0
	}
	return len(s.specDependencyProtos)
}

// SpecDependencyProto reports whether the given proto is a recorded
// spec-dependency. Read-only.
func (s *SpeculationFacts) SpecDependencyProto(proto *vm.FuncProto) bool {
	return s != nil && s.specDependencyProtos != nil && s.specDependencyProtos[proto]
}

// SuppressedSpecGuardPC reports whether the given bytecode PC has its runtime
// guard suppressed. Read-only.
func (s *SpeculationFacts) SuppressedSpecGuardPC(pc int) bool {
	return s != nil && s.suppressedSpecGuardPCs != nil && s.suppressedSpecGuardPCs[pc]
}

// SuppressedSpecGuardKindsMap returns the underlying guard-kind-scoped
// suppression map. Nil is a sentinel meaning kind information is unavailable.
// Callers read or iterate without mutating; mutation goes through
// SetSuppressedSpecGuardKinds.
func (s *SpeculationFacts) SuppressedSpecGuardKindsMap() map[int]map[string]bool {
	if s == nil {
		return nil
	}
	return s.suppressedSpecGuardKinds
}
