package methodjit

import (
	"errors"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
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
