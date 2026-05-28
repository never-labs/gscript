package methodjit

import "testing"

func assertOpSpecTarget(t *testing.T, owner Op, field string, target Op) {
	t.Helper()
	if target == 0 || target == OpMax {
		return
	}
	if target < 0 || target >= OpMax {
		t.Fatalf("%s.%s targets invalid op %d", owner, field, target)
	}
	if _, ok := target.Spec(); !ok {
		t.Fatalf("%s.%s targets op %d without OpSpec", owner, field, target)
	}
}
