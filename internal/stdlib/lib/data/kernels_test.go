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

func TestTryTypedNumericStatsIntegerAndFloat(t *testing.T) {
	intStats, handled, err := TryTypedNumericStats(NewI64Range(1, 2, 4))
	if err != nil {
		t.Fatalf("TryTypedNumericStats integer returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedNumericStats did not handle integer range")
	}
	if intStats.Sum != int64(16) || intStats.Min != int64(1) || intStats.Max != int64(7) || intStats.Count != 4 || !intStats.HasValue {
		t.Fatalf("integer stats = %#v, want sum=16 min=1 max=7 count=4 has=true", intStats)
	}

	floatStats, handled, err := TryTypedNumericStats(NewF64([]float64{2.5, -1.5, 4}))
	if err != nil {
		t.Fatalf("TryTypedNumericStats float returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedNumericStats did not handle float column")
	}
	if floatStats.Sum != 5.0 || floatStats.Min != -1.5 || floatStats.Max != 4.0 || floatStats.Count != 3 || !floatStats.HasValue {
		t.Fatalf("float stats = %#v, want sum=5 min=-1.5 max=4 count=3 has=true", floatStats)
	}

	if _, handled, err := TryTypedNumericStats(NewSymbols([]string{"a", "b"})); err != nil || handled {
		t.Fatalf("symbol stats handled=%v err=%v, want unhandled nil error", handled, err)
	}
}

func TestNumericAnalyticsStatsWindows(t *testing.T) {
	values := NewF64([]float64{1, 2, 3})
	variance, handled, err := NumericArrayVariance(values, true)
	if err != nil || !handled || variance != 1.0 {
		t.Fatalf("NumericArrayVariance = %#v,%v,%v, want 1,true,nil", variance, handled, err)
	}
	stddev, handled, err := NumericArrayStdDev(values, true)
	if err != nil || !handled || stddev != 1.0 {
		t.Fatalf("NumericArrayStdDev = %#v,%v,%v, want 1,true,nil", stddev, handled, err)
	}
	wsum, handled, err := NumericWeightedSum(NewI64([]int64{1, 2, 3}), NewI64([]int64{10, 20, 30}))
	if err != nil || !handled || wsum != 140.0 {
		t.Fatalf("NumericWeightedSum = %#v,%v,%v, want 140,true,nil", wsum, handled, err)
	}
	wsum, handled, err = NumericWeightedSum(int64(2), NewI64([]int64{10, 20, 30}))
	if err != nil || !handled || wsum != 120.0 {
		t.Fatalf("NumericWeightedSum scalar broadcast = %#v,%v,%v, want 120,true,nil", wsum, handled, err)
	}
	wsum, handled, err = NumericWeightedSum(NewColumn("w", []any{int64(1), NullValue, int64(3)}).Data, NewI64([]int64{10, 20, 30}))
	if err != nil || !handled || wsum != 100.0 {
		t.Fatalf("NumericWeightedSum nullable = %#v,%v,%v, want 100,true,nil", wsum, handled, err)
	}
	if _, handled, err := NumericWeightedSum(NewString([]string{"a"}), NewI64([]int64{1})); err != nil || handled {
		t.Fatalf("NumericWeightedSum string handled=%v err=%v, want false,nil", handled, err)
	}
	cov, handled, err := NumericArrayCovariance(values, values, false)
	if err != nil || !handled || cov != float64(2)/3 {
		t.Fatalf("NumericArrayCovariance = %#v,%v,%v, want 2/3,true,nil", cov, handled, err)
	}
	cor, handled, err := NumericArrayCorrelation(values, values)
	if err != nil || !handled || cor != 1.0 {
		t.Fatalf("NumericArrayCorrelation = %#v,%v,%v, want 1,true,nil", cor, handled, err)
	}
	mdev, handled, err := NumericMovingStdDev(values, 2, false)
	if err != nil || !handled {
		t.Fatalf("NumericMovingStdDev returned %#v,%v,%v", mdev, handled, err)
	}
	if got, want := mdev.Values(), []any{0.0, 0.5, 0.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericMovingStdDev values = %#v, want %#v", got, want)
	}
	if mdev.Kind() != KindF64 {
		t.Fatalf("NumericMovingStdDev kind = %s, want %s", mdev.Kind(), KindF64)
	}
	ema, handled, err := NumericExponentialMovingAverage(values, 0.5)
	if err != nil || !handled {
		t.Fatalf("NumericExponentialMovingAverage returned %#v,%v,%v", ema, handled, err)
	}
	if got, want := ema.Values(), []any{1.0, 1.5, 2.25}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericExponentialMovingAverage values = %#v, want %#v", got, want)
	}
	if ema.Kind() != KindF64 {
		t.Fatalf("NumericExponentialMovingAverage kind = %s, want %s", ema.Kind(), KindF64)
	}

	nullable := NewColumn("x", []any{1.0, NullValue, 3.0}).Data
	nullableMdev, handled, err := NumericMovingStdDev(nullable, 2, false)
	if err != nil || !handled {
		t.Fatalf("NumericMovingStdDev nullable returned %#v,%v,%v", nullableMdev, handled, err)
	}
	if got, want := nullableMdev.Values(), []any{0.0, 0.0, 0.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericMovingStdDev nullable values = %#v, want %#v", got, want)
	}
	nullableEMA, handled, err := NumericExponentialMovingAverage(nullable, 0.5)
	if err != nil || !handled {
		t.Fatalf("NumericExponentialMovingAverage nullable returned %#v,%v,%v", nullableEMA, handled, err)
	}
	if got, want := nullableEMA.Values(), []any{1.0, NullValue, 2.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericExponentialMovingAverage nullable values = %#v, want %#v", got, want)
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

func TestTryTypedQNumericUnaryCompareIndexes(t *testing.T) {
	sum, handled, err := TryTypedQNumericUnarySum(NumericUnarySqrt, NewI64([]int64{1, 4, 9, 16}))
	if err != nil || !handled || sum != float64(10) {
		t.Fatalf("TryTypedQNumericUnarySum sqrt = %#v,%v,%v; want 10,true,nil", sum, handled, err)
	}

	indexes, handled, err := TryTypedQNumericUnaryCompareIndexes(NumericUnarySqrt, NewI64([]int64{1, 4, 9, 16}), OpGT, float64(2))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryCompareIndexes returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedQNumericUnaryCompareIndexes did not handle typed numeric input")
	}
	if got, want := indexes.Values(), []any{int64(2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQNumericUnaryCompareIndexes = %v, want %v", got, want)
	}

	if _, handled, err := TryTypedQNumericUnaryCompareIndexes(NumericUnarySqrt, NewString([]string{"a"}), OpGT, float64(2)); err != nil || handled {
		t.Fatalf("TryTypedQNumericUnaryCompareIndexes string = handled %v err %v; want false,nil", handled, err)
	}
}

func TestTryTypedWithinIndexesAndStatsTiledTime(t *testing.T) {
	times, err := TakeRepeat(NewTime([]Time{1, 2, 3, 4}), 10)
	if err != nil {
		t.Fatalf("TakeRepeat time returned error: %v", err)
	}
	indexes, handled, err := TryTypedWithinIndexesI64(times, Time(2), Time(3), true)
	if err != nil {
		t.Fatalf("TryTypedWithinIndexesI64 returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedWithinIndexesI64 did not handle tiled time")
	}
	if got, want := indexes.Values(), []any{int64(1), int64(2), int64(5), int64(6), int64(9)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("within indexes = %v, want %v", got, want)
	}
	count, sum, handled, err := TryTypedWithinIndexStatsI64(times, Time(2), Time(3), true)
	if err != nil {
		t.Fatalf("TryTypedWithinIndexStatsI64 returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedWithinIndexStatsI64 did not handle tiled time")
	}
	if count != 5 || sum != 23 {
		t.Fatalf("within stats count=%d sum=%d, want count=5 sum=23", count, sum)
	}
	count, handled, err = TryTypedWithinCount(times, Time(2), Time(3), true)
	if err != nil {
		t.Fatalf("TryTypedWithinCount returned error: %v", err)
	}
	if !handled || count != 5 {
		t.Fatalf("within count handled=%v count=%d, want handled=true count=5", handled, count)
	}
}

func TestTryTypedWithinIndexStatsBucketViews(t *testing.T) {
	intBuckets, err := BucketFloor(NewI64Range(0, 1, 20), int64(5))
	if err != nil {
		t.Fatalf("BucketFloor int range returned error: %v", err)
	}
	count, sum, handled, err := TryTypedWithinIndexStatsI64(intBuckets, int64(5), int64(10), true)
	if err != nil {
		t.Fatalf("TryTypedWithinIndexStatsI64 int buckets returned error: %v", err)
	}
	if !handled || count != 10 || sum != 95 {
		t.Fatalf("int bucket within stats count=%d sum=%d handled=%v, want 10,95,true", count, sum, handled)
	}
	count, handled, err = TryTypedWithinCount(intBuckets, int64(5), int64(10), true)
	if err != nil {
		t.Fatalf("TryTypedWithinCount int buckets returned error: %v", err)
	}
	if !handled || count != 10 {
		t.Fatalf("int bucket within count=%d handled=%v, want 10,true", count, handled)
	}

	floatBuckets, err := BucketFloor(NewF64([]float64{0.1, 0.5, 0.9, 1.0, 1.49, 1.5}), 0.5)
	if err != nil {
		t.Fatalf("BucketFloor float array returned error: %v", err)
	}
	count, sum, handled, err = TryTypedWithinIndexStatsI64(floatBuckets, 0.5, 1.0, true)
	if err != nil {
		t.Fatalf("TryTypedWithinIndexStatsI64 float buckets returned error: %v", err)
	}
	if !handled || count != 4 || sum != 10 {
		t.Fatalf("float bucket within stats count=%d sum=%d handled=%v, want 4,10,true", count, sum, handled)
	}
}

func TestTryTypedWithinIndexesNullableTemporal(t *testing.T) {
	dates := newNullableArray(KindDate, []any{DateFromDays(10), DateFromDays(11), NullValue, DateFromDays(12), DateFromDays(13)})
	indexes, handled, err := TryTypedWithinIndexesI64(dates, DateFromDays(11), DateFromDays(12), true)
	if err != nil || !handled {
		t.Fatalf("TryTypedWithinIndexesI64 nullable dates handled=%v err=%v; want true,nil", handled, err)
	}
	if got, want := indexes.Values(), []any{int64(1), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nullable date within indexes = %v, want %v", got, want)
	}
	count, sum, handled, err := TryTypedWithinIndexStatsI64(dates, DateFromDays(11), DateFromDays(12), true)
	if err != nil || !handled || count != 2 || sum != 4 {
		t.Fatalf("nullable date within stats = %d,%d,%v,%v; want 2,4,true,nil", count, sum, handled, err)
	}
}

func TestTryTypedCastIntegerArrays(t *testing.T) {
	shorts, handled, err := TryTypedCast(KindI16, NewI64Range(0, 2, 4))
	if err != nil {
		t.Fatalf("TryTypedCast i16 range returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCast i16 range did not handle integer array")
	}
	if got, want := shorts.Values(), []any{int16(0), int16(2), int16(4), int16(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedCast i16 range values = %#v, want %#v", got, want)
	}

	floats, handled, err := TryTypedCast(KindF64, NewI32([]int32{1, 2, 3}))
	if err != nil {
		t.Fatalf("TryTypedCast f64 i32 returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCast f64 i32 did not handle integer array")
	}
	if got, want := floats.Values(), []any{1.0, 2.0, 3.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedCast f64 i32 values = %#v, want %#v", got, want)
	}

	longs, handled, err := TryTypedCast(KindI64, NewF64([]float64{1.9, -2.9, 3.0}))
	if err != nil {
		t.Fatalf("TryTypedCast i64 f64 returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCast i64 f64 did not handle numeric array")
	}
	if got, want := longs.Values(), []any{int64(1), int64(-2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedCast i64 f64 values = %#v, want %#v", got, want)
	}

	ints, handled, err := TryTypedCast(KindI32, NewF32([]float32{1.2, -3.7}))
	if err != nil {
		t.Fatalf("TryTypedCast i32 f32 returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCast i32 f32 did not handle numeric array")
	}
	if got, want := ints.Values(), []any{int32(1), int32(-3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedCast i32 f32 values = %#v, want %#v", got, want)
	}

	floatRange, handled, err := TryTypedCast(KindF64, NewI64Range(0, 2, 4))
	if err != nil {
		t.Fatalf("TryTypedCast f64 range returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCast f64 range did not handle integer range")
	}
	if _, ok := floatRange.(f64RangeArray); !ok {
		t.Fatalf("TryTypedCast f64 range type = %T, want f64RangeArray", floatRange)
	}
	if got, want := floatRange.Values(), []any{0.0, 2.0, 4.0, 6.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedCast f64 range values = %#v, want %#v", got, want)
	}

	if _, handled, err := TryTypedCast(KindI16, NewI64([]int64{32768})); !handled || err == nil {
		t.Fatalf("TryTypedCast i16 overflow handled=%v err=%v, want handled error", handled, err)
	}
	if _, handled, err := TryTypedCast(KindI64, NewF64([]float64{math.Inf(1)})); handled || err != nil {
		t.Fatalf("TryTypedCast i64 inf handled=%v err=%v, want unsupported nil error", handled, err)
	}
}

func TestTypedRunningAndMovingIntegerKernels(t *testing.T) {
	mins, handled, err := TryTypedRunningMinMax(NewI64([]int64{4, 3, 5, 2}), false)
	if err != nil {
		t.Fatalf("TryTypedRunningMinMax mins returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedRunningMinMax mins did not handle integer array")
	}
	if got, want := mins.Values(), []any{int64(4), int64(3), int64(3), int64(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("running mins values = %#v, want %#v", got, want)
	}

	avgs, handled, err := TryTypedAvgs(NewI64Range(1, 1, 4))
	if err != nil {
		t.Fatalf("TryTypedAvgs range returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedAvgs range did not handle integer array")
	}
	if got, want := avgs.Values(), []any{1.0, 1.5, 2.0, 2.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("running avgs values = %#v, want %#v", got, want)
	}

	counts, handled, err := TryTypedMCount(NewI64Range(1, 1, 5), 3)
	if err != nil {
		t.Fatalf("TryTypedMCount returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMCount did not handle integer array")
	}
	if got, want := counts.Values(), []any{int64(1), int64(2), int64(3), int64(3), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mcount values = %#v, want %#v", got, want)
	}

	countSum, handled, err := TryTypedMCountSum(NewI64Range(1, 1, 5), 3)
	if err != nil {
		t.Fatalf("TryTypedMCountSum returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMCountSum did not handle integer array")
	}
	if want := int64(12); countSum != want {
		t.Fatalf("mcount sum = %d, want %d", countSum, want)
	}

	if _, handled, err := TryTypedMCount(NewColumn("x", []any{int64(10), nil, int64(30)}).Data, 2); err != nil || handled {
		t.Fatalf("TryTypedMCount nullable handled=%v err=%v, want fallback", handled, err)
	}

	mmax, handled, err := TryTypedMovingMinMax(NewI64([]int64{3, 1, 4, 2}), 2, true)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMax returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMovingMinMax did not handle integer array")
	}
	if got, want := mmax.Values(), []any{int64(3), int64(3), int64(4), int64(4)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mmax values = %#v, want %#v", got, want)
	}

	maxSum, handled, err := TryTypedMovingMinMaxSum(NewI64([]int64{3, 1, 4, 2}), 2, true)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMaxSum max returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMovingMinMaxSum max did not handle integer array")
	}
	if want := int64(14); maxSum != want {
		t.Fatalf("mmax sum = %d, want %d", maxSum, want)
	}

	mmin, handled, err := TryTypedMovingMinMax(NewI64([]int64{5, 3, 4, 2, 6, 1}), 3, false)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMax min returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMovingMinMax min did not handle integer array")
	}
	if got, want := mmin.Values(), []any{int64(5), int64(3), int64(3), int64(2), int64(2), int64(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mmin values = %#v, want %#v", got, want)
	}

	minSum, handled, err := TryTypedMovingMinMaxSum(NewI64([]int64{5, 3, 4, 2, 6, 1}), 3, false)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMaxSum min returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMovingMinMaxSum min did not handle integer array")
	}
	if want := int64(16); minSum != want {
		t.Fatalf("mmin sum = %d, want %d", minSum, want)
	}

	rangeMinSum, handled, err := TryTypedMovingMinMaxSum(NewI64Range(1, 1, 5), 3, false)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMaxSum range min returned error: %v", err)
	}
	if !handled || rangeMinSum != 8 {
		t.Fatalf("TryTypedMovingMinMaxSum range min = %v, %v; want 8, true", rangeMinSum, handled)
	}
	rangeMaxSum, handled, err := TryTypedMovingMinMaxSum(NewI64Range(1, 1, 5), 3, true)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMaxSum range max returned error: %v", err)
	}
	if !handled || rangeMaxSum != 15 {
		t.Fatalf("TryTypedMovingMinMaxSum range max = %v, %v; want 15, true", rangeMaxSum, handled)
	}
	descRangeMaxSum, handled, err := TryTypedMovingMinMaxSum(NewI64Range(5, -1, 5), 3, true)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMaxSum descending range max returned error: %v", err)
	}
	if !handled || descRangeMaxSum != 22 {
		t.Fatalf("TryTypedMovingMinMaxSum descending range max = %v, %v; want 22, true", descRangeMaxSum, handled)
	}
	wideRangeMinSum, handled, err := TryTypedMovingMinMaxSum(NewI64Range(1, 1, 5), 8, false)
	if err != nil {
		t.Fatalf("TryTypedMovingMinMaxSum wide range min returned error: %v", err)
	}
	if !handled || wideRangeMinSum != 5 {
		t.Fatalf("TryTypedMovingMinMaxSum wide range min = %v, %v; want 5, true", wideRangeMinSum, handled)
	}

	movingSum, handled, err := TryTypedMovingNumericSumSum(NewI64([]int64{10, 20, 30, 40}), 3, false)
	if err != nil {
		t.Fatalf("TryTypedMovingNumericSumSum msum returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMovingNumericSumSum msum did not handle integer array")
	}
	if want := int64(190); movingSum != want {
		t.Fatalf("msum sum = %#v, want %d", movingSum, want)
	}

	movingAvg, handled, err := TryTypedMovingNumericSumSum(NewI64([]int64{10, 20, 30, 40}), 3, true)
	if err != nil {
		t.Fatalf("TryTypedMovingNumericSumSum mavg returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMovingNumericSumSum mavg did not handle integer array")
	}
	if want := 75.0; movingAvg != want {
		t.Fatalf("mavg sum = %#v, want %.1f", movingAvg, want)
	}

	rangeMovingSum, handled, err := TryTypedMovingNumericSumSum(NewI64Range(1, 1, 5), 3, false)
	if err != nil {
		t.Fatalf("TryTypedMovingNumericSumSum range msum returned error: %v", err)
	}
	if !handled || rangeMovingSum != int64(31) {
		t.Fatalf("range msum sum = %#v, %v; want 31, true", rangeMovingSum, handled)
	}
	rangeMovingAvg, handled, err := TryTypedMovingNumericSumSum(NewI64Range(1, 1, 5), 3, true)
	if err != nil {
		t.Fatalf("TryTypedMovingNumericSumSum range mavg returned error: %v", err)
	}
	if !handled || rangeMovingAvg != 11.5 {
		t.Fatalf("range mavg sum = %#v, %v; want 11.5, true", rangeMovingAvg, handled)
	}
	descRangeMovingAvg, handled, err := TryTypedMovingNumericSumSum(NewI64Range(5, -1, 5), 8, true)
	if err != nil {
		t.Fatalf("TryTypedMovingNumericSumSum descending wide range mavg returned error: %v", err)
	}
	if !handled || descRangeMovingAvg != 20.0 {
		t.Fatalf("descending wide range mavg sum = %#v, %v; want 20.0, true", descRangeMovingAvg, handled)
	}

	floatMovingSum, handled, err := TryTypedMovingNumericSumSum(NewF64([]float64{1.5, 2.5, 3.5}), 2, false)
	if err != nil {
		t.Fatalf("TryTypedMovingNumericSumSum float returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedMovingNumericSumSum float did not handle numeric array")
	}
	if want := 11.5; floatMovingSum != want {
		t.Fatalf("float msum sum = %#v, want %.1f", floatMovingSum, want)
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

func TestTypedBinTemporalAndSymbolVectors(t *testing.T) {
	dateDomain := WithArrayAttribute(NewDate([]Date{
		DateFromDays(20610),
		DateFromDays(20611),
		DateFromDays(20611),
		DateFromDays(20613),
	}), ArrayAttributeSorted)
	dateQueries := NewDate([]Date{
		DateFromDays(20609),
		DateFromDays(20610),
		DateFromDays(20611),
		DateFromDays(20612),
		DateFromDays(20614),
	})
	dateIndexes, ok, err := typedKernels.Bin(dateDomain, dateQueries)
	if err != nil {
		t.Fatalf("date Bin vector returned error: %v", err)
	}
	if !ok {
		t.Fatal("date Bin vector did not match typed kernel")
	}
	if got, want := dateIndexes.(Array).Values(), []any{int64(-1), int64(0), int64(2), int64(2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("date Bin vector values = %v, want %v", got, want)
	}

	timeDomain := WithArrayAttribute(NewTime([]Time{Time(34200000), Time(34260000), Time(34320000)}), ArrayAttributeSorted)
	timeIndex, ok, err := typedKernels.Bin(timeDomain, Time(34290000))
	if err != nil {
		t.Fatalf("time Bin scalar returned error: %v", err)
	}
	if !ok || timeIndex != int64(1) {
		t.Fatalf("time Bin scalar = %v, %v; want 1, true", timeIndex, ok)
	}

	symbolDomain := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "MSFT", "NVDA"}), ArrayAttributeSorted)
	symbolIndexes, ok, err := typedKernels.Bin(symbolDomain, NewSymbols([]string{"A", "AAPL", "MSFT", "TSLA"}))
	if err != nil {
		t.Fatalf("symbol Bin vector returned error: %v", err)
	}
	if !ok {
		t.Fatal("symbol Bin vector did not match typed kernel")
	}
	if got, want := symbolIndexes.(Array).Values(), []any{int64(-1), int64(0), int64(2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("symbol Bin vector values = %v, want %v", got, want)
	}
}

func TestTryTypedBinSumAvoidsMaterializedResult(t *testing.T) {
	rangeDomain := NewI64Range(0, 10, 8)
	sum, ok, err := TryTypedBinSum(rangeDomain, NewI64Range(0, 1, 80))
	if err != nil {
		t.Fatalf("TryTypedBinSum range returned error: %v", err)
	}
	if !ok || sum != int64(280) {
		t.Fatalf("TryTypedBinSum range = %v, %v; want 280, true", sum, ok)
	}

	intDomain := WithArrayAttribute(NewI64([]int64{10, 20, 20, 40}), ArrayAttributeSorted)
	sum, ok, err = TryTypedBinSum(intDomain, NewColumn("q", []any{int64(5), int64(10), nil, int64(30), int64(50)}).Data)
	if err != nil {
		t.Fatalf("TryTypedBinSum nullable query returned error: %v", err)
	}
	if !ok || sum != int64(3) {
		t.Fatalf("TryTypedBinSum nullable query = %v, %v; want 3, true", sum, ok)
	}

	dateDomain := WithArrayAttribute(NewDate([]Date{
		DateFromDays(20610),
		DateFromDays(20611),
		DateFromDays(20611),
		DateFromDays(20613),
	}), ArrayAttributeSorted)
	dateQueries := NewDate([]Date{
		DateFromDays(20609),
		DateFromDays(20610),
		DateFromDays(20611),
		DateFromDays(20612),
		DateFromDays(20614),
	})
	sum, ok, err = TryTypedBinSum(dateDomain, dateQueries)
	if err != nil {
		t.Fatalf("TryTypedBinSum date returned error: %v", err)
	}
	if !ok || sum != int64(6) {
		t.Fatalf("TryTypedBinSum date = %v, %v; want 6, true", sum, ok)
	}

	symbolDomain := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "MSFT", "NVDA"}), ArrayAttributeSorted)
	sum, ok, err = TryTypedBinSum(symbolDomain, NewSymbols([]string{"A", "AAPL", "MSFT", "TSLA"}))
	if err != nil {
		t.Fatalf("TryTypedBinSum symbol returned error: %v", err)
	}
	if !ok || sum != int64(4) {
		t.Fatalf("TryTypedBinSum symbol = %v, %v; want 4, true", sum, ok)
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
	if got, want := QueryKernelPlanPipelineShape(changedLiteral), QueryKernelPlanPipelineShape(base); got != want {
		t.Fatalf("QueryKernelPlanPipelineShape changed with literal value: got %q, want %q", got, want)
	}
	if got, want := QueryKernelPlanPipelineShape(base), "scan=frame|where=compare_mask(compare_mask:column_literal)|filter=index|project=typed_expr(typed_binary:1)"; got != want {
		t.Fatalf("QueryKernelPlanPipelineShape = %q, want %q", got, want)
	}
	descriptor := QueryKernelPlanPipelineDescriptor(base)
	if descriptor.WhereFamily != "compare_mask" || descriptor.WhereOps != "compare_mask:column_literal" ||
		descriptor.ProjectionFamily != "typed_expr" || descriptor.ProjectionOps != "typed_binary:1" {
		t.Fatalf("QueryKernelPlanPipelineDescriptor = %+v, want compare-mask typed-expression projection", descriptor)
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
		if got, want := kernel.PipelineShape(), QueryKernelPlanPipelineShape(base); got != want {
			t.Fatalf("%s kernel pipeline shape = %q, want %q", name, got, want)
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
	wantPipeline := "scan=frame|group=key_columns(column_load:1)|project=where_select(where_select:1)|order=post_project:1|distinct=rows|limit=bounded"
	if got := QueryKernelPlanPipelineShape(plan); got != wantPipeline {
		t.Fatalf("QueryKernelPlanPipelineShape = %q, want %q", got, wantPipeline)
	}
	if got := kernel.PipelineShape(); got != wantPipeline {
		t.Fatalf("compiled kernel pipeline shape = %q, want %q", got, wantPipeline)
	}
	descriptor := QueryKernelPlanPipelineDescriptor(plan)
	if descriptor.GroupFamily != "key_columns" || descriptor.GroupOps != "column_load:1" ||
		descriptor.ProjectionFamily != "where_select" || descriptor.ProjectionOps != "where_select:1" {
		t.Fatalf("QueryKernelPlanPipelineDescriptor = %+v, want key-column where-select shape", descriptor)
	}
	var nilKernel *QueryKernel
	if got := nilKernel.Shape(); got != "" {
		t.Fatalf("nil kernel shape = %q, want empty", got)
	}
	if got := nilKernel.PipelineShape(); got != "" {
		t.Fatalf("nil kernel pipeline shape = %q, want empty", got)
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
	indexArray, handled, err := TryTypedWithinIndexesI64(NewI64Range(0, 1, 10), int64(3), int64(6), true)
	if err != nil || !handled {
		t.Fatalf("TryTypedWithinIndexesI64 range = %T,%v,%v; want handled", indexArray, handled, err)
	}
	if got, want := indexArray.Values(), []any{int64(3), int64(4), int64(5), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedWithinIndexesI64 range values = %v, want %v", got, want)
	}
	count, sum, handled, err := TryTypedWithinIndexStatsI64(NewI64Range(0, 1, 10), int64(3), int64(6), true)
	if err != nil || !handled || count != 4 || sum != 18 {
		t.Fatalf("TryTypedWithinIndexStatsI64 range = count %d sum %d handled %v err %v; want 4,18,true,nil", count, sum, handled, err)
	}
	reversedDatesAny, handled, err := Reverse(NewDate([]Date{Date(1), Date(2), Date(3), Date(4)}))
	if err != nil || !handled {
		t.Fatalf("Reverse date column = %T,%v,%v; want handled", reversedDatesAny, handled, err)
	}
	reversedDates := reversedDatesAny.(Array)
	indexArray, handled, err = TryTypedWithinIndexesI64(reversedDates, Date(2), Date(3), true)
	if err != nil || !handled {
		t.Fatalf("TryTypedWithinIndexesI64 indexed date = %T,%v,%v; want handled", indexArray, handled, err)
	}
	if got, want := indexArray.Values(), []any{int64(1), int64(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedWithinIndexesI64 indexed date values = %v, want %v", got, want)
	}
	count, sum, handled, err = TryTypedWithinIndexStatsI64(reversedDates, Date(2), Date(3), true)
	if err != nil || !handled || count != 2 || sum != 3 {
		t.Fatalf("TryTypedWithinIndexStatsI64 indexed date = count %d sum %d handled %v err %v; want 2,3,true,nil", count, sum, handled, err)
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
	if count, ok, err := TryTypedNullCount(NewColumn("x", []any{1, nil, NullValue, 3}).Data); err != nil || !ok || count != 2 {
		t.Fatalf("typed null count nullable = %d, %v, %v; want 2, true, nil", count, ok, err)
	}
	if count, ok, err := TryTypedNullCount(NewI64Range(0, 1, 4)); err != nil || !ok || count != 0 {
		t.Fatalf("typed null count dense = %d, %v, %v; want 0, true, nil", count, ok, err)
	}
}

func TestTypedNumericSumByI64IndexesConsumesLazyIntegerViews(t *testing.T) {
	base := NewI64Range(0, 1, 16)
	scaled, handled, err := TryTypedIntegerDyadic(OpMul, base, int64(3))
	if err != nil || !handled {
		t.Fatalf("typed multiply handled=%v err=%v", handled, err)
	}
	affine, handled, err := TryTypedIntegerDyadic(OpAdd, scaled, int64(7))
	if err != nil || !handled {
		t.Fatalf("typed add handled=%v err=%v", handled, err)
	}

	got, handled, err := TryTypedNumericSumByI64Indexes(affine.(Array), NewI64Range(2, 1, 5))
	if err != nil || !handled {
		t.Fatalf("typed indexed sum handled=%v err=%v", handled, err)
	}
	if got != int64(95) {
		t.Fatalf("typed indexed sum = %v, want 95", got)
	}
}

func TestTypedNumericSumCountWhereCompareConsumesLazyIntegerPredicate(t *testing.T) {
	base := NewI64Range(0, 1, 16)
	scaled, handled, err := TryTypedIntegerDyadic(OpMul, base, int64(3))
	if err != nil || !handled {
		t.Fatalf("typed multiply handled=%v err=%v", handled, err)
	}
	affine, handled, err := TryTypedIntegerDyadic(OpAdd, scaled, int64(7))
	if err != nil || !handled {
		t.Fatalf("typed add handled=%v err=%v", handled, err)
	}
	predicate, handled, err := TryTypedIntegerDyadic(OpMod, affine, int64(5))
	if err != nil || !handled {
		t.Fatalf("typed modulo handled=%v err=%v", handled, err)
	}

	sum, count, handled, err := TryTypedNumericSumCountWhereCompare(affine.(Array), predicate.(Array), OpGT, int64(1))
	if err != nil || !handled {
		t.Fatalf("typed where sum-count handled=%v err=%v", handled, err)
	}
	if sum != int64(304) || count != 10 {
		t.Fatalf("typed where sum-count = sum %v count %d, want 304 10", sum, count)
	}
}

func TestTypedNumericSumCountWhereCompareConsumesLazyFloatPredicate(t *testing.T) {
	base := NewI64Range(0, 1, 16)
	scaled, handled, err := typedKernels.Dyadic(OpMul, base, float64(2))
	if err != nil || !handled {
		t.Fatalf("typed multiply handled=%v err=%v", handled, err)
	}
	affine, handled, err := typedKernels.Dyadic(OpAdd, scaled, float64(7))
	if err != nil || !handled {
		t.Fatalf("typed add handled=%v err=%v", handled, err)
	}
	predicate, handled, err := typedKernels.Dyadic(OpMod, affine, float64(5))
	if err != nil || !handled {
		t.Fatalf("typed modulo handled=%v err=%v", handled, err)
	}

	gatherSum, handled, err := TryTypedNumericSumByI64Indexes(affine.(Array), NewI64Range(0, 1, 16))
	if err != nil || !handled {
		t.Fatalf("typed float indexed sum handled=%v err=%v", handled, err)
	}
	if gatherSum != float64(352) {
		t.Fatalf("typed float indexed sum = %v, want 352", gatherSum)
	}

	sum, count, handled, err := TryTypedNumericSumCountWhereCompare(affine.(Array), predicate.(Array), OpGT, int64(1))
	if err != nil || !handled {
		t.Fatalf("typed float where sum-count handled=%v err=%v", handled, err)
	}
	if sum != float64(214) || count != 10 {
		t.Fatalf("typed float where sum-count = sum %v count %d, want 214 10", sum, count)
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

	sine, ok, err := typedKernels.NumericUnary(NumericUnarySin, NewF64([]float64{0, math.Pi / 2}))
	if err != nil {
		t.Fatalf("sin NumericUnary returned error: %v", err)
	}
	if !ok {
		t.Fatal("sin NumericUnary did not match numeric column")
	}
	gotSin := sine.Values()
	if len(gotSin) != 2 || gotSin[0].(float64) != 0 || math.Abs(gotSin[1].(float64)-1) > 1e-12 {
		t.Fatalf("sin NumericUnary values = %v, want [0 1]", gotSin)
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

	xexp, ok, err := ApplyNumericDyadicFloat(NumericDyadicXExp, NewI64([]int64{2, 3}), int64(3))
	if err != nil || !ok {
		t.Fatalf("ApplyNumericDyadicFloat xexp = %#v,%v,%v; want handled nil error", xexp, ok, err)
	}
	if got, want := xexp.(Array).Values(), []any{8.0, 27.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyNumericDyadicFloat xexp values = %v, want %v", got, want)
	}

	xlog, ok, err := ApplyNumericDyadicFloat(NumericDyadicXLog, int64(2), NewColumn("x", []any{8, NullValue}).Data)
	if err != nil || !ok {
		t.Fatalf("ApplyNumericDyadicFloat xlog = %#v,%v,%v; want handled nil error", xlog, ok, err)
	}
	if got, want := xlog.(Array).Values(), []any{3.0, NullValue}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyNumericDyadicFloat xlog values = %v, want %v", got, want)
	}

	typedXexp, ok, err := TryTypedQNumericDyadicFloat(NumericDyadicXExp, NewI64([]int64{2, 3}), NewI64([]int64{3, 2}))
	if err != nil || !ok {
		t.Fatalf("TryTypedQNumericDyadicFloat xexp array-array = %#v,%v,%v; want handled nil error", typedXexp, ok, err)
	}
	if _, lazy := typedXexp.(f64NumericDyadicArray); !lazy {
		t.Fatalf("TryTypedQNumericDyadicFloat xexp returned %T, want lazy f64NumericDyadicArray", typedXexp)
	}
	lazyProducer, err := newF64NumericProducer(typedXexp, typedXexp.Len())
	if err != nil {
		t.Fatalf("newF64NumericProducer lazy xexp returned error: %v", err)
	}
	if _, ok := lazyProducer.(f64DyadicProducer); !ok {
		t.Fatalf("newF64NumericProducer lazy xexp = %T, want f64DyadicProducer", lazyProducer)
	}
	if lazyArray, ok := typedXexp.(f64NumericDyadicArray); !ok || lazyArray.bound.producer.apply == nil {
		t.Fatalf("TryTypedQNumericDyadicFloat xexp bound = %#v,%v; want pre-bound producer", lazyArray.bound, ok)
	}
	if got, want := typedXexp.Values(), []any{8.0, 9.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQNumericDyadicFloat xexp values = %v, want %v", got, want)
	}

	typedNullable, ok, err := TryTypedQNumericDyadicFloat(NumericDyadicXLog, int64(2), NewColumn("x", []any{8, NullValue, 16}).Data)
	if err != nil || !ok {
		t.Fatalf("TryTypedQNumericDyadicFloat xlog nullable = %#v,%v,%v; want handled nil error", typedNullable, ok, err)
	}
	if got, want := typedNullable.Values(), []any{3.0, NullValue, 4.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQNumericDyadicFloat xlog nullable values = %v, want %v", got, want)
	}

	xexpSum, ok, err := TryTypedQNumericDyadicFloatSum(NumericDyadicXExp, int64(2), NewI64([]int64{0, 1, 2, 3}))
	if err != nil || !ok {
		t.Fatalf("TryTypedQNumericDyadicFloatSum xexp = %#v,%v,%v; want handled nil error", xexpSum, ok, err)
	}
	if got, want := xexpSum.(float64), 15.0; got != want {
		t.Fatalf("TryTypedQNumericDyadicFloatSum xexp = %v, want %v", got, want)
	}
	lazyPow, ok, err := TryTypedQNumericDyadicFloat(NumericDyadicXExp, int64(2), NewI64([]int64{0, 1, 2, 3, 0, 1, 2, 3}))
	if err != nil || !ok {
		t.Fatalf("TryTypedQNumericDyadicFloat repeated xexp = %#v,%v,%v; want handled nil error", lazyPow, ok, err)
	}
	if sum, ok, err := TryTypedRatiosSum(lazyPow); err != nil || !ok || sum != 13.125 {
		t.Fatalf("TryTypedRatiosSum lazy repeated xexp = %#v,%v,%v; want 13.125,true,nil", sum, ok, err)
	}

	xlogSum, ok, err := TryTypedQNumericDyadicFloatSum(NumericDyadicXLog, int64(2), NewColumn("x", []any{2, 4, 8, NullValue}).Data)
	if err != nil || !ok {
		t.Fatalf("TryTypedQNumericDyadicFloatSum xlog = %#v,%v,%v; want handled nil error", xlogSum, ok, err)
	}
	if got, want := xlogSum.(float64), 6.0; got != want {
		t.Fatalf("TryTypedQNumericDyadicFloatSum xlog = %v, want %v", got, want)
	}

	broadcastSum, ok, err := TryTypedQNumericDyadicFloatSum(NumericDyadicXExp, NewI64([]int64{2}), NewI64([]int64{1, 2, 3}))
	if err != nil || !ok {
		t.Fatalf("TryTypedQNumericDyadicFloatSum xexp singleton-array = %#v,%v,%v; want handled nil error", broadcastSum, ok, err)
	}
	if got, want := broadcastSum.(float64), 14.0; got != want {
		t.Fatalf("TryTypedQNumericDyadicFloatSum xexp singleton-array = %v, want %v", got, want)
	}

	lazyRatiosSum, ok, err := TryTypedRatiosSum(typedXexp)
	if err != nil || !ok {
		t.Fatalf("TryTypedRatiosSum lazy xexp = %#v,%v,%v; want handled nil error", lazyRatiosSum, ok, err)
	}
	if got, want := lazyRatiosSum.(float64), 8.0+9.0/8.0; got != want {
		t.Fatalf("TryTypedRatiosSum lazy xexp = %v, want %v", got, want)
	}
	qRatios, ok, err := TryTypedQRatios(NewColumn("x", []any{2, NullValue, 8, 16}).Data)
	if err != nil || !ok {
		t.Fatalf("TryTypedQRatios nullable = %#v,%v,%v; want handled nil error", qRatios, ok, err)
	}
	if _, lazy := qRatios.(qRatiosArray); !lazy {
		t.Fatalf("TryTypedQRatios returned %T, want qRatiosArray", qRatios)
	}
	if got, want := qRatios.Values(), []any{2.0, NullValue, 8.0, 2.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQRatios values = %v, want %v", got, want)
	}
	if got, want := qRatios.Gather([]int{3, 1, 0}).Values(), []any{2.0, NullValue, 2.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQRatios gather = %v, want %v", got, want)
	}
	qRatioSum, ok, err := TryTypedNumericSum(qRatios)
	if err != nil || !ok {
		t.Fatalf("TryTypedNumericSum(qRatios) = %#v,%v,%v; want handled nil error", qRatioSum, ok, err)
	}
	if got, want := qRatioSum.(float64), 12.0; got != want {
		t.Fatalf("TryTypedNumericSum(qRatios) = %v, want %v", got, want)
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

	qNeg, ok, err := TryTypedQNumericUnary(NumericUnaryNeg, NewI32([]int32{2, -3, 0}))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnary neg returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnary neg did not match i32 column")
	}
	if qNeg.Kind() != KindI64 {
		t.Fatalf("TryTypedQNumericUnary neg kind = %s, want i64", qNeg.Kind())
	}
	if got, want := qNeg.Values(), []any{int64(-2), int64(3), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQNumericUnary neg values = %v, want %v", got, want)
	}

	qFloor, ok, err := TryTypedQNumericUnary(NumericUnaryFloor, NewF64([]float64{-1.2, 1.9, 3}))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnary floor returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnary floor did not match f64 column")
	}
	if qFloor.Kind() != KindI64 {
		t.Fatalf("TryTypedQNumericUnary floor kind = %s, want i64", qFloor.Kind())
	}
	if got, want := qFloor.Values(), []any{int64(-2), int64(1), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQNumericUnary floor values = %v, want %v", got, want)
	}

	transcendentalSums := []struct {
		op     string
		values []float64
		fn     func(float64) float64
	}{
		{op: NumericUnarySqrt, values: []float64{1, 4, 9, 16}, fn: math.Sqrt},
		{op: NumericUnaryLog, values: []float64{1, math.E, math.E * math.E}, fn: math.Log},
		{op: NumericUnarySin, values: []float64{0, math.Pi / 2, math.Pi}, fn: math.Sin},
		{op: NumericUnaryCos, values: []float64{0, math.Pi / 2, math.Pi}, fn: math.Cos},
		{op: NumericUnaryTan, values: []float64{0, 0.25, 0.5}, fn: math.Tan},
		{op: NumericUnaryAsin, values: []float64{-0.5, 0, 0.5}, fn: math.Asin},
		{op: NumericUnaryAcos, values: []float64{-0.5, 0, 0.5}, fn: math.Acos},
		{op: NumericUnaryAtan, values: []float64{-1, 0, 1}, fn: math.Atan},
	}
	for _, tt := range transcendentalSums {
		t.Run("transcendental_sum_"+tt.op, func(t *testing.T) {
			base := NewF64(tt.values)
			indexed := indexedArray{source: base, indexes: NewI64([]int64{int64(len(tt.values) - 1), 0, 1}), len: 3}
			tiled := tiledArray{source: base, start: 1, len: len(tt.values)*2 + 1}
			for name, array := range map[string]Array{"base": base, "indexed": indexed, "tiled": tiled} {
				got, ok, err := TryTypedQNumericUnarySum(tt.op, array)
				if err != nil || !ok {
					t.Fatalf("%s TryTypedQNumericUnarySum(%s) = %#v,%v,%v; want handled", name, tt.op, got, ok, err)
				}
				var want float64
				for i := 0; i < array.Len(); i++ {
					value, ok, err := numericAt(array, i)
					if err != nil || !ok {
						t.Fatalf("%s numericAt row %d = %v,%v,%v", name, i, value, ok, err)
					}
					want += tt.fn(value)
				}
				if math.Abs(got.(float64)-want) > 1e-12 {
					t.Fatalf("%s TryTypedQNumericUnarySum(%s) = %.17g, want %.17g", name, tt.op, got.(float64), want)
				}
			}
		})
	}

	tiledInts, err := TakeRepeat(NewI64([]int64{-2, 0, 3}), 10)
	if err != nil {
		t.Fatalf("Take tiled ints returned error: %v", err)
	}
	tiledNegSum, ok, err := TryTypedQNumericUnarySum(NumericUnaryNeg, tiledInts)
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum tiled neg returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnarySum tiled neg did not match integer tiled array")
	}
	if got, want := tiledNegSum, int64(-1); got != want {
		t.Fatalf("TryTypedQNumericUnarySum tiled neg = %v (%T), want %v", got, got, want)
	}
	tiledSignumSum, ok, err := TryTypedQNumericUnarySum(NumericUnarySignum, tiledInts)
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum tiled signum returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnarySum tiled signum did not match integer tiled array")
	}
	if got, want := tiledSignumSum, int64(-1); got != want {
		t.Fatalf("TryTypedQNumericUnarySum tiled signum = %v (%T), want %v", got, got, want)
	}

	rotatedTiledInts := tiledArray{source: NewI64([]int64{1, 2, 3}), start: 1, len: 8}
	rotatedAbsSum, ok, err := TryTypedQNumericUnarySum(NumericUnaryAbs, rotatedTiledInts)
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum rotated tiled abs returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnarySum rotated tiled abs did not match integer tiled array")
	}
	if got, want := rotatedAbsSum, int64(17); got != want {
		t.Fatalf("TryTypedQNumericUnarySum rotated tiled abs = %v (%T), want %v", got, got, want)
	}

	rotatedRange := tiledArray{source: NewI64Range(10, 2, 5), start: 3, len: 12}
	rotatedRangeSum, ok, err := TryTypedNumericSum(rotatedRange)
	if err != nil {
		t.Fatalf("TryTypedNumericSum rotated tiled range returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedNumericSum rotated tiled range did not match integer tiled array")
	}
	if got, want := rotatedRangeSum, int64(174); got != want {
		t.Fatalf("TryTypedNumericSum rotated tiled range = %v (%T), want %v", got, got, want)
	}

	rotatedView, ok, err := TryTypedRotate(NewI64Range(0, 1, 1024), 17)
	if err != nil {
		t.Fatalf("TryTypedRotate range returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedRotate range was not handled")
	}
	reversedRotated, ok, err := Reverse(rotatedView)
	if err != nil {
		t.Fatalf("Reverse rotated range returned error: %v", err)
	}
	if !ok {
		t.Fatal("Reverse rotated range was not handled")
	}
	window, err := Slice(reversedRotated, 0, 128)
	if err != nil {
		t.Fatalf("Slice reversed rotated range returned error: %v", err)
	}
	composite, ok, err := TryTypedNumericSumFirstLast(window)
	if err != nil {
		t.Fatalf("TryTypedNumericSumFirstLast sequence view returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedNumericSumFirstLast sequence view was not handled")
	}
	if got, want := composite, int64(108513); got != want {
		t.Fatalf("TryTypedNumericSumFirstLast sequence view = %v (%T), want %v", got, got, want)
	}

	chainComposite, ok, err := TryTypedSequenceTransformChainNumericSumFirstLast([]SequenceTransformStep{
		{Transform: SequenceTransformRotate, Args: [2]int{17}, ArgCount: 1},
		{Transform: SequenceTransformReverse},
		{Transform: SequenceTransformSublist, Args: [2]int{128}, ArgCount: 1},
	}, NewI64Range(0, 1, 1024))
	if err != nil {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumFirstLast returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedSequenceTransformChainNumericSumFirstLast was not handled")
	}
	if got, want := chainComposite, int64(108513); got != want {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumFirstLast = %v (%T), want %v", got, got, want)
	}
	for _, tc := range []struct {
		name   string
		source Array
		steps  []SequenceTransformStep
	}{
		{
			name:   "non_zero_range_reverse_rotate_sublist",
			source: NewI64Range(10, 3, 37),
			steps: []SequenceTransformStep{
				{Transform: SequenceTransformReverse},
				{Transform: SequenceTransformRotate, Args: [2]int{-9}, ArgCount: 1},
				{Transform: SequenceTransformSublist, Args: [2]int{4, 19}, ArgCount: 2},
			},
		},
		{
			name:   "wrapped_rotate_sublist_count",
			source: NewI64Range(-20, 2, 23),
			steps: []SequenceTransformStep{
				{Transform: SequenceTransformRotate, Args: [2]int{17}, ArgCount: 1},
				{Transform: SequenceTransformSublist, Args: [2]int{15}, ArgCount: 1},
			},
		},
		{
			name:   "cycle_rotate_drop_take_sum_count",
			source: NewI64Range(0, 1, 8192),
			steps: []SequenceTransformStep{
				{Transform: SequenceTransformRotate, Args: [2]int{997}, ArgCount: 1},
				{Transform: SequenceTransformDrop, Args: [2]int{1024}, ArgCount: 1},
				{Transform: SequenceTransformSublist, Args: [2]int{9000}, ArgCount: 1},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantArray := tc.source
			for _, step := range tc.steps {
				var err error
				switch step.Transform {
				case SequenceTransformReverse:
					var handled bool
					wantArray, handled, err = Reverse(wantArray)
					if err != nil || !handled {
						t.Fatalf("Reverse oracle = %T,%v,%v", wantArray, handled, err)
					}
				case SequenceTransformRotate:
					var handled bool
					wantArray, handled, err = TryTypedRotate(wantArray, step.Args[0])
					if err != nil || !handled {
						t.Fatalf("TryTypedRotate oracle = %T,%v,%v", wantArray, handled, err)
					}
				case SequenceTransformDrop:
					if step.ArgCount != 1 {
						t.Fatalf("bad drop arg count %d", step.ArgCount)
					}
					n := step.Args[0]
					if n < 0 {
						n = -n
						wantArray, err = Slice(wantArray, 0, wantArray.Len()-n)
					} else {
						wantArray, err = Slice(wantArray, n, wantArray.Len()-n)
					}
					if err != nil {
						t.Fatalf("drop oracle: %v", err)
					}
				case SequenceTransformSublist:
					switch step.ArgCount {
					case 1:
						wantArray, err = TakeRepeat(wantArray, step.Args[0])
					case 2:
						start, count := boundedStartCount(wantArray.Len(), step.Args[0], step.Args[1])
						wantArray, err = Slice(wantArray, start, count)
					default:
						t.Fatalf("bad sublist arg count %d", step.ArgCount)
					}
					if err != nil {
						t.Fatalf("sublist oracle: %v", err)
					}
				default:
					t.Fatalf("unexpected transform %q", step.Transform)
				}
			}
			want, ok, err := TryTypedNumericSumFirstLast(wantArray)
			if err != nil || !ok {
				t.Fatalf("TryTypedNumericSumFirstLast oracle = %#v,%v,%v", want, ok, err)
			}
			got, ok, err := TryTypedSequenceTransformChainNumericSumFirstLast(tc.steps, tc.source)
			if err != nil || !ok {
				t.Fatalf("TryTypedSequenceTransformChainNumericSumFirstLast = %#v,%v,%v", got, ok, err)
			}
			if got != want {
				t.Fatalf("TryTypedSequenceTransformChainNumericSumFirstLast = %#v, want %#v", got, want)
			}
			wantSum, ok, err := TryTypedNumericSum(wantArray)
			if err != nil || !ok {
				t.Fatalf("TryTypedNumericSum oracle = %#v,%v,%v", wantSum, ok, err)
			}
			wantSumCount := wantSum.(int64) + int64(wantArray.Len())
			gotSumCount, ok, err := TryTypedSequenceTransformChainNumericSumCount(tc.steps, tc.source)
			if err != nil || !ok {
				t.Fatalf("TryTypedSequenceTransformChainNumericSumCount = %#v,%v,%v", gotSumCount, ok, err)
			}
			if gotSumCount != wantSumCount {
				t.Fatalf("TryTypedSequenceTransformChainNumericSumCount = %#v, want %#v", gotSumCount, wantSumCount)
			}
		})
	}

	dropTakeSumCount, ok, err := TryTypedSequenceTransformChainNumericSumCount([]SequenceTransformStep{
		{Transform: SequenceTransformDrop, Args: [2]int{128}, ArgCount: 1},
		{Transform: SequenceTransformSublist, Args: [2]int{1024}, ArgCount: 1},
		{Transform: SequenceTransformSublist, Args: [2]int{512}, ArgCount: 1},
	}, NewI64Range(0, 1, 4096))
	if err != nil || !ok {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumCount drop/take/sublist = %#v,%v,%v", dropTakeSumCount, ok, err)
	}
	if got, want := dropTakeSumCount, int64(196864); got != want {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumCount drop/take/sublist = %#v, want %#v", got, want)
	}

	repeatTakeSteps := []SequenceTransformStep{
		{Transform: SequenceTransformRotate, Args: [2]int{997}, ArgCount: 1},
		{Transform: SequenceTransformDrop, Args: [2]int{1024}, ArgCount: 1},
		{Transform: SequenceTransformSublist, Args: [2]int{9000}, ArgCount: 1},
	}
	repeatTakeSource := NewI64Range(0, 1, 8192)
	repeatTakeEdge, ok, err := TryTypedSequenceTransformChainNumericSumFirstLast(repeatTakeSteps, repeatTakeSource)
	if err != nil || !ok {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumFirstLast repeat take = %#v,%v,%v", repeatTakeEdge, ok, err)
	}
	repeatTakeSumCount, ok, err := TryTypedSequenceTransformChainNumericSumCount(repeatTakeSteps, repeatTakeSource)
	if err != nil || !ok {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumCount repeat take = %#v,%v,%v", repeatTakeSumCount, ok, err)
	}
	var repeatSum int64
	period := 8192 - 1024
	for i := 0; i < 9000; i++ {
		repeatSum += int64((997 + 1024 + (i % period)) % 8192)
	}
	if got, want := repeatTakeEdge, repeatSum+2021+int64((997+1024+((9000-1)%period))%8192); got != want {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumFirstLast repeat take = %#v, want %#v", got, want)
	}
	if got, want := repeatTakeSumCount, repeatSum+9000; got != want {
		t.Fatalf("TryTypedSequenceTransformChainNumericSumCount repeat take = %#v, want %#v", got, want)
	}

	tiledFloats, err := TakeRepeat(NewF64([]float64{0, 1.5}), 7)
	if err != nil {
		t.Fatalf("Take tiled floats returned error: %v", err)
	}
	tiledExpSum, ok, err := TryTypedQNumericUnarySum(NumericUnaryExp, tiledFloats)
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum tiled exp returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnarySum tiled exp did not match float tiled array")
	}
	wantExp := 4*math.Exp(0) + 3*math.Exp(1.5)
	if got := tiledExpSum.(float64); math.Abs(got-wantExp) > 1e-12 {
		t.Fatalf("TryTypedQNumericUnarySum tiled exp = %.17g, want %.17g", got, wantExp)
	}
	tiledFloorSum, ok, err := TryTypedQNumericUnarySum(NumericUnaryFloor, tiledFloats)
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum tiled floor returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnarySum tiled floor did not match float tiled array")
	}
	if got, want := tiledFloorSum, int64(3); got != want {
		t.Fatalf("TryTypedQNumericUnarySum tiled floor = %v (%T), want %v", got, got, want)
	}

	tiledReciprocalDyadicSum, ok, err := TryTypedQNumericUnaryDyadicSum(NumericUnaryRecip, OpAdd, int64(1), tiledInts)
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum reciprocal scalar+tiled returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnaryDyadicSum reciprocal scalar+tiled did not match")
	}
	var wantReciprocal float64
	for _, value := range []float64{-2, 0, 3, -2, 0, 3, -2, 0, 3, -2} {
		wantReciprocal += 1 / (1 + value)
	}
	if got := tiledReciprocalDyadicSum.(float64); math.Abs(got-wantReciprocal) > 1e-12 {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum reciprocal scalar+tiled = %.17g, want %.17g", got, wantReciprocal)
	}

	tiledSignumDyadicSum, ok, err := TryTypedQNumericUnaryDyadicSum(NumericUnarySignum, OpSub, tiledInts, int64(1))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum signum tiled-scalar returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnaryDyadicSum signum tiled-scalar did not match")
	}
	if got, want := tiledSignumDyadicSum, int64(-4); got != want {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum signum tiled-scalar = %v (%T), want %v", got, got, want)
	}

	notMaskValue, ok, err := TryTypedNot(tiledInts)
	if err != nil {
		t.Fatalf("TryTypedNot tiled ints returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedNot tiled ints did not match")
	}
	if got, want := notMaskValue.Values(), []any{false, true, false, false, true, false, false, true, false, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedNot tiled ints values = %v, want %v", got, want)
	}
	notCount, ok, err := TryTypedTrueCount(notMaskValue)
	if err != nil {
		t.Fatalf("TryTypedTrueCount not mask returned error: %v", err)
	}
	if !ok || notCount != 3 {
		t.Fatalf("TryTypedTrueCount not mask = %d, %v; want 3, true", notCount, ok)
	}
	modInput, err := TakeRepeat(NewI64([]int64{0, 1, 2, 3}), 10)
	if err != nil {
		t.Fatalf("TakeRepeat mod input returned error: %v", err)
	}
	modTiled, ok, err := TryTypedIntegerDyadic(OpMod, modInput, int64(2))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic tiled mod returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic tiled mod did not match")
	}
	if _, ok := modTiled.(i64ScalarDyadicArray); !ok {
		t.Fatalf("TryTypedIntegerDyadic tiled mod returned %T, want i64ScalarDyadicArray", modTiled)
	}
	if got, want := modTiled.(Array).Values(), []any{int64(0), int64(1), int64(0), int64(1), int64(0), int64(1), int64(0), int64(1), int64(0), int64(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedIntegerDyadic tiled mod values = %v, want %v", got, want)
	}
	modColumn, ok, err := TryTypedIntegerDyadic(OpMod, NewI64([]int64{0, 1, 2, 3, 4}), int64(3))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic column mod returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic column mod did not match")
	}
	if _, ok := modColumn.(i64ScalarDyadicArray); !ok {
		t.Fatalf("TryTypedIntegerDyadic column mod returned %T, want i64ScalarDyadicArray", modColumn)
	}
	if got, want := modColumn.(Array).Values(), []any{int64(0), int64(1), int64(2), int64(0), int64(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedIntegerDyadic column mod values = %v, want %v", got, want)
	}
	negativeMod, ok, err := TryTypedIntegerDyadic(OpMod, NewI64([]int64{-3, -2, -1, 0, 1}), int64(2))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic negative mod returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic negative mod did not match")
	}
	if got, want := negativeMod.(Array).Values(), []any{int64(1), int64(0), int64(1), int64(0), int64(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedIntegerDyadic negative mod values = %v, want %v", got, want)
	}
	for _, tt := range []struct {
		name    string
		source  Array
		modulus int64
		want    int64
	}{
		{name: "til", source: NewI64Range(0, 1, 40), modulus: 17, want: 287},
		{name: "offset", source: NewI64Range(5, 1, 20), modulus: 7, want: 59},
		{name: "negative start", source: NewI64Range(-5, 1, 12), modulus: 4, want: 18},
		{name: "stepped", source: NewI64Range(7, 3, 12), modulus: 5, want: 22},
	} {
		mod, ok, err := TryTypedIntegerDyadic(OpMod, tt.source, tt.modulus)
		if err != nil {
			t.Fatalf("%s mod returned error: %v", tt.name, err)
		}
		if !ok {
			t.Fatalf("%s mod did not match", tt.name)
		}
		sum, ok, err := TryTypedNumericSum(mod.(Array))
		if err != nil {
			t.Fatalf("%s sum returned error: %v", tt.name, err)
		}
		if !ok || sum != tt.want {
			t.Fatalf("%s sum = %v,%v; want %d,true", tt.name, sum, ok, tt.want)
		}
	}
	innerMod, ok, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 12), int64(5))
	if err != nil || !ok {
		t.Fatalf("nested inner mod handled=%v err=%v; want true,nil", ok, err)
	}
	outerMod, ok, err := TryTypedIntegerDyadic(OpMod, innerMod, int64(2))
	if err != nil || !ok {
		t.Fatalf("nested outer mod handled=%v err=%v; want true,nil", ok, err)
	}
	nestedSum, ok, err := TryTypedNumericSum(outerMod.(Array))
	if err != nil || !ok || nestedSum != int64(5) {
		t.Fatalf("nested mod sum = %v,%v,%v; want 5,true,nil", nestedSum, ok, err)
	}
	xrank, ok, err := TryTypedXrank(4, NewI64Range(0, 1, 9))
	if err != nil {
		t.Fatalf("TryTypedXrank til returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedXrank til did not match")
	}
	if got, want := xrank.Values(), []any{int64(0), int64(0), int64(0), int64(1), int64(1), int64(2), int64(2), int64(3), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedXrank til values = %v, want %v", got, want)
	}
	xrank, ok, err = TryTypedXrank(2, NewI64([]int64{40, 10, 30, 20}))
	if err != nil {
		t.Fatalf("TryTypedXrank slice returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedXrank slice did not match")
	}
	if got, want := xrank.Values(), []any{int64(1), int64(0), int64(1), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedXrank slice values = %v, want %v", got, want)
	}
	modRankInput, ok, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 8192), int64(100))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic xrank mod returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic xrank mod did not match")
	}
	xrank, ok, err = TryTypedXrank(10, modRankInput.(Array))
	if err != nil {
		t.Fatalf("TryTypedXrank modulo returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedXrank modulo did not match")
	}
	if sum, ok, err := TryTypedNumericSum(xrank); err != nil || !ok || sum != int64(36828) {
		t.Fatalf("TryTypedXrank modulo sum = %v,%v,%v; want 36828,true,nil", sum, ok, err)
	}
	count, indexSum, ok, err := TryTypedModuloCompareIndexStatsI64(NewI64Range(0, 1, 12), int64(4), OpEQ, int64(2))
	if err != nil || !ok || count != 3 || indexSum != 18 {
		t.Fatalf("TryTypedModuloCompareIndexStatsI64 eq = %d,%d,%v,%v; want 3,18,true,nil", count, indexSum, ok, err)
	}
	count, indexSum, ok, err = TryTypedModuloCompareIndexStatsI64(NewI64Range(0, 1, 10), int64(5), OpNE, int64(3))
	if err != nil || !ok || count != 8 || indexSum != 34 {
		t.Fatalf("TryTypedModuloCompareIndexStatsI64 ne = %d,%d,%v,%v; want 8,34,true,nil", count, indexSum, ok, err)
	}
	count, indexSum, ok, err = TryTypedModuloCompareIndexStatsI64(NewI64Range(5, 1, 12), int64(4), OpEQ, int64(1))
	if err != nil || !ok || count != 3 || indexSum != 12 {
		t.Fatalf("TryTypedModuloCompareIndexStatsI64 offset eq = %d,%d,%v,%v; want 3,12,true,nil", count, indexSum, ok, err)
	}
	indexes, ok, err := TryTypedModuloCompareIndexesI64(NewI64Range(0, 1, 10), int64(3), OpEQ, int64(0))
	if err != nil || !ok {
		t.Fatalf("TryTypedModuloCompareIndexesI64 = %v,%v; want handled nil", ok, err)
	}
	if got, want := indexes.Values(), []any{int64(0), int64(3), int64(6), int64(9)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedModuloCompareIndexesI64 values = %v, want %v", got, want)
	}
	valueSum, ok, err := TryTypedNumericSumWhereModuloCompare(NewI64Range(1, 2, 10), NewI64Range(0, 1, 10), int64(3), OpEQ, int64(0))
	if err != nil || !ok || valueSum != int64(40) {
		t.Fatalf("TryTypedNumericSumWhereModuloCompare = %v,%v,%v; want 40,true,nil", valueSum, ok, err)
	}
	periodicSum, ok, err := TryTypedNumericSumWhereModuloCompare(NewI64Range(1, 2, 12), NewI64Range(0, 1, 12), int64(5), OpNE, int64(3))
	if err != nil || !ok || periodicSum != int64(120) {
		t.Fatalf("TryTypedNumericSumWhereModuloCompare ne = %v,%v,%v; want 120,true,nil", periodicSum, ok, err)
	}
	attributedNot, ok, err := TryTypedNot(WithArrayAttribute(NewI64([]int64{0, 1}), ArrayAttributeSorted))
	if err != nil {
		t.Fatalf("TryTypedNot attributed returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedNot attributed did not match")
	}
	if metadata := ArrayMetadataOf(attributedNot); len(metadata.Attributes) != 0 {
		t.Fatalf("TryTypedNot preserved attributes = %v, want none", metadata.Attributes)
	}

	value, ok, err := TryTypedQNumericUnarySum(NumericUnaryAbs, NewI64([]int64{-2, 3, 0}))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum abs returned error: %v", err)
	}
	if !ok || value != int64(5) {
		t.Fatalf("TryTypedQNumericUnarySum abs = %v, %v; want 5, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnarySum(NumericUnaryNeg, NewI64Range(0, 1, 5))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum neg range returned error: %v", err)
	}
	if !ok || value != int64(-10) {
		t.Fatalf("TryTypedQNumericUnarySum neg range = %v, %v; want -10, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnarySum(NumericUnaryAbs, NewI64Range(-5, -1, 4))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum abs negative range returned error: %v", err)
	}
	if !ok || value != int64(26) {
		t.Fatalf("TryTypedQNumericUnarySum abs negative range = %v, %v; want 26, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnarySum(NumericUnaryFloor, NewF64([]float64{-1.2, 1.9, 3}))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum floor returned error: %v", err)
	}
	if !ok || value != int64(2) {
		t.Fatalf("TryTypedQNumericUnarySum floor = %v, %v; want 2, true", value, ok)
	}

	lazyFloat, ok, err := typedKernels.Dyadic(OpAdd, NewI64Range(0, 1, 4), float64(0.25))
	if err != nil {
		t.Fatalf("Dyadic range+float returned error: %v", err)
	}
	if !ok {
		t.Fatal("Dyadic range+float did not match numeric range scalar")
	}
	if _, ok := lazyFloat.(f64RangeArray); !ok {
		t.Fatalf("Dyadic range+float returned %T, want f64RangeArray", lazyFloat)
	}
	lazyFloor, ok, err := TryTypedQNumericUnary(NumericUnaryFloor, lazyFloat.(Array))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnary floor lazy f64 range returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedQNumericUnary floor lazy f64 range did not match")
	}
	if got, want := lazyFloor.Values(), []any{int64(0), int64(1), int64(2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedQNumericUnary floor lazy f64 range = %v, want %v", got, want)
	}
	value, ok, err = TryTypedQNumericUnarySum(NumericUnaryCeiling, lazyFloat.(Array))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnarySum ceiling lazy f64 range returned error: %v", err)
	}
	if !ok || value != int64(10) {
		t.Fatalf("TryTypedQNumericUnarySum ceiling lazy f64 range = %v, %v; want 10, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryAbs, OpMul, int64(-1), NewI64([]int64{1, 2, 3}))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum abs scalar*array returned error: %v", err)
	}
	if !ok || value != float64(6) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum abs scalar*array = %v, %v; want 6, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryAbs, OpMul, int64(-1), NewI64Range(0, 1, 5))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum abs scalar*range returned error: %v", err)
	}
	if !ok || value != float64(10) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum abs scalar*range = %v, %v; want 10, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryFloor, OpMul, NewI64([]int64{0, 1, 2, 3}), float64(1.5))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum floor array*scalar returned error: %v", err)
	}
	if !ok || value != int64(8) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum floor array*scalar = %v, %v; want 8, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryCeiling, OpMul, NewI64Range(0, 1, 4), float64(1.5))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum ceiling range*scalar returned error: %v", err)
	}
	if !ok || value != int64(10) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum ceiling range*scalar = %v, %v; want 10, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryFloor, OpMul, NewI64Range(-3, 1, 7), float64(1.5))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum floor negative range*scalar returned error: %v", err)
	}
	if !ok || value != int64(-2) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum floor negative range*scalar = %v, %v; want -2, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryCeiling, OpMul, NewI64Range(-3, 1, 7), float64(1.5))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum ceiling negative range*scalar returned error: %v", err)
	}
	if !ok || value != int64(2) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum ceiling negative range*scalar = %v, %v; want 2, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryFloor, OpDiv, NewI64Range(0, 1, 6), float64(2))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum floor range%%scalar returned error: %v", err)
	}
	if !ok || value != int64(6) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum floor range%%scalar = %v, %v; want 6, true", value, ok)
	}

	value, ok, err = TryTypedQNumericUnaryDyadicSum(NumericUnaryCeiling, OpDiv, NewI64Range(0, 1, 6), float64(2))
	if err != nil {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum ceiling range%%scalar returned error: %v", err)
	}
	if !ok || value != int64(9) {
		t.Fatalf("TryTypedQNumericUnaryDyadicSum ceiling range%%scalar = %v, %v; want 9, true", value, ok)
	}

	minSum, ok, err := TryTypedDyadicMinMaxSum(NewI64Range(0, 1, 8), NewI64Range(7, -1, 8), false)
	if err != nil {
		t.Fatalf("TryTypedDyadicMinMaxSum range min returned error: %v", err)
	}
	if !ok || minSum != int64(12) {
		t.Fatalf("TryTypedDyadicMinMaxSum range min = %v,%v; want 12,true", minSum, ok)
	}
	maxSum, ok, err := TryTypedDyadicMinMaxSum(NewI64Range(0, 1, 8), NewI64Range(7, -1, 8), true)
	if err != nil {
		t.Fatalf("TryTypedDyadicMinMaxSum range max returned error: %v", err)
	}
	if !ok || maxSum != int64(44) {
		t.Fatalf("TryTypedDyadicMinMaxSum range max = %v,%v; want 44,true", maxSum, ok)
	}
	scalarMin, ok, err := TryTypedDyadicMinMaxSum(NewI64([]int64{10, 20, 30, 40}), int64(25), false)
	if err != nil {
		t.Fatalf("TryTypedDyadicMinMaxSum array-scalar min returned error: %v", err)
	}
	if !ok || scalarMin != int64(80) {
		t.Fatalf("TryTypedDyadicMinMaxSum array-scalar min = %v,%v; want 80,true", scalarMin, ok)
	}
	floatMax, ok, err := TryTypedDyadicMinMaxSum(NewF64([]float64{1.5, 4.5, 2.5}), NewF64([]float64{2, 3, 4}), true)
	if err != nil {
		t.Fatalf("TryTypedDyadicMinMaxSum float max returned error: %v", err)
	}
	if !ok || math.Abs(floatMax.(float64)-10.5) > 1e-12 {
		t.Fatalf("TryTypedDyadicMinMaxSum float max = %v,%v; want 10.5,true", floatMax, ok)
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

	scaled, ok, err := typedKernels.NumericBinary(OpMul, NewI64Range(0, 1, 4), NewF64([]float64{0.5, 0.5, 0.5, 0.5}))
	if err != nil {
		t.Fatalf("NumericBinary range*f64 column returned error: %v", err)
	}
	if !ok {
		t.Fatal("NumericBinary range*f64 column did not match numeric arrays")
	}
	if got, want := scaled.Values(), []any{0.0, 0.5, 1.0, 1.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericBinary range*f64 values = %v, want %v", got, want)
	}

	affine, ok, err := typedKernels.Dyadic(OpAdd, NewI64Range(0, 1, 4), float64(1.5))
	if err != nil {
		t.Fatalf("Dyadic range+float returned error: %v", err)
	}
	if !ok {
		t.Fatal("Dyadic range+float did not match numeric range scalar")
	}
	if _, ok := affine.(f64RangeArray); !ok {
		t.Fatalf("Dyadic range+float returned %T, want f64RangeArray", affine)
	}
	if got, want := affine.(Array).Values(), []any{1.5, 2.5, 3.5, 4.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Dyadic range+float values = %v, want %v", got, want)
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

	value, ok, err = TryTypedNumericSum(NewI32([]int32{1, 2, 3}))
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

	value, ok, err = TryTypedNumericSum(shiftedArray{source: NewI64([]int64{10, 20, 30, 40}), offset: -2})
	if err != nil {
		t.Fatalf("TryTypedNumericSum shifted integer returned error: %v", err)
	}
	if !ok || value != int64(30) {
		t.Fatalf("TryTypedNumericSum shifted integer = %v, %v; want 30, true", value, ok)
	}

	value, ok, err = TryTypedNumericSum(NewI64Range(0, 1, 8192))
	if err != nil {
		t.Fatalf("TryTypedNumericSum range returned error: %v", err)
	}
	if want := int64(8192 * 8191 / 2); !ok || value != want {
		t.Fatalf("TryTypedNumericSum range = %v, %v; want %d, true", value, ok, want)
	}

	nestedScaled, ok, err := TryTypedIntegerDyadic(OpMul, NewI64Range(0, 1, 16), int64(3))
	if err != nil || !ok {
		t.Fatalf("TryTypedIntegerDyadic scaled range = %T,%v,%v; want handled", nestedScaled, ok, err)
	}
	nestedAffine, ok, err := TryTypedIntegerDyadic(OpAdd, nestedScaled.(Array), int64(-7))
	if err != nil || !ok {
		t.Fatalf("TryTypedIntegerDyadic affine range = %T,%v,%v; want handled", nestedAffine, ok, err)
	}
	nestedMod, ok, err := TryTypedIntegerDyadic(OpMod, nestedAffine.(Array), int64(5))
	if err != nil || !ok {
		t.Fatalf("TryTypedIntegerDyadic nested mod = %T,%v,%v; want handled", nestedMod, ok, err)
	}
	value, ok, err = TryTypedNumericSum(nestedMod.(Array))
	if err != nil || !ok || value != int64(33) {
		t.Fatalf("TryTypedNumericSum nested scalar dyadic mod = %v,%v,%v; want 33,true,nil", value, ok, err)
	}

	floatMod, ok, err := TryTypedDyadic(OpMod, NewF64([]float64{5.5, -1.5, 7.25}), int64(2))
	if err != nil || !ok {
		t.Fatalf("TryTypedDyadic float mod = %T,%v,%v; want handled", floatMod, ok, err)
	}
	if got, want := floatMod.(Array).Values(), []any{1.5, 0.5, 1.25}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedDyadic float mod values = %v, want %v", got, want)
	}
	rotatedFloat, ok, err := TryTypedRotate(NewF64([]float64{0.5, 1.5, 2.5, 3.5}), 1)
	if err != nil {
		t.Fatalf("TryTypedRotate float returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedRotate float was not handled")
	}
	tiledFloatMod, ok, err := TryTypedDyadic(OpMod, rotatedFloat, int64(2))
	if err != nil || !ok {
		t.Fatalf("TryTypedDyadic tiled float mod = %T,%v,%v; want handled", tiledFloatMod, ok, err)
	}
	if got, want := tiledFloatMod.(Array).Values(), []any{1.5, 0.5, 1.5, 0.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedDyadic tiled float mod values = %v, want %v", got, want)
	}
	indexedFloatMod, ok, err := TryTypedDyadic(OpMod, NewF64([]float64{0.5, 1.5, 2.5, 3.5}).Gather([]int{3, 1, 0}), int64(2))
	if err != nil || !ok {
		t.Fatalf("TryTypedDyadic indexed float mod = %T,%v,%v; want handled", indexedFloatMod, ok, err)
	}
	if got, want := indexedFloatMod.(Array).Values(), []any{1.5, 1.5, 0.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedDyadic indexed float mod values = %v, want %v", got, want)
	}
	nullableFloatMod, ok, err := TryTypedDyadic(OpMod, NewColumn("x", []any{5.5, NullValue, -1.5}).Data, int64(2))
	if err != nil || !ok {
		t.Fatalf("TryTypedDyadic nullable float mod = %T,%v,%v; want handled", nullableFloatMod, ok, err)
	}
	if got, want := nullableFloatMod.(Array).Values(), []any{1.5, NullValue, 0.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedDyadic nullable float mod values = %v, want %v", got, want)
	}
	filledFloat, ok, err := TryTypedScalarFill(0.5, NewColumn("x", []any{5.5, NullValue, -1.5}).Data)
	if err != nil || !ok {
		t.Fatalf("TryTypedScalarFill float = %T,%v,%v; want handled", filledFloat, ok, err)
	}
	filledFloatMod, ok, err := TryTypedDyadic(OpMod, filledFloat, int64(2))
	if err != nil || !ok {
		t.Fatalf("TryTypedDyadic filled float mod = %T,%v,%v; want handled", filledFloatMod, ok, err)
	}
	if got, want := filledFloatMod.(Array).Values(), []any{1.5, 0.5, 0.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedDyadic filled float mod values = %v, want %v", got, want)
	}

	value, ok, err = TryTypedNumericSumByI64Indexes(NewI64Range(0, 1, 8), NewI64([]int64{2, 4, 6}))
	if err != nil {
		t.Fatalf("TryTypedNumericSumByI64Indexes returned error: %v", err)
	}
	if !ok || value != int64(12) {
		t.Fatalf("TryTypedNumericSumByI64Indexes = %v, %v; want 12, true", value, ok)
	}
	value, ok, err = TryTypedNumericSumWhereMask(NewI64Range(0, 1, 8), NewBool([]bool{false, true, false, true, false, true, false, false}))
	if err != nil {
		t.Fatalf("TryTypedNumericSumWhereMask returned error: %v", err)
	}
	if !ok || value != int64(9) {
		t.Fatalf("TryTypedNumericSumWhereMask = %v, %v; want 9, true", value, ok)
	}
	ge, handled, err := TryTypedDyadic(OpGE, NewI64Range(0, 1, 10), int64(2))
	if err != nil || !handled {
		t.Fatalf("TryTypedDyadic range >= scalar = %T, %v, %v; want handled", ge, handled, err)
	}
	lt, handled, err := TryTypedDyadic(OpLT, NewI64Range(0, 1, 10), int64(6))
	if err != nil || !handled {
		t.Fatalf("TryTypedDyadic range < scalar = %T, %v, %v; want handled", lt, handled, err)
	}
	intervalMask, handled, err := TryTypedBoolLogical("and", ge, lt)
	if err != nil || !handled {
		t.Fatalf("TryTypedBoolLogical range interval mask = %T, %v, %v; want handled", intervalMask, handled, err)
	}
	value, ok, err = TryTypedNumericSumWhereMask(NewI64Range(7, 3, 10), intervalMask)
	if err != nil {
		t.Fatalf("TryTypedNumericSumWhereMask range interval returned error: %v", err)
	}
	if !ok || value != int64(70) {
		t.Fatalf("TryTypedNumericSumWhereMask range interval = %v, %v; want 70, true", value, ok)
	}
	value, ok, err = TryTypedNumericSumWhereMask(NewF64([]float64{1.5, 2.5, 3.5}), NewBool([]bool{true, false, true}))
	if err != nil {
		t.Fatalf("TryTypedNumericSumWhereMask f64 returned error: %v", err)
	}
	if !ok || value != float64(5) {
		t.Fatalf("TryTypedNumericSumWhereMask f64 = %v, %v; want 5, true", value, ok)
	}
	value, ok, err = TryTypedNumericSumWhereMask(NewI64Range(0, 1, 3), NewBool([]bool{false, false, false}))
	if err != nil {
		t.Fatalf("TryTypedNumericSumWhereMask empty selection returned error: %v", err)
	}
	if !ok || value != NullValue {
		t.Fatalf("TryTypedNumericSumWhereMask empty selection = %v, %v; want null, true", value, ok)
	}

	value, ok, err = TryTypedNumericAvg(NewI64Range(0, 1, 4))
	if err != nil {
		t.Fatalf("TryTypedNumericAvg range returned error: %v", err)
	}
	if !ok || value != 1.5 {
		t.Fatalf("TryTypedNumericAvg range = %v, %v; want 1.5, true", value, ok)
	}

	value, ok, err = TryTypedNumericProduct(NewI64([]int64{2, 3, 4}))
	if err != nil {
		t.Fatalf("TryTypedNumericProduct i64 returned error: %v", err)
	}
	if !ok || value != int64(24) {
		t.Fatalf("TryTypedNumericProduct i64 = %v, %v; want 24, true", value, ok)
	}

	productScan, ok, err := TryTypedNumericProducts(NewI64Range(1, 1, 4))
	if err != nil {
		t.Fatalf("TryTypedNumericProducts range returned error: %v", err)
	}
	if !ok || productScan.Kind() != KindI64 {
		t.Fatalf("TryTypedNumericProducts range kind = %s, %v; want i64, true", productScan.Kind(), ok)
	}
	if got, want := productScan.Values(), []any{int64(1), int64(2), int64(6), int64(24)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedNumericProducts range values = %v, want %v", got, want)
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
	if _, ok := scan.(i64RunningSumArray); !ok {
		t.Fatalf("TryTypedNumericSums range returned %T, want i64RunningSumArray", scan)
	}
	if got, want := scan.Values(), []any{int64(0), int64(1), int64(3), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedNumericSums range values = %v, want %v", got, want)
	}

	modValues, ok, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 8), int64(3))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic range mod scalar returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic range mod scalar was not handled")
	}
	modScan, ok, err := TryTypedNumericSums(modValues.(Array))
	if err != nil {
		t.Fatalf("TryTypedNumericSums lazy dyadic returned error: %v", err)
	}
	if !ok || modScan.Kind() != KindI64 {
		t.Fatalf("TryTypedNumericSums lazy dyadic kind = %s, %v; want i64, true", modScan.Kind(), ok)
	}
	if _, ok := modScan.(i64ScalarDyadicRunningSumArray); !ok {
		t.Fatalf("TryTypedNumericSums lazy dyadic returned %T, want i64ScalarDyadicRunningSumArray", modScan)
	}
	if got, want := modScan.Values(), []any{int64(0), int64(1), int64(3), int64(3), int64(4), int64(6), int64(6), int64(7)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedNumericSums lazy dyadic values = %v, want %v", got, want)
	}

	divModSum, ok, err := TryTypedIntegerDivModSumCount(
		NewI64Range(1, 1, 1024),
		[]IntegerDivModReducerTerm{{Op: OpDiv, Divisor: 3}, {Op: OpMod, Divisor: 3}},
		true,
	)
	if err != nil {
		t.Fatalf("TryTypedIntegerDivModSumCount range returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDivModSumCount range was not handled")
	}
	var wantDivMod int64 = 1024
	for i := int64(1); i <= 1024; i++ {
		wantDivMod += floorDivInt64(i, 3) + qModInt64(i, 3)
	}
	if divModSum != wantDivMod {
		t.Fatalf("TryTypedIntegerDivModSumCount range = %d, want %d", divModSum, wantDivMod)
	}

	steppedDivModSum, ok, err := TryTypedIntegerDivModSumCount(
		NewI64Range(-11, 4, 17),
		[]IntegerDivModReducerTerm{{Op: OpDiv, Divisor: 5}, {Op: OpMod, Divisor: 5}},
		true,
	)
	if err != nil {
		t.Fatalf("TryTypedIntegerDivModSumCount stepped range returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDivModSumCount stepped range was not handled")
	}
	var wantStepped int64 = 17
	for row := 0; row < 17; row++ {
		value := int64(-11 + 4*row)
		wantStepped += floorDivInt64(value, 5) + qModInt64(value, 5)
	}
	if steppedDivModSum != wantStepped {
		t.Fatalf("TryTypedIntegerDivModSumCount stepped range = %d, want %d", steppedDivModSum, wantStepped)
	}

	arrayDivModSum, ok, err := TryTypedIntegerDivModSumCount(
		NewI64([]int64{-8, -1, 0, 7, 13}),
		[]IntegerDivModReducerTerm{{Op: OpDiv, Divisor: 4}, {Op: OpMod, Divisor: 4}},
		true,
	)
	if err != nil {
		t.Fatalf("TryTypedIntegerDivModSumCount array returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDivModSumCount array was not handled")
	}
	var wantArray int64 = 5
	for _, value := range []int64{-8, -1, 0, 7, 13} {
		wantArray += floorDivInt64(value, 4) + qModInt64(value, 4)
	}
	if arrayDivModSum != wantArray {
		t.Fatalf("TryTypedIntegerDivModSumCount array = %d, want %d", arrayDivModSum, wantArray)
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

	min, handled, has, err = TryTypedMinMax(NewI64Range(10, -2, 4), false)
	if err != nil {
		t.Fatalf("TryTypedMinMax range min returned error: %v", err)
	}
	if !handled || !has || min != int64(4) {
		t.Fatalf("TryTypedMinMax range min = %v, %v, %v; want 4, true, true", min, handled, has)
	}
	max, handled, has, err = TryTypedMinMax(NewI64Range(10, -2, 4), true)
	if err != nil {
		t.Fatalf("TryTypedMinMax range max returned error: %v", err)
	}
	if !handled || !has || max != int64(10) {
		t.Fatalf("TryTypedMinMax range max = %v, %v, %v; want 10, true, true", max, handled, has)
	}

	deltas, handled, err := TryTypedDeltas(NewI64Range(3, 2, 4))
	if err != nil {
		t.Fatalf("TryTypedDeltas range returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedDeltas range did not handle i64 range")
	}
	if got, want := deltas.Values(), []any{int64(3), int64(2), int64(2), int64(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedDeltas range values = %v, want %v", got, want)
	}

	value, ok, err = TryTypedDeltasSum(NewI32([]int32{10, 15, 14, 20}))
	if err != nil || !ok || value != int64(20) {
		t.Fatalf("TryTypedDeltasSum i32 = %v, %v, %v; want 20, true, nil", value, ok, err)
	}
	value, ok, err = TryTypedDeltasSum(NewI64Range(3, 2, 4))
	if err != nil || !ok || value != int64(9) {
		t.Fatalf("TryTypedDeltasSum range = %v, %v, %v; want 9, true, nil", value, ok, err)
	}
	value, ok, err = TryTypedDeltasSum(NewF32([]float32{1.5, 2.25, 4.5}))
	if err != nil || !ok || value != float64(4.5) {
		t.Fatalf("TryTypedDeltasSum f32 = %v, %v, %v; want 4.5, true, nil", value, ok, err)
	}
	value, ok, err = TryTypedDeltasSum(NewI64(nil))
	if err != nil || !ok || value != NullValue {
		t.Fatalf("TryTypedDeltasSum empty = %v, %v, %v; want null, true, nil", value, ok, err)
	}
	value, ok, err = TryTypedDeltasSum(NewColumn("x", []any{int64(1), NullValue, int64(3)}).Data)
	if err != nil || !ok || value != int64(1) {
		t.Fatalf("TryTypedDeltasSum nullable = %v, %v, %v; want 1, true, nil", value, ok, err)
	}

}

func TestF64NumericProducerDirectReductions(t *testing.T) {
	bound, handled, err := BindNumericDyadicFloat(NumericDyadicXExp, int64(2), NewI64([]int64{0, 1, 2, 3}))
	if err != nil || !handled {
		t.Fatalf("BindNumericDyadicFloat xexp = %#v,%v,%v; want handled nil error", bound, handled, err)
	}
	if bound.Len() != 4 {
		t.Fatalf("BindNumericDyadicFloat Len = %d, want 4", bound.Len())
	}
	boundSum, err := bound.Sum()
	if err != nil {
		t.Fatalf("NumericDyadicFloatBound.Sum returned error: %v", err)
	}
	if boundSum != 15 {
		t.Fatalf("NumericDyadicFloatBound.Sum = %v, want 15", boundSum)
	}
	boundRatios, err := bound.RatiosSum()
	if err != nil {
		t.Fatalf("NumericDyadicFloatBound.RatiosSum returned error: %v", err)
	}
	if boundRatios != 7 {
		t.Fatalf("NumericDyadicFloatBound.RatiosSum = %v, want 7", boundRatios)
	}
	if got, want := bound.Array().Values(), []any{1.0, 2.0, 4.0, 8.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NumericDyadicFloatBound.Array values = %v, want %v", got, want)
	}
	nested, handled, err := BindNumericDyadicFloat(NumericDyadicXLog, int64(2), bound.Array())
	if err != nil || !handled {
		t.Fatalf("BindNumericDyadicFloat nested xlog = %#v,%v,%v; want handled nil error", nested, handled, err)
	}
	nestedSum, err := nested.Sum()
	if err != nil {
		t.Fatalf("nested NumericDyadicFloatBound.Sum returned error: %v", err)
	}
	if nestedSum != 6 {
		t.Fatalf("nested NumericDyadicFloatBound.Sum = %v, want 6", nestedSum)
	}
	fusedSum, handled, err := TryTypedNumericSumPlusScalarDyadicFloatSum(bound.Array(), NumericDyadicXLog, int64(2), true)
	if err != nil || !handled {
		t.Fatalf("TryTypedNumericSumPlusScalarDyadicFloatSum = %#v,%v,%v; want handled nil error", fusedSum, handled, err)
	}
	if fusedSum != 21.0 {
		t.Fatalf("TryTypedNumericSumPlusScalarDyadicFloatSum = %v, want 21", fusedSum)
	}

	columnProducer, err := newF64NumericProducer(NewF64([]float64{2, 4, 8, 16}), 4)
	if err != nil {
		t.Fatalf("newF64NumericProducer column returned error: %v", err)
	}
	columnSum, err := f64ProducerSum(columnProducer)
	if err != nil {
		t.Fatalf("f64ProducerSum column returned error: %v", err)
	}
	if columnSum != 30 {
		t.Fatalf("f64ProducerSum column = %v, want 30", columnSum)
	}
	columnRatios, err := f64ProducerRatiosSum(columnProducer)
	if err != nil {
		t.Fatalf("f64ProducerRatiosSum column returned error: %v", err)
	}
	if columnRatios != 8 {
		t.Fatalf("f64ProducerRatiosSum column = %v, want 8", columnRatios)
	}
	zeroRatios, err := f64ProducerRatiosSum(f64ScalarProducer{value: 0, len: 3})
	if err != nil {
		t.Fatalf("f64ProducerRatiosSum zero scalar returned error: %v", err)
	}
	if !math.IsNaN(zeroRatios) {
		t.Fatalf("f64ProducerRatiosSum zero scalar = %v, want NaN", zeroRatios)
	}

	scaled, ok := applyI64RangeScalar(OpMod, i64RangeArray{start: 0, step: 1, len: 8}, 4, false)
	if !ok {
		t.Fatal("applyI64RangeScalar mod did not return typed array")
	}
	lazy, handled, err := TryTypedQNumericDyadicFloat(NumericDyadicXExp, int64(2), scaled)
	if err != nil || !handled {
		t.Fatalf("TryTypedQNumericDyadicFloat lazy = %#v,%v,%v; want handled nil error", lazy, handled, err)
	}
	if _, ok := lazy.(f64NumericDyadicArray); !ok {
		t.Fatalf("TryTypedQNumericDyadicFloat lazy = %T, want f64NumericDyadicArray", lazy)
	}
	if got, want := lazy.Values(), []any{1.0, 2.0, 4.0, 8.0, 1.0, 2.0, 4.0, 8.0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lazy dyadic values = %v, want %v", got, want)
	}

	sumValue, handled, err := typedKernels.NumericSumValue(lazy)
	if err != nil || !handled {
		t.Fatalf("NumericSumValue lazy = %#v,%v,%v; want handled nil error", sumValue, handled, err)
	}
	if sumValue != 30.0 {
		t.Fatalf("NumericSumValue lazy = %v, want 30", sumValue)
	}
	directSum, handled, err := TryTypedQNumericDyadicFloatSum(NumericDyadicXExp, int64(2), scaled)
	if err != nil || !handled {
		t.Fatalf("TryTypedQNumericDyadicFloatSum lazy = %#v,%v,%v; want handled nil error", directSum, handled, err)
	}
	if directSum != 30.0 {
		t.Fatalf("TryTypedQNumericDyadicFloatSum lazy = %v, want 30", directSum)
	}
	ratiosSum, handled, err := TryTypedRatiosSum(lazy)
	if err != nil || !handled {
		t.Fatalf("TryTypedRatiosSum lazy = %#v,%v,%v; want handled nil error", ratiosSum, handled, err)
	}
	if got, want := ratiosSum.(float64), 1.0+2.0+2.0+2.0+0.125+2.0+2.0+2.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("TryTypedRatiosSum lazy = %v, want %v", got, want)
	}

	offsetScaled, ok := applyI64RangeScalar(OpMod, i64RangeArray{start: 3, step: 1, len: 37}, 5, false)
	if !ok {
		t.Fatal("applyI64RangeScalar offset mod did not return typed array")
	}
	offsetLazy, handled, err := TryTypedQNumericDyadicFloat(NumericDyadicXExp, int64(2), offsetScaled)
	if err != nil || !handled {
		t.Fatalf("TryTypedQNumericDyadicFloat offset lazy = %#v,%v,%v; want handled nil error", offsetLazy, handled, err)
	}
	offsetRatiosSum, handled, err := TryTypedRatiosSum(offsetLazy)
	if err != nil || !handled {
		t.Fatalf("TryTypedRatiosSum offset lazy = %#v,%v,%v; want handled nil error", offsetRatiosSum, handled, err)
	}
	var expectedOffsetRatios float64
	var previous float64
	for row := 0; row < 37; row++ {
		residue := qPositiveMod(3+int64(row), 5)
		current := math.Exp2(float64(residue))
		if row == 0 {
			expectedOffsetRatios += current
		} else {
			expectedOffsetRatios += current / previous
		}
		previous = current
	}
	if got := offsetRatiosSum.(float64); math.Abs(got-expectedOffsetRatios) > 1e-12 {
		t.Fatalf("TryTypedRatiosSum offset lazy = %v, want %v", got, expectedOffsetRatios)
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
	if _, ok := integerRangeRight.(i64RangeArray); !ok {
		t.Fatalf("TryTypedIntegerDyadic range scalar returned %T, want i64RangeArray", integerRangeRight)
	}

	rangeLeft, ok, err := TryTypedIntegerDyadic(OpSub, int64(10), NewI64Range(0, 2, 4))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic scalar-range returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic scalar-range did not match numeric range")
	}
	if got, want := rangeLeft.(Array).Values(), []any{int64(10), int64(8), int64(6), int64(4)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedIntegerDyadic scalar-range values = %v, want %v", got, want)
	}
	if _, ok := rangeLeft.(i64RangeArray); !ok {
		t.Fatalf("TryTypedIntegerDyadic scalar-range returned %T, want i64RangeArray", rangeLeft)
	}

	rangeRange, ok, err := TryTypedIntegerDyadic(OpAdd, NewI64Range(0, 2, 4), NewI64Range(10, -1, 4))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic range-range returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic range-range did not match numeric ranges")
	}
	if got, want := rangeRange.(Array).Values(), []any{int64(10), int64(11), int64(12), int64(13)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedIntegerDyadic range-range values = %v, want %v", got, want)
	}
	if _, ok := rangeRange.(i64RangeArray); !ok {
		t.Fatalf("TryTypedIntegerDyadic range-range returned %T, want i64RangeArray", rangeRange)
	}

	rangeProduct, ok, err := TryTypedIntegerDyadic(OpMul, NewI64Range(1, 1, 4), NewI64Range(10, 10, 4))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic range product returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic range product did not match numeric ranges")
	}
	if got, want := rangeProduct.(Array).Values(), []any{int64(10), int64(40), int64(90), int64(160)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedIntegerDyadic range product values = %v, want %v", got, want)
	}
	if _, ok := rangeProduct.(i64ProductArray); !ok {
		t.Fatalf("TryTypedIntegerDyadic range product returned %T, want i64ProductArray", rangeProduct)
	}
	productSum, handled, err := TryTypedNumericSum(rangeProduct.(Array))
	if err != nil {
		t.Fatalf("TryTypedNumericSum range product returned error: %v", err)
	}
	if !handled || productSum != int64(300) {
		t.Fatalf("TryTypedNumericSum range product = %v, %v; want 300, true", productSum, handled)
	}
	descProduct, ok, err := TryTypedIntegerDyadic(OpMul, NewI64Range(10, -2, 5), NewI64Range(-3, 4, 5))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic descending range product returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic descending range product did not match numeric ranges")
	}
	descSum, handled, err := TryTypedNumericSum(descProduct.(Array))
	if err != nil {
		t.Fatalf("TryTypedNumericSum descending range product returned error: %v", err)
	}
	if !handled || descSum != int64(70) {
		t.Fatalf("TryTypedNumericSum descending range product = %v, %v; want 70, true", descSum, handled)
	}
	emptyProduct, ok, err := TryTypedIntegerDyadic(OpMul, NewI64Range(1, 1, 0), NewI64Range(10, 10, 0))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic empty range product returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic empty range product did not match numeric ranges")
	}
	emptySum, handled, err := TryTypedNumericSum(emptyProduct.(Array))
	if err != nil {
		t.Fatalf("TryTypedNumericSum empty range product returned error: %v", err)
	}
	if !handled || emptySum != int64(0) {
		t.Fatalf("TryTypedNumericSum empty range product = %v, %v; want 0, true", emptySum, handled)
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
