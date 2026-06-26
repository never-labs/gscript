package data

import (
	"sort"
	"strings"
	"unsafe"
)

// Typed asof/window join matching.
//
// The boxed asof/window pipeline builds a map[string][]int partition index on
// the right frame and probes it with per-left-row encoded string keys, paying
// several allocations per row plus boxed At calls inside every binary-search
// step. These kernels keep the same partition + sorted-time-walk semantics
// but operate on typed key slices (map[T][]int built once) and dense int64
// time reprs, so matching is allocation-free per row. They cover the dominant
// shapes — int64-family time columns with zero or one typed partition column —
// and return ok=false for everything else, deferring to the boxed path.

// asofTimeI64 returns the dense int64 repr of an int64-backed time column.
// All covered kinds have int64 as their underlying type, so the repr is a
// zero-copy read-only view of the column storage.
func asofTimeI64(array Array) ([]int64, bool) {
	switch a := unwrapAttributedArray(array).(type) {
	case columnArray[int64]:
		return a.data, true
	case columnArray[Month]:
		return int64SliceView(a.data), true
	case columnArray[Date]:
		return int64SliceView(a.data), true
	case columnArray[DateTime]:
		return int64SliceView(a.data), true
	case columnArray[Timespan]:
		return int64SliceView(a.data), true
	case columnArray[Minute]:
		return int64SliceView(a.data), true
	case columnArray[Second]:
		return int64SliceView(a.data), true
	case columnArray[Time]:
		return int64SliceView(a.data), true
	case columnArray[Timestamp]:
		return int64SliceView(a.data), true
	default:
		return nil, false
	}
}

// int64SliceView reinterprets a named-int64 slice as []int64 without copying.
// The result aliases the source storage and must be treated as read-only.
func int64SliceView[T ~int64](values []T) []int64 {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*int64)(unsafe.Pointer(unsafe.SliceData(values))), len(values))
}

// asofMatchIndexesTypedFast computes the per-left-row right matches for an
// asof join entirely through typed kernels. ok=false defers to the boxed
// SortedRowsByPartition + AsofMatchIndexes pipeline.
func asofMatchIndexesTypedFast(left Frame, leftTime Array, leftPartitionCols []Symbol, right Frame, rightTime Array, rightPartitionCols []Symbol) ([]int, bool) {
	leftTimes, ok := asofTimeI64(leftTime)
	if !ok {
		return nil, false
	}
	rightTimes, ok := asofTimeI64(rightTime)
	if !ok {
		return nil, false
	}
	switch len(leftPartitionCols) {
	case 0:
		rows := sortedTimeRows(rightTimes, allIndexes(len(rightTimes)))
		// Match vectors are transient: AsofJoinOnWithOptions gathers through
		// them and releases them back to the bulk pool.
		out := bulkIntGetLen(len(leftTimes))
		asofMatchAllRowsInto(leftTimes, rows, rightTimes, out)
		return out, true
	case 1:
		leftPart, okL := left.Column(leftPartitionCols[0])
		rightPart, okR := right.Column(rightPartitionCols[0])
		if !okL || !okR {
			return nil, false
		}
		if _, leftRows, resolved, ok := partitionAlignedRows(left, leftPartitionCols[0], leftPart, right, rightPartitionCols[0], rightPart, rightTimes); ok {
			out := bulkIntGetLen(len(leftTimes))
			for g, lrows := range leftRows {
				asofMatchRowsInto(lrows, leftTimes, resolved[g], rightTimes, out)
			}
			return out, true
		}
		return asofMatchSinglePartitionTyped(leftPart, rightPart, leftTimes, rightTimes)
	default:
		return nil, false
	}
}

// partitionAlignedRows aligns the left partition column's groups with the
// right partition column's time-sorted row lists through both columns' cached
// grouped ArrayIndexes: one probe per distinct left key instead of one string
// hash per left row. It also exposes the left groups' row lists so callers
// can merge-walk time-sorted partitions. It requires identical partition
// kinds (the common case; cross-kind coercion stays on the boxed path).
func partitionAlignedRows(left Frame, leftName Symbol, leftPart Array, right Frame, rightName Symbol, rightPart Array, rightTimes []int64) ([]int, [][]int, [][]int, bool) {
	if leftPart.Kind() != rightPart.Kind() {
		return nil, nil, nil, false
	}
	switch leftPart.Kind() {
	case KindF32, KindF64, KindAny, KindNull:
		// Float/boxed partition keys keep the boxed encoder semantics.
		return nil, nil, nil, false
	}
	leftIndex, ok := groupedArrayIndexCached(left, leftName, leftPart)
	if !ok {
		return nil, nil, nil, false
	}
	rightIndex, ok := groupedArrayIndexCached(right, rightName, rightPart)
	if !ok {
		return nil, nil, nil, false
	}
	leftIDs, err := rowToGroupFromIndex(leftIndex)
	if err != nil || len(leftIDs) != leftPart.Len() {
		return nil, nil, nil, false
	}
	kind := leftPart.Kind()
	resolved := make([][]int, len(leftIndex.Keys))
	for g, key := range leftIndex.Keys {
		rows, ok := typedIndexRowsByKey(rightIndex, key)
		if !ok {
			rows = rightIndex.RowsByKey[arrayValueKey(kind, key)]
		}
		if len(rows) > 0 && !int64RowsSorted(rightTimes, rows) {
			rows = append([]int(nil), rows...)
			sort.SliceStable(rows, func(i, j int) bool {
				return rightTimes[rows[i]] < rightTimes[rows[j]]
			})
		}
		resolved[g] = rows
	}
	return leftIDs, leftIndex.Rows, resolved, true
}

func typedIndexRowsByKey(index ArrayIndex, key any) ([]int, bool) {
	switch rowsByKey := index.typedRowsByKey.(type) {
	case map[bool][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[int8][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[int16][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[int32][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[int64][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[uint8][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[uint16][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[uint32][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[uint64][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[string][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Symbol][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Month][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Date][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[DateTime][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Timespan][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Minute][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Second][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Time][]int:
		return rowsByTypedKey(rowsByKey, key)
	case map[Timestamp][]int:
		return rowsByTypedKey(rowsByKey, key)
	default:
		return nil, false
	}
}

func rowsByTypedKey[T comparable](rowsByKey map[T][]int, key any) ([]int, bool) {
	typed, ok := key.(T)
	if !ok {
		return nil, false
	}
	return rowsByKey[typed], true
}

// windowMatchIndexesTypedFast computes per-left-row right row windows for a
// window join through typed kernels. The returned row slices alias shared
// backing storage and must be treated as read-only.
func windowMatchIndexesTypedFast(left Frame, leftTime Array, leftPartitionCols []Symbol, right Frame, rightTime Array, rightPartitionCols []Symbol, opts WindowJoinOptions) ([][]int, bool) {
	leftTimes, ok := asofTimeI64(leftTime)
	if !ok {
		return nil, false
	}
	rightTimes, ok := asofTimeI64(rightTime)
	if !ok {
		return nil, false
	}
	bounds := windowI64Bounds{}
	if opts.HasBounds {
		low, okLow := windowDeltaI64(leftTime.Kind(), opts.Low)
		high, okHigh := windowDeltaI64(leftTime.Kind(), opts.High)
		if !okLow || !okHigh || low > high {
			return nil, false
		}
		bounds = windowI64Bounds{has: true, low: low, high: high}
	}
	switch len(leftPartitionCols) {
	case 0:
		rows := sortedTimeRows(rightTimes, allIndexes(len(rightTimes)))
		out := make([][]int, len(leftTimes))
		for row, t := range leftTimes {
			out[row] = windowSliceRows(rows, rightTimes, t, bounds)
		}
		return out, true
	case 1:
		leftPart, okL := left.Column(leftPartitionCols[0])
		rightPart, okR := right.Column(rightPartitionCols[0])
		if !okL || !okR {
			return nil, false
		}
		if leftIDs, _, resolved, ok := partitionAlignedRows(left, leftPartitionCols[0], leftPart, right, rightPartitionCols[0], rightPart, rightTimes); ok {
			out := make([][]int, len(leftTimes))
			for row, t := range leftTimes {
				out[row] = windowSliceRows(resolved[leftIDs[row]], rightTimes, t, bounds)
			}
			return out, true
		}
		return windowMatchSinglePartitionTyped(leftPart, rightPart, leftTimes, rightTimes, bounds)
	default:
		return nil, false
	}
}

// windowLastMatchIndexesTypedFast computes the wj1/right-last shape directly.
// It avoids materializing a per-left-row window list when the caller only
// needs the last matching right row.
func windowLastMatchIndexesTypedFast(left Frame, leftTime Array, leftPartitionCols []Symbol, right Frame, rightTime Array, rightPartitionCols []Symbol, opts WindowJoinOptions) ([]int, bool) {
	leftTimes, ok := asofTimeI64(leftTime)
	if !ok {
		return nil, false
	}
	rightTimes, ok := asofTimeI64(rightTime)
	if !ok {
		return nil, false
	}
	bounds := windowI64Bounds{}
	if opts.HasBounds {
		low, okLow := windowDeltaI64(leftTime.Kind(), opts.Low)
		high, okHigh := windowDeltaI64(leftTime.Kind(), opts.High)
		if !okLow || !okHigh || low > high {
			return nil, false
		}
		bounds = windowI64Bounds{has: true, low: low, high: high}
	}
	switch len(leftPartitionCols) {
	case 0:
		rows := sortedTimeRows(rightTimes, allIndexes(len(rightTimes)))
		out := bulkIntGetLen(len(leftTimes))
		for row, t := range leftTimes {
			out[row] = windowLastRow(rows, rightTimes, t, bounds)
		}
		return out, true
	case 1:
		leftPart, okL := left.Column(leftPartitionCols[0])
		rightPart, okR := right.Column(rightPartitionCols[0])
		if !okL || !okR {
			return nil, false
		}
		if leftIDs, _, resolved, ok := partitionAlignedRows(left, leftPartitionCols[0], leftPart, right, rightPartitionCols[0], rightPart, rightTimes); ok {
			out := bulkIntGetLen(len(leftTimes))
			for row, t := range leftTimes {
				out[row] = windowLastRow(resolved[leftIDs[row]], rightTimes, t, bounds)
			}
			return out, true
		}
		return windowLastMatchSinglePartitionTyped(leftPart, rightPart, leftTimes, rightTimes, bounds)
	default:
		return nil, false
	}
}

type windowI64Bounds struct {
	has  bool
	low  int64
	high int64
}

// windowDeltaI64 converts a window bound delta into the time column's int64
// repr units, mirroring addWindowDelta/addWindowIntDelta (whose temporal
// From*/To* conversions are all identity on the int64 repr).
func windowDeltaI64(timeKind Kind, delta any) (int64, bool) {
	if d, ok := numeric(delta); ok {
		if d != float64(int64(d)) {
			return 0, false
		}
		switch timeKind {
		case KindI64, KindMonth, KindDate, KindDateTime, KindTimespan, KindMinute, KindSecond, KindTime, KindTimestamp:
			return int64(d), true
		}
		return 0, false
	}
	if d, ok := delta.(Timespan); ok {
		switch timeKind {
		case KindDateTime, KindTimespan, KindTime, KindTimestamp:
			return int64(d), true
		}
	}
	return 0, false
}

func asofMatchSinglePartitionTyped(leftPart, rightPart Array, leftTimes, rightTimes []int64) ([]int, bool) {
	switch lk := unwrapAttributedArray(leftPart).(type) {
	case columnArray[Symbol]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[Symbol]); ok && lk.kind == rk.kind {
			return asofMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes), true
		}
	case columnArray[string]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[string]); ok && lk.kind == rk.kind {
			return asofMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes), true
		}
	case columnArray[int64]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[int64]); ok && lk.kind == rk.kind {
			return asofMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes), true
		}
	case columnArray[int32]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[int32]); ok && lk.kind == rk.kind {
			return asofMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes), true
		}
	}
	return nil, false
}

func windowMatchSinglePartitionTyped(leftPart, rightPart Array, leftTimes, rightTimes []int64, bounds windowI64Bounds) ([][]int, bool) {
	switch lk := unwrapAttributedArray(leftPart).(type) {
	case columnArray[Symbol]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[Symbol]); ok && lk.kind == rk.kind {
			return windowMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	case columnArray[string]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[string]); ok && lk.kind == rk.kind {
			return windowMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	case columnArray[int64]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[int64]); ok && lk.kind == rk.kind {
			return windowMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	case columnArray[int32]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[int32]); ok && lk.kind == rk.kind {
			return windowMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	}
	return nil, false
}

func windowLastMatchSinglePartitionTyped(leftPart, rightPart Array, leftTimes, rightTimes []int64, bounds windowI64Bounds) ([]int, bool) {
	switch lk := unwrapAttributedArray(leftPart).(type) {
	case columnArray[Symbol]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[Symbol]); ok && lk.kind == rk.kind {
			return windowLastMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	case columnArray[string]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[string]); ok && lk.kind == rk.kind {
			return windowLastMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	case columnArray[int64]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[int64]); ok && lk.kind == rk.kind {
			return windowLastMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	case columnArray[int32]:
		if rk, ok := unwrapAttributedArray(rightPart).(columnArray[int32]); ok && lk.kind == rk.kind {
			return windowLastMatchPartitionedTyped(lk.data, leftTimes, rk.data, rightTimes, bounds), true
		}
	}
	return nil, false
}

// typedPartitionRows builds the right-side partition index: per distinct key,
// row indexes in time order.
func typedPartitionRows[T comparable](rightKeys []T, rightTimes []int64) map[T][]int {
	rowsByKey := make(map[T][]int, 64)
	for row, key := range rightKeys {
		rowsByKey[key] = append(rowsByKey[key], row)
	}
	for key, rows := range rowsByKey {
		rowsByKey[key] = sortedTimeRows(rightTimes, rows)
	}
	return rowsByKey
}

func sortedTimeRows(times []int64, rows []int) []int {
	if !int64RowsSorted(times, rows) {
		sort.SliceStable(rows, func(i, j int) bool {
			return times[rows[i]] < times[rows[j]]
		})
	}
	return rows
}

// asofMatchRowsInto resolves the asof match for every left row in lrows
// against the time-sorted right rows. When the left rows are themselves in
// nondecreasing time order (the dominant temporal-table layout) the matches
// come from a single merge walk; otherwise each row binary-searches.
func asofMatchRowsInto(lrows []int, leftTimes []int64, rrows []int, rightTimes []int64, out []int) {
	if !int64RowsSorted(leftTimes, lrows) {
		for _, row := range lrows {
			out[row] = asofSearchRows(rrows, rightTimes, leftTimes[row])
		}
		return
	}
	j := 0
	for _, row := range lrows {
		t := leftTimes[row]
		for j < len(rrows) && rightTimes[rrows[j]] <= t {
			j++
		}
		if j == 0 {
			out[row] = -1
		} else {
			out[row] = rrows[j-1]
		}
	}
}

// asofMatchAllRowsInto is asofMatchRowsInto for an unpartitioned left side
// (every left row participates in time order when sorted).
func asofMatchAllRowsInto(leftTimes []int64, rrows []int, rightTimes []int64, out []int) {
	if !int64SliceSorted(leftTimes) {
		for row, t := range leftTimes {
			out[row] = asofSearchRows(rrows, rightTimes, t)
		}
		return
	}
	j := 0
	for row, t := range leftTimes {
		for j < len(rrows) && rightTimes[rrows[j]] <= t {
			j++
		}
		if j == 0 {
			out[row] = -1
		} else {
			out[row] = rrows[j-1]
		}
	}
}

func int64SliceSorted(values []int64) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			return false
		}
	}
	return true
}

// asofSearchRows returns the last row whose time is <= t, or -1.
func asofSearchRows(rows []int, times []int64, t int64) int {
	lo, hi := 0, len(rows)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if times[rows[mid]] > t {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if lo == 0 {
		return -1
	}
	return rows[lo-1]
}

// windowSearchRows returns the first index whose time satisfies after(t)
// per the predicate "times[rows[i]] > limit" (strict) or ">= limit".
func windowSearchRows(rows []int, times []int64, limit int64, strict bool) int {
	lo, hi := 0, len(rows)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		v := times[rows[mid]]
		if v > limit || (!strict && v >= limit) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func windowSliceRows(rows []int, times []int64, t int64, bounds windowI64Bounds) []int {
	if !bounds.has {
		end := windowSearchRows(rows, times, t, true)
		return rows[:end]
	}
	start := windowSearchRows(rows, times, t+bounds.low, false)
	end := windowSearchRows(rows, times, t+bounds.high, true)
	if start > end {
		start = end
	}
	return rows[start:end]
}

func windowLastRow(rows []int, times []int64, t int64, bounds windowI64Bounds) int {
	if !bounds.has {
		return asofSearchRows(rows, times, t)
	}
	start := windowSearchRows(rows, times, t+bounds.low, false)
	end := windowSearchRows(rows, times, t+bounds.high, true)
	if start >= end {
		return -1
	}
	return rows[end-1]
}

func asofMatchPartitionedTyped[T comparable](leftKeys []T, leftTimes []int64, rightKeys []T, rightTimes []int64) []int {
	rowsByKey := typedPartitionRows(rightKeys, rightTimes)
	out := bulkIntGetLen(len(leftKeys))
	for row, key := range leftKeys {
		out[row] = asofSearchRows(rowsByKey[key], rightTimes, leftTimes[row])
	}
	return out
}

func windowMatchPartitionedTyped[T comparable](leftKeys []T, leftTimes []int64, rightKeys []T, rightTimes []int64, bounds windowI64Bounds) [][]int {
	rowsByKey := typedPartitionRows(rightKeys, rightTimes)
	out := make([][]int, len(leftKeys))
	for row, key := range leftKeys {
		out[row] = windowSliceRows(rowsByKey[key], rightTimes, leftTimes[row], bounds)
	}
	return out
}

func windowLastMatchPartitionedTyped[T comparable](leftKeys []T, leftTimes []int64, rightKeys []T, rightTimes []int64, bounds windowI64Bounds) []int {
	rowsByKey := typedPartitionRows(rightKeys, rightTimes)
	out := bulkIntGetLen(len(leftKeys))
	for row, key := range leftKeys {
		out[row] = windowLastRow(rowsByKey[key], rightTimes, leftTimes[row], bounds)
	}
	return out
}

func windowLastMatchIndexesI64(left Frame, leftTime []int64, leftPartitionColumns []Symbol, rightTime []int64, rightByPartition map[string][]int, partitionKinds []Kind, bounds windowI64Bounds) ([]int, error) {
	encoder, err := newRowKeyEncoderWithKinds(left, leftPartitionColumns, partitionKinds)
	if err != nil {
		return nil, err
	}
	rightIndexes := make([]int, left.Len())
	var b strings.Builder
	for row := 0; row < left.Len(); row++ {
		rightIndexes[row] = -1
		key, err := encoder.keyWithBuilder(row, &b)
		if err != nil {
			return nil, err
		}
		rightIndexes[row] = windowLastRow(rightByPartition[key], rightTime, leftTime[row], bounds)
	}
	return rightIndexes, nil
}
