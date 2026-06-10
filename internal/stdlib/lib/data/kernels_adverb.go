package data

// Typed adverb kernels.
//
// q adverbs (each-prior `':`, over `/`, scan `\`) historically applied their
// dyadic verb one boxed pair at a time through the generic scalar evaluator,
// paying interface dispatch plus []any boxing per row. The kernels below run
// the same pairwise / left-fold loops over bulk-materialized []int64 or
// []float64 values in one tight pass and return dense columnar output.
//
// Semantics mirror the boxed path exactly:
//   - i64 kernels accept only KindI64 sources with integer-typed seeds and use
//     wrapping Go int64 arithmetic, like the scalar evaluator.
//   - f64 kernels accept only KindF64 sources and fold in source order, so
//     float rounding matches the sequential boxed evaluation bit-for-bit.
//   - Bulk materialization (tryBulkI64Values / tryBulkF64Values) bails out on
//     null rows, so null propagation always reaches the boxed fallback.
//   - Empty vectors are not handled, keeping empty-result kind inference in
//     the caller unchanged.

func adverbI64DyadicSupported(op Op) bool {
	switch op {
	case OpAdd, OpSub, OpMul:
		return true
	default:
		return false
	}
}

func adverbF64DyadicSupported(op Op) bool {
	switch op {
	case OpAdd, OpSub, OpMul, OpDiv:
		return true
	default:
		return false
	}
}

// adverbIntegerSeed mirrors the q scalar evaluator's integerValue: only
// signed Go integers seed an integer fold; floats and other values fall back
// so mixed-kind promotion keeps its boxed semantics.
func adverbIntegerSeed(initial any) (int64, bool) {
	switch n := initial.(type) {
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}

// adverbFloatSeed mirrors the q scalar evaluator's numeric coercion.
func adverbFloatSeed(initial any) (float64, bool) {
	switch n := initial.(type) {
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func applyAdverbI64Dyadic(op Op, left, right int64) int64 {
	switch op {
	case OpAdd:
		return left + right
	case OpSub:
		return left - right
	case OpMul:
		return left * right
	default:
		return 0
	}
}

func applyAdverbF64Dyadic(op Op, left, right float64) float64 {
	switch op {
	case OpAdd:
		return left + right
	case OpSub:
		return left - right
	case OpMul:
		return left * right
	case OpDiv:
		return left / right
	default:
		return 0
	}
}

// TryTypedEachPriorDyadic applies a dyadic verb-prior scan: out[i] is
// op(v[i], v[i-1]) with q argument order (current item on the left). A nil
// initial preserves the first element unchanged; a non-nil initial seeds
// out[0] = op(v[0], initial).
func TryTypedEachPriorDyadic(op Op, initial any, array Array) (Array, bool, error) {
	n := array.Len()
	if n == 0 {
		return nil, false, nil
	}
	switch array.Kind() {
	case KindI64:
		if !adverbI64DyadicSupported(op) {
			return nil, false, nil
		}
		var seed int64
		hasSeed := initial != nil
		if hasSeed {
			value, ok := adverbIntegerSeed(initial)
			if !ok {
				return nil, false, nil
			}
			seed = value
		}
		values, owned, ok := tryBulkI64Values(array)
		if !ok || len(values) < n {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		values = values[:n]
		out := make([]int64, n)
		if hasSeed {
			out[0] = applyAdverbI64Dyadic(op, values[0], seed)
		} else {
			out[0] = values[0]
		}
		switch op {
		case OpAdd:
			for i := 1; i < n; i++ {
				out[i] = values[i] + values[i-1]
			}
		case OpSub:
			for i := 1; i < n; i++ {
				out[i] = values[i] - values[i-1]
			}
		case OpMul:
			for i := 1; i < n; i++ {
				out[i] = values[i] * values[i-1]
			}
		}
		bulkI64Release(values, owned)
		return newI64Trusted(out), true, nil
	case KindF64:
		if !adverbF64DyadicSupported(op) {
			return nil, false, nil
		}
		var seed float64
		hasSeed := initial != nil
		if hasSeed {
			value, ok := adverbFloatSeed(initial)
			if !ok {
				return nil, false, nil
			}
			seed = value
		}
		values, owned, ok := tryBulkF64Values(array)
		if !ok || len(values) < n {
			bulkF64Release(values, owned)
			return nil, false, nil
		}
		values = values[:n]
		out := make([]float64, n)
		if hasSeed {
			out[0] = applyAdverbF64Dyadic(op, values[0], seed)
		} else {
			out[0] = values[0]
		}
		switch op {
		case OpAdd:
			for i := 1; i < n; i++ {
				out[i] = values[i] + values[i-1]
			}
		case OpSub:
			for i := 1; i < n; i++ {
				out[i] = values[i] - values[i-1]
			}
		case OpMul:
			for i := 1; i < n; i++ {
				out[i] = values[i] * values[i-1]
			}
		case OpDiv:
			for i := 1; i < n; i++ {
				out[i] = values[i] / values[i-1]
			}
		}
		bulkF64Release(values, owned)
		return newF64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedOverDyadic left-folds a dyadic verb over a numeric vector with an
// optional initial accumulator: acc = op(acc, v[i]) in source order, exactly
// like the boxed over loop.
func TryTypedOverDyadic(op Op, initial any, array Array) (any, bool, error) {
	n := array.Len()
	if n == 0 {
		return nil, false, nil
	}
	switch array.Kind() {
	case KindI64:
		if !adverbI64DyadicSupported(op) {
			return nil, false, nil
		}
		var seed int64
		hasSeed := initial != nil
		if hasSeed {
			value, ok := adverbIntegerSeed(initial)
			if !ok {
				return nil, false, nil
			}
			seed = value
		}
		values, owned, ok := tryBulkI64Values(array)
		if !ok || len(values) < n {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		values = values[:n]
		acc := seed
		start := 0
		if !hasSeed {
			acc = values[0]
			start = 1
		}
		switch op {
		case OpAdd:
			for _, v := range values[start:] {
				acc += v
			}
		case OpSub:
			for _, v := range values[start:] {
				acc -= v
			}
		case OpMul:
			for _, v := range values[start:] {
				acc *= v
			}
		}
		bulkI64Release(values, owned)
		return acc, true, nil
	case KindF64:
		if !adverbF64DyadicSupported(op) {
			return nil, false, nil
		}
		var seed float64
		hasSeed := initial != nil
		if hasSeed {
			value, ok := adverbFloatSeed(initial)
			if !ok {
				return nil, false, nil
			}
			seed = value
		}
		values, owned, ok := tryBulkF64Values(array)
		if !ok || len(values) < n {
			bulkF64Release(values, owned)
			return nil, false, nil
		}
		values = values[:n]
		acc := seed
		start := 0
		if !hasSeed {
			acc = values[0]
			start = 1
		}
		switch op {
		case OpAdd:
			for _, v := range values[start:] {
				acc += v
			}
		case OpSub:
			for _, v := range values[start:] {
				acc -= v
			}
		case OpMul:
			for _, v := range values[start:] {
				acc *= v
			}
		case OpDiv:
			for _, v := range values[start:] {
				acc /= v
			}
		}
		bulkF64Release(values, owned)
		return acc, true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedScanDyadic is TryTypedOverDyadic with running output: out[i] holds
// the accumulator after consuming v[i].
func TryTypedScanDyadic(op Op, initial any, array Array) (Array, bool, error) {
	n := array.Len()
	if n == 0 {
		return nil, false, nil
	}
	switch array.Kind() {
	case KindI64:
		if !adverbI64DyadicSupported(op) {
			return nil, false, nil
		}
		var seed int64
		hasSeed := initial != nil
		if hasSeed {
			value, ok := adverbIntegerSeed(initial)
			if !ok {
				return nil, false, nil
			}
			seed = value
		}
		values, owned, ok := tryBulkI64Values(array)
		if !ok || len(values) < n {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		values = values[:n]
		out := make([]int64, n)
		acc := seed
		start := 0
		if !hasSeed {
			acc = values[0]
			out[0] = acc
			start = 1
		}
		switch op {
		case OpAdd:
			for i := start; i < n; i++ {
				acc += values[i]
				out[i] = acc
			}
		case OpSub:
			for i := start; i < n; i++ {
				acc -= values[i]
				out[i] = acc
			}
		case OpMul:
			for i := start; i < n; i++ {
				acc *= values[i]
				out[i] = acc
			}
		}
		bulkI64Release(values, owned)
		return newI64Trusted(out), true, nil
	case KindF64:
		if !adverbF64DyadicSupported(op) {
			return nil, false, nil
		}
		var seed float64
		hasSeed := initial != nil
		if hasSeed {
			value, ok := adverbFloatSeed(initial)
			if !ok {
				return nil, false, nil
			}
			seed = value
		}
		values, owned, ok := tryBulkF64Values(array)
		if !ok || len(values) < n {
			bulkF64Release(values, owned)
			return nil, false, nil
		}
		values = values[:n]
		out := make([]float64, n)
		acc := seed
		start := 0
		if !hasSeed {
			acc = values[0]
			out[0] = acc
			start = 1
		}
		switch op {
		case OpAdd:
			for i := start; i < n; i++ {
				acc += values[i]
				out[i] = acc
			}
		case OpSub:
			for i := start; i < n; i++ {
				acc -= values[i]
				out[i] = acc
			}
		case OpMul:
			for i := start; i < n; i++ {
				acc *= values[i]
				out[i] = acc
			}
		case OpDiv:
			for i := start; i < n; i++ {
				acc /= values[i]
				out[i] = acc
			}
		}
		bulkF64Release(values, owned)
		return newF64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}
