package container

import (
	"container/heap"
	"testing"
)

func TestItemHeapOrdersByProvidedLess(t *testing.T) {
	h := &ItemHeap[int]{
		LessFunc: func(left, right int) bool {
			return left < right
		},
	}
	heap.Init(h)
	heap.Push(h, 3)
	heap.Push(h, 1)
	heap.Push(h, 2)

	for _, want := range []int{1, 2, 3} {
		if got := heap.Pop(h).(int); got != want {
			t.Fatalf("heap.Pop() = %d, want %d", got, want)
		}
	}
}
