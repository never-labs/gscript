package methodjit

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/never-labs/leia/internal/runtime"
)

// QQueryHotPath describes an IR pattern for the q query primitive pipeline:
// column load -> typed compare mask -> frame filter -> optional row reorder or
// prefix slice -> frame projection -> projected column load.
type QQueryHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Mask         *Instr
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
			} else {
				continue
			}
			out = append(out, QQueryHotPath{
				SourceColumn: sourceColumn,
				Compare:      compare,
				Mask:         mask,
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
			len(paths), paths[0].Shape(), qQueryHotPathCompareOpName(paths[0].Compare)),
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
			continue
		}
		spec, args, ok := qQueryFrameSelectColumnSpec(fn, path)
		if !ok {
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
	for _, instr := range []*Instr{path.SourceColumn, path.Compare, path.Mask, path.RowOrder, path.RowGather, path.RowSlice, path.Project} {
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

func qQueryFrameSelectColumnSpec(fn *Function, path QQueryHotPath) (QFrameSelectColumnSpec, []*Value, bool) {
	if fn == nil || fn.Proto == nil || path.Filter == nil || path.Project == nil || path.ResultColumn == nil {
		return QFrameSelectColumnSpec{}, nil, false
	}
	if path.ResultColumn.Aux < 0 || path.ResultColumn.Aux >= int64(len(fn.Proto.Constants)) ||
		path.Project.Aux < 0 || path.Project.Aux >= int64(len(fn.Proto.Constants)) {
		return QFrameSelectColumnSpec{}, nil, false
	}
	spec := QFrameSelectColumnSpec{
		Shape:             path.Shape(),
		SourceColumnConst: -1,
		MaskSpecConst:     -1,
		RowMode:           QFrameSelectColumnRowsNone,
		RowOrderConst:     -1,
		ProjectConst:      int(path.Project.Aux),
		ResultColumnConst: int(path.ResultColumn.Aux),
	}
	frameArg := path.Filter.Args[0]
	args := []*Value{frameArg}
	if path.Compare != nil {
		if path.SourceColumn == nil || path.SourceColumn.Aux < 0 || path.SourceColumn.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, false
		}
		rhs := qQueryCompareRHS(path.Compare, path.SourceColumn)
		if rhs == nil {
			return QFrameSelectColumnSpec{}, nil, false
		}
		spec.SourceColumnConst = int(path.SourceColumn.Aux)
		spec.CompareOp = runtime.DenseArrayBinaryOp(path.Compare.Aux)
		if rhsConst, ok := qQueryConstRuntimeValue(rhs); ok {
			spec.CompareRHSConst = rhsConst
			spec.HasCompareRHSConst = true
		} else if path.RowGather == nil && path.RowSlice == nil && path.RowOrder == nil {
			args = append(args, rhs)
		} else {
			return QFrameSelectColumnSpec{}, nil, false
		}
	} else if path.Mask != nil {
		if path.Mask.Aux < 0 || path.Mask.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, false
		}
		spec.MaskSpecConst = int(path.Mask.Aux)
	} else {
		return QFrameSelectColumnSpec{}, nil, false
	}
	switch {
	case path.RowOrder != nil && path.RowGather != nil:
		if path.RowOrder.Aux < 0 || path.RowOrder.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, false
		}
		spec.RowMode = QFrameSelectColumnRowsOrderGather
		spec.RowOrderConst = int(path.RowOrder.Aux)
	case path.RowGather != nil:
		spec.RowMode = QFrameSelectColumnRowsGather
		if len(path.RowGather.Args) != 2 || path.RowGather.Args[1] == nil {
			return QFrameSelectColumnSpec{}, nil, false
		}
		if qQueryOpaqueConst(path.RowGather.Args[1]) {
			return QFrameSelectColumnSpec{}, nil, false
		}
		args = append(args, path.RowGather.Args[1])
	case path.RowSlice != nil:
		spec.RowMode = QFrameSelectColumnRowsSlice
		if len(path.RowSlice.Args) != 2 || path.RowSlice.Args[1] == nil {
			return QFrameSelectColumnSpec{}, nil, false
		}
		if qQueryOpaqueConst(path.RowSlice.Args[1]) {
			return QFrameSelectColumnSpec{}, nil, false
		}
		args = append(args, path.RowSlice.Args[1])
	}
	return spec, args, true
}

func qQueryOpaqueConst(value *Value) bool {
	return value != nil && value.Def != nil && value.Def.Op == OpConstNil && value.Def.Type == TypeAny
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
		fmt.Fprintf(&b, "  [%d] shape=%s compare=%s", i, path.Shape(), qQueryHotPathCompareOpName(path.Compare))
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

func qQueryHotPathCompareOpName(compare *Instr) string {
	if compare == nil {
		return "frame-mask"
	}
	switch runtime.DenseArrayBinaryOp(compare.Aux) {
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
		return fmt.Sprintf("op(%d)", compare.Aux)
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
