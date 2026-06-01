package table

import (
	"strings"
	"testing"
)

func TestResolveRange(t *testing.T) {
	tests := []struct {
		name                string
		defaultFirst        int64
		defaultLast         int64
		first               int64
		last                int64
		hasFirst            bool
		hasLast             bool
		wantFirst, wantLast int64
	}{
		{name: "defaults", defaultFirst: 1, defaultLast: 3, wantFirst: 1, wantLast: 3},
		{name: "first only", defaultFirst: 1, defaultLast: 3, first: 2, hasFirst: true, wantFirst: 2, wantLast: 3},
		{name: "last only", defaultFirst: 1, defaultLast: 3, last: 2, hasLast: true, wantFirst: 1, wantLast: 2},
		{name: "both", defaultFirst: 1, defaultLast: 3, first: 2, last: 4, hasFirst: true, hasLast: true, wantFirst: 2, wantLast: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ResolveRange(tt.defaultFirst, tt.defaultLast, tt.first, tt.last, tt.hasFirst, tt.hasLast)
			if r.First != tt.wantFirst || r.Last != tt.wantLast {
				t.Fatalf("range = (%d,%d), want (%d,%d)", r.First, r.Last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

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
			for n := int64(0); n < plan.Count; n++ {
				offset := plan.Offset(n)
				if offset < 0 || offset >= plan.Count {
					t.Fatalf("Offset(%d) = %d outside [0,%d)", n, offset, plan.Count)
				}
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

func TestPlanInsertShift(t *testing.T) {
	plan := PlanInsertShift(4, 2)
	if plan.Start != 4 || plan.End != 2 || plan.Count != 3 {
		t.Fatalf("PlanInsertShift = %+v, want start=4 end=2 count=3", plan)
	}

	plan = PlanInsertShift(3, 4)
	if plan.Count != 0 {
		t.Fatalf("append shift count = %d, want 0", plan.Count)
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

func TestPlanRemoveShift(t *testing.T) {
	plan := PlanRemoveShift(2, 5)
	if plan.Start != 2 || plan.End != 4 || plan.Count != 3 {
		t.Fatalf("PlanRemoveShift = %+v, want start=2 end=4 count=3", plan)
	}

	plan = PlanRemoveShift(3, 3)
	if plan.Count != 0 {
		t.Fatalf("tail remove shift count = %d, want 0", plan.Count)
	}
}
