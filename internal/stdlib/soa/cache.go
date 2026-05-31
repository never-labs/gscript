package soa

// ColumnCacheKey is the runtime-independent part of a cached single-column
// operation. Runtime code still owns object identity checks.
type ColumnCacheKey struct {
	Column string
	Array  DenseArrayMeta
}

func NewColumnCacheKey(column, dtype string, version uint64) ColumnCacheKey {
	return ColumnCacheKey{Column: column, Array: NewDenseArrayMeta(dtype, version)}
}

func (k ColumnCacheKey) Matches(column string, array DenseArrayMeta) bool {
	return k.Column == column && k.Array == array
}

func ResultMetaValid(result DenseArrayMeta, currentVersion uint64) bool {
	return result.Present && result.Version == currentVersion
}

func NextRingSlot(next, size int) (slot int, following int) {
	if size <= 0 {
		return 0, next
	}
	return next % size, next + 1
}
