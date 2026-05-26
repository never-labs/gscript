// table_construct.go — Table constructor variants and small-table constructor
// shape caches.
//
// Pure code movement from table.go: the NewXTable factory family, the
// freshly-constructed-slot Init* helpers, and the SmallTableCtor2/N
// constructor-shape caches that build fixed-shape literal tables in one pass.

package runtime

import "sync"

// NewTable creates a new empty table (non-concurrent by default).
func NewTable() *Table {
	return NewEmptyTable()
}

// NewEmptyTable creates an empty table with a clean iteration-key cache.
// Mutating operations mark keysDirty before adding or removing entries, so a
// fresh table does not need an initial rebuild for pairs/Next semantics.
func NewEmptyTable() *Table {
	t := DefaultHeap.AllocTable()
	return t
}

// NewTableSized creates a table with pre-allocated capacity hints.
func NewTableSized(arrayHint, hashHint int) *Table {
	if arrayHint == 0 && hashHint == 0 {
		return NewEmptyTable()
	}
	return NewTableSizedKind(arrayHint, hashHint, ArrayMixed)
}

// NewTableSizedKind creates a table with pre-allocated capacity hints and, for
// scalar array builders, an optional typed-array backing. The mixed layout keeps
// the historical length-1 sentinel allocation; typed arrays start at length 0 so
// key 0 can use the native append path.
func NewTableSizedKind(arrayHint, hashHint int, kind ArrayKind) *Table {
	if arrayHint == 0 && hashHint == 0 {
		return NewEmptyTable()
	}
	if arrayHint == 0 && hashHint > 0 && hashHint <= smallFieldCap && kind == ArrayMixed {
		t, svals := DefaultHeap.AllocTableWithSvals(hashHint)
		t.svals = svals
		return t
	}
	t := DefaultHeap.AllocTable()
	if arrayHint > 0 {
		capHint := arrayHint + 1
		switch kind {
		case ArrayInt:
			t.arrayKind = ArrayInt
			t.intArray = DefaultHeap.AllocInt64s(0, capHint)
		case ArrayFloat:
			t.arrayKind = ArrayFloat
			t.floatArray = DefaultHeap.AllocFloat64s(0, capHint)
		case ArrayBool:
			t.arrayKind = ArrayBool
			t.boolArray = DefaultHeap.AllocByteSlice(0, capHint)
		default:
			if capHint <= sparseArrayMax+1 {
				t.array = DefaultHeap.AllocValues(1, capHint)
			} else {
				t.array = DefaultHeap.AllocValues(1, sparseArrayMax+1)
				t.arrayHint = capHint
			}
			t.array[0] = NilValue()
		}
	}
	if hashHint > 0 && hashHint <= smallFieldCap {
		t.svals = DefaultHeap.AllocValues(0, hashHint)
	}
	return t
}

// NewDenseMixedArrayTable creates a mixed-value table with a full dense array
// capacity. This is intentionally separate from NewTableSizedKind: ordinary
// large mixed tables stay lazy to reduce GC scan cost, while JIT-proven dense
// builders can opt into the larger backing to keep integer-key stores native.
func NewDenseMixedArrayTable(arrayHint, hashHint int) *Table {
	if arrayHint <= 0 {
		return NewTableSizedKind(arrayHint, hashHint, ArrayMixed)
	}
	t := DefaultHeap.AllocTable()
	t.array = DefaultHeap.AllocValues(1, arrayHint+1)
	t.array[0] = NilValue()
	if hashHint > 0 && hashHint <= smallFieldCap {
		t.svals = DefaultHeap.AllocValues(0, hashHint)
	}
	return t
}

// NewPlainIntArrayMapTable creates a plain table for runtime-proven builders
// that fill every positive integer slot 1..arrayLen and also need pre-sized
// integer/string map parts. The table is ordinary after construction; callers
// must initialize the advertised array slots before exposing it to user code.
func NewPlainIntArrayMapTable(arrayLen, intMapHint, stringMapHint int) *Table {
	if arrayLen < 0 {
		arrayLen = 0
	}
	t := DefaultHeap.AllocTable()
	if arrayLen > 0 {
		t.arrayKind = ArrayInt
		t.intArray = DefaultHeap.AllocInt64s(arrayLen+1, arrayLen+1)
	}
	if intMapHint > 0 {
		t.imap = make(map[int64]Value, intMapHint)
	}
	if stringMapHint > 0 {
		t.smap = make(map[string]Value, stringMapHint)
	}
	t.keysDirty = true
	return t
}

// InitIntArraySlot initializes a slot in a freshly constructed ArrayInt table.
// It returns false if the table no longer has the required storage shape.
func (t *Table) InitIntArraySlot(index int64, value int64) bool {
	if t == nil || t.arrayKind != ArrayInt || index < 0 || index >= int64(len(t.intArray)) {
		return false
	}
	if index == 0 {
		t.arrayZeroValid = true
	}
	t.intArray[index] = value
	return true
}

// InitIntMapSlot initializes a non-array integer key in a freshly constructed
// table. It uses Value storage because integer-map entries may hold any type.
func (t *Table) InitIntMapSlot(key int64, val Value) bool {
	if t == nil || val.IsNil() {
		return false
	}
	if t.imap == nil {
		t.imap = make(map[int64]Value)
	}
	t.imap[key] = val
	return true
}

// InitStringMapSlot initializes a string-map key in a freshly constructed table.
func (t *Table) InitStringMapSlot(key string, val Value) bool {
	if t == nil || val.IsNil() {
		return false
	}
	if t.smap == nil {
		t.smap = make(map[string]Value)
	}
	t.smap[key] = val
	return true
}

// NewSequentialArrayTable creates a table whose 1-based array part has exactly
// length slots ready for direct sequential fill by runtime builders.
func NewSequentialArrayTable(length int) *Table {
	if length <= 0 {
		return NewEmptyTable()
	}
	if length+1 <= tableSvalsNInlineCap {
		t, values := DefaultHeap.AllocTableWithSvals(length + 1)
		t.array = values[: length+1 : length+1]
		return t
	}
	t := DefaultHeap.AllocTable()
	t.array = DefaultHeap.AllocValues(1, length+1)[: length+1 : length+1]
	return t
}

func NewAppendArrayTable(capacity int) *Table {
	if capacity < 1 {
		capacity = 1
	}
	if capacity <= tableSvalsNInlineCap {
		t, values := DefaultHeap.AllocTableWithSvals(capacity)
		t.array = values[:1:capacity]
		t.array[0] = NilValue()
		return t
	}
	t := DefaultHeap.AllocTable()
	t.array = DefaultHeap.AllocValues(1, capacity)
	return t
}

// SmallTableCtor2 caches the final shape for a static two-string-field table
// constructor. It is stored on bytecode prototypes, not on Table, so the common
// object-literal allocation path can skip per-instance shape transitions
// without growing every table.
type SmallTableCtor2 struct {
	Key1 string
	Key2 string

	Shape *Shape

	shapeID   uint32
	fieldKeys []string
	single1   smallCtorShape
	single2   smallCtorShape
}

// SmallTableCtorN caches the final shape for static small string-field table
// constructors with more than two fields. It is the generic counterpart to
// SmallTableCtor2; nil runtime values still take the sequential RawSetString
// fallback so table literal omission semantics are preserved.
type SmallTableCtorN struct {
	Keys []string

	Shape *Shape

	shapeID   uint32
	fieldKeys []string
}

type sparseCtorNKey struct {
	shapeID uint32
	mask    uint64
}

var sparseCtorNCache sync.Map // map[sparseCtorNKey]SmallTableCtorN

type smallCtorShape struct {
	shape     *Shape
	shapeID   uint32
	fieldKeys []string
}

func newSmallCtorShape(shape *Shape) smallCtorShape {
	if shape == nil {
		return smallCtorShape{}
	}
	return smallCtorShape{
		shape:     shape,
		shapeID:   shape.ID,
		fieldKeys: shape.FieldKeys,
	}
}

func NewSmallTableCtor2(key1, key2 string) SmallTableCtor2 {
	ctor := SmallTableCtor2{Key1: key1, Key2: key2}
	ctor.single1 = newSmallCtorShape(getOrCreateSingleFieldShape(key1))
	ctor.single2 = newSmallCtorShape(getOrCreateSingleFieldShape(key2))
	if key1 != key2 {
		ctor.Shape = GetShape([]string{key1, key2})
		ctor.shapeID = ctor.Shape.ID
		ctor.fieldKeys = ctor.Shape.FieldKeys
	}
	return ctor
}

func NewSmallTableCtorN(keys []string) SmallTableCtorN {
	owned := append([]string(nil), keys...)
	ctor := SmallTableCtorN{Keys: owned}
	if len(owned) == 0 {
		return ctor
	}
	seen := make(map[string]struct{}, len(owned))
	for _, key := range owned {
		if _, ok := seen[key]; ok {
			return ctor
		}
		seen[key] = struct{}{}
	}
	ctor.Shape = GetShape(owned)
	if ctor.Shape != nil {
		ctor.shapeID = ctor.Shape.ID
		ctor.fieldKeys = ctor.Shape.FieldKeys
	}
	return ctor
}

// NewTableFromCtor2 constructs a small two-field string table in one pass.
// Runtime nil values omit their fields just like sequential SETFIELD bytecode.
func NewTableFromCtor2(ctor *SmallTableCtor2, val1, val2 Value) *Table {
	if ctor != nil {
		shape := ctor.Shape
		if shape != nil && !val1.IsNil() && !val2.IsNil() {
			return newTableFromCtor2Shape(ctor, shape, val1, val2)
		}
	}
	return newTableFromCtor2Fallback(ctor, val1, val2)
}

// NewTableFromCtor2NonNil constructs a cacheable two-field string table when
// the caller has already proven both values are non-nil. It is equivalent to
// NewTableFromCtor2 for valid cacheable constructors and non-nil values, but
// avoids the generic nil/duplicate-key fallback checks in native constructor
// protocols.
func NewTableFromCtor2NonNil(ctor *SmallTableCtor2, val1, val2 Value) *Table {
	if ctor == nil || ctor.Shape == nil || val1.IsNil() || val2.IsNil() {
		return NewTableFromCtor2(ctor, val1, val2)
	}
	return newTableFromCtor2Shape(ctor, ctor.Shape, val1, val2)
}

func newTableFromCtor2Shape(ctor *SmallTableCtor2, shape *Shape, val1, val2 Value) *Table {
	t, svals := DefaultHeap.AllocTableWithSvals2()
	t.svals = svals
	t.svals[0] = val1
	t.svals[1] = val2
	t.shape = shape
	t.shapeID = ctor.shapeID
	t.skeys = ctor.fieldKeys
	ObserveShapeFieldValueOnShape(shape, 0, val1)
	ObserveShapeFieldValueOnShape(shape, 1, val2)
	return t
}

// NewTableFromCtorN constructs a fixed-shape small string table in one pass
// when all runtime values are non-nil. If any value is nil, it falls back to
// sequential RawSetString so omitted fields and duplicate-key behavior match
// ordinary table literal execution.
func NewTableFromCtorN(ctor *SmallTableCtorN, vals []Value) *Table {
	if ctor != nil && ctor.Shape != nil && len(vals) >= len(ctor.Keys) {
		n := len(ctor.Keys)
		t, svals := DefaultHeap.AllocTableWithSvals(n)
		t.svals = svals[:n]
		for i := 0; i < n; i++ {
			v := vals[i]
			if v.IsNil() {
				return newTableFromCtorNFallback(ctor, vals)
			}
			t.svals[i] = v
		}
		t.shape = ctor.Shape
		t.shapeID = ctor.shapeID
		t.skeys = ctor.fieldKeys
		for i := 0; i < n; i++ {
			ObserveShapeFieldValueOnShape(ctor.Shape, i, t.svals[i])
		}
		return t
	}
	return newTableFromCtorNFallback(ctor, vals)
}

// NewTableFromCtorNNonNil constructs a fixed-shape small string table when the
// caller has already proven every constructor value is non-nil.
func NewTableFromCtorNNonNil(ctor *SmallTableCtorN, vals []Value) *Table {
	return newTableFromCtorNNonNil(ctor, vals, true)
}

// NewTableFromCtorNNonNilCache constructs a cache placeholder without updating
// shape-field type feedback. Native code overwrites every field before the
// object is exposed to program code.
func NewTableFromCtorNNonNilCache(ctor *SmallTableCtorN, vals []Value) *Table {
	return newTableFromCtorNNonNil(ctor, vals, false)
}

func newTableFromCtorNNonNil(ctor *SmallTableCtorN, vals []Value, observeTypes bool) *Table {
	if ctor == nil || ctor.Shape == nil || len(vals) < len(ctor.Keys) {
		return NewTableFromCtorN(ctor, vals)
	}
	return newTableFromCtorNNonNilWithCapacity(ctor, vals, len(ctor.Keys), observeTypes)
}

func newTableFromCtorNNonNilWithCapacity(ctor *SmallTableCtorN, vals []Value, capacity int, observeTypes bool) *Table {
	if ctor == nil || ctor.Shape == nil || len(vals) < len(ctor.Keys) {
		return NewTableFromCtorN(ctor, vals)
	}
	n := len(ctor.Keys)
	if capacity < n {
		capacity = n
	}
	t, svals := DefaultHeap.AllocTableWithSvals(capacity)
	t.svals = svals[:n]
	copy(t.svals, vals[:n])
	t.shape = ctor.Shape
	t.shapeID = ctor.shapeID
	t.skeys = ctor.fieldKeys
	if observeTypes {
		for i := 0; i < n; i++ {
			ObserveShapeFieldValueOnShape(ctor.Shape, i, t.svals[i])
		}
	}
	return t
}

func newTableFromCtorNFallback(ctor *SmallTableCtorN, vals []Value) *Table {
	if ctor == nil || len(ctor.Keys) == 0 {
		return NewEmptyTable()
	}
	if ctor.Shape != nil && len(ctor.Keys) <= 64 {
		n := len(ctor.Keys)
		if len(vals) < n {
			n = len(vals)
		}
		nonNil := 0
		var mask uint64
		for i := 0; i < n; i++ {
			if !vals[i].IsNil() {
				nonNil++
				mask |= uint64(1) << uint(i)
			}
		}
		switch nonNil {
		case 0:
			return NewEmptyTable()
		case len(ctor.Keys):
			return newTableFromCtorNNonNil(ctor, vals, true)
		default:
			key := sparseCtorNKey{shapeID: ctor.shapeID, mask: mask}
			cached, ok := sparseCtorNCache.Load(key)
			var sparse SmallTableCtorN
			if ok {
				sparse = cached.(SmallTableCtorN)
			} else {
				keys := make([]string, 0, nonNil)
				for i := 0; i < n; i++ {
					if mask&(uint64(1)<<uint(i)) != 0 {
						keys = append(keys, ctor.Keys[i])
					}
				}
				sparse = NewSmallTableCtorN(keys)
				if sparse.Shape == nil {
					break
				}
				actual, _ := sparseCtorNCache.LoadOrStore(key, sparse)
				sparse = actual.(SmallTableCtorN)
			}
			return newTableFromSparseCtorNWithCapacity(&sparse, vals, mask, nonNil, len(ctor.Keys), true)
		}
	}
	t := NewTableSized(0, len(ctor.Keys))
	for i, key := range ctor.Keys {
		if i >= len(vals) {
			break
		}
		t.RawSetString(key, vals[i])
	}
	return t
}

func newTableFromSparseCtorNWithCapacity(ctor *SmallTableCtorN, vals []Value, mask uint64, n, capacity int, observeTypes bool) *Table {
	if ctor == nil || ctor.Shape == nil || n <= 0 || len(vals) == 0 {
		return NewEmptyTable()
	}
	if capacity < n {
		capacity = n
	}
	t, svals := DefaultHeap.AllocTableWithSvals(capacity)
	t.svals = svals[:n]
	dst := 0
	for i := 0; i < len(vals) && dst < n; i++ {
		if mask&(uint64(1)<<uint(i)) == 0 {
			continue
		}
		t.svals[dst] = vals[i]
		dst++
	}
	t.shape = ctor.Shape
	t.shapeID = ctor.shapeID
	t.skeys = ctor.fieldKeys
	if observeTypes {
		for i := 0; i < n; i++ {
			ObserveShapeFieldValueOnShape(ctor.Shape, i, t.svals[i])
		}
	}
	return t
}

func newTableFromCtor2Fallback(ctor *SmallTableCtor2, val1, val2 Value) *Table {
	if ctor == nil {
		return NewTableSized(0, 2)
	}
	if ctor.Key1 == ctor.Key2 || ctor.Shape == nil {
		t := NewTableSized(0, 2)
		t.RawSetString(ctor.Key1, val1)
		t.RawSetString(ctor.Key2, val2)
		return t
	}

	val1Nil := val1.IsNil()
	val2Nil := val2.IsNil()
	if val1Nil {
		if val2Nil {
			return NewTableSized(0, 0)
		}
		return newTableFromCtorShape1(ctor.single2, val2)
	}
	if val2Nil {
		return newTableFromCtorShape1(ctor.single1, val1)
	}

	t := NewTableSized(0, 2)
	t.RawSetString(ctor.Key1, val1)
	t.RawSetString(ctor.Key2, val2)
	return t
}

func newTableFromCtorShape1(shape smallCtorShape, val Value) *Table {
	if shape.shape == nil {
		return NewTableSized(0, 0)
	}
	t, svals := DefaultHeap.AllocTableWithSvals1()
	t.svals = svals
	t.svals[0] = val
	t.shape = shape.shape
	t.shapeID = shape.shapeID
	t.skeys = shape.fieldKeys
	return t
}
