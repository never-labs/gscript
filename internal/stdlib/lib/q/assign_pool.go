package q

import (
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// Assignment statement-slot pooling.
//
// Session-warm q.eval scripts re-execute the same statement list every call.
// Statements that assign a lazy long/float carrier (`x:((til n) mod 8)*0.25`)
// otherwise force every downstream statement to re-flatten the carrier tree;
// eagerly materializing into fresh allocations was measured to regress via
// 64KB-class GC churn. The assignment pool materializes such carriers into
// buffers owned by per-name slots on the EvalState and recycles a buffer only
// two assignment generations after it was handed out.
//
// Safety contract (why two-generation recycling is sound):
//   - Pooling activates only while the SAME script plan is re-executed
//     end-to-end at top level (consecutive identical sources, depth 1).
//     Scripts are linear statement lists: every statement re-runs every
//     generation, so any env value derived from a pooled column is
//     recomputed each generation. Nothing observable can still reference a
//     buffer handed out two generations ago.
//   - Names assigned more than once per script are excluded (their slots
//     would rotate faster than one generation).
//   - The final script result must be a scalar; a non-scalar result could
//     carry a pooled reference out of the session, so the pool orphans its
//     buffers (drops them to the GC without recycling) and bars the source.
//   - Any anomaly — source switch, nested eval (lambdas, `value`, IPC),
//     evaluation error, system statements — likewise orphans instead of
//     recycling. Orphaned buffers stay sole-owned by the arrays that hold
//     them, so correctness never depends on the gating heuristics.
//
// This is buffer reuse, not result memoization: values are recomputed on
// every call; only backing storage is recycled.

type qAssignPoolSlot struct {
	prev data.OwnedNumericBuffer
	last data.OwnedNumericBuffer
}

type qAssignPool struct {
	key    *qScriptStatement // identity of the current script plan
	runs   int               // consecutive top-level runs of key's script
	barred bool              // pooling disabled for key's script
	active bool              // pooling enabled for the current run
	skip   map[string]bool   // resolved names excluded from pooling
	slots  map[string]*qAssignPoolSlot
}

// orphan forgets all pooled buffers without recycling them: every handed-out
// column keeps sole ownership of its storage and decays to ordinary GC-owned
// data. This is the universally safe exit used on any anomaly.
func (p *qAssignPool) orphan() {
	p.slots = nil
	p.active = false
}

// assignPoolBegin runs at the top of every top-level script-plan execution.
func (s *EvalState) assignPoolBegin(plan *qScriptPlan) {
	p := &s.assignPool
	var key *qScriptStatement
	if len(plan.statements) > 1 && !s.oneShot {
		key = &plan.statements[0]
	}
	if key == nil {
		if p.key != nil {
			p.orphan()
			p.key = nil
		}
		return
	}
	if p.key != key {
		p.orphan()
		p.key = key
		p.runs = 1
		p.barred = false
		p.skip = nil
		return
	}
	p.runs++
	if p.barred {
		return
	}
	if p.skip == nil {
		skip, ok := s.buildAssignPoolSkipSet(plan.statements)
		if !ok {
			p.barred = true
			return
		}
		p.skip = skip
	}
	p.active = true
}

// assignPoolEnd closes the current top-level run: errors and non-scalar
// results orphan the pool and bar the source.
func (s *EvalState) assignPoolEnd(out any, err error) {
	p := &s.assignPool
	if p.key == nil {
		return
	}
	if err != nil || !qAssignPoolScalarResult(out) {
		p.orphan()
		p.barred = true
		return
	}
	p.active = false
}

// assignPoolNested invalidates pooling when script evaluation re-enters
// (lambdas, `value`, IPC handlers): nested scripts may capture references on
// schedules the linear-generation contract cannot see.
func (s *EvalState) assignPoolNested() {
	p := &s.assignPool
	if p.key == nil {
		return
	}
	p.orphan()
	p.barred = true
}

// buildAssignPoolSkipSet excludes names assigned more than once per script
// (their slots would rotate faster than one generation). Scripts containing
// system statements (`\d` can re-point name resolution mid-script) decline
// pooling entirely.
func (s *EvalState) buildAssignPoolSkipSet(statements []qScriptStatement) (map[string]bool, bool) {
	counts := make(map[string]int, len(statements))
	for i := range statements {
		stmt := &statements[i]
		if strings.HasPrefix(stmt.src, "\\") {
			return nil, false
		}
		if stmt.assign == "" {
			continue
		}
		counts[s.resolveAssignmentName(stmt.assign)]++
	}
	skip := make(map[string]bool, 2)
	for name, n := range counts {
		if n > 1 {
			skip[name] = true
		}
	}
	return skip, true
}

// assignPoolMaterialize converts an assigned lazy long/float carrier into a
// dense column backed by the name's slot buffers, recycling the buffer handed
// out two generations ago. Values that decline materialization pass through
// unchanged.
func (s *EvalState) assignPoolMaterialize(name string, v any) any {
	p := &s.assignPool
	array, ok := v.(data.Array)
	if !ok {
		return v
	}
	out, buf, ok := data.MaterializeOwnedNumericColumn(array)
	if !ok {
		return v
	}
	if p.skip[name] || (s.deferScanAssignments != nil && s.deferScanAssignments[name]) {
		// Excluded names keep their original value; the freshly flattened
		// buffer goes straight back to the bulk pool.
		buf.Release()
		return v
	}
	if p.slots == nil {
		p.slots = make(map[string]*qAssignPoolSlot, 4)
	}
	slot := p.slots[name]
	if slot == nil {
		slot = &qAssignPoolSlot{}
		p.slots[name] = slot
	}
	// Recycle the buffer handed out two generations ago; everything derived
	// from it has been recomputed since (see the safety contract above).
	slot.prev.Release()
	slot.prev = slot.last
	slot.last = buf
	return out
}

// qFillsFillPlan is the syntactic shape of `<long atom>^fills <name>`
// assignment statements. The combined kernel computes the forward fill and
// the scalar fill in one pass written directly into the name's slot buffer,
// eliminating both per-call dense expansions.
type qFillsFillPlan struct {
	fill   int64
	source string
}

// buildQFillsFillPlan recognizes `<long atom>^fills <bare name>`.
func buildQFillsFillPlan(rhs string) *qFillsFillPlan {
	caret := findTopLevel(rhs, "^")
	if caret <= 0 {
		return nil
	}
	fillText := strings.TrimSpace(rhs[:caret])
	fill, err := strconv.ParseInt(fillText, 10, 64)
	if err != nil {
		return nil
	}
	rest := strings.TrimSpace(rhs[caret+1:])
	source, ok := strings.CutPrefix(rest, "fills ")
	if !ok {
		return nil
	}
	source = strings.TrimSpace(source)
	if !isQAssignmentName(source) {
		return nil
	}
	return &qFillsFillPlan{fill: fill, source: source}
}

// tryEvalAssignPoolFillsFill runs the fused fills→scalar-fill kernel for a
// pool-eligible assignment, writing into the slot buffer rotated under the
// standard two-generation contract. Declines (handled=false) fall back to
// the generic statement path with identical semantics; the source operand is
// a bare name, so the lookup here is pure and repeatable.
func (s *EvalState) tryEvalAssignPoolFillsFill(stmt *qScriptStatement, name string) (any, bool) {
	if !stmt.fillsFillChecked {
		stmt.fillsFillChecked = true
		stmt.fillsFillPlan = buildQFillsFillPlan(stmt.rhs)
	}
	plan := stmt.fillsFillPlan
	if plan == nil {
		return nil, false
	}
	p := &s.assignPool
	if p.skip[name] {
		return nil, false
	}
	if s.deferScanAssignments != nil && s.deferScanAssignments[name] {
		return nil, false
	}
	value, ok := s.lookupName(plan.source)
	if !ok {
		return nil, false
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false
	}
	if p.slots == nil {
		p.slots = make(map[string]*qAssignPoolSlot, 4)
	}
	slot := p.slots[name]
	if slot == nil {
		slot = &qAssignPoolSlot{}
		p.slots[name] = slot
	}
	// Write into the buffer handed out two generations ago (see the safety
	// contract above), then rotate it back to the front.
	out, buf, ok := data.FillsScalarFillI64Into(array, plan.fill, slot.prev)
	if !ok {
		slot.prev = buf
		return nil, false
	}
	slot.prev = slot.last
	slot.last = buf
	return out, true
}

// qAssignPoolScalarResult reports whether a script result is a plain scalar
// that cannot carry a reference to pooled storage out of the session.
func qAssignPoolScalarResult(v any) bool {
	switch v.(type) {
	case nil, bool, int, int64, float32, float64, string, data.Symbol,
		data.Month, data.Date, data.DateTime, data.Timespan,
		data.Minute, data.Second, data.Time, data.Timestamp:
		return true
	default:
		return false
	}
}
