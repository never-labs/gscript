package bind

import "testing"

type qRuntimeKernelExecutionExternalStatForTest struct {
	Source  string
	Kernel  string
	Shape   string
	Route   string
	Outcome string
	Count   uint64
}

func qRuntimeKernelExecutionExternalStatToBindForTest(stat qRuntimeKernelExecutionExternalStatForTest) QRuntimeKernelExecutionStat {
	return QRuntimeKernelExecutionStat{
		Source:  stat.Source,
		Kernel:  stat.Kernel,
		Shape:   stat.Shape,
		Route:   stat.Route,
		Outcome: stat.Outcome,
		Count:   stat.Count,
	}
}

func TestQRuntimeKernelExecutionStatsFromMapsExternalRows(t *testing.T) {
	stats := QRuntimeKernelExecutionStatsFrom([]qRuntimeKernelExecutionExternalStatForTest{
		{
			Source:  "methodjit_q_frame_runtime",
			Kernel:  "QFrameSelectColumn",
			Shape:   "compare/filter/project/column",
			Route:   "typed_runtime_op_exit",
			Outcome: "success",
			Count:   7,
		},
	}, qRuntimeKernelExecutionExternalStatToBindForTest)
	if len(stats) != 1 {
		t.Fatalf("mapped stats length = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.Source != "methodjit_q_frame_runtime" || got.Kernel != "QFrameSelectColumn" ||
		got.Shape != "compare/filter/project/column" || got.Route != "typed_runtime_op_exit" ||
		got.Outcome != "success" || got.Count != 7 {
		t.Fatalf("mapped stat = %#v, want field-preserving conversion", got)
	}
	if got := QRuntimeKernelExecutionStatsFrom([]qRuntimeKernelExecutionExternalStatForTest{{Count: 1}}, nil); got != nil {
		t.Fatalf("nil converter mapped stats = %#v, want nil", got)
	}
}

func TestMappedQRuntimeKernelExecutionStatsProviderFeedsCacheStats(t *testing.T) {
	qClearCaches()
	restore := SetMappedQRuntimeKernelExecutionStatsProvider(func() []qRuntimeKernelExecutionExternalStatForTest {
		return []qRuntimeKernelExecutionExternalStatForTest{
			{
				Source:  "methodjit_q_frame_runtime",
				Kernel:  "QFrameSelectColumn",
				Shape:   "compare/filter/project/column",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   2,
			},
			{
				Source:  "methodjit_q_frame_runtime",
				Kernel:  "QFrameSelectColumn",
				Shape:   "compare/filter/project/column",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   3,
			},
		}
	}, qRuntimeKernelExecutionExternalStatToBindForTest)
	defer restore()

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_execution")
	if got := row.RawGetString("executions"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution executions = %v, want 5", got)
	}
	stats := row.RawGetString("stats").Table()
	if stats == nil || stats.Length() != 1 {
		t.Fatalf("q_runtime_kernel_execution stats table = %v, want one aggregated row", stats)
	}
	stat := stats.RawGetInt(1).Table()
	if stat == nil {
		t.Fatal("q_runtime_kernel_execution stats[1] is nil")
	}
	for field, want := range map[string]string{
		"source":  "methodjit_q_frame_runtime",
		"kernel":  "QFrameSelectColumn",
		"shape":   "compare/filter/project/column",
		"route":   "typed_runtime_op_exit",
		"outcome": "success",
	} {
		if got := stat.RawGetString(field); !got.IsString() || got.Str() != want {
			t.Fatalf("q_runtime_kernel_execution stats[1].%s = %v, want %q", field, got, want)
		}
	}
	if got := stat.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution stats[1].count = %v, want 5", got)
	}
}
