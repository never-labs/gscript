package benchdisc

import "strings"

// IsOptionalQBenchmark reports whether a benchmark belongs to the in-repository
// q extension rather than the default Leia product surface.
func IsOptionalQBenchmark(group, name string) bool {
	if group != "data" {
		return false
	}
	return strings.HasPrefix(name, "q_") ||
		strings.HasPrefix(name, "qsql_") ||
		strings.HasPrefix(name, "frame_qsql_")
}
