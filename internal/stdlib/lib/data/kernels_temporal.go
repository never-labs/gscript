package data

// Temporal dyadic kernels: kind-preserving + / - over the int64-backed
// temporal column carriers (date, month, minute, second, time, timestamp,
// timespan, datetime). q's temporal arithmetic is integer arithmetic on the
// underlying encoding, with a per-kind unit scale for plain-integer operands
// and a per-kind span kind for same-kind differences. The kernels here keep
// dense temporal columns typed (date-vector+1 stays a date column) instead of
// falling back to boxed row evaluation.

// IsTemporalKind reports whether kind is one of the int64-backed temporal
// kinds.
func IsTemporalKind(kind Kind) bool {
	switch kind {
	case KindMonth, KindDate, KindDateTime, KindTimespan,
		KindMinute, KindSecond, KindTime, KindTimestamp:
		return true
	default:
		return false
	}
}

// TemporalUnderlying returns the underlying int64 encoding and kind of a
// temporal scalar.
func TemporalUnderlying(v any) (int64, Kind, bool) {
	switch x := v.(type) {
	case Month:
		return int64(x), KindMonth, true
	case Date:
		return int64(x), KindDate, true
	case DateTime:
		return int64(x), KindDateTime, true
	case Timespan:
		return int64(x), KindTimespan, true
	case Minute:
		return int64(x), KindMinute, true
	case Second:
		return int64(x), KindSecond, true
	case Time:
		return int64(x), KindTime, true
	case Timestamp:
		return int64(x), KindTimestamp, true
	default:
		return 0, "", false
	}
}

// NewTemporalValue builds the temporal scalar of kind from its underlying
// int64 encoding.
func NewTemporalValue(kind Kind, n int64) (any, bool) {
	switch kind {
	case KindMonth:
		return Month(n), true
	case KindDate:
		return Date(n), true
	case KindDateTime:
		return DateTime(n), true
	case KindTimespan:
		return Timespan(n), true
	case KindMinute:
		return Minute(n), true
	case KindSecond:
		return Second(n), true
	case KindTime:
		return Time(n), true
	case KindTimestamp:
		return Timestamp(n), true
	default:
		return nil, false
	}
}

// TemporalIntUnitScale returns how many underlying units one plain-integer
// operand step represents in q temporal arithmetic: dates step in days,
// months in months, minutes/seconds in their own resolution, q times in
// milliseconds (the backing stores nanoseconds), timestamps and timespans in
// nanoseconds, and datetimes in days.
func TemporalIntUnitScale(kind Kind) (int64, bool) {
	switch kind {
	case KindMonth, KindDate, KindMinute, KindSecond, KindTimestamp, KindTimespan:
		return 1, true
	case KindTime:
		return 1_000_000, true
	case KindDateTime:
		return nanosPerDay, true
	default:
		return 0, false
	}
}

// TemporalSpanKind returns the result kind of a same-kind temporal
// difference: date and month differences are plain integers, instant and
// duration differences are timespans, and time-of-day differences keep their
// kind.
func TemporalSpanKind(kind Kind) (Kind, bool) {
	switch kind {
	case KindDate, KindMonth:
		return KindI64, true
	case KindTimestamp, KindTimespan, KindDateTime:
		return KindTimespan, true
	case KindMinute, KindSecond, KindTime:
		return kind, true
	default:
		return "", false
	}
}

// NewTemporalArrayFromInt64 builds a dense column of kind from underlying
// int64 encodings. KindI64 is accepted so same-kind difference results (date
// minus date) share the construction path.
func NewTemporalArrayFromInt64(kind Kind, values []int64) (Array, bool) {
	switch kind {
	case KindI64:
		return newI64Trusted(values), true
	case KindMonth:
		return temporalColumnFromInt64[Month](kind, values), true
	case KindDate:
		return temporalColumnFromInt64[Date](kind, values), true
	case KindDateTime:
		return temporalColumnFromInt64[DateTime](kind, values), true
	case KindTimespan:
		return temporalColumnFromInt64[Timespan](kind, values), true
	case KindMinute:
		return temporalColumnFromInt64[Minute](kind, values), true
	case KindSecond:
		return temporalColumnFromInt64[Second](kind, values), true
	case KindTime:
		return temporalColumnFromInt64[Time](kind, values), true
	case KindTimestamp:
		return temporalColumnFromInt64[Timestamp](kind, values), true
	default:
		return nil, false
	}
}

func temporalColumnFromInt64[T ~int64](kind Kind, values []int64) Array {
	out := make([]T, len(values))
	for i, v := range values {
		out[i] = T(v)
	}
	return columnArray[T]{kind: kind, data: out}
}

func temporalColumnInt64s[T ~int64](values []T) []int64 {
	out := make([]int64, len(values))
	for i, v := range values {
		out[i] = int64(v)
	}
	return out
}

// temporalArrayInt64Values extracts the underlying int64 encodings of a dense
// temporal array. Arrays containing nulls (boxed nullable storage) decline so
// callers keep their null-propagating fallback.
func temporalArrayInt64Values(array Array) ([]int64, bool) {
	switch a := array.(type) {
	case attributedArray:
		return temporalArrayInt64Values(a.array)
	case columnArray[Month]:
		return temporalColumnInt64s(a.data), true
	case columnArray[Date]:
		return temporalColumnInt64s(a.data), true
	case columnArray[DateTime]:
		return temporalColumnInt64s(a.data), true
	case columnArray[Timespan]:
		return temporalColumnInt64s(a.data), true
	case columnArray[Minute]:
		return temporalColumnInt64s(a.data), true
	case columnArray[Second]:
		return temporalColumnInt64s(a.data), true
	case columnArray[Time]:
		return temporalColumnInt64s(a.data), true
	case columnArray[Timestamp]:
		return temporalColumnInt64s(a.data), true
	default:
		// Lazy views (gather/tile/subarray wrappers) flatten by access.
		n := array.Len()
		out := make([]int64, n)
		for i := 0; i < n; i++ {
			v, ok := array.At(i)
			if !ok {
				return nil, false
			}
			u, _, ok := TemporalUnderlying(v)
			if !ok {
				return nil, false
			}
			out[i] = u
		}
		return out, true
	}
}

// denseIntegerInt64Values extracts int64 values from a dense integer array,
// declining on nulls so callers keep their null-propagating fallback.
func denseIntegerInt64Values(array Array) ([]int64, bool) {
	if !isIntegerArray(array) {
		return nil, false
	}
	if values, owned, ok := tryBulkI64Values(array); ok {
		out := make([]int64, len(values))
		copy(out, values)
		bulkI64Release(values, owned)
		return out, true
	}
	n := array.Len()
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		v, ok, err := integerArrayAt(array, i)
		if err != nil || !ok {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

// temporalDyadicOperand is one side of a temporal dyadic kernel call.
type temporalDyadicOperand struct {
	isArray  bool
	temporal bool
	kind     Kind
	values   []int64
	scalar   int64
}

func describeTemporalDyadicOperand(v any) (temporalDyadicOperand, bool) {
	if array, ok := v.(Array); ok {
		kind := array.Kind()
		if IsTemporalKind(kind) {
			values, ok := temporalArrayInt64Values(array)
			if !ok {
				return temporalDyadicOperand{}, false
			}
			return temporalDyadicOperand{isArray: true, temporal: true, kind: kind, values: values}, true
		}
		values, ok := denseIntegerInt64Values(array)
		if !ok {
			return temporalDyadicOperand{}, false
		}
		return temporalDyadicOperand{isArray: true, kind: kind, values: values}, true
	}
	if u, kind, ok := TemporalUnderlying(v); ok {
		return temporalDyadicOperand{temporal: true, kind: kind, scalar: u}, true
	}
	if n, ok := integerScalarValue(v); ok {
		return temporalDyadicOperand{scalar: n}, true
	}
	if b, ok := v.(bool); ok {
		n := int64(0)
		if b {
			n = 1
		}
		return temporalDyadicOperand{scalar: n}, true
	}
	return temporalDyadicOperand{}, false
}

func (o temporalDyadicOperand) at(i int) int64 {
	if o.isArray {
		return o.values[i]
	}
	return o.scalar
}

func (o temporalDyadicOperand) length() int {
	if o.isArray {
		return len(o.values)
	}
	return 1
}

// TryTypedTemporalDyadic evaluates + / - when at least one operand carries a
// temporal kind and at least one side is an array, producing a kind-preserving
// dense column. Supported shapes:
//
//	temporal ± integer      -> temporal kind (integer scaled per kind)
//	integer + temporal      -> temporal kind
//	temporal - temporal     -> span kind (same temporal kind on both sides)
//	tod/timespan + same     -> same kind (minute/second/time/timespan sums)
//
// Anything else (mixed temporal kinds, nulls, floats) declines so callers
// keep their scalar fallback semantics.
func TryTypedTemporalDyadic(op Op, left, right any) (Array, bool, error) {
	if op != OpAdd && op != OpSub {
		return nil, false, nil
	}
	l, lok := describeTemporalDyadicOperand(left)
	if !lok {
		return nil, false, nil
	}
	r, rok := describeTemporalDyadicOperand(right)
	if !rok {
		return nil, false, nil
	}
	if !l.isArray && !r.isArray {
		return nil, false, nil
	}
	if !l.temporal && !r.temporal {
		return nil, false, nil
	}
	if l.isArray && r.isArray && len(l.values) != len(r.values) {
		return nil, false, nil
	}
	length := l.length()
	if r.isArray {
		length = r.length()
	}
	switch {
	case l.temporal && !r.temporal:
		scale, ok := TemporalIntUnitScale(l.kind)
		if !ok {
			return nil, false, nil
		}
		out := make([]int64, length)
		if op == OpAdd {
			for i := range out {
				out[i] = l.at(i) + r.at(i)*scale
			}
		} else {
			for i := range out {
				out[i] = l.at(i) - r.at(i)*scale
			}
		}
		array, ok := NewTemporalArrayFromInt64(l.kind, out)
		return array, ok, nil
	case r.temporal && !l.temporal:
		if op != OpAdd {
			return nil, false, nil
		}
		scale, ok := TemporalIntUnitScale(r.kind)
		if !ok {
			return nil, false, nil
		}
		out := make([]int64, length)
		for i := range out {
			out[i] = r.at(i) + l.at(i)*scale
		}
		array, ok := NewTemporalArrayFromInt64(r.kind, out)
		return array, ok, nil
	default:
		if l.kind != r.kind {
			return nil, false, nil
		}
		resultKind := Kind("")
		if op == OpSub {
			span, ok := TemporalSpanKind(l.kind)
			if !ok {
				return nil, false, nil
			}
			resultKind = span
		} else {
			// Sums only keep meaning for duration-like kinds.
			switch l.kind {
			case KindMinute, KindSecond, KindTime, KindTimespan:
				resultKind = l.kind
			default:
				return nil, false, nil
			}
		}
		out := make([]int64, length)
		if op == OpAdd {
			for i := range out {
				out[i] = l.at(i) + r.at(i)
			}
		} else {
			for i := range out {
				out[i] = l.at(i) - r.at(i)
			}
		}
		array, ok := NewTemporalArrayFromInt64(resultKind, out)
		return array, ok, nil
	}
}
