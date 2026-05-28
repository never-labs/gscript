package methodjit

import (
	"bytes"
	"encoding/json"
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
			"validator":      row.Validator,
			"builder":        row.Builder,
			"oracle":         row.Oracle,
			"emitter":        row.Emitter,
			"regalloc":       row.Regalloc,
			"deopt":          row.Deopt,
			"arg_policy":     row.ArgPolicy,
			"oracle_support": row.OracleSupport,
			"emitter_family": row.EmitterFamily,
		} {
			if value == "" {
				t.Fatalf("%s has empty %s audit column", row.Name, name)
			}
		}
	}
}

func TestOpAuditMatrixExplainsOracleUnsupportedOps(t *testing.T) {
	rows := OpAuditMatrix()
	for _, row := range rows {
		if row.Op == OpYield {
			if row.Oracle != "unsupported(coroutine)" {
				t.Fatalf("OpYield oracle audit = %q, want unsupported(coroutine)", row.Oracle)
			}
			return
		}
	}
	t.Fatal("OpYield missing from OpAuditMatrix")
}

func TestPrintOpAuditMatrix(t *testing.T) {
	matrix := FormatOpAuditMatrix()
	if !strings.Contains(matrix, "validator") || !strings.Contains(matrix, "oracle") || !strings.Contains(matrix, "regalloc") {
		t.Fatalf("matrix header missing expected columns:\n%s", matrix)
	}
	t.Logf("\n%s", matrix)
}

func TestWriteOpAuditMatrixJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOpAuditMatrixJSON(&buf); err != nil {
		t.Fatalf("WriteOpAuditMatrixJSON: %v", err)
	}
	var rows []OpAuditRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal audit JSON: %v\n%s", err, buf.String())
	}
	if len(rows) != int(OpMax) {
		t.Fatalf("audit JSON rows=%d, want %d", len(rows), OpMax)
	}
	if rows[0].Name == "" || rows[0].Validator == "" {
		t.Fatalf("first audit JSON row missing expected fields: %+v", rows[0])
	}
	var raw []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal raw audit JSON: %v", err)
	}
	for _, key := range []string{"op", "name", "validator", "builder", "oracle", "emitter", "regalloc", "deopt", "arg_policy", "oracle_support", "emitter_family", "may_deopt"} {
		if _, ok := raw[0][key]; !ok {
			t.Fatalf("audit JSON first row missing key %q: %v", key, raw[0])
		}
	}
}

func TestOpAuditMatrixStructuredFieldsMatchOpSpec(t *testing.T) {
	for _, row := range OpAuditMatrix() {
		spec, ok := row.Op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", row.Name)
		}
		if row.ArgPolicy != spec.ArgPolicy.String() {
			t.Fatalf("%s arg_policy = %q, want %q", row.Name, row.ArgPolicy, spec.ArgPolicy.String())
		}
		if row.OracleSupport != spec.OracleSupport.String() {
			t.Fatalf("%s oracle_support = %q, want %q", row.Name, row.OracleSupport, spec.OracleSupport.String())
		}
		if row.OracleReason != spec.OracleUnsupportedReason {
			t.Fatalf("%s oracle_reason = %q, want %q", row.Name, row.OracleReason, spec.OracleUnsupportedReason)
		}
		if row.EmitterFamily != spec.EmitterFamily.String() {
			t.Fatalf("%s emitter_family = %q, want %q", row.Name, row.EmitterFamily, spec.EmitterFamily.String())
		}
		if row.MayDeopt != spec.MayDeopt {
			t.Fatalf("%s may_deopt = %t, want %t", row.Name, row.MayDeopt, spec.MayDeopt)
		}
	}
}
