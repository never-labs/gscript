// Package data provides Leia's runtime-independent columnar data foundation.
package data

import (
	"container/heap"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Kind string

const (
	KindAny       Kind = "any"
	KindNull      Kind = "null"
	KindBool      Kind = "bool"
	KindI8        Kind = "i8"
	KindI16       Kind = "i16"
	KindI32       Kind = "i32"
	KindI64       Kind = "i64"
	KindU8        Kind = "u8"
	KindU16       Kind = "u16"
	KindU32       Kind = "u32"
	KindU64       Kind = "u64"
	KindF32       Kind = "f32"
	KindF64       Kind = "f64"
	KindString    Kind = "string"
	KindSymbol    Kind = "symbol"
	KindMonth     Kind = "month"
	KindDate      Kind = "date"
	KindDateTime  Kind = "datetime"
	KindTimespan  Kind = "timespan"
	KindMinute    Kind = "minute"
	KindSecond    Kind = "second"
	KindTime      Kind = "time"
	KindTimestamp Kind = "timestamp"
)

// RuntimeValueKind names data values when they cross into the runtime through a
// table facade. The constants live in the data foundation so q/data bindings can
// agree on stable categories without duplicating marker strings.
type RuntimeValueKind string

const (
	RuntimeValueFrame      RuntimeValueKind = "data_frame"
	RuntimeValueColumn     RuntimeValueKind = "data_column"
	RuntimeValueKeyedFrame RuntimeValueKind = "data_keyed_frame"
)

// Symbol is Leia's categorical scalar. q symbols lower to this type, but the
// type is general-purpose and belongs to the data foundation.
type Symbol string

// Date is a calendar date encoded as days since 1970-01-01.
type Date int64

// Month is a calendar month encoded as months since 1970-01.
type Month int64

// DateTime is a civil date-time encoded as nanoseconds since 1970-01-01
// 00:00:00 on the proleptic Gregorian calendar.
type DateTime int64

// Timespan is a duration encoded as nanoseconds.
type Timespan int64

// Minute is a time-of-day encoded as minutes since midnight.
type Minute int64

// Second is a time-of-day encoded as seconds since midnight.
type Second int64

// Time is a time-of-day encoded as nanoseconds since midnight.
type Time int64

// Timestamp is an instant encoded as Unix nanoseconds.
type Timestamp int64

const nanosPerDay int64 = 24 * 60 * 60 * 1_000_000_000

func MonthFromMonths(months int64) Month { return Month(months) }

func (m Month) Months() int64 { return int64(m) }

func DateFromDays(days int64) Date { return Date(days) }

func (d Date) Days() int64 { return int64(d) }

func DateTimeFromUnixNanos(nanos int64) DateTime { return DateTime(nanos) }

func (d DateTime) UnixNanos() int64 { return int64(d) }

func TimespanFromNanos(nanos int64) Timespan { return Timespan(nanos) }

func (t Timespan) Nanos() int64 { return int64(t) }

func MinuteFromMinutes(minutes int64) Minute { return Minute(minutes) }

func (m Minute) Minutes() int64 { return int64(m) }

func (m Minute) Valid() bool { return m >= 0 && int64(m) < 24*60 }

func SecondFromSeconds(seconds int64) Second { return Second(seconds) }

func (s Second) Seconds() int64 { return int64(s) }

func (s Second) Valid() bool { return s >= 0 && int64(s) < 24*60*60 }

func TimeFromNanos(nanos int64) Time { return Time(nanos) }

func (t Time) Nanos() int64 { return int64(t) }

func (t Time) Valid() bool { return t >= 0 && int64(t) < nanosPerDay }

func TimestampFromUnixNanos(nanos int64) Timestamp { return Timestamp(nanos) }

func (t Timestamp) UnixNanos() int64 { return int64(t) }

// Null is Leia's missing scalar. Use NullValue for explicit nulls; nil inputs
// are accepted by constructors and normalized to NullValue in arrays.
type Null struct{}

var NullValue = Null{}

// TypedNull is a missing scalar with an intended data kind. It is useful at
// expression boundaries, where the source language carries a null's target type
// before it is normalized into a columnar array.
type TypedNull struct {
	Kind Kind
}

func NullForKind(kind Kind) any {
	if kind == "" || kind == KindAny || kind == KindNull {
		return NullValue
	}
	return TypedNull{Kind: kind}
}

func IsNull(v any) bool {
	if v == nil {
		return true
	}
	switch v.(type) {
	case Null, TypedNull:
		return true
	default:
		return false
	}
}

func NullKind(v any) (Kind, bool) {
	if typed, ok := v.(TypedNull); ok {
		return typed.Kind, typed.Kind != "" && typed.Kind != KindAny && typed.Kind != KindNull
	}
	if IsNull(v) {
		return KindNull, true
	}
	return "", false
}

type Array interface {
	Kind() Kind
	Len() int
	At(row int) (any, bool)
	Values() []any
	Gather(indexes []int) Array
}

const (
	ArrayAttributeSorted  Symbol = "s"
	ArrayAttributeGrouped Symbol = "g"
	ArrayAttributeParted  Symbol = "p"
	ArrayAttributeUnique  Symbol = "u"
)

// ArrayMetadata carries optional planner-facing facts about a columnar array.
// It is intentionally small and runtime-independent: q attributes, future
// indexes, and typed kernels can all preserve these facts without importing q.
type ArrayMetadata struct {
	Attributes []Symbol
	Indexes    map[Symbol]ArrayIndex
}

func (m ArrayMetadata) HasAttribute(attr Symbol) bool {
	for _, existing := range m.Attributes {
		if existing == attr {
			return true
		}
	}
	return false
}

func (m ArrayMetadata) HasIndex(attr Symbol) bool {
	_, ok := m.Indexes[attr]
	return ok
}

func (m ArrayMetadata) Index(attr Symbol) (ArrayIndex, bool) {
	index, ok := m.Indexes[attr]
	if !ok {
		return ArrayIndex{}, false
	}
	return index.clone(), true
}

func (m ArrayMetadata) indexBorrowed(attr Symbol) (ArrayIndex, bool) {
	index, ok := m.Indexes[attr]
	return index, ok
}

type arrayMetadataProvider interface {
	ArrayMetadata() ArrayMetadata
}

// ArrayIndex is a reusable value-to-rows sidecar for planner attributes such as
// q's `g#` and `u#`. Keys and row lists preserve first-seen order.
type ArrayIndex struct {
	Attribute      Symbol
	Keys           []any
	KeyArray       Array
	Rows           [][]int
	RowsByKey      map[string][]int
	RowToGroup     []int
	typedRowsByKey any
}

func (idx ArrayIndex) clone() ArrayIndex {
	out := ArrayIndex{
		Attribute:      idx.Attribute,
		Keys:           append([]any(nil), idx.Keys...),
		KeyArray:       idx.KeyArray,
		Rows:           make([][]int, len(idx.Rows)),
		RowToGroup:     append([]int(nil), idx.RowToGroup...),
		typedRowsByKey: idx.typedRowsByKey,
	}
	if idx.RowsByKey != nil {
		out.RowsByKey = make(map[string][]int, len(idx.RowsByKey))
	}
	for i, rows := range idx.Rows {
		out.Rows[i] = append([]int(nil), rows...)
	}
	for key, rows := range idx.RowsByKey {
		out.RowsByKey[key] = append([]int(nil), rows...)
	}
	return out
}

func (m ArrayMetadata) clone() ArrayMetadata {
	out := ArrayMetadata{Attributes: append([]Symbol(nil), m.Attributes...)}
	if len(m.Indexes) > 0 {
		out.Indexes = make(map[Symbol]ArrayIndex, len(m.Indexes))
		for attr, index := range m.Indexes {
			out.Indexes[attr] = index.clone()
		}
	}
	return out
}

func (m ArrayMetadata) cloneWithoutIndexes() ArrayMetadata {
	return ArrayMetadata{Attributes: append([]Symbol(nil), m.Attributes...)}
}

func (m ArrayMetadata) cloneWithRebuiltIndexes(array Array) ArrayMetadata {
	out := ArrayMetadata{Attributes: append([]Symbol(nil), m.Attributes...)}
	if len(m.Indexes) == 0 || array == nil {
		return out
	}
	out.Indexes = make(map[Symbol]ArrayIndex, len(m.Indexes))
	for attr := range m.Indexes {
		index, err := BuildArrayIndex(array, attr)
		if err == nil {
			out.Indexes[attr] = index
		}
	}
	return out
}

type attributedArray struct {
	array    Array
	metadata ArrayMetadata
	// lazy, when non-nil, carries index sidecars that are rebuilt on first
	// use instead of eagerly on every derive (gather/slice/reverse/rotate).
	// metadata.Indexes is empty while lazy is pending; all index reads go
	// through resolvedMetadata.
	lazy *lazyRebuiltIndexes
}

// lazyRebuiltIndexes defers BuildArrayIndex for derived attributed arrays.
// Most derived columns (query projections, mutation survivors) never have
// their index sidecar queried, so rebuilding value->rows maps per derive is
// pure waste; the cell rebuilds once on first index access.
type lazyRebuiltIndexes struct {
	attrs   []Symbol
	once    sync.Once
	indexes map[Symbol]ArrayIndex
}

func (l *lazyRebuiltIndexes) resolve(array Array) map[Symbol]ArrayIndex {
	l.once.Do(func() {
		indexes := make(map[Symbol]ArrayIndex, len(l.attrs))
		for _, attr := range l.attrs {
			if index, err := BuildArrayIndex(array, attr); err == nil {
				indexes[attr] = index
			}
		}
		l.indexes = indexes
	})
	return l.indexes
}

// resolvedMetadata returns the metadata with index sidecars materialized.
func (a attributedArray) resolvedMetadata() ArrayMetadata {
	if a.lazy == nil {
		return a.metadata
	}
	m := a.metadata
	m.Indexes = a.lazy.resolve(a.array)
	return m
}

// indexedAttrs lists the attributes that carry (or will carry) an index.
func (a attributedArray) indexedAttrs() []Symbol {
	if a.lazy != nil {
		return a.lazy.attrs
	}
	if len(a.metadata.Indexes) == 0 {
		return nil
	}
	attrs := make([]Symbol, 0, len(a.metadata.Indexes))
	for attr := range a.metadata.Indexes {
		attrs = append(attrs, attr)
	}
	return attrs
}

// withLazyRebuiltIndexes wraps derived storage with the same attributes and
// index sidecars deferred to first use.
func (a attributedArray) withLazyRebuiltIndexes(derived Array) attributedArray {
	out := attributedArray{array: derived, metadata: a.metadata.cloneWithoutIndexes()}
	if attrs := a.indexedAttrs(); len(attrs) > 0 {
		out.lazy = &lazyRebuiltIndexes{attrs: attrs}
	}
	return out
}

// WithArrayAttribute returns an Array carrying an additional planner attribute.
func WithArrayAttribute(array Array, attr Symbol) Array {
	if array == nil || attr == "" {
		return array
	}
	metadata := ArrayMetadataOf(array)
	if metadata.HasAttribute(attr) {
		return attributedArray{array: array, metadata: metadata}
	}
	metadata.Attributes = append(metadata.Attributes, attr)
	if attr == ArrayAttributeGrouped || attr == ArrayAttributeUnique {
		if index, err := BuildArrayIndex(array, attr); err == nil {
			if metadata.Indexes == nil {
				metadata.Indexes = make(map[Symbol]ArrayIndex, 1)
			}
			metadata.Indexes[attr] = index
		}
	}
	return attributedArray{array: array, metadata: metadata}
}

func ArrayMetadataOf(array Array) ArrayMetadata {
	if array == nil {
		return ArrayMetadata{}
	}
	if provider, ok := array.(arrayMetadataProvider); ok {
		return provider.ArrayMetadata().clone()
	}
	return ArrayMetadata{}
}

func ArrayHasAttribute(array Array, attr Symbol) bool {
	return ArrayMetadataOf(array).HasAttribute(attr)
}

func (a attributedArray) Kind() Kind { return a.array.Kind() }

func (a attributedArray) Len() int { return a.array.Len() }

func (a attributedArray) At(row int) (any, bool) { return a.array.At(row) }

func (a attributedArray) Values() []any { return a.array.Values() }

func (a attributedArray) Gather(indexes []int) Array {
	return a.withLazyRebuiltIndexes(a.array.Gather(indexes))
}

func (a attributedArray) ArrayMetadata() ArrayMetadata {
	return a.resolvedMetadata().clone()
}

func BuildArrayIndex(array Array, attr Symbol) (ArrayIndex, error) {
	if array == nil {
		return ArrayIndex{}, fmt.Errorf("array index source is nil")
	}
	if index, ok := buildArrayIndexTyped(array, attr); ok {
		return index, nil
	}
	index := ArrayIndex{
		Attribute:  attr,
		Keys:       make([]any, 0),
		Rows:       make([][]int, 0),
		RowsByKey:  make(map[string][]int),
		RowToGroup: make([]int, array.Len()),
	}
	positionByKey := make(map[string]int)
	for row := 0; row < array.Len(); row++ {
		value, ok := array.At(row)
		if !ok {
			return ArrayIndex{}, fmt.Errorf("array index row %d out of range", row)
		}
		key := arrayValueKey(array.Kind(), value)
		if position, ok := positionByKey[key]; ok {
			index.RowToGroup[row] = position
			index.Rows[position] = append(index.Rows[position], row)
			index.RowsByKey[key] = append(index.RowsByKey[key], row)
			continue
		}
		positionByKey[key] = len(index.Keys)
		index.RowToGroup[row] = len(index.Keys)
		index.Keys = append(index.Keys, value)
		index.Rows = append(index.Rows, []int{row})
		index.RowsByKey[key] = []int{row}
	}
	index.typedRowsByKey = typedRowsByKey(array)
	if keyArray, ok, err := typedGroupedAggregateKeyArrayIdentity(array.Kind(), index.Keys); err == nil && ok {
		index.KeyArray = keyArray
	}
	return index, nil
}

// buildArrayIndexTyped builds the grouped/unique ArrayIndex through the typed
// fby group-id kernels instead of per-row boxed At + string keys. It is
// restricted to kinds whose typed equality matches arrayValueKey equality
// (floats are excluded: -0.0 and 0.0 compare equal typed but key apart).
func buildArrayIndexTyped(array Array, attr Symbol) (ArrayIndex, bool) {
	switch array.Kind() {
	case KindF32, KindF64, KindAny, KindNull:
		return ArrayIndex{}, false
	}
	ids, count, err := fbyGroupIDs(array)
	if err != nil || len(ids) != array.Len() {
		return ArrayIndex{}, false
	}
	rows := rowGroupsFromIDs(ids, count)
	keys := make([]any, count)
	rowsByKey := make(map[string][]int, count)
	for g, groupRows := range rows {
		v, ok := array.At(groupRows[0])
		if !ok {
			return ArrayIndex{}, false
		}
		keys[g] = v
		key := arrayValueKey(array.Kind(), v)
		if _, dup := rowsByKey[key]; dup {
			// Typed equality split keys that the string encoding merges;
			// defer to the boxed builder for exact legacy grouping.
			return ArrayIndex{}, false
		}
		rowsByKey[key] = groupRows
	}
	keyArray, keyArrayOK, err := typedGroupedAggregateKeyArrayIdentity(array.Kind(), keys)
	if err != nil || !keyArrayOK {
		return ArrayIndex{}, false
	}
	return ArrayIndex{
		Attribute:      attr,
		Keys:           keys,
		KeyArray:       keyArray,
		Rows:           rows,
		RowsByKey:      rowsByKey,
		RowToGroup:     ids,
		typedRowsByKey: typedRowsByKey(array),
	}, true
}

func typedRowsByKey(array Array) any {
	switch a := array.(type) {
	case attributedArray:
		return typedRowsByKey(a.array)
	case columnArray[bool]:
		return typedRowsByKeyFor(a.data)
	case columnArray[int8]:
		return typedRowsByKeyFor(a.data)
	case columnArray[int16]:
		return typedRowsByKeyFor(a.data)
	case columnArray[int32]:
		return typedRowsByKeyFor(a.data)
	case columnArray[int64]:
		return typedRowsByKeyFor(a.data)
	case columnArray[uint8]:
		return typedRowsByKeyFor(a.data)
	case columnArray[uint16]:
		return typedRowsByKeyFor(a.data)
	case columnArray[uint32]:
		return typedRowsByKeyFor(a.data)
	case columnArray[uint64]:
		return typedRowsByKeyFor(a.data)
	case columnArray[float32]:
		return typedRowsByKeyFor(a.data)
	case columnArray[float64]:
		return typedRowsByKeyFor(a.data)
	case columnArray[string]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Symbol]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Month]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Date]:
		return typedRowsByKeyFor(a.data)
	case columnArray[DateTime]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Timespan]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Minute]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Second]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Time]:
		return typedRowsByKeyFor(a.data)
	case columnArray[Timestamp]:
		return typedRowsByKeyFor(a.data)
	default:
		return nil
	}
}

func typedRowsByKeyFor[T comparable](values []T) map[T][]int {
	rowsByKey := make(map[T][]int, len(values))
	for row, value := range values {
		rowsByKey[value] = append(rowsByKey[value], row)
	}
	return rowsByKey
}

func ArrayIndexFor(array Array, attr Symbol) (ArrayIndex, bool) {
	if array == nil {
		return ArrayIndex{}, false
	}
	if index, ok := ArrayMetadataOf(array).Index(attr); ok {
		return index, true
	}
	return ArrayIndex{}, false
}

func arrayIndexForBorrowed(array Array, attr Symbol) (ArrayIndex, bool) {
	switch a := array.(type) {
	case attributedArray:
		return a.resolvedMetadata().indexBorrowed(attr)
	case storedAttributedEncodedArray:
		return a.metadata.indexBorrowed(attr)
	default:
		if index, ok := ArrayIndexFor(array, attr); ok {
			return index, true
		}
		return ArrayIndex{}, false
	}
}

func arrayValueKey(kind Kind, value any) string {
	var b strings.Builder
	appendKeyPart(&b, kind, value)
	return b.String()
}

type columnArray[T any] struct {
	kind Kind
	data []T
}

type i64RangeArray struct {
	start int64
	step  int64
	len   int
}

type f64RangeArray struct {
	start float64
	step  float64
	len   int
}

type i64RunningSumArray struct {
	source i64RangeArray
}

type f64RunningSumArray struct {
	source f64RangeArray
}

type i64SegmentArray struct {
	segments []i64RangeArray
	len      int
}

type i64Int32IndexArray struct {
	rows []int32
}

type i64PeriodicIndexArray struct {
	period       int64
	residues     []int64
	tailResidues []int64
	fullCycles   int64
	len          int
}

type i64ProductArray struct {
	left  i64RangeArray
	right i64RangeArray
}

type i64DyadicProductArray struct {
	left  Array
	right Array
	len   int
}

type i64BucketArray struct {
	source Array
	width  int64
	len    int
}

type f64BucketArray struct {
	source Array
	width  float64
	kind   Kind
	len    int
}

// castF32Array is an immutable lazy `real$` (f32) cast view over a null-free
// numeric source carrier. It defers materializing a float32 column per
// evaluation; consumers stream it through NumericAt or the bulk flatteners
// with float32 rounding applied per element.
type castF32Array struct {
	source Array
}

// castI64Array is an immutable lazy `long$` cast view over a float source
// whose values were validated (finite, in int64 range) when the cast was
// evaluated, preserving eager cast error semantics.
type castI64Array struct {
	source Array
}

type i64XrankArray struct {
	source      Array
	bucketCount int64
	domainSize  int64
	len         int
}

type nullableArray struct {
	kind Kind
	data []any
}

type tiledArray struct {
	source Array
	start  int
	len    int
}

type i64SparseAmendArray struct {
	source  Array
	indexes []int
	values  []int64
}

type shiftedArray struct {
	source Array
	offset int
}

type encodedArray struct {
	kind   Kind
	domain []any
	codes  []int32
}

type indexedArray struct {
	source  Array
	indexes Array
	len     int
}

type intIndexArray struct {
	rows []int
}

// EncodedArrayInfo exposes dictionary-encoded column storage. The decoded
// values remain visible through Array.At and Array.Values; consumers that can
// exploit categorical storage can inspect the stable domain and row codes.
type EncodedArrayInfo interface {
	EncodedDomain() []any
	EncodedCodes() []int32
}

func NewBool(values []bool) Array {
	return columnArray[bool]{kind: KindBool, data: append([]bool(nil), values...)}
}

func newBoolTrusted(values []bool) Array {
	return columnArray[bool]{kind: KindBool, data: values}
}

// NewBoolBorrowed wraps values without copying. Callers must keep the backing
// slice immutable for the lifetime of the returned Array.
func NewBoolBorrowed(values []bool) Array {
	return newBoolTrusted(values)
}

func NewI8(values []int8) Array {
	return columnArray[int8]{kind: KindI8, data: append([]int8(nil), values...)}
}

func NewI16(values []int16) Array {
	return columnArray[int16]{kind: KindI16, data: append([]int16(nil), values...)}
}

func NewI32(values []int32) Array {
	return columnArray[int32]{kind: KindI32, data: append([]int32(nil), values...)}
}

func NewI64(values []int64) Array {
	return columnArray[int64]{kind: KindI64, data: append([]int64(nil), values...)}
}

func newI64Trusted(values []int64) Array {
	return columnArray[int64]{kind: KindI64, data: values}
}

// NewI64Borrowed wraps values without copying. Callers must keep the backing
// slice immutable for the lifetime of the returned Array.
func NewI64Borrowed(values []int64) Array {
	return newI64Trusted(values)
}

func NewI64Range(start, step int64, length int) Array {
	if length < 0 {
		panic(fmt.Sprintf("negative i64 range length %d", length))
	}
	return i64RangeArray{start: start, step: step, len: length}
}

func NewU8(values []uint8) Array {
	return columnArray[uint8]{kind: KindU8, data: append([]uint8(nil), values...)}
}

func NewU16(values []uint16) Array {
	return columnArray[uint16]{kind: KindU16, data: append([]uint16(nil), values...)}
}

func NewU32(values []uint32) Array {
	return columnArray[uint32]{kind: KindU32, data: append([]uint32(nil), values...)}
}

func NewU64(values []uint64) Array {
	return columnArray[uint64]{kind: KindU64, data: append([]uint64(nil), values...)}
}

func NewF32(values []float32) Array {
	return columnArray[float32]{kind: KindF32, data: append([]float32(nil), values...)}
}

func NewF64(values []float64) Array {
	return columnArray[float64]{kind: KindF64, data: append([]float64(nil), values...)}
}

func newF64Trusted(values []float64) Array {
	return columnArray[float64]{kind: KindF64, data: values}
}

// NewF64Borrowed wraps values without copying. Callers must keep the backing
// slice immutable for the lifetime of the returned Array.
func NewF64Borrowed(values []float64) Array {
	return newF64Trusted(values)
}

func NewString(values []string) Array {
	return columnArray[string]{kind: KindString, data: append([]string(nil), values...)}
}

// NewStringBorrowed wraps values without copying. Callers must keep the
// backing slice immutable for the lifetime of the returned Array.
func NewStringBorrowed(values []string) Array {
	return columnArray[string]{kind: KindString, data: values}
}

func NewSymbols(values []string) Array {
	out := make([]Symbol, len(values))
	for i, value := range values {
		out[i] = Symbol(value)
	}
	return columnArray[Symbol]{kind: KindSymbol, data: out}
}

func NewMonth(values []Month) Array {
	return columnArray[Month]{kind: KindMonth, data: append([]Month(nil), values...)}
}

func NewDate(values []Date) Array {
	return columnArray[Date]{kind: KindDate, data: append([]Date(nil), values...)}
}

func NewDateTime(values []DateTime) Array {
	return columnArray[DateTime]{kind: KindDateTime, data: append([]DateTime(nil), values...)}
}

func NewTimespan(values []Timespan) Array {
	return columnArray[Timespan]{kind: KindTimespan, data: append([]Timespan(nil), values...)}
}

func NewMinute(values []Minute) Array {
	return columnArray[Minute]{kind: KindMinute, data: append([]Minute(nil), values...)}
}

func NewSecond(values []Second) Array {
	return columnArray[Second]{kind: KindSecond, data: append([]Second(nil), values...)}
}

func NewTime(values []Time) Array {
	return columnArray[Time]{kind: KindTime, data: append([]Time(nil), values...)}
}

func NewTimestamp(values []Timestamp) Array {
	return columnArray[Timestamp]{kind: KindTimestamp, data: append([]Timestamp(nil), values...)}
}

func NewAny(values []any) Array {
	return nullableArray{kind: KindAny, data: normalizeNulls(values)}
}

func NewEncoded(kind Kind, domain []any, codes []int32) (Array, error) {
	copiedDomain := normalizeNulls(domain)
	for i, code := range codes {
		if code < -1 || int(code) >= len(copiedDomain) {
			return nil, fmt.Errorf("encoded array code %d at row %d outside domain length %d", code, i, len(copiedDomain))
		}
	}
	return encodedArray{kind: kind, domain: copiedDomain, codes: append([]int32(nil), codes...)}, nil
}

func NewEncodedSymbols(values []Symbol) Array {
	domain := make([]any, 0, len(values))
	index := make(map[Symbol]int32, len(values))
	codes := make([]int32, len(values))
	for i, value := range values {
		code, ok := index[value]
		if !ok {
			code = int32(len(domain))
			index[value] = code
			domain = append(domain, value)
		}
		codes[i] = code
	}
	array, err := NewEncoded(KindSymbol, domain, codes)
	if err != nil {
		panic(err)
	}
	return array
}

func EncodedDomainOf(array Array) ([]any, bool) {
	encoded, ok := array.(EncodedArrayInfo)
	if !ok {
		return nil, false
	}
	return encoded.EncodedDomain(), true
}

func EncodedCodesOf(array Array) ([]int32, bool) {
	encoded, ok := array.(EncodedArrayInfo)
	if !ok {
		return nil, false
	}
	return encoded.EncodedCodes(), true
}

func (a columnArray[T]) Kind() Kind { return a.kind }

func (a columnArray[T]) Len() int { return len(a.data) }

func (a columnArray[T]) At(row int) (any, bool) {
	if row < 0 || row >= len(a.data) {
		return nil, false
	}
	return a.data[row], true
}

func (a columnArray[T]) Values() []any {
	out := make([]any, len(a.data))
	for i, v := range a.data {
		out[i] = v
	}
	return out
}

func (a columnArray[T]) Gather(indexes []int) Array {
	out := make([]T, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.data) {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		out[i] = a.data[row]
	}
	return columnArray[T]{kind: a.kind, data: out}
}

func (a indexedArray) Kind() Kind { return a.source.Kind() }

func (a indexedArray) Len() int { return a.len }

func (a indexedArray) At(row int) (any, bool) {
	index, ok, err := i64IndexArrayAt(a.indexes, row)
	if err != nil || !ok {
		return nil, false
	}
	return a.source.At(index)
}

func (a indexedArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("indexed array row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a indexedArray) Gather(indexes []int) Array {
	return indexedArray{
		source:  a.source,
		indexes: a.indexes.Gather(indexes),
		len:     len(indexes),
	}
}

func (a indexedArray) ArrayMetadata() ArrayMetadata {
	return ArrayMetadataOf(a.source).cloneWithRebuiltIndexes(a)
}

func (a intIndexArray) Kind() Kind { return KindI64 }

func (a intIndexArray) Len() int { return len(a.rows) }

func (a intIndexArray) At(row int) (any, bool) {
	if row < 0 || row >= len(a.rows) {
		return nil, false
	}
	return int64(a.rows[row]), true
}

func (a intIndexArray) Values() []any {
	out := make([]any, len(a.rows))
	for i, row := range a.rows {
		out[i] = int64(row)
	}
	return out
}

func (a intIndexArray) Gather(indexes []int) Array {
	out := make([]int, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.rows) {
			panic(fmt.Sprintf("index %d out of bounds for index-array length %d", row, len(a.rows)))
		}
		out[i] = a.rows[row]
	}
	return intIndexArray{rows: out}
}

func (a i64Int32IndexArray) Kind() Kind { return KindI64 }

func (a i64Int32IndexArray) Len() int { return len(a.rows) }

func (a i64Int32IndexArray) At(row int) (any, bool) {
	if row < 0 || row >= len(a.rows) {
		return nil, false
	}
	return int64(a.rows[row]), true
}

func (a i64Int32IndexArray) Values() []any {
	out := make([]any, len(a.rows))
	for i, row := range a.rows {
		out[i] = int64(row)
	}
	return out
}

func (a i64Int32IndexArray) Gather(indexes []int) Array {
	out := make([]int32, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.rows) {
			panic(fmt.Sprintf("index %d out of bounds for int32 index-array length %d", row, len(a.rows)))
		}
		out[i] = a.rows[row]
	}
	return i64Int32IndexArray{rows: out}
}

func (a i64RangeArray) Kind() Kind { return KindI64 }

func (a i64RangeArray) Len() int { return a.len }

func (a i64RangeArray) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	return a.start + int64(row)*a.step, true
}

func (a i64RangeArray) Values() []any {
	out := make([]any, a.len)
	for i := range out {
		out[i] = a.start + int64(i)*a.step
	}
	return out
}

func (a i64RangeArray) Gather(indexes []int) Array {
	if gathered, ok := a.gatherRange(indexes); ok {
		return gathered
	}
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data range gather index %d out of range", row))
		}
		out[i] = a.start + int64(row)*a.step
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func (a i64RangeArray) gatherRange(indexes []int) (Array, bool) {
	switch len(indexes) {
	case 0:
		return i64RangeArray{start: a.start, step: a.step, len: 0}, true
	case 1:
		row := indexes[0]
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data range gather index %d out of range", row))
		}
		return i64RangeArray{start: a.start + int64(row)*a.step, step: a.step, len: 1}, true
	}
	first := indexes[0]
	if first < 0 || first >= a.len {
		panic(fmt.Sprintf("data range gather index %d out of range", first))
	}
	indexStep := indexes[1] - indexes[0]
	for _, row := range indexes[1:] {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data range gather index %d out of range", row))
		}
	}
	for i := 2; i < len(indexes); i++ {
		if indexes[i]-indexes[i-1] != indexStep {
			return nil, false
		}
	}
	return i64RangeArray{
		start: a.start + int64(first)*a.step,
		step:  int64(indexStep) * a.step,
		len:   len(indexes),
	}, true
}

func (a i64BucketArray) Kind() Kind { return KindI64 }

func (a i64BucketArray) Len() int { return a.len }

func (a i64BucketArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64BucketArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data i64 bucket row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64BucketArray) Gather(indexes []int) Array {
	if gathered, ok := gatherI64BucketRange(a, indexes); ok {
		return gathered
	}
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data i64 bucket gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64BucketArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	value, ok, err := integerArrayAt(a.source, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return floorInt64(value, a.width), true, nil
}

func (a f64BucketArray) Kind() Kind { return a.kind }

func (a f64BucketArray) Len() int { return a.len }

func (a f64BucketArray) At(row int) (any, bool) {
	value, ok, err := a.f64At(row)
	if err != nil || !ok {
		return nil, false
	}
	if a.kind == KindF32 {
		return float32(value), true
	}
	return value, true
}

func (a f64BucketArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.f64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data f64 bucket row %d out of range", row))
		}
		if a.kind == KindF32 {
			out[row] = float32(value)
		} else {
			out[row] = value
		}
	}
	return out
}

func (a f64BucketArray) Gather(indexes []int) Array {
	out := make([]float64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.f64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data f64 bucket gather row %d out of range", row))
		}
		out[i] = value
	}
	if a.kind == KindF32 {
		values := make([]float32, len(out))
		for i, value := range out {
			values[i] = float32(value)
		}
		return columnArray[float32]{kind: KindF32, data: values}
	}
	return newF64Trusted(out)
}

func (a f64BucketArray) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	value, ok, err := typedKernels.NumericAt(a.source, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return math.Floor(value/a.width) * a.width, true, nil
}

func (a castF32Array) Kind() Kind { return KindF32 }

func (a castF32Array) Len() int { return a.source.Len() }

func (a castF32Array) f64At(row int) (float64, bool, error) {
	value, ok, err := typedKernels.NumericAt(a.source, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return float64(float32(value)), true, nil
}

func (a castF32Array) At(row int) (any, bool) {
	value, ok, err := a.f64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return float32(value), true
}

func (a castF32Array) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.f64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data cast f32 row %d out of range", row))
		}
		out[row] = float32(value)
	}
	return out
}

func (a castF32Array) Gather(indexes []int) Array {
	out := make([]float32, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.f64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data cast f32 gather row %d out of range", row))
		}
		out[i] = float32(value)
	}
	return columnArray[float32]{kind: KindF32, data: out}
}

func (a castI64Array) Kind() Kind { return KindI64 }

func (a castI64Array) Len() int { return a.source.Len() }

func (a castI64Array) i64At(row int) (int64, bool, error) {
	value, ok, err := typedKernels.NumericAt(a.source, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	// Canonical q integer casts round half-to-even.
	return int64(math.RoundToEven(value)), true, nil
}

func (a castI64Array) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a castI64Array) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data cast i64 row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a castI64Array) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data cast i64 gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64XrankArray) Kind() Kind { return KindI64 }

func (a i64XrankArray) Len() int { return a.len }

func (a i64XrankArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64XrankArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data xrank row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64XrankArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data xrank gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64XrankArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	if a.bucketCount <= 0 || a.domainSize <= 0 {
		return 0, false, fmt.Errorf("xrank domain is invalid")
	}
	value, ok, err := integerArrayAt(a.source, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	if value < 0 || value >= a.domainSize {
		return 0, false, nil
	}
	bucket := (value * a.bucketCount) / a.domainSize
	if bucket >= a.bucketCount {
		bucket = a.bucketCount - 1
	}
	return bucket, true, nil
}

func (a i64SegmentArray) Kind() Kind { return KindI64 }

func (a i64SegmentArray) Len() int { return a.len }

func (a i64SegmentArray) At(row int) (any, bool) {
	value, ok := a.i64At(row)
	if !ok {
		return nil, false
	}
	return value, true
}

func (a i64SegmentArray) i64At(row int) (int64, bool) {
	if row < 0 || row >= a.len {
		return 0, false
	}
	offset := row
	for _, segment := range a.segments {
		if offset < segment.len {
			return segment.start + int64(offset)*segment.step, true
		}
		offset -= segment.len
	}
	return 0, false
}

func (a i64SegmentArray) Values() []any {
	out := make([]any, a.len)
	next := 0
	for _, segment := range a.segments {
		for i := 0; i < segment.len; i++ {
			out[next] = segment.start + int64(i)*segment.step
			next++
		}
	}
	return out
}

func (a i64SegmentArray) Gather(indexes []int) Array {
	if gathered, ok := a.gatherRange(indexes); ok {
		return gathered
	}
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok := a.i64At(row)
		if !ok {
			panic(fmt.Sprintf("data segment gather index %d out of range", row))
		}
		out[i] = value
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func (a i64SegmentArray) gatherRange(indexes []int) (Array, bool) {
	switch len(indexes) {
	case 0:
		return i64RangeArray{len: 0}, true
	case 1:
		value, ok := a.i64At(indexes[0])
		if !ok {
			panic(fmt.Sprintf("data segment gather index %d out of range", indexes[0]))
		}
		return i64RangeArray{start: value, step: 1, len: 1}, true
	}
	first, ok := a.i64At(indexes[0])
	if !ok {
		panic(fmt.Sprintf("data segment gather index %d out of range", indexes[0]))
	}
	second, ok := a.i64At(indexes[1])
	if !ok {
		panic(fmt.Sprintf("data segment gather index %d out of range", indexes[1]))
	}
	step := second - first
	prev := second
	for _, row := range indexes[2:] {
		current, ok := a.i64At(row)
		if !ok {
			panic(fmt.Sprintf("data segment gather index %d out of range", row))
		}
		if current-prev != step {
			return nil, false
		}
		prev = current
	}
	return i64RangeArray{start: first, step: step, len: len(indexes)}, true
}

func (a i64PeriodicIndexArray) Kind() Kind { return KindI64 }

func (a i64PeriodicIndexArray) Len() int { return a.len }

func (a i64PeriodicIndexArray) At(row int) (any, bool) {
	value, ok := a.i64At(row)
	if !ok {
		return nil, false
	}
	return value, true
}

func (a i64PeriodicIndexArray) i64At(row int) (int64, bool) {
	if row < 0 || row >= a.len || a.period <= 0 || len(a.residues) == 0 {
		return 0, false
	}
	fullLen := int(a.fullCycles) * len(a.residues)
	if row < fullLen {
		cycle := row / len(a.residues)
		offset := row % len(a.residues)
		return int64(cycle)*a.period + a.residues[offset], true
	}
	tailRow := row - fullLen
	if tailRow < 0 || tailRow >= len(a.tailResidues) {
		return 0, false
	}
	return a.fullCycles*a.period + a.tailResidues[tailRow], true
}

func (a i64PeriodicIndexArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok := a.i64At(row)
		if !ok {
			panic(fmt.Sprintf("data periodic index row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64PeriodicIndexArray) Gather(indexes []int) Array {
	if gathered, ok := a.gatherRange(indexes); ok {
		return gathered
	}
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok := a.i64At(row)
		if !ok {
			panic(fmt.Sprintf("data periodic index gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64PeriodicIndexArray) gatherRange(indexes []int) (Array, bool) {
	switch len(indexes) {
	case 0:
		return i64RangeArray{len: 0}, true
	case 1:
		value, ok := a.i64At(indexes[0])
		if !ok {
			panic(fmt.Sprintf("data periodic index gather row %d out of range", indexes[0]))
		}
		return i64RangeArray{start: value, step: 1, len: 1}, true
	}
	first, ok := a.i64At(indexes[0])
	if !ok {
		panic(fmt.Sprintf("data periodic index gather row %d out of range", indexes[0]))
	}
	second, ok := a.i64At(indexes[1])
	if !ok {
		panic(fmt.Sprintf("data periodic index gather row %d out of range", indexes[1]))
	}
	step := second - first
	prev := second
	for _, row := range indexes[2:] {
		current, ok := a.i64At(row)
		if !ok {
			panic(fmt.Sprintf("data periodic index gather row %d out of range", row))
		}
		if current-prev != step {
			return nil, false
		}
		prev = current
	}
	return i64RangeArray{start: first, step: step, len: len(indexes)}, true
}

func (a f64RangeArray) Kind() Kind { return KindF64 }

func (a f64RangeArray) Len() int { return a.len }

func (a f64RangeArray) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	return a.start + float64(row)*a.step, true
}

func (a f64RangeArray) Values() []any {
	out := make([]any, a.len)
	for i := range out {
		out[i] = a.start + float64(i)*a.step
	}
	return out
}

func (a f64RangeArray) Gather(indexes []int) Array {
	if gathered, ok := a.gatherRange(indexes); ok {
		return gathered
	}
	out := make([]float64, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data f64 range gather index %d out of range", row))
		}
		out[i] = a.start + float64(row)*a.step
	}
	return columnArray[float64]{kind: KindF64, data: out}
}

func (a f64RangeArray) gatherRange(indexes []int) (Array, bool) {
	if len(indexes) == 0 {
		return f64RangeArray{len: 0}, true
	}
	firstRow := indexes[0]
	if firstRow < 0 || firstRow >= a.len {
		panic(fmt.Sprintf("data f64 range gather index %d out of range", firstRow))
	}
	first := a.start + float64(firstRow)*a.step
	if len(indexes) == 1 {
		return f64RangeArray{start: first, step: a.step, len: 1}, true
	}
	secondRow := indexes[1]
	if secondRow < 0 || secondRow >= a.len {
		panic(fmt.Sprintf("data f64 range gather index %d out of range", secondRow))
	}
	second := a.start + float64(secondRow)*a.step
	step := second - first
	prev := second
	for _, row := range indexes[2:] {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data f64 range gather index %d out of range", row))
		}
		current := a.start + float64(row)*a.step
		if current-prev != step {
			return nil, false
		}
		prev = current
	}
	return f64RangeArray{start: first, step: step, len: len(indexes)}, true
}

func (a i64RunningSumArray) Kind() Kind { return KindI64 }

func (a i64RunningSumArray) Len() int { return a.source.len }

func (a i64RunningSumArray) At(row int) (any, bool) {
	value, ok := a.i64At(row)
	if !ok {
		return nil, false
	}
	return value, true
}

func (a i64RunningSumArray) i64At(row int) (int64, bool) {
	if row < 0 || row >= a.source.len {
		return 0, false
	}
	n := int64(row + 1)
	last := a.source.start + int64(row)*a.source.step
	endpoints := a.source.start + last
	if n%2 == 0 {
		return (n / 2) * endpoints, true
	}
	return n * (endpoints / 2), true
}

func (a i64RunningSumArray) Values() []any {
	out := make([]any, a.Len())
	for i := range out {
		value, _ := a.i64At(i)
		out[i] = value
	}
	return out
}

func (a i64RunningSumArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok := a.i64At(row)
		if !ok {
			panic(fmt.Sprintf("data running sum gather index %d out of range", row))
		}
		out[i] = value
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func (a f64RunningSumArray) Kind() Kind { return KindF64 }

func (a f64RunningSumArray) Len() int { return a.source.len }

func (a f64RunningSumArray) At(row int) (any, bool) {
	value, ok := a.f64At(row)
	if !ok {
		return nil, false
	}
	return value, true
}

func (a f64RunningSumArray) f64At(row int) (float64, bool) {
	if row < 0 || row >= a.source.len {
		return 0, false
	}
	n := float64(row + 1)
	last := a.source.start + float64(row)*a.source.step
	return n * (a.source.start + last) / 2, true
}

func (a f64RunningSumArray) Values() []any {
	out := make([]any, a.Len())
	for i := range out {
		value, _ := a.f64At(i)
		out[i] = value
	}
	return out
}

func (a f64RunningSumArray) Gather(indexes []int) Array {
	out := make([]float64, len(indexes))
	for i, row := range indexes {
		value, ok := a.f64At(row)
		if !ok {
			panic(fmt.Sprintf("data running sum gather index %d out of range", row))
		}
		out[i] = value
	}
	return columnArray[float64]{kind: KindF64, data: out}
}

func (a i64ProductArray) Kind() Kind { return KindI64 }

func (a i64ProductArray) Len() int { return a.left.len }

func (a i64ProductArray) At(row int) (any, bool) {
	value, ok := a.i64At(row)
	if !ok {
		return nil, false
	}
	return value, true
}

func (a i64ProductArray) i64At(row int) (int64, bool) {
	if row < 0 || row >= a.left.len || row >= a.right.len {
		return 0, false
	}
	left := a.left.start + int64(row)*a.left.step
	right := a.right.start + int64(row)*a.right.step
	return left * right, true
}

func (a i64ProductArray) Values() []any {
	out := make([]any, a.Len())
	for i := range out {
		value, _ := a.i64At(i)
		out[i] = value
	}
	return out
}

func (a i64ProductArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok := a.i64At(row)
		if !ok {
			panic(fmt.Sprintf("data product gather index %d out of range", row))
		}
		out[i] = value
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func (a i64DyadicProductArray) Kind() Kind { return KindI64 }

func (a i64DyadicProductArray) Len() int { return a.len }

func (a i64DyadicProductArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64DyadicProductArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, nil
	}
	left, ok, err := integerArrayAt(a.left, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	right, ok, err := integerArrayAt(a.right, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return left * right, true, nil
}

func (a i64DyadicProductArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data i64 product row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64DyadicProductArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data i64 product gather index %d out of range", row))
		}
		out[i] = value
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func (a nullableArray) Kind() Kind { return a.kind }

func (a nullableArray) Len() int { return len(a.data) }

func (a nullableArray) At(row int) (any, bool) {
	if row < 0 || row >= len(a.data) {
		return nil, false
	}
	return a.data[row], true
}

func (a nullableArray) Values() []any {
	return append([]any(nil), a.data...)
}

func (a nullableArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.data) {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		out[i] = a.data[row]
	}
	return nullableArray{kind: a.kind, data: out}
}

func (a tiledArray) Kind() Kind { return a.source.Kind() }

func (a tiledArray) Len() int { return a.len }

func (a tiledArray) At(row int) (any, bool) {
	if row < 0 || row >= a.len || a.source.Len() == 0 {
		return nil, false
	}
	return a.source.At((a.start + row) % a.source.Len())
}

func (a tiledArray) Values() []any {
	out := make([]any, a.len)
	for i := range out {
		value, ok := a.At(i)
		if !ok {
			panic(fmt.Sprintf("data tiled array row %d out of range", i))
		}
		out[i] = value
	}
	return out
}

func (a tiledArray) Gather(indexes []int) Array {
	// Delegate to the source array so gathers keep the source's typed
	// representation (symbol/int columns) instead of boxing every row into a
	// nullableArray, which would knock all downstream kernels off their
	// typed paths (frame column clones, fby grouping, reductions).
	sourceLen := a.source.Len()
	if sourceLen > 0 {
		mapped := make([]int, len(indexes))
		for i, row := range indexes {
			if row < 0 || row >= a.len {
				panic(fmt.Sprintf("data tiled gather index %d out of range", row))
			}
			mapped[i] = (a.start + row) % sourceLen
		}
		return a.source.Gather(mapped)
	}
	out := make([]any, len(indexes))
	for i, row := range indexes {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data tiled gather index %d out of range", row))
		}
		out[i] = value
	}
	return nullableArray{kind: a.Kind(), data: out}
}

func (a shiftedArray) Kind() Kind { return a.source.Kind() }

func (a shiftedArray) Len() int { return a.source.Len() }

func (a shiftedArray) At(row int) (any, bool) {
	if row < 0 || row >= a.Len() {
		return nil, false
	}
	sourceRow := row + a.offset
	if sourceRow < 0 || sourceRow >= a.source.Len() {
		return NullValue, true
	}
	value, ok := a.source.At(sourceRow)
	if !ok {
		return nil, false
	}
	if IsNull(value) {
		return NullValue, true
	}
	return value, true
}

func (a shiftedArray) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data shifted row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a shiftedArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		value, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data shifted gather index %d out of range", row))
		}
		out[i] = value
	}
	return nullableArray{kind: a.Kind(), data: out}
}

func (a encodedArray) Kind() Kind { return a.kind }

func (a encodedArray) Len() int { return len(a.codes) }

func (a encodedArray) At(row int) (any, bool) {
	if row < 0 || row >= len(a.codes) {
		return nil, false
	}
	code := a.codes[row]
	if code < 0 {
		return NullValue, true
	}
	if int(code) >= len(a.domain) {
		return nil, false
	}
	return a.domain[code], true
}

func (a encodedArray) Values() []any {
	out := make([]any, len(a.codes))
	for i := range a.codes {
		v, ok := a.At(i)
		if !ok {
			panic(fmt.Sprintf("encoded array code %d at row %d outside domain length %d", a.codes[i], i, len(a.domain)))
		}
		out[i] = v
	}
	return out
}

func (a encodedArray) Gather(indexes []int) Array {
	out := make([]int32, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.codes) {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		out[i] = a.codes[row]
	}
	return encodedArray{kind: a.kind, domain: append([]any(nil), a.domain...), codes: out}
}

func (a encodedArray) EncodedDomain() []any {
	return append([]any(nil), a.domain...)
}

func (a encodedArray) EncodedCodes() []int32 {
	return append([]int32(nil), a.codes...)
}

type Column struct {
	Name Symbol
	Data Array
}

func NewColumn(name Symbol, values []any) Column {
	return Column{Name: name, Data: InferArray(values)}
}

// NewColumnWithKind builds a column with an explicit storage kind, preserving
// that kind even when all values are null. Bindings use it when source metadata
// already describes the intended column type.
func NewColumnWithKind(name Symbol, kind Kind, values []any) (Column, error) {
	return columnWithKind(name, kind, values)
}

type Schema struct {
	names []Symbol
	kinds map[Symbol]Kind
	// fp memoizes Fingerprint for schemas that are sealed after construction
	// (newFrame attaches it once the column set is final). Schema copies share
	// the same memo cell, so every cache key derived from a frame schema pays
	// the hash + format cost once per frame instead of once per query.
	fp *schemaFingerprintMemo
}

type schemaFingerprintMemo struct {
	once  sync.Once
	value string
}

const cachedFrameSchemaMaxColumns = 8

type cachedFrameSchemaKey struct {
	n      int
	n0, n1 Symbol
	n2, n3 Symbol
	n4, n5 Symbol
	n6, n7 Symbol
	k0, k1 Kind
	k2, k3 Kind
	k4, k5 Kind
	k6, k7 Kind
}

func (k *cachedFrameSchemaKey) set(i int, name Symbol, kind Kind) {
	switch i {
	case 0:
		k.n0, k.k0 = name, kind
	case 1:
		k.n1, k.k1 = name, kind
	case 2:
		k.n2, k.k2 = name, kind
	case 3:
		k.n3, k.k3 = name, kind
	case 4:
		k.n4, k.k4 = name, kind
	case 5:
		k.n5, k.k5 = name, kind
	case 6:
		k.n6, k.k6 = name, kind
	case 7:
		k.n7, k.k7 = name, kind
	}
}

func (k cachedFrameSchemaKey) column(i int) (Symbol, Kind) {
	switch i {
	case 0:
		return k.n0, k.k0
	case 1:
		return k.n1, k.k1
	case 2:
		return k.n2, k.k2
	case 3:
		return k.n3, k.k3
	case 4:
		return k.n4, k.k4
	case 5:
		return k.n5, k.k5
	case 6:
		return k.n6, k.k6
	case 7:
		return k.n7, k.k7
	default:
		return "", ""
	}
}

type cachedFrameSchemaEntry struct {
	key    cachedFrameSchemaKey
	schema Schema
}

var cachedFrameSchemaSlots [64]atomic.Pointer[cachedFrameSchemaEntry]

func buildCachedFrameSchema(key cachedFrameSchemaKey) Schema {
	names := make([]Symbol, key.n)
	kinds := make(map[Symbol]Kind, key.n)
	for i := 0; i < key.n; i++ {
		name, kind := key.column(i)
		names[i] = name
		kinds[name] = kind
	}
	return Schema{names: names, kinds: kinds, fp: &schemaFingerprintMemo{}}
}

func cachedFrameSchema(key cachedFrameSchemaKey) Schema {
	for i := range cachedFrameSchemaSlots {
		entry := cachedFrameSchemaSlots[i].Load()
		if entry != nil && entry.key == key {
			return entry.schema
		}
	}
	schema := buildCachedFrameSchema(key)
	entry := &cachedFrameSchemaEntry{key: key, schema: schema}
	for i := range cachedFrameSchemaSlots {
		if cachedFrameSchemaSlots[i].CompareAndSwap(nil, entry) {
			return schema
		}
	}
	return schema
}

func (s Schema) Names() []Symbol {
	return append([]Symbol(nil), s.names...)
}

func (s Schema) Kind(name Symbol) (Kind, bool) {
	kind, ok := s.kinds[name]
	return kind, ok
}

func (s Schema) CompatibleWith(other Schema) bool {
	if len(s.names) != len(other.names) {
		return false
	}
	for i, name := range s.names {
		if other.names[i] != name {
			return false
		}
		if s.kinds[name] != other.kinds[name] {
			return false
		}
	}
	return true
}

func (s Schema) Fingerprint() string {
	if s.fp == nil {
		return s.computeFingerprint()
	}
	s.fp.once.Do(func() { s.fp.value = s.computeFingerprint() })
	return s.fp.value
}

func (s Schema) computeFingerprint() string {
	h := fnv.New64a()
	var sep [1]byte
	for _, name := range s.names {
		_, _ = io.WriteString(h, string(name))
		sep[0] = 0
		_, _ = h.Write(sep[:])
		_, _ = io.WriteString(h, string(s.kinds[name]))
		sep[0] = 0xff
		_, _ = h.Write(sep[:])
	}
	const hexDigits = "0123456789abcdef"
	var buf [16]byte
	v := h.Sum64()
	for i := 15; i >= 0; i-- {
		buf[i] = hexDigits[v&0xf]
		v >>= 4
	}
	return string(buf[:])
}

type Frame struct {
	schema  Schema
	columns map[Symbol]Array
	rows    int
}

func NewFrame(cols ...Column) (Frame, error) {
	return newFrame(cols, true)
}

func newFrameTrusted(cols ...Column) (Frame, error) {
	return newFrame(cols, false)
}

// NewFrameAdoptingColumns builds a frame that adopts the given column arrays
// without the defensive per-column clone NewFrame performs. Arrays are
// immutable value carriers in this runtime, so in-process producers that
// already own their columns (q flip, joins, group pipelines) use this to
// keep lazy carriers intact and skip a full table materialization.
func NewFrameAdoptingColumns(cols ...Column) (Frame, error) {
	return newFrameTrusted(cols...)
}

func newFrame(cols []Column, cloneColumns bool) (Frame, error) {
	if len(cols) == 0 {
		return Frame{}, fmt.Errorf("frame requires at least one column")
	}
	cacheSchema := !cloneColumns && len(cols) <= cachedFrameSchemaMaxColumns
	var schemaKey cachedFrameSchemaKey
	if cacheSchema {
		schemaKey.n = len(cols)
	}
	schema := Schema{}
	if !cacheSchema {
		schema = Schema{
			names: make([]Symbol, 0, len(cols)),
			kinds: make(map[Symbol]Kind, len(cols)),
		}
	}
	frame := Frame{
		columns: make(map[Symbol]Array, len(cols)),
		rows:    -1,
	}
	for i, col := range cols {
		if col.Name == "" {
			return Frame{}, fmt.Errorf("frame column %d name must not be empty", i+1)
		}
		if col.Data == nil {
			return Frame{}, fmt.Errorf("frame column %q is nil", col.Name)
		}
		if _, exists := frame.columns[col.Name]; exists {
			return Frame{}, fmt.Errorf("frame column %q is duplicated", col.Name)
		}
		if frame.rows < 0 {
			frame.rows = col.Data.Len()
		} else if frame.rows != col.Data.Len() {
			return Frame{}, fmt.Errorf("frame column %q length %d does not match frame length %d", col.Name, col.Data.Len(), frame.rows)
		}
		kind := col.Data.Kind()
		if cacheSchema {
			schemaKey.set(i, col.Name, kind)
		} else {
			schema.names = append(schema.names, col.Name)
			schema.kinds[col.Name] = kind
		}
		if cloneColumns {
			frame.columns[col.Name] = col.Data.Gather(allIndexes(col.Data.Len()))
		} else {
			frame.columns[col.Name] = col.Data
		}
	}
	if cacheSchema {
		frame.schema = cachedFrameSchema(schemaKey)
	} else {
		schema.fp = &schemaFingerprintMemo{}
		frame.schema = schema
	}
	return frame, nil
}

func (f Frame) Len() int { return f.rows }

// FrameColumnNames returns the frame's column names without touching row data.
func FrameColumnNames(frame Frame) []Symbol {
	return frame.schema.Names()
}

// FrameColumnKinds returns the frame's column kinds in schema order without
// cloning the full schema map.
func FrameColumnKinds(frame Frame) []Kind {
	kinds := make([]Kind, len(frame.schema.names))
	for i, name := range frame.schema.names {
		kinds[i] = frame.schema.kinds[name]
	}
	return kinds
}

// FrameColumnAttributes returns the first q-style attribute recorded for each
// column, or the zero symbol when a column has no attribute.
func FrameColumnAttributes(frame Frame) []Symbol {
	attrs := make([]Symbol, len(frame.schema.names))
	for i, name := range frame.schema.names {
		if column := frame.columns[name]; column != nil {
			metadata := ArrayMetadataOf(column)
			if len(metadata.Attributes) > 0 {
				attrs[i] = metadata.Attributes[0]
			}
		}
	}
	return attrs
}

func (f Frame) Schema() Schema {
	kinds := make(map[Symbol]Kind, len(f.schema.kinds))
	for name, kind := range f.schema.kinds {
		kinds[name] = kind
	}
	return Schema{names: append([]Symbol(nil), f.schema.names...), kinds: kinds, fp: f.schema.fp}
}

func (f Frame) SchemaFingerprint() string {
	return f.schema.Fingerprint()
}

func (f Frame) Clone() (Frame, error) {
	return f.Gather(allIndexes(f.rows))
}

func (f Frame) Columns() []Column {
	cols := make([]Column, 0, len(f.schema.names))
	for _, name := range f.schema.names {
		cols = append(cols, Column{Name: name, Data: f.columns[name].Gather(allIndexes(f.rows))})
	}
	return cols
}

func (f Frame) Column(name Symbol) (Array, bool) {
	col, ok := f.columns[name]
	return col, ok
}

func (f Frame) Row(row int) (map[Symbol]any, error) {
	if row < 0 || row >= f.rows {
		return nil, fmt.Errorf("frame row index %d out of range", row)
	}
	out := make(map[Symbol]any, len(f.schema.names))
	for _, name := range f.schema.names {
		v, _ := f.columns[name].At(row)
		out[name] = v
	}
	return out, nil
}

func (f Frame) Gather(indexes []int) (Frame, error) {
	cols := make([]Column, 0, len(f.schema.names))
	for _, name := range f.schema.names {
		cols = append(cols, Column{Name: name, Data: f.columns[name].Gather(indexes)})
	}
	return newFrameTrusted(cols...)
}

func Gather(array Array, indexes []int) (Array, error) {
	if array == nil {
		return nil, fmt.Errorf("gather array is nil")
	}
	if err := validateIndexes(array.Len(), indexes); err != nil {
		return nil, err
	}
	return array.Gather(indexes), nil
}

func Slice(array Array, start, count int) (Array, error) {
	if array == nil {
		return nil, fmt.Errorf("slice array is nil")
	}
	if start < 0 || count < 0 || start > array.Len() || start+count > array.Len() {
		return nil, fmt.Errorf("slice range start=%d count=%d outside length %d", start, count, array.Len())
	}
	if count == 0 {
		return array.Gather(nil), nil
	}
	switch a := array.(type) {
	case attributedArray:
		sliced, err := Slice(a.array, start, count)
		if err != nil {
			return nil, err
		}
		return a.withLazyRebuiltIndexes(sliced), nil
	case tiledArray:
		if a.source.Len() == 0 {
			return array.Gather(nil), nil
		}
		return tiledArray{source: a.source, start: (a.start + start) % a.source.Len(), len: count}, nil
	case i64RangeArray:
		return i64RangeArray{start: a.start + int64(start)*a.step, step: a.step, len: count}, nil
	case i64ScalarDyadicArray:
		// Slice the source view and keep the scalar-dyadic carrier lazy:
		// cut/sublist segments stay O(1) and downstream consumers flatten
		// them through the bulk kernels in one dense pass per segment.
		if a.source != nil && a.source.Len() == a.len {
			source, err := Slice(a.source, start, count)
			if err == nil {
				return i64ScalarDyadicArray{source: source, op: a.op, scalar: a.scalar, scalarLeft: a.scalarLeft, len: count}, nil
			}
		}
		return Gather(array, contiguousIndexes(start, count))
	case i64SegmentArray:
		return sliceI64SegmentArray(a, start, count), nil
	case i64Int32IndexArray:
		return i64Int32IndexArray{rows: a.rows[start : start+count]}, nil
	case f64RangeArray:
		return f64RangeArray{start: a.start + float64(start)*a.step, step: a.step, len: count}, nil
	case columnArray[bool]:
		return columnArray[bool]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[int8]:
		return columnArray[int8]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[int16]:
		return columnArray[int16]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[int32]:
		return columnArray[int32]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[int64]:
		return columnArray[int64]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[uint8]:
		return columnArray[uint8]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[uint16]:
		return columnArray[uint16]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[uint32]:
		return columnArray[uint32]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[uint64]:
		return columnArray[uint64]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[float32]:
		return columnArray[float32]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[float64]:
		return columnArray[float64]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[string]:
		return columnArray[string]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Symbol]:
		return columnArray[Symbol]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Month]:
		return columnArray[Month]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Date]:
		return columnArray[Date]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[DateTime]:
		return columnArray[DateTime]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Timespan]:
		return columnArray[Timespan]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Minute]:
		return columnArray[Minute]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Second]:
		return columnArray[Second]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Time]:
		return columnArray[Time]{kind: a.kind, data: a.data[start : start+count]}, nil
	case columnArray[Timestamp]:
		return columnArray[Timestamp]{kind: a.kind, data: a.data[start : start+count]}, nil
	case nullableArray:
		return nullableArray{kind: a.kind, data: a.data[start : start+count]}, nil
	case nullBitmapCarrier:
		return a.subarray(start, count), nil
	case encodedArray:
		return encodedArray{kind: a.kind, domain: a.domain, codes: a.codes[start : start+count]}, nil
	default:
		return Gather(array, contiguousIndexes(start, count))
	}
}

func Reverse(array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("reverse array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		reversed, handled, err := Reverse(a.array)
		if err != nil || !handled {
			return reversed, handled, err
		}
		return a.withLazyRebuiltIndexes(reversed), true, nil
	case i64RangeArray:
		if a.len == 0 {
			return a, true, nil
		}
		return i64RangeArray{start: a.start + int64(a.len-1)*a.step, step: -a.step, len: a.len}, true, nil
	case f64RangeArray:
		if a.len == 0 {
			return a, true, nil
		}
		return f64RangeArray{start: a.start + float64(a.len-1)*a.step, step: -a.step, len: a.len}, true, nil
	case i64SegmentArray:
		if a.len == 0 {
			return i64RangeArray{len: 0}, true, nil
		}
		segments := make([]i64RangeArray, 0, len(a.segments))
		for i := len(a.segments) - 1; i >= 0; i-- {
			segment := a.segments[i]
			if segment.len <= 0 {
				continue
			}
			segments = append(segments, i64RangeArray{
				start: segment.start + int64(segment.len-1)*segment.step,
				step:  -segment.step,
				len:   segment.len,
			})
		}
		return newI64SegmentArray(segments...), true, nil
	default:
		length := array.Len()
		return indexedArray{
			source:  array,
			indexes: i64RangeArray{start: int64(length - 1), step: -1, len: length},
			len:     length,
		}, true, nil
	}
}

func sliceI64SegmentArray(array i64SegmentArray, start, count int) Array {
	if count <= 0 {
		return i64RangeArray{len: 0}
	}
	out := make([]i64RangeArray, 0, len(array.segments))
	offset := start
	remaining := count
	for _, segment := range array.segments {
		if offset >= segment.len {
			offset -= segment.len
			continue
		}
		take := segment.len - offset
		if take > remaining {
			take = remaining
		}
		out = append(out, i64RangeArray{
			start: segment.start + int64(offset)*segment.step,
			step:  segment.step,
			len:   take,
		})
		remaining -= take
		if remaining == 0 {
			break
		}
		offset = 0
	}
	return newI64SegmentArray(out...)
}

func TakeRepeat(array Array, n int) (Array, error) {
	if array == nil {
		return nil, fmt.Errorf("take array is nil")
	}
	if n == 0 {
		return array.Gather(nil), nil
	}
	length := array.Len()
	if length == 0 {
		return array.Gather(nil), nil
	}
	count := n
	if count < 0 {
		count = -count
	}
	if count > length {
		start := 0
		if n < 0 {
			start = length - count%length
			if start == length {
				start = 0
			}
		}
		return tiledArray{source: array, start: start, len: count}, nil
	}
	if n < 0 {
		return Slice(array, length-count, count)
	}
	return Slice(array, 0, count)
}

func GatherFrame(frame Frame, indexes []int) (Frame, error) {
	if err := validateIndexes(frame.Len(), indexes); err != nil {
		return Frame{}, err
	}
	return frame.Gather(indexes)
}

func EmptyLike(frame Frame) (Frame, error) {
	return frame.Gather(nil)
}

func SelectFrameColumns(frame Frame, names ...Symbol) (Frame, error) {
	if len(names) == 0 {
		return EmptyLike(frame)
	}
	cols := make([]Column, 0, len(names))
	seen := make(map[Symbol]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			return Frame{}, fmt.Errorf("select column name must not be empty")
		}
		if _, ok := seen[name]; ok {
			return Frame{}, fmt.Errorf("select column %q is duplicated", name)
		}
		col, ok := frame.Column(name)
		if !ok {
			return Frame{}, fmt.Errorf("select column %q does not exist", name)
		}
		cols = append(cols, Column{Name: name, Data: col})
		seen[name] = struct{}{}
	}
	return newFrameTrusted(cols...)
}

// ReorderFrameColumns implements q xcols-style projection: requested columns
// are moved to the front and unspecified columns keep their original order.
func ReorderFrameColumns(frame Frame, requested ...Symbol) (Frame, error) {
	order, err := reorderFrameColumnOrder(frame.schema.names, requested)
	if err != nil {
		return Frame{}, err
	}
	return SelectFrameColumns(frame, order...)
}

func reorderFrameColumnOrder(existing []Symbol, requested []Symbol) ([]Symbol, error) {
	seenExisting := make(map[Symbol]struct{}, len(existing))
	for _, name := range existing {
		seenExisting[name] = struct{}{}
	}
	seenRequested := make(map[Symbol]struct{}, len(requested))
	order := make([]Symbol, 0, len(existing))
	for _, name := range requested {
		if name == "" {
			return nil, fmt.Errorf("xcols column name must not be empty")
		}
		if _, ok := seenExisting[name]; !ok {
			return nil, fmt.Errorf("xcols column %q does not exist", name)
		}
		if _, ok := seenRequested[name]; ok {
			return nil, fmt.Errorf("xcols column %q is duplicated", name)
		}
		seenRequested[name] = struct{}{}
		order = append(order, name)
	}
	for _, name := range existing {
		if _, ok := seenRequested[name]; !ok {
			order = append(order, name)
		}
	}
	return order, nil
}

func SameSchema(left, right Frame) bool {
	return left.schema.CompatibleWith(right.schema)
}

func Take(array Array, n int) (Array, error) {
	if array == nil {
		return nil, fmt.Errorf("take array is nil")
	}
	n, err := takeCount(array.Len(), n)
	if err != nil {
		return nil, err
	}
	return takeArray(array, n), nil
}

func TakeFrame(frame Frame, n int) (Frame, error) {
	n, err := takeCount(frame.Len(), n)
	if err != nil {
		return Frame{}, err
	}
	cols := make([]Column, 0, len(frame.schema.names))
	for _, name := range frame.schema.names {
		cols = append(cols, Column{Name: name, Data: takeArray(frame.columns[name], n)})
	}
	return newFrameTrusted(cols...)
}

func WhereMask(mask Array) ([]int, error) {
	if mask == nil {
		return nil, fmt.Errorf("where mask is nil")
	}
	if mask.Kind() != KindBool {
		return nil, fmt.Errorf("where mask kind is %s, want %s", mask.Kind(), KindBool)
	}
	if indexes, handled, err := typedWhereMaskIndexes(mask); handled || err != nil {
		return indexes, err
	}
	indexes := make([]int, 0, mask.Len())
	for i := 0; i < mask.Len(); i++ {
		v, ok := mask.At(i)
		if !ok {
			return nil, fmt.Errorf("where mask row %d out of range", i)
		}
		if IsNull(v) {
			continue
		}
		keep, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("where mask row %d is %T, want bool", i, v)
		}
		if keep {
			indexes = append(indexes, i)
		}
	}
	return indexes, nil
}

// TryTypedWhereMaskI64 converts a boolean mask into q's where index vector
// without routing every row through Array.At and []any boxing.
func TryTypedWhereMaskI64(mask Array) (Array, bool, error) {
	if mask == nil {
		return nil, true, fmt.Errorf("where mask is nil")
	}
	if mask.Kind() != KindBool {
		return nil, true, fmt.Errorf("where mask kind is %s, want %s", mask.Kind(), KindBool)
	}
	if out, handled, err := typedWhereMaskIndexArray(mask); handled || err != nil {
		return out, handled, err
	}
	if out, ok := fusedPredicateWhereIndexArray(mask); ok {
		return out, true, nil
	}
	if values, owned, ok := tryBulkBoolValues(mask); ok {
		count := 0
		for _, keep := range values {
			if keep {
				count++
			}
		}
		out := make([]int64, 0, count)
		for row, keep := range values {
			if keep {
				out = append(out, int64(row))
			}
		}
		bulkBoolRelease(values, owned)
		return newI64Trusted(out), true, nil
	}
	indexes, handled, err := typedWhereMaskIndexes(mask)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	out := make([]int64, len(indexes))
	for i, index := range indexes {
		out[i] = int64(index)
	}
	return newI64Trusted(out), true, nil
}

// arrayCyclePeriod reports the row-cycle period of lazily tiled carriers and
// mask trees built over them: a cyclic take repeats every source-length rows,
// truthiness/not wrappers preserve their operand's cycle, and logical masks
// compose cycles through the lcm. ok=false means no provable cycle.
func arrayCyclePeriod(array Array) (int64, bool) {
	switch a := array.(type) {
	case attributedArray:
		return arrayCyclePeriod(a.array)
	case tiledArray:
		if a.source.Len() <= 0 {
			return 0, false
		}
		return int64(a.source.Len()), true
	case notMask:
		return arrayCyclePeriod(a.array)
	case boolLogicalMask:
		leftPeriod := int64(1)
		if !a.leftIsScalar {
			if a.left.Len() != 1 {
				if a.left.Len() != a.len {
					return 0, false
				}
				period, ok := arrayCyclePeriod(a.left)
				if !ok {
					return 0, false
				}
				leftPeriod = period
			}
		}
		rightPeriod := int64(1)
		if !a.rightIsScalar {
			if a.right.Len() != 1 {
				if a.right.Len() != a.len {
					return 0, false
				}
				period, ok := arrayCyclePeriod(a.right)
				if !ok {
					return 0, false
				}
				rightPeriod = period
			}
		}
		return lcmInt64(leftPeriod, rightPeriod)
	default:
		return 0, false
	}
}

// periodicBoolMaskIndexArray lowers `where mask` for cyclically periodic bool
// masks (tiled pattern columns and logical trees over them) by evaluating one
// cycle and emitting a periodic index carrier instead of scanning every row.
func periodicBoolMaskIndexArray(mask Array) (Array, bool) {
	length := mask.Len()
	period, ok := arrayCyclePeriod(mask)
	if !ok || period <= 0 || period > 65536 || int64(length) <= period {
		return nil, false
	}
	residues := make([]int64, 0, period)
	for row := int64(0); row < period; row++ {
		value, ok := mask.At(int(row))
		if !ok {
			return nil, false
		}
		keep, isBool := value.(bool)
		if !isBool {
			return nil, false
		}
		if keep {
			residues = append(residues, row)
		}
	}
	if len(residues) == 0 {
		return i64RangeArray{len: 0}, true
	}
	return newI64PeriodicIndexArray(period, residues, length), true
}

func typedWhereMaskIndexArray(mask Array) (Array, bool, error) {
	if mask.Kind() == KindBool {
		if out, ok := periodicBoolMaskIndexArray(mask); ok {
			return out, true, nil
		}
	}
	switch a := mask.(type) {
	case attributedArray:
		return typedWhereMaskIndexArray(a.array)
	case columnArray[bool]:
		return boolMaskIndexArray(a.data), true, nil
	case nullableArray:
		return nullableBoolMaskIndexArray(a)
	case i64RangeCompareMask:
		return i64RangeCompareMaskIndexArray(a)
	case i64SegmentCompareMask:
		return i64SegmentCompareMaskIndexArray(a)
	case i64ScalarDyadicCompareMask:
		return i64ScalarDyadicCompareMaskIndexArray(a)
	case i64ArrayCompareMask:
		return i64ArrayCompareMaskIndexArray(a)
	case boolLogicalMask:
		return boolLogicalMaskIndexArray(a)
	default:
		return nil, false, nil
	}
}

func boolMaskIndexArray(mask []bool) Array {
	segments := make([]i64RangeArray, 0)
	for row := 0; row < len(mask); {
		if !mask[row] {
			row++
			continue
		}
		start := row
		row++
		for row < len(mask) && mask[row] {
			row++
		}
		segments = append(segments, i64RangeArray{start: int64(start), step: 1, len: row - start})
	}
	return newI64SegmentArray(segments...)
}

func nullableBoolMaskIndexArray(mask nullableArray) (Array, bool, error) {
	segments := make([]i64RangeArray, 0)
	for row := 0; row < len(mask.data); {
		value := mask.data[row]
		if IsNull(value) {
			row++
			continue
		}
		keep, ok := value.(bool)
		if !ok {
			return nil, true, fmt.Errorf("where mask row %d is %T, want bool", row, value)
		}
		if !keep {
			row++
			continue
		}
		start := row
		row++
		for row < len(mask.data) {
			value = mask.data[row]
			if IsNull(value) {
				break
			}
			keep, ok = value.(bool)
			if !ok {
				return nil, true, fmt.Errorf("where mask row %d is %T, want bool", row, value)
			}
			if !keep {
				break
			}
			row++
		}
		segments = append(segments, i64RangeArray{start: int64(start), step: 1, len: row - start})
	}
	return newI64SegmentArray(segments...), true, nil
}

func boolLogicalMaskIndexArray(mask boolLogicalMask) (Array, bool, error) {
	if out, ok := boolLogicalModuloCompareMaskIndexArray(mask); ok {
		return out, true, nil
	}
	if mask.op != "and" || mask.leftIsScalar || mask.rightIsScalar {
		return nil, false, nil
	}
	left, leftOK := mask.left.(i64RangeCompareMask)
	right, rightOK := mask.right.(i64RangeCompareMask)
	if leftOK && rightOK && sameI64Range(left.values, right.values) && left.values.step == 1 {
		leftLow, leftHigh, ok := compareMaskValueInterval(left)
		if !ok {
			return nil, false, nil
		}
		rightLow, rightHigh, ok := compareMaskValueInterval(right)
		if !ok {
			return nil, false, nil
		}
		return i64RangeIntervalIndexArray(left.values, maxInt64Value(leftLow, rightLow), minInt64Value(leftHigh, rightHigh)), true, nil
	}
	leftSegment, leftSegmentOK := mask.left.(i64SegmentCompareMask)
	rightSegment, rightSegmentOK := mask.right.(i64SegmentCompareMask)
	if !leftSegmentOK || !rightSegmentOK || !sameI64Segment(leftSegment.values, rightSegment.values) {
		return nil, false, nil
	}
	leftLow, leftHigh, ok := compareSegmentMaskValueInterval(leftSegment)
	if !ok {
		return nil, false, nil
	}
	rightLow, rightHigh, ok := compareSegmentMaskValueInterval(rightSegment)
	if !ok {
		return nil, false, nil
	}
	out, ok := i64SegmentIntervalIndexArray(leftSegment.values, maxInt64Value(leftLow, rightLow), minInt64Value(leftHigh, rightHigh))
	return out, ok, nil
}

func i64RangeCompareMaskIndexArray(mask i64RangeCompareMask) (Array, bool, error) {
	if mask.values.step != 1 {
		return nil, false, nil
	}
	low, high, ok := compareMaskValueInterval(mask)
	if !ok {
		return nil, false, nil
	}
	return i64RangeIntervalIndexArray(mask.values, low, high), true, nil
}

func i64SegmentCompareMaskIndexArray(mask i64SegmentCompareMask) (Array, bool, error) {
	low, high, ok := compareSegmentMaskValueInterval(mask)
	if !ok {
		return nil, false, nil
	}
	out, ok := i64SegmentIntervalIndexArray(mask.values, low, high)
	return out, ok, nil
}

func i64ArrayCompareMaskIndexArray(mask i64ArrayCompareMask) (Array, bool, error) {
	values, owned, ok := tryBulkI64Values(mask.source)
	if !ok || len(values) < mask.len {
		bulkI64Release(values, owned)
		return nil, false, nil
	}
	values = values[:mask.len]
	op := effectiveRangeCompareOp(mask.op, mask.scalarLeft)
	count := 0
	for _, value := range values {
		if boolCompare(op, value == mask.scalar, compareInt64(value, mask.scalar)) {
			count++
		}
	}
	out := make([]int64, 0, count)
	for row, value := range values {
		if boolCompare(op, value == mask.scalar, compareInt64(value, mask.scalar)) {
			out = append(out, int64(row))
		}
	}
	bulkI64Release(values, owned)
	return newI64Trusted(out), true, nil
}

func i64ScalarDyadicCompareMaskIndexArray(mask i64ScalarDyadicCompareMask) (Array, bool, error) {
	if indexes, ok := i64ScalarDyadicCompareSegmentModuloIndexes(mask); ok {
		return indexes, true, nil
	}
	plan, ok := i64ScalarDyadicCompareModuloPlan(mask)
	if !ok {
		return nil, false, nil
	}
	return i64ModuloComparePlanIndexArray(plan), true, nil
}

func i64ModuloComparePlanIndexArray(plan i64ModuloComparePlan) Array {
	if plan.length <= 0 {
		return i64RangeArray{len: 0}
	}
	if plan.op == OpEQ {
		return i64ModuloCompareEqualIndexArray(plan, plan.scalar)
	}
	count, ok := plan.trueCount()
	if !ok || count <= 0 {
		return i64RangeArray{len: 0}
	}
	if count == int64(plan.length) {
		return i64RangeArray{start: 0, step: 1, len: plan.length}
	}
	if plan.op == OpNE {
		return i64ModuloCompareNotEqualIndexArray(plan, plan.scalar)
	}
	if plan.modulus <= 65536 {
		residues := make([]int64, 0)
		for row := int64(0); row < plan.modulus; row++ {
			if plan.valueAtRow(int(row)) {
				residues = append(residues, row)
			}
		}
		return newI64PeriodicIndexArray(plan.modulus, residues, plan.length)
	}
	return i64ModuloComparePlanIndexArrayByRuns(plan)
}

func i64ModuloComparePlanIndexArrayByRuns(plan i64ModuloComparePlan) Array {
	segments := make([]i64RangeArray, 0)
	for row := 0; row < plan.length; {
		if !plan.valueAtRow(row) {
			row++
			continue
		}
		start := row
		row++
		for row < plan.length && plan.valueAtRow(row) {
			row++
		}
		segments = append(segments, i64RangeArray{start: int64(start), step: 1, len: row - start})
	}
	return newI64SegmentArray(segments...)
}

func i64ModuloCompareEqualIndexArray(plan i64ModuloComparePlan, target int64) Array {
	if target < 0 || target >= plan.modulus || plan.length <= 0 {
		return i64RangeArray{len: 0}
	}
	first := qPositiveMod(target-plan.startResidue, plan.modulus)
	if first >= int64(plan.length) {
		return i64RangeArray{len: 0}
	}
	length := int((int64(plan.length)-first-1)/plan.modulus) + 1
	return i64RangeArray{start: first, step: plan.modulus, len: length}
}

func i64ModuloCompareNotEqualIndexArray(plan i64ModuloComparePlan, target int64) Array {
	if plan.modulus <= 65536 {
		excluded := i64ModuloCompareEqualIndexArray(i64ModuloComparePlan{
			startResidue: plan.startResidue,
			modulus:      plan.modulus,
			length:       int(plan.modulus),
			op:           OpEQ,
			scalar:       target,
		}, target)
		excludedRange, ok := excluded.(i64RangeArray)
		if !ok || excludedRange.len == 0 {
			return i64RangeArray{start: 0, step: 1, len: plan.length}
		}
		residues := make([]int64, 0, int(plan.modulus)-1)
		excludedRow := excludedRange.start
		for row := int64(0); row < plan.modulus; row++ {
			if row != excludedRow {
				residues = append(residues, row)
			}
		}
		return newI64PeriodicIndexArray(plan.modulus, residues, plan.length)
	}
	equalIndexes := i64ModuloCompareEqualIndexArray(plan, target)
	equalRange, ok := equalIndexes.(i64RangeArray)
	if !ok || equalRange.len == 0 {
		return i64RangeArray{start: 0, step: 1, len: plan.length}
	}
	segments := make([]i64RangeArray, 0, equalRange.len+1)
	next := int64(0)
	for i := 0; i < equalRange.len; i++ {
		row := equalRange.start + int64(i)*equalRange.step
		if row > next {
			segments = append(segments, i64RangeArray{start: next, step: 1, len: int(row - next)})
		}
		next = row + 1
	}
	if next < int64(plan.length) {
		segments = append(segments, i64RangeArray{start: next, step: 1, len: int(int64(plan.length) - next)})
	}
	return newI64SegmentArray(segments...)
}

func boolLogicalModuloCompareMaskIndexArray(mask boolLogicalMask) (Array, bool) {
	plan, ok := boolLogicalModuloComparePlan(mask)
	if !ok {
		return nil, false
	}
	count, ok := plan.trueCount()
	if !ok || count <= 0 {
		return i64RangeArray{len: 0}, true
	}
	if count == int64(plan.length) {
		return i64RangeArray{start: 0, step: 1, len: plan.length}, true
	}
	if plan.period <= 65536 {
		residues := make([]int64, 0)
		for row := int64(0); row < plan.period; row++ {
			left := plan.left.valueAtRow(int(row))
			right := plan.right.valueAtRow(int(row))
			if applyBoolLogical(plan.op, left, right) {
				residues = append(residues, row)
			}
		}
		return newI64PeriodicIndexArray(plan.period, residues, plan.length), true
	}
	return boolLogicalModuloCompareMaskIndexArrayByRuns(plan), true
}

func boolLogicalModuloCompareMaskIndexArrayByRuns(plan boolLogicalModuloPlan) Array {
	segments := make([]i64RangeArray, 0)
	for row := 0; row < plan.length; {
		left := plan.left.valueAtRow(row)
		right := plan.right.valueAtRow(row)
		if !applyBoolLogical(plan.op, left, right) {
			row++
			continue
		}
		start := row
		row++
		for row < plan.length {
			left = plan.left.valueAtRow(row)
			right = plan.right.valueAtRow(row)
			if !applyBoolLogical(plan.op, left, right) {
				break
			}
			row++
		}
		segments = append(segments, i64RangeArray{start: int64(start), step: 1, len: row - start})
	}
	return newI64SegmentArray(segments...)
}

func i64RangeIntervalIndexArray(values i64RangeArray, low, high int64) Array {
	if values.len == 0 || high < low {
		return i64RangeArray{len: 0}
	}
	startRow := low - values.start
	if startRow < 0 {
		startRow = 0
	}
	endRow := high - values.start
	lastRow := int64(values.len - 1)
	if endRow > lastRow {
		endRow = lastRow
	}
	if endRow < startRow {
		return i64RangeArray{len: 0}
	}
	return i64RangeArray{start: startRow, step: 1, len: int(endRow-startRow) + 1}
}

func typedWhereMaskIndexes(mask Array) ([]int, bool, error) {
	switch a := mask.(type) {
	case attributedArray:
		return typedWhereMaskIndexes(a.array)
	case columnArray[bool]:
		count := 0
		for _, keep := range a.data {
			if keep {
				count++
			}
		}
		out := make([]int, count)
		next := 0
		for row, keep := range a.data {
			if keep {
				out[next] = row
				next++
			}
		}
		return out, true, nil
	case nullableArray:
		out := make([]int, 0, len(a.data))
		for row, value := range a.data {
			if IsNull(value) {
				continue
			}
			keep, ok := value.(bool)
			if !ok {
				return nil, true, fmt.Errorf("where mask row %d is %T, want bool", row, value)
			}
			if keep {
				out = append(out, row)
			}
		}
		return out, true, nil
	default:
		if mask.Kind() != KindBool {
			return nil, false, nil
		}
		if values, owned, ok := tryBulkBoolValues(mask); ok {
			count := 0
			for _, keep := range values {
				if keep {
					count++
				}
			}
			out := make([]int, 0, count)
			for row, keep := range values {
				if keep {
					out = append(out, row)
				}
			}
			bulkBoolRelease(values, owned)
			return out, true, nil
		}
		capacity := mask.Len()
		if count, handled, err := TryTypedTrueCount(mask); err != nil {
			return nil, true, err
		} else if handled && count >= 0 && int64(int(count)) == count {
			capacity = int(count)
		}
		out := make([]int, 0, capacity)
		for row := 0; row < mask.Len(); row++ {
			keep, ok, err := boolArrayAt(mask, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			if keep {
				out = append(out, row)
			}
		}
		return out, true, nil
	}
}

// TryTypedCompareIndexesI64 returns q-style row indexes for a typed
// array/scalar comparison without materializing an intermediate boolean mask.
func TryTypedCompareIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("compare array is nil")
	}
	if out, ok := typedCompareIndexRangeI64(array, op, value); ok {
		return out, true, nil
	}
	if out, ok, err := typedCompareContiguousIndexesI64(array, op, value); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := typedCompareSegmentedIndexesI64(array, op, value); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := typedCompareScalarDyadicIndexesI64(array, op, value); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := nullBitmapCompareIndexesI64(array, op, value); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := typedCompareTiledIndexesI64(array, op, value); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := typedCompareCarrierIndexesI64(array, op, value); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := typedCompareBulkIndexesI64(array, op, value); ok || err != nil {
		return out, ok, err
	}
	if out, ok := typedCompareBoolIndexesI64(array, op, value); ok {
		return out, true, nil
	}
	indexes, ok := typedKernels.CompareIndexes(array, op, value, nil)
	if !ok {
		return nil, false, nil
	}
	out := make([]int64, len(indexes))
	for i, index := range indexes {
		out[i] = int64(index)
	}
	return newI64Trusted(out), true, nil
}

func typedCompareBulkIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	if IsNull(value) {
		return nil, false, nil
	}
	if target, ok := coerceInt64Exact(value); ok {
		values, owned, ok := TryBulkI64(array)
		if ok {
			out := make([]int64, 0, len(values))
			for row, item := range values {
				if boolCompare(op, item == target, compareInt64(item, target)) {
					out = append(out, int64(row))
				}
			}
			BulkI64Release(values, owned)
			return newI64Trusted(out), true, nil
		}
	}
	target, ok := numeric(value)
	if !ok {
		return nil, false, nil
	}
	values, owned, ok := TryBulkF64(array)
	if !ok {
		return nil, false, nil
	}
	out := make([]int64, 0, len(values))
	for row, item := range values {
		if boolCompare(op, item == target, compareFloat64(item, target)) {
			out = append(out, int64(row))
		}
	}
	BulkF64Release(values, owned)
	return newI64Trusted(out), true, nil
}

// typedCompareBoolIndexesI64 selects matching rows of a dense bool column in
// two passes (count, then exact-size fill) so the hot boolean where shapes
// allocate the index vector once. The op/target dispatch is hoisted out of
// the row loops: a bool comparison only ever partitions rows into the
// true-keep / false-keep classes, so the per-row work is a single branch.
func typedCompareBoolIndexesI64(array Array, op Op, value any) (Array, bool) {
	a, isBool := unwrapAttributedArray(array).(columnArray[bool])
	if !isBool {
		return nil, false
	}
	target, isBool := value.(bool)
	if !isBool {
		return nil, false
	}
	keepTrue := boolCompare(op, target, compareBool(true, target))
	keepFalse := boolCompare(op, !target, compareBool(false, target))
	switch {
	case keepTrue && keepFalse:
		return i64RangeArray{start: 0, step: 1, len: len(a.data)}, true
	case !keepTrue && !keepFalse:
		return i64RangeArray{len: 0}, true
	}
	trues := 0
	for _, v := range a.data {
		if v {
			trues++
		}
	}
	count := trues
	if keepFalse {
		count = len(a.data) - trues
	}
	out := make([]int64, 0, count)
	if keepTrue {
		for i, v := range a.data {
			if v {
				out = append(out, int64(i))
			}
		}
	} else {
		for i, v := range a.data {
			if !v {
				out = append(out, int64(i))
			}
		}
	}
	return newI64Trusted(out), true
}

// TryTypedCompareIndexStatsI64 returns count and sum of q-style row indexes
// selected by an array/scalar comparison without materializing the index vector.
func TryTypedCompareIndexStatsI64(array Array, op Op, value any) (count, sum int64, handled bool, err error) {
	if array == nil {
		return 0, 0, true, fmt.Errorf("compare array is nil")
	}
	if count, sum, handled, err := nullBitmapCompareIndexStatsI64(array, op, value); handled || err != nil {
		return count, sum, handled, err
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedCompareIndexStatsI64(a.array, op, value)
	case i64RangeArray:
		target, ok := coerceInt64Exact(value)
		if !ok {
			return 0, 0, false, nil
		}
		indexes, ok := compareI64RangeIndexArray(a, op, target)
		if !ok {
			return 0, 0, false, nil
		}
		return int64(indexes.Len()), i64IndexArraySum(indexes), true, nil
	case i64SegmentArray:
		target, ok := coerceInt64Exact(value)
		if !ok {
			return 0, 0, false, nil
		}
		return compareI64SegmentIndexStats(a, op, target)
	case tiledArray:
		return compareTiledIndexStats(a, op, value)
	case i64ScalarDyadicArray:
		return compareScalarDyadicIndexStats(a, op, value)
	case indexedArray:
		// A reversal permutation maps row -> len-1-row, so compare stats can
		// be derived from the source stats: counts match and selected row
		// indexes mirror around the midpoint.
		if isReversalIndexedArray(a) {
			count, sum, handled, err := TryTypedCompareIndexStatsI64(a.source, op, value)
			if err != nil || !handled {
				if err != nil {
					return 0, 0, handled, err
				}
				return typedCompareCarrierIndexStatsI64(a, op, value)
			}
			return count, count*int64(a.len-1) - sum, true, nil
		}
		return typedCompareCarrierIndexStatsI64(a, op, value)
	case nullableArray:
		return typedCompareCarrierIndexStatsI64(a, op, value)
	default:
		return typedCompareIndexStatsI64(a, op, value)
	}
}

// isReversalIndexedArray reports whether array is a full-length descending
// reindex of its source (the canonical `reverse` carrier).
func isReversalIndexedArray(array indexedArray) bool {
	indexes, ok := array.indexes.(i64RangeArray)
	if !ok {
		return false
	}
	return indexes.len == array.len &&
		array.source.Len() == array.len &&
		indexes.step == -1 &&
		indexes.start == int64(array.len-1)
}

// TryTypedCompareCount returns the number of rows selected by an array/scalar
// comparison without materializing a mask or index vector.
func TryTypedCompareCount(array Array, op Op, value any) (count int64, handled bool, err error) {
	if array == nil {
		return 0, true, fmt.Errorf("compare array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedCompareCount(a.array, op, value)
	case tiledArray:
		return compareTiledCount(a, op, value)
	default:
		count, _, handled, err := TryTypedCompareIndexStatsI64(a, op, value)
		if handled || err != nil {
			return count, handled, err
		}
		if count, ok := compareCountBulk(a, op, value); ok {
			return count, true, nil
		}
		return 0, false, nil
	}
}

// TryTypedWithinIndexesI64 returns q-style row indexes selected by a typed
// within comparison without materializing a boolean mask.
func TryTypedWithinIndexesI64(array Array, low, high any, highClosed bool) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("within array is nil")
	}
	if IsNull(low) || IsNull(high) {
		return i64RangeArray{len: 0}, true, nil
	}
	low = normalizeScalar(array.Kind(), low)
	high = normalizeScalar(array.Kind(), high)
	if out, handled, err := nullBitmapWithinIndexesI64(array, low, high, highClosed); handled || err != nil {
		return out, handled, err
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedWithinIndexesI64(a.array, low, high, highClosed)
	case indexedArray:
		return typedWithinIndexedIndexesI64(a, low, high, highClosed)
	case i64RangeArray:
		lowI, lowOK := coerceInt64Exact(low)
		highI, highOK := coerceInt64Exact(high)
		if !lowOK || !highOK {
			return nil, false, nil
		}
		out := make([]int64, 0)
		for row := 0; row < a.len; row++ {
			value := a.start + int64(row)*a.step
			if value < lowI || value > highI || (!highClosed && value == highI) {
				continue
			}
			out = append(out, int64(row))
		}
		return newI64Trusted(out), true, nil
	case tiledArray:
		residues, handled, err := typedWithinTiledResidues(a, low, high, highClosed)
		if err != nil || !handled {
			return nil, handled, err
		}
		return newI64PeriodicIndexArray(int64(a.source.Len()), residues, a.len), true, nil
	case nullableArray:
		return typedWithinNullableIndexesI64(a, low, high, highClosed)
	default:
		indexes, ok := typedKernels.WithinIndexes(a, low, high, highClosed, nil)
		if !ok {
			return nil, false, nil
		}
		out := make([]int64, len(indexes))
		for i, index := range indexes {
			out[i] = int64(index)
		}
		return newI64Trusted(out), true, nil
	}
}

// TryTypedWithinIndexStatsI64 returns count and sum of q-style row indexes
// selected by a typed within comparison without materializing indexes.
func TryTypedWithinIndexStatsI64(array Array, low, high any, highClosed bool) (count, sum int64, handled bool, err error) {
	if array == nil {
		return 0, 0, true, fmt.Errorf("within array is nil")
	}
	if IsNull(low) || IsNull(high) {
		return 0, 0, true, nil
	}
	low = normalizeScalar(array.Kind(), low)
	high = normalizeScalar(array.Kind(), high)
	switch a := array.(type) {
	case attributedArray:
		return TryTypedWithinIndexStatsI64(a.array, low, high, highClosed)
	case indexedArray:
		// Reversal permutations preserve the selected multiset; row indexes
		// mirror around the midpoint (see TryTypedCompareIndexStatsI64).
		if isReversalIndexedArray(a) {
			count, sum, handled, err := TryTypedWithinIndexStatsI64(a.source, low, high, highClosed)
			if err == nil && handled {
				return count, count*int64(a.len-1) - sum, true, nil
			}
			if err != nil {
				return 0, 0, handled, err
			}
		}
		return typedWithinIndexedIndexStatsI64(a, low, high, highClosed)
	case i64BucketArray:
		return withinI64BucketIndexStats(a, low, high, highClosed)
	case f64BucketArray:
		return withinF64BucketIndexStats(a, low, high, highClosed)
	case tiledArray:
		residues, handled, err := typedWithinTiledResidues(a, low, high, highClosed)
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		indexes := newI64PeriodicIndexArray(int64(a.source.Len()), residues, a.len)
		return int64(indexes.Len()), i64IndexArraySum(indexes), true, nil
	default:
		indexes, handled, err := TryTypedWithinIndexesI64(a, low, high, highClosed)
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		return int64(indexes.Len()), i64IndexArraySum(indexes), true, nil
	}
}

func typedWithinIndexedIndexesI64(array indexedArray, low, high any, highClosed bool) (Array, bool, error) {
	if array.len == 0 {
		return i64RangeArray{len: 0}, true, nil
	}
	out := make([]int64, 0)
	for row := 0; row < array.len; row++ {
		value, ok := array.At(row)
		if !ok {
			return nil, true, fmt.Errorf("within indexed row %d out of range", row)
		}
		if typedWithinScalar(value, low, high, highClosed) {
			out = append(out, int64(row))
		}
	}
	return newI64Trusted(out), true, nil
}

func typedWithinIndexedIndexStatsI64(array indexedArray, low, high any, highClosed bool) (count, sum int64, handled bool, err error) {
	for row := 0; row < array.len; row++ {
		value, ok := array.At(row)
		if !ok {
			return 0, 0, true, fmt.Errorf("within indexed row %d out of range", row)
		}
		if typedWithinScalar(value, low, high, highClosed) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func typedWithinScalar(value, low, high any, highClosed bool) bool {
	if IsNull(value) || IsNull(low) || IsNull(high) {
		return false
	}
	if compare(value, low) < 0 {
		return false
	}
	cmpHigh := compare(value, high)
	return cmpHigh < 0 || (highClosed && cmpHigh == 0)
}

func withinI64BucketIndexStats(array i64BucketArray, low, high any, highClosed bool) (count, sum int64, handled bool, err error) {
	lowI, lowOK := coerceInt64Exact(low)
	highI, highOK := coerceInt64Exact(high)
	if !lowOK || !highOK {
		return 0, 0, false, nil
	}
	if highI < lowI || (!highClosed && highI == lowI) {
		return 0, 0, true, nil
	}
	if count, sum, ok := withinI64BucketRangeStepOneIndexStats(array, lowI, highI, highClosed); ok {
		return count, sum, true, nil
	}
	for row := 0; row < array.len; row++ {
		value, ok, err := array.i64At(row)
		if err != nil {
			return 0, 0, true, err
		}
		if !ok {
			continue
		}
		if value < lowI || value > highI || (!highClosed && value == highI) {
			continue
		}
		count++
		sum += int64(row)
	}
	return count, sum, true, nil
}

func withinI64BucketRangeStepOneIndexStats(array i64BucketArray, low, high int64, highClosed bool) (count, sum int64, ok bool) {
	source, ok := array.source.(i64RangeArray)
	if !ok || source.step != 1 || source.len != array.len || array.width <= 0 {
		return 0, 0, false
	}
	kLow := ceilDivInt64(low, array.width)
	var kHigh int64
	if highClosed {
		kHigh = floorDivInt64(high, array.width)
	} else {
		if high == math.MinInt64 {
			return 0, 0, true
		}
		kHigh = floorDivInt64(high-1, array.width)
	}
	if kHigh < kLow {
		return 0, 0, true
	}
	valueLow := kLow * array.width
	valueHigh := (kHigh+1)*array.width - 1
	indexes := i64RangeIntervalIndexArray(source, valueLow, valueHigh)
	return int64(indexes.Len()), i64IndexArraySum(indexes), true
}

func ceilDivInt64(value, width int64) int64 {
	quotient := value / width
	remainder := value % width
	if remainder != 0 && value > 0 {
		quotient++
	}
	return quotient
}

func withinF64BucketIndexStats(array f64BucketArray, low, high any, highClosed bool) (count, sum int64, handled bool, err error) {
	lowF, lowOK := numeric(low)
	highF, highOK := numeric(high)
	if !lowOK || !highOK {
		return 0, 0, false, nil
	}
	if math.IsNaN(lowF) || math.IsNaN(highF) || highF < lowF || (!highClosed && highF == lowF) {
		return 0, 0, true, nil
	}
	for row := 0; row < array.len; row++ {
		value, ok, err := array.f64At(row)
		if err != nil {
			return 0, 0, true, err
		}
		if !ok {
			continue
		}
		if value < lowF || value > highF || (!highClosed && value == highF) {
			continue
		}
		count++
		sum += int64(row)
	}
	return count, sum, true, nil
}

// TryTypedWithinCount returns the number of rows selected by a typed within
// comparison without materializing a mask or index vector.
func TryTypedWithinCount(array Array, low, high any, highClosed bool) (count int64, handled bool, err error) {
	count, _, handled, err = TryTypedWithinIndexStatsI64(array, low, high, highClosed)
	return count, handled, err
}

func typedCompareScalarDyadicIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	values, ok := array.(i64ScalarDyadicArray)
	if !ok {
		if attributed, ok := array.(attributedArray); ok {
			return typedCompareScalarDyadicIndexesI64(attributed.array, op, value)
		}
		return nil, false, nil
	}
	target, ok := coerceInt64Exact(value)
	if !ok {
		return nil, false, nil
	}
	// Periodic modulo carriers compare to a closed-form index plan (range,
	// periodic, or segment view) without materializing an index vector.
	if out, ok, err := i64ScalarDyadicCompareMaskIndexArray(i64ScalarDyadicCompareMask{values: values, op: op, scalar: target}); ok || err != nil {
		return out, ok, err
	}
	if out, ok := analyticWhereMask(i64ScalarDyadicCompareMask{values: values, op: op, scalar: target}); ok {
		return out, true, nil
	}
	if flat, owned, ok := TryBulkI64(values); ok && len(flat) == values.len {
		out := make([]int64, 0, len(flat))
		for row, item := range flat {
			if boolCompare(op, item == target, compareInt64(item, target)) {
				out = append(out, int64(row))
			}
		}
		BulkI64Release(flat, owned)
		return newI64Trusted(out), true, nil
	}
	out := make([]int64, 0)
	for row := 0; row < values.len; row++ {
		item, ok, err := values.i64At(row)
		if err != nil || !ok {
			return nil, ok, err
		}
		if boolCompare(op, item == target, compareInt64(item, target)) {
			out = append(out, int64(row))
		}
	}
	return newI64Trusted(out), true, nil
}

func typedWithinTiledResidues(array tiledArray, low, high any, highClosed bool) ([]int64, bool, error) {
	period := array.source.Len()
	if period == 0 || array.len == 0 {
		return nil, true, nil
	}
	sourceMask := make([]bool, period)
	low = normalizeScalar(array.source.Kind(), low)
	high = normalizeScalar(array.source.Kind(), high)
	if ok := typedKernels.WithinMask(array.source, low, high, highClosed, sourceMask); !ok {
		sourceIndexes, handled, err := TryTypedWithinIndexesI64(array.source, low, high, highClosed)
		if err != nil || !handled {
			return nil, handled, err
		}
		residues := make([]int64, 0, sourceIndexes.Len())
		for row := 0; row < sourceIndexes.Len(); row++ {
			value, ok := sourceIndexes.At(row)
			if !ok {
				return nil, true, fmt.Errorf("within tiled source index row %d out of range", row)
			}
			sourceRow, ok := coerceInt64Exact(value)
			if !ok {
				return nil, false, nil
			}
			residues = append(residues, int64(positiveModInt(int(sourceRow)-array.start, period)))
		}
		return residues, true, nil
	}
	residues := make([]int64, 0)
	for sourceRow, keep := range sourceMask {
		if keep {
			residues = append(residues, int64(positiveModInt(sourceRow-array.start, period)))
		}
	}
	return residues, true, nil
}

func typedWithinNullableIndexesI64(array nullableArray, low, high any, highClosed bool) (Array, bool, error) {
	out := make([]int64, 0)
	for row, value := range array.data {
		if IsNull(value) {
			continue
		}
		lowCmp, ok := compareSameKind(value, low)
		if !ok {
			return nil, false, nil
		}
		highCmp, ok := compareSameKind(value, high)
		if !ok {
			return nil, false, nil
		}
		if lowCmp >= 0 && (highCmp < 0 || (highClosed && highCmp == 0)) {
			out = append(out, int64(row))
		}
	}
	return newI64Trusted(out), true, nil
}

func typedCompareCarrierIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	if !typedCompareCarrierCanHandle(array) {
		return nil, false, nil
	}
	target := normalizeScalar(array.Kind(), value)
	if !typedCompareCarrierTargetCompatible(array.Kind(), target) {
		return nil, false, nil
	}
	if view, ok := unwrapAttributedArray(array).(indexedArray); ok {
		if out, handled, err := typedCompareIndexedDenseIndexesI64(view, op, target); handled || err != nil {
			return out, handled, err
		}
	}
	out := make([]int64, 0)
	for row := 0; row < array.Len(); row++ {
		keep, handled, err := typedCompareCarrierRow(array, row, op, target)
		if err != nil || !handled {
			return nil, handled, err
		}
		if keep {
			out = append(out, int64(row))
		}
	}
	return newI64Trusted(out), true, nil
}

// typedCompareIndexedDenseIndexesI64 short-circuits the boxed carrier row
// loop for the canonical result-view carrier: an index view over a dense
// i64/f64 column (the shape subquery and projection stages produce). Rows
// resolve through the typed index vector and compare with the same
// comparators as the dense CompareIndexes kernels; out-of-range source rows
// surface the same error the boxed At walk would.
func typedCompareIndexedDenseIndexesI64(view indexedArray, op Op, target any) (Array, bool, error) {
	idx, owned, ok := tryBulkI64Values(view.indexes)
	if !ok {
		return nil, false, nil
	}
	if len(idx) != view.len {
		bulkI64Release(idx, owned)
		return nil, false, nil
	}
	var out []int64
	var handled bool
	var err error
	switch src := unwrapAttributedArray(view.source).(type) {
	case columnArray[float64]:
		t, tok := numeric(target)
		if !tok {
			bulkI64Release(idx, owned)
			return nil, false, nil
		}
		out, handled, err = indexedDenseCompareIndexesI64(src.data, idx, op, t)
	case columnArray[int64]:
		t, tok := coerceInt64Exact(target)
		if !tok {
			bulkI64Release(idx, owned)
			return nil, false, nil
		}
		out, handled, err = indexedDenseCompareIndexesI64(src.data, idx, op, t)
	default:
		bulkI64Release(idx, owned)
		return nil, false, nil
	}
	bulkI64Release(idx, owned)
	if err != nil || !handled {
		return nil, handled && err != nil, err
	}
	return newI64Trusted(out), true, nil
}

// indexedDenseCompareIndexesI64 runs the count+exact-fill compare over a
// dense source resolved through a typed index vector. The op dispatch is
// hoisted out of the row loops; LE/GE use the negated strict forms so float
// NaN rows keep the same boolCompare(compareFloat64)==0 incomparable
// semantics as the boxed carrier walk (NaN keeps for LE/GE, drops for
// LT/GT, standard IEEE behavior for EQ/NE).
func indexedDenseCompareIndexesI64[T int64 | float64](data []T, idx []int64, op Op, t T) ([]int64, bool, error) {
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return nil, false, nil
	}
	bound := int64(len(data))
	for _, row := range idx {
		if row < 0 || row >= bound {
			return nil, true, fmt.Errorf("compare row %d out of range", row)
		}
	}
	count := 0
	switch op {
	case OpEQ:
		for _, row := range idx {
			if data[row] == t {
				count++
			}
		}
	case OpNE:
		for _, row := range idx {
			if data[row] != t {
				count++
			}
		}
	case OpLT:
		for _, row := range idx {
			if data[row] < t {
				count++
			}
		}
	case OpLE:
		for _, row := range idx {
			if !(data[row] > t) {
				count++
			}
		}
	case OpGT:
		for _, row := range idx {
			if data[row] > t {
				count++
			}
		}
	case OpGE:
		for _, row := range idx {
			if !(data[row] < t) {
				count++
			}
		}
	}
	out := make([]int64, 0, count)
	switch op {
	case OpEQ:
		for i, row := range idx {
			if data[row] == t {
				out = append(out, int64(i))
			}
		}
	case OpNE:
		for i, row := range idx {
			if data[row] != t {
				out = append(out, int64(i))
			}
		}
	case OpLT:
		for i, row := range idx {
			if data[row] < t {
				out = append(out, int64(i))
			}
		}
	case OpLE:
		for i, row := range idx {
			if !(data[row] > t) {
				out = append(out, int64(i))
			}
		}
	case OpGT:
		for i, row := range idx {
			if data[row] > t {
				out = append(out, int64(i))
			}
		}
	case OpGE:
		for i, row := range idx {
			if !(data[row] < t) {
				out = append(out, int64(i))
			}
		}
	}
	return out, true, nil
}

func typedCompareCarrierIndexStatsI64(array Array, op Op, value any) (count, sum int64, handled bool, err error) {
	if !typedCompareCarrierCanHandle(array) {
		return 0, 0, false, nil
	}
	target := normalizeScalar(array.Kind(), value)
	if !typedCompareCarrierTargetCompatible(array.Kind(), target) {
		return 0, 0, false, nil
	}
	for row := 0; row < array.Len(); row++ {
		keep, handled, err := typedCompareCarrierRow(array, row, op, target)
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		if keep {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func typedCompareCarrierCanHandle(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return typedCompareCarrierCanHandle(a.array)
	case indexedArray, nullableArray:
		return typedCompareKindCanHandle(array.Kind())
	default:
		return false
	}
}

func typedCompareKindCanHandle(kind Kind) bool {
	switch kind {
	case KindBool,
		KindI8, KindI16, KindI32, KindI64,
		KindU8, KindU16, KindU32, KindU64,
		KindF32, KindF64,
		KindString, KindSymbol,
		KindMonth, KindDate, KindDateTime, KindTimespan,
		KindMinute, KindSecond, KindTime, KindTimestamp:
		return true
	default:
		return false
	}
}

func typedCompareCarrierTargetCompatible(kind Kind, value any) bool {
	if IsNull(value) {
		return true
	}
	switch kind {
	case KindF64:
		_, ok := numeric(value)
		return ok
	case KindString:
		_, ok := coerceComparableString(value)
		return ok
	case KindSymbol:
		_, ok := coerceComparableSymbol(value)
		return ok
	default:
		_, err := NormalizeValueForKind(kind, value)
		return err == nil
	}
}

func typedCompareCarrierRow(array Array, row int, op Op, target any) (bool, bool, error) {
	value, ok := array.At(row)
	if !ok {
		return false, true, fmt.Errorf("compare row %d out of range", row)
	}
	if IsNull(value) || IsNull(target) {
		equal := IsNull(value) && IsNull(target)
		switch op {
		case OpEQ, OpNE:
			return boolCompare(op, equal, 0), true, nil
		default:
			return false, true, nil
		}
	}
	cmp, ok := compareSameKind(value, target)
	if !ok {
		return false, false, nil
	}
	return boolCompare(op, cmp == 0, cmp), true, nil
}

func compareScalarDyadicIndexStats(array i64ScalarDyadicArray, op Op, value any) (count, sum int64, handled bool, err error) {
	target, ok := coerceInt64Exact(value)
	if !ok {
		return 0, 0, false, nil
	}
	// Flatten the lazy dyadic carrier once and compare over the dense slice:
	// tryBulkI64ScalarDyadicValues produces exactly the i64At row values, so
	// count/sum are identical to the per-row walk below.
	if values, owned, ok := tryBulkI64ScalarDyadicValues(array); ok {
		for row, item := range values {
			if boolCompare(op, item == target, compareInt64(item, target)) {
				count++
				sum += int64(row)
			}
		}
		bulkI64Release(values, owned)
		return count, sum, true, nil
	}
	for row := 0; row < array.len; row++ {
		item, ok, err := array.i64At(row)
		if err != nil || !ok {
			return 0, 0, ok, err
		}
		if boolCompare(op, item == target, compareInt64(item, target)) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func typedCompareTiledIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	tiled, ok := array.(tiledArray)
	if !ok {
		if attributed, ok := array.(attributedArray); ok {
			return typedCompareTiledIndexesI64(attributed.array, op, value)
		}
		return nil, false, nil
	}
	if !typedCompareTiledCanHandle(tiled.source, op, value) {
		return nil, false, nil
	}
	if tiled.source.Len() == 0 || tiled.len == 0 {
		return i64RangeArray{len: 0}, true, nil
	}
	out := make([]int64, 0)
	for row := 0; row < tiled.len; row++ {
		sourceRow := (tiled.start + row) % tiled.source.Len()
		sourceValue, ok := tiled.source.At(sourceRow)
		if !ok {
			return nil, true, fmt.Errorf("tiled compare source row %d out of range", sourceRow)
		}
		if compareTiledValue(sourceValue, op, value) {
			out = append(out, int64(row))
		}
	}
	return newI64Trusted(out), true, nil
}

func compareTiledIndexStats(array tiledArray, op Op, value any) (count, sum int64, handled bool, err error) {
	if !typedCompareTiledCanHandle(array.source, op, value) {
		return 0, 0, false, nil
	}
	if array.source.Len() == 0 || array.len == 0 {
		return 0, 0, true, nil
	}
	if count, sum, ok := compareTiledIndexStatsI64(array, op, value); ok {
		return count, sum, true, nil
	}
	period := array.source.Len()
	for sourceRow := 0; sourceRow < period; sourceRow++ {
		sourceValue, ok := array.source.At(sourceRow)
		if !ok {
			return 0, 0, true, fmt.Errorf("tiled compare source row %d out of range", sourceRow)
		}
		if !compareTiledValue(sourceValue, op, value) {
			continue
		}
		first := positiveModInt(sourceRow-array.start, period)
		if first >= array.len {
			continue
		}
		hits := int64(1 + (array.len-1-first)/period)
		count += hits
		sum += hits*int64(first) + int64(period)*hits*(hits-1)/2
	}
	return count, sum, true, nil
}

func compareTiledCount(array tiledArray, op Op, value any) (count int64, handled bool, err error) {
	if !typedCompareTiledCanHandle(array.source, op, value) {
		return 0, false, nil
	}
	if array.source.Len() == 0 || array.len == 0 {
		return 0, true, nil
	}
	if count, ok := compareTiledCountI64(array, op, value); ok {
		return count, true, nil
	}
	period := array.source.Len()
	for sourceRow := 0; sourceRow < period; sourceRow++ {
		sourceValue, ok := array.source.At(sourceRow)
		if !ok {
			return 0, true, fmt.Errorf("tiled compare source row %d out of range", sourceRow)
		}
		if !compareTiledValue(sourceValue, op, value) {
			continue
		}
		first := positiveModInt(sourceRow-array.start, period)
		if first >= array.len {
			continue
		}
		count += int64(1 + (array.len-1-first)/period)
	}
	return count, true, nil
}

func compareTiledIndexStatsI64(array tiledArray, op Op, value any) (count, sum int64, ok bool) {
	values, owned, bulkOK := tryBulkI64Values(array.source)
	if !bulkOK || len(values) != array.source.Len() {
		bulkI64Release(values, owned)
		return 0, 0, false
	}
	target, exact := coerceInt64Exact(value)
	if !exact {
		bulkI64Release(values, owned)
		return 0, 0, false
	}
	period := len(values)
	for sourceRow, sourceValue := range values {
		if !boolCompare(op, sourceValue == target, compareInt64(sourceValue, target)) {
			continue
		}
		first := positiveModInt(sourceRow-array.start, period)
		if first >= array.len {
			continue
		}
		hits := int64(1 + (array.len-1-first)/period)
		count += hits
		sum += hits*int64(first) + int64(period)*hits*(hits-1)/2
	}
	bulkI64Release(values, owned)
	return count, sum, true
}

func compareTiledCountI64(array tiledArray, op Op, value any) (int64, bool) {
	values, owned, bulkOK := tryBulkI64Values(array.source)
	if !bulkOK || len(values) != array.source.Len() {
		bulkI64Release(values, owned)
		return 0, false
	}
	target, exact := coerceInt64Exact(value)
	if !exact {
		bulkI64Release(values, owned)
		return 0, false
	}
	period := len(values)
	var count int64
	for sourceRow, sourceValue := range values {
		if !boolCompare(op, sourceValue == target, compareInt64(sourceValue, target)) {
			continue
		}
		first := positiveModInt(sourceRow-array.start, period)
		if first >= array.len {
			continue
		}
		count += int64(1 + (array.len-1-first)/period)
	}
	bulkI64Release(values, owned)
	return count, true
}

func positiveModInt(value, modulus int) int {
	out := value % modulus
	if out < 0 {
		out += modulus
	}
	return out
}

func typedCompareTiledCanHandle(source Array, op Op, value any) bool {
	switch s := source.(type) {
	case attributedArray:
		return typedCompareTiledCanHandle(s.array, op, value)
	case nullableArray:
		if IsNull(value) {
			return true
		}
		target := normalizeScalar(s.Kind(), value)
		return typedCompareCarrierTargetCompatible(s.Kind(), target)
	case nullBitmapArray[int8]:
		return IsNull(value) || typedCompareCarrierTargetCompatible(s.Kind(), normalizeScalar(s.Kind(), value))
	case nullBitmapArray[int16]:
		return IsNull(value) || typedCompareCarrierTargetCompatible(s.Kind(), normalizeScalar(s.Kind(), value))
	case nullBitmapArray[int32]:
		return IsNull(value) || typedCompareCarrierTargetCompatible(s.Kind(), normalizeScalar(s.Kind(), value))
	case nullBitmapArray[int64]:
		return IsNull(value) || typedCompareCarrierTargetCompatible(s.Kind(), normalizeScalar(s.Kind(), value))
	case nullBitmapArray[float32]:
		return IsNull(value) || typedCompareCarrierTargetCompatible(s.Kind(), normalizeScalar(s.Kind(), value))
	case nullBitmapArray[float64]:
		return IsNull(value) || typedCompareCarrierTargetCompatible(s.Kind(), normalizeScalar(s.Kind(), value))
	case columnArray[bool]:
		_, ok := value.(bool)
		return ok
	case columnArray[int8]:
		_, ok := value.(int8)
		return ok
	case columnArray[int16]:
		_, ok := value.(int16)
		return ok
	case columnArray[int32]:
		_, ok := value.(int32)
		return ok
	case columnArray[int64], i64RangeArray:
		_, ok := coerceInt64Exact(value)
		return ok
	case columnArray[uint8]:
		_, ok := value.(uint8)
		return ok
	case columnArray[uint16]:
		_, ok := value.(uint16)
		return ok
	case columnArray[uint32]:
		_, ok := value.(uint32)
		return ok
	case columnArray[uint64]:
		_, ok := value.(uint64)
		return ok
	case columnArray[float32]:
		_, ok := value.(float32)
		return ok
	case columnArray[float64]:
		_, ok := numeric(value)
		return ok
	case columnArray[string]:
		_, ok := coerceComparableString(value)
		return ok
	case columnArray[Symbol]:
		_, ok := coerceComparableSymbol(value)
		return ok
	case columnArray[Month]:
		_, ok := value.(Month)
		return ok
	case columnArray[Date]:
		_, ok := value.(Date)
		return ok
	case columnArray[DateTime]:
		_, ok := value.(DateTime)
		return ok
	case columnArray[Timespan]:
		_, ok := value.(Timespan)
		return ok
	case columnArray[Minute]:
		_, ok := value.(Minute)
		return ok
	case columnArray[Second]:
		_, ok := value.(Second)
		return ok
	case columnArray[Time]:
		_, ok := value.(Time)
		return ok
	case columnArray[Timestamp]:
		_, ok := value.(Timestamp)
		return ok
	default:
		return false
	}
}

func compareTiledValue(left any, op Op, right any) bool {
	if IsNull(left) || IsNull(right) {
		eq := IsNull(left) && IsNull(right)
		switch op {
		case OpEQ:
			return eq
		case OpNE:
			return !eq
		default:
			return false
		}
	}
	cmp := compare(left, right)
	return boolCompare(op, cmp == 0, cmp)
}

func typedCompareIndexStatsI64(array Array, op Op, value any) (count, sum int64, handled bool, err error) {
	switch a := array.(type) {
	case columnArray[bool]:
		v, ok := value.(bool)
		return compareBoolIndexStats(a.data, v, ok, op)
	case columnArray[int8]:
		v, ok := value.(int8)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[int16]:
		v, ok := value.(int16)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[int32]:
		v, ok := value.(int32)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[int64]:
		v, ok := coerceInt64Exact(value)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[uint8]:
		v, ok := value.(uint8)
		return compareUnsignedIndexStats(a.data, v, ok, op)
	case columnArray[uint16]:
		v, ok := value.(uint16)
		return compareUnsignedIndexStats(a.data, v, ok, op)
	case columnArray[uint32]:
		v, ok := value.(uint32)
		return compareUnsignedIndexStats(a.data, v, ok, op)
	case columnArray[uint64]:
		v, ok := value.(uint64)
		return compareUnsignedIndexStats(a.data, v, ok, op)
	case columnArray[float32]:
		v, ok := value.(float32)
		return compareFloatIndexStats(a.data, v, ok, op)
	case columnArray[float64]:
		v, ok := numeric(value)
		return compareFloatIndexStats(a.data, v, ok, op)
	case columnArray[string]:
		v, ok := coerceComparableString(value)
		return compareStringIndexStats(a.data, v, ok, op)
	case columnArray[Symbol]:
		v, ok := coerceComparableSymbol(value)
		return compareSymbolIndexStats(a.data, v, ok, op)
	case columnArray[Month]:
		v, ok := value.(Month)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[Date]:
		v, ok := value.(Date)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[DateTime]:
		v, ok := value.(DateTime)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[Timespan]:
		v, ok := value.(Timespan)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[Minute]:
		v, ok := value.(Minute)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[Second]:
		v, ok := value.(Second)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[Time]:
		v, ok := value.(Time)
		return compareSignedIndexStats(a.data, v, ok, op)
	case columnArray[Timestamp]:
		v, ok := value.(Timestamp)
		return compareSignedIndexStats(a.data, v, ok, op)
	default:
		return 0, 0, false, nil
	}
}

func compareBoolIndexStats(values []bool, target bool, ok bool, op Op) (count, sum int64, handled bool, err error) {
	if !ok {
		return 0, 0, false, nil
	}
	for row, value := range values {
		if boolCompare(op, value == target, compareBool(value, target)) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func compareSignedIndexStats[T signedScalar](values []T, target T, ok bool, op Op) (count, sum int64, handled bool, err error) {
	if !ok {
		return 0, 0, false, nil
	}
	for row, value := range values {
		if boolCompare(op, value == target, compareInt64(int64(value), int64(target))) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func compareUnsignedIndexStats[T unsignedScalar](values []T, target T, ok bool, op Op) (count, sum int64, handled bool, err error) {
	if !ok {
		return 0, 0, false, nil
	}
	for row, value := range values {
		if boolCompare(op, value == target, compareUint64(uint64(value), uint64(target))) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func compareFloatIndexStats[T floatScalar](values []T, target T, ok bool, op Op) (count, sum int64, handled bool, err error) {
	if !ok {
		return 0, 0, false, nil
	}
	for row, value := range values {
		if boolCompare(op, value == target, compareFloat64(float64(value), float64(target))) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func compareStringIndexStats(values []string, target string, ok bool, op Op) (count, sum int64, handled bool, err error) {
	if !ok {
		return 0, 0, false, nil
	}
	for row, value := range values {
		if boolCompare(op, value == target, compareString(value, target)) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func compareSymbolIndexStats(values []Symbol, target Symbol, ok bool, op Op) (count, sum int64, handled bool, err error) {
	if !ok {
		return 0, 0, false, nil
	}
	for row, value := range values {
		if boolCompare(op, value == target, compareString(string(value), string(target))) {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

func i64IndexArraySum(array Array) int64 {
	switch a := array.(type) {
	case i64RangeArray:
		return i64RangeSum(a)
	case i64SegmentArray:
		return i64SegmentSum(a)
	case i64PeriodicIndexArray:
		return i64PeriodicIndexSum(a)
	default:
		var total int64
		for row := 0; row < array.Len(); row++ {
			value, ok := array.At(row)
			if !ok {
				return 0
			}
			n, ok := coerceInt64Exact(value)
			if !ok {
				return 0
			}
			total += n
		}
		return total
	}
}

// TryGatherByI64IndexArray gathers array rows using a typed i64 index vector
// without boxing the index array through Values().
func TryGatherByI64IndexArray(array Array, indexes Array) (Array, bool, error) {
	if array == nil || indexes == nil {
		return nil, true, fmt.Errorf("gather array and indexes must be non-nil")
	}
	if indexes.Kind() != KindI64 {
		return nil, true, fmt.Errorf("index vector kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if out, ok, err := tryGatherRangeByI64IndexArray(array, indexes); ok || err != nil {
		return out, ok, err
	}
	if ok, err := validateI64IndexArray(indexes, array.Len()); err != nil || ok {
		if err != nil {
			return nil, true, err
		}
		return indexedArray{source: array, indexes: indexes, len: indexes.Len()}, true, nil
	}
	rows, handled, err := TryTypedI64Indexes(indexes)
	if err != nil || !handled {
		return nil, handled, err
	}
	out, err := Gather(array, rows)
	return out, true, err
}

func tryGatherRangeByI64IndexArray(array Array, indexes Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		out, handled, err := tryGatherRangeByI64IndexArray(a.array, indexes)
		if err != nil || !handled {
			return out, handled, err
		}
		return a.withLazyRebuiltIndexes(out), true, nil
	case i64RangeArray:
		switch idx := indexes.(type) {
		case attributedArray:
			return tryGatherRangeByI64IndexArray(array, idx.array)
		case i64RangeArray:
			if err := validateI64IndexRange(idx, a.len); err != nil {
				return nil, true, err
			}
			return i64RangeArray{
				start: a.start + idx.start*a.step,
				step:  idx.step * a.step,
				len:   idx.len,
			}, true, nil
		case i64SegmentArray:
			out := make([]i64RangeArray, len(idx.segments))
			for i, segment := range idx.segments {
				if err := validateI64IndexRange(segment, a.len); err != nil {
					return nil, true, err
				}
				out[i] = i64RangeArray{
					start: a.start + segment.start*a.step,
					step:  segment.step * a.step,
					len:   segment.len,
				}
			}
			return newI64SegmentArray(out...), true, nil
		default:
			return nil, false, nil
		}
	case i64SegmentArray:
		switch idx := indexes.(type) {
		case attributedArray:
			return tryGatherRangeByI64IndexArray(array, idx.array)
		case i64RangeArray:
			if err := validateI64IndexRange(idx, a.len); err != nil {
				return nil, true, err
			}
			return gatherI64SegmentByI64Range(a, idx), true, nil
		case i64SegmentArray:
			out := make([]i64RangeArray, 0, len(idx.segments))
			for _, segment := range idx.segments {
				if err := validateI64IndexRange(segment, a.len); err != nil {
					return nil, true, err
				}
				gathered := gatherI64SegmentByI64Range(a, segment)
				if gatheredSegment, ok := gathered.(i64SegmentArray); ok {
					out = append(out, gatheredSegment.segments...)
				} else if gatheredRange, ok := gathered.(i64RangeArray); ok && gatheredRange.len > 0 {
					out = append(out, gatheredRange)
				}
			}
			return newI64SegmentArray(out...), true, nil
		default:
			return nil, false, nil
		}
	default:
		if idx, ok := unwrapI64RangeIndex(indexes); ok && idx.step == 1 {
			if err := validateI64IndexRange(idx, array.Len()); err != nil {
				return nil, true, err
			}
			start, err := checkedI64Index(idx.start)
			if err != nil {
				return nil, true, err
			}
			out, err := Slice(array, start, idx.len)
			return out, true, err
		}
		return nil, false, nil
	}
}

func gatherI64SegmentByI64Range(array i64SegmentArray, indexes i64RangeArray) Array {
	switch indexes.len {
	case 0:
		return i64RangeArray{len: 0}
	case 1:
		value, _ := array.i64At(int(indexes.start))
		return i64RangeArray{start: value, step: 1, len: 1}
	}
	first, _ := array.i64At(int(indexes.start))
	secondIndex := indexes.start + indexes.step
	second, _ := array.i64At(int(secondIndex))
	step := second - first
	prev := second
	for i := 2; i < indexes.len; i++ {
		row := indexes.start + int64(i)*indexes.step
		current, _ := array.i64At(int(row))
		if current-prev != step {
			if indexes.step == 1 {
				start, _ := checkedI64Index(indexes.start)
				return sliceI64SegmentArray(array, start, indexes.len)
			}
			values := make([]int64, indexes.len)
			values[0] = first
			values[1] = second
			for j := 2; j < i; j++ {
				values[j], _ = array.i64At(int(indexes.start + int64(j)*indexes.step))
			}
			values[i] = current
			for j := i + 1; j < indexes.len; j++ {
				values[j], _ = array.i64At(int(indexes.start + int64(j)*indexes.step))
			}
			return newI64Trusted(values)
		}
		prev = current
	}
	return i64RangeArray{start: first, step: step, len: indexes.len}
}

func unwrapI64RangeIndex(indexes Array) (i64RangeArray, bool) {
	switch idx := indexes.(type) {
	case attributedArray:
		return unwrapI64RangeIndex(idx.array)
	case i64RangeArray:
		return idx, true
	default:
		return i64RangeArray{}, false
	}
}

func validateI64IndexRange(indexes i64RangeArray, length int) error {
	if indexes.len == 0 {
		return nil
	}
	first := indexes.start
	last := indexes.start + int64(indexes.len-1)*indexes.step
	lo, hi := first, last
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < 0 || hi >= int64(length) {
		return fmt.Errorf("index range %d..%d out of bounds for length %d", lo, hi, length)
	}
	return nil
}

func validateI64IndexArray(indexes Array, length int) (bool, error) {
	switch idx := indexes.(type) {
	case attributedArray:
		return validateI64IndexArray(idx.array, length)
	case i64RangeArray:
		if err := validateI64IndexRange(idx, length); err != nil {
			return true, err
		}
		return true, nil
	case i64SegmentArray:
		for _, segment := range idx.segments {
			if err := validateI64IndexRange(segment, length); err != nil {
				return true, err
			}
		}
		return true, nil
	case i64Int32IndexArray:
		for i, value := range idx.rows {
			if value < 0 || int(value) >= length {
				return true, fmt.Errorf("index vector row %d value %d outside length %d", i, value, length)
			}
		}
		return true, nil
	case i64PeriodicIndexArray:
		// Periodic where-results are monotone increasing (sorted residues,
		// strictly increasing cycle bases), so checking the first and last
		// rows bounds the whole vector in O(1).
		if idx.len == 0 {
			return true, nil
		}
		first, okFirst := idx.i64At(0)
		last, okLast := idx.i64At(idx.len - 1)
		if !okFirst || !okLast {
			return true, fmt.Errorf("index vector row %d out of range", idx.len-1)
		}
		if first < 0 {
			return true, fmt.Errorf("index vector row %d value %d outside length %d", 0, first, length)
		}
		if last >= int64(length) {
			return true, fmt.Errorf("index vector row %d value %d outside length %d", idx.len-1, last, length)
		}
		return true, nil
	case intIndexArray:
		for i, value := range idx.rows {
			if value < 0 || value >= length {
				return true, fmt.Errorf("index vector row %d value %d outside length %d", i, value, length)
			}
		}
		return true, nil
	case columnArray[int64]:
		for i, value := range idx.data {
			if value < 0 || value >= int64(length) {
				return true, fmt.Errorf("index vector row %d value %d outside length %d", i, value, length)
			}
		}
		return true, nil
	default:
		return false, nil
	}
}

func i64IndexArrayAt(indexes Array, row int) (int, bool, error) {
	switch idx := indexes.(type) {
	case attributedArray:
		return i64IndexArrayAt(idx.array, row)
	case i64RangeArray:
		if row < 0 || row >= idx.len {
			return 0, false, nil
		}
		out, err := checkedI64Index(idx.start + int64(row)*idx.step)
		return out, err == nil, err
	case i64SegmentArray:
		value, ok := idx.i64At(row)
		if !ok {
			return 0, false, nil
		}
		out, err := checkedI64Index(value)
		return out, err == nil, err
	case i64Int32IndexArray:
		if row < 0 || row >= len(idx.rows) {
			return 0, false, nil
		}
		value := idx.rows[row]
		if value < 0 {
			return 0, false, fmt.Errorf("index vector must contain non-negative integers")
		}
		return int(value), true, nil
	case i64PeriodicIndexArray:
		value, ok := idx.i64At(row)
		if !ok {
			return 0, false, nil
		}
		out, err := checkedI64Index(value)
		return out, err == nil, err
	case intIndexArray:
		if row < 0 || row >= len(idx.rows) {
			return 0, false, nil
		}
		return idx.rows[row], true, nil
	case columnArray[int64]:
		if row < 0 || row >= len(idx.data) {
			return 0, false, nil
		}
		out, err := checkedI64Index(idx.data[row])
		return out, err == nil, err
	default:
		value, ok := indexes.At(row)
		if !ok {
			return 0, false, nil
		}
		n, ok := coerceInt64Exact(value)
		if !ok {
			return 0, false, fmt.Errorf("index vector row %d is %T, want i64", row, value)
		}
		out, err := checkedI64Index(n)
		return out, err == nil, err
	}
}

// TryTypedI64Indexes converts typed i64 index arrays to []int without boxing.
func TryTypedI64Indexes(indexes Array) ([]int, bool, error) {
	if indexes == nil {
		return nil, true, fmt.Errorf("index vector is nil")
	}
	if indexes.Kind() != KindI64 {
		return nil, true, fmt.Errorf("index vector kind is %s, want %s", indexes.Kind(), KindI64)
	}
	switch idx := indexes.(type) {
	case attributedArray:
		return TryTypedI64Indexes(idx.array)
	case i64RangeArray:
		out := make([]int, idx.len)
		for i := range out {
			value := idx.start + int64(i)*idx.step
			n, err := checkedI64Index(value)
			if err != nil {
				return nil, true, err
			}
			out[i] = n
		}
		return out, true, nil
	case i64SegmentArray:
		out := make([]int, idx.len)
		next := 0
		for _, segment := range idx.segments {
			for i := 0; i < segment.len; i++ {
				value := segment.start + int64(i)*segment.step
				n, err := checkedI64Index(value)
				if err != nil {
					return nil, true, err
				}
				out[next] = n
				next++
			}
		}
		return out, true, nil
	case i64PeriodicIndexArray:
		out := make([]int, idx.Len())
		for i := range out {
			value, ok := idx.i64At(i)
			if !ok {
				return nil, true, fmt.Errorf("index vector row %d is out of range", i)
			}
			n, err := checkedI64Index(value)
			if err != nil {
				return nil, true, err
			}
			out[i] = n
		}
		return out, true, nil
	case intIndexArray:
		return idx.rows, true, nil
	case i64Int32IndexArray:
		out := make([]int, len(idx.rows))
		for i, value := range idx.rows {
			if value < 0 {
				return nil, true, fmt.Errorf("index vector must contain non-negative integers")
			}
			out[i] = int(value)
		}
		return out, true, nil
	case columnArray[int64]:
		out := make([]int, len(idx.data))
		for i, value := range idx.data {
			n, err := checkedI64Index(value)
			if err != nil {
				return nil, true, err
			}
			out[i] = n
		}
		return out, true, nil
	case nullableArray:
		out := make([]int, len(idx.data))
		for i, value := range idx.data {
			n64, ok := value.(int64)
			if !ok {
				return nil, true, fmt.Errorf("index vector row %d is %T, want i64", i, value)
			}
			n, err := checkedI64Index(n64)
			if err != nil {
				return nil, true, err
			}
			out[i] = n
		}
		return out, true, nil
	default:
		return nil, false, nil
	}
}

func checkedI64Index(value int64) (int, error) {
	if value < 0 || int64(int(value)) != value {
		return 0, fmt.Errorf("index vector must contain non-negative integers")
	}
	return int(value), nil
}

// TryTypedAmendIndexes applies indexed updates without materializing the source
// array as boxed []any values.
func TryTypedAmendIndexes(array Array, indexes []int, values []any) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("amend array is nil")
	}
	if len(indexes) != len(values) {
		return nil, true, fmt.Errorf("amend value length mismatch")
	}
	switch array.Kind() {
	case KindI64:
		if out, ok, err := tryTypedSparseI64Amend(array, indexes, values); ok || err != nil {
			return out, ok, err
		}
		out := make([]int64, array.Len())
		for row := range out {
			value, ok, err := integerArrayAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[row] = value
		}
		for i, index := range indexes {
			if index < 0 || index >= len(out) {
				return nil, true, fmt.Errorf("amend index %d out of range", index)
			}
			value, err := normalizeScalarForKind(KindI64, values[i], i)
			if err != nil {
				return nil, true, err
			}
			next, ok := value.(int64)
			if !ok {
				return nil, false, nil
			}
			out[index] = next
		}
		return columnArray[int64]{kind: KindI64, data: out}, true, nil
	default:
		return nil, false, nil
	}
}

// amendI64SourceCopy materializes the amend source into a fresh dense []int64,
// using the bulk flatteners when the carrier supports them.
func amendI64SourceCopy(array Array) ([]int64, bool, error) {
	out := make([]int64, array.Len())
	if isZeroLikeI64Array(array) {
		return out, true, nil
	}
	if values, owned, ok := TryBulkI64(array); ok && len(values) == len(out) {
		copy(out, values)
		BulkI64Release(values, owned)
		return out, true, nil
	}
	for row := range out {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		out[row] = value
	}
	return out, true, nil
}

// amendF64SourceCopy is the float64 sibling of amendI64SourceCopy.
func amendF64SourceCopy(array Array) ([]float64, bool, error) {
	out := make([]float64, array.Len())
	if values, owned, ok := TryBulkF64(array); ok && len(values) == len(out) {
		copy(out, values)
		BulkF64Release(values, owned)
		return out, true, nil
	}
	for row := range out {
		value, ok, err := typedAmendAddF64ArrayAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		out[row] = value
	}
	return out, true, nil
}

// TryTypedAmendAddIndexes applies @[array; indexes; +; values] as a typed
// indexed accumulation. Repeated indexes are accumulated in q order.
func TryTypedAmendAddIndexes(array Array, indexes []int, values any) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("amend array is nil")
	}
	switch array.Kind() {
	case KindI64:
		if out, ok, err := tryTypedSparseI64AmendAdd(array, indexes, values); ok || err != nil {
			return out, ok, err
		}
		out, ok, err := amendI64SourceCopy(array)
		if err != nil || !ok {
			return nil, ok, err
		}
		for row, index := range indexes {
			if index < 0 || index >= len(out) {
				return nil, true, fmt.Errorf("amend index %d out of range", index)
			}
			value, ok, err := typedAmendAddI64ValueAt(values, row, len(indexes))
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[index] += value
		}
		return newI64Trusted(out), true, nil
	case KindF64:
		out, ok, err := amendF64SourceCopy(array)
		if err != nil || !ok {
			return nil, ok, err
		}
		for row, index := range indexes {
			if index < 0 || index >= len(out) {
				return nil, true, fmt.Errorf("amend index %d out of range", index)
			}
			value, ok, err := typedAmendAddF64ValueAt(values, row, len(indexes))
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[index] += value
		}
		return newF64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedAmendAddIndexArray applies @[array; indexArray; +; values] without
// first materializing the index vector as a Go []int.
func TryTypedAmendAddIndexArray(array Array, indexes Array, values any) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("amend array is nil")
	}
	if indexes == nil {
		return nil, true, fmt.Errorf("amend indexes are nil")
	}
	switch array.Kind() {
	case KindI64:
		if idx, ok := trySmallAmendIndexes(indexes); ok {
			if out, handled, err := tryTypedSparseI64AmendAdd(array, idx, values); handled || err != nil {
				return out, handled, err
			}
		}
		if out, handled, err := tryBulkI64AmendAddIndexArray(array, indexes, values); handled || err != nil {
			return out, handled, err
		}
		out, ok, err := amendI64SourceCopy(array)
		if err != nil || !ok {
			return nil, ok, err
		}
		for row := 0; row < indexes.Len(); row++ {
			index, ok, err := i64IndexArrayAt(indexes, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			if index < 0 || index >= len(out) {
				return nil, true, fmt.Errorf("amend index %d out of range", index)
			}
			value, ok, err := typedAmendAddI64ValueAt(values, row, indexes.Len())
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[index] += value
		}
		return newI64Trusted(out), true, nil
	case KindF64:
		out, ok, err := amendF64SourceCopy(array)
		if err != nil || !ok {
			return nil, ok, err
		}
		for row := 0; row < indexes.Len(); row++ {
			index, ok, err := i64IndexArrayAt(indexes, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			if index < 0 || index >= len(out) {
				return nil, true, fmt.Errorf("amend index %d out of range", index)
			}
			value, ok, err := typedAmendAddF64ValueAt(values, row, indexes.Len())
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[index] += value
		}
		return newF64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

// tryBulkI64AmendAddIndexArray applies @[array; indexes; +; values] with the
// index and value vectors flattened once through the bulk kernels, replacing
// the per-row interface-dispatch loop for dense integer carriers (where-index
// vectors, periodic index carriers, lazy dyadic value chains).
func tryBulkI64AmendAddIndexArray(array Array, indexes Array, values any) (Array, bool, error) {
	valueArray, ok := values.(Array)
	if !ok || indexes.Len() <= 1 || valueArray.Len() != indexes.Len() {
		return nil, false, nil
	}
	idx, idxOwned, ok := tryBulkI64Values(indexes)
	if !ok {
		return nil, false, nil
	}
	defer bulkI64Release(idx, idxOwned)
	if len(idx) < indexes.Len() {
		return nil, false, nil
	}
	idx = idx[:indexes.Len()]
	vals, valsOwned, ok := tryBulkI64Values(valueArray)
	if !ok {
		return nil, false, nil
	}
	defer bulkI64Release(vals, valsOwned)
	if len(vals) < len(idx) {
		return nil, false, nil
	}
	out, ok, err := amendI64SourceCopy(array)
	if err != nil || !ok {
		return nil, ok, err
	}
	for row, index := range idx {
		if index < 0 || index >= int64(len(out)) {
			return nil, true, fmt.Errorf("amend index %d out of range", index)
		}
		out[index] += vals[row]
	}
	return newI64Trusted(out), true, nil
}

func isZeroLikeI64Array(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return isZeroLikeI64Array(a.array)
	case columnArray[int64]:
		return len(a.data) == 1 && a.data[0] == 0
	case columnArray[int32]:
		return len(a.data) == 1 && a.data[0] == 0
	case columnArray[int16]:
		return len(a.data) == 1 && a.data[0] == 0
	case columnArray[int8]:
		return len(a.data) == 1 && a.data[0] == 0
	case columnArray[uint64]:
		return len(a.data) == 1 && a.data[0] == 0
	case columnArray[uint32]:
		return len(a.data) == 1 && a.data[0] == 0
	case columnArray[uint16]:
		return len(a.data) == 1 && a.data[0] == 0
	case columnArray[uint8]:
		return len(a.data) == 1 && a.data[0] == 0
	case i64RangeArray:
		return a.start == 0 && a.step == 0
	case tiledArray:
		return isZeroLikeI64Array(a.source)
	case i64FillArray:
		return a.fill == 0 && isZeroLikeI64Array(a.source)
	default:
		return false
	}
}

const maxSparseAmendAddIndexes = 256

// trySmallAmendIndexes lowers an index vector to []int when it is small
// enough for the sparse amend representation.
func trySmallAmendIndexes(indexes Array) ([]int, bool) {
	n := indexes.Len()
	if n == 0 || n > maxSparseAmendAddIndexes {
		return nil, false
	}
	out := make([]int, n)
	for row := 0; row < n; row++ {
		value, ok, err := i64IndexArrayAt(indexes, row)
		if err != nil || !ok {
			return nil, false
		}
		out[row] = value
	}
	return out, true
}

func tryTypedSparseI64AmendAdd(array Array, indexes []int, values any) (Array, bool, error) {
	if !isDenseIntegerArray(array) || len(indexes) == 0 || len(indexes) > maxSparseAmendAddIndexes || len(indexes)*4 > array.Len() {
		return nil, false, nil
	}
	nextValues := make([]int64, len(indexes))
	prev := -1
	for i, index := range indexes {
		if index < 0 || index >= array.Len() {
			return nil, true, fmt.Errorf("amend index %d out of range", index)
		}
		if index <= prev {
			return nil, false, nil
		}
		prev = index
		old, ok, err := integerArrayAt(array, index)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		add, ok, err := typedAmendAddI64ValueAt(values, i, len(indexes))
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		nextValues[i] = old + add
	}
	return i64SparseAmendArray{
		source:  array,
		indexes: append([]int(nil), indexes...),
		values:  nextValues,
	}, true, nil
}

func typedAmendAddI64ValueAt(values any, row int, count int) (int64, bool, error) {
	if count == 1 {
		value, err := normalizeScalarForKind(KindI64, values, row)
		if err != nil {
			return 0, true, err
		}
		next, ok := value.(int64)
		return next, ok, nil
	}
	array, ok := values.(Array)
	if !ok {
		return 0, false, nil
	}
	if array.Len() != count {
		return 0, true, fmt.Errorf("amend value length mismatch")
	}
	return integerArrayAt(array, row)
}

func typedAmendAddF64ValueAt(values any, row int, count int) (float64, bool, error) {
	if count == 1 {
		value, err := normalizeScalarForKind(KindF64, values, row)
		if err != nil {
			return 0, true, err
		}
		next, ok := value.(float64)
		return next, ok, nil
	}
	array, ok := values.(Array)
	if !ok {
		return 0, false, nil
	}
	if array.Len() != count {
		return 0, true, fmt.Errorf("amend value length mismatch")
	}
	return typedAmendAddF64ArrayAt(array, row)
}

func typedAmendAddF64ArrayAt(array Array, row int) (float64, bool, error) {
	value, ok := array.At(row)
	if !ok {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	if IsNull(value) {
		return 0, false, nil
	}
	next, ok := numeric(value)
	if !ok {
		return 0, false, nil
	}
	return next, true, nil
}

func tryTypedSparseI64Amend(array Array, indexes []int, values []any) (Array, bool, error) {
	if !isDenseIntegerArray(array) || len(indexes) == 0 || len(indexes)*4 > array.Len() {
		return nil, false, nil
	}
	nextValues := make([]int64, len(values))
	prev := -1
	for i, index := range indexes {
		if index < 0 || index >= array.Len() {
			return nil, true, fmt.Errorf("amend index %d out of range", index)
		}
		if index <= prev {
			return nil, false, nil
		}
		prev = index
		value, err := normalizeScalarForKind(KindI64, values[i], i)
		if err != nil {
			return nil, true, err
		}
		next, ok := value.(int64)
		if !ok {
			return nil, false, nil
		}
		nextValues[i] = next
	}
	return i64SparseAmendArray{
		source:  array,
		indexes: append([]int(nil), indexes...),
		values:  nextValues,
	}, true, nil
}

func typedCompareIndexRangeI64(array Array, op Op, value any) (Array, bool) {
	switch a := array.(type) {
	case attributedArray:
		return typedCompareIndexRangeI64(a.array, op, value)
	case i64RangeArray:
		target, ok := coerceInt64Exact(value)
		if !ok {
			return nil, false
		}
		return compareI64RangeIndexArray(a, op, target)
	case i64SegmentArray:
		target, ok := coerceInt64Exact(value)
		if !ok {
			return nil, false
		}
		return compareI64SegmentIndexArray(a, op, target)
	default:
		return nil, false
	}
}

func typedCompareContiguousIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedCompareContiguousIndexesI64(a.array, op, value)
	case columnArray[int64]:
		target, ok := coerceInt64Exact(value)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Month]:
		target, ok := value.(Month)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Date]:
		target, ok := value.(Date)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[DateTime]:
		target, ok := value.(DateTime)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Timespan]:
		target, ok := value.(Timespan)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Minute]:
		target, ok := value.(Minute)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Second]:
		target, ok := value.(Second)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Time]:
		target, ok := value.(Time)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Timestamp]:
		target, ok := value.(Timestamp)
		return compareContiguousSignedIndexesI64(a.data, target, ok, op)
	default:
		return nil, false, nil
	}
}

func typedCompareSegmentedIndexesI64(array Array, op Op, value any) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedCompareSegmentedIndexesI64(a.array, op, value)
	case columnArray[int64]:
		target, ok := coerceInt64Exact(value)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Month]:
		target, ok := value.(Month)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Date]:
		target, ok := value.(Date)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[DateTime]:
		target, ok := value.(DateTime)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Timespan]:
		target, ok := value.(Timespan)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Minute]:
		target, ok := value.(Minute)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Second]:
		target, ok := value.(Second)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Time]:
		target, ok := value.(Time)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	case columnArray[Timestamp]:
		target, ok := value.(Timestamp)
		return compareSegmentedSignedIndexesI64(a.data, target, ok, op)
	default:
		return nil, false, nil
	}
}

func compareContiguousSignedIndexesI64[T signedScalar](values []T, target T, ok bool, op Op) (Array, bool, error) {
	if !ok {
		return nil, false, nil
	}
	start := -1
	end := -1
	closed := false
	for row, value := range values {
		keep := boolCompare(op, int64(value) == int64(target), compareInt64(int64(value), int64(target)))
		if keep {
			if closed {
				return nil, false, nil
			}
			if start < 0 {
				start = row
			}
			end = row
			continue
		}
		if start >= 0 {
			closed = true
		}
	}
	if start < 0 {
		return i64RangeArray{len: 0}, true, nil
	}
	return i64RangeArray{start: int64(start), step: 1, len: end - start + 1}, true, nil
}

func compareSegmentedSignedIndexesI64[T signedScalar](values []T, target T, ok bool, op Op) (Array, bool, error) {
	if !ok {
		return nil, false, nil
	}
	segments := make([]i64RangeArray, 0, 8)
	start := -1
	matches := 0
	for row, value := range values {
		keep := boolCompare(op, int64(value) == int64(target), compareInt64(int64(value), int64(target)))
		if keep {
			matches++
			if start < 0 {
				start = row
			}
			continue
		}
		if start >= 0 {
			segments = append(segments, i64RangeArray{start: int64(start), step: 1, len: row - start})
			start = -1
		}
	}
	if start >= 0 {
		segments = append(segments, i64RangeArray{start: int64(start), step: 1, len: len(values) - start})
	}
	if matches == 0 {
		return i64RangeArray{len: 0}, true, nil
	}
	if len(segments) <= 1 {
		return nil, false, nil
	}
	if len(segments)*3 > matches {
		return nil, false, nil
	}
	return newI64SegmentArray(segments...), true, nil
}

func compareI64RangeIndexArray(values i64RangeArray, op Op, target int64) (Array, bool) {
	if values.len == 0 {
		return i64RangeArray{len: 0}, true
	}
	if values.step == 0 {
		keep := boolCompare(op, values.start == target, compareInt64(values.start, target))
		if keep {
			return i64RangeArray{start: 0, step: 1, len: values.len}, true
		}
		return i64RangeArray{len: 0}, true
	}
	if !i64RangeIsMonotonic(values) {
		return compareI64RangeIndexArraySlow(values, op, target)
	}
	if values.step > 0 {
		return compareAscendingI64RangeIndexes(values, op, target)
	}
	return compareDescendingI64RangeIndexes(values, op, target)
}

func compareI64SegmentIndexArray(values i64SegmentArray, op Op, target int64) (Array, bool) {
	if values.len == 0 {
		return i64RangeArray{len: 0}, true
	}
	out := make([]i64RangeArray, 0, len(values.segments))
	offset := int64(0)
	for _, segment := range values.segments {
		indexes, ok := compareI64RangeIndexArray(segment, op, target)
		if !ok {
			return nil, false
		}
		if translated, ok := translateI64IndexArray(indexes, offset); ok {
			out = append(out, translated...)
		} else {
			return nil, false
		}
		offset += int64(segment.len)
	}
	return newI64SegmentArray(out...), true
}

func compareI64SegmentIndexStats(values i64SegmentArray, op Op, target int64) (count, sum int64, handled bool, err error) {
	indexes, ok := compareI64SegmentIndexArray(values, op, target)
	if !ok {
		return 0, 0, false, nil
	}
	return int64(indexes.Len()), i64IndexArraySum(indexes), true, nil
}

func i64SegmentIntervalIndexArray(values i64SegmentArray, low, high int64) (Array, bool) {
	if values.len == 0 || high < low {
		return i64RangeArray{len: 0}, true
	}
	out := make([]i64RangeArray, 0, len(values.segments))
	offset := int64(0)
	for _, segment := range values.segments {
		rows, ok := i64RangeIntervalRowsForAnyStep(segment, low, high)
		if !ok {
			return nil, false
		}
		if rows.len > 0 {
			rows.start += offset
			out = append(out, rows)
		}
		offset += int64(segment.len)
	}
	return newI64SegmentArray(out...), true
}

func i64RangeIntervalRowsForAnyStep(values i64RangeArray, low, high int64) (i64RangeArray, bool) {
	ge, ok := compareI64RangeIndexArray(values, OpGE, low)
	if !ok {
		return i64RangeArray{}, false
	}
	le, ok := compareI64RangeIndexArray(values, OpLE, high)
	if !ok {
		return i64RangeArray{}, false
	}
	left, leftOK := ge.(i64RangeArray)
	right, rightOK := le.(i64RangeArray)
	if !leftOK || !rightOK || left.step != 1 || right.step != 1 {
		return i64RangeArray{}, false
	}
	start := maxInt64Value(left.start, right.start)
	end := minInt64Value(left.start+int64(left.len)-1, right.start+int64(right.len)-1)
	if left.len == 0 || right.len == 0 || end < start {
		return i64RangeArray{len: 0}, true
	}
	return i64RangeArray{start: start, step: 1, len: int(end-start) + 1}, true
}

func translateI64IndexArray(indexes Array, offset int64) ([]i64RangeArray, bool) {
	switch idx := indexes.(type) {
	case i64RangeArray:
		if idx.len == 0 {
			return nil, true
		}
		return []i64RangeArray{{start: idx.start + offset, step: idx.step, len: idx.len}}, true
	case i64SegmentArray:
		out := make([]i64RangeArray, 0, len(idx.segments))
		for _, segment := range idx.segments {
			if segment.len <= 0 {
				continue
			}
			out = append(out, i64RangeArray{start: segment.start + offset, step: segment.step, len: segment.len})
		}
		return out, true
	default:
		return nil, false
	}
}

func compareI64RangeIndexArraySlow(values i64RangeArray, op Op, target int64) (Array, bool) {
	start := -1
	end := -1
	for row := 0; row < values.len; row++ {
		v := values.start + int64(row)*values.step
		if boolCompare(op, v == target, compareInt64(v, target)) {
			if start < 0 {
				start = row
			}
			end = row
		} else if start >= 0 {
			break
		}
	}
	if start < 0 {
		return i64RangeArray{len: 0}, true
	}
	if op == OpNE && end-start+1 != values.len {
		return nil, false
	}
	return i64RangeArray{start: int64(start), step: 1, len: end - start + 1}, true
}

func i64RangeIsMonotonic(values i64RangeArray) bool {
	if values.len <= 1 || values.step == 0 {
		return true
	}
	offset, ok := checkedI64Mul(int64(values.len-1), values.step)
	if !ok {
		return false
	}
	last, ok := checkedI64Add(values.start, offset)
	if !ok {
		return false
	}
	if values.step > 0 {
		return last >= values.start
	}
	return last <= values.start
}

func checkedI64Mul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == math.MinInt64 && b == -1) || (a == -1 && b == math.MinInt64) {
		return 0, false
	}
	out := a * b
	if out/b != a {
		return 0, false
	}
	return out, true
}

func checkedI64Add(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, false
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, false
	}
	return a + b, true
}

func compareAscendingI64RangeIndexes(values i64RangeArray, op Op, target int64) (Array, bool) {
	switch op {
	case OpEQ:
		row, ok := i64RangeRowOf(values, target)
		if !ok {
			return i64RangeArray{len: 0}, true
		}
		return i64RangeArray{start: int64(row), step: 1, len: 1}, true
	case OpNE:
		if _, ok := i64RangeRowOf(values, target); ok {
			return nil, false
		}
		return i64RangeArray{start: 0, step: 1, len: values.len}, true
	case OpLT:
		end := firstI64RangeRow(values, func(v int64) bool { return v >= target })
		return i64RangeArray{start: 0, step: 1, len: end}, true
	case OpLE:
		end := firstI64RangeRow(values, func(v int64) bool { return v > target })
		return i64RangeArray{start: 0, step: 1, len: end}, true
	case OpGT:
		start := firstI64RangeRow(values, func(v int64) bool { return v > target })
		return i64RangeArray{start: int64(start), step: 1, len: values.len - start}, true
	case OpGE:
		start := firstI64RangeRow(values, func(v int64) bool { return v >= target })
		return i64RangeArray{start: int64(start), step: 1, len: values.len - start}, true
	default:
		return nil, false
	}
}

func compareDescendingI64RangeIndexes(values i64RangeArray, op Op, target int64) (Array, bool) {
	switch op {
	case OpEQ:
		row, ok := i64RangeRowOf(values, target)
		if !ok {
			return i64RangeArray{len: 0}, true
		}
		return i64RangeArray{start: int64(row), step: 1, len: 1}, true
	case OpNE:
		if _, ok := i64RangeRowOf(values, target); ok {
			return nil, false
		}
		return i64RangeArray{start: 0, step: 1, len: values.len}, true
	case OpLT:
		start := firstI64RangeRow(values, func(v int64) bool { return v < target })
		return i64RangeArray{start: int64(start), step: 1, len: values.len - start}, true
	case OpLE:
		start := firstI64RangeRow(values, func(v int64) bool { return v <= target })
		return i64RangeArray{start: int64(start), step: 1, len: values.len - start}, true
	case OpGT:
		end := firstI64RangeRow(values, func(v int64) bool { return v <= target })
		return i64RangeArray{start: 0, step: 1, len: end}, true
	case OpGE:
		end := firstI64RangeRow(values, func(v int64) bool { return v < target })
		return i64RangeArray{start: 0, step: 1, len: end}, true
	default:
		return nil, false
	}
}

func firstI64RangeRow(values i64RangeArray, pred func(int64) bool) int {
	lo, hi := 0, values.len
	for lo < hi {
		mid := lo + (hi-lo)/2
		value := values.start + int64(mid)*values.step
		if pred(value) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func i64RangeRowOf(values i64RangeArray, target int64) (int, bool) {
	delta := target - values.start
	if delta%values.step != 0 {
		return 0, false
	}
	row := delta / values.step
	if row < 0 || row >= int64(values.len) {
		return 0, false
	}
	return int(row), true
}

func EqualMask(array Array, value any) (Array, error) {
	return compareMask(array, OpEQ, value)
}

func CompareMask(array Array, op Op, value any) (Array, error) {
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
	default:
		return nil, fmt.Errorf("compare mask unsupported operator %s", op)
	}
	return compareMask(array, op, value)
}

func WithinMask(array Array, low, high any, highClosed bool) (Array, error) {
	if array == nil {
		return nil, fmt.Errorf("within mask array is nil")
	}
	out := make([]bool, array.Len())
	if IsNull(low) || IsNull(high) {
		return newBoolTrusted(out), nil
	}
	low = normalizeScalar(array.Kind(), low)
	high = normalizeScalar(array.Kind(), high)
	if mask, ok := lazyWithinMask(array, low, high, highClosed); ok {
		return mask, nil
	}
	if ok := withinMaskTyped(array, low, high, highClosed, out); ok {
		return newBoolTrusted(out), nil
	}
	for row := 0; row < array.Len(); row++ {
		v, ok := array.At(row)
		if !ok {
			return nil, fmt.Errorf("within mask row %d out of range", row)
		}
		if IsNull(v) {
			continue
		}
		if compare(v, low) < 0 {
			continue
		}
		if highClosed {
			out[row] = compare(v, high) <= 0
		} else {
			out[row] = compare(v, high) < 0
		}
	}
	return newBoolTrusted(out), nil
}

func compareMask(array Array, op Op, value any) (Array, error) {
	if array == nil {
		return nil, fmt.Errorf("compare mask array is nil")
	}
	out := make([]bool, array.Len())
	if IsNull(value) {
		for row := 0; row < array.Len(); row++ {
			v, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("compare mask row %d out of range", row)
			}
			out[row] = nullOrderedCompare(op, IsNull(v), true)
		}
		return newBoolTrusted(out), nil
	}
	value = normalizeScalar(array.Kind(), value)
	if ok := compareMaskTyped(array, op, value, out); ok {
		return newBoolTrusted(out), nil
	}
	for row := 0; row < array.Len(); row++ {
		v, ok := array.At(row)
		if !ok {
			return nil, fmt.Errorf("compare mask row %d out of range", row)
		}
		if IsNull(v) {
			out[row] = nullOrderedCompare(op, true, false)
			continue
		}
		result, err := ApplyBinary(op, v, value)
		if err != nil {
			return nil, err
		}
		keep, ok := result.(bool)
		if !ok {
			return nil, fmt.Errorf("compare mask operator %s did not return bool", op)
		}
		out[row] = keep
	}
	return newBoolTrusted(out), nil
}

func FilterMask(frame Frame, mask Array) (Frame, error) {
	if mask == nil {
		return Frame{}, fmt.Errorf("filter mask is nil")
	}
	if mask.Len() != frame.Len() {
		return Frame{}, fmt.Errorf("filter mask length %d does not match frame length %d", mask.Len(), frame.Len())
	}
	if out, handled, err := TryFilterFrameByBoolMask(frame, mask); handled || err != nil {
		return out, err
	}
	indexes, err := WhereMask(mask)
	if err != nil {
		return Frame{}, err
	}
	return frame.Gather(indexes)
}

// TryFilterFrameByBoolMask filters frame rows through typed mask and gather
// primitives, preserving lazy index/range views when the mask can be lowered to
// a typed i64 index vector.
func TryFilterFrameByBoolMask(frame Frame, mask Array) (Frame, bool, error) {
	if mask == nil {
		return Frame{}, true, fmt.Errorf("filter mask is nil")
	}
	if mask.Kind() != KindBool {
		return Frame{}, true, fmt.Errorf("filter mask kind is %s, want %s", mask.Kind(), KindBool)
	}
	if mask.Len() != frame.Len() {
		return Frame{}, true, fmt.Errorf("filter mask length %d does not match frame length %d", mask.Len(), frame.Len())
	}
	indexes, handled, err := TryTypedWhereMaskI64(mask)
	if err != nil || !handled {
		return Frame{}, handled, err
	}
	return TryGatherFrameByI64IndexArray(frame, indexes)
}

func BucketFloor(array Array, interval any) (Array, error) {
	if array == nil {
		return nil, fmt.Errorf("bucket floor array is nil")
	}
	if bucketed, ok, err := bucketFloorTyped(array, interval); ok || err != nil {
		recordDataRuntimeKernelProbe("DataBucketFloor", bucketFloorRuntimeShape(array), ok, err)
		return bucketed, err
	} else {
		recordDataRuntimeKernelProbe("DataBucketFloor", bucketFloorRuntimeShape(array), false, nil)
	}
	values := make([]any, array.Len())
	switch array.Kind() {
	case KindI8, KindI16, KindI32, KindI64, KindMonth, KindDate, KindDateTime, KindTimespan, KindMinute, KindSecond, KindTime, KindTimestamp:
		width, err := bucketInt64Interval(array.Kind(), interval)
		if err != nil {
			return nil, err
		}
		for row := 0; row < array.Len(); row++ {
			v, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("bucket floor row %d out of range", row)
			}
			if IsNull(v) {
				values[row] = NullValue
				continue
			}
			bucket, err := bucketFloorInt64Value(array.Kind(), v, width)
			if err != nil {
				return nil, fmt.Errorf("bucket floor row %d: %w", row, err)
			}
			values[row] = bucket
		}
	case KindU8, KindU16, KindU32, KindU64:
		width, err := bucketUint64Interval(interval)
		if err != nil {
			return nil, err
		}
		for row := 0; row < array.Len(); row++ {
			v, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("bucket floor row %d out of range", row)
			}
			if IsNull(v) {
				values[row] = NullValue
				continue
			}
			bucket, err := bucketFloorUint64Value(array.Kind(), v, width)
			if err != nil {
				return nil, fmt.Errorf("bucket floor row %d: %w", row, err)
			}
			values[row] = bucket
		}
	case KindF32, KindF64:
		width, err := bucketFloat64Interval(interval)
		if err != nil {
			return nil, err
		}
		for row := 0; row < array.Len(); row++ {
			v, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("bucket floor row %d out of range", row)
			}
			if IsNull(v) {
				values[row] = NullValue
				continue
			}
			bucket, err := bucketFloorFloatValue(array.Kind(), v, width)
			if err != nil {
				return nil, fmt.Errorf("bucket floor row %d: %w", row, err)
			}
			values[row] = bucket
		}
	default:
		return nil, fmt.Errorf("bucket floor kind %s is not supported", array.Kind())
	}
	return arrayWithKind(array.Kind(), values)
}

func bucketFloorRuntimeShape(array Array) string {
	kind := KindAny
	if array != nil {
		kind = array.Kind()
	}
	return "bucket-floor/xbar/" + string(kind)
}

func bucketFloorTyped(array Array, interval any) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return bucketFloorTyped(a.array, interval)
	case columnArray[Month]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[Date]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[DateTime]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[Timespan]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[Minute]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[Second]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[Time]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[Timestamp]:
		return bucketFloorTemporalColumn(a, interval)
	case columnArray[int64]:
		width, err := bucketInt64Interval(KindI64, interval)
		if err != nil {
			return nil, true, err
		}
		return i64BucketArray{source: a, width: width, len: a.Len()}, true, nil
	case i64RangeArray:
		width, err := bucketInt64Interval(KindI64, interval)
		if err != nil {
			return nil, true, err
		}
		return i64BucketArray{source: a, width: width, len: a.Len()}, true, nil
	case i64SegmentArray:
		width, err := bucketInt64Interval(KindI64, interval)
		if err != nil {
			return nil, true, err
		}
		return i64BucketArray{source: a, width: width, len: a.Len()}, true, nil
	case i64ScalarDyadicArray:
		width, err := bucketInt64Interval(KindI64, interval)
		if err != nil {
			return nil, true, err
		}
		return i64BucketArray{source: a, width: width, len: a.Len()}, true, nil
	case i64RunningSumArray:
		width, err := bucketInt64Interval(KindI64, interval)
		if err != nil {
			return nil, true, err
		}
		return i64BucketArray{source: a, width: width, len: a.Len()}, true, nil
	case i64ScalarDyadicRunningSumArray:
		width, err := bucketInt64Interval(KindI64, interval)
		if err != nil {
			return nil, true, err
		}
		return i64BucketArray{source: a, width: width, len: a.Len()}, true, nil
	case f64RangeArray:
		width, err := bucketFloat64Interval(interval)
		if err != nil {
			return nil, true, err
		}
		return f64BucketArray{source: a, width: width, kind: KindF64, len: a.Len()}, true, nil
	case columnArray[float32]:
		width, err := bucketFloat64Interval(interval)
		if err != nil {
			return nil, true, err
		}
		return f64BucketArray{source: a, width: width, kind: KindF32, len: a.Len()}, true, nil
	case columnArray[float64]:
		width, err := bucketFloat64Interval(interval)
		if err != nil {
			return nil, true, err
		}
		return f64BucketArray{source: a, width: width, kind: KindF64, len: a.Len()}, true, nil
	default:
		// Any other float-kind numeric carrier (lazy dyadic trees, producer
		// views) buckets through bulk flattening into a dense float column.
		// Null-bearing carriers fail the bulk flatten and keep the generic
		// NullValue-producing fallback.
		if array.Kind() == KindF64 && isNumericArray(array) {
			width, err := bucketFloat64Interval(interval)
			if err != nil {
				return nil, true, err
			}
			if values, owned, ok := tryBulkF64Values(array); ok {
				out := make([]float64, len(values))
				for i, v := range values {
					out[i] = math.Floor(v/width) * width
				}
				bulkF64Release(values, owned)
				return newF64Trusted(out), true, nil
			}
		}
		return nil, false, nil
	}
}

// bucketFloorTemporalColumn floors a dense named-int64 temporal column in
// place-type, preserving the source kind (mirroring bucketFloorInt64Value's
// kind coercion without per-row boxing).
func bucketFloorTemporalColumn[T ~int64](a columnArray[T], interval any) (Array, bool, error) {
	width, err := bucketInt64Interval(a.kind, interval)
	if err != nil {
		return nil, true, err
	}
	out := make([]T, len(a.data))
	for i, v := range a.data {
		out[i] = T(floorInt64(int64(v), width))
	}
	return columnArray[T]{kind: a.kind, data: out}, true, nil
}

func bucketFloorI64Slice(values []int64, width int64) Array {
	out := make([]int64, len(values))
	for i, value := range values {
		out[i] = floorInt64(value, width)
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func bucketFloorI64Range(values i64RangeArray, width int64) Array {
	out := make([]int64, values.len)
	for i := range out {
		out[i] = floorInt64(values.start+int64(i)*values.step, width)
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func Filter(frame Frame, keep func(row map[Symbol]any) (bool, error)) (Frame, error) {
	if keep == nil {
		return Frame{}, fmt.Errorf("filter predicate is nil")
	}
	indexes := make([]int, 0, frame.Len())
	for i := 0; i < frame.Len(); i++ {
		row, err := frame.Row(i)
		if err != nil {
			return Frame{}, err
		}
		ok, err := keep(row)
		if err != nil {
			return Frame{}, err
		}
		if ok {
			indexes = append(indexes, i)
		}
	}
	return frame.Gather(indexes)
}

func Update(frame Frame, match func(row map[Symbol]any) (bool, error), assignments map[Symbol]func(row map[Symbol]any) (any, error)) (Frame, error) {
	if match == nil {
		return Frame{}, fmt.Errorf("update predicate is nil")
	}
	if len(assignments) == 0 {
		return Frame{}, fmt.Errorf("update requires at least one assignment")
	}
	for name, assign := range assignments {
		if assign == nil {
			return Frame{}, fmt.Errorf("update assignment for column %q is nil", name)
		}
		if _, ok := frame.Column(name); !ok {
			return Frame{}, fmt.Errorf("update column %q does not exist", name)
		}
	}
	matched := make([]bool, frame.Len())
	rowValues := make([]map[Symbol]any, frame.Len())
	for row := 0; row < frame.Len(); row++ {
		values, err := frame.Row(row)
		if err != nil {
			return Frame{}, err
		}
		ok, err := match(values)
		if err != nil {
			return Frame{}, err
		}
		matched[row] = ok
		rowValues[row] = values
	}
	cols := make([]Column, 0, len(frame.schema.names))
	for _, name := range frame.schema.names {
		values := frame.columns[name].Values()
		assign, ok := assignments[name]
		if ok {
			for row := 0; row < frame.Len(); row++ {
				if !matched[row] {
					continue
				}
				v, err := assign(rowValues[row])
				if err != nil {
					return Frame{}, err
				}
				values[row] = v
			}
		}
		col, err := columnWithKind(name, frame.columns[name].Kind(), values)
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, col)
	}
	return newFrameTrusted(cols...)
}

func UpdateWhere(frame Frame, where Expr, assignments map[Symbol]Expr) (Frame, error) {
	if len(assignments) == 0 {
		return Frame{}, fmt.Errorf("update requires at least one assignment")
	}
	for name, expr := range assignments {
		if expr == nil {
			return Frame{}, fmt.Errorf("update assignment for column %q is nil", name)
		}
	}
	indexes, err := filterIndexes(frame, where)
	if err != nil {
		return Frame{}, err
	}
	return updateRowsWhere(frame, indexes, assignments)
}

// updateRowsWhere applies update assignments at the matched row indexes.
// indexes must be unique, in-range, and ascending; they are borrowed and never
// mutated, so index-owned row lists may be passed directly.
func updateRowsWhere(frame Frame, indexes []int, assignments map[Symbol]Expr) (Frame, error) {
	if len(assignments) == 0 {
		return Frame{}, fmt.Errorf("update requires at least one assignment")
	}
	for name, expr := range assignments {
		if expr == nil {
			return Frame{}, fmt.Errorf("update assignment for column %q is nil", name)
		}
	}
	cols := make([]Column, 0, len(frame.schema.names)+len(assignments))
	for _, name := range frame.schema.names {
		col := frame.columns[name]
		expr, assigned := assignments[name]
		if !assigned || len(indexes) == 0 {
			// Copy-on-write: untouched (or no-match) columns are shared.
			cols = append(cols, Column{Name: name, Data: col})
			continue
		}
		vals, err := evalProjectionExprRows(frame, indexes, expr)
		if err != nil {
			return Frame{}, err
		}
		if out, ok := arrayScatterArray(col, indexes, vals); ok {
			cols = append(cols, Column{Name: name, Data: out})
			continue
		}
		// Boxed fallback preserving legacy kind-coercion semantics.
		values := col.Values()
		for i, row := range indexes {
			v, ok := vals.At(i)
			if !ok {
				return Frame{}, fmt.Errorf("update assignment for column %q row %d out of range", name, row)
			}
			values[row] = v
		}
		out, err := columnWithKind(name, col.Kind(), values)
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, out)
	}
	for name, expr := range assignments {
		if _, ok := frame.Column(name); ok {
			continue
		}
		if len(indexes) == 0 {
			values := make([]any, frame.Len())
			for row := range values {
				values[row] = NullValue
			}
			cols = append(cols, NewColumn(name, values))
			continue
		}
		vals, err := evalProjectionExprRows(frame, indexes, expr)
		if err != nil {
			return Frame{}, err
		}
		if out, ok := newSparseColumnArray(frame.Len(), indexes, vals); ok {
			cols = append(cols, Column{Name: name, Data: out})
			continue
		}
		values := make([]any, frame.Len())
		for row := range values {
			values[row] = NullValue
		}
		for i, row := range indexes {
			v, ok := vals.At(i)
			if !ok {
				return Frame{}, fmt.Errorf("update assignment for column %q row %d out of range", name, row)
			}
			values[row] = v
		}
		cols = append(cols, NewColumn(name, values))
	}
	return newFrameTrusted(cols...)
}

func UpdateBy(frame Frame, where Expr, by []SelectItem, assignments []GroupedAssignment) (Frame, error) {
	if len(by) == 0 {
		return Frame{}, fmt.Errorf("grouped update requires at least one by expression")
	}
	if len(assignments) == 0 {
		return Frame{}, fmt.Errorf("grouped update requires at least one assignment")
	}
	byInputs, err := bindGroupInputs(frame, by)
	if err != nil {
		return Frame{}, err
	}
	hasWindowAssignments := false
	aggs := make([]aggregateInput, len(assignments))
	for i, assign := range assignments {
		if assign.Name == "" {
			return Frame{}, fmt.Errorf("grouped update assignment %d has empty name", i)
		}
		if assign.Expr == nil {
			return Frame{}, fmt.Errorf("grouped update assignment for column %q is nil", assign.Name)
		}
		if assign.Func == "" {
			// Grouped window assignment: the expression is evaluated over each
			// group's rows and scattered back row-by-row (e.g. s:sums v by g).
			hasWindowAssignments = true
		} else if !isSupportedAggregate(assign.Func) {
			return Frame{}, fmt.Errorf("unsupported grouped update aggregate %q", assign.Func)
		}
		aggs[i].Aggregate = Aggregate{Name: assign.Name, Func: assign.Func, Expr: assign.Expr, Weight: assign.Weight}
		if ref, ok := assign.Expr.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return Frame{}, fmt.Errorf("unknown column %q", ref.Name)
			}
			aggs[i].column = col
		}
		if ref, ok := assign.Weight.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return Frame{}, fmt.Errorf("unknown column %q", ref.Name)
			}
			aggs[i].weightColumn = col
		}
	}
	indexes, err := filterIndexes(frame, where)
	if err != nil {
		return Frame{}, err
	}
	if !hasWindowAssignments {
		if out, ok, err := updateByGroupedTyped(frame, indexes, byInputs, aggs, assignments); ok {
			return out, err
		}
	}
	matched := make([]bool, frame.Len())
	rowKeys := make([]string, frame.Len())
	groups := map[string]*groupState{}
	var groupRows map[string][]int
	var groupOrder []string
	if hasWindowAssignments {
		groupRows = map[string][]int{}
	}
	var keyBuilder strings.Builder
	for _, row := range indexes {
		matched[row] = true
		keyVals := make([]any, len(byInputs))
		keyBuilder.Reset()
		for i, item := range byInputs {
			v, err := item.value(frame, row)
			if err != nil {
				return Frame{}, err
			}
			keyVals[i] = v
			appendKeyPart(&keyBuilder, item.keyKind(), v)
		}
		key := keyBuilder.String()
		rowKeys[row] = key
		state := groups[key]
		if state == nil {
			state = &groupState{keys: keyVals, aggs: make([]aggregateState, len(aggs))}
			for i, agg := range aggs {
				state.aggs[i].fn = agg.Func
			}
			groups[key] = state
			groupOrder = append(groupOrder, key)
		}
		if groupRows != nil {
			groupRows[key] = append(groupRows[key], row)
		}
		for i, agg := range aggs {
			if agg.Func == "" {
				continue
			}
			if err := accumulateAggregate(&state.aggs[i], agg, frame, row); err != nil {
				return Frame{}, err
			}
		}
	}
	windowValues, err := updateByWindowValues(frame, assignments, groupOrder, groupRows)
	if err != nil {
		return Frame{}, err
	}
	assignmentByName := make(map[Symbol]int, len(assignments))
	for i, assign := range assignments {
		if _, ok := assignmentByName[assign.Name]; ok {
			return Frame{}, fmt.Errorf("duplicate grouped update assignment for column %q", assign.Name)
		}
		assignmentByName[assign.Name] = i
	}
	assignedValue := func(assignIndex, row int) any {
		if windowValues != nil && windowValues[assignIndex] != nil {
			return windowValues[assignIndex][row]
		}
		return aggregateResult(groups[rowKeys[row]].aggs[assignIndex])
	}
	cols := make([]Column, 0, len(frame.schema.names)+len(assignments))
	for _, name := range frame.schema.names {
		values := frame.columns[name].Values()
		assignIndex, ok := assignmentByName[name]
		if ok {
			for row := 0; row < frame.Len(); row++ {
				if !matched[row] {
					continue
				}
				values[row] = assignedValue(assignIndex, row)
			}
		}
		col, err := columnWithKind(name, frame.columns[name].Kind(), values)
		if err != nil {
			if !ok {
				return Frame{}, err
			}
			// Assigned values may change the column kind (e.g. sums over a
			// float column written into an integer column).
			col = NewColumn(name, values)
		}
		cols = append(cols, col)
	}
	for i, assign := range assignments {
		if _, ok := frame.Column(assign.Name); ok {
			continue
		}
		values := make([]any, frame.Len())
		for row := 0; row < frame.Len(); row++ {
			if !matched[row] {
				values[row] = NullValue
				continue
			}
			values[row] = assignedValue(i, row)
		}
		cols = append(cols, NewColumn(assign.Name, values))
	}
	return newFrameTrusted(cols...)
}

// updateByWindowValues evaluates grouped window assignments (Func == "") per
// group and scatters the per-row results into frame-length value vectors.
// The outer slice is indexed by assignment; nil entries are aggregates.
func updateByWindowValues(frame Frame, assignments []GroupedAssignment, groupOrder []string, groupRows map[string][]int) ([][]any, error) {
	if groupRows == nil {
		return nil, nil
	}
	var out [][]any
	for i, assign := range assignments {
		if assign.Func != "" {
			continue
		}
		if out == nil {
			out = make([][]any, len(assignments))
		}
		values := make([]any, frame.Len())
		projector, isProjector := assign.Expr.(vectorProjector)
		for _, key := range groupOrder {
			rows := groupRows[key]
			if isProjector {
				array, err := projector.EvalRows(frame, rows)
				if err != nil {
					return nil, err
				}
				if array.Len() != len(rows) {
					return nil, fmt.Errorf("grouped update assignment %q returned %d rows for a %d-row group", assign.Name, array.Len(), len(rows))
				}
				for k, row := range rows {
					v, ok := array.At(k)
					if !ok {
						return nil, fmt.Errorf("grouped update assignment %q group row %d out of range", assign.Name, k)
					}
					values[row] = v
				}
				continue
			}
			for _, row := range rows {
				v, err := assign.Expr.EvalRow(frame, row)
				if err != nil {
					return nil, err
				}
				values[row] = v
			}
		}
		out[i] = values
	}
	return out, nil
}

func Delete(frame Frame, match func(row map[Symbol]any) (bool, error)) (Frame, error) {
	if match == nil {
		return Frame{}, fmt.Errorf("delete predicate is nil")
	}
	indexes := make([]int, 0, frame.Len())
	for row := 0; row < frame.Len(); row++ {
		rowValues, err := frame.Row(row)
		if err != nil {
			return Frame{}, err
		}
		matched, err := match(rowValues)
		if err != nil {
			return Frame{}, err
		}
		if !matched {
			indexes = append(indexes, row)
		}
	}
	return frame.Gather(indexes)
}

func DeleteWhere(frame Frame, where Expr) (Frame, error) {
	deleteIndexes, err := filterIndexes(frame, where)
	if err != nil {
		return Frame{}, err
	}
	if len(deleteIndexes) == 0 {
		// Frames are immutable values; an empty match is a no-op.
		bulkIntRelease(deleteIndexes)
		return frame, nil
	}
	// The matched-row vector is transient: only its complement survives in
	// the result views, so it goes back to the bulk pool.
	defer bulkIntRelease(deleteIndexes)
	if indexes, ok, err := typedKernels.ComplementSortedIndexes(frame.Len(), deleteIndexes); ok || err != nil {
		if err != nil {
			return Frame{}, err
		}
		// Survivors are exposed through lazy index views (one shared index,
		// no dense per-column copies), mirroring DeleteWhereKeyed.
		return gatherFrameRowsView(frame, indexes)
	}
	deleted := make([]bool, frame.Len())
	for _, row := range deleteIndexes {
		deleted[row] = true
	}
	indexes := make([]int, 0, frame.Len()-len(deleteIndexes))
	for row := 0; row < frame.Len(); row++ {
		if !deleted[row] {
			indexes = append(indexes, row)
		}
	}
	return gatherFrameRowsView(frame, indexes)
}

func DropColumns(frame Frame, names ...Symbol) (Frame, error) {
	drop := make(map[Symbol]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			return Frame{}, fmt.Errorf("drop column name must not be empty")
		}
		if _, ok := frame.Column(name); !ok {
			return Frame{}, fmt.Errorf("drop column %q does not exist", name)
		}
		drop[name] = struct{}{}
	}
	if len(drop) == len(frame.schema.names) {
		return Frame{}, fmt.Errorf("drop would remove all columns")
	}
	cols := make([]Column, 0, len(frame.schema.names)-len(drop))
	for _, name := range frame.schema.names {
		if _, ok := drop[name]; ok {
			continue
		}
		cols = append(cols, Column{Name: name, Data: frame.columns[name]})
	}
	// Frames are immutable values: dropping columns adopts the kept column
	// arrays instead of dense-copying every survivor.
	return NewFrameAdoptingColumns(cols...)
}

func InsertRow(frame Frame, columns []Symbol, values []any) (Frame, error) {
	_, row, err := rowMutationRecord(frame, columns, values)
	if err != nil {
		return Frame{}, err
	}
	cols := make([]Column, 0, len(frame.schema.names))
	for _, name := range frame.schema.names {
		current := frame.columns[name]
		if out, ok := arrayWithTypedEdits(current, nil, nil, []any{row[name]}); ok {
			cols = append(cols, Column{Name: name, Data: out})
			continue
		}
		colValues := current.Values()
		colValues = append(colValues, row[name])
		col, err := columnWithKind(name, current.Kind(), colValues)
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, col)
	}
	return newFrameTrusted(cols...)
}

func UpsertRow(frame Frame, columns []Symbol, values []any) (Frame, error) {
	return InsertRow(frame, columns, values)
}

func (k KeyedFrame) InsertRow(columns []Symbol, values []any) (KeyedFrame, error) {
	if len(k.keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("keyed frame is not initialized")
	}
	if err := rowMutationRequireKeyColumns(k.keys, columns); err != nil {
		return KeyedFrame{}, err
	}
	delta, err := rowMutationDeltaFrame(k.frame, columns, values)
	if err != nil {
		return KeyedFrame{}, err
	}
	key, _, err := deltaRowKey(k, delta, 0)
	if err != nil {
		return KeyedFrame{}, err
	}
	if rows := k.lookupRowsByKey(key); len(rows) > 0 {
		return KeyedFrame{}, fmt.Errorf("keyed insert duplicate key")
	}
	valueColumns, err := rowMutationValueColumns(k.keys, k.frame.schema.names, columns)
	if err != nil {
		return KeyedFrame{}, err
	}
	return k.Upsert(delta, valueColumns...)
}

func (k KeyedFrame) UpsertRow(columns []Symbol, values []any) (KeyedFrame, error) {
	if len(k.keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("keyed frame is not initialized")
	}
	if err := rowMutationRequireKeyColumns(k.keys, columns); err != nil {
		return KeyedFrame{}, err
	}
	delta, err := rowMutationDeltaFrame(k.frame, columns, values)
	if err != nil {
		return KeyedFrame{}, err
	}
	valueColumns, err := rowMutationValueColumns(k.keys, k.frame.schema.names, columns)
	if err != nil {
		return KeyedFrame{}, err
	}
	return k.Upsert(delta, valueColumns...)
}

func InsertRowKeyed(keyed KeyedFrame, columns []Symbol, values []any) (KeyedFrame, error) {
	return keyed.InsertRow(columns, values)
}

func UpsertRowKeyed(keyed KeyedFrame, columns []Symbol, values []any) (KeyedFrame, error) {
	return keyed.UpsertRow(columns, values)
}

func RenameColumns(frame Frame, renames map[Symbol]Symbol) (Frame, error) {
	if len(renames) == 0 {
		return frame.Gather(allIndexes(frame.Len()))
	}
	for from, to := range renames {
		if from == "" || to == "" {
			return Frame{}, fmt.Errorf("rename column names must not be empty")
		}
		if _, ok := frame.Column(from); !ok {
			return Frame{}, fmt.Errorf("rename column %q does not exist", from)
		}
	}
	seen := make(map[Symbol]struct{}, len(frame.schema.names))
	cols := make([]Column, 0, len(frame.schema.names))
	for _, name := range frame.schema.names {
		outName := name
		if renamed, ok := renames[name]; ok {
			outName = renamed
		}
		if _, ok := seen[outName]; ok {
			return Frame{}, fmt.Errorf("rename produces duplicate column %q", outName)
		}
		seen[outName] = struct{}{}
		cols = append(cols, Column{Name: outName, Data: frame.columns[name]})
	}
	return NewFrame(cols...)
}

func Distinct(frame Frame, columns ...Symbol) (Frame, error) {
	keyColumns := append([]Symbol(nil), columns...)
	if len(keyColumns) == 0 {
		keyColumns = frame.schema.Names()
	}
	for _, name := range keyColumns {
		if _, ok := frame.Column(name); !ok {
			return Frame{}, fmt.Errorf("distinct column %q does not exist", name)
		}
	}
	if len(keyColumns) == 1 {
		if indexes, ok := distinctSingleColumnIndexes(frame, keyColumns[0]); ok {
			return frame.Gather(indexes)
		}
	}
	seen := make(map[string]struct{}, frame.Len())
	indexes := make([]int, 0, frame.Len())
	for row := 0; row < frame.Len(); row++ {
		key, err := rowKey(frame, row, keyColumns)
		if err != nil {
			return Frame{}, err
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		indexes = append(indexes, row)
	}
	return frame.Gather(indexes)
}

func distinctSingleColumnIndexes(frame Frame, name Symbol) ([]int, bool) {
	column, ok := frame.Column(name)
	if !ok {
		return nil, false
	}
	if index, ok := arrayIndexForBorrowed(column, ArrayAttributeUnique); ok {
		return firstRowsFromArrayIndex(index), true
	}
	if index, ok := arrayIndexForBorrowed(column, ArrayAttributeGrouped); ok {
		return firstRowsFromArrayIndex(index), true
	}
	return nil, false
}

func firstRowsFromArrayIndex(index ArrayIndex) []int {
	indexes := make([]int, 0, len(index.Rows))
	for _, rows := range index.Rows {
		if len(rows) == 0 {
			continue
		}
		indexes = append(indexes, rows[0])
	}
	return indexes
}

// XGroup groups a frame by key columns, producing a keyed frame whose value
// columns contain nested arrays of each group's original row values.
func XGroup(frame Frame, keys ...Symbol) (KeyedFrame, error) {
	if len(keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("xgroup requires at least one key column")
	}
	keyColumns, err := validateKeyColumns(frame, keys, "xgroup")
	if err != nil {
		return KeyedFrame{}, err
	}
	keySet := make(map[Symbol]struct{}, len(keyColumns))
	for _, name := range keyColumns {
		keySet[name] = struct{}{}
	}
	valueColumns := make([]Symbol, 0, len(frame.schema.names)-len(keySet))
	for _, name := range frame.schema.names {
		if _, ok := keySet[name]; !ok {
			valueColumns = append(valueColumns, name)
		}
	}

	var groups [][]int
	if ids, groupCount, ok := frameRowGroupIDs(frame, keyColumns); ok {
		// Typed path: reuse the fby group-id kernels instead of building a
		// quoted string key per row.
		groups = rowGroupsFromIDs(ids, groupCount)
	} else {
		positionByKey := make(map[string]int, frame.Len())
		for row := 0; row < frame.Len(); row++ {
			key, err := rowKey(frame, row, keyColumns)
			if err != nil {
				return KeyedFrame{}, err
			}
			position, ok := positionByKey[key]
			if !ok {
				position = len(groups)
				positionByKey[key] = position
				groups = append(groups, nil)
			}
			groups[position] = append(groups[position], row)
		}
	}

	cols := make([]Column, 0, len(keyColumns)+len(valueColumns))
	for _, name := range keyColumns {
		source, _ := frame.Column(name)
		values := make([]any, len(groups))
		for i, rows := range groups {
			v, ok := source.At(rows[0])
			if !ok {
				return KeyedFrame{}, fmt.Errorf("xgroup key column %q row %d out of range", name, rows[0])
			}
			values[i] = v
		}
		cols = append(cols, NewColumn(name, values))
	}
	for _, name := range valueColumns {
		source, _ := frame.Column(name)
		values := make([]any, len(groups))
		for i, rows := range groups {
			values[i] = source.Gather(rows)
		}
		cols = append(cols, NewColumn(name, values))
	}
	grouped, err := NewFrame(cols...)
	if err != nil {
		return KeyedFrame{}, err
	}
	return KeyBy(grouped, keyColumns...)
}

// Ungroup expands nested array/list columns in a frame. Nested columns in the
// same source row must have the same length; scalar columns are repeated.
func Ungroup(frame Frame) (Frame, error) {
	if out, handled, err := ungroupColumnar(frame); handled {
		return out, err
	}
	names := frame.schema.Names()
	out := make(map[Symbol][]any, len(names))
	for _, name := range names {
		out[name] = make([]any, 0, frame.Len())
	}
	for row := 0; row < frame.Len(); row++ {
		width := -1
		nested := make(map[Symbol]nestedRowValue, len(names))
		scalars := make(map[Symbol]any, len(names))
		for _, name := range names {
			col := frame.columns[name]
			value, ok := col.At(row)
			if !ok {
				return Frame{}, fmt.Errorf("ungroup column %q row %d out of range", name, row)
			}
			if n, ok := asNestedRowValue(value); ok {
				if width < 0 {
					width = n.Len()
				} else if width != n.Len() {
					return Frame{}, fmt.Errorf("ungroup row %d has mismatched nested column lengths", row)
				}
				nested[name] = n
				continue
			}
			scalars[name] = value
		}
		if width < 0 {
			width = 1
		}
		for i := 0; i < width; i++ {
			for _, name := range names {
				if n, ok := nested[name]; ok {
					value, ok := n.At(i)
					if !ok {
						return Frame{}, fmt.Errorf("ungroup column %q nested row %d out of range", name, i)
					}
					out[name] = append(out[name], value)
					continue
				}
				out[name] = append(out[name], scalars[name])
			}
		}
	}
	cols := make([]Column, 0, len(names))
	for _, name := range names {
		cols = append(cols, NewColumn(name, out[name]))
	}
	return NewFrame(cols...)
}

type nestedRowValue struct {
	array dataNestedArray
	list  []any
}

type dataNestedArray interface {
	Len() int
	At(row int) (any, bool)
}

func asNestedRowValue(value any) (nestedRowValue, bool) {
	if array, ok := value.(Array); ok {
		return nestedRowValue{array: array}, true
	}
	if list, ok := value.([]any); ok {
		return nestedRowValue{list: list}, true
	}
	return nestedRowValue{}, false
}

func (n nestedRowValue) Len() int {
	if n.array != nil {
		return n.array.Len()
	}
	return len(n.list)
}

func (n nestedRowValue) At(row int) (any, bool) {
	if n.array != nil {
		return n.array.At(row)
	}
	if row < 0 || row >= len(n.list) {
		return nil, false
	}
	return n.list[row], true
}

type KeyedFrame struct {
	frame     Frame
	keys      []Symbol
	rowsByKey map[string][]int
	// rowsOverlay holds key->rows entries for keys appended after rowsByKey
	// was built; its key set is disjoint from rowsByKey. Keeping appends in a
	// small overlay makes single-row keyed inserts O(delta) instead of
	// cloning the whole index map; mutateTyped flattens the overlay once it
	// grows past keyedRowsOverlayFlattenLimit.
	rowsOverlay map[string][]int
	index       KeyedIndexMetadata
	// fp memoizes the O(rows) key fingerprint; it is computed lazily so that
	// keyed mutations stay O(delta). The cell is shared between derived keyed
	// frames whose row->key mapping is provably unchanged.
	fp *keyedFingerprintCell
}

const keyedRowsOverlayFlattenLimit = 256

// lookupRowsByKey resolves a key against the base index plus the append
// overlay.
func (k KeyedFrame) lookupRowsByKey(key string) []int {
	if rows, ok := k.rowsOverlay[key]; ok {
		return rows
	}
	return k.rowsByKey[key]
}

func (k KeyedFrame) indexKeyCount() int {
	return len(k.rowsByKey) + len(k.rowsOverlay)
}

// forEachIndexKey visits every key->rows entry (base first, then overlay).
func (k KeyedFrame) forEachIndexKey(fn func(key string, rows []int)) {
	for key, rows := range k.rowsByKey {
		fn(key, rows)
	}
	for key, rows := range k.rowsOverlay {
		fn(key, rows)
	}
}

// mergedRowsByKey materializes the base+overlay index into a single map.
func (k KeyedFrame) mergedRowsByKey() map[string][]int {
	if len(k.rowsOverlay) == 0 {
		return k.rowsByKey
	}
	out := make(map[string][]int, k.indexKeyCount())
	for key, rows := range k.rowsByKey {
		out[key] = rows
	}
	for key, rows := range k.rowsOverlay {
		out[key] = rows
	}
	return out
}

type keyedFingerprintCell struct {
	once  sync.Once
	value uint64
	err   error
}

// KeyedIndexMetadata describes the key index sidecar attached to a KeyedFrame.
// Frames are immutable values; keyed mutations return a fresh KeyedFrame with a
// rebuilt index instead of reusing the old rowsByKey map.
type KeyedIndexMetadata struct {
	Rows        int
	Keys        []Symbol
	SchemaHash  string
	Fingerprint uint64
}

func newKeyedFrameWithIndex(frame Frame, keys []Symbol, rowsByKey map[string][]int) (KeyedFrame, error) {
	if _, err := validateKeyColumns(frame, keys, "keyed frame index"); err != nil {
		return KeyedFrame{}, err
	}
	return KeyedFrame{
		frame:     frame,
		keys:      append([]Symbol(nil), keys...),
		rowsByKey: rowsByKey,
		index: KeyedIndexMetadata{
			Rows:       frame.Len(),
			Keys:       append([]Symbol(nil), keys...),
			SchemaHash: frame.SchemaFingerprint(),
		},
		fp: &keyedFingerprintCell{},
	}, nil
}

func buildKeyedIndexMetadata(frame Frame, keys []Symbol) (KeyedIndexMetadata, error) {
	if _, err := validateKeyColumns(frame, keys, "keyed frame index"); err != nil {
		return KeyedIndexMetadata{}, err
	}
	schemaHash := frame.SchemaFingerprint()
	fingerprint, err := keyedRowsFingerprint(frame, keys, schemaHash)
	if err != nil {
		return KeyedIndexMetadata{}, err
	}
	return KeyedIndexMetadata{
		Rows:        frame.Len(),
		Keys:        append([]Symbol(nil), keys...),
		SchemaHash:  schemaHash,
		Fingerprint: fingerprint,
	}, nil
}

func keyedRowsFingerprint(frame Frame, keys []Symbol, schemaHash string) (uint64, error) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(schemaHash))
	_, _ = h.Write([]byte{0})
	for _, key := range keys {
		_, _ = h.Write([]byte(key))
		_, _ = h.Write([]byte{0xff})
	}
	for row := 0; row < frame.Len(); row++ {
		key, err := rowKey(frame, row, keys)
		if err != nil {
			return 0, err
		}
		_, _ = h.Write([]byte(key))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64(), nil
}

// indexFingerprintFromRows reproduces keyedRowsFingerprint from the stored
// key index (base map plus append overlay) instead of re-deriving per-row key
// strings from the frame: index keys are exactly the rowKey() strings, so
// hashing them in row order yields the same fingerprint without per-row
// formatting. Deriving the fingerprint from the index (not the live frame)
// keeps ValidateIndex able to detect index/frame divergence.
func (k KeyedFrame) indexFingerprintFromRows() (uint64, error) {
	rows := k.index.Rows
	keyByRow := make([]string, rows)
	var rangeErr error
	k.forEachIndexKey(func(key string, keyRows []int) {
		if rangeErr != nil {
			return
		}
		for _, row := range keyRows {
			if row < 0 || row >= rows {
				rangeErr = fmt.Errorf("keyed frame index row %d out of range", row)
				return
			}
			keyByRow[row] = key
		}
	})
	if rangeErr != nil {
		return 0, rangeErr
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(k.index.SchemaHash))
	_, _ = h.Write([]byte{0})
	for _, key := range k.index.Keys {
		_, _ = h.Write([]byte(key))
		_, _ = h.Write([]byte{0xff})
	}
	for _, key := range keyByRow {
		_, _ = h.Write([]byte(key))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64(), nil
}

// indexFingerprint forces the lazily-computed key fingerprint.
func (k KeyedFrame) indexFingerprint() (uint64, error) {
	if k.index.Fingerprint != 0 || len(k.keys) == 0 {
		return k.index.Fingerprint, nil
	}
	if k.fp == nil {
		return k.indexFingerprintFromRows()
	}
	k.fp.once.Do(func() {
		k.fp.value, k.fp.err = k.indexFingerprintFromRows()
	})
	return k.fp.value, k.fp.err
}

func cloneRowsByKey(rowsByKey map[string][]int) map[string][]int {
	out := make(map[string][]int, len(rowsByKey))
	for key, rows := range rowsByKey {
		out[key] = append([]int(nil), rows...)
	}
	return out
}

func KeyBy(frame Frame, keys ...Symbol) (KeyedFrame, error) {
	if len(keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("keyed frame requires at least one key")
	}
	keyColumns, err := validateKeyColumns(frame, keys, "keyed frame")
	if err != nil {
		return KeyedFrame{}, err
	}
	rowsByKey := make(map[string][]int, frame.Len())
	if len(keyColumns) == 1 {
		if keyColumn, ok := frame.Column(keyColumns[0]); ok {
			if index, ok := arrayIndexForBorrowed(keyColumn, ArrayAttributeUnique); ok {
				for key, rows := range index.RowsByKey {
					rowsByKey[key] = append([]int(nil), rows...)
				}
				return newKeyedFrameWithIndex(frame, keyColumns, rowsByKey)
			}
			if index, ok := arrayIndexForBorrowed(keyColumn, ArrayAttributeGrouped); ok {
				for key, rows := range index.RowsByKey {
					rowsByKey[key] = append([]int(nil), rows...)
				}
				return newKeyedFrameWithIndex(frame, keyColumns, rowsByKey)
			}
		}
	}
	for row := 0; row < frame.Len(); row++ {
		key, err := rowKey(frame, row, keyColumns)
		if err != nil {
			return KeyedFrame{}, err
		}
		rowsByKey[key] = append(rowsByKey[key], row)
	}
	return newKeyedFrameWithIndex(frame, keyColumns, rowsByKey)
}

func (k KeyedFrame) Frame() Frame {
	// Frames are immutable values, so the backing frame can be shared
	// directly instead of gathering a full defensive copy.
	return k.frame
}

// KeyedFrameColumnNames returns the underlying table's column names without
// cloning row data.
func KeyedFrameColumnNames(keyed KeyedFrame) []Symbol {
	return keyed.frame.schema.Names()
}

// KeyedFrameColumnKinds returns the underlying table's column kinds in schema
// order without cloning row data.
func KeyedFrameColumnKinds(keyed KeyedFrame) []Kind {
	return FrameColumnKinds(keyed.frame)
}

// KeyedFrameColumnAttributes returns the underlying table's first column
// attributes in schema order without cloning row data.
func KeyedFrameColumnAttributes(keyed KeyedFrame) []Symbol {
	return FrameColumnAttributes(keyed.frame)
}

// KeyedFrameLen returns the underlying row count without cloning row data.
func KeyedFrameLen(keyed KeyedFrame) int {
	return keyed.frame.Len()
}

// ReorderKeyedFrameColumns reorders the underlying table columns while
// preserving the existing key columns and keyed index sidecar.
func ReorderKeyedFrameColumns(keyed KeyedFrame, requested ...Symbol) (KeyedFrame, error) {
	frame, err := ReorderFrameColumns(keyed.frame, requested...)
	if err != nil {
		return KeyedFrame{}, err
	}
	return newKeyedFrameWithIndex(frame, keyed.keys, keyed.mergedRowsByKey())
}

// XGroupKeyed groups a keyed frame's underlying table without first cloning it.
func XGroupKeyed(keyed KeyedFrame, keys ...Symbol) (KeyedFrame, error) {
	return XGroup(keyed.frame, keys...)
}

// UngroupKeyedFrame expands a keyed frame's underlying table without first
// cloning it.
func UngroupKeyedFrame(keyed KeyedFrame) (Frame, error) {
	return Ungroup(keyed.frame)
}

// KeyByKeyed retargets a keyed frame's key columns without cloning row data.
func KeyByKeyed(keyed KeyedFrame, keys ...Symbol) (KeyedFrame, error) {
	return KeyBy(keyed.frame, keys...)
}

func (k KeyedFrame) ValueFrame() (Frame, error) {
	if len(k.keys) == 0 {
		return Frame{}, fmt.Errorf("keyed frame is not initialized")
	}
	return DropColumns(k.Frame(), k.keys...)
}

func (k KeyedFrame) KeyFrame() (Frame, error) {
	if len(k.keys) == 0 {
		return Frame{}, fmt.Errorf("keyed frame is not initialized")
	}
	cols := make([]Column, 0, len(k.keys))
	for _, name := range k.keys {
		col, ok := k.frame.Column(name)
		if !ok {
			return Frame{}, fmt.Errorf("key column %q does not exist", name)
		}
		cols = append(cols, Column{Name: name, Data: col})
	}
	return NewFrame(cols...)
}

func (k KeyedFrame) LatestFrame() (Frame, error) {
	if len(k.keys) == 0 {
		return Frame{}, fmt.Errorf("keyed frame is not initialized")
	}
	latest := make([]int, 0, k.indexKeyCount())
	positionByKey := make(map[string]int, k.indexKeyCount())
	for row := 0; row < k.frame.Len(); row++ {
		key, err := rowKey(k.frame, row, k.keys)
		if err != nil {
			return Frame{}, err
		}
		if position, ok := positionByKey[key]; ok {
			latest[position] = row
			continue
		}
		positionByKey[key] = len(latest)
		latest = append(latest, row)
	}
	return k.frame.Gather(latest)
}

func (k KeyedFrame) Keys() []Symbol {
	return append([]Symbol(nil), k.keys...)
}

func (k KeyedFrame) IndexMetadata() KeyedIndexMetadata {
	fingerprint, err := k.indexFingerprint()
	if err != nil {
		fingerprint = 0
	}
	return KeyedIndexMetadata{
		Rows:        k.index.Rows,
		Keys:        append([]Symbol(nil), k.index.Keys...),
		SchemaHash:  k.index.SchemaHash,
		Fingerprint: fingerprint,
	}
}

func (k KeyedFrame) ValidateIndex() error {
	if len(k.keys) == 0 {
		return fmt.Errorf("keyed frame is not initialized")
	}
	if err := k.validateIndexShape(); err != nil {
		return err
	}
	current, err := buildKeyedIndexMetadata(k.frame, k.keys)
	if err != nil {
		return err
	}
	fingerprint, err := k.indexFingerprint()
	if err != nil {
		return err
	}
	if current.Fingerprint != fingerprint {
		return fmt.Errorf("keyed frame index fingerprint mismatch")
	}
	return nil
}

func (k KeyedFrame) LookupByKey(values ...any) (Frame, error) {
	if len(k.keys) == 0 {
		return Frame{}, fmt.Errorf("keyed frame is not initialized")
	}
	if err := k.validateIndexShape(); err != nil {
		return Frame{}, err
	}
	key, err := lookupKey(k.frame, k.keys, values)
	if err != nil {
		return Frame{}, err
	}
	rows := k.lookupRowsByKey(key)
	if rows == nil {
		rows = []int{}
	}
	return k.frame.Gather(rows)
}

func (k KeyedFrame) LookupValueByKey(values ...any) (Frame, error) {
	frame, err := k.LookupByKey(values...)
	if err != nil {
		return Frame{}, err
	}
	if frame.Len() > 1 {
		frame, err = frame.Gather([]int{frame.Len() - 1})
		if err != nil {
			return Frame{}, err
		}
	}
	return DropColumns(frame, k.keys...)
}

func (k KeyedFrame) LookupByKeyRecord(values map[Symbol]any) (Frame, error) {
	keyValues, err := k.keyValuesFromRecord(values)
	if err != nil {
		return Frame{}, err
	}
	return k.LookupByKey(keyValues...)
}

func (k KeyedFrame) LookupValueByKeyRecord(values map[Symbol]any) (Frame, error) {
	keyValues, err := k.keyValuesFromRecord(values)
	if err != nil {
		return Frame{}, err
	}
	return k.LookupValueByKey(keyValues...)
}

func (k KeyedFrame) Amend(delta Frame, valueColumns ...Symbol) (KeyedFrame, error) {
	return k.mutate(delta, false, valueColumns...)
}

func (k KeyedFrame) Upsert(delta Frame, valueColumns ...Symbol) (KeyedFrame, error) {
	return k.mutate(delta, true, valueColumns...)
}

func LookupByKey(keyed KeyedFrame, values ...any) (Frame, error) {
	return keyed.LookupByKey(values...)
}

func LookupValueByKey(keyed KeyedFrame, values ...any) (Frame, error) {
	return keyed.LookupValueByKey(values...)
}

func KeyFrame(keyed KeyedFrame) (Frame, error) {
	return keyed.KeyFrame()
}

func ValueFrame(keyed KeyedFrame) (Frame, error) {
	return keyed.ValueFrame()
}

func KeyValueFrames(keyed KeyedFrame) (Frame, Frame, error) {
	keys, err := keyed.KeyFrame()
	if err != nil {
		return Frame{}, Frame{}, err
	}
	values, err := keyed.ValueFrame()
	if err != nil {
		return Frame{}, Frame{}, err
	}
	return keys, values, nil
}

func LatestFrame(keyed KeyedFrame) (Frame, error) {
	return keyed.LatestFrame()
}

func AmendKeyed(keyed KeyedFrame, delta Frame, valueColumns ...Symbol) (KeyedFrame, error) {
	return keyed.Amend(delta, valueColumns...)
}

func UpsertKeyed(keyed KeyedFrame, delta Frame, valueColumns ...Symbol) (KeyedFrame, error) {
	return keyed.Upsert(delta, valueColumns...)
}

func (k KeyedFrame) mutate(delta Frame, appendMissing bool, valueColumns ...Symbol) (KeyedFrame, error) {
	if len(k.keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("keyed frame is not initialized")
	}
	if err := validateMutationKeys(k, delta); err != nil {
		return KeyedFrame{}, err
	}
	valueCols, err := keyedMutationValueColumns(k, delta, valueColumns)
	if err != nil {
		return KeyedFrame{}, err
	}
	if out, ok, err := k.mutateTyped(delta, appendMissing, valueCols); ok {
		return out, err
	}
	cols, colValues, err := keyedMutationColumns(k, delta, valueCols)
	if err != nil {
		return KeyedFrame{}, err
	}
	valueSet := make(map[Symbol]struct{}, len(valueCols))
	for _, name := range valueCols {
		valueSet[name] = struct{}{}
	}
	for row := 0; row < delta.Len(); row++ {
		key, keyValues, err := deltaRowKey(k, delta, row)
		if err != nil {
			return KeyedFrame{}, err
		}
		targetRows := k.lookupRowsByKey(key)
		if len(targetRows) > 0 {
			for _, targetRow := range targetRows {
				for _, name := range valueCols {
					v, _ := delta.columns[name].At(row)
					colValues[name][targetRow] = v
				}
			}
			continue
		}
		if !appendMissing {
			continue
		}
		for _, col := range cols {
			keyIndex := symbolIndex(k.keys, col.Name)
			switch {
			case keyIndex >= 0:
				colValues[col.Name] = append(colValues[col.Name], keyValues[keyIndex])
			case hasColumn(delta, col.Name) && hasSymbol(valueSet, col.Name):
				v, _ := delta.columns[col.Name].At(row)
				colValues[col.Name] = append(colValues[col.Name], v)
			default:
				colValues[col.Name] = append(colValues[col.Name], NullValue)
			}
		}
	}
	outCols := make([]Column, 0, len(cols))
	for _, col := range cols {
		out, err := columnWithKind(col.Name, col.Data.Kind(), colValues[col.Name])
		if err != nil {
			return KeyedFrame{}, err
		}
		outCols = append(outCols, out)
	}
	out, err := NewFrame(outCols...)
	if err != nil {
		return KeyedFrame{}, err
	}
	return KeyBy(out, k.keys...)
}

func (k KeyedFrame) keyValuesFromRecord(values map[Symbol]any) ([]any, error) {
	if len(k.keys) == 0 {
		return nil, fmt.Errorf("keyed frame is not initialized")
	}
	out := make([]any, 0, len(k.keys))
	for _, key := range k.keys {
		value, ok := values[key]
		if !ok {
			return nil, fmt.Errorf("key %q is missing", key)
		}
		out = append(out, value)
	}
	return out, nil
}

func (k KeyedFrame) validateIndexShape() error {
	if k.index.Rows != k.frame.Len() {
		return fmt.Errorf("keyed frame index rows mismatch: index has %d rows, frame has %d", k.index.Rows, k.frame.Len())
	}
	if k.index.SchemaHash != k.frame.SchemaFingerprint() {
		return fmt.Errorf("keyed frame index schema mismatch")
	}
	if !sameSymbols(k.index.Keys, k.keys) {
		return fmt.Errorf("keyed frame index key mismatch")
	}
	return nil
}

type JoinKey struct {
	Left  Symbol
	Right Symbol
}

type JoinOptions struct {
	LeftColumns  []Symbol
	RightColumns []Symbol
	OrderBy      []JoinOrderSpec
	LimitN       int
}

type JoinOrderSpec struct {
	Column Symbol
	Right  bool
	Desc   bool
}

func InnerJoin(left, right Frame, keys ...Symbol) (Frame, error) {
	joinKeys := make([]JoinKey, len(keys))
	for i, key := range keys {
		joinKeys[i] = JoinKey{Left: key, Right: key}
	}
	return InnerJoinOn(left, right, joinKeys...)
}

func LeftJoin(left, right Frame, keys ...Symbol) (Frame, error) {
	joinKeys := make([]JoinKey, len(keys))
	for i, key := range keys {
		joinKeys[i] = JoinKey{Left: key, Right: key}
	}
	return LeftJoinOn(left, right, joinKeys...)
}

func LeftJoinOn(left, right Frame, keys ...JoinKey) (Frame, error) {
	return joinOnWithOptions(left, right, true, JoinOptions{}, keys...)
}

func InnerJoinOn(left, right Frame, keys ...JoinKey) (Frame, error) {
	return joinOnWithOptions(left, right, false, JoinOptions{}, keys...)
}

func LeftJoinOnWithOptions(left, right Frame, opts JoinOptions, keys ...JoinKey) (Frame, error) {
	return joinOnWithOptions(left, right, true, opts, keys...)
}

func InnerJoinOnWithOptions(left, right Frame, opts JoinOptions, keys ...JoinKey) (Frame, error) {
	return joinOnWithOptions(left, right, false, opts, keys...)
}

func LeftJoinKeyed(left Frame, right KeyedFrame) (Frame, error) {
	return LeftJoinKeyedOn(left, right)
}

func InnerJoinKeyed(left Frame, right KeyedFrame) (Frame, error) {
	return InnerJoinKeyedOn(left, right)
}

func LeftJoinKeyedOn(left Frame, right KeyedFrame, keys ...JoinKey) (Frame, error) {
	return joinKeyedOn(left, right, true, keys...)
}

func InnerJoinKeyedOn(left Frame, right KeyedFrame, keys ...JoinKey) (Frame, error) {
	return joinKeyedOn(left, right, false, keys...)
}

func UnionJoinOn(left, right Frame, keys ...JoinKey) (Frame, error) {
	return UnionJoinOnWithOptions(left, right, JoinOptions{}, keys...)
}

// UnionJoinOnWithOptions runs a union join restricted to opts.LeftColumns /
// opts.RightColumns when set (key columns must be included by the caller);
// OrderBy/LimitN are not applied by union joins.
func UnionJoinOnWithOptions(left, right Frame, opts JoinOptions, keys ...JoinKey) (Frame, error) {
	if len(keys) == 0 {
		return Frame{}, fmt.Errorf("union join requires at least one key")
	}
	if err := validateJoinKeys(left, right, keys); err != nil {
		return Frame{}, err
	}
	leftIndexes, rightIndexes, typedMatch := unionJoinIndexesTypedFast(left, right, keys)
	if !typedMatch {
		rightRowsByKey, _, err := rightRowsByJoinKey(right, keys)
		if err != nil {
			return Frame{}, err
		}
		leftKeyCols := leftKeyColumns(keys)

		leftIndexes = make([]int, 0, left.Len()+right.Len())
		rightIndexes = make([]int, 0, left.Len()+right.Len())
		matchedRight := make([]bool, right.Len())
		for row := 0; row < left.Len(); row++ {
			key, err := rowKey(left, row, leftKeyCols)
			if err != nil {
				return Frame{}, err
			}
			matches := rightRowsByKey[key]
			if len(matches) == 0 {
				leftIndexes = append(leftIndexes, row)
				rightIndexes = append(rightIndexes, -1)
				continue
			}
			for _, rightRow := range matches {
				leftIndexes = append(leftIndexes, row)
				rightIndexes = append(rightIndexes, rightRow)
				matchedRight[rightRow] = true
			}
		}
		for row := 0; row < right.Len(); row++ {
			if matchedRight[row] {
				continue
			}
			leftIndexes = append(leftIndexes, -1)
			rightIndexes = append(rightIndexes, row)
		}
	}

	leftNames := joinOutputLeftColumns(left, opts.LeftColumns)
	rightNames := joinOutputRightColumns(right, opts.RightColumns)
	cols := make([]Column, 0, len(leftNames)+len(rightNames))
	usedNames := make(map[Symbol]struct{}, len(leftNames)+len(rightNames))
	rightByLeftKey := make(map[Symbol]Symbol, len(keys))
	rightKeys := make(map[Symbol]struct{}, len(keys))
	for _, key := range keys {
		rightByLeftKey[key.Left] = key.Right
		rightKeys[key.Right] = struct{}{}
	}
	for _, name := range leftNames {
		leftCol := left.columns[name]
		rightName, isKey := rightByLeftKey[name]
		if !isKey {
			cols = append(cols, Column{Name: name, Data: gatherOptional(leftCol, leftIndexes)})
			usedNames[name] = struct{}{}
			continue
		}
		if col, ok := unionCoalesceKeyTyped(leftCol, leftIndexes, right.columns[rightName], rightIndexes); ok {
			cols = append(cols, Column{Name: name, Data: col})
			usedNames[name] = struct{}{}
			continue
		}
		values := make([]any, len(leftIndexes))
		for i, leftRow := range leftIndexes {
			if leftRow >= 0 {
				v, ok := leftCol.At(leftRow)
				if !ok {
					return Frame{}, fmt.Errorf("union join left column %q row %d out of range", name, leftRow)
				}
				values[i] = v
				continue
			}
			rightCol := right.columns[rightName]
			v, ok := rightCol.At(rightIndexes[i])
			if !ok {
				return Frame{}, fmt.Errorf("union join right key column %q row %d out of range", rightName, rightIndexes[i])
			}
			values[i] = v
		}
		col, err := columnWithKind(name, leftCol.Kind(), values)
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, col)
		usedNames[name] = struct{}{}
	}
	for _, name := range rightNames {
		if _, isJoinKey := rightKeys[name]; isJoinKey {
			continue
		}
		outName := rightJoinColumnName(name, usedNames)
		cols = append(cols, Column{Name: outName, Data: gatherOptional(right.columns[name], rightIndexes)})
		usedNames[outName] = struct{}{}
	}
	return NewFrame(cols...)
}

func PlusJoinOn(left, right Frame, keys ...JoinKey) (Frame, error) {
	return PlusJoinOnWithOptions(left, right, JoinOptions{}, keys...)
}

// PlusJoinOnWithOptions runs a plus join restricted to opts.LeftColumns /
// opts.RightColumns when set. Shared (added) column names must be kept on
// both sides by the caller for the add semantics to hold; OrderBy/LimitN are
// not applied by plus joins.
func PlusJoinOnWithOptions(left, right Frame, opts JoinOptions, keys ...JoinKey) (Frame, error) {
	if len(keys) == 0 {
		return Frame{}, fmt.Errorf("plus join requires at least one key")
	}
	if err := validateJoinKeys(left, right, keys); err != nil {
		return Frame{}, err
	}
	leftKeys := make(map[Symbol]struct{}, len(keys))
	rightKeys := make(map[Symbol]struct{}, len(keys))
	for _, key := range keys {
		leftKeys[key.Left] = struct{}{}
		rightKeys[key.Right] = struct{}{}
	}

	matchedRight, typedMatch := plusJoinMatchTypedFast(left, right, keys)
	if !typedMatch {
		rightRowsByKey, _, err := rightRowsByJoinKey(right, keys)
		if err != nil {
			return Frame{}, err
		}
		leftKeyCols := leftKeyColumns(keys)
		matchedRight = make([]int, left.Len())
		for i := range matchedRight {
			matchedRight[i] = -1
		}
		for row := 0; row < left.Len(); row++ {
			key, err := rowKey(left, row, leftKeyCols)
			if err != nil {
				return Frame{}, err
			}
			matches := rightRowsByKey[key]
			if len(matches) > 0 {
				matchedRight[row] = matches[0]
			}
		}
	}

	leftNames := joinOutputLeftColumns(left, opts.LeftColumns)
	rightNames := joinOutputRightColumns(right, opts.RightColumns)
	cols := make([]Column, 0, len(leftNames)+len(rightNames))
	usedNames := make(map[Symbol]struct{}, len(leftNames)+len(rightNames))
	for _, name := range leftNames {
		leftCol := left.columns[name]
		rightCol, hasRight := right.Column(name)
		_, isLeftKey := leftKeys[name]
		if hasRight && !isLeftKey {
			if col, ok := plusAddSharedTyped(leftCol, rightCol, matchedRight); ok {
				cols = append(cols, Column{Name: name, Data: col})
				usedNames[name] = struct{}{}
				continue
			}
			values := make([]any, left.Len())
			for row := 0; row < left.Len(); row++ {
				leftValue, ok := leftCol.At(row)
				if !ok {
					return Frame{}, fmt.Errorf("plus join left column %q row %d out of range", name, row)
				}
				rightRow := matchedRight[row]
				if rightRow < 0 {
					values[row] = leftValue
					continue
				}
				rightValue, ok := rightCol.At(rightRow)
				if !ok {
					return Frame{}, fmt.Errorf("plus join right column %q row %d out of range", name, rightRow)
				}
				sum, err := ApplyBinary(OpAdd, leftValue, rightValue)
				if err != nil {
					return Frame{}, fmt.Errorf("plus join column %q: %w", name, err)
				}
				values[row] = sum
			}
			cols = append(cols, NewColumn(name, values))
		} else {
			// Frames are immutable values, so untouched left columns are
			// adopted without a defensive gather copy.
			cols = append(cols, Column{Name: name, Data: leftCol})
		}
		usedNames[name] = struct{}{}
	}
	for _, name := range rightNames {
		if _, isJoinKey := rightKeys[name]; isJoinKey {
			continue
		}
		if _, exists := usedNames[name]; exists {
			continue
		}
		outName := rightJoinColumnName(name, usedNames)
		cols = append(cols, Column{Name: outName, Data: gatherOptional(right.columns[name], matchedRight)})
		usedNames[outName] = struct{}{}
	}
	return NewFrame(cols...)
}

func UnionJoin(left, right Frame) (Frame, error) {
	names := make([]Symbol, 0, len(left.schema.names)+len(right.schema.names))
	seen := make(map[Symbol]struct{}, len(left.schema.names)+len(right.schema.names))
	for _, name := range left.schema.names {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for _, name := range right.schema.names {
		if _, ok := seen[name]; ok {
			continue
		}
		names = append(names, name)
		seen[name] = struct{}{}
	}
	cols := make([]Column, 0, len(names))
	for _, name := range names {
		leftCol, hasLeft := left.Column(name)
		rightCol, hasRight := right.Column(name)
		kind := KindAny
		switch {
		case hasLeft && hasRight:
			if leftCol.Kind() != rightCol.Kind() {
				return Frame{}, fmt.Errorf("union join column %q kind mismatch: %s vs %s", name, leftCol.Kind(), rightCol.Kind())
			}
			kind = leftCol.Kind()
		case hasLeft:
			kind = leftCol.Kind()
		case hasRight:
			kind = rightCol.Kind()
		}
		values := make([]any, 0, left.Len()+right.Len())
		for row := 0; row < left.Len(); row++ {
			if !hasLeft {
				values = append(values, NullValue)
				continue
			}
			v, ok := leftCol.At(row)
			if !ok {
				return Frame{}, fmt.Errorf("union join left column %q row %d out of range", name, row)
			}
			values = append(values, v)
		}
		for row := 0; row < right.Len(); row++ {
			if !hasRight {
				values = append(values, NullValue)
				continue
			}
			v, ok := rightCol.At(row)
			if !ok {
				return Frame{}, fmt.Errorf("union join right column %q row %d out of range", name, row)
			}
			values = append(values, v)
		}
		col, err := columnWithKind(name, kind, values)
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, col)
	}
	return NewFrame(cols...)
}

func joinOnWithOptions(left, right Frame, keepUnmatchedLeft bool, opts JoinOptions, keys ...JoinKey) (Frame, error) {
	if len(keys) == 0 {
		return Frame{}, fmt.Errorf("join requires at least one key")
	}
	if err := validateJoinKeys(left, right, keys); err != nil {
		return Frame{}, err
	}
	if out, handled, err := tryAlignedSingleColumnJoinOn(left, right, keepUnmatchedLeft, opts, keys); handled || err != nil {
		return out, err
	}

	leftIndexes, rightIndexes, ok, err := joinIndexesWithOptions(left, right, keepUnmatchedLeft, opts, keys)
	if err != nil {
		return Frame{}, err
	}
	if !ok {
		leftIndexes, rightIndexes, err = typedKernels.JoinIndexes(left, right, keepUnmatchedLeft, keys)
		if err != nil {
			return Frame{}, err
		}
		leftIndexes, rightIndexes, err = limitJoinIndexesByOrder(left, right, leftIndexes, rightIndexes, opts)
		if err != nil {
			return Frame{}, err
		}
	}

	leftNames := joinOutputLeftColumns(left, opts.LeftColumns)
	rightNames := joinOutputRightColumns(right, opts.RightColumns)
	cols := make([]Column, 0, len(leftNames)+len(rightNames))
	usedNames := make(map[Symbol]struct{}, len(left.schema.names)+len(right.schema.names))
	for _, name := range left.schema.names {
		usedNames[name] = struct{}{}
	}
	for _, name := range leftNames {
		cols = append(cols, Column{Name: name, Data: joinGather(left.columns[name], leftIndexes)})
	}

	rightKeys := make(map[Symbol]struct{}, len(keys))
	for _, key := range keys {
		rightKeys[key.Right] = struct{}{}
	}
	for _, name := range rightNames {
		if _, isJoinKey := rightKeys[name]; isJoinKey {
			continue
		}
		outName := rightJoinColumnName(name, usedNames)
		if keepUnmatchedLeft {
			cols = append(cols, Column{Name: outName, Data: joinGatherOptional(right.columns[name], rightIndexes)})
		} else {
			cols = append(cols, Column{Name: outName, Data: joinGather(right.columns[name], rightIndexes)})
		}
		usedNames[outName] = struct{}{}
	}
	return newFrameTrusted(cols...)
}

func tryAlignedSingleColumnJoinOn(left, right Frame, keepUnmatchedLeft bool, opts JoinOptions, keys []JoinKey) (Frame, bool, error) {
	if len(keys) != 1 || opts.LimitN > 0 || len(opts.OrderBy) > 0 {
		return Frame{}, false, nil
	}
	leftKey, ok := left.Column(keys[0].Left)
	if !ok {
		return Frame{}, true, fmt.Errorf("join left key column %q does not exist", keys[0].Left)
	}
	rightKey, ok := right.Column(keys[0].Right)
	if !ok {
		return Frame{}, true, fmt.Errorf("join right key column %q does not exist", keys[0].Right)
	}
	leftIndexes, rightIndexes, ok := alignedJoinIndexArrays(leftKey, rightKey)
	if !ok {
		return Frame{}, false, nil
	}

	leftNames := joinOutputLeftColumns(left, opts.LeftColumns)
	rightNames := joinOutputRightColumns(right, opts.RightColumns)
	cols := make([]Column, 0, len(leftNames)+len(rightNames))
	usedNames := make(map[Symbol]struct{}, len(left.schema.names)+len(right.schema.names))
	for _, name := range left.schema.names {
		usedNames[name] = struct{}{}
	}
	for _, name := range leftNames {
		cols = append(cols, Column{Name: name, Data: joinGatherByIndexArray(left.columns[name], leftIndexes)})
	}

	rightKeys := map[Symbol]struct{}{keys[0].Right: {}}
	for _, name := range rightNames {
		if _, isJoinKey := rightKeys[name]; isJoinKey {
			continue
		}
		outName := rightJoinColumnName(name, usedNames)
		cols = append(cols, Column{Name: outName, Data: joinGatherByIndexArray(right.columns[name], rightIndexes)})
		usedNames[outName] = struct{}{}
	}
	_ = keepUnmatchedLeft
	frame, err := newFrameTrusted(cols...)
	return frame, true, err
}

func alignedJoinIndexArrays(left, right Array) (Array, Array, bool) {
	left = unwrapAttributedArray(left)
	right = unwrapAttributedArray(right)
	if left.Kind() != right.Kind() || left.Len() != right.Len() {
		return nil, nil, false
	}
	if left.Len() == 0 {
		empty := i64RangeArray{len: 0}
		return empty, empty, true
	}
	if !arraysEqualUniqueAtRows(left, right) {
		return nil, nil, false
	}
	indexes := i64RangeArray{start: 0, step: 1, len: left.Len()}
	return indexes, indexes, true
}

func arraysEqualUniqueAtRows(left, right Array) bool {
	switch l := left.(type) {
	case columnArray[bool]:
		r, ok := right.(columnArray[bool])
		return ok && slicesEqualUnique(l.data, r.data)
	case columnArray[int8]:
		r, ok := right.(columnArray[int8])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[int16]:
		r, ok := right.(columnArray[int16])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[int32]:
		r, ok := right.(columnArray[int32])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[int64]:
		r, ok := right.(columnArray[int64])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[uint8]:
		r, ok := right.(columnArray[uint8])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[uint16]:
		r, ok := right.(columnArray[uint16])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[uint32]:
		r, ok := right.(columnArray[uint32])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[uint64]:
		r, ok := right.(columnArray[uint64])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[float32]:
		r, ok := right.(columnArray[float32])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[float64]:
		r, ok := right.(columnArray[float64])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[string]:
		r, ok := right.(columnArray[string])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Symbol]:
		r, ok := right.(columnArray[Symbol])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Month]:
		r, ok := right.(columnArray[Month])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Date]:
		r, ok := right.(columnArray[Date])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[DateTime]:
		r, ok := right.(columnArray[DateTime])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Timespan]:
		r, ok := right.(columnArray[Timespan])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Minute]:
		r, ok := right.(columnArray[Minute])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Second]:
		r, ok := right.(columnArray[Second])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Time]:
		r, ok := right.(columnArray[Time])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case columnArray[Timestamp]:
		r, ok := right.(columnArray[Timestamp])
		return ok && slicesEqualStrictlyOrdered(l.data, r.data)
	case i64RangeArray:
		r, ok := right.(i64RangeArray)
		return ok && l.start == r.start && l.step == r.step && l.len == r.len && l.step != 0
	default:
		return false
	}
}

func slicesEqualUnique[T comparable](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[T]struct{}, len(left))
	for i, value := range left {
		if right[i] != value {
			return false
		}
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

type joinOrderedScalar interface {
	~int8 | ~int16 | ~int32 | ~int64 |
		~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~string
}

func slicesEqualStrictlyOrdered[T joinOrderedScalar](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	switch len(left) {
	case 0:
		return true
	case 1:
		return left[0] == right[0]
	}
	if left[0] != right[0] || left[1] != right[1] || left[0] == left[1] {
		return false
	}
	ascending := left[0] < left[1]
	for i := 2; i < len(left); i++ {
		if left[i] != right[i] {
			return false
		}
		if ascending {
			if left[i-1] >= left[i] {
				return false
			}
			continue
		}
		if left[i-1] <= left[i] {
			return false
		}
	}
	return true
}

func joinIndexesWithOptions(left, right Frame, keepUnmatchedLeft bool, opts JoinOptions, keys []JoinKey) ([]int, []int, bool, error) {
	if keepUnmatchedLeft || opts.LimitN <= 0 || len(opts.OrderBy) != 1 || len(keys) != 1 {
		return nil, nil, false, nil
	}
	leftKey, ok := left.Column(keys[0].Left)
	if !ok {
		return nil, nil, true, fmt.Errorf("join left key column %q does not exist", keys[0].Left)
	}
	rightKey, ok := right.Column(keys[0].Right)
	if !ok {
		return nil, nil, true, fmt.Errorf("join right key column %q does not exist", keys[0].Right)
	}
	if leftKey.Kind() != rightKey.Kind() {
		return nil, nil, false, nil
	}
	if _, ok := arrayIndexForBorrowed(rightKey, ArrayAttributeUnique); !ok {
		if _, ok := arrayIndexForBorrowed(rightKey, ArrayAttributeGrouped); !ok {
			rightKey = WithArrayAttribute(rightKey, ArrayAttributeGrouped)
			right.columns[keys[0].Right] = rightKey
		}
	}
	orderSpec := opts.OrderBy[0]
	orderFrame := left
	if orderSpec.Right {
		orderFrame = right
	}
	orderColumn, ok := orderFrame.Column(orderSpec.Column)
	if !ok {
		return nil, nil, true, fmt.Errorf("join order column %q does not exist", orderSpec.Column)
	}
	return singleColumnTypedJoinTopKIndexes(leftKey, rightKey, orderColumn, orderSpec.Right, orderSpec.Desc, opts.LimitN)
}

func singleColumnTypedJoinTopKIndexes(leftKey, rightKey, orderColumn Array, orderRight bool, desc bool, limit int) ([]int, []int, bool, error) {
	switch left := leftKey.(type) {
	case attributedArray:
		return singleColumnTypedJoinTopKIndexes(left.array, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[bool]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[int8]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[int16]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[int32]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[int64]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[uint8]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[uint16]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[uint32]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[uint64]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[float32]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[float64]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[string]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Symbol]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Month]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Date]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[DateTime]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Timespan]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Minute]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Second]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Time]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	case columnArray[Timestamp]:
		return singleColumnTypedJoinTopKIndexesFor(left, rightKey, orderColumn, orderRight, desc, limit)
	default:
		return nil, nil, false, nil
	}
}

func singleColumnTypedJoinTopKIndexesFor[T comparable](leftKey columnArray[T], rightKey Array, orderColumn Array, orderRight bool, desc bool, limit int) ([]int, []int, bool, error) {
	rightKey = unwrapAttributedArray(rightKey)
	right, ok := rightKey.(columnArray[T])
	if !ok {
		return nil, nil, false, nil
	}
	rowsByKey, ok := typedRowsByKeyForArray[T](rightKey)
	if !ok {
		rowsByKey = typedRowsByKeyFor(right.data)
	}
	spec := boundOrderSpec{spec: OrderSpec{Desc: desc}, column: orderColumn}
	before := makeJoinTopKBefore(spec)
	h := &joinTopKHeap{items: make([]joinTopKPair, 0, limit), spec: spec, before: before}
	seq := 0
	for leftRow, value := range leftKey.data {
		for _, rightRow := range rowsByKey[value] {
			orderRow := leftRow
			if orderRight {
				orderRow = rightRow
			}
			pair := joinTopKPair{left: leftRow, right: rightRow, orderRow: orderRow, seq: seq}
			seq++
			if h.Len() < limit {
				h.push(pair)
				continue
			}
			if before(pair, h.items[0]) {
				h.items[0] = pair
				h.fixRoot()
			}
		}
	}
	out := h.items
	sort.SliceStable(out, func(i, j int) bool {
		return before(out[i], out[j])
	})
	leftIndexes := make([]int, len(out))
	rightIndexes := make([]int, len(out))
	for i, pair := range out {
		leftIndexes[i] = pair.left
		rightIndexes[i] = pair.right
	}
	return leftIndexes, rightIndexes, true, nil
}

func typedRowsByKeyForArray[T comparable](array Array) (map[T][]int, bool) {
	if index, ok := arrayIndexForBorrowed(array, ArrayAttributeUnique); ok {
		if rowsByKey, ok := index.typedRowsByKey.(map[T][]int); ok {
			return rowsByKey, true
		}
	}
	if index, ok := arrayIndexForBorrowed(array, ArrayAttributeGrouped); ok {
		if rowsByKey, ok := index.typedRowsByKey.(map[T][]int); ok {
			return rowsByKey, true
		}
	}
	return nil, false
}

func limitJoinIndexesByOrder(left, right Frame, leftIndexes, rightIndexes []int, opts JoinOptions) ([]int, []int, error) {
	if opts.LimitN <= 0 || opts.LimitN >= len(leftIndexes) {
		return leftIndexes, rightIndexes, nil
	}
	if opts.LimitN == 0 {
		return []int{}, []int{}, nil
	}
	if len(opts.OrderBy) != 1 {
		return leftIndexes, rightIndexes, nil
	}
	spec := opts.OrderBy[0]
	frame := left
	indexes := leftIndexes
	if spec.Right {
		frame = right
		indexes = rightIndexes
		for _, row := range rightIndexes {
			if row < 0 {
				return leftIndexes, rightIndexes, nil
			}
		}
	}
	column, ok := frame.Column(spec.Column)
	if !ok {
		return nil, nil, fmt.Errorf("join order column %q does not exist", spec.Column)
	}
	pairs := topKJoinPairs(leftIndexes, rightIndexes, indexes, boundOrderSpec{
		spec:   OrderSpec{Column: spec.Column, Desc: spec.Desc},
		column: column,
	}, opts.LimitN)
	outLeft := make([]int, len(pairs))
	outRight := make([]int, len(pairs))
	for i, pair := range pairs {
		outLeft[i] = pair.left
		outRight[i] = pair.right
	}
	return outLeft, outRight, nil
}

type joinTopKPair struct {
	left     int
	right    int
	orderRow int
	seq      int
}

type joinTopKHeap struct {
	items  []joinTopKPair
	spec   boundOrderSpec
	before func(left, right joinTopKPair) bool
}

func (h joinTopKHeap) Len() int { return len(h.items) }

func (h joinTopKHeap) Less(i, j int) bool {
	if h.before != nil {
		return h.before(h.items[j], h.items[i])
	}
	return joinTopKBefore(h.spec, h.items[j], h.items[i])
}

func (h joinTopKHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *joinTopKHeap) push(item joinTopKPair) {
	h.items = append(h.items, item)
	for child := len(h.items) - 1; child > 0; {
		parent := (child - 1) / 2
		if !h.less(child, parent) {
			break
		}
		h.items[child], h.items[parent] = h.items[parent], h.items[child]
		child = parent
	}
}

func (h *joinTopKHeap) fixRoot() {
	for parent := 0; ; {
		left := parent*2 + 1
		if left >= len(h.items) {
			return
		}
		child := left
		if right := left + 1; right < len(h.items) && h.less(right, left) {
			child = right
		}
		if !h.less(child, parent) {
			return
		}
		h.items[parent], h.items[child] = h.items[child], h.items[parent]
		parent = child
	}
}

func (h joinTopKHeap) less(i, j int) bool {
	if h.before != nil {
		return h.before(h.items[j], h.items[i])
	}
	return joinTopKBefore(h.spec, h.items[j], h.items[i])
}

func topKJoinPairs(leftIndexes, rightIndexes, orderRows []int, spec boundOrderSpec, limit int) []joinTopKPair {
	before := makeJoinTopKBefore(spec)
	h := &joinTopKHeap{items: make([]joinTopKPair, 0, limit), spec: spec, before: before}
	for seq, orderRow := range orderRows {
		pair := joinTopKPair{left: leftIndexes[seq], right: rightIndexes[seq], orderRow: orderRow, seq: seq}
		if h.Len() < limit {
			h.push(pair)
			continue
		}
		if before(pair, h.items[0]) {
			h.items[0] = pair
			h.fixRoot()
		}
	}
	out := h.items
	sort.SliceStable(out, func(i, j int) bool {
		return before(out[i], out[j])
	})
	return out
}

func joinTopKBefore(spec boundOrderSpec, left, right joinTopKPair) bool {
	cmp := compareArrayRows(spec.column, left.orderRow, right.orderRow)
	if cmp != 0 {
		if spec.spec.Desc {
			return cmp > 0
		}
		return cmp < 0
	}
	return left.seq < right.seq
}

func makeJoinTopKBefore(spec boundOrderSpec) func(left, right joinTopKPair) bool {
	switch a := spec.column.(type) {
	case attributedArray:
		return makeJoinTopKBefore(boundOrderSpec{spec: spec.spec, column: a.array})
	case columnArray[bool]:
		return makeJoinTopKBeforeBy(a.data, spec.spec.Desc, compareBool)
	case columnArray[int8]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[int16]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[int32]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[int64]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[uint8]:
		return makeJoinTopKBeforeUnsigned(a.data, spec.spec.Desc)
	case columnArray[uint16]:
		return makeJoinTopKBeforeUnsigned(a.data, spec.spec.Desc)
	case columnArray[uint32]:
		return makeJoinTopKBeforeUnsigned(a.data, spec.spec.Desc)
	case columnArray[uint64]:
		return makeJoinTopKBeforeUnsigned(a.data, spec.spec.Desc)
	case columnArray[float32]:
		return makeJoinTopKBeforeFloat(a.data, spec.spec.Desc)
	case columnArray[float64]:
		return makeJoinTopKBeforeFloat(a.data, spec.spec.Desc)
	case columnArray[string]:
		return makeJoinTopKBeforeBy(a.data, spec.spec.Desc, compareString)
	case columnArray[Symbol]:
		return makeJoinTopKBeforeBy(a.data, spec.spec.Desc, func(left, right Symbol) int {
			return compareString(string(left), string(right))
		})
	case columnArray[Month]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[Date]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[DateTime]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[Timespan]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[Minute]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[Second]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[Time]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	case columnArray[Timestamp]:
		return makeJoinTopKBeforeSigned(a.data, spec.spec.Desc)
	default:
		return func(left, right joinTopKPair) bool {
			return joinTopKBefore(spec, left, right)
		}
	}
}

func makeJoinTopKBeforeBy[T any](values []T, desc bool, compare func(T, T) int) func(left, right joinTopKPair) bool {
	return func(left, right joinTopKPair) bool {
		cmp := compare(values[left.orderRow], values[right.orderRow])
		if cmp != 0 {
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return left.seq < right.seq
	}
}

func makeJoinTopKBeforeSigned[T signedScalar](values []T, desc bool) func(left, right joinTopKPair) bool {
	return func(left, right joinTopKPair) bool {
		cmp := compareInt64(int64(values[left.orderRow]), int64(values[right.orderRow]))
		if cmp != 0 {
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return left.seq < right.seq
	}
}

func makeJoinTopKBeforeUnsigned[T unsignedScalar](values []T, desc bool) func(left, right joinTopKPair) bool {
	return func(left, right joinTopKPair) bool {
		cmp := compareUint64(uint64(values[left.orderRow]), uint64(values[right.orderRow]))
		if cmp != 0 {
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return left.seq < right.seq
	}
}

func makeJoinTopKBeforeFloat[T floatScalar](values []T, desc bool) func(left, right joinTopKPair) bool {
	return func(left, right joinTopKPair) bool {
		cmp := compareFloat64(float64(values[left.orderRow]), float64(values[right.orderRow]))
		if cmp != 0 {
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return left.seq < right.seq
	}
}

func joinOutputLeftColumns(frame Frame, requested []Symbol) []Symbol {
	if len(requested) == 0 {
		return append([]Symbol(nil), frame.schema.names...)
	}
	out := make([]Symbol, 0, len(requested))
	seen := make(map[Symbol]struct{}, len(requested))
	for _, name := range requested {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		if _, ok := frame.columns[name]; !ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func joinOutputRightColumns(frame Frame, requested []Symbol) []Symbol {
	if len(requested) == 0 {
		return append([]Symbol(nil), frame.schema.names...)
	}
	out := make([]Symbol, 0, len(requested))
	seen := make(map[Symbol]struct{}, len(requested))
	for _, name := range requested {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		if _, ok := frame.columns[name]; !ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func joinKeyedOn(left Frame, right KeyedFrame, keepUnmatchedLeft bool, keys ...JoinKey) (Frame, error) {
	if len(right.keys) == 0 {
		return Frame{}, fmt.Errorf("keyed join right frame is not initialized")
	}
	if len(keys) == 0 {
		keys = make([]JoinKey, len(right.keys))
		for i, key := range right.keys {
			keys[i] = JoinKey{Left: key, Right: key}
		}
	}
	if len(keys) != len(right.keys) {
		return Frame{}, fmt.Errorf("keyed join has %d keys, want %d", len(keys), len(right.keys))
	}
	for i, key := range keys {
		if key.Right != right.keys[i] {
			return Frame{}, fmt.Errorf("keyed join right key %d is %q, want %q", i+1, key.Right, right.keys[i])
		}
	}
	rightFrame, err := right.LatestFrame()
	if err != nil {
		return Frame{}, err
	}
	return joinOnWithOptions(left, rightFrame, keepUnmatchedLeft, JoinOptions{}, keys...)
}

func AsofJoin(left, right Frame, timeKey Symbol, partitionKeys ...Symbol) (Frame, error) {
	joinKeys := make([]JoinKey, len(partitionKeys))
	for i, key := range partitionKeys {
		joinKeys[i] = JoinKey{Left: key, Right: key}
	}
	return AsofJoinOn(left, right, JoinKey{Left: timeKey, Right: timeKey}, joinKeys...)
}

func AsofJoinOn(left, right Frame, timeKey JoinKey, partitionKeys ...JoinKey) (Frame, error) {
	return AsofJoinOnWithOptions(left, right, AsofJoinOptions{TimeKey: timeKey, PartitionKeys: partitionKeys})
}

type AsofJoinOptions struct {
	TimeKey           JoinKey
	PartitionKeys     []JoinKey
	PreserveRightTime bool
}

func AsofJoinOnWithOptions(left, right Frame, opts AsofJoinOptions) (Frame, error) {
	timeKey := opts.TimeKey
	partitionKeys := opts.PartitionKeys
	if err := validateAsofJoinKeys(left, right, timeKey, partitionKeys); err != nil {
		return Frame{}, err
	}

	rightTime, _ := right.Column(timeKey.Right)
	rightPartitionCols := make([]Symbol, len(partitionKeys))
	for i, key := range partitionKeys {
		rightPartitionCols[i] = key.Right
	}
	leftTime, _ := left.Column(timeKey.Left)
	leftPartitionCols := make([]Symbol, len(partitionKeys))
	for i, key := range partitionKeys {
		leftPartitionCols[i] = key.Left
	}
	rightIndexes, matched := asofMatchIndexesTypedFast(left, leftTime, leftPartitionCols, right, rightTime, rightPartitionCols)
	if matched {
		// The typed match vector is pooled and consumed entirely by the
		// gathers below (every gather copies), so it goes back to the pool
		// once the output columns are materialized.
		defer bulkIntRelease(rightIndexes)
	} else {
		rightByPartition, err := typedKernels.SortedRowsByPartition(right, rightTime, rightPartitionCols)
		if err != nil {
			return Frame{}, err
		}
		rightIndexes, err = typedKernels.AsofMatchIndexes(left, leftTime, leftPartitionCols, rightTime, rightByPartition, joinRightKeyKinds(right, partitionKeys))
		if err != nil {
			return Frame{}, err
		}
	}

	cols := make([]Column, 0, len(left.schema.names)+len(right.schema.names))
	usedNames := make(map[Symbol]struct{}, len(left.schema.names)+len(right.schema.names))
	for _, name := range left.schema.names {
		col := left.columns[name]
		if opts.PreserveRightTime && name == timeKey.Left {
			col = joinGatherOptional(right.columns[timeKey.Right], rightIndexes)
		}
		cols = append(cols, Column{Name: name, Data: col})
		usedNames[name] = struct{}{}
	}

	rightKeys := make(map[Symbol]struct{}, len(partitionKeys)+1)
	rightKeys[timeKey.Right] = struct{}{}
	for _, key := range partitionKeys {
		rightKeys[key.Right] = struct{}{}
	}
	for _, name := range right.schema.names {
		if _, isJoinKey := rightKeys[name]; isJoinKey {
			continue
		}
		outName := rightJoinColumnName(name, usedNames)
		cols = append(cols, Column{Name: outName, Data: joinGatherOptional(right.columns[name], rightIndexes)})
		usedNames[outName] = struct{}{}
	}
	return NewFrameAdoptingColumns(cols...)
}

func WindowJoinOn(left, right Frame, timeKey JoinKey, partitionKeys ...JoinKey) (Frame, error) {
	return WindowJoinOnWithOptions(left, right, WindowJoinOptions{TimeKey: timeKey, PartitionKeys: partitionKeys})
}

type WindowJoinOptions struct {
	TimeKey       JoinKey
	PartitionKeys []JoinKey
	Low           any
	High          any
	HasBounds     bool
	Last          bool
}

func WindowJoinOnWithOptions(left, right Frame, opts WindowJoinOptions) (Frame, error) {
	timeKey := opts.TimeKey
	partitionKeys := opts.PartitionKeys
	if err := validateAsofJoinKeys(left, right, timeKey, partitionKeys); err != nil {
		return Frame{}, err
	}

	rightTime, _ := right.Column(timeKey.Right)
	rightPartitionCols := make([]Symbol, len(partitionKeys))
	for i, key := range partitionKeys {
		rightPartitionCols[i] = key.Right
	}
	leftTime, _ := left.Column(timeKey.Left)
	leftPartitionCols := make([]Symbol, len(partitionKeys))
	for i, key := range partitionKeys {
		leftPartitionCols[i] = key.Left
	}
	rightIndexes, matched := windowMatchIndexesTypedFast(left, leftTime, leftPartitionCols, right, rightTime, rightPartitionCols, opts)
	if !matched {
		rightByPartition, err := typedKernels.SortedRowsByPartition(right, rightTime, rightPartitionCols)
		if err != nil {
			return Frame{}, err
		}
		rightIndexes, err = typedKernels.WindowMatchIndexes(left, leftTime, leftPartitionCols, rightTime, rightByPartition, opts, joinRightKeyKinds(right, partitionKeys))
		if err != nil {
			return Frame{}, err
		}
	}

	cols := make([]Column, 0, len(left.schema.names)+len(right.schema.names))
	usedNames := make(map[Symbol]struct{}, len(left.schema.names)+len(right.schema.names))
	for _, name := range left.schema.names {
		cols = append(cols, Column{Name: name, Data: left.columns[name]})
		usedNames[name] = struct{}{}
	}

	rightKeys := make(map[Symbol]struct{}, len(partitionKeys)+1)
	rightKeys[timeKey.Right] = struct{}{}
	for _, key := range partitionKeys {
		rightKeys[key.Right] = struct{}{}
	}
	for _, name := range right.schema.names {
		if _, isJoinKey := rightKeys[name]; isJoinKey {
			continue
		}
		outName := rightJoinColumnName(name, usedNames)
		if opts.Last {
			cols = append(cols, Column{Name: outName, Data: gatherLastOptional(right.columns[name], rightIndexes)})
		} else {
			cols = append(cols, Column{Name: outName, Data: gatherWindowLists(right.columns[name], rightIndexes)})
		}
		usedNames[outName] = struct{}{}
	}
	return NewFrameAdoptingColumns(cols...)
}

func windowJoinAbsoluteBounds(timeValue, low, high any) (any, any, error) {
	lo, err := addWindowDelta(timeValue, low)
	if err != nil {
		return nil, nil, fmt.Errorf("lower bound: %w", err)
	}
	hi, err := addWindowDelta(timeValue, high)
	if err != nil {
		return nil, nil, fmt.Errorf("upper bound: %w", err)
	}
	if compare(lo, hi) > 0 {
		return nil, nil, fmt.Errorf("lower bound is greater than upper bound")
	}
	return lo, hi, nil
}

func addWindowDelta(base, delta any) (any, error) {
	if IsNull(base) || IsNull(delta) {
		return nil, fmt.Errorf("null bound")
	}
	if d, ok := numeric(delta); ok {
		if d != float64(int64(d)) {
			return nil, fmt.Errorf("delta must be an integer for %T", base)
		}
		return addWindowIntDelta(base, int64(d))
	}
	if d, ok := delta.(Timespan); ok {
		switch x := base.(type) {
		case DateTime:
			return DateTimeFromUnixNanos(x.UnixNanos() + d.Nanos()), nil
		case Timespan:
			return TimespanFromNanos(x.Nanos() + d.Nanos()), nil
		case Time:
			return TimeFromNanos(x.Nanos() + d.Nanos()), nil
		case Timestamp:
			return TimestampFromUnixNanos(x.UnixNanos() + d.Nanos()), nil
		default:
			return nil, fmt.Errorf("timespan delta is not valid for %T", base)
		}
	}
	return nil, fmt.Errorf("unsupported delta %T", delta)
}

func addWindowIntDelta(base any, delta int64) (any, error) {
	switch x := base.(type) {
	case int8:
		return int8(int64(x) + delta), nil
	case int16:
		return int16(int64(x) + delta), nil
	case int32:
		return int32(int64(x) + delta), nil
	case int64:
		return x + delta, nil
	case int:
		return x + int(delta), nil
	case Month:
		return MonthFromMonths(x.Months() + delta), nil
	case Date:
		return DateFromDays(x.Days() + delta), nil
	case DateTime:
		return DateTimeFromUnixNanos(x.UnixNanos() + delta), nil
	case Timespan:
		return TimespanFromNanos(x.Nanos() + delta), nil
	case Minute:
		return MinuteFromMinutes(x.Minutes() + delta), nil
	case Second:
		return SecondFromSeconds(x.Seconds() + delta), nil
	case Time:
		return TimeFromNanos(x.Nanos() + delta), nil
	case Timestamp:
		return TimestampFromUnixNanos(x.UnixNanos() + delta), nil
	case float32:
		return float32(float64(x) + float64(delta)), nil
	case float64:
		return x + float64(delta), nil
	default:
		return nil, fmt.Errorf("unsupported time value %T", base)
	}
}

type Expr interface {
	EvalRow(frame Frame, row int) (any, error)
}

type vectorProjector interface {
	EvalRows(frame Frame, indexes []int) (Array, error)
}

type ColumnRef struct {
	Name Symbol
}

func (e ColumnRef) EvalRow(frame Frame, row int) (any, error) {
	col, ok := frame.Column(e.Name)
	if !ok {
		return nil, fmt.Errorf("unknown column %q", e.Name)
	}
	v, ok := col.At(row)
	if !ok {
		return nil, fmt.Errorf("column %q row %d out of range", e.Name, row)
	}
	return v, nil
}

type Literal struct {
	Value any
}

func (e Literal) EvalRow(Frame, int) (any, error) { return e.Value, nil }

// RowIndexRef is the qSQL virtual column i: it resolves to the source column
// named "i" when one exists and otherwise to the 0-based row index.
type RowIndexRef struct{}

func (RowIndexRef) EvalRow(frame Frame, row int) (any, error) {
	if col, ok := frame.Column("i"); ok {
		v, ok := col.At(row)
		if !ok {
			return nil, fmt.Errorf("column \"i\" row %d out of range", row)
		}
		return v, nil
	}
	if row < 0 || row >= frame.Len() {
		return nil, fmt.Errorf("virtual row index %d out of range", row)
	}
	return int64(row), nil
}

// ScalarAggregateExpr is a whole-frame aggregate appearing as a scalar
// subexpression (canonical qSQL `where v=max v`). filterIndexes resolves it
// to a literal before row evaluation; the EvalRow fallback recomputes the
// aggregate per call and exists only for correctness on uncommon paths.
type ScalarAggregateExpr struct {
	Func   string
	Expr   Expr
	Weight Expr
}

func (e ScalarAggregateExpr) EvalRow(frame Frame, _ int) (any, error) {
	return evalScalarAggregateValue(frame, e)
}

func evalScalarAggregateValue(frame Frame, e ScalarAggregateExpr) (any, error) {
	inner, err := resolveScalarAggregateExprs(frame, e.Expr)
	if err != nil {
		return nil, err
	}
	weight, err := resolveScalarAggregateExprs(frame, e.Weight)
	if err != nil {
		return nil, err
	}
	agg := Aggregate{Name: Symbol(e.Func), Func: e.Func, Expr: inner, Weight: weight}
	inputs, err := bindAggregateInputs(frame, []Aggregate{agg})
	if err != nil {
		return nil, err
	}
	state := aggregateState{fn: e.Func}
	for row := 0; row < frame.Len(); row++ {
		if err := accumulateAggregate(&state, inputs[0], frame, row); err != nil {
			return nil, err
		}
	}
	value := aggregateResult(state)
	if e.Func == "sum" && sumPreservesIntegerKind(frame, agg) {
		if f, ok := value.(float64); ok {
			if n := int64(f); float64(n) == f {
				return n, nil
			}
		}
	}
	return value, nil
}

// hasScalarAggregateExpr reports whether the expression tree contains a
// ScalarAggregateExpr, so the hot filter path can skip the resolving rewrite.
func hasScalarAggregateExpr(expr Expr) bool {
	switch e := expr.(type) {
	case ScalarAggregateExpr:
		return true
	case Binary:
		return hasScalarAggregateExpr(e.Left) || hasScalarAggregateExpr(e.Right)
	case Logical:
		return hasScalarAggregateExpr(e.Left) || hasScalarAggregateExpr(e.Right)
	case Not:
		return hasScalarAggregateExpr(e.Expr)
	case Conditional:
		return hasScalarAggregateExpr(e.Cond) || hasScalarAggregateExpr(e.Then) || hasScalarAggregateExpr(e.Else)
	case In:
		return hasScalarAggregateExpr(e.Expr)
	case Within:
		return hasScalarAggregateExpr(e.Expr)
	default:
		return false
	}
}

// resolveScalarAggregateExprs replaces every ScalarAggregateExpr in the tree
// with the literal aggregate value computed over the whole frame. It returns
// the input expression unchanged when no scalar aggregates are present.
func resolveScalarAggregateExprs(frame Frame, expr Expr) (Expr, error) {
	switch e := expr.(type) {
	case nil:
		return nil, nil
	case ScalarAggregateExpr:
		value, err := evalScalarAggregateValue(frame, e)
		if err != nil {
			return nil, err
		}
		return Literal{Value: value}, nil
	case Binary:
		left, err := resolveScalarAggregateExprs(frame, e.Left)
		if err != nil {
			return nil, err
		}
		right, err := resolveScalarAggregateExprs(frame, e.Right)
		if err != nil {
			return nil, err
		}
		e.Left, e.Right = left, right
		return e, nil
	case Logical:
		left, err := resolveScalarAggregateExprs(frame, e.Left)
		if err != nil {
			return nil, err
		}
		right, err := resolveScalarAggregateExprs(frame, e.Right)
		if err != nil {
			return nil, err
		}
		e.Left, e.Right = left, right
		return e, nil
	case Not:
		inner, err := resolveScalarAggregateExprs(frame, e.Expr)
		if err != nil {
			return nil, err
		}
		e.Expr = inner
		return e, nil
	case Conditional:
		cond, err := resolveScalarAggregateExprs(frame, e.Cond)
		if err != nil {
			return nil, err
		}
		then, err := resolveScalarAggregateExprs(frame, e.Then)
		if err != nil {
			return nil, err
		}
		elseExpr, err := resolveScalarAggregateExprs(frame, e.Else)
		if err != nil {
			return nil, err
		}
		e.Cond, e.Then, e.Else = cond, then, elseExpr
		return e, nil
	case In:
		inner, err := resolveScalarAggregateExprs(frame, e.Expr)
		if err != nil {
			return nil, err
		}
		e.Expr = inner
		return e, nil
	case Within:
		inner, err := resolveScalarAggregateExprs(frame, e.Expr)
		if err != nil {
			return nil, err
		}
		e.Expr = inner
		return e, nil
	default:
		return expr, nil
	}
}

type Conditional struct {
	Cond Expr
	Then Expr
	Else Expr
}

func (e Conditional) EvalRow(frame Frame, row int) (any, error) {
	if e.Cond == nil || e.Then == nil || e.Else == nil {
		return nil, fmt.Errorf("conditional expression requires condition, then, and else")
	}
	cond, err := e.Cond.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	if IsNull(cond) {
		return e.Else.EvalRow(frame, row)
	}
	keep, ok := cond.(bool)
	if !ok {
		return nil, fmt.Errorf("conditional condition must evaluate to bool")
	}
	if keep {
		return e.Then.EvalRow(frame, row)
	}
	return e.Else.EvalRow(frame, row)
}

func (e Conditional) EvalRows(frame Frame, indexes []int) (Array, error) {
	if e.Cond == nil || e.Then == nil || e.Else == nil {
		return nil, fmt.Errorf("conditional expression requires condition, then, and else")
	}
	conds, err := evalProjectionExprRows(frame, indexes, e.Cond)
	if err != nil {
		return nil, err
	}
	thens, err := evalProjectionExprRows(frame, indexes, e.Then)
	if err != nil {
		return nil, err
	}
	elses, err := evalProjectionExprRows(frame, indexes, e.Else)
	if err != nil {
		return nil, err
	}
	if conds.Len() != len(indexes) || thens.Len() != len(indexes) || elses.Len() != len(indexes) {
		return nil, fmt.Errorf("conditional expression returned mismatched row counts")
	}
	values := make([]any, len(indexes))
	for i := range indexes {
		cond, ok := conds.At(i)
		if !ok {
			return nil, fmt.Errorf("conditional condition row %d out of range", i)
		}
		useThen := false
		if !IsNull(cond) {
			keep, ok := cond.(bool)
			if !ok {
				return nil, fmt.Errorf("conditional condition row %d is %T, want bool", i, cond)
			}
			useThen = keep
		}
		var v any
		if useThen {
			v, ok = thens.At(i)
		} else {
			v, ok = elses.At(i)
		}
		if !ok {
			return nil, fmt.Errorf("conditional branch row %d out of range", i)
		}
		values[i] = v
	}
	return inferConditionalArray(values, thens.Kind(), elses.Kind()), nil
}

func inferConditionalArray(values []any, thenKind, elseKind Kind) Array {
	if conditionalBranchesPreferSymbol(thenKind, elseKind, values) {
		if array, err := nullableArrayWithKind(KindSymbol, values); err == nil {
			return array
		}
	}
	return InferArray(values)
}

func conditionalBranchesPreferSymbol(thenKind, elseKind Kind, values []any) bool {
	if thenKind != KindSymbol && elseKind != KindSymbol {
		return false
	}
	for _, v := range values {
		if IsNull(v) {
			continue
		}
		switch v.(type) {
		case Symbol, string:
		default:
			return false
		}
	}
	return true
}

func hasAnyNull(values []any) bool {
	for _, v := range values {
		if IsNull(v) {
			return true
		}
	}
	return false
}

// VectorTransformExpr evaluates q-style whole-vector projection verbs in the
// projection row order. These transforms are order-sensitive, so QueryPlan.Exec
// calls EvalRows with the filtered/pre-ordered indexes instead of evaluating
// against the full source column and gathering afterward.
type VectorTransformExpr struct {
	Func string
	Expr Expr
	Arg  Expr
}

func (e VectorTransformExpr) EvalRow(frame Frame, row int) (any, error) {
	indexes := allIndexes(frame.Len())
	array, err := e.EvalRows(frame, indexes)
	if err != nil {
		return nil, err
	}
	v, ok := array.At(row)
	if !ok {
		return nil, fmt.Errorf("%s row %d out of range", e.Func, row)
	}
	return v, nil
}

func (e VectorTransformExpr) EvalRows(frame Frame, indexes []int) (Array, error) {
	if e.Expr == nil {
		return nil, fmt.Errorf("vector transform %q expression is nil", e.Func)
	}
	values, err := evalProjectionExprRows(frame, indexes, e.Expr)
	if err != nil {
		return nil, err
	}
	switch e.Func {
	case "null":
		mask := make([]bool, values.Len())
		handled := typedKernels.NullMask(values, mask)
		recordDataRuntimeKernelProbe("DataVectorTransformNull", vectorTransformRuntimeShape(e.Func, values), handled, nil)
		if !handled {
			for i := 0; i < values.Len(); i++ {
				v, ok := values.At(i)
				if !ok {
					return nil, fmt.Errorf("null row %d out of range", i)
				}
				mask[i] = IsNull(v)
			}
		}
		return NewBool(mask), nil
	case "rank":
		out, handled, err := TryTypedRankI64(values)
		recordDataRuntimeKernelProbe("DataVectorTransformRank", vectorTransformRuntimeShape(e.Func, values), handled, err)
		if err != nil {
			return nil, err
		}
		if handled {
			return out, nil
		}
		return rankArray(values)
	case "abs", "neg", "sqrt", "log", "exp", "sin", "cos", "tan", "asin", "acos", "atan", "reciprocal", "signum", "floor", "ceiling":
		out, ok, err := typedKernels.NumericUnary(e.Func, values)
		recordDataRuntimeKernelProbe("DataVectorTransformNumericUnary", vectorTransformRuntimeShape(e.Func, values), ok, err)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%s expects numeric values", e.Func)
		}
		return out, nil
	case "prev":
		return vectorPrev(values), nil
	case "next":
		return vectorNext(values), nil
	case "deltas":
		return vectorDeltas(values)
	case "fills":
		return vectorFills(values), nil
	case "ratios":
		return vectorRatios(values)
	case "sums":
		return vectorRunningNumeric(values, e.Func)
	case "prds":
		return vectorRunningNumeric(values, e.Func)
	case "mins", "maxs":
		return vectorRunningMinMax(values, e.Func)
	case "avgs":
		return vectorRunningNumeric(values, e.Func)
	case "xprev":
		offset, err := e.intArg(frame)
		if err != nil {
			return nil, err
		}
		return vectorXPrev(values, offset), nil
	case "moving":
		window, err := e.intArg(frame)
		if err != nil {
			return nil, err
		}
		return vectorMoving(values, window)
	default:
		return nil, fmt.Errorf("unsupported vector transform %q", e.Func)
	}
}

func vectorTransformRuntimeShape(verb string, values Array) string {
	kind := KindAny
	if values != nil {
		kind = values.Kind()
	}
	return "vector-transform/" + verb + "/" + string(kind)
}

func rankArray(values Array) (Array, error) {
	indexes := make([]int, values.Len())
	for i := range indexes {
		indexes[i] = i
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		left, ok := values.At(indexes[i])
		if !ok {
			return false
		}
		right, ok := values.At(indexes[j])
		if !ok {
			return true
		}
		return compare(left, right) < 0
	})
	out := make([]int64, len(indexes))
	for sortedPosition, originalIndex := range indexes {
		out[originalIndex] = int64(sortedPosition)
	}
	return newI64Trusted(out), nil
}

func (e VectorTransformExpr) intArg(frame Frame) (int, error) {
	if e.Arg == nil {
		return 0, fmt.Errorf("vector transform %q requires an integer argument", e.Func)
	}
	lit, ok := e.Arg.(Literal)
	if !ok {
		return 0, fmt.Errorf("vector transform %q requires a literal integer argument", e.Func)
	}
	n, ok := coerceInt64Exact(lit.Value)
	if !ok {
		return 0, fmt.Errorf("vector transform %q requires an integer argument", e.Func)
	}
	return int(n), nil
}

func vectorPrev(values Array) Array {
	return shiftedArray{source: values, offset: -1}
}

func TryTypedPrev(values Array) (Array, bool, error) {
	if values == nil {
		return nil, true, fmt.Errorf("prev values must be non-nil")
	}
	return vectorPrev(values), true, nil
}

func (a i64SparseAmendArray) Kind() Kind { return KindI64 }

func (a i64SparseAmendArray) Len() int { return a.source.Len() }

func (a i64SparseAmendArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64SparseAmendArray) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data sparse amend row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64SparseAmendArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data sparse amend gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64SparseAmendArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.Len() {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	pos := sort.SearchInts(a.indexes, row)
	if pos < len(a.indexes) && a.indexes[pos] == row {
		return a.values[pos], true, nil
	}
	return integerArrayAt(a.source, row)
}

func vectorNext(values Array) Array {
	return shiftedArray{source: values, offset: 1}
}

func TryTypedNext(values Array) (Array, bool, error) {
	if values == nil {
		return nil, true, fmt.Errorf("next values must be non-nil")
	}
	return vectorNext(values), true, nil
}

func vectorXPrev(values Array, offset int) Array {
	return shiftedArray{source: values, offset: -offset}
}

func TryTypedXPrev(values Array, offset int) (Array, bool, error) {
	if values == nil {
		return nil, true, fmt.Errorf("xprev values must be non-nil")
	}
	return vectorXPrev(values, offset), true, nil
}

func vectorDeltas(values Array) (Array, error) {
	out := make([]any, values.Len())
	for i := 0; i < values.Len(); i++ {
		current, _ := values.At(i)
		if i == 0 {
			out[i] = current
			continue
		}
		previous, _ := values.At(i - 1)
		if IsNull(current) || IsNull(previous) {
			out[i] = NullValue
			continue
		}
		delta, err := ApplyBinary(OpSub, current, previous)
		if err != nil {
			return nil, fmt.Errorf("deltas row %d: %w", i, err)
		}
		out[i] = delta
	}
	return InferArray(out), nil
}

func vectorRatios(values Array) (Array, error) {
	out := make([]any, values.Len())
	for i := 0; i < values.Len(); i++ {
		if i == 0 {
			out[i] = NullValue
			continue
		}
		current, _ := values.At(i)
		previous, _ := values.At(i - 1)
		if IsNull(current) || IsNull(previous) {
			out[i] = NullValue
			continue
		}
		ratio, err := ApplyBinary(OpDiv, current, previous)
		if err != nil {
			return nil, fmt.Errorf("ratios row %d: %w", i, err)
		}
		out[i] = ratio
	}
	return InferArray(out), nil
}

func vectorFills(values Array) Array {
	if out, ok := typedNullBitmapFills(values); ok {
		return out
	}
	if out, ok := typedForwardFillArray(values); ok {
		return out
	}
	out := make([]any, values.Len())
	var last any
	hasLast := false
	for row := range out {
		value, ok := values.At(row)
		if !ok {
			panic(fmt.Sprintf("data fills row %d out of range", row))
		}
		if !IsNull(value) {
			last = value
			hasLast = true
		}
		if hasLast {
			out[row] = last
		} else {
			out[row] = NullValue
		}
	}
	return nullableArray{kind: values.Kind(), data: out}
}

// typedForwardFillArray materializes a forward-fill into a typed column in a
// single pass. It applies when the source is a KindI64/KindF64 vector whose
// first row is non-null, so no nulls remain after filling and the result can
// use unboxed storage with O(1) row access. It reports ok=false otherwise so
// the caller falls back to the generic boxed forward pass.
func typedForwardFillArray(values Array) (Array, bool) {
	n := values.Len()
	if n == 0 {
		return nil, false
	}
	first, ok := values.At(0)
	if !ok || IsNull(first) {
		return nil, false
	}
	switch values.Kind() {
	case KindI64:
		out := make([]int64, n)
		var last int64
		for row := 0; row < n; row++ {
			value, ok := values.At(row)
			if !ok {
				return nil, false
			}
			if IsNull(value) {
				out[row] = last
				continue
			}
			v, ok := coerceInt64Exact(value)
			if !ok {
				return nil, false
			}
			last = v
			out[row] = v
		}
		return newI64Trusted(out), true
	case KindF64:
		out := make([]float64, n)
		var last float64
		for row := 0; row < n; row++ {
			value, ok := values.At(row)
			if !ok {
				return nil, false
			}
			if IsNull(value) {
				out[row] = last
				continue
			}
			v, ok := numeric(value)
			if !ok {
				return nil, false
			}
			last = v
			out[row] = v
		}
		return newF64Trusted(out), true
	}
	return nil, false
}

func TryTypedFills(values Array) (Array, bool, error) {
	if values == nil {
		return nil, true, fmt.Errorf("fills values must be non-nil")
	}
	return vectorFills(values), true, nil
}

func vectorRunningNumeric(values Array, fn string) (Array, error) {
	out := make([]any, values.Len())
	sum := 0.0
	product := 1.0
	count := int64(0)
	for i := 0; i < values.Len(); i++ {
		v, _ := values.At(i)
		if IsNull(v) {
			if count == 0 {
				out[i] = NullValue
				continue
			}
		} else {
			n, ok := numeric(v)
			if !ok {
				return nil, fmt.Errorf("%s row %d expects numeric value, got %T", fn, i, v)
			}
			switch fn {
			case "sums", "avgs":
				sum += n
			case "prds":
				product *= n
			default:
				return nil, fmt.Errorf("unsupported running numeric transform %q", fn)
			}
			count++
		}
		switch fn {
		case "sums":
			out[i] = sum
		case "prds":
			out[i] = product
		case "avgs":
			out[i] = sum / float64(count)
		}
	}
	return InferArray(out), nil
}

func vectorRunningMinMax(values Array, fn string) (Array, error) {
	out := make([]any, values.Len())
	var best any
	hasBest := false
	for i := 0; i < values.Len(); i++ {
		v, _ := values.At(i)
		if IsNull(v) {
			if hasBest {
				out[i] = best
			} else {
				out[i] = NullForKind(values.Kind())
			}
			continue
		}
		if !hasBest {
			best = v
			hasBest = true
		} else {
			cmp := compare(v, best)
			if (fn == "mins" && cmp < 0) || (fn == "maxs" && cmp > 0) {
				best = v
			}
		}
		out[i] = best
	}
	return InferArray(out), nil
}

func vectorMoving(values Array, window int) (Array, error) {
	if window <= 0 {
		return nil, fmt.Errorf("moving window must be positive")
	}
	out := make([]any, values.Len())
	for i := 0; i < values.Len(); i++ {
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		windowValues := make([]any, 0, i-start+1)
		for j := start; j <= i; j++ {
			v, _ := values.At(j)
			windowValues = append(windowValues, v)
		}
		out[i] = windowValues
	}
	return NewAny(out), nil
}

type Op string

const (
	OpAdd Op = "+"
	OpSub Op = "-"
	OpMul Op = "*"
	OpDiv Op = "/"
	// OpIDiv is q integer `div`: floor division over integer operands.
	OpIDiv Op = "div"
	OpMod  Op = "mod"
	OpEQ   Op = "="
	OpNE   Op = "!="
	OpLT   Op = "<"
	OpLE   Op = "<="
	OpGT   Op = ">"
	OpGE   Op = ">="
)

type Binary struct {
	Op          Op
	Left, Right Expr
}

func (e Binary) EvalRow(frame Frame, row int) (any, error) {
	left, err := e.Left.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	right, err := e.Right.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	return ApplyBinary(e.Op, left, right)
}

func ApplyBinary(op Op, left, right any) (any, error) {
	if out, ok, err := typedKernels.Dyadic(op, left, right); ok || err != nil {
		return out, err
	}
	switch op {
	case OpEQ:
		return equalScalar(left, right), nil
	case OpNE:
		return !equalScalar(left, right), nil
	}
	if IsNull(left) || IsNull(right) {
		switch op {
		case OpAdd, OpSub, OpMul, OpDiv, OpMod:
			return promotedNullForBinary(left, right), nil
		case OpLT, OpLE, OpGT, OpGE:
			return nullOrderedCompare(op, IsNull(left), IsNull(right)), nil
		}
	}
	if cmp, ok := compareSameKind(left, right); ok {
		switch op {
		case OpLT:
			return cmp < 0, nil
		case OpLE:
			return cmp <= 0, nil
		case OpGT:
			return cmp > 0, nil
		case OpGE:
			return cmp >= 0, nil
		}
	}
	lf, lok := numeric(left)
	rf, rok := numeric(right)
	if !lok || !rok {
		return nil, fmt.Errorf("operator %s expects numeric operands", op)
	}
	switch op {
	case OpAdd:
		return lf + rf, nil
	case OpSub:
		return lf - rf, nil
	case OpMul:
		return lf * rf, nil
	case OpDiv:
		return lf / rf, nil
	case OpMod:
		if rf == 0 {
			return NullValue, nil
		}
		return lf - rf*math.Floor(lf/rf), nil
	case OpLT:
		return lf < rf, nil
	case OpLE:
		return lf <= rf, nil
	case OpGT:
		return lf > rf, nil
	case OpGE:
		return lf >= rf, nil
	default:
		return nil, fmt.Errorf("unsupported operator %s", op)
	}
}

// TryTypedDyadic attempts the shared typed binary kernel without falling
// back to scalar row evaluation. Callers with language-specific broadcasting or
// result-kind rules can probe this first and keep their existing fallback.
func TryTypedDyadic(op Op, left, right any) (any, bool, error) {
	return typedKernels.Dyadic(op, left, right)
}

// TryTypedIntegerDyadic attempts the integer-preserving typed binary kernel.
// It is intended for language surfaces, such as q.eval, whose arithmetic rules
// keep integer vector +,-,* results as integer vectors.
func TryTypedIntegerDyadic(op Op, left, right any) (any, bool, error) {
	return typedKernels.IntegerDyadic(op, left, right)
}

// TryTypedRotate returns a lazy typed view for supported array rotations.
func TryTypedRotate(array Array, n int) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		rotated, ok, err := TryTypedRotate(a.array, n)
		if err != nil || !ok {
			return rotated, ok, err
		}
		return a.withLazyRebuiltIndexes(rotated), true, nil
	case i64RangeArray:
		if a.len == 0 {
			return a, true, nil
		}
		shift := n % a.len
		if shift < 0 {
			shift += a.len
		}
		if shift == 0 {
			return a, true, nil
		}
		first := i64RangeArray{
			start: a.start + int64(shift)*a.step,
			step:  a.step,
			len:   a.len - shift,
		}
		second := i64RangeArray{
			start: a.start,
			step:  a.step,
			len:   shift,
		}
		return newI64SegmentArray(first, second), true, nil
	default:
		length := array.Len()
		if length == 0 {
			return array, true, nil
		}
		shift := n % length
		if shift < 0 {
			shift += length
		}
		if shift == 0 {
			return array, true, nil
		}
		rotated := tiledArray{source: array, start: shift, len: length}
		if metadata := ArrayMetadataOf(array); len(metadata.Attributes) > 0 {
			return attributedArray{array: rotated, metadata: metadata.cloneWithRebuiltIndexes(rotated)}, true, nil
		}
		return rotated, true, nil
	}
}

func newI64SegmentArray(segments ...i64RangeArray) Array {
	out := i64SegmentArray{segments: make([]i64RangeArray, 0, len(segments))}
	for _, segment := range segments {
		if segment.len <= 0 {
			continue
		}
		out.segments = append(out.segments, segment)
		out.len += segment.len
	}
	if len(out.segments) == 0 {
		return i64RangeArray{len: 0}
	}
	if len(out.segments) == 1 {
		return out.segments[0]
	}
	return out
}

func newI64PeriodicIndexArray(period int64, residues []int64, sourceLen int) Array {
	if period <= 0 || sourceLen <= 0 || len(residues) == 0 {
		return i64RangeArray{len: 0}
	}
	residues = append([]int64(nil), residues...)
	sort.Slice(residues, func(i, j int) bool { return residues[i] < residues[j] })
	unique := residues[:0]
	for _, residue := range residues {
		if residue < 0 || residue >= period {
			continue
		}
		if len(unique) == 0 || unique[len(unique)-1] != residue {
			unique = append(unique, residue)
		}
	}
	if len(unique) == 0 {
		return i64RangeArray{len: 0}
	}
	residues = append([]int64(nil), unique...)
	fullCycles := int64(sourceLen) / period
	remainder := int64(sourceLen) % period
	tail := make([]int64, 0, len(residues))
	for _, residue := range residues {
		if residue < remainder {
			tail = append(tail, residue)
		}
	}
	length64 := fullCycles*int64(len(residues)) + int64(len(tail))
	if length64 == 0 {
		return i64RangeArray{len: 0}
	}
	if length64 != int64(int(length64)) {
		values := make([]int64, 0)
		for cycle := int64(0); cycle < fullCycles; cycle++ {
			base := cycle * period
			for _, residue := range residues {
				values = append(values, base+residue)
			}
		}
		base := fullCycles * period
		for _, residue := range tail {
			values = append(values, base+residue)
		}
		return newI64Trusted(values)
	}
	return i64PeriodicIndexArray{
		period:       period,
		residues:     residues,
		tailResidues: tail,
		fullCycles:   fullCycles,
		len:          int(length64),
	}
}

func promotedNullForBinary(left, right any) any {
	if !hasTypedNullKind(left) && !hasTypedNullKind(right) {
		return NullValue
	}
	leftKind, leftOK := NullKind(left)
	rightKind, rightOK := NullKind(right)
	if !leftOK {
		leftKind = kindOfScalar(left)
	}
	if !rightOK {
		rightKind = kindOfScalar(right)
	}
	kind := mergeTypedNullArrayKind(leftKind, rightKind)
	return NullForKind(kind)
}

func hasTypedNullKind(v any) bool {
	kind, ok := NullKind(v)
	return ok && kind != KindNull
}

type Logical struct {
	Op          string
	Left, Right Expr
}

func (e Logical) EvalRow(frame Frame, row int) (any, error) {
	left, err := e.Left.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	lb, ok := left.(bool)
	if !ok {
		return nil, fmt.Errorf("logical %s expects bool operands", e.Op)
	}
	switch e.Op {
	case "and":
		if !lb {
			return false, nil
		}
	case "or":
		if lb {
			return true, nil
		}
	default:
		return nil, fmt.Errorf("unsupported logical operator %q", e.Op)
	}
	right, err := e.Right.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	rb, ok := right.(bool)
	if !ok {
		return nil, fmt.Errorf("logical %s expects bool operands", e.Op)
	}
	if e.Op == "and" {
		return lb && rb, nil
	}
	return lb || rb, nil
}

type Not struct {
	Expr Expr
}

func (e Not) EvalRow(frame Frame, row int) (any, error) {
	v, err := e.Expr.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	b, ok := v.(bool)
	if !ok {
		return nil, fmt.Errorf("not expects bool operand")
	}
	return !b, nil
}

type In struct {
	Expr   Expr
	Values []any
}

func (e In) EvalRow(frame Frame, row int) (any, error) {
	v, err := e.Expr.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	for _, candidate := range e.Values {
		if equalScalar(v, candidate) {
			return true, nil
		}
	}
	return false, nil
}

type Within struct {
	Expr       Expr
	Low, High  any
	HighClosed bool
}

func (e Within) EvalRow(frame Frame, row int) (any, error) {
	v, err := e.Expr.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	if compare(v, e.Low) < 0 {
		return false, nil
	}
	if e.HighClosed {
		return compare(v, e.High) <= 0, nil
	}
	return compare(v, e.High) < 0, nil
}

type BucketFloorExpr struct {
	Expr     Expr
	Interval any
}

func (e BucketFloorExpr) EvalRow(frame Frame, row int) (any, error) {
	if e.Expr == nil {
		return nil, fmt.Errorf("bucket floor expression is nil")
	}
	v, err := e.Expr.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	if IsNull(v) {
		return NullValue, nil
	}
	bucketed, err := BucketFloor(NewColumn("_", []any{v}).Data, e.Interval)
	if err != nil {
		return nil, err
	}
	out, ok := bucketed.At(0)
	if !ok {
		return nil, fmt.Errorf("bucket floor result row 0 out of range")
	}
	return out, nil
}

func (e BucketFloorExpr) EvalRows(frame Frame, indexes []int) (Array, error) {
	if e.Expr == nil {
		return nil, fmt.Errorf("bucket floor expression is nil")
	}
	switch expr := e.Expr.(type) {
	case ColumnRef:
		col, ok := frame.Column(expr.Name)
		if !ok {
			return nil, fmt.Errorf("unknown column %q", expr.Name)
		}
		if indexesCoverAllRows(indexes, col.Len()) {
			return BucketFloor(col, e.Interval)
		}
		return BucketFloor(col.Gather(indexes), e.Interval)
	case vectorProjector:
		array, err := expr.EvalRows(frame, indexes)
		if err != nil {
			return nil, err
		}
		return BucketFloor(array, e.Interval)
	}
	values := make([]any, len(indexes))
	for i, row := range indexes {
		v, err := e.Expr.EvalRow(frame, row)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return BucketFloor(NewColumn("_", values).Data, e.Interval)
}

type ListAggregateExpr struct {
	Func string
	Expr Expr
}

func (e ListAggregateExpr) EvalRow(frame Frame, row int) (any, error) {
	if e.Expr == nil {
		return nil, fmt.Errorf("list aggregate expression is nil")
	}
	v, err := e.Expr.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	return e.evalValue(v)
}

func (e ListAggregateExpr) EvalRows(frame Frame, indexes []int) (Array, error) {
	if e.Expr == nil {
		return nil, fmt.Errorf("list aggregate expression is nil")
	}
	array, err := evalProjectionExprRows(frame, indexes, e.Expr)
	if err != nil {
		return nil, err
	}
	if wl, ok := unwrapAttributedArray(array).(windowListArray); ok {
		if out, handled, err := windowListAggregateRows(e.Func, wl); handled || err != nil {
			return out, err
		}
	}
	values := make([]any, array.Len())
	for i := 0; i < array.Len(); i++ {
		v, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("list aggregate row %d out of range", i)
		}
		agg, err := e.evalValue(v)
		if err != nil {
			return nil, err
		}
		values[i] = agg
	}
	return InferArray(values), nil
}

// windowListAggregateRows aggregates every window of a lazy window-list
// column straight from the typed source column, mirroring evalValue's boxed
// semantics (per-window accumulation order, sum of an empty window is 0,
// avg of an empty window is null). handled=false defers to the boxed path.
func windowListAggregateRows(fn string, wl windowListArray) (Array, bool, error) {
	switch fn {
	case "sum", "avg", "count":
	default:
		return nil, false, nil
	}
	if fn == "count" {
		out := make([]int64, len(wl.windows))
		for i, rows := range wl.windows {
			out[i] = int64(len(rows))
		}
		return columnArray[int64]{kind: KindI64, data: out}, true, nil
	}
	n := wl.source.Len()
	values := make([]float64, n)
	if ok, err := TryExportF64Copy(wl.source, values); !ok || err != nil {
		ints := make([]int64, n)
		if ok, err := TryExportI64Copy(wl.source, ints); !ok || err != nil {
			return nil, false, nil
		}
		for i, v := range ints {
			values[i] = float64(v)
		}
	}
	sums := make([]float64, len(wl.windows))
	hasEmpty := false
	for i, rows := range wl.windows {
		if len(rows) == 0 {
			hasEmpty = true
			continue
		}
		var sum float64
		for _, row := range rows {
			if row < 0 || row >= n {
				return nil, true, fmt.Errorf("window list row %d out of range", row)
			}
			sum += values[row]
		}
		sums[i] = sum
	}
	if fn == "sum" {
		return columnArray[float64]{kind: KindF64, data: sums}, true, nil
	}
	// avg: empty windows are null, matching the boxed count==0 path.
	if !hasEmpty {
		for i, rows := range wl.windows {
			sums[i] /= float64(len(rows))
		}
		return columnArray[float64]{kind: KindF64, data: sums}, true, nil
	}
	out := make([]any, len(wl.windows))
	for i, rows := range wl.windows {
		if len(rows) == 0 {
			out[i] = NullValue
			continue
		}
		out[i] = sums[i] / float64(len(rows))
	}
	return InferArray(out), true, nil
}

func (e ListAggregateExpr) evalValue(v any) (any, error) {
	if arr, isArray := v.(Array); isArray {
		if out, handled, err := listAggregateTypedArrayValue(e.Func, arr); handled || err != nil {
			return out, err
		}
	}
	values, ok := listAggregateValues(v)
	if !ok {
		if IsNull(v) {
			values = nil
		} else {
			values = []any{v}
		}
	}
	switch e.Func {
	case "count":
		return int64(len(values)), nil
	case "sum", "avg", "var", "dev", "med":
		var sum float64
		var sumsq float64
		var nums []float64
		var count int64
		for _, item := range values {
			if IsNull(item) {
				continue
			}
			n, ok := numeric(item)
			if !ok {
				return nil, fmt.Errorf("%s expects numeric values, got %T (%v)", e.Func, item, item)
			}
			sum += n
			sumsq += n * n
			if e.Func == "med" {
				nums = append(nums, n)
			}
			count++
		}
		if e.Func == "sum" {
			return sum, nil
		}
		if count == 0 {
			return NullValue, nil
		}
		switch e.Func {
		case "avg":
			return sum / float64(count), nil
		case "var":
			mean := sum / float64(count)
			return sumsq/float64(count) - mean*mean, nil
		case "dev":
			mean := sum / float64(count)
			return math.Sqrt(sumsq/float64(count) - mean*mean), nil
		case "med":
			sort.Float64s(nums)
			mid := len(nums) / 2
			if len(nums)%2 == 1 {
				return nums[mid], nil
			}
			return (nums[mid-1] + nums[mid]) / 2, nil
		default:
			return NullValue, nil
		}
	case "min", "max":
		var best any
		hasBest := false
		for _, item := range values {
			if IsNull(item) {
				continue
			}
			if !hasBest || (e.Func == "min" && compare(item, best) < 0) || (e.Func == "max" && compare(item, best) > 0) {
				best = item
				hasBest = true
			}
		}
		if !hasBest {
			return NullValue, nil
		}
		return best, nil
	case "first":
		if len(values) == 0 {
			return NullValue, nil
		}
		return normalizeAggregateValue(values[0]), nil
	case "last":
		if len(values) == 0 {
			return NullValue, nil
		}
		return normalizeAggregateValue(values[len(values)-1]), nil
	default:
		return nil, fmt.Errorf("unsupported list aggregate %q", e.Func)
	}
}

func listAggregateValues(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case Array:
		values := make([]any, x.Len())
		for i := 0; i < x.Len(); i++ {
			item, ok := x.At(i)
			if !ok {
				return nil, false
			}
			values[i] = item
		}
		return values, true
	default:
		return nil, false
	}
}

type SelectItem struct {
	Name Symbol
	Expr Expr
}

type Aggregate struct {
	Name   Symbol
	Func   string
	Expr   Expr
	Weight Expr
}

type GroupedAssignment struct {
	Name   Symbol
	Func   string
	Expr   Expr
	Weight Expr
}

type OrderSpec struct {
	Column Symbol
	Desc   bool
}

type SortDirection int

const (
	Asc SortDirection = iota
	Desc
)

type QueryPlan struct {
	Source          Frame
	Where           Expr
	By              []Symbol
	ByExprs         []SelectItem
	Select          []SelectItem
	Aggregates      []Aggregate
	OrderBy         []OrderSpec
	PreProjectOrder bool
	LimitN          int
	Distinct        bool
}

func From(frame Frame) QueryPlan {
	return QueryPlan{Source: frame, LimitN: -1}
}

func (p QueryPlan) WhereExpr(expr Expr) QueryPlan {
	p.Where = expr
	return p
}

func (p QueryPlan) WhereEq(name Symbol, value any) QueryPlan {
	p.Where = Binary{Op: OpEQ, Left: ColumnRef{Name: name}, Right: Literal{Value: value}}
	return p
}

func (p QueryPlan) SelectColumns(names ...Symbol) QueryPlan {
	p.Select = p.Select[:0]
	for _, name := range names {
		p.Select = append(p.Select, SelectItem{Name: name, Expr: ColumnRef{Name: name}})
	}
	return p
}

func (p QueryPlan) GroupBy(names ...Symbol) QueryPlan {
	p.By = append([]Symbol(nil), names...)
	p.ByExprs = nil
	return p
}

func (p QueryPlan) GroupByExprs(items ...SelectItem) QueryPlan {
	p.By = nil
	p.ByExprs = append([]SelectItem(nil), items...)
	return p
}

func (p QueryPlan) Sum(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "sum", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) Min(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "min", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) Max(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "max", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) First(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "first", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) Last(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "last", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) Count(as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "count"})
	return p
}

func (p QueryPlan) Var(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "var", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) Dev(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "dev", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) Med(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "med", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) WAvg(weight, source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "wavg", Expr: ColumnRef{Name: source}, Weight: ColumnRef{Name: weight}})
	return p
}

func (p QueryPlan) OrderByColumn(name Symbol, dir SortDirection) QueryPlan {
	p.OrderBy = []OrderSpec{{Column: name, Desc: dir == Desc}}
	return p
}

func (p QueryPlan) OrderByColumns(specs ...OrderSpec) QueryPlan {
	p.OrderBy = append([]OrderSpec(nil), specs...)
	return p
}

func (p QueryPlan) Limit(n int) QueryPlan {
	p.LimitN = n
	return p
}

func (p QueryPlan) DistinctRows() QueryPlan {
	p.Distinct = true
	return p
}

func (p QueryPlan) Exec() (Frame, error) {
	return Exec(p.Source, p)
}

func Exec(frame Frame, plan QueryPlan) (Frame, error) {
	return execPlan(frame, plan, false)
}

// ExecConsume executes the plan for a caller that consumes the result frame
// exactly once (a value exporter) and then drops it: freshly materialized
// projection columns are marked ownership-transferable so the exporter can
// adopt their storage instead of re-copying. See transfer_owned.go.
func ExecConsume(frame Frame, plan QueryPlan) (Frame, error) {
	return execPlan(frame, plan, true)
}

func execPlan(frame Frame, plan QueryPlan, transferOwned bool) (Frame, error) {
	if frame.Len() < 0 {
		return Frame{}, fmt.Errorf("query frame is empty")
	}
	if out, ok, err := execUngroupedFilteredWhere(frame, plan); ok || err != nil {
		if err != nil {
			return Frame{}, err
		}
		return finishGroupedQueryResult(out, plan)
	}
	if out, ok, err := execGroupedFilteredWhere(frame, plan); ok || err != nil {
		if err != nil {
			return Frame{}, err
		}
		return finishGroupedQueryResult(out, plan)
	}
	if out, ok, err := execTypedFilterProject(frame, plan); ok || err != nil {
		return out, err
	}
	if out, ok, err := execTypedGroupedProjectionProject(frame, plan); ok || err != nil {
		return out, err
	}
	indexes, err := filterIndexes(frame, plan.Where)
	if err != nil {
		return Frame{}, err
	}
	// The where-index vector is consumed entirely inside this call (see the
	// matching release in QueryKernel.Exec).
	defer bulkIntRelease(indexes)
	projectedOrderBy, projectOrderBeforeProjection := projectedSourceOrderSpecs(plan)
	if len(plan.OrderBy) > 0 && plan.PreProjectOrder {
		if canLimitBeforeProjection(plan) {
			indexes, err = orderIndexesLimit(frame, indexes, plan.OrderBy, plan.LimitN)
		} else {
			indexes, err = orderIndexes(frame, indexes, plan.OrderBy)
		}
		if err != nil {
			return Frame{}, err
		}
	}
	if len(projectedOrderBy) > 0 && projectOrderBeforeProjection {
		indexes, err = orderIndexesLimit(frame, indexes, projectedOrderBy, plan.LimitN)
		if err != nil {
			return Frame{}, err
		}
	}
	if canLimitBeforeProjection(plan) && plan.LimitN < len(indexes) {
		indexes = indexes[:plan.LimitN]
	}
	var out Frame
	if len(plan.By) > 0 || len(plan.ByExprs) > 0 || len(plan.Aggregates) > 0 {
		out, err = execGrouped(frame, indexes, plan)
	} else {
		out, err = execProjectTransfer(frame, indexes, plan.Select, transferOwned && !plan.Distinct && len(plan.OrderBy) == 0)
	}
	if err != nil {
		return Frame{}, err
	}
	if plan.Distinct {
		out, err = Distinct(out)
		if err != nil {
			return Frame{}, err
		}
	}
	if len(plan.OrderBy) > 0 && !plan.PreProjectOrder && !projectOrderBeforeProjection {
		out, err = orderFrameLimit(out, plan.OrderBy, plan.LimitN)
		if err != nil {
			return Frame{}, err
		}
	}
	if plan.LimitN >= 0 && plan.LimitN < out.Len() {
		return out.Gather(allIndexes(plan.LimitN))
	}
	return out, nil
}

func finishGroupedQueryResult(out Frame, plan QueryPlan) (Frame, error) {
	var err error
	if plan.Distinct {
		out, err = Distinct(out)
		if err != nil {
			return Frame{}, err
		}
	}
	if len(plan.OrderBy) > 0 && !plan.PreProjectOrder {
		out, err = orderFrameLimit(out, plan.OrderBy, plan.LimitN)
		if err != nil {
			return Frame{}, err
		}
	}
	if plan.LimitN >= 0 && plan.LimitN < out.Len() {
		return out.Gather(allIndexes(plan.LimitN))
	}
	return out, nil
}

func execTypedFilterProject(frame Frame, plan QueryPlan) (Frame, bool, error) {
	// Mirror TryExecuteQueryKernelTypedCarrier's plan-shape preconditions.
	if plan.Distinct || plan.PreProjectOrder || plan.LimitN >= 0 || len(plan.OrderBy) > 0 ||
		len(plan.By) > 0 || len(plan.ByExprs) > 0 || len(plan.Aggregates) > 0 {
		return Frame{}, false, nil
	}
	// Apply the same gates DescribeQueryKernelPlan applies, but per execution
	// instead of per describe: skip the deep plan clone, the fingerprint /
	// cache-key serialization, and — critically — the describe-time filter
	// probe, which ran the full typed compare only to throw the index vector
	// away before the carrier exec ran it a second time.
	if !typedFilterProjectPlanSupported(plan) {
		return Frame{}, false, nil
	}
	if err := validateQueryKernelFrame(frame, plan); err != nil {
		return Frame{}, true, err
	}
	if plan.Where != nil && queryKernelWherePipelineShape(plan.Where) == "" {
		return Frame{}, false, nil
	}
	if !queryKernelProjectionUsesTypedCarrier(plan) {
		return Frame{}, false, nil
	}
	indexes, handled, err := typedFilterIndexArray(frame, plan.Where)
	if err != nil {
		return Frame{}, true, err
	}
	if !handled {
		return Frame{}, false, nil
	}
	out, err := execProjectByI64IndexArray(frame, indexes, plan.Select)
	return out, true, err
}

// typedFilterProjectPlanSupported is the projection/where subset of
// QueryKernelSupportReason for plans already known to carry no by/aggregate/
// order/distinct/limit clauses, without building the success reason string.
func typedFilterProjectPlanSupported(plan QueryPlan) bool {
	for _, item := range plan.Select {
		if queryKernelExprUnsupportedReason(item.Expr) != "" {
			return false
		}
	}
	return queryKernelWhereUnsupportedReason(plan.Where) == ""
}

func execTypedGroupedProjectionProject(frame Frame, plan QueryPlan) (Frame, bool, error) {
	// Mirror TryExecuteQueryKernelGroupedProjectionCarrier's plan-shape
	// preconditions.
	if plan.Distinct || plan.PreProjectOrder || plan.LimitN >= 0 || len(plan.OrderBy) > 0 ||
		len(plan.Aggregates) > 0 || len(plan.Select) == 0 {
		return Frame{}, false, nil
	}
	if len(plan.By) == 0 && len(plan.ByExprs) == 0 {
		return Frame{}, false, nil
	}
	// Same per-execution gating as execTypedFilterProject (see above), plus
	// the by-expression support check DescribeQueryKernelPlan would apply.
	for _, item := range plan.ByExprs {
		if queryKernelExprUnsupportedReason(item.Expr) != "" {
			return Frame{}, false, nil
		}
	}
	if !typedFilterProjectPlanSupported(plan) {
		return Frame{}, false, nil
	}
	if err := validateQueryKernelFrame(frame, plan); err != nil {
		return Frame{}, true, err
	}
	if plan.Where != nil && queryKernelWherePipelineShape(plan.Where) == "" {
		return Frame{}, false, nil
	}
	if !queryKernelProjectionUsesTypedCarrier(plan) {
		return Frame{}, false, nil
	}
	indexes, handled, err := typedFilterIndexArray(frame, plan.Where)
	if err != nil {
		return Frame{}, true, err
	}
	if !handled {
		return Frame{}, false, nil
	}
	out, err := execProjectByI64IndexArray(frame, indexes, plan.Select)
	return out, true, err
}

func canLimitBeforeProjection(plan QueryPlan) bool {
	if !canLimitRowsBeforeProjection(plan) {
		return false
	}
	return len(plan.OrderBy) == 0 || plan.PreProjectOrder
}

func canLimitRowsBeforeProjection(plan QueryPlan) bool {
	if plan.LimitN < 0 || plan.Distinct || len(plan.By) > 0 || len(plan.ByExprs) > 0 || len(plan.Aggregates) > 0 {
		return false
	}
	for _, item := range plan.Select {
		if exprNeedsFullProjectionRows(item.Expr) {
			return false
		}
	}
	return true
}

func exprNeedsFullProjectionRows(expr Expr) bool {
	switch e := expr.(type) {
	case nil, ColumnRef, Literal:
		return false
	case vectorProjector:
		return true
	case Binary:
		return exprNeedsFullProjectionRows(e.Left) || exprNeedsFullProjectionRows(e.Right)
	case Conditional:
		return exprNeedsFullProjectionRows(e.Cond) || exprNeedsFullProjectionRows(e.Then) || exprNeedsFullProjectionRows(e.Else)
	case Logical:
		return exprNeedsFullProjectionRows(e.Left) || exprNeedsFullProjectionRows(e.Right)
	case Not:
		return exprNeedsFullProjectionRows(e.Expr)
	case In:
		return exprNeedsFullProjectionRows(e.Expr)
	case Within:
		return exprNeedsFullProjectionRows(e.Expr)
	case BucketFloorExpr:
		return exprNeedsFullProjectionRows(e.Expr)
	case ListAggregateExpr:
		return exprNeedsFullProjectionRows(e.Expr)
	default:
		return false
	}
}

func projectedSourceOrderSpecs(plan QueryPlan) ([]OrderSpec, bool) {
	if plan.PreProjectOrder || len(plan.OrderBy) == 0 || !canLimitRowsBeforeProjection(plan) {
		return nil, false
	}
	if len(plan.Select) == 0 {
		return append([]OrderSpec(nil), plan.OrderBy...), true
	}
	byOutput := make(map[Symbol]ColumnRef, len(plan.Select))
	for _, item := range plan.Select {
		if ref, ok := item.Expr.(ColumnRef); ok {
			byOutput[item.Name] = ref
		}
	}
	out := make([]OrderSpec, 0, len(plan.OrderBy))
	for _, spec := range plan.OrderBy {
		ref, ok := byOutput[spec.Column]
		if !ok {
			return nil, false
		}
		out = append(out, OrderSpec{Column: ref.Name, Desc: spec.Desc})
	}
	return out, true
}

func filterIndexes(frame Frame, where Expr) ([]int, error) {
	if where == nil {
		return allIndexes(frame.Len()), nil
	}
	if hasScalarAggregateExpr(where) {
		resolved, err := resolveScalarAggregateExprs(frame, where)
		if err != nil {
			return nil, err
		}
		where = resolved
	}
	if indexes, ok, err := fastFilterIndexes(frame, where); ok || err != nil {
		return indexes, err
	}
	indexes := make([]int, 0, frame.Len())
	for i := 0; i < frame.Len(); i++ {
		if where == nil {
			indexes = append(indexes, i)
			continue
		}
		v, err := where.EvalRow(frame, i)
		if err != nil {
			return nil, err
		}
		keep, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("where expression must evaluate to bool")
		}
		if keep {
			indexes = append(indexes, i)
		}
	}
	return indexes, nil
}

func typedFilterIndexArray(frame Frame, where Expr) (Array, bool, error) {
	switch expr := where.(type) {
	case nil:
		return i64RangeArray{start: 0, step: 1, len: frame.Len()}, true, nil
	case Binary:
		if !isComparisonOp(expr.Op) {
			return nil, false, nil
		}
		ref, op, literal, ok := binaryColumnLiteral(expr)
		if !ok {
			return nil, false, nil
		}
		col, ok := frame.Column(ref.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", ref.Name)
		}
		normalized := normalizeScalar(col.Kind(), literal.Value)
		indexes, handled, err := TryTypedCompareIndexesI64(col, op, normalized)
		if err != nil || handled {
			return indexes, handled, err
		}
		return nil, false, nil
	case Logical:
		if expr.Op != "and" {
			return nil, false, nil
		}
		rows, handled, err := fastLogicalAndFilterIndexes(frame, expr)
		if err != nil {
			return nil, true, err
		}
		if handled {
			return intIndexArray{rows: rows}, true, nil
		}
		return nil, false, nil
	case Within:
		ref, ok := expr.Expr.(ColumnRef)
		if !ok {
			return nil, false, nil
		}
		col, ok := frame.Column(ref.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", ref.Name)
		}
		indexes, handled, err := TryTypedWithinIndexesI64(col, expr.Low, expr.High, expr.HighClosed)
		if err != nil || handled {
			return indexes, handled, err
		}
		return nil, false, nil
	case In:
		ref, ok := expr.Expr.(ColumnRef)
		if !ok {
			return nil, false, nil
		}
		col, ok := frame.Column(ref.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", ref.Name)
		}
		indexes, handled, err := TryTypedInIndexesI64(col, expr.Values)
		if err != nil || handled {
			return indexes, handled, err
		}
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

func fastFilterIndexes(frame Frame, where Expr) ([]int, bool, error) {
	switch expr := where.(type) {
	case nil:
		return nil, false, nil
	case Binary:
		if !isComparisonOp(expr.Op) {
			return nil, false, nil
		}
		ref, op, literal, ok := binaryColumnLiteral(expr)
		if ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return nil, true, fmt.Errorf("unknown column %q", ref.Name)
			}
			if op == OpEQ {
				if rows, ok := indexedEqualRows(col, literal.Value); ok {
					return rows, true, nil
				}
			}
			normalized := normalizeScalar(col.Kind(), literal.Value)
			if rows, ok := typedKernels.CompareIndexes(col, op, normalized, nil); ok {
				return rows, true, nil
			}
			mask, err := CompareMask(col, op, literal.Value)
			if err != nil {
				return nil, true, err
			}
			indexes, err := WhereMask(mask)
			return indexes, true, err
		}
		if mask, ok, err := evalBinaryArray(frame, expr); ok || err != nil {
			if err != nil {
				return nil, true, err
			}
			indexes, err := WhereMask(mask)
			return indexes, true, err
		}
		return nil, false, nil
	case Logical:
		if expr.Op == "and" {
			if indexes, ok, err := fastLogicalAndFilterIndexes(frame, expr); ok || err != nil {
				return indexes, ok, err
			}
		}
		left, lok, err := fastFilterIndexes(frame, expr.Left)
		if err != nil || !lok {
			return nil, lok, err
		}
		right, rok, err := fastFilterIndexes(frame, expr.Right)
		if err != nil || !rok {
			return nil, rok, err
		}
		switch expr.Op {
		case "and":
			return intersectSortedIndexes(left, right), true, nil
		case "or":
			return unionSortedIndexes(left, right), true, nil
		default:
			return nil, false, nil
		}
	case Within:
		ref, ok := expr.Expr.(ColumnRef)
		if !ok {
			return nil, false, nil
		}
		col, ok := frame.Column(ref.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", ref.Name)
		}
		low := normalizeScalar(col.Kind(), expr.Low)
		high := normalizeScalar(col.Kind(), expr.High)
		if indexes, ok := typedKernels.WithinIndexes(col, low, high, expr.HighClosed, filterIndexScratch(col.Len())); ok {
			return indexes, true, nil
		}
		mask, err := WithinMask(col, expr.Low, expr.High, expr.HighClosed)
		if err != nil {
			return nil, true, err
		}
		indexes, err := WhereMask(mask)
		return indexes, true, err
	case In:
		ref, ok := expr.Expr.(ColumnRef)
		if !ok {
			return nil, false, nil
		}
		col, ok := frame.Column(ref.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", ref.Name)
		}
		if indexes, ok := typedKernels.IndexedInRows(col, expr.Values); ok {
			return indexes, true, nil
		}
		if indexes, ok := typedKernels.InIndexes(col, expr.Values, filterIndexScratch(col.Len())); ok {
			return indexes, true, nil
		}
		combined := make([]bool, col.Len())
		for _, value := range expr.Values {
			mask, err := CompareMask(col, OpEQ, value)
			if err != nil {
				return nil, true, err
			}
			for row := 0; row < mask.Len(); row++ {
				v, ok := mask.At(row)
				if !ok {
					return nil, true, fmt.Errorf("in mask row %d out of range", row)
				}
				if keep, ok := v.(bool); ok && keep {
					combined[row] = true
				}
			}
		}
		indexes, err := WhereMask(NewBool(combined))
		return indexes, true, err
	default:
		return nil, false, nil
	}
}

type rowPredicate func(row int) bool

func fastLogicalAndFilterIndexes(frame Frame, expr Logical) ([]int, bool, error) {
	if out, ok := typedAndCompareFilterIndexes(frame, expr); ok {
		return out, true, nil
	}
	predicate, ok, err := filterRowPredicate(frame, expr)
	if err != nil || !ok {
		return nil, ok, err
	}
	out := filterIndexScratch(frame.Len())
	for row := 0; row < frame.Len(); row++ {
		if predicate(row) {
			out = append(out, row)
		}
	}
	return out, true, nil
}

func filterRowPredicate(frame Frame, expr Expr) (rowPredicate, bool, error) {
	switch e := expr.(type) {
	case Binary:
		return comparisonRowPredicate(frame, e)
	case Logical:
		left, lok, err := filterRowPredicate(frame, e.Left)
		if err != nil || !lok {
			return nil, lok, err
		}
		right, rok, err := filterRowPredicate(frame, e.Right)
		if err != nil || !rok {
			return nil, rok, err
		}
		switch e.Op {
		case "and":
			return func(row int) bool {
				return left(row) && right(row)
			}, true, nil
		case "or":
			return func(row int) bool {
				return left(row) || right(row)
			}, true, nil
		default:
			return nil, false, nil
		}
	default:
		return nil, false, nil
	}
}

func comparisonRowPredicate(frame Frame, expr Expr) (rowPredicate, bool, error) {
	binary, ok := expr.(Binary)
	if !ok || !isComparisonOp(binary.Op) {
		return nil, false, nil
	}
	ref, op, literal, ok := binaryColumnLiteral(binary)
	if !ok {
		return nil, false, nil
	}
	col, ok := frame.Column(ref.Name)
	if !ok {
		return nil, true, fmt.Errorf("unknown column %q", ref.Name)
	}
	return typedCompareRowPredicate(col, op, normalizeScalar(col.Kind(), literal.Value))
}

func typedCompareRowPredicate(array Array, op Op, value any) (rowPredicate, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedCompareRowPredicate(a.array, op, value)
	case columnArray[bool]:
		target, ok := value.(bool)
		return compareBoolRowPredicate(a.data, target, ok, op)
	case columnArray[int8]:
		target, ok := value.(int8)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[int16]:
		target, ok := value.(int16)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[int32]:
		target, ok := value.(int32)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[int64]:
		target, ok := coerceInt64Exact(value)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[uint8]:
		target, ok := value.(uint8)
		return compareUnsignedRowPredicate(a.data, target, ok, op)
	case columnArray[uint16]:
		target, ok := value.(uint16)
		return compareUnsignedRowPredicate(a.data, target, ok, op)
	case columnArray[uint32]:
		target, ok := value.(uint32)
		return compareUnsignedRowPredicate(a.data, target, ok, op)
	case columnArray[uint64]:
		target, ok := value.(uint64)
		return compareUnsignedRowPredicate(a.data, target, ok, op)
	case columnArray[float32]:
		target, ok := value.(float32)
		return compareFloatRowPredicate(a.data, target, ok, op)
	case columnArray[float64]:
		target, ok := numeric(value)
		return compareFloatRowPredicate(a.data, target, ok, op)
	case columnArray[string]:
		target, ok := coerceComparableString(value)
		return compareStringRowPredicate(a.data, target, ok, op)
	case columnArray[Symbol]:
		target, ok := coerceComparableSymbol(value)
		return compareSymbolRowPredicate(a.data, target, ok, op)
	case columnArray[Month]:
		target, ok := value.(Month)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[Date]:
		target, ok := value.(Date)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[DateTime]:
		target, ok := value.(DateTime)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[Timespan]:
		target, ok := value.(Timespan)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[Minute]:
		target, ok := value.(Minute)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[Second]:
		target, ok := value.(Second)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[Time]:
		target, ok := value.(Time)
		return compareSignedRowPredicate(a.data, target, ok, op)
	case columnArray[Timestamp]:
		target, ok := value.(Timestamp)
		return compareSignedRowPredicate(a.data, target, ok, op)
	default:
		return compareArrayCarrierRowPredicate(array, value, op)
	}
}

func compareArrayCarrierRowPredicate(array Array, value any, op Op) (rowPredicate, bool, error) {
	if array == nil || !typedCompareKindCanHandle(array.Kind()) {
		return nil, false, nil
	}
	target := normalizeScalar(array.Kind(), value)
	if !typedCompareCarrierTargetCompatible(array.Kind(), target) {
		return nil, false, nil
	}
	return func(row int) bool {
		left, ok := array.At(row)
		if !ok {
			return false
		}
		if IsNull(left) || IsNull(target) {
			equal := IsNull(left) && IsNull(target)
			switch op {
			case OpEQ, OpNE:
				return boolCompare(op, equal, 0)
			default:
				return false
			}
		}
		cmp, ok := compareSameKind(left, target)
		if !ok {
			return false
		}
		return boolCompare(op, cmp == 0, cmp)
	}, true, nil
}

func compareBoolRowPredicate(values []bool, target bool, ok bool, op Op) (rowPredicate, bool, error) {
	if !ok {
		return nil, false, nil
	}
	return func(row int) bool {
		v := values[row]
		return boolCompare(op, v == target, compareBool(v, target))
	}, true, nil
}

func compareSignedRowPredicate[T signedScalar](values []T, target T, ok bool, op Op) (rowPredicate, bool, error) {
	if !ok {
		return nil, false, nil
	}
	return func(row int) bool {
		v := values[row]
		return boolCompare(op, int64(v) == int64(target), compareInt64(int64(v), int64(target)))
	}, true, nil
}

func compareUnsignedRowPredicate[T unsignedScalar](values []T, target T, ok bool, op Op) (rowPredicate, bool, error) {
	if !ok {
		return nil, false, nil
	}
	return func(row int) bool {
		v := values[row]
		return boolCompare(op, uint64(v) == uint64(target), compareUint64(uint64(v), uint64(target)))
	}, true, nil
}

func compareFloatRowPredicate[T floatScalar](values []T, target T, ok bool, op Op) (rowPredicate, bool, error) {
	if !ok {
		return nil, false, nil
	}
	return func(row int) bool {
		v := values[row]
		return boolCompare(op, float64(v) == float64(target), compareFloat64(float64(v), float64(target)))
	}, true, nil
}

func compareStringRowPredicate(values []string, target string, ok bool, op Op) (rowPredicate, bool, error) {
	if !ok {
		return nil, false, nil
	}
	return func(row int) bool {
		v := values[row]
		return boolCompare(op, v == target, compareString(v, target))
	}, true, nil
}

func compareSymbolRowPredicate(values []Symbol, target Symbol, ok bool, op Op) (rowPredicate, bool, error) {
	if !ok {
		return nil, false, nil
	}
	return func(row int) bool {
		v := values[row]
		return boolCompare(op, v == target, compareString(string(v), string(target)))
	}, true, nil
}

func intersectSortedIndexes(left, right []int) []int {
	out := make([]int, 0, min(len(left), len(right)))
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		switch {
		case left[i] == right[j]:
			out = append(out, left[i])
			i++
			j++
		case left[i] < right[j]:
			i++
		default:
			j++
		}
	}
	return out
}

func unionSortedIndexes(left, right []int) []int {
	out := make([]int, 0, len(left)+len(right))
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		switch {
		case left[i] == right[j]:
			out = append(out, left[i])
			i++
			j++
		case left[i] < right[j]:
			out = append(out, left[i])
			i++
		default:
			out = append(out, right[j])
			j++
		}
	}
	out = append(out, left[i:]...)
	out = append(out, right[j:]...)
	return out
}

func filterIndexScratch(length int) []int {
	if length <= 0 {
		return nil
	}
	capacity := length / 2
	if capacity < 16 {
		capacity = 16
	}
	if capacity > length {
		capacity = length
	}
	// Pool-backed: index vectors are 64KB-class temporaries whose
	// allocate/zero/collect churn dominates warm filter paths. Callers that
	// can prove the vector dies (QueryKernel.Exec) release it back.
	return bulkIntGet(capacity)
}

func indexedEqualRows(array Array, value any) ([]int, bool) {
	uniqueIndex, uniqueOK := arrayIndexForBorrowed(array, ArrayAttributeUnique)
	groupedIndex, groupedOK := arrayIndexForBorrowed(array, ArrayAttributeGrouped)
	if !uniqueOK && !groupedOK {
		return nil, false
	}
	normalized, err := normalizeKeyValue(array.Kind(), value)
	if err != nil {
		return nil, false
	}
	key := arrayValueKey(array.Kind(), normalized)
	if uniqueOK {
		return append([]int(nil), uniqueIndex.RowsByKey[key]...), true
	}
	if groupedOK {
		return append([]int(nil), groupedIndex.RowsByKey[key]...), true
	}
	return nil, false
}

func isComparisonOp(op Op) bool {
	switch op {
	case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
		return true
	default:
		return false
	}
}

func binaryColumnLiteral(expr Binary) (ColumnRef, Op, Literal, bool) {
	leftRef, leftRefOK := expr.Left.(ColumnRef)
	rightRef, rightRefOK := expr.Right.(ColumnRef)
	leftLit, leftLitOK := expr.Left.(Literal)
	rightLit, rightLitOK := expr.Right.(Literal)
	switch {
	case leftRefOK && rightLitOK:
		return leftRef, expr.Op, rightLit, true
	case leftLitOK && rightRefOK:
		return rightRef, reverseComparisonOp(expr.Op), leftLit, true
	default:
		return ColumnRef{}, "", Literal{}, false
	}
}

func reverseComparisonOp(op Op) Op {
	switch op {
	case OpLT:
		return OpGT
	case OpLE:
		return OpGE
	case OpGT:
		return OpLT
	case OpGE:
		return OpLE
	default:
		return op
	}
}

func evalExprArray(frame Frame, expr Expr) (Array, bool, error) {
	switch e := expr.(type) {
	case vectorProjector:
		indexes := allIndexes(frame.Len())
		array, err := e.EvalRows(frame, indexes)
		return array, true, err
	case ColumnRef:
		col, ok := frame.Column(e.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", e.Name)
		}
		return col, true, nil
	case Binary:
		return evalBinaryArray(frame, e)
	default:
		return nil, false, nil
	}
}

func evalBinaryArray(frame Frame, expr Binary) (Array, bool, error) {
	left, lok, err := evalDyadicOperand(frame, expr.Left)
	if err != nil || !lok {
		return nil, lok, err
	}
	right, rok, err := evalDyadicOperand(frame, expr.Right)
	if err != nil || !rok {
		return nil, rok, err
	}
	return applyProjectionDyadic(expr.Op, left, right)
}

// applyProjectionDyadic lowers a columnar binary projection, preserving
// integer kinds for integer arithmetic operands (canonical q: long+long stays
// long; % alone widens to float) before the f64-widening kernel fallback.
func applyProjectionDyadic(op Op, left, right any) (Array, bool, error) {
	if out, ok, err := tryProjectionIntegerDyadic(op, left, right); ok || err != nil {
		if err != nil {
			return nil, true, err
		}
		if array, isArray := out.(Array); isArray {
			return array, true, nil
		}
	}
	out, ok, err := typedKernels.Dyadic(op, left, right)
	if err != nil || !ok {
		return nil, ok, err
	}
	array, ok := out.(Array)
	if !ok {
		return nil, false, nil
	}
	return array, true, nil
}

func tryProjectionIntegerDyadic(op Op, left, right any) (any, bool, error) {
	switch op {
	case OpAdd, OpSub, OpMul:
		return typedKernels.IntegerDyadic(op, left, right)
	default:
		return nil, false, nil
	}
}

func evalDyadicOperand(frame Frame, expr Expr) (any, bool, error) {
	switch e := expr.(type) {
	case ColumnRef:
		col, ok := frame.Column(e.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", e.Name)
		}
		return col, true, nil
	case Literal:
		return e.Value, true, nil
	default:
		return nil, false, nil
	}
}

func evalProjectionExprRows(frame Frame, indexes []int, expr Expr) (Array, error) {
	if projector, ok := expr.(vectorProjector); ok {
		array, err := projector.EvalRows(frame, indexes)
		if err != nil {
			return nil, err
		}
		if array.Len() != len(indexes) {
			return nil, fmt.Errorf("vector projection returned %d rows, want %d", array.Len(), len(indexes))
		}
		return array, nil
	}
	if array, ok, err := evalExprArray(frame, expr); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return array.Gather(indexes), nil
	}
	values := make([]any, len(indexes))
	for i, row := range indexes {
		v, err := expr.EvalRow(frame, row)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return InferArray(values), nil
}

func execProjectByI64IndexArray(frame Frame, indexes Array, items []SelectItem) (Frame, error) {
	if indexes == nil {
		return Frame{}, fmt.Errorf("project index vector is nil")
	}
	if indexes.Kind() != KindI64 {
		return Frame{}, fmt.Errorf("project index vector kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if len(items) == 0 {
		items = make([]SelectItem, 0, len(frame.schema.names))
		for _, name := range frame.schema.names {
			items = append(items, SelectItem{Name: name, Expr: ColumnRef{Name: name}})
		}
	}
	allRows := i64IndexArrayCoversAllRows(indexes, frame.Len())
	var rows []int
	rowsReady := false
	indexRows := func() ([]int, error) {
		if rowsReady {
			return rows, nil
		}
		out, handled, err := TryTypedI64Indexes(indexes)
		if err != nil {
			return nil, err
		}
		if !handled {
			return nil, fmt.Errorf("project index vector is not a supported typed i64 index")
		}
		rows, rowsReady = out, true
		return rows, nil
	}
	// The index vector is shared by every projected column, so its bounds
	// check against the frame length is memoized instead of re-scanning the
	// whole vector per column inside TryGatherByI64IndexArray.
	frameLen := frame.Len()
	frameLenValid := false
	frameLenChecked := false
	gatherFrameColumn := func(col Array) (Array, bool, error) {
		if col.Len() != frameLen {
			return TryGatherByI64IndexArray(col, indexes)
		}
		if out, ok, err := tryGatherRangeByI64IndexArray(col, indexes); ok || err != nil {
			return out, ok, err
		}
		if !frameLenChecked {
			ok, err := validateI64IndexArray(indexes, frameLen)
			if err != nil {
				return nil, true, err
			}
			frameLenValid, frameLenChecked = ok, true
		}
		if frameLenValid {
			return indexedArray{source: col, indexes: indexes, len: indexes.Len()}, true, nil
		}
		return TryGatherByI64IndexArray(col, indexes)
	}
	cols := make([]Column, 0, len(items))
	for _, item := range items {
		if item.Name == "" {
			return Frame{}, fmt.Errorf("select item name must not be empty")
		}
		if ref, ok := item.Expr.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return Frame{}, fmt.Errorf("unknown column %q", ref.Name)
			}
			if allRows {
				cols = append(cols, Column{Name: item.Name, Data: col})
				continue
			}
			out, handled, err := gatherFrameColumn(col)
			if err != nil {
				return Frame{}, err
			}
			if handled {
				cols = append(cols, Column{Name: item.Name, Data: out})
				continue
			}
			rs, err := indexRows()
			if err != nil {
				return Frame{}, err
			}
			cols = append(cols, Column{Name: item.Name, Data: col.Gather(rs)})
			continue
		}
		if projector, ok := item.Expr.(vectorProjector); ok {
			rs, err := indexRows()
			if err != nil {
				return Frame{}, err
			}
			array, err := projector.EvalRows(frame, rs)
			if err != nil {
				return Frame{}, err
			}
			if array.Len() != len(rs) {
				return Frame{}, fmt.Errorf("select item %q returned %d rows, want %d", item.Name, array.Len(), len(rs))
			}
			cols = append(cols, Column{Name: item.Name, Data: array})
			continue
		}
		if !allRows {
			if array, ok, err := evalExprByI64IndexArray(frame, indexes, item.Expr); ok || err != nil {
				if err != nil {
					return Frame{}, err
				}
				cols = append(cols, Column{Name: item.Name, Data: array})
				continue
			}
		}
		if array, ok, err := evalExprArray(frame, item.Expr); ok || err != nil {
			if err != nil {
				return Frame{}, err
			}
			if allRows {
				cols = append(cols, Column{Name: item.Name, Data: array})
				continue
			}
			out, handled, err := gatherFrameColumn(array)
			if err != nil {
				return Frame{}, err
			}
			if handled {
				cols = append(cols, Column{Name: item.Name, Data: out})
				continue
			}
			rs, err := indexRows()
			if err != nil {
				return Frame{}, err
			}
			cols = append(cols, Column{Name: item.Name, Data: array.Gather(rs)})
			continue
		}
		rs, err := indexRows()
		if err != nil {
			return Frame{}, err
		}
		values := make([]any, len(rs))
		for i, row := range rs {
			v, err := item.Expr.EvalRow(frame, row)
			if err != nil {
				return Frame{}, err
			}
			values[i] = v
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	return newFrameTrusted(cols...)
}

func evalExprByI64IndexArray(frame Frame, indexes Array, expr Expr) (Array, bool, error) {
	switch e := expr.(type) {
	case ColumnRef:
		col, ok := frame.Column(e.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", e.Name)
		}
		return TryGatherByI64IndexArray(col, indexes)
	case Binary:
		return evalBinaryByI64IndexArray(frame, indexes, e)
	default:
		return nil, false, nil
	}
}

func evalBinaryByI64IndexArray(frame Frame, indexes Array, expr Binary) (Array, bool, error) {
	left, lok, err := evalDyadicOperandByI64IndexArray(frame, indexes, expr.Left)
	if err != nil || !lok {
		return nil, lok, err
	}
	right, rok, err := evalDyadicOperandByI64IndexArray(frame, indexes, expr.Right)
	if err != nil || !rok {
		return nil, rok, err
	}
	return applyProjectionDyadic(expr.Op, left, right)
}

func evalDyadicOperandByI64IndexArray(frame Frame, indexes Array, expr Expr) (any, bool, error) {
	switch e := expr.(type) {
	case ColumnRef:
		col, ok := frame.Column(e.Name)
		if !ok {
			return nil, true, fmt.Errorf("unknown column %q", e.Name)
		}
		return TryGatherByI64IndexArray(col, indexes)
	case Literal:
		return e.Value, true, nil
	default:
		return nil, false, nil
	}
}

// TryGatherFrameByI64IndexArray gathers frame rows with a typed i64 index
// vector, preserving lazy column/index views when the column kernels support it.
func TryGatherFrameByI64IndexArray(frame Frame, indexes Array) (Frame, bool, error) {
	if indexes == nil {
		return Frame{}, true, fmt.Errorf("frame gather index vector is nil")
	}
	if indexes.Kind() != KindI64 {
		return Frame{}, false, nil
	}
	out, err := execProjectByI64IndexArray(frame, indexes, nil)
	return out, true, err
}

// TrySortFrameByColumns sorts frames through typed sort-index and gather
// kernels for common single-column qSQL/q table order paths.
func TrySortFrameByColumns(frame Frame, names []Symbol, descending bool) (Frame, bool, error) {
	if len(names) != 1 {
		return Frame{}, false, nil
	}
	col, ok := frame.Column(names[0])
	if !ok {
		return Frame{}, true, fmt.Errorf("sort column %q does not exist", names[0])
	}
	indexes, handled, err := TryTypedSortIndexesI64(col, descending)
	if err != nil || !handled {
		return Frame{}, handled, err
	}
	out, gathered, err := TryGatherFrameByI64IndexArray(frame, indexes)
	if err != nil || !gathered {
		return Frame{}, gathered, err
	}
	return out, true, nil
}

func i64IndexArrayCoversAllRows(indexes Array, rows int) bool {
	switch idx := indexes.(type) {
	case attributedArray:
		return i64IndexArrayCoversAllRows(idx.array, rows)
	case i64RangeArray:
		return idx.start == 0 && idx.step == 1 && idx.len == rows
	default:
		return false
	}
}

func execProject(frame Frame, indexes []int, items []SelectItem) (Frame, error) {
	return execProjectTransfer(frame, indexes, items, false)
}

// execProjectTransfer projects the selected items; transferOwned marks the
// freshly gathered ColumnRef outputs as ownership-transferable for a caller
// that consumes the result frame exactly once (see transfer_owned.go).
func execProjectTransfer(frame Frame, indexes []int, items []SelectItem, transferOwned bool) (Frame, error) {
	if len(items) == 0 {
		items = make([]SelectItem, 0, len(frame.schema.names))
		for _, name := range frame.schema.names {
			items = append(items, SelectItem{Name: name, Expr: ColumnRef{Name: name}})
		}
	}
	cols := make([]Column, 0, len(items))
	for _, item := range items {
		if item.Name == "" {
			return Frame{}, fmt.Errorf("select item name must not be empty")
		}
		if ref, ok := item.Expr.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return Frame{}, fmt.Errorf("unknown column %q", ref.Name)
			}
			gathered := col.Gather(indexes)
			if transferOwned {
				// Gather always materializes fresh storage, so the marked
				// column is safe to surrender to a single exporter.
				gathered = markTransferOwned(unwrapAttributedArray(gathered))
			}
			cols = append(cols, Column{Name: item.Name, Data: gathered})
			continue
		}
		if projector, ok := item.Expr.(vectorProjector); ok {
			array, err := projector.EvalRows(frame, indexes)
			if err != nil {
				return Frame{}, err
			}
			if array.Len() != len(indexes) {
				return Frame{}, fmt.Errorf("select item %q returned %d rows, want %d", item.Name, array.Len(), len(indexes))
			}
			cols = append(cols, Column{Name: item.Name, Data: array})
			continue
		}
		if array, ok, err := evalExprArray(frame, item.Expr); ok || err != nil {
			if err != nil {
				return Frame{}, err
			}
			cols = append(cols, Column{Name: item.Name, Data: array.Gather(indexes)})
			continue
		}
		values := make([]any, len(indexes))
		for i, row := range indexes {
			v, err := item.Expr.EvalRow(frame, row)
			if err != nil {
				return Frame{}, err
			}
			values[i] = v
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	return newFrameTrusted(cols...)
}

type groupState struct {
	keys []any
	aggs []aggregateState
}

type aggregateState struct {
	fn         string
	sum        float64
	sumsq      float64
	weight     float64
	values     []float64
	count      int64
	value      any
	hasValue   bool
	lastValue  any
	hasLastVal bool
}

type aggregateInput struct {
	Aggregate
	column       Array
	weightColumn Array
	binaryOp     Op
	leftColumn   Array
	rightColumn  Array
}

type groupInput struct {
	SelectItem
	column Array
}

func execGrouped(frame Frame, indexes []int, plan QueryPlan) (Frame, error) {
	byItems := groupByItems(plan)
	if len(byItems) == 0 {
		if len(plan.Aggregates) > 0 {
			return execUngroupedAggregates(frame, indexes, plan)
		}
		return Frame{}, fmt.Errorf("aggregate queries require at least one by column")
	}
	if len(plan.Aggregates) == 0 {
		if len(plan.Select) > 0 {
			return execGroupedProjection(frame, indexes, plan, byItems)
		}
		return Frame{}, fmt.Errorf("grouped queries require aggregates")
	}
	byInputs, err := bindGroupInputs(frame, byItems)
	if err != nil {
		return Frame{}, err
	}
	aggs := make([]aggregateInput, len(plan.Aggregates))
	for i, agg := range plan.Aggregates {
		aggs[i].Aggregate = agg
		if ref, ok := agg.Expr.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return Frame{}, fmt.Errorf("unknown column %q", ref.Name)
			}
			aggs[i].column = col
		}
		if bin, ok := agg.Expr.(Binary); ok {
			leftRef, leftOK := bin.Left.(ColumnRef)
			rightRef, rightOK := bin.Right.(ColumnRef)
			if leftOK && rightOK && isNumericBinaryAggregateOp(bin.Op) {
				leftCol, ok := frame.Column(leftRef.Name)
				if !ok {
					return Frame{}, fmt.Errorf("unknown column %q", leftRef.Name)
				}
				rightCol, ok := frame.Column(rightRef.Name)
				if !ok {
					return Frame{}, fmt.Errorf("unknown column %q", rightRef.Name)
				}
				aggs[i].binaryOp = bin.Op
				aggs[i].leftColumn = leftCol
				aggs[i].rightColumn = rightCol
			}
		}
		if ref, ok := agg.Weight.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return Frame{}, fmt.Errorf("unknown column %q", ref.Name)
			}
			aggs[i].weightColumn = col
		}
	}
	// indexes == nil means "all rows": the kernel skips materializing an
	// identity index vector for unfiltered grouped queries.
	allRows := indexes == nil || indexesCoverAllRows(indexes, frame.Len())
	if out, ok, err := execGroupedFusedNumeric(frame, indexes, allRows, byInputs, aggs); ok || err != nil {
		return out, err
	}
	if index, ok, err := groupIndexForSingleColumn(frame, byInputs); err != nil {
		return Frame{}, err
	} else if ok {
		if allRows && byInputs[0].column.Len() == frame.Len() {
			return execGroupedFromArrayIndex(frame, byInputs, aggs, index)
		}
		if indexes != nil {
			if out, ok, err := execGroupedFromFilteredArrayIndex(frame, byInputs, aggs, index, indexes); ok || err != nil {
				return out, err
			}
		}
	}
	if out, ok, err := execGroupedTypedRowIDs(frame, indexes, allRows, byInputs, aggs); ok || err != nil {
		return out, err
	}
	if indexes == nil {
		indexes = allIndexes(frame.Len())
	}
	groups := map[string]*groupState{}
	order := make([]string, 0)
	var keyBuilder strings.Builder
	for _, row := range indexes {
		keyVals := make([]any, len(byInputs))
		keyBuilder.Reset()
		for i, item := range byInputs {
			v, err := item.value(frame, row)
			if err != nil {
				return Frame{}, err
			}
			keyVals[i] = v
			appendKeyPart(&keyBuilder, item.keyKind(), v)
		}
		key := keyBuilder.String()
		state := groups[key]
		if state == nil {
			state = &groupState{keys: keyVals, aggs: make([]aggregateState, len(aggs))}
			for i, agg := range aggs {
				state.aggs[i].fn = agg.Func
			}
			groups[key] = state
			order = append(order, key)
		}
		for i, agg := range aggs {
			if err := accumulateAggregate(&state.aggs[i], agg, frame, row); err != nil {
				return Frame{}, err
			}
		}
	}

	cols := make([]Column, 0, len(byInputs)+len(plan.Aggregates))
	for i, item := range byInputs {
		values := make([]any, len(order))
		for row, key := range order {
			values[row] = groups[key].keys[i]
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	for i, agg := range plan.Aggregates {
		values := make([]any, len(order))
		for row, key := range order {
			values[row] = aggregateResult(groups[key].aggs[i])
		}
		cols = append(cols, aggregateOutputColumn(frame, agg, values))
	}
	return NewFrame(cols...)
}

// execUngroupedAggregates executes whole-table (no-by) aggregates over the
// filtered rows, producing the canonical one-row qSQL aggregate table.
// indexes == nil means "all rows" (same contract as execGrouped).
func execUngroupedAggregates(frame Frame, indexes []int, plan QueryPlan) (Frame, error) {
	if len(plan.Select) > 0 {
		return Frame{}, fmt.Errorf("ungrouped aggregate queries cannot mix aggregates with plain projections")
	}
	aggs, err := bindAggregateInputs(frame, plan.Aggregates)
	if err != nil {
		return Frame{}, err
	}
	states := make([]aggregateState, len(aggs))
	for i, agg := range aggs {
		states[i].fn = agg.Func
	}
	if !ungroupedAggregateStatesTyped(frame, indexes, aggs, states) {
		accumulateRow := func(row int) error {
			for i := range aggs {
				if err := accumulateAggregate(&states[i], aggs[i], frame, row); err != nil {
					return err
				}
			}
			return nil
		}
		if indexes == nil {
			for row := 0; row < frame.Len(); row++ {
				if err := accumulateRow(row); err != nil {
					return Frame{}, err
				}
			}
		} else {
			for _, row := range indexes {
				if err := accumulateRow(row); err != nil {
					return Frame{}, err
				}
			}
		}
	}
	cols := make([]Column, 0, len(plan.Aggregates))
	for i, agg := range plan.Aggregates {
		values := []any{aggregateResult(states[i])}
		cols = append(cols, aggregateOutputColumn(frame, agg, values))
	}
	return NewFrame(cols...)
}

func execGroupedFilteredWhere(frame Frame, plan QueryPlan) (Frame, bool, error) {
	if plan.Where == nil || plan.PreProjectOrder || len(plan.Aggregates) == 0 {
		return Frame{}, false, nil
	}
	byItems := groupByItems(plan)
	if len(byItems) == 0 {
		return Frame{}, false, nil
	}
	predicate, ok, err := filterRowPredicate(frame, plan.Where)
	if err != nil || !ok {
		return Frame{}, ok, err
	}
	byInputs, err := bindGroupInputs(frame, byItems)
	if err != nil {
		return Frame{}, true, err
	}
	aggs, err := bindAggregateInputs(frame, plan.Aggregates)
	if err != nil {
		return Frame{}, true, err
	}
	index, ok, err := groupIndexForSingleColumn(frame, byInputs)
	if err != nil || !ok {
		return Frame{}, ok, err
	}
	out, ok, err := execGroupedFromFilteredPredicateIndex(frame, byInputs, aggs, index, predicate)
	return out, ok, err
}

func bindAggregateInputs(frame Frame, aggregates []Aggregate) ([]aggregateInput, error) {
	aggs := make([]aggregateInput, len(aggregates))
	for i, agg := range aggregates {
		aggs[i].Aggregate = agg
		if ref, ok := agg.Expr.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return nil, fmt.Errorf("unknown column %q", ref.Name)
			}
			aggs[i].column = col
		}
		if bin, ok := agg.Expr.(Binary); ok {
			leftRef, leftOK := bin.Left.(ColumnRef)
			rightRef, rightOK := bin.Right.(ColumnRef)
			if leftOK && rightOK && isNumericBinaryAggregateOp(bin.Op) {
				leftCol, ok := frame.Column(leftRef.Name)
				if !ok {
					return nil, fmt.Errorf("unknown column %q", leftRef.Name)
				}
				rightCol, ok := frame.Column(rightRef.Name)
				if !ok {
					return nil, fmt.Errorf("unknown column %q", rightRef.Name)
				}
				aggs[i].binaryOp = bin.Op
				aggs[i].leftColumn = leftCol
				aggs[i].rightColumn = rightCol
			}
		}
		if ref, ok := agg.Weight.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return nil, fmt.Errorf("unknown column %q", ref.Name)
			}
			aggs[i].weightColumn = col
		}
	}
	return aggs, nil
}

type projectionGroup struct {
	rows      []int
	positions []int
}

func execGroupedProjection(frame Frame, indexes []int, plan QueryPlan, byItems []SelectItem) (Frame, error) {
	// indexes == nil means "all rows": the kernel skips materializing an
	// identity index vector for unfiltered grouped queries (same contract as
	// the grouped-aggregate path in execGrouped).
	if indexes == nil {
		indexes = allIndexes(frame.Len())
	}
	if !selectItemsNeedGroupedRows(plan.Select) {
		indexArray := queryIndexesArray(indexes, frame.Len())
		return execProjectByI64IndexArray(frame, indexArray, plan.Select)
	}
	byInputs, err := bindGroupInputs(frame, byItems)
	if err != nil {
		return Frame{}, err
	}
	groups := map[string]*projectionGroup{}
	order := make([]string, 0)
	var keyBuilder strings.Builder
	for pos, row := range indexes {
		keyBuilder.Reset()
		for _, item := range byInputs {
			v, err := item.value(frame, row)
			if err != nil {
				return Frame{}, err
			}
			appendKeyPart(&keyBuilder, item.keyKind(), v)
		}
		key := keyBuilder.String()
		group := groups[key]
		if group == nil {
			group = &projectionGroup{}
			groups[key] = group
			order = append(order, key)
		}
		group.rows = append(group.rows, row)
		group.positions = append(group.positions, pos)
	}

	if len(plan.Select) == 0 {
		return execProject(frame, indexes, nil)
	}
	cols := make([]Column, 0, len(plan.Select))
	for _, item := range plan.Select {
		if item.Name == "" {
			return Frame{}, fmt.Errorf("select item name must not be empty")
		}
		if ref, ok := item.Expr.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return Frame{}, fmt.Errorf("unknown column %q", ref.Name)
			}
			cols = append(cols, Column{Name: item.Name, Data: col.Gather(indexes)})
			continue
		}
		if projector, ok := item.Expr.(vectorProjector); ok {
			values := make([]any, len(indexes))
			for _, key := range order {
				group := groups[key]
				array, err := projector.EvalRows(frame, group.rows)
				if err != nil {
					return Frame{}, err
				}
				if array.Len() != len(group.rows) {
					return Frame{}, fmt.Errorf("select item %q returned %d grouped rows, want %d", item.Name, array.Len(), len(group.rows))
				}
				for i, pos := range group.positions {
					v, ok := array.At(i)
					if !ok {
						return Frame{}, fmt.Errorf("select item %q grouped row %d out of range", item.Name, i)
					}
					values[pos] = v
				}
			}
			cols = append(cols, NewColumn(item.Name, values))
			continue
		}
		if array, ok, err := evalExprArray(frame, item.Expr); ok || err != nil {
			if err != nil {
				return Frame{}, err
			}
			cols = append(cols, Column{Name: item.Name, Data: array.Gather(indexes)})
			continue
		}
		values := make([]any, len(indexes))
		for i, row := range indexes {
			v, err := item.Expr.EvalRow(frame, row)
			if err != nil {
				return Frame{}, err
			}
			values[i] = v
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	return NewFrame(cols...)
}

func selectItemsNeedGroupedRows(items []SelectItem) bool {
	for _, item := range items {
		if exprNeedsGroupedRows(item.Expr) {
			return true
		}
	}
	return false
}

func exprNeedsGroupedRows(expr Expr) bool {
	switch e := expr.(type) {
	case nil, ColumnRef, Literal:
		return false
	case vectorProjector:
		return true
	case Binary:
		return exprNeedsGroupedRows(e.Left) || exprNeedsGroupedRows(e.Right)
	case Conditional:
		return exprNeedsGroupedRows(e.Cond) || exprNeedsGroupedRows(e.Then) || exprNeedsGroupedRows(e.Else)
	case Logical:
		return exprNeedsGroupedRows(e.Left) || exprNeedsGroupedRows(e.Right)
	case Not:
		return exprNeedsGroupedRows(e.Expr)
	case In:
		return exprNeedsGroupedRows(e.Expr)
	case Within:
		return exprNeedsGroupedRows(e.Expr)
	case BucketFloorExpr:
		return exprNeedsGroupedRows(e.Expr)
	case ListAggregateExpr:
		return exprNeedsGroupedRows(e.Expr)
	default:
		return false
	}
}

func queryIndexesArray(indexes []int, frameLen int) Array {
	if indexesCoverAllRows(indexes, frameLen) {
		return i64RangeArray{start: 0, step: 1, len: frameLen}
	}
	values := make([]int64, len(indexes))
	for i, row := range indexes {
		values[i] = int64(row)
	}
	return newI64Trusted(values)
}

func groupIndexForSingleColumn(frame Frame, byInputs []groupInput) (ArrayIndex, bool, error) {
	if len(byInputs) != 1 {
		return ArrayIndex{}, false, nil
	}
	column := byInputs[0].column
	if column == nil {
		return ArrayIndex{}, false, nil
	}
	if index, ok := arrayIndexForBorrowed(column, ArrayAttributeUnique); ok {
		return index, true, nil
	}
	if index, ok := arrayIndexForBorrowed(column, ArrayAttributeGrouped); ok {
		return index, true, nil
	}
	if ref, ok := byInputs[0].Expr.(ColumnRef); ok {
		indexed := WithArrayAttribute(column, ArrayAttributeGrouped)
		if index, ok := arrayIndexForBorrowed(indexed, ArrayAttributeGrouped); ok {
			frame.columns[ref.Name] = indexed
			byInputs[0].column = indexed
			return index, true, nil
		}
	}
	index, err := BuildArrayIndex(column, ArrayAttributeGrouped)
	if err != nil {
		return ArrayIndex{}, false, err
	}
	return index, true, nil
}

func indexesCoverAllRows(indexes []int, n int) bool {
	if len(indexes) != n {
		return false
	}
	for i, row := range indexes {
		if row != i {
			return false
		}
	}
	return true
}

func aggregateInputsAllCount(aggs []aggregateInput) bool {
	if len(aggs) == 0 {
		return false
	}
	for _, agg := range aggs {
		if agg.Func != "count" {
			return false
		}
	}
	return true
}

func execGroupedFromArrayIndex(frame Frame, byInputs []groupInput, aggs []aggregateInput, index ArrayIndex) (Frame, error) {
	if out, ok, err := execTypedGroupedAggregateFrame(frame, byInputs, aggs, index, allIndexes(len(index.Rows)), nil, nil); ok || err != nil {
		return out, err
	}
	if out, ok, err := execGroupedTypedRowIDs(frame, nil, true, byInputs, aggs); ok || err != nil {
		return out, err
	}
	if states, ok, err := typedKernels.GroupAggregateStates(index, aggs); ok || err != nil {
		if err != nil {
			return Frame{}, err
		}
		return buildGroupedAggregateFrame(frame, byInputs, aggs, states, allIndexes(len(states)))
	}
	groupCounts, hasGroupCounts := typedKernels.GroupCounts(index)
	hasRowAggregates := false
	for _, agg := range aggs {
		if agg.Func != "count" || !hasGroupCounts {
			hasRowAggregates = true
			break
		}
	}
	states := make([]groupState, len(index.Rows))
	for group, rows := range index.Rows {
		states[group] = groupState{
			keys: []any{index.Keys[group]},
			aggs: make([]aggregateState, len(aggs)),
		}
		for i, agg := range aggs {
			states[group].aggs[i].fn = agg.Func
			if hasGroupCounts && agg.Func == "count" {
				states[group].aggs[i].count = groupCounts[group]
			}
		}
		if hasRowAggregates {
			for _, row := range rows {
				for i, agg := range aggs {
					if hasGroupCounts && agg.Func == "count" {
						continue
					}
					if err := accumulateAggregate(&states[group].aggs[i], agg, frame, row); err != nil {
						return Frame{}, err
					}
				}
			}
		}
	}

	return buildGroupedAggregateFrame(frame, byInputs, aggs, states, allIndexes(len(states)))
}

func execGroupedFromFilteredArrayIndex(frame Frame, byInputs []groupInput, aggs []aggregateInput, index ArrayIndex, indexes []int) (Frame, bool, error) {
	if out, handled, err := execTypedGroupedAggregateFrame(frame, byInputs, aggs, index, nil, indexes, nil); handled || err != nil {
		return out, handled, err
	}
	if out, ok, err := execSimpleFilteredGroupedAggregates(frame, byInputs, aggs, index, indexes, nil); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := execGroupedTypedRowIDs(frame, indexes, false, byInputs, aggs); ok || err != nil {
		return out, ok, err
	}
	if groupOrder, states, ok, err := typedKernels.FilteredGroupAggregateStates(index, indexes, aggs); ok || err != nil {
		if err != nil {
			return Frame{}, true, err
		}
		out, err := buildGroupedAggregateFrame(frame, byInputs, aggs, states, groupOrder)
		return out, true, err
	}
	if !aggregateInputsAllCount(aggs) {
		return Frame{}, false, nil
	}
	groupOrder, groupCounts, ok, err := typedKernels.FilteredGroupCounts(index, indexes)
	if err != nil || !ok {
		return Frame{}, ok, err
	}
	cols := make([]Column, 0, len(byInputs)+len(aggs))
	for i, item := range byInputs {
		if i > 0 {
			return Frame{}, false, nil
		}
		values := make([]any, len(groupOrder))
		for row, group := range groupOrder {
			values[row] = index.Keys[group]
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	for _, agg := range aggs {
		values := make([]any, len(groupOrder))
		for row, group := range groupOrder {
			values[row] = groupCounts[group]
		}
		cols = append(cols, NewColumn(agg.Name, values))
	}
	out, err := newFrameTrusted(cols...)
	return out, true, err
}

func execGroupedFromFilteredPredicateIndex(frame Frame, byInputs []groupInput, aggs []aggregateInput, index ArrayIndex, predicate rowPredicate) (Frame, bool, error) {
	if predicate == nil || !groupAggregatesSupportedByTypedIndex(aggs) {
		return Frame{}, false, nil
	}
	if out, handled, err := execTypedGroupedAggregateFrame(frame, byInputs, aggs, index, nil, nil, predicate); handled || err != nil {
		return out, handled, err
	}
	if out, ok, err := execSimpleFilteredGroupedAggregates(frame, byInputs, aggs, index, nil, predicate); ok || err != nil {
		return out, ok, err
	}
	states := make([]groupState, len(index.Rows))
	for group := range index.Rows {
		states[group] = groupState{
			keys: []any{index.Keys[group]},
			aggs: make([]aggregateState, len(aggs)),
		}
		for i, agg := range aggs {
			states[group].aggs[i].fn = agg.Func
		}
	}
	rowToGroup, err := rowToGroupFromIndex(index)
	if err != nil {
		return Frame{}, true, err
	}
	seen := make([]bool, len(index.Rows))
	groupOrder := make([]int, 0, len(index.Rows))
	for row := 0; row < frame.Len(); row++ {
		if !predicate(row) {
			continue
		}
		if row >= len(rowToGroup) {
			return Frame{}, true, fmt.Errorf("filter row %d out of range for grouped index", row)
		}
		group := rowToGroup[row]
		if group < 0 {
			return Frame{}, true, fmt.Errorf("filter row %d is missing from grouped index", row)
		}
		if !seen[group] {
			seen[group] = true
			groupOrder = append(groupOrder, group)
		}
		for i, agg := range aggs {
			if err := accumulateIndexedAggregateRow(&states[group].aggs[i], agg, row); err != nil {
				return Frame{}, true, err
			}
		}
	}
	out, err := buildGroupedAggregateFrame(frame, byInputs, aggs, states, groupOrder)
	return out, true, err
}

type typedGroupedAggregateAccumulator struct {
	input    aggregateInput
	valueFn  func(row int) (float64, bool)
	weightFn func(row int) (float64, bool)
	sum      []float64
	weight   []float64
	count    []int64
	value    []any
	hasVal   []bool
}

func execTypedGroupedAggregateFrame(frame Frame, byInputs []groupInput, aggs []aggregateInput, index ArrayIndex, groupOrder []int, indexes []int, predicate rowPredicate) (Frame, bool, error) {
	if len(byInputs) != 1 || len(aggs) == 0 || len(groupOrder) > len(index.Rows) {
		return Frame{}, false, nil
	}
	keyKind := byInputs[0].keyKind()
	if !isTypedGroupedAggregateKeyKind(keyKind) || !typedGroupedAggregateInputsSupported(aggs) {
		return Frame{}, false, nil
	}
	accumulators := make([]typedGroupedAggregateAccumulator, len(aggs))
	for i, agg := range aggs {
		accumulators[i] = typedGroupedAggregateAccumulator{
			input: agg,
			sum:   make([]float64, len(index.Rows)),
			count: make([]int64, len(index.Rows)),
		}
		if agg.Func == "wavg" {
			accumulators[i].weight = make([]float64, len(index.Rows))
			accumulators[i].valueFn = resolveAggNumericFn(agg)
			accumulators[i].weightFn = resolveNumericColumnFn(agg.weightColumn)
			if accumulators[i].valueFn == nil || accumulators[i].weightFn == nil {
				return Frame{}, false, nil
			}
		}
		if agg.Func == "min" || agg.Func == "max" {
			accumulators[i].value = make([]any, len(index.Rows))
			accumulators[i].hasVal = make([]bool, len(index.Rows))
		}
	}
	seen := []bool(nil)
	if indexes != nil || predicate != nil {
		seen = make([]bool, len(index.Rows))
		groupOrder = make([]int, 0, len(index.Rows))
	} else if groupOrder == nil {
		groupOrder = allIndexes(len(index.Rows))
	}
	accumulate := func(group, row int) error {
		if group < 0 || group >= len(index.Rows) {
			return fmt.Errorf("group %d out of range", group)
		}
		if seen != nil && !seen[group] {
			seen[group] = true
			groupOrder = append(groupOrder, group)
		}
		for i := range accumulators {
			acc := &accumulators[i]
			agg := acc.input
			switch agg.Func {
			case "count":
				acc.count[group]++
			case "sum":
				value, ok, err := aggregateIndexedNumericValue(agg, row)
				if err != nil {
					return err
				}
				if ok {
					acc.sum[group] += value
					acc.count[group]++
				}
			case "wavg":
				value, vok := acc.valueFn(row)
				weight, wok := acc.weightFn(row)
				if vok && wok {
					acc.sum[group] += value * weight
					acc.weight[group] += weight
					acc.count[group]++
				}
			case "min", "max":
				value, err := agg.value(frame, row)
				if err != nil {
					return err
				}
				if IsNull(value) {
					continue
				}
				if !acc.hasVal[group] ||
					(agg.Func == "min" && compare(value, acc.value[group]) < 0) ||
					(agg.Func == "max" && compare(value, acc.value[group]) > 0) {
					acc.value[group] = value
					acc.hasVal[group] = true
				}
			default:
				return fmt.Errorf("unsupported aggregate %q", agg.Func)
			}
		}
		return nil
	}
	if predicate != nil {
		rowToGroup, err := rowToGroupFromIndex(index)
		if err != nil {
			return Frame{}, true, err
		}
		for row := 0; row < frame.Len(); row++ {
			if !predicate(row) {
				continue
			}
			if row < 0 || row >= len(rowToGroup) {
				return Frame{}, true, fmt.Errorf("filter row %d out of range for grouped index", row)
			}
			if err := accumulate(rowToGroup[row], row); err != nil {
				return Frame{}, true, err
			}
		}
	} else if indexes != nil {
		rowToGroup, err := rowToGroupFromIndex(index)
		if err != nil {
			return Frame{}, true, err
		}
		for _, row := range indexes {
			if row < 0 || row >= len(rowToGroup) {
				return Frame{}, true, fmt.Errorf("filter row %d out of range for grouped index", row)
			}
			if err := accumulate(rowToGroup[row], row); err != nil {
				return Frame{}, true, err
			}
		}
	} else {
		for group, rows := range index.Rows {
			for _, row := range rows {
				if err := accumulate(group, row); err != nil {
					return Frame{}, true, err
				}
			}
		}
	}

	cols := make([]Column, 0, 1+len(aggs))
	keyColumn, ok, err := typedGroupedAggregateKeyColumn(byInputs[0].Name, keyKind, index.Keys, groupOrder)
	if err != nil || !ok {
		return Frame{}, ok, err
	}
	cols = append(cols, keyColumn)
	for _, acc := range accumulators {
		column, err := typedGroupedAggregateOutputColumn(frame, acc, groupOrder)
		if err != nil {
			return Frame{}, true, err
		}
		cols = append(cols, column)
	}
	out, err := newFrameTrusted(cols...)
	return out, true, err
}

func isTypedGroupedAggregateKeyKind(kind Kind) bool {
	switch kind {
	case KindSymbol, KindString, KindI8, KindI16, KindI32, KindI64:
		return true
	default:
		return false
	}
}

func typedGroupedAggregateInputsSupported(aggs []aggregateInput) bool {
	for _, agg := range aggs {
		switch agg.Func {
		case "count":
			if agg.Weight != nil {
				return false
			}
		case "sum":
			if agg.Weight != nil {
				return false
			}
			if agg.column != nil && isNumericArray(agg.column) {
				continue
			}
			if agg.leftColumn != nil && agg.rightColumn != nil && isNumericArray(agg.leftColumn) && isNumericArray(agg.rightColumn) {
				continue
			}
			return false
		case "wavg":
			if agg.Weight == nil || agg.weightColumn == nil || !isNumericArray(agg.weightColumn) {
				return false
			}
			if agg.column != nil && isNumericArray(agg.column) {
				continue
			}
			if agg.leftColumn != nil && agg.rightColumn != nil && isNumericArray(agg.leftColumn) && isNumericArray(agg.rightColumn) {
				continue
			}
			return false
		case "min", "max":
			if agg.Weight != nil {
				return false
			}
			if agg.column == nil || !isTypedMinMaxArray(agg.column) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func typedGroupedAggregateKeyColumn(name Symbol, kind Kind, keys []any, order []int) (Column, bool, error) {
	switch kind {
	case KindSymbol:
		values := make([]Symbol, len(order))
		for row, group := range order {
			value, ok := keys[group].(Symbol)
			if !ok {
				if text, textOK := keys[group].(string); textOK {
					value, ok = Symbol(text), true
				}
			}
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be symbol-compatible, got %T", group, keys[group])
			}
			values[row] = value
		}
		return Column{Name: name, Data: columnArray[Symbol]{kind: KindSymbol, data: values}}, true, nil
	case KindString:
		values := make([]string, len(order))
		for row, group := range order {
			value, ok := keys[group].(string)
			if !ok {
				if sym, symOK := keys[group].(Symbol); symOK {
					value, ok = string(sym), true
				}
			}
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be string-compatible, got %T", group, keys[group])
			}
			values[row] = value
		}
		return Column{Name: name, Data: columnArray[string]{kind: KindString, data: values}}, true, nil
	case KindI8:
		values := make([]int8, len(order))
		for row, group := range order {
			value, ok := signedGroupKey[int8](keys[group])
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i8-compatible, got %T", group, keys[group])
			}
			values[row] = value
		}
		return Column{Name: name, Data: columnArray[int8]{kind: KindI8, data: values}}, true, nil
	case KindI16:
		values := make([]int16, len(order))
		for row, group := range order {
			value, ok := signedGroupKey[int16](keys[group])
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i16-compatible, got %T", group, keys[group])
			}
			values[row] = value
		}
		return Column{Name: name, Data: columnArray[int16]{kind: KindI16, data: values}}, true, nil
	case KindI32:
		values := make([]int32, len(order))
		for row, group := range order {
			value, ok := signedGroupKey[int32](keys[group])
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i32-compatible, got %T", group, keys[group])
			}
			values[row] = value
		}
		return Column{Name: name, Data: columnArray[int32]{kind: KindI32, data: values}}, true, nil
	case KindI64:
		values := make([]int64, len(order))
		for row, group := range order {
			value, ok := signedGroupKey[int64](keys[group])
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i64-compatible, got %T", group, keys[group])
			}
			values[row] = value
		}
		return Column{Name: name, Data: columnArray[int64]{kind: KindI64, data: values}}, true, nil
	default:
		return Column{}, false, nil
	}
}

func typedGroupedAggregateKeyColumnIdentity(name Symbol, kind Kind, keys []any) (Column, bool, error) {
	switch kind {
	case KindSymbol:
		values := make([]Symbol, len(keys))
		for group, key := range keys {
			value, ok := key.(Symbol)
			if !ok {
				if text, textOK := key.(string); textOK {
					value, ok = Symbol(text), true
				}
			}
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be symbol-compatible, got %T", group, key)
			}
			values[group] = value
		}
		return Column{Name: name, Data: columnArray[Symbol]{kind: KindSymbol, data: values}}, true, nil
	case KindString:
		values := make([]string, len(keys))
		for group, key := range keys {
			value, ok := key.(string)
			if !ok {
				if sym, symOK := key.(Symbol); symOK {
					value, ok = string(sym), true
				}
			}
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be string-compatible, got %T", group, key)
			}
			values[group] = value
		}
		return Column{Name: name, Data: columnArray[string]{kind: KindString, data: values}}, true, nil
	case KindI8:
		values := make([]int8, len(keys))
		for group, key := range keys {
			value, ok := signedGroupKey[int8](key)
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i8-compatible, got %T", group, key)
			}
			values[group] = value
		}
		return Column{Name: name, Data: columnArray[int8]{kind: KindI8, data: values}}, true, nil
	case KindI16:
		values := make([]int16, len(keys))
		for group, key := range keys {
			value, ok := signedGroupKey[int16](key)
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i16-compatible, got %T", group, key)
			}
			values[group] = value
		}
		return Column{Name: name, Data: columnArray[int16]{kind: KindI16, data: values}}, true, nil
	case KindI32:
		values := make([]int32, len(keys))
		for group, key := range keys {
			value, ok := signedGroupKey[int32](key)
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i32-compatible, got %T", group, key)
			}
			values[group] = value
		}
		return Column{Name: name, Data: columnArray[int32]{kind: KindI32, data: values}}, true, nil
	case KindI64:
		values := make([]int64, len(keys))
		for group, key := range keys {
			value, ok := signedGroupKey[int64](key)
			if !ok {
				return Column{}, true, fmt.Errorf("group key %d must be i64-compatible, got %T", group, key)
			}
			values[group] = value
		}
		return Column{Name: name, Data: columnArray[int64]{kind: KindI64, data: values}}, true, nil
	default:
		return Column{}, false, nil
	}
}

func typedGroupedAggregateKeyArrayIdentity(kind Kind, keys []any) (Array, bool, error) {
	col, ok, err := typedGroupedAggregateKeyColumnIdentity("_", kind, keys)
	return col.Data, ok, err
}

func signedGroupKey[T signedScalar](value any) (T, bool) {
	switch v := value.(type) {
	case int:
		return T(v), true
	case int8:
		return T(v), true
	case int16:
		return T(v), true
	case int32:
		return T(v), true
	case int64:
		return T(v), true
	default:
		return 0, false
	}
}

func typedGroupedAggregateOutputColumn(frame Frame, acc typedGroupedAggregateAccumulator, order []int) (Column, error) {
	agg := acc.input.Aggregate
	switch agg.Func {
	case "count":
		values := make([]int64, len(order))
		for row, group := range order {
			values[row] = acc.count[group]
		}
		return Column{Name: agg.Name, Data: columnArray[int64]{kind: KindI64, data: values}}, nil
	case "sum":
		values := make([]float64, len(order))
		for row, group := range order {
			values[row] = acc.sum[group]
		}
		return sumOutputColumnFromF64(frame, agg, values), nil
	case "wavg":
		sums := make([]float64, len(order))
		weights := make([]float64, len(order))
		counts := make([]int64, len(order))
		for row, group := range order {
			sums[row] = acc.sum[group]
			weights[row] = acc.weight[group]
			counts[row] = acc.count[group]
		}
		return groupedWavgOutputColumn(agg.Name, sums, weights, counts), nil
	case "min", "max":
		values := make([]any, len(order))
		for row, group := range order {
			if acc.hasVal[group] {
				values[row] = acc.value[group]
			} else {
				values[row] = NullValue
			}
		}
		if kind, ok := aggregateOutputKind(frame, agg); ok {
			return columnWithKind(agg.Name, kind, values)
		}
		return NewColumn(agg.Name, values), nil
	default:
		return Column{}, fmt.Errorf("unsupported aggregate %q", agg.Func)
	}
}

type simpleGroupedAggregate struct {
	input aggregateInput
	sum   []float64
	count []int64
}

func execSimpleFilteredGroupedAggregates(frame Frame, byInputs []groupInput, aggs []aggregateInput, index ArrayIndex, indexes []int, predicate rowPredicate) (Frame, bool, error) {
	if len(byInputs) != 1 || len(aggs) == 0 || (indexes == nil && predicate == nil) {
		return Frame{}, false, nil
	}
	states := make([]simpleGroupedAggregate, len(aggs))
	for i, agg := range aggs {
		switch agg.Func {
		case "count":
		case "sum", "avg":
			if agg.column != nil {
				if !isNumericArray(agg.column) {
					return Frame{}, false, nil
				}
			} else if agg.leftColumn != nil && agg.rightColumn != nil {
				if !isNumericBinaryAggregateOp(agg.binaryOp) || !isNumericArray(agg.leftColumn) || !isNumericArray(agg.rightColumn) {
					return Frame{}, false, nil
				}
			} else {
				return Frame{}, false, nil
			}
		default:
			return Frame{}, false, nil
		}
		states[i].input = agg
		states[i].sum = make([]float64, len(index.Rows))
		states[i].count = make([]int64, len(index.Rows))
	}
	rowToGroup, err := rowToGroupFromIndex(index)
	if err != nil {
		return Frame{}, true, err
	}
	seen := make([]bool, len(index.Rows))
	groupOrder := make([]int, 0, len(index.Rows))
	accumulate := func(row int) error {
		if row < 0 || row >= len(rowToGroup) {
			return fmt.Errorf("filter row %d out of range for grouped index", row)
		}
		group := rowToGroup[row]
		if group < 0 {
			return fmt.Errorf("filter row %d is missing from grouped index", row)
		}
		if !seen[group] {
			seen[group] = true
			groupOrder = append(groupOrder, group)
		}
		for i := range states {
			agg := states[i].input
			switch agg.Func {
			case "count":
				states[i].count[group]++
			case "sum", "avg":
				value, ok, err := simpleGroupedAggregateNumericValue(agg, row)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				states[i].sum[group] += value
				states[i].count[group]++
			}
		}
		return nil
	}
	if predicate != nil {
		for row := 0; row < frame.Len(); row++ {
			if !predicate(row) {
				continue
			}
			if err := accumulate(row); err != nil {
				return Frame{}, true, err
			}
		}
	} else {
		for _, row := range indexes {
			if err := accumulate(row); err != nil {
				return Frame{}, true, err
			}
		}
	}
	out, err := buildSimpleGroupedAggregateFrame(frame, byInputs, states, index, groupOrder)
	return out, true, err
}

func simpleGroupedAggregateNumericValue(agg aggregateInput, row int) (float64, bool, error) {
	if agg.column != nil {
		return typedKernels.NumericAt(agg.column, row)
	}
	return aggregateBinaryNumericValue(agg, row)
}

func buildSimpleGroupedAggregateFrame(frame Frame, byInputs []groupInput, aggs []simpleGroupedAggregate, index ArrayIndex, order []int) (Frame, error) {
	cols := make([]Column, 0, len(byInputs)+len(aggs))
	for _, item := range byInputs {
		values := make([]any, len(order))
		for row, group := range order {
			values[row] = index.Keys[group]
		}
		if kind := item.keyKind(); kind != "" && kind != KindAny {
			col, err := columnWithKind(item.Name, kind, values)
			if err != nil {
				return Frame{}, err
			}
			cols = append(cols, col)
			continue
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	for _, state := range aggs {
		agg := state.input
		switch agg.Func {
		case "count":
			values := make([]int64, len(order))
			for row, group := range order {
				values[row] = state.count[group]
			}
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[int64]{kind: KindI64, data: values}})
		case "sum":
			values := make([]float64, len(order))
			for row, group := range order {
				values[row] = state.sum[group]
			}
			cols = append(cols, sumOutputColumnFromF64(frame, agg.Aggregate, values))
		case "avg":
			values := make([]float64, len(order))
			for row, group := range order {
				if count := state.count[group]; count > 0 {
					values[row] = state.sum[group] / float64(count)
				}
			}
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[float64]{kind: KindF64, data: values}})
		default:
			return Frame{}, fmt.Errorf("unsupported aggregate %q", agg.Func)
		}
	}
	return newFrameTrusted(cols...)
}

func buildGroupedAggregateFrame(frame Frame, byInputs []groupInput, aggs []aggregateInput, states []groupState, order []int) (Frame, error) {
	cols := make([]Column, 0, len(byInputs)+len(aggs))
	for i, item := range byInputs {
		values := make([]any, len(order))
		for row, group := range order {
			values[row] = states[group].keys[i]
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	for i, agg := range aggs {
		values := make([]any, len(order))
		for row, group := range order {
			values[row] = aggregateResult(states[group].aggs[i])
		}
		cols = append(cols, aggregateOutputColumn(frame, agg.Aggregate, values))
	}
	return newFrameTrusted(cols...)
}

func bindGroupInputs(frame Frame, items []SelectItem) ([]groupInput, error) {
	inputs := make([]groupInput, len(items))
	for i, item := range items {
		inputs[i].SelectItem = item
		if ref, ok := item.Expr.(ColumnRef); ok {
			col, ok := frame.Column(ref.Name)
			if !ok {
				return nil, fmt.Errorf("unknown column %q", ref.Name)
			}
			inputs[i].column = col
		}
	}
	return inputs, nil
}

func (item groupInput) value(frame Frame, row int) (any, error) {
	if item.column == nil {
		return item.Expr.EvalRow(frame, row)
	}
	v, ok := item.column.At(row)
	if !ok {
		ref := item.Expr.(ColumnRef)
		return nil, fmt.Errorf("column %q row %d out of range", ref.Name, row)
	}
	return v, nil
}

func (item groupInput) keyKind() Kind {
	if item.column == nil {
		return KindAny
	}
	return item.column.Kind()
}

func (agg aggregateInput) value(frame Frame, row int) (any, error) {
	if agg.column == nil {
		return agg.Expr.EvalRow(frame, row)
	}
	v, ok := agg.column.At(row)
	if !ok {
		ref := agg.Expr.(ColumnRef)
		return nil, fmt.Errorf("column %q row %d out of range", ref.Name, row)
	}
	return v, nil
}

func aggregateNumericValue(agg aggregateInput, frame Frame, row int) (float64, bool, error) {
	if agg.column != nil {
		return numericAt(agg.column, row)
	}
	if agg.leftColumn != nil && agg.rightColumn != nil {
		return aggregateBinaryNumericValue(agg, row)
	}
	v, err := agg.value(frame, row)
	if err != nil {
		return 0, false, err
	}
	if IsNull(v) {
		return 0, false, nil
	}
	n, ok := numeric(v)
	if !ok {
		return 0, false, fmt.Errorf("aggregate %s expects numeric expression, got %T (%v)", agg.Func, v, v)
	}
	return n, true, nil
}

func aggregateBinaryNumericValue(agg aggregateInput, row int) (float64, bool, error) {
	if value, ok, handled, err := aggregateBinaryNumericValueFast(agg.leftColumn, agg.binaryOp, agg.rightColumn, row); handled || err != nil {
		return value, ok, err
	}
	left, lok, err := typedKernels.NumericAt(agg.leftColumn, row)
	if err != nil || !lok {
		return 0, lok, err
	}
	right, rok, err := typedKernels.NumericAt(agg.rightColumn, row)
	if err != nil || !rok {
		return 0, rok, err
	}
	switch agg.binaryOp {
	case OpAdd:
		return left + right, true, nil
	case OpSub:
		return left - right, true, nil
	case OpMul:
		return left * right, true, nil
	case OpDiv:
		if right == 0 {
			return 0, false, nil
		}
		return left / right, true, nil
	default:
		return 0, false, fmt.Errorf("unsupported aggregate binary op %q", agg.binaryOp)
	}
}

func aggregateBinaryNumericValueFast(left Array, op Op, right Array, row int) (float64, bool, bool, error) {
	if leftAttr, ok := left.(attributedArray); ok {
		return aggregateBinaryNumericValueFast(leftAttr.array, op, right, row)
	}
	if rightAttr, ok := right.(attributedArray); ok {
		return aggregateBinaryNumericValueFast(left, op, rightAttr.array, row)
	}
	if op != OpMul {
		return 0, false, false, nil
	}
	switch l := left.(type) {
	case columnArray[float64]:
		if row < 0 || row >= len(l.data) {
			return 0, false, true, fmt.Errorf("left aggregate row %d out of range", row)
		}
		switch r := right.(type) {
		case columnArray[int64]:
			if row < 0 || row >= len(r.data) {
				return 0, false, true, fmt.Errorf("right aggregate row %d out of range", row)
			}
			return l.data[row] * float64(r.data[row]), true, true, nil
		case columnArray[float64]:
			if row < 0 || row >= len(r.data) {
				return 0, false, true, fmt.Errorf("right aggregate row %d out of range", row)
			}
			return l.data[row] * r.data[row], true, true, nil
		}
	case columnArray[int64]:
		if row < 0 || row >= len(l.data) {
			return 0, false, true, fmt.Errorf("left aggregate row %d out of range", row)
		}
		switch r := right.(type) {
		case columnArray[float64]:
			if row < 0 || row >= len(r.data) {
				return 0, false, true, fmt.Errorf("right aggregate row %d out of range", row)
			}
			return float64(l.data[row]) * r.data[row], true, true, nil
		case columnArray[int64]:
			if row < 0 || row >= len(r.data) {
				return 0, false, true, fmt.Errorf("right aggregate row %d out of range", row)
			}
			return float64(l.data[row] * r.data[row]), true, true, nil
		}
	}
	return 0, false, false, nil
}

func isNumericBinaryAggregateOp(op Op) bool {
	switch op {
	case OpAdd, OpSub, OpMul, OpDiv:
		return true
	default:
		return false
	}
}

func aggregateWeightValue(agg aggregateInput, frame Frame, row int) (float64, bool, error) {
	if agg.Weight == nil {
		return 0, false, fmt.Errorf("aggregate wavg expects a weight expression")
	}
	if agg.weightColumn != nil {
		return numericAt(agg.weightColumn, row)
	}
	v, err := agg.Weight.EvalRow(frame, row)
	if err != nil {
		return 0, false, err
	}
	if IsNull(v) {
		return 0, false, nil
	}
	n, ok := numeric(v)
	if !ok {
		return 0, false, fmt.Errorf("aggregate wavg expects numeric weight, got %T (%v)", v, v)
	}
	return n, true, nil
}

func isSupportedAggregate(fn string) bool {
	switch fn {
	case "count", "sum", "avg", "var", "dev", "med", "wavg", "min", "max", "first", "last":
		return true
	default:
		return false
	}
}

func accumulateAggregate(state *aggregateState, agg aggregateInput, frame Frame, row int) error {
	switch agg.Func {
	case "count":
		state.count++
	case "sum", "avg", "var", "dev", "med":
		n, ok, err := aggregateNumericValue(agg, frame, row)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		state.sum += n
		state.sumsq += n * n
		if agg.Func == "med" {
			state.values = append(state.values, n)
		}
		state.count++
	case "wavg":
		n, nok, err := aggregateNumericValue(agg, frame, row)
		if err != nil {
			return err
		}
		w, wok, err := aggregateWeightValue(agg, frame, row)
		if err != nil {
			return err
		}
		if !nok || !wok {
			return nil
		}
		state.sum += n * w
		state.weight += w
		state.count++
	case "min", "max":
		v, err := agg.value(frame, row)
		if err != nil {
			return err
		}
		if IsNull(v) {
			return nil
		}
		if !state.hasValue || (agg.Func == "min" && compare(v, state.value) < 0) || (agg.Func == "max" && compare(v, state.value) > 0) {
			state.value = v
			state.hasValue = true
		}
	case "first":
		if !state.hasValue {
			v, err := agg.value(frame, row)
			if err != nil {
				return err
			}
			state.value = normalizeAggregateValue(v)
			state.hasValue = true
		}
	case "last":
		v, err := agg.value(frame, row)
		if err != nil {
			return err
		}
		state.lastValue = normalizeAggregateValue(v)
		state.hasLastVal = true
	default:
		return fmt.Errorf("unsupported aggregate %q", agg.Func)
	}
	return nil
}

func aggregateResult(state aggregateState) any {
	switch state.fn {
	case "count":
		return state.count
	case "sum":
		return state.sum
	case "avg":
		if state.count == 0 {
			return float64(0)
		}
		return state.sum / float64(state.count)
	case "var":
		if state.count == 0 {
			return float64(0)
		}
		mean := state.sum / float64(state.count)
		return state.sumsq/float64(state.count) - mean*mean
	case "dev":
		if state.count == 0 {
			return float64(0)
		}
		mean := state.sum / float64(state.count)
		return math.Sqrt(state.sumsq/float64(state.count) - mean*mean)
	case "med":
		if len(state.values) == 0 {
			return NullValue
		}
		values := append([]float64(nil), state.values...)
		sort.Float64s(values)
		mid := len(values) / 2
		if len(values)%2 == 1 {
			return values[mid]
		}
		return (values[mid-1] + values[mid]) / 2
	case "wavg":
		if state.count == 0 || state.weight == 0 {
			return NullValue
		}
		return state.sum / state.weight
	case "min", "max":
		if state.hasValue {
			return state.value
		}
		return NullValue
	case "first":
		if state.hasValue {
			return state.value
		}
		return NullValue
	case "last":
		if state.hasLastVal {
			return state.lastValue
		}
		return NullValue
	default:
		return NullValue
	}
}

func aggregateOutputKind(frame Frame, agg Aggregate) (Kind, bool) {
	switch agg.Func {
	case "min", "max", "first", "last":
		ref, ok := agg.Expr.(ColumnRef)
		if !ok {
			return "", false
		}
		return frame.schema.Kind(ref.Name)
	case "sum":
		// Canonical q: sum over integer columns stays a long; only avg/var/
		// dev/med widen to float.
		if sumPreservesIntegerKind(frame, agg) {
			return KindI64, true
		}
		return "", false
	default:
		return "", false
	}
}

// sumPreservesIntegerKind reports whether a sum aggregate's accumulated f64
// totals should be emitted as an i64 column (integer source expression).
func sumPreservesIntegerKind(frame Frame, agg Aggregate) bool {
	if agg.Func != "sum" || agg.Weight != nil {
		return false
	}
	return aggregateExprIsIntegerKind(frame, agg.Expr)
}

func aggregateExprIsIntegerKind(frame Frame, expr Expr) bool {
	switch e := expr.(type) {
	case ColumnRef:
		kind, ok := frame.schema.Kind(e.Name)
		if !ok {
			return false
		}
		switch kind {
		case KindI8, KindI16, KindI32, KindI64:
			return true
		default:
			return false
		}
	case Binary:
		switch e.Op {
		case OpAdd, OpSub, OpMul:
			return aggregateExprIsIntegerKind(frame, e.Left) && aggregateExprIsIntegerKind(frame, e.Right)
		default:
			return false
		}
	default:
		return false
	}
}

// aggregateOutputColumn builds an aggregate result column, preserving the
// kind-stable output (integer sums stay i64; min/max/first/last keep the
// source kind) and falling back to a generic column when the values cannot be
// represented in the preferred kind (e.g. integer sums beyond float exactness).
func aggregateOutputColumn(frame Frame, agg Aggregate, values []any) Column {
	if kind, ok := aggregateOutputKind(frame, agg); ok {
		if col, err := columnWithKind(agg.Name, kind, values); err == nil {
			return col
		}
	}
	return NewColumn(agg.Name, values)
}

// sumOutputColumnFromF64 converts accumulated f64 group sums into the output
// column, emitting i64 storage for integer source expressions when every
// total is exactly representable. The caller hands over ownership of sums.
func sumOutputColumnFromF64(frame Frame, agg Aggregate, sums []float64) Column {
	if sumPreservesIntegerKind(frame, agg) {
		out := make([]int64, len(sums))
		exact := true
		for i, v := range sums {
			n := int64(v)
			if float64(n) != v {
				exact = false
				break
			}
			out[i] = n
		}
		if exact {
			return Column{Name: agg.Name, Data: columnArray[int64]{kind: KindI64, data: out}}
		}
	}
	return Column{Name: agg.Name, Data: columnArray[float64]{kind: KindF64, data: sums}}
}

func groupedWavgOutputColumn(name Symbol, sums, weights []float64, counts []int64) Column {
	values := make([]float64, len(sums))
	hasNull := false
	for i := range sums {
		if counts[i] == 0 || weights[i] == 0 {
			hasNull = true
			continue
		}
		values[i] = sums[i] / weights[i]
	}
	if !hasNull {
		return Column{Name: name, Data: columnArray[float64]{kind: KindF64, data: values}}
	}
	out := make([]any, len(sums))
	for i, value := range values {
		if counts[i] == 0 || weights[i] == 0 {
			out[i] = NullValue
			continue
		}
		out[i] = value
	}
	return NewColumn(name, out)
}

func groupByItems(plan QueryPlan) []SelectItem {
	if len(plan.ByExprs) > 0 {
		return plan.ByExprs
	}
	items := make([]SelectItem, 0, len(plan.By))
	for _, name := range plan.By {
		items = append(items, SelectItem{Name: name, Expr: ColumnRef{Name: name}})
	}
	return items
}

func normalizeAggregateValue(v any) any {
	if IsNull(v) {
		return NullValue
	}
	return v
}

func orderFrame(frame Frame, specs []OrderSpec) (Frame, error) {
	indexes, err := orderIndexes(frame, allIndexes(frame.Len()), specs)
	if err != nil {
		return Frame{}, err
	}
	return frame.Gather(indexes)
}

func orderFrameLimit(frame Frame, specs []OrderSpec, limit int) (Frame, error) {
	indexes, err := orderIndexesLimit(frame, allIndexes(frame.Len()), specs, limit)
	if err != nil {
		return Frame{}, err
	}
	return frame.Gather(indexes)
}

// SortFrameByColumns orders a frame by named columns using the same typed
// ordering path as qSQL order-by.
func SortFrameByColumns(frame Frame, names []Symbol, descending bool) (Frame, error) {
	if len(names) == 0 {
		return frame.Gather(allIndexes(frame.Len()))
	}
	if out, handled, err := TrySortFrameByColumns(frame, names, descending); err != nil || handled {
		if err != nil {
			return Frame{}, err
		}
		return out, nil
	}
	specs := make([]OrderSpec, len(names))
	for i, name := range names {
		specs[i] = OrderSpec{Column: name, Desc: descending}
	}
	return orderFrame(frame, specs)
}

// SortKeyedFrameByColumns orders the underlying frame without first cloning it,
// then rebuilds the keyed sidecar for the sorted frame.
func SortKeyedFrameByColumns(keyed KeyedFrame, names []Symbol, descending bool) (KeyedFrame, error) {
	frame, err := SortFrameByColumns(keyed.frame, names, descending)
	if err != nil {
		return KeyedFrame{}, err
	}
	return KeyBy(frame, keyed.keys...)
}

func orderIndexes(frame Frame, indexes []int, specs []OrderSpec) ([]int, error) {
	bound := make([]boundOrderSpec, len(specs))
	for i, spec := range specs {
		col, ok := frame.Column(spec.Column)
		if !ok {
			return nil, fmt.Errorf("order column %q does not exist", spec.Column)
		}
		// Densify lazy carriers up front: the comparison loop below touches
		// each bound column O(n log n) times, and MaterializeArray is free
		// for columns that are already dense.
		bound[i] = boundOrderSpec{spec: spec, column: MaterializeArray(col)}
	}
	if out, ok := orderIndexesBySortedAttribute(indexes, bound); ok {
		return out, nil
	}
	if len(bound) == 1 {
		if out, ok := orderIndexesTypedSingle(indexes, bound[0]); ok {
			return out, nil
		}
	}
	out := append([]int(nil), indexes...)
	sort.SliceStable(out, func(i, j int) bool {
		leftRow, rightRow := out[i], out[j]
		for _, spec := range bound {
			cmp := compareArrayRows(spec.column, leftRow, rightRow)
			if cmp == 0 {
				continue
			}
			if spec.spec.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
	return out, nil
}

func orderIndexesLimit(frame Frame, indexes []int, specs []OrderSpec, limit int) ([]int, error) {
	if limit < 0 || limit >= len(indexes) {
		return orderIndexes(frame, indexes, specs)
	}
	if limit == 0 {
		return []int{}, nil
	}
	bound := make([]boundOrderSpec, len(specs))
	for i, spec := range specs {
		col, ok := frame.Column(spec.Column)
		if !ok {
			return nil, fmt.Errorf("order column %q does not exist", spec.Column)
		}
		bound[i] = boundOrderSpec{spec: spec, column: MaterializeArray(col)}
	}
	if out, ok := orderIndexesBySortedAttribute(indexes, bound); ok {
		return out[:limit], nil
	}
	if len(bound) != 1 {
		return topKOrderIndexesMulti(indexes, bound, limit), nil
	}
	return topKOrderIndexes(indexes, bound[0], limit), nil
}

func orderIndexesBySortedAttribute(indexes []int, bound []boundOrderSpec) ([]int, bool) {
	if len(bound) != 1 {
		return nil, false
	}
	spec := bound[0]
	if !ArrayHasAttribute(spec.column, ArrayAttributeSorted) {
		return nil, false
	}
	out := append([]int(nil), indexes...)
	if spec.spec.Desc {
		for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
			out[left], out[right] = out[right], out[left]
		}
	}
	return out, true
}

type boundOrderSpec struct {
	spec   OrderSpec
	column Array
}

type orderTopKItem struct {
	row int
	seq int
}

type orderTopKHeap struct {
	items []orderTopKItem
	spec  boundOrderSpec
}

type orderTopKMultiHeap struct {
	items []orderTopKItem
	specs []boundOrderSpec
}

func (h orderTopKHeap) Len() int { return len(h.items) }

func (h orderTopKHeap) Less(i, j int) bool {
	return orderTopKBefore(h.spec, h.items[j], h.items[i])
}

func (h orderTopKHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *orderTopKHeap) Push(x any) {
	h.items = append(h.items, x.(orderTopKItem))
}

func (h *orderTopKHeap) Pop() any {
	old := h.items
	n := len(old)
	item := old[n-1]
	h.items = old[:n-1]
	return item
}

func (h orderTopKMultiHeap) Len() int { return len(h.items) }

func (h orderTopKMultiHeap) Less(i, j int) bool {
	return orderTopKBeforeMulti(h.specs, h.items[j], h.items[i])
}

func (h orderTopKMultiHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *orderTopKMultiHeap) Push(x any) {
	h.items = append(h.items, x.(orderTopKItem))
}

func (h *orderTopKMultiHeap) Pop() any {
	old := h.items
	n := len(old)
	item := old[n-1]
	h.items = old[:n-1]
	return item
}

func topKOrderIndexes(indexes []int, spec boundOrderSpec, limit int) []int {
	h := &orderTopKHeap{items: make([]orderTopKItem, 0, limit), spec: spec}
	for seq, row := range indexes {
		item := orderTopKItem{row: row, seq: seq}
		if h.Len() < limit {
			heap.Push(h, item)
			continue
		}
		if orderTopKBefore(spec, item, h.items[0]) {
			h.items[0] = item
			heap.Fix(h, 0)
		}
	}
	outItems := append([]orderTopKItem(nil), h.items...)
	sort.SliceStable(outItems, func(i, j int) bool {
		return orderTopKBefore(spec, outItems[i], outItems[j])
	})
	out := make([]int, len(outItems))
	for i, item := range outItems {
		out[i] = item.row
	}
	return out
}

func topKOrderIndexesMulti(indexes []int, specs []boundOrderSpec, limit int) []int {
	h := &orderTopKMultiHeap{items: make([]orderTopKItem, 0, limit), specs: specs}
	for seq, row := range indexes {
		item := orderTopKItem{row: row, seq: seq}
		if h.Len() < limit {
			heap.Push(h, item)
			continue
		}
		if orderTopKBeforeMulti(specs, item, h.items[0]) {
			h.items[0] = item
			heap.Fix(h, 0)
		}
	}
	outItems := append([]orderTopKItem(nil), h.items...)
	sort.SliceStable(outItems, func(i, j int) bool {
		return orderTopKBeforeMulti(specs, outItems[i], outItems[j])
	})
	out := make([]int, len(outItems))
	for i, item := range outItems {
		out[i] = item.row
	}
	return out
}

func orderTopKBefore(spec boundOrderSpec, left, right orderTopKItem) bool {
	cmp := compareArrayRows(spec.column, left.row, right.row)
	if cmp != 0 {
		if spec.spec.Desc {
			return cmp > 0
		}
		return cmp < 0
	}
	return left.seq < right.seq
}

func orderTopKBeforeMulti(specs []boundOrderSpec, left, right orderTopKItem) bool {
	for _, spec := range specs {
		cmp := compareArrayRows(spec.column, left.row, right.row)
		if cmp == 0 {
			continue
		}
		if spec.spec.Desc {
			return cmp > 0
		}
		return cmp < 0
	}
	return left.seq < right.seq
}

func compareArrayRows(array Array, leftRow, rightRow int) int {
	switch a := array.(type) {
	case attributedArray:
		return compareArrayRows(a.array, leftRow, rightRow)
	case columnArray[bool]:
		return compareBool(a.data[leftRow], a.data[rightRow])
	case columnArray[int8]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[int16]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[int32]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[int64]:
		return compareInt64(a.data[leftRow], a.data[rightRow])
	case columnArray[uint8]:
		return compareUint64(uint64(a.data[leftRow]), uint64(a.data[rightRow]))
	case columnArray[uint16]:
		return compareUint64(uint64(a.data[leftRow]), uint64(a.data[rightRow]))
	case columnArray[uint32]:
		return compareUint64(uint64(a.data[leftRow]), uint64(a.data[rightRow]))
	case columnArray[uint64]:
		return compareUint64(a.data[leftRow], a.data[rightRow])
	case columnArray[float32]:
		return compareFloat64(float64(a.data[leftRow]), float64(a.data[rightRow]))
	case columnArray[float64]:
		return compareFloat64(a.data[leftRow], a.data[rightRow])
	case columnArray[string]:
		return compareString(a.data[leftRow], a.data[rightRow])
	case columnArray[Symbol]:
		return compareString(string(a.data[leftRow]), string(a.data[rightRow]))
	case columnArray[Month]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[Date]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[DateTime]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[Timespan]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[Minute]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[Second]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[Time]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	case columnArray[Timestamp]:
		return compareInt64(int64(a.data[leftRow]), int64(a.data[rightRow]))
	default:
		lv, _ := array.At(leftRow)
		rv, _ := array.At(rightRow)
		return compare(lv, rv)
	}
}

func InferArray(values []any) Array {
	hasNull, hasValue := false, false
	typedNullKind := Kind("")
	allBool := true
	allI8, allI16, allI32, allI64 := true, true, true, true
	allU8, allU16, allU32, allU64 := true, true, true, true
	allF32, allF64, allNumber := true, true, true
	allString, allSymbol, allStringOrSymbol := true, true, true
	allMonth, allDate, allDateTime, allTimespan := true, true, true, true
	allMinute, allSecond, allTime, allTimestamp := true, true, true, true
	for _, v := range values {
		if IsNull(v) {
			hasNull = true
			if kind, ok := NullKind(v); ok && kind != KindNull {
				typedNullKind = mergeTypedNullArrayKind(typedNullKind, kind)
			}
			continue
		}
		hasValue = true
		if _, ok := v.(bool); !ok {
			allBool = false
		}
		if _, ok := v.(int8); !ok {
			allI8 = false
		}
		if _, ok := v.(int16); !ok {
			allI16 = false
		}
		if _, ok := v.(int32); !ok {
			allI32 = false
		}
		switch v.(type) {
		case int, int64:
		default:
			allI64 = false
		}
		if _, ok := v.(uint8); !ok {
			allU8 = false
		}
		if _, ok := v.(uint16); !ok {
			allU16 = false
		}
		if _, ok := v.(uint32); !ok {
			allU32 = false
		}
		if _, ok := v.(uint64); !ok {
			allU64 = false
		}
		if _, ok := v.(float32); !ok {
			allF32 = false
		}
		if _, ok := v.(float64); !ok {
			allF64 = false
		}
		if _, ok := numeric(v); !ok {
			allNumber = false
		}
		if _, ok := v.(string); !ok {
			allString = false
		}
		if _, ok := v.(Symbol); !ok {
			allSymbol = false
		}
		switch v.(type) {
		case string, Symbol:
		default:
			allStringOrSymbol = false
		}
		if _, ok := v.(Month); !ok {
			allMonth = false
		}
		if _, ok := v.(Date); !ok {
			allDate = false
		}
		if _, ok := v.(DateTime); !ok {
			allDateTime = false
		}
		if _, ok := v.(Timespan); !ok {
			allTimespan = false
		}
		if _, ok := v.(Minute); !ok {
			allMinute = false
		}
		if _, ok := v.(Second); !ok {
			allSecond = false
		}
		if _, ok := v.(Time); !ok {
			allTime = false
		}
		if _, ok := v.(Timestamp); !ok {
			allTimestamp = false
		}
	}
	if !hasValue {
		if typedNullKind != "" {
			if array, err := nullableArrayWithKind(typedNullKind, values); err == nil {
				return array
			}
		}
		return nullableArray{kind: KindNull, data: normalizeNulls(values)}
	}
	if typedNullKind != "" {
		if array, err := arrayWithKind(typedNullKind, values); err == nil {
			return array
		}
	}
	switch {
	case allBool:
		out := make([]bool, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(bool)
		}
		if hasNull {
			return newNullableArray(KindBool, values)
		}
		return NewBool(out)
	case allI8:
		out := make([]int8, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(int8)
		}
		if hasNull {
			return newNullableArray(KindI8, values)
		}
		return NewI8(out)
	case allI16:
		out := make([]int16, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(int16)
		}
		if hasNull {
			return newNullableArray(KindI16, values)
		}
		return NewI16(out)
	case allI32:
		out := make([]int32, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(int32)
		}
		if hasNull {
			return newNullableArray(KindI32, values)
		}
		return NewI32(out)
	case allI64:
		out := make([]int64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			switch x := v.(type) {
			case int:
				out[i] = int64(x)
			case int64:
				out[i] = x
			}
		}
		if hasNull {
			return newNullableArray(KindI64, values)
		}
		return NewI64(out)
	case allU8:
		out := make([]uint8, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint8)
		}
		if hasNull {
			return newNullableArray(KindU8, values)
		}
		return NewU8(out)
	case allU16:
		out := make([]uint16, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint16)
		}
		if hasNull {
			return newNullableArray(KindU16, values)
		}
		return NewU16(out)
	case allU32:
		out := make([]uint32, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint32)
		}
		if hasNull {
			return newNullableArray(KindU32, values)
		}
		return NewU32(out)
	case allU64:
		out := make([]uint64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint64)
		}
		if hasNull {
			return newNullableArray(KindU64, values)
		}
		return NewU64(out)
	case allF32:
		out := make([]float32, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(float32)
		}
		if hasNull {
			return newNullableArray(KindF32, values)
		}
		return NewF32(out)
	case allF64:
		out := make([]float64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(float64)
		}
		if hasNull {
			return newNullableArray(KindF64, values)
		}
		return NewF64(out)
	case allNumber:
		out := make([]float64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i], _ = numeric(v)
		}
		if hasNull {
			return newNullableArray(KindF64, values)
		}
		return NewF64(out)
	case allString:
		out := make([]string, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(string)
		}
		if hasNull {
			return newNullableArray(KindString, values)
		}
		return NewString(out)
	case allSymbol:
		out := make([]string, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = string(v.(Symbol))
		}
		if hasNull {
			return newNullableArray(KindSymbol, values)
		}
		return NewSymbols(out)
	case allStringOrSymbol:
		out := make([]string, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			switch x := v.(type) {
			case string:
				out[i] = x
			case Symbol:
				out[i] = string(x)
			}
		}
		if hasNull {
			return newNullableArray(KindSymbol, values)
		}
		return NewSymbols(out)
	case allMonth:
		out := make([]Month, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Month)
		}
		if hasNull {
			return newNullableArray(KindMonth, values)
		}
		return NewMonth(out)
	case allDate:
		out := make([]Date, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Date)
		}
		if hasNull {
			return newNullableArray(KindDate, values)
		}
		return NewDate(out)
	case allDateTime:
		out := make([]DateTime, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(DateTime)
		}
		if hasNull {
			return newNullableArray(KindDateTime, values)
		}
		return NewDateTime(out)
	case allTimespan:
		out := make([]Timespan, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Timespan)
		}
		if hasNull {
			return newNullableArray(KindTimespan, values)
		}
		return NewTimespan(out)
	case allMinute:
		out := make([]Minute, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Minute)
		}
		if hasNull {
			return newNullableArray(KindMinute, values)
		}
		return NewMinute(out)
	case allSecond:
		out := make([]Second, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Second)
		}
		if hasNull {
			return newNullableArray(KindSecond, values)
		}
		return NewSecond(out)
	case allTime:
		out := make([]Time, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Time)
		}
		if hasNull {
			return newNullableArray(KindTime, values)
		}
		return NewTime(out)
	case allTimestamp:
		out := make([]Timestamp, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Timestamp)
		}
		if hasNull {
			return newNullableArray(KindTimestamp, values)
		}
		return NewTimestamp(out)
	default:
		return NewAny(values)
	}
}

func mergeTypedNullArrayKind(left, right Kind) Kind {
	if left == "" || left == KindNull || left == KindAny {
		return right
	}
	if right == "" || right == KindNull || right == KindAny || left == right {
		return left
	}
	if isNumericKind(left) && isNumericKind(right) {
		if left == KindF64 || right == KindF64 || left == KindF32 || right == KindF32 {
			return KindF64
		}
		return KindI64
	}
	return ""
}

func isNumericKind(kind Kind) bool {
	switch kind {
	case KindI8, KindI16, KindI32, KindI64, KindU8, KindU16, KindU32, KindU64, KindF32, KindF64:
		return true
	default:
		return false
	}
}

func columnWithKind(name Symbol, kind Kind, values []any) (Column, error) {
	if kind == "" || kind == KindAny {
		return NewColumn(name, values), nil
	}
	array, err := arrayWithKind(kind, values)
	if err != nil {
		return Column{}, fmt.Errorf("column %q: %w", name, err)
	}
	return Column{Name: name, Data: array}, nil
}

func arrayWithKind(kind Kind, values []any) (Array, error) {
	hasNull := false
	for _, v := range values {
		if IsNull(v) {
			hasNull = true
			break
		}
	}
	if hasNull {
		return nullableArrayWithKind(kind, values)
	}
	switch kind {
	case KindBool:
		out := make([]bool, len(values))
		for i, v := range values {
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("value %d must be bool for %s", i+1, kind)
			}
			out[i] = b
		}
		return NewBool(out), nil
	case KindI8:
		out := make([]int8, len(values))
		for i, v := range values {
			n, ok := coerceInt64Exact(v)
			if ok && n >= -128 && n <= 127 {
				out[i] = int8(n)
				continue
			}
			x, ok := v.(int8)
			if !ok {
				return nil, fmt.Errorf("value %d must be i8 for %s", i+1, kind)
			}
			out[i] = x
		}
		return NewI8(out), nil
	case KindI16:
		out := make([]int16, len(values))
		for i, v := range values {
			n, ok := coerceInt64Exact(v)
			if ok && n >= -32768 && n <= 32767 {
				out[i] = int16(n)
				continue
			}
			x, ok := v.(int16)
			if !ok {
				return nil, fmt.Errorf("value %d must be i16 for %s", i+1, kind)
			}
			out[i] = x
		}
		return NewI16(out), nil
	case KindI32:
		out := make([]int32, len(values))
		for i, v := range values {
			n, ok := coerceInt64Exact(v)
			if ok && n >= -2147483648 && n <= 2147483647 {
				out[i] = int32(n)
				continue
			}
			x, ok := v.(int32)
			if !ok {
				return nil, fmt.Errorf("value %d must be i32 for %s", i+1, kind)
			}
			out[i] = x
		}
		return NewI32(out), nil
	case KindI64:
		out := make([]int64, len(values))
		for i, v := range values {
			if n, ok := coerceInt64Exact(v); ok {
				out[i] = n
				continue
			}
			return nil, fmt.Errorf("value %d must be i64-compatible for %s", i+1, kind)
		}
		return NewI64(out), nil
	case KindU8:
		out := make([]uint8, len(values))
		for i, v := range values {
			n, ok := v.(uint8)
			if !ok {
				return nil, fmt.Errorf("value %d must be u8 for %s", i+1, kind)
			}
			out[i] = n
		}
		return NewU8(out), nil
	case KindU16:
		out := make([]uint16, len(values))
		for i, v := range values {
			n, ok := v.(uint16)
			if !ok {
				return nil, fmt.Errorf("value %d must be u16 for %s", i+1, kind)
			}
			out[i] = n
		}
		return NewU16(out), nil
	case KindU32:
		out := make([]uint32, len(values))
		for i, v := range values {
			n, ok := v.(uint32)
			if !ok {
				return nil, fmt.Errorf("value %d must be u32 for %s", i+1, kind)
			}
			out[i] = n
		}
		return NewU32(out), nil
	case KindU64:
		out := make([]uint64, len(values))
		for i, v := range values {
			n, ok := v.(uint64)
			if !ok {
				return nil, fmt.Errorf("value %d must be u64 for %s", i+1, kind)
			}
			out[i] = n
		}
		return NewU64(out), nil
	case KindF32:
		out := make([]float32, len(values))
		for i, v := range values {
			switch n := v.(type) {
			case float32:
				out[i] = n
			default:
				return nil, fmt.Errorf("value %d must be f32 for %s", i+1, kind)
			}
		}
		return NewF32(out), nil
	case KindF64:
		out := make([]float64, len(values))
		for i, v := range values {
			n, ok := numeric(v)
			if !ok {
				return nil, fmt.Errorf("value %d must be numeric for %s", i+1, kind)
			}
			out[i] = n
		}
		return NewF64(out), nil
	case KindString:
		out := make([]string, len(values))
		for i, v := range values {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("value %d must be string for %s", i+1, kind)
			}
			out[i] = s
		}
		return NewString(out), nil
	case KindSymbol:
		out := make([]string, len(values))
		for i, v := range values {
			switch s := v.(type) {
			case Symbol:
				out[i] = string(s)
			case string:
				out[i] = s
			default:
				return nil, fmt.Errorf("value %d must be symbol-compatible for %s", i+1, kind)
			}
		}
		return NewSymbols(out), nil
	case KindMonth:
		out := make([]Month, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be month for %s", i+1, kind)
			}
			out[i] = n.(Month)
		}
		return NewMonth(out), nil
	case KindDate:
		out := make([]Date, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be date for %s", i+1, kind)
			}
			out[i] = n.(Date)
		}
		return NewDate(out), nil
	case KindDateTime:
		out := make([]DateTime, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be datetime for %s", i+1, kind)
			}
			out[i] = n.(DateTime)
		}
		return NewDateTime(out), nil
	case KindTimespan:
		out := make([]Timespan, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be timespan for %s", i+1, kind)
			}
			out[i] = n.(Timespan)
		}
		return NewTimespan(out), nil
	case KindMinute:
		out := make([]Minute, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be minute for %s", i+1, kind)
			}
			out[i] = n.(Minute)
		}
		return NewMinute(out), nil
	case KindSecond:
		out := make([]Second, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be second for %s", i+1, kind)
			}
			out[i] = n.(Second)
		}
		return NewSecond(out), nil
	case KindTime:
		out := make([]Time, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be time for %s", i+1, kind)
			}
			out[i] = n.(Time)
		}
		return NewTime(out), nil
	case KindTimestamp:
		out := make([]Timestamp, len(values))
		for i, v := range values {
			n, err := normalizeScalarForKind(kind, v, i)
			if err != nil {
				return nil, fmt.Errorf("value %d must be timestamp for %s", i+1, kind)
			}
			out[i] = n.(Timestamp)
		}
		return NewTimestamp(out), nil
	default:
		return InferArray(values), nil
	}
}

func coerceInt64Exact(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		// isIntegerArray admits unsigned columns (byte casts, ...); coerce
		// them here too so lazy integer carriers built over them (fills,
		// scalar dyadics) never defer a "want integer" error into a
		// materialization panic.
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case float32:
		f := float64(n)
		i := int64(f)
		return i, float64(i) == f
	case float64:
		i := int64(n)
		return i, float64(i) == n
	default:
		return 0, false
	}
}

func bucketInt64Interval(kind Kind, interval any) (int64, error) {
	if width, ok, err := temporalBucketInt64Interval(kind, interval); ok || err != nil {
		return width, err
	}
	width, ok := coerceInt64Exact(interval)
	if !ok {
		return 0, fmt.Errorf("bucket floor interval must be an integer width")
	}
	if width <= 0 {
		return 0, fmt.Errorf("bucket floor interval must be positive")
	}
	return width, nil
}

func temporalBucketInt64Interval(kind Kind, interval any) (int64, bool, error) {
	nanos, nanosOK := temporalIntervalNanos(interval)
	switch kind {
	case KindDateTime, KindTimestamp, KindTimespan, KindTime:
		if nanosOK {
			if nanos <= 0 {
				return 0, true, fmt.Errorf("bucket floor interval must be positive")
			}
			return nanos, true, nil
		}
	case KindSecond:
		if nanosOK {
			if nanos <= 0 {
				return 0, true, fmt.Errorf("bucket floor interval must be positive")
			}
			if nanos%1_000_000_000 != 0 {
				return 0, true, fmt.Errorf("bucket floor interval must align to whole seconds")
			}
			return nanos / 1_000_000_000, true, nil
		}
	case KindMinute:
		if nanosOK {
			if nanos <= 0 {
				return 0, true, fmt.Errorf("bucket floor interval must be positive")
			}
			if nanos%(60*1_000_000_000) != 0 {
				return 0, true, fmt.Errorf("bucket floor interval must align to whole minutes")
			}
			return nanos / (60 * 1_000_000_000), true, nil
		}
	case KindDate:
		if nanosOK {
			if nanos <= 0 {
				return 0, true, fmt.Errorf("bucket floor interval must be positive")
			}
			if nanos%nanosPerDay != 0 {
				return 0, true, fmt.Errorf("bucket floor interval must align to whole days")
			}
			return nanos / nanosPerDay, true, nil
		}
	case KindMonth:
		if width, ok := interval.(Month); ok {
			if width.Months() <= 0 {
				return 0, true, fmt.Errorf("bucket floor interval must be positive")
			}
			return width.Months(), true, nil
		}
	}
	return 0, false, nil
}

func temporalIntervalNanos(interval any) (int64, bool) {
	switch x := interval.(type) {
	case Timespan:
		return x.Nanos(), true
	case Time:
		return x.Nanos(), true
	case Second:
		return x.Seconds() * 1_000_000_000, true
	case Minute:
		return x.Minutes() * 60 * 1_000_000_000, true
	case string:
		return parseTimespanNanos(x)
	default:
		return 0, false
	}
}

func bucketUint64Interval(interval any) (uint64, error) {
	switch n := interval.(type) {
	case uint8:
		if n == 0 {
			return 0, fmt.Errorf("bucket floor interval must be positive")
		}
		return uint64(n), nil
	case uint16:
		if n == 0 {
			return 0, fmt.Errorf("bucket floor interval must be positive")
		}
		return uint64(n), nil
	case uint32:
		if n == 0 {
			return 0, fmt.Errorf("bucket floor interval must be positive")
		}
		return uint64(n), nil
	case uint64:
		if n == 0 {
			return 0, fmt.Errorf("bucket floor interval must be positive")
		}
		return n, nil
	}
	width, ok := coerceInt64Exact(interval)
	if !ok || width <= 0 {
		if ok {
			return 0, fmt.Errorf("bucket floor interval must be positive")
		}
		return 0, fmt.Errorf("bucket floor interval must be an integer width")
	}
	return uint64(width), nil
}

func bucketFloat64Interval(interval any) (float64, error) {
	width, ok := numeric(interval)
	if !ok {
		return 0, fmt.Errorf("bucket floor interval must be numeric")
	}
	if width <= 0 || math.IsNaN(width) || math.IsInf(width, 0) {
		return 0, fmt.Errorf("bucket floor interval must be finite and positive")
	}
	return width, nil
}

func bucketFloorInt64Value(kind Kind, v any, width int64) (any, error) {
	value, err := int64BucketInput(kind, v)
	if err != nil {
		return nil, err
	}
	bucket := floorInt64(value, width)
	switch kind {
	case KindI8:
		if bucket < -128 || bucket > 127 {
			return nil, fmt.Errorf("bucket %d overflows %s", bucket, kind)
		}
		return int8(bucket), nil
	case KindI16:
		if bucket < -32768 || bucket > 32767 {
			return nil, fmt.Errorf("bucket %d overflows %s", bucket, kind)
		}
		return int16(bucket), nil
	case KindI32:
		if bucket < -2147483648 || bucket > 2147483647 {
			return nil, fmt.Errorf("bucket %d overflows %s", bucket, kind)
		}
		return int32(bucket), nil
	case KindI64:
		return bucket, nil
	case KindMonth:
		return Month(bucket), nil
	case KindDate:
		return Date(bucket), nil
	case KindDateTime:
		return DateTime(bucket), nil
	case KindTimespan:
		return Timespan(bucket), nil
	case KindMinute:
		return Minute(bucket), nil
	case KindSecond:
		return Second(bucket), nil
	case KindTime:
		return Time(bucket), nil
	case KindTimestamp:
		return Timestamp(bucket), nil
	default:
		return nil, fmt.Errorf("kind %s is not integer-like", kind)
	}
}

func int64BucketInput(kind Kind, v any) (int64, error) {
	switch kind {
	case KindI8:
		n, ok := v.(int8)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindI16:
		n, ok := v.(int16)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindI32:
		n, ok := v.(int32)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindI64:
		n, ok := v.(int64)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return n, nil
	case KindMonth:
		n, ok := v.(Month)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindDate:
		n, ok := v.(Date)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindDateTime:
		n, ok := v.(DateTime)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindTimespan:
		n, ok := v.(Timespan)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindMinute:
		n, ok := v.(Minute)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindSecond:
		n, ok := v.(Second)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindTime:
		n, ok := v.(Time)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	case KindTimestamp:
		n, ok := v.(Timestamp)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return int64(n), nil
	default:
		return 0, fmt.Errorf("kind %s is not integer-like", kind)
	}
}

func bucketFloorUint64Value(kind Kind, v any, width uint64) (any, error) {
	value, err := uint64BucketInput(kind, v)
	if err != nil {
		return nil, err
	}
	bucket := (value / width) * width
	switch kind {
	case KindU8:
		return uint8(bucket), nil
	case KindU16:
		return uint16(bucket), nil
	case KindU32:
		return uint32(bucket), nil
	case KindU64:
		return bucket, nil
	default:
		return nil, fmt.Errorf("kind %s is not unsigned integer-like", kind)
	}
}

func uint64BucketInput(kind Kind, v any) (uint64, error) {
	switch kind {
	case KindU8:
		n, ok := v.(uint8)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return uint64(n), nil
	case KindU16:
		n, ok := v.(uint16)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return uint64(n), nil
	case KindU32:
		n, ok := v.(uint32)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return uint64(n), nil
	case KindU64:
		n, ok := v.(uint64)
		if !ok {
			return 0, fmt.Errorf("value must be %s", kind)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("kind %s is not unsigned integer-like", kind)
	}
}

func bucketFloorFloatValue(kind Kind, v any, width float64) (any, error) {
	value, ok := numeric(v)
	if !ok {
		return nil, fmt.Errorf("value must be numeric")
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("value must be finite")
	}
	bucket := math.Floor(value/width) * width
	switch kind {
	case KindF32:
		return float32(bucket), nil
	case KindF64:
		return bucket, nil
	default:
		return nil, fmt.Errorf("kind %s is not float-like", kind)
	}
}

func floorInt64(value, width int64) int64 {
	quotient := value / width
	remainder := value % width
	if remainder != 0 && value < 0 {
		quotient--
	}
	return quotient * width
}

func newNullableArray(kind Kind, values []any) Array {
	array, err := nullableArrayWithKind(kind, values)
	if err != nil {
		panic(err)
	}
	return array
}

func nullableArrayWithKind(kind Kind, values []any) (Array, error) {
	if nullBitmapKindSupported(kind) {
		return nullBitmapArrayWithKind(kind, values)
	}
	out := make([]any, len(values))
	for i, v := range values {
		if IsNull(v) {
			out[i] = NullValue
			continue
		}
		normalized, err := normalizeScalarForKind(kind, v, i)
		if err != nil {
			return nil, err
		}
		out[i] = normalized
	}
	return nullableArray{kind: kind, data: out}, nil
}

func normalizeNulls(values []any) []any {
	out := make([]any, len(values))
	for i, v := range values {
		if IsNull(v) {
			out[i] = NullValue
		} else {
			out[i] = v
		}
	}
	return out
}

func normalizeScalar(kind Kind, v any) any {
	if normalized, ok := coerceTemporalValue(kind, v); ok {
		return normalized
	}
	normalized, err := NormalizeValueForKind(kind, v)
	if err == nil {
		return normalized
	}
	return v
}

func NormalizeValueForKind(kind Kind, v any) (any, error) {
	if IsNull(v) {
		if typedKind, ok := NullKind(v); ok && typedKind != KindNull {
			return NullForKind(kind), nil
		}
		return NullValue, nil
	}
	if kind == KindAny {
		return v, nil
	}
	if kind == KindNull {
		return nil, fmt.Errorf("value must be null for %s", kind)
	}
	return normalizeScalarForKind(kind, v, 0)
}

func normalizeScalarForKind(kind Kind, v any, index int) (any, error) {
	switch kind {
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("value %d must be bool for %s", index+1, kind)
		}
		return b, nil
	case KindI8:
		if n, ok := coerceInt64Exact(v); ok && n >= -128 && n <= 127 {
			return int8(n), nil
		}
		return nil, fmt.Errorf("value %d must be i8 for %s", index+1, kind)
	case KindI16:
		if n, ok := coerceInt64Exact(v); ok && n >= -32768 && n <= 32767 {
			return int16(n), nil
		}
		return nil, fmt.Errorf("value %d must be i16 for %s", index+1, kind)
	case KindI32:
		if n, ok := coerceInt64Exact(v); ok && n >= -2147483648 && n <= 2147483647 {
			return int32(n), nil
		}
		return nil, fmt.Errorf("value %d must be i32 for %s", index+1, kind)
	case KindI64:
		if n, ok := coerceInt64Exact(v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be i64-compatible for %s", index+1, kind)
	case KindU8:
		if n, ok := v.(uint8); ok {
			return n, nil
		}
		if n, ok := coerceInt64Exact(v); ok && n >= 0 && n <= 255 {
			return uint8(n), nil
		}
		return nil, fmt.Errorf("value %d must be u8-compatible for %s", index+1, kind)
	case KindU16:
		n, ok := v.(uint16)
		if !ok {
			return nil, fmt.Errorf("value %d must be u16 for %s", index+1, kind)
		}
		return n, nil
	case KindU32:
		n, ok := v.(uint32)
		if !ok {
			return nil, fmt.Errorf("value %d must be u32 for %s", index+1, kind)
		}
		return n, nil
	case KindU64:
		n, ok := v.(uint64)
		if !ok {
			return nil, fmt.Errorf("value %d must be u64 for %s", index+1, kind)
		}
		return n, nil
	case KindF32:
		if n, ok := v.(float32); ok {
			return n, nil
		}
		n, ok := numeric(v)
		if !ok {
			return nil, fmt.Errorf("value %d must be f32 for %s", index+1, kind)
		}
		return float32(n), nil
	case KindF64:
		n, ok := numeric(v)
		if !ok {
			return nil, fmt.Errorf("value %d must be numeric for %s", index+1, kind)
		}
		return n, nil
	case KindString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("value %d must be string for %s", index+1, kind)
		}
		return s, nil
	case KindSymbol:
		switch s := v.(type) {
		case Symbol:
			return s, nil
		case string:
			return Symbol(s), nil
		default:
			return nil, fmt.Errorf("value %d must be symbol-compatible for %s", index+1, kind)
		}
	case KindMonth:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be month for %s", index+1, kind)
	case KindDate:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be date for %s", index+1, kind)
	case KindDateTime:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be datetime for %s", index+1, kind)
	case KindTimespan:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be timespan for %s", index+1, kind)
	case KindMinute:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be minute for %s", index+1, kind)
	case KindSecond:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be second for %s", index+1, kind)
	case KindTime:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be time for %s", index+1, kind)
	case KindTimestamp:
		if n, ok := coerceTemporalTypedValue(kind, v); ok {
			return n, nil
		}
		return nil, fmt.Errorf("value %d must be timestamp for %s", index+1, kind)
	}
	return v, nil
}

func coerceTemporalTypedValue(kind Kind, v any) (any, bool) {
	switch x := v.(type) {
	case Month:
		return x, kind == KindMonth
	case Date:
		return x, kind == KindDate
	case DateTime:
		return x, kind == KindDateTime
	case Timespan:
		return x, kind == KindTimespan
	case Minute:
		return x, kind == KindMinute
	case Second:
		return x, kind == KindSecond
	case Time:
		return x, kind == KindTime
	case Timestamp:
		return x, kind == KindTimestamp
	default:
		return nil, false
	}
}

func coerceTemporalValue(kind Kind, v any) (any, bool) {
	switch x := v.(type) {
	case Month:
		return x, kind == KindMonth
	case Date:
		return x, kind == KindDate
	case DateTime:
		return x, kind == KindDateTime
	case Timespan:
		return x, kind == KindTimespan
	case Minute:
		return x, kind == KindMinute
	case Second:
		return x, kind == KindSecond
	case Time:
		return x, kind == KindTime
	case Timestamp:
		return x, kind == KindTimestamp
	case string:
		return parseTemporalString(kind, x)
	}
	return nil, false
}

func parseTemporalString(kind Kind, s string) (any, bool) {
	switch kind {
	case KindMonth:
		for _, layout := range []string{"2006.01", "2006-01"} {
			if tm, err := time.Parse(layout, s); err == nil {
				return MonthFromMonths(int64((tm.Year()-1970)*12 + int(tm.Month()) - 1)), true
			}
		}
	case KindDate:
		for _, layout := range []string{"2006.01.02", "2006-01-02"} {
			if tm, err := time.Parse(layout, s); err == nil {
				return DateFromDays(tm.Unix() / 86400), true
			}
		}
	case KindDateTime:
		for _, layout := range temporalTimestampLayouts() {
			if tm, err := time.Parse(layout, s); err == nil {
				return DateTimeFromUnixNanos(tm.UnixNano()), true
			}
		}
	case KindTimespan:
		if nanos, ok := parseTimespanNanos(s); ok {
			return TimespanFromNanos(nanos), true
		}
	case KindMinute:
		if nanos, ok := parseTimeOfDayNanos(s); ok && nanos%(60*1_000_000_000) == 0 {
			return MinuteFromMinutes(nanos / (60 * 1_000_000_000)), true
		}
	case KindSecond:
		if nanos, ok := parseTimeOfDayNanos(s); ok && nanos%1_000_000_000 == 0 {
			return SecondFromSeconds(nanos / 1_000_000_000), true
		}
	case KindTime:
		if nanos, ok := parseTimeOfDayNanos(s); ok {
			return TimeFromNanos(nanos), true
		}
	case KindTimestamp:
		for _, layout := range temporalTimestampLayouts() {
			if tm, err := time.Parse(layout, s); err == nil {
				return TimestampFromUnixNanos(tm.UnixNano()), true
			}
		}
	}
	return nil, false
}

func temporalTimestampLayouts() []string {
	return []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006.01.02D15:04:05.999999999",
		"2006.01.02D15:04:05",
		"2006.01.02T15:04:05.999999999",
		"2006.01.02T15:04:05",
	}
}

func parseTimeOfDayNanos(s string) (int64, bool) {
	for _, layout := range []string{"15:04", "15:04:05", "15:04:05.999", "15:04:05.999999", "15:04:05.999999999"} {
		if tm, err := time.Parse(layout, s); err == nil {
			nanos := int64(tm.Hour())*3600*1_000_000_000 + int64(tm.Minute())*60*1_000_000_000 + int64(tm.Second())*1_000_000_000 + int64(tm.Nanosecond())
			return nanos, true
		}
	}
	return 0, false
}

func parseTimespanNanos(s string) (int64, bool) {
	sign := int64(1)
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = strings.TrimPrefix(s, "-")
	} else {
		s = strings.TrimPrefix(s, "+")
	}
	days := int64(0)
	if parts := strings.SplitN(s, "D", 2); len(parts) == 2 {
		n, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, false
		}
		days = n
		s = parts[1]
	}
	nanos, ok := parseTimeOfDayNanos(s)
	if !ok {
		return 0, false
	}
	return sign * (days*nanosPerDay + nanos), true
}

func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

func integerValue(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	default:
		return 0, false
	}
}

func kindOfScalar(v any) Kind {
	switch v.(type) {
	case bool:
		return KindBool
	case int8:
		return KindI8
	case int16:
		return KindI16
	case int32:
		return KindI32
	case int, int64:
		return KindI64
	case uint8:
		return KindU8
	case uint16:
		return KindU16
	case uint32:
		return KindU32
	case uint64:
		return KindU64
	case float32:
		return KindF32
	case float64:
		return KindF64
	case string:
		return KindString
	case Symbol:
		return KindSymbol
	case Month:
		return KindMonth
	case Date:
		return KindDate
	case DateTime:
		return KindDateTime
	case Timespan:
		return KindTimespan
	case Minute:
		return KindMinute
	case Second:
		return KindSecond
	case Time:
		return KindTime
	case Timestamp:
		return KindTimestamp
	default:
		return ""
	}
}

func equalScalar(left, right any) bool {
	if IsNull(left) || IsNull(right) {
		return IsNull(left) && IsNull(right)
	}
	switch left.(type) {
	case Symbol:
		_, ok := right.(Symbol)
		if !ok {
			return false
		}
	case string:
		_, ok := right.(string)
		if !ok {
			return false
		}
	}
	switch right.(type) {
	case Symbol:
		_, ok := left.(Symbol)
		if !ok {
			return false
		}
	case string:
		_, ok := left.(string)
		if !ok {
			return false
		}
	}
	if cmp, ok := compareSameKind(left, right); ok {
		return cmp == 0
	}
	// Go == panics on uncomparable operands (dicts, callables, slices that
	// leak in through []any values); fall back to a deep comparison so the
	// kernels return a value instead of crashing. The q layer intercepts
	// dict/callable equality with canonical semantics before reaching here.
	if !comparableScalarType(left) || !comparableScalarType(right) {
		return reflect.DeepEqual(left, right)
	}
	return left == right
}

func comparableScalarType(v any) bool {
	t := reflect.TypeOf(v)
	return t == nil || t.Comparable()
}

func compareSameKind(left, right any) (int, bool) {
	switch l := left.(type) {
	case bool:
		r, ok := right.(bool)
		if !ok {
			return 0, false
		}
		switch {
		case l == r:
			return 0, true
		case !l && r:
			return -1, true
		default:
			return 1, true
		}
	case int8:
		r, ok := right.(int8)
		return compareInt64(int64(l), int64(r)), ok
	case int16:
		r, ok := right.(int16)
		return compareInt64(int64(l), int64(r)), ok
	case int32:
		r, ok := right.(int32)
		return compareInt64(int64(l), int64(r)), ok
	case int64:
		r, ok := right.(int64)
		return compareInt64(l, r), ok
	case uint8:
		r, ok := right.(uint8)
		return compareUint64(uint64(l), uint64(r)), ok
	case uint16:
		r, ok := right.(uint16)
		return compareUint64(uint64(l), uint64(r)), ok
	case uint32:
		r, ok := right.(uint32)
		return compareUint64(uint64(l), uint64(r)), ok
	case uint64:
		r, ok := right.(uint64)
		return compareUint64(l, r), ok
	case float32:
		r, ok := right.(float32)
		return compareFloat64(float64(l), float64(r)), ok
	case float64:
		r, ok := right.(float64)
		return compareFloat64(l, r), ok
	case Month:
		r, ok := right.(Month)
		return compareInt64(int64(l), int64(r)), ok
	case Date:
		r, ok := right.(Date)
		return compareInt64(int64(l), int64(r)), ok
	case DateTime:
		r, ok := right.(DateTime)
		return compareInt64(int64(l), int64(r)), ok
	case Timespan:
		r, ok := right.(Timespan)
		return compareInt64(int64(l), int64(r)), ok
	case Minute:
		r, ok := right.(Minute)
		return compareInt64(int64(l), int64(r)), ok
	case Second:
		r, ok := right.(Second)
		return compareInt64(int64(l), int64(r)), ok
	case Time:
		r, ok := right.(Time)
		return compareInt64(int64(l), int64(r)), ok
	case Timestamp:
		r, ok := right.(Timestamp)
		return compareInt64(int64(l), int64(r)), ok
	case Symbol:
		r, ok := right.(Symbol)
		return compareString(string(l), string(r)), ok
	case string:
		r, ok := right.(string)
		return compareString(l, r), ok
	}
	return 0, false
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareUint64(left, right uint64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareFloat64(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareString(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareBool(left, right bool) int {
	switch {
	case left == right:
		return 0
	case !left && right:
		return -1
	default:
		return 1
	}
}

// nullOrderedCompare reports the canonical q result of a comparison when at
// least one operand is null: null sorts before every value and equals itself
// (0N<1 -> 1b, 1<=0N -> 0b, 0N>=0N -> 1b, 0N=0N -> 1b).
func nullOrderedCompare(op Op, leftNull, rightNull bool) bool {
	switch op {
	case OpEQ:
		return leftNull && rightNull
	case OpNE:
		return leftNull != rightNull
	case OpLT:
		return leftNull && !rightNull
	case OpGT:
		return !leftNull && rightNull
	case OpLE:
		return leftNull
	case OpGE:
		return rightNull
	default:
		return false
	}
}

func boolCompare(op Op, equal bool, cmp int) bool {
	switch op {
	case OpEQ:
		return equal
	case OpNE:
		return !equal
	case OpLT:
		return cmp < 0
	case OpLE:
		return cmp <= 0
	case OpGT:
		return cmp > 0
	case OpGE:
		return cmp >= 0
	default:
		return false
	}
}

func compare(left, right any) int {
	if IsNull(left) || IsNull(right) {
		switch {
		case IsNull(left) && IsNull(right):
			return 0
		case IsNull(left):
			return -1
		default:
			return 1
		}
	}
	if cmp, ok := compareSameKind(left, right); ok {
		return cmp
	}
	if lf, ok := numeric(left); ok {
		if rf, ok := numeric(right); ok {
			switch {
			case lf < rf:
				return -1
			case lf > rf:
				return 1
			default:
				return 0
			}
		}
	}
	ls, rs := fmt.Sprint(left), fmt.Sprint(right)
	switch {
	case ls < rs:
		return -1
	case ls > rs:
		return 1
	default:
		return 0
	}
}

func allIndexes(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func contiguousIndexes(start, count int) []int {
	out := make([]int, count)
	for i := range out {
		out[i] = start + i
	}
	return out
}

func validateIndexes(length int, indexes []int) error {
	for i, row := range indexes {
		if row < 0 || row >= length {
			return fmt.Errorf("index %d has row %d out of range for length %d", i, row, length)
		}
	}
	return nil
}

func takeIndexes(length, n int) ([]int, error) {
	n, err := takeCount(length, n)
	if err != nil {
		return nil, err
	}
	return allIndexes(n), nil
}

func takeCount(length, n int) (int, error) {
	if n < 0 {
		return 0, fmt.Errorf("take count %d must not be negative", n)
	}
	if n > length {
		n = length
	}
	return n, nil
}

func validateKeyColumns(frame Frame, keys []Symbol, context string) ([]Symbol, error) {
	out := make([]Symbol, len(keys))
	seen := make(map[Symbol]struct{}, len(keys))
	for i, key := range keys {
		if key == "" {
			return nil, fmt.Errorf("%s key %d must not be empty", context, i+1)
		}
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("%s key column %q is duplicated", context, key)
		}
		seen[key] = struct{}{}
		if _, ok := frame.Column(key); !ok {
			return nil, fmt.Errorf("%s key column %q does not exist", context, key)
		}
		out[i] = key
	}
	return out, nil
}

func validateMutationKeys(keyed KeyedFrame, delta Frame) error {
	for _, key := range keyed.keys {
		if _, ok := delta.Column(key); !ok {
			return fmt.Errorf("keyed mutation delta key column %q does not exist", key)
		}
	}
	return nil
}

func keyedMutationValueColumns(keyed KeyedFrame, delta Frame, requested []Symbol) ([]Symbol, error) {
	keySet := make(map[Symbol]struct{}, len(keyed.keys))
	for _, key := range keyed.keys {
		keySet[key] = struct{}{}
	}
	if len(requested) == 0 {
		out := make([]Symbol, 0, len(delta.schema.names))
		for _, name := range delta.schema.names {
			if _, ok := keySet[name]; ok {
				continue
			}
			out = append(out, name)
		}
		return out, nil
	}
	out := make([]Symbol, len(requested))
	seen := make(map[Symbol]struct{}, len(requested))
	for i, name := range requested {
		if name == "" {
			return nil, fmt.Errorf("keyed mutation value column %d must not be empty", i+1)
		}
		if _, ok := keySet[name]; ok {
			return nil, fmt.Errorf("keyed mutation cannot assign key column %q", name)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("keyed mutation value column %q is duplicated", name)
		}
		if _, ok := delta.Column(name); !ok {
			return nil, fmt.Errorf("keyed mutation value column %q does not exist", name)
		}
		seen[name] = struct{}{}
		out[i] = name
	}
	return out, nil
}

func keyedMutationColumns(keyed KeyedFrame, delta Frame, valueColumns []Symbol) ([]Column, map[Symbol][]any, error) {
	cols := keyed.frame.Columns()
	values := make(map[Symbol][]any, len(cols)+len(valueColumns))
	for _, col := range cols {
		values[col.Name] = col.Data.Values()
	}
	seen := make(map[Symbol]struct{}, len(cols)+len(valueColumns))
	for _, col := range cols {
		seen[col.Name] = struct{}{}
	}
	for _, name := range valueColumns {
		if _, ok := seen[name]; ok {
			continue
		}
		col, ok := delta.Column(name)
		if !ok {
			return nil, nil, fmt.Errorf("keyed mutation value column %q does not exist", name)
		}
		initial := make([]any, keyed.frame.Len())
		for i := range initial {
			initial[i] = NullValue
		}
		cols = append(cols, Column{Name: name, Data: col.Gather(nil)})
		values[name] = initial
		seen[name] = struct{}{}
	}
	return cols, values, nil
}

func rowMutationDeltaFrame(frame Frame, columns []Symbol, values []any) (Frame, error) {
	names, row, err := rowMutationRecord(frame, columns, values)
	if err != nil {
		return Frame{}, err
	}
	cols := make([]Column, 0, len(names))
	for _, name := range names {
		col, ok := frame.Column(name)
		if !ok {
			return Frame{}, fmt.Errorf("insert column %q does not exist", name)
		}
		out, err := columnWithKind(name, col.Kind(), []any{row[name]})
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, out)
	}
	return NewFrame(cols...)
}

func rowMutationRecord(frame Frame, columns []Symbol, values []any) ([]Symbol, map[Symbol]any, error) {
	if frame.Len() < 0 {
		return nil, nil, fmt.Errorf("frame is not initialized")
	}
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("insert requires at least one value")
	}
	names := frame.schema.names
	if len(columns) == 0 {
		if len(values) != len(names) {
			return nil, nil, fmt.Errorf("insert values count %d does not match table column count %d", len(values), len(names))
		}
		columns = names
	} else if len(columns) != len(values) {
		return nil, nil, fmt.Errorf("insert column count %d does not match values count %d", len(columns), len(values))
	}
	row := make(map[Symbol]any, len(names))
	for _, name := range names {
		row[name] = NullValue
	}
	seen := make(map[Symbol]struct{}, len(columns))
	for i, name := range columns {
		if name == "" {
			return nil, nil, fmt.Errorf("insert column name must not be empty")
		}
		if _, ok := frame.Column(name); !ok {
			return nil, nil, fmt.Errorf("insert column %q does not exist", name)
		}
		if _, ok := seen[name]; ok {
			return nil, nil, fmt.Errorf("insert column %q is duplicated", name)
		}
		seen[name] = struct{}{}
		row[name] = values[i]
	}
	return names, row, nil
}

func rowMutationValueColumns(keys, names, columns []Symbol) ([]Symbol, error) {
	keySet := make(map[Symbol]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	if len(columns) == 0 {
		columns = names
	}
	out := make([]Symbol, 0, len(columns))
	seen := make(map[Symbol]struct{}, len(columns))
	for _, name := range columns {
		if _, ok := keySet[name]; ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

func rowMutationRequireKeyColumns(keys, columns []Symbol) error {
	if len(columns) == 0 {
		return nil
	}
	present := make(map[Symbol]struct{}, len(columns))
	for _, name := range columns {
		present[name] = struct{}{}
	}
	for _, key := range keys {
		if _, ok := present[key]; !ok {
			return fmt.Errorf("keyed mutation delta key column %q does not exist", key)
		}
	}
	return nil
}

func validateJoinKeys(left, right Frame, keys []JoinKey) error {
	seen := make(map[JoinKey]struct{}, len(keys))
	for i, key := range keys {
		if key.Left == "" || key.Right == "" {
			return fmt.Errorf("inner join key %d must not be empty", i+1)
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("inner join key %q=%q is duplicated", key.Left, key.Right)
		}
		seen[key] = struct{}{}
		if _, ok := left.Column(key.Left); !ok {
			return fmt.Errorf("inner join left key column %q does not exist", key.Left)
		}
		if _, ok := right.Column(key.Right); !ok {
			return fmt.Errorf("inner join right key column %q does not exist", key.Right)
		}
	}
	return nil
}

func leftKeyColumns(keys []JoinKey) []Symbol {
	cols := make([]Symbol, len(keys))
	for i, key := range keys {
		cols[i] = key.Left
	}
	return cols
}

func rightRowsByJoinKey(right Frame, keys []JoinKey) (map[string][]int, []Symbol, error) {
	rightKeyCols := make([]Symbol, len(keys))
	for i, key := range keys {
		rightKeyCols[i] = key.Right
	}
	if len(rightKeyCols) == 1 {
		name := rightKeyCols[0]
		if col, ok := right.Column(name); ok && !ArrayHasAttribute(col, ArrayAttributeUnique) && !ArrayHasAttribute(col, ArrayAttributeGrouped) {
			right.columns[name] = WithArrayAttribute(col, ArrayAttributeGrouped)
		}
	}
	rowsByKey, err := typedKernels.RowsByKey(right, rightKeyCols)
	if err != nil {
		return nil, nil, err
	}
	return rowsByKey, rightKeyCols, nil
}

func joinRightKeyKinds(right Frame, keys []JoinKey) []Kind {
	kinds := make([]Kind, len(keys))
	for i, key := range keys {
		if col, ok := right.Column(key.Right); ok {
			kinds[i] = col.Kind()
		}
	}
	return kinds
}

func joinLookupKey(left, right Frame, row int, keys []JoinKey) (string, error) {
	var b strings.Builder
	for _, key := range keys {
		leftCol, ok := left.Column(key.Left)
		if !ok {
			return "", fmt.Errorf("join left key column %q does not exist", key.Left)
		}
		rightCol, ok := right.Column(key.Right)
		if !ok {
			return "", fmt.Errorf("join right key column %q does not exist", key.Right)
		}
		v, ok := leftCol.At(row)
		if !ok {
			return "", fmt.Errorf("join left key column %q row %d out of range", key.Left, row)
		}
		normalized, err := normalizeKeyValue(rightCol.Kind(), v)
		if err != nil {
			return "\xffjoin-miss", nil
		}
		appendKeyPart(&b, rightCol.Kind(), normalized)
	}
	return b.String(), nil
}

func validateAsofJoinKeys(left, right Frame, timeKey JoinKey, partitionKeys []JoinKey) error {
	if timeKey.Left == "" || timeKey.Right == "" {
		return fmt.Errorf("asof join time key must not be empty")
	}
	leftTime, ok := left.Column(timeKey.Left)
	if !ok {
		return fmt.Errorf("asof join left time key column %q does not exist", timeKey.Left)
	}
	rightTime, ok := right.Column(timeKey.Right)
	if !ok {
		return fmt.Errorf("asof join right time key column %q does not exist", timeKey.Right)
	}
	if leftTime.Kind() != rightTime.Kind() {
		return fmt.Errorf("asof join time key kinds differ: %q is %s and %q is %s", timeKey.Left, leftTime.Kind(), timeKey.Right, rightTime.Kind())
	}
	if !isAsofTimeKind(leftTime.Kind()) {
		return fmt.Errorf("asof join time key %q=%q has non-time kind %s", timeKey.Left, timeKey.Right, leftTime.Kind())
	}
	if err := validateJoinKeys(left, right, partitionKeys); err != nil {
		return err
	}
	for _, key := range partitionKeys {
		if key == timeKey {
			return fmt.Errorf("asof join time key %q=%q is also a partition key", timeKey.Left, timeKey.Right)
		}
	}
	return nil
}

func isAsofTimeKind(kind Kind) bool {
	switch kind {
	case KindI8, KindI16, KindI32, KindI64, KindU8, KindU16, KindU32, KindU64, KindF32, KindF64,
		KindMonth, KindDate, KindDateTime, KindTimespan, KindMinute, KindSecond, KindTime, KindTimestamp:
		return true
	default:
		return false
	}
}

func gatherOptional(array Array, indexes []int) Array {
	return typedKernels.GatherOptional(array, indexes)
}

func joinGather(array Array, indexes []int) Array {
	if indexArray, ok := i64RangeIndexArrayFromInts(indexes); ok {
		return joinGatherByIndexArray(array, indexArray)
	}
	return array.Gather(indexes)
}

func joinGatherByIndexArray(array Array, indexArray Array) Array {
	if out, handled, err := TryGatherByI64IndexArray(array, indexArray); err == nil && handled {
		return out
	}
	indexes, handled, err := TryTypedI64Indexes(indexArray)
	if err == nil && handled {
		return array.Gather(indexes)
	}
	values := indexArray.Values()
	indexes = make([]int, len(values))
	for i, value := range values {
		indexes[i] = int(value.(int64))
	}
	return array.Gather(indexes)
}

func joinGatherOptional(array Array, indexes []int) Array {
	if allIndexesPresent(indexes) {
		return joinGather(array, indexes)
	}
	return typedKernels.GatherOptional(array, indexes)
}

func allIndexesPresent(indexes []int) bool {
	for _, row := range indexes {
		if row < 0 {
			return false
		}
	}
	return true
}

func i64RangeIndexArrayFromInts(indexes []int) (Array, bool) {
	switch len(indexes) {
	case 0:
		return i64RangeArray{len: 0}, true
	case 1:
		return i64RangeArray{start: int64(indexes[0]), step: 1, len: 1}, true
	}
	first := indexes[0]
	step := indexes[1] - indexes[0]
	for i := 2; i < len(indexes); i++ {
		if indexes[i]-indexes[i-1] != step {
			return nil, false
		}
	}
	return i64RangeArray{start: int64(first), step: int64(step), len: len(indexes)}, true
}

func takeArray(array Array, n int) Array {
	switch a := array.(type) {
	case columnArray[bool]:
		return takeColumnArray(a, n)
	case columnArray[int8]:
		return takeColumnArray(a, n)
	case columnArray[int16]:
		return takeColumnArray(a, n)
	case columnArray[int32]:
		return takeColumnArray(a, n)
	case columnArray[int64]:
		return takeColumnArray(a, n)
	case columnArray[uint8]:
		return takeColumnArray(a, n)
	case columnArray[uint16]:
		return takeColumnArray(a, n)
	case columnArray[uint32]:
		return takeColumnArray(a, n)
	case columnArray[uint64]:
		return takeColumnArray(a, n)
	case columnArray[float32]:
		return takeColumnArray(a, n)
	case columnArray[float64]:
		return takeColumnArray(a, n)
	case columnArray[string]:
		return takeColumnArray(a, n)
	case columnArray[Symbol]:
		return takeColumnArray(a, n)
	case columnArray[Month]:
		return takeColumnArray(a, n)
	case columnArray[Date]:
		return takeColumnArray(a, n)
	case columnArray[DateTime]:
		return takeColumnArray(a, n)
	case columnArray[Timespan]:
		return takeColumnArray(a, n)
	case columnArray[Minute]:
		return takeColumnArray(a, n)
	case columnArray[Second]:
		return takeColumnArray(a, n)
	case columnArray[Time]:
		return takeColumnArray(a, n)
	case columnArray[Timestamp]:
		return takeColumnArray(a, n)
	case nullableArray:
		return nullableArray{kind: a.kind, data: append([]any(nil), a.data[:n]...)}
	case nullBitmapCarrier:
		return a.takePrefix(n)
	default:
		return array.Gather(allIndexes(n))
	}
}

func takeColumnArray[T any](array columnArray[T], n int) Array {
	return columnArray[T]{kind: array.kind, data: append([]T(nil), array.data[:n]...)}
}

func gatherWindowLists(array Array, indexes [][]int) Array {
	return typedKernels.GatherWindowLists(array, indexes)
}

func gatherLastOptional(array Array, indexes [][]int) Array {
	return typedKernels.GatherLastOptional(array, indexes)
}

func rightJoinColumnName(name Symbol, used map[Symbol]struct{}) Symbol {
	if _, ok := used[name]; !ok {
		return name
	}
	base := Symbol(string(name) + "_right")
	if _, ok := used[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		candidate := Symbol(fmt.Sprintf("%s%d", base, i))
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func rowKey(frame Frame, row int, columns []Symbol) (string, error) {
	var b strings.Builder
	for _, name := range columns {
		col, ok := frame.Column(name)
		if !ok {
			return "", fmt.Errorf("key column %q does not exist", name)
		}
		v, ok := col.At(row)
		if !ok {
			return "", fmt.Errorf("key column %q row %d out of range", name, row)
		}
		appendKeyPart(&b, col.Kind(), v)
	}
	return b.String(), nil
}

func sameSymbols(left, right []Symbol) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

type stringHash interface {
	Write([]byte) (int, error)
}

func writeHashString(h stringHash, value string) {
	_, _ = h.Write([]byte(value))
	_, _ = h.Write([]byte{0})
}

func deltaRowKey(keyed KeyedFrame, delta Frame, row int) (string, []any, error) {
	var b strings.Builder
	values := make([]any, len(keyed.keys))
	for i, name := range keyed.keys {
		targetCol, ok := keyed.frame.Column(name)
		if !ok {
			return "", nil, fmt.Errorf("key column %q does not exist", name)
		}
		deltaCol, ok := delta.Column(name)
		if !ok {
			return "", nil, fmt.Errorf("delta key column %q does not exist", name)
		}
		v, ok := deltaCol.At(row)
		if !ok {
			return "", nil, fmt.Errorf("delta key column %q row %d out of range", name, row)
		}
		normalized, err := normalizeKeyValue(targetCol.Kind(), v)
		if err != nil {
			return "", nil, fmt.Errorf("delta key column %q: %w", name, err)
		}
		values[i] = normalized
		appendKeyPart(&b, targetCol.Kind(), normalized)
	}
	return b.String(), values, nil
}

func lookupKey(frame Frame, columns []Symbol, values []any) (string, error) {
	if len(values) != len(columns) {
		return "", fmt.Errorf("lookup key has %d values, want %d", len(values), len(columns))
	}
	var b strings.Builder
	for i, name := range columns {
		col, ok := frame.Column(name)
		if !ok {
			return "", fmt.Errorf("key column %q does not exist", name)
		}
		v, err := normalizeKeyValue(col.Kind(), values[i])
		if err != nil {
			return "", fmt.Errorf("lookup key column %q: %w", name, err)
		}
		appendKeyPart(&b, col.Kind(), v)
	}
	return b.String(), nil
}

func hasColumn(frame Frame, name Symbol) bool {
	_, ok := frame.Column(name)
	return ok
}

func hasSymbol(symbols map[Symbol]struct{}, name Symbol) bool {
	_, ok := symbols[name]
	return ok
}

func symbolIndex(names []Symbol, target Symbol) int {
	for i, name := range names {
		if name == target {
			return i
		}
	}
	return -1
}

func normalizeKeyValue(kind Kind, value any) (any, error) {
	if normalized, ok := coerceTemporalLookupValue(kind, value); ok {
		return normalized, nil
	}
	return NormalizeValueForKind(kind, value)
}

func coerceTemporalLookupValue(kind Kind, value any) (any, bool) {
	if normalized, ok := coerceTemporalValue(kind, value); ok {
		return normalized, true
	}
	if n, ok := coerceInt64Exact(value); ok {
		switch kind {
		case KindMonth:
			return MonthFromMonths(n), true
		case KindDate:
			return DateFromDays(n), true
		case KindDateTime:
			return DateTimeFromUnixNanos(n), true
		case KindTimespan:
			return TimespanFromNanos(n), true
		case KindMinute:
			return MinuteFromMinutes(n), true
		case KindSecond:
			return SecondFromSeconds(n), true
		case KindTime:
			return TimeFromNanos(n), true
		case KindTimestamp:
			return TimestampFromUnixNanos(n), true
		}
	}
	return nil, false
}

func appendKeyPart(b *strings.Builder, kind Kind, v any) {
	if IsNull(v) {
		b.WriteString(string(kind))
		b.WriteString(":null\x00")
		return
	}
	b.WriteString(string(kind))
	b.WriteByte(':')
	switch x := v.(type) {
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int8:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int16:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case uint8:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint16:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(x, 10))
	case float32:
		b.WriteString(strconv.FormatFloat(float64(x), 'g', -1, 32))
	case float64:
		b.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	case string:
		b.WriteString(strconv.Quote(x))
	case Symbol:
		b.WriteString(strconv.Quote(string(x)))
	case Month:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case Date:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case DateTime:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case Timespan:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case Minute:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case Second:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case Time:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case Timestamp:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	default:
		fmt.Fprintf(b, "%T:%#v", v, v)
	}
	b.WriteByte(0)
}
