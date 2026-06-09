package data

import "fmt"

// SequenceItems exposes scalar-or-array inputs as a flat item slice for
// language frontends that support scalar extension over list operations.
func SequenceItems(value any) []any {
	if array, ok := value.(Array); ok {
		return array.Values()
	}
	return []any{value}
}

// SequenceCount returns the logical element count for reusable sequence-like
// values. Scalars count as one item, while strings count runes because they are
// list-like character sequences in q/Leia frontends.
func SequenceCount(value any) int64 {
	switch x := value.(type) {
	case Array:
		return int64(x.Len())
	case Frame:
		return int64(x.Len())
	case string:
		return int64(len([]rune(x)))
	default:
		return 1
	}
}

// Cross returns the Cartesian product of two scalar-or-array values. The
// product rows are two-item arrays so callers can preserve tuple-like shape.
func Cross(left, right any) Array {
	leftValues := SequenceItems(left)
	rightValues := SequenceItems(right)
	out := make([]any, 0, len(leftValues)*len(rightValues))
	for _, l := range leftValues {
		for _, r := range rightValues {
			out = append(out, NewAny([]any{l, r}))
		}
	}
	return NewAny(out)
}

// CrossCount returns the row count of Cross without materializing tuple rows.
// It mirrors SequenceItems scalar extension: non-array values, including
// strings, are a single cross item.
func CrossCount(left, right any) int64 {
	return sequenceItemsCount(left) * sequenceItemsCount(right)
}

// Cut splits an array, string, or frame at monotonically interpreted segment
// starts. Starts outside bounds are clamped, matching existing q cut/drop
// behavior while keeping the operation runtime-independent.
func Cut(indexes []int, value any) (any, error) {
	switch x := value.(type) {
	case Array:
		segments := make([]any, len(indexes))
		for i, start := range indexes {
			end := x.Len()
			if i+1 < len(indexes) {
				end = indexes[i+1]
			}
			part, err := Gather(x, SegmentIndexes(x.Len(), start, end))
			if err != nil {
				return nil, err
			}
			segments[i] = part
		}
		return NewAny(segments), nil
	case string:
		runes := []rune(x)
		segments := make([]any, len(indexes))
		for i, start := range indexes {
			end := len(runes)
			if i+1 < len(indexes) {
				end = indexes[i+1]
			}
			gather := SegmentIndexes(len(runes), start, end)
			out := make([]rune, len(gather))
			for j, index := range gather {
				out[j] = runes[index]
			}
			segments[i] = string(out)
		}
		return NewAny(segments), nil
	case Frame:
		segments := make([]any, len(indexes))
		for i, start := range indexes {
			end := x.Len()
			if i+1 < len(indexes) {
				end = indexes[i+1]
			}
			part, err := GatherFrame(x, SegmentIndexes(x.Len(), start, end))
			if err != nil {
				return nil, err
			}
			segments[i] = part
		}
		return NewAny(segments), nil
	default:
		return nil, fmt.Errorf("cut expects an array, string, or frame")
	}
}

// CutCount returns the number of segments produced by Cut without
// materializing the segments.
func CutCount(indexes []int, value any) (int64, error) {
	switch value.(type) {
	case Array, string, Frame:
		return int64(len(indexes)), nil
	default:
		return 0, fmt.Errorf("cut expects an array, string, or frame")
	}
}

// Sublist returns a contiguous slice using q's start/count convention. It is a
// reusable runtime primitive for q and Leia frontends that need list slicing
// without materializing an intermediate drop result.
func Sublist(start, count int, value any) (any, error) {
	if start < 0 || count < 0 {
		return nil, fmt.Errorf("sublist expects non-negative start and count")
	}
	switch x := value.(type) {
	case Array:
		start, count = boundedStartCount(x.Len(), start, count)
		return Slice(x, start, count)
	case string:
		runes := []rune(x)
		start, count = boundedStartCount(len(runes), start, count)
		return string(runes[start : start+count]), nil
	case Frame:
		start, count = boundedStartCount(x.Len(), start, count)
		return GatherFrame(x, SegmentIndexes(x.Len(), start, start+count))
	default:
		return nil, fmt.Errorf("sublist expects an array, string, or frame")
	}
}

// SublistCount returns the row/rune count of Sublist without materializing the
// contiguous slice.
func SublistCount(start, count int, value any) (int64, error) {
	if start < 0 || count < 0 {
		return 0, fmt.Errorf("sublist expects non-negative start and count")
	}
	switch x := value.(type) {
	case Array:
		_, count = boundedStartCount(x.Len(), start, count)
		return int64(count), nil
	case string:
		_, count = boundedStartCount(len([]rune(x)), start, count)
		return int64(count), nil
	case Frame:
		_, count = boundedStartCount(x.Len(), start, count)
		return int64(count), nil
	default:
		return 0, fmt.Errorf("sublist expects an array, string, or frame")
	}
}

func SegmentIndexes(length, start, end int) []int {
	if start < 0 {
		start = 0
	}
	if start > length {
		start = length
	}
	if end < start {
		end = start
	}
	if end > length {
		end = length
	}
	indexes := make([]int, end-start)
	for i := range indexes {
		indexes[i] = start + i
	}
	return indexes
}

func boundedStartCount(length, start, count int) (int, int) {
	if start > length {
		start = length
	}
	if count > length-start {
		count = length - start
	}
	return start, count
}

func sequenceItemsCount(value any) int64 {
	if array, ok := value.(Array); ok {
		return int64(array.Len())
	}
	return 1
}
