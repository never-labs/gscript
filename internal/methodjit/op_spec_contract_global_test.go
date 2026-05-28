package methodjit

import "testing"

func TestOpSpecGlobalAccessTraits(t *testing.T) {
	if !opIsGlobalRead(OpGetGlobal) {
		t.Fatalf("GetGlobal should be classified as a global read by OpSpec")
	}
	if !opIsGlobalWrite(OpSetGlobal) {
		t.Fatalf("SetGlobal should be classified as a global write by OpSpec")
	}
	for _, op := range []Op{OpGuardGlobalConst, OpGetField, OpSetField, OpCall} {
		if opIsGlobalRead(op) || opIsGlobalWrite(op) {
			t.Fatalf("%s should not be classified as direct global access", op)
		}
	}
}
