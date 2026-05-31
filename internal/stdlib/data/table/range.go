package table

import "fmt"

// Range describes a 1-based inclusive table range after optional arguments
// have been resolved.
type Range struct {
	First int64
	Last  int64
}

// ResolveRange applies optional first/last arguments to the supplied defaults.
func ResolveRange(defaultFirst, defaultLast, first, last int64, hasFirst, hasLast bool) Range {
	r := Range{First: defaultFirst, Last: defaultLast}
	if hasFirst {
		r.First = first
	}
	if hasLast {
		r.Last = last
	}
	return r
}

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

// Offset returns the source/destination offset for the nth copy in a move plan.
func (p MovePlan) Offset(n int64) int64 {
	if p.Forward {
		return n
	}
	return p.Count - 1 - n
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

// InsertShiftPlan describes the array segment shifted right by table.insert.
type InsertShiftPlan struct {
	Start int64
	End   int64
	Count int64
}

// PlanInsertShift returns the descending shift range for inserting at pos.
func PlanInsertShift(length, pos int64) InsertShiftPlan {
	plan := InsertShiftPlan{Start: length, End: pos}
	if length >= pos {
		plan.Count = length - pos + 1
	}
	return plan
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

// RemoveShiftPlan describes the array segment shifted left by table.remove.
type RemoveShiftPlan struct {
	Start int64
	End   int64
	Count int64
}

// PlanRemoveShift returns the ascending shift range after removing pos.
func PlanRemoveShift(pos, length int64) RemoveShiftPlan {
	plan := RemoveShiftPlan{Start: pos, End: length - 1}
	if pos < length {
		plan.Count = length - pos
	}
	return plan
}
