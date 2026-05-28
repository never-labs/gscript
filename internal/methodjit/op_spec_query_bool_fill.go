package methodjit

type boolTableFillStoreLayout struct {
	TableArg   int
	KeyArg     int
	ValueArg   int
	KindSource OpBoolTableFillKindSource
}

func boolTableFillStoreLayoutForOp(op Op) (boolTableFillStoreLayout, bool) {
	spec, ok := op.Spec()
	if !ok || !spec.BoolTableFillStore ||
		spec.BoolTableFillStoreTableArg < 0 ||
		spec.BoolTableFillStoreKeyArg < 0 ||
		spec.BoolTableFillStoreValueArg < 0 ||
		spec.BoolTableFillStoreKindSource == OpBoolTableFillKindNone {
		return boolTableFillStoreLayout{}, false
	}
	return boolTableFillStoreLayout{
		TableArg:   spec.BoolTableFillStoreTableArg,
		KeyArg:     spec.BoolTableFillStoreKeyArg,
		ValueArg:   spec.BoolTableFillStoreValueArg,
		KindSource: spec.BoolTableFillStoreKindSource,
	}, true
}
