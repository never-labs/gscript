package data

import (
	"fmt"
	"math/bits"
)

// Null-bitmap column carrier.
//
// nullBitmapArray stores null-mixed numeric vectors as a dense typed value
// slice plus a null bitmap instead of per-element boxed []any storage
// (nullableArray). Null rows hold the zero value in data and have their bit
// set in nulls. The boundary contract matches nullableArray exactly:
// At(row) returns the kind-width typed scalar for value rows and NullValue
// for null rows, so unconverted consumers behave identically.
//
// q null semantics preserved by the kernels below:
//   - elementwise arithmetic propagates nulls (and integer mod/div by zero
//     produces null, matching the boxed fallback)
//   - comparisons follow canonical q null ordering (nullOrderedCompare):
//     null sorts before every value and equals itself, so 0N<x and 0N<=x
//     match for non-null x, x>0N and x>=0N match for non-null x, =0N
//     matches null rows, and <> matches null-vs-value; within treats null
//     as false (a<=0N never holds)
//   - sum/avg skip nulls
//   - fills forward-fills from the last non-null row; deltas propagates nulls
type nullBitmapElem interface {
	int8 | int16 | int32 | int64 | float32 | float64 |
		Month | Date | DateTime | Timespan | Minute | Second | Time | Timestamp
}

type nullBitmapArray[T nullBitmapElem] struct {
	kind Kind
	data []T
	// nulls is a little-endian bitmap: row r is null when bit r&63 of
	// nulls[r>>6] is set. len(nulls) == (len(data)+63)/64.
	nulls []uint64
}

func newNullBitmap(n int) []uint64 {
	return make([]uint64, (n+63)>>6)
}

func nullBitGet(nulls []uint64, row int) bool {
	return nulls[row>>6]>>(uint(row)&63)&1 != 0
}

func nullBitSet(nulls []uint64, row int) {
	nulls[row>>6] |= 1 << (uint(row) & 63)
}

// nullBitmapHasNull reports whether any of the first n bits is set.
func nullBitmapHasNull(nulls []uint64, n int) bool {
	if len(nulls) == 0 {
		return false
	}
	full := n >> 6
	for _, word := range nulls[:full] {
		if word != 0 {
			return true
		}
	}
	if rest := n & 63; rest != 0 && full < len(nulls) {
		return nulls[full]&(1<<uint(rest)-1) != 0
	}
	return false
}

// nullBitmapCount returns the number of null rows among the first n rows.
func nullBitmapCount(nulls []uint64, n int) int {
	if len(nulls) == 0 {
		return 0
	}
	count := 0
	full := n >> 6
	for _, word := range nulls[:full] {
		count += bits.OnesCount64(word)
	}
	if rest := n & 63; rest != 0 && full < len(nulls) {
		count += bits.OnesCount64(nulls[full] & (1<<uint(rest) - 1))
	}
	return count
}

// nullBitmapSlice copies bit range [start, start+count) into a fresh bitmap.
func nullBitmapSlice(nulls []uint64, start, count int) []uint64 {
	out := newNullBitmap(count)
	for i := 0; i < count; i++ {
		if nullBitGet(nulls, start+i) {
			nullBitSet(out, i)
		}
	}
	return out
}

func (a nullBitmapArray[T]) Kind() Kind { return a.kind }

func (a nullBitmapArray[T]) Len() int { return len(a.data) }

func (a nullBitmapArray[T]) At(row int) (any, bool) {
	if row < 0 || row >= len(a.data) {
		return nil, false
	}
	if nullBitGet(a.nulls, row) {
		return NullValue, true
	}
	return a.data[row], true
}

func (a nullBitmapArray[T]) Values() []any {
	out := make([]any, len(a.data))
	for i, v := range a.data {
		if nullBitGet(a.nulls, i) {
			out[i] = NullValue
			continue
		}
		out[i] = v
	}
	return out
}

func (a nullBitmapArray[T]) Gather(indexes []int) Array {
	data := make([]T, len(indexes))
	nulls := newNullBitmap(len(indexes))
	hasNull := false
	for i, row := range indexes {
		if row < 0 || row >= len(a.data) {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		if nullBitGet(a.nulls, row) {
			nullBitSet(nulls, i)
			hasNull = true
			continue
		}
		data[i] = a.data[row]
	}
	if !hasNull {
		return columnArray[T]{kind: a.kind, data: data}
	}
	return nullBitmapArray[T]{kind: a.kind, data: data, nulls: nulls}
}

// subarray returns rows [start, start+count) preserving the bitmap carrier.
func (a nullBitmapArray[T]) subarray(start, count int) Array {
	return nullBitmapArray[T]{
		kind:  a.kind,
		data:  a.data[start : start+count],
		nulls: nullBitmapSlice(a.nulls, start, count),
	}
}

// tileExpand materializes a cyclic take of the carrier into a dense carrier.
func (a nullBitmapArray[T]) tileExpand(start, length int) Array {
	period := len(a.data)
	data := make([]T, length)
	nulls := newNullBitmap(length)
	hasNull := false
	for i := range data {
		row := (start + i) % period
		if nullBitGet(a.nulls, row) {
			nullBitSet(nulls, i)
			hasNull = true
			continue
		}
		data[i] = a.data[row]
	}
	if !hasNull {
		return columnArray[T]{kind: a.kind, data: data}
	}
	return nullBitmapArray[T]{kind: a.kind, data: data, nulls: nulls}
}

// forwardFill applies q fills: every null row takes the last non-null value.
// Leading nulls (before the first value row) stay null, matching the boxed
// forward pass in vectorFills.
func (a nullBitmapArray[T]) forwardFill() Array {
	n := len(a.data)
	data := make([]T, n)
	var last T
	hasLast := false
	prefix := 0
	for i := 0; i < n; i++ {
		if !nullBitGet(a.nulls, i) {
			last = a.data[i]
			if !hasLast {
				hasLast = true
				prefix = i
			}
		}
		data[i] = last
	}
	if !hasLast {
		prefix = n
	}
	if prefix == 0 {
		return columnArray[T]{kind: a.kind, data: data}
	}
	nulls := newNullBitmap(n)
	for i := 0; i < prefix; i++ {
		nullBitSet(nulls, i)
	}
	return nullBitmapArray[T]{kind: a.kind, data: data, nulls: nulls}
}

// tileForwardFill fuses fills over a cyclic take of the carrier into one
// output buffer, skipping the intermediate dense expansion.
func (a nullBitmapArray[T]) tileForwardFill(start, length int) Array {
	period := len(a.data)
	data := make([]T, length)
	var last T
	hasLast := false
	prefix := 0
	for i := 0; i < length; i++ {
		row := (start + i) % period
		if !nullBitGet(a.nulls, row) {
			last = a.data[row]
			if !hasLast {
				hasLast = true
				prefix = i
			}
		}
		data[i] = last
	}
	if !hasLast {
		prefix = length
	}
	if prefix == 0 {
		return columnArray[T]{kind: a.kind, data: data}
	}
	nulls := newNullBitmap(length)
	for i := 0; i < prefix; i++ {
		nullBitSet(nulls, i)
	}
	return nullBitmapArray[T]{kind: a.kind, data: data, nulls: nulls}
}

// nullBitmapCarrier is the kind-erased view kernels use to reach a bitmap
// carrier without enumerating every element-type instantiation.
type nullBitmapCarrier interface {
	Array
	nullBits() []uint64
	subarray(start, count int) Array
	tileExpand(start, length int) Array
	forwardFill() Array
	tileForwardFill(start, length int) Array
	deltas() Array
	takePrefix(n int) Array
	isIntegerCarrier() bool
	// f64At returns the row value widened to float64; callers must skip null
	// rows via nullBits first.
	f64At(row int) float64
}

func (a nullBitmapArray[T]) nullBits() []uint64 { return a.nulls }

func (a nullBitmapArray[T]) f64At(row int) float64 { return float64(a.data[row]) }

// nullBitmapNumericSumRows mirrors the boxed NumericSumRows reduction over a
// bitmap carrier: selected null rows are skipped, out-of-range rows error.
func nullBitmapNumericSumRows(carrier nullBitmapCarrier, rows []int) (float64, int64, bool, error) {
	nulls := carrier.nullBits()
	length := carrier.Len()
	var sum float64
	var count int64
	for _, row := range rows {
		if row < 0 || row >= length {
			return 0, 0, true, fmt.Errorf("sum row %d out of range", row)
		}
		if nullBitGet(nulls, row) {
			continue
		}
		sum += carrier.f64At(row)
		count++
	}
	return sum, count, true, nil
}

// takePrefix clones the first n rows, mirroring the boxed nullableArray take.
func (a nullBitmapArray[T]) takePrefix(n int) Array {
	return nullBitmapArray[T]{
		kind:  a.kind,
		data:  append([]T(nil), a.data[:n]...),
		nulls: nullBitmapSlice(a.nulls, 0, n),
	}
}

func (a nullBitmapArray[T]) isIntegerCarrier() bool {
	switch a.kind {
	case KindF32, KindF64:
		return false
	default:
		return true
	}
}

// asNullBitmapCarrier unwraps attribute wrappers down to a bitmap carrier.
func asNullBitmapCarrier(array Array) (nullBitmapCarrier, bool) {
	for {
		switch a := array.(type) {
		case attributedArray:
			array = a.array
		case nullBitmapCarrier:
			return a, true
		default:
			return nil, false
		}
	}
}

// --- construction -----------------------------------------------------------

// nullBitmapKindSupported reports the numeric kinds stored as bitmap carriers
// instead of boxed nullableArray storage.
func nullBitmapKindSupported(kind Kind) bool {
	switch kind {
	case KindI8, KindI16, KindI32, KindI64, KindF32, KindF64:
		return true
	default:
		return false
	}
}

// nullBitmapArrayWithKind builds a bitmap carrier from boxed values,
// preserving nullableArrayWithKind normalization and error behavior.
func nullBitmapArrayWithKind(kind Kind, values []any) (Array, error) {
	switch kind {
	case KindI8:
		return nullBitmapFromBoxed[int8](kind, values)
	case KindI16:
		return nullBitmapFromBoxed[int16](kind, values)
	case KindI32:
		return nullBitmapFromBoxed[int32](kind, values)
	case KindI64:
		return nullBitmapFromBoxed[int64](kind, values)
	case KindF32:
		return nullBitmapFromBoxed[float32](kind, values)
	case KindF64:
		return nullBitmapFromBoxed[float64](kind, values)
	default:
		return nil, fmt.Errorf("null bitmap carrier does not support kind %s", kind)
	}
}

func nullBitmapFromBoxed[T nullBitmapElem](kind Kind, values []any) (Array, error) {
	data := make([]T, len(values))
	nulls := newNullBitmap(len(values))
	for i, v := range values {
		if IsNull(v) {
			nullBitSet(nulls, i)
			continue
		}
		normalized, err := normalizeScalarForKind(kind, v, i)
		if err != nil {
			return nil, err
		}
		typed, ok := normalized.(T)
		if !ok {
			return nil, fmt.Errorf("value %d must be %s-compatible for %s", i+1, kind, kind)
		}
		data[i] = typed
	}
	return nullBitmapArray[T]{kind: kind, data: data, nulls: nulls}, nil
}

// newNullBitmapI64 wraps a computed int64 vector + bitmap, densifying when no
// null bit is set so downstream kernels keep their typed fast paths.
func newNullBitmapI64(data []int64, nulls []uint64) Array {
	if !nullBitmapHasNull(nulls, len(data)) {
		return newI64Trusted(data)
	}
	return nullBitmapArray[int64]{kind: KindI64, data: data, nulls: nulls}
}

func newNullBitmapF64(data []float64, nulls []uint64) Array {
	if !nullBitmapHasNull(nulls, len(data)) {
		return newF64Trusted(data)
	}
	return nullBitmapArray[float64]{kind: KindF64, data: data, nulls: nulls}
}

// --- bulk flatteners --------------------------------------------------------

// tryBulkI64NullableValues flattens integer carriers that may contain nulls
// into dense values plus a null bitmap. A nil bitmap means no null rows. The
// returned bitmap may alias carrier storage and must be treated read-only;
// values follow the tryBulkI64Values ownership protocol.
func tryBulkI64NullableValues(array Array) (values []int64, nulls []uint64, owned bool, ok bool) {
	switch a := array.(type) {
	case attributedArray:
		return tryBulkI64NullableValues(a.array)
	case nullBitmapArray[int64]:
		return a.data, a.nulls, false, true
	case nullBitmapArray[int32]:
		out := bulkI64Get(len(a.data))
		for i, v := range a.data {
			out[i] = int64(v)
		}
		return out, a.nulls, true, true
	case nullBitmapArray[int16]:
		out := bulkI64Get(len(a.data))
		for i, v := range a.data {
			out[i] = int64(v)
		}
		return out, a.nulls, true, true
	case nullBitmapArray[int8]:
		out := bulkI64Get(len(a.data))
		for i, v := range a.data {
			out[i] = int64(v)
		}
		return out, a.nulls, true, true
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return nil, nil, false, false
		}
		source, sourceNulls, sourceOwned, ok := tryBulkI64NullableValues(a.source)
		if !ok || len(source) != sourceLen {
			bulkI64Release(source, sourceOwned)
			return nil, nil, false, false
		}
		if sourceNulls == nil {
			bulkI64Release(source, sourceOwned)
			values, owned, ok = tryBulkI64Values(array)
			return values, nil, owned, ok
		}
		out := bulkI64Get(a.len)
		outNulls := newNullBitmap(a.len)
		for i := range out {
			row := (a.start + i) % sourceLen
			out[i] = source[row]
			if nullBitGet(sourceNulls, row) {
				nullBitSet(outNulls, i)
			}
		}
		bulkI64Release(source, sourceOwned)
		return out, outNulls, true, true
	default:
		values, owned, ok = tryBulkI64Values(array)
		return values, nil, owned, ok
	}
}

// tryBulkF64NullableValues is the float64 analog of tryBulkI64NullableValues.
func tryBulkF64NullableValues(array Array) (values []float64, nulls []uint64, owned bool, ok bool) {
	switch a := array.(type) {
	case attributedArray:
		return tryBulkF64NullableValues(a.array)
	case nullBitmapArray[float64]:
		return a.data, a.nulls, false, true
	case nullBitmapArray[float32]:
		out := bulkF64Get(len(a.data))
		for i, v := range a.data {
			out[i] = float64(v)
		}
		return out, a.nulls, true, true
	case nullBitmapArray[int64], nullBitmapArray[int32], nullBitmapArray[int16], nullBitmapArray[int8]:
		ints, intNulls, intsOwned, ok := tryBulkI64NullableValues(array)
		if !ok {
			return nil, nil, false, false
		}
		out := bulkF64Get(len(ints))
		for i, v := range ints {
			out[i] = float64(v)
		}
		bulkI64Release(ints, intsOwned)
		return out, intNulls, true, true
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return nil, nil, false, false
		}
		source, sourceNulls, sourceOwned, ok := tryBulkF64NullableValues(a.source)
		if !ok || len(source) != sourceLen {
			bulkF64Release(source, sourceOwned)
			return nil, nil, false, false
		}
		if sourceNulls == nil {
			bulkF64Release(source, sourceOwned)
			values, owned, ok = tryBulkF64Values(array)
			return values, nil, owned, ok
		}
		out := bulkF64Get(a.len)
		outNulls := newNullBitmap(a.len)
		for i := range out {
			row := (a.start + i) % sourceLen
			out[i] = source[row]
			if nullBitGet(sourceNulls, row) {
				nullBitSet(outNulls, i)
			}
		}
		bulkF64Release(source, sourceOwned)
		return out, outNulls, true, true
	default:
		values, owned, ok = tryBulkF64Values(array)
		return values, nil, owned, ok
	}
}

// nullBitmapBackedArray reports whether the carrier tree bottoms out in a
// bitmap carrier, so null-aware bulk kernels only engage when the new
// representation is actually present.
func nullBitmapBackedArray(array Array) bool {
	for {
		switch a := array.(type) {
		case attributedArray:
			array = a.array
		case tiledArray:
			array = a.source
		case nullBitmapCarrier:
			return true
		default:
			return false
		}
	}
}

// --- dyadic arithmetic ------------------------------------------------------

type nullBitmapI64Operand struct {
	values   []int64
	nulls    []uint64
	owned    bool
	scalar   int64
	isScalar bool
}

func loadNullBitmapI64Operand(value any, length int) (nullBitmapI64Operand, bool) {
	if array, isArray := value.(Array); isArray {
		if array.Len() != length {
			return nullBitmapI64Operand{}, false
		}
		values, nulls, owned, ok := tryBulkI64NullableValues(array)
		if !ok || len(values) < length {
			bulkI64Release(values, owned)
			return nullBitmapI64Operand{}, false
		}
		return nullBitmapI64Operand{values: values[:length], nulls: nulls, owned: owned}, true
	}
	if IsNull(value) {
		return nullBitmapI64Operand{}, false
	}
	scalar, ok := integerScalarValue(value)
	if !ok {
		return nullBitmapI64Operand{}, false
	}
	return nullBitmapI64Operand{scalar: scalar, isScalar: true}, true
}

func (o nullBitmapI64Operand) at(i int) (int64, bool) {
	if o.isScalar {
		return o.scalar, false
	}
	if o.nulls != nil && nullBitGet(o.nulls, i) {
		return 0, true
	}
	return o.values[i], false
}

func (o nullBitmapI64Operand) release() {
	bulkI64Release(o.values, o.owned)
}

// numericIntegerDyadicNullBitmapBulk lowers integer dyadics over null-bitmap
// carriers to dense loops with bitmap null propagation. Div/mod by zero
// produces a null row, matching the boxed fallback. It only engages when an
// operand is bitmap-backed so dense paths stay untouched.
func numericIntegerDyadicNullBitmapBulk(op Op, left, right any, length int) (Array, bool) {
	switch op {
	case OpAdd, OpSub, OpMul, OpIDiv, OpMod:
	default:
		return nil, false
	}
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if (!leftIsArray || !nullBitmapBackedArray(leftArray)) &&
		(!rightIsArray || !nullBitmapBackedArray(rightArray)) {
		return nil, false
	}
	lo, ok := loadNullBitmapI64Operand(left, length)
	if !ok {
		return nil, false
	}
	ro, ok := loadNullBitmapI64Operand(right, length)
	if !ok {
		lo.release()
		return nil, false
	}
	out := make([]int64, length)
	nulls := mergedNullBitmap(length, lo.nulls, ro.nulls)
	// Null slots hold zero in carrier storage, so Add/Sub/Mul run branch-free
	// over every slot; the merged bitmap masks the null rows.
	switch op {
	case OpAdd:
		switch {
		case lo.isScalar:
			for i, v := range ro.values {
				out[i] = lo.scalar + v
			}
		case ro.isScalar:
			for i, v := range lo.values {
				out[i] = v + ro.scalar
			}
		default:
			for i, v := range lo.values {
				out[i] = v + ro.values[i]
			}
		}
	case OpSub:
		switch {
		case lo.isScalar:
			for i, v := range ro.values {
				out[i] = lo.scalar - v
			}
		case ro.isScalar:
			for i, v := range lo.values {
				out[i] = v - ro.scalar
			}
		default:
			for i, v := range lo.values {
				out[i] = v - ro.values[i]
			}
		}
	case OpMul:
		switch {
		case lo.isScalar:
			for i, v := range ro.values {
				out[i] = lo.scalar * v
			}
		case ro.isScalar:
			for i, v := range lo.values {
				out[i] = v * ro.scalar
			}
		default:
			for i, v := range lo.values {
				out[i] = v * ro.values[i]
			}
		}
	case OpIDiv:
		const minInt64 = -1 << 63
		for i := 0; i < length; i++ {
			if nullBitGet(nulls, i) {
				continue
			}
			lv, _ := lo.at(i)
			rv, _ := ro.at(i)
			if rv == 0 || (lv == minInt64 && rv == -1) {
				nullBitSet(nulls, i)
				continue
			}
			out[i] = floorDivInt64(lv, rv)
		}
	case OpMod:
		for i := 0; i < length; i++ {
			if nullBitGet(nulls, i) {
				continue
			}
			lv, _ := lo.at(i)
			rv, _ := ro.at(i)
			if rv == 0 {
				nullBitSet(nulls, i)
				continue
			}
			out[i] = qModInt64(lv, rv)
		}
	}
	lo.release()
	ro.release()
	return newNullBitmapI64(out, nulls), true
}

// mergedNullBitmap returns a fresh bitmap with the union of the operand null
// bits (nil operand bitmaps contribute nothing).
func mergedNullBitmap(length int, left, right []uint64) []uint64 {
	out := newNullBitmap(length)
	if left != nil {
		copy(out, left[:len(out)])
	}
	if right != nil {
		for i, word := range right[:len(out)] {
			out[i] |= word
		}
	}
	return out
}

type nullBitmapF64Operand struct {
	values   []float64
	nulls    []uint64
	owned    bool
	scalar   float64
	isScalar bool
}

func loadNullBitmapF64Operand(value any, length int) (nullBitmapF64Operand, bool) {
	if array, isArray := value.(Array); isArray {
		if array.Len() != length {
			return nullBitmapF64Operand{}, false
		}
		values, nulls, owned, ok := tryBulkF64NullableValues(array)
		if !ok || len(values) < length {
			bulkF64Release(values, owned)
			return nullBitmapF64Operand{}, false
		}
		return nullBitmapF64Operand{values: values[:length], nulls: nulls, owned: owned}, true
	}
	if IsNull(value) {
		return nullBitmapF64Operand{}, false
	}
	scalar, ok := numeric(value)
	if !ok {
		return nullBitmapF64Operand{}, false
	}
	return nullBitmapF64Operand{scalar: scalar, isScalar: true}, true
}

func (o nullBitmapF64Operand) at(i int) (float64, bool) {
	if o.isScalar {
		return o.scalar, false
	}
	if o.nulls != nil && nullBitGet(o.nulls, i) {
		return 0, true
	}
	return o.values[i], false
}

func (o nullBitmapF64Operand) release() {
	bulkF64Release(o.values, o.owned)
}

// numericDyadicNullBitmapBulk lowers float dyadics over null-bitmap carriers
// to dense loops with bitmap null propagation.
func numericDyadicNullBitmapBulk(op Op, left, right any, length int) (Array, bool) {
	switch op {
	case OpAdd, OpSub, OpMul, OpDiv:
	default:
		return nil, false
	}
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if (!leftIsArray || !nullBitmapBackedArray(leftArray)) &&
		(!rightIsArray || !nullBitmapBackedArray(rightArray)) {
		return nil, false
	}
	lo, ok := loadNullBitmapF64Operand(left, length)
	if !ok {
		return nil, false
	}
	ro, ok := loadNullBitmapF64Operand(right, length)
	if !ok {
		lo.release()
		return nil, false
	}
	out := make([]float64, length)
	nulls := mergedNullBitmap(length, lo.nulls, ro.nulls)
	// Null slots hold zero in carrier storage; results computed there are
	// masked by the merged bitmap, so every op runs branch-free.
	switch op {
	case OpAdd:
		switch {
		case lo.isScalar:
			for i, v := range ro.values {
				out[i] = lo.scalar + v
			}
		case ro.isScalar:
			for i, v := range lo.values {
				out[i] = v + ro.scalar
			}
		default:
			for i, v := range lo.values {
				out[i] = v + ro.values[i]
			}
		}
	case OpSub:
		switch {
		case lo.isScalar:
			for i, v := range ro.values {
				out[i] = lo.scalar - v
			}
		case ro.isScalar:
			for i, v := range lo.values {
				out[i] = v - ro.scalar
			}
		default:
			for i, v := range lo.values {
				out[i] = v - ro.values[i]
			}
		}
	case OpMul:
		switch {
		case lo.isScalar:
			for i, v := range ro.values {
				out[i] = lo.scalar * v
			}
		case ro.isScalar:
			for i, v := range lo.values {
				out[i] = v * ro.scalar
			}
		default:
			for i, v := range lo.values {
				out[i] = v * ro.values[i]
			}
		}
	case OpDiv:
		switch {
		case lo.isScalar:
			for i, v := range ro.values {
				out[i] = lo.scalar / v
			}
		case ro.isScalar:
			for i, v := range lo.values {
				out[i] = v / ro.scalar
			}
		default:
			for i, v := range lo.values {
				out[i] = v / ro.values[i]
			}
		}
	}
	lo.release()
	ro.release()
	return newNullBitmapF64(out, nulls), true
}

// --- reductions -------------------------------------------------------------

// nullBitmapSumInteger sums non-null rows of an integer bitmap carrier,
// matching the nullableArray reduction (int64 total, nulls skipped).
func nullBitmapSumInteger[T int8 | int16 | int32 | int64](a nullBitmapArray[T]) int64 {
	var total int64
	for i, v := range a.data {
		if nullBitGet(a.nulls, i) {
			continue
		}
		total += int64(v)
	}
	return total
}

// nullBitmapSumFloat replicates the boxed nullableArray float reduction: the
// total stays int64 while every non-null value is integer-exact and becomes
// float64 once any fractional value appears.
func nullBitmapSumFloat[T float32 | float64](a nullBitmapArray[T]) any {
	var sumI int64
	var sumF float64
	hasFloat := false
	for i, v := range a.data {
		if nullBitGet(a.nulls, i) {
			continue
		}
		f := float64(v)
		n := int64(f)
		if float64(n) == f {
			sumI += n
			sumF += f
			continue
		}
		hasFloat = true
		sumF += f
	}
	if hasFloat {
		return sumF
	}
	return sumI
}

// --- null mask / where ------------------------------------------------------

// nullBitmapNullMask fills out with the carrier's null bits.
func nullBitmapNullMask(carrier nullBitmapCarrier, out []bool) {
	nulls := carrier.nullBits()
	for i := range out[:carrier.Len()] {
		out[i] = nullBitGet(nulls, i)
	}
}

// nullBitmapNumericAt is the unboxed float row accessor: null rows report
// ok=false with no error, mirroring boxed null handling.
func nullBitmapNumericAt[T nullBitmapElem](a nullBitmapArray[T], row int) (float64, bool, error) {
	if row < 0 || row >= len(a.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	if nullBitGet(a.nulls, row) {
		return 0, false, nil
	}
	return float64(a.data[row]), true, nil
}

// nullBitmapIntegerAt is the unboxed row accessor integer kernels use:
// null rows report ok=false with no error, mirroring boxed null handling.
func nullBitmapIntegerAt[T int8 | int16 | int32 | int64](a nullBitmapArray[T], row int) (int64, bool, error) {
	if row < 0 || row >= len(a.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	if nullBitGet(a.nulls, row) {
		return 0, false, nil
	}
	return int64(a.data[row]), true, nil
}

// --- compare / within indexes -----------------------------------------------

// compareNullBitmapDyadicMask lowers array-vs-scalar and array-vs-array
// comparisons over bitmap-backed carriers to one dense mask loop with q null
// compare semantics: ordered compares with a null operand are false; = is
// false for null-vs-value and true for null-vs-null; <> is the negation.
func compareNullBitmapDyadicMask(op Op, left, right any, length int) (Array, bool) {
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return nil, false
	}
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if (!leftIsArray || !nullBitmapBackedArray(leftArray)) &&
		(!rightIsArray || !nullBitmapBackedArray(rightArray)) {
		return nil, false
	}
	leftNull := !leftIsArray && IsNull(left)
	rightNull := !rightIsArray && IsNull(right)
	if leftNull || rightNull {
		// Null scalar: canonical q null compare (null sorts first, equals
		// itself) row by row against the null bitmap.
		array := leftArray
		if array == nil {
			array = rightArray
		}
		if array == nil || array.Len() != length {
			return nil, false
		}
		carrier, ok := asNullBitmapCarrier(MaterializeArray(array))
		if !ok {
			return nil, false
		}
		nulls := carrier.nullBits()
		out := make([]bool, length)
		for i := range out {
			rowNull := nullBitGet(nulls, i)
			if leftNull {
				out[i] = nullOrderedCompare(op, true, rowNull)
			} else {
				out[i] = nullOrderedCompare(op, rowNull, true)
			}
		}
		return newBoolTrusted(out), true
	}
	// Integer-vs-integer compares stay exact in int64; everything else
	// flattens to float64, matching compareDyadicBulk precedence.
	leftIntOK := !leftIsArray || isIntegerArray(leftArray)
	rightIntOK := !rightIsArray || isIntegerArray(rightArray)
	leftIntScalar := leftIsArray || func() bool { _, ok := coerceInt64Exact(left); return ok && !isFloatScalar(left) }()
	rightIntScalar := rightIsArray || func() bool { _, ok := coerceInt64Exact(right); return ok && !isFloatScalar(right) }()
	if leftIntOK && rightIntOK && leftIntScalar && rightIntScalar {
		lo, ok := loadNullBitmapI64Operand(left, length)
		if !ok {
			return nil, false
		}
		ro, ok := loadNullBitmapI64Operand(right, length)
		if !ok {
			lo.release()
			return nil, false
		}
		out := make([]bool, length)
		for i := 0; i < length; i++ {
			lv, lNull := lo.at(i)
			rv, rNull := ro.at(i)
			if lNull || rNull {
				out[i] = nullOrderedCompare(op, lNull, rNull)
				continue
			}
			out[i] = boolCompare(op, lv == rv, compareInt64(lv, rv))
		}
		lo.release()
		ro.release()
		return newBoolTrusted(out), true
	}
	lo, ok := loadNullBitmapF64Operand(left, length)
	if !ok {
		return nil, false
	}
	ro, ok := loadNullBitmapF64Operand(right, length)
	if !ok {
		lo.release()
		return nil, false
	}
	out := make([]bool, length)
	for i := 0; i < length; i++ {
		lv, lNull := lo.at(i)
		rv, rNull := ro.at(i)
		if lNull || rNull {
			out[i] = nullOrderedCompare(op, lNull, rNull)
			continue
		}
		out[i] = boolCompare(op, lv == rv, compareFloat64(lv, rv))
	}
	lo.release()
	ro.release()
	return newBoolTrusted(out), true
}

func isFloatScalar(value any) bool {
	switch value.(type) {
	case float32, float64:
		return true
	default:
		return false
	}
}

// nullBitmapCompareIndexesI64 returns selected row indexes for carrier-vs-
// scalar comparisons over bitmap-backed integer/float trees, with canonical
// q null ordering: null sorts before every value and equals itself
// (nullOrderedCompare), so null rows match <, <= and =0N, and value rows
// match >0N, >=0N and <>0N.
func nullBitmapCompareIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	if !nullBitmapBackedArray(array) {
		return nil, false, nil
	}
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return nil, false, nil
	}
	length := array.Len()
	if IsNull(value) {
		// Canonical q null compare against a null scalar: null rows match
		// =0N, <=0N, >=0N; value rows match <>0N, >0N, >=0N.
		values, nulls, owned, ok := tryBulkI64NullableValues(array)
		release := func() { bulkI64Release(values, owned) }
		if !ok {
			var fvalues []float64
			var fowned bool
			fvalues, nulls, fowned, ok = tryBulkF64NullableValues(array)
			if !ok {
				return nil, false, nil
			}
			release = func() { bulkF64Release(fvalues, fowned) }
			length = len(fvalues)
		} else {
			length = len(values)
		}
		out := make([]int64, 0, length)
		for i := 0; i < length; i++ {
			isNull := nulls != nil && nullBitGet(nulls, i)
			if nullOrderedCompare(op, isNull, true) {
				out = append(out, int64(i))
			}
		}
		release()
		return newI64Trusted(out), true, nil
	}
	if values, nulls, owned, ok := tryBulkI64NullableValues(array); ok {
		target, exact := coerceInt64Exact(value)
		if !exact {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		scratch := bulkI64Get(length)
		n := 0
		for i, v := range values[:length] {
			if nulls != nil && nullBitGet(nulls, i) {
				if nullOrderedCompare(op, true, false) {
					scratch[n] = int64(i)
					n++
				}
				continue
			}
			if boolCompare(op, v == target, compareInt64(v, target)) {
				scratch[n] = int64(i)
				n++
			}
		}
		bulkI64Release(values, owned)
		out := make([]int64, n)
		copy(out, scratch[:n])
		bulkI64Release(scratch, true)
		return newI64Trusted(out), true, nil
	}
	if values, nulls, owned, ok := tryBulkF64NullableValues(array); ok {
		target, numericOK := numeric(value)
		if !numericOK {
			bulkF64Release(values, owned)
			return nil, false, nil
		}
		scratch := bulkI64Get(length)
		n := 0
		for i, v := range values[:length] {
			if nulls != nil && nullBitGet(nulls, i) {
				if nullOrderedCompare(op, true, false) {
					scratch[n] = int64(i)
					n++
				}
				continue
			}
			if boolCompare(op, v == target, compareFloat64(v, target)) {
				scratch[n] = int64(i)
				n++
			}
		}
		bulkF64Release(values, owned)
		out := make([]int64, n)
		copy(out, scratch[:n])
		bulkI64Release(scratch, true)
		return newI64Trusted(out), true, nil
	}
	return nil, false, nil
}

// nullBitmapCompareIndexStatsI64 mirrors nullBitmapCompareIndexesI64 but only
// accumulates count and index sum.
func nullBitmapCompareIndexStatsI64(array Array, op Op, value any) (count, sum int64, handled bool, err error) {
	if !nullBitmapBackedArray(array) {
		return 0, 0, false, nil
	}
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return 0, 0, false, nil
	}
	length := array.Len()
	if IsNull(value) {
		values, nulls, owned, ok := tryBulkI64NullableValues(array)
		release := func() { bulkI64Release(values, owned) }
		if !ok {
			var fvalues []float64
			var fowned bool
			fvalues, nulls, fowned, ok = tryBulkF64NullableValues(array)
			if !ok {
				return 0, 0, false, nil
			}
			release = func() { bulkF64Release(fvalues, fowned) }
			length = len(fvalues)
		} else {
			length = len(values)
		}
		for i := 0; i < length; i++ {
			isNull := nulls != nil && nullBitGet(nulls, i)
			if nullOrderedCompare(op, isNull, true) {
				count++
				sum += int64(i)
			}
		}
		release()
		return count, sum, true, nil
	}
	if values, nulls, owned, ok := tryBulkI64NullableValues(array); ok {
		target, exact := coerceInt64Exact(value)
		if !exact {
			bulkI64Release(values, owned)
			return 0, 0, false, nil
		}
		for i, v := range values[:length] {
			if nulls != nil && nullBitGet(nulls, i) {
				if nullOrderedCompare(op, true, false) {
					count++
					sum += int64(i)
				}
				continue
			}
			if boolCompare(op, v == target, compareInt64(v, target)) {
				count++
				sum += int64(i)
			}
		}
		bulkI64Release(values, owned)
		return count, sum, true, nil
	}
	if values, nulls, owned, ok := tryBulkF64NullableValues(array); ok {
		target, numericOK := numeric(value)
		if !numericOK {
			bulkF64Release(values, owned)
			return 0, 0, false, nil
		}
		for i, v := range values[:length] {
			if nulls != nil && nullBitGet(nulls, i) {
				if nullOrderedCompare(op, true, false) {
					count++
					sum += int64(i)
				}
				continue
			}
			if boolCompare(op, v == target, compareFloat64(v, target)) {
				count++
				sum += int64(i)
			}
		}
		bulkF64Release(values, owned)
		return count, sum, true, nil
	}
	return 0, 0, false, nil
}

// nullBitmapWithinIndexesI64 returns rows inside [low, high] (high open when
// !highClosed); null rows never match.
func nullBitmapWithinIndexesI64(array Array, low, high any, highClosed bool) (Array, bool, error) {
	if !nullBitmapBackedArray(array) {
		return nil, false, nil
	}
	if values, nulls, owned, ok := tryBulkI64NullableValues(array); ok {
		lowI, lowOK := coerceInt64Exact(low)
		highI, highOK := coerceInt64Exact(high)
		if !lowOK || !highOK {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		out := make([]int64, 0, len(values))
		for i, v := range values {
			if nulls != nil && nullBitGet(nulls, i) {
				continue
			}
			if v < lowI || v > highI || (!highClosed && v == highI) {
				continue
			}
			out = append(out, int64(i))
		}
		bulkI64Release(values, owned)
		return newI64Trusted(out), true, nil
	}
	if values, nulls, owned, ok := tryBulkF64NullableValues(array); ok {
		lowF, lowOK := numeric(low)
		highF, highOK := numeric(high)
		if !lowOK || !highOK {
			bulkF64Release(values, owned)
			return nil, false, nil
		}
		out := make([]int64, 0, len(values))
		for i, v := range values {
			if nulls != nil && nullBitGet(nulls, i) {
				continue
			}
			if v < lowF || v > highF || (!highClosed && v == highF) {
				continue
			}
			out = append(out, int64(i))
		}
		bulkF64Release(values, owned)
		return newI64Trusted(out), true, nil
	}
	return nil, false, nil
}

// numericUnaryNullBitmap mirrors numericUnaryNullable over bitmap carriers:
// nulls pass through, non-null rows apply the float unary, and the result is
// a KindF64 carrier.
func numericUnaryNullBitmap(op string, array Array) (Array, bool, error) {
	values, nulls, owned, ok := tryBulkF64NullableValues(array)
	if !ok {
		return nil, false, nil
	}
	out := make([]float64, len(values))
	outNulls := newNullBitmap(len(values))
	for i, v := range values {
		if nulls != nil && nullBitGet(nulls, i) {
			nullBitSet(outNulls, i)
			continue
		}
		result, err := applyNumericUnaryFloat(op, v)
		if err != nil {
			bulkF64Release(values, owned)
			return nil, true, err
		}
		out[i] = result
	}
	bulkF64Release(values, owned)
	return newNullBitmapF64(out, outNulls), true, nil
}

// --- deltas -----------------------------------------------------------------

// deltas applies q deltas over the carrier: out[0] = x[0];
// out[i] = x[i]-x[i-1] with nulls propagating. Element width is preserved
// (kind-width wrapping subtraction) and float results report KindF64,
// matching deltasNullableArray over the boxed carrier exactly.
func (a nullBitmapArray[T]) deltas() Array {
	n := len(a.data)
	data := make([]T, n)
	nulls := newNullBitmap(n)
	for i := 0; i < n; i++ {
		if nullBitGet(a.nulls, i) || (i > 0 && nullBitGet(a.nulls, i-1)) {
			nullBitSet(nulls, i)
			continue
		}
		if i == 0 {
			data[0] = a.data[0]
			continue
		}
		data[i] = a.data[i] - a.data[i-1]
	}
	kind := a.kind
	if !a.isIntegerCarrier() {
		kind = KindF64
	}
	return nullBitmapArray[T]{kind: kind, data: data, nulls: nulls}
}

// typedNullBitmapFills applies q fills over bitmap carriers (and cyclic takes
// of them) in one dense pass, keeping the source element kind.
func typedNullBitmapFills(array Array) (Array, bool) {
	switch a := array.(type) {
	case attributedArray:
		return typedNullBitmapFills(a.array)
	case tiledArray:
		carrier, ok := asNullBitmapCarrier(a.source)
		if !ok || a.source.Len() == 0 {
			return nil, false
		}
		return carrier.tileForwardFill(a.start, a.len), true
	default:
		carrier, ok := asNullBitmapCarrier(array)
		if !ok {
			return nil, false
		}
		return carrier.forwardFill(), true
	}
}
