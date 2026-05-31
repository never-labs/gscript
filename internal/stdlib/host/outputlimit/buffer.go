package outputlimit

import (
	"bytes"
	"fmt"
)

// Budget is shared by output buffers whose combined bytes count against one
// host result limit.
type Budget struct {
	max      int64
	used     int64
	exceeded bool
}

// Buffer captures host output while enforcing an optional shared byte budget.
type Buffer struct {
	buf    bytes.Buffer
	budget *Budget
}

// NewBuffers returns two buffers that share the same byte budget. A max of zero
// or less disables limiting.
func NewBuffers(max int64) (*Buffer, *Buffer) {
	budget := &Budget{max: max}
	return &Buffer{budget: budget}, &Buffer{budget: budget}
}

func (b *Buffer) Write(p []byte) (int, error) {
	if b.budget == nil || b.budget.max <= 0 {
		return b.buf.Write(p)
	}
	remaining := b.budget.max - b.budget.used
	if remaining <= 0 {
		b.budget.exceeded = true
		return 0, fmt.Errorf("host result byte limit exceeded (%d)", b.budget.max)
	}
	if int64(len(p)) > remaining {
		if remaining > 0 {
			_, _ = b.buf.Write(p[:remaining])
			b.budget.used += remaining
		}
		b.budget.exceeded = true
		return int(remaining), fmt.Errorf("host result byte limit exceeded (%d)", b.budget.max)
	}
	n, err := b.buf.Write(p)
	b.budget.used += int64(n)
	return n, err
}

func (b *Buffer) String() string {
	if b == nil {
		return ""
	}
	return b.buf.String()
}

// Exceeded reports whether any buffer sharing this budget crossed the limit.
func (b *Buffer) Exceeded() bool {
	return b != nil && b.budget != nil && b.budget.exceeded
}

// Limit returns the configured byte limit for this buffer.
func (b *Buffer) Limit() int64 {
	if b == nil || b.budget == nil {
		return 0
	}
	return b.budget.max
}
