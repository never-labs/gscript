package vm

import (
	"math"
	"testing"

	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
	"github.com/gscript/gscript/internal/runtime"
)

const soaAffineUpdateSource = `
func affine_update(cols, scale, bias) {
    src := soa.column(cols, "x")
    dst := soa.column(cols, "y")
    n := soa.len(cols)
    for i := 1; i <= n; i++ {
        dst[i] = src[i] * scale + bias
    }
}
`

func TestSoAColumnAffineUpdateRuntimeSpecialization(t *testing.T) {
	proto, v := compileSpectralSpecializationTestProgram(t, soaAffineUpdateSource)
	defer v.Close()
	affine := findTestProtoByName(proto, "affine_update")
	if affine == nil {
		t.Fatal("missing affine_update proto")
	}
	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "soa_column_affine_update")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(affine), "soa_column_affine_update")
	if !cachedCallSiteNoResultRuntimeSpecializationRecognized(affine, callSiteNoResultRuntimeSpecializationSoAColumnAffineUpdate) {
		t.Fatal("soa_column_affine_update rejected by no-result runtime specialization cache")
	}

	if _, err := v.Execute(proto); err != nil {
		t.Fatal(err)
	}
	soa := mustTestSoA(t, []float64{1, 2, 4, 8}, []float64{0, 0, 0, 0})
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()
	_, err := v.CallValue(v.GetGlobal("affine_update"), []runtime.Value{
		runtime.SoAValue(soa),
		runtime.FloatValue(1.5),
		runtime.FloatValue(0.25),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteNoResult, "soa_column_affine_update"); got != 1 {
		t.Fatalf("soa_column_affine_update structural hit count = %d, want 1", got)
	}
	_, y := testSoAColumns(t, soa)
	want := []float64{1.75, 3.25, 6.25, 12.25}
	for i, got := range y {
		if math.Abs(got-want[i]) > 1e-12 {
			t.Fatalf("y[%d] = %.12f, want %.12f", i, got, want[i])
		}
	}
}

func TestSoAColumnAffineUpdateIgnoresBenchmarkMetadata(t *testing.T) {
	proto, v := compileSpectralSpecializationTestProgram(t, soaAffineUpdateSource)
	defer v.Close()
	affine := findTestProtoByName(proto, "affine_update")
	if affine == nil {
		t.Fatal("missing affine_update proto")
	}
	affine.Name = "not_a_benchmark_kernel"
	affine.Source = "generated/shape_only.gs"
	if !isSoAColumnAffineUpdateProto(affine) {
		t.Fatal("soa affine update should recognize bytecode shape independent of name/source")
	}
}

func TestSoAColumnAffineUpdateRecognizesArbitraryColumnNames(t *testing.T) {
	src := `
func rewrite_columns(records, mul, add) {
    left := soa.column(records, "mass")
    right := soa.column(records, "energy")
    count := soa.len(records)
    for idx := 1; idx <= count; idx++ {
        right[idx] = left[idx] * mul + add
    }
}
`
	proto, v := compileSpectralSpecializationTestProgram(t, src)
	defer v.Close()
	rewrite := findTestProtoByName(proto, "rewrite_columns")
	if rewrite == nil {
		t.Fatal("missing rewrite_columns proto")
	}
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(rewrite), "soa_column_affine_update")
	if _, err := v.Execute(proto); err != nil {
		t.Fatal(err)
	}
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"mass":   runtime.NewDenseArrayF64([]float64{2, 3}),
		"energy": runtime.NewDenseArrayF64([]float64{0, 0}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.CallValue(v.GetGlobal("rewrite_columns"), []runtime.Value{
		runtime.SoAValue(soa),
		runtime.FloatValue(4),
		runtime.FloatValue(0.5),
	}); err != nil {
		t.Fatal(err)
	}
	energy, ok := soa.Column("energy")
	if !ok {
		t.Fatal("missing energy column")
	}
	got, ok := energy.F64()
	if !ok {
		t.Fatal("energy is not f64")
	}
	if got[0] != 8.5 || got[1] != 12.5 {
		t.Fatalf("energy = %v, want [8.5 12.5]", got)
	}
}

func TestSoAColumnAffineUpdateSupportsI64SourceColumn(t *testing.T) {
	proto, v := compileSpectralSpecializationTestProgram(t, soaAffineUpdateSource)
	defer v.Close()
	affine := findTestProtoByName(proto, "affine_update")
	if affine == nil {
		t.Fatal("missing affine_update proto")
	}
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(affine), "soa_column_affine_update")
	if _, err := v.Execute(proto); err != nil {
		t.Fatal(err)
	}
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x": runtime.NewDenseArrayI64([]int64{2, 4, 8}),
		"y": runtime.NewDenseArrayF64([]float64{0, 0, 0}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.CallValue(v.GetGlobal("affine_update"), []runtime.Value{
		runtime.SoAValue(soa),
		runtime.FloatValue(0.5),
		runtime.FloatValue(1),
	}); err != nil {
		t.Fatal(err)
	}
	yCol, ok := soa.Column("y")
	if !ok {
		t.Fatal("missing y column")
	}
	y, ok := yCol.F64()
	if !ok {
		t.Fatal("y is not f64")
	}
	want := []float64{2, 3, 5}
	for i, got := range y {
		if got != want[i] {
			t.Fatalf("y[%d] = %v, want %v", i, got, want[i])
		}
	}
}

func TestDenseArrayVMIndexFallbackForSoAKernel(t *testing.T) {
	src := `
func affine_update_fallback(cols, scale, bias) {
    src := soa.column(cols, "x")
    dst := soa.column(cols, "y")
    n := soa.len(cols)
    for i := 1; i <= n; i++ {
        dst[i] = bias + src[i] * scale
    }
}
`
	proto, v := compileSpectralSpecializationTestProgram(t, src)
	defer v.Close()
	fallback := findTestProtoByName(proto, "affine_update_fallback")
	if fallback == nil {
		t.Fatal("missing affine_update_fallback proto")
	}
	rejectRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(fallback), "soa_column_affine_update")
	if _, err := v.Execute(proto); err != nil {
		t.Fatal(err)
	}
	soa := mustTestSoA(t, []float64{3, 5}, []float64{0, 0})
	if _, err := v.CallValue(v.GetGlobal("affine_update_fallback"), []runtime.Value{
		runtime.SoAValue(soa),
		runtime.FloatValue(2),
		runtime.FloatValue(1),
	}); err != nil {
		t.Fatal(err)
	}
	_, y := testSoAColumns(t, soa)
	if y[0] != 7 || y[1] != 11 {
		t.Fatalf("fallback y = %v, want [7 11]", y)
	}
}

func BenchmarkSoAStdlibAddScaledFastArg(b *testing.B) {
	src := `
func add_scaled_kernel(cols, scale) {
    soa.addScaled(cols, "x", "y", scale)
}
`
	v, fn := compileBenchmarkFunction(b, src, "add_scaled_kernel")
	defer v.Close()
	soa := mustBenchSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa), runtime.FloatValue(1.00001)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	x, _ := testSoAColumns(b, soa)
	benchmarkFloatSink = x[len(x)-1]
}

func BenchmarkDenseArrayAddScaledVMFallback(b *testing.B) {
	src := `
func add_scaled_loop(cols, scale) {
    src := soa.column(cols, "y")
    dst := soa.column(cols, "x")
    n := soa.len(cols)
    for i := 1; i <= n; i++ {
        dst[i] = dst[i] + src[i] * scale
    }
}
`
	v, fn := compileBenchmarkFunction(b, src, "add_scaled_loop")
	defer v.Close()
	soa := mustBenchSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa), runtime.FloatValue(1.00001)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	x, _ := testSoAColumns(b, soa)
	benchmarkFloatSink = x[len(x)-1]
}

func BenchmarkSoAStdlibSumFastArg(b *testing.B) {
	src := `
func sum_kernel(cols) {
    return soa.sum(cols, "x")
}
`
	v, fn := compileBenchmarkFunction(b, src, "sum_kernel")
	defer v.Close()
	soa := mustBenchSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFloatSink = results[0].Number()
	}
}

func BenchmarkDenseArraySumVMFallback(b *testing.B) {
	src := `
func sum_loop(cols) {
    xs := soa.column(cols, "x")
    n := soa.len(cols)
    total := 0.0
    for i := 1; i <= n; i++ {
        total = total + xs[i]
    }
    return total
}
`
	v, fn := compileBenchmarkFunction(b, src, "sum_loop")
	defer v.Close()
	soa := mustBenchSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFloatSink = results[0].Number()
	}
}

func BenchmarkSoAColumnAffineUpdateRuntimeSpecialization(b *testing.B) {
	v, fn := compileBenchmarkFunction(b, soaAffineUpdateSource, "affine_update")
	defer v.Close()
	soa := mustBenchSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa), runtime.FloatValue(1.00001), runtime.FloatValue(0.25)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	_, y := testSoAColumns(b, soa)
	benchmarkFloatSink = y[len(y)-1]
}

func BenchmarkDenseArrayAffineUpdateVMFallback(b *testing.B) {
	src := `
func affine_update_fallback(cols, scale, bias) {
    src := soa.column(cols, "x")
    dst := soa.column(cols, "y")
    n := soa.len(cols)
    for i := 1; i <= n; i++ {
        dst[i] = bias + src[i] * scale
    }
}
`
	v, fn := compileBenchmarkFunction(b, src, "affine_update_fallback")
	defer v.Close()
	soa := mustBenchSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa), runtime.FloatValue(1.00001), runtime.FloatValue(0.25)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	_, y := testSoAColumns(b, soa)
	benchmarkFloatSink = y[len(y)-1]
}

func BenchmarkTableAoSAffineUpdateVM(b *testing.B) {
	src := `
func aos_affine_update(rows, scale, bias) {
    n := len(rows)
    for i := 1; i <= n; i++ {
        row := rows[i]
        row.y = row.x * scale + bias
    }
}
`
	v, fn := compileBenchmarkFunction(b, src, "aos_affine_update")
	defer v.Close()
	rows := mustBenchRows(32768)
	args := []runtime.Value{runtime.TableValue(rows), runtime.FloatValue(1.00001), runtime.FloatValue(0.25)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	last := rows.RawGetInt(32768).Table().RawGetString("y")
	benchmarkFloatSink = last.Number()
}

var benchmarkFloatSink float64

func compileBenchmarkFunction(b testing.TB, src, name string) (*VM, runtime.Value) {
	b.Helper()
	proto := compileSourceTB(b, src)
	v := New(runtime.NewInterpreterGlobals())
	if _, err := v.Execute(proto); err != nil {
		b.Fatal(err)
	}
	fn := v.GetGlobal(name)
	if !fn.IsFunction() {
		b.Fatalf("%s is not a function: %v", name, fn)
	}
	return v, fn
}

func compileSourceTB(tb testing.TB, src string) *FuncProto {
	tb.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		tb.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		tb.Fatalf("parse error: %v", err)
	}
	proto, err := Compile(prog)
	if err != nil {
		tb.Fatalf("compile error: %v", err)
	}
	return proto
}

func mustTestSoA(t testing.TB, x, y []float64) *runtime.SoA {
	t.Helper()
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x": runtime.NewDenseArrayF64(x),
		"y": runtime.NewDenseArrayF64(y),
	})
	if err != nil {
		t.Fatal(err)
	}
	return soa
}

func mustBenchSoA(b testing.TB, n int) *runtime.SoA {
	b.Helper()
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range x {
		x[i] = 1 + float64(i)*0.001
	}
	return mustTestSoA(b, x, y)
}

func testSoAColumns(t testing.TB, soa *runtime.SoA) ([]float64, []float64) {
	t.Helper()
	xCol, ok := soa.Column("x")
	if !ok {
		t.Fatal("missing x column")
	}
	yCol, ok := soa.Column("y")
	if !ok {
		t.Fatal("missing y column")
	}
	x, ok := xCol.F64()
	if !ok {
		t.Fatal("x is not f64")
	}
	y, ok := yCol.F64()
	if !ok {
		t.Fatal("y is not f64")
	}
	return x, y
}

func mustBenchRows(n int) *runtime.Table {
	rows := runtime.NewTable()
	for i := 1; i <= n; i++ {
		row := runtime.NewTable()
		row.RawSetString("x", runtime.FloatValue(1+float64(i-1)*0.001))
		row.RawSetString("y", runtime.FloatValue(0))
		rows.RawSetInt(int64(i), runtime.TableValue(row))
	}
	return rows
}
