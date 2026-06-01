package modules

import "github.com/never-labs/leia/internal/runtime"

type (
	DenseArray           = runtime.DenseArray
	DenseArrayDType      = runtime.DenseArrayDType
	Channel              = runtime.Channel
	Interpreter          = runtime.Interpreter
	GoFunction           = runtime.GoFunction
	LLMMessage           = runtime.LLMMessage
	LLMProvider          = runtime.LLMProvider
	LLMProviderConfig    = runtime.LLMProviderConfig
	LLMProviderFactory   = runtime.LLMProviderFactory
	LLMTool              = runtime.LLMTool
	LLMToolCall          = runtime.LLMToolCall
	LLMTraceEvent        = runtime.LLMTraceEvent
	LLMTraceSink         = runtime.LLMTraceSink
	LLMTurnRequest       = runtime.LLMTurnRequest
	LLMTurnResult        = runtime.LLMTurnResult
	LLMTurnUsage         = runtime.LLMTurnUsage
	ScriptFunctionCaller = runtime.ScriptFunctionCaller
	SoA                  = runtime.SoA
	SoAAffinePlan        = runtime.SoAAffinePlan
	SoAAffineTerm        = runtime.SoAAffineTerm
	SoAShapeSnapshot     = runtime.SoAShapeSnapshot
	Table                = runtime.Table
	Value                = runtime.Value
)

const (
	TypeBool   = runtime.TypeBool
	TypeFloat  = runtime.TypeFloat
	TypeInt    = runtime.TypeInt
	TypeNil    = runtime.TypeNil
	TypeString = runtime.TypeString
	TypeTable  = runtime.TypeTable

	NativeKindStdSoAAffineMany = runtime.NativeKindStdSoAAffineMany

	LLMProviderErrorProvider = runtime.LLMProviderErrorProvider
)

var (
	ApplySoAAffinePlans           = runtime.ApplySoAAffinePlans
	BoolValue                     = runtime.BoolValue
	CheckProjectedHostStringBytes = runtime.CheckProjectedHostStringBytes
	CheckHostResultBytes          = runtime.CheckHostResultBytes
	ClassifyLLMProviderError      = runtime.ClassifyLLMProviderError
	ConcatOperandString           = runtime.ConcatOperandString
	ContextCancelledValue         = runtime.ContextCancelledValue
	DenseArrayBool                = runtime.DenseArrayBool
	DenseArrayF64                 = runtime.DenseArrayF64
	DenseArrayI64                 = runtime.DenseArrayI64
	DenseArrayReduce              = runtime.DenseArrayReduce
	DenseArrayReduceSum           = runtime.DenseArrayReduceSum
	DenseArrayValue               = runtime.DenseArrayValue
	FloatValue                    = runtime.FloatValue
	FunctionValue                 = runtime.FunctionValue
	IntValue                      = runtime.IntValue
	JSONGoToValue                 = runtime.JSONGoToValue
	New                           = runtime.NewCore
	NewAppendArrayTable           = runtime.NewAppendArrayTable
	NewDenseArrayF64              = runtime.NewDenseArrayF64
	NewDenseArrayI64              = runtime.NewDenseArrayI64
	NewDenseArrayOfLen            = runtime.NewDenseArrayOfLen
	NewDenseMatrix                = runtime.NewDenseMatrix
	NewSoA                        = runtime.NewSoA
	NewSequentialArrayTable       = runtime.NewSequentialArrayTable
	NewTable                      = runtime.NewTable
	NewTableSized                 = runtime.NewTableSized
	NilValue                      = runtime.NilValue
	ReadAllWithHostResultLimit    = runtime.ReadAllWithHostResultLimit
	StringLen                     = runtime.StringLen
	StringValue                   = runtime.StringValue
	SoAValue                      = runtime.SoAValue
	ScriptContextDoneAndErr       = runtime.ScriptContextDoneAndErr
	StdSoAAffineManyIdentityPtr   = runtime.StdSoAAffineManyIdentityPtr
	TableValue                    = runtime.TableValue
)
