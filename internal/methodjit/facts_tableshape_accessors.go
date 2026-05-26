package methodjit

// Accessors for table/shape facts owned by TableShapeFacts. These route domain
// access through TableShapeFacts so single-domain passes do not touch the
// AnalysisResult struct fields directly.

// FixedRecordNewTableSitesPopulated reports whether the FixedRecordNewTableSites
// map has been computed for the owning analysis result.
func (t *TableShapeFacts) FixedRecordNewTableSitesPopulated() bool {
	return t != nil && t.FixedRecordNewTableSites != nil
}

// SetFixedRecordNewTableSites installs the computed FixedRecordNewTableSites map.
func (t *TableShapeFacts) SetFixedRecordNewTableSites(sites map[int]bool) {
	if t == nil {
		return
	}
	t.FixedRecordNewTableSites = sites
	t.bindOwner()
}

// FixedRecordNewTableSite reports whether the given OpNewFixedTable instruction
// ID remains a local fixed-record construction site.
func (t *TableShapeFacts) FixedRecordNewTableSite(id int) bool {
	if t == nil || t.FixedRecordNewTableSites == nil {
		return false
	}
	return t.FixedRecordNewTableSites[id]
}

// FixedRecordNewTableSiteMap returns the underlying FixedRecordNewTableSites map.
func (t *TableShapeFacts) FixedRecordNewTableSiteMap() map[int]bool {
	if t == nil {
		return nil
	}
	return t.FixedRecordNewTableSites
}

// FixedTableConstructorCount returns the number of recorded fixed table
// constructor facts.
func (t *TableShapeFacts) FixedTableConstructorCount() int {
	if t == nil {
		return 0
	}
	return len(t.FixedTableConstructors)
}

// FixedTableConstructorFact returns the fixed table constructor fact for the
// given OpNewTable instruction ID.
func (t *TableShapeFacts) FixedTableConstructorFact(id int) (FixedTableConstructorFact, bool) {
	if t == nil || t.FixedTableConstructors == nil {
		return FixedTableConstructorFact{}, false
	}
	fact, ok := t.FixedTableConstructors[id]
	return fact, ok
}

// RecordFixedTableConstructor records a fixed table constructor fact for the
// given OpNewTable instruction ID, lazily allocating the map.
func (t *TableShapeFacts) RecordFixedTableConstructor(id int, fact FixedTableConstructorFact) {
	if t == nil {
		return
	}
	if t.FixedTableConstructors == nil {
		t.FixedTableConstructors = make(map[int]FixedTableConstructorFact)
	}
	t.FixedTableConstructors[id] = fact
	t.bindOwner()
}

// ForEachFixedTableConstructor iterates the recorded fixed table constructor
// facts.
func (t *TableShapeFacts) ForEachFixedTableConstructor(visit func(id int, fact FixedTableConstructorFact) bool) {
	if t == nil || t.FixedTableConstructors == nil || visit == nil {
		return
	}
	for id, fact := range t.FixedTableConstructors {
		if !visit(id, fact) {
			return
		}
	}
}

// FixedShapeTableFactFor returns the fixed-shape table fact for the given SSA
// value ID.
func (t *TableShapeFacts) FixedShapeTableFactFor(id int) (FixedShapeTableFact, bool) {
	if t == nil || t.FixedShapeTables == nil {
		return FixedShapeTableFact{}, false
	}
	fact, ok := t.FixedShapeTables[id]
	return fact, ok
}

// SetFixedShapeTables installs the computed FixedShapeTables map.
func (t *TableShapeFacts) SetFixedShapeTables(facts map[int]FixedShapeTableFact) {
	if t == nil {
		return
	}
	t.FixedShapeTables = facts
	t.bindOwner()
}

// FixedShapeTableMap returns the underlying FixedShapeTables map.
func (t *TableShapeFacts) FixedShapeTableMap() map[int]FixedShapeTableFact {
	if t == nil {
		return nil
	}
	return t.FixedShapeTables
}

// FixedShapeArgFact returns the guarded fixed-shape fact for the given parameter
// index.
func (t *TableShapeFacts) FixedShapeArgFact(idx int) (FixedShapeTableFact, bool) {
	if t == nil || t.FixedShapeArgFacts == nil {
		return FixedShapeTableFact{}, false
	}
	fact, ok := t.FixedShapeArgFacts[idx]
	return fact, ok
}

// RecordFixedShapeArgFact records a guarded fixed-shape fact for the given
// parameter index, lazily allocating the map.
func (t *TableShapeFacts) RecordFixedShapeArgFact(idx int, fact FixedShapeTableFact) {
	if t == nil {
		return
	}
	if t.FixedShapeArgFacts == nil {
		t.FixedShapeArgFacts = make(map[int]FixedShapeTableFact)
	}
	t.FixedShapeArgFacts[idx] = fact
	t.bindOwner()
}

// FixedShapeArgFactMap returns the underlying FixedShapeArgFacts map.
func (t *TableShapeFacts) FixedShapeArgFactMap() map[int]FixedShapeTableFact {
	if t == nil {
		return nil
	}
	return t.FixedShapeArgFacts
}

// RecordFixedShapeEntryGuard records a parameter shape entry guard for the given
// parameter index, lazily allocating the map.
func (t *TableShapeFacts) RecordFixedShapeEntryGuard(idx int, fact FixedShapeTableFact) {
	if t == nil {
		return
	}
	if t.FixedShapeEntryGuards == nil {
		t.FixedShapeEntryGuards = make(map[int]FixedShapeTableFact)
	}
	t.FixedShapeEntryGuards[idx] = fact
	t.bindOwner()
}

// FixedShapeEntryGuardMap returns the underlying FixedShapeEntryGuards map.
func (t *TableShapeFacts) FixedShapeEntryGuardMap() map[int]FixedShapeTableFact {
	if t == nil {
		return nil
	}
	return t.FixedShapeEntryGuards
}

// ShapeFieldTypeElidedLoad reports whether the given field-load instruction ID
// has its result type guarded once and may skip the per-load tag check.
func (t *TableShapeFacts) ShapeFieldTypeElidedLoad(id int) bool {
	if t == nil || t.ShapeFieldTypeElidedLoads == nil {
		return false
	}
	return t.ShapeFieldTypeElidedLoads[id]
}

// RecordShapeFieldTypeElidedLoad marks the given field-load instruction ID as
// having its result type guarded once, lazily allocating the map.
func (t *TableShapeFacts) RecordShapeFieldTypeElidedLoad(id int) {
	if t == nil {
		return
	}
	if t.ShapeFieldTypeElidedLoads == nil {
		t.ShapeFieldTypeElidedLoads = make(map[int]bool)
	}
	t.ShapeFieldTypeElidedLoads[id] = true
	t.bindOwner()
}
