//go:build leia_q

package benchdisc

// EnabledInBuild reports whether a benchmark belongs to the current product
// build. The leia_q build validates both core and optional q workloads.
func EnabledInBuild(group, name string) bool {
	return true
}
