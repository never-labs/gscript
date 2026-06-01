package container

const (
	InitialQueueHead int64 = 1
	InitialQueueTail int64 = 0
)

type QueueBounds struct {
	Head int64
	Tail int64
}

func NewQueueBounds() QueueBounds {
	return QueueBounds{
		Head: InitialQueueHead,
		Tail: InitialQueueTail,
	}
}

func (b QueueBounds) Empty() bool {
	return b.Head > b.Tail
}

func (b QueueBounds) Size() int64 {
	size := b.Tail - b.Head + 1
	if size < 0 {
		return 0
	}
	return size
}

func (b QueueBounds) PushBack() (QueueBounds, int64) {
	tail, index := PushBackIndex(b.Tail)
	b.Tail = tail
	return b, index
}

func (b QueueBounds) PushFront() (QueueBounds, int64) {
	head, index := PushFrontIndex(b.Head)
	b.Head = head
	return b, index
}

func (b QueueBounds) PopFront() (QueueBounds, int64, bool) {
	if b.Empty() {
		return b, 0, false
	}
	index := b.Head
	b.Head++
	return b, index, true
}

func (b QueueBounds) PopBack() (QueueBounds, int64, bool) {
	if b.Empty() {
		return b, 0, false
	}
	index := b.Tail
	b.Tail--
	return b, index, true
}

func PushBackIndex(tail int64) (int64, int64) {
	tail++
	return tail, tail
}

func PushFrontIndex(head int64) (int64, int64) {
	head--
	return head, head
}
