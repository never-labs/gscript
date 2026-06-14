//go:build darwin && arm64

package methodjit

import (
	"errors"
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
)

func executeQTypedRuntimeValueIntoSlot(regs []runtime.Value, absSlot, planID int, exec func() (runtime.Value, bool, error), outOfRangeMessage, unhandledFormat string) error {
	if absSlot < 0 || absSlot >= len(regs) {
		return errors.New(outOfRangeMessage)
	}
	out, handled, err := exec()
	if err != nil {
		return err
	}
	if !handled {
		return fmt.Errorf(unhandledFormat, planID)
	}
	regs[absSlot] = out
	return nil
}
