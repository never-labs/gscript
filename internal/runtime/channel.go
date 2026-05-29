package runtime

import (
	"fmt"
	"reflect"
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
func (c *Channel) Send(val Value) (err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("send on closed channel")
	}
	c.mu.Unlock()
	defer func() {
		if recover() != nil {
			// Closed concurrently after the optimistic closed check. The script
			// visible behavior should be an ordinary send error, not a host panic.
			err = fmt.Errorf("send on closed channel")
		}
	}()
	c.ch <- val
	return nil
}

func (c *Channel) TrySend(val Value) (ok bool, err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false, fmt.Errorf("send on closed channel")
	}
	c.mu.Unlock()
	defer func() {
		if recover() != nil {
			ok = false
			err = fmt.Errorf("send on closed channel")
		}
	}()
	select {
	case c.ch <- val:
		return true, nil
	default:
		return false, nil
	}
}

func (c *Channel) TryRecv() (Value, bool) {
	val, ready, _ := c.TryRecvOK()
	return val, ready
}

func (c *Channel) TryRecvOK() (Value, bool, bool) {
	select {
	case val, ok := <-c.ch:
		if !ok {
			return NilValue(), true, false
		}
		return val, true, true
	default:
		return NilValue(), false, false
	}
}

type ChannelSelectKind uint8

const (
	ChannelSelectRecv ChannelSelectKind = iota
	ChannelSelectSend
)

type ChannelSelectCase struct {
	Kind    ChannelSelectKind
	Channel *Channel
	Value   Value
}

func ChannelSelect(cases []ChannelSelectCase) (chosen int, value Value, recvOK bool, err error) {
	defer func() {
		if recover() != nil {
			chosen = -1
			value = NilValue()
			recvOK = false
			err = fmt.Errorf("send on closed channel")
		}
	}()
	if len(cases) == 0 {
		return -1, NilValue(), false, fmt.Errorf("select requires at least one case")
	}
	reflectCases := make([]reflect.SelectCase, len(cases))
	for i, cls := range cases {
		if cls.Channel == nil {
			return -1, NilValue(), false, fmt.Errorf("select case uses non-channel value")
		}
		switch cls.Kind {
		case ChannelSelectRecv:
			reflectCases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(cls.Channel.ch),
			}
		case ChannelSelectSend:
			cls.Channel.mu.Lock()
			closed := cls.Channel.closed
			cls.Channel.mu.Unlock()
			if closed {
				return -1, NilValue(), false, fmt.Errorf("send on closed channel")
			}
			reflectCases[i] = reflect.SelectCase{
				Dir:  reflect.SelectSend,
				Chan: reflect.ValueOf(cls.Channel.ch),
				Send: reflect.ValueOf(cls.Value),
			}
		default:
			return -1, NilValue(), false, fmt.Errorf("invalid select case kind")
		}
	}
	chosen, recv, ok := reflect.Select(reflectCases)
	if cases[chosen].Kind == ChannelSelectRecv {
		if !ok {
			return chosen, NilValue(), false, nil
		}
		return chosen, recv.Interface().(Value), true, nil
	}
	return chosen, NilValue(), false, nil
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
