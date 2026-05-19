package vm

func protoStringConstant(proto *FuncProto, idx int) (string, bool) {
	if proto == nil || idx < 0 || idx >= len(proto.Constants) || !proto.Constants[idx].IsString() {
		return "", false
	}
	return proto.Constants[idx].Str(), true
}
