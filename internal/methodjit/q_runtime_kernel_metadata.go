package methodjit

func qRuntimePrimitiveExecutionMetadata(op Op) (source, kernel, shape, route string, ok bool) {
	route = "typed_runtime_op_exit"
	switch op {
	case OpFrameLen:
		return "methodjit_q_frame_runtime", "FrameLen", "len", route, true
	case OpFrameColumn:
		return "methodjit_q_frame_runtime", "FrameColumn", "column", route, true
	case OpFrameMask:
		return "methodjit_q_frame_runtime", "FrameMask", "mask", route, true
	case OpFrameProject:
		return "methodjit_q_frame_runtime", "FrameProject", "project", route, true
	case OpFrameFilter:
		return "methodjit_q_frame_runtime", "FrameFilter", "filter", route, true
	case OpFrameFilterProject:
		return "methodjit_q_frame_runtime", "FrameFilterProject", "filter/project", route, true
	case OpFrameGather:
		return "methodjit_q_frame_runtime", "FrameGather", "gather", route, true
	case OpFrameSlice:
		return "methodjit_q_frame_runtime", "FrameSlice", "slice", route, true
	case OpFrameOrder:
		return "methodjit_q_frame_runtime", "FrameOrder", "order", route, true
	case OpFrameOrderGather:
		return "methodjit_q_frame_runtime", "FrameOrderGather", "order/gather", route, true
	case OpFrameProjectColumn:
		return "methodjit_q_frame_runtime", "FrameProjectColumn", "project/column", route, true
	case OpFrameFilterProjectColumn:
		return "methodjit_q_frame_runtime", "FrameFilterProjectColumn", "filter/project/column", route, true
	case OpFrameGroupAggregate:
		return "methodjit_q_frame_runtime", "FrameGroupAggregate", "group/aggregate", route, true
	case OpVectorGather:
		return "methodjit_q_vector_runtime", "VectorGather", "vector-gather", route, true
	case OpVectorCompare:
		return "methodjit_q_vector_runtime", "VectorCompare", "vector-compare", route, true
	case OpVectorMask:
		return "methodjit_q_vector_runtime", "VectorMask", "vector-mask", route, true
	case OpVectorWhere:
		return "methodjit_q_vector_runtime", "VectorWhere", "vector-where", route, true
	case OpVectorReduce:
		return "methodjit_q_vector_runtime", "VectorReduce", "vector/vector-reduce", route, true
	case OpQVectorGatherReduce:
		return "methodjit_q_vector_runtime", "QVectorGatherReduce", "gather/vector-reduce", route, true
	case OpQEvalPipelinePlan:
		return "methodjit_q_eval_runtime", "QEvalPipelinePlan", "q-eval/pipeline-plan", route, true
	case OpQSQLKernelPlan:
		return QSQLKernelRuntimeSource, "QSQLKernelPlan", "qsql/kernel-plan", route, true
	case OpQEvalSessionEval:
		return "methodjit_q_eval_runtime", "QEvalSessionEval", "q-eval/session-eval", route, true
	case OpVectorScan:
		return "methodjit_q_vector_runtime", "VectorScan", "vector-scan", route, true
	default:
		return "", "", "", "", false
	}
}
