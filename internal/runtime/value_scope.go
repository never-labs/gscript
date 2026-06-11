package runtime

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Value scopes: deterministic NaN-box root lifetimes for no-safe-point paths
// ---------------------------------------------------------------------------
//
// The NaN-box root log (value_gc.go) is append-only: every Go-heap pointer
// hidden inside a Value is rooted forever, because Go, native, and JIT frames
// can hold pointer-tagged Values outside any scanner's view. Code that calls
// result-producing runtime paths directly from Go (the q bind boundary,
// embedders, benchmarks) never reaches a point where those roots are released,
// so the heap grows without bound.
//
// A ValueScope gives such callers a mark-and-release discipline that is safe
// by construction instead of by scanning:
//
//   - PushValueScope() redirects this goroutine's subsequent root-log appends
//     into a scope-private buffer. The buffer is an ordinary GC-scanned
//     []unsafe.Pointer, so captured objects stay fully alive while the scope
//     is open.
//   - Release(keep...) re-publishes every captured pointer that is reachable
//     from the kept result values (transitively, via ScanValueRoots) to the
//     enclosing sink (parent scope, or the global root log), and drops the
//     rest.
//
// Safety invariants:
//
//  1. Per-goroutine capture. Appends from other goroutines never land in this
//     scope, so Release cannot drop another goroutine's roots. Goroutines
//     spawned inside a scope fall through to the global log.
//  2. Shared infrastructure roots bypass scopes. Slab publish roots
//     (table/string-box slabs) cover objects allocated by *any* goroutine at
//     *any* later time and therefore always go to the global log
//     (keepAliveGlobal), as do interface roots, whose ifaceRoots map entries
//     are keyed by address and must never dangle.
//  3. Script execution is barriered. Entering the VM while a scope is active
//     pushes a barrier scope (PushValueScopeBarrierIfActive) that routes all
//     appends to the global log: script code can publish Values to places no
//     result scan will find (globals, upvalues, coroutines), so its values
//     keep today's append-only lifetime.
//  4. Release must run on the goroutine that pushed the scope, after every
//     Value created inside the scope is either dead or reachable from the
//     keep set. Returning scope-created Values through any channel other than
//     keep (global caches, pre-existing tables, channels) is a contract
//     violation; runtime-internal publication points use
//     GlobalizeValueRoots/keepAliveGlobal so they remain safe regardless.
//
// Scopes are inert (PushValueScope returns nil, Release is a no-op) on
// platforms without getgptr support and when the small slot table is full;
// inert means "no truncation", never "unsafe".

const valueScopeSlotCount = 16

type valueScope struct {
	gid     uintptr
	parent  *valueScope
	barrier bool
	entries []unsafe.Pointer
}

// ValueScope is the public handle returned by PushValueScope. The zero value
// is inert: Release and CapturedRootCount are no-ops on it. The handle is
// returned by value so push/release adds no heap allocation of its own.
type ValueScope struct {
	s    *valueScope
	slot int
}

// Active reports whether this handle refers to a live (not yet released)
// capture scope.
func (vs *ValueScope) Active() bool { return vs != nil && vs.s != nil }

// valueScopePool recycles scope records (and their entries buffers) across
// pushes; a released scope is unreachable from the registry, so reuse is safe.
var valueScopePool = sync.Pool{New: func() any { return &valueScope{} }}

var (
	// valueScopeActive counts goroutines that currently have at least one
	// scope (or barrier) registered. keepAlive checks it before doing any
	// per-goroutine work, so the no-scope fast path costs one atomic load.
	valueScopeActive int32

	valueScopeGs     [valueScopeSlotCount]atomic.Uintptr
	valueScopeScopes [valueScopeSlotCount]atomic.Pointer[valueScope]
)

// currentValueScope returns the innermost scope registered for the calling
// goroutine, or nil. Only the owning goroutine mutates its slot, so the
// G-match guarantees the scope pointer read is the owner's current scope.
func currentValueScope() *valueScope {
	g := getgptr()
	if g == 0 {
		return nil
	}
	for i := range valueScopeGs {
		if valueScopeGs[i].Load() == g {
			return valueScopeScopes[i].Load()
		}
	}
	return nil
}

func newValueScope(g uintptr, parent *valueScope, barrier bool) *valueScope {
	s := valueScopePool.Get().(*valueScope)
	s.gid = g
	s.parent = parent
	s.barrier = barrier
	s.entries = s.entries[:0]
	return s
}

func pushValueScopeInternal(barrier bool) ValueScope {
	g := getgptr()
	if g == 0 {
		return ValueScope{}
	}
	// Nested scope on a goroutine that already owns a slot.
	for i := range valueScopeGs {
		if valueScopeGs[i].Load() == g {
			parent := valueScopeScopes[i].Load()
			s := newValueScope(g, parent, barrier)
			valueScopeScopes[i].Store(s)
			return ValueScope{s: s, slot: i}
		}
	}
	// First scope on this goroutine: claim a free slot.
	for i := range valueScopeGs {
		if valueScopeGs[i].Load() == 0 && valueScopeGs[i].CompareAndSwap(0, g) {
			s := newValueScope(g, nil, barrier)
			valueScopeScopes[i].Store(s)
			atomic.AddInt32(&valueScopeActive, 1)
			return ValueScope{s: s, slot: i}
		}
	}
	// Slot table full: inert. All appends keep going to the global log.
	return ValueScope{}
}

// PushValueScope opens a root-capture scope on the calling goroutine.
// The returned handle may be inert (zero value); Release is always safe.
func PushValueScope() ValueScope {
	return pushValueScopeInternal(false)
}

// PushValueScopeBarrierIfActive pushes a barrier scope if — and only if — the
// calling goroutine currently has an active capture scope. While the barrier
// is the innermost scope, all root-log appends from this goroutine go to the
// global log. VM/interpreter entry points call this so script execution that
// starts inside a value scope keeps append-only root lifetimes.
//
// Returns an inert handle (no-op Release) when no scope is active: the common
// case costs a single atomic load.
func PushValueScopeBarrierIfActive() ValueScope {
	if atomic.LoadInt32(&valueScopeActive) == 0 {
		return ValueScope{}
	}
	if s := currentValueScope(); s == nil || s.barrier {
		// No scope on this goroutine, or already barriered.
		return ValueScope{}
	}
	return pushValueScopeInternal(true)
}

// Release closes the scope. Captured roots that are transitively reachable
// from the keep values are re-published to the enclosing sink (parent scope
// or global root log); all other captured roots are dropped, allowing Go's GC
// to reclaim the objects once no Go-visible reference remains.
//
// Must be called on the goroutine that pushed the scope, with inner scopes
// already released. Inert (zero) handles are no-ops.
func (vs *ValueScope) Release(keep ...Value) {
	if vs == nil || vs.s == nil {
		return
	}
	s := vs.s
	vs.s = nil
	if g := getgptr(); g != s.gid {
		panic("runtime: ValueScope.Release called on a different goroutine than PushValueScope")
	}
	if valueScopeScopes[vs.slot].Load() != s {
		panic("runtime: ValueScope.Release called with an inner scope still open")
	}
	// Unregister first so the re-publishing below routes to the parent sink.
	if s.parent != nil {
		valueScopeScopes[vs.slot].Store(s.parent)
	} else {
		valueScopeScopes[vs.slot].Store(nil)
		valueScopeGs[vs.slot].Store(0)
		atomic.AddInt32(&valueScopeActive, -1)
	}

	if len(s.entries) == 0 {
		recycleValueScope(s)
		return
	}
	if len(keep) > 0 {
		scratch := valueScopeScratchPool.Get().(*valueScopeScratch)
		live, seen := scratch.live, scratch.seen
		visitor := func(p unsafe.Pointer) {
			if p != nil {
				live[uintptr(p)] = struct{}{}
			}
		}
		for _, v := range keep {
			ScanValueRoots(v, visitor, seen)
		}
		if len(live) > 0 {
			for _, p := range s.entries {
				if p == nil {
					continue
				}
				if _, ok := live[uintptr(p)]; ok {
					keepAlive(p, nil)
				}
			}
		}
		clear(live)
		clear(seen)
		valueScopeScratchPool.Put(scratch)
	}
	// Drop the remaining captured roots: clearing the buffer removes the last
	// GC-visible reference this scope held.
	recycleValueScope(s)
}

// recycleValueScope clears the captured-pointer buffer (releasing the last
// GC-visible references this scope held) and returns the record to the pool.
func recycleValueScope(s *valueScope) {
	clear(s.entries) // zero the used prefix; the GC scans the full backing
	s.entries = s.entries[:0]
	s.parent = nil
	s.gid = 0
	s.barrier = false
	valueScopePool.Put(s)
}

type valueScopeScratch struct {
	live map[uintptr]struct{}
	seen map[uintptr]struct{}
}

var valueScopeScratchPool = sync.Pool{
	New: func() any {
		return &valueScopeScratch{
			live: make(map[uintptr]struct{}, 64),
			seen: make(map[uintptr]struct{}, 64),
		}
	},
}

// CapturedRootCount reports how many root-log appends this scope has captured
// so far. Diagnostic only.
func (vs *ValueScope) CapturedRootCount() int {
	if vs == nil || vs.s == nil {
		return 0
	}
	return len(vs.s.entries)
}

// HasActiveValueScope reports whether the calling goroutine's root-log appends
// are currently captured by a ValueScope (i.e. a non-barrier scope is the
// innermost scope). Lazy caches that memoize Values in scan-invisible storage
// (Go closure maps) use this to skip caching while a scope is active, since a
// cached value would lose its roots when the scope is released.
func HasActiveValueScope() bool {
	if atomic.LoadInt32(&valueScopeActive) == 0 {
		return false
	}
	s := currentValueScope()
	return s != nil && !s.barrier
}

// GlobalizeValueRoots re-publishes every root pointer reachable from v to the
// global root log, regardless of any active scope. Runtime-internal caches
// that publish Values to process-global storage call this so a value created
// inside a scope survives the scope's release. No-op when no scope is active
// on the calling goroutine.
func GlobalizeValueRoots(v Value) {
	if atomic.LoadInt32(&valueScopeActive) == 0 {
		return
	}
	if s := currentValueScope(); s == nil || s.barrier {
		return
	}
	seen := make(map[uintptr]struct{}, 16)
	ScanValueRoots(v, func(p unsafe.Pointer) {
		if p != nil {
			keepAliveGlobal(p)
		}
	}, seen)
}
