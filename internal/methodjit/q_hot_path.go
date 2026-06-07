package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
)

// QQueryHotPath describes an IR pattern for the q query primitive pipeline:
// column load -> typed compare mask -> frame filter -> optional row reorder or
// prefix slice -> frame projection -> projected column load.
type QQueryHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Filter       *Instr
	RowGather    *Instr
	RowSlice     *Instr
	Project      *Instr
	ResultColumn *Instr
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
			if gather := valueDef(filterInput, OpFrameGather); gather != nil {
				if len(gather.Args) != 2 {
					continue
				}
				rowGather = gather
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
			if compare == nil || len(compare.Args) != 2 {
				continue
			}
			sourceColumn := qQueryCompareColumn(compare)
			if sourceColumn == nil || len(sourceColumn.Args) != 1 {
				continue
			}
			if filter.Args[0] == nil || sourceColumn.Args[0] == nil || filter.Args[0].ID != sourceColumn.Args[0].ID {
				continue
			}
			out = append(out, QQueryHotPath{
				SourceColumn: sourceColumn,
				Compare:      compare,
				Filter:       filter,
				RowGather:    rowGather,
				RowSlice:     rowSlice,
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
		fmt.Sprintf("recognized %d q query primitive hot path(s), first compare %s; native lowering pending",
			len(paths), qQueryHotPathCompareOpName(paths[0].Compare)),
	)
	return fn, nil
}

func qQueryHotPathCompareOpName(compare *Instr) string {
	if compare == nil {
		return "unknown"
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
