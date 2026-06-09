//go:build darwin && arm64

package methodjit

import (
	"sort"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func (tm *TieringManager) recordQRuntimePrimitiveExecution(proto *vm.FuncProto, op Op, outcome string) {
	if tm == nil || proto == nil {
		return
	}
	cf, _ := tm.tier2CompiledFor(proto)
	if cf == nil {
		return
	}
	cf.recordQRuntimePrimitiveExecution(op, outcome)
}

func (cf *CompiledFunction) recordQRuntimePrimitiveExecution(op Op, outcome string) {
	if cf == nil {
		return
	}
	source, kernel, shape, route, ok := qRuntimePrimitiveExecutionMetadata(op)
	if !ok {
		return
	}
	cf.recordQKernelExecution(source, kernel, shape, route, outcome)
}

func (cf *CompiledFunction) recordQEvalPipelinePlanExecution(id int, outcome string) {
	cf.recordQEvalPipelinePlanExecutionWithRoute(id, "typed_runtime_op_exit", outcome)
}

func (cf *CompiledFunction) recordQEvalPipelinePlanExecutionWithRoute(id int, route, outcome string) {
	if cf == nil {
		return
	}
	if cf.recordQEvalPipelinePlanExecutionCounter(id, route, outcome) {
		return
	}
	cf.recordQKernelExecution(
		"methodjit_q_eval_runtime",
		"QEvalPipelinePlan",
		qEvalPipelinePlanExecutionShape(cf.QEvalPipelinePlans, id),
		route,
		outcome,
	)
}

func (cf *CompiledFunction) recordQEvalPipelinePlanExecutionCounter(id int, route, outcome string) bool {
	if cf == nil || id < 0 || id >= len(cf.QEvalPipelinePlanStats) {
		return false
	}
	counter := &cf.QEvalPipelinePlanStats[id]
	switch route {
	case "typed_runtime_native_exit":
		if outcome == "success" {
			counter.nativeSuccess.Add(1)
			return true
		}
		if outcome == "error" {
			counter.nativeError.Add(1)
			return true
		}
	case "typed_runtime_op_exit":
		if outcome == "success" {
			counter.opSuccess.Add(1)
			return true
		}
		if outcome == "error" {
			counter.opError.Add(1)
			return true
		}
	case "typed_runtime_direct_entry":
		if outcome == "success" {
			counter.directSuccess.Add(1)
			return true
		}
		if outcome == "error" {
			counter.directError.Add(1)
			return true
		}
	}
	return false
}

func qVectorRuntimeKernelShapesByID(fn *Function) map[int]string {
	kernels := DetectQVectorRuntimeKernels(fn)
	if len(kernels) == 0 {
		return nil
	}
	out := make(map[int]string, len(kernels))
	for _, kernel := range kernels {
		if kernel.Instr == nil {
			continue
		}
		if shape := kernel.Shape(); shape != "" && shape != "unknown" {
			out[kernel.Instr.ID] = shape
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (cf *CompiledFunction) qVectorRuntimeKernelShape(instrID int, fallback string) string {
	if cf == nil || len(cf.QVectorRuntimeKernelShapesByID) == 0 {
		return fallback
	}
	if shape := cf.QVectorRuntimeKernelShapesByID[instrID]; shape != "" {
		return shape
	}
	return fallback
}

func (cf *CompiledFunction) recordQKernelExecution(source, kernel, shape, route, outcome string) {
	if cf == nil {
		return
	}
	cf.recordQKernelExecutionWithSchema(source, kernel, shape, route, outcome, "")
}

func (cf *CompiledFunction) recordQKernelExecutionForFrame(source, kernel, shape, route, outcome string, frameVal runtime.Value) {
	if cf == nil {
		return
	}
	cf.recordQKernelExecutionWithSchema(source, kernel, shape, route, outcome, qKernelSchemaHashForValue(frameVal))
}

func (cf *CompiledFunction) recordQKernelExecutionWithSchema(source, kernel, shape, route, outcome, schemaHash string) {
	if cf == nil {
		return
	}
	if source == "" {
		source = "unknown"
	}
	if kernel == "" {
		kernel = "unknown"
	}
	if shape == "" {
		shape = "unknown"
	}
	if route == "" {
		route = "unknown"
	}
	if outcome == "" {
		outcome = "unknown"
	}
	if schemaHash == "" {
		schemaHash = "unknown"
	}
	key := qKernelExecutionKey{
		source:  source,
		kernel:  kernel,
		shape:   shape,
		route:   route,
		outcome: outcome,
	}
	cf.qKernelStatsMu.Lock()
	if cf.qKernelStats == nil {
		cf.qKernelStats = make(map[qKernelExecutionKey]uint64)
	}
	cf.qKernelStats[key]++
	cf.qKernelStatsMu.Unlock()
	cf.recordQKernelDescriptorCacheLookup(qKernelDescriptorCacheKey{
		source:     source,
		kernel:     kernel,
		shape:      shape,
		route:      route,
		schemaHash: schemaHash,
	})
}

func qKernelSchemaHashForValue(v runtime.Value) string {
	info, ok := v.NativeFramePayloadInfo()
	if !ok || info.SchemaHash == "" {
		return "unknown"
	}
	return info.SchemaHash
}

type qKernelDescriptorCacheKey struct {
	source     string
	kernel     string
	shape      string
	route      string
	schemaHash string
}

type qKernelDescriptorCacheCounters struct {
	hits      uint64
	misses    uint64
	evictions uint64
}

func (cf *CompiledFunction) recordQKernelDescriptorCacheLookup(key qKernelDescriptorCacheKey) {
	if cf == nil {
		return
	}
	cf.qKernelDescriptorCacheMu.Lock()
	defer cf.qKernelDescriptorCacheMu.Unlock()
	if cf.qKernelDescriptorCache == nil {
		cf.qKernelDescriptorCache = make(map[qKernelDescriptorCacheKey]bool)
	}
	if cf.qKernelDescriptorStats == nil {
		cf.qKernelDescriptorStats = make(map[qKernelDescriptorCacheKey]qKernelDescriptorCacheCounters)
	}
	counters := cf.qKernelDescriptorStats[key]
	if cf.qKernelDescriptorCache[key] {
		counters.hits++
	} else {
		cf.qKernelDescriptorCache[key] = true
		counters.misses++
	}
	cf.qKernelDescriptorStats[key] = counters
}

func (cf *CompiledFunction) QKernelDescriptorCacheStats() []QKernelDescriptorCacheStat {
	if cf == nil {
		return nil
	}
	cf.qKernelDescriptorCacheMu.Lock()
	defer cf.qKernelDescriptorCacheMu.Unlock()
	if len(cf.qKernelDescriptorStats) == 0 {
		return nil
	}
	keys := make([]qKernelDescriptorCacheKey, 0, len(cf.qKernelDescriptorStats))
	for key := range cf.qKernelDescriptorStats {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		if keys[i].kernel != keys[j].kernel {
			return keys[i].kernel < keys[j].kernel
		}
		if keys[i].shape != keys[j].shape {
			return keys[i].shape < keys[j].shape
		}
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].schemaHash < keys[j].schemaHash
	})
	out := make([]QKernelDescriptorCacheStat, 0, len(keys))
	for _, key := range keys {
		counters := cf.qKernelDescriptorStats[key]
		entries := uint64(0)
		if cf.qKernelDescriptorCache[key] {
			entries = 1
		}
		out = append(out, QKernelDescriptorCacheStat{
			Source:     key.source,
			Kernel:     key.kernel,
			Shape:      key.shape,
			Route:      key.route,
			SchemaHash: key.schemaHash,
			Entries:    entries,
			Hits:       counters.hits,
			Misses:     counters.misses,
			Evictions:  counters.evictions,
		})
	}
	return out
}

func (cf *CompiledFunction) QKernelExecutionStats() []QKernelExecutionStat {
	if cf == nil {
		return nil
	}
	cf.qKernelStatsMu.Lock()
	merged := make(map[qKernelExecutionKey]uint64, len(cf.qKernelStats)+len(cf.QEvalPipelinePlanStats)*4)
	for key, count := range cf.qKernelStats {
		if count != 0 {
			merged[key] += count
		}
	}
	cf.qKernelStatsMu.Unlock()

	cf.appendQEvalPipelinePlanExecutionStats(merged)
	if len(merged) == 0 {
		return nil
	}
	keys := make([]qKernelExecutionKey, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		if keys[i].kernel != keys[j].kernel {
			return keys[i].kernel < keys[j].kernel
		}
		if keys[i].shape != keys[j].shape {
			return keys[i].shape < keys[j].shape
		}
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].outcome < keys[j].outcome
	})
	out := make([]QKernelExecutionStat, 0, len(keys))
	for _, key := range keys {
		out = append(out, QKernelExecutionStat{
			Source:  key.source,
			Kernel:  key.kernel,
			Shape:   key.shape,
			Route:   key.route,
			Outcome: key.outcome,
			Count:   merged[key],
		})
	}
	return out
}

func (cf *CompiledFunction) appendQEvalPipelinePlanExecutionStats(out map[qKernelExecutionKey]uint64) {
	if cf == nil || len(cf.QEvalPipelinePlanStats) == 0 {
		return
	}
	for id := range cf.QEvalPipelinePlanStats {
		counter := &cf.QEvalPipelinePlanStats[id]
		shape := qEvalPipelinePlanExecutionShape(cf.QEvalPipelinePlans, id)
		appendQEvalPipelinePlanCounter(out, shape, "typed_runtime_native_exit", "success", counter.nativeSuccess.Load())
		appendQEvalPipelinePlanCounter(out, shape, "typed_runtime_native_exit", "error", counter.nativeError.Load())
		appendQEvalPipelinePlanCounter(out, shape, "typed_runtime_op_exit", "success", counter.opSuccess.Load())
		appendQEvalPipelinePlanCounter(out, shape, "typed_runtime_op_exit", "error", counter.opError.Load())
		appendQEvalPipelinePlanCounter(out, shape, "typed_runtime_direct_entry", "success", counter.directSuccess.Load())
		appendQEvalPipelinePlanCounter(out, shape, "typed_runtime_direct_entry", "error", counter.directError.Load())
	}
}

func appendQEvalPipelinePlanCounter(out map[qKernelExecutionKey]uint64, shape, route, outcome string, count uint64) {
	if count == 0 {
		return
	}
	out[qKernelExecutionKey{
		source:  "methodjit_q_eval_runtime",
		kernel:  "QEvalPipelinePlan",
		shape:   shape,
		route:   route,
		outcome: outcome,
	}] += count
}

func qFrameSelectColumnExecutionShape(specs []QFrameSelectColumnSpec, specIdx int) string {
	if specIdx < 0 || specIdx >= len(specs) {
		return "unknown"
	}
	if specs[specIdx].Shape != "" {
		return specs[specIdx].Shape
	}
	return qFrameSelectColumnSpecShape(specs[specIdx])
}
