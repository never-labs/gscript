package main

import (
	"os"

	"github.com/never-labs/gscript/benchmarks/data_oriented/report"
)

func main() {
	os.Exit(report.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
