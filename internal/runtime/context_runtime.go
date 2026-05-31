package runtime

func contextDoneAndErr(v Value) (*Channel, Value, bool) {
	return ScriptContextDoneAndErr(v)
}

func contextCancelledValue(done *Channel, errFn Value) (Value, bool) {
	return ContextCancelledValue(done, errFn)
}

func ContextCancelledValue(done *Channel, errFn Value) (Value, bool) {
	select {
	case _, ok := <-done.ch:
		if !ok {
			return contextErrValue(errFn), true
		}
		return StringValue("cancelled"), true
	default:
		return NilValue(), false
	}
}

func contextErrValue(errFn Value) Value {
	return ScriptContextErrValue(errFn)
}
