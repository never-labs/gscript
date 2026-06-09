package q

import (
	"fmt"
	"math"
	"os"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// EvalDict is a q dictionary literal lowered into library-owned values.
type EvalDict struct {
	Keys   []any
	Values []any
}

type Dict = EvalDict

type qLambda struct {
	body      string
	env       map[string]any
	namespace string
}

type qLambdaFastKind uint8

const (
	qLambdaFastInvalid qLambdaFastKind = iota
	qLambdaFastDyadic
	qLambdaFastSumPlusRight
	qLambdaFastSumPlusCountRight
)

type qLambdaFastPlan struct {
	kind qLambdaFastKind
	op   byte
}

type qProjection struct {
	fn   any
	args []projectionArg
}

type qAdverbFunction struct {
	verb   string
	adverb string
}

type qCallableAdverb struct {
	fn     any
	adverb string
}

type qAttributedVector struct {
	attribute data.Symbol
	vector    data.Array
}

type qEnumVector struct {
	domain  data.Symbol
	encoded data.Array
}

type qIPCHandle struct {
	target  string
	async   bool
	session *EvalState
}

type qDyadicFunction struct {
	name string
	fn   func(any, any) (any, error)
}

type qUnaryFunction struct {
	name string
	fn   func(any) (any, error)
}

type qComposition struct {
	funcs []qUnaryFunction
}

type qScanView struct {
	name   string
	source data.Array
}

type qCompareIndexStatsView struct {
	source data.Array
	op     data.Op
	scalar any
	count  int64
	sum    int64
}

// RuntimeKernelExecutionStat reports q.eval typed-runtime primitive execution.
// The shape matches bind's q.cache_stats runtime-kernel rows without importing
// bind into the q evaluator.
type RuntimeKernelExecutionStat struct {
	Source        string
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	Outcome       string
	ReasonCode    string
	Count         uint64
}

type runtimeKernelExecutionKey struct {
	source     string
	kernel     string
	shape      string
	route      string
	outcome    string
	reasonCode string
}

type runtimeKernelExecutionCounter struct {
	key           runtimeKernelExecutionKey
	pipelineShape string
	count         atomic.Uint64
}

type runtimeKernelProbeKey struct {
	kernel string
	shape  string
}

type runtimeKernelProbeCounters struct {
	key      runtimeKernelProbeKey
	attempt  *runtimeKernelExecutionCounter
	hit      *runtimeKernelExecutionCounter
	fallback *runtimeKernelExecutionCounter
	err      *runtimeKernelExecutionCounter
}

const runtimeKernelCounterCacheSize = 64

var (
	runtimeKernelStatsMu      sync.RWMutex
	runtimeKernelStats        map[runtimeKernelExecutionKey]*runtimeKernelExecutionCounter
	runtimeKernelProbeStats   map[runtimeKernelProbeKey]*runtimeKernelProbeCounters
	runtimeKernelStatCounters []*runtimeKernelExecutionCounter
	runtimeKernelLastCounter  atomic.Pointer[runtimeKernelExecutionCounter]
	runtimeKernelLastProbe    atomic.Pointer[runtimeKernelProbeCounters]
	runtimeKernelCounterCache [runtimeKernelCounterCacheSize]atomic.Pointer[runtimeKernelExecutionCounter]
	runtimeKernelProbeCache   [runtimeKernelCounterCacheSize]atomic.Pointer[runtimeKernelProbeCounters]
	qRuntimeShapeCache        sync.Map
)

func recordRuntimeKernelExecution(kernel, shape, outcome, reasonCode string) {
	recordRuntimeExecution("q_eval_vector_runtime", kernel, shape, "typed_data_kernel", outcome, reasonCode)
}

func recordRuntimeExecution(source, kernel, shape, route, outcome, reasonCode string) {
	reasonCode = normalizeRuntimeKernelReasonCode(outcome, reasonCode)
	key := runtimeKernelExecutionKey{
		source:     normalizeRuntimeStatField(source, "q_eval_runtime"),
		kernel:     normalizeRuntimeStatField(kernel, "unknown"),
		shape:      normalizeRuntimeStatField(shape, "unknown"),
		route:      normalizeRuntimeStatField(route, "runtime_primitive"),
		outcome:    normalizeRuntimeStatField(outcome, "unknown"),
		reasonCode: normalizeRuntimeStatField(reasonCode, outcome),
	}
	if key.reasonCode == "" {
		key.reasonCode = key.outcome
	}
	runtimeKernelCounterFor(key).count.Add(1)
}

func normalizeRuntimeKernelReasonCode(outcome, reasonCode string) string {
	switch outcome {
	case "fallback":
		return RuntimeFallbackReasonCode(reasonCode)
	case "error":
		return RuntimeFallbackRuntimeError
	default:
		return reasonCode
	}
}

func recordRuntimeFramePrimitive(kernel, shape string, err error) {
	recordRuntimeExecution("q_eval_frame_runtime", kernel, shape, "frame_runtime_primitive", "attempt", "attempt")
	if err != nil {
		recordRuntimeExecution("q_eval_frame_runtime", kernel, shape, "frame_runtime_primitive", "error", "runtime_error")
		return
	}
	recordRuntimeExecution("q_eval_frame_runtime", kernel, shape, "frame_runtime_primitive", "hit", "frame_runtime")
}

func normalizeRuntimeStatField(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func recordRuntimeKernelProbe(kernel, shape string, handled bool, err error) {
	recordRuntimeKernelProbeReason(kernel, shape, handled, err, RuntimeFallbackUnsupportedShape)
}

func recordRuntimeKernelProbeReason(kernel, shape string, handled bool, err error, fallbackReason string) {
	if kernel == "" {
		kernel = "unknown"
	}
	if shape == "" {
		shape = "unknown"
	}
	counters := runtimeKernelProbeCountersFor(kernel, shape)
	counters.attempt.count.Add(1)
	switch {
	case err != nil:
		counters.err.count.Add(1)
	case handled:
		counters.hit.count.Add(1)
	default:
		reasonCode := RuntimeFallbackReasonCode(fallbackReason)
		if reasonCode == RuntimeFallbackUnsupportedShape {
			counters.fallback.count.Add(1)
			return
		}
		recordRuntimeKernelExecution(kernel, shape, "fallback", reasonCode)
	}
}

// RuntimeKernelExecutionStats returns a stable snapshot of q.eval typed
// primitive executions for q.cache_stats.
func RuntimeKernelExecutionStats() []RuntimeKernelExecutionStat {
	counters := runtimeKernelCountersSnapshot()
	if len(counters) == 0 {
		return nil
	}
	out := make([]RuntimeKernelExecutionStat, 0, len(counters))
	for _, counter := range counters {
		count := counter.count.Load()
		if count == 0 {
			continue
		}
		key := counter.key
		out = append(out, RuntimeKernelExecutionStat{
			Source:        key.source,
			Kernel:        key.kernel,
			Shape:         key.shape,
			PipelineShape: counter.pipelineShape,
			Route:         key.route,
			Outcome:       key.outcome,
			ReasonCode:    key.reasonCode,
			Count:         count,
		})
	}
	if len(out) == 0 {
		return nil
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
		if a.Outcome != b.Outcome {
			return a.Outcome < b.Outcome
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

func runtimeKernelProbeCountersFor(kernel, shape string) *runtimeKernelProbeCounters {
	key := runtimeKernelProbeKey{kernel: kernel, shape: shape}
	if counters := runtimeKernelLastProbe.Load(); counters != nil && counters.key == key {
		return counters
	}
	cacheSlot := runtimeKernelProbeCacheIndex(key)
	if counters := runtimeKernelProbeCache[cacheSlot].Load(); counters != nil && counters.key == key {
		runtimeKernelLastProbe.Store(counters)
		return counters
	}
	runtimeKernelStatsMu.RLock()
	counters := runtimeKernelProbeStats[key]
	runtimeKernelStatsMu.RUnlock()
	if counters != nil {
		runtimeKernelLastProbe.Store(counters)
		runtimeKernelProbeCache[cacheSlot].Store(counters)
		return counters
	}

	runtimeKernelStatsMu.Lock()
	defer runtimeKernelStatsMu.Unlock()
	if runtimeKernelProbeStats == nil {
		runtimeKernelProbeStats = make(map[runtimeKernelProbeKey]*runtimeKernelProbeCounters)
	}
	if counters = runtimeKernelProbeStats[key]; counters != nil {
		runtimeKernelLastProbe.Store(counters)
		runtimeKernelProbeCache[cacheSlot].Store(counters)
		return counters
	}
	counters = &runtimeKernelProbeCounters{
		key: key,
		attempt: registerRuntimeKernelCounterLocked(runtimeKernelExecutionKey{
			source:     "q_eval_vector_runtime",
			kernel:     kernel,
			shape:      shape,
			route:      "typed_data_kernel",
			outcome:    "attempt",
			reasonCode: "attempt",
		}),
		hit: registerRuntimeKernelCounterLocked(runtimeKernelExecutionKey{
			source:     "q_eval_vector_runtime",
			kernel:     kernel,
			shape:      shape,
			route:      "typed_data_kernel",
			outcome:    "hit",
			reasonCode: "typed_kernel",
		}),
		fallback: registerRuntimeKernelCounterLocked(runtimeKernelExecutionKey{
			source:     "q_eval_vector_runtime",
			kernel:     kernel,
			shape:      shape,
			route:      "typed_data_kernel",
			outcome:    "fallback",
			reasonCode: RuntimeFallbackUnsupportedShape,
		}),
		err: registerRuntimeKernelCounterLocked(runtimeKernelExecutionKey{
			source:     "q_eval_vector_runtime",
			kernel:     kernel,
			shape:      shape,
			route:      "typed_data_kernel",
			outcome:    "error",
			reasonCode: RuntimeFallbackRuntimeError,
		}),
	}
	runtimeKernelProbeStats[key] = counters
	runtimeKernelLastProbe.Store(counters)
	runtimeKernelProbeCache[cacheSlot].Store(counters)
	return counters
}

func runtimeKernelCounterFor(key runtimeKernelExecutionKey) *runtimeKernelExecutionCounter {
	if counter := runtimeKernelLastCounter.Load(); counter != nil && counter.key == key {
		return counter
	}
	cacheSlot := runtimeKernelCounterCacheIndex(key)
	if counter := runtimeKernelCounterCache[cacheSlot].Load(); counter != nil && counter.key == key {
		runtimeKernelLastCounter.Store(counter)
		return counter
	}
	runtimeKernelStatsMu.RLock()
	counter := runtimeKernelStats[key]
	runtimeKernelStatsMu.RUnlock()
	if counter != nil {
		runtimeKernelLastCounter.Store(counter)
		runtimeKernelCounterCache[cacheSlot].Store(counter)
		return counter
	}

	runtimeKernelStatsMu.Lock()
	defer runtimeKernelStatsMu.Unlock()
	if runtimeKernelStats == nil {
		runtimeKernelStats = make(map[runtimeKernelExecutionKey]*runtimeKernelExecutionCounter)
	}
	counter = registerRuntimeKernelCounterLocked(key)
	runtimeKernelLastCounter.Store(counter)
	runtimeKernelCounterCache[cacheSlot].Store(counter)
	return counter
}

func registerRuntimeKernelCounterLocked(key runtimeKernelExecutionKey) *runtimeKernelExecutionCounter {
	if runtimeKernelStats == nil {
		runtimeKernelStats = make(map[runtimeKernelExecutionKey]*runtimeKernelExecutionCounter)
	}
	if counter := runtimeKernelStats[key]; counter != nil {
		return counter
	}
	counter := &runtimeKernelExecutionCounter{
		key:           key,
		pipelineShape: qRuntimeKernelPipelineShape(key.kernel, key.shape),
	}
	runtimeKernelStats[key] = counter
	runtimeKernelStatCounters = append(runtimeKernelStatCounters, counter)
	return counter
}

func runtimeKernelCountersSnapshot() []*runtimeKernelExecutionCounter {
	runtimeKernelStatsMu.RLock()
	defer runtimeKernelStatsMu.RUnlock()
	if len(runtimeKernelStatCounters) == 0 {
		return nil
	}
	return append([]*runtimeKernelExecutionCounter(nil), runtimeKernelStatCounters...)
}

func runtimeKernelCounterCacheIndex(key runtimeKernelExecutionKey) uintptr {
	hash := runtimeKernelStringHash(key.kernel)
	hash = hash*33 + runtimeKernelStringHash(key.shape)
	hash = hash*33 + runtimeKernelStringHash(key.outcome)
	hash = hash*33 + runtimeKernelStringHash(key.reasonCode)
	hash = hash*33 + runtimeKernelStringHash(key.source)
	hash = hash*33 + runtimeKernelStringHash(key.route)
	return hash & (runtimeKernelCounterCacheSize - 1)
}

func runtimeKernelProbeCacheIndex(key runtimeKernelProbeKey) uintptr {
	hash := runtimeKernelStringHash(key.kernel)
	hash = hash*33 + runtimeKernelStringHash(key.shape)
	return hash & (runtimeKernelCounterCacheSize - 1)
}

func runtimeKernelStringHash(value string) uintptr {
	if value == "" {
		return 0
	}
	last := len(value) - 1
	hash := uintptr(len(value))*131 + uintptr(value[0])*17 + uintptr(value[last])
	if len(value) > 2 {
		hash += uintptr(value[len(value)/2]) * 31
	}
	return hash
}

func qRuntimeKernelPipelineShape(kernel, shape string) string {
	switch {
	case kernel == "QPipelinePlan":
		if planShape := qRuntimeKernelQPipelinePlanShape(shape); planShape != "" {
			return planShape
		}
		return qRuntimeKernelPipelineShape("", shape)
	case strings.HasPrefix(shape, "gather-reduce/"):
		return "where_gather_reduce"
	case strings.HasPrefix(shape, "script-pipeline/"):
		return "script_pipeline"
	case strings.HasPrefix(shape, "logical/"):
		return "mask_combine"
	case strings.HasPrefix(shape, "where-reduce/"), strings.HasPrefix(shape, "where-index-reduce/"):
		return "mask_reduce"
	case shape == "compare-to-index-sum", shape == "compare-to-index-count", strings.HasPrefix(shape, "compare-to-index-sum/"), strings.HasPrefix(shape, "compare-to-index-count/"):
		return "mask_reduce"
	case strings.HasPrefix(shape, "compare-count/"):
		return "compare_count"
	case strings.HasPrefix(shape, "within-count/"):
		return "within_count"
	case strings.HasPrefix(shape, "compare-to-index-count-sum-stats/"), strings.HasPrefix(shape, "compare-to-index-count-stats/"), strings.HasPrefix(shape, "compare-to-index-sum-stats/"):
		return "compare_index_stats"
	case strings.HasPrefix(shape, "within-to-index-count-sum-stats/"), strings.HasPrefix(shape, "within-to-index-count-stats/"), strings.HasPrefix(shape, "within-to-index-sum-stats/"):
		return "within_index_stats"
	case strings.HasPrefix(shape, "in-to-index-count-sum-stats/"), strings.HasPrefix(shape, "in-to-index-count-stats/"), strings.HasPrefix(shape, "in-to-index-sum-stats/"),
		strings.HasPrefix(shape, "in-to-index-count-sum/"), strings.HasPrefix(shape, "in-to-index-count/"), strings.HasPrefix(shape, "in-to-index-sum/"):
		return "membership_index_stats"
	case strings.HasPrefix(shape, "compare-to-index/"):
		return "compare_index"
	case strings.HasPrefix(shape, "within-to-index/"):
		return "within_index"
	case strings.HasPrefix(shape, "in-to-index/"):
		return "membership_index"
	case strings.HasPrefix(shape, "mask-to-index/"):
		return "mask_to_index"
	case strings.HasPrefix(shape, "vector-dyadic/"), strings.HasPrefix(shape, "composite-dyadic/"):
		return "vector_map"
	case strings.HasPrefix(shape, "vector-reduce/"):
		return "vector_reduce"
	case strings.HasPrefix(shape, "vector-count/"):
		return "vector_scan"
	case strings.HasPrefix(shape, "gather/"):
		return "gather"
	case strings.HasPrefix(shape, "matrix-row/"):
		return "matrix_row_index"
	case strings.HasPrefix(shape, "scalar-index/"), strings.HasPrefix(shape, "apply-index/"), strings.HasPrefix(shape, "callable-dot/"):
		return "apply_index"
	case strings.HasPrefix(shape, "and/"), strings.HasPrefix(shape, "or/"):
		return "mask_combine"
	case strings.HasPrefix(shape, "sort-index/"):
		return "sort_index"
	case strings.HasPrefix(shape, "sort-gather/"):
		return "sort_gather"
	case strings.HasPrefix(shape, "rank/"):
		return "rank"
	case strings.HasPrefix(shape, "fby-"), strings.Contains(shape, "/fby-"):
		return "group_aggregate"
	case strings.HasPrefix(shape, "like-count/"), strings.HasPrefix(shape, "in-count/"):
		return "predicate_count"
	case strings.HasPrefix(shape, "bin-reduce/"):
		return "search_index_reduce"
	case strings.HasPrefix(shape, "bin/"):
		return "search_index"
	case strings.HasPrefix(shape, "last-scan/"), strings.HasPrefix(shape, "vector-last/"):
		return "terminal_scan"
	case strings.HasPrefix(shape, "amend-indexes/"):
		return "indexed_amend"
	case strings.HasPrefix(shape, "scalar-fill/"), strings.HasPrefix(shape, "null-count/"), strings.HasPrefix(shape, "true-count/"):
		return "null_mask"
	case strings.HasPrefix(shape, "count-reverse/"):
		return "reverse_count"
	case strings.HasPrefix(shape, "string-cast/"), strings.HasPrefix(shape, "string-case/"):
		return "string_map"
	case strings.HasPrefix(shape, "vector-unary/"):
		return "vector_map"
	case strings.HasPrefix(shape, "frame-gather/"):
		return "frame_gather"
	case strings.HasPrefix(shape, "frame-sort/"):
		return "frame_sort"
	case strings.HasPrefix(shape, "frame-reorder/"):
		return "frame_reorder"
	case strings.HasPrefix(shape, "frame-group/"):
		return "frame_group"
	case strings.HasPrefix(shape, "frame-ungroup/"):
		return "frame_ungroup"
	case strings.HasPrefix(shape, "frame-key/"):
		return "frame_key"
	case strings.HasPrefix(shape, "frame-meta/"):
		return "frame_metadata"
	case kernel != "":
		return "kernel/" + kernel
	default:
		return "unknown"
	}
}

func qRuntimeKernelQPipelinePlanShape(shape string) string {
	switch shape {
	case "where-reduce/sum", "where-index-reduce/sum",
		"compare-to-index-sum", "compare-to-index-count",
		"compare-to-index-sum-mod", "compare-to-index-count-mod":
		return "mask_reduce"
	case "gather-reduce/sum":
		return "gather_reduce"
	case "compare-to-index", "compare-to-index-mod":
		return "compare_index"
	case "vector-reduce/sum-deltas", "vector-reduce/sum-expr",
		"vector-reduce/sum-dyadic-min", "vector-reduce/sum-dyadic-max",
		"vector-reduce/sum-dyadic-float-xexp", "vector-reduce/sum-dyadic-float-xlog",
		"vector-reduce/sum-reverse", "vector-reduce/sum-rotate",
		"vector-reduce/sum-sublist", "vector-reduce/sum-ratios",
		"vector-reduce/sum-raze", "vector-reduce/sum-msum",
		"vector-reduce/sum-mavg", "vector-reduce/sum-mcount",
		"vector-reduce/sum-mmin", "vector-reduce/sum-mmax":
		return "vector_reduce"
	case "vector-count/expr", "vector-count/sums", "vector-count/prds",
		"vector-count/mins", "vector-count/maxs", "vector-count/avgs",
		"vector-last/sums", "vector-last/prds", "vector-last/mins",
		"vector-last/maxs", "vector-last/avgs":
		return "vector_scan"
	case "sequence-count/trim", "sequence-count/ltrim", "sequence-count/rtrim",
		"sequence-count/cross", "sequence-count/cut", "sequence-count/sublist",
		"sequence-count/raze", "sequence-count/value":
		return "sequence_count"
	case "bin-reduce/sum":
		return "search_index_reduce"
	case "apply-index/scalar-at", "apply-index/scalar-dot", "apply-index/path-dot":
		return "apply_index"
	default:
		switch {
		case strings.HasPrefix(shape, "runtime-unary/"):
			return qRuntimePrimitivePipelineShape(strings.TrimPrefix(shape, "runtime-unary/"))
		case strings.HasPrefix(shape, "runtime-dyadic/"):
			return qRuntimePrimitivePipelineShape(strings.TrimPrefix(shape, "runtime-dyadic/"))
		default:
			return ""
		}
	}
}

type qRuntimeShapeKey struct {
	family string
	op     string
	left   data.Kind
	right  data.Kind
	args   int
}

func qRuntimeKernelDyadicFloatSumShape(op string, leftKind, rightKind data.Kind) string {
	key := qRuntimeShapeKey{
		family: "vector-reduce/sum-dyadic-float",
		op:     op,
		left:   leftKind,
		right:  rightKind,
	}
	if value, ok := qRuntimeShapeCache.Load(key); ok {
		return value.(string)
	}
	shape := "vector-reduce/sum-dyadic-float-" + op + "/" + string(leftKind) + "/" + string(rightKind)
	value, _ := qRuntimeShapeCache.LoadOrStore(key, shape)
	return value.(string)
}

func qRuntimeKernelSequenceTransformSumShape(transform string, valueKind data.Kind, argCount int) string {
	key := qRuntimeShapeKey{
		family: "vector-reduce/sum-sequence-transform",
		op:     transform,
		left:   valueKind,
		args:   argCount,
	}
	if value, ok := qRuntimeShapeCache.Load(key); ok {
		return value.(string)
	}
	shape := "vector-reduce/sum-" + transform + "/" + string(valueKind)
	if argCount > 0 {
		shape += "/args-" + strconv.Itoa(argCount)
	}
	value, _ := qRuntimeShapeCache.LoadOrStore(key, shape)
	return value.(string)
}

func qFrameGatherShape(op string, frame data.Frame, indexes []int) string {
	return "frame-gather/" + op + "/" + qRuntimeCardinalityShape(len(indexes)) + "/cols-" + strconv.Itoa(len(data.FrameColumnNames(frame)))
}

func qFrameMetadataShape(op string, rows int, cols int) string {
	return "frame-meta/" + op + "/" + qRuntimeCardinalityShape(rows) + "/cols-" + strconv.Itoa(cols)
}

func qRecordFrameMetadataPrimitive(op string, rows int, cols int, err error) {
	recordRuntimeFramePrimitive("FrameMetadata", qFrameMetadataShape(op, rows, cols), err)
}

func qRuntimeCardinalityShape(n int) string {
	switch {
	case n == 0:
		return "rows-0"
	case n == 1:
		return "rows-1"
	case n <= 16:
		return "rows-small"
	case n <= 1024:
		return "rows-medium"
	default:
		return "rows-large"
	}
}

func qGatherFrameRuntime(op string, frame data.Frame, indexes []int) (data.Frame, error) {
	out, err := data.GatherFrame(frame, indexes)
	recordRuntimeFramePrimitive("FrameGather", qFrameGatherShape(op, frame, indexes), err)
	return out, err
}

// ClearRuntimeKernelExecutionStats resets q.eval runtime-kernel counters.
func ClearRuntimeKernelExecutionStats() {
	for _, counter := range runtimeKernelCountersSnapshot() {
		counter.count.Store(0)
	}
}

type projectionArg struct {
	value   any
	missing bool
}

// EvalState carries q script bindings through recursive evaluation without
// relying on package-global state.
type EvalState struct {
	env                  map[string]any
	port                 int64
	namespace            string
	oneShot              bool
	scriptCache          map[string]qScriptPlan
	valueExprCache       map[string]Expr
	pipelineCache        map[string]qPipelinePlan
	applyIndexCache      map[string]qScalarApplyIndexPlan
	dotApplyCache        map[string]qDotApplyPlan
	deferScanAssignments map[string]bool
}

const qGlobalScriptPlanCacheLimit = 512
const qGlobalPipelinePlanCacheLimit = 512
const qGlobalPipelineBindingCacheLimit = 1024

// EvalPlanCacheStats reports process-wide q.eval plan-cache observability.
// The cached artifacts are schema-free q expression plans keyed only by the
// normalized source text; executable mutable state is cloned before use.
type EvalPlanCacheStats struct {
	ScriptEntries            int
	ScriptHits               uint64
	ScriptMisses             uint64
	ScriptStores             uint64
	ScriptEvictions          uint64
	PipelineEntries          int
	PipelineHits             uint64
	PipelineMisses           uint64
	PipelineStores           uint64
	PipelineEvictions        uint64
	PipelineBindingEntries   int
	PipelineBindingHits      uint64
	PipelineBindingMisses    uint64
	PipelineBindingStores    uint64
	PipelineBindingEvictions uint64
}

var (
	qGlobalScriptPlanCacheMu         sync.Mutex
	qGlobalScriptPlanCache           = make(map[string]qScriptPlan)
	qGlobalScriptPlanCacheOrder      []string
	qGlobalPipelinePlanCache         = make(map[string]qPipelinePlan)
	qGlobalPipelinePlanCacheOrder    []string
	qGlobalPipelineBindingCache      = make(map[qPipelineBindingCacheKey]qPipelineBoundPlan)
	qGlobalPipelineBindingCacheOrder []qPipelineBindingCacheKey
	qGlobalScriptPlanStats           EvalPlanCacheStats
)

func NewEvalState(env map[string]any) *EvalState {
	return &EvalState{env: cloneEnv(env), namespace: "."}
}

func Eval(src string) (any, error) {
	return NewEvalState(nil).Eval(src)
}

func EvalWithEnv(src string, env map[string]any) (any, error) {
	return (&EvalState{env: cloneEnv(env), namespace: ".", oneShot: true}).Eval(src)
}

// EvalSourceCacheable reports whether src is safe for callers to memoize across
// stateless Eval calls. Stateful workspace, system, IPC, and filesystem forms
// stay uncached so repeated q.eval calls remain observably fresh.
func EvalSourceCacheable(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	scan := qEvalCacheScanText(src)
	lower := strings.ToLower(scan)
	parts := splitQScriptStatements(lower)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "\\") {
			return false
		}
	}
	uncacheable := []string{
		".z.", "hopen", "hsym", "set ", "get ", "`:", "system ",
	}
	for _, marker := range uncacheable {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func qEvalCacheScanText(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if ch != '"' {
			out.WriteByte(ch)
			continue
		}
		out.WriteByte(' ')
		for i++; i < len(src); i++ {
			ch = src[i]
			out.WriteByte(' ')
			if ch == '\\' {
				i++
				if i < len(src) {
					out.WriteByte(' ')
				}
				continue
			}
			if ch == '"' {
				break
			}
		}
	}
	return out.String()
}

// EvalValueCacheable reports whether a returned Eval value is immutable enough
// for bind layers to store and convert again on cache hits.
func EvalValueCacheable(v any) bool {
	switch v.(type) {
	case Dict, data.Array, data.Frame, data.KeyedFrame,
		nil, bool, int, int64, float32, float64, string, data.Symbol,
		data.Month, data.Date, data.DateTime, data.Timespan,
		data.Minute, data.Second, data.Time, data.Timestamp:
		return true
	default:
		return false
	}
}

func (s *EvalState) Eval(src string) (any, error) {
	return s.evalScript(strings.TrimSpace(src))
}

func (s *EvalState) evalScript(src string) (any, error) {
	plan := s.qScriptPlan(src)
	if plan.executable != nil {
		if out, handled, err := s.evalQScriptExecutablePlan(plan.executable); err != nil || handled {
			return out, err
		}
	}
	if plan.scriptPipeline != nil {
		if out, handled, err := s.tryEvalQScriptPipeline(plan.scriptPipeline); err != nil || handled {
			return out, err
		}
	}
	previousDeferredScans := s.deferScanAssignments
	if len(plan.statements) > 1 && plan.deferScanCandidates {
		s.deferScanAssignments = deferredScanAssignments(plan.statements, s)
	}
	defer func() {
		s.deferScanAssignments = previousDeferredScans
	}()
	if len(plan.statements) == 1 {
		return s.evalScriptStatement(plan.statements[0])
	}
	var last any
	for i := 0; i < len(plan.statements); i++ {
		stmt := plan.statements[i]
		if stmt.src == "" {
			continue
		}
		if value, next, handled, err := s.tryEvalNumericReductionBundle(plan.statements, i); err != nil || handled {
			if err != nil {
				return nil, err
			}
			last = value
			i = next - 1
			continue
		}
		v, err := s.evalScriptStatement(stmt)
		if err != nil {
			return nil, err
		}
		last = v
	}
	if last == nil {
		return nil, fmt.Errorf("empty q script")
	}
	return last, nil
}

type qScriptPlan struct {
	statements          []qScriptStatement
	deferScanCandidates bool
	scriptPipeline      *qScriptPipelineDescriptor
	executable          *qScriptExecutablePlan
}

type qScriptStatement struct {
	src         string
	assign      string
	rhs         string
	valueExpr   Expr
	bindingPlan qScriptBindingPlan
	fastPlan    qEvalFastPlan
}

type qEvalFastPlanKind uint8

const (
	qEvalFastInvalid qEvalFastPlanKind = iota
	qEvalFastPipeline
	qEvalFastScalarApplyIndex
)

type qEvalFastPlan struct {
	kind        qEvalFastPlanKind
	pipeline    qPipelinePlan
	scalarIndex qScalarApplyIndexPlan
}

type qScriptExecutableKind uint8

const (
	qScriptExecutableInvalid qScriptExecutableKind = iota
	qScriptExecutableSingleStatement
)

type qScriptExecutablePlan struct {
	kind      qScriptExecutableKind
	statement qScriptStatement
}

func (s *EvalState) qScriptPlan(src string) qScriptPlan {
	src = strings.TrimSpace(src)
	if s.scriptCache != nil {
		if plan, ok := s.scriptCache[src]; ok {
			return plan
		}
	}
	if qScriptPlanGlobalCacheable(src) {
		if plan, ok := qGlobalScriptPlanCacheProbe(src); ok {
			s.rememberQScriptPlan(src, plan)
			return plan
		}
	}
	plan := buildQScriptPlan(src)
	if qScriptPlanGlobalCacheable(src) {
		qGlobalScriptPlanCacheStore(src, plan)
	}
	s.rememberQScriptPlan(src, plan)
	return plan
}

func (s *EvalState) rememberQScriptPlan(src string, plan qScriptPlan) {
	if s.oneShot {
		return
	}
	if s.scriptCache == nil {
		s.scriptCache = make(map[string]qScriptPlan, 16)
	} else if len(s.scriptCache) >= 256 {
		s.scriptCache = make(map[string]qScriptPlan, 16)
	}
	s.scriptCache[src] = plan
}

func qScriptPlanGlobalCacheable(src string) bool {
	return EvalSourceCacheable(src)
}

func qGlobalScriptPlanCacheProbe(src string) (qScriptPlan, bool) {
	qGlobalScriptPlanCacheMu.Lock()
	plan, ok := qGlobalScriptPlanCache[src]
	if ok {
		qGlobalScriptPlanStats.ScriptHits++
	} else {
		qGlobalScriptPlanStats.ScriptMisses++
	}
	qGlobalScriptPlanCacheMu.Unlock()
	if !ok {
		return qScriptPlan{}, false
	}
	qRecordScriptPlanFastPipelineCacheHits(plan)
	return plan, true
}

func qRecordScriptPlanFastPipelineCacheHits(plan qScriptPlan) {
	for _, stmt := range plan.statements {
		if stmt.fastPlan.kind != qEvalFastPipeline || stmt.fastPlan.pipeline.source == "" {
			continue
		}
		qGlobalPipelinePlanCacheProbe(stmt.fastPlan.pipeline.source)
	}
}

func qGlobalScriptPlanCacheStore(src string, plan qScriptPlan) {
	if src == "" {
		return
	}
	qGlobalScriptPlanCacheMu.Lock()
	if _, ok := qGlobalScriptPlanCache[src]; !ok {
		qGlobalScriptPlanCacheOrder = append(qGlobalScriptPlanCacheOrder, src)
	}
	qGlobalScriptPlanCache[src] = plan
	qGlobalScriptPlanStats.ScriptStores++
	for len(qGlobalScriptPlanCacheOrder) > qGlobalScriptPlanCacheLimit {
		evict := qGlobalScriptPlanCacheOrder[0]
		qGlobalScriptPlanCacheOrder = qGlobalScriptPlanCacheOrder[1:]
		delete(qGlobalScriptPlanCache, evict)
		qGlobalScriptPlanStats.ScriptEvictions++
	}
	qGlobalScriptPlanCacheMu.Unlock()
}

// EvalPlanCacheStatsSnapshot returns a stable snapshot of process-wide q.eval
// parse/plan cache state.
func EvalPlanCacheStatsSnapshot() EvalPlanCacheStats {
	qGlobalScriptPlanCacheMu.Lock()
	stats := qGlobalScriptPlanStats
	stats.ScriptEntries = len(qGlobalScriptPlanCache)
	stats.PipelineEntries = len(qGlobalPipelinePlanCache)
	stats.PipelineBindingEntries = len(qGlobalPipelineBindingCache)
	qGlobalScriptPlanCacheMu.Unlock()
	return stats
}

// EvalPlanCacheLimit returns the aggregate bounded capacity of q.eval plan
// caches exposed through bind's q.cache_stats table.
func EvalPlanCacheLimit() int {
	return qGlobalScriptPlanCacheLimit + qGlobalPipelinePlanCacheLimit + qGlobalPipelineBindingCacheLimit
}

// ClearEvalPlanCaches resets process-wide q.eval parse/plan caches.
func ClearEvalPlanCaches() {
	qGlobalScriptPlanCacheMu.Lock()
	qGlobalScriptPlanCache = make(map[string]qScriptPlan)
	qGlobalScriptPlanCacheOrder = nil
	qGlobalPipelinePlanCache = make(map[string]qPipelinePlan)
	qGlobalPipelinePlanCacheOrder = nil
	qGlobalPipelineBindingCache = make(map[qPipelineBindingCacheKey]qPipelineBoundPlan)
	qGlobalPipelineBindingCacheOrder = nil
	qGlobalScriptPlanStats = EvalPlanCacheStats{}
	qGlobalScriptPlanCacheMu.Unlock()
}

func buildQScriptPlan(src string) qScriptPlan {
	parts := splitQScriptStatements(src)
	statements := make([]qScriptStatement, 0, len(parts))
	deferScanCandidates := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		stmt := qScriptStatement{src: part}
		if name, op, rhs, ok := splitTopLevelAugmentedAssignment(part); ok {
			stmt.assign = name
			stmt.rhs = name + op + rhs
			stmt.valueExpr = parseCachedValueExpr(stmt.rhs)
			stmt.bindingPlan = buildQScriptWarmBindingPlan(stmt.rhs, stmt.valueExpr)
			stmt.fastPlan = buildQEvalFastPlan(stmt.rhs)
			if _, _, ok := parseDeferredScan(stmt.rhs); ok {
				deferScanCandidates = true
			}
		} else if name, rhs, ok := splitTopLevelAssignment(part); ok {
			stmt.assign = name
			stmt.rhs = rhs
			stmt.valueExpr = parseCachedValueExpr(rhs)
			stmt.bindingPlan = buildQScriptWarmBindingPlan(rhs, stmt.valueExpr)
			stmt.fastPlan = buildQEvalFastPlan(rhs)
			if _, _, ok := parseDeferredScan(rhs); ok {
				deferScanCandidates = true
			}
		} else {
			stmt.valueExpr = parseCachedValueExpr(part)
			stmt.fastPlan = buildQEvalFastPlan(part)
		}
		statements = append(statements, stmt)
	}
	pipeline, _ := buildQScriptPipelineDescriptor(statements)
	plan := qScriptPlan{statements: statements, deferScanCandidates: deferScanCandidates, scriptPipeline: pipeline}
	plan.executable = buildQScriptExecutablePlan(plan)
	return plan
}

func buildQScriptExecutablePlan(plan qScriptPlan) *qScriptExecutablePlan {
	if plan.deferScanCandidates || len(plan.statements) != 1 {
		return nil
	}
	stmt := plan.statements[0]
	if stmt.assign != "" || stmt.src == "" {
		return nil
	}
	if stmt.bindingPlan.kind == qScriptBindingInvalid && stmt.fastPlan.kind == qEvalFastInvalid && stmt.valueExpr == nil {
		return nil
	}
	return &qScriptExecutablePlan{kind: qScriptExecutableSingleStatement, statement: stmt}
}

func (s *EvalState) evalQScriptExecutablePlan(plan *qScriptExecutablePlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	switch plan.kind {
	case qScriptExecutableSingleStatement:
		stmt := plan.statement
		out, err := s.evalCachedOrString(stmt.src, stmt.valueExpr, &stmt.bindingPlan, &stmt.fastPlan)
		return out, true, err
	default:
		return nil, false, nil
	}
}

func cloneQScriptPlan(plan qScriptPlan) qScriptPlan {
	out := qScriptPlan{
		deferScanCandidates: plan.deferScanCandidates,
		scriptPipeline:      cloneQScriptPipelineDescriptor(plan.scriptPipeline),
		executable:          cloneQScriptExecutablePlan(plan.executable),
	}
	if len(plan.statements) > 0 {
		out.statements = make([]qScriptStatement, len(plan.statements))
		for i := range plan.statements {
			out.statements[i] = cloneQScriptStatement(plan.statements[i])
		}
	}
	return out
}

func cloneQScriptExecutablePlan(in *qScriptExecutablePlan) *qScriptExecutablePlan {
	if in == nil {
		return nil
	}
	out := *in
	out.statement = cloneQScriptStatement(in.statement)
	return &out
}

func cloneQScriptStatement(stmt qScriptStatement) qScriptStatement {
	stmt.bindingPlan = cloneQScriptBindingPlan(stmt.bindingPlan)
	stmt.fastPlan = cloneQEvalFastPlan(stmt.fastPlan)
	return stmt
}

func cloneQEvalFastPlan(in qEvalFastPlan) qEvalFastPlan {
	out := in
	out.pipeline = cloneQPipelinePlan(in.pipeline)
	return out
}

func cloneQScriptPipelineDescriptor(in *qScriptPipelineDescriptor) *qScriptPipelineDescriptor {
	if in == nil {
		return nil
	}
	out := *in
	out.valuePlan = cloneQScriptBindingPlan(in.valuePlan)
	out.indexPlan = cloneQScriptBindingPlan(in.indexPlan)
	out.maskPlan = cloneQScriptBindingPlan(in.maskPlan)
	out.rowValuePlan = cloneQScriptBindingPlan(in.rowValuePlan)
	out.rowIndexPlan = cloneQScriptBindingPlan(in.rowIndexPlan)
	out.colIndexPlan = cloneQScriptBindingPlan(in.colIndexPlan)
	out.scalarPlan = cloneQScriptBindingPlan(in.scalarPlan)
	out.sequenceValuePlan = cloneQScriptBindingPlan(in.sequenceValuePlan)
	out.terminalPlan = cloneQPipelinePlan(in.terminalPlan)
	if in.moduloMaskPlan != nil {
		plan := cloneQPipelinePlan(*in.moduloMaskPlan)
		out.moduloMaskPlan = &plan
	}
	if len(in.assignments) > 0 {
		out.assignments = make([]qScriptPipelineAssignment, len(in.assignments))
		for i := range in.assignments {
			out.assignments[i] = in.assignments[i]
			out.assignments[i].binding = cloneQScriptBindingPlan(in.assignments[i].binding)
		}
	}
	return &out
}

func cloneQPipelinePlan(in qPipelinePlan) qPipelinePlan {
	out := in
	if len(in.operands) > 0 {
		out.operands = make([]qPipelineOperandPlan, len(in.operands))
		for i := range in.operands {
			out.operands[i] = in.operands[i]
			out.operands[i].plan = cloneQScriptBindingPlan(in.operands[i].plan)
		}
	}
	out.valuePlan = cloneQScriptBindingPlan(in.valuePlan)
	out.indexPlan = cloneQScriptBindingPlan(in.indexPlan)
	out.maskPlan = cloneQScriptBindingPlan(in.maskPlan)
	out.leftPlan = cloneQScriptBindingPlan(in.leftPlan)
	out.rightPlan = cloneQScriptBindingPlan(in.rightPlan)
	out.modPlan = cloneQScriptBindingPlan(in.modPlan)
	out.modulusPlan = cloneQScriptBindingPlan(in.modulusPlan)
	out.modTargetPlan = cloneQScriptBindingPlan(in.modTargetPlan)
	out.reductionPlan = cloneQScriptBindingPlan(in.reductionPlan)
	if in.moduloMaskPlan != nil {
		plan := cloneQPipelinePlan(*in.moduloMaskPlan)
		out.moduloMaskPlan = &plan
	}
	return out
}

func cloneQScriptBindingPlan(in qScriptBindingPlan) qScriptBindingPlan {
	out := in
	out.cached = false
	out.cache = nil
	if len(in.items) > 0 {
		out.items = make([]qScriptBindingPlan, len(in.items))
		for i := range in.items {
			out.items[i] = cloneQScriptBindingPlan(in.items[i])
		}
	}
	if in.left != nil {
		left := cloneQScriptBindingPlan(*in.left)
		out.left = &left
	}
	if in.right != nil {
		right := cloneQScriptBindingPlan(*in.right)
		out.right = &right
	}
	return out
}

func qScriptBindingPlanTextEligible(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	if strings.ContainsAny(src, "`'\\\"") {
		return false
	}
	for _, marker := range []string{"0W", "0w", "0N", "0n"} {
		if strings.Contains(src, marker) {
			return false
		}
	}
	if strings.HasPrefix(src, "-0") {
		return false
	}
	if _, ok := lookupUnaryVerb(src); ok {
		return false
	}
	if _, ok := lookupDyadicVerbFunc(src); ok {
		return false
	}
	if _, ok := findAdverb(src); ok {
		return false
	}
	return true
}

func parseCachedValueExpr(src string) Expr {
	if !cachedValueExprTextEligible(src) {
		return nil
	}
	expr, ok, err := parseValueExpr(src)
	if err != nil || !ok {
		return nil
	}
	if !cachedValueExprEligible(expr) {
		return nil
	}
	return expr
}

func buildQScriptWarmBindingPlan(src string, expr Expr) qScriptBindingPlan {
	if plan := buildQScriptRangeBindingPlan(src); plan.kind != qScriptBindingInvalid {
		return plan
	}
	if expr != nil {
		return buildQScriptBindingPlanForRHS(src, expr)
	}
	if qScriptBindingPlanTextEligible(src) {
		return buildQScriptBindingPlanForRHS(src, nil)
	}
	return buildQScriptPrefixBindingPlan(src)
}

func cachedValueExprTextEligible(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return true
	}
	if strings.ContainsAny(src, "`'\\/$\"") {
		return false
	}
	if strings.Contains(src, "0W") || strings.Contains(src, "0w") {
		return false
	}
	if !isQArithmeticText(src) && !strings.Contains(src, "#") && !looksLikeTemporalVector(src) {
		return false
	}
	if !isQIdentStart(src[0]) {
		return true
	}
	end := 1
	for end < len(src) && isQIdentRest(src[end]) {
		end++
	}
	if end >= len(src) || !isQWhitespace(src[end]) {
		return true
	}
	next := end
	for next < len(src) && isQWhitespace(src[next]) {
		next++
	}
	if next >= len(src) {
		return true
	}
	switch src[next] {
	case '+', '-', '*', '%', ')', ']', '<', '>', '=':
		return true
	}
	return false
}

func isQArithmeticText(src string) bool {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '+', '-', '*', '%':
			return true
		}
	}
	return false
}

func cachedValueExprEligible(expr Expr) bool {
	switch x := expr.(type) {
	case Binary:
		switch strings.ToLower(x.Op) {
		case "and", "or", "in", "within", "=", "<", ">", "<=", ">=", "<>", "~":
			return false
		}
		if x.Op != "#" && cachedValueExprContainsTemporal(x) {
			return false
		}
		return cachedValueExprEligible(x.Left) && cachedValueExprEligible(x.Right)
	case Call:
		switch strings.ToLower(x.Func) {
		case "where", "null", "not", "like":
			return false
		}
		return cachedValueExprEligible(x.Arg)
	case Vector:
		for _, item := range x.Items {
			if !cachedValueExprEligible(item) {
				return false
			}
		}
		return true
	case DictExpr:
		return cachedValueExprEligible(x.Keys) && cachedValueExprEligible(x.Values)
	case IndexExpr:
		return cachedValueExprEligible(x.Expr) && cachedValueExprEligible(x.Index)
	case Flip:
		for _, column := range x.Keys {
			if !cachedValueExprEligible(column.Expr) {
				return false
			}
		}
		for _, column := range x.Columns {
			if !cachedValueExprEligible(column.Expr) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func cachedValueExprContainsTemporal(expr Expr) bool {
	switch x := expr.(type) {
	case Temporal, TypedNull:
		return true
	case Binary:
		return cachedValueExprContainsTemporal(x.Left) || cachedValueExprContainsTemporal(x.Right)
	case Call:
		return cachedValueExprContainsTemporal(x.Arg)
	case Vector:
		for _, item := range x.Items {
			if cachedValueExprContainsTemporal(item) {
				return true
			}
		}
	case DictExpr:
		return cachedValueExprContainsTemporal(x.Keys) || cachedValueExprContainsTemporal(x.Values)
	case IndexExpr:
		return cachedValueExprContainsTemporal(x.Expr) || cachedValueExprContainsTemporal(x.Index)
	case Flip:
		for _, column := range x.Keys {
			if cachedValueExprContainsTemporal(column.Expr) {
				return true
			}
		}
		for _, column := range x.Columns {
			if cachedValueExprContainsTemporal(column.Expr) {
				return true
			}
		}
	}
	return false
}

func (s *EvalState) evalScriptStatement(stmt qScriptStatement) (any, error) {
	target := stmt.src
	if stmt.assign != "" {
		target = stmt.rhs
	}
	var v any
	var err error
	handled := false
	if stmt.assign != "" && s.deferScanAssignments != nil && s.deferScanAssignments[s.resolveAssignmentName(stmt.assign)] {
		v, handled, err = s.tryEvalDeferredScanAssignment(target)
		if err != nil {
			return nil, err
		}
	}
	if !handled && stmt.assign != "" {
		v, handled, err = s.tryEvalCompareIndexStatsAssignment(target)
		if err != nil {
			return nil, err
		}
	}
	if !handled {
		v, err = s.evalCachedOrString(target, stmt.valueExpr, &stmt.bindingPlan, &stmt.fastPlan)
	}
	if err != nil {
		return nil, err
	}
	if stmt.assign != "" {
		s.env[s.resolveAssignmentName(stmt.assign)] = v
	}
	return v, nil
}

func (s *EvalState) tryEvalCompareIndexStatsAssignment(src string) (any, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(src), "where ") {
		return nil, false, nil
	}
	plan, ok := buildQPipelineWhereComparePlan(src, qPipelineWhereCompareIndexes, "compare-to-index")
	if !ok {
		return nil, false, nil
	}
	if plan.kind != qPipelineWhereCompareIndexes {
		return nil, false, nil
	}
	plan = qPipelinePlanWithBindingPlans(plan)
	left, right, err := s.evalQPipelineCompareOperands(&plan)
	if err != nil {
		return nil, true, err
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, plan.compareOp)
	if !ok {
		return nil, false, nil
	}
	if array.Kind() != data.KindSymbol && array.Kind() != data.KindString {
		return nil, false, nil
	}
	count, sum, handled, err := data.TryTypedCompareIndexStatsI64(array, dataOp, scalar)
	shape := "compare-to-index-view/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	recordRuntimeKernelProbe("ArrayWhereCompareIndexView", shape, handled, err)
	if err != nil || !handled {
		return nil, handled, err
	}
	if count > int64(int(count)) {
		return nil, true, fmt.Errorf("where index count %d exceeds int range", count)
	}
	return qCompareIndexStatsView{source: array, op: dataOp, scalar: scalar, count: count, sum: sum}, true, nil
}

func deferredScanAssignments(statements []qScriptStatement, state *EvalState) map[string]bool {
	out := make(map[string]bool)
	for i, stmt := range statements {
		if stmt.assign == "" {
			continue
		}
		if _, _, ok := parseDeferredScan(stmt.rhs); !ok {
			continue
		}
		name := state.resolveAssignmentName(stmt.assign)
		for j := i + 1; j < len(statements); j++ {
			next := statements[j]
			if next.assign != "" && state.resolveAssignmentName(next.assign) == name {
				break
			}
			if qExprContainsLastName(next.src, stmt.assign) {
				out[name] = true
				break
			}
		}
	}
	return out
}

func qExprContainsLastName(src, name string) bool {
	src = strings.TrimSpace(src)
	if src == "" || name == "" {
		return false
	}
	needle := "last " + name
	for start := 0; ; {
		idx := strings.Index(src[start:], needle)
		if idx < 0 {
			return false
		}
		pos := start + idx
		if wordBoundary(src, pos, pos+len("last")) && wordBoundary(src, pos+len("last "), pos+len(needle)) {
			return true
		}
		start = pos + len("last")
	}
}

type numericReductionBinding struct {
	assign string
	op     string
	source string
}

func (s *EvalState) tryEvalNumericReductionBundle(statements []qScriptStatement, start int) (any, int, bool, error) {
	first, ok := parseNumericReductionBinding(statements[start])
	if !ok {
		return nil, start, false, nil
	}
	sourceName := s.resolveAssignmentName(first.source)
	source, ok := s.lookupName(first.source)
	if !ok {
		return nil, start, false, nil
	}
	array, ok := source.(data.Array)
	if !ok {
		return nil, start, false, nil
	}
	bindings := []numericReductionBinding{first}
	next := start + 1
	for next < len(statements) {
		binding, ok := parseNumericReductionBinding(statements[next])
		if !ok || s.resolveAssignmentName(binding.source) != sourceName {
			break
		}
		bindings = append(bindings, binding)
		next++
	}
	if len(bindings) < 2 {
		return nil, start, false, nil
	}
	for _, binding := range bindings {
		if s.resolveAssignmentName(binding.assign) == sourceName {
			return nil, start, false, nil
		}
	}
	stats, handled, err := data.TryTypedNumericStats(array)
	recordRuntimeKernelProbe("ArrayNumericStats", "vector-reduce/bundle/"+string(array.Kind()), handled, err)
	if err != nil || !handled {
		return nil, start, handled, err
	}
	var last any
	for _, binding := range bindings {
		value := numericReductionStatsValue(stats, binding.op)
		s.env[s.resolveAssignmentName(binding.assign)] = value
		last = value
	}
	return last, next, true, nil
}

func parseNumericReductionBinding(stmt qScriptStatement) (numericReductionBinding, bool) {
	if stmt.assign == "" {
		return numericReductionBinding{}, false
	}
	op, source, ok := parseNumericReductionExpr(stmt.rhs)
	if !ok {
		return numericReductionBinding{}, false
	}
	return numericReductionBinding{assign: stmt.assign, op: op, source: source}, true
}

func parseNumericReductionExpr(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "+/") {
		source := strings.TrimSpace(src[2:])
		if isQAssignmentName(source) {
			return "sum", source, true
		}
		return "", "", false
	}
	for _, op := range []string{"sum", "min", "max", "count"} {
		prefix := op + " "
		if strings.HasPrefix(src, prefix) {
			source := strings.TrimSpace(src[len(prefix):])
			if isQAssignmentName(source) {
				return op, source, true
			}
			return "", "", false
		}
	}
	return "", "", false
}

func numericReductionStatsValue(stats data.NumericStats, op string) any {
	switch op {
	case "sum":
		if stats.HasValue {
			return stats.Sum
		}
		return data.NullValue
	case "min":
		if stats.HasValue {
			return stats.Min
		}
		return data.NullValue
	case "max":
		if stats.HasValue {
			return stats.Max
		}
		return data.NullValue
	case "count":
		return stats.Count
	default:
		return data.NullValue
	}
}

func buildQEvalFastPlan(src string) qEvalFastPlan {
	src = strings.TrimSpace(src)
	if src == "" {
		return qEvalFastPlan{}
	}
	if scalar, ok := buildScalarApplyIndexPlan(src); ok {
		return qEvalFastPlan{kind: qEvalFastScalarApplyIndex, scalarIndex: scalar}
	}
	if qPipelinePlanCandidate(src) {
		if qPipelinePlanGlobalCacheable(src) {
			if pipeline, ok := qGlobalPipelinePlanCacheProbe(src); ok {
				return qEvalFastPlan{kind: qEvalFastPipeline, pipeline: pipeline}
			}
		}
		if pipeline := buildQPipelinePlan(src); pipeline.kind != qPipelineInvalid {
			if qPipelinePlanGlobalCacheable(src) {
				qGlobalPipelinePlanCacheStore(src, pipeline)
			}
			return qEvalFastPlan{kind: qEvalFastPipeline, pipeline: pipeline}
		}
	}
	return qEvalFastPlan{}
}

func (s *EvalState) evalQFastPlan(plan *qEvalFastPlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	switch plan.kind {
	case qEvalFastPipeline:
		if plan.pipeline.kind == qPipelineInvalid {
			return nil, false, nil
		}
		return s.evalQPipelinePlan(&plan.pipeline)
	case qEvalFastScalarApplyIndex:
		target, ok := s.lookupName(plan.scalarIndex.target)
		if !ok || isCallable(target) {
			return nil, false, nil
		}
		return scalarApplyIndexPlanValue(plan.scalarIndex, target)
	default:
		return nil, false, nil
	}
}

func (s *EvalState) evalCachedOrString(src string, expr Expr, bindingPlan *qScriptBindingPlan, fastPlan *qEvalFastPlan) (any, error) {
	if bindingPlan != nil && bindingPlan.kind != qScriptBindingInvalid {
		value, handled, err := s.evalQScriptBindingPlan(bindingPlan)
		if err != nil {
			return nil, err
		}
		if handled {
			return value, nil
		}
	}
	if out, handled, err := s.evalQFastPlan(fastPlan); err != nil || handled {
		return out, err
	}
	if plan := s.qPipelinePlan(src); plan.kind != qPipelineInvalid {
		if out, handled, err := s.evalQPipelinePlan(&plan); err != nil || handled {
			return out, err
		}
	}
	if out, handled, err := s.tryEvalWhereCompareCountSum(src); err != nil || handled {
		return out, err
	}
	if out, handled, err := s.tryEvalScalarAddChain(src); err != nil || handled {
		return out, err
	}
	if expr != nil {
		value, err := s.evalValueExpr(expr)
		if err == nil {
			return value, nil
		}
	}
	return s.eval(src)
}

func (s *EvalState) tryEvalDeferredScanAssignment(src string) (any, bool, error) {
	name, arg, ok := parseDeferredScan(src)
	if !ok {
		return nil, false, nil
	}
	value, err := s.eval(arg)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	return qScanView{name: name, source: array}, true, nil
}

func (s *EvalState) evalConditionalSpecialForm(src string) (any, bool, error) {
	src = strings.TrimSpace(src)
	if len(src) < 4 || (src[0] != '$' && src[0] != '?') || src[1] != '[' || src[len(src)-1] != ']' {
		return nil, false, nil
	}
	args := splitQBracketFormArgs(src[2 : len(src)-1])
	if len(args) != 3 {
		return nil, true, fmt.Errorf("%c[] conditional expects three arguments", src[0])
	}
	cond, err := s.eval(args[0])
	if err != nil {
		return nil, true, err
	}
	truth, err := boolValue(cond)
	if err != nil {
		return nil, true, err
	}
	if truth {
		out, err := s.eval(args[1])
		return out, true, err
	}
	out, err := s.eval(args[2])
	return out, true, err
}

func (s *EvalState) evalControlSpecialForm(src string) (any, bool, error) {
	name, inner, ok := parseNamedBracketForm(src)
	if !ok {
		return nil, false, nil
	}
	switch name {
	case "if":
		args := splitQBracketFormArgs(inner)
		if len(args) != 2 {
			return nil, true, fmt.Errorf("if[] expects condition and body")
		}
		cond, err := s.eval(args[0])
		if err != nil {
			return nil, true, err
		}
		truth, err := boolValue(cond)
		if err != nil {
			return nil, true, err
		}
		if !truth {
			return data.NullValue, true, nil
		}
		out, err := s.evalScript(args[1])
		return out, true, err
	case "do":
		args := splitQBracketFormArgs(inner)
		if len(args) != 2 {
			return nil, true, fmt.Errorf("do[] expects count and body")
		}
		countValue, err := s.eval(args[0])
		if err != nil {
			return nil, true, err
		}
		count, ok := integerValue(countValue)
		if !ok || count < 0 {
			return nil, true, fmt.Errorf("do[] count must be a non-negative integer")
		}
		var out any = data.NullValue
		for i := int64(0); i < count; i++ {
			out, err = s.evalScript(args[1])
			if err != nil {
				return nil, true, err
			}
		}
		return out, true, nil
	case "while":
		args := splitQBracketFormArgs(inner)
		if len(args) != 2 {
			return nil, true, fmt.Errorf("while[] expects condition and body")
		}
		var out any = data.NullValue
		for {
			cond, err := s.eval(args[0])
			if err != nil {
				return nil, true, err
			}
			truth, err := boolValue(cond)
			if err != nil {
				return nil, true, err
			}
			if !truth {
				return out, true, nil
			}
			out, err = s.evalScript(args[1])
			if err != nil {
				return nil, true, err
			}
		}
	default:
		return nil, false, nil
	}
}

func parseNamedBracketForm(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	open := strings.IndexByte(src, '[')
	if open <= 0 || src[len(src)-1] != ']' {
		return "", "", false
	}
	name := strings.ToLower(strings.TrimSpace(src[:open]))
	if name != "if" && name != "do" && name != "while" {
		return "", "", false
	}
	if !enclosed(src[open:], '[', ']') {
		return "", "", false
	}
	return name, strings.TrimSpace(src[open+1 : len(src)-1]), true
}

func parseDeferredScan(src string) (name, arg string, ok bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, scan := range []struct {
		prefix string
		name   string
	}{
		{"+\\", "sums"},
		{"sums ", "sums"},
		{"prds ", "prds"},
		{"mins ", "mins"},
		{"maxs ", "maxs"},
		{"avgs ", "avgs"},
	} {
		if !strings.HasPrefix(src, scan.prefix) {
			continue
		}
		arg = strings.TrimSpace(src[len(scan.prefix):])
		if arg == "" {
			return "", "", false
		}
		if scan.prefix == "+\\" && strings.HasPrefix(arg, "[") {
			return "", "", false
		}
		return scan.name, arg, true
	}
	return "", "", false
}

func (v qScanView) Kind() data.Kind {
	if v.name == "avgs" {
		return data.KindF64
	}
	return v.source.Kind()
}

func (v qScanView) Len() int {
	return v.source.Len()
}

func (v qScanView) At(row int) (any, bool) {
	if row < 0 || row >= v.Len() {
		return nil, false
	}
	value, err := v.prefixAt(row)
	if err != nil {
		return nil, false
	}
	return value, true
}

func (v qScanView) Values() []any {
	out := make([]any, v.Len())
	for i := range out {
		value, ok := v.At(i)
		if !ok {
			out[i] = data.NullValue
			continue
		}
		out[i] = value
	}
	return out
}

func (v qScanView) Gather(indexes []int) data.Array {
	out := make([]any, len(indexes))
	for i, index := range indexes {
		value, ok := v.At(index)
		if !ok {
			out[i] = data.NullValue
			continue
		}
		out[i] = value
	}
	return inferQArray(out, v.Kind())
}

func (v qScanView) prefixAt(row int) (any, error) {
	switch v.name {
	case "sums":
		return qScanPrefixSum(v.source, row)
	case "prds":
		return qScanPrefixProduct(v.source, row)
	case "mins":
		return qScanPrefixExtrema(v.source, row, false)
	case "maxs":
		return qScanPrefixExtrema(v.source, row, true)
	case "avgs":
		return qScanPrefixAvg(v.source, row)
	default:
		return nil, fmt.Errorf("unknown scan view %q", v.name)
	}
}

func (v qScanView) terminal() (any, error) {
	if v.Len() == 0 {
		return data.NullValue, nil
	}
	switch v.name {
	case "sums":
		return sum(v.source)
	case "prds":
		return prd(v.source)
	case "mins":
		return minValue(v.source)
	case "maxs":
		return maxValue(v.source)
	case "avgs":
		return avg(v.source)
	default:
		return v.prefixAt(v.Len() - 1)
	}
}

func (v qCompareIndexStatsView) Kind() data.Kind { return data.KindI64 }

func (v qCompareIndexStatsView) Len() int { return int(v.count) }

func (v qCompareIndexStatsView) At(row int) (any, bool) {
	if row < 0 || row >= v.Len() {
		return nil, false
	}
	seen := 0
	for sourceRow := 0; sourceRow < v.source.Len(); sourceRow++ {
		value, ok := v.source.At(sourceRow)
		if !ok {
			return nil, false
		}
		if qCompareIndexStatsValueMatches(value, v.op, v.scalar) {
			if seen == row {
				return int64(sourceRow), true
			}
			seen++
		}
	}
	return nil, false
}

func (v qCompareIndexStatsView) Values() []any {
	out := make([]any, 0, v.Len())
	for sourceRow := 0; sourceRow < v.source.Len(); sourceRow++ {
		value, ok := v.source.At(sourceRow)
		if !ok {
			panic(fmt.Sprintf("q compare index view source row %d out of range", sourceRow))
		}
		if qCompareIndexStatsValueMatches(value, v.op, v.scalar) {
			out = append(out, int64(sourceRow))
		}
	}
	return out
}

func (v qCompareIndexStatsView) Gather(indexes []int) data.Array {
	out := make([]int64, len(indexes))
	for i, index := range indexes {
		value, ok := v.At(index)
		if !ok {
			panic(fmt.Sprintf("q compare index view gather row %d out of range", index))
		}
		out[i] = value.(int64)
	}
	return data.NewI64(out)
}

func qCompareIndexStatsValueMatches(value any, op data.Op, scalar any) bool {
	cmp, ok := qCompareIndexStatsCompare(value, scalar)
	if !ok {
		return false
	}
	return qBoolCompareDataOp(op, cmp == 0, cmp)
}

func qCompareIndexStatsCompare(left, right any) (int, bool) {
	if l, ok := integerValue(left); ok {
		if r, ok := integerValue(right); ok {
			return compareInt64(l, r), true
		}
	}
	if l, ok := numeric(left); ok {
		if r, ok := numeric(right); ok {
			return compareFloat(l, r), true
		}
	}
	if l, ok := qComparableString(left); ok {
		if r, ok := qComparableString(right); ok {
			return strings.Compare(l, r), true
		}
	}
	return 0, false
}

func qComparableString(value any) (string, bool) {
	switch x := value.(type) {
	case string:
		return x, true
	case data.Symbol:
		return string(x), true
	default:
		return "", false
	}
}

func qBoolCompareDataOp(op data.Op, equal bool, cmp int) bool {
	switch op {
	case data.OpEQ:
		return equal
	case data.OpNE:
		return !equal
	case data.OpLT:
		return cmp < 0
	case data.OpLE:
		return cmp <= 0
	case data.OpGT:
		return cmp > 0
	case data.OpGE:
		return cmp >= 0
	default:
		return false
	}
}

func qScanPrefixSum(array data.Array, row int) (any, error) {
	totalI := int64(0)
	totalF := float64(0)
	hasFloat := false
	for i := 0; i <= row; i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("sums row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		if n, ok := integerValue(item); ok {
			totalI += n
			totalF += float64(n)
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF += float64(n)
		case float64:
			hasFloat = true
			totalF += n
		default:
			return nil, fmt.Errorf("sums expects a numeric vector")
		}
	}
	if hasFloat {
		return totalF, nil
	}
	return totalI, nil
}

func qScanPrefixProduct(array data.Array, row int) (any, error) {
	totalI := int64(1)
	totalF := float64(1)
	hasFloat := false
	seen := false
	for i := 0; i <= row; i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("prds row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		seen = true
		if n, ok := integerValue(item); ok {
			totalI *= n
			totalF *= float64(n)
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF *= float64(n)
		case float64:
			hasFloat = true
			totalF *= n
		default:
			return nil, fmt.Errorf("prds expects a numeric vector")
		}
	}
	if !seen {
		return data.NullValue, nil
	}
	if hasFloat {
		return totalF, nil
	}
	return totalI, nil
}

func qScanPrefixExtrema(array data.Array, row int, maximum bool) (any, error) {
	var best any
	hasBest := false
	for i := 0; i <= row; i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("running extrema row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		if !hasBest {
			best = item
			hasBest = true
			continue
		}
		cmp, err := compareOrdered(item, best)
		if err != nil {
			return nil, err
		}
		if (!maximum && cmp < 0) || (maximum && cmp > 0) {
			best = item
		}
	}
	if !hasBest {
		return data.NullValue, nil
	}
	return best, nil
}

func qScanPrefixAvg(array data.Array, row int) (any, error) {
	total := float64(0)
	count := 0
	for i := 0; i <= row; i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("avgs row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return nil, fmt.Errorf("avgs expects a numeric vector")
		}
		total += n
		count++
	}
	if count == 0 {
		return data.NullValue, nil
	}
	return total / float64(count), nil
}

func cloneEnv(env map[string]any) map[string]any {
	if len(env) == 0 {
		return make(map[string]any)
	}
	out := make(map[string]any, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}

func (s *EvalState) currentNamespace() string {
	if s.namespace == "" {
		return "."
	}
	return s.namespace
}

func (s *EvalState) resolveAssignmentName(name string) string {
	if strings.HasPrefix(name, ".") {
		return name
	}
	if s.currentNamespace() == "." {
		return name
	}
	return s.currentNamespace() + "." + name
}

func (s *EvalState) lookupName(name string) (any, bool) {
	if strings.HasPrefix(name, ".") {
		v, ok := s.env[name]
		return v, ok
	}
	if s.currentNamespace() != "." {
		v, ok := s.env[s.currentNamespace()+"."+name]
		return v, ok
	}
	v, ok := s.env[name]
	return v, ok
}

func (s *EvalState) visibleEnv() map[string]any {
	if s.currentNamespace() == "." {
		out := make(map[string]any)
		for name, value := range s.env {
			if strings.HasPrefix(name, ".") {
				continue
			}
			out[name] = value
		}
		return out
	}
	prefix := s.currentNamespace() + "."
	out := make(map[string]any)
	for name, value := range s.env {
		if strings.HasPrefix(name, prefix) {
			out[strings.TrimPrefix(name, prefix)] = value
		}
	}
	return out
}

func (s *EvalState) eval(src string) (any, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty q expression")
	}
	if out, ok, err := s.evalConditionalSpecialForm(src); ok || err != nil {
		return out, err
	}
	if out, ok, err := s.evalControlSpecialForm(src); ok || err != nil {
		return out, err
	}
	if plan := s.qPipelinePlan(src); plan.kind != qPipelineInvalid {
		if out, handled, err := s.evalQPipelinePlan(&plan); err != nil || handled {
			return out, err
		}
	}
	if value, ok, err := s.evalSystemCommand(src); ok {
		return value, err
	}
	if isQAssignmentName(src) {
		if v, ok := s.lookupName(src); ok {
			return v, nil
		}
	}
	if src == "()" {
		return data.NewAny(nil), nil
	}
	if value, ok, err := evalZNamespaceValue(src); ok {
		return value, err
	}
	if v, ok, err := s.evalTableLiteral(src); ok || err != nil {
		return v, err
	}
	if fn, ok := parseDyadicAdverbFunction(src); ok {
		return fn, nil
	}
	if enclosed(src, '(', ')') {
		inner := strings.TrimSpace(src[1 : len(src)-1])
		if parts := splitTopLevelDelim(inner, ';'); len(parts) > 1 {
			items := make([]any, 0, len(parts))
			for _, part := range parts {
				v, err := s.eval(part)
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
			return inferQArray(items), nil
		}
		return s.eval(inner)
	}
	if value, ok, err := parseTemporalAtomOrVector(src); ok {
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	if value, ok, err := parseQBoolLiteral(src); ok || err != nil {
		return value, err
	}
	if strings.HasPrefix(src, "set ") {
		return s.evalSetPrefix(strings.TrimSpace(src[len("set "):]))
	}
	if leftExpr, rightExpr, ok := splitPathStyleSet(src); ok {
		return s.evalSet(leftExpr, rightExpr)
	}
	if strings.HasPrefix(src, "`:") {
		syms, err := parseSymbolList(src)
		if err == nil && len(syms) == 1 {
			return syms[0], nil
		}
	}
	if strings.HasPrefix(src, "@[") {
		return s.evalAmend(src)
	}
	if value, ok, err := s.evalListStringFunctionCall(src); ok || err != nil {
		return value, err
	}
	if value, ok, err := s.evalApplyIndexForm(src); ok || err != nil {
		return value, err
	}
	if callableExpr, rightExpr, ok := splitTopLevelWord(src, "each"); ok {
		callable, err := s.eval(callableExpr)
		if err != nil {
			return nil, err
		}
		if !isCallable(callable) {
			return nil, fmt.Errorf("left side of callable each is not callable")
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return s.evalCallableAdverb(callable, "'", right)
	}
	if funcs, ok := parseUnaryComposition(src); ok {
		return qComposition{funcs: funcs}, nil
	}
	if callableExpr, adverb, rightExpr, ok := s.findCallablePostfixAdverb(src); ok {
		callable, err := s.eval(callableExpr)
		if err != nil {
			return nil, err
		}
		if !isCallable(callable) {
			return nil, fmt.Errorf("left side of callable adverb is not callable")
		}
		if strings.HasPrefix(rightExpr, "[") && enclosed(rightExpr, '[', ']') {
			return s.applyCallableIndex(qCallableAdverb{fn: callable, adverb: adverb}, strings.TrimSpace(rightExpr[1:len(rightExpr)-1]))
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return s.evalCallableAdverb(callable, adverb, right)
	}
	if collectionExpr, indexExpr, ok := findPostfixIndex(src); ok {
		if fn, ok := parseDyadicAdverbFunction(collectionExpr); ok {
			return s.applyCallableIndex(fn, indexExpr)
		}
		collection, err := s.eval(collectionExpr)
		if err != nil {
			return nil, err
		}
		if isCallable(collection) {
			return s.applyCallableIndex(collection, indexExpr)
		}
		index, err := s.eval(indexExpr)
		if err != nil {
			return nil, err
		}
		return indexValue(collection, index)
	}
	if collectionExpr, symbolExpr, ok := findPostfixSymbolLookup(src); ok {
		collection, err := s.eval(collectionExpr)
		if err != nil {
			return nil, err
		}
		key, err := s.eval(symbolExpr)
		if err != nil {
			return nil, err
		}
		return indexValue(collection, key)
	}
	if lambdaSrc, adverb, ok := findLambdaAdverbFunction(src); ok {
		return qCallableAdverb{
			fn:     qLambda{body: strings.TrimSpace(lambdaSrc[1 : len(lambdaSrc)-1]), env: cloneEnv(s.env), namespace: s.namespace},
			adverb: adverb,
		}, nil
	}
	if callableExpr, adverb, ok := s.findCallableAdverbFunction(src); ok {
		callable, err := s.eval(callableExpr)
		if err != nil {
			return nil, err
		}
		if !isCallable(callable) {
			return nil, fmt.Errorf("left side of callable adverb is not callable")
		}
		return qCallableAdverb{fn: callable, adverb: adverb}, nil
	}
	if lambdaSrc, adverb, rightExpr, ok := findLambdaAdverb(src); ok {
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return s.evalCallableAdverb(qLambda{body: strings.TrimSpace(lambdaSrc[1 : len(lambdaSrc)-1]), env: cloneEnv(s.env), namespace: s.namespace}, adverb, right)
	}
	if enclosed(src, '{', '}') {
		return qLambda{body: strings.TrimSpace(src[1 : len(src)-1]), env: cloneEnv(s.env), namespace: s.namespace}, nil
	}
	if strings.HasPrefix(src, "?") {
		v, err := s.eval(strings.TrimSpace(src[1:]))
		if err != nil {
			return nil, err
		}
		return distinct(v)
	}
	if strings.HasPrefix(src, "lookup ") {
		return s.evalLookup(strings.TrimSpace(src[len("lookup "):]))
	}
	if strings.HasPrefix(src, "get ") {
		return s.evalGet(strings.TrimSpace(src[len("get "):]))
	}
	if strings.HasPrefix(src, "hopen ") {
		return s.evalHopen(strings.TrimSpace(src[len("hopen "):]))
	}
	if strings.HasPrefix(src, "+/") {
		right := strings.TrimSpace(src[2:])
		if right == "" {
			return qAdverbFunction{verb: "+", adverb: "/"}, nil
		}
		if out, handled, err := s.tryEvalSumFby(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalSumWhereGatherReduce(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalSumWhereCompare(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalSequenceTransformSum(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalSumDeltas(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalSortIndexSum(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalTypedDyadicFloatSum(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalTypedUnarySum(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalTypedMovingWindowSum(right); err != nil || handled {
			return out, err
		}
		v, err := s.eval(right)
		if err != nil {
			return nil, err
		}
		return sum(v)
	}
	if leftExpr, rightExpr, ok := splitTopLevelWord(src, "fby"); ok {
		return s.evalFby(leftExpr, rightExpr)
	}
	if strings.HasPrefix(src, "+\\") {
		if strings.TrimSpace(src[2:]) == "" {
			return qAdverbFunction{verb: "+", adverb: "\\"}, nil
		}
		v, err := s.eval(strings.TrimSpace(src[2:]))
		if err != nil {
			return nil, err
		}
		return sums(v)
	}
	if fn, ok := lookupDyadicVerbFunc(src); ok {
		return qDyadicFunction{name: strings.TrimSpace(src), fn: fn}, nil
	}
	if fn, ok := lookupUnaryVerb(src); ok {
		return qUnaryFunction{name: strings.TrimSpace(src), fn: fn}, nil
	}
	if expr, ok := findAdverb(src); ok {
		return s.evalAdverb(expr)
	}
	if strings.HasPrefix(src, "flip ") {
		v, err := s.eval(strings.TrimSpace(src[len("flip "):]))
		if err != nil {
			return nil, err
		}
		return flip(v)
	}
	if strings.HasPrefix(src, "enlist ") {
		v, err := s.eval(strings.TrimSpace(src[len("enlist "):]))
		if err != nil {
			return nil, err
		}
		return data.NewAny([]any{v}), nil
	}
	if strings.HasPrefix(src, "til ") {
		return s.evalTil(strings.TrimSpace(src[len("til "):]))
	}
	if strings.HasPrefix(src, "take ") {
		return s.evalTake(strings.TrimSpace(src[len("take "):]))
	}
	if strings.HasPrefix(src, "drop ") {
		return s.evalDrop(strings.TrimSpace(src[len("drop "):]))
	}
	if strings.HasPrefix(src, "count ") {
		if out, handled, err := s.tryEvalCountFrameMetadata(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountFlip(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountGroup(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountDistinct(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountWhereCompare(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountWhereLike(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountWhereIn(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountWhereNull(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountWhereMask(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountFby(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountSequencePrimitive(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountReverse(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountLengthPreservingTransform(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountRunningScan(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountXbar(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
	}
	if strings.HasPrefix(src, "last ") {
		lastInput := strings.TrimSpace(src[len("last "):])
		if out, handled, err := s.tryEvalLastDyadicTerminal(lastInput); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalLastScan(lastInput); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalLastCallableScan(lastInput); err != nil || handled {
			return out, err
		}
	}
	for _, prefix := range []struct {
		word string
		fn   func(any) (any, error)
	}{
		{"where ", where},
		{"not ", notValue},
		{"null ", nullValue},
		{"raze ", raze},
		{"keys ", keys},
		{"key ", keys},
		{"value ", value},
		{"cols ", cols},
		{"meta ", meta},
		{"attr ", attrValue},
		{"codes ", enumCodes},
		{"domain ", enumDomainValues},
		{"group ", group},
		{"ungroup ", ungroup},
		{"type ", typeOf},
		{"string ", stringValue},
		{"lower ", lowerValue},
		{"upper ", upperValue},
		{"trim ", qTrimValue},
		{"ltrim ", qLTrimValue},
		{"rtrim ", qRTrimValue},
		{"ssr ", qSSRValue},
		{"count ", count},
		{"first ", first},
		{"last ", last},
		{"sum ", sum},
		{"avg ", avg},
		{"var ", varValue},
		{"dev ", devValue},
		{"svar ", svarValue},
		{"sdev ", sdevValue},
		{"wsum ", wsumUnaryValue},
		{"med ", medValue},
		{"min ", minValue},
		{"max ", maxValue},
		{"prd ", prd},
		{"distinct ", distinct},
		{"reverse ", reverse},
		{"prior ", prev},
		{"prev ", prev},
		{"next ", nextValue},
		{"deltas ", deltas},
		{"fills ", fills},
		{"differ ", differ},
		{"ratios ", ratios},
		{"asc ", asc},
		{"desc ", desc},
		{"iasc ", iasc},
		{"idesc ", idesc},
		{"rank ", rank},
		{"neg ", negValue},
		{"abs ", absValue},
		{"sqrt ", sqrtValue},
		{"log ", logValue},
		{"exp ", expValue},
		{"sin ", sinValue},
		{"cos ", cosValue},
		{"tan ", tanValue},
		{"asin ", asinValue},
		{"acos ", acosValue},
		{"atan ", atanValue},
		{"reciprocal ", reciprocalValue},
		{"signum ", signumValue},
		{"floor ", floorValue},
		{"ceiling ", ceilingValue},
		{"inv ", matrixInverseValue},
		{"sums ", sums},
		{"prds ", prds},
		{"mins ", mins},
		{"maxs ", maxs},
		{"avgs ", avgs},
		{"all ", allValue},
		{"any ", anyValue},
	} {
		if strings.HasPrefix(src, prefix.word) {
			arg := strings.TrimSpace(src[len(prefix.word):])
			if prefix.word == "where " {
				if out, ok, err := s.evalWhereCompare(arg); ok || err != nil {
					return out, err
				}
				if out, ok, err := s.evalWhereIn(arg); ok || err != nil {
					return out, err
				}
			}
			if prefix.word == "sum " {
				if out, handled, err := s.tryEvalSumDeltas(arg); err != nil || handled {
					return out, err
				}
			}
			v, err := s.eval(arg)
			if err != nil {
				return nil, err
			}
			return prefix.fn(v)
		}
	}
	if kind, text, ok := parseTemporalToken(src); ok {
		return ParseTemporal(kind, text)
	}
	if looksLikeTemporalVector(src) {
		return parseAtomOrVector(src)
	}
	for _, op := range []string{"<>", "<=", ">="} {
		if leftExpr, rightExpr, ok := splitTopLevelOperator(src, op); ok {
			left, err := s.eval(leftExpr)
			if err != nil {
				return nil, err
			}
			right, err := s.eval(rightExpr)
			if err != nil {
				return nil, err
			}
			return applyCompositeDyadic(op, left, right)
		}
	}
	if leftExpr, rightExpr, ok := splitTopLevelOperator(src, "~"); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return matchValue(left, right), nil
	}
	if leftExpr, rightExpr, ok := splitTopLevelOperator(src, "?"); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return findValue(left, right)
	}
	if leftExpr, rightExpr, ok := findPostfixLookup(src); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return dictLookup(left, right)
	}
	for _, op := range []struct {
		word string
		fn   func(any, any) (any, error)
	}{
		{"bin", bin},
		{"binr", binr},
		{"xbar", xbar},
		{"xrank", xrank},
		{"msum", msum},
		{"mavg", mavg},
		{"mcount", mcount},
		{"mmin", mmin},
		{"mmax", mmax},
		{"mdev", mdevValue},
		{"ema", emaValue},
		{"xprev", xprev},
		{"xrank", xrank},
		{"mmu", matrixMultiplyValue},
		{"xcols", xcols},
		{"xkey", xkey},
		{"xgroup", xgroup},
		{"xasc", xasc},
		{"xdesc", xdesc},
		{"rotate", rotateValue},
		{"cut", qCutValue},
		{"sublist", qSublistValue},
		{"cross", qCrossValue},
		{"ss", qSSValue},
		{"ssr", qSSRWithSourceValue},
		{"sv", qSVValue},
		{"vs", qVSValue},
		{"plus", dyadicVerbFunc('+')},
		{"minus", dyadicVerbFunc('-')},
		{"times", dyadicVerbFunc('*')},
		{"divide", dyadicVerbFunc('%')},
		{"div", dyadicVerbFunc('d')},
		{"mod", modValue},
		{"xexp", xexpValue},
		{"xlog", xlogValue},
		{"fill", dyadicVerbFunc('^')},
		{"match", func(left, right any) (any, error) { return matchValue(left, right), nil }},
		{"like", likeValue},
		{"equal", dyadicVerbFunc('=')},
		{"equals", dyadicVerbFunc('=')},
		{"less", dyadicVerbFunc('<')},
		{"more", dyadicVerbFunc('>')},
		{"greater", dyadicVerbFunc('>')},
		{"min", dyadicVerbFunc('m')},
		{"max", dyadicVerbFunc('M')},
		{"wavg", wavg},
		{"wsum", wsumValue},
		{"cov", covValue},
		{"scov", scovValue},
		{"cor", corValue},
		{"left", dyadicVerbFunc('L')},
		{"right", dyadicVerbFunc('R')},
		{"within", within},
		{"in", membership},
		{"and", logicalAnd},
		{"or", logicalOr},
	} {
		if leftExpr, rightExpr, ok := splitTopLevelWord(src, op.word); ok {
			left, err := s.eval(leftExpr)
			if err != nil {
				return nil, err
			}
			right, err := s.eval(rightExpr)
			if err != nil {
				return nil, err
			}
			return op.fn(left, right)
		}
	}
	for _, op := range []struct {
		word string
		fn   func(any, any) (any, error)
	}{
		{"intersect", inter},
		{"except", except},
		{"union", union},
		{"inter", inter},
	} {
		if leftExpr, rightExpr, ok := splitTopLevelWord(src, op.word); ok {
			left, err := s.eval(leftExpr)
			if err != nil {
				return nil, err
			}
			right, err := s.eval(rightExpr)
			if err != nil {
				return nil, err
			}
			return op.fn(left, right)
		}
	}
	if hash := findTopLevel(src, "#"); hash >= 0 {
		if marker, ok := parseAttributeMarker(strings.TrimSpace(src[:hash])); ok {
			v, err := s.eval(strings.TrimSpace(src[hash+1:]))
			if err != nil {
				return nil, err
			}
			return attributeVector(marker, v)
		}
		leftSrc := strings.TrimSpace(src[:hash])
		v, err := s.eval(strings.TrimSpace(src[hash+1:]))
		if err != nil {
			return nil, err
		}
		leftValue, err := s.eval(leftSrc)
		if err != nil {
			return nil, err
		}
		if _, ok := leftValue.(data.Array); ok {
			return reshapeValue(leftValue, v)
		}
		n, ok := integerValue(leftValue)
		if !ok || int64(int(n)) != n {
			return nil, fmt.Errorf("# left operand must be an integer count")
		}
		return take(int(n), v)
	}
	if underscore := findTopLevel(src, "_"); underscore >= 0 {
		left, err := s.eval(strings.TrimSpace(src[:underscore]))
		if err != nil {
			return nil, err
		}
		right, err := s.eval(strings.TrimSpace(src[underscore+1:]))
		if err != nil {
			return nil, err
		}
		return cutOrDrop(left, right)
	}
	if dollar := findTopLevel(src, "$"); dollar >= 0 {
		domain, err := s.evalCastDomain(strings.TrimSpace(src[:dollar]))
		if err != nil {
			return nil, err
		}
		values, err := s.eval(strings.TrimSpace(src[dollar+1:]))
		if err != nil {
			return nil, err
		}
		return castOrEnum(domain, values)
	}
	if bang := findTopLevel(src, "!"); bang >= 0 {
		keys, err := s.eval(strings.TrimSpace(src[:bang]))
		if err != nil {
			return nil, err
		}
		values, err := s.eval(strings.TrimSpace(src[bang+1:]))
		if err != nil {
			return nil, err
		}
		if table, ok, err := keyedTableByCount(keys, values); ok || err != nil {
			return table, err
		}
		if keyed, ok, err := keyedTable(keys, values); ok || err != nil {
			return keyed, err
		}
		return dict(keys, values)
	}
	if leftExpr, rightExpr, ok := findPostfixSymbolLookup(src); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return dictLookup(left, right)
	}
	if parts := splitTopLevelDelim(src, ';'); len(parts) > 1 {
		out := make([]any, len(parts))
		for i, part := range parts {
			v, err := s.eval(part)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return data.NewAny(out), nil
	}
	if strings.HasPrefix(src, "`:") {
		return data.Symbol(src[1:]), nil
	}
	if looksLikeLiteralVector(src) {
		return parseAtomOrVector(src)
	}
	if out, handled, err := s.tryEvalWhereCompareCountSum(src); err != nil || handled {
		return out, err
	}
	if out, handled, err := s.tryEvalScalarAddChain(src); err != nil || handled {
		return out, err
	}
	if out, handled, err := s.tryEvalFirstLastDyadic(src); err != nil || handled {
		return out, err
	}
	if strings.HasPrefix(src, "flip ") {
		v, err := s.eval(strings.TrimSpace(src[len("flip "):]))
		if err != nil {
			return nil, err
		}
		return flip(v)
	}
	if idx, op, ok := findDyadic(src); ok {
		left, err := s.eval(strings.TrimSpace(src[:idx]))
		if err != nil {
			return nil, err
		}
		right, err := s.eval(strings.TrimSpace(src[idx+1:]))
		if err != nil {
			return nil, err
		}
		return applyDyadic(op, left, right)
	}
	if strings.HasPrefix(src, "`") {
		syms, err := parseSymbolList(src)
		if err == nil {
			if len(syms) == 1 {
				return syms[0], nil
			}
			values := make([]string, len(syms))
			for i, sym := range syms {
				values[i] = string(sym)
			}
			return data.NewSymbols(values), nil
		}
	}
	if value, ok, err := s.evalNameVector(src); ok || err != nil {
		return value, err
	}
	if value, ok, err := s.evalParsedValueExpr(src); ok || err != nil {
		return value, err
	}
	return parseAtomOrVector(src)
}

func (s *EvalState) evalNameVector(src string) (any, bool, error) {
	parts := splitTopLevelFields(src)
	if len(parts) < 2 {
		return nil, false, nil
	}
	values := make([]any, len(parts))
	for i, part := range parts {
		if !isQAssignmentName(part) {
			return nil, false, nil
		}
		value, ok := s.lookupName(part)
		if !ok {
			return nil, false, nil
		}
		values[i] = value
	}
	array, err := evalValueVector(values)
	return array, true, err
}

func (s *EvalState) evalParsedValueExpr(src string) (any, bool, error) {
	expr, ok, err := s.cachedValueExpr(src)
	if err != nil || !ok {
		return nil, false, nil
	}
	value, err := s.evalValueExpr(expr)
	if err != nil {
		if isUnsupportedEvalValueExpr(err) {
			return nil, false, nil
		}
		return nil, true, err
	}
	return value, true, nil
}

func (s *EvalState) cachedValueExpr(src string) (Expr, bool, error) {
	if s.valueExprCache != nil {
		if expr, ok := s.valueExprCache[src]; ok {
			return expr, true, nil
		}
	}
	expr, ok, err := parseValueExpr(src)
	if err != nil || !ok {
		return nil, ok, err
	}
	if s.valueExprCache == nil {
		s.valueExprCache = make(map[string]Expr, 32)
	} else if len(s.valueExprCache) >= 512 {
		s.valueExprCache = make(map[string]Expr, 32)
	}
	s.valueExprCache[src] = expr
	return expr, true, nil
}

type unsupportedEvalValueExpr struct {
	expr Expr
}

func (e unsupportedEvalValueExpr) Error() string {
	return fmt.Sprintf("unsupported q value expression %T", e.expr)
}

func isUnsupportedEvalValueExpr(err error) bool {
	_, ok := err.(unsupportedEvalValueExpr)
	return ok
}

func (s *EvalState) evalValueExpr(expr Expr) (any, error) {
	switch x := expr.(type) {
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		return value, err
	case String:
		return x.Value, nil
	case Symbol:
		return data.Symbol(x.Name), nil
	case Bool:
		return x.Value, nil
	case Null:
		return data.NullValue, nil
	case Temporal:
		return parseQTemporal(x.Kind, x.Text)
	case TypedNull:
		return data.NullForKind(data.Kind(x.Kind)), nil
	case Ident:
		if value, ok := s.lookupName(x.Name); ok {
			return value, nil
		}
		if fn, ok := lookupUnaryVerb(x.Name); ok {
			return qUnaryFunction{name: x.Name, fn: fn}, nil
		}
		if fn, ok := lookupDyadicVerbFunc(x.Name); ok {
			return qDyadicFunction{name: x.Name, fn: fn}, nil
		}
		return nil, unsupportedEvalValueExpr{expr: expr}
	case Vector:
		values := make([]any, len(x.Items))
		for i, item := range x.Items {
			value, err := s.evalValueExpr(item)
			if err != nil {
				return nil, err
			}
			values[i] = value
		}
		return evalValueVector(values)
	case DictExpr:
		keys, err := s.evalValueExpr(x.Keys)
		if err != nil {
			return nil, err
		}
		values, err := s.evalValueExpr(x.Values)
		if err != nil {
			return nil, err
		}
		if table, ok, err := keyedTableByCount(keys, values); ok || err != nil {
			return table, err
		}
		if keyed, ok, err := keyedTable(keys, values); ok || err != nil {
			return keyed, err
		}
		return dict(keys, values)
	case IndexExpr:
		collection, err := s.evalValueExpr(x.Expr)
		if err != nil {
			return nil, err
		}
		index, err := s.evalValueExpr(x.Index)
		if err != nil {
			return nil, err
		}
		if isCallable(collection) {
			return s.applyCallable(collection, []any{index})
		}
		return indexValue(collection, index)
	case Binary:
		left, err := s.evalValueExpr(x.Left)
		if err != nil {
			return nil, err
		}
		right, err := s.evalValueExpr(x.Right)
		if err != nil {
			return nil, err
		}
		return evalValueBinary(x.Op, left, right)
	case Call:
		return s.evalValueCall(x)
	case Flip:
		frame, err := lowerStaticFrame(x)
		if err != nil {
			return nil, err
		}
		if len(x.Keys) == 0 {
			return frame, nil
		}
		keys := make([]data.Symbol, len(x.Keys))
		for i, column := range x.Keys {
			keys[i] = data.Symbol(column.Name)
		}
		return data.KeyBy(frame, keys...)
	default:
		return nil, unsupportedEvalValueExpr{expr: expr}
	}
}

func (s *EvalState) evalValueCall(expr Call) (any, error) {
	arg, err := s.evalValueExpr(expr.Arg)
	if err != nil {
		return nil, err
	}
	if fn, ok := lookupUnaryVerb(expr.Func); ok {
		return fn(arg)
	}
	if fn, ok := lookupDyadicVerbFunc(expr.Func); ok {
		args, err := vectorValues(arg)
		if err != nil {
			return nil, unsupportedEvalValueExpr{expr: expr}
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s expects 2 arguments, got %d", expr.Func, len(args))
		}
		return fn(args[0], args[1])
	}
	return nil, unsupportedEvalValueExpr{expr: expr}
}

func evalValueBinary(op string, left, right any) (any, error) {
	switch op {
	case "<>", "<=", ">=":
		return applyCompositeDyadic(op, left, right)
	case "~":
		return matchValue(left, right), nil
	case "#":
		if _, ok := left.(data.Array); ok {
			return reshapeValue(left, right)
		}
		n, ok := integerValue(left)
		if !ok || int64(int(n)) != n {
			return nil, fmt.Errorf("# left operand must be an integer count")
		}
		return take(int(n), right)
	}
	if op == "-" && isNumericZero(left) {
		if value, ok := negateTypedNumeric(right); ok {
			return value, nil
		}
	}
	if fn, ok := lookupDyadicVerbFunc(op); ok {
		return fn(left, right)
	}
	return nil, unsupportedEvalValueExpr{expr: Binary{Op: op}}
}

func isNumericZero(v any) bool {
	switch x := v.(type) {
	case int:
		return x == 0
	case int16:
		return x == 0
	case int32:
		return x == 0
	case int64:
		return x == 0
	case float32:
		return x == 0
	case float64:
		return x == 0
	default:
		return false
	}
}

func negateTypedNumeric(v any) (any, bool) {
	switch x := v.(type) {
	case int16:
		return -x, true
	case int32:
		return -x, true
	case float32:
		return -x, true
	default:
		return nil, false
	}
}

func evalValueVector(values []any) (data.Array, error) {
	kinds := make([]data.Kind, 0, len(values))
	temporalKinds := make([]data.Kind, len(values))
	hasTemporal := false
	for i, value := range values {
		kind := qKindOfValue(value)
		if kind != "" {
			kinds = append(kinds, kind)
		}
		if kind == "" {
			continue
		}
		if isTemporalDataKind(kind) {
			temporalKinds[i] = kind
			hasTemporal = true
		}
	}
	if hasTemporal {
		for i, value := range values {
			if data.IsNull(value) {
				continue
			}
			if temporalKindOfValue(value) == "" {
				return nil, fmt.Errorf("mixed temporal and non-temporal vectors are not supported")
			}
			if temporalKinds[i] == "" {
				temporalKinds[i] = temporalKindOfValue(value)
			}
		}
		kind, err := coerceTemporalVectorValues(values, temporalKinds)
		if err != nil {
			return nil, err
		}
		column, err := data.NewColumnWithKind("_", kind, values)
		if err != nil {
			return nil, err
		}
		return column.Data, nil
	}
	return inferQArray(values, kinds...), nil
}

func isTemporalDataKind(kind data.Kind) bool {
	switch kind {
	case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan,
		data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		return true
	default:
		return false
	}
}

func (s *EvalState) evalSystemCommand(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "\\") {
		return nil, false, nil
	}
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return nil, true, fmt.Errorf("empty q system command")
	}
	cmd := fields[0]
	switch cmd {
	case "\\p":
		if len(fields) == 1 {
			return s.port, true, nil
		}
		if len(fields) != 2 {
			return nil, true, fmt.Errorf(`q system command \p expects zero or one numeric port argument`)
		}
		port, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || port < 0 || port > 65535 {
			return nil, true, fmt.Errorf(`q system command \p expects a port in range 0..65535`)
		}
		s.port = port
		return s.port, true, nil
	case "\\v":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \v expects no arguments`)
		}
		return s.systemNames(qSystemVariables), true, nil
	case "\\f":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \f expects no arguments`)
		}
		return s.systemNames(qSystemFunctions), true, nil
	case "\\a":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \a expects no arguments`)
		}
		return s.systemNames(qSystemTables), true, nil
	case "\\b":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \b expects no arguments`)
		}
		return data.NewSymbols(nil), true, nil
	case "\\d":
		if len(fields) == 1 {
			return data.Symbol(s.currentNamespace()), true, nil
		}
		if len(fields) != 2 {
			return nil, true, fmt.Errorf(`q system command \d expects zero or one namespace argument`)
		}
		if !isQNamespaceName(fields[1]) {
			return nil, true, fmt.Errorf(`q system command \d expects . or a single-level namespace like .foo`)
		}
		s.namespace = fields[1]
		return data.Symbol(s.currentNamespace()), true, nil
	case "\\w":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \w expects no arguments`)
		}
		return systemWorkspaceStats(), true, nil
	case "\\l":
		if len(fields) != 2 {
			return nil, true, fmt.Errorf(`q system command \l expects one file path argument`)
		}
		out, err := s.loadScriptFile(fields[1])
		return out, true, err
	default:
		return nil, true, fmt.Errorf("unsupported q system command %q", cmd)
	}
}

func (s *EvalState) loadScriptFile(arg string) (any, error) {
	file := strings.TrimSpace(arg)
	if path, ok := qPathLiteralString(file); ok {
		file = path
	} else if strings.HasPrefix(file, "\"") && strings.HasSuffix(file, "\"") && len(file) >= 2 {
		file = strings.Trim(file, "\"")
	}
	if file == "" || strings.HasPrefix(file, ":") {
		return nil, fmt.Errorf(`q system command \l expects a local file path`)
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf(`q system command \l %q: %w`, file, err)
	}
	return s.evalScript(strings.TrimSpace(string(src)))
}

type qSystemNameKind int

const (
	qSystemVariables qSystemNameKind = iota
	qSystemFunctions
	qSystemTables
)

func (s *EvalState) systemNames(kind qSystemNameKind) data.Array {
	names := make([]string, 0, len(s.env))
	for name, value := range s.env {
		if s.currentNamespace() == "." {
			if strings.HasPrefix(name, ".") {
				continue
			}
		} else {
			prefix := s.currentNamespace() + "."
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			name = strings.TrimPrefix(name, prefix)
		}
		switch kind {
		case qSystemVariables:
			if isCallable(value) || isTableLikeValue(value) {
				continue
			}
		case qSystemFunctions:
			if !isCallable(value) {
				continue
			}
		case qSystemTables:
			if !isTableLikeValue(value) {
				continue
			}
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return data.NewSymbols(names)
}

func isTableLikeValue(v any) bool {
	switch v.(type) {
	case data.Frame, data.KeyedFrame:
		return true
	default:
		return false
	}
}

func systemWorkspaceStats() data.Array {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return data.NewI64([]int64{
		uint64ToInt64(stats.Alloc),
		uint64ToInt64(stats.TotalAlloc),
		uint64ToInt64(stats.Sys),
		uint64ToInt64(uint64(stats.NumGC)),
	})
}

func uint64ToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

func (s *EvalState) evalTableLiteral(src string) (any, bool, error) {
	if !looksLikeTableLiteral(src) {
		return nil, false, nil
	}
	expr, ok, err := parseValueExpr(src)
	if err != nil || !ok {
		return nil, ok, err
	}
	flipExpr, ok := expr.(Flip)
	if !ok {
		return nil, false, nil
	}
	frame, err := lowerStaticFrame(flipExpr)
	if err != nil {
		return nil, true, err
	}
	if len(flipExpr.Keys) == 0 {
		return frame, true, nil
	}
	keys := make([]data.Symbol, len(flipExpr.Keys))
	for i, column := range flipExpr.Keys {
		keys[i] = data.Symbol(column.Name)
	}
	keyed, err := data.KeyBy(frame, keys...)
	if err != nil {
		return nil, true, fmt.Errorf("keyed table literal: %w", err)
	}
	return keyed, true, nil
}

func looksLikeTableLiteral(src string) bool {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "(") {
		return false
	}
	for i := 1; i < len(src); i++ {
		if isQWhitespace(src[i]) {
			continue
		}
		return src[i] == '['
	}
	return false
}

func parseValueExpr(src string) (Expr, bool, error) {
	tokens, err := lex(src)
	if err != nil {
		return nil, true, err
	}
	p := parser{tokens: tokens}
	expr, err := p.parseExpr(0)
	if err != nil {
		return nil, true, err
	}
	if p.peek().kind != tokenEOF {
		return nil, false, nil
	}
	return expr, true, nil
}

func evalZNamespaceValue(src string) (any, bool, error) {
	if !strings.HasPrefix(src, ".z.") {
		return nil, false, nil
	}
	now := time.Now()
	utc := now.UTC()
	switch src {
	case ".z.P":
		return data.TimestampFromUnixNanos(utc.UnixNano()), true, nil
	case ".z.p":
		return data.TimestampFromUnixNanos(civilUnixNanos(now)), true, nil
	case ".z.D":
		return dateFromCivil(utc), true, nil
	case ".z.d":
		return dateFromCivil(now), true, nil
	case ".z.T":
		return timeFromCivil(utc), true, nil
	case ".z.t":
		return timeFromCivil(now), true, nil
	case ".z.Z":
		return data.DateTimeFromUnixNanos(civilUnixNanos(utc)), true, nil
	case ".z.z":
		return data.DateTimeFromUnixNanos(civilUnixNanos(now)), true, nil
	case ".z.i":
		return int64(os.Getpid()), true, nil
	case ".z.K":
		return int64(4), true, nil
	case ".z.k":
		return "leia-q", true, nil
	case ".z.o":
		return "leia", true, nil
	case ".z.q":
		return data.Symbol("leia"), true, nil
	default:
		return nil, true, fmt.Errorf("unsupported q .z namespace value %q", src)
	}
}

func dateFromCivil(t time.Time) data.Date {
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return data.DateFromDays(date.Unix() / 86400)
}

func timeFromCivil(t time.Time) data.Time {
	return data.TimeFromNanos(int64(t.Hour())*3600*1_000_000_000 +
		int64(t.Minute())*60*1_000_000_000 +
		int64(t.Second())*1_000_000_000 +
		int64(t.Nanosecond()))
}

func civilUnixNanos(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixNano()
}

func (s *EvalState) evalHopen(src string) (any, error) {
	target, err := s.eval(src)
	if err != nil {
		return nil, err
	}
	targetName, ok := qIPCTargetName(target)
	if !ok {
		return nil, fmt.Errorf("hopen expects a loopback target string or symbol")
	}
	switch targetName {
	case "loopback", ":loopback", "local", ":local", "memory", ":memory":
		return &qIPCHandle{
			target:  "loopback",
			session: NewEvalState(nil),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported q IPC target %q; only in-process loopback is supported", targetName)
	}
}

func (s *EvalState) evalGet(src string) (any, error) {
	path, ok := qPathLiteralString(src)
	if !ok {
		value, err := s.eval(src)
		if err != nil {
			return nil, err
		}
		path, ok = qPathString(value)
		if !ok {
			return nil, fmt.Errorf("get expects a q path symbol like `:/path")
		}
	}
	frame, err := data.LoadPartitionedFrameDir(path, nil)
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", path, err)
	}
	return frame, nil
}

func (s *EvalState) evalSetPrefix(src string) (any, error) {
	pathExpr, valueExpr, ok := splitLeadingSetArg(src)
	if !ok {
		return nil, fmt.Errorf("set expects a q path symbol and frame value")
	}
	return s.evalSet(pathExpr, valueExpr)
}

func (s *EvalState) evalSet(pathExpr, valueExpr string) (any, error) {
	path, ok := qPathLiteralString(pathExpr)
	if !ok {
		value, err := s.eval(pathExpr)
		if err != nil {
			return nil, err
		}
		path, ok = qPathString(value)
		if !ok {
			return nil, fmt.Errorf("set expects a q path symbol like `:/path")
		}
	}
	value, err := s.eval(valueExpr)
	if err != nil {
		return nil, err
	}
	frame, ok := qStorageFrame(value)
	if !ok {
		return nil, fmt.Errorf("set %q expects a table value", path)
	}
	if err := data.SaveFrameDir(path, frame); err != nil {
		return nil, fmt.Errorf("set %q: %w", path, err)
	}
	return value, nil
}

func qStorageFrame(value any) (data.Frame, bool) {
	switch x := value.(type) {
	case data.Frame:
		return x, true
	case data.KeyedFrame:
		return x.Frame(), true
	default:
		return data.Frame{}, false
	}
}

func qPathLiteralString(src string) (string, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "`:") || strings.Contains(src[1:], "`") {
		return "", false
	}
	path := strings.TrimPrefix(src[1:], ":")
	if path == "" {
		return "", false
	}
	return path, true
}

func splitPathStyleSet(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "`:") {
		end := qSymbolLiteralEnd(src, 0)
		if end <= 0 || end >= len(src) {
			return "", "", false
		}
		rest := strings.TrimSpace(src[end:])
		if strings.HasPrefix(rest, "set") && wordBoundary(rest, 0, len("set")) {
			right := strings.TrimSpace(rest[len("set"):])
			if right != "" {
				return strings.TrimSpace(src[:end]), right, true
			}
		}
		return "", "", false
	}
	return splitTopLevelWord(src, "set")
}

func splitLeadingSetArg(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", false
	}
	end := 0
	switch {
	case strings.HasPrefix(src, "`"):
		end = qSymbolLiteralEnd(src, 0)
	case src[0] == '(':
		end = findMatchingDelimiter(src, 0, '(', ')') + 1
	default:
		for end < len(src) && !isQWhitespace(src[end]) {
			end++
		}
	}
	if end <= 0 || end >= len(src) {
		return "", "", false
	}
	right := strings.TrimSpace(src[end:])
	return strings.TrimSpace(src[:end]), right, right != ""
}

func qPathString(v any) (string, bool) {
	sym, ok := v.(data.Symbol)
	if !ok {
		return "", false
	}
	text := string(sym)
	if !strings.HasPrefix(text, ":") {
		return "", false
	}
	path := strings.TrimPrefix(text, ":")
	if path == "" {
		return "", false
	}
	return path, true
}

func isQWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func qIPCTargetName(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case data.Symbol:
		return string(x), true
	default:
		return "", false
	}
}

func enclosed(src string, open, close byte) bool {
	src = strings.TrimSpace(src)
	if len(src) < 2 || src[0] != open || src[len(src)-1] != close {
		return false
	}
	depth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == open {
			depth++
		} else if ch == close {
			depth--
			if depth == 0 && i != len(src)-1 {
				return false
			}
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func splitTopLevelDelim(src string, sep byte) []string {
	var parts []string
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	start := 0
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case sep:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				parts = append(parts, strings.TrimSpace(src[start:i]))
				start = i + 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	parts = append(parts, strings.TrimSpace(src[start:]))
	return parts
}

func splitQScriptStatements(src string) []string {
	parts := splitTopLevelDelim(src, ';')
	if len(parts) == 0 {
		return []string{strings.TrimSpace(src)}
	}
	return parts
}

func splitQBracketFormArgs(src string) []string {
	parts := splitTopLevelDelim(src, ';')
	if parts == nil {
		return []string{strings.TrimSpace(src)}
	}
	return parts
}

func splitTopLevel(src string, sep byte) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(src[start:i]))
				start = i + 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	parts = append(parts, strings.TrimSpace(src[start:]))
	return parts
}

func splitTopLevelAssignment(src string) (string, string, bool) {
	colon := findTopLevel(src, ":")
	if colon < 0 {
		return "", "", false
	}
	name := strings.TrimSpace(src[:colon])
	if !isQAssignmentName(name) {
		return "", "", false
	}
	rhs := strings.TrimSpace(src[colon+1:])
	return name, rhs, rhs != ""
}

func splitTopLevelAugmentedAssignment(src string) (string, string, string, bool) {
	colon := findTopLevel(src, ":")
	if colon <= 0 {
		return "", "", "", false
	}
	left := strings.TrimSpace(src[:colon])
	if len(left) < 2 {
		return "", "", "", false
	}
	op := left[len(left)-1:]
	if !strings.Contains("+-*%^&|", op) {
		return "", "", "", false
	}
	name := strings.TrimSpace(left[:len(left)-1])
	if !isQAssignmentName(name) {
		return "", "", "", false
	}
	rhs := strings.TrimSpace(src[colon+1:])
	return name, op, rhs, rhs != ""
}

func expandScriptVars(src string, vars map[string]any) string {
	if len(vars) == 0 {
		return strings.TrimSpace(src)
	}
	var out strings.Builder
	for i := 0; i < len(src); {
		switch src[i] {
		case '"':
			end := i + 1
			for end < len(src) {
				if src[end] == '\\' {
					end += 2
					continue
				}
				if src[end] == '"' {
					end++
					break
				}
				end++
			}
			out.WriteString(src[i:end])
			i = end
		case '`':
			end := i + 1
			for end < len(src) && !isQSymbolDelimiter(src[end]) {
				end++
			}
			out.WriteString(src[i:end])
			i = end
		case '{':
			end := findMatchingDelimiter(src, i, '{', '}')
			if end < 0 {
				out.WriteByte(src[i])
				i++
				continue
			}
			out.WriteString(src[i : end+1])
			i = end + 1
		default:
			if isQIdentStart(src[i]) && (i == 0 || src[i-1] != '.') {
				end := i + 1
				for end < len(src) && isQIdentRest(src[end]) {
					end++
				}
				name := src[i:end]
				if v, ok := vars[name]; ok {
					if isCallable(v) {
						out.WriteString(name)
					} else {
						out.WriteString(qExprLiteral(v))
					}
				} else {
					out.WriteString(name)
				}
				i = end
				continue
			}
			out.WriteByte(src[i])
			i++
		}
	}
	return strings.TrimSpace(out.String())
}

func findTopLevel(src string, chars string) int {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		default:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && strings.ContainsRune(chars, rune(ch)) && !isSign(src, i) {
				return i
			}
		}
	}
	return -1
}

func qSymbolLiteralEnd(src string, start int) int {
	if start < 0 || start >= len(src) || src[start] != '`' {
		return start + 1
	}
	pos := start + 1
	if pos < len(src) && src[pos] == ':' {
		pos++
		for pos < len(src) && !isQPathSymbolDelimiter(src[pos]) {
			pos++
		}
		return pos
	}
	for pos < len(src) && !isQSymbolDelimiter(src[pos]) {
		pos++
	}
	return pos
}

func splitTopLevelWord(src, word string) (string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i+len(word) <= len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		default:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && strings.HasPrefix(src[i:], word) && wordBoundary(src, i, len(word)) {
				left := strings.TrimSpace(src[:i])
				right := strings.TrimSpace(src[i+len(word):])
				return left, right, left != "" && right != ""
			}
		}
	}
	return "", "", false
}

func wordBoundary(src string, start, length int) bool {
	before := start == 0 || !isQIdentRest(src[start-1])
	afterAt := start + length
	after := afterAt >= len(src) || !isQIdentRest(src[afterAt])
	return before && after
}

func splitTopLevelOperator(src, op string) (string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i <= len(src)-len(op); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		default:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && strings.HasPrefix(src[i:], op) {
				if len(op) == 1 && (op == "+" || op == "-") && isSign(src, i) {
					continue
				}
				left := strings.TrimSpace(src[:i])
				right := strings.TrimSpace(src[i+len(op):])
				return left, right, left != "" && right != ""
			}
		}
	}
	return "", "", false
}

func findPostfixSymbolLookup(src string) (string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	candidate := -1
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '`':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && i > 0 {
				left := strings.TrimSpace(src[:i])
				if left == "" || strings.HasPrefix(left, "`") || isDyadicOp(left[len(left)-1]) || findTopLevel(left, "!") >= 0 || qPrefixExprTakesSymbolArgument(left) || qLeftLooksLikePrefixArgument(left) || !qCanReceivePostfixSymbolLookup(left) {
					continue
				}
				candidate = i
				i = len(src)
			}
		}
	}
	if candidate < 0 {
		return "", "", false
	}
	collection := strings.TrimSpace(src[:candidate])
	symbols := strings.TrimSpace(src[candidate:])
	if collection == "" || symbols == "" || !strings.HasPrefix(symbols, "`") {
		return "", "", false
	}
	if _, err := parseSymbolList(symbols); err != nil {
		return "", "", false
	}
	return collection, symbols, true
}

func qLeftLooksLikePrefixArgument(left string) bool {
	left = strings.TrimSpace(left)
	return strings.HasSuffix(left, " flip") || strings.HasSuffix(left, "!flip") || strings.HasSuffix(left, " enlist") || strings.HasSuffix(left, "!enlist")
}

func qCanReceivePostfixSymbolLookup(left string) bool {
	left = strings.TrimSpace(left)
	if left == "" {
		return false
	}
	if strings.HasSuffix(left, ")") || strings.HasSuffix(left, "]") {
		return true
	}
	return isQAssignmentName(left)
}

func qPrefixExprTakesSymbolArgument(left string) bool {
	left = strings.TrimSpace(left)
	if strings.HasPrefix(left, "flip ") || strings.HasPrefix(left, "enlist ") {
		return true
	}
	first := left
	if fields := strings.Fields(left); len(fields) > 0 {
		first = fields[0]
	}
	switch first {
	case "flip", "enlist", "keys", "key", "value", "cols", "meta", "attr", "type", "count",
		"where", "domain", "codes", "lookup", "hopen", "hsym", "system",
		"group", "string", "not", "iasc", "idesc", "rank", "asc", "desc", "min", "max",
		"sum", "avg", "sums", "prds", "mins", "maxs", "avgs", "distinct", "reverse", "prior", "prev", "next", "deltas", "fills", "differ",
		"first", "last", "take", "drop", "til", "lower", "upper", "null", "get":
		return true
	default:
		return false
	}
}

func findPostfixIndex(src string) (string, string, bool) {
	if !strings.HasSuffix(src, "]") {
		return "", "", false
	}
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	candidate := -1
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				candidate = i
			}
			bracketDepth++
		case ']':
			bracketDepth--
			if i == len(src)-1 && parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && candidate >= 0 {
				collection := strings.TrimSpace(src[:candidate])
				index := strings.TrimSpace(src[candidate+1 : len(src)-1])
				if collection == "" || !qCanReceivePostfixIndex(collection) {
					return "", "", false
				}
				return collection, index, index != "" || strings.Contains(src[candidate+1:len(src)-1], ";")
			}
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		}
	}
	return "", "", false
}

func qCanReceivePostfixIndex(collection string) bool {
	collection = strings.TrimSpace(collection)
	if collection == "" {
		return false
	}
	if _, _, ok := findDyadic(collection); ok {
		return false
	}
	if isQAssignmentName(collection) {
		return true
	}
	if _, ok := lookupUnaryVerb(collection); ok {
		return true
	}
	if _, ok := lookupDyadicVerbFunc(collection); ok {
		return true
	}
	if _, ok := parseDyadicAdverbFunction(collection); ok {
		return true
	}
	if qCanReceivePostfixCallableAdverbIndex(collection) {
		return true
	}
	if strings.HasPrefix(collection, "{") && strings.HasSuffix(collection, "}") {
		return true
	}
	return strings.HasSuffix(collection, ")") || strings.HasSuffix(collection, "]")
}

func qCanReceivePostfixCallableAdverbIndex(collection string) bool {
	for _, adverb := range []string{"\\:", "/:", "':", "'", "/", "\\"} {
		if !strings.HasSuffix(collection, adverb) {
			continue
		}
		base := strings.TrimSpace(collection[:len(collection)-len(adverb)])
		if base == "" {
			continue
		}
		if strings.HasPrefix(base, "{") && strings.HasSuffix(base, "}") {
			return true
		}
		if isQAssignmentName(base) {
			return true
		}
		if strings.HasSuffix(base, ")") || strings.HasSuffix(base, "]") {
			return true
		}
		if _, ok := lookupDyadicVerbFunc(base); ok {
			return true
		}
		if _, ok := lookupUnaryVerb(base); ok {
			return true
		}
	}
	return false
}

func findMatchingDelimiter(src string, start int, open, close byte) int {
	if start < 0 || start >= len(src) || src[start] != open {
		return -1
	}
	depth := 0
	inString := false
	for i := start; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isQAssignmentName(name string) bool {
	if isQBareName(name) {
		return true
	}
	if !strings.HasPrefix(name, ".") {
		return false
	}
	parts := strings.Split(name[1:], ".")
	if len(parts) != 2 {
		return false
	}
	return isQBareName(parts[0]) && isQBareName(parts[1])
}

func isQNamespaceName(name string) bool {
	if name == "." {
		return true
	}
	if !strings.HasPrefix(name, ".") {
		return false
	}
	return isQBareName(strings.TrimPrefix(name, "."))
}

func isQBareName(name string) bool {
	if name == "" || !isQIdentStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isQIdentRest(name[i]) {
			return false
		}
	}
	return true
}

func parseAttributeMarker(src string) (data.Symbol, bool) {
	src = strings.TrimSpace(src)
	if len(src) == 2 && src[0] == '`' && isQAttributeMarker(src[1]) {
		return data.Symbol(src[1:]), true
	}
	return "", false
}

func isQAttributeMarker(ch byte) bool {
	switch ch {
	case 's', 'g', 'p', 'u':
		return true
	default:
		return false
	}
}

func attributeVector(marker data.Symbol, v any) (qAttributedVector, error) {
	array, ok := v.(data.Array)
	if !ok {
		return qAttributedVector{}, fmt.Errorf("q attribute marker `%s expects a vector", marker)
	}
	return qAttributedVector{attribute: marker, vector: data.WithArrayAttribute(array, marker)}, nil
}

func (v qAttributedVector) Kind() data.Kind {
	return v.vector.Kind()
}

func (v qAttributedVector) Len() int {
	return v.vector.Len()
}

func (v qAttributedVector) At(row int) (any, bool) {
	return v.vector.At(row)
}

func (v qAttributedVector) Values() []any {
	return v.vector.Values()
}

func (v qAttributedVector) Gather(indexes []int) data.Array {
	return qAttributedVector{attribute: v.attribute, vector: v.vector.Gather(indexes)}
}

func (v qAttributedVector) ArrayMetadata() data.ArrayMetadata {
	metadata := data.ArrayMetadataOf(v.vector)
	if !metadata.HasAttribute(v.attribute) {
		return data.ArrayMetadataOf(data.WithArrayAttribute(v.vector, v.attribute))
	}
	return metadata
}

func (v qEnumVector) Kind() data.Kind {
	return v.encoded.Kind()
}

func (v qEnumVector) Len() int {
	return v.encoded.Len()
}

func (v qEnumVector) At(row int) (any, bool) {
	return v.encoded.At(row)
}

func (v qEnumVector) Values() []any {
	return v.encoded.Values()
}

func (v qEnumVector) Gather(indexes []int) data.Array {
	return qEnumVector{domain: v.domain, encoded: v.encoded.Gather(indexes)}
}

func (v qEnumVector) decodedArray() data.Array {
	return data.InferArray(v.Values())
}

func (v qEnumVector) EncodedDomain() []any {
	if domain, ok := data.EncodedDomainOf(v.encoded); ok {
		return domain
	}
	return v.Values()
}

func (v qEnumVector) EncodedCodes() []int32 {
	if codes, ok := data.EncodedCodesOf(v.encoded); ok {
		return codes
	}
	codes := make([]int32, v.Len())
	for i := range codes {
		codes[i] = int32(i)
	}
	return codes
}

func parseDyadicAdverbFunction(src string) (qAdverbFunction, bool) {
	src = strings.TrimSpace(src)
	body := src
	if enclosed(src, '(', ')') {
		body = strings.TrimSpace(src[1 : len(src)-1])
	}
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if strings.HasSuffix(body, adverb) {
			verb := strings.TrimSpace(body[:len(body)-len(adverb)])
			if verb == "" {
				return qAdverbFunction{}, false
			}
			if _, ok := lookupDyadicVerbFunc(verb); ok {
				return qAdverbFunction{verb: verb, adverb: adverb}, true
			}
		}
	}
	return qAdverbFunction{}, false
}

func isQIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isQIdentRest(ch byte) bool {
	return isQIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func isQSymbolDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '(', ')', '[', ']', ';', ',', '!', '+', '-', '/', '=', '<', '>', '#', '$':
		return true
	default:
		return false
	}
}

func isQPathSymbolDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '(', ')', '[', ']', ';', ',':
		return true
	default:
		return false
	}
}

func qExprLiteral(v any) string {
	if data.IsNull(v) {
		return "0N"
	}
	if array, ok := v.(data.Array); ok {
		items := make([]string, 0, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, _ := array.At(i)
			items = append(items, qExprLiteral(item))
		}
		switch array.Kind() {
		case data.KindSymbol:
			return strings.Join(items, "")
		case data.KindAny:
			return "(" + strings.Join(items, ";") + ")"
		}
		return strings.Join(items, " ")
	}
	if dict, ok := v.(EvalDict); ok {
		keys := make([]string, len(dict.Keys))
		for i, key := range dict.Keys {
			keys[i] = qExprLiteral(key)
		}
		values := make([]string, len(dict.Values))
		for i, value := range dict.Values {
			values[i] = qExprLiteral(value)
		}
		keySep := " "
		if len(keys) > 0 && strings.HasPrefix(keys[0], "`") {
			keySep = ""
		}
		valueSrc := strings.Join(values, " ")
		if len(values) > 1 {
			valueSrc = "(" + strings.Join(values, ";") + ")"
		}
		return "(" + strings.Join(keys, keySep) + "!" + valueSrc + ")"
	}
	if s, ok := FormatTemporal(v); ok {
		return s
	}
	switch x := v.(type) {
	case data.Symbol:
		return "`" + string(x)
	case string:
		return strconv.Quote(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int8, int16, int32, int64, float32, float64:
		return fmt.Sprint(x)
	default:
		return fmt.Sprint(x)
	}
}

func findPostfixLookup(src string) (string, string, bool) {
	if !strings.HasPrefix(src, "(") {
		return "", "", false
	}
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				key := strings.TrimSpace(src[i+1:])
				if key == "" {
					return "", "", false
				}
				switch {
				case strings.HasPrefix(key, "[") && strings.HasSuffix(key, "]") && enclosed(key, '[', ']'):
					key = strings.TrimSpace(key[1 : len(key)-1])
				case strings.HasPrefix(key, "`"):
				default:
					return "", "", false
				}
				return strings.TrimSpace(src[:i+1]), key, true
			}
		}
	}
	return "", "", false
}

func isSign(src string, i int) bool {
	if i+1 >= len(src) || !isQNumericSignStart(src[i+1]) {
		return false
	}
	if i == 0 {
		return true
	}
	switch src[i-1] {
	case ' ', '\t', '\n', '(', ';':
		return true
	default:
		return false
	}
}

func isQNumericSignStart(ch byte) bool {
	return (ch >= '0' && ch <= '9') || ch == '.'
}

func findDyadic(src string) (int, byte, bool) {
	for _, ops := range []string{"=<>", "+-", "*%", "&|", "^"} {
		if idx := findTopLevel(src, ops); idx >= 0 {
			return idx, src[idx], true
		}
	}
	return 0, 0, false
}

type adverbExpr struct {
	left   string
	verb   string
	adverb string
	right  string
}

func findAdverb(src string) (adverbExpr, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		if inString {
			if src[i] == '\\' {
				i++
				continue
			}
			if src[i] == '"' {
				inString = false
			}
			continue
		}
		switch src[i] {
		case '"':
			inString = true
			continue
		case '(':
			parenDepth++
			continue
		case ')':
			parenDepth--
			continue
		case '[':
			bracketDepth++
			continue
		case ']':
			bracketDepth--
			continue
		case '{':
			braceDepth++
			continue
		case '}':
			braceDepth--
			continue
		}
		if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
			continue
		}
		adverb := ""
		switch {
		case strings.HasPrefix(src[i:], "\\:"):
			adverb = "\\:"
		case strings.HasPrefix(src[i:], "/:"):
			adverb = "/:"
		case strings.HasPrefix(src[i:], "':"):
			adverb = "':"
		case src[i] == '/':
			adverb = "/"
		case src[i] == '\\':
			adverb = "\\"
		case src[i] == '\'':
			adverb = "'"
		default:
			continue
		}
		verbStart, verbEnd, ok := adverbVerbBounds(src, i)
		if !ok {
			continue
		}
		left := strings.TrimSpace(src[:verbStart])
		right := strings.TrimSpace(src[i+len(adverb):])
		if right == "" {
			continue
		}
		verb := strings.TrimSpace(src[verbStart:verbEnd])
		if _, ok := lookupDyadicVerbFunc(verb); !ok {
			if _, ok := lookupUnaryVerb(verb); !ok {
				continue
			}
		}
		return adverbExpr{left: left, verb: verb, adverb: adverb, right: right}, true
	}
	return adverbExpr{}, false
}

func adverbVerbBounds(src string, adverbStart int) (int, int, bool) {
	end := adverbStart
	for end > 0 && isSpace(src[end-1]) {
		end--
	}
	if end == 0 {
		return 0, 0, false
	}
	if isDyadicOp(src[end-1]) {
		return end - 1, end, true
	}
	start := end
	for start > 0 && isIdentByte(src[start-1]) {
		start--
	}
	return start, end, start < end
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDyadicOp(b byte) bool {
	return b == '+' || b == '-' || b == '*' || b == '%' || b == '=' || b == '<' || b == '>' || b == '&' || b == '|' || b == '^' || b == '~'
}

func (s *EvalState) evalAdverb(expr adverbExpr) (any, error) {
	if expr.left == "" && expr.adverb == "/" && expr.verb == "+" {
		if out, handled, err := s.tryEvalSumFby(expr.right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalSequenceTransformSum(expr.right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalSumDeltas(expr.right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalTypedDyadicFloatSum(expr.right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalTypedUnarySum(expr.right); err != nil || handled {
			return out, err
		}
	}
	right, err := s.eval(expr.right)
	if err != nil {
		return nil, err
	}
	if expr.left == "" {
		switch expr.adverb {
		case "'":
			return applyEachUnary(expr.verb, right)
		case "':":
			fn, ok := lookupDyadicVerbFunc(expr.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with each-prior", expr.verb)
			}
			return applyEachPriorFunc(fn, nil, right)
		case "/":
			op, _, ok := lookupDyadicVerb(expr.verb)
			if ok {
				return applyOver(op, nil, right)
			}
			if _, ok := lookupDyadicVerbFunc(expr.verb); ok {
				return nil, fmt.Errorf("%s cannot be used with over", expr.verb)
			}
			if fn, ok := lookupUnaryVerb(expr.verb); ok {
				return fn(right)
			}
			return nil, fmt.Errorf("%s cannot be used with over", expr.verb)
		case "\\":
			op, _, ok := lookupDyadicVerb(expr.verb)
			if !ok {
				if _, ok := lookupDyadicVerbFunc(expr.verb); ok {
					return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
				}
				return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
			}
			return applyScan(op, nil, right)
		default:
			return nil, fmt.Errorf("%s requires a left operand", expr.adverb)
		}
	}
	left, err := s.eval(expr.left)
	if err != nil {
		return nil, err
	}
	op, _, hasDyadicOp := lookupDyadicVerb(expr.verb)
	fn, hasDyadicFunc := lookupDyadicVerbFunc(expr.verb)
	if !hasDyadicFunc {
		return nil, fmt.Errorf("%s cannot be used as a dyadic verb", expr.verb)
	}
	switch expr.adverb {
	case "'":
		if hasDyadicOp {
			return applyEachDyadic(op, left, right)
		}
		return applyEachDyadicFunc(fn, left, right)
	case "':":
		return applyEachPriorFunc(fn, left, right)
	case "\\:":
		if hasDyadicOp {
			return applyEachLeft(op, left, right)
		}
		return applyEachLeftFunc(fn, left, right)
	case "/:":
		if hasDyadicOp {
			return applyEachRight(op, left, right)
		}
		return applyEachRightFunc(fn, left, right)
	case "/":
		if !hasDyadicOp {
			return nil, fmt.Errorf("%s cannot be used with over", expr.verb)
		}
		return applyOver(op, left, right)
	case "\\":
		if !hasDyadicOp {
			return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
		}
		return applyScan(op, left, right)
	default:
		return nil, fmt.Errorf("adverb %q is not supported", expr.adverb)
	}
}

func (s *EvalState) tryEvalTypedDyadicFloatSum(src string) (any, bool, error) {
	for _, op := range []string{data.NumericDyadicXExp, data.NumericDyadicXLog} {
		leftExpr, rightExpr, ok := splitTopLevelWord(src, op)
		if !ok {
			continue
		}
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, true, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, true, err
		}
		out, handled, err := data.TryTypedQNumericDyadicFloatSum(op, left, right)
		shape := qRuntimeKernelDyadicFloatSumShape(op, qRuntimeKernelOperandKind(left, nil), qRuntimeKernelOperandKind(right, nil))
		out, handled, err = qTypedRuntimeResult("ArrayNumericDyadicFloatSum", shape, out, handled, err)
		if err != nil {
			return nil, true, fmt.Errorf("sum %s: %w", op, err)
		}
		return out, handled, nil
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalSequenceTransformSum(src string) (any, bool, error) {
	transform, args, valueExpr, ok, err := s.sequenceTransformExpr(src)
	if err != nil || !ok {
		return nil, ok, err
	}
	value, err := s.eval(valueExpr)
	if err != nil {
		return nil, true, err
	}
	out, handled, err := data.TryTypedSequenceTransformNumericSum(transform, args, value)
	shape := qRuntimeKernelSequenceTransformSumShape(transform, qRuntimeKernelOperandKind(value, nil), len(args))
	out, handled, err = qTypedRuntimeResultReason("SequenceTransformSum", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	if err != nil {
		return nil, true, err
	}
	return out, handled, nil
}

func (s *EvalState) sequenceTransformExpr(src string) (string, []int, string, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, spec := range []struct {
		prefix    string
		transform string
	}{
		{"reverse ", data.SequenceTransformReverse},
		{"raze ", data.SequenceTransformRaze},
		{"deltas ", data.SequenceTransformDeltas},
		{"ratios ", data.SequenceTransformRatios},
	} {
		if strings.HasPrefix(src, spec.prefix) && wordBoundary(src, 0, len(strings.TrimSpace(spec.prefix))) {
			arg := strings.TrimSpace(src[len(spec.prefix):])
			if arg == "" {
				return "", nil, "", false, nil
			}
			return spec.transform, nil, arg, true, nil
		}
	}
	if leftExpr, rightExpr, ok := splitTopLevelWord(src, "rotate"); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return "", nil, "", true, err
		}
		n, ok := integerValue(left)
		if !ok || int64(int(n)) != n {
			return "", nil, "", true, fmt.Errorf("rotate expects an integer count")
		}
		return data.SequenceTransformRotate, []int{int(n)}, rightExpr, true, nil
	}
	var leftExpr, rightExpr string
	if args, ok := qFunctionCallArgs(src); ok && strings.TrimSpace(src[:strings.Index(src, "[")]) == "sublist" {
		if len(args) != 2 {
			return "", nil, "", true, fmt.Errorf("sublist expects 2 arguments")
		}
		leftExpr, rightExpr = args[0], args[1]
	} else {
		var ok bool
		leftExpr, rightExpr, ok = splitTopLevelWord(src, "sublist")
		if !ok {
			return "", nil, "", false, nil
		}
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return "", nil, "", true, err
	}
	indexes, err := qIntegerIndexes("sublist", left)
	if err != nil {
		return "", nil, "", true, err
	}
	return data.SequenceTransformSublist, indexes, rightExpr, true, nil
}

func (s *EvalState) tryEvalSortIndexSum(src string) (any, bool, error) {
	descending := false
	arg := ""
	switch {
	case strings.HasPrefix(src, "iasc "):
		arg = strings.TrimSpace(src[len("iasc "):])
	case strings.HasPrefix(src, "idesc "):
		descending = true
		arg = strings.TrimSpace(src[len("idesc "):])
	default:
		return nil, false, nil
	}
	if arg == "" {
		return nil, false, nil
	}
	value, err := s.eval(arg)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return int64(0), true, nil
	}
	shape := "sort-index-sum/" + string(array.Kind())
	if descending {
		shape += "/desc"
	} else {
		shape += "/asc"
	}
	out, handled, err := data.TryTypedSortIndexSumI64(array, descending)
	out, handled, err = qTypedRuntimeResult("ArraySortIndexSum", shape, out, handled, err)
	if err != nil || handled {
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalTypedUnarySum(src string) (any, bool, error) {
	op, arg, ok := splitLeadingNumericUnary(src)
	if !ok {
		return nil, false, nil
	}
	if out, handled, err := s.tryEvalTypedUnaryDyadicSum(op, arg); err != nil || handled {
		return out, handled, err
	}
	value, err := s.eval(arg)
	if err != nil {
		return nil, true, err
	}
	if array, ok := value.(data.Array); ok {
		shape := "vector-reduce/sum-" + op + "/" + string(array.Kind())
		out, handled, err := data.TryTypedQNumericUnarySum(op, array)
		out, handled, err = qTypedRuntimeResult("ArrayNumericUnarySum", shape, out, handled, err)
		if err != nil || handled {
			if err != nil {
				return nil, true, fmt.Errorf("sum %s: %w", op, err)
			}
			return out, true, nil
		}
	}
	fn, ok := lookupUnaryVerb(op)
	if !ok {
		return nil, false, nil
	}
	unary, err := fn(value)
	if err != nil {
		return nil, true, err
	}
	out, err := sum(unary)
	return out, true, err
}

func (s *EvalState) tryEvalTypedUnaryDyadicSum(unaryOp, src string) (any, bool, error) {
	leftExpr, dyadicOp, rightExpr, ok := splitTopLevelArithmeticOperator(src)
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	out, handled, err := data.TryTypedQNumericUnaryDyadicSum(unaryOp, data.Op(string(dyadicOp)), left, right)
	if err != nil || handled {
		shape := "vector-reduce/sum-" + unaryOp + "-dyadic-" + string(dyadicOp)
		if array, ok := left.(data.Array); ok {
			shape += "/left-" + string(array.Kind())
		}
		if array, ok := right.(data.Array); ok {
			shape += "/right-" + string(array.Kind())
		}
		out, handled, err = qTypedRuntimeResult("ArrayNumericUnaryDyadicSum", shape, out, handled, err)
		if err != nil {
			return nil, true, fmt.Errorf("sum %s %s: %w", unaryOp, string(dyadicOp), err)
		}
		return out, true, nil
	} else {
		_, _, _ = qTypedRuntimeResult("ArrayNumericUnaryDyadicSum", "vector-reduce/sum-"+unaryOp+"-dyadic-"+string(dyadicOp), out, handled, err)
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalTypedMovingWindowSum(src string) (any, bool, error) {
	for _, spec := range []struct {
		word    string
		wantMax bool
		average bool
	}{
		{word: "msum"},
		{word: "mavg", average: true},
		{word: "mcount"},
		{word: "mmin"},
		{word: "mmax", wantMax: true},
	} {
		leftExpr, rightExpr, ok := splitTopLevelWord(src, spec.word)
		if !ok {
			continue
		}
		widthValue, err := s.eval(leftExpr)
		if err != nil {
			return nil, true, err
		}
		width, ok := integerValue(widthValue)
		if !ok || width <= 0 || int64(int(width)) != width {
			return nil, true, fmt.Errorf("%s width must be a positive integer", spec.word)
		}
		value, err := s.eval(rightExpr)
		if err != nil {
			return nil, true, err
		}
		array, ok := value.(data.Array)
		if !ok {
			var moving any
			switch spec.word {
			case "msum":
				moving, err = msum(widthValue, value)
			case "mavg":
				moving, err = mavg(widthValue, value)
			case "mcount":
				moving, err = mcount(widthValue, value)
			case "mmin":
				moving, err = mmin(widthValue, value)
			case "mmax":
				moving, err = mmax(widthValue, value)
			}
			if err != nil {
				return nil, true, err
			}
			out, err := sum(moving)
			return out, true, err
		}
		if spec.word == "mcount" {
			if out, handled, err := data.TryTypedMCountSum(array, int(width)); err != nil || handled {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-mcount/"+string(array.Kind()), handled, err)
				if err != nil {
					return nil, true, fmt.Errorf("sum mcount: %w", err)
				}
				return out, true, nil
			} else {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-mcount/"+string(array.Kind()), handled, err)
			}
		} else if spec.word == "msum" || spec.word == "mavg" {
			if out, handled, err := data.TryTypedMovingNumericSumSum(array, int(width), spec.average); err != nil || handled {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-"+spec.word+"/"+string(array.Kind()), handled, err)
				if err != nil {
					return nil, true, fmt.Errorf("sum %s: %w", spec.word, err)
				}
				return out, true, nil
			} else {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-"+spec.word+"/"+string(array.Kind()), handled, err)
			}
		} else {
			if out, handled, err := data.TryTypedMovingMinMaxSum(array, int(width), spec.wantMax); err != nil || handled {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-"+spec.word+"/"+string(array.Kind()), handled, err)
				if err != nil {
					return nil, true, fmt.Errorf("sum %s: %w", spec.word, err)
				}
				return out, true, nil
			} else {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-"+spec.word+"/"+string(array.Kind()), handled, err)
			}
		}
		var moving any
		switch spec.word {
		case "msum":
			moving, err = msum(widthValue, value)
		case "mavg":
			moving, err = mavg(widthValue, value)
		case "mcount":
			moving, err = mcount(widthValue, value)
		case "mmin":
			moving, err = mmin(widthValue, value)
		case "mmax":
			moving, err = mmax(widthValue, value)
		}
		if err != nil {
			return nil, true, err
		}
		out, err := sum(moving)
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalSumFby(src string) (any, bool, error) {
	leftExpr, groupExpr, ok := splitTopLevelWord(stripEnclosingParens(strings.TrimSpace(src)), "fby")
	if !ok {
		return nil, false, nil
	}
	agg, valueExpr, err := parseFbyAggregate(leftExpr)
	if err != nil {
		return nil, true, err
	}
	if agg != "sum" {
		return nil, false, nil
	}
	values, err := s.eval(valueExpr)
	if err != nil {
		return nil, true, err
	}
	valueArray, ok := values.(data.Array)
	if !ok {
		return nil, false, nil
	}
	groups, err := s.eval(groupExpr)
	if err != nil {
		return nil, true, err
	}
	groupArray, ok := groups.(data.Array)
	if !ok {
		return nil, false, nil
	}
	if valueArray.Len() != groupArray.Len() {
		return nil, true, fmt.Errorf("fby value length %d does not match group length %d", valueArray.Len(), groupArray.Len())
	}
	out, handled, err := data.TryTypedFbySumTotal(valueArray, groupArray)
	shape := "fby-sum-total/" + string(valueArray.Kind()) + "/" + string(groupArray.Kind())
	recordRuntimeKernelProbe("ArrayFbySumTotal", shape, handled, err)
	if err != nil || handled {
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalScalarAddChain(src string) (any, bool, error) {
	terms := splitTopLevelPlusChain(src)
	if len(terms) < 2 {
		return nil, false, nil
	}
	for _, term := range terms {
		if !isScalarAddChainTerm(term) {
			return nil, false, nil
		}
	}
	if out, handled, err := s.tryEvalNamedScalarReducerAddChain(terms); err != nil || handled {
		return out, handled, err
	}
	var acc any
	for i, term := range terms {
		value, err := s.evalScalarAddChainTerm(term)
		if err != nil {
			return nil, true, err
		}
		if i == 0 {
			acc = value
			continue
		}
		if out, ok := addScalarNumericFast(acc, value); ok {
			acc = out
			continue
		}
		acc, err = applyDyadic('+', acc, value)
		if err != nil {
			return nil, true, err
		}
	}
	return acc, true, nil
}

func (s *EvalState) tryEvalNamedScalarReducerAddChain(terms []string) (any, bool, error) {
	if len(terms) < 2 {
		return nil, false, nil
	}
	name := ""
	reducers := make([]string, 0, len(terms))
	for _, term := range terms {
		reducer, arg, ok := namedScalarReducerTerm(term)
		if !ok {
			return nil, false, nil
		}
		if name == "" {
			name = arg
		} else if name != arg {
			return nil, false, nil
		}
		reducers = append(reducers, reducer)
	}
	value, ok := s.lookupName(name)
	if !ok || isCallable(value) {
		return nil, false, nil
	}
	var acc any
	for i, reducer := range reducers {
		part, err := namedScalarReducerValue(reducer, value)
		if err != nil {
			return nil, true, err
		}
		if i == 0 {
			acc = part
			continue
		}
		if out, ok := addScalarNumericFast(acc, part); ok {
			acc = out
			continue
		}
		out, err := applyDyadic('+', acc, part)
		if err != nil {
			return nil, true, err
		}
		acc = out
	}
	return acc, true, nil
}

func namedScalarReducerTerm(src string) (reducer, name string, ok bool) {
	src = strings.TrimSpace(stripEnclosingParens(src))
	if strings.HasPrefix(src, "+/") {
		name = strings.TrimSpace(src[len("+/"):])
		if isQAssignmentName(name) {
			return "sum", name, true
		}
		return "", "", false
	}
	for _, word := range []string{"sum", "first", "last", "count"} {
		if strings.HasPrefix(src, word) && wordBoundary(src, 0, len(word)) {
			name = strings.TrimSpace(src[len(word):])
			if isQAssignmentName(name) {
				return word, name, true
			}
		}
	}
	return "", "", false
}

func namedScalarReducerValue(reducer string, value any) (any, error) {
	switch reducer {
	case "sum":
		return sum(value)
	case "first":
		return first(value)
	case "last":
		return last(value)
	case "count":
		return count(value)
	default:
		return nil, fmt.Errorf("unsupported scalar reducer %q", reducer)
	}
}

func (s *EvalState) evalScalarAddChainTerm(src string) (any, error) {
	src = strings.TrimSpace(stripEnclosingParens(src))
	if src == "" {
		return nil, fmt.Errorf("empty q expression")
	}
	if qScalarAddChainTermMayBeNumber(src) {
		if value, _, err := parseNumberOrBool(src); err == nil {
			return value, nil
		}
	}
	if qScalarAddChainTermMayBeScalarIndex(src) {
		if scalar, ok := buildScalarApplyIndexPlan(src); ok {
			target, found := s.lookupName(scalar.target)
			if found && !isCallable(target) {
				if value, handled, err := scalarApplyIndexPlanValue(scalar, target); err != nil || handled {
					return value, err
				}
			}
		}
	}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		arg := strings.TrimSpace(src[len("count "):])
		if value, ok := s.lookupName(arg); ok {
			return count(value)
		}
	}
	return s.eval(src)
}

func qScalarAddChainTermMayBeNumber(src string) bool {
	if src == "" {
		return false
	}
	ch := src[0]
	if ch >= '0' && ch <= '9' {
		return true
	}
	if ch == '.' {
		return len(src) > 1 && src[1] >= '0' && src[1] <= '9'
	}
	if ch != '-' && ch != '+' {
		return false
	}
	if len(src) < 2 {
		return false
	}
	next := src[1]
	return next == '.' || (next >= '0' && next <= '9')
}

func qScalarAddChainTermMayBeScalarIndex(src string) bool {
	return strings.Contains(src, "@") || strings.Contains(src, " . ")
}

func addScalarNumericFast(left, right any) (any, bool) {
	if li, ok := integerValue(left); ok {
		if ri, ok := integerValue(right); ok {
			return li + ri, true
		}
	}
	lf, lok := numeric(left)
	rf, rok := numeric(right)
	if lok && rok {
		return lf + rf, true
	}
	return nil, false
}

func splitTopLevelPlusChain(src string) []string {
	var terms []string
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	start := 0
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '+':
			if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 || isSign(src, i) {
				continue
			}
			if i+1 < len(src) && (src[i+1] == '/' || src[i+1] == '\\') {
				continue
			}
			term := strings.TrimSpace(src[start:i])
			if term == "" {
				return nil
			}
			terms = append(terms, term)
			start = i + 1
		}
	}
	if len(terms) == 0 {
		return nil
	}
	last := strings.TrimSpace(src[start:])
	if last == "" {
		return nil
	}
	terms = append(terms, last)
	return terms
}

func isScalarAddChainTerm(src string) bool {
	src = strings.TrimSpace(stripEnclosingParens(src))
	if src == "" {
		return false
	}
	for _, word := range []string{"plus", "minus", "times", "divide"} {
		left, right, ok := splitTopLevelWord(src, word)
		if ok {
			return isScalarAddChainTerm(left) && isScalarAddChainTerm(right)
		}
	}
	if strings.HasPrefix(src, "+/") {
		return true
	}
	if _, ok := buildScalarApplyIndexPlan(src); ok {
		return true
	}
	for _, word := range []string{"sum", "prd", "avg", "min", "max", "first", "last", "count"} {
		if strings.HasPrefix(src, word) && wordBoundary(src, 0, len(word)) && strings.TrimSpace(src[len(word):]) != "" {
			return true
		}
	}
	if _, _, err := parseNumberOrBool(src); err == nil {
		return true
	}
	return false
}

func stripEnclosingParens(src string) string {
	for {
		src = strings.TrimSpace(src)
		if !enclosed(src, '(', ')') {
			return src
		}
		src = strings.TrimSpace(src[1 : len(src)-1])
	}
}

func (s *EvalState) tryEvalFirstLastDyadic(src string) (any, bool, error) {
	for _, spec := range []struct {
		word string
		last bool
	}{
		{word: "first"},
		{word: "last", last: true},
	} {
		if !strings.HasPrefix(src, spec.word) || !wordBoundary(src, 0, len(spec.word)) {
			continue
		}
		arg := strings.TrimSpace(src[len(spec.word):])
		if arg == "" {
			continue
		}
		idx, op, ok := findDyadic(arg)
		if !ok {
			continue
		}
		left, err := s.eval(strings.TrimSpace(arg[:idx]))
		if err != nil {
			return nil, true, err
		}
		right, err := s.eval(strings.TrimSpace(arg[idx+1:]))
		if err != nil {
			return nil, true, err
		}
		leftArray, leftIsArray := left.(data.Array)
		rightArray, rightIsArray := right.(data.Array)
		if leftIsArray && rightIsArray {
			switch {
			case leftArray.Len() == rightArray.Len():
			case leftArray.Len() == 1:
			case rightArray.Len() == 1:
			default:
				return nil, true, fmt.Errorf("vector length mismatch")
			}
		}
		resultLen := -1
		switch {
		case leftIsArray && rightIsArray:
			resultLen = leftArray.Len()
			if rightArray.Len() > resultLen {
				resultLen = rightArray.Len()
			}
		case leftIsArray:
			resultLen = leftArray.Len()
		case rightIsArray:
			resultLen = rightArray.Len()
		default:
			out, err := applyDyadic(op, left, right)
			return out, true, err
		}
		if resultLen == 0 {
			return data.NullValue, true, nil
		}
		row := 0
		if spec.last {
			row = resultLen - 1
		}
		leftValue, err := firstLastDyadicOperandValue(left, leftArray, leftIsArray, row)
		if err != nil {
			return nil, true, err
		}
		rightValue, err := firstLastDyadicOperandValue(right, rightArray, rightIsArray, row)
		if err != nil {
			return nil, true, err
		}
		out, err := applyDyadic(op, leftValue, rightValue)
		return out, true, err
	}
	return nil, false, nil
}

func firstLastDyadicOperandValue(value any, array data.Array, isArray bool, row int) (any, error) {
	if !isArray {
		return value, nil
	}
	if array.Len() == 1 {
		row = 0
	}
	item, ok := array.At(row)
	if !ok {
		return nil, fmt.Errorf("array row %d out of range", row)
	}
	return item, nil
}

func (s *EvalState) tryEvalCountDistinct(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "distinct ") {
		return nil, false, nil
	}
	out, err := s.eval(strings.TrimSpace(src[len("distinct "):]))
	if err != nil {
		return nil, true, err
	}
	value, err := countDistinct(out)
	return value, true, err
}

func (s *EvalState) tryEvalCountGroup(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "group ") {
		return nil, false, nil
	}
	value, err := s.eval(strings.TrimSpace(src[len("group "):]))
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "group-count/" + string(array.Kind())
	out, handled, err := data.TryTypedGroupCount(array)
	out, handled, err = qTypedRuntimeResult("ArrayGroupCount", shape, out, handled, err)
	if err != nil || handled {
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalCountWhereCompare(src string) (any, bool, error) {
	count, handled, err := s.tryEvalWhereCompareCount(src, "compare-count")
	if err != nil || handled {
		return count, handled, err
	}
	count, _, handled, err = s.tryEvalWhereCompareIndexStats(src, "compare-to-index-count")
	if err != nil || handled {
		return count, handled, err
	}
	indexes, handled, err := s.tryEvalWhereCompareIndexes(src, "compare-to-index-count")
	if err != nil || !handled {
		return nil, handled, err
	}
	return int64(indexes.Len()), true, nil
}

func (s *EvalState) tryEvalCountWhereLike(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "where ") {
		return nil, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, ok := splitTopLevelWord(arg, "like")
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return nil, false, nil
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	pattern, ok := qLikeText(right)
	if !ok {
		return nil, false, nil
	}
	shape := "like-count/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayStringLikeCount",
		shape:  shape,
		call: func() (int64, bool, error) {
			return data.TryTypedStringLikeCount(array, pattern)
		},
	})
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

func (s *EvalState) tryEvalCountWhereIn(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "where ") {
		return nil, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, ok := splitTopLevelWord(arg, "in")
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return nil, false, nil
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	values, err := setItems(right)
	if err != nil {
		return nil, true, err
	}
	shape := "in-count/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayInCount",
		shape:  shape,
		call: func() (int64, bool, error) {
			return data.TryTypedInCount(array, values)
		},
	})
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

func (s *EvalState) tryEvalWhereInIndexes(src, shapePrefix string) (data.Array, bool, error) {
	if !strings.HasPrefix(src, "where ") {
		return nil, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, ok := splitTopLevelWord(arg, "in")
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return nil, false, nil
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	values, err := setItems(right)
	if err != nil {
		return nil, true, err
	}
	shape := shapePrefix + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel: "ArrayWhereIn",
		shape:  shape,
		call: func() (data.Array, bool, error) {
			return data.TryTypedInIndexesI64(array, values)
		},
	})
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func (s *EvalState) tryEvalWhereInIndexStats(src, shapePrefix string) (count, sum int64, handled bool, err error) {
	if !strings.HasPrefix(src, "where ") {
		return 0, 0, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, ok := splitTopLevelWord(arg, "in")
	if !ok {
		return 0, 0, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return 0, 0, true, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return 0, 0, false, nil
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return 0, 0, true, err
	}
	values, err := setItems(right)
	if err != nil {
		return 0, 0, true, err
	}
	shape := shapePrefix + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	count, sum, handled, err = evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
		kernel: "ArrayWhereInStats",
		shape:  shape,
		call: func() (int64, int64, bool, error) {
			return data.TryTypedInIndexStatsI64(array, values)
		},
	})
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	return count, sum, true, nil
}

func (s *EvalState) tryEvalCountReverse(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "reverse ") {
		return nil, false, nil
	}
	value, err := s.eval(strings.TrimSpace(src[len("reverse "):]))
	if err != nil {
		return nil, true, err
	}
	out, err := count(value)
	if err != nil {
		return nil, true, err
	}
	recordRuntimeKernelProbe("ArrayCountReverse", "count-reverse/"+string(qRuntimeKernelOperandKind(value, nil)), true, nil)
	return out, true, nil
}

func (s *EvalState) tryEvalCountSequencePrimitive(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if out, handled, err := s.tryEvalCountSequenceTransform(src); err != nil || handled {
		return out, handled, err
	}
	if out, handled, err := s.tryEvalCountStringTransform(src); err != nil || handled {
		return out, handled, err
	}
	if out, handled, err := s.tryEvalCountCross(src); err != nil || handled {
		return out, handled, err
	}
	if out, handled, err := s.tryEvalCountCut(src); err != nil || handled {
		return out, handled, err
	}
	if out, handled, err := s.tryEvalCountSublist(src); err != nil || handled {
		return out, handled, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalCountSequenceTransform(src string) (any, bool, error) {
	transform, args, valueExpr, ok, err := s.sequenceTransformExpr(src)
	if err != nil || !ok {
		return nil, ok, err
	}
	value, err := s.eval(valueExpr)
	if err != nil {
		return nil, true, err
	}
	out, handled, err := data.SequenceTransformCount(transform, args, value)
	shape := transform + "-count/" + string(qRuntimeKernelOperandKind(value, nil))
	if len(args) > 0 {
		shape += "/args-" + strconv.Itoa(len(args))
	}
	recordRuntimeKernelProbe("SequenceTransformCount", shape, handled && err == nil, err)
	if err != nil || handled {
		return out, handled, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalCountStringTransform(src string) (any, bool, error) {
	for _, transform := range []struct {
		word     string
		kernel   string
		fn       func(any) (int64, error)
		takeFn   func(int, any) (int64, error)
		takeName string
	}{
		{"trim ", "StringTrimCount", data.TrimmedStringCount, data.RepeatedTrimmedStringCount, "StringRepeatedTrimCount"},
		{"ltrim ", "StringLeftTrimCount", data.LTrimmedStringCount, data.RepeatedLTrimmedStringCount, "StringRepeatedLeftTrimCount"},
		{"rtrim ", "StringRightTrimCount", data.RTrimmedStringCount, data.RepeatedRTrimmedStringCount, "StringRepeatedRightTrimCount"},
	} {
		if !strings.HasPrefix(src, transform.word) || !wordBoundary(src, 0, len(strings.TrimSpace(transform.word))) {
			continue
		}
		arg := strings.TrimSpace(src[len(transform.word):])
		if out, handled, err := s.tryEvalCountRepeatedStringTransform(arg, transform.takeName, transform.takeFn); err != nil || handled {
			return out, handled, err
		}
		value, err := s.eval(arg)
		if err != nil {
			return nil, true, err
		}
		shape := "string-count/" + strings.TrimSpace(transform.word) + "/" + string(qRuntimeKernelOperandKind(value, nil))
		out, err := transform.fn(value)
		recordRuntimeKernelProbe(transform.kernel, shape, err == nil, err)
		if err != nil {
			return nil, true, err
		}
		return out, true, nil
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalCountRepeatedStringTransform(src, kernel string, fn func(int, any) (int64, error)) (any, bool, error) {
	hash := findTopLevel(src, "#")
	if hash < 0 {
		return nil, false, nil
	}
	left, err := s.eval(strings.TrimSpace(src[:hash]))
	if err != nil {
		return nil, true, err
	}
	n, ok := integerValue(left)
	if !ok || int64(int(n)) != n {
		return nil, false, nil
	}
	right, err := s.eval(strings.TrimSpace(src[hash+1:]))
	if err != nil {
		return nil, true, err
	}
	switch right.(type) {
	case string, data.Symbol:
	default:
		return nil, false, nil
	}
	shape := "string-count/take/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	out, err := fn(int(n), right)
	recordRuntimeKernelProbe(kernel, shape, err == nil, err)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

func (s *EvalState) tryEvalCountCross(src string) (any, bool, error) {
	leftExpr, rightExpr, ok := splitTopLevelWord(src, "cross")
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	out := data.CrossCount(left, right)
	shape := "cross-count/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	recordRuntimeKernelProbe("SequenceCrossCount", shape, true, nil)
	return out, true, nil
}

func (s *EvalState) tryEvalCountCut(src string) (any, bool, error) {
	var leftExpr, rightExpr string
	if args, ok := qFunctionCallArgs(src); ok && strings.TrimSpace(src[:strings.Index(src, "[")]) == "cut" {
		if len(args) != 2 {
			return nil, true, fmt.Errorf("cut expects 2 arguments")
		}
		leftExpr, rightExpr = args[0], args[1]
	} else {
		var ok bool
		leftExpr, rightExpr, ok = splitTopLevelWord(src, "cut")
		if !ok {
			return nil, false, nil
		}
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	indexes, err := qIntegerIndexes("cut", left)
	if err != nil {
		return nil, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	out, err := data.CutCount(indexes, right)
	shape := "cut-count/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	recordRuntimeKernelProbe("SequenceCutCount", shape, err == nil, err)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

func (s *EvalState) tryEvalCountSublist(src string) (any, bool, error) {
	var leftExpr, rightExpr string
	if args, ok := qFunctionCallArgs(src); ok && strings.TrimSpace(src[:strings.Index(src, "[")]) == "sublist" {
		if len(args) != 2 {
			return nil, true, fmt.Errorf("sublist expects 2 arguments")
		}
		leftExpr, rightExpr = args[0], args[1]
	} else {
		var ok bool
		leftExpr, rightExpr, ok = splitTopLevelWord(src, "sublist")
		if !ok {
			return nil, false, nil
		}
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	args, err := qIntegerIndexes("sublist", left)
	if err != nil {
		return nil, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	var out int64
	switch len(args) {
	case 1:
		out = qTakeCount(args[0], right)
	case 2:
		out, err = data.SublistCount(args[0], args[1], right)
	default:
		err = fmt.Errorf("sublist expects count or start count")
	}
	shape := "sublist-count/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	recordRuntimeKernelProbe("SequenceSublistCount", shape, err == nil, err)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

func (s *EvalState) tryEvalCountLengthPreservingTransform(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, transform := range []struct {
		word    string
		kernel  string
		kindOK  func(data.Kind) bool
		verbErr string
	}{
		{"prior ", "ArrayCountPrev", nil, ""},
		{"prev ", "ArrayCountPrev", nil, ""},
		{"next ", "ArrayCountNext", nil, ""},
		{"fills ", "ArrayCountFills", nil, ""},
		{"deltas ", "ArrayCountDeltas", qKindIsNumeric, "deltas expects a numeric vector"},
		{"ratios ", "ArrayCountRatios", qKindIsNumeric, "ratios expects a numeric vector"},
	} {
		if !strings.HasPrefix(src, transform.word) || !wordBoundary(src, 0, len(strings.TrimSpace(transform.word))) {
			continue
		}
		value, err := s.eval(strings.TrimSpace(src[len(transform.word):]))
		if err != nil {
			return nil, true, err
		}
		array, ok := value.(data.Array)
		if !ok {
			return nil, false, nil
		}
		shape := "vector-count/" + strings.TrimSpace(transform.word) + "/" + string(array.Kind())
		if transform.kindOK != nil && !transform.kindOK(array.Kind()) {
			err := fmt.Errorf("%s", transform.verbErr)
			recordRuntimeKernelProbe(transform.kernel, shape, false, err)
			return nil, true, err
		}
		recordRuntimeKernelProbe(transform.kernel, shape, true, nil)
		return int64(array.Len()), true, nil
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalSumWhereCompare(src string) (any, bool, error) {
	_, sum, handled, err := s.tryEvalWhereCompareIndexStats(src, "compare-to-index-sum")
	if err != nil || handled {
		return sum, handled, err
	}
	_, sum, handled, err = s.tryEvalWhereInIndexStats(src, "in-to-index-sum")
	if err != nil || handled {
		return sum, handled, err
	}
	indexes, handled, err := s.tryEvalWhereCompareIndexes(src, "compare-to-index-sum")
	if err != nil || !handled {
		indexes, handled, err = s.tryEvalWhereInIndexes(src, "in-to-index-sum")
		if err != nil || !handled {
			return nil, handled, err
		}
	}
	shape := "index-sum/" + string(indexes.Kind())
	out, handled, err := data.TryTypedNumericSum(indexes)
	out, handled, err = qTypedRuntimeResult("ArrayWhereCompareSum", shape, out, handled, err)
	if err != nil || handled {
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalSumDeltas(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, "deltas ") || !wordBoundary(src, 0, len("deltas")) {
		return nil, false, nil
	}
	value, err := s.eval(strings.TrimSpace(src[len("deltas "):]))
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		delta, err := deltas(value)
		if err != nil {
			return nil, true, err
		}
		out, err := sum(delta)
		return out, true, err
	}
	shape := "vector-reduce/sum-deltas/" + string(array.Kind())
	if out, handled, err := data.TryTypedDeltasSum(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayDeltasSum", shape, handled, err)
		return out, true, err
	} else {
		recordRuntimeKernelProbe("ArrayDeltasSum", shape, handled, err)
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalSumWhereGatherReduce(src string) (any, bool, error) {
	plan, ok := buildQPipelineSumGatherPlan(src)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := s.evalQPipelinePlan(&plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func directWhereMaskExpr(indexExpr string) (string, bool) {
	indexExpr = strings.TrimSpace(indexExpr)
	if strings.HasPrefix(indexExpr, "where ") {
		return strings.TrimSpace(indexExpr[len("where "):]), true
	}
	return "", false
}

func (s *EvalState) tryEvalWhereCompareCountSum(src string) (any, bool, error) {
	idx, op, ok := findDyadic(src)
	if !ok || op != '+' {
		return nil, false, nil
	}
	left := stripEnclosedParens(strings.TrimSpace(src[:idx]))
	right := stripEnclosedParens(strings.TrimSpace(src[idx+1:]))
	whereExpr, ok := matchingCountSumWhereCompare(left, right)
	if !ok {
		return nil, false, nil
	}
	count, sum, handled, err := s.tryEvalWhereCompareIndexStats(whereExpr, "compare-to-index-count-sum")
	recordRuntimeKernelProbe("ArrayWhereCompareCountSum", "count-sum", handled, err)
	if err != nil || !handled {
		return nil, handled, err
	}
	return count + sum, true, nil
}

func matchingCountSumWhereCompare(left, right string) (string, bool) {
	leftCount, leftWhere := parseCountWhereExpr(left)
	rightSum, rightSumWhere := parseSumWhereExpr(right)
	if leftCount && rightSum && leftWhere == rightSumWhere {
		return leftWhere, true
	}
	rightCount, rightWhere := parseCountWhereExpr(right)
	leftSum, leftSumWhere := parseSumWhereExpr(left)
	if rightCount && leftSum && rightWhere == leftSumWhere {
		return rightWhere, true
	}
	return "", false
}

func parseCountWhereExpr(src string) (bool, string) {
	if !strings.HasPrefix(src, "count ") {
		return false, ""
	}
	whereExpr := strings.TrimSpace(src[len("count "):])
	if !strings.HasPrefix(whereExpr, "where ") {
		return false, ""
	}
	if _, _, _, ok := splitWhereCompareExpr(strings.TrimSpace(whereExpr[len("where "):])); !ok {
		return false, ""
	}
	return true, whereExpr
}

func parseSumWhereExpr(src string) (bool, string) {
	if !strings.HasPrefix(src, "+/") {
		return false, ""
	}
	whereExpr := strings.TrimSpace(src[len("+/"):])
	if !strings.HasPrefix(whereExpr, "where ") {
		return false, ""
	}
	if _, _, _, ok := splitWhereCompareExpr(strings.TrimSpace(whereExpr[len("where "):])); !ok {
		return false, ""
	}
	return true, whereExpr
}

func stripEnclosedParens(src string) string {
	for enclosed(src, '(', ')') {
		src = strings.TrimSpace(src[1 : len(src)-1])
	}
	return src
}

func (s *EvalState) tryEvalWhereCompareIndexStats(src, shapePrefix string) (count, sum int64, handled bool, err error) {
	if !strings.HasPrefix(src, "where ") {
		return 0, 0, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(arg)
	if !ok {
		return 0, 0, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return 0, 0, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return 0, 0, true, err
	}
	if op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return 0, 0, ok, err
		}
		shape := shapePrefix + "-stats/within/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		count, sum, handled, err = evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
			kernel: "ArrayWhereWithinStats",
			shape:  shape,
			call: func() (int64, int64, bool, error) {
				return data.TryTypedWithinIndexStatsI64(array, low, high, true)
			},
		})
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		return count, sum, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return 0, 0, false, nil
	}
	shape := shapePrefix + "-stats/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, sum, handled, err = evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
		kernel: "ArrayWhereCompareStats",
		shape:  shape,
		call: func() (int64, int64, bool, error) {
			return data.TryTypedCompareIndexStatsI64(array, dataOp, scalar)
		},
	})
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	return count, sum, true, nil
}

func (s *EvalState) tryEvalWhereCompareCount(src, shapePrefix string) (count int64, handled bool, err error) {
	if !strings.HasPrefix(src, "where ") {
		return 0, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(arg)
	if !ok {
		return 0, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return 0, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return 0, true, err
	}
	if op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return 0, ok, err
		}
		shape := shapePrefix + "/within/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		count, handled, err = evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
			kernel: "ArrayWhereWithinCount",
			shape:  shape,
			call: func() (int64, bool, error) {
				return data.TryTypedWithinCount(array, low, high, true)
			},
		})
		if err != nil || !handled {
			return 0, handled, err
		}
		return count, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return 0, false, nil
	}
	shape := shapePrefix + "/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, handled, err = evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayWhereCompareCount",
		shape:  shape,
		call: func() (int64, bool, error) {
			return data.TryTypedCompareCount(array, dataOp, scalar)
		},
	})
	if err != nil || !handled {
		return 0, handled, err
	}
	return count, true, nil
}

func (s *EvalState) tryEvalWhereCompareIndexes(src, shapePrefix string) (data.Array, bool, error) {
	if !strings.HasPrefix(src, "where ") {
		return nil, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(arg)
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	if op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return nil, ok, err
		}
		shape := "within-to-index/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
			kernel: "ArrayWhereWithin",
			shape:  shape,
			call: func() (data.Array, bool, error) {
				return data.TryTypedWithinIndexesI64(array, low, high, true)
			},
		})
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return nil, false, nil
		}
		return out, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return nil, false, nil
	}
	shape := shapePrefix + "/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel: "ArrayWhereCompare",
		shape:  shape,
		call: func() (data.Array, bool, error) {
			return data.TryTypedCompareIndexesI64(array, dataOp, scalar)
		},
	})
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func (s *EvalState) tryEvalCountWhereNull(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "where ") {
		return nil, false, nil
	}
	arg := strings.TrimSpace(src[len("where "):])
	if !strings.HasPrefix(arg, "null ") {
		return nil, false, nil
	}
	value, err := s.eval(strings.TrimSpace(arg[len("null "):]))
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		if data.IsNull(value) {
			return int64(1), true, nil
		}
		return int64(0), true, nil
	}
	shape := "null-count/" + string(array.Kind())
	out, handled, err := data.TryTypedNullCount(array)
	out, handled, err = qTypedRuntimeResult("ArrayNullCount", shape, out, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, true, err
		}
		return out, true, nil
	}
	nulls, err := nullValue(value)
	if err != nil {
		return nil, true, err
	}
	indexes, err := where(nulls)
	if err != nil {
		return nil, true, err
	}
	fallbackOut, err := count(indexes)
	return fallbackOut, true, err
}

func (s *EvalState) tryEvalCountFrameMetadata(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	op := ""
	arg := ""
	switch {
	case strings.HasPrefix(src, "cols "):
		op = "cols"
		arg = strings.TrimSpace(src[len("cols "):])
	case strings.HasPrefix(src, "meta "):
		op = "meta"
		arg = strings.TrimSpace(src[len("meta "):])
	default:
		return nil, false, nil
	}
	names, rows, handled, err := s.tryEvalFrameColumnNamesExpr(arg)
	if !handled {
		return nil, false, nil
	}
	recordRuntimeFramePrimitive("FrameMetadata", "frame-meta/count-"+op+"/"+qRuntimeCardinalityShape(rows)+"/cols-"+strconv.Itoa(len(names)), err)
	if err != nil {
		return nil, true, err
	}
	return int64(len(names)), true, nil
}

func (s *EvalState) tryEvalFrameColumnNamesExpr(src string) ([]data.Symbol, int, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "flip ") {
		value, err := s.eval(strings.TrimSpace(src[len("flip "):]))
		if err != nil {
			return nil, 0, true, err
		}
		names, rows, err := flipColumnNamesAndRows(value)
		return names, rows, true, err
	}
	for _, word := range []string{"xcols", "xasc", "xdesc", "xkey", "xgroup"} {
		leftExpr, rightExpr, ok := splitTopLevelWord(src, word)
		if !ok {
			continue
		}
		requestedValue, err := s.eval(leftExpr)
		if err != nil {
			return nil, 0, true, err
		}
		requested, err := qColumnNameList(requestedValue)
		if err != nil {
			return nil, 0, true, fmt.Errorf("%s: %w", word, err)
		}
		names, rows, handled, err := s.tryEvalFrameColumnNamesExpr(rightExpr)
		if !handled || err != nil {
			return names, rows, handled, err
		}
		switch word {
		case "xcols":
			order, err := xcolsOrder(names, requested)
			if err != nil {
				return nil, 0, true, err
			}
			return order, rows, true, nil
		default:
			if err := qValidateFrameColumns(names, requested, word); err != nil {
				return nil, 0, true, err
			}
			return names, rows, true, nil
		}
	}
	value, err := s.eval(src)
	if err != nil {
		return nil, 0, true, err
	}
	switch x := value.(type) {
	case data.Frame:
		return data.FrameColumnNames(x), x.Len(), true, nil
	case data.KeyedFrame:
		return data.KeyedFrameColumnNames(x), data.KeyedFrameLen(x), true, nil
	default:
		return nil, 0, false, nil
	}
}

func (s *EvalState) tryEvalCountFlip(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, "flip ") {
		return nil, false, nil
	}
	value, err := s.eval(strings.TrimSpace(src[len("flip "):]))
	if err != nil {
		return nil, true, err
	}
	rows, cols, err := flipRowCount(value)
	recordRuntimeFramePrimitive("FrameMetadata", "frame-meta/count-flip/"+qRuntimeCardinalityShape(rows)+"/cols-"+strconv.Itoa(cols), err)
	if err != nil {
		return nil, true, err
	}
	return int64(rows), true, nil
}

func (s *EvalState) tryEvalCountWhereMask(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "where ") {
		return nil, false, nil
	}
	value, err := s.eval(strings.TrimSpace(src[len("where "):]))
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok || array.Kind() != data.KindBool {
		return nil, false, nil
	}
	out, handled, err := data.TryTypedTrueCount(array)
	out, handled, err = qTypedRuntimeResult("ArrayTrueCount", "true-count/"+string(array.Kind()), out, handled, err)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

func (s *EvalState) tryEvalCountRunningScan(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, scan := range []struct {
		word    string
		kernel  string
		fn      func(any) (any, error)
		kindOK  func(data.Kind) bool
		scanErr string
	}{
		{"sums ", "ArrayCountSums", sums, qKindIsNumeric, "sums expects a numeric vector"},
		{"prds ", "ArrayCountProducts", prds, qKindIsNumeric, "prds expects a numeric vector"},
		{"mins ", "ArrayCountMins", mins, qTypedCompareKindOK, "mins expects an ordered vector"},
		{"maxs ", "ArrayCountMaxs", maxs, qTypedCompareKindOK, "maxs expects an ordered vector"},
		{"avgs ", "ArrayCountAvgs", avgs, qKindIsNumeric, "avgs expects a numeric vector"},
	} {
		if !strings.HasPrefix(src, scan.word) {
			continue
		}
		value, err := s.eval(strings.TrimSpace(src[len(scan.word):]))
		if err != nil {
			return nil, true, err
		}
		array, ok := value.(data.Array)
		if !ok {
			result, err := scan.fn(value)
			if err != nil {
				return nil, true, err
			}
			out, err := count(result)
			return out, true, err
		}
		shape := "vector-count/" + strings.TrimSpace(scan.word) + "/" + string(array.Kind())
		if !scan.kindOK(array.Kind()) {
			err := fmt.Errorf("%s", scan.scanErr)
			recordRuntimeKernelProbe(scan.kernel, shape, false, err)
			return nil, true, err
		}
		recordRuntimeKernelProbe(scan.kernel, shape, true, nil)
		return int64(array.Len()), true, nil
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalLastDyadicTerminal(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, candidate := range []struct {
		token string
		op    byte
		word  bool
	}{
		{token: "+", op: '+'},
		{token: "-", op: '-'},
		{token: "*", op: '*'},
		{token: "%", op: '%'},
		{token: "mod", op: 'r', word: true},
		{token: "div", op: 'd', word: true},
		{token: "<", op: '<'},
		{token: ">", op: '>'},
		{token: "=", op: '='},
	} {
		var leftExpr, rightExpr string
		var ok bool
		if candidate.word {
			leftExpr, rightExpr, ok = splitTopLevelWord(src, candidate.token)
		} else {
			leftExpr, rightExpr, ok = splitTopLevelOperator(src, candidate.token)
		}
		if !ok {
			continue
		}
		left, err := s.evalQLastTerminalOperand(leftExpr)
		if err != nil {
			return nil, true, err
		}
		right, err := s.evalQLastTerminalOperand(rightExpr)
		if err != nil {
			return nil, true, err
		}
		out, err := applyDyadic(candidate.op, left, right)
		shape := "last-dyadic/" + qAdverbOperatorShapeToken(candidate.token, candidate.op) + "/" + string(qKindOfValue(left)) + "/" + string(qKindOfValue(right))
		recordQTypedRuntimeKernelReason("QTerminalLastDyadic", shape, err == nil, err, RuntimeFallbackRuntimeError)
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) evalQLastTerminalOperand(src string) (any, error) {
	value, err := s.eval(src)
	if err != nil {
		return nil, err
	}
	if _, ok := value.(data.Array); ok {
		return last(value)
	}
	return value, nil
}

func qAdverbOperatorShapeToken(token string, op byte) string {
	if token != "" {
		return token
	}
	return string(op)
}

func (s *EvalState) tryEvalCountFby(src string) (any, bool, error) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	leftExpr, groupExpr, ok := splitTopLevelWord(src, "fby")
	if !ok {
		return nil, false, nil
	}
	agg, valueExpr, err := parseFbyAggregate(leftExpr)
	if err != nil {
		return nil, true, err
	}
	values, err := s.eval(valueExpr)
	if err != nil {
		return nil, true, err
	}
	valueArray, ok := values.(data.Array)
	if !ok {
		return nil, true, fmt.Errorf("fby aggregate value must be a vector")
	}
	groups, err := s.eval(groupExpr)
	if err != nil {
		return nil, true, err
	}
	groupArray, ok := groups.(data.Array)
	if !ok {
		return nil, true, fmt.Errorf("fby group must be a vector")
	}
	if valueArray.Len() != groupArray.Len() {
		return nil, true, fmt.Errorf("fby value length %d does not match group length %d", valueArray.Len(), groupArray.Len())
	}
	shape := "vector-count/fby-" + agg + "/" + string(valueArray.Kind()) + "/" + string(groupArray.Kind())
	recordRuntimeKernelProbe("ArrayCountFby", shape, true, nil)
	return int64(groupArray.Len()), true, nil
}

func (s *EvalState) tryEvalCountXbar(src string) (any, bool, error) {
	leftExpr, rightExpr, ok := splitTopLevelWord(src, "xbar")
	if !ok {
		return nil, false, nil
	}
	width, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	value, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	normalizedWidth := normalizeXbarIntervalForKind(width, array.Kind())
	if _, ok := numeric(normalizedWidth); !ok {
		return nil, false, nil
	}
	if out, handled := data.TryTypedNumericArrayLen(array); handled {
		recordRuntimeKernelProbe("ArrayCountXbar", "vector-count/xbar/"+string(array.Kind()), true, nil)
		return out, true, nil
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalLastScan(src string) (any, bool, error) {
	for _, scan := range []struct {
		word string
		fn   func(any) (any, error)
	}{
		{"sums ", sum},
		{"prds ", prd},
		{"mins ", minValue},
		{"maxs ", maxValue},
		{"avgs ", avg},
	} {
		if !strings.HasPrefix(src, scan.word) {
			continue
		}
		value, err := s.eval(strings.TrimSpace(src[len(scan.word):]))
		if err != nil {
			return nil, true, err
		}
		if array, ok := value.(data.Array); ok {
			if array.Len() == 0 {
				return nil, false, nil
			}
			recordRuntimeKernelProbe("ArrayLastScan", "vector-last/"+strings.TrimSpace(scan.word)+"/"+string(array.Kind()), true, nil)
		}
		out, err := scan.fn(value)
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalLastCallableScan(src string) (any, bool, error) {
	fnSrc, initialSrc, valueSrc, ok := parseCallableScanApplication(stripEnclosingParens(src))
	if !ok {
		return nil, false, nil
	}
	fn, err := s.eval(fnSrc)
	if err != nil {
		return nil, true, err
	}
	if !isCallable(fn) {
		return nil, false, nil
	}
	var initial any
	if initialSrc != "" {
		initial, err = s.eval(initialSrc)
		if err != nil {
			return nil, true, err
		}
	}
	value, err := s.eval(valueSrc)
	if err != nil {
		return nil, true, err
	}
	out, err := s.applyOverCallable(fn, initial, value)
	recordRuntimeKernelProbe("CallableLastScan", "last-scan/"+string(qRuntimeKernelOperandKind(value, nil)), err == nil, err)
	return out, true, err
}

func parseCallableScanApplication(src string) (fnSrc, initialSrc, valueSrc string, ok bool) {
	if !strings.HasSuffix(src, "]") {
		return "", "", "", false
	}
	open := findMatchingCallOpen(src)
	if open < 0 {
		return "", "", "", false
	}
	head := strings.TrimSpace(src[:open])
	if !strings.HasSuffix(head, "\\") {
		return "", "", "", false
	}
	fnSrc = strings.TrimSpace(head[:len(head)-1])
	if fnSrc == "" {
		return "", "", "", false
	}
	args := splitTopLevel(src[open+1:len(src)-1], ';')
	switch len(args) {
	case 1:
		valueSrc = strings.TrimSpace(args[0])
	case 2:
		initialSrc = strings.TrimSpace(args[0])
		valueSrc = strings.TrimSpace(args[1])
	default:
		return "", "", "", false
	}
	if valueSrc == "" {
		return "", "", "", false
	}
	return fnSrc, initialSrc, valueSrc, true
}

func findMatchingCallOpen(src string) int {
	depthParen, depthBracket, depthBrace := 0, 0, 0
	inString := false
	for i := len(src) - 1; i >= 0; i-- {
		ch := src[i]
		if inString {
			if ch == '"' && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case ')':
			depthParen++
		case '(':
			if depthParen > 0 {
				depthParen--
			}
		case '}':
			depthBrace++
		case '{':
			if depthBrace > 0 {
				depthBrace--
			}
		case ']':
			depthBracket++
		case '[':
			if depthBracket > 0 {
				depthBracket--
				if depthBracket == 0 && depthParen == 0 && depthBrace == 0 {
					return i
				}
			}
		}
	}
	return -1
}

func splitTopLevelArithmeticOperator(src string) (string, byte, string, bool) {
	for _, op := range []string{"+", "-", "*", "%"} {
		if left, right, ok := splitTopLevelOperator(src, op); ok {
			return left, op[0], right, true
		}
	}
	return "", 0, "", false
}

func splitLeadingNumericUnary(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	for _, op := range []string{
		data.NumericUnaryRecip,
		data.NumericUnaryCeiling,
		data.NumericUnarySignum,
		data.NumericUnaryFloor,
		data.NumericUnaryAbs,
		data.NumericUnaryExp,
		data.NumericUnaryNeg,
	} {
		if src == op {
			return op, "", false
		}
		if strings.HasPrefix(src, op) && len(src) > len(op) && isSpace(src[len(op)]) {
			return op, strings.TrimSpace(src[len(op):]), true
		}
	}
	return "", "", false
}

func (s *EvalState) evalFby(leftExpr, groupExpr string) (any, error) {
	agg, valueExpr, err := parseFbyAggregate(leftExpr)
	if err != nil {
		return nil, err
	}
	values, err := s.eval(valueExpr)
	if err != nil {
		return nil, err
	}
	valueArray, ok := values.(data.Array)
	if !ok {
		return nil, fmt.Errorf("fby aggregate value must be a vector")
	}
	groups, err := s.eval(groupExpr)
	if err != nil {
		return nil, err
	}
	groupArray, ok := groups.(data.Array)
	if !ok {
		return nil, fmt.Errorf("fby group must be a vector")
	}
	if valueArray.Len() != groupArray.Len() {
		return nil, fmt.Errorf("fby value length %d does not match group length %d", valueArray.Len(), groupArray.Len())
	}
	if agg == "sum" {
		out, handled, err := data.TryTypedFbySum(valueArray, groupArray)
		shape := "fby-sum/" + string(valueArray.Kind()) + "/" + string(groupArray.Kind())
		recordRuntimeKernelProbe("ArrayFbySum", shape, handled, err)
		if err != nil {
			return nil, err
		}
		if handled {
			return out, nil
		}
	}
	groupRows := make(map[string][]int, groupArray.Len())
	groupOrder := make([]string, 0, groupArray.Len())
	for row, value := range groupArray.Values() {
		key := evalValueKey(value)
		if _, ok := groupRows[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groupRows[key] = append(groupRows[key], row)
	}
	groupValues := make(map[string]any, len(groupRows))
	for _, key := range groupOrder {
		rows := groupRows[key]
		part := valueArray.Gather(rows)
		v, err := applyFbyAggregate(agg, part)
		if err != nil {
			return nil, err
		}
		groupValues[key] = v
	}
	out := make([]any, groupArray.Len())
	for row, value := range groupArray.Values() {
		out[row] = groupValues[evalValueKey(value)]
	}
	return data.InferArray(out), nil
}

func evalValueKey(v any) string {
	if data.IsNull(v) {
		return "null"
	}
	return fmt.Sprintf("%T:%#v", v, v)
}

func parseFbyAggregate(src string) (string, string, error) {
	src = strings.TrimSpace(src)
	for _, name := range []string{"sum", "sums", "avg", "var", "dev", "med", "min", "max", "first", "last", "count"} {
		if src == name {
			return "", "", fmt.Errorf("fby %s requires a value vector", name)
		}
		prefix := name + " "
		if strings.HasPrefix(src, prefix) {
			return name, strings.TrimSpace(src[len(prefix):]), nil
		}
	}
	return "", "", fmt.Errorf("fby left operand must be an aggregate expression")
}

func parseUnaryComposition(src string) ([]qUnaryFunction, bool) {
	parts := splitTopLevelFields(src)
	if len(parts) < 2 {
		return nil, false
	}
	funcs := make([]qUnaryFunction, 0, len(parts))
	for _, part := range parts {
		fn, ok := lookupUnaryVerb(part)
		if !ok {
			return nil, false
		}
		funcs = append(funcs, qUnaryFunction{name: part, fn: fn})
	}
	return funcs, true
}

func splitTopLevelFields(src string) []string {
	var parts []string
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	start := -1
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			if start < 0 {
				start = i
			}
			inString = true
		case '(':
			if start < 0 {
				start = i
			}
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			if start < 0 {
				start = i
			}
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			if start < 0 {
				start = i
			}
			braceDepth++
		case '}':
			braceDepth--
		case ' ', '\t', '\n', '\r':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				if start >= 0 {
					parts = append(parts, strings.TrimSpace(src[start:i]))
					start = -1
				}
			}
		default:
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		parts = append(parts, strings.TrimSpace(src[start:]))
	}
	return parts
}

func applyFbyAggregate(name string, values data.Array) (any, error) {
	switch name {
	case "sum":
		return sum(values)
	case "sums":
		return sums(values)
	case "avg":
		return avg(values)
	case "var":
		return varValue(values)
	case "dev":
		return devValue(values)
	case "med":
		return medValue(values)
	case "min":
		return minValue(values)
	case "max":
		return maxValue(values)
	case "first":
		return first(values)
	case "last":
		return last(values)
	case "count":
		return count(values)
	default:
		return nil, fmt.Errorf("fby aggregate %q is not supported", name)
	}
}

func lookupDyadicVerb(verb string) (byte, string, bool) {
	switch strings.TrimSpace(verb) {
	case "+", "plus":
		return '+', "plus", true
	case "-", "minus":
		return '-', "minus", true
	case "*", "times":
		return '*', "times", true
	case "%", "divide":
		return '%', "divide", true
	case "div":
		return 'd', "div", true
	case "mod":
		return 'r', "mod", true
	case "^", "fill":
		return '^', "fill", true
	case "~", "match":
		return '~', "match", true
	case "=", "equal", "equals":
		return '=', "equal", true
	case "<", "less":
		return '<', "less", true
	case ">", "more", "greater":
		return '>', "more", true
	case "min":
		return 'm', "min", true
	case "max":
		return 'M', "max", true
	case "[", "left":
		return 'L', "left", true
	case "]", "right":
		return 'R', "right", true
	case "&", "and":
		return '&', "and", true
	case "|", "or":
		return '|', "or", true
	default:
		return 0, "", false
	}
}

func lookupDyadicVerbFunc(verb string) (func(any, any) (any, error), bool) {
	verb = strings.TrimSpace(verb)
	if op, _, ok := lookupDyadicVerb(verb); ok {
		return dyadicVerbFunc(op), true
	}
	switch verb {
	case "bin":
		return bin, true
	case "binr":
		return binr, true
	case "xbar":
		return xbar, true
	case "xrank":
		return xrank, true
	case "msum":
		return msum, true
	case "mavg":
		return mavg, true
	case "mcount":
		return mcount, true
	case "mmin":
		return mmin, true
	case "mmax":
		return mmax, true
	case "mdev":
		return mdevValue, true
	case "ema":
		return emaValue, true
	case "xprev":
		return xprev, true
	case "xcols":
		return xcols, true
	case "xkey":
		return xkey, true
	case "xgroup":
		return xgroup, true
	case "xasc":
		return xasc, true
	case "xdesc":
		return xdesc, true
	case "rotate":
		return rotateValue, true
	case "cut":
		return qCutValue, true
	case "sublist":
		return qSublistValue, true
	case "cross":
		return qCrossValue, true
	case "ss":
		return qSSValue, true
	case "ssr":
		return qSSRWithSourceValue, true
	case "sv":
		return qSVValue, true
	case "vs":
		return qVSValue, true
	case "mmu":
		return matrixMultiplyValue, true
	case "xexp":
		return xexpValue, true
	case "xlog":
		return xlogValue, true
	case "wavg":
		return wavg, true
	case "wsum":
		return wsumValue, true
	case "cov":
		return covValue, true
	case "scov":
		return scovValue, true
	case "cor":
		return corValue, true
	case "within":
		return within, true
	case "like":
		return likeValue, true
	case "in":
		return membership, true
	case "intersect", "inter":
		return inter, true
	case "except":
		return except, true
	case "union":
		return union, true
	default:
		return nil, false
	}
}

func lookupUnaryVerb(verb string) (func(any) (any, error), bool) {
	switch strings.TrimSpace(verb) {
	case "count":
		return count, true
	case "enlist":
		return enlist, true
	case "type":
		return typeOf, true
	case "string":
		return stringValue, true
	case "lower":
		return lowerValue, true
	case "upper":
		return upperValue, true
	case "trim":
		return qTrimValue, true
	case "ltrim":
		return qLTrimValue, true
	case "rtrim":
		return qRTrimValue, true
	case "ssr":
		return qSSRValue, true
	case "distinct":
		return distinct, true
	case "first":
		return first, true
	case "last":
		return last, true
	case "keys":
		return keys, true
	case "key":
		return keys, true
	case "value":
		return value, true
	case "cols":
		return cols, true
	case "meta":
		return meta, true
	case "attr":
		return attrValue, true
	case "codes":
		return enumCodes, true
	case "domain":
		return enumDomainValues, true
	case "group":
		return group, true
	case "ungroup":
		return ungroup, true
	case "raze":
		return raze, true
	case "avg":
		return avg, true
	case "var":
		return varValue, true
	case "dev":
		return devValue, true
	case "svar":
		return svarValue, true
	case "sdev":
		return sdevValue, true
	case "wsum":
		return wsumUnaryValue, true
	case "med":
		return medValue, true
	case "prd":
		return prd, true
	case "min":
		return minValue, true
	case "max":
		return maxValue, true
	case "reverse":
		return reverse, true
	case "prior":
		return prev, true
	case "prev":
		return prev, true
	case "next":
		return nextValue, true
	case "deltas":
		return deltas, true
	case "fills":
		return fills, true
	case "differ":
		return differ, true
	case "ratios":
		return ratios, true
	case "asc":
		return asc, true
	case "desc":
		return desc, true
	case "iasc":
		return iasc, true
	case "idesc":
		return idesc, true
	case "rank":
		return rank, true
	case "neg":
		return negValue, true
	case "abs":
		return absValue, true
	case "sqrt":
		return sqrtValue, true
	case "log":
		return logValue, true
	case "exp":
		return expValue, true
	case "sin":
		return sinValue, true
	case "cos":
		return cosValue, true
	case "tan":
		return tanValue, true
	case "asin":
		return asinValue, true
	case "acos":
		return acosValue, true
	case "atan":
		return atanValue, true
	case "reciprocal":
		return reciprocalValue, true
	case "signum":
		return signumValue, true
	case "floor":
		return floorValue, true
	case "ceiling":
		return ceilingValue, true
	case "inv":
		return matrixInverseValue, true
	case "sum":
		return sum, true
	case "sums":
		return sums, true
	case "prds":
		return prds, true
	case "mins":
		return mins, true
	case "maxs":
		return maxs, true
	case "avgs":
		return avgs, true
	case "all":
		return allValue, true
	case "any":
		return anyValue, true
	case "where":
		return where, true
	case "not":
		return notValue, true
	case "null":
		return nullValue, true
	default:
		return nil, false
	}
}

func isCallable(v any) bool {
	switch v.(type) {
	case qLambda, qProjection, qAdverbFunction, qCallableAdverb, qDyadicFunction, qUnaryFunction, qComposition, *qIPCHandle:
		return true
	default:
		return false
	}
}

func (s *EvalState) applyCallableIndex(fn any, argSrc string) (any, error) {
	args, hasMissing, err := s.parseCallableArgs(argSrc)
	if err != nil {
		return nil, err
	}
	if hasMissing {
		return qProjection{fn: fn, args: args}, nil
	}
	switch len(args) {
	case 0:
		return s.applyCallable(fn, nil)
	case 1:
		return s.applyCallable1(fn, args[0].value)
	case 2:
		return s.applyCallable2(fn, args[0].value, args[1].value)
	case 3:
		return s.applyCallable3(fn, args[0].value, args[1].value, args[2].value)
	}
	values := make([]any, len(args))
	for i, arg := range args {
		values[i] = arg.value
	}
	return s.applyCallable(fn, values)
}

func (s *EvalState) parseCallableArgs(src string) ([]projectionArg, bool, error) {
	parts := splitQBracketFormArgs(src)
	args := make([]projectionArg, len(parts))
	hasMissing := false
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			args[i] = projectionArg{missing: true}
			hasMissing = true
			continue
		}
		v, err := s.eval(part)
		if err != nil {
			return nil, false, err
		}
		args[i] = projectionArg{value: v}
	}
	return args, hasMissing, nil
}

func (s *EvalState) applyCallable(fn any, args []any) (any, error) {
	switch f := fn.(type) {
	case qLambda:
		return s.applyLambda(f, args)
	case qUnaryFunction:
		return applyUnaryFunction(f, args)
	case qComposition:
		return applyComposition(f, args)
	case qDyadicFunction:
		return applyDyadicFunction(f, args)
	case qAdverbFunction:
		return applyAdverbFunction(f, args)
	case qCallableAdverb:
		return s.applyCallableAdverbFunction(f, args)
	case *qIPCHandle:
		return s.applyIPCHandle(f, args)
	case qProjection:
		projected := make([]projectionArg, 0, len(f.args))
		merged := make([]any, 0, len(f.args)+len(args))
		next := 0
		hasMissing := false
		for _, arg := range f.args {
			if arg.missing {
				if next >= len(args) {
					projected = append(projected, projectionArg{missing: true})
					hasMissing = true
					continue
				}
				projected = append(projected, projectionArg{value: args[next]})
				merged = append(merged, args[next])
				next++
				continue
			}
			projected = append(projected, arg)
			merged = append(merged, arg.value)
		}
		if hasMissing {
			return qProjection{fn: f.fn, args: projected}, nil
		}
		merged = append(merged, args[next:]...)
		return s.applyCallable(f.fn, merged)
	default:
		return nil, fmt.Errorf("value is not callable")
	}
}

func (s *EvalState) applyCallable1(fn any, arg any) (any, error) {
	switch f := fn.(type) {
	case qLambda:
		return s.applyLambda1(f, arg)
	case qUnaryFunction:
		return f.fn(arg)
	case qProjection:
		return s.applyCallable(f, []any{arg})
	default:
		return s.applyCallable(fn, []any{arg})
	}
}

func (s *EvalState) applyCallable2(fn any, left, right any) (any, error) {
	switch f := fn.(type) {
	case qLambda:
		return s.applyLambda2(f, left, right)
	case qDyadicFunction:
		return f.fn(left, right)
	case qProjection:
		return s.applyCallable(f, []any{left, right})
	default:
		return s.applyCallable(fn, []any{left, right})
	}
}

func (s *EvalState) applyCallable3(fn any, arg0, arg1, arg2 any) (any, error) {
	switch f := fn.(type) {
	case qLambda:
		return s.applyLambda(f, []any{arg0, arg1, arg2})
	case qProjection:
		return s.applyCallable(f, []any{arg0, arg1, arg2})
	default:
		return s.applyCallable(fn, []any{arg0, arg1, arg2})
	}
}

func (s *EvalState) applyIPCHandle(h *qIPCHandle, args []any) (any, error) {
	if h == nil || h.session == nil {
		return nil, fmt.Errorf("q IPC handle is closed")
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("q IPC handle call expected 1 command argument, got %d", len(args))
	}
	value, err := h.session.evalIPCMessage(args[0])
	if err != nil {
		return nil, fmt.Errorf("q IPC %s: %w", h.target, err)
	}
	if h.async {
		return data.NullValue, nil
	}
	return value, nil
}

func (s *EvalState) evalIPCMessage(message any) (any, error) {
	if list, ok := message.(data.Array); ok {
		items := list.Values()
		if len(items) == 0 {
			return nil, fmt.Errorf("q IPC message list is empty")
		}
		return s.evalIPCListMessage(items)
	}
	command, ok := message.(string)
	if !ok {
		return nil, fmt.Errorf("q IPC handle call expects a string command or message list")
	}
	return s.evalIPCStringCommand(command, nil)
}

func (s *EvalState) evalIPCListMessage(items []any) (any, error) {
	head := items[0]
	args := items[1:]
	switch fn := head.(type) {
	case string:
		return s.evalIPCStringCommand(fn, args)
	case data.Symbol:
		v, ok := s.env[string(fn)]
		if !ok {
			return nil, fmt.Errorf("q IPC function %q is not defined", fn)
		}
		if !isCallable(v) {
			return nil, fmt.Errorf("q IPC function %q is not callable", fn)
		}
		return s.applyCallable(v, args)
	default:
		if isCallable(fn) {
			return s.applyCallable(fn, args)
		}
		return nil, fmt.Errorf("q IPC message list head must be a command string or callable")
	}
}

func (s *EvalState) evalIPCStringCommand(command string, args []any) (any, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("q IPC handle command is empty")
	}
	if strings.HasPrefix(command, "\\") {
		return nil, fmt.Errorf("q IPC system commands are not supported")
	}
	restore := s.bindIPCArgs(args)
	value, err := s.Eval(command)
	restore()
	if err != nil {
		return nil, err
	}
	if len(args) > 0 && isCallable(value) {
		return s.applyCallable(value, args)
	}
	return value, nil
}

func (s *EvalState) bindIPCArgs(args []any) func() {
	if len(args) == 0 {
		return func() {}
	}
	names := make([]string, 0, len(args)+3)
	for i := range args {
		names = append(names, fmt.Sprintf("x%d", i))
	}
	for i, name := range []string{"x", "y", "z"} {
		if i < len(args) {
			names = append(names, name)
		}
	}
	type oldValue struct {
		value any
		ok    bool
	}
	old := make(map[string]oldValue, len(names))
	for _, name := range names {
		if _, exists := old[name]; exists {
			continue
		}
		v, ok := s.env[name]
		old[name] = oldValue{value: v, ok: ok}
	}
	for i, arg := range args {
		s.env[fmt.Sprintf("x%d", i)] = arg
	}
	if len(args) > 0 {
		s.env["x"] = args[0]
	}
	if len(args) > 1 {
		s.env["y"] = args[1]
	}
	if len(args) > 2 {
		s.env["z"] = args[2]
	}
	return func() {
		for name, previous := range old {
			if previous.ok {
				s.env[name] = previous.value
				continue
			}
			delete(s.env, name)
		}
	}
}

func applyUnaryFunction(fn qUnaryFunction, args []any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("unary function %s expected 1 argument, got %d", fn.name, len(args))
	}
	return fn.fn(args[0])
}

func applyComposition(fn qComposition, args []any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("composition expected 1 argument, got %d", len(args))
	}
	if isCountDistinctComposition(fn) {
		return countDistinct(args[0])
	}
	value := args[0]
	for i := len(fn.funcs) - 1; i >= 0; i-- {
		next, err := fn.funcs[i].fn(value)
		if err != nil {
			return nil, fmt.Errorf("composition %s failed: %w", fn.funcs[i].name, err)
		}
		value = next
	}
	return value, nil
}

func isCountDistinctComposition(fn qComposition) bool {
	return len(fn.funcs) == 2 && fn.funcs[0].name == "count" && fn.funcs[1].name == "distinct"
}

func applyDyadicFunction(fn qDyadicFunction, args []any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("dyadic function %s expected 2 arguments, got %d", fn.name, len(args))
	}
	return fn.fn(args[0], args[1])
}

func (s *EvalState) applyCallableAdverbFunction(fn qCallableAdverb, args []any) (any, error) {
	if err := validateCallableReduceScanBoundary(fn.fn, fn.adverb); err != nil {
		return nil, err
	}
	switch len(args) {
	case 1:
		switch fn.adverb {
		case "\\:", "/:":
			return nil, fmt.Errorf("callable %s expected 2 arguments, got 1", adverbName(fn.adverb))
		}
		return s.evalCallableAdverb(fn.fn, fn.adverb, args[0])
	case 2:
		switch fn.adverb {
		case "'":
			return s.applyEachDyadicCallable(fn.fn, args[0], args[1])
		case "/":
			return s.applyOverCallable(fn.fn, args[0], args[1])
		case "\\":
			return s.applyScanCallable(fn.fn, args[0], args[1])
		case "':":
			return s.applyEachPriorCallable(fn.fn, args[0], args[1])
		case "\\:":
			return s.applyEachLeftCallable(fn.fn, args[0], args[1])
		case "/:":
			return s.applyEachRightCallable(fn.fn, args[0], args[1])
		default:
			return nil, fmt.Errorf("callable adverb %q is not supported", fn.adverb)
		}
	default:
		switch fn.adverb {
		case "'":
			return nil, fmt.Errorf("callable each expected 1 or 2 arguments, got %d", len(args))
		case "/", "\\":
			return nil, fmt.Errorf("callable %s expected 1 or 2 arguments, got %d", adverbName(fn.adverb), len(args))
		case "':":
			return nil, fmt.Errorf("callable %s expected 1 or 2 arguments, got %d", adverbName(fn.adverb), len(args))
		case "\\:", "/:":
			return nil, fmt.Errorf("callable %s expected 2 arguments, got %d", adverbName(fn.adverb), len(args))
		default:
			return nil, fmt.Errorf("callable adverb %q is not supported", fn.adverb)
		}
	}
}

func applyAdverbFunction(fn qAdverbFunction, args []any) (any, error) {
	dyad, ok := lookupDyadicVerbFunc(fn.verb)
	if !ok {
		return nil, fmt.Errorf("%s cannot be used as a dyadic verb", fn.verb)
	}
	op, _, hasDyadicOp := lookupDyadicVerb(fn.verb)
	switch len(args) {
	case 1:
		switch fn.adverb {
		case "/":
			op, _, ok := lookupDyadicVerb(fn.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with over", fn.verb)
			}
			return applyOver(op, nil, args[0])
		case "\\":
			op, _, ok := lookupDyadicVerb(fn.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with scan", fn.verb)
			}
			return applyScan(op, nil, args[0])
		case "':":
			return applyEachPriorFunc(dyad, nil, args[0])
		case "'":
			return nil, fmt.Errorf("adverb function each expected 2 arguments, got 1")
		case "\\:", "/:":
			return nil, fmt.Errorf("adverb function %s expected 2 arguments, got 1", adverbName(fn.adverb))
		default:
			return nil, fmt.Errorf("adverb %q is not supported as a function", fn.adverb)
		}
	case 2:
		switch fn.adverb {
		case "'":
			if hasDyadicOp {
				return applyEachDyadic(op, args[0], args[1])
			}
			return applyEachDyadicFunc(dyad, args[0], args[1])
		case "/":
			if !hasDyadicOp {
				return nil, fmt.Errorf("%s cannot be used with over", fn.verb)
			}
			return applyOver(op, args[0], args[1])
		case "\\":
			if !hasDyadicOp {
				return nil, fmt.Errorf("%s cannot be used with scan", fn.verb)
			}
			return applyScan(op, args[0], args[1])
		case "':":
			return applyEachPriorFunc(dyad, args[0], args[1])
		case "\\:":
			if hasDyadicOp {
				return applyEachLeft(op, args[0], args[1])
			}
			return applyEachLeftFunc(dyad, args[0], args[1])
		case "/:":
			if hasDyadicOp {
				return applyEachRight(op, args[0], args[1])
			}
			return applyEachRightFunc(dyad, args[0], args[1])
		default:
			return nil, fmt.Errorf("adverb %q is not supported as a function", fn.adverb)
		}
	default:
		switch fn.adverb {
		case "/", "\\", "':":
			return nil, fmt.Errorf("adverb function %s expected 1 or 2 arguments, got %d", adverbName(fn.adverb), len(args))
		case "\\:", "/:":
			return nil, fmt.Errorf("adverb function %s expected 2 arguments, got %d", adverbName(fn.adverb), len(args))
		default:
			return nil, fmt.Errorf("adverb function expected 1 or 2 arguments, got %d", len(args))
		}
	}
}

func (s *EvalState) applyLambda(fn qLambda, args []any) (any, error) {
	params, body, err := lambdaSignature(fn.body)
	if err != nil {
		return nil, err
	}
	if len(args) > len(params) {
		return nil, fmt.Errorf("lambda expected at most %d arguments, got %d", len(params), len(args))
	}
	vars := cloneEnv(fn.env)
	state := NewEvalState(vars)
	state.namespace = fn.namespace
	for i, arg := range args {
		state.env[state.resolveAssignmentName(params[i])] = arg
	}
	return state.Eval(body)
}

func (s *EvalState) applyLambda1(fn qLambda, arg any) (any, error) {
	return s.applyLambda(fn, []any{arg})
}

func (s *EvalState) applyLambda2(fn qLambda, left, right any) (any, error) {
	if plan, ok := qLambdaFastPlanFor(fn.body); ok {
		switch plan.kind {
		case qLambdaFastDyadic:
			return applyDyadic(plan.op, left, right)
		case qLambdaFastSumPlusRight:
			sumValue, err := sum(left)
			if err != nil {
				return nil, err
			}
			return applyDyadic('+', sumValue, right)
		case qLambdaFastSumPlusCountRight:
			return qLambdaFastSumPlusCountRightValue(left, right)
		}
	}
	return s.applyLambda(fn, []any{left, right})
}

func qLambdaFastPlanFor(src string) (qLambdaFastPlan, bool) {
	params, body, err := lambdaSignature(src)
	if err != nil || len(params) < 2 {
		return qLambdaFastPlan{}, false
	}
	body = stripEnclosingParens(strings.TrimSpace(body))
	if op, ok := qLambdaFastDyadicOp(body, params[0], params[1]); ok {
		return qLambdaFastPlan{kind: qLambdaFastDyadic, op: op}, true
	}
	if qLambdaFastSumPlusParam(body, params[0], params[1]) {
		return qLambdaFastPlan{kind: qLambdaFastSumPlusRight}, true
	}
	if qLambdaFastSumPlusCountParam(body, params[0], params[1]) {
		return qLambdaFastPlan{kind: qLambdaFastSumPlusCountRight}, true
	}
	return qLambdaFastPlan{}, false
}

func qLambdaFastDyadicOp(body, leftParam, rightParam string) (byte, bool) {
	for _, candidate := range []struct {
		token string
		op    byte
		word  bool
	}{
		{token: "+", op: '+'},
		{token: "-", op: '-'},
		{token: "*", op: '*'},
		{token: "%", op: '%'},
		{token: "mod", op: 'r', word: true},
		{token: "div", op: 'd', word: true},
	} {
		var left, right string
		var ok bool
		if candidate.word {
			left, right, ok = splitTopLevelWord(body, candidate.token)
		} else {
			left, right, ok = splitTopLevelOperator(body, candidate.token)
		}
		if !ok {
			continue
		}
		if stripEnclosingParens(left) == leftParam && stripEnclosingParens(right) == rightParam {
			return candidate.op, true
		}
	}
	return 0, false
}

func qLambdaFastSumPlusParam(body, sumParam, rightParam string) bool {
	left, right, ok := splitTopLevelOperator(body, "+")
	if !ok {
		return false
	}
	left = stripEnclosingParens(left)
	right = stripEnclosingParens(right)
	return (qLambdaFastSumTerm(left, sumParam) && right == rightParam) ||
		(qLambdaFastSumTerm(right, sumParam) && left == rightParam)
}

func qLambdaFastSumPlusCountParam(body, sumParam, rightParam string) bool {
	left, right, ok := splitTopLevelOperator(body, "+")
	if !ok {
		return false
	}
	left = stripEnclosingParens(left)
	right = stripEnclosingParens(right)
	return (qLambdaFastSumTerm(left, sumParam) && qLambdaFastCountTerm(right, rightParam)) ||
		(qLambdaFastSumTerm(right, sumParam) && qLambdaFastCountTerm(left, rightParam))
}

func qLambdaFastSumTerm(src, param string) bool {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		return stripEnclosingParens(strings.TrimSpace(src[len("+/"):])) == param
	}
	if strings.HasPrefix(src, "sum") && wordBoundary(src, 0, len("sum")) {
		return stripEnclosingParens(strings.TrimSpace(src[len("sum"):])) == param
	}
	return false
}

func qLambdaFastCountTerm(src, param string) bool {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "#") {
		return stripEnclosingParens(strings.TrimSpace(src[len("#"):])) == param
	}
	if strings.HasPrefix(src, "count") && wordBoundary(src, 0, len("count")) {
		return stripEnclosingParens(strings.TrimSpace(src[len("count"):])) == param
	}
	return false
}

func qLambdaFastSumPlusCountRightValue(left, right any) (any, error) {
	leftArray, ok := left.(data.Array)
	if !ok {
		sumValue, err := sum(left)
		if err != nil {
			return nil, err
		}
		countValue, err := count(right)
		if err != nil {
			return nil, err
		}
		return applyDyadic('+', sumValue, countValue)
	}
	countValue, err := count(right)
	if err != nil {
		return nil, err
	}
	shape := "callable-dot/sum-plus-count/" + string(leftArray.Kind()) + "/" + string(qRuntimeKernelOperandKind(countValue, nil))
	sumValue, handled, err := data.TryTypedNumericSum(leftArray)
	sumValue, handled, err = qTypedRuntimeResultReason("CallableDotSumPlusCount", shape, RuntimeFallbackUnsupportedType, sumValue, handled, err)
	if err != nil {
		return nil, err
	}
	if !handled {
		sumValue, err = sum(left)
		if err != nil {
			return nil, err
		}
	}
	return applyDyadic('+', sumValue, countValue)
}

func lambdaSignature(src string) ([]string, string, error) {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "[") {
		end := findMatchingDelimiter(src, 0, '[', ']')
		if end < 0 {
			return nil, "", fmt.Errorf("lambda parameter list is not closed")
		}
		paramSrc := strings.TrimSpace(src[1:end])
		parts := []string(nil)
		if paramSrc != "" {
			parts = splitQBracketFormArgs(paramSrc)
		}
		params := make([]string, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if !isQBareName(name) {
				return nil, "", fmt.Errorf("invalid lambda parameter %q", name)
			}
			params = append(params, name)
		}
		return params, strings.TrimSpace(src[end+1:]), nil
	}
	return []string{"x", "y", "z"}, src, nil
}

func findLambdaAdverb(src string) (string, string, string, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "{") {
		return "", "", "", false
	}
	end := findMatchingDelimiter(src, 0, '{', '}')
	if end < 0 {
		return "", "", "", false
	}
	rest := strings.TrimSpace(src[end+1:])
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if strings.HasPrefix(rest, adverb) {
			right := strings.TrimSpace(rest[len(adverb):])
			if strings.HasPrefix(right, "[") {
				return "", "", "", false
			}
			return src[:end+1], adverb, right, right != ""
		}
	}
	return "", "", "", false
}

func findLambdaAdverbFunction(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "{") {
		return "", "", false
	}
	end := findMatchingDelimiter(src, 0, '{', '}')
	if end < 0 {
		return "", "", false
	}
	rest := strings.TrimSpace(src[end+1:])
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if rest == adverb {
			return src[:end+1], adverb, true
		}
	}
	return "", "", false
}

func (s *EvalState) findCallableAdverbFunction(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if !strings.HasSuffix(src, adverb) {
			continue
		}
		left := strings.TrimSpace(src[:len(src)-len(adverb)])
		if left == "" {
			continue
		}
		if _, ok := lookupDyadicVerbFunc(left); ok {
			return left, adverb, true
		}
		if !s.isPotentialCallableExpr(left) {
			continue
		}
		return left, adverb, true
	}
	return "", "", false
}

func (s *EvalState) findCallablePostfixAdverb(src string) (string, string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
			continue
		case '(':
			parenDepth++
			continue
		case ')':
			parenDepth--
			continue
		case '[':
			bracketDepth++
			continue
		case ']':
			bracketDepth--
			continue
		case '{':
			braceDepth++
			continue
		case '}':
			braceDepth--
			continue
		}
		if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
			continue
		}
		adverb := ""
		switch {
		case strings.HasPrefix(src[i:], "':"):
			adverb = "':"
		case strings.HasPrefix(src[i:], "\\:"):
			adverb = "\\:"
		case strings.HasPrefix(src[i:], "/:"):
			adverb = "/:"
		case src[i] == '\'':
			adverb = "'"
		case src[i] == '/':
			adverb = "/"
		case src[i] == '\\':
			adverb = "\\"
		default:
			continue
		}
		left := strings.TrimSpace(src[:i])
		right := strings.TrimSpace(src[i+len(adverb):])
		if left == "" || right == "" {
			continue
		}
		if adverb == "':" && strings.HasPrefix(right, "[") {
			continue
		}
		if !s.isPotentialCallableExpr(left) {
			continue
		}
		return left, adverb, right, true
	}
	return "", "", "", false
}

func (s *EvalState) isPotentialCallableExpr(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	if isQAssignmentName(src) {
		if v, ok := s.env[src]; ok && isCallable(v) {
			return true
		}
	}
	if strings.HasSuffix(src, "}") || strings.HasSuffix(src, "]") || strings.HasSuffix(src, ")") {
		return true
	}
	return false
}

func validateCallableReduceScanBoundary(fn any, adverb string) error {
	if adverb != "/" && adverb != "\\" {
		return nil
	}
	if _, ok := fn.(qComposition); ok {
		return fmt.Errorf("composition cannot be used with %s", adverbName(adverb))
	}
	dyad, ok := fn.(qDyadicFunction)
	if !ok {
		return nil
	}
	if _, _, ok := lookupDyadicVerb(dyad.name); ok {
		return nil
	}
	switch adverb {
	case "/":
		return fmt.Errorf("%s cannot be used with over", dyad.name)
	case "\\":
		return fmt.Errorf("%s cannot be used with scan", dyad.name)
	default:
		return nil
	}
}

func (s *EvalState) evalCallableAdverb(fn any, adverb string, right any) (any, error) {
	if err := validateCallableReduceScanBoundary(fn, adverb); err != nil {
		return nil, err
	}
	switch adverb {
	case "'":
		return s.applyEachCallable(fn, right)
	case "/":
		return s.applyOverCallable(fn, nil, right)
	case "\\":
		return s.applyScanCallable(fn, nil, right)
	case "':":
		return s.applyEachPriorCallable(fn, nil, right)
	case "\\:", "/:":
		return nil, fmt.Errorf("callable %s expected 2 arguments, got 1", adverbName(adverb))
	default:
		return nil, fmt.Errorf("callable adverb %q is not supported", adverb)
	}
}

func adverbName(adverb string) string {
	switch adverb {
	case "/":
		return "over"
	case "\\":
		return "scan"
	case "'":
		return "each"
	case "':":
		return "each-prior"
	case "/:":
		return "each-right"
	case "\\:":
		return "each-left"
	default:
		return "adverb"
	}
}

func (s *EvalState) applyEachCallable(fn any, v any) (any, error) {
	if dict, ok := v.(EvalDict); ok {
		out := EvalDict{
			Keys:   append([]any(nil), dict.Keys...),
			Values: make([]any, len(dict.Values)),
		}
		if isCountDistinctCallable(fn) {
			for i, item := range dict.Values {
				value, err := countDistinct(item)
				if err != nil {
					return nil, err
				}
				out.Values[i] = value
			}
			recordRuntimeKernelProbe("CallableEachCountDistinct", "dict", true, nil)
			return out, nil
		}
		for i, item := range dict.Values {
			value, err := s.applyCallable(fn, []any{item})
			if err != nil {
				return nil, err
			}
			out.Values[i] = value
		}
		return out, nil
	}
	items, err := vectorValues(v)
	if err != nil {
		return s.applyCallable(fn, []any{v})
	}
	out := make([]any, len(items))
	if isCountDistinctCallable(fn) {
		for i, item := range items {
			value, err := countDistinct(item)
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		recordRuntimeKernelProbe("CallableEachCountDistinct", "vector", true, nil)
		return inferQArray(out, data.KindI64), nil
	}
	for i, item := range items {
		value, err := s.applyCallable(fn, []any{item})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(v)), nil
}

func isCountDistinctCallable(fn any) bool {
	composition, ok := fn.(qComposition)
	return ok && isCountDistinctComposition(composition)
}

func (s *EvalState) applyEachDyadicCallable(fn any, left any, right any) (any, error) {
	if isUnaryOnlyCallable(fn) {
		return s.applyCallable(fn, []any{right})
	}
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if !lok && !rok {
		return s.applyCallable(fn, []any{left, right})
	}
	if lok && rok && la.Len() != ra.Len() {
		return nil, fmt.Errorf("callable each length mismatch")
	}
	n := 0
	switch {
	case lok:
		n = la.Len()
	case rok:
		n = ra.Len()
	}
	out := make([]any, n)
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if lok {
			var ok bool
			lv, ok = la.At(i)
			if !ok {
				return nil, fmt.Errorf("callable each left row %d out of range", i)
			}
		}
		if rok {
			var ok bool
			rv, ok = ra.At(i)
			if !ok {
				return nil, fmt.Errorf("callable each right row %d out of range", i)
			}
		}
		value, err := s.applyCallable(fn, []any{lv, rv})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func isUnaryOnlyCallable(fn any) bool {
	switch fn.(type) {
	case qUnaryFunction, qComposition:
		return true
	default:
		return false
	}
}

func (s *EvalState) applyEachLeftCallable(fn any, left any, right any) (any, error) {
	items, err := vectorValues(right)
	if err != nil {
		return s.applyCallable(fn, []any{left, right})
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := s.applyCallable(fn, []any{left, item})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(right) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func (s *EvalState) applyEachRightCallable(fn any, left any, right any) (any, error) {
	items, err := vectorValues(left)
	if err != nil {
		return s.applyCallable(fn, []any{left, right})
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := s.applyCallable(fn, []any{item, right})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(left) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func (s *EvalState) applyOverCallable(fn any, initial any, v any) (any, error) {
	if adverbFn, ok := fn.(qAdverbFunction); ok && adverbFn.verb == "+" && adverbFn.adverb == "/" && initial == nil {
		return sum(v)
	}
	if isCallableAdd(fn) {
		reduced, err := sum(v)
		if err != nil {
			return nil, err
		}
		if initial == nil {
			return reduced, nil
		}
		return applyDyadic('+', initial, reduced)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return s.applyCallable(fn, []any{initial, v})
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil {
			return initial, nil
		}
		return data.NullValue, nil
	}
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := s.applyCallable(fn, []any{acc, items[i]})
		if err != nil {
			return nil, err
		}
		acc = next
	}
	return acc, nil
}

func (s *EvalState) applyScanCallable(fn any, initial any, v any) (any, error) {
	if adverbFn, ok := fn.(qAdverbFunction); ok && adverbFn.verb == "+" && adverbFn.adverb == "\\" && initial == nil {
		return sums(v)
	}
	if isCallableAdd(fn) {
		scan, err := sums(v)
		if err != nil {
			return nil, err
		}
		if initial == nil {
			return scan, nil
		}
		return applyDyadic('+', scan, initial)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return s.applyCallable(fn, []any{initial, v})
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil {
			return data.InferArray(nil), nil
		}
		return inferQArray(nil, qKindOfValue(initial), qKindOfValue(v)), nil
	}
	out := make([]any, len(items))
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		out[0] = acc
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := s.applyCallable(fn, []any{acc, items[i]})
		if err != nil {
			return nil, err
		}
		acc = next
		out[i] = acc
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func isCallableAdd(fn any) bool {
	switch f := fn.(type) {
	case qDyadicFunction:
		return f.name == "+"
	case qLambda:
		plan, ok := qLambdaFastPlanFor(f.body)
		return ok && plan.kind == qLambdaFastDyadic && plan.op == '+'
	default:
		return false
	}
}

func (s *EvalState) applyEachPriorCallable(fn any, initial any, v any) (any, error) {
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return s.applyCallable(fn, []any{v, initial})
		}
		return v, nil
	}
	out := make([]any, len(items))
	var previous any
	if initial != nil {
		previous = initial
	}
	for i, item := range items {
		if i == 0 && initial == nil {
			out[i] = item
		} else {
			value, err := s.applyCallable(fn, []any{item, previous})
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		previous = item
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func applyEachUnary(verb string, v any) (any, error) {
	fn, ok := lookupUnaryVerb(verb)
	if !ok {
		return nil, fmt.Errorf("%s cannot be used monadically with each", verb)
	}
	if dict, ok := v.(EvalDict); ok {
		out := EvalDict{
			Keys:   append([]any(nil), dict.Keys...),
			Values: make([]any, len(dict.Values)),
		}
		for i, item := range dict.Values {
			value, err := fn(item)
			if err != nil {
				return nil, err
			}
			out.Values[i] = value
		}
		return out, nil
	}
	items, err := vectorValues(v)
	if err != nil {
		return fn(v)
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := fn(item)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(v)), nil
}

func applyEachDyadic(op byte, left, right any) (any, error) {
	if out, handled, err := tryApplyTypedAdverbDyadic(op, "'", left, right); err != nil || handled {
		return out, err
	}
	return applyEachDyadicFunc(dyadicVerbFunc(op), left, right)
}

func tryApplyTypedAdverbDyadic(op byte, adverb string, left, right any) (any, bool, error) {
	if _, ok := qDataArithmeticOp(op); !ok {
		return nil, false, nil
	}
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if !lok && !rok {
		return nil, false, nil
	}
	shape := qAdverbArithmeticShape(adverb, op, left, right, la, ra)
	if adverb == "'" && lok && rok && la.Len() != ra.Len() {
		err := fmt.Errorf("each length mismatch")
		recordQTypedRuntimeKernelReason("QAdverbArithmetic", shape, false, err, RuntimeFallbackSemanticGuard)
		return nil, true, err
	}
	out, err := applyDyadic(op, left, right)
	if err != nil {
		recordQTypedRuntimeKernelReason("QAdverbArithmetic", shape, false, err, RuntimeFallbackRuntimeError)
		return nil, true, err
	}
	if _, ok := out.(data.Array); !ok {
		recordQTypedRuntimeKernelReason("QAdverbArithmetic", shape, false, nil, RuntimeFallbackUnsupportedType)
		return nil, false, nil
	}
	recordQTypedRuntimeKernel("QAdverbArithmetic", shape, true, nil)
	return out, true, nil
}

func qAdverbArithmeticShape(adverb string, op byte, left, right any, la, ra data.Array) string {
	return "adverb-dyadic/" + qAdverbShapeName(adverb) + "/" + string(op) + "/" + string(qRuntimeKernelOperandKind(left, la)) + "/" + string(qRuntimeKernelOperandKind(right, ra))
}

func qAdverbShapeName(adverb string) string {
	switch adverb {
	case "'":
		return "each"
	case "\\:":
		return "each-left"
	case "/:":
		return "each-right"
	case "':":
		return "each-prior"
	case "/":
		return "over"
	case "\\":
		return "scan"
	default:
		return "adverb"
	}
}

func applyEachDyadicFunc(fn func(any, any) (any, error), left, right any) (any, error) {
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if !lok && !rok {
		return fn(left, right)
	}
	if lok && rok && la.Len() != ra.Len() {
		return nil, fmt.Errorf("each length mismatch")
	}
	n := 0
	switch {
	case lok:
		n = la.Len()
	case rok:
		n = ra.Len()
	}
	out := make([]any, n)
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if lok {
			var ok bool
			lv, ok = la.At(i)
			if !ok {
				return nil, fmt.Errorf("left vector row %d out of range", i)
			}
		}
		if rok {
			var ok bool
			rv, ok = ra.At(i)
			if !ok {
				return nil, fmt.Errorf("right vector row %d out of range", i)
			}
		}
		value, err := fn(lv, rv)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func applyEachPrior(op byte, initial any, v any) (any, error) {
	return applyEachPriorFunc(dyadicVerbFunc(op), initial, v)
}

func applyEachPriorFunc(fn func(any, any) (any, error), initial any, v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		if initial != nil {
			return fn(v, initial)
		}
		return v, nil
	}
	out := make([]any, array.Len())
	var previous any
	if initial != nil {
		previous = initial
	}
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("each-prior row %d out of range", i)
		}
		if i == 0 && initial == nil {
			out[i] = item
		} else {
			value, err := fn(item, previous)
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		previous = item
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func applyEachLeft(op byte, left, right any) (any, error) {
	if out, handled, err := tryApplyTypedAdverbDyadic(op, "\\:", left, right); err != nil || handled {
		return out, err
	}
	return applyEachLeftFunc(dyadicVerbFunc(op), left, right)
}

func applyEachLeftFunc(fn func(any, any) (any, error), left, right any) (any, error) {
	items, err := vectorValues(right)
	if err != nil {
		return fn(left, right)
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := fn(left, item)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(right) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func applyEachRight(op byte, left, right any) (any, error) {
	if out, handled, err := tryApplyTypedAdverbDyadic(op, "/:", left, right); err != nil || handled {
		return out, err
	}
	return applyEachRightFunc(dyadicVerbFunc(op), left, right)
}

func applyEachRightFunc(fn func(any, any) (any, error), left, right any) (any, error) {
	items, err := vectorValues(left)
	if err != nil {
		return fn(left, right)
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := fn(item, right)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(left) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func isTypedEmptyAdverbSource(v any) bool {
	array, ok := v.(data.Array)
	if !ok || array.Len() != 0 {
		return false
	}
	kind := array.Kind()
	return kind != "" && kind != data.KindNull && kind != data.KindAny
}

func applyOver(op byte, initial any, v any) (any, error) {
	if op == '+' && initial == nil {
		return sum(v)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return applyDyadic(op, initial, v)
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil {
			return initial, nil
		}
		return data.NullValue, nil
	}
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := applyDyadic(op, acc, items[i])
		if err != nil {
			return nil, err
		}
		acc = next
	}
	return acc, nil
}

func applyScan(op byte, initial any, v any) (any, error) {
	if op == '+' && initial == nil {
		return sums(v)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return applyDyadic(op, initial, v)
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil && !isTypedEmptyAdverbSource(v) {
			return data.InferArray(nil), nil
		}
		return inferQArray(nil, qKindOfValue(initial), qKindOfValue(v)), nil
	}
	out := make([]any, len(items))
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		out[0] = acc
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := applyDyadic(op, acc, items[i])
		if err != nil {
			return nil, err
		}
		acc = next
		out[i] = acc
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func parseSymbolList(src string) ([]data.Symbol, error) {
	if !strings.HasPrefix(src, "`") {
		return nil, fmt.Errorf("symbol list must start with `")
	}
	var out []data.Symbol
	for len(src) > 0 {
		if src[0] != '`' {
			return nil, fmt.Errorf("malformed symbol list near %q", src)
		}
		src = src[1:]
		next := strings.IndexByte(src, '`')
		var sym string
		if next < 0 {
			sym = strings.TrimSpace(src)
			src = ""
		} else {
			sym = strings.TrimSpace(src[:next])
			src = src[next:]
		}
		if sym == "" {
			return nil, fmt.Errorf("empty symbol")
		}
		out = append(out, data.Symbol(sym))
	}
	return out, nil
}

func parseAtomOrVector(src string) (any, error) {
	if strings.HasPrefix(src, "\"") && strings.HasSuffix(src, "\"") {
		v, err := strconv.Unquote(src)
		if err == nil {
			return v, nil
		}
	}
	if value, ok, err := parseQBoolLiteral(src); ok || err != nil {
		return value, err
	}
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty q expression")
	}
	values := make([]any, len(fields))
	temporalKinds := make([]data.Kind, len(fields))
	hasTemporal := false
	hasFloat := false
	hasNull := false
	hasBool := false
	hasTypedScalar := false
	for i, field := range fields {
		if strings.HasPrefix(field, "\"") {
			v, err := strconv.Unquote(field)
			if err != nil {
				return nil, fmt.Errorf("invalid string %q", field)
			}
			values[i] = v
			continue
		}
		if kind, text, ok := parseTemporalToken(field); ok {
			v, err := parseQTemporal(kind, text)
			if err != nil {
				return nil, err
			}
			temporalKinds[i] = data.Kind(kind)
			hasTemporal = true
			values[i] = v
			continue
		}
		v, isFloat, err := parseNumberOrBool(field)
		if err != nil {
			return nil, err
		}
		values[i] = v
		hasFloat = hasFloat || isFloat
		hasNull = hasNull || data.IsNull(v)
		if _, ok := v.(bool); ok {
			hasBool = true
		}
		switch v.(type) {
		case int16, int32, float32:
			hasTypedScalar = true
		}
	}
	if len(values) == 1 {
		return values[0], nil
	}
	if kind, ok := commonTypedNullKind(values); ok && allValuesNull(values) {
		column, err := data.NewColumnWithKind("_", kind, values)
		if err != nil {
			return nil, err
		}
		return column.Data, nil
	}
	if _, ok := values[0].(string); ok {
		xs := make([]string, len(values))
		for i, v := range values {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("mixed string and non-string vectors are not supported")
			}
			xs[i] = s
		}
		return data.NewString(xs), nil
	}
	if hasTemporal {
		for _, v := range values {
			if data.IsNull(v) {
				continue
			}
			if temporalKindOfValue(v) == "" {
				return nil, fmt.Errorf("mixed temporal and non-temporal vectors are not supported")
			}
		}
		kind, err := coerceTemporalVectorValues(values, temporalKinds)
		if err != nil {
			return nil, err
		}
		column, err := data.NewColumnWithKind("_", kind, values)
		if err != nil {
			return nil, err
		}
		return column.Data, nil
	}
	if hasTypedScalar {
		return data.NewColumn("_", values).Data, nil
	}
	if hasFloat {
		xs := make([]float64, len(values))
		for i, v := range values {
			if data.IsNull(v) {
				continue
			}
			n, ok := numeric(v)
			if !ok {
				return data.InferArray(values), nil
			}
			xs[i] = n
		}
		if hasNull {
			return data.NewColumn("_", values).Data, nil
		}
		return data.NewF64(xs), nil
	}
	if _, ok := values[0].(bool); ok {
		xs := make([]bool, len(values))
		for i, v := range values {
			if data.IsNull(v) {
				continue
			}
			b, ok := v.(bool)
			if !ok {
				return data.InferArray(values), nil
			}
			xs[i] = b
		}
		if hasNull {
			return data.NewColumn("_", values).Data, nil
		}
		return data.NewBool(xs), nil
	}
	if hasNull {
		return data.NewColumn("_", values).Data, nil
	}
	if hasBool {
		return data.InferArray(values), nil
	}
	xs := make([]int64, len(values))
	for i, v := range values {
		xs[i] = v.(int64)
	}
	return data.NewI64(xs), nil
}

func looksLikeTemporalVector(src string) bool {
	fields := strings.Fields(src)
	if len(fields) <= 1 {
		return false
	}
	hasTemporal := false
	for _, field := range fields {
		if _, _, ok := parseTemporalToken(field); ok {
			hasTemporal = true
			continue
		}
		if _, _, err := parseNumberOrBool(field); err == nil {
			continue
		}
		return false
	}
	return hasTemporal
}

func looksLikeLiteralVector(src string) bool {
	if value, ok, err := parseQBoolLiteral(src); ok && err == nil {
		if _, isArray := value.(data.Array); isArray {
			return true
		}
		return false
	}
	fields := strings.Fields(src)
	if len(fields) <= 1 {
		return false
	}
	for _, field := range fields {
		if strings.HasPrefix(field, "\"") {
			if _, err := strconv.Unquote(field); err != nil {
				return false
			}
			continue
		}
		if containsInternalDyadicSign(field) {
			return false
		}
		if _, _, ok := parseTemporalToken(field); ok {
			continue
		}
		if _, _, err := parseNumberOrBool(field); err == nil {
			continue
		}
		return false
	}
	return true
}

func containsInternalDyadicSign(field string) bool {
	for i := 1; i < len(field); i++ {
		if field[i] == '+' || field[i] == '-' {
			return true
		}
	}
	return false
}

func parseNumberOrBool(src string) (any, bool, error) {
	if strings.HasPrefix(src, "-") || strings.HasPrefix(src, "+") {
		sign := src[0]
		body := src[1:]
		if v, isFloat, ok := parseQInfinity(body); ok {
			if sign == '-' {
				v = negateQInfinity(v)
			}
			return v, isFloat, nil
		}
	}
	switch src {
	case "true":
		return true, false, nil
	case "false":
		return false, false, nil
	case "0N", "0n":
		return data.NullValue, false, nil
	case "0Nb", "0Nx", "0Nc", "0Nh", "0Ni", "0Nj", "0Ne", "0Nf":
		switch src {
		case "0Nb":
			return data.NullForKind(data.KindBool), false, nil
		case "0Nx":
			return data.NullForKind(data.KindU8), false, nil
		case "0Nc":
			return data.NullForKind(data.KindString), false, nil
		case "0Nh":
			return data.NullForKind(data.KindI16), false, nil
		case "0Ni":
			return data.NullForKind(data.KindI32), false, nil
		case "0Nj":
			return data.NullForKind(data.KindI64), false, nil
		case "0Ne":
			return data.NullForKind(data.KindF32), true, nil
		case "0Nf":
			return data.NullForKind(data.KindF64), true, nil
		}
	}
	if v, isFloat, ok := parseQInfinity(src); ok {
		return v, isFloat, nil
	}
	if len(src) > 1 {
		suffix := src[len(src)-1]
		body := src[:len(src)-1]
		switch suffix {
		case 'h':
			i, err := strconv.ParseInt(body, 10, 16)
			if err != nil {
				return nil, false, fmt.Errorf("invalid short literal %q", src)
			}
			return int16(i), false, nil
		case 'i':
			i, err := strconv.ParseInt(body, 10, 32)
			if err != nil {
				return nil, false, fmt.Errorf("invalid int literal %q", src)
			}
			return int32(i), false, nil
		case 'j':
			i, err := strconv.ParseInt(body, 10, 64)
			if err != nil {
				return nil, false, fmt.Errorf("invalid long literal %q", src)
			}
			return i, false, nil
		case 'e':
			f, err := strconv.ParseFloat(body, 32)
			if err != nil {
				return nil, false, fmt.Errorf("invalid real literal %q", src)
			}
			return float32(f), true, nil
		case 'f':
			f, err := strconv.ParseFloat(body, 64)
			if err != nil {
				return nil, false, fmt.Errorf("invalid float literal %q", src)
			}
			return f, true, nil
		}
	}
	if strings.ContainsAny(src, ".eE") {
		f, err := strconv.ParseFloat(src, 64)
		if err != nil {
			return nil, false, fmt.Errorf("invalid number %q", src)
		}
		return f, true, nil
	}
	i, err := strconv.ParseInt(src, 10, 64)
	if err != nil {
		return nil, false, fmt.Errorf("invalid number %q", src)
	}
	return i, false, nil
}

func commonTypedNullKind(values []any) (data.Kind, bool) {
	kind := data.Kind("")
	for _, value := range values {
		valueKind, ok := data.NullKind(value)
		if !ok || valueKind == data.KindNull || valueKind == data.KindAny {
			return "", false
		}
		merged, ok := mergeQResultKinds(kind, valueKind)
		if !ok {
			return "", false
		}
		kind = merged
	}
	return kind, kind != ""
}

func allValuesNull(values []any) bool {
	for _, value := range values {
		if !data.IsNull(value) {
			return false
		}
	}
	return len(values) > 0
}

func parseQInfinity(src string) (any, bool, bool) {
	switch src {
	case "0W", "0Wj":
		return int64(math.MaxInt64), false, true
	case "0Wh":
		return int16(math.MaxInt16), false, true
	case "0Wi":
		return int32(math.MaxInt32), false, true
	case "0w", "0Wf":
		return math.Inf(1), true, true
	case "0We":
		return float32(math.Inf(1)), true, true
	default:
		return nil, false, false
	}
}

func negateQInfinity(v any) any {
	switch x := v.(type) {
	case int16:
		return -x
	case int32:
		return -x
	case int64:
		return -x
	case float32:
		return float32(math.Inf(-1))
	case float64:
		return math.Inf(-1)
	default:
		return v
	}
}

func parseTakeCount(src string) (int, error) {
	v, err := parseAtomOrVector(src)
	if err != nil {
		return 0, fmt.Errorf("# left operand must be an integer count")
	}
	i, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("# left operand must be an integer count")
	}
	return int(i), nil
}

func (s *EvalState) evalTake(src string) (any, error) {
	fields := strings.Fields(src)
	if len(fields) < 2 {
		return nil, fmt.Errorf("take expects a count and value")
	}
	n, err := parseTakeCount(fields[0])
	if err != nil {
		return nil, err
	}
	v, err := s.eval(strings.TrimSpace(src[len(fields[0]):]))
	if err != nil {
		return nil, err
	}
	return take(n, v)
}

func (s *EvalState) evalDrop(src string) (any, error) {
	fields := strings.Fields(src)
	if len(fields) < 2 {
		return nil, fmt.Errorf("drop expects a count and value")
	}
	n, err := parseTakeCount(fields[0])
	if err != nil {
		return nil, err
	}
	v, err := s.eval(strings.TrimSpace(src[len(fields[0]):]))
	if err != nil {
		return nil, err
	}
	return drop(n, v)
}

func (s *EvalState) evalTil(src string) (any, error) {
	v, err := s.eval(src)
	if err != nil {
		return nil, err
	}
	n, ok := v.(int64)
	if !ok {
		return nil, fmt.Errorf("til expects an integer")
	}
	if n < 0 {
		return nil, fmt.Errorf("til expects a non-negative integer")
	}
	if int64(int(n)) != n {
		return nil, fmt.Errorf("til count is too large")
	}
	return data.NewI64Range(0, 1, int(n)), nil
}

func (s *EvalState) evalLookup(src string) (any, error) {
	dictExpr, keyExpr, ok := splitLookupArgs(src)
	if !ok {
		return nil, fmt.Errorf("lookup expects a dictionary and key")
	}
	d, err := s.eval(dictExpr)
	if err != nil {
		return nil, err
	}
	key, err := s.eval(keyExpr)
	if err != nil {
		return nil, err
	}
	return indexValue(d, key)
}

func splitLookupArgs(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", false
	}
	if strings.HasPrefix(src, "(") {
		depth := 0
		for i := 0; i < len(src); i++ {
			switch src[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					rest := strings.TrimSpace(src[i+1:])
					return strings.TrimSpace(src[:i+1]), rest, rest != ""
				}
			}
		}
		return "", "", false
	}
	fields := strings.Fields(src)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], strings.TrimSpace(src[len(fields[0]):]), true
}

func (s *EvalState) evalAmend(src string) (any, error) {
	if !strings.HasPrefix(src, "@[") || !strings.HasSuffix(src, "]") {
		return nil, fmt.Errorf("amend expects @[dict;key;op;value]")
	}
	inner := strings.TrimSpace(src[2 : len(src)-1])
	parts := splitQBracketFormArgs(inner)
	if len(parts) != 4 {
		return nil, fmt.Errorf("amend expects @[dict;key;op;value]")
	}
	d, err := s.eval(parts[0])
	if err != nil {
		return nil, err
	}
	key, err := s.eval(parts[1])
	if err != nil {
		return nil, err
	}
	value, err := s.eval(parts[3])
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parts[2]) == ":" {
		return amendValue(d, key, value)
	}
	op, err := s.eval(parts[2])
	if err != nil {
		return nil, err
	}
	return s.amendValueWithFunction(d, key, op, value)
}

func (s *EvalState) evalDotAmend(src string) (any, error) {
	if !strings.HasPrefix(src, ".[") || !strings.HasSuffix(src, "]") {
		return nil, fmt.Errorf("dot amend expects .[dict;path;op;value]")
	}
	inner := strings.TrimSpace(src[2 : len(src)-1])
	parts := splitQBracketFormArgs(inner)
	if len(parts) != 4 {
		return nil, fmt.Errorf("dot amend expects .[dict;path;op;value]")
	}
	target, err := s.eval(parts[0])
	if err != nil {
		return nil, err
	}
	pathValue, err := s.eval(parts[1])
	if err != nil {
		return nil, err
	}
	pathItems, err := amendPathItems(pathValue)
	if err != nil {
		return nil, err
	}
	value, err := s.eval(parts[3])
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parts[2]) == ":" {
		return s.amendPath(target, pathItems, nil, value)
	}
	op, err := s.eval(parts[2])
	if err != nil {
		return nil, err
	}
	return s.amendPath(target, pathItems, op, value)
}

func amendPathItems(v any) ([]any, error) {
	if array, ok := v.(data.Array); ok {
		return array.Values(), nil
	}
	return []any{v}, nil
}

func (s *EvalState) amendPath(target any, path []any, op any, value any) (any, error) {
	if len(path) == 0 {
		if op == nil {
			return value, nil
		}
		return s.applyCallable(op, []any{target, value})
	}
	if len(path) == 1 {
		if op == nil {
			return amendValue(target, path[0], value)
		}
		return s.amendValueWithFunction(target, path[0], op, value)
	}
	child, err := indexValue(target, path[0])
	if err != nil {
		return nil, err
	}
	next, err := s.amendPath(child, path[1:], op, value)
	if err != nil {
		return nil, err
	}
	return setContainerChild(target, path[0], next)
}

func dict(keys any, values any) (EvalDict, error) {
	keyItems, err := dictKeyItems(keys)
	if err != nil {
		return EvalDict{}, err
	}
	items, err := vectorValues(values)
	if err != nil {
		if len(keyItems) != 1 {
			return EvalDict{}, fmt.Errorf("dict key/value length mismatch")
		}
		items = []any{values}
	}
	if len(keyItems) != len(items) {
		return EvalDict{}, fmt.Errorf("dict key/value length mismatch")
	}
	return EvalDict{Keys: append([]any(nil), keyItems...), Values: append([]any(nil), items...)}, nil
}

func dictKeyItems(keys any) ([]any, error) {
	switch x := keys.(type) {
	case data.Array:
		items := x.Values()
		out := make([]any, len(items))
		copy(out, items)
		return out, nil
	default:
		return []any{x}, nil
	}
}

func dictLookup(v any, key any) (any, error) {
	d, ok := v.(EvalDict)
	if !ok {
		return nil, fmt.Errorf("lookup expects a dictionary")
	}
	keys, scalar, err := lookupKeys(key)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		var value interface{} = data.NullValue
		for j, existing := range d.Keys {
			if equalValue(existing, k) {
				value = d.Values[j]
				break
			}
		}
		if value == nil {
			value = data.NullValue
		}
		out[i] = value
	}
	if scalar {
		return out[0], nil
	}
	return data.InferArray(out), nil
}

func indexValue(v any, index any) (any, error) {
	if _, ok := v.(EvalDict); ok {
		return dictLookup(v, index)
	}
	if frame, ok := v.(data.Frame); ok {
		return frameColumnLookup(frame, index)
	}
	if keyed, ok := v.(data.KeyedFrame); ok {
		if frameHasLookupColumns(keyed.Frame(), index) {
			return frameColumnLookup(keyed.Frame(), index)
		}
		return keyedTableLookup(keyed, index)
	}
	if array, ok := v.(data.Array); ok {
		if indexArray, ok := index.(data.Array); ok && indexArray.Len() != 1 {
			out, handled, err := data.TryGatherByI64IndexArray(array, indexArray)
			recordRuntimeKernelProbe("ArrayGatherI64Indexes", "gather/"+string(array.Kind())+"/"+string(indexArray.Kind()), handled, err)
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
		}
	}
	indexes, scalar, err := indexInts(index)
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case data.Array:
		if scalar {
			if row, handled, err := data.TryRowArrayIndex(x, indexes[0]); err != nil || handled {
				shape := "matrix-row/" + string(x.Kind()) + "/" + qRuntimeCardinalityShape(rowArrayLen(row))
				recordRuntimeKernelProbe("ArrayMatrixRowIndex", shape, handled, err)
				if err != nil {
					return nil, err
				}
				return row, nil
			}
			item, ok := x.At(indexes[0])
			if !ok {
				return nil, fmt.Errorf("index %d out of range", indexes[0])
			}
			return item, nil
		}
		return data.Gather(x, indexes)
	case string:
		runes := []rune(x)
		if scalar {
			if indexes[0] < 0 || indexes[0] >= len(runes) {
				return nil, fmt.Errorf("index %d out of range", indexes[0])
			}
			return string(runes[indexes[0]]), nil
		}
		out := make([]string, len(indexes))
		for i, index := range indexes {
			if index < 0 || index >= len(runes) {
				return nil, fmt.Errorf("index %d out of range", index)
			}
			out[i] = string(runes[index])
		}
		return data.NewString(out), nil
	default:
		return nil, fmt.Errorf("index expects a vector, string, or dictionary")
	}
}

func rowArrayLen(row data.Array) int {
	if row == nil {
		return 0
	}
	return row.Len()
}

func frameColumnLookup(frame data.Frame, key any) (any, error) {
	keys, scalar, err := lookupKeys(key)
	if err != nil {
		return nil, err
	}
	cols := make([]data.Column, 0, len(keys))
	for _, raw := range keys {
		sym, ok := raw.(data.Symbol)
		if !ok {
			if s, ok := raw.(string); ok {
				sym = data.Symbol(s)
			} else {
				return nil, fmt.Errorf("table column lookup expects symbol keys")
			}
		}
		col, ok := frame.Column(sym)
		if !ok {
			return nil, fmt.Errorf("table column %q not found", sym)
		}
		if scalar {
			return col, nil
		}
		cols = append(cols, data.Column{Name: sym, Data: col})
	}
	return data.NewFrame(cols...)
}

func frameHasLookupColumns(frame data.Frame, key any) bool {
	keys, _, err := lookupKeys(key)
	if err != nil || len(keys) == 0 {
		return false
	}
	for _, raw := range keys {
		sym, ok := raw.(data.Symbol)
		if !ok {
			if s, ok := raw.(string); ok {
				sym = data.Symbol(s)
			} else {
				return false
			}
		}
		if _, ok := frame.Column(sym); !ok {
			return false
		}
	}
	return true
}

func keyedTableLookup(keyed data.KeyedFrame, key any) (data.Frame, error) {
	if selector, ok := key.(EvalDict); ok {
		keyValues, err := keyedLookupDictValues(keyed, selector)
		if err != nil {
			return data.Frame{}, err
		}
		frame, err := keyed.LookupValueByKey(keyValues...)
		if err != nil {
			return data.Frame{}, err
		}
		return frame, nil
	}
	keyValues := []any{key}
	if array, ok := key.(data.Array); ok && len(keyed.Keys()) > 1 {
		keyValues = array.Values()
	}
	frame, err := keyed.LookupValueByKey(keyValues...)
	if err != nil {
		return data.Frame{}, err
	}
	return frame, nil
}

func keyedLookupDictValues(keyed data.KeyedFrame, selector EvalDict) ([]any, error) {
	keyValues := make([]any, 0, len(keyed.Keys()))
	for _, name := range keyed.Keys() {
		found := false
		for i, rawKey := range selector.Keys {
			sym, ok := rawKey.(data.Symbol)
			if !ok {
				return nil, fmt.Errorf("keyed table lookup selector keys must be symbols")
			}
			if sym != name {
				continue
			}
			keyValues = append(keyValues, selector.Values[i])
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("keyed table lookup selector missing key column %q", name)
		}
	}
	return keyValues, nil
}

func indexInts(v any) ([]int, bool, error) {
	switch x := v.(type) {
	case int64:
		if x < 0 || int64(int(x)) != x {
			return nil, false, fmt.Errorf("index must be a non-negative integer")
		}
		return []int{int(x)}, true, nil
	case data.Array:
		if x.Kind() != data.KindI64 {
			return nil, false, fmt.Errorf("index vector must contain integers")
		}
		if indexes, handled, err := data.TryTypedI64Indexes(x); err != nil || handled {
			return indexes, len(indexes) == 1, err
		}
		values := x.Values()
		indexes := make([]int, len(values))
		for i, value := range values {
			n, ok := value.(int64)
			if !ok || n < 0 || int64(int(n)) != n {
				return nil, false, fmt.Errorf("index vector must contain non-negative integers")
			}
			indexes[i] = int(n)
		}
		return indexes, len(indexes) == 1, nil
	default:
		return nil, false, fmt.Errorf("index must be an integer or integer vector")
	}
}

func amendValue(v any, key any, value any) (any, error) {
	if d, ok := v.(EvalDict); ok {
		return dictAmend(d, key, value)
	}
	if array, ok := v.(data.Array); ok {
		return vectorAmend(array, key, value)
	}
	if frame, ok := v.(data.Frame); ok {
		return frameAmend(frame, key, value)
	}
	if keyed, ok := v.(data.KeyedFrame); ok {
		return keyedFrameAmend(keyed, key, value)
	}
	return nil, fmt.Errorf("amend expects a dictionary, vector, table, or keyed table")
}

func setContainerChild(target any, key any, value any) (any, error) {
	switch x := target.(type) {
	case EvalDict:
		return dictAmend(x, key, value)
	case data.Array:
		return vectorAmend(x, key, value)
	case data.Frame:
		if name, err := symbolName(key); err == nil {
			array, ok := value.(data.Array)
			if !ok {
				return nil, fmt.Errorf("table dot amend column %q expects a vector", name)
			}
			return frameColumnAmend(x, name, array)
		}
		return frameAmend(x, key, value)
	case data.KeyedFrame:
		if name, err := symbolName(key); err == nil {
			array, ok := value.(data.Array)
			if !ok {
				return nil, fmt.Errorf("keyed table dot amend column %q expects a vector", name)
			}
			frame, err := frameColumnAmend(x.Frame(), name, array)
			if err != nil {
				return nil, err
			}
			return data.KeyBy(frame, x.Keys()...)
		}
		return keyedFrameAmend(x, key, value)
	default:
		return nil, fmt.Errorf("dot amend path expects a dictionary, vector, table, or keyed table")
	}
}

func frameColumnAmend(frame data.Frame, name data.Symbol, array data.Array) (data.Frame, error) {
	if array.Len() != frame.Len() {
		return data.Frame{}, fmt.Errorf("table dot amend column %q length %d does not match row count %d", name, array.Len(), frame.Len())
	}
	schema := frame.Schema()
	kind, ok := schema.Kind(name)
	if !ok {
		return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
	}
	cols := make([]data.Column, 0, len(schema.Names()))
	for _, colName := range schema.Names() {
		if colName != name {
			col, ok := frame.Column(colName)
			if !ok {
				return data.Frame{}, fmt.Errorf("table amend column %q does not exist", colName)
			}
			cols = append(cols, data.Column{Name: colName, Data: col})
			continue
		}
		values := array.Values()
		next, err := data.NewColumnWithKind(name, kind, values)
		if err != nil {
			return data.Frame{}, err
		}
		cols = append(cols, next)
	}
	return data.NewFrame(cols...)
}

func (s *EvalState) amendValueWithFunction(v any, key any, fn any, value any) (any, error) {
	if !isCallable(fn) {
		return nil, fmt.Errorf("amend function is not callable")
	}
	switch x := v.(type) {
	case EvalDict:
		return s.dictAmendFunction(x, key, fn, value)
	case data.Array:
		return s.vectorAmendFunction(x, key, fn, value)
	case data.Frame:
		return s.frameAmendFunction(x, key, fn, value)
	case data.KeyedFrame:
		return s.keyedFrameAmendFunction(x, key, fn, value)
	default:
		return nil, fmt.Errorf("amend expects a dictionary, vector, table, or keyed table")
	}
}

func (s *EvalState) dictAmendFunction(d EvalDict, key any, fn any, value any) (EvalDict, error) {
	keys, _, err := lookupKeys(key)
	if err != nil {
		return EvalDict{}, err
	}
	values, err := amendInputValues(value, len(keys))
	if err != nil {
		return EvalDict{}, err
	}
	out := d
	for i, k := range keys {
		old, err := dictLookup(out, k)
		if err != nil {
			return EvalDict{}, err
		}
		next, err := s.applyCallable(fn, []any{old, values[i]})
		if err != nil {
			return EvalDict{}, err
		}
		amended, err := dictAmend(out, k, next)
		if err != nil {
			return EvalDict{}, err
		}
		out = amended
	}
	return out, nil
}

func (s *EvalState) vectorAmendFunction(array data.Array, key any, fn any, value any) (data.Array, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return nil, err
	}
	values, err := amendInputValues(value, len(indexes))
	if err != nil {
		return nil, err
	}
	nextValues := make([]any, len(indexes))
	for i, index := range indexes {
		old, ok := array.At(index)
		if !ok {
			return nil, fmt.Errorf("amend index %d out of range", index)
		}
		next, err := s.applyCallable(fn, []any{old, values[i]})
		if err != nil {
			return nil, err
		}
		nextValues[i] = next
	}
	return vectorAmendIndexes(array, indexes, nextValues)
}

func (s *EvalState) frameAmendFunction(frame data.Frame, key any, fn any, value any) (data.Frame, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return data.Frame{}, err
	}
	for _, index := range indexes {
		if index < 0 || index >= frame.Len() {
			return data.Frame{}, fmt.Errorf("amend row %d out of range", index)
		}
	}
	updates, err := tableAmendRows(value, len(indexes))
	if err != nil {
		return data.Frame{}, err
	}
	for row, update := range updates {
		for name, nextValue := range update {
			col, ok := frame.Column(name)
			if !ok {
				return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
			}
			old, ok := col.At(indexes[row])
			if !ok {
				return data.Frame{}, fmt.Errorf("amend row %d out of range", indexes[row])
			}
			updated, err := s.applyCallable(fn, []any{old, nextValue})
			if err != nil {
				return data.Frame{}, err
			}
			update[name] = updated
		}
	}
	return applyFrameRowUpdates(frame, indexes, updates)
}

func (s *EvalState) keyedFrameAmendFunction(keyed data.KeyedFrame, key any, fn any, value any) (data.KeyedFrame, error) {
	keyRow, err := keyedSelectorRow(keyed, key)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	updates, err := tableAmendRows(value, 1)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	row := updates[0]
	for name, keyValue := range keyRow {
		if existing, ok := row[name]; ok && !equalValue(existing, keyValue) {
			return data.KeyedFrame{}, fmt.Errorf("keyed table amend key column %q conflicts with selector", name)
		}
		delete(row, name)
		_ = keyValue
	}
	hit, err := keyed.LookupValueByKey(keyedSelectorValues(keyed, keyRow)...)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	if hit.Len() == 1 {
		amended, err := s.frameAmendFunction(hit, int64(0), fn, evalDictFromRow(row))
		if err != nil {
			return data.KeyedFrame{}, err
		}
		row, err = amended.Row(0)
		if err != nil {
			return data.KeyedFrame{}, err
		}
	} else {
		for name, nextValue := range row {
			kind, ok := keyed.Frame().Schema().Kind(name)
			if !ok {
				return data.KeyedFrame{}, fmt.Errorf("table amend column %q does not exist", name)
			}
			old := keyedMissingAmendOldValue(fn, kind)
			updated, err := s.applyCallable(fn, []any{old, nextValue})
			if err != nil {
				return data.KeyedFrame{}, err
			}
			row[name] = updated
		}
	}
	for name, keyValue := range keyRow {
		row[name] = keyValue
	}
	delta, err := rowMapsFrame([]map[data.Symbol]any{row}, keyed.Frame().Schema().Names(), true)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.UpsertKeyed(keyed, delta)
}

func keyedMissingAmendOldValue(fn any, kind data.Kind) any {
	if dyad, ok := fn.(qDyadicFunction); ok && dyad.name == "+" {
		if zero, ok := zeroValueForKind(kind); ok {
			return zero
		}
	}
	return data.NullForKind(kind)
}

func zeroValueForKind(kind data.Kind) (any, bool) {
	switch kind {
	case data.KindBool:
		return false, true
	case data.KindI8:
		return int8(0), true
	case data.KindI16:
		return int16(0), true
	case data.KindI32:
		return int32(0), true
	case data.KindI64:
		return int64(0), true
	case data.KindU8:
		return uint8(0), true
	case data.KindU16:
		return uint16(0), true
	case data.KindU32:
		return uint32(0), true
	case data.KindU64:
		return uint64(0), true
	case data.KindF32:
		return float32(0), true
	case data.KindF64:
		return float64(0), true
	default:
		return nil, false
	}
}

func keyedSelectorValues(keyed data.KeyedFrame, row map[data.Symbol]any) []any {
	values := make([]any, 0, len(keyed.Keys()))
	for _, name := range keyed.Keys() {
		values = append(values, row[name])
	}
	return values
}

func evalDictFromRow(row map[data.Symbol]any) EvalDict {
	keys := make([]any, 0, len(row))
	values := make([]any, 0, len(row))
	for name, value := range row {
		keys = append(keys, name)
		values = append(values, value)
	}
	return EvalDict{Keys: keys, Values: values}
}

func amendInputValues(value any, count int) ([]any, error) {
	if count == 1 {
		return []any{value}, nil
	}
	items, err := vectorValues(value)
	if err != nil {
		return nil, fmt.Errorf("amend value length mismatch")
	}
	if len(items) != count {
		return nil, fmt.Errorf("amend value length mismatch")
	}
	return items, nil
}

func vectorAmend(array data.Array, key any, value any) (data.Array, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return nil, err
	}
	values := make([]any, len(indexes))
	if len(indexes) == 1 {
		values[0] = value
	} else {
		items, err := vectorValues(value)
		if err != nil {
			return nil, fmt.Errorf("amend value length mismatch")
		}
		if len(items) != len(indexes) {
			return nil, fmt.Errorf("amend value length mismatch")
		}
		copy(values, items)
	}
	return vectorAmendIndexes(array, indexes, values)
}

func vectorAmendIndexes(array data.Array, indexes []int, values []any) (data.Array, error) {
	if len(indexes) != len(values) {
		return nil, fmt.Errorf("amend value length mismatch")
	}
	if typed, handled, err := data.TryTypedAmendIndexes(array, indexes, values); err != nil || handled {
		recordRuntimeKernelProbe("ArrayAmendIndexes", "amend-indexes/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, err
		}
		return typed, nil
	} else {
		recordRuntimeKernelProbe("ArrayAmendIndexes", "amend-indexes/"+string(array.Kind()), handled, err)
	}
	out := array.Values()
	for i, index := range indexes {
		if index < 0 || index >= len(out) {
			return nil, fmt.Errorf("amend index %d out of range", index)
		}
		out[index] = normalizeVectorAmendValue(array.Kind(), values[i])
	}
	return inferQArray(out, array.Kind()), nil
}

func normalizeVectorAmendValue(kind data.Kind, value any) any {
	if kind == "" || kind == data.KindAny || kind == data.KindNull {
		return value
	}
	normalized, err := data.NormalizeValueForKind(kind, value)
	if err != nil {
		return value
	}
	return normalized
}

func frameAmend(frame data.Frame, key any, value any) (data.Frame, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return data.Frame{}, err
	}
	for _, index := range indexes {
		if index < 0 || index >= frame.Len() {
			return data.Frame{}, fmt.Errorf("amend row %d out of range", index)
		}
	}
	updates, err := tableAmendRows(value, len(indexes))
	if err != nil {
		return data.Frame{}, err
	}
	return applyFrameRowUpdates(frame, indexes, updates)
}

func keyedFrameAmend(keyed data.KeyedFrame, key any, value any) (data.KeyedFrame, error) {
	keyRow, err := keyedSelectorRow(keyed, key)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	updates, err := tableAmendRows(value, 1)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	row := updates[0]
	for name, keyValue := range keyRow {
		if existing, ok := row[name]; ok && !equalValue(existing, keyValue) {
			return data.KeyedFrame{}, fmt.Errorf("keyed table amend key column %q conflicts with selector", name)
		}
		row[name] = keyValue
	}
	delta, err := rowMapsFrame([]map[data.Symbol]any{row}, keyed.Frame().Schema().Names(), true)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.UpsertKeyed(keyed, delta)
}

func keyedSelectorRow(keyed data.KeyedFrame, key any) (map[data.Symbol]any, error) {
	keys := keyed.Keys()
	if len(keys) == 0 {
		return nil, fmt.Errorf("keyed table is not initialized")
	}
	values := []any{key}
	if len(keys) > 1 {
		array, ok := key.(data.Array)
		if !ok {
			return nil, fmt.Errorf("keyed table amend expects %d key values", len(keys))
		}
		values = array.Values()
	}
	if len(values) != len(keys) {
		return nil, fmt.Errorf("keyed table amend expects %d key values", len(keys))
	}
	out := make(map[data.Symbol]any, len(keys))
	for i, name := range keys {
		out[name] = values[i]
	}
	return out, nil
}

func applyFrameRowUpdates(frame data.Frame, indexes []int, updates []map[data.Symbol]any) (data.Frame, error) {
	if len(indexes) != len(updates) {
		return data.Frame{}, fmt.Errorf("amend update row count mismatch")
	}
	schema := frame.Schema()
	known := make(map[data.Symbol]struct{}, len(schema.Names()))
	for _, name := range schema.Names() {
		known[name] = struct{}{}
	}
	for _, update := range updates {
		for name := range update {
			if _, ok := known[name]; !ok {
				return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
			}
		}
	}
	cols := make([]data.Column, 0, len(schema.Names()))
	for _, name := range schema.Names() {
		col, ok := frame.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
		}
		values := col.Values()
		for i, index := range indexes {
			if v, ok := updates[i][name]; ok {
				values[index] = v
			}
		}
		next, err := data.NewColumnWithKind(name, col.Kind(), values)
		if err != nil {
			return data.Frame{}, err
		}
		cols = append(cols, next)
	}
	return data.NewFrame(cols...)
}

func tableAmendRows(value any, count int) ([]map[data.Symbol]any, error) {
	if count <= 0 {
		return nil, fmt.Errorf("amend requires at least one target row")
	}
	switch x := value.(type) {
	case EvalDict:
		return dictRowUpdates(x, count)
	case data.Frame:
		return frameRowUpdates(x, count)
	case data.KeyedFrame:
		return frameRowUpdates(x.Frame(), count)
	default:
		return nil, fmt.Errorf("table amend expects a dictionary or table value")
	}
}

func dictRowUpdates(d EvalDict, count int) ([]map[data.Symbol]any, error) {
	out := make([]map[data.Symbol]any, count)
	for i := range out {
		out[i] = make(map[data.Symbol]any, len(d.Keys))
	}
	for i, rawKey := range d.Keys {
		name, err := symbolName(rawKey)
		if err != nil {
			return nil, fmt.Errorf("table amend: %w", err)
		}
		values := make([]any, count)
		if array, ok := d.Values[i].(data.Array); ok && array.Len() == count {
			copy(values, array.Values())
		} else {
			for row := range values {
				values[row] = d.Values[i]
			}
		}
		for row := range out {
			out[row][name] = values[row]
		}
	}
	return out, nil
}

func frameRowUpdates(frame data.Frame, count int) ([]map[data.Symbol]any, error) {
	if frame.Len() != count {
		return nil, fmt.Errorf("table amend value row count %d does not match target row count %d", frame.Len(), count)
	}
	out := make([]map[data.Symbol]any, count)
	for row := 0; row < count; row++ {
		values, err := frame.Row(row)
		if err != nil {
			return nil, err
		}
		out[row] = values
	}
	return out, nil
}

func rowMapsFrame(rows []map[data.Symbol]any, preferred []data.Symbol, allowNewColumns bool) (data.Frame, error) {
	if len(rows) == 0 {
		return data.Frame{}, fmt.Errorf("table amend requires at least one row")
	}
	names := make([]data.Symbol, 0, len(preferred))
	seen := make(map[data.Symbol]struct{}, len(preferred))
	for _, name := range preferred {
		if name == "" {
			continue
		}
		if _, ok := rows[0][name]; !ok && !allowNewColumns {
			continue
		}
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for _, row := range rows {
		for name := range row {
			if _, ok := seen[name]; ok {
				continue
			}
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}
	cols := make([]data.Column, 0, len(names))
	for _, name := range names {
		values := make([]any, len(rows))
		for row := range rows {
			if v, ok := rows[row][name]; ok {
				values[row] = v
			} else {
				values[row] = data.NullValue
			}
		}
		cols = append(cols, data.NewColumn(name, values))
	}
	return data.NewFrame(cols...)
}

func symbolName(v any) (data.Symbol, error) {
	switch x := v.(type) {
	case data.Symbol:
		return x, nil
	case string:
		if x == "" {
			return "", fmt.Errorf("column name must not be empty")
		}
		return data.Symbol(x), nil
	default:
		return "", fmt.Errorf("table amend column names must be symbols")
	}
}

func dictAmend(d EvalDict, key any, value any) (EvalDict, error) {
	keys, _, err := lookupKeys(key)
	if err != nil {
		return EvalDict{}, err
	}
	values := make([]any, len(keys))
	if len(keys) == 1 {
		values[0] = value
	} else {
		items, err := vectorValues(value)
		if err != nil {
			return EvalDict{}, fmt.Errorf("amend value length mismatch")
		}
		if len(items) != len(keys) {
			return EvalDict{}, fmt.Errorf("amend value length mismatch")
		}
		copy(values, items)
	}

	out := EvalDict{
		Keys:   append([]any(nil), d.Keys...),
		Values: append([]any(nil), d.Values...),
	}
	for i, key := range keys {
		found := false
		for j, existing := range out.Keys {
			if equalValue(existing, key) {
				out.Values[j] = values[i]
				found = true
				break
			}
		}
		if !found {
			out.Keys = append(out.Keys, key)
			out.Values = append(out.Values, values[i])
		}
	}
	return out, nil
}

func lookupKeys(v any) ([]any, bool, error) {
	switch x := v.(type) {
	case data.Symbol:
		return []any{x}, true, nil
	case data.Array:
		keys := x.Values()
		return keys, len(keys) == 1, nil
	default:
		return []any{x}, true, nil
	}
}

func dictSymbolKeys(d EvalDict) ([]data.Symbol, error) {
	keys := make([]data.Symbol, len(d.Keys))
	for i, key := range d.Keys {
		sym, ok := key.(data.Symbol)
		if !ok {
			return nil, fmt.Errorf("dictionary key %d is %T, want symbol", i, key)
		}
		keys[i] = sym
	}
	return keys, nil
}

func keyedTableByCount(keys any, values any) (any, bool, error) {
	frame, ok := values.(data.Frame)
	if !ok {
		return nil, false, nil
	}
	count, ok := keys.(int64)
	if !ok {
		return nil, false, nil
	}
	if count < 0 || int64(int(count)) != count {
		return nil, true, fmt.Errorf("keyed table key count must be a non-negative integer")
	}
	names := frame.Schema().Names()
	n := int(count)
	if n > len(names) {
		return nil, true, fmt.Errorf("keyed table key count %d exceeds column count %d", n, len(names))
	}
	if n == 0 {
		return frame, true, nil
	}
	keyed, err := data.KeyBy(frame, names[:n]...)
	if err != nil {
		return nil, true, fmt.Errorf("keyed table: %w", err)
	}
	return keyed, true, nil
}

func keyedTable(keys any, values any) (data.KeyedFrame, bool, error) {
	keyFrame, keyOK := keys.(data.Frame)
	valueFrame, valueOK := values.(data.Frame)
	if !keyOK && !valueOK {
		return data.KeyedFrame{}, false, nil
	}
	if !keyOK || !valueOK {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table expects key and value tables")
	}
	if keyFrame.Len() != valueFrame.Len() {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table key/value row length mismatch")
	}
	keyNames := keyFrame.Schema().Names()
	cols := make([]data.Column, 0, len(keyNames)+len(valueFrame.Schema().Names()))
	cols = append(cols, keyFrame.Columns()...)
	cols = append(cols, valueFrame.Columns()...)
	frame, err := data.NewFrame(cols...)
	if err != nil {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table: %w", err)
	}
	keyed, err := data.KeyBy(frame, keyNames...)
	if err != nil {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table: %w", err)
	}
	return keyed, true, nil
}

func flip(v any) (any, error) {
	if matrix, ok, err := qMatrixValue(v); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return data.TransposeMatrix(matrix)
	}
	if _, ok := v.(data.KeyedFrame); ok {
		return nil, fmt.Errorf("flip expects a plain dictionary, got keyed table")
	}
	d, ok := v.(EvalDict)
	if !ok {
		return nil, fmt.Errorf("flip expects a dictionary")
	}
	keys, err := dictSymbolKeys(d)
	if err != nil {
		return nil, fmt.Errorf("flip expects symbol column names: %w", err)
	}
	values := make(map[data.Symbol]any, len(d.Keys))
	for i, key := range keys {
		values[key] = d.Values[i]
	}
	rows, _, err := flipRowCountFromDict(keys, values)
	if err != nil {
		return nil, err
	}
	cols := make([]data.Column, 0, len(keys))
	for _, name := range keys {
		array, ok := values[name].(data.Array)
		if !ok {
			repeated := make([]any, rows)
			for i := range repeated {
				repeated[i] = values[name]
			}
			array = data.InferArray(repeated)
		}
		cols = append(cols, data.Column{Name: name, Data: array})
	}
	return data.NewFrame(cols...)
}

func reshapeValue(shapeValue, value any) (any, error) {
	shape, err := qReshapeShape(shapeValue)
	if err != nil {
		return nil, err
	}
	var source data.Array
	switch x := value.(type) {
	case data.Array:
		source = x
	case string:
		chars := make([]string, 0, len(x))
		for _, r := range x {
			chars = append(chars, string(r))
		}
		source = data.NewString(chars)
	default:
		source = inferQArray([]any{x}, qKindOfValue(x))
	}
	return data.ReshapeArray(shape, source)
}

func matrixMultiplyValue(left, right any) (any, error) {
	leftMatrix, ok, err := qMatrixValue(left)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("mmu left operand must be a matrix")
	}
	rightMatrix, ok, err := qMatrixValue(right)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("mmu right operand must be a matrix")
	}
	return data.MatrixMultiplyNumeric(leftMatrix, rightMatrix)
}

func matrixInverseValue(value any) (any, error) {
	matrix, ok, err := qMatrixValue(value)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("inv operand must be a matrix")
	}
	return data.MatrixInverseNumeric(matrix)
}

func qReshapeShape(value any) ([]int, error) {
	array, ok := value.(data.Array)
	if !ok {
		n, ok := integerValue(value)
		if !ok || int64(int(n)) != n {
			return nil, fmt.Errorf("# left operand must be an integer count")
		}
		return []int{int(n)}, nil
	}
	if array.Len() == 0 {
		return nil, fmt.Errorf("reshape expects at least one dimension")
	}
	out := make([]int, array.Len())
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("reshape dimension row %d out of range", i)
		}
		n, ok := integerValue(item)
		if !ok || int64(int(n)) != n {
			return nil, fmt.Errorf("reshape dimension %d must be an integer", i)
		}
		if n < 0 {
			return nil, fmt.Errorf("reshape dimension %d must be non-negative", i)
		}
		out[i] = int(n)
	}
	return out, nil
}

func qMatrixValue(value any) (data.Matrix, bool, error) {
	if matrix, ok := value.(data.Matrix); ok {
		return matrix, true, nil
	}
	outer, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	return data.MatrixFromRows(outer)
}

func flipRowCount(v any) (rows int, cols int, err error) {
	if _, ok := v.(data.KeyedFrame); ok {
		return 0, 0, fmt.Errorf("flip expects a plain dictionary, got keyed table")
	}
	d, ok := v.(EvalDict)
	if !ok {
		return 0, 0, fmt.Errorf("flip expects a dictionary")
	}
	keys, err := dictSymbolKeys(d)
	if err != nil {
		return 0, 0, fmt.Errorf("flip expects symbol column names: %w", err)
	}
	values := make(map[data.Symbol]any, len(d.Keys))
	for i, key := range keys {
		values[key] = d.Values[i]
	}
	rows, _, err = flipRowCountFromDict(keys, values)
	return rows, len(keys), err
}

func flipColumnNamesAndRows(v any) ([]data.Symbol, int, error) {
	if _, ok := v.(data.KeyedFrame); ok {
		return nil, 0, fmt.Errorf("flip expects a plain dictionary, got keyed table")
	}
	d, ok := v.(EvalDict)
	if !ok {
		return nil, 0, fmt.Errorf("flip expects a dictionary")
	}
	keys, err := dictSymbolKeys(d)
	if err != nil {
		return nil, 0, fmt.Errorf("flip expects symbol column names: %w", err)
	}
	values := make(map[data.Symbol]any, len(d.Keys))
	for i, key := range keys {
		values[key] = d.Values[i]
	}
	rows, _, err := flipRowCountFromDict(keys, values)
	if err != nil {
		return nil, 0, err
	}
	return keys, rows, nil
}

func flipRowCountFromDict(keys []data.Symbol, values map[data.Symbol]any) (int, int, error) {
	rows := 1
	for _, name := range keys {
		if array, ok := values[name].(data.Array); ok && array.Len() > rows {
			rows = array.Len()
		}
	}
	for _, name := range keys {
		if array, ok := values[name].(data.Array); ok && array.Len() != rows && array.Len() != 1 {
			return 0, 0, fmt.Errorf("flip column %q length %d cannot conform to row count %d", name, array.Len(), rows)
		}
	}
	return rows, len(keys), nil
}

func qValidateFrameColumns(existing []data.Symbol, requested []data.Symbol, op string) error {
	seen := make(map[data.Symbol]struct{}, len(existing))
	for _, name := range existing {
		seen[name] = struct{}{}
	}
	for _, name := range requested {
		if name == "" {
			return fmt.Errorf("%s column name must not be empty", op)
		}
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("%s column %q does not exist", op, name)
		}
	}
	return nil
}

func applyDyadic(op byte, left, right any) (any, error) {
	if op == '~' {
		return matchValue(left, right), nil
	}
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if lok || rok {
		return applyVectorDyadic(op, left, right, la, ra)
	}
	switch op {
	case 'L':
		return left, nil
	case 'R':
		return right, nil
	case '&', '|':
		return applyDyadicLogical(op, left, right)
	}
	if data.IsNull(left) || data.IsNull(right) {
		switch op {
		case 'm':
			return minDyadic(left, right)
		case 'M':
			return maxDyadic(left, right)
		case '+', '-', '*', '%', 'r', 'd':
			if !hasTypedNullKind(left) && !hasTypedNullKind(right) {
				return data.NullValue, nil
			}
			return data.NullForKind(qDyadicResultKind(op, left, right)), nil
		case '^':
			if data.IsNull(right) {
				if data.IsNull(left) {
					return data.NullValue, nil
				}
				return left, nil
			}
			return right, nil
		case '=':
			return data.IsNull(left) && data.IsNull(right), nil
		case '<', '>':
			cmp, err := compareSortable(left, right)
			if err != nil {
				return nil, err
			}
			if op == '<' {
				return cmp < 0, nil
			}
			return cmp > 0, nil
		}
	}
	if op == 'm' {
		return minDyadic(left, right)
	}
	if op == 'M' {
		return maxDyadic(left, right)
	}
	if op == '^' {
		return right, nil
	}
	ln, lok := numeric(left)
	rn, rok := numeric(right)
	if !lok || !rok {
		switch op {
		case '=':
			return equalComparableValue(left, right), nil
		case '<', '>':
			cmp, err := compareOrdered(left, right)
			if err != nil {
				return nil, err
			}
			if op == '<' {
				return cmp < 0, nil
			}
			return cmp > 0, nil
		default:
			return nil, fmt.Errorf("operator %q expects numeric operands", string(op))
		}
	}
	if li, ok := integerValue(left); ok && op != '%' {
		if ri, ok := integerValue(right); ok {
			switch op {
			case '+':
				return li + ri, nil
			case '-':
				return li - ri, nil
			case '*':
				return li * ri, nil
			case 'r':
				if ri == 0 {
					return data.NullValue, nil
				}
				return li - ri*int64(math.Floor(float64(li)/float64(ri))), nil
			case 'd':
				return floorDivInt(li, ri)
			case '=':
				return li == ri, nil
			case '<':
				return li < ri, nil
			case '>':
				return li > ri, nil
			}
		}
	}
	switch op {
	case 'm':
		return minDyadic(left, right)
	case 'M':
		return maxDyadic(left, right)
	case '^':
		return right, nil
	case '+':
		return ln + rn, nil
	case '-':
		return ln - rn, nil
	case '*':
		return ln * rn, nil
	case '%':
		return ln / rn, nil
	case 'r':
		if rn == 0 {
			return data.NullValue, nil
		}
		return ln - rn*math.Floor(ln/rn), nil
	case 'd':
		if rn == 0 {
			return data.NullValue, nil
		}
		return int64(math.Floor(ln / rn)), nil
	case '=':
		return ln == rn, nil
	case '<':
		return ln < rn, nil
	case '>':
		return ln > rn, nil
	default:
		return nil, fmt.Errorf("operator %q is not supported", string(op))
	}
}

func hasTypedNullKind(v any) bool {
	kind, ok := data.NullKind(v)
	return ok && kind != data.KindNull
}

func applyDyadicLogical(op byte, left, right any) (any, error) {
	lv, err := boolValue(left)
	if err != nil {
		return nil, err
	}
	rv, err := boolValue(right)
	if err != nil {
		return nil, err
	}
	switch op {
	case '&':
		return lv && rv, nil
	case '|':
		return lv || rv, nil
	default:
		return nil, fmt.Errorf("operator %q is not a logical verb", string(op))
	}
}

func dyadicVerbFunc(op byte) func(any, any) (any, error) {
	return func(left, right any) (any, error) {
		return applyDyadic(op, left, right)
	}
}

func modValue(left, right any) (any, error) {
	return applyDyadic('r', left, right)
}

func floorDivInt(left, right int64) (any, error) {
	if right == 0 {
		return data.NullValue, nil
	}
	if left == math.MinInt64 && right == -1 {
		return data.NullValue, nil
	}
	q := left / right
	r := left % right
	if r != 0 && ((r > 0) != (right > 0)) {
		q--
	}
	return q, nil
}

func minDyadic(left, right any) (any, error) {
	if data.IsNull(left) {
		return right, nil
	}
	if data.IsNull(right) {
		return left, nil
	}
	cmp, err := compareOrdered(left, right)
	if err != nil {
		return nil, err
	}
	if cmp <= 0 {
		return left, nil
	}
	return right, nil
}

func maxDyadic(left, right any) (any, error) {
	if data.IsNull(left) {
		return right, nil
	}
	if data.IsNull(right) {
		return left, nil
	}
	cmp, err := compareOrdered(left, right)
	if err != nil {
		return nil, err
	}
	if cmp >= 0 {
		return left, nil
	}
	return right, nil
}

func applyCompositeDyadic(op string, left, right any) (any, error) {
	if dataOp, ok := qDataCompositeComparisonOp(op); ok {
		la, lok := left.(data.Array)
		ra, rok := right.(data.Array)
		if lok || rok {
			shape := qRuntimeKernelCompositeVectorDyadicShape(op, left, right, la, ra)
			if !qVectorDyadicCanUseTypedCompare(left, right, la, ra) {
				recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "attempt", "attempt")
				recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "fallback", RuntimeFallbackUnsupportedType)
			} else if out, handled, err := qTryTypedCompareMask(dataOp, left, right, la, ra); err != nil || handled {
				out, handled, err = qTypedRuntimeResult("ArrayDyadicCompare", shape, out, handled, err)
				if err != nil {
					return nil, err
				}
				return out, nil
			} else {
				_, _, _ = qTypedRuntimeResult("ArrayDyadicCompare", shape, out, handled, err)
			}
		} else if out, err := data.ApplyBinary(dataOp, left, right); err == nil {
			return out, nil
		}
	}
	var base any
	var err error
	switch op {
	case "<>":
		base, err = applyDyadic('=', left, right)
	case "<=":
		base, err = applyDyadic('>', left, right)
	case ">=":
		base, err = applyDyadic('<', left, right)
	default:
		return nil, fmt.Errorf("operator %q is not supported", op)
	}
	if err != nil {
		return nil, err
	}
	return invertBoolResult(base)
}

func invertBoolResult(v any) (any, error) {
	switch x := v.(type) {
	case bool:
		return !x, nil
	case data.Array:
		if x.Kind() != data.KindBool {
			return nil, fmt.Errorf("composite comparison produced non-bool vector")
		}
		values := x.Values()
		out := make([]bool, len(values))
		for i, value := range values {
			b, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("composite comparison produced non-bool vector")
			}
			out[i] = !b
		}
		return data.NewBool(out), nil
	default:
		return nil, fmt.Errorf("composite comparison produced %T", v)
	}
}

func qDataCompositeComparisonOp(op string) (data.Op, bool) {
	switch op {
	case "<>":
		return data.OpNE, true
	case "<=":
		return data.OpLE, true
	case ">=":
		return data.OpGE, true
	default:
		return "", false
	}
}

func applyVectorDyadic(op byte, left, right any, la, ra data.Array) (data.Array, error) {
	n := 0
	switch {
	case la != nil && ra != nil:
		switch {
		case la.Len() == ra.Len():
			n = la.Len()
		case la.Len() == 1:
			n = ra.Len()
		case ra.Len() == 1:
			n = la.Len()
		default:
			return nil, fmt.Errorf("vector length mismatch")
		}
	case la != nil:
		n = la.Len()
	case ra != nil:
		n = ra.Len()
	}
	if op == '^' {
		if la == nil && ra != nil {
			out, handled, err := data.TryTypedScalarFill(left, ra)
			shape := "scalar-fill/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(ra.Kind())
			out, handled, err = qTypedRuntimeResult("ArrayScalarFill", shape, out, handled, err)
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
		}
		if la != nil && ra == nil {
			out, handled, err := data.TryTypedScalarFill(right, la)
			shape := "scalar-fill/" + string(qRuntimeKernelOperandKind(right, nil)) + "/" + string(la.Kind())
			out, handled, err = qTypedRuntimeResult("ArrayScalarFill", shape, out, handled, err)
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
		}
	}
	if dataOp, ok := qDataArithmeticOp(op); ok {
		shape := qRuntimeKernelVectorDyadicShape(op, left, right, la, ra)
		typedLeft, typedRight, canUse, err := qVectorDyadicTypedOperands(left, right, la, ra)
		if err != nil {
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "error", "runtime_error")
			return nil, err
		}
		if !canUse || !qVectorDyadicCanUseTypedArithmetic(typedLeft, typedRight) {
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "attempt", "attempt")
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "fallback", RuntimeFallbackUnsupportedType)
		} else if out, handled, err := qTryTypedArithmeticDyadic(dataOp, typedLeft, typedRight); err != nil || handled {
			out, handled, err = qTypedRuntimeResult("ArrayDyadicArithmetic", shape, out, handled, err)
			if err != nil {
				return nil, err
			}
			if array, ok := out.(data.Array); ok {
				return array, nil
			}
		} else {
			_, _, _ = qTypedRuntimeResult("ArrayDyadicArithmetic", shape, out, handled, err)
		}
	}
	if dataOp, ok := qDataComparisonOp(op); ok {
		shape := qRuntimeKernelVectorDyadicShape(op, left, right, la, ra)
		if !qVectorDyadicCanUseTypedCompare(left, right, la, ra) {
			recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "attempt", "attempt")
			recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "fallback", RuntimeFallbackUnsupportedType)
		} else if out, handled, err := qTryTypedCompareMask(dataOp, left, right, la, ra); err != nil || handled {
			out, handled, err = qTypedRuntimeResult("ArrayDyadicCompare", shape, out, handled, err)
			if err != nil {
				return nil, err
			}
			if array, ok := out.(data.Array); ok {
				return array, nil
			}
		} else {
			_, _, _ = qTypedRuntimeResult("ArrayDyadicCompare", shape, out, handled, err)
		}
	}
	out := make([]any, n)
	hasFloat := op == '%'
	hasNull := false
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if la != nil {
			var ok bool
			row := i
			if la.Len() == 1 {
				row = 0
			}
			lv, ok = la.At(row)
			if !ok {
				return nil, fmt.Errorf("left vector row %d out of range", i)
			}
		}
		if ra != nil {
			var ok bool
			row := i
			if ra.Len() == 1 {
				row = 0
			}
			rv, ok = ra.At(row)
			if !ok {
				return nil, fmt.Errorf("right vector row %d out of range", i)
			}
		}
		v, err := applyDyadic(op, lv, rv)
		if err != nil {
			return nil, err
		}
		if data.IsNull(v) {
			hasNull = true
			out[i] = data.NullValue
			continue
		}
		if b, ok := v.(bool); ok {
			out[i] = b
			continue
		}
		if _, ok := v.(float64); ok {
			hasFloat = true
		}
		out[i] = v
	}
	if n == 0 {
		return data.InferArray(out), nil
	}
	if _, ok := out[0].(bool); ok {
		xs := make([]bool, n)
		for i, v := range out {
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("operator %q produced mixed result types", string(op))
			}
			xs[i] = b
		}
		return data.NewBool(xs), nil
	}
	if hasNull {
		return inferQArray(out, qDyadicResultKind(op, left, right)), nil
	}
	if hasFloat {
		xs := make([]float64, n)
		for i, v := range out {
			f, ok := numeric(v)
			if !ok {
				return nil, fmt.Errorf("operator %q expects numeric operands", string(op))
			}
			xs[i] = f
		}
		return data.NewF64(xs), nil
	}
	xs := make([]int64, n)
	for i, v := range out {
		x, ok := integerValue(v)
		if !ok {
			return data.InferArray(out), nil
		}
		xs[i] = x
	}
	return data.NewI64(xs), nil
}

func qDataArithmeticOp(op byte) (data.Op, bool) {
	switch op {
	case '+':
		return data.OpAdd, true
	case '-':
		return data.OpSub, true
	case '*':
		return data.OpMul, true
	case '%':
		return data.OpDiv, true
	case 'r':
		return data.OpMod, true
	default:
		return "", false
	}
}

func qTryTypedArithmeticDyadic(op data.Op, left, right any) (any, bool, error) {
	if op == data.OpMod && (!qTypedIntegerOperandOK(left) || !qTypedIntegerOperandOK(right)) {
		return nil, false, nil
	}
	if op != data.OpDiv && qTypedIntegerOperandOK(left) && qTypedIntegerOperandOK(right) {
		return data.TryTypedIntegerDyadic(op, left, right)
	}
	return data.TryTypedDyadic(op, left, right)
}

func qTryTypedCompareMask(op data.Op, left, right any, la, ra data.Array) (data.Array, bool, error) {
	if out, handled, err := data.TryTypedDyadic(op, left, right); err != nil || handled {
		if err != nil {
			return nil, handled, err
		}
		array, ok := out.(data.Array)
		return array, ok, nil
	}
	switch {
	case la != nil && ra == nil:
		out, err := data.CompareMask(la, op, right)
		return out, err == nil, err
	case la == nil && ra != nil:
		reversed, ok := reverseDataCompareOp(op)
		if !ok {
			return nil, false, nil
		}
		out, err := data.CompareMask(ra, reversed, left)
		return out, err == nil, err
	default:
		return nil, false, nil
	}
}

func reverseDataCompareOp(op data.Op) (data.Op, bool) {
	switch op {
	case data.OpLT:
		return data.OpGT, true
	case data.OpLE:
		return data.OpGE, true
	case data.OpGT:
		return data.OpLT, true
	case data.OpGE:
		return data.OpLE, true
	case data.OpEQ, data.OpNE:
		return op, true
	default:
		return "", false
	}
}

func qDataComparisonOp(op byte) (data.Op, bool) {
	switch op {
	case '=':
		return data.OpEQ, true
	case '<':
		return data.OpLT, true
	case '>':
		return data.OpGT, true
	default:
		return "", false
	}
}

func qRuntimeKernelVectorDyadicShape(op byte, left, right any, la, ra data.Array) string {
	leftKind := qRuntimeKernelOperandKind(left, la)
	rightKind := qRuntimeKernelOperandKind(right, ra)
	if leftKind == data.KindI64 && rightKind == data.KindI64 {
		switch op {
		case '+':
			return "vector-dyadic/+/i64/i64"
		case '-':
			return "vector-dyadic/-/i64/i64"
		case '*':
			return "vector-dyadic/*/i64/i64"
		case '%':
			return "vector-dyadic/%/i64/i64"
		case '<':
			return "vector-dyadic/</i64/i64"
		case '>':
			return "vector-dyadic/>/i64/i64"
		case '=':
			return "vector-dyadic/=/i64/i64"
		}
	}
	key := qRuntimeShapeKey{
		family: "vector-dyadic",
		op:     string(op),
		left:   leftKind,
		right:  rightKind,
	}
	if value, ok := qRuntimeShapeCache.Load(key); ok {
		return value.(string)
	}
	shape := "vector-dyadic/" + string(op) + "/" + string(leftKind) + "/" + string(rightKind)
	value, _ := qRuntimeShapeCache.LoadOrStore(key, shape)
	return value.(string)
}

func qRuntimeKernelCompositeVectorDyadicShape(op string, left, right any, la, ra data.Array) string {
	leftKind := qRuntimeKernelOperandKind(left, la)
	rightKind := qRuntimeKernelOperandKind(right, ra)
	if leftKind == data.KindI64 && rightKind == data.KindI64 {
		switch op {
		case "<":
			return "vector-dyadic/</i64/i64"
		case "<=":
			return "vector-dyadic/<=/i64/i64"
		case ">":
			return "vector-dyadic/>/i64/i64"
		case ">=":
			return "vector-dyadic/>=/i64/i64"
		case "=":
			return "vector-dyadic/=/i64/i64"
		case "<>":
			return "vector-dyadic/<>/i64/i64"
		}
	}
	key := qRuntimeShapeKey{
		family: "vector-dyadic",
		op:     op,
		left:   leftKind,
		right:  rightKind,
	}
	if value, ok := qRuntimeShapeCache.Load(key); ok {
		return value.(string)
	}
	shape := "vector-dyadic/" + op + "/" + string(leftKind) + "/" + string(rightKind)
	value, _ := qRuntimeShapeCache.LoadOrStore(key, shape)
	return value.(string)
}

func qVectorDyadicTypedOperands(left, right any, la, ra data.Array) (any, any, bool, error) {
	typedLeft := left
	typedRight := right
	if la != nil && ra != nil {
		switch {
		case la.Len() == ra.Len():
		case la.Len() == 1:
			value, ok := la.At(0)
			if !ok {
				return nil, nil, false, fmt.Errorf("left vector row 0 out of range")
			}
			typedLeft = value
		case ra.Len() == 1:
			value, ok := ra.At(0)
			if !ok {
				return nil, nil, false, fmt.Errorf("right vector row 0 out of range")
			}
			typedRight = value
		default:
			return nil, nil, false, nil
		}
	}
	return typedLeft, typedRight, true, nil
}

func qRuntimeKernelOperandKind(value any, array data.Array) data.Kind {
	if array != nil {
		return array.Kind()
	}
	kind := qKindOfValue(value)
	if kind == "" {
		return data.KindAny
	}
	return kind
}

func qVectorDyadicCanUseTypedArithmetic(left, right any) bool {
	return qTypedArithmeticOperandOK(left) && qTypedArithmeticOperandOK(right)
}

func qTypedArithmeticOperandOK(value any) bool {
	if array, ok := value.(data.Array); ok {
		return qKindIsNumeric(array.Kind())
	}
	if data.IsNull(value) {
		return true
	}
	return qKindIsNumeric(qKindOfValue(value))
}

func qTypedIntegerOperandOK(value any) bool {
	if array, ok := value.(data.Array); ok {
		return qKindIsInteger(array.Kind())
	}
	if kind, ok := data.NullKind(value); ok {
		return qKindIsInteger(kind)
	}
	return qKindIsInteger(qKindOfValue(value))
}

func qVectorDyadicCanUseTypedCompare(left, right any, la, ra data.Array) bool {
	if la != nil && ra != nil && la.Len() != ra.Len() {
		return false
	}
	return qTypedCompareOperandOK(left, la) && qTypedCompareOperandOK(right, ra)
}

func qTypedCompareOperandOK(value any, array data.Array) bool {
	if array != nil {
		return qTypedCompareKindOK(array.Kind())
	}
	return qTypedCompareKindOK(qKindOfValue(value))
}

func qTypedCompareKindOK(kind data.Kind) bool {
	switch kind {
	case data.KindBool,
		data.KindI8, data.KindI16, data.KindI32, data.KindI64,
		data.KindU8, data.KindU16, data.KindU32, data.KindU64,
		data.KindF32, data.KindF64,
		data.KindMonth, data.KindDate, data.KindDateTime,
		data.KindTimespan, data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		return true
	default:
		return false
	}
}

func vectorIndex(i, length int) int {
	if length == 1 {
		return 0
	}
	return i
}

func inferQArray(values []any, hints ...data.Kind) data.Array {
	inferred := data.InferArray(values)
	if len(values) == 0 {
		if typed, ok := emptyQArrayFromHints(hints...); ok {
			return typed
		}
		return inferred
	}
	if inferred.Kind() != data.KindNull {
		return inferred
	}
	kind := data.Kind("")
	for _, value := range values {
		valueKind := qKindOfValue(value)
		if valueKind == "" || valueKind == data.KindNull || valueKind == data.KindAny {
			continue
		}
		merged, ok := mergeQResultKinds(kind, valueKind)
		if !ok {
			return inferred
		}
		kind = merged
	}
	for _, hint := range hints {
		if hint == "" || hint == data.KindNull || hint == data.KindAny {
			continue
		}
		merged, ok := mergeQResultKinds(kind, hint)
		if !ok {
			return inferred
		}
		kind = merged
	}
	if kind == "" {
		return inferred
	}
	column, err := data.NewColumnWithKind("_", kind, values)
	if err != nil {
		return inferred
	}
	return column.Data
}

func emptyQArrayFromHints(hints ...data.Kind) (data.Array, bool) {
	kind := data.Kind("")
	for _, hint := range hints {
		if hint == "" || hint == data.KindNull || hint == data.KindAny {
			continue
		}
		merged, ok := mergeQResultKinds(kind, hint)
		if !ok {
			return nil, false
		}
		kind = merged
	}
	if kind == "" {
		return nil, false
	}
	column, err := data.NewColumnWithKind("_", kind, nil)
	if err != nil {
		return nil, false
	}
	return column.Data, true
}

func qDyadicResultKind(op byte, left, right any) data.Kind {
	switch op {
	case '=', '<', '>', '&', '|', '~':
		return data.KindBool
	case '%':
		return data.KindF64
	case '+', '-', '*', 'r', 'd':
		leftKind := qKindOfValue(left)
		rightKind := qKindOfValue(right)
		if qKindIsNumeric(leftKind) || qKindIsNumeric(rightKind) {
			kind, ok := mergeQResultKinds(leftKind, rightKind)
			if ok {
				return kind
			}
		}
	case 'm', 'M', '^':
		kind, ok := mergeQResultKinds(qKindOfValue(left), qKindOfValue(right))
		if ok {
			return kind
		}
	case 'L':
		return qKindOfValue(left)
	case 'R':
		return qKindOfValue(right)
	}
	return ""
}

func qKindOfValue(v any) data.Kind {
	if kind, ok := data.NullKind(v); ok {
		return kind
	}
	switch x := v.(type) {
	case data.Array:
		return x.Kind()
	case bool:
		return data.KindBool
	case int8:
		return data.KindI8
	case int16:
		return data.KindI16
	case int32:
		return data.KindI32
	case int, int64:
		return data.KindI64
	case uint8:
		return data.KindU8
	case uint16:
		return data.KindU16
	case uint32:
		return data.KindU32
	case uint64:
		return data.KindU64
	case float32:
		return data.KindF32
	case float64:
		return data.KindF64
	case string:
		return data.KindString
	case data.Symbol:
		return data.KindSymbol
	case data.Month:
		return data.KindMonth
	case data.Date:
		return data.KindDate
	case data.DateTime:
		return data.KindDateTime
	case data.Timespan:
		return data.KindTimespan
	case data.Minute:
		return data.KindMinute
	case data.Second:
		return data.KindSecond
	case data.Time:
		return data.KindTime
	case data.Timestamp:
		return data.KindTimestamp
	default:
		return ""
	}
}

func mergeQResultKinds(left, right data.Kind) (data.Kind, bool) {
	if left == "" || left == data.KindNull || left == data.KindAny {
		return right, true
	}
	if right == "" || right == data.KindNull || right == data.KindAny {
		return left, true
	}
	if left == right {
		return left, true
	}
	if qKindIsNumeric(left) && qKindIsNumeric(right) {
		if left == data.KindF64 || right == data.KindF64 || left == data.KindF32 || right == data.KindF32 {
			return data.KindF64, true
		}
		return data.KindI64, true
	}
	if merged, ok := mergeTemporalKinds(left, right); ok {
		return merged, true
	}
	return "", false
}

func qKindIsNumeric(kind data.Kind) bool {
	switch kind {
	case data.KindI8, data.KindI16, data.KindI32, data.KindI64, data.KindF32, data.KindF64:
		return true
	default:
		return false
	}
}

func qKindIsInteger(kind data.Kind) bool {
	switch kind {
	case data.KindI8, data.KindI16, data.KindI32, data.KindI64:
		return true
	default:
		return false
	}
}

func count(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return int64(0), nil
	case qEnumVector:
		return int64(x.Len()), nil
	case data.Array:
		return int64(x.Len()), nil
	case data.Frame:
		qRecordFrameMetadataPrimitive("count", x.Len(), len(data.FrameColumnNames(x)), nil)
		return int64(x.Len()), nil
	case data.KeyedFrame:
		qRecordFrameMetadataPrimitive("count", data.KeyedFrameLen(x), len(data.KeyedFrameColumnNames(x)), nil)
		return int64(data.KeyedFrameLen(x)), nil
	case EvalDict:
		return int64(len(x.Keys)), nil
	case string:
		return int64(len([]rune(x))), nil
	default:
		return int64(1), nil
	}
}

func enlist(v any) (any, error) {
	return data.NewAny([]any{v}), nil
}

func typeOf(v any) (any, error) {
	if kind, ok := data.NullKind(v); ok && kind != data.KindNull {
		return int64(qTypeCode(kind, true)), nil
	}
	if data.IsNull(v) {
		return int64(101), nil
	}
	switch x := v.(type) {
	case bool:
		return int64(-1), nil
	case int16:
		return int64(-5), nil
	case int32:
		return int64(-6), nil
	case int64:
		return int64(-7), nil
	case uint8:
		return int64(-4), nil
	case float32:
		return int64(-8), nil
	case float64:
		return int64(-9), nil
	case string:
		return int64(10), nil
	case data.Symbol:
		return int64(-11), nil
	case data.Month:
		return int64(-13), nil
	case data.Date:
		return int64(-14), nil
	case data.DateTime:
		return int64(-15), nil
	case data.Timespan:
		return int64(-16), nil
	case data.Minute:
		return int64(-17), nil
	case data.Second:
		return int64(-18), nil
	case data.Time:
		return int64(-19), nil
	case data.Timestamp:
		return int64(-12), nil
	case qEnumVector:
		return int64(qTypeCode(x.Kind(), false)), nil
	case data.Array:
		return int64(qTypeCode(x.Kind(), false)), nil
	case EvalDict:
		return int64(99), nil
	case data.Frame:
		return int64(98), nil
	default:
		return int64(0), nil
	}
}

func qTypeCode(kind data.Kind, atom bool) int {
	code := 0
	switch kind {
	case data.KindBool:
		code = 1
	case data.KindI8, data.KindU8:
		code = 4
	case data.KindI16:
		code = 5
	case data.KindI32:
		code = 6
	case data.KindI64:
		code = 7
	case data.KindF32:
		code = 8
	case data.KindF64:
		code = 9
	case data.KindString:
		code = 10
	case data.KindSymbol:
		code = 11
	case data.KindMonth:
		code = 13
	case data.KindTimestamp:
		code = 12
	case data.KindDate:
		code = 14
	case data.KindDateTime:
		code = 15
	case data.KindTimespan:
		code = 16
	case data.KindMinute:
		code = 17
	case data.KindSecond:
		code = 18
	case data.KindTime:
		code = 19
	case data.KindNull:
		code = 101
	default:
		code = 0
	}
	if atom && code > 0 {
		return -code
	}
	return code
}

func stringValue(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		if typed, handled, err := data.TryTypedStringCast(array); err != nil || handled {
			recordRuntimeKernelProbe("ArrayStringCast", "string-cast/"+string(array.Kind()), handled, err)
			return typed, err
		} else {
			recordRuntimeKernelProbe("ArrayStringCast", "string-cast/"+string(array.Kind()), handled, err)
		}
		out := make([]string, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("string row %d out of range", i)
			}
			out[i] = qStringScalar(item)
		}
		return data.NewString(out), nil
	}
	return qStringScalar(v), nil
}

func qStringScalar(v any) string {
	if data.IsNull(v) {
		return ""
	}
	switch x := v.(type) {
	case data.Symbol:
		return string(x)
	}
	if s, ok := FormatTemporal(v); ok {
		return s
	}
	return fmt.Sprint(v)
}

func lowerValue(v any) (any, error) {
	return mapStringValue("lower", v, strings.ToLower)
}

func upperValue(v any) (any, error) {
	return mapStringValue("upper", v, strings.ToUpper)
}

func mapStringValue(name string, v any, fn func(string) string) (any, error) {
	if array, ok := v.(data.Array); ok {
		if name == "upper" || name == "lower" {
			typed, handled, err := data.TryTypedStringCase(array, name == "upper")
			recordRuntimeKernelProbe("ArrayStringCase", name+"/"+string(array.Kind()), handled, err)
			if err != nil || handled {
				return typed, err
			}
		}
	}
	return data.TransformStringValue(name, v, fn)
}

func sum(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("sum expects a numeric vector")
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	shape := "vector-reduce/sum/" + string(array.Kind())
	out, handled, err := data.TryTypedNumericSum(array)
	out, handled, err = qTypedRuntimeResultReason("ArraySum", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	totalI := int64(0)
	totalF := float64(0)
	hasFloat := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("sum row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		if n, ok := integerValue(item); ok {
			totalI += n
			totalF += float64(n)
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF += float64(n)
		case float64:
			hasFloat = true
			totalF += n
		default:
			return nil, fmt.Errorf("sum expects a numeric vector")
		}
	}
	if hasFloat {
		return totalF, nil
	}
	return totalI, nil
}

func avg(v any) (any, error) {
	if n, ok := numeric(v); ok {
		return n, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("avg expects a numeric vector")
	}
	shape := "vector-reduce/avg/" + string(array.Kind())
	out, handled, err := data.TryTypedNumericAvg(array)
	out, handled, err = qTypedRuntimeResultReason("ArrayAvg", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	total := float64(0)
	count := 0
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("avg row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return nil, fmt.Errorf("avg expects a numeric vector")
		}
		total += n
		count++
	}
	if count == 0 {
		return data.NullValue, nil
	}
	return total / float64(count), nil
}

func varValue(v any) (any, error) {
	total, sumsq, count, err := numericAggregateStats("var", v)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return data.NullValue, nil
	}
	mean := total / float64(count)
	return sumsq/float64(count) - mean*mean, nil
}

func devValue(v any) (any, error) {
	variance, err := varValue(v)
	if err != nil {
		return nil, err
	}
	if data.IsNull(variance) {
		return data.NullValue, nil
	}
	n, ok := numeric(variance)
	if !ok {
		return nil, fmt.Errorf("dev expects a numeric vector")
	}
	return math.Sqrt(n), nil
}

func medValue(v any) (any, error) {
	if n, ok := numeric(v); ok {
		return n, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("med expects a numeric vector")
	}
	values := make([]float64, 0, array.Len())
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("med row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return nil, fmt.Errorf("med expects a numeric vector")
		}
		values = append(values, n)
	}
	if len(values) == 0 {
		return data.NullValue, nil
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid], nil
	}
	return (values[mid-1] + values[mid]) / 2, nil
}

func numericAggregateStats(name string, v any) (float64, float64, int, error) {
	if n, ok := numeric(v); ok {
		return n, n * n, 1, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return 0, 0, 0, fmt.Errorf("%s expects a numeric vector", name)
	}
	total := float64(0)
	sumsq := float64(0)
	count := 0
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return 0, 0, 0, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return 0, 0, 0, fmt.Errorf("%s expects a numeric vector", name)
		}
		total += n
		sumsq += n * n
		count++
	}
	return total, sumsq, count, nil
}

func wavg(weights, values any) (any, error) {
	weightArray, weightIsArray := weights.(data.Array)
	valueArray, valueIsArray := values.(data.Array)
	if !weightIsArray && !valueIsArray {
		w, wok := numeric(weights)
		v, vok := numeric(values)
		if !wok || !vok {
			return nil, fmt.Errorf("wavg expects numeric weights and values")
		}
		if w == 0 {
			return data.NullValue, nil
		}
		return v, nil
	}
	length := 1
	if weightIsArray {
		length = weightArray.Len()
	}
	if valueIsArray {
		if weightIsArray && valueArray.Len() != length {
			return nil, fmt.Errorf("wavg weight length %d does not match value length %d", length, valueArray.Len())
		}
		length = valueArray.Len()
	}
	total := float64(0)
	denom := float64(0)
	for i := 0; i < length; i++ {
		weightItem := weights
		if weightIsArray {
			item, ok := weightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("wavg weight row %d out of range", i)
			}
			weightItem = item
		}
		valueItem := values
		if valueIsArray {
			item, ok := valueArray.At(i)
			if !ok {
				return nil, fmt.Errorf("wavg value row %d out of range", i)
			}
			valueItem = item
		}
		if data.IsNull(weightItem) || data.IsNull(valueItem) {
			continue
		}
		w, wok := numeric(weightItem)
		v, vok := numeric(valueItem)
		if !wok || !vok {
			return nil, fmt.Errorf("wavg expects numeric weights and values")
		}
		total += w * v
		denom += w
	}
	if denom == 0 {
		return data.NullValue, nil
	}
	return total / denom, nil
}

func prd(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("prd expects a numeric vector")
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	shape := "vector-reduce/product/" + string(array.Kind())
	out, handled, err := data.TryTypedNumericProduct(array)
	out, handled, err = qTypedRuntimeResult("ArrayProduct", shape, out, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	totalI := int64(1)
	totalF := float64(1)
	hasFloat := false
	seen := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("prd row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		seen = true
		if n, ok := integerValue(item); ok {
			totalI *= n
			totalF *= float64(n)
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF *= float64(n)
		case float64:
			hasFloat = true
			totalF *= n
		default:
			return nil, fmt.Errorf("prd expects a numeric vector")
		}
	}
	if !seen {
		return data.NullValue, nil
	}
	if hasFloat {
		return totalF, nil
	}
	return totalI, nil
}

func sums(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("sums expects a numeric vector")
	}
	shape := "vector-scan/sum/" + string(array.Kind())
	typedOut, handled, err := data.TryTypedNumericSums(array)
	typedOut, handled, err = qTypedRuntimeResult("ArraySums", shape, typedOut, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, err
		}
		return typedOut, nil
	}
	out := make([]any, array.Len())
	totalI := int64(0)
	totalF := float64(0)
	hasFloat := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("sums row %d out of range", i)
		}
		if data.IsNull(item) {
			if hasFloat {
				out[i] = totalF
			} else {
				out[i] = totalI
			}
			continue
		}
		if n, ok := integerValue(item); ok {
			totalI += n
			totalF += float64(n)
			if hasFloat {
				out[i] = totalF
			} else {
				out[i] = totalI
			}
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF += float64(n)
			out[i] = totalF
		case float64:
			hasFloat = true
			totalF += n
			out[i] = totalF
		default:
			return nil, fmt.Errorf("sums expects a numeric vector")
		}
	}
	if hasFloat {
		xs := make([]float64, len(out))
		for i, v := range out {
			xs[i], _ = numeric(v)
		}
		return data.NewF64(xs), nil
	}
	xs := make([]int64, len(out))
	for i, v := range out {
		xs[i] = v.(int64)
	}
	return data.NewI64(xs), nil
}

func prds(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("prds expects a numeric vector")
	}
	shape := "vector-scan/product/" + string(array.Kind())
	typedOut, handled, err := data.TryTypedNumericProducts(array)
	typedOut, handled, err = qTypedRuntimeResult("ArrayProducts", shape, typedOut, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, err
		}
		return typedOut, nil
	}
	out := make([]any, array.Len())
	totalI := int64(1)
	totalF := float64(1)
	hasFloat := false
	seen := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("prds row %d out of range", i)
		}
		if data.IsNull(item) {
			if seen {
				if hasFloat {
					out[i] = totalF
				} else {
					out[i] = totalI
				}
			} else {
				out[i] = data.NullValue
			}
			continue
		}
		seen = true
		if n, ok := integerValue(item); ok {
			totalI *= n
			totalF *= float64(n)
			if hasFloat {
				out[i] = totalF
			} else {
				out[i] = totalI
			}
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF *= float64(n)
			out[i] = totalF
		case float64:
			hasFloat = true
			totalF *= n
			out[i] = totalF
		default:
			return nil, fmt.Errorf("prds expects a numeric vector")
		}
	}
	if hasFloat {
		xs := make([]float64, len(out))
		for i, v := range out {
			xs[i], _ = numeric(v)
		}
		return data.NewF64(xs), nil
	}
	return data.InferArray(out), nil
}

func mins(v any) (any, error) {
	return runningExtrema("mins", v, false)
}

func maxs(v any) (any, error) {
	return runningExtrema("maxs", v, true)
}

func runningExtrema(name string, v any, maximum bool) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	if data.IsNull(v) {
		return v, nil
	}
	if _, err := compareOrdered(v, v); err == nil {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("%s expects an ordered vector", name)
	}
	if typed, handled, err := data.TryTypedRunningMinMax(array, maximum); handled || err != nil {
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return typed, nil
	}
	out := make([]any, array.Len())
	var best any
	hasBest := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			if hasBest {
				out[i] = best
			} else {
				out[i] = data.NullValue
			}
			continue
		}
		if !hasBest {
			best = item
			hasBest = true
			out[i] = best
			continue
		}
		cmp, err := compareOrdered(item, best)
		if err != nil {
			return nil, fmt.Errorf("%s expects ordered values: %w", name, err)
		}
		if (!maximum && cmp < 0) || (maximum && cmp > 0) {
			best = item
		}
		out[i] = best
	}
	return inferQArray(out, array.Kind()), nil
}

func avgs(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("avgs expects a numeric vector")
	}
	if typed, handled, err := data.TryTypedAvgs(array); handled || err != nil {
		if err != nil {
			return nil, fmt.Errorf("avgs: %w", err)
		}
		return typed, nil
	}
	out := make([]any, array.Len())
	total := float64(0)
	count := 0
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("avgs row %d out of range", i)
		}
		if !data.IsNull(item) {
			n, ok := numeric(item)
			if !ok {
				return nil, fmt.Errorf("avgs expects a numeric vector")
			}
			total += n
			count++
		}
		if count == 0 {
			out[i] = data.NullForKind(data.KindF64)
		} else {
			out[i] = total / float64(count)
		}
	}
	return inferQArray(out, data.KindF64), nil
}

func ratios(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	if data.IsNull(v) {
		return data.NullForKind(data.KindF64), nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("ratios expects a numeric vector")
	}
	out := make([]any, array.Len())
	var previous any
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("ratios row %d out of range", i)
		}
		if data.IsNull(item) {
			out[i] = data.NullValue
			previous = item
			continue
		}
		if _, ok := numeric(item); !ok {
			return nil, fmt.Errorf("ratios expects a numeric vector")
		}
		if i == 0 || data.IsNull(previous) {
			out[i] = item
		} else {
			ratio, err := applyDyadic('%', item, previous)
			if err != nil {
				return nil, err
			}
			out[i] = ratio
		}
		previous = item
	}
	xs := make([]float64, len(out))
	for i, v := range out {
		if data.IsNull(v) {
			return data.InferArray(out), nil
		}
		n, ok := numeric(v)
		if !ok {
			return nil, fmt.Errorf("ratios expects a numeric vector")
		}
		xs[i] = n
	}
	return data.NewF64(xs), nil
}

func negValue(v any) (any, error) {
	if h, ok := v.(*qIPCHandle); ok {
		if h == nil || h.session == nil {
			return nil, fmt.Errorf("q IPC handle is closed")
		}
		return &qIPCHandle{
			target:  h.target,
			async:   true,
			session: h.session,
		}, nil
	}
	return mapNumericUnary("neg", v, func(n float64, isInt bool) any {
		if isInt {
			return int64(-n)
		}
		return -n
	})
}

func absValue(v any) (any, error) {
	return mapNumericUnary("abs", v, func(n float64, isInt bool) any {
		if n < 0 {
			n = -n
		}
		if isInt {
			return int64(n)
		}
		return n
	})
}

func sqrtValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnarySqrt, v)
}

func logValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryLog, v)
}

func expValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryExp, v)
}

func reciprocalValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryRecip, v)
}

func signumValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnarySignum, v)
}

func floorValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryFloor, v)
}

func ceilingValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryCeiling, v)
}

func mapNumericUnary(name string, v any, fn func(float64, bool) any) (any, error) {
	if data.IsNull(v) {
		return data.NullValue, nil
	}
	if n, ok := numeric(v); ok {
		_, isInt := integerValue(v)
		return fn(n, isInt), nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("%s expects a numeric value or vector", name)
	}
	shape := "vector-unary/" + name + "/" + string(array.Kind())
	typed, handled, err := data.TryTypedQNumericUnary(name, array)
	typed, handled, err = qTypedRuntimeResult("ArrayNumericUnary", shape, typed, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return typed, nil
	}
	out := make([]any, array.Len())
	hasFloat := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			out[i] = data.NullValue
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return nil, fmt.Errorf("%s expects a numeric value or vector", name)
		}
		_, isInt := integerValue(item)
		value := fn(n, isInt)
		if _, ok := value.(float64); ok {
			hasFloat = true
		}
		out[i] = value
	}
	if hasFloat {
		for _, value := range out {
			if data.IsNull(value) {
				return data.InferArray(out), nil
			}
		}
		xs := make([]float64, len(out))
		for i, value := range out {
			xs[i], _ = numeric(value)
		}
		return data.NewF64(xs), nil
	}
	return data.InferArray(out), nil
}

func allValue(v any) (any, error) {
	return boolAggregate("all", v, true, func(acc, item bool) bool { return acc && item })
}

func anyValue(v any) (any, error) {
	return boolAggregate("any", v, false, func(acc, item bool) bool { return acc || item })
}

func boolAggregate(name string, v any, initial bool, fn func(bool, bool) bool) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		b, err := boolValue(v)
		if err != nil {
			return nil, fmt.Errorf("%s expects bool or numeric values", name)
		}
		return b, nil
	}
	acc := initial
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			continue
		}
		b, err := boolValue(item)
		if err != nil {
			return nil, fmt.Errorf("%s expects bool or numeric values", name)
		}
		acc = fn(acc, b)
	}
	return acc, nil
}

func first(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	item, ok := array.At(0)
	if !ok {
		return nil, fmt.Errorf("first row 0 out of range")
	}
	return item, nil
}

func last(v any) (any, error) {
	if scan, ok := v.(qScanView); ok {
		out, err := scan.terminal()
		recordRuntimeKernelProbe("ArrayLastScanView", "vector-last/"+scan.name+"/"+string(scan.source.Kind()), err == nil, err)
		return out, err
	}
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	item, ok := array.At(array.Len() - 1)
	if !ok {
		return nil, fmt.Errorf("last row %d out of range", array.Len()-1)
	}
	return item, nil
}

func keys(v any) (any, error) {
	switch x := v.(type) {
	case qAttributedVector:
		if qAttributeHasIndex(x) {
			return symbolArray([]data.Symbol{"attribute", "value", "index"}), nil
		}
		return symbolArray([]data.Symbol{"attribute", "value"}), nil
	case qEnumVector:
		return symbolArray([]data.Symbol{"domain", "value"}), nil
	case EvalDict:
		return data.InferArray(x.Keys), nil
	case data.KeyedFrame:
		return symbolArray(x.Keys()), nil
	case data.Frame:
		return data.NewSymbols(nil), nil
	default:
		return nil, fmt.Errorf("keys expects a dictionary or keyed table")
	}
}

func value(v any) (any, error) {
	switch x := v.(type) {
	case qAttributedVector:
		return x.vector, nil
	case qEnumVector:
		return x.decodedArray(), nil
	case EvalDict:
		return data.NewAny(x.Values), nil
	case data.KeyedFrame:
		return x.ValueFrame()
	default:
		return v, nil
	}
}

func cols(v any) (any, error) {
	switch x := v.(type) {
	case data.Frame:
		names := data.FrameColumnNames(x)
		qRecordFrameMetadataPrimitive("cols", x.Len(), len(names), nil)
		return symbolArray(names), nil
	case data.KeyedFrame:
		names := data.KeyedFrameColumnNames(x)
		qRecordFrameMetadataPrimitive("cols", data.KeyedFrameLen(x), len(names), nil)
		return symbolArray(names), nil
	case EvalDict:
		keys, err := dictSymbolKeys(x)
		if err != nil {
			return nil, fmt.Errorf("cols expects dictionary symbol keys: %w", err)
		}
		return symbolArray(keys), nil
	default:
		return nil, fmt.Errorf("cols expects a table or dictionary")
	}
}

func xcols(left any, right any) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xcols: %w", err)
	}
	switch x := right.(type) {
	case data.Frame:
		out, err := data.ReorderFrameColumns(x, names...)
		recordRuntimeFramePrimitive("FrameReorder", "frame-reorder/xcols/"+qRuntimeCardinalityShape(x.Len())+"/cols-"+strconv.Itoa(len(data.FrameColumnNames(x))), err)
		return out, err
	case data.KeyedFrame:
		out, err := data.ReorderKeyedFrameColumns(x, names...)
		recordRuntimeFramePrimitive("FrameReorder", "frame-reorder/xcols-keyed/"+qRuntimeCardinalityShape(data.KeyedFrameLen(x))+"/cols-"+strconv.Itoa(len(data.KeyedFrameColumnNames(x))), err)
		return out, err
	case EvalDict:
		return reorderDictColumns(x, names)
	default:
		return nil, fmt.Errorf("xcols expects a table or dictionary")
	}
}

func xgroup(left any, right any) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xgroup: %w", err)
	}
	switch x := right.(type) {
	case data.Frame:
		out, err := data.XGroup(x, names...)
		recordRuntimeFramePrimitive("FrameGroup", "frame-group/xgroup/"+qRuntimeCardinalityShape(x.Len())+"/cols-"+strconv.Itoa(len(data.FrameColumnNames(x))), err)
		return out, err
	case data.KeyedFrame:
		out, err := data.XGroupKeyed(x, names...)
		recordRuntimeFramePrimitive("FrameGroup", "frame-group/xgroup-keyed/"+qRuntimeCardinalityShape(data.KeyedFrameLen(x))+"/cols-"+strconv.Itoa(len(data.KeyedFrameColumnNames(x))), err)
		return out, err
	default:
		return nil, fmt.Errorf("xgroup expects a table")
	}
}

func xkey(left any, right any) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xkey: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("xkey requires at least one key column")
	}
	switch x := right.(type) {
	case data.Frame:
		out, err := data.KeyBy(x, names...)
		recordRuntimeFramePrimitive("FrameKey", "frame-key/xkey/"+qRuntimeCardinalityShape(x.Len())+"/cols-"+strconv.Itoa(len(data.FrameColumnNames(x))), err)
		return out, err
	case data.KeyedFrame:
		out, err := data.KeyByKeyed(x, names...)
		recordRuntimeFramePrimitive("FrameKey", "frame-key/xkey-keyed/"+qRuntimeCardinalityShape(data.KeyedFrameLen(x))+"/cols-"+strconv.Itoa(len(data.KeyedFrameColumnNames(x))), err)
		return out, err
	default:
		return nil, fmt.Errorf("xkey expects a table")
	}
}

func ungroup(v any) (any, error) {
	switch x := v.(type) {
	case data.Frame:
		out, err := data.Ungroup(x)
		recordRuntimeFramePrimitive("FrameUngroup", "frame-ungroup/"+qRuntimeCardinalityShape(x.Len())+"/cols-"+strconv.Itoa(len(data.FrameColumnNames(x))), err)
		return out, err
	case data.KeyedFrame:
		out, err := data.UngroupKeyedFrame(x)
		recordRuntimeFramePrimitive("FrameUngroup", "frame-ungroup/keyed/"+qRuntimeCardinalityShape(data.KeyedFrameLen(x))+"/cols-"+strconv.Itoa(len(data.KeyedFrameColumnNames(x))), err)
		return out, err
	default:
		return nil, fmt.Errorf("ungroup expects a table")
	}
}

func xasc(left any, right any) (any, error) {
	return xsort(left, right, false)
}

func xdesc(left any, right any) (any, error) {
	return xsort(left, right, true)
}

func xsort(left any, right any, descending bool) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xsort: %w", err)
	}
	switch x := right.(type) {
	case data.Frame:
		out, err := data.SortFrameByColumns(x, names, descending)
		op := "xasc"
		if descending {
			op = "xdesc"
		}
		recordRuntimeFramePrimitive("FrameSort", "frame-sort/"+op+"/"+qRuntimeCardinalityShape(x.Len())+"/cols-"+strconv.Itoa(len(data.FrameColumnNames(x))), err)
		return out, err
	case data.KeyedFrame:
		out, err := data.SortKeyedFrameByColumns(x, names, descending)
		op := "xasc-keyed"
		if descending {
			op = "xdesc-keyed"
		}
		recordRuntimeFramePrimitive("FrameSort", "frame-sort/"+op+"/"+qRuntimeCardinalityShape(data.KeyedFrameLen(x))+"/cols-"+strconv.Itoa(len(data.KeyedFrameColumnNames(x))), err)
		return out, err
	default:
		return nil, fmt.Errorf("xsort expects a table")
	}
}

func sortFrameByColumns(frame data.Frame, names []data.Symbol, descending bool) (data.Frame, error) {
	if out, handled, err := data.TrySortFrameByColumns(frame, names, descending); err != nil || handled {
		shape := "frame-gather/sort/" + qRuntimeCardinalityShape(frame.Len()) + "/cols-" + strconv.Itoa(len(frame.Schema().Names()))
		recordRuntimeFramePrimitive("FrameGather", shape, err)
		if err != nil {
			return data.Frame{}, err
		}
		return out, nil
	}
	bound := make([]data.Array, len(names))
	for i, name := range names {
		col, ok := frame.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("sort column %q does not exist", name)
		}
		bound[i] = col
	}
	indexes := make([]int, frame.Len())
	for i := range indexes {
		indexes[i] = i
	}
	var cmpErr error
	sort.SliceStable(indexes, func(i, j int) bool {
		leftRow, rightRow := indexes[i], indexes[j]
		for _, col := range bound {
			left, lok := col.At(leftRow)
			right, rok := col.At(rightRow)
			if !lok || !rok {
				cmpErr = fmt.Errorf("sort row out of range")
				return false
			}
			cmp, err := compareSortable(left, right)
			if err != nil {
				cmpErr = err
				return false
			}
			if cmp == 0 {
				continue
			}
			if descending {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
	if cmpErr != nil {
		return data.Frame{}, cmpErr
	}
	return qGatherFrameRuntime("sort", frame, indexes)
}

func qColumnNameList(v any) ([]data.Symbol, error) {
	switch x := v.(type) {
	case data.Symbol:
		return []data.Symbol{x}, nil
	case string:
		return []data.Symbol{data.Symbol(x)}, nil
	case data.Array:
		values := x.Values()
		out := make([]data.Symbol, len(values))
		for i, value := range values {
			switch item := value.(type) {
			case data.Symbol:
				out[i] = item
			case string:
				out[i] = data.Symbol(item)
			default:
				return nil, fmt.Errorf("column %d is %T, want symbol", i, value)
			}
			if out[i] == "" {
				return nil, fmt.Errorf("column %d is empty", i)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("left operand must be a symbol or symbol vector")
	}
}

func reorderFrameColumns(frame data.Frame, requested []data.Symbol) (data.Frame, error) {
	order, err := xcolsOrder(frame.Schema().Names(), requested)
	if err != nil {
		return data.Frame{}, err
	}
	return data.SelectFrameColumns(frame, order...)
}

func reorderDictColumns(d EvalDict, requested []data.Symbol) (EvalDict, error) {
	keys, err := dictSymbolKeys(d)
	if err != nil {
		return EvalDict{}, fmt.Errorf("dictionary keys must be symbols: %w", err)
	}
	order, err := xcolsOrder(keys, requested)
	if err != nil {
		return EvalDict{}, err
	}
	position := make(map[data.Symbol]int, len(keys))
	for i, key := range keys {
		position[key] = i
	}
	out := EvalDict{
		Keys:   make([]any, 0, len(order)),
		Values: make([]any, 0, len(order)),
	}
	for _, name := range order {
		index := position[name]
		out.Keys = append(out.Keys, name)
		out.Values = append(out.Values, d.Values[index])
	}
	return out, nil
}

func xcolsOrder(existing []data.Symbol, requested []data.Symbol) ([]data.Symbol, error) {
	available := make(map[data.Symbol]struct{}, len(existing))
	for _, name := range existing {
		available[name] = struct{}{}
	}
	seen := make(map[data.Symbol]struct{}, len(existing))
	order := make([]data.Symbol, 0, len(existing))
	for _, name := range requested {
		if name == "" {
			return nil, fmt.Errorf("column name must not be empty")
		}
		if _, ok := available[name]; !ok {
			return nil, fmt.Errorf("column %q does not exist", name)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("column %q is duplicated", name)
		}
		order = append(order, name)
		seen[name] = struct{}{}
	}
	for _, name := range existing {
		if _, ok := seen[name]; ok {
			continue
		}
		order = append(order, name)
	}
	return order, nil
}

func meta(v any) (any, error) {
	if x, ok := v.(qAttributedVector); ok {
		typ, err := typeOf(x.vector)
		if err != nil {
			return nil, err
		}
		indexed := data.Symbol("")
		if qAttributeHasIndex(x) {
			indexed = x.attribute
		}
		return EvalDict{
			Keys:   []any{data.Symbol("attribute"), data.Symbol("type"), data.Symbol("count"), data.Symbol("index")},
			Values: []any{x.attribute, typ, int64(x.Len()), indexed},
		}, nil
	}
	if x, ok := v.(qEnumVector); ok {
		typ, err := typeOf(x.decodedArray())
		if err != nil {
			return nil, err
		}
		return EvalDict{
			Keys:   []any{data.Symbol("domain"), data.Symbol("type"), data.Symbol("count")},
			Values: []any{x.domain, typ, int64(x.Len())},
		}, nil
	}
	var names []data.Symbol
	var kinds []data.Kind
	var columnAttrs []data.Symbol
	rows := 0
	switch x := v.(type) {
	case data.Frame:
		names = data.FrameColumnNames(x)
		kinds = data.FrameColumnKinds(x)
		columnAttrs = data.FrameColumnAttributes(x)
		rows = x.Len()
	case data.KeyedFrame:
		names = data.KeyedFrameColumnNames(x)
		kinds = data.KeyedFrameColumnKinds(x)
		columnAttrs = data.KeyedFrameColumnAttributes(x)
		rows = data.KeyedFrameLen(x)
	default:
		return nil, fmt.Errorf("meta expects a table")
	}
	typeNames := make([]string, len(names))
	attributes := make([]any, len(names))
	for i := range names {
		typeNames[i] = string(kinds[i])
		attributes[i] = data.NullValue
		if i < len(columnAttrs) && columnAttrs[i] != "" {
			attributes[i] = columnAttrs[i]
		}
	}
	out, err := data.NewFrame(
		data.Column{Name: "c", Data: symbolArray(names)},
		data.Column{Name: "t", Data: data.NewString(typeNames)},
		data.Column{Name: "a", Data: data.NewAny(attributes)},
	)
	qRecordFrameMetadataPrimitive("meta", rows, len(names), err)
	return out, err
}

func attrValue(v any) (any, error) {
	if x, ok := v.(qAttributedVector); ok {
		return x.attribute, nil
	}
	if array, ok := v.(data.Array); ok {
		metadata := data.ArrayMetadataOf(array)
		if len(metadata.Attributes) > 0 {
			return metadata.Attributes[0], nil
		}
		return data.Symbol(""), nil
	}
	return data.Symbol(""), nil
}

func enumCodes(v any) (any, error) {
	var codes []int32
	switch x := v.(type) {
	case qEnumVector:
		codes = x.EncodedCodes()
	case data.Array:
		var ok bool
		codes, ok = data.EncodedCodesOf(x)
		if !ok {
			return nil, fmt.Errorf("codes expects an encoded vector")
		}
	default:
		return nil, fmt.Errorf("codes expects an encoded vector")
	}
	out := make([]int64, len(codes))
	for i, code := range codes {
		out[i] = int64(code)
	}
	return data.NewI64(out), nil
}

func enumDomainValues(v any) (any, error) {
	var domain []any
	switch x := v.(type) {
	case qEnumVector:
		domain = x.EncodedDomain()
	case data.Array:
		var ok bool
		domain, ok = data.EncodedDomainOf(x)
		if !ok {
			return nil, fmt.Errorf("domain expects an encoded vector")
		}
	default:
		return nil, fmt.Errorf("domain expects an encoded vector")
	}
	return data.InferArray(domain), nil
}

func group(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		if index, ok := data.ArrayIndexFor(array, data.ArrayAttributeGrouped); ok {
			return qGroupFromArrayIndex(index), nil
		}
		if index, ok := data.ArrayIndexFor(array, data.ArrayAttributeUnique); ok {
			return qGroupFromArrayIndex(index), nil
		}
		if index, err := data.BuildArrayIndex(array, data.ArrayAttributeGrouped); err == nil {
			recordRuntimeKernelProbe("ArrayGroup", "group-index/"+string(array.Kind()), true, nil)
			return qGroupFromArrayIndex(index), nil
		} else {
			recordRuntimeKernelProbe("ArrayGroup", "group-index/"+string(array.Kind()), false, err)
		}
	}
	values, err := setItems(v)
	if err != nil {
		return nil, err
	}
	keys := make([]any, 0)
	indexes := make([][]int64, 0)
	for i, value := range values {
		found := -1
		for j, key := range keys {
			if equalValue(key, value) {
				found = j
				break
			}
		}
		if found < 0 {
			keys = append(keys, value)
			indexes = append(indexes, []int64{int64(i)})
			continue
		}
		indexes[found] = append(indexes[found], int64(i))
	}
	out := make([]any, len(indexes))
	for i, rows := range indexes {
		out[i] = data.NewI64(rows)
	}
	return EvalDict{Keys: keys, Values: out}, nil
}

func qAttributeHasIndex(v qAttributedVector) bool {
	_, ok := data.ArrayIndexFor(v.vector, v.attribute)
	return ok
}

func qGroupFromArrayIndex(index data.ArrayIndex) EvalDict {
	out := make([]any, len(index.Rows))
	for i, rows := range index.Rows {
		ints := make([]int64, len(rows))
		for j, row := range rows {
			ints[j] = int64(row)
		}
		out[i] = data.NewI64(ints)
	}
	return EvalDict{Keys: append([]any(nil), index.Keys...), Values: out}
}

func symbolArray(symbols []data.Symbol) data.Array {
	names := make([]string, len(symbols))
	for i, sym := range symbols {
		names[i] = string(sym)
	}
	return data.NewSymbols(names)
}

func minValue(v any) (any, error) {
	return extrema(v, false)
}

func maxValue(v any) (any, error) {
	return extrema(v, true)
}

func extrema(v any, wantMax bool) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if value, handled, has, err := data.TryTypedMinMax(array, wantMax); err != nil || handled {
		kernel := "ArrayMin"
		if wantMax {
			kernel = "ArrayMax"
		}
		recordRuntimeKernelProbe(kernel, "vector-reduce/extrema/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, err
		}
		if !has {
			return data.NullValue, nil
		}
		return value, nil
	}
	var best any
	hasBest := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("extrema row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		if !hasBest {
			best = item
			hasBest = true
			continue
		}
		cmp, err := compareOrdered(item, best)
		if err != nil {
			return nil, err
		}
		if (wantMax && cmp > 0) || (!wantMax && cmp < 0) {
			best = item
		}
	}
	if !hasBest {
		return data.NullValue, nil
	}
	return best, nil
}

func where(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		if truth, ok := v.(bool); ok {
			if truth {
				return data.NewI64([]int64{0}), nil
			}
			return data.NewI64(nil), nil
		}
		n, ok := integerValue(v)
		if !ok {
			return nil, fmt.Errorf("where expects a bool or integer vector")
		}
		if n < 0 {
			return nil, fmt.Errorf("where expects non-negative integer counts")
		}
		return data.NewI64(make([]int64, n)), nil
	}
	if array.Kind() == data.KindBool {
		typedOut, handled, err := data.TryTypedWhereMaskI64(array)
		recordRuntimeKernelProbe("ArrayWhere", "mask-to-index/i64", handled, err)
		if err != nil {
			return nil, err
		}
		if handled {
			return typedOut, nil
		}
		indexes, err := data.WhereMask(array)
		if err != nil {
			return nil, err
		}
		out := make([]int64, len(indexes))
		for i, index := range indexes {
			out[i] = int64(index)
		}
		return data.NewI64(out), nil
	}
	var out []int64
	for i, value := range array.Values() {
		if data.IsNull(value) {
			continue
		}
		n, ok := integerValue(value)
		if !ok {
			return nil, fmt.Errorf("where expects a bool or integer vector")
		}
		if n < 0 {
			return nil, fmt.Errorf("where expects non-negative integer counts")
		}
		for j := int64(0); j < n; j++ {
			out = append(out, int64(i))
		}
	}
	return data.NewI64(out), nil
}

func (s *EvalState) evalWhereCompare(src string) (any, bool, error) {
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(src)
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	if op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return nil, ok, err
		}
		shape := "within-to-index/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		out, handled, err := data.TryTypedWithinIndexesI64(array, low, high, true)
		recordRuntimeKernelProbe("ArrayWhereWithin", shape, handled, err)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return nil, false, nil
		}
		return out, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return nil, false, nil
	}
	shape := "compare-to-index/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	out, handled, err := data.TryTypedCompareIndexesI64(array, dataOp, scalar)
	recordRuntimeKernelProbe("ArrayWhereCompare", shape, handled, err)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

func (s *EvalState) evalWhereIn(src string) (any, bool, error) {
	return s.tryEvalWhereInIndexes("where "+src, "in-to-index")
}

func splitWhereCompareExpr(src string) (string, string, string, bool) {
	if _, _, ok := splitTopLevelWord(src, "and"); ok {
		return "", "", "", false
	}
	if _, _, ok := splitTopLevelWord(src, "or"); ok {
		return "", "", "", false
	}
	for _, op := range []string{"<>", "<=", ">=", "=", "<", ">"} {
		if left, right, ok := splitTopLevelOperator(src, op); ok {
			return left, right, op, true
		}
	}
	if left, right, ok := splitTopLevelWord(src, "within"); ok {
		return left, right, "within", true
	}
	return "", "", "", false
}

func qWhereCompareOperands(left, right any, op string) (data.Array, any, data.Op, bool) {
	if array, ok := left.(data.Array); ok {
		if _, rightIsArray := right.(data.Array); rightIsArray {
			return nil, nil, "", false
		}
		dataOp, ok := qDataCompareOpString(op)
		return array, right, dataOp, ok
	}
	array, ok := right.(data.Array)
	if !ok {
		return nil, nil, "", false
	}
	dataOp, ok := qDataCompareOpString(qReverseCompareOpString(op))
	return array, left, dataOp, ok
}

func qDataCompareOpString(op string) (data.Op, bool) {
	switch op {
	case "=":
		return data.OpEQ, true
	case "<>":
		return data.OpNE, true
	case "<":
		return data.OpLT, true
	case "<=":
		return data.OpLE, true
	case ">":
		return data.OpGT, true
	case ">=":
		return data.OpGE, true
	default:
		return "", false
	}
}

func qWithinOperands(left, right any) (data.Array, any, any, bool, error) {
	array, ok := left.(data.Array)
	if !ok {
		return nil, nil, nil, false, nil
	}
	bounds, err := vectorValues(right)
	if err != nil {
		return nil, nil, nil, true, fmt.Errorf("within expects a two-item bounds vector")
	}
	if len(bounds) != 2 {
		return nil, nil, nil, true, fmt.Errorf("within expects a two-item bounds vector")
	}
	return array, bounds[0], bounds[1], true, nil
}

func qReverseCompareOpString(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	default:
		return op
	}
}

func notValue(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		if out, handled, err := data.TryTypedNot(array); err != nil || handled {
			recordRuntimeKernelProbe("ArrayNot", "not/"+string(array.Kind()), handled, err)
			if err != nil {
				return nil, err
			}
			return out, nil
		} else {
			recordRuntimeKernelProbe("ArrayNot", "not/"+string(array.Kind()), handled, err)
		}
		out := make([]bool, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("not row %d out of range", i)
			}
			truth, err := boolValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = !truth
		}
		return data.NewBool(out), nil
	}
	truth, err := boolValue(v)
	if err != nil {
		return nil, err
	}
	return !truth, nil
}

func nullValue(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		out := make([]bool, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("null row %d out of range", i)
			}
			out[i] = data.IsNull(item)
		}
		return data.NewBool(out), nil
	}
	return data.IsNull(v), nil
}

func logicalAnd(left, right any) (any, error) {
	if out, handled, err := data.TryTypedBoolLogical("and", left, right); err != nil || handled {
		recordRuntimeKernelProbe("ArrayBoolLogical", "and/"+logicalShape(left)+"/"+logicalShape(right), handled, err)
		if err != nil {
			return nil, err
		}
		return out, nil
	} else {
		recordRuntimeKernelProbe("ArrayBoolLogical", "and/"+logicalShape(left)+"/"+logicalShape(right), handled, err)
	}
	return applyLogical(left, right, func(a, b bool) bool { return a && b })
}

func logicalOr(left, right any) (any, error) {
	if out, handled, err := data.TryTypedBoolLogical("or", left, right); err != nil || handled {
		recordRuntimeKernelProbe("ArrayBoolLogical", "or/"+logicalShape(left)+"/"+logicalShape(right), handled, err)
		if err != nil {
			return nil, err
		}
		return out, nil
	} else {
		recordRuntimeKernelProbe("ArrayBoolLogical", "or/"+logicalShape(left)+"/"+logicalShape(right), handled, err)
	}
	return applyLogical(left, right, func(a, b bool) bool { return a || b })
}

func logicalShape(value any) string {
	if array, ok := value.(data.Array); ok {
		return string(array.Kind())
	}
	return string(qRuntimeKernelOperandKind(value, nil))
}

func applyLogical(left, right any, fn func(bool, bool) bool) (any, error) {
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if !lok && !rok {
		lv, err := boolValue(left)
		if err != nil {
			return nil, err
		}
		rv, err := boolValue(right)
		if err != nil {
			return nil, err
		}
		return fn(lv, rv), nil
	}
	n := 0
	switch {
	case lok && rok:
		if la.Len() != ra.Len() && la.Len() != 1 && ra.Len() != 1 {
			return nil, fmt.Errorf("logical length mismatch")
		}
		n = la.Len()
		if ra.Len() > n {
			n = ra.Len()
		}
	case lok:
		n = la.Len()
	case rok:
		n = ra.Len()
	}
	out := make([]bool, n)
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if lok {
			var ok bool
			lv, ok = la.At(vectorIndex(i, la.Len()))
			if !ok {
				return nil, fmt.Errorf("logical left row %d out of range", i)
			}
		}
		if rok {
			var ok bool
			rv, ok = ra.At(vectorIndex(i, ra.Len()))
			if !ok {
				return nil, fmt.Errorf("logical right row %d out of range", i)
			}
		}
		lt, err := boolValue(lv)
		if err != nil {
			return nil, err
		}
		rt, err := boolValue(rv)
		if err != nil {
			return nil, err
		}
		out[i] = fn(lt, rt)
	}
	return data.NewBool(out), nil
}

func boolValue(v any) (bool, error) {
	if data.IsNull(v) {
		return false, nil
	}
	switch x := v.(type) {
	case bool:
		return x, nil
	case int:
		return x != 0, nil
	case int8:
		return x != 0, nil
	case int16:
		return x != 0, nil
	case int32:
		return x != 0, nil
	case int64:
		return x != 0, nil
	case uint:
		return x != 0, nil
	case uint8:
		return x != 0, nil
	case uint16:
		return x != 0, nil
	case uint32:
		return x != 0, nil
	case uint64:
		return x != 0, nil
	case float32:
		return x != 0, nil
	case float64:
		return x != 0, nil
	default:
		return false, fmt.Errorf("boolean operation expects bool or numeric values")
	}
}

func findValue(left, right any) (any, error) {
	domain, err := findDomainValues(left)
	if err != nil {
		return nil, err
	}
	queries, scalar, err := findQueryValues(right)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(queries))
	for i, query := range queries {
		out[i] = int64(len(domain))
		for j, candidate := range domain {
			if equalValue(candidate, query) {
				out[i] = int64(j)
				break
			}
		}
	}
	if scalar {
		return out[0], nil
	}
	return data.NewI64(out), nil
}

func findDomainValues(v any) ([]any, error) {
	switch x := v.(type) {
	case data.Array:
		return x.Values(), nil
	case string:
		runes := []rune(x)
		out := make([]any, len(runes))
		for i, r := range runes {
			out[i] = string(r)
		}
		return out, nil
	default:
		return []any{v}, nil
	}
}

func findQueryValues(v any) ([]any, bool, error) {
	switch x := v.(type) {
	case data.Array:
		return x.Values(), false, nil
	case string:
		runes := []rune(x)
		if len(runes) == 1 {
			return []any{x}, true, nil
		}
		out := make([]any, len(runes))
		for i, r := range runes {
			out[i] = string(r)
		}
		return out, false, nil
	default:
		return []any{v}, true, nil
	}
}

func membership(left, right any) (any, error) {
	if leftArray, ok := left.(data.Array); ok {
		out := make([]bool, leftArray.Len())
		for i := 0; i < leftArray.Len(); i++ {
			value, ok := leftArray.At(i)
			if !ok {
				return nil, fmt.Errorf("in left row %d out of range", i)
			}
			out[i] = containsValue(right, value)
		}
		return data.NewBool(out), nil
	}
	return containsValue(right, left), nil
}

func bin(left, right any) (any, error) {
	if domainArray, ok := left.(data.Array); ok {
		result, ok, err := data.Bin(domainArray, right)
		recordRuntimeKernelProbe("ArrayBin", qBinShape(domainArray, right), ok, err)
		if err != nil {
			return nil, err
		}
		if ok {
			return result, nil
		}
	}
	domain, err := vectorValues(left)
	if err != nil {
		return nil, fmt.Errorf("bin expects a sorted vector domain")
	}
	if rightArray, ok := right.(data.Array); ok {
		out := make([]int64, rightArray.Len())
		for i := 0; i < rightArray.Len(); i++ {
			value, ok := rightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("bin query row %d out of range", i)
			}
			out[i], err = binScalar(domain, value)
			if err != nil {
				return nil, err
			}
		}
		return data.NewI64(out), nil
	}
	return binScalar(domain, right)
}

func qBinShape(domain data.Array, query any) string {
	return "bin/" + string(domain.Kind()) + "/" + string(qRuntimeKernelOperandKind(query, nil))
}

func binScalar(domain []any, query any) (int64, error) {
	if len(domain) == 0 || data.IsNull(query) {
		return -1, nil
	}
	lo, hi := 0, len(domain)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp, err := compareOrdered(domain[mid], query)
		if err != nil {
			return 0, fmt.Errorf("bin compare: %w", err)
		}
		if cmp <= 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return int64(lo - 1), nil
}

func binr(left, right any) (any, error) {
	domain, err := vectorValues(left)
	if err != nil {
		return nil, fmt.Errorf("binr expects a sorted vector domain")
	}
	if rightArray, ok := right.(data.Array); ok {
		out := make([]int64, rightArray.Len())
		for i := 0; i < rightArray.Len(); i++ {
			value, ok := rightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("binr query row %d out of range", i)
			}
			out[i], err = binrScalar(domain, value)
			if err != nil {
				return nil, err
			}
		}
		return data.NewI64(out), nil
	}
	return binrScalar(domain, right)
}

func binrScalar(domain []any, query any) (int64, error) {
	if len(domain) == 0 || data.IsNull(query) {
		return 0, nil
	}
	lo, hi := 0, len(domain)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp, err := compareOrdered(domain[mid], query)
		if err != nil {
			return 0, fmt.Errorf("binr compare: %w", err)
		}
		if cmp < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return int64(lo), nil
}

func within(left, right any) (any, error) {
	bounds, err := vectorValues(right)
	if err != nil || len(bounds) != 2 {
		return nil, fmt.Errorf("within expects a two-item bounds vector")
	}
	if leftArray, ok := left.(data.Array); ok {
		mask, err := data.WithinMask(leftArray, bounds[0], bounds[1], true)
		if err == nil {
			return mask, nil
		}
		out := make([]bool, leftArray.Len())
		for i := 0; i < leftArray.Len(); i++ {
			value, ok := leftArray.At(i)
			if !ok {
				return nil, fmt.Errorf("within left row %d out of range", i)
			}
			ok, err := scalarWithin(value, bounds[0], bounds[1])
			if err != nil {
				return nil, err
			}
			out[i] = ok
		}
		return data.NewBool(out), nil
	}
	return scalarWithin(left, bounds[0], bounds[1])
}

func scalarWithin(value, lo, hi any) (bool, error) {
	if data.IsNull(value) || data.IsNull(lo) || data.IsNull(hi) {
		return false, nil
	}
	loCmp, err := compareOrdered(value, lo)
	if err != nil {
		return false, fmt.Errorf("within lower bound: %w", err)
	}
	hiCmp, err := compareOrdered(value, hi)
	if err != nil {
		return false, fmt.Errorf("within upper bound: %w", err)
	}
	return loCmp >= 0 && hiCmp <= 0, nil
}

func except(left, right any) (any, error) {
	leftArray, ok := left.(data.Array)
	if !ok {
		if containsValue(right, left) {
			return data.NewAny(nil), nil
		}
		return data.NewAny([]any{left}), nil
	}
	indexes := make([]int, 0, leftArray.Len())
	for i := 0; i < leftArray.Len(); i++ {
		value, ok := leftArray.At(i)
		if !ok {
			return nil, fmt.Errorf("except left row %d out of range", i)
		}
		if !containsValue(right, value) {
			indexes = append(indexes, i)
		}
	}
	return data.Gather(leftArray, indexes)
}

func inter(left, right any) (any, error) {
	leftArray, ok := left.(data.Array)
	if !ok {
		if containsValue(right, left) {
			return data.NewAny([]any{left}), nil
		}
		return data.NewAny(nil), nil
	}
	indexes := make([]int, 0, leftArray.Len())
	var seen []any
	for i := 0; i < leftArray.Len(); i++ {
		value, ok := leftArray.At(i)
		if !ok {
			return nil, fmt.Errorf("inter left row %d out of range", i)
		}
		if containsRawValue(seen, value) || !containsValue(right, value) {
			continue
		}
		seen = append(seen, value)
		indexes = append(indexes, i)
	}
	return data.Gather(leftArray, indexes)
}

func union(left, right any) (any, error) {
	leftItems, err := setItems(left)
	if err != nil {
		return nil, err
	}
	rightItems, err := setItems(right)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(leftItems)+len(rightItems))
	for _, value := range leftItems {
		if containsRawValue(out, value) {
			continue
		}
		out = append(out, value)
	}
	for _, value := range rightItems {
		if containsRawValue(out, value) {
			continue
		}
		out = append(out, value)
	}
	return data.InferArray(out), nil
}

func setItems(v any) ([]any, error) {
	if array, ok := v.(data.Array); ok {
		return array.Values(), nil
	}
	return []any{v}, nil
}

func containsValue(collection any, needle any) bool {
	items, err := setItems(collection)
	if err != nil {
		return false
	}
	return containsRawValue(items, needle)
}

func containsRawValue(items []any, needle any) bool {
	for _, item := range items {
		if equalValue(item, needle) {
			return true
		}
	}
	return false
}

func asc(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if indexes, handled, err := data.TryTypedSortIndexesI64(array, false); err != nil || handled {
		shape := "sort-gather/" + string(array.Kind()) + "/asc"
		recordRuntimeKernelProbe("ArraySortGather", shape, handled, err)
		if err != nil {
			return nil, err
		}
		out, gathered, err := data.TryGatherByI64IndexArray(array, indexes)
		recordRuntimeKernelProbe("ArrayGatherI64Indexes", "gather/"+string(array.Kind())+"/"+string(indexes.Kind()), gathered, err)
		if err != nil {
			return nil, err
		}
		if gathered {
			return out, nil
		}
	}
	indexes, err := sortedIndexes(array, false)
	if err != nil {
		return nil, err
	}
	return array.Gather(indexes), nil
}

func desc(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if indexes, handled, err := data.TryTypedSortIndexesI64(array, true); err != nil || handled {
		shape := "sort-gather/" + string(array.Kind()) + "/desc"
		recordRuntimeKernelProbe("ArraySortGather", shape, handled, err)
		if err != nil {
			return nil, err
		}
		out, gathered, err := data.TryGatherByI64IndexArray(array, indexes)
		recordRuntimeKernelProbe("ArrayGatherI64Indexes", "gather/"+string(array.Kind())+"/"+string(indexes.Kind()), gathered, err)
		if err != nil {
			return nil, err
		}
		if gathered {
			return out, nil
		}
	}
	indexes, err := sortedIndexes(array, true)
	if err != nil {
		return nil, err
	}
	return array.Gather(indexes), nil
}

func iasc(v any) (any, error) {
	return sortIndex(v, false)
}

func idesc(v any) (any, error) {
	return sortIndex(v, true)
}

func rank(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NewI64([]int64{0}), nil
	}
	if out, handled, err := data.TryTypedRankI64(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayRank", "rank/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	indexes, err := sortedIndexes(array, false)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(indexes))
	for sortedPosition, originalIndex := range indexes {
		out[originalIndex] = int64(sortedPosition)
	}
	return data.NewI64(out), nil
}

func sortIndex(v any, descending bool) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NewI64([]int64{0}), nil
	}
	if out, handled, err := data.TryTypedSortIndexesI64(array, descending); err != nil || handled {
		shape := "sort-index/" + string(array.Kind())
		if descending {
			shape += "/desc"
		} else {
			shape += "/asc"
		}
		recordRuntimeKernelProbe("ArraySortIndexes", shape, handled, err)
		if err != nil {
			return nil, err
		}
		return out, nil
	} else {
		shape := "sort-index/" + string(array.Kind())
		if descending {
			shape += "/desc"
		} else {
			shape += "/asc"
		}
		recordRuntimeKernelProbe("ArraySortIndexes", shape, handled, err)
	}
	indexes, err := sortedIndexes(array, descending)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(indexes))
	for i, index := range indexes {
		out[i] = int64(index)
	}
	return data.NewI64(out), nil
}

func sortedIndexes(array data.Array, descending bool) ([]int, error) {
	indexes := make([]int, array.Len())
	for i := range indexes {
		indexes[i] = i
	}
	values := array.Values()
	var cmpErr error
	sort.SliceStable(indexes, func(i, j int) bool {
		cmp, err := compareSortable(values[indexes[i]], values[indexes[j]])
		if err != nil {
			cmpErr = err
			return false
		}
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	if cmpErr != nil {
		return nil, cmpErr
	}
	return indexes, nil
}

func compareSortable(left, right any) (int, error) {
	leftNull := data.IsNull(left)
	rightNull := data.IsNull(right)
	switch {
	case leftNull && rightNull:
		return 0, nil
	case leftNull:
		return -1, nil
	case rightNull:
		return 1, nil
	}
	cmp, err := compareOrdered(left, right)
	if err != nil {
		return 0, fmt.Errorf("sort expects comparable values: %w", err)
	}
	return cmp, nil
}

func distinct(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	values := array.Values()
	indexes := make([]int, 0, len(values))
	for i, value := range values {
		seen := false
		for _, index := range indexes {
			if equalValue(values[index], value) {
				seen = true
				break
			}
		}
		if !seen {
			indexes = append(indexes, i)
		}
	}
	return array.Gather(indexes), nil
}

func countDistinct(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return count(v)
	}
	if value, handled, err := data.TryTypedDistinctCount(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayDistinctCount", "distinct-count/"+string(array.Kind()), handled, err)
		return value, err
	} else {
		recordRuntimeKernelProbe("ArrayDistinctCount", "distinct-count/"+string(array.Kind()), handled, err)
	}
	indexes := make([]int, 0, array.Len())
	for i := 0; i < array.Len(); i++ {
		value, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("distinct row %d out of range", i)
		}
		seen := false
		for _, index := range indexes {
			existing, ok := array.At(index)
			if !ok {
				return nil, fmt.Errorf("distinct row %d out of range", index)
			}
			if equalValue(existing, value) {
				seen = true
				break
			}
		}
		if !seen {
			indexes = append(indexes, i)
		}
	}
	return int64(len(indexes)), nil
}

func reverse(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		if s, ok := v.(string); ok {
			runes := []rune(s)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return string(runes), nil
		}
		return v, nil
	}
	if reversed, handled, err := data.Reverse(array); err != nil || handled {
		if err != nil {
			return nil, err
		}
		return reversed, nil
	}
	indexes := make([]int, array.Len())
	for i := range indexes {
		indexes[i] = array.Len() - 1 - i
	}
	return array.Gather(indexes), nil
}

func prev(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NullValue, nil
	}
	if out, handled, err := data.TryTypedPrev(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayPrev", "vector-shift/prev/"+string(array.Kind()), handled, err)
		return out, err
	}
	values := array.Values()
	out := make([]any, len(values))
	if len(out) == 0 {
		return data.InferArray(out), nil
	}
	out[0] = data.NullValue
	copy(out[1:], values[:len(values)-1])
	return inferQArray(out, array.Kind()), nil
}

func nextValue(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NullValue, nil
	}
	if out, handled, err := data.TryTypedNext(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayNext", "vector-shift/next/"+string(array.Kind()), handled, err)
		return out, err
	}
	values := array.Values()
	out := make([]any, len(values))
	if len(out) == 0 {
		return data.InferArray(out), nil
	}
	copy(out, values[1:])
	out[len(out)-1] = data.NullValue
	return inferQArray(out, array.Kind()), nil
}

func xprev(width any, v any) (any, error) {
	n64, ok := integerValue(width)
	if !ok || int64(int(n64)) != n64 {
		return nil, fmt.Errorf("xprev expects an integer count")
	}
	array, ok := v.(data.Array)
	if !ok {
		if n64 == 0 {
			return v, nil
		}
		return data.NullValue, nil
	}
	n := int(n64)
	if out, handled, err := data.TryTypedXPrev(array, n); err != nil || handled {
		recordRuntimeKernelProbe("ArrayXPrev", "vector-shift/xprev/"+string(array.Kind()), handled, err)
		return out, err
	}
	values := array.Values()
	out := make([]any, len(values))
	for i := range out {
		src := i - n
		if src < 0 || src >= len(values) {
			out[i] = data.NullValue
			continue
		}
		out[i] = values[src]
	}
	return inferQArray(out, array.Kind()), nil
}

func differ(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return true, nil
	}
	out := make([]bool, array.Len())
	for i := 0; i < array.Len(); i++ {
		if i == 0 {
			out[i] = true
			continue
		}
		current, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("differ row %d out of range", i)
		}
		previous, ok := array.At(i - 1)
		if !ok {
			return nil, fmt.Errorf("differ row %d out of range", i-1)
		}
		out[i] = !equalValue(current, previous)
	}
	return data.NewBool(out), nil
}

func deltas(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("deltas expects a numeric vector")
	}
	if out, handled, err := data.TryTypedDeltas(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayDeltas", "vector-scan/deltas/"+string(array.Kind()), handled, err)
		return out, err
	}
	out := make([]any, array.Len())
	var previous any
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("deltas row %d out of range", i)
		}
		if !data.IsNull(item) {
			if _, ok := numeric(item); !ok {
				return nil, fmt.Errorf("deltas expects a numeric vector")
			}
		}
		if !data.IsNull(previous) {
			if _, ok := numeric(previous); !ok {
				return nil, fmt.Errorf("deltas expects a numeric vector")
			}
		}
		if !data.IsNull(item) && i == 0 {
			out[i] = item
		} else if i == 0 {
			out[i] = data.NullValue
		} else {
			delta, err := applyDyadic('-', item, previous)
			if err != nil {
				return nil, err
			}
			out[i] = delta
		}
		previous = item
	}
	return inferQArray(out, array.Kind()), nil
}

func fills(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if out, handled, err := data.TryTypedFills(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayFills", "vector-scan/fills/"+string(array.Kind()), handled, err)
		return out, err
	}
	values := array.Values()
	out := make([]any, len(values))
	var fill any
	hasFill := false
	for i, value := range values {
		if data.IsNull(value) {
			if hasFill {
				out[i] = fill
			} else {
				out[i] = data.NullValue
			}
			continue
		}
		out[i] = value
		fill = value
		hasFill = true
	}
	if kind := array.Kind(); kind != "" && kind != data.KindNull && kind != data.KindAny {
		if column, err := data.NewColumnWithKind("_", kind, out); err == nil {
			return column.Data, nil
		}
	}
	return inferQArray(out, array.Kind()), nil
}

func raze(v any) (any, error) {
	if flattened, handled, err := data.FlattenNestedArray(v); err != nil || handled {
		if err != nil {
			return nil, err
		}
		if flattened.Len() == 0 {
			return data.InferArray(nil), nil
		}
		return flattened, nil
	}
	items, err := vectorValues(v)
	if err != nil {
		return v, nil
	}
	var out []any
	for _, item := range items {
		if array, ok := item.(data.Array); ok {
			out = append(out, array.Values()...)
			continue
		}
		out = append(out, item)
	}
	return data.InferArray(out), nil
}

func take(n int, v any) (any, error) {
	switch x := v.(type) {
	case data.Array:
		return data.TakeRepeat(x, n)
	case data.Frame:
		indexes := qTakeIndexes(x.Len(), n)
		if len(indexes) == 0 && n != 0 && x.Len() == 0 {
			return qGatherFrameRuntime("take", x, nil)
		}
		return qGatherFrameRuntime("take", x, indexes)
	case data.KeyedFrame:
		return takeKeyedFrame(x, n)
	case string:
		return takeString(n, x), nil
	default:
		if n == 0 {
			return data.NewAny(nil), nil
		}
		return data.TakeRepeat(inferQArray([]any{v}, qKindOfValue(v)), n)
	}
}

func xbar(width any, v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		width = normalizeXbarIntervalForKind(width, array.Kind())
		return data.BucketFloor(array, width)
	}
	column := data.NewColumn("_", []any{v}).Data
	width = normalizeXbarIntervalForKind(width, column.Kind())
	bucketed, err := data.BucketFloor(column, width)
	if err != nil {
		return nil, err
	}
	out, ok := bucketed.At(0)
	if !ok {
		return nil, fmt.Errorf("xbar result row 0 out of range")
	}
	return out, nil
}

func xrank(bucketCount any, v any) (any, error) {
	n64, ok := integerValue(bucketCount)
	if !ok || int64(int(n64)) != n64 {
		return nil, fmt.Errorf("xrank expects an integer bucket count")
	}
	if n64 <= 0 {
		return nil, fmt.Errorf("xrank expects a positive bucket count")
	}
	buckets := int(n64)
	array, ok := v.(data.Array)
	if !ok {
		if data.IsNull(v) {
			return data.NullValue, nil
		}
		return int64(0), nil
	}
	if out, handled, err := data.TryTypedXrank(n64, array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayXrank", "xrank/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, err
		}
		return out, nil
	} else {
		recordRuntimeKernelProbeReason("ArrayXrank", "xrank/"+string(array.Kind()), handled, err, RuntimeFallbackUnsupportedType)
	}
	values := array.Values()
	out := make([]any, len(values))
	indexes := make([]int, 0, len(values))
	for i, value := range values {
		if data.IsNull(value) {
			out[i] = data.NullValue
			continue
		}
		indexes = append(indexes, i)
	}
	if len(indexes) == 0 {
		return inferQArray(out, data.KindI64), nil
	}
	var cmpErr error
	sort.SliceStable(indexes, func(i, j int) bool {
		cmp, err := compareSortable(values[indexes[i]], values[indexes[j]])
		if err != nil {
			cmpErr = err
			return false
		}
		return cmp < 0
	})
	if cmpErr != nil {
		return nil, fmt.Errorf("xrank expects ordered values: %w", cmpErr)
	}
	rank := 0
	for rank < len(indexes) {
		next := rank + 1
		for next < len(indexes) {
			cmp, err := compareSortable(values[indexes[rank]], values[indexes[next]])
			if err != nil {
				return nil, fmt.Errorf("xrank expects ordered values: %w", err)
			}
			if cmp != 0 {
				break
			}
			next++
		}
		bucket := int64((rank * buckets) / len(indexes))
		if bucket >= int64(buckets) {
			bucket = int64(buckets - 1)
		}
		for _, index := range indexes[rank:next] {
			out[index] = bucket
		}
		rank = next
	}
	return inferQArray(out, data.KindI64), nil
}

func normalizeXbarInterval(width any) any {
	return normalizeXbarIntervalForKind(width, "")
}

func normalizeXbarIntervalForKind(width any, kind data.Kind) any {
	if normalized, ok := normalizeTemporalXbarIntervalForKind(width, kind); ok {
		return normalized
	}
	switch x := width.(type) {
	case data.Timespan:
		return x.Nanos()
	case data.Time:
		return x.Nanos()
	case data.Timestamp:
		return x.UnixNanos()
	case data.DateTime:
		return x.UnixNanos()
	case data.Second:
		return x.Seconds()
	case data.Minute:
		return x.Minutes()
	case data.Date:
		return x.Days()
	case data.Month:
		return x.Months()
	default:
		return width
	}
}

func normalizeTemporalXbarIntervalForKind(width any, kind data.Kind) (any, bool) {
	if kind == "" {
		return nil, false
	}
	nanos, nanosOK := xbarIntervalNanos(width)
	switch kind {
	case data.KindDateTime, data.KindTimestamp, data.KindTime, data.KindTimespan:
		if nanosOK {
			return nanos, true
		}
	case data.KindSecond:
		if nanosOK && nanos%1_000_000_000 == 0 {
			return nanos / 1_000_000_000, true
		}
	case data.KindMinute:
		if nanosOK && nanos%(60*1_000_000_000) == 0 {
			return nanos / (60 * 1_000_000_000), true
		}
	case data.KindDate:
		if nanosOK && nanos%(24*60*60*1_000_000_000) == 0 {
			return nanos / (24 * 60 * 60 * 1_000_000_000), true
		}
	case data.KindMonth:
		if x, ok := width.(data.Month); ok {
			return x.Months(), true
		}
	}
	return nil, false
}

func xbarIntervalNanos(width any) (int64, bool) {
	switch x := width.(type) {
	case data.Timespan:
		return x.Nanos(), true
	case data.Time:
		return x.Nanos(), true
	case data.Timestamp:
		return x.UnixNanos(), true
	case data.DateTime:
		return x.UnixNanos(), true
	case data.Second:
		return x.Seconds() * 1_000_000_000, true
	case data.Minute:
		return x.Minutes() * 60 * 1_000_000_000, true
	case data.Date:
		return x.Days() * 24 * 60 * 60 * 1_000_000_000, true
	default:
		return 0, false
	}
}

func msum(width any, v any) (any, error) {
	return movingNumericWindow("msum", width, v, false)
}

func mavg(width any, v any) (any, error) {
	return movingNumericWindow("mavg", width, v, true)
}

func mcount(width any, v any) (any, error) {
	n, ok := integerValue(width)
	if !ok || n <= 0 || int64(int(n)) != n {
		return nil, fmt.Errorf("mcount width must be a positive integer")
	}
	array, ok := v.(data.Array)
	if !ok {
		if data.IsNull(v) {
			return int64(0), nil
		}
		return int64(1), nil
	}
	if typed, handled, err := data.TryTypedMCount(array, int(n)); handled || err != nil {
		if err != nil {
			return nil, fmt.Errorf("mcount: %w", err)
		}
		return typed, nil
	}
	out := make([]int64, array.Len())
	for i := 0; i < array.Len(); i++ {
		start := i - int(n) + 1
		if start < 0 {
			start = 0
		}
		count := int64(0)
		for row := start; row <= i; row++ {
			item, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("mcount row %d out of range", row)
			}
			if !data.IsNull(item) {
				count++
			}
		}
		out[i] = count
	}
	return data.NewI64(out), nil
}

func mmin(width any, v any) (any, error) {
	return movingExtremaWindow("mmin", width, v, false)
}

func mmax(width any, v any) (any, error) {
	return movingExtremaWindow("mmax", width, v, true)
}

func movingExtremaWindow(name string, width any, v any, wantMax bool) (any, error) {
	n, ok := integerValue(width)
	if !ok || n <= 0 || int64(int(n)) != n {
		return nil, fmt.Errorf("%s width must be a positive integer", name)
	}
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if typed, handled, err := data.TryTypedMovingMinMax(array, int(n), wantMax); handled || err != nil {
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return typed, nil
	}
	out := make([]any, array.Len())
	for i := 0; i < array.Len(); i++ {
		start := i - int(n) + 1
		if start < 0 {
			start = 0
		}
		var best any
		hasBest := false
		for row := start; row <= i; row++ {
			item, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("%s row %d out of range", name, row)
			}
			if data.IsNull(item) {
				continue
			}
			if !hasBest {
				best = item
				hasBest = true
				continue
			}
			cmp, err := compareOrdered(item, best)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if (wantMax && cmp > 0) || (!wantMax && cmp < 0) {
				best = item
			}
		}
		if hasBest {
			out[i] = best
		} else {
			out[i] = data.NullValue
		}
	}
	return inferQArray(out, array.Kind()), nil
}

func movingNumericWindow(name string, width any, v any, average bool) (any, error) {
	n, ok := integerValue(width)
	if !ok || n <= 0 || int64(int(n)) != n {
		return nil, fmt.Errorf("%s width must be a positive integer", name)
	}
	array, ok := v.(data.Array)
	if !ok {
		if data.IsNull(v) {
			return data.NullValue, nil
		}
		if _, ok := numeric(v); !ok {
			return nil, fmt.Errorf("%s expects a numeric vector", name)
		}
		return v, nil
	}
	out := make([]any, array.Len())
	hasFloat := average
	for i := 0; i < array.Len(); i++ {
		start := i - int(n) + 1
		if start < 0 {
			start = 0
		}
		totalI := int64(0)
		totalF := float64(0)
		count := 0
		localFloat := false
		for row := start; row <= i; row++ {
			item, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("%s row %d out of range", name, row)
			}
			if data.IsNull(item) {
				continue
			}
			if integer, ok := integerValue(item); ok {
				totalI += integer
				totalF += float64(integer)
				count++
				continue
			}
			numeric, ok := numeric(item)
			if !ok {
				return nil, fmt.Errorf("%s expects a numeric vector", name)
			}
			localFloat = true
			hasFloat = true
			totalF += numeric
			count++
		}
		if count == 0 {
			out[i] = data.NullValue
			continue
		}
		if average {
			out[i] = totalF / float64(count)
		} else if localFloat {
			out[i] = totalF
		} else {
			out[i] = totalI
		}
	}
	if hasFloat {
		for _, value := range out {
			if data.IsNull(value) {
				return data.InferArray(out), nil
			}
		}
		xs := make([]float64, len(out))
		for i, value := range out {
			xs[i], _ = numeric(value)
		}
		return data.NewF64(xs), nil
	}
	return data.InferArray(out), nil
}

func qTakeIndexes(length, n int) []int {
	if n == 0 || length == 0 {
		return nil
	}
	count := n
	if count < 0 {
		count = -count
	}
	indexes := make([]int, count)
	start := 0
	if n < 0 {
		start = length - count%length
		if start == length {
			start = 0
		}
	}
	for i := range indexes {
		indexes[i] = (start + i) % length
	}
	return indexes
}

func qTakeCount(n int, v any) int64 {
	if n == 0 {
		return 0
	}
	switch x := v.(type) {
	case data.Array:
		if x.Len() == 0 {
			return 0
		}
	case data.Frame:
		if x.Len() == 0 {
			return 0
		}
	case data.KeyedFrame:
		if data.KeyedFrameLen(x) == 0 {
			return 0
		}
	case string:
		if len([]rune(x)) == 0 {
			return 0
		}
	}
	if n < 0 {
		n = -n
	}
	return int64(n)
}

func takeString(n int, v string) string {
	runes := []rune(v)
	indexes := qTakeIndexes(len(runes), n)
	out := make([]rune, len(indexes))
	for i, index := range indexes {
		out[i] = runes[index]
	}
	return string(out)
}

func drop(n int, v any) (any, error) {
	switch x := v.(type) {
	case data.Array:
		return dropArray(x, n)
	case data.Frame:
		return dropFrame(x, n)
	case data.KeyedFrame:
		return dropKeyedFrame(x, n)
	case string:
		return dropString(x, n), nil
	default:
		if n == 0 {
			return v, nil
		}
		return data.NewAny(nil), nil
	}
}

func rotateValue(left any, right any) (any, error) {
	n64, ok := left.(int64)
	if !ok || int64(int(n64)) != n64 {
		return nil, fmt.Errorf("rotate expects an integer count")
	}
	n := int(n64)
	switch x := right.(type) {
	case data.Array:
		if rotated, handled, err := data.TryTypedRotate(x, n); err != nil || handled {
			if err != nil {
				return nil, err
			}
			return rotated, nil
		}
		return data.Gather(x, rotateIndexes(x.Len(), n))
	case data.Frame:
		return qGatherFrameRuntime("rotate", x, rotateIndexes(x.Len(), n))
	case string:
		runes := []rune(x)
		indexes := rotateIndexes(len(runes), n)
		out := make([]rune, len(indexes))
		for i, index := range indexes {
			out[i] = runes[index]
		}
		return string(out), nil
	default:
		return right, nil
	}
}

func rotateIndexes(length, n int) []int {
	indexes := make([]int, length)
	if length == 0 {
		return indexes
	}
	n %= length
	if n < 0 {
		n += length
	}
	for i := range indexes {
		indexes[i] = (i + n) % length
	}
	return indexes
}

func cutOrDrop(left any, right any) (any, error) {
	switch x := left.(type) {
	case int64:
		return drop(int(x), right)
	case data.Array:
		if x.Kind() != data.KindI64 {
			return nil, fmt.Errorf("_ left vector must contain integer cut indexes")
		}
		indexes := make([]int, x.Len())
		for i, value := range x.Values() {
			n, ok := value.(int64)
			if !ok || n < 0 || int64(int(n)) != n {
				return nil, fmt.Errorf("_ cut indexes must be non-negative integers")
			}
			indexes[i] = int(n)
		}
		return data.Cut(indexes, right)
	default:
		return nil, fmt.Errorf("_ left operand must be an integer or integer vector")
	}
}

func dropArray(array data.Array, n int) (data.Array, error) {
	start := 0
	count := array.Len()
	if n >= 0 {
		if n > count {
			n = count
		}
		start = n
		count -= n
	} else {
		n = -n
		if n > count {
			n = count
		}
		count -= n
	}
	return data.Slice(array, start, count)
}

func dropFrame(frame data.Frame, n int) (data.Frame, error) {
	indexes := dropIndexes(frame.Len(), n)
	return qGatherFrameRuntime("drop", frame, indexes)
}

func takeKeyedFrame(frame data.KeyedFrame, n int) (data.KeyedFrame, error) {
	gathered, err := take(n, frame.Frame())
	if err != nil {
		return data.KeyedFrame{}, err
	}
	out, ok := gathered.(data.Frame)
	if !ok {
		return data.KeyedFrame{}, fmt.Errorf("take keyed table returned %T, want table", gathered)
	}
	return data.KeyBy(out, frame.Keys()...)
}

func dropKeyedFrame(frame data.KeyedFrame, n int) (data.KeyedFrame, error) {
	gathered, err := dropFrame(frame.Frame(), n)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.KeyBy(gathered, frame.Keys()...)
}

func dropString(s string, n int) string {
	runes := []rune(s)
	indexes := dropIndexes(len(runes), n)
	out := make([]rune, len(indexes))
	for i, index := range indexes {
		out[i] = runes[index]
	}
	return string(out)
}

func dropIndexes(length, n int) []int {
	start := 0
	end := length
	if n >= 0 {
		if n > length {
			n = length
		}
		start = n
	} else {
		n = -n
		if n > length {
			n = length
		}
		end = length - n
	}
	indexes := make([]int, end-start)
	for i := range indexes {
		indexes[i] = start + i
	}
	return indexes
}

func takeArrayTail(array data.Array, n int) (data.Array, error) {
	if n > array.Len() {
		n = array.Len()
	}
	start := array.Len() - n
	return data.Slice(array, start, n)
}

func takeFrameTail(frame data.Frame, n int) (data.Frame, error) {
	if n > frame.Len() {
		n = frame.Len()
	}
	start := frame.Len() - n
	indexes := make([]int, n)
	for i := range indexes {
		indexes[i] = start + i
	}
	return qGatherFrameRuntime("take-tail", frame, indexes)
}

func vectorValues(v any) ([]any, error) {
	switch x := v.(type) {
	case data.Array:
		return x.Values(), nil
	default:
		return nil, fmt.Errorf("not a vector")
	}
}

func equalValue(left, right any) bool {
	if data.IsNull(left) || data.IsNull(right) {
		return data.IsNull(left) && data.IsNull(right)
	}
	return reflect.DeepEqual(left, right)
}

func equalComparableValue(left, right any) bool {
	if data.IsNull(left) || data.IsNull(right) {
		return data.IsNull(left) && data.IsNull(right)
	}
	switch l := left.(type) {
	case string:
		if r, ok := right.(data.Symbol); ok {
			return l == string(r)
		}
	case data.Symbol:
		if r, ok := right.(string); ok {
			return string(l) == r
		}
	}
	return reflect.DeepEqual(left, right)
}

func matchValue(left, right any) bool {
	if data.IsNull(left) || data.IsNull(right) {
		return data.IsNull(left) && data.IsNull(right)
	}
	leftEnum, leftIsEnum := left.(qEnumVector)
	rightEnum, rightIsEnum := right.(qEnumVector)
	if leftIsEnum || rightIsEnum {
		if !leftIsEnum || !rightIsEnum || leftEnum.domain != rightEnum.domain {
			return false
		}
		return reflect.DeepEqual(leftEnum.EncodedDomain(), rightEnum.EncodedDomain()) &&
			reflect.DeepEqual(leftEnum.EncodedCodes(), rightEnum.EncodedCodes())
	}
	leftArray, leftIsArray := left.(data.Array)
	rightArray, rightIsArray := right.(data.Array)
	if leftIsArray || rightIsArray {
		if !leftIsArray || !rightIsArray || leftArray.Kind() != rightArray.Kind() || leftArray.Len() != rightArray.Len() {
			return false
		}
		for i := 0; i < leftArray.Len(); i++ {
			leftItem, leftOK := leftArray.At(i)
			rightItem, rightOK := rightArray.At(i)
			if !leftOK || !rightOK || !matchValue(leftItem, rightItem) {
				return false
			}
		}
		return true
	}
	leftDict, leftIsDict := left.(EvalDict)
	rightDict, rightIsDict := right.(EvalDict)
	if leftIsDict || rightIsDict {
		if !leftIsDict || !rightIsDict || len(leftDict.Keys) != len(rightDict.Keys) || len(leftDict.Values) != len(rightDict.Values) {
			return false
		}
		for i := range leftDict.Keys {
			if !matchValue(leftDict.Keys[i], rightDict.Keys[i]) || !matchValue(leftDict.Values[i], rightDict.Values[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(left, right)
}

func likeValue(left, right any) (any, error) {
	leftArray, leftIsArray := left.(data.Array)
	rightArray, rightIsArray := right.(data.Array)
	if !leftIsArray && !rightIsArray {
		return likeScalar(left, right)
	}
	length := 1
	if leftIsArray {
		length = leftArray.Len()
	}
	if rightIsArray {
		if leftIsArray && rightArray.Len() != length {
			return nil, fmt.Errorf("like vector length mismatch: %d vs %d", leftArray.Len(), rightArray.Len())
		}
		length = rightArray.Len()
	}
	out := make([]bool, length)
	for i := 0; i < length; i++ {
		leftItem := left
		if leftIsArray {
			item, ok := leftArray.At(i)
			if !ok {
				return nil, fmt.Errorf("like left row %d out of range", i)
			}
			leftItem = item
		}
		rightItem := right
		if rightIsArray {
			item, ok := rightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("like right row %d out of range", i)
			}
			rightItem = item
		}
		matched, err := likeScalar(leftItem, rightItem)
		if err != nil {
			return nil, err
		}
		out[i] = matched
	}
	return data.NewBool(out), nil
}

func likeScalar(left, right any) (bool, error) {
	if data.IsNull(left) || data.IsNull(right) {
		return false, nil
	}
	text, ok := qLikeText(left)
	if !ok {
		return false, fmt.Errorf("like left expects string or symbol, got %T", left)
	}
	pattern, ok := qLikeText(right)
	if !ok {
		return false, fmt.Errorf("like right expects string or symbol pattern, got %T", right)
	}
	matched, err := path.Match(pattern, text)
	if err != nil {
		return false, fmt.Errorf("like pattern %q: %w", pattern, err)
	}
	return matched, nil
}

func qLikeText(value any) (string, bool) {
	switch x := value.(type) {
	case string:
		return x, true
	case data.Symbol:
		return string(x), true
	default:
		return "", false
	}
}

func qValueKey(v any) string {
	if data.IsNull(v) {
		return "null"
	}
	return fmt.Sprintf("%T:%v", v, v)
}

func compareOrdered(left, right any) (int, error) {
	if ln, ok := numeric(left); ok {
		rn, ok := numeric(right)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareFloat(ln, rn), nil
	}
	switch l := left.(type) {
	case string:
		if r, ok := right.(data.Symbol); ok {
			return strings.Compare(l, string(r)), nil
		}
		r, ok := right.(string)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return strings.Compare(l, r), nil
	case data.Symbol:
		if r, ok := right.(string); ok {
			return strings.Compare(string(l), r), nil
		}
		r, ok := right.(data.Symbol)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return strings.Compare(string(l), string(r)), nil
	case data.Date:
		r, ok := right.(data.Date)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(l.Days(), r.Days()), nil
	case data.Month:
		r, ok := right.(data.Month)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.DateTime:
		r, ok := right.(data.DateTime)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Timespan:
		r, ok := right.(data.Timespan)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Minute:
		r, ok := right.(data.Minute)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Second:
		r, ok := right.(data.Second)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Time:
		r, ok := right.(data.Time)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(l.Nanos(), r.Nanos()), nil
	case data.Timestamp:
		r, ok := right.(data.Timestamp)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(l.UnixNanos(), r.UnixNanos()), nil
	default:
		return 0, fmt.Errorf("ordered comparison is not supported for %T", left)
	}
}

func compareFloat(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
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

func integerValue(v any) (int64, bool) {
	switch n := v.(type) {
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}
