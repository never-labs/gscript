package modules

import (
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/runtime"
)

type Interpreter = runtime.Interpreter
type Value = runtime.Value

var (
	New         = runtime.NewCore
	StringValue = runtime.StringValue
	TableValue  = runtime.TableValue
)

func runProgram(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	installTestModules(interp)
	execOnInterp(t, interp, src)
	return interp
}

func runWithLib(t *testing.T, src string, libName string, lib *runtime.Table) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	interp.SetGlobal(libName, runtime.TableValue(lib))
	interp.SetModule(libName, runtime.TableValue(lib))
	execOnInterp(t, interp, src)
	return interp
}

func execOnInterp(t *testing.T, interp *runtime.Interpreter, src string) {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := interp.Exec(prog); err != nil {
		t.Fatalf("exec error: %v", err)
	}
}

func installTestModules(interp *runtime.Interpreter) {
	installTestModule(interp, "base64", runtime.TableValue(BuildBase64(interp.MaxHostResultBytes)))
	installTestModule(interp, "bits", runtime.TableValue(BuildBits()))
	installTestModule(interp, "hash", runtime.TableValue(BuildHash()))
	installTestModule(interp, "path", runtime.TableValue(BuildPath()))
	installTestModule(interp, "uuid", runtime.TableValue(BuildUUID()))
}

func installTestModule(interp *runtime.Interpreter, name string, module runtime.Value) {
	interp.SetGlobal(name, module)
	interp.SetModule(name, module)
}
