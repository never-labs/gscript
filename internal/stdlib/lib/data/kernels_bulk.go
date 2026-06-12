package data

import (
	"math"
	"sync"
)

// Bulk materialization kernels.
//
// Lazy carriers (i64ScalarDyadicArray, boolLogicalMask, notMask, compare
// masks, ...) evaluate one row per call through nested interface dispatch.
// That is fine for point lookups but disastrous for whole-vector consumers
// such as `where`, membership (`in`), find (`?`), `within`, and scalar
// compares over derived vectors: every row pays the full carrier-tree walk
// and, on generic fallbacks, []any boxing.
//
// The helpers below flatten a carrier tree into a dense typed slice with one
// tight loop per tree node. They return ok=false on any shape or row they
// cannot represent exactly (nulls, unsupported ops, mod-by-zero), so callers
// keep their existing fallback semantics.
//
// Ownership: each helper also reports whether the returned slice is owned
// (freshly produced) or aliases array storage. Owned slices are recycled
// through bulk pools by the consuming kernel via bulkI64Release and friends;
// this keeps 64KB-class temporaries out of the page allocator, whose
// span release/reuse churn (madvise) otherwise dominates hot q.eval loops.
// Pooled buffers are always fully overwritten before use.

const bulkPoolMaxLen = 1 << 21

var bulkI64Pool = sync.Pool{}
var bulkF64Pool = sync.Pool{}
var bulkBoolPool = sync.Pool{}
var bulkIntPool = sync.Pool{}
var bulkI32Pool = sync.Pool{}

func bulkI64Get(n int) []int64 {
	if v := bulkI64Pool.Get(); v != nil {
		s := v.([]int64)
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]int64, n)
}

func bulkI64Release(values []int64, owned bool) {
	if !owned || cap(values) == 0 || cap(values) > bulkPoolMaxLen {
		return
	}
	bulkI64Pool.Put(values[:0])
}

func bulkF64Get(n int) []float64 {
	if v := bulkF64Pool.Get(); v != nil {
		s := v.([]float64)
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]float64, n)
}

func bulkF64Release(values []float64, owned bool) {
	if !owned || cap(values) == 0 || cap(values) > bulkPoolMaxLen {
		return
	}
	bulkF64Pool.Put(values[:0])
}

func bulkBoolGet(n int) []bool {
	if v := bulkBoolPool.Get(); v != nil {
		s := v.([]bool)
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]bool, n)
}

func bulkBoolRelease(values []bool, owned bool) {
	if !owned || cap(values) == 0 || cap(values) > bulkPoolMaxLen {
		return
	}
	bulkBoolPool.Put(values[:0])
}

// bulkIntGet returns a zero-length []int with capacity at least n for
// transient row-index vectors (where filters, gather index lists). Contents
// beyond len are unspecified; callers must append or reslice-and-overwrite.
func bulkIntGet(n int) []int {
	if v := bulkIntPool.Get(); v != nil {
		s := v.([]int)
		if cap(s) >= n {
			return s[:0]
		}
	}
	return make([]int, 0, n)
}

// bulkIntGetLen returns a length-n []int for transient row-index vectors
// that are fully overwritten by the producer. Contents are unspecified.
func bulkIntGetLen(n int) []int {
	if v := bulkIntPool.Get(); v != nil {
		s := v.([]int)
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]int, n)
}

// bulkIntRelease recycles a transient row-index vector. The caller must
// guarantee nothing still references the slice's backing storage.
func bulkIntRelease(values []int) {
	if cap(values) == 0 || cap(values) > bulkPoolMaxLen {
		return
	}
	bulkIntPool.Put(values[:0])
}

// bulkI32Get returns a length-n []int32 for transient per-row group-id
// vectors. Contents are unspecified; callers must fully overwrite or clear.
func bulkI32Get(n int) []int32 {
	if v := bulkI32Pool.Get(); v != nil {
		s := v.([]int32)
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]int32, n)
}

// bulkI32Release recycles a transient group-id vector. The caller must
// guarantee nothing still references the slice's backing storage.
func bulkI32Release(values []int32) {
	if cap(values) == 0 || cap(values) > bulkPoolMaxLen {
		return
	}
	bulkI32Pool.Put(values[:0])
}

// TryBulkF64 exposes bulk float64 carrier flattening to sibling runtime
// packages (q): lazy carrier trees become dense slices with one tight loop
// per node. Callers must pass the returned slice and owned flag to
// BulkF64Release when done.
func TryBulkF64(array Array) (values []float64, owned bool, ok bool) {
	return tryBulkF64Values(array)
}

// BulkF64Release recycles a slice produced by TryBulkF64.
func BulkF64Release(values []float64, owned bool) {
	bulkF64Release(values, owned)
}

// TryBulkI64 exposes bulk int64 carrier flattening to sibling runtime
// packages. See TryBulkF64.
func TryBulkI64(array Array) (values []int64, owned bool, ok bool) {
	return tryBulkI64Values(array)
}

// BulkI64Release recycles a slice produced by TryBulkI64.
func BulkI64Release(values []int64, owned bool) {
	bulkI64Release(values, owned)
}

// tryBulkI64Values materializes any integer-producing array into []int64.
func tryBulkI64Values(array Array) (values []int64, owned bool, ok bool) {
	switch a := array.(type) {
	case attributedArray:
		return tryBulkI64Values(a.array)
	case columnArray[int64]:
		return a.data, false, true
	case columnArray[int32]:
		out := bulkI64Get(len(a.data))
		for i, v := range a.data {
			out[i] = int64(v)
		}
		return out, true, true
	case columnArray[int16]:
		out := bulkI64Get(len(a.data))
		for i, v := range a.data {
			out[i] = int64(v)
		}
		return out, true, true
	case columnArray[int8]:
		out := bulkI64Get(len(a.data))
		for i, v := range a.data {
			out[i] = int64(v)
		}
		return out, true, true
	case i64RangeArray:
		out := bulkI64Get(a.len)
		value := a.start
		for i := range out {
			out[i] = value
			value += a.step
		}
		return out, true, true
	case i64SegmentArray:
		out := bulkI64Get(a.len)
		row := 0
		for _, segment := range a.segments {
			value := segment.start
			for i := 0; i < segment.len; i++ {
				out[row] = value
				value += segment.step
				row++
			}
		}
		return out, true, true
	case i64ScalarDyadicArray:
		return tryBulkI64ScalarDyadicValues(a)
	case i64FillArray:
		// scalar ^ shifted/plain sources flatten in one pass: in-range rows
		// take the source value, out-of-range (shift-created null) rows take
		// the fill — exactly i64FillArray.valueAt per row. Sources with
		// stored nulls decline the inner flatten and keep the boxed route.
		if shifted, isShifted := a.source.(shiftedArray); isShifted {
			source, sourceOwned, ok := tryBulkI64Values(shifted.source)
			if !ok {
				return nil, false, false
			}
			n := shifted.Len()
			out := bulkI64Get(n)
			for i := range out {
				sourceRow := i + shifted.offset
				if sourceRow < 0 || sourceRow >= len(source) {
					out[i] = a.fill
				} else {
					out[i] = source[sourceRow]
				}
			}
			bulkI64Release(source, sourceOwned)
			return out, true, true
		}
		return tryBulkI64Values(a.source)
	case i64RunningSumArray:
		// Streaming scan producer: one dense prefix-sum pass instead of a
		// closed-form evaluation per row. Wrapping int64 addition matches the
		// dense numericSumsInteger path bit-for-bit.
		out := bulkI64Get(a.source.len)
		var acc int64
		value := a.source.start
		for i := range out {
			acc += value
			out[i] = acc
			value += a.source.step
		}
		return out, true, true
	case i64ScalarDyadicRunningSumArray:
		// Flatten the dyadic source once, then stream the running sum in a
		// single dense pass so downstream stages (xbar, gather, reduce)
		// consume a flat []int64 instead of re-walking the carrier per row.
		source, sourceOwned, ok := tryBulkI64Values(a.source)
		if !ok || len(source) < a.source.len {
			bulkI64Release(source, sourceOwned)
			return nil, false, false
		}
		source = source[:a.source.len]
		out := bulkI64Get(a.source.len)
		var acc int64
		for i, v := range source {
			acc += v
			out[i] = acc
		}
		bulkI64Release(source, sourceOwned)
		return out, true, true
	case i64PeriodicIndexArray:
		if a.period <= 0 || len(a.residues) == 0 {
			return nil, false, false
		}
		out := bulkI64Get(a.len)
		row := 0
		for cycle := int64(0); cycle < a.fullCycles && row < a.len; cycle++ {
			base := cycle * a.period
			for _, residue := range a.residues {
				if row >= a.len {
					break
				}
				out[row] = base + residue
				row++
			}
		}
		base := a.fullCycles * a.period
		for _, residue := range a.tailResidues {
			if row >= a.len {
				break
			}
			out[row] = base + residue
			row++
		}
		if row != a.len {
			bulkI64Release(out, true)
			return nil, false, false
		}
		return out, true, true
	case indexedArray:
		// Affine range sources gather in closed form: out[i] = start +
		// step*index[i], skipping the source materialization pass entirely.
		if rangeSource, isRange := a.source.(i64RangeArray); isRange {
			indexes, indexesOwned, ok := tryBulkI64Values(a.indexes)
			if ok && len(indexes) == a.len {
				out := bulkI64Get(a.len)
				for i, index := range indexes {
					if index < 0 || index >= int64(rangeSource.len) {
						bulkI64Release(indexes, indexesOwned)
						bulkI64Release(out, true)
						return nil, false, false
					}
					out[i] = rangeSource.start + rangeSource.step*index
				}
				bulkI64Release(indexes, indexesOwned)
				return out, true, true
			}
			bulkI64Release(indexes, indexesOwned)
		}
		source, sourceOwned, ok := tryBulkI64Values(a.source)
		if !ok {
			return tryBulkI64ValuesGeneric(array)
		}
		indexes, indexesOwned, ok := tryBulkI64Values(a.indexes)
		if !ok || len(indexes) != a.len {
			bulkI64Release(source, sourceOwned)
			bulkI64Release(indexes, indexesOwned)
			return tryBulkI64ValuesGeneric(array)
		}
		out := bulkI64Get(a.len)
		for i, index := range indexes {
			if index < 0 || index >= int64(len(source)) {
				bulkI64Release(source, sourceOwned)
				bulkI64Release(indexes, indexesOwned)
				bulkI64Release(out, true)
				return nil, false, false
			}
			out[i] = source[index]
		}
		bulkI64Release(source, sourceOwned)
		bulkI64Release(indexes, indexesOwned)
		return out, true, true
	case castI64Array:
		source, sourceOwned, ok := tryBulkF64Values(a.source)
		if !ok {
			return tryBulkI64ValuesGeneric(array)
		}
		out := bulkI64Get(len(source))
		for i, v := range source {
			out[i] = int64(v)
		}
		bulkF64Release(source, sourceOwned)
		return out, true, true
	case i64BucketArray:
		if a.width == 0 {
			return nil, false, false
		}
		source, sourceOwned, ok := tryBulkI64Values(a.source)
		if !ok || len(source) < a.len {
			bulkI64Release(source, sourceOwned)
			return tryBulkI64ValuesGeneric(array)
		}
		source = source[:a.len]
		out := bulkI64Get(a.len)
		for i, v := range source {
			out[i] = floorInt64(v, a.width)
		}
		bulkI64Release(source, sourceOwned)
		return out, true, true
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return nil, false, false
		}
		source, sourceOwned, ok := tryBulkI64Values(a.source)
		if !ok || len(source) != sourceLen {
			bulkI64Release(source, sourceOwned)
			return tryBulkI64ValuesGeneric(array)
		}
		out := bulkI64Get(a.len)
		for i := range out {
			out[i] = source[(a.start+i)%sourceLen]
		}
		bulkI64Release(source, sourceOwned)
		return out, true, true
	default:
		return tryBulkI64ValuesGeneric(array)
	}
}

type i64ScalarDyadicStep struct {
	op         Op
	scalar     int64
	scalarLeft bool
}

type i64DyadicStepStatus uint8

const (
	i64DyadicStepsOK i64DyadicStepStatus = iota
	i64DyadicStepsGeneric
	i64DyadicStepsFail
)

// collectI64ScalarDyadicSteps walks a scalar-dyadic chain outermost-first
// into steps, returning the non-dyadic base carrier. Generic means the
// caller should use the boxed fallback; Fail means the chain is invalid
// (mod/idiv by zero).
func collectI64ScalarDyadicSteps(a i64ScalarDyadicArray, steps *[4]i64ScalarDyadicStep) (Array, int, i64DyadicStepStatus) {
	stepCount := 0
	length := a.len
	base := Array(a)
	for stepCount < len(steps) {
		dyadic, ok := base.(i64ScalarDyadicArray)
		if !ok || dyadic.len < length {
			break
		}
		switch dyadic.op {
		case OpAdd, OpSub, OpMul, OpMod, OpIDiv:
		default:
			return nil, 0, i64DyadicStepsGeneric
		}
		if dyadic.op == OpMod && !dyadic.scalarLeft && dyadic.scalar == 0 {
			return nil, 0, i64DyadicStepsFail
		}
		if dyadic.op == OpIDiv && !dyadic.scalarLeft && dyadic.scalar == 0 {
			return nil, 0, i64DyadicStepsFail
		}
		steps[stepCount] = i64ScalarDyadicStep{op: dyadic.op, scalar: dyadic.scalar, scalarLeft: dyadic.scalarLeft}
		stepCount++
		base = dyadic.source
	}
	if _, stillDyadic := base.(i64ScalarDyadicArray); stillDyadic || stepCount == 0 {
		return nil, 0, i64DyadicStepsGeneric
	}
	return base, stepCount, i64DyadicStepsOK
}

// tryBulkI64ScalarDyadicValues flattens chains like ((x mod 97)+5) into a
// single output slice, applying every scalar-dyadic step inside one loop
// nest instead of materializing each intermediate carrier.
func tryBulkI64ScalarDyadicValues(a i64ScalarDyadicArray) ([]int64, bool, bool) {
	var steps [4]i64ScalarDyadicStep
	length := a.len
	base, stepCount, status := collectI64ScalarDyadicSteps(a, &steps)
	switch status {
	case i64DyadicStepsGeneric:
		return tryBulkI64ValuesGeneric(a)
	case i64DyadicStepsFail:
		return nil, false, false
	}
	// Range bases fuse the source walk with the innermost step in a single
	// pass: same int64 walk tryBulkI64Values would materialize, same step
	// expression applyI64ScalarDyadicStep applies, so results are identical
	// without the intermediate source buffer.
	if r, isRange := unwrapAttributedArray(base).(i64RangeArray); isRange && r.len >= length && stepCount >= 1 {
		out := bulkI64Get(length)
		if applyI64ScalarDyadicStepRange(steps[stepCount-1], r.start, r.step, out) {
			for i := stepCount - 2; i >= 0; i-- {
				if !applyI64ScalarDyadicStep(steps[i], out, out) {
					bulkI64Release(out, true)
					return nil, false, false
				}
			}
			return out, true, true
		}
		bulkI64Release(out, true)
		return nil, false, false
	}
	source, sourceOwned, ok := tryBulkI64Values(base)
	if !ok || len(source) < length {
		bulkI64Release(source, sourceOwned)
		return tryBulkI64ValuesGeneric(a)
	}
	source = source[:length]
	out := bulkI64Get(length)
	// Steps were collected outermost-first; apply innermost-first. After the
	// first step the data is in out, and later steps transform it in place.
	input := source
	for i := stepCount - 1; i >= 0; i-- {
		if !applyI64ScalarDyadicStep(steps[i], input, out) {
			bulkI64Release(source, sourceOwned)
			bulkI64Release(out, true)
			return nil, false, false
		}
		input = out
	}
	bulkI64Release(source, sourceOwned)
	return out, true, true
}

// qModPow2Mask reports the AND mask for a positive power-of-two modulus.
func qModPow2Mask(m int64) (int64, bool) {
	if m > 0 && m&(m-1) == 0 {
		return m - 1, true
	}
	return 0, false
}

// applyI64ScalarDyadicStepRange applies one step over a generated affine
// walk (start + i*step), mirroring applyI64ScalarDyadicStep over the slice
// tryBulkI64Values would produce for the range.
func applyI64ScalarDyadicStepRange(step i64ScalarDyadicStep, start, stride int64, out []int64) bool {
	v := start
	switch step.op {
	case OpAdd:
		for i := range out {
			out[i] = v + step.scalar
			v += stride
		}
	case OpSub:
		if step.scalarLeft {
			for i := range out {
				out[i] = step.scalar - v
				v += stride
			}
		} else {
			for i := range out {
				out[i] = v - step.scalar
				v += stride
			}
		}
	case OpMul:
		for i := range out {
			out[i] = v * step.scalar
			v += stride
		}
	case OpMod:
		if step.scalarLeft {
			for i := range out {
				if v == 0 {
					return false
				}
				out[i] = qModInt64(step.scalar, v)
				v += stride
			}
		} else {
			if step.scalar == 0 {
				return false
			}
			if mask, ok := qModPow2Mask(step.scalar); ok {
				for i := range out {
					if v >= 0 {
						out[i] = v & mask
					} else {
						out[i] = qModInt64(v, step.scalar)
					}
					v += stride
				}
				return true
			}
			if step.scalar > 1 && stride == 1 && start >= 0 {
				// Unit-stride non-negative walk over a non-power-of-two
				// modulus: the residue advances by one and wraps at the
				// modulus, replacing the per-element divide. Identical to
				// qModInt64(v, m) for v >= 0, m > 0.
				r := start % step.scalar
				for i := range out {
					out[i] = r
					r++
					if r == step.scalar {
						r = 0
					}
				}
				return true
			}
			for i := range out {
				out[i] = qModInt64(v, step.scalar)
				v += stride
			}
		}
	case OpIDiv:
		const minInt64 = -1 << 63
		if step.scalarLeft {
			for i := range out {
				if v == 0 || (step.scalar == minInt64 && v == -1) {
					return false
				}
				out[i] = floorDivInt64(step.scalar, v)
				v += stride
			}
		} else {
			if step.scalar == 0 || step.scalar == -1 {
				return false
			}
			for i := range out {
				out[i] = floorDivInt64(v, step.scalar)
				v += stride
			}
		}
	default:
		return false
	}
	return true
}

func applyI64ScalarDyadicStep(step i64ScalarDyadicStep, source, out []int64) bool {
	switch step.op {
	case OpAdd:
		for i, v := range source {
			out[i] = v + step.scalar
		}
	case OpSub:
		if step.scalarLeft {
			for i, v := range source {
				out[i] = step.scalar - v
			}
		} else {
			for i, v := range source {
				out[i] = v - step.scalar
			}
		}
	case OpMul:
		for i, v := range source {
			out[i] = v * step.scalar
		}
	case OpMod:
		if step.scalarLeft {
			for i, v := range source {
				if v == 0 {
					return false
				}
				out[i] = qModInt64(step.scalar, v)
			}
		} else {
			if step.scalar == 0 {
				return false
			}
			if mask, ok := qModPow2Mask(step.scalar); ok {
				// Floored modulo by a positive power of two is a bitwise AND
				// in two's complement for non-negative values; negative
				// values keep the exact qModInt64 route so flattened rows
				// stay bit-identical to per-row At evaluation.
				for i, v := range source {
					if v >= 0 {
						out[i] = v & mask
					} else {
						out[i] = qModInt64(v, step.scalar)
					}
				}
				return true
			}
			for i, v := range source {
				out[i] = qModInt64(v, step.scalar)
			}
		}
	case OpIDiv:
		const minInt64 = -1 << 63
		if step.scalarLeft {
			for i, v := range source {
				if v == 0 || (step.scalar == minInt64 && v == -1) {
					return false
				}
				out[i] = floorDivInt64(step.scalar, v)
			}
		} else {
			if step.scalar == 0 || step.scalar == -1 {
				return false
			}
			for i, v := range source {
				out[i] = floorDivInt64(v, step.scalar)
			}
		}
	default:
		return false
	}
	return true
}

func tryBulkI64ValuesGeneric(array Array) ([]int64, bool, bool) {
	switch array.Kind() {
	case KindI8, KindI16, KindI32, KindI64, KindU8, KindU16, KindU32, KindU64:
	default:
		return nil, false, false
	}
	out := bulkI64Get(array.Len())
	for row := range out {
		value, ok, err := integerArrayAt(array, row)
		if err != nil || !ok {
			bulkI64Release(out, true)
			return nil, false, false
		}
		out[row] = value
	}
	return out, true, true
}

// tryBulkF64Values materializes any numeric array into []float64. Null rows
// bail out so callers keep their null-aware fallback semantics.
func tryBulkF64Values(array Array) (values []float64, owned bool, ok bool) {
	switch a := array.(type) {
	case attributedArray:
		return tryBulkF64Values(a.array)
	case columnArray[float64]:
		return a.data, false, true
	case columnArray[float32]:
		out := bulkF64Get(len(a.data))
		for i, v := range a.data {
			out[i] = float64(v)
		}
		return out, true, true
	case castF32Array:
		source, sourceOwned, ok := tryBulkF64Values(a.source)
		if !ok {
			return nil, false, false
		}
		out := bulkF64Get(len(source))
		for i, v := range source {
			out[i] = float64(float32(v))
		}
		bulkF64Release(source, sourceOwned)
		return out, true, true
	case castI64Array:
		source, sourceOwned, ok := tryBulkF64Values(a.source)
		if !ok {
			return nil, false, false
		}
		out := bulkF64Get(len(source))
		for i, v := range source {
			out[i] = float64(int64(v))
		}
		bulkF64Release(source, sourceOwned)
		return out, true, true
	case f64RangeArray:
		out := bulkF64Get(a.len)
		for i := range out {
			out[i] = a.start + float64(i)*a.step
		}
		return out, true, true
	case i64RangeArray:
		// Direct one-pass conversion: float64 of the same int64 walk the
		// generic tryBulkI64Values flatten would produce, skipping the
		// intermediate []int64 buffer.
		out := bulkF64Get(a.len)
		value := a.start
		for i := range out {
			out[i] = float64(value)
			value += a.step
		}
		return out, true, true
	case f64NumericDyadicArray:
		if out, ok := tryBulkF64NumericDyadicValues(a); ok {
			return out, true, true
		}
	case f64BucketArray:
		if source, sourceOwned, ok := tryBulkF64Values(a.source); ok {
			if len(source) < a.len {
				bulkF64Release(source, sourceOwned)
				return nil, false, false
			}
			out := bulkF64Get(a.len)
			for i, v := range source[:a.len] {
				out[i] = math.Floor(v/a.width) * a.width
			}
			bulkF64Release(source, sourceOwned)
			return out, true, true
		}
		return nil, false, false
	}
	if values, valuesOwned, ok := tryBulkI64Values(array); ok {
		out := bulkF64Get(len(values))
		for i, v := range values {
			out[i] = float64(v)
		}
		bulkI64Release(values, valuesOwned)
		return out, true, true
	}
	if !isNumericArray(array) {
		return nil, false, false
	}
	producer, err := newF64NumericArrayProducer(array)
	if err != nil {
		return nil, false, false
	}
	return tryBulkF64ProducerValues(producer)
}

func tryBulkF64ProducerValues(producer f64NumericProducer) ([]float64, bool, bool) {
	switch p := producer.(type) {
	case f64F64ColumnProducer:
		return p.data, false, true
	case f64I64ColumnProducer:
		out := bulkF64Get(len(p.data))
		for i, v := range p.data {
			out[i] = float64(v)
		}
		return out, true, true
	case f64I64RangeProducer:
		out := bulkF64Get(p.values.len)
		value := p.values.start
		for i := range out {
			out[i] = float64(value)
			value += p.values.step
		}
		return out, true, true
	case f64F64RangeProducer:
		out := bulkF64Get(p.values.len)
		for i := range out {
			out[i] = p.values.start + float64(i)*p.values.step
		}
		return out, true, true
	case f64I64ScalarDyadicProducer:
		values, valuesOwned, ok := tryBulkI64Values(p.values)
		if !ok || len(values) < p.values.len {
			bulkI64Release(values, valuesOwned)
			return nil, false, false
		}
		values = values[:p.values.len]
		out := bulkF64Get(len(values))
		for i, v := range values {
			out[i] = float64(v)
		}
		bulkI64Release(values, valuesOwned)
		return out, true, true
	case f64ScalarProducer:
		out := bulkF64Get(p.len)
		for i := range out {
			out[i] = p.value
		}
		return out, true, true
	case f64BroadcastProducer:
		value, ok, err := p.source.f64At(0)
		if err != nil || !ok {
			return nil, false, false
		}
		out := bulkF64Get(p.len)
		for i := range out {
			out[i] = value
		}
		return out, true, true
	case f64CastF32Producer:
		source, sourceOwned, ok := tryBulkF64ProducerValues(p.source)
		if !ok {
			return nil, false, false
		}
		out := bulkF64Get(len(source))
		for i, v := range source {
			out[i] = float64(float32(v))
		}
		bulkF64Release(source, sourceOwned)
		return out, true, true
	case f64CastI64Producer:
		source, sourceOwned, ok := tryBulkF64ProducerValues(p.source)
		if !ok {
			return nil, false, false
		}
		out := bulkF64Get(len(source))
		for i, v := range source {
			out[i] = float64(int64(v))
		}
		bulkF64Release(source, sourceOwned)
		return out, true, true
	case f64DyadicProducer:
		// Scalar-operand fusion: a broadcast scalar side skips its dense
		// materialization pass; integer-backed left operands additionally
		// fuse the float64 conversion into the op pass. Element values are
		// the same expressions the staged passes compute, in the same order.
		if rp, scalarRight := p.right.(f64ScalarProducer); scalarRight {
			if out, handled, ok := bulkF64FusedScalarRightDyadic(p, rp.value); ok {
				return out, handled, handled
			}
		}
		if lp, scalarLeft := p.left.(f64ScalarProducer); scalarLeft {
			if out, handled, ok := bulkF64FusedScalarLeftDyadic(p, lp.value); ok {
				return out, handled, handled
			}
		}
		left, leftOwned, ok := tryBulkF64ProducerValues(p.left)
		if !ok || len(left) < p.len {
			bulkF64Release(left, leftOwned)
			return nil, false, false
		}
		right, rightOwned, ok := tryBulkF64ProducerValues(p.right)
		if !ok || len(right) < p.len {
			bulkF64Release(left, leftOwned)
			bulkF64Release(right, rightOwned)
			return nil, false, false
		}
		left = left[:p.len]
		right = right[:p.len]
		out := bulkF64Get(p.len)
		if !applyBulkF64Dyadic(p.op, p.apply, left, right, out) {
			bulkF64Release(left, leftOwned)
			bulkF64Release(right, rightOwned)
			bulkF64Release(out, true)
			return nil, false, false
		}
		bulkF64Release(left, leftOwned)
		bulkF64Release(right, rightOwned)
		return out, true, true
	default:
		out := bulkF64Get(producer.Len())
		for row := range out {
			value, ok, err := producer.f64At(row)
			if err != nil || !ok {
				bulkF64Release(out, true)
				return nil, false, false
			}
			out[row] = value
		}
		return out, true, true
	}
}

// bulkF64FusedScalarRightDyadic evaluates left <op> scalar without
// materializing the broadcast scalar side. ok=false sends the caller to the
// generic two-operand flatten; handled=false propagates a flatten failure.
// Integer-backed left producers fuse float64(iv) <op> scalar in one pass —
// identical per-element values to the staged convert-then-op passes.
// bulkF64ModFloat is q float modulo, identical to numericDyadicFloatFunc's
// OpMod case so fused loops stay bit-exact with the staged apply route.
func bulkF64ModFloat(left, right float64) float64 {
	if right == 0 {
		return math.NaN()
	}
	return left - right*math.Floor(left/right)
}

func bulkF64FusedScalarRightDyadic(p f64DyadicProducer, scalar float64) ([]float64, bool, bool) {
	switch p.op {
	case string(OpAdd), string(OpSub), string(OpMul), string(OpDiv), string(OpMod):
	default:
		return nil, false, false
	}
	if li, isIntDyadic := p.left.(f64I64ScalarDyadicProducer); isIntDyadic {
		ivs, ivsOwned, ok := tryBulkI64Values(li.values)
		if !ok || len(ivs) < p.len {
			bulkI64Release(ivs, ivsOwned)
			return nil, false, true
		}
		ivs = ivs[:p.len]
		out := bulkF64Get(p.len)
		switch p.op {
		case string(OpAdd):
			for i, iv := range ivs {
				out[i] = float64(iv) + scalar
			}
		case string(OpSub):
			for i, iv := range ivs {
				out[i] = float64(iv) - scalar
			}
		case string(OpMul):
			for i, iv := range ivs {
				out[i] = float64(iv) * scalar
			}
		case string(OpDiv):
			for i, iv := range ivs {
				out[i] = float64(iv) / scalar
			}
		case string(OpMod):
			for i, iv := range ivs {
				out[i] = bulkF64ModFloat(float64(iv), scalar)
			}
		}
		bulkI64Release(ivs, ivsOwned)
		return out, true, true
	}
	left, leftOwned, ok := tryBulkF64ProducerValues(p.left)
	if !ok || len(left) < p.len {
		bulkF64Release(left, leftOwned)
		return nil, false, true
	}
	left = left[:p.len]
	out := bulkF64Get(p.len)
	switch p.op {
	case string(OpAdd):
		for i, v := range left {
			out[i] = v + scalar
		}
	case string(OpSub):
		for i, v := range left {
			out[i] = v - scalar
		}
	case string(OpMul):
		for i, v := range left {
			out[i] = v * scalar
		}
	case string(OpDiv):
		for i, v := range left {
			out[i] = v / scalar
		}
	case string(OpMod):
		for i, v := range left {
			out[i] = bulkF64ModFloat(v, scalar)
		}
	}
	bulkF64Release(left, leftOwned)
	return out, true, true
}

// bulkF64FusedScalarLeftDyadic mirrors bulkF64FusedScalarRightDyadic for
// scalar <op> array trees: the broadcast scalar side skips its dense
// materialization pass and integer-backed right operands fuse the float64
// conversion into the op pass. Element values are scalar <op> right[i] —
// identical to the staged two-operand flatten.
func bulkF64FusedScalarLeftDyadic(p f64DyadicProducer, scalar float64) ([]float64, bool, bool) {
	switch p.op {
	case string(OpAdd), string(OpSub), string(OpMul), string(OpDiv), string(OpMod):
	default:
		return nil, false, false
	}
	if ri, isIntDyadic := p.right.(f64I64ScalarDyadicProducer); isIntDyadic {
		ivs, ivsOwned, ok := tryBulkI64Values(ri.values)
		if !ok || len(ivs) < p.len {
			bulkI64Release(ivs, ivsOwned)
			return nil, false, true
		}
		ivs = ivs[:p.len]
		out := bulkF64Get(p.len)
		switch p.op {
		case string(OpAdd):
			for i, iv := range ivs {
				out[i] = scalar + float64(iv)
			}
		case string(OpSub):
			for i, iv := range ivs {
				out[i] = scalar - float64(iv)
			}
		case string(OpMul):
			for i, iv := range ivs {
				out[i] = scalar * float64(iv)
			}
		case string(OpDiv):
			for i, iv := range ivs {
				out[i] = scalar / float64(iv)
			}
		case string(OpMod):
			for i, iv := range ivs {
				out[i] = bulkF64ModFloat(scalar, float64(iv))
			}
		}
		bulkI64Release(ivs, ivsOwned)
		return out, true, true
	}
	right, rightOwned, ok := tryBulkF64ProducerValues(p.right)
	if !ok || len(right) < p.len {
		bulkF64Release(right, rightOwned)
		return nil, false, true
	}
	right = right[:p.len]
	out := bulkF64Get(p.len)
	switch p.op {
	case string(OpAdd):
		for i, v := range right {
			out[i] = scalar + v
		}
	case string(OpSub):
		for i, v := range right {
			out[i] = scalar - v
		}
	case string(OpMul):
		for i, v := range right {
			out[i] = scalar * v
		}
	case string(OpDiv):
		for i, v := range right {
			out[i] = scalar / v
		}
	case string(OpMod):
		for i, v := range right {
			out[i] = bulkF64ModFloat(scalar, v)
		}
	}
	bulkF64Release(right, rightOwned)
	return out, true, true
}

func applyBulkF64Dyadic(op string, apply f64DyadicFunc, left, right, out []float64) bool {
	switch op {
	case string(OpAdd):
		for i := range out {
			out[i] = left[i] + right[i]
		}
	case string(OpSub):
		for i := range out {
			out[i] = left[i] - right[i]
		}
	case string(OpMul):
		for i := range out {
			out[i] = left[i] * right[i]
		}
	case string(OpDiv):
		for i := range out {
			out[i] = left[i] / right[i]
		}
	case string(OpMod):
		for i := range out {
			out[i] = bulkF64ModFloat(left[i], right[i])
		}
	default:
		if apply == nil {
			return false
		}
		for i := range out {
			out[i] = apply(left[i], right[i])
		}
	}
	return true
}

func tryBulkF64NumericDyadicValues(a f64NumericDyadicArray) ([]float64, bool) {
	apply, applyOK := numericDyadicFloatFunc(a.op)
	if !applyOK {
		return nil, false
	}
	// Integer-array (op) float-scalar trees fuse the int64 flatten, the
	// float64 conversion, and the scalar op into one pass. Element values
	// are float64(iv) <op> scalar — exactly what the staged flatten
	// (convert pass, then scalar pass) computes.
	if la, isArray := a.left.(Array); isArray && a.right != nil {
		if _, rightIsArray := a.right.(Array); !rightIsArray && !IsNull(a.right) {
			switch la.Kind() {
			case KindI64, KindI32, KindI16, KindI8:
				if rs, rok := numeric(a.right); rok {
					var fused bool
					switch a.op {
					case string(OpAdd), string(OpSub), string(OpMul), string(OpDiv):
						fused = true
					}
					if fused && la.Len() >= a.len {
						if ivs, ivsOwned, ok := tryBulkI64Values(la); ok {
							if len(ivs) >= a.len {
								ivs = ivs[:a.len]
								out := bulkF64Get(a.len)
								switch a.op {
								case string(OpAdd):
									for i, iv := range ivs {
										out[i] = float64(iv) + rs
									}
								case string(OpSub):
									for i, iv := range ivs {
										out[i] = float64(iv) - rs
									}
								case string(OpMul):
									for i, iv := range ivs {
										out[i] = float64(iv) * rs
									}
								case string(OpDiv):
									for i, iv := range ivs {
										out[i] = float64(iv) / rs
									}
								}
								bulkI64Release(ivs, ivsOwned)
								return out, true
							}
							bulkI64Release(ivs, ivsOwned)
						}
					}
				}
			}
		}
	}
	leftValues, leftOwned, leftScalar, leftIsScalar, ok := bulkF64Operand(a.left, a.len)
	if !ok {
		return nil, false
	}
	rightValues, rightOwned, rightScalar, rightIsScalar, ok := bulkF64Operand(a.right, a.len)
	if !ok {
		bulkF64Release(leftValues, leftOwned)
		return nil, false
	}
	out := bulkF64Get(a.len)
	handled := true
	switch {
	case !leftIsScalar && !rightIsScalar:
		handled = applyBulkF64Dyadic(a.op, apply, leftValues, rightValues, out)
	case !leftIsScalar:
		switch a.op {
		case string(OpAdd):
			for i, v := range leftValues {
				out[i] = v + rightScalar
			}
		case string(OpSub):
			for i, v := range leftValues {
				out[i] = v - rightScalar
			}
		case string(OpMul):
			for i, v := range leftValues {
				out[i] = v * rightScalar
			}
		case string(OpDiv):
			for i, v := range leftValues {
				out[i] = v / rightScalar
			}
		default:
			for i, v := range leftValues {
				out[i] = apply(v, rightScalar)
			}
		}
	case !rightIsScalar:
		switch a.op {
		case string(OpAdd):
			for i, v := range rightValues {
				out[i] = leftScalar + v
			}
		case string(OpMul):
			for i, v := range rightValues {
				out[i] = leftScalar * v
			}
		default:
			for i, v := range rightValues {
				out[i] = apply(leftScalar, v)
			}
		}
	default:
		value := apply(leftScalar, rightScalar)
		for i := range out {
			out[i] = value
		}
	}
	bulkF64Release(leftValues, leftOwned)
	bulkF64Release(rightValues, rightOwned)
	if !handled {
		bulkF64Release(out, true)
		return nil, false
	}
	return out, true
}

func bulkF64Operand(value any, length int) (values []float64, owned bool, scalar float64, isScalar bool, ok bool) {
	if array, isArray := value.(Array); isArray {
		if array.Len() == 1 && length != 1 {
			single, singleOwned, singleOK := tryBulkF64Values(array)
			if !singleOK || len(single) != 1 {
				bulkF64Release(single, singleOwned)
				return nil, false, 0, false, false
			}
			head := single[0]
			bulkF64Release(single, singleOwned)
			return nil, false, head, true, true
		}
		if array.Len() != length {
			return nil, false, 0, false, false
		}
		bulk, bulkOwned, bulkOK := tryBulkF64Values(array)
		if !bulkOK || len(bulk) != length {
			bulkF64Release(bulk, bulkOwned)
			return nil, false, 0, false, false
		}
		return bulk, bulkOwned, 0, false, true
	}
	if IsNull(value) {
		return nil, false, 0, false, false
	}
	n, numericOK := numeric(value)
	if !numericOK {
		return nil, false, 0, false, false
	}
	return nil, false, n, true, true
}

// tryBulkBoolValues materializes a boolean mask tree into []bool with one
// tight loop per mask node instead of per-row nested dispatch.
func tryBulkBoolValues(mask Array) (values []bool, owned bool, ok bool) {
	switch a := mask.(type) {
	case attributedArray:
		return tryBulkBoolValues(a.array)
	case columnArray[bool]:
		return a.data, false, true
	case notMask:
		if a.array.Kind() == KindBool {
			source, sourceOwned, ok := tryBulkBoolValues(a.array)
			if !ok {
				return nil, false, false
			}
			out := bulkBoolGet(len(source))
			for i, v := range source {
				out[i] = !v
			}
			bulkBoolRelease(source, sourceOwned)
			return out, true, true
		}
		if source, sourceOwned, ok := tryBulkI64Values(a.array); ok {
			out := bulkBoolGet(len(source))
			for i, v := range source {
				out[i] = v == 0
			}
			bulkI64Release(source, sourceOwned)
			return out, true, true
		}
		if source, sourceOwned, ok := tryBulkF64Values(a.array); ok {
			out := bulkBoolGet(len(source))
			for i, v := range source {
				out[i] = v == 0
			}
			bulkF64Release(source, sourceOwned)
			return out, true, true
		}
		return nil, false, false
	case boolLogicalMask:
		return tryBulkBoolLogicalValues(a)
	case i64MembershipMask:
		source, sourceOwned, ok := tryBulkI64Values(a.source)
		if !ok || len(source) < a.len {
			bulkI64Release(source, sourceOwned)
			return nil, false, false
		}
		source = source[:a.len]
		out := bulkBoolGet(len(source))
		for i, v := range source {
			out[i] = i64ProbesContain(a.probes, v)
		}
		bulkI64Release(source, sourceOwned)
		return out, true, true
	case i64WithinMask:
		source, sourceOwned, ok := tryBulkI64Values(a.source)
		if !ok || len(source) < a.len {
			bulkI64Release(source, sourceOwned)
			return nil, false, false
		}
		source = source[:a.len]
		out := bulkBoolGet(len(source))
		for i, v := range source {
			out[i] = a.valueOf(v)
		}
		bulkI64Release(source, sourceOwned)
		return out, true, true
	case f64CompareMask:
		source, sourceOwned, ok := tryBulkF64Values(a.source)
		if !ok || len(source) < a.source.len {
			bulkF64Release(source, sourceOwned)
			return nil, false, false
		}
		source = source[:a.source.len]
		out := bulkBoolGet(len(source))
		compareFloatSlice(source, a.scalar, true, a.op, out)
		bulkF64Release(source, sourceOwned)
		return out, true, true
	case i64ScalarDyadicCompareMask:
		source, sourceOwned, ok := tryBulkI64Values(a.values)
		if !ok || len(source) < a.values.len {
			bulkI64Release(source, sourceOwned)
			return nil, false, false
		}
		source = source[:a.values.len]
		out := bulkBoolGet(len(source))
		fillCompareI64Mask(source, effectiveRangeCompareOp(a.op, a.scalarLeft), a.scalar, out)
		bulkI64Release(source, sourceOwned)
		return out, true, true
	case i64RangeCompareMask:
		out := bulkBoolGet(a.values.len)
		for i := range out {
			out[i] = a.valueAt(i)
		}
		return out, true, true
	case i64SegmentCompareMask:
		out := bulkBoolGet(a.values.len)
		for i := range out {
			out[i] = a.valueAt(i)
		}
		return out, true, true
	case tiledArray:
		if mask.Kind() != KindBool {
			return nil, false, false
		}
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return nil, false, false
		}
		source, sourceOwned, ok := tryBulkBoolValues(a.source)
		if !ok || len(source) != sourceLen {
			bulkBoolRelease(source, sourceOwned)
			return nil, false, false
		}
		out := bulkBoolGet(a.len)
		for i := range out {
			out[i] = source[(a.start+i)%sourceLen]
		}
		bulkBoolRelease(source, sourceOwned)
		return out, true, true
	default:
		return nil, false, false
	}
}

func tryBulkBoolLogicalValues(mask boolLogicalMask) ([]bool, bool, bool) {
	if mask.op != "and" && mask.op != "or" {
		return nil, false, false
	}
	var left, right []bool
	var leftOwned, rightOwned bool
	if !mask.leftIsScalar {
		values, valuesOwned, ok := tryBulkBoolValues(mask.left)
		if !ok {
			return nil, false, false
		}
		left, leftOwned = values, valuesOwned
	}
	if !mask.rightIsScalar {
		values, valuesOwned, ok := tryBulkBoolValues(mask.right)
		if !ok {
			bulkBoolRelease(left, leftOwned)
			return nil, false, false
		}
		right, rightOwned = values, valuesOwned
	}
	out := bulkBoolGet(mask.len)
	handled := true
	switch {
	case left != nil && right != nil:
		switch {
		case len(left) == mask.len && len(right) == mask.len:
			if mask.op == "and" {
				for i := range out {
					out[i] = left[i] && right[i]
				}
			} else {
				for i := range out {
					out[i] = left[i] || right[i]
				}
			}
		case len(left) == 0 || len(right) == 0:
			handled = false
		default:
			for i := range out {
				out[i] = applyBoolLogical(mask.op, left[i%len(left)], right[i%len(right)])
			}
		}
	case left != nil:
		handled = bulkBoolScalarLogical(mask.op, left, mask.rightScalar, mask.len, out)
	case right != nil:
		handled = bulkBoolScalarLogical(mask.op, right, mask.leftScalar, mask.len, out)
	default:
		handled = false
	}
	bulkBoolRelease(left, leftOwned)
	bulkBoolRelease(right, rightOwned)
	if !handled {
		bulkBoolRelease(out, true)
		return nil, false, false
	}
	return out, true, true
}

func bulkBoolScalarLogical(op string, values []bool, scalar bool, length int, out []bool) bool {
	if len(values) == 0 {
		return false
	}
	if op == "and" && !scalar {
		for i := range out {
			out[i] = false
		}
		return true
	}
	if op == "or" && scalar {
		for i := range out {
			out[i] = true
		}
		return true
	}
	if len(values) == length {
		copy(out, values)
		return true
	}
	for i := range out {
		out[i] = values[i%len(values)]
	}
	return true
}

func fillCompareI64Mask(values []int64, op Op, scalar int64, out []bool) {
	switch op {
	case OpEQ:
		for i, v := range values {
			out[i] = v == scalar
		}
	case OpNE:
		for i, v := range values {
			out[i] = v != scalar
		}
	case OpLT:
		for i, v := range values {
			out[i] = v < scalar
		}
	case OpLE:
		for i, v := range values {
			out[i] = v <= scalar
		}
	case OpGT:
		for i, v := range values {
			out[i] = v > scalar
		}
	case OpGE:
		for i, v := range values {
			out[i] = v >= scalar
		}
	default:
		for i, v := range values {
			out[i] = boolCompare(op, v == scalar, compareInt64(v, scalar))
		}
	}
}

// bulkI64MembershipMask applies a small literal membership set against
// materialized int64 values. Small sets use a linear probe (faster than a map
// for the literal `in 25 33 41` shape); larger sets fall back to the map.
func bulkI64MembershipMask(values []int64, set map[int64]struct{}) []bool {
	out := make([]bool, len(values))
	if len(set) <= 8 {
		probes := make([]int64, 0, 8)
		for v := range set {
			probes = append(probes, v)
		}
		for i, v := range values {
			matched := false
			for _, probe := range probes {
				if v == probe {
					matched = true
					break
				}
			}
			out[i] = matched
		}
		return out
	}
	for i, v := range values {
		_, out[i] = set[v]
	}
	return out
}

// compareDyadicBulk lowers array/scalar and array/array comparisons over
// lazy numeric carriers to dense typed loops, avoiding per-row At boxing.
func compareDyadicBulk(op Op, left, right any, length int) (Array, bool) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	switch {
	case leftIsArray && !rightIsArray:
		return compareArrayScalarBulk(op, leftArray, right, length, false)
	case !leftIsArray && rightIsArray:
		return compareArrayScalarBulk(op, rightArray, left, length, true)
	case leftIsArray && rightIsArray:
		if leftArray.Len() != length || rightArray.Len() != length {
			return nil, false
		}
		if lv, lvOwned, ok := tryBulkI64Values(leftArray); ok {
			rv, rvOwned, ok := tryBulkI64Values(rightArray)
			if !ok {
				bulkI64Release(lv, lvOwned)
				return nil, false
			}
			out := make([]bool, length)
			for i := 0; i < length; i++ {
				out[i] = boolCompare(op, lv[i] == rv[i], compareInt64(lv[i], rv[i]))
			}
			bulkI64Release(lv, lvOwned)
			bulkI64Release(rv, rvOwned)
			return newBoolTrusted(out), true
		}
		if lv, lvOwned, ok := tryBulkF64Values(leftArray); ok {
			rv, rvOwned, ok := tryBulkF64Values(rightArray)
			if !ok {
				bulkF64Release(lv, lvOwned)
				return nil, false
			}
			out := make([]bool, length)
			for i := 0; i < length; i++ {
				out[i] = boolCompare(op, lv[i] == rv[i], compareFloat64(lv[i], rv[i]))
			}
			bulkF64Release(lv, lvOwned)
			bulkF64Release(rv, rvOwned)
			return newBoolTrusted(out), true
		}
		return nil, false
	default:
		return nil, false
	}
}

func compareArrayScalarBulk(op Op, array Array, scalar any, length int, scalarLeft bool) (Array, bool) {
	if array.Len() != length || IsNull(scalar) {
		return nil, false
	}
	op = effectiveRangeCompareOp(op, scalarLeft)
	if values, valuesOwned, ok := tryBulkI64Values(array); ok {
		target, exact := coerceInt64Exact(scalar)
		if !exact {
			bulkI64Release(values, valuesOwned)
		} else {
			out := make([]bool, length)
			fillCompareI64Mask(values, op, target, out)
			bulkI64Release(values, valuesOwned)
			return newBoolTrusted(out), true
		}
	}
	if values, valuesOwned, ok := tryBulkF64Values(array); ok {
		target, numericOK := numeric(scalar)
		if !numericOK {
			bulkF64Release(values, valuesOwned)
			return nil, false
		}
		out := make([]bool, length)
		handled := compareFloatSlice(values, target, true, op, out)
		bulkF64Release(values, valuesOwned)
		if handled {
			return newBoolTrusted(out), true
		}
	}
	return nil, false
}

// compareCountBulk counts comparison matches over lazy numeric carriers
// without materializing a mask or index vector.
func compareCountBulk(array Array, op Op, value any) (int64, bool) {
	if array == nil || IsNull(value) {
		return 0, false
	}
	if values, valuesOwned, ok := tryBulkI64Values(array); ok {
		target, exact := coerceInt64Exact(value)
		if !exact {
			bulkI64Release(values, valuesOwned)
		} else {
			var count int64
			switch op {
			case OpEQ:
				for _, v := range values {
					if v == target {
						count++
					}
				}
			case OpNE:
				for _, v := range values {
					if v != target {
						count++
					}
				}
			case OpLT:
				for _, v := range values {
					if v < target {
						count++
					}
				}
			case OpLE:
				for _, v := range values {
					if v <= target {
						count++
					}
				}
			case OpGT:
				for _, v := range values {
					if v > target {
						count++
					}
				}
			case OpGE:
				for _, v := range values {
					if v >= target {
						count++
					}
				}
			default:
				bulkI64Release(values, valuesOwned)
				return 0, false
			}
			bulkI64Release(values, valuesOwned)
			return count, true
		}
	}
	if values, valuesOwned, ok := tryBulkF64Values(array); ok {
		target, numericOK := numeric(value)
		if !numericOK {
			bulkF64Release(values, valuesOwned)
			return 0, false
		}
		var count int64
		switch op {
		case OpEQ:
			for _, v := range values {
				if v == target {
					count++
				}
			}
		case OpNE:
			for _, v := range values {
				if v != target {
					count++
				}
			}
		case OpLT:
			for _, v := range values {
				if v < target {
					count++
				}
			}
		case OpLE:
			for _, v := range values {
				if v <= target {
					count++
				}
			}
		case OpGT:
			for _, v := range values {
				if v > target {
					count++
				}
			}
		case OpGE:
			for _, v := range values {
				if v >= target {
					count++
				}
			}
		default:
			bulkF64Release(values, valuesOwned)
			return 0, false
		}
		bulkF64Release(values, valuesOwned)
		return count, true
	}
	return 0, false
}

// bulkNumericSumCountWhereMask reduces a numeric values array over a dense
// boolean mask produced by a bulk mask kernel (CompareMask, WithinMask, ...).
// It mirrors typedNumericSumCountWherePredicate semantics (int64 totals for
// dense integer arrays, float64 otherwise, NullValue on empty selection) while
// replacing per-row predicate dispatch with two tight loops.
//
// A nil predicate means the predicate array is the values array itself; the
// flattened values then feed the mask kernel directly so the carrier tree is
// walked once instead of twice.
func bulkNumericSumCountWhereMask(values, predicate Array, fillMask func(Array, []bool) bool) (any, int64, bool) {
	length := values.Len()
	var sum any
	var count int64
	handled := false
	if isDenseIntegerArray(values) {
		bulk, owned, ok := tryBulkI64Values(values)
		if !ok || len(bulk) < length {
			bulkI64Release(bulk, owned)
			return nil, 0, false
		}
		maskSource := predicate
		if maskSource == nil {
			maskSource = columnArray[int64]{kind: KindI64, data: bulk[:length]}
		}
		mask := bulkBoolGet(length)
		if fillMask(maskSource, mask) {
			var total int64
			for i, selected := range mask[:length] {
				if selected {
					count++
					total += bulk[i]
				}
			}
			sum, handled = total, true
		}
		bulkBoolRelease(mask, true)
		bulkI64Release(bulk, owned)
	} else if isNumericArray(values) {
		bulk, owned, ok := tryBulkF64Values(values)
		if !ok || len(bulk) < length {
			bulkF64Release(bulk, owned)
			return nil, 0, false
		}
		maskSource := predicate
		if maskSource == nil {
			maskSource = columnArray[float64]{kind: KindF64, data: bulk[:length]}
		}
		mask := bulkBoolGet(length)
		if fillMask(maskSource, mask) {
			var total float64
			for i, selected := range mask[:length] {
				if selected {
					count++
					total += bulk[i]
				}
			}
			sum, handled = total, true
		}
		bulkBoolRelease(mask, true)
		bulkF64Release(bulk, owned)
	}
	if !handled {
		return nil, 0, false
	}
	if count == 0 {
		return NullValue, 0, true
	}
	return sum, count, true
}

// numericIntegerDyadicBulk lowers array(+|-|*|mod)array integer dyadics over
// lazy carriers to dense typed loops. It bails (ok=false) on nulls, length
// mismatches, and zero mod divisors so callers keep null-aware fallbacks.
func numericIntegerDyadicBulk(op Op, left, right any, length int) (Array, bool) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray || !rightIsArray || leftArray.Len() != length || rightArray.Len() != length {
		return nil, false
	}
	leftValues, leftOwned, ok := tryBulkI64Values(leftArray)
	if !ok || len(leftValues) < length {
		bulkI64Release(leftValues, leftOwned)
		return nil, false
	}
	rightValues, rightOwned, ok := tryBulkI64Values(rightArray)
	if !ok || len(rightValues) < length {
		bulkI64Release(leftValues, leftOwned)
		bulkI64Release(rightValues, rightOwned)
		return nil, false
	}
	leftValues = leftValues[:length]
	rightValues = rightValues[:length]
	out := make([]int64, length)
	handled := true
	switch op {
	case OpAdd:
		for i, v := range leftValues {
			out[i] = v + rightValues[i]
		}
	case OpSub:
		for i, v := range leftValues {
			out[i] = v - rightValues[i]
		}
	case OpMul:
		for i, v := range leftValues {
			out[i] = v * rightValues[i]
		}
	case OpMod:
		for i, v := range leftValues {
			divisor := rightValues[i]
			if divisor == 0 {
				handled = false
				break
			}
			out[i] = qModInt64(v, divisor)
		}
	case OpIDiv:
		const minInt64 = -1 << 63
		for i, v := range leftValues {
			divisor := rightValues[i]
			if divisor == 0 || (v == minInt64 && divisor == -1) {
				handled = false
				break
			}
			out[i] = floorDivInt64(v, divisor)
		}
	default:
		handled = false
	}
	bulkI64Release(leftValues, leftOwned)
	bulkI64Release(rightValues, rightOwned)
	if !handled {
		return nil, false
	}
	return columnArray[int64]{kind: KindI64, data: out}, true
}

// TryTypedIntegerFloorDivide vectorizes q integer `div` (floor division) over
// dense/lazy integer carriers without boxing each row. Divide-by-zero and the
// MinInt64/-1 overflow row bail out so callers keep null-producing fallbacks.
func TryTypedIntegerFloorDivide(left, right any, length int) (Array, bool, error) {
	const minInt64 = -1 << 63
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	switch {
	case leftIsArray && !rightIsArray:
		if leftArray.Len() != length || !isDenseIntegerArray(leftArray) {
			return nil, false, nil
		}
		divisor, ok := integerScalarValue(right)
		if !ok {
			return nil, false, nil
		}
		return applyI64ArrayScalar(OpIDiv, leftArray, divisor, false)
	case !leftIsArray && rightIsArray:
		if rightArray.Len() != length || !isDenseIntegerArray(rightArray) {
			return nil, false, nil
		}
		dividend, ok := integerScalarValue(left)
		if !ok {
			return nil, false, nil
		}
		values, owned, ok := tryBulkI64Values(rightArray)
		if !ok || len(values) < length {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		out := make([]int64, length)
		for i, divisor := range values[:length] {
			if divisor == 0 || (dividend == minInt64 && divisor == -1) {
				bulkI64Release(values, owned)
				return nil, false, nil
			}
			out[i] = floorDivInt64(dividend, divisor)
		}
		bulkI64Release(values, owned)
		return columnArray[int64]{kind: KindI64, data: out}, true, nil
	case leftIsArray && rightIsArray:
		if !isDenseIntegerArray(leftArray) || !isDenseIntegerArray(rightArray) {
			return nil, false, nil
		}
		out, ok := numericIntegerDyadicBulk(OpIDiv, left, right, length)
		if !ok {
			return nil, false, nil
		}
		return out, true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedFindI64 vectorizes q find (`?`) for integer domain/query pairs:
// out[i] is the first domain index matching query[i], or count[domain] when
// the value is absent.
func TryTypedFindI64(domain, query Array) (Array, bool) {
	if domain == nil || query == nil {
		return nil, false
	}
	domainValues, domainOwned, ok := tryBulkI64Values(domain)
	if !ok {
		return nil, false
	}
	queryValues, queryOwned, ok := tryBulkI64Values(query)
	if !ok {
		bulkI64Release(domainValues, domainOwned)
		return nil, false
	}
	miss := int64(len(domainValues))
	out := make([]int64, len(queryValues))
	if table, minValue, ok := findI64DenseTable(domainValues); ok {
		span := int64(len(table))
		for i, v := range queryValues {
			index := miss
			if offset := v - minValue; offset >= 0 && offset < span {
				if hit := table[offset]; hit >= 0 {
					index = hit
				}
			}
			out[i] = index
		}
	} else if len(domainValues) <= 8 {
		for i, v := range queryValues {
			index := miss
			for j, candidate := range domainValues {
				if candidate == v {
					index = int64(j)
					break
				}
			}
			out[i] = index
		}
	} else {
		lookup := make(map[int64]int64, len(domainValues))
		for j := len(domainValues) - 1; j >= 0; j-- {
			lookup[domainValues[j]] = int64(j)
		}
		for i, v := range queryValues {
			index, found := lookup[v]
			if !found {
				index = miss
			}
			out[i] = index
		}
	}
	bulkI64Release(domainValues, domainOwned)
	bulkI64Release(queryValues, queryOwned)
	return newI64Trusted(out), true
}

// findI64DenseTable builds a value→first-index table for small-span integer
// find domains (first occurrence wins, mirroring the linear scan). -1 marks
// absent values. ok=false when the domain is empty, too large, or too
// sparse to justify a table.
func findI64DenseTable(domainValues []int64) ([]int64, int64, bool) {
	const maxDomain = 64
	const maxSpan = 1024
	if len(domainValues) == 0 || len(domainValues) > maxDomain {
		return nil, 0, false
	}
	minValue, maxValue := domainValues[0], domainValues[0]
	for _, v := range domainValues[1:] {
		if v < minValue {
			minValue = v
		}
		if v > maxValue {
			maxValue = v
		}
	}
	span := maxValue - minValue + 1
	if span <= 0 || span > maxSpan {
		return nil, 0, false
	}
	table := make([]int64, span)
	for i := range table {
		table[i] = -1
	}
	for j := len(domainValues) - 1; j >= 0; j-- {
		table[domainValues[j]-minValue] = int64(j)
	}
	return table, minValue, true
}

// TryTypedFindI64Sum reduces +/domain?query without materializing the index
// vector. Per-element indexes are exactly TryTypedFindI64's (first matching
// domain index, count[domain] on miss) and the sum accumulates wrapping
// int64 in row order, matching the staged find-then-sum route bit for bit.
// Empty queries decline so the staged route keeps its empty-sum semantics.
func TryTypedFindI64Sum(domain, query Array) (int64, bool) {
	if domain == nil || query == nil {
		return 0, false
	}
	domainValues, domainOwned, ok := tryBulkI64Values(domain)
	if !ok {
		return 0, false
	}
	queryValues, queryOwned, ok := tryBulkI64Values(query)
	if !ok || len(queryValues) == 0 {
		bulkI64Release(domainValues, domainOwned)
		bulkI64Release(queryValues, queryOwned)
		return 0, false
	}
	miss := int64(len(domainValues))
	var sum int64
	if table, minValue, ok := findI64DenseTable(domainValues); ok {
		span := int64(len(table))
		for _, v := range queryValues {
			index := miss
			if offset := v - minValue; offset >= 0 && offset < span {
				if hit := table[offset]; hit >= 0 {
					index = hit
				}
			}
			sum += index
		}
	} else if len(domainValues) <= 8 {
		for _, v := range queryValues {
			index := miss
			for j, candidate := range domainValues {
				if candidate == v {
					index = int64(j)
					break
				}
			}
			sum += index
		}
	} else {
		lookup := make(map[int64]int64, len(domainValues))
		for j := len(domainValues) - 1; j >= 0; j-- {
			lookup[domainValues[j]] = int64(j)
		}
		for _, v := range queryValues {
			index, found := lookup[v]
			if !found {
				index = miss
			}
			sum += index
		}
	}
	bulkI64Release(domainValues, domainOwned)
	bulkI64Release(queryValues, queryOwned)
	return sum, true
}

// TryTypedSetOpIndexes returns left-side row indexes for the q set verbs
// inter/intersect (op "inter") and except (op "except") without boxing rows
// or running O(n*m) DeepEqual membership probes. "inter" keeps the first
// occurrence of each left value present in the right-hand value set (q dedup
// semantics); "except" keeps every left row absent from it. Membership uses
// strict typed equality, matching DeepEqual semantics for int64, float64
// (NaN never equals NaN on either path), Symbol, and string rows.
func TryTypedSetOpIndexes(op string, array Array, values []any) ([]int, bool) {
	intersect := op == "inter"
	if !intersect && op != "except" {
		return nil, false
	}
	if attributed, ok := array.(attributedArray); ok {
		array = attributed.array
	}
	switch a := array.(type) {
	case columnArray[Symbol]:
		set, ok := exactMembership[Symbol](values)
		if !ok {
			return nil, false
		}
		return setOpIndexesComparable(a.data, set, intersect), true
	case columnArray[string]:
		set, ok := exactMembership[string](values)
		if !ok {
			return nil, false
		}
		return setOpIndexesComparable(a.data, set, intersect), true
	case columnArray[float64]:
		set, ok := exactMembership[float64](values)
		if !ok {
			return nil, false
		}
		return setOpIndexesComparable(a.data, set, intersect), true
	}
	if array.Kind() != KindI64 {
		return nil, false
	}
	rows, owned, ok := tryBulkI64Values(array)
	if !ok {
		return nil, false
	}
	set, ok := exactMembership[int64](values)
	if !ok {
		bulkI64Release(rows, owned)
		return nil, false
	}
	out := setOpIndexesComparable(rows, set, intersect)
	bulkI64Release(rows, owned)
	return out, true
}

func setOpIndexesComparable[T comparable](rows []T, set map[T]struct{}, intersect bool) []int {
	// Sentinel-sized sets: a linear probe over a flattened key slice beats a
	// hash probe per row.
	if len(set) <= 16 {
		keys := make([]T, 0, len(set))
		for key := range set {
			keys = append(keys, key)
		}
		return setOpIndexesSmall(rows, keys, intersect)
	}
	if intersect {
		out := make([]int, 0, len(set))
		var seen map[T]struct{}
		for i, value := range rows {
			if _, member := set[value]; !member {
				continue
			}
			if seen == nil {
				seen = make(map[T]struct{}, len(set))
			} else if _, dup := seen[value]; dup {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, i)
		}
		return out
	}
	out := make([]int, 0, len(rows))
	for i, value := range rows {
		if _, member := set[value]; !member {
			out = append(out, i)
		}
	}
	return out
}

func setOpIndexesSmall[T comparable](rows []T, keys []T, intersect bool) []int {
	member := func(value T) bool {
		for _, key := range keys {
			if key == value {
				return true
			}
		}
		return false
	}
	if intersect {
		out := make([]int, 0, len(keys))
		var seen []T
		for i, value := range rows {
			if !member(value) {
				continue
			}
			dup := false
			for _, existing := range seen {
				if existing == value {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			seen = append(seen, value)
			out = append(out, i)
			if len(out) == len(keys) {
				break
			}
		}
		return out
	}
	out := make([]int, 0, len(rows))
	for i, value := range rows {
		if !member(value) {
			out = append(out, i)
		}
	}
	return out
}

// TryTypedDistinctIndexes returns first-occurrence row indexes for typed
// arrays so distinct can gather without per-row boxing or O(n^2) DeepEqual
// scans. Strict typed equality matches the boxed fallback for non-null rows.
func TryTypedDistinctIndexes(array Array) ([]int, bool) {
	if attributed, ok := array.(attributedArray); ok {
		array = attributed.array
	}
	switch a := array.(type) {
	case columnArray[Symbol]:
		return distinctIndexesComparable(a.data), true
	case columnArray[string]:
		return distinctIndexesComparable(a.data), true
	case columnArray[float64]:
		return distinctIndexesComparable(a.data), true
	}
	if array.Kind() != KindI64 {
		return nil, false
	}
	rows, owned, ok := tryBulkI64Values(array)
	if !ok {
		return nil, false
	}
	out := distinctIndexesComparable(rows)
	bulkI64Release(rows, owned)
	return out, true
}

func distinctIndexesComparable[T comparable](rows []T) []int {
	out := make([]int, 0, min(len(rows), 16))
	seen := make(map[T]struct{}, min(len(rows), 64))
	for i, value := range rows {
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, i)
	}
	return out
}

// TryTypedConcat returns the q `,` (join) concatenation of two same-kind
// typed arrays as one dense typed column, without boxing the elements. Lazy
// carriers (ranges, tiles, cast views) are densified first; arrays whose
// materialized carriers are not the same columnArray type (or whose kinds
// differ) decline so the caller's boxed fallback keeps the semantics.
func TryTypedConcat(left, right Array) (Array, bool) {
	l := MaterializeArray(left)
	if attributed, ok := l.(attributedArray); ok {
		l = attributed.array
	}
	r := MaterializeArray(right)
	if attributed, ok := r.(attributedArray); ok {
		r = attributed.array
	}
	if l.Kind() != r.Kind() {
		return nil, false
	}
	return concatTypedArrays([]Array{l, r}, l.Len()+r.Len())
}

// TryTypedUnion returns the q union of two same-kind typed arrays: left
// values deduplicated in first-occurrence order followed by unseen right
// values, with one map instead of O((n+m)^2) boxed DeepEqual scans.
func TryTypedUnion(left, right Array) (Array, bool) {
	if attributed, ok := left.(attributedArray); ok {
		left = attributed.array
	}
	if attributed, ok := right.(attributedArray); ok {
		right = attributed.array
	}
	switch l := left.(type) {
	case columnArray[Symbol]:
		r, ok := right.(columnArray[Symbol])
		if !ok {
			return nil, false
		}
		return columnArray[Symbol]{kind: KindSymbol, data: unionComparable(l.data, r.data)}, true
	case columnArray[string]:
		r, ok := right.(columnArray[string])
		if !ok {
			return nil, false
		}
		return columnArray[string]{kind: KindString, data: unionComparable(l.data, r.data)}, true
	case columnArray[float64]:
		r, ok := right.(columnArray[float64])
		if !ok {
			return nil, false
		}
		return columnArray[float64]{kind: KindF64, data: unionComparable(l.data, r.data)}, true
	}
	if left.Kind() != KindI64 || right.Kind() != KindI64 {
		return nil, false
	}
	leftRows, leftOwned, ok := tryBulkI64Values(left)
	if !ok {
		return nil, false
	}
	rightRows, rightOwned, ok := tryBulkI64Values(right)
	if !ok {
		bulkI64Release(leftRows, leftOwned)
		return nil, false
	}
	out := unionComparable(leftRows, rightRows)
	bulkI64Release(leftRows, leftOwned)
	bulkI64Release(rightRows, rightOwned)
	return newI64Trusted(out), true
}

func unionComparable[T comparable](left, right []T) []T {
	out := make([]T, 0, len(left)+len(right))
	seen := make(map[T]struct{}, len(left)+len(right))
	for _, value := range left {
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range right {
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// MaterializeFrameColumn densifies a lazy carrier for frame adoption like
// MaterializeArray, but keeps cyclic tiled vectors of pointer-kind elements
// (symbols, strings) intact: the tiled carrier delegates Gather to its typed
// source and the group-id kernels have a dedicated tiled path, so expanding
// would only copy one string header per row and put that pointer-heavy dense
// copy on the GC scan path.
func MaterializeFrameColumn(array Array) Array {
	switch a := array.(type) {
	case attributedArray:
		return attributedArray{array: MaterializeFrameColumn(a.array), metadata: a.metadata, lazy: a.lazy}
	case tiledArray:
		switch a.source.Kind() {
		case KindSymbol, KindString:
			return array
		}
	case i64RangeArray, i64ScalarDyadicArray:
		// One-shot bulk-flattenable numeric carriers: downstream kernels pull
		// dense values through the pooled bulk extractor on demand, so frame
		// adoption does not need to pin a dense copy per column.
		return array
	}
	return MaterializeArray(array)
}

// MaterializeArray returns a dense representation of array. Dense carriers
// (typed columns, boxed nullable columns, encoded vectors) return unchanged
// with zero copies; lazy carriers (ranges, scalar-dyadic chains, tiled
// cycles) flatten through the bulk kernels so downstream row-loop consumers
// (sorts, group scans, frame columns) never pay a carrier-tree walk per row.
func MaterializeArray(array Array) Array {
	switch a := array.(type) {
	case attributedArray:
		return attributedArray{array: MaterializeArray(a.array), metadata: a.metadata, lazy: a.lazy}
	case columnArray[bool], columnArray[int8], columnArray[int16], columnArray[int32],
		columnArray[int64], columnArray[uint8], columnArray[uint16], columnArray[uint32],
		columnArray[uint64], columnArray[float32], columnArray[float64],
		columnArray[string], columnArray[Symbol], columnArray[Month], columnArray[Date],
		columnArray[DateTime], columnArray[Timespan], columnArray[Minute],
		columnArray[Second], columnArray[Time], columnArray[Timestamp],
		nullableArray, encodedArray,
		nullBitmapArray[int8], nullBitmapArray[int16], nullBitmapArray[int32],
		nullBitmapArray[int64], nullBitmapArray[float32], nullBitmapArray[float64]:
		return array
	case tiledArray:
		if out, ok := tileExpandTyped(a); ok {
			return out
		}
	}
	// External metadata-carrying wrappers (q attribute vectors) keep their
	// own Gather semantics so attributes survive materialization.
	if _, ok := array.(arrayMetadataProvider); ok {
		return array.Gather(allIndexes(array.Len()))
	}
	if values, owned, ok := tryBulkI64Values(array); ok {
		if !owned {
			values = append([]int64(nil), values...)
		}
		return newI64Trusted(values)
	}
	if array.Kind() == KindF64 {
		if values, owned, ok := tryBulkF64Values(array); ok {
			if !owned {
				values = append([]float64(nil), values...)
			}
			return newF64Trusted(values)
		}
	}
	return array.Gather(allIndexes(array.Len()))
}

// tileExpandTyped expands cyclic takes (n#`a`b`c) of typed sources into a
// dense column with one modulo loop, skipping the mapped index slice and the
// per-row dispatch a generic gather would pay.
func tileExpandTyped(a tiledArray) (Array, bool) {
	if a.len <= 0 || a.source.Len() == 0 {
		return nil, false
	}
	source := MaterializeArray(a.source)
	if attributed, ok := source.(attributedArray); ok {
		source = attributed.array
	}
	switch sc := source.(type) {
	case columnArray[Symbol]:
		return columnArray[Symbol]{kind: sc.kind, data: tileExpandSlice(sc.data, a.start, a.len)}, true
	case columnArray[string]:
		return columnArray[string]{kind: sc.kind, data: tileExpandSlice(sc.data, a.start, a.len)}, true
	case columnArray[bool]:
		return columnArray[bool]{kind: sc.kind, data: tileExpandSlice(sc.data, a.start, a.len)}, true
	case columnArray[float64]:
		return columnArray[float64]{kind: sc.kind, data: tileExpandSlice(sc.data, a.start, a.len)}, true
	case nullBitmapCarrier:
		return sc.tileExpand(a.start, a.len), true
	default:
		return nil, false
	}
}

func tileExpandSlice[T any](source []T, start, n int) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = source[(start+i)%len(source)]
	}
	return out
}
