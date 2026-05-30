//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/Never-Labs/gscript/internal/vm"
)

func dumpProtoBytecode(t *testing.T, proto *vm.FuncProto) {
	t.Helper()
	for pc, inst := range proto.Code {
		t.Logf("[%02d] %s A=%d B=%d C=%d Bx=%d sBx=%d", pc, vm.OpName(vm.DecodeOp(inst)),
			vm.DecodeA(inst), vm.DecodeB(inst), vm.DecodeC(inst), vm.DecodeBx(inst), vm.DecodesBx(inst))
	}
}
