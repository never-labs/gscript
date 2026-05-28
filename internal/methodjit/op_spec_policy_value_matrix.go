package methodjit

var opMatrixNativePolicies = [...]bool{
	OpMatrixDense:          true,
	OpMatrixGetF:           true,
	OpMatrixSetF:           true,
	OpMatrixFlat:           true,
	OpMatrixStride:         true,
	OpMatrixLoadFAt:        true,
	OpMatrixStoreFAt:       true,
	OpMatrixRowPtr:         true,
	OpMatrixLoadFRow:       true,
	OpMatrixStoreFRow:      true,
	OpMatrixLoadFRowConst:  true,
	OpMatrixStoreFRowConst: true,
}

var opMatrixLoweredOpPolicies = [...]opTargetPolicy{
	OpMatrixGetF: {Op: OpMatrixLoadFAt, Set: true},
	OpMatrixSetF: {Op: OpMatrixStoreFAt, Set: true},
}

var opMatrixRowLoweredOpPolicies = [...]opTargetPolicy{
	OpMatrixLoadFAt:  {Op: OpMatrixLoadFRow, Set: true},
	OpMatrixStoreFAt: {Op: OpMatrixStoreFRow, Set: true},
}

var opMatrixRowConstLoweredOpPolicies = [...]opTargetPolicy{
	OpMatrixLoadFAt:  {Op: OpMatrixLoadFRowConst, Set: true},
	OpMatrixStoreFAt: {Op: OpMatrixStoreFRowConst, Set: true},
}

var opMatrixNestedLoweredOpPolicies = [...]opTargetPolicy{
	OpTableArrayNestedLoad: {Op: OpMatrixLoadFAt, Set: true},
}
