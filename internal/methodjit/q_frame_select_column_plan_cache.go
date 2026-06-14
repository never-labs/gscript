//go:build darwin && arm64

package methodjit

import (
	"fmt"
	"sort"

	"github.com/never-labs/leia/internal/runtime"
)

const qFrameSelectColumnPlanCacheRoute = "schema_stable_plan_cache"

type qFrameSelectColumnPlanKey struct {
	specIdx    int
	schemaHash string
}

type qFrameSelectColumnPlanCounters struct {
	hits   uint64
	misses uint64
}

func (cf *CompiledFunction) qFrameSelectColumnRuntimePlan(constants []runtime.Value, specIdx int, frameVal runtime.Value) (qFrameSelectColumnRuntimePlan, error) {
	if cf == nil {
		return qFrameSelectColumnRuntimePlan{}, fmt.Errorf("QFrameSelectColumn plan cache requires compiled function")
	}
	if specIdx < 0 || specIdx >= len(cf.QFrameSelectColumnSpecs) {
		return qFrameSelectColumnRuntimePlan{}, fmt.Errorf("QFrameSelectColumn spec index %d is out of range", specIdx)
	}
	key := qFrameSelectColumnPlanKey{
		specIdx:    specIdx,
		schemaHash: qKernelSchemaHashForValue(frameVal),
	}
	cf.qFrameSelectColumnPlanMu.Lock()
	if cf.qFrameSelectColumnPlans != nil {
		if plan, ok := cf.qFrameSelectColumnPlans[key]; ok {
			if cf.qFrameSelectColumnPlanStats == nil {
				cf.qFrameSelectColumnPlanStats = make(map[qFrameSelectColumnPlanKey]qFrameSelectColumnPlanCounters)
			}
			counters := cf.qFrameSelectColumnPlanStats[key]
			counters.hits++
			cf.qFrameSelectColumnPlanStats[key] = counters
			cf.qFrameSelectColumnPlanMu.Unlock()
			return plan, nil
		}
	}
	cf.qFrameSelectColumnPlanMu.Unlock()

	plan, err := buildQFrameSelectColumnRuntimePlan(constants, cf.QFrameSelectColumnSpecs, specIdx)
	if err != nil {
		return qFrameSelectColumnRuntimePlan{}, err
	}

	cf.qFrameSelectColumnPlanMu.Lock()
	if cf.qFrameSelectColumnPlans == nil {
		cf.qFrameSelectColumnPlans = make(map[qFrameSelectColumnPlanKey]qFrameSelectColumnRuntimePlan)
	}
	if cf.qFrameSelectColumnPlanStats == nil {
		cf.qFrameSelectColumnPlanStats = make(map[qFrameSelectColumnPlanKey]qFrameSelectColumnPlanCounters)
	}
	if existing, ok := cf.qFrameSelectColumnPlans[key]; ok {
		counters := cf.qFrameSelectColumnPlanStats[key]
		counters.hits++
		cf.qFrameSelectColumnPlanStats[key] = counters
		cf.qFrameSelectColumnPlanMu.Unlock()
		return existing, nil
	}
	cf.qFrameSelectColumnPlans[key] = plan
	counters := cf.qFrameSelectColumnPlanStats[key]
	counters.misses++
	cf.qFrameSelectColumnPlanStats[key] = counters
	cf.qFrameSelectColumnPlanMu.Unlock()
	return plan, nil
}

func (cf *CompiledFunction) qFrameSelectColumnPlanStatsSnapshot() map[qFrameSelectColumnPlanKey]qFrameSelectColumnPlanCounters {
	if cf == nil {
		return nil
	}
	cf.qFrameSelectColumnPlanMu.Lock()
	defer cf.qFrameSelectColumnPlanMu.Unlock()
	if len(cf.qFrameSelectColumnPlanStats) == 0 {
		return nil
	}
	out := make(map[qFrameSelectColumnPlanKey]qFrameSelectColumnPlanCounters, len(cf.qFrameSelectColumnPlanStats))
	for key, counters := range cf.qFrameSelectColumnPlanStats {
		out[key] = counters
	}
	return out
}

func (cf *CompiledFunction) QFrameSelectColumnPlanCacheStats() []QKernelDescriptorCacheStat {
	if cf == nil {
		return nil
	}
	cf.qFrameSelectColumnPlanMu.Lock()
	defer cf.qFrameSelectColumnPlanMu.Unlock()
	if len(cf.qFrameSelectColumnPlanStats) == 0 {
		return nil
	}
	keys := make([]qFrameSelectColumnPlanKey, 0, len(cf.qFrameSelectColumnPlanStats))
	for key := range cf.qFrameSelectColumnPlanStats {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].specIdx != keys[j].specIdx {
			return keys[i].specIdx < keys[j].specIdx
		}
		return keys[i].schemaHash < keys[j].schemaHash
	})
	out := make([]QKernelDescriptorCacheStat, 0, len(keys))
	for _, key := range keys {
		counters := cf.qFrameSelectColumnPlanStats[key]
		shape := "unknown"
		if key.specIdx >= 0 && key.specIdx < len(cf.QFrameSelectColumnSpecs) && cf.QFrameSelectColumnSpecs[key.specIdx].Shape != "" {
			shape = cf.QFrameSelectColumnSpecs[key.specIdx].Shape
		}
		entries := uint64(0)
		if cf.qFrameSelectColumnPlans != nil {
			if _, ok := cf.qFrameSelectColumnPlans[key]; ok {
				entries = 1
			}
		}
		out = append(out, QKernelDescriptorCacheStat{
			Source:     "methodjit_q_frame_runtime",
			Kernel:     "QFrameSelectColumn",
			Shape:      shape,
			Route:      qFrameSelectColumnPlanCacheRoute,
			SchemaHash: key.schemaHash,
			Entries:    entries,
			Hits:       counters.hits,
			Misses:     counters.misses,
		})
	}
	return out
}
