package data

// Fused typed where-filter for compound `and` predicates.
//
// fastLogicalAndFilterIndexes' closure pipeline pays three indirect calls per
// row (left leaf, right leaf, and combiner) for shapes like
// `where active=true,price>=100`. The kernels below compile each
// column-vs-literal compare leaf of a pure-`and` tree into a typed
// scan/refine pair instead: the first leaf emits surviving row indexes in one
// monomorphic pass over the typed column, and every further leaf compacts the
// surviving index list in place. No per-row closure calls and no mask
// temporaries; per-row work matches a hand-written Go filter loop.
//
// Coverage is intentionally the same column/literal/coercion surface as
// typedCompareRowPredicate; any unsupported leaf, `or` node, or coercion
// failure returns ok=false and the caller falls back to the closure path,
// which remains the semantic ground truth (including float NaN ordering,
// reproduced here through the same boolCompare identities).

type typedWhereLeaf struct {
	scan   func(dst []int) []int
	refine func(rows []int) []int
	// count, when non-nil, returns the exact scan match count so the caller
	// can size the index vector once instead of paying append regrowth.
	count func() int
}

// typedAndCompareFilterIndexes filters the frame through a pure-`and` tree of
// typed compare leaves. ok=false defers to the closure-based path.
func typedAndCompareFilterIndexes(frame Frame, expr Logical) ([]int, bool) {
	leaves, ok := collectTypedAndCompareLeaves(frame, expr, nil)
	if !ok || len(leaves) == 0 {
		return nil, false
	}
	var out []int
	if count := leaves[0].count; count != nil {
		out = make([]int, 0, count())
	} else {
		out = filterIndexScratch(frame.Len())
	}
	out = leaves[0].scan(out)
	for _, leaf := range leaves[1:] {
		if len(out) == 0 {
			break
		}
		out = leaf.refine(out)
	}
	return out, true
}

func collectTypedAndCompareLeaves(frame Frame, expr Expr, leaves []typedWhereLeaf) ([]typedWhereLeaf, bool) {
	switch e := expr.(type) {
	case Logical:
		if e.Op != "and" {
			return nil, false
		}
		left, ok := collectTypedAndCompareLeaves(frame, e.Left, leaves)
		if !ok {
			return nil, false
		}
		return collectTypedAndCompareLeaves(frame, e.Right, left)
	case Binary:
		if !isComparisonOp(e.Op) {
			return nil, false
		}
		ref, op, literal, ok := binaryColumnLiteral(e)
		if !ok {
			return nil, false
		}
		col, ok := frame.Column(ref.Name)
		if !ok {
			return nil, false
		}
		leaf, ok := typedCompareWhereLeaf(col, op, normalizeScalar(col.Kind(), literal.Value))
		if !ok {
			return nil, false
		}
		return append(leaves, leaf), true
	default:
		return nil, false
	}
}

func typedCompareWhereLeaf(array Array, op Op, value any) (typedWhereLeaf, bool) {
	switch a := unwrapAttributedArray(array).(type) {
	case columnArray[bool]:
		target, ok := value.(bool)
		if !ok {
			return typedWhereLeaf{}, false
		}
		return boolWhereLeaf(a.data, target, op)
	case columnArray[int8]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[int8])
	case columnArray[int16]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[int16])
	case columnArray[int32]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[int32])
	case columnArray[int64]:
		return coercedOrderedWhereLeaf(a.data, value, op, coerceInt64Exact)
	case columnArray[uint8]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[uint8])
	case columnArray[uint16]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[uint16])
	case columnArray[uint32]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[uint32])
	case columnArray[uint64]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[uint64])
	case columnArray[float32]:
		target, ok := value.(float32)
		if !ok {
			return typedWhereLeaf{}, false
		}
		return floatWhereLeaf(a.data, target, op)
	case columnArray[float64]:
		target, ok := numeric(value)
		if !ok {
			return typedWhereLeaf{}, false
		}
		return floatWhereLeaf(a.data, target, op)
	case columnArray[string]:
		target, ok := coerceComparableString(value)
		if !ok {
			return typedWhereLeaf{}, false
		}
		return orderedWhereLeaf(a.data, target, op)
	case columnArray[Symbol]:
		target, ok := coerceComparableSymbol(value)
		if !ok {
			return typedWhereLeaf{}, false
		}
		return orderedWhereLeaf(a.data, target, op)
	case columnArray[Month]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[Month])
	case columnArray[Date]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[Date])
	case columnArray[DateTime]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[DateTime])
	case columnArray[Timespan]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[Timespan])
	case columnArray[Minute]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[Minute])
	case columnArray[Second]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[Second])
	case columnArray[Time]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[Time])
	case columnArray[Timestamp]:
		return coercedOrderedWhereLeaf(a.data, value, op, assertScalar[Timestamp])
	default:
		return typedWhereLeaf{}, false
	}
}

func assertScalar[T any](value any) (T, bool) {
	out, ok := value.(T)
	return out, ok
}

func coercedOrderedWhereLeaf[T whereOrderedScalar](values []T, value any, op Op, coerce func(any) (T, bool)) (typedWhereLeaf, bool) {
	target, ok := coerce(value)
	if !ok {
		return typedWhereLeaf{}, false
	}
	return orderedWhereLeaf(values, target, op)
}

type whereOrderedScalar interface {
	~int8 | ~int16 | ~int32 | ~int64 |
		~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~string
}

// orderedWhereLeaf covers integer, named-temporal, string, and symbol columns,
// whose `<`/`==` operators agree exactly with the boolCompare identities.
func orderedWhereLeaf[T whereOrderedScalar](values []T, target T, op Op) (typedWhereLeaf, bool) {
	switch op {
	case OpEQ:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v == target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] == target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpNE:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v != target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] != target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpLT:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v < target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] < target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpLE:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v <= target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] <= target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpGT:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v > target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] > target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpGE:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v >= target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] >= target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	default:
		return typedWhereLeaf{}, false
	}
}

// floatWhereLeaf reproduces compareFloatRowPredicate's NaN ordering exactly:
// boolCompare(OpLE) is `!(v > target)` and boolCompare(OpGE) is
// `!(v < target)`, both of which hold when either side is NaN.
func floatWhereLeaf[T floatScalar](values []T, target T, op Op) (typedWhereLeaf, bool) {
	switch op {
	case OpEQ:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v == target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] == target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpNE:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v != target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] != target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpLT:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v < target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] < target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpLE:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if !(v > target) {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if !(values[row] > target) {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpGT:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if v > target {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if values[row] > target {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	case OpGE:
		return typedWhereLeaf{
			scan: func(dst []int) []int {
				for row, v := range values {
					if !(v < target) {
						dst = append(dst, row)
					}
				}
				return dst
			},
			refine: func(rows []int) []int {
				k := 0
				for _, row := range rows {
					if !(values[row] < target) {
						rows[k] = row
						k++
					}
				}
				return rows[:k]
			},
		}, true
	default:
		return typedWhereLeaf{}, false
	}
}

// boolWhereLeaf reproduces compareBoolRowPredicate's
// boolCompare(op, v == target, compareBool(v, target)) identity by
// precomputing whether each of the two possible cell values holds.
func boolWhereLeaf(values []bool, target bool, op Op) (typedWhereLeaf, bool) {
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return typedWhereLeaf{}, false
	}
	holdsFalse := boolCompare(op, !target, compareBool(false, target))
	holdsTrue := boolCompare(op, target, compareBool(true, target))
	return typedWhereLeaf{
		count: func() int {
			trues := 0
			for _, v := range values {
				if v {
					trues++
				}
			}
			n := 0
			if holdsTrue {
				n += trues
			}
			if holdsFalse {
				n += len(values) - trues
			}
			return n
		},
		scan: func(dst []int) []int {
			for row, v := range values {
				if (v && holdsTrue) || (!v && holdsFalse) {
					dst = append(dst, row)
				}
			}
			return dst
		},
		refine: func(rows []int) []int {
			k := 0
			for _, row := range rows {
				v := values[row]
				if (v && holdsTrue) || (!v && holdsFalse) {
					rows[k] = row
					k++
				}
			}
			return rows[:k]
		},
	}, true
}
