package methodjit

var opFusableComparisonPolicies = [...]bool{
	OpEq:         true,
	OpLtInt:      true,
	OpLeInt:      true,
	OpEqInt:      true,
	OpModZeroInt: true,
	OpLtFloat:    true,
	OpLeFloat:    true,
}

var opModZeroCompareLoweredOpPolicies = [...]opTargetPolicy{
	OpEq:    {Op: OpModZeroInt, Set: true},
	OpEqInt: {Op: OpModZeroInt, Set: true},
}

var opLoopBoundComparisonPolicies = [...]bool{
	OpLtInt: true,
	OpLeInt: true,
	OpEqInt: true,
}
