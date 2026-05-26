package methodjit

// Additional loop-specialization-domain accessors layered on top of the
// LoopSpecializationFacts surface. These route table-array data-pointer facts
// through the same accessor pattern as the other loopspec facts.

// SetTableArrayDataPtrs replaces the table-array data-pointer fact map.
func (k *LoopSpecializationFacts) SetTableArrayDataPtrs(facts map[int]TableArrayDataPtrFact) {
	if k == nil {
		return
	}
	k.tableArrayDataPtrs = facts
	k.bindOwner()
}

// TableArrayDataPtr returns the data-pointer fact recorded for valueID, if any.
func (k *LoopSpecializationFacts) TableArrayDataPtr(valueID int) (TableArrayDataPtrFact, bool) {
	if k == nil || k.tableArrayDataPtrs == nil {
		return TableArrayDataPtrFact{}, false
	}
	fact, ok := k.tableArrayDataPtrs[valueID]
	return fact, ok
}

// TableArrayDataPtrCount reports the number of recorded data-pointer facts.
func (k *LoopSpecializationFacts) TableArrayDataPtrCount() int {
	if k == nil {
		return 0
	}
	return len(k.tableArrayDataPtrs)
}

// ForEachTableArrayDataPtr visits each recorded data-pointer fact. Returning
// false from visit stops iteration.
func (k *LoopSpecializationFacts) ForEachTableArrayDataPtr(visit func(valueID int, fact TableArrayDataPtrFact) bool) {
	if k == nil || k.tableArrayDataPtrs == nil || visit == nil {
		return
	}
	for id, fact := range k.tableArrayDataPtrs {
		if !visit(id, fact) {
			return
		}
	}
}
