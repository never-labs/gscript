// table_specialize.go — guarded whole-function/record numeric specialization
// accessors that expose backing storage after shape/kind validation.
//
// Pure code movement from table.go: PlainFloatArrayForNumericSpecialization,
// PlainArrayValuesForRecordSpecialization, Load/StoreFloatRecord...,
// NumericSvalsForRecordSpecialization, MarkArrayMutationForNumericSpecialization.

package runtime

// PlainFloatArrayForNumericSpecialization exposes the backing float array for guarded
// whole-function numeric specializations. It is intentionally narrower than RawGetInt:
// callers may only use the slice when ordinary table indexing for keys
// 0..n-1 is known to hit a plain, non-concurrent, non-lazy float array without
// metamethod fallback.
func (t *Table) PlainFloatArrayForNumericSpecialization(n int) ([]float64, bool) {
	if n < 0 || t == nil || t.mu != nil || t.lazyTree != nil || t.metatable != nil || t.arrayKind != ArrayFloat {
		return nil, false
	}
	if len(t.floatArray) < n {
		return nil, false
	}
	if n > 0 && !t.arrayZeroValid {
		return nil, false
	}
	return t.floatArray, true
}

// PlainArrayValuesForRecordSpecialization exposes the mixed array prefix for guarded
// whole-call record specializations. It is intentionally narrow: callers may only use
// it when ordinary table indexing for keys 1..n is known to hit a plain mixed
// array without metamethod or concurrent-table fallback.
func (t *Table) PlainArrayValuesForRecordSpecialization(n int) ([]Value, bool) {
	if n < 0 || t == nil || t.mu != nil || t.lazyTree != nil || t.metatable != nil || t.arrayKind != ArrayMixed {
		return nil, false
	}
	if len(t.array) <= n {
		return nil, false
	}
	return t.array, true
}

// LoadFloatRecordForNumericSpecialization copies numeric string fields from a stable
// small-field record into out. Int fields are accepted with normal numeric
// widening; non-numeric fields make the guard fail.
func (t *Table) LoadFloatRecordForNumericSpecialization(shapeID uint32, idxs []int, out []float64) bool {
	if t == nil || t.mu != nil || t.lazyTree != nil || t.metatable != nil || t.shapeID == 0 || t.shapeID != shapeID {
		return false
	}
	if len(idxs) > len(out) {
		return false
	}
	for i, idx := range idxs {
		if idx < 0 || idx >= len(t.svals) {
			return false
		}
		v := t.svals[idx]
		switch v.Type() {
		case TypeFloat:
			out[i] = v.Float()
		case TypeInt:
			out[i] = float64(v.Int())
		default:
			return false
		}
	}
	return true
}

// StoreFloatRecordForNumericSpecialization writes float fields back to a stable
// small-field record. Shape and table-kind guards are repeated so a stale
// cached plan cannot write through a mutated table layout.
func (t *Table) StoreFloatRecordForNumericSpecialization(shapeID uint32, idxs []int, vals []float64) bool {
	if t == nil || t.mu != nil || t.lazyTree != nil || t.metatable != nil || t.shapeID == 0 || t.shapeID != shapeID {
		return false
	}
	if len(idxs) > len(vals) {
		return false
	}
	for i, idx := range idxs {
		if idx < 0 || idx >= len(t.svals) {
			return false
		}
		t.svals[idx] = FloatValue(vals[i])
	}
	t.keysDirty = true
	return true
}

// NumericSvalsForRecordSpecialization exposes the string-field value slice for guarded
// runtime-generated numeric record specializations after validating the table shape.
func (t *Table) NumericSvalsForRecordSpecialization(shapeID uint32) ([]Value, bool) {
	if t == nil || t.mu != nil || t.lazyTree != nil || t.metatable != nil || t.shapeID == 0 || t.shapeID != shapeID {
		return nil, false
	}
	return t.svals, true
}

// MarkArrayMutationForNumericSpecialization mirrors RawSetInt's observable iteration
// invalidation for guarded specializations that overwrite existing array slots.
func (t *Table) MarkArrayMutationForNumericSpecialization() {
	t.keysDirty = true
}
