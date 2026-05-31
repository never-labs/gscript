package vm

import (
	"math"
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/runtime"
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

func TestSoAColumnAffineUpdateRefreshesGuardAfterColumnMutation(t *testing.T) {
	proto, v := compileSpectralSpecializationTestProgram(t, soaAffineUpdateSource)
	defer v.Close()
	if _, err := v.Execute(proto); err != nil {
		t.Fatal(err)
	}
	soa := mustTestSoA(t, []float64{1, 2}, []float64{0, 0})
	fn := v.GetGlobal("affine_update")
	for _, scale := range []float64{2, 3} {
		if _, err := v.CallValue(fn, []runtime.Value{
			runtime.SoAValue(soa),
			runtime.FloatValue(scale),
			runtime.FloatValue(1),
		}); err != nil {
			t.Fatal(err)
		}
		x, ok := soa.Column("x")
		if !ok {
			t.Fatal("missing x column")
		}
		x0, err := x.At(0)
		if err != nil {
			t.Fatal(err)
		}
		if err := x.Set(0, runtime.FloatValue(x0.Number()+1)); err != nil {
			t.Fatal(err)
		}
	}
	_, y := testSoAColumns(t, soa)
	if y[0] != 7 || y[1] != 7 {
		t.Fatalf("y = %v, want [7 7]", y)
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

func TestSoAAffineManyLiteralRuntimeSpecialization(t *testing.T) {
	src := `
func affine_many_kernel(cols, scale, bias) {
    soa.affineMany(cols, {
        {dst: "x", src: "vx", scale: scale, bias: bias},
        {dst: "y", src: "vy", scale: scale, bias: bias},
    })
}
`
	proto, v := compileSpectralSpecializationTestProgram(t, src)
	defer v.Close()
	fn := findTestProtoByName(proto, "affine_many_kernel")
	if fn == nil {
		t.Fatal("missing affine_many_kernel proto")
	}
	if _, ok := soaAffineManyLiteralSpecForProto(fn, 3); !ok {
		t.Fatalf("soa affineMany literal specialization did not recognize bytecode:\n%s", Disassemble(fn))
	}
	if _, err := v.Execute(proto); err != nil {
		t.Fatal(err)
	}
	soa := mustBenchParticleSoA(t, 4)
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()
	if _, err := v.CallValue(v.GetGlobal("affine_many_kernel"), []runtime.Value{
		runtime.SoAValue(soa),
		runtime.FloatValue(2),
		runtime.FloatValue(1),
	}); err != nil {
		t.Fatal(err)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteNoResult, "soa_affine_many_literal"); got != 1 {
		t.Fatalf("soa_affine_many_literal hit count = %d, want 1", got)
	}
	x, y := testSoAColumns(t, soa)
	if math.Abs(x[0]-1.22) > 1e-12 || math.Abs(x[3]-1.22006) > 1e-12 {
		t.Fatalf("x = %v, want affine vx*2+1", x)
	}
	if math.Abs(y[0]-1.44) > 1e-12 || math.Abs(y[3]-1.44012) > 1e-12 {
		t.Fatalf("y = %v, want affine vy*2+1", y)
	}
}

func TestSoAAffineManyLiteralRuntimeSpecializationInsideLoop(t *testing.T) {
	src := `
func affine_many_kernel(cols, steps, scale) {
    for step := 1; step <= steps; step++ {
        soa.affineMany(cols, {
            {dst: "x", src: "vx", scale: scale, bias: 0.0},
            {dst: "y", src: "vy", scale: scale, bias: 1.0},
            {dst: "z", src: "vz", scale: scale, bias: 2.0},
        })
    }
}
`
	proto, v := compileSpectralSpecializationTestProgram(t, src)
	defer v.Close()
	fn := findTestProtoByName(proto, "affine_many_kernel")
	if fn == nil {
		t.Fatal("missing affine_many_kernel proto")
	}
	if _, ok := soaAffineManyLiteralSpecForProto(fn, 7); !ok {
		t.Fatalf("loop soa affineMany literal specialization did not recognize bytecode:\n%s", Disassemble(fn))
	}
	if _, err := v.Execute(proto); err != nil {
		t.Fatal(err)
	}
	soa := mustBenchParticleSoA(t, 4)
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()
	if _, err := v.CallValue(v.GetGlobal("affine_many_kernel"), []runtime.Value{
		runtime.SoAValue(soa),
		runtime.IntValue(3),
		runtime.FloatValue(2),
	}); err != nil {
		t.Fatal(err)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteNoResult, "soa_affine_many_literal"); got != 3 {
		t.Fatalf("soa_affine_many_literal hit count = %d, want 3", got)
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

func BenchmarkSoAStdlibLenFastArg(b *testing.B) {
	src := `
func len_kernel(cols) {
    return soa.len(cols)
}
`
	v, fn := compileBenchmarkFunction(b, src, "len_kernel")
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
		benchmarkFloatSink = float64(results[0].Int())
	}
}

func BenchmarkSoAStdlibColumnFastArg(b *testing.B) {
	src := `
func column_kernel(cols) {
    return soa.column(cols, "x")
}
`
	v, fn := compileBenchmarkFunction(b, src, "column_kernel")
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
		benchmarkFloatSink = float64(results[0].DenseArray().Len())
	}
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

func BenchmarkSoAStdlibAffineMany(b *testing.B) {
	src := `
func affine_many_kernel(cols, scale, bias) {
    soa.affineMany(cols, {
        {dst: "x", src: "vx", scale: scale, bias: bias},
        {dst: "y", src: "vy", scale: scale, bias: bias},
    })
}
`
	v, fn := compileBenchmarkFunction(b, src, "affine_many_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa), runtime.FloatValue(1.00001), runtime.FloatValue(0.25)}
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

func BenchmarkSoAStdlibAffineWhere(b *testing.B) {
	src := `
func affine_where_kernel(cols, scale, mask, bias) {
    soa.affineWhere(cols, "x", "vx", scale, mask, bias)
}
`
	v, fn := compileBenchmarkFunction(b, src, "affine_where_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	mask := runtime.NewDenseArrayBool(makeAlternatingMask(32768))
	args := []runtime.Value{runtime.SoAValue(soa), runtime.FloatValue(1.00001), runtime.DenseArrayValue(mask), runtime.FloatValue(0.25)}
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

func BenchmarkSoAStdlibSlice(b *testing.B) {
	src := `
func slice_kernel(cols, first, last) {
    return soa.slice(cols, first, last)
}
`
	v, fn := compileBenchmarkFunction(b, src, "slice_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	args := []runtime.Value{runtime.SoAValue(soa), runtime.IntValue(2048), runtime.IntValue(24575)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFloatSink = float64(results[0].SoA().Len())
	}
}

func BenchmarkSoAStdlibFilter(b *testing.B) {
	src := `
func filter_kernel(cols, mask) {
    return soa.filter(cols, mask)
}
`
	v, fn := compileBenchmarkFunction(b, src, "filter_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	mask := runtime.NewDenseArrayBool(makeAlternatingMask(32768))
	args := []runtime.Value{runtime.SoAValue(soa), runtime.DenseArrayValue(mask)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFloatSink = float64(results[0].SoA().Len())
	}
}

func BenchmarkSoAStdlibGather(b *testing.B) {
	src := `
func gather_kernel(cols, indices) {
    return soa.gather(cols, indices)
}
`
	v, fn := compileBenchmarkFunction(b, src, "gather_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	indices := runtime.NewDenseArrayI64(makeAlternatingIndices(32768))
	args := []runtime.Value{runtime.SoAValue(soa), runtime.DenseArrayValue(indices)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFloatSink = float64(results[0].SoA().Len())
	}
}

func BenchmarkSoAStdlibSumWhere(b *testing.B) {
	src := `
func sum_where_kernel(cols, mask) {
    return soa.sumWhere(cols, "x", mask)
}
`
	v, fn := compileBenchmarkFunction(b, src, "sum_where_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	mask := runtime.NewDenseArrayBool(makeAlternatingMask(32768))
	args := []runtime.Value{runtime.SoAValue(soa), runtime.DenseArrayValue(mask)}
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

func BenchmarkSoAStdlibCountWhere(b *testing.B) {
	src := `
func count_where_kernel(cols, mask) {
    return soa.countWhere(cols, mask)
}
`
	v, fn := compileBenchmarkFunction(b, src, "count_where_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	mask := runtime.NewDenseArrayBool(makeAlternatingMask(32768))
	args := []runtime.Value{runtime.SoAValue(soa), runtime.DenseArrayValue(mask)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFloatSink = float64(results[0].Int())
	}
}

func BenchmarkSoAStdlibMinWhere(b *testing.B) {
	benchmarkSoAMaskedColumnAggregate(b, "minWhere")
}

func BenchmarkSoAStdlibMeanWhere(b *testing.B) {
	benchmarkSoAMaskedColumnAggregate(b, "meanWhere")
}

func BenchmarkSoAStdlibStatsWhere(b *testing.B) {
	src := `
func stats_where_kernel(cols, mask) {
    return soa.statsWhere(cols, "x", mask)
}
`
	v, fn := compileBenchmarkFunction(b, src, "stats_where_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	mask := runtime.NewDenseArrayBool(makeAlternatingMask(32768))
	args := []runtime.Value{runtime.SoAValue(soa), runtime.DenseArrayValue(mask)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFloatSink = results[0].Table().RawGetString("mean").Number()
	}
}

func BenchmarkSoAStdlibMaxWhere(b *testing.B) {
	benchmarkSoAMaskedColumnAggregate(b, "maxWhere")
}

func benchmarkSoAMaskedColumnAggregate(b *testing.B, name string) {
	src := `
func aggregate_where_kernel(cols, mask) {
    return soa.` + name + `(cols, "x", mask)
}
`
	v, fn := compileBenchmarkFunction(b, src, "aggregate_where_kernel")
	defer v.Close()
	soa := mustBenchParticleSoA(b, 32768)
	mask := runtime.NewDenseArrayBool(makeAlternatingMask(32768))
	args := []runtime.Value{runtime.SoAValue(soa), runtime.DenseArrayValue(mask)}
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

func mustBenchParticleSoA(b testing.TB, n int) *runtime.SoA {
	b.Helper()
	x := make([]float64, n)
	y := make([]float64, n)
	z := make([]float64, n)
	vx := make([]float64, n)
	vy := make([]float64, n)
	vz := make([]float64, n)
	for i := range x {
		f := 1 + float64(i)*0.001
		x[i] = f
		y[i] = f * 2
		z[i] = f * 3
		vx[i] = 0.1 + f*0.01
		vy[i] = 0.2 + f*0.02
		vz[i] = 0.3 + f*0.03
	}
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64(x),
		"y":  runtime.NewDenseArrayF64(y),
		"z":  runtime.NewDenseArrayF64(z),
		"vx": runtime.NewDenseArrayF64(vx),
		"vy": runtime.NewDenseArrayF64(vy),
		"vz": runtime.NewDenseArrayF64(vz),
	})
	if err != nil {
		b.Fatal(err)
	}
	return soa
}

func makeAlternatingMask(n int) []bool {
	mask := make([]bool, n)
	for i := range mask {
		mask[i] = i%2 == 0
	}
	return mask
}

func makeAlternatingIndices(n int) []int64 {
	indices := make([]int64, n/2)
	for i := range indices {
		indices[i] = int64(n - i*2)
	}
	return indices
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
