package bind

import "sync"

// QRuntimeKernelExecutionStat is the q.cache_stats-facing shape for MethodJIT
// typed-runtime q kernel execution observations. It intentionally stays
// separate from qSQL semantic cache hit/miss accounting.
type QRuntimeKernelExecutionStat struct {
	Source        string
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	Outcome       string
	ReasonCode    string
	Count         uint64
}

// QRuntimeKernelDescriptorCacheStat is the q.cache_stats-facing shape for
// MethodJIT schema-stable q runtime descriptor cache accounting.
type QRuntimeKernelDescriptorCacheStat struct {
	Source        string
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	SchemaHash    string
	Entries       uint64
	Hits          uint64
	Misses        uint64
	Evictions     uint64
}

// QRuntimeKernelLoweringStat is the q.cache_stats-facing shape for MethodJIT
// q typed-runtime kernel lowering fallbacks. These rows explain why a hot-path
// shape did not become a typed runtime kernel.
type QRuntimeKernelLoweringStat struct {
	Source        string
	Kind          string
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	Outcome       string
	ReasonFamily  string
	ReasonCode    string
	Count         uint64
}

var (
	qRuntimeKernelExecutionStatsProviderMu            sync.Mutex
	qRuntimeKernelExecutionStatsProviderCurrent       *qRuntimeKernelExecutionStatsProviderState
	qRuntimeKernelDescriptorCacheStatsProviderMu      sync.Mutex
	qRuntimeKernelDescriptorCacheStatsProviderCurrent *qRuntimeKernelDescriptorCacheStatsProviderState
	qRuntimeKernelLoweringStatsProviderMu             sync.Mutex
	qRuntimeKernelLoweringStatsProviderCurrent        *qRuntimeKernelLoweringStatsProviderState
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

type qRuntimeKernelDescriptorCacheStatsProviderState struct {
	provider func() []QRuntimeKernelDescriptorCacheStat
	previous *qRuntimeKernelDescriptorCacheStatsProviderState
	active   bool
}

func SetQRuntimeKernelDescriptorCacheStatsProvider(provider func() []QRuntimeKernelDescriptorCacheStat) func() {
	qRuntimeKernelDescriptorCacheStatsProviderMu.Lock()
	state := &qRuntimeKernelDescriptorCacheStatsProviderState{
		provider: provider,
		previous: qRuntimeKernelDescriptorCacheStatsProviderCurrent,
		active:   true,
	}
	qRuntimeKernelDescriptorCacheStatsProviderCurrent = state
	qRuntimeKernelDescriptorCacheStatsProviderMu.Unlock()
	return func() {
		qRuntimeKernelDescriptorCacheStatsProviderMu.Lock()
		if state.active {
			state.active = false
			if qRuntimeKernelDescriptorCacheStatsProviderCurrent == state {
				qRuntimeKernelDescriptorCacheStatsProviderCurrent = qRuntimeKernelDescriptorCacheStatsNextActiveProvider(state.previous)
			}
		}
		qRuntimeKernelDescriptorCacheStatsProviderMu.Unlock()
	}
}

func qRuntimeKernelDescriptorCacheStatsNextActiveProvider(state *qRuntimeKernelDescriptorCacheStatsProviderState) *qRuntimeKernelDescriptorCacheStatsProviderState {
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
