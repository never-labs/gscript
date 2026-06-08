package methodjit

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/never-labs/leia/internal/runtime"
)

const (
	qQueryLoweringFallbackUnsupportedSpec         = "unsupported_spec"
	qQueryLoweringFallbackMultiUse                = "multi_use"
	qQueryLoweringFallbackMissingProto            = "missing_proto"
	qQueryLoweringFallbackBadProjectOrResultConst = "bad_project_or_result_const"
	qQueryLoweringFallbackBadSourceColumnConst    = "bad_source_column_const"
	qQueryLoweringFallbackMissingCompareRHS       = "missing_compare_rhs"
	qQueryLoweringFallbackTooManyDynamicArgs      = "too_many_dynamic_args"
	qQueryLoweringFallbackBadMaskSpecConst        = "bad_mask_spec_const"
	qQueryLoweringFallbackMissingPredicate        = "missing_predicate"
	qQueryLoweringFallbackBadOrderConst           = "bad_order_const"
	qQueryLoweringFallbackMissingRowValue         = "missing_row_value"
	qQueryLoweringFallbackOpaqueRowConst          = "opaque_row_const"
	qQueryLoweringFallbackMaskCombineUnsupported  = "mask_combine_unsupported"
)

// QQueryHotPath describes an IR pattern for the q query primitive pipeline:
// column load -> typed compare mask -> frame filter -> optional row reorder or
// prefix slice -> frame projection -> projected column load.
type QQueryHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Mask         *Instr
	MaskCombine  *Instr
	Filter       *Instr
	RowGather    *Instr
	RowSlice     *Instr
	RowOrder     *Instr
	Project      *Instr
	ResultColumn *Instr
}

func (p QQueryHotPath) Shape() string {
	prefix := "compare/filter"
	if p.Mask != nil {
		prefix = "mask/filter"
	} else if p.MaskCombine != nil {
		prefix = "mask-combine/filter"
	}
	switch {
	case p.RowOrder != nil && p.RowGather != nil:
		return prefix + "/order/gather/project/column"
	case p.RowGather != nil:
		return prefix + "/gather/project/column"
	case p.RowSlice != nil:
		return prefix + "/slice/project/column"
	default:
		return prefix + "/project/column"
	}
}

func CountQQueryHotPathShapes(paths []QQueryHotPath) map[string]int {
	if len(paths) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, path := range paths {
		counts[path.Shape()]++
	}
	return counts
}

func CountQFrameSelectColumnSpecShapes(specs []QFrameSelectColumnSpec) map[string]int {
	if len(specs) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, spec := range specs {
		shape := spec.Shape
		if shape == "" {
			shape = "unknown"
		}
		counts[shape]++
	}
	return counts
}

func CountQQueryLoweringFallbackReasons(remarks []OptimizationRemark) map[string]int {
	counts := make(map[string]int)
	for _, remark := range remarks {
		if remark.Pass != "QQueryNativeLowering" || remark.Kind != "missed" {
			continue
		}
		reason, ok := qQueryLoweringFallbackReasonFromRemark(remark.Reason)
		if !ok {
			continue
		}
		counts[reason]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func qQueryLoweringFallbackReasonFromRemark(reason string) (string, bool) {
	const prefix = "reason_code="
	for _, field := range strings.Fields(reason) {
		if strings.HasPrefix(field, prefix) {
			code := strings.TrimRight(strings.TrimPrefix(field, prefix), ",;")
			return code, code != ""
		}
	}
	return "", false
}

// DetectQQueryHotPaths returns q query primitive pipelines visible in Method
// JIT IR. It is intentionally a recognizer only: execution still uses the
// existing primitive op-exit/runtime helpers until a later lowering consumes
// this metadata.
func DetectQQueryHotPaths(fn *Function) []QQueryHotPath {
	if fn == nil {
		return nil
	}
	var out []QQueryHotPath
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpFrameColumn || len(instr.Args) != 1 {
				continue
			}
			project := valueDef(instr.Args[0], OpFrameProject)
			if project == nil || len(project.Args) != 1 {
				continue
			}
			filterInput := project.Args[0]
			var rowGather *Instr
			var rowSlice *Instr
			var rowOrder *Instr
			if gather := valueDef(filterInput, OpFrameGather); gather != nil {
				if len(gather.Args) != 2 {
					continue
				}
				rowGather = gather
				rowOrder = valueDef(gather.Args[1], OpFrameOrder)
				filterInput = gather.Args[0]
			} else if slice := valueDef(filterInput, OpFrameSlice); slice != nil {
				if len(slice.Args) != 2 {
					continue
				}
				rowSlice = slice
				filterInput = slice.Args[0]
			}
			filter := valueDef(filterInput, OpFrameFilter)
			if filter == nil || len(filter.Args) != 2 {
				continue
			}
			compare := valueDef(filter.Args[1], OpVectorCompare)
			mask := valueDef(filter.Args[1], OpFrameMask)
			maskCombine := valueDef(filter.Args[1], OpVectorMask)
			var sourceColumn *Instr
			if compare != nil {
				if len(compare.Args) != 2 {
					continue
				}
				sourceColumn = qQueryCompareColumn(compare)
				if sourceColumn == nil || len(sourceColumn.Args) != 1 {
					continue
				}
				if filter.Args[0] == nil || sourceColumn.Args[0] == nil || filter.Args[0].ID != sourceColumn.Args[0].ID {
					continue
				}
			} else if mask != nil {
				if len(mask.Args) != 1 || filter.Args[0] == nil || mask.Args[0] == nil || filter.Args[0].ID != mask.Args[0].ID {
					continue
				}
			} else if maskCombine != nil {
				if !qQueryMaskCombineUsesFrame(filter.Args[0], maskCombine) {
					continue
				}
			} else {
				continue
			}
			out = append(out, QQueryHotPath{
				SourceColumn: sourceColumn,
				Compare:      compare,
				Mask:         mask,
				MaskCombine:  maskCombine,
				Filter:       filter,
				RowGather:    rowGather,
				RowSlice:     rowSlice,
				RowOrder:     rowOrder,
				Project:      project,
				ResultColumn: instr,
			})
		}
	}
	return out
}

// QQueryHotPathRemarkPass records visible q query primitive hot paths in the
// structured optimization remark stream. It does not mutate IR; the remark is a
// handoff point for diagnostics and future native lowering policy.
func QQueryHotPathRemarkPass(fn *Function) (*Function, error) {
	paths := DetectQQueryHotPaths(fn)
	if len(paths) == 0 {
		return fn, nil
	}
	first := paths[0].ResultColumn
	blockID, valueID := 0, 0
	if first != nil {
		valueID = first.ID
		if first.Block != nil {
			blockID = first.Block.ID
		}
	}
	functionRemarks(fn).Add(
		"QQueryHotPath",
		"missed",
		blockID,
		valueID,
		OpFrameColumn,
		fmt.Sprintf("recognized %d q query primitive hot path(s), first shape %s, compare %s; native lowering pending",
			len(paths), paths[0].Shape(), qQueryHotPathPredicateName(paths[0])),
	)
	return fn, nil
}

// QQueryNativeLoweringPass folds simple q primitive hot paths into a single
// runtime-kernel op-exit. This is the first executable lowering step after
// recognition; full native codegen can target OpQFrameSelectColumn later.
func QQueryNativeLoweringPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	uses := qQueryValueUseCounts(fn)
	for _, path := range DetectQQueryHotPaths(fn) {
		if !qQueryHotPathSingleUse(path, uses) {
			qQueryLoweringFallbackRemark(fn, path, qQueryLoweringFallbackMultiUse)
			continue
		}
		spec, args, reason, ok := qQueryFrameSelectColumnSpec(fn, path)
		if !ok {
			qQueryLoweringFallbackRemark(fn, path, reason)
			continue
		}
		specIdx := len(fn.QFrameSelectColumnSpecs)
		fn.QFrameSelectColumnSpecs = append(fn.QFrameSelectColumnSpecs, spec)
		result := path.ResultColumn
		result.Op = OpQFrameSelectColumn
		result.Type = TypeAny
		result.Args = args
		result.Aux = int64(specIdx)
		result.Aux2 = 0
		qQueryNop(path.SourceColumn)
		qQueryNop(path.Compare)
		qQueryNop(path.Mask)
		qQueryNop(path.Filter)
		qQueryNop(path.RowOrder)
		qQueryNop(path.RowGather)
		qQueryNop(path.RowSlice)
		qQueryNop(path.Project)
		if result.Block != nil {
			functionRemarks(fn).Add("QQueryNativeLowering", "changed", result.Block.ID, result.ID, OpQFrameSelectColumn,
				fmt.Sprintf("lowered q query primitive hot path shape %s to typed runtime kernel op-exit", spec.Shape))
		}
	}
	return fn, nil
}

func qQueryLoweringFallbackRemark(fn *Function, path QQueryHotPath, reason string) {
	if reason == "" {
		reason = qQueryLoweringFallbackUnsupportedSpec
	}
	blockID, valueID := 0, 0
	if path.ResultColumn != nil {
		valueID = path.ResultColumn.ID
		if path.ResultColumn.Block != nil {
			blockID = path.ResultColumn.Block.ID
		}
	}
	functionRemarks(fn).Add("QQueryNativeLowering", "missed", blockID, valueID, OpFrameColumn,
		fmt.Sprintf("reason_code=%s shape=%s compare=%s; q query hot path remains on primitive fallback",
			reason, path.Shape(), qQueryHotPathPredicateName(path)))
}

func qQueryValueUseCounts(fn *Function) map[int]int {
	uses := make(map[int]int)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			for _, arg := range instr.Args {
				if arg != nil {
					uses[arg.ID]++
				}
			}
		}
	}
	return uses
}

func qQueryHotPathSingleUse(path QQueryHotPath, uses map[int]int) bool {
	for _, instr := range []*Instr{path.SourceColumn, path.Compare, path.Mask, path.MaskCombine, path.RowOrder, path.RowGather, path.RowSlice, path.Project} {
		if instr != nil && uses[instr.ID] != 1 {
			return false
		}
	}
	filterUses := 1
	if path.RowOrder != nil && path.RowGather != nil {
		filterUses = 2
	}
	if path.Filter != nil && uses[path.Filter.ID] != filterUses {
		return false
	}
	return true
}

func qQueryFrameSelectColumnSpec(fn *Function, path QQueryHotPath) (QFrameSelectColumnSpec, []*Value, string, bool) {
	if fn == nil || fn.Proto == nil || path.Filter == nil || path.Project == nil || path.ResultColumn == nil {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingProto, false
	}
	if path.ResultColumn.Aux < 0 || path.ResultColumn.Aux >= int64(len(fn.Proto.Constants)) ||
		path.Project.Aux < 0 || path.Project.Aux >= int64(len(fn.Proto.Constants)) {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadProjectOrResultConst, false
	}
	spec := QFrameSelectColumnSpec{
		Shape:             path.Shape(),
		SourceColumnConst: -1,
		MaskSpecConst:     -1,
		RowMode:           QFrameSelectColumnRowsNone,
		RowOrderConst:     -1,
		DynamicArgRole:    QFrameSelectColumnArgNone,
		ProjectConst:      int(path.Project.Aux),
		ResultColumnConst: int(path.ResultColumn.Aux),
	}
	frameArg := path.Filter.Args[0]
	args := []*Value{frameArg}
	if path.Compare != nil {
		if path.SourceColumn == nil || path.SourceColumn.Aux < 0 || path.SourceColumn.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadSourceColumnConst, false
		}
		rhs := qQueryCompareRHS(path.Compare, path.SourceColumn)
		if rhs == nil {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingCompareRHS, false
		}
		spec.SourceColumnConst = int(path.SourceColumn.Aux)
		spec.CompareOp = runtime.DenseArrayBinaryOp(path.Compare.Aux)
		if rhsConst, ok := qQueryConstRuntimeValue(rhs); ok {
			spec.CompareRHSConst = rhsConst
			spec.HasCompareRHSConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			spec.DynamicArgRole = QFrameSelectColumnArgCompareRHS
			args = append(args, rhs)
		} else {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
	} else if path.Mask != nil {
		if path.Mask.Aux < 0 || path.Mask.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadMaskSpecConst, false
		}
		spec.MaskSpecConst = int(path.Mask.Aux)
	} else if path.MaskCombine != nil {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMaskCombineUnsupported, false
	} else {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingPredicate, false
	}
	switch {
	case path.RowOrder != nil && path.RowGather != nil:
		if path.RowOrder.Aux < 0 || path.RowOrder.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadOrderConst, false
		}
		spec.RowMode = QFrameSelectColumnRowsOrderGather
		spec.RowOrderConst = int(path.RowOrder.Aux)
	case path.RowGather != nil:
		spec.RowMode = QFrameSelectColumnRowsGather
		if len(path.RowGather.Args) != 2 || path.RowGather.Args[1] == nil {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingRowValue, false
		}
		if qQueryOpaqueConst(path.RowGather.Args[1]) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackOpaqueRowConst, false
		}
		if rowConst, ok := qQueryConstRuntimeValue(path.RowGather.Args[1]); ok {
			spec.RowValueConst = rowConst
			spec.HasRowValueConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			spec.DynamicArgRole = QFrameSelectColumnArgRowValue
			args = append(args, path.RowGather.Args[1])
		} else {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
	case path.RowSlice != nil:
		spec.RowMode = QFrameSelectColumnRowsSlice
		if len(path.RowSlice.Args) != 2 || path.RowSlice.Args[1] == nil {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingRowValue, false
		}
		if qQueryOpaqueConst(path.RowSlice.Args[1]) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackOpaqueRowConst, false
		}
		if rowConst, ok := qQueryConstRuntimeValue(path.RowSlice.Args[1]); ok {
			spec.RowValueConst = rowConst
			spec.HasRowValueConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			spec.DynamicArgRole = QFrameSelectColumnArgRowValue
			args = append(args, path.RowSlice.Args[1])
		} else {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
	}
	return spec, args, "", true
}

func qQueryOpaqueConst(value *Value) bool {
	return value != nil && value.Def != nil && value.Def.Op == OpConstNil && value.Def.Type == TypeAny
}

func qQueryMaskCombineUsesFrame(frame *Value, mask *Instr) bool {
	if frame == nil || mask == nil || mask.Op != OpVectorMask || len(mask.Args) != 2 {
		return false
	}
	for _, arg := range mask.Args {
		if !qQueryMaskValueUsesFrame(frame, arg) {
			return false
		}
	}
	return true
}

func qQueryMaskValueUsesFrame(frame *Value, value *Value) bool {
	instr := valueDef(value, OpFrameMask)
	if instr != nil {
		return len(instr.Args) == 1 && instr.Args[0] != nil && instr.Args[0].ID == frame.ID
	}
	instr = valueDef(value, OpVectorCompare)
	if instr != nil {
		sourceColumn := qQueryCompareColumn(instr)
		return sourceColumn != nil &&
			len(sourceColumn.Args) == 1 &&
			sourceColumn.Args[0] != nil &&
			sourceColumn.Args[0].ID == frame.ID
	}
	instr = valueDef(value, OpVectorMask)
	if instr != nil {
		return qQueryMaskCombineUsesFrame(frame, instr)
	}
	return false
}

func qQueryConstRuntimeValue(value *Value) (runtime.Value, bool) {
	if value == nil || value.Def == nil {
		return runtime.NilValue(), false
	}
	switch value.Def.Op {
	case OpConstInt:
		return runtime.IntValue(value.Def.Aux), true
	case OpConstFloat:
		return runtime.FloatValue(math.Float64frombits(uint64(value.Def.Aux))), true
	case OpConstBool:
		return runtime.BoolValue(value.Def.Aux != 0), true
	default:
		return runtime.NilValue(), false
	}
}

func qQueryCompareRHS(compare, sourceColumn *Instr) *Value {
	if compare == nil || sourceColumn == nil {
		return nil
	}
	for _, arg := range compare.Args {
		if arg == nil || arg.ID == sourceColumn.ID {
			continue
		}
		return arg
	}
	return nil
}

func qQueryNop(instr *Instr) {
	if instr == nil {
		return
	}
	instr.Op = OpNop
	instr.Type = TypeUnknown
	instr.Args = nil
	instr.Aux = 0
	instr.Aux2 = 0
}

func formatQQueryHotPaths(paths []QQueryHotPath) string {
	if len(paths) == 0 {
		return "0 primitive pipeline(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d primitive pipeline(s)\n", len(paths))
	if counts := CountQQueryHotPathShapes(paths); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, path := range paths {
		fmt.Fprintf(&b, "  [%d] shape=%s compare=%s", i, path.Shape(), qQueryHotPathPredicateName(path))
		if path.Mask != nil {
			fmt.Fprintf(&b, " mask_aux=%d", path.Mask.Aux)
		}
		if path.RowOrder != nil {
			fmt.Fprintf(&b, " order_aux=%d", path.RowOrder.Aux)
		}
		if path.RowSlice != nil {
			fmt.Fprintf(&b, " slice=true")
		}
		if path.RowGather != nil {
			fmt.Fprintf(&b, " gather=true")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatQQueryHotPathShapeCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	shapes := make([]string, 0, len(counts))
	for shape := range counts {
		shapes = append(shapes, shape)
	}
	sort.Strings(shapes)
	parts := make([]string, 0, len(shapes))
	for _, shape := range shapes {
		parts = append(parts, fmt.Sprintf("%s=%d", shape, counts[shape]))
	}
	return strings.Join(parts, ", ")
}

func formatQQueryLoweringFallbackReasons(counts map[string]int) string {
	if len(counts) == 0 {
		return "0 fallback reason(s)\n"
	}
	return fmt.Sprintf("%d fallback reason(s): %s\n", len(counts), formatQQueryHotPathShapeCounts(counts))
}

func formatQFrameSelectColumnSpecs(specs []QFrameSelectColumnSpec) string {
	if len(specs) == 0 {
		return "0 typed runtime kernel(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d typed runtime kernel(s)\n", len(specs))
	if counts := CountQFrameSelectColumnSpecShapes(specs); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, spec := range specs {
		fmt.Fprintf(&b, "  [%d] shape=%s mask=%s rows=%s dynamic_arg=%s project_const=%d result_const=%d",
			i,
			qFrameSelectColumnSpecShape(spec),
			qFrameSelectColumnSpecMaskKind(spec),
			qFrameSelectColumnRowModeName(spec.RowMode),
			qFrameSelectColumnDynamicArgRoleName(spec.DynamicArgRole),
			spec.ProjectConst,
			spec.ResultColumnConst,
		)
		if spec.RowOrderConst >= 0 {
			fmt.Fprintf(&b, " order_const=%d", spec.RowOrderConst)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func qFrameSelectColumnSpecShape(spec QFrameSelectColumnSpec) string {
	if spec.Shape == "" {
		return "unknown"
	}
	return spec.Shape
}

func qFrameSelectColumnSpecMaskKind(spec QFrameSelectColumnSpec) string {
	if spec.MaskSpecConst >= 0 {
		return fmt.Sprintf("frame-mask:%d", spec.MaskSpecConst)
	}
	if spec.SourceColumnConst >= 0 {
		return fmt.Sprintf("compare:%s:%d", qDenseArrayCompareOpName(spec.CompareOp), spec.SourceColumnConst)
	}
	return "unknown"
}

func qDenseArrayCompareOpName(op runtime.DenseArrayBinaryOp) string {
	switch op {
	case runtime.DenseArrayEQ:
		return "=="
	case runtime.DenseArrayNE:
		return "!="
	case runtime.DenseArrayLT:
		return "<"
	case runtime.DenseArrayLE:
		return "<="
	case runtime.DenseArrayGT:
		return ">"
	case runtime.DenseArrayGE:
		return ">="
	default:
		return fmt.Sprintf("op(%d)", op)
	}
}

func qFrameSelectColumnRowModeName(mode QFrameSelectColumnRowMode) string {
	switch mode {
	case QFrameSelectColumnRowsNone:
		return "none"
	case QFrameSelectColumnRowsGather:
		return "gather"
	case QFrameSelectColumnRowsSlice:
		return "slice"
	case QFrameSelectColumnRowsOrderGather:
		return "order/gather"
	default:
		return fmt.Sprintf("unknown(%d)", mode)
	}
}

func qFrameSelectColumnDynamicArgRoleName(role QFrameSelectColumnDynamicArgRole) string {
	switch role {
	case QFrameSelectColumnArgNone:
		return "none"
	case QFrameSelectColumnArgCompareRHS:
		return "compare_rhs"
	case QFrameSelectColumnArgRowValue:
		return "row_value"
	default:
		return fmt.Sprintf("unknown(%d)", role)
	}
}

func qQueryHotPathCompareOpName(compare *Instr) string {
	if compare == nil {
		return "frame-mask"
	}
	return qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(compare.Aux))
}

func qQueryHotPathPredicateName(path QQueryHotPath) string {
	switch {
	case path.Compare != nil:
		return qQueryHotPathCompareOpName(path.Compare)
	case path.MaskCombine != nil:
		return "mask-combine"
	default:
		return "frame-mask"
	}
}

func qQueryCompareColumn(compare *Instr) *Instr {
	for _, arg := range compare.Args {
		if col := valueDef(arg, OpFrameColumn); col != nil {
			return col
		}
	}
	return nil
}

func valueDef(value *Value, op Op) *Instr {
	if value == nil || value.Def == nil || value.Def.Op != op {
		return nil
	}
	return value.Def
}
