package runtime

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// ValueType represents the type of a Leia value.
type ValueType uint8

const (
	TypeNil        ValueType = iota
	TypeBool                 // boolean
	TypeInt                  // integer numbers
	TypeFloat                // floating-point numbers
	TypeString               // strings
	TypeTable                // tables (associative arrays)
	TypeFunction             // functions (closures and Go functions)
	TypeCoroutine            // coroutines
	TypeChannel              // channels
	TypeDenseArray           // data-oriented dense arrays
	TypeSoA                  // structure-of-arrays records
	TypeFrame                // native data frame facade
	TypeKeyedFrame           // native keyed data frame facade
)

// ---------------------------------------------------------------------------
// NaN-boxing constants
// ---------------------------------------------------------------------------
//
// Value is an 8-byte NaN-boxed uint64.
//
//	Float64:  any IEEE 754 bit pattern where bits 50-62 are NOT all 1.
//	Tagged:   bits 50-62 all 1 (qNaN), sign=1, bits 48-49 = type tag,
//	          bits 0-47 = 48-bit payload.
//
//	tag 00 = nil      (payload 0)
//	tag 01 = bool     (payload 0=false, 1=true)
//	tag 10 = int48    (48-bit two's complement)
//	tag 11 = pointer  (bits 0-43 = 44-bit address, bits 44-47 = ptr sub-type)

const (
	// nanBits: bits 50-62 all set = quiet NaN with our discriminator bit (50).
	nanBits uint64 = 0x7FFC000000000000

	// Type tags (sign=1 + nanBits + 2-bit tag in bits 48-49).
	tagNil  uint64 = 0xFFFC000000000000 // sign=1, tag=00
	tagBool uint64 = 0xFFFD000000000000 // sign=1, tag=01
	tagInt  uint64 = 0xFFFE000000000000 // sign=1, tag=10
	tagPtr  uint64 = 0xFFFF000000000000 // sign=1, tag=11

	// Masks.
	tagMask     uint64 = 0xFFFF000000000000 // top 16 bits
	payloadMask uint64 = 0x0000FFFFFFFFFFFF // bottom 48 bits

	// Pre-built special values.
	valNil   uint64 = tagNil
	valFalse uint64 = tagBool     // payload = 0
	valTrue  uint64 = tagBool | 1 // payload = 1

	// Canonical NaN (Go/IEEE 754 standard quiet NaN). Bit 50 is 0, so it
	// does NOT collide with our tagged space (which requires bit 50 = 1).
	canonicalNaN uint64 = 0x7FF8000000000000

	// Int48 range limits.
	maxInt48 int64 = (1 << 47) - 1 //  140_737_488_355_327
	minInt48 int64 = -(1 << 47)    // -140_737_488_355_328

	// Pointer sub-type bits (stored in bits 44-47 of the pointer payload).
	// macOS ARM64 pointers use ~41 bits, so bits 44-47 are free.
	ptrSubShift        = 44
	ptrSubMask  uint64 = 0xF << ptrSubShift     // bits 44-47
	ptrAddrMask uint64 = (1 << ptrSubShift) - 1 // bits 0-43

	ptrSubTable       uint64 = 0 << ptrSubShift
	ptrSubString      uint64 = 1 << ptrSubShift
	ptrSubClosure     uint64 = 2 << ptrSubShift // *runtime.Closure
	ptrSubGoFunction  uint64 = 3 << ptrSubShift // *GoFunction
	ptrSubCoroutine   uint64 = 4 << ptrSubShift // *Coroutine
	ptrSubChannel     uint64 = 5 << ptrSubShift
	ptrSubAnyFunction uint64 = 6 << ptrSubShift  // interface-based function (needs ifaceRoots)
	ptrSubAnyCoro     uint64 = 7 << ptrSubShift  // interface-based coroutine (needs ifaceRoots)
	ptrSubVMClosure   uint64 = 8 << ptrSubShift  // *vm.Closure (direct pointer, fast OP_CALL path)
	ptrSubLazyString  uint64 = 9 << ptrSubShift  // *lazyString
	ptrSubVMCoroutine uint64 = 10 << ptrSubShift // *vm.VMCoroutine (direct pointer, fast coroutine path)
	ptrSubFixedRecord uint64 = 11 << ptrSubShift // *FixedRecord, materializes to *Table on generic table use
	ptrSubDenseArray  uint64 = 12 << ptrSubShift // *DenseArray
	ptrSubSoA         uint64 = 13 << ptrSubShift // *SoA

)

// Value is a NaN-boxed 8-byte representation of all Leia values.
// Replaces the old 24-byte struct {typ, data, ptr}.
type Value uint64

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

func NilValue() Value {
	return Value(valNil)
}

func BoolValue(b bool) Value {
	if b {
		return Value(valTrue)
	}
	return Value(valFalse)
}

func IntValue(i int64) Value {
	if i > maxInt48 || i < minInt48 {
		// Overflow: promote to float64 (matches LuaJIT semantics).
		return FloatValue(float64(i))
	}
	return Value(tagInt | (uint64(i) & payloadMask))
}

func FloatValue(f float64) Value {
	bits := math.Float64bits(f)
	// Canonicalize exotic NaN patterns that collide with our tag space.
	if bits&nanBits == nanBits {
		return Value(canonicalNaN)
	}
	return Value(bits)
}

func StringValue(s string) Value {
	var sp *string
	if DefaultHeap != nil {
		sp = DefaultHeap.AllocStringBox(s)
	} else {
		sp = new(string)
		*sp = s
	}
	p := unsafe.Pointer(sp)
	if DefaultHeap == nil {
		keepAlive(p, sp)
	}
	return Value(tagPtr | ptrSubString | (uint64(uintptr(p)) & ptrAddrMask))
}

const (
	cachedIntStringMin = 0
	cachedIntStringMax = 1023
)

var (
	cachedIntStringOnce   sync.Once
	cachedIntStringValues [cachedIntStringMax - cachedIntStringMin + 1]Value
)

func CachedIntStringValue(i int64) (Value, bool) {
	if i < cachedIntStringMin || i > cachedIntStringMax {
		return NilValue(), false
	}
	cachedIntStringOnce.Do(func() {
		for n := cachedIntStringMin; n <= cachedIntStringMax; n++ {
			cachedIntStringValues[n-cachedIntStringMin] = StringValue(strconv.FormatInt(int64(n), 10))
		}
	})
	return cachedIntStringValues[i-cachedIntStringMin], true
}

const lazyConcatThreshold = 64

type lazyString struct {
	leftString  string
	leftLazy    *lazyString
	rightString string
	rightLazy   *lazyString
	length      int
}

func LazyStringValue(left, right Value) Value {
	if !canNativeConcat(left) || !canNativeConcat(right) {
		return ConcatValues([]Value{left, right})
	}
	total := StringLen(left) + StringLen(right)
	if total <= lazyConcatThreshold {
		l, _ := ConcatOperandString(left)
		r, _ := ConcatOperandString(right)
		return StringValue(l + r)
	}
	ls := &lazyString{length: total}
	setLazyPart(&ls.leftString, &ls.leftLazy, left)
	setLazyPart(&ls.rightString, &ls.rightLazy, right)
	p := unsafe.Pointer(ls)
	keepAlive(p, ls)
	return Value(tagPtr | ptrSubLazyString | (uint64(uintptr(p)) & ptrAddrMask))
}

func setLazyPart(dstString *string, dstLazy **lazyString, v Value) {
	if lz := v.lazyString(); lz != nil {
		*dstLazy = lz
		return
	}
	s, _ := ConcatOperandString(v)
	*dstString = s
}

func (v Value) lazyString() *lazyString {
	if uint64(v)&tagMask != tagPtr || v.ptrSubType() != ptrSubLazyString {
		return nil
	}
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	return (*lazyString)(p)
}

func scanLazyStringRoots(ls *lazyString, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if ls == nil {
		return
	}
	if ls.leftLazy != nil {
		p := unsafe.Pointer(ls.leftLazy)
		addr := uintptr(p)
		if _, ok := seen[addr]; !ok {
			seen[addr] = struct{}{}
			visitor(p)
			scanLazyStringRoots(ls.leftLazy, visitor, seen)
		}
	}
	if ls.rightLazy != nil {
		p := unsafe.Pointer(ls.rightLazy)
		addr := uintptr(p)
		if _, ok := seen[addr]; !ok {
			seen[addr] = struct{}{}
			visitor(p)
			scanLazyStringRoots(ls.rightLazy, visitor, seen)
		}
	}
}

func (ls *lazyString) materialize() string {
	if ls == nil {
		return ""
	}
	var b strings.Builder
	b.Grow(ls.length)
	writeLazyStringTree(&b, ls)
	return b.String()
}

func writeLazyStringTree(b *strings.Builder, ls *lazyString) {
	if ls != nil {
		writeLazyStringPart(b, ls.leftString, ls.leftLazy)
		writeLazyStringPart(b, ls.rightString, ls.rightLazy)
		return
	}
}

func writeLazyStringPart(b *strings.Builder, s string, ls *lazyString) {
	if ls != nil {
		writeLazyStringTree(b, ls)
		return
	}
	b.WriteString(s)
}

func canNativeConcat(v Value) bool {
	return v.IsString() || v.IsNumber()
}

func ConcatOperandString(v Value) (string, bool) {
	switch v.Type() {
	case TypeString:
		return v.Str(), true
	case TypeInt:
		return strconv.FormatInt(v.Int(), 10), true
	case TypeFloat:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64), true
	default:
		return "", false
	}
}

func StringLen(v Value) int {
	if lz := v.lazyString(); lz != nil {
		return lz.length
	}
	if v.IsString() {
		return len(v.Str())
	}
	return 0
}

// ConcatValues joins concat operands. Binary string/number chains use an
// immutable lazy node once the result is large enough; other cases keep the
// exact-size materializing builder used by cold paths and tests.
func ConcatValues(values []Value) Value {
	switch len(values) {
	case 0:
		RecordRuntimePathStringConcatBuilder()
		return StringValue("")
	case 1:
		RecordRuntimePathStringConcatBuilder()
		return StringValue(values[0].String())
	case 2:
		if canNativeConcat(values[0]) && canNativeConcat(values[1]) {
			RecordRuntimePathStringConcatLazy()
			return LazyStringValue(values[0], values[1])
		}
		RecordRuntimePathStringConcatBuilder()
		left := concatString(values[0])
		right := concatString(values[1])
		var b strings.Builder
		b.Grow(len(left) + len(right))
		b.WriteString(left)
		b.WriteString(right)
		return StringValue(b.String())
	}

	RecordRuntimePathStringConcatBuilder()
	var local [8]string
	parts := local[:0]
	if len(values) > len(local) {
		parts = make([]string, 0, len(values))
	}
	total := 0
	for _, v := range values {
		s := concatString(v)
		total += len(s)
		parts = append(parts, s)
	}

	var b strings.Builder
	b.Grow(total)
	for _, s := range parts {
		b.WriteString(s)
	}
	return StringValue(b.String())
}

func concatString(v Value) string {
	if v.IsString() {
		return v.Str()
	}
	return v.String()
}

func TableValue(t *Table) Value {
	if t == nil {
		return Value(valNil)
	}
	p := unsafe.Pointer(t)
	if DefaultHeap == nil || !DefaultHeap.tablePointerInCurrentSlab(uintptr(p)) {
		keepAlive(p, t)
	}
	return Value(tagPtr | ptrSubTable | (uint64(uintptr(p)) & ptrAddrMask))
}

// FreshTableValue boxes a table that was just allocated by DefaultHeap.
// The table slab root is already kept alive when the slab is published, so
// fresh allocation sites can avoid the generic TableValue root-log check.
func FreshTableValue(t *Table) Value {
	if t == nil {
		return Value(valNil)
	}
	p := unsafe.Pointer(t)
	return Value(tagPtr | ptrSubTable | (uint64(uintptr(p)) & ptrAddrMask))
}

// TableGCRoot returns the Go-visible pointer that keeps t alive while a
// NaN-boxed Value hides the table pointer from the Go GC. Slab-backed tables
// can share one root per slab; non-slab tables root themselves.
func TableGCRoot(t *Table) unsafe.Pointer {
	if t == nil {
		return nil
	}
	p := unsafe.Pointer(t)
	if DefaultHeap != nil {
		if root := DefaultHeap.tableRootInCurrentSlab(uintptr(p)); root != nil {
			return root
		}
	}
	if root := tableSlabRootForPointer(p); root != nil {
		return root
	}
	return p
}

// iface is the memory layout of a Go interface{}/any value.
type iface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

// FunctionValue stores a function value (either *Closure or *GoFunction or any
// other pointer type). The pointer sub-type bits distinguish Closure from
// GoFunction. For other types, we use ptrSubAnyFunction and store the full
// interface in gcRoots for later reconstruction.
func FunctionValue(f interface{}) Value {
	if f == nil {
		return Value(valNil)
	}
	switch fn := f.(type) {
	case *Closure:
		p := unsafe.Pointer(fn)
		keepAlive(p, f)
		return Value(tagPtr | ptrSubClosure | (uint64(uintptr(p)) & ptrAddrMask))
	case *GoFunction:
		p := unsafe.Pointer(fn)
		keepAlive(p, f)
		return Value(tagPtr | ptrSubGoFunction | (uint64(uintptr(p)) & ptrAddrMask))
	default:
		// Unknown function type (e.g. *vm.Closure) -- store via interface
		i := (*iface)(unsafe.Pointer(&f))
		p := i.data
		keepAliveIface(p, f) // store the full interface for later reconstruction
		return Value(tagPtr | ptrSubAnyFunction | (uint64(uintptr(p)) & ptrAddrMask))
	}
}

// VMClosureFunctionValue stores a vm.Closure pointer using ptrSubVMClosure (8).
// The JIT can fast-check sub-type == 8 to know this is a vm.Closure and safely
// dereference the 44-bit pointer to access Proto and CompiledCodePtr.
// The caller must pass the raw unsafe.Pointer to the vm.Closure struct.
// The original interface is stored via keepAliveIface for Go-side reconstruction.
func VMClosureFunctionValue(p unsafe.Pointer, f interface{}) Value {
	if p == nil {
		return Value(valNil)
	}
	keepAliveIface(p, f)
	return Value(tagPtr | ptrSubVMClosure | (uint64(uintptr(p)) & ptrAddrMask))
}

// VMClosureFastValue stores a VM-owned closure pointer without recording a
// recoverable interface entry. Use this for VM/JIT-created bytecode closures
// that are recovered through VMClosurePointer rather than Value.Ptr().
func VMClosureFastValue(p unsafe.Pointer) Value {
	if p == nil {
		return Value(valNil)
	}
	keepAlive(p, nil)
	return Value(tagPtr | ptrSubVMClosure | (uint64(uintptr(p)) & ptrAddrMask))
}

// VMClosurePointer returns the raw pointer stored by a VM closure value.
//
// The runtime package cannot name internal/vm.Closure without creating an
// import cycle, so VM/JIT callers cast the returned pointer in their package.
// Values created by VMClosureFunctionValue can still be reconstructed through
// Ptr(); values created by VMClosureFastValue intentionally cannot.
func (v Value) VMClosurePointer() unsafe.Pointer {
	if uint64(v)&tagMask != tagPtr || v.ptrSubType() != ptrSubVMClosure {
		return nil
	}
	return v.ptrPayload()
}

func CoroutineValue(c *Coroutine) Value {
	if c == nil {
		return Value(valNil)
	}
	p := unsafe.Pointer(c)
	keepAlive(p, c)
	return Value(tagPtr | ptrSubCoroutine | (uint64(uintptr(p)) & ptrAddrMask))
}

// AnyCoroutineValue stores a coroutine value from an arbitrary pointer type
// (e.g. *VMCoroutine from the vm package).
func AnyCoroutineValue(c any) Value {
	if c == nil {
		return Value(valNil)
	}
	i := (*iface)(unsafe.Pointer(&c))
	p := i.data
	keepAliveIface(p, c) // store full interface for lookupIface
	return Value(tagPtr | ptrSubAnyCoro | (uint64(uintptr(p)) & ptrAddrMask))
}

// VMCoroutineValue stores a VM-owned coroutine pointer. Hot VM paths recover
// the raw pointer through AnyCoroutinePointer, while Ptr still works for callers
// that use the public Value API.
func VMCoroutineValue(p unsafe.Pointer, c any) Value {
	if p == nil {
		return Value(valNil)
	}
	keepAlive(p, c)
	return Value(tagPtr | ptrSubVMCoroutine | (uint64(uintptr(p)) & ptrAddrMask))
}

func ChannelValue(ch *Channel) Value {
	if ch == nil {
		return Value(valNil)
	}
	p := unsafe.Pointer(ch)
	keepAlive(p, ch)
	return Value(tagPtr | ptrSubChannel | (uint64(uintptr(p)) & ptrAddrMask))
}

// ---------------------------------------------------------------------------
// Internal helpers for NaN-box decoding
// ---------------------------------------------------------------------------

// pointerFromUintptr recovers a Go pointer from an address managed by the Leia
// runtime. It is used only at low-level NaN-box/slab decode boundaries.
//
// Leia intentionally stores non-moving Go heap pointers in integer payloads for
// Value tags and slab metadata. The heap/iface root tables keep referents alive;
// this function is the corresponding pointer recovery primitive. Go's checkptr
// instrumentation cannot prove that invariant from the integer bits alone, so
// keep the exemption local instead of disabling checkptr for the package or
// tests.
//
//go:nocheckptr
func pointerFromUintptr(addr uintptr) unsafe.Pointer {
	return unsafe.Pointer(addr)
}

// ptrPayload extracts the raw pointer from a pointer-tagged Value.
func (v Value) ptrPayload() unsafe.Pointer {
	return pointerFromUintptr(uintptr(uint64(v) & ptrAddrMask))
}

// ptrSubType extracts the pointer sub-type bits (44-47) from a pointer-tagged Value.
func (v Value) ptrSubType() uint64 {
	return uint64(v) & ptrSubMask
}

// ---------------------------------------------------------------------------
// Type checks
// ---------------------------------------------------------------------------

func (v Value) Type() ValueType {
	bits := uint64(v)

	// Float: bits 50-62 are NOT all set.
	if bits&nanBits != nanBits {
		return TypeFloat
	}

	// Tagged value: check tag bits.
	tag := bits & tagMask
	switch tag {
	case tagNil:
		return TypeNil
	case tagBool:
		return TypeBool
	case tagInt:
		return TypeInt
	case tagPtr:
		// Determine specific pointer type from sub-type bits.
		sub := bits & ptrSubMask
		switch sub {
		case ptrSubTable, ptrSubFixedRecord:
			if kind, ok := v.nativeFramePayloadKind(); ok {
				if typ, ok := kind.ValueType(); ok {
					return typ
				}
			}
			return TypeTable
		case ptrSubString, ptrSubLazyString:
			return TypeString
		case ptrSubClosure, ptrSubGoFunction, ptrSubAnyFunction, ptrSubVMClosure:
			return TypeFunction
		case ptrSubCoroutine, ptrSubAnyCoro, ptrSubVMCoroutine:
			return TypeCoroutine
		case ptrSubChannel:
			return TypeChannel
		case ptrSubDenseArray:
			return TypeDenseArray
		case ptrSubSoA:
			return TypeSoA
		default:
			return TypeTable // fallback
		}
	default:
		return TypeNil
	}
}

func (v Value) IsNil() bool    { return uint64(v) == valNil }
func (v Value) IsBool() bool   { return uint64(v)&tagMask == tagBool }
func (v Value) IsInt() bool    { return uint64(v)&tagMask == tagInt }
func (v Value) IsFloat() bool  { return uint64(v)&nanBits != nanBits }
func (v Value) IsNumber() bool { return v.IsFloat() || v.IsInt() }

func (v Value) IsString() bool {
	if uint64(v)&tagMask != tagPtr {
		return false
	}
	sub := v.ptrSubType()
	return sub == ptrSubString || sub == ptrSubLazyString
}

func (v Value) IsTable() bool {
	if uint64(v)&tagMask != tagPtr {
		return false
	}
	sub := v.ptrSubType()
	return sub == ptrSubTable || sub == ptrSubFixedRecord
}

func (v Value) IsFunction() bool {
	if uint64(v)&tagMask != tagPtr {
		return false
	}
	sub := v.ptrSubType()
	return sub == ptrSubClosure || sub == ptrSubGoFunction || sub == ptrSubAnyFunction || sub == ptrSubVMClosure
}

func (v Value) IsCoroutine() bool {
	if uint64(v)&tagMask != tagPtr {
		return false
	}
	sub := v.ptrSubType()
	return sub == ptrSubCoroutine || sub == ptrSubAnyCoro || sub == ptrSubVMCoroutine
}

// AnyCoroutinePointer returns the raw data pointer stored by AnyCoroutineValue
// or VMCoroutineValue.
// It is intentionally narrower than Ptr(): VM coroutine hot paths only need to
// recover their own concrete pointer and should not take the ifaceRoots mutex.
func (v Value) AnyCoroutinePointer() unsafe.Pointer {
	if uint64(v)&tagMask != tagPtr {
		return nil
	}
	sub := v.ptrSubType()
	if sub != ptrSubAnyCoro && sub != ptrSubVMCoroutine {
		return nil
	}
	return v.ptrPayload()
}

func (v Value) IsChannel() bool {
	return uint64(v)&tagMask == tagPtr && v.ptrSubType() == ptrSubChannel
}

// IsFrame reports whether the value is a native frame facade.
func (v Value) IsFrame() bool { return v.Type() == TypeFrame }

// IsKeyedFrame reports whether the value is a native keyed frame facade.
func (v Value) IsKeyedFrame() bool { return v.Type() == TypeKeyedFrame }

func (v Value) nativeFramePayloadKind() (NativePayloadKind, bool) {
	if !v.IsTable() {
		return NativePayloadNone, false
	}
	tbl := v.Table()
	if tbl == nil {
		return NativePayloadNone, false
	}
	return tbl.NativeFramePayloadKind()
}

// ---------------------------------------------------------------------------
// Value accessors
// ---------------------------------------------------------------------------

func (v Value) Bool() bool {
	return uint64(v)&1 != 0
}

func (v Value) Int() int64 {
	// Branchless sign-extend 48-bit integer to 64-bit.
	return int64(uint64(v)<<16) >> 16
}

func (v Value) Float() float64 {
	return math.Float64frombits(uint64(v))
}

func (v Value) Number() float64 {
	if v.IsInt() {
		return float64(v.Int())
	}
	return math.Float64frombits(uint64(v))
}

func (v Value) Str() string {
	if !v.IsString() {
		return ""
	}
	if lz := v.lazyString(); lz != nil {
		return lz.materialize()
	}
	p := v.ptrPayload()
	if p == nil {
		return ""
	}
	return *(*string)(p)
}

func (v Value) Table() *Table {
	if !v.IsTable() {
		return nil
	}
	if fr := v.FixedRecord(); fr != nil {
		return fr.materialize()
	}
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	return (*Table)(p)
}

// Closure returns the value as *runtime.Closure, or nil.
func (v Value) Closure() *Closure {
	if uint64(v)&tagMask != tagPtr {
		return nil
	}
	sub := v.ptrSubType()
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	switch sub {
	case ptrSubClosure:
		return (*Closure)(p)
	case ptrSubAnyFunction:
		// Recover from gcRoots and type-assert.
		if obj := lookupIface(p); obj != nil {
			c, _ := obj.(*Closure)
			return c
		}
		return nil
	default:
		return nil
	}
}

// GoFunction returns the value as *GoFunction, or nil.
func (v Value) GoFunction() *GoFunction {
	if uint64(v)&tagMask != tagPtr {
		return nil
	}
	sub := v.ptrSubType()
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	switch sub {
	case ptrSubGoFunction:
		return (*GoFunction)(p)
	case ptrSubAnyFunction:
		if obj := lookupIface(p); obj != nil {
			gf, _ := obj.(*GoFunction)
			return gf
		}
		return nil
	default:
		return nil
	}
}

// Ptr reconstructs the original interface{} value from the NaN-boxed pointer.
func (v Value) Ptr() any {
	if uint64(v)&tagMask != tagPtr {
		return nil
	}
	sub := v.ptrSubType()
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	switch sub {
	case ptrSubTable:
		return (*Table)(p)
	case ptrSubFixedRecord:
		return (*FixedRecord)(p).materialize()
	case ptrSubString:
		return *(*string)(p)
	case ptrSubLazyString:
		return (*lazyString)(p).materialize()
	case ptrSubClosure:
		return (*Closure)(p)
	case ptrSubGoFunction:
		return (*GoFunction)(p)
	case ptrSubCoroutine:
		return (*Coroutine)(p)
	case ptrSubChannel:
		return (*Channel)(p)
	case ptrSubDenseArray:
		return (*DenseArray)(p)
	case ptrSubSoA:
		return (*SoA)(p)
	case ptrSubAnyFunction, ptrSubAnyCoro, ptrSubVMClosure:
		// Recover the original interface from gcRoots.
		return lookupIface(p)
	case ptrSubVMCoroutine:
		if vmCoroutinePtrResolver != nil {
			return vmCoroutinePtrResolver(p)
		}
		return nil
	default:
		return nil
	}
}

func (v Value) Coroutine() *Coroutine {
	if uint64(v)&tagMask != tagPtr {
		return nil
	}
	sub := v.ptrSubType()
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	switch sub {
	case ptrSubCoroutine:
		return (*Coroutine)(p)
	case ptrSubAnyCoro:
		if obj := lookupIface(p); obj != nil {
			c, _ := obj.(*Coroutine)
			return c
		}
		return nil
	default:
		return nil
	}
}

func (v Value) Channel() *Channel {
	if !v.IsChannel() {
		return nil
	}
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	return (*Channel)(p)
}

// ---------------------------------------------------------------------------
// TypeName, Truthiness, Equality
// ---------------------------------------------------------------------------

func (v Value) TypeName() string {
	if kind, ok := v.nativeFramePayloadKind(); ok {
		if name, ok := kind.TypeName(); ok {
			return name
		}
	}
	switch v.Type() {
	case TypeNil:
		return "nil"
	case TypeBool:
		return "boolean"
	case TypeInt, TypeFloat:
		return "number"
	case TypeString:
		return "string"
	case TypeTable:
		return "table"
	case TypeFunction:
		return "function"
	case TypeCoroutine:
		return "coroutine"
	case TypeChannel:
		return "channel"
	case TypeDenseArray:
		return "array"
	case TypeSoA:
		return "soa"
	case TypeFrame:
		return "frame"
	case TypeKeyedFrame:
		return "keyed frame"
	default:
		return "unknown"
	}
}

func (v Value) Truthy() bool {
	return uint64(v) != valNil && uint64(v) != valFalse
}

func (v Value) Equal(other Value) bool {
	if v.IsFloat() && math.IsNaN(v.Float()) {
		return false
	}
	if other.IsFloat() && math.IsNaN(other.Float()) {
		return false
	}
	// Fast path: identical bit patterns.
	if uint64(v) == uint64(other) {
		return true
	}

	vt := v.Type()
	ot := other.Type()

	if vt != ot {
		// Cross-type number equality (int == float).
		if v.IsNumber() && other.IsNumber() {
			return v.Number() == other.Number()
		}
		return false
	}

	switch vt {
	case TypeNil:
		return true
	case TypeBool:
		return v.Bool() == other.Bool()
	case TypeInt:
		return v.Int() == other.Int()
	case TypeFloat:
		return v.Float() == other.Float()
	case TypeString:
		return v.Str() == other.Str()
	case TypeTable, TypeFunction, TypeCoroutine, TypeChannel, TypeDenseArray, TypeSoA, TypeFrame, TypeKeyedFrame:
		// Pointer identity: compare the raw address (strip sub-type bits).
		return v.ptrPayload() == other.ptrPayload()
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Arithmetic / conversion helpers
// ---------------------------------------------------------------------------

func (v Value) ToNumber() (Value, bool) {
	if v.IsInt() || v.IsFloat() {
		return v, true
	}
	if !v.IsString() {
		return NilValue(), false
	}
	return ParseNumberString(v.Str())
}

// ---------------------------------------------------------------------------
// fmt.Stringer
// ---------------------------------------------------------------------------

func (v Value) String() string {
	if kind, ok := v.nativeFramePayloadKind(); ok {
		if name, ok := kind.TypeName(); ok {
			return fmt.Sprintf("%s: %p", name, v.ptrPayload())
		}
	}
	switch v.Type() {
	case TypeNil:
		return "nil"
	case TypeBool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case TypeInt:
		return strconv.FormatInt(v.Int(), 10)
	case TypeFloat:
		f := v.Float()
		s := strconv.FormatFloat(f, 'g', -1, 64)
		if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") && !strings.Contains(s, "Inf") && !strings.Contains(s, "NaN") {
			s += ".0"
		}
		return s
	case TypeString:
		return v.Str()
	case TypeTable:
		return fmt.Sprintf("table: %p", v.ptrPayload())
	case TypeFrame:
		return fmt.Sprintf("frame: %p", v.ptrPayload())
	case TypeKeyedFrame:
		return fmt.Sprintf("keyed frame: %p", v.ptrPayload())
	case TypeFunction:
		if c := v.Closure(); c != nil {
			return fmt.Sprintf("function: %p", c)
		}
		if gf := v.GoFunction(); gf != nil {
			return fmt.Sprintf("function: %s", gf.Name)
		}
		return "function: <unknown>"
	case TypeCoroutine:
		return fmt.Sprintf("coroutine: %p", v.ptrPayload())
	case TypeChannel:
		return fmt.Sprintf("channel: %p", v.ptrPayload())
	case TypeDenseArray:
		if a := v.DenseArray(); a != nil {
			return a.String()
		}
		return "array<nil>[]"
	case TypeSoA:
		if s := v.SoA(); s != nil {
			return s.String()
		}
		return "soa<nil>"
	default:
		return "unknown"
	}
}

func (v Value) hashKey() Value {
	return v
}

func (v Value) LessThan(other Value) (bool, bool) {
	if v.IsNumber() && other.IsNumber() {
		return v.Number() < other.Number(), true
	}
	if v.IsString() && other.IsString() {
		return v.Str() < other.Str(), true
	}
	return false, false
}

func floatIsInt(f float64) bool {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return false
	}
	return f == math.Trunc(f) && f >= math.MinInt64 && f <= math.MaxInt64
}

// ---------------------------------------------------------------------------
// Raw access (for VM / JIT)
// ---------------------------------------------------------------------------

// Raw returns the underlying uint64 bits.
func (v Value) Raw() uint64 {
	return uint64(v)
}

// FromRaw constructs a Value from raw uint64 bits (no validation).
func FromRaw(bits uint64) Value {
	return Value(bits)
}

// ---------------------------------------------------------------------------
// NaN-boxing tag constants (exported for JIT / nanbox package)
// ---------------------------------------------------------------------------

const (
	NanBits     = nanBits
	TagNil      = tagNil
	TagBool     = tagBool
	TagInt      = tagInt
	TagPtr      = tagPtr
	TagMask     = tagMask
	PayloadMask = payloadMask
	ValNil      = valNil
	ValFalse    = valFalse
	ValTrue     = valTrue
)

// MakeNilSlice creates a []Value of length n with all elements set to NilValue().
// With NaN-boxing, Go's zero value (0) is float64(0.0), NOT nil.
// Use this instead of make([]Value, n) whenever uninitialized slots must read as nil.
func MakeNilSlice(n int) []Value {
	s := make([]Value, n)
	nv := NilValue()
	for i := range s {
		s[i] = nv
	}
	return s
}

// MakeNilSliceCap creates a []Value of length n and capacity cap with all elements set to NilValue().
func MakeNilSliceCap(n, cap int) []Value {
	s := make([]Value, n, cap)
	nv := NilValue()
	for i := range s {
		s[i] = nv
	}
	return s
}

// ReuseValueSlice1 returns a one-element Value slice backed by buf when
// possible. It is used by VM/JIT return paths where the caller immediately
// consumes or owns the reusable result buffer.
func ReuseValueSlice1(buf []Value, v Value) []Value {
	if cap(buf) > 0 {
		buf = buf[:1]
		buf[0] = v
		return buf
	}
	return []Value{v}
}
