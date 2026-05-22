//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

func TestCompileTier2RecordsProductionGlobalDependency(t *testing.T) {
	top := compileTop(t, `
limit := 7
func add_limit(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + limit
    }
    return s
}
`)
	globals := runtime.NewInterpreterGlobals()
	v := vm.New(globals)
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("execute top: %v", err)
	}
	proto := findProtoByName(top, "add_limit")
	if proto == nil {
		t.Fatal("add_limit proto not found")
	}
	fn := v.GetGlobal("add_limit")
	if _, err := v.CallValue(fn, []runtime.Value{runtime.IntValue(2)}); err != nil {
		t.Fatalf("warm add_limit: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(add_limit): %v", err)
	}
	compiled := tm.tier2CompiledSnapshot()[proto]
	if compiled == nil {
		t.Fatal("add_limit did not produce a Tier 2 compiled function")
	}
	if len(compiled.CompilationDependencies) == 0 {
		t.Fatal("Tier 2 compile recorded no compilation dependencies")
	}
	if invalid := compiled.ValidateCompilationDependencies(CompilationDependencyContext{Globals: v}); invalid != nil {
		t.Fatalf("recorded production dependencies are stale immediately after compile: %v", invalid)
	}
	foundGlobal := false
	for _, dep := range compiled.CompilationDependencies {
		if dep.Kind() == CompilationDependencyGlobal && dep.Key() == "global=limit" {
			foundGlobal = true
			break
		}
	}
	if !foundGlobal {
		t.Fatalf("dependencies = %v, want global=limit", compiled.CompilationDependencies)
	}
}
