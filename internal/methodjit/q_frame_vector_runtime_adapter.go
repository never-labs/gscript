//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
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

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameProjectColumn(frameVal, specVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "project/column"
	names, resultName, err := frameProjectColumnSpec(specVal)
	if err != nil {
		a.recordFrame("FrameProjectColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	out, err := executeFrameProjectColumnValue(frameVal, names, resultName)
	if err != nil {
		a.recordFrame("FrameProjectColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameProjectColumn", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameFilterProjectColumn(frameVal, maskVal, specVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "filter/project/column"
	names, resultName, err := frameProjectColumnSpec(specVal)
	if err != nil {
		a.recordFrame("FrameFilterProjectColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	out, err := executeFrameFilterProjectColumnValue(frameVal, maskVal, names, resultName)
	if err != nil {
		a.recordFrame("FrameFilterProjectColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameFilterProjectColumn", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameGather(frameVal, indexVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "gather"
	out, err := executeFrameGatherValue(frameVal, indexVal)
	if err != nil {
		a.recordFrame("FrameGather", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameGather", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameSlice(frameVal, endVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "slice"
	out, err := executeFrameSliceValue(frameVal, endVal)
	if err != nil {
		a.recordFrame("FrameSlice", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameSlice", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameOrder(frameVal, specVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "order"
	out, err := executeFrameOrderValue(frameVal, specVal)
	if err != nil {
		a.recordFrame("FrameOrder", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameOrder", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameOrderGather(frameVal, specVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "order/gather"
	out, err := executeFrameOrderGatherValue(frameVal, specVal)
	if err != nil {
		a.recordFrame("FrameOrderGather", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameOrderGather", shape, route, "success", frameVal)
	return out, nil
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
	const shape = "compare/vector-where/vector-reduce"
	out, err := executeQVectorWhereReduceValue(opCode, maskVal, trueVal, falseVal)
	if err != nil {
		a.recordVectorByInstrID(instrID, "QVectorWhereReduce", shape, route, "error")
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "QVectorWhereReduce", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeQVectorGatherReduce(instrID, opCode int, vectorVal, indexVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "gather/vector-reduce"
	out, err := executeQVectorGatherReduceValue(opCode, vectorVal, indexVal)
	if err != nil {
		a.recordVectorByInstrID(instrID, "QVectorGatherReduce", shape, route, "error")
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "QVectorGatherReduce", shape, route, "success")
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

func (a qFrameVectorRuntimeExecutionAdapter) recordVectorByInstrID(instrID int, kernel, fallbackShape string, route qTypedRuntimeExecutionRoute, outcome string) {
	if a.cf == nil {
		return
	}
	a.cf.recordQVectorRuntimeKernelExecution(instrID, kernel, fallbackShape, route, outcome)
}
