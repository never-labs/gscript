package methodjit

import (
	"errors"
	"fmt"
	"strings"
)

var ErrOracleUnsupported = errors.New("IR interpreter oracle unsupported")

type OracleUnsupportedError struct {
	Ops     []Op
	Reasons map[Op]string
}

func (e *OracleUnsupportedError) Error() string {
	names := make([]string, 0, len(e.Ops))
	for _, op := range e.Ops {
		if reason := e.Reasons[op]; reason != "" {
			names = append(names, op.String()+"("+reason+")")
			continue
		}
		names = append(names, op.String())
	}
	return "IR interpreter oracle unsupported op(s): " + strings.Join(names, ", ")
}

func (e *OracleUnsupportedError) Is(target error) bool {
	if target == ErrOracleUnsupported {
		return true
	}
	_, ok := target.(*OracleUnsupportedError)
	return ok
}

func ValidateOracleSupport(fn *Function) error {
	if fn == nil {
		return fmt.Errorf("IR interpreter oracle: nil function")
	}
	seen := make(map[Op]bool)
	var unsupported []Op
	reasons := make(map[Op]string)
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
				reasons[instr.Op] = opOracleUnsupportedReason(instr.Op)
			}
		}
	}
	if len(unsupported) > 0 {
		return &OracleUnsupportedError{Ops: unsupported, Reasons: reasons}
	}
	return nil
}
