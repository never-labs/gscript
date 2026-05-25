package methodjit

// IntNonNegativeMap returns the underlying set of integer SSA value IDs whose
// runtime result is provably >= 0. Callers iterate or read the map without
// mutating it; mutation must go through RecordIntNonNegative/SetIntNonNegative.
func (n *NumericFacts) IntNonNegativeMap() map[int]bool {
	if n == nil {
		return nil
	}
	return n.IntNonNegative
}
