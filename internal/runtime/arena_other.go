//go:build !(darwin && arm64)

package runtime

import (
	"sync"
	"unsafe"
)

// Heap is the portable fallback allocator for platforms that do not use the
// darwin/arm64 mmap arena. It keeps the same public allocation surface while
// relying on ordinary Go heap slices for storage.
type Heap struct {
	mu                   sync.Mutex
	overWords            [][]uint64
	tableSlab            tableSlab
	tableSlabNext        uintptr
	tableSlabStart       uintptr
	tableSlabEnd         uintptr
	tableSvalsSlab       tableSvalsSlab
	tableSvalsSlabNext   uintptr
	tableSvalsSlabStart  uintptr
	tableSvalsSlabEnd    uintptr
	tableSvalsNSlab      tableSvalsNSlab
	tableSvalsNSlabNext  uintptr
	tableSvalsNSlabStart uintptr
	tableSvalsNSlabEnd   uintptr
	fixedRecordSlab      fixedRecordSlab
	fixedRecordSlabNext  uintptr
	fixedRecordSlabStart uintptr
	fixedRecordSlabEnd   uintptr
	stringSlab           stringSlab
	stringBoxSlab        stringBoxSlab
	stringBoxSlabNext    uintptr
	stringBoxSlabStart   uintptr
	stringBoxSlabEnd     uintptr
}

func NewHeap() *Heap {
	return &Heap{}
}

func (h *Heap) AllocBytes(size int) unsafe.Pointer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.allocBytesLocked(size)
}

func (h *Heap) allocBytesLocked(size int) unsafe.Pointer {
	if size <= 0 {
		size = 1
	}
	words := make([]uint64, (size+7)/8)
	h.overWords = append(h.overWords, words)
	return unsafe.Pointer(&words[0])
}

func (h *Heap) AllocValues(length, capacity int) []Value {
	if capacity < length {
		capacity = length
	}
	s := make([]Value, length, capacity)
	nv := NilValue()
	for i := 0; i < length; i++ {
		s[i] = nv
	}
	return s
}

func (h *Heap) GrowValues(old []Value, newCap int) []Value {
	if newCap < len(old) {
		newCap = len(old)
	}
	s := h.AllocValues(len(old), newCap)
	copy(s, old)
	return s
}

func (h *Heap) Free() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.overWords = nil
}

func (h *Heap) AllocInt64s(length, capacity int) []int64 {
	if capacity < length {
		capacity = length
	}
	if capacity == 0 {
		return nil
	}
	return make([]int64, length, capacity)
}

func (h *Heap) GrowInt64s(old []int64, newCap int) []int64 {
	if newCap < len(old) {
		newCap = len(old)
	}
	s := h.AllocInt64s(len(old), newCap)
	copy(s, old)
	return s
}

func (h *Heap) AllocFloat64s(length, capacity int) []float64 {
	if capacity < length {
		capacity = length
	}
	if capacity == 0 {
		return nil
	}
	return make([]float64, length, capacity)
}

func (h *Heap) GrowFloat64s(old []float64, newCap int) []float64 {
	if newCap < len(old) {
		newCap = len(old)
	}
	s := h.AllocFloat64s(len(old), newCap)
	copy(s, old)
	return s
}

func (h *Heap) AllocByteSlice(length, capacity int) []byte {
	if capacity < length {
		capacity = length
	}
	if capacity == 0 {
		return nil
	}
	return make([]byte, length, capacity)
}

func (h *Heap) GrowByteSlice(old []byte, newCap int) []byte {
	if newCap < len(old) {
		newCap = len(old)
	}
	s := h.AllocByteSlice(len(old), newCap)
	copy(s, old)
	return s
}

func arenaAppendInt64(h *Heap, s *[]int64, val int64) {
	old := *s
	if len(old) == cap(old) {
		*s = h.GrowInt64s(old, cap(old)*2+1)
		old = *s
	}
	*s = old[:len(old)+1]
	(*s)[len(*s)-1] = val
}

func arenaAppendFloat64(h *Heap, s *[]float64, val float64) {
	old := *s
	if len(old) == cap(old) {
		*s = h.GrowFloat64s(old, cap(old)*2+1)
		old = *s
	}
	*s = old[:len(old)+1]
	(*s)[len(*s)-1] = val
}

func arenaAppendByte(h *Heap, s *[]byte, val byte) {
	old := *s
	if len(old) == cap(old) {
		*s = h.GrowByteSlice(old, cap(old)*2+1)
		old = *s
	}
	*s = old[:len(old)+1]
	(*s)[len(*s)-1] = val
}

func arenaAppendValue(h *Heap, s *[]Value, val Value) {
	old := *s
	if len(old) == cap(old) {
		*s = h.GrowValues(old, cap(old)*2+1)
		old = *s
	}
	*s = old[:len(old)+1]
	(*s)[len(*s)-1] = val
}

var DefaultHeap *Heap

func init() {
	DefaultHeap = NewHeap()
}
