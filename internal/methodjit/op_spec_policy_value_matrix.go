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
