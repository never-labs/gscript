package methodjit

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
	"github.com/never-labs/leia/internal/runtime"
	qbind "github.com/never-labs/leia/internal/stdlib/bind"
	"github.com/never-labs/leia/internal/vm"
)

func TestQRuntimeKernelExecutionStatsProviderAggregatesMethodJITDiagnoseRoutesAndKernels(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_runtime_kernel_execution_stats_provider_aggregates_methodjit",
		NumParams: 8,
		MaxStack:  8,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_VECTOR_WHERE_REDUCE, 0, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 4, 5, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_VECTOR_WHERE_REDUCE, 4, 6, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_MOVE, 1, 4, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 3, 0),
		},
	}
	args := []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
		runtime.IntValue(4),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.IntValue(0),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{8, 2, 9})),
		runtime.IntValue(7),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{40, 50, 60})),
		runtime.IntValue(0),
	}
	report := Diagnose(proto, args)
	if report.NativeError != nil {
		t.Fatalf("Diagnose native error: %v\n%s", report.NativeError, report.String())
	}
	if len(report.QKernelExecutionStats) == 0 {
		t.Fatalf("Diagnose QKernelExecutionStats empty:\n%s", report.String())
	}

	restore := qbind.SetMappedQRuntimeKernelExecutionStatsProvider(func() []QKernelExecutionStat {
		return report.QKernelExecutionStats
	}, func(stat QKernelExecutionStat) qbind.QRuntimeKernelExecutionStat {
		return qbind.QRuntimeKernelExecutionStat{
			Source:        stat.Source,
			Kernel:        stat.Kernel,
			Shape:         stat.Shape,
			PipelineShape: stat.PipelineShape,
			Route:         stat.Route,
			Outcome:       stat.Outcome,
			ReasonCode:    stat.ReasonCode,
			Count:         stat.Count,
		}
	})
	defer restore()

	row := qBindCacheStatsRow(t, qBindCacheStats(t), "q_runtime_kernel_execution")
	qBindAssertIntField(t, row, "executions", 4)
	qBindAssertIntField(t, row, "successes", 4)
	qBindAssertIntField(t, row, "errors", 0)

	stat := qBindNestedRowByFields(t, row, "stats", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "QVectorWhereReduce",
		"shape":   "compare/vector-where/vector-reduce",
		"route":   "typed_runtime_op_exit",
		"outcome": "success",
	})
	qBindAssertIntField(t, stat, "count", 2)
	stat = qBindNestedRowByFields(t, row, "stats", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "VectorCompare",
		"shape":   "vector-compare",
		"route":   "typed_runtime_op_exit",
		"outcome": "success",
	})
	qBindAssertIntField(t, stat, "count", 2)

	kernel := qBindNestedRowByFields(t, row, "kernels", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "QVectorWhereReduce",
		"outcome": "success",
	})
	qBindAssertIntField(t, kernel, "count", 2)
	kernel = qBindNestedRowByFields(t, row, "kernels", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "VectorCompare",
		"outcome": "success",
	})
	qBindAssertIntField(t, kernel, "count", 2)

	shape := qBindNestedRowByFields(t, row, "shapes", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"shape":   "compare/vector-where/vector-reduce",
		"outcome": "success",
	})
	qBindAssertIntField(t, shape, "count", 2)
	shape = qBindNestedRowByFields(t, row, "shapes", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"shape":   "vector-compare",
		"outcome": "success",
	})
	qBindAssertIntField(t, shape, "count", 2)
}

func TestQRuntimeKernelLoweringStatsProviderMapsMethodJITFallbacks(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_runtime_kernel_lowering_stats_provider_maps_methodjit_fallbacks",
		NumParams: 2,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 2, 1, 0),
			vm.EncodeABC(vm.OP_MOVE, 3, 2, 0),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 2, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_VECTOR_SCAN, 3, 3, 0),
			vm.EncodeABC(vm.OP_RETURN, 2, 3, 0),
		},
	}
	args := []runtime.Value{
		runtime.TableValue(qMethodJITBridgeFrame(t)),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	}
	report := Diagnose(proto, args)
	if report.NativeError != nil {
		t.Fatalf("Diagnose native error: %v\n%s", report.NativeError, report.String())
	}

	restore := qbind.SetMappedQRuntimeKernelLoweringStatsProviderFiltered(func() []QKernelDescriptor {
		return report.QKernelDescriptors
	}, func(stat QKernelDescriptor) (qbind.QRuntimeKernelLoweringStat, bool) {
		if stat.Kind != "fallback" {
			return qbind.QRuntimeKernelLoweringStat{}, false
		}
		return qbind.QRuntimeKernelLoweringStat{
			Source:        stat.Source,
			Kind:          stat.Kind,
			Kernel:        stat.Kernel,
			Shape:         stat.Shape,
			PipelineShape: stat.PipelineShape,
			Route:         stat.Route,
			Outcome:       stat.Outcome,
			ReasonFamily:  stat.ReasonFamily,
			ReasonCode:    stat.ReasonCode,
			Count:         1,
		}, true
	})
	defer restore()

	row := qBindCacheStatsRow(t, qBindCacheStats(t), "q_runtime_kernel_lowering")
	qBindAssertIntField(t, row, "lowerings", 1)
	qBindAssertIntField(t, row, "supported", 0)
	qBindAssertIntField(t, row, "fallbacks", 1)

	stat := qBindNestedRowByFields(t, row, "stats", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"kernel":        "QVectorGatherReduce",
		"shape":         "gather/vector-reduce",
		"route":         "lowering",
		"outcome":       "fallback",
		"reason_family": "lowering",
		"reason_code":   "shared_gather",
	})
	qBindAssertIntField(t, stat, "count", 1)

	reason := qBindNestedRowByFields(t, row, "reasons", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"reason_family": "lowering",
		"reason_code":   "shared_gather",
	})
	qBindAssertIntField(t, reason, "count", 1)

	reasonShape := qBindNestedRowByFields(t, row, "reason_shapes", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"kernel":        "QVectorGatherReduce",
		"shape":         "gather/vector-reduce",
		"route":         "lowering",
		"outcome":       "fallback",
		"reason_family": "lowering",
		"reason_code":   "shared_gather",
	})
	qBindAssertIntField(t, reasonShape, "count", 1)

	route := qBindNestedRowByFields(t, row, "routes", map[string]string{
		"source":  "methodjit_q_vector_lowering",
		"kind":    "fallback",
		"kernel":  "QVectorGatherReduce",
		"route":   "lowering",
		"outcome": "fallback",
	})
	qBindAssertIntField(t, route, "count", 1)
}

func TestQRuntimeKernelDescriptorCacheStatsProviderMapsMethodJITSchemaStats(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "q_runtime_kernel_descriptor_cache_stats_provider_maps_methodjit",
		NumParams: 1,
		MaxStack:  3,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qMethodJITBridgeFrame(t))})
	if report.NativeError != nil {
		t.Fatalf("Diagnose native error: %v\n%s", report.NativeError, report.String())
	}
	if len(report.QKernelDescriptorCacheStats) == 0 {
		t.Fatalf("Diagnose QKernelDescriptorCacheStats empty:\n%s", report.String())
	}

	restore := qbind.SetMappedQRuntimeKernelDescriptorCacheStatsProvider(func() []QKernelDescriptorCacheStat {
		return report.QKernelDescriptorCacheStats
	}, func(stat QKernelDescriptorCacheStat) qbind.QRuntimeKernelDescriptorCacheStat {
		return qbind.QRuntimeKernelDescriptorCacheStat{
			Source:        stat.Source,
			Kernel:        stat.Kernel,
			Shape:         stat.Shape,
			PipelineShape: stat.PipelineShape,
			Route:         stat.Route,
			SchemaHash:    stat.SchemaHash,
			Entries:       stat.Entries,
			Hits:          stat.Hits,
			Misses:        stat.Misses,
			Evictions:     stat.Evictions,
		}
	})
	defer restore()

	row := qBindCacheStatsRow(t, qBindCacheStats(t), "q_runtime_kernel_descriptor_cache")
	qBindAssertIntField(t, row, "entries", 1)
	qBindAssertIntField(t, row, "hits", 0)
	qBindAssertIntField(t, row, "misses", 1)
	stat := qBindNestedRowByFields(t, row, "stats", map[string]string{
		"source":      "methodjit_q_frame_runtime",
		"kernel":      "QFrameSelectColumn",
		"shape":       "compare/filter/project/column",
		"route":       "typed_runtime_op_exit",
		"schema_hash": "q-methodjit-bridge-test",
	})
	qBindAssertIntField(t, stat, "entries", 1)
	qBindAssertIntField(t, stat, "misses", 1)
}

func TestQSQLKernelPipelineShapeHandoffFeedsMethodJITDescriptorStats(t *testing.T) {
	ref := QSQLKernelPipelineRef{
		Shape:         "where=1|select=2|by=0|order=1|limit=bounded",
		PipelineShape: "scan=frame|where=compare_mask(compare_mask:column_literal)|filter=index|project=typed_expr(typed_binary:1)|order=post_project:1|limit=bounded",
		SchemaHash:    "qsql-schema-a",
	}
	descriptor := QSQLKernelRuntimeDescriptor(ref)
	if descriptor.Source != QSQLKernelRuntimeSource || descriptor.Kernel != QSQLKernelName || descriptor.PipelineShape != ref.PipelineShape {
		t.Fatalf("QSQLKernelRuntimeDescriptor = %+v, want qSQL runtime descriptor with pipeline shape", descriptor)
	}
	cf := &CompiledFunction{}
	cf.RecordQSQLKernelDescriptorCacheLookup(ref)
	cf.RecordQSQLKernelDescriptorCacheLookup(ref)
	stats := cf.QKernelDescriptorCacheStats()
	if len(stats) != 1 {
		t.Fatalf("QKernelDescriptorCacheStats = %+v, want one qSQL pipeline row", stats)
	}
	stat := stats[0]
	if stat.Source != QSQLKernelRuntimeSource || stat.Kernel != QSQLKernelName ||
		stat.Shape != ref.Shape || stat.PipelineShape != ref.PipelineShape ||
		stat.Route != QSQLKernelRuntimeRoute || stat.SchemaHash != ref.SchemaHash ||
		stat.Entries != 1 || stat.Hits != 1 || stat.Misses != 1 {
		t.Fatalf("qSQL descriptor cache stat = %+v, want stable shape/pipeline/schema hit-miss row", stat)
	}

	restore := qbind.SetMappedQRuntimeKernelDescriptorCacheStatsProvider(func() []QKernelDescriptorCacheStat {
		return stats
	}, func(stat QKernelDescriptorCacheStat) qbind.QRuntimeKernelDescriptorCacheStat {
		return qbind.QRuntimeKernelDescriptorCacheStat{
			Source:        stat.Source,
			Kernel:        stat.Kernel,
			Shape:         stat.Shape,
			PipelineShape: stat.PipelineShape,
			Route:         stat.Route,
			SchemaHash:    stat.SchemaHash,
			Entries:       stat.Entries,
			Hits:          stat.Hits,
			Misses:        stat.Misses,
			Evictions:     stat.Evictions,
		}
	})
	defer restore()

	row := qBindCacheStatsRow(t, qBindCacheStats(t), "q_runtime_kernel_descriptor_cache")
	qBindAssertIntField(t, row, "entries", 1)
	qBindAssertIntField(t, row, "hits", 1)
	qBindAssertIntField(t, row, "misses", 1)
	nested := qBindNestedRowByFields(t, row, "stats", map[string]string{
		"source":         QSQLKernelRuntimeSource,
		"kernel":         QSQLKernelName,
		"shape":          ref.Shape,
		"pipeline_shape": ref.PipelineShape,
		"route":          QSQLKernelRuntimeRoute,
		"schema_hash":    ref.SchemaHash,
	})
	qBindAssertIntField(t, nested, "entries", 1)
	qBindAssertIntField(t, nested, "hits", 1)
	qBindAssertIntField(t, nested, "misses", 1)
}

func BenchmarkQFrameVectorMethodJITRoute(b *testing.B) {
	cf := &CompiledFunction{}
	adapter := cf.qFrameVectorRuntimeExecutionAdapter()
	mask := runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true, false, true, false, true, false}))
	trueValues := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30, 40, 50, 60, 70, 80}))
	falseValues := runtime.IntValue(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cf.recordQRuntimePrimitiveExecution(OpFrameColumn, "success")
		out, err := adapter.executeQVectorWhereReduce(
			0,
			int(runtime.DenseArrayReduceSum),
			mask,
			trueValues,
			falseValues,
			qTypedRuntimeExecutionRouteOpExit,
		)
		if err != nil {
			b.Fatalf("executeQVectorWhereReduce: %v", err)
		}
		if !out.IsInt() || out.Int() != 160 {
			b.Fatalf("executeQVectorWhereReduce = %v, want int 160", out)
		}
	}
	b.StopTimer()
	reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, cf.QKernelExecutionStats())
}

func BenchmarkQFrameVectorMethodJITRouteTier2QVectorWhereReduce(b *testing.B) {
	proto := qVectorWhereReduceRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qVectorWhereReduceRouteArgs()
	if _, err := v.CallValue(fn, args); err != nil {
		b.Fatalf("warm QVectorWhereReduce closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		b.Fatalf("CompileTier2(QVectorWhereReduce): %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatalf("settle QVectorWhereReduce closure: %v", err)
		}
	}

	before := tm.QKernelExecutionStatsFor(proto)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatalf("Tier2 QVectorWhereReduce: %v", err)
		}
		if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 57 {
			b.Fatalf("Tier2 QVectorWhereReduce result = %v, want int 57", results)
		}
	}
	b.StopTimer()
	reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
}

func BenchmarkQFrameVectorMethodJITRouteTier2QVectorGatherReduce(b *testing.B) {
	proto := qVectorGatherReduceRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qVectorGatherReduceRouteArgs(b)
	if _, err := v.CallValue(fn, args); err != nil {
		b.Fatalf("warm QVectorGatherReduce closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		b.Fatalf("CompileTier2(QVectorGatherReduce): %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatalf("settle QVectorGatherReduce closure: %v", err)
		}
	}

	before := tm.QKernelExecutionStatsFor(proto)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatalf("Tier2 QVectorGatherReduce: %v", err)
		}
		if len(results) != 1 || !results[0].IsFloat() || results[0].Float() != 200.25 {
			b.Fatalf("Tier2 QVectorGatherReduce result = %v, want float 200.25", results)
		}
	}
	b.StopTimer()
	reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
}

func BenchmarkQFrameVectorMethodJITRouteTier2FrameGroupAggregate(b *testing.B) {
	proto := qFrameGroupAggregateRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameGroupAggregateRouteArgs(b)
	if _, err := v.CallValue(fn, args); err != nil {
		b.Fatalf("warm FrameGroupAggregate closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		b.Fatalf("CompileTier2(FrameGroupAggregate): %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatalf("settle FrameGroupAggregate closure: %v", err)
		}
	}

	before := tm.QKernelExecutionStatsFor(proto)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatalf("Tier2 FrameGroupAggregate: %v", err)
		}
		if len(results) != 1 {
			b.Fatalf("Tier2 FrameGroupAggregate results = %v, want one result", results)
		}
		assertFrameGroupAggregateRouteResult(b, results[0])
	}
	b.StopTimer()
	reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
}

func BenchmarkQFrameVectorMethodJITRouteTier2QFrameSelectColumn(b *testing.B) {
	proto := qFrameSelectColumnRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameSelectColumnRouteArgs(b)
	if _, err := v.CallValue(fn, args); err != nil {
		b.Fatalf("warm QFrameSelectColumn closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		b.Fatalf("CompileTier2(QFrameSelectColumn): %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatalf("settle QFrameSelectColumn closure: %v", err)
		}
	}

	before := tm.QKernelExecutionStatsFor(proto)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatalf("Tier2 QFrameSelectColumn: %v", err)
		}
		if len(results) != 1 {
			b.Fatalf("Tier2 QFrameSelectColumn results = %v, want one result", results)
		}
		assertQFrameSelectColumnRouteResult(b, results[0])
	}
	b.StopTimer()
	reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
}

func BenchmarkQFrameVectorMethodJITRouteTier2FrameProjectColumn(b *testing.B) {
	proto := qFrameProjectColumnRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameProjectColumnRouteArgs(b)
	if _, err := v.CallValue(fn, args); err != nil {
		b.Fatalf("warm FrameProjectColumn closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		b.Fatalf("CompileTier2(FrameProjectColumn): %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatalf("settle FrameProjectColumn closure: %v", err)
		}
	}

	before := tm.QKernelExecutionStatsFor(proto)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatalf("Tier2 FrameProjectColumn: %v", err)
		}
		if len(results) != 1 {
			b.Fatalf("Tier2 FrameProjectColumn results = %v, want one result", results)
		}
		assertFrameProjectColumnRouteResult(b, results[0])
	}
	b.StopTimer()
	reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
}

func BenchmarkQFrameVectorMethodJITRouteTier2FrameFilterProjectColumn(b *testing.B) {
	proto := qFrameFilterProjectColumnRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameFilterProjectColumnRouteArgs(b)
	if _, err := v.CallValue(fn, args); err != nil {
		b.Fatalf("warm FrameFilterProjectColumn closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		b.Fatalf("CompileTier2(FrameFilterProjectColumn): %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := v.CallValue(fn, args); err != nil {
			b.Fatalf("settle FrameFilterProjectColumn closure: %v", err)
		}
	}

	before := tm.QKernelExecutionStatsFor(proto)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			b.Fatalf("Tier2 FrameFilterProjectColumn: %v", err)
		}
		if len(results) != 1 {
			b.Fatalf("Tier2 FrameFilterProjectColumn results = %v, want one result", results)
		}
		assertFrameFilterProjectColumnRouteResult(b, results[0])
	}
	b.StopTimer()
	reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
}

func BenchmarkQFrameVectorMethodJITRouteTier2FrameRowPrimitives(b *testing.B) {
	for _, tc := range qFrameRowPrimitiveRouteCases() {
		b.Run(tc.name, func(b *testing.B) {
			proto := tc.proto()
			cl := vm.NewClosure(proto)
			fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
			v := vm.New(map[string]runtime.Value{})
			defer v.Close()
			tm := NewTieringManager()
			v.SetMethodJIT(tm)
			args := tc.args(b)
			if _, err := v.CallValue(fn, args); err != nil {
				b.Fatalf("warm %s closure: %v", tc.kernel, err)
			}
			if err := tm.CompileTier2(proto); err != nil {
				b.Fatalf("CompileTier2(%s): %v", tc.kernel, err)
			}
			for i := 0; i < 4; i++ {
				if _, err := v.CallValue(fn, args); err != nil {
					b.Fatalf("settle %s closure: %v", tc.kernel, err)
				}
			}

			before := tm.QKernelExecutionStatsFor(proto)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := v.CallValue(fn, args)
				if err != nil {
					b.Fatalf("Tier2 %s: %v", tc.kernel, err)
				}
				if len(results) != 1 {
					b.Fatalf("Tier2 %s results = %v, want one result", tc.kernel, results)
				}
				tc.assert(b, results[0])
			}
			b.StopTimer()
			reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
		})
	}
}

func BenchmarkQFrameVectorMethodJITRouteTier2FrameBasePrimitives(b *testing.B) {
	for _, tc := range qFrameBasePrimitiveRouteCases() {
		b.Run(tc.name, func(b *testing.B) {
			proto := tc.proto()
			cl := vm.NewClosure(proto)
			fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
			v := vm.New(map[string]runtime.Value{})
			defer v.Close()
			tm := NewTieringManager()
			v.SetMethodJIT(tm)
			args := tc.args(b)
			if _, err := v.CallValue(fn, args); err != nil {
				b.Fatalf("warm %s closure: %v", tc.kernel, err)
			}
			if err := tm.CompileTier2(proto); err != nil {
				b.Fatalf("CompileTier2(%s): %v", tc.kernel, err)
			}
			for i := 0; i < 4; i++ {
				if _, err := v.CallValue(fn, args); err != nil {
					b.Fatalf("settle %s closure: %v", tc.kernel, err)
				}
			}

			before := tm.QKernelExecutionStatsFor(proto)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := v.CallValue(fn, args)
				if err != nil {
					b.Fatalf("Tier2 %s: %v", tc.kernel, err)
				}
				if len(results) != 1 {
					b.Fatalf("Tier2 %s results = %v, want one result", tc.kernel, results)
				}
				tc.assert(b, results[0])
			}
			b.StopTimer()
			reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
		})
	}
}

func BenchmarkQFrameVectorMethodJITRouteTier2VectorPrimitives(b *testing.B) {
	for _, tc := range qVectorPrimitiveRouteCases() {
		b.Run(tc.name, func(b *testing.B) {
			proto := tc.proto()
			cl := vm.NewClosure(proto)
			fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
			v := vm.New(map[string]runtime.Value{})
			defer v.Close()
			tm := NewTieringManager()
			v.SetMethodJIT(tm)
			args := tc.args(b)
			if _, err := v.CallValue(fn, args); err != nil {
				b.Fatalf("warm %s closure: %v", tc.kernel, err)
			}
			if err := tm.CompileTier2(proto); err != nil {
				b.Fatalf("CompileTier2(%s): %v", tc.kernel, err)
			}
			for i := 0; i < 4; i++ {
				if _, err := v.CallValue(fn, args); err != nil {
					b.Fatalf("settle %s closure: %v", tc.kernel, err)
				}
			}

			before := tm.QKernelExecutionStatsFor(proto)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := v.CallValue(fn, args)
				if err != nil {
					b.Fatalf("Tier2 %s: %v", tc.kernel, err)
				}
				if len(results) != 1 {
					b.Fatalf("Tier2 %s results = %v, want one result", tc.kernel, results)
				}
				tc.assert(b, results[0])
			}
			b.StopTimer()
			reportMethodJITFrameVectorRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, tm.QKernelExecutionStatsFor(proto)))
		})
	}
}

func TestTier2DirectHelperBridgeQVectorWhereReduceRecordsDirectRoute(t *testing.T) {
	cf := &CompiledFunction{
		QVectorRuntimeKernelShapesByID: map[int]string{
			42: "mask-combine/vector-where/vector-reduce",
		},
	}
	regs := make([]runtime.Value, 5)
	const base = 1
	regs[base+1] = runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true, false}))
	regs[base+2] = runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30, 40}))
	regs[base+3] = runtime.IntValue(0)
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQVectorWhereReduce),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 3,
		OpExitAux:  int64(runtime.DenseArrayReduceSum),
		OpExitID:   42,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QVectorWhereReduce error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := regs[base]; !got.IsInt() || got.Int() != 40 {
		t.Fatalf("QVectorWhereReduce direct helper result = %v, want int 40", got)
	}
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_vector_runtime", "QVectorWhereReduce", "mask-combine/vector-where/vector-reduce", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_vector_runtime", "QVectorWhereReduce", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
}

func TestTier2DirectHelperBridgeQVectorWhereReduceReportsTempSlotRangeError(t *testing.T) {
	cf := &CompiledFunction{}
	regs := make([]runtime.Value, 4)
	const base = 1
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQVectorWhereReduce),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 3,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QVectorWhereReduce direct helper calls = %d, want 1", got)
	}
	const want = "tier2: direct helper: QVectorWhereReduce register range out of bounds"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QVectorWhereReduce error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
}

func TestTier2DirectHelperBridgeQVectorWhereReduceReportsInvalidTempArgCount(t *testing.T) {
	cf := &CompiledFunction{}
	regs := make([]runtime.Value, 5)
	const base = 1
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQVectorWhereReduce),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 2,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	const want = "tier2: direct helper: QVectorWhereReduce register range out of bounds"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QVectorWhereReduce nArgs error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
}

func TestTier2DirectHelperBridgeQVectorGatherReduceRecordsDirectRoute(t *testing.T) {
	cf := &CompiledFunction{
		QVectorRuntimeKernelShapesByID: map[int]string{
			43: "gather/vector-reduce",
		},
	}
	regs := make([]runtime.Value, 4)
	const base = 1
	regs[base+1] = runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20.25, 30}))
	regs[base+2] = runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1}))
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQVectorGatherReduce),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 2,
		OpExitAux:  int64(runtime.DenseArrayReduceSum),
		OpExitID:   43,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QVectorGatherReduce error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := regs[base]; !got.IsFloat() || got.Float() != 40 {
		t.Fatalf("QVectorGatherReduce direct helper result = %v, want float 40", got)
	}
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_vector_runtime", "QVectorGatherReduce", "gather/vector-reduce", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_vector_runtime", "QVectorGatherReduce", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
}

func TestTier2DirectHelperBridgeFrameGroupAggregateRecordsDirectRoute(t *testing.T) {
	proto := qFrameGroupAggregateRouteProto()
	cf := &CompiledFunction{Proto: proto}
	regs := make([]runtime.Value, 4)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	regs[base+2] = runtime.NilValue()
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpFrameGroupAggregate),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 2,
		OpExitAux:  0,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge FrameGroupAggregate error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	assertFrameGroupAggregateRouteResult(t, regs[base])
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "FrameGroupAggregate", "group/aggregate", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_frame_runtime", "FrameGroupAggregate", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
}

func TestTier2DirectHelperBridgeFramePrimitivesRecordRuntimeErrorRoute(t *testing.T) {
	tests := []struct {
		name    string
		kernel  string
		shape   string
		op      Op
		aux     int64
		proto   func() *vm.FuncProto
		args    []runtime.Value
		wantErr string
	}{
		{
			name:    "filter bad mask",
			kernel:  "FrameFilter",
			shape:   "filter",
			op:      OpFrameFilter,
			proto:   qFrameFilterRouteProto,
			args:    []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(t)), runtime.IntValue(1)},
			wantErr: "FrameFilter mask must be dense array",
		},
		{
			name:    "gather bad indexes",
			kernel:  "FrameGather",
			shape:   "gather",
			op:      OpFrameGather,
			proto:   qFrameGatherRouteProto,
			args:    []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(t)), runtime.IntValue(1)},
			wantErr: "FrameGather indexes must be dense array",
		},
		{
			name:    "group aggregate bad spec",
			kernel:  "FrameGroupAggregate",
			shape:   "group/aggregate",
			op:      OpFrameGroupAggregate,
			aux:     0,
			proto:   qFrameGroupAggregateBadSpecRouteProto,
			args:    []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(t)), runtime.NilValue()},
			wantErr: "FRAME_GROUP_AGGREGATE spec must be a table",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cf := &CompiledFunction{Proto: proto}
			regs := make([]runtime.Value, 1+len(tc.args)+1)
			const base = 1
			copy(regs[base+1:], tc.args)
			ctx := &ExecContext{
				HelperCF:   uintptr(unsafe.Pointer(cf)),
				RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
				Regs:       uintptr(unsafe.Pointer(&regs[base])),
				RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
				OpExitOp:   int64(tc.op),
				OpExitSlot: 0,
				OpExitArg1: 1,
				OpExitArg2: 2,
				OpExitAux:  tc.aux,
			}

			beforeDirect := Tier2DirectHelperCallCount()
			tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
			if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
				t.Fatalf("%s direct helper calls = %d, want 1", tc.kernel, got)
			}
			if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || !strings.Contains(ctx.HelperErr.Error(), tc.wantErr) {
				t.Fatalf("%s runtime error flag=%d err=%v, want %q", tc.kernel, ctx.HelperErrFlag, ctx.HelperErr, tc.wantErr)
			}
			stats := cf.QKernelExecutionStats()
			assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", tc.kernel, tc.shape, string(qTypedRuntimeExecutionRouteDirectHelper), "error", 1)
			if got := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "error"); got != 0 {
				t.Fatalf("%s op-exit error route count = %d, want 0", tc.kernel, got)
			}
		})
	}
}

func TestTier2DirectHelperBridgeQFrameSelectColumnRecordsDirectRoute(t *testing.T) {
	proto := qFrameSelectColumnRouteProto()
	cf := &CompiledFunction{
		Proto:                   proto,
		QFrameSelectColumnSpecs: []QFrameSelectColumnSpec{qFrameSelectColumnRouteSpec()},
	}
	regs := make([]runtime.Value, 3)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQFrameSelectColumn),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: -1,
		OpExitAux:  0,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QFrameSelectColumn error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QFrameSelectColumn direct helper calls = %d, want 1", got)
	}
	assertQFrameSelectColumnRouteResult(t, regs[base])
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", "compare/filter/project/column", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_frame_runtime", "QFrameSelectColumn", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertNoQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "FrameMask")
	assertNoQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "FrameFilter")
	assertNoQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "FrameProjectColumn")
	assertNoQKernelExecutionStat(t, stats, "methodjit_q_vector_runtime", "VectorCompare")
	if got := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", string(qTypedRuntimeExecutionRouteOpExit), "success"); got != 0 {
		t.Fatalf("QFrameSelectColumn op-exit route count = %d, want 0", got)
	}
}

func TestTier2DirectHelperBridgeQFrameSelectColumnRecordsRuntimeErrorRoute(t *testing.T) {
	tests := []struct {
		name    string
		proto   func() *vm.FuncProto
		spec    func() QFrameSelectColumnSpec
		wantErr string
	}{
		{
			name: "plan error",
			proto: func() *vm.FuncProto {
				proto := qFrameSelectColumnRouteProto()
				proto.Constants = []runtime.Value{
					runtime.StringValue("price"),
					runtime.FloatValue(100),
					runtime.IntValue(1),
					runtime.StringValue("size"),
				}
				return proto
			},
			spec: func() QFrameSelectColumnSpec {
				return qFrameSelectColumnRouteSpec()
			},
			wantErr: "FrameProject column list must be a string or string array",
		},
		{
			name: "planned execution error",
			proto: func() *vm.FuncProto {
				proto := qFrameSelectColumnRouteProto()
				proto.Constants = []runtime.Value{
					runtime.StringValue("price"),
					runtime.FloatValue(100),
					qHotPathNamesValue("size"),
					runtime.StringValue("size"),
				}
				return proto
			},
			spec: func() QFrameSelectColumnSpec {
				spec := qFrameSelectColumnRouteSpec()
				spec.HasCompareRHSConst = false
				spec.CompareRHSConst = runtime.NilValue()
				spec.DynamicArgRole = QFrameSelectColumnArgCompareRHS
				return spec
			},
			wantErr: "QFrameSelectColumn compare path requires rhs",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cf := &CompiledFunction{
				Proto:                   proto,
				QFrameSelectColumnSpecs: []QFrameSelectColumnSpec{tc.spec()},
			}
			regs := make([]runtime.Value, 3)
			const base = 1
			regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
			ctx := &ExecContext{
				HelperCF:   uintptr(unsafe.Pointer(cf)),
				RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
				Regs:       uintptr(unsafe.Pointer(&regs[base])),
				RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
				OpExitOp:   int64(OpQFrameSelectColumn),
				OpExitSlot: 0,
				OpExitArg1: 1,
				OpExitArg2: -1,
				OpExitAux:  0,
			}

			beforeDirect := Tier2DirectHelperCallCount()
			tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
			if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
				t.Fatalf("QFrameSelectColumn direct helper calls = %d, want 1", got)
			}
			if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || !strings.Contains(ctx.HelperErr.Error(), tc.wantErr) {
				t.Fatalf("QFrameSelectColumn runtime error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, tc.wantErr)
			}
			stats := cf.QKernelExecutionStats()
			assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", "compare/filter/project/column", string(qTypedRuntimeExecutionRouteDirectHelper), "error", 1)
			assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_frame_runtime", "QFrameSelectColumn", string(qTypedRuntimeExecutionRouteDirectHelper), "error", 1)
			if got := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", string(qTypedRuntimeExecutionRouteOpExit), "error"); got != 0 {
				t.Fatalf("QFrameSelectColumn op-exit error route count = %d, want 0", got)
			}
		})
	}
}

func TestTier2DirectHelperBridgeQFrameSelectColumnUsesDynamicCompareRHS(t *testing.T) {
	proto := qFrameSelectColumnRouteProto()
	spec := qFrameSelectColumnRouteSpec()
	spec.HasCompareRHSConst = false
	spec.CompareRHSConst = runtime.NilValue()
	spec.DynamicArgRole = QFrameSelectColumnArgCompareRHS
	cf := &CompiledFunction{
		Proto:                   proto,
		QFrameSelectColumnSpecs: []QFrameSelectColumnSpec{spec},
	}
	regs := make([]runtime.Value, 4)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	regs[base+2] = runtime.FloatValue(100)
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQFrameSelectColumn),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 2,
		OpExitAux:  0,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QFrameSelectColumn dynamic RHS error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QFrameSelectColumn dynamic RHS direct helper calls = %d, want 1", got)
	}
	assertQFrameSelectColumnRouteResult(t, regs[base])
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", "compare/filter/project/column", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
}

func TestTier2DirectHelperBridgeQFrameSelectColumnUsesDynamicRowValue(t *testing.T) {
	proto := qFrameSelectColumnRouteProto()
	spec := qFrameSelectColumnRouteSpec()
	spec.Shape = "gather/project/column"
	spec.SourceColumnConst = -1
	spec.CompareRHSConst = runtime.NilValue()
	spec.HasCompareRHSConst = false
	spec.RowMode = QFrameSelectColumnRowsGather
	spec.DynamicArgRole = QFrameSelectColumnArgRowValue
	cf := &CompiledFunction{
		Proto:                   proto,
		QFrameSelectColumnSpecs: []QFrameSelectColumnSpec{spec},
	}
	regs := make([]runtime.Value, 4)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	regs[base+2] = runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1}))
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQFrameSelectColumn),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 2,
		OpExitAux:  0,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QFrameSelectColumn dynamic row error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	assertDenseI64Values(t, "QFrameSelectColumn dynamic row", regs[base], []int64{20, 5})
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", "gather/project/column", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
}

func TestTier2DirectHelperBridgeQFrameSelectColumnReportsDynamicArgRangeError(t *testing.T) {
	proto := qFrameSelectColumnRouteProto()
	cf := &CompiledFunction{
		Proto:                   proto,
		QFrameSelectColumnSpecs: []QFrameSelectColumnSpec{qFrameSelectColumnRouteSpec()},
	}
	regs := make([]runtime.Value, 3)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQFrameSelectColumn),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 2,
		OpExitAux:  0,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QFrameSelectColumn dynamic arg range direct helper calls = %d, want 1", got)
	}
	const want = "tier2: direct helper: QFrameSelectColumn dynamic arg out of bounds"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QFrameSelectColumn dynamic arg error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
}

func TestTier2DirectHelperBridgeQFrameSelectColumnReportsSpecIndexRangeError(t *testing.T) {
	proto := qFrameSelectColumnRouteProto()
	cf := &CompiledFunction{
		Proto:                   proto,
		QFrameSelectColumnSpecs: []QFrameSelectColumnSpec{qFrameSelectColumnRouteSpec()},
	}
	regs := make([]runtime.Value, 3)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQFrameSelectColumn),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: -1,
		OpExitAux:  1,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	const want = "tier2: direct helper: QFrameSelectColumn spec index is out of range"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QFrameSelectColumn spec index error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
	if len(cf.QKernelExecutionStats()) != 0 {
		t.Fatalf("QFrameSelectColumn spec index stats = %+v, want none before runtime execution", cf.QKernelExecutionStats())
	}
}

func TestTier2DirectHelperBridgeFrameProjectColumnRecordsDirectRoute(t *testing.T) {
	proto := qFrameProjectColumnRouteProto()
	cf := &CompiledFunction{Proto: proto}
	regs := make([]runtime.Value, 3)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpFrameProjectColumn),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitAux:  0,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge FrameProjectColumn error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("FrameProjectColumn direct helper calls = %d, want 1", got)
	}
	assertFrameProjectColumnRouteResult(t, regs[base])
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "FrameProjectColumn", "project/column", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_frame_runtime", "FrameProjectColumn", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	if got := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameProjectColumn", string(qTypedRuntimeExecutionRouteOpExit), "success"); got != 0 {
		t.Fatalf("FrameProjectColumn op-exit route count = %d, want 0", got)
	}
}

func TestTier2DirectHelperBridgeFrameFilterProjectColumnRecordsDirectRoute(t *testing.T) {
	proto := qFrameFilterProjectColumnRouteProto()
	cf := &CompiledFunction{Proto: proto}
	regs := make([]runtime.Value, 4)
	const base = 1
	regs[base+1] = runtime.TableValue(qVectorGatherReduceRouteFrame(t))
	regs[base+2] = runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true}))
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[base])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpFrameFilterProjectColumn),
		OpExitSlot: 0,
		OpExitArg1: 1,
		OpExitArg2: 2,
		OpExitAux:  0,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge FrameFilterProjectColumn error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("FrameFilterProjectColumn direct helper calls = %d, want 1", got)
	}
	assertFrameFilterProjectColumnRouteResult(t, regs[base])
	stats := cf.QKernelExecutionStats()
	assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", "FrameFilterProjectColumn", "filter/project/column", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_frame_runtime", "FrameFilterProjectColumn", string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	if got := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameFilterProjectColumn", string(qTypedRuntimeExecutionRouteOpExit), "success"); got != 0 {
		t.Fatalf("FrameFilterProjectColumn op-exit route count = %d, want 0", got)
	}
}

func TestTier2DirectHelperBridgeVectorPrimitivesRecordDirectRoute(t *testing.T) {
	for _, tc := range qVectorPrimitiveRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cf := &CompiledFunction{Proto: proto}
			args := tc.args(t)
			regs := make([]runtime.Value, 1+len(args)+1)
			const base = 1
			copy(regs[base+1:], args)
			ctx := &ExecContext{
				HelperCF:   uintptr(unsafe.Pointer(cf)),
				RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
				Regs:       uintptr(unsafe.Pointer(&regs[base])),
				RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
				OpExitOp:   int64(tc.op),
				OpExitSlot: 0,
				OpExitArg1: 1,
				OpExitArg2: 2,
				OpExitAux:  tc.aux,
				OpExitID:   42,
			}
			if tc.op == OpVectorWhere {
				ctx.OpExitArg2 = int64(len(args))
			}

			beforeDirect := Tier2DirectHelperCallCount()
			tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
			if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
				t.Fatalf("tier2JITHelperBridge %s error flag=%d err=%v", tc.kernel, ctx.HelperErrFlag, ctx.HelperErr)
			}
			if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
				t.Fatalf("%s direct helper calls = %d, want 1", tc.kernel, got)
			}
			tc.assert(t, regs[base])
			stats := cf.QKernelExecutionStats()
			assertQKernelExecutionStat(t, stats, "methodjit_q_vector_runtime", tc.kernel, tc.shape, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
			assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_vector_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
			if got := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "success"); got != 0 {
				t.Fatalf("%s op-exit route count = %d, want 0", tc.kernel, got)
			}
		})
	}
}

func TestTier2DirectHelperBridgeVectorPrimitivesReportStableErrors(t *testing.T) {
	t.Run("nil compiled function", func(t *testing.T) {
		ctx := &ExecContext{OpExitOp: int64(OpVectorGather)}
		beforeDirect := Tier2DirectHelperCallCount()
		tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
		if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
			t.Fatalf("direct helper calls = %d, want 1", got)
		}
		if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != "tier2: direct helper: nil CompiledFunction" {
			t.Fatalf("error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
		}
	})

	t.Run("invalid register window", func(t *testing.T) {
		cf := &CompiledFunction{}
		ctx := &ExecContext{
			HelperCF: uintptr(unsafe.Pointer(cf)),
			OpExitOp: int64(OpVectorGather),
		}
		beforeDirect := Tier2DirectHelperCallCount()
		tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
		if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
			t.Fatalf("direct helper calls = %d, want 1", got)
		}
		if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != "tier2: direct helper: invalid register window" {
			t.Fatalf("error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
		}
	})

	rangeCases := []struct {
		name string
		op   Op
		err  string
	}{
		{name: "gather", op: OpVectorGather, err: "tier2: direct helper: VectorGather register range out of bounds"},
		{name: "compare", op: OpVectorCompare, err: "tier2: direct helper: VectorCompare register range out of bounds"},
		{name: "mask", op: OpVectorMask, err: "tier2: direct helper: VectorMask register range out of bounds"},
		{name: "where", op: OpVectorWhere, err: "tier2: direct helper: VectorWhere register range out of bounds"},
		{name: "reduce", op: OpVectorReduce, err: "tier2: direct helper: VectorReduce register range out of bounds"},
		{name: "scan", op: OpVectorScan, err: "tier2: direct helper: VectorScan register range out of bounds"},
	}
	for _, tc := range rangeCases {
		t.Run(tc.name+" range", func(t *testing.T) {
			cf := &CompiledFunction{}
			regs := make([]runtime.Value, 3)
			const base = 1
			ctx := &ExecContext{
				HelperCF:   uintptr(unsafe.Pointer(cf)),
				RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
				Regs:       uintptr(unsafe.Pointer(&regs[base])),
				RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
				OpExitOp:   int64(tc.op),
				OpExitSlot: -2,
				OpExitArg1: 1,
				OpExitArg2: 2,
			}
			if tc.op == OpVectorWhere {
				ctx.OpExitSlot = 0
				ctx.OpExitArg1 = 2
				ctx.OpExitArg2 = 3
			}
			beforeDirect := Tier2DirectHelperCallCount()
			tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
			if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
				t.Fatalf("direct helper calls = %d, want 1", got)
			}
			if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != tc.err {
				t.Fatalf("error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, tc.err)
			}
		})
	}

	runtimeCases := []struct {
		name    string
		kernel  string
		shape   string
		op      Op
		aux     int64
		args    []runtime.Value
		wantErr string
	}{
		{
			name:    "gather runtime",
			kernel:  "VectorGather",
			shape:   "vector-gather",
			op:      OpVectorGather,
			args:    []runtime.Value{runtime.IntValue(10), runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1}))},
			wantErr: "VectorGather operand must be dense array",
		},
		{
			name:    "compare runtime",
			kernel:  "VectorCompare",
			shape:   "vector-compare",
			op:      OpVectorCompare,
			aux:     99,
			args:    []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10})), runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1}))},
			wantErr: "VectorCompare op 99 is not a comparison",
		},
		{
			name:    "mask runtime",
			kernel:  "VectorMask",
			shape:   "vector-mask",
			op:      OpVectorMask,
			aux:     int64(runtime.DenseArrayMaskAndNot),
			args:    []runtime.Value{runtime.IntValue(10), runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true}))},
			wantErr: "scalar is not supported for dense array operation",
		},
		{
			name:    "where runtime",
			kernel:  "VectorWhere",
			shape:   "vector-where",
			op:      OpVectorWhere,
			args:    []runtime.Value{runtime.IntValue(10), runtime.IntValue(1), runtime.IntValue(0)},
			wantErr: "VectorWhere mask must be dense array",
		},
		{
			name:    "reduce runtime",
			kernel:  "VectorReduce",
			shape:   "vector/vector-reduce",
			op:      OpVectorReduce,
			aux:     int64(runtime.DenseArrayReduceSum),
			args:    []runtime.Value{runtime.IntValue(10)},
			wantErr: "VectorReduce operand must be dense array",
		},
		{
			name:    "scan runtime",
			kernel:  "VectorScan",
			shape:   "vector-scan",
			op:      OpVectorScan,
			args:    []runtime.Value{runtime.IntValue(10)},
			wantErr: "VectorScan operand must be dense array",
		},
	}
	for _, tc := range runtimeCases {
		t.Run(tc.name, func(t *testing.T) {
			cf := &CompiledFunction{
				QVectorRuntimeKernelShapesByID: map[int]string{
					42: tc.shape,
				},
			}
			regs := make([]runtime.Value, 8)
			const base = 1
			copy(regs[base+1:], tc.args)
			ctx := &ExecContext{
				HelperCF:   uintptr(unsafe.Pointer(cf)),
				RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
				Regs:       uintptr(unsafe.Pointer(&regs[base])),
				RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
				OpExitOp:   int64(tc.op),
				OpExitSlot: 0,
				OpExitArg1: 1,
				OpExitArg2: 2,
				OpExitAux:  tc.aux,
				OpExitID:   42,
			}
			if tc.op == OpVectorWhere {
				ctx.OpExitArg2 = 3
			}

			beforeDirect := Tier2DirectHelperCallCount()
			tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
			if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
				t.Fatalf("%s direct helper calls = %d, want 1", tc.kernel, got)
			}
			if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || !strings.Contains(ctx.HelperErr.Error(), tc.wantErr) {
				t.Fatalf("%s runtime error flag=%d err=%v, want %q", tc.kernel, ctx.HelperErrFlag, ctx.HelperErr, tc.wantErr)
			}
			stats := cf.QKernelExecutionStats()
			assertQKernelExecutionStat(t, stats, "methodjit_q_vector_runtime", tc.kernel, tc.shape, string(qTypedRuntimeExecutionRouteDirectHelper), "error", 1)
			if got := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "error"); got != 0 {
				t.Fatalf("%s op-exit error route count = %d, want 0", tc.kernel, got)
			}
		})
	}
}

func TestTier2DirectHelperBridgeFrameRowPrimitivesRecordDirectRoute(t *testing.T) {
	for _, tc := range qFrameRowPrimitiveRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cf := &CompiledFunction{Proto: proto}
			args := tc.args(t)
			regs := make([]runtime.Value, 1+len(args)+1)
			const base = 1
			copy(regs[base+1:], args)
			ctx := &ExecContext{
				HelperCF:   uintptr(unsafe.Pointer(cf)),
				RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
				Regs:       uintptr(unsafe.Pointer(&regs[base])),
				RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
				OpExitOp:   int64(tc.op),
				OpExitSlot: 0,
				OpExitArg1: 1,
				OpExitArg2: 2,
				OpExitAux:  0,
			}

			beforeDirect := Tier2DirectHelperCallCount()
			tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
			if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
				t.Fatalf("tier2JITHelperBridge %s error flag=%d err=%v", tc.kernel, ctx.HelperErrFlag, ctx.HelperErr)
			}
			if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
				t.Fatalf("%s direct helper calls = %d, want 1", tc.kernel, got)
			}
			tc.assert(t, regs[base])
			stats := cf.QKernelExecutionStats()
			assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", tc.kernel, tc.shape, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
			assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
			if got := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "success"); got != 0 {
				t.Fatalf("%s op-exit route count = %d, want 0", tc.kernel, got)
			}
		})
	}
}

func TestTier2DirectHelperBridgeFrameBasePrimitivesRecordDirectRoute(t *testing.T) {
	for _, tc := range qFrameBasePrimitiveRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cf := &CompiledFunction{Proto: proto}
			args := tc.args(t)
			regs := make([]runtime.Value, 1+len(args)+1)
			const base = 1
			copy(regs[base+1:], args)
			ctx := &ExecContext{
				HelperCF:   uintptr(unsafe.Pointer(cf)),
				RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
				Regs:       uintptr(unsafe.Pointer(&regs[base])),
				RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
				OpExitOp:   int64(tc.op),
				OpExitSlot: 0,
				OpExitArg1: 1,
				OpExitArg2: 2,
				OpExitAux:  0,
			}

			beforeDirect := Tier2DirectHelperCallCount()
			tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
			if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
				t.Fatalf("tier2JITHelperBridge %s error flag=%d err=%v", tc.kernel, ctx.HelperErrFlag, ctx.HelperErr)
			}
			if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
				t.Fatalf("%s direct helper calls = %d, want 1", tc.kernel, got)
			}
			tc.assert(t, regs[base])
			stats := cf.QKernelExecutionStats()
			assertQKernelExecutionStat(t, stats, "methodjit_q_frame_runtime", tc.kernel, tc.shape, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
			assertQKernelExecutionRouteSummary(t, BuildQKernelExecutionRouteSummary(stats), "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
			if got := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "success"); got != 0 {
				t.Fatalf("%s op-exit route count = %d, want 0", tc.kernel, got)
			}
		})
	}
}

func TestTier2QVectorWhereReduceUsesExpectedRuntimeRoute(t *testing.T) {
	proto := qVectorWhereReduceRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qVectorWhereReduceRouteArgs()
	if _, err := v.CallValue(fn, args); err != nil {
		t.Fatalf("warm QVectorWhereReduce closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(QVectorWhereReduce): %v", err)
	}

	beforeDirect := Tier2DirectHelperCallCount()
	const calls = 8
	for i := 0; i < calls; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("Tier2 QVectorWhereReduce call %d: %v", i, err)
		}
		if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 57 {
			t.Fatalf("Tier2 QVectorWhereReduce result = %v, want int 57", results)
		}
	}
	if proto.EnteredTier2 == 0 {
		t.Fatalf("QVectorWhereReduce closure never entered Tier2")
	}

	stats := tm.QKernelExecutionStatsFor(proto)
	directCount := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", "QVectorWhereReduce", string(qTypedRuntimeExecutionRouteDirectHelper), "success")
	opExitCount := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", "QVectorWhereReduce", string(qTypedRuntimeExecutionRouteOpExit), "success")
	if tier2AltStackEnabled() {
		if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
			t.Fatalf("QVectorWhereReduce direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
		if directCount == 0 {
			t.Fatalf("QVectorWhereReduce direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
	} else if directCount != 0 {
		t.Fatalf("QVectorWhereReduce direct helper route recorded with alt-stack disabled; stats=%+v", stats)
	}
	if directCount+opExitCount == 0 {
		t.Fatalf("QVectorWhereReduce runtime route stats missing; stats=%+v", stats)
	}
}

func TestTier2QVectorGatherReduceUsesExpectedRuntimeRoute(t *testing.T) {
	proto := qVectorGatherReduceRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qVectorGatherReduceRouteArgs(t)
	if _, err := v.CallValue(fn, args); err != nil {
		t.Fatalf("warm QVectorGatherReduce closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(QVectorGatherReduce): %v", err)
	}

	beforeDirect := Tier2DirectHelperCallCount()
	const calls = 8
	for i := 0; i < calls; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("Tier2 QVectorGatherReduce call %d: %v", i, err)
		}
		if len(results) != 1 || !results[0].IsFloat() || results[0].Float() != 200.25 {
			t.Fatalf("Tier2 QVectorGatherReduce result = %v, want float 200.25", results)
		}
	}
	if proto.EnteredTier2 == 0 {
		t.Fatalf("QVectorGatherReduce closure never entered Tier2")
	}

	stats := tm.QKernelExecutionStatsFor(proto)
	directCount := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", "QVectorGatherReduce", string(qTypedRuntimeExecutionRouteDirectHelper), "success")
	opExitCount := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", "QVectorGatherReduce", string(qTypedRuntimeExecutionRouteOpExit), "success")
	if tier2AltStackEnabled() {
		if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
			t.Fatalf("QVectorGatherReduce direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
		if directCount == 0 {
			t.Fatalf("QVectorGatherReduce direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
	} else if directCount != 0 {
		t.Fatalf("QVectorGatherReduce direct helper route recorded with alt-stack disabled; stats=%+v", stats)
	}
	if directCount+opExitCount == 0 {
		t.Fatalf("QVectorGatherReduce runtime route stats missing; stats=%+v", stats)
	}
}

func TestTier2FrameGroupAggregateUsesExpectedRuntimeRoute(t *testing.T) {
	proto := qFrameGroupAggregateRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameGroupAggregateRouteArgs(t)
	if _, err := v.CallValue(fn, args); err != nil {
		t.Fatalf("warm FrameGroupAggregate closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(FrameGroupAggregate): %v", err)
	}

	beforeDirect := Tier2DirectHelperCallCount()
	const calls = 8
	for i := 0; i < calls; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("Tier2 FrameGroupAggregate call %d: %v", i, err)
		}
		if len(results) != 1 {
			t.Fatalf("Tier2 FrameGroupAggregate results = %v, want one result", results)
		}
		assertFrameGroupAggregateRouteResult(t, results[0])
	}
	if proto.EnteredTier2 == 0 {
		t.Fatalf("FrameGroupAggregate closure never entered Tier2")
	}

	stats := tm.QKernelExecutionStatsFor(proto)
	directCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameGroupAggregate", string(qTypedRuntimeExecutionRouteDirectHelper), "success")
	opExitCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameGroupAggregate", string(qTypedRuntimeExecutionRouteOpExit), "success")
	if tier2AltStackEnabled() {
		if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
			t.Fatalf("FrameGroupAggregate direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
		if directCount == 0 {
			t.Fatalf("FrameGroupAggregate direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
	} else if directCount != 0 {
		t.Fatalf("FrameGroupAggregate direct helper route recorded with alt-stack disabled; stats=%+v", stats)
	}
	if directCount+opExitCount == 0 {
		t.Fatalf("FrameGroupAggregate runtime route stats missing; stats=%+v", stats)
	}
}

func TestTier2QFrameSelectColumnUsesExpectedRuntimeRoute(t *testing.T) {
	proto := qFrameSelectColumnRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameSelectColumnRouteArgs(t)
	if _, err := v.CallValue(fn, args); err != nil {
		t.Fatalf("warm QFrameSelectColumn closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(QFrameSelectColumn): %v", err)
	}

	beforeDirect := Tier2DirectHelperCallCount()
	const calls = 8
	for i := 0; i < calls; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("Tier2 QFrameSelectColumn call %d: %v", i, err)
		}
		if len(results) != 1 {
			t.Fatalf("Tier2 QFrameSelectColumn results = %v, want one result", results)
		}
		assertQFrameSelectColumnRouteResult(t, results[0])
	}
	if proto.EnteredTier2 == 0 {
		t.Fatalf("QFrameSelectColumn closure never entered Tier2")
	}

	stats := tm.QKernelExecutionStatsFor(proto)
	directCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", string(qTypedRuntimeExecutionRouteDirectHelper), "success")
	opExitCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "QFrameSelectColumn", string(qTypedRuntimeExecutionRouteOpExit), "success")
	if tier2AltStackEnabled() {
		if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
			t.Fatalf("QFrameSelectColumn direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
		if directCount == 0 {
			t.Fatalf("QFrameSelectColumn direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
	} else if directCount != 0 {
		t.Fatalf("QFrameSelectColumn direct helper route recorded with alt-stack disabled; stats=%+v", stats)
	}
	if directCount+opExitCount == 0 {
		t.Fatalf("QFrameSelectColumn runtime route stats missing; stats=%+v", stats)
	}
}

func TestTier2FrameProjectColumnUsesExpectedRuntimeRoute(t *testing.T) {
	proto := qFrameProjectColumnRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameProjectColumnRouteArgs(t)
	if _, err := v.CallValue(fn, args); err != nil {
		t.Fatalf("warm FrameProjectColumn closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(FrameProjectColumn): %v", err)
	}

	beforeDirect := Tier2DirectHelperCallCount()
	const calls = 8
	for i := 0; i < calls; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("Tier2 FrameProjectColumn call %d: %v", i, err)
		}
		if len(results) != 1 {
			t.Fatalf("Tier2 FrameProjectColumn results = %v, want one result", results)
		}
		assertFrameProjectColumnRouteResult(t, results[0])
	}
	if proto.EnteredTier2 == 0 {
		t.Fatalf("FrameProjectColumn closure never entered Tier2")
	}

	stats := tm.QKernelExecutionStatsFor(proto)
	directCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameProjectColumn", string(qTypedRuntimeExecutionRouteDirectHelper), "success")
	opExitCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameProjectColumn", string(qTypedRuntimeExecutionRouteOpExit), "success")
	if tier2AltStackEnabled() {
		if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
			t.Fatalf("FrameProjectColumn direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
		if directCount == 0 {
			t.Fatalf("FrameProjectColumn direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
	} else if directCount != 0 {
		t.Fatalf("FrameProjectColumn direct helper route recorded with alt-stack disabled; stats=%+v", stats)
	}
	if directCount+opExitCount == 0 {
		t.Fatalf("FrameProjectColumn runtime route stats missing; stats=%+v", stats)
	}
}

func TestTier2FrameFilterProjectColumnUsesExpectedRuntimeRoute(t *testing.T) {
	proto := qFrameFilterProjectColumnRouteProto()
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := qFrameFilterProjectColumnRouteArgs(t)
	if _, err := v.CallValue(fn, args); err != nil {
		t.Fatalf("warm FrameFilterProjectColumn closure: %v", err)
	}
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(FrameFilterProjectColumn): %v", err)
	}

	beforeDirect := Tier2DirectHelperCallCount()
	const calls = 8
	for i := 0; i < calls; i++ {
		results, err := v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("Tier2 FrameFilterProjectColumn call %d: %v", i, err)
		}
		if len(results) != 1 {
			t.Fatalf("Tier2 FrameFilterProjectColumn results = %v, want one result", results)
		}
		assertFrameFilterProjectColumnRouteResult(t, results[0])
	}
	if proto.EnteredTier2 == 0 {
		t.Fatalf("FrameFilterProjectColumn closure never entered Tier2")
	}

	stats := tm.QKernelExecutionStatsFor(proto)
	directCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameFilterProjectColumn", string(qTypedRuntimeExecutionRouteDirectHelper), "success")
	opExitCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", "FrameFilterProjectColumn", string(qTypedRuntimeExecutionRouteOpExit), "success")
	if tier2AltStackEnabled() {
		if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
			t.Fatalf("FrameFilterProjectColumn direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
		if directCount == 0 {
			t.Fatalf("FrameFilterProjectColumn direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", stats)
		}
	} else if directCount != 0 {
		t.Fatalf("FrameFilterProjectColumn direct helper route recorded with alt-stack disabled; stats=%+v", stats)
	}
	if directCount+opExitCount == 0 {
		t.Fatalf("FrameFilterProjectColumn runtime route stats missing; stats=%+v", stats)
	}
}

func TestTier2VectorPrimitivesUseExpectedRuntimeRoute(t *testing.T) {
	for _, tc := range qVectorPrimitiveRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cl := vm.NewClosure(proto)
			fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
			v := vm.New(map[string]runtime.Value{})
			defer v.Close()
			tm := NewTieringManager()
			v.SetMethodJIT(tm)
			args := tc.args(t)
			if _, err := v.CallValue(fn, args); err != nil {
				t.Fatalf("warm %s closure: %v", tc.kernel, err)
			}
			if err := tm.CompileTier2(proto); err != nil {
				t.Fatalf("CompileTier2(%s): %v", tc.kernel, err)
			}

			beforeDirect := Tier2DirectHelperCallCount()
			const calls = 8
			for i := 0; i < calls; i++ {
				results, err := v.CallValue(fn, args)
				if err != nil {
					t.Fatalf("Tier2 %s call %d: %v", tc.kernel, i, err)
				}
				if len(results) != 1 {
					t.Fatalf("Tier2 %s results = %v, want one result", tc.kernel, results)
				}
				tc.assert(t, results[0])
			}
			if proto.EnteredTier2 == 0 {
				t.Fatalf("%s closure never entered Tier2", tc.kernel)
			}

			stats := tm.QKernelExecutionStatsFor(proto)
			directCount := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteDirectHelper), "success")
			opExitCount := qKernelExecutionCount(stats, "methodjit_q_vector_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "success")
			if tier2AltStackEnabled() {
				if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
					t.Fatalf("%s direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", tc.kernel, stats)
				}
				if directCount == 0 {
					t.Fatalf("%s direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", tc.kernel, stats)
				}
			} else if directCount != 0 {
				t.Fatalf("%s direct helper route recorded with alt-stack disabled; stats=%+v", tc.kernel, stats)
			}
			if directCount+opExitCount == 0 {
				t.Fatalf("%s runtime route stats missing; stats=%+v", tc.kernel, stats)
			}
		})
	}
}

func TestTier2FrameRowPrimitivesUseExpectedRuntimeRoute(t *testing.T) {
	for _, tc := range qFrameRowPrimitiveRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cl := vm.NewClosure(proto)
			fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
			v := vm.New(map[string]runtime.Value{})
			defer v.Close()
			tm := NewTieringManager()
			v.SetMethodJIT(tm)
			args := tc.args(t)
			if _, err := v.CallValue(fn, args); err != nil {
				t.Fatalf("warm %s closure: %v", tc.kernel, err)
			}
			if err := tm.CompileTier2(proto); err != nil {
				t.Fatalf("CompileTier2(%s): %v", tc.kernel, err)
			}

			beforeDirect := Tier2DirectHelperCallCount()
			const calls = 8
			for i := 0; i < calls; i++ {
				results, err := v.CallValue(fn, args)
				if err != nil {
					t.Fatalf("Tier2 %s call %d: %v", tc.kernel, i, err)
				}
				if len(results) != 1 {
					t.Fatalf("Tier2 %s results = %v, want one result", tc.kernel, results)
				}
				tc.assert(t, results[0])
			}
			if proto.EnteredTier2 == 0 {
				t.Fatalf("%s closure never entered Tier2", tc.kernel)
			}

			stats := tm.QKernelExecutionStatsFor(proto)
			directCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteDirectHelper), "success")
			opExitCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "success")
			if tier2AltStackEnabled() {
				if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
					t.Fatalf("%s direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", tc.kernel, stats)
				}
				if directCount == 0 {
					t.Fatalf("%s direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", tc.kernel, stats)
				}
			} else if directCount != 0 {
				t.Fatalf("%s direct helper route recorded with alt-stack disabled; stats=%+v", tc.kernel, stats)
			}
			if directCount+opExitCount == 0 {
				t.Fatalf("%s runtime route stats missing; stats=%+v", tc.kernel, stats)
			}
		})
	}
}

func TestTier2FrameBasePrimitivesUseExpectedRuntimeRoute(t *testing.T) {
	for _, tc := range qFrameBasePrimitiveRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			proto := tc.proto()
			cl := vm.NewClosure(proto)
			fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
			v := vm.New(map[string]runtime.Value{})
			defer v.Close()
			tm := NewTieringManager()
			v.SetMethodJIT(tm)
			args := tc.args(t)
			if _, err := v.CallValue(fn, args); err != nil {
				t.Fatalf("warm %s closure: %v", tc.kernel, err)
			}
			if err := tm.CompileTier2(proto); err != nil {
				t.Fatalf("CompileTier2(%s): %v", tc.kernel, err)
			}

			beforeDirect := Tier2DirectHelperCallCount()
			const calls = 8
			for i := 0; i < calls; i++ {
				results, err := v.CallValue(fn, args)
				if err != nil {
					t.Fatalf("Tier2 %s call %d: %v", tc.kernel, i, err)
				}
				if len(results) != 1 {
					t.Fatalf("Tier2 %s results = %v, want one result", tc.kernel, results)
				}
				tc.assert(t, results[0])
			}
			if proto.EnteredTier2 == 0 {
				t.Fatalf("%s closure never entered Tier2", tc.kernel)
			}

			stats := tm.QKernelExecutionStatsFor(proto)
			directCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteDirectHelper), "success")
			opExitCount := qKernelExecutionCount(stats, "methodjit_q_frame_runtime", tc.kernel, string(qTypedRuntimeExecutionRouteOpExit), "success")
			if tier2AltStackEnabled() {
				if direct := Tier2DirectHelperCallCount() - beforeDirect; direct == 0 {
					t.Fatalf("%s direct helper calls did not increase under LEIA_JIT_ALT_STACK=1; stats=%+v", tc.kernel, stats)
				}
				if directCount == 0 {
					t.Fatalf("%s direct helper route missing under LEIA_JIT_ALT_STACK=1; stats=%+v", tc.kernel, stats)
				}
			} else if directCount != 0 {
				t.Fatalf("%s direct helper route recorded with alt-stack disabled; stats=%+v", tc.kernel, stats)
			}
			if directCount+opExitCount == 0 {
				t.Fatalf("%s runtime route stats missing; stats=%+v", tc.kernel, stats)
			}
		})
	}
}

func qKernelExecutionCount(rows []QKernelExecutionStat, source, kernel, route, outcome string) uint64 {
	var count uint64
	for _, row := range rows {
		if row.Source == source && row.Kernel == kernel && row.Route == route && row.Outcome == outcome {
			count += row.Count
		}
	}
	return count
}

func assertNoQKernelExecutionStat(tb testing.TB, rows []QKernelExecutionStat, source, kernel string) {
	tb.Helper()
	for _, row := range rows {
		if row.Source == source && row.Kernel == kernel {
			tb.Fatalf("unexpected q kernel execution stat for %s/%s: %+v in %+v", source, kernel, row, rows)
		}
	}
}

func qVectorWhereReduceRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "q_vector_where_reduce_tier2_route",
		NumParams: 4,
		MaxStack:  4,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_VECTOR_WHERE_REDUCE, 0, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qVectorWhereReduceRouteArgs() []runtime.Value {
	return []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
		runtime.IntValue(4),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.IntValue(7),
	}
}

func qVectorGatherReduceRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "q_vector_gather_reduce_tier2_route",
		NumParams: 2,
		MaxStack:  3,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 2, 1, 0),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 2, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 2, 2, 0),
		},
	}
}

func qVectorGatherReduceRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.TableValue(qVectorGatherReduceRouteFrame(tb)),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	}
}

func qVectorGatherReduceRouteFrame(tb testing.TB) *runtime.Table {
	tb.Helper()
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{99, 100.5, 101.25}),
		"size":  runtime.NewDenseArrayI64([]int64{5, 10, 20}),
	})
	if err != nil {
		tb.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "q-vector-gather-route",
	})
	return frame
}

func qFrameGroupAggregateRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_group_aggregate_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(qFrameGroupAggregateSpec("size", []runtime.FrameAggregateSpec{
				{Name: "total", Op: "sum", Column: "price"},
				{Name: "fills", Op: "count"},
			})),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_GROUP_AGGREGATE, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}
}

func qFrameGroupAggregateBadSpecRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_group_aggregate_bad_spec_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.IntValue(1),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_GROUP_AGGREGATE, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}
}

func qFrameSelectColumnRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "q_frame_select_column_tier2_route",
		NumParams: 1,
		MaxStack:  3,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			qHotPathNamesValue("size"),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameProjectColumnRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_project_column_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			qFrameProjectColumnSpecValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_PROJECT_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameFilterProjectColumnRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_filter_project_column_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			qFrameProjectColumnSpecValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT_COLUMN, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}
}

type qFrameRowPrimitiveRouteCase struct {
	name   string
	kernel string
	shape  string
	op     Op
	aux    int64
	proto  func() *vm.FuncProto
	args   func(testing.TB) []runtime.Value
	assert func(testing.TB, runtime.Value)
}

type qFrameBasePrimitiveRouteCase = qFrameRowPrimitiveRouteCase
type qVectorPrimitiveRouteCase = qFrameRowPrimitiveRouteCase

func qVectorPrimitiveRouteCases() []qVectorPrimitiveRouteCase {
	return []qVectorPrimitiveRouteCase{
		{
			name:   "gather",
			kernel: "VectorGather",
			shape:  "vector-gather",
			op:     OpVectorGather,
			proto:  qVectorGatherRouteProto,
			args:   qVectorGatherRouteArgs,
			assert: assertVectorGatherRouteResult,
		},
		{
			name:   "compare",
			kernel: "VectorCompare",
			shape:  "vector-compare",
			op:     OpVectorCompare,
			aux:    int64(runtime.DenseArrayGE),
			proto:  qVectorCompareRouteProto,
			args:   qVectorCompareRouteArgs,
			assert: assertVectorCompareRouteResult,
		},
		{
			name:   "mask",
			kernel: "VectorMask",
			shape:  "vector-mask",
			op:     OpVectorMask,
			aux:    int64(runtime.DenseArrayMaskAndNot),
			proto:  qVectorMaskRouteProto,
			args:   qVectorMaskRouteArgs,
			assert: assertVectorMaskRouteResult,
		},
		{
			name:   "where",
			kernel: "VectorWhere",
			shape:  "vector-where",
			op:     OpVectorWhere,
			proto:  qVectorWhereRouteProto,
			args:   qVectorWhereRouteArgs,
			assert: assertVectorWhereRouteResult,
		},
		{
			name:   "reduce",
			kernel: "VectorReduce",
			shape:  "vector/vector-reduce",
			op:     OpVectorReduce,
			aux:    int64(runtime.DenseArrayReduceMax),
			proto:  qVectorReduceRouteProto,
			args:   qVectorReduceRouteArgs,
			assert: assertVectorReduceRouteResult,
		},
		{
			name:   "scan",
			kernel: "VectorScan",
			shape:  "vector-scan",
			op:     OpVectorScan,
			proto:  qVectorScanRouteProto,
			args:   qVectorScanRouteArgs,
			assert: assertVectorScanRouteResult,
		},
	}
}

func qFrameBasePrimitiveRouteCases() []qFrameBasePrimitiveRouteCase {
	return []qFrameBasePrimitiveRouteCase{
		{
			name:   "len",
			kernel: "FrameLen",
			shape:  "len",
			op:     OpFrameLen,
			proto:  qFrameLenRouteProto,
			args:   qFrameLenRouteArgs,
			assert: assertFrameLenRouteResult,
		},
		{
			name:   "column",
			kernel: "FrameColumn",
			shape:  "column",
			op:     OpFrameColumn,
			proto:  qFrameColumnRouteProto,
			args:   qFrameColumnRouteArgs,
			assert: assertFrameColumnRouteResult,
		},
		{
			name:   "mask",
			kernel: "FrameMask",
			shape:  "mask",
			op:     OpFrameMask,
			proto:  qFrameMaskRouteProto,
			args:   qFrameMaskRouteArgs,
			assert: assertFrameMaskRouteResult,
		},
		{
			name:   "project",
			kernel: "FrameProject",
			shape:  "project",
			op:     OpFrameProject,
			proto:  qFrameProjectRouteProto,
			args:   qFrameProjectRouteArgs,
			assert: assertFrameProjectRouteResult,
		},
		{
			name:   "filter",
			kernel: "FrameFilter",
			shape:  "filter",
			op:     OpFrameFilter,
			proto:  qFrameFilterRouteProto,
			args:   qFrameFilterRouteArgs,
			assert: assertFrameFilterRouteResult,
		},
		{
			name:   "filter_project",
			kernel: "FrameFilterProject",
			shape:  "filter/project",
			op:     OpFrameFilterProject,
			proto:  qFrameFilterProjectRouteProto,
			args:   qFrameFilterProjectRouteArgs,
			assert: assertFrameFilterProjectRouteResult,
		},
	}
}

func qFrameRowPrimitiveRouteCases() []qFrameRowPrimitiveRouteCase {
	return []qFrameRowPrimitiveRouteCase{
		{
			name:   "gather",
			kernel: "FrameGather",
			shape:  "gather",
			op:     OpFrameGather,
			proto:  qFrameGatherRouteProto,
			args:   qFrameGatherRouteArgs,
			assert: assertFrameGatherRouteResult,
		},
		{
			name:   "slice",
			kernel: "FrameSlice",
			shape:  "slice",
			op:     OpFrameSlice,
			proto:  qFrameSliceRouteProto,
			args:   qFrameSliceRouteArgs,
			assert: assertFrameSliceRouteResult,
		},
		{
			name:   "order",
			kernel: "FrameOrder",
			shape:  "order",
			op:     OpFrameOrder,
			proto:  qFrameOrderRouteProto,
			args:   qFrameOrderRouteArgs,
			assert: assertFrameOrderRouteResult,
		},
		{
			name:   "order_gather",
			kernel: "FrameOrderGather",
			shape:  "order/gather",
			op:     OpFrameOrderGather,
			proto:  qFrameOrderGatherRouteProto,
			args:   qFrameOrderGatherRouteArgs,
			assert: assertFrameOrderGatherRouteResult,
		},
	}
}

func qVectorGatherRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "vector_gather_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qVectorCompareRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "vector_compare_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qVectorMaskRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "vector_mask_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_MASK, 0, 1, int(runtime.DenseArrayMaskAndNot)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qVectorWhereRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "vector_where_tier2_route",
		NumParams: 3,
		MaxStack:  3,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 0, 1, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qVectorReduceRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "vector_reduce_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 0, 0, int(runtime.DenseArrayReduceMax)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qVectorScanRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "vector_scan_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_SCAN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameLenRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_len_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_LEN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameColumnRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_column_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameMaskRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_mask_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			qFrameRouteMaskSpecValue("price", ">=", runtime.FloatValue(100)),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_MASK, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameProjectRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_project_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			qHotPathNamesValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameFilterRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_filter_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameFilterProjectRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_filter_project_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			qHotPathNamesValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}
}

func qFrameRouteMaskSpecValue(column, op string, value runtime.Value) runtime.Value {
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue(column))
	spec.RawSetString("op", runtime.StringValue(op))
	spec.RawSetString("value", value)
	return runtime.TableValue(spec)
}

func qFrameGatherRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_gather_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameSliceRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_slice_tier2_route",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameOrderRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_order_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			qFrameOrderSpecValue("price", true, 2),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_ORDER, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameOrderGatherRouteProto() *vm.FuncProto {
	return &vm.FuncProto{
		Name:      "frame_order_gather_tier2_route",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			qFrameOrderSpecValue("price", true, 2),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_ORDER_GATHER, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}

func qFrameOrderSpecValue(column string, desc bool, limit int64) runtime.Value {
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue(column))
	spec.RawSetString("desc", runtime.BoolValue(desc))
	spec.RawSetString("limit", runtime.IntValue(limit))
	return runtime.TableValue(spec)
}

func qFrameProjectColumnSpecValue(name string) runtime.Value {
	spec := runtime.NewTable()
	spec.RawSetString("project", runtime.StringValue(name))
	spec.RawSetString("column", runtime.StringValue(name))
	return runtime.TableValue(spec)
}

func qFrameSelectColumnRouteSpec() QFrameSelectColumnSpec {
	return QFrameSelectColumnSpec{
		Shape:              "compare/filter/project/column",
		SourceColumnConst:  0,
		MaskSpecConst:      -1,
		ProjectConst:       2,
		ResultColumnConst:  3,
		CompareOp:          runtime.DenseArrayGE,
		CompareRHSConst:    runtime.FloatValue(100),
		HasCompareRHSConst: true,
		RowMode:            QFrameSelectColumnRowsNone,
		RowOrderConst:      -1,
		DynamicArgRole:     QFrameSelectColumnArgNone,
		MaskRoot:           -1,
	}
}

func qFrameSelectColumnRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qFrameLenRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qFrameColumnRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qFrameMaskRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qFrameProjectRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qFrameFilterRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.TableValue(qVectorGatherReduceRouteFrame(tb)),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
	}
}

func qFrameFilterProjectRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.TableValue(qVectorGatherReduceRouteFrame(tb)),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
	}
}

func qFrameProjectColumnRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qFrameFilterProjectColumnRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.TableValue(qVectorGatherReduceRouteFrame(tb)),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
	}
}

func qFrameGatherRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.TableValue(qVectorGatherReduceRouteFrame(tb)),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	}
}

func qFrameSliceRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.TableValue(qVectorGatherReduceRouteFrame(tb)),
		runtime.IntValue(2),
	}
}

func qFrameOrderRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qFrameOrderGatherRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.TableValue(qVectorGatherReduceRouteFrame(tb))}
}

func qVectorGatherRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20, 30})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	}
}

func qVectorCompareRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
		runtime.IntValue(4),
	}
}

func qVectorMaskRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true, false})),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{false, true, true, false})),
	}
}

func qVectorWhereRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.IntValue(7),
	}
}

func qVectorReduceRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{1.5, 6.25, 2}))}
}

func qVectorScanRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, -1, 4}))}
}

func assertQFrameSelectColumnRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("QFrameSelectColumn result = %v, want dense array", value)
	}
	got, ok := value.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 20 {
		tb.Fatalf("QFrameSelectColumn values = %#v, want [10 20]", got)
	}
}

func assertVectorGatherRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertDenseF64Values(tb, "VectorGather", value, []float64{30, 10})
}

func assertVectorCompareRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertDenseBoolValues(tb, "VectorCompare", value, []bool{false, true, true})
}

func assertVectorMaskRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertDenseBoolValues(tb, "VectorMask", value, []bool{true, false, false, false})
}

func assertVectorWhereRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertDenseI64Values(tb, "VectorWhere", value, []int64{10, 7, 30})
}

func assertVectorReduceRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsFloat() || value.Float() != 6.25 {
		tb.Fatalf("VectorReduce result = %v, want float 6.25", value)
	}
}

func assertVectorScanRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertDenseI64Values(tb, "VectorScan", value, []int64{2, 1, 5})
}

func assertFrameLenRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsInt() || value.Int() != 3 {
		tb.Fatalf("FrameLen result = %v, want 3", value)
	}
}

func assertFrameColumnRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("FrameColumn result = %v, want dense array", value)
	}
	got, ok := value.DenseArray().I64()
	if !ok || len(got) != 3 || got[0] != 5 || got[1] != 10 || got[2] != 20 {
		tb.Fatalf("FrameColumn values = %#v, want [5 10 20]", got)
	}
}

func assertFrameMaskRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("FrameMask result = %v, want dense array", value)
	}
	got, ok := value.DenseArray().Bool()
	if !ok || len(got) != 3 || got[0] || !got[1] || !got[2] {
		tb.Fatalf("FrameMask values = %#v, want [false true true]", got)
	}
}

func assertFrameProjectRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertFrameSizeColumn(tb, value, []int64{5, 10, 20})
}

func assertFrameFilterRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertFrameSizeColumn(tb, value, []int64{5, 20})
}

func assertFrameFilterProjectRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertFrameSizeColumn(tb, value, []int64{5, 20})
	if _, err := executeFrameColumnValue(value, "price"); err == nil {
		tb.Fatalf("FrameFilterProject result unexpectedly retained price column")
	}
}

func assertFrameGatherRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertFrameSizeColumn(tb, value, []int64{20, 5})
}

func assertFrameSliceRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertFrameSizeColumn(tb, value, []int64{5, 10})
}

func assertFrameOrderRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("FrameOrder result = %v, want dense array", value)
	}
	got, ok := value.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 3 || got[1] != 2 {
		tb.Fatalf("FrameOrder indexes = %#v, want [3 2]", got)
	}
}

func assertFrameOrderGatherRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	assertFrameSizeColumn(tb, value, []int64{20, 10})
}

func assertFrameProjectColumnRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("FrameProjectColumn result = %v, want dense array", value)
	}
	got, ok := value.DenseArray().I64()
	if !ok || len(got) != 3 || got[0] != 5 || got[1] != 10 || got[2] != 20 {
		tb.Fatalf("FrameProjectColumn values = %#v, want [5 10 20]", got)
	}
}

func assertFrameFilterProjectColumnRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("FrameFilterProjectColumn result = %v, want dense array", value)
	}
	got, ok := value.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 5 || got[1] != 20 {
		tb.Fatalf("FrameFilterProjectColumn values = %#v, want [5 20]", got)
	}
}

func assertFrameSizeColumn(tb testing.TB, value runtime.Value, want []int64) {
	tb.Helper()
	col, err := executeFrameColumnValue(value, "size")
	if err != nil {
		tb.Fatalf("frame size column: %v", err)
	}
	if !col.IsDenseArray() {
		tb.Fatalf("frame size column result = %v, want dense array", col)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != len(want) {
		tb.Fatalf("frame size values = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			tb.Fatalf("frame size values = %#v, want %#v", got, want)
		}
	}
}

func assertDenseI64Values(tb testing.TB, label string, value runtime.Value, want []int64) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("%s result = %v, want dense array", label, value)
	}
	got, ok := value.DenseArray().I64()
	if !ok || len(got) != len(want) {
		tb.Fatalf("%s values = %#v, want %#v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			tb.Fatalf("%s values = %#v, want %#v", label, got, want)
		}
	}
}

func assertDenseF64Values(tb testing.TB, label string, value runtime.Value, want []float64) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("%s result = %v, want dense array", label, value)
	}
	got, ok := value.DenseArray().F64()
	if !ok || len(got) != len(want) {
		tb.Fatalf("%s values = %#v, want %#v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			tb.Fatalf("%s values = %#v, want %#v", label, got, want)
		}
	}
}

func assertDenseBoolValues(tb testing.TB, label string, value runtime.Value, want []bool) {
	tb.Helper()
	if !value.IsDenseArray() {
		tb.Fatalf("%s result = %v, want dense array", label, value)
	}
	got, ok := value.DenseArray().Bool()
	if !ok || len(got) != len(want) {
		tb.Fatalf("%s values = %#v, want %#v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			tb.Fatalf("%s values = %#v, want %#v", label, got, want)
		}
	}
}

func qFrameGroupAggregateRouteArgs(tb testing.TB) []runtime.Value {
	tb.Helper()
	return []runtime.Value{
		runtime.TableValue(qVectorGatherReduceRouteFrame(tb)),
		runtime.NilValue(),
	}
}

func assertFrameGroupAggregateRouteResult(tb testing.TB, value runtime.Value) {
	tb.Helper()
	if !value.IsTable() {
		tb.Fatalf("FrameGroupAggregate result = %v, want table", value)
	}
	payload, info, ok := value.Table().NativeFramePayload()
	if !ok || info.Rows != 3 || info.Columns != 3 {
		tb.Fatalf("FrameGroupAggregate payload = %#v info=%#v ok=%v, want 3x3 native frame", payload, info, ok)
	}
	soa, ok := payload.(*runtime.SoA)
	if !ok {
		tb.Fatalf("FrameGroupAggregate payload type = %T, want *runtime.SoA", payload)
	}
	size, ok := soa.Column("size")
	if !ok {
		tb.Fatalf("FrameGroupAggregate result missing size column")
	}
	total, ok := soa.Column("total")
	if !ok {
		tb.Fatalf("FrameGroupAggregate result missing total column")
	}
	fills, ok := soa.Column("fills")
	if !ok {
		tb.Fatalf("FrameGroupAggregate result missing fills column")
	}
	sizeVals, _ := size.I64()
	totalVals, _ := total.F64()
	fillVals, _ := fills.I64()
	if len(sizeVals) != 3 || sizeVals[0] != 5 || sizeVals[1] != 10 || sizeVals[2] != 20 {
		tb.Fatalf("FrameGroupAggregate size values = %#v, want [5 10 20]", sizeVals)
	}
	if len(totalVals) != 3 || totalVals[0] != 99 || totalVals[1] != 100.5 || totalVals[2] != 101.25 {
		tb.Fatalf("FrameGroupAggregate total values = %#v, want [99 100.5 101.25]", totalVals)
	}
	if len(fillVals) != 3 || fillVals[0] != 1 || fillVals[1] != 1 || fillVals[2] != 1 {
		tb.Fatalf("FrameGroupAggregate fills values = %#v, want [1 1 1]", fillVals)
	}
}

func qKernelExecutionStatsDelta(before, after []QKernelExecutionStat) []QKernelExecutionStat {
	counts := make(map[QKernelExecutionStat]int64, len(after))
	for _, row := range after {
		key := row
		key.Count = 0
		counts[key] += int64(row.Count)
	}
	for _, row := range before {
		key := row
		key.Count = 0
		counts[key] -= int64(row.Count)
	}
	out := make([]QKernelExecutionStat, 0, len(counts))
	for key, count := range counts {
		if count <= 0 {
			continue
		}
		key.Count = uint64(count)
		out = append(out, key)
	}
	return out
}

func qMethodJITBridgeFrame(t testing.TB) *runtime.Table {
	t.Helper()
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{99, 100.5, 101.25}),
		"size":  runtime.NewDenseArrayI64([]int64{5, 10, 20}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "q-methodjit-bridge-test",
	})
	return frame
}

func qBindCacheStats(t *testing.T) *runtime.Table {
	t.Helper()
	fn := qbind.BuildQ().RawGetString("cache_stats").GoFunction()
	if fn == nil {
		t.Fatal("q.cache_stats is not a Go function")
	}
	values, err := fn.Fn(nil)
	if err != nil {
		t.Fatalf("q.cache_stats: %v", err)
	}
	if len(values) != 1 || values[0].Table() == nil {
		t.Fatalf("q.cache_stats returned %#v, want one table", values)
	}
	return values[0].Table()
}

func qBindCacheStatsRow(t *testing.T, tbl *runtime.Table, cache string) *runtime.Table {
	t.Helper()
	for i := int64(1); i <= int64(tbl.Length()); i++ {
		row := tbl.RawGetInt(i).Table()
		if row == nil {
			continue
		}
		name := row.RawGetString("cache")
		if name.IsString() && name.Str() == cache {
			return row
		}
	}
	t.Fatalf("q.cache_stats row %q not found", cache)
	return nil
}

func qBindNestedRowByFields(t *testing.T, row *runtime.Table, field string, want map[string]string) *runtime.Table {
	t.Helper()
	nested := row.RawGetString(field).Table()
	if nested == nil {
		t.Fatalf("%s nested table is nil", field)
	}
	for i := int64(1); i <= int64(nested.Length()); i++ {
		item := nested.RawGetInt(i).Table()
		if item == nil {
			continue
		}
		matches := true
		for name, value := range want {
			got := item.RawGetString(name)
			if !got.IsString() || got.Str() != value {
				matches = false
				break
			}
		}
		if matches {
			return item
		}
	}
	t.Fatalf("%s row matching %+v not found", field, want)
	return nil
}

func qBindAssertIntField(t *testing.T, row *runtime.Table, field string, want int64) {
	t.Helper()
	if got := row.RawGetString(field); !got.IsInt() || got.Int() != want {
		t.Fatalf("%s = %v, want %d", field, got, want)
	}
}
