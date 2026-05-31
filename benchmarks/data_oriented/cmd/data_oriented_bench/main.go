package main

import (
	"os"

	"github.com/never-labs/gscript/benchmarks/data_oriented/report"
)

func main() {
	// Compatibility forwarder for the legacy command path.
	// The canonical entrypoint is benchmarks/cmd/data_oriented_bench.
	os.Exit(report.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
