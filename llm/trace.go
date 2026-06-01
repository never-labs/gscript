package llm

import "sync"

// TraceRecorder is a thread-safe trace sink for tests, diagnostics, and
// host-side observability. Pass recorder.Record to leia.WithLLMTrace.
type TraceRecorder struct {
	mu     sync.Mutex
	events []TraceEvent
}

func NewTraceRecorder(events ...TraceEvent) *TraceRecorder {
	rec := &TraceRecorder{}
	rec.events = append(rec.events, events...)
	return rec
}

func (r *TraceRecorder) Record(event TraceEvent) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *TraceRecorder) Events() []TraceEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]TraceEvent, len(r.events))
	copy(out, r.events)
	return out
}

func (r *TraceRecorder) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}
