// pass_context.go defines the domain-scoped PassContext: the execution-level
// counterpart to the test-time read/write contract. A pass migrated to the ctx
// form receives a PassContext whose domain accessors are gated by the set of
// fact domains the module declared (Requires ∪ Provides ∪ Updates ∪
// OptionalReads). With enforcement enabled (tests), reaching an undeclared
// domain panics; in production enforcement is off and the
// accessors are pure delegation to fn.Analysis with zero overhead.
//
// This is the keystone of the modular pass interface: declaring a fact domain
// is no longer only validated at test time, it is the only domain a ctx-form
// pass can physically reach when enforcement is on.

package methodjit

// passContextEnforce gates execution-level domain isolation. It is false in
// production builds so a PassContext is a thin delegating wrapper with no
// allow-set checks. Tests set it true (see pass_context_test.go) to assert that
// a migrated pass only touches the domains it declared.
var passContextEnforce bool

// PassContext is the domain-scoped handle a ctx-form optimization pass uses to
// reach the IR (Func) and the analysis fact domains it declared. Each domain
// accessor consults allowed when enforce is set; an undeclared access panics.
type PassContext struct {
	fn      *Function
	opts    *Tier2PipelineOpts
	allowed map[factDomain]bool
	enforce bool
}

// newPassContext builds a PassContext over fn/opts with the given allowed-domain
// set and enforcement flag.
func newPassContext(fn *Function, opts *Tier2PipelineOpts, allowed map[factDomain]bool, enforce bool) *PassContext {
	return &PassContext{fn: fn, opts: opts, allowed: allowed, enforce: enforce}
}

// Func returns the Function the pass operates on.
func (c *PassContext) Func() *Function { return c.fn }

// Opts returns the pipeline options for the current run (may be nil).
func (c *PassContext) Opts() *Tier2PipelineOpts { return c.opts }

// guard panics if enforcement is on and the domain was not declared/allowed.
func (c *PassContext) guard(d factDomain) {
	if c.enforce && !c.allowed[d] {
		panic("pass accessed undeclared domain " + string(d) +
			"; declare the owning fact in Requires/Provides/Updates or OptionalReads")
	}
}

// Numeric returns the numeric fact domain, gated by the allow-set.
func (c *PassContext) Numeric() *NumericFacts {
	c.guard(factDomainNumeric)
	if c.fn == nil || c.fn.Analysis == nil {
		return nil
	}
	return c.fn.Analysis.NumericFacts()
}

// Call returns the call fact domain, gated by the allow-set.
func (c *PassContext) Call() *CallFacts {
	c.guard(factDomainCall)
	if c.fn == nil || c.fn.Analysis == nil {
		return nil
	}
	return c.fn.Analysis.CallFacts()
}

// Speculation returns the speculation fact domain, gated by the allow-set.
func (c *PassContext) Speculation() *SpeculationFacts {
	c.guard(factDomainSpeculation)
	if c.fn == nil || c.fn.Analysis == nil {
		return nil
	}
	return c.fn.Analysis.SpeculationFacts()
}

// TableShape returns the table-shape fact domain, gated by the allow-set.
func (c *PassContext) TableShape() *TableShapeFacts {
	c.guard(factDomainTableShape)
	if c.fn == nil || c.fn.Analysis == nil {
		return nil
	}
	return c.fn.Analysis.TableShapeFacts()
}

// LoopSpecialization returns the loop-specialization fact domain, gated by the
// allow-set.
func (c *PassContext) LoopSpecialization() *LoopSpecializationFacts {
	c.guard(factDomainLoopSpec)
	if c.fn == nil || c.fn.Analysis == nil {
		return nil
	}
	return c.fn.Analysis.LoopSpecializationFacts()
}

// Global returns the global fact domain, gated by the allow-set.
func (c *PassContext) Global() *GlobalFacts {
	c.guard(factDomainGlobal)
	if c.fn == nil || c.fn.Analysis == nil {
		return nil
	}
	return c.fn.Analysis.GlobalFacts()
}

// allowedDomainsForModule computes the set of fact domains a module may access:
// the union of the owning domains of its declared facts (Requires ∪ Provides ∪
// Updates ∪ OptionalReads). OptionalReads are opportunistic hint reads that do
// not participate in dependency ordering.
func allowedDomainsForModule(requires, provides, updates []AnalysisFact, optionalReads ...[]AnalysisFact) map[factDomain]bool {
	allowed := make(map[factDomain]bool, 6)
	add := func(facts []AnalysisFact) {
		for _, f := range facts {
			if d, ok := analysisFactDomain[f]; ok {
				allowed[d] = true
			}
		}
	}
	add(requires)
	add(provides)
	add(updates)
	for _, facts := range optionalReads {
		add(facts)
	}
	return allowed
}
