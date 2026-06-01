package leia_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestOSEexitReturnsCatchableExitError(t *testing.T) {
	for _, tc := range []struct {
		name string
		vm   *leia.VM
	}{
		{name: "interpreter", vm: leia.New()},
		{name: "bytecode", vm: leia.New(leia.WithVM())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vm.Exec(`os.exit(7)`)
			if err == nil {
				t.Fatal("expected exit error")
			}
			var exitErr *leia.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected ExitError, got %T %v", err, err)
			}
			if exitErr.Code != 7 {
				t.Fatalf("exit code = %d, want 7", exitErr.Code)
			}
		})
	}
}

func TestOSEexitBooleanStatus(t *testing.T) {
	vm := leia.New()
	err := vm.Exec(`os.exit(false)`)
	var exitErr *leia.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("exit code = %d, want 1", exitErr.Code)
	}
}

// --- Error handling tests ---

func TestError_parseError(t *testing.T) {
	vm := leia.New()
	err := vm.Exec(`func {`)
	if err == nil {
		t.Fatal("expected error")
	}
	leiaErr, ok := err.(*leia.Error)
	if !ok {
		t.Fatalf("expected *leia.Error, got %T", err)
	}
	if leiaErr.Kind != leia.ErrParse {
		t.Fatalf("expected ErrParse, got %s", leiaErr.Kind)
	}
}

func TestError_runtimeError(t *testing.T) {
	vm := leia.New()
	err := vm.Exec(`x := 1 + "abc"`)
	if err == nil {
		t.Fatal("expected runtime error")
	}
	leiaErr, ok := err.(*leia.Error)
	if !ok {
		t.Fatalf("expected *leia.Error, got %T", err)
	}
	if leiaErr.Kind != leia.ErrRuntime {
		t.Fatalf("expected ErrRuntime, got %s", leiaErr.Kind)
	}
}

// --- Options tests ---

func TestWithPrint(t *testing.T) {
	var captured []string
	vm := leia.New(leia.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		captured = append(captured, strings.Join(parts, " "))
	}))
	vm.Exec(`print("test", 123)`)
	if len(captured) != 1 {
		t.Fatalf("expected 1 captured, got %d", len(captured))
	}
	if captured[0] != "test 123" {
		t.Fatalf("expected 'test 123', got %q", captured[0])
	}
}

func TestWithLibs(t *testing.T) {
	// LibSafe should still work for basic math
	vm := leia.New(leia.WithLibs(leia.LibSafe))
	err := vm.Exec(`x := 1 + 2`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEachPublicLibFlagExposesNamedGlobal(t *testing.T) {
	tests := []struct {
		name   string
		flag   leia.LibFlags
		global string
	}{
		{"bytes", leia.LibBytes, "bytes"},
		{"url", leia.LibURL, "url"},
		{"bits", leia.LibBits, "bits"},
		{"csv", leia.LibCSV, "csv"},
		{"uuid", leia.LibUUID, "uuid"},
		{"matrix", leia.LibMatrix, "matrix"},
		{"compress", leia.LibCompress, "compress"},
		{"container", leia.LibContainer, "container"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(leia.WithLibs(tc.flag))
			if err := vm.Exec(`result := type(` + tc.global + `)`); err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("result")
			if err != nil {
				t.Fatal(err)
			}
			if got != "table" {
				t.Fatalf("type(%s) = %v, want table", tc.global, got)
			}
		})
	}
}

// --- Integration: Go functions called from Leia ---

func TestIntegration_goFuncWithScriptCallback(t *testing.T) {
	var output []string
	vm := leia.New(leia.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))

	vm.RegisterFunc("applyTwice", func(x int64) int64 {
		return x * 2 * 2
	})

	err := vm.Exec(`
		result := applyTwice(5)
		print(result)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0] != "20" {
		t.Fatalf("expected '20', got %v", output)
	}
}
