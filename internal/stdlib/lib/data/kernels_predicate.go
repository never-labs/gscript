package data

import (
	"fmt"
	"math"
)

// Fused predicate-tree where evaluator.
//
// Compound `where (a) and (b) and (not (c)) ...` predicates arrive as lazy
// boolLogicalMask trees whose leaves are compare masks, membership masks, and
// already-materialized bool columns. The generic bulk path flattens every
// leaf into its own []bool temporary (re-flattening shared lazy sources such
// as `y:(x mod 97)+5` once per leaf) and then combines them pairwise, paying
// one full pass plus pool traffic per tree node.
//
// The evaluator below compiles the mask tree once per call into a small plan:
//   - distinct int64-producing leaf sources are deduplicated structurally and
//     flattened exactly once;
//   - `not` is folded into the leaves (inverted compare op, negated
//     membership / bool-slice flags) via De Morgan during compilation;
//   - the tree is then evaluated column-wise into a single accumulator mask,
//     each leaf fusing its predicate with the and/or combine in one pass, so
//     no per-leaf mask temporaries exist;
//   - where-indexes are emitted directly from the accumulator.
//
// Anything the compiler does not recognize becomes a generic bool-slice leaf
// through tryBulkBoolValues, so coverage is a strict superset of the previous
// bulk path; compile failure falls back to the existing routes.

type predCombine uint8

const (
	predCombineSet predCombine = iota
	predCombineAnd
	predCombineOr
)

type predNodeKind uint8

const (
	predNodeAnd predNodeKind = iota
	predNodeOr
	predNodeCmp
	predNodeIn
	predNodeWithin
	predNodeCmpF64
	predNodeBools
)

type predNode struct {
	kind        predNodeKind
	left, right *predNode

	// predNodeCmp / predNodeIn / predNodeWithin. modBy != 0 folds a trailing
	// `mod` step of the leaf's source chain into the predicate pass itself,
	// so `(y mod 4) = 1` shares y's flattened buffer instead of
	// materializing y mod 4.
	src      int
	op       Op
	scalar   int64
	hi       int64
	hiClosed bool
	modBy    int64
	probes   []int64
	negate   bool

	// predNodeCmpF64
	fscalar float64

	// predNodeBools
	bools      []bool
	boolsOwned bool
}

type predFlattenedBase struct {
	array  Array
	values []int64
	owned  bool
}

type predCompiler struct {
	length     int
	sources    []Array
	sourcesF64 []Array
	nodeBudget int
	ownedBools [][]bool
	bases      []predFlattenedBase
}

const predNodeBudget = 32

// fusedPredicateWhereIndexArray lowers `where <compound mask tree>` to index
// emission with one fused pass per leaf. ok=false leaves the mask untouched
// for the existing paths.
func fusedPredicateWhereIndexArray(mask Array) (Array, bool) {
	switch mask.(type) {
	case boolLogicalMask, notMask, i64ArrayCompareMask, i64MembershipMask, i64WithinMask:
	default:
		return nil, false
	}
	length := mask.Len()
	if length == 0 {
		return nil, false
	}
	c := predCompiler{length: length, nodeBudget: predNodeBudget}
	root, ok := c.compile(mask, false)
	if !ok {
		c.releaseOwnedBools()
		return nil, false
	}
	c.foldSingleUseLeafChains(root)
	if out, ok := c.tryAnalyticWhereIndexes(root); ok {
		c.releaseOwnedBools()
		return out, true
	}
	acc, ok := c.evalDenseBools(root)
	if !ok {
		return nil, false
	}
	indexes := bulkI64Get(length)
	count := 0
	for row, keep := range acc {
		if keep {
			indexes[count] = int64(row)
			count++
		}
	}
	bulkBoolRelease(acc, true)
	out := make([]int64, count)
	copy(out, indexes[:count])
	bulkI64Release(indexes, true)
	return newI64Trusted(out), true
}

// fusedPredicateDenseBoolMask lowers a compound mask tree to one pooled dense
// bool vector through the same compiled predicate tree `where` uses, so
// streaming reductions over compound masks share its exact row semantics
// without the per-row carrier walk. The returned buffer is owned by the
// caller: release it with bulkBoolRelease(mask, true).
func fusedPredicateDenseBoolMask(mask Array) ([]bool, bool) {
	switch mask.(type) {
	case boolLogicalMask, notMask, i64ArrayCompareMask, i64MembershipMask, i64WithinMask:
	default:
		return nil, false
	}
	length := mask.Len()
	if length == 0 {
		return nil, false
	}
	c := predCompiler{length: length, nodeBudget: predNodeBudget}
	root, ok := c.compile(mask, false)
	if !ok {
		c.releaseOwnedBools()
		return nil, false
	}
	c.foldSingleUseLeafChains(root)
	return c.evalDenseBools(root)
}

type predI64CompareLeaf struct {
	values []int64
	op     Op
	scalar int64
}

// fusedPredicateI64CompareAndSum lowers `sum values where c1 and c2 ...`
// for pure integer compare predicates to a single tight loop over the value
// column and flattened predicate columns. It handles the common q columnar
// shape without materializing intermediate compare masks or the final dense
// boolean mask; unsupported predicate trees fall back to the existing fused
// bool-mask path.
func fusedPredicateI64CompareAndSum(values, mask Array) (int64, int, bool) {
	length := mask.Len()
	if values == nil || mask == nil || length == 0 || values.Len() != length {
		return 0, 0, false
	}
	valueData, valueOwned, ok := tryBulkI64Values(values)
	if !ok || len(valueData) < length {
		bulkI64Release(valueData, valueOwned)
		return 0, 0, false
	}
	if total, count, ok := fusedPredicateI64CompareAndSumDirect(valueData[:length], mask, length); ok {
		bulkI64Release(valueData, valueOwned)
		return total, count, true
	}
	c := predCompiler{length: length, nodeBudget: predNodeBudget}
	root, ok := c.compile(mask, false)
	if !ok {
		c.releaseOwnedBools()
		bulkI64Release(valueData, valueOwned)
		return 0, 0, false
	}
	c.foldSingleUseLeafChains(root)
	sources := make([][]int64, len(c.sources))
	sourcesOwned := make([]bool, len(c.sources))
	release := func() {
		for i, source := range sources {
			bulkI64Release(source, sourcesOwned[i])
		}
		for _, base := range c.bases {
			bulkI64Release(base.values, base.owned)
		}
		c.releaseOwnedBools()
		bulkI64Release(valueData, valueOwned)
	}
	for i, sourceArray := range c.sources {
		if sourceArray == nil {
			continue
		}
		source, owned, ok := c.flattenSource(i, sourceArray, sources, length)
		if !ok || len(source) < length {
			bulkI64Release(source, owned)
			release()
			return 0, 0, false
		}
		sources[i] = source[:length]
		sourcesOwned[i] = owned
	}
	nodes := make([]*predNode, 0, predNodeBudget)
	if !collectPredAndLeaves(root, &nodes) || len(nodes) == 0 || len(nodes) > 8 {
		release()
		return 0, 0, false
	}
	leaves := make([]predI64CompareLeaf, 0, len(nodes))
	for _, node := range nodes {
		if node.kind != predNodeCmp || node.modBy != 0 || node.src < 0 || node.src >= len(sources) || sources[node.src] == nil {
			release()
			return 0, 0, false
		}
		leaves = append(leaves, predI64CompareLeaf{values: sources[node.src], op: node.op, scalar: node.scalar})
	}
	total, count, ok := sumI64WhereCompareAnd(valueData[:length], leaves)
	release()
	return total, count, ok
}

func fusedPredicateI64CompareAndSumDirect(values []int64, mask Array, length int) (int64, int, bool) {
	var leaves [8]predI64CompareLeaf
	var owned [8]bool
	count := 0
	if !collectI64CompareAndLeaves(mask, length, &leaves, &owned, &count) || count == 0 {
		releaseI64CompareLeaves(leaves[:count], owned[:count])
		return 0, 0, false
	}
	total, selected, ok := sumI64WhereCompareAnd(values, leaves[:count])
	releaseI64CompareLeaves(leaves[:count], owned[:count])
	return total, selected, ok
}

func collectI64CompareAndLeaves(mask Array, length int, leaves *[8]predI64CompareLeaf, owned *[8]bool, count *int) bool {
	switch a := mask.(type) {
	case attributedArray:
		return collectI64CompareAndLeaves(a.array, length, leaves, owned, count)
	case boolLogicalMask:
		if a.op != "and" || a.leftIsScalar || a.rightIsScalar || a.len != length || a.left == nil || a.right == nil {
			return false
		}
		if a.left.Len() != length || a.right.Len() != length {
			return false
		}
		return collectI64CompareAndLeaves(a.left, length, leaves, owned, count) &&
			collectI64CompareAndLeaves(a.right, length, leaves, owned, count)
	case i64ArrayCompareMask:
		if a.len != length || a.source == nil || a.source.Len() != length || *count >= len(leaves) {
			return false
		}
		op := effectiveRangeCompareOp(a.op, a.scalarLeft)
		switch op {
		case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
		default:
			return false
		}
		source, sourceOwned, ok := tryBulkI64Values(a.source)
		if !ok || len(source) < length {
			bulkI64Release(source, sourceOwned)
			return false
		}
		leaves[*count] = predI64CompareLeaf{values: source[:length], op: op, scalar: a.scalar}
		owned[*count] = sourceOwned
		*count = *count + 1
		return true
	default:
		return false
	}
}

func releaseI64CompareLeaves(leaves []predI64CompareLeaf, owned []bool) {
	for i := range leaves {
		bulkI64Release(leaves[i].values, owned[i])
	}
}

func sumI64WhereCompareAnd(values []int64, leaves []predI64CompareLeaf) (int64, int, bool) {
	if len(leaves) == 0 {
		return 0, 0, false
	}
	op := leaves[0].op
	for _, leaf := range leaves[1:] {
		if leaf.op != op {
			return 0, 0, false
		}
	}
	switch op {
	case OpGT:
		return sumI64WhereCompareAndGT(values, leaves)
	case OpGE:
		return sumI64WhereCompareAndGE(values, leaves)
	case OpLT:
		return sumI64WhereCompareAndLT(values, leaves)
	case OpLE:
		return sumI64WhereCompareAndLE(values, leaves)
	case OpEQ:
		return sumI64WhereCompareAndEQ(values, leaves)
	case OpNE:
		return sumI64WhereCompareAndNE(values, leaves)
	default:
		return 0, 0, false
	}
}

func sumI64WhereCompareAndGT(values []int64, leaves []predI64CompareLeaf) (int64, int, bool) {
	var total int64
	count := 0
	switch len(leaves) {
	case 1:
		a, ca := leaves[0].values, leaves[0].scalar
		for i, v := range values {
			if a[i] > ca {
				total += v
				count++
			}
		}
	case 2:
		a, b := leaves[0].values, leaves[1].values
		ca, cb := leaves[0].scalar, leaves[1].scalar
		for i, v := range values {
			if a[i] > ca && b[i] > cb {
				total += v
				count++
			}
		}
	case 3:
		a, b, c := leaves[0].values, leaves[1].values, leaves[2].values
		ca, cb, cc := leaves[0].scalar, leaves[1].scalar, leaves[2].scalar
		for i, v := range values {
			if a[i] > ca && b[i] > cb && c[i] > cc {
				total += v
				count++
			}
		}
	case 4:
		a, b, c, d := leaves[0].values, leaves[1].values, leaves[2].values, leaves[3].values
		ca, cb, cc, cd := leaves[0].scalar, leaves[1].scalar, leaves[2].scalar, leaves[3].scalar
		for i, v := range values {
			if a[i] > ca && b[i] > cb && c[i] > cc && d[i] > cd {
				total += v
				count++
			}
		}
	default:
		return sumI64WhereCompareAndSameOp(values, leaves, func(v, c int64) bool { return v > c })
	}
	return total, count, true
}

func sumI64WhereCompareAndGE(values []int64, leaves []predI64CompareLeaf) (int64, int, bool) {
	return sumI64WhereCompareAndSameOp(values, leaves, func(v, c int64) bool { return v >= c })
}

func sumI64WhereCompareAndLT(values []int64, leaves []predI64CompareLeaf) (int64, int, bool) {
	return sumI64WhereCompareAndSameOp(values, leaves, func(v, c int64) bool { return v < c })
}

func sumI64WhereCompareAndLE(values []int64, leaves []predI64CompareLeaf) (int64, int, bool) {
	return sumI64WhereCompareAndSameOp(values, leaves, func(v, c int64) bool { return v <= c })
}

func sumI64WhereCompareAndEQ(values []int64, leaves []predI64CompareLeaf) (int64, int, bool) {
	return sumI64WhereCompareAndSameOp(values, leaves, func(v, c int64) bool { return v == c })
}

func sumI64WhereCompareAndNE(values []int64, leaves []predI64CompareLeaf) (int64, int, bool) {
	return sumI64WhereCompareAndSameOp(values, leaves, func(v, c int64) bool { return v != c })
}

func sumI64WhereCompareAndSameOp(values []int64, leaves []predI64CompareLeaf, keep func(int64, int64) bool) (int64, int, bool) {
	var total int64
	count := 0
	for i, v := range values {
		matched := true
		for _, leaf := range leaves {
			if !keep(leaf.values[i], leaf.scalar) {
				matched = false
				break
			}
		}
		if matched {
			total += v
			count++
		}
	}
	return total, count, true
}

// evalDenseBools flattens the compiled tree's leaf sources and evaluates the
// predicate into one pooled dense bool buffer. All compiler-owned state is
// released before returning; on ok the caller owns the returned buffer and
// releases it with bulkBoolRelease(acc, true).
func (c *predCompiler) evalDenseBools(root *predNode) ([]bool, bool) {
	length := c.length
	sources := make([][]int64, len(c.sources))
	sourcesOwned := make([]bool, len(c.sources))
	sourcesF64 := make([][]float64, len(c.sourcesF64))
	sourcesF64Owned := make([]bool, len(c.sourcesF64))
	release := func() {
		for i, values := range sources {
			bulkI64Release(values, sourcesOwned[i])
		}
		for i, values := range sourcesF64 {
			bulkF64Release(values, sourcesF64Owned[i])
		}
		for _, base := range c.bases {
			bulkI64Release(base.values, base.owned)
		}
		c.releaseOwnedBools()
	}
	for i, sourceArray := range c.sources {
		if sourceArray == nil {
			continue
		}
		values, owned, ok := c.flattenSource(i, sourceArray, sources, length)
		if !ok || len(values) < length {
			bulkI64Release(values, owned)
			release()
			return nil, false
		}
		sources[i] = values[:length]
		sourcesOwned[i] = owned
	}
	for i, sourceArray := range c.sourcesF64 {
		values, owned, ok := tryBulkF64Values(sourceArray)
		if !ok || len(values) < length {
			bulkF64Release(values, owned)
			release()
			return nil, false
		}
		sourcesF64[i] = values[:length]
		sourcesF64Owned[i] = owned
	}
	acc := bulkBoolGet(length)
	evalPredInto(root, acc, predCombineSet, sources, sourcesF64)
	release()
	return acc, true
}

// flattenSource materializes leaf source idx, reusing the flattened values of
// an earlier source — or the retained base of an earlier chain — when this
// one is a scalar-dyadic chain built on top of it (`(y mod 4)` over
// registered `y`, or `1+(n mod 64)` next to `100+(n mod 90)` sharing base
// n), so shared derivation prefixes are computed once instead of once per
// leaf chain.
func (c *predCompiler) flattenSource(idx int, sourceArray Array, flattened [][]int64, length int) ([]int64, bool, bool) {
	dyadic, isDyadic := sourceArray.(i64ScalarDyadicArray)
	if !isDyadic || dyadic.len < length {
		return tryBulkI64Values(sourceArray)
	}
	var steps [4]i64ScalarDyadicStep
	stepCount := 0
	cur := Array(dyadic)
	for stepCount < len(steps) {
		inner, ok := cur.(i64ScalarDyadicArray)
		if !ok || inner.len < length {
			break
		}
		validOp := false
		switch inner.op {
		case OpAdd, OpSub, OpMul, OpMod, OpIDiv:
			validOp = true
		}
		if !validOp {
			break
		}
		steps[stepCount] = i64ScalarDyadicStep{op: inner.op, scalar: inner.scalar, scalarLeft: inner.scalarLeft}
		stepCount++
		cur = inner.source
		if input, ok := c.lookupFlattenedPrefix(cur, idx, flattened); ok {
			return c.applyChainSteps(steps[:stepCount], input, length)
		}
	}
	if stepCount > 0 {
		if _, stillDyadic := cur.(i64ScalarDyadicArray); !stillDyadic {
			baseValues, baseOwned, ok := tryBulkI64Values(cur)
			if ok && len(baseValues) >= length {
				baseValues = baseValues[:length]
				c.bases = append(c.bases, predFlattenedBase{array: cur, values: baseValues, owned: baseOwned})
				return c.applyChainSteps(steps[:stepCount], baseValues, length)
			}
			bulkI64Release(baseValues, baseOwned)
		}
	}
	return tryBulkI64Values(sourceArray)
}

func (c *predCompiler) lookupFlattenedPrefix(cur Array, idx int, flattened [][]int64) ([]int64, bool) {
	for j := 0; j < idx; j++ {
		if flattened[j] != nil && c.sources[j] != nil && samePredI64Source(c.sources[j], cur) {
			return flattened[j], true
		}
	}
	for i := range c.bases {
		if samePredI64Source(c.bases[i].array, cur) {
			return c.bases[i].values, true
		}
	}
	return nil, false
}

func (c *predCompiler) applyChainSteps(steps []i64ScalarDyadicStep, input []int64, length int) ([]int64, bool, bool) {
	out := bulkI64Get(length)
	in := input[:length]
	for s := len(steps) - 1; s >= 0; s-- {
		if !applyI64ScalarDyadicStep(steps[s], in, out) {
			bulkI64Release(out, true)
			return nil, false, false
		}
		in = out
	}
	return out, true, true
}

func (c *predCompiler) compile(mask Array, negate bool) (*predNode, bool) {
	if c.nodeBudget <= 0 {
		return nil, false
	}
	c.nodeBudget--
	switch a := mask.(type) {
	case attributedArray:
		return c.compile(a.array, negate)
	case boolLogicalMask:
		if a.leftIsScalar || a.rightIsScalar || a.len != c.length {
			return c.boolsLeaf(mask, negate)
		}
		if a.left == nil || a.right == nil || a.left.Len() != a.len || a.right.Len() != a.len {
			return c.boolsLeaf(mask, negate)
		}
		var kind predNodeKind
		switch a.op {
		case "and":
			kind = predNodeAnd
		case "or":
			kind = predNodeOr
		default:
			return c.boolsLeaf(mask, negate)
		}
		if negate {
			// De Morgan: not(l and r) = (not l) or (not r), and vice versa.
			if kind == predNodeAnd {
				kind = predNodeOr
			} else {
				kind = predNodeAnd
			}
		}
		left, ok := c.compile(a.left, negate)
		if !ok {
			return nil, false
		}
		right, ok := c.compile(a.right, negate)
		if !ok {
			return nil, false
		}
		return &predNode{kind: kind, left: left, right: right}, true
	case notMask:
		if a.array != nil && a.array.Kind() == KindBool {
			return c.compile(a.array, !negate)
		}
		return c.boolsLeaf(mask, negate)
	case i64ScalarDyadicCompareMask:
		if a.values.len != c.length {
			return c.boolsLeaf(mask, negate)
		}
		return c.cmpLeaf(a.values, a.op, a.scalar, a.scalarLeft, negate, mask)
	case i64ArrayCompareMask:
		if a.len != c.length || a.source == nil || a.source.Len() != c.length {
			return c.boolsLeaf(mask, negate)
		}
		return c.cmpLeaf(a.source, a.op, a.scalar, a.scalarLeft, negate, mask)
	case i64RangeCompareMask:
		if a.values.len != c.length {
			return c.boolsLeaf(mask, negate)
		}
		return c.cmpLeaf(a.values, a.op, a.scalar, a.scalarLeft, negate, mask)
	case i64SegmentCompareMask:
		if a.values.len != c.length {
			return c.boolsLeaf(mask, negate)
		}
		return c.cmpLeaf(a.values, a.op, a.scalar, a.scalarLeft, negate, mask)
	case i64MembershipMask:
		if a.len != c.length {
			return c.boolsLeaf(mask, negate)
		}
		source, modBy := c.peelTrailingMod(a.source)
		return &predNode{kind: predNodeIn, src: c.addSource(source), modBy: modBy, probes: a.probes, negate: negate}, true
	case i64WithinMask:
		if a.len != c.length {
			return c.boolsLeaf(mask, negate)
		}
		source, modBy := c.peelTrailingMod(a.source)
		return &predNode{
			kind: predNodeWithin, src: c.addSource(source), modBy: modBy,
			scalar: a.lo, hi: a.hi, hiClosed: a.highClosed, negate: negate,
		}, true
	case f64CompareMask:
		if a.source.len != c.length {
			return c.boolsLeaf(mask, negate)
		}
		op := a.op
		if negate {
			inverted, ok := invertedCompareOp(op)
			if !ok {
				return c.boolsLeaf(mask, negate)
			}
			op = inverted
		}
		c.sourcesF64 = append(c.sourcesF64, a.source)
		return &predNode{kind: predNodeCmpF64, src: len(c.sourcesF64) - 1, op: op, fscalar: a.scalar}, true
	case columnArray[bool]:
		if len(a.data) != c.length {
			return nil, false
		}
		return &predNode{kind: predNodeBools, bools: a.data, negate: negate}, true
	default:
		return c.boolsLeaf(mask, negate)
	}
}

func (c *predCompiler) cmpLeaf(values Array, op Op, scalar int64, scalarLeft, negate bool, mask Array) (*predNode, bool) {
	op = effectiveRangeCompareOp(op, scalarLeft)
	if negate {
		inverted, ok := invertedCompareOp(op)
		if !ok {
			return c.boolsLeaf(mask, negate)
		}
		op = inverted
	}
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return c.boolsLeaf(mask, negate)
	}
	source, modBy := c.peelTrailingMod(values)
	return &predNode{kind: predNodeCmp, src: c.addSource(source), op: op, scalar: scalar, modBy: modBy}, true
}

// foldSingleUseLeafChains rewrites leaves whose source chain has the shape
// `(base mod m) ± c1 ± c2 ...` and is referenced by exactly one leaf: the
// additive constants fold into the leaf's compare bounds / probes and the
// mod folds into modBy, so the derived vector (`100+(n mod 90)`) is never
// materialized and the leaf reads the shared base directly. The mod step
// bounds the value range to [0, m-1], which makes the additive folding exact
// (all intermediate values and adjusted bounds are overflow-checked).
func (c *predCompiler) foldSingleUseLeafChains(root *predNode) {
	if len(c.sources) == 0 {
		return
	}
	usage := make([]int, len(c.sources))
	countPredSourceUsage(root, usage)
	c.foldLeafChains(root, usage)
}

func countPredSourceUsage(node *predNode, usage []int) {
	switch node.kind {
	case predNodeAnd, predNodeOr:
		countPredSourceUsage(node.left, usage)
		countPredSourceUsage(node.right, usage)
	case predNodeCmp, predNodeIn, predNodeWithin:
		usage[node.src]++
	}
}

func (c *predCompiler) foldLeafChains(node *predNode, usage []int) {
	switch node.kind {
	case predNodeAnd, predNodeOr:
		c.foldLeafChains(node.left, usage)
		c.foldLeafChains(node.right, usage)
	case predNodeCmp, predNodeIn, predNodeWithin:
		if node.modBy != 0 || usage[node.src] != 1 {
			return
		}
		chain, ok := c.sources[node.src].(i64ScalarDyadicArray)
		if !ok || chain.len < c.length {
			return
		}
		// Peel outermost additive steps; their composition is one shift.
		shift := int64(0)
		peeled := 0
		cur := Array(chain)
		for peeled < 3 {
			d, ok := cur.(i64ScalarDyadicArray)
			if !ok || d.len < c.length {
				break
			}
			var delta int64
			switch {
			case d.op == OpAdd:
				delta = d.scalar
			case d.op == OpSub && !d.scalarLeft:
				delta = -d.scalar
				if d.scalar == math.MinInt64 {
					return
				}
			default:
				delta = 0
			}
			if delta == 0 && !(d.op == OpAdd && d.scalar == 0) {
				break
			}
			next, ok := addInt64Checked(shift, delta)
			if !ok {
				return
			}
			shift = next
			peeled++
			cur = d.source
		}
		if peeled == 0 {
			return
		}
		mod, ok := cur.(i64ScalarDyadicArray)
		if !ok || mod.op != OpMod || mod.scalarLeft || mod.scalar <= 0 || mod.len < c.length {
			return
		}
		// Kernel values are (base mod m) + shift with (m-1)+shift in range:
		// folding is exact only when the kernel itself cannot wrap.
		if _, ok := addInt64Checked(mod.scalar-1, shift); !ok {
			return
		}
		switch node.kind {
		case predNodeCmp:
			adjusted, ok := addInt64Checked(node.scalar, -shift)
			if !ok {
				return
			}
			node.scalar = adjusted
		case predNodeWithin:
			lo, ok := addInt64Checked(node.scalar, -shift)
			if !ok {
				return
			}
			hi, ok := addInt64Checked(node.hi, -shift)
			if !ok {
				return
			}
			node.scalar, node.hi = lo, hi
		case predNodeIn:
			translated := make([]int64, len(node.probes))
			for i, probe := range node.probes {
				adjusted, ok := addInt64Checked(probe, -shift)
				if !ok {
					return
				}
				translated[i] = adjusted
			}
			node.probes = translated
		}
		node.modBy = mod.scalar
		c.retargetLeafSource(node, mod.source)
	}
}

// retargetLeafSource points a folded leaf at the chain's base, reusing an
// existing registered source when one matches and dropping the orphaned
// chain from materialization.
func (c *predCompiler) retargetLeafSource(node *predNode, base Array) {
	for i, existing := range c.sources {
		if i != node.src && existing != nil && samePredI64Source(existing, base) {
			c.sources[node.src] = nil
			node.src = i
			return
		}
	}
	c.sources[node.src] = base
}

func addInt64Checked(a, b int64) (int64, bool) {
	s := a + b
	if (b > 0 && s < a) || (b < 0 && s > a) {
		return 0, false
	}
	return s, true
}

// peelTrailingMod folds a leaf source's outermost `mod <scalar>` step into
// the predicate node, so the (usually single-use) mod-derived vector is
// never materialized and the underlying source can be shared across leaves.
func (c *predCompiler) peelTrailingMod(values Array) (Array, int64) {
	if dyadic, ok := values.(i64ScalarDyadicArray); ok &&
		dyadic.op == OpMod && !dyadic.scalarLeft && dyadic.scalar != 0 && dyadic.len >= c.length {
		return dyadic.source, dyadic.scalar
	}
	return values, 0
}

func (c *predCompiler) boolsLeaf(mask Array, negate bool) (*predNode, bool) {
	if mask.Kind() != KindBool || mask.Len() != c.length {
		return nil, false
	}
	values, owned, ok := tryBulkBoolValues(mask)
	if !ok || len(values) != c.length {
		bulkBoolRelease(values, owned)
		return nil, false
	}
	if owned {
		c.ownedBools = append(c.ownedBools, values)
	}
	return &predNode{kind: predNodeBools, bools: values, boolsOwned: owned, negate: negate}, true
}

func (c *predCompiler) releaseOwnedBools() {
	for _, values := range c.ownedBools {
		bulkBoolRelease(values, true)
	}
	c.ownedBools = nil
}

func (c *predCompiler) addSource(array Array) int {
	if attributed, ok := array.(attributedArray); ok {
		return c.addSource(attributed.array)
	}
	for i, existing := range c.sources {
		if samePredI64Source(existing, array) {
			return i
		}
	}
	c.sources = append(c.sources, array)
	return len(c.sources) - 1
}

// samePredI64Source reports structural identity between integer leaf sources
// so shared derived vectors (`y:(x mod 97)+5` referenced by several clauses)
// are flattened once. Slices compare by backing-array identity; unknown
// carrier types conservatively report false.
func samePredI64Source(a, b Array) bool {
	switch x := a.(type) {
	case attributedArray:
		return samePredI64Source(x.array, b)
	case i64ScalarDyadicArray:
		y, ok := b.(i64ScalarDyadicArray)
		return ok && x.op == y.op && x.scalar == y.scalar && x.scalarLeft == y.scalarLeft &&
			x.len == y.len && samePredI64Source(x.source, y.source)
	case i64RangeArray:
		y, ok := b.(i64RangeArray)
		return ok && x == y
	case i64SegmentArray:
		y, ok := b.(i64SegmentArray)
		return ok && x.len == y.len && len(x.segments) == len(y.segments) &&
			(len(x.segments) == 0 || &x.segments[0] == &y.segments[0])
	case i64BucketArray:
		y, ok := b.(i64BucketArray)
		return ok && x.width == y.width && x.len == y.len && samePredI64Source(x.source, y.source)
	case tiledArray:
		y, ok := b.(tiledArray)
		return ok && x.start == y.start && x.len == y.len && samePredI64Source(x.source, y.source)
	case columnArray[int64]:
		y, ok := b.(columnArray[int64])
		return ok && sameSliceIdentity(x.data, y.data)
	case columnArray[int32]:
		y, ok := b.(columnArray[int32])
		return ok && sameSliceIdentity(x.data, y.data)
	case columnArray[int16]:
		y, ok := b.(columnArray[int16])
		return ok && sameSliceIdentity(x.data, y.data)
	case columnArray[int8]:
		y, ok := b.(columnArray[int8])
		return ok && sameSliceIdentity(x.data, y.data)
	default:
		if attributed, ok := b.(attributedArray); ok {
			return samePredI64Source(a, attributed.array)
		}
		return false
	}
}

func sameSliceIdentity[T any](a, b []T) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

func invertedCompareOp(op Op) (Op, bool) {
	switch op {
	case OpEQ:
		return OpNE, true
	case OpNE:
		return OpEQ, true
	case OpLT:
		return OpGE, true
	case OpLE:
		return OpGT, true
	case OpGT:
		return OpLE, true
	case OpGE:
		return OpLT, true
	default:
		return op, false
	}
}

func evalPredInto(node *predNode, acc []bool, mode predCombine, sources [][]int64, sourcesF64 [][]float64) {
	switch node.kind {
	case predNodeAnd:
		switch mode {
		case predCombineSet:
			evalPredInto(node.left, acc, predCombineSet, sources, sourcesF64)
			evalPredInto(node.right, acc, predCombineAnd, sources, sourcesF64)
		case predCombineAnd:
			evalPredInto(node.left, acc, predCombineAnd, sources, sourcesF64)
			evalPredInto(node.right, acc, predCombineAnd, sources, sourcesF64)
		case predCombineOr:
			tmp := bulkBoolGet(len(acc))
			evalPredInto(node.left, tmp, predCombineSet, sources, sourcesF64)
			evalPredInto(node.right, tmp, predCombineAnd, sources, sourcesF64)
			for i := range acc {
				acc[i] = acc[i] || tmp[i]
			}
			bulkBoolRelease(tmp, true)
		}
	case predNodeOr:
		switch mode {
		case predCombineSet:
			evalPredInto(node.left, acc, predCombineSet, sources, sourcesF64)
			evalPredInto(node.right, acc, predCombineOr, sources, sourcesF64)
		case predCombineOr:
			evalPredInto(node.left, acc, predCombineOr, sources, sourcesF64)
			evalPredInto(node.right, acc, predCombineOr, sources, sourcesF64)
		case predCombineAnd:
			tmp := bulkBoolGet(len(acc))
			evalPredInto(node.left, tmp, predCombineSet, sources, sourcesF64)
			evalPredInto(node.right, tmp, predCombineOr, sources, sourcesF64)
			for i := range acc {
				acc[i] = acc[i] && tmp[i]
			}
			bulkBoolRelease(tmp, true)
		}
	case predNodeCmp:
		predCmpInto(acc, sources[node.src], node.op, node.scalar, node.modBy, mode)
	case predNodeIn:
		predInInto(acc, sources[node.src], node.probes, node.modBy, node.negate, mode)
	case predNodeWithin:
		predWithinInto(acc, sources[node.src], node, mode)
	case predNodeCmpF64:
		predCmpF64Into(acc, sourcesF64[node.src], node.op, node.fscalar, mode)
	case predNodeBools:
		predBoolsInto(acc, node.bools, node.negate, mode)
	}
}

// predWithinInto fuses `(v [mod m]) within lo hi` into one pass over the
// shared flattened source. The closed-high no-mod no-negate shape gets
// dedicated branch-free loops; the rest goes through the general loops.
func predWithinInto(acc []bool, values []int64, node *predNode, mode predCombine) {
	lo, hi, hiClosed, modBy, negate := node.scalar, node.hi, node.hiClosed, node.modBy, node.negate
	if hiClosed && !negate {
		switch mode {
		case predCombineSet:
			if modBy != 0 {
				for i, v := range values {
					m := qModInt64(v, modBy)
					acc[i] = m >= lo && m <= hi
				}
			} else {
				for i, v := range values {
					acc[i] = v >= lo && v <= hi
				}
			}
		case predCombineAnd:
			if modBy != 0 {
				for i, v := range values {
					if acc[i] {
						m := qModInt64(v, modBy)
						acc[i] = m >= lo && m <= hi
					}
				}
			} else {
				for i, v := range values {
					acc[i] = acc[i] && v >= lo && v <= hi
				}
			}
		case predCombineOr:
			if modBy != 0 {
				for i, v := range values {
					if !acc[i] {
						m := qModInt64(v, modBy)
						acc[i] = m >= lo && m <= hi
					}
				}
			} else {
				for i, v := range values {
					acc[i] = acc[i] || (v >= lo && v <= hi)
				}
			}
		}
		return
	}
	inRange := func(v int64) bool {
		if modBy != 0 {
			v = qModInt64(v, modBy)
		}
		if v < lo {
			return false
		}
		if hiClosed {
			return v <= hi
		}
		return v < hi
	}
	switch mode {
	case predCombineSet:
		for i, v := range values {
			acc[i] = inRange(v) != negate
		}
	case predCombineAnd:
		for i, v := range values {
			if acc[i] {
				acc[i] = inRange(v) != negate
			}
		}
	case predCombineOr:
		for i, v := range values {
			if !acc[i] {
				acc[i] = inRange(v) != negate
			}
		}
	}
}

// predCmpF64Into mirrors boolCompare over compareFloat64 semantics: LE/GE
// are computed as negations of the strict comparisons so NaN rows (nulls)
// order as compareFloat64 does.
func predCmpF64Into(acc []bool, values []float64, op Op, scalar float64, mode predCombine) {
	switch mode {
	case predCombineSet:
		switch op {
		case OpEQ:
			for i, v := range values {
				acc[i] = v == scalar
			}
		case OpNE:
			for i, v := range values {
				acc[i] = v != scalar
			}
		case OpLT:
			for i, v := range values {
				acc[i] = v < scalar
			}
		case OpLE:
			for i, v := range values {
				acc[i] = !(v > scalar)
			}
		case OpGT:
			for i, v := range values {
				acc[i] = v > scalar
			}
		case OpGE:
			for i, v := range values {
				acc[i] = !(v < scalar)
			}
		}
	case predCombineAnd:
		switch op {
		case OpEQ:
			for i, v := range values {
				acc[i] = acc[i] && v == scalar
			}
		case OpNE:
			for i, v := range values {
				acc[i] = acc[i] && v != scalar
			}
		case OpLT:
			for i, v := range values {
				acc[i] = acc[i] && v < scalar
			}
		case OpLE:
			for i, v := range values {
				acc[i] = acc[i] && !(v > scalar)
			}
		case OpGT:
			for i, v := range values {
				acc[i] = acc[i] && v > scalar
			}
		case OpGE:
			for i, v := range values {
				acc[i] = acc[i] && !(v < scalar)
			}
		}
	case predCombineOr:
		switch op {
		case OpEQ:
			for i, v := range values {
				acc[i] = acc[i] || v == scalar
			}
		case OpNE:
			for i, v := range values {
				acc[i] = acc[i] || v != scalar
			}
		case OpLT:
			for i, v := range values {
				acc[i] = acc[i] || v < scalar
			}
		case OpLE:
			for i, v := range values {
				acc[i] = acc[i] || !(v > scalar)
			}
		case OpGT:
			for i, v := range values {
				acc[i] = acc[i] || v > scalar
			}
		case OpGE:
			for i, v := range values {
				acc[i] = acc[i] || !(v < scalar)
			}
		}
	}
}

func predCmpInto(acc []bool, values []int64, op Op, scalar, modBy int64, mode predCombine) {
	if modBy != 0 {
		predModCmpInto(acc, values, op, scalar, modBy, mode)
		return
	}
	switch mode {
	case predCombineSet:
		fillCompareI64Mask(values, op, scalar, acc)
	case predCombineAnd:
		switch op {
		case OpEQ:
			for i, v := range values {
				acc[i] = acc[i] && v == scalar
			}
		case OpNE:
			for i, v := range values {
				acc[i] = acc[i] && v != scalar
			}
		case OpLT:
			for i, v := range values {
				acc[i] = acc[i] && v < scalar
			}
		case OpLE:
			for i, v := range values {
				acc[i] = acc[i] && v <= scalar
			}
		case OpGT:
			for i, v := range values {
				acc[i] = acc[i] && v > scalar
			}
		case OpGE:
			for i, v := range values {
				acc[i] = acc[i] && v >= scalar
			}
		}
	case predCombineOr:
		switch op {
		case OpEQ:
			for i, v := range values {
				acc[i] = acc[i] || v == scalar
			}
		case OpNE:
			for i, v := range values {
				acc[i] = acc[i] || v != scalar
			}
		case OpLT:
			for i, v := range values {
				acc[i] = acc[i] || v < scalar
			}
		case OpLE:
			for i, v := range values {
				acc[i] = acc[i] || v <= scalar
			}
		case OpGT:
			for i, v := range values {
				acc[i] = acc[i] || v > scalar
			}
		case OpGE:
			for i, v := range values {
				acc[i] = acc[i] || v >= scalar
			}
		}
	}
}

// predModCmpInto fuses `(v mod m) op scalar` into one pass. The and-mode
// loops skip the division for rows the accumulator has already rejected.
func predModCmpInto(acc []bool, values []int64, op Op, scalar, modBy int64, mode predCombine) {
	switch mode {
	case predCombineSet:
		switch op {
		case OpEQ:
			for i, v := range values {
				acc[i] = qModInt64(v, modBy) == scalar
			}
		case OpNE:
			for i, v := range values {
				acc[i] = qModInt64(v, modBy) != scalar
			}
		case OpLT:
			for i, v := range values {
				acc[i] = qModInt64(v, modBy) < scalar
			}
		case OpLE:
			for i, v := range values {
				acc[i] = qModInt64(v, modBy) <= scalar
			}
		case OpGT:
			for i, v := range values {
				acc[i] = qModInt64(v, modBy) > scalar
			}
		case OpGE:
			for i, v := range values {
				acc[i] = qModInt64(v, modBy) >= scalar
			}
		}
	case predCombineAnd:
		for i, v := range values {
			if !acc[i] {
				continue
			}
			m := qModInt64(v, modBy)
			switch op {
			case OpEQ:
				acc[i] = m == scalar
			case OpNE:
				acc[i] = m != scalar
			case OpLT:
				acc[i] = m < scalar
			case OpLE:
				acc[i] = m <= scalar
			case OpGT:
				acc[i] = m > scalar
			case OpGE:
				acc[i] = m >= scalar
			}
		}
	case predCombineOr:
		for i, v := range values {
			if acc[i] {
				continue
			}
			m := qModInt64(v, modBy)
			switch op {
			case OpEQ:
				acc[i] = m == scalar
			case OpNE:
				acc[i] = m != scalar
			case OpLT:
				acc[i] = m < scalar
			case OpLE:
				acc[i] = m <= scalar
			case OpGT:
				acc[i] = m > scalar
			case OpGE:
				acc[i] = m >= scalar
			}
		}
	}
}

func i64ProbesContain(probes []int64, v int64) bool {
	for _, probe := range probes {
		if v == probe {
			return true
		}
	}
	return false
}

func predInInto(acc []bool, values []int64, probes []int64, modBy int64, negate bool, mode predCombine) {
	switch mode {
	case predCombineSet:
		for i, v := range values {
			if modBy != 0 {
				v = qModInt64(v, modBy)
			}
			acc[i] = i64ProbesContain(probes, v) != negate
		}
	case predCombineAnd:
		for i, v := range values {
			if acc[i] {
				if modBy != 0 {
					v = qModInt64(v, modBy)
				}
				acc[i] = i64ProbesContain(probes, v) != negate
			}
		}
	case predCombineOr:
		for i, v := range values {
			if !acc[i] {
				if modBy != 0 {
					v = qModInt64(v, modBy)
				}
				acc[i] = i64ProbesContain(probes, v) != negate
			}
		}
	}
}

func predBoolsInto(acc []bool, bools []bool, negate bool, mode predCombine) {
	switch mode {
	case predCombineSet:
		for i, v := range bools {
			acc[i] = v != negate
		}
	case predCombineAnd:
		for i, v := range bools {
			acc[i] = acc[i] && (v != negate)
		}
	case predCombineOr:
		for i, v := range bools {
			acc[i] = acc[i] || (v != negate)
		}
	}
}

// i64MembershipMask is a lazy boolean carrier for `values in <small literal
// set>` over derived integer vectors. Materializing the mask eagerly forces
// an extra full flatten of the source at `in` time; keeping it lazy lets the
// fused where evaluator fold the probe into its single pass over the shared
// flattened source. Non-where consumers materialize through
// tryBulkBoolValues or per-row At with identical semantics.
type i64MembershipMask struct {
	source Array
	probes []int64
	len    int
}

func (a i64MembershipMask) Kind() Kind { return KindBool }

func (a i64MembershipMask) Len() int { return a.len }

func (a i64MembershipMask) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	value, ok, err := integerArrayAt(a.source, row)
	if err != nil {
		return nil, false
	}
	if !ok {
		return false, true
	}
	return i64ProbesContain(a.probes, value), true
}

func (a i64MembershipMask) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data membership mask row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64MembershipMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data membership mask gather index %d out of range", row))
		}
		out[i] = value.(bool)
	}
	return newBoolTrusted(out)
}

// i64WithinMask is a lazy boolean carrier for `values within lo hi` over
// derived integer vectors, mirroring i64MembershipMask: the fused where
// evaluator folds the range check into its single pass over the shared
// flattened source instead of materializing the mask at `within` time.
type i64WithinMask struct {
	source     Array
	lo, hi     int64
	highClosed bool
	len        int
}

func (a i64WithinMask) Kind() Kind { return KindBool }

func (a i64WithinMask) Len() int { return a.len }

func (a i64WithinMask) valueOf(v int64) bool {
	if v < a.lo {
		return false
	}
	if a.highClosed {
		return v <= a.hi
	}
	return v < a.hi
}

func (a i64WithinMask) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	value, ok, err := integerArrayAt(a.source, row)
	if err != nil {
		return nil, false
	}
	if !ok {
		return false, true
	}
	return a.valueOf(value), true
}

func (a i64WithinMask) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data within mask row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64WithinMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data within mask gather index %d out of range", row))
		}
		out[i] = value.(bool)
	}
	return newBoolTrusted(out)
}

// f64CompareMask is a lazy boolean carrier for float compares over lazy
// numeric carriers (`((x mod 64)*0.25) > 4.5`): the fused where evaluator
// folds the compare into one pass over the flattened float source instead of
// materializing the mask eagerly at compare time. op is normalized to the
// value-on-left orientation. Row semantics replicate boolCompare over
// compareFloat64 exactly (nulls evaluate as NaN).
type f64CompareMask struct {
	source f64NumericDyadicArray
	op     Op
	scalar float64
}

func (a f64CompareMask) Kind() Kind { return KindBool }

func (a f64CompareMask) Len() int { return a.source.len }

func (a f64CompareMask) valueAt(row int) (bool, bool, error) {
	value, ok, err := a.source.f64At(row)
	if err != nil {
		return false, false, err
	}
	if !ok {
		value = math.NaN()
	}
	return boolCompare(a.op, value == a.scalar, compareFloat64(value, a.scalar)), true, nil
}

func (a f64CompareMask) At(row int) (any, bool) {
	if row < 0 || row >= a.source.len {
		return nil, false
	}
	value, ok, err := a.valueAt(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a f64CompareMask) Values() []any {
	out := make([]any, a.source.len)
	for row := range out {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data float compare mask row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a f64CompareMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.source.len {
			panic(fmt.Sprintf("data float compare mask gather index %d out of range", row))
		}
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data float compare mask row %d out of range", row))
		}
		out[i] = value
	}
	return newBoolTrusted(out)
}

// lazyWithinMask lowers `array within lo hi` to a lazy carrier for derived
// integer sources so compound where predicates can fuse the range check.
func lazyWithinMask(array Array, low, high any, highClosed bool) (Array, bool) {
	switch array.(type) {
	case i64ScalarDyadicArray, i64RangeArray, i64BucketArray:
	default:
		return nil, false
	}
	lo, ok := coerceInt64Exact(low)
	if !ok {
		return nil, false
	}
	hi, ok := coerceInt64Exact(high)
	if !ok {
		return nil, false
	}
	return i64WithinMask{source: array, lo: lo, hi: hi, highClosed: highClosed, len: array.Len()}, true
}

// lazyF64CompareMask lowers float compares over lazy float dyadic carriers
// to a lazy mask. Scalar-on-left orientation normalizes into op.
func lazyF64CompareMask(op Op, left, right any, length int) (Array, bool) {
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return nil, false
	}
	if array, ok := left.(f64NumericDyadicArray); ok && array.len == length {
		if scalar, ok := f64CompareScalarValue(right); ok {
			return f64CompareMask{source: array, op: effectiveRangeCompareOp(op, false), scalar: scalar}, true
		}
	}
	if array, ok := right.(f64NumericDyadicArray); ok && array.len == length {
		if scalar, ok := f64CompareScalarValue(left); ok {
			return f64CompareMask{source: array, op: effectiveRangeCompareOp(op, true), scalar: scalar}, true
		}
	}
	return nil, false
}

func f64CompareScalarValue(value any) (float64, bool) {
	if _, isArray := value.(Array); isArray || IsNull(value) {
		return 0, false
	}
	return numeric(value)
}
