package soa

import "fmt"

// ColumnShape is the runtime-independent metadata exposed by soa.shape.
type ColumnShape struct {
	Name    string
	DType   string
	Length  int
	Version uint64
}

// Shape is the runtime-independent metadata exposed by soa.shape.
type Shape struct {
	Length  int
	Version uint64
	Columns []ColumnShape
}

// NewShape validates and copies SOA metadata before the runtime adapts it to a
// script table.
func NewShape(length int, version uint64, columns []ColumnShape) (Shape, error) {
	if length < 0 {
		return Shape{}, fmt.Errorf("soa shape length must be non-negative")
	}
	out := Shape{
		Length:  length,
		Version: version,
		Columns: make([]ColumnShape, len(columns)),
	}
	for i, col := range columns {
		if col.Name == "" {
			return Shape{}, fmt.Errorf("soa shape column %d name must not be empty", i+1)
		}
		if col.Length < 0 {
			return Shape{}, fmt.Errorf("soa shape column %q length must be non-negative", col.Name)
		}
		out.Columns[i] = col
	}
	return out, nil
}
