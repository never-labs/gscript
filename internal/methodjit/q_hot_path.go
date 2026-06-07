package methodjit

// QQueryHotPath describes an IR pattern for the q query primitive pipeline:
// column load -> typed compare mask -> frame filter -> frame projection ->
// projected column load.
type QQueryHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Filter       *Instr
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
			filter := valueDef(project.Args[0], OpFrameFilter)
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
				Project:      project,
				ResultColumn: instr,
			})
		}
	}
	return out
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
