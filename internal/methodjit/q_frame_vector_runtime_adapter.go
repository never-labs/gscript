//go:build darwin && arm64

package methodjit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/runtime"
)

const (
	qFrameRuntimeExecutionSource  = "methodjit_q_frame_runtime"
	qVectorRuntimeExecutionSource = "methodjit_q_vector_runtime"

	qFrameSelectColumnReasonPlanBuildError = "plan_build_error"
	qFrameSelectColumnReasonExecutionError = "plan_execution_error"

	qVectorRuntimeReasonOperandError = "operand_error"
	qVectorRuntimeReasonDTypeError   = "dtype_error"
	qVectorRuntimeReasonLengthError  = "length_error"
	qVectorRuntimeReasonEmptyError   = "empty_error"
	qVectorRuntimeReasonOpError      = "unsupported_op"
)

type qFrameVectorRuntimeExecutionAdapter struct {
	cf *CompiledFunction
}

func (cf *CompiledFunction) qFrameVectorRuntimeExecutionAdapter() qFrameVectorRuntimeExecutionAdapter {
	return qFrameVectorRuntimeExecutionAdapter{cf: cf}
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameLen(frameVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "len"
	out, err := executeFrameLenValue(frameVal)
	if err != nil {
		a.recordFrame("FrameLen", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameLen", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameColumn(frameVal, nameVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "column"
	if !nameVal.IsString() {
		a.recordFrame("FrameColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), fmt.Errorf("FrameColumn column name must be a string constant")
	}
	out, err := executeFrameColumnValue(frameVal, nameVal.Str())
	if err != nil {
		a.recordFrame("FrameColumn", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameColumn", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameMask(frameVal, specVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "mask"
	out, err := executeFrameMaskValue(frameVal, specVal)
	if err != nil {
		a.recordFrame("FrameMask", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameMask", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameProject(frameVal, namesVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "project"
	names, err := frameProjectColumnNames(namesVal)
	if err != nil {
		a.recordFrame("FrameProject", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	out, err := executeFrameProjectValue(frameVal, names)
	if err != nil {
		a.recordFrame("FrameProject", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameProject", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameFilter(frameVal, maskVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "filter"
	out, err := executeFrameFilterValue(frameVal, maskVal)
	if err != nil {
		a.recordFrame("FrameFilter", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameFilter", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeFrameFilterProject(frameVal, maskVal, namesVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "filter/project"
	names, err := frameProjectColumnNames(namesVal)
	if err != nil {
		a.recordFrame("FrameFilterProject", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	out, err := executeFrameFilterProjectValue(frameVal, maskVal, names)
	if err != nil {
		a.recordFrame("FrameFilterProject", shape, route, "error", frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("FrameFilterProject", shape, route, "success", frameVal)
	return out, nil
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
		a.recordFrameWithReason("QFrameSelectColumn", shape, route, "error", qFrameSelectColumnReasonPlanBuildError, frameVal)
		return runtime.NilValue(), err
	}
	out, err := executeQFrameSelectColumnPlannedValue(constants, a.cf.QFrameSelectColumnSpecs, specIdx, plan, frameVal, argVal, hasArg)
	if err != nil {
		a.recordFrameWithReason("QFrameSelectColumn", shape, route, "error", qFrameSelectColumnReasonExecutionError, frameVal)
		return runtime.NilValue(), err
	}
	a.recordFrame("QFrameSelectColumn", shape, route, "success", frameVal)
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeQVectorWhereReduce(instrID, opCode int, maskVal, trueVal, falseVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "compare/vector-where/vector-reduce"
	out, err := executeQVectorWhereReduceValue(opCode, maskVal, trueVal, falseVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "QVectorWhereReduce", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "QVectorWhereReduce", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeQVectorGatherReduce(instrID, opCode int, vectorVal, indexVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "gather/vector-reduce"
	out, err := executeQVectorGatherReduceValue(opCode, vectorVal, indexVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "QVectorGatherReduce", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "QVectorGatherReduce", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeVectorGather(instrID int, vectorVal, indexVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "vector-gather"
	out, err := executeVectorGatherValue(vectorVal, indexVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "VectorGather", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "VectorGather", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeVectorCompare(instrID, opCode int, leftVal, rightVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "vector-compare"
	out, err := executeVectorCompareValue(opCode, leftVal, rightVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "VectorCompare", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "VectorCompare", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeVectorMask(instrID, opCode int, leftVal, rightVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "vector-mask"
	out, err := executeVectorMaskValue(opCode, leftVal, rightVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "VectorMask", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "VectorMask", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeVectorWhere(instrID int, maskVal, trueVal, falseVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "vector-where"
	out, err := executeVectorWhereValue(maskVal, trueVal, falseVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "VectorWhere", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "VectorWhere", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeVectorReduce(instrID, opCode int, vectorVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "vector/vector-reduce"
	out, err := executeVectorReduceValue(opCode, vectorVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "VectorReduce", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "VectorReduce", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) executeVectorScan(instrID int, vectorVal runtime.Value, route qTypedRuntimeExecutionRoute) (runtime.Value, error) {
	const shape = "vector-scan"
	out, err := executeVectorScanValue(vectorVal)
	if err != nil {
		a.recordVectorByInstrIDWithReason(instrID, "VectorScan", shape, route, "error", qVectorRuntimeReasonForError(err))
		return runtime.NilValue(), err
	}
	a.recordVectorByInstrID(instrID, "VectorScan", shape, route, "success")
	return out, nil
}

func (a qFrameVectorRuntimeExecutionAdapter) recordFrame(kernel, shape string, route qTypedRuntimeExecutionRoute, outcome string, frameVal runtime.Value) {
	if a.cf == nil {
		return
	}
	a.cf.recordQKernelExecutionForFrame(qFrameRuntimeExecutionSource, kernel, shape, string(route), outcome, frameVal)
}

func (a qFrameVectorRuntimeExecutionAdapter) recordFrameWithReason(kernel, shape string, route qTypedRuntimeExecutionRoute, outcome, reasonCode string, frameVal runtime.Value) {
	if a.cf == nil {
		return
	}
	a.cf.recordQKernelExecutionWithPipelineShapeAndReason(qFrameRuntimeExecutionSource, kernel, shape, "", string(route), outcome, qKernelSchemaHashForValue(frameVal), reasonCode)
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

func (a qFrameVectorRuntimeExecutionAdapter) recordVectorByInstrIDWithReason(instrID int, kernel, fallbackShape string, route qTypedRuntimeExecutionRoute, outcome, reasonCode string) {
	if a.cf == nil {
		return
	}
	a.cf.recordQKernelExecutionWithPipelineShapeAndReason(qVectorRuntimeExecutionSource, kernel, a.cf.qVectorRuntimeKernelShape(instrID, fallbackShape), "", string(route), outcome, "", reasonCode)
}

func qVectorRuntimeReasonForError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, runtime.ErrDenseArrayOperand):
		return qVectorRuntimeReasonOperandError
	case errors.Is(err, runtime.ErrDenseArrayDType):
		return qVectorRuntimeReasonDTypeError
	case errors.Is(err, runtime.ErrDenseArrayLength):
		return qVectorRuntimeReasonLengthError
	case errors.Is(err, runtime.ErrDenseArrayEmpty):
		return qVectorRuntimeReasonEmptyError
	case errors.Is(err, runtime.ErrDenseArrayReduceOp), errors.Is(err, runtime.ErrDenseArrayMaskOp):
		return qVectorRuntimeReasonOpError
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "must be dense array"):
		return qVectorRuntimeReasonOperandError
	case strings.Contains(msg, " is not a comparison"):
		return qVectorRuntimeReasonOpError
	default:
		return ""
	}
}
