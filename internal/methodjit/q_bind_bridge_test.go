package methodjit

import (
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

func TestTier2QVectorWhereReduceUsesExpectedRuntimeRoute(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_where_reduce_tier2_route",
		NumParams: 4,
		MaxStack:  4,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_VECTOR_WHERE_REDUCE, 0, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
	cl := vm.NewClosure(proto)
	fn := runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)
	v := vm.New(map[string]runtime.Value{})
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	args := []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
		runtime.IntValue(4),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.IntValue(7),
	}
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

func qKernelExecutionCount(rows []QKernelExecutionStat, source, kernel, route, outcome string) uint64 {
	var count uint64
	for _, row := range rows {
		if row.Source == source && row.Kernel == kernel && row.Route == route && row.Outcome == outcome {
			count += row.Count
		}
	}
	return count
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
