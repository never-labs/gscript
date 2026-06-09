//go:build darwin && arm64

package methodjit

import "testing"

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

	stats := cf.QKernelExecutionStats()
	assertQEvalPipelineExecutionStat(t, stats, ref.Shape, "success", 1)
	assertQEvalPipelineExecutionStat(t, stats, "q-eval/pipeline-plan", "error", 1)
}

func assertQEvalPipelineExecutionStat(t *testing.T, stats []QKernelExecutionStat, shape, outcome string, count uint64) {
	t.Helper()
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == "QEvalPipelinePlan" &&
			stat.Shape == shape &&
			stat.Route == "typed_runtime_op_exit" &&
			stat.Outcome == outcome {
			if stat.Count != count {
				t.Fatalf("QEvalPipelinePlan execution stat %s/%s count = %d, want %d; stats=%+v",
					shape, outcome, stat.Count, count, stats)
			}
			return
		}
	}
	t.Fatalf("missing QEvalPipelinePlan execution stat shape=%s outcome=%s; stats=%+v", shape, outcome, stats)
}
