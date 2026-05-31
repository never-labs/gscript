package container

import "testing"

func TestQueueBoundsOperations(t *testing.T) {
	bounds := NewQueueBounds()
	if !bounds.Empty() || bounds.Size() != 0 {
		t.Fatalf("new bounds = %+v, empty=%v size=%d", bounds, bounds.Empty(), bounds.Size())
	}

	var index int64
	bounds, index = bounds.PushBack()
	if index != 1 || bounds.Size() != 1 {
		t.Fatalf("PushBack index=%d bounds=%+v size=%d", index, bounds, bounds.Size())
	}

	bounds, index = bounds.PushFront()
	if index != 0 || bounds.Size() != 2 {
		t.Fatalf("PushFront index=%d bounds=%+v size=%d", index, bounds, bounds.Size())
	}

	var ok bool
	bounds, index, ok = bounds.PopFront()
	if !ok || index != 0 || bounds.Size() != 1 {
		t.Fatalf("PopFront index=%d ok=%v bounds=%+v size=%d", index, ok, bounds, bounds.Size())
	}

	bounds, index, ok = bounds.PopBack()
	if !ok || index != 1 || !bounds.Empty() {
		t.Fatalf("PopBack index=%d ok=%v bounds=%+v empty=%v", index, ok, bounds, bounds.Empty())
	}
}
