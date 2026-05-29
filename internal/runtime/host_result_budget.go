package runtime

import (
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
