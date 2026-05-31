package table

import (
	"strings"
	"testing"
)

func TestPlanMove(t *testing.T) {
	tests := []struct {
		name      string
		first     int64
		last      int64
		target    int64
		sameTable bool
		count     int64
		forward   bool
	}{
		{name: "empty", first: 4, last: 3, target: 1, sameTable: true, count: 0, forward: true},
		{name: "same table forward", first: 2, last: 4, target: 1, sameTable: true, count: 3, forward: true},
		{name: "same table backward", first: 2, last: 4, target: 3, sameTable: true, count: 3, forward: false},
		{name: "different table forward", first: 2, last: 4, target: 3, sameTable: false, count: 3, forward: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := PlanMove(tt.first, tt.last, tt.target, tt.sameTable)
			if plan.Count != tt.count {
				t.Fatalf("Count = %d, want %d", plan.Count, tt.count)
			}
			if plan.Forward != tt.forward {
				t.Fatalf("Forward = %v, want %v", plan.Forward, tt.forward)
			}
			if plan.First != tt.first || plan.Last != tt.last || plan.Target != tt.target {
				t.Fatalf("range = (%d,%d,%d), want (%d,%d,%d)", plan.First, plan.Last, plan.Target, tt.first, tt.last, tt.target)
			}
		})
	}
}

func TestInsertPosition(t *testing.T) {
	pos, err := InsertPosition(3, 0, false)
	if err != nil {
		t.Fatalf("InsertPosition without pos returned error: %v", err)
	}
	if pos != 4 {
		t.Fatalf("pos = %d, want 4", pos)
	}

	pos, err = InsertPosition(3, 2, true)
	if err != nil {
		t.Fatalf("InsertPosition with pos returned error: %v", err)
	}
	if pos != 2 {
		t.Fatalf("pos = %d, want 2", pos)
	}

	_, err = InsertPosition(3, 5, true)
	if err == nil {
		t.Fatal("out-of-bounds insert returned nil error")
	}
	if !strings.Contains(err.Error(), "table.insert") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemovePosition(t *testing.T) {
	pos, end, err := RemovePosition(3, 0, false)
	if err != nil {
		t.Fatalf("RemovePosition without pos returned error: %v", err)
	}
	if pos != 3 || end {
		t.Fatalf("pos,end = %d,%v, want 3,false", pos, end)
	}

	pos, end, err = RemovePosition(3, 4, true)
	if err != nil {
		t.Fatalf("one-past-end returned error: %v", err)
	}
	if pos != 4 || !end {
		t.Fatalf("pos,end = %d,%v, want 4,true", pos, end)
	}

	pos, end, err = RemovePosition(0, 0, false)
	if err != nil {
		t.Fatalf("empty remove without pos returned error: %v", err)
	}
	if pos != 0 || end {
		t.Fatalf("pos,end = %d,%v, want 0,false", pos, end)
	}

	_, _, err = RemovePosition(3, 0, true)
	if err == nil {
		t.Fatal("out-of-bounds remove returned nil error")
	}
	if !strings.Contains(err.Error(), "table.remove") {
		t.Fatalf("unexpected error: %v", err)
	}
}
