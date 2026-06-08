//go:build darwin && arm64

package methodjit

import (
	"sort"

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
}

func (cf *CompiledFunction) QKernelExecutionStats() []QKernelExecutionStat {
	if cf == nil {
		return nil
	}
	cf.qKernelStatsMu.Lock()
	defer cf.qKernelStatsMu.Unlock()
	if len(cf.qKernelStats) == 0 {
		return nil
	}
	keys := make([]qKernelExecutionKey, 0, len(cf.qKernelStats))
	for key := range cf.qKernelStats {
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
			Count:   cf.qKernelStats[key],
		})
	}
	return out
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
