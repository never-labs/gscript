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
	names   []string
	columns map[string]*DenseArray
	length  int
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
	return &SoA{names: names, columns: copied, length: length}, nil
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
