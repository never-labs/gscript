// Package gscript is the public Go embedding API for GScript.
//
// The legacy github.com/never-labs/gscript/gscript package remains available,
// but new embedders should import github.com/never-labs/gscript.
package gscript

import (
	"context"

	legacy "github.com/never-labs/gscript/gscript"
)

type (
	VM      = legacy.VM
	Program = legacy.Program
	Option  = legacy.Option

	CompileOption = legacy.CompileOption

	LibFlags        = legacy.LibFlags
	CapabilityFlags = legacy.CapabilityFlags
	SecurityPolicy  = legacy.SecurityPolicy

	Kind  = legacy.Kind
	Value = legacy.Value

	Module           = legacy.Module
	ModuleFromOption = legacy.ModuleFromOption

	Error                  = legacy.Error
	ErrorKind              = legacy.ErrorKind
	HostCallbackError      = legacy.HostCallbackError
	HostCallbackPanicError = legacy.HostCallbackPanicError
	ExitError              = legacy.ExitError
	BudgetError            = legacy.BudgetError

	LLMProvider        = legacy.LLMProvider
	LLMMessage         = legacy.LLMMessage
	LLMTool            = legacy.LLMTool
	LLMToolCall        = legacy.LLMToolCall
	LLMTurnRequest     = legacy.LLMTurnRequest
	LLMTurnResult      = legacy.LLMTurnResult
	LLMTurnUsage       = legacy.LLMTurnUsage
	LLMTraceEvent      = legacy.LLMTraceEvent
	LLMTraceSink       = legacy.LLMTraceSink
	LLMRecordSink      = legacy.LLMRecordSink
	LLMProviderConfig  = legacy.LLMProviderConfig
	LLMProviderFactory = legacy.LLMProviderFactory
	LLMRecord          = legacy.LLMRecord

	LLMTraceRecorder        = legacy.LLMTraceRecorder
	LLMRecorder             = legacy.LLMRecorder
	LLMReplayProvider       = legacy.LLMReplayProvider
	LLMReplayMismatchError  = legacy.LLMReplayMismatchError
	LLMReplayExhaustedError = legacy.LLMReplayExhaustedError

	CommandLLMProvider             = legacy.CommandLLMProvider
	OpenAICompatibleLLMProvider    = legacy.OpenAICompatibleLLMProvider
	OpenAICompatibleLLMError       = legacy.OpenAICompatibleLLMError
	AnthropicCompatibleLLMProvider = legacy.AnthropicCompatibleLLMProvider
	AnthropicCompatibleLLMError    = legacy.AnthropicCompatibleLLMError

	HotLoader       = legacy.HotLoader
	HotLoaderOption = legacy.HotLoaderOption
	ReloadResult    = legacy.ReloadResult
	ModuleHandle    = legacy.ModuleHandle
	HotInstance     = legacy.HotInstance

	Pool          = legacy.Pool
	PoolResetFunc = legacy.PoolResetFunc
)

const (
	LibString    = legacy.LibString
	LibTable     = legacy.LibTable
	LibMath      = legacy.LibMath
	LibIO        = legacy.LibIO
	LibOS        = legacy.LibOS
	LibCoroutine = legacy.LibCoroutine
	LibHTTP      = legacy.LibHTTP
	LibGL        = legacy.LibGL
	LibJSON      = legacy.LibJSON
	LibBase64    = legacy.LibBase64
	LibHash      = legacy.LibHash
	LibFS        = legacy.LibFS
	LibPath      = legacy.LibPath
	LibTime      = legacy.LibTime
	LibNet       = legacy.LibNet
	LibVec       = legacy.LibVec
	LibColor     = legacy.LibColor
	LibRegexp    = legacy.LibRegexp
	LibUTF8      = legacy.LibUTF8
	LibBit32     = legacy.LibBit32
	LibBinary    = legacy.LibBinary
	LibBits      = legacy.LibBits
	LibBytes     = legacy.LibBytes
	LibCSV       = legacy.LibCSV
	LibURL       = legacy.LibURL
	LibUUID      = legacy.LibUUID
	LibProcess   = legacy.LibProcess
	LibScript    = legacy.LibScript
	LibDebug     = legacy.LibDebug
	LibTestkit   = legacy.LibTestkit
	LibMatrix    = legacy.LibMatrix
	LibRand      = legacy.LibRand
	LibSort      = legacy.LibSort
	LibEncoding  = legacy.LibEncoding
	LibCompress  = legacy.LibCompress
	LibCrypto    = legacy.LibCrypto
	LibContainer = legacy.LibContainer
	LibLog       = legacy.LibLog
	LibArray     = legacy.LibArray
	LibSoA       = legacy.LibSoA
	LibLLM       = legacy.LibLLM
	LibRL        = legacy.LibRL
	LibAll       = legacy.LibAll
	LibSafe      = legacy.LibSafe
	LibApp       = legacy.LibApp
	LibGame      = legacy.LibGame

	CapModuleLoading    = legacy.CapModuleLoading
	CapFilesystemRead   = legacy.CapFilesystemRead
	CapFilesystemWrite  = legacy.CapFilesystemWrite
	CapEnvironmentRead  = legacy.CapEnvironmentRead
	CapEnvironmentWrite = legacy.CapEnvironmentWrite
	CapFilesystem       = legacy.CapFilesystem
	CapEnvironment      = legacy.CapEnvironment
	CapAll              = legacy.CapAll
	CapSafe             = legacy.CapSafe

	KindNil       = legacy.KindNil
	KindBool      = legacy.KindBool
	KindInt       = legacy.KindInt
	KindFloat     = legacy.KindFloat
	KindString    = legacy.KindString
	KindTable     = legacy.KindTable
	KindFunction  = legacy.KindFunction
	KindCoroutine = legacy.KindCoroutine
	KindChannel   = legacy.KindChannel
	KindUnknown   = legacy.KindUnknown

	ErrLex     = legacy.ErrLex
	ErrParse   = legacy.ErrParse
	ErrRuntime = legacy.ErrRuntime
	ErrScript  = legacy.ErrScript
)

func New(opts ...Option) *VM { return legacy.New(opts...) }

func Compile(src string, opts ...CompileOption) (*Program, error) {
	return legacy.Compile(src, opts...)
}

func CompileContext(ctx context.Context, src string, opts ...CompileOption) (*Program, error) {
	return legacy.CompileContext(ctx, src, opts...)
}

func CompileFile(path string, opts ...CompileOption) (*Program, error) {
	return legacy.CompileFile(path, opts...)
}

func CompileFileContext(ctx context.Context, path string, opts ...CompileOption) (*Program, error) {
	return legacy.CompileFileContext(ctx, path, opts...)
}

func WithSourceName(name string) CompileOption { return legacy.WithSourceName(name) }

func WithArgs(script string, args ...string) Option { return legacy.WithArgs(script, args...) }
func WithLibs(libs LibFlags) Option                 { return legacy.WithLibs(libs) }
func WithCapabilities(caps CapabilityFlags) Option  { return legacy.WithCapabilities(caps) }
func WithModuleLoading(enabled bool) Option         { return legacy.WithModuleLoading(enabled) }
func WithFilesystem(enabled bool) Option            { return legacy.WithFilesystem(enabled) }
func WithFilesystemRead(enabled bool) Option        { return legacy.WithFilesystemRead(enabled) }
func WithFilesystemWrite(enabled bool) Option       { return legacy.WithFilesystemWrite(enabled) }
func WithEnvironment(enabled bool) Option           { return legacy.WithEnvironment(enabled) }
func WithEnvironmentRead(enabled bool) Option       { return legacy.WithEnvironmentRead(enabled) }
func WithEnvironmentWrite(enabled bool) Option      { return legacy.WithEnvironmentWrite(enabled) }
func WithEnvironmentAllowlist(names ...string) Option {
	return legacy.WithEnvironmentAllowlist(names...)
}
func WithProcessExecution(enabled bool) Option      { return legacy.WithProcessExecution(enabled) }
func WithProcessShell(enabled bool) Option          { return legacy.WithProcessShell(enabled) }
func WithFilesystemRoot(root string) Option         { return legacy.WithFilesystemRoot(root) }
func WithDynamicEval(enabled bool) Option           { return legacy.WithDynamicEval(enabled) }
func WithNetworkAccess(enabled bool) Option         { return legacy.WithNetworkAccess(enabled) }
func WithDebugAccess(enabled bool) Option           { return legacy.WithDebugAccess(enabled) }
func WithTestkitAccess(enabled bool) Option         { return legacy.WithTestkitAccess(enabled) }
func WithSandbox() Option                           { return legacy.WithSandbox() }
func SecuritySandbox() Option                       { return legacy.SecuritySandbox() }
func WithSecurity(policy SecurityPolicy) Option     { return legacy.WithSecurity(policy) }
func WithRequirePath(path string) Option            { return legacy.WithRequirePath(path) }
func WithGoImports(imports map[string]any) Option   { return legacy.WithGoImports(imports) }
func WithPrint(fn func(args ...interface{})) Option { return legacy.WithPrint(fn) }
func WithMaxSteps(max int64) Option                 { return legacy.WithMaxSteps(max) }
func WithMaxNativeCalls(max int64) Option           { return legacy.WithMaxNativeCalls(max) }
func WithMaxCallDepth(max int64) Option             { return legacy.WithMaxCallDepth(max) }
func WithMaxGoroutines(max int64) Option            { return legacy.WithMaxGoroutines(max) }
func WithMaxChannelCapacity(max int64) Option       { return legacy.WithMaxChannelCapacity(max) }
func WithMaxHostResultBytes(max int64) Option       { return legacy.WithMaxHostResultBytes(max) }
func WithMaxModuleBytes(max int64) Option           { return legacy.WithMaxModuleBytes(max) }
func WithMaxModuleDepth(max int64) Option           { return legacy.WithMaxModuleDepth(max) }
func WithMaxFilesystemReadBytes(max int64) Option   { return legacy.WithMaxFilesystemReadBytes(max) }
func WithMaxFilesystemWriteBytes(max int64) Option {
	return legacy.WithMaxFilesystemWriteBytes(max)
}
func WithVM() Option      { return legacy.WithVM() }
func WithJIT() Option     { return legacy.WithJIT() }
func WithTracing() Option { return legacy.WithTracing() }

func Nil() Value                          { return legacy.Nil() }
func Bool(v bool) Value                   { return legacy.Bool(v) }
func Int(v int64) Value                   { return legacy.Int(v) }
func Float(v float64) Value               { return legacy.Float(v) }
func String(v string) Value               { return legacy.String(v) }
func Decode(v interface{}) (Value, error) { return legacy.Decode(v) }
func Encode(v Value) (interface{}, error) { return legacy.Encode(v) }

func ModuleFrom(source interface{}, opts ...ModuleFromOption) (Module, error) {
	return legacy.ModuleFrom(source, opts...)
}

func WithModuleNameMapper(mapper func(string) string) ModuleFromOption {
	return legacy.WithModuleNameMapper(mapper)
}

func WithModuleExactNames() ModuleFromOption { return legacy.WithModuleExactNames() }

func NewLLMTraceRecorder(events ...LLMTraceEvent) *LLMTraceRecorder {
	return legacy.NewLLMTraceRecorder(events...)
}

func NewLLMRecorder(records ...LLMRecord) *LLMRecorder {
	return legacy.NewLLMRecorder(records...)
}

func NewLLMReplayProvider(records []LLMRecord) *LLMReplayProvider {
	return legacy.NewLLMReplayProvider(records)
}

func WithLLMProvider(provider LLMProvider) Option { return legacy.WithLLMProvider(provider) }
func WithLLMProviderFactory(factory LLMProviderFactory) Option {
	return legacy.WithLLMProviderFactory(factory)
}
func WithLLMTrace(sink LLMTraceSink) Option     { return legacy.WithLLMTrace(sink) }
func WithLLMRecorder(sink LLMRecordSink) Option { return legacy.WithLLMRecorder(sink) }
func WithLLMReplay(records []LLMRecord) Option  { return legacy.WithLLMReplay(records) }
func WithLLMCommand(command string, args ...string) Option {
	return legacy.WithLLMCommand(command, args...)
}
func WithOpenAICompatibleLLM(endpoint, apiKey, model string) Option {
	return legacy.WithOpenAICompatibleLLM(endpoint, apiKey, model)
}
func WithAnthropicCompatibleLLM(endpoint, apiKey, model string) Option {
	return legacy.WithAnthropicCompatibleLLM(endpoint, apiKey, model)
}

func NewHotLoader(opts ...HotLoaderOption) *HotLoader {
	return legacy.NewHotLoader(opts...)
}

func WithHotLoaderCompileOptions(opts ...CompileOption) HotLoaderOption {
	return legacy.WithHotLoaderCompileOptions(opts...)
}

func WithHotLoaderVMOptions(opts ...Option) HotLoaderOption {
	return legacy.WithHotLoaderVMOptions(opts...)
}

func NewPool(max int, init func() *VM) *Pool {
	return legacy.NewPool(max, init)
}

func NewPoolWithReset(max int, init func() *VM, reset PoolResetFunc) *Pool {
	return legacy.NewPoolWithReset(max, init, reset)
}
