package vm

import "testing"

func TestMethodJITTierCallableVarargBoundary(t *testing.T) {
	tests := []struct {
		name       string
		proto      *FuncProto
		wantTier1  bool
		wantTier2  bool
		wantLegacy bool
	}{
		{
			name:       "fixed arity",
			proto:      &FuncProto{},
			wantTier1:  true,
			wantTier2:  true,
			wantLegacy: true,
		},
		{
			name:       "declared vararg unused",
			proto:      &FuncProto{IsVarArg: true},
			wantTier1:  true,
			wantTier2:  false,
			wantLegacy: true,
		},
		{
			name:       "declared vararg reads varargs",
			proto:      &FuncProto{IsVarArg: true, UsesVarargBytecode: true},
			wantTier1:  false,
			wantTier2:  false,
			wantLegacy: false,
		},
		{
			name:       "vararg bytecode without declaration",
			proto:      &FuncProto{UsesVarargBytecode: true},
			wantTier1:  true,
			wantTier2:  false,
			wantLegacy: true,
		},
		{
			name:       "nil",
			proto:      nil,
			wantTier1:  false,
			wantTier2:  false,
			wantLegacy: false,
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
		})
	}
}
