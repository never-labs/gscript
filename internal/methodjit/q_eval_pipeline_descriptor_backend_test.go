package methodjit

import (
	"errors"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

var qEvalPipelineDescriptorBenchmarkSink runtime.Value

func TestQEvalPipelineRuntimeBackendPrefersDescriptorOverSourcePlanner(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want int64
	}{
		{name: "bin_reduce_sum", src: "+/til 8192 bin til 8192", want: 33550336},
		{name: "modulo_where_count", src: "count where (til 8192 mod 4)=1", want: 2048},
		{name: "script_modulo_gather_reduce", src: "x:til 8192;y:x+1;idx:where (x mod 4)=1;+/y[idx]", want: 8388608},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := qEvalPipelineDescriptorBackendTestRef(t, tc.src)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			descriptorCalls := 0
			sourceCalls := 0
			backend.executeDescriptor = func(descriptor stdq.EvalPipelineDescriptor) (any, bool, error) {
				descriptorCalls++
				if descriptor.Source != "not a q pipeline" {
					t.Fatalf("descriptor Source = %q, want preserved ref source sentinel", descriptor.Source)
				}
				return stdq.ExecuteEvalPipelineDescriptor(descriptor)
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
			if descriptorCalls != 1 || sourceCalls != 0 {
				t.Fatalf("descriptor/source calls = %d/%d, want 1/0", descriptorCalls, sourceCalls)
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
	descriptorCalls := 0
	cf.QEvalPipelineBackend.executeDescriptor = func(descriptor stdq.EvalPipelineDescriptor) (any, bool, error) {
		descriptorCalls++
		return stdq.ExecuteEvalPipelineDescriptor(descriptor)
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
	if descriptorCalls != 1 {
		t.Fatalf("descriptor calls = %d, want compiled function backend to be reused", descriptorCalls)
	}
}

func TestCompiledFunctionUsesQEvalPipelineHelperSlot(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
	descriptorCalls := 0
	backend.executeDescriptor = func(descriptor stdq.EvalPipelineDescriptor) (any, bool, error) {
		descriptorCalls++
		return stdq.ExecuteEvalPipelineDescriptor(descriptor)
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
	if descriptorCalls != 1 {
		t.Fatalf("descriptor calls = %d, want helper slot execution", descriptorCalls)
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
		b.Run("DescriptorBackend/"+tc.name, func(b *testing.B) {
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
