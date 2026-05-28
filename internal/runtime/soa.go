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

func (s *SoA) WithColumn(name string, col *DenseArray) (*SoA, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if name == "" {
		return nil, fmt.Errorf("soa column name must not be empty")
	}
	if col == nil {
		return nil, fmt.Errorf("soa column %q is nil", name)
	}
	if col.Len() != s.length {
		return nil, fmt.Errorf("soa column length mismatch")
	}
	cols := make(map[string]*DenseArray, len(s.columns)+1)
	for _, existing := range s.names {
		cols[existing] = s.columns[existing]
	}
	cols[name] = col
	return NewSoA(cols)
}

func (s *SoA) DropColumn(name string) (*SoA, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if name == "" {
		return nil, fmt.Errorf("soa column name must not be empty")
	}
	if _, ok := s.columns[name]; !ok {
		return nil, fmt.Errorf("soa column %q not found", name)
	}
	if len(s.columns) == 1 {
		return nil, fmt.Errorf("soa requires at least one column")
	}
	cols := make(map[string]*DenseArray, len(s.columns)-1)
	for _, existing := range s.names {
		if existing != name {
			cols[existing] = s.columns[existing]
		}
	}
	return NewSoA(cols)
}

func (s *SoA) Resize(n int) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	if n < 0 {
		return fmt.Errorf("soa length must be non-negative")
	}
	if n == s.length {
		return nil
	}
	for _, name := range s.names {
		if err := s.columns[name].Resize(n); err != nil {
			return fmt.Errorf("soa column %q: %w", name, err)
		}
	}
	s.length = n
	s.bumpShapeVersion()
	return nil
}

func (s *SoA) AppendRow(row *Table) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	if row == nil {
		return fmt.Errorf("soa row must be a table")
	}
	values := make([]Value, len(s.names))
	for i, name := range s.names {
		v := row.RawGetString(name)
		if v.IsNil() {
			return fmt.Errorf("soa row missing column %q", name)
		}
		col := s.columns[name]
		if err := col.CanAppend(v); err != nil {
			return fmt.Errorf("soa column %q: %w", name, err)
		}
		values[i] = v
	}
	for i, name := range s.names {
		if err := s.columns[name].Append(values[i]); err != nil {
			return fmt.Errorf("soa column %q: %w", name, err)
		}
	}
	s.length++
	s.bumpShapeVersion()
	return nil
}

func (s *SoA) Fill(columnName string, value Value) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	col, ok := s.Column(columnName)
	if !ok {
		return fmt.Errorf("soa column %q not found", columnName)
	}
	return col.Fill(value)
}

func (s *SoA) FillWhere(columnName string, mask *DenseArray, value Value) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return fmt.Errorf("soa fillWhere mask must be a bool dense array")
	}
	col, ok := s.Column(columnName)
	if !ok {
		return fmt.Errorf("soa column %q not found", columnName)
	}
	return col.FillWhere(mask, value)
}

func (s *SoA) Select(mask *DenseArray, trueValue, falseValue Value) (*DenseArray, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return nil, fmt.Errorf("soa select mask must be a bool dense array")
	}
	trueResolved, err := s.resolveSelectOperand(trueValue)
	if err != nil {
		return nil, err
	}
	falseResolved, err := s.resolveSelectOperand(falseValue)
	if err != nil {
		return nil, err
	}
	return denseArraySelect(mask, trueResolved, falseResolved, s.length)
}

func (s *SoA) SelectInto(dstName string, mask *DenseArray, trueValue, falseValue Value) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return fmt.Errorf("soa selectInto mask must be a bool dense array")
	}
	dst, ok := s.Column(dstName)
	if !ok {
		return fmt.Errorf("soa column %q not found", dstName)
	}
	trueResolved, err := s.resolveSelectOperand(trueValue)
	if err != nil {
		return err
	}
	falseResolved, err := s.resolveSelectOperand(falseValue)
	if err != nil {
		return err
	}
	return denseArraySelectInto(dst, mask, trueResolved, falseResolved, s.length)
}

func (s *SoA) SumSelect(mask *DenseArray, trueValue, falseValue Value) (Value, error) {
	if s == nil {
		return NilValue(), fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return NilValue(), fmt.Errorf("soa sumSelect mask must be a bool dense array")
	}
	trueResolved, err := s.resolveSelectOperand(trueValue)
	if err != nil {
		return NilValue(), err
	}
	falseResolved, err := s.resolveSelectOperand(falseValue)
	if err != nil {
		return NilValue(), err
	}
	return denseArraySumSelect(mask, trueResolved, falseResolved, s.length)
}

func (s *SoA) resolveSelectOperand(v Value) (Value, error) {
	if v.IsString() {
		col, ok := s.Column(v.Str())
		if !ok {
			return NilValue(), fmt.Errorf("soa column %q not found", v.Str())
		}
		return DenseArrayValue(col), nil
	}
	return v, nil
}

func (s *SoA) bumpShapeVersion() {
	if s == nil {
		return
	}
	s.shapeVersion++
	if s.shapeVersion == 0 {
		s.shapeVersion = 1
	}
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

func (s *SoA) IndicesWhere(mask *DenseArray) (*DenseArray, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return nil, fmt.Errorf("soa indicesWhere mask must be a bool dense array")
	}
	if mask.Len() != s.length {
		return nil, ErrDenseArrayLength
	}
	return denseArrayIndicesWhere(mask)
}

func (s *SoA) Mask(columnName, op string, rhs Value) (*DenseArray, error) {
	if s == nil {
		return nil, fmt.Errorf("soa is nil")
	}
	left, ok := s.Column(columnName)
	if !ok {
		return nil, fmt.Errorf("soa column %q not found", columnName)
	}
	denseOp, err := denseArrayCompareOp(op)
	if err != nil {
		return nil, err
	}
	right := rhs
	if rhs.IsString() {
		col, ok := s.Column(rhs.Str())
		if !ok {
			return nil, fmt.Errorf("soa column %q not found", rhs.Str())
		}
		right = DenseArrayValue(col)
	}
	return denseArrayCompareMask(left, denseOp, right)
}

func denseArrayCompareOp(op string) (DenseArrayBinaryOp, error) {
	switch op {
	case "==", "eq":
		return DenseArrayEQ, nil
	case "!=", "ne":
		return DenseArrayNE, nil
	case "<", "lt":
		return DenseArrayLT, nil
	case "<=", "le":
		return DenseArrayLE, nil
	case ">", "gt":
		return DenseArrayGT, nil
	case ">=", "ge":
		return DenseArrayGE, nil
	default:
		return DenseArrayEQ, fmt.Errorf("soa mask comparison %q is not supported", op)
	}
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

func (s *SoA) ScatterInto(dstName string, indices *DenseArray, values Value) error {
	if s == nil {
		return fmt.Errorf("soa is nil")
	}
	if indices == nil || indices.DType() != DenseArrayI64 {
		return fmt.Errorf("soa scatterInto indices must be an i64 dense array")
	}
	dst, ok := s.Column(dstName)
	if !ok {
		return fmt.Errorf("soa column %q not found", dstName)
	}
	return denseArrayScatterInto(dst, indices, values)
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

func (s *SoA) AffineWhere(dstName, srcName string, mask *DenseArray, scale, bias float64) error {
	dst, src, err := s.numericColumns(dstName, srcName)
	if err != nil {
		return err
	}
	if mask == nil || mask.DType() != DenseArrayBool {
		return fmt.Errorf("soa affineWhere mask must be a bool dense array")
	}
	return denseArrayAffineWhere(dst, src, mask, scale, bias)
}

func (s *SoA) AffineMany(terms []SoAAffineTerm) error {
	plans, _, _, err := s.affineManyPlan(terms)
	if err != nil {
		return err
	}
	return applySoAAffinePlans(plans)
}

func (s *SoA) affineManyPlan(terms []SoAAffineTerm) ([]soaAffinePlan, []string, SoAShapeSnapshot, error) {
	if s == nil {
		return nil, nil, SoAShapeSnapshot{}, fmt.Errorf("soa is nil")
	}
	if len(terms) == 0 {
		return nil, nil, SoAShapeSnapshot{}, nil
	}
	var stackPlans [8]soaAffinePlan
	plans := stackPlans[:]
	if len(terms) > len(stackPlans) {
		plans = make([]soaAffinePlan, len(terms))
	} else {
		plans = plans[:len(terms)]
	}
	writeNames := make([]string, len(terms))
	snapshotNames := make([]string, 0, len(terms)*2)
	for i, term := range terms {
		if term.Dst == "" || term.Src == "" {
			return nil, nil, SoAShapeSnapshot{}, fmt.Errorf("soa.affineMany: term %d has empty column name", i+1)
		}
		for j := 0; j < i; j++ {
			if terms[j].Dst == term.Dst {
				return nil, nil, SoAShapeSnapshot{}, fmt.Errorf("soa.affineMany: duplicate destination column %q", term.Dst)
			}
		}
		dst, src, err := s.numericColumns(term.Dst, term.Src)
		if err != nil {
			return nil, nil, SoAShapeSnapshot{}, fmt.Errorf("soa.affineMany term %d: %w", i+1, err)
		}
		plans[i] = soaAffinePlan{dst: dst, src: src, scale: term.Scale, bias: term.Bias}
		writeNames[i] = term.Dst
		snapshotNames = append(snapshotNames, term.Dst, term.Src)
	}
	for i, term := range terms {
		for _, candidate := range terms {
			if candidate.Dst == term.Src {
				return nil, nil, SoAShapeSnapshot{}, fmt.Errorf("soa.affineMany: source column %q in term %d is also written; split dependent updates to preserve order", term.Src, i+1)
			}
		}
	}
	guard, err := s.Snapshot(snapshotNames...)
	if err != nil {
		return nil, nil, SoAShapeSnapshot{}, err
	}
	ownedPlans := append([]soaAffinePlan(nil), plans...)
	return ownedPlans, writeNames, guard, nil
}

func applySoAAffinePlans(plans []soaAffinePlan) error {
	if denseArrayAffineManyF64(plans) {
		return nil
	}
	for _, plan := range plans {
		if err := denseArrayAffine(plan.dst, plan.src, plan.scale, plan.bias); err != nil {
			return err
		}
	}
	return nil
}

func denseArrayAffineManyF64(plans []soaAffinePlan) bool {
	if len(plans) == 0 {
		return false
	}
	for _, plan := range plans {
		if plan.dst == nil || plan.src == nil || plan.dst.dtype != DenseArrayF64 || plan.src.dtype != DenseArrayF64 {
			return false
		}
	}
	n := plans[0].dst.Len()
	switch len(plans) {
	case 1:
		p0 := plans[0]
		d0, s0 := p0.dst.f64, p0.src.f64
		scale0, bias0 := p0.scale, p0.bias
		if n > 0 {
			_, _ = d0[n-1], s0[n-1]
		}
		for i := 0; i < n; i++ {
			d0[i] = s0[i]*scale0 + bias0
		}
	case 2:
		p0, p1 := plans[0], plans[1]
		d0, s0 := p0.dst.f64, p0.src.f64
		d1, s1 := p1.dst.f64, p1.src.f64
		scale0, bias0 := p0.scale, p0.bias
		scale1, bias1 := p1.scale, p1.bias
		if n > 0 {
			_, _, _, _ = d0[n-1], s0[n-1], d1[n-1], s1[n-1]
		}
		for i := 0; i < n; i++ {
			d0[i] = s0[i]*scale0 + bias0
			d1[i] = s1[i]*scale1 + bias1
		}
	case 3:
		p0, p1, p2 := plans[0], plans[1], plans[2]
		denseArrayAffineMany3F64(p0.dst.f64, p0.src.f64, p0.scale, p0.bias, p1.dst.f64, p1.src.f64, p1.scale, p1.bias, p2.dst.f64, p2.src.f64, p2.scale, p2.bias)
	case 4:
		p0, p1, p2, p3 := plans[0], plans[1], plans[2], plans[3]
		d0, s0 := p0.dst.f64, p0.src.f64
		d1, s1 := p1.dst.f64, p1.src.f64
		d2, s2 := p2.dst.f64, p2.src.f64
		d3, s3 := p3.dst.f64, p3.src.f64
		scale0, bias0 := p0.scale, p0.bias
		scale1, bias1 := p1.scale, p1.bias
		scale2, bias2 := p2.scale, p2.bias
		scale3, bias3 := p3.scale, p3.bias
		if n > 0 {
			_, _, _, _, _, _, _, _ = d0[n-1], s0[n-1], d1[n-1], s1[n-1], d2[n-1], s2[n-1], d3[n-1], s3[n-1]
		}
		for i := 0; i < n; i++ {
			d0[i] = s0[i]*scale0 + bias0
			d1[i] = s1[i]*scale1 + bias1
			d2[i] = s2[i]*scale2 + bias2
			d3[i] = s3[i]*scale3 + bias3
		}
	default:
		return false
	}
	for _, plan := range plans {
		plan.dst.bumpVersion()
	}
	return true
}

func denseArrayAffineMany3F64(d0, s0 []float64, scale0, bias0 float64, d1, s1 []float64, scale1, bias1 float64, d2, s2 []float64, scale2, bias2 float64) {
	n := len(d0)
	if n == 0 {
		return
	}
	_, _, _, _, _, _ = d0[n-1], s0[n-1], d1[n-1], s1[n-1], d2[n-1], s2[n-1]
	for i := 0; i < n; i++ {
		d0[i] = s0[i]*scale0 + bias0
		d1[i] = s1[i]*scale1 + bias1
		d2[i] = s2[i]*scale2 + bias2
	}
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

func (s *SoA) StatsWhere(columnName string, mask *DenseArray) (*Table, error) {
	col, err := s.maskedAggregateColumn("soa statsWhere", columnName, mask)
	if err != nil {
		return nil, err
	}
	return col.StatsWhere(mask)
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
