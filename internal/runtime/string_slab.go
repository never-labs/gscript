// string_slab.go — R14 bump allocator for small []string slices (skeys).
//
// `skeys` is a []string parallel to svals for small string-keyed tables.
// In NewTableSized, `make([]string, 0, hashHint)` was a per-call
// mallocgc. This slab amortizes that cost: hand out zero-length sub-
// slices from a shared Go-heap []string backing. Growth past the
// original cap escapes back to Go (rare for small tables — vec3 uses
// cap=3 and never appends past it).
//
// Cannot use arena (mmap) because []string contains GC pointers (string
// data). Backing must be Go-heap so GC scans string contents.
//
// stringSlab itself is not thread-safe. Heap serializes public allocation
// entry points before calling into it.

package runtime

import (
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"
)

const stringSlabSize = 4096 // 4096 * 16 B = 64 KB per backing refill

type stringSlab struct {
	backing []string
	idx     int
	// Retained older backings stay alive via their outstanding sub-slices;
	// this field just documents intent and keeps a slab-level reference
	// while the Heap exists.
	retained [][]string
}

// allocStringKeys returns a zero-length []string with the requested capacity,
// backed by the current slab. Falls back to `make` for requests too large
// for a single slab slot.
func (s *stringSlab) allocStringKeys(capacity int) []string {
	if capacity <= 0 {
		return nil
	}
	if capacity > stringSlabSize/4 {
		// Large requests: Go-heap directly. Not worth dedicating most of
		// a backing to one table.
		return make([]string, 0, capacity)
	}
	if s.backing == nil || s.idx+capacity > len(s.backing) {
		s.refill()
	}
	out := s.backing[s.idx : s.idx : s.idx+capacity]
	s.idx += capacity
	return out
}

func (s *stringSlab) refill() {
	next := make([]string, stringSlabSize)
	if s.backing != nil {
		s.retained = append(s.retained, s.backing)
	}
	s.backing = next
	s.idx = 0
}

// AllocStringKeys returns a zero-length []string with the requested
// capacity, backed by the Heap's string slab. Suitable for
// NewTableSized's skeys initialization.
func (h *Heap) AllocStringKeys(capacity int) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stringSlab.allocStringKeys(capacity)
}

const stringBoxSlabSize = 16384 // 16384 * 16 B = 256 KB per backing refill
const stringBoxSlabElemSize = uintptr(unsafe.Sizeof(string("")))

type stringBoxSlab struct {
	backing []string
	idx     int
}

type stringBoxSlabRange struct {
	start uintptr
	end   uintptr
}

var stringBoxSlabRanges struct {
	sync.RWMutex
	ranges []stringBoxSlabRange
}

//go:nocheckptr
func (h *Heap) tryAllocStringBoxFast(value string) *string {
	if h == nil {
		return nil
	}
	for {
		next := atomic.LoadUintptr(&h.stringBoxSlabNext)
		if next == 0 {
			return nil
		}
		end := atomic.LoadUintptr(&h.stringBoxSlabEnd)
		if end == 0 || next > end-stringBoxSlabElemSize {
			return nil
		}
		if atomic.CompareAndSwapUintptr(&h.stringBoxSlabNext, next, next+stringBoxSlabElemSize) {
			p := (*string)(unsafe.Pointer(next))
			*p = value
			return p
		}
	}
}

func (s *stringBoxSlab) allocSlow(h *Heap, value string) *string {
	s.backing = make([]string, stringBoxSlabSize)
	s.idx = 1
	if h != nil {
		h.publishStringBoxSlab(s.backing)
	}
	p := &s.backing[0]
	*p = value
	return p
}

func (h *Heap) AllocStringBox(value string) *string {
	if p := h.tryAllocStringBoxFast(value); p != nil {
		return p
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if p := h.tryAllocStringBoxFast(value); p != nil {
		return p
	}
	return h.stringBoxSlab.allocSlow(h, value)
}

func (h *Heap) publishStringBoxSlab(backing []string) {
	if len(backing) == 0 {
		atomic.StoreUintptr(&h.stringBoxSlabNext, 0)
		atomic.StoreUintptr(&h.stringBoxSlabStart, 0)
		atomic.StoreUintptr(&h.stringBoxSlabEnd, 0)
		return
	}
	root := unsafe.Pointer(&backing[0])
	start := uintptr(root)
	end := start + uintptr(len(backing))*unsafe.Sizeof(backing[0])
	registerStringBoxSlabRange(start, end)
	keepAliveGlobal(root) // slab roots cover later allocations by any goroutine; never scope-captured
	atomic.StoreUintptr(&h.stringBoxSlabNext, 0)
	atomic.StoreUintptr(&h.stringBoxSlabStart, 0)
	atomic.StoreUintptr(&h.stringBoxSlabEnd, end)
	atomic.StoreUintptr(&h.stringBoxSlabStart, start)
	atomic.StoreUintptr(&h.stringBoxSlabNext, start+stringBoxSlabElemSize)
}

func registerStringBoxSlabRange(start, end uintptr) {
	if start == 0 || end <= start {
		return
	}
	stringBoxSlabRanges.Lock()
	defer stringBoxSlabRanges.Unlock()

	i := sort.Search(len(stringBoxSlabRanges.ranges), func(i int) bool {
		return stringBoxSlabRanges.ranges[i].start >= start
	})
	if i < len(stringBoxSlabRanges.ranges) &&
		stringBoxSlabRanges.ranges[i].start == start &&
		stringBoxSlabRanges.ranges[i].end == end {
		return
	}
	stringBoxSlabRanges.ranges = append(stringBoxSlabRanges.ranges, stringBoxSlabRange{})
	copy(stringBoxSlabRanges.ranges[i+1:], stringBoxSlabRanges.ranges[i:])
	stringBoxSlabRanges.ranges[i] = stringBoxSlabRange{start: start, end: end}
}

func stringBoxSlabRootForPointer(p unsafe.Pointer) unsafe.Pointer {
	addr := uintptr(p)
	stringBoxSlabRanges.RLock()
	defer stringBoxSlabRanges.RUnlock()

	i := sort.Search(len(stringBoxSlabRanges.ranges), func(i int) bool {
		return stringBoxSlabRanges.ranges[i].start > addr
	})
	for j := i - 1; j >= 0; j-- {
		r := stringBoxSlabRanges.ranges[j]
		if addr >= r.start && addr < r.end {
			return pointerFromUintptr(r.start)
		}
		if addr >= r.end || r.start < addr {
			break
		}
	}
	return nil
}

func visitStringRoot(p unsafe.Pointer, visitor func(unsafe.Pointer)) {
	if root := stringBoxSlabRootForPointer(p); root != nil {
		visitor(root)
		return
	}
	visitor(p)
}
