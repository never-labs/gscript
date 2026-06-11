package data

import "math"

// Fused monad-chain sum kernel.
//
// +/op_k ... op_1 [deltas] x evaluates a vertical chain of numeric monads
// capped by a sum. The staged path materializes one intermediate vector per
// stage (deltas, abs, sqrt, ...) and pays a full source pass plus allocator
// traffic per stage. The fused kernel flattens the source exactly once and
// applies the whole chain per element into scalar accumulators.
//
// Exactness contract: every stage kernel this replaces is elementwise
// (TryTypedDeltas / each-prior subtraction, qNumericUnaryIntegerArray,
// qNumericUnaryFloatSlice) and the final sum accumulates in row order, so
// applying the same per-element expressions in the same order is
// bit-identical. Domain transitions mirror the staged kernels exactly:
//   - integer domain: neg/abs decline on minInt64 (the staged integer
//     kernels decline those inputs), signum maps to -1/0/1, floor/ceiling
//     are identity;
//   - the first float-producing op converts with float64(v) like the staged
//     integer kernels;
//   - float-domain signum/floor/ceiling return to the integer domain with
//     int64(math.Floor/Ceil) exactly like qNumericUnaryFloatSlice;
//   - the final accumulator domain (int64 vs float64) matches the staged
//     TryTypedQNumericUnarySum route for the outermost op.
//
// Restrictions that keep the staged fallback semantics: deltas roots only
// fuse for KindI64 sources (narrow integer deltas wrap at the narrow width
// in the staged kernel) and float sources only fuse for KindF64 (staged
// float32 chains round per stage). Anything else declines.

// QNumericUnaryChainSumPlan caches opcode resolution for a fused
// monad-chain sum. Ops are innermost-first (application order).
type QNumericUnaryChainSumPlan struct {
	deltas bool
	codes  []monadSumOpcode
}

// NewQNumericUnaryChainSumPlan resolves ops (innermost-first); ok=false when
// the chain is trivial (a single op without a deltas root is the existing
// single-op kernel's domain) or any op is unsupported.
func NewQNumericUnaryChainSumPlan(ops []string, deltas bool) (*QNumericUnaryChainSumPlan, bool) {
	if len(ops) == 0 || (len(ops) == 1 && !deltas) {
		return nil, false
	}
	codes := make([]monadSumOpcode, len(ops))
	for i, op := range ops {
		codes[i] = monadSumOpcodeFor(op)
		if codes[i] == monadSumInvalid {
			return nil, false
		}
	}
	return &QNumericUnaryChainSumPlan{deltas: deltas, codes: codes}, true
}

// TryTypedQNumericUnaryChainSum is the unplanned entry point.
func TryTypedQNumericUnaryChainSum(ops []string, deltas bool, array Array) (any, bool, error) {
	plan, ok := NewQNumericUnaryChainSumPlan(ops, deltas)
	if !ok {
		return nil, false, nil
	}
	return TryTypedQNumericUnaryChainSumPlanned(plan, array)
}

// TryTypedQNumericUnaryChainSumPlanned computes the fused chain sum.
// handled=false keeps the caller on the staged per-stage route.
func TryTypedQNumericUnaryChainSumPlanned(plan *QNumericUnaryChainSumPlan, array Array) (any, bool, error) {
	if plan == nil || array == nil {
		return nil, false, nil
	}
	array = unwrapAttributedArray(array)
	if array.Len() == 0 {
		return nil, false, nil
	}
	switch array.Kind() {
	case KindF64:
		values, owned, ok := tryBulkF64Values(array)
		if !ok {
			return nil, false, nil
		}
		out, handled := monadChainSumF64(plan.codes, plan.deltas, values)
		bulkF64Release(values, owned)
		return out, handled, nil
	case KindI64:
		values, owned, ok := tryBulkI64Values(array)
		if !ok || len(values) != array.Len() {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		out, handled := monadChainSumI64(plan.codes, plan.deltas, values)
		bulkI64Release(values, owned)
		return out, handled, nil
	case KindI8, KindI16, KindI32:
		if plan.deltas {
			// Narrow deltas wrap at the narrow width in the staged kernel.
			return nil, false, nil
		}
		values, owned, ok := tryBulkI64Values(array)
		if !ok || len(values) != array.Len() {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		out, handled := monadChainSumI64(plan.codes, plan.deltas, values)
		bulkI64Release(values, owned)
		return out, handled, nil
	}
	return nil, false, nil
}

// TryTypedIntCastChainSum reduces +/`k_n$...`k_1$x for integer cast chains
// without materializing the intermediate cast columns. kinds are
// innermost-first. Per element the value is range-checked and truncated per
// cast exactly like TryTypedCast's per-kind loops (int64(int16(v)), ...),
// and the final sum accumulates wrapping int64 in row order like the staged
// integer sum. Out-of-range values decline (handled=false) so the staged
// route raises its own cast error unchanged.
func TryTypedIntCastChainSum(kinds []Kind, array Array) (any, bool) {
	if len(kinds) < 2 || array == nil {
		return nil, false
	}
	for _, kind := range kinds {
		switch kind {
		case KindI16, KindI32, KindI64:
		default:
			return nil, false
		}
	}
	array = unwrapAttributedArray(array)
	switch array.Kind() {
	case KindI64, KindI32, KindI16, KindI8:
	default:
		return nil, false
	}
	values, owned, ok := tryBulkI64Values(array)
	if !ok || len(values) != array.Len() || len(values) == 0 {
		bulkI64Release(values, owned)
		return nil, false
	}
	var sum int64
	// Hot pair shapes run unrolled loops; anything else takes the generic
	// per-op walk below (same per-element expressions either way).
	if len(kinds) == 2 && kinds[1] == KindI64 {
		switch kinds[0] {
		case KindI32:
			for _, v := range values {
				if v < -2147483648 || v > 2147483647 {
					bulkI64Release(values, owned)
					return nil, false
				}
				sum += int64(int32(v))
			}
			bulkI64Release(values, owned)
			return sum, true
		case KindI16:
			for _, v := range values {
				if v < -32768 || v > 32767 {
					bulkI64Release(values, owned)
					return nil, false
				}
				sum += int64(int16(v))
			}
			bulkI64Release(values, owned)
			return sum, true
		}
	}
	for _, v := range values {
		for _, kind := range kinds {
			switch kind {
			case KindI16:
				if v < -32768 || v > 32767 {
					bulkI64Release(values, owned)
					return nil, false
				}
				v = int64(int16(v))
			case KindI32:
				if v < -2147483648 || v > 2147483647 {
					bulkI64Release(values, owned)
					return nil, false
				}
				v = int64(int32(v))
			case KindI64:
				// identity
			}
		}
		sum += v
	}
	bulkI64Release(values, owned)
	return sum, true
}

// monadChainIntDomainOp reports whether the op keeps an int64 element in the
// integer domain (mirrors qNumericUnaryIntegerArray's integer-result ops).
func monadChainIntDomainOp(code monadSumOpcode) bool {
	switch code {
	case monadSumNeg, monadSumAbs, monadSumSignum, monadSumFloor, monadSumCeiling:
		return true
	default:
		return false
	}
}

// monadChainFloatOp mirrors applyNumericUnaryFloat for the float-result ops.
func monadChainFloatOp(code monadSumOpcode, v float64) float64 {
	switch code {
	case monadSumNeg:
		return -v
	case monadSumAbs:
		return math.Abs(v)
	case monadSumSqrt:
		return math.Sqrt(v)
	case monadSumLog:
		return math.Log(v)
	case monadSumExp:
		return math.Exp(v)
	case monadSumSin:
		return math.Sin(v)
	case monadSumCos:
		return math.Cos(v)
	case monadSumTan:
		return math.Tan(v)
	case monadSumAsin:
		return math.Asin(v)
	case monadSumAcos:
		return math.Acos(v)
	case monadSumAtan:
		return math.Atan(v)
	case monadSumRecip:
		return 1 / v
	}
	return v
}

// monadChainSumI64 dispatches an integer-source chain: monomorphized fused
// loops for the common "0-1 integer ops, one float conversion op, optional
// trailing floor/ceiling/signum" shapes, the generic automaton otherwise.
func monadChainSumI64(codes []monadSumOpcode, deltas bool, values []int64) (any, bool) {
	split := 0
	for split < len(codes) && monadChainIntDomainOp(codes[split]) {
		split++
	}
	intOps := codes[:split]
	if split == len(codes) {
		return monadChainSumI64Pure(intOps, deltas, values)
	}
	if split <= 1 {
		var intOp monadSumOpcode
		if split == 1 {
			intOp = intOps[0]
			if intOp == monadSumFloor || intOp == monadSumCeiling {
				intOp = monadSumInvalid // identity on integers
			}
		}
		conv := codes[split]
		rest := codes[split+1:]
		fin := monadSumInvalid
		fused := len(rest) == 0
		if len(rest) == 1 {
			switch rest[0] {
			case monadSumFloor, monadSumCeiling, monadSumSignum:
				fin = rest[0]
				fused = true
			}
		}
		if fused {
			if out, handled, ok := monadChainFusedI64(values, deltas, intOp, conv, fin); ok {
				return out, handled
			}
		}
	}
	return monadChainSumI64Generic(codes, deltas, values)
}

// monadChainFusedI64 runs hand-fused loops for the hot chain shapes
// (optional abs prefix, sqrt/log conversion, optional floor cap). Returns
// ok=false when no fused loop covers the shape, leaving the generic
// automaton to handle it. Loops accumulate per element in row order with
// exactly the staged kernels' expressions, so results stay bit-identical.
func monadChainFusedI64(values []int64, deltas bool, intOp, conv, fin monadSumOpcode) (any, bool, bool) {
	const minInt64 = -1 << 63
	switch {
	case intOp == monadSumAbs && conv == monadSumSqrt && fin == monadSumFloor:
		var sum int64
		var prev int64
		for i, v := range values {
			x := v
			if deltas {
				if i > 0 {
					x = v - prev
				}
				prev = v
			}
			if x == minInt64 {
				return nil, false, true
			}
			if x < 0 {
				x = -x
			}
			sum += int64(math.Floor(math.Sqrt(float64(x))))
		}
		return sum, true, true
	case intOp == monadSumAbs && conv == monadSumSqrt && fin == monadSumInvalid:
		var sum float64
		var prev int64
		for i, v := range values {
			x := v
			if deltas {
				if i > 0 {
					x = v - prev
				}
				prev = v
			}
			if x == minInt64 {
				return nil, false, true
			}
			if x < 0 {
				x = -x
			}
			sum += math.Sqrt(float64(x))
		}
		return sum, true, true
	case intOp == monadSumInvalid && conv == monadSumSqrt && fin == monadSumFloor:
		var sum int64
		var prev int64
		for i, v := range values {
			x := v
			if deltas {
				if i > 0 {
					x = v - prev
				}
				prev = v
			}
			sum += int64(math.Floor(math.Sqrt(float64(x))))
		}
		return sum, true, true
	case intOp == monadSumInvalid && conv == monadSumSqrt && fin == monadSumInvalid:
		var sum float64
		var prev int64
		for i, v := range values {
			x := v
			if deltas {
				if i > 0 {
					x = v - prev
				}
				prev = v
			}
			sum += math.Sqrt(float64(x))
		}
		return sum, true, true
	case intOp == monadSumInvalid && conv == monadSumLog && fin == monadSumInvalid:
		var sum float64
		var prev int64
		for i, v := range values {
			x := v
			if deltas {
				if i > 0 {
					x = v - prev
				}
				prev = v
			}
			sum += math.Log(float64(x))
		}
		return sum, true, true
	case intOp == monadSumAbs && conv == monadSumLog && fin == monadSumInvalid:
		var sum float64
		var prev int64
		for i, v := range values {
			x := v
			if deltas {
				if i > 0 {
					x = v - prev
				}
				prev = v
			}
			if x == minInt64 {
				return nil, false, true
			}
			if x < 0 {
				x = -x
			}
			sum += math.Log(float64(x))
		}
		return sum, true, true
	}
	return nil, false, false
}

// monadChainApplyIntOps applies a multi-op integer-domain prefix; ok=false
// declines on minInt64 exactly like the staged integer kernels.
func monadChainApplyIntOps(intOps []monadSumOpcode, x int64) (int64, bool) {
	const minInt64 = -1 << 63
	for _, op := range intOps {
		switch op {
		case monadSumNeg:
			if x == minInt64 {
				return 0, false
			}
			x = -x
		case monadSumAbs:
			if x == minInt64 {
				return 0, false
			}
			if x < 0 {
				x = -x
			}
		case monadSumSignum:
			switch {
			case x < 0:
				x = -1
			case x > 0:
				x = 1
			default:
				x = 0
			}
		default:
			// floor/ceiling: identity on integers.
		}
	}
	return x, true
}

// monadChainSumI64Pure handles chains that never leave the integer domain.
// The accumulator wraps like the staged integer sum kernels. Single-op
// chains run hand-fused loops; longer prefixes take the generic op walk.
func monadChainSumI64Pure(intOps []monadSumOpcode, deltas bool, values []int64) (any, bool) {
	const minInt64 = -1 << 63
	var sum int64
	var prev int64
	if len(intOps) == 1 {
		switch intOps[0] {
		case monadSumAbs:
			for i, v := range values {
				x := v
				if deltas {
					if i > 0 {
						x = v - prev
					}
					prev = v
				}
				if x == minInt64 {
					return nil, false
				}
				if x < 0 {
					x = -x
				}
				sum += x
			}
			return sum, true
		case monadSumNeg:
			for i, v := range values {
				x := v
				if deltas {
					if i > 0 {
						x = v - prev
					}
					prev = v
				}
				if x == minInt64 {
					return nil, false
				}
				sum -= x
			}
			return sum, true
		case monadSumSignum:
			for i, v := range values {
				x := v
				if deltas {
					if i > 0 {
						x = v - prev
					}
					prev = v
				}
				switch {
				case x < 0:
					sum--
				case x > 0:
					sum++
				}
			}
			return sum, true
		}
	}
	for i, v := range values {
		x := v
		if deltas {
			if i > 0 {
				x = v - prev
			}
			prev = v
		}
		x, ok := monadChainApplyIntOps(intOps, x)
		if !ok {
			return nil, false
		}
		sum += x
	}
	return sum, true
}

// monadChainSumI64Generic is the full dual-domain automaton for chains the
// fused loops do not cover. Per-element semantics are identical to the
// staged kernels (see file header).
func monadChainSumI64Generic(codes []monadSumOpcode, deltas bool, values []int64) (any, bool) {
	const minInt64 = -1 << 63
	var isum int64
	var fsum float64
	finalInt := true
	var prev int64
	for i, v := range values {
		x := v
		if deltas {
			if i > 0 {
				x = v - prev
			}
			prev = v
		}
		iv := x
		var fv float64
		isInt := true
		for _, c := range codes {
			if isInt {
				switch c {
				case monadSumNeg:
					if iv == minInt64 {
						return nil, false
					}
					iv = -iv
				case monadSumAbs:
					if iv == minInt64 {
						return nil, false
					}
					if iv < 0 {
						iv = -iv
					}
				case monadSumSignum:
					switch {
					case iv < 0:
						iv = -1
					case iv > 0:
						iv = 1
					default:
						iv = 0
					}
				case monadSumFloor, monadSumCeiling:
					// identity on integers
				default:
					fv = monadChainFloatOp(c, float64(iv))
					isInt = false
				}
			} else {
				switch c {
				case monadSumSignum:
					switch {
					case fv < 0:
						iv = -1
					case fv > 0:
						iv = 1
					default:
						iv = 0
					}
					isInt = true
				case monadSumFloor:
					iv = int64(math.Floor(fv))
					isInt = true
				case monadSumCeiling:
					iv = int64(math.Ceil(fv))
					isInt = true
				default:
					fv = monadChainFloatOp(c, fv)
				}
			}
		}
		if isInt {
			isum += iv
		} else {
			fsum += fv
		}
		finalInt = isInt
	}
	if finalInt {
		return isum, true
	}
	return fsum, true
}

// monadChainSumF64 runs the dual-domain automaton over a float64 source.
// Float-domain op semantics mirror qNumericUnaryFloatSlice; integer-domain
// segments (after a floor/ceiling/signum) mirror qNumericUnaryIntegerArray.
func monadChainSumF64(codes []monadSumOpcode, deltas bool, values []float64) (any, bool) {
	const minInt64 = -1 << 63
	var isum int64
	var fsum float64
	finalInt := false
	var prev float64
	for i, v := range values {
		fv := v
		if deltas {
			if i > 0 {
				fv = v - prev
			}
			prev = v
		}
		var iv int64
		isInt := false
		for _, c := range codes {
			if isInt {
				switch c {
				case monadSumNeg:
					if iv == minInt64 {
						return nil, false
					}
					iv = -iv
				case monadSumAbs:
					if iv == minInt64 {
						return nil, false
					}
					if iv < 0 {
						iv = -iv
					}
				case monadSumSignum:
					switch {
					case iv < 0:
						iv = -1
					case iv > 0:
						iv = 1
					default:
						iv = 0
					}
				case monadSumFloor, monadSumCeiling:
					// identity on integers
				default:
					fv = monadChainFloatOp(c, float64(iv))
					isInt = false
				}
			} else {
				switch c {
				case monadSumSignum:
					switch {
					case fv < 0:
						iv = -1
					case fv > 0:
						iv = 1
					default:
						iv = 0
					}
					isInt = true
				case monadSumFloor:
					iv = int64(math.Floor(fv))
					isInt = true
				case monadSumCeiling:
					iv = int64(math.Ceil(fv))
					isInt = true
				default:
					fv = monadChainFloatOp(c, fv)
				}
			}
		}
		if isInt {
			isum += iv
		} else {
			fsum += fv
		}
		finalInt = isInt
	}
	if finalInt {
		return isum, true
	}
	return fsum, true
}
