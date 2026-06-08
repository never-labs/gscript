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

// NativePayloadKind classifies host-side table facades without exposing the
// concrete payload type to runtime.
type NativePayloadKind string

const (
	NativePayloadNone       NativePayloadKind = ""
	NativePayloadDataFrame  NativePayloadKind = "data_frame"
	NativePayloadDataColumn NativePayloadKind = "data_column"
	NativePayloadKeyedFrame NativePayloadKind = "data_keyed_frame"
)

// IsFrameFacadeKind reports whether this native payload kind is represented as
// a first-class runtime frame value instead of a plain table.
func (k NativePayloadKind) IsFrameFacadeKind() bool {
	return k == NativePayloadDataFrame || k == NativePayloadKeyedFrame
}

// ValueType returns the runtime value type carried by this payload kind, when
// it has one. Column payloads remain table facades from runtime's perspective.
func (k NativePayloadKind) ValueType() (ValueType, bool) {
	switch k {
	case NativePayloadDataFrame:
		return TypeFrame, true
	case NativePayloadKeyedFrame:
		return TypeKeyedFrame, true
	default:
		return TypeNil, false
	}
}

// TypeName returns the user-facing runtime type name for payload kinds that are
// promoted to first-class frame values.
func (k NativePayloadKind) TypeName() (string, bool) {
	switch k {
	case NativePayloadDataFrame:
		return "frame", true
	case NativePayloadKeyedFrame:
		return "keyed frame", true
	default:
		return "", false
	}
}

// Table is Leia's associative array / object type.
// Tables have an optimized array part for sequential integer keys 1..n,
// flat slices for small string-keyed tables (most Leia objects),
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

	// lazyIntGetter exposes a dense integer-keyed view without materializing
	// the array part. Data-frame bindings use it for row access over columnar
	// storage: t[1] can build one row while #t still reports the frame length.
	// Keep this cold field after JIT-verified layout fields.
	lazyIntGetter func(int64) (Value, bool)
	lazyIntLength int

	// nativePayload is an optional host-side representation for tables that
	// primarily expose runtime data through a table facade. It is deliberately
	// opaque to runtime so packages above runtime can attach columnar frames or
	// similar payloads without introducing import cycles.
	nativePayload     any
	nativePayloadInfo *NativePayloadInfo

	// lazyStringGetter exposes deferred string fields for native facades whose
	// script-visible wrappers are expensive and often never inspected.
	lazyStringGetter func(string) (Value, bool)
}

// NativePayloadInfo is a small runtime-facing description of an opaque native
// table payload. It lets stdlib boundaries identify stable value categories and
// schema facts without importing the payload's owning package.
type NativePayloadInfo struct {
	Kind       NativePayloadKind
	Rows       int
	Columns    int
	ColumnKind string
	SchemaHash string
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
	if t.lazyIntGetter != nil && key >= 1 && key <= int64(t.lazyIntLength) {
		if v, ok := t.lazyIntGetter(key); ok {
			return v
		}
	}
	return NilValue()
}

// SetLazyIntGetter installs a deferred dense integer-keyed view. Existing
// concrete integer entries keep priority over the getter. Any later integer
// write clears the getter to avoid stale derived values.
func (t *Table) SetLazyIntGetter(length int, getter func(int64) (Value, bool)) {
	if t == nil {
		return
	}
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	if length <= 0 || getter == nil {
		t.lazyIntGetter = nil
		t.lazyIntLength = 0
		return
	}
	t.lazyIntGetter = getter
	t.lazyIntLength = length
}

// SetLazyStringGetter installs a deferred string-keyed view. Concrete string
// fields keep priority over the getter. Any later write clears the getter along
// with the native payload to avoid stale derived fields.
func (t *Table) SetLazyStringGetter(getter func(string) (Value, bool)) {
	if t == nil {
		return
	}
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	t.lazyStringGetter = getter
}

// SetNativePayload attaches an opaque host payload to the table. The payload is
// invalidated by raw table writes, so callers should install it after finishing
// visible table decoration.
func (t *Table) SetNativePayload(payload any) {
	t.SetNativePayloadWithInfo(payload, NativePayloadInfo{})
}

// SetNativePayloadWithInfo attaches an opaque host payload with a stable
// runtime-facing description. The payload and its info are invalidated together
// by raw table writes.
func (t *Table) SetNativePayloadWithInfo(payload any, info NativePayloadInfo) {
	if t == nil {
		return
	}
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	t.nativePayload = payload
	if info == (NativePayloadInfo{}) {
		t.nativePayloadInfo = nil
		return
	}
	copied := info
	t.nativePayloadInfo = &copied
}

// NativePayload returns the table's opaque host payload, if still valid.
func (t *Table) NativePayload() any {
	if t == nil {
		return nil
	}
	if t.mu != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	return t.nativePayload
}

// NativePayloadInfo returns the current native payload description, if one was
// installed and the payload has not been invalidated.
func (t *Table) NativePayloadInfo() (NativePayloadInfo, bool) {
	if t == nil {
		return NativePayloadInfo{}, false
	}
	if t.mu != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	if t.nativePayload == nil || t.nativePayloadInfo == nil {
		return NativePayloadInfo{}, false
	}
	return *t.nativePayloadInfo, true
}

// NativePayloadKind reports the stable category attached to the native
// payload, if one is installed and still valid.
func (t *Table) NativePayloadKind() (NativePayloadKind, bool) {
	info, ok := t.NativePayloadInfo()
	if !ok {
		return NativePayloadNone, false
	}
	return info.Kind, true
}

// NativeFramePayloadInfo returns native payload metadata only for table
// facades that runtime promotes to frame/keyed-frame values.
func (t *Table) NativeFramePayloadInfo() (NativePayloadInfo, bool) {
	info, ok := t.NativePayloadInfo()
	if !ok || !info.Kind.IsFrameFacadeKind() {
		return NativePayloadInfo{}, false
	}
	return info, true
}

// NativeFramePayload returns the opaque payload plus metadata for native frame
// facades. It keeps the runtime frame carrier check in one place so callers do
// not have to separately inspect payload kind and then fetch the payload.
func (t *Table) NativeFramePayload() (any, NativePayloadInfo, bool) {
	if t == nil {
		return nil, NativePayloadInfo{}, false
	}
	if t.mu != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	if t.nativePayload == nil || t.nativePayloadInfo == nil || !t.nativePayloadInfo.Kind.IsFrameFacadeKind() {
		return nil, NativePayloadInfo{}, false
	}
	return t.nativePayload, *t.nativePayloadInfo, true
}

// NativeFramePayloadKind reports whether this table currently carries a native
// frame or keyed-frame facade payload.
func (t *Table) NativeFramePayloadKind() (NativePayloadKind, bool) {
	info, ok := t.NativeFramePayloadInfo()
	if !ok {
		return NativePayloadNone, false
	}
	return info.Kind, true
}

// HasNativePayloadKind reports whether the table currently carries the
// requested native payload category.
func (t *Table) HasNativePayloadKind(kind NativePayloadKind) bool {
	if kind == NativePayloadNone {
		return false
	}
	got, ok := t.NativePayloadKind()
	return ok && got == kind
}

// IsFrameFacade reports whether the table currently carries a runtime frame
// facade payload.
func (t *Table) IsFrameFacade() bool {
	kind, ok := t.NativeFramePayloadKind()
	return ok && kind == NativePayloadDataFrame
}

// IsKeyedFrameFacade reports whether the table currently carries a runtime
// keyed-frame facade payload.
func (t *Table) IsKeyedFrameFacade() bool {
	kind, ok := t.NativeFramePayloadKind()
	return ok && kind == NativePayloadKeyedFrame
}

// IsNativeFrame reports whether the table currently carries a native frame payload.
func (t *Table) IsNativeFrame() bool { return t.IsFrameFacade() }

// IsNativeKeyedFrame reports whether the table currently carries a native keyed frame payload.
func (t *Table) IsNativeKeyedFrame() bool { return t.IsKeyedFrameFacade() }

// IsNativeColumn reports whether the table currently carries a native column payload.
func (t *Table) IsNativeColumn() bool {
	return t.HasNativePayloadKind(NativePayloadDataColumn)
}

func (t *Table) clearNativePayloadLocked() {
	t.nativePayload = nil
	t.nativePayloadInfo = nil
	t.lazyIntGetter = nil
	t.lazyIntLength = 0
	t.lazyStringGetter = nil
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
		t.clearNativePayloadLocked()
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
	t.clearNativePayloadLocked()
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
	lazyLen := 0
	if t.lazyIntGetter != nil && t.lazyIntLength > 0 {
		lazyLen = t.lazyIntLength
	}
	switch t.arrayKind {
	case ArrayInt:
		// All slots are valid (no nil concept for int64), length is always full.
		if len(t.intArray) == 0 {
			return lazyLen
		}
		if n := len(t.intArray) - 1; n > lazyLen {
			return n
		}
		return lazyLen
	case ArrayFloat:
		// All slots are valid for float64 as well.
		if len(t.floatArray) == 0 {
			return lazyLen
		}
		if n := len(t.floatArray) - 1; n > lazyLen {
			return n
		}
		return lazyLen
	case ArrayBool:
		// Scan backwards past nil sentinels (0 = unset)
		n := len(t.boolArray) - 1
		for n > 0 && t.boolArray[n] == 0 {
			n--
		}
		if lazyLen > n {
			return lazyLen
		}
		return n
	default:
		if len(t.array) == 0 {
			return lazyLen
		}
		n := len(t.array) - 1
		for n > 0 && t.array[n].IsNil() {
			n--
		}
		if lazyLen > n {
			return lazyLen
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
