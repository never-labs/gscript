//go:build darwin && arm64

package methodjit

import "testing"

type qBenchRouteMetricSpec struct {
	Source        string
	Kernel        string
	SuccessMetric string
	ErrorMetric   string
	RouteMetrics  map[string]string
	ShapeMetric   string
}

func reportQKernelRouteMetrics(b *testing.B, iterations int, stats []QKernelExecutionStat, specs ...qBenchRouteMetricSpec) {
	b.Helper()
	if iterations <= 0 {
		return
	}
	for _, spec := range specs {
		summary := summarizeQKernelRouteMetrics(stats, spec)
		denom := float64(iterations)
		if spec.SuccessMetric != "" {
			b.ReportMetric(float64(summary.success)/denom, spec.SuccessMetric)
		}
		if spec.ErrorMetric != "" {
			b.ReportMetric(float64(summary.errors)/denom, spec.ErrorMetric)
		}
		for route, metric := range spec.RouteMetrics {
			if metric == "" {
				continue
			}
			b.ReportMetric(float64(summary.routes[route])/denom, metric)
		}
		if spec.ShapeMetric != "" {
			b.ReportMetric(float64(len(summary.shapes)), spec.ShapeMetric)
		}
	}
}

type qBenchRouteMetricSummary struct {
	success uint64
	errors  uint64
	routes  map[string]uint64
	shapes  map[string]struct{}
}

func summarizeQKernelRouteMetrics(stats []QKernelExecutionStat, spec qBenchRouteMetricSpec) qBenchRouteMetricSummary {
	out := qBenchRouteMetricSummary{
		routes: make(map[string]uint64),
		shapes: make(map[string]struct{}),
	}
	for _, stat := range stats {
		if stat.Source != spec.Source {
			continue
		}
		if spec.Kernel != "" && stat.Kernel != spec.Kernel {
			continue
		}
		switch stat.Outcome {
		case "success":
			out.success += stat.Count
		case "error":
			out.errors += stat.Count
		}
		if stat.Route != "" {
			out.routes[stat.Route] += stat.Count
		}
		if stat.Shape != "" {
			out.shapes[stat.Shape] = struct{}{}
		}
	}
	return out
}

func reportQEvalPipelineJITRouteBenchmarkStats(b *testing.B, iterations int, stats []QKernelExecutionStat) {
	b.Helper()
	reportQKernelRouteMetrics(b, iterations, stats, qBenchRouteMetricSpec{
		Source:        "methodjit_q_eval_runtime",
		Kernel:        "QEvalPipelinePlan",
		SuccessMetric: "jit_typed_kernel_success/op",
		ErrorMetric:   "jit_typed_kernel_errors/op",
		RouteMetrics: map[string]string{
			"typed_runtime_direct_entry": "jit_typed_direct_return/op",
			"typed_runtime_native_exit":  "jit_typed_native_exit/op",
			"typed_runtime_op_exit":      "jit_typed_op_exit/op",
		},
		ShapeMetric: "jit_typed_pipeline_shapes",
	})
}

func reportMethodJITFrameVectorRouteBenchmarkStats(b *testing.B, iterations int, stats []QKernelExecutionStat) {
	b.Helper()
	reportQKernelRouteMetrics(
		b,
		iterations,
		stats,
		qBenchRouteMetricSpec{
			Source:        "methodjit_q_frame_runtime",
			SuccessMetric: "methodjit_frame_runtime_success/op",
			ErrorMetric:   "methodjit_frame_runtime_errors/op",
			RouteMetrics: map[string]string{
				string(qTypedRuntimeExecutionRouteDirectHelper): "methodjit_frame_runtime_direct_helper/op",
				string(qTypedRuntimeExecutionRouteNativeExit):   "methodjit_frame_runtime_native_exit/op",
				string(qTypedRuntimeExecutionRouteOpExit):       "methodjit_frame_runtime_op_exit/op",
			},
		},
		qBenchRouteMetricSpec{
			Source:        "methodjit_q_vector_runtime",
			SuccessMetric: "methodjit_vector_runtime_success/op",
			ErrorMetric:   "methodjit_vector_runtime_errors/op",
			RouteMetrics: map[string]string{
				string(qTypedRuntimeExecutionRouteDirectHelper): "methodjit_vector_runtime_direct_helper/op",
				string(qTypedRuntimeExecutionRouteNativeExit):   "methodjit_vector_runtime_native_exit/op",
				string(qTypedRuntimeExecutionRouteOpExit):       "methodjit_vector_runtime_op_exit/op",
			},
		},
	)
}

func reportMethodJITQSQLRouteBenchmarkStats(b *testing.B, iterations int, stats []QKernelExecutionStat) {
	b.Helper()
	reportQKernelRouteMetrics(b, iterations, stats, qBenchRouteMetricSpec{
		Source:        QSQLKernelRuntimeSource,
		SuccessMetric: "methodjit_qsql_runtime_success/op",
		ErrorMetric:   "methodjit_qsql_runtime_errors/op",
		RouteMetrics: map[string]string{
			string(qTypedRuntimeExecutionRouteDirectHelper): "methodjit_qsql_runtime_direct_helper/op",
			string(qTypedRuntimeExecutionRouteNativeExit):   "methodjit_qsql_runtime_native_exit/op",
			string(qTypedRuntimeExecutionRouteOpExit):       "methodjit_qsql_runtime_op_exit/op",
			QSQLKernelRuntimeRoute:                          "methodjit_qsql_runtime_backend/op",
		},
		ShapeMetric: "methodjit_qsql_runtime_shapes",
	})
}

func reportQEvalArrayBridgeBenchmarkStats(b *testing.B, iterations, bulkHits, fallbacks, errors, rows int) {
	b.Helper()
	if iterations <= 0 {
		return
	}
	denom := float64(iterations)
	b.ReportMetric(float64(bulkHits)/denom, "q_array_bridge_bulk_hits/op")
	b.ReportMetric(float64(fallbacks)/denom, "q_array_bridge_fallbacks/op")
	b.ReportMetric(float64(errors)/denom, "q_array_bridge_errors/op")
	b.ReportMetric(float64(rows), "q_array_bridge_rows/op")
}

func TestQBenchRouteMetricSummaryFiltersSourceKernelRouteAndShapes(t *testing.T) {
	stats := []QKernelExecutionStat{
		{Source: "methodjit_q_eval_runtime", Kernel: "QEvalPipelinePlan", Shape: "shape-a", Route: "typed_runtime_direct_entry", Outcome: "success", Count: 2},
		{Source: "methodjit_q_eval_runtime", Kernel: "QEvalPipelinePlan", Shape: "shape-a", Route: "typed_runtime_native_exit", Outcome: "error", Count: 1},
		{Source: "methodjit_q_eval_runtime", Kernel: "Other", Shape: "shape-b", Route: "typed_runtime_op_exit", Outcome: "success", Count: 7},
		{Source: "methodjit_q_vector_runtime", Kernel: "QVectorWhereReduce", Shape: "shape-c", Route: "typed_runtime_op_exit", Outcome: "success", Count: 11},
	}
	got := summarizeQKernelRouteMetrics(stats, qBenchRouteMetricSpec{
		Source: "methodjit_q_eval_runtime",
		Kernel: "QEvalPipelinePlan",
	})
	if got.success != 2 || got.errors != 1 {
		t.Fatalf("summary success/errors = %d/%d, want 2/1", got.success, got.errors)
	}
	if got.routes["typed_runtime_direct_entry"] != 2 || got.routes["typed_runtime_native_exit"] != 1 || got.routes["typed_runtime_op_exit"] != 0 {
		t.Fatalf("summary routes = %+v, want direct 2 native 1 op 0", got.routes)
	}
	if len(got.shapes) != 1 {
		t.Fatalf("summary shapes = %+v, want one distinct matching shape", got.shapes)
	}
}

func TestMethodJITFrameVectorRouteMetricsExposeRouteSplit(t *testing.T) {
	stats := []QKernelExecutionStat{
		{Source: "methodjit_q_frame_runtime", Kernel: "FrameColumn", Shape: "column", Route: string(qTypedRuntimeExecutionRouteOpExit), Outcome: "success", Count: 2},
		{Source: "methodjit_q_vector_runtime", Kernel: "QVectorWhereReduce", Shape: "compare/vector-where/vector-reduce", Route: string(qTypedRuntimeExecutionRouteOpExit), Outcome: "success", Count: 3},
		{Source: "methodjit_q_vector_runtime", Kernel: "QVectorWhereReduce", Shape: "compare/vector-where/vector-reduce", Route: string(qTypedRuntimeExecutionRouteNativeExit), Outcome: "success", Count: 5},
	}

	frame := summarizeQKernelRouteMetrics(stats, qBenchRouteMetricSpec{
		Source: "methodjit_q_frame_runtime",
		RouteMetrics: map[string]string{
			string(qTypedRuntimeExecutionRouteOpExit): "methodjit_frame_runtime_op_exit/op",
		},
	})
	vector := summarizeQKernelRouteMetrics(stats, qBenchRouteMetricSpec{
		Source: "methodjit_q_vector_runtime",
		RouteMetrics: map[string]string{
			string(qTypedRuntimeExecutionRouteOpExit):     "methodjit_vector_runtime_op_exit/op",
			string(qTypedRuntimeExecutionRouteNativeExit): "methodjit_vector_runtime_native_exit/op",
		},
	})

	if frame.success != 2 || frame.routes[string(qTypedRuntimeExecutionRouteOpExit)] != 2 {
		t.Fatalf("frame route summary = %+v, want op-exit success 2", frame)
	}
	if vector.success != 8 ||
		vector.routes[string(qTypedRuntimeExecutionRouteOpExit)] != 3 ||
		vector.routes[string(qTypedRuntimeExecutionRouteNativeExit)] != 5 {
		t.Fatalf("vector route summary = %+v, want op-exit 3 native 5", vector)
	}
}

func TestMethodJITQSQLRouteMetricsExposeRouteSplitAndShapes(t *testing.T) {
	stats := []QKernelExecutionStat{
		{Source: QSQLKernelRuntimeSource, Kernel: QSQLKernelName, Shape: "select/where/project", PipelineShape: "pipeline-a", Route: string(qTypedRuntimeExecutionRouteDirectHelper), Outcome: "success", Count: 4},
		{Source: QSQLKernelRuntimeSource, Kernel: QSQLKernelName, Shape: "select/by/aggregate", PipelineShape: "pipeline-b", Route: string(qTypedRuntimeExecutionRouteOpExit), Outcome: "success", Count: 2},
		{Source: QSQLKernelRuntimeSource, Kernel: QSQLKernelName, Shape: "select/by/aggregate", PipelineShape: "pipeline-b", Route: string(qTypedRuntimeExecutionRouteOpExit), Outcome: "error", Count: 1},
		{Source: "methodjit_q_frame_runtime", Kernel: "QFrameSelectColumn", Shape: "compare/filter/project/column", Route: string(qTypedRuntimeExecutionRouteDirectHelper), Outcome: "success", Count: 9},
	}
	got := summarizeQKernelRouteMetrics(stats, qBenchRouteMetricSpec{
		Source: QSQLKernelRuntimeSource,
		RouteMetrics: map[string]string{
			string(qTypedRuntimeExecutionRouteDirectHelper): "methodjit_qsql_runtime_direct_helper/op",
			string(qTypedRuntimeExecutionRouteOpExit):       "methodjit_qsql_runtime_op_exit/op",
		},
		ShapeMetric: "methodjit_qsql_runtime_shapes",
	})
	if got.success != 6 || got.errors != 1 {
		t.Fatalf("qSQL route summary success/errors = %d/%d, want 6/1", got.success, got.errors)
	}
	if got.routes[string(qTypedRuntimeExecutionRouteDirectHelper)] != 4 || got.routes[string(qTypedRuntimeExecutionRouteOpExit)] != 3 {
		t.Fatalf("qSQL route summary routes = %+v, want direct/op 4/3", got.routes)
	}
	if len(got.shapes) != 2 {
		t.Fatalf("qSQL route summary shapes = %+v, want two distinct shapes", got.shapes)
	}
}
