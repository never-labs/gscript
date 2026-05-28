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
			"validator":                             row.Validator,
			"builder":                               row.Builder,
			"oracle":                                row.Oracle,
			"emitter":                               row.Emitter,
			"regalloc":                              row.Regalloc,
			"deopt":                                 row.Deopt,
			"side_effect":                           row.SideEffect,
			"arg_policy":                            row.ArgPolicy,
			"oracle_support":                        row.OracleSupport,
			"emitter_family":                        row.EmitterFamily,
			"fixed_shape_array_element_write_role":  row.FixedShapeArrayElementWriteRole,
			"fixed_shape_array_element_read_role":   row.FixedShapeArrayElementReadRole,
			"fixed_shape_return_array_element_role": row.FixedShapeReturnArrayElementRole,
			"local_string_array_table_use_role":     row.LocalStringArrayTableUseRole,
			"readonly_table_param_use_role":         row.ReadonlyTableParamUseRole,
			"inline_allocation_role":                row.InlineAllocationRole,
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
	if !strings.Contains(matrix, "validator") || !strings.Contains(matrix, "oracle") || !strings.Contains(matrix, "regalloc") ||
		!strings.Contains(matrix, "effect") || !strings.Contains(matrix, "term") || !strings.Contains(matrix, "keep") {
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
	for _, key := range []string{
		"op", "name", "validator", "builder", "oracle", "emitter", "regalloc", "deopt",
		"side_effect", "terminator", "keep_unused", "arg_policy", "oracle_support", "emitter_family", "may_deopt",
		"direct_deopt_without_full_flush", "native_replay_may_exit",
		"native_replay_visible_side_effect", "native_replay_visible_table_mutation",
		"native_callee_resume_unsafe", "restart_visible_side_effect",
		"fixed_shape_array_element_write_role", "fixed_shape_array_element_read_role",
		"fixed_shape_return_array_element_role", "local_string_array_table_use_role",
		"readonly_table_param_use_role", "inline_allocation_role",
		"tier2_call_boundary_loop_blocker", "tier2_loop_allocation_blocker",
	} {
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
		if row.SideEffect != spec.SideEffect.String() {
			t.Fatalf("%s side_effect = %q, want %q", row.Name, row.SideEffect, spec.SideEffect.String())
		}
		if row.Terminator != spec.Terminator {
			t.Fatalf("%s terminator = %t, want %t", row.Name, row.Terminator, spec.Terminator)
		}
		if row.KeepUnused != spec.KeepUnused {
			t.Fatalf("%s keep_unused = %t, want %t", row.Name, row.KeepUnused, spec.KeepUnused)
		}
		if row.MayDeopt != spec.MayDeopt {
			t.Fatalf("%s may_deopt = %t, want %t", row.Name, row.MayDeopt, spec.MayDeopt)
		}
		if row.DirectDeoptWithoutFullFlush != spec.DirectDeoptWithoutFullFlush {
			t.Fatalf("%s direct_deopt_without_full_flush = %t, want %t", row.Name, row.DirectDeoptWithoutFullFlush, spec.DirectDeoptWithoutFullFlush)
		}
		if row.NativeReplayMayExit != spec.NativeReplayMayExit {
			t.Fatalf("%s native_replay_may_exit = %t, want %t", row.Name, row.NativeReplayMayExit, spec.NativeReplayMayExit)
		}
		if row.NativeReplayVisibleSideEffect != spec.NativeReplayVisibleSideEffect {
			t.Fatalf("%s native_replay_visible_side_effect = %t, want %t", row.Name, row.NativeReplayVisibleSideEffect, spec.NativeReplayVisibleSideEffect)
		}
		if row.NativeReplayVisibleTableMutation != spec.NativeReplayVisibleTableMutation {
			t.Fatalf("%s native_replay_visible_table_mutation = %t, want %t", row.Name, row.NativeReplayVisibleTableMutation, spec.NativeReplayVisibleTableMutation)
		}
		if row.NativeCalleeResumeUnsafe != spec.NativeCalleeResumeUnsafe {
			t.Fatalf("%s native_callee_resume_unsafe = %t, want %t", row.Name, row.NativeCalleeResumeUnsafe, spec.NativeCalleeResumeUnsafe)
		}
		if row.RestartVisibleSideEffect != spec.RestartVisibleSideEffect {
			t.Fatalf("%s restart_visible_side_effect = %t, want %t", row.Name, row.RestartVisibleSideEffect, spec.RestartVisibleSideEffect)
		}
		if row.FixedShapeArrayElementWriteRole != spec.FixedShapeArrayElementWriteRole.String() {
			t.Fatalf("%s fixed_shape_array_element_write_role = %q, want %q", row.Name, row.FixedShapeArrayElementWriteRole, spec.FixedShapeArrayElementWriteRole.String())
		}
		if row.FixedShapeArrayElementReadRole != spec.FixedShapeArrayElementReadRole.String() {
			t.Fatalf("%s fixed_shape_array_element_read_role = %q, want %q", row.Name, row.FixedShapeArrayElementReadRole, spec.FixedShapeArrayElementReadRole.String())
		}
		if row.FixedShapeReturnArrayElementRole != spec.FixedShapeReturnArrayElementRole.String() {
			t.Fatalf("%s fixed_shape_return_array_element_role = %q, want %q", row.Name, row.FixedShapeReturnArrayElementRole, spec.FixedShapeReturnArrayElementRole.String())
		}
		if row.LocalStringArrayTableUseRole != spec.LocalStringArrayTableUseRole.String() {
			t.Fatalf("%s local_string_array_table_use_role = %q, want %q", row.Name, row.LocalStringArrayTableUseRole, spec.LocalStringArrayTableUseRole.String())
		}
		if row.ReadonlyTableParamUseRole != spec.ReadonlyTableParamUseRole.String() {
			t.Fatalf("%s readonly_table_param_use_role = %q, want %q", row.Name, row.ReadonlyTableParamUseRole, spec.ReadonlyTableParamUseRole.String())
		}
		if row.InlineAllocationRole != spec.InlineAllocationRole.String() {
			t.Fatalf("%s inline_allocation_role = %q, want %q", row.Name, row.InlineAllocationRole, spec.InlineAllocationRole.String())
		}
		if row.Tier2CallBoundaryLoopBlocker != spec.Tier2CallBoundaryLoopBlocker {
			t.Fatalf("%s tier2_call_boundary_loop_blocker = %t, want %t", row.Name, row.Tier2CallBoundaryLoopBlocker, spec.Tier2CallBoundaryLoopBlocker)
		}
		if row.Tier2LoopAllocationBlocker != spec.Tier2LoopAllocationBlocker {
			t.Fatalf("%s tier2_loop_allocation_blocker = %t, want %t", row.Name, row.Tier2LoopAllocationBlocker, spec.Tier2LoopAllocationBlocker)
		}
	}
}
