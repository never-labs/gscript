package methodjit

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type OpAuditRow struct {
	Op                               Op     `json:"op"`
	Name                             string `json:"name"`
	Validator                        string `json:"validator"`
	Builder                          string `json:"builder"`
	Oracle                           string `json:"oracle"`
	Emitter                          string `json:"emitter"`
	Regalloc                         string `json:"regalloc"`
	Deopt                            string `json:"deopt"`
	ArgPolicy                        string `json:"arg_policy"`
	OracleSupport                    string `json:"oracle_support"`
	OracleReason                     string `json:"oracle_reason,omitempty"`
	EmitterFamily                    string `json:"emitter_family"`
	MayDeopt                         bool   `json:"may_deopt"`
	FixedShapeArrayElementWriteRole  string `json:"fixed_shape_array_element_write_role"`
	FixedShapeArrayElementReadRole   string `json:"fixed_shape_array_element_read_role"`
	FixedShapeReturnArrayElementRole string `json:"fixed_shape_return_array_element_role"`
	LocalStringArrayTableUseRole     string `json:"local_string_array_table_use_role"`
	ReadonlyTableParamUseRole        string `json:"readonly_table_param_use_role"`
	InlineAllocationRole             string `json:"inline_allocation_role"`
	Tier2CallBoundaryLoopBlocker     bool   `json:"tier2_call_boundary_loop_blocker"`
	Tier2LoopAllocationBlocker       bool   `json:"tier2_loop_allocation_blocker"`
}

func OpAuditMatrix() []OpAuditRow {
	rows := make([]OpAuditRow, 0, OpMax)
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			continue
		}
		rows = append(rows, OpAuditRow{
			Op:                               op,
			Name:                             spec.Name,
			Validator:                        opAuditValidator(spec),
			Builder:                          opAuditBuilder(spec),
			Oracle:                           opAuditOracle(spec),
			Emitter:                          opAuditEmitter(spec),
			Regalloc:                         opAuditRegalloc(spec),
			Deopt:                            opAuditDeopt(spec),
			ArgPolicy:                        spec.ArgPolicy.String(),
			OracleSupport:                    spec.OracleSupport.String(),
			OracleReason:                     spec.OracleUnsupportedReason,
			EmitterFamily:                    spec.EmitterFamily.String(),
			MayDeopt:                         spec.MayDeopt,
			FixedShapeArrayElementWriteRole:  spec.FixedShapeArrayElementWriteRole.String(),
			FixedShapeArrayElementReadRole:   spec.FixedShapeArrayElementReadRole.String(),
			FixedShapeReturnArrayElementRole: spec.FixedShapeReturnArrayElementRole.String(),
			LocalStringArrayTableUseRole:     spec.LocalStringArrayTableUseRole.String(),
			ReadonlyTableParamUseRole:        spec.ReadonlyTableParamUseRole.String(),
			InlineAllocationRole:             spec.InlineAllocationRole.String(),
			Tier2CallBoundaryLoopBlocker:     spec.Tier2CallBoundaryLoopBlocker,
			Tier2LoopAllocationBlocker:       spec.Tier2LoopAllocationBlocker,
		})
	}
	return rows
}

func FormatOpAuditMatrix() string {
	rows := OpAuditMatrix()
	widths := []int{2, 4, 9, 7, 6, 7, 8, 5}
	for _, row := range rows {
		widths[0] = max(widths[0], len(fmt.Sprint(int(row.Op))))
		widths[1] = max(widths[1], len(row.Name))
		widths[2] = max(widths[2], len(row.Validator))
		widths[3] = max(widths[3], len(row.Builder))
		widths[4] = max(widths[4], len(row.Oracle))
		widths[5] = max(widths[5], len(row.Emitter))
		widths[6] = max(widths[6], len(row.Regalloc))
		widths[7] = max(widths[7], len(row.Deopt))
	}

	var b strings.Builder
	writeOpAuditRow(&b, widths, "op", "name", "validator", "builder", "oracle", "emitter", "regalloc", "deopt")
	writeOpAuditSep(&b, widths)
	for _, row := range rows {
		writeOpAuditRow(&b, widths,
			fmt.Sprint(int(row.Op)),
			row.Name,
			row.Validator,
			row.Builder,
			row.Oracle,
			row.Emitter,
			row.Regalloc,
			row.Deopt,
		)
	}
	return b.String()
}

func WriteOpAuditMatrixJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(OpAuditMatrix())
}

func writeOpAuditRow(b *strings.Builder, widths []int, cols ...string) {
	for i, col := range cols {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(col)
		for pad := widths[i] - len(col); pad > 0; pad-- {
			b.WriteByte(' ')
		}
	}
	b.WriteByte('\n')
}

func writeOpAuditSep(b *strings.Builder, widths []int) {
	for i, width := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("-", width))
	}
	b.WriteByte('\n')
}

func opAuditValidator(spec OpSpec) string {
	args := spec.ArgPolicy.String()
	if spec.ArgCount.Set {
		args += "(" + spec.ArgCount.describe() + ")"
	}
	if spec.Terminator {
		args += fmt.Sprintf("/succ:%d", spec.SuccCount)
	}
	return args
}

func opAuditBuilder(spec OpSpec) string {
	if spec.ArgCount.Set {
		return "base+count"
	}
	return "base"
}

func opAuditOracle(spec OpSpec) string {
	if spec.OracleSupport == OpOracleUnsupported && spec.OracleUnsupportedReason != "" {
		return spec.OracleSupport.String() + "(" + spec.OracleUnsupportedReason + ")"
	}
	return spec.OracleSupport.String()
}

func opAuditEmitter(spec OpSpec) string {
	return spec.EmitterFamily.String()
}

func opAuditRegalloc(spec OpSpec) string {
	var flags []string
	if spec.NoSSAResult {
		flags = append(flags, "no-ssa")
	}
	if spec.RawIntResult {
		flags = append(flags, "raw-int")
	}
	if spec.RawFloatResult {
		flags = append(flags, "raw-float")
	}
	if spec.RawTablePtrResult || spec.TableResultRawTablePtr {
		flags = append(flags, "raw-table")
	}
	if spec.RawDataPtrResult {
		flags = append(flags, "raw-data")
	}
	if spec.FloatRegResult {
		flags = append(flags, "fpr")
	}
	if spec.FloatRegResultBlocked {
		flags = append(flags, "no-fpr")
	}
	if spec.TableArrayGPRInvariant {
		flags = append(flags, "gpr-inv")
	}
	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, ",")
}

func opAuditDeopt(spec OpSpec) string {
	var flags []string
	if spec.MayDeopt {
		flags = append(flags, "may")
	}
	if spec.DirectDeoptWithoutFullFlush {
		flags = append(flags, "direct")
	}
	if spec.NativeReplayMayExit {
		flags = append(flags, "replay")
	}
	if spec.NativeCalleeResumeUnsafe {
		flags = append(flags, "callee")
	}
	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, ",")
}
