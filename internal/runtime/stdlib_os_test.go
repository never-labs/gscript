package runtime

import (
	"os"
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
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

func TestOSEnvironReturnsEnvironmentTable(t *testing.T) {
	t.Setenv("GSCRIPT_OS_ENVIRON_TEST", "present")

	results, err := callOSFunction(t, buildOSLib(), "environ")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].IsTable() {
		t.Fatalf("os.environ returned %v, want table", results)
	}
	got := results[0].Table().RawGetString("GSCRIPT_OS_ENVIRON_TEST")
	if !got.IsString() || got.Str() != "present" {
		t.Fatalf("GSCRIPT_OS_ENVIRON_TEST = %v, want present", got)
	}
}

func TestOSEnvironmentReadCapabilityRequired(t *testing.T) {
	t.Setenv("GSCRIPT_OS_READ_DISABLED_TEST", "secret")
	interp := newCoreWithTableModule("os", buildOSLib())
	interp.SetEnvironmentCapabilities(false, true)

	for _, tc := range []struct {
		name string
		src  string
	}{
		{name: "getenv", src: `x := os.getenv("GSCRIPT_OS_READ_DISABLED_TEST")`},
		{name: "environ", src: `x := os.environ()`},
		{name: "expand", src: `x := os.expand("$GSCRIPT_OS_READ_DISABLED_TEST")`},
	} {
		err := execOSProgram(t, interp, tc.src)
		if err == nil || !strings.Contains(err.Error(), "environment read access disabled") {
			t.Fatalf("%s error = %v, want environment read access disabled", tc.name, err)
		}
	}

	if err := execOSProgram(t, interp, `ok := os.setenv("GSCRIPT_OS_READ_DISABLED_WRITE_OK", "yes")`); err != nil {
		t.Fatalf("setenv with write capability enabled failed: %v", err)
	}
	if got := os.Getenv("GSCRIPT_OS_READ_DISABLED_WRITE_OK"); got != "yes" {
		t.Fatalf("GSCRIPT_OS_READ_DISABLED_WRITE_OK = %q, want yes", got)
	}
	t.Cleanup(func() { _ = os.Unsetenv("GSCRIPT_OS_READ_DISABLED_WRITE_OK") })
}

func TestOSEnvironmentWriteCapabilityRequired(t *testing.T) {
	t.Setenv("GSCRIPT_OS_WRITE_DISABLED_TEST", "visible")
	interp := newCoreWithTableModule("os", buildOSLib())
	interp.SetEnvironmentCapabilities(true, false)

	if err := execOSProgram(t, interp, `x := os.getenv("GSCRIPT_OS_WRITE_DISABLED_TEST")`); err != nil {
		t.Fatalf("getenv with read capability enabled failed: %v", err)
	}
	if got := interp.GetGlobal("x"); !got.IsString() || got.Str() != "visible" {
		t.Fatalf("x = %v, want visible", got)
	}

	for _, tc := range []struct {
		name string
		src  string
	}{
		{name: "setenv", src: `ok := os.setenv("GSCRIPT_OS_WRITE_DISABLED_SET", "no")`},
		{name: "unsetenv", src: `ok := os.unsetenv("GSCRIPT_OS_WRITE_DISABLED_TEST")`},
	} {
		err := execOSProgram(t, interp, tc.src)
		if err == nil || !strings.Contains(err.Error(), "environment write access disabled") {
			t.Fatalf("%s error = %v, want environment write access disabled", tc.name, err)
		}
	}
}
