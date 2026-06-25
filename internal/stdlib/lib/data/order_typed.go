package data

import (
	"cmp"
	"math"
	"slices"
)

const (
	maxTypedSortModuloResidues = 1 << 16
	sortMaxInt64Value          = int64(^uint64(0) >> 1)
	sortMinInt64Value          = -sortMaxInt64Value - 1
)

// orderIndexesTypedSingle sorts indexes by one bound order spec through a
// typed key extraction plus a pdqsort over (key, position) pairs. The
// position tie-break reproduces stable-sort semantics exactly, while the
// comparison runs on machine values instead of one interface dispatch plus
// boxed At per comparison. ok=false defers to the generic compare loop.
func orderIndexesTypedSingle(indexes []int, spec boundOrderSpec) ([]int, bool) {
	column := spec.column
	if att, ok := column.(attributedArray); ok {
		column = att.array
	}
	if values, owned, ok := tryBulkI64Values(column); ok {
		keys := make([]uint64, len(indexes))
		for i, row := range indexes {
			k := uint64(values[row]) ^ (1 << 63)
			if spec.spec.Desc {
				k = ^k
			}
			keys[i] = k
		}
		bulkI64Release(values, owned)
		return sortIndexesByPreparedKeys(indexes, keys), true
	}
	switch a := column.(type) {
	case columnArray[string]:
		return sortIndexesByKeys(indexes, a.data, spec.spec.Desc), true
	case columnArray[Symbol]:
		return sortIndexesByKeys(indexes, a.data, spec.spec.Desc), true
	}
	if column.Kind() == KindF64 {
		if values, owned, ok := tryBulkF64Values(column); ok {
			for _, v := range values {
				if v != v {
					// NaN keys: keep the generic path so the established
					// compareFloat64 tie semantics stay byte-identical.
					bulkF64Release(values, owned)
					return nil, false
				}
			}
			keys := make([]uint64, len(indexes))
			for i, row := range indexes {
				bits := math.Float64bits(values[row])
				if bits>>63 == 1 {
					bits = ^bits
				} else {
					bits |= 1 << 63
				}
				if spec.spec.Desc {
					bits = ^bits
				}
				keys[i] = bits
			}
			bulkF64Release(values, owned)
			return sortIndexesByPreparedKeys(indexes, keys), true
		}
	}
	return nil, false
}

// sortIndexesByPreparedKeys stably sorts indexes by pre-biased uint64 keys
// (keys[i] belongs to indexes[i]). Large inputs run a stable LSD radix sort
// whose pass count scales with the observed key span; small inputs use the
// pdq pair sort. Stability comes from the radix scatter (or the position
// tie-break in the pair sort), matching sort.SliceStable semantics.
func sortIndexesByPreparedKeys(indexes []int, keys []uint64) []int {
	n := len(indexes)
	if n == 0 {
		return []int{}
	}
	perm := stableKeyPermutation(keys)
	if perm == nil {
		return append([]int(nil), indexes...)
	}
	out := make([]int, n)
	for i, pos := range perm {
		out[i] = indexes[pos]
	}
	return out
}

// typedSortIndexesI64Keys stably sorts identity row indexes by dense int64
// values through the same prepared-key machinery as the frame order-by route:
// sign-biased uint64 keys (complemented for descending) feed the span-scaled
// LSD radix sort, or the position-tie-break pair sort for small inputs. The
// output permutation is identical to the sort.SliceStable comparison route it
// replaces, without the per-comparison closure dispatch.
func typedSortIndexesI64Keys(values []int64, descending bool) Array {
	n := len(values)
	if n == 0 {
		return NewI64Range(0, 1, 0)
	}
	keys := make([]uint64, n)
	if descending {
		for i, v := range values {
			keys[i] = ^(uint64(v) ^ (1 << 63))
		}
	} else {
		for i, v := range values {
			keys[i] = uint64(v) ^ (1 << 63)
		}
	}
	out := make([]int64, n)
	perm := stableKeyPermutation(keys)
	if perm == nil {
		for i := range out {
			out[i] = int64(i)
		}
		return newI64Trusted(out)
	}
	for i, pos := range perm {
		out[i] = int64(pos)
	}
	return newI64Trusted(out)
}

func typedSortIndexesI64ModuloRange(array i64ScalarDyadicArray, descending bool) (Array, bool) {
	array, descending, ok := normalizeI64ModuloSortArray(array, descending)
	if !ok || array.len < 0 {
		return nil, false
	}
	var source i64RangeArray
	switch s := array.source.(type) {
	case i64RangeArray:
		source = s
	case i64ScalarDyadicArray:
		affine, ok := i64ScalarDyadicAffineRange(s)
		if !ok {
			return nil, false
		}
		source = affine
	default:
		return nil, false
	}
	if source.len < array.len {
		return nil, false
	}
	if array.len == 0 {
		return NewI64Range(0, 1, 0), true
	}
	if source.step == 0 {
		return NewI64Range(0, 1, array.len), true
	}
	modulus := array.scalar
	if modulus > maxTypedSortModuloResidues {
		return nil, false
	}
	period := modulus / gcdInt64(source.step, modulus)
	if period <= 0 || period > maxTypedSortModuloResidues {
		return nil, false
	}
	offsets := make([]int, int(modulus))
	for i := range offsets {
		offsets[i] = -1
	}
	value := qPositiveMod(source.start, modulus)
	step := qPositiveMod(source.step, modulus)
	for offset := int64(0); offset < period; offset++ {
		offsets[int(value)] = int(offset)
		value = (value + step) % modulus
	}
	segments := make([]i64RangeArray, 0, min(int(modulus), array.len))
	writeResidue := func(residue int64, outLen int) int {
		offset := offsets[int(residue)]
		if offset < 0 {
			return outLen
		}
		segmentLen := ((array.len - 1 - offset) / int(period)) + 1
		if segmentLen <= 0 {
			return outLen
		}
		segments = append(segments, i64RangeArray{start: int64(offset), step: period, len: segmentLen})
		return outLen + segmentLen
	}
	outLen := 0
	if descending {
		for residue := modulus - 1; residue >= 0; residue-- {
			outLen = writeResidue(residue, outLen)
		}
	} else {
		for residue := int64(0); residue < modulus; residue++ {
			outLen = writeResidue(residue, outLen)
		}
	}
	if outLen != array.len {
		return nil, false
	}
	return newI64SegmentArray(segments...), true
}

func normalizeI64ModuloSortArray(array i64ScalarDyadicArray, descending bool) (i64ScalarDyadicArray, bool, bool) {
	switch array.op {
	case OpMod:
		if array.scalarLeft || array.scalar <= 0 {
			return i64ScalarDyadicArray{}, false, false
		}
		return array, descending, true
	case OpAdd:
		inner, ok := array.source.(i64ScalarDyadicArray)
		if !ok {
			return i64ScalarDyadicArray{}, false, false
		}
		mod, desc, ok := normalizeI64ModuloSortArray(inner, descending)
		if !ok || !i64SortAddPreservesModuloRange(array.scalar, mod.scalar) {
			return i64ScalarDyadicArray{}, false, false
		}
		return mod, desc, true
	case OpSub:
		inner, ok := array.source.(i64ScalarDyadicArray)
		if !ok {
			return i64ScalarDyadicArray{}, false, false
		}
		mod, desc, ok := normalizeI64ModuloSortArray(inner, descending)
		if !ok {
			return i64ScalarDyadicArray{}, false, false
		}
		if array.scalarLeft {
			if !i64SortLeftSubPreservesModuloRange(array.scalar, mod.scalar) {
				return i64ScalarDyadicArray{}, false, false
			}
			return mod, !desc, true
		}
		if array.scalar == sortMinInt64Value || !i64SortAddPreservesModuloRange(-array.scalar, mod.scalar) {
			return i64ScalarDyadicArray{}, false, false
		}
		return mod, desc, true
	default:
		return i64ScalarDyadicArray{}, false, false
	}
}

func i64SortAddPreservesModuloRange(shift, modulus int64) bool {
	if modulus <= 0 {
		return false
	}
	maxValue := modulus - 1
	if shift > 0 {
		return shift <= sortMaxInt64Value-maxValue
	}
	return true
}

func i64SortLeftSubPreservesModuloRange(shift, modulus int64) bool {
	if modulus <= 0 {
		return false
	}
	maxValue := modulus - 1
	return shift >= sortMinInt64Value+maxValue
}

// stableKeyPermutation returns the stable ascending order of keys as original
// positions: result[i] is the position of the i-th smallest key, and equal
// keys keep their original relative order (sort.SliceStable semantics). A nil
// result means every key is equal, i.e. the identity permutation. Large
// inputs run a stable LSD radix sort whose pass count scales with the
// observed key span; small inputs use the pdq pair sort with a position
// tie-break. The radix path scrambles the caller's keys slice in place.
func stableKeyPermutation(keys []uint64) []int {
	n := len(keys)
	minKey, maxKey := keys[0], keys[0]
	for _, k := range keys[1:] {
		if k < minKey {
			minKey = k
		}
		if k > maxKey {
			maxKey = k
		}
	}
	if minKey == maxKey {
		return nil
	}
	if n < 256 || n > math.MaxInt32 {
		pairs := make([]orderKeyPos[uint64], n)
		for i := 0; i < n; i++ {
			pairs[i] = orderKeyPos[uint64]{key: keys[i], pos: i}
		}
		slices.SortFunc(pairs, func(a, b orderKeyPos[uint64]) int {
			if a.key < b.key {
				return -1
			}
			if a.key > b.key {
				return 1
			}
			return a.pos - b.pos
		})
		out := make([]int, n)
		for i, pair := range pairs {
			out[i] = pair.pos
		}
		return out
	}
	span := maxKey - minKey
	passes := 0
	for s := span; s != 0; s >>= 8 {
		passes++
	}
	positions := make([]int32, n)
	for i := range positions {
		positions[i] = int32(i)
	}
	tmpKeys := make([]uint64, n)
	tmpPositions := make([]int32, n)
	var counts [256]int
	for pass := 0; pass < passes; pass++ {
		shift := uint(pass * 8)
		for i := range counts {
			counts[i] = 0
		}
		for i := 0; i < n; i++ {
			counts[byte((keys[i]-minKey)>>shift)]++
		}
		offset := 0
		for i := 0; i < 256; i++ {
			c := counts[i]
			counts[i] = offset
			offset += c
		}
		for i := 0; i < n; i++ {
			digit := byte((keys[i] - minKey) >> shift)
			j := counts[digit]
			counts[digit]++
			tmpKeys[j] = keys[i]
			tmpPositions[j] = positions[i]
		}
		keys, tmpKeys = tmpKeys, keys
		positions, tmpPositions = tmpPositions, positions
	}
	out := make([]int, n)
	for i := 0; i < n; i++ {
		out[i] = int(positions[i])
	}
	return out
}

type orderKeyPos[T cmp.Ordered] struct {
	key T
	pos int
}

func sortIndexesByKeys[T cmp.Ordered](indexes []int, values []T, desc bool) []int {
	pairs := make([]orderKeyPos[T], len(indexes))
	for i, row := range indexes {
		pairs[i] = orderKeyPos[T]{key: values[row], pos: i}
	}
	if desc {
		slices.SortFunc(pairs, func(a, b orderKeyPos[T]) int {
			if a.key < b.key {
				return 1
			}
			if a.key > b.key {
				return -1
			}
			return a.pos - b.pos
		})
	} else {
		slices.SortFunc(pairs, func(a, b orderKeyPos[T]) int {
			if a.key < b.key {
				return -1
			}
			if a.key > b.key {
				return 1
			}
			return a.pos - b.pos
		})
	}
	out := make([]int, len(pairs))
	for i, pair := range pairs {
		out[i] = indexes[pair.pos]
	}
	return out
}
