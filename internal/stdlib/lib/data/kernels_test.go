package data

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

type queryKernelFallbackTestExpr struct{}

func (queryKernelFallbackTestExpr) EvalRow(Frame, int) (any, error) {
	return Symbol("fallback"), nil
}

type queryKernelFingerprintFallbackExpr struct {
	Name string
}

func (e queryKernelFingerprintFallbackExpr) EvalRow(Frame, int) (any, error) {
	return e.Name, nil
}

type queryKernelFingerprintHiddenStruct struct {
	hidden int
}

func TestTypedKernelRegistryHelpers(t *testing.T) {
	mask := make([]bool, 4)
	if ok := typedKernels.CompareMask(NewI32([]int32{1, 2, 3, 2}), OpGE, int32(2), mask); !ok {
		t.Fatal("typed compare kernel did not match i32 column")
	}
	if want := []bool{false, true, true, true}; !reflect.DeepEqual(mask, want) {
		t.Fatalf("typed compare mask = %v, want %v", mask, want)
	}

	mask = make([]bool, 4)
	if ok := typedKernels.WithinMask(NewTimestamp([]Timestamp{10, 20, 30, 20}), Timestamp(10), Timestamp(20), true, mask); !ok {
		t.Fatal("typed within kernel did not match timestamp column")
	}
	if want := []bool{true, true, false, true}; !reflect.DeepEqual(mask, want) {
		t.Fatalf("typed within mask = %v, want %v", mask, want)
	}

	n, ok, err := typedKernels.NumericAt(NewF32([]float32{1.25, 2.5}), 1)
	if err != nil {
		t.Fatalf("NumericAt returned error: %v", err)
	}
	if !ok || n != 2.5 {
		t.Fatalf("NumericAt = %v, %v; want 2.5, true", n, ok)
	}

	if ok := typedKernels.CompareMask(NewI32([]int32{1}), OpEQ, int64(1), make([]bool, 1)); ok {
		t.Fatal("typed compare kernel matched incompatible literal kind")
	}
}

func TestTypedCompareMaskOperatorsAndBounds(t *testing.T) {
	tests := []struct {
		name  string
		array Array
		op    Op
		value any
		want  []bool
	}{
		{name: "bool ne", array: NewBool([]bool{false, true}), op: OpNE, value: true, want: []bool{true, false}},
		{name: "i64 ge coerces int literal", array: NewI64([]int64{-1, 0, 1}), op: OpGE, value: 0, want: []bool{false, true, true}},
		{name: "f64 lt coerces float32 literal", array: NewF64([]float64{1, 1.5, 2}), op: OpLT, value: float32(1.5), want: []bool{true, false, false}},
		{name: "symbol le", array: NewSymbols([]string{"a", "b", "c"}), op: OpLE, value: Symbol("b"), want: []bool{true, true, false}},
		{name: "symbol eq string literal", array: NewSymbols([]string{"a", "b", "c"}), op: OpEQ, value: "b", want: []bool{false, true, false}},
		{name: "string eq symbol literal", array: NewString([]string{"a", "b", "c"}), op: OpEQ, value: Symbol("b"), want: []bool{false, true, false}},
		{name: "timestamp gt", array: NewTimestamp([]Timestamp{10, 20, 30}), op: OpGT, value: Timestamp(20), want: []bool{false, false, true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]bool, tt.array.Len())
			if ok := typedKernels.CompareMask(tt.array, tt.op, tt.value, got); !ok {
				t.Fatalf("typed compare kernel did not match %s", tt.array.Kind())
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("typed compare mask = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypedWithinMaskOpenClosedAndNullBounds(t *testing.T) {
	open := make([]bool, 4)
	if ok := typedKernels.WithinMask(NewI32([]int32{9, 10, 20, 21}), int32(10), int32(20), false, open); !ok {
		t.Fatal("typed within kernel did not match i32 column")
	}
	if want := []bool{false, true, false, false}; !reflect.DeepEqual(open, want) {
		t.Fatalf("open typed within mask = %v, want %v", open, want)
	}

	closed := make([]bool, 4)
	if ok := typedKernels.WithinMask(NewI32([]int32{9, 10, 20, 21}), int32(10), int32(20), true, closed); !ok {
		t.Fatal("typed within kernel did not match i32 column")
	}
	if want := []bool{false, true, true, false}; !reflect.DeepEqual(closed, want) {
		t.Fatalf("closed typed within mask = %v, want %v", closed, want)
	}

	if ok := typedKernels.WithinMask(NewI32([]int32{10}), NullValue, int32(20), true, make([]bool, 1)); ok {
		t.Fatal("typed within kernel accepted null lower bound")
	}
	if ok := typedKernels.WithinMask(NewI32([]int32{10}), int32(10), NullValue, true, make([]bool, 1)); ok {
		t.Fatal("typed within kernel accepted null upper bound")
	}

	symbolString := make([]bool, 3)
	if ok := typedKernels.WithinMask(NewSymbols([]string{"AAPL", "MSFT", "NVDA"}), "AAPL", "NVDA", false, symbolString); !ok {
		t.Fatal("typed within kernel did not match symbol column with string bounds")
	}
	if want := []bool{true, true, false}; !reflect.DeepEqual(symbolString, want) {
		t.Fatalf("symbol/string typed within mask = %v, want %v", symbolString, want)
	}

	stringSymbol := make([]bool, 3)
	if ok := typedKernels.WithinMask(NewString([]string{"AAPL", "MSFT", "NVDA"}), Symbol("AAPL"), Symbol("NVDA"), true, stringSymbol); !ok {
		t.Fatal("typed within kernel did not match string column with symbol bounds")
	}
	if want := []bool{true, true, true}; !reflect.DeepEqual(stringSymbol, want) {
		t.Fatalf("string/symbol typed within mask = %v, want %v", stringSymbol, want)
	}
}

func TestTypedBinScalarAndVectorBoundaries(t *testing.T) {
	domain := WithArrayAttribute(NewI64([]int64{10, 20, 20, 40}), ArrayAttributeSorted)
	index, ok, err := typedKernels.Bin(domain, int64(20))
	if err != nil {
		t.Fatalf("Bin scalar returned error: %v", err)
	}
	if !ok || index != int64(2) {
		t.Fatalf("Bin scalar = %v, %v; want 2, true", index, ok)
	}

	vector, ok, err := typedKernels.Bin(domain, NewColumn("q", []any{int64(5), int64(10), nil, int64(30), int64(50)}).Data)
	if err != nil {
		t.Fatalf("Bin vector returned error: %v", err)
	}
	if !ok {
		t.Fatal("Bin vector did not match typed kernel")
	}
	if got, want := vector.(Array).Values(), []any{int64(-1), int64(0), int64(-1), int64(2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Bin vector values = %v, want %v", got, want)
	}

	if _, _, err := typedKernels.Bin(nil, int64(1)); err == nil {
		t.Fatal("Bin accepted nil domain")
	}
}

func TestQueryKernelSupportReasonForTimeSeriesVectorTransforms(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("ts", []any{TimestampFromUnixNanos(1_000), TimestampFromUnixNanos(2_000)}),
		NewColumn("bid", []any{[]any{100.0, 101.0}, []any{102.0}}),
	)

	supported := []struct {
		name string
		plan QueryPlan
	}{
		{
			name: "bucket floor by expression",
			plan: QueryPlan{
				ByExprs: []SelectItem{{
					Name: "bucket",
					Expr: BucketFloorExpr{Expr: ColumnRef{Name: "ts"}, Interval: TimespanFromNanos(1_000)},
				}},
				Aggregates: []Aggregate{{Name: "n", Func: "count"}},
			},
		},
		{
			name: "window list aggregate projection",
			plan: QueryPlan{
				Select: []SelectItem{{
					Name: "avg_bid",
					Expr: ListAggregateExpr{Func: "avg", Expr: ColumnRef{Name: "bid"}},
				}},
			},
		},
	}

	for _, tt := range supported {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := QueryKernelSupportReason(tt.plan)
			if !ok {
				t.Fatalf("QueryKernelSupportReason ok = false, reason %q; want supported", reason)
			}
			kernel, ok, err := CompileQueryKernel(frame, tt.plan)
			if err != nil || !ok || kernel == nil {
				t.Fatalf("CompileQueryKernel = kernel %v, ok %v, err %v; want compiled kernel", kernel, ok, err)
			}
			if _, err := kernel.Exec(frame); err != nil {
				t.Fatalf("kernel Exec returned error: %v", err)
			}
			got, err := ExecQueryKernelOrPlan(kernel, tt.plan, frame)
			if err != nil {
				t.Fatalf("ExecQueryKernelOrPlan kernel path returned error: %v", err)
			}
			want, err := Exec(frame, tt.plan)
			if err != nil {
				t.Fatalf("QueryPlan Exec returned error: %v", err)
			}
			if !SameSchema(got, want) || got.Len() != want.Len() {
				t.Fatalf("ExecQueryKernelOrPlan kernel frame schema/len = %#v/%d, want %#v/%d", got.Schema(), got.Len(), want.Schema(), want.Len())
			}
		})
	}

	fallback := QueryPlan{
		Select: []SelectItem{{
			Name: "kind",
			Expr: queryKernelFallbackTestExpr{},
		}},
	}
	ok, reason := QueryKernelSupportReason(fallback)
	if ok {
		t.Fatalf("QueryKernelSupportReason custom expression ok = true, want false")
	}
	want := "unsupported expression"
	if !strings.Contains(reason, want) {
		t.Fatalf("QueryKernelSupportReason reason = %q, want it to contain %q", reason, want)
	}
	if kernel, ok, err := CompileQueryKernel(frame, fallback); err != nil || ok || kernel != nil {
		t.Fatalf("CompileQueryKernel custom expression = kernel %v, ok %v, err %v; want fallback without error", kernel, ok, err)
	}
	if ok, reason, err := QueryKernelCompileReason(frame, fallback); err != nil || ok || !strings.Contains(reason, want) {
		t.Fatalf("QueryKernelCompileReason custom expression = ok %v reason %q err %v; want fallback reason containing %q", ok, reason, err, want)
	}
	validation := QueryPlan{
		Select: []SelectItem{{
			Name: "missing",
			Expr: ColumnRef{Name: "missing"},
		}},
	}
	if ok, reason, err := QueryKernelCompileReason(frame, validation); err == nil || ok || reason != "" {
		t.Fatalf("QueryKernelCompileReason validation = ok %v reason %q err %v; want validation error", ok, reason, err)
	}

	executableFallback := QueryPlan{
		Select: []SelectItem{{
			Name: "marker",
			Expr: queryKernelFallbackTestExpr{},
		}},
		LimitN: -1,
	}
	if kernel, ok, err := CompileQueryKernel(frame, executableFallback); err != nil || ok || kernel != nil {
		t.Fatalf("CompileQueryKernel custom expression = kernel %v, ok %v, err %v; want fallback without error", kernel, ok, err)
	}
	got, err := ExecQueryKernelOrPlan(nil, executableFallback, frame)
	if err != nil {
		t.Fatalf("ExecQueryKernelOrPlan fallback returned error: %v", err)
	}
	assertColumnValues(t, got, "marker", []any{Symbol("fallback"), Symbol("fallback")})
}

func TestQueryKernelCacheKeyIsSchemaStable(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20})},
	)
	sameSchema := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"NVDA"})},
		Column{Name: "qty", Data: NewI32([]int32{30})},
	)
	differentOrder := mustFrame(t,
		Column{Name: "qty", Data: NewI32([]int32{30})},
		Column{Name: "sym", Data: NewSymbols([]string{"NVDA"})},
	)
	plan := QueryPlan{
		Where: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(10)}},
		Select: []SelectItem{{
			Name: "sym",
			Expr: ColumnRef{Name: "sym"},
		}},
	}

	key := QueryKernelCacheKey("select sym from trades where qty>=10", frame, plan)
	keyParts, ok := ParseSchemaStableCacheKey(key)
	if !ok {
		t.Fatalf("ParseSchemaStableCacheKey(%q) failed", key)
	}
	if keyParts.Namespace != "select sym from trades where qty>=10" || keyParts.Kind != "kernel" || keyParts.SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("kernel key parts = %+v, want namespace/query kind/kernel schema hash/%s", keyParts, frame.SchemaFingerprint())
	}
	if len(keyParts.Extra) != 1 || keyParts.Extra[0] != QueryKernelPlanFingerprint(plan) {
		t.Fatalf("kernel key extra = %#v, want plan fingerprint", keyParts.Extra)
	}
	if _, ok := ParseSchemaStableCacheKey("3:abc"); ok {
		t.Fatalf("ParseSchemaStableCacheKey accepted unterminated key")
	}
	if got := QueryKernelCacheKey("select sym from trades where qty>=10", sameSchema, plan); got != key {
		t.Fatalf("same schema key = %q, want %q", got, key)
	}
	if got := QueryKernelCacheKey("select sym from trades where qty>=10", differentOrder, plan); got == key {
		t.Fatalf("different column order key = %q, want it to differ", got)
	}
	if got := QueryKernelCacheKey("other source", frame, plan); got == key {
		t.Fatalf("different namespace key = %q, want it to differ", got)
	}
	schemaKey := FrameSchemaCacheKey("select sym from trades where qty>=10", frame)
	if got := FrameSchemaCacheKey("select sym from trades where qty>=10", sameSchema); got != schemaKey {
		t.Fatalf("same schema frame key = %q, want %q", got, schemaKey)
	}
	if got := FrameSchemaCacheKey("select sym from trades where qty>=10", differentOrder); got == schemaKey {
		t.Fatalf("different column order frame key = %q, want it to differ", got)
	}
	if got := FrameSchemaCacheKey("other source", frame); got == schemaKey {
		t.Fatalf("different namespace frame key = %q, want it to differ", got)
	}
	alignedPlanKey := QueryAlignedPlanCacheKey("select sym from trades where qty>=10", frame)
	if got := QueryAlignedPlanCacheKey("select sym from trades where qty>=10", sameSchema); got != alignedPlanKey {
		t.Fatalf("same schema aligned plan key = %q, want %q", got, alignedPlanKey)
	}
	if got := QueryAlignedPlanCacheKey("select sym from trades where qty>=10", differentOrder); got == alignedPlanKey {
		t.Fatalf("different column order aligned plan key = %q, want it to differ", got)
	}
	if got := QueryAlignedPlanCacheKey("other source", frame); got == alignedPlanKey {
		t.Fatalf("different namespace aligned plan key = %q, want it to differ", got)
	}
	alignedMutationKey := QueryAlignedMutationCacheKey("select sym from trades where qty>=10", frame)
	if got := QueryAlignedMutationCacheKey("select sym from trades where qty>=10", sameSchema); got != alignedMutationKey {
		t.Fatalf("same schema aligned mutation key = %q, want %q", got, alignedMutationKey)
	}
	if got := QueryAlignedMutationCacheKey("select sym from trades where qty>=10", differentOrder); got == alignedMutationKey {
		t.Fatalf("different column order aligned mutation key = %q, want it to differ", got)
	}
	if got := QueryAlignedMutationCacheKey("other source", frame); got == alignedMutationKey {
		t.Fatalf("different namespace aligned mutation key = %q, want it to differ", got)
	}
	if alignedMutationKey == alignedPlanKey {
		t.Fatalf("aligned mutation key = %q, want different from plan key", alignedMutationKey)
	}
	if alignedPlanKey == schemaKey {
		t.Fatalf("aligned plan key = %q, want different from frame schema key", alignedPlanKey)
	}

	changedPlan := plan
	changedPlan.LimitN = 1
	if got := QueryKernelCacheKey("select sym from trades where qty>=10", frame, changedPlan); got == key {
		t.Fatalf("different plan key = %q, want it to differ", got)
	}
	if got := QueryKernelCacheKey("", frame, plan); got == key {
		t.Fatalf("empty namespace kernel key = %q, want it to differ from namespaced key", got)
	}

	if got := querySchemaStableCacheKey("ns", "kernel", frame, "a\x00b"); got == querySchemaStableCacheKey("ns", "kernel", frame, "a", "b") {
		t.Fatalf("single extra containing separator collided with split extras: %q", got)
	}
	if got := querySchemaStableCacheKey("ns\x00kernel", "schema", frame); got == querySchemaStableCacheKey("ns", "kernel\x00schema", frame) {
		t.Fatalf("namespace/kind boundary collided: %q", got)
	}
	if got := QueryKernelCacheKey("ns", frame, plan); got == QueryAlignedPlanCacheKey("ns", frame) {
		t.Fatalf("kernel key collided with aligned plan key: %q", got)
	}
	if got := QueryKernelCacheKey("ns", frame, plan); got == QueryAlignedMutationCacheKey("ns", frame) {
		t.Fatalf("kernel key collided with aligned mutation key: %q", got)
	}
}

func TestCompiledQueryKernelClonesPlanMutableFields(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "AAPL"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 5})},
	)
	whereValues := []any{Symbol("AAPL")}
	selectItems := []SelectItem{{
		Name: "qty",
		Expr: ColumnRef{Name: "qty"},
	}}
	orderBy := []OrderSpec{{Column: "qty"}}
	plan := QueryPlan{
		Where:   In{Expr: ColumnRef{Name: "sym"}, Values: whereValues},
		Select:  selectItems,
		OrderBy: orderBy,
		LimitN:  -1,
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil || !ok || kernel == nil {
		t.Fatalf("CompileQueryKernel = kernel %v, ok %v, err %v; want compiled kernel", kernel, ok, err)
	}

	whereValues[0] = Symbol("MSFT")
	selectItems[0] = SelectItem{Name: "sym", Expr: ColumnRef{Name: "sym"}}
	orderBy[0] = OrderSpec{Column: "qty", Desc: true}

	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("compiled kernel Exec returned error after source plan mutation: %v", err)
	}
	assertColumnValues(t, got, "qty", []any{int32(5), int32(10)})
	if _, ok := got.Column("sym"); ok {
		t.Fatalf("compiled kernel selected mutated column %q", "sym")
	}
}

func TestCompiledQueryKernelClonesNestedLiteralLists(t *testing.T) {
	nested := []any{Symbol("AAPL")}
	empty := []any{}
	var nilList []any
	values := []any{nested}
	plan := QueryPlan{
		Where: In{Expr: ColumnRef{Name: "sym"}, Values: values},
		Select: []SelectItem{{
			Name: "empty",
			Expr: Literal{Value: empty},
		}, {
			Name: "nil_list",
			Expr: Literal{Value: nilList},
		}},
		LimitN: -1,
	}
	compiled := cloneQueryKernelPlan(plan)

	nested[0] = Symbol("MSFT")
	values[0] = []any{Symbol("NVDA")}

	where, ok := compiled.Where.(In)
	if !ok {
		t.Fatalf("compiled where = %T, want In", compiled.Where)
	}
	gotNested, ok := where.Values[0].([]any)
	if !ok {
		t.Fatalf("compiled first literal = %T, want []any", where.Values[0])
	}
	if !reflect.DeepEqual(gotNested, []any{Symbol("AAPL")}) {
		t.Fatalf("compiled nested literal = %v, want [AAPL]", gotNested)
	}
	gotEmpty, ok := compiled.Select[0].Expr.(Literal).Value.([]any)
	if !ok {
		t.Fatalf("compiled empty literal = %T, want []any", compiled.Select[0].Expr.(Literal).Value)
	}
	if gotEmpty == nil || len(gotEmpty) != 0 {
		t.Fatalf("compiled empty literal = %#v, want non-nil empty []any", gotEmpty)
	}
	if gotNil, ok := compiled.Select[1].Expr.(Literal).Value.([]any); !ok || gotNil != nil {
		t.Fatalf("compiled nil list literal = %#v (%T), want nil []any", compiled.Select[1].Expr.(Literal).Value, compiled.Select[1].Expr.(Literal).Value)
	}
}

func TestCompiledQueryKernelHandlesRecursiveLiteralLists(t *testing.T) {
	recursive := make([]any, 1)
	recursive[0] = recursive
	type recursiveSymbols []recursiveSymbols
	typedRecursive := make(recursiveSymbols, 1)
	typedRecursive[0] = typedRecursive
	plan := QueryPlan{
		Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{
			recursive,
			typedRecursive,
		}},
		LimitN: -1,
	}

	fingerprint := QueryKernelPlanFingerprint(plan)
	if fingerprint == "" {
		t.Fatal("QueryKernelPlanFingerprint returned empty fingerprint for recursive literal")
	}
	if got := QueryKernelPlanFingerprint(plan); got != fingerprint {
		t.Fatalf("QueryKernelPlanFingerprint recursive literal = %q, want stable %q", got, fingerprint)
	}

	compiled := cloneQueryKernelPlan(plan)
	where, ok := compiled.Where.(In)
	if !ok {
		t.Fatalf("compiled where = %T, want In", compiled.Where)
	}
	if len(where.Values) != 2 {
		t.Fatalf("compiled where values len = %d, want 2", len(where.Values))
	}
	clonedRecursive, ok := where.Values[0].([]any)
	if !ok {
		t.Fatalf("compiled first recursive literal = %T, want []any", where.Values[0])
	}
	if len(clonedRecursive) != 1 {
		t.Fatalf("compiled recursive literal len = %d, want 1", len(clonedRecursive))
	}
	if &clonedRecursive[0] == &recursive[0] {
		t.Fatal("compiled recursive literal aliases source slice")
	}
	if clonedRecursive[0] == nil {
		t.Fatal("compiled recursive literal lost recursive element")
	}
	clonedSelf, ok := clonedRecursive[0].([]any)
	if !ok {
		t.Fatalf("compiled recursive element = %T, want []any", clonedRecursive[0])
	}
	if len(clonedSelf) != 1 || &clonedSelf[0] != &clonedRecursive[0] {
		t.Fatal("compiled recursive literal does not point back to cloned slice")
	}
	recursive[0] = Symbol("MSFT")
	if clonedSelfAfterMutation, ok := clonedRecursive[0].([]any); !ok || len(clonedSelfAfterMutation) != 1 || &clonedSelfAfterMutation[0] != &clonedRecursive[0] {
		t.Fatal("compiled recursive literal changed after source slice mutation")
	}

	clonedTyped, ok := where.Values[1].(recursiveSymbols)
	if !ok {
		t.Fatalf("compiled typed recursive literal = %T, want recursiveSymbols", where.Values[1])
	}
	if len(clonedTyped) != 1 {
		t.Fatalf("compiled typed recursive literal len = %d, want 1", len(clonedTyped))
	}
	if &clonedTyped[0] == &typedRecursive[0] {
		t.Fatal("compiled typed recursive literal aliases source slice")
	}
	if len(clonedTyped[0]) != 1 || &clonedTyped[0][0] != &clonedTyped[0] {
		t.Fatal("compiled typed recursive literal does not point back to cloned slice")
	}
	typedRecursive[0] = nil
	if len(clonedTyped[0]) != 1 || &clonedTyped[0][0] != &clonedTyped[0] {
		t.Fatal("compiled typed recursive literal changed after source slice mutation")
	}
}

func TestCompiledQueryKernelClonesArrayLiterals(t *testing.T) {
	nested := []any{Symbol("AAPL")}
	symbols := []Symbol{"AAPL"}
	array := [2]any{nested, symbols}
	plan := QueryPlan{
		Select: []SelectItem{{
			Name: "array",
			Expr: Literal{Value: array},
		}},
		LimitN: -1,
	}

	compiled := cloneQueryKernelPlan(plan)
	nested[0] = Symbol("MSFT")
	symbols[0] = Symbol("MSFT")

	got, ok := compiled.Select[0].Expr.(Literal).Value.([2]any)
	if !ok {
		t.Fatalf("compiled array literal = %T, want [2]any", compiled.Select[0].Expr.(Literal).Value)
	}
	gotNested, ok := got[0].([]any)
	if !ok {
		t.Fatalf("compiled array first element = %T, want []any", got[0])
	}
	if !reflect.DeepEqual(gotNested, []any{Symbol("AAPL")}) {
		t.Fatalf("compiled array nested list = %v, want [AAPL]", gotNested)
	}
	gotSymbols, ok := got[1].([]Symbol)
	if !ok {
		t.Fatalf("compiled array second element = %T, want []Symbol", got[1])
	}
	if !reflect.DeepEqual(gotSymbols, []Symbol{"AAPL"}) {
		t.Fatalf("compiled array typed slice = %v, want [AAPL]", gotSymbols)
	}
}

func TestCompiledQueryKernelClonesStructLiterals(t *testing.T) {
	type literalStruct struct {
		Nested  []any
		Symbols []Symbol
	}
	nested := []any{Symbol("AAPL")}
	symbols := []Symbol{"AAPL"}
	plan := QueryPlan{
		Select: []SelectItem{{
			Name: "struct",
			Expr: Literal{Value: literalStruct{Nested: nested, Symbols: symbols}},
		}},
		LimitN: -1,
	}

	compiled := cloneQueryKernelPlan(plan)
	nested[0] = Symbol("MSFT")
	symbols[0] = Symbol("MSFT")

	got, ok := compiled.Select[0].Expr.(Literal).Value.(literalStruct)
	if !ok {
		t.Fatalf("compiled struct literal = %T, want literalStruct", compiled.Select[0].Expr.(Literal).Value)
	}
	if !reflect.DeepEqual(got.Nested, []any{Symbol("AAPL")}) {
		t.Fatalf("compiled struct nested list = %v, want [AAPL]", got.Nested)
	}
	if !reflect.DeepEqual(got.Symbols, []Symbol{"AAPL"}) {
		t.Fatalf("compiled struct typed slice = %v, want [AAPL]", got.Symbols)
	}
}

func TestCompiledQueryKernelClonesPointerLiterals(t *testing.T) {
	type literalStruct struct {
		Nested []any
	}
	type recursiveStruct struct {
		Next *recursiveStruct
	}
	nested := []any{Symbol("AAPL")}
	recursive := &recursiveStruct{}
	recursive.Next = recursive
	plan := QueryPlan{
		Select: []SelectItem{{
			Name: "ptr",
			Expr: Literal{Value: &literalStruct{Nested: nested}},
		}, {
			Name: "recursive",
			Expr: Literal{Value: recursive},
		}},
		LimitN: -1,
	}

	compiled := cloneQueryKernelPlan(plan)
	nested[0] = Symbol("MSFT")
	recursive.Next = nil

	got, ok := compiled.Select[0].Expr.(Literal).Value.(*literalStruct)
	if !ok {
		t.Fatalf("compiled pointer literal = %T, want *literalStruct", compiled.Select[0].Expr.(Literal).Value)
	}
	if got == plan.Select[0].Expr.(Literal).Value.(*literalStruct) {
		t.Fatal("compiled pointer literal aliases source pointer")
	}
	if !reflect.DeepEqual(got.Nested, []any{Symbol("AAPL")}) {
		t.Fatalf("compiled pointer nested list = %v, want [AAPL]", got.Nested)
	}

	gotRecursive, ok := compiled.Select[1].Expr.(Literal).Value.(*recursiveStruct)
	if !ok {
		t.Fatalf("compiled recursive pointer literal = %T, want *recursiveStruct", compiled.Select[1].Expr.(Literal).Value)
	}
	if gotRecursive == recursive {
		t.Fatal("compiled recursive pointer literal aliases source pointer")
	}
	if gotRecursive.Next != gotRecursive {
		t.Fatal("compiled recursive pointer literal does not point back to cloned pointer")
	}
}

func TestCompiledQueryKernelClonesMapLiterals(t *testing.T) {
	nested := []any{Symbol("AAPL")}
	recursive := map[string]any{}
	recursive["self"] = recursive
	plan := QueryPlan{
		Select: []SelectItem{{
			Name: "map",
			Expr: Literal{Value: map[string]any{"symbols": nested}},
		}, {
			Name: "recursive",
			Expr: Literal{Value: recursive},
		}},
		LimitN: -1,
	}

	compiled := cloneQueryKernelPlan(plan)
	nested[0] = Symbol("MSFT")
	recursive["self"] = Symbol("MSFT")

	got, ok := compiled.Select[0].Expr.(Literal).Value.(map[string]any)
	if !ok {
		t.Fatalf("compiled map literal = %T, want map[string]any", compiled.Select[0].Expr.(Literal).Value)
	}
	gotNested, ok := got["symbols"].([]any)
	if !ok {
		t.Fatalf("compiled map nested value = %T, want []any", got["symbols"])
	}
	if !reflect.DeepEqual(gotNested, []any{Symbol("AAPL")}) {
		t.Fatalf("compiled map nested list = %v, want [AAPL]", gotNested)
	}

	gotRecursive, ok := compiled.Select[1].Expr.(Literal).Value.(map[string]any)
	if !ok {
		t.Fatalf("compiled recursive map literal = %T, want map[string]any", compiled.Select[1].Expr.(Literal).Value)
	}
	gotSelf, ok := gotRecursive["self"].(map[string]any)
	if !ok {
		t.Fatalf("compiled recursive map self = %T, want map[string]any", gotRecursive["self"])
	}
	if reflect.ValueOf(gotSelf["self"]).Pointer() != reflect.ValueOf(gotRecursive).Pointer() {
		t.Fatal("compiled recursive map does not point back to cloned map")
	}
}

func TestCompiledQueryKernelClonesTypedSliceLiterals(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
	)
	symbols := []Symbol{"AAPL", "MSFT"}
	nested := []any{[]Symbol{"AAPL", "MSFT"}}
	plan := QueryPlan{
		Select: []SelectItem{{
			Name: "symbols",
			Expr: Literal{Value: symbols},
		}, {
			Name: "nested",
			Expr: Literal{Value: nested},
		}},
		LimitN: -1,
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil || !ok || kernel == nil {
		t.Fatalf("CompileQueryKernel = kernel %v, ok %v, err %v; want compiled kernel", kernel, ok, err)
	}

	symbols[0] = "NVDA"
	nested[0].([]Symbol)[0] = "NVDA"

	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("compiled kernel Exec returned error after typed slice literal mutation: %v", err)
	}
	symbolCol, ok := got.Column("symbols")
	if !ok {
		t.Fatal("compiled kernel result missing symbols column")
	}
	nestedCol, ok := got.Column("nested")
	if !ok {
		t.Fatal("compiled kernel result missing nested column")
	}
	for row := 0; row < got.Len(); row++ {
		value, ok := symbolCol.At(row)
		if !ok {
			t.Fatalf("symbols row %d missing", row)
		}
		if !reflect.DeepEqual(value, []Symbol{"AAPL", "MSFT"}) {
			t.Fatalf("symbols row %d = %#v, want original typed slice", row, value)
		}
		nestedValue, ok := nestedCol.At(row)
		if !ok {
			t.Fatalf("nested row %d missing", row)
		}
		if !reflect.DeepEqual(nestedValue, []any{[]Symbol{"AAPL", "MSFT"}}) {
			t.Fatalf("nested row %d = %#v, want original nested typed slice", row, nestedValue)
		}
	}
}

func TestCompiledQueryKernelClonesSupportedExprMutableLiterals(t *testing.T) {
	whereLow := []any{int32(10)}
	whereHigh := []any{int32(20)}
	bucketInterval := []any{TimespanFromNanos(1_000)}
	listThen := []any{Symbol("then")}
	listElse := []any{Symbol("else")}
	vectorArg := []any{int32(2)}
	conditionalCondValues := []any{[]any{Symbol("AAPL")}}
	conditionalThen := []any{Symbol("buy")}
	conditionalElse := []any{Symbol("sell")}
	aggregateWeight := []any{float64(0.5)}
	plan := QueryPlan{
		Where: Within{
			Expr:       ColumnRef{Name: "qty"},
			Low:        whereLow,
			High:       whereHigh,
			HighClosed: true,
		},
		ByExprs: []SelectItem{{
			Name: "bucket",
			Expr: BucketFloorExpr{
				Expr:     ColumnRef{Name: "ts"},
				Interval: bucketInterval,
			},
		}},
		Select: []SelectItem{{
			Name: "list_cond",
			Expr: ListAggregateExpr{
				Func: "avg",
				Expr: Conditional{
					Cond: Literal{Value: true},
					Then: Literal{Value: listThen},
					Else: Literal{Value: listElse},
				},
			},
		}, {
			Name: "vector_arg",
			Expr: VectorTransformExpr{
				Func: "deltas",
				Expr: ColumnRef{Name: "qty"},
				Arg:  Literal{Value: vectorArg},
			},
		}, {
			Name: "conditional",
			Expr: Conditional{
				Cond: In{Expr: ColumnRef{Name: "sym"}, Values: conditionalCondValues},
				Then: Literal{Value: conditionalThen},
				Else: Literal{Value: conditionalElse},
			},
		}},
		Aggregates: []Aggregate{{
			Name:   "weighted",
			Func:   "wavg",
			Expr:   ColumnRef{Name: "qty"},
			Weight: Literal{Value: aggregateWeight},
		}},
		LimitN: -1,
	}
	compiled := cloneQueryKernelPlan(plan)

	whereLow[0] = int32(30)
	whereHigh[0] = int32(40)
	bucketInterval[0] = TimespanFromNanos(2_000)
	listThen[0] = Symbol("mutated_then")
	listElse[0] = Symbol("mutated_else")
	vectorArg[0] = int32(3)
	conditionalCondValues[0].([]any)[0] = Symbol("MSFT")
	conditionalThen[0] = Symbol("mutated_buy")
	conditionalElse[0] = Symbol("mutated_sell")
	aggregateWeight[0] = float64(0.75)

	where, ok := compiled.Where.(Within)
	if !ok {
		t.Fatalf("compiled where = %T, want Within", compiled.Where)
	}
	if !reflect.DeepEqual(where.Low, []any{int32(10)}) || !reflect.DeepEqual(where.High, []any{int32(20)}) {
		t.Fatalf("compiled within bounds = %v/%v, want original literals", where.Low, where.High)
	}
	bucket, ok := compiled.ByExprs[0].Expr.(BucketFloorExpr)
	if !ok {
		t.Fatalf("compiled by expr = %T, want BucketFloorExpr", compiled.ByExprs[0].Expr)
	}
	if !reflect.DeepEqual(bucket.Interval, []any{TimespanFromNanos(1_000)}) {
		t.Fatalf("compiled bucket interval = %v, want original literal", bucket.Interval)
	}
	listAgg, ok := compiled.Select[0].Expr.(ListAggregateExpr)
	if !ok {
		t.Fatalf("compiled select[0] = %T, want ListAggregateExpr", compiled.Select[0].Expr)
	}
	listCond, ok := listAgg.Expr.(Conditional)
	if !ok {
		t.Fatalf("compiled list aggregate expr = %T, want Conditional", listAgg.Expr)
	}
	if got := listCond.Then.(Literal).Value; !reflect.DeepEqual(got, []any{Symbol("then")}) {
		t.Fatalf("compiled list aggregate then literal = %v, want original literal", got)
	}
	if got := listCond.Else.(Literal).Value; !reflect.DeepEqual(got, []any{Symbol("else")}) {
		t.Fatalf("compiled list aggregate else literal = %v, want original literal", got)
	}
	vector, ok := compiled.Select[1].Expr.(VectorTransformExpr)
	if !ok {
		t.Fatalf("compiled select[1] = %T, want VectorTransformExpr", compiled.Select[1].Expr)
	}
	if got := vector.Arg.(Literal).Value; !reflect.DeepEqual(got, []any{int32(2)}) {
		t.Fatalf("compiled vector arg literal = %v, want original literal", got)
	}
	conditional, ok := compiled.Select[2].Expr.(Conditional)
	if !ok {
		t.Fatalf("compiled select[2] = %T, want Conditional", compiled.Select[2].Expr)
	}
	condIn := conditional.Cond.(In)
	if !reflect.DeepEqual(condIn.Values, []any{[]any{Symbol("AAPL")}}) {
		t.Fatalf("compiled conditional in values = %v, want original literals", condIn.Values)
	}
	if got := conditional.Then.(Literal).Value; !reflect.DeepEqual(got, []any{Symbol("buy")}) {
		t.Fatalf("compiled conditional then literal = %v, want original literal", got)
	}
	if got := conditional.Else.(Literal).Value; !reflect.DeepEqual(got, []any{Symbol("sell")}) {
		t.Fatalf("compiled conditional else literal = %v, want original literal", got)
	}
	if got := compiled.Aggregates[0].Weight.(Literal).Value; !reflect.DeepEqual(got, []any{float64(0.5)}) {
		t.Fatalf("compiled aggregate weight literal = %v, want original literal", got)
	}
}

func TestQueryKernelPlanFingerprintCoversSemanticFields(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20})},
	)
	otherSource := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"NVDA"})},
		Column{Name: "qty", Data: NewI32([]int32{30})},
	)
	base := QueryPlan{
		Source:   frame,
		Distinct: true,
		Where:    Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(10)}},
		By:       []Symbol{"sym"},
		ByExprs: []SelectItem{{
			Name: "bucket",
			Expr: BucketFloorExpr{Expr: ColumnRef{Name: "qty"}, Interval: int32(10)},
		}},
		Select: []SelectItem{{
			Name: "sym_out",
			Expr: ColumnRef{Name: "sym"},
		}},
		Aggregates: []Aggregate{{
			Name:   "wavg_qty",
			Func:   "wavg",
			Expr:   ColumnRef{Name: "qty"},
			Weight: ColumnRef{Name: "qty"},
		}},
		OrderBy:         []OrderSpec{{Column: "sym_out", Desc: true}},
		PreProjectOrder: true,
		LimitN:          2,
	}
	baseFingerprint := QueryKernelPlanFingerprint(base)
	baseKey := QueryKernelCacheKey("source", frame, base)

	sourceChanged := base
	sourceChanged.Source = otherSource
	if got := QueryKernelPlanFingerprint(sourceChanged); got != baseFingerprint {
		t.Fatalf("source-only fingerprint = %q, want %q", got, baseFingerprint)
	}
	if got := QueryKernelCacheKey("source", frame, sourceChanged); got != baseKey {
		t.Fatalf("source-only kernel key = %q, want %q", got, baseKey)
	}

	cases := []struct {
		name string
		edit func(QueryPlan) QueryPlan
	}{
		{name: "distinct", edit: func(p QueryPlan) QueryPlan { p.Distinct = false; return p }},
		{name: "where", edit: func(p QueryPlan) QueryPlan {
			p.Where = Binary{Op: OpGT, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(10)}}
			return p
		}},
		{name: "by", edit: func(p QueryPlan) QueryPlan { p.By = []Symbol{"qty"}; return p }},
		{name: "by expr name", edit: func(p QueryPlan) QueryPlan {
			p.ByExprs[0].Name = "bucket2"
			return p
		}},
		{name: "by expr", edit: func(p QueryPlan) QueryPlan {
			p.ByExprs[0].Expr = BucketFloorExpr{Expr: ColumnRef{Name: "qty"}, Interval: int32(5)}
			return p
		}},
		{name: "select name", edit: func(p QueryPlan) QueryPlan {
			p.Select[0].Name = "sym_alias"
			return p
		}},
		{name: "select expr", edit: func(p QueryPlan) QueryPlan {
			p.Select[0].Expr = Literal{Value: Symbol("fixed")}
			return p
		}},
		{name: "aggregate name", edit: func(p QueryPlan) QueryPlan {
			p.Aggregates[0].Name = "avg_qty"
			return p
		}},
		{name: "aggregate func", edit: func(p QueryPlan) QueryPlan {
			p.Aggregates[0].Func = "sum"
			return p
		}},
		{name: "aggregate expr", edit: func(p QueryPlan) QueryPlan {
			p.Aggregates[0].Expr = Literal{Value: int32(1)}
			return p
		}},
		{name: "aggregate weight", edit: func(p QueryPlan) QueryPlan {
			p.Aggregates[0].Weight = Literal{Value: int32(1)}
			return p
		}},
		{name: "order column", edit: func(p QueryPlan) QueryPlan {
			p.OrderBy[0].Column = "qty"
			return p
		}},
		{name: "order direction", edit: func(p QueryPlan) QueryPlan {
			p.OrderBy[0].Desc = false
			return p
		}},
		{name: "pre project order", edit: func(p QueryPlan) QueryPlan {
			p.PreProjectOrder = false
			return p
		}},
		{name: "limit", edit: func(p QueryPlan) QueryPlan { p.LimitN = 1; return p }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			changed := tt.edit(base)
			if got := QueryKernelPlanFingerprint(changed); got == baseFingerprint {
				t.Fatalf("changed %s fingerprint = %q, want it to differ", tt.name, got)
			}
			if got := QueryKernelCacheKey("source", frame, changed); got == baseKey {
				t.Fatalf("changed %s kernel key = %q, want it to differ", tt.name, got)
			}
		})
	}
}

func TestQueryKernelPlanFingerprintAvoidsStructuralCollisions(t *testing.T) {
	byWithEmbeddedSeparator := QueryPlan{By: []Symbol{"a,b", "c"}, LimitN: -1}
	bySplitSeparator := QueryPlan{By: []Symbol{"a", "b,c"}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(byWithEmbeddedSeparator); got == QueryKernelPlanFingerprint(bySplitSeparator) {
		t.Fatalf("by symbol boundary collided: %q", got)
	}

	listWithEmbeddedSeparator := QueryPlan{
		Where:  In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("a,b"), Symbol("c")}},
		LimitN: -1,
	}
	listSplitSeparator := QueryPlan{
		Where:  In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("a"), Symbol("b,c")}},
		LimitN: -1,
	}
	if got := QueryKernelPlanFingerprint(listWithEmbeddedSeparator); got == QueryKernelPlanFingerprint(listSplitSeparator) {
		t.Fatalf("literal list boundary collided: %q", got)
	}

	symbolLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("AAPL")}}, LimitN: -1}
	stringLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: "AAPL"}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(symbolLiteral); got == QueryKernelPlanFingerprint(stringLiteral) {
		t.Fatalf("symbol/string literal kind collided: %q", got)
	}

	timestampLiteral := QueryPlan{Where: Binary{Op: OpGE, Left: ColumnRef{Name: "ts"}, Right: Literal{Value: TimestampFromUnixNanos(10)}}, LimitN: -1}
	i64Literal := QueryPlan{Where: Binary{Op: OpGE, Left: ColumnRef{Name: "ts"}, Right: Literal{Value: int64(10)}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(timestampLiteral); got == QueryKernelPlanFingerprint(i64Literal) {
		t.Fatalf("timestamp/i64 literal kind collided: %q", got)
	}
	dateLiteral := QueryPlan{Where: Binary{Op: OpGE, Left: ColumnRef{Name: "ts"}, Right: Literal{Value: DateFromDays(10)}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(timestampLiteral); got == QueryKernelPlanFingerprint(dateLiteral) {
		t.Fatalf("timestamp/date literal kind collided: %q", got)
	}

	unsupportedExprA := QueryPlan{Where: queryKernelFingerprintFallbackExpr{Name: "a"}, LimitN: -1}
	unsupportedExprB := QueryPlan{Where: queryKernelFingerprintFallbackExpr{Name: "b"}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(unsupportedExprA); got == QueryKernelPlanFingerprint(unsupportedExprB) {
		t.Fatalf("unsupported expression value collided: %q", got)
	}

	typedNull := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: NullForKind(KindI32)}}, LimitN: -1}
	untypedNull := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: NullValue}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(typedNull); got == QueryKernelPlanFingerprint(untypedNull) {
		t.Fatalf("typed/untyped null literal collided: %q", got)
	}
	timestampNull := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "ts"}, Right: Literal{Value: NullForKind(KindTimestamp)}}, LimitN: -1}
	dateNull := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "ts"}, Right: Literal{Value: NullForKind(KindDate)}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(timestampNull); got == QueryKernelPlanFingerprint(dateNull) {
		t.Fatalf("typed temporal null kind collided: %q", got)
	}

	nanLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "px"}, Right: Literal{Value: math.NaN()}}, LimitN: -1}
	nanPayloadLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "px"}, Right: Literal{Value: math.Float64frombits(0x7ff8000000000001)}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(nanLiteral); got != QueryKernelPlanFingerprint(nanPayloadLiteral) {
		t.Fatalf("NaN literal fingerprint = %q, want canonical NaN fingerprint", got)
	}
	f32NaNLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "px"}, Right: Literal{Value: math.Float32frombits(0x7fc00000)}}, LimitN: -1}
	f32NaNPayloadLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "px"}, Right: Literal{Value: math.Float32frombits(0x7fc00001)}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(f32NaNLiteral); got != QueryKernelPlanFingerprint(f32NaNPayloadLiteral) {
		t.Fatalf("float32 NaN literal fingerprint = %q, want canonical NaN fingerprint", got)
	}
	f64NaNLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "px"}, Right: Literal{Value: math.Float64frombits(0x7ff8000000000000)}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(f32NaNLiteral); got == QueryKernelPlanFingerprint(f64NaNLiteral) {
		t.Fatalf("float32/float64 NaN literal kind collided: %q", got)
	}
	posInfLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "px"}, Right: Literal{Value: math.Inf(1)}}, LimitN: -1}
	negInfLiteral := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "px"}, Right: Literal{Value: math.Inf(-1)}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(posInfLiteral); got == QueryKernelPlanFingerprint(negInfLiteral) {
		t.Fatalf("+Inf/-Inf literal collided: %q", got)
	}

	nestedListA := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]any{Symbol("a"), Symbol("b,c")}, Symbol("d")}}, LimitN: -1}
	nestedListB := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]any{Symbol("a,b"), Symbol("c")}, Symbol("d")}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(nestedListA); got == QueryKernelPlanFingerprint(nestedListB) {
		t.Fatalf("nested literal list boundary collided: %q", got)
	}

	typedSymbolSlice := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]Symbol{"AAPL", "MSFT"}}}, LimitN: -1}
	stringSlice := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]string{"AAPL", "MSFT"}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(typedSymbolSlice); got == QueryKernelPlanFingerprint(stringSlice) {
		t.Fatalf("typed symbol/string slice literal collided: %q", got)
	}
	typedSliceWithEmbeddedSeparator := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]Symbol{"a,b", "c"}}}, LimitN: -1}
	typedSliceSplitSeparator := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]Symbol{"a", "b,c"}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(typedSliceWithEmbeddedSeparator); got == QueryKernelPlanFingerprint(typedSliceSplitSeparator) {
		t.Fatalf("typed slice literal boundary collided: %q", got)
	}
	nilTypedSlice := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{([]Symbol)(nil)}}, LimitN: -1}
	emptyTypedSlice := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]Symbol{}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(nilTypedSlice); got == QueryKernelPlanFingerprint(emptyTypedSlice) {
		t.Fatalf("nil/empty typed slice literal collided: %q", got)
	}
	nilListLiteral := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{([]any)(nil)}}, LimitN: -1}
	emptyListLiteral := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[]any{}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(nilListLiteral); got == QueryKernelPlanFingerprint(emptyListLiteral) {
		t.Fatalf("nil/empty list literal collided: %q", got)
	}
	typedNaNSlice := QueryPlan{Where: In{Expr: ColumnRef{Name: "px"}, Values: []any{[]float64{math.NaN()}}}, LimitN: -1}
	typedNaNPayloadSlice := QueryPlan{Where: In{Expr: ColumnRef{Name: "px"}, Values: []any{[]float64{math.Float64frombits(0x7ff8000000000001)}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(typedNaNSlice); got != QueryKernelPlanFingerprint(typedNaNPayloadSlice) {
		t.Fatalf("typed NaN slice literal fingerprint = %q, want canonical NaN fingerprint", got)
	}
	typedSymbolArray := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[2]Symbol{"AAPL", "MSFT"}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(typedSymbolArray); got == QueryKernelPlanFingerprint(typedSymbolSlice) {
		t.Fatalf("typed array/slice literal collided: %q", got)
	}
	typedArrayWithEmbeddedSeparator := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[2]Symbol{"a,b", "c"}}}, LimitN: -1}
	typedArraySplitSeparator := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[2]Symbol{"a", "b,c"}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(typedArrayWithEmbeddedSeparator); got == QueryKernelPlanFingerprint(typedArraySplitSeparator) {
		t.Fatalf("typed array literal boundary collided: %q", got)
	}
	arrayWithNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[1]any{[]any{Symbol("a,b"), Symbol("c")}}}}, LimitN: -1}
	arrayWithSplitNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{[1]any{[]any{Symbol("a"), Symbol("b,c")}}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(arrayWithNestedList); got == QueryKernelPlanFingerprint(arrayWithSplitNestedList) {
		t.Fatalf("array nested list boundary collided: %q", got)
	}

	structLiteralA := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "x"}, Right: Literal{Value: struct{ Name string }{Name: "a"}}}, LimitN: -1}
	structLiteralB := QueryPlan{Where: Binary{Op: OpEQ, Left: ColumnRef{Name: "x"}, Right: Literal{Value: struct{ Name string }{Name: "b"}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(structLiteralA); got == QueryKernelPlanFingerprint(structLiteralB) {
		t.Fatalf("fallback literal value collided: %q", got)
	}
	structWithNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{struct{ Values []any }{Values: []any{Symbol("a,b"), Symbol("c")}}}}, LimitN: -1}
	structWithSplitNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{struct{ Values []any }{Values: []any{Symbol("a"), Symbol("b,c")}}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(structWithNestedList); got == QueryKernelPlanFingerprint(structWithSplitNestedList) {
		t.Fatalf("struct nested list boundary collided: %q", got)
	}
	structHiddenA := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{queryKernelFingerprintHiddenStruct{hidden: 1}}}, LimitN: -1}
	structHiddenB := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{queryKernelFingerprintHiddenStruct{hidden: 2}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(structHiddenA); got == QueryKernelPlanFingerprint(structHiddenB) {
		t.Fatalf("struct hidden field value collided: %q", got)
	}
	type pointerStruct struct {
		Values []any
	}
	pointerWithNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{&pointerStruct{Values: []any{Symbol("a,b"), Symbol("c")}}}}, LimitN: -1}
	equivalentPointerWithNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{&pointerStruct{Values: []any{Symbol("a,b"), Symbol("c")}}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(pointerWithNestedList); got != QueryKernelPlanFingerprint(equivalentPointerWithNestedList) {
		t.Fatalf("equivalent pointer literal fingerprint = %q, want stable structural fingerprint", got)
	}
	pointerWithSplitNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{&pointerStruct{Values: []any{Symbol("a"), Symbol("b,c")}}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(pointerWithNestedList); got == QueryKernelPlanFingerprint(pointerWithSplitNestedList) {
		t.Fatalf("pointer nested list boundary collided: %q", got)
	}
	type recursivePointerStruct struct {
		Next *recursivePointerStruct
	}
	recursivePointer := &recursivePointerStruct{}
	recursivePointer.Next = recursivePointer
	recursivePointerLiteral := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{recursivePointer}}, LimitN: -1}
	pointerFingerprint := QueryKernelPlanFingerprint(recursivePointerLiteral)
	if pointerFingerprint == "" {
		t.Fatal("recursive pointer literal fingerprint is empty")
	}
	if got := QueryKernelPlanFingerprint(recursivePointerLiteral); got != pointerFingerprint {
		t.Fatalf("recursive pointer literal fingerprint = %q, want stable %q", got, pointerFingerprint)
	}
	mapLiteralA := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{map[Symbol]any{Symbol("b"): []any{Symbol("MSFT")}, Symbol("a"): []any{Symbol("AAPL")}}}}, LimitN: -1}
	mapLiteralB := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{map[Symbol]any{Symbol("a"): []any{Symbol("AAPL")}, Symbol("b"): []any{Symbol("MSFT")}}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(mapLiteralA); got != QueryKernelPlanFingerprint(mapLiteralB) {
		t.Fatalf("map literal fingerprint = %q, want stable key order", got)
	}
	mapWithNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{map[string]any{"x": []any{Symbol("a,b"), Symbol("c")}}}}, LimitN: -1}
	mapWithSplitNestedList := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{map[string]any{"x": []any{Symbol("a"), Symbol("b,c")}}}}, LimitN: -1}
	if got := QueryKernelPlanFingerprint(mapWithNestedList); got == QueryKernelPlanFingerprint(mapWithSplitNestedList) {
		t.Fatalf("map nested list boundary collided: %q", got)
	}
	recursiveMap := map[string]any{}
	recursiveMap["self"] = recursiveMap
	recursiveMapLiteral := QueryPlan{Where: In{Expr: ColumnRef{Name: "sym"}, Values: []any{recursiveMap}}, LimitN: -1}
	fingerprint := QueryKernelPlanFingerprint(recursiveMapLiteral)
	if fingerprint == "" {
		t.Fatal("recursive map literal fingerprint is empty")
	}
	if got := QueryKernelPlanFingerprint(recursiveMapLiteral); got != fingerprint {
		t.Fatalf("recursive map literal fingerprint = %q, want stable %q", got, fingerprint)
	}
}

func TestQueryKernelSupportReasonClassifiesHotExpressionPaths(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA"}), ArrayAttributeGrouped)},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30, 40})},
		Column{Name: "px", Data: NewF64([]float64{100, 101, 102, 103})},
		Column{Name: "ts", Data: NewTimestamp([]Timestamp{1_000, 2_000, 3_000, 4_000})},
		Column{Name: "book", Data: NewColumn("book", []any{[]any{100.0, 101.0}, []any{101.0}, []any{102.0}, []any{103.0}}).Data},
	)

	cases := []struct {
		name      string
		plan      QueryPlan
		want      []string
		wantShape string
	}{
		{
			name: "typed column literal filter and binary projection",
			plan: QueryPlan{
				Where: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(20)}},
				Select: []SelectItem{{
					Name: "notional",
					Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "qty"}, Right: ColumnRef{Name: "px"}},
				}},
				LimitN: -1,
			},
			want:      []string{"filtered projection path", "typed column-literal filter", "typed binary projection"},
			wantShape: "filtered_projection|where=typed_column_literal|projection=typed_binary",
		},
		{
			name: "boolean projection",
			plan: QueryPlan{
				Where: Within{Expr: ColumnRef{Name: "qty"}, Low: int32(10), High: int32(40), HighClosed: true},
				Select: []SelectItem{{
					Name: "large",
					Expr: In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("AAPL"), Symbol("NVDA")}},
				}},
				LimitN: -1,
			},
			want:      []string{"filtered projection path", "typed within filter", "boolean projection"},
			wantShape: "filtered_projection|where=typed_within|projection=boolean",
		},
		{
			name: "bucketed grouped aggregate",
			plan: QueryPlan{
				ByExprs: []SelectItem{{
					Name: "bucket",
					Expr: BucketFloorExpr{Expr: ColumnRef{Name: "ts"}, Interval: TimespanFromNanos(2_000)},
				}},
				Aggregates: []Aggregate{{Name: "n", Func: "count"}},
				LimitN:     -1,
			},
			want:      []string{"grouped aggregate path", "bucketed by expression", "typed column aggregate"},
			wantShape: "grouped_aggregate|by=bucketed|aggregate=typed_column",
		},
		{
			name: "grouped projection",
			plan: QueryPlan{
				By: []Symbol{"sym"},
				Select: []SelectItem{{
					Name: "side",
					Expr: Conditional{
						Cond: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(20)}},
						Then: Literal{Value: Symbol("large")},
						Else: Literal{Value: Symbol("small")},
					},
				}},
				Distinct: true,
				OrderBy:  []OrderSpec{{Column: "side"}},
				LimitN:   2,
			},
			want:      []string{"grouped projection path", "conditional projection", "distinct rows", "post-project order", "limit"},
			wantShape: "grouped_projection|by=columns|projection=conditional|order=post_project:1|limit=bounded|distinct=true",
		},
		{
			name: "list aggregate projection",
			plan: QueryPlan{
				Select: []SelectItem{{
					Name: "avg_book",
					Expr: ListAggregateExpr{Func: "avg", Expr: ColumnRef{Name: "book"}},
				}},
				LimitN: -1,
			},
			want:      []string{"projection path", "list aggregate projection"},
			wantShape: "projection|projection=list_aggregate",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := QueryKernelSupportReason(tt.plan)
			if !ok {
				t.Fatalf("QueryKernelSupportReason ok = false, reason %q; want supported", reason)
			}
			for _, want := range tt.want {
				if !strings.Contains(reason, want) {
					t.Fatalf("QueryKernelSupportReason reason = %q, want it to contain %q", reason, want)
				}
			}
			kernel, ok, err := CompileQueryKernel(frame, tt.plan)
			if err != nil || !ok || kernel == nil {
				t.Fatalf("CompileQueryKernel = kernel %v, ok %v, err %v; want compiled kernel", kernel, ok, err)
			}
			if got := QueryKernelPlanShape(tt.plan); got != tt.wantShape {
				t.Fatalf("QueryKernelPlanShape = %q, want %q", got, tt.wantShape)
			}
			if got := kernel.Shape(); got != tt.wantShape {
				t.Fatalf("compiled kernel shape = %q, want %q", got, tt.wantShape)
			}
			for _, want := range tt.want {
				if !strings.Contains(kernel.Reason(), want) {
					t.Fatalf("compiled kernel reason = %q, want it to contain %q", kernel.Reason(), want)
				}
			}
			compileOK, compileReason, compileErr := QueryKernelCompileReason(frame, tt.plan)
			if compileErr != nil || !compileOK {
				t.Fatalf("QueryKernelCompileReason = ok %v reason %q err %v; want supported", compileOK, compileReason, compileErr)
			}
			for _, want := range tt.want {
				if !strings.Contains(compileReason, want) {
					t.Fatalf("QueryKernelCompileReason reason = %q, want it to contain %q", compileReason, want)
				}
			}
		})
	}

	base := QueryPlan{
		Where: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(20)}},
		Select: []SelectItem{{
			Name: "notional",
			Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "qty"}, Right: ColumnRef{Name: "px"}},
		}},
		LimitN: -1,
	}
	changedLiteral := base
	changedLiteral.Where = Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(200)}}
	if got, want := QueryKernelPlanShape(changedLiteral), QueryKernelPlanShape(base); got != want {
		t.Fatalf("QueryKernelPlanShape changed with literal value: got %q, want %q", got, want)
	}
}

func TestQueryKernelPlanShapeAggregatesFingerprintSplitPlans(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30, 40})},
		Column{Name: "px", Data: NewF64([]float64{100, 80, 210, 190})},
	)
	base := QueryPlan{
		Where: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(20)}},
		Select: []SelectItem{{
			Name: "notional",
			Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "qty"}, Right: ColumnRef{Name: "px"}},
		}},
		LimitN: -1,
	}
	changedLiteral := base
	changedLiteral.Where = Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(30)}}

	if got, want := QueryKernelPlanShape(changedLiteral), QueryKernelPlanShape(base); got != want {
		t.Fatalf("QueryKernelPlanShape changed with literal value: got %q, want %q", got, want)
	}
	if got := QueryKernelPlanFingerprint(changedLiteral); got == QueryKernelPlanFingerprint(base) {
		t.Fatalf("QueryKernelPlanFingerprint did not split changed literal: %q", got)
	}
	if got := QueryKernelCacheKey("source", frame, changedLiteral); got == QueryKernelCacheKey("source", frame, base) {
		t.Fatalf("QueryKernelCacheKey did not split changed literal: %q", got)
	}

	for name, plan := range map[string]QueryPlan{"base": base, "changed_literal": changedLiteral} {
		kernel, ok, err := CompileQueryKernel(frame, plan)
		if err != nil || !ok || kernel == nil {
			t.Fatalf("%s CompileQueryKernel = kernel %v, ok %v, err %v; want compiled kernel", name, kernel, ok, err)
		}
		if got, want := kernel.Shape(), QueryKernelPlanShape(base); got != want {
			t.Fatalf("%s kernel shape = %q, want %q", name, got, want)
		}
	}
}

func TestQueryKernelPlanShapeClassifiesCompositePaths(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30, 40})},
	)
	plan := QueryPlan{
		By: []Symbol{"sym"},
		Select: []SelectItem{{
			Name: "size_bucket",
			Expr: Conditional{
				Cond: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(20)}},
				Then: Literal{Value: Symbol("large")},
				Else: Literal{Value: Symbol("small")},
			},
		}},
		Distinct: true,
		OrderBy:  []OrderSpec{{Column: "size_bucket"}},
		LimitN:   2,
	}
	want := "grouped_projection|by=columns|projection=conditional|order=post_project:1|limit=bounded|distinct=true"
	if got := QueryKernelPlanShape(plan); got != want {
		t.Fatalf("QueryKernelPlanShape = %q, want %q", got, want)
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil || !ok || kernel == nil {
		t.Fatalf("CompileQueryKernel = kernel %v, ok %v, err %v; want compiled kernel", kernel, ok, err)
	}
	if got := kernel.Shape(); got != want {
		t.Fatalf("compiled kernel shape = %q, want %q", got, want)
	}
	var nilKernel *QueryKernel
	if got := nilKernel.Shape(); got != "" {
		t.Fatalf("nil kernel shape = %q, want empty", got)
	}
}

func TestNumericAtTypedNullableAndBoundary(t *testing.T) {
	n, ok, err := typedKernels.NumericAt(NewI64([]int64{-2, 4}), 0)
	if err != nil {
		t.Fatalf("typed NumericAt returned error: %v", err)
	}
	if !ok || n != -2 {
		t.Fatalf("typed NumericAt = %v, %v; want -2, true", n, ok)
	}

	if _, _, err := typedKernels.NumericAt(NewI64([]int64{1}), -1); err == nil {
		t.Fatal("typed NumericAt accepted negative row")
	}
	if _, _, err := typedKernels.NumericAt(NewI64([]int64{1}), 1); err == nil {
		t.Fatal("typed NumericAt accepted row past end")
	}

	nullable := NewColumn("x", []any{int64(1), nil}).Data
	n, ok, err = typedKernels.NumericAt(nullable, 0)
	if err != nil {
		t.Fatalf("nullable NumericAt returned error: %v", err)
	}
	if !ok || n != 1 {
		t.Fatalf("nullable NumericAt row 0 = %v, %v; want 1, true", n, ok)
	}
	n, ok, err = typedKernels.NumericAt(nullable, 1)
	if err != nil {
		t.Fatalf("nullable null NumericAt returned error: %v", err)
	}
	if ok || n != 0 {
		t.Fatalf("nullable NumericAt null row = %v, %v; want 0, false", n, ok)
	}

	if _, _, err := typedKernels.NumericAt(NewString([]string{"x"}), 0); err == nil {
		t.Fatal("NumericAt accepted non-numeric typed column")
	}
}

func TestTypedCompareIndexesAndNullMasks(t *testing.T) {
	indexes, ok := typedKernels.CompareIndexes(NewI64([]int64{3, 5, 7, 5}), OpEQ, int64(5), nil)
	if !ok {
		t.Fatal("typed compare indexes did not match i64 column")
	}
	if want := []int{1, 3}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("typed compare indexes = %v, want %v", indexes, want)
	}

	indexes, ok = typedKernels.CompareIndexes(NewI64Range(0, 1, 6), OpGE, int64(3), indexes)
	if !ok {
		t.Fatal("typed compare indexes did not match i64 range")
	}
	if want := []int{3, 4, 5}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("range compare indexes = %v, want %v", indexes, want)
	}
	rangeMask := make([]bool, 6)
	if ok := typedKernels.CompareMask(NewI64Range(0, 1, 6), OpLT, int64(3), rangeMask); !ok {
		t.Fatal("typed compare mask did not match i64 range")
	}
	if want := []bool{true, true, true, false, false, false}; !reflect.DeepEqual(rangeMask, want) {
		t.Fatalf("range compare mask = %v, want %v", rangeMask, want)
	}

	indexes = []int{99}
	indexes, ok = typedKernels.CompareIndexes(NewString([]string{"a", "b", "c"}), OpGE, "b", indexes)
	if !ok {
		t.Fatal("typed compare indexes did not match string column")
	}
	if want := []int{1, 2}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("typed compare indexes with reused output = %v, want %v", indexes, want)
	}

	indexes, ok = typedKernels.CompareIndexes(NewSymbols([]string{"AAPL", "MSFT", "NVDA"}), OpLT, "NVDA", nil)
	if !ok {
		t.Fatal("typed compare indexes did not match symbol/string column")
	}
	if want := []int{0, 1}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("symbol compare indexes = %v, want %v", indexes, want)
	}

	indexes, ok = typedKernels.WithinIndexes(NewTimestamp([]Timestamp{10, 20, 30, 40}), Timestamp(15), Timestamp(30), true, nil)
	if !ok {
		t.Fatal("typed within indexes did not match timestamp column")
	}
	if want := []int{1, 2}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("timestamp within indexes = %v, want %v", indexes, want)
	}

	encoded := NewEncodedSymbols([]Symbol{"AAPL", "MSFT", "AAPL", "NVDA"})
	indexes, ok = typedKernels.CompareIndexes(encoded, OpEQ, "AAPL", nil)
	if !ok {
		t.Fatal("typed compare indexes did not match encoded symbol column")
	}
	if want := []int{0, 2}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("encoded symbol indexes = %v, want %v", indexes, want)
	}
	mask := make([]bool, encoded.Len())
	if ok := typedKernels.CompareMask(encoded, OpNE, Symbol("AAPL"), mask); !ok {
		t.Fatal("typed compare mask did not match encoded symbol column")
	}
	if want := []bool{false, true, false, true}; !reflect.DeepEqual(mask, want) {
		t.Fatalf("encoded symbol mask = %v, want %v", mask, want)
	}

	mask = make([]bool, 4)
	if ok := typedKernels.NullMask(NewColumn("x", []any{int64(1), nil, NullValue, int64(4)}).Data, mask); !ok {
		t.Fatal("typed null mask did not match nullable column")
	}
	if want := []bool{false, true, true, false}; !reflect.DeepEqual(mask, want) {
		t.Fatalf("typed null mask = %v, want %v", mask, want)
	}

	mask = make([]bool, 3)
	if ok := typedKernels.NullMask(NewF64([]float64{1, 2, 3}), mask); !ok {
		t.Fatal("typed null mask did not match f64 column")
	}
	if want := []bool{false, false, false}; !reflect.DeepEqual(mask, want) {
		t.Fatalf("typed null mask for dense column = %v, want %v", mask, want)
	}

	if count, ok := typedKernels.Count(NewBool([]bool{true, false})); !ok || count != 2 {
		t.Fatalf("typed count = %d, %v; want 2, true", count, ok)
	}
	if count, ok := typedKernels.NonNullCount(NewColumn("x", []any{1, nil, 3}).Data); !ok || count != 2 {
		t.Fatalf("typed non-null count = %d, %v; want 2, true", count, ok)
	}
}

func TestTypedInIndexesScansNonIndexedColumns(t *testing.T) {
	indexes, ok := typedKernels.InIndexes(NewI32([]int32{10, 20, 30, 20, 40}), []any{int64(20), int32(40)}, nil)
	if !ok {
		t.Fatal("typed in indexes did not match i32 column")
	}
	if want := []int{1, 3, 4}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("i32 in indexes = %v, want %v", indexes, want)
	}

	indexes = []int{99}
	indexes, ok = typedKernels.InIndexes(NewSymbols([]string{"AAPL", "MSFT", "NVDA", "AAPL"}), []any{"NVDA", Symbol("AAPL")}, indexes)
	if !ok {
		t.Fatal("typed in indexes did not match symbol column")
	}
	if want := []int{0, 2, 3}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("symbol in indexes = %v, want %v", indexes, want)
	}

	indexes, ok = typedKernels.InIndexes(NewTimestamp([]Timestamp{10, 20, 30, 40}), []any{Timestamp(20), Timestamp(40)}, nil)
	if !ok {
		t.Fatal("typed in indexes did not match timestamp column")
	}
	if want := []int{1, 3}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("timestamp in indexes = %v, want %v", indexes, want)
	}

	encoded := NewEncodedSymbols([]Symbol{"AAPL", "MSFT", "AAPL", "NVDA"})
	indexes, ok = typedKernels.InIndexes(encoded, []any{"MSFT", Symbol("AAPL")}, nil)
	if !ok {
		t.Fatal("typed in indexes did not match encoded symbol column")
	}
	if want := []int{0, 1, 2}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("encoded symbol in indexes = %v, want %v", indexes, want)
	}
}

func TestIndexedInRowsUsesAttributeIndexInRowOrder(t *testing.T) {
	indexed := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"}), ArrayAttributeGrouped)
	rows, ok := typedKernels.IndexedInRows(indexed, []any{"MSFT", Symbol("AAPL"), "MSFT"})
	if !ok {
		t.Fatal("IndexedInRows did not use grouped attribute index")
	}
	if want := []int{0, 1, 2, 4}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("IndexedInRows = %v, want %v", rows, want)
	}

	rows, ok = typedKernels.IndexedInRows(indexed, nil)
	if !ok || len(rows) != 0 {
		t.Fatalf("IndexedInRows empty = %v, %v; want empty rows through indexed path", rows, ok)
	}
	if rows, ok := typedKernels.IndexedInRows(NewSymbols([]string{"AAPL"}), []any{"AAPL"}); ok || rows != nil {
		t.Fatalf("IndexedInRows without index = %v, %v; want unsupported", rows, ok)
	}
	if rows, ok := typedKernels.IndexedInRows(indexed, []any{int64(1)}); ok || rows != nil {
		t.Fatalf("IndexedInRows incompatible literal = %v, %v; want fallback", rows, ok)
	}
}

func TestGroupCountsUsesArrayIndexRows(t *testing.T) {
	indexed := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"}), ArrayAttributeGrouped)
	index, ok := ArrayIndexFor(indexed, ArrayAttributeGrouped)
	if !ok {
		t.Fatal("grouped attribute did not expose index")
	}
	counts, ok := typedKernels.GroupCounts(index)
	if !ok {
		t.Fatal("GroupCounts did not accept grouped index")
	}
	if want := []int64{2, 2, 1}; !reflect.DeepEqual(counts, want) {
		t.Fatalf("GroupCounts = %v, want %v", counts, want)
	}
}

func TestFilteredGroupCountsPreservesFilteredFirstSeenOrder(t *testing.T) {
	indexed := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"}), ArrayAttributeGrouped)
	index, ok := ArrayIndexFor(indexed, ArrayAttributeGrouped)
	if !ok {
		t.Fatal("grouped attribute did not expose index")
	}
	order, counts, ok, err := typedKernels.FilteredGroupCounts(index, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("FilteredGroupCounts returned error: %v", err)
	}
	if !ok {
		t.Fatal("FilteredGroupCounts did not accept grouped index")
	}
	if want := []int{1, 0, 2}; !reflect.DeepEqual(order, want) {
		t.Fatalf("FilteredGroupCounts order = %v, want %v", order, want)
	}
	if want := []int64{1, 1, 1}; !reflect.DeepEqual(counts, want) {
		t.Fatalf("FilteredGroupCounts counts = %v, want %v", counts, want)
	}

	order, counts, ok, err = typedKernels.FilteredGroupCounts(index, nil)
	if err != nil || !ok || len(order) != 0 || !reflect.DeepEqual(counts, []int64{0, 0, 0}) {
		t.Fatalf("FilteredGroupCounts empty = order %v counts %v ok %v err %v; want empty order zero counts", order, counts, ok, err)
	}
}

func TestGroupedAttributeMixedAggregateKernel(t *testing.T) {
	indexed := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"}), ArrayAttributeGrouped)
	index, ok := ArrayIndexFor(indexed, ArrayAttributeGrouped)
	if !ok {
		t.Fatal("grouped attribute did not expose index")
	}
	qty := WithArrayAttribute(NewI32([]int32{10, 20, 30, 40, 50}), ArrayAttributeSorted)
	px := WithArrayAttribute(NewF64([]float64{100, 200, 110, 300, 210}), ArrayAttributeSorted)
	venue := NewString([]string{"XNAS", "BATS", "IEX", "ARCX", "EDGX"})
	aggs := []aggregateInput{
		{Aggregate: Aggregate{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}}, column: qty},
		{Aggregate: Aggregate{Name: "avg_px", Func: "avg", Expr: ColumnRef{Name: "px"}}, column: px},
		{Aggregate: Aggregate{Name: "lo_px", Func: "min", Expr: ColumnRef{Name: "px"}}, column: px},
		{Aggregate: Aggregate{Name: "hi_px", Func: "max", Expr: ColumnRef{Name: "px"}}, column: px},
		{Aggregate: Aggregate{Name: "n", Func: "count"}},
		{Aggregate: Aggregate{Name: "first_venue", Func: "first", Expr: ColumnRef{Name: "venue"}}, column: venue},
		{Aggregate: Aggregate{Name: "last_venue", Func: "last", Expr: ColumnRef{Name: "venue"}}, column: venue},
	}
	states, ok, err := typedKernels.GroupAggregateStates(index, aggs)
	if err != nil || !ok {
		t.Fatalf("GroupAggregateStates ok %v err %v; want typed aggregate states", ok, err)
	}
	if got, want := aggregateResult(states[0].aggs[0]), 40.0; got != want {
		t.Fatalf("AAPL sum = %v, want %v", got, want)
	}
	if got, want := aggregateResult(states[1].aggs[1]), 205.0; got != want {
		t.Fatalf("MSFT avg = %v, want %v", got, want)
	}
	if got, want := aggregateResult(states[0].aggs[2]), 100.0; got != want {
		t.Fatalf("AAPL min = %v, want %v", got, want)
	}
	if got, want := aggregateResult(states[1].aggs[3]), 210.0; got != want {
		t.Fatalf("MSFT max = %v, want %v", got, want)
	}
	if got, want := aggregateResult(states[2].aggs[4]), int64(1); got != want {
		t.Fatalf("NVDA count = %v, want %v", got, want)
	}
	if got, want := aggregateResult(states[0].aggs[5]), "XNAS"; got != want {
		t.Fatalf("AAPL first venue = %v, want %v", got, want)
	}
	if got, want := aggregateResult(states[1].aggs[6]), "EDGX"; got != want {
		t.Fatalf("MSFT last venue = %v, want %v", got, want)
	}

	order, filtered, ok, err := typedKernels.FilteredGroupAggregateStates(index, []int{2, 4, 3}, aggs)
	if err != nil || !ok {
		t.Fatalf("FilteredGroupAggregateStates ok %v err %v; want typed aggregate states", ok, err)
	}
	if want := []int{0, 1, 2}; !reflect.DeepEqual(order, want) {
		t.Fatalf("filtered group order = %v, want %v", order, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[0]), 30.0; got != want {
		t.Fatalf("filtered AAPL sum = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[1].aggs[1]), 210.0; got != want {
		t.Fatalf("filtered MSFT avg = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[2].aggs[4]), int64(1); got != want {
		t.Fatalf("filtered NVDA count = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[5]), "IEX"; got != want {
		t.Fatalf("filtered AAPL first venue = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[1].aggs[6]), "EDGX"; got != want {
		t.Fatalf("filtered MSFT last venue = %v, want %v", got, want)
	}

	order, filtered, ok, err = typedKernels.FilteredGroupAggregateStates(index, []int{4, 2, 0, 4}, aggs)
	if err != nil || !ok {
		t.Fatalf("FilteredGroupAggregateStates duplicate rows ok %v err %v; want typed aggregate states", ok, err)
	}
	if want := []int{1, 0}; !reflect.DeepEqual(order, want) {
		t.Fatalf("duplicate filtered group order = %v, want %v", order, want)
	}
	if got, want := aggregateResult(filtered[1].aggs[0]), 100.0; got != want {
		t.Fatalf("duplicate filtered MSFT sum = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[1].aggs[1]), 210.0; got != want {
		t.Fatalf("duplicate filtered MSFT avg = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[0]), 40.0; got != want {
		t.Fatalf("duplicate filtered AAPL sum = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[1]), 105.0; got != want {
		t.Fatalf("duplicate filtered AAPL avg = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[5]), "IEX"; got != want {
		t.Fatalf("duplicate filtered AAPL first venue = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[6]), "XNAS"; got != want {
		t.Fatalf("duplicate filtered AAPL last venue = %v, want %v", got, want)
	}
}

func TestFilteredGroupedAggregateKernelPreservesNullAndFilteredOrder(t *testing.T) {
	indexed := WithArrayAttribute(NewSymbols([]string{"AAPL", "AAPL", "MSFT", "MSFT"}), ArrayAttributeGrouped)
	index, ok := ArrayIndexFor(indexed, ArrayAttributeGrouped)
	if !ok {
		t.Fatal("grouped attribute did not expose index")
	}
	qty := NewColumn("qty", []any{nil, int64(10), nil, int64(20)}).Data
	venue := NewColumn("venue", []any{nil, "x", nil, "y"}).Data
	aggs := []aggregateInput{
		{Aggregate: Aggregate{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}}, column: qty},
		{Aggregate: Aggregate{Name: "avg_qty", Func: "avg", Expr: ColumnRef{Name: "qty"}}, column: qty},
		{Aggregate: Aggregate{Name: "first_venue", Func: "first", Expr: ColumnRef{Name: "venue"}}, column: venue},
		{Aggregate: Aggregate{Name: "last_venue", Func: "last", Expr: ColumnRef{Name: "venue"}}, column: venue},
	}

	order, filtered, ok, err := typedKernels.FilteredGroupAggregateStates(index, []int{0, 1, 3, 2}, aggs)
	if err != nil || !ok {
		t.Fatalf("FilteredGroupAggregateStates nullable ok %v err %v; want typed aggregate states", ok, err)
	}
	if want := []int{0, 1}; !reflect.DeepEqual(order, want) {
		t.Fatalf("nullable filtered group order = %v, want %v", order, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[0]), 10.0; got != want {
		t.Fatalf("nullable filtered AAPL sum = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[0].aggs[1]), 10.0; got != want {
		t.Fatalf("nullable filtered AAPL avg = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[1].aggs[0]), 20.0; got != want {
		t.Fatalf("nullable filtered MSFT sum = %v, want %v", got, want)
	}
	if got, want := aggregateResult(filtered[1].aggs[1]), 20.0; got != want {
		t.Fatalf("nullable filtered MSFT avg = %v, want %v", got, want)
	}
	if got := aggregateResult(filtered[0].aggs[2]); got != NullValue {
		t.Fatalf("nullable filtered AAPL first venue = %v, want NullValue", got)
	}
	if got := aggregateResult(filtered[1].aggs[3]); got != NullValue {
		t.Fatalf("nullable filtered MSFT last venue = %v, want NullValue from filtered order", got)
	}
}

func TestComplementSortedIndexesKernel(t *testing.T) {
	got, ok, err := typedKernels.ComplementSortedIndexes(6, []int{1, 3, 5})
	if err != nil {
		t.Fatalf("ComplementSortedIndexes returned error: %v", err)
	}
	if !ok {
		t.Fatal("ComplementSortedIndexes did not accept sorted excludes")
	}
	if want := []int{0, 2, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ComplementSortedIndexes = %v, want %v", got, want)
	}

	got, ok, err = typedKernels.ComplementSortedIndexes(3, nil)
	if err != nil {
		t.Fatalf("ComplementSortedIndexes nil returned error: %v", err)
	}
	if !ok || !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("ComplementSortedIndexes nil = %v, %v; want all indexes", got, ok)
	}

	got, ok, err = typedKernels.ComplementSortedIndexes(3, []int{0, 1, 2})
	if err != nil {
		t.Fatalf("ComplementSortedIndexes all returned error: %v", err)
	}
	if !ok || len(got) != 0 {
		t.Fatalf("ComplementSortedIndexes all = %v, %v; want empty", got, ok)
	}

	if _, ok, err = typedKernels.ComplementSortedIndexes(4, []int{2, 1}); err != nil || ok {
		t.Fatalf("ComplementSortedIndexes unsorted = ok %v, err %v; want fallback", ok, err)
	}
	if _, ok, err = typedKernels.ComplementSortedIndexes(4, []int{2, 2}); err != nil || ok {
		t.Fatalf("ComplementSortedIndexes duplicate = ok %v, err %v; want fallback", ok, err)
	}
	if _, _, err = typedKernels.ComplementSortedIndexes(4, []int{4}); err == nil {
		t.Fatal("ComplementSortedIndexes accepted out-of-range row")
	}
}

func TestTypedNumericUnaryBinaryAndAggregates(t *testing.T) {
	neg, ok, err := typedKernels.NumericUnary(NumericUnaryNeg, NewI32([]int32{2, -3, 0}))
	if err != nil {
		t.Fatalf("NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("NumericUnary did not match i32 column")
	}
	if got, want := neg.Values(), []any{-2.0, 3.0, -0.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericUnary values = %v, want %v", got, want)
	}

	abs, ok, err := typedKernels.NumericUnary(NumericUnaryAbs, NewColumn("x", []any{float64(-1.5), nil, int64(2)}).Data)
	if err != nil {
		t.Fatalf("nullable NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("nullable NumericUnary did not match numeric nullable column")
	}
	if got, want := abs.Values(), []any{1.5, NullValue, 2.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nullable NumericUnary values = %v, want %v", got, want)
	}

	sqrt, ok, err := typedKernels.NumericUnary(NumericUnarySqrt, NewF64([]float64{4, 9, 16}))
	if err != nil {
		t.Fatalf("sqrt NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("sqrt NumericUnary did not match numeric column")
	}
	if got, want := sqrt.Values(), []any{2.0, 3.0, 4.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sqrt NumericUnary values = %v, want %v", got, want)
	}

	logged, ok, err := typedKernels.NumericUnary(NumericUnaryLog, NewF64([]float64{1, math.E}))
	if err != nil {
		t.Fatalf("log NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("log NumericUnary did not match numeric column")
	}
	gotLog := logged.Values()
	if len(gotLog) != 2 || gotLog[0].(float64) != 0 || math.Abs(gotLog[1].(float64)-1) > 1e-12 {
		t.Fatalf("log NumericUnary values = %v, want [0 1]", gotLog)
	}

	exponent, ok, err := typedKernels.NumericUnary(NumericUnaryExp, NewF64([]float64{0, 1}))
	if err != nil {
		t.Fatalf("exp NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("exp NumericUnary did not match numeric column")
	}
	gotExp := exponent.Values()
	if len(gotExp) != 2 || gotExp[0].(float64) != 1 || math.Abs(gotExp[1].(float64)-math.E) > 1e-12 {
		t.Fatalf("exp NumericUnary values = %v, want [1 e]", gotExp)
	}

	recip, ok, err := typedKernels.NumericUnary(NumericUnaryRecip, NewF64([]float64{2, 4, 0}))
	if err != nil {
		t.Fatalf("reciprocal NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("reciprocal NumericUnary did not match numeric column")
	}
	if got, want := recip.Values(), []any{0.5, 0.25, math.Inf(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reciprocal NumericUnary values = %v, want %v", got, want)
	}

	sign, ok, err := typedKernels.NumericUnary(NumericUnarySignum, NewF64([]float64{-2.5, 0, 7}))
	if err != nil {
		t.Fatalf("signum NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("signum NumericUnary did not match numeric column")
	}
	if got, want := sign.Values(), []any{-1.0, 0.0, 1.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("signum NumericUnary values = %v, want %v", got, want)
	}

	floored, ok, err := typedKernels.NumericUnary(NumericUnaryFloor, NewF64([]float64{-1.2, 1.9, 3}))
	if err != nil {
		t.Fatalf("floor NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("floor NumericUnary did not match numeric column")
	}
	if got, want := floored.Values(), []any{-2.0, 1.0, 3.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("floor NumericUnary values = %v, want %v", got, want)
	}

	ceiled, ok, err := typedKernels.NumericUnary(NumericUnaryCeiling, NewF64([]float64{-1.2, 1.1, 3}))
	if err != nil {
		t.Fatalf("ceiling NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("ceiling NumericUnary did not match numeric column")
	}
	if got, want := ceiled.Values(), []any{-1.0, 2.0, 3.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ceiling NumericUnary values = %v, want %v", got, want)
	}

	sum, ok, err := typedKernels.NumericBinary(OpAdd, NewI32([]int32{1, 2, 3}), NewF64([]float64{0.5, 1.5, 2.5}))
	if err != nil {
		t.Fatalf("NumericBinary returned error: %v", err)
	}
	if !ok {
		t.Fatal("NumericBinary did not match numeric columns")
	}
	if got, want := sum.Values(), []any{1.5, 3.5, 5.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericBinary values = %v, want %v", got, want)
	}

	div, ok, err := typedKernels.NumericBinary(OpDiv, NewColumn("x", []any{float64(6), nil, float64(9)}).Data, NewF64([]float64{2, 3, 3}))
	if err != nil {
		t.Fatalf("nullable NumericBinary returned error: %v", err)
	}
	if !ok {
		t.Fatal("nullable NumericBinary did not match numeric columns")
	}
	if got, want := div.Values(), []any{3.0, NullValue, 3.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nullable NumericBinary values = %v, want %v", got, want)
	}

	total, count, ok, err := typedKernels.NumericSum(NewColumn("x", []any{float64(1.25), nil, int64(4)}).Data)
	if err != nil {
		t.Fatalf("NumericSum returned error: %v", err)
	}
	if !ok || total != 5.25 || count != 2 {
		t.Fatalf("NumericSum = %v, %d, %v; want 5.25, 2, true", total, count, ok)
	}

	total, count, ok, err = typedKernels.NumericSumRows(NewI32([]int32{10, 20, 30, 40}), []int{2, 0, 2})
	if err != nil {
		t.Fatalf("NumericSumRows returned error: %v", err)
	}
	if !ok || total != 70 || count != 3 {
		t.Fatalf("NumericSumRows = %v, %d, %v; want 70, 3, true", total, count, ok)
	}
	total, count, ok, err = typedKernels.NumericSumRows(NewColumn("x", []any{float64(1.25), nil, int64(4)}).Data, []int{0, 1, 2})
	if err != nil {
		t.Fatalf("nullable NumericSumRows returned error: %v", err)
	}
	if !ok || total != 5.25 || count != 2 {
		t.Fatalf("nullable NumericSumRows = %v, %d, %v; want 5.25, 2, true", total, count, ok)
	}
	if _, _, _, err := typedKernels.NumericSumRows(NewI32([]int32{1}), []int{1}); err == nil {
		t.Fatal("NumericSumRows accepted row past end")
	}

	value, ok, err := TryTypedNumericSum(NewI32([]int32{1, 2, 3}))
	if err != nil {
		t.Fatalf("TryTypedNumericSum i32 returned error: %v", err)
	}
	if !ok || value != int64(6) {
		t.Fatalf("TryTypedNumericSum i32 = %v, %v; want 6, true", value, ok)
	}

	value, ok, err = TryTypedNumericSum(NewF32([]float32{1.5, 2.5}))
	if err != nil {
		t.Fatalf("TryTypedNumericSum f32 returned error: %v", err)
	}
	if !ok || value != float64(4) {
		t.Fatalf("TryTypedNumericSum f32 = %v, %v; want 4.0, true", value, ok)
	}

	value, ok, err = TryTypedNumericSum(NewColumn("x", []any{int64(1), nil, float64(2.5)}).Data)
	if err != nil {
		t.Fatalf("TryTypedNumericSum nullable returned error: %v", err)
	}
	if !ok || value != float64(3.5) {
		t.Fatalf("TryTypedNumericSum nullable = %v, %v; want 3.5, true", value, ok)
	}

	value, ok, err = TryTypedNumericSum(NewI64Range(0, 1, 8192))
	if err != nil {
		t.Fatalf("TryTypedNumericSum range returned error: %v", err)
	}
	if want := int64(8192 * 8191 / 2); !ok || value != want {
		t.Fatalf("TryTypedNumericSum range = %v, %v; want %d, true", value, ok, want)
	}

	scan, ok, err := TryTypedNumericSums(NewI64([]int64{1, 2, 3}))
	if err != nil {
		t.Fatalf("TryTypedNumericSums i64 returned error: %v", err)
	}
	if !ok || scan.Kind() != KindI64 {
		t.Fatalf("TryTypedNumericSums i64 kind = %s, %v; want i64, true", scan.Kind(), ok)
	}
	if got, want := scan.Values(), []any{int64(1), int64(3), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedNumericSums i64 values = %v, want %v", got, want)
	}

	scan, ok, err = TryTypedNumericSums(NewI64Range(0, 1, 4))
	if err != nil {
		t.Fatalf("TryTypedNumericSums range returned error: %v", err)
	}
	if !ok || scan.Kind() != KindI64 {
		t.Fatalf("TryTypedNumericSums range kind = %s, %v; want i64, true", scan.Kind(), ok)
	}
	if got, want := scan.Values(), []any{int64(0), int64(1), int64(3), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedNumericSums range values = %v, want %v", got, want)
	}

	total, count, ok, err = typedKernels.NumericSumRows(NewI64Range(10, 2, 5), []int{4, 0, 2})
	if err != nil {
		t.Fatalf("range NumericSumRows returned error: %v", err)
	}
	if !ok || total != 42 || count != 3 {
		t.Fatalf("range NumericSumRows = %v, %d, %v; want 42, 3, true", total, count, ok)
	}

	scan, ok, err = TryTypedNumericSums(NewColumn("x", []any{int64(1), nil, float64(2.5)}).Data)
	if err != nil {
		t.Fatalf("TryTypedNumericSums nullable returned error: %v", err)
	}
	if !ok || scan.Kind() != KindF64 {
		t.Fatalf("TryTypedNumericSums nullable kind = %s, %v; want f64, true", scan.Kind(), ok)
	}
	if got, want := scan.Values(), []any{float64(1), float64(1), float64(3.5)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedNumericSums nullable values = %v, want %v", got, want)
	}

	min, has, ok, err := typedKernels.Min(NewTimestamp([]Timestamp{30, 10, 20}))
	if err != nil {
		t.Fatalf("Min returned error: %v", err)
	}
	if !ok || !has || min != Timestamp(10) {
		t.Fatalf("Min = %v, %v, %v; want 10, true, true", min, has, ok)
	}
	max, has, ok, err := typedKernels.Max(NewColumn("x", []any{NullValue, Symbol("b"), Symbol("a")}).Data)
	if err != nil {
		t.Fatalf("Max returned error: %v", err)
	}
	if !ok || !has || max != Symbol("b") {
		t.Fatalf("Max = %v, %v, %v; want b, true, true", max, has, ok)
	}
}

func TestTypedDyadicBroadcastPromotionAndNullPropagation(t *testing.T) {
	scalarRight, ok, err := typedKernels.Dyadic(OpAdd, NewI32([]int32{1, 2, 3}), int64(10))
	if err != nil {
		t.Fatalf("Dyadic scalar right returned error: %v", err)
	}
	if !ok {
		t.Fatal("Dyadic scalar right did not match numeric column")
	}
	if got, want := scalarRight.(Array).Values(), []any{11.0, 12.0, 13.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Dyadic scalar right values = %v, want %v", got, want)
	}

	rangeRight, ok, err := typedKernels.Dyadic(OpMul, NewI64Range(0, 2, 4), int64(3))
	if err != nil {
		t.Fatalf("Dyadic range scalar returned error: %v", err)
	}
	if !ok {
		t.Fatal("Dyadic range scalar did not match numeric range")
	}
	if got, want := rangeRight.(Array).Values(), []any{0.0, 6.0, 12.0, 18.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Dyadic range scalar values = %v, want %v", got, want)
	}

	integerRangeRight, ok, err := TryTypedIntegerDyadic(OpMul, NewI64Range(0, 2, 4), int64(3))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic range scalar returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic range scalar did not match numeric range")
	}
	if got, want := integerRangeRight.(Array).Values(), []any{int64(0), int64(6), int64(12), int64(18)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedIntegerDyadic range scalar values = %v, want %v", got, want)
	}

	scalarLeft, ok, err := typedKernels.Dyadic(OpSub, float64(10), NewI64([]int64{1, 2, 3}))
	if err != nil {
		t.Fatalf("Dyadic scalar left returned error: %v", err)
	}
	if !ok {
		t.Fatal("Dyadic scalar left did not match numeric column")
	}
	if got, want := scalarLeft.(Array).Values(), []any{9.0, 8.0, 7.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Dyadic scalar left values = %v, want %v", got, want)
	}

	withNull, ok, err := typedKernels.Dyadic(OpMul, NewColumn("x", []any{int64(2), nil, int64(4)}).Data, NewF32([]float32{1.5, 2.5, 3.5}))
	if err != nil {
		t.Fatalf("Dyadic nullable returned error: %v", err)
	}
	if !ok {
		t.Fatal("Dyadic nullable did not match numeric columns")
	}
	if got, want := withNull.(Array).Values(), []any{3.0, NullValue, 14.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Dyadic nullable values = %v, want %v", got, want)
	}

	if _, ok, err := typedKernels.Dyadic(OpAdd, NewI64([]int64{1, 2}), NewI64([]int64{1})); err == nil || !ok {
		t.Fatalf("Dyadic length mismatch err = %v, ok %v; want handled error", err, ok)
	}
}

func TestTypedDyadicSymbolAndTemporalComparisons(t *testing.T) {
	applied, handled, err := TryTypedDyadic(OpLT, NewI64([]int64{1, 2, 3}), NewI64([]int64{2, 2, 2}))
	if err != nil {
		t.Fatalf("TryTypedDyadic returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedDyadic did not handle typed comparison")
	}
	if got, want := applied.(Array).Values(), []any{true, false, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedDyadic values = %v, want %v", got, want)
	}
	if _, handled, err := TryTypedDyadic(OpLT, NewI64([]int64{1, 2}), NewI64([]int64{1})); err == nil || !handled {
		t.Fatalf("TryTypedDyadic mismatch err = %v handled = %v, want handled mismatch error", err, handled)
	}

	symEq, ok, err := typedKernels.Dyadic(OpEQ, NewSymbols([]string{"a", "b", "c"}), Symbol("b"))
	if err != nil {
		t.Fatalf("symbol Dyadic returned error: %v", err)
	}
	if !ok {
		t.Fatal("symbol Dyadic did not match")
	}
	if got, want := symEq.(Array).Values(), []any{false, true, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("symbol Dyadic values = %v, want %v", got, want)
	}

	symStringEq, ok, err := typedKernels.Dyadic(OpEQ, NewSymbols([]string{"a", "b", "c"}), "b")
	if err != nil {
		t.Fatalf("symbol/string Dyadic returned error: %v", err)
	}
	if !ok {
		t.Fatal("symbol/string Dyadic did not match")
	}
	if got, want := symStringEq.(Array).Values(), []any{false, true, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("symbol/string Dyadic values = %v, want %v", got, want)
	}

	temporalGE, ok, err := typedKernels.Dyadic(OpGE, NewDate([]Date{DateFromDays(1), DateFromDays(2), DateFromDays(3)}), NewDate([]Date{DateFromDays(2), DateFromDays(2), DateFromDays(2)}))
	if err != nil {
		t.Fatalf("temporal Dyadic returned error: %v", err)
	}
	if !ok {
		t.Fatal("temporal Dyadic did not match")
	}
	if got, want := temporalGE.(Array).Values(), []any{false, true, true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("temporal Dyadic values = %v, want %v", got, want)
	}

	nulls, ok, err := typedKernels.Dyadic(OpEQ, NewColumn("x", []any{Symbol("a"), nil, Symbol("b")}).Data, NewColumn("y", []any{Symbol("a"), nil, nil}).Data)
	if err != nil {
		t.Fatalf("nullable compare Dyadic returned error: %v", err)
	}
	if !ok {
		t.Fatal("nullable compare Dyadic did not match")
	}
	if got, want := nulls.(Array).Values(), []any{true, true, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nullable compare Dyadic values = %v, want %v", got, want)
	}
}

func TestTypedJoinRowsByKeyIncludesNullAndDuplicateKeys(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), nil, Symbol("a"), NullValue}),
		NewColumn("venue", []any{"x", "x", "x", "x"}),
		NewColumn("qty", []any{1, 2, 3, 4}),
	)

	rowsByKey, err := typedKernels.RowsByKey(frame, []Symbol{"sym", "venue"})
	if err != nil {
		t.Fatalf("RowsByKey returned error: %v", err)
	}

	aKey, err := rowKey(frame, 0, []Symbol{"sym", "venue"})
	if err != nil {
		t.Fatalf("rowKey returned error: %v", err)
	}
	nullKey, err := rowKey(frame, 1, []Symbol{"sym", "venue"})
	if err != nil {
		t.Fatalf("rowKey null returned error: %v", err)
	}
	if got, want := rowsByKey[aKey], []int{0, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicate key rows = %v, want %v", got, want)
	}
	if got, want := rowsByKey[nullKey], []int{1, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("null key rows = %v, want %v", got, want)
	}
}

func TestTypedJoinRowsByKeyUsesRebuiltSingleColumnAttributeIndex(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL"}), ArrayAttributeGrouped)},
		NewColumn("qty", []any{10, 20, 30}),
	)
	gathered, err := frame.Gather([]int{2, 0, 1})
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}
	rowsByKey, err := typedKernels.RowsByKey(gathered, []Symbol{"sym"})
	if err != nil {
		t.Fatalf("RowsByKey returned error: %v", err)
	}
	aaplKey, err := rowKey(gathered, 0, []Symbol{"sym"})
	if err != nil {
		t.Fatalf("rowKey returned error: %v", err)
	}
	msftKey, err := rowKey(gathered, 2, []Symbol{"sym"})
	if err != nil {
		t.Fatalf("rowKey MSFT returned error: %v", err)
	}
	if got, want := rowsByKey[aaplKey], []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AAPL rows = %v, want %v", got, want)
	}
	if got, want := rowsByKey[msftKey], []int{2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MSFT rows = %v, want %v", got, want)
	}
}

func TestTypedJoinRowsByKeyUsesSingleColumnTypedKeys(t *testing.T) {
	tests := []struct {
		name   string
		column Column
		want   []int
	}{
		{
			name:   "u32",
			column: Column{Name: "k", Data: NewU32([]uint32{7, 9, 7, 11})},
			want:   []int{0, 2},
		},
		{
			name:   "date",
			column: Column{Name: "k", Data: NewDate([]Date{DateFromDays(1), DateFromDays(2), DateFromDays(1)})},
			want:   []int{0, 2},
		},
		{
			name:   "encoded symbol",
			column: Column{Name: "k", Data: NewEncodedSymbols([]Symbol{"AAPL", "MSFT", "AAPL"})},
			want:   []int{0, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := mustFrame(t, tt.column)
			rowsByKey, err := typedKernels.RowsByKey(frame, []Symbol{"k"})
			if err != nil {
				t.Fatalf("RowsByKey returned error: %v", err)
			}
			key, err := rowKey(frame, 0, []Symbol{"k"})
			if err != nil {
				t.Fatalf("rowKey returned error: %v", err)
			}
			if got := rowsByKey[key]; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("RowsByKey[%q] = %v, want %v", key, got, tt.want)
			}
		})
	}
}

func TestSingleColumnJoinEncoderUsesTypedKeyWhenTargetKindMatches(t *testing.T) {
	frame := mustFrame(t, NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}))
	encoder, err := newRowKeyEncoderWithKinds(frame, []Symbol{"sym"}, []Kind{KindSymbol})
	if err != nil {
		t.Fatalf("newRowKeyEncoderWithKinds matching kind returned error: %v", err)
	}
	if encoder.single == nil {
		t.Fatal("matching single-column join encoder did not use typed key fast path")
	}
	got, ok, err := encoder.lookupKeyWithBuilder(0, &strings.Builder{})
	if err != nil || !ok {
		t.Fatalf("matching single-column lookup key = %q ok=%v err=%v, want key without error", got, ok, err)
	}
	want, err := rowKey(frame, 0, []Symbol{"sym"})
	if err != nil {
		t.Fatalf("rowKey returned error: %v", err)
	}
	if got != want {
		t.Fatalf("matching single-column lookup key = %q, want %q", got, want)
	}

	coerced, err := newRowKeyEncoderWithKinds(frame, []Symbol{"sym"}, []Kind{KindString})
	if err != nil {
		t.Fatalf("newRowKeyEncoderWithKinds coercing kind returned error: %v", err)
	}
	if coerced.single != nil {
		t.Fatal("coercing single-column join encoder used typed key fast path; want normalization path")
	}
}

func TestTypedAsofAndWindowMatchIndexesBoundaries(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("ts", []any{int64(9), int64(10), nil, int64(12)}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("ts", []any{int64(10), int64(8), int64(10), int64(13), nil}),
		NewColumn("quote", []any{"a10-first", "a8", "a10-last", "b13", "null-time"}),
	)
	rightTime, _ := right.Column("ts")
	rightByPartition, err := typedKernels.SortedRowsByPartition(right, rightTime, []Symbol{"sym"})
	if err != nil {
		t.Fatalf("SortedRowsByPartition returned error: %v", err)
	}

	leftTime, _ := left.Column("ts")
	asof, err := typedKernels.AsofMatchIndexes(left, leftTime, []Symbol{"sym"}, rightTime, rightByPartition)
	if err != nil {
		t.Fatalf("AsofMatchIndexes returned error: %v", err)
	}
	if want := []int{1, 2, -1, -1}; !reflect.DeepEqual(asof, want) {
		t.Fatalf("AsofMatchIndexes = %v, want %v", asof, want)
	}

	window, err := typedKernels.WindowMatchIndexes(left, leftTime, []Symbol{"sym"}, rightTime, rightByPartition, WindowJoinOptions{
		Low:       int64(0),
		High:      int64(0),
		HasBounds: true,
	})
	if err != nil {
		t.Fatalf("WindowMatchIndexes returned error: %v", err)
	}
	if want := [][]int{{}, {0, 2}, {}, {}}; !reflect.DeepEqual(window, want) {
		t.Fatalf("WindowMatchIndexes = %v, want %v", window, want)
	}
	if got, want := typedKernels.GatherLastOptional(right.columns["quote"], window).Values(), []any{NullValue, "a10-last", NullValue, NullValue}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GatherLastOptional = %v, want %v", got, want)
	}
	if got, want := typedKernels.GatherWindowLists(right.columns["quote"], window).Values(), []any{[]any{}, []any{"a10-first", "a10-last"}, []any{}, []any{}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GatherWindowLists = %v, want %v", got, want)
	}
}

func TestTypedAsofRecognizesSortedAttributeMetadata(t *testing.T) {
	left := mustFrame(t,
		NewColumn("ts", []any{int64(6), int64(11)}),
	)
	right := mustFrame(t,
		Column{Name: "ts", Data: WithArrayAttribute(NewI64([]int64{5, 10, 15}), ArrayAttributeSorted)},
		NewColumn("quote", []any{"q5", "q10", "q15"}),
	)
	rightTime, _ := right.Column("ts")
	if !ArrayHasAttribute(rightTime, ArrayAttributeSorted) {
		t.Fatalf("right time metadata = %#v, want sorted", ArrayMetadataOf(rightTime))
	}
	rightByPartition, err := typedKernels.SortedRowsByPartition(right, rightTime, nil)
	if err != nil {
		t.Fatalf("SortedRowsByPartition returned error: %v", err)
	}
	leftTime, _ := left.Column("ts")
	matches, err := typedKernels.AsofMatchIndexes(left, leftTime, nil, rightTime, rightByPartition)
	if err != nil {
		t.Fatalf("AsofMatchIndexes returned error: %v", err)
	}
	if want := []int{0, 1}; !reflect.DeepEqual(matches, want) {
		t.Fatalf("AsofMatchIndexes = %v, want %v", matches, want)
	}
}

func TestTypedWindowMatchIndexesGlobalPartitionAndGatherOptional(t *testing.T) {
	left := mustFrame(t,
		NewColumn("ts", []any{int64(5), int64(10), int64(15)}),
	)
	right := mustFrame(t,
		NewColumn("ts", []any{int64(3), int64(12), int64(8)}),
		NewColumn("px", []any{30.0, 120.0, 80.0}),
	)
	rightTime, _ := right.Column("ts")
	rightByPartition, err := typedKernels.SortedRowsByPartition(right, rightTime, nil)
	if err != nil {
		t.Fatalf("SortedRowsByPartition returned error: %v", err)
	}
	if got, want := rightByPartition[""], []int{0, 2, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("global sorted partition rows = %v, want %v", got, want)
	}

	leftTime, _ := left.Column("ts")
	matches, err := typedKernels.WindowMatchIndexes(left, leftTime, nil, rightTime, rightByPartition, WindowJoinOptions{})
	if err != nil {
		t.Fatalf("WindowMatchIndexes returned error: %v", err)
	}
	if want := [][]int{{0}, {0, 2}, {0, 2, 1}}; !reflect.DeepEqual(matches, want) {
		t.Fatalf("WindowMatchIndexes = %v, want %v", matches, want)
	}
	if got, want := typedKernels.GatherOptional(right.columns["px"], []int{0, -1, 2}).Values(), []any{30.0, NullValue, 80.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GatherOptional = %v, want %v", got, want)
	}
}

func TestTypedAsofPartitionIndexSurvivesGatherAndSortsByTime(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("ts", []any{int64(11), int64(9)}),
	)
	right := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"AAPL", "AAPL", "MSFT"}), ArrayAttributeGrouped)},
		NewColumn("ts", []any{int64(10), int64(8), int64(7)}),
		NewColumn("quote", []any{"a10", "a8", "m7"}),
	)
	gatheredRight, err := right.Gather([]int{1, 0, 2})
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}
	rightTime, _ := gatheredRight.Column("ts")
	rightByPartition, err := typedKernels.SortedRowsByPartition(gatheredRight, rightTime, []Symbol{"sym"})
	if err != nil {
		t.Fatalf("SortedRowsByPartition returned error: %v", err)
	}
	leftTime, _ := left.Column("ts")
	matches, err := typedKernels.AsofMatchIndexes(left, leftTime, []Symbol{"sym"}, rightTime, rightByPartition)
	if err != nil {
		t.Fatalf("AsofMatchIndexes returned error: %v", err)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(matches, want) {
		t.Fatalf("AsofMatchIndexes = %v, want %v", matches, want)
	}
}

func TestTypedWindowMatchIndexesRejectsInvalidTemporalBounds(t *testing.T) {
	left := mustFrame(t, NewColumn("ts", []any{TimestampFromUnixNanos(10)}))
	right := mustFrame(t, NewColumn("ts", []any{TimestampFromUnixNanos(10)}))
	rightTime, _ := right.Column("ts")
	rightByPartition, err := typedKernels.SortedRowsByPartition(right, rightTime, nil)
	if err != nil {
		t.Fatalf("SortedRowsByPartition returned error: %v", err)
	}
	leftTime, _ := left.Column("ts")

	if _, err := typedKernels.WindowMatchIndexes(left, leftTime, nil, rightTime, rightByPartition, WindowJoinOptions{
		Low:       int64(1),
		High:      int64(-1),
		HasBounds: true,
	}); err == nil {
		t.Fatal("WindowMatchIndexes accepted inverted bounds")
	}
	if _, err := typedKernels.WindowMatchIndexes(left, leftTime, nil, rightTime, rightByPartition, WindowJoinOptions{
		Low:       float64(0.5),
		High:      int64(0),
		HasBounds: true,
	}); err == nil {
		t.Fatal("WindowMatchIndexes accepted fractional temporal delta")
	}
}

func TestQueryWhereColumnLiteralAndTypedGroupAggregates(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"a", "a", "b", "b"})},
		Column{Name: "qty", Data: NewI32([]int32{5, 2, 7, 4})},
		Column{Name: "px", Data: NewF64([]float64{10, 20, 30, 40})},
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpGT, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(3)}},
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "avg_px", Func: "avg", Expr: ColumnRef{Name: "px"}},
			{Name: "fills", Func: "count"},
		},
		OrderBy: []OrderSpec{{Column: "sym"}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b")})
	assertColumnValues(t, got, "total_qty", []any{5.0, 11.0})
	assertColumnValues(t, got, "avg_px", []any{10.0, 35.0})
	assertColumnValues(t, got, "fills", []any{int64(1), int64(2)})
}
