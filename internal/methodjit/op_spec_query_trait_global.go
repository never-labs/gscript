package methodjit

func opIsGlobalRead(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.EmitterFamily == OpEmitterGlobal && spec.SideEffect == OpSideEffectRead
}

func opIsGlobalWrite(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.EmitterFamily == OpEmitterGlobal && spec.SideEffect == OpSideEffectWrite
}
