package methodjit

import (
	"fmt"
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
	switch {
	case p.RowOrder != nil && p.RowGather != nil:
		return "filter/order/gather/project/column"
	case p.RowGather != nil:
		return "filter/gather/project/column"
	case p.RowSlice != nil:
		return "filter/slice/project/column"
	default:
		return "filter/project/column"
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
