package q

import (
	"fmt"
	"strings"
)

const evalSessionPlanCacheLimit = 512

// SessionPlannedEvalField is the reserved key under which bind layers expose a
// per-source planned-eval resolver on q session tables. Callers (MethodJIT's
// constant-source session-eval op-exit) invoke the resolver once per
// (session, source) pair and then re-execute the returned handle directly,
// skipping the per-call string-eval shell (TrimSpace + source-string plan
// cache probe + host eval field lookup) while running the exact same cached
// plan chain as EvalSession.Eval.
const SessionPlannedEvalField = "__planned_eval"

// EvalSession is a stateful q evaluator with a source-stable warm plan cache.
// It keeps q bindings in one EvalState while caching immutable parse/planning
// artifacts for repeated ordinary q eval source.
type EvalSession struct {
	state *EvalState
	cache map[string]*evalSessionPlan
	order []string
}

type evalSessionPlan struct {
	source     string
	script     qScriptPlan
	descriptor EvalPipelineDescriptor
	backend    EvalPipelineBackendPlan
	executable EvalPipelineExecutablePlan
}

// NewEvalSession constructs a stateful q session. The supplied env is cloned by
// EvalState, so callers keep ownership of their input map.
func NewEvalSession(env map[string]any) *EvalSession {
	return &EvalSession{state: NewEvalState(env)}
}

// Eval evaluates source in session order, reusing parsed script plans and
// predecoded typed-runtime pipeline plans for stable sources.
func (s *EvalSession) Eval(source string) (any, error) {
	if s == nil {
		return nil, nil
	}
	source = strings.TrimSpace(source)
	return s.evalPlanEntry(s.plan(source))
}

// EvalScalar evaluates source through scalar-only hot paths when possible.
// ok=false means callers should use Eval for the generic result path.
func (s *EvalSession) EvalScalar(source string) (EvalScalarResult, bool, error) {
	if s == nil {
		return EvalScalarResult{}, false, nil
	}
	source = strings.TrimSpace(source)
	return s.evalPlanEntryScalar(s.plan(source))
}

// evalPlanEntry executes one resolved plan entry through the normal script-plan
// dispatcher. Plan entries are source-derived and state-independent; all
// session state binds at execution time, so re-executing a pinned entry is
// semantically identical to re-resolving the source on every call.
func (s *EvalSession) evalPlanEntry(entry *evalSessionPlan) (any, error) {
	if entry == nil {
		return nil, nil
	}
	return s.state.evalScriptPlan(entry.script)
}

func (s *EvalSession) evalPlanEntryScalar(entry *evalSessionPlan) (EvalScalarResult, bool, error) {
	if entry == nil {
		return EvalScalarResult{}, false, nil
	}
	return s.state.evalScriptPlanScalar(entry.script)
}

// EvalSessionPlanned pins one source's cached plan entry so warm callers can
// re-execute the plan chain without the per-call resolution shell. The handle
// has the same execution semantics as EvalSession.Eval(source) for the pinned
// source — including per-call typed-kernel execution (no result memoization).
// Like EvalSession itself it is not safe for concurrent use; callers must
// serialize with the same lock that guards the owning session.
type EvalSessionPlanned struct {
	session *EvalSession
	entry   *evalSessionPlan
}

// Planned resolves source once and returns a pinned execution handle. The
// returned handle stays valid for the session's lifetime even if the session
// plan cache is later reset (the entry pointer is pinned).
func (s *EvalSession) Planned(source string) *EvalSessionPlanned {
	if s == nil {
		return nil
	}
	source = strings.TrimSpace(source)
	entry := s.plan(source)
	if entry == nil {
		return nil
	}
	return &EvalSessionPlanned{session: s, entry: entry}
}

// Eval executes the pinned plan chain against the owning session's state.
func (p *EvalSessionPlanned) Eval() (any, error) {
	if p == nil || p.session == nil || p.entry == nil {
		return nil, fmt.Errorf("q: planned session eval handle is empty")
	}
	return p.session.evalPlanEntry(p.entry)
}

// EvalScalar executes the pinned plan through scalar-only hot paths. ok=false
// means the caller should use Eval for the generic result path.
func (p *EvalSessionPlanned) EvalScalar() (EvalScalarResult, bool, error) {
	if p == nil || p.session == nil || p.entry == nil {
		return EvalScalarResult{}, false, fmt.Errorf("q: planned session eval handle is empty")
	}
	return p.session.evalPlanEntryScalar(p.entry)
}

func (s *EvalSession) plan(source string) *evalSessionPlan {
	if s.cache != nil {
		if entry, ok := s.cache[source]; ok {
			return entry
		}
	}
	entry := s.buildPlan(source)
	s.rememberPlan(source, entry)
	return entry
}

func (s *EvalSession) buildPlan(source string) *evalSessionPlan {
	plan := s.state.qScriptPlan(source)
	entry := &evalSessionPlan{source: source, script: plan}
	backend, executable, ok := s.state.compileEvalPipelineSource(source)
	if !ok {
		return entry
	}
	entry.descriptor = backend.Descriptor
	entry.backend = backend
	entry.executable = executable
	return entry
}

func (s *EvalSession) rememberPlan(source string, entry *evalSessionPlan) {
	if source == "" {
		return
	}
	if s.cache == nil {
		s.cache = make(map[string]*evalSessionPlan, 16)
	} else if len(s.cache) >= evalSessionPlanCacheLimit {
		s.cache = make(map[string]*evalSessionPlan, 16)
		s.order = s.order[:0]
	}
	if _, ok := s.cache[source]; !ok {
		s.order = append(s.order, source)
	}
	s.cache[source] = entry
}
