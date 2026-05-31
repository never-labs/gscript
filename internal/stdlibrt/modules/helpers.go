package modules

import (
	"bytes"
	"fmt"

	"github.com/never-labs/gscript/internal/runtime"
)

func hostResultLimit(maxHostResult func() int64) int64 {
	if maxHostResult == nil {
		return 0
	}
	return maxHostResult()
}

func toInt(v runtime.Value) int64 {
	if v.IsInt() {
		return v.Int()
	}
	if v.IsFloat() {
		return int64(v.Float())
	}
	return 0
}

func toFloat(v runtime.Value) float64 {
	if v.IsFloat() {
		return v.Float()
	}
	if v.IsInt() {
		return float64(v.Int())
	}
	return 0
}

type hostResultBuffer struct {
	buf bytes.Buffer
	max int64
}

func newHostResultBuffer(max int64) *hostResultBuffer {
	return &hostResultBuffer{max: max}
}

func (b *hostResultBuffer) Write(p []byte) (int, error) {
	if b.max <= 0 {
		return b.buf.Write(p)
	}
	remaining := b.max - int64(b.buf.Len())
	if remaining <= 0 {
		return 0, fmt.Errorf("host result byte limit exceeded (%d)", b.max)
	}
	if int64(len(p)) > remaining {
		if remaining > 0 {
			_, _ = b.buf.Write(p[:remaining])
		}
		return int(remaining), fmt.Errorf("host result byte limit exceeded (%d)", b.max)
	}
	return b.buf.Write(p)
}

func (b *hostResultBuffer) String() string {
	return b.buf.String()
}
