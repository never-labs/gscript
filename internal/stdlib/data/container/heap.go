package container

// ItemHeap adapts a typed slice and comparison function to heap.Interface.
type ItemHeap[T any] struct {
	Items    []T
	LessFunc func(left, right T) bool
}

func (h ItemHeap[T]) Len() int { return len(h.Items) }

func (h ItemHeap[T]) Less(i, j int) bool {
	if h.LessFunc == nil {
		return false
	}
	return h.LessFunc(h.Items[i], h.Items[j])
}

func (h ItemHeap[T]) Swap(i, j int) { h.Items[i], h.Items[j] = h.Items[j], h.Items[i] }

func (h *ItemHeap[T]) PushItem(item T) { h.Items = append(h.Items, item) }

func (h *ItemHeap[T]) PopItem() T {
	old := h.Items
	n := len(old)
	item := old[n-1]
	h.Items = old[:n-1]
	return item
}

func (h *ItemHeap[T]) Push(x interface{}) { h.PushItem(x.(T)) }

func (h *ItemHeap[T]) Pop() interface{} { return h.PopItem() }
