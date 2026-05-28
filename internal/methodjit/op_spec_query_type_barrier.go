package methodjit

func opIsCallLikeFactBarrier(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.CallLikeFactBarrier
}

func opIsTableMutationFirstArg(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableMutationFirstArg
}
