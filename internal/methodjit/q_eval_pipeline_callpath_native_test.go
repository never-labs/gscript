//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func TestQEvalPipelinePlanExecutionStatsUsePlanShape(t *testing.T) {
	ref := QEvalPipelinePlanRef{
		ID:      0,
		Kernel:  "QScriptPipelinePlan",
		Shape:   "script-pipeline/where-index-reduce/sum/assignments",
		Backend: qEvalPipelineTypedRuntimeBackend,
	}
	cf := &CompiledFunction{QEvalPipelinePlans: []QEvalPipelinePlanRef{ref}}

	cf.recordQEvalPipelinePlanExecution(0, "success")
	cf.recordQEvalPipelinePlanExecution(7, "error")
	cf.recordQEvalPipelinePlanExecutionWithRoute(0, "typed_runtime_native_exit", "success")

	stats := cf.QKernelExecutionStats()
	assertQEvalPipelineExecutionStat(t, stats, ref.Shape, "typed_runtime_op_exit", "success", 1)
	assertQEvalPipelineExecutionStat(t, stats, "q-eval/pipeline-plan", "typed_runtime_op_exit", "error", 1)
	assertQEvalPipelineExecutionStat(t, stats, ref.Shape, "typed_runtime_native_exit", "success", 1)
}

func TestQEvalPipelinePlanNativeExitExecutesTypedPlanRef(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	cf := &CompiledFunction{
		QEvalPipelinePlans:   []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend: newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
	}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		ExitCode:   ExitQEvalPipelinePlan,
		OpExitSlot: 0,
		OpExitAux:  int64(ref.ID),
		OpExitID:   17,
	}

	if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, "typed_runtime_native_exit"); err != nil {
		t.Fatalf("executeQEvalPipelinePlanExit: %v", err)
	}
	if !regs[0].IsInt() || regs[0].Int() != 16 {
		t.Fatalf("executeQEvalPipelinePlanExit = %v, want int 16", regs[0])
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), ref.Shape, "typed_runtime_native_exit", "success", 1)
}

func TestQEvalPipelinePlanCodegenUsesDedicatedExitKind(t *testing.T) {
	withExitResumeCheck(t, func() {
		fn := &Function{
			Proto: &vm.FuncProto{
				Name:     "q_eval_pipeline_plan",
				MaxStack: 1,
			},
			NumRegs: 1,
		}
		ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
		ref.ID = 0
		fn.QEvalPipelinePlans = []QEvalPipelinePlanRef{ref}
		b := &Block{ID: 0, defs: make(map[int]*Value)}
		plan := &Instr{ID: fn.newValueID(), Op: OpQEvalPipelinePlan, Type: TypeAny, Aux: int64(ref.ID), Block: b}
		ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Type: TypeUnknown, Args: []*Value{plan.Value()}, Block: b}
		b.Instrs = []*Instr{plan, ret}
		fn.Entry = b
		fn.Blocks = []*Block{b}

		cf, err := Compile(fn, AllocateRegisters(fn))
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		defer cf.Code.Free()
		if len(cf.QEvalPipelinePlanHelpers) != 1 || !cf.QEvalPipelinePlanHelpers[0].validForID(ref.ID) {
			t.Fatalf("compiled q eval pipeline helpers = %+v, want valid helper for plan %d", cf.QEvalPipelinePlanHelpers, ref.ID)
		}

		site, ok := cf.ExitResumeCheck.Sites[exitResumeCheckKey{InstrID: plan.ID, ExitCode: ExitQEvalPipelinePlan}]
		if !ok {
			t.Fatalf("missing ExitQEvalPipelinePlan resume-check site; sites=%+v", cf.ExitResumeCheck.Sites)
		}
		if len(site.ModifiedSlots) != 1 {
			t.Fatalf("ExitQEvalPipelinePlan modified slots = %+v, want one slot", site.ModifiedSlots)
		}
		result, err := cf.Execute(nil)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if len(result) != 1 || !result[0].IsInt() || result[0].Int() != 16 {
			t.Fatalf("Execute result = %v, want int 16", result)
		}
		assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), ref.Shape, "typed_runtime_native_exit", "success", 1)
	})
}

func TestQEvalPipelinePlanDirectReturnExecutesTypedPlanRef(t *testing.T) {
	t.Setenv(exitResumeCheckEnv, "")
	cf := compileQEvalPipelineNativeExitBenchmark(t, "count where (til 64 mod 4)=1")
	defer cf.Code.Free()
	if !cf.QEvalPipelineDirectReturn || cf.QEvalPipelineDirectReturnID != 0 {
		t.Fatalf("compiled direct q eval return = %v/%d, want true/0", cf.QEvalPipelineDirectReturn, cf.QEvalPipelineDirectReturnID)
	}

	result, err := cf.Execute(nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result) != 1 || !result[0].IsInt() || result[0].Int() != 16 {
		t.Fatalf("Execute result = %v, want int 16", result)
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), cf.QEvalPipelinePlans[0].Shape, "typed_runtime_direct_entry", "success", 1)
}

func TestQEvalPipelineTerminalReturnTable(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "+/til 16")
	fn := &Function{QEvalPipelinePlans: []QEvalPipelinePlanRef{ref}}
	b := &Block{ID: 0, defs: make(map[int]*Value)}
	plan := &Instr{ID: fn.newValueID(), Op: OpQEvalPipelinePlan, Type: TypeAny, Aux: int64(ref.ID), Block: b}
	ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Type: TypeUnknown, Args: []*Value{plan.Value()}, Block: b}
	b.Instrs = []*Instr{plan, ret}
	fn.Entry = b
	fn.Blocks = []*Block{b}

	table := qEvalPipelineTerminalReturnTable(fn)
	if plan.ID >= len(table) || !table[plan.ID] {
		t.Fatalf("qEvalPipelineTerminalReturnTable[%d] = false, want true", plan.ID)
	}

	b.Instrs = []*Instr{
		plan,
		{ID: fn.newValueID(), Op: OpAdd, Type: TypeInt, Args: []*Value{plan.Value(), plan.Value()}, Block: b},
		ret,
	}
	table = qEvalPipelineTerminalReturnTable(fn)
	if plan.ID < len(table) && table[plan.ID] {
		t.Fatalf("qEvalPipelineTerminalReturnTable marked non-terminal plan %d", plan.ID)
	}
}

func BenchmarkQEvalPipelineNativeExitCallpath(b *testing.B) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{name: "BinReduceSum", src: "+/til 8192 bin til 8192"},
		{name: "ModuloWhereCount", src: "count where (til 8192 mod 4)=1"},
		{name: "ScriptModuloGatherReduce", src: "x:til 8192;y:x+1;idx:where (x mod 4)=1;+/y[idx]"},
	} {
		b.Run("CodegenNativeExit/"+tc.name, func(b *testing.B) {
			cf := compileQEvalPipelineNativeExitBenchmark(b, tc.src)
			defer cf.Code.Free()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := cf.Execute(nil)
				if err != nil {
					b.Fatalf("Execute: %v", err)
				}
				qEvalPipelineDescriptorBenchmarkSink = result[0]
			}
		})
		b.Run("GoHandlerNativeExit/"+tc.name, func(b *testing.B) {
			ref := qEvalPipelineSourceBackedTestRef(b, tc.src)
			cf := &CompiledFunction{
				QEvalPipelinePlans:   []QEvalPipelinePlanRef{ref},
				QEvalPipelineBackend: newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
			}
			regs := []runtime.Value{runtime.NilValue()}
			ctx := &ExecContext{OpExitSlot: 0, OpExitAux: int64(ref.ID)}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, "typed_runtime_native_exit"); err != nil {
					b.Fatalf("executeQEvalPipelinePlanExit: %v", err)
				}
				qEvalPipelineDescriptorBenchmarkSink = regs[0]
			}
		})
		b.Run("GoHandlerHelperSlot/"+tc.name, func(b *testing.B) {
			ref := qEvalPipelineSourceBackedTestRef(b, tc.src)
			backend := newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref})
			cf := &CompiledFunction{
				QEvalPipelinePlans:       []QEvalPipelinePlanRef{ref},
				QEvalPipelineBackend:     backend,
				QEvalPipelinePlanHelpers: newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef{ref}, backend),
			}
			regs := []runtime.Value{runtime.NilValue()}
			ctx := &ExecContext{OpExitSlot: 0, OpExitAux: int64(ref.ID)}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, "typed_runtime_native_exit"); err != nil {
					b.Fatalf("executeQEvalPipelinePlanExit: %v", err)
				}
				qEvalPipelineDescriptorBenchmarkSink = regs[0]
			}
		})
		b.Run("GoHandlerOpExit/"+tc.name, func(b *testing.B) {
			ref := qEvalPipelineSourceBackedTestRef(b, tc.src)
			cf := &CompiledFunction{
				QEvalPipelinePlans:   []QEvalPipelinePlanRef{ref},
				QEvalPipelineBackend: newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
			}
			regs := []runtime.Value{runtime.NilValue()}
			ctx := &ExecContext{OpExitOp: int64(OpQEvalPipelinePlan), OpExitSlot: 0, OpExitAux: int64(ref.ID)}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := cf.executeOpExit(ctx, regs); err != nil {
					b.Fatalf("executeOpExit: %v", err)
				}
				qEvalPipelineDescriptorBenchmarkSink = regs[0]
			}
		})
	}
}

func compileQEvalPipelineNativeExitBenchmark(tb testing.TB, source string) *CompiledFunction {
	tb.Helper()
	fn := &Function{
		Proto: &vm.FuncProto{
			Name:     "q_eval_pipeline_plan_bench",
			MaxStack: 1,
		},
		NumRegs: 1,
	}
	ref := qEvalPipelineDescriptorBackendTestRef(tb, source)
	ref.ID = 0
	ref.Source = source
	fn.QEvalPipelinePlans = []QEvalPipelinePlanRef{ref}
	b := &Block{ID: 0, defs: make(map[int]*Value)}
	plan := &Instr{ID: fn.newValueID(), Op: OpQEvalPipelinePlan, Type: TypeAny, Aux: int64(ref.ID), Block: b}
	ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Type: TypeUnknown, Args: []*Value{plan.Value()}, Block: b}
	b.Instrs = []*Instr{plan, ret}
	fn.Entry = b
	fn.Blocks = []*Block{b}

	cf, err := Compile(fn, AllocateRegisters(fn))
	if err != nil {
		tb.Fatalf("Compile: %v", err)
	}
	return cf
}

func assertQEvalPipelineExecutionStat(t *testing.T, stats []QKernelExecutionStat, shape, route, outcome string, count uint64) {
	t.Helper()
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == "QEvalPipelinePlan" &&
			stat.Shape == shape &&
			stat.Route == route &&
			stat.Outcome == outcome {
			if stat.Count != count {
				t.Fatalf("QEvalPipelinePlan execution stat %s/%s/%s count = %d, want %d; stats=%+v",
					shape, route, outcome, stat.Count, count, stats)
			}
			return
		}
	}
	t.Fatalf("missing QEvalPipelinePlan execution stat shape=%s route=%s outcome=%s; stats=%+v", shape, route, outcome, stats)
}
