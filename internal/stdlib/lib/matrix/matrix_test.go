package matrix

import "testing"

func TestShape(t *testing.T) {
	rows, cols, err := Shape(3, 4)
	if err != nil {
		t.Fatalf("Shape returned error: %v", err)
	}
	if rows != 3 || cols != 4 {
		t.Fatalf("Shape = (%d, %d), want (3, 4)", rows, cols)
	}

	if _, _, err := Shape(-1, 4); err == nil {
		t.Fatal("Shape accepted negative rows")
	}
	if _, _, err := Shape(1, -4); err == nil {
		t.Fatal("Shape accepted negative columns")
	}
}

func TestOffset(t *testing.T) {
	offset, err := Offset("matrix.getf", 2, 3, 5, 8)
	if err != nil {
		t.Fatalf("Offset returned error: %v", err)
	}
	if offset != 19 {
		t.Fatalf("Offset = %d, want 19", offset)
	}

	for _, tc := range []struct {
		row int64
		col int64
	}{
		{row: -1, col: 0},
		{row: 5, col: 0},
		{row: 0, col: -1},
		{row: 0, col: 8},
	} {
		if _, err := Offset("matrix.getf", tc.row, tc.col, 5, 8); err == nil {
			t.Fatalf("Offset accepted row=%d col=%d", tc.row, tc.col)
		}
	}
}
