package q

// SupportedEvalForms returns the curated list of non-verb special forms the
// q.eval expression interpreter supports. Verbs (unary/dyadic words and symbol
// operators) are intentionally excluded: those are derived mechanically from
// the lookupUnaryVerb / lookupDyadicVerb / lookupDyadicVerbFunc dispatch
// tables in eval.go. This list covers everything else — control flow, adverbs,
// apply forms, literals, and callable-shaping forms — that benchmark coverage
// gates must account for.
//
// Every name listed here must evaluate successfully through Eval; see
// TestSupportedEvalFormsAllEvaluate, which keeps the list free of dead names.
func SupportedEvalForms() []string {
	return []string{
		// Control flow special forms (evalControlSpecialForm,
		// evalConditionalSpecialForm).
		"if",
		"do",
		"while",
		"cond", // $[c;t;f] / ?[c;t;f]

		// Adverbs.
		"each",
		"each-prior", // f':
		"each-left",  // f\:
		"each-right", // f/:
		"over",       // f/
		"scan",       // f\

		// Apply forms. The 3-argument shapes (@[f;x;e] trap, @[x;i;f] unary
		// amend, and the .[...] equivalents) and the 'x signal form ride the
		// same evaluator entries; see trap_signal.go. They are intentionally
		// not separate names here: this list feeds the benchmark coverage
		// gate, and error-path forms are not benchmark surface.
		"apply-at",  // @[x;i] / @[x;i;f] / @[x;i;f;y]
		"apply-dot", // .[f;args] / .[x;i;f] / .[x;i;f;y]

		// Literal constructors.
		"dict-literal",  // `a`b!1 2
		"table-literal", // ([] a:1 2; b:3 4)

		// Callable-shaping and query forms.
		"fby",
		"projection",  // f[x;] partial application
		"composition", // (f g) juxtaposition
	}
}
