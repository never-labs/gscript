package data

import (
	"sync"
	"unsafe"
)

const groupIDCacheMaxEntries = 128

type groupIDCacheKey struct {
	kind Kind
	ptr  uintptr
	len  int
}

type groupIDCacheEntry struct {
	source Array
	ids    []int
	count  int
}

var groupIDCache = struct {
	sync.Mutex
	entries map[groupIDCacheKey]groupIDCacheEntry
	order   []groupIDCacheKey
}{entries: make(map[groupIDCacheKey]groupIDCacheEntry)}

func cachedDenseGroupIDs(array Array) ([]int, int, bool) {
	key, source, ok := denseGroupIDCacheKey(array)
	if !ok {
		return nil, 0, false
	}
	groupIDCache.Lock()
	entry, ok := groupIDCache.entries[key]
	groupIDCache.Unlock()
	if !ok || entry.source.Len() != source.Len() {
		return nil, 0, false
	}
	ids := bulkIntGetLen(len(entry.ids))
	copy(ids, entry.ids)
	return ids, entry.count, true
}

func storeDenseGroupIDs(array Array, ids []int, count int) {
	key, source, ok := denseGroupIDCacheKey(array)
	if !ok || len(ids) == 0 || len(ids) > bulkPoolMaxLen {
		return
	}
	owned := append([]int(nil), ids...)
	groupIDCache.Lock()
	if _, exists := groupIDCache.entries[key]; !exists {
		groupIDCache.order = append(groupIDCache.order, key)
	}
	groupIDCache.entries[key] = groupIDCacheEntry{source: source, ids: owned, count: count}
	for len(groupIDCache.order) > groupIDCacheMaxEntries {
		evict := groupIDCache.order[0]
		copy(groupIDCache.order, groupIDCache.order[1:])
		groupIDCache.order = groupIDCache.order[:len(groupIDCache.order)-1]
		delete(groupIDCache.entries, evict)
	}
	groupIDCache.Unlock()
}

func denseGroupIDCacheKey(array Array) (groupIDCacheKey, Array, bool) {
	source := unwrapAttributedArray(array)
	switch a := source.(type) {
	case columnArray[Symbol]:
		return sliceGroupIDCacheKey(a.kind, a.data), source, len(a.data) > 0
	case columnArray[string]:
		return sliceGroupIDCacheKey(a.kind, a.data), source, len(a.data) > 0
	case columnArray[int64]:
		return sliceGroupIDCacheKey(a.kind, a.data), source, len(a.data) > 0
	case columnArray[int32]:
		return sliceGroupIDCacheKey(a.kind, a.data), source, len(a.data) > 0
	case columnArray[int16]:
		return sliceGroupIDCacheKey(a.kind, a.data), source, len(a.data) > 0
	case columnArray[int8]:
		return sliceGroupIDCacheKey(a.kind, a.data), source, len(a.data) > 0
	case columnArray[bool]:
		return sliceGroupIDCacheKey(a.kind, a.data), source, len(a.data) > 0
	default:
		return groupIDCacheKey{}, nil, false
	}
}

func sliceGroupIDCacheKey[T any](kind Kind, values []T) groupIDCacheKey {
	return groupIDCacheKey{
		kind: kind,
		ptr:  uintptr(unsafe.Pointer(unsafe.SliceData(values))),
		len:  len(values),
	}
}
