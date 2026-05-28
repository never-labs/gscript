package methodjit

import "testing"

func TestTableIntArrayBodyContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpAddInt, OpGuardTableKind, OpJump, OpNop} {
		spec, ok := op.Spec()
		if !ok || !spec.TableIntArraySwapPairsBodyBenign || !tableIntArraySwapPairsBodyBenign(op) {
			t.Fatalf("%s swap-pairs body benign contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAddInt, OpGuardTableKind, OpJump} {
		spec, ok := op.Spec()
		if !ok || !spec.TableIntArrayCopyPrefixBodyBenign || !tableIntArrayCopyPrefixBodyBenign(op) {
			t.Fatalf("%s copy-prefix body benign contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAddInt, OpSubInt, OpGuardTableKind, OpJump, OpNop} {
		spec, ok := op.Spec()
		if !ok || !spec.TableIntArrayReverseBodyBenign || !tableIntArrayReverseBodyBenign(op) {
			t.Fatalf("%s reverse-prefix body benign contract should be driven by OpSpec", op)
		}
	}
	if tableIntArrayCopyPrefixBodyBenign(OpNop) {
		t.Fatalf("%s should stay outside copy-prefix body benign policy", OpNop)
	}
}
