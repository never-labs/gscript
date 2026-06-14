package methodjit

import "testing"

func TestQEvalPipelinePlanCallpathContracts(t *testing.T) {
	spec, ok := OpQEvalPipelinePlan.Spec()
	if !ok {
		t.Fatalf("OpQEvalPipelinePlan has no OpSpec")
	}
	if spec.Name != "QEvalPipelinePlan" ||
		spec.EmitterFamily != OpEmitterTable ||
		spec.ArgPolicy != OpArgFixedAux ||
		!spec.ArgCount.Set ||
		spec.ArgCount.Min != 0 ||
		spec.ArgCount.Max != 0 ||
		spec.SideEffect != OpSideEffectRead {
		t.Fatalf("OpQEvalPipelinePlan spec = %+v, want zero-arg read-side typed runtime op", spec)
	}
	if !spec.NativeReplayMayExit || !tier2OpMayExitForNativeReplay(&Instr{Op: OpQEvalPipelinePlan}) {
		t.Fatalf("OpQEvalPipelinePlan should be native-replay may-exit through OpSpec")
	}

	source, kernel, shape, route, ok := qRuntimePrimitiveExecutionMetadata(OpQEvalPipelinePlan)
	if !ok ||
		source != "methodjit_q_eval_runtime" ||
		kernel != "QEvalPipelinePlan" ||
		shape != "q-eval/pipeline-plan" ||
		route != "typed_runtime_op_exit" {
		t.Fatalf("qRuntimePrimitiveExecutionMetadata(OpQEvalPipelinePlan) = %q/%q/%q/%q/%v, want q eval typed runtime op-exit metadata",
			source, kernel, shape, route, ok)
	}

	fn := &Function{Blocks: []*Block{{Instrs: []*Instr{{ID: 1, Op: OpQEvalPipelinePlan, Aux: 0}}}}}
	if !fnHasResultProducingOpExit(fn) {
		t.Fatalf("OpQEvalPipelinePlan should be treated as a result-producing op-exit by native codegen")
	}
	if got := qEvalPipelinePlanExecutionShape(nil, 0); got != "q-eval/pipeline-plan" {
		t.Fatalf("qEvalPipelinePlanExecutionShape(nil, 0) = %q, want fallback shape", got)
	}
}

func TestQSQLKernelPlanCallpathContracts(t *testing.T) {
	spec, ok := OpQSQLKernelPlan.Spec()
	if !ok {
		t.Fatalf("OpQSQLKernelPlan has no OpSpec")
	}
	if spec.Name != "QSQLKernelPlan" ||
		spec.EmitterFamily != OpEmitterTable ||
		spec.ArgPolicy != OpArgFixedAux ||
		!spec.ArgCount.Set ||
		spec.ArgCount.Min != 0 ||
		spec.ArgCount.Max != 0 ||
		spec.SideEffect != OpSideEffectRead {
		t.Fatalf("OpQSQLKernelPlan spec = %+v, want zero-arg read-side typed runtime op", spec)
	}
	if !spec.NativeReplayMayExit || !tier2OpMayExitForNativeReplay(&Instr{Op: OpQSQLKernelPlan}) {
		t.Fatalf("OpQSQLKernelPlan should be native-replay may-exit through OpSpec")
	}

	source, kernel, shape, route, ok := qRuntimePrimitiveExecutionMetadata(OpQSQLKernelPlan)
	if !ok ||
		source != QSQLKernelRuntimeSource ||
		kernel != "QSQLKernelPlan" ||
		shape != "qsql/kernel-plan" ||
		route != "typed_runtime_op_exit" {
		t.Fatalf("qRuntimePrimitiveExecutionMetadata(OpQSQLKernelPlan) = %q/%q/%q/%q/%v, want qSQL typed runtime op-exit metadata",
			source, kernel, shape, route, ok)
	}

	fn := &Function{Blocks: []*Block{{Instrs: []*Instr{{ID: 1, Op: OpQSQLKernelPlan, Aux: 0}}}}}
	if !fnHasResultProducingOpExit(fn) {
		t.Fatalf("OpQSQLKernelPlan should be treated as a result-producing op-exit by native codegen")
	}
}

func TestQEvalPipelinePlanExecutionShapeUsesPlanRefShape(t *testing.T) {
	refs := []QEvalPipelinePlanRef{{
		ID:      0,
		Kernel:  "QScriptPipelinePlan",
		Shape:   "script-pipeline/where-index-reduce/sum/assignments",
		Backend: qEvalPipelineTypedRuntimeBackend,
	}}
	if got := qEvalPipelinePlanExecutionShape(refs, 0); got != refs[0].Shape {
		t.Fatalf("qEvalPipelinePlanExecutionShape = %q, want %q", got, refs[0].Shape)
	}
	if got := qEvalPipelinePlanExecutionShape(refs, 7); got != "q-eval/pipeline-plan" {
		t.Fatalf("qEvalPipelinePlanExecutionShape for missing ref = %q, want fallback", got)
	}
}

func TestQVectorWhereReduceCallpathContracts(t *testing.T) {
	spec, ok := OpQVectorWhereReduce.Spec()
	if !ok {
		t.Fatalf("OpQVectorWhereReduce has no OpSpec")
	}
	if spec.Name != "QVectorWhereReduce" ||
		spec.EmitterFamily != OpEmitterTable ||
		spec.ArgPolicy != OpArgFixedAux ||
		!spec.ArgCount.Set ||
		spec.ArgCount.Min != 3 ||
		spec.ArgCount.Max != 3 ||
		spec.SideEffect != OpSideEffectRead {
		t.Fatalf("OpQVectorWhereReduce spec = %+v, want three-arg read-side typed runtime op", spec)
	}

	source, kernel, shape, route, ok := qRuntimePrimitiveExecutionMetadata(OpQVectorWhereReduce)
	if !ok ||
		source != "methodjit_q_vector_runtime" ||
		kernel != "QVectorWhereReduce" ||
		shape != "compare/vector-where/vector-reduce" ||
		route != string(qTypedRuntimeExecutionRouteOpExit) {
		t.Fatalf("qRuntimePrimitiveExecutionMetadata(OpQVectorWhereReduce) = %q/%q/%q/%q/%v, want q vector typed runtime op-exit metadata",
			source, kernel, shape, route, ok)
	}
}

func TestQTypedRuntimeOpsDeclareExecutionRouteContracts(t *testing.T) {
	ops := []Op{
		OpFrameLen,
		OpFrameColumn,
		OpFrameMask,
		OpFrameProject,
		OpFrameFilter,
		OpFrameFilterProject,
		OpFrameGather,
		OpFrameSlice,
		OpFrameOrder,
		OpFrameOrderGather,
		OpFrameProjectColumn,
		OpFrameFilterProjectColumn,
		OpFrameGroupAggregate,
		OpVectorGather,
		OpVectorCompare,
		OpVectorMask,
		OpVectorWhere,
		OpVectorReduce,
		OpQVectorGatherReduce,
		OpQVectorWhereReduce,
		OpQEvalPipelinePlan,
		OpQSQLKernelPlan,
		OpQEvalSessionEval,
		OpVectorScan,
	}
	seen := make(map[Op]bool, len(ops))
	for _, op := range ops {
		if seen[op] {
			t.Fatalf("duplicate q typed runtime op in contract list: %s", op)
		}
		seen[op] = true
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if spec.EmitterFamily != OpEmitterTable {
			t.Fatalf("%s emitter family = %s, want table/runtime family", op, spec.EmitterFamily)
		}
		if _, _, _, route, ok := qRuntimePrimitiveExecutionMetadata(op); !ok || route == "" {
			t.Fatalf("%s missing q runtime execution metadata: route=%q ok=%v", op, route, ok)
		}
		if !qTypedRuntimeOpHasTier2DirectHelper(op) && !tier2OpMayExitForNativeReplay(&Instr{Op: op}) {
			t.Fatalf("%s has q runtime metadata but no Tier2 direct helper or native-replay exit policy", op)
		}
	}
	for op := Op(0); op < OpMax; op++ {
		if _, _, _, _, ok := qRuntimePrimitiveExecutionMetadata(op); ok && !seen[op] {
			t.Fatalf("%s has q runtime metadata but is missing from q typed runtime route contract list", op)
		}
	}
}
