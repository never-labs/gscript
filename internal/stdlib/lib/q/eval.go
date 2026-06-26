package q

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
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
	source        string
	kernel        string
	shape         string
	pipelineShape string
	route         string
	outcome       string
	reasonCode    string
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
	recordRuntimeExecutionWithPipelineShape(source, kernel, shape, "", route, outcome, reasonCode)
}

func recordRuntimeExecutionWithPipelineShape(source, kernel, shape, pipelineShape, route, outcome, reasonCode string) {
	reasonCode = normalizeRuntimeKernelReasonCode(outcome, reasonCode)
	shape = normalizeRuntimeStatField(shape, "unknown")
	if pipelineShape == "" {
		pipelineShape = qRuntimeKernelPipelineShape(kernel, shape)
	}
	pipelineShape = normalizeRuntimeStatField(pipelineShape, "unknown")
	key := runtimeKernelExecutionKey{
		source:        normalizeRuntimeStatField(source, "q_eval_runtime"),
		kernel:        normalizeRuntimeStatField(kernel, "unknown"),
		shape:         shape,
		pipelineShape: pipelineShape,
		route:         normalizeRuntimeStatField(route, "runtime_primitive"),
		outcome:       normalizeRuntimeStatField(outcome, "unknown"),
		reasonCode:    normalizeRuntimeStatField(reasonCode, outcome),
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
		if reasonCode == "" {
			return RuntimeFallbackRuntimeError
		}
		return RuntimeErrorReasonCode(reasonCode)
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
		pipelineShape: normalizeRuntimeStatField(key.pipelineShape, qRuntimeKernelPipelineShape(key.kernel, key.shape)),
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
	hash = hash*33 + runtimeKernelStringHash(key.pipelineShape)
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
	case strings.HasPrefix(shape, "sort-rank-reducer-bundle/"),
		strings.HasPrefix(shape, "sort-index-sum/"),
		strings.HasPrefix(shape, "rank-sum/"),
		strings.HasPrefix(shape, "sort-edge/"):
		return "sort_gather"
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
	case strings.HasPrefix(shape, "q-cast/"), strings.HasPrefix(shape, "cast-envelope/"):
		return "cast"
	case strings.HasPrefix(shape, "runtime-unary/"):
		return qRuntimePrimitivePipelineShape(strings.TrimPrefix(shape, "runtime-unary/"))
	case strings.HasPrefix(shape, "runtime-dyadic/"):
		return qRuntimePrimitivePipelineShape(strings.TrimPrefix(shape, "runtime-dyadic/"))
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
	if spec, ok := qPipelineShapeSpecForShape(shape); ok {
		return spec.PipelineShape
	}
	return ""
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
	envBorrowed          bool
	port                 int64
	namespace            string
	oneShot              bool
	scriptCache          map[string]qScriptPlan
	valueExprCache       map[string]Expr
	pipelineCache        map[string]*qPipelinePlan
	pipelineCache1Src    string
	pipelineCache1Plan   *qPipelinePlan
	skipPipelineRemember bool
	constValueCache      map[string]any
	applyIndexCache      map[string]qScalarApplyIndexPlan
	dotApplyCache        map[string]qDotApplyPlan
	deferScanAssignments map[string]bool
	assignPool           qAssignPool
	scriptDepth          int
	// valueDepth guards the session-aware value/eval verbs against
	// self-referential recursion (a:"value a";value a); see value_eval_parse.go.
	valueDepth int
	// rng backs the nondeterministic roll/deal (`x?y`) and `rand` verbs.
	// It is lazily seeded with qDefaultRandSeed so a fresh session is
	// reproducible (q's \S analog: a fixed default seed, settable later);
	// within one session successive draws advance the stream.
	rng *rand.Rand
}

// qDefaultRandSeed is the fixed default PRNG seed for roll/deal/rand,
// mirroring q's deterministic default \S seed.
const qDefaultRandSeed int64 = 0x5EED

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
	state := EvalState{env: env, envBorrowed: true, namespace: ".", oneShot: true}
	return state.Eval(src)
}

// PreparedEval pins the source-derived q.eval plan chain for repeated
// execution with caller-supplied environments. It caches parse, script, and
// typed-runtime executable plans, but never memoizes results.
//
// PreparedEval is safe for concurrent callers; execution is serialized because
// the underlying plan chain contains lazy runtime metadata caches.
type PreparedEval struct {
	source string
	entry  *evalSessionPlan
	state  EvalState
	mu     sync.Mutex
}

// PrepareEval compiles a cacheable q source into a reusable execution handle.
// Stateful, random, filesystem, IPC, and empty sources are rejected for the
// same reason EvalSourceCacheable rejects them: callers must not accidentally
// stabilize observably dynamic q code.
func PrepareEval(src string) (*PreparedEval, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("q: cannot prepare empty source")
	}
	if !EvalSourceCacheable(src) {
		return nil, fmt.Errorf("q: source is not safe for prepared eval")
	}
	session := &EvalSession{state: &EvalState{namespace: ".", oneShot: true}}
	entry := session.plan(src)
	if entry == nil {
		return nil, fmt.Errorf("q: prepared eval plan is empty")
	}
	return &PreparedEval{source: src, entry: entry}, nil
}

// Source returns the normalized source pinned by the prepared handle.
func (p *PreparedEval) Source() string {
	if p == nil {
		return ""
	}
	return p.source
}

// EvalWithEnv evaluates the prepared q source against env. Assignments and
// temporary q bindings remain local to the evaluation and do not mutate env.
func (p *PreparedEval) EvalWithEnv(env map[string]any) (any, error) {
	if p == nil || p.entry == nil {
		return nil, fmt.Errorf("q: prepared eval handle is empty")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.resetBorrowedEnv(env)
	defer p.state.clearBorrowedEnv()
	return p.state.evalScriptPlan(p.entry.script)
}

// EvalSourceCacheable reports whether src is safe for callers to memoize across
// stateless Eval calls. Stateful workspace, system, IPC, and filesystem forms
// stay uncached so repeated q.eval calls remain observably fresh, and
// random (roll/deal/rand) forms are never memoized. Verb classification
// comes from the verb metadata table (verb_metadata.go).
func EvalSourceCacheable(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	return !qVerbMemoUnsafe(qSourceVerbFlags(src))
}

// qSourceHasRandomVerb conservatively reports whether src may invoke a
// Random-flagged verb (roll/deal via dyadic `?`, or `rand`); see
// qScanMentionsRandomVerb in verb_metadata.go for the scan rules.
func qSourceHasRandomVerb(src string) bool {
	return qScanMentionsRandomVerb(qEvalCacheScanText(src))
}

// qEvalConstantWords lists identifier tokens that keep an expression
// session-constant beyond the unary/dyadic verb registries: adverb words,
// dyadic word forms, and literal keywords. Nondeterministic words (rand,
// roll, deal) and workspace-reading words stay out so they are treated as
// names and disqualify constant memoization.
var qEvalConstantWords = map[string]struct{}{
	"each": {}, "over": {}, "scan": {}, "prior": {}, "fby": {},
	"til": {}, "where": {}, "mod": {}, "div": {}, "and": {}, "or": {},
	"in": {}, "within": {}, "like": {}, "xbar": {}, "xrank": {},
	"cut": {}, "sublist": {}, "rotate": {}, "cross": {}, "vs": {}, "sv": {},
	"union": {}, "inter": {}, "intersect": {}, "except": {},
	"msum": {}, "mavg": {}, "mcount": {}, "mmin": {}, "mmax": {}, "mdev": {},
	"ema": {}, "xprev": {}, "xcols": {}, "xkey": {}, "xgroup": {},
	"xasc": {}, "xdesc": {}, "bin": {}, "binr": {}, "fill": {},
	"true": {}, "false": {}, "left": {}, "right": {},
}

// qEvalConstantStatementSource reports whether src is a closed q expression:
// every bare identifier resolves to a deterministic builtin verb, so the
// expression denotes the same value on every session-warm call and its
// result may be memoized. Assignments, lambdas, amend forms, temporal
// colon literals, and random (`?`) forms are rejected conservatively.
func qEvalConstantStatementSource(src string) bool {
	if !EvalSourceCacheable(src) {
		return false
	}
	scan := qEvalCacheScanText(src)
	for i := 0; i < len(scan); i++ {
		ch := scan[i]
		switch ch {
		case '{', '}', ':', '?', '@', '.':
			return false
		case '`':
			// Skip symbol atoms: their identifier text is data, not a name.
			j := i + 1
			for j < len(scan) && isQIdentByte(scan[j]) {
				j++
			}
			i = j - 1
			continue
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			start := i
			j := i
			for j < len(scan) && isQIdentByte(scan[j]) {
				j++
			}
			word := scan[start:j]
			i = j - 1
			// Numeric type suffixes (10f, 0N, 0w) ride directly on a digit.
			if start > 0 && scan[start-1] >= '0' && scan[start-1] <= '9' {
				continue
			}
			// Verb classification comes from the metadata table: only
			// known deterministic verbs/forms keep the expression closed;
			// random/stateful/clock names and free user names disqualify it.
			props, known := qVerbMetadataLookup(word)
			if !known || qVerbMemoUnsafe(props) {
				return false
			}
			continue
		}
	}
	return true
}

func isQIdentByte(ch byte) bool {
	return ch == '_' || (ch >= '0' && ch <= '9') ||
		(ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
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
	return s.evalScriptPlan(plan)
}

func (s *EvalState) evalScriptPlan(plan qScriptPlan) (any, error) {
	s.scriptDepth++
	defer func() { s.scriptDepth-- }()
	if s.scriptDepth > 1 {
		s.assignPoolNested()
		return s.evalScriptPlanBody(plan)
	}
	s.assignPoolBegin(&plan)
	out, err := s.evalScriptPlanBody(plan)
	s.assignPoolEnd(out, err)
	return out, err
}

func (s *EvalState) evalScriptPlanScalar(plan qScriptPlan) (EvalScalarResult, bool, error) {
	s.scriptDepth++
	defer func() { s.scriptDepth-- }()
	return s.evalScriptPlanScalarBody(plan)
}

func (s *EvalState) evalScriptPlanScalarBody(plan qScriptPlan) (EvalScalarResult, bool, error) {
	if plan.numericSum != nil {
		if out, handled, err := s.evalQScriptNumericSumPlanScalar(plan.numericSum); err != nil || handled {
			return out, handled, err
		}
	}
	if plan.numericMultiSum != nil {
		if out, handled, err := s.evalQScriptNumericMultiSumPlanScalar(plan.numericMultiSum); err != nil || handled {
			return out, handled, err
		}
	}
	if plan.whereIndexSum != nil {
		if out, handled, err := s.evalQScriptWhereIndexSumPlanScalar(plan.whereIndexSum); err != nil || handled {
			return out, handled, err
		}
	}
	if plan.whereIndexOnlySum != nil {
		if out, handled, err := s.evalQScriptWhereIndexOnlySumPlanScalar(plan.whereIndexOnlySum); err != nil || handled {
			return out, handled, err
		}
	}
	if plan.countWhere != nil {
		if out, handled, err := s.evalQScriptCountWherePlanScalar(plan.countWhere); err != nil || handled {
			return out, handled, err
		}
	}
	return EvalScalarResult{}, false, nil
}

func (s *EvalState) evalScriptPlanBody(plan qScriptPlan) (any, error) {
	if plan.numericSum != nil {
		if out, handled, err := s.evalQScriptNumericSumPlan(plan.numericSum); err != nil || handled {
			return out, err
		}
	}
	if plan.numericMultiSum != nil {
		if out, handled, err := s.evalQScriptNumericMultiSumPlan(plan.numericMultiSum); err != nil || handled {
			return out, err
		}
	}
	if plan.numericStats != nil {
		if out, handled, err := s.evalQScriptNumericStatsPlan(plan.numericStats); err != nil || handled {
			return out, err
		}
	}
	if plan.whereIndexSum != nil {
		if out, handled, err := s.evalQScriptWhereIndexSumPlan(plan.whereIndexSum); err != nil || handled {
			return out, err
		}
	}
	if plan.whereIndexOnlySum != nil {
		if out, handled, err := s.evalQScriptWhereIndexOnlySumPlan(plan.whereIndexOnlySum); err != nil || handled {
			return out, err
		}
	}
	if plan.whereIndexFbySum != nil {
		if out, handled, err := s.evalQScriptWhereIndexFbySumPlan(plan.whereIndexFbySum); err != nil || handled {
			return out, err
		}
	}
	if plan.whereIndexWindow != nil {
		if out, handled, err := s.evalQScriptWhereIndexWindowPlan(plan.whereIndexWindow); err != nil || handled {
			return out, err
		}
	}
	if plan.countWhere != nil {
		if out, handled, err := s.evalQScriptCountWherePlan(plan.countWhere); err != nil || handled {
			return out, err
		}
	}
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
		return s.evalScriptStatement(&plan.statements[0])
	}
	var last any
	for i := 0; i < len(plan.statements); i++ {
		stmt := &plan.statements[i]
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
	numericSum          *qScriptNumericSumPlan
	numericMultiSum     *qScriptNumericMultiSumPlan
	numericStats        *qScriptNumericStatsPlan
	whereIndexSum       *qScriptWhereIndexSumPlan
	whereIndexOnlySum   *qScriptWhereIndexOnlySumPlan
	whereIndexFbySum    *qScriptWhereIndexFbySumPlan
	whereIndexWindow    *qScriptWhereIndexWindowPlan
	countWhere          *qScriptCountWherePlan
	fastPipelineSources []string
}

type qScriptStatement struct {
	src    string
	assign string
	rhs    string
	// idxAssignName/idxAssignIndex carry indexed-assignment statements
	// (`name[i;j;...]: rhs`, the statement form of @/. amend). assign stays
	// empty for them so plain-binding probes (assign pool, deferred scans,
	// reduction bundles, pipeline descriptors) skip these statements; rhs
	// and the rhs plans below are still populated. idxAssignOp carries the
	// compound operator for `name[i]op: rhs` (the @[;;op;] amend form).
	idxAssignName  string
	idxAssignIndex string
	idxAssignOp    string
	valueExpr      Expr
	bindingPlan    qScriptBindingPlan
	fastPlan       qEvalFastPlan
	// compareIndexChecked/compareIndexPlan memoize the
	// tryEvalCompareIndexStatsAssignment plan build (a per-call string walk
	// otherwise); the runtime operand evaluation and kind checks still run
	// per call.
	compareIndexChecked bool
	compareIndexPlan    *qPipelinePlan
	// reduction/reductionOK memoize parseNumericReductionBinding at plan
	// build time: the classification is purely syntactic (assign + rhs),
	// and re-parsing it per evaluation costs string concatenation in hot
	// multi-statement scripts.
	reduction   numericReductionBinding
	reductionOK bool
	// fillsFillChecked/fillsFillPlan memoize the buildQFillsFillPlan
	// syntactic probe for the pooled `<atom>^fills <name>` assignment route.
	fillsFillChecked bool
	fillsFillPlan    *qFillsFillPlan
}

type qEvalFastPlanKind uint8

const (
	qEvalFastInvalid qEvalFastPlanKind = iota
	qEvalFastPipeline
	qEvalFastScalarApplyIndex
	qEvalFastSortRankReducerBundle
	qEvalFastNamePostfixSymbol
	qEvalFastFby
	qEvalFastAddChain
	qEvalFastSingleTerm
)

// qEvalSyntaxState memoizes a pure syntactic predicate computed at plan
// build time. The zero value (unknown) keeps the per-call probe for plans
// built outside buildQEvalFastPlan.
type qEvalSyntaxState uint8

const (
	qEvalSyntaxUnknown qEvalSyntaxState = iota
	qEvalSyntaxYes
	qEvalSyntaxNo
)

type qEvalConstState uint8

const (
	qEvalConstUnknown qEvalConstState = iota
	qEvalConstValue
	qEvalConstNot
)

type qEvalFastPlan struct {
	kind                    qEvalFastPlanKind
	pipeline                qPipelinePlan
	scalarIndex             qScalarApplyIndexPlan
	sortRankTerms           []qSortRankReducerTermPlan
	sortRankStaticResult    int64
	hasSortRankStaticResult bool
	postfixName             string
	postfixSymbol           data.Symbol
	constState              qEvalConstState
	fby                     *qFbyFastPlan
	addChainTerms           []qAddChainTermPlan
	// applyIndexState memoizes the syntactic gate of evalApplyIndexForm so
	// warm statements skip the per-call @/. apply string walks.
	applyIndexState qEvalSyntaxState
}

type qScriptExecutableKind uint8

const (
	qScriptExecutableInvalid qScriptExecutableKind = iota
	qScriptExecutableSingleStatement
	qScriptExecutablePipelineBackend
)

type qScriptExecutablePlan struct {
	kind         qScriptExecutableKind
	statement    qScriptStatement
	uncachedWarm bool
	pipeline     EvalPipelineExecutablePlan
}

func (s *EvalState) qScriptPlan(src string) qScriptPlan {
	src = strings.TrimSpace(src)
	if s.scriptCache != nil {
		if plan, ok := s.scriptCache[src]; ok {
			return plan
		}
	}
	if s.oneShot {
		if plan, ok := qGlobalScriptPlanCacheHit(src); ok {
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
		qRecordScriptPlanFastPipelineCacheHitsLocked(plan)
	} else {
		qGlobalScriptPlanStats.ScriptMisses++
	}
	qGlobalScriptPlanCacheMu.Unlock()
	if !ok {
		return qScriptPlan{}, false
	}
	return plan, true
}

func qGlobalScriptPlanCacheHit(src string) (qScriptPlan, bool) {
	qGlobalScriptPlanCacheMu.Lock()
	plan, ok := qGlobalScriptPlanCache[src]
	if ok {
		qGlobalScriptPlanStats.ScriptHits++
		qRecordScriptPlanFastPipelineCacheHitsLocked(plan)
	}
	qGlobalScriptPlanCacheMu.Unlock()
	if !ok {
		return qScriptPlan{}, false
	}
	return plan, true
}

func qRecordScriptPlanFastPipelineCacheHitsLocked(plan qScriptPlan) {
	for _, src := range plan.fastPipelineSources {
		if _, ok := qGlobalPipelinePlanCache[src]; ok {
			qGlobalScriptPlanStats.PipelineHits++
		} else {
			qGlobalScriptPlanStats.PipelineMisses++
		}
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
	indexedAssign := false
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
		} else if name, indexSrc, op, rhs, ok := splitTopLevelIndexedAssignment(part); ok {
			stmt.idxAssignName = name
			stmt.idxAssignIndex = indexSrc
			stmt.idxAssignOp = op
			stmt.rhs = rhs
			stmt.valueExpr = parseCachedValueExpr(rhs)
			stmt.bindingPlan = buildQScriptWarmBindingPlan(rhs, stmt.valueExpr)
			stmt.fastPlan = buildQEvalFastPlan(rhs)
			indexedAssign = true
		} else {
			stmt.valueExpr = parseCachedValueExpr(part)
			stmt.fastPlan = buildQEvalFastPlan(part)
		}
		stmt.reduction, stmt.reductionOK = parseNumericReductionBinding(stmt)
		statements = append(statements, stmt)
	}
	// Indexed-assignment statements rebind names through the amend
	// machinery; the pipeline descriptor only models plain `name: rhs`
	// bindings, so scripts containing them stay on the statement walker.
	var pipeline *qScriptPipelineDescriptor
	if !indexedAssign {
		pipeline, _ = buildQScriptPipelineDescriptor(statements)
	}
	plan := qScriptPlan{
		statements:          statements,
		deferScanCandidates: deferScanCandidates,
		scriptPipeline:      pipeline,
		numericSum:          buildQScriptNumericSumPlan(statements),
		numericMultiSum:     buildQScriptNumericMultiSumPlan(statements),
		numericStats:        buildQScriptNumericStatsPlan(statements),
		whereIndexSum:       buildQScriptWhereIndexSumPlan(statements),
		whereIndexOnlySum:   buildQScriptWhereIndexOnlySumPlan(statements),
		whereIndexFbySum:    buildQScriptWhereIndexFbySumPlan(statements),
		whereIndexWindow:    buildQScriptWhereIndexWindowPlan(statements),
		countWhere:          buildQScriptCountWherePlan(statements),
		fastPipelineSources: qScriptPlanFastPipelineSources(statements),
	}
	plan.executable = buildQScriptExecutablePlan(plan)
	return plan
}

func qScriptPlanFastPipelineSources(statements []qScriptStatement) []string {
	var sources []string
	for _, stmt := range statements {
		if stmt.fastPlan.kind != qEvalFastPipeline || stmt.fastPlan.pipeline.source == "" {
			continue
		}
		sources = append(sources, stmt.fastPlan.pipeline.source)
	}
	return sources
}

func buildQScriptExecutablePlan(plan qScriptPlan) *qScriptExecutablePlan {
	if plan.scriptPipeline != nil {
		return &qScriptExecutablePlan{
			kind:     qScriptExecutablePipelineBackend,
			pipeline: evalPipelineScriptExecutable(plan.scriptPipeline),
		}
	}
	if plan.deferScanCandidates || len(plan.statements) != 1 {
		return nil
	}
	stmt := plan.statements[0]
	if stmt.assign != "" || stmt.idxAssignName != "" || stmt.src == "" {
		return nil
	}
	if stmt.bindingPlan.kind == qScriptBindingInvalid && stmt.fastPlan.kind == qEvalFastInvalid && stmt.valueExpr == nil {
		return nil
	}
	return &qScriptExecutablePlan{
		kind:         qScriptExecutableSingleStatement,
		statement:    stmt,
		uncachedWarm: stmt.fastPlan.constState == qEvalConstNot,
	}
}

func (s *EvalState) evalQScriptExecutablePlan(plan *qScriptExecutablePlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	switch plan.kind {
	case qScriptExecutableSingleStatement:
		if s.oneShot || plan.uncachedWarm {
			stmtPtr := &plan.statement
			out, err := s.evalCachedOrStringUncached(stmtPtr.src, stmtPtr.valueExpr, &stmtPtr.bindingPlan, &stmtPtr.fastPlan)
			return out, true, err
		}
		stmt := plan.statement
		out, err := s.evalCachedOrString(stmt.src, stmt.valueExpr, &stmt.bindingPlan, &stmt.fastPlan)
		return out, true, err
	case qScriptExecutablePipelineBackend:
		out, handled, err := s.ExecuteEvalPipelineExecutablePlanRef(&plan.pipeline)
		if handled && err == nil {
			recordQEvalDispatch("<script pipeline backend>", EvalDispatchPipelineBackend)
		}
		return out, handled, err
	default:
		return nil, false, nil
	}
}

func cloneQScriptPlan(plan qScriptPlan) qScriptPlan {
	out := qScriptPlan{
		deferScanCandidates: plan.deferScanCandidates,
		scriptPipeline:      cloneQScriptPipelineDescriptor(plan.scriptPipeline),
		executable:          cloneQScriptExecutablePlan(plan.executable),
		numericSum:          plan.numericSum,
		numericMultiSum:     plan.numericMultiSum,
		numericStats:        plan.numericStats,
		whereIndexSum:       plan.whereIndexSum,
		whereIndexOnlySum:   plan.whereIndexOnlySum,
		whereIndexFbySum:    plan.whereIndexFbySum,
		whereIndexWindow:    plan.whereIndexWindow,
		countWhere:          plan.countWhere,
		fastPipelineSources: append([]string(nil), plan.fastPipelineSources...),
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
	out.pipeline = cloneEvalPipelineExecutablePlan(in.pipeline)
	return &out
}

func cloneEvalPipelineExecutablePlan(in EvalPipelineExecutablePlan) EvalPipelineExecutablePlan {
	return in.clone()
}

func cloneQScriptStatement(stmt qScriptStatement) qScriptStatement {
	stmt.bindingPlan = cloneQScriptBindingPlan(stmt.bindingPlan)
	stmt.fastPlan = cloneQEvalFastPlan(stmt.fastPlan)
	if stmt.compareIndexPlan != nil {
		plan := cloneQPipelinePlan(*stmt.compareIndexPlan)
		stmt.compareIndexPlan = &plan
	}
	return stmt
}

func cloneQAddChainTermPlan(in qAddChainTermPlan) qAddChainTermPlan {
	out := in
	if in.fast != nil {
		cloned := cloneQEvalFastPlan(*in.fast)
		out.fast = &cloned
	}
	if in.monadBundle != nil {
		bundle := &qMonadSumBundlePlan{
			ops:    append([]string(nil), in.monadBundle.ops...),
			span:   in.monadBundle.span,
			terms:  make([]qAddChainTermPlan, len(in.monadBundle.terms)),
			kernel: in.monadBundle.kernel, // immutable after build; shareable
		}
		for i := range in.monadBundle.terms {
			bundle.terms[i] = cloneQAddChainTermPlan(in.monadBundle.terms[i])
		}
		out.monadBundle = bundle
	}
	return out
}

func cloneQEvalFastPlan(in qEvalFastPlan) qEvalFastPlan {
	out := in
	out.pipeline = cloneQPipelinePlan(in.pipeline)
	if in.fby != nil {
		fby := *in.fby
		out.fby = &fby
	}
	if len(in.addChainTerms) > 0 {
		out.addChainTerms = make([]qAddChainTermPlan, len(in.addChainTerms))
		for i := range in.addChainTerms {
			out.addChainTerms[i] = cloneQAddChainTermPlan(in.addChainTerms[i])
		}
	}
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
	if len(in.stringValues) > 0 {
		out.stringValues = append([]string(nil), in.stringValues...)
	}
	if in.whereIndexPlan != nil {
		plan := cloneQPipelinePlan(*in.whereIndexPlan)
		out.whereIndexPlan = &plan
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
	if len(in.castTerms) > 0 {
		out.castTerms = make([]qPipelineCastTermPlan, len(in.castTerms))
		for i := range in.castTerms {
			out.castTerms[i] = in.castTerms[i]
			out.castTerms[i].valuePlan = cloneQScriptBindingPlan(in.castTerms[i].valuePlan)
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
	// Nondeterministic roll/deal/rand must not enter cached binding plans.
	if qSourceHasRandomVerb(src) {
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
	// Nondeterministic roll/deal/rand must not enter the cached value-expr
	// (compiled statement) route.
	if qSourceHasRandomVerb(src) {
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
		case ",":
			// The Pratt tree groups `#` tighter and `!` looser than `,`,
			// while the string cascade gives the leftmost form the outermost
			// position (2#1 2,3 4 is 2#(1 2,3 4) there). Join statements stay
			// on the compiled route, which mirrors the cascade splits.
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
		// The Pratt tree gives `!` the loosest binding, so `1+2!3` parses
		// as (1+2)!3 here, while the string cascade follows q's leftmost-
		// verb-outermost rule (1+(2!3)). Dict keys carrying a dyadic verb
		// stay on the string cascade so the routes agree.
		if cachedValueExprContainsBinary(x.Keys) {
			return false
		}
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

// cachedValueExprContainsBinary reports whether the subtree carries any
// dyadic application (see the DictExpr eligibility comment).
func cachedValueExprContainsBinary(expr Expr) bool {
	switch x := expr.(type) {
	case Binary:
		return true
	case Call:
		return cachedValueExprContainsBinary(x.Arg)
	case Vector:
		for _, item := range x.Items {
			if cachedValueExprContainsBinary(item) {
				return true
			}
		}
	case DictExpr:
		return cachedValueExprContainsBinary(x.Keys) || cachedValueExprContainsBinary(x.Values)
	case IndexExpr:
		return cachedValueExprContainsBinary(x.Expr) || cachedValueExprContainsBinary(x.Index)
	}
	return false
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

func (s *EvalState) evalScriptStatement(stmt *qScriptStatement) (any, error) {
	if stmt.idxAssignName != "" {
		return s.evalIndexedAssignStatement(stmt)
	}
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
		if handled {
			recordQEvalDispatch(target, EvalDispatchDeferredScan)
		}
	}
	if !handled && stmt.assign != "" {
		if !stmt.compareIndexChecked {
			stmt.compareIndexChecked = true
			stmt.compareIndexPlan = buildQCompareIndexStatsPlan(target)
		}
		if stmt.compareIndexPlan != nil {
			v, handled, err = s.evalQCompareIndexStatsPlan(stmt.compareIndexPlan)
			if err != nil {
				return nil, err
			}
			if handled {
				recordQEvalDispatch(target, EvalDispatchCompareIndexPlan)
			}
		}
	}
	if !handled && stmt.assign != "" && s.assignPool.active {
		v, handled = s.tryEvalAssignPoolFillsFill(stmt, s.resolveAssignmentName(stmt.assign))
		if handled {
			recordQEvalDispatch(target, EvalDispatchAssignPool)
		}
	}
	if !handled {
		v, err = s.evalCachedOrString(target, stmt.valueExpr, &stmt.bindingPlan, &stmt.fastPlan)
	}
	if err != nil {
		return nil, err
	}
	if stmt.assign != "" {
		name := s.resolveAssignmentName(stmt.assign)
		if s.assignPool.active {
			v = s.assignPoolMaterialize(name, v)
		}
		s.ensureOwnedEnv()
		s.env[name] = v
	}
	return v, nil
}

// evalIndexedAssignStatement executes `name[i;j;...]: rhs`: the rhs and the
// index expressions evaluate first, the existing amend machinery produces an
// amended copy (the same kernels @[v;i;:;y] and .[m;path;:;y] use), and the
// name re-binds to that copy (assign-pool COW contract). The statement value
// is the assigned rhs, mirroring plain assignment.
func (s *EvalState) evalIndexedAssignStatement(stmt *qScriptStatement) (any, error) {
	value, err := s.evalCachedOrString(stmt.rhs, stmt.valueExpr, &stmt.bindingPlan, &stmt.fastPlan)
	if err != nil {
		return nil, err
	}
	target, ok := s.lookupName(stmt.idxAssignName)
	if !ok {
		return nil, fmt.Errorf("name %q is not defined", stmt.idxAssignName)
	}
	parts := splitQBracketFormArgs(stmt.idxAssignIndex)
	path := make([]any, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("indexed assignment %q has an empty index", stmt.src)
		}
		axis, err := s.eval(part)
		if err != nil {
			return nil, err
		}
		path[i] = axis
	}
	var amended any
	switch {
	case stmt.idxAssignOp != "":
		// Compound indexed amend `name[i]op: rhs` reuses the @[;;op;]
		// machinery with the dyadic verb as the amend function.
		fn, ok := lookupDyadicVerbFunc(stmt.idxAssignOp)
		if !ok {
			return nil, fmt.Errorf("indexed assignment operator %q is not a dyadic verb", stmt.idxAssignOp)
		}
		op := qDyadicFunction{name: stmt.idxAssignOp, fn: fn}
		if len(path) == 1 {
			amended, err = s.amendValueWithFunction(target, path[0], op, value)
		} else {
			amended, err = s.amendPath(target, path, op, value)
		}
	case len(path) == 1:
		amended, err = amendValue(target, path[0], value)
	default:
		amended, err = s.amendPath(target, path, nil, value)
	}
	if err != nil {
		return nil, err
	}
	name := s.resolveAssignmentName(stmt.idxAssignName)
	if s.assignPool.active {
		amended = s.assignPoolMaterialize(name, amended)
	}
	s.ensureOwnedEnv()
	s.env[name] = amended
	return value, nil
}

func (s *EvalState) tryEvalCompareIndexStatsAssignment(src string) (any, bool, error) {
	plan := buildQCompareIndexStatsPlan(src)
	if plan == nil {
		return nil, false, nil
	}
	return s.evalQCompareIndexStatsPlan(plan)
}

// buildQCompareIndexStatsPlan is the syntactic half of
// tryEvalCompareIndexStatsAssignment; the result can be cached per
// statement.
func buildQCompareIndexStatsPlan(src string) *qPipelinePlan {
	if !strings.HasPrefix(strings.TrimSpace(src), "where ") {
		return nil
	}
	plan, ok := buildQPipelineWhereComparePlan(src, qPipelineWhereCompareIndexes, "compare-to-index")
	if !ok || plan.kind != qPipelineWhereCompareIndexes {
		return nil
	}
	plan = qPipelinePlanWithBindingPlans(plan)
	return &plan
}

func (s *EvalState) evalQCompareIndexStatsPlan(plan *qPipelinePlan) (any, bool, error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return nil, true, err
	}
	desc, ok := qTypedWhereCompareIndexViewStatsDescriptor(left, right, plan.compareOp, "compare-to-index-view")
	if !ok {
		return nil, false, nil
	}
	if desc.array.Kind() != data.KindSymbol && desc.array.Kind() != data.KindString {
		return nil, false, nil
	}
	count, sum, handled, err := evalQTypedWhereCompareIndexStats(desc)
	if err != nil || !handled {
		return nil, handled, err
	}
	if count > int64(int(count)) {
		return nil, true, fmt.Errorf("where index count %d exceeds int range", count)
	}
	return qCompareIndexStatsView{source: desc.array, op: desc.op, scalar: desc.scalar, count: count, sum: sum}, true, nil
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
	if !statements[start].reductionOK {
		return nil, start, false, nil
	}
	first := statements[start].reduction
	sourceName := s.resolveAssignmentName(first.source)
	source, ok := s.lookupName(first.source)
	if !ok {
		return nil, start, false, nil
	}
	array, ok := source.(data.Array)
	if !ok {
		return nil, start, false, nil
	}
	next := start + 1
	for next < len(statements) {
		if !statements[next].reductionOK || s.resolveAssignmentName(statements[next].reduction.source) != sourceName {
			break
		}
		next++
	}
	if next-start < 2 {
		return nil, start, false, nil
	}
	bindings := statements[start:next]
	for i := range bindings {
		if s.resolveAssignmentName(bindings[i].reduction.assign) == sourceName {
			return nil, start, false, nil
		}
	}
	stats, handled, err := data.TryTypedNumericStats(array)
	recordRuntimeKernelProbe("ArrayNumericStats", "vector-reduce/bundle/"+string(array.Kind()), handled, err)
	if err != nil || !handled {
		return nil, start, handled, err
	}
	var last any
	for i := range bindings {
		value := numericReductionStatsValue(stats, bindings[i].reduction.op)
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(bindings[i].reduction.assign)] = value
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

// buildQEvalFastPlan builds the statement-level fast plan used by
// evalCachedOrStringUncached. The add-chain route belongs ONLY here: it
// mirrors the statement-level tryEvalScalarAddChain probe, which does not
// exist inside s.eval, so term-level plans (buildQEvalFastPlanTermRoutes)
// must not include it.
func buildQEvalFastPlan(src string) qEvalFastPlan {
	src = strings.TrimSpace(src)
	plan := buildQEvalFastPlanRoutes(src, true)
	if qMayBeApplyIndexForm(src) {
		plan.applyIndexState = qEvalSyntaxYes
	} else {
		plan.applyIndexState = qEvalSyntaxNo
	}
	return plan
}

// buildQEvalFastPlanTermRoutes builds a fast plan for a sub-expression whose
// per-call fallback is s.eval: only routes that are faithful to the s.eval
// probe cascade (pipeline, sort-rank, fby, reducer-over-name) are allowed.
func buildQEvalFastPlanTermRoutes(src string) qEvalFastPlan {
	return buildQEvalFastPlanRoutes(strings.TrimSpace(src), false)
}

func buildQEvalFastPlanRoutes(src string, statementLevel bool) qEvalFastPlan {
	if src == "" {
		return qEvalFastPlan{}
	}
	if scalar, ok := buildScalarApplyIndexPlan(src); ok {
		return qEvalFastPlan{kind: qEvalFastScalarApplyIndex, scalarIndex: scalar, constState: qEvalConstNot}
	}
	if name, sym, ok := buildNamePostfixSymbolPlan(src); ok {
		return qEvalFastPlan{kind: qEvalFastNamePostfixSymbol, postfixName: name, postfixSymbol: sym}
	}
	if sortRankTerms := buildQSortRankReducerBundlePlan(src); len(sortRankTerms) > 0 {
		out := qEvalFastPlan{kind: qEvalFastSortRankReducerBundle, sortRankTerms: sortRankTerms}
		if static, ok := qStaticSortRankReducerBundle(sortRankTerms); ok {
			out.sortRankStaticResult = static
			out.hasSortRankStaticResult = true
		}
		return out
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
	if fby := buildQFbyFastPlan(src); fby != nil {
		return qEvalFastPlan{kind: qEvalFastFby, fby: fby}
	}
	if statementLevel {
		if terms := buildQAddChainPlan(src); terms != nil {
			return qEvalFastPlan{kind: qEvalFastAddChain, addChainTerms: terms}
		}
	}
	if term, ok := buildQSingleTermFastPlan(src); ok {
		return qEvalFastPlan{kind: qEvalFastSingleTerm, addChainTerms: []qAddChainTermPlan{term}}
	}
	return qEvalFastPlan{}
}

// qMayBeApplyIndexForm is the exact syntactic gate of evalApplyIndexForm:
// when false, no branch of the per-call probe can handle src.
func qMayBeApplyIndexForm(src string) bool {
	if strings.HasPrefix(src, "@[") && strings.HasSuffix(src, "]") {
		return true
	}
	if strings.HasPrefix(src, ".[") && strings.HasSuffix(src, "]") {
		return true
	}
	if _, _, ok := splitTopLevelOperator(src, "@"); ok {
		return true
	}
	if _, _, ok := splitTopLevelDotApply(src); ok {
		return true
	}
	return false
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
	case qEvalFastSortRankReducerBundle:
		if plan.hasSortRankStaticResult {
			recordRuntimeKernelProbeReason("SortRankReducerBundle", "sort-rank-reducer-bundle/"+strconv.Itoa(len(plan.sortRankTerms))+"/static", true, nil, RuntimeFallbackUnsupportedType)
			return plan.sortRankStaticResult, true, nil
		}
		return s.evalSortRankReducerBundlePlan(plan.sortRankTerms)
	case qEvalFastFby:
		return s.evalQFbyFastPlan(plan.fby)
	case qEvalFastAddChain:
		return s.evalQAddChainPlan(plan.addChainTerms)
	case qEvalFastSingleTerm:
		return s.evalQSingleTermFastPlan(&plan.addChainTerms[0])
	case qEvalFastNamePostfixSymbol:
		collection, ok := s.lookupName(plan.postfixName)
		if !ok || isCallable(collection) {
			return nil, false, nil
		}
		out, err := indexValue(collection, plan.postfixSymbol)
		return out, true, err
	default:
		return nil, false, nil
	}
}

// buildNamePostfixSymbolPlan recognizes `name`sym` column/key reads (t`px,
// d`a): a bare non-verb identifier indexed by one symbol literal. Execution
// is a name lookup plus indexValue, skipping the per-call string walk.
func buildNamePostfixSymbolPlan(src string) (string, data.Symbol, bool) {
	collectionExpr, symbolExpr, ok := findPostfixSymbolLookup(src)
	if !ok {
		return "", "", false
	}
	name := strings.TrimSpace(collectionExpr)
	if !isQAssignmentName(name) {
		return "", "", false
	}
	if _, ok := qEvalConstantWords[name]; ok {
		return "", "", false
	}
	if _, ok := lookupUnaryVerb(name); ok {
		return "", "", false
	}
	if _, ok := lookupDyadicVerbFunc(name); ok {
		return "", "", false
	}
	syms, err := parseSymbolList(strings.TrimSpace(symbolExpr))
	if err != nil || len(syms) != 1 {
		return "", "", false
	}
	return name, syms[0], true
}

func (s *EvalState) evalCachedOrString(src string, expr Expr, bindingPlan *qScriptBindingPlan, fastPlan *qEvalFastPlan) (any, error) {
	if s.oneShot {
		return s.evalCachedOrStringUncached(src, expr, bindingPlan, fastPlan)
	}
	// Constant statement memoization: closed expressions (no free names, no
	// nondeterministic forms) evaluate to the same value on every call, so
	// the session-warm steady state is a single map probe. Values memoize
	// per EvalState only — plans are shared through the global script-plan
	// cache and fresh sessions must still execute (and record) kernels.
	// Amend forms rebuild arrays copy-on-write, matching the binding-plan
	// literal cache contract.
	if fastPlan != nil {
		switch fastPlan.constState {
		case qEvalConstValue:
			if value, ok := s.constValueCache[src]; ok {
				recordQEvalDispatch(src, EvalDispatchConstMemo)
				return value, nil
			}
		case qEvalConstUnknown:
			if qEvalConstantStatementSource(src) {
				fastPlan.constState = qEvalConstValue
			} else {
				fastPlan.constState = qEvalConstNot
			}
		}
		if fastPlan.constState == qEvalConstValue {
			value, err := s.evalCachedOrStringUncached(src, expr, bindingPlan, fastPlan)
			if err == nil && !s.oneShot {
				if s.constValueCache == nil {
					s.constValueCache = make(map[string]any, 8)
				} else if len(s.constValueCache) >= 256 {
					s.constValueCache = make(map[string]any, 8)
				}
				s.constValueCache[src] = value
			}
			return value, err
		}
	}
	return s.evalCachedOrStringUncached(src, expr, bindingPlan, fastPlan)
}

func (s *EvalState) evalCachedOrStringUncached(src string, expr Expr, bindingPlan *qScriptBindingPlan, fastPlan *qEvalFastPlan) (any, error) {
	if fastPlan == nil || fastPlan.applyIndexState != qEvalSyntaxNo {
		// Scalar apply plans execute the exact code tryEvalScalarApplyIndexFastPath
		// (the first apply-walk branch) runs, minus the per-call plan-cache
		// probe; declines fall into the same walk unchanged.
		if fastPlan != nil && fastPlan.kind == qEvalFastScalarApplyIndex {
			if out, handled, err := s.evalQFastPlan(fastPlan); err != nil || handled {
				recordQEvalDispatch(src, EvalDispatchFastPlan)
				return out, err
			}
		}
		if out, handled, err := s.evalQApplyFormPlan(src); err != nil || handled {
			recordQEvalDispatch(src, EvalDispatchApplyIndexPlan)
			return out, err
		}
		if out, handled, err := s.evalApplyIndexForm(src); err != nil || handled {
			recordQEvalDispatch(src, EvalDispatchApplyIndexString)
			return out, err
		}
	}
	if bindingPlan != nil && bindingPlan.kind != qScriptBindingInvalid {
		value, handled, err := s.evalQScriptBindingPlan(bindingPlan)
		if err != nil {
			return nil, err
		}
		if handled {
			recordQEvalDispatch(src, EvalDispatchBindingPlan)
			return value, nil
		}
	}
	if out, handled, err := s.evalQFastPlan(fastPlan); err != nil || handled {
		recordQEvalDispatch(src, EvalDispatchFastPlan)
		return out, err
	}
	if plan := s.qPipelinePlanRef(src); plan.kind != qPipelineInvalid {
		if out, handled, err := s.evalQPipelinePlan(plan); err != nil || handled {
			recordQEvalDispatch(src, EvalDispatchPipelinePlan)
			return out, err
		}
	}
	if out, handled, err := s.tryEvalWhereCompareCountSum(src); err != nil || handled {
		recordQEvalDispatch(src, EvalDispatchWhereCompareSum)
		return out, err
	}
	if out, handled, err := s.tryEvalScalarAddChain(src); err != nil || handled {
		recordQEvalDispatch(src, EvalDispatchScalarAddChain)
		return out, err
	}
	if expr != nil {
		value, err := s.evalValueExpr(expr)
		if err == nil {
			recordQEvalDispatch(src, EvalDispatchValueExpr)
			return value, nil
		}
	}
	// Statement compilation: the default warm path for statements no plan
	// claimed. The compiled tree mirrors the string evaluator's dispatch
	// (same split order, same verb functions), so compiled evaluation is
	// value- and error-identical; only unsupported-expression errors (for
	// example unbound names that the string cascade resolves further) fall
	// back to the string evaluator.
	if compiled := compiledQStatementExprCached(src); compiled != nil {
		value, err := s.evalValueExpr(compiled)
		if err == nil {
			recordQEvalDispatch(src, EvalDispatchCompiledExpr)
			return value, nil
		}
		if !isUnsupportedEvalValueExpr(err) {
			recordQEvalDispatch(src, EvalDispatchCompiledExpr)
			return nil, err
		}
	}
	recordQEvalDispatch(src, EvalDispatchStringEval)
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
	if src[0] == '?' && len(args) >= 4 {
		// Functional qSQL select/exec ?[t;c;b;a] (functional_query.go).
		out, err := s.evalFunctionalQueryForm(args)
		return out, true, err
	}
	if src[0] == '$' && len(args) > 3 {
		// Canonical multi-branch $[c1;t1;c2;t2;...;f]: odd argument counts
		// chain cond/then pairs lazily with a trailing else.
		if len(args)%2 == 0 {
			return nil, true, fmt.Errorf("$[] conditional expects an odd number of arguments")
		}
		i := 0
		for ; i+1 < len(args); i += 2 {
			cond, err := s.eval(args[i])
			if err != nil {
				return nil, true, err
			}
			truth, err := boolValue(cond)
			if err != nil {
				return nil, true, err
			}
			if truth {
				out, err := s.eval(args[i+1])
				return out, true, err
			}
		}
		out, err := s.eval(args[i])
		return out, true, err
	}
	if len(args) != 3 {
		return nil, true, fmt.Errorf("%c[] conditional expects three arguments", src[0])
	}
	cond, err := s.eval(args[0])
	if err != nil {
		return nil, true, err
	}
	if src[0] == '?' {
		// Vector conditional ?[v;t;f]: a boolean-vector condition selects
		// elementwise (canonical q), disambiguated from roll/deal (atom
		// left, dyadic) and functional select (4+ arguments, table arg0).
		if condArray, ok := cond.(data.Array); ok && condArray.Kind() == data.KindBool {
			out, err := s.evalVectorConditional(condArray, args[1], args[2])
			return out, true, err
		}
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

// evalVectorConditional implements canonical ?[v;t;f] with a boolean vector
// condition: elementwise selection with atom broadcast. Both branches
// evaluate eagerly (the canonical vector conditional is not lazy).
func (s *EvalState) evalVectorConditional(cond data.Array, trueSrc, falseSrc string) (any, error) {
	trueValue, err := s.eval(trueSrc)
	if err != nil {
		return nil, err
	}
	falseValue, err := s.eval(falseSrc)
	if err != nil {
		return nil, err
	}
	n := cond.Len()
	itemAt := func(v any, i int) (any, error) {
		array, ok := v.(data.Array)
		if !ok {
			return v, nil
		}
		if array.Len() != n {
			return nil, fmt.Errorf("?[] vector conditional operands must conform to the condition length")
		}
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("?[] vector conditional row %d out of range", i)
		}
		return item, nil
	}
	out := make([]any, n)
	for i := 0; i < n; i++ {
		flag, ok := cond.At(i)
		if !ok {
			return nil, fmt.Errorf("?[] vector conditional row %d out of range", i)
		}
		truth, err := boolValue(flag)
		if err != nil {
			return nil, err
		}
		branch := falseValue
		if truth {
			branch = trueValue
		}
		item, err := itemAt(branch, i)
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	return inferQArray(out), nil
}

func (s *EvalState) evalControlSpecialForm(src string) (any, bool, error) {
	name, inner, ok := parseNamedBracketForm(src)
	if !ok {
		return nil, false, nil
	}
	switch name {
	case "if":
		args := splitQBracketFormArgs(inner)
		body, ok := qControlBodyScript(args)
		if !ok {
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
		out, err := s.evalScript(body)
		return out, true, err
	case "do":
		args := splitQBracketFormArgs(inner)
		body, ok := qControlBodyScript(args)
		if !ok {
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
			out, err = s.evalScript(body)
			if err != nil {
				return nil, true, err
			}
		}
		return out, true, nil
	case "while":
		args := splitQBracketFormArgs(inner)
		body, ok := qControlBodyScript(args)
		if !ok {
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
			out, err = s.evalScript(body)
			if err != nil {
				return nil, true, err
			}
		}
	default:
		return nil, false, nil
	}
}

func qControlBodyScript(args []string) (string, bool) {
	if len(args) < 2 {
		return "", false
	}
	return strings.Join(args[1:], ";"), true
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

func (s *EvalState) ensureOwnedEnv() {
	if !s.envBorrowed {
		if s.env == nil {
			s.env = make(map[string]any)
		}
		return
	}
	s.env = cloneEnv(s.env)
	s.envBorrowed = false
}

func (s *EvalState) resetBorrowedEnv(env map[string]any) {
	*s = EvalState{env: env, envBorrowed: true, namespace: ".", oneShot: true}
}

func (s *EvalState) clearBorrowedEnv() {
	s.env = nil
	s.envBorrowed = false
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
	switch src {
	case "<=", ">=", "<>":
		// Composite comparison atoms: the infix forms ride
		// applyCompositeDyadic, and the bare spellings would otherwise be
		// split as malformed infix expressions. Needed so canonical
		// functional parse trees like (<=;`a;2) construct naturally.
		op := src
		return qDyadicFunction{name: op, fn: func(left, right any) (any, error) {
			return applyCompositeDyadic(op, left, right)
		}}, nil
	}
	if src[0] == '\'' {
		// Signal form 'x (trap_signal.go). Unclaimed '-prefixed statements
		// keep their pre-existing cascade errors.
		if handled, err := s.evalSignalForm(src); handled {
			return nil, err
		}
	}
	if out, ok, err := s.evalConditionalSpecialForm(src); ok || err != nil {
		return out, err
	}
	if out, ok, err := s.evalControlSpecialForm(src); ok || err != nil {
		return out, err
	}
	if out, ok, err := evalConsoleWriteForm(src); ok || err != nil {
		return out, err
	}
	if out, ok, err := s.evalFunctionalAmendForm(src); ok || err != nil {
		return out, err
	}
	if out, handled, err := s.tryEvalSortRankReducerBundle(src); err != nil || handled {
		return out, err
	}
	if plan := s.qPipelinePlanRef(src); plan.kind != qPipelineInvalid {
		if out, handled, err := s.evalQPipelinePlan(plan); err != nil || handled {
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
	if out, handled, err := s.evalNamedInsertUpsert(src); handled || err != nil {
		return out, err
	}
	if strings.HasPrefix(src, "@[") {
		if value, ok, err := s.evalApplyIndexForm(src); ok || err != nil {
			return value, err
		}
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
	if out, handled, err := s.evalWordAdverbInfix(src); handled || err != nil {
		return out, err
	}
	if out, handled, err := s.evalNamedAdverbBracketCall(src); handled || err != nil {
		return out, err
	}
	if out, handled, err := s.evalParenDerivedVerbJuxtaposed(src); handled || err != nil {
		return out, err
	}
	if funcs, ok := parseUnaryComposition(src); ok {
		return qComposition{funcs: s.bindStatefulUnaryFns(funcs)}, nil
	}
	if callableExpr, adverb, rightExpr, ok := s.findCallablePostfixAdverb(src); ok {
		callable, err := s.eval(callableExpr)
		if err != nil || !isCallable(callable) {
			// `seed f/ x` (do/while-iterate or seeded fold): the text left of
			// the adverb is a seed expression followed by a callable term.
			if out, handled, err2 := s.tryEvalSeededAdverbInfix(callableExpr, adverb, rightExpr); handled || err2 != nil {
				return out, err2
			}
			if err != nil {
				return nil, err
			}
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
		// Multi-axis data indexing m[i;j;...] walks one axis at a time,
		// matching `.` dot-apply path semantics (t[0;`a] is the cell,
		// m[1;0] the matrix element). Elided axes keep the prior cascade.
		if parts := splitTopLevelDelim(indexExpr, ';'); len(parts) > 1 && !qHasEmptyPart(parts) {
			current := collection
			for _, part := range parts {
				axis, err := s.eval(part)
				if err != nil {
					return nil, err
				}
				next, err := indexValue(current, axis)
				if err != nil {
					return nil, err
				}
				current = next
			}
			return current, nil
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
		// Callable left (verb name or bound lambda) is application, not
		// indexing — `med `sym` applies med, mirroring the compiled route's
		// IndexExpr callable dispatch.
		if isCallable(collection) {
			return s.applyCallable(collection, []any{key})
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
	if strings.HasPrefix(src, "rand ") {
		right, err := s.eval(strings.TrimSpace(src[len("rand "):]))
		if err != nil {
			return nil, err
		}
		return s.evalRand(right)
	}
	if strings.HasPrefix(src, "hopen ") {
		return s.evalHopen(strings.TrimSpace(src[len("hopen "):]))
	}
	if strings.HasPrefix(src, "+/") && !strings.HasPrefix(src, "+//") {
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
		if out, handled, err := s.tryEvalRankSum(right); err != nil || handled {
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
		if out, handled, err := s.tryEvalFindSum(right); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCastChainSum(right); err != nil || handled {
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
	if fn, ok := s.unaryVerbFunc(src); ok {
		return qUnaryFunction{name: strings.TrimSpace(src), fn: fn}, nil
	}
	if expr, ok := findAdverb(src); ok {
		return s.evalAdverb(expr)
	}
	// `seed name/ x` iterate with a session-bound callable name (the verb
	// registry routes above handle builtin verbs).
	if leftSrc, adverb, rightSrc, ok := findSeededIterateInfix(src); ok {
		if out, handled, err := s.tryEvalSeededAdverbInfix(leftSrc, adverb, rightSrc); handled || err != nil {
			return out, err
		}
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
		return enlist(v)
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
		// Length-preserving transform chains (count deltas fills x) resolve to
		// the base vector's length without materializing any transform, so
		// they run before the sequence primitives that evaluate the operand.
		if out, handled, err := s.tryEvalCountLengthPreservingTransform(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountSequencePrimitive(strings.TrimSpace(src[len("count "):])); err != nil || handled {
			return out, err
		}
		if out, handled, err := s.tryEvalCountReverse(strings.TrimSpace(src[len("count "):])); err != nil || handled {
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
	if out, handled, err := s.tryEvalSortedEdge(src); err != nil || handled {
		return out, err
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
		{"key ", keyVerb},
		{"value ", s.valueVerb},
		{"eval ", s.evalTreeVerb},
		{"parse ", qParseVerb},
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
	// `,` join: claimed before the loose-binding dyadic probes (~ ? word
	// verbs # _ $ !) whenever the comma is the LEFTMOST top-level dyadic
	// form, so canonical right-to-left grouping holds (see join.go).
	if idx, ok := qTopLevelJoinSplit(src); ok {
		if idx == 0 {
			// Prefix `,x` is enlist (same shape as the `enlist ` branch).
			right, err := s.eval(strings.TrimSpace(src[idx+1:]))
			if err != nil {
				return nil, err
			}
			return enlist(right)
		}
		left, err := s.eval(strings.TrimSpace(src[:idx]))
		if err != nil {
			return nil, err
		}
		right, err := s.eval(strings.TrimSpace(src[idx+1:]))
		if err != nil {
			return nil, err
		}
		return joinValue(left, right)
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
		// Integer-atom LHS is canonical q roll/deal, not find.
		if n, ok := qRollDealCount(left); ok {
			return s.rollOrDeal(n, right)
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
	if op, leftExpr, rightExpr, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps); ok {
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
	if op, leftExpr, rightExpr, ok := splitTopLevelDyadicWordMap(src, qSetWordOps); ok {
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
	if hash := findTopLevel(src, "#"); hash >= 0 {
		if marker, ok := parseAttributeMarker(strings.TrimSpace(src[:hash])); ok {
			v, err := s.eval(strings.TrimSpace(src[hash+1:]))
			if err != nil {
				return nil, err
			}
			return attributeVector(marker, v)
		}
		// Left operand first: every other dyadic branch (and the compiled
		// route's Binary execution) evaluates left before right, so the
		// surfaced error must be the left one (`0$00#A` reports the cast
		// error, not the unbound name).
		leftValue, err := s.eval(strings.TrimSpace(src[:hash]))
		if err != nil {
			return nil, err
		}
		v, err := s.eval(strings.TrimSpace(src[hash+1:]))
		if err != nil {
			return nil, err
		}
		return takeOrReshapeValue(leftValue, v)
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
	if bang := findTopLevel(src, "!"); bang >= 0 && !qBangYieldsToLeftmostDyadic(src, bang) {
		keys, err := s.eval(strings.TrimSpace(src[:bang]))
		if err != nil {
			return nil, err
		}
		values, err := s.eval(strings.TrimSpace(src[bang+1:]))
		if err != nil {
			return nil, err
		}
		return qBangValue(keys, values)
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
		if isCallable(left) {
			return s.applyCallable(left, []any{right})
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

// evalJuxtaposedIndexVector applies canonical q juxtaposition when a bound
// container is followed by numeric literals: `x:10 20 30;x 1` indexes x
// (-> 20, same as x[1]) and `"hello" 1` indexes the string (-> "e") instead
// of building a generic list. Only Ident/String heads with all-Number tails
// are claimed; every other juxtaposed shape keeps its historic semantics.
func (s *EvalState) evalJuxtaposedIndexVector(x Vector) (any, bool, error) {
	if !qJuxtaposedIndexVectorShape(x) {
		return nil, false, nil
	}
	values := make([]any, len(x.Items))
	for i, item := range x.Items {
		value, err := s.evalValueExpr(item)
		if err != nil {
			return nil, false, nil
		}
		values[i] = value
	}
	if out, handled, err := qJuxtaposedIndexVectorValue(values); handled || err != nil {
		return out, handled, err
	}
	// A callable head juxtaposed onto literals is application, never list
	// construction: `f 2 3` applies f to the vector 2 3, and a word verb
	// head (`msum 2 1 3`) surfaces the verb's own arity error instead of
	// silently building a list containing the function value.
	if isCallable(values[0]) {
		arg, err := qJuxtaposedTailArgument(values)
		if err != nil {
			return nil, true, err
		}
		out, err := s.applyCallable(values[0], []any{arg})
		return out, true, err
	}
	return nil, false, nil
}

// qJuxtaposedTailArgument materializes the literal tail of a juxtaposed
// application as the single argument value (atom for one item, vector
// otherwise).
func qJuxtaposedTailArgument(values []any) (any, error) {
	if len(values) == 2 {
		return values[1], nil
	}
	return evalValueVector(values[1:])
}

// qJuxtaposedIndexVectorShape reports the static juxtaposed-index shape: an
// Ident or String head followed by Number literals only.
func qJuxtaposedIndexVectorShape(x Vector) bool {
	if len(x.Items) < 2 {
		return false
	}
	switch x.Items[0].(type) {
	case Ident, String:
	default:
		return false
	}
	for _, item := range x.Items[1:] {
		if _, ok := item.(Number); !ok {
			return false
		}
	}
	return true
}

// qJuxtaposedIndexVectorValue indexes values[0] with the numeric tail when
// the head is a container; indexed=false defers to list construction.
func qJuxtaposedIndexVectorValue(values []any) (out any, indexed bool, err error) {
	switch values[0].(type) {
	case data.Array, string:
	default:
		return nil, false, nil
	}
	var index any
	if len(values) == 2 {
		index = values[1]
	} else {
		vector, err := evalValueVector(values[1:])
		if err != nil {
			return nil, false, nil
		}
		index = vector
	}
	out, err = indexValue(values[0], index)
	return out, true, err
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
		// Dyadic before unary mirrors the string evaluator's bare-verb
		// resolution order for names that are both (min, max, wsum, where).
		if fn, ok := lookupDyadicVerbFunc(x.Name); ok {
			return qDyadicFunction{name: x.Name, fn: fn}, nil
		}
		if fn, ok := s.unaryVerbFunc(x.Name); ok {
			return qUnaryFunction{name: x.Name, fn: fn}, nil
		}
		return nil, unsupportedEvalValueExpr{expr: expr}
	case Const:
		return x.Value, nil
	case ListExpr:
		values := make([]any, len(x.Items))
		for i, item := range x.Items {
			value, err := s.evalValueExpr(item)
			if err != nil {
				return nil, err
			}
			values[i] = value
		}
		if x.Bare {
			return data.NewAny(values), nil
		}
		return inferQArray(values), nil
	case CastExpr:
		var domain any
		if x.Domain == nil {
			domain = qSymbolCastTarget()
		} else {
			value, err := s.evalValueExpr(x.Domain)
			if err == nil {
				domain = value
			} else if x.BareSym != "" {
				domain = x.BareSym
			} else {
				return nil, err
			}
		}
		values, err := s.evalValueExpr(x.Value)
		if err != nil {
			return nil, err
		}
		return castOrEnum(domain, values)
	case DyadicWordExpr:
		ops := qDyadicWordOps
		if x.Set {
			ops = qSetWordOps
		}
		op, ok := ops[x.Word]
		if !ok {
			return nil, unsupportedEvalValueExpr{expr: expr}
		}
		left, err := s.evalValueExpr(x.Left)
		if err != nil {
			return nil, err
		}
		right, err := s.evalValueExpr(x.Right)
		if err != nil {
			return nil, err
		}
		return op.fn(left, right)
	case NameVectorExpr:
		values := make([]any, len(x.Names))
		for i, name := range x.Names {
			value, ok := s.lookupName(name)
			if !ok {
				return nil, unsupportedEvalValueExpr{expr: expr}
			}
			values[i] = value
		}
		return evalValueVector(values)
	case Vector:
		if out, handled, err := s.evalJuxtaposedIndexVector(x); handled || err != nil {
			return out, err
		}
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
		return qBangValue(keys, values)
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
	case SafeCall:
		return s.evalValueCall(Call{Func: x.Func, Arg: x.Arg})
	case FusedCountWhere:
		return s.evalFusedCountWhere(x)
	case FusedCountLen:
		return s.evalFusedCountLen(x)
	case FusedSumUnary:
		return s.evalFusedSumUnary(x)
	case FusedSumFind:
		return s.evalFusedSumFind(x)
	case FusedSumCastChain:
		return s.evalFusedSumCastChain(x)
	case ApplyAtExpr:
		return s.evalApplyAtExpr(x)
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
	// til/enlist/flip mirror the string evaluator's dedicated prefix
	// branches; they are not unary-verb registry entries.
	switch expr.Func {
	case "til":
		n, ok := arg.(int64)
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
	case "enlist":
		return enlist(arg)
	case "flip":
		return flip(arg)
	case "value":
		// Symbol dereference is a read-only env probe and stays on the
		// compiled route; string and parse-tree evaluation are stateful
		// (arbitrary source against the live session) and decline so the
		// string evaluator is the single execution route — the same pattern
		// roll/deal use to keep the dual-route differential side-effect free.
		switch x := arg.(type) {
		case data.Symbol:
			return s.valueSymbolDeref(x)
		case string:
			return nil, unsupportedEvalValueExpr{expr: expr}
		case data.Array:
			if qIsParseTreeList(x) {
				return nil, unsupportedEvalValueExpr{expr: expr}
			}
		}
	case "eval":
		// Parse-tree evaluation can reach value-of-string through a tree;
		// session-route only (see the value case above).
		return nil, unsupportedEvalValueExpr{expr: expr}
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
	case "_":
		return cutOrDrop(left, right)
	case "?":
		if _, ok := qRollDealCount(left); ok {
			// Roll/deal is nondeterministic and needs session PRNG state;
			// decline the compiled route so the string evaluator (the only
			// execution) draws exactly once.
			return nil, unsupportedEvalValueExpr{expr: Binary{Op: op}}
		}
		return findValue(left, right)
	case "#":
		return takeOrReshapeValue(left, right)
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
		if x == 0 {
			// IEEE 754: 0 - (±0) is +0, while -x of +0 would be -0.
			return float32(0), true
		}
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
	p := parser{tokens: tokens, commaJoin: true}
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

// NormalizeRawSourceStatements turns top-level newlines in q raw source blocks
// into semicolon statement separators. It intentionally lives outside the
// normal q.eval string path so historical string semantics remain unchanged.
func NormalizeRawSourceStatements(src string) string {
	var b strings.Builder
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	var lastSignificant byte
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			b.WriteByte(ch)
			if ch == '\\' && i+1 < len(src) {
				i++
				b.WriteByte(src[i])
				continue
			}
			if ch == '"' {
				inString = false
				lastSignificant = ch
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
			b.WriteByte(ch)
			lastSignificant = ch
		case '`':
			end := qSymbolLiteralEnd(src, i)
			b.WriteString(src[i:end])
			lastSignificant = '`'
			i = end - 1
		case '(':
			parenDepth++
			b.WriteByte(ch)
			lastSignificant = ch
		case ')':
			parenDepth--
			b.WriteByte(ch)
			lastSignificant = ch
		case '[':
			bracketDepth++
			b.WriteByte(ch)
			lastSignificant = ch
		case ']':
			bracketDepth--
			b.WriteByte(ch)
			lastSignificant = ch
		case '{':
			braceDepth++
			b.WriteByte(ch)
			lastSignificant = ch
		case '}':
			braceDepth--
			b.WriteByte(ch)
			lastSignificant = ch
		case '\r':
			continue
		case '\n':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				if lastSignificant != 0 && lastSignificant != ';' {
					b.WriteByte(';')
					lastSignificant = ';'
				}
				continue
			}
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
			if ch != ' ' && ch != '\t' && ch != '\v' && ch != '\f' {
				lastSignificant = ch
			}
		}
	}
	return strings.TrimRight(strings.TrimRight(b.String(), " \t\r\n"), ";")
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

func qHasEmptyPart(parts []string) bool {
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return true
		}
	}
	return false
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

// splitTopLevelIndexedAssignment recognizes `name[i;j;...]: rhs` and the
// compound spelling `name[i;j;...]op: rhs` (op one of + - * % ^ & |): a
// bound name followed by exactly one bracket group spanning the rest of the
// assignment target. Returns the name, the bracket interior (index
// expressions separated by `;`), the compound operator ("" for plain
// assignment), and the rhs source.
func splitTopLevelIndexedAssignment(src string) (string, string, string, string, bool) {
	colon := findTopLevel(src, ":")
	if colon <= 0 {
		return "", "", "", "", false
	}
	left := strings.TrimSpace(src[:colon])
	op := ""
	if len(left) > 1 && strings.Contains("+-*%^&|", left[len(left)-1:]) {
		op = left[len(left)-1:]
		left = strings.TrimSpace(left[:len(left)-1])
	}
	if !strings.HasSuffix(left, "]") {
		return "", "", "", "", false
	}
	open := strings.IndexByte(left, '[')
	if open <= 0 {
		return "", "", "", "", false
	}
	name := strings.TrimSpace(left[:open])
	if !isQAssignmentName(name) {
		return "", "", "", "", false
	}
	if findMatchingDelimiter(left, open, '[', ']') != len(left)-1 {
		return "", "", "", "", false
	}
	indexSrc := strings.TrimSpace(left[open+1 : len(left)-1])
	if indexSrc == "" {
		return "", "", "", "", false
	}
	rhs := strings.TrimSpace(src[colon+1:])
	return name, indexSrc, op, rhs, rhs != ""
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

type qDyadicWordOp struct {
	word string
	fn   func(any, any) (any, error)
}

func splitTopLevelWord(src, word string) (string, string, bool) {
	pos := findTopLevelWord(src, word)
	if pos < 0 {
		return "", "", false
	}
	left := strings.TrimSpace(src[:pos])
	right := strings.TrimSpace(src[pos+len(word):])
	return left, right, left != "" && right != ""
}

// qDyadicWordOps and qSetWordOps are the word-form dyadic verb registries the
// string evaluator splits on. Built once: the per-call probe used to rebuild
// these tables and run one full source scan per word; the map variant scans
// the source once and looks each top-level identifier token up directly.
var qDyadicWordOps = map[string]qDyadicWordOp{}
var qSetWordOps = map[string]qDyadicWordOp{}

func init() {
	for _, op := range []qDyadicWordOp{
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
		{"where", whereFilterValue},
		{"in", membership},
		{"and", logicalAnd},
		{"or", logicalOr},
	} {
		qDyadicWordOps[op.word] = op
	}
	for _, op := range []qDyadicWordOp{
		{"intersect", inter},
		{"except", except},
		{"union", union},
		{"inter", inter},
	} {
		qSetWordOps[op.word] = op
	}
}

// splitTopLevelDyadicWordMap finds the leftmost top-level identifier token
// present in ops and splits src around it. Equivalent to probing
// findTopLevelWord once per registered word, but with a single scan.
func splitTopLevelDyadicWordMap(src string, ops map[string]qDyadicWordOp) (qDyadicWordOp, string, string, bool) {
	var zero qDyadicWordOp
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
			if !isQIdentRest(ch) {
				continue
			}
			j := i + 1
			for j < len(src) && isQIdentRest(src[j]) {
				j++
			}
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				if op, ok := ops[src[i:j]]; ok {
					// Leftmost registered word decides the split; an empty
					// side rejects the whole form, matching the per-word
					// probe variant below.
					left := strings.TrimSpace(src[:i])
					right := strings.TrimSpace(src[j:])
					if left == "" || right == "" {
						return zero, "", "", false
					}
					return op, left, right, true
				}
			}
			i = j - 1
		}
	}
	return zero, "", "", false
}

func splitTopLevelDyadicWord(src string, ops []qDyadicWordOp) (qDyadicWordOp, string, string, bool) {
	var zero qDyadicWordOp
	best := -1
	bestIndex := -1
	for i, op := range ops {
		pos := findTopLevelWord(src, op.word)
		if pos < 0 {
			continue
		}
		if best < 0 || pos < best || pos == best && len(op.word) > len(ops[bestIndex].word) {
			best = pos
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return zero, "", "", false
	}
	op := ops[bestIndex]
	left := strings.TrimSpace(src[:best])
	right := strings.TrimSpace(src[best+len(op.word):])
	if left == "" || right == "" {
		return zero, "", "", false
	}
	return op, left, right, true
}

func findTopLevelWord(src, word string) int {
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
				return i
			}
		}
	}
	return -1
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
	case "flip", "enlist", "keys", "key", "value", "eval", "parse", "cols", "meta", "attr", "type", "count",
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
	// Derived verbs (`+/`, `-':`, `,//`) must claim before the dyadic-infix
	// rejection: findDyadic sees the leading operator and would misread the
	// bracket call `+/[x]` as a malformed infix expression.
	if _, ok := parseDyadicAdverbFunction(collection); ok {
		return true
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
	for _, adverb := range []string{"':", "\\:", "/:", "'", "//", "/", "\\"} {
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
	case ' ', '\t', '\n', '\r', '(', ')', '[', ']', '{', '}', ';', ',', '!', '+', '-', '/', '=', '<', '>', '#', '$':
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
	if src[i] == ',' {
		// `,` is never a numeric sign: `,1` is the prefix enlist join form
		// and `x,1`/`x ,1` are joins.
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

// qBangYieldsToLeftmostDyadic reports whether the `!` claim must yield to a
// top-level symbol dyadic verb LEFT of the first top-level bang: under q's
// right-to-left grouping the leftmost verb is the outermost operation, so
// `d1+`b`c!10 20` is d1+(`b`c!10 20), not (d1+`b`c)!10 20. The `,` join keeps
// its own leftmost claim (qTopLevelJoinSplit) and yields to a bang left of
// the comma, matching the established cascade order.
func qBangYieldsToLeftmostDyadic(src string, bang int) bool {
	idx, _, ok := findDyadic(src)
	return ok && idx < bang
}

func findDyadic(src string) (int, byte, bool) {
	// Canonical kdb+/q: no precedence among the dyadic symbol verbs and strict
	// right-to-left evaluation. Splitting at the LEFTMOST top-level operator
	// makes the leftmost verb the outermost operation and the rest of the
	// expression its right argument (evaluated recursively, again leftmost),
	// which is exactly q's right-associative grouping. `2*3+1` splits at `*`
	// into 2*(3+1)=8; `10-4-2` splits at the first `-` into 10-(4-2)=8.
	// `,` (join) shares the single dyadic level: `1+2,3` is 1+(2,3) and
	// `1 2,3+4` is 1 2,(3+4). Comma-leftmost sources are claimed early by
	// qTopLevelJoinSplit; applyDyadic carries the ',' backstop.
	if idx := findTopLevel(src, "=<>+-*%&|^,"); idx >= 0 {
		return idx, src[idx], true
	}
	return 0, 0, false
}

// addChainHeadShadowsCanonical reports whether the head term of a top-level
// `+`-split chain itself contains a top-level dyadic verb to the LEFT of the
// first `+`. When it does, that verb — not the `+` — is the canonical leftmost
// (outermost) operation, so the left-associative add-chain accumulation would
// diverge from q's right-to-left grouping (`2 times 3+1` is 2 times (3+1), not
// (2 times 3)+1). Such chains must fall through to the leftmost-verb splitters.
func addChainHeadShadowsCanonical(head string) bool {
	if _, _, ok := findDyadic(head); ok {
		return true
	}
	if _, _, _, ok := splitTopLevelDyadicWordMap(head, qDyadicWordOps); ok {
		return true
	}
	return false
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
		case strings.HasPrefix(src[i:], "//"):
			// Composed over-of-over: `,//x` is converge of the raze fold.
			adverb = "//"
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
		if left != "" {
			// A top-level dyadic operator anywhere in the left part (`s$+\x`,
			// `1+*/x`, `()!count +/or`) is the outermost (leftmost) split
			// under canonical right-to-left grouping; the adverb belongs to
			// its RIGHT argument, so the adverb claim must yield to the
			// cascade's operator splits.
			if _, _, ok := findDyadic(left); ok {
				continue
			}
			if findTopLevel(left, "$#_!@?~") >= 0 {
				continue
			}
			// Same yield for a stranded dyadic WORD (`00 in+/max`,
			// `00000 except+/0`, glued `0:in+/0`): the word map's leftmost
			// split is the outermost operation and the derived verb is its
			// right argument. The trailing IDENT TOKEN decides (tokens, not
			// whitespace fields, are what the word map scans).
			if end := len(left); end > 0 {
				start := end
				for start > 0 && isQIdentRest(left[start-1]) {
					start--
				}
				if start > 0 && start < end {
					token := left[start:end]
					if _, ok := qDyadicWordOps[token]; ok {
						continue
					}
					if _, ok := qSetWordOps[token]; ok {
						continue
					}
				}
			}
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

// prefixVerbChain resolves a whitespace-separated run of bare builtin
// unary-verb words (none shadowed by a session binding) to their application
// functions. It mirrors the string evaluator's unary prefix branches: the
// dedicated til/flip/enlist forms plus the unary verb registry.
func (s *EvalState) prefixVerbChain(left string) ([]func(any) (any, error), bool) {
	fields := strings.Fields(strings.TrimSpace(left))
	if len(fields) == 0 {
		return nil, false
	}
	fns := make([]func(any) (any, error), 0, len(fields))
	for _, name := range fields {
		if !isQBareName(name) {
			return nil, false
		}
		if _, bound := s.lookupName(name); bound {
			return nil, false
		}
		switch name {
		case "til":
			fns = append(fns, qTilValue)
			continue
		case "flip":
			fns = append(fns, flip)
			continue
		case "enlist":
			fns = append(fns, enlist)
			continue
		}
		fn, ok := s.unaryVerbFunc(name)
		if !ok {
			return nil, false
		}
		fns = append(fns, fn)
	}
	return fns, true
}

// qTilValue is `til` applied to an already-evaluated operand (the same
// contract as the compiled route's evalValueCall til case).
func qTilValue(v any) (any, error) {
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
	return b == '+' || b == '-' || b == '*' || b == '%' || b == '=' || b == '<' || b == '>' || b == '&' || b == '|' || b == '^' || b == '~' || b == ','
}

func (s *EvalState) evalAdverb(expr adverbExpr) (any, error) {
	// A left operand made of bare builtin prefix-verb words is prefix
	// application to the derived-verb expression, not a seed: canonical q
	// evaluates right-to-left, so `til +/0` is til (+/0) and `count next/x`
	// is count (next/x). Session bindings shadow verb words (bound names are
	// nouns and stay seeds), mirroring the compiled route's prefix-word
	// compilation order.
	if expr.left != "" {
		if fns, ok := s.prefixVerbChain(expr.left); ok {
			out, err := s.evalAdverb(adverbExpr{verb: expr.verb, adverb: expr.adverb, right: expr.right})
			if err != nil {
				return nil, err
			}
			for i := len(fns) - 1; i >= 0; i-- {
				if out, err = fns[i](out); err != nil {
					return nil, err
				}
			}
			return out, nil
		}
	}
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
		if out, handled, err := s.tryEvalTypedIntegerDyadicSum(expr.right); err != nil || handled {
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
			return s.applyEachUnary(expr.verb, right)
		case "':":
			if op, _, ok := lookupDyadicVerb(expr.verb); ok {
				return applyEachPrior(op, nil, right)
			}
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
			if fn, ok := s.unaryVerbFunc(expr.verb); ok {
				// Monadic verb with over is q's converge iterator.
				return s.applyIterateOver(qUnaryFunction{name: expr.verb, fn: fn}, nil, right, false)
			}
			return nil, fmt.Errorf("%s cannot be used with over", expr.verb)
		case "\\":
			op, _, ok := lookupDyadicVerb(expr.verb)
			if !ok {
				if _, ok := lookupDyadicVerbFunc(expr.verb); ok {
					return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
				}
				if fn, ok := s.unaryVerbFunc(expr.verb); ok {
					// Monadic verb with scan is converge collecting intermediates.
					return s.applyIterateOver(qUnaryFunction{name: expr.verb, fn: fn}, nil, right, true)
				}
				return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
			}
			return applyScan(op, nil, right)
		case "//":
			// Composed over-of-over: f// is (f/)/ — the derived fold f/ is
			// monadic, so the outer over is q's converge iterator (`,//x`
			// razes nested lists to depth).
			op, _, ok := lookupDyadicVerb(expr.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with over-over", expr.verb)
			}
			return convergeOverOp(op, right)
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
		if expr.adverb == "/" || expr.adverb == "\\" {
			if unary, ok := s.unaryVerbFunc(expr.verb); ok {
				// A bare builtin verb word on the left is prefix application
				// to the derived verb (`count next/x` = count (next/x)), not
				// a seed: canonical q seeds are nouns or function literals.
				if leftName := strings.TrimSpace(expr.left); isQBareName(leftName) && qIsBuiltinVerbName(leftName) {
					if outer, ok := s.unaryVerbFunc(leftName); ok {
						iterated, err := s.applyIterateOver(qUnaryFunction{name: expr.verb, fn: unary}, nil, right, expr.adverb == "\\")
						if err != nil {
							return nil, err
						}
						return outer(iterated)
					}
				}
				// `n verb/ x` / `n verb\ x`: do-iterate a monadic verb.
				return s.applyIterateOver(qUnaryFunction{name: expr.verb, fn: unary}, left, right, expr.adverb == "\\")
			}
		}
		return nil, fmt.Errorf("%s cannot be used as a dyadic verb", expr.verb)
	}
	switch expr.adverb {
	case "'":
		if hasDyadicOp {
			return applyEachDyadic(op, left, right)
		}
		return applyEachDyadicFunc(fn, left, right)
	case "':":
		if hasDyadicOp {
			return applyEachPrior(op, left, right)
		}
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

func (s *EvalState) tryEvalTypedIntegerDyadicSum(src string) (any, bool, error) {
	leftExpr, dyadicOp, rightExpr, ok := splitTopLevelArithmeticOperator(src)
	if !ok || dyadicOp == '%' {
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
	if qPipelineFusedSumEmptyOperand(left, right) {
		return nil, false, nil
	}
	if !qTypedIntegerOperandOK(left) || !qTypedIntegerOperandOK(right) {
		return nil, false, nil
	}
	out, handled, err := data.TryTypedIntegerDyadicSum(data.Op(string(dyadicOp)), left, right)
	shape := "vector-reduce/sum-integer-dyadic-" + string(dyadicOp)
	if array, ok := left.(data.Array); ok {
		shape += "/left-" + string(array.Kind())
	}
	if array, ok := right.(data.Array); ok {
		shape += "/right-" + string(array.Kind())
	}
	out, handled, err = qTypedRuntimeResult("ArrayIntegerDyadicSum", shape, out, handled, err)
	if err != nil {
		return nil, true, fmt.Errorf("sum integer %s: %w", string(dyadicOp), err)
	}
	return out, handled, nil
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
	// `,/x` is canonical raze (join over); `,/:` (join each-right) is not.
	if strings.HasPrefix(src, ",/") && !strings.HasPrefix(src, ",/:") {
		if arg := strings.TrimSpace(src[len(",/"):]); arg != "" {
			return data.SequenceTransformRaze, nil, arg, true, nil
		}
	}
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
	// q's one-argument sublist clamps (no overtake); the data layer's
	// one-argument sublist step is take, so encode start/count instead.
	converted, ok := qSublistTransformArgs(indexes)
	if !ok {
		return "", nil, "", false, nil
	}
	return data.SequenceTransformSublist, converted, rightExpr, true, nil
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

func (s *EvalState) tryEvalRankSum(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "rank ") {
		return nil, false, nil
	}
	arg := strings.TrimSpace(src[len("rank "):])
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
	shape := "rank-sum/" + string(array.Kind())
	out, handled, err := data.TryTypedRankSumI64(array)
	out, handled, err = qTypedRuntimeResultReason("ArrayRankSum", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	if err != nil || handled {
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) tryEvalSortedEdge(src string) (any, bool, error) {
	edge := ""
	rest := ""
	switch {
	case strings.HasPrefix(src, "first ") && wordBoundary(src, 0, len("first")):
		edge = "first"
		rest = strings.TrimSpace(src[len("first "):])
	case strings.HasPrefix(src, "last ") && wordBoundary(src, 0, len("last")):
		edge = "last"
		rest = strings.TrimSpace(src[len("last "):])
	default:
		return nil, false, nil
	}
	descending := false
	arg := ""
	switch {
	case strings.HasPrefix(rest, "asc ") && wordBoundary(rest, 0, len("asc")):
		arg = strings.TrimSpace(rest[len("asc "):])
	case strings.HasPrefix(rest, "desc ") && wordBoundary(rest, 0, len("desc")):
		descending = true
		arg = strings.TrimSpace(rest[len("desc "):])
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
		return nil, false, nil
	}
	order := "asc"
	if descending {
		order = "desc"
	}
	shape := "sort-edge/" + string(array.Kind()) + "/" + order + "/" + edge
	out, handled, err := data.TryTypedSortedEdge(array, descending, edge == "last")
	out, handled, err = qTypedRuntimeResultReason("ArraySortedEdge", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	if err != nil || handled {
		return out, handled, err
	}
	return nil, false, nil
}

type qSortRankReducerTermKind uint8

const (
	qSortRankReducerTermInvalid qSortRankReducerTermKind = iota
	qSortRankReducerTermSortIndexSum
	qSortRankReducerTermRankSum
	qSortRankReducerTermSortedEdge
)

type qSortRankReducerTermPlan struct {
	kind            qSortRankReducerTermKind
	arg             string
	argExpr         Expr
	argValue        any
	hasArgValue     bool
	staticResult    int64
	hasStaticResult bool
	descending      bool
	last            bool
}

func (s *EvalState) tryEvalSortRankReducerBundle(src string) (any, bool, error) {
	terms := buildQSortRankReducerBundlePlan(src)
	if len(terms) < 2 {
		return nil, false, nil
	}
	return s.evalSortRankReducerBundlePlan(terms)
}

func (s *EvalState) evalSortRankReducerBundlePlan(terms []qSortRankReducerTermPlan) (any, bool, error) {
	if len(terms) < 2 {
		return nil, false, nil
	}
	var total int64
	for _, term := range terms {
		value, handled, err := s.evalSortRankReducerTermPlan(term)
		if err != nil {
			recordRuntimeKernelProbeReason("SortRankReducerBundle", "sort-rank-reducer-bundle/"+strconv.Itoa(len(terms)), false, err, RuntimeFallbackUnsupportedType)
			return nil, true, err
		}
		if !handled {
			return nil, false, nil
		}
		n, ok := integerValue(value)
		if !ok {
			err := fmt.Errorf("sort/rank reducer term returned non-integer %T", value)
			recordRuntimeKernelProbeReason("SortRankReducerBundle", "sort-rank-reducer-bundle/"+strconv.Itoa(len(terms)), false, err, RuntimeFallbackUnsupportedType)
			return nil, true, err
		}
		total += n
	}
	recordRuntimeKernelProbeReason("SortRankReducerBundle", "sort-rank-reducer-bundle/"+strconv.Itoa(len(terms)), true, nil, RuntimeFallbackUnsupportedType)
	return total, true, nil
}

func (s *EvalState) evalSortRankReducerTermPlan(term qSortRankReducerTermPlan) (any, bool, error) {
	if term.hasStaticResult {
		return term.staticResult, true, nil
	}
	switch term.kind {
	case qSortRankReducerTermSortIndexSum:
		return s.evalSortIndexSumPlan(term.arg, term.argExpr, term.argValue, term.hasArgValue, term.descending)
	case qSortRankReducerTermRankSum:
		return s.evalRankSumPlan(term.arg, term.argExpr, term.argValue, term.hasArgValue)
	case qSortRankReducerTermSortedEdge:
		return s.evalSortedEdgePlan(term.arg, term.argExpr, term.argValue, term.hasArgValue, term.descending, term.last)
	default:
		return nil, false, nil
	}
}

func (s *EvalState) evalSortIndexSumPlan(arg string, expr Expr, staticValue any, hasStaticValue bool, descending bool) (any, bool, error) {
	value, err := s.evalSortRankReducerArgValue(arg, expr, staticValue, hasStaticValue)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return int64(0), true, nil
	}
	order := "asc"
	if descending {
		order = "desc"
	}
	shape := "sort-index-sum/" + string(array.Kind()) + "/" + order
	out, handled, err := data.TryTypedSortIndexSumI64(array, descending)
	return qTypedRuntimeResult("ArraySortIndexSum", shape, out, handled, err)
}

func (s *EvalState) evalRankSumPlan(arg string, expr Expr, staticValue any, hasStaticValue bool) (any, bool, error) {
	value, err := s.evalSortRankReducerArgValue(arg, expr, staticValue, hasStaticValue)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return int64(0), true, nil
	}
	shape := "rank-sum/" + string(array.Kind())
	out, handled, err := data.TryTypedRankSumI64(array)
	return qTypedRuntimeResultReason("ArrayRankSum", shape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalSortedEdgePlan(arg string, expr Expr, staticValue any, hasStaticValue bool, descending bool, last bool) (any, bool, error) {
	value, err := s.evalSortRankReducerArgValue(arg, expr, staticValue, hasStaticValue)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	order := "asc"
	if descending {
		order = "desc"
	}
	edge := "first"
	if last {
		edge = "last"
	}
	shape := "sort-edge/" + string(array.Kind()) + "/" + order + "/" + edge
	out, handled, err := data.TryTypedSortedEdge(array, descending, last)
	return qTypedRuntimeResultReason("ArraySortedEdge", shape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalSortRankReducerArgValue(arg string, expr Expr, staticValue any, hasStaticValue bool) (any, error) {
	if hasStaticValue {
		return staticValue, nil
	}
	if expr != nil {
		value, err := s.evalValueExpr(expr)
		if err == nil {
			return value, nil
		}
		if !isUnsupportedEvalValueExpr(err) {
			return nil, err
		}
	}
	return s.eval(arg)
}

func qSortRankReducerPlusTerms(src string) []string {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if src == "" {
		return nil
	}
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
			if i+1 < len(src) && (src[i+1] == '/' || src[i+1] == '\\' || src[i+1] == '\'') {
				continue
			}
			left := strings.TrimSpace(src[:i])
			right := strings.TrimSpace(src[i+1:])
			if left == "" || right == "" {
				continue
			}
			terms := qSortRankReducerPlusTerms(left)
			terms = append(terms, qSortRankReducerPlusTerms(right)...)
			return terms
		}
	}
	return []string{src}
}

func qSortRankReducerBundleCandidate(src string) bool {
	return len(buildQSortRankReducerBundlePlan(src)) > 0
}

func buildQSortRankReducerBundlePlan(src string) []qSortRankReducerTermPlan {
	terms := qSortRankReducerPlusTerms(src)
	if len(terms) < 2 {
		return nil
	}
	out := make([]qSortRankReducerTermPlan, 0, len(terms))
	for _, term := range terms {
		plan, ok := qSortRankReducerTermPlanFor(term)
		if !ok {
			return nil
		}
		out = append(out, plan)
	}
	return out
}

func qSortRankReducerTermCandidate(src string) bool {
	_, ok := qSortRankReducerTermPlanFor(src)
	return ok
}

func qSortRankReducerTermPlanFor(src string) (qSortRankReducerTermPlan, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		body := strings.TrimSpace(src[len("+/"):])
		switch {
		case strings.HasPrefix(body, "iasc ") && wordBoundary(body, 0, len("iasc")):
			arg := strings.TrimSpace(body[len("iasc "):])
			expr, value, hasValue := qSortRankReducerArgPlan(arg)
			return qSortRankReducerTermPlanForValue(qSortRankReducerTermPlan{kind: qSortRankReducerTermSortIndexSum, arg: arg, argExpr: expr, argValue: value, hasArgValue: hasValue}), true
		case strings.HasPrefix(body, "idesc ") && wordBoundary(body, 0, len("idesc")):
			arg := strings.TrimSpace(body[len("idesc "):])
			expr, value, hasValue := qSortRankReducerArgPlan(arg)
			return qSortRankReducerTermPlanForValue(qSortRankReducerTermPlan{kind: qSortRankReducerTermSortIndexSum, arg: arg, argExpr: expr, argValue: value, hasArgValue: hasValue, descending: true}), true
		case strings.HasPrefix(body, "rank ") && wordBoundary(body, 0, len("rank")):
			arg := strings.TrimSpace(body[len("rank "):])
			expr, value, hasValue := qSortRankReducerArgPlan(arg)
			return qSortRankReducerTermPlanForValue(qSortRankReducerTermPlan{kind: qSortRankReducerTermRankSum, arg: arg, argExpr: expr, argValue: value, hasArgValue: hasValue}), true
		default:
			return qSortRankReducerTermPlan{}, false
		}
	}
	for _, edge := range []string{"first ", "last "} {
		if !strings.HasPrefix(src, edge) {
			continue
		}
		last := strings.HasPrefix(edge, "last")
		body := strings.TrimSpace(src[len(edge):])
		switch {
		case strings.HasPrefix(body, "asc ") && wordBoundary(body, 0, len("asc")):
			arg := strings.TrimSpace(body[len("asc "):])
			expr, value, hasValue := qSortRankReducerArgPlan(arg)
			return qSortRankReducerTermPlanForValue(qSortRankReducerTermPlan{kind: qSortRankReducerTermSortedEdge, arg: arg, argExpr: expr, argValue: value, hasArgValue: hasValue, last: last}), true
		case strings.HasPrefix(body, "desc ") && wordBoundary(body, 0, len("desc")):
			arg := strings.TrimSpace(body[len("desc "):])
			expr, value, hasValue := qSortRankReducerArgPlan(arg)
			return qSortRankReducerTermPlanForValue(qSortRankReducerTermPlan{kind: qSortRankReducerTermSortedEdge, arg: arg, argExpr: expr, argValue: value, hasArgValue: hasValue, descending: true, last: last}), true
		default:
			return qSortRankReducerTermPlan{}, false
		}
	}
	return qSortRankReducerTermPlan{}, false
}

func qSortRankReducerTermPlanForValue(plan qSortRankReducerTermPlan) qSortRankReducerTermPlan {
	if !plan.hasArgValue {
		return plan
	}
	static, ok := qStaticSortRankReducerTerm(plan)
	if ok {
		plan.staticResult = static
		plan.hasStaticResult = true
	}
	return plan
}

func qStaticSortRankReducerBundle(terms []qSortRankReducerTermPlan) (int64, bool) {
	if len(terms) < 2 {
		return 0, false
	}
	var total int64
	for _, term := range terms {
		if !term.hasStaticResult {
			return 0, false
		}
		total += term.staticResult
	}
	return total, true
}

func qStaticSortRankReducerTerm(term qSortRankReducerTermPlan) (int64, bool) {
	array, ok := term.argValue.(data.Array)
	switch term.kind {
	case qSortRankReducerTermSortIndexSum:
		if !ok {
			return 0, true
		}
		out, handled, err := data.TryTypedSortIndexSumI64(array, term.descending)
		return out, handled && err == nil
	case qSortRankReducerTermRankSum:
		if !ok {
			return 0, true
		}
		out, handled, err := data.TryTypedRankSumI64(array)
		return out, handled && err == nil
	case qSortRankReducerTermSortedEdge:
		if !ok {
			return 0, false
		}
		out, handled, err := data.TryTypedSortedEdge(array, term.descending, term.last)
		if err != nil || !handled {
			return 0, false
		}
		value, ok := integerValue(out)
		return value, ok
	default:
		return 0, false
	}
}

func qSortRankReducerArgPlan(arg string) (Expr, any, bool) {
	expr, ok, err := parseValueExpr(arg)
	if err != nil || !ok {
		return nil, nil, false
	}
	value, ok := staticQValueExpr(expr)
	return expr, value, ok
}

func staticQValueExpr(expr Expr) (any, bool) {
	switch x := expr.(type) {
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		return value, err == nil
	case String:
		return x.Value, true
	case Symbol:
		return data.Symbol(x.Name), true
	case Bool:
		return x.Value, true
	case Null:
		return data.NullValue, true
	case Temporal:
		value, err := parseQTemporal(x.Kind, x.Text)
		return value, err == nil
	case TypedNull:
		return data.NullForKind(data.Kind(x.Kind)), true
	case Vector:
		values := make([]any, len(x.Items))
		for i, item := range x.Items {
			value, ok := staticQValueExpr(item)
			if !ok {
				return nil, false
			}
			values[i] = value
		}
		value, err := evalValueVector(values)
		return value, err == nil
	default:
		return nil, false
	}
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
	if array, ok := value.(data.Array); ok && array.Len() > 0 {
		// Empty inputs keep the generic empty-sum identity (typed zero of
		// the SOURCE kind); the float-accumulating kernel must decline.
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
	if qPipelineFusedSumEmptyOperand(left, right) {
		// Empty operands keep the generic empty-sum identity (typed zero of
		// the SOURCE kind); the float-accumulating kernel must decline.
		return nil, false, nil
	}
	if qTypedIntegerOperandOK(left) && qTypedIntegerOperandOK(right) {
		// Integer-only operands sum to int64 on the generic route; the
		// float-accumulating kernel would change the result type.
		return nil, false, nil
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
		{word: "mdev"},
		{word: "ema"},
	} {
		leftExpr, rightExpr, ok := splitTopLevelWord(src, spec.word)
		if !ok {
			continue
		}
		leftValue, err := s.eval(leftExpr)
		if err != nil {
			return nil, true, err
		}
		width := int64(0)
		alpha := 0.0
		if spec.word == "ema" {
			var ok bool
			alpha, ok = numeric(leftValue)
			if !ok {
				return nil, true, fmt.Errorf("ema alpha must be numeric")
			}
			if alpha < 0 || alpha > 1 {
				return nil, true, fmt.Errorf("ema alpha must be in range 0..1")
			}
		} else {
			var ok bool
			width, ok = integerValue(leftValue)
			if !ok || width <= 0 || int64(int(width)) != width {
				return nil, true, fmt.Errorf("%s width must be a positive integer", spec.word)
			}
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
				moving, err = msum(leftValue, value)
			case "mavg":
				moving, err = mavg(leftValue, value)
			case "mcount":
				moving, err = mcount(leftValue, value)
			case "mmin":
				moving, err = mmin(leftValue, value)
			case "mmax":
				moving, err = mmax(leftValue, value)
			case "mdev":
				moving, err = mdevValue(leftValue, value)
			case "ema":
				moving, err = emaValue(leftValue, value)
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
		} else if spec.word == "mdev" {
			if out, handled, err := data.NumericMovingStdDevSum(array, int(width), false); err != nil || handled {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-mdev/"+string(array.Kind()), handled, err)
				if err != nil {
					return nil, true, fmt.Errorf("sum mdev: %w", err)
				}
				return out, true, nil
			} else {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-mdev/"+string(array.Kind()), handled, err)
			}
		} else if spec.word == "ema" {
			if out, handled, err := data.NumericExponentialMovingAverageSum(array, alpha); err != nil || handled {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-ema/"+string(array.Kind()), handled, err)
				if err != nil {
					return nil, true, fmt.Errorf("sum ema: %w", err)
				}
				return out, true, nil
			} else {
				recordRuntimeKernelProbe("ArrayMovingWindowSum", "vector-reduce/sum-ema/"+string(array.Kind()), handled, err)
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
			moving, err = msum(leftValue, value)
		case "mavg":
			moving, err = mavg(leftValue, value)
		case "mcount":
			moving, err = mcount(leftValue, value)
		case "mmin":
			moving, err = mmin(leftValue, value)
		case "mmax":
			moving, err = mmax(leftValue, value)
		case "mdev":
			moving, err = mdevValue(leftValue, value)
		case "ema":
			moving, err = emaValue(leftValue, value)
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
	if addChainHeadShadowsCanonical(terms[0]) {
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
	// Terms evaluate left-to-right (the compiled Binary tree's operand
	// order) but accumulate right-to-left (a+(b+c)): canonical q grouping,
	// and the association the compiled route's split produces — float chains
	// must be bit-identical across routes.
	values := make([]any, len(terms))
	for i, term := range terms {
		value, err := s.evalScalarAddChainTerm(term)
		if err != nil {
			return nil, true, err
		}
		values[i] = value
	}
	acc := values[len(values)-1]
	for i := len(values) - 2; i >= 0; i-- {
		if out, ok := addScalarNumericFast(values[i], acc); ok {
			acc = out
			continue
		}
		out, err := applyDyadic('+', values[i], acc)
		if err != nil {
			return nil, true, err
		}
		acc = out
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
	// Right-to-left accumulation: see tryEvalScalarAddChain.
	parts := make([]any, len(reducers))
	for i, reducer := range reducers {
		part, err := namedScalarReducerValue(reducer, value)
		if err != nil {
			return nil, true, err
		}
		parts[i] = part
	}
	acc := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		if out, ok := addScalarNumericFast(parts[i], acc); ok {
			acc = out
			continue
		}
		out, err := applyDyadic('+', parts[i], acc)
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
	if value, handled, err := s.tryEvalCallableOverScalar(src); err != nil || handled {
		return value, err
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

func (s *EvalState) tryEvalCallableOverScalar(src string) (any, bool, error) {
	fnSrc, initialSrc, valueSrc, ok := parseCallableOverApplication(src)
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
	if isCallableAdd(fn) {
		recordRuntimeKernelProbe("CallableOverScalar", "over-scalar/add/"+string(qRuntimeKernelOperandKind(value, nil)), err == nil, err)
	}
	return out, true, err
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
		case '!', '#', '_', '$', '@', '?', '~':
			if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
				continue
			}
			// A top-level non-arithmetic dyadic claims everything to its
			// right ((count"")+count!+0 is (count"")+(count!(+0))), so the
			// remainder is one term: stop splitting.
			i = len(src)
		case '+':
			if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 || isSign(src, i) {
				continue
			}
			if i+1 < len(src) && (src[i+1] == '/' || src[i+1] == '\\') {
				// A derived +/ (sum-over) or +\ (running-sum) folds EVERYTHING
				// to its right under canonical right-to-left grouping
				// ((0)++/(0)+x is (0)+(+/((0)+x))), so the remainder is one
				// term: stop splitting.
				i = len(src)
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
	// A parenthesized expression list ((count 0;9)) is a LIST literal, not a
	// scalar term: stripping its parens would change its meaning.
	if parts := splitTopLevelDelim(src, ';'); len(parts) > 1 {
		return false
	}
	// Temporal tokens that also lex as numbers (month 0000.01, minute 12:30)
	// are temporals on the compiled route and in canonical q; the chain must
	// not re-read them as floats.
	if qScalarAddChainTermLooksTemporal(src) {
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
	if _, _, _, ok := parseCallableOverApplication(src); ok {
		return true
	}
	if _, ok := buildScalarApplyIndexPlan(src); ok {
		return true
	}
	for _, word := range []string{"sum", "prd", "avg", "min", "max", "first", "last", "count", "med", "var", "dev", "svar", "sdev"} {
		if strings.HasPrefix(src, word) && wordBoundary(src, 0, len(word)) && strings.TrimSpace(src[len(word):]) != "" {
			return true
		}
	}
	if _, _, _, ok := qAddChainDyadicWordApplyTerm(src); ok {
		return true
	}
	if _, _, err := parseNumberOrBool(src); err == nil {
		return true
	}
	if qAddChainBareNameTerm(src) {
		return true
	}
	return false
}

// qScalarAddChainTermLooksTemporal reports whether a chain term is really a
// temporal literal (month 0000.01, time 12:30, ...), optionally signed. The
// dot/colon gate keeps the temporal probe off plain integer/name terms.
func qScalarAddChainTermLooksTemporal(src string) bool {
	if !strings.ContainsAny(src, ".:") {
		return false
	}
	probes := []string{src}
	if len(src) > 1 && (src[0] == '+' || src[0] == '-') {
		// Signed spellings: the temporal lexers accept some signed forms
		// (+000.01) but not others (+0000.01), so probe both.
		probes = append(probes, src[1:])
	}
	for _, probe := range probes {
		if _, _, ok := parseTemporalToken(probe); ok {
			return true
		}
		if value, ok, err := parseTemporalAtomOrVector(probe); ok && err == nil && value != nil {
			return true
		}
	}
	return false
}

// qAddChainBareNameTerm reports whether src is a plain binding name usable as
// an add-chain term: the term route is one lookupName, and verb/constant
// words stay on their existing routes.
func qAddChainBareNameTerm(src string) bool {
	if !isQAssignmentName(src) {
		return false
	}
	if _, ok := qEvalConstantWords[src]; ok {
		return false
	}
	if _, ok := lookupUnaryVerb(src); ok {
		return false
	}
	if _, ok := lookupDyadicVerbFunc(src); ok {
		return false
	}
	return true
}

func parseCallableOverApplication(src string) (fnSrc, initialSrc, valueSrc string, ok bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasSuffix(src, "]") {
		return "", "", "", false
	}
	open := findMatchingCallOpen(src)
	if open < 0 {
		return "", "", "", false
	}
	head := strings.TrimSpace(src[:open])
	if !strings.HasSuffix(head, "/") || strings.HasSuffix(head, "+/") {
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
		if len(src) == len(spec.word) {
			continue
		}
		if strings.IndexByte("=<>+-*%&|^,", src[len(spec.word)]) >= 0 {
			// The operator glues directly onto the word (`last%x`,
			// `last%0%0`): the word is the dyadic's LEFT OPERAND (a verb
			// atom), not a unary prefix application. Decline so the generic
			// dyadic split handles it, mirroring the compiled route's
			// tokenisation.
			continue
		}
		arg := strings.TrimSpace(src[len(spec.word):])
		if arg == "" {
			continue
		}
		idx, op, ok := findDyadic(arg)
		if !ok || op == ',' {
			// Join changes the result length (l+r); the elementwise
			// first/last shortcut below would be wrong for it.
			continue
		}
		if idx == 0 || strings.TrimSpace(arg[:idx]) == "" {
			// Operator-leading remainders (`first %x` via leading space) are
			// the same operand shape; keep them on the generic split too.
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
	out, handled, err := evalQTypedRuntimeArrayInt64("ArrayGroupCount", "group-count", array, data.TryTypedGroupCount)
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
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	indexes, atom, err := qAtomCutStarts(left, right)
	if err != nil {
		return nil, true, err
	}
	if !atom {
		indexes, err = qIntegerIndexes("cut", left)
		if err != nil {
			return nil, true, err
		}
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
		out = qSublistTakeCount(args[0], right)
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
	transforms := []struct {
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
	}
	// Strip the whole chain of length- and numeric-kind-preserving transforms
	// (count deltas fills x) before evaluating the base, so the inner
	// transforms are never materialized just to take a count.
	src = stripEnclosingParens(strings.TrimSpace(src))
	var applied []int
	for {
		matched := false
		for t, transform := range transforms {
			if !strings.HasPrefix(src, transform.word) || !wordBoundary(src, 0, len(strings.TrimSpace(transform.word))) {
				continue
			}
			applied = append(applied, t)
			src = stripEnclosingParens(strings.TrimSpace(src[len(transform.word):]))
			matched = true
			break
		}
		if !matched {
			break
		}
	}
	if len(applied) == 0 {
		return nil, false, nil
	}
	value, err := s.eval(src)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	// Apply kind checks innermost-first; every transform in the table
	// preserves numeric-ness, so the base kind stands in for each operand.
	for i := len(applied) - 1; i >= 0; i-- {
		transform := transforms[applied[i]]
		shape := "vector-count/" + strings.TrimSpace(transform.word) + "/" + string(array.Kind())
		if transform.kindOK != nil && !transform.kindOK(array.Kind()) {
			err := fmt.Errorf("%s", transform.verbErr)
			recordRuntimeKernelProbe(transform.kernel, shape, false, err)
			return nil, true, err
		}
		recordRuntimeKernelProbe(transform.kernel, shape, true, nil)
	}
	return int64(array.Len()), true, nil
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
	desc, ok, err := qTypedWhereCompareStatsDescriptor(left, right, op, shapePrefix, shapePrefix+"-stats/within")
	if err != nil || !ok {
		return 0, 0, ok, err
	}
	count, sum, handled, err = evalQTypedWhereCompareIndexStats(desc)
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
	desc, ok, err := qTypedWhereCompareCountDescriptor(left, right, op, shapePrefix, shapePrefix+"/within")
	if err != nil || !ok {
		return 0, ok, err
	}
	count, handled, err = evalQTypedWhereCompareCount(desc)
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
	desc, ok, err := qTypedWhereCompareIndexesDescriptor(left, right, op, shapePrefix, "within-to-index")
	if err != nil || !ok {
		return nil, ok, err
	}
	out, handled, err := evalQTypedWhereCompareIndexes(desc)
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
	out, handled, err := evalQTypedRuntimeArrayInt64("ArrayNullCount", "null-count", array, data.TryTypedNullCount)
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
	if !ok {
		return nil, false, nil
	}
	if array.Kind() != data.KindBool {
		// Integer carriers: `count where v` is the replication total (the
		// validated sum of the non-negative counts), so it streams over the
		// bulk-flattened values — lazy min/max nodes fuse the combine into
		// the accumulation — without materializing the index vector. Errors
		// and limits are exactly the `where` replication loop's.
		if total, bad, status, handled := data.TryTypedWhereReplicationTotal(array, qMaxListLength); handled {
			var err error
			switch status {
			case data.WhereReplicationNegative:
				err = fmt.Errorf("where expects non-negative integer counts")
			case data.WhereReplicationLimit:
				err = fmt.Errorf("where count %d exceeds the %d list limit", bad, int64(qMaxListLength))
			}
			recordRuntimeKernelProbe("ArrayWhereReplicationCount", "where-count/"+string(array.Kind()), err == nil, err)
			if err != nil {
				return nil, true, err
			}
			return total, true, nil
		}
		return nil, false, nil
	}
	out, handled, err := evalQTypedRuntimeArrayInt64("ArrayTrueCount", "true-count", array, data.TryTypedTrueCount)
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
		if array.Len() == 0 {
			recordRuntimeKernelProbe(scan.kernel, shape, true, nil)
			return int64(0), true, nil
		}
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
		data.NumericUnarySqrt,
		data.NumericUnaryLog,
		data.NumericUnaryFloor,
		data.NumericUnarySin,
		data.NumericUnaryCos,
		data.NumericUnaryTan,
		data.NumericUnaryAsin,
		data.NumericUnaryAcos,
		data.NumericUnaryAtan,
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
	return s.evalFbyArrays(agg, valueArray, groupArray)
}

// evalFbyArrays is the array-level core of evalFby, shared with the cached
// qFbyFastPlan route; typed kernels and probes run per call.
func (s *EvalState) evalFbyArrays(agg string, valueArray, groupArray data.Array) (any, error) {
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

var qFbyAggregateNames = []string{"sum", "sums", "avg", "var", "dev", "med", "min", "max", "first", "last", "count"}

func parseFbyAggregate(src string) (string, string, error) {
	src = strings.TrimSpace(src)
	// Canonical tuple spelling (agg;values) fby group, accepted alongside the
	// dialect's word form `agg values fby group`.
	if enclosed(src, '(', ')') {
		inner := strings.TrimSpace(src[1 : len(src)-1])
		if parts := splitTopLevelDelim(inner, ';'); len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			for _, agg := range qFbyAggregateNames {
				if name == agg {
					return agg, strings.TrimSpace(parts[1]), nil
				}
			}
		}
	}
	for _, name := range qFbyAggregateNames {
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
	case ",":
		return ',', "join", true
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
	case "where":
		return whereFilterValue, true
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
		return lookupNumericDyadicFloatVerb(verb)
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
		return keyVerb, true
	case "value":
		return value, true
	case "parse":
		return qParseVerb, true
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
		return lookupNumericUnaryVerb(strings.TrimSpace(verb))
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
		s.ensureOwnedEnv()
		s.env[fmt.Sprintf("x%d", i)] = arg
	}
	if len(args) > 0 {
		s.ensureOwnedEnv()
		s.env["x"] = args[0]
	}
	if len(args) > 1 {
		s.ensureOwnedEnv()
		s.env["y"] = args[1]
	}
	if len(args) > 2 {
		s.ensureOwnedEnv()
		s.env["z"] = args[2]
	}
	return func() {
		for name, previous := range old {
			if previous.ok {
				s.ensureOwnedEnv()
				s.env[name] = previous.value
				continue
			}
			s.ensureOwnedEnv()
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
			if hasDyadicOp {
				return applyEachPrior(op, nil, args[0])
			}
			return applyEachPriorFunc(dyad, nil, args[0])
		case "//":
			if !hasDyadicOp {
				return nil, fmt.Errorf("%s cannot be used with over-over", fn.verb)
			}
			return convergeOverOp(op, args[0])
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
			if hasDyadicOp {
				return applyEachPrior(op, args[0], args[1])
			}
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
		state.ensureOwnedEnv()
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

// tryEvalSeededAdverbInfix evaluates `seed f/ x` / `seed f\ x` where the
// adverb's left text is a seed expression followed by a trailing callable
// term (`3 {2*x}/ 1`, `{x<100} {2*x}/ 1`). With a monadic f these are q's
// do/while iterate forms; with a dyadic f the seeded fold/scan.
func (s *EvalState) tryEvalSeededAdverbInfix(leftSrc, adverb, rightSrc string) (any, bool, error) {
	if adverb != "/" && adverb != "\\" {
		return nil, false, nil
	}
	if strings.HasPrefix(rightSrc, "[") {
		return nil, false, nil
	}
	seedSrc, fnSrc, ok := splitTrailingCallableTerm(leftSrc)
	if !ok {
		return nil, false, nil
	}
	fn, err := s.eval(fnSrc)
	if err != nil || !isCallable(fn) {
		return nil, false, nil
	}
	seed, err := s.eval(seedSrc)
	if err != nil {
		return nil, false, nil
	}
	right, err := s.eval(rightSrc)
	if err != nil {
		return nil, true, err
	}
	if adverb == "/" {
		out, err := s.applyOverCallable(fn, seed, right)
		return out, true, err
	}
	out, err := s.applyScanCallable(fn, seed, right)
	return out, true, err
}

// qIsBuiltinVerbName reports whether name is a builtin verb word in any of
// the dispatch tables. Used to keep bare verb words out of seed position in
// seeded-iterate recognition (q seeds are nouns or function literals).
func qIsBuiltinVerbName(name string) bool {
	if _, ok := lookupUnaryVerb(name); ok {
		return true
	}
	if _, ok := lookupDyadicVerbFunc(name); ok {
		return true
	}
	if _, _, ok := lookupDyadicVerb(name); ok {
		return true
	}
	return false
}

// findSeededIterateInfix detects `seed name/ x` / `seed name\ x` where name
// is a bare identifier (a session-bound callable) preceded by a seed
// expression. Builtin verbs are handled by findAdverb before this probe.
func findSeededIterateInfix(src string) (string, string, string, bool) {
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
		if ch != '/' && ch != '\\' {
			continue
		}
		if i+1 < len(src) && src[i+1] == ':' {
			// each-left/each-right, not over/scan.
			i++
			continue
		}
		left := strings.TrimSpace(src[:i])
		right := strings.TrimSpace(src[i+1:])
		if left == "" || right == "" || strings.HasPrefix(right, "[") {
			continue
		}
		prefix, fnSrc, ok := splitTrailingCallableTerm(left)
		if !ok || !isQBareName(fnSrc) {
			continue
		}
		if seed := strings.TrimSpace(prefix); isQBareName(seed) && qIsBuiltinVerbName(seed) {
			// `count next/x` is prefix application of a verb word to the
			// derived verb (count (next/x)), not a seeded iterate: canonical
			// q seeds are nouns or function literals, never bare verb words.
			continue
		}
		return left, string(ch), right, true
	}
	return "", "", "", false
}

// splitTrailingCallableTerm splits src into a non-empty prefix expression and
// a trailing callable term: a `{...}` lambda, a parenthesized expression, or
// a bare name.
func splitTrailingCallableTerm(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", false
	}
	last := src[len(src)-1]
	var start int
	switch last {
	case '}':
		start = findMatchingOpenBackward(src, '{', '}')
	case ')':
		start = findMatchingOpenBackward(src, '(', ')')
	default:
		if !isQIdentRest(last) {
			return "", "", false
		}
		start = len(src) - 1
		for start > 0 && isQIdentRest(src[start-1]) {
			start--
		}
		if !isQIdentStart(src[start]) {
			return "", "", false
		}
	}
	if start <= 0 {
		return "", "", false
	}
	prefix := strings.TrimSpace(src[:start])
	if prefix == "" {
		return "", "", false
	}
	return prefix, strings.TrimSpace(src[start:]), true
}

func findMatchingOpenBackward(src string, open, close byte) int {
	depth := 0
	for i := len(src) - 1; i >= 0; i-- {
		switch src[i] {
		case close:
			depth++
		case open:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
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
		if _, ok := lookupUnaryVerb(src); ok {
			return true
		}
		if _, ok := lookupDyadicVerbFunc(src); ok {
			return true
		}
	}
	if strings.HasSuffix(src, "}") || strings.HasSuffix(src, "]") || strings.HasSuffix(src, ")") {
		return true
	}
	return false
}

func validateCallableReduceScanBoundary(fn any, adverb string) error {
	if adverb == "':" {
		if unary, ok := fn.(qUnaryFunction); ok {
			return fmt.Errorf("%s cannot be used with each-prior", unary.name)
		}
	}
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

// qWordAdverbs maps the word spellings of the iteration adverbs to their
// symbol forms: `f over x` = f/x, `f scan x` = f\x, `f prior x` = f':x
// (`each` rides its own handler with hard errors, matching its history).
var qWordAdverbs = []struct {
	word   string
	adverb string
}{
	{"over", "/"},
	{"scan", "\\"},
	{"prior", "':"},
}

// evalWordAdverbInfix evaluates `f over x` / `f scan x` / `f prior x` where f
// is any callable expression (verb in parens, lambda, bound name). Statements
// whose left side is not callable fall through to the rest of the cascade.
func (s *EvalState) evalWordAdverbInfix(src string) (any, bool, error) {
	for _, wa := range qWordAdverbs {
		callableExpr, rightExpr, ok := splitTopLevelWord(src, wa.word)
		if !ok {
			continue
		}
		callable, err := s.eval(callableExpr)
		if err != nil || !isCallable(callable) {
			continue
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, true, err
		}
		out, err := s.evalCallableAdverb(callable, wa.adverb, right)
		return out, true, err
	}
	return nil, false, nil
}

// qNamedAdverbWords are the bracket-call spellings each[f;x], over[f;x],
// scan[f;x], prior[f;x].
var qNamedAdverbWords = map[string]string{
	"each":  "'",
	"over":  "/",
	"scan":  "\\",
	"prior": "':",
}

// evalNamedAdverbBracketCall evaluates the named adverb call forms
// each[f;x] / over[f;x] / scan[f;x] / prior[f;x].
func (s *EvalState) evalNamedAdverbBracketCall(src string) (any, bool, error) {
	open := strings.IndexByte(src, '[')
	if open <= 0 || !strings.HasSuffix(src, "]") {
		return nil, false, nil
	}
	name := strings.TrimSpace(src[:open])
	adverb, ok := qNamedAdverbWords[name]
	if !ok || !enclosed(src[open:], '[', ']') {
		return nil, false, nil
	}
	args := splitQBracketFormArgs(strings.TrimSpace(src[open+1 : len(src)-1]))
	if len(args) != 2 {
		return nil, false, nil
	}
	fn, err := s.eval(strings.TrimSpace(args[0]))
	if err != nil {
		return nil, true, err
	}
	if !isCallable(fn) {
		return nil, true, fmt.Errorf("%s[f;x] first argument is not callable", name)
	}
	right, err := s.eval(strings.TrimSpace(args[1]))
	if err != nil {
		return nil, true, err
	}
	out, err := s.evalCallableAdverb(fn, adverb, right)
	return out, true, err
}

// evalParenDerivedVerbJuxtaposed evaluates a parenthesized derived verb
// juxtaposed onto its operand: `(+/)x`, `(+/) til 5`, `(,/)x`, `(+\)x`,
// `(,//)x`. Bracket calls `(+/)[x]` ride the postfix-index path; this
// handler covers the noun-juxtaposition spelling.
func (s *EvalState) evalParenDerivedVerbJuxtaposed(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "(") {
		return nil, false, nil
	}
	end := findMatchingDelimiter(src, 0, '(', ')')
	if end <= 0 || end == len(src)-1 {
		return nil, false, nil
	}
	rest := strings.TrimSpace(src[end+1:])
	if rest == "" || !qDerivedVerbJuxtapositionLead(rest[0]) {
		return nil, false, nil
	}
	if fn, ok := parseDyadicAdverbFunction(src[:end+1]); ok {
		right, err := s.eval(rest)
		if err != nil {
			return nil, true, err
		}
		out, err := s.applyCallable(fn, []any{right})
		return out, true, err
	}
	// `'`-derived monadic verbs spelled with parens: `(count') x`,
	// `(reverse') x`. The bracket call count'[x] already rides the
	// postfix-index path; this covers the juxtaposed spelling.
	body := strings.TrimSpace(src[1:end])
	if !strings.HasSuffix(body, "'") || strings.HasSuffix(body, "':") {
		return nil, false, nil
	}
	verb := strings.TrimSpace(body[:len(body)-1])
	unary, ok := lookupUnaryVerb(verb)
	if !ok {
		return nil, false, nil
	}
	right, err := s.eval(rest)
	if err != nil {
		return nil, true, err
	}
	out, err := s.evalCallableAdverb(qUnaryFunction{name: verb, fn: unary}, "'", right)
	return out, true, err
}

// qDerivedVerbJuxtapositionLead reports whether ch can begin the operand of a
// juxtaposed derived verb. Leading infix operator characters fall through so
// the derived verb can still serve as the left operand of an infix form.
func qDerivedVerbJuxtapositionLead(ch byte) bool {
	return isQIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '(' || ch == '`' || ch == '"' || ch == '.'
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
	if len(items) == 0 && !isTypedEmptyAdverbSource(v) {
		// Canonical empty-each: f each () is () (the generic empty list),
		// not a null-kind empty.
		return data.NewAny(nil), nil
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
	if op, ok := callableFastDyadicOp(fn); ok {
		if out, handled, err := tryApplyTypedAdverbDyadic(op, "'", left, right); handled || err != nil {
			return out, err
		}
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

// applyEachLeftCallable: canonical q each-left iterates the LEFT operand,
// applying f(item, whole-right) per left item.
func (s *EvalState) applyEachLeftCallable(fn any, left any, right any) (any, error) {
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

// applyEachRightCallable: canonical q each-right iterates the RIGHT operand,
// applying f(whole-left, item) per right item.
func (s *EvalState) applyEachRightCallable(fn any, left any, right any) (any, error) {
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

// qCallableMonadic reports whether fn is a rank-1 (or rank-0) callable.
// q's over/scan adverbs are rank-dispatched: a MONADIC f makes f/ the
// converge/do/while iterators, while a dyadic f makes f/ the seeded fold.
func qCallableMonadic(fn any) bool {
	switch f := fn.(type) {
	case qLambda:
		rank, ok := qLambdaRank(f.body)
		return ok && rank <= 1
	case qUnaryFunction:
		return true
	default:
		return false
	}
}

// qLambdaRank reports a lambda's rank: explicit parameter count when the
// body declares `[...]`, otherwise the highest implicit x/y/z referenced
// (nested lambdas excluded; their implicit params are their own).
func qLambdaRank(body string) (int, bool) {
	src := strings.TrimSpace(body)
	if strings.HasPrefix(src, "[") {
		end := findMatchingDelimiter(src, 0, '[', ']')
		if end < 0 {
			return 0, false
		}
		paramSrc := strings.TrimSpace(src[1:end])
		if paramSrc == "" {
			return 0, true
		}
		return len(splitQBracketFormArgs(paramSrc)), true
	}
	rank := 1
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
		case '{':
			braceDepth++
			continue
		case '}':
			braceDepth--
			continue
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
			continue
		}
		if braceDepth != 0 {
			continue
		}
		if isQIdentStart(ch) {
			start := i
			j := i
			for j < len(src) && isQIdentRest(src[j]) {
				j++
			}
			word := src[start:j]
			i = j - 1
			if start > 0 && isQIdentRest(src[start-1]) {
				continue
			}
			switch word {
			case "y":
				if rank < 2 {
					rank = 2
				}
			case "z":
				rank = 3
			}
		}
	}
	return rank, true
}

// qIterateLimit caps converge/while iteration so non-terminating canonical
// forms surface an error instead of hanging the process.
const qIterateLimit = 1 << 20

// qIterateWallBudget bounds wall-clock time of converge/do/while iteration.
// Canonical q would spin forever on non-terminating iterates; this sandbox
// guard converts them into a q error. Tests/fuzzers may lower it via
// SetIterateWallBudget.
var qIterateWallBudget atomic.Int64

func init() { qIterateWallBudget.Store(int64(3 * time.Second)) }

// SetIterateWallBudget overrides the converge/do/while wall-clock budget and
// returns the previous value. Intended for tests and fuzz harnesses.
func SetIterateWallBudget(d time.Duration) time.Duration {
	return time.Duration(qIterateWallBudget.Swap(int64(d)))
}

type qIterateClock struct {
	start  time.Time
	budget time.Duration
}

func newQIterateClock() qIterateClock {
	return qIterateClock{start: time.Now(), budget: time.Duration(qIterateWallBudget.Load())}
}

func (c qIterateClock) exceeded(iter int64) bool {
	// Exponentially-growing iterates (converge of {x,x}) blow up within the
	// first few dozen iterations, so check densely early and cheaply later.
	if iter < 1024 {
		if iter&7 != 7 {
			return false
		}
	} else if iter&1023 != 1023 {
		return false
	}
	return time.Since(c.start) > c.budget
}

// applyIterateOver implements the canonical q monadic-f iterate family:
//   - f/[x]        converge: apply f until the result matches the previous
//     result or the original argument
//   - n f/ x, f/[n;x]      do-iterate: apply f n times
//   - cond f/ x, f/[c;x]   while-iterate: apply f while cond(result) is true
//
// scan (f\) returns every intermediate value, starting with x itself.
func (s *EvalState) applyIterateOver(fn any, seed any, v any, scan bool) (any, error) {
	collect := []any(nil)
	if scan {
		collect = append(collect, v)
	}
	finish := func(value any) (any, error) {
		if scan {
			return inferQArray(collect, qKindOfValue(v)), nil
		}
		return value, nil
	}
	if seed == nil {
		// Converge.
		prev := v
		clock := newQIterateClock()
		for iter := 0; ; iter++ {
			if iter >= qIterateLimit || clock.exceeded(int64(iter)) {
				return nil, fmt.Errorf("converge did not terminate within %d iterations", iter)
			}
			next, err := s.applyCallable(fn, []any{prev})
			if err != nil {
				return nil, err
			}
			if matchValue(next, prev) {
				return finish(prev)
			}
			if iter > 0 && matchValue(next, v) {
				// Cycle back to the original argument.
				return finish(next)
			}
			if scan {
				collect = append(collect, next)
			}
			prev = next
		}
	}
	if isCallable(seed) {
		// While-iterate.
		current := v
		clock := newQIterateClock()
		for iter := 0; ; iter++ {
			if iter >= qIterateLimit || clock.exceeded(int64(iter)) {
				return nil, fmt.Errorf("while-iterate did not terminate within %d iterations", iter)
			}
			cond, err := s.applyCallable(seed, []any{current})
			if err != nil {
				return nil, err
			}
			proceed, err := qIterateTruthy(cond)
			if err != nil {
				return nil, err
			}
			if !proceed {
				return finish(current)
			}
			next, err := s.applyCallable(fn, []any{current})
			if err != nil {
				return nil, err
			}
			if scan {
				collect = append(collect, next)
			}
			current = next
		}
	}
	n, ok := integerValue(seed)
	if !ok {
		return nil, fmt.Errorf("iterate left operand must be an integer count or a predicate")
	}
	if n < 0 {
		return nil, fmt.Errorf("do-iterate count must be non-negative")
	}
	current := v
	clock := newQIterateClock()
	for i := int64(0); i < n; i++ {
		if clock.exceeded(i) {
			return nil, fmt.Errorf("do-iterate did not terminate within budget after %d iterations", i)
		}
		next, err := s.applyCallable(fn, []any{current})
		if err != nil {
			return nil, err
		}
		if scan {
			collect = append(collect, next)
		}
		current = next
	}
	return finish(current)
}

func qIterateTruthy(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	default:
		if data.IsNull(v) {
			return false, nil
		}
		if n, ok := numeric(v); ok {
			return n != 0, nil
		}
		return false, fmt.Errorf("while-iterate predicate produced %T, want boolean", v)
	}
}

func (s *EvalState) applyOverCallable(fn any, initial any, v any) (any, error) {
	if qCallableMonadic(fn) {
		return s.applyIterateOver(fn, initial, v, false)
	}
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
	if op, ok := callableFastDyadicOp(fn); ok {
		if out, handled, err := tryApplyTypedOverScan(op, initial, v, false); handled || err != nil {
			return out, err
		}
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
	if qCallableMonadic(fn) {
		return s.applyIterateOver(fn, initial, v, true)
	}
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
	if op, ok := callableFastDyadicOp(fn); ok {
		if out, handled, err := tryApplyTypedOverScan(op, initial, v, true); handled || err != nil {
			return out, err
		}
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
	if op, ok := callableFastDyadicOp(fn); ok {
		if out, handled, err := tryApplyTypedEachPrior(op, initial, v); handled || err != nil {
			return out, err
		}
	}
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

func (s *EvalState) applyEachUnary(verb string, v any) (any, error) {
	fn, ok := s.unaryVerbFunc(verb)
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
	// Each-left/each-right over two vector operands build a row per item of
	// the iterated side (canonical q), which plain broadcast arithmetic does
	// not represent: fall through to the generic row builder.
	if (adverb == "\\:" || adverb == "/:") && lok && rok {
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
	if out, handled, err := tryApplyTypedEachPrior(op, initial, v); handled || err != nil {
		return out, err
	}
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

// applyEachLeftFunc: canonical q each-left `x f\:y` applies f between each
// item of the LEFT operand and the whole right operand.
func applyEachLeftFunc(fn func(any, any) (any, error), left, right any) (any, error) {
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

func applyEachRight(op byte, left, right any) (any, error) {
	if out, handled, err := tryApplyTypedAdverbDyadic(op, "/:", left, right); err != nil || handled {
		return out, err
	}
	return applyEachRightFunc(dyadicVerbFunc(op), left, right)
}

// applyEachRightFunc: canonical q each-right `x f/:y` applies f between the
// whole left operand and each item of the RIGHT operand.
func applyEachRightFunc(fn func(any, any) (any, error), left, right any) (any, error) {
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

func isTypedEmptyAdverbSource(v any) bool {
	array, ok := v.(data.Array)
	if !ok || array.Len() != 0 {
		return false
	}
	kind := array.Kind()
	return kind != "" && kind != data.KindNull && kind != data.KindAny
}

// convergeOverOp implements the composed adverb `f//x` = (f/)/x for builtin
// dyadic ops: the derived fold f/ is applied repeatedly until the value stops
// changing (q's converge iterator). `,//x` is the canonical deep raze.
func convergeOverOp(op byte, v any) (any, error) {
	prev := v
	clock := newQIterateClock()
	for iter := 0; ; iter++ {
		if iter >= qIterateLimit || clock.exceeded(int64(iter)) {
			return nil, fmt.Errorf("converge did not terminate within %d iterations", iter)
		}
		next, err := applyOver(op, nil, prev)
		if err != nil {
			return nil, err
		}
		if matchValue(next, prev) {
			return prev, nil
		}
		if iter > 0 && matchValue(next, v) {
			// Cycle back to the original argument.
			return next, nil
		}
		prev = next
	}
}

func applyOver(op byte, initial any, v any) (any, error) {
	if op == '+' && initial == nil {
		return sum(v)
	}
	if out, handled, err := tryApplyTypedOverScan(op, initial, v, false); handled || err != nil {
		return out, err
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
		if identity, ok := qReduceIdentity(op, v); ok {
			return identity, nil
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

// qReduceIdentity yields the canonical empty-vector identities for the
// derived reducers: `&/()` is the positive infinity of the element type
// (0W, 0w, 1b) and `|/()` the negative one (-0W, -0w, 0b); `+/()` is the
// element type's zero and `*/()` its one (canonical (+/)[til 0] -> 0,
// (*/)[til 0] -> 1).
func qReduceIdentity(op byte, v any) (any, bool) {
	kind := data.Kind("")
	if array, ok := v.(data.Array); ok {
		kind = array.Kind()
	}
	switch op {
	case '&', '|':
		return qMinMaxIdentityForKind(op == '|', kind)
	case '+':
		return qSumIdentityForKind(kind), true
	case '*':
		return qProductIdentityForKind(kind), true
	default:
		return nil, false
	}
}

// qSumIdentityForKind is the canonical empty-sum identity: the element
// domain's zero (long 0 for integer and untyped vectors, the float zero for
// float vectors), matching what the typed sum kernels produce for empty
// typed inputs.
func qSumIdentityForKind(kind data.Kind) any {
	switch kind {
	case data.KindF32, data.KindF64:
		// f32 sums accumulate in float64 on every route (generic walk and
		// typed kernels), so the empty identity is float64 too.
		return float64(0)
	default:
		return int64(0)
	}
}

// qProductIdentityForKind is the canonical empty-product identity: the
// element domain's one.
func qProductIdentityForKind(kind data.Kind) any {
	switch kind {
	case data.KindF32, data.KindF64:
		// f32 products accumulate in float64 on every route (see
		// qSumIdentityForKind), so the empty identity is float64 too.
		return float64(1)
	default:
		return int64(1)
	}
}

func qMinMaxIdentityForKind(wantMax bool, kind data.Kind) (any, bool) {
	switch kind {
	case data.KindBool:
		return !wantMax, true
	case data.KindF32:
		if wantMax {
			return float32(math.Inf(-1)), true
		}
		return float32(math.Inf(1)), true
	case data.KindF64:
		if wantMax {
			return math.Inf(-1), true
		}
		return math.Inf(1), true
	case data.KindI16:
		if wantMax {
			return int16(-math.MaxInt16), true
		}
		return int16(math.MaxInt16), true
	case data.KindI32:
		if wantMax {
			return int32(-math.MaxInt32), true
		}
		return int32(math.MaxInt32), true
	default:
		// Integer vectors and the untyped empty list reduce with the long
		// identities, matching canonical `&/()` -> 0W and `|/()` -> -0W.
		if wantMax {
			return int64(-math.MaxInt64), true
		}
		return int64(math.MaxInt64), true
	}
}

func applyScan(op byte, initial any, v any) (any, error) {
	if op == '+' && initial == nil {
		return sums(v)
	}
	if out, handled, err := tryApplyTypedOverScan(op, initial, v, true); handled || err != nil {
		return out, err
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
		if strings.ContainsAny(sym, " \t\n\r") {
			// `t insert (...)` and similar are expressions, not one symbol
			// literal with embedded spaces; echoing them back silently as a
			// symbol hid real parse failures.
			return nil, fmt.Errorf("malformed symbol list near %q", sym)
		}
		out = append(out, data.Symbol(sym))
	}
	return out, nil
}

// evalConsoleWriteForm implements the canonical q console handle write
// h "text": handle 1/-1 is stdout, 2/-2 is stderr, the chars are written
// (negative handles append a newline) and the handle is returned. Only the
// literal-handle literal-string spelling is claimed — anything else falls
// through to the ordinary cascade — so the check is pure syntax and never
// double-evaluates an expression with side effects.
func evalConsoleWriteForm(src string) (any, bool, error) {
	if len(src) == 0 {
		return nil, false, nil
	}
	switch src[0] {
	case '1', '2', '-':
	default:
		return nil, false, nil
	}
	sp := strings.IndexAny(src, " \t")
	if sp < 0 {
		return nil, false, nil
	}
	var handle int64
	switch src[:sp] {
	case "1":
		handle = 1
	case "-1":
		handle = -1
	case "2":
		handle = 2
	case "-2":
		handle = -2
	default:
		return nil, false, nil
	}
	rest := strings.TrimSpace(src[sp:])
	if len(rest) < 2 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return nil, false, nil
	}
	text, err := strconv.Unquote(rest)
	if err != nil {
		return nil, false, nil
	}
	out := os.Stdout
	if handle == 2 || handle == -2 {
		out = os.Stderr
	}
	if handle < 0 {
		fmt.Fprintln(out, text)
	} else {
		fmt.Fprint(out, text)
	}
	return handle, true, nil
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
		n, ok := v.(int64)
		if !ok {
			// Mixed atom-vector literals like `1 "hello"` are not a typed
			// vector; canonical q would need (1;"hello") for a generic list.
			// Reject with a parse error instead of panicking.
			return nil, fmt.Errorf("mixed-type vector literal %q is not supported; use (a;b) for a generic list", src)
		}
		xs[i] = n
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
		if containsInternalDyadicSign(field) {
			// `0000.+1 0` is an arithmetic split (0000. + 1 0), not a
			// temporal vector; the lenient temporal lexer must not claim
			// fields glued to a dyadic operator.
			return false
		}
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
	for i, field := range fields {
		if strings.HasPrefix(field, "\"") {
			if _, err := strconv.Unquote(field); err != nil {
				return false
			}
			continue
		}
		if i > 0 && strings.HasPrefix(fields[0], "\"") {
			// String-headed juxtaposition with a non-string tail is
			// canonical q string indexing ("hello" 1 -> "e"), not a literal
			// vector; let the parsed-value route claim it.
			return false
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

// errNotNumericLiteral is the shared probe error for sources that cannot be
// numeric literals. Number parsing is used as a per-evaluation probe on
// identifiers, so the early guard must not allocate a formatted error.
var errNotNumericLiteral = errors.New("not a numeric literal")

func parseNumberOrBool(src string) (any, bool, error) {
	if src == "" {
		return nil, false, errNotNumericLiteral
	}
	switch c := src[0]; {
	case c >= '0' && c <= '9', c == '-', c == '+', c == '.':
		// Numeric-looking: run the full literal parse below.
	case src == "true":
		return true, false, nil
	case src == "false":
		return false, false, nil
	default:
		return nil, false, errNotNumericLiteral
	}
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

// qBangValue dispatches dyadic ! on evaluated operands. A null atom left is
// the canonical 0N!x display verb (print x, return x — the q debugging
// idiom); otherwise count-keyed tables, keyed tables, then dict construction.
// Both the string evaluator and the compiled DictExpr route funnel here so
// the routes agree by construction.
func qBangValue(keys, values any) (any, error) {
	if data.IsNull(keys) {
		fmt.Println(qExprLiteral(values))
		return values, nil
	}
	if table, ok, err := keyedTableByCount(keys, values); ok || err != nil {
		return table, err
	}
	if keyed, ok, err := keyedTable(keys, values); ok || err != nil {
		return keyed, err
	}
	return dict(keys, values)
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
	if sym, ok := key.(data.Symbol); ok && !data.IsNull(sym) {
		// Scalar symbol probe: a non-null Symbol only ever equalValue-matches
		// a Symbol entry (DeepEqual is type-strict), so scan with direct
		// string equality instead of boxed slices plus reflect.DeepEqual.
		for j, existing := range d.Keys {
			if s, ok := existing.(data.Symbol); ok && s == sym {
				if d.Values[j] == nil {
					return data.NullValue, nil
				}
				return d.Values[j], nil
			}
		}
		return data.NullValue, nil
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
		if out, handled, err := frameRowIndexValue(frame, index); handled || err != nil {
			return out, err
		}
		return frameColumnLookup(frame, index)
	}
	if keyed, ok := v.(data.KeyedFrame); ok {
		if frameHasLookupColumns(keyed.Frame(), index) {
			return frameColumnLookup(keyed.Frame(), index)
		}
		frame, err := keyedTableLookup(keyed, index)
		if err != nil {
			return nil, err
		}
		// Canonical keyed-table indexing: a single key yields the value-row
		// DICT (column name -> cell), null-filled when the key is absent.
		return keyedLookupResultDict(frame)
	}
	switch x := v.(type) {
	case data.Array:
		return arrayReadIndexValue(x, index)
	case string:
		return stringReadIndexValue(x, index)
	default:
		return nil, fmt.Errorf("index expects a vector, string, or dictionary")
	}
}

// qReadIndexRows converts a read-path index (integer atom, null atom, or
// integer vector possibly containing nulls) into row numbers. Null indexes
// map to -1, which the read path treats like any other out-of-range row and
// null-fills (canonical q: (1 2 3)@5 -> 0N, @-1 -> 0N, @0N -> 0N). Non-integer
// indexes keep erroring; assignment/amend targets do not use this path.
func qReadIndexRows(index any) (rows []int64, scalar bool, err error) {
	switch x := index.(type) {
	case int64:
		return []int64{x}, true, nil
	case int32:
		return []int64{int64(x)}, true, nil
	case int16:
		return []int64{int64(x)}, true, nil
	}
	if data.IsNull(index) {
		if kind, ok := data.NullKind(index); ok {
			switch kind {
			case data.KindI16, data.KindI32, data.KindI64, data.KindNull, data.Kind(""):
			default:
				return nil, false, fmt.Errorf("index must be an integer or integer vector")
			}
		}
		return []int64{-1}, true, nil
	}
	array, ok := index.(data.Array)
	if !ok {
		return nil, false, fmt.Errorf("index must be an integer or integer vector")
	}
	switch array.Kind() {
	case data.KindI16, data.KindI32, data.KindI64, data.KindNull:
	default:
		return nil, false, fmt.Errorf("index vector must contain integers")
	}
	rows = make([]int64, array.Len())
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok || data.IsNull(item) {
			rows[i] = -1
			continue
		}
		n, ok := integerValue(item)
		if !ok {
			return nil, false, fmt.Errorf("index vector must contain integers")
		}
		rows[i] = n
	}
	// A one-row index vector keeps yielding an atom, mirroring the historic
	// indexInts contract shared by every eval route.
	return rows, len(rows) == 1, nil
}

// arrayReadIndexValue is the read path for vector@index: out-of-range,
// negative, and null indexes null-fill instead of erroring (canonical q;
// amend/assignment targets keep their index errors).
func arrayReadIndexValue(array data.Array, index any) (any, error) {
	if indexArray, ok := index.(data.Array); ok && indexArray.Len() != 1 {
		out, handled, err := qEvalArrayGatherI64Primitive(array, indexArray)
		if handled && err == nil {
			return out, nil
		}
		// Out-of-range or null rows: fall through to the null-filling path.
	}
	rows, scalar, err := qReadIndexRows(index)
	if err != nil {
		return nil, err
	}
	length := int64(array.Len())
	if scalar {
		row := rows[0]
		if row < 0 || row >= length {
			return data.NullValue, nil
		}
		if rowArr, handled, err := data.TryRowArrayIndex(array, int(row)); err == nil && handled {
			shape := "matrix-row/" + string(array.Kind()) + "/" + qRuntimeCardinalityShape(rowArrayLen(rowArr))
			recordRuntimeKernelProbe("ArrayMatrixRowIndex", shape, true, nil)
			return rowArr, nil
		}
		item, ok := array.At(int(row))
		if !ok {
			return data.NullValue, nil
		}
		return item, nil
	}
	allValid := true
	for _, row := range rows {
		if row < 0 || row >= length {
			allValid = false
			break
		}
	}
	if allValid {
		indexes := make([]int, len(rows))
		for i, row := range rows {
			indexes[i] = int(row)
		}
		return data.Gather(array, indexes)
	}
	out := make([]any, len(rows))
	for i, row := range rows {
		if row < 0 || row >= length {
			out[i] = data.NullValue
			continue
		}
		item, ok := array.At(int(row))
		if !ok {
			out[i] = data.NullValue
			continue
		}
		out[i] = item
	}
	return inferQArray(out, array.Kind()), nil
}

// stringReadIndexValue indexes a string on the read path: out-of-range,
// negative, and null indexes fill with the char null " " (canonical q
// "abc" 5 -> " ").
func stringReadIndexValue(s string, index any) (any, error) {
	rows, scalar, err := qReadIndexRows(index)
	if err != nil {
		return nil, err
	}
	runes := []rune(s)
	charAt := func(row int64) string {
		if row < 0 || row >= int64(len(runes)) {
			return " "
		}
		return string(runes[row])
	}
	if scalar {
		return charAt(rows[0]), nil
	}
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = charAt(row)
	}
	return data.NewString(out), nil
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

// keyedLookupResultDict converts the (at most one row) value frame from a
// keyed-table key lookup into the canonical value-row dictionary; an empty
// frame (key miss) yields the null-filled dictionary over the value columns.
func keyedLookupResultDict(frame data.Frame) (EvalDict, error) {
	if frame.Len() > 0 {
		return frameRowDict(frame, int64(frame.Len()-1))
	}
	names := frame.Schema().Names()
	keys := make([]any, len(names))
	values := make([]any, len(names))
	for i, name := range names {
		keys[i] = name
		values[i] = data.NullValue
	}
	return EvalDict{Keys: keys, Values: values}, nil
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
		if out, handled, err := frameColumnAssignValue(frame, key, value); handled || err != nil {
			return out, err
		}
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
	if isCallableAdd(fn) {
		if indexArray, ok := key.(data.Array); ok {
			shape := "amend-add-index-array/" + string(array.Kind()) + "/" + string(indexArray.Kind()) + "/" + string(qKindOfValue(value))
			if typed, handled, err := data.TryTypedAmendAddIndexArray(array, indexArray, value); err != nil || handled {
				recordRuntimeKernelProbe("ArrayAmendAddIndexArray", shape, handled, err)
				if err != nil {
					return nil, err
				}
				return typed, nil
			} else {
				recordRuntimeKernelProbe("ArrayAmendAddIndexArray", shape, handled, err)
			}
		}
	}
	indexes, _, err := indexInts(key)
	if err != nil {
		return nil, err
	}
	if isCallableAdd(fn) {
		shape := "amend-add-indexes/" + string(array.Kind()) + "/" + string(qKindOfValue(value))
		if typed, handled, err := data.TryTypedAmendAddIndexes(array, indexes, value); err != nil || handled {
			recordRuntimeKernelProbe("ArrayAmendAddIndexes", shape, handled, err)
			if err != nil {
				return nil, err
			}
			return typed, nil
		} else {
			recordRuntimeKernelProbe("ArrayAmendAddIndexes", shape, handled, err)
		}
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
	// Canonical q broadcasts an atom amend value over every index
	// (@[1 2 3;0 1;+;10] -> 11 12 3).
	if _, isArray := value.(data.Array); !isArray {
		items := make([]any, count)
		for i := range items {
			items[i] = value
		}
		return items, nil
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
	values, err := amendInputValues(value, len(indexes))
	if err != nil {
		return nil, err
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
	// Canonical n! applies to keyed tables too: 0!kt unkeys (the 0!1!t
	// round-trip), n!kt rekeys the flattened table on its first n columns.
	if keyed, ok := values.(data.KeyedFrame); ok {
		values = keyed.Frame()
	}
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
		cols = append(cols, data.Column{Name: name, Data: data.MaterializeFrameColumn(array)})
	}
	// q-side arrays are immutable value carriers: adopt the densely
	// materialized columns directly instead of routing every column through
	// NewFrame's defensive per-row boxed clone gather.
	return data.NewFrameAdoptingColumns(cols...)
}

func reshapeValue(shapeValue, value any) (any, error) {
	shape, nullDim, err := qReshapeShape(shapeValue)
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
	if nullDim >= 0 {
		// Canonical null reshape dimension: 0N k#x chunks x into k-sized
		// rows and n 0N#x into n rows of ceil(count/n), exactly atom-cut
		// semantics (the last row may be short when the count does not
		// divide).
		if len(shape) != 2 {
			return nil, fmt.Errorf("reshape supports a null dimension only in two-dimensional shapes")
		}
		size := 0
		if nullDim == 0 {
			size = shape[1]
		} else {
			rows := shape[0]
			if rows <= 0 {
				return nil, fmt.Errorf("reshape dimension 0 must be positive with a null dimension 1")
			}
			size = (source.Len() + rows - 1) / rows
		}
		if size <= 0 {
			return nil, fmt.Errorf("reshape null dimension needs a positive companion dimension")
		}
		starts := make([]int, 0, (source.Len()+size-1)/size)
		for i := 0; i < source.Len(); i += size {
			starts = append(starts, i)
		}
		return data.Cut(starts, source)
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

// qReshapeShape parses the reshape dimensions. nullDim is the position of a
// single null dimension (0N 2#x / 2 0N#x compute it from the other one), or
// -1 when every dimension is concrete.
func qReshapeShape(value any) ([]int, int, error) {
	array, ok := value.(data.Array)
	if !ok {
		n, ok := integerValue(value)
		if !ok || int64(int(n)) != n {
			return nil, -1, fmt.Errorf("# left operand must be an integer count")
		}
		return []int{int(n)}, -1, nil
	}
	if array.Len() == 0 {
		return nil, -1, fmt.Errorf("reshape expects at least one dimension")
	}
	out := make([]int, array.Len())
	nullDim := -1
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, -1, fmt.Errorf("reshape dimension row %d out of range", i)
		}
		if data.IsNull(item) {
			if nullDim >= 0 {
				return nil, -1, fmt.Errorf("reshape allows at most one null dimension")
			}
			nullDim = i
			continue
		}
		n, ok := integerValue(item)
		if !ok || int64(int(n)) != n {
			return nil, -1, fmt.Errorf("reshape dimension %d must be an integer", i)
		}
		if n < 0 {
			return nil, -1, fmt.Errorf("reshape dimension %d must be non-negative", i)
		}
		out[i] = int(n)
	}
	return out, nullDim, nil
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
	if op == ',' {
		// Join is a whole-value verb: it must not broadcast elementwise.
		return joinValue(left, right)
	}
	if op == '~' {
		return matchValue(left, right), nil
	}
	if op == '=' {
		if out, handled, err := equalsDictOrCallable(left, right); handled || err != nil {
			return out, err
		}
	}
	if out, handled, err := applyDictDyadic(op, left, right); handled || err != nil {
		return out, err
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
			if !hasTypedNullKind(left) && !hasTypedNullKind(right) &&
				temporalKindOfValue(left) == "" && temporalKindOfValue(right) == "" {
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
	if out, handled, err := applyTemporalDyadic(op, left, right); handled || err != nil {
		return out, err
	}
	// `0-x` preserves the sized numeric type of x (0-0e is real, 0-0i is
	// int), the same shortcut the compiled route's evalValueBinary applies.
	if op == '-' && isNumericZero(left) {
		if value, ok := negateTypedNumeric(right); ok {
			return value, nil
		}
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

func applyScalarNumericAdd4(a, b, c, d any) (any, bool) {
	if ai, ok := integerValue(a); ok {
		if bi, ok := integerValue(b); ok {
			if ci, ok := integerValue(c); ok {
				if di, ok := integerValue(d); ok {
					return ai + bi + ci + di, true
				}
			}
		}
	}
	af, aok := numeric(a)
	bf, bok := numeric(b)
	cf, cok := numeric(c)
	df, dok := numeric(d)
	if !aok || !bok || !cok || !dok {
		return nil, false
	}
	return af + bf + cf + df, true
}

func hasTypedNullKind(v any) bool {
	kind, ok := data.NullKind(v)
	return ok && kind != data.KindNull
}

// applyDyadicLogical implements canonical q `&`/`and` (elementwise minimum)
// and `|`/`or` (elementwise maximum). On booleans min/max IS logical and/or,
// so strictly-boolean operands keep their boolean result. Nulls follow the
// canonical sort order (nulls are smallest): the minimum against a null is
// the null, the maximum is the other operand. Mixed bool/numeric operands
// promote the boolean to its integer value per q type promotion.
func applyDyadicLogical(op byte, left, right any) (any, error) {
	if op != '&' && op != '|' {
		return nil, fmt.Errorf("operator %q is not a logical verb", string(op))
	}
	if data.IsNull(left) || qIsNaNScalar(left) {
		if op == '&' {
			return left, nil
		}
		return qMinMaxPromoteAgainstNull(right, left), nil
	}
	if data.IsNull(right) || qIsNaNScalar(right) {
		if op == '&' {
			return right, nil
		}
		return qMinMaxPromoteAgainstNull(left, right), nil
	}
	lb, leftIsBool := left.(bool)
	rb, rightIsBool := right.(bool)
	if leftIsBool && rightIsBool {
		if op == '&' {
			return lb && rb, nil
		}
		return lb || rb, nil
	}
	if leftIsBool {
		left = qBoolToInt64(lb)
	}
	if rightIsBool {
		right = qBoolToInt64(rb)
	}
	cmp, err := compareOrdered(left, right)
	if err != nil {
		return nil, err
	}
	if (op == '&' && cmp <= 0) || (op == '|' && cmp >= 0) {
		return left, nil
	}
	return right, nil
}

func qBoolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// qMinMaxPromoteAgainstNull returns the surviving max operand, promoting a
// boolean to its long value when the losing null is not a boolean null
// (canonical q: 0N|0b is 0, while 0Nb|0b stays 0b).
func qMinMaxPromoteAgainstNull(winner, null any) any {
	b, isBool := winner.(bool)
	if !isBool {
		return winner
	}
	if kind, ok := data.NullKind(null); ok && kind == data.KindBool {
		return winner
	}
	if !data.IsNull(null) {
		// NaN float null: promote to float per q widening.
		return float64(qBoolToInt64(b))
	}
	return qBoolToInt64(b)
}

// qIsNaNScalar reports float NaN operands so `&`/`|` rank them with the
// nulls (canonical q sorts 0n below every other float).
func qIsNaNScalar(v any) bool {
	switch x := v.(type) {
	case float64:
		return math.IsNaN(x)
	case float32:
		return math.IsNaN(float64(x))
	default:
		return false
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
		return qPromoteMinMaxResult(left, right), nil
	}
	return qPromoteMinMaxResult(right, left), nil
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
		return qPromoteMinMaxResult(left, right), nil
	}
	return qPromoteMinMaxResult(right, left), nil
}

// qPromoteMinMaxResult applies canonical q type promotion to a min/max
// winner: long & float is FLOAT regardless of which operand wins (0 min .0
// is 0f), matching the typed float kernels so generic and fused routes agree.
func qPromoteMinMaxResult(winner, other any) any {
	wi, ok := integerValue(winner)
	if !ok {
		return winner
	}
	switch other.(type) {
	case float64:
		return float64(wi)
	case float32:
		return float32(wi)
	}
	return winner
}

// equalsDictOrCallable handles q `=` (and the `<>` complement through
// invertBoolResult) over operands the scalar/vector kernels cannot compare:
// dictionaries and callables are uncomparable Go types, so reaching the
// kernels' interface == panics ("comparing uncomparable type").
//
//   - dict = dict is the canonical per-key comparison: a dict of booleans
//     over the union of the keys (left key order first, then unseen right
//     keys). Keys present on one side only compare against null and yield 0b,
//     mirroring 1=0N.
//   - dict = atom broadcasts the atom over the dict values, keeping the keys.
//   - callable = callable is match-style identity (the same rule ~ uses).
//
// Dict-vs-vector and callable-vs-non-callable stay unhandled so the existing
// dispatch keeps its errors/fallbacks.
func equalsDictOrCallable(left, right any) (any, bool, error) {
	leftDict, leftIsDict := left.(EvalDict)
	rightDict, rightIsDict := right.(EvalDict)
	switch {
	case leftIsDict && rightIsDict:
		out, err := dictEqualsDict(leftDict, rightDict)
		return out, true, err
	case leftIsDict:
		if _, isArray := right.(data.Array); isArray {
			return nil, false, nil
		}
		out, err := dictEqualsAtom(leftDict, right)
		return out, true, err
	case rightIsDict:
		if _, isArray := left.(data.Array); isArray {
			return nil, false, nil
		}
		out, err := dictEqualsAtom(rightDict, left)
		return out, true, err
	}
	if isCallable(left) && isCallable(right) {
		return matchValue(left, right), true, nil
	}
	return nil, false, nil
}

func dictEqualsDict(left, right EvalDict) (EvalDict, error) {
	keys := make([]any, 0, len(left.Keys))
	values := make([]any, 0, len(left.Keys))
	for i, key := range left.Keys {
		keys = append(keys, key)
		j, found := dictKeyIndex(right, key)
		if !found {
			values = append(values, false)
			continue
		}
		eq, err := applyDyadic('=', left.Values[i], right.Values[j])
		if err != nil {
			return EvalDict{}, err
		}
		values = append(values, eq)
	}
	for _, key := range right.Keys {
		if _, found := dictKeyIndex(left, key); found {
			continue
		}
		keys = append(keys, key)
		values = append(values, false)
	}
	return EvalDict{Keys: keys, Values: values}, nil
}

func dictEqualsAtom(d EvalDict, scalar any) (EvalDict, error) {
	values := make([]any, len(d.Values))
	for i, value := range d.Values {
		eq, err := applyDyadic('=', value, scalar)
		if err != nil {
			return EvalDict{}, err
		}
		values[i] = eq
	}
	return EvalDict{Keys: append([]any(nil), d.Keys...), Values: values}, nil
}

func dictKeyIndex(d EvalDict, key any) (int, bool) {
	for i, existing := range d.Keys {
		if equalValue(existing, key) {
			return i, true
		}
	}
	return 0, false
}

func applyCompositeDyadic(op string, left, right any) (any, error) {
	if op == "<>" {
		if out, handled, err := equalsDictOrCallable(left, right); handled || err != nil {
			if err != nil {
				return nil, err
			}
			return invertBoolResult(out)
		}
	}
	if dataOp, ok := qDataCompositeComparisonOp(op); ok {
		la, lok := left.(data.Array)
		ra, rok := right.(data.Array)
		if lok || rok {
			if out, handled, err := qTryTypedRuntimeVectorCompareDyadic(op, dataOp, left, right, la, ra, true); err != nil || handled {
				if err != nil {
					return nil, err
				}
				return out, nil
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
	case EvalDict:
		values := make([]any, len(x.Values))
		for i, value := range x.Values {
			inverted, err := invertBoolResult(value)
			if err != nil {
				return nil, err
			}
			values[i] = inverted
		}
		return EvalDict{Keys: x.Keys, Values: values}, nil
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

// qTryTiledCycleVectorDyadic pushes an elementwise dyadic verb with one
// scalar operand onto the short cycle of a cyclic-take carrier (n#xs) and
// re-tiles the result, so pattern vectors pay per-cycle instead of per-row
// cost. Semantics match the per-row fallback exactly because the verb is
// applied through the same applyVectorDyadic pipeline on the cycle.
func qTryTiledCycleVectorDyadic(op byte, left, right any, la, ra data.Array) (data.Array, bool, error) {
	var tiled data.Array
	var scalar any
	scalarLeft := false
	switch {
	case la != nil && ra == nil:
		tiled, scalar = la, right
	case ra != nil && la == nil:
		tiled, scalar, scalarLeft = ra, left, true
	default:
		return nil, false, nil
	}
	source, start, length, ok := data.TiledCycleView(tiled)
	if !ok || source.Len() == 0 || source.Len() >= length {
		return nil, false, nil
	}
	shape := "tiled-cycle/" + string(op) + "/" + string(source.Kind())
	var inner data.Array
	var err error
	if scalarLeft {
		inner, err = applyVectorDyadic(op, scalar, source, nil, source)
	} else {
		inner, err = applyVectorDyadic(op, source, scalar, source, nil)
	}
	if err != nil {
		_, _, _ = qTypedRuntimeResult[data.Array]("ArrayTiledCycleDyadic", shape, nil, true, err)
		return nil, true, err
	}
	if inner == nil || inner.Len() != source.Len() {
		recordRuntimeKernelProbeReason("ArrayTiledCycleDyadic", shape, false, nil, RuntimeFallbackUnsupportedType)
		return nil, false, nil
	}
	out, ok := data.NewTiledCycleView(inner, start, length)
	if !ok {
		recordRuntimeKernelProbeReason("ArrayTiledCycleDyadic", shape, false, nil, RuntimeFallbackUnsupportedType)
		return nil, false, nil
	}
	out2, handled, err := qTypedRuntimeResult("ArrayTiledCycleDyadic", shape, out, true, nil)
	if err != nil || !handled {
		return nil, false, err
	}
	return out2, true, nil
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
	// left/right word verbs are elementwise operand selectors: with matching
	// shapes the result is the selected operand itself (each row of the
	// generic loop returns that operand's element unchanged), so alias it
	// instead of rebuilding a boxed copy row by row. Scalar-broadcast and
	// length-1 shapes keep the generic route.
	if op == 'L' || op == 'R' {
		if la != nil && ra != nil && la.Len() == ra.Len() {
			if op == 'L' {
				return la, nil
			}
			return ra, nil
		}
		if op == 'L' && la != nil && ra == nil {
			return la, nil
		}
		if op == 'R' && ra != nil && la == nil {
			return ra, nil
		}
	}
	if out, handled, err := qTryTiledCycleVectorDyadic(op, left, right, la, ra); err != nil || handled {
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	if op == '&' || op == '|' {
		logical := "and"
		if op == '|' {
			logical = "or"
		}
		// Canonical `&`/`|` are elementwise min/max. On booleans min/max IS
		// logical and/or, so strictly-boolean operands keep the lazy logical
		// mask carrier and the periodic where/count fast paths.
		if qLogicalOperandCouldBeBool(left, la) && qLogicalOperandCouldBeBool(right, ra) {
			shape := "bool-logical/" + logical + "/" + string(qRuntimeKernelOperandKind(left, la)) + "/" + string(qRuntimeKernelOperandKind(right, ra))
			if out, handled, err := qTryTypedRuntimeBoolLogical(logical, shape, left, right); err != nil || handled {
				if err != nil {
					return nil, err
				}
				if handled {
					if array, ok := out.(data.Array); ok {
						return array, nil
					}
				}
			}
		}
		// Numeric operands materialize the typed elementwise min/max; null
		// carriers fall through to the boxed loop, which applies canonical
		// null ordering per row (null&x is null, null|x is x).
		if out, handled, err := qTryTypedRuntimeDyadicMinMax(logical, op == '|', left, right, la, ra); err != nil || handled {
			if err != nil {
				return nil, err
			}
			if handled {
				if array, ok := out.(data.Array); ok {
					return array, nil
				}
			}
		}
	}
	if op == '^' {
		if la == nil && ra != nil {
			out, handled, err := qTryTypedRuntimeScalarFill(left, ra)
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
		}
		if la != nil && ra == nil {
			out, handled, err := qTryTypedRuntimeScalarFill(right, la)
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
		}
	}
	if op == 'd' {
		if out, handled, err := qTryTypedRuntimeIntegerFloorDivide(op, left, right, la, ra, n); err != nil || handled {
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
		}
	}
	if op == '+' || op == '-' {
		dataOp := data.OpAdd
		if op == '-' {
			dataOp = data.OpSub
		}
		if out, handled, err := qTryTypedRuntimeTemporalDyadic(op, dataOp, left, right, la, ra); err != nil || handled {
			if err != nil {
				return nil, err
			}
			if handled {
				if array, ok := out.(data.Array); ok {
					return array, nil
				}
			}
		}
	}
	if dataOp, ok := qDataArithmeticOp(op); ok {
		out, handled, err := qTryTypedRuntimeVectorArithmeticDyadic(op, dataOp, left, right, la, ra, true)
		if err != nil {
			return nil, err
		}
		if handled {
			if array, ok := out.(data.Array); ok {
				return array, nil
			}
		}
	}
	if dataOp, ok := qDataComparisonOp(op); ok {
		out, handled, err := qTryTypedRuntimeVectorCompareDyadic(string(op), dataOp, left, right, la, ra, true)
		if err != nil {
			return nil, err
		}
		if handled {
			if array, ok := out.(data.Array); ok {
				return array, nil
			}
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
	if op != data.OpDiv && qTypedIntegerOperandOK(left) && qTypedIntegerOperandOK(right) {
		return data.TryTypedIntegerDyadic(op, left, right)
	}
	return data.TryTypedDyadic(op, left, right)
}

func qTryTypedCompareMask(op data.Op, left, right any, la, ra data.Array) (data.Array, bool, error) {
	// Generic (any) and empty () lists keep the cascade's per-element
	// comparison: its empty result stays kind-null and its error text is
	// canonical, so every typed kernel must decline them up front.
	if la != nil && (la.Kind() == data.KindAny || la.Kind() == data.KindNull) {
		return nil, false, nil
	}
	if ra != nil && (ra.Kind() == data.KindAny || ra.Kind() == data.KindNull) {
		return nil, false, nil
	}
	if out, handled, err := data.TryTypedDyadic(op, left, right); err != nil || handled {
		if err != nil {
			return nil, handled, err
		}
		array, ok := out.(data.Array)
		return array, ok, nil
	}
	switch {
	case la != nil && ra == nil:
		if la.Kind() == data.KindAny || la.Kind() == data.KindNull {
			// Generic and empty () lists keep the cascade's per-element
			// comparison (its empty result stays kind-null, and its error
			// text is canonical); the typed mask kernel must decline.
			return nil, false, nil
		}
		out, err := data.CompareMask(la, op, right)
		if err != nil {
			// Decline on kernel errors too: the generic route raises the
			// cascade's canonical error text for incomparable operands.
			return nil, false, nil
		}
		return out, true, nil
	case la == nil && ra != nil:
		if ra.Kind() == data.KindAny || ra.Kind() == data.KindNull {
			return nil, false, nil
		}
		reversed, ok := reverseDataCompareOp(op)
		if !ok {
			return nil, false, nil
		}
		out, err := data.CompareMask(ra, reversed, left)
		if err != nil {
			return nil, false, nil
		}
		return out, true, nil
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

func qLogicalOperandCouldBeBool(value any, array data.Array) bool {
	if array != nil {
		return array.Kind() == data.KindBool
	}
	if a, ok := value.(data.Array); ok {
		return a.Kind() == data.KindBool
	}
	if _, ok := value.(bool); ok {
		return true
	}
	if kind, ok := data.NullKind(value); ok {
		return kind == data.KindBool
	}
	return false
}

// qVectorDyadicHasTemporalOperand reports whether either broadcast operand
// carries a temporal kind, gating the kind-preserving temporal dyadic kernel.
func qVectorDyadicHasTemporalOperand(left, right any) bool {
	return qVectorDyadicOperandIsTemporal(left) || qVectorDyadicOperandIsTemporal(right)
}

func qVectorDyadicOperandIsTemporal(value any) bool {
	if array, ok := value.(data.Array); ok {
		return data.IsTemporalKind(array.Kind())
	}
	return temporalKindOfValue(value) != ""
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
	case '=', '<', '>', '~':
		return data.KindBool
	case '%':
		return data.KindF64
	case '+', '-', '*', 'r', 'd':
		leftKind := qKindOfValue(left)
		rightKind := qKindOfValue(right)
		if kind, ok := temporalDyadicResultKind(op, leftKind, rightKind); ok {
			return kind
		}
		if qKindIsNumeric(leftKind) || qKindIsNumeric(rightKind) {
			kind, ok := mergeQResultKinds(leftKind, rightKind)
			if ok {
				return kind
			}
		}
	case 'm', 'M', '^', '&', '|':
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
	// Canonical enlist on an atom yields a typed one-item vector
	// (enlist 1 ~ 1#1); containers keep the generic one-item list.
	switch v.(type) {
	case data.Array, string, EvalDict, data.Frame, data.KeyedFrame:
		return data.NewAny([]any{v}), nil
	}
	if isCallable(v) {
		return data.NewAny([]any{v}), nil
	}
	if kind := qKindOfValue(v); kind != "" && kind != data.KindNull && kind != data.KindAny {
		return inferQArray([]any{v}, kind), nil
	}
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
	case bool:
		// Canonical q: string of a boolean is its digit ("1"/"0").
		if x {
			return "1"
		}
		return "0"
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
	// Canonical q: upper/lower are type-preserving on symbols (`upper `ab
	// is `AB, not "AB"); symbol atoms and symbol vectors stay symbols.
	if name == "upper" || name == "lower" {
		if sym, ok := v.(data.Symbol); ok {
			return data.Symbol(fn(string(sym))), nil
		}
	}
	if array, ok := v.(data.Array); ok {
		if name == "upper" || name == "lower" {
			if array.Kind() == data.KindSymbol {
				out := make([]string, array.Len())
				for i, item := range array.Values() {
					switch sym := item.(type) {
					case data.Symbol:
						out[i] = fn(string(sym))
					case string:
						out[i] = fn(sym)
					default:
						return nil, fmt.Errorf("%s symbol row %d is not a symbol", name, i+1)
					}
				}
				return data.NewSymbols(out), nil
			}
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
	v = dictAggregateArgument(v)
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("sum expects a numeric vector")
	}
	if array.Len() == 0 {
		// Canonical empty-sum identity: sum () -> 0 (typed kernels return
		// the same zero for empty typed vectors).
		return qSumIdentityForKind(array.Kind()), nil
	}
	if out, handled, err := qColumnarAggregate("+", array); handled || err != nil {
		return out, err
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

// qColumnarAggregate folds a generic list of conformable vectors pairwise
// with the given dyadic verb: sum (1 2;3 4) is 4 6 in canonical q (atoms
// broadcast). It only engages when the list holds at least one vector item;
// flat numeric lists keep the scalar reduction path.
func qColumnarAggregate(op string, array data.Array) (any, bool, error) {
	if array.Kind() != data.KindAny || array.Len() == 0 {
		return nil, false, nil
	}
	items := array.Values()
	hasVector := false
	for _, item := range items {
		if _, ok := item.(data.Array); ok {
			hasVector = true
			break
		}
	}
	if !hasVector {
		return nil, false, nil
	}
	fn, ok := lookupDyadicVerbFunc(op)
	if !ok {
		return nil, false, nil
	}
	acc := items[0]
	for _, item := range items[1:] {
		out, err := fn(acc, item)
		if err != nil {
			return nil, true, err
		}
		acc = out
	}
	return acc, true, nil
}

func avg(v any) (any, error) {
	v = dictAggregateArgument(v)
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
	return devFromVariance(variance)
}

// devFromVariance is devValue's tail after the variance computation, shared
// with the add-chain var+dev pairing so both routes produce identical bits.
func devFromVariance(variance any) (any, error) {
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
	v = dictAggregateArgument(v)
	if n, ok := numeric(v); ok {
		return n, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("med expects a numeric vector")
	}
	// Range fast path: an affine walk is already sorted (ascending for
	// step>=0, descending otherwise), so the median indexes are closed-form.
	// The selected values are exactly the float64 conversions the sort-based
	// route below would place at mid-1/mid, and the even-count midpoint uses
	// the same (a+b)/2 expression, so results are bit-identical.
	if start, step, n, ok := data.I64RangeView(array); ok && n > 0 {
		at := func(i int) float64 { return float64(start + int64(i)*step) }
		mid := n / 2
		lo, hi := mid-1, mid
		if step < 0 {
			lo, hi = n-1-(mid-1), n-1-mid
		}
		if n%2 == 1 {
			return at(hi), nil
		}
		return (at(lo) + at(hi)) / 2, nil
	}
	if bulk, owned, ok := data.TryBulkF64(array); ok {
		values := bulk
		if !owned {
			// Owned bulk buffers are scratch and may be sorted in place;
			// unowned slices alias column storage and must be copied first.
			values = make([]float64, len(bulk))
			copy(values, bulk)
		}
		if len(values) == 0 {
			data.BulkF64Release(bulk, owned)
			return data.NullValue, nil
		}
		sort.Float64s(values)
		mid := len(values) / 2
		out := values[mid]
		if len(values)%2 == 0 {
			out = (values[mid-1] + values[mid]) / 2
		}
		data.BulkF64Release(bulk, owned)
		return out, nil
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
	// Range fast path: stream the affine walk without materializing it. The
	// float64 conversions and accumulation order match the bulk-flatten loop
	// below bit for bit.
	if start, step, count, ok := data.I64RangeView(array); ok {
		total := float64(0)
		sumsq := float64(0)
		value := start
		for i := 0; i < count; i++ {
			n := float64(value)
			total += n
			sumsq += n * n
			value += step
		}
		return total, sumsq, count, nil
	}
	if values, owned, ok := data.TryBulkF64(array); ok {
		total := float64(0)
		sumsq := float64(0)
		for _, n := range values {
			total += n
			sumsq += n * n
		}
		count := len(values)
		data.BulkF64Release(values, owned)
		return total, sumsq, count, nil
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
	if weightIsArray && valueIsArray {
		// Range-range fast path: stream both affine walks; same per-row
		// products and accumulation order as the bulk loop below.
		if wStart, wStep, wLen, ok := data.I64RangeView(weightArray); ok {
			if vStart, vStep, vLen, ok := data.I64RangeView(valueArray); ok && vLen == wLen {
				total := float64(0)
				denom := float64(0)
				wv, vv := wStart, vStart
				for i := 0; i < wLen; i++ {
					w := float64(wv)
					total += w * float64(vv)
					denom += w
					wv += wStep
					vv += vStep
				}
				if denom == 0 {
					return data.NullValue, nil
				}
				return total / denom, nil
			}
		}
		if weightBulk, weightOwned, ok := data.TryBulkF64(weightArray); ok {
			if valueBulk, valueOwned, ok := data.TryBulkF64(valueArray); ok && len(valueBulk) == len(weightBulk) {
				total := float64(0)
				denom := float64(0)
				for i, w := range weightBulk {
					total += w * valueBulk[i]
					denom += w
				}
				data.BulkF64Release(weightBulk, weightOwned)
				data.BulkF64Release(valueBulk, valueOwned)
				if denom == 0 {
					return data.NullValue, nil
				}
				return total / denom, nil
			} else if ok {
				data.BulkF64Release(valueBulk, valueOwned)
			}
			data.BulkF64Release(weightBulk, weightOwned)
		}
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
	v = dictAggregateArgument(v)
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("prd expects a numeric vector")
	}
	if array.Len() == 0 {
		// Canonical empty-product identity: prd () -> 1.
		return qProductIdentityForKind(array.Kind()), nil
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
		// Canonical q: prd ignores nulls, so an all-null vector is the empty
		// product 1 (prd 0N 0N -> 1), mirroring sum 0N 0N -> 0.
		return qProductIdentityForKind(array.Kind()), nil
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
	if typed, handled, err := data.TryTypedQRatios(array); handled || err != nil {
		recordRuntimeKernelProbe("ArrayRatios", "vector-scan/ratios/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, fmt.Errorf("ratios: %w", err)
		}
		return typed, nil
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
	return qDataNumericUnary(data.NumericUnaryNeg, v)
}

func absValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryAbs, v)
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

func allValue(v any) (any, error) {
	return boolAggregate("all", v, true, func(acc, item bool) bool { return acc && item })
}

func anyValue(v any) (any, error) {
	return boolAggregate("any", v, false, func(acc, item bool) bool { return acc || item })
}

func boolAggregate(name string, v any, initial bool, fn func(bool, bool) bool) (any, error) {
	v = dictAggregateArgument(v)
	array, ok := v.(data.Array)
	if !ok {
		b, err := boolValue(v)
		if err != nil {
			return nil, fmt.Errorf("%s expects bool or numeric values", name)
		}
		return b, nil
	}
	shape := "vector-reduce/" + name + "/" + string(array.Kind())
	typed, handled, err := data.TryTypedBoolAggregate(array, name == "any")
	typed, handled, err = qTypedRuntimeResult("ArrayBoolAggregate", shape, typed, handled, err)
	if err != nil || handled {
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return typed, nil
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

// keyVerb implements canonical `key` where it diverges from `keys`: key of a
// keyed table is its key TABLE (keys returns the key column names) and key of
// a typed vector is its type-name symbol. Dicts and the dialect introspection
// wrappers share the keys behavior.
func keyVerb(v any) (any, error) {
	switch x := v.(type) {
	case qAttributedVector, qEnumVector, EvalDict:
		return keys(v)
	case data.KeyedFrame:
		return x.KeyFrame()
	case data.Array:
		if name, ok := qTypeNameForKind(x.Kind()); ok {
			return name, nil
		}
	}
	return keys(v)
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

// value is the stateless slice of the `value` verb (dict/enum/attributed/
// keyed dispatch plus atom identity). The session-dependent cases — string
// source evaluation, symbol dereference, parse-tree evaluation — live in
// (*EvalState).valueVerb (value_eval_parse.go); every dispatch route with a
// session in scope goes through it, so the errors below only surface from a
// route that cannot see the env (and never silently pass the input through).
// KNOWN GAP: canonical q value on a lambda returns its metadata list; this
// dialect returns the lambda unchanged (default case).
func value(v any) (any, error) {
	switch x := v.(type) {
	case qAttributedVector:
		return x.vector, nil
	case qEnumVector:
		return x.decodedArray(), nil
	case EvalDict:
		// Canonical q: value of a dict is its value LIST as stored —
		// homogeneous values compact to a typed vector (value `a`b!1 2 is
		// 1 2j, not a generic list), mixed values stay generic.
		return data.InferArray(x.Values), nil
	case data.KeyedFrame:
		return x.ValueFrame()
	case string:
		return nil, fmt.Errorf("value: evaluating a source string requires the session evaluator")
	case data.Symbol:
		return nil, fmt.Errorf("value: dereferencing a symbol requires the session evaluator")
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
	v = dictAggregateArgument(v)
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if array.Len() == 0 {
		// Canonical empty-vector identities: min () -> 0W, max () -> -0W
		// (per element type), matching the derived &/ |/ reducers. All-null
		// non-empty vectors still reduce to null below.
		if identity, ok := qMinMaxIdentityForKind(wantMax, array.Kind()); ok {
			return identity, nil
		}
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

// qMaxListLength caps eagerly allocated replication results (where on
// integer counts, take from empty lists): kdb+ errors ('limit/'wsfull) once a
// list outgrows the workspace, and counts near 0W otherwise overflow Go's
// makeslice ("len out of range" panic).
const qMaxListLength = math.MaxInt32

func where(v any) (any, error) {
	if d, ok := v.(EvalDict); ok {
		return dictWhere(d)
	}
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
		if n > qMaxListLength {
			return nil, fmt.Errorf("where count %d exceeds the %d list limit", n, int64(qMaxListLength))
		}
		return data.NewI64(make([]int64, n)), nil
	}
	if array.Kind() == data.KindBool {
		typedOut, handled, err := qEvalWhereMaskI64Primitive(array)
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
	// Replication over a dense integer carrier: validate and size the result
	// from the bulk values instead of boxing every row through Values().
	if values, owned, ok := data.TryBulkI64(array); ok {
		total, err := qWhereReplicationTotal(values)
		if err != nil {
			data.BulkI64Release(values, owned)
			return nil, err
		}
		out := make([]int64, 0, total)
		for i, n := range values {
			for j := int64(0); j < n; j++ {
				out = append(out, int64(i))
			}
		}
		data.BulkI64Release(values, owned)
		return data.NewI64(out), nil
	}
	var out []int64
	total := int64(0)
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
		if n > qMaxListLength-total {
			return nil, fmt.Errorf("where count %d exceeds the %d list limit", n, int64(qMaxListLength))
		}
		total += n
		for j := int64(0); j < n; j++ {
			out = append(out, int64(i))
		}
	}
	return data.NewI64(out), nil
}

// qWhereReplicationTotal validates where's replication counts and returns
// their sum, with the exact errors of the boxed row loop.
func qWhereReplicationTotal(values []int64) (int64, error) {
	total := int64(0)
	for _, n := range values {
		if n < 0 {
			return 0, fmt.Errorf("where expects non-negative integer counts")
		}
		if n > qMaxListLength-total {
			return 0, fmt.Errorf("where count %d exceeds the %d list limit", n, int64(qMaxListLength))
		}
		total += n
	}
	return total, nil
}

func whereFilterValue(left, right any) (any, error) {
	switch left.(type) {
	case data.Frame, data.KeyedFrame:
		return nil, fmt.Errorf("where filter for tables is not supported in q eval; use q.sql where")
	}
	indexes, err := where(right)
	if err != nil {
		return nil, err
	}
	return indexValue(left, indexes)
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
	desc, ok, err := qTypedWhereCompareIndexesDescriptor(left, right, op, "compare-to-index", "within-to-index")
	if err != nil || !ok {
		return nil, ok, err
	}
	out, handled, err := evalQTypedWhereCompareIndexes(desc)
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
	// Canonical right-to-left: a top-level & or | binds the comparison on its
	// right (`(x>1)&x<9` is (x>1)&(x<9)); splitting at the comparison here
	// would mis-group, so decline and let the general routes evaluate.
	if _, _, ok := splitTopLevelOperator(src, "&"); ok {
		return "", "", "", false
	}
	if _, _, ok := splitTopLevelOperator(src, "|"); ok {
		return "", "", "", false
	}
	// Same yield for the cascade splits that claim BEFORE the comparison
	// dyadics (join, take, drop/cut, cast, dict-bang, apply, match, find):
	// `0 0<count $A00000` is the cast ((0 0<count)$A00000), not a compare.
	if findTopLevel(src, ",#_$!@?~") >= 0 {
		return "", "", "", false
	}
	// The word-map split also precedes the symbol comparisons in the
	// cascade: a registered dyadic word claims first (`0 0<count where 0` is
	// (0 0<count) where 0). within keeps its compare spelling; every other
	// word declines the compare probe. A declined word map (leftmost word
	// with an empty side, e.g. a binding named `times`) mirrors the
	// cascade's decline and keeps the compare/within probes below.
	if op, left, right, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps); ok {
		if op.word == "within" {
			return left, right, "within", true
		}
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
		if out, handled, err := data.TryTypedIsNullMask(array); err != nil || handled {
			recordRuntimeKernelProbe("ArrayIsNull", "is-null/"+string(array.Kind()), handled, err)
			if err != nil {
				return nil, err
			}
			return out, nil
		} else {
			recordRuntimeKernelProbeReason("ArrayIsNull", "is-null/"+string(array.Kind()), handled, err, RuntimeFallbackUnsupportedType)
		}
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

// logicalAnd / logicalOr are the word forms of canonical `&` (min) and `|`
// (max); they share the verb dispatch so every route agrees.
func logicalAnd(left, right any) (any, error) {
	return applyDyadic('&', left, right)
}

func logicalOr(left, right any) (any, error) {
	return applyDyadic('|', left, right)
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

// splitLeadingTickCast splits a leading `word$ cast prefix off src.
func splitLeadingTickCast(src string) (word, rest string, ok bool) {
	if len(src) < 3 || src[0] != '`' {
		return "", "", false
	}
	i := 1
	for i < len(src) && (src[i] >= 'a' && src[i] <= 'z') {
		i++
	}
	if i == 1 || i >= len(src) || src[i] != '$' {
		return "", "", false
	}
	return src[1:i], strings.TrimSpace(src[i+1:]), true
}

// tryEvalCastChainSum fuses +/`k_n$...`k_1$<expr> integer cast chains into a
// single pass through data.TryTypedIntCastChainSum, skipping the per-cast
// column materializations. Declines (non-integer kinds, out-of-range rows,
// non-array values) keep the staged cast-then-sum route unchanged.
func (s *EvalState) tryEvalCastChainSum(src string) (any, bool, error) {
	var kinds []data.Kind
	rest := src
	for {
		word, after, ok := splitLeadingTickCast(rest)
		if !ok {
			break
		}
		kind, ok := qCastKindFromTypeText(word)
		if !ok {
			return nil, false, nil
		}
		kinds = append(kinds, kind)
		rest = after
	}
	if len(kinds) < 2 || rest == "" {
		return nil, false, nil
	}
	// kinds were collected outermost-first; the kernel applies innermost-first.
	for i, j := 0, len(kinds)-1; i < j; i, j = i+1, j-1 {
		kinds[i], kinds[j] = kinds[j], kinds[i]
	}
	value, err := s.eval(rest)
	if err != nil {
		return nil, false, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	out, handled := data.TryTypedIntCastChainSum(kinds, array)
	recordRuntimeKernelProbe("ArrayIntCastChainSum", "vector-reduce/cast-chain-sum/"+string(array.Kind()), handled, nil)
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

// tryEvalFindSum fuses +/domain?query into a single pass through
// data.TryTypedFindComparableSum, skipping the index-vector materialization the
// staged find-then-sum route pays. Non-array operands and kernel declines
// keep the staged route (the find cascade re-evaluates the expression).
func (s *EvalState) tryEvalFindSum(src string) (any, bool, error) {
	leftExpr, rightExpr, ok := splitTopLevelOperator(src, "?")
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, false, nil
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, false, nil
	}
	desc, ok := qTypedFindDescriptorFor(left, right)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := evalQTypedFindSum(desc)
	if err != nil {
		return nil, false, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

// randSource returns the per-session PRNG behind roll/deal/rand, lazily
// seeded with the fixed default seed so fresh sessions are reproducible.
func (s *EvalState) randSource() *rand.Rand {
	if s.rng == nil {
		s.rng = rand.New(rand.NewSource(qDefaultRandSeed))
	}
	return s.rng
}

// qRollDealCount reports whether left is an integer atom, which makes a
// dyadic `?` a roll (x>=0) or deal (x<0) instead of find.
func qRollDealCount(left any) (int64, bool) {
	switch x := left.(type) {
	case int64:
		return x, true
	case int32:
		return int64(x), true
	case int16:
		return int64(x), true
	case int:
		return int64(x), true
	default:
		return 0, false
	}
}

// rollOrDeal implements canonical q `x?y` with integer-atom x:
//   - x?y (y int atom):  x random ints in [0,y); y=0 rolls full-range longs
//   - x?y (y float atom): x random floats in [0,y)
//   - x?y (y list/string): x random picks from y (with replacement)
//   - negative x deals: |x| DISTINCT draws (errors when |x| exceeds the
//     domain size, q's 'length)
//
// Draws come from the per-session seeded PRNG (see randSource), so results
// are reproducible per fresh session and nondeterministic within a session.
func (s *EvalState) rollOrDeal(n int64, right any) (any, error) {
	deal := n < 0
	count64 := n
	if deal {
		count64 = -count64
	}
	if int64(int(count64)) != count64 {
		return nil, fmt.Errorf("roll count is too large")
	}
	count := int(count64)
	rng := s.randSource()
	switch y := right.(type) {
	case int64, int32, int16, int:
		domain, _ := qRollDealCount(y)
		if domain < 0 {
			return nil, fmt.Errorf("roll domain must be non-negative")
		}
		if domain == 0 {
			if deal {
				return nil, fmt.Errorf("deal domain must be positive")
			}
			out := make([]int64, count)
			for i := range out {
				out[i] = int64(rng.Uint64())
			}
			return data.NewI64(out), nil
		}
		if deal {
			if count64 > domain {
				return nil, fmt.Errorf("deal count %d exceeds domain %d", count, domain)
			}
			return data.NewI64(dealDistinctInts(rng, count, domain)), nil
		}
		out := make([]int64, count)
		for i := range out {
			out[i] = rng.Int63n(domain)
		}
		return data.NewI64(out), nil
	case float64:
		if deal {
			return nil, fmt.Errorf("deal expects an integer or list domain")
		}
		if y <= 0 {
			return nil, fmt.Errorf("roll domain must be positive")
		}
		out := make([]float64, count)
		for i := range out {
			out[i] = rng.Float64() * y
		}
		return data.NewF64(out), nil
	case float32:
		return s.rollOrDeal(n, float64(y))
	case data.Array:
		length := y.Len()
		if length == 0 {
			return nil, fmt.Errorf("roll list domain is empty")
		}
		var indexes []int64
		if deal {
			if count > length {
				return nil, fmt.Errorf("deal count %d exceeds list length %d", count, length)
			}
			indexes = dealDistinctInts(rng, count, int64(length))
		} else {
			indexes = make([]int64, count)
			for i := range indexes {
				indexes[i] = rng.Int63n(int64(length))
			}
		}
		out := make([]any, count)
		for i, index := range indexes {
			value, ok := y.At(int(index))
			if !ok {
				return nil, fmt.Errorf("roll list row %d out of range", index)
			}
			out[i] = value
		}
		return inferQArray(out, y.Kind()), nil
	case string:
		runes := []rune(y)
		if len(runes) == 0 {
			return nil, fmt.Errorf("roll string domain is empty")
		}
		var indexes []int64
		if deal {
			if count > len(runes) {
				return nil, fmt.Errorf("deal count %d exceeds string length %d", count, len(runes))
			}
			indexes = dealDistinctInts(rng, count, int64(len(runes)))
		} else {
			indexes = make([]int64, count)
			for i := range indexes {
				indexes[i] = rng.Int63n(int64(len(runes)))
			}
		}
		out := make([]rune, count)
		for i, index := range indexes {
			out[i] = runes[index]
		}
		return string(out), nil
	default:
		return nil, fmt.Errorf("roll/deal right operand %T is not supported", right)
	}
}

// dealDistinctInts draws count distinct int64s from [0,domain) in random
// order (Floyd's algorithm plus a final shuffle).
func dealDistinctInts(rng *rand.Rand, count int, domain int64) []int64 {
	chosen := make(map[int64]struct{}, count)
	out := make([]int64, 0, count)
	for v := domain - int64(count); v < domain; v++ {
		candidate := rng.Int63n(v + 1)
		if _, taken := chosen[candidate]; taken {
			candidate = v
		}
		chosen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// evalRand implements the unary `rand x` verb: a single draw, i.e. first 1?x.
func (s *EvalState) evalRand(right any) (any, error) {
	out, err := s.rollOrDeal(1, right)
	if err != nil {
		return nil, err
	}
	if array, ok := out.(data.Array); ok && array.Len() == 1 {
		value, _ := array.At(0)
		return value, nil
	}
	if str, ok := out.(string); ok && len(str) > 0 {
		return string([]rune(str)[0]), nil
	}
	return out, nil
}

func findValue(left, right any) (any, error) {
	if dict, ok := left.(EvalDict); ok {
		return dictFindValue(dict, right)
	}
	if desc, ok := qTypedFindDescriptorFor(left, right); ok {
		out, handled, err := evalQTypedFind(desc)
		if err != nil {
			return nil, err
		}
		if handled {
			return out, nil
		}
	}
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

// dictFindValue implements canonical q `?` on a dictionary: dict?value is a
// reverse lookup returning the first key whose value matches; a missing value
// returns the null of the key type (`(`a`b!1 2)?9` -> `).
func dictFindValue(dict EvalDict, right any) (any, error) {
	var queries []any
	var scalar bool
	if s, ok := right.(string); ok {
		// A string is a scalar dictionary value, not a char-vector of queries.
		queries, scalar = []any{s}, true
	} else {
		var err error
		queries, scalar, err = findQueryValues(right)
		if err != nil {
			return nil, err
		}
	}
	missing := dictFindMissingKey(dict)
	out := make([]any, len(queries))
	for i, query := range queries {
		out[i] = missing
		for j, candidate := range dict.Values {
			if equalValue(candidate, query) {
				out[i] = dict.Keys[j]
				break
			}
		}
	}
	if scalar {
		return out[0], nil
	}
	return inferQArray(out), nil
}

func dictFindMissingKey(dict EvalDict) any {
	for _, key := range dict.Keys {
		if data.IsNull(key) {
			continue
		}
		switch key.(type) {
		case data.Symbol:
			return data.Symbol("")
		case string:
			return ""
		default:
			if kind := qKindOfValue(key); kind != "" && kind != data.KindAny {
				return data.NullForKind(kind)
			}
			return data.NullValue
		}
	}
	return data.NullValue
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
		values, _, err := findQueryValues(right)
		if err == nil {
			mask, handled, err := data.TryTypedInMask(leftArray, values)
			shape := "in-mask/" + string(leftArray.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
			recordRuntimeKernelProbe("ArrayInMask", shape, handled, err)
			if err != nil {
				return nil, err
			}
			if handled {
				return mask, nil
			}
		}
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
	if indexes, handled := trySetOpIndexes("except", leftArray, right); handled {
		return data.Gather(leftArray, indexes)
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
	if indexes, handled := trySetOpIndexes("inter", leftArray, right); handled {
		return data.Gather(leftArray, indexes)
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

// trySetOpIndexes routes inter/except left-row selection through the typed
// set-op kernel, replacing per-row boxed DeepEqual membership scans.
func trySetOpIndexes(op string, leftArray data.Array, right any) ([]int, bool) {
	items, err := setItems(right)
	if err != nil {
		return nil, false
	}
	indexes, handled := data.TryTypedSetOpIndexes(op, leftArray, items)
	recordRuntimeKernelProbe("ArraySetOpIndexes", op+"/"+string(leftArray.Kind()), handled, nil)
	return indexes, handled
}

func union(left, right any) (any, error) {
	if leftArray, ok := left.(data.Array); ok {
		if rightArray, ok := right.(data.Array); ok {
			if out, handled := data.TryTypedUnion(leftArray, rightArray); handled {
				recordRuntimeKernelProbe("ArrayUnion", "union/"+string(leftArray.Kind()), handled, nil)
				return out, nil
			}
		}
	}
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
		out, gathered, err := qEvalArrayGatherI64Primitive(array, indexes)
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
		out, gathered, err := qEvalArrayGatherI64Primitive(array, indexes)
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
	if indexes, handled := data.TryTypedDistinctIndexes(array); handled {
		recordRuntimeKernelProbe("ArrayDistinctIndexes", "distinct/"+string(array.Kind()), handled, nil)
		return array.Gather(indexes), nil
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
	if d, ok := v.(EvalDict); ok {
		// Canonical q reverses both sides of a dict: reverse `a`b!1 2 is
		// `b`a!2 1.
		keys := make([]any, len(d.Keys))
		for i := range d.Keys {
			keys[i] = d.Keys[len(d.Keys)-1-i]
		}
		values := make([]any, len(d.Values))
		for i := range d.Values {
			values[i] = d.Values[len(d.Values)-1-i]
		}
		return EvalDict{Keys: keys, Values: values}, nil
	}
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
	if array, ok := v.(data.Array); ok && array.Len() == 0 {
		// Canonical raze of an empty list is the list itself: raze () -> ().
		return v, nil
	}
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
	case EvalDict:
		return dictTakeCount(x, n)
	case data.Array:
		if x.Len() == 0 && n != 0 {
			return takeFromEmptyArray(x.Kind(), n)
		}
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
			// 0#atom keeps the atom's type so a later n#(0#x) take can fill
			// with the type's zero (canonical: 3#0#1 -> 0 0 0).
			return data.TakeRepeat(inferQArray([]any{v}, qKindOfValue(v)), 0)
		}
		return data.TakeRepeat(inferQArray([]any{v}, qKindOfValue(v)), n)
	}
}

// takeFromEmptyArray implements canonical q take from an empty list: the
// result is |n| copies of the list type's zero/default fill (0, 0f, 0b,
// empty symbol, ""), mirroring kdb+ `3#0#1` -> `0 0 0`. Kinds without an
// obvious zero (generic/any, temporal) fill with null.
func takeFromEmptyArray(kind data.Kind, n int) (any, error) {
	count := n
	if count < 0 {
		count = -count
	}
	if count > qMaxListLength {
		// The fill below allocates eagerly; counts near 0W would overflow
		// makeslice (see qMaxListLength).
		return nil, fmt.Errorf("take count %d exceeds the %d list limit", count, int64(qMaxListLength))
	}
	fill := qTakeFillForKind(kind)
	values := make([]any, count)
	for i := range values {
		values[i] = fill
	}
	if fill == data.NullValue {
		return data.InferArray(values), nil
	}
	return inferQArray(values, kind), nil
}

func qTakeFillForKind(kind data.Kind) any {
	switch kind {
	case data.KindI16:
		return int16(0)
	case data.KindI32:
		return int32(0)
	case data.KindI64:
		return int64(0)
	case data.KindF32:
		return float32(0)
	case data.KindF64:
		return float64(0)
	case data.KindBool:
		return false
	case data.KindSymbol:
		return data.Symbol("")
	case data.KindString:
		return ""
	default:
		return data.NullValue
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
	case EvalDict:
		return dictDropCount(x, n), nil
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
	// Dict drop: `a _ d (and d _ `a) removes keys; integer-atom left keeps
	// entry-count drop semantics through drop()'s EvalDict case.
	if d, ok := right.(EvalDict); ok {
		if keys, ok := qDictDropKeyOperand(left); ok {
			return dictDropKeys(d, keys), nil
		}
	}
	if d, ok := left.(EvalDict); ok {
		if keys, ok := qDictDropKeyOperand(right); ok {
			return dictDropKeys(d, keys), nil
		}
		return nil, fmt.Errorf("_ on a dictionary expects symbol keys or an integer count")
	}
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
	// Effectively unlimited budget: the ~ verb keeps exact semantics.
	budget := math.MaxInt
	return matchValueBudget(left, right, &budget)
}

// matchValueBudget is matchValue with a comparison-work budget. When the
// budget is exhausted the comparison gives up and reports a match: callers
// that bound the budget (the compiled/string differential oracle) prefer a
// rare false-negative on astronomically large lazy values over an unbounded
// element walk.
func matchValueBudget(left, right any, budget *int) bool {
	*budget--
	if *budget < 0 {
		return true
	}
	if data.IsNull(left) || data.IsNull(right) {
		return data.IsNull(left) && data.IsNull(right)
	}
	// q match treats NaN as matching NaN: (0%0)~0%0 is 1b in canonical q.
	// reflect.DeepEqual would report false (NaN != NaN), which also poisons
	// the compiled/string differential oracle whenever both routes produce
	// the same NaN.
	if lf, lok := left.(float64); lok {
		if rf, rok := right.(float64); rok && math.IsNaN(lf) && math.IsNaN(rf) {
			return true
		}
	}
	if lf, lok := left.(float32); lok {
		if rf, rok := right.(float32); rok && lf != lf && rf != rf {
			return true
		}
	}
	// q match treats identical callables as matching: count~count and
	// {x+1}~{x+1} are 1b in canonical q, and callables can appear inside
	// lists and dict keys/values. reflect.DeepEqual on func-typed fields is
	// always false, which would also poison the compiled/string differential
	// oracle whenever both routes produce the same callable.
	switch l := left.(type) {
	case qUnaryFunction:
		r, ok := right.(qUnaryFunction)
		return ok && l.name == r.name
	case qDyadicFunction:
		r, ok := right.(qDyadicFunction)
		return ok && l.name == r.name
	case qAdverbFunction:
		r, ok := right.(qAdverbFunction)
		return ok && l == r
	case qLambda:
		// Lambdas match on identical source text. Closures compare their
		// captured environments by IDENTITY (the same env map): that makes a
		// closure match itself (`f~f`, and the dual-route oracle comparing one
		// session binding against itself) while distinct env maps stay
		// conservatively unmatched (envs can be self-referential, so recursing
		// into them is not safe).
		r, ok := right.(qLambda)
		if !ok || l.body != r.body || l.namespace != r.namespace {
			return false
		}
		if len(l.env) == 0 && len(r.env) == 0 {
			return true
		}
		return len(l.env) == len(r.env) && reflect.ValueOf(l.env).Pointer() == reflect.ValueOf(r.env).Pointer()
	case qComposition:
		r, ok := right.(qComposition)
		if !ok || len(l.funcs) != len(r.funcs) {
			return false
		}
		for i := range l.funcs {
			if l.funcs[i].name != r.funcs[i].name {
				return false
			}
		}
		return true
	case qCallableAdverb:
		r, ok := right.(qCallableAdverb)
		return ok && l.adverb == r.adverb && matchValueBudget(l.fn, r.fn, budget)
	case qProjection:
		r, ok := right.(qProjection)
		if !ok || len(l.args) != len(r.args) || !matchValueBudget(l.fn, r.fn, budget) {
			return false
		}
		for i := range l.args {
			if l.args[i].missing != r.args[i].missing {
				return false
			}
			if !l.args[i].missing && !matchValueBudget(l.args[i].value, r.args[i].value, budget) {
				return false
			}
		}
		return true
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
			if *budget < 0 {
				return true
			}
			leftItem, leftOK := leftArray.At(i)
			rightItem, rightOK := rightArray.At(i)
			if !leftOK || !rightOK || !matchValueBudget(leftItem, rightItem, budget) {
				return false
			}
		}
		return true
	}
	leftFrame, leftIsFrame := left.(data.Frame)
	rightFrame, rightIsFrame := right.(data.Frame)
	if leftIsFrame || rightIsFrame {
		if !leftIsFrame || !rightIsFrame {
			return false
		}
		return matchFrameBudget(leftFrame, rightFrame, budget)
	}
	leftKeyed, leftIsKeyed := left.(data.KeyedFrame)
	rightKeyed, rightIsKeyed := right.(data.KeyedFrame)
	if leftIsKeyed || rightIsKeyed {
		if !leftIsKeyed || !rightIsKeyed {
			return false
		}
		leftKeys, rightKeys := leftKeyed.Keys(), rightKeyed.Keys()
		if len(leftKeys) != len(rightKeys) {
			return false
		}
		for i := range leftKeys {
			if leftKeys[i] != rightKeys[i] {
				return false
			}
		}
		return matchFrameBudget(leftKeyed.Frame(), rightKeyed.Frame(), budget)
	}
	leftDict, leftIsDict := left.(EvalDict)
	rightDict, rightIsDict := right.(EvalDict)
	if leftIsDict || rightIsDict {
		if !leftIsDict || !rightIsDict || len(leftDict.Keys) != len(rightDict.Keys) || len(leftDict.Values) != len(rightDict.Values) {
			return false
		}
		for i := range leftDict.Keys {
			if !matchValueBudget(leftDict.Keys[i], rightDict.Keys[i], budget) || !matchValueBudget(leftDict.Values[i], rightDict.Values[i], budget) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(left, right)
}

// matchFrameBudget compares tables structurally: same column names in the
// same order and element-wise matching columns. Lazy column views (index,
// fused-kernel, scan views) compare by value, which reflect.DeepEqual cannot.
func matchFrameBudget(left, right data.Frame, budget *int) bool {
	leftNames := left.Schema().Names()
	rightNames := right.Schema().Names()
	if len(leftNames) != len(rightNames) || left.Len() != right.Len() {
		return false
	}
	for i := range leftNames {
		if leftNames[i] != rightNames[i] {
			return false
		}
	}
	for _, name := range leftNames {
		leftColumn, leftOK := left.Column(name)
		rightColumn, rightOK := right.Column(name)
		if !leftOK || !rightOK || !matchValueBudget(leftColumn, rightColumn, budget) {
			return false
		}
	}
	return true
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
	case bool:
		// Booleans are not numbers in this dialect, but they ARE ordered:
		// 0b < 1b (canonical asc 101b -> 011b).
		r, ok := right.(bool)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		switch {
		case l == r:
			return 0, nil
		case r:
			return -1, nil
		default:
			return 1, nil
		}
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
