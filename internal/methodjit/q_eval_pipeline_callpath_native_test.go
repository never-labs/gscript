//go:build darwin && arm64 && qextension

package methodjit

import (
	"testing"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
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

	shape := qEvalPipelinePlanRefShape(ref)
	stats := cf.QKernelExecutionStats()
	assertQEvalPipelineExecutionStat(t, stats, shape, "typed_runtime_op_exit", "success", 1)
	assertQEvalPipelineExecutionStat(t, stats, "q-eval/pipeline-plan", "typed_runtime_op_exit", "error", 1)
	assertQEvalPipelineExecutionStat(t, stats, shape, "typed_runtime_native_exit", "success", 1)
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

	if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteNativeExit); err != nil {
		t.Fatalf("executeQEvalPipelinePlanExit: %v", err)
	}
	if !regs[0].IsInt() || regs[0].Int() != 16 {
		t.Fatalf("executeQEvalPipelinePlanExit = %v, want int 16", regs[0])
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), qEvalPipelinePlanRefShape(ref), "typed_runtime_native_exit", "success", 1)
}

func TestQEvalPipelinePlanDescriptorCacheTracksRouteWarmPath(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	cf := &CompiledFunction{
		QEvalPipelinePlans:   []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend: newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
	}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{OpExitSlot: 0, OpExitAux: int64(ref.ID)}

	if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteOpExit); err != nil {
		t.Fatalf("executeQEvalPipelinePlanExit first run: %v", err)
	}
	assertQEvalPipelineDescriptorCacheStat(t, cf.QKernelDescriptorCacheStats(), ref, string(qEvalPipelineExecutionRouteOpExit), 1, 0, 1, 0)
	if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteOpExit); err != nil {
		t.Fatalf("executeQEvalPipelinePlanExit second run: %v", err)
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), qEvalPipelinePlanRefShape(ref), "typed_runtime_op_exit", "success", 2)
	assertQEvalPipelineDescriptorCacheStat(t, cf.QKernelDescriptorCacheStats(), ref, string(qEvalPipelineExecutionRouteOpExit), 1, 1, 1, 0)
}

func TestQEvalPipelinePlanCounterStatsRecordDescriptorCache(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	cf := &CompiledFunction{
		QEvalPipelinePlans:     []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend:   newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
		QEvalPipelinePlanStats: newQEvalPipelinePlanExecutionCounters([]QEvalPipelinePlanRef{ref}),
	}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{OpExitSlot: 0, OpExitAux: int64(ref.ID)}

	if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteNativeExit); err != nil {
		t.Fatalf("executeQEvalPipelinePlanExit first run: %v", err)
	}
	assertQEvalPipelineDescriptorCacheStat(t, cf.QKernelDescriptorCacheStats(), ref, string(qEvalPipelineExecutionRouteNativeExit), 1, 0, 1, 0)
	if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteNativeExit); err != nil {
		t.Fatalf("executeQEvalPipelinePlanExit second run: %v", err)
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), qEvalPipelinePlanRefShape(ref), "typed_runtime_native_exit", "success", 2)
	assertQEvalPipelineDescriptorCacheStat(t, cf.QKernelDescriptorCacheStats(), ref, string(qEvalPipelineExecutionRouteNativeExit), 1, 1, 1, 0)
	if got := qEvalPipelineDescriptorCacheCount(cf.QKernelDescriptorCacheStats(), ref, string(qEvalPipelineExecutionRouteOpExit)); got != 0 {
		t.Fatalf("op-exit descriptor cache rows = %d, want 0", got)
	}
}

func TestQEvalPipelinePlanOpExitRecordsErrorRoute(t *testing.T) {
	cf := &CompiledFunction{}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		ExitCode:   ExitOpExit,
		OpExitSlot: 0,
		OpExitAux:  7,
		OpExitID:   17,
	}

	err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteOpExit)
	const want = "QEvalPipelinePlan exit plan 7 was not handled"
	if err == nil || err.Error() != want {
		t.Fatalf("executeQEvalPipelinePlanExit op-exit error = %v, want %q", err, want)
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_op_exit", "error", 1)
	assertQEvalPipelineExecutionReason(t, cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_op_exit", "error", qEvalPipelineReasonPlanUnhandled, 1)
	if got := qEvalPipelineExecutionCount(cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_native_exit", "error"); got != 0 {
		t.Fatalf("native-exit error count = %d, want 0", got)
	}
}

func TestQEvalPipelinePlanNativeExitRecordsErrorRoute(t *testing.T) {
	cf := &CompiledFunction{}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		ExitCode:   ExitQEvalPipelinePlan,
		OpExitSlot: 0,
		OpExitAux:  7,
		OpExitID:   17,
	}

	err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteNativeExit)
	const want = "QEvalPipelinePlan exit plan 7 was not handled"
	if err == nil || err.Error() != want {
		t.Fatalf("executeQEvalPipelinePlanExit native-exit error = %v, want %q", err, want)
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_native_exit", "error", 1)
	assertQEvalPipelineExecutionReason(t, cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_native_exit", "error", qEvalPipelineReasonPlanUnhandled, 1)
	if got := qEvalPipelineExecutionCount(cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_op_exit", "error"); got != 0 {
		t.Fatalf("op-exit error count = %d, want 0", got)
	}
}

func TestTier2DirectHelperBridgeQEvalPipelinePlanRecordsNativeExitStats(t *testing.T) {
	ref := qEvalPipelineDescriptorBackendTestRef(t, "count where (til 64 mod 4)=1")
	proto := &vm.FuncProto{Name: "q_eval_pipeline_plan_direct_helper", MaxStack: 1}
	cf := &CompiledFunction{
		Proto:                  proto,
		QEvalPipelinePlans:     []QEvalPipelinePlanRef{ref},
		QEvalPipelineBackend:   newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef{ref}),
		QEvalPipelinePlanStats: newQEvalPipelinePlanExecutionCounters([]QEvalPipelinePlanRef{ref}),
	}
	regs := []runtime.Value{runtime.NilValue()}
	tm := NewTieringManager()
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		HelperTM:   tm,
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[0])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQEvalPipelinePlan),
		OpExitSlot: 0,
		OpExitAux:  int64(ref.ID),
		OpExitID:   17,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QEvalPipelinePlan error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QEvalPipelinePlan direct helper calls = %d, want 1", got)
	}
	if !regs[0].IsInt() || regs[0].Int() != 16 {
		t.Fatalf("QEvalPipelinePlan direct helper result = %v, want int 16", regs[0])
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), qEvalPipelinePlanRefShape(ref), "typed_runtime_native_exit", "success", 1)
	snap := tm.ExitStats()
	if got := snap.ByExitCode["ExitQEvalPipelinePlan"]; got != 1 {
		t.Fatalf("ExitQEvalPipelinePlan stats = %d, want 1; snap=%+v", got, snap)
	}
}

func TestTier2DirectHelperBridgeQEvalPipelinePlanRecordsNativeExitErrorStats(t *testing.T) {
	cf := &CompiledFunction{
		Proto:                  &vm.FuncProto{Name: "q_eval_pipeline_plan_direct_helper_error", MaxStack: 1},
		QEvalPipelinePlanStats: newQEvalPipelinePlanExecutionCounters(nil),
	}
	regs := []runtime.Value{runtime.NilValue()}
	tm := NewTieringManager()
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		HelperTM:   tm,
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[0])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQEvalPipelinePlan),
		OpExitSlot: 0,
		OpExitAux:  7,
		OpExitID:   17,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QEvalPipelinePlan direct helper calls = %d, want 1", got)
	}
	const want = "QEvalPipelinePlan exit plan 7 was not handled"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QEvalPipelinePlan direct helper error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_native_exit", "error", 1)
	if got := qEvalPipelineExecutionCount(cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_op_exit", "error"); got != 0 {
		t.Fatalf("op-exit error count = %d, want 0", got)
	}
	if got := qEvalPipelineExecutionCount(cf.QKernelExecutionStats(), "q-eval/pipeline-plan", "typed_runtime_direct_entry", "error"); got != 0 {
		t.Fatalf("direct-entry error count = %d, want 0", got)
	}
	if got := tm.ExitStats().ByExitCode["ExitQEvalPipelinePlan"]; got != 1 {
		t.Fatalf("ExitQEvalPipelinePlan stats = %d, want 1", got)
	}
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
		assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), qEvalPipelinePlanRefShape(ref), "typed_runtime_native_exit", "success", 1)
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
	assertQEvalPipelineExecutionStat(t, cf.QKernelExecutionStats(), qEvalPipelinePlanRefShape(cf.QEvalPipelinePlans[0]), "typed_runtime_direct_entry", "success", 1)
}

func TestQEvalPipelineDirectReturnRejectsLegacyMirrorOnlyRef(t *testing.T) {
	fn := &Function{}
	block := &Block{ID: 0}
	fn.Entry = block
	fn.Blocks = []*Block{block}
	fn.QEvalPipelinePlans = []QEvalPipelinePlanRef{{
		ID:            0,
		Kernel:        "QPipelinePlan",
		Shape:         "vector-reduce/sum",
		PipelineShape: "expression",
		Backend:       qEvalPipelineTypedRuntimeBackend,
		Kind:          "expression",
	}}
	plan := &Instr{ID: fn.newValueID(), Op: OpQEvalPipelinePlan, Type: TypeAny, Aux: 0, Block: block}
	retValue := &Value{ID: plan.ID, Def: plan}
	ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Args: []*Value{retValue}, Block: block}
	block.Instrs = []*Instr{plan, ret}

	if got := qEvalPipelineDirectReturnPlanID(fn); got != -1 {
		t.Fatalf("qEvalPipelineDirectReturnPlanID = %d, want -1 for legacy mirror-only ref", got)
	}
}

func TestQEvalPipelineTieringEntryUsesDirectReturn(t *testing.T) {
	t.Setenv(exitResumeCheckEnv, "")
	cf := compileQEvalPipelineNativeExitBenchmark(t, "count where (til 64 mod 4)=1")
	defer cf.Code.Free()
	if !cf.QEvalPipelineDirectReturn || cf.QEvalPipelineDirectReturnID != 0 {
		t.Fatalf("compiled direct q eval return = %v/%d, want true/0", cf.QEvalPipelineDirectReturn, cf.QEvalPipelineDirectReturnID)
	}

	tm := NewTieringManager()
	regs := runtime.MakeNilSlice(cf.numRegs + 1)
	var storage [1]runtime.Value
	result, err := tm.executeTier2WithResultBuffer(cf, regs, 0, cf.Proto, storage[:0])
	if err != nil {
		t.Fatalf("executeTier2WithResultBuffer: %v", err)
	}
	if len(result) != 1 || &result[0] != &storage[0] {
		t.Fatalf("executeTier2WithResultBuffer result buffer = %p len %d, want storage %p len 1", result, len(result), &storage[0])
	}
	if !result[0].IsInt() || result[0].Int() != 16 {
		t.Fatalf("executeTier2WithResultBuffer result = %v, want int 16", result)
	}
	stats := cf.QKernelExecutionStats()
	shape := qEvalPipelinePlanRefShape(cf.QEvalPipelinePlans[0])
	assertQEvalPipelineExecutionStat(t, stats, shape, "typed_runtime_direct_entry", "success", 1)
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == "QEvalPipelinePlan" &&
			stat.Shape == shape &&
			stat.Route == "typed_runtime_native_exit" &&
			stat.Outcome == "success" &&
			stat.Count != 0 {
			t.Fatalf("native q eval pipeline exits = %d, want 0; stats=%+v", stat.Count, stats)
		}
	}
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
		{name: "ScalarWhereReduce", src: "+/til 8192 where til 8192>4096"},
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
			b.StopTimer()
			reportQEvalPipelineJITRouteStats(b, cf)
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
				if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteNativeExit); err != nil {
					b.Fatalf("executeQEvalPipelinePlanExit: %v", err)
				}
				qEvalPipelineDescriptorBenchmarkSink = regs[0]
			}
			b.StopTimer()
			reportQEvalPipelineJITRouteStats(b, cf)
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
				if err := cf.executeQEvalPipelinePlanExit(ctx, regs, 0, qEvalPipelineExecutionRouteNativeExit); err != nil {
					b.Fatalf("executeQEvalPipelinePlanExit: %v", err)
				}
				qEvalPipelineDescriptorBenchmarkSink = regs[0]
			}
			b.StopTimer()
			reportQEvalPipelineJITRouteStats(b, cf)
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
			b.StopTimer()
			reportQEvalPipelineJITRouteStats(b, cf)
		})
	}
}

func reportQEvalPipelineJITRouteStats(b *testing.B, cf *CompiledFunction) {
	b.Helper()
	if cf == nil {
		return
	}
	reportQEvalPipelineJITRouteBenchmarkStats(b, b.N, cf.QKernelExecutionStats())
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

func qEvalPipelineExecutionCount(stats []QKernelExecutionStat, shape, route, outcome string) uint64 {
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == "QEvalPipelinePlan" &&
			stat.Shape == shape &&
			stat.Route == route &&
			stat.Outcome == outcome {
			return stat.Count
		}
	}
	return 0
}

func assertQEvalPipelineExecutionReason(t *testing.T, stats []QKernelExecutionStat, shape, route, outcome, reasonCode string, count uint64) {
	t.Helper()
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == "QEvalPipelinePlan" &&
			stat.Shape == shape &&
			stat.Route == route &&
			stat.Outcome == outcome &&
			stat.ReasonCode == reasonCode {
			if stat.Count != count {
				t.Fatalf("QEvalPipelinePlan execution reason %s/%s/%s/%s count = %d, want %d; stats=%+v",
					shape, route, outcome, reasonCode, stat.Count, count, stats)
			}
			return
		}
	}
	t.Fatalf("missing QEvalPipelinePlan execution reason shape=%s route=%s outcome=%s reason=%s; stats=%+v", shape, route, outcome, reasonCode, stats)
}

func assertQEvalPipelineDescriptorCacheStat(t *testing.T, stats []QKernelDescriptorCacheStat, ref QEvalPipelinePlanRef, route string, entries, hits, misses, evictions uint64) {
	t.Helper()
	shape := qEvalPipelinePlanRefShape(ref)
	pipelineShape := qEvalPipelinePlanRefPipelineShape(ref)
	if pipelineShape == "" || pipelineShape == "unknown" {
		pipelineShape = qKernelExecutionPipelineShape("QPipelinePlan", shape)
	}
	if pipelineShape == "" {
		pipelineShape = "unknown"
	}
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == "QEvalPipelinePlan" &&
			stat.Shape == shape &&
			stat.PipelineShape == pipelineShape &&
			stat.Route == route &&
			stat.SchemaHash == "unknown" &&
			stat.Entries == entries &&
			stat.Hits == hits &&
			stat.Misses == misses &&
			stat.Evictions == evictions {
			return
		}
	}
	t.Fatalf("missing QEvalPipelinePlan descriptor cache stat shape=%s pipeline=%s route=%s; stats=%+v", shape, pipelineShape, route, stats)
}

func qEvalPipelineDescriptorCacheCount(stats []QKernelDescriptorCacheStat, ref QEvalPipelinePlanRef, route string) uint64 {
	shape := qEvalPipelinePlanRefShape(ref)
	pipelineShape := qEvalPipelinePlanRefPipelineShape(ref)
	if pipelineShape == "" || pipelineShape == "unknown" {
		pipelineShape = qKernelExecutionPipelineShape("QPipelinePlan", shape)
	}
	if pipelineShape == "" {
		pipelineShape = "unknown"
	}
	var count uint64
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == "QEvalPipelinePlan" &&
			stat.Shape == shape &&
			stat.PipelineShape == pipelineShape &&
			stat.Route == route &&
			stat.SchemaHash == "unknown" {
			count++
		}
	}
	return count
}
