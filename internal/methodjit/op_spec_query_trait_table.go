package methodjit

func opIsBoolTableFillBodyBenign(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableFillBodyBenign
}

func opIsBoolTableFillStore(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableFillStore
}

func opIsBoolTableCountLoadBodyBenign(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountLoadBodyBenign
}

func opIsBoolTableCountLoad(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountLoad
}

func opIsBoolTableCountIncrementBenign(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountIncrementBenign
}

func opIsBoolTableCountIncrement(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountIncrement
}

func opIsTableArraySwapPureBetween(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArraySwapPureBetween
}

func opIsTableArrayRegionGlobalBarrier(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionGlobalBarrier
}

func opIsTableArrayRegionAliasingCall(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionAliasingCall
}

func opIsTableArrayRegionAliasingAlways(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionAliasingAlways
}

func opIsTableArrayRegionTableMutation(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionTableMutation
}

func opIsTableArrayGPRInvariant(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayGPRInvariant
}

func tableArrayGPRInvariantUseMask(op Op) uint8 {
	spec, ok := op.Spec()
	if !ok {
		return 0
	}
	return spec.TableArrayGPRInvariantUseMask
}

func tableArrayGPRInvariantRank(op Op) int {
	spec, ok := op.Spec()
	if !ok {
		return 1
	}
	return spec.TableArrayGPRInvariantRank
}

func opIsStaticTableLenBenignUse(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.StaticTableLenBenignUse
}

func tableArrayKeyArgIndex(op Op) (int, bool) {
	spec, ok := op.Spec()
	return spec.TableArrayKeyArgIndex, ok && spec.TableArrayKeyArgIndex >= 0
}

type tableArrayAccessLayout struct {
	TableArg int
	DataArg  int
	LenArg   int
	KeyArg   int
}

func tableArrayAccessLayoutForOp(op Op) (tableArrayAccessLayout, bool) {
	spec, ok := op.Spec()
	if !ok || spec.TableArrayDataArgIndex < 0 || spec.TableArrayLenArgIndex < 0 || spec.TableArrayKeyArgIndex < 0 {
		return tableArrayAccessLayout{}, false
	}
	return tableArrayAccessLayout{
		TableArg: spec.TableArrayTableArgIndex,
		DataArg:  spec.TableArrayDataArgIndex,
		LenArg:   spec.TableArrayLenArgIndex,
		KeyArg:   spec.TableArrayKeyArgIndex,
	}, true
}

func tableArrayFactRole(op Op) OpTableArrayFactRole {
	spec, ok := op.Spec()
	if !ok {
		return OpTableArrayFactNone
	}
	return spec.TableArrayFactRole
}

func opIsTableMetatableMutationBarrier(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableMetatableMutationBarrier
}

func opIsNestedFloatPhiOverrideSafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NestedFloatPhiOverrideSafe
}
