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

// QVectorWhereHotPath describes a q vector conditional projection:
// typed mask -> vector where. The true/false operands may be frame columns or
// scalars; this keeps the shape visible before a later fused vector lowering.
type QVectorWhereHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Mask         *Instr
	MaskCombine  *Instr
	TrueColumn   *Instr
	FalseColumn  *Instr
	Where        *Instr
}

// QVectorReduceHotPath describes a q vector aggregation primitive. The input
// may be a frame column, gathered vector, conditional vector, or another dense
// vector expression that is reduced through a typed runtime op-exit.
type QVectorReduceHotPath struct {
	SourceColumn *Instr
	Gather       *Instr
	Where        *Instr
	Reduce       *Instr
}

// QVectorRuntimeKernel records vector primitives that already execute through
// typed runtime helpers/op-exits and should be visible in q diagnostics.
type QVectorRuntimeKernel struct {
	Instr     *Instr
	Kernel    string
	ShapeName string
	Detail    string
}

func (p QQueryHotPath) Shape() string {
	if p.Compare == nil && p.Mask == nil && p.MaskCombine == nil {
		switch {
		case p.RowOrder != nil && p.RowGather != nil:
			return "order/gather/project/column"
		case p.RowGather != nil:
			return "gather/project/column"
		case p.RowSlice != nil:
			return "slice/project/column"
		default:
			return "project/column"
		}
	}
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

func (p QVectorWhereHotPath) Shape() string {
	prefix := "compare"
	if p.Mask != nil {
		prefix = "mask"
	} else if p.MaskCombine != nil {
		prefix = "mask-combine"
	}
	return prefix + "/vector-where"
}

func (p QVectorReduceHotPath) Shape() string {
	switch {
	case p.Where != nil:
		return "where/vector-reduce"
	case p.Gather != nil:
		return "gather/vector-reduce"
	case p.SourceColumn != nil:
		return "column/vector-reduce"
	default:
		return "vector/vector-reduce"
	}
}

func (k QVectorRuntimeKernel) Shape() string {
	if k.ShapeName == "" {
		return "unknown"
	}
	return k.ShapeName
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

func CountQVectorWhereHotPathShapes(paths []QVectorWhereHotPath) map[string]int {
	if len(paths) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, path := range paths {
		counts[path.Shape()]++
	}
	return counts
}

func CountQVectorReduceHotPathShapes(paths []QVectorReduceHotPath) map[string]int {
	if len(paths) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, path := range paths {
		counts[path.Shape()]++
	}
	return counts
}

func CountQVectorRuntimeKernelShapes(kernels []QVectorRuntimeKernel) map[string]int {
	if len(kernels) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, kernel := range kernels {
		counts[kernel.Shape()]++
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
			compare, mask, maskCombine := (*Instr)(nil), (*Instr)(nil), (*Instr)(nil)
			var sourceColumn *Instr
			if filter != nil {
				if len(filter.Args) != 2 {
					continue
				}
				compare = valueDef(filter.Args[1], OpVectorCompare)
				mask = valueDef(filter.Args[1], OpFrameMask)
				maskCombine = valueDef(filter.Args[1], OpVectorMask)
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

// DetectQVectorWhereHotPaths returns q vector conditional-select pipelines
// visible in Method JIT IR. It is diagnostic metadata today; native lowering
// still uses OpVectorWhere's typed runtime op-exit.
func DetectQVectorWhereHotPaths(fn *Function) []QVectorWhereHotPath {
	if fn == nil {
		return nil
	}
	var out []QVectorWhereHotPath
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpVectorWhere || len(instr.Args) != 3 {
				continue
			}
			compare, mask, maskCombine := qVectorWherePredicate(instr.Args[0])
			if compare == nil && mask == nil && maskCombine == nil {
				continue
			}
			sourceColumn := (*Instr)(nil)
			if compare != nil {
				sourceColumn = qQueryCompareColumn(compare)
				if sourceColumn == nil {
					continue
				}
			}
			out = append(out, QVectorWhereHotPath{
				SourceColumn: sourceColumn,
				Compare:      compare,
				Mask:         mask,
				MaskCombine:  maskCombine,
				TrueColumn:   valueDef(instr.Args[1], OpFrameColumn),
				FalseColumn:  valueDef(instr.Args[2], OpFrameColumn),
				Where:        instr,
			})
		}
	}
	return out
}

// DetectQVectorReduceHotPaths returns q vector aggregate primitives visible in
// Method JIT IR. These already execute through typed runtime op-exits; the
// diagnostic shape is used to track how much aggregate work still falls back.
func DetectQVectorReduceHotPaths(fn *Function) []QVectorReduceHotPath {
	if fn == nil {
		return nil
	}
	var out []QVectorReduceHotPath
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpVectorReduce || len(instr.Args) != 1 {
				continue
			}
			arg := instr.Args[0]
			out = append(out, QVectorReduceHotPath{
				SourceColumn: valueDef(arg, OpFrameColumn),
				Gather:       valueDef(arg, OpVectorGather),
				Where:        valueDef(arg, OpVectorWhere),
				Reduce:       instr,
			})
		}
	}
	return out
}

// DetectQVectorRuntimeKernels returns vector primitives that are carried as
// typed runtime kernels in Method JIT. This intentionally covers standalone
// vector gather/compare/mask/reduce/scan plus conditional vector projection.
func DetectQVectorRuntimeKernels(fn *Function) []QVectorRuntimeKernel {
	if fn == nil {
		return nil
	}
	var out []QVectorRuntimeKernel
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpVectorGather:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorGather", ShapeName: "vector-gather"})
			case OpVectorCompare:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorCompare", ShapeName: "vector-compare", Detail: "op=" + qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(instr.Aux))})
			case OpVectorMask:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorMask", ShapeName: "vector-mask", Detail: "op=" + qDenseArrayMaskOpName(runtime.DenseArrayMaskOp(instr.Aux))})
			case OpVectorWhere:
				shape := "vector-where"
				detail := ""
				if path := qVectorWhereHotPath(instr); path.Where != nil {
					shape = path.Shape()
					detail = "predicate=" + qVectorWherePredicateName(path)
				}
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorWhere", ShapeName: shape, Detail: detail})
			case OpVectorReduce:
				path := qVectorReduceHotPath(instr)
				shape := "vector/vector-reduce"
				detail := "op=" + qDenseArrayReduceOpName(runtime.DenseArrayReduceOp(instr.Aux))
				if path.Reduce != nil {
					shape = path.Shape()
					detail = "op=" + qVectorReduceOpName(path) + " input=" + qVectorReduceInputName(path)
				}
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorReduce", ShapeName: shape, Detail: detail})
			case OpQVectorWhereReduce:
				detail := "op=" + qDenseArrayReduceOpName(runtime.DenseArrayReduceOp(instr.Aux))
				if len(instr.Args) == 3 {
					detail += " predicate=" + qVectorWherePredicateValueDetail(instr.Args[0])
				}
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "QVectorWhereReduce", ShapeName: qVectorWhereReduceShape(instr), Detail: detail})
			case OpVectorScan:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorScan", ShapeName: "vector-scan"})
			}
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
		qQueryNopPredicateIfSingleUse(path, uses)
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
	qVectorWhereReduceLoweringPass(fn, uses)
	return fn, nil
}

func qVectorWhereReduceLoweringPass(fn *Function, uses map[int]int) {
	if fn == nil {
		return
	}
	for _, path := range DetectQVectorReduceHotPaths(fn) {
		if path.Reduce == nil || path.Where == nil || uses[path.Where.ID] != 1 || len(path.Where.Args) != 3 {
			continue
		}
		reduce := path.Reduce
		where := path.Where
		reduce.Op = OpQVectorWhereReduce
		reduce.Type = TypeAny
		reduce.Args = append([]*Value(nil), where.Args...)
		reduce.Aux2 = 0
		qQueryNop(where)
		if reduce.Block != nil {
			functionRemarks(fn).Add("QQueryNativeLowering", "changed", reduce.Block.ID, reduce.ID, OpQVectorWhereReduce,
				fmt.Sprintf("lowered q vector hot path shape %s to fused typed runtime kernel op-exit", qVectorWhereReduceShape(reduce)))
		}
	}
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
	for _, instr := range []*Instr{path.RowOrder, path.RowGather, path.RowSlice, path.Project} {
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
	if fn == nil || fn.Proto == nil || path.Project == nil || path.ResultColumn == nil {
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
		MaskRoot:          -1,
		RowMode:           QFrameSelectColumnRowsNone,
		RowOrderConst:     -1,
		DynamicArgRole:    QFrameSelectColumnArgNone,
		ProjectConst:      int(path.Project.Aux),
		ResultColumnConst: int(path.ResultColumn.Aux),
	}
	frameArg := qQueryHotPathFrameArg(path)
	if frameArg == nil {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingProto, false
	}
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
		root, reason, ok := qQueryFrameMaskTermSpec(fn, &spec, path.MaskCombine.Value(), &args)
		if !ok {
			return QFrameSelectColumnSpec{}, nil, reason, false
		}
		spec.MaskRoot = root
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

func qQueryHotPathFrameArg(path QQueryHotPath) *Value {
	switch {
	case path.Filter != nil && len(path.Filter.Args) >= 1:
		return path.Filter.Args[0]
	case path.RowGather != nil && len(path.RowGather.Args) >= 1:
		return path.RowGather.Args[0]
	case path.RowSlice != nil && len(path.RowSlice.Args) >= 1:
		return path.RowSlice.Args[0]
	case path.Project != nil && len(path.Project.Args) >= 1:
		return path.Project.Args[0]
	default:
		return nil
	}
}

func qQueryOpaqueConst(value *Value) bool {
	return value != nil && value.Def != nil && value.Def.Op == OpConstNil && value.Def.Type == TypeAny
}

func qQueryFrameMaskTermSpec(fn *Function, spec *QFrameSelectColumnSpec, value *Value, args *[]*Value) (int, string, bool) {
	if fn == nil || fn.Proto == nil || spec == nil || value == nil || args == nil {
		return -1, qQueryLoweringFallbackMissingPredicate, false
	}
	if mask := valueDef(value, OpFrameMask); mask != nil {
		if mask.Aux < 0 || mask.Aux >= int64(len(fn.Proto.Constants)) {
			return -1, qQueryLoweringFallbackBadMaskSpecConst, false
		}
		return qFrameMaskAppendTerm(spec, QFrameMaskTermSpec{
			Kind:              QFrameMaskTermFrameMask,
			MaskSpecConst:     int(mask.Aux),
			SourceColumnConst: -1,
			LeftTerm:          -1,
			RightTerm:         -1,
		}), "", true
	}
	if compare := valueDef(value, OpVectorCompare); compare != nil {
		sourceColumn := qQueryCompareColumn(compare)
		if sourceColumn == nil || sourceColumn.Aux < 0 || sourceColumn.Aux >= int64(len(fn.Proto.Constants)) {
			return -1, qQueryLoweringFallbackBadSourceColumnConst, false
		}
		rhs := qQueryCompareRHS(compare, sourceColumn)
		if rhs == nil {
			return -1, qQueryLoweringFallbackMissingCompareRHS, false
		}
		term := QFrameMaskTermSpec{
			Kind:              QFrameMaskTermCompare,
			SourceColumnConst: int(sourceColumn.Aux),
			MaskSpecConst:     -1,
			CompareOp:         runtime.DenseArrayBinaryOp(compare.Aux),
			LeftTerm:          -1,
			RightTerm:         -1,
		}
		if rhsConst, ok := qQueryConstRuntimeValue(rhs); ok {
			term.CompareRHSConst = rhsConst
			term.HasCompareRHSConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			term.DynamicCompareRHS = true
			spec.DynamicArgRole = QFrameSelectColumnArgCompareRHS
			*args = append(*args, rhs)
		} else {
			return -1, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
		return qFrameMaskAppendTerm(spec, term), "", true
	}
	if combine := valueDef(value, OpVectorMask); combine != nil {
		if len(combine.Args) != 2 {
			return -1, qQueryLoweringFallbackMissingPredicate, false
		}
		left, reason, ok := qQueryFrameMaskTermSpec(fn, spec, combine.Args[0], args)
		if !ok {
			return -1, reason, false
		}
		right, reason, ok := qQueryFrameMaskTermSpec(fn, spec, combine.Args[1], args)
		if !ok {
			return -1, reason, false
		}
		return qFrameMaskAppendTerm(spec, QFrameMaskTermSpec{
			Kind:              QFrameMaskTermCombine,
			SourceColumnConst: -1,
			MaskSpecConst:     -1,
			CombineOp:         runtime.DenseArrayMaskOp(combine.Aux),
			LeftTerm:          left,
			RightTerm:         right,
		}), "", true
	}
	return -1, qQueryLoweringFallbackMissingPredicate, false
}

func qVectorWherePredicate(value *Value) (*Instr, *Instr, *Instr) {
	if compare := valueDef(value, OpVectorCompare); compare != nil {
		return compare, nil, nil
	}
	if mask := valueDef(value, OpFrameMask); mask != nil {
		return nil, mask, nil
	}
	if combine := valueDef(value, OpVectorMask); combine != nil {
		return nil, nil, combine
	}
	return nil, nil, nil
}

func qVectorWhereHotPath(instr *Instr) QVectorWhereHotPath {
	if instr == nil || instr.Op != OpVectorWhere || len(instr.Args) != 3 {
		return QVectorWhereHotPath{}
	}
	compare, mask, maskCombine := qVectorWherePredicate(instr.Args[0])
	if compare == nil && mask == nil && maskCombine == nil {
		return QVectorWhereHotPath{}
	}
	sourceColumn := (*Instr)(nil)
	if compare != nil {
		sourceColumn = qQueryCompareColumn(compare)
		if sourceColumn == nil {
			return QVectorWhereHotPath{}
		}
	}
	return QVectorWhereHotPath{
		SourceColumn: sourceColumn,
		Compare:      compare,
		Mask:         mask,
		MaskCombine:  maskCombine,
		TrueColumn:   valueDef(instr.Args[1], OpFrameColumn),
		FalseColumn:  valueDef(instr.Args[2], OpFrameColumn),
		Where:        instr,
	}
}

func qVectorWhereReduceShape(instr *Instr) string {
	if instr == nil || len(instr.Args) != 3 {
		return "vector-where/vector-reduce"
	}
	return qVectorWherePredicateValueName(instr.Args[0]) + "/vector-where/vector-reduce"
}

func qVectorWherePredicateValueName(value *Value) string {
	compare, mask, maskCombine := qVectorWherePredicate(value)
	switch {
	case compare != nil:
		return "compare"
	case mask != nil:
		return "mask"
	case maskCombine != nil:
		return "mask-combine"
	default:
		return "vector"
	}
}

func qVectorWherePredicateValueDetail(value *Value) string {
	compare, mask, maskCombine := qVectorWherePredicate(value)
	switch {
	case compare != nil:
		return "compare " + qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(compare.Aux))
	case mask != nil:
		return "mask"
	case maskCombine != nil:
		return "mask-combine " + qDenseArrayMaskOpName(runtime.DenseArrayMaskOp(maskCombine.Aux))
	default:
		return "vector"
	}
}

func qVectorReduceHotPath(instr *Instr) QVectorReduceHotPath {
	if instr == nil || instr.Op != OpVectorReduce || len(instr.Args) != 1 {
		return QVectorReduceHotPath{}
	}
	arg := instr.Args[0]
	return QVectorReduceHotPath{
		SourceColumn: valueDef(arg, OpFrameColumn),
		Gather:       valueDef(arg, OpVectorGather),
		Where:        valueDef(arg, OpVectorWhere),
		Reduce:       instr,
	}
}

func qFrameMaskAppendTerm(spec *QFrameSelectColumnSpec, term QFrameMaskTermSpec) int {
	spec.MaskTerms = append(spec.MaskTerms, term)
	return len(spec.MaskTerms) - 1
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

func qQueryNopPredicateIfSingleUse(path QQueryHotPath, uses map[int]int) {
	switch {
	case path.Compare != nil:
		qQueryNopCompareIfSingleUse(path.Compare, uses)
	case path.Mask != nil:
		qQueryNopIfSingleUse(path.Mask, uses)
	case path.MaskCombine != nil:
		qQueryNopMaskTreeIfSingleUse(path.MaskCombine, uses)
	}
}

func qQueryNopMaskTreeIfSingleUse(instr *Instr, uses map[int]int) {
	if instr == nil || uses[instr.ID] != 1 {
		return
	}
	args := append([]*Value(nil), instr.Args...)
	for _, arg := range args {
		if child := valueDef(arg, OpVectorMask); child != nil {
			qQueryNopMaskTreeIfSingleUse(child, uses)
			continue
		}
		if child := valueDef(arg, OpVectorCompare); child != nil {
			qQueryNopCompareIfSingleUse(child, uses)
			continue
		}
		if child := valueDef(arg, OpFrameMask); child != nil {
			qQueryNopIfSingleUse(child, uses)
		}
	}
	qQueryNop(instr)
}

func qQueryNopCompareIfSingleUse(compare *Instr, uses map[int]int) {
	if compare == nil || uses[compare.ID] != 1 {
		return
	}
	qQueryNopIfSingleUse(qQueryCompareColumn(compare), uses)
	qQueryNop(compare)
}

func qQueryNopIfSingleUse(instr *Instr, uses map[int]int) {
	if instr != nil && uses[instr.ID] == 1 {
		qQueryNop(instr)
	}
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

func formatQVectorWhereHotPaths(paths []QVectorWhereHotPath) string {
	if len(paths) == 0 {
		return "0 vector conditional pipeline(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d vector conditional pipeline(s)\n", len(paths))
	if counts := CountQVectorWhereHotPathShapes(paths); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, path := range paths {
		fmt.Fprintf(&b, "  [%d] shape=%s predicate=%s true=%s false=%s\n",
			i,
			path.Shape(),
			qVectorWherePredicateName(path),
			qVectorWhereOperandName(path.TrueColumn),
			qVectorWhereOperandName(path.FalseColumn),
		)
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

func formatQTypedVectorRuntimeKernelReport(kernels []QVectorRuntimeKernel) string {
	if len(kernels) == 0 {
		return "0 typed vector runtime kernel(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d typed vector runtime kernel(s)\n", len(kernels))
	if counts := CountQVectorRuntimeKernelShapes(kernels); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, kernel := range kernels {
		fmt.Fprintf(&b, "  [%d] shape=%s kernel=%s", i, kernel.Shape(), kernel.Kernel)
		if kernel.Detail != "" {
			fmt.Fprintf(&b, " %s", kernel.Detail)
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
	if len(spec.MaskTerms) > 0 {
		return fmt.Sprintf("mask-terms:%d root=%d", len(spec.MaskTerms), spec.MaskRoot)
	}
	if spec.MaskSpecConst >= 0 {
		return fmt.Sprintf("frame-mask:%d", spec.MaskSpecConst)
	}
	if spec.SourceColumnConst >= 0 {
		return fmt.Sprintf("compare:%s:%d", qDenseArrayCompareOpName(spec.CompareOp), spec.SourceColumnConst)
	}
	return "none"
}

func qVectorWherePredicateName(path QVectorWhereHotPath) string {
	switch {
	case path.Compare != nil:
		return "compare " + qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(path.Compare.Aux))
	case path.Mask != nil:
		return "frame-mask"
	case path.MaskCombine != nil:
		return "mask-combine"
	default:
		return "unknown"
	}
}

func qVectorWhereOperandName(column *Instr) string {
	if column == nil {
		return "scalar"
	}
	return "frame-column"
}

func qVectorReduceOpName(path QVectorReduceHotPath) string {
	if path.Reduce == nil {
		return "unknown"
	}
	switch runtime.DenseArrayReduceOp(path.Reduce.Aux) {
	case runtime.DenseArrayReduceSum:
		return "sum"
	case runtime.DenseArrayReduceMin:
		return "min"
	case runtime.DenseArrayReduceMax:
		return "max"
	case runtime.DenseArrayReduceMean:
		return "mean"
	default:
		return fmt.Sprintf("op(%d)", path.Reduce.Aux)
	}
}

func qVectorReduceInputName(path QVectorReduceHotPath) string {
	switch {
	case path.Where != nil:
		return "vector-where"
	case path.Gather != nil:
		return "vector-gather"
	case path.SourceColumn != nil:
		return "frame-column"
	default:
		return "vector"
	}
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

func qDenseArrayMaskOpName(op runtime.DenseArrayMaskOp) string {
	switch op {
	case runtime.DenseArrayMaskAnd:
		return "and"
	case runtime.DenseArrayMaskOr:
		return "or"
	case runtime.DenseArrayMaskXor:
		return "xor"
	case runtime.DenseArrayMaskAndNot:
		return "andnot"
	default:
		return fmt.Sprintf("op(%d)", op)
	}
}

func qDenseArrayReduceOpName(op runtime.DenseArrayReduceOp) string {
	switch op {
	case runtime.DenseArrayReduceSum:
		return "sum"
	case runtime.DenseArrayReduceMin:
		return "min"
	case runtime.DenseArrayReduceMax:
		return "max"
	case runtime.DenseArrayReduceMean:
		return "mean"
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
	case path.Filter == nil:
		return "none"
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
