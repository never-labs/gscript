package main

import (
	"os"
	"testing"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("LEIA_TEST_HELPER") == "" {
		return
	}
	switch os.Getenv("LEIA_TEST_HELPER") {
	case "bench":
		_, _ = os.Stdout.WriteString("bench helper ok\n")
		os.Exit(0)
	case "bench-exit-profile":
		_, _ = os.Stdout.WriteString("Time: 0.125s\n")
		_, _ = os.Stdout.WriteString("{\n  \"total\": 3,\n  \"by_exit_code\": {\"ExitDeopt\": 3},\n  \"sites\": [{\"count\": 3, \"proto\": \"main\", \"exit_name\": \"ExitDeopt\", \"pc\": 7, \"op_id\": 11, \"reason\": \"guard:type\"}]\n}\n")
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
	t.Setenv("LEIA_TEST_HELPER", helper)
	return os.Args[0], args
}
