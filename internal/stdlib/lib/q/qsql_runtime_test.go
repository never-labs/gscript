package q

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestQSQLRuntimeDescriptorClassifiesReadPipelines(t *testing.T) {
	tests := []struct {
		name             string
		src              string
		wantShape        string
		wantPipelinePart string
		wantSupported    bool
	}{
		{
			name:             "filter project",
			src:              "select notional:size*price from trades where size>=20",
			wantShape:        "qsql/select/filtered_projection|where=typed_column_literal|projection=typed_binary",
			wantPipelinePart: "where=compare_mask(compare_mask:column_literal)|filter=index|project=typed_expr(typed_binary:1)",
			wantSupported:    true,
		},
		{
			name:             "group aggregate",
			src:              "select total:sum size by sym from trades",
			wantShape:        "qsql/select/grouped_aggregate|by=computed|aggregate=typed_column",
			wantPipelinePart: "group=key_expr(column_load:1)|aggregate=column_reduce(column_load:1,reduce:sum:1)",
			wantSupported:    true,
		},
		{
			name:             "join descriptor fallback",
			src:              "select sym,price,bid from trades join quotes on sym",
			wantShape:        "qsql/select/projection|projection=columns|join=inner/keys-1/count-1",
			wantPipelinePart: "join=inner/keys-1/count-1|scan=frame|project=column_load(column_load:3)",
			wantSupported:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lowered, err := Lower(mustParse(t, tt.src))
			if err != nil {
				t.Fatalf("Lower returned error: %v", err)
			}
			descriptor := lowered.RuntimeDescriptor()
			if !descriptor.Valid() {
				t.Fatalf("RuntimeDescriptor invalid: %+v", descriptor)
			}
			if descriptor.Shape != tt.wantShape {
				t.Fatalf("shape = %q, want %q", descriptor.Shape, tt.wantShape)
			}
			if !strings.Contains(descriptor.PipelineShape, tt.wantPipelinePart) {
				t.Fatalf("pipeline shape = %q, want it to contain %q", descriptor.PipelineShape, tt.wantPipelinePart)
			}
			if descriptor.Supported != tt.wantSupported {
				t.Fatalf("supported = %v, want %v reason=%q", descriptor.Supported, tt.wantSupported, descriptor.Reason)
			}
			if got := descriptor.Pipeline.String(); got != descriptor.PipelineShape {
				t.Fatalf("structured pipeline string = %q, want descriptor pipeline %q", got, descriptor.PipelineShape)
			}
			if descriptor.QueryShape == "" {
				t.Fatalf("query shape missing from descriptor: %+v", descriptor)
			}
			if !descriptor.Supported && descriptor.Unsupported.ReasonCode == "" {
				t.Fatalf("unsupported reason code missing from descriptor: %+v", descriptor)
			}
		})
	}
}

func TestQSQLRuntimeDescriptorCarriesStructuredJoinFallback(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select sym,price,bid from trades join quotes on sym"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	descriptor := lowered.RuntimeDescriptor()
	if descriptor.Pipeline.Join.Count != 1 {
		t.Fatalf("join count = %d, want 1 descriptor=%+v", descriptor.Pipeline.Join.Count, descriptor)
	}
	if descriptor.Pipeline.Join.KeyCount != 1 {
		t.Fatalf("join key count = %d, want 1 descriptor=%+v", descriptor.Pipeline.Join.KeyCount, descriptor)
	}
	if got, want := descriptor.Pipeline.Join.Kinds, []string{"inner"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("join kinds = %#v, want %#v", got, want)
	}
	if descriptor.Unsupported.ReasonCode != RuntimeFallbackPlannerUnhandled {
		t.Fatalf("unsupported reason code = %q, want %q reason=%q", descriptor.Unsupported.ReasonCode, RuntimeFallbackPlannerUnhandled, descriptor.Unsupported.Reason)
	}
	if !strings.Contains(descriptor.Unsupported.Reason, "join plan") {
		t.Fatalf("unsupported reason = %q, want join-specific reason", descriptor.Unsupported.Reason)
	}
}

func TestQSQLExecRuntimeUsesTypedBackendAndExplicitPipelineStats(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	frame := mustQSQLRuntimeFrame(t)
	lowered, err := Lower(mustParse(t, "select notional:size*price from trades where size>=20"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	descriptor := lowered.RuntimeDescriptor()
	out, err := lowered.ExecRuntime(frame)
	if err != nil {
		t.Fatalf("ExecRuntime returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("out len = %d, want 2", out.Len())
	}
	assertColumnValue(t, out, "notional", 0, 2000.0)
	assertColumnValue(t, out, "notional", 1, 2625.0)

	var attempts, hits uint64
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Source != qSQLRuntimeSource || stat.Kernel != qSQLPlanKernel || stat.Shape != descriptor.Shape {
			continue
		}
		if stat.PipelineShape != descriptor.PipelineShape {
			t.Fatalf("stat pipeline shape = %q, want descriptor %q", stat.PipelineShape, descriptor.PipelineShape)
		}
		if stat.Route != qSQLRuntimeRouteTypedBackend {
			t.Fatalf("typed qSQL stat route = %q, want %q", stat.Route, qSQLRuntimeRouteTypedBackend)
		}
		switch stat.Outcome {
		case "attempt":
			attempts += stat.Count
		case "hit":
			hits += stat.Count
		case "fallback", "error":
			t.Fatalf("unexpected typed qSQL outcome: %+v", stat)
		}
	}
	if attempts != 1 || hits != 1 {
		t.Fatalf("qSQL typed backend attempts=%d hits=%d stats=%#v", attempts, hits, RuntimeKernelExecutionStats())
	}
}

func TestQSQLExecRuntimeFallbackIsObservableWithPipelineShape(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	frame := mustQSQLRuntimeFrame(t)
	lowered := &Lowered{
		Op: SelectQuery,
		Plan: data.QueryPlan{
			Select: []data.SelectItem{{Name: "marker", Expr: qSQLRuntimeUnsupportedExpr{Value: data.Symbol("fallback")}}},
			LimitN: -1,
		},
	}
	descriptor := lowered.RuntimeDescriptor()
	if descriptor.Supported {
		t.Fatalf("descriptor supported = true, want unsupported custom select expression")
	}
	out, err := lowered.ExecRuntime(frame)
	if err != nil {
		t.Fatalf("ExecRuntime fallback returned error: %v", err)
	}
	if out.Len() != frame.Len() {
		t.Fatalf("fallback output len = %d, want %d", out.Len(), frame.Len())
	}
	assertColumnValue(t, out, "marker", 0, data.Symbol("fallback"))

	var fallback uint64
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Source != qSQLRuntimeSource || stat.Kernel != qSQLPlanKernel || stat.Shape != descriptor.Shape || stat.Outcome != "fallback" {
			continue
		}
		if stat.Route != qSQLRuntimeRoutePlanFallback {
			t.Fatalf("fallback route = %q, want %q", stat.Route, qSQLRuntimeRoutePlanFallback)
		}
		if stat.PipelineShape != descriptor.PipelineShape {
			t.Fatalf("fallback pipeline shape = %q, want %q", stat.PipelineShape, descriptor.PipelineShape)
		}
		fallback += stat.Count
	}
	if fallback != 1 {
		t.Fatalf("fallback count = %d, want 1 stats=%#v", fallback, RuntimeKernelExecutionStats())
	}
}

func TestQSQLExecRuntimeBackendUnsupportedFallbackStatsAreExplicit(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	frame := mustQSQLRuntimeFrame(t)
	lowered, err := Lower(mustParse(t, "select notional:size*price from trades where size>=20"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	descriptor := lowered.RuntimeDescriptor()
	out, err := lowered.execRuntime(frame, qSQLRuntimeUnsupportedBackend{})
	if err != nil {
		t.Fatalf("ExecRuntime fallback returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("fallback output len = %d, want 2", out.Len())
	}

	var attempts, fallbacks uint64
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Source != qSQLRuntimeSource || stat.Kernel != qSQLPlanKernel || stat.Shape != descriptor.Shape {
			continue
		}
		switch stat.Outcome {
		case "attempt":
			if stat.Route != "test_backend" {
				t.Fatalf("attempt route = %q, want test_backend", stat.Route)
			}
			attempts += stat.Count
		case "fallback":
			if stat.Route != qSQLRuntimeRoutePlanFallback {
				t.Fatalf("fallback route = %q, want %q", stat.Route, qSQLRuntimeRoutePlanFallback)
			}
			if stat.ReasonCode != RuntimeFallbackUnsupportedShape {
				t.Fatalf("fallback reason code = %q, want %q", stat.ReasonCode, RuntimeFallbackUnsupportedShape)
			}
			fallbacks += stat.Count
		}
	}
	if attempts != 1 || fallbacks != 1 {
		t.Fatalf("backend attempts=%d fallbacks=%d stats=%#v", attempts, fallbacks, RuntimeKernelExecutionStats())
	}
}

type qSQLRuntimeUnsupportedExpr struct {
	Value any
}

func (e qSQLRuntimeUnsupportedExpr) EvalRow(data.Frame, int) (any, error) {
	return e.Value, nil
}

type qSQLRuntimeUnsupportedBackend struct{}

func (qSQLRuntimeUnsupportedBackend) Route() string {
	return "test_backend"
}

func (qSQLRuntimeUnsupportedBackend) Compile(data.Frame, data.QueryPlan, QSQLRuntimeDescriptor) (qSQLRuntimeExecutable, bool, error) {
	return nil, false, nil
}

func mustQSQLRuntimeFrame(t *testing.T) data.Frame {
	t.Helper()
	frame, err := data.NewFrame(
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL")}),
		data.NewColumn("price", []any{100.0, 80.0, 87.5}),
		data.NewColumn("size", []any{10, 25, 30}),
	)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}
	return frame
}
