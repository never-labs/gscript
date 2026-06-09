package data

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// RuntimeKernelExecutionStat reports typed data-runtime kernel observations.
// It is intentionally independent from q/bind so qSQL and Leia-native callers
// can share the same data runtime visibility.
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
	key   runtimeKernelExecutionKey
	count atomic.Uint64
}

var (
	runtimeKernelStatsMu sync.RWMutex
	runtimeKernelStats   map[runtimeKernelExecutionKey]*runtimeKernelExecutionCounter
)

func recordDataRuntimeKernelProbe(kernel, shape string, handled bool, err error) {
	recordDataRuntimeKernelExecution(kernel, shape, "attempt", "attempt")
	switch {
	case err != nil:
		recordDataRuntimeKernelExecution(kernel, shape, "error", "runtime_error")
	case handled:
		recordDataRuntimeKernelExecution(kernel, shape, "hit", "typed_kernel")
	default:
		recordDataRuntimeKernelExecution(kernel, shape, "fallback", "unsupported_type")
	}
}

func recordDataRuntimeKernelExecution(kernel, shape, outcome, reasonCode string) {
	key := runtimeKernelExecutionKey{
		source:     normalizeDataRuntimeStatField("data_query_runtime", "data_runtime"),
		kernel:     normalizeDataRuntimeStatField(kernel, "unknown"),
		shape:      normalizeDataRuntimeStatField(shape, "unknown"),
		route:      normalizeDataRuntimeStatField("typed_data_kernel", "runtime_primitive"),
		outcome:    normalizeDataRuntimeStatField(outcome, "unknown"),
		reasonCode: normalizeDataRuntimeStatField(reasonCode, outcome),
	}
	runtimeKernelCounterFor(key).count.Add(1)
}

func normalizeDataRuntimeStatField(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func runtimeKernelCounterFor(key runtimeKernelExecutionKey) *runtimeKernelExecutionCounter {
	runtimeKernelStatsMu.RLock()
	if counter := runtimeKernelStats[key]; counter != nil {
		runtimeKernelStatsMu.RUnlock()
		return counter
	}
	runtimeKernelStatsMu.RUnlock()

	runtimeKernelStatsMu.Lock()
	defer runtimeKernelStatsMu.Unlock()
	if runtimeKernelStats == nil {
		runtimeKernelStats = make(map[runtimeKernelExecutionKey]*runtimeKernelExecutionCounter)
	}
	if counter := runtimeKernelStats[key]; counter != nil {
		return counter
	}
	counter := &runtimeKernelExecutionCounter{key: key}
	runtimeKernelStats[key] = counter
	return counter
}

// RuntimeKernelExecutionStats returns a stable snapshot of data-runtime typed
// kernel observations for diagnostics and q.cache_stats aggregation.
func RuntimeKernelExecutionStats() []RuntimeKernelExecutionStat {
	runtimeKernelStatsMu.RLock()
	counters := make([]*runtimeKernelExecutionCounter, 0, len(runtimeKernelStats))
	for _, counter := range runtimeKernelStats {
		counters = append(counters, counter)
	}
	runtimeKernelStatsMu.RUnlock()
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
			PipelineShape: RuntimeKernelPipelineShape(key.kernel, key.shape),
			Route:         key.route,
			Outcome:       key.outcome,
			ReasonCode:    key.reasonCode,
			Count:         count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Kernel != out[j].Kernel {
			return out[i].Kernel < out[j].Kernel
		}
		if out[i].Shape != out[j].Shape {
			return out[i].Shape < out[j].Shape
		}
		if out[i].Route != out[j].Route {
			return out[i].Route < out[j].Route
		}
		if out[i].Outcome != out[j].Outcome {
			return out[i].Outcome < out[j].Outcome
		}
		return out[i].ReasonCode < out[j].ReasonCode
	})
	return out
}

// RuntimeKernelPipelineShape maps concrete data-runtime probe shapes onto the
// lower-cardinality pipeline families used by q.cache_stats consumers.
func RuntimeKernelPipelineShape(kernel, shape string) string {
	switch {
	case strings.HasPrefix(shape, "bucket-floor/"):
		return "bucket_transform"
	case strings.HasPrefix(shape, "vector-transform/"):
		return "vector_transform"
	case strings.HasPrefix(shape, "query/"):
		return "query_kernel"
	default:
		return "unknown"
	}
}

// ClearRuntimeKernelExecutionStats resets data-runtime typed kernel counters.
func ClearRuntimeKernelExecutionStats() {
	runtimeKernelStatsMu.Lock()
	runtimeKernelStats = nil
	runtimeKernelStatsMu.Unlock()
}
