package bind

// QRuntimeKernelExecutionStatsFrom maps external q runtime-kernel execution
// observations into the q.cache_stats-facing bind shape without coupling bind
// to the producer package. Use the filtered variant when a producer needs to
// skip rows; this mapper preserves every converted row.
func QRuntimeKernelExecutionStatsFrom[T any](stats []T, convert func(T) QRuntimeKernelExecutionStat) []QRuntimeKernelExecutionStat {
	if len(stats) == 0 || convert == nil {
		return nil
	}
	out := make([]QRuntimeKernelExecutionStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, convert(stat))
	}
	return out
}

// QRuntimeKernelExecutionStatsFromFiltered maps external execution
// observations and lets the producer explicitly skip rows that do not belong
// in the bind execution stats table.
func QRuntimeKernelExecutionStatsFromFiltered[T any](stats []T, convert func(T) (QRuntimeKernelExecutionStat, bool)) []QRuntimeKernelExecutionStat {
	if len(stats) == 0 || convert == nil {
		return nil
	}
	out := make([]QRuntimeKernelExecutionStat, 0, len(stats))
	for _, stat := range stats {
		row, ok := convert(stat)
		if !ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

// SetMappedQRuntimeKernelExecutionStatsProvider adapts a producer-owned stats
// type to bind's q.cache_stats row shape. The conversion remains caller-owned
// so bind does not import MethodJIT or any other runtime stats producer.
func SetMappedQRuntimeKernelExecutionStatsProvider[T any](provider func() []T, convert func(T) QRuntimeKernelExecutionStat) func() {
	if provider == nil || convert == nil {
		return SetQRuntimeKernelExecutionStatsProvider(nil)
	}
	return SetQRuntimeKernelExecutionStatsProvider(func() []QRuntimeKernelExecutionStat {
		return QRuntimeKernelExecutionStatsFrom(provider(), convert)
	})
}

// SetMappedQRuntimeKernelExecutionStatsProviderFiltered adapts producer-owned
// execution stats while making row filtering explicit at the bridge boundary.
func SetMappedQRuntimeKernelExecutionStatsProviderFiltered[T any](provider func() []T, convert func(T) (QRuntimeKernelExecutionStat, bool)) func() {
	if provider == nil || convert == nil {
		return SetQRuntimeKernelExecutionStatsProvider(nil)
	}
	return SetQRuntimeKernelExecutionStatsProvider(func() []QRuntimeKernelExecutionStat {
		return QRuntimeKernelExecutionStatsFromFiltered(provider(), convert)
	})
}

// QRuntimeKernelDescriptorCacheStatsFrom maps external q runtime-kernel
// descriptor cache observations into the q.cache_stats-facing bind shape
// without coupling bind to the producer package.
func QRuntimeKernelDescriptorCacheStatsFrom[T any](stats []T, convert func(T) QRuntimeKernelDescriptorCacheStat) []QRuntimeKernelDescriptorCacheStat {
	if len(stats) == 0 || convert == nil {
		return nil
	}
	out := make([]QRuntimeKernelDescriptorCacheStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, convert(stat))
	}
	return out
}

// QRuntimeKernelDescriptorCacheStatsFromFiltered maps external descriptor
// cache observations and lets the producer explicitly skip rows.
func QRuntimeKernelDescriptorCacheStatsFromFiltered[T any](stats []T, convert func(T) (QRuntimeKernelDescriptorCacheStat, bool)) []QRuntimeKernelDescriptorCacheStat {
	if len(stats) == 0 || convert == nil {
		return nil
	}
	out := make([]QRuntimeKernelDescriptorCacheStat, 0, len(stats))
	for _, stat := range stats {
		row, ok := convert(stat)
		if !ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

// SetMappedQRuntimeKernelDescriptorCacheStatsProvider adapts producer-owned
// descriptor cache stats to bind's q.cache_stats row shape.
func SetMappedQRuntimeKernelDescriptorCacheStatsProvider[T any](provider func() []T, convert func(T) QRuntimeKernelDescriptorCacheStat) func() {
	if provider == nil || convert == nil {
		return SetQRuntimeKernelDescriptorCacheStatsProvider(nil)
	}
	return SetQRuntimeKernelDescriptorCacheStatsProvider(func() []QRuntimeKernelDescriptorCacheStat {
		return QRuntimeKernelDescriptorCacheStatsFrom(provider(), convert)
	})
}

// SetMappedQRuntimeKernelDescriptorCacheStatsProviderFiltered adapts
// producer-owned descriptor cache stats while making row filtering explicit.
func SetMappedQRuntimeKernelDescriptorCacheStatsProviderFiltered[T any](provider func() []T, convert func(T) (QRuntimeKernelDescriptorCacheStat, bool)) func() {
	if provider == nil || convert == nil {
		return SetQRuntimeKernelDescriptorCacheStatsProvider(nil)
	}
	return SetQRuntimeKernelDescriptorCacheStatsProvider(func() []QRuntimeKernelDescriptorCacheStat {
		return QRuntimeKernelDescriptorCacheStatsFromFiltered(provider(), convert)
	})
}

// QRuntimeKernelLoweringStatsFrom maps external q runtime-kernel lowering
// decision observations into the q.cache_stats-facing bind shape without
// coupling bind to the producer package. Use the filtered variant when a
// producer needs to skip rows; this mapper preserves every converted row.
func QRuntimeKernelLoweringStatsFrom[T any](stats []T, convert func(T) QRuntimeKernelLoweringStat) []QRuntimeKernelLoweringStat {
	if len(stats) == 0 || convert == nil {
		return nil
	}
	out := make([]QRuntimeKernelLoweringStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, convert(stat))
	}
	return out
}

// QRuntimeKernelLoweringStatsFromFiltered maps external lowering observations
// and lets the producer explicitly skip rows that do not belong in the bind
// lowering stats table.
func QRuntimeKernelLoweringStatsFromFiltered[T any](stats []T, convert func(T) (QRuntimeKernelLoweringStat, bool)) []QRuntimeKernelLoweringStat {
	if len(stats) == 0 || convert == nil {
		return nil
	}
	out := make([]QRuntimeKernelLoweringStat, 0, len(stats))
	for _, stat := range stats {
		row, ok := convert(stat)
		if !ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

// SetMappedQRuntimeKernelLoweringStatsProvider adapts a producer-owned
// lowering fallback type to bind's q.cache_stats row shape. The conversion
// remains caller-owned so bind does not import MethodJIT or any other producer.
func SetMappedQRuntimeKernelLoweringStatsProvider[T any](provider func() []T, convert func(T) QRuntimeKernelLoweringStat) func() {
	if provider == nil || convert == nil {
		return SetQRuntimeKernelLoweringStatsProvider(nil)
	}
	return SetQRuntimeKernelLoweringStatsProvider(func() []QRuntimeKernelLoweringStat {
		return QRuntimeKernelLoweringStatsFrom(provider(), convert)
	})
}

// SetMappedQRuntimeKernelLoweringStatsProviderFiltered adapts producer-owned
// lowering stats while making row filtering explicit at the bridge boundary.
func SetMappedQRuntimeKernelLoweringStatsProviderFiltered[T any](provider func() []T, convert func(T) (QRuntimeKernelLoweringStat, bool)) func() {
	if provider == nil || convert == nil {
		return SetQRuntimeKernelLoweringStatsProvider(nil)
	}
	return SetQRuntimeKernelLoweringStatsProvider(func() []QRuntimeKernelLoweringStat {
		return QRuntimeKernelLoweringStatsFromFiltered(provider(), convert)
	})
}
