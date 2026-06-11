//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
)

type qTypedRuntimeExecutionRoute string

const (
	qTypedRuntimeExecutionRouteOpExit qTypedRuntimeExecutionRoute = "typed_runtime_op_exit"
)

const (
	qFrameRuntimeExecutionSource  = "methodjit_q_frame_runtime"
	qVectorRuntimeExecutionSource = "methodjit_q_vector_runtime"
)

type qFrameVectorRuntimeExecutionAdapter struct {
	cf *CompiledFunction
}

func (cf *CompiledFunction) qFrameVectorRuntimeExecutionAdapter() qFrameVectorRuntimeExecutionAdapter {
	return qFrameVectorRuntimeExecutionAdapter{cf: cf}
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameGroupAggregate(frameVal, maskVal, specVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	shape := qFrameGroupAggregateRuntimeShapeFromMaskValue(maskVal)
	out, err := executeFrameGroupAggregateValue(frameVal, maskVal, specVal)
	if err != nil {
		a.recordFrame("FrameGroupAggregate", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameGroupAggregate", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeQFrameSelectColumn(constants []runtime.Value, specIdx int, frameVal, argVal runtime.Value, hasArg bool, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	if a.cf == nil {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn op-exit missing compiled function")
	}
	shape := qFrameSelectColumnExecutionShape(a.cf.QFrameSelectColumnSpecs, specIdx)
	plan, err := a.cf.qFrameSelectColumnRuntimePlan(constants, specIdx, frameVal)
	if err != nil {
		a.recordFrame("QFrameSelectColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	out, err := executeQFrameSelectColumnPlannedValue(constants, a.cf.QFrameSelectColumnSpecs, specIdx, plan, frameVal, argVal, hasArg)
	if err != nil {
		a.recordFrame("QFrameSelectColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("QFrameSelectColumn", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeQVectorWhereReduce(instrID, opCode int, maskVal, trueVal, falseVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	shape := "compare/vector-where/vector-reduce"
	if a.cf != nil {
		shape = a.cf.qVectorRuntimeKernelShape(instrID, shape)
	}
	out, err := executeQVectorWhereReduceValue(opCode, maskVal, trueVal, falseVal)
	if err != nil {
		a.recordVector("QVectorWhereReduce", shape, route, "error")
		return runtime.NilValue(), err
	}
	a.recordVector("QVectorWhereReduce", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeQVectorGatherReduce(opCode int, vectorVal, indexVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "gather/vector-reduce"
	out, err := executeQVectorGatherReduceValue(opCode, vectorVal, indexVal)
	if err != nil {
		a.recordVector("QVectorGatherReduce", shape, route, "error")
		return runtime.NilValue(), err
	}
	a.recordVector("QVectorGatherReduce", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) recordFrame(kernel, shape string, route qTypedRuntimeExecutionRoute, outcome string, frameVal runtime.Value) {
	if a.cf == nil {
		return
	}
	a.cf.recordQKernelExecutionForFrame(qFrameRuntimeExecutionSource, kernel, shape, string(route), outcome, frameVal)
}

func (a qFrameVectorRuntimeExecutionAdapter) recordVector(kernel, shape string, route qTypedRuntimeExecutionRoute, outcome string) {
	if a.cf == nil {
		return
	}
	a.cf.recordQKernelExecution(qVectorRuntimeExecutionSource, kernel, shape, string(route), outcome)
}
