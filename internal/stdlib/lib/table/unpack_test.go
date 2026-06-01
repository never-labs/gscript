package table

import (
	"strings"
	"testing"
)

func TestCheckUnpackRange(t *testing.T) {
	count, err := CheckUnpackRange("unpack", 2, 4)
	if err != nil {
		t.Fatalf("CheckUnpackRange returned error: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	count, err = CheckUnpackRange("spread", 5, 4)
	if err != nil {
		t.Fatalf("inverted range returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("inverted count = %d, want 0", count)
	}
}

func TestCheckUnpackRangeLimit(t *testing.T) {
	_, err := CheckUnpackRange("unpack", 1, int64(UnpackMaxResults)+1)
	if err == nil {
		t.Fatal("over-limit range returned nil error")
	}
	if !strings.Contains(err.Error(), "too many results to table.unpack") {
		t.Fatalf("unexpected error: %v", err)
	}
}
