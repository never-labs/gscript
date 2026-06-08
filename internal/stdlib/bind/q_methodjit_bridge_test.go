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
	assertIntField(t, row, "executions", 2)
	assertIntField(t, row, "successes", 2)
	assertIntField(t, row, "errors", 0)

	stat := onlyNestedRow(t, row, "stats")
	assertStringField(t, stat, "source", "methodjit_q_vector_runtime")
	assertStringField(t, stat, "kernel", "QVectorWhereReduce")
	assertStringField(t, stat, "shape", "compare/vector-where/vector-reduce")
	assertStringField(t, stat, "route", "typed_runtime_op_exit")
	assertStringField(t, stat, "outcome", "success")
	assertIntField(t, stat, "count", 2)

	kernel := onlyNestedRow(t, row, "kernels")
	assertStringField(t, kernel, "source", "methodjit_q_vector_runtime")
	assertStringField(t, kernel, "kernel", "QVectorWhereReduce")
	assertStringField(t, kernel, "outcome", "success")
	assertIntField(t, kernel, "count", 2)

	shape := onlyNestedRow(t, row, "shapes")
	assertStringField(t, shape, "source", "methodjit_q_vector_runtime")
	assertStringField(t, shape, "shape", "compare/vector-where/vector-reduce")
	assertStringField(t, shape, "outcome", "success")
	assertIntField(t, shape, "count", 2)
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

func onlyNestedRow(t *testing.T, row *runtime.Table, field string) *runtime.Table {
	t.Helper()
	nested := row.RawGetString(field).Table()
	if nested == nil || nested.Length() != 1 {
		t.Fatalf("%s nested table = %v, want one row", field, nested)
	}
	item := nested.RawGetInt(1).Table()
	if item == nil {
		t.Fatalf("%s[1] is nil", field)
	}
	return item
}

func assertStringField(t *testing.T, row *runtime.Table, field, want string) {
	t.Helper()
	if got := row.RawGetString(field); !got.IsString() || got.Str() != want {
		t.Fatalf("%s = %v, want %q", field, got, want)
	}
}

func assertIntField(t *testing.T, row *runtime.Table, field string, want int64) {
	t.Helper()
	if got := row.RawGetString(field); !got.IsInt() || got.Int() != want {
		t.Fatalf("%s = %v, want %d", field, got, want)
	}
}
