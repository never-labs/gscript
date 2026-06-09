package methodjit

import (
	"errors"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
	"github.com/never-labs/leia/internal/vm"
)

var qEvalPipelineDescriptorBenchmarkSink runtime.Value

func TestQEvalPipelineRuntimeBackendPrefersBackendPlanOverSourcePlanner(t *testing.T) {
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			sourceCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				if plan.Descriptor.Source != "not a q pipeline" {
					t.Fatalf("backend plan Source = %q, want preserved ref source sentinel", plan.Descriptor.Source)
				}
				return stdq.ExecuteEvalPipelineBackendPlan(plan)
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
			if backendPlanCalls != 1 || sourceCalls != 0 {
				t.Fatalf("backend plan/source calls = %d/%d, want 1/0", backendPlanCalls, sourceCalls)
			}
		})
	}
}

func TestQEvalPipelineRuntimeBackendExecutesStatsPrimitiveBackendPlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want float64
	}{
		{name: "weighted_sum", src: "1 2 3 wsum 10 20 30", want: 140},
		{name: "correlation", src: "1 2 3 cor 1 2 3", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			if ref.Kernel != "QPipelinePlan" || ref.Shape == "" || ref.PipelineShape != "numeric_stats" {
				t.Fatalf("ref = %+v, want numeric stats q pipeline", ref)
			}
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				return stdq.ExecuteEvalPipelineBackendPlan(plan)
			}
			backend.executeSource = func(source string) (any, bool, error) {
				return nil, false, errors.New("source planner fallback should not execute")
			}

			value, handled, err := executeQEvalPipelinePlanValue(backend, ref)
			if err != nil {
				t.Fatalf("executeQEvalPipelinePlanValue: %v", err)
			}
			if !handled || !value.IsFloat() || value.Float() != tc.want {
				t.Fatalf("executeQEvalPipelinePlanValue = %v handled %v, want float %v handled", value, handled, tc.want)
			}
			if backendPlanCalls != 1 {
				t.Fatalf("backend plan calls = %d, want 1", backendPlanCalls)
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
			if ref.Kernel != "QPipelinePlan" ||
				ref.Shape != tc.shape ||
				ref.PipelineShape != "numeric_math" ||
				ref.ShapeFamily != "vector" ||
				ref.ShapeTransform == "" {
				t.Fatalf("ref = %+v, want numeric math q pipeline shape %q", ref, tc.shape)
			}
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			sourceCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				return stdq.ExecuteEvalPipelineBackendPlan(plan)
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
			if backendPlanCalls != 1 || sourceCalls != 0 {
				t.Fatalf("backend plan/source calls = %d/%d, want 1/0", backendPlanCalls, sourceCalls)
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
		{name: "scalar_at_script", src: "x:10 20 30;x@1", shape: "script-pipeline/apply-index/scalar-at/assignments", pipelineShape: "script_pipeline", wantInt: 20},
		{name: "scalar_dot_script", src: "x:10 20 30;x . 2", shape: "script-pipeline/apply-index/scalar-dot/assignments", pipelineShape: "script_pipeline", wantInt: 30},
		{name: "matrix_dot_cell_script", src: "m:2 3#til 6;m . 1 2", shape: "script-pipeline/apply-index/path-dot/assignments", pipelineShape: "script_pipeline", wantInt: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			if ref.Kernel == "" ||
				ref.Shape != tc.shape ||
				ref.PipelineShape != tc.pipelineShape ||
				ref.Backend != qEvalPipelineTypedRuntimeBackend {
				t.Fatalf("ref = %+v, want shape=%q pipeline=%q typed backend", ref, tc.shape, tc.pipelineShape)
			}
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			backendPlanCalls := 0
			sourceCalls := 0
			backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
				backendPlanCalls++
				return stdq.ExecuteEvalPipelineBackendPlan(plan)
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
			if backendPlanCalls != 1 || sourceCalls != 0 {
				t.Fatalf("backend plan/source calls = %d/%d, want 1/0", backendPlanCalls, sourceCalls)
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
			if ref.Shape != tc.shape || ref.PipelineShape != "numeric_math" || ref.Backend != qEvalPipelineTypedRuntimeBackend {
				t.Fatalf("lowered ref = %+v, want typed numeric math backend shape %q", ref, tc.shape)
			}
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
			if ref.Shape != tc.shape || ref.PipelineShape != tc.pipelineShape || ref.Backend != qEvalPipelineTypedRuntimeBackend {
				t.Fatalf("lowered ref = %+v, want typed backend shape=%q pipeline=%q", ref, tc.shape, tc.pipelineShape)
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
		return stdq.ExecuteEvalPipelineBackendPlan(plan)
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
	if backendPlanCalls != 1 {
		t.Fatalf("backend plan calls = %d, want compiled function backend to be reused", backendPlanCalls)
	}
}

func TestCompiledFunctionUsesQEvalPipelineHelperSlot(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
	backendPlanCalls := 0
	backend.executeBackendPlan = func(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
		backendPlanCalls++
		return stdq.ExecuteEvalPipelineBackendPlan(plan)
	}
	backend.executeSource = func(source string) (any, bool, error) {
		return nil, false, errors.New("source planner fallback should not execute")
	}
	cf := &CompiledFunction{
		QEvalPipelinePlans:       []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend:     qRuntimeEvalPipelineBackend{},
		QEvalPipelinePlanHelpers: newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{ref}, backend),
	}

	value, handled, err := cf.ExecuteQEvalPipelinePlanValue(ref.ID)
	if err != nil {
		t.Fatalf("ExecuteQEvalPipelinePlanValue: %v", err)
	}
	if !handled || !value.IsInt() || value.Int() != 16 {
		t.Fatalf("ExecuteQEvalPipelinePlanValue = %v handled %v, want int 16 handled", value, handled)
	}
	if backendPlanCalls != 1 {
		t.Fatalf("backend plan calls = %d, want helper slot execution", backendPlanCalls)
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
	if len(customHelpers) != 1 || customHelpers[0].evalState != nil {
		t.Fatalf("custom executor helper evalState = %+v, want nil", customHelpers)
	}
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
	descriptor, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipeline(source)
	if !ok {
		tb.Fatalf("DescribeQEvalPipeline(%q) did not recognize pipeline", source)
	}
	plan := qEvalHotPlan{
		Kernel:         descriptor.Kernel,
		Shape:          descriptor.Shape,
		PipelineShape:  descriptor.PipelineShape,
		ShapeFamily:    descriptor.ShapeFamily,
		ShapeReducer:   descriptor.ShapeReducer,
		ShapeSelector:  descriptor.ShapeSelector,
		ShapeTransform: descriptor.ShapeTransform,
		Backend:        descriptor.Backend,
		Detail:         descriptor.Detail,
		Kind:           descriptor.Kind,
		Terminal:       descriptor.Terminal,
		AssignmentText: descriptor.AssignmentText,
		ValueExpr:      descriptor.ValueExpr,
		ValueBinding:   descriptor.ValueBinding,
		IndexExpr:      descriptor.IndexExpr,
		IndexBinding:   descriptor.IndexBinding,
		MaskExpr:       descriptor.MaskExpr,
		MaskBinding:    descriptor.MaskBinding,
		RowValueExpr:   descriptor.RowValueExpr,
		RowIndexExpr:   descriptor.RowIndexExpr,
		ColIndexExpr:   descriptor.ColIndexExpr,
		CallableExpr:   descriptor.CallableExpr,
		DyadicOp:       descriptor.DyadicOp,
		ScalarExpr:     descriptor.ScalarExpr,
		ScalarLeft:     descriptor.ScalarLeft,
		LeftExpr:       descriptor.LeftExpr,
		RightExpr:      descriptor.RightExpr,
		CompareOp:      descriptor.CompareOp,
		ComparePrefix:  descriptor.ComparePrefix,
		ModExpr:        descriptor.ModExpr,
		ModulusExpr:    descriptor.ModulusExpr,
		ModTargetExpr:  descriptor.ModTargetExpr,
		ReductionInput: descriptor.ReductionInput,
	}
	fn := &Function{}
	return fn.addQEvalPipelinePlan("not a q pipeline", plan)
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
