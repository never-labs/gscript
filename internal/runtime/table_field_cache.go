// table_field_cache.go — inline-cache types, constants, process-wide native
// query caches, and the per-table string-lookup cache helpers.
//
// Pure code movement from table.go: FieldCacheEntry / FieldPolyCacheEntry /
// TableStringKeyCacheEntry / StringLookupCache(Entry) / native query cache
// entry types, their slot accessors, and the lock-held remember/lookup helpers
// for string-map value caches and dynamic/poly field caches.

package runtime

import "unsafe"

// FieldCacheEntry is a hint-based inline cache entry for field access.
// It caches the index of a field name in a table's skeys slice and the
// table's shapeID when the cache was populated. On lookup, if the table's
// shapeID matches the cached shapeID, the field index is valid without
// needing string comparison. Works across different tables with the
// same field layout (e.g., all nbody body tables).
type FieldCacheEntry struct {
	FieldIdx      int    // cached index into skeys/svals (-1 = not cached)
	ShapeID       uint32 // shapeID when cache was populated for existing-field access
	AppendShapeID uint32 // pre-append shapeID for constructor-style SETFIELD
	AppendShape   *Shape // result shape for constructor-style SETFIELD
}

// FieldPolyCacheWays is the number of polymorphic static string-field cache
// entries assigned to each bytecode PC.
const FieldPolyCacheWays = 4

// FieldPolyCacheEntry caches a static string-field lookup by table shape.
// It complements FieldCacheEntry's monomorphic fast path for object dispatch
// sites that alternate among a small number of stable shapes.
type FieldPolyCacheEntry struct {
	FieldIdx int
	ShapeID  uint32
}

// FieldPolyCacheSlot returns the cache ways for one bytecode PC.
func FieldPolyCacheSlot(cache []FieldPolyCacheEntry, pc int) []FieldPolyCacheEntry {
	if pc < 0 {
		return nil
	}
	start := pc * FieldPolyCacheWays
	end := start + FieldPolyCacheWays
	if start < 0 || end > len(cache) {
		return nil
	}
	return cache[start:end]
}

// TableStringKeyCacheWays is the number of polymorphic dynamic string-key
// table cache entries assigned to each bytecode PC.
const TableStringKeyCacheWays = 8

// TableStringKeyCacheEntry caches a dynamic string-key table lookup by string
// backing pointer/length plus table shape. It is a hint only: callers must
// fall back to the normal table path on miss.
type TableStringKeyCacheEntry struct {
	Key      string
	KeyData  uintptr
	KeyLen   int
	FieldIdx int
	ShapeID  uint32
	// AppendShapeID/AppendShape describe the transition used when this key is
	// appended to a small shaped table. They are hints for JIT fast paths; the
	// regular table path remains authoritative on a mismatch.
	AppendShapeID uint32
	AppendShape   *Shape
}

// StringLookupCacheEntry is a table-local value cache for large string maps.
// It is intentionally separate from TableStringKeyCacheEntry: per-PC caches
// identify small-table field slots by shape, while this cache identifies stable
// entries in a large map by key backing pointer/length. String-map mutations
// advance the owning table's version, so native query-cache probes never see
// stale entries after an update or delete.
type StringLookupCacheEntry struct {
	Key     string
	KeyData uintptr
	KeyLen  int
	Value   Value
	Hash    uintptr
	Valid   uint8
	_       [15]byte
}

// StringLookupCache is an open-addressed direct value cache owned by one table.
// Mask is len(Entries)-1 so native code can probe without calling into Go.
type StringLookupCache struct {
	Entries []StringLookupCacheEntry
	Mask    uintptr
}

// NativeStringQueryCacheEntry is a process-wide Tier 2 hint for repeated
// dynamic string-key lookups where the query string object is stable. The
// table version makes stale entries harmless after string-map mutation.
type NativeStringQueryCacheEntry struct {
	Table   uintptr
	Version uint64
	KeyData uintptr
	KeyLen  uintptr
	Value   Value
}

// NativeFormattedIntQueryCacheEntry caches dynamic string-key lookups whose
// key is produced by string.format(pattern, int). This lets Tier 2 skip both
// the formatted string object and the string-map probe when the table version
// is unchanged.
type NativeFormattedIntQueryCacheEntry struct {
	Table       uintptr
	Version     uint64
	PatternData uintptr
	PatternLen  uintptr
	N           int64
	Value       Value
}

const (
	stringLookupCacheMinEntries = 256
	stringLookupCacheMaxEntries = 16384
	stringLookupCacheProbeLimit = 8
	// StringLookupCacheProbeLimit is exported for native cache probes that must
	// mirror the runtime insertion bound.
	StringLookupCacheProbeLimit = stringLookupCacheProbeLimit

	NativeStringQueryCacheSize       = 65536
	NativeStringQueryCacheWays       = 4
	NativeStringQueryCacheSets       = NativeStringQueryCacheSize / NativeStringQueryCacheWays
	NativeFormattedIntQueryCacheSize = 65536
)

var nativeStringQueryCache [NativeStringQueryCacheSize]NativeStringQueryCacheEntry
var nativeFormattedIntQueryCache [NativeFormattedIntQueryCacheSize]NativeFormattedIntQueryCacheEntry

// NativeStringQueryCachePtr returns the base address for the Tier 2 dynamic
// string-key query cache.
func NativeStringQueryCachePtr() unsafe.Pointer {
	return unsafe.Pointer(&nativeStringQueryCache[0])
}

func NativeFormattedIntQueryCachePtr() unsafe.Pointer {
	return unsafe.Pointer(&nativeFormattedIntQueryCache[0])
}

func nativeFormattedIntQueryCacheSlot(table uintptr, pattern string, n int64) *NativeFormattedIntQueryCacheEntry {
	slot := (table ^ stringDataPtr(pattern) ^ uintptr(n)) & uintptr(NativeFormattedIntQueryCacheSize-1)
	return &nativeFormattedIntQueryCache[slot]
}

func stringDataPtr(s string) uintptr {
	if len(s) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(unsafe.StringData(s)))
}

// RawGetStringFormatIntCached formats pattern with n, performs a dynamic
// string-key table lookup, and populates the native formatted-int query cache.
func RawGetStringFormatIntCached(t *Table, pattern string, n int64, cache []TableStringKeyCacheEntry) (Value, bool, error) {
	keyVal, ok, err := StringFormatSingleInt(pattern, n)
	if err != nil || !ok {
		return NilValue(), false, err
	}
	key := keyVal.Str()
	result := t.RawGetStringDynamicCached(key, cache)
	entry := nativeFormattedIntQueryCacheSlot(uintptr(unsafe.Pointer(t)), pattern, n)
	entry.Table = uintptr(unsafe.Pointer(t))
	entry.Version = t.stringLookupVersion
	entry.PatternData = stringDataPtr(pattern)
	entry.PatternLen = uintptr(len(pattern))
	entry.N = n
	entry.Value = result
	return result, true, nil
}

// TableStringKeyCacheSlot returns the cache ways for one bytecode PC.
func TableStringKeyCacheSlot(cache []TableStringKeyCacheEntry, pc int) []TableStringKeyCacheEntry {
	if pc < 0 {
		return nil
	}
	start := pc * TableStringKeyCacheWays
	end := start + TableStringKeyCacheWays
	if start < 0 || end > len(cache) {
		return nil
	}
	return cache[start:end]
}

func stringCacheKey(key string) (uintptr, int) {
	if len(key) == 0 {
		return 0, 0
	}
	return uintptr(unsafe.Pointer(unsafe.StringData(key))), len(key)
}

func dynamicStringCacheReplaceIndex(data uintptr, keyLen, n int) int {
	if n <= 1 {
		return 0
	}
	h := (data >> 4) ^ (data >> 12) ^ uintptr(keyLen)
	return int(h % uintptr(n))
}

func stringLookupHashString(key string) uintptr {
	h := uintptr(1469598103934665603)
	for i := 0; i < len(key); i++ {
		h ^= uintptr(key[i])
		h *= 1099511628211
	}
	if h == 0 {
		return 1
	}
	return h
}

func stringLookupCacheSize(n int) int {
	size := stringLookupCacheMinEntries
	target := n * 2
	for size < target && size < stringLookupCacheMaxEntries {
		size <<= 1
	}
	return size
}

func (t *Table) invalidateStringLookupCacheLocked() {
	t.stringLookupCache = nil
}

func (t *Table) bumpStringLookupVersionLocked() {
	if t.stringLookupVersion == 0 {
		return
	}
	t.stringLookupVersion++
	if t.stringLookupVersion == 0 {
		t.stringLookupVersion = 1
	}
}

func (t *Table) enableStringLookupVersionLocked() uint64 {
	if t.stringLookupVersion == 0 {
		t.stringLookupVersion = 1
	}
	return t.stringLookupVersion
}

func (t *Table) promoteStringFieldsToMapLocked(key string, val Value) {
	t.enableStringLookupVersionLocked()
	t.smap = make(map[string]Value, initialStringMapCap)
	for i, k := range t.skeys {
		t.smap[k] = t.svals[i]
	}
	t.smap[key] = val
	t.skeys = nil
	t.svals = nil
	t.setShape(nil)
}

func (t *Table) ensureStringLookupCacheLocked() *StringLookupCache {
	if t.smap == nil {
		return nil
	}
	wantSize := stringLookupCacheSize(len(t.smap))
	if c := t.stringLookupCache; c != nil && len(c.Entries) >= wantSize {
		return c
	}
	c := &StringLookupCache{
		Entries: make([]StringLookupCacheEntry, wantSize),
		Mask:    uintptr(wantSize - 1),
	}
	t.stringLookupCache = c
	for key, val := range t.smap {
		data, keyLen := stringCacheKey(key)
		rememberStringMapValueCacheEntry(c, key, data, keyLen, val)
	}
	return c
}

func (t *Table) lookupStringMapValueCacheLocked(key string, data uintptr, keyLen int) (Value, bool) {
	c := t.stringLookupCache
	if c == nil || len(c.Entries) == 0 {
		return NilValue(), false
	}
	hash := stringLookupHashString(key)
	idx := hash & c.Mask
	for probe := 0; probe < stringLookupCacheProbeLimit; probe++ {
		entry := &c.Entries[idx]
		if entry.Valid == 0 {
			return NilValue(), false
		}
		if entry.Hash == hash && entry.KeyLen == keyLen && (entry.KeyData == data || entry.Key == key) {
			return entry.Value, true
		}
		idx = (idx + 1) & c.Mask
	}
	return NilValue(), false
}

func (t *Table) rememberStringMapValueCacheLocked(key string, data uintptr, keyLen int, val Value) {
	c := t.ensureStringLookupCacheLocked()
	if c == nil || len(c.Entries) == 0 {
		return
	}
	rememberStringMapValueCacheEntry(c, key, data, keyLen, val)
}

func (t *Table) rememberStringMapValueCacheIfPresentLocked(key string, data uintptr, keyLen int, val Value) {
	c := t.stringLookupCache
	if c == nil || len(c.Entries) == 0 {
		return
	}
	rememberStringMapValueCacheEntry(c, key, data, keyLen, val)
}

func rememberStringMapValueCacheEntry(c *StringLookupCache, key string, data uintptr, keyLen int, val Value) {
	if c == nil || len(c.Entries) == 0 {
		return
	}
	hash := stringLookupHashString(key)
	idx := hash & c.Mask
	firstStale := -1
	for probe := 0; probe < stringLookupCacheProbeLimit; probe++ {
		entry := &c.Entries[idx]
		if entry.Valid != 0 && entry.Hash == hash && entry.KeyLen == keyLen && (entry.KeyData == data || entry.Key == key) {
			entry.Key = key
			entry.KeyData = data
			entry.Value = val
			return
		}
		if entry.Valid == 0 {
			firstStale = int(idx)
			break
		}
		idx = (idx + 1) & c.Mask
	}
	if firstStale < 0 {
		firstStale = int(hash & c.Mask)
	}
	c.Entries[firstStale] = StringLookupCacheEntry{
		Key:     key,
		KeyData: data,
		KeyLen:  keyLen,
		Value:   val,
		Hash:    hash,
		Valid:   1,
	}
}

func fieldPolyCacheReplaceIndex(shapeID uint32, n int) int {
	if n <= 1 {
		return 0
	}
	return int(shapeID % uint32(n))
}

func (t *Table) rememberFieldPolyCacheLocked(fieldIdx int, cache []FieldPolyCacheEntry) {
	if t.shapeID == 0 || fieldIdx < 0 || fieldIdx >= len(t.svals) || len(cache) == 0 {
		return
	}
	empty := -1
	for i := range cache {
		entry := &cache[i]
		if entry.ShapeID == t.shapeID {
			entry.FieldIdx = fieldIdx
			return
		}
		if empty < 0 && entry.ShapeID == 0 {
			empty = i
		}
	}
	if empty < 0 {
		empty = fieldPolyCacheReplaceIndex(t.shapeID, len(cache))
	}
	cache[empty] = FieldPolyCacheEntry{
		FieldIdx: fieldIdx,
		ShapeID:  t.shapeID,
	}
}

func (t *Table) lookupDynamicStringCacheLocked(data uintptr, keyLen int, cache []TableStringKeyCacheEntry) (int, bool) {
	shapeID := t.shapeID
	if shapeID == 0 || len(cache) == 0 {
		return 0, false
	}
	for i := range cache {
		entry := &cache[i]
		if entry.ShapeID == shapeID && entry.KeyData == data && entry.KeyLen == keyLen {
			idx := entry.FieldIdx
			if idx >= 0 && idx < len(t.svals) {
				return idx, true
			}
			return 0, false
		}
	}
	return 0, false
}

func (t *Table) rememberDynamicStringCacheLocked(key string, data uintptr, keyLen, fieldIdx int, cache []TableStringKeyCacheEntry) {
	if t.shapeID == 0 || fieldIdx < 0 || fieldIdx >= len(t.svals) || len(cache) == 0 {
		return
	}
	empty := -1
	for i := range cache {
		entry := &cache[i]
		if entry.ShapeID == t.shapeID && entry.KeyData == data && entry.KeyLen == keyLen {
			entry.FieldIdx = fieldIdx
			return
		}
		if empty < 0 && entry.ShapeID == 0 {
			empty = i
		}
	}
	if empty < 0 {
		empty = dynamicStringCacheReplaceIndex(data, keyLen, len(cache))
	}
	cache[empty] = TableStringKeyCacheEntry{
		Key:      key,
		KeyData:  data,
		KeyLen:   keyLen,
		FieldIdx: fieldIdx,
		ShapeID:  t.shapeID,
	}
}
