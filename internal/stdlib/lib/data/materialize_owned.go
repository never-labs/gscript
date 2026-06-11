package data

// Pooled materialization of assigned lazy numeric carriers.
//
// q script statements that assign a lazy long/float carrier (scalar-dyadic
// trees, cast views, fill/tile views, ...) leave every downstream statement
// to re-flatten the same carrier through tryBulk*Values once per use. The q
// statement-slot pool materializes such carriers exactly once per assignment
// into a bulk-pool buffer the slot owns, so downstream statements consume a
// dense column with zero-copy flattening.
//
// This is buffer reuse, not result memoization: the carrier is recomputed and
// re-flattened on every assignment; only the backing storage is recycled.
//
// Ownership contract: the caller owns the returned buffer and must keep it
// alive for as long as any value produced from the returned column may be
// observed. The q statement-slot pool guarantees this by (a) only recycling a
// buffer two assignment generations after it was handed out, (b) recycling
// only while the same script source is re-executed end-to-end (every
// derived value is recomputed each generation, so nothing can still
// reference a two-generations-old buffer), and (c) orphaning buffers — never
// recycling them — on any anomaly (source switch, nested eval, error, or a
// non-scalar script result).

// ownedNumericColumnMinLen filters out tiny vectors where the flatten
// overhead outweighs any downstream zero-copy benefit.
const ownedNumericColumnMinLen = 64

// OwnedNumericBuffer is the recyclable backing storage handed out by
// MaterializeOwnedNumericColumn. Release returns it to the shared bulk pools;
// dropping the handle without Release safely orphans the storage to the GC.
type OwnedNumericBuffer struct {
	i64 []int64
	f64 []float64
}

// Release recycles the buffer through the shared bulk pools. The caller must
// guarantee no live value still references the storage (see the q
// statement-slot generation contract above).
func (b OwnedNumericBuffer) Release() {
	if b.i64 != nil {
		bulkI64Release(b.i64, true)
	}
	if b.f64 != nil {
		bulkF64Release(b.f64, true)
	}
}

// ownedNumericMaterializeCandidate selects the carrier shapes whose
// downstream consumption measurably pays for per-statement re-expansion.
// Scalar-dyadic trees, producers, and cast views are deliberately NOT
// included: their consumers (compare-count, predicate trees, gather-reduce)
// already stream the lazy tree in one fused pass, so materializing them only
// adds a write+read of the full vector. Fill carriers (`s^...` over fills /
// shifted sources) have no fused consumers and otherwise re-expand once per
// consuming statement.
func ownedNumericMaterializeCandidate(array Array) bool {
	switch array.(type) {
	case i64FillArray, f64FillArray:
		return true
	}
	// i64ScalarDyadicArray / f64NumericDyadicArray were measured (same-binary
	// runtime toggle, 2026-06) and REGRESS when materialized: their consumers
	// stream the lazy tree in one fused pass (bulkF64FusedScalarRightDyadic,
	// predicate trees), so densification only adds a full write+read pass.
	return false
}

// FillsScalarFillI64Into computes q `fill ^ fills array` for long null-bitmap
// carriers (plain or behind a cyclic take) in one fused pass written into
// buf's storage, skipping both the dense fills expansion and the scalar-fill
// copy. The returned column adopts buf's (possibly regrown) storage; the
// caller owns it under the same generation contract as
// MaterializeOwnedNumericColumn. ok=false declines unsupported shapes with no
// side effects, so callers keep their existing two-pass fallback semantics.
func FillsScalarFillI64Into(array Array, fill int64, buf OwnedNumericBuffer) (Array, OwnedNumericBuffer, bool) {
	if array == nil || array.Kind() != KindI64 {
		return nil, buf, false
	}
	var source nullBitmapArray[int64]
	start := 0
	length := array.Len()
	switch a := array.(type) {
	case nullBitmapArray[int64]:
		source = a
	case tiledArray:
		inner, ok := a.source.(nullBitmapArray[int64])
		if !ok {
			return nil, buf, false
		}
		source = inner
		start = a.start
	default:
		return nil, buf, false
	}
	period := len(source.data)
	if period == 0 || length == 0 {
		return nil, buf, false
	}
	dst := buf.i64
	if cap(dst) < length {
		// The previous buffer is two generations old (caller contract), so
		// recycling it before regrowing is safe.
		if dst != nil {
			bulkI64Release(dst, true)
		}
		dst = bulkI64Get(length)
	}
	dst = dst[:length]
	// One pass, identical to fills-then-scalar-fill: forward-fill non-null
	// values; rows before the first non-null take the scalar fill.
	last := fill
	for i := 0; i < length; i++ {
		row := (start + i) % period
		if !nullBitGet(source.nulls, row) {
			last = source.data[row]
		}
		dst[i] = last
	}
	buf.i64 = dst
	return columnArray[int64]{kind: KindI64, data: dst}, buf, true
}

// MaterializeOwnedNumericColumn flattens a lazy long/float carrier tree into
// a dense column backed by a bulk-pool buffer that the caller owns. ok is
// false when the carrier is already dense, aliases its own storage, contains
// nulls, is too small, or is a shape whose laziness downstream kernels rely
// on; callers keep the original value in those cases.
func MaterializeOwnedNumericColumn(array Array) (Array, OwnedNumericBuffer, bool) {
	if array == nil || array.Len() < ownedNumericColumnMinLen {
		return nil, OwnedNumericBuffer{}, false
	}
	if !ownedNumericMaterializeCandidate(array) {
		return nil, OwnedNumericBuffer{}, false
	}
	switch array.Kind() {
	case KindI64:
		values, owned, ok := tryBulkI64Values(array)
		if !ok || !owned {
			// !owned means the flatten aliases the array's own storage:
			// the value is effectively dense already and there is nothing
			// to take ownership of.
			return nil, OwnedNumericBuffer{}, false
		}
		return columnArray[int64]{kind: KindI64, data: values}, OwnedNumericBuffer{i64: values}, true
	case KindF64:
		values, owned, ok := tryBulkF64Values(array)
		if !ok || !owned {
			return nil, OwnedNumericBuffer{}, false
		}
		return columnArray[float64]{kind: KindF64, data: values}, OwnedNumericBuffer{f64: values}, true
	}
	return nil, OwnedNumericBuffer{}, false
}
