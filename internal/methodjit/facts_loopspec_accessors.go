package methodjit

// Additional loop-specialization-domain accessors layered on top of the
// LoopSpecializationFacts surface. These route table-array data-pointer facts
// through the same accessor pattern as the other loopspec facts so callers no
// longer touch the flat AnalysisResult compatibility fields directly.
//
// TableArrayDataPtrs is not yet a field on LoopSpecializationFacts; it remains
// backed by the owning AnalysisResult's compatibility field. These accessors
// preserve the exact nil-map semantics of direct field access.

// SetTableArrayDataPtrs replaces the table-array data-pointer fact map.
func (k *LoopSpecializationFacts) SetTableArrayDataPtrs(facts map[int]TableArrayDataPtrFact) {
	if k == nil || k.owner == nil {
		return
	}
	k.owner.TableArrayDataPtrs = facts
	k.bindOwner()
}

// TableArrayDataPtr returns the data-pointer fact recorded for valueID, if any.
// It mirrors a direct nil-checked map read on the flat field.
func (k *LoopSpecializationFacts) TableArrayDataPtr(valueID int) (TableArrayDataPtrFact, bool) {
	if k == nil || k.owner == nil || k.owner.TableArrayDataPtrs == nil {
		return TableArrayDataPtrFact{}, false
	}
	fact, ok := k.owner.TableArrayDataPtrs[valueID]
	return fact, ok
}
