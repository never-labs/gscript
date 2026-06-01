package methodjit

import (
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// Accessors for cross-proto global/ABI input facts owned by GlobalFacts. These
// route domain access through GlobalFacts so callers do not touch the
// AnalysisResult struct fields directly.

// SetGlobals installs the global function name -> proto map. Nil is a
// meaningful sentinel (see GlobalFacts.Globals) and is preserved.
func (g *GlobalFacts) SetGlobals(globals map[string]*vm.FuncProto) {
	if g == nil {
		return
	}
	g.globals = globals
	g.bindOwner()
}

// GlobalsPopulated reports whether the Globals sentinel map has been installed.
func (g *GlobalFacts) GlobalsPopulated() bool {
	return g != nil && g.globals != nil
}

// GlobalsMap returns the underlying global function name -> proto map. It may be
// nil, which is a meaningful sentinel.
func (g *GlobalFacts) GlobalsMap() map[string]*vm.FuncProto {
	if g == nil {
		return nil
	}
	return g.globals
}

// GlobalProto returns the proto registered for the given global function name.
func (g *GlobalFacts) GlobalProto(name string) (*vm.FuncProto, bool) {
	if g == nil || g.globals == nil {
		return nil, false
	}
	p, ok := g.globals[name]
	return p, ok
}

// ForEachGlobal iterates the global function name -> proto map.
func (g *GlobalFacts) ForEachGlobal(fn func(name string, proto *vm.FuncProto)) {
	if g == nil || g.globals == nil || fn == nil {
		return
	}
	for name, proto := range g.globals {
		fn(name, proto)
	}
}

// SetNumericGlobalValues installs the stable global numeric values map.
func (g *GlobalFacts) SetNumericGlobalValues(values map[string]runtime.Value) {
	if g == nil {
		return
	}
	g.numericGlobalValues = values
	g.bindOwner()
}

// NumericGlobalValuesMap returns the underlying stable global numeric values map.
func (g *GlobalFacts) NumericGlobalValuesMap() map[string]runtime.Value {
	if g == nil {
		return nil
	}
	return g.numericGlobalValues
}

// SetGlobalArrayElementFacts installs the stable global array-element facts map.
func (g *GlobalFacts) SetGlobalArrayElementFacts(facts map[string]FixedShapeTableFact) {
	if g == nil {
		return
	}
	g.globalArrayElementFacts = facts
	g.bindOwner()
}

// GlobalArrayElementFactsMap returns the underlying stable global array-element
// facts map.
func (g *GlobalFacts) GlobalArrayElementFactsMap() map[string]FixedShapeTableFact {
	if g == nil {
		return nil
	}
	return g.globalArrayElementFacts
}
