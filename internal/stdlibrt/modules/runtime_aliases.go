package modules

import "github.com/never-labs/gscript/internal/runtime"

type (
	DenseArray      = runtime.DenseArray
	DenseArrayDType = runtime.DenseArrayDType
	Interpreter     = runtime.Interpreter
	GoFunction      = runtime.GoFunction
	Table           = runtime.Table
	Value           = runtime.Value
)

const (
	TypeFloat = runtime.TypeFloat
	TypeInt   = runtime.TypeInt
)

var (
	BoolValue                     = runtime.BoolValue
	CheckProjectedHostStringBytes = runtime.CheckProjectedHostStringBytes
	DenseArrayBool                = runtime.DenseArrayBool
	DenseArrayF64                 = runtime.DenseArrayF64
	DenseArrayI64                 = runtime.DenseArrayI64
	DenseArrayReduce              = runtime.DenseArrayReduce
	DenseArrayReduceSum           = runtime.DenseArrayReduceSum
	DenseArrayValue               = runtime.DenseArrayValue
	FloatValue                    = runtime.FloatValue
	FunctionValue                 = runtime.FunctionValue
	IntValue                      = runtime.IntValue
	New                           = runtime.NewCore
	NewAppendArrayTable           = runtime.NewAppendArrayTable
	NewDenseArrayBool             = runtime.NewDenseArrayBool
	NewDenseArrayF64              = runtime.NewDenseArrayF64
	NewDenseArrayI64              = runtime.NewDenseArrayI64
	NewDenseArrayOfLen            = runtime.NewDenseArrayOfLen
	NewSequentialArrayTable       = runtime.NewSequentialArrayTable
	NewTable                      = runtime.NewTable
	NewTableSized                 = runtime.NewTableSized
	NilValue                      = runtime.NilValue
	ReadAllWithHostResultLimit    = runtime.ReadAllWithHostResultLimit
	StringLen                     = runtime.StringLen
	StringValue                   = runtime.StringValue
	TableValue                    = runtime.TableValue
)
