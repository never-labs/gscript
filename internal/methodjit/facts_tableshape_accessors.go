package methodjit

// Accessors for table/shape facts that currently live as flat AnalysisResult
// fields (FixedRecordNewTableSites, FixedTableConstructors). These route domain
// access through TableShapeFacts via the bound owner so single-domain passes do
// not touch the flat compatibility surface directly.

// FixedRecordNewTableSitesPopulated reports whether the FixedRecordNewTableSites
// map has been computed for the owning analysis result.
func (t *TableShapeFacts) FixedRecordNewTableSitesPopulated() bool {
	return t != nil && t.owner != nil && t.owner.FixedRecordNewTableSites != nil
}

// SetFixedRecordNewTableSites installs the computed FixedRecordNewTableSites map.
func (t *TableShapeFacts) SetFixedRecordNewTableSites(sites map[int]bool) {
	if t == nil || t.owner == nil {
		return
	}
	t.owner.FixedRecordNewTableSites = sites
	t.bindOwner()
}

// FixedRecordNewTableSite reports whether the given OpNewFixedTable instruction
// ID remains a local fixed-record construction site.
func (t *TableShapeFacts) FixedRecordNewTableSite(id int) bool {
	if t == nil || t.owner == nil || t.owner.FixedRecordNewTableSites == nil {
		return false
	}
	return t.owner.FixedRecordNewTableSites[id]
}

// FixedTableConstructorCount returns the number of recorded fixed table
// constructor facts.
func (t *TableShapeFacts) FixedTableConstructorCount() int {
	if t == nil || t.owner == nil {
		return 0
	}
	return len(t.owner.FixedTableConstructors)
}

// FixedTableConstructorFact returns the fixed table constructor fact for the
// given OpNewTable instruction ID.
func (t *TableShapeFacts) FixedTableConstructorFact(id int) (FixedTableConstructorFact, bool) {
	if t == nil || t.owner == nil || t.owner.FixedTableConstructors == nil {
		return FixedTableConstructorFact{}, false
	}
	fact, ok := t.owner.FixedTableConstructors[id]
	return fact, ok
}
