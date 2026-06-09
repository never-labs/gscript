package q

import "strings"

const evalSessionPlanCacheLimit = 512

// EvalSession is a stateful q evaluator with a source-stable warm plan cache.
// It keeps q bindings in one EvalState while caching immutable parse/planning
// artifacts for repeated ordinary q eval source.
type EvalSession struct {
	state *EvalState
	cache map[string]evalSessionPlan
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
	entry := s.plan(source)
	if entry.executable.Valid() {
		if out, handled, err := s.state.ExecuteEvalPipelineExecutablePlan(entry.executable); err != nil || handled {
			return out, err
		}
	}
	return s.state.evalScriptPlan(entry.script)
}

func (s *EvalSession) plan(source string) evalSessionPlan {
	if s.cache != nil {
		if entry, ok := s.cache[source]; ok {
			return entry
		}
	}
	entry := s.buildPlan(source)
	s.rememberPlan(source, entry)
	return entry
}

func (s *EvalSession) buildPlan(source string) evalSessionPlan {
	plan := s.state.qScriptPlan(source)
	entry := evalSessionPlan{source: source, script: plan}
	if plan.scriptPipeline == nil {
		return entry
	}
	descriptor := evalScriptPipelineDescriptor(source, plan.scriptPipeline)
	backend := EvalPipelineBackendPlan{
		Backend:    EvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	}
	executable, _ := CompileEvalPipelineBackendPlan(backend)
	entry.descriptor = descriptor
	entry.backend = backend
	entry.executable = executable
	return entry
}

func (s *EvalSession) rememberPlan(source string, entry evalSessionPlan) {
	if source == "" {
		return
	}
	if s.cache == nil {
		s.cache = make(map[string]evalSessionPlan, 16)
	} else if len(s.cache) >= evalSessionPlanCacheLimit {
		s.cache = make(map[string]evalSessionPlan, 16)
		s.order = s.order[:0]
	}
	if _, ok := s.cache[source]; !ok {
		s.order = append(s.order, source)
	}
	s.cache[source] = entry
}
