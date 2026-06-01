package main

import (
	"os"

	"github.com/never-labs/leia/benchmarks/layoutbench/report"
)

func main() {
	os.Exit(report.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
