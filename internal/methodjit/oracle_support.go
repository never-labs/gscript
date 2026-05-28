package methodjit

import (
	"fmt"
	"strings"
)

type OracleUnsupportedError struct {
	Ops []Op
}

func (e *OracleUnsupportedError) Error() string {
	names := make([]string, 0, len(e.Ops))
	for _, op := range e.Ops {
		names = append(names, op.String())
	}
	return "IR interpreter oracle unsupported op(s): " + strings.Join(names, ", ")
}

func (e *OracleUnsupportedError) Is(target error) bool {
	_, ok := target.(*OracleUnsupportedError)
	return ok
}

func ValidateOracleSupport(fn *Function) error {
	if fn == nil {
		return fmt.Errorf("IR interpreter oracle: nil function")
	}
	seen := make(map[Op]bool)
	var unsupported []Op
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || seen[instr.Op] {
				continue
			}
			seen[instr.Op] = true
			if opOracleSupport(instr.Op) == OpOracleUnsupported {
				unsupported = append(unsupported, instr.Op)
			}
		}
	}
	if len(unsupported) > 0 {
		return &OracleUnsupportedError{Ops: unsupported}
	}
	return nil
}
