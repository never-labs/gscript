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

type OracleSupportSummary struct {
	BySupport map[OpOracleSupport][]Op
	Reasons   map[Op]string
}

func ClassifyOracleSupport(fn *Function) (OracleSupportSummary, error) {
	if fn == nil {
		return OracleSupportSummary{}, fmt.Errorf("IR interpreter oracle: nil function")
	}
	summary := OracleSupportSummary{
		BySupport: make(map[OpOracleSupport][]Op),
		Reasons:   make(map[Op]string),
	}
	seen := make(map[Op]bool)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || seen[instr.Op] {
				continue
			}
			seen[instr.Op] = true
			support := opOracleSupport(instr.Op)
			summary.BySupport[support] = append(summary.BySupport[support], instr.Op)
			if support == OpOracleUnsupported {
				summary.Reasons[instr.Op] = opOracleUnsupportedReason(instr.Op)
			}
		}
	}
	return summary, nil
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
	summary, err := ClassifyOracleSupport(fn)
	if err != nil {
		return err
	}
	unsupported := summary.BySupport[OpOracleUnsupported]
	if len(unsupported) > 0 {
		return &OracleUnsupportedError{Ops: unsupported, Reasons: summary.Reasons}
	}
	return nil
}
