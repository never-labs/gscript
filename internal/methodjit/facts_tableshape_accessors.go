package methodjit

// Accessors for table/shape facts owned by TableShapeFacts. These route domain
// access through TableShapeFacts so single-domain passes do not touch the
// AnalysisResult struct fields directly.

// FieldPolyShapeFactsMap returns the underlying guarded polymorphic field-cache
// map keyed by instruction ID. Callers read or iterate without mutating;
// mutation goes through RecordFieldPolyShapeCases/DeleteFieldPolyShapeCases.
func (t *TableShapeFacts) FieldPolyShapeFactsMap() map[int][]FieldPolyShapeCase {
	if t == nil {
		return nil
	}
	return t.fieldPolyShapeFacts
}

// FieldCallPolyLenFusionMap returns the underlying field-call/field-len fusion
// map keyed by instruction ID. Callers read or iterate without mutating;
// mutation goes through RecordFieldCallPolyLenFusions.
func (t *TableShapeFacts) FieldCallPolyLenFusionMap() map[int][]FieldCallPolyLenFusion {
	if t == nil {
		return nil
	}
	return t.fieldCallPolyLenFusions
}

// FixedRecordNewTableSitesPopulated reports whether the FixedRecordNewTableSites
// map has been computed for the owning analysis result.
func (t *TableShapeFacts) FixedRecordNewTableSitesPopulated() bool {
	return t != nil && t.fixedRecordNewTableSites != nil
}

// SetFixedRecordNewTableSites installs the computed FixedRecordNewTableSites map.
func (t *TableShapeFacts) SetFixedRecordNewTableSites(sites map[int]bool) {
	if t == nil {
		return
	}
	t.fixedRecordNewTableSites = sites
	t.bindOwner()
}

// FixedRecordNewTableSite reports whether the given OpNewFixedTable instruction
// ID remains a local fixed-record construction site.
func (t *TableShapeFacts) FixedRecordNewTableSite(id int) bool {
	if t == nil || t.fixedRecordNewTableSites == nil {
		return false
	}
	return t.fixedRecordNewTableSites[id]
}

// FixedRecordNewTableSiteMap returns the underlying FixedRecordNewTableSites map.
func (t *TableShapeFacts) FixedRecordNewTableSiteMap() map[int]bool {
	if t == nil {
		return nil
	}
	return t.fixedRecordNewTableSites
}

// FixedTableConstructorCount returns the number of recorded fixed table
// constructor facts.
func (t *TableShapeFacts) FixedTableConstructorCount() int {
	if t == nil {
		return 0
	}
	return len(t.fixedTableConstructors)
}

// FixedTableConstructorFact returns the fixed table constructor fact for the
// given OpNewTable instruction ID.
func (t *TableShapeFacts) FixedTableConstructorFact(id int) (FixedTableConstructorFact, bool) {
	if t == nil || t.fixedTableConstructors == nil {
		return FixedTableConstructorFact{}, false
	}
	fact, ok := t.fixedTableConstructors[id]
	return fact, ok
}

// RecordFixedTableConstructor records a fixed table constructor fact for the
// given OpNewTable instruction ID, lazily allocating the map.
func (t *TableShapeFacts) RecordFixedTableConstructor(id int, fact FixedTableConstructorFact) {
	if t == nil {
		return
	}
	if t.fixedTableConstructors == nil {
		t.fixedTableConstructors = make(map[int]FixedTableConstructorFact)
	}
	t.fixedTableConstructors[id] = fact
	t.bindOwner()
}

// ForEachFixedTableConstructor iterates the recorded fixed table constructor
// facts.
func (t *TableShapeFacts) ForEachFixedTableConstructor(visit func(id int, fact FixedTableConstructorFact) bool) {
	if t == nil || t.fixedTableConstructors == nil || visit == nil {
		return
	}
	for id, fact := range t.fixedTableConstructors {
		if !visit(id, fact) {
			return
		}
	}
}

// FixedShapeTableFactFor returns the fixed-shape table fact for the given SSA
// value ID.
func (t *TableShapeFacts) FixedShapeTableFactFor(id int) (FixedShapeTableFact, bool) {
	if t == nil || t.fixedShapeTables == nil {
		return FixedShapeTableFact{}, false
	}
	fact, ok := t.fixedShapeTables[id]
	return fact, ok
}

// SetFixedShapeTables installs the computed FixedShapeTables map.
func (t *TableShapeFacts) SetFixedShapeTables(facts map[int]FixedShapeTableFact) {
	if t == nil {
		return
	}
	t.fixedShapeTables = facts
	t.bindOwner()
}

// FixedShapeTableMap returns the underlying FixedShapeTables map.
func (t *TableShapeFacts) FixedShapeTableMap() map[int]FixedShapeTableFact {
	if t == nil {
		return nil
	}
	return t.fixedShapeTables
}

// FixedShapeArgFact returns the guarded fixed-shape fact for the given parameter
// index.
func (t *TableShapeFacts) FixedShapeArgFact(idx int) (FixedShapeTableFact, bool) {
	if t == nil || t.fixedShapeArgFacts == nil {
		return FixedShapeTableFact{}, false
	}
	fact, ok := t.fixedShapeArgFacts[idx]
	return fact, ok
}

// RecordFixedShapeArgFact records a guarded fixed-shape fact for the given
// parameter index, lazily allocating the map.
func (t *TableShapeFacts) RecordFixedShapeArgFact(idx int, fact FixedShapeTableFact) {
	if t == nil {
		return
	}
	if t.fixedShapeArgFacts == nil {
		t.fixedShapeArgFacts = make(map[int]FixedShapeTableFact)
	}
	t.fixedShapeArgFacts[idx] = fact
	t.bindOwner()
}

// FixedShapeArgFactMap returns the underlying FixedShapeArgFacts map.
func (t *TableShapeFacts) FixedShapeArgFactMap() map[int]FixedShapeTableFact {
	if t == nil {
		return nil
	}
	return t.fixedShapeArgFacts
}

// RecordFixedShapeEntryGuard records a parameter shape entry guard for the given
// parameter index, lazily allocating the map.
func (t *TableShapeFacts) RecordFixedShapeEntryGuard(idx int, fact FixedShapeTableFact) {
	if t == nil {
		return
	}
	if t.fixedShapeEntryGuards == nil {
		t.fixedShapeEntryGuards = make(map[int]FixedShapeTableFact)
	}
	t.fixedShapeEntryGuards[idx] = fact
	t.bindOwner()
}

// SetFixedShapeEntryGuards installs the computed FixedShapeEntryGuards map.
func (t *TableShapeFacts) SetFixedShapeEntryGuards(facts map[int]FixedShapeTableFact) {
	if t == nil {
		return
	}
	t.fixedShapeEntryGuards = facts
	t.bindOwner()
}

// FixedShapeEntryGuardMap returns the underlying FixedShapeEntryGuards map.
func (t *TableShapeFacts) FixedShapeEntryGuardMap() map[int]FixedShapeTableFact {
	if t == nil {
		return nil
	}
	return t.fixedShapeEntryGuards
}

// FixedShapeEntryGuard returns the entry-guard fact recorded for the given
// parameter index, if any.
func (t *TableShapeFacts) FixedShapeEntryGuard(idx int) (FixedShapeTableFact, bool) {
	if t == nil || t.fixedShapeEntryGuards == nil {
		return FixedShapeTableFact{}, false
	}
	fact, ok := t.fixedShapeEntryGuards[idx]
	return fact, ok
}

// ShapeFieldTypeElidedLoad reports whether the given field-load instruction ID
// has its result type guarded once and may skip the per-load tag check.
func (t *TableShapeFacts) ShapeFieldTypeElidedLoad(id int) bool {
	if t == nil || t.shapeFieldTypeElidedLoads == nil {
		return false
	}
	return t.shapeFieldTypeElidedLoads[id]
}

// RecordShapeFieldTypeElidedLoad marks the given field-load instruction ID as
// having its result type guarded once, lazily allocating the map.
func (t *TableShapeFacts) RecordShapeFieldTypeElidedLoad(id int) {
	if t == nil {
		return
	}
	if t.shapeFieldTypeElidedLoads == nil {
		t.shapeFieldTypeElidedLoads = make(map[int]bool)
	}
	t.shapeFieldTypeElidedLoads[id] = true
	t.bindOwner()
}
