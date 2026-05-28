package runtime

import (
	"sync"
	"unsafe"
)

// ArrayKind indicates the type specialization of a table's array part.
type ArrayKind uint8

const (
	ArrayMixed ArrayKind = 0 // []Value (current, default)
	ArrayInt   ArrayKind = 1 // []int64 (int and bool values)
	ArrayFloat ArrayKind = 2 // []float64
	ArrayBool  ArrayKind = 3 // []byte (1 byte per bool, no GC pointers)
)

// smallFieldCap is the threshold for using flat slices vs maps for string keys.
const smallFieldCap = 12
const initialStringMapCap = 64

// SmallFieldCap is the maximum string-field count retained in the small shaped
// table representation.
const SmallFieldCap = smallFieldCap

// Table is GScript's associative array / object type.
// Tables have an optimized array part for sequential integer keys 1..n,
// flat slices for small string-keyed tables (most GScript objects),
// and maps for larger tables.
//
// Tables start WITHOUT a mutex (fast single-threaded path). When shared
// across goroutines, call SetConcurrent(true) to enable locking.
type Table struct {
	mu    *sync.RWMutex   // nil for single-threaded tables (fast default)
	array []Value         // 0-indexed: array[0] is usable by user code
	imap  map[int64]Value // integer keys not in array range
	// String keys: small tables use canonical Shape.FieldKeys plus per-table
	// values, large tables use map. Do not mutate skeys in place: it may be
	// shared by every table with the same shape.
	skeys     []string         // parallel with svals for small tables
	svals     []Value          // parallel with skeys for small tables
	smap      map[string]Value // only for tables with >smallFieldCap string keys
	hash      map[Value]Value  // everything else (bool, float, table, function keys)
	metatable *Table
	keys      []Value // ordered keys for Next() iteration
	keysDirty bool
	// Type-specialized array fields (placed at end to preserve existing offsets)
	arrayKind      ArrayKind
	arrayZeroValid bool
	shapeID        uint32 // shape identifier for field cache validation
	intArray       []int64
	floatArray     []float64
	boolArray      []byte // 1 byte per bool, no GC pointers → zero GC scan
	// Encoding: 0 = nil/unset, 1 = false, 2 = true
	// shape is the hidden-class descriptor for the string-keyed fields.
	// Always nil when shapeID == 0 (empty table or hash-mode table).
	// Kept in sync with shapeID by applyShape / clearShape.
	shape *Shape

	// DenseMatrix descriptor. When dmStride > 0, this Table is a DenseMatrix
	// outer whose rows share the backing at dmFlat. The JIT fast path for
	// nested float loads reads *((*float64)(dmFlat) + i*dmStride + j).
	// Backing storage is kept alive by dmMeta and by row floatArray slices;
	// dmFlat is only the JIT load address.
	dmFlat   unsafe.Pointer
	dmStride int32

	// arrayHint carries large array capacity hints until the first typed-array
	// promotion. Keep this after all JIT-verified fields.
	arrayHint int
	// dmMeta is a cold DenseMatrix side pointer. DenseMatrix outers and adopted
	// rows share one metadata object so every Table pays one pointer, not a
	// backing slice header plus parent pointer.
	dmMeta *denseMatrixMeta

	// lazyTree is a semantics-preserving deferred representation for qualified
	// fixed recursive two-field table builders. Generic table operations
	// materialize it before mutation or iteration.
	lazyTree *LazyRecursiveTable

	// stringLookupCache is a table-local inline cache for large string-key maps.
	// Small shaped tables use shape/slot caches; hash-mode string maps need a
	// different fact: key identity -> value. The cache is lazy so
	// tables that are never dynamically probed pay only this pointer.
	stringLookupCache   *StringLookupCache
	stringLookupVersion uint64
	arrayVersion        uint64

	nextKey   Value
	nextIndex int
	nextValid bool
}

// SetConcurrent enables or disables mutex protection for concurrent access.
func (t *Table) SetConcurrent(on bool) {
	if on && t.mu == nil {
		t.mu = &sync.RWMutex{}
	}
}

// cleanHashKey normalizes a Value for use as a Go map key.
// With NaN-boxing, Value is uint64 so map keys compare by bits.
// We still normalize float/int/string to ensure consistent hashing
// (e.g., -0.0 vs 0.0, or equivalent int/float representations).
func cleanHashKey(key Value) Value {
	switch key.Type() {
	case TypeInt:
		return IntValue(key.Int())
	case TypeFloat:
		return FloatValue(key.Float())
	case TypeString:
		return StringValue(key.Str())
	default:
		return key
	}
}

// RawGet retrieves a value by key, bypassing metamethods.
func (t *Table) RawGet(key Value) Value {
	if key.IsNil() {
		return NilValue()
	}
	if key.Type() == TypeInt {
		return t.RawGetInt(key.Int())
	}
	if key.Type() == TypeString {
		return t.RawGetString(key.Str())
	}
	// General hash for other types
	if t.mu != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	if t.hash != nil {
		if val, ok := t.hash[cleanHashKey(key)]; ok {
			return val
		}
	}
	return NilValue()
}

func (t *Table) rawGetForNextLocked(key Value) Value {
	if key.IsNil() {
		return NilValue()
	}
	if key.Type() == TypeInt {
		k := key.Int()
		if t.lazyTree != nil {
			return NilValue()
		}
		switch t.arrayKind {
		case ArrayInt:
			if k == 0 && !t.arrayZeroValid {
				return NilValue()
			}
			if k >= 0 && k < int64(len(t.intArray)) {
				return IntValue(t.intArray[k])
			}
		case ArrayFloat:
			if k == 0 && !t.arrayZeroValid {
				return NilValue()
			}
			if k >= 0 && k < int64(len(t.floatArray)) {
				return FloatValue(t.floatArray[k])
			}
		case ArrayBool:
			if k >= 0 && k < int64(len(t.boolArray)) {
				b := t.boolArray[k]
				if b == 0 {
					return NilValue()
				}
				return BoolValue(b == 2)
			}
		default:
			if k == 0 && !t.arrayZeroValid && len(t.array) > 0 && t.array[0] == 0 {
				return NilValue()
			}
			if k >= 0 && k < int64(len(t.array)) {
				return t.array[k]
			}
		}
		if t.imap != nil {
			if v, ok := t.imap[k]; ok {
				return v
			}
		}
		return NilValue()
	}
	if key.Type() == TypeString {
		k := key.Str()
		if t.lazyTree != nil {
			return t.lazyTree.get(t, k)
		}
		for i, field := range t.skeys {
			if field == k {
				return t.svals[i]
			}
		}
		if t.smap != nil {
			if v, ok := t.smap[k]; ok {
				return v
			}
		}
		return NilValue()
	}
	if t.hash != nil {
		if val, ok := t.hash[cleanHashKey(key)]; ok {
			return val
		}
	}
	return NilValue()
}

// ForEachPlainRaw visits a plain table's currently stored raw entries without
// materializing the Next/pairs key cache or performing a second lookup for each
// key. It is intentionally limited to tables whose traversal cannot be affected
// by metatables, lazy materialization, or concurrent mutation.
func (t *Table) ForEachPlainRaw(visit func(key, val Value) bool) bool {
	if t == nil || visit == nil || t.mu != nil || t.lazyTree != nil || t.metatable != nil {
		return false
	}
	switch t.arrayKind {
	case ArrayInt:
		if t.arrayZeroValid && len(t.intArray) > 0 {
			if !visit(IntValue(0), IntValue(t.intArray[0])) {
				return true
			}
		}
		for i := 1; i < len(t.intArray); i++ {
			if !visit(IntValue(int64(i)), IntValue(t.intArray[i])) {
				return true
			}
		}
	case ArrayFloat:
		if t.arrayZeroValid && len(t.floatArray) > 0 {
			if !visit(IntValue(0), FloatValue(t.floatArray[0])) {
				return true
			}
		}
		for i := 1; i < len(t.floatArray); i++ {
			if !visit(IntValue(int64(i)), FloatValue(t.floatArray[i])) {
				return true
			}
		}
	case ArrayBool:
		for i, b := range t.boolArray {
			if b == 0 {
				continue
			}
			if !visit(IntValue(int64(i)), BoolValue(b == 2)) {
				return true
			}
		}
	default:
		for i, v := range t.array {
			if v.IsNil() {
				continue
			}
			if !visit(IntValue(int64(i)), v) {
				return true
			}
		}
	}
	for k, v := range t.imap {
		if v.IsNil() {
			continue
		}
		if !visit(IntValue(k), v) {
			return true
		}
	}
	for i, k := range t.skeys {
		v := t.svals[i]
		if v.IsNil() {
			continue
		}
		if !visit(StringValue(k), v) {
			return true
		}
	}
	for k, v := range t.smap {
		if v.IsNil() {
			continue
		}
		if !visit(StringValue(k), v) {
			return true
		}
	}
	for k, v := range t.hash {
		if v.IsNil() {
			continue
		}
		if !visit(k, v) {
			return true
		}
	}
	return true
}

// RawGetInt retrieves a value by integer key (fast path, no Value boxing).
func (t *Table) RawGetInt(key int64) Value {
	if t.mu != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	if t.lazyTree != nil {
		RecordRuntimePathTableArrayGetFallback()
		return NilValue()
	}
	tableArrayGetPath(key, t)
	switch t.arrayKind {
	case ArrayInt:
		if key == 0 && !t.arrayZeroValid {
			return NilValue()
		}
		if key >= 0 && key < int64(len(t.intArray)) {
			return IntValue(t.intArray[key])
		}
	case ArrayFloat:
		if key == 0 && !t.arrayZeroValid {
			return NilValue()
		}
		if key >= 0 && key < int64(len(t.floatArray)) {
			return FloatValue(t.floatArray[key])
		}
	case ArrayBool:
		if key >= 0 && key < int64(len(t.boolArray)) {
			b := t.boolArray[key]
			if b == 0 { // nil/unset
				return NilValue()
			}
			return BoolValue(b == 2) // 1=false, 2=true
		}
	default:
		if key == 0 && !t.arrayZeroValid && len(t.array) > 0 && t.array[0] == 0 {
			return NilValue()
		}
		if key >= 0 && key < int64(len(t.array)) {
			return t.array[key]
		}
	}
	if t.imap != nil {
		if v, ok := t.imap[key]; ok {
			return v
		}
	}
	return NilValue()
}

// FieldIndex returns the index of a string key in the skeys slice, or -1 if not found.
// Used by the trace JIT to capture field positions at recording time.
func (t *Table) FieldIndex(key string) int {
	for i, k := range t.skeys {
		if k == key {
			return i
		}
	}
	return -1
}

// SkeysLen returns the length of the skeys slice.
func (t *Table) SkeysLen() int {
	return len(t.skeys)
}

// ShapeFieldNames returns the ordered string-field names for the table's
// current small-table shape. The returned slice is a copy so profiling code can
// safely retain it across later table mutations.
func (t *Table) ShapeFieldNames() []string {
	if t == nil || t.shapeID == 0 || len(t.skeys) == 0 {
		return nil
	}
	return append([]string(nil), t.skeys...)
}

// SvalsGet returns the value at index i in the svals slice.
// Used by the SSA interpreter (golden model) to access fields by index.
func (t *Table) SvalsGet(i int) Value {
	if i >= 0 && i < len(t.svals) {
		return t.svals[i]
	}
	return NilValue()
}

// SvalsSet sets the value at index i in the svals slice.
// Used by the SSA interpreter (golden model) to write fields by index.
func (t *Table) SvalsSet(i int, v Value) {
	if i >= 0 && i < len(t.svals) {
		t.svals[i] = v
		t.keysDirty = true
	}
}

// HasMetatable returns true if the table has a metatable.
func (t *Table) HasMetatable() bool {
	return t.metatable != nil
}

// RawSet assigns a value by key, bypassing metamethods.
func (t *Table) RawSet(key, val Value) {
	if key.IsNil() {
		return
	}
	kt := key.Type()
	if kt == TypeFloat && floatIsInt(key.Float()) {
		key = IntValue(int64(key.Float()))
		kt = TypeInt
	}
	if kt == TypeInt {
		t.RawSetInt(key.Int(), val)
		return
	}
	if kt == TypeString {
		t.RawSetString(key.Str(), val)
		return
	}
	// General hash
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if t.lazyTree != nil {
		t.materializeLazyTreeLocked()
	}
	t.keysDirty = true
	if t.hash == nil {
		if val.IsNil() {
			return
		}
		t.hash = make(map[Value]Value)
	}
	ck := cleanHashKey(key)
	if val.IsNil() {
		delete(t.hash, ck)
	} else {
		t.hash[ck] = val
	}
}

func (t *Table) bumpArrayVersionLocked() {
	t.arrayVersion++
	if t.arrayVersion == 0 {
		t.arrayVersion = 1
	}
}

// MutationVersion returns a best-effort mutation counter for caches derived
// from raw table contents. It tracks array writes and string-field writes once
// the string-field counter has been enabled by a cache user.
func (t *Table) MutationVersion() uint64 {
	if t == nil {
		return 0
	}
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	return t.arrayVersion ^ (t.enableStringLookupVersionLocked() << 32)
}

// SampleStringTableValues visits up to limit table-valued entries stored under
// string keys. It samples both fixed-shape string fields and the hash part so
// profiling can learn generic string-map value shapes without exposing table
// internals.
func (t *Table) SampleStringTableValues(limit int, visit func(Value)) {
	if t == nil || limit <= 0 || visit == nil {
		return
	}
	if t.mu != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	seen := 0
	for _, val := range t.svals {
		if !val.IsTable() {
			continue
		}
		visit(val)
		seen++
		if seen >= limit {
			return
		}
	}
	for _, val := range t.smap {
		if !val.IsTable() {
			continue
		}
		visit(val)
		seen++
		if seen >= limit {
			return
		}
	}
	for key, val := range t.hash {
		if !key.IsString() || !val.IsTable() {
			continue
		}
		visit(val)
		seen++
		if seen >= limit {
			return
		}
	}
}

// Length returns the length of the array part (the # operator).
func (t *Table) Length() int {
	switch t.arrayKind {
	case ArrayInt:
		// All slots are valid (no nil concept for int64), length is always full.
		if len(t.intArray) == 0 {
			return 0
		}
		return len(t.intArray) - 1
	case ArrayFloat:
		// All slots are valid for float64 as well.
		if len(t.floatArray) == 0 {
			return 0
		}
		return len(t.floatArray) - 1
	case ArrayBool:
		// Scan backwards past nil sentinels (0 = unset)
		n := len(t.boolArray) - 1
		for n > 0 && t.boolArray[n] == 0 {
			n--
		}
		return n
	default:
		if len(t.array) == 0 {
			return 0
		}
		n := len(t.array) - 1
		for n > 0 && t.array[n].IsNil() {
			n--
		}
		return n
	}
}

// Len returns the length of the array part (alias for Length, used by VM).
func (t *Table) Len() int {
	return t.Length()
}

// Append adds a value to the end of the array part.
func (t *Table) Append(v Value) {
	n := t.Length()
	t.RawSet(IntValue(int64(n+1)), v)
}

// GetMetatable returns the table's metatable, or nil.
func (t *Table) GetMetatable() *Table {
	return t.metatable
}

// SetMetatable sets the table's metatable.
func (t *Table) SetMetatable(mt *Table) {
	t.metatable = mt
}

// ShapeID returns the table's shape identifier.
func (t *Table) ShapeID() uint32 { return t.shapeID }

// TableShapeIDOffset returns the offset of shapeID for JIT verification.

// GetArrayKind returns the array kind for testing/JIT inspection.
func (t *Table) GetArrayKind() ArrayKind {
	return t.arrayKind
}

// DMStride returns the DenseMatrix stride; 0 for non-DenseMatrix tables.
// Used by tests and feedback-driven intrinsic gating.
func (t *Table) DMStride() int32 { return t.dmStride }
