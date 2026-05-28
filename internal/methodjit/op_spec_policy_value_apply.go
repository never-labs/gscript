package methodjit

func applyOpSpecValuePolicies(op Op, spec *OpSpec) {
	applyOpSpecValueResultPolicies(op, spec)
	applyOpSpecValueMatrixPolicies(op, spec)
	applyOpSpecValueTablePolicies(op, spec)
}

func applyOpSpecValueResultPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opNoSSAResultPolicies) {
		spec.NoSSAResult = opNoSSAResultPolicies[op]
	}
	if int(op) < len(opRawIntResultPolicies) {
		spec.RawIntResult = opRawIntResultPolicies[op]
	}
	if int(op) < len(opRawTablePtrResultPolicies) {
		spec.RawTablePtrResult = opRawTablePtrResultPolicies[op]
	}
	if int(op) < len(opRawDataPtrResultPolicies) {
		spec.RawDataPtrResult = opRawDataPtrResultPolicies[op]
	}
	if int(op) < len(opRawFloatResultPolicies) {
		spec.RawFloatResult = opRawFloatResultPolicies[op]
	}
}

func applyOpSpecValueMatrixPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opMatrixNativePolicies) {
		spec.MatrixNative = opMatrixNativePolicies[op]
	}
	if int(op) < len(opMatrixLoweredOpPolicies) && opMatrixLoweredOpPolicies[op].Set {
		spec.MatrixLoweredOp = opMatrixLoweredOpPolicies[op].Op
	}
	if int(op) < len(opMatrixRowLoweredOpPolicies) && opMatrixRowLoweredOpPolicies[op].Set {
		spec.MatrixRowLoweredOp = opMatrixRowLoweredOpPolicies[op].Op
	}
	if int(op) < len(opMatrixRowConstLoweredOpPolicies) && opMatrixRowConstLoweredOpPolicies[op].Set {
		spec.MatrixRowConstLoweredOp = opMatrixRowConstLoweredOpPolicies[op].Op
	}
}

func applyOpSpecValueTablePolicies(op Op, spec *OpSpec) {
	if int(op) < len(opTableArrayGPRInvariantPolicies) {
		spec.TableArrayGPRInvariant = opTableArrayGPRInvariantPolicies[op]
	}
	if int(op) < len(opTableArrayGPRInvariantRankPolicies) && opTableArrayGPRInvariantRankPolicies[op] != 0 {
		spec.TableArrayGPRInvariantRank = int(opTableArrayGPRInvariantRankPolicies[op]) - 1
	}
	if int(op) < len(opTableArrayGPRInvariantUseMaskPolicies) {
		spec.TableArrayGPRInvariantUseMask = opTableArrayGPRInvariantUseMaskPolicies[op]
	}
	if int(op) < len(opTableArrayKeyArgIndexPolicies) && opTableArrayKeyArgIndexPolicies[op] != 0 {
		spec.TableArrayKeyArgIndex = int(opTableArrayKeyArgIndexPolicies[op]) - 1
	}
	if int(op) < len(opTableArrayTableArgIndexPolicies) && opTableArrayTableArgIndexPolicies[op] != 0 {
		spec.TableArrayTableArgIndex = int(opTableArrayTableArgIndexPolicies[op]) - 1
	}
	if int(op) < len(opTableArrayDataArgIndexPolicies) && opTableArrayDataArgIndexPolicies[op] != 0 {
		spec.TableArrayDataArgIndex = int(opTableArrayDataArgIndexPolicies[op]) - 1
	}
	if int(op) < len(opTableArrayLenArgIndexPolicies) && opTableArrayLenArgIndexPolicies[op] != 0 {
		spec.TableArrayLenArgIndex = int(opTableArrayLenArgIndexPolicies[op]) - 1
	}
	if int(op) < len(opTableArrayLoweredOpPolicies) && opTableArrayLoweredOpPolicies[op].Set {
		spec.TableArrayLoweredOp = opTableArrayLoweredOpPolicies[op].Op
	}
	if int(op) < len(opTableArrayNestedLoweredOpPolicies) && opTableArrayNestedLoweredOpPolicies[op].Set {
		spec.TableArrayNestedLoweredOp = opTableArrayNestedLoweredOpPolicies[op].Op
	}
	if int(op) < len(opTableArraySwapLoweredOpPolicies) && opTableArraySwapLoweredOpPolicies[op].Set {
		spec.TableArraySwapLoweredOp = opTableArraySwapLoweredOpPolicies[op].Op
	}
	if int(op) < len(opTableIntArraySwapPairsBodyBenignPolicies) {
		spec.TableIntArraySwapPairsBodyBenign = opTableIntArraySwapPairsBodyBenignPolicies[op]
	}
	if int(op) < len(opTableIntArrayCopyPrefixBodyBenignPolicies) {
		spec.TableIntArrayCopyPrefixBodyBenign = opTableIntArrayCopyPrefixBodyBenignPolicies[op]
	}
	if int(op) < len(opTableIntArrayReverseBodyBenignPolicies) {
		spec.TableIntArrayReverseBodyBenign = opTableIntArrayReverseBodyBenignPolicies[op]
	}
	if int(op) < len(opFixedShapeArrayElementWriteRolePolicies) {
		spec.FixedShapeArrayElementWriteRole = opFixedShapeArrayElementWriteRolePolicies[op]
	}
	if int(op) < len(opFixedShapeArrayElementReadRolePolicies) {
		spec.FixedShapeArrayElementReadRole = opFixedShapeArrayElementReadRolePolicies[op]
	}
	if int(op) < len(opFixedShapeReturnArrayElementRolePolicies) {
		spec.FixedShapeReturnArrayElementRole = opFixedShapeReturnArrayElementRolePolicies[op]
	}
	if int(op) < len(opLocalStringArrayTableUseRolePolicies) {
		spec.LocalStringArrayTableUseRole = opLocalStringArrayTableUseRolePolicies[op]
	}
	if int(op) < len(opLocalStringArrayTableArgIndexPolicies) && opLocalStringArrayTableArgIndexPolicies[op] != 0 {
		spec.LocalStringArrayTableArgIndex = int(opLocalStringArrayTableArgIndexPolicies[op]) - 1
	}
	if int(op) < len(opReadonlyTableParamUseRolePolicies) {
		spec.ReadonlyTableParamUseRole = opReadonlyTableParamUseRolePolicies[op]
	}
	if int(op) < len(opInlineAllocationRolePolicies) {
		spec.InlineAllocationRole = opInlineAllocationRolePolicies[op]
	}
}
