package data

import "sort"

// Analytic leaf bundling for the fused predicate-where evaluator.
//
// Compound conjunctive predicates over derived integer chains
// (`where (y>20) and (y<90) and ((y mod 4)=1) and (not (y in 25 33 41))` with
// `y:(x mod 97)+5` over `x:til n`) previously paid one full-length pass per
// leaf plus a full flatten of every shared source. When every leaf of the
// AND-tree reads a source whose value is a *closed-form function of the row*
// the whole tree collapses analytically:
//
//   - sources containing a `mod m` step over an affine (til-derived) base are
//     periodic in the row with period m, so every leaf over them is decided
//     by `row mod P` for the lcm P of the leaf periods. All periodic leaves
//     are bundled into a single accumulator pass over ONE period prefix of
//     the shared source (truncated lazy chain, flattened once at length P)
//     instead of k passes over all n rows;
//   - leaves over plain affine sources (`x>64`, `x within 100 7000`,
//     `not (x in 70 73 76)`) become row-interval clamps and tiny
//     include/exclude row sets;
//   - where-indexes are emitted directly from the residue table restricted to
//     the row interval — O(P + selected) instead of O(k·n). Fully periodic
//     full-range trees emit the lazy i64PeriodicIndexArray carrier so
//     downstream gather/reduce stages stay analytic too.
//
// Any leaf that does not classify (materialized bool columns, non-unit
// strides, value overflow risks, OR nodes) bails the whole bundle back to the
// existing per-leaf fused passes, so coverage is unchanged.

// predRowShapeKind classifies how a leaf source's value depends on the row.
type predRowShapeKind uint8

const (
	predRowShapeAffine   predRowShapeKind = iota // value = base + step*row, exact (no wrap) for all rows
	predRowShapePeriodic                         // value depends on the row only through row mod period
)

type predRowShape struct {
	kind   predRowShapeKind
	base   int64 // affine
	step   int64 // affine
	period int64 // periodic
}

const predAnalyticMaxPeriod = 4096

// classifyPredRowShapeI64 resolves an integer leaf source to its row shape.
// Affine shapes are guarded against int64 wrap over rows [0, length), which
// makes both the interval math and the mod-periodicity argument exact; any
// step that could wrap declines classification.
func classifyPredRowShapeI64(array Array, length int) (predRowShape, bool) {
	switch a := array.(type) {
	case attributedArray:
		return classifyPredRowShapeI64(a.array, length)
	case i64RangeArray:
		if a.len < length {
			return predRowShape{}, false
		}
		shape := predRowShape{kind: predRowShapeAffine, base: a.start, step: a.step}
		if !predAffineBoundsOK(shape, length) {
			return predRowShape{}, false
		}
		return shape, true
	case i64ScalarDyadicArray:
		if a.len < length {
			return predRowShape{}, false
		}
		inner, ok := classifyPredRowShapeI64(a.source, length)
		if !ok {
			return predRowShape{}, false
		}
		switch a.op {
		case OpMod:
			if a.scalarLeft || a.scalar <= 0 {
				return predRowShape{}, false
			}
			if inner.kind == predRowShapePeriodic {
				return inner, true
			}
			return predRowShape{kind: predRowShapePeriodic, period: a.scalar}, true
		case OpAdd, OpSub, OpMul:
			if inner.kind == predRowShapePeriodic {
				// Pointwise steps over a periodic value stay periodic with
				// the same period regardless of wrap: equal inputs always
				// produce equal outputs.
				return inner, true
			}
			return applyPredAffineStep(a.op, a.scalar, a.scalarLeft, inner, length)
		case OpIDiv:
			if inner.kind == predRowShapePeriodic && !a.scalarLeft && a.scalar != 0 {
				return inner, true
			}
			return predRowShape{}, false
		default:
			return predRowShape{}, false
		}
	case i64BucketArray:
		if a.len < length || a.width == 0 {
			return predRowShape{}, false
		}
		inner, ok := classifyPredRowShapeI64(a.source, length)
		if !ok || inner.kind != predRowShapePeriodic {
			return predRowShape{}, false
		}
		return inner, true
	default:
		return predRowShape{}, false
	}
}

// applyPredAffineStep folds one scalar-dyadic step into an affine shape with
// checked arithmetic; the result must stay wrap-free over [0, length).
func applyPredAffineStep(op Op, scalar int64, scalarLeft bool, inner predRowShape, length int) (predRowShape, bool) {
	out := inner
	switch op {
	case OpAdd:
		base, ok := addInt64Checked(inner.base, scalar)
		if !ok {
			return predRowShape{}, false
		}
		out.base = base
	case OpSub:
		if scalarLeft {
			base, ok := addInt64Checked(scalar, -inner.base)
			if !ok || inner.base == minInt64Sentinel {
				return predRowShape{}, false
			}
			if inner.step == minInt64Sentinel {
				return predRowShape{}, false
			}
			out.base, out.step = base, -inner.step
		} else {
			if scalar == minInt64Sentinel {
				return predRowShape{}, false
			}
			base, ok := addInt64Checked(inner.base, -scalar)
			if !ok {
				return predRowShape{}, false
			}
			out.base = base
		}
	case OpMul:
		base, ok := mulInt64Checked(inner.base, scalar)
		if !ok {
			return predRowShape{}, false
		}
		step, ok := mulInt64Checked(inner.step, scalar)
		if !ok {
			return predRowShape{}, false
		}
		out.base, out.step = base, step
	default:
		return predRowShape{}, false
	}
	if !predAffineBoundsOK(out, length) {
		return predRowShape{}, false
	}
	return out, true
}

const minInt64Sentinel = int64(-1) << 63

func mulInt64Checked(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	p := a * b
	if p/b != a || (a == minInt64Sentinel && b == -1) || (b == minInt64Sentinel && a == -1) {
		return 0, false
	}
	return p, true
}

// predAffineBoundsOK verifies base + step*row cannot wrap for row < length.
func predAffineBoundsOK(shape predRowShape, length int) bool {
	if length <= 0 {
		return false
	}
	span, ok := mulInt64Checked(shape.step, int64(length-1))
	if !ok {
		return false
	}
	_, ok = addInt64Checked(shape.base, span)
	return ok
}

// predF64PeriodicSource describes a float compare leaf source of the shape
// `<periodic integer chain> <op> <scalar>` (either operand order): the float
// value is a pointwise function of a row-periodic integer, so the leaf is
// row-periodic and one period evaluates from the flattened integer prefix
// through the same applyNumericDyadicFloatNumbers kernel the row path uses.
type predF64PeriodicSource struct {
	intSide    Array
	scalar     float64
	scalarLeft bool
	op         string
}

// classifyPredRowShapeF64 resolves a float compare leaf source to a periodic
// row shape plus its evaluation descriptor. Sources arrive either with raw
// left/right operands or pre-bound producer pairs; both forms expose one
// integer-chain side and one scalar side.
func classifyPredRowShapeF64(array Array, length int) (predRowShape, predF64PeriodicSource, bool) {
	dyadic, ok := array.(f64NumericDyadicArray)
	if !ok || dyadic.len < length {
		return predRowShape{}, predF64PeriodicSource{}, false
	}
	desc, ok := predF64DyadicSides(dyadic)
	if !ok {
		return predRowShape{}, predF64PeriodicSource{}, false
	}
	inner, ok := classifyPredRowShapeI64(desc.intSide, length)
	if !ok || inner.kind != predRowShapePeriodic {
		return predRowShape{}, predF64PeriodicSource{}, false
	}
	return inner, desc, true
}

func predF64DyadicSides(dyadic f64NumericDyadicArray) (predF64PeriodicSource, bool) {
	desc := predF64PeriodicSource{op: dyadic.op}
	if dyadic.left != nil || dyadic.right != nil {
		leftArray, leftIsArray := dyadic.left.(Array)
		rightArray, rightIsArray := dyadic.right.(Array)
		if leftIsArray == rightIsArray {
			return predF64PeriodicSource{}, false
		}
		if leftIsArray {
			scalar, ok := f64CompareScalarValue(dyadic.right)
			if !ok {
				return predF64PeriodicSource{}, false
			}
			desc.intSide, desc.scalar = leftArray, scalar
		} else {
			scalar, ok := f64CompareScalarValue(dyadic.left)
			if !ok {
				return predF64PeriodicSource{}, false
			}
			desc.intSide, desc.scalar, desc.scalarLeft = rightArray, scalar, true
		}
		return desc, true
	}
	producer := dyadic.bound.producer
	if producer.apply == nil || producer.len != dyadic.len {
		return predF64PeriodicSource{}, false
	}
	leftArray, leftScalar, leftKind := predF64ProducerSide(producer.left)
	rightArray, rightScalar, rightKind := predF64ProducerSide(producer.right)
	if leftKind == predF64SideArray && rightKind == predF64SideScalar {
		desc.intSide, desc.scalar = leftArray, rightScalar
		return desc, true
	}
	if leftKind == predF64SideScalar && rightKind == predF64SideArray {
		desc.intSide, desc.scalar, desc.scalarLeft = rightArray, leftScalar, true
		return desc, true
	}
	return predF64PeriodicSource{}, false
}

type predF64SideKind uint8

const (
	predF64SideOther predF64SideKind = iota
	predF64SideArray
	predF64SideScalar
)

func predF64ProducerSide(producer f64NumericProducer) (Array, float64, predF64SideKind) {
	switch p := producer.(type) {
	case f64ScalarProducer:
		return nil, p.value, predF64SideScalar
	case f64I64ScalarDyadicProducer:
		return p.values, 0, predF64SideArray
	case f64I64RangeProducer:
		return p.values, 0, predF64SideArray
	default:
		return nil, 0, predF64SideOther
	}
}

// truncatePredRowSourceI64 rebuilds a row-local lazy integer chain at length
// n, so one period prefix flattens through the existing bulk kernels with
// semantics identical to the full-length flatten.
func truncatePredRowSourceI64(array Array, n int) (Array, bool) {
	switch a := array.(type) {
	case attributedArray:
		return truncatePredRowSourceI64(a.array, n)
	case i64RangeArray:
		if a.len < n {
			return nil, false
		}
		a.len = n
		return a, true
	case i64ScalarDyadicArray:
		if a.len < n {
			return nil, false
		}
		source, ok := truncatePredRowSourceI64(a.source, n)
		if !ok {
			return nil, false
		}
		a.source, a.len = source, n
		return a, true
	case i64BucketArray:
		if a.len < n {
			return nil, false
		}
		source, ok := truncatePredRowSourceI64(a.source, n)
		if !ok {
			return nil, false
		}
		a.source, a.len = source, n
		return a, true
	default:
		return nil, false
	}
}

// predAnalyticBundle accumulates the analytic form of a conjunctive tree.
type predAnalyticBundle struct {
	lo, hi         int64
	excludes       []int64
	includes       []int64
	hasIncludes    bool
	empty          bool
	periodicLeaves []*predNode
	f64Descs       map[*predNode]predF64PeriodicSource
	period         int64
}

func collectPredAndLeaves(node *predNode, leaves *[]*predNode) bool {
	switch node.kind {
	case predNodeAnd:
		return collectPredAndLeaves(node.left, leaves) && collectPredAndLeaves(node.right, leaves)
	case predNodeCmp, predNodeIn, predNodeWithin, predNodeCmpF64:
		*leaves = append(*leaves, node)
		return true
	default:
		return false
	}
}

// analyticWhereMask runs the analytic bundle over a standalone mask (used for
// single compare masks over derived chains that the modulo-plan lowering does
// not cover, e.g. `where px>=140` with `px:100+(x mod 80)`).
func analyticWhereMask(mask Array) (Array, bool) {
	length := mask.Len()
	if length == 0 {
		return nil, false
	}
	c := predCompiler{length: length, nodeBudget: predNodeBudget}
	root, ok := c.compile(mask, false)
	if !ok || root.kind == predNodeBools {
		c.releaseOwnedBools()
		return nil, false
	}
	c.foldSingleUseLeafChains(root)
	out, ok := c.tryAnalyticWhereIndexes(root)
	c.releaseOwnedBools()
	return out, ok
}

// tryAnalyticWhereIndexes lowers a pure AND tree of classifiable leaves to
// direct index emission. ok=false leaves the compiled tree to the existing
// per-leaf fused passes.
func (c *predCompiler) tryAnalyticWhereIndexes(root *predNode) (Array, bool) {
	leaves := make([]*predNode, 0, predNodeBudget)
	if !collectPredAndLeaves(root, &leaves) || len(leaves) == 0 {
		return nil, false
	}
	b := predAnalyticBundle{lo: 0, hi: int64(c.length), period: 1}
	for _, leaf := range leaves {
		if !c.classifyAnalyticLeaf(leaf, &b) {
			return nil, false
		}
	}
	if b.period > predAnalyticMaxPeriod || b.period > int64(c.length) {
		return nil, false
	}
	if b.lo < 0 {
		b.lo = 0
	}
	if b.hi > int64(c.length) {
		b.hi = int64(c.length)
	}
	if b.empty || b.lo >= b.hi || (b.hasIncludes && len(b.includes) == 0) {
		return i64RangeArray{len: 0}, true
	}

	var table []bool
	if len(b.periodicLeaves) > 0 {
		built, ok := c.buildPredResidueTable(&b, int(b.period))
		if !ok {
			return nil, false
		}
		table = built
		defer bulkBoolRelease(table, true)
	}
	return c.emitAnalyticIndexes(&b, table), true
}

// classifyAnalyticLeaf folds one leaf into the bundle: periodic leaves join
// the residue-table list, affine unit-stride leaves clamp the row interval or
// land in the include/exclude row sets.
func (c *predCompiler) classifyAnalyticLeaf(leaf *predNode, b *predAnalyticBundle) bool {
	if leaf.kind == predNodeCmpF64 {
		shape, desc, ok := classifyPredRowShapeF64(c.sourcesF64[leaf.src], c.length)
		if !ok {
			return false
		}
		if _, ok := numericDyadicFloatFunc(desc.op); !ok {
			return false
		}
		if !b.addPeriodicLeaf(leaf, shape.period) {
			return false
		}
		if b.f64Descs == nil {
			b.f64Descs = make(map[*predNode]predF64PeriodicSource, 2)
		}
		b.f64Descs[leaf] = desc
		return true
	}
	shape, ok := classifyPredRowShapeI64(c.sources[leaf.src], c.length)
	if !ok {
		return false
	}
	if shape.kind == predRowShapePeriodic {
		return b.addPeriodicLeaf(leaf, shape.period)
	}
	if leaf.modBy != 0 {
		if leaf.modBy <= 0 {
			return false
		}
		return b.addPeriodicLeaf(leaf, leaf.modBy)
	}
	if shape.step != 1 {
		return false
	}
	switch leaf.kind {
	case predNodeCmp:
		t, ok := addInt64Checked(leaf.scalar, -shape.base)
		if !ok || shape.base == minInt64Sentinel {
			return false
		}
		switch leaf.op {
		case OpEQ:
			b.intersectIncludes([]int64{t})
		case OpNE:
			b.excludes = append(b.excludes, t)
		case OpLT:
			b.clampHi(t)
		case OpLE:
			b.clampHi(predSaturatingInc(t))
		case OpGT:
			b.clampLo(predSaturatingInc(t))
		case OpGE:
			b.clampLo(t)
		default:
			return false
		}
		return true
	case predNodeWithin:
		if leaf.negate || shape.base == minInt64Sentinel {
			return false
		}
		lo, ok := addInt64Checked(leaf.scalar, -shape.base)
		if !ok {
			return false
		}
		hi, ok := addInt64Checked(leaf.hi, -shape.base)
		if !ok {
			return false
		}
		b.clampLo(lo)
		if leaf.hiClosed {
			hi = predSaturatingInc(hi)
		}
		b.clampHi(hi)
		return true
	case predNodeIn:
		if shape.base == minInt64Sentinel {
			return false
		}
		rows := make([]int64, 0, len(leaf.probes))
		for _, probe := range leaf.probes {
			row, ok := addInt64Checked(probe, -shape.base)
			if !ok {
				return false
			}
			rows = append(rows, row)
		}
		if leaf.negate {
			b.excludes = append(b.excludes, rows...)
		} else {
			b.intersectIncludes(rows)
		}
		return true
	default:
		return false
	}
}

func predSaturatingInc(v int64) int64 {
	if v == int64(^uint64(0)>>1) {
		return v
	}
	return v + 1
}

func (b *predAnalyticBundle) addPeriodicLeaf(leaf *predNode, period int64) bool {
	if period <= 0 {
		return false
	}
	combined, ok := lcmInt64(b.period, period)
	if !ok || combined <= 0 || combined > predAnalyticMaxPeriod {
		return false
	}
	b.period = combined
	b.periodicLeaves = append(b.periodicLeaves, leaf)
	return true
}

func (b *predAnalyticBundle) clampLo(lo int64) {
	if lo > b.lo {
		b.lo = lo
	}
}

func (b *predAnalyticBundle) clampHi(hi int64) {
	if hi < b.hi {
		b.hi = hi
	}
}

func (b *predAnalyticBundle) intersectIncludes(rows []int64) {
	if !b.hasIncludes {
		b.hasIncludes = true
		b.includes = rows
		return
	}
	kept := b.includes[:0]
	for _, row := range b.includes {
		if i64ProbesContain(rows, row) {
			kept = append(kept, row)
		}
	}
	b.includes = kept
}

// buildPredResidueTable bundles every periodic leaf into one accumulator pass
// over a single period prefix of its (shared) source: the lazy chains are
// rebuilt at length P, flattened once through the bulk kernels, and combined
// with the existing fused leaf loops.
func (c *predCompiler) buildPredResidueTable(b *predAnalyticBundle, period int) ([]bool, bool) {
	sources := make([][]int64, len(c.sources))
	sourcesOwned := make([]bool, len(c.sources))
	sourcesF64 := make([][]float64, len(b.periodicLeaves))
	release := func() {
		for i, values := range sources {
			if values != nil {
				bulkI64Release(values, sourcesOwned[i])
			}
		}
		for _, values := range sourcesF64 {
			if values != nil {
				bulkF64Release(values, true)
			}
		}
	}
	defer release()
	for i, leaf := range b.periodicLeaves {
		if leaf.kind == predNodeCmpF64 {
			values, ok := c.flattenPredF64Period(b.f64Descs[leaf], period)
			if !ok {
				return nil, false
			}
			sourcesF64[i] = values
			continue
		}
		if sources[leaf.src] != nil {
			continue
		}
		truncated, ok := truncatePredRowSourceI64(c.sources[leaf.src], period)
		if !ok {
			return nil, false
		}
		values, owned, ok := tryBulkI64Values(truncated)
		if !ok || len(values) < period {
			bulkI64Release(values, owned)
			return nil, false
		}
		sources[leaf.src] = values[:period]
		sourcesOwned[leaf.src] = owned
	}
	table := bulkBoolGet(period)
	for i, leaf := range b.periodicLeaves {
		mode := predCombineAnd
		if i == 0 {
			mode = predCombineSet
		}
		switch leaf.kind {
		case predNodeCmp:
			predCmpInto(table, sources[leaf.src], leaf.op, leaf.scalar, leaf.modBy, mode)
		case predNodeIn:
			predInInto(table, sources[leaf.src], leaf.probes, leaf.modBy, leaf.negate, mode)
		case predNodeWithin:
			predWithinInto(table, sources[leaf.src], leaf, mode)
		case predNodeCmpF64:
			predCmpF64Into(table, sourcesF64[i], leaf.op, leaf.fscalar, mode)
		default:
			bulkBoolRelease(table, true)
			return nil, false
		}
	}
	return table, true
}

// flattenPredF64Period evaluates one period prefix of a float compare leaf
// source: the periodic integer side flattens through the bulk kernels, then
// the scalar dyadic float op — the same kernel the per-row evaluation path
// applies — runs over the prefix.
func (c *predCompiler) flattenPredF64Period(desc predF64PeriodicSource, period int) ([]float64, bool) {
	apply, ok := numericDyadicFloatFunc(desc.op)
	if !ok {
		return nil, false
	}
	truncated, ok := truncatePredRowSourceI64(desc.intSide, period)
	if !ok {
		return nil, false
	}
	ints, owned, ok := tryBulkI64Values(truncated)
	if !ok || len(ints) < period {
		bulkI64Release(ints, owned)
		return nil, false
	}
	out := bulkF64Get(period)
	if desc.scalarLeft {
		for i, v := range ints[:period] {
			out[i] = apply(desc.scalar, float64(v))
		}
	} else {
		for i, v := range ints[:period] {
			out[i] = apply(float64(v), desc.scalar)
		}
	}
	bulkI64Release(ints, owned)
	return out, true
}

// emitAnalyticProgression lowers a single-residue bundle to stepped range
// segments: rows ≡ residue (mod period) inside [lo, hi), with each exclude
// splitting the progression. No dense index vector is materialized.
func emitAnalyticProgression(b *predAnalyticBundle, residue int64) (Array, bool) {
	period := b.period
	first := b.lo + ((residue-b.lo)%period+period)%period
	if first >= b.hi {
		return i64RangeArray{len: 0}, true
	}
	hits := make([]int64, 0, len(b.excludes))
	for _, row := range b.excludes {
		if row >= first && row < b.hi && (row-first)%period == 0 {
			hits = append(hits, row)
		}
	}
	if len(hits) == 0 {
		return i64RangeArray{start: first, step: period, len: int((b.hi-1-first)/period + 1)}, true
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i] < hits[j] })
	segments := make([]i64RangeArray, 0, len(hits)+1)
	start := first
	for _, hit := range hits {
		if hit < start {
			continue // duplicate exclude
		}
		if n := (hit - start) / period; n > 0 {
			segments = append(segments, i64RangeArray{start: start, step: period, len: int(n)})
		}
		start = hit + period
	}
	if start < b.hi {
		segments = append(segments, i64RangeArray{start: start, step: period, len: int((b.hi-1-start)/period + 1)})
	}
	return newI64SegmentArray(segments...), true
}

// emitAnalyticIndexes materializes the where result from the bundle:
// candidate rows walk the residue table inside the clamped interval, skipping
// the (tiny) exclude set. Fully periodic full-range results stay lazy as
// i64PeriodicIndexArray so downstream gather/reduce remains analytic.
func (c *predCompiler) emitAnalyticIndexes(b *predAnalyticBundle, table []bool) Array {
	length := int64(c.length)
	if b.hasIncludes {
		rows := append([]int64(nil), b.includes...)
		sort.Slice(rows, func(i, j int) bool { return rows[i] < rows[j] })
		out := make([]int64, 0, len(rows))
		for _, row := range rows {
			if row < b.lo || row >= b.hi {
				continue
			}
			if len(out) > 0 && out[len(out)-1] == row {
				continue
			}
			if len(b.excludes) > 0 && i64ProbesContain(b.excludes, row) {
				continue
			}
			if table != nil && !table[row%b.period] {
				continue
			}
			out = append(out, row)
		}
		return newI64Trusted(out)
	}
	if table == nil {
		if len(b.excludes) == 0 {
			return i64RangeArray{start: b.lo, step: 1, len: int(b.hi - b.lo)}
		}
		out := make([]int64, 0, b.hi-b.lo)
		for row := b.lo; row < b.hi; row++ {
			if i64ProbesContain(b.excludes, row) {
				continue
			}
			out = append(out, row)
		}
		return newI64Trusted(out)
	}
	residues := make([]int64, 0, b.period)
	for r := int64(0); r < b.period; r++ {
		if table[r] {
			residues = append(residues, r)
		}
	}
	if len(residues) == 0 {
		return i64RangeArray{len: 0}
	}
	if len(residues) == 1 {
		// Single-residue tables are arithmetic progressions; excludes split
		// the progression into stepped segments, so no dense vector is
		// materialized at all.
		if out, ok := emitAnalyticProgression(b, residues[0]); ok {
			return out
		}
	}
	if len(b.excludes) == 0 {
		if b.lo == 0 && b.hi == length {
			return newI64PeriodicIndexArray(b.period, residues, c.length)
		}
		// Clamped intervals stay analytic too: rebase the residue table to
		// the interval start and emit the (lazy) shifted periodic carrier
		// instead of a dense index vector.
		shift := b.lo % b.period
		rotated := make([]int64, 0, len(residues))
		for _, residue := range residues {
			rotated = append(rotated, (residue-shift+b.period)%b.period)
		}
		out := newI64PeriodicIndexArray(b.period, rotated, int(b.hi-b.lo))
		if b.lo == 0 || out.Len() == 0 {
			return out
		}
		return i64ScalarDyadicArray{source: out, op: OpAdd, scalar: b.lo, len: out.Len()}
	}
	scratch := bulkI64Get(int(b.hi - b.lo))
	count := 0
	for base := (b.lo / b.period) * b.period; base < b.hi; base += b.period {
		for _, residue := range residues {
			row := base + residue
			if row < b.lo {
				continue
			}
			if row >= b.hi {
				break
			}
			if len(b.excludes) > 0 && i64ProbesContain(b.excludes, row) {
				continue
			}
			scratch[count] = row
			count++
		}
	}
	out := make([]int64, count)
	copy(out, scratch[:count])
	bulkI64Release(scratch, true)
	return newI64Trusted(out)
}
