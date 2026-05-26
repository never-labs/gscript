package methodjit

// IntNonNegativeMap returns the underlying set of integer SSA value IDs whose
// runtime result is provably >= 0. Callers iterate or read the map without
// mutating it; mutation must go through RecordIntNonNegative/SetIntNonNegative.
func (n *NumericFacts) IntNonNegativeMap() map[int]bool {
	if n == nil {
		return nil
	}
	return n.intNonNegative
}

// Int48SafeMap returns the underlying set of int48-safe SSA value IDs. Callers
// read or iterate without mutating; mutation goes through
// RecordInt48Safe/SetInt48Safe.
func (n *NumericFacts) Int48SafeMap() map[int]bool {
	if n == nil {
		return nil
	}
	return n.int48Safe
}

// RecordInt48Safe marks one SSA value ID as int48-safe.
func (n *NumericFacts) RecordInt48Safe(id int) {
	if n == nil {
		return
	}
	if n.int48Safe == nil {
		n.int48Safe = make(map[int]bool)
	}
	n.int48Safe[id] = true
	n.bindOwner()
}

// RecordIntRange records one integer range fact, unconditionally (the caller is
// responsible for the range's validity).
func (n *NumericFacts) RecordIntRange(id int, r intRange) {
	if n == nil {
		return
	}
	if n.intRanges == nil {
		n.intRanges = make(map[int]intRange)
	}
	n.intRanges[id] = r
	n.bindOwner()
}
