package q

import (
	"sort"
	"sync"
	"sync/atomic"
)

// q.eval statement dispatch tracing.
//
// Every ordinary q statement funnels through evalScriptStatement /
// evalCachedOrStringUncached, which tries compiled routes (binding plan, fast
// plan, pipeline plan, parsed value expression) before falling back to the
// per-call string evaluator cascade (s.eval). This trace classifies, per
// statement source, which route ultimately produced the value, so tests and
// audits can measure how much of a warm workload still pays per-call string
// dispatch. Tracing is off by default and costs one atomic load per
// statement when disabled.

// EvalDispatchRoute names the statement-level route that produced a value.
type EvalDispatchRoute string

const (
	// Compiled routes: per-call work is plan execution only.
	EvalDispatchPipelineBackend  EvalDispatchRoute = "pipeline_backend"
	EvalDispatchCompareIndexPlan EvalDispatchRoute = "compare_index_plan"
	EvalDispatchAssignPool       EvalDispatchRoute = "assign_pool"
	EvalDispatchBindingPlan      EvalDispatchRoute = "binding_plan"
	EvalDispatchFastPlan         EvalDispatchRoute = "fast_plan"
	EvalDispatchPipelinePlan     EvalDispatchRoute = "pipeline_plan"
	EvalDispatchValueExpr        EvalDispatchRoute = "value_expr"
	EvalDispatchCompiledExpr     EvalDispatchRoute = "compiled_expr"
	EvalDispatchConstMemo        EvalDispatchRoute = "const_memo"

	// String routes: per-call string probing/walking happened.
	EvalDispatchApplyIndexString EvalDispatchRoute = "apply_index_string"
	EvalDispatchWhereCompareSum  EvalDispatchRoute = "where_compare_sum_string"
	EvalDispatchScalarAddChain   EvalDispatchRoute = "scalar_add_chain_string"
	EvalDispatchDeferredScan     EvalDispatchRoute = "deferred_scan_string"
	EvalDispatchStringEval       EvalDispatchRoute = "string_eval"
)

// IsCompiledEvalDispatchRoute reports whether route executes from persistent
// compiled artifacts (no per-call statement-string walking).
func IsCompiledEvalDispatchRoute(route EvalDispatchRoute) bool {
	switch route {
	case EvalDispatchPipelineBackend, EvalDispatchCompareIndexPlan,
		EvalDispatchAssignPool, EvalDispatchBindingPlan, EvalDispatchFastPlan,
		EvalDispatchPipelinePlan, EvalDispatchValueExpr, EvalDispatchCompiledExpr,
		EvalDispatchConstMemo:
		return true
	default:
		return false
	}
}

var (
	qEvalDispatchTraceEnabled atomic.Bool
	qEvalDispatchTraceMu      sync.Mutex
	qEvalDispatchTraceCounts  map[string]map[EvalDispatchRoute]uint64
)

// SetEvalDispatchTraceEnabled toggles statement dispatch tracing.
func SetEvalDispatchTraceEnabled(enabled bool) {
	qEvalDispatchTraceEnabled.Store(enabled)
}

// ResetEvalDispatchTrace clears accumulated dispatch trace counts.
func ResetEvalDispatchTrace() {
	qEvalDispatchTraceMu.Lock()
	qEvalDispatchTraceCounts = nil
	qEvalDispatchTraceMu.Unlock()
}

// EvalDispatchTraceEntry is one statement source with per-route counts.
type EvalDispatchTraceEntry struct {
	Src    string
	Routes map[EvalDispatchRoute]uint64
}

// EvalDispatchTraceSnapshot returns accumulated per-statement dispatch
// counts, sorted by statement source for stable output.
func EvalDispatchTraceSnapshot() []EvalDispatchTraceEntry {
	qEvalDispatchTraceMu.Lock()
	defer qEvalDispatchTraceMu.Unlock()
	out := make([]EvalDispatchTraceEntry, 0, len(qEvalDispatchTraceCounts))
	for src, routes := range qEvalDispatchTraceCounts {
		copied := make(map[EvalDispatchRoute]uint64, len(routes))
		for route, count := range routes {
			copied[route] = count
		}
		out = append(out, EvalDispatchTraceEntry{Src: src, Routes: copied})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Src < out[j].Src })
	return out
}

func recordQEvalDispatch(src string, route EvalDispatchRoute) {
	if !qEvalDispatchTraceEnabled.Load() {
		return
	}
	qEvalDispatchTraceMu.Lock()
	if qEvalDispatchTraceCounts == nil {
		qEvalDispatchTraceCounts = make(map[string]map[EvalDispatchRoute]uint64, 64)
	}
	routes := qEvalDispatchTraceCounts[src]
	if routes == nil {
		routes = make(map[EvalDispatchRoute]uint64, 4)
		qEvalDispatchTraceCounts[src] = routes
	}
	routes[route]++
	qEvalDispatchTraceMu.Unlock()
}
