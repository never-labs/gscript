//go:build qextension

package methodjit

import (
	"testing"

	stdbind "github.com/never-labs/leia/internal/stdlib/bind"
)

func TestQKernelExecutionStatsMapToBindRuntimeStats(t *testing.T) {
	stats := stdbind.QRuntimeKernelExecutionStatsFrom([]QKernelExecutionStat{
		{
			Source:        "methodjit_q_frame_runtime",
			Kernel:        "QFrameSelectColumn",
			Shape:         "compare/filter/project/column",
			PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column",
			Route:         "typed_runtime_op_exit",
			Outcome:       "success",
			ReasonCode:    "typed_kernel",
			Count:         7,
		},
	}, func(stat QKernelExecutionStat) stdbind.QRuntimeKernelExecutionStat {
		return stdbind.QRuntimeKernelExecutionStat{
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
	if len(stats) != 1 {
		t.Fatalf("mapped stats length = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.Source != "methodjit_q_frame_runtime" || got.Kernel != "QFrameSelectColumn" ||
		got.Shape != "compare/filter/project/column" ||
		got.PipelineShape != "scan=frame|where=compare_mask:column_literal|filter=index|project=column" ||
		got.Route != "typed_runtime_op_exit" || got.Outcome != "success" ||
		got.ReasonCode != "typed_kernel" || got.Count != 7 {
		t.Fatalf("mapped stat = %#v, want MethodJIT q execution stat fields preserved", got)
	}
}

func TestQKernelDescriptorCacheStatsMapToBindRuntimeStats(t *testing.T) {
	stats := stdbind.QRuntimeKernelDescriptorCacheStatsFrom([]QKernelDescriptorCacheStat{
		{
			Source:     "methodjit_q_frame_runtime",
			Kernel:     "QFrameSelectColumn",
			Shape:      "compare/filter/project/column",
			Route:      "typed_runtime_op_exit",
			SchemaHash: "schema-a",
			Entries:    1,
			Hits:       2,
			Misses:     1,
		},
	}, func(stat QKernelDescriptorCacheStat) stdbind.QRuntimeKernelDescriptorCacheStat {
		return stdbind.QRuntimeKernelDescriptorCacheStat{
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
	if len(stats) != 1 {
		t.Fatalf("mapped descriptor cache stats length = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.Source != "methodjit_q_frame_runtime" || got.Kernel != "QFrameSelectColumn" ||
		got.Shape != "compare/filter/project/column" || got.Route != "typed_runtime_op_exit" ||
		got.SchemaHash != "schema-a" || got.Entries != 1 || got.Hits != 2 || got.Misses != 1 {
		t.Fatalf("mapped descriptor cache stat = %#v, want MethodJIT q descriptor cache fields preserved", got)
	}
}
