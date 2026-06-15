package leia_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestPublicSDKRecommendedAPISignaturesHideInternalRuntime(t *testing.T) {
	vmType := reflect.TypeOf((*leia.VM)(nil))
	for _, name := range []string{
		"Exec",
		"ExecContext",
		"ExecFile",
		"ExecFileContext",
		"Run",
		"RunContext",
		"Call",
		"CallContext",
		"CallValue",
		"CallValueContext",
		"Set",
		"Get",
		"GetPublicValue",
		"SetPublicValue",
		"CallPublicValue",
		"RegisterFunc",
		"RegisterTable",
		"RegisterModule",
		"RegisterModuleFrom",
		"Reset",
		"SetArgs",
	} {
		method, ok := vmType.MethodByName(name)
		if !ok {
			t.Fatalf("VM.%s missing", name)
		}
		assertNoInternalRuntimeType(t, "VM."+name, method.Type)
	}

	for name, fn := range map[string]interface{}{
		"New":                         leia.New,
		"Compile":                     leia.Compile,
		"CompileFile":                 leia.CompileFile,
		"Decode":                      leia.Decode,
		"Encode":                      leia.Encode,
		"Nil":                         leia.Nil,
		"Bool":                        leia.Bool,
		"Int":                         leia.Int,
		"Float":                       leia.Float,
		"String":                      leia.String,
		"WithDialect":                 leia.WithDialect,
		"WithSandbox":                 leia.WithSandbox,
		"SecuritySandbox":             leia.SecuritySandbox,
		"WithSecurity":                leia.WithSecurity,
		"WithMaxSteps":                leia.WithMaxSteps,
		"WithMaxNativeCalls":          leia.WithMaxNativeCalls,
		"WithMaxCallDepth":            leia.WithMaxCallDepth,
		"WithMaxGoroutines":           leia.WithMaxGoroutines,
		"WithMaxChannelCapacity":      leia.WithMaxChannelCapacity,
		"WithMaxHostResultBytes":      leia.WithMaxHostResultBytes,
		"WithMaxModuleBytes":          leia.WithMaxModuleBytes,
		"WithMaxModuleDepth":          leia.WithMaxModuleDepth,
		"WithMaxFilesystemReadBytes":  leia.WithMaxFilesystemReadBytes,
		"WithMaxFilesystemWriteBytes": leia.WithMaxFilesystemWriteBytes,
		"NewHotLoader":                leia.NewHotLoader,
	} {
		assertNoInternalRuntimeType(t, name, reflect.TypeOf(fn))
	}
	assertNoInternalRuntimeType(t, "DialectHandler", reflect.TypeOf((*leia.DialectHandler)(nil)).Elem())
	assertNoInternalRuntimeType(t, "SecurityPolicy", reflect.TypeOf(leia.SecurityPolicy{}))
	assertNoInternalRuntimeType(t, "BudgetError", reflect.TypeOf((*leia.BudgetError)(nil)).Elem())
	assertNoInternalRuntimeType(t, "HotLoader", reflect.TypeOf((*leia.HotLoader)(nil)).Elem())
	assertNoInternalRuntimeType(t, "HotInstance", reflect.TypeOf((*leia.HotInstance)(nil)).Elem())
}

func TestPublicSDKRawRuntimeMethodsAreNotExported(t *testing.T) {
	vmType := reflect.TypeOf((*leia.VM)(nil))
	for _, name := range []string{
		"GetValue",
		"SetValue",
		"CallFunction",
		"Interpreter",
	} {
		if _, ok := vmType.MethodByName(name); ok {
			t.Fatalf("VM.%s should not be exported from the public root SDK", name)
		}
	}

}

func TestRootPackageGoDocHidesRawRuntimeValues(t *testing.T) {
	out := rootGoDoc(t)
	for _, forbidden := range []string{
		"runtime.Value",
		"runtime.Interpreter",
		"type LLMMessage = runtime.",
		"type LLMProviderConfig = runtime.",
		"type LLMTool = runtime.",
		"type LLMToolCall = runtime.",
		"type LLMTraceEvent = runtime.",
		"type LLMTurnRequest = runtime.",
		"type LLMTurnResult = runtime.",
		"type LLMTurnUsage = runtime.",
		"LLMProviderErrorNetwork = runtime.",
		"func ToValue",
		"func MustToValue",
		"func FromValue",
		"func ToPublicValue",
		"func MustToPublicValue",
		"func FromPublicValue",
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("root package go doc exposes %q:\n%s", forbidden, out)
		}
	}
}

func TestRootPackageLLMTypesStayFacadeAliases(t *testing.T) {
	files := parseRootPackageFiles(t)

	for filename, file := range files {
		imports := importAliases(file)
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok || !ast.IsExported(spec.Name.Name) || !strings.Contains(spec.Name.Name, "LLM") {
				return true
			}
			if isTypeAliasToLLMSubpackage(spec, imports) {
				return true
			}
			t.Fatalf("%s defines root LLM type %s; put concrete LLM provider/replay/trace/helper types under github.com/never-labs/leia/llm/... and keep root as a facade alias", filepath.Base(filename), spec.Name.Name)
			return true
		})
	}
}

func TestRootPackageLLMFacadeDoesNotReimplementHostedHelpers(t *testing.T) {
	files := parseRootPackageFiles(t)

	for filename, file := range files {
		if !strings.HasPrefix(filepath.Base(filename), "llm") {
			continue
		}
		for _, importPath := range importAliases(file) {
			switch importPath {
			case "bufio",
				"bytes",
				"encoding/json",
				"io",
				"net/http",
				"os",
				"os/exec",
				"time":
				t.Fatalf("%s imports %s; root LLM files should stay facade-only and delegate implementation to github.com/never-labs/leia/llm/...", filepath.Base(filename), importPath)
			}
		}
	}
}

func TestPublicValueBoundaryWorksWithoutRawRuntimeTypes(t *testing.T) {
	vm := leia.New(leia.WithVM())

	limit := leia.Int(40)
	if limit.Kind() != leia.KindInt || limit.Int() != 40 {
		t.Fatalf("limit = %s/%d, want int/40", limit.Kind(), limit.Int())
	}
	if err := vm.Set("limit", limit); err != nil {
		t.Fatal(err)
	}
	if err := vm.Exec(`
		config := {label: "answer", values: [limit, 2]}
		func add(a, b) { return a + b }
		result := add(config.values[1], config.values[2])
	`); err != nil {
		t.Fatal(err)
	}

	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}

	encoded, err := leia.String("answer").Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "answer" {
		t.Fatalf("encoded string = %v (%T), want answer", encoded, encoded)
	}

	decoded, err := leia.Decode(map[string]interface{}{
		"label": "answer",
		"count": 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Kind() != leia.KindTable {
		t.Fatalf("decoded kind = %s, want table", decoded.Kind())
	}
	roundTrip, err := leia.Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := roundTrip.(map[string]interface{})
	if !ok {
		t.Fatalf("roundTrip = %T, want map[string]interface{}", roundTrip)
	}
	if m["label"] != "answer" || m["count"] != int64(42) {
		t.Fatalf("roundTrip = %#v, want label/count fields", m)
	}
}

func TestPublicValueCanCallHostAndScriptBoundaries(t *testing.T) {
	vm := leia.New(leia.WithVM())
	if err := vm.RegisterFunc("scale", func(v leia.Value) int64 {
		if v.Kind() != leia.KindInt {
			t.Fatalf("scale arg kind = %s, want int", v.Kind())
		}
		return v.Int() * 2
	}); err != nil {
		t.Fatal(err)
	}
	if err := vm.Exec(`
		func apply(fn, value) {
			return fn(value)
		}
		result := apply(scale, 21)
	`); err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("apply", func(v leia.Value) int64 {
		if v.Kind() != leia.KindInt {
			t.Fatalf("apply arg kind = %s, want int", v.Kind())
		}
		return v.Int() + 1
	}, leia.Int(41))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != int64(42) {
		t.Fatalf("apply result = %#v, want [42]", results)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}
}

func rootGoDoc(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "doc", ".")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go doc .: %v\n%s", err, out)
	}
	return string(out)
}

func assertNoInternalRuntimeType(t *testing.T, name string, typ reflect.Type) {
	t.Helper()
	signature := typ.String()
	if strings.Contains(signature, "/internal/runtime.") || strings.Contains(signature, "runtime.Value") || strings.Contains(signature, "runtime.Interpreter") {
		t.Fatalf("%s exposes internal runtime in recommended API signature: %s", name, signature)
	}
}

func parseRootPackageFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, repoRoot(t), func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse root package: %v", err)
	}
	files, ok := pkgs["leia"]
	if !ok {
		t.Fatal("root package leia not found")
	}
	return files.Files
}

func importAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, spec := range file.Imports {
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		} else {
			parts := strings.Split(strings.Trim(spec.Path.Value, `"`), "/")
			name = parts[len(parts)-1]
		}
		aliases[name] = strings.Trim(spec.Path.Value, `"`)
	}
	return aliases
}

func isTypeAliasToLLMSubpackage(spec *ast.TypeSpec, imports map[string]string) bool {
	if spec.Assign == 0 {
		return false
	}
	switch typ := spec.Type.(type) {
	case *ast.SelectorExpr:
		ident, ok := typ.X.(*ast.Ident)
		if !ok {
			return false
		}
		path := imports[ident.Name]
		return path == "github.com/never-labs/leia/llm" || strings.HasPrefix(path, "github.com/never-labs/leia/llm/")
	case *ast.Ident:
		return typ.Name == "LLMProvider"
	default:
		return false
	}
}
