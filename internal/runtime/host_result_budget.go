package runtime

import (
	"bytes"
	"fmt"
	"io"
)

// CheckHostResultBytes verifies that native-call results do not exceed a host
// materialization budget. It currently charges strings, including strings
// nested in returned tables, which covers the high-risk stdlib and callback
// paths that materialize host data into script values.
func CheckHostResultBytes(max int64, values ...Value) error {
	if max <= 0 {
		return nil
	}
	var used int64
	seen := make(map[*Table]bool)
	var visit func(Value) error
	visit = func(v Value) error {
		if v.IsString() {
			used += int64(StringLen(v))
			if used > max {
				return fmt.Errorf("host result byte limit exceeded (%d)", max)
			}
			return nil
		}
		if !v.IsTable() {
			return nil
		}
		t := v.Table()
		if t == nil || seen[t] {
			return nil
		}
		seen[t] = true
		for _, key := range t.PairsKeysSnapshot() {
			if err := visit(key); err != nil {
				return err
			}
			if err := visit(t.RawGet(key)); err != nil {
				return err
			}
		}
		return nil
	}
	for _, v := range values {
		if err := visit(v); err != nil {
			return err
		}
	}
	return nil
}

func ReadAllWithHostResultLimit(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return io.ReadAll(r)
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("host result byte limit exceeded (%d)", max)
	}
	return data, nil
}

func CheckProjectedHostStringBytes(max int64, size int) error {
	if max <= 0 {
		return nil
	}
	if int64(size) > max {
		return fmt.Errorf("host result byte limit exceeded (%d)", max)
	}
	return nil
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
