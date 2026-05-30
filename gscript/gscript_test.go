package gscript_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gs "github.com/gscript/gscript/gscript"
	"github.com/gscript/gscript/internal/runtime"
)

type hostModuleService struct {
	Prefix string
	count  int64
}

func (s *hostModuleService) Label(id int64) string {
	return fmt.Sprintf("%s-%03d", s.Prefix, id)
}

func (s *hostModuleService) Bump() int64 {
	s.count++
	return s.count
}

// --- Basic VM tests ---

func TestExec(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
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
	prog, err := gs.Compile(`result := 40 + 2`, gs.WithSourceName("calc.gs"))
	if err != nil {
		t.Fatal(err)
	}
	if prog.SourceName() != "calc.gs" {
		t.Fatalf("SourceName = %q, want calc.gs", prog.SourceName())
	}
	vm := gs.New()
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
	prog, err := gs.Compile(`func add(a, b) { return a + b }`)
	if err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM())
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
	mainPath := filepath.Join(dir, "main.gs")
	helperPath := filepath.Join(dir, "helper.gs")
	if err := os.WriteFile(helperPath, []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(`helper := require("helper"); result := helper.value`), 0644); err != nil {
		t.Fatal(err)
	}
	prog, err := gs.CompileFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if prog.SourceName() != mainPath {
		t.Fatalf("SourceName = %q, want %q", prog.SourceName(), mainPath)
	}
	vm := gs.New()
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
	vm := gs.New()
	if err := vm.ExecContext(ctx, `x := 1`); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecContext err = %v, want context.Canceled", err)
	}
	if _, err := gs.CompileContext(ctx, `x := 1`); !errors.Is(err, context.Canceled) {
		t.Fatalf("CompileContext err = %v, want context.Canceled", err)
	}
	if _, err := vm.CallContext(ctx, "missing"); !errors.Is(err, context.Canceled) {
		t.Fatalf("CallContext err = %v, want context.Canceled", err)
	}
}

func TestExecContextCancelsInterpreterLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	vm := gs.New()
	err := vm.ExecContext(ctx, `for {}`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecContext err = %v, want context deadline", err)
	}
}

func TestExecContextCancelsBytecodeLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	vm := gs.New(gs.WithVM())
	err := vm.ExecContext(ctx, `for {}`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecContext err = %v, want context deadline", err)
	}
}

func TestCallContextCancelsRunningFunction(t *testing.T) {
	vm := gs.New()
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

func TestWithMaxStepsLimitsInterpreterExecution(t *testing.T) {
	vm := gs.New(gs.WithMaxSteps(8))
	err := vm.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	if err == nil {
		t.Fatal("expected max step error")
	}
	if !strings.Contains(err.Error(), "execution step limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithMaxStepsLimitsEmptyInterpreterLoop(t *testing.T) {
	vm := gs.New(gs.WithMaxSteps(8))
	err := vm.Exec(`for {}`)
	if err == nil {
		t.Fatal("expected step limit error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxStepsAllowsInterpreterExecutionWithinBudget(t *testing.T) {
	vm := gs.New(gs.WithMaxSteps(64))
	if err := vm.Exec(`
		sum := 0
		for i := 0; i < 5; i++ {
			sum += i
		}
	`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("sum")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(10) {
		t.Fatalf("sum = %v (%T), want int64(10)", got, got)
	}
}

func TestWithMaxNativeCallsLimitsInterpreterHostCalls(t *testing.T) {
	vm := gs.New(gs.WithMaxNativeCalls(3))
	var calls int64
	if err := vm.RegisterFunc("tick", func() int64 {
		calls++
		return calls
	}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		for i := 1; i <= 5; i++ {
			tick()
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 3 {
		t.Fatalf("budget = %s %d, want native_calls 3", budgetErr.Resource, budgetErr.Limit)
	}
	if calls != 3 {
		t.Fatalf("host calls = %d, want 3", calls)
	}
}

func TestWithMaxCallDepthLimitsInterpreterRecursion(t *testing.T) {
	vm := gs.New(gs.WithMaxCallDepth(8))
	err := vm.Exec(`
		func recurse(n) {
			return recurse(n + 1)
		}
		recurse(0)
	`)
	if err == nil {
		t.Fatal("expected call depth budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "call_depth" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want call_depth 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxGoroutinesLimitsInterpreterGoStatements(t *testing.T) {
	vm := gs.New(gs.WithMaxGoroutines(0))
	if err := vm.Exec(`func done() {}; go done()`); err != nil {
		t.Fatal(err)
	}

	limited := gs.New(gs.WithMaxGoroutines(1))
	err := limited.Exec(`
		block := make(chan)
		func worker() { <-block }
		go worker()
		go worker()
	`)
	if err == nil {
		t.Fatal("expected goroutine budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "goroutines" || budgetErr.Limit != 1 {
		t.Fatalf("budget = %s %d, want goroutines 1", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxChannelCapacityLimitsInterpreterMakeChan(t *testing.T) {
	vm := gs.New(gs.WithMaxChannelCapacity(2))
	err := vm.Exec(`ch := make(chan, 3)`)
	if err == nil {
		t.Fatal("expected channel capacity budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "channel_capacity" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want channel_capacity 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsInterpreterHostCallback(t *testing.T) {
	vm := gs.New(gs.WithMaxHostResultBytes(4))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`value := large()`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("budget = %s %d, want host_result_bytes 4", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsInterpreterProcessOutput(t *testing.T) {
	for _, src := range []string{
		`result := process.run("echo hello")`,
		`result := process.exec("echo", "hello")`,
		`result := process.shell("echo hello")`,
	} {
		vm := gs.New(gs.WithLibs(gs.LibString|gs.LibProcess), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxHostResultBytesLimitsInterpreterNetworkResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "12345")
	}))
	defer server.Close()

	for _, src := range []string{
		fmt.Sprintf(`result := net.get(%q)`, server.URL),
		fmt.Sprintf(`result := http.get(%q)`, server.URL),
	} {
		vm := gs.New(gs.WithLibs(gs.LibString|gs.LibNet|gs.LibHTTP), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxModuleBytesLimitsInterpreterRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.gs"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsInterpreterNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.gs"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.gs"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}

func TestWithMaxStepsLimitsBytecodeExecution(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxSteps(8))
	err := vm.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	if err == nil {
		t.Fatal("expected max step error")
	}
	if !strings.Contains(err.Error(), "execution step limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithMaxStepsLimitsEmptyBytecodeLoop(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxSteps(8))
	err := vm.Exec(`for {}`)
	if err == nil {
		t.Fatal("expected step limit error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxNativeCallsLimitsBytecodeHostCalls(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxNativeCalls(3))
	var calls int64
	if err := vm.RegisterFunc("tick", func() int64 {
		calls++
		return calls
	}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		for i := 1; i <= 5; i++ {
			tick()
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 3 {
		t.Fatalf("budget = %s %d, want native_calls 3", budgetErr.Resource, budgetErr.Limit)
	}
	if calls != 3 {
		t.Fatalf("host calls = %d, want 3", calls)
	}
}

func TestWithMaxNativeCallsLimitsBytecodeFastStdlibCalls(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxNativeCalls(2))
	err := vm.Exec(`
		s := "abcdef"
		for i := 1; i <= 4; i++ {
			string.len(s)
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want native_calls 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxCallDepthLimitsBytecodeRecursion(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxCallDepth(8))
	err := vm.Exec(`
		func recurse(n) {
			return recurse(n + 1)
		}
		recurse(0)
	`)
	if err == nil {
		t.Fatal("expected call depth budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "call_depth" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want call_depth 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxGoroutinesLimitsBytecodeGoStatements(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxGoroutines(1))
	err := vm.Exec(`
		block := make(chan)
		func worker() { <-block }
		go worker()
		go worker()
	`)
	if err == nil {
		t.Fatal("expected goroutine budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "goroutines" || budgetErr.Limit != 1 {
		t.Fatalf("budget = %s %d, want goroutines 1", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxChannelCapacityLimitsBytecodeMakeChan(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxChannelCapacity(2))
	err := vm.Exec(`ch := make(chan, 3)`)
	if err == nil {
		t.Fatal("expected channel capacity budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "channel_capacity" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want channel_capacity 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeHostCallback(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxHostResultBytes(4))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`value := large()`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("budget = %s %d, want host_result_bytes 4", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeFastStdlibResult(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxHostResultBytes(4))
	err := vm.Exec(`value := base64.encode("1234")`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("budget = %s %d, want host_result_bytes 4", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesPreflightsEncodingStdlibResults(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := base64.encode("1234")`,
				`value := base64.decode("MTIzNDU=")`,
				`value := encoding.hexEncode("123")`,
				`value := encoding.base32Encode("1234")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibBase64 | gs.LibEncoding),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsCSVEncoding(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := csv.encode({{"12345"}})`,
				`value := csv.encodeWithHeaders({{name: "12345"}}, {"name"})`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibCSV),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsBytesAndBinaryOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`buf := bytes.fromString("12345"); value := buf.toString()`,
				`buf := bytes.fromString("123"); value := buf.toHex()`,
				`value := bytes.toHex("123")`,
				`value := bytes.repeat("12", 3)`,
				`value := bytes.concat("12", "345")`,
				`value := binary.pack("bytes:5", "12345")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibBytes | gs.LibBinary),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsCompressDecodeExpansion(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	blob := buf.String()

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibCompress),
				gs.WithMaxHostResultBytes(4),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Set("blob", blob); err != nil {
				t.Fatal(err)
			}
			err := vm.Exec(`value := compress.gzipDecode(blob)`)
			var budgetErr *gs.BudgetError
			if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
				t.Fatalf("expected host_result_bytes budget 4, got %T %v", err, err)
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeProcessOutput(t *testing.T) {
	for _, src := range []string{
		`result := process.run("echo hello")`,
		`result := process.exec("echo", "hello")`,
		`result := process.shell("echo hello")`,
	} {
		vm := gs.New(gs.WithVM(), gs.WithLibs(gs.LibString|gs.LibProcess), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeNetworkResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "12345")
	}))
	defer server.Close()

	for _, src := range []string{
		fmt.Sprintf(`result := net.get(%q)`, server.URL),
		fmt.Sprintf(`result := http.get(%q)`, server.URL),
	} {
		vm := gs.New(gs.WithVM(), gs.WithLibs(gs.LibString|gs.LibNet|gs.LibHTTP), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxModuleBytesLimitsBytecodeRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.gs"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM(), gs.WithRequirePath(dir), gs.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsBytecodeNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.gs"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.gs"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM(), gs.WithRequirePath(dir), gs.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}

func TestWithMaxStepsAllowsBytecodeExecutionWithinBudget(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxSteps(256))
	if err := vm.Exec(`
		sum := 0
		for i := 0; i < 5; i++ {
			sum += i
		}
	`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("sum")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(10) {
		t.Fatalf("sum = %v (%T), want int64(10)", got, got)
	}
}

func TestExecGoStyleNumberLiteralsWithVM(t *testing.T) {
	vm := gs.New(gs.WithVM())
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
	vm := gs.New()
	err := vm.Exec(`x :=`)
	if err == nil {
		t.Fatal("expected parse error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrParse {
		t.Fatalf("expected ErrParse, got %s", gsErr.Kind)
	}
}

func TestCall(t *testing.T) {
	vm := gs.New()
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
	// GScript int + int returns int
	if results[0] != int64(7) {
		t.Fatalf("expected 7, got %v (%T)", results[0], results[0])
	}
}

func TestCallFunctionRoutesBytecodeClosures(t *testing.T) {
	vm := gs.New(gs.WithVM())
	if err := vm.Exec(`
		func add(a, b) {
			return a + b
		}
	`); err != nil {
		t.Fatal(err)
	}
	fn := vm.GetValue("add")
	if !fn.IsFunction() {
		t.Fatalf("add = %s, want function", fn.TypeName())
	}
	results, err := vm.CallFunction(fn, []runtime.Value{runtime.IntValue(3), runtime.IntValue(4)})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Int() != 7 {
		t.Fatalf("CallFunction results = %v, want 7", results)
	}
}

func TestCallNotFound(t *testing.T) {
	vm := gs.New()
	_, err := vm.Call("nonexistent")
	if err == nil {
		t.Fatal("expected error calling nonexistent function")
	}
}

func TestSetGet(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("x", 42); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if val != int64(42) {
		t.Fatalf("expected 42, got %v (%T)", val, val)
	}
}

func TestSetGet_string(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("name", "gscript"); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("name")
	if err != nil {
		t.Fatal(err)
	}
	if val != "gscript" {
		t.Fatalf("expected 'gscript', got %v", val)
	}
}

func TestSetGet_float(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("pi", 3.14); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("pi")
	if err != nil {
		t.Fatal(err)
	}
	if val != 3.14 {
		t.Fatalf("expected 3.14, got %v", val)
	}
}

func TestSetGet_bool(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("flag", true); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("flag")
	if err != nil {
		t.Fatal(err)
	}
	if val != true {
		t.Fatalf("expected true, got %v", val)
	}
}

func TestSetGet_nil(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("nothing", nil); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("nothing")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil, got %v", val)
	}
}

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

func TestRegisterFunc(t *testing.T) {
	vm := gs.New()
	err := vm.RegisterFunc("square", func(x float64) float64 {
		return x * x
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("square", 5.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] != float64(25) {
		t.Fatalf("expected 25.0, got %v (%T)", results[0], results[0])
	}
}

func TestRegisterFunc_multiReturn(t *testing.T) {
	vm := gs.New()
	err := vm.RegisterFunc("divmod", func(a, b int64) (int64, int64) {
		return a / b, a % b
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("divmod", 17, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != int64(3) || results[1] != int64(2) {
		t.Fatalf("expected [3 2], got %v", results)
	}
}

func TestRegisterFunc_error(t *testing.T) {
	vm := gs.New()
	err := vm.RegisterFunc("fail", func() error {
		return fmt.Errorf("something went wrong")
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = vm.Call("fail")
	if err == nil {
		t.Fatal("expected error from fail()")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterFunc_panicReturnsError(t *testing.T) {
	vm := gs.New()
	err := vm.RegisterFunc("explode", func() int64 {
		panic("boom")
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = vm.Call("explode")
	if err == nil {
		t.Fatal("expected error from panic")
	}
	msg := err.Error()
	if !strings.Contains(msg, "explode") || !strings.Contains(msg, "boom") {
		t.Fatalf("panic error = %q, want function name and panic value", msg)
	}
}

func TestRegisterFunc_panicFromScriptReturnsError(t *testing.T) {
	vm := gs.New()
	err := vm.RegisterFunc("explodeFromScript", func() {
		panic("script boom")
	})
	if err != nil {
		t.Fatal(err)
	}

	err = vm.Exec(`explodeFromScript()`)
	if err == nil {
		t.Fatal("expected error from script-called panic")
	}
	msg := err.Error()
	if !strings.Contains(msg, "explodeFromScript") || !strings.Contains(msg, "script boom") {
		t.Fatalf("panic error = %q, want function name and panic value", msg)
	}
}

func TestRegisterFunc_fromScript(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	vm.RegisterFunc("double", func(x int64) int64 { return x * 2 })
	err := vm.Exec(`print(double(21))`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0] != "42" {
		t.Fatalf("expected '42', got %v", output)
	}
}

func TestRegisterTable(t *testing.T) {
	vm := gs.New()
	err := vm.RegisterTable("mymath", map[string]interface{}{
		"add": func(a, b float64) float64 { return a + b },
		"mul": func(a, b float64) float64 { return a * b },
		"pi":  3.14159,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = vm.Exec(`result := mymath.add(mymath.pi, 1.0)`)
	if err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	expected := 3.14159 + 1.0
	if val != expected {
		t.Fatalf("expected %v, got %v", expected, val)
	}
}

func TestRegisterModuleRequire(t *testing.T) {
	vm := gs.New(gs.WithSandbox())
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
		"join":  strings.Join,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
result := strings.upper("hello")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "HELLO" {
		t.Fatalf("result = %v, want HELLO", val)
	}
}

func TestRegisterModuleFromService(t *testing.T) {
	vm := gs.New(gs.WithSandbox(), gs.WithModuleLoading(false))
	service := &hostModuleService{Prefix: "job"}
	if err := vm.RegisterModuleFrom("go/host", service); err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
host := require("go/host")
label := host.label(7)
prefix := host.prefix
first := host.bump()
second := host.bump()
`); err != nil {
		t.Fatal(err)
	}

	if got, err := vm.Get("label"); err != nil || got != "job-007" {
		t.Fatalf("label = %v, %v; want job-007, nil", got, err)
	}
	if got, err := vm.Get("prefix"); err != nil || got != "job" {
		t.Fatalf("prefix = %v, %v; want job, nil", got, err)
	}
	if got, err := vm.Get("second"); err != nil || got != int64(2) {
		t.Fatalf("second = %v, %v; want 2, nil", got, err)
	}
}

func TestRegisterModuleFromExactNames(t *testing.T) {
	vm := gs.New()
	service := &hostModuleService{Prefix: "task"}
	if err := vm.RegisterModuleFrom("go/exact", service, gs.WithModuleExactNames()); err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
host := require("go/exact")
result := host.Label(3)
`); err != nil {
		t.Fatal(err)
	}
	if got, err := vm.Get("result"); err != nil || got != "task-003" {
		t.Fatalf("result = %v, %v; want task-003, nil", got, err)
	}
}

func TestRegisterModuleRequireBytecodeVM(t *testing.T) {
	vm := gs.New(gs.WithVM())
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
result := strings.upper("vm")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "VM" {
		t.Fatalf("result = %v, want VM", val)
	}
}

func TestRegisterModuleRequireBytecodeVMAfterInit(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithModuleLoading(false))
	if err := vm.Exec(`warmup := true`); err != nil {
		t.Fatal(err)
	}
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
same := package.loaded["go/strings"] == strings
result := strings.upper("late")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "LATE" {
		t.Fatalf("result = %v, want LATE", val)
	}
	same, err := vm.Get("same")
	if err != nil {
		t.Fatal(err)
	}
	if same != true {
		t.Fatalf("same = %v, want true", same)
	}
}

func TestRegisterModuleRequireBytecodeVMNativeRequireAfterInit(t *testing.T) {
	vm := gs.New(gs.WithVM())
	if err := vm.Exec(`warmup := true`); err != nil {
		t.Fatal(err)
	}
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
same := package.loaded["go/strings"] == strings
result := strings.upper("native")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "NATIVE" {
		t.Fatalf("result = %v, want NATIVE", val)
	}
	same, err := vm.Get("same")
	if err != nil {
		t.Fatal(err)
	}
	if same != true {
		t.Fatalf("same = %v, want true", same)
	}
}

// --- Type conversion tests ---

func TestToValue_slice(t *testing.T) {
	vm := gs.New()
	err := vm.Set("arr", []int{10, 20, 30})
	if err != nil {
		t.Fatal(err)
	}
	// GScript: arr is a 1-based table
	err = vm.Exec(`result := arr[1] + arr[2] + arr[3]`)
	if err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != int64(60) {
		t.Fatalf("expected 60, got %v", val)
	}
}

func TestToValue_map(t *testing.T) {
	vm := gs.New()
	err := vm.Set("data", map[string]interface{}{
		"name": "test",
		"val":  42,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = vm.Exec(`name := data.name`)
	if err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("name")
	if err != nil {
		t.Fatal(err)
	}
	if val != "test" {
		t.Fatalf("expected 'test', got %v", val)
	}
}

func TestToValue_func(t *testing.T) {
	vm := gs.New()
	err := vm.Set("greet", func(name string) string {
		return "Hello, " + name + "!"
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("greet", "world")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got %v", results)
	}
}

// --- Struct binding tests ---

type Vec2 struct {
	X, Y float64
}

func (v Vec2) Length() float64     { return math.Sqrt(v.X*v.X + v.Y*v.Y) }
func (v Vec2) Add(other Vec2) Vec2 { return Vec2{v.X + other.X, v.Y + other.Y} }
func (v *Vec2) Scale(f float64)    { v.X *= f; v.Y *= f }
func (v Vec2) String() string      { return fmt.Sprintf("Vec2(%g, %g)", v.X, v.Y) }

func TestBindStruct_new(t *testing.T) {
	vm := gs.New()
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`v := Vec2.new(3, 4)`)
	if err != nil {
		t.Fatal(err)
	}
	// v should be a table wrapping Vec2{3, 4}
	val, err := vm.Get("v")
	if err != nil {
		t.Fatal(err)
	}
	if val == nil {
		t.Fatal("expected non-nil value for v")
	}
}

func TestBindStruct_fieldAccess(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		v := Vec2.new(3, 4)
		print(v.X)
		print(v.Y)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 {
		t.Fatalf("expected 2 outputs, got %d: %v", len(output), output)
	}
	if output[0] != "3.0" && output[0] != "3" {
		t.Fatalf("expected X=3, got %q", output[0])
	}
	if output[1] != "4.0" && output[1] != "4" {
		t.Fatalf("expected Y=4, got %q", output[1])
	}
}

func TestBindStruct_fieldSet(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		v := Vec2.new(3, 4)
		v.X = 10
		print(v.X)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 output, got %d: %v", len(output), output)
	}
	if output[0] != "10.0" && output[0] != "10" {
		t.Fatalf("expected X=10, got %q", output[0])
	}
}

func TestBindStruct_methodCall(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		v := Vec2.new(3, 4)
		print(v.Length())
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 output, got %d: %v", len(output), output)
	}
	if output[0] != "5.0" && output[0] != "5" {
		t.Fatalf("expected Length()=5, got %q", output[0])
	}
}

func TestBindStruct_returnStruct(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		a := Vec2.new(1, 2)
		b := Vec2.new(3, 4)
		c := a.Add(b)
		print(c.X)
		print(c.Y)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 {
		t.Fatalf("expected 2 outputs, got %d: %v", len(output), output)
	}
	if output[0] != "4.0" && output[0] != "4" {
		t.Fatalf("expected c.X=4, got %q", output[0])
	}
	if output[1] != "6.0" && output[1] != "6" {
		t.Fatalf("expected c.Y=6, got %q", output[1])
	}
}

func TestBindStructWithConstructor(t *testing.T) {
	vm := gs.New()

	type Player struct {
		Name  string
		HP    int
		Level int
	}

	if err := vm.BindStructWithConstructor("Player", Player{}, func(name string) Player {
		return Player{Name: name, HP: 100, Level: 1}
	}); err != nil {
		t.Fatal(err)
	}

	var output []string
	vm = gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStructWithConstructor("Player", Player{}, func(name string) Player {
		return Player{Name: name, HP: 100, Level: 1}
	}); err != nil {
		t.Fatal(err)
	}

	err := vm.Exec(`
		p := Player.new("Alice")
		print(p.Name)
		print(p.HP)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 {
		t.Fatalf("expected 2 outputs, got %d: %v", len(output), output)
	}
	if output[0] != "Alice" {
		t.Fatalf("expected 'Alice', got %q", output[0])
	}
	if output[1] != "100" {
		t.Fatalf("expected '100', got %q", output[1])
	}
}

// --- Pool tests ---

func TestPool(t *testing.T) {
	pool := gs.NewPool(5, func() *gs.VM {
		return gs.New()
	})

	vm := pool.Get()
	if vm == nil {
		t.Fatal("expected non-nil VM")
	}
	pool.Put(vm)
	if pool.Size() != 1 {
		t.Fatalf("expected pool size 1, got %d", pool.Size())
	}

	// Get should reuse
	vm2 := pool.Get()
	if vm2 == nil {
		t.Fatal("expected non-nil VM")
	}
	if pool.Size() != 0 {
		t.Fatalf("expected pool size 0, got %d", pool.Size())
	}
}

func TestPool_concurrent(t *testing.T) {
	pool := gs.NewPool(10, func() *gs.VM {
		vm := gs.New()
		vm.RegisterFunc("square", func(x int64) int64 { return x * x })
		return vm
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := pool.Do(func(vm *gs.VM) error {
				results, err := vm.Call("square", int64(n))
				if err != nil {
					return err
				}
				expected := int64(n) * int64(n)
				if results[0] != expected {
					return fmt.Errorf("expected %d^2=%d, got %v", n, expected, results[0])
				}
				return nil
			})
			if err != nil {
				t.Errorf("goroutine %d: %v", n, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestPool_Do(t *testing.T) {
	pool := gs.NewPool(2, func() *gs.VM {
		vm := gs.New()
		vm.RegisterFunc("inc", func(x int64) int64 { return x + 1 })
		return vm
	})

	err := pool.Do(func(vm *gs.VM) error {
		results, err := vm.Call("inc", 41)
		if err != nil {
			return err
		}
		if results[0] != int64(42) {
			return fmt.Errorf("expected 42, got %v", results[0])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// VM should be returned to pool
	if pool.Size() != 1 {
		t.Fatalf("expected pool size 1 after Do, got %d", pool.Size())
	}
}

func TestPoolPreservesStateByDefault(t *testing.T) {
	pool := gs.NewPool(1, func() *gs.VM {
		return gs.New()
	})

	vm := pool.Get()
	if err := vm.Set("x", int64(42)); err != nil {
		t.Fatal(err)
	}
	pool.Put(vm)

	reused := pool.Get()
	got, err := reused.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("x = %v (%T), want int64(42)", got, got)
	}
}

func TestPoolWithResetClearsGlobalsBeforeReuse(t *testing.T) {
	pool := gs.NewPoolWithReset(1, func() *gs.VM {
		return gs.New()
	}, func(vm *gs.VM) bool {
		vm.Reset()
		return true
	})

	vm := pool.Get()
	if err := vm.Set("x", int64(42)); err != nil {
		t.Fatal(err)
	}
	pool.Put(vm)

	reused := pool.Get()
	got, err := reused.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("x = %v (%T), want nil after reset", got, got)
	}
}

func TestPoolWithResetCanDiscardVM(t *testing.T) {
	created := 0
	pool := gs.NewPoolWithReset(1, func() *gs.VM {
		created++
		vm := gs.New()
		if err := vm.Set("id", int64(created)); err != nil {
			t.Fatal(err)
		}
		return vm
	}, func(vm *gs.VM) bool {
		return false
	})

	first := pool.Get()
	pool.Put(first)
	if pool.Size() != 0 {
		t.Fatalf("expected discarded VM to leave pool empty, got size %d", pool.Size())
	}
	second := pool.Get()
	got, err := second.Get("id")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(2) {
		t.Fatalf("id = %v (%T), want int64(2)", got, got)
	}
}

func TestVMResetClearsGlobalsAndModuleCache(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "helper.gs")
	if err := os.WriteFile(modPath, []byte(`return { value: 1 }`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithRequirePath(dir))
	if err := vm.Exec(`helper := require("helper"); extra := 99`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modPath, []byte(`return { value: 2 }`), 0644); err != nil {
		t.Fatal(err)
	}

	vm.Reset()
	if got, err := vm.Get("extra"); err != nil {
		t.Fatal(err)
	} else if got != nil {
		t.Fatalf("extra = %v (%T), want nil after reset", got, got)
	}
	if err := vm.Exec(`helper := require("helper"); result := helper.value`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(2) {
		t.Fatalf("result = %v (%T), want int64(2)", got, got)
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

func TestWithLibsRestrictsUnsafeGlobals(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	err := vm.Exec(`
		hasMath := type(math)
		hasJSON := type(json)
		hasBytes := type(bytes)
		hasURL := type(url)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hasMath", "hasJSON", "hasBytes", "hasURL"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != "table" {
			t.Fatalf("%s = %v, want table", name, got)
		}
	}
	for _, name := range []string{"io", "os", "fs", "net", "http", "process", "script", "debug", "testkit"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithLibsRestrictsBytecodeVM(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibSafe), gs.WithVM())
	err := vm.Exec(`
		hasString := type(string)
		hasBytes := type(bytes)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hasString", "hasBytes"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != "table" {
			t.Fatalf("%s = %v, want table", name, got)
		}
	}
	for _, name := range []string{"http", "debug", "testkit"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithSandboxDisablesFilesystemCapabilities(t *testing.T) {
	vm := gs.New(gs.WithSandbox())
	if err := vm.Exec(`hasJSON := type(json)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fs", "dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
	got, err := vm.Get("hasJSON")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("hasJSON = %v, want table", got)
	}
}

func TestSecuritySandboxDisablesHostCapabilitiesAndJIT(t *testing.T) {
	vm := gs.New(gs.WithJIT(), gs.SecuritySandbox(), gs.WithMaxSteps(16))
	if err := vm.Exec(`hasJSON := type(require("json"))`); err != nil {
		t.Fatalf("safe stdlib should remain available: %v", err)
	}
	for _, src := range []string{
		`fs.readfile("x")`,
		`os.getenv("PATH")`,
		`process.pid()`,
		`require("helper")`,
	} {
		if err := vm.Exec(src); err == nil {
			t.Fatalf("SecuritySandbox allowed %s", src)
		}
	}
	err := vm.Exec(`for {}`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected step budget in sandboxed loop, got %T %v", err, err)
	}
	if err := vm.Exec(`fn, loadErr := load("x := 1")`); err != nil {
		t.Fatal(err)
	}
	loadErr, err := vm.Get("loadErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := loadErr.(string); !ok || !strings.Contains(msg, "dynamic eval disabled") {
		t.Fatalf("loadErr = %v, want dynamic eval disabled", loadErr)
	}
}

func TestWithDynamicEvalFalseBlocksScriptStringCompilation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithDynamicEval(false)}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`fn, loadErr := load("x := 1")`); err != nil {
				t.Fatal(err)
			}
			fn, err := vm.Get("fn")
			if err != nil {
				t.Fatal(err)
			}
			if fn != nil {
				t.Fatalf("fn = %v, want nil", fn)
			}
			loadErr, err := vm.Get("loadErr")
			if err != nil {
				t.Fatal(err)
			}
			if msg, ok := loadErr.(string); !ok || !strings.Contains(msg, "dynamic eval disabled") {
				t.Fatalf("loadErr = %v, want dynamic eval disabled", loadErr)
			}
			err = vm.Exec(`script.eval("x := 1")`)
			if err == nil || !strings.Contains(err.Error(), "dynamic eval disabled") {
				t.Fatalf("script.eval err = %v, want dynamic eval disabled", err)
			}
		})
	}
}

func TestWithSecurityAppliesSandboxAndBudgets(t *testing.T) {
	vm := gs.New(gs.WithJIT(), gs.WithSecurity(gs.SecurityPolicy{
		Libs:                    gs.LibSafe,
		Capabilities:            gs.CapSafe,
		DisableModuleLoading:    true,
		DisableJIT:              true,
		MaxSteps:                32,
		MaxNativeCalls:          4,
		MaxCallDepth:            8,
		MaxGoroutines:           1,
		MaxChannelCapacity:      2,
		MaxHostResultBytes:      4,
		MaxModuleBytes:          128,
		MaxModuleDepth:          1,
		MaxFilesystemReadBytes:  128,
		MaxFilesystemWriteBytes: 128,
		EnvironmentAllowlist:    []string{"GSCRIPT_PUBLIC_ENV_CAP_TEST"},
		DisableDynamicEval:      true,
		DisableNetworkAccess:    true,
		DisableDebugAccess:      true,
		DisableTestkitAccess:    true,
		DisableProcessExecution: true,
		DisableProcessShell:     true,
	}))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	if got, err := vm.Get("json"); err != nil || got == nil {
		t.Fatalf("safe stdlib should remain available: got=%v err=%v", got, err)
	}
	if err := vm.Exec(`fs.readfile("x")`); err == nil {
		t.Fatal("WithSecurity allowed filesystem API")
	}
	err := vm.Exec(`value := large()`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected host_result_bytes budget 4, got %T %v", err, err)
	}
	err = vm.Exec(`for {}`)
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "steps" || budgetErr.Limit != 32 {
		t.Fatalf("expected steps budget 32, got %T %v", err, err)
	}
}

func TestEnvironmentCapabilities(t *testing.T) {
	t.Setenv("GSCRIPT_PUBLIC_ENV_CAP_TEST", "visible")

	tests := []struct {
		name    string
		opts    []gs.Option
		src     string
		wantErr string
	}{
		{
			name:    "environment disabled blocks getenv",
			opts:    []gs.Option{gs.WithEnvironment(false)},
			src:     `value := os.getenv("GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "read disabled blocks expand",
			opts:    []gs.Option{gs.WithEnvironmentRead(false)},
			src:     `value := os.expand("$GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "write disabled blocks setenv",
			opts:    []gs.Option{gs.WithEnvironmentWrite(false)},
			src:     `ok := os.setenv("GSCRIPT_PUBLIC_ENV_WRITE_TEST", "blocked")`,
			wantErr: "environment write access disabled",
		},
		{
			name: "read only still reads",
			opts: []gs.Option{gs.WithEnvironmentWrite(false)},
			src:  `value := os.getenv("GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(tc.opts...)
			err := vm.Exec(tc.src)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Exec error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("value")
			if err != nil {
				t.Fatal(err)
			}
			if got != "visible" {
				t.Fatalf("value = %v, want visible", got)
			}
		})
	}
}

func TestWithProcessShellFalseBlocksShell(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithProcessShell(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.shell("echo blocked")`)
			if err == nil || !strings.Contains(err.Error(), "process shell access disabled") {
				t.Fatalf("process.shell err = %v, want process shell access disabled", err)
			}
		})
	}
}

func TestWithProcessExecutionFalseBlocksRunExecAndWhich(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithProcessExecution(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`result := process.run("echo blocked")`,
				`result := process.exec("echo", "blocked")`,
				`result := process.which("echo")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "process execution access disabled") {
					t.Fatalf("%s err = %v, want process execution access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemRootConfinesProcessRunDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := gs.New(opts...)
			src := fmt.Sprintf(`result := process.run({"pwd"}, {dir: %q})`, outside)
			err := vm.Exec(src)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("process.run dir escape err = %v, want filesystem access denied", err)
			}
		})
	}
}

func TestProcessRunEnvFollowsEnvironmentPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name+"/write-disabled", func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithEnvironmentWrite(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.run({"pwd"}, {env: {GSCRIPT_PROCESS_ENV_POLICY_TEST: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment write access disabled") {
				t.Fatalf("process.run env err = %v, want environment write access disabled", err)
			}
		})

		t.Run(tc.name+"/allowlist", func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithEnvironmentAllowlist("GSCRIPT_PROCESS_ENV_ALLOWED"),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.run({"pwd"}, {env: {GSCRIPT_PROCESS_ENV_BLOCKED: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed: GSCRIPT_PROCESS_ENV_BLOCKED") {
				t.Fatalf("process.run env allowlist err = %v, want environment variable not allowed", err)
			}
		})
	}
}

func TestWithNetworkAccessFalseBlocksNetAndHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibNet | gs.LibHTTP),
				gs.WithNetworkAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`resp := net.get("http://127.0.0.1:1")`,
				`resp := net.request({url: "http://127.0.0.1:1"})`,
				`resp := http.get("http://127.0.0.1:1")`,
				`server := http.listen("127.0.0.1:0", func(req, res) {}, {background: true})`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "network access disabled") {
					t.Fatalf("%s err = %v, want network access disabled", src, err)
				}
			}
		})
	}
}

func TestWithDebugAccessFalseBlocksDebugAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibDebug),
				gs.WithDebugAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`stack := debug.stack()`,
				`globals := debug.globals()`,
				`raw := debug.goStack()`,
				`debug.setHook(func(event) {})`,
				`debug.emit("blocked")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "debug access disabled") {
					t.Fatalf("%s err = %v, want debug access disabled", src, err)
				}
			}
		})
	}
}

func TestWithTestkitAccessFalseBlocksTestkitAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibTestkit),
				gs.WithTestkitAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`stats := testkit.memory()`,
				`info := testkit.value(42)`,
				`kind := testkit.typeOf(42)`,
				`result := testkit.protect(func() { return 1 })`,
				`same := testkit.sameFunction(print, print)`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "testkit access disabled") {
					t.Fatalf("%s err = %v, want testkit access disabled", src, err)
				}
			}
		})
	}
}

func TestEnvironmentAllowlist(t *testing.T) {
	t.Setenv("GSCRIPT_PUBLIC_ENV_ALLOWED", "visible")
	t.Setenv("GSCRIPT_PUBLIC_ENV_BLOCKED", "secret")

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithEnvironmentAllowlist("GSCRIPT_PUBLIC_ENV_ALLOWED")}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
				allowed := os.getenv("GSCRIPT_PUBLIC_ENV_ALLOWED")
				blocked := os.getenv("GSCRIPT_PUBLIC_ENV_BLOCKED")
				expanded := os.expand("$GSCRIPT_PUBLIC_ENV_ALLOWED:$GSCRIPT_PUBLIC_ENV_BLOCKED")
				all := os.environ()
				procEnv := process.env()
			`); err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]interface{}{
				"allowed":  "visible",
				"blocked":  nil,
				"expanded": "visible:",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("%s = %v, want %v", name, got, want)
				}
			}
			for _, tableName := range []string{"all", "procEnv"} {
				got, err := vm.Get(tableName)
				if err != nil {
					t.Fatal(err)
				}
				env, ok := got.(map[string]interface{})
				if !ok {
					t.Fatalf("%s = %T, want map", tableName, got)
				}
				if env["GSCRIPT_PUBLIC_ENV_ALLOWED"] != "visible" {
					t.Fatalf("%s allowed = %v, want visible", tableName, env["GSCRIPT_PUBLIC_ENV_ALLOWED"])
				}
				if _, ok := env["GSCRIPT_PUBLIC_ENV_BLOCKED"]; ok {
					t.Fatalf("%s exposed blocked environment variable", tableName)
				}
			}
			err := vm.Exec(`os.setenv("GSCRIPT_PUBLIC_ENV_BLOCKED", "changed")`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed") {
				t.Fatalf("setenv blocked err = %v, want environment variable not allowed", err)
			}
		})
	}
}

func TestWithModuleLoadingFalseAllowsStdlibRequireButBlocksFileModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.gs"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithModuleLoading(false))
	if err := vm.Exec(`result := type(require("json"))`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("stdlib require result = %v, want table", got)
	}
	err = vm.Exec(`require("helper")`)
	if err == nil || !strings.Contains(err.Error(), "module loading disabled") {
		t.Fatalf("require helper error = %v, want module loading disabled", err)
	}
}

func TestWithModuleLoadingFalseRestrictsBytecodeVM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.gs"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithModuleLoading(false), gs.WithVM())
	if err := vm.Exec(`stdlibResult := type(require("json"))`); err != nil {
		t.Fatalf("stdlib require should still work with module loading disabled: %v", err)
	}
	got, err := vm.Get("stdlibResult")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("stdlibResult = %v, want table", got)
	}
	err = vm.Exec(`require("helper")`)
	if err == nil {
		t.Fatal("expected require to fail when module loading is disabled")
	}
}

func TestWithFilesystemFalseRemovesFilesystemGlobals(t *testing.T) {
	vm := gs.New(gs.WithFilesystem(false))
	for _, name := range []string{"fs", "dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithFilesystemFalseClearsRootEnabledFilesystem(t *testing.T) {
	root := t.TempDir()
	vm := gs.New(gs.WithFilesystemRoot(root), gs.WithFilesystem(false))
	for _, name := range []string{"fs", "dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithFilesystemWriteFalseBlocksOSFileMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibOS),
				gs.WithFilesystemWrite(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`ok := os.remove("blocked.txt")`,
				`ok := os.rename("old.txt", "new.txt")`,
				`name := os.tmpname()`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
					t.Fatalf("%s err = %v, want filesystem write access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemCapabilitiesGateIOLibFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "in.txt"), []byte("hello"), 0644); err != nil {
				t.Fatal(err)
			}
			readOnly := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibIO),
				gs.WithFilesystemRoot(root),
				gs.WithFilesystemWrite(false),
			}, tc.opts...)
			vm := gs.New(readOnly...)
			if err := vm.Exec(`f := io.open("in.txt", "r"); data := f:read("a"); f:close()`); err != nil {
				t.Fatalf("io.open read in read-only filesystem failed: %v", err)
			}
			got, err := vm.Get("data")
			if err != nil {
				t.Fatal(err)
			}
			if got != "hello" {
				t.Fatalf("data = %v, want hello", got)
			}
			for _, src := range []string{
				`f := io.open("out.txt", "w")`,
				`io.output("out.txt")`,
				`tmp := io.tmpfile()`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
					t.Fatalf("%s err = %v, want filesystem write access disabled", src, err)
				}
			}

			writeOnly := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibIO),
				gs.WithFilesystemRoot(root),
				gs.WithFilesystemRead(false),
			}, tc.opts...)
			vm = gs.New(writeOnly...)
			if err := vm.Exec(`f := io.open("out.txt", "w"); f:write("ok"); f:close()`); err != nil {
				t.Fatalf("io.open write in write-only filesystem failed: %v", err)
			}
			for _, src := range []string{
				`f := io.open("in.txt", "r")`,
				`iter := io.lines("in.txt")`,
				`io.input("in.txt")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "filesystem read access disabled") {
					t.Fatalf("%s err = %v, want filesystem read access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemRootConfinesIOLibFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibIO),
				gs.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
				f := io.open("inside.txt", "w")
				assert(f:write("ok"))
				assert(f:close())
			`); err != nil {
				t.Fatalf("io.open inside root failed: %v", err)
			}
			if got, err := os.ReadFile(filepath.Join(root, "inside.txt")); err != nil || string(got) != "ok" {
				t.Fatalf("inside file = %q err=%v, want ok", got, err)
			}
			err := vm.Exec(`f := io.open("../escape.txt", "w")`)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("io.open escape err = %v, want filesystem access denied", err)
			}
			if err := vm.Exec(`
				tmp := io.tmpfile()
				assert(tmp:write("x"))
				assert(tmp:close())
			`); err != nil {
				t.Fatalf("io.tmpfile in root failed: %v", err)
			}
		})
	}
}

func TestWithFilesystemRootConfinesOSFileMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("ok"), 0644); err != nil {
				t.Fatal(err)
			}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibOS),
				gs.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`ok := os.rename("old.txt", "new.txt")`); err != nil {
				t.Fatalf("os.rename inside root failed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "new.txt")); err != nil {
				t.Fatalf("renamed file missing: %v", err)
			}
			err := vm.Exec(`ok := os.remove("../escape.txt")`)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("os.remove escape err = %v, want filesystem access denied", err)
			}
			if err := vm.Exec(`name := os.tmpname()`); err != nil {
				t.Fatalf("os.tmpname in root failed: %v", err)
			}
			got, err := vm.Get("name")
			if err != nil {
				t.Fatal(err)
			}
			name, ok := got.(string)
			if !ok || !strings.HasPrefix(name, root+string(os.PathSeparator)) {
				t.Fatalf("tmpname = %v, want path inside %s", got, root)
			}
			_ = os.Remove(name)
		})
	}
}

func TestWithFilesystemReadOnlyAllowsReadAndBlocksWrite(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemWrite(false))
	if err := vm.Exec(`content := fs.readfile("inside.txt")`); err != nil {
		t.Fatalf("readfile with read-only filesystem failed: %v", err)
	}
	content, err := vm.Get("content")
	if err != nil {
		t.Fatal(err)
	}
	if content != "inside" {
		t.Fatalf("content = %v, want inside", content)
	}
	err = vm.Exec(`fs.writefile("new.txt", "new")`)
	if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
		t.Fatalf("writefile error = %v, want write access disabled", err)
	}
}

func TestWithFilesystemRootReadOnlyAllowsFileLoadsAndConfinesReads(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.gs"), []byte(`loaded := "inside"`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "outside.txt"), []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemWrite(false))
	if err := vm.Exec(`
		dofile("inside.gs")
		inside, insideErr := fs.readfile("inside.txt")
		outside, outsideErr := fs.readfile("../outside.txt")
	`); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]interface{}{
		"loaded":    "inside",
		"inside":    "inside",
		"insideErr": nil,
		"outside":   nil,
	} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s = %v (%T), want %v", name, got, got, want)
		}
	}
	outsideErr, err := vm.Get("outsideErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := outsideErr.(string); !ok || !strings.Contains(msg, "escapes root") {
		t.Fatalf("outsideErr = %v, want escapes root string", outsideErr)
	}
	err = vm.Exec(`fs.writefile("blocked.txt", "blocked")`)
	if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
		t.Fatalf("writefile error = %v, want write access disabled", err)
	}
	loadfile, err := vm.Get("loadfile")
	if err != nil {
		t.Fatal(err)
	}
	if got := publicAPIType(loadfile); got != "function" {
		t.Fatalf("loadfile type = %v, want function", got)
	}
}

func TestWithFilesystemWriteOnlyAllowsWriteAndBlocksRead(t *testing.T) {
	root := t.TempDir()
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemRead(false))

	if err := vm.Exec(`ok := fs.writefile("out.txt", "out")`); err != nil {
		t.Fatalf("writefile with write-only filesystem failed: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "out.txt")); err != nil || string(got) != "out" {
		t.Fatalf("host file = %q, %v; want out, nil", string(got), err)
	}
	for _, name := range []string{"dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
	err := vm.Exec(`fs.readfile("out.txt")`)
	if err == nil || !strings.Contains(err.Error(), "filesystem read access disabled") {
		t.Fatalf("readfile error = %v, want read access disabled", err)
	}
}

func TestWithFilesystemRootWriteOnlyConfinesWritesAndRemovesFileLoads(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemRead(false))
	if err := vm.Exec(`
		insideOK, insideErr := fs.writefile("inside.txt", "inside")
		outsideOK, outsideErr := fs.writefile("../outside.txt", "outside")
	`); err != nil {
		t.Fatal(err)
	}
	insideOK, err := vm.Get("insideOK")
	if err != nil {
		t.Fatal(err)
	}
	if insideOK != true {
		t.Fatalf("insideOK = %v, want true", insideOK)
	}
	insideErr, err := vm.Get("insideErr")
	if err != nil {
		t.Fatal(err)
	}
	if insideErr != nil {
		t.Fatalf("insideErr = %v, want nil", insideErr)
	}
	if got, err := os.ReadFile(filepath.Join(root, "inside.txt")); err != nil || string(got) != "inside" {
		t.Fatalf("host file = %q, %v; want inside, nil", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(base, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside file stat err = %v, want not exist", err)
	}
	outsideOK, err := vm.Get("outsideOK")
	if err != nil {
		t.Fatal(err)
	}
	if outsideOK != nil {
		t.Fatalf("outsideOK = %v, want nil", outsideOK)
	}
	outsideErr, err := vm.Get("outsideErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := outsideErr.(string); !ok || !strings.Contains(msg, "escapes root") {
		t.Fatalf("outsideErr = %v, want escapes root string", outsideErr)
	}
	for _, name := range []string{"dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestFilesystemReadCapabilityControlsBytecodeFileLoadGlobals(t *testing.T) {
	tests := []struct {
		name      string
		opts      []gs.Option
		wantFS    string
		wantFiles string
	}{
		{
			name:      "filesystem disabled",
			opts:      []gs.Option{gs.WithFilesystem(false)},
			wantFS:    "nil",
			wantFiles: "nil",
		},
		{
			name:      "read only",
			opts:      []gs.Option{gs.WithFilesystemWrite(false)},
			wantFS:    "table",
			wantFiles: "function",
		},
		{
			name:      "write only",
			opts:      []gs.Option{gs.WithFilesystemRead(false)},
			wantFS:    "table",
			wantFiles: "nil",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithVM()}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`probe := true`); err != nil {
				t.Fatal(err)
			}
			gotFS, err := vm.Get("fs")
			if err != nil {
				t.Fatal(err)
			}
			if got := publicAPIType(gotFS); got != tc.wantFS {
				t.Fatalf("fs type = %v, want %s", got, tc.wantFS)
			}
			for _, name := range []string{"dofile", "loadfile"} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatal(err)
				}
				if gotType := publicAPIType(got); gotType != tc.wantFiles {
					t.Fatalf("%s type = %v, want %s", name, gotType, tc.wantFiles)
				}
			}
		})
	}
}

func TestMaxFilesystemReadBytesLimitsFSReadFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte("12345"), 0644); err != nil {
				t.Fatal(err)
			}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibFS),
				gs.WithFilesystemRoot(root),
				gs.WithMaxFilesystemReadBytes(4),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`content, readErr := fs.readfile("big.txt")`); err != nil {
				t.Fatal(err)
			}
			content, err := vm.Get("content")
			if err != nil {
				t.Fatal(err)
			}
			if content != nil {
				t.Fatalf("content = %v, want nil", content)
			}
			readErr, err := vm.Get("readErr")
			if err != nil {
				t.Fatal(err)
			}
			if msg, ok := readErr.(string); !ok || !strings.Contains(msg, "filesystem read byte limit exceeded (4)") {
				t.Fatalf("readErr = %v, want read byte budget string", readErr)
			}
		})
	}
}

func TestMaxFilesystemWriteBytesLimitsFSWriteFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibFS),
				gs.WithFilesystemRoot(root),
				gs.WithMaxFilesystemWriteBytes(4),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`ok, writeErr := fs.writefile("big.txt", "12345")`); err != nil {
				t.Fatal(err)
			}
			ok, err := vm.Get("ok")
			if err != nil {
				t.Fatal(err)
			}
			if ok != nil {
				t.Fatalf("ok = %v, want nil", ok)
			}
			writeErr, err := vm.Get("writeErr")
			if err != nil {
				t.Fatal(err)
			}
			if msg, ok := writeErr.(string); !ok || !strings.Contains(msg, "filesystem write byte limit exceeded (4)") {
				t.Fatalf("writeErr = %v, want write byte budget string", writeErr)
			}
			if _, err := os.Stat(filepath.Join(root, "big.txt")); !os.IsNotExist(err) {
				t.Fatalf("big.txt stat err = %v, want not exist", err)
			}
		})
	}
}

func publicAPIType(v interface{}) string {
	if v == nil {
		return "nil"
	}
	if val, ok := v.(runtime.Value); ok && val.IsFunction() {
		return "function"
	}
	if _, ok := v.(map[string]interface{}); ok {
		return "table"
	}
	return fmt.Sprintf("%T", v)
}

func TestWithFilesystemRootConfinesFSModule(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "outside.txt"), []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root))
	if err := vm.Exec(`
		inside, insideErr := fs.readfile("inside.txt")
		outside, outsideErr := fs.readfile("../outside.txt")
	`); err != nil {
		t.Fatal(err)
	}
	inside, err := vm.Get("inside")
	if err != nil {
		t.Fatal(err)
	}
	if inside != "inside" {
		t.Fatalf("inside = %v, want inside", inside)
	}
	outside, err := vm.Get("outside")
	if err != nil {
		t.Fatal(err)
	}
	if outside != nil {
		t.Fatalf("outside = %v, want nil", outside)
	}
	outsideErr, err := vm.Get("outsideErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := outsideErr.(string); !ok || !strings.Contains(msg, "escapes root") {
		t.Fatalf("outsideErr = %v, want escapes root string", outsideErr)
	}
}

func TestWithFilesystemRootConfinesBytecodeRequire(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "outside.gs"), []byte(`return { value: 99 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(root), gs.WithFilesystemRoot(root), gs.WithVM())
	err := vm.Exec(`require("../outside")`)
	if err == nil || !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("require outside error = %v, want escapes root", err)
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
		{"rl", gs.LibRL, "rl"},
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
