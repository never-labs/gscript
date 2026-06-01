package modules

import (
	"os"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/stdlibrt"
)

func execOSProgram(t *testing.T, interp *Interpreter, src string) error {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return err
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return err
	}
	return interp.Exec(prog)
}

func callOSFunction(t *testing.T, lib *Table, name string, args ...Value) ([]Value, error) {
	t.Helper()
	fn := lib.RawGetString(name)
	if !fn.IsFunction() {
		t.Fatalf("os.%s is %s, want function", name, fn.TypeName())
	}
	return fn.GoFunction().Fn(args)
}

func newCoreWithOSModule() *Interpreter {
	interp := New()
	osModule := TableValue(BuildOSWithPolicy(stdlibrt.HostOptions{
		EnvironmentRead:  interp.EnvironmentReadEnabled,
		EnvironmentWrite: interp.EnvironmentWriteEnabled,
		EnvironmentAllowed: func(name string) bool {
			return interp.EnvironmentAllowed(name)
		},
		FilesystemRoot:  interp.FilesystemRoot,
		FilesystemWrite: interp.FilesystemWriteEnabled,
	}))
	interp.SetGlobal("os", osModule)
	interp.SetModule("os", osModule)
	return interp
}

func TestOSEnvironReturnsEnvironmentTable(t *testing.T) {
	t.Setenv("LEIA_OS_ENVIRON_TEST", "present")

	results, err := callOSFunction(t, BuildOS(), "environ")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].IsTable() {
		t.Fatalf("os.environ returned %v, want table", results)
	}
	got := results[0].Table().RawGetString("LEIA_OS_ENVIRON_TEST")
	if !got.IsString() || got.Str() != "present" {
		t.Fatalf("LEIA_OS_ENVIRON_TEST = %v, want present", got)
	}
}

func TestOSEnvironmentReadCapabilityRequired(t *testing.T) {
	t.Setenv("LEIA_OS_READ_DISABLED_TEST", "secret")
	interp := newCoreWithOSModule()
	interp.SetEnvironmentCapabilities(false, true)

	for _, tc := range []struct {
		name string
		src  string
	}{
		{name: "getenv", src: `x := os.getenv("LEIA_OS_READ_DISABLED_TEST")`},
		{name: "environ", src: `x := os.environ()`},
		{name: "expand", src: `x := os.expand("$LEIA_OS_READ_DISABLED_TEST")`},
	} {
		err := execOSProgram(t, interp, tc.src)
		if err == nil || !strings.Contains(err.Error(), "environment read access disabled") {
			t.Fatalf("%s error = %v, want environment read access disabled", tc.name, err)
		}
	}

	if err := execOSProgram(t, interp, `ok := os.setenv("LEIA_OS_READ_DISABLED_WRITE_OK", "yes")`); err != nil {
		t.Fatalf("setenv with write capability enabled failed: %v", err)
	}
	if got := os.Getenv("LEIA_OS_READ_DISABLED_WRITE_OK"); got != "yes" {
		t.Fatalf("LEIA_OS_READ_DISABLED_WRITE_OK = %q, want yes", got)
	}
	t.Cleanup(func() { _ = os.Unsetenv("LEIA_OS_READ_DISABLED_WRITE_OK") })
}

func TestOSEnvironmentWriteCapabilityRequired(t *testing.T) {
	t.Setenv("LEIA_OS_WRITE_DISABLED_TEST", "visible")
	interp := newCoreWithOSModule()
	interp.SetEnvironmentCapabilities(true, false)

	if err := execOSProgram(t, interp, `x := os.getenv("LEIA_OS_WRITE_DISABLED_TEST")`); err != nil {
		t.Fatalf("getenv with read capability enabled failed: %v", err)
	}
	if got := interp.GetGlobal("x"); !got.IsString() || got.Str() != "visible" {
		t.Fatalf("x = %v, want visible", got)
	}

	for _, tc := range []struct {
		name string
		src  string
	}{
		{name: "setenv", src: `ok := os.setenv("LEIA_OS_WRITE_DISABLED_SET", "no")`},
		{name: "unsetenv", src: `ok := os.unsetenv("LEIA_OS_WRITE_DISABLED_TEST")`},
	} {
		err := execOSProgram(t, interp, tc.src)
		if err == nil || !strings.Contains(err.Error(), "environment write access disabled") {
			t.Fatalf("%s error = %v, want environment write access disabled", tc.name, err)
		}
	}
}
