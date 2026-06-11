package data

import (
	"cmp"
	"math"
	"slices"
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
		return append([]int(nil), indexes...)
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
			out[i] = indexes[pair.pos]
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
		out[i] = indexes[positions[i]]
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
