package leia_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestWithMaxHostResultBytesLimitsInterpreterHostCallback(t *testing.T) {
	vm := leia.New(leia.WithMaxHostResultBytes(4))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`value := large()`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *leia.BudgetError
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
		vm := leia.New(leia.WithLibs(leia.LibString|leia.LibProcess), leia.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *leia.BudgetError
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
		vm := leia.New(leia.WithLibs(leia.LibString|leia.LibNet|leia.LibHTTP), leia.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *leia.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeHostCallback(t *testing.T) {
	vm := leia.New(leia.WithVM(), leia.WithMaxHostResultBytes(4))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`value := large()`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("budget = %s %d, want host_result_bytes 4", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeFastStdlibResult(t *testing.T) {
	vm := leia.New(leia.WithVM(), leia.WithMaxHostResultBytes(4))
	err := vm.Exec(`value := base64.encode("1234")`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := base64.encode("1234")`,
				`value := base64.decode("MTIzNDU=")`,
				`value := base64.urlEncode("1234")`,
				`value := base64.urlDecode("MTIzNDU")`,
				`value, err := base64.decode("!!!!!!!!")`,
				`value, err := base64.urlDecode("!!!!!!!!")`,
				`value := encoding.hexEncode("123")`,
				`value := encoding.base32Encode("1234")`,
			} {
				opts := append([]leia.Option{
					leia.WithLibs(leia.LibString | leia.LibBase64 | leia.LibEncoding),
					leia.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := leia.New(opts...)
				err := vm.Exec(src)
				var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := csv.encode({{"12345"}})`,
				`value := csv.encodeWithHeaders({{name: "12345"}}, {"name"})`,
				`value := dialect.eval("csv", {{"12345"}}, {mode: "encode"})`,
				`value := dialect.eval("tsv", {{"12345"}}, {mode: "encode"})`,
			} {
				opts := append([]leia.Option{
					leia.WithLibs(leia.LibString | leia.LibCSV | leia.LibDialect),
					leia.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := leia.New(opts...)
				err := vm.Exec(src)
				var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
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
				opts := append([]leia.Option{
					leia.WithLibs(leia.LibString | leia.LibBytes | leia.LibBinary),
					leia.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := leia.New(opts...)
				err := vm.Exec(src)
				var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := crypto.randomBytes(5)`,
				`value := crypto.randomHex(3)`,
				`value := crypto.generateKey(16)`,
				`key := "1234567890123456"; value := crypto.aesGcmEncrypt(key, "x")`,
			} {
				opts := append([]leia.Option{
					leia.WithLibs(leia.LibString | leia.LibCrypto),
					leia.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := leia.New(opts...)
				err := vm.Exec(src)
				var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
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
				opts := append([]leia.Option{
					leia.WithLibs(leia.LibString | leia.LibURL),
					leia.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := leia.New(opts...)
				err := vm.Exec(src)
				var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
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
				opts := append([]leia.Option{
					leia.WithLibs(leia.LibString | leia.LibUTF8),
					leia.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := leia.New(opts...)
				err := vm.Exec(src)
				var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
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
				opts := append([]leia.Option{
					leia.WithLibs(leia.LibString),
					leia.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := leia.New(opts...)
				err := vm.Exec(src)
				var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibCompress),
				leia.WithMaxHostResultBytes(4),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Set("blob", blob); err != nil {
				t.Fatal(err)
			}
			err := vm.Exec(`value := compress.gzipDecode(blob)`)
			var budgetErr *leia.BudgetError
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
		vm := leia.New(leia.WithVM(), leia.WithLibs(leia.LibString|leia.LibProcess), leia.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *leia.BudgetError
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
		vm := leia.New(leia.WithVM(), leia.WithLibs(leia.LibString|leia.LibNet|leia.LibHTTP), leia.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *leia.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}
