package sort

import (
	"reflect"
	"testing"
)

func TestDirectionLess(t *testing.T) {
	values := []int{3, 1}
	less := func(left, right int) bool {
		return values[left] < values[right]
	}

	if !Ascending.Less(1, 0, less) {
		t.Fatalf("ascending should preserve less direction")
	}
	if !Descending.Less(0, 1, less) {
		t.Fatalf("descending should reverse less direction")
	}
}

func TestReversePairs(t *testing.T) {
	var pairs [][2]int
	ReversePairs(5, func(left, right int) {
		pairs = append(pairs, [2]int{left, right})
	})

	want := [][2]int{{1, 5}, {2, 4}}
	if !reflect.DeepEqual(pairs, want) {
		t.Fatalf("pairs = %v, want %v", pairs, want)
	}
}

func TestBinarySearch1Based(t *testing.T) {
	values := []int{2, 4, 6, 8, 10}
	idx, ok := BinarySearch1Based(len(values), func(index int) (bool, bool) {
		value := values[index-1]
		return value == 8, value < 8
	})
	if !ok || idx != 4 {
		t.Fatalf("found (%d, %v), want (4, true)", idx, ok)
	}

	idx, ok = BinarySearch1Based(len(values), func(index int) (bool, bool) {
		value := values[index-1]
		return value == 7, value < 7
	})
	if ok || idx != 0 {
		t.Fatalf("found (%d, %v), want (0, false)", idx, ok)
	}
}
