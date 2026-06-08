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

type qRuntimeKernelDescriptorCacheExternalStatForTest struct {
	Source     string
	Kernel     string
	Shape      string
	Route      string
	SchemaHash string
	Entries    uint64
	Hits       uint64
	Misses     uint64
	Evictions  uint64
}

type qRuntimeKernelLoweringExternalStatForTest struct {
	Source       string
	Kind         string
	Kernel       string
	Shape        string
	Route        string
	Outcome      string
	ReasonFamily string
	ReasonCode   string
	Count        uint64
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

func qRuntimeKernelDescriptorCacheExternalStatToBindForTest(stat qRuntimeKernelDescriptorCacheExternalStatForTest) QRuntimeKernelDescriptorCacheStat {
	return QRuntimeKernelDescriptorCacheStat{
		Source:     stat.Source,
		Kernel:     stat.Kernel,
		Shape:      stat.Shape,
		Route:      stat.Route,
		SchemaHash: stat.SchemaHash,
		Entries:    stat.Entries,
		Hits:       stat.Hits,
		Misses:     stat.Misses,
		Evictions:  stat.Evictions,
	}
}

func qRuntimeKernelLoweringExternalStatToBindForTest(stat qRuntimeKernelLoweringExternalStatForTest) QRuntimeKernelLoweringStat {
	return QRuntimeKernelLoweringStat{
		Source:       stat.Source,
		Kind:         stat.Kind,
		Kernel:       stat.Kernel,
		Shape:        stat.Shape,
		Route:        stat.Route,
		Outcome:      stat.Outcome,
		ReasonFamily: stat.ReasonFamily,
		ReasonCode:   stat.ReasonCode,
		Count:        stat.Count,
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
	zeroRows := QRuntimeKernelExecutionStatsFrom([]qRuntimeKernelExecutionExternalStatForTest{{Count: 1}}, func(qRuntimeKernelExecutionExternalStatForTest) QRuntimeKernelExecutionStat {
		return QRuntimeKernelExecutionStat{}
	})
	if len(zeroRows) != 1 {
		t.Fatalf("zero execution row mapping length = %d, want 1; use filtered mapper to skip rows", len(zeroRows))
	}
}

func TestQRuntimeKernelExecutionStatsFromFilteredSkipsExternalRows(t *testing.T) {
	stats := QRuntimeKernelExecutionStatsFromFiltered([]qRuntimeKernelExecutionExternalStatForTest{
		{
			Source:  "methodjit_q_frame_runtime",
			Kernel:  "QFrameSelectColumn",
			Shape:   "compare/filter/project/column",
			Route:   "typed_runtime_op_exit",
			Outcome: "success",
			Count:   4,
		},
		{
			Source:  "methodjit_q_vector_runtime",
			Kernel:  "QVectorGatherReduce",
			Shape:   "gather/vector-reduce",
			Route:   "typed_runtime_op_exit",
			Outcome: "success",
			Count:   7,
		},
	}, func(stat qRuntimeKernelExecutionExternalStatForTest) (QRuntimeKernelExecutionStat, bool) {
		if stat.Kernel != "QVectorGatherReduce" {
			return QRuntimeKernelExecutionStat{}, false
		}
		return qRuntimeKernelExecutionExternalStatToBindForTest(stat), true
	})
	if len(stats) != 1 {
		t.Fatalf("filtered execution stats length = %d, want 1", len(stats))
	}
	if stats[0].Kernel != "QVectorGatherReduce" || stats[0].Count != 7 {
		t.Fatalf("filtered execution stat = %#v, want vector row", stats[0])
	}
	if got := QRuntimeKernelExecutionStatsFromFiltered([]qRuntimeKernelExecutionExternalStatForTest{{Count: 1}}, nil); got != nil {
		t.Fatalf("nil filtered converter mapped execution stats = %#v, want nil", got)
	}
}

func TestQRuntimeKernelDescriptorCacheStatsFromMapsExternalRows(t *testing.T) {
	stats := QRuntimeKernelDescriptorCacheStatsFrom([]qRuntimeKernelDescriptorCacheExternalStatForTest{
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
	}, qRuntimeKernelDescriptorCacheExternalStatToBindForTest)
	if len(stats) != 1 {
		t.Fatalf("mapped descriptor cache stats length = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.Source != "methodjit_q_frame_runtime" || got.Kernel != "QFrameSelectColumn" ||
		got.Shape != "compare/filter/project/column" || got.Route != "typed_runtime_op_exit" ||
		got.SchemaHash != "schema-a" || got.Entries != 1 || got.Hits != 2 || got.Misses != 1 {
		t.Fatalf("mapped descriptor cache stat = %#v, want field-preserving conversion", got)
	}
	if got := QRuntimeKernelDescriptorCacheStatsFrom([]qRuntimeKernelDescriptorCacheExternalStatForTest{{Hits: 1}}, nil); got != nil {
		t.Fatalf("nil converter mapped descriptor cache stats = %#v, want nil", got)
	}
}

func TestQRuntimeKernelDescriptorCacheStatsFromFilteredSkipsExternalRows(t *testing.T) {
	stats := QRuntimeKernelDescriptorCacheStatsFromFiltered([]qRuntimeKernelDescriptorCacheExternalStatForTest{
		{
			Kernel: "QFrameSelectColumn",
			Hits:   1,
		},
		{
			Kernel: "FrameGroupAggregate",
			Misses: 1,
		},
	}, func(stat qRuntimeKernelDescriptorCacheExternalStatForTest) (QRuntimeKernelDescriptorCacheStat, bool) {
		if stat.Kernel != "FrameGroupAggregate" {
			return QRuntimeKernelDescriptorCacheStat{}, false
		}
		return qRuntimeKernelDescriptorCacheExternalStatToBindForTest(stat), true
	})
	if len(stats) != 1 {
		t.Fatalf("filtered descriptor cache stats length = %d, want 1", len(stats))
	}
	if stats[0].Kernel != "FrameGroupAggregate" || stats[0].Misses != 1 {
		t.Fatalf("filtered descriptor cache stat = %#v, want group aggregate row", stats[0])
	}
	if got := QRuntimeKernelDescriptorCacheStatsFromFiltered([]qRuntimeKernelDescriptorCacheExternalStatForTest{{Hits: 1}}, nil); got != nil {
		t.Fatalf("nil filtered converter mapped descriptor cache stats = %#v, want nil", got)
	}
}

func TestQRuntimeKernelLoweringStatsFromMapsExternalRows(t *testing.T) {
	stats := QRuntimeKernelLoweringStatsFrom([]qRuntimeKernelLoweringExternalStatForTest{
		{
			Source:       "methodjit_q_vector_lowering",
			Kind:         "fallback",
			Kernel:       "QVectorGatherReduce",
			Shape:        "gather/vector-reduce",
			Route:        "lowering",
			Outcome:      "fallback",
			ReasonFamily: "lowering",
			ReasonCode:   "shared_gather",
			Count:        7,
		},
	}, qRuntimeKernelLoweringExternalStatToBindForTest)
	if len(stats) != 1 {
		t.Fatalf("mapped lowering stats length = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.Source != "methodjit_q_vector_lowering" || got.Kind != "fallback" ||
		got.Kernel != "QVectorGatherReduce" ||
		got.Shape != "gather/vector-reduce" || got.Route != "lowering" ||
		got.Outcome != "fallback" || got.ReasonFamily != "lowering" ||
		got.ReasonCode != "shared_gather" || got.Count != 7 {
		t.Fatalf("mapped lowering stat = %#v, want field-preserving conversion", got)
	}
	if got := QRuntimeKernelLoweringStatsFrom([]qRuntimeKernelLoweringExternalStatForTest{{Count: 1}}, nil); got != nil {
		t.Fatalf("nil converter mapped lowering stats = %#v, want nil", got)
	}
	zeroRows := QRuntimeKernelLoweringStatsFrom([]qRuntimeKernelLoweringExternalStatForTest{{Count: 1}}, func(qRuntimeKernelLoweringExternalStatForTest) QRuntimeKernelLoweringStat {
		return QRuntimeKernelLoweringStat{}
	})
	if len(zeroRows) != 1 {
		t.Fatalf("zero lowering row mapping length = %d, want 1; use filtered mapper to skip rows", len(zeroRows))
	}
}

func TestQRuntimeKernelLoweringStatsFromFilteredSkipsExternalRows(t *testing.T) {
	stats := QRuntimeKernelLoweringStatsFromFiltered([]qRuntimeKernelLoweringExternalStatForTest{
		{
			Source:  "methodjit_q_vector_runtime",
			Kind:    "runtime_kernel",
			Kernel:  "QVectorGatherReduce",
			Shape:   "gather/vector-reduce",
			Route:   "typed_runtime_op_exit",
			Outcome: "supported",
			Count:   4,
		},
		{
			Source:     "methodjit_q_vector_lowering",
			Kernel:     "QVectorGatherReduce",
			Shape:      "gather/vector-reduce",
			ReasonCode: "shared_gather",
			Count:      7,
		},
	}, func(stat qRuntimeKernelLoweringExternalStatForTest) (QRuntimeKernelLoweringStat, bool) {
		if stat.Outcome == "supported" {
			return QRuntimeKernelLoweringStat{}, false
		}
		return qRuntimeKernelLoweringExternalStatToBindForTest(stat), true
	})
	if len(stats) != 1 {
		t.Fatalf("filtered lowering stats length = %d, want 1", len(stats))
	}
	if stats[0].Kernel != "QVectorGatherReduce" || stats[0].Count != 7 {
		t.Fatalf("filtered lowering stat = %#v, want fallback row", stats[0])
	}
	if got := QRuntimeKernelLoweringStatsFromFiltered([]qRuntimeKernelLoweringExternalStatForTest{{Count: 1}}, nil); got != nil {
		t.Fatalf("nil filtered converter mapped lowering stats = %#v, want nil", got)
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

func TestMappedQRuntimeKernelExecutionStatsProviderFilteredFeedsCacheStats(t *testing.T) {
	qClearCaches()
	restore := SetMappedQRuntimeKernelExecutionStatsProviderFiltered(func() []qRuntimeKernelExecutionExternalStatForTest {
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
				Source:  "methodjit_q_vector_runtime",
				Kernel:  "QVectorGatherReduce",
				Shape:   "gather/vector-reduce",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   5,
			},
		}
	}, func(stat qRuntimeKernelExecutionExternalStatForTest) (QRuntimeKernelExecutionStat, bool) {
		if stat.Kernel != "QVectorGatherReduce" {
			return QRuntimeKernelExecutionStat{}, false
		}
		return qRuntimeKernelExecutionExternalStatToBindForTest(stat), true
	})
	defer restore()

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_execution")
	if got := row.RawGetString("executions"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution executions = %v, want filtered count 5", got)
	}
	stat := qTestNestedRowByFields(t, row, "stats", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kernel":  "QVectorGatherReduce",
		"shape":   "gather/vector-reduce",
		"route":   "typed_runtime_op_exit",
		"outcome": "success",
	})
	if got := stat.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution filtered stat count = %v, want 5", got)
	}
}

func TestMappedQRuntimeKernelDescriptorCacheStatsProviderFeedsCacheStats(t *testing.T) {
	qClearCaches()
	restore := SetMappedQRuntimeKernelDescriptorCacheStatsProvider(func() []qRuntimeKernelDescriptorCacheExternalStatForTest {
		return []qRuntimeKernelDescriptorCacheExternalStatForTest{
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
			{
				Source:     "methodjit_q_frame_runtime",
				Kernel:     "QFrameSelectColumn",
				Shape:      "compare/filter/project/column",
				Route:      "typed_runtime_op_exit",
				SchemaHash: "schema-a",
				Hits:       3,
			},
		}
	}, qRuntimeKernelDescriptorCacheExternalStatToBindForTest)
	defer restore()

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_descriptor_cache")
	if got := row.RawGetString("hits"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_descriptor_cache hits = %v, want 5", got)
	}
	stat := qTestNestedRowByFields(t, row, "stats", map[string]string{
		"source":      "methodjit_q_frame_runtime",
		"kernel":      "QFrameSelectColumn",
		"shape":       "compare/filter/project/column",
		"route":       "typed_runtime_op_exit",
		"schema_hash": "schema-a",
	})
	if got := stat.RawGetString("misses"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("q_runtime_kernel_descriptor_cache mapped misses = %v, want 1", got)
	}
}

func TestMappedQRuntimeKernelDescriptorCacheStatsProviderFilteredFeedsCacheStats(t *testing.T) {
	qClearCaches()
	restore := SetMappedQRuntimeKernelDescriptorCacheStatsProviderFiltered(func() []qRuntimeKernelDescriptorCacheExternalStatForTest {
		return []qRuntimeKernelDescriptorCacheExternalStatForTest{
			{
				Kernel: "QFrameSelectColumn",
				Hits:   9,
			},
			{
				Source:     "methodjit_q_frame_runtime",
				Kernel:     "FrameGroupAggregate",
				Shape:      "filter/group/aggregate",
				Route:      "typed_runtime_op_exit",
				SchemaHash: "schema-b",
				Entries:    1,
				Misses:     1,
			},
		}
	}, func(stat qRuntimeKernelDescriptorCacheExternalStatForTest) (QRuntimeKernelDescriptorCacheStat, bool) {
		if stat.Kernel != "FrameGroupAggregate" {
			return QRuntimeKernelDescriptorCacheStat{}, false
		}
		return qRuntimeKernelDescriptorCacheExternalStatToBindForTest(stat), true
	})
	defer restore()

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_descriptor_cache")
	if got := row.RawGetString("misses"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("q_runtime_kernel_descriptor_cache filtered misses = %v, want 1", got)
	}
	stat := qTestNestedRowByFields(t, row, "stats", map[string]string{
		"kernel":      "FrameGroupAggregate",
		"schema_hash": "schema-b",
	})
	if got := stat.RawGetString("entries"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("q_runtime_kernel_descriptor_cache filtered entries = %v, want 1", got)
	}
}

func TestMappedQRuntimeKernelLoweringStatsProviderFeedsCacheStats(t *testing.T) {
	qClearCaches()
	restore := SetMappedQRuntimeKernelLoweringStatsProvider(func() []qRuntimeKernelLoweringExternalStatForTest {
		return []qRuntimeKernelLoweringExternalStatForTest{
			{
				Source:       "methodjit_q_vector_lowering",
				Kind:         "fallback",
				Kernel:       "QVectorGatherReduce",
				Shape:        "gather/vector-reduce",
				Route:        "lowering",
				Outcome:      "fallback",
				ReasonFamily: "lowering",
				ReasonCode:   "shared_gather",
				Count:        2,
			},
			{
				Source:       "methodjit_q_vector_lowering",
				Kind:         "fallback",
				Kernel:       "QVectorGatherReduce",
				Shape:        "gather/vector-reduce",
				Route:        "lowering",
				Outcome:      "fallback",
				ReasonFamily: "lowering",
				ReasonCode:   "shared_gather",
				Count:        3,
			},
			{
				Source:  "methodjit_q_vector_runtime",
				Kind:    "runtime_kernel",
				Kernel:  "QVectorGatherReduce",
				Shape:   "gather/vector-reduce",
				Route:   "typed_runtime_op_exit",
				Outcome: "supported",
				Count:   4,
			},
		}
	}, qRuntimeKernelLoweringExternalStatToBindForTest)
	defer restore()

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_lowering")
	if got := row.RawGetString("lowerings"); !got.IsInt() || got.Int() != 9 {
		t.Fatalf("q_runtime_kernel_lowering lowerings = %v, want 9", got)
	}
	if got := row.RawGetString("supported"); !got.IsInt() || got.Int() != 4 {
		t.Fatalf("q_runtime_kernel_lowering supported = %v, want 4", got)
	}
	if got := row.RawGetString("fallbacks"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering fallbacks = %v, want 5", got)
	}
	stats := row.RawGetString("stats").Table()
	if stats == nil || stats.Length() != 2 {
		t.Fatalf("q_runtime_kernel_lowering stats table = %v, want two aggregated rows", stats)
	}
	stat := stats.RawGetInt(1).Table()
	if stat == nil {
		t.Fatal("q_runtime_kernel_lowering stats[1] is nil")
	}
	for field, want := range map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"kernel":        "QVectorGatherReduce",
		"shape":         "gather/vector-reduce",
		"route":         "lowering",
		"outcome":       "fallback",
		"reason_family": "lowering",
		"reason_code":   "shared_gather",
	} {
		if got := stat.RawGetString(field); !got.IsString() || got.Str() != want {
			t.Fatalf("q_runtime_kernel_lowering stats[1].%s = %v, want %q", field, got, want)
		}
	}
	if got := stat.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering stats[1].count = %v, want 5", got)
	}
	reasons := row.RawGetString("reasons").Table()
	if reasons == nil || reasons.Length() != 1 {
		t.Fatalf("q_runtime_kernel_lowering reasons table = %v, want one row", reasons)
	}
	reason := reasons.RawGetInt(1).Table()
	if reason == nil {
		t.Fatal("q_runtime_kernel_lowering reasons[1] is nil")
	}
	if got := reason.RawGetString("reason_code"); !got.IsString() || got.Str() != "shared_gather" {
		t.Fatalf("q_runtime_kernel_lowering reasons[1].reason_code = %v, want shared_gather", got)
	}
	if got := reason.RawGetString("kind"); !got.IsString() || got.Str() != "fallback" {
		t.Fatalf("q_runtime_kernel_lowering reasons[1].kind = %v, want fallback", got)
	}
	if got := reason.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering reasons[1].count = %v, want 5", got)
	}
	reasonShapes := row.RawGetString("reason_shapes").Table()
	if reasonShapes == nil || reasonShapes.Length() != 1 {
		t.Fatalf("q_runtime_kernel_lowering reason_shapes table = %v, want one fallback row", reasonShapes)
	}
	reasonShape := qTestNestedRowByFields(t, row, "reason_shapes", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"kernel":        "QVectorGatherReduce",
		"shape":         "gather/vector-reduce",
		"route":         "lowering",
		"outcome":       "fallback",
		"reason_family": "lowering",
		"reason_code":   "shared_gather",
	})
	if got := reasonShape.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering reason_shapes count = %v, want 5", got)
	}
	routes := row.RawGetString("routes").Table()
	if routes == nil || routes.Length() != 2 {
		t.Fatalf("q_runtime_kernel_lowering routes table = %v, want two rows", routes)
	}
	route := qTestNestedRowByFields(t, row, "routes", map[string]string{
		"source":  "methodjit_q_vector_lowering",
		"kind":    "fallback",
		"kernel":  "QVectorGatherReduce",
		"route":   "lowering",
		"outcome": "fallback",
	})
	if got := route.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering fallback route count = %v, want 5", got)
	}
	route = qTestNestedRowByFields(t, row, "routes", map[string]string{
		"source":  "methodjit_q_vector_runtime",
		"kind":    "runtime_kernel",
		"kernel":  "QVectorGatherReduce",
		"route":   "typed_runtime_op_exit",
		"outcome": "supported",
	})
	if got := route.RawGetString("count"); !got.IsInt() || got.Int() != 4 {
		t.Fatalf("q_runtime_kernel_lowering supported route count = %v, want 4", got)
	}
}

func TestMappedQRuntimeKernelLoweringStatsProviderFilteredFeedsCacheStatsWithDefaults(t *testing.T) {
	qClearCaches()
	restore := SetMappedQRuntimeKernelLoweringStatsProviderFiltered(func() []qRuntimeKernelLoweringExternalStatForTest {
		return []qRuntimeKernelLoweringExternalStatForTest{
			{
				Source:  "methodjit_q_vector_runtime",
				Kind:    "runtime_kernel",
				Kernel:  "QVectorGatherReduce",
				Shape:   "gather/vector-reduce",
				Route:   "typed_runtime_op_exit",
				Outcome: "supported",
				Count:   4,
			},
			{
				Source: "methodjit_q_vector_lowering",
				Kernel: "QVectorGatherReduce",
				Shape:  "gather/vector-reduce",
				Count:  5,
			},
		}
	}, func(stat qRuntimeKernelLoweringExternalStatForTest) (QRuntimeKernelLoweringStat, bool) {
		if stat.Outcome == "supported" {
			return QRuntimeKernelLoweringStat{}, false
		}
		return qRuntimeKernelLoweringExternalStatToBindForTest(stat), true
	})
	defer restore()

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_lowering")
	if got := row.RawGetString("lowerings"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering lowerings = %v, want 5", got)
	}
	if got := row.RawGetString("fallbacks"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering fallbacks = %v, want 5", got)
	}
	stats := row.RawGetString("stats").Table()
	if stats == nil || stats.Length() != 1 {
		t.Fatalf("q_runtime_kernel_lowering stats table = %v, want one filtered fallback row", stats)
	}
	stat := qTestNestedRowByFields(t, row, "stats", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"kernel":        "QVectorGatherReduce",
		"shape":         "gather/vector-reduce",
		"route":         "lowering",
		"outcome":       "fallback",
		"reason_family": "lowering",
		"reason_code":   "unspecified",
	})
	if got := stat.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering stats count = %v, want 5", got)
	}
	reasonShape := qTestNestedRowByFields(t, row, "reason_shapes", map[string]string{
		"source":        "methodjit_q_vector_lowering",
		"kind":          "fallback",
		"kernel":        "QVectorGatherReduce",
		"shape":         "gather/vector-reduce",
		"route":         "lowering",
		"outcome":       "fallback",
		"reason_family": "lowering",
		"reason_code":   "unspecified",
	})
	if got := reasonShape.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_lowering reason_shapes count = %v, want 5", got)
	}
}

func qTestNestedRowByFields(t *testing.T, row *Table, field string, want map[string]string) *Table {
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
