package bind

import "github.com/never-labs/leia/internal/runtime"

type (
	DenseArray           = runtime.DenseArray
	DenseArrayBinaryOp   = runtime.DenseArrayBinaryOp
	DenseArrayDType      = runtime.DenseArrayDType
	Channel              = runtime.Channel
	Interpreter          = runtime.Interpreter
	GoFunction           = runtime.GoFunction
	LLMMessage           = runtime.LLMMessage
	LLMProvider          = runtime.LLMProvider
	LLMProviderConfig    = runtime.LLMProviderConfig
	LLMProviderFactory   = runtime.LLMProviderFactory
	LLMStreamEvent       = runtime.LLMStreamEvent
	LLMStreamSink        = runtime.LLMStreamSink
	LLMStreamingProvider = runtime.LLMStreamingProvider
	LLMTool              = runtime.LLMTool
	LLMToolCall          = runtime.LLMToolCall
	LLMTraceEvent        = runtime.LLMTraceEvent
	LLMTraceSink         = runtime.LLMTraceSink
	LLMTurnRequest       = runtime.LLMTurnRequest
	LLMTurnResult        = runtime.LLMTurnResult
	LLMTurnUsage         = runtime.LLMTurnUsage
	NativePayloadInfo    = runtime.NativePayloadInfo
	NativePayloadKind    = runtime.NativePayloadKind
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
	NativeKindStdQSQL          = runtime.NativeKindStdQSQL
	NativeKindStdQSelect       = runtime.NativeKindStdQSelect

	NativePayloadNone       = runtime.NativePayloadNone
	NativePayloadDataColumn = runtime.NativePayloadDataColumn
	NativePayloadDataFrame  = runtime.NativePayloadDataFrame
	NativePayloadKeyedFrame = runtime.NativePayloadKeyedFrame

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
	DenseArrayAdd                 = runtime.DenseArrayAdd
	DenseArraySub                 = runtime.DenseArraySub
	DenseArrayMul                 = runtime.DenseArrayMul
	DenseArrayDiv                 = runtime.DenseArrayDiv
	DenseArrayEQ                  = runtime.DenseArrayEQ
	DenseArrayNE                  = runtime.DenseArrayNE
	DenseArrayLT                  = runtime.DenseArrayLT
	DenseArrayLE                  = runtime.DenseArrayLE
	DenseArrayGT                  = runtime.DenseArrayGT
	DenseArrayGE                  = runtime.DenseArrayGE
	DenseArrayElementwise         = runtime.DenseArrayElementwise
	DenseArrayF64                 = runtime.DenseArrayF64
	DenseArrayI64                 = runtime.DenseArrayI64
	DenseArrayString              = runtime.DenseArrayString
	DenseArrayReduce              = runtime.DenseArrayReduce
	DenseArrayReduceSum           = runtime.DenseArrayReduceSum
	DenseArrayScan                = runtime.DenseArrayScan
	DenseArrayValue               = runtime.DenseArrayValue
	FloatValue                    = runtime.FloatValue
	FunctionValue                 = runtime.FunctionValue
	IntValue                      = runtime.IntValue
	JSONGoToValue                 = runtime.JSONGoToValue
	New                           = runtime.NewCore
	NewAppendArrayTable           = runtime.NewAppendArrayTable
	NewDenseArrayBool             = runtime.NewDenseArrayBool
	NewDenseArrayBoolOwned        = runtime.NewDenseArrayBoolOwned
	NewDenseArrayF64              = runtime.NewDenseArrayF64
	NewDenseArrayF64Owned         = runtime.NewDenseArrayF64Owned
	NewDenseArrayI64              = runtime.NewDenseArrayI64
	NewDenseArrayI64Owned         = runtime.NewDenseArrayI64Owned
	NewDenseArrayString           = runtime.NewDenseArrayString
	NewDenseArrayStringOwned      = runtime.NewDenseArrayStringOwned
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
	StdQSQLIdentityPtr            = runtime.StdQSQLIdentityPtr
	StdQSelectIdentityPtr         = runtime.StdQSelectIdentityPtr
	TableValue                    = runtime.TableValue
)
