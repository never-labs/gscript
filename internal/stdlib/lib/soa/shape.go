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

func NewColumnShape(name, dtype string, length int, version uint64) ColumnShape {
	return ColumnShape{Name: name, DType: dtype, Length: length, Version: version}
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

// ShapeMetadata preserves the historical runtime behavior: validate when the
// metadata is well-formed, but still return the raw metadata if a runtime
// snapshot contains invalid shape fields.
func ShapeMetadata(length int, version uint64, columns []ColumnShape) Shape {
	shape, err := NewShape(length, version, columns)
	if err == nil {
		return shape
	}
	return Shape{
		Length:  length,
		Version: version,
		Columns: append([]ColumnShape(nil), columns...),
	}
}
