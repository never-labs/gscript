package main

import (
	"os"

	"github.com/never-labs/gscript/benchmarks/layoutbench/report"
)

func main() {
	os.Exit(report.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
