//go:build darwin && arm64

package methodjit

import "github.com/gscript/gscript/internal/vm"

// ForEachSpecDependencyProto visits each recorded spec-dependency proto. The
// visit callback may return false to stop iteration early. It is a read-only
// accessor and does not mutate the facts, so no bindOwner() is required.
func (s *SpeculationFacts) ForEachSpecDependencyProto(visit func(proto *vm.FuncProto) bool) {
	if s == nil || s.SpecDependencyProtos == nil || visit == nil {
		return
	}
	for proto := range s.SpecDependencyProtos {
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
	return len(s.SpecDependencyProtos)
}
