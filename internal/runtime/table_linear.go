package runtime

// NewLinearModuloIntArray builds a dense 1-based integer array for loops of the
// form:
//
//	a[i] = (i*indexMul + salt*saltMul + n*lengthMul) % modulus
//
// It is used by guarded VM runtime specializations after bytecode and runtime
// type checks prove the loop has ordinary table semantics.
func NewLinearModuloIntArray(n, salt, indexMul, saltMul, lengthMul, modulus int64) *Table {
	if n < 0 {
		n = 0
	}
	size := int(n) + 1
	t := NewTableSizedKind(int(n), 0, ArrayInt)
	if cap(t.intArray) < size {
		t.intArray = DefaultHeap.GrowInt64s(t.intArray, size)
	}
	t.intArray = t.intArray[:size]
	clear(t.intArray)
	for i := int64(1); i <= n; i++ {
		t.intArray[i] = (i*indexMul + salt*saltMul + n*lengthMul) % modulus
	}
	t.arrayZeroValid = false
	t.keysDirty = true
	return t
}
