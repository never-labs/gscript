package data

import "fmt"

const (
	SequenceTransformReverse = "reverse"
	SequenceTransformRotate  = "rotate"
	SequenceTransformSublist = "sublist"
	SequenceTransformCut     = "cut"
	SequenceTransformRaze    = "raze"
	SequenceTransformDeltas  = "deltas"
	SequenceTransformRatios  = "ratios"
)

type SequenceTransformStep struct {
	Transform string
	Args      [2]int
	ArgCount  int
}

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

// RazeCount returns count raze value without building the flattened sequence.
// Array rows contribute their row length; scalar rows contribute one item.
func RazeCount(value any) (int64, bool, error) {
	switch x := value.(type) {
	case Matrix:
		shape := x.Shape()
		if len(shape) != 2 {
			return 0, true, fmt.Errorf("raze count expects a two-dimensional matrix")
		}
		return int64(shape[0] * shape[1]), true, nil
	case Array:
		var count int64
		for row := 0; row < x.Len(); row++ {
			item, ok := x.At(row)
			if !ok {
				return 0, true, fmt.Errorf("raze count row %d out of range", row)
			}
			if array, ok := item.(Array); ok {
				count += int64(array.Len())
			} else {
				count++
			}
		}
		return count, true, nil
	default:
		return 0, false, nil
	}
}

// SequenceTransformCount returns the output length for list transforms whose
// count can be computed without materializing the transformed value.
func SequenceTransformCount(transform string, args []int, value any) (int64, bool, error) {
	switch transform {
	case SequenceTransformReverse, SequenceTransformRotate, SequenceTransformDeltas, SequenceTransformRatios:
		switch x := value.(type) {
		case Array:
			if (transform == SequenceTransformDeltas || transform == SequenceTransformRatios) && !isNumericArray(x) {
				return 0, false, nil
			}
			return int64(x.Len()), true, nil
		case Frame:
			return int64(x.Len()), true, nil
		case string:
			return int64(len([]rune(x))), true, nil
		default:
			if transform == SequenceTransformDeltas || transform == SequenceTransformRatios {
				if IsNull(value) {
					return 1, true, nil
				}
				if _, ok := numeric(value); ok {
					return 1, true, nil
				}
				return 0, false, nil
			}
			return 1, true, nil
		}
	case SequenceTransformSublist:
		switch len(args) {
		case 1:
			return TakeRepeatCount(args[0], value), true, nil
		case 2:
			out, err := SublistCount(args[0], args[1], value)
			return out, err == nil, err
		default:
			return 0, true, fmt.Errorf("sublist expects count or start count")
		}
	case SequenceTransformCut:
		out, err := CutCount(args, value)
		return out, err == nil, err
	case SequenceTransformRaze:
		return RazeCount(value)
	default:
		return 0, false, nil
	}
}

// TryTypedSequenceTransformNumericSum reduces common list transforms directly
// through reusable runtime primitives. It is intentionally transform-oriented
// so q and Leia frontends can share the same view/reduce behavior.
func TryTypedSequenceTransformNumericSum(transform string, args []int, value any) (any, bool, error) {
	switch transform {
	case SequenceTransformReverse:
		array, ok := value.(Array)
		if !ok {
			return nil, false, nil
		}
		return TryTypedNumericSum(array)
	case SequenceTransformRotate:
		if len(args) != 1 {
			return nil, true, fmt.Errorf("rotate expects an integer count")
		}
		array, ok := value.(Array)
		if !ok {
			return nil, false, nil
		}
		return TryTypedNumericSum(array)
	case SequenceTransformSublist:
		array, ok := value.(Array)
		if !ok {
			return nil, false, nil
		}
		var sliced Array
		var err error
		switch len(args) {
		case 1:
			sliced, err = TakeRepeat(array, args[0])
		case 2:
			start, count := boundedStartCount(array.Len(), args[0], args[1])
			sliced, err = Slice(array, start, count)
		default:
			return nil, true, fmt.Errorf("sublist expects count or start count")
		}
		if err != nil {
			return nil, true, err
		}
		return TryTypedNumericSum(sliced)
	case SequenceTransformRaze:
		return TryTypedNestedNumericSum(value)
	case SequenceTransformDeltas:
		array, ok := value.(Array)
		if !ok {
			return nil, false, nil
		}
		return TryTypedDeltasSum(array)
	case SequenceTransformRatios:
		array, ok := value.(Array)
		if !ok {
			return nil, false, nil
		}
		return TryTypedRatiosSum(array)
	default:
		return nil, false, nil
	}
}

// TryTypedSequenceTransformChainNumericSumFirstLast reduces
// sum(chain(value))+first(chain(value))+last(chain(value)) for index-only list
// transforms without constructing intermediate views. The primitive is shared by
// q and Leia frontends that lower expression DAGs to sequence pipeline shapes.
func TryTypedSequenceTransformChainNumericSumFirstLast(steps []SequenceTransformStep, value any) (any, bool, error) {
	array, ok := value.(Array)
	if !ok || len(steps) == 0 {
		return nil, false, nil
	}
	if len(steps) > 8 {
		return nil, false, nil
	}
	var lengths [9]int
	lengths[0] = array.Len()
	for i, step := range steps {
		next, handled, err := sequenceTransformStepLength(lengths[i], step)
		if err != nil || !handled {
			return nil, handled, err
		}
		lengths[i+1] = next
	}
	finalLen := lengths[len(steps)]
	if finalLen == 0 {
		return nil, false, nil
	}
	if values, ok := array.(i64RangeArray); ok {
		if total, handled := sequenceTransformChainI64RangeSumFirstLast(steps, lengths, values, finalLen); handled {
			return total, true, nil
		}
	}
	if isIntegerArray(array) {
		var total int64
		var first int64
		var last int64
		for row := 0; row < finalLen; row++ {
			sourceRow, ok, err := sequenceTransformSourceRow(row, steps, lengths)
			if err != nil || !ok {
				return nil, ok, err
			}
			value, ok, err := integerArrayAt(array, sourceRow)
			if err != nil || !ok {
				return nil, ok, err
			}
			if row == 0 {
				first = value
			}
			if row == finalLen-1 {
				last = value
			}
			total += value
		}
		return total + first + last, true, nil
	}
	var total float64
	var first float64
	var last float64
	for row := 0; row < finalLen; row++ {
		sourceRow, ok, err := sequenceTransformSourceRow(row, steps, lengths)
		if err != nil || !ok {
			return nil, ok, err
		}
		raw, ok := array.At(sourceRow)
		if !ok {
			return nil, false, fmt.Errorf("array row %d out of range", sourceRow)
		}
		value, ok := numeric(raw)
		if !ok {
			return nil, false, nil
		}
		if row == 0 {
			first = value
		}
		if row == finalLen-1 {
			last = value
		}
		total += value
	}
	return total + first + last, true, nil
}

func sequenceTransformChainI64RangeSumFirstLast(steps []SequenceTransformStep, lengths [9]int, values i64RangeArray, finalLen int) (int64, bool) {
	if values.len != lengths[0] || finalLen <= 0 {
		return 0, false
	}
	offset := int64(0)
	stride := int64(1)
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		prevLen := lengths[i]
		if prevLen <= 0 {
			return 0, false
		}
		switch step.Transform {
		case SequenceTransformReverse:
			offset = int64(prevLen-1) - offset
			stride = -stride
		case SequenceTransformRotate:
			shift := step.Args[0] % prevLen
			if shift < 0 {
				shift += prevLen
			}
			offset += int64(shift)
		case SequenceTransformSublist:
			switch step.ArgCount {
			case 1:
				start := 0
				if step.Args[0] < 0 {
					count := -step.Args[0]
					start = prevLen - count%prevLen
					if start == prevLen {
						start = 0
					}
				}
				offset += int64(start)
			case 2:
				start, _ := boundedStartCount(prevLen, step.Args[0], step.Args[1])
				offset += int64(start)
			default:
				return 0, false
			}
		default:
			return 0, false
		}
	}
	baseLen := int64(values.len)
	firstRow := positiveModInt64(offset, baseLen)
	lastRow := positiveModInt64(offset+stride*int64(finalLen-1), baseLen)
	if sequenceAffineRowsNoWrap(offset, stride, finalLen, baseLen) {
		rowSum := int64(finalLen) * (2*firstRow + int64(finalLen-1)*stride) / 2
		total := int64(finalLen)*values.start + values.step*rowSum
		return total + (values.start + values.step*firstRow) + (values.start + values.step*lastRow), true
	}
	var total int64
	for row := 0; row < finalLen; row++ {
		sourceRow := positiveModInt64(offset+stride*int64(row), baseLen)
		total += values.start + values.step*sourceRow
	}
	return total + (values.start + values.step*firstRow) + (values.start + values.step*lastRow), true
}

func sequenceAffineRowsNoWrap(offset, stride int64, length int, modulus int64) bool {
	if length <= 0 || modulus <= 0 {
		return false
	}
	first := offset
	last := offset + stride*int64(length-1)
	if stride >= 0 {
		return first >= 0 && last < modulus
	}
	return last >= 0 && first < modulus
}

func positiveModInt64(value, modulus int64) int64 {
	if modulus <= 0 {
		return 0
	}
	out := value % modulus
	if out < 0 {
		out += modulus
	}
	return out
}

func sequenceTransformStepLength(length int, step SequenceTransformStep) (int, bool, error) {
	switch step.Transform {
	case SequenceTransformReverse, SequenceTransformRotate:
		return length, true, nil
	case SequenceTransformSublist:
		switch step.ArgCount {
		case 1:
			if length == 0 || step.Args[0] == 0 {
				return 0, true, nil
			}
			count := step.Args[0]
			if count < 0 {
				count = -count
			}
			return count, true, nil
		case 2:
			if step.Args[0] < 0 || step.Args[1] < 0 {
				return 0, true, fmt.Errorf("sublist expects non-negative start and count")
			}
			_, count := boundedStartCount(length, step.Args[0], step.Args[1])
			return count, true, nil
		default:
			return 0, true, fmt.Errorf("sublist expects count or start count")
		}
	default:
		return 0, false, nil
	}
}

func sequenceTransformSourceRow(row int, steps []SequenceTransformStep, lengths [9]int) (int, bool, error) {
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		prevLen := lengths[i]
		if prevLen == 0 {
			return 0, false, nil
		}
		switch step.Transform {
		case SequenceTransformReverse:
			row = prevLen - 1 - row
		case SequenceTransformRotate:
			shift := step.Args[0] % prevLen
			if shift < 0 {
				shift += prevLen
			}
			row = (shift + row) % prevLen
		case SequenceTransformSublist:
			switch step.ArgCount {
			case 1:
				start := 0
				if step.Args[0] < 0 {
					count := -step.Args[0]
					start = prevLen - count%prevLen
					if start == prevLen {
						start = 0
					}
				}
				row = (start + row) % prevLen
			case 2:
				start, _ := boundedStartCount(prevLen, step.Args[0], step.Args[1])
				row = start + row
			default:
				return 0, false, fmt.Errorf("sublist expects count or start count")
			}
		default:
			return 0, false, nil
		}
	}
	return row, true, nil
}

// FlattenNestedArray returns a lazy raze view for matrices and nested arrays.
// The view preserves list semantics while avoiding flat []any allocation on hot
// reduce/count paths.
func FlattenNestedArray(value any) (Array, bool, error) {
	switch x := value.(type) {
	case Matrix:
		if m, ok := x.(matrixArray); ok {
			return m.data, true, nil
		}
		count, _, err := RazeCount(x)
		if err != nil {
			return nil, true, err
		}
		return flattenArray{source: x, len: int(count), kind: matrixElementKind(x)}, true, nil
	case Array:
		count, ok, err := RazeCount(x)
		if err != nil || !ok {
			return nil, ok, err
		}
		return flattenArray{source: x, len: int(count), kind: nestedElementKind(x)}, true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedNestedNumericSum reduces raze value directly. It recognizes matrix
// views and lists of numeric arrays without materializing the flattened array.
func TryTypedNestedNumericSum(value any) (any, bool, error) {
	sum, handled, err := nestedNumericSum(value)
	if err != nil || !handled {
		return nil, handled, err
	}
	if sum.hasFloat {
		return sum.float, true, nil
	}
	return sum.integer, true, nil
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
			start, end = boundedSegment(x.Len(), start, end)
			part, err := Slice(x, start, end-start)
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

func TakeRepeatCount(n int, value any) int64 {
	if n == 0 {
		return 0
	}
	switch x := value.(type) {
	case Array:
		if x.Len() == 0 {
			return 0
		}
	case Frame:
		if x.Len() == 0 {
			return 0
		}
	case string:
		if len([]rune(x)) == 0 {
			return 0
		}
	}
	if n < 0 {
		n = -n
	}
	return int64(n)
}

func SegmentIndexes(length, start, end int) []int {
	start, end = boundedSegment(length, start, end)
	indexes := make([]int, end-start)
	for i := range indexes {
		indexes[i] = start + i
	}
	return indexes
}

func boundedSegment(length, start, end int) (int, int) {
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
	return start, end
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

type flattenArray struct {
	source Array
	len    int
	kind   Kind
}

type nestedSum struct {
	integer  int64
	float    float64
	hasFloat bool
}

func (a flattenArray) Kind() Kind { return a.kind }

func (a flattenArray) Len() int { return a.len }

func (a flattenArray) At(index int) (any, bool) {
	if index < 0 || index >= a.len {
		return nil, false
	}
	if matrix, ok := a.source.(Matrix); ok {
		shape := matrix.Shape()
		if len(shape) != 2 || shape[1] == 0 {
			return nil, false
		}
		return matrix.Cell(index/shape[1], index%shape[1])
	}
	offset := 0
	for row := 0; row < a.source.Len(); row++ {
		item, ok := a.source.At(row)
		if !ok {
			return nil, false
		}
		if array, ok := item.(Array); ok {
			next := offset + array.Len()
			if index < next {
				return array.At(index - offset)
			}
			offset = next
			continue
		}
		if index == offset {
			return item, true
		}
		offset++
	}
	return nil, false
}

func (a flattenArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data flatten row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a flattenArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, index := range indexes {
		value, ok := a.At(index)
		if !ok {
			panic(fmt.Sprintf("data flatten gather row %d out of range", index))
		}
		out[i] = value
	}
	return InferArray(out)
}

func nestedElementKind(array Array) Kind {
	kind := KindNull
	for row := 0; row < array.Len(); row++ {
		item, ok := array.At(row)
		if !ok {
			return KindAny
		}
		itemKind := KindAny
		if child, ok := item.(Array); ok {
			itemKind = child.Kind()
		} else {
			itemKind = scalarSequenceKind(item)
		}
		if itemKind == KindNull {
			continue
		}
		if kind == KindNull {
			kind = itemKind
			continue
		}
		if kind != itemKind {
			return KindAny
		}
	}
	if kind == KindNull {
		return KindAny
	}
	return kind
}

func scalarSequenceKind(value any) Kind {
	switch value.(type) {
	case bool:
		return KindBool
	case int8:
		return KindI8
	case int16:
		return KindI16
	case int32:
		return KindI32
	case int, int64:
		return KindI64
	case uint8:
		return KindU8
	case uint16:
		return KindU16
	case uint32:
		return KindU32
	case uint, uint64:
		return KindU64
	case float32:
		return KindF32
	case float64:
		return KindF64
	case string:
		return KindString
	case Symbol:
		return KindSymbol
	case Month:
		return KindMonth
	case Date:
		return KindDate
	case DateTime:
		return KindDateTime
	case Timestamp:
		return KindTimestamp
	case Timespan:
		return KindTimespan
	case Minute:
		return KindMinute
	case Second:
		return KindSecond
	case Time:
		return KindTime
	default:
		if IsNull(value) {
			return KindNull
		}
		return KindAny
	}
}

func matrixElementKind(matrix Matrix) Kind {
	if row, ok := matrix.RowArray(0); ok {
		return row.Kind()
	}
	return KindAny
}

func nestedNumericSum(value any) (nestedSum, bool, error) {
	switch x := value.(type) {
	case Matrix:
		if source, ok := x.(interface{ SourceMatrix() Matrix }); ok {
			return nestedNumericSum(source.SourceMatrix())
		}
		if m, ok := x.(matrixArray); ok {
			return nestedNumericArraySum(m.data)
		}
		return nestedNumericMatrixSum(x)
	case Array:
		if x.Kind() != KindAny && x.Kind() != KindNull {
			return nestedNumericArraySum(x)
		}
		var total nestedSum
		for row := 0; row < x.Len(); row++ {
			item, ok := x.At(row)
			if !ok {
				return nestedSum{}, true, fmt.Errorf("nested sum row %d out of range", row)
			}
			part, handled, err := nestedNumericSum(item)
			if err != nil || !handled {
				return nestedSum{}, handled, err
			}
			total.add(part)
		}
		return total, true, nil
	default:
		if IsNull(value) {
			return nestedSum{}, true, nil
		}
		if n, ok := integerValue(value); ok {
			return nestedSum{integer: n, float: float64(n)}, true, nil
		}
		if n, ok := numeric(value); ok {
			return nestedSum{float: n, hasFloat: true}, true, nil
		}
		return nestedSum{}, false, nil
	}
}

func nestedNumericArraySum(array Array) (nestedSum, bool, error) {
	out, handled, err := TryTypedNumericSum(array)
	if err != nil || !handled {
		return nestedSum{}, handled, err
	}
	switch value := out.(type) {
	case int64:
		return nestedSum{integer: value, float: float64(value)}, true, nil
	case int32:
		n := int64(value)
		return nestedSum{integer: n, float: float64(n)}, true, nil
	case int16:
		n := int64(value)
		return nestedSum{integer: n, float: float64(n)}, true, nil
	case int8:
		n := int64(value)
		return nestedSum{integer: n, float: float64(n)}, true, nil
	case float32:
		return nestedSum{float: float64(value), hasFloat: true}, true, nil
	case float64:
		return nestedSum{float: value, hasFloat: true}, true, nil
	default:
		if IsNull(value) {
			return nestedSum{}, true, nil
		}
		if n, ok := integerValue(value); ok {
			return nestedSum{integer: n, float: float64(n)}, true, nil
		}
		if n, ok := numeric(value); ok {
			return nestedSum{float: n, hasFloat: true}, true, nil
		}
		return nestedSum{}, false, nil
	}
}

func nestedNumericMatrixSum(matrix Matrix) (nestedSum, bool, error) {
	shape := matrix.Shape()
	if len(shape) != 2 {
		return nestedSum{}, true, fmt.Errorf("nested sum expects a two-dimensional matrix")
	}
	var total nestedSum
	for row := 0; row < shape[0]; row++ {
		for col := 0; col < shape[1]; col++ {
			value, ok := matrix.Cell(row, col)
			if !ok {
				return nestedSum{}, true, fmt.Errorf("matrix cell %d,%d out of range", row, col)
			}
			part, handled, err := nestedNumericSum(value)
			if err != nil || !handled {
				return nestedSum{}, handled, err
			}
			total.add(part)
		}
	}
	return total, true, nil
}

func (s *nestedSum) add(other nestedSum) {
	s.integer += other.integer
	s.float += other.float
	s.hasFloat = s.hasFloat || other.hasFloat
}
