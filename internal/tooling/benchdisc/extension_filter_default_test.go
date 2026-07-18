//go:build !leia_q

package benchdisc

import "testing"

func TestDomainSpecsExcludeOptionalQBenchmarksFromDefaultBuild(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"soa_dot", "q_operator_pipeline", "qsql_join_variants", "frame_qsql_rollup"} {
		writeBenchFile(t, root, "data", name)
	}

	specs, err := DomainSpecs(root, "data")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].ID() != "data/soa_dot" {
		t.Fatalf("default data specs = %#v, want only data/soa_dot", specs)
	}
}
