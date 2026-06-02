// table_string.go — string-key access fast paths and shape transition helpers.
//
// Pure code movement from table.go: RawGetString / RawSetString and their
// inline-cache and dynamic-cache variants, plus the shape descriptor helpers
// (setShape/applyShape/appendShape) and small-string field append/delete
// primitives they depend on.

package runtime

// RawGetString retrieves a value by string key (fast path, no Value boxing).
func (t *Table) RawGetString(key string) Value {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		return t.lazyTree.get(t, key)
	}
	for i, k := range t.skeys {
		if k == key {
			return t.svals[i]
		}
	}
	if t.smap != nil {
		data, keyLen := stringCacheKey(key)
		if v, ok := t.lookupStringMapValueCacheLocked(key, data, keyLen); ok {
			return v
		}
		if v, ok := t.smap[key]; ok {
			t.rememberStringMapValueCacheLocked(key, data, keyLen, v)
			return v
		}
	}
	return NilValue()
}

// RawGetStringNoCache retrieves a string field without populating lookup caches
// or otherwise mutating table iteration state. It is intended for snapshotting
// shared read-mostly tables while creating isolated concurrent runtimes.
func (t *Table) RawGetStringNoCache(key string) Value {
	if t == nil {
		return NilValue()
	}
	if t.mu != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	if t.lazyTree != nil {
		return t.lazyTree.get(t, key)
	}
	for i, k := range t.skeys {
		if k == key {
			return t.svals[i]
		}
	}
	if t.smap != nil {
		if v, ok := t.smap[key]; ok {
			return v
		}
	}
	return NilValue()
}

// RawGetStringCached retrieves a value by string key using an inline cache hint.
// The cache stores the field index and the table's shapeID from a previous lookup.
// On cache hit (shapeID match), avoids both string comparison and O(n) scan.
// Works across different tables sharing the same field layout.
func (t *Table) RawGetStringCached(key string, cache *FieldCacheEntry) Value {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		return t.lazyTree.get(t, key)
	}
	// ShapeID-based cache: if shape matches, the field index is valid
	idx := cache.FieldIdx
	if t.shapeID != 0 && cache.ShapeID == t.shapeID && idx >= 0 && idx < len(t.svals) {
		return t.svals[idx]
	}
	// Cache miss — linear scan and update cache
	for i, k := range t.skeys {
		if k == key {
			cache.FieldIdx = i
			cache.ShapeID = t.shapeID
			return t.svals[i]
		}
	}
	if t.smap != nil {
		data, keyLen := stringCacheKey(key)
		if v, ok := t.lookupStringMapValueCacheLocked(key, data, keyLen); ok {
			return v
		}
		if v, ok := t.smap[key]; ok {
			t.rememberStringMapValueCacheLocked(key, data, keyLen, v)
			return v
		}
	}
	return NilValue()
}

// RawGetStringCachedPoly retrieves a static string field and also populates a
// small polymorphic shape cache for the bytecode PC. The monomorphic cache is
// still updated because it remains the shortest native fast path.
func (t *Table) RawGetStringCachedPoly(key string, cache *FieldCacheEntry, poly []FieldPolyCacheEntry) Value {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		return t.lazyTree.get(t, key)
	}
	idx := cache.FieldIdx
	if t.shapeID != 0 && cache.ShapeID == t.shapeID && idx >= 0 && idx < len(t.svals) {
		t.rememberFieldPolyCacheLocked(idx, poly)
		return t.svals[idx]
	}
	for i, k := range t.skeys {
		if k == key {
			cache.FieldIdx = i
			cache.ShapeID = t.shapeID
			t.rememberFieldPolyCacheLocked(i, poly)
			return t.svals[i]
		}
	}
	if t.smap != nil {
		data, keyLen := stringCacheKey(key)
		if v, ok := t.lookupStringMapValueCacheLocked(key, data, keyLen); ok {
			return v
		}
		if v, ok := t.smap[key]; ok {
			t.rememberStringMapValueCacheLocked(key, data, keyLen, v)
			return v
		}
	}
	return NilValue()
}

// RawGetStringDynamicCached retrieves a dynamic string key using a small
// polymorphic per-PC cache. The cache is valid only for shaped small-string
// tables; misses and large string maps use the normal path.
func (t *Table) RawGetStringDynamicCached(key string, cache []TableStringKeyCacheEntry) Value {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		return t.lazyTree.get(t, key)
	}
	data, keyLen := stringCacheKey(key)
	if idx, ok := t.lookupDynamicStringCacheLocked(data, keyLen, cache); ok {
		RecordRuntimePathTableStringGetCacheHit()
		return t.svals[idx]
	}
	for i, k := range t.skeys {
		if k == key {
			t.rememberDynamicStringCacheLocked(key, data, keyLen, i, cache)
			RecordRuntimePathTableStringGetScanHit()
			return t.svals[i]
		}
	}
	if t.smap != nil {
		if v, ok := t.lookupStringMapValueCacheLocked(key, data, keyLen); ok {
			RecordRuntimePathTableStringGetMapHit()
			return v
		}
		if v, ok := t.smap[key]; ok {
			t.rememberStringMapValueCacheLocked(key, data, keyLen, v)
			RecordRuntimePathTableStringGetMapHit()
			return v
		}
	}
	RecordRuntimePathTableStringGetMiss()
	return NilValue()
}

// RawSetStringCached assigns a value by string key using an inline cache hint.
// Uses shapeID-based cache to find existing keys faster on cache hit.
func (t *Table) RawSetStringCached(key string, val Value, cache *FieldCacheEntry) {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		t.materializeLazyTreeLocked()
	}
	valIsNil := val.IsNil()
	if valIsNil && t.shapeID == 0 && len(t.skeys) == 0 && t.smap == nil {
		return
	}
	t.keysDirty = true
	if valIsNil && (t.smap != nil || t.stringLookupCache != nil) {
		t.invalidateStringLookupCacheLocked()
	}

	// ShapeID-based cache: if shape matches, the field index is valid
	idx := cache.FieldIdx
	if t.shapeID != 0 && cache.ShapeID == t.shapeID && idx >= 0 && idx < len(t.svals) {
		oldShapeID := t.shapeID
		if valIsNil {
			t.deleteSmallStringField(idx)
			cache.FieldIdx = 0 // reset cache
			cache.ShapeID = 0
		} else {
			t.svals[idx] = val
			ObserveShapeFieldValue(oldShapeID, idx, val)
		}
		RecordShapeFieldMutation(oldShapeID, idx)
		return
	}

	if !valIsNil &&
		t.smap == nil &&
		cache.AppendShapeID == t.shapeID &&
		idx == len(t.svals) &&
		idx == len(t.skeys) &&
		idx < smallFieldCap {
		if cache.AppendShape != nil {
			t.appendSmallStringValue(val)
			t.applyShape(cache.AppendShape)
		} else {
			t.appendSmallStringField(key, val)
			cache.AppendShape = t.shape
		}
		cache.ShapeID = t.shapeID
		return
	}

	// Fall back to normal path
	for i, k := range t.skeys {
		if k == key {
			oldShapeID := t.shapeID
			if valIsNil {
				t.deleteSmallStringField(i)
			} else {
				t.svals[i] = val
				cache.FieldIdx = i
				cache.ShapeID = t.shapeID
				ObserveShapeFieldValue(oldShapeID, i, val)
			}
			t.bumpStringLookupVersionLocked()
			RecordShapeFieldMutation(oldShapeID, i)
			return
		}
	}

	if t.smap != nil {
		t.bumpStringLookupVersionLocked()
		if valIsNil {
			delete(t.smap, key)
		} else {
			t.smap[key] = val
			data, keyLen := stringCacheKey(key)
			t.rememberStringMapValueCacheIfPresentLocked(key, data, keyLen, val)
		}
		return
	}

	if !valIsNil {
		if len(t.skeys) < smallFieldCap {
			preShapeID := t.shapeID
			idx := len(t.svals)
			t.appendSmallStringField(key, val)
			t.bumpStringLookupVersionLocked()
			cache.FieldIdx = idx
			cache.ShapeID = t.shapeID
			cache.AppendShapeID = preShapeID
			cache.AppendShape = t.shape
		} else {
			RecordShapeMutation(t.shapeID)
			t.promoteStringFieldsToMapLocked(key, val)
		}
	}
}

// RawSetStringDynamicCached assigns a dynamic string key and updates the
// per-PC polymorphic cache when the key resolves to a small shaped-table field.
func (t *Table) RawSetStringDynamicCached(key string, val Value, cache []TableStringKeyCacheEntry) {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		t.materializeLazyTreeLocked()
	}
	valIsNil := val.IsNil()
	if valIsNil && t.shapeID == 0 && len(t.skeys) == 0 && t.smap == nil {
		return
	}
	t.keysDirty = true
	if valIsNil && (t.smap != nil || t.stringLookupCache != nil) {
		t.invalidateStringLookupCacheLocked()
	}
	data, keyLen := stringCacheKey(key)

	if !valIsNil {
		if idx, ok := t.lookupDynamicStringCacheLocked(data, keyLen, cache); ok {
			RecordShapeFieldMutation(t.shapeID, idx)
			t.svals[idx] = val
			ObserveShapeFieldValue(t.shapeID, idx, val)
			t.bumpStringLookupVersionLocked()
			RecordRuntimePathTableStringSetCacheHit()
			return
		}
	}

	for i, k := range t.skeys {
		if k == key {
			oldShapeID := t.shapeID
			if valIsNil {
				t.deleteSmallStringField(i)
			} else {
				t.svals[i] = val
				t.rememberDynamicStringCacheLocked(key, data, keyLen, i, cache)
				ObserveShapeFieldValue(oldShapeID, i, val)
			}
			t.bumpStringLookupVersionLocked()
			RecordShapeFieldMutation(oldShapeID, i)
			RecordRuntimePathTableStringSetScanHit()
			return
		}
	}

	if t.smap != nil {
		t.bumpStringLookupVersionLocked()
		if valIsNil {
			delete(t.smap, key)
		} else {
			t.smap[key] = val
			t.rememberStringMapValueCacheIfPresentLocked(key, data, keyLen, val)
		}
		RecordRuntimePathTableStringSetMap()
		return
	}

	if !valIsNil {
		if len(t.skeys) < smallFieldCap {
			idx := len(t.svals)
			preShapeID := t.shapeID
			t.appendSmallStringField(key, val)
			t.bumpStringLookupVersionLocked()
			t.rememberDynamicStringCacheLocked(key, data, keyLen, idx, cache)
			if cache != nil {
				for i := range cache {
					if cache[i].KeyData == data && cache[i].KeyLen == keyLen && cache[i].FieldIdx == idx && cache[i].ShapeID == t.shapeID {
						cache[i].AppendShapeID = preShapeID
						cache[i].AppendShape = t.shape
						break
					}
				}
			}
			RecordRuntimePathTableStringSetAppend()
		} else {
			RecordShapeMutation(t.shapeID)
			RecordShapeLayoutMutation(t.shapeID)
			t.promoteStringFieldsToMapLocked(key, val)
			RecordRuntimePathTableStringSetPromote()
		}
	}
}

// setShape updates both t.shape and t.shapeID from the current skeys.
// Pass nil/empty skeys to clear (hash-mode or empty table).
// Must be called with lock held (if mu != nil).
func (t *Table) setShape(skeys []string) {
	t.applyShape(GetShape(skeys))
}

func (t *Table) applyShape(s *Shape) {
	t.shape = s
	if s != nil {
		t.shapeID = s.ID
		t.skeys = s.FieldKeys
	} else {
		t.shapeID = 0
		t.skeys = nil
	}
}

// appendShape advances the hidden-class descriptor for the common case where a
// new string field is appended. It avoids rebuilding the full joined shape key
// for every object with the same field insertion order.
func (t *Table) appendShape(key string) {
	if t.shapeID != 0 {
		RecordShapeLayoutMutation(t.shapeID)
	}
	var s *Shape
	if t.shape != nil {
		s = t.shape.Transition(key)
	} else {
		s = getOrCreateSingleFieldShape(key)
	}
	t.shape = s
	if s != nil {
		t.shapeID = s.ID
		t.skeys = s.FieldKeys
	} else {
		t.shapeID = 0
		t.skeys = nil
	}
}

func (t *Table) appendSmallStringField(key string, val Value) {
	t.appendSmallStringValue(val)
	t.appendShape(key)
	ObserveShapeFieldValue(t.shapeID, len(t.svals)-1, val)
}

func (t *Table) appendSmallStringValue(val Value) {
	if len(t.svals) < cap(t.svals) {
		n := len(t.svals)
		t.svals = t.svals[:n+1]
		t.svals[n] = val
	} else {
		arenaAppendValue(DefaultHeap, &t.svals, val)
	}
}

// deleteSmallStringField removes skeys[idx]/svals[idx] from a small string
// table. skeys may alias an immutable Shape.FieldKeys slice, so this must not
// mutate the key slice in place. Value order follows the historical swap-delete
// behavior used by RawSetString.
func (t *Table) deleteSmallStringField(idx int) {
	last := len(t.skeys) - 1
	if idx < 0 || idx > last {
		return
	}
	if idx != last {
		t.svals[idx] = t.svals[last]
	}
	RecordShapeLayoutMutation(t.shapeID)
	t.svals = t.svals[:last]
	if last == 0 {
		t.setShape(nil)
		return
	}
	keys := make([]string, last)
	copy(keys, t.skeys[:last])
	if idx != last {
		keys[idx] = t.skeys[last]
	}
	t.setShape(keys)
}

// RawSetString assigns a value by string key (fast path).
func (t *Table) RawSetString(key string, val Value) {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		t.materializeLazyTreeLocked()
	}
	valIsNil := val.IsNil()
	if valIsNil && t.shapeID == 0 && len(t.skeys) == 0 && t.smap == nil {
		return
	}
	t.keysDirty = true
	if valIsNil && (t.smap != nil || t.stringLookupCache != nil) {
		t.invalidateStringLookupCacheLocked()
	}

	for i, k := range t.skeys {
		if k == key {
			oldShapeID := t.shapeID
			if valIsNil {
				t.deleteSmallStringField(i)
			} else {
				t.svals[i] = val
				ObserveShapeFieldValue(oldShapeID, i, val)
			}
			t.bumpStringLookupVersionLocked()
			RecordShapeFieldMutation(oldShapeID, i)
			return
		}
	}

	if t.smap != nil {
		t.bumpStringLookupVersionLocked()
		if valIsNil {
			delete(t.smap, key)
		} else {
			t.smap[key] = val
			data, keyLen := stringCacheKey(key)
			t.rememberStringMapValueCacheIfPresentLocked(key, data, keyLen, val)
		}
		return
	}

	if !valIsNil {
		if len(t.skeys) < smallFieldCap {
			t.appendSmallStringField(key, val)
			t.bumpStringLookupVersionLocked()
		} else {
			RecordShapeMutation(t.shapeID)
			RecordShapeLayoutMutation(t.shapeID)
			t.promoteStringFieldsToMapLocked(key, val)
		}
	}
}
