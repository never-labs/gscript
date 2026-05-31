package table

import "fmt"

// MovePlan describes the pure range/direction part of table.move.
type MovePlan struct {
	First   int64
	Last    int64
	Target  int64
	Count   int64
	Forward bool
}

// PlanMove returns the copy count and direction for a table.move range.
func PlanMove(first, last, target int64, sameTable bool) MovePlan {
	plan := MovePlan{
		First:   first,
		Last:    last,
		Target:  target,
		Forward: target <= first || !sameTable,
	}
	if last >= first {
		plan.Count = last - first + 1
	}
	return plan
}

// InsertPosition resolves and validates table.insert's optional position.
func InsertPosition(length, pos int64, hasPos bool) (int64, error) {
	if !hasPos {
		return length + 1, nil
	}
	if pos < 1 || pos > length+1 {
		return 0, fmt.Errorf("bad argument #2 to 'table.insert' (position out of bounds)")
	}
	return pos, nil
}

// RemovePosition resolves and validates table.remove's optional position.
// It returns true when the resolved position is the one-past-the-end case
// that should return nil without mutating the table.
func RemovePosition(length, pos int64, hasPos bool) (int64, bool, error) {
	if !hasPos {
		pos = length
	}
	if pos < 0 || pos > length+1 || (pos == 0 && length > 0) {
		return 0, false, fmt.Errorf("bad argument #2 to 'table.remove' (position out of bounds)")
	}
	return pos, pos == length+1, nil
}
