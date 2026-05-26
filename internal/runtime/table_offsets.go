// table_offsets.go — struct field byte-offset accessors used by the JIT to
// verify its hardcoded Table/cache layout assumptions.
//
// Pure code movement from table.go: the Table*Offset / *Offsets functions and
// the DenseMatrix descriptor offset accessors.

package runtime

import "unsafe"

// TableFieldOffsets returns the byte offsets of key Table fields for JIT verification.
// This allows the JIT to verify its hardcoded offsets match the actual struct layout.
func TableFieldOffsets() (arrayKind, intArray, floatArray, boolArray uintptr) {
	var t Table
	return unsafe.Offsetof(t.arrayKind), unsafe.Offsetof(t.intArray), unsafe.Offsetof(t.floatArray), unsafe.Offsetof(t.boolArray)
}

// TableArrayZeroValidOffset returns the byte offset of the typed numeric
// key-0 presence bit for JIT verification and guards.
func TableArrayZeroValidOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.arrayZeroValid)
}

// TableMapOffsets returns byte offsets of sparse integer/general hash maps for JIT verification.
func TableMapOffsets() (imap, hash uintptr) {
	var t Table
	return unsafe.Offsetof(t.imap), unsafe.Offsetof(t.hash)
}

// TableStringMapOffset returns the byte offset of the large string-key map.
func TableStringMapOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.smap)
}

// TableTypedArrayCapOffsets returns byte offsets of typed-array cap fields for JIT verification.
func TableTypedArrayCapOffsets() (intArrayCap, floatArrayCap, boolArrayCap uintptr) {
	var t Table
	return unsafe.Offsetof(t.intArray) + 16, unsafe.Offsetof(t.floatArray) + 16, unsafe.Offsetof(t.boolArray) + 16
}

// TableKeysDirtyOffset returns the byte offset of the keysDirty field for JIT verification.
func TableKeysDirtyOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.keysDirty)
}

// TableLazyTreeOffset returns the byte offset of the lazy recursive table side
// pointer for JIT guards that must not treat lazy tables as empty shape-less
// tables.
func TableLazyTreeOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.lazyTree)
}

// TableStringLookupCacheOffset returns the byte offset for the table-local
// large string-map lookup cache used by native dynamic GETTABLE probes.
func TableStringLookupCacheOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.stringLookupCache)
}

// TableStringLookupVersionOffset returns the byte offset for the string-map
// mutation version used to validate native dynamic string query cache hits.
func TableStringLookupVersionOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.stringLookupVersion)
}

// TableArrayVersionOffset returns the byte offset for the array-structure
// mutation version used by native record-array validation caches.
func TableArrayVersionOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.arrayVersion)
}

// StringLookupCacheOffsets returns byte offsets for StringLookupCache.
func StringLookupCacheOffsets() (entriesData, entriesLen, entriesCap, mask uintptr) {
	var c StringLookupCache
	entries := unsafe.Offsetof(c.Entries)
	return entries, entries + 8, entries + 16, unsafe.Offsetof(c.Mask)
}

// NativeStringQueryCacheEntryOffsets returns byte offsets for native query
// cache entries.
func NativeStringQueryCacheEntryOffsets() (table, version, keyData, keyLen, value uintptr) {
	var e NativeStringQueryCacheEntry
	return unsafe.Offsetof(e.Table), unsafe.Offsetof(e.Version), unsafe.Offsetof(e.KeyData), unsafe.Offsetof(e.KeyLen), unsafe.Offsetof(e.Value)
}

func NativeFormattedIntQueryCacheEntryOffsets() (table, version, patternData, patternLen, n, value uintptr) {
	var e NativeFormattedIntQueryCacheEntry
	return unsafe.Offsetof(e.Table), unsafe.Offsetof(e.Version), unsafe.Offsetof(e.PatternData), unsafe.Offsetof(e.PatternLen), unsafe.Offsetof(e.N), unsafe.Offsetof(e.Value)
}

// StringLookupCacheEntryOffsets returns byte offsets for StringLookupCacheEntry.
func StringLookupCacheEntryOffsets() (keyData, keyLen, value, hash, valid uintptr) {
	var e StringLookupCacheEntry
	return unsafe.Offsetof(e.KeyData), unsafe.Offsetof(e.KeyLen), unsafe.Offsetof(e.Value), unsafe.Offsetof(e.Hash), unsafe.Offsetof(e.Valid)
}

func TableShapeIDOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.shapeID)
}

// TableShapeOffset returns the offset of shape for JIT verification.
func TableShapeOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.shape)
}

// TableDMFlatOffset / TableDMStrideOffset return the byte offsets of
// the DenseMatrix descriptor fields for JIT verification (R43).
func TableDMFlatOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.dmFlat)
}

func TableDMStrideOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.dmStride)
}

func TableDMMetaOffset() uintptr {
	var t Table
	return unsafe.Offsetof(t.dmMeta)
}

func DenseMatrixMetaOffsets() (backingData, backingLen, backingCap, parent uintptr) {
	var m denseMatrixMeta
	backing := unsafe.Offsetof(m.backing)
	return backing, backing + 8, backing + 16, unsafe.Offsetof(m.parent)
}
