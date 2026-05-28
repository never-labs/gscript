package methodjit

import (
	"strings"
	"testing"
)

func TestOpAuditMatrixCoversEveryOp(t *testing.T) {
	rows := OpAuditMatrix()
	if len(rows) != int(OpMax) {
		t.Fatalf("audit rows=%d, want %d", len(rows), OpMax)
	}
	for i, row := range rows {
		if row.Op != Op(i) {
			t.Fatalf("row %d has op %s", i, row.Op)
		}
		for name, value := range map[string]string{
			"validator": row.Validator,
			"builder":   row.Builder,
			"oracle":    row.Oracle,
			"emitter":   row.Emitter,
			"regalloc":  row.Regalloc,
			"deopt":     row.Deopt,
		} {
			if value == "" {
				t.Fatalf("%s has empty %s audit column", row.Name, name)
			}
		}
	}
}

func TestPrintOpAuditMatrix(t *testing.T) {
	matrix := FormatOpAuditMatrix()
	if !strings.Contains(matrix, "validator") || !strings.Contains(matrix, "oracle") || !strings.Contains(matrix, "regalloc") {
		t.Fatalf("matrix header missing expected columns:\n%s", matrix)
	}
	t.Logf("\n%s", matrix)
}
