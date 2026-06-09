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
