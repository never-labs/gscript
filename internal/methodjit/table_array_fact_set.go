package methodjit

// tableArrayFactSet owns the block-local consistency discipline for typed table-array facts.
//
// Invariant: a complete fact for (table, kind) is visible only after a matching
// TableArrayHeader, TableArrayLen, and TableArrayData have all been observed
// without an intervening structural mutation of that table or a call. Checked
// OpTableArrayStore and OpTableArraySwap preserve that fact chain;
// OpSetTable/OpAppend/OpSetList do not unless they have already been lowered.
type tableArrayFactSet struct {
	headersByTable map[tableArrayHeaderKey]*tableArrayHeaderFact
	headersByID    map[int]*tableArrayHeaderFact
	lens           map[tableArrayDerivedKey]*Value
	datas          map[tableArrayDerivedKey]*Value
}

type tableArrayHeaderFact struct {
	table  *Value
	header *Value
	kind   int64
}

type tableArrayCompleteFact struct {
	table    *Value
	header   *Value
	headerID int
	data     *Value
	len      *Value
	kind     int64
}

func newTableArrayFactSet() tableArrayFactSet {
	return tableArrayFactSet{
		headersByTable: make(map[tableArrayHeaderKey]*tableArrayHeaderFact),
		headersByID:    make(map[int]*tableArrayHeaderFact),
		lens:           make(map[tableArrayDerivedKey]*Value),
		datas:          make(map[tableArrayDerivedKey]*Value),
	}
}

func (s *tableArrayFactSet) Reset() {
	*s = newTableArrayFactSet()
}

func (s *tableArrayFactSet) Empty() bool {
	return len(s.headersByTable) == 0 && len(s.headersByID) == 0 && len(s.lens) == 0 && len(s.datas) == 0
}

func (s *tableArrayFactSet) LookupHeader(instr *Instr) *Value {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return nil
	}
	fact := s.headersByTable[tableArrayHeaderKey{objID: instr.Args[0].ID, kind: instr.Aux}]
	if fact == nil {
		return nil
	}
	return fact.header
}

func (s *tableArrayFactSet) LookupByRole(role OpTableArrayFactRole, instr *Instr) *Value {
	switch role {
	case OpTableArrayFactHeader:
		return s.LookupHeader(instr)
	case OpTableArrayFactLen:
		return s.LookupLen(instr)
	case OpTableArrayFactData:
		return s.LookupData(instr)
	default:
		return nil
	}
}

func (s *tableArrayFactSet) RecordHeader(instr *Instr) {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return
	}
	tableKey := tableArrayHeaderKey{objID: instr.Args[0].ID, kind: instr.Aux}
	if _, exists := s.headersByTable[tableKey]; exists {
		return
	}
	fact := &tableArrayHeaderFact{
		table:  instr.Args[0],
		header: instr.Value(),
		kind:   instr.Aux,
	}
	s.headersByTable[tableKey] = fact
	s.headersByID[instr.ID] = fact
}

func (s *tableArrayFactSet) RecordByRole(role OpTableArrayFactRole, instr *Instr) bool {
	switch role {
	case OpTableArrayFactHeader:
		if instr == nil || !tableArrayLowerableKind(instr.Aux) {
			return false
		}
		s.RecordHeader(instr)
		return true
	case OpTableArrayFactLen:
		s.RecordLen(instr)
		return true
	case OpTableArrayFactData:
		s.RecordData(instr)
		return true
	default:
		return false
	}
}

func (s *tableArrayFactSet) LookupLen(instr *Instr) *Value {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return nil
	}
	return s.lens[tableArrayDerivedKey{headerID: instr.Args[0].ID, kind: instr.Aux}]
}

func (s *tableArrayFactSet) RecordLen(instr *Instr) {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return
	}
	key := tableArrayDerivedKey{headerID: instr.Args[0].ID, kind: instr.Aux}
	if _, exists := s.lens[key]; exists {
		return
	}
	s.lens[key] = instr.Value()
}

func (s *tableArrayFactSet) LookupData(instr *Instr) *Value {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return nil
	}
	return s.datas[tableArrayDerivedKey{headerID: instr.Args[0].ID, kind: instr.Aux}]
}

func (s *tableArrayFactSet) RecordData(instr *Instr) {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return
	}
	key := tableArrayDerivedKey{headerID: instr.Args[0].ID, kind: instr.Aux}
	if _, exists := s.datas[key]; exists {
		return
	}
	s.datas[key] = instr.Value()
}

func (s *tableArrayFactSet) Complete(tableID int, kind int64) (tableArrayCompleteFact, bool) {
	headerFact := s.headersByTable[tableArrayHeaderKey{objID: tableID, kind: kind}]
	if headerFact == nil || headerFact.header == nil {
		return tableArrayCompleteFact{}, false
	}
	key := tableArrayDerivedKey{headerID: headerFact.header.ID, kind: kind}
	lenVal := s.lens[key]
	dataVal := s.datas[key]
	if lenVal == nil || dataVal == nil {
		return tableArrayCompleteFact{}, false
	}
	return tableArrayCompleteFact{
		table:    headerFact.table,
		header:   headerFact.header,
		headerID: headerFact.header.ID,
		data:     dataVal,
		len:      lenVal,
		kind:     kind,
	}, true
}

func (s *tableArrayFactSet) CompleteFacts() []tableArrayCompleteFact {
	if len(s.headersByID) == 0 {
		return nil
	}
	out := make([]tableArrayCompleteFact, 0, len(s.headersByID))
	for headerID, headerFact := range s.headersByID {
		if headerFact == nil || headerFact.table == nil || headerFact.header == nil {
			continue
		}
		key := tableArrayDerivedKey{headerID: headerID, kind: headerFact.kind}
		lenVal := s.lens[key]
		dataVal := s.datas[key]
		if lenVal == nil || dataVal == nil {
			continue
		}
		out = append(out, tableArrayCompleteFact{
			table:    headerFact.table,
			header:   headerFact.header,
			headerID: headerID,
			data:     dataVal,
			len:      lenVal,
			kind:     headerFact.kind,
		})
	}
	return out
}

func tableArrayHeaderForDataValue(v *Value) (*Instr, bool) {
	if v == nil || v.Def == nil || tableArrayFactRole(v.Def.Op) != OpTableArrayFactData || len(v.Def.Args) < 1 || v.Def.Args[0] == nil {
		return nil, false
	}
	header := v.Def.Args[0].Def
	if header == nil || tableArrayFactRole(header.Op) != OpTableArrayFactHeader || len(header.Args) < 1 || header.Args[0] == nil {
		return nil, false
	}
	return header, true
}

func tableArrayTableValueForDataValue(v *Value) (*Value, bool) {
	header, ok := tableArrayHeaderForDataValue(v)
	if !ok {
		return nil, false
	}
	return header.Args[0], true
}

func (s *tableArrayFactSet) InvalidateTable(tableID int) bool {
	var killedHeaders []int
	for key, fact := range s.headersByTable {
		if key.objID != tableID {
			continue
		}
		delete(s.headersByTable, key)
		if fact != nil && fact.header != nil {
			killedHeaders = append(killedHeaders, fact.header.ID)
		}
	}
	if len(killedHeaders) == 0 {
		return false
	}
	for _, headerID := range killedHeaders {
		delete(s.headersByID, headerID)
		for key := range s.lens {
			if key.headerID == headerID {
				delete(s.lens, key)
			}
		}
		for key := range s.datas {
			if key.headerID == headerID {
				delete(s.datas, key)
			}
		}
	}
	return true
}
