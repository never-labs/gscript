package runtime

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// ---------------------------------------------------------------------------
// GC roots: keeps Go-heap pointers alive while hidden inside uint64 Values
// ---------------------------------------------------------------------------
//
// NaN-boxed pointers are invisible to Go's GC. The root log keeps them alive.
// Periodic compaction removes dead entries to prevent unbounded growth.
//
// The root log is an append-only slice with an atomic cursor. keepAlive is
// lock-free for the common case (no mutex, just an atomic increment).
// lookupIface uses a separate locked map for the rare interface-based
// function/coroutine values that need type recovery.

// gcRootLog is a lock-free append-only log for keeping Go-heap pointers alive.
// Uses []unsafe.Pointer instead of []any for 2x less GC scan overhead
// (1 word per entry vs 2 words for interface).
type gcRootLog struct {
	entries []unsafe.Pointer
	cursor  int64 // next free index (accessed atomically)
}

type gcCompactScratchState struct {
	vms     []GCRootScanner
	liveSet map[uintptr]struct{}
}

// GCRootScanner is implemented by VMs to enumerate all live GC root pointers.
// Used by gcCompact to determine which gcRootLog entries are still needed.
type GCRootScanner interface {
	ScanGCRoots(visitor func(unsafe.Pointer))
}

const (
	// gcCompactInterval triggers compaction every N new allocations.
	// 1M balances keeping the log small vs amortizing compaction cost.
	gcCompactInterval int64 = 1 << 20 // 1M
)

var (
	gcLog gcRootLog
	// Separate locked map for interface-based values that need lookupIface.
	// Only used by AnyFunction/AnyCoroutine (cold path).
	ifaceMu    sync.Mutex
	ifaceRoots = make(map[uintptr]any, 64)

	// activeVMs tracks all live VMs for GC root scanning.
	activeVMsMu   sync.Mutex
	activeVMs     []GCRootScanner
	activeVMCount int32 // atomic count for fast check in keepAlive

	// gcCompacting prevents re-entrant compaction.
	gcCompacting int32

	// gcNeedsCompact is set by keepAlive when compaction threshold is reached.
	// Actual compaction is deferred to a VM safe point via CheckGC().
	gcNeedsCompact int32
	gcScratch      gcCompactScratchState

	vmCoroutinePtrResolver func(unsafe.Pointer) any
	vmClosureRootScanner   func(unsafe.Pointer, func(unsafe.Pointer), map[uintptr]struct{})
	vmCoroutineRootScanner func(unsafe.Pointer, func(unsafe.Pointer), map[uintptr]struct{})
)

// RegisterVM adds a VM to the active set for GC root scanning.
func RegisterVM(scanner GCRootScanner) {
	activeVMsMu.Lock()
	activeVMs = append(activeVMs, scanner)
	atomic.StoreInt32(&activeVMCount, int32(len(activeVMs)))
	activeVMsMu.Unlock()
}

// UnregisterVM removes a VM from the active set.
func UnregisterVM(scanner GCRootScanner) {
	activeVMsMu.Lock()
	for i, s := range activeVMs {
		if s == scanner {
			activeVMs[i] = activeVMs[len(activeVMs)-1]
			activeVMs[len(activeVMs)-1] = nil
			activeVMs = activeVMs[:len(activeVMs)-1]
			break
		}
	}
	atomic.StoreInt32(&activeVMCount, int32(len(activeVMs)))
	activeVMsMu.Unlock()
}

func init() {
	gcLog.entries = make([]unsafe.Pointer, 1<<20) // 1M entries (~8MB), grows if needed
}

// keepAlive registers a Go-heap pointer in the root log so the GC does not
// collect the object while it is hidden inside a NaN-boxed Value.
//
// When the calling goroutine has an active ValueScope (value_scope.go), the
// pointer is captured in the scope's private buffer instead of the global
// append-only log, giving the scope owner a deterministic release point.
// Shared, load-bearing roots (slab publish roots, interface roots) must call
// keepAliveGlobal directly: they cover objects created by other goroutines or
// at later times and may never be scope-released.
func keepAlive(p unsafe.Pointer, _ any) {
	if atomic.LoadInt32(&valueScopeActive) != 0 {
		if s := currentValueScope(); s != nil && !s.barrier {
			s.entries = append(s.entries, p)
			return
		}
	}
	keepAliveGlobal(p)
}

// keepAliveGlobal appends to the global append-only root log, bypassing any
// active value scope. Lock-free: uses atomic increment on the cursor.
func keepAliveGlobal(p unsafe.Pointer) {
	idx := atomic.AddInt64(&gcLog.cursor, 1) - 1
	if idx < int64(len(gcLog.entries)) {
		gcLog.entries[idx] = p
	} else {
		gcLogGrowAt(idx, p)
	}
	// Signal that GC compaction is needed; actual compaction is deferred
	// to a VM safe point (CheckGC) where all values are in registers.
	if idx > 0 && idx%gcCompactInterval == 0 && atomic.LoadInt32(&activeVMCount) > 0 {
		atomic.StoreInt32(&gcNeedsCompact, 1)
	}
}

func gcLogGrowAt(idx int64, p unsafe.Pointer) {
	ifaceMu.Lock()
	if idx >= int64(len(gcLog.entries)) {
		newLen := len(gcLog.entries) * 2
		if newLen == 0 {
			newLen = 1 << 20
		}
		if int64(newLen) <= idx {
			newLen = int(idx) + 1
		}
		grown := make([]unsafe.Pointer, newLen)
		copy(grown, gcLog.entries)
		gcLog.entries = grown
	}
	gcLog.entries[idx] = p
	ifaceMu.Unlock()
}

// CheckGC runs deferred GC maintenance if needed. The current NaN-box runtime
// keeps a conservative append-only root log because Go and JIT/native frames can
// temporarily hold pointer-tagged Values outside VM register scanning.
func CheckGC() {
	if atomic.LoadInt32(&gcNeedsCompact) != 0 && atomic.CompareAndSwapInt32(&gcNeedsCompact, 1, 0) {
		gcCompact()
	}
}

// RequestGCCompaction records a script request for GC maintenance. It does not
// make the root log precise: some live NaN-boxed values can sit in Go/native/JIT
// frames that are not visible to VM root scanners.
func RequestGCCompaction() {
	if atomic.LoadInt32(&activeVMCount) == 0 {
		return
	}
	atomic.StoreInt32(&gcNeedsCompact, 1)
}

// ForceGCCompaction preserves compatibility with tests and diagnostics. It is
// intentionally conservative: runtime correctness depends on the root log
// keeping all previously published NaN-boxed pointers alive.
func ForceGCCompaction() {
	if atomic.LoadInt32(&activeVMCount) == 0 {
		return
	}
	gcCompact()
	atomic.StoreInt32(&gcNeedsCompact, 0)
}

// gcCompact used to rebuild gcRootLog with only pointers reachable from active
// VMs. That is unsafe for a NaN-boxed runtime with JIT/native frames: a Value
// can be live in a Go argument slice, native frame, or CPU register while not
// present in VM registers. Keep the log append-only until the runtime has a
// proven stop-the-world safe point that materializes every live Value.
func gcCompact() {
	atomic.StoreInt32(&gcNeedsCompact, 0)
}

// ScanValueRoots scans a single Value for GC root pointers.
// If the value is a pointer type, calls visitor with its raw pointer.
// If it's a table, recursively scans the table's contents.
// The `seen` map prevents infinite loops on circular table references.
func ScanValueRoots(v Value, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	bits := uint64(v)
	if bits&nanBits != nanBits {
		return // float, no pointer
	}
	if bits&tagMask != tagPtr {
		return // nil, bool, or int — no pointer
	}
	p := v.ptrPayload()
	if p == nil {
		return
	}
	addr := uintptr(p)

	sub := bits & ptrSubMask
	if sub == ptrSubTable {
		visitTableRoot(p, visitor)
		if _, already := seen[addr]; already {
			return
		}
		seen[addr] = struct{}{}
		t := (*Table)(p)
		scanTableRoots(t, visitor, seen)
		return
	}
	if sub == ptrSubFixedRecord {
		visitor(p)
		fr := (*FixedRecord)(p)
		scanFixedRecordRoots(fr, visitor, seen)
		return
	}
	if sub == ptrSubLazyString {
		visitor(p)
		ls := (*lazyString)(p)
		scanLazyStringRoots(ls, visitor, seen)
		return
	}
	if sub == ptrSubClosure {
		visitor(p)
		closure := (*Closure)(p)
		scanClosureRoots(closure, visitor, seen)
		return
	}
	if sub == ptrSubCoroutine {
		visitor(p)
		co := (*Coroutine)(p)
		scanCoroutineRoots(co, visitor, seen)
		return
	}
	if sub == ptrSubVMClosure {
		visitor(p)
		if vmClosureRootScanner != nil {
			vmClosureRootScanner(p, visitor, seen)
		}
		return
	}
	if sub == ptrSubVMCoroutine {
		visitor(p)
		if vmCoroutineRootScanner != nil {
			vmCoroutineRootScanner(p, visitor, seen)
		}
		return
	}
	if sub == ptrSubString {
		visitStringRoot(p, visitor)
		return
	}
	if sub == ptrSubSoA {
		visitor(p)
		soa := (*SoA)(p)
		scanSoARoots(soa, visitor, seen)
		return
	}
	visitor(p)
}

func visitTableRoot(p unsafe.Pointer, visitor func(unsafe.Pointer)) {
	visitor(p)
	if root := tableSlabRootForPointer(p); root != nil && root != p {
		visitor(root)
	}
}

// scanTableRoots scans all Values inside a table for GC root pointers.
func scanTableRoots(t *Table, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if t == nil {
		return
	}

	// Array part (only ArrayMixed can contain pointers)
	if t.arrayKind == ArrayMixed && t.array != nil {
		for _, v := range t.array {
			ScanValueRoots(v, visitor, seen)
		}
	}
	// String-keyed values (svals)
	if t.svals != nil {
		for _, v := range t.svals {
			ScanValueRoots(v, visitor, seen)
		}
	}
	if t.lazyTree != nil {
		for _, v := range t.lazyTree.childValues {
			ScanValueRoots(v, visitor, seen)
		}
	}
	for _, k := range t.keys {
		ScanValueRoots(k, visitor, seen)
	}
	ScanValueRoots(t.nextKey, visitor, seen)
	// String map values
	for _, v := range t.smap {
		ScanValueRoots(v, visitor, seen)
	}
	// Integer map values
	for _, v := range t.imap {
		ScanValueRoots(v, visitor, seen)
	}
	// General hash values (both keys and values can be pointers)
	for k, v := range t.hash {
		ScanValueRoots(k, visitor, seen)
		ScanValueRoots(v, visitor, seen)
	}
	// Metatable
	if t.metatable != nil {
		mp := unsafe.Pointer(t.metatable)
		addr := uintptr(mp)
		if _, already := seen[addr]; !already {
			seen[addr] = struct{}{}
			visitTableRoot(mp, visitor)
			scanTableRoots(t.metatable, visitor, seen)
		}
	}
}

func scanClosureRoots(cl *Closure, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if cl == nil {
		return
	}
	for _, uv := range cl.Upvalues {
		if uv != nil {
			ScanValueRoots(uv.Get(), visitor, seen)
		}
	}
	if cl.Env != nil {
		scanEnvironmentRoots(cl.Env, visitor, seen)
	}
}

func scanEnvironmentRoots(env *Environment, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if env == nil {
		return
	}
	addr := uintptr(unsafe.Pointer(env))
	if _, already := seen[addr]; already {
		return
	}
	seen[addr] = struct{}{}

	env.mu.RLock()
	upvalues := make([]*Upvalue, 0, len(env.vars))
	for _, uv := range env.vars {
		upvalues = append(upvalues, uv)
	}
	parent := env.parent
	env.mu.RUnlock()

	for _, uv := range upvalues {
		if uv != nil {
			ScanValueRoots(uv.Get(), visitor, seen)
		}
	}
	scanEnvironmentRoots(parent, visitor, seen)
}

func scanCoroutineRoots(co *Coroutine, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if co == nil {
		return
	}
	ScanValueRoots(co.fn, visitor, seen)
}

// ScanTableRootsExported is the exported version of scanTableRoots.
// Used by the vm package to scan string metatables and other table roots.
func ScanTableRootsExported(t *Table, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	scanTableRoots(t, visitor, seen)
}

func scanCachedIntStringRoots(visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	for _, v := range cachedIntStringValues {
		ScanValueRoots(v, visitor, seen)
	}
}

// GCRootLogSize returns the current number of entries in the root log (for diagnostics).
func GCRootLogSize() int64 {
	return atomic.LoadInt64(&gcLog.cursor)
}

// GCRootScannerCount returns the number of active VM root scanners. It is a
// diagnostic hook used by VM lifecycle tests and testkit memory snapshots.
func GCRootScannerCount() int {
	return int(atomic.LoadInt32(&activeVMCount))
}

// keepAliveIface registers a Go-heap pointer AND stores the full interface
// for later type recovery via lookupIface. Used only for AnyFunction/AnyCoroutine.
// Always global: ifaceRoots entries are keyed by address and never deleted, so
// the object must stay alive forever to prevent address-reuse collisions.
func keepAliveIface(p unsafe.Pointer, obj any) {
	keepAliveGlobal(p)
	ifaceMu.Lock()
	ifaceRoots[uintptr(p)] = obj
	ifaceMu.Unlock()
}

// lookupIface retrieves the original interface{} stored for a given pointer.
// Used by Ptr()/Closure()/GoFunction() for interface-based function/coroutine values.
func lookupIface(p unsafe.Pointer) any {
	ifaceMu.Lock()
	v := ifaceRoots[uintptr(p)]
	ifaceMu.Unlock()
	return v
}

// RegisterVMCoroutinePtrResolver lets the VM package preserve Value.Ptr()
// compatibility for VM coroutine values without storing every coroutine in the
// interface-root map.
func RegisterVMCoroutinePtrResolver(fn func(unsafe.Pointer) any) {
	vmCoroutinePtrResolver = fn
}

// RegisterVMClosureRootScanner lets the VM package recursively scan bytecode
// closure roots held inside runtime.Value tables without making runtime import
// vm. The scanner must visit closure upvalues and proto constants.
func RegisterVMClosureRootScanner(fn func(unsafe.Pointer, func(unsafe.Pointer), map[uintptr]struct{})) {
	vmClosureRootScanner = fn
}

// RegisterVMCoroutineRootScanner lets the VM package recursively scan
// coroutine roots held inside runtime.Value without importing vm.
func RegisterVMCoroutineRootScanner(fn func(unsafe.Pointer, func(unsafe.Pointer), map[uintptr]struct{})) {
	vmCoroutineRootScanner = fn
}
