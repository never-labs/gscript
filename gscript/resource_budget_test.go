package gscript_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

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

func TestWithMaxHostResultBytesLimitsCryptoOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := crypto.randomBytes(5)`,
				`value := crypto.randomHex(3)`,
				`value := crypto.generateKey(16)`,
				`key := "1234567890123456"; value := crypto.aesGcmEncrypt(key, "x")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibCrypto),
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

func TestWithMaxHostResultBytesLimitsURLOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := url.encode("12345")`,
				`value := url.decode("12345")`,
				`value := url.build({scheme: "https", host: "example.com"})`,
				`value := url.queryEncode({name: "12345"})`,
				`value := url.join("https://example.com/", "12345")`,
				`value := url.getHost("https://example.com/path")`,
				`value := url.getPath("https://example.com/12345")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibURL),
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

func TestWithMaxHostResultBytesLimitsUTF8Output(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := utf8.char(49, 50, 51, 52, 53)`,
				`value := utf8.sanitize("12345")`,
				`value := utf8.reverse("12345")`,
				`value := utf8.sub("12345", 1, 5)`,
				`value := utf8.upper("abcde")`,
				`value := utf8.lower("ABCDE")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibUTF8),
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

func TestWithMaxHostResultBytesPreflightsStringOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := string.char(49, 50, 51, 52, 53)`,
				`value := string.rep("12", 3)`,
				`value := string.rep("1", 3, "-")`,
				`value := string.repeat("12", 3)`,
				`value := string.join({"12", "345"}, "")`,
				`value := string.padLeft("1", 5, "0")`,
				`value := string.padRight("1", 5, "0")`,
				`value := string.pack("bytes:5", "12345")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString),
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
