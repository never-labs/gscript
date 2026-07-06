//go:build darwin && arm64 && qextension

package methodjit

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestQEvalPipelinePlanOpExitExecutesTypedPlanRef(t *testing.T) {
	ref := QEvalPipelinePlanRef{
		ID:            0,
		Kernel:        "QScriptPipelinePlan",
		Shape:         "script-pipeline/where-index-reduce/sum/assignments",
		PipelineShape: "script_pipeline",
		Source:        "x:til 64;y:x+1;idx:where x>10;+/y[idx]",
		Backend:       qEvalPipelineTypedRuntimeBackend,
	}
	cf := &CompiledFunction{QEvalPipelinePlans: []QEvalPipelinePlanRef{ref}}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		OpExitOp:   int64(OpQEvalPipelinePlan),
		OpExitSlot: 0,
		OpExitAux:  0,
	}

	if err := cf.executeOpExit(ctx, regs); err != nil {
		t.Fatalf("executeOpExit(QEvalPipelinePlan): %v", err)
	}
	if !regs[0].IsInt() || regs[0].Int() != 2014 {
		t.Fatalf("executeOpExit(QEvalPipelinePlan) = %v, want int 2014", regs[0])
	}
}
