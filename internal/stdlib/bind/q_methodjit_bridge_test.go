package bind_test

import (
	"testing"

	"github.com/never-labs/leia/internal/methodjit"
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
	report := methodjit.Diagnose(proto, args)
	if report.NativeError != nil {
		t.Fatalf("Diagnose native error: %v\n%s", report.NativeError, report.String())
	}
	if len(report.QKernelExecutionStats) == 0 {
		t.Fatalf("Diagnose QKernelExecutionStats empty:\n%s", report.String())
	}

	restore := qbind.SetMappedQRuntimeKernelExecutionStatsProvider(func() []methodjit.QKernelExecutionStat {
		return report.QKernelExecutionStats
	}, func(stat methodjit.QKernelExecutionStat) qbind.QRuntimeKernelExecutionStat {
		return qbind.QRuntimeKernelExecutionStat{
			Source:  stat.Source,
			Kernel:  stat.Kernel,
			Shape:   stat.Shape,
			Route:   stat.Route,
			Outcome: stat.Outcome,
			Count:   stat.Count,
		}
	})
	defer restore()

	row := qCacheStatsRow(t, qCacheStats(t), "q_runtime_kernel_execution")
	assertIntField(t, row, "executions", 4)
	assertIntField(t, row, "successes", 4)
	assertIntField(t, row, "errors", 0)

	stat := nestedRowByFields(t, row, "stats", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "QVectorWhereReduce",
		"shape":   "compare/vector-where/vector-reduce",
		"route":   "typed_runtime_op_exit",
		"outcome": "success",
	})
	assertIntField(t, stat, "count", 2)
	stat = nestedRowByFields(t, row, "stats", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "VectorCompare",
		"shape":   "vector-compare",
		"route":   "typed_runtime_op_exit",
		"outcome": "success",
	})
	assertIntField(t, stat, "count", 2)

	kernel := nestedRowByFields(t, row, "kernels", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "QVectorWhereReduce",
		"outcome": "success",
	})
	assertIntField(t, kernel, "count", 2)
	kernel = nestedRowByFields(t, row, "kernels", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "VectorCompare",
		"outcome": "success",
	})
	assertIntField(t, kernel, "count", 2)

	shape := nestedRowByFields(t, row, "shapes", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"shape":   "compare/vector-where/vector-reduce",
		"outcome": "success",
	})
	assertIntField(t, shape, "count", 2)
	shape = nestedRowByFields(t, row, "shapes", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"shape":   "vector-compare",
		"outcome": "success",
	})
	assertIntField(t, shape, "count", 2)
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
	report := methodjit.Diagnose(proto, args)
	if report.NativeError != nil {
		t.Fatalf("Diagnose native error: %v\n%s", report.NativeError, report.String())
	}

	restore := qbind.SetMappedQRuntimeKernelLoweringStatsProvider(func() []methodjit.QKernelDescriptor {
		return report.QKernelDescriptors
	}, func(stat methodjit.QKernelDescriptor) qbind.QRuntimeKernelLoweringStat {
		if stat.Kind != "fallback" {
			return qbind.QRuntimeKernelLoweringStat{}
		}
		return qbind.QRuntimeKernelLoweringStat{
			Source:       stat.Source,
			Kind:         stat.Kind,
			Kernel:       stat.Kernel,
			Shape:        stat.Shape,
			Route:        stat.Route,
			Outcome:      stat.Outcome,
			ReasonFamily: stat.ReasonFamily,
			ReasonCode:   stat.ReasonCode,
			Count:        1,
		}
	})
	defer restore()

	row := qCacheStatsRow(t, qCacheStats(t), "q_runtime_kernel_lowering")
	assertIntField(t, row, "lowerings", 1)
	assertIntField(t, row, "supported", 0)
	assertIntField(t, row, "fallbacks", 1)

	stat := nestedRowByFields(t, row, "stats", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"kernel":        "QVectorGatherReduce",
		"shape":         "gather/vector-reduce",
		"route":         "lowering",
		"outcome":       "fallback",
		"reason_family": "lowering",
		"reason_code":   "shared_gather",
	})
	assertIntField(t, stat, "count", 1)

	reason := nestedRowByFields(t, row, "reasons", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"reason_family": "lowering",
		"reason_code":   "shared_gather",
	})
	assertIntField(t, reason, "count", 1)

	route := nestedRowByFields(t, row, "routes", map[string]string{
		"source":  "methodjit_q_vector_lowering",
		"kind":    "fallback",
		"kernel":  "QVectorGatherReduce",
		"route":   "lowering",
		"outcome": "fallback",
	})
	assertIntField(t, route, "count", 1)
}

func qMethodJITBridgeFrame(t *testing.T) *runtime.Table {
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

func qCacheStats(t *testing.T) *runtime.Table {
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

func qCacheStatsRow(t *testing.T, tbl *runtime.Table, cache string) *runtime.Table {
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

func nestedRowByFields(t *testing.T, row *runtime.Table, field string, want map[string]string) *runtime.Table {
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

func assertIntField(t *testing.T, row *runtime.Table, field string, want int64) {
	t.Helper()
	if got := row.RawGetString(field); !got.IsInt() || got.Int() != want {
		t.Fatalf("%s = %v, want %d", field, got, want)
	}
}
