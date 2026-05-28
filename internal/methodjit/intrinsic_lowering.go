package methodjit

type intrinsicCallLowering struct {
	module   string
	field    string
	argCount int
	op       Op
	result   Type
	note     string
}

var intrinsicCallLowerings = [...]intrinsicCallLowering{
	{module: "math", field: "sqrt", argCount: 1, op: OpSqrt, result: TypeFloat, note: "intrinsic: math.sqrt -> OpSqrt"},
	{module: "math", field: "floor", argCount: 1, op: OpFloor, result: TypeInt, note: "intrinsic: math.floor -> OpFloor"},
	{module: "matrix", field: "dense", argCount: 2, op: OpMatrixDense, result: TypeTable, note: "intrinsic: matrix.dense -> OpMatrixDense"},
	{module: "matrix", field: "getf", argCount: 3, op: OpMatrixGetF, result: TypeFloat, note: "intrinsic: matrix.getf -> OpMatrixGetF"},
	{module: "matrix", field: "setf", argCount: 4, op: OpMatrixSetF, result: TypeUnknown, note: "intrinsic: matrix.setf -> OpMatrixSetF"},
}

func lookupIntrinsicCallLowering(module, field string, userArgCount int) (intrinsicCallLowering, bool) {
	for _, lowering := range intrinsicCallLowerings {
		if lowering.module == module && lowering.field == field && lowering.argCount == userArgCount {
			return lowering, true
		}
	}
	return intrinsicCallLowering{}, false
}

func applyIntrinsicCallLowering(instr *Instr, lowering intrinsicCallLowering) {
	instr.Op = lowering.op
	instr.Type = lowering.result
	instr.Args = append([]*Value(nil), instr.Args[1:]...)
	instr.Aux = 0
	instr.Aux2 = 0
}
