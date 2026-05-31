package modules

import "github.com/never-labs/gscript/internal/runtime"

// BuildSOA creates the "soa" standard library table.
//
// The SOA binding layer still owns many runtime fast paths and value adapters,
// while the heavy shape/projection logic already lives in internal/stdlib/soa.
// Keep one builder for now so stdlibrt and the legacy runtime path cannot drift.
func BuildSOA() *runtime.Table {
	return runtime.BuildSoALib()
}
