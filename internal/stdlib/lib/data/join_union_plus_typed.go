package data

// Typed union/plus join matching and column materialization.
//
// The boxed union/plus join pipeline probes a map[string][]int with one
// encoded string key per left row and materializes output columns through
// per-row boxed At calls into []any storage. These kernels keep the same
// match semantics but build the right-side index once over typed key slices
// (map[T][]int / map[T]int) and materialize the merged key and shared-add
// columns through dense typed copies, so the per-row path is allocation-free.
// They cover the dominant single-key shapes — symbol/string/int64/int32 key
// columns of identical kind — and return ok=false for everything else,
// deferring to the boxed path.

// unionJoinIndexesTypedFast computes the (leftIndexes, rightIndexes) row
// pairing for a union join through typed kernels: every left row in order
// (matched right rows fanned out, unmatched paired with -1), then unmatched
// right rows appended with left index -1.
func unionJoinIndexesTypedFast(left, right Frame, keys []JoinKey) ([]int, []int, bool) {
	if len(keys) != 1 {
		return nil, nil, false
	}
	leftKey, okL := left.Column(keys[0].Left)
	rightKey, okR := right.Column(keys[0].Right)
	if !okL || !okR {
		return nil, nil, false
	}
	switch lk := unwrapAttributedArray(leftKey).(type) {
	case columnArray[Symbol]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[Symbol]); ok && lk.kind == rk.kind {
			l, r := unionJoinIndexesTyped(lk.data, rk.data)
			return l, r, true
		}
	case columnArray[string]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[string]); ok && lk.kind == rk.kind {
			l, r := unionJoinIndexesTyped(lk.data, rk.data)
			return l, r, true
		}
	case columnArray[int64]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[int64]); ok && lk.kind == rk.kind {
			l, r := unionJoinIndexesTyped(lk.data, rk.data)
			return l, r, true
		}
	case columnArray[int32]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[int32]); ok && lk.kind == rk.kind {
			l, r := unionJoinIndexesTyped(lk.data, rk.data)
			return l, r, true
		}
	}
	return nil, nil, false
}

func unionJoinIndexesTyped[T comparable](leftKeys, rightKeys []T) ([]int, []int) {
	rowsByKey := make(map[T][]int, 64)
	for row, key := range rightKeys {
		rowsByKey[key] = append(rowsByKey[key], row)
	}
	leftIndexes := make([]int, 0, len(leftKeys)+len(rightKeys))
	rightIndexes := make([]int, 0, len(leftKeys)+len(rightKeys))
	matchedRight := make([]bool, len(rightKeys))
	for row, key := range leftKeys {
		matches := rowsByKey[key]
		if len(matches) == 0 {
			leftIndexes = append(leftIndexes, row)
			rightIndexes = append(rightIndexes, -1)
			continue
		}
		for _, rightRow := range matches {
			leftIndexes = append(leftIndexes, row)
			rightIndexes = append(rightIndexes, rightRow)
			matchedRight[rightRow] = true
		}
	}
	for row := range rightKeys {
		if matchedRight[row] {
			continue
		}
		leftIndexes = append(leftIndexes, -1)
		rightIndexes = append(rightIndexes, row)
	}
	return leftIndexes, rightIndexes
}

// plusJoinMatchTypedFast computes the per-left-row first right match (-1 for
// no match) for a plus join through typed kernels.
func plusJoinMatchTypedFast(left, right Frame, keys []JoinKey) ([]int, bool) {
	if len(keys) != 1 {
		return nil, false
	}
	leftKey, okL := left.Column(keys[0].Left)
	rightKey, okR := right.Column(keys[0].Right)
	if !okL || !okR {
		return nil, false
	}
	switch lk := unwrapAttributedArray(leftKey).(type) {
	case columnArray[Symbol]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[Symbol]); ok && lk.kind == rk.kind {
			return plusJoinMatchTyped(lk.data, rk.data), true
		}
	case columnArray[string]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[string]); ok && lk.kind == rk.kind {
			return plusJoinMatchTyped(lk.data, rk.data), true
		}
	case columnArray[int64]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[int64]); ok && lk.kind == rk.kind {
			return plusJoinMatchTyped(lk.data, rk.data), true
		}
	case columnArray[int32]:
		if rk, ok := unwrapAttributedArray(rightKey).(columnArray[int32]); ok && lk.kind == rk.kind {
			return plusJoinMatchTyped(lk.data, rk.data), true
		}
	}
	return nil, false
}

func plusJoinMatchTyped[T comparable](leftKeys, rightKeys []T) []int {
	firstByKey := make(map[T]int, 64)
	for row := len(rightKeys) - 1; row >= 0; row-- {
		firstByKey[rightKeys[row]] = row
	}
	matched := make([]int, len(leftKeys))
	for row, key := range leftKeys {
		if rightRow, ok := firstByKey[key]; ok {
			matched[row] = rightRow
		} else {
			matched[row] = -1
		}
	}
	return matched
}

// unionCoalesceKeyTyped materializes a union join key column: values from the
// left column where leftIndexes[i] >= 0, otherwise from the right key column
// at rightIndexes[i]. Requires identical typed carriers on both sides.
func unionCoalesceKeyTyped(leftCol Array, leftIndexes []int, rightCol Array, rightIndexes []int) (Array, bool) {
	switch lc := unwrapAttributedArray(leftCol).(type) {
	case columnArray[Symbol]:
		if rc, ok := unwrapAttributedArray(rightCol).(columnArray[Symbol]); ok && lc.kind == rc.kind {
			return coalesceColumnArray(lc.kind, lc.data, leftIndexes, rc.data, rightIndexes), true
		}
	case columnArray[string]:
		if rc, ok := unwrapAttributedArray(rightCol).(columnArray[string]); ok && lc.kind == rc.kind {
			return coalesceColumnArray(lc.kind, lc.data, leftIndexes, rc.data, rightIndexes), true
		}
	case columnArray[int64]:
		if rc, ok := unwrapAttributedArray(rightCol).(columnArray[int64]); ok && lc.kind == rc.kind {
			return coalesceColumnArray(lc.kind, lc.data, leftIndexes, rc.data, rightIndexes), true
		}
	case columnArray[int32]:
		if rc, ok := unwrapAttributedArray(rightCol).(columnArray[int32]); ok && lc.kind == rc.kind {
			return coalesceColumnArray(lc.kind, lc.data, leftIndexes, rc.data, rightIndexes), true
		}
	case columnArray[float64]:
		if rc, ok := unwrapAttributedArray(rightCol).(columnArray[float64]); ok && lc.kind == rc.kind {
			return coalesceColumnArray(lc.kind, lc.data, leftIndexes, rc.data, rightIndexes), true
		}
	case columnArray[Timestamp]:
		if rc, ok := unwrapAttributedArray(rightCol).(columnArray[Timestamp]); ok && lc.kind == rc.kind {
			return coalesceColumnArray(lc.kind, lc.data, leftIndexes, rc.data, rightIndexes), true
		}
	}
	return nil, false
}

func coalesceColumnArray[T any](kind Kind, leftData []T, leftIndexes []int, rightData []T, rightIndexes []int) Array {
	out := make([]T, len(leftIndexes))
	for i, leftRow := range leftIndexes {
		if leftRow >= 0 {
			out[i] = leftData[leftRow]
			continue
		}
		out[i] = rightData[rightIndexes[i]]
	}
	return columnArray[T]{kind: kind, data: out}
}

// plusAddSharedTyped materializes a plus join shared numeric column:
// left[i] + right[matched[i]] where matched, otherwise left[i] unchanged.
// The boxed path computes each sum through scalar ApplyBinary(OpAdd), which
// always yields float64, and then column inference promotes any column that
// holds at least one sum to KindF64 (a fully unmatched column keeps the left
// values and kind). Requires plain int64/float64 carriers on both sides so
// that promotion contract is reproduced exactly.
func plusAddSharedTyped(leftCol, rightCol Array, matched []int) (Array, bool) {
	anyMatch := false
	for _, rightRow := range matched {
		if rightRow >= 0 {
			anyMatch = true
			break
		}
	}
	lc := unwrapAttributedArray(leftCol)
	rc := unwrapAttributedArray(rightCol)
	switch l := lc.(type) {
	case columnArray[int64]:
		if l.kind != KindI64 {
			return nil, false
		}
		if !anyMatch {
			out := make([]int64, len(matched))
			copy(out, l.data)
			return columnArray[int64]{kind: KindI64, data: out}, true
		}
		switch r := rc.(type) {
		case columnArray[int64]:
			if r.kind != KindI64 {
				return nil, false
			}
			return plusAddSharedF64(l.data, r.data, matched), true
		case columnArray[float64]:
			if r.kind != KindF64 {
				return nil, false
			}
			return plusAddSharedF64(l.data, r.data, matched), true
		}
	case columnArray[float64]:
		if l.kind != KindF64 {
			return nil, false
		}
		if !anyMatch {
			out := make([]float64, len(matched))
			copy(out, l.data)
			return columnArray[float64]{kind: KindF64, data: out}, true
		}
		switch r := rc.(type) {
		case columnArray[int64]:
			if r.kind != KindI64 {
				return nil, false
			}
			return plusAddSharedF64(l.data, r.data, matched), true
		case columnArray[float64]:
			if r.kind != KindF64 {
				return nil, false
			}
			return plusAddSharedF64(l.data, r.data, matched), true
		}
	}
	return nil, false
}

func plusAddSharedF64[L int64 | float64, R int64 | float64](leftData []L, rightData []R, matched []int) Array {
	out := make([]float64, len(matched))
	for i, rightRow := range matched {
		if rightRow >= 0 {
			out[i] = float64(leftData[i]) + float64(rightData[rightRow])
			continue
		}
		out[i] = float64(leftData[i])
	}
	return columnArray[float64]{kind: KindF64, data: out}
}
