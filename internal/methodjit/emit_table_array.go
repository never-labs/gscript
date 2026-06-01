//go:build darwin && arm64

// emit_table_array.go implements ARM64 code generation for table array/dynamic
// key operations (OpNewTable, OpGetTable, OpSetTable) in the Method JIT.
// These handle integer-keyed array access with type-specialized fast paths
// and exit-resume fallbacks for complex cases.

package methodjit

import (
	"github.com/never-labs/leia/internal/jit"
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

const (
	tableArrayHeaderFlagHoisted    int64 = 1 << 8
	tableArrayLoadFlagProvenString int64 = 1 << 0

	// tableArrayStoreFlagAllowGrow lets OpTableArrayStore use the same
	// capacity-only append/sparse typed-array path as OpSetTable. Misses still
	// precise-deopt unless tableArrayStoreFlagExitResumeOnMiss is also set.
	tableArrayStoreFlagAllowGrow int64 = 1 << iota
	// tableArrayStoreFlagExitResumeOnMiss routes misses through SetTable and
	// resumes. The resume path refreshes table-array data/len facts so later
	// raw array operations do not keep stale backing pointers after fallback.
	tableArrayStoreFlagExitResumeOnMiss
)

type tableArrayBoundKey struct {
	tableID int
	keyID   int
}

func fbKindToAK(kind int64) (uint16, bool) {
	switch kind {
	case int64(vm.FBKindMixed):
		return jit.AKMixed, true
	case int64(vm.FBKindInt):
		return jit.AKInt, true
	case int64(vm.FBKindFloat):
		return jit.AKFloat, true
	case int64(vm.FBKindBool):
		return jit.AKBool, true
	default:
		return 0, false
	}
}

func tableArrayOffsets(kind int64) (dataOff, lenOff int, ok bool) {
	switch kind {
	case int64(vm.FBKindMixed):
		return jit.TableOffArray, jit.TableOffArrayLen, true
	case int64(vm.FBKindInt):
		return jit.TableOffIntArray, jit.TableOffIntArrayLen, true
	case int64(vm.FBKindFloat):
		return jit.TableOffFloatArray, jit.TableOffFloatArrayLen, true
	case int64(vm.FBKindBool):
		return jit.TableOffBoolArray, jit.TableOffBoolArrayLen, true
	default:
		return 0, 0, false
	}
}

func tableArrayStoreOffsets(kind int64) (dataOff, lenOff, capOff int, ok bool) {
	switch kind {
	case int64(vm.FBKindMixed):
		return jit.TableOffArray, jit.TableOffArrayLen, jit.TableOffArrayCap, true
	case int64(vm.FBKindInt):
		return jit.TableOffIntArray, jit.TableOffIntArrayLen, jit.TableOffIntArrayCap, true
	case int64(vm.FBKindFloat):
		return jit.TableOffFloatArray, jit.TableOffFloatArrayLen, jit.TableOffFloatArrayCap, true
	case int64(vm.FBKindBool):
		return jit.TableOffBoolArray, jit.TableOffBoolArrayLen, jit.TableOffBoolArrayCap, true
	default:
		return 0, 0, 0, false
	}
}

type tableArrayRawStoreConfig struct {
	labelPrefix             string
	kind                    int64
	valueID                 int
	tableReg                jit.Reg
	keyReg                  jit.Reg
	dataReg                 jit.Reg
	lenReg                  jit.Reg
	missLabel               string
	successLabel            string
	loadDataFromTable       bool
	priorLoadBounds         bool
	upperBoundSafe          bool
	keysDirtyAlreadyWritten bool
	allowGrowWithinCapacity bool
	carryLenOnGrow          bool
	fallthroughOnSuccess    bool
}

func (ec *emitContext) emitTableArrayKeyToReg(key *Value, deoptLabel string) bool {
	if key == nil {
		return false
	}
	asm := ec.asm
	keyID := key.ID
	if kv, isConst := ec.constInts[keyID]; isConst {
		asm.LoadImm64(jit.X1, kv)
		return true
	}
	if ec.hasReg(keyID) && ec.valueReprOf(keyID) == valueReprRawInt {
		reg := ec.physReg(keyID)
		if reg != jit.X1 {
			asm.MOVreg(jit.X1, reg)
		}
		return true
	}
	if ec.irTypes[keyID] == TypeInt {
		ec.resolveValueToReg(keyID, jit.X1)
		ec.emitUnboxInt48(jit.X1)
		return true
	}
	ec.resolveValueToReg(keyID, jit.X1)
	ec.emitIntTagCheckBranch(jit.X1, jit.X4, jit.X5, jit.CondNE, deoptLabel)
	ec.emitUnboxInt48(jit.X1)
	return true
}

func (ec *emitContext) intNonNegative(id int) bool {
	return functionNumericFacts(ec.fn).IsIntNonNegative(id)
}

func (ec *emitContext) tableArrayUpperBoundSafe(id int) bool {
	return functionLoopSpecializationFacts(ec.fn).TableArrayUpperBoundIsSafe(id)
}

func (ec *emitContext) tableArrayLowerBoundSafe(id int) bool {
	return functionLoopSpecializationFacts(ec.fn).TableArrayLowerBoundIsSafe(id)
}

func (ec *emitContext) tableArrayKeyKnownNonZero(id int) bool {
	if kv, ok := ec.constInts[id]; ok {
		return kv != 0
	}
	numeric := functionNumericFacts(ec.fn)
	if numeric == nil {
		return false
	}
	r, ok := numeric.IntRange(id)
	return ok && r.known && (r.min > 0 || r.max < 0)
}

func (ec *emitContext) tableArrayKeyBounded(tableID, keyID int) bool {
	if ec == nil || ec.tableArrayBoundedKeys == nil {
		return false
	}
	return ec.tableArrayBoundedKeys[tableArrayBoundKey{tableID: tableID, keyID: keyID}]
}

func (ec *emitContext) clearTableArrayBoundedKeys() {
	if ec != nil && len(ec.tableArrayBoundedKeys) > 0 {
		ec.tableArrayBoundedKeys = make(map[tableArrayBoundKey]bool)
	}
}

func (ec *emitContext) isLocalNewTableWithoutMetatable(v *Value) bool {
	return ec != nil && ec.localNewTablesNoMetatable && v != nil && v.Def != nil && v.Def.Op == OpNewTable
}

func (ec *emitContext) localNewTableFBKind(v *Value) (uint16, bool) {
	if !ec.isLocalNewTableWithoutMetatable(v) {
		return 0, false
	}
	_, kind := unpackNewTableAux2(v.Def.Aux2)
	switch kind {
	case runtime.ArrayMixed:
		return uint16(vm.FBKindMixed), true
	case runtime.ArrayInt:
		return uint16(vm.FBKindInt), true
	case runtime.ArrayFloat:
		return uint16(vm.FBKindFloat), true
	case runtime.ArrayBool:
		return uint16(vm.FBKindBool), true
	default:
		return 0, false
	}
}

func functionHasNoTableMetatableMutationSurface(fn *Function) bool {
	if fn == nil {
		return false
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if opIsTableMetatableMutationBarrier(instr.Op) {
				return false
			}
		}
	}
	return true
}

func (ec *emitContext) setTablePreservesLocalArrayFacts(instr *Instr) bool {
	if ec == nil || instr == nil || len(instr.Args) < 3 || instr.Args[0] == nil || !ec.isLocalNewTableWithoutMetatable(instr.Args[0]) {
		return false
	}
	switch instr.Aux2 {
	case int64(vm.FBKindMixed):
		return true
	case int64(vm.FBKindInt):
		valueID := instr.Args[2].ID
		_, isConst := ec.constInts[valueID]
		return isConst || (ec.hasReg(valueID) && ec.valueReprOf(valueID) == valueReprRawInt) || ec.irTypes[valueID] == TypeInt
	case int64(vm.FBKindFloat):
		return ec.irTypes[instr.Args[2].ID] == TypeFloat
	case int64(vm.FBKindBool):
		valueID := instr.Args[2].ID
		_, isConst := ec.constBools[valueID]
		return isConst || ec.irTypes[valueID] == TypeBool || ec.irTypes[valueID] == TypeUnknown && instr.Args[2].Def != nil && instr.Args[2].Def.Op == OpConstNil
	default:
		return false
	}
}
