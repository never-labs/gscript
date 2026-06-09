package data

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func assertDataRuntimeKernelStat(t *testing.T, stats []RuntimeKernelExecutionStat, kernel, shape, outcome string, count uint64) {
	t.Helper()
	for _, stat := range stats {
		if stat.Source != "data_query_runtime" || stat.Kernel != kernel || stat.Shape != shape || stat.Outcome != outcome {
			continue
		}
		if stat.Route != "typed_data_kernel" {
			t.Fatalf("runtime kernel %s route = %q, want typed_data_kernel; stats=%#v", kernel, stat.Route, stats)
		}
		if stat.Count != count {
			t.Fatalf("runtime kernel %s/%s/%s count = %d, want %d; stats=%#v", kernel, shape, outcome, stat.Count, count, stats)
		}
		return
	}
	t.Fatalf("runtime kernel stat %s/%s/%s not found in %#v", kernel, shape, outcome, stats)
}

func TestNewFrameRejectsUnequalColumnLengths(t *testing.T) {
	_, err := NewFrame(
		NewColumn("a", []any{1, 2}),
		NewColumn("b", []any{1}),
	)
	if err == nil {
		t.Fatal("NewFrame accepted columns with unequal lengths")
	}
	if !strings.Contains(err.Error(), "length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSequenceCutArrayStringAndFrame(t *testing.T) {
	array := NewI64([]int64{10, 20, 30, 40, 50})
	got, err := Cut([]int{0, 2, 4}, array)
	if err != nil {
		t.Fatalf("Cut array returned error: %v", err)
	}
	segments := got.(Array).Values()
	if len(segments) != 3 {
		t.Fatalf("Cut array segments = %d, want 3", len(segments))
	}
	if values := segments[0].(Array).Values(); !reflect.DeepEqual(values, []any{int64(10), int64(20)}) {
		t.Fatalf("Cut array segment 0 = %#v", values)
	}
	if values := segments[2].(Array).Values(); !reflect.DeepEqual(values, []any{int64(50)}) {
		t.Fatalf("Cut array segment 2 = %#v", values)
	}

	text, err := Cut([]int{0, 2}, "abcdef")
	if err != nil {
		t.Fatalf("Cut string returned error: %v", err)
	}
	if values := text.(Array).Values(); !reflect.DeepEqual(values, []any{"ab", "cdef"}) {
		t.Fatalf("Cut string = %#v", values)
	}

	frame, err := NewFrame(NewColumn("qty", []any{10, 20, 30}))
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}
	frameCut, err := Cut([]int{0, 2}, frame)
	if err != nil {
		t.Fatalf("Cut frame returned error: %v", err)
	}
	frameSegments := frameCut.(Array).Values()
	if len(frameSegments) != 2 {
		t.Fatalf("Cut frame segments = %d, want 2", len(frameSegments))
	}
	if gotLen := frameSegments[0].(Frame).Len(); gotLen != 2 {
		t.Fatalf("Cut frame segment 0 length = %d, want 2", gotLen)
	}
	if gotLen := frameSegments[1].(Frame).Len(); gotLen != 1 {
		t.Fatalf("Cut frame segment 1 length = %d, want 1", gotLen)
	}

	count, err := CutCount([]int{0, 2}, frame)
	if err != nil || count != 2 {
		t.Fatalf("CutCount frame = %d,%v; want 2,nil", count, err)
	}
	if _, err := CutCount([]int{0}, 42); err == nil {
		t.Fatalf("CutCount scalar succeeded")
	}
}

func TestSequenceSublistArrayStringAndFrame(t *testing.T) {
	got, err := Sublist(1, 3, NewI64([]int64{10, 20, 30, 40, 50}))
	if err != nil {
		t.Fatalf("Sublist array returned error: %v", err)
	}
	if values := got.(Array).Values(); !reflect.DeepEqual(values, []any{int64(20), int64(30), int64(40)}) {
		t.Fatalf("Sublist array = %#v", values)
	}

	got, err = Sublist(1, 8, NewI64Range(10, 10, 4))
	if err != nil {
		t.Fatalf("Sublist range returned error: %v", err)
	}
	if values := got.(Array).Values(); !reflect.DeepEqual(values, []any{int64(20), int64(30), int64(40)}) {
		t.Fatalf("Sublist range = %#v", values)
	}

	text, err := Sublist(1, 2, "åßcd")
	if err != nil {
		t.Fatalf("Sublist string returned error: %v", err)
	}
	if text != "ßc" {
		t.Fatalf("Sublist string = %q, want ßc", text)
	}

	frame, err := NewFrame(
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		Column{Name: "qty", Data: NewI64([]int64{10, 20, 30})},
	)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}
	frameValue, err := Sublist(1, 2, frame)
	if err != nil {
		t.Fatalf("Sublist frame returned error: %v", err)
	}
	slicedFrame, ok := frameValue.(Frame)
	if !ok {
		t.Fatalf("Sublist frame = %#v, want Frame", frameValue)
	}
	if slicedFrame.Len() != 2 {
		t.Fatalf("Sublist frame len = %d, want 2", slicedFrame.Len())
	}
	if values := slicedFrame.columns[Symbol("sym")].Values(); !reflect.DeepEqual(values, []any{Symbol("MSFT"), Symbol("NVDA")}) {
		t.Fatalf("Sublist frame sym values = %#v", values)
	}

	if _, err := Sublist(-1, 1, NewI64([]int64{1})); err == nil {
		t.Fatalf("Sublist negative start succeeded")
	}

	count, err := SublistCount(1, 8, NewI64Range(10, 10, 4))
	if err != nil || count != 3 {
		t.Fatalf("SublistCount range = %d,%v; want 3,nil", count, err)
	}
	count, err = SublistCount(1, 8, "åßcd")
	if err != nil || count != 3 {
		t.Fatalf("SublistCount string = %d,%v; want 3,nil", count, err)
	}
	count, err = SublistCount(1, 8, frame)
	if err != nil || count != 2 {
		t.Fatalf("SublistCount frame = %d,%v; want 2,nil", count, err)
	}
	if _, err := SublistCount(-1, 1, NewI64([]int64{1})); err == nil {
		t.Fatalf("SublistCount negative start succeeded")
	}
}

func TestSequenceCrossAndItems(t *testing.T) {
	got := Cross(NewSymbols([]string{"a", "b"}), NewI64([]int64{1, 2, 3}))
	if got.Len() != 6 {
		t.Fatalf("Cross length = %d, want 6", got.Len())
	}
	first, _ := got.At(0)
	if values := first.(Array).Values(); !reflect.DeepEqual(values, []any{Symbol("a"), int64(1)}) {
		t.Fatalf("Cross first = %#v", values)
	}
	last, _ := got.At(5)
	if values := last.(Array).Values(); !reflect.DeepEqual(values, []any{Symbol("b"), int64(3)}) {
		t.Fatalf("Cross last = %#v", values)
	}
	if values := SequenceItems(Symbol("x")); !reflect.DeepEqual(values, []any{Symbol("x")}) {
		t.Fatalf("SequenceItems scalar = %#v", values)
	}
	if got := CrossCount(NewSymbols([]string{"a", "b"}), NewI64([]int64{1, 2, 3})); got != 6 {
		t.Fatalf("CrossCount array array = %d, want 6", got)
	}
	if got := CrossCount("abc", NewI64([]int64{1, 2})); got != 2 {
		t.Fatalf("CrossCount scalar string array = %d, want 2", got)
	}
	if got := SequenceCount("åßc"); got != 3 {
		t.Fatalf("SequenceCount string = %d, want 3", got)
	}
	if got := SequenceCount(NewI64([]int64{1, 2})); got != 2 {
		t.Fatalf("SequenceCount array = %d, want 2", got)
	}
	if got := SequenceCount(Symbol("x")); got != 1 {
		t.Fatalf("SequenceCount scalar = %d, want 1", got)
	}
}

func TestReusableStringHelpers(t *testing.T) {
	if got, err := TrimStringValue("  abc \t"); err != nil || got != "abc" {
		t.Fatalf("TrimStringValue = %#v,%v; want abc,nil", got, err)
	}
	if got, err := LTrimStringValue(Symbol("  abc")); err != nil || got != "abc" {
		t.Fatalf("LTrimStringValue = %#v,%v; want abc,nil", got, err)
	}
	if got, err := RTrimStringValue("abc  "); err != nil || got != "abc" {
		t.Fatalf("RTrimStringValue = %#v,%v; want abc,nil", got, err)
	}
	if values := StringSearch("banana", "an").Values(); !reflect.DeepEqual(values, []any{int64(1), int64(3)}) {
		t.Fatalf("StringSearch = %#v, want 1 3", values)
	}
	if got := StringReplaceAll("banana", "an", "ON"); got != "bONONa" {
		t.Fatalf("StringReplaceAll = %q, want bONONa", got)
	}
	joined, err := StringJoin(",", []any{"AAPL", Symbol("MSFT")})
	if err != nil || joined != "AAPL,MSFT" {
		t.Fatalf("StringJoin = %q,%v; want AAPL,MSFT,nil", joined, err)
	}
	if values := StringSplit(",", "AAPL,MSFT").Values(); !reflect.DeepEqual(values, []any{"AAPL", "MSFT"}) {
		t.Fatalf("StringSplit comma = %#v", values)
	}
	if values := StringSplit("", "åß").Values(); !reflect.DeepEqual(values, []any{"å", "ß"}) {
		t.Fatalf("StringSplit runes = %#v", values)
	}

	if got, err := TrimmedStringCount("  åß \t"); err != nil || got != 2 {
		t.Fatalf("TrimmedStringCount scalar = %d,%v; want 2,nil", got, err)
	}
	if got, err := LTrimmedStringCount(Symbol("  åß  ")); err != nil || got != 4 {
		t.Fatalf("LTrimmedStringCount symbol = %d,%v; want 4,nil", got, err)
	}
	if got, err := RTrimmedStringCount("  åß  "); err != nil || got != 4 {
		t.Fatalf("RTrimmedStringCount scalar = %d,%v; want 4,nil", got, err)
	}
	if got, err := TrimmedStringCount(NewString([]string{" A ", "B"})); err != nil || got != 2 {
		t.Fatalf("TrimmedStringCount array = %d,%v; want 2,nil", got, err)
	}
	if got, err := TrimmedStringCount(NullValue); err != nil || got != 0 {
		t.Fatalf("TrimmedStringCount null = %d,%v; want 0,nil", got, err)
	}
	if _, err := TrimmedStringCount(42); err == nil {
		t.Fatalf("TrimmedStringCount numeric succeeded")
	}
}

func TestTryTypedWhereMaskI64(t *testing.T) {
	got, handled, err := TryTypedWhereMaskI64(NewBool([]bool{true, false, true, true, false}))
	if err != nil {
		t.Fatalf("TryTypedWhereMaskI64 bool returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedWhereMaskI64 bool did not handle typed bool array")
	}
	if got.Kind() != KindI64 {
		t.Fatalf("TryTypedWhereMaskI64 kind = %s, want %s", got.Kind(), KindI64)
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(0), int64(2), int64(3)}) {
		t.Fatalf("TryTypedWhereMaskI64 bool values = %#v", values)
	}

	nullableColumn, err := NewColumnWithKind("_", KindBool, []any{true, NullValue, false, true})
	if err != nil {
		t.Fatalf("NewColumnWithKind nullable bool returned error: %v", err)
	}
	nullable := nullableColumn.Data
	got, handled, err = TryTypedWhereMaskI64(nullable)
	if err != nil {
		t.Fatalf("TryTypedWhereMaskI64 nullable returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedWhereMaskI64 nullable did not handle bool nullable array")
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(0), int64(3)}) {
		t.Fatalf("TryTypedWhereMaskI64 nullable values = %#v", values)
	}
}

func TestTryTypedWhereMaskI64UsesLazyModuloCompare(t *testing.T) {
	mod, handled, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 20), int64(5))
	if err != nil || !handled {
		t.Fatalf("mod range handled=%v err=%v; want true,nil", handled, err)
	}
	maskValue, handled, err := TryTypedDyadic(OpEQ, mod, int64(2))
	if err != nil || !handled {
		t.Fatalf("mod compare handled=%v err=%v; want true,nil", handled, err)
	}
	mask := maskValue.(Array)
	if _, ok := mask.(i64ScalarDyadicCompareMask); !ok {
		t.Fatalf("mod compare returned %T, want lazy i64ScalarDyadicCompareMask", mask)
	}
	if count, handled, err := TryTypedTrueCount(mask); err != nil || !handled || count != 4 {
		t.Fatalf("mod compare true count = %d,%v,%v; want 4,true,nil", count, handled, err)
	}
	indexes, handled, err := TryTypedWhereMaskI64(mask)
	if err != nil || !handled {
		t.Fatalf("mod compare where handled=%v err=%v; want true,nil", handled, err)
	}
	if got, want := indexes.Values(), []any{int64(2), int64(7), int64(12), int64(17)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mod compare where = %#v, want %#v", got, want)
	}
	if _, ok := indexes.(i64RangeArray); !ok {
		t.Fatalf("mod compare where returned %T, want lazy i64RangeArray", indexes)
	}

	notMaskValue, handled, err := TryTypedDyadic(OpNE, mod, int64(2))
	if err != nil || !handled {
		t.Fatalf("mod not-equal compare handled=%v err=%v; want true,nil", handled, err)
	}
	notIndexes, handled, err := TryTypedWhereMaskI64(notMaskValue.(Array))
	if err != nil || !handled {
		t.Fatalf("mod not-equal where handled=%v err=%v; want true,nil", handled, err)
	}
	if got, want := notIndexes.Values(), []any{int64(0), int64(1), int64(3), int64(4), int64(5), int64(6), int64(8), int64(9), int64(10), int64(11), int64(13), int64(14), int64(15), int64(16), int64(18), int64(19)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mod not-equal where = %#v, want %#v", got, want)
	}
	if _, ok := notIndexes.(i64PeriodicIndexArray); !ok {
		t.Fatalf("mod not-equal where returned %T, want lazy i64PeriodicIndexArray", notIndexes)
	}
}

func TestTryTypedWhereMaskI64UsesLazyModuloLogicalMask(t *testing.T) {
	mod, handled, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 18), int64(6))
	if err != nil || !handled {
		t.Fatalf("mod range handled=%v err=%v; want true,nil", handled, err)
	}
	lower, handled, err := TryTypedDyadic(OpGT, mod, int64(1))
	if err != nil || !handled {
		t.Fatalf("mod lower compare handled=%v err=%v; want true,nil", handled, err)
	}
	upper, handled, err := TryTypedDyadic(OpLT, mod, int64(4))
	if err != nil || !handled {
		t.Fatalf("mod upper compare handled=%v err=%v; want true,nil", handled, err)
	}
	band, handled, err := TryTypedBoolLogical("and", lower, upper)
	if err != nil || !handled {
		t.Fatalf("mod logical band handled=%v err=%v; want true,nil", handled, err)
	}
	if count, handled, err := TryTypedTrueCount(band); err != nil || !handled || count != 6 {
		t.Fatalf("mod logical band true count = %d,%v,%v; want 6,true,nil", count, handled, err)
	}
	indexes, handled, err := TryTypedWhereMaskI64(band)
	if err != nil || !handled {
		t.Fatalf("mod logical band where handled=%v err=%v; want true,nil", handled, err)
	}
	if got, want := indexes.Values(), []any{int64(2), int64(3), int64(8), int64(9), int64(14), int64(15)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mod logical band where = %#v, want %#v", got, want)
	}
	if _, ok := indexes.(i64PeriodicIndexArray); !ok {
		t.Fatalf("mod logical band where returned %T, want lazy i64PeriodicIndexArray", indexes)
	}

	mod7, handled, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 24), int64(7))
	if err != nil || !handled {
		t.Fatalf("mod7 handled=%v err=%v; want true,nil", handled, err)
	}
	mod11, handled, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 24), int64(11))
	if err != nil || !handled {
		t.Fatalf("mod11 handled=%v err=%v; want true,nil", handled, err)
	}
	eq7, handled, err := TryTypedDyadic(OpEQ, mod7, int64(0))
	if err != nil || !handled {
		t.Fatalf("mod7 compare handled=%v err=%v; want true,nil", handled, err)
	}
	eq11, handled, err := TryTypedDyadic(OpEQ, mod11, int64(0))
	if err != nil || !handled {
		t.Fatalf("mod11 compare handled=%v err=%v; want true,nil", handled, err)
	}
	either, handled, err := TryTypedBoolLogical("or", eq7, eq11)
	if err != nil || !handled {
		t.Fatalf("mixed-period modulo or handled=%v err=%v; want true,nil", handled, err)
	}
	if count, handled, err := TryTypedTrueCount(either); err != nil || !handled || count != 6 {
		t.Fatalf("mixed-period modulo or true count = %d,%v,%v; want 6,true,nil", count, handled, err)
	}
	eitherIndexes, handled, err := TryTypedWhereMaskI64(either)
	if err != nil || !handled {
		t.Fatalf("mixed-period modulo or where handled=%v err=%v; want true,nil", handled, err)
	}
	if got, want := eitherIndexes.Values(), []any{int64(0), int64(7), int64(11), int64(14), int64(21), int64(22)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed-period modulo or where = %#v, want %#v", got, want)
	}
	if _, ok := eitherIndexes.(i64PeriodicIndexArray); !ok {
		t.Fatalf("mixed-period modulo or where returned %T, want lazy i64PeriodicIndexArray", eitherIndexes)
	}
}

func TestTryTypedCompareIndexesI64(t *testing.T) {
	got, handled, err := TryTypedCompareIndexesI64(NewI64Range(0, 1, 6), OpGE, int64(3))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 range returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 range did not handle typed compare")
	}
	if got.Kind() != KindI64 {
		t.Fatalf("TryTypedCompareIndexesI64 kind = %s, want %s", got.Kind(), KindI64)
	}
	if _, ok := got.(i64RangeArray); !ok {
		t.Fatalf("TryTypedCompareIndexesI64 range returned %T, want lazy i64RangeArray", got)
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(3), int64(4), int64(5)}) {
		t.Fatalf("TryTypedCompareIndexesI64 range values = %#v", values)
	}

	got, handled, err = TryTypedCompareIndexesI64(NewI64Range(10, -2, 6), OpGT, int64(4))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 descending range returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 descending range did not handle typed compare")
	}
	if _, ok := got.(i64RangeArray); !ok {
		t.Fatalf("TryTypedCompareIndexesI64 descending range returned %T, want lazy i64RangeArray", got)
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(0), int64(1), int64(2)}) {
		t.Fatalf("TryTypedCompareIndexesI64 descending range values = %#v", values)
	}

	got, handled, err = TryTypedCompareIndexesI64(NewI64Range(0, 1, 6), OpNE, int64(3))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 non-contiguous range returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 non-contiguous range did not handle typed compare")
	}
	if _, ok := got.(i64RangeArray); ok {
		t.Fatalf("TryTypedCompareIndexesI64 non-contiguous range returned lazy range, want materialized indexes")
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(0), int64(1), int64(2), int64(4), int64(5)}) {
		t.Fatalf("TryTypedCompareIndexesI64 non-contiguous range values = %#v", values)
	}

	got, handled, err = TryTypedCompareIndexesI64(NewSymbols([]string{"AAPL", "MSFT", "NVDA"}), OpLT, "NVDA")
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 symbols returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 symbols did not handle typed compare")
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(0), int64(1)}) {
		t.Fatalf("TryTypedCompareIndexesI64 symbol values = %#v", values)
	}

	modRange, ok, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 12), int64(4))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic range mod returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic range mod did not handle")
	}
	got, handled, err = TryTypedCompareIndexesI64(modRange.(Array), OpEQ, int64(2))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 lazy mod returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 lazy mod did not handle")
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(2), int64(6), int64(10)}) {
		t.Fatalf("TryTypedCompareIndexesI64 lazy mod values = %#v", values)
	}

	got, handled, err = TryTypedCompareIndexesI64(NewI64Range(math.MaxInt64-1, 1, 4), OpLT, int64(0))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 wrapped ascending returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 wrapped ascending did not handle typed compare")
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(2), int64(3)}) {
		t.Fatalf("TryTypedCompareIndexesI64 wrapped ascending values = %#v", values)
	}

	got, handled, err = TryTypedCompareIndexesI64(NewI64Range(math.MinInt64+1, -1, 4), OpGT, int64(0))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 wrapped descending returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 wrapped descending did not handle typed compare")
	}
	if values := got.Values(); !reflect.DeepEqual(values, []any{int64(2), int64(3)}) {
		t.Fatalf("TryTypedCompareIndexesI64 wrapped descending values = %#v", values)
	}
}

func TestTryTypedCompareIndexStatsI64(t *testing.T) {
	count, sum, handled, err := TryTypedCompareIndexStatsI64(NewI64Range(0, 1, 6), OpGE, int64(3))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 ascending returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 ascending did not handle typed compare")
	}
	if count != 3 || sum != 12 {
		t.Fatalf("TryTypedCompareIndexStatsI64 ascending = count %d sum %d; want 3, 12", count, sum)
	}

	count, sum, handled, err = TryTypedCompareIndexStatsI64(NewI64Range(10, -2, 6), OpGT, int64(4))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 descending returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 descending did not handle typed compare")
	}
	if count != 3 || sum != 3 {
		t.Fatalf("TryTypedCompareIndexStatsI64 descending = count %d sum %d; want 3, 3", count, sum)
	}

	count, sum, handled, err = TryTypedCompareIndexStatsI64(NewI64Range(0, 1, 6), OpLT, int64(0))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 empty returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 empty did not handle typed compare")
	}
	if count != 0 || sum != 0 {
		t.Fatalf("TryTypedCompareIndexStatsI64 empty = count %d sum %d; want 0, 0", count, sum)
	}

	count, sum, handled, err = TryTypedCompareIndexStatsI64(NewI64Range(0, 2, 4), OpNE, int64(3))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 ne absent returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 ne absent did not handle typed compare")
	}
	if count != 4 || sum != 6 {
		t.Fatalf("TryTypedCompareIndexStatsI64 ne absent = count %d sum %d; want 4, 6", count, sum)
	}

	_, _, handled, err = TryTypedCompareIndexStatsI64(NewI64Range(0, 1, 6), OpNE, int64(3))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 non-contiguous returned error: %v", err)
	}
	if handled {
		t.Fatal("TryTypedCompareIndexStatsI64 non-contiguous NE handled, want fallback")
	}

	count, sum, handled, err = TryTypedCompareIndexStatsI64(NewDate([]Date{
		DateFromDays(20610),
		DateFromDays(20611),
		DateFromDays(20612),
		DateFromDays(20610),
	}), OpGE, DateFromDays(20611))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 date returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 date did not handle typed compare")
	}
	if count != 2 || sum != 3 {
		t.Fatalf("TryTypedCompareIndexStatsI64 date = count %d sum %d; want 2, 3", count, sum)
	}

	count, sum, handled, err = TryTypedCompareIndexStatsI64(NewSymbols([]string{"AAPL", "MSFT", "NVDA"}), OpLT, "NVDA")
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 symbol returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 symbol did not handle typed compare")
	}
	if count != 2 || sum != 1 {
		t.Fatalf("TryTypedCompareIndexStatsI64 symbol = count %d sum %d; want 2, 1", count, sum)
	}

	modRange, ok, err := TryTypedIntegerDyadic(OpMod, NewI64Range(0, 1, 12), int64(4))
	if err != nil {
		t.Fatalf("TryTypedIntegerDyadic range mod returned error: %v", err)
	}
	if !ok {
		t.Fatal("TryTypedIntegerDyadic range mod did not handle")
	}
	count, sum, handled, err = TryTypedCompareIndexStatsI64(modRange.(Array), OpEQ, int64(2))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 lazy mod returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 lazy mod did not handle")
	}
	if count != 3 || sum != 18 {
		t.Fatalf("TryTypedCompareIndexStatsI64 lazy mod = count %d sum %d; want 3, 18", count, sum)
	}

	tiledDates, err := TakeRepeat(NewDate([]Date{
		DateFromDays(20610),
		DateFromDays(20611),
		DateFromDays(20612),
	}), 9)
	if err != nil {
		t.Fatalf("TakeRepeat date returned error: %v", err)
	}
	count, sum, handled, err = TryTypedCompareIndexStatsI64(tiledDates, OpGE, DateFromDays(20611))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 tiled date returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 tiled date did not handle typed compare")
	}
	if count != 6 || sum != 27 {
		t.Fatalf("TryTypedCompareIndexStatsI64 tiled date = count %d sum %d; want 6, 27", count, sum)
	}

	tiledIndexes, handled, err := TryTypedCompareIndexesI64(tiledDates, OpGE, DateFromDays(20611))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 tiled date returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 tiled date did not handle typed compare")
	}
	if got, want := tiledIndexes.Values(), []any{int64(1), int64(2), int64(4), int64(5), int64(7), int64(8)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedCompareIndexesI64 tiled date = %#v, want %#v", got, want)
	}

	rotatedDates, err := Slice(tiledDates, 1, 7)
	if err != nil {
		t.Fatalf("Slice tiled date returned error: %v", err)
	}
	count, sum, handled, err = TryTypedCompareIndexStatsI64(rotatedDates, OpGE, DateFromDays(20611))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 rotated tiled date returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexStatsI64 rotated tiled date did not handle typed compare")
	}
	if count != 5 || sum != 14 {
		t.Fatalf("TryTypedCompareIndexStatsI64 rotated tiled date = count %d sum %d; want 5, 14", count, sum)
	}

	count, handled, err = TryTypedCompareCount(rotatedDates, OpGE, DateFromDays(20611))
	if err != nil {
		t.Fatalf("TryTypedCompareCount rotated tiled date returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareCount rotated tiled date did not handle typed compare")
	}
	if count != 5 {
		t.Fatalf("TryTypedCompareCount rotated tiled date = %d; want 5", count)
	}
}

func TestNewI64RangeArraySemantics(t *testing.T) {
	values := NewI64Range(10, 2, 5)
	if values.Kind() != KindI64 {
		t.Fatalf("range kind = %s, want %s", values.Kind(), KindI64)
	}
	if values.Len() != 5 {
		t.Fatalf("range len = %d, want 5", values.Len())
	}
	for row, want := range []any{int64(10), int64(12), int64(14), int64(16), int64(18)} {
		got, ok := values.At(row)
		if !ok || got != want {
			t.Fatalf("range At(%d) = %v, %v; want %v, true", row, got, ok, want)
		}
	}
	if got, ok := values.At(5); ok || got != nil {
		t.Fatalf("range At(out of bounds) = %v, %v; want nil, false", got, ok)
	}
	if got, want := values.Values(), []any{int64(10), int64(12), int64(14), int64(16), int64(18)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range Values = %#v, want %#v", got, want)
	}
	gathered := values.Gather([]int{4, 1, 0})
	if got, want := gathered.Values(), []any{int64(18), int64(12), int64(10)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range Gather values = %#v, want %#v", got, want)
	}

	sliced := values.Gather([]int{1, 2, 3})
	if _, ok := sliced.(i64RangeArray); !ok {
		t.Fatalf("range Gather consecutive returned %T, want i64RangeArray", sliced)
	}
	if got, want := sliced.Values(), []any{int64(12), int64(14), int64(16)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range Gather consecutive values = %#v, want %#v", got, want)
	}

	reversed := values.Gather([]int{4, 3, 2, 1, 0})
	if _, ok := reversed.(i64RangeArray); !ok {
		t.Fatalf("range Gather reverse returned %T, want i64RangeArray", reversed)
	}
	if got, want := reversed.Values(), []any{int64(18), int64(16), int64(14), int64(12), int64(10)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range Gather reverse values = %#v, want %#v", got, want)
	}

	rotated, handled, err := TryTypedRotate(NewI64Range(0, 1, 5), 2)
	if err != nil {
		t.Fatalf("TryTypedRotate returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedRotate did not handle i64 range")
	}
	if _, ok := rotated.(i64SegmentArray); !ok {
		t.Fatalf("TryTypedRotate returned %T, want i64SegmentArray", rotated)
	}
	if got, want := rotated.Values(), []any{int64(2), int64(3), int64(4), int64(0), int64(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedRotate values = %#v, want %#v", got, want)
	}
	sum, handled, err := TryTypedNumericSum(rotated)
	if err != nil {
		t.Fatalf("TryTypedNumericSum rotated returned error: %v", err)
	}
	if !handled || sum != int64(10) {
		t.Fatalf("TryTypedNumericSum rotated = %v, %v; want 10, true", sum, handled)
	}
}

func TestI64SegmentCompareKernels(t *testing.T) {
	rotated, handled, err := TryTypedRotate(NewI64Range(0, 1, 16), 11)
	if err != nil {
		t.Fatalf("TryTypedRotate returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedRotate did not handle i64 range")
	}
	if _, ok := rotated.(i64SegmentArray); !ok {
		t.Fatalf("TryTypedRotate returned %T, want i64SegmentArray", rotated)
	}

	indexes, handled, err := TryTypedCompareIndexesI64(rotated, OpLT, int64(4))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexesI64 segment returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedCompareIndexesI64 segment did not handle")
	}
	if got, want := indexes.Values(), []any{int64(5), int64(6), int64(7), int64(8)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("segment compare indexes = %#v, want %#v", got, want)
	}

	count, sum, handled, err := TryTypedCompareIndexStatsI64(rotated, OpLT, int64(4))
	if err != nil {
		t.Fatalf("TryTypedCompareIndexStatsI64 segment returned error: %v", err)
	}
	if !handled || count != 4 || sum != 26 {
		t.Fatalf("segment compare stats = count %d sum %d handled %v; want 4, 26, true", count, sum, handled)
	}

	ge, handled, err := TryTypedDyadic(OpGE, rotated, int64(3))
	if err != nil || !handled {
		t.Fatalf("TryTypedDyadic segment ge handled=%v err=%v", handled, err)
	}
	lt, handled, err := TryTypedDyadic(OpLT, rotated, int64(7))
	if err != nil || !handled {
		t.Fatalf("TryTypedDyadic segment lt handled=%v err=%v", handled, err)
	}
	mask, handled, err := TryTypedBoolLogical("and", ge, lt)
	if err != nil || !handled {
		t.Fatalf("TryTypedBoolLogical segment handled=%v err=%v", handled, err)
	}
	trueCount, handled, err := TryTypedTrueCount(mask)
	if err != nil || !handled || trueCount != 4 {
		t.Fatalf("TryTypedTrueCount segment mask = %d,%v,%v; want 4,true,nil", trueCount, handled, err)
	}
	where, handled, err := TryTypedWhereMaskI64(mask)
	if err != nil || !handled {
		t.Fatalf("TryTypedWhereMaskI64 segment mask handled=%v err=%v", handled, err)
	}
	if got, want := where.Values(), []any{int64(8), int64(9), int64(10), int64(11)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("segment logical where = %#v, want %#v", got, want)
	}

	gathered, handled, err := TryGatherByI64IndexArray(rotated, where)
	if err != nil || !handled {
		t.Fatalf("TryGatherByI64IndexArray segment handled=%v err=%v", handled, err)
	}
	if got, want := gathered.Values(), []any{int64(3), int64(4), int64(5), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("segment logical gather = %#v, want %#v", got, want)
	}
}

func TestNewFramePreservesColumnOrderAndKinds(t *testing.T) {
	frame, err := NewFrame(
		NewColumn("z", []any{int64(3)}),
		NewColumn("a", []any{"x"}),
		NewColumn("m", []any{Symbol("MSFT")}),
	)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}

	if got, want := frame.Schema().Names(), []Symbol{"z", "a", "m"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("column order = %v, want %v", got, want)
	}
	if got, _ := frame.Schema().Kind("m"); got != KindSymbol {
		t.Fatalf("kind(m) = %s, want %s", got, KindSymbol)
	}
}

func TestFrameSchemaFingerprintCompatibilityAndClone(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "AAPL"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30})},
		Column{Name: "price", Data: NewF64([]float64{100.5, 80.25, 101.0})},
	)
	hash := frame.SchemaFingerprint()
	if hash == "" {
		t.Fatal("schema fingerprint is empty")
	}

	clone, err := frame.Clone()
	if err != nil {
		t.Fatalf("Clone returned error: %v", err)
	}
	if !SameSchema(frame, clone) {
		t.Fatal("Clone changed frame schema")
	}
	if clone.SchemaFingerprint() != hash {
		t.Fatalf("clone schema hash = %s, want %s", clone.SchemaFingerprint(), hash)
	}

	gathered, err := frame.Gather([]int{2, 0})
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}
	if !frame.Schema().CompatibleWith(gathered.Schema()) {
		t.Fatal("Gather changed schema compatibility")
	}
	if gathered.SchemaFingerprint() != hash {
		t.Fatalf("gathered schema hash = %s, want %s", gathered.SchemaFingerprint(), hash)
	}

	projected, err := SelectFrameColumns(frame, "qty", "sym")
	if err != nil {
		t.Fatalf("SelectFrameColumns returned error: %v", err)
	}
	if SameSchema(frame, projected) {
		t.Fatal("projected frame unexpectedly has same schema")
	}
	if projected.SchemaFingerprint() == hash {
		t.Fatalf("projected schema hash = %s, want different from %s", projected.SchemaFingerprint(), hash)
	}

	sameShapeDifferentRows := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"NVDA"})},
		Column{Name: "qty", Data: NewI32([]int32{1})},
		Column{Name: "price", Data: NewF64([]float64{120.75})},
	)
	if !frame.Schema().CompatibleWith(sameShapeDifferentRows.Schema()) {
		t.Fatal("schemas with same column order and kinds should be compatible")
	}
	if sameShapeDifferentRows.SchemaFingerprint() != hash {
		t.Fatalf("same schema hash = %s, want %s", sameShapeDifferentRows.SchemaFingerprint(), hash)
	}
}

func TestArrayAttributeMetadataPropagatesThroughFrameGatherAndLookup(t *testing.T) {
	sorted := WithArrayAttribute(NewTimestamp([]Timestamp{10, 20, 30}), ArrayAttributeSorted)
	if !ArrayHasAttribute(sorted, ArrayAttributeSorted) {
		t.Fatal("sorted attribute was not visible on attributed array")
	}
	frame := mustFrame(t,
		Column{Name: "ts", Data: sorted},
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("c")}),
		NewColumn("qty", []any{10, 20, 30}),
	)
	ts, ok := frame.Column("ts")
	if !ok || !ArrayHasAttribute(ts, ArrayAttributeSorted) {
		t.Fatalf("frame ts attribute visible = %v, ok %v; want sorted", ArrayMetadataOf(ts), ok)
	}
	gathered, err := frame.Gather([]int{0, 2})
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}
	gatheredTS, _ := gathered.Column("ts")
	if !ArrayHasAttribute(gatheredTS, ArrayAttributeSorted) {
		t.Fatalf("gathered ts metadata = %#v, want sorted", ArrayMetadataOf(gatheredTS))
	}
	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	looked, err := keyed.LookupByKey(Symbol("b"))
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	lookedTS, _ := looked.Column("ts")
	if !ArrayHasAttribute(lookedTS, ArrayAttributeSorted) {
		t.Fatalf("lookup ts metadata = %#v, want sorted", ArrayMetadataOf(lookedTS))
	}
}

func TestGroupedAndUniqueAttributesBuildReusableIndexes(t *testing.T) {
	grouped := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL"}), ArrayAttributeGrouped)
	index, ok := ArrayIndexFor(grouped, ArrayAttributeGrouped)
	if !ok {
		t.Fatalf("grouped metadata = %#v, want index", ArrayMetadataOf(grouped))
	}
	if got, want := index.Keys, []any{Symbol("AAPL"), Symbol("MSFT")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grouped index keys = %#v, want %#v", got, want)
	}
	if got, want := index.Rows, [][]int{{0, 2}, {1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grouped index rows = %#v, want %#v", got, want)
	}

	frame := mustFrame(t,
		Column{Name: "sym", Data: grouped},
		NewColumn("qty", []any{10, 20, 30}),
	)
	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	got, err := keyed.LookupByKey(Symbol("AAPL"))
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, got, "qty", []any{int64(10), int64(30)})

	unique := WithArrayAttribute(NewI64([]int64{10, 20, 30}), ArrayAttributeUnique)
	if _, ok := ArrayIndexFor(unique, ArrayAttributeUnique); !ok {
		t.Fatalf("unique metadata = %#v, want index", ArrayMetadataOf(unique))
	}
	gathered := unique.Gather([]int{0, 2})
	if ArrayHasAttribute(gathered, ArrayAttributeUnique) == false {
		t.Fatalf("gathered metadata = %#v, want unique attribute", ArrayMetadataOf(gathered))
	}
	gatheredIndex, ok := ArrayIndexFor(gathered, ArrayAttributeUnique)
	if !ok {
		t.Fatalf("gathered metadata = %#v, want rebuilt unique index", ArrayMetadataOf(gathered))
	}
	if got, want := gatheredIndex.Rows, [][]int{{0}, {1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gathered unique rows = %#v, want %#v", got, want)
	}

	regrouped := grouped.Gather([]int{1, 0, 2})
	regroupedIndex, ok := ArrayIndexFor(regrouped, ArrayAttributeGrouped)
	if !ok {
		t.Fatalf("regrouped metadata = %#v, want rebuilt grouped index", ArrayMetadataOf(regrouped))
	}
	if got, want := regroupedIndex.Keys, []any{Symbol("MSFT"), Symbol("AAPL")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("regrouped keys = %#v, want %#v", got, want)
	}
	if got, want := regroupedIndex.Rows, [][]int{{0}, {1, 2}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("regrouped rows = %#v, want %#v", got, want)
	}
}

func TestAttributeIndexSurvivesFrameGatherForFilterAndKeyedLookup(t *testing.T) {
	grouped := WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA"}), ArrayAttributeGrouped)
	frame := mustFrame(t,
		Column{Name: "sym", Data: grouped},
		NewColumn("seq", []any{1, 2, 3, 4}),
	)
	gathered, err := frame.Gather([]int{2, 0, 1})
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}
	sym, _ := gathered.Column("sym")
	index, ok := ArrayIndexFor(sym, ArrayAttributeGrouped)
	if !ok {
		t.Fatalf("gathered sym metadata = %#v, want grouped index", ArrayMetadataOf(sym))
	}
	aaplKey := arrayValueKey(KindSymbol, Symbol("AAPL"))
	if got, want := index.RowsByKey[aaplKey], []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gathered AAPL rows = %v, want %v", got, want)
	}

	indexes, err := filterIndexes(gathered, Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("AAPL")}})
	if err != nil {
		t.Fatalf("filterIndexes returned error: %v", err)
	}
	if want := []int{0, 1}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("filter indexes = %v, want %v", indexes, want)
	}

	keyed, err := KeyBy(gathered, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	hit, err := keyed.LookupByKey("AAPL")
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, hit, "seq", []any{int64(3), int64(1)})
}

func TestFilterIndexesUsesTypedIndexKernelsWithNormalizedLiterals(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30, 40})},
		Column{Name: "ts", Data: NewTimestamp([]Timestamp{100, 200, 300, 400})},
	)

	indexes, err := filterIndexes(frame, Binary{
		Op:    OpGE,
		Left:  ColumnRef{Name: "qty"},
		Right: Literal{Value: int64(30)},
	})
	if err != nil {
		t.Fatalf("filterIndexes compare returned error: %v", err)
	}
	if want := []int{2, 3}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("compare filter indexes = %v, want %v", indexes, want)
	}

	indexes, err = filterIndexes(frame, Within{
		Expr:       ColumnRef{Name: "ts"},
		Low:        Timestamp(150),
		High:       Timestamp(300),
		HighClosed: true,
	})
	if err != nil {
		t.Fatalf("filterIndexes within returned error: %v", err)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("within filter indexes = %v, want %v", indexes, want)
	}
}

func TestFilterIndexesUsesAttributeIndexForIn(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"}), ArrayAttributeGrouped)},
		NewColumn("qty", []any{10, 20, 30, 40, 50}),
	)
	indexes, err := filterIndexes(frame, In{
		Expr:   ColumnRef{Name: "sym"},
		Values: []any{"MSFT", Symbol("AAPL"), "MSFT"},
	})
	if err != nil {
		t.Fatalf("filterIndexes in returned error: %v", err)
	}
	if want := []int{0, 1, 2, 4}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("in filter indexes = %v, want %v", indexes, want)
	}

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Where: In{
			Expr:   ColumnRef{Name: "sym"},
			Values: []any{"NVDA", "AAPL"},
		},
		Select: []SelectItem{{Name: "qty", Expr: ColumnRef{Name: "qty"}}},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec in returned error: %v", err)
	}
	assertColumnValues(t, got, "qty", []any{int64(10), int64(30), int64(40)})
}

func TestFilterIndexesUsesTypedInKernelWithoutAttributeIndex(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30, 20, 40})},
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "NVDA", "AAPL", "IBM"})},
	)

	indexes, err := filterIndexes(frame, In{
		Expr:   ColumnRef{Name: "qty"},
		Values: []any{int64(20), int32(40)},
	})
	if err != nil {
		t.Fatalf("filterIndexes typed in returned error: %v", err)
	}
	if want := []int{1, 3, 4}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("typed in filter indexes = %v, want %v", indexes, want)
	}

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Where: In{
			Expr:   ColumnRef{Name: "sym"},
			Values: []any{"AAPL", Symbol("IBM")},
		},
		Select: []SelectItem{{Name: "qty", Expr: ColumnRef{Name: "qty"}}},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec typed in returned error: %v", err)
	}
	assertColumnValues(t, got, "qty", []any{int32(10), int32(20), int32(40)})
}

func TestFilterIndexesFusesLogicalTypedIndexes(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "active", Data: NewBool([]bool{true, false, true, true, false})},
		Column{Name: "price", Data: NewF64([]float64{99, 101, 102, 88, 120})},
	)

	indexes, err := filterIndexes(frame, Logical{
		Op:    "and",
		Left:  Binary{Op: OpEQ, Left: ColumnRef{Name: "active"}, Right: Literal{Value: true}},
		Right: Binary{Op: OpGE, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 100.0}},
	})
	if err != nil {
		t.Fatalf("filterIndexes and returned error: %v", err)
	}
	if want := []int{2}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("and filter indexes = %v, want %v", indexes, want)
	}

	indexes, err = filterIndexes(frame, Logical{
		Op:    "or",
		Left:  Binary{Op: OpEQ, Left: ColumnRef{Name: "active"}, Right: Literal{Value: true}},
		Right: Binary{Op: OpGE, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 100.0}},
	})
	if err != nil {
		t.Fatalf("filterIndexes or returned error: %v", err)
	}
	if want := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("or filter indexes = %v, want %v", indexes, want)
	}
}

func TestExecFilteredGroupedCountUsesAttributeIndexOrder(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"}), ArrayAttributeGrouped)},
		NewColumn("qty", []any{5, 20, 30, 40, 9}),
	)
	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpGT, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(10)}},
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "rows", Func: "count"},
			{Name: "fills", Func: "count"},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec filtered grouped count returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "rows", "fills"})
	assertColumnValues(t, got, "sym", []any{Symbol("MSFT"), Symbol("AAPL"), Symbol("NVDA")})
	assertColumnValues(t, got, "rows", []any{int64(1), int64(1), int64(1)})
	assertColumnValues(t, got, "fills", []any{int64(1), int64(1), int64(1)})
}

func TestQueryKernelSupportsVectorTransformProjectionInFilteredOrder(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("qty", []any{5, 20, 30, 40}),
		NewColumn("px", []any{100.0, 101.0, 80.0, 103.0}),
	)
	plan := QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpGT, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(10)}},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "prev_px", Expr: VectorTransformExpr{Func: "prev", Expr: ColumnRef{Name: "px"}}},
			{Name: "running_qty", Expr: VectorTransformExpr{Func: "sums", Expr: ColumnRef{Name: "qty"}}},
		},
		LimitN: -1,
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		_, reason := QueryKernelSupportReason(plan)
		t.Fatalf("CompileQueryKernel ok = false, reason: %s", reason)
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("kernel Exec returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "prev_px", "running_qty"})
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")})
	assertColumnValues(t, got, "prev_px", []any{NullValue, 101.0, 80.0})
	assertColumnValues(t, got, "running_qty", []any{20.0, 50.0, 90.0})
}

func TestEncodedSymbolsExposeDomainAndCodesWhileDecodingValues(t *testing.T) {
	array := NewEncodedSymbols([]Symbol{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"})
	if got := array.Kind(); got != KindSymbol {
		t.Fatalf("kind = %s, want %s", got, KindSymbol)
	}
	if got, want := array.Values(), []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL"), Symbol("NVDA"), Symbol("MSFT")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("values = %#v, want %#v", got, want)
	}
	domain, ok := EncodedDomainOf(array)
	if !ok {
		t.Fatal("EncodedDomainOf returned ok=false")
	}
	if want := []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("NVDA")}; !reflect.DeepEqual(domain, want) {
		t.Fatalf("domain = %#v, want %#v", domain, want)
	}
	codes, ok := EncodedCodesOf(array)
	if !ok {
		t.Fatal("EncodedCodesOf returned ok=false")
	}
	if want := []int32{0, 1, 0, 2, 1}; !reflect.DeepEqual(codes, want) {
		t.Fatalf("codes = %#v, want %#v", codes, want)
	}
	gathered := array.Gather([]int{4, 2, 3})
	if got, want := gathered.Values(), []any{Symbol("MSFT"), Symbol("AAPL"), Symbol("NVDA")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gathered values = %#v, want %#v", got, want)
	}
	gatheredDomain, _ := EncodedDomainOf(gathered)
	if !reflect.DeepEqual(gatheredDomain, domain) {
		t.Fatalf("gathered domain = %#v, want preserved %#v", gatheredDomain, domain)
	}
	gatheredCodes, _ := EncodedCodesOf(gathered)
	if want := []int32{1, 0, 2}; !reflect.DeepEqual(gatheredCodes, want) {
		t.Fatalf("gathered codes = %#v, want %#v", gatheredCodes, want)
	}
}

func TestInferArrayRetainsTypedKindsWithNulls(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		kind   Kind
		want   []any
	}{
		{name: "bool", values: []any{true, nil, false}, kind: KindBool, want: []any{true, NullValue, false}},
		{name: "i8", values: []any{int8(1), nil, int8(3)}, kind: KindI8, want: []any{int8(1), NullValue, int8(3)}},
		{name: "i16", values: []any{int16(1), nil, int16(3)}, kind: KindI16, want: []any{int16(1), NullValue, int16(3)}},
		{name: "i32", values: []any{int32(1), nil, int32(3)}, kind: KindI32, want: []any{int32(1), NullValue, int32(3)}},
		{name: "i64", values: []any{1, nil, int64(3)}, kind: KindI64, want: []any{int64(1), NullValue, int64(3)}},
		{name: "u8", values: []any{uint8(1), nil, uint8(3)}, kind: KindU8, want: []any{uint8(1), NullValue, uint8(3)}},
		{name: "u16", values: []any{uint16(1), nil, uint16(3)}, kind: KindU16, want: []any{uint16(1), NullValue, uint16(3)}},
		{name: "u32", values: []any{uint32(1), nil, uint32(3)}, kind: KindU32, want: []any{uint32(1), NullValue, uint32(3)}},
		{name: "u64", values: []any{uint64(1), nil, uint64(3)}, kind: KindU64, want: []any{uint64(1), NullValue, uint64(3)}},
		{name: "f32", values: []any{float32(1.5), nil, float32(2.5)}, kind: KindF32, want: []any{float32(1.5), NullValue, float32(2.5)}},
		{name: "f64", values: []any{1.5, nil, float32(2.5)}, kind: KindF64, want: []any{1.5, NullValue, 2.5}},
		{name: "string", values: []any{"a", nil, "b"}, kind: KindString, want: []any{"a", NullValue, "b"}},
		{name: "symbol", values: []any{Symbol("a"), nil, Symbol("b")}, kind: KindSymbol, want: []any{Symbol("a"), NullValue, Symbol("b")}},
		{name: "month", values: []any{MonthFromMonths(1), nil, MonthFromMonths(3)}, kind: KindMonth, want: []any{Month(1), NullValue, Month(3)}},
		{name: "date", values: []any{DateFromDays(1), nil, DateFromDays(3)}, kind: KindDate, want: []any{Date(1), NullValue, Date(3)}},
		{name: "datetime", values: []any{DateTimeFromUnixNanos(100), nil, DateTimeFromUnixNanos(300)}, kind: KindDateTime, want: []any{DateTime(100), NullValue, DateTime(300)}},
		{name: "timespan", values: []any{TimespanFromNanos(100), nil, TimespanFromNanos(300)}, kind: KindTimespan, want: []any{Timespan(100), NullValue, Timespan(300)}},
		{name: "minute", values: []any{MinuteFromMinutes(10), nil, MinuteFromMinutes(30)}, kind: KindMinute, want: []any{Minute(10), NullValue, Minute(30)}},
		{name: "second", values: []any{SecondFromSeconds(10), nil, SecondFromSeconds(30)}, kind: KindSecond, want: []any{Second(10), NullValue, Second(30)}},
		{name: "time", values: []any{TimeFromNanos(10), nil, TimeFromNanos(30)}, kind: KindTime, want: []any{Time(10), NullValue, Time(30)}},
		{name: "timestamp", values: []any{TimestampFromUnixNanos(100), nil, TimestampFromUnixNanos(300)}, kind: KindTimestamp, want: []any{Timestamp(100), NullValue, Timestamp(300)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := NewColumn(Symbol(tt.name), tt.values)
			if got := col.Data.Kind(); got != tt.kind {
				t.Fatalf("kind = %s, want %s", got, tt.kind)
			}
			if got := col.Data.Values(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("values = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizeValueForKindAcceptsIntegerByteInputs(t *testing.T) {
	for _, input := range []any{int(1), int64(2), int32(3), int16(4), int8(5), uint8(6)} {
		got, err := NormalizeValueForKind(KindU8, input)
		if err != nil {
			t.Fatalf("NormalizeValueForKind(u8, %#v) returned error: %v", input, err)
		}
		if _, ok := got.(uint8); !ok {
			t.Fatalf("NormalizeValueForKind(u8, %#v) = %T, want uint8", input, got)
		}
	}
	for _, input := range []any{-1, 256, 1.5} {
		if _, err := NormalizeValueForKind(KindU8, input); err == nil {
			t.Fatalf("NormalizeValueForKind(u8, %#v) returned nil error", input)
		}
	}
}

func TestTakePreservesTypedNullableKindsAndSchema(t *testing.T) {
	ts, err := NewColumnWithKind("ts", KindTimestamp, []any{
		TimestampFromUnixNanos(100),
		nil,
		TimestampFromUnixNanos(300),
	})
	if err != nil {
		t.Fatalf("NewColumnWithKind ts returned error: %v", err)
	}
	px, err := NewColumnWithKind("px", KindF64, []any{nil, 10.5, 11.5})
	if err != nil {
		t.Fatalf("NewColumnWithKind px returned error: %v", err)
	}
	frame := mustFrame(t, ts, px)

	takenArray, err := Take(ts.Data, 2)
	if err != nil {
		t.Fatalf("Take returned error: %v", err)
	}
	if got := takenArray.Kind(); got != KindTimestamp {
		t.Fatalf("taken array kind = %s, want %s", got, KindTimestamp)
	}
	if got, want := takenArray.Values(), []any{Timestamp(100), NullValue}; !reflect.DeepEqual(got, want) {
		t.Fatalf("taken array values = %#v, want %#v", got, want)
	}

	takenFrame, err := TakeFrame(frame, 2)
	if err != nil {
		t.Fatalf("TakeFrame returned error: %v", err)
	}
	assertColumnNames(t, takenFrame, []Symbol{"ts", "px"})
	assertColumnValues(t, takenFrame, "ts", []any{Timestamp(100), NullValue})
	assertColumnValues(t, takenFrame, "px", []any{NullValue, 10.5})
	if got, ok := takenFrame.Schema().Kind("ts"); !ok || got != KindTimestamp {
		t.Fatalf("taken frame ts kind = %s, ok %v; want %s", got, ok, KindTimestamp)
	}
	if got, ok := takenFrame.Schema().Kind("px"); !ok || got != KindF64 {
		t.Fatalf("taken frame px kind = %s, ok %v; want %s", got, ok, KindF64)
	}
}

func TestNewColumnWithKindValidatesAndNormalizesNullableTypedValues(t *testing.T) {
	sym, err := NewColumnWithKind("sym", KindSymbol, []any{"a", nil, Symbol("b")})
	if err != nil {
		t.Fatalf("NewColumnWithKind symbol returned error: %v", err)
	}
	if got := sym.Data.Kind(); got != KindSymbol {
		t.Fatalf("symbol kind = %s, want %s", got, KindSymbol)
	}
	if got, want := sym.Data.Values(), []any{Symbol("a"), NullValue, Symbol("b")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("symbol values = %#v, want %#v", got, want)
	}

	i32, err := NewColumnWithKind("qty", KindI32, []any{1, nil, int32(3)})
	if err != nil {
		t.Fatalf("NewColumnWithKind i32 returned error: %v", err)
	}
	if got, want := i32.Data.Values(), []any{int32(1), NullValue, int32(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("i32 values = %#v, want %#v", got, want)
	}

	if _, err := NewColumnWithKind("bad", KindTimestamp, []any{TimestampFromUnixNanos(1), nil, int64(3)}); err == nil {
		t.Fatal("NewColumnWithKind accepted incompatible nullable timestamp value")
	}
	if _, err := NewColumnWithKind("bad", KindTimestamp, []any{TimestampFromUnixNanos(1), nil, "1970-01-01T00:00:00Z"}); err == nil {
		t.Fatal("NewColumnWithKind accepted string timestamp value")
	}
	if _, err := NewColumnWithKind("bad", KindDate, []any{DateFromDays(1), "1970-01-02"}); err == nil {
		t.Fatal("NewColumnWithKind accepted string date value")
	}
	if _, err := NewColumnWithKind("bad", KindI8, []any{int64(128), nil}); err == nil {
		t.Fatal("NewColumnWithKind accepted overflowing nullable i8 value")
	}
}

func TestInferArrayUsesTypedNullKindHints(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		kind   Kind
		want   []any
	}{
		{
			name:   "i32 hint narrows compatible integers",
			values: []any{int64(1), NullForKind(KindI32), int32(3)},
			kind:   KindI32,
			want:   []any{int32(1), NullValue, int32(3)},
		},
		{
			name:   "f32 hint promotes compatible integers",
			values: []any{int64(1), int64(2), NullForKind(KindF32)},
			kind:   KindF32,
			want:   []any{float32(1), float32(2), NullValue},
		},
		{
			name:   "f64 hint promotes compatible integers",
			values: []any{int64(1), int64(2), NullForKind(KindF64)},
			kind:   KindF64,
			want:   []any{1.0, 2.0, NullValue},
		},
		{
			name:   "temporal hint preserves all-null kind",
			values: []any{NullForKind(KindTimestamp), NullForKind(KindTimestamp)},
			kind:   KindTimestamp,
			want:   []any{NullValue, NullValue},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferArray(tt.values)
			if got.Kind() != tt.kind {
				t.Fatalf("InferArray kind = %s, want %s", got.Kind(), tt.kind)
			}
			if values := got.Values(); !reflect.DeepEqual(values, tt.want) {
				t.Fatalf("InferArray values = %#v, want %#v", values, tt.want)
			}
		})
	}
}

func TestInferArrayNumericPromotionBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		kind   Kind
		want   []any
	}{
		{
			name:   "i16 i32 promotes to f64 fallback",
			values: []any{int16(1), int32(2)},
			kind:   KindF64,
			want:   []any{1.0, 2.0},
		},
		{
			name:   "i32 i64 promotes to f64 fallback",
			values: []any{int32(1), int64(2)},
			kind:   KindF64,
			want:   []any{1.0, 2.0},
		},
		{
			name:   "i64 f32 promotes to f64",
			values: []any{int64(1), float32(2.5)},
			kind:   KindF64,
			want:   []any{1.0, 2.5},
		},
		{
			name:   "f32 f64 promotes to f64",
			values: []any{float32(1.5), float64(2.5)},
			kind:   KindF64,
			want:   []any{1.5, 2.5},
		},
		{
			name:   "typed f32 null hints compatible integer nullable",
			values: []any{int32(1), NullForKind(KindF32)},
			kind:   KindF32,
			want:   []any{float32(1), NullValue},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferArray(tt.values)
			if got.Kind() != tt.kind {
				t.Fatalf("InferArray kind = %s, want %s", got.Kind(), tt.kind)
			}
			if values := got.Values(); !reflect.DeepEqual(values, tt.want) {
				t.Fatalf("InferArray values = %#v, want %#v", values, tt.want)
			}
		})
	}
}

func TestApplyBinaryTypedNullPromotionAndTypedCompareConsistency(t *testing.T) {
	got, err := ApplyBinary(OpAdd, NullForKind(KindI32), int16(1))
	if err != nil {
		t.Fatalf("ApplyBinary typed i32+i16 returned error: %v", err)
	}
	if got != NullForKind(KindI64) {
		t.Fatalf("ApplyBinary typed i32+i16 = %#v, want typed i64 null", got)
	}

	got, err = ApplyBinary(OpMul, float32(2), NullForKind(KindF64))
	if err != nil {
		t.Fatalf("ApplyBinary f32*typed f64 returned error: %v", err)
	}
	if got != NullForKind(KindF64) {
		t.Fatalf("ApplyBinary f32*typed f64 = %#v, want typed f64 null", got)
	}

	symbolMask, err := CompareMask(NewSymbols([]string{"AAPL", "MSFT"}), OpEQ, "MSFT")
	if err != nil {
		t.Fatalf("symbol CompareMask returned error: %v", err)
	}
	if values := symbolMask.Values(); !reflect.DeepEqual(values, []any{false, true}) {
		t.Fatalf("symbol/string CompareMask = %#v, want [false true]", values)
	}
	stringMask, err := CompareMask(NewString([]string{"AAPL", "MSFT"}), OpGT, Symbol("IBM"))
	if err != nil {
		t.Fatalf("string CompareMask returned error: %v", err)
	}
	if values := stringMask.Values(); !reflect.DeepEqual(values, []any{false, true}) {
		t.Fatalf("string/symbol CompareMask = %#v, want [false true]", values)
	}
}

func TestNumericConstructors(t *testing.T) {
	tests := []struct {
		name string
		got  Array
		kind Kind
		want []any
	}{
		{name: "i8", got: NewI8([]int8{1, 2}), kind: KindI8, want: []any{int8(1), int8(2)}},
		{name: "i16", got: NewI16([]int16{1, 2}), kind: KindI16, want: []any{int16(1), int16(2)}},
		{name: "i32", got: NewI32([]int32{1, 2}), kind: KindI32, want: []any{int32(1), int32(2)}},
		{name: "u8", got: NewU8([]uint8{1, 2}), kind: KindU8, want: []any{uint8(1), uint8(2)}},
		{name: "u16", got: NewU16([]uint16{1, 2}), kind: KindU16, want: []any{uint16(1), uint16(2)}},
		{name: "u32", got: NewU32([]uint32{1, 2}), kind: KindU32, want: []any{uint32(1), uint32(2)}},
		{name: "u64", got: NewU64([]uint64{1, 2}), kind: KindU64, want: []any{uint64(1), uint64(2)}},
		{name: "f32", got: NewF32([]float32{1.5, 2.5}), kind: KindF32, want: []any{float32(1.5), float32(2.5)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got.Kind(); got != tt.kind {
				t.Fatalf("kind = %s, want %s", got, tt.kind)
			}
			if got := tt.got.Values(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("values = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTemporalConstructors(t *testing.T) {
	month := NewMonth([]Month{MonthFromMonths(1), MonthFromMonths(2)})
	if got := month.Kind(); got != KindMonth {
		t.Fatalf("month kind = %s, want %s", got, KindMonth)
	}
	if got, want := month.Values(), []any{Month(1), Month(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("month values = %#v, want %#v", got, want)
	}

	date := NewDate([]Date{DateFromDays(1), DateFromDays(2)})
	if got := date.Kind(); got != KindDate {
		t.Fatalf("date kind = %s, want %s", got, KindDate)
	}
	if got, want := date.Values(), []any{Date(1), Date(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("date values = %#v, want %#v", got, want)
	}

	datetime := NewDateTime([]DateTime{DateTimeFromUnixNanos(100), DateTimeFromUnixNanos(200)})
	if got := datetime.Kind(); got != KindDateTime {
		t.Fatalf("datetime kind = %s, want %s", got, KindDateTime)
	}
	if got, want := datetime.Values(), []any{DateTime(100), DateTime(200)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("datetime values = %#v, want %#v", got, want)
	}

	timespan := NewTimespan([]Timespan{TimespanFromNanos(100), TimespanFromNanos(200)})
	if got := timespan.Kind(); got != KindTimespan {
		t.Fatalf("timespan kind = %s, want %s", got, KindTimespan)
	}
	if got, want := timespan.Values(), []any{Timespan(100), Timespan(200)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("timespan values = %#v, want %#v", got, want)
	}

	minute := NewMinute([]Minute{MinuteFromMinutes(10), MinuteFromMinutes(20)})
	if got := minute.Kind(); got != KindMinute {
		t.Fatalf("minute kind = %s, want %s", got, KindMinute)
	}
	if !MinuteFromMinutes(20).Valid() {
		t.Fatal("expected in-day minute to be valid")
	}
	if MinuteFromMinutes(24 * 60).Valid() {
		t.Fatal("expected next-midnight minute to be invalid")
	}

	second := NewSecond([]Second{SecondFromSeconds(10), SecondFromSeconds(20)})
	if got := second.Kind(); got != KindSecond {
		t.Fatalf("second kind = %s, want %s", got, KindSecond)
	}
	if !SecondFromSeconds(20).Valid() {
		t.Fatal("expected in-day second to be valid")
	}
	if SecondFromSeconds(24 * 60 * 60).Valid() {
		t.Fatal("expected next-midnight second to be invalid")
	}

	time := NewTime([]Time{TimeFromNanos(10), TimeFromNanos(20)})
	if got := time.Kind(); got != KindTime {
		t.Fatalf("time kind = %s, want %s", got, KindTime)
	}
	if !TimeFromNanos(20).Valid() {
		t.Fatal("expected in-day time to be valid")
	}
	if TimeFromNanos(24 * 60 * 60 * 1_000_000_000).Valid() {
		t.Fatal("expected next-midnight time to be invalid")
	}

	timestamp := NewTimestamp([]Timestamp{TimestampFromUnixNanos(100), TimestampFromUnixNanos(200)})
	if got := timestamp.Kind(); got != KindTimestamp {
		t.Fatalf("timestamp kind = %s, want %s", got, KindTimestamp)
	}
	if got, want := timestamp.Values(), []any{Timestamp(100), Timestamp(200)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("timestamp values = %#v, want %#v", got, want)
	}
}

func TestColumnarFrameStoreRoundTripPreservesSchemaAndNulls(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), nil}),
		NewColumn("qty", []any{int64(10), nil, int64(30)}),
		NewColumn("price", []any{100.5, 101.25, nil}),
		NewColumn("ts", []any{TimestampFromUnixNanos(100), nil, TimestampFromUnixNanos(300)}),
	)
	dir := t.TempDir()
	if err := SaveFrameDir(dir, frame); err != nil {
		t.Fatalf("SaveFrameDir returned error: %v", err)
	}
	info, err := ReadFrameStoreInfo(dir)
	if err != nil {
		t.Fatalf("ReadFrameStoreInfo returned error: %v", err)
	}
	if got, want := info.Format, "leia-columnar-frame"; got != want {
		t.Fatalf("format = %q, want %q", got, want)
	}
	if got, want := info.Rows, 3; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	loaded, err := LoadFrameDir(dir)
	if err != nil {
		t.Fatalf("LoadFrameDir returned error: %v", err)
	}
	assertColumnNames(t, loaded, []Symbol{"sym", "qty", "price", "ts"})
	assertColumnValues(t, loaded, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), NullValue})
	assertColumnValues(t, loaded, "qty", []any{int64(10), NullValue, int64(30)})
	assertColumnValues(t, loaded, "price", []any{100.5, 101.25, NullValue})
	assertColumnValues(t, loaded, "ts", []any{Timestamp(100), NullValue, Timestamp(300)})
	if got, _ := loaded.Schema().Kind("ts"); got != KindTimestamp {
		t.Fatalf("ts kind = %s, want timestamp", got)
	}
}

func TestPartitionedFrameStoreLoadsMatchingPartitions(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("date", []any{Date(1), Date(1), Date(2), Date(2)}),
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("qty", []any{int64(10), int64(20), int64(30), int64(40)}),
	)
	dir := t.TempDir()
	if err := SavePartitionedFrameDir(dir, frame, "date", "sym"); err != nil {
		t.Fatalf("SavePartitionedFrameDir returned error: %v", err)
	}
	info, err := ReadPartitionedStoreInfo(dir)
	if err != nil {
		t.Fatalf("ReadPartitionedStoreInfo returned error: %v", err)
	}
	if got, want := len(info.Partitions), 4; got != want {
		t.Fatalf("partitions = %d, want %d", got, want)
	}
	loaded, err := LoadPartitionedFrameDir(dir, map[Symbol]any{"sym": Symbol("AAPL")})
	if err != nil {
		t.Fatalf("LoadPartitionedFrameDir returned error: %v", err)
	}
	assertColumnNames(t, loaded, []Symbol{"date", "sym", "qty"})
	assertColumnValues(t, loaded, "date", []any{Date(1), Date(2)})
	assertColumnValues(t, loaded, "sym", []any{Symbol("AAPL"), Symbol("AAPL")})
	assertColumnValues(t, loaded, "qty", []any{int64(10), int64(30)})

	empty, err := LoadPartitionedFrameDir(dir, map[Symbol]any{"sym": Symbol("IBM")})
	if err != nil {
		t.Fatalf("LoadPartitionedFrameDir empty returned error: %v", err)
	}
	if got := empty.Len(); got != 0 {
		t.Fatalf("empty filtered frame len = %d, want 0", got)
	}
	if got, _ := empty.Schema().Kind("qty"); got != KindI64 {
		t.Fatalf("empty filtered qty kind = %s, want i64", got)
	}
}

func TestBucketFloorNumericValues(t *testing.T) {
	bucketed, err := BucketFloor(NewColumn("x", []any{int64(-11), int64(-10), int64(-9), nil, int64(0), int64(9), int64(10)}).Data, int64(10))
	if err != nil {
		t.Fatalf("BucketFloor returned error: %v", err)
	}
	if got := bucketed.Kind(); got != KindI64 {
		t.Fatalf("bucket kind = %s, want %s", got, KindI64)
	}
	if got, want := bucketed.Values(), []any{int64(-20), int64(-10), int64(-10), NullValue, int64(0), int64(0), int64(10)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bucket values = %#v, want %#v", got, want)
	}

	rangeBucketed, err := BucketFloor(NewI64Range(-3, 1, 7), int64(2))
	if err != nil {
		t.Fatalf("BucketFloor range returned error: %v", err)
	}
	if got := rangeBucketed.Kind(); got != KindI64 {
		t.Fatalf("range bucket kind = %s, want %s", got, KindI64)
	}
	if got, want := rangeBucketed.Values(), []any{int64(-4), int64(-2), int64(-2), int64(0), int64(0), int64(2), int64(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range bucket values = %#v, want %#v", got, want)
	}
	if sum, ok, err := TryTypedNumericSum(rangeBucketed); err != nil || !ok || sum != int64(-4) {
		t.Fatalf("range bucket typed sum = %v,%v,%v; want -4,true,nil", sum, ok, err)
	}

	tilBucketed, err := BucketFloor(NewI64Range(0, 1, 8192), int64(60))
	if err != nil {
		t.Fatalf("BucketFloor til range returned error: %v", err)
	}
	if _, ok := tilBucketed.(i64BucketArray); !ok {
		t.Fatalf("BucketFloor til range returned %T, want i64BucketArray", tilBucketed)
	}
	if sum, ok, err := TryTypedNumericSum(tilBucketed); err != nil || !ok || sum != int64(33309120) {
		t.Fatalf("til bucket typed sum = %v,%v,%v; want 33309120,true,nil", sum, ok, err)
	}

	floatBucketed, err := BucketFloor(NewF64([]float64{-1.25, -1.0, -0.75, 0, 0.74, 0.75}), 0.5)
	if err != nil {
		t.Fatalf("BucketFloor float returned error: %v", err)
	}
	if got, want := floatBucketed.Values(), []any{-1.5, -1.0, -1.0, 0.0, 0.5, 0.5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("float bucket values = %#v, want %#v", got, want)
	}

	if _, err := BucketFloor(NewString([]string{"a"}), int64(10)); err == nil {
		t.Fatal("BucketFloor accepted non-bucketable kind")
	}
	if _, err := BucketFloor(NewI64([]int64{1}), int64(0)); err == nil {
		t.Fatal("BucketFloor accepted non-positive interval")
	}
}

func TestBucketFloorTemporalValues(t *testing.T) {
	months, err := BucketFloor(NewMonth([]Month{MonthFromMonths(0), MonthFromMonths(1), MonthFromMonths(2), MonthFromMonths(3)}), int64(3))
	if err != nil {
		t.Fatalf("BucketFloor months returned error: %v", err)
	}
	if got, want := months.Values(), []any{Month(0), Month(0), Month(0), Month(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("month bucket values = %#v, want %#v", got, want)
	}

	dates, err := BucketFloor(NewColumn("d", []any{DateFromDays(0), DateFromDays(1), DateFromDays(6), DateFromDays(7), nil}).Data, int64(7))
	if err != nil {
		t.Fatalf("BucketFloor dates returned error: %v", err)
	}
	if got := dates.Kind(); got != KindDate {
		t.Fatalf("date bucket kind = %s, want %s", got, KindDate)
	}
	if got, want := dates.Values(), []any{Date(0), Date(0), Date(0), Date(7), NullValue}; !reflect.DeepEqual(got, want) {
		t.Fatalf("date bucket values = %#v, want %#v", got, want)
	}

	minutes, err := BucketFloor(NewMinute([]Minute{
		MinuteFromMinutes(59),
		MinuteFromMinutes(60),
		MinuteFromMinutes(119),
	}), int64(60))
	if err != nil {
		t.Fatalf("BucketFloor minutes returned error: %v", err)
	}
	if got, want := minutes.Values(), []any{Minute(0), Minute(60), Minute(60)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("minute bucket values = %#v, want %#v", got, want)
	}

	seconds, err := BucketFloor(NewSecond([]Second{
		SecondFromSeconds(59),
		SecondFromSeconds(60),
		SecondFromSeconds(119),
	}), int64(60))
	if err != nil {
		t.Fatalf("BucketFloor seconds returned error: %v", err)
	}
	if got, want := seconds.Values(), []any{Second(0), Second(60), Second(60)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second bucket values = %#v, want %#v", got, want)
	}

	times, err := BucketFloor(NewTime([]Time{
		TimeFromNanos(999),
		TimeFromNanos(1_000),
		TimeFromNanos(1_999),
	}), int64(1_000))
	if err != nil {
		t.Fatalf("BucketFloor times returned error: %v", err)
	}
	if got, want := times.Values(), []any{Time(0), Time(1000), Time(1000)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("time bucket values = %#v, want %#v", got, want)
	}

	timespans, err := BucketFloor(NewTimespan([]Timespan{
		TimespanFromNanos(-1),
		TimespanFromNanos(0),
		TimespanFromNanos(999),
		TimespanFromNanos(1_000),
	}), int64(1_000))
	if err != nil {
		t.Fatalf("BucketFloor timespans returned error: %v", err)
	}
	if got, want := timespans.Values(), []any{Timespan(-1000), Timespan(0), Timespan(0), Timespan(1000)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("timespan bucket values = %#v, want %#v", got, want)
	}

	datetimes, err := BucketFloor(NewDateTime([]DateTime{
		DateTimeFromUnixNanos(-1),
		DateTimeFromUnixNanos(0),
		DateTimeFromUnixNanos(999),
		DateTimeFromUnixNanos(1_000),
	}), int64(1_000))
	if err != nil {
		t.Fatalf("BucketFloor datetimes returned error: %v", err)
	}
	if got, want := datetimes.Values(), []any{DateTime(-1000), DateTime(0), DateTime(0), DateTime(1000)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("datetime bucket values = %#v, want %#v", got, want)
	}

	timestamps, err := BucketFloor(NewTimestamp([]Timestamp{
		TimestampFromUnixNanos(-1),
		TimestampFromUnixNanos(0),
		TimestampFromUnixNanos(999),
		TimestampFromUnixNanos(1_000),
	}), int64(1_000))
	if err != nil {
		t.Fatalf("BucketFloor timestamps returned error: %v", err)
	}
	if got, want := timestamps.Values(), []any{Timestamp(-1000), Timestamp(0), Timestamp(0), Timestamp(1000)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("timestamp bucket values = %#v, want %#v", got, want)
	}
}

func TestBucketFloorNullableTimePreservesKindAndNull(t *testing.T) {
	times, err := BucketFloor(NewColumn("time", []any{
		nil,
		TimeFromNanos(59_999_999_999),
		TimeFromNanos(60_000_000_000),
		TimeFromNanos(119_999_999_999),
	}).Data, int64(60_000_000_000))
	if err != nil {
		t.Fatalf("BucketFloor nullable times returned error: %v", err)
	}
	if got := times.Kind(); got != KindTime {
		t.Fatalf("time bucket kind = %s, want %s", got, KindTime)
	}
	if got, want := times.Values(), []any{NullValue, Time(0), Time(60_000_000_000), Time(60_000_000_000)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("time bucket values = %#v, want %#v", got, want)
	}
}

func TestBucketFloorExprEvalRowsPreservesTemporalKindAndNull(t *testing.T) {
	ts, err := NewColumnWithKind("ts", KindTimestamp, []any{
		TimestampFromUnixNanos(59_999_999_999),
		NullForKind(KindTimestamp),
		TimestampFromUnixNanos(60_000_000_000),
		TimestampFromUnixNanos(119_999_999_999),
	})
	if err != nil {
		t.Fatalf("NewColumnWithKind returned error: %v", err)
	}
	frame := mustFrame(t, ts)
	expr := BucketFloorExpr{
		Expr:     ColumnRef{Name: "ts"},
		Interval: TimespanFromNanos(60_000_000_000),
	}
	array, err := expr.EvalRows(frame, []int{0, 1, 2, 3})
	if err != nil {
		t.Fatalf("EvalRows returned error: %v", err)
	}
	if got := array.Kind(); got != KindTimestamp {
		t.Fatalf("bucket kind = %s, want %s", got, KindTimestamp)
	}
	if got, want := array.Values(), []any{
		TimestampFromUnixNanos(0),
		NullValue,
		TimestampFromUnixNanos(60_000_000_000),
		TimestampFromUnixNanos(60_000_000_000),
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bucket values = %#v, want %#v", got, want)
	}
}

func TestBucketFloorTemporalIntervalsUseColumnUnits(t *testing.T) {
	seconds, err := BucketFloor(NewSecond([]Second{
		SecondFromSeconds(34_215),
		SecondFromSeconds(34_259),
		SecondFromSeconds(34_260),
	}), MinuteFromMinutes(1))
	if err != nil {
		t.Fatalf("BucketFloor seconds by minute returned error: %v", err)
	}
	if got, want := seconds.Values(), []any{Second(34_200), Second(34_200), Second(34_260)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second minute bucket values = %#v, want %#v", got, want)
	}

	times, err := BucketFloor(NewTime([]Time{
		TimeFromNanos(34_215_000_000_000),
		TimeFromNanos(34_260_000_000_000),
	}), MinuteFromMinutes(1))
	if err != nil {
		t.Fatalf("BucketFloor times by minute returned error: %v", err)
	}
	if got, want := times.Values(), []any{Time(34_200_000_000_000), Time(34_260_000_000_000)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("time minute bucket values = %#v, want %#v", got, want)
	}

	timestamps, err := BucketFloor(NewTimestamp([]Timestamp{
		TimestampFromUnixNanos(34_215_000_000_000),
		TimestampFromUnixNanos(34_260_000_000_000),
	}), TimespanFromNanos(60_000_000_000))
	if err != nil {
		t.Fatalf("BucketFloor timestamps by timespan returned error: %v", err)
	}
	if got, want := timestamps.Values(), []any{Timestamp(34_200_000_000_000), Timestamp(34_260_000_000_000)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("timestamp timespan bucket values = %#v, want %#v", got, want)
	}

	dates, err := BucketFloor(NewDate([]Date{
		DateFromDays(19724),
		DateFromDays(19730),
		DateFromDays(19731),
	}), TimespanFromNanos(7*nanosPerDay))
	if err != nil {
		t.Fatalf("BucketFloor dates by timespan returned error: %v", err)
	}
	if got, want := dates.Values(), []any{Date(19719), Date(19726), Date(19726)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("date timespan bucket values = %#v, want %#v", got, want)
	}

	if _, err := BucketFloor(NewMinute([]Minute{MinuteFromMinutes(1)}), SecondFromSeconds(30)); err == nil {
		t.Fatal("BucketFloor accepted sub-minute interval for minute column")
	}
}

func TestInferArrayNullAndMixedKinds(t *testing.T) {
	nulls := NewColumn("missing", []any{nil, NullValue})
	if got := nulls.Data.Kind(); got != KindNull {
		t.Fatalf("all-null kind = %s, want %s", got, KindNull)
	}
	if got, want := nulls.Data.Values(), []any{NullValue, NullValue}; !reflect.DeepEqual(got, want) {
		t.Fatalf("all-null values = %#v, want %#v", got, want)
	}

	mixed := NewColumn("mixed", []any{"a", nil, Symbol("a")})
	if got := mixed.Data.Kind(); got != KindSymbol {
		t.Fatalf("mixed string/symbol kind = %s, want %s", got, KindSymbol)
	}
	if got, want := mixed.Data.Values(), []any{Symbol("a"), NullValue, Symbol("a")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed string/symbol values = %#v, want %#v", got, want)
	}
}

func TestFoundationNumericEqualityAndComparison(t *testing.T) {
	tests := []struct {
		name        string
		op          Op
		left, right any
		want        any
	}{
		{name: "i32 less", op: OpLT, left: int32(-2), right: int32(3), want: true},
		{name: "i32 equal i32", op: OpEQ, left: int32(7), right: int32(7), want: true},
		{name: "i32 not equal i64", op: OpEQ, left: int32(7), right: int64(7), want: false},
		{name: "u64 greater preserves integer order", op: OpGT, left: uint64(1 << 63), right: uint64(1<<63 - 1), want: true},
		{name: "f32 less or equal", op: OpLE, left: float32(1.25), right: float32(1.5), want: true},
		{name: "f32 equal f32", op: OpEQ, left: float32(2.5), right: float32(2.5), want: true},
		{name: "f32 not equal f64", op: OpEQ, left: float32(2.5), right: float64(2.5), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyBinary(tt.op, tt.left, tt.right)
			if err != nil {
				t.Fatalf("ApplyBinary returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ApplyBinary = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTemporalEqualityAndComparison(t *testing.T) {
	tests := []struct {
		name        string
		op          Op
		left, right any
		want        any
	}{
		{name: "date equal", op: OpEQ, left: DateFromDays(2), right: DateFromDays(2), want: true},
		{name: "date not equal to i64", op: OpEQ, left: DateFromDays(2), right: int64(2), want: false},
		{name: "month less", op: OpLT, left: MonthFromMonths(1), right: MonthFromMonths(2), want: true},
		{name: "datetime less", op: OpLT, left: DateTimeFromUnixNanos(1), right: DateTimeFromUnixNanos(2), want: true},
		{name: "timespan less", op: OpLT, left: TimespanFromNanos(1), right: TimespanFromNanos(2), want: true},
		{name: "minute less", op: OpLT, left: MinuteFromMinutes(1), right: MinuteFromMinutes(2), want: true},
		{name: "second less", op: OpLT, left: SecondFromSeconds(1), right: SecondFromSeconds(2), want: true},
		{name: "time less", op: OpLT, left: TimeFromNanos(1), right: TimeFromNanos(2), want: true},
		{name: "timestamp greater or equal", op: OpGE, left: TimestampFromUnixNanos(5), right: TimestampFromUnixNanos(5), want: true},
		{name: "null equal null", op: OpEQ, left: nil, right: NullValue, want: true},
		{name: "null not equal date", op: OpNE, left: nil, right: DateFromDays(0), want: true},
		{name: "null arithmetic propagates", op: OpSub, left: nil, right: 1.5, want: NullValue},
		{name: "null ordered comparison is false", op: OpGT, left: nil, right: 0.35, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyBinary(tt.op, tt.left, tt.right)
			if err != nil {
				t.Fatalf("ApplyBinary returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ApplyBinary = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSymbolAndStringAreDistinctScalars(t *testing.T) {
	got, err := ApplyBinary(OpEQ, Symbol("a"), "a")
	if err != nil {
		t.Fatalf("ApplyBinary returned error: %v", err)
	}
	if got != false {
		t.Fatalf("Symbol(\"a\") = \"a\" returned %v, want false", got)
	}
}

func TestQueryBoolComparisonAndGrouping(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("active", []any{true, false, true, nil}),
		NewColumn("qty", []any{2, 5, 3, 7}),
	)

	filtered, err := From(frame).
		WhereEq("active", true).
		SelectColumns("qty").
		Exec()
	if err != nil {
		t.Fatalf("filtered Exec returned error: %v", err)
	}
	assertColumnValues(t, filtered, "qty", []any{int64(2), int64(3)})

	grouped, err := From(frame).
		GroupBy("active").
		Count("n").
		OrderByColumn("active", Asc).
		Exec()
	if err != nil {
		t.Fatalf("grouped Exec returned error: %v", err)
	}
	assertColumnValues(t, grouped, "active", []any{NullValue, false, true})
	assertColumnValues(t, grouped, "n", []any{int64(1), int64(1), int64(2)})
}

func TestQueryFilteredSingleColumnGroupBuildsTypedIndex(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("NVDA"), nil, Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		Column{Name: "qty", Data: NewI64([]int64{-1, 10, 20, 30, 40})},
		Column{Name: "px", Data: NewF64([]float64{1, 100, 10, 20, 30})},
		Column{Name: "venue", Data: NewString([]string{"XNAS", "XASE", "XNYS", "ARCX", "BATS"})},
	)
	plan := QueryPlan{
		Where: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(10)}},
		By:    []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "fills", Func: "count"},
			{Name: "qty_sum", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "px_avg", Func: "avg", Expr: ColumnRef{Name: "px"}},
			{Name: "px_min", Func: "min", Expr: ColumnRef{Name: "px"}},
			{Name: "px_max", Func: "max", Expr: ColumnRef{Name: "px"}},
			{Name: "first_venue", Func: "first", Expr: ColumnRef{Name: "venue"}},
			{Name: "last_venue", Func: "last", Expr: ColumnRef{Name: "venue"}},
		},
		LimitN: -1,
	}
	got, err := Exec(frame, plan)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{NullValue, Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, got, "fills", []any{int64(1), int64(2), int64(1)})
	assertColumnValues(t, got, "qty_sum", []any{10.0, 60.0, 30.0})
	assertColumnValues(t, got, "px_avg", []any{100.0, 20.0, 20.0})
	assertColumnValues(t, got, "px_min", []any{100.0, 10.0, 20.0})
	assertColumnValues(t, got, "px_max", []any{100.0, 30.0, 20.0})
	assertColumnValues(t, got, "first_venue", []any{"XASE", "XNYS", "ARCX"})
	assertColumnValues(t, got, "last_venue", []any{"XASE", "BATS", "ARCX"})
}

func TestQueryTypedFilterProjectRangePreservesColumnarSemantics(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "id", Data: NewI64([]int64{0, 1, 2, 3, 4, 5})},
		Column{Name: "sym", Data: NewSymbols([]string{"a", "b", "c", "d", "e", "f"})},
		Column{Name: "px", Data: NewF64([]float64{10, 11, 12, 13, 14, 15})},
	)
	got, err := Exec(frame, QueryPlan{
		Where: Binary{Op: OpGE, Left: ColumnRef{Name: "id"}, Right: Literal{Value: int64(2)}},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "px", Expr: ColumnRef{Name: "px"}},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("c"), Symbol("d"), Symbol("e"), Symbol("f")})
	assertColumnValues(t, got, "px", []any{12.0, 13.0, 14.0, 15.0})
}

func TestQueryFoundationNumericComparisonAndOrder(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("qty", []any{int32(3), nil, int32(1), int32(3)}),
		NewColumn("seq", []any{uint64(3), uint64(1 << 63), uint64(2), uint64(1<<63 - 1)}),
		NewColumn("score", []any{float32(2.5), float32(1.5), float32(3.5), float32(2.5)}),
	)

	filtered, err := From(frame).
		WhereEq("qty", int32(3)).
		SelectColumns("seq", "score").
		OrderByColumn("seq", Asc).
		Exec()
	if err != nil {
		t.Fatalf("filtered Exec returned error: %v", err)
	}
	assertColumnValues(t, filtered, "seq", []any{uint64(3), uint64(1<<63 - 1)})
	assertColumnValues(t, filtered, "score", []any{float32(2.5), float32(2.5)})

	ordered, err := From(frame).
		SelectColumns("score", "qty").
		OrderByColumn("score", Desc).
		Exec()
	if err != nil {
		t.Fatalf("ordered Exec returned error: %v", err)
	}
	assertColumnValues(t, ordered, "score", []any{float32(3.5), float32(2.5), float32(2.5), float32(1.5)})
	assertColumnValues(t, ordered, "qty", []any{int32(1), int32(3), int32(3), NullValue})
}

func TestTypedMaskKernelsAndFastWhere(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "qty", Data: NewI32([]int32{1, 5, 9, 5})},
		Column{Name: "sym", Data: NewSymbols([]string{"a", "b", "a", "c"})},
		NewColumn("score", []any{1.5, nil, 3.5, 4.5}),
	)

	eq, err := EqualMask(mustColumn(t, frame, "qty"), int32(5))
	if err != nil {
		t.Fatalf("EqualMask returned error: %v", err)
	}
	if got, want := eq.Values(), []any{false, true, false, true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EqualMask values = %#v, want %#v", got, want)
	}

	within, err := WithinMask(mustColumn(t, frame, "score"), 1.5, 4.0, true)
	if err != nil {
		t.Fatalf("WithinMask returned error: %v", err)
	}
	if got, want := within.Values(), []any{true, false, true, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WithinMask values = %#v, want %#v", got, want)
	}

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpLE, Left: Literal{Value: int32(5)}, Right: ColumnRef{Name: "qty"}},
		Select: []SelectItem{{Name: "sym", Expr: ColumnRef{Name: "sym"}}},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec with reversed literal comparison returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("b"), Symbol("a"), Symbol("c")})
}

func TestTypedDyadicKernelsInFastWhereAndProjection(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "qty", Data: NewI32([]int32{1, 5, 9, 5})},
		Column{Name: "limit", Data: NewI64([]int64{2, 5, 8, 7})},
		NewColumn("px", []any{float64(10), nil, float64(30), float64(40)}),
		Column{Name: "sym", Data: NewSymbols([]string{"a", "b", "c", "d"})},
		Column{Name: "ts", Data: NewDate([]Date{DateFromDays(1), DateFromDays(2), DateFromDays(3), DateFromDays(4)})},
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpLE, Left: ColumnRef{Name: "qty"}, Right: ColumnRef{Name: "limit"}},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "notional", Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "qty"}, Right: ColumnRef{Name: "px"}}},
			{Name: "bumped", Expr: Binary{Op: OpAdd, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(10)}}},
			{Name: "recent", Expr: Binary{Op: OpGE, Left: ColumnRef{Name: "ts"}, Right: Literal{Value: DateFromDays(2)}}},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec with typed dyadic kernels returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("d")})
	assertColumnValues(t, got, "notional", []any{10.0, NullValue, 200.0})
	assertColumnValues(t, got, "bumped", []any{11.0, 15.0, 15.0})
	assertColumnValues(t, got, "recent", []any{false, true, true})
}

func TestDivideExpressionsInWhereProjectionOrderAndUpdate(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		Column{Name: "price", Data: NewF64([]float64{100.0, 90.0, 120.0})},
		Column{Name: "qty", Data: NewI64([]int64{10, 30, 20})},
	)

	ratio := Binary{Op: OpDiv, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "qty"}}
	got, err := Exec(frame, QueryPlan{
		Source:  frame,
		Where:   Binary{Op: OpGE, Left: ratio, Right: Literal{Value: 6.0}},
		Select:  []SelectItem{{Name: "sym", Expr: ColumnRef{Name: "sym"}}, {Name: "ratio", Expr: ratio}},
		OrderBy: []OrderSpec{{Column: "ratio", Desc: true}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("Exec divide query returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("NVDA")})
	assertColumnValues(t, got, "ratio", []any{10.0, 6.0})

	updated, err := UpdateWhere(frame,
		Binary{Op: OpGE, Left: ratio, Right: Literal{Value: 6.0}},
		map[Symbol]Expr{"ratio": ratio},
	)
	if err != nil {
		t.Fatalf("UpdateWhere divide returned error: %v", err)
	}
	assertColumnValues(t, updated, "ratio", []any{10.0, NullValue, 6.0})
}

func TestNullableComparisonAndWithinBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		array Array
		op    Op
		value any
		want  []any
	}{
		{
			name:  "string ne skips null equality",
			array: NewColumn("s", []any{"a", nil, "b"}).Data,
			op:    OpNE,
			value: "a",
			want:  []any{false, true, true},
		},
		{
			name:  "symbol le preserves symbol kind",
			array: NewColumn("sym", []any{Symbol("a"), NullValue, Symbol("c")}).Data,
			op:    OpLE,
			value: Symbol("b"),
			want:  []any{true, false, false},
		},
		{
			name:  "date ge skips null rows",
			array: NewColumn("d", []any{DateFromDays(2), nil, DateFromDays(1)}).Data,
			op:    OpGE,
			value: DateFromDays(2),
			want:  []any{true, false, false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareMask(tt.array, tt.op, tt.value)
			if err != nil {
				t.Fatalf("CompareMask returned error: %v", err)
			}
			if values := got.Values(); !reflect.DeepEqual(values, tt.want) {
				t.Fatalf("CompareMask values = %#v, want %#v", values, tt.want)
			}
		})
	}

	stringWithin, err := WithinMask(NewColumn("s", []any{"a", nil, "b", "c"}).Data, "a", "c", false)
	if err != nil {
		t.Fatalf("WithinMask string returned error: %v", err)
	}
	if got, want := stringWithin.Values(), []any{true, false, true, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WithinMask string values = %#v, want %#v", got, want)
	}

	nullBound, err := WithinMask(NewColumn("d", []any{DateFromDays(1), nil, DateFromDays(2)}).Data, NullValue, DateFromDays(3), true)
	if err != nil {
		t.Fatalf("WithinMask null lower bound returned error: %v", err)
	}
	if got, want := nullBound.Values(), []any{false, false, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WithinMask null lower bound values = %#v, want %#v", got, want)
	}
}

func TestFrameSchemaStableHelpers(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"a", "b", "c"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30})},
		NewColumn("venue", []any{"x", "y", "z"}),
	)

	projected, err := SelectFrameColumns(frame, "qty", "sym")
	if err != nil {
		t.Fatalf("SelectFrameColumns returned error: %v", err)
	}
	assertColumnNames(t, projected, []Symbol{"qty", "sym"})
	if kind, ok := projected.Schema().Kind("qty"); !ok || kind != KindI32 {
		t.Fatalf("projected qty kind = %s, ok %v; want %s", kind, ok, KindI32)
	}
	if kind, ok := projected.Schema().Kind("sym"); !ok || kind != KindSymbol {
		t.Fatalf("projected sym kind = %s, ok %v; want %s", kind, ok, KindSymbol)
	}

	empty, err := EmptyLike(frame)
	if err != nil {
		t.Fatalf("EmptyLike returned error: %v", err)
	}
	if empty.Len() != 0 {
		t.Fatalf("empty length = %d, want 0", empty.Len())
	}
	if !SameSchema(frame, empty) {
		t.Fatal("EmptyLike did not preserve schema")
	}

	cols := frame.Columns()
	if len(cols) != 3 {
		t.Fatalf("Columns length = %d, want 3", len(cols))
	}
	cols[0].Name = "mutated"
	if got := frame.Schema().Names()[0]; got != "sym" {
		t.Fatalf("mutating Columns result changed frame schema to %q", got)
	}
	if _, err := SelectFrameColumns(frame, "qty", "qty"); err == nil {
		t.Fatal("SelectFrameColumns accepted duplicate column")
	}
}

func TestQueryPreProjectOrderUsesSourceColumns(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("price", []any{100.0, 80.0, 120.0}),
		NewColumn("trade_id", []any{int64(2), int64(3), int64(1)}),
	)

	ordered, err := Exec(frame, QueryPlan{
		Source:          frame,
		Select:          []SelectItem{{Name: "sym", Expr: ColumnRef{Name: "sym"}}, {Name: "price", Expr: ColumnRef{Name: "price"}}},
		OrderBy:         []OrderSpec{{Column: "trade_id"}},
		PreProjectOrder: true,
		LimitN:          -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertColumnValues(t, ordered, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, ordered, "price", []any{120.0, 100.0, 80.0})
}

func TestQueryTemporalComparisonGroupingAndOrder(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("d", []any{DateFromDays(2), nil, DateFromDays(1), DateFromDays(2)}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(20),
			TimestampFromUnixNanos(10),
			TimestampFromUnixNanos(30),
			TimestampFromUnixNanos(40),
		}),
	)

	filtered, err := From(frame).
		WhereExpr(Binary{Op: OpGE, Left: ColumnRef{Name: "d"}, Right: Literal{Value: DateFromDays(2)}}).
		SelectColumns("ts").
		Exec()
	if err != nil {
		t.Fatalf("temporal comparison with null returned error: %v", err)
	}
	assertColumnValues(t, filtered, "ts", []any{Timestamp(20), Timestamp(40)})

	filtered, err = From(frame).
		WhereEq("d", DateFromDays(2)).
		SelectColumns("ts").
		OrderByColumn("ts", Desc).
		Exec()
	if err != nil {
		t.Fatalf("filtered Exec returned error: %v", err)
	}
	assertColumnValues(t, filtered, "ts", []any{Timestamp(40), Timestamp(20)})

	grouped, err := From(frame).
		GroupBy("d").
		Count("n").
		OrderByColumn("d", Asc).
		Exec()
	if err != nil {
		t.Fatalf("grouped Exec returned error: %v", err)
	}
	assertColumnValues(t, grouped, "d", []any{NullValue, Date(1), Date(2)})
	assertColumnValues(t, grouped, "n", []any{int64(1), int64(1), int64(2)})
}

func TestQueryWhereSelect(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("qty", []any{2, 5, 3}),
		NewColumn("price", []any{10, 20, 30}),
	)

	got, err := From(frame).
		WhereEq("sym", Symbol("a")).
		SelectColumns("qty", "price").
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"qty", "price"})
	assertColumnValues(t, got, "qty", []any{int64(2), int64(3)})
	assertColumnValues(t, got, "price", []any{int64(10), int64(30)})
}

func TestQueryComputedProjection(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("price", []any{10, 20}),
		NewColumn("size", []any{3, 4}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{{
			Name: "notional",
			Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "size"}},
		}},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertColumnValues(t, got, "notional", []any{30.0, 80.0})
}

func TestQueryVectorTransformProjection(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("v", []any{2, 4, 0, 8}),
		NewColumn("zero_one", []any{0, 1, 0, 1}),
		NewColumn("maybe", []any{NullValue, 3, NullValue, 5}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{
			{Name: "prev", Expr: VectorTransformExpr{Func: "prev", Expr: ColumnRef{Name: "v"}}},
			{Name: "next", Expr: VectorTransformExpr{Func: "next", Expr: ColumnRef{Name: "v"}}},
			{Name: "deltas", Expr: VectorTransformExpr{Func: "deltas", Expr: ColumnRef{Name: "v"}}},
			{Name: "fills", Expr: VectorTransformExpr{Func: "fills", Expr: ColumnRef{Name: "maybe"}}},
			{Name: "ratios", Expr: VectorTransformExpr{Func: "ratios", Expr: ColumnRef{Name: "v"}}},
			{Name: "sums", Expr: VectorTransformExpr{Func: "sums", Expr: ColumnRef{Name: "v"}}},
			{Name: "prds", Expr: VectorTransformExpr{Func: "prds", Expr: ColumnRef{Name: "v"}}},
			{Name: "mins", Expr: VectorTransformExpr{Func: "mins", Expr: ColumnRef{Name: "v"}}},
			{Name: "maxs", Expr: VectorTransformExpr{Func: "maxs", Expr: ColumnRef{Name: "v"}}},
			{Name: "avgs", Expr: VectorTransformExpr{Func: "avgs", Expr: ColumnRef{Name: "v"}}},
			{Name: "neg", Expr: VectorTransformExpr{Func: "neg", Expr: ColumnRef{Name: "v"}}},
			{Name: "sqrt", Expr: VectorTransformExpr{Func: "sqrt", Expr: ColumnRef{Name: "v"}}},
			{Name: "log", Expr: VectorTransformExpr{Func: "log", Expr: ColumnRef{Name: "v"}}},
			{Name: "exp", Expr: VectorTransformExpr{Func: "exp", Expr: ColumnRef{Name: "v"}}},
			{Name: "sin", Expr: VectorTransformExpr{Func: "sin", Expr: ColumnRef{Name: "v"}}},
			{Name: "cos", Expr: VectorTransformExpr{Func: "cos", Expr: ColumnRef{Name: "v"}}},
			{Name: "tan", Expr: VectorTransformExpr{Func: "tan", Expr: ColumnRef{Name: "v"}}},
			{Name: "asin", Expr: VectorTransformExpr{Func: "asin", Expr: ColumnRef{Name: "zero_one"}}},
			{Name: "acos", Expr: VectorTransformExpr{Func: "acos", Expr: ColumnRef{Name: "zero_one"}}},
			{Name: "atan", Expr: VectorTransformExpr{Func: "atan", Expr: ColumnRef{Name: "zero_one"}}},
			{Name: "reciprocal", Expr: VectorTransformExpr{Func: "reciprocal", Expr: ColumnRef{Name: "v"}}},
			{Name: "signum", Expr: VectorTransformExpr{Func: "signum", Expr: ColumnRef{Name: "v"}}},
			{Name: "floor", Expr: VectorTransformExpr{Func: "floor", Expr: ColumnRef{Name: "maybe"}}},
			{Name: "ceiling", Expr: VectorTransformExpr{Func: "ceiling", Expr: ColumnRef{Name: "maybe"}}},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "prev", []any{NullValue, int64(2), int64(4), int64(0)})
	assertColumnValues(t, got, "next", []any{int64(4), int64(0), int64(8), NullValue})
	assertColumnValues(t, got, "deltas", []any{2.0, 2.0, -4.0, 8.0})
	assertColumnValues(t, got, "fills", []any{NullValue, int64(3), int64(3), int64(5)})
	assertColumnValues(t, got, "ratios", []any{NullValue, 2.0, 0.0, math.Inf(1)})
	assertColumnValues(t, got, "sums", []any{2.0, 6.0, 6.0, 14.0})
	assertColumnValues(t, got, "prds", []any{2.0, 8.0, 0.0, 0.0})
	assertColumnValues(t, got, "mins", []any{int64(2), int64(2), int64(0), int64(0)})
	assertColumnValues(t, got, "maxs", []any{int64(2), int64(4), int64(4), int64(8)})
	assertColumnValues(t, got, "avgs", []any{2.0, 3.0, 2.0, 3.5})
	assertColumnValues(t, got, "neg", []any{-2.0, -4.0, -0.0, -8.0})
	assertColumnValues(t, got, "sqrt", []any{math.Sqrt(2), 2.0, 0.0, math.Sqrt(8)})
	assertColumnValues(t, got, "log", []any{math.Log(2), math.Log(4), math.Inf(-1), math.Log(8)})
	assertColumnValues(t, got, "exp", []any{math.Exp(2), math.Exp(4), 1.0, math.Exp(8)})
	assertColumnValues(t, got, "sin", []any{math.Sin(2), math.Sin(4), 0.0, math.Sin(8)})
	assertColumnValues(t, got, "cos", []any{math.Cos(2), math.Cos(4), 1.0, math.Cos(8)})
	assertColumnValues(t, got, "tan", []any{math.Tan(2), math.Tan(4), 0.0, math.Tan(8)})
	assertColumnValues(t, got, "asin", []any{0.0, math.Pi / 2, 0.0, math.Pi / 2})
	assertColumnValues(t, got, "acos", []any{math.Pi / 2, 0.0, math.Pi / 2, 0.0})
	assertColumnValues(t, got, "atan", []any{0.0, math.Pi / 4, 0.0, math.Pi / 4})
	assertColumnValues(t, got, "reciprocal", []any{0.5, 0.25, math.Inf(1), 0.125})
	assertColumnValues(t, got, "signum", []any{1.0, 1.0, 0.0, 1.0})
	assertColumnValues(t, got, "floor", []any{NullValue, 3.0, NullValue, 5.0})
	assertColumnValues(t, got, "ceiling", []any{NullValue, 3.0, NullValue, 5.0})
}

func TestQueryVectorTransformProjectionRuntimeKernelStats(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	frame := mustFrame(t,
		NewColumn("px", []any{2.0, 4.0, 8.0, 16.0}),
		NewColumn("qty", []any{4, 1, 3, 2}),
		NewColumn("ts", []any{int64(0), int64(9), int64(10), int64(19)}),
	)
	_, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{
			{Name: "sqrt_px", Expr: VectorTransformExpr{Func: "sqrt", Expr: ColumnRef{Name: "px"}}},
			{Name: "log_px", Expr: VectorTransformExpr{Func: "log", Expr: ColumnRef{Name: "px"}}},
			{Name: "sin_px", Expr: VectorTransformExpr{Func: "sin", Expr: ColumnRef{Name: "px"}}},
			{Name: "cos_px", Expr: VectorTransformExpr{Func: "cos", Expr: ColumnRef{Name: "px"}}},
			{Name: "r", Expr: VectorTransformExpr{Func: "rank", Expr: ColumnRef{Name: "qty"}}},
			{Name: "bucket", Expr: BucketFloorExpr{Interval: int64(10), Expr: ColumnRef{Name: "ts"}}},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	stats := RuntimeKernelExecutionStats()
	for _, tc := range []struct {
		kernel string
		shape  string
	}{
		{"DataVectorTransformNumericUnary", "vector-transform/sqrt/f64"},
		{"DataVectorTransformNumericUnary", "vector-transform/log/f64"},
		{"DataVectorTransformNumericUnary", "vector-transform/sin/f64"},
		{"DataVectorTransformNumericUnary", "vector-transform/cos/f64"},
		{"DataVectorTransformRank", "vector-transform/rank/i64"},
		{"DataBucketFloor", "bucket-floor/xbar/i64"},
	} {
		assertDataRuntimeKernelStat(t, stats, tc.kernel, tc.shape, "attempt", 1)
		assertDataRuntimeKernelStat(t, stats, tc.kernel, tc.shape, "hit", 1)
	}
}

func TestQueryVectorTransformProjectionUsesFilteredAndOrderedRows(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{int64(30), int64(10), int64(20), int64(40)}),
		NewColumn("price", []any{300, 100, 200, 400}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("a")}},
		Select: []SelectItem{
			{Name: "ts", Expr: ColumnRef{Name: "ts"}},
			{Name: "prev_price", Expr: VectorTransformExpr{Func: "prev", Expr: ColumnRef{Name: "price"}}},
			{Name: "delta", Expr: VectorTransformExpr{Func: "deltas", Expr: ColumnRef{Name: "price"}}},
		},
		OrderBy:         []OrderSpec{{Column: "ts"}},
		PreProjectOrder: true,
		LimitN:          -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "ts", []any{int64(20), int64(30), int64(40)})
	assertColumnValues(t, got, "prev_price", []any{NullValue, int64(200), int64(300)})
	assertColumnValues(t, got, "delta", []any{200.0, 100.0, 100.0})
}

func TestQueryVectorTransformProjectionPreventsEarlyLimit(t *testing.T) {
	frame := mustFrame(t, NewColumn("v", []any{10, 20, 30}))

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{{
			Name: "next",
			Expr: VectorTransformExpr{Func: "next", Expr: ColumnRef{Name: "v"}},
		}},
		LimitN: 2,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "next", []any{int64(20), int64(30)})
}

func TestQueryVectorTransformXPrevAndMovingFoundation(t *testing.T) {
	frame := mustFrame(t, NewColumn("v", []any{1, 2, 3, 4}))

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{
			{Name: "xprev", Expr: VectorTransformExpr{Func: "xprev", Arg: Literal{Value: int64(2)}, Expr: ColumnRef{Name: "v"}}},
			{Name: "moving", Expr: VectorTransformExpr{Func: "moving", Arg: Literal{Value: int64(3)}, Expr: ColumnRef{Name: "v"}}},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "xprev", []any{NullValue, NullValue, int64(1), int64(2)})
	assertColumnValues(t, got, "moving", []any{
		[]any{int64(1)},
		[]any{int64(1), int64(2)},
		[]any{int64(1), int64(2), int64(3)},
		[]any{int64(2), int64(3), int64(4)},
	})
}

func TestQueryGroupedVectorTransformProjectionUsesGroupAndPreProjectOrder(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a"), Symbol("b"), Symbol("a"), Symbol("b")}),
		NewColumn("ts", []any{int64(30), int64(20), int64(10), int64(40), int64(50), int64(15)}),
		NewColumn("price", []any{300, 20, 100, 40, 500, 15}),
		NewColumn("maybe", []any{NullValue, 2, 1, NullValue, NullValue, 3}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		By:     []Symbol{"sym"},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "ts", Expr: ColumnRef{Name: "ts"}},
			{Name: "price", Expr: ColumnRef{Name: "price"}},
			{Name: "deltas", Expr: VectorTransformExpr{Func: "deltas", Expr: ColumnRef{Name: "price"}}},
			{Name: "fills", Expr: VectorTransformExpr{Func: "fills", Expr: ColumnRef{Name: "maybe"}}},
			{Name: "xprev", Expr: VectorTransformExpr{Func: "xprev", Arg: Literal{Value: int64(2)}, Expr: ColumnRef{Name: "price"}}},
			{Name: "moving", Expr: VectorTransformExpr{Func: "moving", Arg: Literal{Value: int64(2)}, Expr: ColumnRef{Name: "price"}}},
		},
		OrderBy:         []OrderSpec{{Column: "ts"}},
		PreProjectOrder: true,
		LimitN:          -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "ts", "price", "deltas", "fills", "xprev", "moving"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("b"), Symbol("a"), Symbol("b"), Symbol("a")})
	assertColumnValues(t, got, "ts", []any{int64(10), int64(15), int64(20), int64(30), int64(40), int64(50)})
	assertColumnValues(t, got, "price", []any{int64(100), int64(15), int64(20), int64(300), int64(40), int64(500)})
	assertColumnValues(t, got, "deltas", []any{100.0, 15.0, 5.0, 200.0, 20.0, 200.0})
	assertColumnValues(t, got, "fills", []any{int64(1), int64(3), int64(2), int64(1), int64(2), int64(1)})
	assertColumnValues(t, got, "xprev", []any{NullValue, NullValue, NullValue, NullValue, int64(15), int64(100)})
	assertColumnValues(t, got, "moving", []any{
		[]any{int64(100)},
		[]any{int64(15)},
		[]any{int64(15), int64(20)},
		[]any{int64(100), int64(300)},
		[]any{int64(20), int64(40)},
		[]any{int64(300), int64(500)},
	})
}

func TestQueryLimitCanRunBeforeProjection(t *testing.T) {
	frame := mustFrame(t, NewColumn("v", []any{1, 2, 3, 4, 5}))
	expr := &countingExpr{expr: Binary{Op: OpMul, Left: ColumnRef{Name: "v"}, Right: Literal{Value: int64(10)}}}

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{{
			Name: "scaled",
			Expr: expr,
		}},
		LimitN: 2,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "scaled", []any{10.0, 20.0})
	if expr.calls != 2 {
		t.Fatalf("projection evaluated %d rows, want 2", expr.calls)
	}
}

type countingExpr struct {
	expr  Expr
	calls int
}

func (e *countingExpr) EvalRow(frame Frame, row int) (any, error) {
	e.calls++
	return e.expr.EvalRow(frame, row)
}

func TestQueryGroupBySymbolSumCount(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("b"), Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("qty", []any{2, 5, 3, 7}),
	)

	got, err := From(frame).
		GroupBy("sym").
		Sum("qty", "total_qty").
		Count("n").
		OrderByColumn("sym", Asc).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "total_qty", "n"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b")})
	assertColumnValues(t, got, "total_qty", []any{12.0, 5.0})
	assertColumnValues(t, got, "n", []any{int64(2), int64(2)})
}

func TestQueryGroupBySumAvgSkipNulls(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b"), Symbol("b")}),
		NewColumn("qty", []any{10, nil, 20, nil, 5}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "avg_qty", Func: "avg", Expr: ColumnRef{Name: "qty"}},
			{Name: "fills", Func: "count"},
		},
		OrderBy: []OrderSpec{{Column: "sym"}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b")})
	assertColumnValues(t, got, "total_qty", []any{30.0, 5.0})
	assertColumnValues(t, got, "avg_qty", []any{15.0, 5.0})
	assertColumnValues(t, got, "fills", []any{int64(3), int64(2)})
}

func TestQueryGroupByCommonAggregatesNullBoundaries(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), Symbol("b")}),
		NewColumn("qty", []any{nil, nil, 10, nil}),
		NewColumn("px", []any{NullValue, 20.0, NullValue, 40.0}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "avg_qty", Func: "avg", Expr: ColumnRef{Name: "qty"}},
			{Name: "lo_qty", Func: "min", Expr: ColumnRef{Name: "qty"}},
			{Name: "hi_qty", Func: "max", Expr: ColumnRef{Name: "qty"}},
			{Name: "first_px", Func: "first", Expr: ColumnRef{Name: "px"}},
			{Name: "last_px", Func: "last", Expr: ColumnRef{Name: "px"}},
			{Name: "fills", Func: "count"},
		},
		OrderBy: []OrderSpec{{Column: "sym"}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b")})
	assertColumnValues(t, got, "total_qty", []any{0.0, 10.0})
	assertColumnValues(t, got, "avg_qty", []any{0.0, 10.0})
	assertColumnValues(t, got, "lo_qty", []any{NullValue, int64(10)})
	assertColumnValues(t, got, "hi_qty", []any{NullValue, int64(10)})
	assertColumnValues(t, got, "first_px", []any{NullValue, NullValue})
	assertColumnValues(t, got, "last_px", []any{20.0, 40.0})
	assertColumnValues(t, got, "fills", []any{int64(2), int64(2)})
}

func TestQueryGroupByExtendedAggregates(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b"), Symbol("b")}),
		NewColumn("price", []any{10.0, 20.0, 30.0, 10.0, NullValue}),
		NewColumn("size", []any{1, 2, 3, 4, 5}),
	)

	got, err := From(frame).
		GroupBy("sym").
		Var("price", "var_price").
		Dev("price", "dev_price").
		Med("price", "med_price").
		WAvg("size", "price", "wavg_price").
		OrderByColumn("sym", Asc).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b")})
	assertColumnValues(t, got, "var_price", []any{66.66666666666669, 0.0})
	assertColumnValues(t, got, "dev_price", []any{8.16496580927726, 0.0})
	assertColumnValues(t, got, "med_price", []any{20.0, 10.0})
	assertColumnValues(t, got, "wavg_price", []any{23.333333333333332, 10.0})
}

func TestQueryGroupByColumnRefFastPathPreservesTypedKeys(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("channel", []any{int32(2), int32(1), int32(2), int32(1), int32(2)}),
		NewColumn("ts_bucket", []any{
			TimestampFromUnixNanos(1_000),
			TimestampFromUnixNanos(1_000),
			TimestampFromUnixNanos(2_000),
			TimestampFromUnixNanos(1_000),
			TimestampFromUnixNanos(2_000),
		}),
		NewColumn("qty", []any{10, 20, 30, 40, 50}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		By:     []Symbol{"channel", "ts_bucket"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
		},
		OrderBy: []OrderSpec{{Column: "channel"}, {Column: "ts_bucket"}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"channel", "ts_bucket", "total_qty"})
	assertColumnValues(t, got, "channel", []any{int32(1), int32(2), int32(2)})
	assertColumnValues(t, got, "ts_bucket", []any{
		TimestampFromUnixNanos(1_000),
		TimestampFromUnixNanos(1_000),
		TimestampFromUnixNanos(2_000),
	})
	assertColumnValues(t, got, "total_qty", []any{60.0, 10.0, 80.0})
	if kind, ok := got.Schema().Kind("channel"); !ok || kind != KindI32 {
		t.Fatalf("channel kind = %s, ok %v; want %s", kind, ok, KindI32)
	}
	if kind, ok := got.Schema().Kind("ts_bucket"); !ok || kind != KindTimestamp {
		t.Fatalf("ts_bucket kind = %s, ok %v; want %s", kind, ok, KindTimestamp)
	}
}

type countingMetadataArray struct {
	array Array
	ats   int
}

func (a *countingMetadataArray) Kind() Kind { return a.array.Kind() }

func (a *countingMetadataArray) Len() int { return a.array.Len() }

func (a *countingMetadataArray) At(row int) (any, bool) {
	a.ats++
	return a.array.At(row)
}

func (a *countingMetadataArray) Values() []any { return a.array.Values() }

func (a *countingMetadataArray) Gather(indexes []int) Array {
	return &countingMetadataArray{array: a.array.Gather(indexes)}
}

func (a *countingMetadataArray) ArrayMetadata() ArrayMetadata {
	return ArrayMetadataOf(a.array)
}

func TestQueryGroupByUsesSingleColumnAttributeIndexForKeys(t *testing.T) {
	sym := &countingMetadataArray{
		array: WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "AAPL", "MSFT"}), ArrayAttributeGrouped),
	}
	frame := mustFrame(t,
		Column{Name: "sym", Data: sym},
		NewColumn("qty", []any{10, 20, 30, 40, 50}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "n", Func: "count"},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if sym.ats != 0 {
		t.Fatalf("group-by key column At called %d times; want indexed key path", sym.ats)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, got, "total_qty", []any{80.0, 70.0})
	assertColumnValues(t, got, "n", []any{int64(3), int64(2)})
}

func TestQueryGroupByTypedAggregateFrameSupportsSymbolAndIntKeys(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"})},
		Column{Name: "bucket", Data: NewI64([]int64{1, 2, 1, 2, 2})},
		Column{Name: "qty", Data: NewI64([]int64{10, 20, 30, 40, 50})},
		Column{Name: "px", Data: NewF64([]float64{100, 101, 102, 103, 104})},
	)
	symbolPlan := QueryPlan{
		By: []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "n", Func: "count"},
			{Name: "min_px", Func: "min", Expr: ColumnRef{Name: "px"}},
			{Name: "max_px", Func: "max", Expr: ColumnRef{Name: "px"}},
		},
		LimitN: -1,
	}
	kernel, ok, err := CompileQueryKernel(frame, symbolPlan)
	if err != nil || !ok {
		t.Fatalf("CompileQueryKernel symbol grouped aggregate = %v,%v", ok, err)
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("kernel Exec symbol grouped aggregate returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("NVDA")})
	assertColumnValues(t, got, "qty", []any{40.0, 70.0, 40.0})
	assertColumnValues(t, got, "n", []any{int64(2), int64(2), int64(1)})
	assertColumnValues(t, got, "min_px", []any{100.0, 101.0, 103.0})
	assertColumnValues(t, got, "max_px", []any{102.0, 104.0, 103.0})
	if col, _ := got.Column("sym"); col.Kind() != KindSymbol {
		t.Fatalf("symbol key kind = %s, want %s", col.Kind(), KindSymbol)
	}
	if col, _ := got.Column("n"); col.Kind() != KindI64 {
		t.Fatalf("count kind = %s, want %s", col.Kind(), KindI64)
	}
	if col, _ := got.Column("qty"); col.Kind() != KindF64 {
		t.Fatalf("sum kind = %s, want %s", col.Kind(), KindF64)
	}

	intPlan := QueryPlan{
		By: []Symbol{"bucket"},
		Aggregates: []Aggregate{
			{Name: "qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "min_px", Func: "min", Expr: ColumnRef{Name: "px"}},
			{Name: "max_px", Func: "max", Expr: ColumnRef{Name: "px"}},
		},
		LimitN: -1,
	}
	kernel, ok, err = CompileQueryKernel(frame, intPlan)
	if err != nil || !ok {
		t.Fatalf("CompileQueryKernel int grouped aggregate = %v,%v", ok, err)
	}
	got, err = kernel.Exec(frame)
	if err != nil {
		t.Fatalf("kernel Exec int grouped aggregate returned error: %v", err)
	}
	assertColumnValues(t, got, "bucket", []any{int64(1), int64(2)})
	assertColumnValues(t, got, "qty", []any{40.0, 110.0})
	assertColumnValues(t, got, "min_px", []any{100.0, 101.0})
	assertColumnValues(t, got, "max_px", []any{102.0, 104.0})
	if col, _ := got.Column("bucket"); col.Kind() != KindI64 {
		t.Fatalf("int key kind = %s, want %s", col.Kind(), KindI64)
	}
}

func TestQueryGroupByCachesSingleColumnGroupedIndex(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT"})},
		Column{Name: "qty", Data: NewI64([]int64{10, 20, 30, 40, 50})},
	)
	if sym, _ := frame.Column("sym"); ArrayHasAttribute(sym, ArrayAttributeGrouped) {
		t.Fatal("sym unexpectedly starts with grouped attribute")
	}

	_, err := Exec(frame, QueryPlan{
		Source: frame,
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
		},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	sym, _ := frame.Column("sym")
	if _, ok := ArrayIndexFor(sym, ArrayAttributeGrouped); !ok {
		t.Fatal("sym grouped index was not cached on frame column")
	}
}

func TestQueryGroupByBucket(t *testing.T) {
	raw := NewColumn("ts", []any{
		TimestampFromUnixNanos(100),
		TimestampFromUnixNanos(1_100),
		TimestampFromUnixNanos(1_900),
		TimestampFromUnixNanos(2_000),
		nil,
	}).Data
	bucketed, err := BucketFloor(raw, int64(1_000))
	if err != nil {
		t.Fatalf("BucketFloor returned error: %v", err)
	}
	frame := mustFrame(t,
		Column{Name: "bucket", Data: bucketed},
		NewColumn("qty", []any{2, 3, 5, 7, 11}),
	)

	got, err := From(frame).
		GroupBy("bucket").
		Sum("qty", "total_qty").
		Count("n").
		OrderByColumn("bucket", Asc).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "bucket", []any{NullValue, Timestamp(0), Timestamp(1000), Timestamp(2000)})
	assertColumnValues(t, got, "total_qty", []any{11.0, 2.0, 8.0, 7.0})
	assertColumnValues(t, got, "n", []any{int64(1), int64(1), int64(2), int64(1)})
}

func TestQueryGroupByMinMaxFirstLast(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("b"), Symbol("a"), Symbol("b"), Symbol("a"), Symbol("c")}),
		NewColumn("qty", []any{2, nil, 3, 7, nil}),
		NewColumn("venue", []any{"x", "y", "z", nil, "w"}),
	)

	got, err := From(frame).
		GroupBy("sym").
		Min("qty", "min_qty").
		Max("qty", "max_qty").
		First("venue", "first_venue").
		Last("venue", "last_venue").
		OrderByColumn("sym", Asc).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "min_qty", "max_qty", "first_venue", "last_venue"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c")})
	assertColumnValues(t, got, "min_qty", []any{int64(7), int64(2), NullValue})
	assertColumnValues(t, got, "max_qty", []any{int64(7), int64(3), NullValue})
	assertColumnValues(t, got, "first_venue", []any{"y", "x", "w"})
	assertColumnValues(t, got, "last_venue", []any{NullValue, "z", "w"})
}

func TestQueryOrderLimit(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{"a", "b", "c"}),
		NewColumn("qty", []any{2, 5, 3}),
	)

	got, err := From(frame).
		OrderByColumn("qty", Desc).
		Limit(2).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{"b", "c"})
	assertColumnValues(t, got, "qty", []any{int64(5), int64(3)})
}

func TestQuerySingleColumnOrderLimitUsesStableTopK(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{"a", "b", "c", "d", "e", "f"}),
		NewColumn("qty", []any{5, 7, 7, 4, 7, 1}),
	)

	got, err := From(frame).
		OrderByColumn("qty", Desc).
		Limit(3).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{"b", "c", "e"})
	assertColumnValues(t, got, "qty", []any{int64(7), int64(7), int64(7)})
}

func TestQueryLimitBeforeProjectionAvoidsExtraEval(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("id", []any{1, 2, 3, 4, 5}),
		NewColumn("price", []any{10.0, 11.0, 12.0, 13.0, 14.0}),
		NewColumn("size", []any{2, 3, 4, 5, 6}),
	)
	notional := &countingExpr{expr: Binary{Op: OpMul, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "size"}}}

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{
			{Name: "id", Expr: ColumnRef{Name: "id"}},
			{Name: "notional", Expr: notional},
		},
		LimitN: 2,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	if got.Len() != 2 {
		t.Fatalf("limited length = %d, want 2", got.Len())
	}
	if notional.calls != 2 {
		t.Fatalf("computed projection calls = %d, want 2", notional.calls)
	}
	assertColumnValues(t, got, "notional", []any{20.0, 33.0})
}

func TestQueryOrderLimitOnDirectProjectionAvoidsExtraEval(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("id", []any{1, 2, 3, 4, 5}),
		NewColumn("price", []any{10.0, 14.0, 12.0, 13.0, 11.0}),
		NewColumn("size", []any{2, 3, 4, 5, 6}),
	)
	notional := &countingExpr{expr: Binary{Op: OpMul, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "size"}}}

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{
			{Name: "price", Expr: ColumnRef{Name: "price"}},
			{Name: "notional", Expr: notional},
		},
		OrderBy: []OrderSpec{{Column: "price", Desc: true}},
		LimitN:  2,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	if notional.calls != 2 {
		t.Fatalf("computed projection calls = %d, want 2", notional.calls)
	}
	assertColumnValues(t, got, "price", []any{14.0, 13.0})
	assertColumnValues(t, got, "notional", []any{42.0, 65.0})
}

func TestQueryLimitZeroAfterPreProjectOrderPreservesSchema(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("b"), Symbol("a"), Symbol("c")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(200),
			TimestampFromUnixNanos(100),
			TimestampFromUnixNanos(300),
		}),
		NewColumn("qty", []any{int32(2), int32(1), int32(3)}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "ts", Expr: ColumnRef{Name: "ts"}},
			{Name: "qty", Expr: ColumnRef{Name: "qty"}},
		},
		OrderBy:         []OrderSpec{{Column: "ts"}},
		PreProjectOrder: true,
		LimitN:          0,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	if got.Len() != 0 {
		t.Fatalf("limited frame length = %d, want 0", got.Len())
	}
	assertColumnNames(t, got, []Symbol{"sym", "ts", "qty"})
	if kind, ok := got.Schema().Kind("sym"); !ok || kind != KindSymbol {
		t.Fatalf("sym kind = %s, ok %v; want %s", kind, ok, KindSymbol)
	}
	if kind, ok := got.Schema().Kind("ts"); !ok || kind != KindTimestamp {
		t.Fatalf("ts kind = %s, ok %v; want %s", kind, ok, KindTimestamp)
	}
	if kind, ok := got.Schema().Kind("qty"); !ok || kind != KindI32 {
		t.Fatalf("qty kind = %s, ok %v; want %s", kind, ok, KindI32)
	}
}

func TestQueryKernelExecMatchesQueryPlanAndRejectsSchemaDrift(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("NVDA"), Symbol("AAPL")}),
		NewColumn("qty", []any{int32(5), int32(4), int32(2), int32(7)}),
		NewColumn("px", []any{100.0, 101.0, 102.0, 103.0}),
	)
	plan := QueryPlan{
		Where: Logical{Op: "and",
			Left:  Within{Expr: ColumnRef{Name: "qty"}, Low: int32(2), High: int32(5), HighClosed: true},
			Right: Not{Expr: In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("MSFT")}}},
		},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "score", Expr: Binary{Op: OpAdd, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(10)}}},
		},
		OrderBy:         []OrderSpec{{Column: "qty", Desc: true}},
		PreProjectOrder: true,
		LimitN:          2,
	}

	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		t.Fatal("CompileQueryKernel did not accept supported projection query")
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("QueryKernel Exec returned error: %v", err)
	}
	want, err := Exec(frame, plan)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if !SameSchema(got, want) {
		t.Fatalf("kernel schema = %#v, want %#v", got.Schema(), want.Schema())
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("NVDA")})
	assertColumnValues(t, got, "score", []any{15.0, 12.0})

	drifted := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL")}),
		NewColumn("qty", []any{int64(5)}),
		NewColumn("px", []any{100.0}),
	)
	if _, err := kernel.Exec(drifted); err == nil {
		t.Fatal("QueryKernel Exec accepted a frame with a drifted qty kind")
	}
}

func TestQueryKernelOrderBySortedColumnFastPath(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "seq", Data: WithArrayAttribute(NewI64([]int64{
			100,
			200,
			300,
			400,
		}), ArrayAttributeSorted)},
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")}),
		NewColumn("qty", []any{int32(1), int32(2), int32(3), int32(4)}),
	)
	plan := QueryPlan{
		Select: []SelectItem{
			{Name: "seq", Expr: ColumnRef{Name: "seq"}},
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
		},
		OrderBy:         []OrderSpec{{Column: "seq"}},
		PreProjectOrder: true,
		LimitN:          -1,
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		t.Fatal("CompileQueryKernel did not accept sorted order query")
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("QueryKernel Exec returned error: %v", err)
	}
	assertColumnValues(t, got, "seq", []any{int64(100), int64(200), int64(300), int64(400)})
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})

	descPlan := plan
	descPlan.OrderBy = []OrderSpec{{Column: "seq", Desc: true}}
	descKernel, ok, err := CompileQueryKernel(frame, descPlan)
	if err != nil {
		t.Fatalf("CompileQueryKernel desc returned error: %v", err)
	}
	if !ok {
		t.Fatal("CompileQueryKernel did not accept descending sorted order query")
	}
	descGot, err := descKernel.Exec(frame)
	if err != nil {
		t.Fatalf("descending QueryKernel Exec returned error: %v", err)
	}
	assertColumnValues(t, descGot, "seq", []any{int64(400), int64(300), int64(200), int64(100)})
	assertColumnValues(t, descGot, "sym", []any{Symbol("TSLA"), Symbol("MSFT"), Symbol("AAPL"), Symbol("AAPL")})
}

func TestOrderIndexesUsesSortedAttributeForFilteredSubset(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "seq", Data: WithArrayAttribute(NewI64([]int64{10, 20, 30, 40, 50}), ArrayAttributeSorted)},
		NewColumn("sym", []any{Symbol("A"), Symbol("B"), Symbol("C"), Symbol("D"), Symbol("E")}),
	)
	subset := []int{1, 3, 4}
	asc, err := orderIndexes(frame, subset, []OrderSpec{{Column: "seq"}})
	if err != nil {
		t.Fatalf("orderIndexes asc returned error: %v", err)
	}
	if want := []int{1, 3, 4}; !reflect.DeepEqual(asc, want) {
		t.Fatalf("orderIndexes asc = %v, want %v", asc, want)
	}
	desc, err := orderIndexes(frame, subset, []OrderSpec{{Column: "seq", Desc: true}})
	if err != nil {
		t.Fatalf("orderIndexes desc returned error: %v", err)
	}
	if want := []int{4, 3, 1}; !reflect.DeepEqual(desc, want) {
		t.Fatalf("orderIndexes desc = %v, want %v", desc, want)
	}
	if want := []int{1, 3, 4}; !reflect.DeepEqual(subset, want) {
		t.Fatalf("orderIndexes mutated input subset = %v, want %v", subset, want)
	}
}

func TestQueryKernelCompileFallbackAndValidation(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("qty", []any{int32(1), int32(1), int32(2)}),
	)
	distinctPlan := QueryPlan{
		Source:   frame,
		Distinct: true,
		LimitN:   -1,
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "qty", Expr: ColumnRef{Name: "qty"}},
		},
	}
	distinctKernel, ok, err := CompileQueryKernel(frame, distinctPlan)
	if err != nil {
		t.Fatalf("CompileQueryKernel distinct returned error: %v", err)
	}
	if !ok || distinctKernel == nil {
		t.Fatal("CompileQueryKernel did not accept distinct query")
	}
	if reason := distinctKernel.Reason(); !strings.Contains(reason, "distinct projection path") {
		t.Fatalf("distinct QueryKernel reason = %q, want distinct projection path", reason)
	}
	distinctGot, err := distinctKernel.Exec(frame)
	if err != nil {
		t.Fatalf("distinct QueryKernel Exec returned error: %v", err)
	}
	distinctWant, err := distinctPlan.Exec()
	if err != nil {
		t.Fatalf("distinct QueryPlan Exec returned error: %v", err)
	}
	if !SameSchema(distinctGot, distinctWant) || distinctGot.Len() != distinctWant.Len() {
		t.Fatalf("distinct kernel frame schema/len = %#v/%d, want %#v/%d", distinctGot.Schema(), distinctGot.Len(), distinctWant.Schema(), distinctWant.Len())
	}
	assertColumnValues(t, distinctGot, "sym", []any{Symbol("a"), Symbol("b")})
	assertColumnValues(t, distinctGot, "qty", []any{int32(1), int32(2)})

	grouped := QueryPlan{
		By:         []Symbol{"sym"},
		Aggregates: []Aggregate{{Name: "n", Func: "count"}},
	}
	kernel, ok, err := CompileQueryKernel(frame, grouped)
	if err != nil {
		t.Fatalf("CompileQueryKernel grouped returned error: %v", err)
	}
	if !ok {
		t.Fatal("CompileQueryKernel did not accept supported grouped aggregate")
	}
	if reason := kernel.Reason(); !strings.Contains(reason, "indexed single-column grouped mixed aggregate fast path") {
		t.Fatalf("grouped QueryKernel reason = %q, want indexed grouped aggregate fast path", reason)
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("grouped QueryKernel Exec returned error: %v", err)
	}
	want, err := Exec(frame, grouped)
	if err != nil {
		t.Fatalf("grouped Exec returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", want.columns["sym"].Values())
	assertColumnValues(t, got, "n", want.columns["n"].Values())

	mixedFrame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"a", "a", "b", "b", "c"}), ArrayAttributeGrouped)},
		Column{Name: "qty", Data: NewI32([]int32{5, 2, 7, 4, 9})},
		Column{Name: "px", Data: NewF64([]float64{10, 20, 30, 40, 50})},
	)
	mixed := QueryPlan{
		Where: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(4)}},
		By:    []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "avg_px", Func: "avg", Expr: ColumnRef{Name: "px"}},
			{Name: "lo_px", Func: "min", Expr: ColumnRef{Name: "px"}},
			{Name: "hi_px", Func: "max", Expr: ColumnRef{Name: "px"}},
			{Name: "fills", Func: "count"},
		},
		LimitN: -1,
	}
	mixedKernel, ok, err := CompileQueryKernel(mixedFrame, mixed)
	if err != nil {
		t.Fatalf("CompileQueryKernel mixed returned error: %v", err)
	}
	if !ok {
		t.Fatal("CompileQueryKernel did not accept indexed grouped mixed aggregate")
	}
	if reason := mixedKernel.Reason(); !strings.Contains(reason, "indexed single-column grouped mixed aggregate fast path") {
		t.Fatalf("QueryKernel reason = %q, want indexed grouped mixed aggregate fast path", reason)
	}
	mixedGot, err := mixedKernel.Exec(mixedFrame)
	if err != nil {
		t.Fatalf("mixed grouped QueryKernel Exec returned error: %v", err)
	}
	assertColumnValues(t, mixedGot, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c")})
	assertColumnValues(t, mixedGot, "total_qty", []any{5.0, 11.0, 9.0})
	assertColumnValues(t, mixedGot, "avg_px", []any{10.0, 35.0, 50.0})
	assertColumnValues(t, mixedGot, "lo_px", []any{10.0, 30.0, 50.0})
	assertColumnValues(t, mixedGot, "hi_px", []any{10.0, 40.0, 50.0})
	assertColumnValues(t, mixedGot, "fills", []any{int64(1), int64(2), int64(1)})

	if _, ok, err := CompileQueryKernel(frame, QueryPlan{
		Select: []SelectItem{{Name: "missing", Expr: ColumnRef{Name: "missing"}}},
	}); err == nil || !ok {
		t.Fatalf("CompileQueryKernel missing column err = %v, ok %v; want supported validation error", err, ok)
	}
	if _, ok, err := CompileQueryKernel(frame, QueryPlan{
		Select:  []SelectItem{{Name: "qty", Expr: ColumnRef{Name: "qty"}}},
		OrderBy: []OrderSpec{{Column: "sym"}},
	}); err == nil || !ok {
		t.Fatalf("CompileQueryKernel post-project order err = %v, ok %v; want validation error", err, ok)
	}
	var nilKernel *QueryKernel
	if _, err := nilKernel.Exec(frame); err == nil {
		t.Fatal("nil QueryKernel Exec returned nil error")
	}
}

func TestQueryKernelGroupedAggregateWithComputedWhereOrderLimit(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "AAPL", "MSFT", "NVDA", "MSFT", "TSLA"})},
		Column{Name: "side", Data: NewSymbols([]string{"buy", "sell", "buy", "buy", "sell", "buy"})},
		Column{Name: "price", Data: NewF64([]float64{100.5, 101.0, 80.0, 120.0, 82.0, 200.0})},
		Column{Name: "size", Data: NewI64([]int64{10, 20, 30, 5, 50, 1})},
	)
	plan := QueryPlan{
		Source: frame,
		Where: Logical{
			Op: "and",
			Left: In{
				Expr:   ColumnRef{Name: "sym"},
				Values: []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("NVDA")},
			},
			Right: Logical{
				Op: "or",
				Left: Binary{
					Op:    OpEQ,
					Left:  ColumnRef{Name: "side"},
					Right: Literal{Value: Symbol("buy")},
				},
				Right: Binary{
					Op:    OpGE,
					Left:  ColumnRef{Name: "price"},
					Right: Literal{Value: 100.0},
				},
			},
		},
		By: []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "notional", Func: "sum", Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "size"}}},
			{Name: "fills", Func: "count"},
			{Name: "avg_px", Func: "avg", Expr: ColumnRef{Name: "price"}},
		},
		OrderBy: []OrderSpec{{Column: "notional", Desc: true}},
		LimitN:  2,
	}

	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		_, reason := QueryKernelSupportReason(plan)
		t.Fatalf("CompileQueryKernel ok = false, reason: %s", reason)
	}
	if reason := kernel.Reason(); !strings.Contains(reason, "grouped aggregate path") {
		t.Fatalf("QueryKernel reason = %q, want grouped aggregate path", reason)
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("QueryKernel Exec returned error: %v", err)
	}
	want, err := plan.Exec()
	if err != nil {
		t.Fatalf("fallback Exec returned error: %v", err)
	}
	if !SameSchema(got, want) || got.Len() != want.Len() {
		t.Fatalf("kernel schema/len = %#v/%d, want %#v/%d", got.Schema(), got.Len(), want.Schema(), want.Len())
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, got, "notional", []any{3025.0, 2400.0})
	assertColumnValues(t, got, "fills", []any{int64(2), int64(1)})
	assertColumnValues(t, got, "avg_px", []any{100.75, 80.0})
	assertColumnValues(t, want, "notional", []any{3025.0, 2400.0})
}

func TestQueryKernelFilteredGroupedAggregateSkipsNullableNumericNulls(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"AAPL", "AAPL", "MSFT", "MSFT", "NVDA"}), ArrayAttributeGrouped)},
		Column{Name: "qty", Data: NewColumn("qty", []any{nil, int64(10), nil, int64(20), int64(30)}).Data},
	)
	plan := QueryPlan{
		Source: frame,
		Where: In{
			Expr:   ColumnRef{Name: "sym"},
			Values: []any{Symbol("AAPL"), Symbol("MSFT")},
		},
		By: []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "avg_qty", Func: "avg", Expr: ColumnRef{Name: "qty"}},
			{Name: "fills", Func: "count"},
		},
		OrderBy: []OrderSpec{{Column: "sym"}},
		LimitN:  -1,
	}

	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		_, reason := QueryKernelSupportReason(plan)
		t.Fatalf("CompileQueryKernel ok = false, reason: %s", reason)
	}
	if reason := kernel.Reason(); !strings.Contains(reason, "indexed single-column grouped mixed aggregate fast path") {
		t.Fatalf("QueryKernel reason = %q, want indexed grouped aggregate fast path", reason)
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("QueryKernel Exec returned error: %v", err)
	}
	want, err := plan.Exec()
	if err != nil {
		t.Fatalf("fallback Exec returned error: %v", err)
	}
	if !SameSchema(got, want) || got.Len() != want.Len() {
		t.Fatalf("kernel schema/len = %#v/%d, want %#v/%d", got.Schema(), got.Len(), want.Schema(), want.Len())
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, got, "total_qty", []any{10.0, 20.0})
	assertColumnValues(t, got, "avg_qty", []any{10.0, 20.0})
	assertColumnValues(t, got, "fills", []any{int64(2), int64(2)})
	assertColumnValues(t, want, "total_qty", []any{10.0, 20.0})
	assertColumnValues(t, want, "avg_qty", []any{10.0, 20.0})
}

func TestQueryKernelGroupedProjectionWithoutAggregates(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "AAPL", "MSFT", "MSFT", "NVDA"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30, 30, 40})},
		Column{Name: "px", Data: NewF64([]float64{100, 101, 80, 82, 120})},
	)
	plan := QueryPlan{
		Source: frame,
		Where:  In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("AAPL"), Symbol("MSFT")}},
		By:     []Symbol{"sym"},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "bucket", Expr: Conditional{
				Cond: Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(30)}},
				Then: Literal{Value: Symbol("large")},
				Else: Literal{Value: Symbol("small")},
			}},
			{Name: "notional", Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "qty"}, Right: ColumnRef{Name: "px"}}},
		},
		Distinct: true,
		OrderBy:  []OrderSpec{{Column: "notional", Desc: true}},
		LimitN:   3,
	}

	ok, reason := QueryKernelSupportReason(plan)
	if !ok {
		t.Fatalf("QueryKernelSupportReason rejected grouped projection: %s", reason)
	}
	if !strings.Contains(reason, "grouped projection path") || !strings.Contains(reason, "conditional projection") {
		t.Fatalf("QueryKernelSupportReason = %q, want grouped conditional projection details", reason)
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		t.Fatal("CompileQueryKernel did not accept grouped projection")
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("QueryKernel Exec returned error: %v", err)
	}
	want, err := plan.Exec()
	if err != nil {
		t.Fatalf("fallback Exec returned error: %v", err)
	}
	if !SameSchema(got, want) || got.Len() != want.Len() {
		t.Fatalf("kernel schema/len = %#v/%d, want %#v/%d", got.Schema(), got.Len(), want.Schema(), want.Len())
	}
	assertColumnValues(t, got, "sym", []any{Symbol("MSFT"), Symbol("MSFT"), Symbol("AAPL")})
	assertColumnValues(t, got, "bucket", []any{Symbol("large"), Symbol("large"), Symbol("small")})
	assertColumnValues(t, got, "notional", []any{2460.0, 2400.0, 2020.0})
}

func TestQueryDistinctRows(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a"), Symbol("a")}),
		NewColumn("qty", []any{2, 5, 2, 3}),
	)

	got, err := From(frame).
		SelectColumns("sym", "qty").
		DistinctRows().
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("a")})
	assertColumnValues(t, got, "qty", []any{int64(2), int64(5), int64(3)})
}

func TestDistinctSingleColumnUsesReusableArrayIndex(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"a", "b", "a", "c", "b"}), ArrayAttributeGrouped)},
		NewColumn("qty", []any{10, 20, 30, 40, 50}),
	)

	got, err := Distinct(frame, "sym")
	if err != nil {
		t.Fatalf("Distinct returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "qty"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(20), int64(40)})
	sym, ok := got.Column("sym")
	if !ok {
		t.Fatal("distinct result is missing sym column")
	}
	if _, ok := ArrayIndexFor(sym, ArrayAttributeGrouped); !ok {
		t.Fatalf("distinct result sym metadata = %#v, want rebuilt grouped index", ArrayMetadataOf(sym))
	}
}

func TestQueryKernelDistinctSingleColumnHotPath(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols([]string{"a", "b", "a", "c", "b"}), ArrayAttributeGrouped)},
		NewColumn("qty", []any{10, 20, 30, 40, 50}),
	)
	plan := From(frame).SelectColumns("sym").DistinctRows()
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		t.Fatal("CompileQueryKernel did not accept distinct single-column query")
	}

	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("QueryKernel Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c")})
	sym, ok := got.Column("sym")
	if !ok {
		t.Fatal("kernel distinct result is missing sym column")
	}
	if _, ok := ArrayIndexFor(sym, ArrayAttributeGrouped); !ok {
		t.Fatalf("kernel distinct result metadata = %#v, want rebuilt grouped index", ArrayMetadataOf(sym))
	}
}

func TestQueryOrderByMultipleColumns(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a"), Symbol("b")}),
		NewColumn("qty", []any{2, 2, 5, 1}),
		NewColumn("seq", []any{1, 2, 3, 4}),
	)

	got, err := From(frame).
		OrderByColumns(
			OrderSpec{Column: "sym"},
			OrderSpec{Column: "qty", Desc: true},
		).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), Symbol("b")})
	assertColumnValues(t, got, "qty", []any{int64(5), int64(2), int64(2), int64(1)})
	assertColumnValues(t, got, "seq", []any{int64(3), int64(1), int64(2), int64(4)})
}

func TestDataFoundationGroupJoinAsofAndXbarBasics(t *testing.T) {
	trades := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(1_000),
			TimestampFromUnixNanos(1_900),
			TimestampFromUnixNanos(2_100),
			TimestampFromUnixNanos(2_400),
		}),
		NewColumn("qty", []any{int64(10), int64(20), int64(5), int64(7)}),
		NewColumn("venue", []any{Symbol("X"), Symbol("Y"), Symbol("X"), Symbol("X")}),
	)
	venues := mustFrame(t,
		NewColumn("venue", []any{Symbol("X"), Symbol("Y")}),
		NewColumn("region", []any{"us", "eu"}),
	)
	enriched, err := LeftJoin(trades, venues, "venue")
	if err != nil {
		t.Fatalf("LeftJoin returned error: %v", err)
	}
	assertColumnValues(t, enriched, "region", []any{"us", "eu", "us", "us"})

	quotes := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(900),
			TimestampFromUnixNanos(2_000),
			TimestampFromUnixNanos(2_000),
		}),
		NewColumn("bid", []any{99.5, 100.5, 50.25}),
	)
	joined, err := AsofJoin(enriched, quotes, "ts", "sym")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}
	assertColumnValues(t, joined, "bid", []any{99.5, 99.5, 50.25, 100.5})

	rollup, err := Exec(joined, QueryPlan{
		ByExprs: []SelectItem{
			{Name: "bucket", Expr: BucketFloorExpr{Expr: ColumnRef{Name: "ts"}, Interval: TimespanFromNanos(1_000)}},
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
		},
		Aggregates: []Aggregate{
			{Name: "qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "last_bid", Func: "last", Expr: ColumnRef{Name: "bid"}},
			{Name: "fills", Func: "count"},
		},
		OrderBy: []OrderSpec{{Column: "bucket"}, {Column: "sym"}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("Exec rollup returned error: %v", err)
	}
	assertColumnValues(t, rollup, "bucket", []any{Timestamp(1_000), Timestamp(2_000), Timestamp(2_000)})
	assertColumnValues(t, rollup, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, rollup, "qty", []any{30.0, 7.0, 5.0})
	assertColumnValues(t, rollup, "last_bid", []any{99.5, 100.5, 50.25})
	assertColumnValues(t, rollup, "fills", []any{int64(2), int64(1), int64(1)})
}

func TestGatherAndTakeOperators(t *testing.T) {
	array := NewI64([]int64{10, 20, 30})
	gathered, err := Gather(array, []int{2, 0, 2})
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}
	if got, want := gathered.Values(), []any{int64(30), int64(10), int64(30)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gathered values = %#v, want %#v", got, want)
	}
	if _, err := Gather(array, []int{3}); err == nil {
		t.Fatal("Gather accepted out-of-range index")
	}

	taken, err := Take(array, 2)
	if err != nil {
		t.Fatalf("Take returned error: %v", err)
	}
	if got, want := taken.Values(), []any{int64(10), int64(20)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("taken values = %#v, want %#v", got, want)
	}
	if _, err := Take(array, -1); err == nil {
		t.Fatal("Take accepted negative count")
	}
	head, err := TakeRepeat(NewI64Range(10, 2, 5), 3)
	if err != nil {
		t.Fatalf("TakeRepeat range head returned error: %v", err)
	}
	if _, ok := head.(i64RangeArray); !ok {
		t.Fatalf("TakeRepeat range head returned %T, want i64RangeArray", head)
	}
	if got, want := head.Values(), []any{int64(10), int64(12), int64(14)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TakeRepeat range head values = %#v, want %#v", got, want)
	}
	tail, err := TakeRepeat(NewI64Range(10, 2, 5), -2)
	if err != nil {
		t.Fatalf("TakeRepeat range tail returned error: %v", err)
	}
	if _, ok := tail.(i64RangeArray); !ok {
		t.Fatalf("TakeRepeat range tail returned %T, want i64RangeArray", tail)
	}
	if got, want := tail.Values(), []any{int64(16), int64(18)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TakeRepeat range tail values = %#v, want %#v", got, want)
	}
	sliced, err := Slice(NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA"}), 1, 2)
	if err != nil {
		t.Fatalf("Slice symbols returned error: %v", err)
	}
	if got, want := sliced.Values(), []any{Symbol("MSFT"), Symbol("NVDA")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Slice symbols values = %#v, want %#v", got, want)
	}
	reversedRange, handled, err := Reverse(NewI64Range(10, 2, 4))
	if err != nil {
		t.Fatalf("Reverse range returned error: %v", err)
	}
	if !handled {
		t.Fatal("Reverse range did not handle typed array")
	}
	if _, ok := reversedRange.(i64RangeArray); !ok {
		t.Fatalf("Reverse range returned %T, want i64RangeArray", reversedRange)
	}
	if got, want := reversedRange.Values(), []any{int64(16), int64(14), int64(12), int64(10)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Reverse range values = %#v, want %#v", got, want)
	}

	repeated, err := TakeRepeat(NewAny([]any{int64(1), NullValue, int64(2)}), 8)
	if err != nil {
		t.Fatalf("TakeRepeat returned error: %v", err)
	}
	if got, want := repeated.Values(), []any{int64(1), NullValue, int64(2), int64(1), NullValue, int64(2), int64(1), NullValue}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TakeRepeat values = %#v, want %#v", got, want)
	}
	if count, ok, err := TryTypedNullCount(repeated); err != nil || !ok || count != 3 {
		t.Fatalf("TakeRepeat null count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
	if count, ok, err := TryTypedDistinctCount(repeated); err != nil || !ok || count != 3 {
		t.Fatalf("TakeRepeat distinct count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
	if count, ok, err := TryTypedStringLikeCount(takeRepeatMust(t, NewSymbols([]string{"AAPL", "MSFT", "AMD", "ASK"}), 10), "A*"); err != nil || !ok || count != 7 {
		t.Fatalf("TakeRepeat symbol like count = %d,%v,%v; want 7,true,nil", count, ok, err)
	}

	repeatedTail, err := TakeRepeat(NewI64([]int64{10, 20, 30}), -5)
	if err != nil {
		t.Fatalf("TakeRepeat negative returned error: %v", err)
	}
	if got, want := repeatedTail.Values(), []any{int64(20), int64(30), int64(10), int64(20), int64(30)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TakeRepeat negative values = %#v, want %#v", got, want)
	}
	if count, ok, err := TryTypedDistinctCount(repeatedTail); err != nil || !ok || count != 3 {
		t.Fatalf("TakeRepeat negative distinct count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
	shortRepeat, err := TakeRepeat(NewI64([]int64{10, 20, 10, 30}), 2)
	if err != nil {
		t.Fatalf("TakeRepeat short returned error: %v", err)
	}
	if count, ok, err := TryTypedDistinctCount(shortRepeat); err != nil || !ok || count != 2 {
		t.Fatalf("TakeRepeat short distinct count = %d,%v,%v; want 2,true,nil", count, ok, err)
	}

	rotated, handled, err := TryTypedRotate(NewSymbols([]string{"AAPL", "MSFT", "NVDA", "AAPL"}), -1)
	if err != nil {
		t.Fatalf("TryTypedRotate symbols returned error: %v", err)
	}
	if !handled {
		t.Fatal("TryTypedRotate symbols did not handle typed array")
	}
	if _, ok := rotated.(tiledArray); !ok {
		t.Fatalf("TryTypedRotate symbols returned %T, want lazy tiledArray", rotated)
	}
	if got, want := rotated.Values(), []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("NVDA")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TryTypedRotate symbols values = %#v, want %#v", got, want)
	}
	if count, ok, err := TryTypedDistinctCount(rotated); err != nil || !ok || count != 3 {
		t.Fatalf("TryTypedRotate symbols distinct count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
}

func takeRepeatMust(t *testing.T, array Array, n int) Array {
	t.Helper()
	out, err := TakeRepeat(array, n)
	if err != nil {
		t.Fatalf("TakeRepeat returned error: %v", err)
	}
	return out
}

func TestTryTypedStringLikeCount(t *testing.T) {
	if count, ok, err := TryTypedStringLikeCount(NewString([]string{"AAPL", "MSFT", "AMD", "ASK"}), "A*"); err != nil || !ok || count != 3 {
		t.Fatalf("string like prefix count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
	if count, ok, err := TryTypedStringLikeCount(NewSymbols([]string{"AAPL", "MSFT", "AMD", "ASK"}), "A?D"); err != nil || !ok || count != 1 {
		t.Fatalf("symbol like glob count = %d,%v,%v; want 1,true,nil", count, ok, err)
	}
	if count, ok, err := TryTypedStringLikeCount(NewColumn("x", []any{"AAPL", NullValue, Symbol("ASK"), int64(1)}).Data, "A*"); err != nil || ok || count != 0 {
		t.Fatalf("mixed nullable like count = %d,%v,%v; want 0,false,nil", count, ok, err)
	}
	if _, ok, err := TryTypedStringLikeCount(NewI64([]int64{1, 2, 3}), "A*"); err != nil || ok {
		t.Fatalf("integer like count handled=%v err=%v; want false,nil", ok, err)
	}
}

func TestTryTypedInCount(t *testing.T) {
	if count, ok, err := TryTypedInCount(NewI32([]int32{10, 20, 30, 20, 40}), []any{int64(20), int32(40)}); err != nil || !ok || count != 3 {
		t.Fatalf("i32 in count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
	if count, ok, err := TryTypedInCount(NewSymbols([]string{"AAPL", "MSFT", "NVDA", "AAPL"}), []any{"NVDA", Symbol("AAPL")}); err != nil || !ok || count != 3 {
		t.Fatalf("symbol in count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
	repeated := takeRepeatMust(t, NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA"}), 10)
	if count, ok, err := TryTypedInCount(repeated, []any{Symbol("AAPL"), "MSFT"}); err != nil || !ok || count != 6 {
		t.Fatalf("tiled symbol in count = %d,%v,%v; want 6,true,nil", count, ok, err)
	}
	if count, ok, err := TryTypedInCount(NewColumn("x", []any{"AAPL", NullValue, Symbol("AAPL")}).Data, []any{"AAPL"}); err != nil || ok || count != 0 {
		t.Fatalf("nullable in count = %d,%v,%v; want 0,false,nil", count, ok, err)
	}
}

func TestTryTypedFbySum(t *testing.T) {
	out, ok, err := TryTypedFbySum(NewI64([]int64{1, 2, 3, 4}), NewSymbols([]string{"a", "a", "b", "b"}))
	if err != nil || !ok {
		t.Fatalf("TryTypedFbySum handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := out.Values(), []any{int64(3), int64(3), int64(7), int64(7)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fby sum values = %#v, want %#v", got, want)
	}
	groups := takeRepeatMust(t, NewSymbols([]string{"a", "b"}), 5)
	out, ok, err = TryTypedFbySum(NewI64Range(0, 1, 5), groups)
	if err != nil || !ok {
		t.Fatalf("TryTypedFbySum tiled handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := out.Values(), []any{int64(6), int64(4), int64(6), int64(4), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tiled fby sum values = %#v, want %#v", got, want)
	}
}

func TestTryTypedFbySumTotalAndGroupCount(t *testing.T) {
	groups := takeRepeatMust(t, NewSymbols([]string{"a", "b"}), 5)
	out, ok, err := TryTypedFbySum(NewI64Range(0, 1, 5), groups)
	if err != nil || !ok {
		t.Fatalf("TryTypedFbySum tiled handled=%v err=%v; want true,nil", ok, err)
	}
	if _, ok := out.(fbyI64TiledBroadcastArray); !ok {
		t.Fatalf("TryTypedFbySum tiled returned %T, want fbyI64TiledBroadcastArray", out)
	}
	total, ok, err := TryTypedFbySumTotal(NewI64Range(0, 1, 5), groups)
	if err != nil || !ok {
		t.Fatalf("TryTypedFbySumTotal handled=%v err=%v; want true,nil", ok, err)
	}
	if total != int64(26) {
		t.Fatalf("TryTypedFbySumTotal = %#v, want 26", total)
	}
	count, ok, err := TryTypedGroupCount(NewI64([]int64{1, 2, 1, 3, 2}))
	if err != nil || !ok || count != 3 {
		t.Fatalf("TryTypedGroupCount = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
}

func TestTryTypedBoolLogical(t *testing.T) {
	out, ok, err := TryTypedBoolLogical("and", NewBool([]bool{true, false, true}), NewBool([]bool{true, true, false}))
	if err != nil || !ok {
		t.Fatalf("TryTypedBoolLogical handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := out.Values(), []any{true, false, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bool logical and values = %#v, want %#v", got, want)
	}
	out, ok, err = TryTypedBoolLogical("or", NewBool([]bool{true, false, false}), true)
	if err != nil || !ok {
		t.Fatalf("TryTypedBoolLogical scalar handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := out.Values(), []any{true, true, true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bool logical or scalar values = %#v, want %#v", got, want)
	}
	if count, ok, err := TryTypedTrueCount(out); err != nil || !ok || count != 3 {
		t.Fatalf("true count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}

	cmp, ok, err := TryTypedDyadic(OpGE, NewI64Range(0, 1, 5), int64(2))
	if err != nil || !ok {
		t.Fatalf("range compare handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := cmp.(Array).Values(), []any{false, false, true, true, true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range compare values = %#v, want %#v", got, want)
	}
	if count, ok, err := TryTypedTrueCount(cmp.(Array)); err != nil || !ok || count != 3 {
		t.Fatalf("range compare true count = %d,%v,%v; want 3,true,nil", count, ok, err)
	}
	lower, ok, err := TryTypedDyadic(OpGE, NewI64Range(0, 1, 8), int64(2))
	if err != nil || !ok {
		t.Fatalf("lower range compare handled=%v err=%v; want true,nil", ok, err)
	}
	upper, ok, err := TryTypedDyadic(OpLT, NewI64Range(0, 1, 8), int64(6))
	if err != nil || !ok {
		t.Fatalf("upper range compare handled=%v err=%v; want true,nil", ok, err)
	}
	between, ok, err := TryTypedBoolLogical("and", lower, upper)
	if err != nil || !ok {
		t.Fatalf("range logical and handled=%v err=%v; want true,nil", ok, err)
	}
	if count, ok, err := TryTypedTrueCount(between); err != nil || !ok || count != 4 {
		t.Fatalf("range logical and true count = %d,%v,%v; want 4,true,nil", count, ok, err)
	}
	outside, ok, err := TryTypedBoolLogical("or", lower, upper)
	if err != nil || !ok {
		t.Fatalf("range logical or handled=%v err=%v; want true,nil", ok, err)
	}
	if count, ok, err := TryTypedTrueCount(outside); err != nil || !ok || count != 8 {
		t.Fatalf("range logical or true count = %d,%v,%v; want 8,true,nil", count, ok, err)
	}
}

func TestTryTypedScalarFill(t *testing.T) {
	column, err := NewColumnWithKind("x", KindI64, []any{int64(1), NullValue, int64(3), NullValue})
	if err != nil {
		t.Fatalf("NewColumnWithKind returned error: %v", err)
	}
	source := takeRepeatMust(t, column.Data, 8)
	out, ok, err := TryTypedScalarFill(int64(0), source)
	if err != nil || !ok {
		t.Fatalf("TryTypedScalarFill handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := out.Values(), []any{int64(1), int64(0), int64(3), int64(0), int64(1), int64(0), int64(3), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("scalar fill values = %#v, want %#v", got, want)
	}
	if sum, ok, err := TryTypedNumericSum(out); err != nil || !ok || sum != int64(8) {
		t.Fatalf("scalar fill sum = %#v,%v,%v; want 8,true,nil", sum, ok, err)
	}
	repeated, err := TakeRepeat(column.Data, 10)
	if err != nil {
		t.Fatalf("TakeRepeat scalar fill source returned error: %v", err)
	}
	out, ok, err = TryTypedScalarFill(int64(5), repeated)
	if err != nil || !ok {
		t.Fatalf("TryTypedScalarFill repeated handled=%v err=%v; want true,nil", ok, err)
	}
	if sum, ok, err := TryTypedNumericSum(out); err != nil || !ok || sum != int64(34) {
		t.Fatalf("repeated scalar fill sum = %#v,%v,%v; want 34,true,nil", sum, ok, err)
	}
}

func TestTryTypedSortIndexesI64(t *testing.T) {
	indexes, ok, err := TryTypedSortIndexesI64(NewI64Range(4, -1, 5), false)
	if err != nil || !ok {
		t.Fatalf("TryTypedSortIndexesI64 handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := indexes.Values(), []any{int64(4), int64(3), int64(2), int64(1), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sort indexes = %#v, want %#v", got, want)
	}
	gathered, ok, err := TryGatherByI64IndexArray(NewI64Range(4, -1, 5), indexes)
	if err != nil || !ok {
		t.Fatalf("TryGatherByI64IndexArray handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := gathered.Values(), []any{int64(0), int64(1), int64(2), int64(3), int64(4)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gathered sorted range = %#v, want %#v", got, want)
	}
}

func TestTryTypedSortIndexesI64TiledStable(t *testing.T) {
	symbols, err := TakeRepeat(NewSymbols([]string{"b", "a", "b", "c"}), 10)
	if err != nil {
		t.Fatalf("TakeRepeat symbols returned error: %v", err)
	}
	indexes, ok, err := TryTypedSortIndexesI64(symbols, false)
	if err != nil || !ok {
		t.Fatalf("TryTypedSortIndexesI64 tiled symbols handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := indexes.Values(), []any{int64(1), int64(5), int64(9), int64(0), int64(2), int64(4), int64(6), int64(8), int64(3), int64(7)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tiled symbol sort indexes = %#v, want %#v", got, want)
	}

	desc, ok, err := TryTypedSortIndexesI64(symbols, true)
	if err != nil || !ok {
		t.Fatalf("TryTypedSortIndexesI64 tiled symbols desc handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := desc.Values(), []any{int64(3), int64(7), int64(0), int64(2), int64(4), int64(6), int64(8), int64(1), int64(5), int64(9)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tiled symbol desc sort indexes = %#v, want %#v", got, want)
	}

	rotated, err := Slice(symbols, 1, 8)
	if err != nil {
		t.Fatalf("Slice tiled symbols returned error: %v", err)
	}
	rotatedIndexes, ok, err := TryTypedSortIndexesI64(rotated, false)
	if err != nil || !ok {
		t.Fatalf("TryTypedSortIndexesI64 rotated tiled symbols handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := rotatedIndexes.Values(), []any{int64(0), int64(4), int64(1), int64(3), int64(5), int64(7), int64(2), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rotated tiled symbol sort indexes = %#v, want %#v", got, want)
	}
}

func TestTryTypedSortIndexSumI64(t *testing.T) {
	symbols, err := TakeRepeat(NewSymbols([]string{"b", "a", "b", "c"}), 10)
	if err != nil {
		t.Fatalf("TakeRepeat symbols returned error: %v", err)
	}
	for _, descending := range []bool{false, true} {
		sum, ok, err := TryTypedSortIndexSumI64(symbols, descending)
		if err != nil || !ok {
			t.Fatalf("TryTypedSortIndexSumI64 descending=%v handled=%v err=%v; want true,nil", descending, ok, err)
		}
		if want := int64(45); sum != want {
			t.Fatalf("TryTypedSortIndexSumI64 descending=%v = %d, want %d", descending, sum, want)
		}
	}
}

func TestTryTypedSortIndexesNullableTemporalAndRankRange(t *testing.T) {
	dates := NewColumn("d", []any{Date(2), NullForKind(KindDate), Date(1)}).Data
	indexes, ok, err := TryTypedSortIndexesI64(dates, false)
	if err != nil || !ok {
		t.Fatalf("nullable date sort handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := indexes.Values(), []any{int64(1), int64(2), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nullable date sort indexes = %#v, want %#v", got, want)
	}
	rank, ok, err := TryTypedRankI64(NewI64Range(4, -1, 5))
	if err != nil || !ok {
		t.Fatalf("range rank handled=%v err=%v; want true,nil", ok, err)
	}
	if got, want := rank.Values(), []any{int64(4), int64(3), int64(2), int64(1), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range rank = %#v, want %#v", got, want)
	}
}

func TestTrySortFrameByColumnsUsesTypedGather(t *testing.T) {
	frame, err := NewFrame(
		NewColumn("sym", []any{"MSFT", "AAPL"}),
		NewColumn("price", []any{int64(101), int64(80)}),
	)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}
	sorted, ok, err := TrySortFrameByColumns(frame, []Symbol{"price"}, false)
	if err != nil || !ok {
		t.Fatalf("TrySortFrameByColumns handled=%v err=%v; want true,nil", ok, err)
	}
	price, _ := sorted.Column("price")
	if got, want := price.Values(), []any{int64(80), int64(101)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted price = %#v, want %#v", got, want)
	}
	sym, _ := sorted.Column("sym")
	if got, want := sym.Values(), []any{"AAPL", "MSFT"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted sym = %#v, want %#v", got, want)
	}
}

func TestTryTypedStringCastAndCasePreserveTiledArrays(t *testing.T) {
	repeated := takeRepeatMust(t, NewSymbols([]string{"aapl", "msft", "amd", "ask"}), 10)
	cast, handled, err := TryTypedStringCast(repeated)
	if err != nil || !handled {
		t.Fatalf("TryTypedStringCast handled=%v err=%v; want true,nil", handled, err)
	}
	if _, ok := cast.(tiledArray); !ok {
		t.Fatalf("TryTypedStringCast returned %T, want tiledArray", cast)
	}
	upper, handled, err := TryTypedStringCase(cast, true)
	if err != nil || !handled {
		t.Fatalf("TryTypedStringCase handled=%v err=%v; want true,nil", handled, err)
	}
	if _, ok := upper.(tiledArray); !ok {
		t.Fatalf("TryTypedStringCase returned %T, want tiledArray", upper)
	}
	if got, want := upper.Values(), []any{"AAPL", "MSFT", "AMD", "ASK", "AAPL", "MSFT", "AMD", "ASK", "AAPL", "MSFT"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("upper tiled values = %#v, want %#v", got, want)
	}
	if count, ok, err := TryTypedStringLikeCount(upper, "A*"); err != nil || !ok || count != 7 {
		t.Fatalf("upper tiled like count = %d,%v,%v; want 7,true,nil", count, ok, err)
	}
}

func TestTryGatherByI64IndexArrayPreservesRanges(t *testing.T) {
	values := NewI64Range(0, 1, 10)
	indexes := NewI64Range(4, 1, 3)
	gathered, handled, err := TryGatherByI64IndexArray(values, indexes)
	if err != nil || !handled {
		t.Fatalf("TryGatherByI64IndexArray handled=%v err=%v; want true,nil", handled, err)
	}
	if _, ok := gathered.(i64RangeArray); !ok {
		t.Fatalf("TryGatherByI64IndexArray returned %T, want i64RangeArray", gathered)
	}
	if got, want := gathered.Values(), []any{int64(4), int64(5), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("range gather values = %#v, want %#v", got, want)
	}

	rows, handled, err := TryTypedI64Indexes(indexes)
	if err != nil || !handled || !reflect.DeepEqual(rows, []int{4, 5, 6}) {
		t.Fatalf("TryTypedI64Indexes = %#v,%v,%v; want [4 5 6],true,nil", rows, handled, err)
	}
}

func TestTryTypedAmendIndexesI64(t *testing.T) {
	values := takeRepeatMust(t, NewI64([]int64{0}), 6)
	amended, handled, err := TryTypedAmendIndexes(values, []int{1, 4}, []any{int64(10), int64(20)})
	if err != nil || !handled {
		t.Fatalf("TryTypedAmendIndexes handled=%v err=%v; want true,nil", handled, err)
	}
	if got, want := amended.Values(), []any{int64(0), int64(10), int64(0), int64(0), int64(20), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("amended values = %#v, want %#v", got, want)
	}
}

func TestTryTypedAmendIndexesI64SparseOverlay(t *testing.T) {
	amended, handled, err := TryTypedAmendIndexes(NewI64Range(0, 1, 16), []int{2, 4, 7}, []any{int64(20), int64(40), int64(70)})
	if err != nil || !handled {
		t.Fatalf("TryTypedAmendIndexes handled=%v err=%v; want true,nil", handled, err)
	}
	if _, ok := amended.(i64SparseAmendArray); !ok {
		t.Fatalf("TryTypedAmendIndexes returned %T, want i64SparseAmendArray", amended)
	}
	if got, want := amended.Values(), []any{int64(0), int64(1), int64(20), int64(3), int64(40), int64(5), int64(6), int64(70), int64(8), int64(9), int64(10), int64(11), int64(12), int64(13), int64(14), int64(15)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sparse amended values = %#v, want %#v", got, want)
	}
	sum, handled, err := TryTypedNumericSum(amended)
	if err != nil || !handled || sum != int64(237) {
		t.Fatalf("TryTypedNumericSum sparse amend = %#v,%v,%v; want 237,true,nil", sum, handled, err)
	}
	n, ok, err := typedKernels.NumericAt(amended, 4)
	if err != nil || !ok || n != 40 {
		t.Fatalf("NumericAt sparse amend = %v,%v,%v; want 40,true,nil", n, ok, err)
	}
	if got, want := amended.Gather([]int{7, 0, 4}).Values(), []any{int64(70), int64(0), int64(40)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sparse amend gather = %#v, want %#v", got, want)
	}
}

func TestTryTypedAmendIndexesI64SparseOverlayFallsBackForOrderedSemantics(t *testing.T) {
	amended, handled, err := TryTypedAmendIndexes(NewI64Range(0, 1, 16), []int{4, 2}, []any{int64(40), int64(20)})
	if err != nil || !handled {
		t.Fatalf("TryTypedAmendIndexes handled=%v err=%v; want true,nil", handled, err)
	}
	if _, ok := amended.(i64SparseAmendArray); ok {
		t.Fatal("TryTypedAmendIndexes used sparse overlay for unsorted indexes")
	}
	if got, want := amended.Values(), []any{int64(0), int64(1), int64(20), int64(3), int64(40), int64(5), int64(6), int64(7), int64(8), int64(9), int64(10), int64(11), int64(12), int64(13), int64(14), int64(15)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dense fallback values = %#v, want %#v", got, want)
	}
}

func TestFrameGatherTakeAndFilterMask(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("c"), Symbol("d")}),
		NewColumn("qty", []any{1, 2, 3, 4}),
	)

	gathered, err := GatherFrame(frame, []int{3, 1})
	if err != nil {
		t.Fatalf("GatherFrame returned error: %v", err)
	}
	assertColumnValues(t, gathered, "sym", []any{Symbol("d"), Symbol("b")})
	assertColumnValues(t, gathered, "qty", []any{int64(4), int64(2)})
	if _, err := GatherFrame(frame, []int{-1}); err == nil {
		t.Fatal("GatherFrame accepted negative index")
	}

	taken, err := TakeFrame(frame, 10)
	if err != nil {
		t.Fatalf("TakeFrame returned error: %v", err)
	}
	if got := taken.Len(); got != frame.Len() {
		t.Fatalf("TakeFrame length = %d, want %d", got, frame.Len())
	}

	filtered, err := FilterMask(frame, NewColumn("mask", []any{true, nil, false, true}).Data)
	if err != nil {
		t.Fatalf("FilterMask returned error: %v", err)
	}
	assertColumnValues(t, filtered, "sym", []any{Symbol("a"), Symbol("d")})
	if _, err := FilterMask(frame, NewBool([]bool{true})); err == nil {
		t.Fatal("FilterMask accepted mismatched mask length")
	}
}

func TestWhereMaskRejectsNonBoolMask(t *testing.T) {
	indexes, err := WhereMask(NewColumn("mask", []any{true, nil, false, true}).Data)
	if err != nil {
		t.Fatalf("WhereMask returned error: %v", err)
	}
	if got, want := indexes, []int{0, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WhereMask indexes = %v, want %v", got, want)
	}
	if _, err := WhereMask(NewString([]string{"true"})); err == nil {
		t.Fatal("WhereMask accepted non-bool mask")
	}
}

func TestFilterOperator(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("c")}),
		NewColumn("qty", []any{1, 3, 2}),
	)

	got, err := Filter(frame, func(row map[Symbol]any) (bool, error) {
		return row["qty"].(int64) >= 2, nil
	})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("b"), Symbol("c")})
	assertColumnValues(t, got, "qty", []any{int64(3), int64(2)})
	if _, err := Filter(frame, nil); err == nil {
		t.Fatal("Filter accepted nil predicate")
	}
}

func TestUpdateWhereReturnsNewFrame(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("qty", []any{2, 5, 3}),
		NewColumn("status", []any{"new", "new", "new"}),
	)

	got, err := UpdateWhere(frame,
		Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("a")}},
		map[Symbol]Expr{
			"qty":    Binary{Op: OpAdd, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: 10}},
			"status": Literal{Value: "done"},
		},
	)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "qty", "status"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("a")})
	assertColumnValues(t, got, "qty", []any{int64(12), int64(5), int64(13)})
	assertColumnValues(t, got, "status", []any{"done", "new", "done"})
	assertColumnValues(t, frame, "qty", []any{int64(2), int64(5), int64(3)})
	assertColumnValues(t, frame, "status", []any{"new", "new", "new"})
}

func TestConditionalSelectsElementwiseWithScalarBranchesAndNullFalse(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("side", []any{Symbol("buy"), Symbol("sell"), NullValue, Symbol("buy")}),
		NewColumn("price", []any{101.0, 99.0, 100.0, 102.0}),
		NewColumn("arrival", []any{100.0, 100.0, 100.0, 100.0}),
	)

	plan := QueryPlan{
		Source: frame,
		LimitN: -1,
		Select: []SelectItem{
			{Name: "side", Expr: ColumnRef{Name: "side"}},
			{
				Name: "slip",
				Expr: Conditional{
					Cond: Binary{Op: OpEQ, Left: ColumnRef{Name: "side"}, Right: Literal{Value: Symbol("buy")}},
					Then: Binary{Op: OpSub, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "arrival"}},
					Else: Binary{Op: OpSub, Left: ColumnRef{Name: "arrival"}, Right: ColumnRef{Name: "price"}},
				},
			},
			{
				Name: "label",
				Expr: Conditional{
					Cond: Binary{Op: OpEQ, Left: ColumnRef{Name: "side"}, Right: Literal{Value: Symbol("buy")}},
					Then: Literal{Value: Symbol("paid")},
					Else: Literal{Value: Symbol("received")},
				},
			},
			{
				Name: "null_cond",
				Expr: Conditional{
					Cond: Literal{Value: NullValue},
					Then: Literal{Value: "then"},
					Else: Literal{Value: "else"},
				},
			},
		},
	}
	got, err := plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertColumnValues(t, got, "slip", []any{1.0, 1.0, 0.0, 2.0})
	assertColumnValues(t, got, "label", []any{Symbol("paid"), Symbol("received"), Symbol("received"), Symbol("paid")})
	assertColumnValues(t, got, "null_cond", []any{"else", "else", "else", "else"})
}

func TestConditionalPreservesTypedNullsAndBroadcastSemantics(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "take", Data: NewBool([]bool{true, false, true, false})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20, 30, 40})},
		Column{Name: "px", Data: NewF64([]float64{1.5, 2.5, 3.5, 4.5})},
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "AAPL", "TSLA"})},
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		LimitN: -1,
		Select: []SelectItem{
			{
				Name: "maybe_qty",
				Expr: Conditional{
					Cond: ColumnRef{Name: "take"},
					Then: ColumnRef{Name: "qty"},
					Else: Literal{Value: NullForKind(KindI32)},
				},
			},
			{
				Name: "broadcast_score",
				Expr: Conditional{
					Cond: Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: "AAPL"}},
					Then: Binary{Op: OpMul, Left: ColumnRef{Name: "px"}, Right: Literal{Value: int64(10)}},
					Else: Binary{Op: OpSub, Left: Literal{Value: 100.0}, Right: ColumnRef{Name: "qty"}},
				},
			},
			{
				Name: "sym_or_string",
				Expr: Conditional{
					Cond: Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: "AAPL"}},
					Then: Literal{Value: Symbol("hit")},
					Else: Literal{Value: "miss"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	maybeQty, ok := got.Column("maybe_qty")
	if !ok {
		t.Fatal("maybe_qty column missing")
	}
	if maybeQty.Kind() != KindI32 {
		t.Fatalf("maybe_qty kind = %s, want %s", maybeQty.Kind(), KindI32)
	}
	assertColumnValues(t, got, "maybe_qty", []any{int32(10), NullValue, int32(30), NullValue})
	assertColumnValues(t, got, "broadcast_score", []any{15.0, 80.0, 35.0, 60.0})
	if kind, ok := got.Schema().Kind("sym_or_string"); !ok || kind != KindSymbol {
		t.Fatalf("sym_or_string kind = %s, ok %v; want %s", kind, ok, KindSymbol)
	}
	assertColumnValues(t, got, "sym_or_string", []any{Symbol("hit"), Symbol("miss"), Symbol("hit"), Symbol("miss")})
}

func TestQueryKernelSupportsConditionalProjectionAndWhere(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "AAPL", "TSLA"})},
		Column{Name: "side", Data: NewSymbols([]string{"buy", "sell", "sell", "buy"})},
		Column{Name: "price", Data: NewF64([]float64{101.0, 99.0, 98.5, 105.0})},
		Column{Name: "arrival", Data: NewF64([]float64{100.0, 100.0, 99.0, 104.0})},
	)
	plan := QueryPlan{
		Source: frame,
		Where: Conditional{
			Cond: Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("AAPL")}},
			Then: Binary{Op: OpGT, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 100.0}},
			Else: Binary{Op: OpLT, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 100.0}},
		},
		LimitN: -1,
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{
				Name: "slip",
				Expr: Conditional{
					Cond: Binary{Op: OpEQ, Left: ColumnRef{Name: "side"}, Right: Literal{Value: Symbol("buy")}},
					Then: Binary{Op: OpSub, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "arrival"}},
					Else: Binary{Op: OpSub, Left: ColumnRef{Name: "arrival"}, Right: ColumnRef{Name: "price"}},
				},
			},
		},
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		_, reason := QueryKernelSupportReason(plan)
		t.Fatalf("CompileQueryKernel ok = false, reason: %s", reason)
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("kernel Exec returned error: %v", err)
	}
	want, err := plan.Exec()
	if err != nil {
		t.Fatalf("fallback Exec returned error: %v", err)
	}
	if !reflect.DeepEqual(got.Schema().Names(), want.Schema().Names()) {
		t.Fatalf("kernel schema = %#v, want %#v", got.Schema().Names(), want.Schema().Names())
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, got, "slip", []any{1.0, 1.0})
	assertColumnValues(t, want, "slip", []any{1.0, 1.0})
}

func TestQueryKernelSupportsBooleanProjectionExpressions(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA"})},
		Column{Name: "qty", Data: NewI32([]int32{2, 7, 5, 1})},
		Column{Name: "px", Data: NewF64([]float64{101.0, 99.0, 103.0, 98.5})},
	)
	plan := QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int32(2)}},
		LimitN: -1,
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "liquid", Expr: Within{Expr: ColumnRef{Name: "qty"}, Low: int32(2), High: int32(6), HighClosed: true}},
			{Name: "tech", Expr: In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("AAPL"), Symbol("NVDA")}}},
			{Name: "not_msft", Expr: Not{Expr: In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("MSFT")}}}},
			{
				Name: "tradable",
				Expr: Logical{
					Op: "and",
					Left: Within{
						Expr:       ColumnRef{Name: "px"},
						Low:        100.0,
						High:       104.0,
						HighClosed: false,
					},
					Right: Logical{
						Op:    "or",
						Left:  In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("AAPL")}},
						Right: In{Expr: ColumnRef{Name: "sym"}, Values: []any{Symbol("NVDA")}},
					},
				},
			},
		},
	}

	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		t.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		_, reason := QueryKernelSupportReason(plan)
		t.Fatalf("CompileQueryKernel ok = false, reason: %s", reason)
	}
	got, err := kernel.Exec(frame)
	if err != nil {
		t.Fatalf("kernel Exec returned error: %v", err)
	}
	want, err := plan.Exec()
	if err != nil {
		t.Fatalf("fallback Exec returned error: %v", err)
	}
	if !SameSchema(got, want) {
		t.Fatalf("kernel schema = %#v, want %#v", got.Schema(), want.Schema())
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("NVDA")})
	assertColumnValues(t, got, "liquid", []any{true, false, true})
	assertColumnValues(t, got, "tech", []any{true, false, true})
	assertColumnValues(t, got, "not_msft", []any{true, false, true})
	assertColumnValues(t, got, "tradable", []any{true, false, true})
	assertColumnValues(t, want, "tradable", []any{true, false, true})
}

func TestUpdatePredicateReturnsNewFrame(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("c")}),
		NewColumn("qty", []any{1, 3, 2}),
	)

	got, err := Update(frame,
		func(row map[Symbol]any) (bool, error) {
			return row["qty"].(int64) >= 2, nil
		},
		map[Symbol]func(row map[Symbol]any) (any, error){
			"sym": func(row map[Symbol]any) (any, error) {
				return Symbol("x"), nil
			},
		},
	)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("x"), Symbol("x")})
	assertColumnValues(t, got, "qty", []any{int64(1), int64(3), int64(2)})
	assertColumnValues(t, frame, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c")})
}

func TestUpdateRejectsInvalidInputs(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a")}),
		NewColumn("qty", []any{1}),
	)

	if _, err := Update(frame, nil, map[Symbol]func(row map[Symbol]any) (any, error){
		"qty": func(row map[Symbol]any) (any, error) { return int64(2), nil },
	}); err == nil {
		t.Fatal("Update accepted nil predicate")
	}
	if _, err := UpdateWhere(frame, nil, nil); err == nil {
		t.Fatal("UpdateWhere accepted empty assignments")
	}
	if _, err := UpdateWhere(frame, Literal{Value: "yes"}, map[Symbol]Expr{"qty": Literal{Value: 2}}); err == nil {
		t.Fatal("UpdateWhere accepted non-bool where expression")
	}
}

func TestUpdateWhereAppendsComputedColumns(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("price", []any{100.0, 80.0, 120.0}),
		NewColumn("size", []any{10, 20, 30}),
	)

	got, err := UpdateWhere(frame,
		Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("a")}},
		map[Symbol]Expr{
			"notional": Binary{Op: OpMul, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "size"}},
		},
	)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "price", "size", "notional"})
	assertColumnValues(t, got, "notional", []any{1000.0, NullValue, 3600.0})
	assertColumnValues(t, frame, "sym", []any{Symbol("a"), Symbol("b"), Symbol("a")})
}

func TestUpdateWhereConditionalAssignment(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("side", []any{Symbol("buy"), Symbol("sell"), Symbol("buy")}),
		NewColumn("size", []any{10, 15, 20}),
	)

	got, err := UpdateWhere(frame, nil, map[Symbol]Expr{
		"signed_qty": Conditional{
			Cond: Binary{Op: OpEQ, Left: ColumnRef{Name: "side"}, Right: Literal{Value: Symbol("buy")}},
			Then: ColumnRef{Name: "size"},
			Else: Binary{Op: OpSub, Left: Literal{Value: int64(0)}, Right: ColumnRef{Name: "size"}},
		},
	})
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	assertColumnValues(t, got, "signed_qty", []any{10.0, -15.0, 20.0})
}

func TestUpdateByWritesGroupedAggregates(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("price", []any{100.0, 80.0, 120.0}),
		NewColumn("size", []any{10, 20, 30}),
	)

	got, err := UpdateBy(frame,
		nil,
		[]SelectItem{{Name: "sym", Expr: ColumnRef{Name: "sym"}}},
		[]GroupedAssignment{
			{Name: "avg_price", Func: "avg", Expr: ColumnRef{Name: "price"}},
			{Name: "fills", Func: "count", Expr: ColumnRef{Name: "price"}},
		},
	)
	if err != nil {
		t.Fatalf("UpdateBy returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "price", "size", "avg_price", "fills"})
	assertColumnValues(t, got, "avg_price", []any{110.0, 80.0, 110.0})
	assertColumnValues(t, got, "fills", []any{int64(2), int64(1), int64(2)})

	filtered, err := UpdateBy(frame,
		Binary{Op: OpGT, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 90.0}},
		[]SelectItem{{Name: "sym", Expr: ColumnRef{Name: "sym"}}},
		[]GroupedAssignment{{Name: "avg_price", Func: "avg", Expr: ColumnRef{Name: "price"}}},
	)
	if err != nil {
		t.Fatalf("filtered UpdateBy returned error: %v", err)
	}
	assertColumnValues(t, filtered, "avg_price", []any{110.0, NullValue, 110.0})
}

func TestUpdateByWritesExtendedGroupedAggregates(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("price", []any{10.0, 30.0, 20.0}),
		NewColumn("size", []any{1, 3, 5}),
	)

	got, err := UpdateBy(frame,
		nil,
		[]SelectItem{{Name: "sym", Expr: ColumnRef{Name: "sym"}}},
		[]GroupedAssignment{
			{Name: "var_price", Func: "var", Expr: ColumnRef{Name: "price"}},
			{Name: "dev_price", Func: "dev", Expr: ColumnRef{Name: "price"}},
			{Name: "med_price", Func: "med", Expr: ColumnRef{Name: "price"}},
			{Name: "wavg_price", Func: "wavg", Expr: ColumnRef{Name: "price"}, Weight: ColumnRef{Name: "size"}},
		},
	)
	if err != nil {
		t.Fatalf("UpdateBy returned error: %v", err)
	}
	assertColumnValues(t, got, "var_price", []any{100.0, 100.0, 0.0})
	assertColumnValues(t, got, "dev_price", []any{10.0, 10.0, 0.0})
	assertColumnValues(t, got, "med_price", []any{20.0, 20.0, 20.0})
	assertColumnValues(t, got, "wavg_price", []any{25.0, 25.0, 20.0})
}

func TestUpdatePreservesColumnKindAndRejectsIncompatibleValues(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
		Column{Name: "qty", Data: NewI64([]int64{10, 20})},
	)

	updated, err := UpdateWhere(frame,
		Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("AAPL")}},
		map[Symbol]Expr{
			"sym": Literal{Value: "NVDA"},
			"qty": Literal{Value: int64(11)},
		},
	)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	if kind, _ := updated.Schema().Kind("sym"); kind != KindSymbol {
		t.Fatalf("sym kind = %s, want %s", kind, KindSymbol)
	}
	if kind, _ := updated.Schema().Kind("qty"); kind != KindI64 {
		t.Fatalf("qty kind = %s, want %s", kind, KindI64)
	}
	assertColumnValues(t, updated, "sym", []any{Symbol("NVDA"), Symbol("MSFT")})
	assertColumnValues(t, updated, "qty", []any{int64(11), int64(20)})

	if _, err := UpdateWhere(frame, nil, map[Symbol]Expr{"qty": Literal{Value: "bad"}}); err == nil {
		t.Fatal("UpdateWhere accepted incompatible typed column value")
	}
}

func TestDeleteWhereAndPredicateReturnNewFrame(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("c"), Symbol("d")}),
		NewColumn("qty", []any{1, 3, 2, 4}),
	)

	whereDeleted, err := DeleteWhere(frame, Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(3)}})
	if err != nil {
		t.Fatalf("DeleteWhere returned error: %v", err)
	}
	assertColumnValues(t, whereDeleted, "sym", []any{Symbol("a"), Symbol("c")})
	assertColumnValues(t, whereDeleted, "qty", []any{int64(1), int64(2)})
	assertColumnValues(t, frame, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c"), Symbol("d")})

	predicateDeleted, err := Delete(frame, func(row map[Symbol]any) (bool, error) {
		return row["sym"] == Symbol("a") || row["sym"] == Symbol("d"), nil
	})
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	assertColumnValues(t, predicateDeleted, "sym", []any{Symbol("b"), Symbol("c")})
	assertColumnValues(t, predicateDeleted, "qty", []any{int64(3), int64(2)})
	if _, err := Delete(frame, nil); err == nil {
		t.Fatal("Delete accepted nil predicate")
	}
	if _, err := DeleteWhere(frame, Literal{Value: "yes"}); err == nil {
		t.Fatal("DeleteWhere accepted non-bool where expression")
	}
}

func TestDeleteWhereUsesIndexedComplementHotPath(t *testing.T) {
	sym := &countingMetadataArray{
		array: WithArrayAttribute(NewSymbols([]string{"AAPL", "MSFT", "AAPL", "NVDA", "AAPL"}), ArrayAttributeGrouped),
	}
	frame := mustFrame(t,
		Column{Name: "sym", Data: sym},
		NewColumn("qty", []any{10, 20, 30, 40, 50}),
	)

	got, err := DeleteWhere(frame, Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("AAPL")}})
	if err != nil {
		t.Fatalf("DeleteWhere returned error: %v", err)
	}
	if sym.ats != 0 {
		t.Fatalf("indexed delete key column At called %d times; want index+complement path", sym.ats)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("MSFT"), Symbol("NVDA")})
	assertColumnValues(t, got, "qty", []any{int64(20), int64(40)})
}

func TestDropAndRenameColumns(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"a", "b"})},
		Column{Name: "qty", Data: NewI32([]int32{10, 20})},
		NewColumn("venue", []any{"x", "y"}),
	)

	dropped, err := DropColumns(frame, "venue")
	if err != nil {
		t.Fatalf("DropColumns returned error: %v", err)
	}
	assertColumnNames(t, dropped, []Symbol{"sym", "qty"})
	assertColumnValues(t, dropped, "sym", []any{Symbol("a"), Symbol("b")})
	if kind, _ := dropped.Schema().Kind("qty"); kind != KindI32 {
		t.Fatalf("dropped qty kind = %s, want %s", kind, KindI32)
	}
	if _, err := DropColumns(frame, "missing"); err == nil {
		t.Fatal("DropColumns accepted missing column")
	}
	if _, err := DropColumns(frame, "sym", "qty", "venue"); err == nil {
		t.Fatal("DropColumns accepted removing all columns")
	}

	renamed, err := RenameColumns(frame, map[Symbol]Symbol{"sym": "ticker", "venue": "market"})
	if err != nil {
		t.Fatalf("RenameColumns returned error: %v", err)
	}
	assertColumnNames(t, renamed, []Symbol{"ticker", "qty", "market"})
	assertColumnValues(t, renamed, "ticker", []any{Symbol("a"), Symbol("b")})
	if kind, _ := renamed.Schema().Kind("ticker"); kind != KindSymbol {
		t.Fatalf("renamed ticker kind = %s, want %s", kind, KindSymbol)
	}
	assertColumnNames(t, frame, []Symbol{"sym", "qty", "venue"})
	if _, err := RenameColumns(frame, map[Symbol]Symbol{"missing": "x"}); err == nil {
		t.Fatal("RenameColumns accepted missing column")
	}
	if _, err := RenameColumns(frame, map[Symbol]Symbol{"sym": "qty"}); err == nil {
		t.Fatal("RenameColumns accepted duplicate output column")
	}
}

func TestDistinctOperator(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), Symbol("a"), nil}),
		NewColumn("venue", []any{"x", "x", "x", "y", "x"}),
		NewColumn("qty", []any{1, 2, 3, 4, 5}),
	)

	got, err := Distinct(frame, "sym", "venue")
	if err != nil {
		t.Fatalf("Distinct returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("a"), NullValue})
	assertColumnValues(t, got, "venue", []any{"x", "x", "y", "x"})
	assertColumnValues(t, got, "qty", []any{int64(1), int64(3), int64(4), int64(5)})

	all, err := Distinct(frame)
	if err != nil {
		t.Fatalf("Distinct all columns returned error: %v", err)
	}
	if got := all.Len(); got != frame.Len() {
		t.Fatalf("Distinct all columns length = %d, want %d", got, frame.Len())
	}
	if _, err := Distinct(frame, "missing"); err == nil {
		t.Fatal("Distinct accepted missing key column")
	}
}

func TestKeyedFrameSingleKeyLookup(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a"), Symbol("c")}),
		NewColumn("qty", []any{10, 20, 30, 40}),
	)

	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	if got, want := keyed.Keys(), []Symbol{"sym"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}

	got, err := LookupByKey(keyed, "a")
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("a")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(30)})

	missing, err := keyed.LookupByKey(Symbol("z"))
	if err != nil {
		t.Fatalf("missing LookupByKey returned error: %v", err)
	}
	assertColumnNames(t, missing, []Symbol{"sym", "qty"})
	if got := missing.Len(); got != 0 {
		t.Fatalf("missing lookup length = %d, want 0", got)
	}
	if got, ok := missing.Schema().Kind("qty"); !ok || got != KindI64 {
		t.Fatalf("missing lookup qty kind = %s, ok %v; want %s", got, ok, KindI64)
	}
}

func TestKeyedFrameTemporalLookupCoercesStringKey(t *testing.T) {
	ts, err := NewColumnWithKind("ts", KindTimestamp, []any{
		TimestampFromUnixNanos(100),
		TimestampFromUnixNanos(200),
		TimestampFromUnixNanos(200),
	})
	if err != nil {
		t.Fatalf("NewColumnWithKind returned error: %v", err)
	}
	frame := mustFrame(t,
		ts,
		NewColumn("seq", []any{1, 2, 3}),
	)
	keyed, err := KeyBy(frame, "ts")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}

	got, err := keyed.LookupByKey("1970-01-01T00:00:00.0000002Z")
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, got, "seq", []any{int64(2), int64(3)})

	missing, err := keyed.LookupByKey("1970-01-01T00:00:00.0000003Z")
	if err != nil {
		t.Fatalf("missing LookupByKey returned error: %v", err)
	}
	if got := missing.Len(); got != 0 {
		t.Fatalf("missing lookup length = %d, want 0", got)
	}
	if got, ok := missing.Schema().Kind("ts"); !ok || got != KindTimestamp {
		t.Fatalf("missing ts kind = %s, ok %v; want %s", got, ok, KindTimestamp)
	}
}

func TestKeyedFrameMultiKeyLookup(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), nil}),
		NewColumn("venue", []any{"x", "y", "x", "x"}),
		NewColumn("qty", []any{10, 20, 30, 40}),
	)

	keyed, err := KeyBy(frame, "sym", "venue")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	got, err := keyed.LookupByKey("a", "y")
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("a")})
	assertColumnValues(t, got, "venue", []any{"y"})
	assertColumnValues(t, got, "qty", []any{int64(20)})

	nullKey, err := keyed.LookupByKey(nil, "x")
	if err != nil {
		t.Fatalf("null LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, nullKey, "sym", []any{NullValue})
	assertColumnValues(t, nullKey, "venue", []any{"x"})
	assertColumnValues(t, nullKey, "qty", []any{int64(40)})
}

func TestKeyedFrameMultiKeyDuplicateAndMissingLookupKeepsSchema(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("venue", []any{Symbol("x"), Symbol("x"), Symbol("y"), Symbol("x")}),
		NewColumn("qty", []any{10, 20, 30, 40}),
		NewColumn("note", []any{"first", "second", "other", "miss"}),
	)

	keyed, err := KeyBy(frame, "sym", "venue")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	got, err := keyed.LookupByKey("a", "x")
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "venue", "qty", "note"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("a")})
	assertColumnValues(t, got, "venue", []any{Symbol("x"), Symbol("x")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(20)})
	assertColumnValues(t, got, "note", []any{"first", "second"})

	missing, err := keyed.LookupByKey("z", "x")
	if err != nil {
		t.Fatalf("missing LookupByKey returned error: %v", err)
	}
	assertColumnNames(t, missing, []Symbol{"sym", "venue", "qty", "note"})
	if got := missing.Len(); got != 0 {
		t.Fatalf("missing lookup length = %d, want 0", got)
	}
	for name, kind := range map[Symbol]Kind{"sym": KindSymbol, "venue": KindSymbol, "qty": KindI64, "note": KindString} {
		if got, ok := missing.Schema().Kind(name); !ok || got != kind {
			t.Fatalf("missing lookup %s kind = %s, ok %v; want %s", name, got, ok, kind)
		}
	}

	all := keyed.Frame()
	assertColumnNames(t, all, []Symbol{"sym", "venue", "qty", "note"})
	assertColumnValues(t, all, "qty", []any{int64(10), int64(20), int64(30), int64(40)})

	valueHit, err := keyed.LookupValueByKey("a", "x")
	if err != nil {
		t.Fatalf("LookupValueByKey returned error: %v", err)
	}
	assertColumnNames(t, valueHit, []Symbol{"qty", "note"})
	assertColumnValues(t, valueHit, "qty", []any{int64(20)})
	assertColumnValues(t, valueHit, "note", []any{"second"})

	valueMissing, err := keyed.LookupValueByKey("z", "x")
	if err != nil {
		t.Fatalf("missing LookupValueByKey returned error: %v", err)
	}
	assertColumnNames(t, valueMissing, []Symbol{"qty", "note"})
	if got := valueMissing.Len(); got != 0 {
		t.Fatalf("missing value lookup length = %d, want 0", got)
	}

	valueFrame, err := keyed.ValueFrame()
	if err != nil {
		t.Fatalf("ValueFrame returned error: %v", err)
	}
	assertColumnNames(t, valueFrame, []Symbol{"qty", "note"})
	assertColumnValues(t, valueFrame, "note", []any{"first", "second", "other", "miss"})
}

func TestKeyedFrameKeyValueLookupKDBSubsetSemantics(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("bucket", []any{"09:30", "09:30", "09:31"}),
		NewColumn("seq", []any{1, 2, 3}),
		NewColumn("px", []any{100.0, 100.5, 80.0}),
	)

	keyed, err := KeyBy(frame, "sym", "bucket")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}

	keyFrame, err := keyed.KeyFrame()
	if err != nil {
		t.Fatalf("KeyFrame returned error: %v", err)
	}
	assertColumnNames(t, keyFrame, []Symbol{"sym", "bucket"})
	assertColumnValues(t, keyFrame, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, keyFrame, "bucket", []any{"09:30", "09:30", "09:31"})

	valueFrame, err := keyed.ValueFrame()
	if err != nil {
		t.Fatalf("ValueFrame returned error: %v", err)
	}
	assertColumnNames(t, valueFrame, []Symbol{"seq", "px"})
	assertColumnValues(t, valueFrame, "seq", []any{int64(1), int64(2), int64(3)})

	allRows, err := keyed.LookupByKey("AAPL", "09:30")
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnNames(t, allRows, []Symbol{"sym", "bucket", "seq", "px"})
	assertColumnValues(t, allRows, "seq", []any{int64(1), int64(2)})

	valueRow, err := keyed.LookupValueByKey("AAPL", "09:30")
	if err != nil {
		t.Fatalf("LookupValueByKey returned error: %v", err)
	}
	assertColumnNames(t, valueRow, []Symbol{"seq", "px"})
	assertColumnValues(t, valueRow, "seq", []any{int64(2)})
	assertColumnValues(t, valueRow, "px", []any{100.5})

	missing, err := keyed.LookupValueByKey("AAPL", "09:32")
	if err != nil {
		t.Fatalf("missing LookupValueByKey returned error: %v", err)
	}
	assertColumnNames(t, missing, []Symbol{"seq", "px"})
	if got := missing.Len(); got != 0 {
		t.Fatalf("missing value lookup length = %d, want 0", got)
	}
	if got, ok := missing.Schema().Kind("px"); !ok || got != KindF64 {
		t.Fatalf("missing px kind = %s, ok %v; want %s", got, ok, KindF64)
	}
}

func TestKeyedFrameTopLevelKeyValueHelpersAndLatestFrame(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("venue", []any{Symbol("XNAS"), Symbol("XNYS"), Symbol("XNYS"), Symbol("XNAS")}),
		NewColumn("seq", []any{1, 2, 3, 4}),
		NewColumn("px", []any{100.0, 101.0, 80.0, 102.0}),
	)

	keyed, err := KeyBy(frame, "sym", "venue")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	keys, values, err := KeyValueFrames(keyed)
	if err != nil {
		t.Fatalf("KeyValueFrames returned error: %v", err)
	}
	assertColumnNames(t, keys, []Symbol{"sym", "venue"})
	assertColumnValues(t, keys, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")})
	assertColumnNames(t, values, []Symbol{"seq", "px"})
	assertColumnValues(t, values, "seq", []any{int64(1), int64(2), int64(3), int64(4)})

	keyFrame, err := KeyFrame(keyed)
	if err != nil {
		t.Fatalf("KeyFrame returned error: %v", err)
	}
	valueFrame, err := ValueFrame(keyed)
	if err != nil {
		t.Fatalf("ValueFrame returned error: %v", err)
	}
	if !SameSchema(keys, keyFrame) || !SameSchema(values, valueFrame) {
		t.Fatal("top-level key/value helpers did not match method schemas")
	}

	latest, err := LatestFrame(keyed)
	if err != nil {
		t.Fatalf("LatestFrame returned error: %v", err)
	}
	assertColumnNames(t, latest, []Symbol{"sym", "venue", "seq", "px"})
	assertColumnValues(t, latest, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, latest, "venue", []any{Symbol("XNAS"), Symbol("XNYS"), Symbol("XNYS")})
	assertColumnValues(t, latest, "seq", []any{int64(4), int64(2), int64(3)})
	assertColumnValues(t, latest, "px", []any{102.0, 101.0, 80.0})
}

func TestKeyedFrameKeyValueFramesAndRecordLookupPreserveKeyOrder(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("venue", []any{Symbol("XNYS"), Symbol("XNYS"), Symbol("XNAS"), Symbol("XNAS")}),
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("trade_id", []any{1, 2, 3, 4}),
		NewColumn("price", []any{100.0, 100.5, 101.0, 80.0}),
	)

	keyed, err := KeyBy(frame, "sym", "venue")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	keyFrame, err := keyed.KeyFrame()
	if err != nil {
		t.Fatalf("KeyFrame returned error: %v", err)
	}
	assertColumnNames(t, keyFrame, []Symbol{"sym", "venue"})
	assertColumnValues(t, keyFrame, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, keyFrame, "venue", []any{Symbol("XNYS"), Symbol("XNYS"), Symbol("XNAS"), Symbol("XNAS")})

	valueFrame, err := keyed.ValueFrame()
	if err != nil {
		t.Fatalf("ValueFrame returned error: %v", err)
	}
	assertColumnNames(t, valueFrame, []Symbol{"trade_id", "price"})
	if got, ok := valueFrame.Schema().Kind("price"); !ok || got != KindF64 {
		t.Fatalf("value frame price kind = %s, ok %v; want %s", got, ok, KindF64)
	}

	got, err := keyed.LookupByKeyRecord(map[Symbol]any{
		"venue": Symbol("XNYS"),
		"sym":   "AAPL",
		"extra": int64(99),
	})
	if err != nil {
		t.Fatalf("LookupByKeyRecord returned error: %v", err)
	}
	assertColumnValues(t, got, "trade_id", []any{int64(1), int64(2)})

	valueHit, err := keyed.LookupValueByKeyRecord(map[Symbol]any{"venue": "XNAS", "sym": "AAPL"})
	if err != nil {
		t.Fatalf("LookupValueByKeyRecord returned error: %v", err)
	}
	assertColumnNames(t, valueHit, []Symbol{"trade_id", "price"})
	assertColumnValues(t, valueHit, "trade_id", []any{int64(3)})

	if _, err := keyed.LookupByKeyRecord(map[Symbol]any{"sym": "AAPL"}); err == nil {
		t.Fatal("LookupByKeyRecord accepted a missing key field")
	}
}

func TestKeyedFrameLookupCoercesToKeyColumnKinds(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "id", Data: NewI32([]int32{1, 2})},
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
		NewColumn("qty", []any{10, 20}),
	)

	keyed, err := KeyBy(frame, "id", "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	got, err := keyed.LookupByKey(2, "MSFT")
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, got, "id", []any{int32(2)})
	assertColumnValues(t, got, "sym", []any{Symbol("MSFT")})
	assertColumnValues(t, got, "qty", []any{int64(20)})

	tsFrame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		Column{Name: "bucket", Data: NewTimestamp([]Timestamp{
			TimestampFromUnixNanos(1_782_810_000_000_000_000),
			TimestampFromUnixNanos(1_782_810_060_000_000_000),
		})},
		NewColumn("size", []any{100, 200}),
	)
	tsKeyed, err := KeyBy(tsFrame, "sym", "bucket")
	if err != nil {
		t.Fatalf("timestamp KeyBy returned error: %v", err)
	}
	tsGot, err := tsKeyed.LookupByKey("MSFT", "2026-06-30T09:01:00Z")
	if err != nil {
		t.Fatalf("timestamp LookupByKey returned error: %v", err)
	}
	assertColumnValues(t, tsGot, "sym", []any{Symbol("MSFT")})
	assertColumnValues(t, tsGot, "bucket", []any{TimestampFromUnixNanos(1_782_810_060_000_000_000)})
	assertColumnValues(t, tsGot, "size", []any{int64(200)})
}

func TestKeyedFrameRejectsInvalidInputs(t *testing.T) {
	frame := mustFrame(t, NewColumn("id", []any{1}))

	if _, err := KeyBy(frame); err == nil {
		t.Fatal("KeyBy accepted no keys")
	}
	if _, err := KeyBy(frame, ""); err == nil {
		t.Fatal("KeyBy accepted empty key")
	}
	if _, err := KeyBy(frame, "missing"); err == nil {
		t.Fatal("KeyBy accepted missing key")
	}
	if _, err := KeyBy(frame, "id", "id"); err == nil {
		t.Fatal("KeyBy accepted duplicate key")
	}

	keyed, err := KeyBy(frame, "id")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	if _, err := keyed.LookupByKey(); err == nil {
		t.Fatal("LookupByKey accepted too few values")
	}
	if _, err := keyed.LookupByKey(1, 2); err == nil {
		t.Fatal("LookupByKey accepted too many values")
	}
	if _, err := keyed.LookupByKey("bad"); err == nil {
		t.Fatal("LookupByKey accepted incompatible key value")
	}
}

func TestKeyedFrameUpsertUpdatesMatchesAndAppendsMisses(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "id", Data: NewI32([]int32{1, 2})},
		NewColumn("qty", []any{10, 20}),
		NewColumn("note", []any{"old-1", "old-2"}),
	)
	keyed, err := KeyBy(frame, "id")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	delta := mustFrame(t,
		NewColumn("id", []any{2, 3}),
		NewColumn("qty", []any{25, 30}),
		NewColumn("note", []any{"new-2", "new-3"}),
	)

	got, err := keyed.Upsert(delta)
	if err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	out := got.Frame()
	assertColumnNames(t, out, []Symbol{"id", "qty", "note"})
	assertColumnValues(t, out, "id", []any{int32(1), int32(2), int32(3)})
	assertColumnValues(t, out, "qty", []any{int64(10), int64(25), int64(30)})
	assertColumnValues(t, out, "note", []any{"old-1", "new-2", "new-3"})
	if gotKind, ok := out.Schema().Kind("id"); !ok || gotKind != KindI32 {
		t.Fatalf("id kind = %s, ok %v; want %s", gotKind, ok, KindI32)
	}

	hit, err := got.LookupValueByKey(3)
	if err != nil {
		t.Fatalf("LookupValueByKey returned error: %v", err)
	}
	assertColumnValues(t, hit, "qty", []any{int64(30)})
}

func TestKeyedFrameIndexMetadataRebuiltAfterMutation(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
		NewColumn("qty", []any{10, 20}),
	)
	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	if err := keyed.ValidateIndex(); err != nil {
		t.Fatalf("initial keyed index invalid: %v", err)
	}
	before := keyed.IndexMetadata()
	if before.Rows != 2 || before.SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("initial metadata = %#v, want rows=2 schema=%s", before, frame.SchemaFingerprint())
	}

	upserted, err := keyed.Upsert(mustFrame(t,
		NewColumn("sym", []any{"MSFT", "TSLA"}),
		NewColumn("qty", []any{25, 30}),
	))
	if err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if err := upserted.ValidateIndex(); err != nil {
		t.Fatalf("upserted keyed index invalid: %v", err)
	}
	after := upserted.IndexMetadata()
	if after.Rows != 3 {
		t.Fatalf("upserted metadata rows = %d, want 3", after.Rows)
	}
	if after.Fingerprint == before.Fingerprint {
		t.Fatalf("upserted metadata fingerprint did not change: %#v", after)
	}
	tsla, err := upserted.LookupValueByKey("TSLA")
	if err != nil {
		t.Fatalf("upserted LookupValueByKey returned error: %v", err)
	}
	assertColumnValues(t, tsla, "qty", []any{int64(30)})

	oldTSLA, err := keyed.LookupValueByKey("TSLA")
	if err != nil {
		t.Fatalf("old keyed LookupValueByKey returned error: %v", err)
	}
	if oldTSLA.Len() != 0 {
		t.Fatalf("old keyed index saw new TSLA row; len = %d, want 0", oldTSLA.Len())
	}
}

func TestKeyedFrameValidateIndexRejectsStaleSameShapeKeyValues(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
		NewColumn("qty", []any{10, 20}),
	)
	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	stale := keyed
	stale.frame = mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "TSLA"})},
		NewColumn("qty", []any{10, 20}),
	)
	if err := stale.ValidateIndex(); err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("ValidateIndex stale error = %v, want fingerprint mismatch", err)
	}

	stale = keyed
	stale.frame = mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "TSLA"})},
		NewColumn("qty", []any{10, 20, 30}),
	)
	if _, err := stale.LookupValueByKey("AAPL"); err == nil || !strings.Contains(err.Error(), "rows mismatch") {
		t.Fatalf("LookupValueByKey stale shape error = %v, want rows mismatch", err)
	}
}

func TestKeyedFrameAmendOnlyUpdatesExistingKeys(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("qty", []any{10, 20}),
	)
	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	delta := mustFrame(t,
		NewColumn("sym", []any{"AAPL", "TSLA"}),
		NewColumn("qty", []any{15, 30}),
	)

	got, err := keyed.Amend(delta)
	if err != nil {
		t.Fatalf("Amend returned error: %v", err)
	}
	out := got.Frame()
	assertColumnValues(t, out, "sym", []any{Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, out, "qty", []any{int64(15), int64(20)})
}

func TestKeyedFrameMutationEmptyDeltaAndSelectedColumns(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("qty", []any{10, 20}),
		NewColumn("note", []any{"old-a", "old-m"}),
	)
	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	emptyDelta := mustFrame(t,
		NewColumn("sym", []any{}),
		NewColumn("qty", []any{}),
		NewColumn("note", []any{}),
	)
	amended, err := keyed.Amend(emptyDelta)
	if err != nil {
		t.Fatalf("Amend empty delta returned error: %v", err)
	}
	assertColumnValues(t, amended.Frame(), "qty", []any{int64(10), int64(20)})
	assertColumnValues(t, amended.Frame(), "note", []any{"old-a", "old-m"})

	upserted, err := keyed.Upsert(mustFrame(t,
		NewColumn("sym", []any{"AAPL", "TSLA"}),
		NewColumn("qty", []any{11, 30}),
		NewColumn("note", []any{"ignored-a", "ignored-t"}),
	), "qty")
	if err != nil {
		t.Fatalf("Upsert selected value columns returned error: %v", err)
	}
	out := upserted.Frame()
	assertColumnNames(t, out, []Symbol{"sym", "qty", "note"})
	assertColumnValues(t, out, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})
	assertColumnValues(t, out, "qty", []any{int64(11), int64(20), int64(30)})
	assertColumnValues(t, out, "note", []any{"old-a", "old-m", NullValue})

	noValues, err := keyed.Amend(mustFrame(t, NewColumn("sym", []any{"AAPL"})))
	if err != nil {
		t.Fatalf("Amend with no value columns returned error: %v", err)
	}
	assertColumnValues(t, noValues.Frame(), "qty", []any{int64(10), int64(20)})
	assertColumnValues(t, noValues.Frame(), "note", []any{"old-a", "old-m"})
	if _, err := keyed.Upsert(mustFrame(t,
		NewColumn("sym", []any{"AAPL"}),
		NewColumn("qty", []any{11}),
	), "qty", "qty"); err == nil {
		t.Fatal("Upsert accepted duplicate selected value column")
	}
}

func TestKeyedFrameMutationRejectsKeyColumnChangeAndAddsValueColumns(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("qty", []any{10, 20}),
	)
	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}

	if _, err := keyed.Upsert(mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL")}),
		NewColumn("qty", []any{11}),
	), "sym"); err == nil {
		t.Fatal("Upsert accepted key assignment")
	}

	got, err := keyed.Upsert(mustFrame(t,
		NewColumn("sym", []any{"AAPL", "TSLA"}),
		NewColumn("venue", []any{"XNAS", "XNYS"}),
	))
	if err != nil {
		t.Fatalf("Upsert with new value column returned error: %v", err)
	}
	out := got.Frame()
	assertColumnNames(t, out, []Symbol{"sym", "qty", "venue"})
	assertColumnValues(t, out, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})
	assertColumnValues(t, out, "qty", []any{int64(10), int64(20), NullValue})
	assertColumnValues(t, out, "venue", []any{"XNAS", NullValue, "XNYS"})
	if gotKind, ok := out.Schema().Kind("venue"); !ok || gotKind != KindString {
		t.Fatalf("venue kind = %s, ok %v; want %s", gotKind, ok, KindString)
	}
}

func TestInsertRowAndKeyedUpsertRow(t *testing.T) {
	frame := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT"})},
		Column{Name: "qty", Data: NewI64([]int64{10, 20})},
		Column{Name: "note", Data: NewString([]string{"old-a", "old-m"})},
	)
	inserted, err := InsertRow(frame, []Symbol{"sym", "qty"}, []any{Symbol("TSLA"), int64(30)})
	if err != nil {
		t.Fatalf("InsertRow returned error: %v", err)
	}
	assertColumnValues(t, inserted, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})
	assertColumnValues(t, inserted, "qty", []any{int64(10), int64(20), int64(30)})
	assertColumnValues(t, inserted, "note", []any{"old-a", "old-m", NullValue})

	keyed, err := KeyBy(frame, "sym")
	if err != nil {
		t.Fatalf("KeyBy returned error: %v", err)
	}
	if _, err := keyed.InsertRow([]Symbol{"sym", "qty"}, []any{Symbol("AAPL"), int64(15)}); err == nil {
		t.Fatalf("keyed InsertRow duplicate returned nil error")
	}
	if _, err := keyed.InsertRow([]Symbol{"qty"}, []any{int64(15)}); err == nil {
		t.Fatalf("keyed InsertRow missing key returned nil error")
	}
	if _, err := keyed.UpsertRow([]Symbol{"qty"}, []any{int64(15)}); err == nil {
		t.Fatalf("keyed UpsertRow missing key returned nil error")
	}
	upserted, err := keyed.UpsertRow([]Symbol{"sym", "qty"}, []any{Symbol("AAPL"), int64(15)})
	if err != nil {
		t.Fatalf("keyed UpsertRow existing returned error: %v", err)
	}
	out := upserted.Frame()
	assertColumnValues(t, out, "qty", []any{int64(15), int64(20)})
	assertColumnValues(t, out, "note", []any{"old-a", "old-m"})

	upserted, err = upserted.UpsertRow([]Symbol{"sym", "qty"}, []any{Symbol("TSLA"), int64(40)})
	if err != nil {
		t.Fatalf("keyed UpsertRow missing returned error: %v", err)
	}
	out = upserted.Frame()
	assertColumnValues(t, out, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})
	assertColumnValues(t, out, "qty", []any{int64(15), int64(20), int64(40)})
	assertColumnValues(t, out, "note", []any{"old-a", "old-m", NullValue})
}

func TestInnerJoinOnSameNameKeysPreservesOrderAndNamesConflicts(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a"), Symbol("c")}),
		NewColumn("qty", []any{10, 20, 30, 40}),
		NewColumn("venue_right", []any{"left-a", "left-b", "left-c", "left-d"}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("d")}),
		NewColumn("qty", []any{1, 2, 3}),
		NewColumn("venue_right", []any{"rx", "ry", "rz"}),
	)

	got, err := InnerJoin(left, right, "sym")
	if err != nil {
		t.Fatalf("InnerJoin returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "qty", "venue_right", "qty_right", "venue_right_right"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("a")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(10), int64(30), int64(30)})
	assertColumnValues(t, got, "venue_right", []any{"left-a", "left-a", "left-c", "left-c"})
	assertColumnValues(t, got, "qty_right", []any{int64(1), int64(2), int64(1), int64(2)})
	assertColumnValues(t, got, "venue_right_right", []any{"rx", "ry", "rx", "ry"})
}

func TestInnerJoinOnSpecifiedKeyColumns(t *testing.T) {
	left := mustFrame(t,
		NewColumn("id", []any{1, 2, 3}),
		NewColumn("left_value", []any{"one", "two", "three"}),
	)
	right := mustFrame(t,
		NewColumn("account_id", []any{3, 1, 1}),
		NewColumn("right_value", []any{"tres", "uno", "one-again"}),
	)

	got, err := InnerJoinOn(left, right, JoinKey{Left: "id", Right: "account_id"})
	if err != nil {
		t.Fatalf("InnerJoinOn returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"id", "left_value", "right_value"})
	assertColumnValues(t, got, "id", []any{int64(1), int64(1), int64(3)})
	assertColumnValues(t, got, "left_value", []any{"one", "one", "three"})
	assertColumnValues(t, got, "right_value", []any{"uno", "one-again", "tres"})
	if _, ok := got.Column("account_id"); ok {
		t.Fatal("InnerJoinOn included duplicate right key column")
	}
}

func TestInnerJoinOnWithOptionsPrunesOutputColumns(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("price", []any{100.0, 80.0, 101.0}),
		NewColumn("size", []any{10, 20, 30}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("bid", []any{99.5, 79.5}),
		NewColumn("ask", []any{100.5, 80.5}),
		NewColumn("venue", []any{Symbol("XNAS"), Symbol("XNYS")}),
	)

	got, err := InnerJoinOnWithOptions(left, right, JoinOptions{
		LeftColumns:  []Symbol{"sym", "price"},
		RightColumns: []Symbol{"sym", "bid", "ask"},
	}, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("InnerJoinOnWithOptions returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "price", "bid", "ask"})
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")})
	assertColumnValues(t, got, "price", []any{100.0, 80.0, 101.0})
	assertColumnValues(t, got, "bid", []any{99.5, 79.5, 99.5})
	if _, ok := got.Column("size"); ok {
		t.Fatal("pruned join output included unrequested left column size")
	}
	if _, ok := got.Column("venue"); ok {
		t.Fatal("pruned join output included unrequested right column venue")
	}
}

func TestInnerJoinOnWithOptionsPreservesRightCollisionNames(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL")}),
		NewColumn("bid", []any{100.0}),
		NewColumn("bid_right", []any{101.0}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL")}),
		NewColumn("bid", []any{99.5}),
		NewColumn("ask", []any{100.5}),
	)

	got, err := InnerJoinOnWithOptions(left, right, JoinOptions{
		LeftColumns:  []Symbol{"sym"},
		RightColumns: []Symbol{"bid", "ask"},
	}, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("InnerJoinOnWithOptions returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "bid_right2", "ask"})
	assertColumnValues(t, got, "bid_right2", []any{99.5})
}

func TestInnerJoinOnWithOptionsOrderLimitLeftColumnDesc(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL"), Symbol("TSLA")}),
		NewColumn("price", []any{100.0, 80.0, 101.0, 120.0}),
		NewColumn("seq", []any{1, 2, 3, 4}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("bid", []any{99.5, 79.5, 99.7}),
	)

	got, err := InnerJoinOnWithOptions(left, right, JoinOptions{
		LeftColumns:  []Symbol{"sym", "price"},
		RightColumns: []Symbol{"bid"},
		OrderBy:      []JoinOrderSpec{{Column: "price", Desc: true}},
		LimitN:       2,
	}, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("InnerJoinOnWithOptions returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("AAPL")})
	assertColumnValues(t, got, "price", []any{101.0, 101.0})
	assertColumnValues(t, got, "bid", []any{99.5, 99.7})
}

func TestInnerJoinOnWithOptionsOrderLimitRightColumnDesc(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")}),
		NewColumn("price", []any{100.0, 80.0, 120.0}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("bid", []any{99.5, 79.5, 100.2}),
		NewColumn("quote_seq", []any{1, 2, 3}),
	)

	got, err := InnerJoinOnWithOptions(left, right, JoinOptions{
		LeftColumns:  []Symbol{"sym", "price"},
		RightColumns: []Symbol{"bid", "quote_seq"},
		OrderBy:      []JoinOrderSpec{{Column: "bid", Right: true, Desc: true}},
		LimitN:       2,
	}, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("InnerJoinOnWithOptions returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("AAPL")})
	assertColumnValues(t, got, "bid", []any{100.2, 99.5})
	assertColumnValues(t, got, "quote_seq", []any{int64(3), int64(1)})
}

func TestInnerJoinOnWithOptionsOrderLimitDuplicateRightRowsStable(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL")}),
		NewColumn("price", []any{100.0}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL")}),
		NewColumn("bid", []any{99.5, 99.5}),
		NewColumn("quote_seq", []any{1, 2}),
	)

	got, err := InnerJoinOnWithOptions(left, right, JoinOptions{
		LeftColumns:  []Symbol{"sym", "price"},
		RightColumns: []Symbol{"bid", "quote_seq"},
		OrderBy:      []JoinOrderSpec{{Column: "bid", Right: true, Desc: true}},
		LimitN:       2,
	}, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("InnerJoinOnWithOptions returned error: %v", err)
	}

	assertColumnValues(t, got, "quote_seq", []any{int64(1), int64(2)})
}

func TestLeftJoinOnWithOptionsOrderLimitKeepsUnmatchedRows(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("TSLA"), Symbol("MSFT")}),
		NewColumn("price", []any{100.0, 120.0, 80.0}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("bid", []any{99.5, 79.5}),
	)

	got, err := LeftJoinOnWithOptions(left, right, JoinOptions{
		LeftColumns:  []Symbol{"sym", "price"},
		RightColumns: []Symbol{"bid"},
		OrderBy:      []JoinOrderSpec{{Column: "price", Desc: true}},
		LimitN:       2,
	}, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("LeftJoinOnWithOptions returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("TSLA"), Symbol("AAPL")})
	assertColumnValues(t, got, "price", []any{120.0, 100.0})
	assertColumnValues(t, got, "bid", []any{NullValue, 99.5})
}

func TestJoinAsofWindowReuseIndexedRightAttributes(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("ts", []any{int64(10), int64(11), int64(12)}),
		NewColumn("qty", []any{100, 200, 300}),
	)
	rightSym := &countingMetadataArray{
		array: WithArrayAttribute(NewSymbols([]string{"AAPL", "AAPL", "MSFT"}), ArrayAttributeGrouped),
	}
	right := mustFrame(t,
		Column{Name: "sym", Data: rightSym},
		Column{Name: "ts", Data: WithArrayAttribute(NewI64([]int64{9, 12, 10}), ArrayAttributeSorted)},
		NewColumn("quote", []any{"a9", "a12", "m10"}),
	)

	joined, err := InnerJoinOn(left, right, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("InnerJoinOn returned error: %v", err)
	}
	if joined.Len() != 5 {
		t.Fatalf("joined len = %d, want 5", joined.Len())
	}
	if rightSym.ats != 0 {
		t.Fatalf("inner join right key At called %d times; want attribute index path", rightSym.ats)
	}

	asofed, err := AsofJoinOn(left, right, JoinKey{Left: "ts", Right: "ts"}, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("AsofJoinOn returned error: %v", err)
	}
	assertColumnValues(t, asofed, "quote", []any{"a9", "m10", "a12"})
	if rightSym.ats != 0 {
		t.Fatalf("asof join right partition At called %d times; want grouped attribute path", rightSym.ats)
	}

	windowed, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           int64(-2),
		High:          int64(0),
		HasBounds:     true,
		Last:          true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions returned error: %v", err)
	}
	assertColumnValues(t, windowed, "quote", []any{"a9", "m10", "a12"})
	if rightSym.ats != 0 {
		t.Fatalf("window join right partition At called %d times; want grouped attribute path", rightSym.ats)
	}
}

func TestInnerJoinOnMultipleKeysAndEmptyResultKeepsSchema(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("venue", []any{"x", "y", "x"}),
		NewColumn("qty", []any{10, 20, 30}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("venue", []any{"z", "x", "y"}),
		NewColumn("price", []any{100, 200, 300}),
	)

	got, err := InnerJoin(left, right, "sym", "venue")
	if err != nil {
		t.Fatalf("InnerJoin returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "venue", "qty", "price"})
	assertColumnValues(t, got, "sym", []any{Symbol("a")})
	assertColumnValues(t, got, "venue", []any{"x"})
	assertColumnValues(t, got, "qty", []any{int64(10)})
	assertColumnValues(t, got, "price", []any{int64(200)})

	empty, err := InnerJoinOn(left, right,
		JoinKey{Left: "sym", Right: "venue"},
		JoinKey{Left: "venue", Right: "sym"},
	)
	if err != nil {
		t.Fatalf("empty InnerJoin returned error: %v", err)
	}
	assertColumnNames(t, empty, []Symbol{"sym", "venue", "qty", "price"})
	if got := empty.Len(); got != 0 {
		t.Fatalf("empty join length = %d, want 0", got)
	}
	if got, ok := empty.Schema().Kind("qty"); !ok || got != KindI64 {
		t.Fatalf("empty join qty kind = %s, ok %v; want %s", got, ok, KindI64)
	}
}

func TestInnerJoinRejectsInvalidKeys(t *testing.T) {
	left := mustFrame(t, NewColumn("id", []any{1}))
	right := mustFrame(t, NewColumn("id", []any{1}))

	if _, err := InnerJoin(left, right); err == nil {
		t.Fatal("InnerJoin accepted no keys")
	}
	if _, err := InnerJoin(left, right, "missing"); err == nil {
		t.Fatal("InnerJoin accepted missing key")
	}
	if _, err := InnerJoinOn(left, right, JoinKey{Left: "id", Right: ""}); err == nil {
		t.Fatal("InnerJoinOn accepted empty key")
	}
	if _, err := InnerJoinOn(left, right, JoinKey{Left: "id", Right: "id"}, JoinKey{Left: "id", Right: "id"}); err == nil {
		t.Fatal("InnerJoinOn accepted duplicate key pair")
	}
}

func TestJoinSingleColumnTypedPathDoesNotRequireGroupedIndex(t *testing.T) {
	left := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "MSFT", "TSLA"})},
		Column{Name: "qty", Data: NewI64([]int64{10, 20, 30})},
	)
	right := mustFrame(t,
		Column{Name: "sym", Data: NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		Column{Name: "bid", Data: NewF64([]float64{100.0, 101.0, 80.0})},
	)
	if sym, _ := right.Column("sym"); ArrayHasAttribute(sym, ArrayAttributeGrouped) {
		t.Fatal("right sym unexpectedly starts with grouped attribute")
	}

	got, err := InnerJoin(left, right, "sym")
	if err != nil {
		t.Fatalf("InnerJoin returned error: %v", err)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(10), int64(20)})
	assertColumnValues(t, got, "bid", []any{100.0, 101.0, 80.0})

	sym, _ := right.Column("sym")
	if _, ok := ArrayIndexFor(sym, ArrayAttributeGrouped); ok {
		t.Fatal("typed single-column join built a grouped string index; want direct typed probe")
	}
}

func TestJoinOutputReusesTypedRangeIndexes(t *testing.T) {
	left := mustFrame(t,
		Column{Name: "id", Data: NewI64([]int64{10, 11, 12, 13})},
		Column{Name: "qty", Data: NewI64Range(100, 1, 4)},
	)
	right := mustFrame(t,
		Column{Name: "id", Data: NewI64([]int64{10, 11, 12, 13})},
		Column{Name: "px", Data: NewI64Range(200, 2, 4)},
	)

	got, err := InnerJoin(left, right, "id")
	if err != nil {
		t.Fatalf("InnerJoin returned error: %v", err)
	}
	assertColumnValues(t, got, "qty", []any{int64(100), int64(101), int64(102), int64(103)})
	assertColumnValues(t, got, "px", []any{int64(200), int64(202), int64(204), int64(206)})
	if col := mustColumn(t, got, "qty"); !isI64RangeArray(col) {
		t.Fatalf("joined qty column = %T, want lazy i64 range", col)
	}
	if col := mustColumn(t, got, "px"); !isI64RangeArray(col) {
		t.Fatalf("joined px column = %T, want lazy i64 range", col)
	}
}

func TestAsofJoinOutputReusesTypedRangeIndexes(t *testing.T) {
	left := mustFrame(t,
		Column{Name: "ts", Data: NewI64([]int64{1, 2, 3, 4})},
		Column{Name: "qty", Data: NewI64Range(10, 10, 4)},
	)
	right := mustFrame(t,
		Column{Name: "ts", Data: NewI64([]int64{1, 2, 3, 4})},
		Column{Name: "bid", Data: NewI64Range(100, 5, 4)},
	)

	got, err := AsofJoin(left, right, "ts")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}
	assertColumnValues(t, got, "bid", []any{int64(100), int64(105), int64(110), int64(115)})
	if col := mustColumn(t, got, "bid"); !isI64RangeArray(col) {
		t.Fatalf("asof bid column = %T, want lazy i64 range", col)
	}
}

func isI64RangeArray(array Array) bool {
	switch a := array.(type) {
	case i64RangeArray:
		return true
	case attributedArray:
		return isI64RangeArray(a.array)
	default:
		return false
	}
}

func TestKeyedJoinUsesRightLatestValueRows(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")}),
		NewColumn("qty", []any{10, 20, 30}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("lot", []any{1, 2, 3}),
		NewColumn("px", []any{100.0, 101.0, 80.0}),
	)
	keyedRight, err := KeyBy(right, "sym")
	if err != nil {
		t.Fatalf("KeyBy right returned error: %v", err)
	}

	got, err := LeftJoinKeyed(left, keyedRight)
	if err != nil {
		t.Fatalf("LeftJoinKeyed returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "qty", "lot", "px"})
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})
	assertColumnValues(t, got, "lot", []any{int64(2), int64(3), NullValue})
	assertColumnValues(t, got, "px", []any{101.0, 80.0, NullValue})

	inner, err := InnerJoinKeyed(left, keyedRight)
	if err != nil {
		t.Fatalf("InnerJoinKeyed returned error: %v", err)
	}
	assertColumnValues(t, inner, "sym", []any{Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, inner, "lot", []any{int64(2), int64(3)})
}

func TestLeftJoinOnDuplicateRightKeysExpandsInRightOrder(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")}),
		NewColumn("qty", []any{10, 20, 30}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("seq", []any{1, 2, 3}),
		NewColumn("bid", []any{99.0, 100.0, 79.0}),
	)

	got, err := LeftJoin(left, right, "sym")
	if err != nil {
		t.Fatalf("LeftJoin returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "qty", "seq", "bid"})
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(10), int64(20), int64(30)})
	assertColumnValues(t, got, "seq", []any{int64(1), int64(2), int64(3), NullValue})
	assertColumnValues(t, got, "bid", []any{99.0, 100.0, 79.0, NullValue})
}

func TestLeftJoinUsesTypedSingleColumnKeyKernel(t *testing.T) {
	left := mustFrame(t,
		Column{Name: "id", Data: NewU32([]uint32{7, 9, 11})},
		NewColumn("qty", []any{10, 20, 30}),
	)
	right := mustFrame(t,
		Column{Name: "id", Data: NewU32([]uint32{7, 7, 9})},
		NewColumn("tag", []any{"a", "b", "c"}),
	)

	got, err := LeftJoin(left, right, "id")
	if err != nil {
		t.Fatalf("LeftJoin returned error: %v", err)
	}

	assertColumnValues(t, got, "id", []any{uint32(7), uint32(7), uint32(9), uint32(11)})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(10), int64(20), int64(30)})
	assertColumnValues(t, got, "tag", []any{"a", "b", "c", NullValue})
}

func TestLeftJoinUsesRightSingleColumnAttributeIndex(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")}),
		NewColumn("qty", []any{10, 20, 30}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("seq", []any{1, 2, 3}),
	)
	sym := &countingMetadataArray{
		array: WithArrayAttribute(NewSymbols([]string{"AAPL", "AAPL", "MSFT"}), ArrayAttributeGrouped),
	}
	right.columns["sym"] = sym

	got, err := LeftJoin(left, right, "sym")
	if err != nil {
		t.Fatalf("LeftJoin returned error: %v", err)
	}
	if sym.ats != 0 {
		t.Fatalf("right join key At called %d times; want attribute index path", sym.ats)
	}
	assertColumnValues(t, got, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")})
	assertColumnValues(t, got, "seq", []any{int64(1), int64(2), int64(3), NullValue})
}

func TestKeyedJoinOnSupportsLeftKeyAliasesAndRejectsNonKeyRightColumns(t *testing.T) {
	left := mustFrame(t,
		NewColumn("ticker", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("qty", []any{10, 20}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("venue", []any{Symbol("XNAS"), Symbol("XNYS")}),
		NewColumn("px", []any{100.0, 80.0}),
	)
	keyedRight, err := KeyBy(right, "sym")
	if err != nil {
		t.Fatalf("KeyBy right returned error: %v", err)
	}

	got, err := LeftJoinKeyedOn(left, keyedRight, JoinKey{Left: "ticker", Right: "sym"})
	if err != nil {
		t.Fatalf("LeftJoinKeyedOn returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"ticker", "qty", "venue", "px"})
	assertColumnValues(t, got, "venue", []any{Symbol("XNAS"), Symbol("XNYS")})

	if _, err := LeftJoinKeyedOn(left, keyedRight, JoinKey{Left: "ticker", Right: "venue"}); err == nil {
		t.Fatal("LeftJoinKeyedOn accepted a non-key right column")
	}
}

func TestUnionJoinOnKeepsMatchedLeftAndUnmatchedRightRows(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b")}),
		NewColumn("qty", []any{10, 20}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("c")}),
		NewColumn("price", []any{100, 300}),
	)

	got, err := UnionJoinOn(left, right, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("UnionJoinOn returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "qty", "price"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(20), NullValue})
	assertColumnValues(t, got, "price", []any{int64(100), NullValue, int64(300)})
}

func TestUnionJoinOnDuplicateRightKeysAndUnmatchedRightRows(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b")}),
		NewColumn("qty", []any{10, 20}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("c")}),
		NewColumn("price", []any{100, 101, 300}),
	)

	got, err := UnionJoinOn(left, right, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("UnionJoinOn returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), Symbol("c")})
	assertColumnValues(t, got, "qty", []any{int64(10), int64(10), int64(20), NullValue})
	assertColumnValues(t, got, "price", []any{int64(100), int64(101), NullValue, int64(300)})
}

func TestPlusJoinOnAddsMatchedSameNameColumns(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("c")}),
		NewColumn("qty", []any{10, 20, 30}),
		NewColumn("px", []any{1.5, 2.5, 3.5}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b")}),
		NewColumn("qty", []any{1, 2}),
		NewColumn("venue", []any{"x", "y"}),
	)

	got, err := PlusJoinOn(left, right, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("PlusJoinOn returned error: %v", err)
	}
	assertColumnNames(t, got, []Symbol{"sym", "qty", "px", "venue"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b"), Symbol("c")})
	assertColumnValues(t, got, "qty", []any{float64(11), float64(22), float64(30)})
	assertColumnValues(t, got, "px", []any{1.5, 2.5, 3.5})
	assertColumnValues(t, got, "venue", []any{"x", "y", NullValue})
}

func TestPlusJoinOnDuplicateRightKeysUsesFirstMatch(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b")}),
		NewColumn("qty", []any{10, 20}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("qty", []any{1, 100, 2}),
		NewColumn("venue", []any{"first", "second", "only"}),
	)

	got, err := PlusJoinOn(left, right, JoinKey{Left: "sym", Right: "sym"})
	if err != nil {
		t.Fatalf("PlusJoinOn returned error: %v", err)
	}

	assertColumnValues(t, got, "qty", []any{float64(11), float64(22)})
	assertColumnValues(t, got, "venue", []any{"first", "only"})
}

func TestAsofJoinOnPartitionsAndLatestRightTime(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), Symbol("a"), Symbol("c")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(10),
			TimestampFromUnixNanos(15),
			TimestampFromUnixNanos(12),
			TimestampFromUnixNanos(4),
			TimestampFromUnixNanos(20),
		}),
		NewColumn("qty", []any{100, 150, 120, 40, 200}),
		NewColumn("price_right", []any{"left-a", "left-b", "left-c", "left-d", "left-e"}),
	)
	right := mustFrame(t,
		NewColumn("ticker", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("time", []any{
			TimestampFromUnixNanos(5),
			TimestampFromUnixNanos(11),
			TimestampFromUnixNanos(10),
			TimestampFromUnixNanos(13),
		}),
		NewColumn("price", []any{50, 110, 100, 130}),
		NewColumn("price_right", []any{"ra5", "ra11", "ra10", "rb13"}),
	)

	got, err := AsofJoinOn(left, right,
		JoinKey{Left: "ts", Right: "time"},
		JoinKey{Left: "sym", Right: "ticker"},
	)
	if err != nil {
		t.Fatalf("AsofJoinOn returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "ts", "qty", "price_right", "price", "price_right_right"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), Symbol("a"), Symbol("c")})
	assertColumnValues(t, got, "ts", []any{
		Timestamp(10),
		Timestamp(15),
		Timestamp(12),
		Timestamp(4),
		Timestamp(20),
	})
	assertColumnValues(t, got, "price", []any{int64(100), int64(110), NullValue, NullValue, NullValue})
	assertColumnValues(t, got, "price_right_right", []any{"ra10", "ra11", NullValue, NullValue, NullValue})
	if gotKind, ok := got.Schema().Kind("price"); !ok || gotKind != KindI64 {
		t.Fatalf("price kind = %s, ok %v; want %s", gotKind, ok, KindI64)
	}
}

func TestWindowJoinOnPartitionsAndCollectsPriorRightRows(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(10),
			TimestampFromUnixNanos(15),
			TimestampFromUnixNanos(12),
			TimestampFromUnixNanos(4),
		}),
		NewColumn("qty", []any{100, 150, 120, 40}),
	)
	right := mustFrame(t,
		NewColumn("ticker", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("time", []any{
			TimestampFromUnixNanos(5),
			TimestampFromUnixNanos(11),
			TimestampFromUnixNanos(10),
			TimestampFromUnixNanos(13),
		}),
		NewColumn("bid", []any{50, 110, 100, 130}),
	)

	got, err := WindowJoinOn(left, right,
		JoinKey{Left: "ts", Right: "time"},
		JoinKey{Left: "sym", Right: "ticker"},
	)
	if err != nil {
		t.Fatalf("WindowJoinOn returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "ts", "qty", "bid"})
	assertColumnValues(t, got, "bid", []any{
		[]any{int64(50), int64(100)},
		[]any{int64(50), int64(100), int64(110)},
		[]any{},
		[]any{},
	})
	if gotKind, ok := got.Schema().Kind("bid"); !ok || gotKind != KindAny {
		t.Fatalf("bid kind = %s, ok %v; want %s", gotKind, ok, KindAny)
	}
}

func TestWindowJoinOnWithBoundsAndLast(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{int64(10), int64(15)}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{int64(4), int64(8), int64(10), int64(16)}),
		NewColumn("bid", []any{40, 80, 100, 160}),
	)

	windowed, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           int64(-2),
		High:          int64(0),
		HasBounds:     true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions returned error: %v", err)
	}
	assertColumnValues(t, windowed, "bid", []any{
		[]any{int64(80), int64(100)},
		[]any{},
	})

	last, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           int64(-10),
		High:          int64(0),
		HasBounds:     true,
		Last:          true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions last returned error: %v", err)
	}
	assertColumnValues(t, last, "bid", []any{int64(100), int64(100)})
	if gotKind, ok := last.Schema().Kind("bid"); !ok || gotKind != KindI64 {
		t.Fatalf("last bid kind = %s, ok %v; want %s", gotKind, ok, KindI64)
	}
}

func TestWindowJoinOnWithBoundsPreservesStableDuplicateTimesAndNulls(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{int64(10), int64(11), int64(20)}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{int64(10), int64(10), int64(11), int64(30)}),
		NewColumn("bid", []any{100, nil, 110, 300}),
		NewColumn("seq", []any{1, 2, 3, 4}),
	)

	windowed, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           int64(0),
		High:          int64(0),
		HasBounds:     true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions returned error: %v", err)
	}
	assertColumnValues(t, windowed, "bid", []any{
		[]any{int64(100), NullValue},
		[]any{int64(110)},
		[]any{},
	})
	assertColumnValues(t, windowed, "seq", []any{
		[]any{int64(1), int64(2)},
		[]any{int64(3)},
		[]any{},
	})

	last, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           int64(0),
		High:          int64(0),
		HasBounds:     true,
		Last:          true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions last returned error: %v", err)
	}
	assertColumnValues(t, last, "bid", []any{NullValue, int64(110), NullValue})
	assertColumnValues(t, last, "seq", []any{int64(2), int64(3), NullValue})
	if gotKind, ok := last.Schema().Kind("bid"); !ok || gotKind != KindI64 {
		t.Fatalf("last bid kind = %s, ok %v; want %s", gotKind, ok, KindI64)
	}
	if gotKind, ok := last.Schema().Kind("seq"); !ok || gotKind != KindI64 {
		t.Fatalf("last seq kind = %s, ok %v; want %s", gotKind, ok, KindI64)
	}
}

func TestWindowJoinOnWithTimespanBoundsForTimestampKey(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(10 * 1_000_000_000),
			TimestampFromUnixNanos(70 * 1_000_000_000),
			NullValue,
		}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(0),
			TimestampFromUnixNanos(10 * 1_000_000_000),
			TimestampFromUnixNanos(11 * 1_000_000_000),
			TimestampFromUnixNanos(70 * 1_000_000_000),
		}),
		NewColumn("bid", []any{99.0, 100.0, 101.0, 170.0}),
	)

	windowed, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           TimespanFromNanos(-60 * 1_000_000_000),
		High:          TimespanFromNanos(0),
		HasBounds:     true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions returned error: %v", err)
	}
	assertColumnValues(t, windowed, "bid", []any{
		[]any{99.0, 100.0},
		[]any{100.0, 101.0, 170.0},
		[]any{},
	})

	last, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           TimespanFromNanos(-60 * 1_000_000_000),
		High:          TimespanFromNanos(0),
		HasBounds:     true,
		Last:          true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions last returned error: %v", err)
	}
	assertColumnValues(t, last, "bid", []any{100.0, 170.0, NullValue})
	if gotKind, ok := last.Schema().Kind("bid"); !ok || gotKind != KindF64 {
		t.Fatalf("last bid kind = %s, ok %v; want %s", gotKind, ok, KindF64)
	}
}

func TestWindowJoinMultiKeyUnsortedTemporalNullsAndAggregates(t *testing.T) {
	left := mustFrame(t,
		NewColumn("trade_id", []any{1, 2, 3, 4, 5}),
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL")}),
		NewColumn("venue", []any{Symbol("XNAS"), Symbol("XNAS"), Symbol("BATS"), Symbol("XNAS"), Symbol("XNAS")}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(30 * 1_000_000_000),
			TimestampFromUnixNanos(60 * 1_000_000_000),
			TimestampFromUnixNanos(45 * 1_000_000_000),
			TimestampFromUnixNanos(20 * 1_000_000_000),
			NullForKind(KindTimestamp),
		}),
	)
	right := mustFrame(t,
		NewColumn("ticker", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("AAPL"), Symbol("AAPL")}),
		NewColumn("market", []any{Symbol("XNAS"), Symbol("XNAS"), Symbol("XNAS"), Symbol("BATS"), Symbol("XNAS"), Symbol("XNAS"), Symbol("BATS")}),
		NewColumn("quote_time", []any{
			TimestampFromUnixNanos(20 * 1_000_000_000),
			TimestampFromUnixNanos(0),
			TimestampFromUnixNanos(70 * 1_000_000_000),
			TimestampFromUnixNanos(40 * 1_000_000_000),
			TimestampFromUnixNanos(10 * 1_000_000_000),
			NullForKind(KindTimestamp),
			TimestampFromUnixNanos(0),
		}),
		NewColumn("bid", []any{101.0, 100.0, 110.0, 200.0, 300.0, 999.0, 190.0}),
		NewColumn("bid_size", []any{20, 10, 110, 50, 30, 999, 19}),
	)

	windowed, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey: JoinKey{Left: "ts", Right: "quote_time"},
		PartitionKeys: []JoinKey{
			{Left: "sym", Right: "ticker"},
			{Left: "venue", Right: "market"},
		},
		Low:       TimespanFromNanos(-30 * 1_000_000_000),
		High:      TimespanFromNanos(0),
		HasBounds: true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions returned error: %v", err)
	}
	assertColumnValues(t, windowed, "bid", []any{
		[]any{100.0, 101.0},
		[]any{},
		[]any{200.0},
		[]any{300.0},
		[]any{},
	})

	plan := From(windowed)
	plan.Select = []SelectItem{
		{Name: "trade_id", Expr: ColumnRef{Name: "trade_id"}},
		{Name: "n", Expr: ListAggregateExpr{Func: "count", Expr: ColumnRef{Name: "bid"}}},
		{Name: "sum_size", Expr: ListAggregateExpr{Func: "sum", Expr: ColumnRef{Name: "bid_size"}}},
		{Name: "avg_bid", Expr: ListAggregateExpr{Func: "avg", Expr: ColumnRef{Name: "bid"}}},
		{Name: "last_bid", Expr: ListAggregateExpr{Func: "last", Expr: ColumnRef{Name: "bid"}}},
	}
	got, err := plan.Exec()
	if err != nil {
		t.Fatalf("window aggregate plan returned error: %v", err)
	}
	assertColumnValues(t, got, "trade_id", []any{int64(1), int64(2), int64(3), int64(4), int64(5)})
	assertColumnValues(t, got, "n", []any{int64(2), int64(0), int64(1), int64(1), int64(0)})
	assertColumnValues(t, got, "sum_size", []any{30.0, 0.0, 50.0, 30.0, 0.0})
	assertColumnValues(t, got, "last_bid", []any{101.0, NullValue, 200.0, 300.0, NullValue})
	assertColumnValues(t, got, "avg_bid", []any{100.5, NullValue, 200.0, 300.0, NullValue})
}

func TestListAggregateExprAggregatesWindowLists(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("c")}),
		NewColumn("bid", []any{
			[]any{100.0, NullValue, 101.0},
			[]any{},
			[]any{NullValue},
		}),
	)
	plan := From(frame)
	plan.Select = []SelectItem{
		{Name: "sym", Expr: ColumnRef{Name: "sym"}},
		{Name: "n", Expr: ListAggregateExpr{Func: "count", Expr: ColumnRef{Name: "bid"}}},
		{Name: "sum_bid", Expr: ListAggregateExpr{Func: "sum", Expr: ColumnRef{Name: "bid"}}},
		{Name: "avg_bid", Expr: ListAggregateExpr{Func: "avg", Expr: ColumnRef{Name: "bid"}}},
		{Name: "var_bid", Expr: ListAggregateExpr{Func: "var", Expr: ColumnRef{Name: "bid"}}},
		{Name: "dev_bid", Expr: ListAggregateExpr{Func: "dev", Expr: ColumnRef{Name: "bid"}}},
		{Name: "med_bid", Expr: ListAggregateExpr{Func: "med", Expr: ColumnRef{Name: "bid"}}},
		{Name: "min_bid", Expr: ListAggregateExpr{Func: "min", Expr: ColumnRef{Name: "bid"}}},
		{Name: "max_bid", Expr: ListAggregateExpr{Func: "max", Expr: ColumnRef{Name: "bid"}}},
		{Name: "first_bid", Expr: ListAggregateExpr{Func: "first", Expr: ColumnRef{Name: "bid"}}},
		{Name: "last_bid", Expr: ListAggregateExpr{Func: "last", Expr: ColumnRef{Name: "bid"}}},
	}
	got, err := plan.Exec()
	if err != nil {
		t.Fatalf("ListAggregateExpr query returned error: %v", err)
	}
	assertColumnValues(t, got, "n", []any{int64(3), int64(0), int64(1)})
	assertColumnValues(t, got, "sum_bid", []any{201.0, 0.0, 0.0})
	assertColumnValues(t, got, "avg_bid", []any{100.5, NullValue, NullValue})
	assertColumnValues(t, got, "var_bid", []any{0.25, NullValue, NullValue})
	assertColumnValues(t, got, "dev_bid", []any{0.5, NullValue, NullValue})
	assertColumnValues(t, got, "med_bid", []any{100.5, NullValue, NullValue})
	assertColumnValues(t, got, "min_bid", []any{100.0, NullValue, NullValue})
	assertColumnValues(t, got, "max_bid", []any{101.0, NullValue, NullValue})
	assertColumnValues(t, got, "first_bid", []any{100.0, NullValue, NullValue})
	assertColumnValues(t, got, "last_bid", []any{101.0, NullValue, NullValue})
}

func TestAsofJoinSameNameKeys(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("ts", []any{int64(1), int64(3), int64(2)}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("ts", []any{int64(2), int64(3), int64(1)}),
		NewColumn("value", []any{"a2", "a3", "b1"}),
	)

	got, err := AsofJoin(left, right, "ts", "sym")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "ts", "value"})
	assertColumnValues(t, got, "value", []any{NullValue, "a3", "b1"})
}

func TestAsofJoinWithoutPartitionUsesGlobalLatestTime(t *testing.T) {
	left := mustFrame(t,
		NewColumn("ts", []any{int64(0), int64(5), int64(10), nil, int64(11)}),
		NewColumn("qty", []any{1, 2, 3, 4, 5}),
		NewColumn("tag", []any{"l0", "l5", "l10", "lnull", "l11"}),
	)
	right := mustFrame(t,
		NewColumn("ts", []any{int64(10), int64(3), nil, int64(10), int64(7)}),
		NewColumn("quote", []any{"r10-first", "r3", "rnull", "r10-last", "r7"}),
		NewColumn("tag", []any{"rt10a", "rt3", "rtnull", "rt10b", "rt7"}),
	)

	got, err := AsofJoin(left, right, "ts")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"ts", "qty", "tag", "quote", "tag_right"})
	assertColumnValues(t, got, "quote", []any{NullValue, "r3", "r10-last", NullValue, "r10-last"})
	assertColumnValues(t, got, "tag_right", []any{NullValue, "rt3", "rt10b", NullValue, "rt10b"})
	if gotKind, ok := got.Schema().Kind("quote"); !ok || gotKind != KindString {
		t.Fatalf("quote kind = %s, ok %v; want %s", gotKind, ok, KindString)
	}
}

func TestWindowJoinWithoutPartitionUsesGlobalTimeWindow(t *testing.T) {
	left := mustFrame(t,
		NewColumn("ts", []any{int64(0), int64(5), int64(10), nil, int64(11)}),
		NewColumn("tag", []any{"l0", "l5", "l10", "lnull", "l11"}),
	)
	right := mustFrame(t,
		NewColumn("ts", []any{int64(10), int64(3), nil, int64(10), int64(7)}),
		NewColumn("quote", []any{"r10-first", "r3", "rnull", "r10-last", "r7"}),
	)

	windowed, err := WindowJoinOn(left, right, JoinKey{Left: "ts", Right: "ts"})
	if err != nil {
		t.Fatalf("WindowJoinOn returned error: %v", err)
	}

	assertColumnNames(t, windowed, []Symbol{"ts", "tag", "quote"})
	assertColumnValues(t, windowed, "quote", []any{[]any{}, []any{"r3"}, []any{"r3", "r7", "r10-first", "r10-last"}, []any{}, []any{"r3", "r7", "r10-first", "r10-last"}})

	last, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey: JoinKey{Left: "ts", Right: "ts"},
		Last:    true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions last returned error: %v", err)
	}
	assertColumnValues(t, last, "quote", []any{NullValue, "r3", "r10-last", NullValue, "r10-last"})
}

func TestAsofJoinDuplicateExactTimesUsesStableLastRightRow(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{int64(9), int64(10), int64(11)}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("a")}),
		NewColumn("ts", []any{int64(8), int64(10), int64(10), int64(12)}),
		NewColumn("quote", []any{"r8", "r10-first", "r10-last", "r12"}),
		NewColumn("seq", []any{1, 2, 3, 4}),
	)

	got, err := AsofJoin(left, right, "ts", "sym")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}

	assertColumnValues(t, got, "quote", []any{"r8", "r10-last", "r10-last"})
	assertColumnValues(t, got, "seq", []any{int64(1), int64(3), int64(3)})
}

func TestAsofAndWindowJoinBoundaryContract(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b"), Symbol("c")}),
		NewColumn("ts", []any{int64(7), int64(10), int64(11), int64(10), int64(10)}),
		NewColumn("trade_id", []any{1, 2, 3, 4, 5}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("ts", []any{int64(8), int64(10), int64(10), int64(11), NullForKind(KindI64)}),
		NewColumn("quote", []any{"a8", "a10-first", "a10-last", "b11", "a-null"}),
		NewColumn("seq", []any{1, 2, 3, 4, 5}),
	)

	asof, err := AsofJoin(left, right, "ts", "sym")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}
	assertColumnValues(t, asof, "quote", []any{NullValue, "a10-last", "a10-last", NullValue, NullValue})
	assertColumnValues(t, asof, "seq", []any{NullValue, int64(3), int64(3), NullValue, NullValue})

	windowed, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           int64(0),
		High:          int64(0),
		HasBounds:     true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions returned error: %v", err)
	}
	assertColumnValues(t, windowed, "quote", []any{
		[]any{},
		[]any{"a10-first", "a10-last"},
		[]any{},
		[]any{},
		[]any{},
	})
	assertColumnValues(t, windowed, "seq", []any{
		[]any{},
		[]any{int64(2), int64(3)},
		[]any{},
		[]any{},
		[]any{},
	})

	last, err := WindowJoinOnWithOptions(left, right, WindowJoinOptions{
		TimeKey:       JoinKey{Left: "ts", Right: "ts"},
		PartitionKeys: []JoinKey{{Left: "sym", Right: "sym"}},
		Low:           int64(0),
		High:          int64(0),
		HasBounds:     true,
		Last:          true,
	})
	if err != nil {
		t.Fatalf("WindowJoinOnWithOptions last returned error: %v", err)
	}
	assertColumnValues(t, last, "quote", []any{NullValue, "a10-last", NullValue, NullValue, NullValue})
	assertColumnValues(t, last, "seq", []any{NullValue, int64(3), NullValue, NullValue, NullValue})
	if gotKind, ok := last.Schema().Kind("seq"); !ok || gotKind != KindI64 {
		t.Fatalf("last seq kind = %s, ok %v; want %s", gotKind, ok, KindI64)
	}
}

func TestAsofJoinTemporalTimeKeySkipsNullTimes(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("time", []any{
			TimeFromNanos(1_000),
			TimeFromNanos(2_500),
			nil,
			TimeFromNanos(2_500),
		}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("a"), Symbol("a"), Symbol("b")}),
		NewColumn("time", []any{
			TimeFromNanos(500),
			TimeFromNanos(2_000),
			nil,
			TimeFromNanos(3_000),
		}),
		NewColumn("quote", []any{"a-500", "a-2000", "a-null", "b-3000"}),
	)

	got, err := AsofJoin(left, right, "time", "sym")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}

	assertColumnValues(t, got, "time", []any{Time(1_000), Time(2_500), NullValue, Time(2_500)})
	assertColumnValues(t, got, "quote", []any{"a-500", "a-2000", NullValue, NullValue})
	if gotKind, ok := got.Schema().Kind("quote"); !ok || gotKind != KindString {
		t.Fatalf("quote kind = %s, ok %v; want %s", gotKind, ok, KindString)
	}
}

func TestQueryMutationJoinAsofTypedBoundary(t *testing.T) {
	trades := mustFrame(t,
		NewColumn("trade_id", []any{int64(1), int64(2), int64(3), int64(4)}),
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT"), Symbol("TSLA")}),
		NewColumn("venue", []any{"XNAS", "XNYS", "XNYS", "XNAS"}),
		NewColumn("trade_date", []any{DateFromDays(20), DateFromDays(20), DateFromDays(21), DateFromDays(21)}),
		NewColumn("event_ts", []any{
			TimestampFromUnixNanos(1_000),
			TimestampFromUnixNanos(2_000),
			TimestampFromUnixNanos(2_500),
			TimestampFromUnixNanos(3_000),
		}),
		NewColumn("price", []any{100.0, 101.0, 80.0, 200.0}),
		NewColumn("size", []any{int64(10), int64(20), int64(30), int64(5)}),
	)

	updated, err := UpdateWhere(trades,
		Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("AAPL")}},
		map[Symbol]Expr{
			"price": Binary{Op: OpAdd, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 1.0}},
			"size":  Binary{Op: OpAdd, Left: ColumnRef{Name: "size"}, Right: Literal{Value: int64(1)}},
		},
	)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	assertColumnValues(t, updated, "price", []any{101.0, 102.0, 80.0, 200.0})
	assertColumnValues(t, updated, "size", []any{int64(11), int64(21), int64(30), int64(5)})
	if gotKind, ok := updated.Schema().Kind("event_ts"); !ok || gotKind != KindTimestamp {
		t.Fatalf("updated event_ts kind = %s, ok %v; want %s", gotKind, ok, KindTimestamp)
	}

	trimmed, err := DeleteWhere(updated, Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("TSLA")}})
	if err != nil {
		t.Fatalf("DeleteWhere returned error: %v", err)
	}
	assertColumnValues(t, trimmed, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})

	venues := mustFrame(t,
		NewColumn("venue", []any{"XNAS", "XNYS"}),
		NewColumn("region", []any{Symbol("US"), Symbol("US")}),
		NewColumn("tier", []any{int64(1), int64(2)}),
	)
	enriched, err := LeftJoin(trimmed, venues, "venue")
	if err != nil {
		t.Fatalf("LeftJoin returned error: %v", err)
	}
	assertColumnValues(t, enriched, "region", []any{Symbol("US"), Symbol("US"), Symbol("US")})
	if gotKind, ok := enriched.Schema().Kind("region"); !ok || gotKind != KindSymbol {
		t.Fatalf("region kind = %s, ok %v; want %s", gotKind, ok, KindSymbol)
	}

	quotes := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("event_ts", []any{
			TimestampFromUnixNanos(500),
			TimestampFromUnixNanos(1_500),
			TimestampFromUnixNanos(2_000),
		}),
		NewColumn("bid", []any{99.0, 100.5, 79.5}),
	)
	joined, err := AsofJoin(enriched, quotes, "event_ts", "sym")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}
	assertColumnValues(t, joined, "bid", []any{99.0, 100.5, 79.5})

	rollup, err := Exec(joined, QueryPlan{
		Source: joined,
		Where:  Within{Expr: ColumnRef{Name: "trade_date"}, Low: DateFromDays(20), High: DateFromDays(21), HighClosed: true},
		By:     []Symbol{"sym", "trade_date"},
		Aggregates: []Aggregate{
			{Name: "notional", Func: "sum", Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "size"}}},
			{Name: "fills", Func: "count"},
			{Name: "first_bid", Func: "first", Expr: ColumnRef{Name: "bid"}},
			{Name: "last_region", Func: "last", Expr: ColumnRef{Name: "region"}},
		},
		OrderBy: []OrderSpec{{Column: "sym"}, {Column: "trade_date"}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("rollup Exec returned error: %v", err)
	}
	assertColumnNames(t, rollup, []Symbol{"sym", "trade_date", "notional", "fills", "first_bid", "last_region"})
	assertColumnValues(t, rollup, "sym", []any{Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, rollup, "trade_date", []any{Date(20), Date(21)})
	assertColumnValues(t, rollup, "notional", []any{3253.0, 2400.0})
	assertColumnValues(t, rollup, "fills", []any{int64(2), int64(1)})
	assertColumnValues(t, rollup, "first_bid", []any{99.0, 79.5})
	assertColumnValues(t, rollup, "last_region", []any{Symbol("US"), Symbol("US")})
	if gotKind, ok := rollup.Schema().Kind("trade_date"); !ok || gotKind != KindDate {
		t.Fatalf("rollup trade_date kind = %s, ok %v; want %s", gotKind, ok, KindDate)
	}
}

func TestQueryTypedUnmatchedJoinAsofAndEmptyMutationBoundary(t *testing.T) {
	trades := mustFrame(t,
		NewColumn("trade_id", []any{int64(1), int64(2)}),
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("venue", []any{"XNAS", "XASE"}),
		NewColumn("event_ts", []any{
			TimestampFromUnixNanos(1_000),
			TimestampFromUnixNanos(2_000),
		}),
		NewColumn("price", []any{100.0, 80.0}),
	)

	updated, err := UpdateWhere(trades,
		Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("IBM")}},
		map[Symbol]Expr{"price": Binary{Op: OpAdd, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 1.0}}},
	)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	assertColumnValues(t, updated, "price", []any{100.0, 80.0})

	venues := mustFrame(t,
		NewColumn("venue", []any{"XNAS"}),
		NewColumn("region", []any{Symbol("US")}),
	)
	enriched, err := LeftJoin(updated, venues, "venue")
	if err != nil {
		t.Fatalf("LeftJoin returned error: %v", err)
	}
	assertColumnValues(t, enriched, "region", []any{Symbol("US"), NullValue})
	if gotKind, ok := enriched.Schema().Kind("region"); !ok || gotKind != KindSymbol {
		t.Fatalf("enriched region kind = %s, ok %v; want %s", gotKind, ok, KindSymbol)
	}

	quotes := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL")}),
		NewColumn("event_ts", []any{TimestampFromUnixNanos(1_500)}),
		NewColumn("bid", []any{99.0}),
	)
	joined, err := AsofJoin(enriched, quotes, "event_ts", "sym")
	if err != nil {
		t.Fatalf("AsofJoin returned error: %v", err)
	}
	assertColumnValues(t, joined, "bid", []any{NullValue, NullValue})
	if gotKind, ok := joined.Schema().Kind("bid"); !ok || gotKind != KindF64 {
		t.Fatalf("joined bid kind = %s, ok %v; want %s", gotKind, ok, KindF64)
	}

	rollup, err := Exec(joined, QueryPlan{
		Source: joined,
		By:     []Symbol{"region"},
		Aggregates: []Aggregate{
			{Name: "fills", Func: "count"},
			{Name: "first_bid", Func: "first", Expr: ColumnRef{Name: "bid"}},
		},
		OrderBy: []OrderSpec{{Column: "region"}},
		LimitN:  -1,
	})
	if err != nil {
		t.Fatalf("rollup Exec returned error: %v", err)
	}
	assertColumnValues(t, rollup, "region", []any{NullValue, Symbol("US")})
	assertColumnValues(t, rollup, "fills", []any{int64(1), int64(1)})
	assertColumnValues(t, rollup, "first_bid", []any{NullValue, NullValue})

	empty, err := DeleteWhere(joined, Binary{Op: OpGE, Left: ColumnRef{Name: "price"}, Right: Literal{Value: 0.0}})
	if err != nil {
		t.Fatalf("DeleteWhere returned error: %v", err)
	}
	if empty.Len() != 0 {
		t.Fatalf("empty Len = %d, want 0", empty.Len())
	}
	if gotKind, ok := empty.Schema().Kind("event_ts"); !ok || gotKind != KindTimestamp {
		t.Fatalf("empty event_ts kind = %s, ok %v; want %s", gotKind, ok, KindTimestamp)
	}
	if gotKind, ok := empty.Schema().Kind("region"); !ok || gotKind != KindSymbol {
		t.Fatalf("empty region kind = %s, ok %v; want %s", gotKind, ok, KindSymbol)
	}
	if gotKind, ok := empty.Schema().Kind("bid"); !ok || gotKind != KindF64 {
		t.Fatalf("empty bid kind = %s, ok %v; want %s", gotKind, ok, KindF64)
	}
}

func TestAsofJoinRejectsInvalidKeys(t *testing.T) {
	left := mustFrame(t,
		NewColumn("sym", []any{Symbol("a")}),
		NewColumn("ts", []any{int64(1)}),
	)
	right := mustFrame(t,
		NewColumn("sym", []any{Symbol("a")}),
		NewColumn("time", []any{int64(1)}),
	)

	if _, err := AsofJoinOn(left, right, JoinKey{Left: "ts", Right: "time"}, JoinKey{Left: "missing", Right: "sym"}); err == nil {
		t.Fatal("AsofJoinOn accepted missing partition key")
	}
	if _, err := AsofJoinOn(left, right, JoinKey{Left: "ts", Right: "missing"}, JoinKey{Left: "sym", Right: "sym"}); err == nil {
		t.Fatal("AsofJoinOn accepted missing time key")
	}
	if _, err := AsofJoinOn(left, right, JoinKey{Left: "ts", Right: "time"}, JoinKey{Left: "ts", Right: "time"}); err == nil {
		t.Fatal("AsofJoinOn accepted time key as a partition key")
	}
	if _, err := AsofJoinOn(left, right, JoinKey{Left: "sym", Right: "sym"}); err == nil {
		t.Fatal("AsofJoinOn accepted non-time symbol key")
	}
	if _, err := WindowJoinOn(left, right, JoinKey{Left: "sym", Right: "sym"}); err == nil {
		t.Fatal("WindowJoinOn accepted non-time symbol key")
	}
}

func TestXGroupAndUngroup(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")}),
		NewColumn("venue", []any{Symbol("XNYS"), Symbol("XNYS"), Symbol("XNAS")}),
		NewColumn("price", []any{int64(100), int64(101), int64(80)}),
		NewColumn("size", []any{int64(10), int64(11), int64(20)}),
	)

	grouped, err := XGroup(frame, "sym", "venue")
	if err != nil {
		t.Fatalf("XGroup returned error: %v", err)
	}
	if keys := grouped.Keys(); !reflect.DeepEqual(keys, []Symbol{"sym", "venue"}) {
		t.Fatalf("grouped keys = %#v, want sym venue", keys)
	}
	groupedFrame := grouped.Frame()
	assertColumnNames(t, groupedFrame, []Symbol{"sym", "venue", "price", "size"})
	if groupedFrame.Len() != 2 {
		t.Fatalf("grouped Len = %d, want 2", groupedFrame.Len())
	}
	priceCol := mustColumn(t, groupedFrame, "price")
	firstPrices, ok := mustArrayCell(t, priceCol, 0)
	if !ok || !reflect.DeepEqual(firstPrices.Values(), []any{int64(100), int64(101)}) {
		t.Fatalf("first grouped prices = %#v", firstPrices)
	}
	secondPrices, ok := mustArrayCell(t, priceCol, 1)
	if !ok || !reflect.DeepEqual(secondPrices.Values(), []any{int64(80)}) {
		t.Fatalf("second grouped prices = %#v", secondPrices)
	}

	ungrouped, err := Ungroup(grouped.Frame())
	if err != nil {
		t.Fatalf("Ungroup returned error: %v", err)
	}
	assertColumnNames(t, ungrouped, []Symbol{"sym", "venue", "price", "size"})
	assertColumnValues(t, ungrouped, "sym", []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("MSFT")})
	assertColumnValues(t, ungrouped, "venue", []any{Symbol("XNYS"), Symbol("XNYS"), Symbol("XNAS")})
	assertColumnValues(t, ungrouped, "price", []any{int64(100), int64(101), int64(80)})
	assertColumnValues(t, ungrouped, "size", []any{int64(10), int64(11), int64(20)})
}

func TestUngroupRejectsMismatchedNestedLengths(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("AAPL")}),
		NewColumn("price", []any{NewI64([]int64{100, 101})}),
		NewColumn("size", []any{NewI64([]int64{10})}),
	)
	if _, err := Ungroup(frame); err == nil {
		t.Fatal("Ungroup accepted mismatched nested column lengths")
	}
}

func mustFrame(t testing.TB, cols ...Column) Frame {
	t.Helper()
	frame, err := NewFrame(cols...)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}
	return frame
}

func mustColumn(t testing.TB, frame Frame, name Symbol) Array {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q", name)
	}
	return col
}

func mustArrayCell(t *testing.T, col Array, row int) (Array, bool) {
	t.Helper()
	value, ok := col.At(row)
	if !ok {
		t.Fatalf("column row %d out of range", row)
	}
	array, ok := value.(Array)
	return array, ok
}

func assertColumnNames(t *testing.T, frame Frame, want []Symbol) {
	t.Helper()
	if got := frame.Schema().Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("column names = %v, want %v", got, want)
	}
}

func assertColumnValues(t *testing.T, frame Frame, name Symbol, want []any) {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q", name)
	}
	if got := col.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("column %q = %#v, want %#v", name, got, want)
	}
}

func BenchmarkQueryKernelTypedFilterProjection(b *testing.B) {
	const rows = 10000
	syms := make([]string, rows)
	qty := make([]int64, rows)
	price := make([]float64, rows)
	for i := 0; i < rows; i++ {
		if i%2 == 0 {
			syms[i] = "AAPL"
		} else {
			syms[i] = "MSFT"
		}
		qty[i] = int64(i % 100)
		price[i] = float64(i) * 0.25
	}
	frame := mustFrame(b,
		Column{Name: "sym", Data: NewSymbols(syms)},
		Column{Name: "qty", Data: NewI64(qty)},
		Column{Name: "price", Data: NewF64(price)},
	)
	plan := QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(50)}},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "qty", Expr: ColumnRef{Name: "qty"}},
			{Name: "notional", Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "qty"}, Right: ColumnRef{Name: "price"}}},
		},
		LimitN: -1,
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		b.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		b.Fatal("CompileQueryKernel returned ok=false")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := kernel.Exec(frame)
		if err != nil {
			b.Fatalf("kernel Exec returned error: %v", err)
		}
		if out.Len() != rows/2 {
			b.Fatalf("kernel output len = %d, want %d", out.Len(), rows/2)
		}
	}
}

func BenchmarkExecTypedRangeFilterProjection(b *testing.B) {
	const rows = 100000
	ids := make([]int64, rows)
	syms := make([]string, rows)
	price := make([]float64, rows)
	for i := 0; i < rows; i++ {
		ids[i] = int64(i)
		if i%2 == 0 {
			syms[i] = "AAPL"
		} else {
			syms[i] = "MSFT"
		}
		price[i] = float64(i) * 0.25
	}
	frame := mustFrame(b,
		Column{Name: "id", Data: NewI64(ids)},
		Column{Name: "sym", Data: NewSymbols(syms)},
		Column{Name: "price", Data: NewF64(price)},
	)
	plan := QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpGE, Left: ColumnRef{Name: "id"}, Right: Literal{Value: int64(rows / 2)}},
		Select: []SelectItem{
			{Name: "sym", Expr: ColumnRef{Name: "sym"}},
			{Name: "price", Expr: ColumnRef{Name: "price"}},
		},
		LimitN: -1,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := Exec(frame, plan)
		if err != nil {
			b.Fatalf("Exec returned error: %v", err)
		}
		if out.Len() != rows/2 {
			b.Fatalf("Exec output len = %d, want %d", out.Len(), rows/2)
		}
	}
}

func BenchmarkJoinTypedRangeIndexReuse(b *testing.B) {
	const rows = 100000
	ids := make([]int64, rows)
	for i := 0; i < rows; i++ {
		ids[i] = int64(i)
	}
	left := mustFrame(b,
		Column{Name: "id", Data: NewI64(ids)},
		Column{Name: "qty", Data: NewI64Range(0, 1, rows)},
	)
	right := mustFrame(b,
		Column{Name: "id", Data: NewI64(ids)},
		Column{Name: "px", Data: NewI64Range(1000, 2, rows)},
	)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := InnerJoin(left, right, "id")
		if err != nil {
			b.Fatalf("InnerJoin returned error: %v", err)
		}
		if out.Len() != rows {
			b.Fatalf("join output len = %d, want %d", out.Len(), rows)
		}
		if col := mustColumn(b, out, "px"); !isI64RangeArray(col) {
			b.Fatalf("joined px column = %T, want lazy i64 range", col)
		}
	}
}

func BenchmarkExecFilteredGroupedCountByIndexedSymbol(b *testing.B) {
	const rows = 100000
	syms := make([]string, rows)
	qty := make([]int64, rows)
	for i := 0; i < rows; i++ {
		switch i % 4 {
		case 0:
			syms[i] = "AAPL"
		case 1:
			syms[i] = "MSFT"
		case 2:
			syms[i] = "NVDA"
		default:
			syms[i] = "TSLA"
		}
		qty[i] = int64(i % 100)
	}
	frame := mustFrame(b,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols(syms), ArrayAttributeGrouped)},
		Column{Name: "qty", Data: NewI64(qty)},
	)
	plan := QueryPlan{
		Source: frame,
		Where:  Binary{Op: OpGT, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(50)}},
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "rows", Func: "count"},
		},
		LimitN: -1,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := Exec(frame, plan)
		if err != nil {
			b.Fatalf("Exec returned error: %v", err)
		}
		if out.Len() != 4 {
			b.Fatalf("Exec output len = %d, want 4", out.Len())
		}
	}
}

func BenchmarkExecIndexedGroupedMixedAggregates(b *testing.B) {
	const rows = 100000
	syms := make([]string, rows)
	qty := make([]int32, rows)
	price := make([]float64, rows)
	for i := 0; i < rows; i++ {
		switch i % 4 {
		case 0:
			syms[i] = "AAPL"
		case 1:
			syms[i] = "MSFT"
		case 2:
			syms[i] = "NVDA"
		default:
			syms[i] = "TSLA"
		}
		qty[i] = int32(i % 100)
		price[i] = 100 + float64(i%50)
	}
	frame := mustFrame(b,
		Column{Name: "sym", Data: WithArrayAttribute(NewSymbols(syms), ArrayAttributeGrouped)},
		Column{Name: "qty", Data: NewI32(qty)},
		Column{Name: "price", Data: NewF64(price)},
	)
	plan := QueryPlan{
		Source: frame,
		By:     []Symbol{"sym"},
		Aggregates: []Aggregate{
			{Name: "total_qty", Func: "sum", Expr: ColumnRef{Name: "qty"}},
			{Name: "avg_price", Func: "avg", Expr: ColumnRef{Name: "price"}},
			{Name: "low_price", Func: "min", Expr: ColumnRef{Name: "price"}},
			{Name: "high_price", Func: "max", Expr: ColumnRef{Name: "price"}},
			{Name: "fills", Func: "count"},
		},
		LimitN: -1,
	}
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		b.Fatalf("CompileQueryKernel returned error: %v", err)
	}
	if !ok {
		b.Fatal("CompileQueryKernel returned ok=false")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := kernel.Exec(frame)
		if err != nil {
			b.Fatalf("kernel Exec returned error: %v", err)
		}
		if out.Len() != 4 {
			b.Fatalf("kernel output len = %d, want 4", out.Len())
		}
	}
}

func BenchmarkFilterIndexesTypedCompare(b *testing.B) {
	const rows = 100000
	qty := make([]int64, rows)
	for i := 0; i < rows; i++ {
		qty[i] = int64(i % 100)
	}
	frame := mustFrame(b, Column{Name: "qty", Data: NewI64(qty)})
	where := Binary{Op: OpGE, Left: ColumnRef{Name: "qty"}, Right: Literal{Value: int64(50)}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexes, err := filterIndexes(frame, where)
		if err != nil {
			b.Fatalf("filterIndexes returned error: %v", err)
		}
		if len(indexes) != rows/2 {
			b.Fatalf("indexes len = %d, want %d", len(indexes), rows/2)
		}
	}
}

func BenchmarkFilterIndexesTypedWithin(b *testing.B) {
	const rows = 100000
	ts := make([]Timestamp, rows)
	for i := 0; i < rows; i++ {
		ts[i] = Timestamp(i % 100)
	}
	frame := mustFrame(b, Column{Name: "ts", Data: NewTimestamp(ts)})
	where := Within{Expr: ColumnRef{Name: "ts"}, Low: Timestamp(25), High: Timestamp(75), HighClosed: false}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexes, err := filterIndexes(frame, where)
		if err != nil {
			b.Fatalf("filterIndexes returned error: %v", err)
		}
		if len(indexes) != rows/2 {
			b.Fatalf("indexes len = %d, want %d", len(indexes), rows/2)
		}
	}
}

func BenchmarkFilterIndexesEncodedSymbolCompare(b *testing.B) {
	const rows = 100000
	syms := make([]Symbol, rows)
	for i := 0; i < rows; i++ {
		switch i % 4 {
		case 0:
			syms[i] = "AAPL"
		case 1:
			syms[i] = "MSFT"
		case 2:
			syms[i] = "NVDA"
		default:
			syms[i] = "TSLA"
		}
	}
	frame := mustFrame(b, Column{Name: "sym", Data: NewEncodedSymbols(syms)})
	where := Binary{Op: OpEQ, Left: ColumnRef{Name: "sym"}, Right: Literal{Value: Symbol("AAPL")}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexes, err := filterIndexes(frame, where)
		if err != nil {
			b.Fatalf("filterIndexes returned error: %v", err)
		}
		if len(indexes) != rows/4 {
			b.Fatalf("indexes len = %d, want %d", len(indexes), rows/4)
		}
	}
}

func TestMatrixReshapeTransposeAndMultiply(t *testing.T) {
	reshaped, err := ReshapeArray([]int{2, 3}, NewI64([]int64{1, 2, 3, 4, 5, 6}))
	if err != nil {
		t.Fatalf("ReshapeArray returned error: %v", err)
	}
	matrix, ok := reshaped.(Matrix)
	if !ok {
		t.Fatalf("ReshapeArray = %T, want Matrix", reshaped)
	}
	if got, want := matrix.Shape(), []int{2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("matrix shape = %#v, want %#v", got, want)
	}
	if got, ok := matrix.Cell(1, 2); !ok || got != int64(6) {
		t.Fatalf("matrix cell 1,2 = %#v ok %v, want 6", got, ok)
	}
	row, ok := matrix.RowArray(0)
	if !ok || !reflect.DeepEqual(row.Values(), []any{int64(1), int64(2), int64(3)}) {
		t.Fatalf("matrix row 0 = %#v ok %v", row, ok)
	}
	gatheredRow := row.Gather([]int{2, 0})
	if got, want := gatheredRow.Kind(), KindI64; got != want {
		t.Fatalf("matrix row gather kind = %s, want %s", got, want)
	}
	if got, want := gatheredRow.Values(), []any{int64(3), int64(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("matrix row gather values = %#v, want %#v", got, want)
	}
	cell, handled, err := TryMatrixCellIndex(matrix, 1, 2)
	if err != nil || !handled || cell != int64(6) {
		t.Fatalf("TryMatrixCellIndex = %#v,%v,%v; want 6,true,nil", cell, handled, err)
	}
	rowSum, handled, err := TryMatrixRowNumericSum(matrix, 1)
	if err != nil || !handled || rowSum != int64(15) {
		t.Fatalf("TryMatrixRowNumericSum = %#v,%v,%v; want 15,true,nil", rowSum, handled, err)
	}
	rowSumCount, handled, err := TryMatrixRowNumericSumCount(matrix, 1)
	if err != nil || !handled || rowSumCount != int64(18) {
		t.Fatalf("TryMatrixRowNumericSumCount = %#v,%v,%v; want 18,true,nil", rowSumCount, handled, err)
	}

	transposed, err := TransposeMatrix(matrix)
	if err != nil {
		t.Fatalf("TransposeMatrix returned error: %v", err)
	}
	tm := transposed.(Matrix)
	if got, want := tm.Shape(), []int{3, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("transpose shape = %#v, want %#v", got, want)
	}
	transposedCell, handled, err := TryMatrixCellIndex(tm, 2, 1)
	if err != nil || !handled || transposedCell != int64(6) {
		t.Fatalf("TryMatrixCellIndex(transposed) = %#v,%v,%v; want 6,true,nil", transposedCell, handled, err)
	}
	if got, ok := tm.Cell(2, 1); !ok || got != int64(6) {
		t.Fatalf("transpose cell 2,1 = %#v ok %v, want 6", got, ok)
	}
	transposedRow, ok := tm.RowArray(2)
	if !ok {
		t.Fatal("transpose row 2 missing")
	}
	gatheredTransposedRow := transposedRow.Gather([]int{1, 0})
	if got, want := gatheredTransposedRow.Kind(), KindI64; got != want {
		t.Fatalf("transpose row gather kind = %s, want %s", got, want)
	}
	if got, want := gatheredTransposedRow.Values(), []any{int64(6), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("transpose row gather values = %#v, want %#v", got, want)
	}
	flattenedTranspose, handled, err := FlattenNestedArray(tm)
	if err != nil || !handled {
		t.Fatalf("FlattenNestedArray(transpose) returned %#v,%v,%v; want handled nil error", flattenedTranspose, handled, err)
	}
	if got, want := flattenedTranspose.Kind(), KindI64; got != want {
		t.Fatalf("flatten transpose kind = %s, want %s", got, want)
	}
	if got, want := flattenedTranspose.Values(), []any{int64(1), int64(4), int64(2), int64(5), int64(3), int64(6)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("flatten transpose values = %#v, want transpose-order flat values %#v", got, want)
	}
	transposedSum, handled, err := TryTypedNestedNumericSum(transposed)
	if err != nil || !handled {
		t.Fatalf("TryTypedNestedNumericSum transposed = %#v,%v,%v; want handled nil error", transposedSum, handled, err)
	}
	if got, want := transposedSum, int64(21); got != want {
		t.Fatalf("TryTypedNestedNumericSum transposed = %#v, want %#v", got, want)
	}

	fromRows, ok, err := MatrixFromRows(NewAny([]any{
		NewI64([]int64{1, 2}),
		NewI64([]int64{3, 4}),
		NewI64([]int64{5, 6}),
	}))
	if err != nil || !ok {
		t.Fatalf("MatrixFromRows returned %#v,%v,%v; want matrix,true,nil", fromRows, ok, err)
	}
	if got, want := fromRows.Shape(), []int{3, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MatrixFromRows shape = %#v, want %#v", got, want)
	}
	if got, ok := fromRows.Cell(2, 1); !ok || got != int64(6) {
		t.Fatalf("MatrixFromRows cell 2,1 = %#v ok %v, want 6", got, ok)
	}
	if _, ok, err := MatrixFromRows(NewAny([]any{
		NewI64([]int64{1, 2}),
		NewI64([]int64{3}),
	})); err == nil || !ok {
		t.Fatalf("MatrixFromRows ragged rows = ok %v err %v; want ok true and error", ok, err)
	}

	right, err := ReshapeArray([]int{3, 2}, NewI64([]int64{10, 20, 30, 40, 50, 60}))
	if err != nil {
		t.Fatalf("right reshape returned error: %v", err)
	}
	product, err := MatrixMultiplyNumeric(matrix, right.(Matrix))
	if err != nil {
		t.Fatalf("MatrixMultiplyNumeric returned error: %v", err)
	}
	pm := product.(Matrix)
	if got, want := pm.Shape(), []int{2, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("product shape = %#v, want %#v", got, want)
	}
	if got, ok := pm.Cell(1, 1); !ok || got != 640.0 {
		t.Fatalf("product cell 1,1 = %#v ok %v, want 640", got, ok)
	}

	inverse, err := MatrixInverseNumeric(matrixArray{
		shape: []int{2, 2},
		data:  NewF64([]float64{4, 7, 2, 6}),
	})
	if err != nil {
		t.Fatalf("MatrixInverseNumeric returned error: %v", err)
	}
	im := inverse.(Matrix)
	want := []float64{0.6, -0.7, -0.2, 0.4}
	for i, expected := range want {
		got, ok := im.Cell(i/2, i%2)
		if !ok {
			t.Fatalf("inverse cell %d missing", i)
		}
		if math.Abs(got.(float64)-expected) > 1e-12 {
			t.Fatalf("inverse cell %d = %v, want %v", i, got, expected)
		}
	}
}

func TestSequenceTransformRuntimeHelpers(t *testing.T) {
	values := NewI64Range(1, 1, 8)

	count, handled, err := SequenceTransformCount(SequenceTransformReverse, nil, values)
	if err != nil || !handled || count != 8 {
		t.Fatalf("SequenceTransformCount reverse = %d,%v,%v; want 8,true,nil", count, handled, err)
	}

	sum, handled, err := TryTypedSequenceTransformNumericSum(SequenceTransformReverse, nil, values)
	if err != nil || !handled || sum != int64(36) {
		t.Fatalf("TryTypedSequenceTransformNumericSum reverse = %#v,%v,%v; want 36,true,nil", sum, handled, err)
	}

	sum, handled, err = TryTypedSequenceTransformNumericSum(SequenceTransformRotate, []int{3}, values)
	if err != nil || !handled || sum != int64(36) {
		t.Fatalf("TryTypedSequenceTransformNumericSum rotate = %#v,%v,%v; want 36,true,nil", sum, handled, err)
	}

	sum, handled, err = TryTypedSequenceTransformNumericSum(SequenceTransformSublist, []int{2, 4}, values)
	if err != nil || !handled || sum != int64(18) {
		t.Fatalf("TryTypedSequenceTransformNumericSum sublist = %#v,%v,%v; want 18,true,nil", sum, handled, err)
	}

	ratioSum, handled, err := TryTypedSequenceTransformNumericSum(SequenceTransformRatios, nil, NewI64([]int64{2, 4, 8, 16}))
	if err != nil || !handled || ratioSum != float64(8) {
		t.Fatalf("TryTypedSequenceTransformNumericSum ratios = %#v,%v,%v; want 8,true,nil", ratioSum, handled, err)
	}

	cut, err := Cut([]int{0, 3, 6}, values)
	if err != nil {
		t.Fatalf("Cut range returned error: %v", err)
	}
	segments := cut.(Array)
	first, _ := segments.At(0)
	if got, want := first.(Array).Values(), []any{int64(1), int64(2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cut first segment = %#v, want %#v", got, want)
	}
	razeSum, handled, err := TryTypedSequenceTransformNumericSum(SequenceTransformRaze, nil, segments)
	if err != nil || !handled || razeSum != int64(36) {
		t.Fatalf("TryTypedSequenceTransformNumericSum raze cut = %#v,%v,%v; want 36,true,nil", razeSum, handled, err)
	}
}
