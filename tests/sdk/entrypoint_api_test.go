package leia_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
)

func TestExec(t *testing.T) {
	var output []string
	vm := leia.New(leia.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	err := vm.Exec(`print("hello", "world")`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0] != "hello\tworld" {
		t.Fatalf("expected 'hello\\tworld', got %v", output)
	}
}

func TestCompileRunProgram(t *testing.T) {
	prog, err := leia.Compile(`result := 40 + 2`, leia.WithSourceName("calc.leia"))
	if err != nil {
		t.Fatal(err)
	}
	if prog.SourceName() != "calc.leia" {
		t.Fatalf("SourceName = %q, want calc.leia", prog.SourceName())
	}
	vm := leia.New()
	if err := vm.Run(prog); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}
}

func TestCompileRunProgramWithVM(t *testing.T) {
	prog, err := leia.Compile(`func add(a, b) { return a + b }`)
	if err != nil {
		t.Fatal(err)
	}
	vm := leia.New(leia.WithVM())
	if err := vm.Run(prog); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Call("add", 20, 22)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != int64(42) {
		t.Fatalf("add result = %v, want [42]", got)
	}
}

func TestCompileFileSetsSourceAndRequireDir(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.leia")
	helperPath := filepath.Join(dir, "helper.leia")
	if err := os.WriteFile(helperPath, []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(`helper := require("helper"); result := helper.value`), 0644); err != nil {
		t.Fatal(err)
	}
	prog, err := leia.CompileFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if prog.SourceName() != mainPath {
		t.Fatalf("SourceName = %q, want %q", prog.SourceName(), mainPath)
	}
	vm := leia.New()
	if err := vm.Run(prog); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}
}

func TestContextEntrypointsRespectCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	vm := leia.New()
	if err := vm.ExecContext(ctx, `x := 1`); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecContext err = %v, want context.Canceled", err)
	}
	if _, err := leia.CompileContext(ctx, `x := 1`); !errors.Is(err, context.Canceled) {
		t.Fatalf("CompileContext err = %v, want context.Canceled", err)
	}
	if _, err := vm.CallContext(ctx, "missing"); !errors.Is(err, context.Canceled) {
		t.Fatalf("CallContext err = %v, want context.Canceled", err)
	}
}

func TestExecContextCancelsInterpreterLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	vm := leia.New()
	err := vm.ExecContext(ctx, `for {}`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecContext err = %v, want context deadline", err)
	}
}

func TestExecContextCancelsBytecodeLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	vm := leia.New(leia.WithVM())
	err := vm.ExecContext(ctx, `for {}`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecContext err = %v, want context deadline", err)
	}
}

func TestCallContextCancelsRunningFunction(t *testing.T) {
	vm := leia.New()
	if err := vm.Exec(`func spin() { for {} }`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := vm.CallContext(ctx, "spin")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CallContext err = %v, want context deadline", err)
	}
}

func TestExecGoStyleNumberLiteralsWithVM(t *testing.T) {
	vm := leia.New(leia.WithVM())
	if err := vm.Exec(`result := 0xFF + 0b1010 + 0o20 + 1_000`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(1281) {
		t.Fatalf("expected 1281, got %v (%T)", got, got)
	}
}

func TestExecError(t *testing.T) {
	vm := leia.New()
	err := vm.Exec(`x :=`)
	if err == nil {
		t.Fatal("expected parse error")
	}
	gsErr, ok := err.(*leia.Error)
	if !ok {
		t.Fatalf("expected *leia.Error, got %T", err)
	}
	if gsErr.Kind != leia.ErrParse {
		t.Fatalf("expected ErrParse, got %s", gsErr.Kind)
	}
}

func TestCall(t *testing.T) {
	vm := leia.New()
	err := vm.Exec(`
		func add(a, b) {
			return a + b
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("add", 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Leia int + int returns int
	if results[0] != int64(7) {
		t.Fatalf("expected 7, got %v (%T)", results[0], results[0])
	}
}

func TestCallPublicValueRoutesBytecodeClosures(t *testing.T) {
	vm := leia.New(leia.WithVM())
	if err := vm.Exec(`
		func add(a, b) {
			return a + b
		}
	`); err != nil {
		t.Fatal(err)
	}
	fn := vm.GetPublicValue("add")
	if got := fn.Kind(); got != leia.KindFunction {
		t.Fatalf("add kind = %s, want function", got)
	}
	results, err := vm.CallPublicValue(fn, leia.Int(3), leia.Int(4))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Int() != 7 {
		t.Fatalf("CallPublicValue results = %v, want 7", results)
	}
}

func TestPublicValueGlobals(t *testing.T) {
	vm := leia.New(leia.WithVM())
	vm.SetPublicValue("answer", leia.Int(42))
	if err := vm.Exec(`answer = answer + 1`); err != nil {
		t.Fatal(err)
	}
	got := vm.GetPublicValue("answer")
	if got.Int() != 43 {
		t.Fatalf("answer = %d, want 43", got.Int())
	}
}

func TestCallNotFound(t *testing.T) {
	vm := leia.New()
	_, err := vm.Call("nonexistent")
	if err == nil {
		t.Fatal("expected error calling nonexistent function")
	}
}
