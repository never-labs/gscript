package methodjit

var expandedOpSpecs = buildExpandedOpSpecs()
var opNameLookup = buildOpNameLookup(expandedOpSpecs)

func buildOpNameLookup(specs [OpMax]OpSpec) map[string]Op {
	out := make(map[string]Op, int(OpMax))
	for op := Op(0); op < OpMax; op++ {
		if specs[op].Name != "" {
			out[specs[op].Name] = op
		}
	}
	return out
}
