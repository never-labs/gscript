package bind

// QRuntimeKernelExecutionStatsFrom maps external q runtime-kernel execution
// observations into the q.cache_stats-facing bind shape without coupling bind
// to the producer package.
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

// QRuntimeKernelLoweringStatsFrom maps external q runtime-kernel lowering
// fallback observations into the q.cache_stats-facing bind shape without
// coupling bind to the producer package.
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
