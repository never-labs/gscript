package data

// Ownership-transfer plumbing between query projection and value export.
//
// An exec-shaped query materializes its result column three times on the way
// out: the projection gather, the export copy buffer, and the destination
// array storage. The gather output is freshly allocated and owned by the
// result frame, so when the caller promises to consume the frame exactly
// once (bind's exec-result export), the projection can mark those columns as
// transferable and the exporter adopts the gathered storage directly instead
// of re-copying.
//
// Contract:
//   - Only producers that materialize fresh, unaliased storage may mark a
//     column (projection ColumnRef gathers qualify by construction).
//   - Marking is requested per execution through the *Consume exec variants;
//     the caller guarantees the result frame is exported at most once and
//     dropped afterwards.
//   - Adoption consumes the storage: the adopter owns it and the frame must
//     not be used again. Non-adopting consumers see a normal Array.

// transferOwnedArray marks a freshly materialized column whose backing
// storage may be adopted by exactly one downstream exporter.
type transferOwnedArray struct {
	inner Array
}

func (a transferOwnedArray) Kind() Kind { return a.inner.Kind() }

func (a transferOwnedArray) Len() int { return a.inner.Len() }

func (a transferOwnedArray) At(row int) (any, bool) { return a.inner.At(row) }

func (a transferOwnedArray) Values() []any { return a.inner.Values() }

// Gather returns the unmarked gather of the inner storage: derived arrays are
// fresh again but their ownership belongs to the deriving operation.
func (a transferOwnedArray) Gather(indexes []int) Array { return a.inner.Gather(indexes) }

// markTransferOwned wraps a freshly materialized projection column. The
// caller asserts the storage is not aliased by any other live value.
func markTransferOwned(array Array) Array {
	if array == nil {
		return nil
	}
	return transferOwnedArray{inner: array}
}

// TryExportI64Adopt surrenders the dense backing storage of a
// transfer-owned i64 column. ok=false means the caller must export by copy.
func TryExportI64Adopt(array Array) ([]int64, bool) {
	t, ok := array.(transferOwnedArray)
	if !ok {
		return nil, false
	}
	if col, ok := unwrapAttributedArray(t.inner).(columnArray[int64]); ok && col.kind == KindI64 {
		return col.data, true
	}
	return nil, false
}

// TryExportF64Adopt surrenders the dense backing storage of a
// transfer-owned f64 column.
func TryExportF64Adopt(array Array) ([]float64, bool) {
	t, ok := array.(transferOwnedArray)
	if !ok {
		return nil, false
	}
	if col, ok := unwrapAttributedArray(t.inner).(columnArray[float64]); ok && col.kind == KindF64 {
		return col.data, true
	}
	return nil, false
}

// TryExportBoolAdopt surrenders the dense backing storage of a
// transfer-owned bool column.
func TryExportBoolAdopt(array Array) ([]bool, bool) {
	t, ok := array.(transferOwnedArray)
	if !ok {
		return nil, false
	}
	if col, ok := unwrapAttributedArray(t.inner).(columnArray[bool]); ok {
		return col.data, true
	}
	return nil, false
}

// unwrapTransferOwned strips the transfer marker for consumers that only
// need read access.
func unwrapTransferOwned(array Array) Array {
	if t, ok := array.(transferOwnedArray); ok {
		return t.inner
	}
	return array
}
