package methodjit

import "testing"

func TestOpSpecLICMLoopEffectRoles(t *testing.T) {
	cases := []struct {
		op   Op
		role OpLICMLoopEffectRole
	}{
		{OpSetField, OpLICMLoopEffectFieldWrite},
		{OpFieldStore, OpLICMLoopEffectFieldSlotWrite},
		{OpSetTable, OpLICMLoopEffectTableShapeWrite},
		{OpTableArrayStore, OpLICMLoopEffectArrayElementWrite},
		{OpTableArraySwap, OpLICMLoopEffectArrayElementWrite},
		{OpAppend, OpLICMLoopEffectTableArrayWrite},
		{OpSetList, OpLICMLoopEffectTableArrayWrite},
		{OpSetGlobal, OpLICMLoopEffectGlobalWrite},
		{OpSetUpval, OpLICMLoopEffectUpvalueWrite},
		{OpCall, OpLICMLoopEffectCall},
		{OpResume, OpLICMLoopEffectResume},
		{OpSelf, OpLICMLoopEffectSelf},
		{OpAddInt, OpLICMLoopEffectNone},
		{OpTableArraySwapPairs, OpLICMLoopEffectNone},
	}
	for _, tc := range cases {
		if got := opLICMLoopEffectRole(tc.op); got != tc.role {
			t.Fatalf("%s LICM loop-effect role = %v, want %v", tc.op, got, tc.role)
		}
	}
}
