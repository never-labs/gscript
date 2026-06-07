package vm

import (
	"testing"

	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
)

func TestQSQLFastArg2UsesVMDirectNativeFastCall(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals, v := compileAndRunQSQLFastNativeFixture(t, `
trades := data.frame({
    sym: data.symbols({"AAPL", "MSFT"}),
    price: data.f64({100.0, 80.0}),
})

rows := q.sql(trades, "select sym,price from trades where price>=90")
row_count := #rows
first_sym := rows[1].sym
first_price := rows[1].price
`)
	defer v.Close()

	for _, name := range []string{"array", "data", "q"} {
		if got := v.GetGlobal(name); !got.IsTable() {
			t.Fatalf("VM global %s = %s, want installed module table", name, got.String())
		}
	}
	if got := globals["row_count"]; !got.IsInt() || got.Int() != 1 {
		t.Fatalf("row_count = %s, want 1", got.String())
	}
	if got := globals["first_sym"]; !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("first_sym = %s, want AAPL", got.String())
	}
	if got := globals["first_price"]; !got.IsNumber() || got.Float() != 100 {
		t.Fatalf("first_price = %s, want 100", got.String())
	}

	var qsqlFast uint64
	for _, entry := range stats.Snapshot().NativeCall.PerBuiltin {
		if entry.Name == "q.sql" {
			qsqlFast = entry.Fast
			break
		}
	}
	if qsqlFast == 0 {
		t.Fatalf("q.sql fast native stats missing from NativeCall.PerBuiltin: %#v", stats.Snapshot().NativeCall.PerBuiltin)
	}
}

func TestQSQLThreeArgumentEnvUsesVMNativeFallback(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals, v := compileAndRunQSQLFastNativeFixture(t, `
trades := data.frame({
    sym: data.symbols({"AAPL", "MSFT", "NVDA"}),
    price: data.f64({100.0, 80.0, 150.0}),
})

rows := q.sql(trades, "select sym,price from trades where price>=threshold order by price asc", {threshold: 90.0})
row_count := #rows
first_sym := rows[1].sym
first_price := rows[1].price
second_sym := rows[2].sym
second_price := rows[2].price
`)
	defer v.Close()

	if got := globals["row_count"]; !got.IsInt() || got.Int() != 2 {
		t.Fatalf("row_count = %s, want 2", got.String())
	}
	if got := globals["first_sym"]; !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("first_sym = %s, want AAPL", got.String())
	}
	if got := globals["first_price"]; !got.IsNumber() || got.Float() != 100 {
		t.Fatalf("first_price = %s, want 100", got.String())
	}
	if got := globals["second_sym"]; !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("second_sym = %s, want NVDA", got.String())
	}
	if got := globals["second_price"]; !got.IsNumber() || got.Float() != 150 {
		t.Fatalf("second_price = %s, want 150", got.String())
	}

	qsqlFast, qsqlFallback := nativeCallBuiltinCounts(stats, "q.sql")
	if qsqlFast != 0 {
		t.Fatalf("q.sql fast native hits = %d, want 0 for three-argument env call", qsqlFast)
	}
	if qsqlFallback == 0 {
		t.Fatalf("q.sql fallback native stats missing from NativeCall.PerBuiltin: %#v", stats.Snapshot().NativeCall.PerBuiltin)
	}
}

func nativeCallBuiltinCounts(stats *runtime.RuntimePathStats, name string) (fast, fallback uint64) {
	for _, entry := range stats.Snapshot().NativeCall.PerBuiltin {
		if entry.Name == name {
			return entry.Fast, entry.Fallback
		}
	}
	return 0, 0
}

func compileAndRunQSQLFastNativeFixture(t *testing.T, src string) (map[string]runtime.Value, *VM) {
	t.Helper()

	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	proto, err := Compile(prog)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	globals := vmtest.NewInterpreterGlobals()
	v := New(globals)
	if _, err := v.Execute(proto); err != nil {
		v.Close()
		t.Fatalf("runtime error: %v", err)
	}
	return globals, v
}
