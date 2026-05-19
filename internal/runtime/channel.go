package runtime

import (
	"fmt"
	"sync"
)

// Channel is GScript's channel type, wrapping a Go channel.
type Channel struct {
	ch     chan Value
	cap    int
	mu     sync.Mutex
	closed bool
}

// NewChannel creates a new channel with the given buffer capacity.
// A capacity of 0 creates an unbuffered channel.
func NewChannel(capacity int) *Channel {
	return &Channel{
		ch:  make(chan Value, capacity),
		cap: capacity,
	}
}

// ChannelCapacityFromValue validates a script-level make(chan, capacity)
// operand before it reaches Go's make(chan, n), which panics on negatives.
func ChannelCapacityFromValue(v Value, name string) (int, error) {
	if !v.IsInt() {
		return 0, fmt.Errorf("%s: capacity must be an integer", name)
	}
	capacity := v.Int()
	if capacity < 0 {
		return 0, fmt.Errorf("%s: capacity must be non-negative", name)
	}
	maxInt := int64(int(^uint(0) >> 1))
	if capacity > maxInt {
		return 0, fmt.Errorf("%s: capacity too large", name)
	}
	return int(capacity), nil
}

// Send sends a value on the channel. Blocks if the channel is full.
func (c *Channel) Send(val Value) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("send on closed channel")
	}
	c.mu.Unlock()
	c.ch <- val
	return nil
}

// Recv receives a value from the channel. Blocks if the channel is empty.
// Returns (value, true) on success, or (NilValue(), false) if the channel is closed and empty.
func (c *Channel) Recv() (Value, bool) {
	val, ok := <-c.ch
	if !ok {
		return NilValue(), false
	}
	return val, true
}

// Close closes the channel.
func (c *Channel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("close of closed channel")
	}
	c.closed = true
	close(c.ch)
	return nil
}

// IsClosed returns whether the channel has been closed.
func (c *Channel) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// Len returns the number of elements queued in the channel buffer.
func (c *Channel) Len() int {
	return len(c.ch)
}

// Cap returns the channel buffer capacity.
func (c *Channel) Cap() int {
	return c.cap
}
