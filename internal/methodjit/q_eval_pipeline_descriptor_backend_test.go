package methodjit

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/stdlib/lib/data"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
	"github.com/never-labs/leia/internal/vm"
)

var qEvalPipelineDescriptorBenchmarkSink runtime.Value

func TestQEvalPipelineRuntimeValueUsesBulkTypedArrayExport(t *testing.T) {
	for _, tc := range []struct {
		name   string
		array  data.Array
		assert func(*testing.T, *runtime.DenseArray)
	}{
		{
			name:  "i64_range",
			array: data.NewI64Range(10, 3, 4),
			assert: func(t *testing.T, dense *runtime.DenseArray) {
				t.Helper()
				got, ok := dense.I64()
				if !ok || !reflect.DeepEqual(got, []int64{10, 13, 16, 19}) {
					t.Fatalf("dense i64 = %v ok %v", got, ok)
				}
			},
		},
		{
			name:  "f64_column",
			array: data.NewF64([]float64{1.25, 2.5, 3.75}),
			assert: func(t *testing.T, dense *runtime.DenseArray) {
				t.Helper()
				got, ok := dense.F64()
				if !ok || !reflect.DeepEqual(got, []float64{1.25, 2.5, 3.75}) {
					t.Fatalf("dense f64 = %v ok %v", got, ok)
				}
			},
		},
		{
			name:  "bool_column",
			array: data.NewBool([]bool{true, false, true}),
			assert: func(t *testing.T, dense *runtime.DenseArray) {
				t.Helper()
				got, ok := dense.Bool()
				if !ok || !reflect.DeepEqual(got, []bool{true, false, true}) {
					t.Fatalf("dense bool = %v ok %v", got, ok)
				}
			},
		},
		{
			name:  "string_column",
			array: data.NewString([]string{"AAPL", "MSFT"}),
			assert: func(t *testing.T, dense *runtime.DenseArray) {
				t.Helper()
				assertDenseArrayStrings(t, dense, []string{"AAPL", "MSFT"})
			},
		},
		{
			name:  "symbol_column",
			array: data.NewSymbols([]string{"AAPL", "MSFT"}),
			assert: func(t *testing.T, dense *runtime.DenseArray) {
				t.Helper()
				assertDenseArrayStrings(t, dense, []string{"AAPL", "MSFT"})
			},
		},
		{
			name:  "encoded_string",
			array: mustQEvalPipelineEncodedArray(t, data.KindString, []any{"AAPL", "MSFT", "NVDA"}, []int32{0, 2, 1, 0}),
			assert: func(t *testing.T, dense *runtime.DenseArray) {
				t.Helper()
				assertDenseArrayStrings(t, dense, []string{"AAPL", "NVDA", "MSFT", "AAPL"})
			},
		},
		{
			name:  "encoded_symbol",
			array: data.NewEncodedSymbols([]data.Symbol{"AAPL", "MSFT", "AAPL", "NVDA"}),
			assert: func(t *testing.T, dense *runtime.DenseArray) {
				t.Helper()
				assertDenseArrayStrings(t, dense, []string{"AAPL", "MSFT", "AAPL", "NVDA"})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, route, err := qEvalPipelineArrayRuntimeValueWithRoute(tc.array)
			if err != nil {
				t.Fatalf("qEvalPipelineRuntimeValue: %v", err)
			}
			if route != qEvalPipelineArrayBridgeRouteBulkTyped {
				t.Fatalf("qEvalPipelineRuntimeValue route = %q, want bulk typed", route)
			}
			if !value.IsDenseArray() {
				t.Fatalf("qEvalPipelineRuntimeValue returned %v, want DenseArray", value)
			}
			tc.assert(t, value.DenseArray())
		})
	}
}

func TestQEvalPipelineRuntimeValueBulkExportsSymbolArrays(t *testing.T) {
	for _, tc := range []struct {
		name  string
		array data.Array
		want  []string
	}{
		{"symbol column", data.NewSymbols([]string{"AAPL", "MSFT"}), []string{"AAPL", "MSFT"}},
		{"encoded symbol", data.NewEncodedSymbols([]data.Symbol{"AAPL", "MSFT", "AAPL"}), []string{"AAPL", "MSFT", "AAPL"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, handled, err := qEvalPipelineTypedArrayRuntimeValue(tc.array)
			if err != nil {
				t.Fatalf("qEvalPipelineTypedArrayRuntimeValue: %v", err)
			}
			if !handled {
				t.Fatal("qEvalPipelineTypedArrayRuntimeValue did not handle symbol array")
			}
			if !value.IsDenseArray() {
				t.Fatalf("symbol array bridge returned %v, want DenseArray", value)
			}
			assertDenseArrayStrings(t, value.DenseArray(), tc.want)
		})
	}
}

func TestQEvalPipelineRuntimeValueKeepsUnsupportedArrayFallback(t *testing.T) {
	array := data.NewAny([]any{int64(7), "AAPL", true})
	value, route, err := qEvalPipelineArrayRuntimeValueWithRoute(array)
	if err != nil {
		t.Fatalf("qEvalPipelineArrayRuntimeValueWithRoute: %v", err)
	}
	if route != qEvalPipelineArrayBridgeRouteFallback {
		t.Fatalf("mixed array bridge route = %q, want fallback", route)
	}
	if !value.IsTable() {
		t.Fatalf("mixed array bridge returned %v, want Table from fallback", value)
	}
	table := value.Table()
	if got := table.RawGetInt(1); !got.IsInt() || got.Int() != 7 {
		t.Fatalf("fallback table row 1 = %v, want int 7", got)
	}
	if got := table.RawGetInt(2); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("fallback table row 2 = %v, want string AAPL", got)
	}
	if got := table.RawGetInt(3); !got.IsBool() || !got.Bool() {
		t.Fatalf("fallback table row 3 = %v, want true", got)
	}
}

func assertDenseArrayStrings(t *testing.T, dense *runtime.DenseArray, want []string) {
	t.Helper()
	if dense == nil || dense.Len() != len(want) {
		t.Fatalf("dense string length = %v, want %d", dense, len(want))
	}
	for i, expected := range want {
		got, err := dense.At(i)
		if err != nil || !got.IsString() || got.String() != expected {
			t.Fatalf("dense string row %d = %v err %v, want %q", i, got, err, expected)
		}
	}
}

func TestQEvalPipelineRuntimeBackendPrefersExecutablePlanOverFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want int64
	}{
		{name: "bin_reduce_sum", src: "+/til 8192 bin til 8192", want: 33550336},
		{name: "modulo_where_count", src: "count where (til 8192 mod 4)=1", want: 2048},
		{name: "script_modulo_gather_reduce", src: "x:til 8192;y:x+1;idx:where (x mod 4)=1;+/y[idx]", want: 8388608},
		{name: "sequence_count_cross", src: "count (til 16) cross til 16", want: 256},
		{name: "sequence_count_trim_take", src: `count trim 10000#" AAPL "`, want: 9999},
		{name: "script_sequence_transform_chain_edge", src: "x:til 16;y:reverse x;z:5 rotate y;w:2 10 sublist z;(+/w)+first w+last w", want: 74},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			sourceCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				return nil, false, errors.New("backend plan fallback should not execute when executable plan is available")
			}
			backend.executeSource = func(source string) (any, bool, error) {
				sourceCalls++
				return nil, false, errors.New("source planner fallback should not execute")
			}

			value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
			if err != nil {
				t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
			}
			if !handled || !value.IsInt() || value.Int() != tc.want {
				t.Fatalf("executeQEvalPipelinePlanValue = %v handled %v, want int %d handled", value, handled, tc.want)
			}
			if backendPlanCalls != 0 || sourceCalls != 0 {
				t.Fatalf("backend plan/source calls = %d/%d, want 0/0", backendPlanCalls, sourceCalls)
			}
		})
	}
}

func TestQEvalPipelineRuntimeBackendExecutesStatsPrimitiveBackendPlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want float64
		// wantInt: integer wsum kind-preserves to long (canonical q).
		wantInt bool
	}{
		{name: "weighted_sum", src: "1 2 3 wsum 10 20 30", want: 140, wantInt: true},
		{name: "correlation", src: "1 2 3 cor 1 2 3", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			assertQEvalPipelinePlanRefView(t, ref, "QPipelinePlan", "", "numeric_stats", qEvalPipelineTypedRuntimeBackend)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				return nil, false, errors.New("backend plan fallback should not execute when executable plan is available")
			}
			backend.executeSource = func(source string) (any, bool, error) {
				return nil, false, errors.New("source planner fallback should not execute")
			}

			value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
			if err != nil {
				t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
			}
			if tc.wantInt {
				if !handled || !value.IsInt() || value.Int() != int64(tc.want) {
					t.Fatalf("executeQEvalPipelinePlanValue = %v handled %v, want int %v handled", value, handled, tc.want)
				}
			} else if !handled || !value.IsFloat() || value.Float() != tc.want {
				t.Fatalf("executeQEvalPipelinePlanValue = %v handled %v, want float %v handled", value, handled, tc.want)
			}
			if backendPlanCalls != 0 {
				t.Fatalf("backend plan calls = %d, want 0", backendPlanCalls)
			}
		})
	}
}

func TestQEvalPipelineRuntimeBackendExecutesMathPrimitiveBackendPlan(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		shape string
		want  float64
	}{
		{name: "sqrt_prefix", src: "sqrt 9", shape: "runtime-unary/sqrt", want: 3},
		{name: "xexp_infix", src: "2 xexp 3", shape: "runtime-dyadic/xexp", want: 8},
		{name: "xlog_call", src: "xlog[2;8]", shape: "runtime-dyadic/xlog", want: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			assertQEvalPipelinePlanRefView(t, ref, "QPipelinePlan", tc.shape, "numeric_math", qEvalPipelineTypedRuntimeBackend)
			descriptor, ok := qEvalPipelineDescriptorViewFromRef(ref)
			if !ok || descriptor.ShapeFamily != "vector" || descriptor.ShapeTransform == "" {
				t.Fatalf("ref = %+v, want numeric math q pipeline shape %q", ref, tc.shape)
			}
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			sourceCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				return nil, false, errors.New("backend plan fallback should not execute when executable plan is available")
			}
			backend.executeSource = func(source string) (any, bool, error) {
				sourceCalls++
				return nil, false, errors.New("source planner fallback should not execute")
			}

			value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
			if err != nil {
				t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
			}
			if !handled || !value.IsFloat() || value.Float() != tc.want {
				t.Fatalf("executeQEvalPipelinePlanValue = %v handled %v, want float %v handled", value, handled, tc.want)
			}
			if backendPlanCalls != 0 || sourceCalls != 0 {
				t.Fatalf("backend plan/source calls = %d/%d, want 0/0", backendPlanCalls, sourceCalls)
			}
		})
	}
}

func TestQEvalPipelineRuntimeBackendExecutesFusedRuntimeShapes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		src           string
		shape         string
		pipelineShape string
		wantInt       int64
		wantFloat     float64
		floatResult   bool
	}{
		{name: "sum_raze", src: "+/raze 2 3#til 6", shape: "vector-reduce/sum-raze", pipelineShape: "vector_reduce", wantInt: 15},
		{name: "sum_dyadic_xexp", src: "+/2 xexp 0 1 2 3", shape: "vector-reduce/sum-dyadic-float-xexp", pipelineShape: "vector_reduce", wantFloat: 15, floatResult: true},
		{name: "sum_dyadic_xlog", src: "+/2 xlog 2 4 8", shape: "vector-reduce/sum-dyadic-float-xlog", pipelineShape: "vector_reduce", wantFloat: 6, floatResult: true},
		{name: "sum_reverse", src: "+/reverse 8#til 4", shape: "vector-reduce/sum-reverse", pipelineShape: "vector_reduce", wantInt: 12},
		{name: "sum_rotate", src: "+/2 rotate 8#til 4", shape: "vector-reduce/sum-rotate", pipelineShape: "vector_reduce", wantInt: 12},
		{name: "sum_sublist", src: "+/2 4 sublist 1+til 8", shape: "vector-reduce/sum-sublist", pipelineShape: "vector_reduce", wantInt: 18},
		{name: "sum_ratios", src: "+/ratios 2 4 8 16", shape: "vector-reduce/sum-ratios", pipelineShape: "vector_reduce", wantFloat: 8, floatResult: true},
		{name: "sum_mdev", src: "+/2 mdev 1 2 3", shape: "vector-reduce/sum-mdev", pipelineShape: "vector_reduce", wantFloat: 1, floatResult: true},
		{name: "sum_ema", src: "+/0.5 ema 1 2 3", shape: "vector-reduce/sum-ema", pipelineShape: "vector_reduce", wantFloat: 4.75, floatResult: true},
		{name: "scalar_at_script", src: "x:10 20 30;x@1", shape: "script-pipeline/apply-index/scalar-at/assignments", pipelineShape: "script_pipeline", wantInt: 20},
		{name: "scalar_dot_script", src: "x:10 20 30;x . 2", shape: "script-pipeline/apply-index/scalar-dot/assignments", pipelineShape: "script_pipeline", wantInt: 30},
		{name: "matrix_dot_cell_script", src: "m:2 3#til 6;m . 1 2", shape: "script-pipeline/apply-index/path-dot/assignments", pipelineShape: "script_pipeline", wantInt: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			assertQEvalPipelinePlanRefView(t, ref, "", tc.shape, tc.pipelineShape, qEvalPipelineTypedRuntimeBackend)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			sourceCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				return nil, false, errors.New("backend plan fallback should not execute when executable plan is available")
			}
			backend.executeSource = func(source string) (any, bool, error) {
				sourceCalls++
				return nil, false, errors.New("source planner fallback should not execute")
			}

			value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
			if err != nil {
				t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
			}
			if !handled {
				t.Fatalf("executeQEvalPipelinePlanValue handled=false")
			}
			if tc.floatResult {
				if !value.IsFloat() || value.Float() != tc.wantFloat {
					t.Fatalf("executeQEvalPipelinePlanValue = %v, want float %v", value, tc.wantFloat)
				}
			} else if !value.IsInt() || value.Int() != tc.wantInt {
				t.Fatalf("executeQEvalPipelinePlanValue = %v, want int %d", value, tc.wantInt)
			}
			if backendPlanCalls != 0 || sourceCalls != 0 {
				t.Fatalf("backend plan/source calls = %d/%d, want 0/0", backendPlanCalls, sourceCalls)
			}
		})
	}
}

func TestQEvalPipelineRuntimeBackendPrefersExecutablePlanByDefault(t *testing.T) {
	for _, tc := range []struct {
		name        string
		src         string
		wantInt     int64
		wantFloat   float64
		floatResult bool
	}{
		{name: "sequence_sum_reverse", src: "+/reverse 8#til 4", wantInt: 12},
		{name: "sequence_sum_ratios", src: "+/ratios 2 4 8 16", wantFloat: 8, floatResult: true},
		{name: "bound_dyadic_xlog_sum", src: "+/2 xlog 2 4 8", wantFloat: 6, floatResult: true},
		{name: "apply_scalar_at", src: "x:10 20 30;x@1", wantInt: 20},
		{name: "matrix_dot_cell", src: "m:2 3#til 6;m . 1 2", wantInt: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			executableCalls := 0
			backend.executeExecutable = func(plan stdq.EvalPipelineExecutablePlan) (any, bool, error) {
				executableCalls++
				return stdq.NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(plan)
			}
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				return nil, false, errors.New("backend plan fallback should not execute")
			}
			backend.executeDescriptor = func(descriptor stdq.EvalPipelineDescriptor) (any, bool, error) {
				return nil, false, errors.New("descriptor fallback should not execute")
			}
			backend.executeSource = func(source string) (any, bool, error) {
				return nil, false, errors.New("source planner fallback should not execute")
			}

			value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
			if err != nil {
				t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
			}
			if !handled {
				t.Fatalf("executeQEvalPipelinePlanValue handled=false")
			}
			if tc.floatResult {
				if !value.IsFloat() || value.Float() != tc.wantFloat {
					t.Fatalf("executeQEvalPipelinePlanValue = %v, want float %v", value, tc.wantFloat)
				}
			} else if !value.IsInt() || value.Int() != tc.wantInt {
				t.Fatalf("executeQEvalPipelinePlanValue = %v, want int %d", value, tc.wantInt)
			}
			if executableCalls != 1 {
				t.Fatalf("executable calls = %d, want 1", executableCalls)
			}
		})
	}
}

func TestQEvalPipelinePlanRefPreservesCallableDotCountDescriptor(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "f:{(+/x)+count y};.[f;(til 8;10#1)]")
	if !ref.IncludeCount || ref.CallableExpr != "f" || ref.ValueExpr != "til 8" || ref.IndexExpr != "10#1" {
		t.Fatalf("plan ref = %+v, want callable/count descriptor fields", ref)
	}
	descriptor, ok := qEvalPipelineDescriptorFromRef(ref)
	if !ok {
		t.Fatalf("qEvalPipelineDescriptorFromRef returned false")
	}
	if !descriptor.IncludeCount || descriptor.CallableExpr != "f" || descriptor.ValueExpr != "til 8" || descriptor.IndexExpr != "10#1" {
		t.Fatalf("descriptor from ref = %+v, want callable/count descriptor fields", descriptor)
	}

	executable, ok := stdq.CompileEvalPipelineBackendPlan(stdq.EvalPipelineBackendPlan{
		Backend:    qEvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	})
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan failed for callable-dot count descriptor")
	}
	out, handled, err := stdq.NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
	if err != nil || !handled || out != int64(38) {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan = %#v,%v,%v; want 38,true,nil", out, handled, err)
	}
}

func TestQEvalPipelinePlanRefPrefersBackendPlanDescriptor(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "+/til 8")
	wantShape := qEvalPipelinePlanRefShape(ref)
	if wantShape == "" || ref.BackendPlan == nil || !ref.BackendPlan.Valid() {
		t.Fatalf("ref = %+v, want embedded backend descriptor", ref)
	}

	ref.Kernel = "stale-kernel"
	ref.Shape = "stale-shape"
	ref.PipelineShape = "stale-pipeline"
	ref.Backend = "stale-backend"
	ref.Kind = "stale-kind"

	if !ref.Valid() {
		t.Fatalf("ref with valid BackendPlan descriptor should remain valid: %+v", ref)
	}
	if got := qEvalPipelinePlanRefKernel(ref); got != ref.BackendPlan.Descriptor.Kernel {
		t.Fatalf("qEvalPipelinePlanRefKernel = %q, want backend descriptor %q", got, ref.BackendPlan.Descriptor.Kernel)
	}
	if got := qEvalPipelinePlanRefShape(ref); got != wantShape {
		t.Fatalf("qEvalPipelinePlanRefShape = %q, want backend descriptor shape %q", got, wantShape)
	}
	if got := qEvalPipelineBackendNameFromRef(ref); got != qEvalPipelineTypedRuntimeBackend {
		t.Fatalf("qEvalPipelineBackendNameFromRef = %q, want typed backend", got)
	}
	if got := qEvalPipelinePlanExecutionShape([]QEvalPipelinePlanRef{ref}, ref.ID); got != wantShape {
		t.Fatalf("qEvalPipelinePlanExecutionShape = %q, want %q", got, wantShape)
	}
	if formatted := formatQEvalPipelinePlanRefs([]QEvalPipelinePlanRef{ref}); strings.Contains(formatted, "stale-") || !strings.Contains(formatted, wantShape) {
		t.Fatalf("formatQEvalPipelinePlanRefs = %q, want backend descriptor fields", formatted)
	}

	descriptor, ok := qEvalPipelineDescriptorFromRef(ref)
	if !ok || descriptor.Shape != wantShape {
		t.Fatalf("qEvalPipelineDescriptorFromRef = %+v,%v; want backend descriptor shape %q", descriptor, ok, wantShape)
	}
	plan, ok := qEvalPipelineBackendPlanFromRef(ref)
	if !ok || plan.Shape() != wantShape || plan.Backend != qEvalPipelineTypedRuntimeBackend {
		t.Fatalf("qEvalPipelineBackendPlanFromRef = %+v,%v; want backend descriptor plan", plan, ok)
	}

	backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
	value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
	if err != nil {
		t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
	}
	if !handled || !value.IsInt() || value.Int() != 28 {
		t.Fatalf("executeQEvalPipelinePlanValue = %v handled %v, want int 28 handled", value, handled)
	}
}

func TestQEvalPipelineBackendPlanFromRefRejectsStaleMirrorWhenSourceRecognized(t *testing.T) {
	const src = "+/til 8"
	ref := qEvalPipelineDescriptorBackendTestRef(t, src)
	ref.BackendPlan = nil
	ref.Source = src
	ref.Kernel = "stale-kernel"
	ref.Shape = "stale-shape"
	ref.PipelineShape = "stale-pipeline"

	if plan, ok := qEvalPipelineBackendPlanFromRef(ref); ok {
		t.Fatalf("qEvalPipelineBackendPlanFromRef accepted stale mirror plan: %+v", plan)
	}
}

func TestQEvalPipelineBackendPlanFromRefKeepsDescriptorOnlyLegacyFallback(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "+/til 8")
	ref.BackendPlan = nil
	ref.Source = ""

	plan, ok := qEvalPipelineBackendPlanFromRef(ref)
	if !ok || !plan.Valid() || plan.Shape() != qEvalPipelinePlanRefShape(ref) {
		t.Fatalf("qEvalPipelineBackendPlanFromRef legacy descriptor = %+v/%v, want valid mirror plan", plan, ok)
	}
	if plan, ok := qEvalPipelineExecutableTypedRuntimeBackendPlanFromRef(ref); ok {
		t.Fatalf("qEvalPipelineExecutableTypedRuntimeBackendPlanFromRef accepted legacy mirror plan: %+v", plan)
	}
	backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
	if plan, ok := backend.lookupBackendPlan(ref); ok {
		t.Fatalf("runtime backend accepted legacy mirror plan: %+v", plan)
	}
	if value, handled, err := executeQEvalPipelinePlanValue(backend, ref); handled || err != nil {
		t.Fatalf("executeQEvalPipelinePlanValue legacy mirror = %v handled %v err %v, want unhandled nil error", value, handled, err)
	}
	helpers := newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{ref}, backend)
	if len(helpers) != 1 {
		t.Fatalf("helper slice length = %d, want 1", len(helpers))
	}
	if helpers[0].validForID(ref.ID) {
		t.Fatalf("legacy mirror helper reported valid: %+v", helpers[0])
	}
}

func TestQEvalPipelineTypedRuntimeBackendPlanRejectsHeuristicPlan(t *testing.T) {
	fn := &Function{}
	ref := fn.addQEvalPipelinePlan("+/unknown", qEvalHeuristicHotPlan("QEvalVectorReduce", "vector-reduce/sum"))

	if ref.BackendPlan != nil {
		t.Fatalf("heuristic ref embedded backend plan: %+v", ref.BackendPlan)
	}
	if plan, ok := qEvalPipelineTypedRuntimeBackendPlanFromRef(ref); ok {
		t.Fatalf("heuristic ref accepted as typed runtime plan: %+v", plan)
	}
}

func TestQEvalPipelineHotPlanPreservesFullBackendDescriptorRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		src    string
		assert func(*testing.T, stdq.EvalPipelineDescriptor)
	}{
		{
			name: "unary_compare",
			src:  "where sqrt 1 4 9 16>2",
			assert: func(t *testing.T, descriptor stdq.EvalPipelineDescriptor) {
				t.Helper()
				if descriptor.UnaryOp != "sqrt" || descriptor.ComparePrefix == "" || descriptor.CompareOp != ">" {
					t.Fatalf("unary descriptor = %+v, want sqrt compare descriptor", descriptor)
				}
			},
		},
		{
			name: "casts",
			src:  "(`long$3.7)+(\"J\"$\"42\")+(\"I\"$\"17\")+(count `$\"AAPL\")+(count string `$\"MSFT\")",
			assert: func(t *testing.T, descriptor stdq.EvalPipelineDescriptor) {
				t.Helper()
				if len(descriptor.CastTerms) != 5 {
					t.Fatalf("cast terms = %+v, want 5 terms", descriptor.CastTerms)
				}
			},
		},
		{
			name: "integer_terms",
			src:  "x:til 1024;y:x+1;(+/y div 3)+(+/y mod 3)+count y",
			assert: func(t *testing.T, descriptor stdq.EvalPipelineDescriptor) {
				t.Helper()
				if len(descriptor.IntegerTerms) != 2 || !descriptor.IncludeCount || descriptor.ValueBinding != "x+1" {
					t.Fatalf("integer descriptor = %+v, want div/mod/count binding descriptor", descriptor)
				}
			},
		},
		{
			name: "assignment_gather",
			src:  "x:til 64;y:x+1;idx:where (x mod 4)=1;+/y[idx]",
			assert: func(t *testing.T, descriptor stdq.EvalPipelineDescriptor) {
				t.Helper()
				if len(descriptor.Assignments) != 3 || descriptor.IndexBinding == "" {
					t.Fatalf("assignment descriptor = %+v, want assignments plus index binding", descriptor)
				}
			},
		},
		{
			name: "callable_dot",
			src:  "f:{(+/x)+count y};.[f;(til 8;10#1)]",
			assert: func(t *testing.T, descriptor stdq.EvalPipelineDescriptor) {
				t.Helper()
				if descriptor.CallableExpr != "f" || !descriptor.IncludeCount {
					t.Fatalf("callable descriptor = %+v, want callable count descriptor", descriptor)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.src
			want, ok := stdq.DescribeEvalPipelineBackendPlan(src)
			if !ok {
				t.Fatalf("DescribeEvalPipelineBackendPlan(%q) failed", src)
			}
			hot, ok := qEvalRuntimePipelinePlan(src)
			if !ok {
				t.Fatalf("qEvalRuntimePipelinePlan(%q) failed", src)
			}
			ref := qEvalPipelinePlanRefFromHotPlan(0, src, hot)
			got, ok := qEvalPipelineBackendPlanFromRef(ref)
			if !ok {
				t.Fatalf("qEvalPipelineBackendPlanFromRef(%q) failed; ref=%+v", src, ref)
			}
			if !reflect.DeepEqual(got.Descriptor, want.Descriptor) {
				t.Fatalf("descriptor round trip mismatch for %q:\n got: %#v\nwant: %#v", src, got.Descriptor, want.Descriptor)
			}
			if got.Backend != want.Backend || got.Detail != want.Detail {
				t.Fatalf("backend plan round trip mismatch for %q: got backend/detail %q/%q want %q/%q",
					src, got.Backend, got.Detail, want.Backend, want.Detail)
			}
			if ref.BackendPlan == nil || !ref.BackendPlan.Valid() {
				t.Fatalf("qEvalPipelinePlanRefFromHotPlan(%q) did not embed backend plan: %+v", src, ref)
			}
			tc.assert(t, got.Descriptor)
		})
	}
}

func TestCompiledFunctionHelperExecutesCallableDotCountExecutablePlan(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "f:{(+/x)+count y};.[f;(til 8;10#1)]")
	backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
	cf := &CompiledFunction{
		QEvalPipelinePlans:       []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend:     qRuntimeEvalPipelineBackend{},
		QEvalPipelinePlanHelpers: newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{ref}, backend),
	}
	if len(cf.QEvalPipelinePlanHelpers) != 1 || !cf.QEvalPipelinePlanHelpers[0].hasExecutablePlan {
		t.Fatalf("compiled helpers = %+v, want executable helper", cf.QEvalPipelinePlanHelpers)
	}

	value, handled, err := cf.ExecuteQEvalPipelinePlanValue(ref.ID)
	if err != nil {
		t.Fatalf("ExecuteQEvalPipelinePlanValue: %v", err)
	}
	if !handled || !value.IsInt() || value.Int() != 38 {
		t.Fatalf("ExecuteQEvalPipelinePlanValue = %v handled %v, want int 38 handled", value, handled)
	}
}

func TestQEvalPipelineExecutionAdapterSharesTypedBackendForDirectAndSlotRoutes(t *testing.T) {
	t.Setenv(exitResumeCheckEnv, "")
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
	executableCalls := 0
	backend.executeExecutable = func(plan stdq.EvalPipelineExecutablePlan) (any, bool, error) {
		if !plan.Valid() {
			t.Fatalf("executeExecutable plan = %+v, want valid executable plan", plan)
		}
		executableCalls++
		return int64(77), true, nil
	}
	backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
		return nil, false, errors.New("backend-plan fallback should not execute when adapter has typed executable helper")
	}
	backend.executeDescriptor = func(descriptor stdq.EvalPipelineDescriptor) (any, bool, error) {
		return nil, false, errors.New("descriptor fallback should not execute when adapter has typed executable helper")
	}
	backend.executeSource = func(source string) (any, bool, error) {
		return nil, false, errors.New("source fallback should not execute when adapter has typed executable helper")
	}
	cf := &CompiledFunction{
		QEvalPipelinePlans:          []QEvalPipelinePlanRef{ref},
		QEvalPipelinePlanHelpers:    newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{ref}, backend),
		QEvalPipelinePlanStats:      newQEvalPipelinePlanExecutionCounters([]QEvalPipelinePlanRef{ref}),
		QEvalPipelineDirectReturn:   true,
		QEvalPipelineDirectReturnID: ref.ID,
	}

	out, handled, err := cf.tryExecuteQEvalPipelineDirectReturnValue()
	if err != nil || !handled || !out.IsInt() || out.Int() != 77 {
		t.Fatalf("direct adapter execution = %v,%v,%v; want int 77,true,nil", out, handled, err)
	}
	regs := []runtime.Value{runtime.NilValue()}
	if err := cf.executeQEvalPipelinePlanSlot(ref.ID, 0, regs, qEvalPipelineExecutionRouteNativeExit); err != nil {
		t.Fatalf("slot adapter execution: %v", err)
	}
	if !regs[0].IsInt() || regs[0].Int() != 77 {
		t.Fatalf("slot adapter result = %v, want int 77", regs[0])
	}
	if executableCalls != 2 {
		t.Fatalf("typed executable calls = %d, want direct and slot routes to share backend helper", executableCalls)
	}
	stats := cf.QKernelExecutionStats()
	shape := qEvalPipelinePlanRefShape(ref)
	assertQEvalPipelineExecutionStat(t, stats, shape, "typed_runtime_direct_entry", "success", 1)
	assertQEvalPipelineExecutionStat(t, stats, shape, "typed_runtime_native_exit", "success", 1)
}

func TestQEvalPipelineLoweringRecognizesMathRuntimePrimitive(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		shape string
	}{
		{name: "sqrt_prefix", src: "sqrt 9", shape: "runtime-unary/sqrt"},
		{name: "xexp_infix", src: "2 xexp 3", shape: "runtime-dyadic/xexp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fn := BuildGraph(qEvalPipelineDescriptorBackendConstProto(tc.src))
			lowered, err := QEvalPipelineLoweringPass(fn)
			if err != nil {
				t.Fatalf("QEvalPipelineLoweringPass: %v", err)
			}
			if counts := countOps(lowered); counts[OpQEvalPipelinePlan] != 1 || counts[OpCall] != 0 {
				t.Fatalf("op counts after q eval pipeline lowering: QEvalPipelinePlan=%d OpCall=%d\n%s", counts[OpQEvalPipelinePlan], counts[OpCall], Print(lowered))
			}
			if len(lowered.QEvalPipelinePlans) != 1 {
				t.Fatalf("QEvalPipelinePlans = %+v, want one math primitive plan", lowered.QEvalPipelinePlans)
			}
			ref := lowered.QEvalPipelinePlans[0]
			assertQEvalPipelinePlanRefView(t, ref, "", tc.shape, "numeric_math", qEvalPipelineTypedRuntimeBackend)
		})
	}
}

func TestQEvalPipelineLoweringRecognizesFusedRuntimeShapes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		src           string
		shape         string
		pipelineShape string
	}{
		{name: "sum_raze", src: "+/raze 2 3#til 6", shape: "vector-reduce/sum-raze", pipelineShape: "vector_reduce"},
		{name: "sum_dyadic_xexp", src: "+/2 xexp 0 1 2 3", shape: "vector-reduce/sum-dyadic-float-xexp", pipelineShape: "vector_reduce"},
		{name: "sum_dyadic_xlog", src: "+/2 xlog 2 4 8", shape: "vector-reduce/sum-dyadic-float-xlog", pipelineShape: "vector_reduce"},
		{name: "sum_reverse", src: "+/reverse 8#til 4", shape: "vector-reduce/sum-reverse", pipelineShape: "vector_reduce"},
		{name: "sum_rotate", src: "+/2 rotate 8#til 4", shape: "vector-reduce/sum-rotate", pipelineShape: "vector_reduce"},
		{name: "sum_sublist", src: "+/2 4 sublist 1+til 8", shape: "vector-reduce/sum-sublist", pipelineShape: "vector_reduce"},
		{name: "sum_ratios", src: "+/ratios 2 4 8 16", shape: "vector-reduce/sum-ratios", pipelineShape: "vector_reduce"},
		{name: "sum_mdev", src: "+/2 mdev 1 2 3", shape: "vector-reduce/sum-mdev", pipelineShape: "vector_reduce"},
		{name: "sum_ema", src: "+/0.5 ema 1 2 3", shape: "vector-reduce/sum-ema", pipelineShape: "vector_reduce"},
		{name: "scalar_at_script", src: "x:10 20 30;x@1", shape: "script-pipeline/apply-index/scalar-at/assignments", pipelineShape: "script_pipeline"},
		{name: "matrix_dot_row_script", src: "m:2 3#til 6;m . 1", shape: "script-pipeline/apply-index/scalar-dot/assignments", pipelineShape: "script_pipeline"},
		{name: "matrix_dot_cell_script", src: "m:2 3#til 6;m . 1 2", shape: "script-pipeline/apply-index/path-dot/assignments", pipelineShape: "script_pipeline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fn := BuildGraph(qEvalPipelineDescriptorBackendConstProto(tc.src))
			lowered, err := QEvalPipelineLoweringPass(fn)
			if err != nil {
				t.Fatalf("QEvalPipelineLoweringPass: %v", err)
			}
			if counts := countOps(lowered); counts[OpQEvalPipelinePlan] != 1 || counts[OpCall] != 0 {
				t.Fatalf("op counts after q eval pipeline lowering: QEvalPipelinePlan=%d OpCall=%d\n%s", counts[OpQEvalPipelinePlan], counts[OpCall], Print(lowered))
			}
			if len(lowered.QEvalPipelinePlans) != 1 {
				t.Fatalf("QEvalPipelinePlans = %+v, want one fused runtime plan", lowered.QEvalPipelinePlans)
			}
			ref := lowered.QEvalPipelinePlans[0]
			assertQEvalPipelinePlanRefView(t, ref, "", tc.shape, tc.pipelineShape, qEvalPipelineTypedRuntimeBackend)
		})
	}
}

func TestQEvalPipelineLoweringCoversRuntimeFallbackClearedShapes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		src           string
		pipelineShape string
	}{
		{
			name:          "bulk_float_compare_indexes",
			src:           "x:(til 80)*0.25;idx:where x>16.5;(+/x[idx])+count idx",
			pipelineShape: "script_pipeline",
		},
		{
			name:          "nullable_tiled_within_stats",
			src:           "x:80#0N 2 5 9;v:til 80;idx:where x within 2 8;(+/v[idx])+count idx",
			pipelineShape: "script_pipeline",
		},
		{
			name:          "symbol_find_sum",
			src:           "+/`AAPL`MSFT`NVDA?`MSFT`TSLA",
			pipelineShape: "vector_reduce",
		},
		{
			name:          "symbol_find_indexes",
			src:           "`AAPL`MSFT`NVDA?`MSFT`TSLA",
			pipelineShape: "find",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fn := BuildGraph(qEvalPipelineDescriptorBackendConstProto(tc.src))
			lowered, err := QEvalPipelineLoweringPass(fn)
			if err != nil {
				t.Fatalf("QEvalPipelineLoweringPass: %v", err)
			}
			if counts := countOps(lowered); counts[OpQEvalPipelinePlan] != 1 || counts[OpCall] != 0 {
				t.Fatalf("op counts after q eval pipeline lowering: QEvalPipelinePlan=%d OpCall=%d\n%s", counts[OpQEvalPipelinePlan], counts[OpCall], Print(lowered))
			}
			if len(lowered.QEvalPipelinePlans) != 1 {
				t.Fatalf("QEvalPipelinePlans = %+v, want one runtime fallback-cleared plan", lowered.QEvalPipelinePlans)
			}
			ref := lowered.QEvalPipelinePlans[0]
			assertQEvalPipelinePlanRefView(t, ref, "", "", tc.pipelineShape, qEvalPipelineTypedRuntimeBackend)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
			if err != nil {
				t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
			}
			if !handled || value.IsNil() {
				t.Fatalf("executeQEvalPipelinePlanValue = %v handled %v, want handled non-nil", value, handled)
			}
		})
	}
}

func TestCompiledFunctionUsesPredecodedQEvalPipelineBackend(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "x:til 8192;y:x+1;idx:where (x mod 4)=1;+/y[idx]")
	cf := &CompiledFunction{
		QEvalPipelinePlans:   []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend: newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
	}
	backendPlanCalls := 0
	cf.QEvalPipelineBackend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
		backendPlanCalls++
		return nil, false, errors.New("backend plan fallback should not execute when executable plan is available")
	}
	cf.QEvalPipelineBackend.executeSource = func(source string) (any, bool, error) {
		return nil, false, errors.New("source planner fallback should not execute")
	}

	value, handled, err := cf.ExecuteQEvalPipelinePlanValue(ref.ID)
	if err != nil {
		t.Fatalf("ExecuteQEvalPipelinePlanValue: %v", err)
	}
	if !handled || !value.IsInt() || value.Int() != 8388608 {
		t.Fatalf("ExecuteQEvalPipelinePlanValue = %v handled %v, want int 8388608 handled", value, handled)
	}
	if backendPlanCalls != 0 {
		t.Fatalf("backend plan calls = %d, want compiled function executable backend to be reused", backendPlanCalls)
	}
}

func TestCompiledFunctionUsesQEvalPipelineHelperSlot(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
	backendPlanCalls := 0
	backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
		backendPlanCalls++
		return nil, false, errors.New("backend plan fallback should not execute when helper has executable plan")
	}
	backend.executeDescriptor = func(descriptor stdq.EvalPipelineDescriptor) (any, bool, error) {
		return nil, false, errors.New("descriptor fallback should not execute when helper has executable plan")
	}
	backend.executeSource = func(source string) (any, bool, error) {
		return nil, false, errors.New("source planner fallback should not execute when helper has executable plan")
	}
	helpers := newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{ref}, backend)
	if len(helpers) != 1 || !helpers[0].hasExecutablePlan || helpers[0].evalState == nil {
		t.Fatalf("helper = %+v, want executable helper with reusable eval state", helpers)
	}
	cf := &CompiledFunction{
		QEvalPipelinePlans:       []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend:     qRuntimeEvalPipelineBackend{},
		QEvalPipelinePlanHelpers: helpers,
	}

	value, handled, err := cf.ExecuteQEvalPipelinePlanValue(ref.ID)
	if err != nil {
		t.Fatalf("ExecuteQEvalPipelinePlanValue: %v", err)
	}
	if !handled || !value.IsInt() || value.Int() != 16 {
		t.Fatalf("ExecuteQEvalPipelinePlanValue = %v handled %v, want int 16 handled", value, handled)
	}
	if backendPlanCalls != 0 {
		t.Fatalf("backend plan calls = %d, want helper executable plan execution", backendPlanCalls)
	}
}

func TestQEvalPipelineHelperReusableEvalStateRequiresDefaultExecutableRuntime(t *testing.T) {
	sourceRef := qEvalPipelineSourceBackedTestRef(t, "count where (til 64 mod 4)=1")
	sourceBackend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{sourceRef})
	sourceHelpers := newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{sourceRef}, sourceBackend)
	if len(sourceHelpers) != 1 || sourceHelpers[0].evalState == nil {
		t.Fatalf("source-backed helper evalState = %+v, want reusable eval state", sourceHelpers)
	}

	descriptorRef := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	descriptorBackend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{descriptorRef})
	descriptorHelpers := newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{descriptorRef}, descriptorBackend)
	if len(descriptorHelpers) != 1 || descriptorHelpers[0].evalState == nil || !descriptorHelpers[0].hasExecutablePlan {
		t.Fatalf("descriptor-only helper evalState/executable = %+v, want reusable executable eval state", descriptorHelpers)
	}

	customBackend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{sourceRef})
	customBackend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
		return int64(16), true, nil
	}
	customHelpers := newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{sourceRef}, customBackend)
	if len(customHelpers) != 1 || customHelpers[0].evalState == nil || !customHelpers[0].hasExecutablePlan {
		t.Fatalf("custom backend-plan executor helper = %+v, want executable helper to keep reusable eval state", customHelpers)
	}

	customExecutableBackend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{sourceRef})
	customExecutableBackend.executeExecutable = func(plan stdq.EvalPipelineExecutablePlan) (any, bool, error) {
		if !plan.Valid() {
			t.Fatalf("custom executable plan = %+v, want valid", plan)
		}
		return int64(99), true, nil
	}
	customExecutableHelpers := newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{sourceRef}, customExecutableBackend)
	if len(customExecutableHelpers) != 1 || customExecutableHelpers[0].evalState != nil || !customExecutableHelpers[0].hasExecutablePlan {
		t.Fatalf("custom executable helper = %+v, want executable helper without reusable eval state", customExecutableHelpers)
	}
	value, handled, err := customExecutableHelpers[0].execute()
	if err != nil || !handled || !value.IsInt() || value.Int() != 99 {
		t.Fatalf("custom executable helper execute = %v,%v,%v; want int 99,true,nil", value, handled, err)
	}
}

func BenchmarkQEvalPipelineArrayRuntimeBridge(b *testing.B) {
	const rows = 8192
	cases := []struct {
		name      string
		array     data.Array
		wantRoute qEvalPipelineArrayBridgeRoute
	}{
		{name: "BulkI64Range", array: data.NewI64Range(0, 1, rows), wantRoute: qEvalPipelineArrayBridgeRouteBulkTyped},
		{name: "BulkF64Column", array: data.NewF64(makeF64BenchmarkColumn(rows)), wantRoute: qEvalPipelineArrayBridgeRouteBulkTyped},
		{name: "BulkBoolColumn", array: data.NewBool(makeBoolBenchmarkColumn(rows)), wantRoute: qEvalPipelineArrayBridgeRouteBulkTyped},
		{name: "BulkStringColumn", array: data.NewString(makeStringBenchmarkColumn(rows)), wantRoute: qEvalPipelineArrayBridgeRouteBulkTyped},
		{name: "BulkEncodedSymbol", array: makeEncodedSymbolBenchmarkColumn(rows), wantRoute: qEvalPipelineArrayBridgeRouteBulkTyped},
		{name: "FallbackMixedAny", array: makeMixedAnyBenchmarkColumn(rows), wantRoute: qEvalPipelineArrayBridgeRouteFallback},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			var bulkHits, fallbacks, errors int
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value, route, err := qEvalPipelineArrayRuntimeValueWithRoute(tc.array)
				if err != nil {
					errors++
					b.Fatalf("qEvalPipelineArrayRuntimeValueWithRoute: %v", err)
				}
				if route != tc.wantRoute {
					errors++
					b.Fatalf("bridge route = %q, want %q", route, tc.wantRoute)
				}
				switch route {
				case qEvalPipelineArrayBridgeRouteBulkTyped:
					bulkHits++
				case qEvalPipelineArrayBridgeRouteFallback:
					fallbacks++
				}
				qEvalPipelineDescriptorBenchmarkSink = value
			}
			reportQEvalArrayBridgeBenchmarkStats(b, b.N, bulkHits, fallbacks, errors, tc.array.Len())
		})
	}
}

func makeF64BenchmarkColumn(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i) * 1.25
	}
	return out
}

func makeBoolBenchmarkColumn(n int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = i%3 == 0
	}
	return out
}

func makeStringBenchmarkColumn(n int) []string {
	seed := []string{"AAPL", "MSFT", "NVDA", "TSLA", "META", "AMZN", "GOOG", "ORCL"}
	out := make([]string, n)
	for i := range out {
		out[i] = seed[i%len(seed)]
	}
	return out
}

func makeEncodedSymbolBenchmarkColumn(n int) data.Array {
	values := make([]data.Symbol, n)
	for i, value := range makeStringBenchmarkColumn(n) {
		values[i] = data.Symbol(value)
	}
	return data.NewEncodedSymbols(values)
}

func makeMixedAnyBenchmarkColumn(n int) data.Array {
	seed := []string{"AAPL", "MSFT", "NVDA", "TSLA", "META", "AMZN", "GOOG", "ORCL"}
	values := make([]any, n)
	for i := range values {
		switch i % 3 {
		case 0:
			values[i] = int64(i)
		case 1:
			values[i] = seed[i%len(seed)]
		default:
			values[i] = i%2 == 0
		}
	}
	return data.NewAny(values)
}

func mustQEvalPipelineEncodedArray(t *testing.T, kind data.Kind, domain []any, codes []int32) data.Array {
	t.Helper()
	array, err := data.NewEncoded(kind, domain, codes)
	if err != nil {
		t.Fatalf("NewEncoded returned error: %v", err)
	}
	return array
}

func BenchmarkQEvalPipelineDescriptorBackend(b *testing.B) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{name: "BinReduceSum", src: "+/til 8192 bin til 8192"},
		{name: "ModuloWhereCount", src: "count where (til 8192 mod 4)=1"},
		{name: "ScriptModuloGatherReduce", src: "x:til 8192;y:x+1;idx:where (x mod 4)=1;+/y[idx]"},
		{name: "SumRaze", src: "+/raze 128 64#til 8192"},
		{name: "SumDyadicXExp", src: "+/2 xexp til 16"},
		{name: "ScalarApplyIndex", src: "x:til 8192;x@4096"},
	} {
		b.Run("BackendPlan/"+tc.name, func(b *testing.B) {
			ref := qEvalPipelineDescriptorBackendTestRef(b, tc.src)
			cf := &CompiledFunction{
				QEvalPipelinePlans:   []QEvalPipelinePlanRef{ref},
				QEvalPipelineBackend: newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value, handled, err := cf.ExecuteQEvalPipelinePlanValue(ref.ID)
				if err != nil || !handled {
					b.Fatalf("executeQEvalPipelinePlanValue handled=%v err=%v", handled, err)
				}
				qEvalPipelineDescriptorBenchmarkSink = value
			}
		})
		b.Run("HelperSlot/"+tc.name, func(b *testing.B) {
			ref := qEvalPipelineDescriptorBackendTestRef(b, tc.src)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			cf := &CompiledFunction{
				QEvalPipelinePlans:       []QEvalPipelinePlanRef{ref},
				QEvalPipelineBackend:     backend,
				QEvalPipelinePlanHelpers: newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{ref}, backend),
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value, handled, err := cf.ExecuteQEvalPipelinePlanValue(ref.ID)
				if err != nil || !handled {
					b.Fatalf("ExecuteQEvalPipelinePlanValue handled=%v err=%v", handled, err)
				}
				qEvalPipelineDescriptorBenchmarkSink = value
			}
		})
		b.Run("SourcePlanner/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, handled, err := stdq.ExecuteEvalPipeline(tc.src)
				if err != nil || !handled {
					b.Fatalf("ExecuteEvalPipeline handled=%v err=%v", handled, err)
				}
				value, err := qEvalPipelineRuntimeValue(out)
				if err != nil {
					b.Fatalf("qEvalPipelineRuntimeValue: %v", err)
				}
				qEvalPipelineDescriptorBenchmarkSink = value
			}
		})
	}
}

func qEvalPipelineDescriptorBackendTestRef(tb testing.TB, source string) QEvalPipelinePlanRef {
	tb.Helper()
	plan, ok := qEvalRuntimePipelinePlan(source)
	if !ok {
		tb.Fatalf("DescribeQEvalPipeline(%q) did not recognize pipeline", source)
	}
	fn := &Function{}
	ref := fn.addQEvalPipelinePlan("not a q pipeline", plan)
	if ref.BackendPlan == nil || !ref.BackendPlan.Valid() {
		tb.Fatalf("q eval pipeline ref = %+v, want embedded q backend plan", ref)
	}
	return ref
}

func assertQEvalPipelinePlanRefView(tb testing.TB, ref QEvalPipelinePlanRef, kernel, shape, pipelineShape, backend string) {
	tb.Helper()
	if kernel != "" && qEvalPipelinePlanRefKernel(ref) != kernel {
		tb.Fatalf("q eval pipeline ref kernel = %q, want %q; ref=%+v", qEvalPipelinePlanRefKernel(ref), kernel, ref)
	}
	if shape != "" && qEvalPipelinePlanRefShape(ref) != shape {
		tb.Fatalf("q eval pipeline ref shape = %q, want %q; ref=%+v", qEvalPipelinePlanRefShape(ref), shape, ref)
	}
	if pipelineShape != "" && qEvalPipelinePlanRefPipelineShape(ref) != pipelineShape {
		tb.Fatalf("q eval pipeline ref pipeline shape = %q, want %q; ref=%+v", qEvalPipelinePlanRefPipelineShape(ref), pipelineShape, ref)
	}
	if backend != "" && qEvalPipelineBackendNameFromRef(ref) != backend {
		tb.Fatalf("q eval pipeline ref backend = %q, want %q; ref=%+v", qEvalPipelineBackendNameFromRef(ref), backend, ref)
	}
}

func qEvalPipelineSourceBackedTestRef(tb testing.TB, source string) QEvalPipelinePlanRef {
	tb.Helper()
	ref := qEvalPipelineDescriptorBackendTestRef(tb, source)
	ref.Source = source
	return ref
}

func qEvalPipelineDescriptorBackendConstProto(source string) *vm.FuncProto {
	return &vm.FuncProto{
		Name:     "q_eval_pipeline_descriptor_backend_const",
		MaxStack: 2,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("eval"),
			runtime.StringValue(source),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 0, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 0, 0, 1),
			vm.EncodeABx(vm.OP_LOADK, 1, 2),
			vm.EncodeABC(vm.OP_CALL, 0, 2, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
}
