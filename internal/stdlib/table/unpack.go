package table

import "fmt"

// UnpackMaxResults is the explicit GScript host boundary for
// table.unpack/table.spread multi-return expansion.
const UnpackMaxResults = 1_000_000

// CheckUnpackRange validates a 1-based inclusive table.unpack/table.spread
// range and returns the exact result count to preallocate.
func CheckUnpackRange(name string, i, j int64) (int, error) {
	if j < i {
		return 0, nil
	}
	count := uint64(j) - uint64(i) + 1
	if count > UnpackMaxResults {
		return 0, fmt.Errorf("too many results to table.%s (limit %d)", name, UnpackMaxResults)
	}
	return int(count), nil
}
