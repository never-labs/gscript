package sort

// Direction describes whether an ordering operation should compare values in
// ascending or descending order.
type Direction int

const (
	Ascending Direction = iota
	Descending
)

// Less reports whether the element at left should be ordered before the
// element at right for this direction. less must implement ascending order.
func (d Direction) Less(left, right int, less func(int, int) bool) bool {
	if d == Descending {
		return less(right, left)
	}
	return less(left, right)
}

// ReversePairs calls fn with each 1-based index pair that should be swapped to
// reverse an array-like range of length elements.
func ReversePairs(length int, fn func(left, right int)) {
	for left, right := 1, length; left < right; left, right = left+1, right-1 {
		fn(left, right)
	}
}

// BinarySearch1Based searches a sorted 1-based range. compare must return
// equal=true when index matches the target, or beforeTarget=true when index is
// ordered before the target and the search should continue to the right.
func BinarySearch1Based(length int, compare func(index int) (equal bool, beforeTarget bool)) (int, bool) {
	lo, hi := 1, length
	for lo <= hi {
		mid := lo + (hi-lo)/2
		equal, beforeTarget := compare(mid)
		if equal {
			return mid, true
		}
		if beforeTarget {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return 0, false
}
