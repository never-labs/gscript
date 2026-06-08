package bind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const qSymbolVectorMarker = "__q_symbol_vector"
const qKeyedFrameMarker = "__q_keyed_frame"
const qDictKeysMarker = "__q_dict_keys"

const qSQLPlanCacheLimit = 128
const qQueryKernelSupportCacheLimit = 128
const qEvalCacheLimit = 256

const (
	qStatsDomainSemanticCache = "semantic_cache"
	qStatsDomainEvalCache     = "eval_cache"
	qStatsDomainJITExecution  = "jit_execution"
	qStatsDomainJITLowering   = "jit_lowering"
	qStatsSourceQBind         = "q_bind"
	qStatsSourceMethodJIT     = "methodjit_diagnose"

	qFallbackKernelUnsupported = "kernel_unsupported"
	qFallbackKernelCompileErr  = "kernel_compile_error"
	qFallbackSourceErr         = "source_error"
	qFallbackJoinErr           = "join_error"
	qFallbackMutationPlan      = "mutation_plan"
	qQueryKernelSupported      = "query_kernel_supported"
	qFallbackQueryKernel       = "query_kernel_unsupported"

	qKernelReasonSupported         = "kernel_supported"
	qKernelReasonMutationPlan      = "mutation_plan"
	qKernelReasonSourceUnavailable = "source_unavailable"
	qKernelReasonJoinUnavailable   = "join_unavailable"
	qKernelReasonCompileError      = "kernel_compile_error"
	qKernelReasonUnsupported       = "kernel_unsupported"
	qQueryKernelReasonUnsupported  = "query_kernel_unsupported"
	qQueryKernelReasonSelect       = "query_select_expression"
	qQueryKernelReasonOrder        = "query_order_expression"

	qFallbackFamilySupported = "supported"
	qFallbackFamilyKernel    = "kernel"
	qFallbackFamilySelect    = "select"
	qFallbackFamilyWhere     = "where"
	qFallbackFamilyOrder     = "order"
	qFallbackFamilyGroup     = "group"
	qFallbackFamilyAggregate = "aggregate"
	qFallbackFamilySource    = "source"
	qFallbackFamilyJoin      = "join"
	qFallbackFamilyMutation  = "mutation"
	qFallbackFamilySchema    = "schema"
	qFallbackFamilyCompile   = "compile"
)

type qSQLPlanTemplate struct {
	source       string
	sourcePath   string
	op           stdq.QueryKind
	literalFrame *data.Frame
	literalKeys  []data.Symbol
	hiddenCols   []data.Symbol
	plan         data.QueryPlan
	execDict     *stdq.ExecDictPlan
	mutation     *stdq.MutationPlan
	join         *stdq.JoinPlan
	joins        []*stdq.JoinPlan
}

type qSQLPlanCacheStats struct {
	TemplateHits            int
	TemplateMisses          int
	TemplateEvictions       int
	AlignedHits             int
	AlignedMisses           int
	AlignedEvictions        int
	KernelHits              int
	KernelMisses            int
	KernelEvictions         int
	KernelDecisionHits      int
	KernelDecisionMisses    int
	KernelDecisionEvictions int
	KernelKeys              []qSQLKernelCacheKeyStats
}

type qSQLKernelCacheKeyStats struct {
	Key       string
	Shape     string
	Hits      int
	Misses    int
	Evictions int
}

// QSQLKernelCacheKeyStatJSONRow is the stable external row shape for qSQL
// semantic kernel cache key statistics.
type QSQLKernelCacheKeyStatJSONRow struct {
	Key             string `json:"key"`
	Namespace       string `json:"namespace"`
	Kind            string `json:"kind"`
	SchemaHash      string `json:"schema_hash"`
	PlanFingerprint string `json:"plan_fingerprint"`
	Shape           string `json:"shape"`
	Hits            int    `json:"hits"`
	Misses          int    `json:"misses"`
	Evictions       int    `json:"evictions"`
}

type qSQLKernelShapeStat struct {
	Shape      string
	SchemaHash string
	Count      int
	Hits       int
	Misses     int
	Evictions  int
}

// QRuntimeKernelExecutionStat is the q.cache_stats-facing shape for MethodJIT
// typed-runtime q kernel execution observations. It intentionally stays
// separate from qSQL semantic cache hit/miss accounting.
type QRuntimeKernelExecutionStat struct {
	Source  string
	Kernel  string
	Shape   string
	Route   string
	Outcome string
	Count   uint64
}

// QRuntimeKernelLoweringStat is the q.cache_stats-facing shape for MethodJIT
// q typed-runtime kernel lowering fallbacks. These rows explain why a hot-path
// shape did not become a typed runtime kernel.
type QRuntimeKernelLoweringStat struct {
	Source       string
	Kind         string
	Kernel       string
	Shape        string
	Route        string
	Outcome      string
	ReasonFamily string
	ReasonCode   string
	Count        uint64
}

type qRuntimeKernelExecutionShapeStat struct {
	Source  string
	Shape   string
	Outcome string
	Count   uint64
}

type qRuntimeKernelExecutionKernelStat struct {
	Source  string
	Kernel  string
	Outcome string
	Count   uint64
}

type qRuntimeKernelExecutionRouteStat struct {
	Source  string
	Kernel  string
	Route   string
	Outcome string
	Count   uint64
}

type qRuntimeKernelLoweringShapeStat struct {
	Source       string
	Kind         string
	Shape        string
	Outcome      string
	ReasonFamily string
	ReasonCode   string
	Count        uint64
}

type qRuntimeKernelLoweringKernelStat struct {
	Source       string
	Kind         string
	Kernel       string
	Outcome      string
	ReasonFamily string
	ReasonCode   string
	Count        uint64
}

type qRuntimeKernelLoweringReasonStat struct {
	Source       string
	Kind         string
	ReasonFamily string
	ReasonCode   string
	Count        uint64
}

type qRuntimeKernelLoweringReasonShapeStat struct {
	Source       string
	Kind         string
	Kernel       string
	Shape        string
	Route        string
	Outcome      string
	ReasonFamily string
	ReasonCode   string
	Count        uint64
}

type qRuntimeKernelLoweringRouteStat struct {
	Source  string
	Kind    string
	Kernel  string
	Route   string
	Outcome string
	Count   uint64
}

type qRuntimeKernelExecutionShapeSummary struct {
	Executions uint64
	Successes  uint64
	Errors     uint64
	Routes     []qRuntimeKernelExecutionRouteStat
}

type qRuntimeKernelLoweringShapeSummary struct {
	Lowerings    uint64
	Supported    uint64
	Fallbacks    uint64
	Reasons      []qRuntimeKernelLoweringReasonStat
	ReasonShapes []qRuntimeKernelLoweringReasonShapeStat
	Routes       []qRuntimeKernelLoweringRouteStat
}

// QQueryKernelSupportKeyStatJSONRow is the stable external row shape for
// q.query native-kernel support cache key statistics.
type QQueryKernelSupportKeyStatJSONRow struct {
	Key             string `json:"key"`
	Namespace       string `json:"namespace"`
	Kind            string `json:"kind"`
	PlanFingerprint string `json:"plan_fingerprint"`
	Supported       bool   `json:"supported"`
	ReasonFamily    string `json:"reason_family"`
	ReasonCode      string `json:"reason_code"`
	SchemaHash      string `json:"schema_hash"`
	Shape           string `json:"shape"`
	Hits            int    `json:"hits"`
	Misses          int    `json:"misses"`
	Evictions       int    `json:"evictions"`
}

type qSQLKernelDecisionKeyStat struct {
	Key          string
	Shape        string
	ReasonCode   string
	ReasonFamily string
	Count        int
}

// QSQLKernelDecisionKeyStatJSONRow is the stable external row shape for qSQL
// unsupported kernel-decision cache key statistics.
type QSQLKernelDecisionKeyStatJSONRow struct {
	Key             string `json:"key"`
	Namespace       string `json:"namespace"`
	Kind            string `json:"kind"`
	SchemaHash      string `json:"schema_hash"`
	PlanFingerprint string `json:"plan_fingerprint"`
	Shape           string `json:"shape"`
	ReasonFamily    string `json:"reason_family"`
	ReasonCode      string `json:"reason_code"`
	Count           int    `json:"count"`
}

var (
	qRuntimeKernelExecutionStatsProviderMu      sync.Mutex
	qRuntimeKernelExecutionStatsProviderCurrent *qRuntimeKernelExecutionStatsProviderState
	qRuntimeKernelLoweringStatsProviderMu       sync.Mutex
	qRuntimeKernelLoweringStatsProviderCurrent  *qRuntimeKernelLoweringStatsProviderState
)

type qRuntimeKernelExecutionStatsProviderState struct {
	provider func() []QRuntimeKernelExecutionStat
	previous *qRuntimeKernelExecutionStatsProviderState
	active   bool
}

func SetQRuntimeKernelExecutionStatsProvider(provider func() []QRuntimeKernelExecutionStat) func() {
	qRuntimeKernelExecutionStatsProviderMu.Lock()
	state := &qRuntimeKernelExecutionStatsProviderState{
		provider: provider,
		previous: qRuntimeKernelExecutionStatsProviderCurrent,
		active:   true,
	}
	qRuntimeKernelExecutionStatsProviderCurrent = state
	qRuntimeKernelExecutionStatsProviderMu.Unlock()
	return func() {
		qRuntimeKernelExecutionStatsProviderMu.Lock()
		if state.active {
			state.active = false
			if qRuntimeKernelExecutionStatsProviderCurrent == state {
				qRuntimeKernelExecutionStatsProviderCurrent = qRuntimeKernelExecutionStatsNextActiveProvider(state.previous)
			}
		}
		qRuntimeKernelExecutionStatsProviderMu.Unlock()
	}
}

func qRuntimeKernelExecutionStatsNextActiveProvider(state *qRuntimeKernelExecutionStatsProviderState) *qRuntimeKernelExecutionStatsProviderState {
	for state != nil && !state.active {
		state = state.previous
	}
	return state
}

type qRuntimeKernelLoweringStatsProviderState struct {
	provider func() []QRuntimeKernelLoweringStat
	previous *qRuntimeKernelLoweringStatsProviderState
	active   bool
}

func SetQRuntimeKernelLoweringStatsProvider(provider func() []QRuntimeKernelLoweringStat) func() {
	qRuntimeKernelLoweringStatsProviderMu.Lock()
	state := &qRuntimeKernelLoweringStatsProviderState{
		provider: provider,
		previous: qRuntimeKernelLoweringStatsProviderCurrent,
		active:   true,
	}
	qRuntimeKernelLoweringStatsProviderCurrent = state
	qRuntimeKernelLoweringStatsProviderMu.Unlock()
	return func() {
		qRuntimeKernelLoweringStatsProviderMu.Lock()
		if state.active {
			state.active = false
			if qRuntimeKernelLoweringStatsProviderCurrent == state {
				qRuntimeKernelLoweringStatsProviderCurrent = qRuntimeKernelLoweringStatsNextActiveProvider(state.previous)
			}
		}
		qRuntimeKernelLoweringStatsProviderMu.Unlock()
	}
}

func qRuntimeKernelLoweringStatsNextActiveProvider(state *qRuntimeKernelLoweringStatsProviderState) *qRuntimeKernelLoweringStatsProviderState {
	for state != nil && !state.active {
		state = state.previous
	}
	return state
}

type qSQLKernelDecisionReasonStat struct {
	ReasonCode   string
	ReasonFamily string
	Count        int
}

type qSQLKernelDecisionShapeStat struct {
	Shape        string
	SchemaHash   string
	ReasonCode   string
	ReasonFamily string
	Count        int
}

type qEvalCacheStats struct {
	Hits      int
	Misses    int
	Evictions int
}

type qQueryKernelSupportCacheStats struct {
	Hits      int
	Misses    int
	Evictions int
}

type qQueryKernelSupportCacheEntry struct {
	Supported  bool
	ReasonCode string
	Reason     string
	SchemaHash string
	Shape      string
}

type qQueryKernelSupportCacheKeyStat struct {
	Key          string
	Namespace    string
	Kind         string
	Supported    bool
	ReasonFamily string
	ReasonCode   string
	SchemaHash   string
	Shape        string
	Hits         int
	Misses       int
	Evictions    int
}

type qFallbackStats struct {
	KernelUnsupported int
	KernelCompileErr  int
	SourceErr         int
	JoinErr           int
	Mutation          int
	QueryKernelHit    int
	QueryKernel       int
	ByReasonCode      map[qFallbackReasonCodeKey]int
	ByReason          map[qFallbackReasonKey]int
	ByAttribution     map[qFallbackAttributionKey]int
}

type qFallbackReasonCodeKey struct {
	Code       string
	ReasonCode string
}

type qFallbackReasonKey struct {
	Code   string
	Reason string
}

type qFallbackAttributionKey struct {
	Code         string
	ReasonCode   string
	ReasonFamily string
	Source       string
	SchemaHash   string
	Shape        string
}

type qQueryKernelSupportShapeKey struct {
	Supported    bool
	ReasonFamily string
	ReasonCode   string
	SchemaHash   string
	Shape        string
}

type qQueryKernelSupportShapeStat struct {
	Key   qQueryKernelSupportShapeKey
	Count int
}

type qSQLArgsResult struct {
	frameValue    Value
	source        string
	resolveSource bool
	envValue      Value
}

var (
	qSQLTemplateCacheMu sync.Mutex
	qSQLTemplateCache   = make(map[string]qSQLPlanTemplate)
	qSQLTemplateOrder   []string
	qSQLTemplateStats   qSQLPlanCacheStats

	qSQLAlignedPlanCacheMu     sync.Mutex
	qSQLAlignedPlanCache       = make(map[string]data.QueryPlan)
	qSQLAlignedPlanOrder       []string
	qSQLAlignedMutationCache   = make(map[string]*stdq.MutationPlan)
	qSQLAlignedMutationOrder   []string
	qSQLKernelCache            = make(map[string]*data.QueryKernel)
	qSQLKernelOrder            []string
	qSQLKernelUnsupported      = make(map[string]string)
	qSQLKernelUnsupportedShape = make(map[string]string)
	qSQLKernelUnsupportedOrder []string
	qSQLKernelStatsByKey       = make(map[string]*qSQLKernelCacheKeyStats)
	qSQLAlignedStats           qSQLPlanCacheStats

	qEvalCacheMu    sync.Mutex
	qEvalCache      = make(map[string]any)
	qEvalCacheOrder []string
	qEvalStats      qEvalCacheStats

	qQueryKernelSupportCacheMu    sync.Mutex
	qQueryKernelSupportCache      = make(map[string]qQueryKernelSupportCacheEntry)
	qQueryKernelSupportCacheOrder []string
	qQueryKernelSupportStats      qQueryKernelSupportCacheStats
	qQueryKernelSupportStatsByKey = make(map[string]*qQueryKernelSupportCacheKeyStat)

	qFallbackStatsMu  sync.Mutex
	qFallbackCounters qFallbackStats
)

// BuildQ creates the "q" column-query helper library. q.query keeps the
// ordinary Leia table-plan API, while q.eval and q`...` expose a small
// q/kdb+-style symbolic vector subset.
func BuildQ() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "q." + name, Fn: fn}))
	}
	set("query", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsSoA() {
			return nil, fmt.Errorf("q.query: argument 1 must be soa")
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("q.query: argument 2 must be a query plan table")
		}
		out, err := qRunQuery(args[0].SoA(), args[1].Table())
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(out)}, nil
	})
	set("explain_query", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsSoA() {
			return nil, fmt.Errorf("q.explain_query: argument 1 must be soa")
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("q.explain_query: argument 2 must be a query plan table")
		}
		out, err := qExplainQuery(args[0].SoA(), args[1].Table())
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(out)}, nil
	})
	set("eval", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("q.eval: argument 1 must be a q source string")
		}
		return qEvalSymbolic(args[0].Str())
	})
	set("session", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.session: expected no arguments")
		}
		return []Value{TableValue(qSessionValue())}, nil
	})
	set("workspace", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.workspace: expected no arguments")
		}
		return []Value{TableValue(qSessionValue())}, nil
	})
	set("key_by", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("q.key_by: expected frame and at least one key")
		}
		frame, err := qDataFrameFromValue(args[0], "")
		if err != nil {
			return nil, fmt.Errorf("q.key_by: %w", err)
		}
		keys, err := qSymbolsFromArgs("q.key_by", args[1:])
		if err != nil {
			return nil, err
		}
		keyed, err := data.KeyBy(frame, keys...)
		if err != nil {
			return nil, fmt.Errorf("q.key_by: %w", err)
		}
		return []Value{qKeyedFrameToValue(keyed)}, nil
	})
	set("lookup", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("q.lookup: expected keyed frame and at least one key value")
		}
		keyed, err := qKeyedFrameFromValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		values, err := qLookupKeyValues(keyed, args[1:])
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		out, err := keyed.LookupValueByKey(values...)
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		rows, err := qRowsFromDataFrame(out)
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("amend", func(args []Value) ([]Value, error) {
		keyed, delta, valueColumns, err := qKeyedMutationArgs("q.amend", args)
		if err != nil {
			return nil, err
		}
		out, err := data.AmendKeyed(keyed, delta, valueColumns...)
		if err != nil {
			return nil, fmt.Errorf("q.amend: %w", err)
		}
		return []Value{qKeyedFrameToValue(out)}, nil
	})
	set("upsert", func(args []Value) ([]Value, error) {
		keyed, delta, valueColumns, err := qKeyedMutationArgs("q.upsert", args)
		if err != nil {
			return nil, err
		}
		out, err := data.UpsertKeyed(keyed, delta, valueColumns...)
		if err != nil {
			return nil, fmt.Errorf("q.upsert: %w", err)
		}
		return []Value{qKeyedFrameToValue(out)}, nil
	})
	set("keys", func(args []Value) ([]Value, error) {
		return qKeys(args, false)
	})
	set("key", func(args []Value) ([]Value, error) {
		return qKeys(args, true)
	})
	set("value", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.value: argument 1 required")
		}
		if keyed, err := qKeyedFrameFromValue(args[0]); err == nil {
			frame, err := keyed.ValueFrame()
			if err != nil {
				return nil, fmt.Errorf("q.value: %w", err)
			}
			rows, err := qRowsFromDataFrame(frame)
			if err != nil {
				return nil, fmt.Errorf("q.value: %w", err)
			}
			return []Value{TableValue(rows)}, nil
		}
		if values, ok := qDictionaryValues(args[0]); ok {
			return []Value{values}, nil
		}
		return []Value{args[0]}, nil
	})
	set("cols", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.cols: argument 1 required")
		}
		frame, err := qFrameLikeValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.cols: %w", err)
		}
		return []Value{qDataSymbolListValue(frame.Schema().Names())}, nil
	})
	set("meta", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.meta: argument 1 required")
		}
		frame, err := qFrameLikeValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.meta: %w", err)
		}
		rows, err := qRowsFromDataFrame(qMetaFrame(frame))
		if err != nil {
			return nil, fmt.Errorf("q.meta: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.encode: argument 1 required")
		}
		v, err := qWireValueFromValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.encode: %w", err)
		}
		buf, err := stdq.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("q.encode: %w", err)
		}
		return []Value{StringValue(string(buf))}, nil
	})
	set("decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("q.decode: argument 1 must be encoded string")
		}
		v, err := stdq.Unmarshal([]byte(args[0].Str()))
		if err != nil {
			return nil, fmt.Errorf("q.decode: %w", err)
		}
		out, err := qEvalValueToValue(v)
		if err != nil {
			return nil, fmt.Errorf("q.decode: %w", err)
		}
		return []Value{out}, nil
	})
	qsql := func(name string, args []Value) ([]Value, error) {
		parsed, err := qSQLArgs(name, args)
		if err != nil {
			return nil, err
		}
		out, err := qRunSQL(name, parsed)
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	}
	setQSQL := func(name string) {
		nativeKind := NativeKindStdQSQL
		nativeData := StdQSQLIdentityPtr()
		if name == "select" {
			nativeKind = NativeKindStdQSelect
			nativeData = StdQSelectIdentityPtr()
		}
		t.RawSetString(name, FunctionValue(&GoFunction{
			Name:       "q." + name,
			NativeKind: nativeKind,
			NativeData: nativeData,
			Fn: func(args []Value) ([]Value, error) {
				return qsql("q."+name, args)
			},
			FastArg2: func(a, b Value) (Value, error) {
				return qSQLFastArg2("q."+name, a, b)
			},
		}))
	}
	setQSQL("sql")
	setQSQL("select")
	set("explain", func(args []Value) ([]Value, error) {
		parsed, err := qSQLArgs("q.explain", args)
		if err != nil {
			return nil, err
		}
		out, err := qExplainSQL(parsed)
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	})
	set("cache_stats", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.cache_stats: expected no arguments")
		}
		return []Value{TableValue(qCacheStatsTable())}, nil
	})
	set("fallback_stats", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.fallback_stats: expected no arguments")
		}
		return []Value{TableValue(qFallbackStatsTable())}, nil
	})
	set("cache_clear", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.cache_clear: expected no arguments")
		}
		qClearCaches()
		return []Value{TableValue(qCacheStatsTable())}, nil
	})
	set("set", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("q.set: expected path and frame")
		}
		path, err := qPathStringArg("q.set", args, 0, "path")
		if err != nil {
			return nil, err
		}
		frame, err := qDataFrameFromValue(args[1], "")
		if err != nil {
			return nil, fmt.Errorf("q.set: %w", err)
		}
		if err := data.SaveFrameDir(path, frame); err != nil {
			return nil, fmt.Errorf("q.set: %w", err)
		}
		return []Value{args[1]}, nil
	})
	set("save_splayed", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("q.save_splayed: expected frame and path")
		}
		frame, err := qDataFrameFromValue(args[0], "")
		if err != nil {
			return nil, fmt.Errorf("q.save_splayed: %w", err)
		}
		path, err := qPathStringArg("q.save_splayed", args, 1, "path")
		if err != nil {
			return nil, err
		}
		if err := data.SaveFrameDir(path, frame); err != nil {
			return nil, fmt.Errorf("q.save_splayed: %w", err)
		}
		return []Value{BoolValue(true)}, nil
	})
	set("load_splayed", func(args []Value) ([]Value, error) {
		path, err := qPathStringArg("q.load_splayed", args, 0, "path")
		if err != nil {
			return nil, err
		}
		frame, err := data.LoadFrameDir(path)
		if err != nil {
			return nil, fmt.Errorf("q.load_splayed: %w", err)
		}
		rows, err := qRowsFromDataFrame(frame)
		if err != nil {
			return nil, fmt.Errorf("q.load_splayed: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("save_partitioned", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("q.save_partitioned: expected frame, path, and partition columns")
		}
		frame, err := qDataFrameFromValue(args[0], "")
		if err != nil {
			return nil, fmt.Errorf("q.save_partitioned: %w", err)
		}
		path, err := qPathStringArg("q.save_partitioned", args, 1, "path")
		if err != nil {
			return nil, err
		}
		cols, err := dataPartitionColumnArgs("q.save_partitioned", args[2:])
		if err != nil {
			return nil, err
		}
		if err := data.SavePartitionedFrameDir(path, frame, cols...); err != nil {
			return nil, fmt.Errorf("q.save_partitioned: %w", err)
		}
		return []Value{BoolValue(true)}, nil
	})
	set("load_partitioned", func(args []Value) ([]Value, error) {
		path, err := qPathStringArg("q.load_partitioned", args, 0, "path")
		if err != nil {
			return nil, err
		}
		filters, err := dataPartitionFilters("q.load_partitioned", args[1:])
		if err != nil {
			return nil, err
		}
		frame, err := data.LoadPartitionedFrameDir(path, filters)
		if err != nil {
			return nil, fmt.Errorf("q.load_partitioned: %w", err)
		}
		rows, err := qRowsFromDataFrame(frame)
		if err != nil {
			return nil, fmt.Errorf("q.load_partitioned: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("count", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.count: argument 1 required")
		}
		return []Value{IntValue(int64(qCount(args[0])))}, nil
	})
	return t
}

func qSQLArgs(name string, args []Value) (qSQLArgsResult, error) {
	if len(args) < 2 {
		return qSQLArgsResult{}, fmt.Errorf("%s: expected (frame, qSQL source) or (qSQL source, frames)", name)
	}
	if parsed, ok, err := qSQLArgs2(name, args[0], args[1]); ok || err != nil {
		if err != nil {
			return qSQLArgsResult{}, err
		}
		env := NilValue()
		if len(args) > 2 && args[1].IsString() {
			env = args[2]
			parsed.envValue = env
		}
		return parsed, nil
	}
	return qSQLArgsResult{}, fmt.Errorf("%s: expected one qSQL source string argument", name)
}

func qSQLArgs2(name string, a, b Value) (qSQLArgsResult, bool, error) {
	if a.IsString() {
		return qSQLArgsResult{frameValue: b, source: a.Str(), resolveSource: true, envValue: b}, true, nil
	}
	if b.IsString() {
		return qSQLArgsResult{frameValue: a, source: b.Str(), envValue: NilValue()}, true, nil
	}
	return qSQLArgsResult{}, false, nil
}

func qSQLFastArg2(name string, a, b Value) (Value, error) {
	parsed, ok, err := qSQLArgs2(name, a, b)
	if err != nil {
		return NilValue(), err
	}
	if !ok {
		return NilValue(), fmt.Errorf("%s: expected one qSQL source string argument", name)
	}
	return qRunSQL(name, parsed)
}

func dialectQ(body Value, opts *Table) ([]Value, error) {
	mode := dialectMode(opts)
	if mode != "" && mode != "eval" && mode != "parse" {
		return dialectUnknownMode("q", mode)
	}
	return qEvalSymbolic(body.String())
}

func qEvalSymbolic(src string) ([]Value, error) {
	cacheable := stdq.EvalSourceCacheable(src)
	var out any
	if cacheable {
		qEvalCacheMu.Lock()
		cached, ok := qEvalCache[src]
		if ok {
			qEvalStats.Hits++
		} else {
			qEvalStats.Misses++
		}
		qEvalCacheMu.Unlock()
		if ok {
			v, err := qEvalValueToValue(cached)
			if err != nil {
				return nil, fmt.Errorf("q dialect: %w", err)
			}
			return []Value{v}, nil
		}
	}
	out, err := stdq.Eval(src)
	if err != nil {
		return nil, fmt.Errorf("q dialect: %w", err)
	}
	if cacheable && stdq.EvalValueCacheable(out) {
		qEvalCacheMu.Lock()
		qEvalCacheStoreLocked(src, out)
		qEvalCacheMu.Unlock()
	}
	v, err := qEvalValueToValue(out)
	if err != nil {
		return nil, fmt.Errorf("q dialect: %w", err)
	}
	return []Value{v}, nil
}

func qSessionValue() *Table {
	state := stdq.NewEvalState(nil)
	var mu sync.Mutex
	t := NewTable()
	t.RawSetString("kind", StringValue("q_session"))
	t.RawSetString("eval", FunctionValue(&GoFunction{Name: "q.session.eval", Fn: func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("q.session.eval: argument 1 must be a q source string")
		}
		mu.Lock()
		out, err := state.Eval(args[0].Str())
		mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("q session: %w", err)
		}
		v, err := qEvalValueToValue(out)
		if err != nil {
			return nil, fmt.Errorf("q session: %w", err)
		}
		return []Value{v}, nil
	}}))
	return t
}

func qEvalCacheStoreLocked(src string, value any) {
	if _, ok := qEvalCache[src]; !ok {
		qEvalCacheOrder = append(qEvalCacheOrder, src)
	}
	qEvalCache[src] = value
	for len(qEvalCacheOrder) > qEvalCacheLimit {
		evict := qEvalCacheOrder[0]
		qEvalCacheOrder = qEvalCacheOrder[1:]
		delete(qEvalCache, evict)
		qEvalStats.Evictions++
	}
}

func qQueryKernelSupportCacheProbe(key string) (qQueryKernelSupportCacheEntry, bool) {
	qQueryKernelSupportCacheMu.Lock()
	entry, ok := qQueryKernelSupportCache[key]
	if ok {
		qQueryKernelSupportStats.Hits++
		qQueryKernelSupportStatsForKeyLocked(key).Hits++
	} else {
		qQueryKernelSupportStats.Misses++
		qQueryKernelSupportStatsForKeyLocked(key).Misses++
	}
	qQueryKernelSupportCacheMu.Unlock()
	return entry, ok
}

func qQueryKernelSupportCachePeek(key string) (qQueryKernelSupportCacheEntry, bool) {
	qQueryKernelSupportCacheMu.Lock()
	entry, ok := qQueryKernelSupportCache[key]
	qQueryKernelSupportCacheMu.Unlock()
	return entry, ok
}

func qQueryKernelSupportCacheStore(key string, entry qQueryKernelSupportCacheEntry) {
	if key == "" {
		return
	}
	if schemaHash := qQueryKernelSchemaHashFromCacheKey(key); schemaHash != "" {
		entry.SchemaHash = schemaHash
	}
	if entry.Shape == "" {
		entry.Shape = qQueryKernelShapeFromCacheKey(key)
	}
	qQueryKernelSupportCacheMu.Lock()
	if _, ok := qQueryKernelSupportCache[key]; !ok {
		qQueryKernelSupportCacheOrder = append(qQueryKernelSupportCacheOrder, key)
	}
	qQueryKernelSupportCache[key] = entry
	qQueryKernelSupportStatsForKeyLocked(key).setEntry(entry)
	for len(qQueryKernelSupportCacheOrder) > qQueryKernelSupportCacheLimit {
		evict := qQueryKernelSupportCacheOrder[0]
		qQueryKernelSupportCacheOrder = qQueryKernelSupportCacheOrder[1:]
		delete(qQueryKernelSupportCache, evict)
		delete(qQueryKernelSupportStatsByKey, evict)
		qQueryKernelSupportStats.Evictions++
	}
	qQueryKernelSupportCacheMu.Unlock()
}

func qQueryKernelSupportCacheKey(s *SoA, spec *Table, selects []qSelect) (string, bool) {
	if s == nil || spec == nil || len(selects) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(selects)*3+8)
	for _, sel := range selects {
		sig, ok := qQueryKernelExprSignature(sel.Expr, 0)
		if !ok {
			return "", false
		}
		parts = append(parts, "select", sel.Name, sig)
	}
	order, err := qOrderSpecs(spec.RawGetString("order_by"))
	if err != nil {
		return "", false
	}
	for _, ord := range order {
		dir := "asc"
		if ord.Desc {
			dir = "desc"
		}
		parts = append(parts, "order", ord.Column, dir)
	}
	limit, err := qLimit(spec.RawGetString("limit"))
	if err != nil {
		return "", false
	}
	parts = append(parts, "limit", strconv.Itoa(limit))
	return qQuerySchemaStableCacheKey("q.query", "query_kernel", qQueryNativeSoASchemaHash(s), parts...), true
}

func qQuerySchemaStableCacheKey(namespace, kind, schemaHash string, extra ...string) string {
	var b strings.Builder
	qWriteSchemaStableCacheKeyPart(&b, namespace)
	qWriteSchemaStableCacheKeyPart(&b, kind)
	qWriteSchemaStableCacheKeyPart(&b, schemaHash)
	for _, part := range extra {
		qWriteSchemaStableCacheKeyPart(&b, part)
	}
	return b.String()
}

func qWriteSchemaStableCacheKeyPart(b *strings.Builder, part string) {
	b.WriteString(strconv.Itoa(len(part)))
	b.WriteByte(':')
	b.WriteString(part)
	b.WriteByte(';')
}

func qQueryKernelCacheKeyParts(key string) ([]string, string) {
	if parsed, ok := data.ParseSchemaStableCacheKey(key); ok && parsed.Namespace == "q.query" && parsed.Kind == "query_kernel" {
		return parsed.Extra, parsed.SchemaHash
	}
	parts := strings.Split(key, "|")
	schemaHash := ""
	for _, part := range parts {
		if strings.HasPrefix(part, "source=") {
			schemaHash = strings.TrimPrefix(part, "source=")
			break
		}
	}
	return parts, schemaHash
}

func qQueryKernelShapeFromCacheKey(key string) string {
	if key == "" {
		return ""
	}
	selectShapes := make([]string, 0, 4)
	orderCount := 0
	limit := "none"
	parts, _ := qQueryKernelCacheKeyParts(key)
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		switch {
		case part == "select" && i+2 < len(parts):
			selectShapes = append(selectShapes, qQueryKernelExprShape(parts[i+2]))
			i += 2
		case part == "order" && i+2 < len(parts):
			orderCount++
			i += 2
		case part == "limit" && i+1 < len(parts):
			limit = parts[i+1]
			if limit == "-1" {
				limit = "none"
			}
			i++
		case strings.HasPrefix(part, "select:"):
			_, sig, ok := strings.Cut(part, "=")
			if !ok {
				selectShapes = append(selectShapes, "unknown")
				continue
			}
			selectShapes = append(selectShapes, qQueryKernelExprShape(sig))
		case strings.HasPrefix(part, "order:"):
			orderCount++
		case strings.HasPrefix(part, "limit="):
			limit = strings.TrimPrefix(part, "limit=")
			if limit == "-1" {
				limit = "none"
			}
		}
	}
	if len(selectShapes) == 0 {
		selectShapes = append(selectShapes, "none")
	}
	return fmt.Sprintf("select=%s|order=%d|limit=%s", strings.Join(selectShapes, ","), orderCount, limit)
}

func qQueryKernelSchemaHashFromCacheKey(key string) string {
	_, schemaHash := qQueryKernelCacheKeyParts(key)
	return schemaHash
}

func qQueryKernelExprShape(sig string) string {
	if sig == "" {
		return "unknown"
	}
	switch {
	case strings.HasPrefix(sig, "s:"):
		return "column"
	case strings.HasPrefix(sig, "i:"), strings.HasPrefix(sig, "f:"), strings.HasPrefix(sig, "b:"), strings.HasPrefix(sig, "nil"):
		return "literal"
	case strings.HasPrefix(sig, "op:"):
		op := ""
		rest := strings.TrimPrefix(sig, "op:")
		if decoded, remaining, ok := qQueryKernelDecodeQuotedPrefix(rest); ok && strings.HasPrefix(remaining, "(") {
			op = decoded
		}
		switch op {
		case "=", "!=", "<", "<=", ">", ">=":
			return "compare"
		case "+", "-", "*", "/":
			return "binary"
		default:
			return "op"
		}
	default:
		return "unknown"
	}
}

func qQueryKernelDecodeQuotedPrefix(src string) (value, rest string, ok bool) {
	if src == "" || src[0] != '"' {
		return "", src, false
	}
	for i := 1; i <= len(src); i++ {
		if i == len(src) || src[i-1] != '"' {
			continue
		}
		value, err := strconv.Unquote(src[:i])
		if err != nil {
			continue
		}
		return value, src[i:], true
	}
	return "", src, false
}

func qQueryKernelExprSignature(expr Value, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	switch {
	case expr.IsNil():
		return "nil", true
	case expr.IsString():
		return "s:" + strconv.Quote(expr.Str()), true
	case expr.IsInt():
		return "i:" + strconv.FormatInt(expr.Int(), 10), true
	case expr.IsFloat():
		return "f:" + strconv.FormatFloat(expr.Float(), 'g', -1, 64), true
	case expr.IsBool():
		return "b:" + strconv.FormatBool(expr.Bool()), true
	case expr.IsTable():
		tbl := expr.Table()
		op := tbl.RawGetString("op")
		if op.IsNil() {
			op = tbl.RawGetInt(1)
		}
		if !op.IsString() {
			return "", false
		}
		left := tbl.RawGetString("left")
		if left.IsNil() {
			left = tbl.RawGetInt(2)
		}
		right := tbl.RawGetString("right")
		if right.IsNil() {
			right = tbl.RawGetInt(3)
		}
		leftSig, ok := qQueryKernelExprSignature(left, depth+1)
		if !ok {
			return "", false
		}
		rightSig, ok := qQueryKernelExprSignature(right, depth+1)
		if !ok {
			return "", false
		}
		return "op:" + strconv.Quote(op.Str()) + "(" + leftSig + "," + rightSig + ")", true
	default:
		return "", false
	}
}

func qEvalValueToValue(v any) (Value, error) {
	if data.IsNull(v) {
		if kind, ok := data.NullKind(v); ok {
			return dataTypedNullValue(kind), nil
		}
		return dataNullValue(), nil
	}
	switch x := v.(type) {
	case nil:
		return NilValue(), nil
	case bool:
		return BoolValue(x), nil
	case int:
		return IntValue(int64(x)), nil
	case int8:
		return IntValue(int64(x)), nil
	case int16:
		return IntValue(int64(x)), nil
	case int32:
		return IntValue(int64(x)), nil
	case int64:
		return IntValue(x), nil
	case uint8:
		return IntValue(int64(x)), nil
	case uint16:
		return IntValue(int64(x)), nil
	case uint32:
		return IntValue(int64(x)), nil
	case uint64:
		return IntValue(int64(x)), nil
	case float32:
		return FloatValue(float64(x)), nil
	case float64:
		return FloatValue(x), nil
	case string:
		return StringValue(x), nil
	case data.Symbol:
		return StringValue(string(x)), nil
	case data.Month, data.Date, data.DateTime, data.Timespan, data.Minute, data.Second, data.Time, data.Timestamp:
		if s, ok := stdq.FormatTemporal(x); ok {
			return StringValue(s), nil
		}
		return StringValue(fmt.Sprint(x)), nil
	case data.Array:
		return qEvalArrayValue(x)
	case stdq.Dict:
		return qEvalDictValue(x)
	case data.Frame:
		rows, err := qRowsFromDataFrame(x)
		if err != nil {
			return NilValue(), err
		}
		return TableValue(rows), nil
	case data.KeyedFrame:
		return qKeyedFrameToValue(x), nil
	default:
		return StringValue(fmt.Sprint(x)), nil
	}
}

func qEvalArrayValue(array data.Array) (Value, error) {
	if array.Len() == 0 {
		if kind := array.Kind(); kind != "" && kind != data.KindAny && kind != data.KindNull {
			return qEvalGenericArrayTable(array)
		}
	}
	switch array.Kind() {
	case data.KindI64:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		xs := make([]int64, array.Len())
		for i := range xs {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			n, ok := v.(int64)
			if !ok {
				return NilValue(), fmt.Errorf("i64 array row %d has %T", i, v)
			}
			xs[i] = n
		}
		return DenseArrayValue(NewDenseArrayI64(xs)), nil
	case data.KindF64:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		xs := make([]float64, array.Len())
		for i := range xs {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			switch n := v.(type) {
			case float64:
				xs[i] = n
			case int64:
				xs[i] = float64(n)
			default:
				return NilValue(), fmt.Errorf("f64 array row %d has %T", i, v)
			}
		}
		return DenseArrayValue(NewDenseArrayF64(xs)), nil
	case data.KindBool:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		out, err := NewDenseArrayOfLen(DenseArrayBool, array.Len())
		if err != nil {
			return NilValue(), err
		}
		for i := 0; i < array.Len(); i++ {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			b, ok := v.(bool)
			if !ok {
				return NilValue(), fmt.Errorf("bool array row %d has %T", i, v)
			}
			if err := out.Set(i, BoolValue(b)); err != nil {
				return NilValue(), err
			}
		}
		return DenseArrayValue(out), nil
	case data.KindSymbol:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		keys := make([]string, array.Len())
		for i := range keys {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			switch s := v.(type) {
			case data.Symbol:
				keys[i] = string(s)
			case string:
				keys[i] = s
			default:
				return NilValue(), fmt.Errorf("symbol array row %d has %T", i, v)
			}
		}
		return qSymbolListValue(keys), nil
	case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan, data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		out := NewAppendArrayTable(array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			if data.IsNull(item) {
				out.RawSetInt(int64(i+1), qAnyToColumnValue(data.NullForKind(array.Kind())))
				continue
			}
			text, ok := stdq.FormatTemporal(item)
			if !ok {
				return NilValue(), fmt.Errorf("%s array row %d has %T", array.Kind(), i, item)
			}
			out.RawSetInt(int64(i+1), StringValue(text))
		}
		return TableValue(out), nil
	default:
		return qEvalGenericArrayTable(array)
	}
}

func dataArrayHasNull(array data.Array) bool {
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok || data.IsNull(item) {
			return true
		}
	}
	return false
}

func qEvalGenericArrayTable(array data.Array) (Value, error) {
	return dataArrayFacadeValue(array, qAnyToColumnValue), nil
}

func qEvalDictValue(dict stdq.Dict) (Value, error) {
	if len(dict.Keys) != len(dict.Values) {
		return NilValue(), fmt.Errorf("dict key/value length mismatch")
	}
	out := NewTable()
	order := NewAppendArrayTable(len(dict.Keys))
	for i, key := range dict.Keys {
		v, err := qEvalValueToValue(dict.Values[i])
		if err != nil {
			return NilValue(), err
		}
		keyString := qEvalDictKeyString(key)
		out.RawSetString(keyString, v)
		order.RawSetInt(int64(i+1), StringValue(keyString))
	}
	out.RawSetString(qDictKeysMarker, TableValue(order))
	return TableValue(out), nil
}

func qEvalDictKeyString(key any) string {
	switch x := key.(type) {
	case data.Symbol:
		return string(x)
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func qSymbolListValue(keys []string) Value {
	out := NewAppendArrayTable(len(keys))
	for i, key := range keys {
		out.RawSetInt(int64(i+1), StringValue(key))
	}
	out.RawSetString(qSymbolVectorMarker, BoolValue(true))
	return TableValue(out)
}

func qDataSymbolListValue(keys []data.Symbol) Value {
	out := NewAppendArrayTable(len(keys))
	for i, key := range keys {
		out.RawSetInt(int64(i+1), StringValue(string(key)))
	}
	out.RawSetString(qSymbolVectorMarker, BoolValue(true))
	return TableValue(out)
}

func qIsSymbolVector(v Value) bool {
	return v.IsTable() && v.Table().RawGetString(qSymbolVectorMarker).Truthy()
}

func qSymbolVectorSymbols(v Value) ([]data.Symbol, bool) {
	if !v.IsTable() {
		return nil, false
	}
	tbl := v.Table()
	items := make(map[int64]string)
	tbl.ForEachPlainRaw(func(key, val Value) bool {
		if key.IsInt() && key.Int() >= 1 && val.IsString() {
			items[key.Int()] = val.Str()
		}
		return true
	})
	if len(items) == 0 {
		if qIsSymbolVector(v) {
			return []data.Symbol{}, true
		}
		return nil, false
	}
	keys := make([]int64, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]data.Symbol, 0, len(keys))
	for _, key := range keys {
		out = append(out, data.Symbol(items[key]))
	}
	return out, true
}

func qDictionaryValues(v Value) (Value, bool) {
	if !v.IsTable() {
		return NilValue(), false
	}
	tbl := v.Table()
	if qLooksLikeFrame(tbl) || qIsKeyedFrameTable(tbl) {
		return NilValue(), false
	}
	if keys, ok := qDictionaryKeyOrder(tbl); ok {
		out := NewAppendArrayTable(len(keys))
		for i, key := range keys {
			out.RawSetInt(int64(i+1), tbl.RawGetString(string(key)))
		}
		return TableValue(out), true
	}
	keys := make([]string, 0)
	tbl.ForEachPlainRaw(func(key, _ Value) bool {
		if key.IsString() {
			keys = append(keys, key.Str())
		}
		return true
	})
	if len(keys) == 0 {
		return NilValue(), false
	}
	sort.Strings(keys)
	out := NewAppendArrayTable(len(keys))
	for i, key := range keys {
		out.RawSetInt(int64(i+1), tbl.RawGetString(key))
	}
	return TableValue(out), true
}

func qDictionaryKeyOrder(tbl *Table) ([]data.Symbol, bool) {
	order := tbl.RawGetString(qDictKeysMarker)
	if !order.IsTable() {
		return nil, false
	}
	keys, err := qSymbolsFromArgs("q.keys", []Value{order})
	if err != nil {
		return nil, false
	}
	return keys, true
}

func qVectorLen(v Value) int {
	if v.IsDenseArray() {
		return v.DenseArray().Len()
	}
	if v.IsTable() {
		return v.Table().Length()
	}
	return 0
}

func qCount(v Value) int {
	switch {
	case v.IsNil():
		return 0
	case v.IsDenseArray(), v.IsTable():
		return qVectorLen(v)
	case v.IsString():
		return len(v.Str())
	default:
		return 1
	}
}

func qRunQuery(s *SoA, spec *Table) (*Table, error) {
	mask, err := qQueryMask(s, spec.RawGetString("where"))
	if err != nil {
		return nil, err
	}
	selects, err := qQuerySelects(spec.RawGetString("select"))
	if err != nil {
		return nil, err
	}
	if len(selects) == 0 {
		for _, name := range s.ColumnNames() {
			selects = append(selects, qSelect{Name: name, Expr: StringValue(name)})
		}
	}
	by, err := qStringList("q.query by", spec.RawGetString("by"))
	if err != nil {
		return nil, err
	}
	aggs, err := qAggregates(spec.RawGetString("aggregate"))
	if err != nil {
		return nil, err
	}
	var rows *Table
	var nativeRows *SoA
	nativeReasonCode := ""
	nativeReason := ""
	nativeCacheKey := ""
	nativeCacheable := false
	nativeCacheEntry := qQueryKernelSupportCacheEntry{}
	nativeCacheHit := false
	if len(aggs) == 0 {
		nativeCacheKey, nativeCacheable = qQueryKernelSupportCacheKey(s, spec, selects)
		if nativeCacheable {
			nativeCacheEntry, nativeCacheHit = qQueryKernelSupportCacheProbe(nativeCacheKey)
		}
		if nativeCacheHit && !nativeCacheEntry.Supported {
			nativeReasonCode = nativeCacheEntry.ReasonCode
			nativeReason = nativeCacheEntry.Reason
		} else {
			nativeRows, nativeReasonCode, nativeReason = qSimpleSelectRowsNativeSoA(s, mask, selects)
		}
		rows, err = qRows(s, mask, selects)
	} else {
		rows, err = qGroupedRows(s, mask, by, selects, aggs)
	}
	if err != nil {
		return nil, err
	}
	rows, err = qApplyOrderAndLimit(rows, spec)
	if err != nil {
		return nil, err
	}
	nativeRows, nativeResultOK, nativeResultReasonCode, nativeResultReason := qQueryNativeRowsForResult(spec, nativeRows)
	if nativeResultOK {
		qAttachRowsNativeSoAPayload(rows, nativeRows)
		if len(aggs) == 0 {
			if nativeCacheable {
				qQueryKernelSupportCacheStore(nativeCacheKey, qQueryKernelSupportCacheEntry{
					Supported:  true,
					ReasonCode: qKernelReasonSupported,
					Reason:     qKernelReasonSupported,
					SchemaHash: qQueryNativeSoASchemaHash(nativeRows),
				})
			}
			qRecordQueryKernelHit()
		}
	} else {
		if len(aggs) == 0 {
			if nativeReason == "" {
				nativeReasonCode = nativeResultReasonCode
				nativeReason = nativeResultReason
			}
			if nativeReason == "" {
				nativeReasonCode = qQueryKernelReasonOrder
				nativeReason = "query native kernel could not preserve order or limit"
			}
			if nativeCacheable {
				qQueryKernelSupportCacheStore(nativeCacheKey, qQueryKernelSupportCacheEntry{
					Supported:  false,
					ReasonCode: nativeReasonCode,
					Reason:     nativeReason,
				})
			}
			qRecordFallbackReasonAttribution(
				qFallbackQueryKernel,
				nativeReasonCode,
				nativeReason,
				"q.query",
				qQueryKernelSchemaHashFromCacheKey(nativeCacheKey),
				qQueryKernelShapeFromCacheKey(nativeCacheKey),
			)
		}
		qAttachRowsNativeFramePayload(rows)
	}
	return rows, nil
}

func qExplainQuery(s *SoA, spec *Table) (*Table, error) {
	sourceSchemaHash := qQueryNativeSoASchemaHash(s)
	mask, err := qQueryMask(s, spec.RawGetString("where"))
	if err != nil {
		return nil, err
	}
	selects, err := qQuerySelects(spec.RawGetString("select"))
	if err != nil {
		return nil, err
	}
	if len(selects) == 0 {
		for _, name := range s.ColumnNames() {
			selects = append(selects, qSelect{Name: name, Expr: StringValue(name)})
		}
	}
	by, err := qStringList("q.query by", spec.RawGetString("by"))
	if err != nil {
		return nil, err
	}
	aggs, err := qAggregates(spec.RawGetString("aggregate"))
	if err != nil {
		return nil, err
	}
	out := NewTable()
	out.RawSetString("op", StringValue("query"))
	out.RawSetString("source_rows", IntValue(int64(s.Len())))
	out.RawSetString("where_mask_rows", IntValue(int64(mask.Len())))
	out.RawSetString("select_count", IntValue(int64(len(selects))))
	out.RawSetString("by_count", IntValue(int64(len(by))))
	out.RawSetString("aggregate_count", IntValue(int64(len(aggs))))
	out.RawSetString("kernel_execution_stats_domain", StringValue(qStatsDomainJITExecution))
	out.RawSetString("kernel_execution_stats_source", StringValue(qStatsSourceMethodJIT))
	out.RawSetString("kernel_execution_stats_cache_backed", BoolValue(false))
	out.RawSetString("kernel_lowering_stats_domain", StringValue(qStatsDomainJITLowering))
	out.RawSetString("kernel_lowering_stats_source", StringValue(qStatsSourceMethodJIT))
	out.RawSetString("kernel_lowering_stats_cache_backed", BoolValue(false))
	kernelCached := false
	kernelShape := ""
	if len(aggs) == 0 {
		if key, ok := qQueryKernelSupportCacheKey(s, spec, selects); ok {
			entry, cached := qQueryKernelSupportCachePeek(key)
			kernelCached = cached
			if entry.Shape != "" {
				kernelShape = entry.Shape
			} else {
				kernelShape = qQueryKernelShapeFromCacheKey(key)
			}
		}
	}
	out.RawSetString("kernel_cached", BoolValue(kernelCached))
	out.RawSetString("kernel_shape", StringValue(kernelShape))
	qExplainAttachRuntimeKernelExecutionSummary(out, kernelShape)
	qExplainAttachRuntimeKernelLoweringSummary(out, kernelShape)
	if len(aggs) != 0 {
		reason := "query native kernel supports non-aggregate selects only"
		out.RawSetString("kernel_supported", BoolValue(false))
		out.RawSetString("kernel_reason_code", StringValue(qQueryKernelReasonUnsupported))
		out.RawSetString("kernel_reason", StringValue(reason))
		qExplainAttachFallbackStats(out, qFallbackQueryKernel, qQueryKernelReasonUnsupported, reason)
		out.RawSetString("source_schema_hash", StringValue(sourceSchemaHash))
		out.RawSetString("kernel_schema_hash", StringValue(""))
		out.RawSetString("kernel_schema", TableValue(NewAppendArrayTable(0)))
		out.RawSetString("kernel_rows", IntValue(0))
		out.RawSetString("kernel_columns", IntValue(0))
		return out, nil
	}
	nativeRows, reasonCode, reason := qSimpleSelectRowsNativeSoA(s, mask, selects)
	nativeRows, ok, resultReasonCode, resultReason := qQueryNativeRowsForResult(spec, nativeRows)
	if !ok {
		if reason == "" {
			reasonCode = resultReasonCode
			reason = resultReason
		}
		if reason == "" {
			reasonCode = qQueryKernelReasonOrder
			reason = "query native kernel could not preserve order or limit"
		}
		out.RawSetString("kernel_supported", BoolValue(false))
		out.RawSetString("kernel_reason_code", StringValue(reasonCode))
		out.RawSetString("kernel_reason", StringValue(reason))
		qExplainAttachFallbackStats(out, qFallbackQueryKernel, reasonCode, reason)
		out.RawSetString("source_schema_hash", StringValue(sourceSchemaHash))
		out.RawSetString("kernel_schema_hash", StringValue(""))
		out.RawSetString("kernel_schema", TableValue(NewAppendArrayTable(0)))
		out.RawSetString("kernel_rows", IntValue(0))
		out.RawSetString("kernel_columns", IntValue(0))
		return out, nil
	}
	kernelSchemaHash := qQueryNativeSoASchemaHash(nativeRows)
	out.RawSetString("kernel_supported", BoolValue(true))
	out.RawSetString("kernel_reason_code", StringValue(qKernelReasonSupported))
	out.RawSetString("kernel_reason", StringValue(qKernelReasonSupported))
	qExplainAttachFallbackStats(out, "", qKernelReasonSupported, qKernelReasonSupported)
	out.RawSetString("source_schema_hash", StringValue(sourceSchemaHash))
	out.RawSetString("kernel_schema_hash", StringValue(kernelSchemaHash))
	out.RawSetString("kernel_schema", qExplainSoASchemaValue(nativeRows))
	out.RawSetString("kernel_rows", IntValue(int64(nativeRows.Len())))
	out.RawSetString("kernel_columns", IntValue(int64(len(nativeRows.ColumnNames()))))
	return out, nil
}

func qRunSQL(name string, args qSQLArgsResult) (Value, error) {
	tmpl, err := qSQLCachedPlanTemplate(name, args.source)
	if err != nil {
		return NilValue(), err
	}
	sourceName := ""
	if args.resolveSource {
		sourceName = tmpl.source
	}
	var frame data.Frame
	var source qSQLSourceCarrier
	hasSourceCarrier := false
	if tmpl.literalFrame != nil {
		frame = *tmpl.literalFrame
	} else if tmpl.sourcePath != "" {
		frame, err = data.LoadPartitionedFrameDir(tmpl.sourcePath, nil)
		if err != nil {
			qRecordFallbackReason(qFallbackSourceErr, qKernelReasonSourceUnavailable, err.Error())
			return NilValue(), fmt.Errorf("%s: load q path source %q: %w", name, tmpl.sourcePath, err)
		}
	} else {
		source, err = qSQLSourceCarrierFromValue(args.frameValue, sourceName)
		if err != nil {
			qRecordFallbackReason(qFallbackSourceErr, qKernelReasonSourceUnavailable, err.Error())
			return NilValue(), fmt.Errorf("%s: %w", name, err)
		}
		frame = source.frame
		hasSourceCarrier = true
	}
	bindings := qSQLScalarBindingsFromValue(args.envValue)
	if tmpl.mutation != nil {
		qRecordFallbackReason(qFallbackMutationPlan, qKernelReasonMutationPlan, "mutation plan cache requires QueryPlan fallback")
		var keyedSource data.KeyedFrame
		hasKeyedSource := false
		if hasSourceCarrier {
			keyedSource = source.keyed
			hasKeyedSource = source.hasKeyed
		}
		mutation := qSQLMutationForFrame(args.source, tmpl.mutation, frame)
		qBindMutationOuterScalars(mutation, frame.Schema(), bindings)
		if hasKeyedSource && (mutation.Kind == stdq.InsertQuery || mutation.Kind == stdq.UpsertQuery) {
			keyedOut, err := qRunSQLKeyedMutation(keyedSource, mutation)
			if err != nil {
				return NilValue(), fmt.Errorf("%s: exec: keyed mutation: %w", name, err)
			}
			return qKeyedFrameToValue(keyedOut), nil
		}
		out, err := qRunSQLMutation(frame, mutation)
		if err != nil {
			return NilValue(), fmt.Errorf("%s: exec: %w", name, err)
		}
		if len(tmpl.literalKeys) > 0 {
			keyedOut, err := data.KeyBy(out, tmpl.literalKeys...)
			if err != nil {
				return NilValue(), fmt.Errorf("%s: exec: keyed literal mutation: %w", name, err)
			}
			return qKeyedFrameToValue(keyedOut), nil
		}
		if hasKeyedSource {
			keyedOut, err := data.KeyBy(out, keyedSource.Keys()...)
			if err != nil {
				return NilValue(), fmt.Errorf("%s: exec: keyed mutation: %w", name, err)
			}
			return qKeyedFrameToValue(keyedOut), nil
		}
		return qDataFrameValue(out)
	}
	if len(qSQLTemplateJoins(tmpl)) > 0 {
		if !args.resolveSource {
			err := fmt.Errorf("join queries require a source table map")
			qRecordFallbackReason(qFallbackJoinErr, qKernelReasonJoinUnavailable, err.Error())
			return NilValue(), fmt.Errorf("%s: %w", name, err)
		}
		for _, join := range qSQLTemplateJoins(tmpl) {
			var err error
			frame, err = qApplySQLJoin(args.frameValue, frame, join)
			if err != nil {
				qRecordFallbackReason(qFallbackJoinErr, qKernelReasonJoinUnavailable, err.Error())
				return NilValue(), fmt.Errorf("%s: join: %w", name, err)
			}
		}
	}
	plan := qPrepareSQLPlanForFrame(args.source, tmpl.plan, frame, bindings, true)
	out, err := qRunSQLPlan(args.source, plan, frame)
	if err != nil {
		return NilValue(), fmt.Errorf("%s: exec: %w", name, err)
	}
	out, err = qDropHiddenSQLColumns(out, tmpl.hiddenCols)
	if err != nil {
		return NilValue(), fmt.Errorf("%s: exec: %w", name, err)
	}
	if tmpl.op == stdq.ExecQuery {
		if tmpl.execDict != nil {
			return qExecDictResultValue(out, tmpl.execDict)
		}
		return qExecResultValue(out)
	}
	return qDataFrameValue(out)
}

func qRunSQLPlan(src string, plan data.QueryPlan, frame data.Frame) (data.Frame, error) {
	kernel, ok, reason, err := qSQLKernelForFrame(src, plan, frame)
	if err != nil {
		qRecordFallbackReason(qFallbackKernelCompileErr, qKernelReasonCompileError, err.Error())
		return data.Frame{}, err
	}
	if !ok {
		reasonCode := stdq.KernelFallbackReasonCode(reason)
		info := qSQLKernelCacheKeyInfo(data.QueryKernelCacheKey(src, frame, plan))
		qRecordFallbackReasonAttribution(
			qFallbackKernelUnsupported,
			reasonCode,
			reason,
			info.Namespace,
			info.SchemaHash,
			data.QueryKernelPlanShape(plan),
		)
		kernel = nil
	}
	return data.ExecQueryKernelOrPlan(kernel, plan, frame)
}

func qSQLTemplateJoins(tmpl qSQLPlanTemplate) []*stdq.JoinPlan {
	if len(tmpl.joins) > 0 {
		return tmpl.joins
	}
	if tmpl.join != nil {
		return []*stdq.JoinPlan{tmpl.join}
	}
	return nil
}

func qApplySQLJoin(sources Value, left data.Frame, join *stdq.JoinPlan) (data.Frame, error) {
	if join == nil {
		return left, nil
	}
	right, err := qSQLSourceCarrierFromValue(sources, join.Right)
	if err != nil {
		return data.Frame{}, err
	}
	switch join.Kind {
	case "inner":
		if right.hasKeyed {
			return data.InnerJoinKeyedOn(left, right.keyed, join.Keys...)
		}
		return data.InnerJoinOn(left, right.frame, join.Keys...)
	case "left":
		if right.hasKeyed {
			return data.LeftJoinKeyedOn(left, right.keyed, join.Keys...)
		}
		return data.LeftJoinOn(left, right.frame, join.Keys...)
	case "union":
		return data.UnionJoinOn(left, right.frame, join.Keys...)
	case "plus":
		return data.PlusJoinOn(left, right.frame, join.Keys...)
	case "asof", "asof0", "asof_fill", "asof_fill0":
		if len(join.Keys) < 1 {
			return data.Frame{}, fmt.Errorf("q asof join requires a time key")
		}
		timeKey := join.Keys[len(join.Keys)-1]
		partitionKeys := join.Keys[:len(join.Keys)-1]
		return data.AsofJoinOnWithOptions(left, right.frame, data.AsofJoinOptions{
			TimeKey:           timeKey,
			PartitionKeys:     partitionKeys,
			PreserveRightTime: join.Kind == "asof0" || join.Kind == "asof_fill0",
		})
	case "window", "window1":
		if len(join.Keys) < 1 {
			return data.Frame{}, fmt.Errorf("q window join requires a time key")
		}
		timeKey := join.Keys[len(join.Keys)-1]
		partitionKeys := join.Keys[:len(join.Keys)-1]
		if join.HasWindow || join.Kind == "window1" {
			return data.WindowJoinOnWithOptions(left, right.frame, data.WindowJoinOptions{
				TimeKey:       timeKey,
				PartitionKeys: partitionKeys,
				Low:           join.WindowLow,
				High:          join.WindowHigh,
				HasBounds:     join.HasWindow,
				Last:          join.Kind == "window1",
			})
		}
		return data.WindowJoinOn(left, right.frame, timeKey, partitionKeys...)
	default:
		return data.Frame{}, fmt.Errorf("q join kind %q is not supported", join.Kind)
	}
}

func qExplainSQL(args qSQLArgsResult) (Value, error) {
	tmpl, err := qSQLCachedPlanTemplate("q.explain", args.source)
	if err != nil {
		return NilValue(), err
	}
	out := NewTable()
	out.RawSetString("op", StringValue(string(tmpl.op)))
	out.RawSetString("source_query", StringValue(args.source))
	out.RawSetString("source", StringValue(tmpl.source))
	out.RawSetString("source_path", StringValue(tmpl.sourcePath))
	out.RawSetString("has_literal_frame", BoolValue(tmpl.literalFrame != nil))
	out.RawSetString("literal_keys", qSymbolsExplainValue(tmpl.literalKeys))
	out.RawSetString("hidden_columns", qSymbolsExplainValue(tmpl.hiddenCols))
	out.RawSetString("distinct", BoolValue(tmpl.plan.Distinct))
	out.RawSetString("where", StringValue(qExplainExpr(tmpl.plan.Where)))
	out.RawSetString("by", qSymbolsExplainValue(tmpl.plan.By))
	out.RawSetString("by_exprs", qExplainSelectItemsValue(tmpl.plan.ByExprs))
	out.RawSetString("select", qExplainSelectItemsValue(tmpl.plan.Select))
	out.RawSetString("aggregates", qExplainAggregatesValue(tmpl.plan.Aggregates))
	out.RawSetString("order_by", qExplainOrdersValue(tmpl.plan.OrderBy))
	out.RawSetString("limit", IntValue(int64(tmpl.plan.LimitN)))
	out.RawSetString("pre_project_order", BoolValue(tmpl.plan.PreProjectOrder))
	out.RawSetString("exec_dict", qExplainExecDictValue(tmpl.execDict))
	out.RawSetString("join", qExplainJoinValue(tmpl.join))
	joins := qSQLTemplateJoins(tmpl)
	out.RawSetString("joins", qExplainJoinsValue(joins))
	out.RawSetString("join_count", IntValue(int64(len(joins))))
	out.RawSetString("mutation", qExplainMutationValue(tmpl.mutation))
	out.RawSetString("cacheable", BoolValue(true))
	sourceInfo := qExplainSourceBridgeInfo(args, tmpl)
	out.RawSetString("source_bridge", StringValue(sourceInfo.bridge))
	out.RawSetString("source_native", BoolValue(sourceInfo.native))
	out.RawSetString("source_keyed", BoolValue(sourceInfo.keyed))
	out.RawSetString("source_keys", qSymbolsExplainValue(sourceInfo.keys))
	out.RawSetString("source_rows", IntValue(int64(sourceInfo.rows)))
	kernelInfo := qExplainKernelInfo(args, tmpl)
	out.RawSetString("source_schema", qExplainSchemaValue(kernelInfo.schema))
	out.RawSetString("source_schema_hash", StringValue(qExplainKernelSchemaHash(kernelInfo)))
	out.RawSetString("kernel_cache_stats_domain", StringValue(qStatsDomainSemanticCache))
	out.RawSetString("kernel_supported", BoolValue(kernelInfo.supported))
	out.RawSetString("kernel_cached", BoolValue(kernelInfo.cached))
	out.RawSetString("kernel_decision_cached", BoolValue(kernelInfo.decisionCached))
	out.RawSetString("kernel_cache_key", StringValue(kernelInfo.cacheKey))
	out.RawSetString("kernel_cache_namespace", StringValue(kernelInfo.cacheNamespace))
	out.RawSetString("kernel_cache_kind", StringValue(kernelInfo.cacheKind))
	out.RawSetString("kernel_cache_schema_hash", StringValue(kernelInfo.cacheSchemaHash))
	out.RawSetString("kernel_cache_schema_match", BoolValue(qExplainKernelCacheSchemaMatches(kernelInfo)))
	out.RawSetString("kernel_plan_fingerprint", StringValue(kernelInfo.planFingerprint))
	out.RawSetString("kernel_shape", StringValue(kernelInfo.shape))
	out.RawSetString("kernel_reason_code", StringValue(kernelInfo.reasonCode))
	out.RawSetString("kernel_reason", StringValue(kernelInfo.reason))
	out.RawSetString("kernel_execution_stats_domain", StringValue(qStatsDomainJITExecution))
	out.RawSetString("kernel_execution_stats_source", StringValue(qStatsSourceMethodJIT))
	out.RawSetString("kernel_execution_stats_cache_backed", BoolValue(false))
	out.RawSetString("kernel_lowering_stats_domain", StringValue(qStatsDomainJITLowering))
	out.RawSetString("kernel_lowering_stats_source", StringValue(qStatsSourceMethodJIT))
	out.RawSetString("kernel_lowering_stats_cache_backed", BoolValue(false))
	qExplainAttachRuntimeKernelExecutionSummary(out, kernelInfo.shape)
	qExplainAttachRuntimeKernelLoweringSummary(out, kernelInfo.shape)
	qExplainAttachFallbackStats(out, qSQLKernelFallbackStatsCode(kernelInfo), kernelInfo.reasonCode, kernelInfo.reason)
	return TableValue(out), nil
}

type qExplainKernelResult struct {
	schema          data.Schema
	schemaHash      string
	cacheKey        string
	cacheNamespace  string
	cacheKind       string
	cacheSchemaHash string
	planFingerprint string
	shape           string
	supported       bool
	cached          bool
	decisionCached  bool
	reasonCode      string
	reason          string
}

type qExplainSourceBridgeResult struct {
	bridge string
	native bool
	keyed  bool
	keys   []data.Symbol
	rows   int
}

type qSQLSourceCarrier struct {
	frame    data.Frame
	keyed    data.KeyedFrame
	hasKeyed bool
	info     NativePayloadInfo
	hasInfo  bool
	bridge   string
	native   bool
	keys     []data.Symbol
	rows     int
}

type qSQLSourceNotFoundError struct {
	name string
}

func (e qSQLSourceNotFoundError) Error() string {
	return fmt.Sprintf("source %q not found", e.name)
}

func qExplainSourceBridgeInfo(args qSQLArgsResult, tmpl qSQLPlanTemplate) qExplainSourceBridgeResult {
	switch {
	case tmpl.literalFrame != nil:
		return qExplainSourceBridgeResult{bridge: "literal_frame", native: true, rows: tmpl.literalFrame.Len()}
	case tmpl.sourcePath != "":
		frame, err := data.LoadPartitionedFrameDir(tmpl.sourcePath, nil)
		if err != nil {
			return qExplainSourceBridgeResult{bridge: "partitioned_frame_unavailable"}
		}
		return qExplainSourceBridgeResult{bridge: "partitioned_frame", native: true, rows: frame.Len()}
	}
	sourceName := ""
	if args.resolveSource {
		sourceName = tmpl.source
	}
	carrier, err := qSQLSourceCarrierFromValue(args.frameValue, sourceName)
	if err != nil {
		if _, ok := err.(qSQLSourceNotFoundError); ok {
			return qExplainSourceBridgeResult{bridge: "source_not_found"}
		}
		value, resolveErr := qResolveSQLSourceValue(args.frameValue, sourceName)
		if resolveErr != nil {
			value = args.frameValue
		}
		return qExplainSourceBridgeResult{bridge: qExplainUnavailableBridge(value)}
	}
	return qExplainSourceBridgeResult{
		bridge: carrier.bridge,
		native: carrier.native,
		keyed:  carrier.hasKeyed,
		keys:   carrier.keys,
		rows:   carrier.rows,
	}
}

func qExplainUnavailableBridge(value Value) string {
	if value.IsSoA() {
		return "soa_unavailable"
	}
	if !value.IsTable() {
		return "unsupported_source"
	}
	tbl := value.Table()
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		switch kind {
		case NativePayloadKeyedFrame:
			return "keyed_frame_unavailable"
		case NativePayloadDataFrame:
			return "frame_native_unavailable"
		}
		return "frame_native_unavailable"
	}
	if qIsKeyedFrameTable(tbl) {
		return "keyed_frame_unavailable"
	}
	if qLooksLikeFrame(tbl) {
		return "frame_wrapper_unavailable"
	}
	return "row_table_unavailable"
}

func qSQLSourceCarrierFromValue(v Value, sourceName string) (qSQLSourceCarrier, error) {
	source, err := qResolveSQLSourceValue(v, sourceName)
	if err != nil {
		return qSQLSourceCarrier{}, err
	}
	return qSQLSourceCarrierFromResolvedValue(source)
}

func qResolveSQLSourceValue(v Value, sourceName string) (Value, error) {
	if sourceName == "" || !qCanResolveSQLSourceFromTable(v) {
		return v, nil
	}
	source := v.Table().RawGetString(sourceName)
	if source.IsNil() {
		return NilValue(), qSQLSourceNotFoundError{name: sourceName}
	}
	return source, nil
}

func qCanResolveSQLSourceFromTable(v Value) bool {
	if !v.IsTable() {
		return false
	}
	tbl := v.Table()
	if _, ok := qNativeFrameRuntimeKind(tbl); ok {
		return false
	}
	return !qLooksLikeFrame(tbl) && !qIsKeyedFrameTable(tbl)
}

func qSQLSourceCarrierFromResolvedValue(v Value) (qSQLSourceCarrier, error) {
	if v.IsSoA() {
		frame, err := qDataFrameFromSoA(v.SoA())
		if err != nil {
			return qSQLSourceCarrier{}, err
		}
		return qSQLSourceCarrier{frame: frame, bridge: "soa", rows: frame.Len()}, nil
	}
	if !v.IsTable() {
		return qSQLSourceCarrier{}, fmt.Errorf("argument 1 must be a frame table or soa")
	}
	tbl := v.Table()
	if carrier, ok, err := qSQLNativeSourceCarrierFromTable(tbl); err != nil {
		return qSQLSourceCarrier{}, err
	} else if ok {
		return carrier, nil
	}
	if qIsKeyedFrameTable(tbl) {
		keyed, err := qKeyedFrameFromValue(v)
		if err != nil {
			return qSQLSourceCarrier{}, err
		}
		info, hasInfo := qFramePayloadInfo(tbl, NativePayloadKeyedFrame)
		native := false
		if qNativeFrameRuntimeKindMatches(tbl, NativePayloadKeyedFrame) {
			native = true
		}
		bridge := "keyed_frame_wrapper"
		if native {
			bridge = "keyed_frame_native"
		}
		frame := keyed.Frame()
		rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
		return qSQLSourceCarrier{
			frame:    frame,
			keyed:    keyed,
			hasKeyed: true,
			info:     info,
			hasInfo:  hasInfo,
			bridge:   bridge,
			native:   native,
			keys:     keyed.Keys(),
			rows:     rows,
		}, nil
	}
	bridge := "row_table"
	native := false
	info, hasInfo := qFramePayloadInfo(tbl, NativePayloadDataFrame)
	if qNativeFrameRuntimeKindMatches(tbl, NativePayloadDataFrame) {
		bridge = "frame_native"
		native = true
	} else if qLooksLikeFrame(tbl) {
		bridge = "frame_wrapper"
	}
	frame, err := qDataFrameFromValue(v, "")
	if err != nil {
		return qSQLSourceCarrier{}, err
	}
	rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
	return qSQLSourceCarrier{frame: frame, info: info, hasInfo: hasInfo, bridge: bridge, native: native, rows: rows}, nil
}

func qSQLNativeSourceCarrierFromTable(tbl *Table) (qSQLSourceCarrier, bool, error) {
	if tbl == nil {
		return qSQLSourceCarrier{}, false, nil
	}
	kind, ok := qNativeFrameRuntimeKind(tbl)
	if !ok {
		return qSQLSourceCarrier{}, false, nil
	}
	switch kind {
	case NativePayloadKeyedFrame:
		keyed, ok, err := qNativeKeyedFramePayload(tbl)
		if err != nil {
			return qSQLSourceCarrier{}, false, err
		}
		if !ok {
			return qSQLSourceCarrier{}, false, fmt.Errorf("native keyed frame payload is invalid")
		}
		frame := keyed.Frame()
		info, hasInfo := qFramePayloadInfo(tbl, NativePayloadKeyedFrame)
		rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
		return qSQLSourceCarrier{
			frame:    frame,
			keyed:    keyed,
			hasKeyed: true,
			info:     info,
			hasInfo:  hasInfo,
			bridge:   "keyed_frame_native",
			native:   true,
			keys:     keyed.Keys(),
			rows:     rows,
		}, true, nil
	case NativePayloadDataFrame:
		frame, ok, err := qNativeDataFramePayload(tbl)
		if err != nil {
			return qSQLSourceCarrier{}, false, err
		}
		if !ok {
			return qSQLSourceCarrier{}, false, fmt.Errorf("native data frame payload is invalid")
		}
		info, hasInfo := qFramePayloadInfo(tbl, NativePayloadDataFrame)
		rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
		return qSQLSourceCarrier{
			frame:   frame,
			info:    info,
			hasInfo: hasInfo,
			bridge:  "frame_native",
			native:  true,
			rows:    rows,
		}, true, nil
	default:
		return qSQLSourceCarrier{}, false, nil
	}
}

func qSourceCarrierRows(frameRows int, info NativePayloadInfo, hasInfo bool) int {
	if hasInfo && info.Rows == frameRows {
		return info.Rows
	}
	return frameRows
}

func qSourceCarrierSchemaHash(frame data.Frame, info NativePayloadInfo, hasInfo bool) string {
	actual := frame.SchemaFingerprint()
	if hasInfo && info.SchemaHash == actual {
		return info.SchemaHash
	}
	return actual
}

func qFramePayloadInfo(tbl *Table, kind NativePayloadKind) (NativePayloadInfo, bool) {
	if tbl == nil {
		return NativePayloadInfo{}, false
	}
	_, info, ok := qTypedNativeFramePayload(tbl)
	if !ok || info.Kind != kind {
		return NativePayloadInfo{}, false
	}
	return info, true
}

func qNativeFrameRuntimeKind(tbl *Table) (NativePayloadKind, bool) {
	if tbl == nil {
		return NativePayloadNone, false
	}
	if _, info, ok := qTypedNativeFramePayload(tbl); ok {
		return info.Kind, true
	}
	return qLegacyNativeFramePayloadKind(tbl)
}

func qLegacyNativeFramePayloadKind(tbl *Table) (NativePayloadKind, bool) {
	if tbl == nil {
		return NativePayloadNone, false
	}
	if _, hasInfo := tbl.NativePayloadInfo(); hasInfo {
		return NativePayloadNone, false
	}
	switch tbl.NativePayload().(type) {
	case data.Frame:
		return NativePayloadDataFrame, true
	case data.KeyedFrame:
		return NativePayloadKeyedFrame, true
	default:
		return NativePayloadNone, false
	}
}

func qNativeFrameRuntimeKindMatches(tbl *Table, want NativePayloadKind) bool {
	kind, ok := qNativeFrameRuntimeKind(tbl)
	return ok && kind == want
}

func qTypedNativeFramePayload(tbl *Table) (any, NativePayloadInfo, bool) {
	if tbl == nil {
		return nil, NativePayloadInfo{}, false
	}
	return tbl.NativeFramePayload()
}

func qLegacyNativeDataFramePayload(tbl *Table) (data.Frame, bool) {
	if kind, ok := qLegacyNativeFramePayloadKind(tbl); !ok || kind != NativePayloadDataFrame {
		return data.Frame{}, false
	}
	native, ok := tbl.NativePayload().(data.Frame)
	return native, ok
}

func qTypedNativeDataFramePayload(tbl *Table) (data.Frame, bool, error) {
	payload, info, ok := qTypedNativeFramePayload(tbl)
	if !ok {
		return data.Frame{}, false, nil
	}
	if info.Kind != NativePayloadDataFrame {
		return data.Frame{}, false, nil
	}
	switch native := payload.(type) {
	case data.Frame:
		return native, true, nil
	case *SoA:
		frame, err := qDataFrameFromSoA(native)
		if err != nil {
			return data.Frame{}, false, err
		}
		return frame, true, nil
	default:
		return data.Frame{}, false, fmt.Errorf("native data frame payload is invalid")
	}
}

func qTypedNativeKeyedFramePayload(tbl *Table) (data.KeyedFrame, bool, error) {
	payload, info, ok := qTypedNativeFramePayload(tbl)
	if !ok {
		return data.KeyedFrame{}, false, nil
	}
	if info.Kind != NativePayloadKeyedFrame {
		return data.KeyedFrame{}, false, nil
	}
	native, hasPayload := payload.(data.KeyedFrame)
	if !hasPayload {
		return data.KeyedFrame{}, false, fmt.Errorf("native keyed frame payload is invalid")
	}
	return native, true, nil
}

func qLegacyNativeKeyedFramePayload(tbl *Table) (data.KeyedFrame, bool) {
	if kind, ok := qLegacyNativeFramePayloadKind(tbl); !ok || kind != NativePayloadKeyedFrame {
		return data.KeyedFrame{}, false
	}
	native, ok := tbl.NativePayload().(data.KeyedFrame)
	return native, ok
}

func qExplainKernelInfo(args qSQLArgsResult, tmpl qSQLPlanTemplate) qExplainKernelResult {
	if tmpl.mutation != nil {
		return qExplainKernelResult{reasonCode: qKernelReasonMutationPlan, reason: "mutation queries use mutation plan cache"}
	}
	var frame data.Frame
	var source qSQLSourceCarrier
	var hasSource bool
	var err error
	sourceName := tmpl.source
	switch {
	case tmpl.literalFrame != nil:
		frame = *tmpl.literalFrame
	case tmpl.sourcePath != "":
		frame, err = data.LoadPartitionedFrameDir(tmpl.sourcePath, nil)
	default:
		if args.resolveSource {
			sourceName = tmpl.source
		} else {
			sourceName = ""
		}
		source, err = qSQLSourceCarrierFromValue(args.frameValue, sourceName)
		hasSource = true
		frame = source.frame
	}
	if err != nil {
		return qExplainKernelResult{reasonCode: qKernelReasonSourceUnavailable, reason: err.Error()}
	}
	if len(qSQLTemplateJoins(tmpl)) > 0 {
		frame, err = qExplainJoinFrame(args, tmpl, frame)
		if err != nil {
			return qExplainKernelResult{schema: frame.Schema(), reasonCode: qKernelReasonJoinUnavailable, reason: err.Error()}
		}
		hasSource = false
	}
	bindings := qSQLScalarBindingsFromValue(args.envValue)
	plan := qPrepareSQLPlanForFrame(args.source, tmpl.plan, frame, bindings, false)
	kernelKey := data.QueryKernelCacheKey(args.source, frame, plan)
	kernelKeyInfo := qSQLKernelCacheKeyInfo(kernelKey)
	planFingerprint := ""
	if len(kernelKeyInfo.Extra) > 0 {
		planFingerprint = kernelKeyInfo.Extra[0]
	}
	schemaHash := ""
	if hasSource && source.hasInfo {
		schemaHash = qSourceCarrierSchemaHash(frame, source.info, source.hasInfo)
	}
	qSQLAlignedPlanCacheMu.Lock()
	cachedKernel, cached := qSQLKernelCache[kernelKey]
	unsupportedReason, unsupportedCached := qSQLKernelUnsupported[kernelKey]
	unsupportedShape := qSQLKernelUnsupportedShape[kernelKey]
	qSQLAlignedPlanCacheMu.Unlock()
	if cached {
		reason := cachedKernel.Reason()
		if reason == "" {
			reason = qKernelReasonSupported
		}
		shape := ""
		if cachedKernel != nil {
			shape = cachedKernel.Shape()
		}
		return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, cacheKey: kernelKey, cacheNamespace: kernelKeyInfo.Namespace, cacheKind: kernelKeyInfo.Kind, cacheSchemaHash: kernelKeyInfo.SchemaHash, planFingerprint: planFingerprint, shape: shape, supported: true, cached: true, decisionCached: true, reasonCode: qKernelReasonSupported, reason: reason}
	}
	if unsupportedCached {
		return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, cacheKey: kernelKey, cacheNamespace: kernelKeyInfo.Namespace, cacheKind: kernelKeyInfo.Kind, cacheSchemaHash: kernelKeyInfo.SchemaHash, planFingerprint: planFingerprint, shape: unsupportedShape, cached: false, decisionCached: true, reasonCode: stdq.KernelFallbackReasonCode(unsupportedReason), reason: unsupportedReason}
	}
	supported, reason, err := data.QueryKernelCompileReason(frame, plan)
	if err != nil {
		return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, cacheKey: kernelKey, cacheNamespace: kernelKeyInfo.Namespace, cacheKind: kernelKeyInfo.Kind, cacheSchemaHash: kernelKeyInfo.SchemaHash, planFingerprint: planFingerprint, cached: cached, decisionCached: cached, reasonCode: qKernelReasonCompileError, reason: err.Error()}
	}
	if !supported {
		return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, cacheKey: kernelKey, cacheNamespace: kernelKeyInfo.Namespace, cacheKind: kernelKeyInfo.Kind, cacheSchemaHash: kernelKeyInfo.SchemaHash, planFingerprint: planFingerprint, shape: data.QueryKernelPlanShape(plan), cached: cached, decisionCached: cached, reasonCode: stdq.KernelFallbackReasonCode(reason), reason: reason}
	}
	return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, cacheKey: kernelKey, cacheNamespace: kernelKeyInfo.Namespace, cacheKind: kernelKeyInfo.Kind, cacheSchemaHash: kernelKeyInfo.SchemaHash, planFingerprint: planFingerprint, shape: data.QueryKernelPlanShape(plan), supported: true, cached: cached, decisionCached: cached, reasonCode: qKernelReasonSupported, reason: reason}
}

func qSQLKernelCacheKeyInfo(key string) data.SchemaStableCacheKeyParts {
	info, ok := data.ParseSchemaStableCacheKey(key)
	if !ok {
		return data.SchemaStableCacheKeyParts{}
	}
	return info
}

func qExplainKernelSchemaHash(info qExplainKernelResult) string {
	if info.schemaHash != "" {
		return info.schemaHash
	}
	return info.schema.Fingerprint()
}

func qExplainKernelCacheSchemaMatches(info qExplainKernelResult) bool {
	if info.cacheSchemaHash == "" {
		return false
	}
	return info.cacheSchemaHash == qExplainKernelSchemaHash(info)
}

func qExplainJoinFrame(args qSQLArgsResult, tmpl qSQLPlanTemplate, frame data.Frame) (data.Frame, error) {
	if len(qSQLTemplateJoins(tmpl)) == 0 {
		return frame, nil
	}
	if !args.resolveSource {
		return data.Frame{}, fmt.Errorf("join queries require a source table map")
	}
	for _, join := range qSQLTemplateJoins(tmpl) {
		var err error
		frame, err = qApplySQLJoin(args.frameValue, frame, join)
		if err != nil {
			return data.Frame{}, fmt.Errorf("join: %w", err)
		}
	}
	return frame, nil
}

func qExplainSchemaValue(schema data.Schema) Value {
	names := schema.Names()
	out := NewAppendArrayTable(len(names))
	for i, name := range names {
		row := NewTable()
		row.RawSetString("name", StringValue(string(name)))
		if kind, ok := schema.Kind(name); ok {
			row.RawSetString("kind", StringValue(string(kind)))
		} else {
			row.RawSetString("kind", StringValue(""))
		}
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainSoASchemaValue(soa *SoA) Value {
	names := soa.ColumnNames()
	out := NewAppendArrayTable(len(names))
	for i, name := range names {
		row := NewTable()
		row.RawSetString("name", StringValue(name))
		kind := ""
		if col, ok := soa.Column(name); ok && col != nil {
			kind = col.DType().String()
		}
		row.RawSetString("kind", StringValue(kind))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qSymbolsExplainValue(symbols []data.Symbol) Value {
	out := NewAppendArrayTable(len(symbols))
	for i, sym := range symbols {
		out.RawSetInt(int64(i+1), StringValue(string(sym)))
	}
	return TableValue(out)
}

func qExplainSelectItemsValue(items []data.SelectItem) Value {
	out := NewAppendArrayTable(len(items))
	for i, item := range items {
		row := NewTable()
		row.RawSetString("name", StringValue(string(item.Name)))
		row.RawSetString("expr", StringValue(qExplainExpr(item.Expr)))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainAggregatesValue(items []data.Aggregate) Value {
	out := NewAppendArrayTable(len(items))
	for i, item := range items {
		row := NewTable()
		row.RawSetString("name", StringValue(string(item.Name)))
		row.RawSetString("fn", StringValue(item.Func))
		row.RawSetString("expr", StringValue(qExplainExpr(item.Expr)))
		row.RawSetString("weight", StringValue(qExplainExpr(item.Weight)))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainOrdersValue(items []data.OrderSpec) Value {
	out := NewAppendArrayTable(len(items))
	for i, item := range items {
		row := NewTable()
		row.RawSetString("column", StringValue(string(item.Column)))
		row.RawSetString("desc", BoolValue(item.Desc))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainExecDictValue(plan *stdq.ExecDictPlan) Value {
	if plan == nil {
		return NilValue()
	}
	out := NewTable()
	out.RawSetString("key", StringValue(string(plan.KeyName)))
	out.RawSetString("value", StringValue(string(plan.ValueName)))
	return TableValue(out)
}

func qExplainJoinValue(plan *stdq.JoinPlan) Value {
	return qExplainJoinValueAt(plan, 0)
}

func qExplainJoinValueAt(plan *stdq.JoinPlan, ordinal int) Value {
	if plan == nil {
		return NilValue()
	}
	out := NewTable()
	if ordinal > 0 {
		out.RawSetString("ordinal", IntValue(int64(ordinal)))
	}
	out.RawSetString("kind", StringValue(plan.Kind))
	out.RawSetString("right", StringValue(plan.Right))
	out.RawSetString("key_count", IntValue(int64(len(plan.Keys))))
	keys := NewAppendArrayTable(len(plan.Keys))
	leftPartitions := make([]data.Symbol, 0, len(plan.Keys))
	rightPartitions := make([]data.Symbol, 0, len(plan.Keys))
	for i, key := range plan.Keys {
		row := NewTable()
		row.RawSetString("left", StringValue(string(key.Left)))
		row.RawSetString("right", StringValue(string(key.Right)))
		keys.RawSetInt(int64(i+1), TableValue(row))
		if i < len(plan.Keys)-1 {
			leftPartitions = append(leftPartitions, key.Left)
			rightPartitions = append(rightPartitions, key.Right)
		}
	}
	out.RawSetString("keys", TableValue(keys))
	if qJoinKindUsesTimeKey(plan.Kind) && len(plan.Keys) > 0 {
		timeKey := plan.Keys[len(plan.Keys)-1]
		out.RawSetString("time_key", StringValue(string(timeKey.Left)))
		out.RawSetString("left_time_key", StringValue(string(timeKey.Left)))
		out.RawSetString("right_time_key", StringValue(string(timeKey.Right)))
		out.RawSetString("partition_keys", qSymbolsExplainValue(leftPartitions))
		out.RawSetString("left_partition_keys", qSymbolsExplainValue(leftPartitions))
		out.RawSetString("right_partition_keys", qSymbolsExplainValue(rightPartitions))
	}
	out.RawSetString("has_window", BoolValue(plan.HasWindow))
	out.RawSetString("window_low", qAnyToValue(plan.WindowLow))
	out.RawSetString("window_high", qAnyToValue(plan.WindowHigh))
	return TableValue(out)
}

func qExplainJoinsValue(plans []*stdq.JoinPlan) Value {
	out := NewAppendArrayTable(len(plans))
	for i, plan := range plans {
		out.RawSetInt(int64(i+1), qExplainJoinValueAt(plan, i+1))
	}
	return TableValue(out)
}

func qJoinKindUsesTimeKey(kind string) bool {
	switch kind {
	case "asof", "asof0", "asof_fill", "asof_fill0", "window", "window1":
		return true
	default:
		return false
	}
}

func qExplainMutationValue(plan *stdq.MutationPlan) Value {
	if plan == nil {
		return NilValue()
	}
	out := NewTable()
	out.RawSetString("kind", StringValue(string(plan.Kind)))
	out.RawSetString("where", StringValue(qExplainExpr(plan.Where)))
	out.RawSetString("by_exprs", qExplainSelectItemsValue(plan.ByExprs))
	assignments := NewAppendArrayTable(len(plan.Assignments))
	for i, item := range plan.Assignments {
		row := NewTable()
		row.RawSetString("name", StringValue(string(item.Name)))
		row.RawSetString("fn", StringValue(item.Func))
		row.RawSetString("expr", StringValue(qExplainExpr(item.Expr)))
		assignments.RawSetInt(int64(i+1), TableValue(row))
	}
	out.RawSetString("assignments", TableValue(assignments))
	out.RawSetString("delete_columns", qSymbolsExplainValue(plan.DeleteColumns))
	out.RawSetString("insert_columns", qSymbolsExplainValue(plan.InsertColumns))
	insertValues := NewAppendArrayTable(len(plan.InsertValues))
	for i, item := range plan.InsertValues {
		insertValues.RawSetInt(int64(i+1), StringValue(qExplainLiteral(item.Value)))
	}
	out.RawSetString("insert_values", TableValue(insertValues))
	return TableValue(out)
}

func qExplainExpr(expr data.Expr) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case data.ColumnRef:
		return string(e.Name)
	case data.Literal:
		return qExplainLiteral(e.Value)
	case data.Binary:
		return "(" + qExplainExpr(e.Left) + " " + string(e.Op) + " " + qExplainExpr(e.Right) + ")"
	case data.Logical:
		return "(" + qExplainExpr(e.Left) + " " + e.Op + " " + qExplainExpr(e.Right) + ")"
	case data.Not:
		return "not " + qExplainExpr(e.Expr)
	case data.In:
		parts := make([]string, 0, len(e.Values))
		for _, value := range e.Values {
			parts = append(parts, qExplainLiteral(value))
		}
		return qExplainExpr(e.Expr) + " in (" + strings.Join(parts, " ") + ")"
	case data.Within:
		return qExplainExpr(e.Expr) + " within (" + qExplainLiteral(e.Low) + " " + qExplainLiteral(e.High) + ")"
	case data.BucketFloorExpr:
		return "xbar " + qExplainLiteral(e.Interval) + " " + qExplainExpr(e.Expr)
	case data.ListAggregateExpr:
		return e.Func + " " + qExplainExpr(e.Expr)
	case data.VectorTransformExpr:
		if e.Arg != nil {
			return qExplainExpr(e.Arg) + " " + e.Func + " " + qExplainExpr(e.Expr)
		}
		return e.Func + " " + qExplainExpr(e.Expr)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func qExplainLiteral(value any) string {
	if data.IsNull(value) {
		return "null"
	}
	switch x := value.(type) {
	case string:
		return strconv.Quote(x)
	case data.Symbol:
		return "`" + string(x)
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			parts = append(parts, qExplainLiteral(item))
		}
		return "(" + strings.Join(parts, " ") + ")"
	}
	if s, ok := stdq.FormatTemporal(value); ok {
		return s
	}
	return fmt.Sprint(value)
}

func qSQLCachedPlanTemplate(name, src string) (qSQLPlanTemplate, error) {
	qSQLTemplateCacheMu.Lock()
	if tmpl, ok := qSQLTemplateCache[src]; ok {
		qSQLTemplateStats.TemplateHits++
		qSQLTemplateCacheMu.Unlock()
		tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
		tmpl.sourcePath = qNormalizePathString(tmpl.sourcePath)
		tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
		tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
		tmpl.hiddenCols = qCloneDataSymbols(tmpl.hiddenCols)
		tmpl.execDict = qCloneQExecDictPlan(tmpl.execDict)
		tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
		tmpl.join = qCloneQJoinPlan(tmpl.join)
		tmpl.joins = qCloneQJoinPlans(tmpl.joins)
		return tmpl, nil
	}
	qSQLTemplateStats.TemplateMisses++
	qSQLTemplateCacheMu.Unlock()

	query, err := stdq.Parse(strings.TrimSpace(src))
	if err != nil {
		return qSQLPlanTemplate{}, fmt.Errorf("%s: parse: %w", name, err)
	}
	lowered, err := stdq.Lower(query)
	if err != nil {
		return qSQLPlanTemplate{}, fmt.Errorf("%s: lower: %w", name, err)
	}
	if lowered.Mutation != nil {
		mutation := qCloneQMutationPlan(lowered.Mutation)
		qNormalizeMutationLiterals(mutation)
		tmpl := qSQLPlanTemplate{
			source:       lowered.Source,
			sourcePath:   qNormalizePathString(lowered.SourcePath),
			op:           lowered.Op,
			literalFrame: qCloneDataFramePtr(lowered.Frame),
			literalKeys:  qCloneDataSymbols(lowered.LiteralKeys),
			mutation:     qCloneQMutationPlan(mutation),
		}

		qSQLTemplateCacheMu.Lock()
		qSQLTemplateCacheStoreLocked(src, tmpl)
		qSQLTemplateCacheMu.Unlock()

		tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
		tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
		tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
		return tmpl, nil
	}
	if lowered.Distinct {
		lowered.Plan.Distinct = true
	}
	qNormalizePlanLiterals(&lowered.Plan)
	if lowered.Frame == nil {
		lowered.Plan.Source = data.Frame{}
	}
	tmpl := qSQLPlanTemplate{
		source:       lowered.Source,
		sourcePath:   qNormalizePathString(lowered.SourcePath),
		op:           lowered.Op,
		literalFrame: qCloneDataFramePtr(lowered.Frame),
		literalKeys:  qCloneDataSymbols(lowered.LiteralKeys),
		hiddenCols:   qCloneDataSymbols(lowered.HiddenCols),
		plan:         qCloneDataQueryPlan(lowered.Plan),
		execDict:     qCloneQExecDictPlan(lowered.ExecDict),
		join:         qCloneQJoinPlan(lowered.Join),
		joins:        qCloneQJoinPlans(lowered.Joins),
	}

	qSQLTemplateCacheMu.Lock()
	qSQLTemplateCacheStoreLocked(src, tmpl)
	qSQLTemplateCacheMu.Unlock()

	tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
	tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
	tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
	tmpl.hiddenCols = qCloneDataSymbols(tmpl.hiddenCols)
	tmpl.execDict = qCloneQExecDictPlan(tmpl.execDict)
	tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
	tmpl.join = qCloneQJoinPlan(tmpl.join)
	tmpl.joins = qCloneQJoinPlans(tmpl.joins)
	return tmpl, nil
}

func qDropHiddenSQLColumns(frame data.Frame, hidden []data.Symbol) (data.Frame, error) {
	if len(hidden) == 0 {
		return frame, nil
	}
	return data.DropColumns(frame, hidden...)
}

func qPrepareSQLPlanForFrame(src string, tmpl data.QueryPlan, frame data.Frame, bindings map[data.Symbol]any, useAlignedCache bool) data.QueryPlan {
	var plan data.QueryPlan
	if useAlignedCache {
		plan = qSQLPlanForFrame(src, tmpl, frame)
	} else {
		plan = qAlignSQLPlanForFrame(tmpl, frame)
	}
	qBindPlanOuterScalars(&plan, frame.Schema(), bindings)
	plan.Source = frame
	return plan
}

func qSQLPlanForFrame(src string, tmpl data.QueryPlan, frame data.Frame) data.QueryPlan {
	key := data.QueryAlignedPlanCacheKey(src, frame)
	qSQLAlignedPlanCacheMu.Lock()
	if plan, ok := qSQLAlignedPlanCache[key]; ok {
		qSQLAlignedStats.AlignedHits++
		qSQLAlignedPlanCacheMu.Unlock()
		return qCloneDataQueryPlan(plan)
	}
	qSQLAlignedStats.AlignedMisses++
	qSQLAlignedPlanCacheMu.Unlock()

	plan := qAlignSQLPlanForFrame(tmpl, frame)

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedPlanCacheStoreLocked(key, plan)
	qSQLAlignedPlanCacheMu.Unlock()

	return qCloneDataQueryPlan(plan)
}

func qAlignSQLPlanForFrame(tmpl data.QueryPlan, frame data.Frame) data.QueryPlan {
	plan := qCloneDataQueryPlan(tmpl)
	qAlignPlanLiteralsToFrame(&plan, frame)
	qExpandAllColumnSelects(&plan, frame)
	qSetPreProjectOrder(&plan, frame)
	return plan
}

func qSQLMutationForFrame(src string, tmpl *stdq.MutationPlan, frame data.Frame) *stdq.MutationPlan {
	key := data.QueryAlignedMutationCacheKey(src, frame)
	qSQLAlignedPlanCacheMu.Lock()
	if mutation, ok := qSQLAlignedMutationCache[key]; ok {
		qSQLAlignedStats.AlignedHits++
		qSQLAlignedPlanCacheMu.Unlock()
		return qCloneQMutationPlan(mutation)
	}
	qSQLAlignedStats.AlignedMisses++
	qSQLAlignedPlanCacheMu.Unlock()

	mutation := qCloneQMutationPlan(tmpl)
	qAlignMutationLiteralsToFrame(mutation, frame)

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedMutationCacheStoreLocked(key, mutation)
	qSQLAlignedPlanCacheMu.Unlock()

	return qCloneQMutationPlan(mutation)
}

func qSQLKernelForFrame(src string, plan data.QueryPlan, frame data.Frame) (*data.QueryKernel, bool, string, error) {
	key := data.QueryKernelCacheKey(src, frame, plan)
	qSQLAlignedPlanCacheMu.Lock()
	if kernel, ok := qSQLKernelCache[key]; ok {
		qSQLAlignedStats.KernelHits++
		qSQLKernelStatsForKeyLocked(key).Hits++
		qSQLAlignedPlanCacheMu.Unlock()
		return kernel, true, "", nil
	}
	if reason, ok := qSQLKernelUnsupported[key]; ok {
		qSQLAlignedStats.KernelDecisionHits++
		qSQLAlignedPlanCacheMu.Unlock()
		return nil, false, reason, nil
	}
	qSQLAlignedPlanCacheMu.Unlock()

	kernel, ok, err := data.CompileQueryKernel(frame, plan)
	if err != nil || !ok {
		reason := ""
		if !ok && err == nil {
			_, reason = data.QueryKernelSupportReason(plan)
			qSQLAlignedPlanCacheMu.Lock()
			qSQLAlignedStats.KernelDecisionMisses++
			qSQLKernelUnsupportedStoreLocked(key, reason, data.QueryKernelPlanShape(plan))
			qSQLAlignedPlanCacheMu.Unlock()
		}
		return nil, ok, reason, err
	}

	qSQLAlignedPlanCacheMu.Lock()
	if cached, ok := qSQLKernelCache[key]; ok {
		qSQLAlignedStats.KernelHits++
		qSQLKernelStatsForKeyLocked(key).Hits++
		qSQLAlignedPlanCacheMu.Unlock()
		return cached, true, "", nil
	}
	qSQLAlignedStats.KernelMisses++
	qSQLKernelStatsForKeyLocked(key).Misses++
	qSQLKernelCacheStoreLocked(key, kernel)
	qSQLAlignedPlanCacheMu.Unlock()
	return kernel, true, "", nil
}

func qRunSQLMutation(frame data.Frame, mutation *stdq.MutationPlan) (data.Frame, error) {
	if mutation == nil {
		return data.Frame{}, fmt.Errorf("mutation plan is nil")
	}
	switch mutation.Kind {
	case stdq.UpdateQuery:
		if len(mutation.ByExprs) > 0 {
			assignments := make([]data.GroupedAssignment, len(mutation.Assignments))
			for i, assign := range mutation.Assignments {
				assignments[i] = data.GroupedAssignment{Name: assign.Name, Func: assign.Func, Expr: assign.Expr}
			}
			return data.UpdateBy(frame, mutation.Where, mutation.ByExprs, assignments)
		}
		assignments := make(map[data.Symbol]data.Expr, len(mutation.Assignments))
		for _, assign := range mutation.Assignments {
			assignments[assign.Name] = assign.Expr
		}
		return data.UpdateWhere(frame, mutation.Where, assignments)
	case stdq.DeleteQuery:
		if len(mutation.DeleteColumns) > 0 {
			return data.DropColumns(frame, mutation.DeleteColumns...)
		}
		return data.DeleteWhere(frame, mutation.Where)
	case stdq.InsertQuery:
		if err := qValidateSQLInsertMutation(frame.Schema(), nil, mutation); err != nil {
			return data.Frame{}, err
		}
		return data.InsertRow(frame, mutation.InsertColumns, qSQLMutationValues(mutation))
	case stdq.UpsertQuery:
		if err := qValidateSQLInsertMutation(frame.Schema(), nil, mutation); err != nil {
			return data.Frame{}, err
		}
		return data.UpsertRow(frame, mutation.InsertColumns, qSQLMutationValues(mutation))
	default:
		return data.Frame{}, fmt.Errorf("unsupported q mutation kind %q", mutation.Kind)
	}
}

func qRunSQLKeyedMutation(keyed data.KeyedFrame, mutation *stdq.MutationPlan) (data.KeyedFrame, error) {
	if mutation == nil {
		return data.KeyedFrame{}, fmt.Errorf("mutation plan is nil")
	}
	switch mutation.Kind {
	case stdq.InsertQuery:
		if err := qValidateSQLInsertMutation(keyed.Frame().Schema(), keyed.Keys(), mutation); err != nil {
			return data.KeyedFrame{}, err
		}
		return data.InsertRowKeyed(keyed, mutation.InsertColumns, qSQLMutationValues(mutation))
	case stdq.UpsertQuery:
		if err := qValidateSQLInsertMutation(keyed.Frame().Schema(), keyed.Keys(), mutation); err != nil {
			return data.KeyedFrame{}, err
		}
		return data.UpsertRowKeyed(keyed, mutation.InsertColumns, qSQLMutationValues(mutation))
	default:
		frame, err := qRunSQLMutation(keyed.Frame(), mutation)
		if err != nil {
			return data.KeyedFrame{}, err
		}
		return data.KeyBy(frame, keyed.Keys()...)
	}
}

func qValidateSQLInsertMutation(schema data.Schema, keys []data.Symbol, mutation *stdq.MutationPlan) error {
	if mutation == nil {
		return fmt.Errorf("mutation plan is nil")
	}
	op := string(mutation.Kind)
	values := len(mutation.InsertValues)
	if values == 0 {
		return fmt.Errorf("q %s requires at least one value", op)
	}
	names := schema.Names()
	if len(mutation.InsertColumns) == 0 {
		if values != len(names) {
			return fmt.Errorf("q %s values count %d does not match target schema column count %d (%s)", op, values, len(names), qFormatSymbolList(names))
		}
		return nil
	}
	if len(mutation.InsertColumns) != values {
		return fmt.Errorf("q %s column count %d does not match values count %d", op, len(mutation.InsertColumns), values)
	}
	seen := make(map[data.Symbol]struct{}, len(mutation.InsertColumns))
	for _, column := range mutation.InsertColumns {
		if column == "" {
			return fmt.Errorf("q %s column name must not be empty", op)
		}
		if _, ok := schema.Kind(column); !ok {
			return fmt.Errorf("q %s column %q does not exist in target schema (%s)", op, column, qFormatSymbolList(names))
		}
		if _, ok := seen[column]; ok {
			return fmt.Errorf("q %s column %q is duplicated", op, column)
		}
		seen[column] = struct{}{}
	}
	for _, key := range keys {
		if _, ok := seen[key]; !ok {
			return fmt.Errorf("q %s keyed mutation requires key column %q in insert column list (%s)", op, key, qFormatSymbolList(keys))
		}
	}
	return nil
}

func qFormatSymbolList(symbols []data.Symbol) string {
	if len(symbols) == 0 {
		return "<empty>"
	}
	parts := make([]string, len(symbols))
	for i, symbol := range symbols {
		parts[i] = string(symbol)
	}
	return strings.Join(parts, ",")
}

func qSQLMutationValues(mutation *stdq.MutationPlan) []any {
	values := make([]any, len(mutation.InsertValues))
	for i, value := range mutation.InsertValues {
		values[i] = value.Value
	}
	return values
}

func qSQLTemplateCacheStoreLocked(key string, tmpl qSQLPlanTemplate) {
	if _, ok := qSQLTemplateCache[key]; !ok {
		qSQLTemplateOrder = append(qSQLTemplateOrder, key)
	}
	tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
	tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
	tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
	tmpl.hiddenCols = qCloneDataSymbols(tmpl.hiddenCols)
	tmpl.execDict = qCloneQExecDictPlan(tmpl.execDict)
	tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
	tmpl.join = qCloneQJoinPlan(tmpl.join)
	tmpl.joins = qCloneQJoinPlans(tmpl.joins)
	qSQLTemplateCache[key] = tmpl
	for len(qSQLTemplateOrder) > qSQLPlanCacheLimit {
		evict := qSQLTemplateOrder[0]
		qSQLTemplateOrder = qSQLTemplateOrder[1:]
		delete(qSQLTemplateCache, evict)
		qSQLTemplateStats.TemplateEvictions++
	}
}

func qSQLAlignedPlanCacheStoreLocked(key string, plan data.QueryPlan) {
	if _, ok := qSQLAlignedPlanCache[key]; !ok {
		qSQLAlignedPlanOrder = append(qSQLAlignedPlanOrder, key)
	}
	qSQLAlignedPlanCache[key] = qCloneDataQueryPlan(plan)
	for len(qSQLAlignedPlanOrder) > qSQLPlanCacheLimit {
		evict := qSQLAlignedPlanOrder[0]
		qSQLAlignedPlanOrder = qSQLAlignedPlanOrder[1:]
		delete(qSQLAlignedPlanCache, evict)
		qSQLAlignedStats.AlignedEvictions++
	}
}

func qSQLAlignedMutationCacheStoreLocked(key string, mutation *stdq.MutationPlan) {
	if _, ok := qSQLAlignedMutationCache[key]; !ok {
		qSQLAlignedMutationOrder = append(qSQLAlignedMutationOrder, key)
	}
	qSQLAlignedMutationCache[key] = qCloneQMutationPlan(mutation)
	for len(qSQLAlignedMutationOrder) > qSQLPlanCacheLimit {
		evict := qSQLAlignedMutationOrder[0]
		qSQLAlignedMutationOrder = qSQLAlignedMutationOrder[1:]
		delete(qSQLAlignedMutationCache, evict)
		qSQLAlignedStats.AlignedEvictions++
	}
}

func qSQLKernelCacheStoreLocked(key string, kernel *data.QueryKernel) {
	if _, ok := qSQLKernelCache[key]; !ok {
		qSQLKernelOrder = append(qSQLKernelOrder, key)
	}
	qSQLKernelCache[key] = kernel
	if kernel != nil {
		qSQLKernelStatsForKeyLocked(key).Shape = kernel.Shape()
	}
	for len(qSQLKernelOrder) > qSQLPlanCacheLimit {
		evict := qSQLKernelOrder[0]
		qSQLKernelOrder = qSQLKernelOrder[1:]
		delete(qSQLKernelCache, evict)
		qSQLAlignedStats.KernelEvictions++
		delete(qSQLKernelStatsByKey, evict)
	}
}

func qSQLKernelUnsupportedStoreLocked(key, reason, shape string) {
	if key == "" || reason == "" {
		return
	}
	if _, ok := qSQLKernelUnsupported[key]; !ok {
		qSQLKernelUnsupportedOrder = append(qSQLKernelUnsupportedOrder, key)
	}
	qSQLKernelUnsupported[key] = reason
	qSQLKernelUnsupportedShape[key] = shape
	for len(qSQLKernelUnsupportedOrder) > qSQLPlanCacheLimit {
		evict := qSQLKernelUnsupportedOrder[0]
		qSQLKernelUnsupportedOrder = qSQLKernelUnsupportedOrder[1:]
		delete(qSQLKernelUnsupported, evict)
		delete(qSQLKernelUnsupportedShape, evict)
		qSQLAlignedStats.KernelDecisionEvictions++
	}
}

func qSQLKernelStatsForKeyLocked(key string) *qSQLKernelCacheKeyStats {
	stats := qSQLKernelStatsByKey[key]
	if stats == nil {
		stats = &qSQLKernelCacheKeyStats{Key: key}
		qSQLKernelStatsByKey[key] = stats
	}
	return stats
}

func qSQLKernelStatsByKeySnapshotLocked() []qSQLKernelCacheKeyStats {
	out := make([]qSQLKernelCacheKeyStats, 0, len(qSQLKernelStatsByKey))
	for _, stats := range qSQLKernelStatsByKey {
		if stats == nil {
			continue
		}
		out = append(out, *stats)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func qSQLPlanCacheStatsSnapshot() qSQLPlanCacheStats {
	qSQLTemplateCacheMu.Lock()
	stats := qSQLTemplateStats
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	stats.AlignedHits = qSQLAlignedStats.AlignedHits
	stats.AlignedMisses = qSQLAlignedStats.AlignedMisses
	stats.AlignedEvictions = qSQLAlignedStats.AlignedEvictions
	stats.KernelHits = qSQLAlignedStats.KernelHits
	stats.KernelMisses = qSQLAlignedStats.KernelMisses
	stats.KernelEvictions = qSQLAlignedStats.KernelEvictions
	stats.KernelDecisionHits = qSQLAlignedStats.KernelDecisionHits
	stats.KernelDecisionMisses = qSQLAlignedStats.KernelDecisionMisses
	stats.KernelDecisionEvictions = qSQLAlignedStats.KernelDecisionEvictions
	stats.KernelKeys = qSQLKernelStatsByKeySnapshotLocked()
	qSQLAlignedPlanCacheMu.Unlock()

	return stats
}

func qCacheStatsTable() *Table {
	qSQLTemplateCacheMu.Lock()
	templateEntries := len(qSQLTemplateCache)
	templateStats := qSQLTemplateStats
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	alignedEntries := len(qSQLAlignedPlanCache) + len(qSQLAlignedMutationCache)
	kernelEntries := len(qSQLKernelCache)
	kernelDecisionEntries := len(qSQLKernelUnsupported)
	alignedStats := qSQLAlignedStats
	kernelStatsByKey := qSQLKernelStatsByKeySnapshotLocked()
	kernelShapeStats := qSQLKernelShapeStats(kernelStatsByKey)
	kernelDecisionKeyStats := qSQLKernelDecisionKeyStatsSnapshotLocked()
	kernelDecisionReasonStats := qSQLKernelDecisionReasonStatsLocked(kernelDecisionKeyStats)
	kernelDecisionShapeStats := qSQLKernelDecisionShapeStatsLocked(kernelDecisionKeyStats)
	qSQLAlignedPlanCacheMu.Unlock()

	qEvalCacheMu.Lock()
	evalEntries := len(qEvalCache)
	evalStats := qEvalStats
	qEvalCacheMu.Unlock()

	qQueryKernelSupportCacheMu.Lock()
	queryKernelEntries := len(qQueryKernelSupportCache)
	queryKernelStats := qQueryKernelSupportStats
	queryKernelKeyStats := qQueryKernelSupportKeyStatsSnapshotLocked()
	queryKernelShapeStats := qQueryKernelSupportCacheShapeStatsLocked()
	qQueryKernelSupportCacheMu.Unlock()

	rows := NewAppendArrayTable(8)
	rows.RawSetInt(1, TableValue(qCacheStatsRow(
		"qsql_template",
		templateEntries,
		templateStats.TemplateHits,
		templateStats.TemplateMisses,
		templateStats.TemplateEvictions,
		qSQLPlanCacheLimit,
	)))
	rows.RawSetInt(2, TableValue(qCacheStatsRow(
		"qsql_aligned",
		alignedEntries,
		alignedStats.AlignedHits,
		alignedStats.AlignedMisses,
		alignedStats.AlignedEvictions,
		qSQLPlanCacheLimit,
	)))
	kernelStatsRow := qCacheStatsRow(
		"qsql_kernel",
		kernelEntries,
		alignedStats.KernelHits,
		alignedStats.KernelMisses,
		alignedStats.KernelEvictions,
		qSQLPlanCacheLimit,
	)
	kernelStatsRow.RawSetString("keys", TableValue(qKernelCacheKeyStatsTable(kernelStatsByKey)))
	kernelStatsRow.RawSetString("shapes", TableValue(qKernelShapeStatsTable(kernelShapeStats)))
	rows.RawSetInt(3, TableValue(kernelStatsRow))
	kernelDecisionStatsRow := qCacheStatsRow(
		"qsql_kernel_decision",
		kernelDecisionEntries,
		alignedStats.KernelDecisionHits,
		alignedStats.KernelDecisionMisses,
		alignedStats.KernelDecisionEvictions,
		qSQLPlanCacheLimit,
	)
	kernelDecisionStatsRow.RawSetString("keys", TableValue(qKernelDecisionKeyStatsTable(kernelDecisionKeyStats)))
	kernelDecisionStatsRow.RawSetString("reasons", TableValue(qKernelDecisionReasonStatsTable(kernelDecisionReasonStats)))
	kernelDecisionStatsRow.RawSetString("shapes", TableValue(qKernelDecisionShapeStatsTable(kernelDecisionShapeStats)))
	rows.RawSetInt(4, TableValue(kernelDecisionStatsRow))
	queryKernelStatsRow := qCacheStatsRow(
		"q_query_kernel",
		queryKernelEntries,
		queryKernelStats.Hits,
		queryKernelStats.Misses,
		queryKernelStats.Evictions,
		qQueryKernelSupportCacheLimit,
	)
	queryKernelStatsRow.RawSetString("keys", TableValue(qQueryKernelSupportKeyStatsTable(queryKernelKeyStats)))
	queryKernelStatsRow.RawSetString("shapes", TableValue(qQueryKernelSupportShapeStatsTable(queryKernelShapeStats)))
	rows.RawSetInt(5, TableValue(queryKernelStatsRow))
	rows.RawSetInt(6, TableValue(qRuntimeKernelExecutionStatsRow()))
	rows.RawSetInt(7, TableValue(qRuntimeKernelLoweringStatsRow()))
	evalStatsRow := qCacheStatsRow(
		"q_eval",
		evalEntries,
		evalStats.Hits,
		evalStats.Misses,
		evalStats.Evictions,
		qEvalCacheLimit,
	)
	evalStatsRow.RawSetString("stats_domain", StringValue(qStatsDomainEvalCache))
	rows.RawSetInt(8, TableValue(evalStatsRow))
	return rows
}

func qCacheStatsRow(name string, entries, hits, misses, evictions, limit int) *Table {
	row := NewTable()
	row.RawSetString("cache", StringValue(name))
	row.RawSetString("stats_domain", StringValue(qStatsDomainSemanticCache))
	row.RawSetString("stats_source", StringValue(qStatsSourceQBind))
	row.RawSetString("cache_backed", BoolValue(true))
	row.RawSetString("entries", IntValue(int64(entries)))
	row.RawSetString("hits", IntValue(int64(hits)))
	row.RawSetString("misses", IntValue(int64(misses)))
	row.RawSetString("evictions", IntValue(int64(evictions)))
	row.RawSetString("limit", IntValue(int64(limit)))
	return row
}

func qRuntimeKernelExecutionStatsRow() *Table {
	stats := qRuntimeKernelExecutionStatsSnapshot()
	executions := uint64(0)
	successes := uint64(0)
	errors := uint64(0)
	for _, stat := range stats {
		executions += stat.Count
		switch stat.Outcome {
		case "success":
			successes += stat.Count
		case "error":
			errors += stat.Count
		}
	}
	row := qCacheStatsRow("q_runtime_kernel_execution", len(stats), 0, 0, 0, 0)
	row.RawSetString("stats_domain", StringValue(qStatsDomainJITExecution))
	row.RawSetString("stats_source", StringValue(qStatsSourceMethodJIT))
	row.RawSetString("cache_backed", BoolValue(false))
	row.RawSetString("executions", qUint64IntValue(executions))
	row.RawSetString("successes", qUint64IntValue(successes))
	row.RawSetString("errors", qUint64IntValue(errors))
	row.RawSetString("stats", TableValue(qRuntimeKernelExecutionStatsTable(stats)))
	row.RawSetString("shapes", TableValue(qRuntimeKernelExecutionShapeStatsTable(qRuntimeKernelExecutionShapeStats(stats))))
	row.RawSetString("kernels", TableValue(qRuntimeKernelExecutionKernelStatsTable(qRuntimeKernelExecutionKernelStats(stats))))
	row.RawSetString("routes", TableValue(qRuntimeKernelExecutionRouteStatsTable(qRuntimeKernelExecutionRouteStats(stats))))
	return row
}

func qRuntimeKernelExecutionStatsSnapshot() []QRuntimeKernelExecutionStat {
	qRuntimeKernelExecutionStatsProviderMu.Lock()
	state := qRuntimeKernelExecutionStatsNextActiveProvider(qRuntimeKernelExecutionStatsProviderCurrent)
	qRuntimeKernelExecutionStatsProviderCurrent = state
	var provider func() []QRuntimeKernelExecutionStat
	if state != nil {
		provider = state.provider
	}
	qRuntimeKernelExecutionStatsProviderMu.Unlock()
	if provider == nil {
		return nil
	}
	stats := provider()
	if len(stats) == 0 {
		return nil
	}
	type statKey struct {
		source  string
		kernel  string
		shape   string
		route   string
		outcome string
	}
	counts := make(map[statKey]uint64, len(stats))
	for _, stat := range stats {
		if stat.Count == 0 {
			continue
		}
		key := statKey{
			source:  qNormalizeRuntimeKernelStatPart(stat.Source),
			kernel:  qNormalizeRuntimeKernelStatPart(stat.Kernel),
			shape:   qNormalizeRuntimeKernelStatPart(stat.Shape),
			route:   qNormalizeRuntimeKernelStatPart(stat.Route),
			outcome: qNormalizeRuntimeKernelStatPart(stat.Outcome),
		}
		counts[key] += stat.Count
	}
	if len(counts) == 0 {
		return nil
	}
	out := make([]QRuntimeKernelExecutionStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, QRuntimeKernelExecutionStat{
			Source:  key.source,
			Kernel:  key.kernel,
			Shape:   key.shape,
			Route:   key.route,
			Outcome: key.outcome,
			Count:   count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		if a.Route != b.Route {
			return a.Route < b.Route
		}
		return a.Outcome < b.Outcome
	})
	return out
}

func qRuntimeKernelLoweringStatsRow() *Table {
	stats := qRuntimeKernelLoweringStatsSnapshot()
	lowerings := uint64(0)
	supported := uint64(0)
	fallbacks := uint64(0)
	for _, stat := range stats {
		lowerings += stat.Count
		switch stat.Outcome {
		case "supported":
			supported += stat.Count
		case "fallback":
			fallbacks += stat.Count
		}
	}
	row := qCacheStatsRow("q_runtime_kernel_lowering", len(stats), 0, 0, 0, 0)
	row.RawSetString("stats_domain", StringValue(qStatsDomainJITLowering))
	row.RawSetString("stats_source", StringValue(qStatsSourceMethodJIT))
	row.RawSetString("cache_backed", BoolValue(false))
	row.RawSetString("lowerings", qUint64IntValue(lowerings))
	row.RawSetString("supported", qUint64IntValue(supported))
	row.RawSetString("fallbacks", qUint64IntValue(fallbacks))
	row.RawSetString("stats", TableValue(qRuntimeKernelLoweringStatsTable(stats)))
	row.RawSetString("shapes", TableValue(qRuntimeKernelLoweringShapeStatsTable(qRuntimeKernelLoweringShapeStats(stats))))
	row.RawSetString("kernels", TableValue(qRuntimeKernelLoweringKernelStatsTable(qRuntimeKernelLoweringKernelStats(stats))))
	row.RawSetString("reasons", TableValue(qRuntimeKernelLoweringReasonStatsTable(qRuntimeKernelLoweringReasonStats(stats))))
	row.RawSetString("reason_shapes", TableValue(qRuntimeKernelLoweringReasonShapeStatsTable(qRuntimeKernelLoweringReasonShapeStats(stats))))
	row.RawSetString("routes", TableValue(qRuntimeKernelLoweringRouteStatsTable(qRuntimeKernelLoweringRouteStats(stats))))
	return row
}

func qRuntimeKernelLoweringStatsSnapshot() []QRuntimeKernelLoweringStat {
	qRuntimeKernelLoweringStatsProviderMu.Lock()
	state := qRuntimeKernelLoweringStatsNextActiveProvider(qRuntimeKernelLoweringStatsProviderCurrent)
	qRuntimeKernelLoweringStatsProviderCurrent = state
	var provider func() []QRuntimeKernelLoweringStat
	if state != nil {
		provider = state.provider
	}
	qRuntimeKernelLoweringStatsProviderMu.Unlock()
	if provider == nil {
		return nil
	}
	stats := provider()
	if len(stats) == 0 {
		return nil
	}
	type statKey struct {
		source       string
		kind         string
		kernel       string
		shape        string
		route        string
		outcome      string
		reasonFamily string
		reasonCode   string
	}
	counts := make(map[statKey]uint64, len(stats))
	for _, stat := range stats {
		if stat.Count == 0 {
			continue
		}
		kind := qNormalizeRuntimeKernelLoweringKind(stat.Kind)
		outcome := qNormalizeRuntimeKernelLoweringOutcome(stat.Outcome, kind)
		key := statKey{
			source:       qNormalizeRuntimeKernelStatPart(stat.Source),
			kind:         kind,
			kernel:       qNormalizeRuntimeKernelStatPart(stat.Kernel),
			shape:        qNormalizeRuntimeKernelStatPart(stat.Shape),
			route:        qNormalizeRuntimeKernelLoweringRoute(stat.Route, kind),
			outcome:      outcome,
			reasonFamily: qNormalizeRuntimeKernelLoweringReasonFamily(stat.ReasonFamily, outcome),
			reasonCode:   qNormalizeRuntimeKernelLoweringReasonCode(stat.ReasonCode, outcome),
		}
		counts[key] += stat.Count
	}
	if len(counts) == 0 {
		return nil
	}
	out := make([]QRuntimeKernelLoweringStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, QRuntimeKernelLoweringStat{
			Source:       key.source,
			Kind:         key.kind,
			Kernel:       key.kernel,
			Shape:        key.shape,
			Route:        key.route,
			Outcome:      key.outcome,
			ReasonFamily: key.reasonFamily,
			ReasonCode:   key.reasonCode,
			Count:        count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		if a.Route != b.Route {
			return a.Route < b.Route
		}
		if a.Outcome != b.Outcome {
			return a.Outcome < b.Outcome
		}
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

func qNormalizeRuntimeKernelLoweringKind(kind string) string {
	if strings.TrimSpace(kind) == "" {
		return "fallback"
	}
	return qNormalizeRuntimeKernelStatPart(kind)
}

func qNormalizeRuntimeKernelLoweringRoute(route, kind string) string {
	if strings.TrimSpace(route) == "" && kind == "fallback" {
		return "lowering"
	}
	return qNormalizeRuntimeKernelStatPart(route)
}

func qNormalizeRuntimeKernelLoweringOutcome(outcome, kind string) string {
	if strings.TrimSpace(outcome) == "" {
		if kind == "runtime_kernel" {
			return "supported"
		}
		return "fallback"
	}
	return qNormalizeRuntimeKernelStatPart(outcome)
}

func qNormalizeRuntimeKernelLoweringReasonFamily(reasonFamily, outcome string) string {
	if strings.TrimSpace(reasonFamily) == "" {
		if outcome == "supported" {
			return "supported"
		}
		return "lowering"
	}
	return qNormalizeRuntimeKernelStatPart(reasonFamily)
}

func qNormalizeRuntimeKernelLoweringReasonCode(reasonCode, outcome string) string {
	if strings.TrimSpace(reasonCode) == "" {
		if outcome == "supported" {
			return "supported"
		}
		return "unspecified"
	}
	return qNormalizeRuntimeKernelStatPart(reasonCode)
}

func qRuntimeKernelExecutionStatsTable(stats []QRuntimeKernelExecutionStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kernel", StringValue(stat.Kernel))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("route", StringValue(stat.Route))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelLoweringStatsTable(stats []QRuntimeKernelLoweringStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kind", StringValue(stat.Kind))
		row.RawSetString("kernel", StringValue(stat.Kernel))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("route", StringValue(stat.Route))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelExecutionSummaryForShape(shape string) qRuntimeKernelExecutionShapeSummary {
	shape = qNormalizeRuntimeKernelStatPart(shape)
	if shape == "" {
		return qRuntimeKernelExecutionShapeSummary{}
	}
	stats := qRuntimeKernelExecutionStatsSnapshot()
	var out qRuntimeKernelExecutionShapeSummary
	for _, stat := range stats {
		if stat.Shape != shape {
			continue
		}
		out.Executions += stat.Count
		switch stat.Outcome {
		case "success":
			out.Successes += stat.Count
		case "error":
			out.Errors += stat.Count
		}
	}
	out.Routes = qRuntimeKernelExecutionRouteStatsForShape(stats, shape)
	return out
}

func qExplainAttachRuntimeKernelExecutionSummary(out *Table, shape string) {
	summary := qRuntimeKernelExecutionSummaryForShape(shape)
	out.RawSetString("kernel_execution_count", qUint64IntValue(summary.Executions))
	out.RawSetString("kernel_execution_success_count", qUint64IntValue(summary.Successes))
	out.RawSetString("kernel_execution_error_count", qUint64IntValue(summary.Errors))
	out.RawSetString("kernel_execution_routes", TableValue(qRuntimeKernelExecutionRouteStatsTable(summary.Routes)))
}

func qRuntimeKernelLoweringSummaryForShape(shape string) qRuntimeKernelLoweringShapeSummary {
	if shape == "" {
		return qRuntimeKernelLoweringShapeSummary{}
	}
	shape = qNormalizeRuntimeKernelStatPart(shape)
	stats := qRuntimeKernelLoweringStatsSnapshot()
	var out qRuntimeKernelLoweringShapeSummary
	for _, stat := range stats {
		if stat.Shape != shape {
			continue
		}
		out.Lowerings += stat.Count
		switch stat.Outcome {
		case "supported":
			out.Supported += stat.Count
		case "fallback":
			out.Fallbacks += stat.Count
		}
	}
	out.Reasons = qRuntimeKernelLoweringReasonStatsForShape(stats, shape)
	out.ReasonShapes = qRuntimeKernelLoweringReasonShapeStatsForShape(stats, shape)
	out.Routes = qRuntimeKernelLoweringRouteStatsForShape(stats, shape)
	return out
}

func qExplainAttachRuntimeKernelLoweringSummary(out *Table, shape string) {
	summary := qRuntimeKernelLoweringSummaryForShape(shape)
	out.RawSetString("kernel_lowering_count", qUint64IntValue(summary.Lowerings))
	out.RawSetString("kernel_lowering_supported_count", qUint64IntValue(summary.Supported))
	out.RawSetString("kernel_lowering_fallback_count", qUint64IntValue(summary.Fallbacks))
	out.RawSetString("kernel_lowering_fallback_reasons", TableValue(qRuntimeKernelLoweringReasonStatsTable(summary.Reasons)))
	out.RawSetString("kernel_lowering_fallback_reason_shapes", TableValue(qRuntimeKernelLoweringReasonShapeStatsTable(summary.ReasonShapes)))
	out.RawSetString("kernel_lowering_routes", TableValue(qRuntimeKernelLoweringRouteStatsTable(summary.Routes)))
}

func qRuntimeKernelExecutionShapeStats(stats []QRuntimeKernelExecutionStat) []qRuntimeKernelExecutionShapeStat {
	type shapeKey struct {
		source  string
		shape   string
		outcome string
	}
	counts := make(map[shapeKey]uint64, len(stats))
	for _, stat := range stats {
		key := shapeKey{source: stat.Source, shape: stat.Shape, outcome: stat.Outcome}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelExecutionShapeStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelExecutionShapeStat{
			Source:  key.source,
			Shape:   key.shape,
			Outcome: key.outcome,
			Count:   count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		return a.Outcome < b.Outcome
	})
	return out
}

func qRuntimeKernelExecutionShapeStatsTable(stats []qRuntimeKernelExecutionShapeStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelExecutionKernelStats(stats []QRuntimeKernelExecutionStat) []qRuntimeKernelExecutionKernelStat {
	type kernelKey struct {
		source  string
		kernel  string
		outcome string
	}
	counts := make(map[kernelKey]uint64, len(stats))
	for _, stat := range stats {
		key := kernelKey{source: stat.Source, kernel: stat.Kernel, outcome: stat.Outcome}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelExecutionKernelStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelExecutionKernelStat{
			Source:  key.source,
			Kernel:  key.kernel,
			Outcome: key.outcome,
			Count:   count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		return a.Outcome < b.Outcome
	})
	return out
}

func qRuntimeKernelExecutionKernelStatsTable(stats []qRuntimeKernelExecutionKernelStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kernel", StringValue(stat.Kernel))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelExecutionRouteStats(stats []QRuntimeKernelExecutionStat) []qRuntimeKernelExecutionRouteStat {
	return qRuntimeKernelExecutionRouteStatsForShape(stats, "")
}

func qRuntimeKernelExecutionRouteStatsForShape(stats []QRuntimeKernelExecutionStat, shape string) []qRuntimeKernelExecutionRouteStat {
	type routeKey struct {
		source  string
		kernel  string
		route   string
		outcome string
	}
	counts := make(map[routeKey]uint64, len(stats))
	for _, stat := range stats {
		if shape != "" && stat.Shape != shape {
			continue
		}
		key := routeKey{source: stat.Source, kernel: stat.Kernel, route: stat.Route, outcome: stat.Outcome}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelExecutionRouteStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelExecutionRouteStat{
			Source:  key.source,
			Kernel:  key.kernel,
			Route:   key.route,
			Outcome: key.outcome,
			Count:   count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		if a.Route != b.Route {
			return a.Route < b.Route
		}
		return a.Outcome < b.Outcome
	})
	return out
}

func qRuntimeKernelExecutionRouteStatsTable(stats []qRuntimeKernelExecutionRouteStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kernel", StringValue(stat.Kernel))
		row.RawSetString("route", StringValue(stat.Route))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelLoweringShapeStats(stats []QRuntimeKernelLoweringStat) []qRuntimeKernelLoweringShapeStat {
	type shapeKey struct {
		source       string
		kind         string
		shape        string
		outcome      string
		reasonFamily string
		reasonCode   string
	}
	counts := make(map[shapeKey]uint64, len(stats))
	for _, stat := range stats {
		key := shapeKey{
			source:       stat.Source,
			kind:         stat.Kind,
			shape:        stat.Shape,
			outcome:      stat.Outcome,
			reasonFamily: stat.ReasonFamily,
			reasonCode:   stat.ReasonCode,
		}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelLoweringShapeStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelLoweringShapeStat{
			Source:       key.source,
			Kind:         key.kind,
			Shape:        key.shape,
			Outcome:      key.outcome,
			ReasonFamily: key.reasonFamily,
			ReasonCode:   key.reasonCode,
			Count:        count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		if a.Outcome != b.Outcome {
			return a.Outcome < b.Outcome
		}
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

func qRuntimeKernelLoweringShapeStatsTable(stats []qRuntimeKernelLoweringShapeStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kind", StringValue(stat.Kind))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelLoweringKernelStats(stats []QRuntimeKernelLoweringStat) []qRuntimeKernelLoweringKernelStat {
	type kernelKey struct {
		source       string
		kind         string
		kernel       string
		outcome      string
		reasonFamily string
		reasonCode   string
	}
	counts := make(map[kernelKey]uint64, len(stats))
	for _, stat := range stats {
		key := kernelKey{
			source:       stat.Source,
			kind:         stat.Kind,
			kernel:       stat.Kernel,
			outcome:      stat.Outcome,
			reasonFamily: stat.ReasonFamily,
			reasonCode:   stat.ReasonCode,
		}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelLoweringKernelStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelLoweringKernelStat{
			Source:       key.source,
			Kind:         key.kind,
			Kernel:       key.kernel,
			Outcome:      key.outcome,
			ReasonFamily: key.reasonFamily,
			ReasonCode:   key.reasonCode,
			Count:        count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		if a.Outcome != b.Outcome {
			return a.Outcome < b.Outcome
		}
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

func qRuntimeKernelLoweringKernelStatsTable(stats []qRuntimeKernelLoweringKernelStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kind", StringValue(stat.Kind))
		row.RawSetString("kernel", StringValue(stat.Kernel))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelLoweringReasonStats(stats []QRuntimeKernelLoweringStat) []qRuntimeKernelLoweringReasonStat {
	return qRuntimeKernelLoweringReasonStatsForShape(stats, "")
}

func qRuntimeKernelLoweringReasonStatsForShape(stats []QRuntimeKernelLoweringStat, shape string) []qRuntimeKernelLoweringReasonStat {
	type reasonKey struct {
		source       string
		kind         string
		reasonFamily string
		reasonCode   string
	}
	counts := make(map[reasonKey]uint64, len(stats))
	for _, stat := range stats {
		if shape != "" && stat.Shape != shape {
			continue
		}
		if stat.Outcome != "fallback" {
			continue
		}
		key := reasonKey{
			source:       stat.Source,
			kind:         stat.Kind,
			reasonFamily: stat.ReasonFamily,
			reasonCode:   stat.ReasonCode,
		}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelLoweringReasonStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelLoweringReasonStat{
			Source:       key.source,
			Kind:         key.kind,
			ReasonFamily: key.reasonFamily,
			ReasonCode:   key.reasonCode,
			Count:        count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

func qRuntimeKernelLoweringReasonStatsTable(stats []qRuntimeKernelLoweringReasonStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kind", StringValue(stat.Kind))
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelLoweringReasonShapeStats(stats []QRuntimeKernelLoweringStat) []qRuntimeKernelLoweringReasonShapeStat {
	return qRuntimeKernelLoweringReasonShapeStatsForShape(stats, "")
}

func qRuntimeKernelLoweringReasonShapeStatsForShape(stats []QRuntimeKernelLoweringStat, shape string) []qRuntimeKernelLoweringReasonShapeStat {
	type reasonShapeKey struct {
		source       string
		kind         string
		kernel       string
		shape        string
		route        string
		outcome      string
		reasonFamily string
		reasonCode   string
	}
	counts := make(map[reasonShapeKey]uint64, len(stats))
	for _, stat := range stats {
		if shape != "" && stat.Shape != shape {
			continue
		}
		if stat.Outcome != "fallback" {
			continue
		}
		key := reasonShapeKey{
			source:       stat.Source,
			kind:         stat.Kind,
			kernel:       stat.Kernel,
			shape:        stat.Shape,
			route:        stat.Route,
			outcome:      stat.Outcome,
			reasonFamily: stat.ReasonFamily,
			reasonCode:   stat.ReasonCode,
		}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelLoweringReasonShapeStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelLoweringReasonShapeStat{
			Source:       key.source,
			Kind:         key.kind,
			Kernel:       key.kernel,
			Shape:        key.shape,
			Route:        key.route,
			Outcome:      key.outcome,
			ReasonFamily: key.reasonFamily,
			ReasonCode:   key.reasonCode,
			Count:        count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		if a.Route != b.Route {
			return a.Route < b.Route
		}
		if a.Outcome != b.Outcome {
			return a.Outcome < b.Outcome
		}
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

func qRuntimeKernelLoweringReasonShapeStatsTable(stats []qRuntimeKernelLoweringReasonShapeStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kind", StringValue(stat.Kind))
		row.RawSetString("kernel", StringValue(stat.Kernel))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("route", StringValue(stat.Route))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRuntimeKernelLoweringRouteStats(stats []QRuntimeKernelLoweringStat) []qRuntimeKernelLoweringRouteStat {
	return qRuntimeKernelLoweringRouteStatsForShape(stats, "")
}

func qRuntimeKernelLoweringRouteStatsForShape(stats []QRuntimeKernelLoweringStat, shape string) []qRuntimeKernelLoweringRouteStat {
	type routeKey struct {
		source  string
		kind    string
		kernel  string
		route   string
		outcome string
	}
	counts := make(map[routeKey]uint64, len(stats))
	for _, stat := range stats {
		if shape != "" && stat.Shape != shape {
			continue
		}
		key := routeKey{
			source:  stat.Source,
			kind:    stat.Kind,
			kernel:  stat.Kernel,
			route:   stat.Route,
			outcome: stat.Outcome,
		}
		counts[key] += stat.Count
	}
	out := make([]qRuntimeKernelLoweringRouteStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, qRuntimeKernelLoweringRouteStat{
			Source:  key.source,
			Kind:    key.kind,
			Kernel:  key.kernel,
			Route:   key.route,
			Outcome: key.outcome,
			Count:   count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		if a.Route != b.Route {
			return a.Route < b.Route
		}
		return a.Outcome < b.Outcome
	})
	return out
}

func qRuntimeKernelLoweringRouteStatsTable(stats []qRuntimeKernelLoweringRouteStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("source", StringValue(stat.Source))
		row.RawSetString("kind", StringValue(stat.Kind))
		row.RawSetString("kernel", StringValue(stat.Kernel))
		row.RawSetString("route", StringValue(stat.Route))
		row.RawSetString("outcome", StringValue(stat.Outcome))
		row.RawSetString("count", qUint64IntValue(stat.Count))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qNormalizeRuntimeKernelStatPart(part string) string {
	if part == "" {
		return "unknown"
	}
	return part
}

func qUint64IntValue(n uint64) Value {
	const maxInt64 = uint64(1<<63 - 1)
	if n > maxInt64 {
		return IntValue(int64(maxInt64))
	}
	return IntValue(int64(n))
}

func qQueryKernelSupportCacheShapeStatsLocked() []qQueryKernelSupportShapeStat {
	counts := make(map[qQueryKernelSupportShapeKey]int, len(qQueryKernelSupportCache))
	for _, entry := range qQueryKernelSupportCache {
		reasonCode := entry.ReasonCode
		if reasonCode == "" {
			reasonCode = qNormalizeQueryKernelFallbackReasonCode(entry.Reason)
		}
		if reasonCode == "" && entry.Supported {
			reasonCode = qKernelReasonSupported
		}
		family := qFallbackReasonFamilyForDetail(qFallbackQueryKernel, reasonCode, entry.Reason)
		if entry.Supported {
			family = qFallbackFamilySupported
		}
		shape := entry.Shape
		if shape == "" {
			shape = "unknown"
		}
		counts[qQueryKernelSupportShapeKey{
			Supported:    entry.Supported,
			ReasonFamily: family,
			ReasonCode:   reasonCode,
			SchemaHash:   entry.SchemaHash,
			Shape:        shape,
		}]++
	}
	stats := make([]qQueryKernelSupportShapeStat, 0, len(counts))
	for key, count := range counts {
		stats = append(stats, qQueryKernelSupportShapeStat{Key: key, Count: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		a, b := stats[i], stats[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.Supported != b.Key.Supported {
			return a.Key.Supported
		}
		if a.Key.ReasonFamily != b.Key.ReasonFamily {
			return a.Key.ReasonFamily < b.Key.ReasonFamily
		}
		if a.Key.ReasonCode != b.Key.ReasonCode {
			return a.Key.ReasonCode < b.Key.ReasonCode
		}
		if a.Key.SchemaHash != b.Key.SchemaHash {
			return a.Key.SchemaHash < b.Key.SchemaHash
		}
		return a.Key.Shape < b.Key.Shape
	})
	return stats
}

func qQueryKernelSupportShapeStatsTable(stats []qQueryKernelSupportShapeStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("supported", BoolValue(stat.Key.Supported))
		row.RawSetString("reason_family", StringValue(stat.Key.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.Key.ReasonCode))
		row.RawSetString("schema_hash", StringValue(stat.Key.SchemaHash))
		row.RawSetString("shape", StringValue(stat.Key.Shape))
		row.RawSetString("count", IntValue(int64(stat.Count)))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qQueryKernelSupportStatsForKeyLocked(key string) *qQueryKernelSupportCacheKeyStat {
	stats := qQueryKernelSupportStatsByKey[key]
	if stats == nil {
		stats = &qQueryKernelSupportCacheKeyStat{
			Key:        key,
			Namespace:  "q.query",
			Kind:       "query_kernel",
			SchemaHash: qQueryKernelSchemaHashFromCacheKey(key),
			Shape:      qQueryKernelShapeFromCacheKey(key),
		}
		qQueryKernelSupportStatsByKey[key] = stats
	}
	return stats
}

func (stat *qQueryKernelSupportCacheKeyStat) setEntry(entry qQueryKernelSupportCacheEntry) {
	if stat == nil {
		return
	}
	stat.Supported = entry.Supported
	reasonCode := entry.ReasonCode
	if reasonCode == "" {
		reasonCode = qNormalizeQueryKernelFallbackReasonCode(entry.Reason)
	}
	if reasonCode == "" && entry.Supported {
		reasonCode = qKernelReasonSupported
	}
	stat.ReasonCode = reasonCode
	stat.ReasonFamily = qFallbackReasonFamilyForDetail(qFallbackQueryKernel, reasonCode, entry.Reason)
	if entry.Supported {
		stat.ReasonFamily = qFallbackFamilySupported
	}
	if entry.SchemaHash != "" {
		stat.SchemaHash = entry.SchemaHash
	}
	if entry.Shape != "" {
		stat.Shape = entry.Shape
	}
}

func qQueryKernelSupportKeyStatsSnapshotLocked() []qQueryKernelSupportCacheKeyStat {
	out := make([]qQueryKernelSupportCacheKeyStat, 0, len(qQueryKernelSupportStatsByKey))
	for key, stat := range qQueryKernelSupportStatsByKey {
		if _, ok := qQueryKernelSupportCache[key]; !ok {
			continue
		}
		if stat == nil {
			continue
		}
		row := *stat
		if row.SchemaHash == "" {
			row.SchemaHash = qQueryKernelSchemaHashFromCacheKey(row.Key)
		}
		if row.Shape == "" {
			row.Shape = qQueryKernelShapeFromCacheKey(row.Key)
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Supported != b.Supported {
			return a.Supported
		}
		if a.SchemaHash != b.SchemaHash {
			return a.SchemaHash < b.SchemaHash
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		return a.Key < b.Key
	})
	return out
}

// QQueryKernelSupportKeyStatJSONRows converts q.query native-kernel support
// cache key stats to a schema-stable JSON row shape.
func QQueryKernelSupportKeyStatJSONRows(stats []qQueryKernelSupportCacheKeyStat) []QQueryKernelSupportKeyStatJSONRow {
	if len(stats) == 0 {
		return nil
	}
	out := make([]QQueryKernelSupportKeyStatJSONRow, 0, len(stats))
	for _, stat := range stats {
		out = append(out, QQueryKernelSupportKeyStatJSONRow{
			Key:             stat.Key,
			Namespace:       stat.Namespace,
			Kind:            stat.Kind,
			PlanFingerprint: "",
			Supported:       stat.Supported,
			ReasonFamily:    stat.ReasonFamily,
			ReasonCode:      stat.ReasonCode,
			SchemaHash:      stat.SchemaHash,
			Shape:           stat.Shape,
			Hits:            stat.Hits,
			Misses:          stat.Misses,
			Evictions:       stat.Evictions,
		})
	}
	return out
}

func qQueryKernelSupportKeyStatsTable(stats []qQueryKernelSupportCacheKeyStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("key", StringValue(stat.Key))
		row.RawSetString("namespace", StringValue(stat.Namespace))
		row.RawSetString("kind", StringValue(stat.Kind))
		row.RawSetString("plan_fingerprint", StringValue(""))
		row.RawSetString("supported", BoolValue(stat.Supported))
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("schema_hash", StringValue(stat.SchemaHash))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("hits", IntValue(int64(stat.Hits)))
		row.RawSetString("misses", IntValue(int64(stat.Misses)))
		row.RawSetString("evictions", IntValue(int64(stat.Evictions)))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qSQLKernelShapeStats(stats []qSQLKernelCacheKeyStats) []qSQLKernelShapeStat {
	if len(stats) == 0 {
		return nil
	}
	type shapeSchemaKey struct {
		shape      string
		schemaHash string
	}
	counts := make(map[shapeSchemaKey]*qSQLKernelShapeStat)
	for _, stat := range stats {
		shape := stat.Shape
		if shape == "" {
			shape = "unknown"
		}
		info := qSQLKernelCacheKeyInfo(stat.Key)
		schemaHash := info.SchemaHash
		if schemaHash == "" {
			schemaHash = "unknown"
		}
		key := shapeSchemaKey{shape: shape, schemaHash: schemaHash}
		shapeStat := counts[key]
		if shapeStat == nil {
			shapeStat = &qSQLKernelShapeStat{Shape: shape, SchemaHash: schemaHash}
			counts[key] = shapeStat
		}
		shapeStat.Count++
		shapeStat.Hits += stat.Hits
		shapeStat.Misses += stat.Misses
		shapeStat.Evictions += stat.Evictions
	}
	out := make([]qSQLKernelShapeStat, 0, len(counts))
	for _, stat := range counts {
		out = append(out, *stat)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Hits != b.Hits {
			return a.Hits > b.Hits
		}
		if a.Misses != b.Misses {
			return a.Misses > b.Misses
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		return a.SchemaHash < b.SchemaHash
	})
	return out
}

func qKernelShapeStatsTable(stats []qSQLKernelShapeStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("schema_hash", StringValue(stat.SchemaHash))
		row.RawSetString("count", IntValue(int64(stat.Count)))
		row.RawSetString("hits", IntValue(int64(stat.Hits)))
		row.RawSetString("misses", IntValue(int64(stat.Misses)))
		row.RawSetString("evictions", IntValue(int64(stat.Evictions)))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

// QSQLKernelCacheKeyStatJSONRows converts qSQL semantic kernel cache key stats
// to a schema-stable JSON row shape.
func QSQLKernelCacheKeyStatJSONRows(stats []qSQLKernelCacheKeyStats) []QSQLKernelCacheKeyStatJSONRow {
	if len(stats) == 0 {
		return nil
	}
	out := make([]QSQLKernelCacheKeyStatJSONRow, 0, len(stats))
	for _, stat := range stats {
		info := qSQLKernelCacheKeyInfo(stat.Key)
		out = append(out, QSQLKernelCacheKeyStatJSONRow{
			Key:             stat.Key,
			Namespace:       info.Namespace,
			Kind:            info.Kind,
			SchemaHash:      info.SchemaHash,
			PlanFingerprint: qSQLKernelCacheKeyPlanFingerprint(info),
			Shape:           stat.Shape,
			Hits:            stat.Hits,
			Misses:          stat.Misses,
			Evictions:       stat.Evictions,
		})
	}
	return out
}

func qKernelCacheKeyStatsTable(stats []qSQLKernelCacheKeyStats) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("key", StringValue(stat.Key))
		info := qSQLKernelCacheKeyInfo(stat.Key)
		row.RawSetString("namespace", StringValue(info.Namespace))
		row.RawSetString("kind", StringValue(info.Kind))
		row.RawSetString("schema_hash", StringValue(info.SchemaHash))
		row.RawSetString("plan_fingerprint", StringValue(qSQLKernelCacheKeyPlanFingerprint(info)))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("hits", IntValue(int64(stat.Hits)))
		row.RawSetString("misses", IntValue(int64(stat.Misses)))
		row.RawSetString("evictions", IntValue(int64(stat.Evictions)))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qSQLKernelCacheKeyPlanFingerprint(info data.SchemaStableCacheKeyParts) string {
	if len(info.Extra) == 0 {
		return ""
	}
	return info.Extra[0]
}

func qSQLKernelDecisionKeyStatsSnapshotLocked() []qSQLKernelDecisionKeyStat {
	out := make([]qSQLKernelDecisionKeyStat, 0, len(qSQLKernelUnsupported))
	for key, reason := range qSQLKernelUnsupported {
		reasonCode := stdq.KernelFallbackReasonCode(reason)
		shape := qSQLKernelUnsupportedShape[key]
		if shape == "" {
			shape = "unknown"
		}
		out = append(out, qSQLKernelDecisionKeyStat{
			Key:          key,
			Shape:        shape,
			ReasonCode:   reasonCode,
			ReasonFamily: qFallbackReasonFamilyForDetail(qFallbackKernelUnsupported, reasonCode, reason),
			Count:        1,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		if a.ReasonCode != b.ReasonCode {
			return a.ReasonCode < b.ReasonCode
		}
		return a.Key < b.Key
	})
	return out
}

func qSQLKernelDecisionShapeStatsLocked(keys []qSQLKernelDecisionKeyStat) []qSQLKernelDecisionShapeStat {
	counts := make(map[qSQLKernelDecisionShapeStat]int, len(keys))
	for _, key := range keys {
		shape := key.Shape
		if shape == "" {
			shape = "unknown"
		}
		info := qSQLKernelCacheKeyInfo(key.Key)
		counts[qSQLKernelDecisionShapeStat{
			Shape:        shape,
			SchemaHash:   info.SchemaHash,
			ReasonCode:   key.ReasonCode,
			ReasonFamily: key.ReasonFamily,
		}] += key.Count
	}
	out := make([]qSQLKernelDecisionShapeStat, 0, len(counts))
	for stat, count := range counts {
		stat.Count = count
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		if a.ReasonCode != b.ReasonCode {
			return a.ReasonCode < b.ReasonCode
		}
		if a.SchemaHash != b.SchemaHash {
			return a.SchemaHash < b.SchemaHash
		}
		return a.Shape < b.Shape
	})
	return out
}

func qSQLKernelDecisionReasonStatsLocked(keys []qSQLKernelDecisionKeyStat) []qSQLKernelDecisionReasonStat {
	counts := make(map[qSQLKernelDecisionReasonStat]int, len(keys))
	for _, key := range keys {
		counts[qSQLKernelDecisionReasonStat{
			ReasonCode:   key.ReasonCode,
			ReasonFamily: key.ReasonFamily,
		}] += key.Count
	}
	out := make([]qSQLKernelDecisionReasonStat, 0, len(counts))
	for stat, count := range counts {
		stat.Count = count
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.ReasonFamily != b.ReasonFamily {
			return a.ReasonFamily < b.ReasonFamily
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

func qKernelDecisionKeyStatsTable(stats []qSQLKernelDecisionKeyStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("key", StringValue(stat.Key))
		info := qSQLKernelCacheKeyInfo(stat.Key)
		row.RawSetString("namespace", StringValue(info.Namespace))
		row.RawSetString("kind", StringValue(info.Kind))
		row.RawSetString("schema_hash", StringValue(info.SchemaHash))
		row.RawSetString("plan_fingerprint", StringValue(qSQLKernelCacheKeyPlanFingerprint(info)))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("count", IntValue(int64(stat.Count)))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

// QSQLKernelDecisionKeyStatJSONRows converts unsupported qSQL kernel decision
// key stats to a schema-stable JSON row shape.
func QSQLKernelDecisionKeyStatJSONRows(stats []qSQLKernelDecisionKeyStat) []QSQLKernelDecisionKeyStatJSONRow {
	if len(stats) == 0 {
		return nil
	}
	out := make([]QSQLKernelDecisionKeyStatJSONRow, 0, len(stats))
	for _, stat := range stats {
		info := qSQLKernelCacheKeyInfo(stat.Key)
		out = append(out, QSQLKernelDecisionKeyStatJSONRow{
			Key:             stat.Key,
			Namespace:       info.Namespace,
			Kind:            info.Kind,
			SchemaHash:      info.SchemaHash,
			PlanFingerprint: qSQLKernelCacheKeyPlanFingerprint(info),
			Shape:           stat.Shape,
			ReasonFamily:    stat.ReasonFamily,
			ReasonCode:      stat.ReasonCode,
			Count:           stat.Count,
		})
	}
	return out
}

func qKernelDecisionReasonStatsTable(stats []qSQLKernelDecisionReasonStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("count", IntValue(int64(stat.Count)))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qKernelDecisionShapeStatsTable(stats []qSQLKernelDecisionShapeStat) *Table {
	rows := NewAppendArrayTable(len(stats))
	for i, stat := range stats {
		row := NewTable()
		row.RawSetString("reason_family", StringValue(stat.ReasonFamily))
		row.RawSetString("reason_code", StringValue(stat.ReasonCode))
		row.RawSetString("shape", StringValue(stat.Shape))
		row.RawSetString("schema_hash", StringValue(stat.SchemaHash))
		row.RawSetString("count", IntValue(int64(stat.Count)))
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows
}

func qRecordFallback(code string) {
	qRecordFallbackReason(code, "", "")
}

func qRecordQueryKernelHit() {
	qFallbackStatsMu.Lock()
	qFallbackCounters.QueryKernelHit++
	qFallbackStatsMu.Unlock()
}

func qRecordFallbackReason(code, reasonCode, reason string) {
	qRecordFallbackReasonAttribution(code, reasonCode, reason, "", "", "")
}

func qRecordFallbackReasonAttribution(code, reasonCode, reason, source, schemaHash, shape string) {
	qFallbackStatsMu.Lock()
	if qFallbackCounters.ByReasonCode == nil {
		qFallbackCounters.ByReasonCode = make(map[qFallbackReasonCodeKey]int)
	}
	if qFallbackCounters.ByReason == nil {
		qFallbackCounters.ByReason = make(map[qFallbackReasonKey]int)
	}
	if qFallbackCounters.ByAttribution == nil {
		qFallbackCounters.ByAttribution = make(map[qFallbackAttributionKey]int)
	}
	switch code {
	case qFallbackKernelUnsupported:
		qFallbackCounters.KernelUnsupported++
	case qFallbackKernelCompileErr:
		qFallbackCounters.KernelCompileErr++
	case qFallbackSourceErr:
		qFallbackCounters.SourceErr++
	case qFallbackJoinErr:
		qFallbackCounters.JoinErr++
	case qFallbackMutationPlan:
		qFallbackCounters.Mutation++
	case qFallbackQueryKernel:
		qFallbackCounters.QueryKernel++
	}
	reason = qNormalizeFallbackReason(reason)
	reasonCode = qNormalizeFallbackReasonCode(code, reasonCode, reason)
	if reasonCode != "" {
		qFallbackCounters.ByReasonCode[qFallbackReasonCodeKey{Code: code, ReasonCode: reasonCode}]++
	}
	if reason != "" {
		qFallbackCounters.ByReason[qFallbackReasonKey{Code: code, Reason: reason}]++
	}
	if source != "" || schemaHash != "" || shape != "" {
		family := qFallbackReasonFamilyForDetail(code, reasonCode, reason)
		qFallbackCounters.ByAttribution[qFallbackAttributionKey{
			Code:         code,
			ReasonCode:   reasonCode,
			ReasonFamily: family,
			Source:       source,
			SchemaHash:   schemaHash,
			Shape:        shape,
		}]++
	}
	qFallbackStatsMu.Unlock()
}

func qExplainAttachFallbackStats(out *Table, code, reasonCode, reason string) {
	if out == nil {
		return
	}
	reasonCodeCount, reasonCount := qFallbackReasonCountsSnapshot(code, reasonCode, reason)
	out.RawSetString("kernel_fallback_code", StringValue(code))
	out.RawSetString("kernel_fallback_family", StringValue(qFallbackReasonFamilyForDetail(code, reasonCode, reason)))
	out.RawSetString("kernel_fallback_reason_code_count", IntValue(int64(reasonCodeCount)))
	out.RawSetString("kernel_fallback_reason_count", IntValue(int64(reasonCount)))
}

func qFallbackReasonCountsSnapshot(code, reasonCode, reason string) (reasonCodeCount, reasonCount int) {
	if code == "" {
		return 0, 0
	}
	reason = qNormalizeFallbackReason(reason)
	reasonCode = qNormalizeFallbackReasonCode(code, reasonCode, reason)
	qFallbackStatsMu.Lock()
	if reasonCode != "" && qFallbackCounters.ByReasonCode != nil {
		reasonCodeCount = qFallbackCounters.ByReasonCode[qFallbackReasonCodeKey{Code: code, ReasonCode: reasonCode}]
	}
	if reason != "" && qFallbackCounters.ByReason != nil {
		reasonCount = qFallbackCounters.ByReason[qFallbackReasonKey{Code: code, Reason: reason}]
	}
	qFallbackStatsMu.Unlock()
	return reasonCodeCount, reasonCount
}

func qSQLKernelFallbackStatsCode(info qExplainKernelResult) string {
	if info.supported {
		return ""
	}
	switch info.reasonCode {
	case qKernelReasonMutationPlan:
		return qFallbackMutationPlan
	case qKernelReasonSourceUnavailable:
		return qFallbackSourceErr
	case qKernelReasonJoinUnavailable:
		return qFallbackJoinErr
	case qKernelReasonCompileError:
		return qFallbackKernelCompileErr
	default:
		return qFallbackKernelUnsupported
	}
}

func qFallbackStatsTable() *Table {
	qFallbackStatsMu.Lock()
	stats := qCloneFallbackStatsLocked()
	qFallbackStatsMu.Unlock()

	familyRows := qFallbackFamilyRows(stats)
	detailRows := qFallbackTopRows(stats, 10)
	attributionRows := qFallbackAttributionRows(stats, 10)
	rows := NewAppendArrayTable(7 + len(familyRows) + len(detailRows) + len(attributionRows))
	rows.RawSetInt(1, TableValue(qFallbackStatsRow(qFallbackKernelUnsupported, stats.KernelUnsupported)))
	rows.RawSetInt(2, TableValue(qFallbackStatsRow(qFallbackKernelCompileErr, stats.KernelCompileErr)))
	rows.RawSetInt(3, TableValue(qFallbackStatsRow(qFallbackSourceErr, stats.SourceErr)))
	rows.RawSetInt(4, TableValue(qFallbackStatsRow(qFallbackJoinErr, stats.JoinErr)))
	rows.RawSetInt(5, TableValue(qFallbackStatsRow(qFallbackMutationPlan, stats.Mutation)))
	rows.RawSetInt(6, TableValue(qFallbackStatsRow(qQueryKernelSupported, stats.QueryKernelHit)))
	rows.RawSetInt(7, TableValue(qFallbackStatsRow(qFallbackQueryKernel, stats.QueryKernel)))
	next := int64(8)
	for _, row := range familyRows {
		rows.RawSetInt(next, TableValue(row))
		next++
	}
	for _, row := range detailRows {
		rows.RawSetInt(next, TableValue(row))
		next++
	}
	for _, row := range attributionRows {
		rows.RawSetInt(next, TableValue(row))
		next++
	}
	return rows
}

func qFallbackStatsRow(code string, count int) *Table {
	row := NewTable()
	row.RawSetString("kind", StringValue("code"))
	row.RawSetString("code", StringValue(code))
	row.RawSetString("reason_family", StringValue(qFallbackReasonFamily(code, "")))
	row.RawSetString("reason_code", StringValue(""))
	row.RawSetString("reason", StringValue(""))
	row.RawSetString("source", StringValue(""))
	row.RawSetString("schema_hash", StringValue(""))
	row.RawSetString("shape", StringValue(""))
	row.RawSetString("count", IntValue(int64(count)))
	return row
}

func qCloneFallbackStatsLocked() qFallbackStats {
	stats := qFallbackCounters
	if len(qFallbackCounters.ByReasonCode) > 0 {
		stats.ByReasonCode = make(map[qFallbackReasonCodeKey]int, len(qFallbackCounters.ByReasonCode))
		for key, count := range qFallbackCounters.ByReasonCode {
			stats.ByReasonCode[key] = count
		}
	}
	if len(qFallbackCounters.ByReason) > 0 {
		stats.ByReason = make(map[qFallbackReasonKey]int, len(qFallbackCounters.ByReason))
		for key, count := range qFallbackCounters.ByReason {
			stats.ByReason[key] = count
		}
	}
	if len(qFallbackCounters.ByAttribution) > 0 {
		stats.ByAttribution = make(map[qFallbackAttributionKey]int, len(qFallbackCounters.ByAttribution))
		for key, count := range qFallbackCounters.ByAttribution {
			stats.ByAttribution[key] = count
		}
	}
	return stats
}

func qFallbackFamilyRows(stats qFallbackStats) []*Table {
	counts := make(map[string]int, len(stats.ByReasonCode)+1)
	for key, count := range stats.ByReasonCode {
		family := qFallbackReasonFamilyForDetail(key.Code, key.ReasonCode, "")
		if family == "" {
			family = qFallbackFamilyKernel
		}
		counts[family] += count
	}
	if stats.QueryKernelHit > 0 {
		counts[qFallbackFamilySupported] += stats.QueryKernelHit
	}
	families := make([]string, 0, len(counts))
	for family, count := range counts {
		if count > 0 {
			families = append(families, family)
		}
	}
	sort.Slice(families, func(i, j int) bool {
		a, b := families[i], families[j]
		if counts[a] != counts[b] {
			return counts[a] > counts[b]
		}
		return a < b
	})
	rows := make([]*Table, 0, len(families))
	for _, family := range families {
		rows = append(rows, qFallbackFamilyStatsRow(family, counts[family]))
	}
	return rows
}

func qFallbackFamilyStatsRow(family string, count int) *Table {
	row := NewTable()
	row.RawSetString("kind", StringValue("reason_family"))
	row.RawSetString("code", StringValue(""))
	row.RawSetString("reason_family", StringValue(family))
	row.RawSetString("reason_code", StringValue(""))
	row.RawSetString("reason", StringValue(""))
	row.RawSetString("source", StringValue(""))
	row.RawSetString("schema_hash", StringValue(""))
	row.RawSetString("shape", StringValue(""))
	row.RawSetString("count", IntValue(int64(count)))
	return row
}

func qFallbackTopRows(stats qFallbackStats, limit int) []*Table {
	rows := make([]*Table, 0, len(stats.ByReasonCode)+len(stats.ByReason))
	reasonCodeRows := make([]qFallbackReasonCodeRow, 0, len(stats.ByReasonCode))
	for key, count := range stats.ByReasonCode {
		reasonCodeRows = append(reasonCodeRows, qFallbackReasonCodeRow{Key: key, Count: count})
	}
	sort.Slice(reasonCodeRows, func(i, j int) bool {
		a, b := reasonCodeRows[i], reasonCodeRows[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.Code != b.Key.Code {
			return a.Key.Code < b.Key.Code
		}
		return a.Key.ReasonCode < b.Key.ReasonCode
	})
	for i, item := range reasonCodeRows {
		if i >= limit {
			break
		}
		rows = append(rows, qFallbackDetailStatsRow("reason_code", item.Key.Code, item.Key.ReasonCode, "", item.Count))
	}

	reasonRows := make([]qFallbackReasonRow, 0, len(stats.ByReason))
	for key, count := range stats.ByReason {
		reasonRows = append(reasonRows, qFallbackReasonRow{Key: key, Count: count})
	}
	sort.Slice(reasonRows, func(i, j int) bool {
		a, b := reasonRows[i], reasonRows[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.Code != b.Key.Code {
			return a.Key.Code < b.Key.Code
		}
		return a.Key.Reason < b.Key.Reason
	})
	for i, item := range reasonRows {
		if i >= limit {
			break
		}
		rows = append(rows, qFallbackDetailStatsRow("reason", item.Key.Code, "", item.Key.Reason, item.Count))
	}
	return rows
}

func qFallbackAttributionRows(stats qFallbackStats, limit int) []*Table {
	rows := make([]qFallbackAttributionRow, 0, len(stats.ByAttribution))
	for key, count := range stats.ByAttribution {
		rows = append(rows, qFallbackAttributionRow{Key: key, Count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.Code != b.Key.Code {
			return a.Key.Code < b.Key.Code
		}
		if a.Key.ReasonFamily != b.Key.ReasonFamily {
			return a.Key.ReasonFamily < b.Key.ReasonFamily
		}
		if a.Key.ReasonCode != b.Key.ReasonCode {
			return a.Key.ReasonCode < b.Key.ReasonCode
		}
		if a.Key.Source != b.Key.Source {
			return a.Key.Source < b.Key.Source
		}
		if a.Key.SchemaHash != b.Key.SchemaHash {
			return a.Key.SchemaHash < b.Key.SchemaHash
		}
		return a.Key.Shape < b.Key.Shape
	})
	out := make([]*Table, 0, len(rows))
	for i, item := range rows {
		if i >= limit {
			break
		}
		out = append(out, qFallbackAttributionStatsRow(item.Key, item.Count))
	}
	return out
}

type qFallbackAttributionRow struct {
	Key   qFallbackAttributionKey
	Count int
}

type qFallbackReasonCodeRow struct {
	Key   qFallbackReasonCodeKey
	Count int
}

type qFallbackReasonRow struct {
	Key   qFallbackReasonKey
	Count int
}

func qFallbackDetailStatsRow(kind, code, reasonCode, reason string, count int) *Table {
	row := NewTable()
	row.RawSetString("kind", StringValue(kind))
	row.RawSetString("code", StringValue(code))
	row.RawSetString("reason_family", StringValue(qFallbackReasonFamilyForDetail(code, reasonCode, reason)))
	row.RawSetString("reason_code", StringValue(reasonCode))
	row.RawSetString("reason", StringValue(reason))
	row.RawSetString("source", StringValue(""))
	row.RawSetString("schema_hash", StringValue(""))
	row.RawSetString("shape", StringValue(""))
	row.RawSetString("count", IntValue(int64(count)))
	return row
}

func qFallbackAttributionStatsRow(key qFallbackAttributionKey, count int) *Table {
	row := NewTable()
	row.RawSetString("kind", StringValue("reason_shape"))
	row.RawSetString("code", StringValue(key.Code))
	row.RawSetString("reason_family", StringValue(key.ReasonFamily))
	row.RawSetString("reason_code", StringValue(key.ReasonCode))
	row.RawSetString("reason", StringValue(""))
	row.RawSetString("source", StringValue(key.Source))
	row.RawSetString("schema_hash", StringValue(key.SchemaHash))
	row.RawSetString("shape", StringValue(key.Shape))
	row.RawSetString("count", IntValue(int64(count)))
	return row
}

func qFallbackReasonFamilyForDetail(code, reasonCode, reason string) string {
	reason = qNormalizeFallbackReason(reason)
	if code == qFallbackQueryKernel && reasonCode == "" {
		reasonCode = qNormalizeQueryKernelFallbackReasonCode(reason)
	}
	reasonCode = qNormalizeFallbackReasonCode(code, reasonCode, reason)
	return qFallbackReasonFamily(code, reasonCode)
}

func qNormalizeQueryKernelFallbackReasonCode(reason string) string {
	reason = strings.ToLower(strings.Join(strings.Fields(reason), " "))
	switch {
	case strings.HasPrefix(reason, "select expression "):
		return qQueryKernelReasonSelect
	case strings.Contains(reason, "order failed"):
		return qQueryKernelReasonOrder
	case strings.Contains(reason, "non-aggregate selects only"):
		return qQueryKernelReasonUnsupported
	default:
		return ""
	}
}

func qFallbackReasonFamily(code, reasonCode string) string {
	switch reasonCode {
	case qKernelReasonSupported:
		return qFallbackFamilySupported
	case qQueryKernelReasonSelect, stdq.KernelFallbackSelectExpression:
		return qFallbackFamilySelect
	case stdq.KernelFallbackWhereExpression:
		return qFallbackFamilyWhere
	case qQueryKernelReasonOrder:
		return qFallbackFamilyOrder
	case stdq.KernelFallbackGroupedProjection, stdq.KernelFallbackByExpression:
		return qFallbackFamilyGroup
	case stdq.KernelFallbackAggregateFunction, stdq.KernelFallbackAggregateExpression, stdq.KernelFallbackAggregateWeight:
		return qFallbackFamilyAggregate
	case qKernelReasonSourceUnavailable, stdq.KernelFallbackSourceUnavailable:
		return qFallbackFamilySource
	case qKernelReasonJoinUnavailable, stdq.KernelFallbackJoinPlan:
		return qFallbackFamilyJoin
	case qKernelReasonMutationPlan, stdq.KernelFallbackMutationPlan:
		return qFallbackFamilyMutation
	case stdq.KernelFallbackSchemaMismatch:
		return qFallbackFamilySchema
	case qKernelReasonCompileError:
		return qFallbackFamilyCompile
	case qKernelReasonUnsupported, qQueryKernelReasonUnsupported:
		return qFallbackFamilyKernel
	}
	switch code {
	case qQueryKernelSupported:
		return qFallbackFamilySupported
	case qFallbackKernelCompileErr:
		return qFallbackFamilyCompile
	case qFallbackSourceErr:
		return qFallbackFamilySource
	case qFallbackJoinErr:
		return qFallbackFamilyJoin
	case qFallbackMutationPlan:
		return qFallbackFamilyMutation
	case qFallbackKernelUnsupported, qFallbackQueryKernel:
		return qFallbackFamilyKernel
	default:
		return ""
	}
}

func qNormalizeFallbackReasonCode(code, reasonCode, reason string) string {
	if code == qFallbackKernelUnsupported && (reasonCode == "" || reasonCode == qKernelReasonUnsupported) {
		return stdq.KernelFallbackReasonCode(reason)
	}
	if reasonCode != "" {
		return reasonCode
	}
	return code
}

func qNormalizeFallbackReason(reason string) string {
	reason = strings.Join(strings.Fields(reason), " ")
	const maxReasonLen = 240
	if len(reason) <= maxReasonLen {
		return reason
	}
	return reason[:maxReasonLen]
}

func qClearCaches() {
	qSQLTemplateCacheMu.Lock()
	qSQLTemplateCache = make(map[string]qSQLPlanTemplate)
	qSQLTemplateOrder = nil
	qSQLTemplateStats = qSQLPlanCacheStats{}
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedPlanCache = make(map[string]data.QueryPlan)
	qSQLAlignedPlanOrder = nil
	qSQLAlignedMutationCache = make(map[string]*stdq.MutationPlan)
	qSQLAlignedMutationOrder = nil
	qSQLKernelCache = make(map[string]*data.QueryKernel)
	qSQLKernelOrder = nil
	qSQLKernelUnsupported = make(map[string]string)
	qSQLKernelUnsupportedShape = make(map[string]string)
	qSQLKernelUnsupportedOrder = nil
	qSQLKernelStatsByKey = make(map[string]*qSQLKernelCacheKeyStats)
	qSQLAlignedStats = qSQLPlanCacheStats{}
	qSQLAlignedPlanCacheMu.Unlock()

	qEvalCacheMu.Lock()
	qEvalCache = make(map[string]any)
	qEvalCacheOrder = nil
	qEvalStats = qEvalCacheStats{}
	qEvalCacheMu.Unlock()

	qQueryKernelSupportCacheMu.Lock()
	qQueryKernelSupportCache = make(map[string]qQueryKernelSupportCacheEntry)
	qQueryKernelSupportCacheOrder = nil
	qQueryKernelSupportStats = qQueryKernelSupportCacheStats{}
	qQueryKernelSupportStatsByKey = make(map[string]*qQueryKernelSupportCacheKeyStat)
	qQueryKernelSupportCacheMu.Unlock()

	qFallbackStatsMu.Lock()
	qFallbackCounters = qFallbackStats{}
	qFallbackStatsMu.Unlock()
}

func qCloneDataQueryPlan(plan data.QueryPlan) data.QueryPlan {
	plan.Source = data.Frame{}
	plan.Where = qCloneDataExpr(plan.Where)
	plan.By = append([]data.Symbol(nil), plan.By...)
	if plan.ByExprs != nil {
		byExprs := make([]data.SelectItem, len(plan.ByExprs))
		for i, item := range plan.ByExprs {
			byExprs[i] = data.SelectItem{Name: item.Name, Expr: qCloneDataExpr(item.Expr)}
		}
		plan.ByExprs = byExprs
	}
	plan.OrderBy = append([]data.OrderSpec(nil), plan.OrderBy...)
	if plan.Select != nil {
		selects := make([]data.SelectItem, len(plan.Select))
		for i, item := range plan.Select {
			selects[i] = data.SelectItem{Name: item.Name, Expr: qCloneDataExpr(item.Expr)}
		}
		plan.Select = selects
	}
	if plan.Aggregates != nil {
		aggs := make([]data.Aggregate, len(plan.Aggregates))
		for i, agg := range plan.Aggregates {
			aggs[i] = data.Aggregate{Name: agg.Name, Func: agg.Func, Expr: qCloneDataExpr(agg.Expr), Weight: qCloneDataExpr(agg.Weight)}
		}
		plan.Aggregates = aggs
	}
	return plan
}

func qCloneDataFramePtr(frame *data.Frame) *data.Frame {
	if frame == nil {
		return nil
	}
	out, err := frame.Gather(dataFrameAllIndexes(*frame))
	if err != nil {
		clone := *frame
		return &clone
	}
	return &out
}

func qCloneDataSymbols(in []data.Symbol) []data.Symbol {
	if in == nil {
		return nil
	}
	return append([]data.Symbol(nil), in...)
}

func dataFrameAllIndexes(frame data.Frame) []int {
	indexes := make([]int, frame.Len())
	for i := range indexes {
		indexes[i] = i
	}
	return indexes
}

func qCloneQMutationPlan(plan *stdq.MutationPlan) *stdq.MutationPlan {
	if plan == nil {
		return nil
	}
	out := *plan
	out.Where = qCloneDataExpr(out.Where)
	if plan.ByExprs != nil {
		out.ByExprs = make([]data.SelectItem, len(plan.ByExprs))
		for i, item := range plan.ByExprs {
			out.ByExprs[i] = data.SelectItem{Name: item.Name, Expr: qCloneDataExpr(item.Expr)}
		}
	}
	if plan.Assignments != nil {
		out.Assignments = make([]stdq.MutationAssignment, len(plan.Assignments))
		for i, assign := range plan.Assignments {
			out.Assignments[i] = stdq.MutationAssignment{Name: assign.Name, Func: assign.Func, Expr: qCloneDataExpr(assign.Expr)}
		}
	}
	out.DeleteColumns = append([]data.Symbol(nil), plan.DeleteColumns...)
	out.InsertColumns = append([]data.Symbol(nil), plan.InsertColumns...)
	out.InsertValues = append([]data.Literal(nil), plan.InsertValues...)
	return &out
}

func qCloneQJoinPlan(plan *stdq.JoinPlan) *stdq.JoinPlan {
	if plan == nil {
		return nil
	}
	out := *plan
	out.Keys = append([]data.JoinKey(nil), plan.Keys...)
	return &out
}

func qCloneQJoinPlans(plans []*stdq.JoinPlan) []*stdq.JoinPlan {
	if plans == nil {
		return nil
	}
	out := make([]*stdq.JoinPlan, len(plans))
	for i, plan := range plans {
		out[i] = qCloneQJoinPlan(plan)
	}
	return out
}

func qCloneQExecDictPlan(plan *stdq.ExecDictPlan) *stdq.ExecDictPlan {
	if plan == nil {
		return nil
	}
	out := *plan
	return &out
}

func qCloneDataExpr(expr data.Expr) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.Binary:
		x.Left = qCloneDataExpr(x.Left)
		x.Right = qCloneDataExpr(x.Right)
		return x
	case data.Logical:
		x.Left = qCloneDataExpr(x.Left)
		x.Right = qCloneDataExpr(x.Right)
		return x
	case data.Not:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.In:
		x.Expr = qCloneDataExpr(x.Expr)
		x.Values = append([]any(nil), x.Values...)
		return x
	case data.Within:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.BucketFloorExpr:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.ListAggregateExpr:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.VectorTransformExpr:
		x.Expr = qCloneDataExpr(x.Expr)
		x.Arg = qCloneDataExpr(x.Arg)
		return x
	default:
		return expr
	}
}

func qSQLScalarBindingsFromValue(v Value) map[data.Symbol]any {
	if !v.IsTable() {
		return nil
	}
	bindings := make(map[data.Symbol]any)
	v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return true
		}
		scalar, ok := qWireScalarFromValue(val)
		if !ok {
			return true
		}
		bindings[data.Symbol(key.Str())] = scalar
		return true
	})
	if len(bindings) == 0 {
		return nil
	}
	return bindings
}

func qBindPlanOuterScalars(plan *data.QueryPlan, schema data.Schema, bindings map[data.Symbol]any) {
	if plan == nil || len(bindings) == 0 {
		return
	}
	plan.Where = qAlignDataExpr(qBindDataExprOuterScalars(plan.Where, schema, bindings), schema)
	for i := range plan.Select {
		plan.Select[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.Select[i].Expr, schema, bindings), schema)
	}
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.ByExprs[i].Expr, schema, bindings), schema)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.Aggregates[i].Expr, schema, bindings), schema)
		if plan.Aggregates[i].Weight != nil {
			plan.Aggregates[i].Weight = qAlignDataExpr(qBindDataExprOuterScalars(plan.Aggregates[i].Weight, schema, bindings), schema)
		}
	}
}

func qBindMutationOuterScalars(plan *stdq.MutationPlan, schema data.Schema, bindings map[data.Symbol]any) {
	if plan == nil || len(bindings) == 0 {
		return
	}
	plan.Where = qAlignDataExpr(qBindDataExprOuterScalars(plan.Where, schema, bindings), schema)
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.ByExprs[i].Expr, schema, bindings), schema)
	}
	for i := range plan.Assignments {
		plan.Assignments[i].Expr = qAlignAssignmentExpr(plan.Assignments[i].Name, qBindDataExprOuterScalars(plan.Assignments[i].Expr, schema, bindings), schema)
	}
}

func qBindDataExprOuterScalars(expr data.Expr, schema data.Schema, bindings map[data.Symbol]any) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.ColumnRef:
		if _, ok := schema.Kind(x.Name); ok {
			return x
		}
		if value, ok := bindings[x.Name]; ok {
			return data.Literal{Value: value}
		}
		return x
	case data.Binary:
		x.Left = qBindDataExprOuterScalars(x.Left, schema, bindings)
		x.Right = qBindDataExprOuterScalars(x.Right, schema, bindings)
		return x
	case data.Logical:
		x.Left = qBindDataExprOuterScalars(x.Left, schema, bindings)
		x.Right = qBindDataExprOuterScalars(x.Right, schema, bindings)
		return x
	case data.Not:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.In:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.Within:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.BucketFloorExpr:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.ListAggregateExpr:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.VectorTransformExpr:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		x.Arg = qBindDataExprOuterScalars(x.Arg, schema, bindings)
		return x
	default:
		return expr
	}
}

func qAlignPlanLiteralsToFrame(plan *data.QueryPlan, frame data.Frame) {
	if plan == nil {
		return
	}
	schema := frame.Schema()
	plan.Where = qAlignDataExpr(plan.Where, schema)
	for i := range plan.Select {
		plan.Select[i].Expr = qAlignDataExpr(plan.Select[i].Expr, schema)
	}
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(plan.ByExprs[i].Expr, schema)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qAlignDataExpr(plan.Aggregates[i].Expr, schema)
		if plan.Aggregates[i].Weight != nil {
			plan.Aggregates[i].Weight = qAlignDataExpr(plan.Aggregates[i].Weight, schema)
		}
	}
}

func qExpandAllColumnSelects(plan *data.QueryPlan, frame data.Frame) {
	if plan == nil || len(plan.Select) == 0 {
		return
	}
	schema := frame.Schema()
	var expanded []data.SelectItem
	changed := false
	for _, item := range plan.Select {
		lit, ok := item.Expr.(data.Literal)
		if !ok {
			expanded = append(expanded, item)
			continue
		}
		if _, ok := lit.Value.(stdq.AllColumns); !ok {
			expanded = append(expanded, item)
			continue
		}
		changed = true
		for _, name := range schema.Names() {
			expanded = append(expanded, data.SelectItem{Name: name, Expr: data.ColumnRef{Name: name}})
		}
	}
	if changed {
		plan.Select = expanded
	}
}

func qSetPreProjectOrder(plan *data.QueryPlan, frame data.Frame) {
	if plan == nil || len(plan.OrderBy) == 0 || len(plan.Aggregates) > 0 || len(plan.By) > 0 || len(plan.ByExprs) > 0 {
		return
	}
	sourceSchema := frame.Schema()
	projected := make(map[data.Symbol]struct{}, len(plan.Select))
	for _, item := range plan.Select {
		projected[item.Name] = struct{}{}
	}
	for _, spec := range plan.OrderBy {
		if _, ok := projected[spec.Column]; ok {
			continue
		}
		if _, ok := sourceSchema.Kind(spec.Column); ok {
			plan.PreProjectOrder = true
			return
		}
	}
}

func qAlignMutationLiteralsToFrame(plan *stdq.MutationPlan, frame data.Frame) {
	if plan == nil {
		return
	}
	schema := frame.Schema()
	plan.Where = qAlignDataExpr(plan.Where, schema)
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(plan.ByExprs[i].Expr, schema)
	}
	for i := range plan.Assignments {
		if plan.Assignments[i].Func != "" {
			plan.Assignments[i].Expr = qAlignDataExpr(plan.Assignments[i].Expr, schema)
		} else {
			plan.Assignments[i].Expr = qAlignAssignmentExpr(plan.Assignments[i].Name, plan.Assignments[i].Expr, schema)
		}
	}
	for i := range plan.InsertValues {
		if len(plan.InsertColumns) > 0 {
			if i >= len(plan.InsertColumns) {
				continue
			}
			plan.InsertValues[i] = qAlignMutationInsertLiteral(plan.InsertColumns[i], plan.InsertValues[i], schema)
			continue
		}
		names := schema.Names()
		if i >= len(names) {
			continue
		}
		plan.InsertValues[i] = qAlignMutationInsertLiteral(names[i], plan.InsertValues[i], schema)
	}
}

func qAlignMutationInsertLiteral(name data.Symbol, lit data.Literal, schema data.Schema) data.Literal {
	kind, ok := schema.Kind(name)
	if !ok {
		return lit
	}
	aligned := qAlignLiteralForKind(lit, kind)
	if out, ok := aligned.(data.Literal); ok {
		return out
	}
	return lit
}

func qAlignAssignmentExpr(name data.Symbol, expr data.Expr, schema data.Schema) data.Expr {
	expr = qAlignDataExpr(expr, schema)
	lit, ok := expr.(data.Literal)
	if !ok {
		return expr
	}
	kind, ok := schema.Kind(name)
	if !ok {
		return expr
	}
	return qAlignLiteralForKind(lit, kind)
}

func qAlignLiteralForKind(lit data.Literal, kind data.Kind) data.Expr {
	switch kind {
	case data.KindString:
		if sym, ok := lit.Value.(data.Symbol); ok {
			return data.Literal{Value: string(sym)}
		}
	case data.KindSymbol:
		if s, ok := lit.Value.(string); ok {
			return data.Literal{Value: data.Symbol(s)}
		}
	case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan,
		data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		if v, err := data.NormalizeValueForKind(kind, lit.Value); err == nil {
			return data.Literal{Value: v}
		}
	}
	return lit
}

func qAlignDataExpr(expr data.Expr, schema data.Schema) data.Expr {
	switch x := expr.(type) {
	case data.Binary:
		x.Left = qAlignDataExpr(x.Left, schema)
		x.Right = qAlignDataExpr(x.Right, schema)
		if x.Op == data.OpEQ || x.Op == data.OpNE {
			x.Left, x.Right = qAlignBinaryLiteral(x.Left, x.Right, schema)
			x.Right, x.Left = qAlignBinaryLiteral(x.Right, x.Left, schema)
		}
		return x
	case data.Logical:
		x.Left = qAlignDataExpr(x.Left, schema)
		x.Right = qAlignDataExpr(x.Right, schema)
		return x
	case data.Not:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		return x
	case data.In:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		x.Values = qAlignInValues(x.Expr, x.Values, schema)
		return x
	case data.Within:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		x.Low = qAlignPredicateValue(x.Expr, x.Low, schema)
		x.High = qAlignPredicateValue(x.Expr, x.High, schema)
		return x
	case data.BucketFloorExpr:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		return x
	case data.ListAggregateExpr:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		return x
	case data.VectorTransformExpr:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		x.Arg = qAlignDataExpr(x.Arg, schema)
		return x
	default:
		return expr
	}
}

func qAlignInValues(expr data.Expr, values []any, schema data.Schema) []any {
	out := append([]any(nil), values...)
	for i, v := range out {
		out[i] = qAlignPredicateValue(expr, v, schema)
	}
	return out
}

func qAlignPredicateValue(expr data.Expr, value any, schema data.Schema) any {
	col, ok := expr.(data.ColumnRef)
	if !ok {
		return value
	}
	kind, ok := schema.Kind(col.Name)
	if !ok {
		return value
	}
	lit := qAlignLiteralForKind(data.Literal{Value: value}, kind)
	if aligned, ok := lit.(data.Literal); ok {
		return aligned.Value
	}
	return value
}

func qAlignBinaryLiteral(left, right data.Expr, schema data.Schema) (data.Expr, data.Expr) {
	col, ok := left.(data.ColumnRef)
	if !ok {
		return left, right
	}
	lit, ok := right.(data.Literal)
	if !ok {
		return left, right
	}
	kind, ok := schema.Kind(col.Name)
	if !ok {
		return left, right
	}
	switch kind {
	case data.KindString, data.KindSymbol,
		data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan,
		data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		right = qAlignLiteralForKind(lit, kind)
	}
	return left, right
}

func qNormalizePlanLiterals(plan *data.QueryPlan) {
	if plan == nil {
		return
	}
	plan.Where = qNormalizeDataExpr(plan.Where)
	for i := range plan.Select {
		plan.Select[i].Expr = qNormalizeDataExpr(plan.Select[i].Expr)
	}
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qNormalizeDataExpr(plan.ByExprs[i].Expr)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qNormalizeDataExpr(plan.Aggregates[i].Expr)
		if plan.Aggregates[i].Weight != nil {
			plan.Aggregates[i].Weight = qNormalizeDataExpr(plan.Aggregates[i].Weight)
		}
	}
}

func qNormalizeMutationLiterals(plan *stdq.MutationPlan) {
	if plan == nil {
		return
	}
	plan.Where = qNormalizeDataExpr(plan.Where)
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qNormalizeDataExpr(plan.ByExprs[i].Expr)
	}
	for i := range plan.Assignments {
		plan.Assignments[i].Expr = qNormalizeDataExpr(plan.Assignments[i].Expr)
	}
	for i := range plan.InsertValues {
		if normalized, ok := qNormalizeDataExpr(plan.InsertValues[i]).(data.Literal); ok {
			plan.InsertValues[i] = normalized
		}
	}
}

func qNormalizeDataExpr(expr data.Expr) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.ColumnRef:
		switch strings.ToLower(string(x.Name)) {
		case "true":
			return data.Literal{Value: true}
		case "false":
			return data.Literal{Value: false}
		case "null":
			return data.Literal{Value: nil}
		default:
			return x
		}
	case data.Binary:
		x.Left = qNormalizeDataExpr(x.Left)
		x.Right = qNormalizeDataExpr(x.Right)
		return x
	case data.ListAggregateExpr:
		x.Expr = qNormalizeDataExpr(x.Expr)
		return x
	default:
		return x
	}
}

type qSQLOptions struct {
	order []data.OrderSpec
	limit int
}

func (o qSQLOptions) apply(plan *data.QueryPlan) {
	if len(o.order) > 0 {
		plan.OrderBy = o.order
	}
	if o.limit >= 0 {
		plan.LimitN = o.limit
	}
}

func qSQLSourceAndOptions(src string) (string, qSQLOptions, error) {
	opts := qSQLOptions{limit: -1}
	base := strings.TrimSpace(src)
	if idx := qLastKeywordIndex(base, "limit"); idx >= 0 {
		n, err := strconv.Atoi(strings.TrimSpace(base[idx+len("limit"):]))
		if err != nil || n < 0 {
			return "", opts, fmt.Errorf("limit must be a non-negative integer")
		}
		opts.limit = n
		base = strings.TrimSpace(base[:idx])
	}
	if idx := qLastKeywordIndex(base, "order by"); idx >= 0 {
		specs, err := qSQLOrderSpecs(strings.TrimSpace(base[idx+len("order by"):]))
		if err != nil {
			return "", opts, err
		}
		opts.order = specs
		base = strings.TrimSpace(base[:idx])
	}
	return base, opts, nil
}

func qLastKeywordIndex(src, keyword string) int {
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return -1
	}
	lowerKeyword := strings.Fields(strings.ToLower(keyword))
	positions := make([]int, 0, len(fields))
	pos := 0
	for _, field := range fields {
		idx := strings.Index(src[pos:], field)
		if idx < 0 {
			return -1
		}
		pos += idx
		positions = append(positions, pos)
		pos += len(field)
	}
	for i := len(fields) - len(lowerKeyword); i >= 0; i-- {
		match := true
		for j, want := range lowerKeyword {
			if strings.ToLower(fields[i+j]) != want {
				match = false
				break
			}
		}
		if match {
			return positions[i]
		}
	}
	return -1
}

func qSQLOrderSpecs(src string) ([]data.OrderSpec, error) {
	if src == "" {
		return nil, fmt.Errorf("order by requires at least one column")
	}
	parts := strings.Split(src, ",")
	specs := make([]data.OrderSpec, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 || len(fields) > 2 {
			return nil, fmt.Errorf("invalid order by specification %q", part)
		}
		spec := data.OrderSpec{Column: data.Symbol(fields[0])}
		if len(fields) == 2 {
			switch strings.ToLower(fields[1]) {
			case "asc":
			case "desc":
				spec.Desc = true
			default:
				return nil, fmt.Errorf("invalid order direction %q", fields[1])
			}
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func qDataFrameFromValue(v Value, sourceName string) (data.Frame, error) {
	source, err := qResolveSQLSourceValue(v, sourceName)
	if err != nil {
		return data.Frame{}, err
	}
	if source != v {
		return qDataFrameFromValue(source, "")
	}
	if keyed, err := qKeyedFrameFromValue(v); err == nil {
		return keyed.Frame(), nil
	}
	if v.IsSoA() {
		return qDataFrameFromSoA(v.SoA())
	}
	if !v.IsTable() {
		return data.Frame{}, fmt.Errorf("argument 1 must be a frame table or soa")
	}
	tbl := v.Table()
	if qNativeFrameRuntimeKindMatches(tbl, NativePayloadKeyedFrame) {
		keyed, ok, err := qNativeKeyedFramePayload(tbl)
		if err != nil {
			return data.Frame{}, err
		}
		if !ok {
			return data.Frame{}, fmt.Errorf("native keyed frame payload is invalid")
		}
		return keyed.Frame(), nil
	}
	if native, ok, err := qNativeDataFramePayload(tbl); err != nil {
		return data.Frame{}, err
	} else if ok {
		return native, nil
	}
	if cols := tbl.RawGetString("columns"); cols.IsTable() {
		if names := tbl.RawGetString("column_names"); names.IsTable() {
			return qDataFrameFromColumns(cols.Table(), names.Table(), qFrameColumnKinds(tbl))
		}
	}
	if soa := tbl.RawGetString("soa"); soa.IsSoA() {
		return qDataFrameFromSoA(soa.SoA())
	}
	return qDataFrameFromRowTable(tbl)
}

func qNativeDataFramePayload(tbl *Table) (data.Frame, bool, error) {
	if tbl == nil {
		return data.Frame{}, false, nil
	}
	if native, ok, err := qTypedNativeDataFramePayload(tbl); ok || err != nil {
		return native, ok, err
	}
	if blocks, err := qNativePayloadKindBlocksLegacyFrameFallback(tbl, NativePayloadDataFrame, "native data frame payload is invalid"); blocks || err != nil {
		return data.Frame{}, false, err
	}
	if native, ok := qLegacyNativeDataFramePayload(tbl); ok {
		return native, true, nil
	}
	return data.Frame{}, false, nil
}

func qSymbolsFromArgs(name string, args []Value) ([]data.Symbol, error) {
	var keys []data.Symbol
	for _, arg := range args {
		switch {
		case arg.IsString():
			keys = append(keys, data.Symbol(arg.Str()))
		case arg.IsTable():
			symbols, ok := qSymbolVectorSymbols(arg)
			if !ok {
				return nil, fmt.Errorf("%s: keys must be strings or an array table of strings", name)
			}
			keys = append(keys, symbols...)
		default:
			return nil, fmt.Errorf("%s: keys must be strings or an array table of strings", name)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s: at least one key is required", name)
	}
	return keys, nil
}

func qKeys(args []Value, keyFrame bool) ([]Value, error) {
	name := "q.keys"
	if keyFrame {
		name = "q.key"
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("%s: argument 1 required", name)
	}
	if keyed, err := qKeyedFrameFromValue(args[0]); err == nil {
		if keyFrame {
			frame, err := keyed.KeyFrame()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			rows, err := qRowsFromDataFrame(frame)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return []Value{TableValue(rows)}, nil
		}
		return []Value{qDataSymbolListValue(keyed.Keys())}, nil
	}
	if _, err := qDataFrameFromValue(args[0], ""); err == nil {
		return []Value{qDataSymbolListValue(nil)}, nil
	}
	if args[0].IsTable() {
		if keys, ok := qDictionaryKeyOrder(args[0].Table()); ok {
			return []Value{qDataSymbolListValue(keys)}, nil
		}
		keys := make([]data.Symbol, 0)
		args[0].Table().ForEachPlainRaw(func(key, _ Value) bool {
			if key.IsString() {
				keys = append(keys, data.Symbol(key.Str()))
			}
			return true
		})
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		return []Value{qDataSymbolListValue(keys)}, nil
	}
	return nil, fmt.Errorf("%s: expected dictionary, table, or keyed frame", name)
}

func qKeyedFrameToValue(keyed data.KeyedFrame) Value {
	t := NewTable()
	t.RawSetString(qKeyedFrameMarker, BoolValue(true))
	t.RawSetString("keys", qDataSymbolListValue(keyed.Keys()))
	if rows, err := qRowsFromDataFrame(keyed.Frame()); err == nil {
		t.RawSetString("frame", TableValue(rows))
	}
	setQKeyedFrameNativePayload(t, keyed)
	return TableValue(t)
}

func setQKeyedFrameNativePayload(table *Table, keyed data.KeyedFrame) {
	if table == nil {
		return
	}
	frame := keyed.Frame()
	table.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
}

func qKeyedFrameFromValue(v Value) (data.KeyedFrame, error) {
	if !v.IsTable() {
		return data.KeyedFrame{}, fmt.Errorf("argument 1 must be a keyed frame")
	}
	tbl := v.Table()
	if !qIsKeyedFrameTable(tbl) {
		return data.KeyedFrame{}, fmt.Errorf("argument 1 must be a keyed frame")
	}
	if native, ok, err := qNativeKeyedFramePayload(tbl); err != nil {
		return data.KeyedFrame{}, err
	} else if ok {
		return native, nil
	}
	frame, err := qDataFrameFromValue(tbl.RawGetString("frame"), "")
	if err != nil {
		return data.KeyedFrame{}, err
	}
	keys, err := qSymbolsFromArgs("q.lookup", []Value{tbl.RawGetString("keys")})
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.KeyBy(frame, keys...)
}

func qNativeKeyedFramePayload(tbl *Table) (data.KeyedFrame, bool, error) {
	if tbl == nil {
		return data.KeyedFrame{}, false, nil
	}
	if native, ok, err := qTypedNativeKeyedFramePayload(tbl); ok || err != nil {
		return native, ok, err
	}
	if blocks, err := qNativePayloadKindBlocksLegacyFrameFallback(tbl, NativePayloadKeyedFrame, "native keyed frame payload is invalid"); blocks || err != nil {
		return data.KeyedFrame{}, false, err
	}
	if native, ok := qLegacyNativeKeyedFramePayload(tbl); ok {
		return native, true, nil
	}
	return data.KeyedFrame{}, false, nil
}

func qNativePayloadKindBlocksLegacyFrameFallback(tbl *Table, want NativePayloadKind, invalid string) (bool, error) {
	if tbl == nil {
		return false, nil
	}
	kind, ok := tbl.NativePayloadKind()
	if !ok {
		return false, nil
	}
	if kind == want {
		return true, fmt.Errorf("%s", invalid)
	}
	return true, nil
}

func qLookupKeyValues(keyed data.KeyedFrame, args []Value) ([]any, error) {
	if len(args) == 1 && args[0].IsTable() && !qIsSymbolVector(args[0]) {
		tbl := args[0].Table()
		if values, ok := qLookupPositionalKeyValues(keyed, tbl); ok {
			return values, nil
		}
		values := make([]any, 0, len(keyed.Keys()))
		for _, key := range keyed.Keys() {
			v := tbl.RawGetString(string(key))
			if v.IsNil() {
				return nil, fmt.Errorf("key %q is missing", key)
			}
			values = append(values, qValueToAny(v))
		}
		return values, nil
	}
	values := make([]any, 0, len(args))
	for _, arg := range args {
		values = append(values, qValueToAny(arg))
	}
	return values, nil
}

func qLookupPositionalKeyValues(keyed data.KeyedFrame, tbl *Table) ([]any, bool) {
	keys := keyed.Keys()
	if len(keys) == 0 || tbl.Length() != len(keys) {
		return nil, false
	}
	values := make([]any, len(keys))
	for i := range keys {
		v := tbl.RawGetInt(int64(i + 1))
		if v.IsNil() {
			return nil, false
		}
		values[i] = qValueToAny(v)
	}
	return values, true
}

func qKeyedMutationArgs(name string, args []Value) (data.KeyedFrame, data.Frame, []data.Symbol, error) {
	if len(args) < 2 {
		return data.KeyedFrame{}, data.Frame{}, nil, fmt.Errorf("%s: expected keyed frame and delta frame", name)
	}
	keyed, err := qKeyedFrameFromValue(args[0])
	if err != nil {
		return data.KeyedFrame{}, data.Frame{}, nil, fmt.Errorf("%s: %w", name, err)
	}
	delta, err := qDataFrameFromValue(args[1], "")
	if err != nil {
		return data.KeyedFrame{}, data.Frame{}, nil, fmt.Errorf("%s: delta: %w", name, err)
	}
	valueColumns, err := qSymbolsFromArgs(name, args[2:])
	if err != nil && len(args) > 2 {
		return data.KeyedFrame{}, data.Frame{}, nil, err
	}
	return keyed, delta, valueColumns, nil
}

func qFrameLikeValue(v Value) (data.Frame, error) {
	if keyed, err := qKeyedFrameFromValue(v); err == nil {
		return keyed.Frame(), nil
	}
	return qDataFrameFromValue(v, "")
}

func qWireValueFromValue(v Value) (any, error) {
	if keyed, err := qKeyedFrameFromValue(v); err == nil {
		return keyed, nil
	}
	if dict, ok, err := qDictFromValue(v); ok || err != nil {
		return dict, err
	}
	if symbols, ok := qSymbolVectorSymbols(v); ok {
		names := make([]string, len(symbols))
		for i, sym := range symbols {
			names[i] = string(sym)
		}
		return data.NewSymbols(names), nil
	}
	if v.IsDenseArray() {
		return qDenseArrayToDataArray(v.DenseArray())
	}
	if frame, err := qDataFrameFromValue(v, ""); err == nil {
		return frame, nil
	}
	if scalar, ok := qWireScalarFromValue(v); ok {
		return scalar, nil
	}
	return nil, fmt.Errorf("unsupported value type %v", v.Type())
}

func qDictFromValue(v Value) (stdq.Dict, bool, error) {
	if !v.IsTable() {
		return stdq.Dict{}, false, nil
	}
	tbl := v.Table()
	if qLooksLikeFrame(tbl) || qIsKeyedFrameTable(tbl) || qIsSymbolVector(v) {
		return stdq.Dict{}, false, nil
	}
	keys, ok := qDictionaryKeyOrder(tbl)
	if !ok {
		return stdq.Dict{}, false, nil
	}
	out := stdq.Dict{
		Keys:   make([]any, len(keys)),
		Values: make([]any, len(keys)),
	}
	for i, key := range keys {
		out.Keys[i] = key
		value, err := qWireValueFromValue(tbl.RawGetString(string(key)))
		if err != nil {
			return stdq.Dict{}, true, err
		}
		out.Values[i] = value
	}
	return out, true, nil
}

func qDenseArrayToDataArray(array *DenseArray) (data.Array, error) {
	switch array.DType() {
	case DenseArrayI64:
		xs := make([]int64, array.Len())
		for i := range xs {
			v, err := array.At(i)
			if err != nil || !v.IsInt() {
				return nil, fmt.Errorf("i64 array row %d is not int", i)
			}
			xs[i] = v.Int()
		}
		return data.NewI64(xs), nil
	case DenseArrayF64:
		xs := make([]float64, array.Len())
		for i := range xs {
			v, err := array.At(i)
			if err != nil || !v.IsFloat() {
				return nil, fmt.Errorf("f64 array row %d is not float", i)
			}
			xs[i] = v.Float()
		}
		return data.NewF64(xs), nil
	case DenseArrayBool:
		xs := make([]bool, array.Len())
		for i := range xs {
			v, err := array.At(i)
			if err != nil || !v.IsBool() {
				return nil, fmt.Errorf("bool array row %d is not bool", i)
			}
			xs[i] = v.Bool()
		}
		return data.NewBool(xs), nil
	default:
		return nil, fmt.Errorf("unsupported dense array dtype %s", array.DType())
	}
}

func qMetaFrame(frame data.Frame) data.Frame {
	names := frame.Schema().Names()
	types := make([]string, len(names))
	for i, name := range names {
		kind, _ := frame.Schema().Kind(name)
		types[i] = string(kind)
	}
	meta, err := data.NewFrame(
		data.Column{Name: "c", Data: data.NewSymbols(symbolNames(names))},
		data.Column{Name: "t", Data: data.NewString(types)},
	)
	if err != nil {
		panic(err)
	}
	return meta
}

func symbolNames(symbols []data.Symbol) []string {
	out := make([]string, len(symbols))
	for i, sym := range symbols {
		out[i] = string(sym)
	}
	return out
}

func qLooksLikeFrame(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		return kind == NativePayloadDataFrame
	}
	return isDataFrameTable(tbl) || qLooksLikeScriptDataFrameFacade(tbl)
}

func qLooksLikeScriptDataFrameFacade(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind := tbl.RawGetString("kind"); kind.IsString() && kind.Str() == "data_frame" {
		return true
	}
	if typ := tbl.RawGetString("type"); typ.IsString() && typ.Str() == "data_frame" {
		return true
	}
	return tbl.RawGetString("columns").IsTable() && tbl.RawGetString("column_names").IsTable()
}

func qIsKeyedFrameTable(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		return kind == NativePayloadKeyedFrame
	}
	return tbl.RawGetString(qKeyedFrameMarker).Truthy()
}

func qDataFrameFromSoA(s *SoA) (data.Frame, error) {
	if s == nil {
		return data.Frame{}, fmt.Errorf("soa is nil")
	}
	cols := make([]data.Column, 0, len(s.ColumnNames()))
	for _, name := range s.ColumnNames() {
		col, ok := s.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("soa column %q not found", name)
		}
		array, err := qDataArrayFromDense(col)
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
	}
	return data.NewFrame(cols...)
}

func qDataFrameFromColumns(columns, columnNames *Table, kinds map[string]data.Kind) (data.Frame, error) {
	cols := make([]data.Column, 0, columnNames.Length())
	for i := 1; i <= columnNames.Length(); i++ {
		nameValue := columnNames.RawGetInt(int64(i))
		if !nameValue.IsString() {
			return data.Frame{}, fmt.Errorf("frame column_names[%d] must be a string", i)
		}
		name := nameValue.Str()
		col, err := qDataColumnFromVector(data.Symbol(name), columns.RawGetString(name), kinds[name])
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, col)
	}
	return data.NewFrame(cols...)
}

func qFrameColumnKinds(frame *Table) map[string]data.Kind {
	if frame == nil {
		return nil
	}
	out := map[string]data.Kind{}
	addKinds := func(v Value) {
		if !v.IsTable() {
			return
		}
		v.Table().ForEachPlainRaw(func(key, val Value) bool {
			if key.IsString() && val.IsString() {
				if kind, ok := qDataKind(val.Str()); ok {
					name := key.Str()
					if existing, exists := out[name]; exists {
						if kind == data.KindAny || kind == data.KindNull {
							return true
						}
						if existing != data.KindAny && existing != data.KindNull {
							return true
						}
					}
					out[name] = kind
				}
			}
			return true
		})
	}
	addKinds(frame.RawGetString("column_kinds"))
	addKinds(frame.RawGetString("kinds"))
	if schema := frame.RawGetString("schema"); schema.IsTable() {
		addKinds(schema.Table().RawGetString("kinds"))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func qDataKind(kind string) (data.Kind, bool) {
	switch strings.ToLower(kind) {
	case "i64", "int", "integer":
		return data.KindI64, true
	case "f64", "float", "real", "number":
		return data.KindF64, true
	case "bool", "boolean":
		return data.KindBool, true
	case "string", "text":
		return data.KindString, true
	case "symbol":
		return data.KindSymbol, true
	case "month":
		return data.KindMonth, true
	case "date":
		return data.KindDate, true
	case "datetime":
		return data.KindDateTime, true
	case "timespan":
		return data.KindTimespan, true
	case "minute":
		return data.KindMinute, true
	case "second":
		return data.KindSecond, true
	case "time":
		return data.KindTime, true
	case "timestamp":
		return data.KindTimestamp, true
	case "null":
		return data.KindNull, true
	case "any":
		return data.KindAny, true
	default:
		return "", false
	}
}

func qDataColumnFromVector(name data.Symbol, v Value, kind data.Kind) (data.Column, error) {
	if native, ok := dataNativeArrayFromValue(dataColumnWrappedValues(v)); ok {
		return data.Column{Name: name, Data: native}, nil
	}
	values, err := qAnyValuesFromVector(v)
	if err != nil {
		return data.Column{}, err
	}
	if kind == "" || kind == data.KindAny {
		return data.NewColumn(name, values), nil
	}
	typedValues := qCoerceDataValues(values, kind)
	if col, err := data.NewColumnWithKind(name, kind, typedValues); err == nil {
		return col, nil
	}
	array, ok := qTypedDataArray(typedValues, kind)
	if !ok {
		return data.NewColumn(name, values), nil
	}
	return data.Column{Name: name, Data: array}, nil
}

func qTypedDataArray(values []any, kind data.Kind) (data.Array, bool) {
	values = qCoerceDataValues(values, kind)
	for _, v := range values {
		if data.IsNull(v) {
			col, err := data.NewColumnWithKind("_", kind, values)
			if err != nil {
				return nil, false
			}
			return col.Data, true
		}
	}
	switch kind {
	case data.KindBool:
		xs := make([]bool, len(values))
		for i, v := range values {
			b, ok := v.(bool)
			if !ok {
				return nil, false
			}
			xs[i] = b
		}
		return data.NewBool(xs), true
	case data.KindI64:
		xs := make([]int64, len(values))
		for i, v := range values {
			switch n := v.(type) {
			case int:
				xs[i] = int64(n)
			case int64:
				xs[i] = n
			default:
				return nil, false
			}
		}
		return data.NewI64(xs), true
	case data.KindF64:
		xs := make([]float64, len(values))
		for i, v := range values {
			n, ok := qNumericAny(v)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewF64(xs), true
	case data.KindString:
		xs := make([]string, len(values))
		for i, v := range values {
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			xs[i] = s
		}
		return data.NewString(xs), true
	case data.KindSymbol:
		xs := make([]string, len(values))
		for i, v := range values {
			switch s := v.(type) {
			case data.Symbol:
				xs[i] = string(s)
			case string:
				xs[i] = s
			default:
				return nil, false
			}
		}
		return data.NewSymbols(xs), true
	case data.KindMonth:
		xs := make([]data.Month, len(values))
		for i, v := range values {
			n, ok := v.(data.Month)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewMonth(xs), true
	case data.KindDate:
		xs := make([]data.Date, len(values))
		for i, v := range values {
			d, ok := v.(data.Date)
			if !ok {
				return nil, false
			}
			xs[i] = d
		}
		return data.NewDate(xs), true
	case data.KindDateTime:
		xs := make([]data.DateTime, len(values))
		for i, v := range values {
			n, ok := v.(data.DateTime)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewDateTime(xs), true
	case data.KindTimespan:
		xs := make([]data.Timespan, len(values))
		for i, v := range values {
			n, ok := v.(data.Timespan)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewTimespan(xs), true
	case data.KindMinute:
		xs := make([]data.Minute, len(values))
		for i, v := range values {
			n, ok := v.(data.Minute)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewMinute(xs), true
	case data.KindSecond:
		xs := make([]data.Second, len(values))
		for i, v := range values {
			n, ok := v.(data.Second)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewSecond(xs), true
	case data.KindTime:
		xs := make([]data.Time, len(values))
		for i, v := range values {
			t, ok := v.(data.Time)
			if !ok {
				return nil, false
			}
			xs[i] = t
		}
		return data.NewTime(xs), true
	case data.KindTimestamp:
		xs := make([]data.Timestamp, len(values))
		for i, v := range values {
			ts, ok := v.(data.Timestamp)
			if !ok {
				return nil, false
			}
			xs[i] = ts
		}
		return data.NewTimestamp(xs), true
	default:
		return nil, false
	}
}

func qCoerceDataValues(values []any, kind data.Kind) []any {
	out := append([]any(nil), values...)
	for i, v := range out {
		if data.IsNull(v) {
			continue
		}
		switch kind {
		case data.KindSymbol:
			if s, ok := v.(string); ok {
				out[i] = data.Symbol(s)
			}
		case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan, data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
			if parsed, ok := qParseTemporalAny(kind, v); ok {
				out[i] = parsed
			} else if parsed, err := data.NormalizeValueForKind(kind, v); err == nil {
				out[i] = parsed
			}
		}
	}
	return out
}

func qParseTemporalAny(kind data.Kind, v any) (any, bool) {
	switch kind {
	case data.KindMonth:
		switch x := v.(type) {
		case data.Month:
			return x, true
		case int64:
			return data.MonthFromMonths(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("month", x); err == nil {
				return parsed, true
			}
		}
	case data.KindDate:
		switch x := v.(type) {
		case data.Date:
			return x, true
		case int64:
			return data.DateFromDays(x), true
		case string:
			for _, layout := range []string{"2006-01-02", "2006.01.02"} {
				if tm, err := time.Parse(layout, x); err == nil {
					return data.DateFromDays(tm.Unix() / 86400), true
				}
			}
		}
	case data.KindDateTime:
		switch x := v.(type) {
		case data.DateTime:
			return x, true
		case int64:
			return data.DateTimeFromUnixNanos(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("datetime", x); err == nil {
				return parsed, true
			}
		}
	case data.KindTimespan:
		switch x := v.(type) {
		case data.Timespan:
			return x, true
		case int64:
			return data.TimespanFromNanos(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("timespan", x); err == nil {
				return parsed, true
			}
		}
	case data.KindMinute:
		switch x := v.(type) {
		case data.Minute:
			return x, true
		case int64:
			return data.MinuteFromMinutes(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("minute", x); err == nil {
				return parsed, true
			}
		}
	case data.KindSecond:
		switch x := v.(type) {
		case data.Second:
			return x, true
		case int64:
			return data.SecondFromSeconds(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("second", x); err == nil {
				return parsed, true
			}
		}
	case data.KindTime:
		switch x := v.(type) {
		case data.Time:
			return x, true
		case int64:
			return data.TimeFromNanos(x), true
		case string:
			for _, layout := range []string{"15:04:05.999999999", "15:04:05.999999", "15:04:05.999", "15:04:05"} {
				if tm, err := time.Parse(layout, x); err == nil {
					nanos := int64(tm.Hour())*3600*1_000_000_000 + int64(tm.Minute())*60*1_000_000_000 + int64(tm.Second())*1_000_000_000 + int64(tm.Nanosecond())
					return data.TimeFromNanos(nanos), true
				}
			}
		}
	case data.KindTimestamp:
		switch x := v.(type) {
		case data.Timestamp:
			return x, true
		case int64:
			return data.TimestampFromUnixNanos(x), true
		case string:
			if tm, err := time.Parse(time.RFC3339Nano, x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
			if tm, err := time.Parse("2006-01-02T15:04:05", x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
			if tm, err := time.Parse("2006.01.02D15:04:05.999999999", x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
			if tm, err := time.Parse("2006.01.02D15:04:05", x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
		}
	}
	return nil, false
}

func qNumericAny(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func qDataFrameFromRowTable(rows *Table) (data.Frame, error) {
	if rows == nil || rows.Length() == 0 {
		return data.Frame{}, fmt.Errorf("frame requires at least one row")
	}
	first := rows.RawGetInt(1)
	if !first.IsTable() {
		return data.Frame{}, fmt.Errorf("frame rows must be tables")
	}
	names, err := qRowColumnNames(first.Table())
	if err != nil {
		return data.Frame{}, err
	}
	values := make(map[string][]any, len(names))
	for _, name := range names {
		values[name] = make([]any, 0, rows.Length())
	}
	for i := 1; i <= rows.Length(); i++ {
		rowValue := rows.RawGetInt(int64(i))
		if !rowValue.IsTable() {
			return data.Frame{}, fmt.Errorf("frame row %d must be a table", i)
		}
		row := rowValue.Table()
		for _, name := range names {
			values[name] = append(values[name], qValueToAny(row.RawGetString(name)))
		}
	}
	cols := make([]data.Column, 0, len(names))
	kinds := qFrameColumnKinds(rows)
	for _, name := range names {
		kind := data.Kind("")
		if kinds != nil {
			kind = kinds[name]
		}
		if kind != "" && kind != data.KindAny {
			if array, ok := qTypedDataArray(values[name], kind); ok {
				cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
				continue
			}
		}
		cols = append(cols, data.NewColumn(data.Symbol(name), values[name]))
	}
	return data.NewFrame(cols...)
}

func qAttachRowsNativeFramePayload(rows *Table) {
	if rows == nil || rows.Length() == 0 {
		return
	}
	frame, err := qDataFrameFromRowTable(rows)
	if err != nil {
		return
	}
	setDataFrameNativePayload(rows, frame)
}

func qAttachRowsNativeSoAPayload(rows *Table, soa *SoA) {
	if rows == nil || soa == nil || rows.Length() != soa.Len() {
		return
	}
	rows.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    len(soa.ColumnNames()),
		SchemaHash: qQueryNativeSoASchemaHash(soa),
	})
}

func qSimpleSelectRowsNativeSoA(s *SoA, mask *DenseArray, selects []qSelect) (*SoA, string, string) {
	if s == nil || mask == nil || len(selects) == 0 {
		return nil, qQueryKernelReasonUnsupported, "query native kernel requires source, mask, and select items"
	}
	filtered, err := s.Filter(mask)
	if err != nil {
		return nil, qKernelReasonUnsupported, "query native kernel filter failed: " + err.Error()
	}
	cols := make(map[string]*DenseArray, len(selects))
	for _, sel := range selects {
		col, ok, reason := qEvalNativeExprReason(filtered, sel.Expr)
		if !ok {
			if reason == "" {
				reason = fmt.Sprintf("select expression %q is not supported by q query native kernel", sel.Name)
			} else {
				reason = fmt.Sprintf("select expression %q is not supported by q query native kernel: %s", sel.Name, reason)
			}
			return nil, qQueryKernelReasonSelect, reason
		}
		cols[sel.Name] = col
	}
	out, err := NewSoA(cols)
	if err != nil {
		return nil, qQueryKernelReasonSelect, "query native kernel projection failed: " + err.Error()
	}
	return out, "", ""
}

func qEvalNativeExpr(s *SoA, expr Value) (*DenseArray, bool) {
	out, ok, _ := qEvalNativeExprReason(s, expr)
	return out, ok
}

func qEvalNativeExprReason(s *SoA, expr Value) (*DenseArray, bool, string) {
	if s == nil {
		return nil, false, "source is nil"
	}
	if expr.IsString() {
		col, ok := s.Column(expr.Str())
		if !ok {
			return nil, false, fmt.Sprintf("column %q not found", expr.Str())
		}
		return col, true, ""
	}
	if expr.IsInt() {
		out, err := NewDenseArrayOfLen(DenseArrayI64, s.Len())
		if err != nil {
			return nil, false, err.Error()
		}
		if err := out.Fill(expr); err != nil {
			return nil, false, err.Error()
		}
		return out, true, ""
	}
	if expr.IsFloat() {
		out, err := NewDenseArrayOfLen(DenseArrayF64, s.Len())
		if err != nil {
			return nil, false, err.Error()
		}
		if err := out.Fill(expr); err != nil {
			return nil, false, err.Error()
		}
		return out, true, ""
	}
	if expr.IsBool() {
		out, err := NewDenseArrayOfLen(DenseArrayBool, s.Len())
		if err != nil {
			return nil, false, err.Error()
		}
		if err := out.Fill(expr); err != nil {
			return nil, false, err.Error()
		}
		return out, true, ""
	}
	if !expr.IsTable() {
		return nil, false, fmt.Sprintf("expression type %s is not native-kernel supported", expr.TypeName())
	}
	tbl := expr.Table()
	opValue := tbl.RawGetString("op")
	if opValue.IsNil() {
		opValue = tbl.RawGetInt(1)
	}
	if !opValue.IsString() {
		return nil, false, "expression table must start with an operator"
	}
	op, ok := qNativeDenseArrayBinaryOp(opValue.Str())
	if !ok {
		return nil, false, fmt.Sprintf("operator %q is not native-kernel supported", opValue.Str())
	}
	left := tbl.RawGetString("left")
	if left.IsNil() {
		left = tbl.RawGetInt(2)
	}
	right := tbl.RawGetString("right")
	if right.IsNil() {
		right = tbl.RawGetInt(3)
	}
	leftValue, ok, reason := qNativeExprOperandReason(s, left)
	if !ok {
		return nil, false, "left operand: " + reason
	}
	rightValue, ok, reason := qNativeExprOperandReason(s, right)
	if !ok {
		return nil, false, "right operand: " + reason
	}
	out, err := DenseArrayElementwise(op, leftValue, rightValue)
	if err != nil || !out.IsDenseArray() {
		if err != nil {
			return nil, false, err.Error()
		}
		return nil, false, "native expression did not produce dense array"
	}
	return out.DenseArray(), true, ""
}

func qNativeExprOperand(s *SoA, expr Value) (Value, bool) {
	out, ok, _ := qNativeExprOperandReason(s, expr)
	return out, ok
}

func qNativeExprOperandReason(s *SoA, expr Value) (Value, bool, string) {
	if expr.IsString() {
		col, ok := s.Column(expr.Str())
		if !ok {
			return NilValue(), false, fmt.Sprintf("column %q not found", expr.Str())
		}
		return DenseArrayValue(col), true, ""
	}
	if expr.IsNumber() {
		return expr, true, ""
	}
	if expr.IsBool() {
		return expr, true, ""
	}
	if expr.IsTable() {
		col, ok, reason := qEvalNativeExprReason(s, expr)
		if !ok {
			return NilValue(), false, reason
		}
		return DenseArrayValue(col), true, ""
	}
	return NilValue(), false, fmt.Sprintf("operand type %s is not native-kernel supported", expr.TypeName())
}

func qNativeDenseArrayBinaryOp(op string) (DenseArrayBinaryOp, bool) {
	switch op {
	case "+":
		return DenseArrayAdd, true
	case "-":
		return DenseArraySub, true
	case "*":
		return DenseArrayMul, true
	case "/":
		return DenseArrayDiv, true
	case "==":
		return DenseArrayEQ, true
	case "!=":
		return DenseArrayNE, true
	case "<":
		return DenseArrayLT, true
	case "<=":
		return DenseArrayLE, true
	case ">":
		return DenseArrayGT, true
	case ">=":
		return DenseArrayGE, true
	default:
		return DenseArrayAdd, false
	}
}

func qQueryNativeRowsForResult(spec *Table, nativeRows *SoA) (*SoA, bool, string, string) {
	if nativeRows == nil {
		return nil, false, qQueryKernelReasonUnsupported, "query native kernel produced no native rows"
	}
	if spec == nil {
		return nativeRows, true, "", ""
	}
	order, err := qOrderSpecs(spec.RawGetString("order_by"))
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, err.Error()
	}
	limit, err := qLimit(spec.RawGetString("limit"))
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, err.Error()
	}
	if len(order) > 0 {
		return qOrderedNativeRowsForResult(nativeRows, order, limit)
	}
	if limit < 0 || limit >= nativeRows.Len() {
		return nativeRows, true, "", ""
	}
	sliced, handled, err := qNativeRowsFrameCarrier(nativeRows).NativeFrameSlice(0, limit)
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, "query native kernel limit failed: " + err.Error()
	}
	if !handled {
		return nil, false, qQueryKernelReasonOrder, "query native kernel limit requires native frame payload"
	}
	out, ok := qNativeRowsFromFrameValue(sliced)
	if !ok {
		return nil, false, qQueryKernelReasonOrder, "query native kernel limit produced unsupported native rows"
	}
	return out, true, "", ""
}

func qOrderedNativeRowsForResult(nativeRows *SoA, order []qOrderSpec, limit int) (*SoA, bool, string, string) {
	if nativeRows == nil || len(order) == 0 {
		if nativeRows == nil {
			return nil, false, qQueryKernelReasonUnsupported, "query native kernel produced no native rows"
		}
		return nativeRows, true, "", ""
	}
	carrier := qNativeRowsFrameCarrier(nativeRows)
	indexValue, handled, err := carrier.NativeFrameOrderIndexes(qOrderColumns(order), qOrderDescFlags(order), limit)
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, "query native kernel order failed: " + err.Error()
	}
	if !handled || !indexValue.IsDenseArray() {
		return nil, false, qQueryKernelReasonOrder, "query native kernel order requires native frame indexes"
	}
	indexes := indexValue.DenseArray()
	gathered, handled, err := carrier.NativeFrameGather(indexes)
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, "query native kernel ordered gather failed: " + err.Error()
	}
	if !handled {
		return nil, false, qQueryKernelReasonOrder, "query native kernel ordered gather requires native frame payload"
	}
	out, ok := qNativeRowsFromFrameValue(gathered)
	if !ok {
		return nil, false, qQueryKernelReasonOrder, "query native kernel ordered gather produced unsupported native rows"
	}
	return out, true, "", ""
}

func qOrderColumns(order []qOrderSpec) []string {
	out := make([]string, len(order))
	for i, ord := range order {
		out[i] = ord.Column
	}
	return out
}

func qOrderDescFlags(order []qOrderSpec) []bool {
	out := make([]bool, len(order))
	for i, ord := range order {
		out[i] = ord.Desc
	}
	return out
}

func qNativeRowsFrameCarrier(nativeRows *SoA) Value {
	carrier := NewTable()
	carrier.SetNativePayloadWithInfo(nativeRows, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       nativeRows.Len(),
		Columns:    len(nativeRows.ColumnNames()),
		SchemaHash: qQueryNativeSoASchemaHash(nativeRows),
	})
	return TableValue(carrier)
}

func qNativeRowsFromFrameValue(value Value) (*SoA, bool) {
	if !value.IsTable() {
		return nil, false
	}
	out, _, ok := value.Table().NativeFramePayload()
	if !ok {
		return nil, false
	}
	soa, ok := out.(*SoA)
	if !ok {
		return nil, false
	}
	return soa, true
}

func qQueryNativeSoASchemaHash(soa *SoA) string {
	if soa == nil {
		return "q.query.kernel:"
	}
	names := soa.ColumnNames()
	parts := make([]string, 0, len(names))
	for _, name := range names {
		col, ok := soa.Column(name)
		if !ok {
			continue
		}
		parts = append(parts, name+":"+col.DType().String())
	}
	return "q.query.kernel:" + strings.Join(parts, ",")
}

func qRowColumnNames(row *Table) ([]string, error) {
	names := make([]string, 0, row.Length())
	ok := row.ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		names = append(names, key.Str())
		return true
	})
	if !ok {
		return nil, fmt.Errorf("frame row must be a plain string-keyed table")
	}
	sort.Strings(names)
	return names, nil
}

func qDataArrayFromDense(col *DenseArray) (data.Array, error) {
	switch col.DType() {
	case DenseArrayF64:
		xs := make([]float64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Number()
		}
		return data.NewF64(xs), nil
	case DenseArrayI64:
		xs := make([]int64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Int()
		}
		return data.NewI64(xs), nil
	case DenseArrayBool:
		xs := make([]bool, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Bool()
		}
		return data.NewBool(xs), nil
	default:
		return nil, fmt.Errorf("unsupported dense array type %s", col.DType())
	}
}

func qAnyValuesFromVector(v Value) ([]any, error) {
	if v.IsDenseArray() {
		col := v.DenseArray()
		out := make([]any, col.Len())
		for i := range out {
			item, err := col.At(i)
			if err != nil {
				return nil, err
			}
			out[i] = qValueToAny(item)
		}
		return out, nil
	}
	if v.IsTable() {
		tbl := v.Table()
		out := make([]any, tbl.Length())
		for i := range out {
			out[i] = qValueToAny(tbl.RawGetInt(int64(i + 1)))
		}
		return out, nil
	}
	return nil, fmt.Errorf("must be a dense array or array table")
}

func qDataFrameValue(frame data.Frame) (Value, error) {
	return qDataFrameFacadeValue(frame)
}

func qExecDictResultValue(frame data.Frame, plan *stdq.ExecDictPlan) (Value, error) {
	if plan == nil {
		return qExecResultValue(frame)
	}
	keyColumn, ok := frame.Column(plan.KeyName)
	if !ok {
		return NilValue(), fmt.Errorf("q exec dictionary key column %q not found", plan.KeyName)
	}
	valueColumn, ok := frame.Column(plan.ValueName)
	if !ok {
		return NilValue(), fmt.Errorf("q exec dictionary value column %q not found", plan.ValueName)
	}
	keys := make([]any, frame.Len())
	values := make([]any, frame.Len())
	for i := 0; i < frame.Len(); i++ {
		key, ok := keyColumn.At(i)
		if !ok {
			return NilValue(), fmt.Errorf("q exec dictionary key row %d out of range", i)
		}
		value, ok := valueColumn.At(i)
		if !ok {
			return NilValue(), fmt.Errorf("q exec dictionary value row %d out of range", i)
		}
		keys[i] = key
		values[i] = value
	}
	return qEvalDictValue(stdq.Dict{Keys: keys, Values: values})
}

func qExecResultValue(frame data.Frame) (Value, error) {
	names := frame.Schema().Names()
	if len(names) != 1 {
		keys := make([]any, 0, len(names))
		values := make([]any, 0, len(names))
		for _, name := range names {
			col, ok := frame.Column(name)
			if !ok {
				return NilValue(), fmt.Errorf("column %q not found", name)
			}
			keys = append(keys, name)
			values = append(values, col)
		}
		return qEvalDictValue(stdq.Dict{Keys: keys, Values: values})
	}
	col, ok := frame.Column(names[0])
	if !ok {
		return NilValue(), fmt.Errorf("column %q not found", names[0])
	}
	return qEvalArrayValue(col)
}

func qRowsFromDataFrame(frame data.Frame) (*Table, error) {
	value, err := qDataFrameFacadeValue(frame)
	if err != nil {
		return nil, err
	}
	return value.Table(), nil
}

func qDataFrameFacadeValue(frame data.Frame) (Value, error) {
	names := frame.Schema().Names()
	cols := NewTable()
	kindNames := make([]string, 0, len(names))
	kindMap := make(map[string]string, len(names))
	for _, name := range names {
		col, ok := frame.Column(name)
		if !ok {
			return NilValue(), fmt.Errorf("column %q not found", name)
		}
		kind, _ := frame.Schema().Kind(name)
		cols.RawSetString(string(name), dataColumnValue(kind, dataArrayFacadeValue(col, qAnyToColumnValue)))
		kindNames = append(kindNames, string(name))
		kindMap[string(name)] = string(kind)
	}
	out := NewTable()
	out.RawSetString(dataFrameMarker, BoolValue(true))
	out.RawSetString("len", IntValue(int64(frame.Len())))
	out.RawSetString("columns", TableValue(cols))
	out.RawSetString("column_names", TableValue(dataColumnNamesTable(kindNames)))
	out.RawSetString("column_kinds", TableValue(dataColumnKindsTable(kindNames, kindMap)))
	out.RawSetString("schema", TableValue(dataSchemaTable(kindNames, kindMap)))
	out.RawSetString("row", FunctionValue(&GoFunction{Name: "data.frame.row", Fn: dataFrameRowMethod(out)}))
	out.RawSetString("gather", FunctionValue(&GoFunction{Name: "data.frame.gather", Fn: dataFrameGatherMethod(out)}))
	for _, name := range kindNames {
		out.RawSetString(name, dataColumnWrappedValues(cols.RawGetString(name)))
	}
	dataDecorateFrameTable(out, nil)
	qInstallLazyDataFrameRows(out, frame)
	setDataFrameNativePayload(out, frame)
	return TableValue(out), nil
}

func qInstallLazyDataFrameRows(out *Table, frame data.Frame) {
	if out == nil || frame.Len() <= 0 {
		return
	}
	names := frame.Schema().Names()
	cache := make(map[int64]Value)
	getRow := func(key int64) (Value, bool) {
		if key < 1 || key > int64(frame.Len()) {
			return NilValue(), false
		}
		if row, ok := cache[key]; ok {
			return row, true
		}
		row := NewTable()
		rowIndex := int(key - 1)
		for _, name := range names {
			col, ok := frame.Column(name)
			if !ok {
				return NilValue(), false
			}
			v, ok := col.At(rowIndex)
			if !ok {
				return NilValue(), false
			}
			row.RawSetString(string(name), qAnyToRowValue(v))
		}
		value := TableValue(row)
		cache[key] = value
		return value, true
	}
	rows := NewTable()
	rows.RawSetString("len", IntValue(int64(frame.Len())))
	rows.SetLazyIntGetter(frame.Len(), getRow)
	out.RawSetString("rows", TableValue(rows))
	out.SetLazyIntGetter(frame.Len(), getRow)
}

func qAnyToRowValue(v any) Value {
	if data.IsNull(v) {
		return NilValue()
	}
	return qAnyToValue(v)
}

func qValueToAny(v Value) any {
	switch {
	case v.IsNil():
		return nil
	case isDataNullValue(v):
		if kind := dataNullValueKind(v); kind != "" {
			return data.NullForKind(kind)
		}
		return data.NullValue
	case v.IsBool():
		return v.Bool()
	case v.IsInt():
		return v.Int()
	case v.IsFloat():
		return v.Float()
	case v.IsString():
		return v.Str()
	case v.IsTable() && v.Table().Length() == 0:
		return nil
	default:
		return v.String()
	}
}

func qWireScalarFromValue(v Value) (any, bool) {
	switch {
	case v.IsNil():
		return nil, true
	case isDataNullValue(v):
		if kind := dataNullValueKind(v); kind != "" {
			return data.NullForKind(kind), true
		}
		return data.NullValue, true
	case v.IsBool():
		return v.Bool(), true
	case v.IsInt():
		return v.Int(), true
	case v.IsFloat():
		return v.Float(), true
	case v.IsString():
		return v.Str(), true
	case v.IsTable() && v.Table().Length() == 0:
		return nil, true
	default:
		return nil, false
	}
}

func qAnyToValue(v any) Value {
	if data.IsNull(v) {
		if kind, ok := data.NullKind(v); ok && kind != data.KindNull {
			return dataTypedNullValue(kind)
		}
		return NilValue()
	}
	switch x := v.(type) {
	case nil:
		return NilValue()
	case bool:
		return BoolValue(x)
	case int:
		return IntValue(int64(x))
	case int8:
		return IntValue(int64(x))
	case int16:
		return IntValue(int64(x))
	case int32:
		return IntValue(int64(x))
	case int64:
		return IntValue(x)
	case uint8:
		return IntValue(int64(x))
	case uint16:
		return IntValue(int64(x))
	case uint32:
		return IntValue(int64(x))
	case uint64:
		return IntValue(int64(x))
	case float32:
		return FloatValue(float64(x))
	case float64:
		return FloatValue(x)
	case string:
		return StringValue(x)
	case data.Array:
		return dataArrayFacadeValue(x, qAnyToColumnValue)
	case stdq.Dict:
		v, err := qEvalDictValue(x)
		if err != nil {
			return StringValue(fmt.Sprint(x))
		}
		return v
	case []any:
		t := NewAppendArrayTable(len(x))
		for i, item := range x {
			t.RawSetInt(int64(i+1), qAnyToColumnValue(item))
		}
		return TableValue(t)
	case data.Symbol:
		return StringValue(string(x))
	case data.Month, data.Date, data.DateTime, data.Timespan, data.Minute, data.Second, data.Time, data.Timestamp:
		if s, ok := stdq.FormatTemporal(x); ok {
			return StringValue(s)
		}
		return StringValue(fmt.Sprint(x))
	default:
		return StringValue(fmt.Sprint(x))
	}
}

func qAnyToColumnValue(v any) Value {
	if data.IsNull(v) {
		if kind, ok := data.NullKind(v); ok {
			return dataTypedNullValue(kind)
		}
		return dataNullValue()
	}
	return qAnyToValue(v)
}

func qQueryMask(s *SoA, where Value) (*DenseArray, error) {
	if where.IsNil() {
		mask, err := NewDenseArrayOfLen(DenseArrayBool, s.Len())
		if err != nil {
			return nil, err
		}
		if err := mask.Fill(BoolValue(true)); err != nil {
			return nil, err
		}
		return mask, nil
	}
	if where.IsDenseArray() {
		mask := where.DenseArray()
		if mask.DType() != DenseArrayBool {
			return nil, fmt.Errorf("q.query where mask must be a bool dense array")
		}
		if mask.Len() != s.Len() {
			return nil, fmt.Errorf("q.query where mask length mismatch")
		}
		return mask, nil
	}
	if !where.IsTable() {
		return nil, fmt.Errorf("q.query where must be a bool dense array or condition table")
	}
	tbl := where.Table()
	column := tbl.RawGetString("column")
	if column.IsNil() {
		column = tbl.RawGetInt(1)
	}
	op := tbl.RawGetString("op")
	if op.IsNil() {
		op = tbl.RawGetInt(2)
	}
	value := tbl.RawGetString("value")
	if value.IsNil() {
		value = tbl.RawGetInt(3)
	}
	if !column.IsString() || !op.IsString() {
		return nil, fmt.Errorf("q.query where condition must provide column and op")
	}
	mask, handled, err := qNativeRowsFrameCarrier(s).NativeFrameMask(column.Str(), op.Str(), value)
	if err != nil {
		return nil, err
	}
	if !handled || !mask.IsDenseArray() {
		return nil, fmt.Errorf("q.query where condition requires native frame mask support")
	}
	return mask.DenseArray(), nil
}

type qSelect struct {
	Name string
	Expr Value
}

func qQuerySelects(v Value) ([]qSelect, error) {
	if v.IsNil() {
		return nil, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query select must be a table")
	}
	var selects []qSelect
	ok := v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		selects = append(selects, qSelect{Name: key.Str(), Expr: val})
		return true
	})
	if !ok {
		return nil, fmt.Errorf("q.query select must be a plain string-keyed table")
	}
	sort.Slice(selects, func(i, j int) bool { return selects[i].Name < selects[j].Name })
	return selects, nil
}

func qStringList(name string, v Value) ([]string, error) {
	if v.IsNil() {
		return nil, nil
	}
	if v.IsString() {
		return []string{v.Str()}, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("%s must be a string or string array", name)
	}
	n := v.Table().Length()
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		item := v.Table().RawGetInt(int64(i))
		if !item.IsString() {
			return nil, fmt.Errorf("%s item %d must be a string", name, i)
		}
		out = append(out, item.Str())
	}
	return out, nil
}

func qAggregates(v Value) (map[string]string, error) {
	if v.IsNil() {
		return nil, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query aggregate must be a table")
	}
	out := make(map[string]string)
	ok := v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() || !val.IsString() {
			return false
		}
		agg := val.Str()
		switch agg {
		case "sum", "mean", "min", "max", "count":
			out[key.Str()] = agg
			return true
		default:
			return false
		}
	})
	if !ok {
		return nil, fmt.Errorf("q.query aggregate must be a plain table of supported aggregate names")
	}
	return out, nil
}

func qRows(s *SoA, mask *DenseArray, selects []qSelect) (*Table, error) {
	rows := NewTable()
	rowIndex := int64(1)
	for i := 0; i < s.Len(); i++ {
		include, err := qMaskAt(mask, i)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		row := NewTable()
		for _, sel := range selects {
			v, err := qEvalExprAt(s, sel.Expr, i)
			if err != nil {
				return nil, fmt.Errorf("q.query select %s: %w", sel.Name, err)
			}
			row.RawSetString(sel.Name, v)
		}
		rows.RawSetInt(rowIndex, TableValue(row))
		rowIndex++
	}
	return rows, nil
}

func qGroupedRows(s *SoA, mask *DenseArray, by []string, selects []qSelect, aggs map[string]string) (*Table, error) {
	groups := make(map[string]*qGroup)
	var order []string
	for i := 0; i < s.Len(); i++ {
		include, err := qMaskAt(mask, i)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		key, vals, err := qGroupKey(s, by, i)
		if err != nil {
			return nil, err
		}
		g := groups[key]
		if g == nil {
			g = &qGroup{ByValues: vals, Aggregates: make(map[string]*qAggState)}
			groups[key] = g
			order = append(order, key)
		}
		for _, sel := range selects {
			agg, ok := aggs[sel.Name]
			if !ok {
				continue
			}
			state := g.Aggregates[sel.Name]
			if state == nil {
				state = &qAggState{Kind: agg}
				g.Aggregates[sel.Name] = state
			}
			if agg == "count" {
				state.Add(IntValue(1))
				continue
			}
			v, err := qEvalExprAt(s, sel.Expr, i)
			if err != nil {
				return nil, fmt.Errorf("q.query aggregate %s: %w", sel.Name, err)
			}
			if !v.IsNumber() {
				return nil, fmt.Errorf("q.query aggregate %s requires numeric values", sel.Name)
			}
			state.Add(v)
		}
	}
	sort.Strings(order)
	rows := NewTable()
	for i, key := range order {
		g := groups[key]
		row := NewTable()
		for idx, name := range by {
			row.RawSetString(name, g.ByValues[idx])
		}
		names := make([]string, 0, len(g.Aggregates))
		for name := range g.Aggregates {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			row.RawSetString(name, g.Aggregates[name].Value())
		}
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows, nil
}

type qOrderSpec struct {
	Column string
	Desc   bool
}

func qApplyOrderAndLimit(rows *Table, spec *Table) (*Table, error) {
	order, err := qOrderSpecs(spec.RawGetString("order_by"))
	if err != nil {
		return nil, err
	}
	limit, err := qLimit(spec.RawGetString("limit"))
	if err != nil {
		return nil, err
	}
	if len(order) == 0 && limit < 0 {
		return rows, nil
	}
	values := make([]Value, 0, rows.Length())
	for i := 1; i <= rows.Length(); i++ {
		row := rows.RawGetInt(int64(i))
		if !row.IsNil() {
			values = append(values, row)
		}
	}
	if len(order) > 0 {
		sort.SliceStable(values, func(i, j int) bool {
			left := values[i].Table()
			right := values[j].Table()
			for _, ord := range order {
				cmp := qCompareValues(left.RawGetString(ord.Column), right.RawGetString(ord.Column))
				if cmp == 0 {
					continue
				}
				if ord.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}
	if limit >= 0 && limit < len(values) {
		values = values[:limit]
	}
	out := NewTable()
	for i, row := range values {
		out.RawSetInt(int64(i+1), row)
	}
	return out, nil
}

func qOrderSpecs(v Value) ([]qOrderSpec, error) {
	if v.IsNil() {
		return nil, nil
	}
	if v.IsString() {
		return []qOrderSpec{{Column: v.Str()}}, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query order_by must be a string, table, or array")
	}
	tbl := v.Table()
	if col := tbl.RawGetString("column"); col.IsString() {
		return []qOrderSpec{{
			Column: col.Str(),
			Desc:   qTruthy(tbl.RawGetString("desc")),
		}}, nil
	}
	n := tbl.Length()
	out := make([]qOrderSpec, 0, n)
	for i := 1; i <= n; i++ {
		item := tbl.RawGetInt(int64(i))
		switch {
		case item.IsString():
			out = append(out, qOrderSpec{Column: item.Str()})
		case item.IsTable():
			col := item.Table().RawGetString("column")
			if !col.IsString() {
				return nil, fmt.Errorf("q.query order_by item %d must provide column", i)
			}
			out = append(out, qOrderSpec{
				Column: col.Str(),
				Desc:   qTruthy(item.Table().RawGetString("desc")),
			})
		default:
			return nil, fmt.Errorf("q.query order_by item %d must be a string or table", i)
		}
	}
	return out, nil
}

func qLimit(v Value) (int, error) {
	if v.IsNil() {
		return -1, nil
	}
	if !v.IsInt() {
		return 0, fmt.Errorf("q.query limit must be an integer")
	}
	if v.Int() < 0 {
		return 0, fmt.Errorf("q.query limit must be non-negative")
	}
	return int(v.Int()), nil
}

func qCompareValues(left, right Value) int {
	switch {
	case left.IsNumber() && right.IsNumber():
		lf, rf := left.Number(), right.Number()
		if lf < rf {
			return -1
		}
		if lf > rf {
			return 1
		}
		return 0
	case left.IsString() && right.IsString():
		if left.Str() < right.Str() {
			return -1
		}
		if left.Str() > right.Str() {
			return 1
		}
		return 0
	case left.IsBool() && right.IsBool():
		if left.Bool() == right.Bool() {
			return 0
		}
		if !left.Bool() {
			return -1
		}
		return 1
	case left.IsNil() && right.IsNil():
		return 0
	case left.IsNil():
		return -1
	case right.IsNil():
		return 1
	default:
		ls, rs := left.String(), right.String()
		if ls < rs {
			return -1
		}
		if ls > rs {
			return 1
		}
		return 0
	}
}

func qTruthy(v Value) bool {
	return !(v.IsNil() || (v.IsBool() && !v.Bool()))
}

type qGroup struct {
	ByValues   []Value
	Aggregates map[string]*qAggState
}

type qAggState struct {
	Kind  string
	Count int64
	Sum   float64
	Min   float64
	Max   float64
}

func (a *qAggState) Add(v Value) {
	a.Count++
	if a.Kind == "count" {
		a.Sum++
		return
	}
	x := v.Number()
	a.Sum += x
	if a.Count == 1 || x < a.Min {
		a.Min = x
	}
	if a.Count == 1 || x > a.Max {
		a.Max = x
	}
}

func (a *qAggState) Value() Value {
	switch a.Kind {
	case "count":
		return IntValue(a.Count)
	case "mean":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Sum / float64(a.Count))
	case "min":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Min)
	case "max":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Max)
	default:
		return FloatValue(a.Sum)
	}
}

func qGroupKey(s *SoA, by []string, row int) (string, []Value, error) {
	if len(by) == 0 {
		return "", nil, nil
	}
	parts := make([]string, len(by))
	values := make([]Value, len(by))
	for i, name := range by {
		col, ok := s.Column(name)
		if !ok {
			return "", nil, fmt.Errorf("q.query by column %q not found", name)
		}
		v, err := col.At(row)
		if err != nil {
			return "", nil, err
		}
		values[i] = v
		parts[i] = qValueKey(v)
	}
	return strings.Join(parts, "\x00"), values, nil
}

func qValueKey(v Value) string {
	switch {
	case v.IsInt():
		return fmt.Sprintf("i:%d", v.Int())
	case v.IsFloat():
		return fmt.Sprintf("f:%g", v.Float())
	case v.IsBool():
		return fmt.Sprintf("b:%t", v.Bool())
	case v.IsString():
		return "s:" + v.Str()
	case v.IsNil():
		return "nil"
	default:
		return v.String()
	}
}

func qEvalExprAt(s *SoA, expr Value, row int) (Value, error) {
	if expr.IsString() {
		col, ok := s.Column(expr.Str())
		if !ok {
			return NilValue(), fmt.Errorf("column %q not found", expr.Str())
		}
		return col.At(row)
	}
	if !expr.IsTable() {
		return expr, nil
	}
	tbl := expr.Table()
	op := tbl.RawGetString("op")
	if op.IsNil() {
		op = tbl.RawGetInt(1)
	}
	if !op.IsString() {
		return NilValue(), fmt.Errorf("expression table must start with an operator")
	}
	left := tbl.RawGetString("left")
	if left.IsNil() {
		left = tbl.RawGetInt(2)
	}
	right := tbl.RawGetString("right")
	if right.IsNil() {
		right = tbl.RawGetInt(3)
	}
	lv, err := qEvalExprAt(s, left, row)
	if err != nil {
		return NilValue(), err
	}
	rv, err := qEvalExprAt(s, right, row)
	if err != nil {
		return NilValue(), err
	}
	switch op.Str() {
	case "+":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() + rv.Number()), nil
	case "-":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() - rv.Number()), nil
	case "*":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() * rv.Number()), nil
	case "/":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() / rv.Number()), nil
	case "==":
		if lv.IsBool() && rv.IsBool() {
			return BoolValue(lv.Bool() == rv.Bool()), nil
		}
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() == rv.Number()), nil
	case "!=":
		if lv.IsBool() && rv.IsBool() {
			return BoolValue(lv.Bool() != rv.Bool()), nil
		}
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() != rv.Number()), nil
	case "<":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() < rv.Number()), nil
	case "<=":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() <= rv.Number()), nil
	case ">":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() > rv.Number()), nil
	case ">=":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() >= rv.Number()), nil
	default:
		return NilValue(), fmt.Errorf("operator %q is not supported", op.Str())
	}
}

func qMaskAt(mask *DenseArray, i int) (bool, error) {
	v, err := mask.At(i)
	if err != nil {
		return false, err
	}
	return v.Bool(), nil
}
