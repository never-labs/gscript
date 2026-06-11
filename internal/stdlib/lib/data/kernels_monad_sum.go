package data

import "math"

// Fused multi-monad sum kernel.
//
// Add chains of the form (+/f x)+(+/g x)+... evaluate several numeric unary
// sums over the same source vector. Evaluating each term independently pays
// the lazy-carrier flatten (tryBulkF64Values) and a full source pass once
// per term. TryTypedQNumericUnaryMultiSum touches the source once:
//
//   - hot op combinations ({sqrt,log}, {floor,ceiling,reciprocal}) run as
//     hand-fused loops with scalar accumulators, streaming range carriers
//     without materializing them;
//   - any other combination flattens the source exactly once into a pooled
//     dense slice and reuses the single-op slice kernels per op.
//
// Exactness contract: every op accumulates the same per-element expression
// in the same element order as its single-op kernel
// (qNumericUnarySumFloatSlice / qNumericUnarySumI64Range), so fused results
// are bit-identical to the per-term evaluation they replace. Cross-op
// fusion cannot change results because each op keeps its own accumulator.

type monadSumOpcode uint8

const (
	monadSumInvalid monadSumOpcode = iota
	monadSumNeg
	monadSumAbs
	monadSumSqrt
	monadSumLog
	monadSumExp
	monadSumSin
	monadSumCos
	monadSumTan
	monadSumAsin
	monadSumAcos
	monadSumAtan
	monadSumRecip
	monadSumSignum
	monadSumFloor
	monadSumCeiling
)

func monadSumOpcodeFor(op string) monadSumOpcode {
	switch op {
	case NumericUnaryNeg:
		return monadSumNeg
	case NumericUnaryAbs:
		return monadSumAbs
	case NumericUnarySqrt:
		return monadSumSqrt
	case NumericUnaryLog:
		return monadSumLog
	case NumericUnaryExp:
		return monadSumExp
	case NumericUnarySin:
		return monadSumSin
	case NumericUnaryCos:
		return monadSumCos
	case NumericUnaryTan:
		return monadSumTan
	case NumericUnaryAsin:
		return monadSumAsin
	case NumericUnaryAcos:
		return monadSumAcos
	case NumericUnaryAtan:
		return monadSumAtan
	case NumericUnaryRecip:
		return monadSumRecip
	case NumericUnarySignum:
		return monadSumSignum
	case NumericUnaryFloor:
		return monadSumFloor
	case NumericUnaryCeiling:
		return monadSumCeiling
	default:
		return monadSumInvalid
	}
}

// monadSumIntAccumulator reports whether the op sums into an int64
// accumulator over float input (mirrors qNumericUnarySumFloatSlice).
func monadSumIntAccumulator(code monadSumOpcode) bool {
	switch code {
	case monadSumSignum, monadSumFloor, monadSumCeiling:
		return true
	default:
		return false
	}
}

// monadSumComboSlots returns, for each opcode in want, the index of its
// (single) occurrence in codes. ok=false when codes is not exactly the
// multiset want with distinct ops.
func monadSumComboSlots(codes []monadSumOpcode, want ...monadSumOpcode) ([]int, bool) {
	if len(codes) != len(want) {
		return nil, false
	}
	slots := make([]int, len(want))
	for w, target := range want {
		found := -1
		for i, code := range codes {
			if code == target {
				if found >= 0 {
					return nil, false
				}
				found = i
			}
		}
		if found < 0 {
			return nil, false
		}
		slots[w] = found
	}
	return slots, true
}

// QNumericUnaryMultiSumPlan caches the opcode resolution for a fused
// multi-monad sum so plan-holding callers skip the per-call string mapping.
type QNumericUnaryMultiSumPlan struct {
	ops   []string
	codes []monadSumOpcode
}

// NewQNumericUnaryMultiSumPlan resolves ops once; ok=false when any op is
// outside the fused kernel's domain (callers keep their per-term route).
func NewQNumericUnaryMultiSumPlan(ops []string) (*QNumericUnaryMultiSumPlan, bool) {
	if len(ops) < 2 {
		return nil, false
	}
	plan := &QNumericUnaryMultiSumPlan{
		ops:   append([]string(nil), ops...),
		codes: make([]monadSumOpcode, len(ops)),
	}
	for i, op := range ops {
		plan.codes[i] = monadSumOpcodeFor(op)
		if plan.codes[i] == monadSumInvalid {
			return nil, false
		}
	}
	return plan, true
}

// TryTypedQNumericUnaryMultiSum computes the per-op numeric unary sums for a
// small list of ops with a single pass over the source array. It returns the
// values in op order; ok=false leaves the caller on its per-term fallback
// path.
func TryTypedQNumericUnaryMultiSum(ops []string, array Array) ([]any, bool, error) {
	plan, ok := NewQNumericUnaryMultiSumPlan(ops)
	if !ok {
		return nil, false, nil
	}
	return TryTypedQNumericUnaryMultiSumPlanned(plan, array)
}

// TryTypedQNumericUnaryMultiSumPlanned is TryTypedQNumericUnaryMultiSum with
// the opcode resolution hoisted into a reusable plan.
func TryTypedQNumericUnaryMultiSumPlanned(plan *QNumericUnaryMultiSumPlan, array Array) ([]any, bool, error) {
	ops, codes := plan.ops, plan.codes
	for {
		attributed, ok := array.(attributedArray)
		if !ok {
			break
		}
		array = attributed.array
	}
	if values, ok := array.(i64RangeArray); ok {
		return monadMultiSumI64Range(ops, codes, values)
	}
	if kind := array.Kind(); kind != KindF64 && kind != KindF32 {
		// Integer-kind arrays route through integer single-op kernels with
		// integer accumulators; the float64 flatten below would not be
		// bit-identical for them.
		return nil, false, nil
	}
	if values, ok := array.(f64RangeArray); ok {
		if slots, ok := monadSumComboSlots(codes, monadSumFloor, monadSumCeiling, monadSumRecip); ok {
			fl, ce, re := monadFusedFloorCeilRecipAffine(values.start, values.step, values.len)
			out := make([]any, len(codes))
			out[slots[0]], out[slots[1]], out[slots[2]] = fl, ce, re
			return out, true, nil
		}
		if slots, ok := monadSumComboSlots(codes, monadSumSqrt, monadSumLog); ok {
			sq, lg := monadFusedSqrtLogAffine(values.start, values.step, values.len)
			out := make([]any, len(codes))
			out[slots[0]], out[slots[1]] = sq, lg
			return out, true, nil
		}
	}
	values, owned, ok := tryBulkF64Values(array)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := monadMultiSumFloatValues(ops, codes, values)
	bulkF64Release(values, owned)
	return out, handled, err
}

// monadMultiSumFloatValues evaluates every op over one shared dense slice:
// fused scalar-accumulator loops for the hot combos, single-op slice kernels
// (one pass per op, zero extra flattens) otherwise.
func monadMultiSumFloatValues(ops []string, codes []monadSumOpcode, values []float64) ([]any, bool, error) {
	if slots, ok := monadSumComboSlots(codes, monadSumFloor, monadSumCeiling, monadSumRecip); ok {
		fl, ce, re := monadFusedFloorCeilRecipSlice(values)
		out := make([]any, len(codes))
		out[slots[0]], out[slots[1]], out[slots[2]] = fl, ce, re
		return out, true, nil
	}
	if slots, ok := monadSumComboSlots(codes, monadSumSqrt, monadSumLog); ok {
		sq, lg := monadFusedSqrtLogSlice(values)
		out := make([]any, len(codes))
		out[slots[0]], out[slots[1]] = sq, lg
		return out, true, nil
	}
	out := make([]any, len(codes))
	for i, op := range ops {
		value, handled, err := qNumericUnarySumFloatSlice(op, values)
		if err != nil || !handled {
			return nil, handled, err
		}
		out[i] = value
	}
	return out, true, nil
}

// monadMultiSumI64Range handles range carriers: closed-form ops delegate to
// qNumericUnarySumI64Range (O(1) each, bit-identical by construction); the
// float-loop ops stream float64(value) exactly like the single-op range
// loops, fused for the hot combo and once per op otherwise.
func monadMultiSumI64Range(ops []string, codes []monadSumOpcode, values i64RangeArray) ([]any, bool, error) {
	if slots, ok := monadSumComboSlots(codes, monadSumSqrt, monadSumLog); ok {
		sq, lg := monadFusedSqrtLogI64Range(values)
		out := make([]any, len(codes))
		out[slots[0]], out[slots[1]] = sq, lg
		return out, true, nil
	}
	out := make([]any, len(codes))
	for i := range codes {
		value, ok := qNumericUnarySumI64Range(ops[i], values)
		if !ok {
			return nil, false, nil
		}
		out[i] = value
	}
	return out, true, nil
}

// monadFusedSqrtLogI64Range mirrors qNumericUnarySumI64Range's sqrt and log
// loops (same float64(value) stream, same per-op accumulation order) in one
// fused pass.
func monadFusedSqrtLogI64Range(values i64RangeArray) (float64, float64) {
	var sq, lg float64
	value := values.start
	for i := 0; i < values.len; i++ {
		x := float64(value)
		// Log before Sqrt measures ~1% faster on Apple Silicon (fewer
		// live FP values across the math.Log call); per-accumulator
		// element order is unchanged, so results stay bit-identical.
		lg += math.Log(x)
		sq += math.Sqrt(x)
		value += values.step
	}
	return sq, lg
}

// monadFusedSqrtLogAffine streams an f64RangeArray with the exact element
// expression tryBulkF64Values materializes (start + float64(i)*step).
func monadFusedSqrtLogAffine(start, step float64, n int) (float64, float64) {
	var sq, lg float64
	for i := 0; i < n; i++ {
		x := start + float64(i)*step
		lg += math.Log(x)
		sq += math.Sqrt(x)
	}
	return sq, lg
}

func monadFusedSqrtLogSlice(values []float64) (float64, float64) {
	var sq, lg float64
	for _, x := range values {
		lg += math.Log(x)
		sq += math.Sqrt(x)
	}
	return sq, lg
}

// monadFusedFloorCeilRecipAffine streams an f64RangeArray with the exact
// element expression tryBulkF64Values materializes.
func monadFusedFloorCeilRecipAffine(start, step float64, n int) (int64, int64, float64) {
	var fl, ce int64
	var re float64
	for i := 0; i < n; i++ {
		x := start + float64(i)*step
		fl += int64(math.Floor(x))
		ce += int64(math.Ceil(x))
		re += 1 / x
	}
	return fl, ce, re
}

func monadFusedFloorCeilRecipSlice(values []float64) (int64, int64, float64) {
	var fl, ce int64
	var re float64
	for _, x := range values {
		fl += int64(math.Floor(x))
		ce += int64(math.Ceil(x))
		re += 1 / x
	}
	return fl, ce, re
}
