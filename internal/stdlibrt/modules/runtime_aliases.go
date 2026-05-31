package modules

import "github.com/never-labs/gscript/internal/runtime"

type (
	Interpreter = runtime.Interpreter
	GoFunction  = runtime.GoFunction
	Table       = runtime.Table
	Value       = runtime.Value
)

var (
	BoolValue                     = runtime.BoolValue
	CheckProjectedHostStringBytes = runtime.CheckProjectedHostStringBytes
	FloatValue                    = runtime.FloatValue
	FunctionValue                 = runtime.FunctionValue
	IntValue                      = runtime.IntValue
	New                           = runtime.NewCore
	NewAppendArrayTable           = runtime.NewAppendArrayTable
	NewSequentialArrayTable       = runtime.NewSequentialArrayTable
	NewTable                      = runtime.NewTable
	NewTableSized                 = runtime.NewTableSized
	NilValue                      = runtime.NilValue
	ReadAllWithHostResultLimit    = runtime.ReadAllWithHostResultLimit
	StringLen                     = runtime.StringLen
	StringValue                   = runtime.StringValue
	TableValue                    = runtime.TableValue
)
