//go:build !leia_q

package benchdisc

// EnabledInBuild reports whether a benchmark belongs to the current product
// build. Optional q workloads stay out of default release and performance gates.
func EnabledInBuild(group, name string) bool {
	return !IsOptionalQBenchmark(group, name)
}
