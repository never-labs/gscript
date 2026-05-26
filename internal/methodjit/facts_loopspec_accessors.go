package methodjit

// Additional loop-specialization-domain accessors layered on top of the
// LoopSpecializationFacts surface. These route table-array data-pointer facts
// through the same accessor pattern as the other loopspec facts.

// SetTableArrayDataPtrs replaces the table-array data-pointer fact map.
func (k *LoopSpecializationFacts) SetTableArrayDataPtrs(facts map[int]TableArrayDataPtrFact) {
	if k == nil {
		return
	}
	k.TableArrayDataPtrs = facts
	k.bindOwner()
}

// TableArrayDataPtr returns the data-pointer fact recorded for valueID, if any.
func (k *LoopSpecializationFacts) TableArrayDataPtr(valueID int) (TableArrayDataPtrFact, bool) {
	if k == nil || k.TableArrayDataPtrs == nil {
		return TableArrayDataPtrFact{}, false
	}
	fact, ok := k.TableArrayDataPtrs[valueID]
	return fact, ok
}
