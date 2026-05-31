// table_iter.go — iteration support: ordered-key rebuild, key snapshots, and
// the Lua next() driver.
//
// Pure code movement from table.go: rebuildKeys / needsKeyRebuild /
// PairsKeysSnapshot / Next.

package runtime

// rebuildKeys rebuilds the ordered key list for iteration.
func (t *Table) rebuildKeys() {
	if t.lazyTree != nil {
		t.materializeLazyTreeLocked()
	}
	t.nextValid = false
	t.keys = t.keys[:0]
	// Typed int/float arrays track whether index 0 was explicitly written,
	// because their zero value is otherwise indistinguishable from nil.
	switch t.arrayKind {
	case ArrayInt:
		if t.arrayZeroValid && len(t.intArray) > 0 {
			t.keys = append(t.keys, IntValue(0))
		}
		for i := 1; i < len(t.intArray); i++ {
			t.keys = append(t.keys, IntValue(int64(i)))
		}
	case ArrayFloat:
		if t.arrayZeroValid && len(t.floatArray) > 0 {
			t.keys = append(t.keys, IntValue(0))
		}
		for i := 1; i < len(t.floatArray); i++ {
			t.keys = append(t.keys, IntValue(int64(i)))
		}
	case ArrayBool:
		for i := 0; i < len(t.boolArray); i++ {
			if t.boolArray[i] != 0 { // skip nil/unset slots
				t.keys = append(t.keys, IntValue(int64(i)))
			}
		}
	default:
		for i := 0; i < len(t.array); i++ {
			if !t.array[i].IsNil() {
				t.keys = append(t.keys, IntValue(int64(i)))
			}
		}
	}
	for k, v := range t.imap {
		if !v.IsNil() {
			t.keys = append(t.keys, IntValue(k))
		}
	}
	// Flat string slices
	for i, k := range t.skeys[:min(len(t.skeys), len(t.svals))] {
		if !t.svals[i].IsNil() {
			t.keys = append(t.keys, StringValue(k))
		}
	}
	// Large string map
	for k, v := range t.smap {
		if !v.IsNil() {
			t.keys = append(t.keys, StringValue(k))
		}
	}
	for k, v := range t.hash {
		if !v.IsNil() {
			t.keys = append(t.keys, k)
		}
	}
	t.keysDirty = false
}

func (t *Table) needsKeyRebuild() bool {
	if t.lazyTree != nil {
		return true
	}
	if t.keysDirty {
		return true
	}
	if len(t.keys) != 0 {
		return false
	}
	switch t.arrayKind {
	case ArrayInt:
		if t.arrayZeroValid || len(t.intArray) > 1 {
			return true
		}
	case ArrayFloat:
		if t.arrayZeroValid || len(t.floatArray) > 1 {
			return true
		}
	case ArrayBool:
		for _, b := range t.boolArray {
			if b != 0 {
				return true
			}
		}
	default:
		for _, v := range t.array {
			if !v.IsNil() {
				return true
			}
		}
	}
	for _, v := range t.imap {
		if !v.IsNil() {
			return true
		}
	}
	for _, v := range t.svals {
		if !v.IsNil() {
			return true
		}
	}
	for _, v := range t.smap {
		if !v.IsNil() {
			return true
		}
	}
	for _, v := range t.hash {
		if !v.IsNil() {
			return true
		}
	}
	return false
}

// PairsKeysSnapshot returns a stable key snapshot for pairs-style iteration.
// It intentionally includes keys present at iteration start so deleting the
// current key during traversal does not prevent later keys from being visited.
func (t *Table) PairsKeysSnapshot() []Value {
	if t == nil {
		return nil
	}
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.needsKeyRebuild() {
		t.rebuildKeys()
	}
	keys := make([]Value, len(t.keys))
	copy(keys, t.keys)
	return keys
}

// Next returns the next key/value pair after the given key.
func (t *Table) Next(key Value) (Value, Value, bool) {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.needsKeyRebuild() {
		t.rebuildKeys()
	}
	if len(t.keys) == 0 {
		t.nextValid = false
		return NilValue(), NilValue(), false
	}
	if key.IsNil() {
		k := t.keys[0]
		t.nextKey = k
		t.nextIndex = 0
		t.nextValid = true
		return k, t.rawGetForNextLocked(k), true
	}
	if t.nextValid && t.nextIndex >= 0 && t.nextIndex < len(t.keys) && t.nextKey.Equal(key) {
		i := t.nextIndex
		if i+1 < len(t.keys) {
			nk := t.keys[i+1]
			t.nextKey = nk
			t.nextIndex = i + 1
			return nk, t.rawGetForNextLocked(nk), true
		}
		t.nextValid = false
		return NilValue(), NilValue(), false
	}
	for i, k := range t.keys {
		if k.Equal(key) {
			if i+1 < len(t.keys) {
				nk := t.keys[i+1]
				t.nextKey = nk
				t.nextIndex = i + 1
				t.nextValid = true
				return nk, t.rawGetForNextLocked(nk), true
			}
			t.nextValid = false
			return NilValue(), NilValue(), false
		}
	}
	t.nextValid = false
	return NilValue(), NilValue(), false
}
