package soa

import "testing"

func TestNewColumnShape(t *testing.T) {
	col := NewColumnShape("x", "f64", 3, 7)
	if col.Name != "x" || col.DType != "f64" || col.Length != 3 || col.Version != 7 {
		t.Fatalf("column shape mismatch: %+v", col)
	}
}

func TestNewShapeCopiesColumns(t *testing.T) {
	cols := []ColumnShape{{Name: "x", DType: "f64", Length: 3, Version: 7}}
	shape, err := NewShape(3, 11, cols)
	if err != nil {
		t.Fatalf("NewShape returned error: %v", err)
	}
	cols[0].Name = "mutated"
	if shape.Length != 3 || shape.Version != 11 {
		t.Fatalf("shape header mismatch: %+v", shape)
	}
	if len(shape.Columns) != 1 || shape.Columns[0].Name != "x" {
		t.Fatalf("columns were not copied: %+v", shape.Columns)
	}
}

func TestShapeMetadataFallsBackAndCopies(t *testing.T) {
	cols := []ColumnShape{{Name: "x", DType: "f64", Length: -1, Version: 7}}
	shape := ShapeMetadata(3, 11, cols)
	cols[0].Name = "mutated"
	if shape.Length != 3 || shape.Version != 11 {
		t.Fatalf("shape header mismatch: %+v", shape)
	}
	if len(shape.Columns) != 1 || shape.Columns[0].Name != "x" || shape.Columns[0].Length != -1 {
		t.Fatalf("fallback columns mismatch: %+v", shape.Columns)
	}
}

func TestNewShapeValidatesMetadata(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		columns []ColumnShape
	}{
		{name: "negative shape length", length: -1},
		{name: "empty column name", length: 1, columns: []ColumnShape{{DType: "f64", Length: 1}}},
		{name: "negative column length", length: 1, columns: []ColumnShape{{Name: "x", DType: "f64", Length: -1}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewShape(tt.length, 1, tt.columns); err == nil {
				t.Fatalf("NewShape succeeded for invalid metadata")
			}
		})
	}
}
