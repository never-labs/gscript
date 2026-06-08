package methodjit

import (
	"testing"

	stdbind "github.com/never-labs/leia/internal/stdlib/bind"
)

func TestQKernelExecutionStatsMapToBindRuntimeStats(t *testing.T) {
	stats := stdbind.QRuntimeKernelExecutionStatsFrom([]QKernelExecutionStat{
		{
			Source:  "methodjit_q_frame_runtime",
			Kernel:  "QFrameSelectColumn",
			Shape:   "compare/filter/project/column",
			Route:   "typed_runtime_op_exit",
			Outcome: "success",
			Count:   7,
		},
	}, func(stat QKernelExecutionStat) stdbind.QRuntimeKernelExecutionStat {
		return stdbind.QRuntimeKernelExecutionStat{
			Source:  stat.Source,
			Kernel:  stat.Kernel,
			Shape:   stat.Shape,
			Route:   stat.Route,
			Outcome: stat.Outcome,
			Count:   stat.Count,
		}
	})
	if len(stats) != 1 {
		t.Fatalf("mapped stats length = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.Source != "methodjit_q_frame_runtime" || got.Kernel != "QFrameSelectColumn" ||
		got.Shape != "compare/filter/project/column" || got.Route != "typed_runtime_op_exit" ||
		got.Outcome != "success" || got.Count != 7 {
		t.Fatalf("mapped stat = %#v, want MethodJIT q execution stat fields preserved", got)
	}
}
