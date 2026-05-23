package vm

import "testing"

func TestMethodJITTierCallableVarargBoundary(t *testing.T) {
	tests := []struct {
		name        string
		proto       *FuncProto
		wantTier1   bool
		wantTier2   bool
		wantLegacy  bool
		tier1Reason string
		tier2Reason string
	}{
		{
			name:        "fixed arity",
			proto:       &FuncProto{},
			wantTier1:   true,
			wantTier2:   true,
			wantLegacy:  true,
			tier1Reason: MethodJITCallableReasonFixedArity,
			tier2Reason: MethodJITCallableReasonFixedArity,
		},
		{
			name:        "declared vararg unused",
			proto:       &FuncProto{IsVarArg: true},
			wantTier1:   true,
			wantTier2:   true,
			wantLegacy:  true,
			tier1Reason: MethodJITCallableReasonDeclaredVarargTier1,
			tier2Reason: MethodJITCallableReasonDeclaredVarargTier2,
		},
		{
			name:        "declared vararg reads varargs",
			proto:       &FuncProto{IsVarArg: true, UsesVarargBytecode: true},
			wantTier1:   false,
			wantTier2:   false,
			wantLegacy:  false,
			tier1Reason: MethodJITCallableReasonOPVarargNeedsVMFrame,
			tier2Reason: MethodJITCallableReasonOPVarargNeedsVMFrame,
		},
		{
			name:        "vararg bytecode without declaration",
			proto:       &FuncProto{UsesVarargBytecode: true},
			wantTier1:   true,
			wantTier2:   false,
			wantLegacy:  true,
			tier1Reason: MethodJITCallableReasonOPVarargTier1,
			tier2Reason: MethodJITCallableReasonOPVarargNeedsVMFrame,
		},
		{
			name:        "nil",
			proto:       nil,
			wantTier1:   false,
			wantTier2:   false,
			wantLegacy:  false,
			tier1Reason: MethodJITCallableReasonNilProto,
			tier2Reason: MethodJITCallableReasonNilProto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.proto.MethodJITTier1Callable(); got != tt.wantTier1 {
				t.Fatalf("MethodJITTier1Callable() = %v, want %v", got, tt.wantTier1)
			}
			if got := tt.proto.MethodJITTier2Callable(); got != tt.wantTier2 {
				t.Fatalf("MethodJITTier2Callable() = %v, want %v", got, tt.wantTier2)
			}
			if got := tt.proto.MethodJITCallable(); got != tt.wantLegacy {
				t.Fatalf("MethodJITCallable() = %v, want %v", got, tt.wantLegacy)
			}
			tier1 := tt.proto.MethodJITTier1CallableDecision()
			if tier1.Allowed != tt.wantTier1 || tier1.Reason != tt.tier1Reason || tier1.Tier != MethodJITTier1 {
				t.Fatalf("MethodJITTier1CallableDecision() = %+v, want allowed=%v reason=%q tier=%s",
					tier1, tt.wantTier1, tt.tier1Reason, MethodJITTier1)
			}
			tier2 := tt.proto.MethodJITTier2CallableDecision()
			if tier2.Allowed != tt.wantTier2 || tier2.Reason != tt.tier2Reason || tier2.Tier != MethodJITTier2 {
				t.Fatalf("MethodJITTier2CallableDecision() = %+v, want allowed=%v reason=%q tier=%s",
					tier2, tt.wantTier2, tt.tier2Reason, MethodJITTier2)
			}
		})
	}
}
