package gscript_test

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

	gs "github.com/never-labs/gscript"
)

func TestPublicSDKRecommendedAPISignaturesHideInternalRuntime(t *testing.T) {
	vmType := reflect.TypeOf((*gs.VM)(nil))
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
		"New":         gs.New,
		"Compile":     gs.Compile,
		"CompileFile": gs.CompileFile,
		"Decode":      gs.Decode,
		"Encode":      gs.Encode,
		"Nil":         gs.Nil,
		"Bool":        gs.Bool,
		"Int":         gs.Int,
		"Float":       gs.Float,
		"String":      gs.String,
	} {
		assertNoInternalRuntimeType(t, name, reflect.TypeOf(fn))
	}
}

func TestPublicSDKRawRuntimeMethodsAreNotExported(t *testing.T) {
	vmType := reflect.TypeOf((*gs.VM)(nil))
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

func TestRootPackageLLMProviderImplementationsStayOutOfRoot(t *testing.T) {
	files := parseRootPackageFiles(t)
	allowedRootConcrete := map[string]bool{
		// Existing compatibility providers are still in the root package while
		// they are migrated. New providers should live under llm/... and be
		// exposed from root only through compatibility aliases or constructors.
		"AnthropicCompatibleLLMProvider": true,
		"OpenAICompatibleLLMProvider":    true,
	}
	seenConcrete := map[string]bool{}

	for filename, file := range files {
		imports := importAliases(file)
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok || !ast.IsExported(spec.Name.Name) || !strings.Contains(spec.Name.Name, "LLMProvider") {
				return true
			}
			if isTypeAliasToLLMSubpackage(spec, imports) {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); ok {
				if allowedRootConcrete[spec.Name.Name] {
					seenConcrete[spec.Name.Name] = true
					return true
				}
				t.Fatalf("%s defines root LLM provider implementation %s; add provider implementations under github.com/never-labs/gscript/llm/... and keep root as a facade", filepath.Base(filename), spec.Name.Name)
			}
			return true
		})
	}

	for name := range allowedRootConcrete {
		if !seenConcrete[name] {
			t.Fatalf("root concrete provider allowlist contains %s, but it no longer exists; remove the allowlist entry", name)
		}
	}
}

func TestPublicValueBoundaryWorksWithoutRawRuntimeTypes(t *testing.T) {
	vm := gs.New(gs.WithVM())

	limit := gs.Int(40)
	if limit.Kind() != gs.KindInt || limit.Int() != 40 {
		t.Fatalf("limit = %s/%d, want int/40", limit.Kind(), limit.Int())
	}
	if err := vm.Set("limit", limit); err != nil {
		t.Fatal(err)
	}
	if err := vm.Exec(`
		config := {label: "answer", values: {limit, 2}}
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

	encoded, err := gs.String("answer").Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "answer" {
		t.Fatalf("encoded string = %v (%T), want answer", encoded, encoded)
	}

	decoded, err := gs.Decode(map[string]interface{}{
		"label": "answer",
		"count": 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Kind() != gs.KindTable {
		t.Fatalf("decoded kind = %s, want table", decoded.Kind())
	}
	roundTrip, err := gs.Encode(decoded)
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
	vm := gs.New(gs.WithVM())
	if err := vm.RegisterFunc("scale", func(v gs.Value) int64 {
		if v.Kind() != gs.KindInt {
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
	results, err := vm.Call("apply", func(v gs.Value) int64 {
		if v.Kind() != gs.KindInt {
			t.Fatalf("apply arg kind = %s, want int", v.Kind())
		}
		return v.Int() + 1
	}, gs.Int(41))
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
	files, ok := pkgs["gscript"]
	if !ok {
		t.Fatal("root package gscript not found")
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
		return path == "github.com/never-labs/gscript/llm" || strings.HasPrefix(path, "github.com/never-labs/gscript/llm/")
	case *ast.Ident:
		return typ.Name == "LLMProvider"
	default:
		return false
	}
}
