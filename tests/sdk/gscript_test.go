package gscript_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript"
)

func TestOSEexitReturnsCatchableExitError(t *testing.T) {
	for _, tc := range []struct {
		name string
		vm   *gs.VM
	}{
		{name: "interpreter", vm: gs.New()},
		{name: "bytecode", vm: gs.New(gs.WithVM())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vm.Exec(`os.exit(7)`)
			if err == nil {
				t.Fatal("expected exit error")
			}
			var exitErr *gs.ExitError
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
	vm := gs.New()
	err := vm.Exec(`os.exit(false)`)
	var exitErr *gs.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("exit code = %d, want 1", exitErr.Code)
	}
}

// --- Error handling tests ---

func TestError_parseError(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`func {`)
	if err == nil {
		t.Fatal("expected error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrParse {
		t.Fatalf("expected ErrParse, got %s", gsErr.Kind)
	}
}

func TestError_runtimeError(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`x := 1 + "abc"`)
	if err == nil {
		t.Fatal("expected runtime error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrRuntime {
		t.Fatalf("expected ErrRuntime, got %s", gsErr.Kind)
	}
}

// --- Options tests ---

func TestWithPrint(t *testing.T) {
	var captured []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
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
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	err := vm.Exec(`x := 1 + 2`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEachPublicLibFlagExposesNamedGlobal(t *testing.T) {
	tests := []struct {
		name   string
		flag   gs.LibFlags
		global string
	}{
		{"bytes", gs.LibBytes, "bytes"},
		{"url", gs.LibURL, "url"},
		{"bits", gs.LibBits, "bits"},
		{"csv", gs.LibCSV, "csv"},
		{"uuid", gs.LibUUID, "uuid"},
		{"matrix", gs.LibMatrix, "matrix"},
		{"compress", gs.LibCompress, "compress"},
		{"container", gs.LibContainer, "container"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(gs.WithLibs(tc.flag))
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

// --- Integration: Go functions called from GScript ---

func TestIntegration_goFuncWithScriptCallback(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
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
