package main

import (
	"os"
	"testing"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GSCRIPT_TEST_HELPER") == "" {
		return
	}
	switch os.Getenv("GSCRIPT_TEST_HELPER") {
	case "bench":
		_, _ = os.Stdout.WriteString("bench helper ok\n")
		os.Exit(0)
	case "ci":
		_, _ = os.Stdout.WriteString("ci helper ok\n")
		os.Exit(0)
	case "diag":
		_, _ = os.Stdout.WriteString("diag helper ok\n")
		os.Exit(0)
	case "doc":
		_, _ = os.Stdout.WriteString("doc helper ok\n")
		os.Exit(0)
	case "docs":
		_, _ = os.Stdout.WriteString("docs helper ok\n")
		os.Exit(0)
	case "manifest":
		_, _ = os.Stdout.WriteString("manifest helper ok\n")
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func testHelperCommand(t *testing.T, helper string) (string, []string) {
	t.Helper()
	args := []string{"-test.run=TestHelperProcess", "--"}
	t.Setenv("GSCRIPT_TEST_HELPER", helper)
	return os.Args[0], args
}
