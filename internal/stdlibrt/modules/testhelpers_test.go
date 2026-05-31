package modules

import (
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/runtime"
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
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "base64", runtime.TableValue(BuildBase64(interp.MaxHostResultBytes)))
	installTestModule(interp, "bits", runtime.TableValue(BuildBits()))
	installTestModule(interp, "compress", runtime.TableValue(BuildCompress(interp.MaxHostResultBytes)))
	installTestModule(interp, "crypto", runtime.TableValue(BuildCrypto(interp.MaxHostResultBytes)))
	installTestModule(interp, "csv", runtime.TableValue(BuildCSV(interp.MaxHostResultBytes)))
	installTestModule(interp, "encoding", runtime.TableValue(BuildEncoding(interp.MaxHostResultBytes)))
	installTestModule(interp, "hash", runtime.TableValue(BuildHash()))
	installTestModule(interp, "path", runtime.TableValue(BuildPath()))
	installTestModule(interp, "rand", runtime.TableValue(BuildRand()))
	installTestModule(interp, "regexp", runtime.TableValue(BuildRegexp()))
	installTestModule(interp, "url", runtime.TableValue(BuildURL(interp.MaxHostResultBytes)))
	installTestModule(interp, "uuid", runtime.TableValue(BuildUUID()))
	installTestModule(interp, "vec", runtime.TableValue(BuildVec()))
}

func installTestModule(interp *runtime.Interpreter, name string, module runtime.Value) {
	interp.SetGlobal(name, module)
	interp.SetModule(name, module)
}
