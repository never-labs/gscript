package runtime

import (
	"fmt"
	"sort"
	"strings"
	"unsafe"
)

// SoA stores logical records as named dense columns. It is intentionally
// separate from Table so data-oriented code can opt into columnar layout without
// inheriting table/hash/metatable semantics on hot paths.
type SoA struct {
	names        []string
	columns      map[string]*DenseArray
	length       int
	shapeVersion uint64
}

type SoAColumnDescriptor struct {
	Name    string
	DType   DenseArrayDType
	Len     int
	Version uint64
}

type SoAShapeSnapshot struct {
	ShapeVersion uint64
	Length       int
	Columns      []SoAColumnDescriptor
}

type SoAAffineTerm struct {
	Dst   string
	Src   string
	Scale float64
	Bias  float64
}

type soaAffinePlan struct {
	dst   *DenseArray
	src   *DenseArray
	scale float64
	bias  float64
}

func NewSoA(columns map[string]*DenseArray) (*SoA, error) {
	if len(columns) == 0 {
		return nil, fmt.Errorf("soa requires at least one column")
	}
	names := make([]string, 0, len(columns))
	length := -1
	copied := make(map[string]*DenseArray, len(columns))
	for name, col := range columns {
		if name == "" {
			return nil, fmt.Errorf("soa column name must not be empty")
		}
		if col == nil {
			return nil, fmt.Errorf("soa column %q is nil", name)
		}
		if length < 0 {
			length = col.Len()
		} else if col.Len() != length {
			return nil, fmt.Errorf("soa column length mismatch")
		}
		names = append(names, name)
		copied[name] = col
	}
	sort.Strings(names)
	return &SoA{names: names, columns: copied, length: length, shapeVersion: 1}, nil
}

func SoAValue(s *SoA) Value {
	if s == nil {
		return NilValue()
	}
	p := unsafe.Pointer(s)
	keepAlive(p, s)
	return Value(tagPtr | ptrSubSoA | (uint64(uintptr(p)) & ptrAddrMask))
}

func (v Value) IsSoA() bool {
	return uint64(v)&tagMask == tagPtr && v.ptrSubType() == ptrSubSoA
}

func (v Value) SoA() *SoA {
	if !v.IsSoA() {
		return nil
	}
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	return (*SoA)(p)
}

func (s *SoA) Len() int {
	if s == nil {
		return 0
	}
	return s.length
}

func (s *SoA) ShapeVersion() uint64 {
	if s == nil {
		return 0
	}
	return s.shapeVersion
}

func (s *SoA) ColumnNames() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.names...)
}

func (s *SoA) Column(name string) (*DenseArray, bool) {
	if s == nil {
		return nil, false
	}
	col, ok := s.columns[name]
	return col, ok
}

func (s *SoA) Unzip() (map[string]*DenseArray, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	out := make(map[string]*DenseArray, len(s.columns))
	for _, name := range s.names {
		col, err := s.columns[name].Clone()
		if err != nil {
			return nil, fmt.Errorf("soa column %q: %w", name, err)
		}
		out[name] = col
	}
	return out, nil
}

func (s *SoA) Slice(start, end int) (*SoA, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if start < 0 || end < start || end > s.length {
		return nil, fmt.Errorf("soa slice out of range")
	}
	cols := make(map[string]*DenseArray, len(s.columns))
	for _, name := range s.names {
		col, err := s.columns[name].Slice(start, end)
		if err != nil {
			return nil, fmt.Errorf("soa column %q: %w", name, err)
		}
		cols[name] = col
	}
	return NewSoA(cols)
}

func (s *SoA) Filter(mask *DenseArray) (*SoA, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return nil, fmt.Errorf("soa filter mask must be a bool dense array")
	}
	if mask.Len() != s.length {
		return nil, ErrDenseArrayLength
	}
	count := denseArrayBoolCount(mask.bools)
	cols := make(map[string]*DenseArray, len(s.columns))
	for _, name := range s.names {
		col, err := s.columns[name].filterKnownCount(mask, count)
		if err != nil {
			return nil, fmt.Errorf("soa column %q: %w", name, err)
		}
		cols[name] = col
	}
	return NewSoA(cols)
}

func (s *SoA) Compact(mask *DenseArray) (*SoA, error) {
	return s.Filter(mask)
}

func (s *SoA) CountWhere(mask *DenseArray) (Value, error) {
	if s == nil {
		return NilValue(), fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return NilValue(), fmt.Errorf("soa countWhere mask must be a bool dense array")
	}
	if mask.Len() != s.length {
		return NilValue(), ErrDenseArrayLength
	}
	return IntValue(int64(denseArrayBoolCount(mask.bools))), nil
}

func (s *SoA) Gather(indices *DenseArray) (*SoA, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if indices == nil || indices.DType() != DenseArrayI64 {
		return nil, fmt.Errorf("soa gather indices must be an i64 dense array")
	}
	if err := denseArrayValidateGatherIndices(indices, s.length); err != nil {
		return nil, err
	}
	cols := make(map[string]*DenseArray, len(s.columns))
	for _, name := range s.names {
		col, err := s.columns[name].gatherValidatedI64(indices.i64)
		if err != nil {
			return nil, fmt.Errorf("soa column %q: %w", name, err)
		}
		cols[name] = col
	}
	return NewSoA(cols)
}

func (s *SoA) ColumnDescriptor(name string) (SoAColumnDescriptor, bool) {
	col, ok := s.Column(name)
	if !ok {
		return SoAColumnDescriptor{}, false
	}
	return SoAColumnDescriptor{
		Name:    name,
		DType:   col.DType(),
		Len:     col.Len(),
		Version: col.Version(),
	}, true
}

func (s *SoA) Snapshot(columnNames ...string) (SoAShapeSnapshot, error) {
	if s == nil {
		return SoAShapeSnapshot{}, fmt.Errorf("soa is nil")
	}
	names := columnNames
	if len(names) == 0 {
		names = s.names
	}
	cols := make([]SoAColumnDescriptor, 0, len(names))
	for _, name := range names {
		desc, ok := s.ColumnDescriptor(name)
		if !ok {
			return SoAShapeSnapshot{}, fmt.Errorf("soa column %q not found", name)
		}
		cols = append(cols, desc)
	}
	return SoAShapeSnapshot{
		ShapeVersion: s.shapeVersion,
		Length:       s.length,
		Columns:      cols,
	}, nil
}

func (s *SoA) ValidateSnapshot(snapshot SoAShapeSnapshot) bool {
	if s == nil || s.shapeVersion != snapshot.ShapeVersion || s.length != snapshot.Length {
		return false
	}
	for _, want := range snapshot.Columns {
		got, ok := s.ColumnDescriptor(want.Name)
		if !ok || got.DType != want.DType || got.Len != want.Len || got.Version != want.Version {
			return false
		}
	}
	return true
}

func (s *SoA) ValidateSnapshotForWrites(snapshot SoAShapeSnapshot, writeNames ...string) bool {
	if s == nil || s.shapeVersion != snapshot.ShapeVersion || s.length != snapshot.Length {
		return false
	}
	for _, want := range snapshot.Columns {
		got, ok := s.ColumnDescriptor(want.Name)
		if !ok || got.DType != want.DType || got.Len != want.Len {
			return false
		}
		if soaNameInList(want.Name, writeNames) {
			continue
		}
		if got.Version != want.Version {
			return false
		}
	}
	return true
}

func soaNameInList(name string, names []string) bool {
	for _, candidate := range names {
		if name == candidate {
			return true
		}
	}
	return false
}

func (s *SoA) Row(i int) (*Table, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if i < 0 || i >= s.length {
		return nil, fmt.Errorf("soa row index out of range")
	}
	row := NewTable()
	for _, name := range s.names {
		v, err := s.columns[name].At(i)
		if err != nil {
			return nil, err
		}
		row.RawSetString(name, v)
	}
	return row, nil
}

func (s *SoA) SetRow(i int, row *Table) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	if row == nil {
		return fmt.Errorf("soa row must be a table")
	}
	if i < 0 || i >= s.length {
		return fmt.Errorf("soa row index out of range")
	}
	for _, name := range s.names {
		v := row.RawGetString(name)
		if v.IsNil() {
			return fmt.Errorf("soa row missing column %q", name)
		}
		if err := s.columns[name].Set(i, v); err != nil {
			return fmt.Errorf("soa column %q: %w", name, err)
		}
	}
	return nil
}

func (s *SoA) AddScaled(dstName, srcName string, scale float64) error {
	dst, src, err := s.numericColumns(dstName, srcName)
	if err != nil {
		return err
	}
	return denseArrayAddScaled(dst, src, scale)
}

func (s *SoA) Affine(dstName, srcName string, scale, bias float64) error {
	dst, src, err := s.numericColumns(dstName, srcName)
	if err != nil {
		return err
	}
	return denseArrayAffine(dst, src, scale, bias)
}

func (s *SoA) AffineMany(terms []SoAAffineTerm) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	if len(terms) == 0 {
		return nil
	}
	var stackPlans [8]soaAffinePlan
	plans := stackPlans[:]
	if len(terms) > len(stackPlans) {
		plans = make([]soaAffinePlan, len(terms))
	} else {
		plans = plans[:len(terms)]
	}
	for i, term := range terms {
		if term.Dst == "" || term.Src == "" {
			return fmt.Errorf("soa.affineMany: term %d has empty column name", i+1)
		}
		for j := 0; j < i; j++ {
			if terms[j].Dst == term.Dst {
				return fmt.Errorf("soa.affineMany: duplicate destination column %q", term.Dst)
			}
		}
		dst, src, err := s.numericColumns(term.Dst, term.Src)
		if err != nil {
			return fmt.Errorf("soa.affineMany term %d: %w", i+1, err)
		}
		plans[i] = soaAffinePlan{dst: dst, src: src, scale: term.Scale, bias: term.Bias}
	}
	for i, term := range terms {
		for _, candidate := range terms {
			if candidate.Dst == term.Src {
				return fmt.Errorf("soa.affineMany: source column %q in term %d is also written; split dependent updates to preserve order", term.Src, i+1)
			}
		}
	}
	for _, plan := range plans {
		if err := denseArrayAffine(plan.dst, plan.src, plan.scale, plan.bias); err != nil {
			return err
		}
	}
	return nil
}

func (s *SoA) Sum(columnName string) (Value, error) {
	if s == nil {
		return NilValue(), fmt.Errorf("soa is nil")
	}
	col, ok := s.Column(columnName)
	if !ok {
		return NilValue(), fmt.Errorf("soa column %q not found", columnName)
	}
	return DenseArrayReduce(DenseArrayReduceSum, col)
}

func (s *SoA) SumWhere(columnName string, mask *DenseArray) (Value, error) {
	if s == nil {
		return NilValue(), fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return NilValue(), fmt.Errorf("soa sumWhere mask must be a bool dense array")
	}
	col, ok := s.Column(columnName)
	if !ok {
		return NilValue(), fmt.Errorf("soa column %q not found", columnName)
	}
	return col.SumWhere(mask)
}

func (s *SoA) MinWhere(columnName string, mask *DenseArray) (Value, error) {
	col, err := s.maskedAggregateColumn("soa minWhere", columnName, mask)
	if err != nil {
		return NilValue(), err
	}
	return col.MinWhere(mask)
}

func (s *SoA) MeanWhere(columnName string, mask *DenseArray) (Value, error) {
	col, err := s.maskedAggregateColumn("soa meanWhere", columnName, mask)
	if err != nil {
		return NilValue(), err
	}
	return col.MeanWhere(mask)
}

func (s *SoA) MaxWhere(columnName string, mask *DenseArray) (Value, error) {
	col, err := s.maskedAggregateColumn("soa maxWhere", columnName, mask)
	if err != nil {
		return NilValue(), err
	}
	return col.MaxWhere(mask)
}

func (s *SoA) maskedAggregateColumn(op, columnName string, mask *DenseArray) (*DenseArray, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return nil, fmt.Errorf("%s mask must be a bool dense array", op)
	}
	col, ok := s.Column(columnName)
	if !ok {
		return nil, fmt.Errorf("soa column %q not found", columnName)
	}
	return col, nil
}

func (s *SoA) numericColumns(dstName, srcName string) (*DenseArray, *DenseArray, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("soa is nil")
	}
	dst, ok := s.Column(dstName)
	if !ok {
		return nil, nil, fmt.Errorf("soa column %q not found", dstName)
	}
	src, ok := s.Column(srcName)
	if !ok {
		return nil, nil, fmt.Errorf("soa column %q not found", srcName)
	}
	if dst.Len() != src.Len() {
		return nil, nil, fmt.Errorf("soa column length mismatch")
	}
	return dst, src, nil
}

func (s *SoA) String() string {
	if s == nil {
		return "soa<nil>"
	}
	return fmt.Sprintf("soa{%s}[%d]", strings.Join(s.names, ", "), s.length)
}

func scanSoARoots(s *SoA, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if s == nil {
		return
	}
	for _, col := range s.columns {
		if col == nil {
			continue
		}
		p := unsafe.Pointer(col)
		addr := uintptr(p)
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		visitor(p)
	}
}
